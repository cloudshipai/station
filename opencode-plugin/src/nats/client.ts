import {
  connect,
  type NatsConnection,
  type JetStreamClient,
  type KV,
  type ObjectStore,
  StringCodec,
  type Subscription,
} from "nats";
import type { PluginConfig } from "../types";

const sc = StringCodec();

export class NATSClient {
  private nc: NatsConnection | null = null;
  private js: JetStreamClient | null = null;
  private kvSessions: KV | null = null;
  private kvState: KV | null = null;
  private objectStore: ObjectStore | null = null;
  private config: PluginConfig;
  private subscriptions: Subscription[] = [];

  constructor(config: PluginConfig) {
    this.config = config;
  }

  async connect(): Promise<boolean> {
    try {
      this.nc = await connect({
        servers: this.config.nats.url,
        timeout: this.config.nats.connectTimeout,
        reconnect: this.config.nats.reconnect,
        maxReconnectAttempts: this.config.nats.maxReconnectAttempts,
      });

      console.log(`[station-plugin] Connected to NATS: ${this.config.nats.url}`);

      this.js = this.nc.jetstream();
      await this.initializeKV();
      await this.initializeObjectStore();

      return true;
    } catch (err) {
      console.warn(`[station-plugin] NATS connection failed, running in standalone mode: ${err}`);
      return false;
    }
  }

  private async initializeKV(): Promise<void> {
    if (!this.js) return;

    const jsm = await this.js.jetstreamManager();

    try {
      this.kvSessions = await this.js.views.kv(this.config.kv.sessions);
    } catch {
      await jsm.streams.add({
        name: `KV_${this.config.kv.sessions}`,
        subjects: [`$KV.${this.config.kv.sessions}.>`],
      });
      this.kvSessions = await this.js.views.kv(this.config.kv.sessions);
    }

    try {
      this.kvState = await this.js.views.kv(this.config.kv.state);
    } catch {
      await jsm.streams.add({
        name: `KV_${this.config.kv.state}`,
        subjects: [`$KV.${this.config.kv.state}.>`],
      });
      this.kvState = await this.js.views.kv(this.config.kv.state);
    }
  }

  private async initializeObjectStore(): Promise<void> {
    if (!this.js) return;

    try {
      this.objectStore = await this.js.views.os(this.config.objectStore.files);
    } catch {
      const jsm = await this.js.jetstreamManager();
      await jsm.streams.add({
        name: `OBJ_${this.config.objectStore.files}`,
        subjects: [`$O.${this.config.objectStore.files}.>`],
      });
      this.objectStore = await this.js.views.os(this.config.objectStore.files);
    }
  }

  isConnected(): boolean {
    return this.nc !== null && !this.nc.isClosed();
  }

  async subscribe(
    subject: string,
    handler: (data: string, reply?: string) => Promise<void>
  ): Promise<Subscription | null> {
    if (!this.nc) return null;

    const sub = this.nc.subscribe(subject, {
      callback: async (err, msg) => {
        if (err) {
          console.error(`[station-plugin] Subscription error on ${subject}:`, err);
          return;
        }
        const data = sc.decode(msg.data);
        await handler(data, msg.reply);
      },
    });

    this.subscriptions.push(sub);
    return sub;
  }

  async publish(subject: string, data: string): Promise<void> {
    if (!this.nc) {
      console.warn(`[station-plugin] Cannot publish to ${subject}: not connected`);
      return;
    }
    this.nc.publish(subject, sc.encode(data));
  }

  async kvGet(key: string, bucket: "sessions" | "state" = "state"): Promise<string | null> {
    const kv = bucket === "sessions" ? this.kvSessions : this.kvState;
    if (!kv) return null;

    try {
      const entry = await kv.get(key);
      if (!entry || !entry.value) return null;
      return sc.decode(entry.value);
    } catch {
      return null;
    }
  }

  async kvPut(key: string, value: string, bucket: "sessions" | "state" = "state"): Promise<boolean> {
    const kv = bucket === "sessions" ? this.kvSessions : this.kvState;
    if (!kv) return false;

    try {
      await kv.put(key, sc.encode(value));
      return true;
    } catch (err) {
      console.error(`[station-plugin] KV put failed for ${key}:`, err);
      return false;
    }
  }

  async kvDelete(key: string, bucket: "sessions" | "state" = "state"): Promise<boolean> {
    const kv = bucket === "sessions" ? this.kvSessions : this.kvState;
    if (!kv) return false;

    try {
      await kv.delete(key);
      return true;
    } catch {
      return false;
    }
  }

  async fileGet(name: string): Promise<Uint8Array | null> {
    if (!this.objectStore) return null;

    try {
      const result = await this.objectStore.get(name);
      if (!result) return null;

      const reader = result.data.getReader();
      const chunks: Uint8Array[] = [];
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value) chunks.push(value);
      }

      const totalLength = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
      const combined = new Uint8Array(totalLength);
      let offset = 0;
      for (const chunk of chunks) {
        combined.set(chunk, offset);
        offset += chunk.length;
      }
      return combined;
    } catch {
      return null;
    }
  }

  async filePut(name: string, data: Uint8Array): Promise<boolean> {
    if (!this.objectStore) return false;

    try {
      const stream = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(data);
          controller.close();
        },
      });
      await this.objectStore.put({ name }, stream);
      return true;
    } catch (err) {
      console.error(`[station-plugin] Object store put failed for ${name}:`, err);
      return false;
    }
  }

  async close(): Promise<void> {
    for (const sub of this.subscriptions) {
      sub.unsubscribe();
    }
    this.subscriptions = [];

    if (this.nc) {
      await this.nc.drain();
      this.nc = null;
    }
  }
}
