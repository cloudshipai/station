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
		continueSession = true,
	): Promise<SessionInfo> {
		if (continueSession) {
			const existing = await this.getSession(sessionName);
			if (existing) {
				const stillExists = await this.verifySessionExists(
					existing.opencodeID,
					workspacePath,
				);
				if (stillExists) {
					await this.updateLastUsed(sessionName);
					return {
						name: sessionName,
						opencodeID: existing.opencodeID,
						created: false,
						messageCount: existing.messageCount,
					};
				}
				console.log(
					`[station-plugin] Session ${existing.opencodeID} no longer exists, creating new`,
				);
			}
		}

		const result = await this.client.session.create({
			query: { directory: workspacePath },
		});

		if (result.error) {
			throw new Error(
				`Failed to create session: ${JSON.stringify(result.error)}`,
			);
		}

		const session = result.data;

		// Wait for session to be fully persisted to disk
		// OpenCode has a race condition where session.create returns before the session file is written
		// For git repos with many files, OpenCode may take 15+ seconds to index
		await this.waitForSessionReady(session.id, workspacePath, 50, 100); // 5 seconds should be enough now

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

	private async verifySessionExists(
		opencodeID: string,
		workspacePath: string,
	): Promise<boolean> {
		try {
			const result = await this.client.session.get({
				path: { id: opencodeID },
				query: { directory: workspacePath },
			});
			return !result.error && !!result.data;
		} catch {
			return false;
		}
	}

	/**
	 * Wait for OpenCode to persist the session to disk.
	 *
	 * OpenCode has a race condition where session.create() returns before the session
	 * JSON file is written to storage. If we send a prompt before the file exists,
	 * we get a NotFoundError that manifests as "JSON Parse error: Unexpected EOF".
	 *
	 * @param sessionId - The OpenCode session ID to wait for
	 * @param maxAttempts - Max polling attempts (default 50 = 5 seconds)
	 * @param intervalMs - Polling interval in ms (default 100)
	 */
	private async waitForSessionReady(
		sessionId: string,
		workspacePath: string,
		maxAttempts = 50,
		intervalMs = 100,
	): Promise<void> {
		const startTime = Date.now();

		for (let attempt = 1; attempt <= maxAttempts; attempt++) {
			const exists = await this.verifySessionExists(sessionId, workspacePath);
			if (exists) {
				const elapsed = Date.now() - startTime;
				if (attempt > 1) {
					console.log(
						`[station-plugin] Session ${sessionId} ready after ${attempt} attempts (${elapsed}ms)`,
					);
				}
				return;
			}

			if (attempt % 10 === 0) {
				console.log(
					`[station-plugin] Waiting for session ${sessionId}... attempt ${attempt}/${maxAttempts}`,
				);
			}

			await new Promise((resolve) => setTimeout(resolve, intervalMs));
		}

		const totalWait = maxAttempts * intervalMs;
		console.error(
			`[station-plugin] Session ${sessionId} NOT ready after ${maxAttempts} attempts (${totalWait}ms)`,
		);
		throw new Error(
			`Session ${sessionId} not persisted after ${totalWait}ms - OpenCode storage race condition`,
		);
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

	private async saveSession(
		sessionName: string,
		state: SessionState,
	): Promise<void> {
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
		git: { url: string; branch: string; lastCommit: string },
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
