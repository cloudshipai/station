console.log("[station-plugin] Module loaded");

import type { Hooks, Plugin, PluginInput } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { NATSClient } from "./nats/client";
import { TaskHandler } from "./nats/handler";
import { DEFAULT_CONFIG, type PluginConfig, type SessionState } from "./types";

export type {
	CodingTask,
	CodingResult,
	CodingStreamEvent,
	PluginConfig,
} from "./types";

let globalNats: NATSClient | null = null;
let globalTaskHandler: TaskHandler | null = null;
let globalConnected = false;

const plugin: Plugin = async (input: PluginInput): Promise<Hooks> => {
	const { client, $: shell, directory } = input;

	const config: PluginConfig = {
		...DEFAULT_CONFIG,
		workspace: {
			...DEFAULT_CONFIG.workspace,
			baseDir: process.env.OPENCODE_WORKSPACE_DIR || directory,
		},
	};

	setImmediate(async () => {
		try {
			globalNats = new NATSClient(config);
			globalConnected = await globalNats.connect();

			if (!globalConnected) {
				console.log("[station-plugin] Standalone mode (no NATS)");
				return;
			}

			globalTaskHandler = new TaskHandler(client, globalNats, shell, config);

			await globalNats.subscribe(config.subjects.task, async (data) => {
				await globalTaskHandler?.handle(data);
			});
			console.log("[station-plugin] Ready");
		} catch (err) {
			console.error("[station-plugin] Init error:", err);
		}
	});

	const hooks: Hooks = {
		event: async ({ event }) => {
			if (event.type === "server.instance.disposed" && globalNats) {
				console.log("[station-plugin] Shutting down");
				await globalNats.close();
				globalNats = null;
				globalTaskHandler = null;
			}
		},

		tool: {
			station_kv_get: tool({
				description: "Get a value from Station's NATS KV store",
				args: {
					key: tool.schema.string().describe("Key to retrieve"),
					bucket: tool.schema
						.enum(["sessions", "state"])
						.default("state")
						.describe("KV bucket"),
				},
				execute: async ({ key, bucket }) => {
					if (!globalConnected || !globalNats) {
						return JSON.stringify({ error: "NATS not connected" });
					}
					const value = await globalNats.kvGet(
						key,
						bucket as "sessions" | "state",
					);
					return JSON.stringify({ value });
				},
			}),

			station_kv_set: tool({
				description: "Set a value in Station's NATS KV store",
				args: {
					key: tool.schema.string().describe("Key to set"),
					value: tool.schema.string().describe("Value to store"),
					bucket: tool.schema
						.enum(["sessions", "state"])
						.default("state")
						.describe("KV bucket"),
				},
				execute: async ({ key, value, bucket }) => {
					if (!globalConnected || !globalNats) {
						return JSON.stringify({ error: "NATS not connected" });
					}
					const success = await globalNats.kvPut(
						key,
						value,
						bucket as "sessions" | "state",
					);
					return JSON.stringify({ success });
				},
			}),

			station_session_info: tool({
				description: "Get information about a Station session",
				args: {
					sessionName: tool.schema.string().describe("Session name"),
				},
				execute: async ({ sessionName }) => {
					if (!globalConnected || !globalNats) {
						return JSON.stringify({ error: "NATS not connected" });
					}
					const data = await globalNats.kvGet(sessionName, "sessions");
					if (!data) {
						return JSON.stringify({ error: "Session not found" });
					}
					try {
						const session = JSON.parse(data) as SessionState;
						return JSON.stringify({
							sessionName: session.sessionName,
							workspaceName: session.workspaceName,
							workspacePath: session.workspacePath,
							gitBranch: session.git?.branch || null,
							gitCommit: session.git?.lastCommit || null,
							messageCount: session.messageCount,
							created: session.created,
							lastUsed: session.lastUsed,
						});
					} catch {
						return JSON.stringify({ error: "Failed to parse session data" });
					}
				},
			}),
		},
	};

	return hooks;
};

export default plugin;
