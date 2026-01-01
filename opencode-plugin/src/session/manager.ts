import type { createOpencodeClient } from "@opencode-ai/sdk";
import type { NATSClient } from "../nats/client";
import type { SessionState } from "../types";

type OpencodeClient = ReturnType<typeof createOpencodeClient>;

export interface SessionInfo {
  name: string;
  opencodeID: string;
  created: boolean;
  messageCount: number;
}

export class SessionManager {
  private client: OpencodeClient;
  private nats: NATSClient | null;
  private localSessions: Map<string, SessionState> = new Map();

  constructor(client: OpencodeClient, nats: NATSClient | null) {
    this.client = client;
    this.nats = nats;
  }

  async resolve(
    sessionName: string,
    workspacePath: string,
    continueSession = true
  ): Promise<SessionInfo> {
    if (continueSession) {
      const existing = await this.getSession(sessionName);
      if (existing) {
        await this.updateLastUsed(sessionName);
        return {
          name: sessionName,
          opencodeID: existing.opencodeID,
          created: false,
          messageCount: existing.messageCount,
        };
      }
    }

    const result = await this.client.session.create({
      query: { directory: workspacePath },
    });

    if (result.error) {
      throw new Error(`Failed to create session: ${JSON.stringify(result.error)}`);
    }

    const session = result.data;

    const state: SessionState = {
      sessionName,
      opencodeID: session.id,
      workspaceName: workspacePath.split("/").pop() || workspacePath,
      workspacePath,
      created: new Date().toISOString(),
      lastUsed: new Date().toISOString(),
      messageCount: 0,
    };

    await this.saveSession(sessionName, state);

    return {
      name: sessionName,
      opencodeID: session.id,
      created: true,
      messageCount: 0,
    };
  }

  private async getSession(sessionName: string): Promise<SessionState | null> {
    if (this.nats) {
      const data = await this.nats.kvGet(sessionName, "sessions");
      if (data) {
        try {
          return JSON.parse(data) as SessionState;
        } catch {
          return null;
        }
      }
    }

    return this.localSessions.get(sessionName) || null;
  }

  private async saveSession(sessionName: string, state: SessionState): Promise<void> {
    if (this.nats) {
      await this.nats.kvPut(sessionName, JSON.stringify(state), "sessions");
    }
    this.localSessions.set(sessionName, state);
  }

  private async updateLastUsed(sessionName: string): Promise<void> {
    const session = await this.getSession(sessionName);
    if (session) {
      session.lastUsed = new Date().toISOString();
      await this.saveSession(sessionName, session);
    }
  }

  async incrementMessageCount(sessionName: string): Promise<void> {
    const session = await this.getSession(sessionName);
    if (session) {
      session.messageCount++;
      session.lastUsed = new Date().toISOString();
      await this.saveSession(sessionName, session);
    }
  }

  async updateGitInfo(
    sessionName: string,
    git: { url: string; branch: string; lastCommit: string }
  ): Promise<void> {
    const session = await this.getSession(sessionName);
    if (session) {
      session.git = git;
      await this.saveSession(sessionName, session);
    }
  }

  async getSessionInfo(sessionName: string): Promise<SessionState | null> {
    return this.getSession(sessionName);
  }
}
