import { connect, StringCodec, type NatsConnection, type JetStreamClient } from "nats";

const sc = StringCodec();

interface CodingTask {
  taskID: string;
  session: { name: string; continue?: boolean };
  workspace: { name: string; git?: { url: string; branch?: string; token?: string } };
  prompt: string;
  agent?: string;
  callback: { streamSubject: string; resultSubject: string };
}

interface CodingResult {
  taskID: string;
  status: "completed" | "error" | "timeout" | "cancelled";
  result?: string;
  error?: string;
  session: { name: string; opencodeID: string; messageCount: number };
  workspace: { name: string; path: string; git?: { branch: string; commit: string; dirty: boolean } };
  metrics: { duration: number; toolCalls: number; streamEvents: number };
}

interface CodingStreamEvent {
  taskID: string;
  seq: number;
  timestamp: string;
  type: string;
  content?: string;
  tool?: { name: string; callID: string; args?: Record<string, unknown>; output?: string };
}

class TestHarness {
  private nc: NatsConnection | null = null;
  private js: JetStreamClient | null = null;
  private taskSubject: string;

  constructor() {
    this.taskSubject = "station.coding.task";
  }

  async connect(url = "nats://localhost:4222"): Promise<void> {
    console.log(`Connecting to NATS at ${url}...`);
    this.nc = await connect({ servers: url });
    this.js = this.nc.jetstream();
    console.log("Connected to NATS");
  }

  async executeTask(
    prompt: string,
    options: {
      sessionName?: string;
      workspaceName?: string;
      timeout?: number;
    } = {}
  ): Promise<{ result: CodingResult; events: CodingStreamEvent[] }> {
    if (!this.nc) throw new Error("Not connected to NATS");

    const taskID = crypto.randomUUID();
    const sessionName = options.sessionName || `test-${taskID.slice(0, 8)}`;
    const workspaceName = options.workspaceName || `workspace-${taskID.slice(0, 8)}`;
    const timeout = options.timeout || 120000;

    const streamSubject = `station.coding.stream.${taskID}`;
    const resultSubject = `station.coding.result.${taskID}`;

    const events: CodingStreamEvent[] = [];
    let result: CodingResult | null = null;

    const streamSub = this.nc.subscribe(streamSubject, {
      callback: (_err, msg) => {
        const event = JSON.parse(sc.decode(msg.data)) as CodingStreamEvent;
        events.push(event);
        console.log(`[STREAM] ${event.type}: ${event.content?.slice(0, 100) || JSON.stringify(event.tool || {})}`);
      },
    });

    const resultPromise = new Promise<CodingResult>((resolve, reject) => {
      const timeoutId = setTimeout(() => {
        reject(new Error(`Task timed out after ${timeout}ms`));
      }, timeout);

      const nc = this.nc;
      if (!nc) return reject(new Error("Not connected"));
      const resultSub = nc.subscribe(resultSubject, {
        callback: (_err, msg) => {
          clearTimeout(timeoutId);
          result = JSON.parse(sc.decode(msg.data)) as CodingResult;
          resultSub.unsubscribe();
          resolve(result);
        },
      });
    });

    const task: CodingTask = {
      taskID,
      session: { name: sessionName, continue: true },
      workspace: { name: workspaceName },
      prompt,
      callback: { streamSubject, resultSubject },
    };

    console.log(`\nPublishing task ${taskID}...`);
    console.log(`  Session: ${sessionName}`);
    console.log(`  Workspace: ${workspaceName}`);
    console.log(`  Prompt: ${prompt.slice(0, 100)}...`);

    this.nc.publish(this.taskSubject, sc.encode(JSON.stringify(task)));

    try {
      result = await resultPromise;
      console.log(`\nTask completed with status: ${result.status}`);
      if (result.result) console.log(`Result: ${result.result.slice(0, 200)}...`);
      if (result.error) console.log(`Error: ${result.error}`);
      console.log(`Metrics: ${JSON.stringify(result.metrics)}`);
    } finally {
      streamSub.unsubscribe();
    }

    if (!result) throw new Error("No result received");
    return { result, events };
  }

  async close(): Promise<void> {
    if (this.nc) {
      await this.nc.drain();
      this.nc = null;
    }
  }
}

async function runTests() {
  const harness = new TestHarness();

  try {
    await harness.connect(process.env.NATS_URL || "nats://localhost:4222");

    console.log("\n=== Test 1: Simple prompt ===");
    const test1 = await harness.executeTask("Say hello and tell me what tools you have available.");

    if (test1.result.status !== "completed") {
      throw new Error(`Test 1 failed: expected completed, got ${test1.result.status}`);
    }
    console.log("Test 1 passed!");

    console.log("\n=== Test 2: Session persistence ===");
    const sessionName = "persistence-test";

    await harness.executeTask("Remember that my favorite color is blue.", { sessionName });

    const test2 = await harness.executeTask("What is my favorite color?", {
      sessionName,
    });

    if (!test2.result.result?.toLowerCase().includes("blue")) {
      console.warn("Test 2 warning: Session may not have persisted correctly");
    } else {
      console.log("Test 2 passed!");
    }

    console.log("\n=== All tests completed ===");
  } catch (err) {
    console.error("Test failed:", err);
    process.exit(1);
  } finally {
    await harness.close();
  }
}

runTests();
