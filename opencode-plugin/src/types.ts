// ============================================================================
// Task Message (Station -> Plugin)
// ============================================================================

export interface CodingTask {
	taskID: string;

	session: {
		name: string;
		continue?: boolean;
	};

	workspace: {
		name: string;
		git?: GitConfig;
	};

	prompt: string;
	agent?: string;

	model?: {
		providerID: string;
		modelID: string;
	};

	timeout?: number;

	callback: {
		streamSubject: string;
		resultSubject: string;
	};
}

export interface GitConfig {
	url: string;
	branch?: string;
	ref?: string;
	token?: string;
	tokenFromKV?: string;
	pull?: boolean;
}

// ============================================================================
// Stream Event (Plugin -> Station)
// ============================================================================

export type StreamEventType =
	| "session_created"
	| "session_reused"
	| "workspace_created"
	| "workspace_reused"
	| "git_clone"
	| "git_pull"
	| "git_checkout"
	| "prompt_sent"
	| "text"
	| "thinking"
	| "tool_start"
	| "tool_end"
	| "error";

export interface CodingStreamEvent {
	taskID: string;
	seq: number;
	timestamp: string;
	type: StreamEventType;
	content?: string;

	tool?: {
		name: string;
		callID: string;
		args?: Record<string, unknown>;
		output?: string;
		duration?: number;
	};

	git?: {
		url?: string;
		branch?: string;
		commit?: string;
	};

	session?: {
		name: string;
		opencodeID: string;
	};
}

// ============================================================================
// Result Message (Plugin -> Station)
// ============================================================================

export type ResultStatus = "completed" | "error" | "timeout" | "cancelled";

export interface CodingResult {
	taskID: string;
	status: ResultStatus;
	result?: string;
	error?: string;
	errorType?: string;

	session: {
		name: string;
		opencodeID: string;
		messageCount: number;
	};

	workspace: {
		name: string;
		path: string;
		git?: {
			branch: string;
			commit: string;
			dirty: boolean;
		};
	};

	metrics: {
		duration: number;
		promptTokens?: number;
		completionTokens?: number;
		toolCalls: number;
		streamEvents: number;
	};
}

// ============================================================================
// Session State (NATS KV)
// ============================================================================

export interface SessionState {
	sessionName: string;
	opencodeID: string;
	workspaceName: string;
	workspacePath: string;
	created: string;
	lastUsed: string;
	messageCount: number;

	git?: {
		url: string;
		branch: string;
		lastCommit: string;
	};
}

// ============================================================================
// Plugin Configuration
// ============================================================================

export interface PluginConfig {
	nats: {
		url: string;
		connectTimeout: number;
		reconnect: boolean;
		maxReconnectAttempts: number;
	};
	subjects: {
		task: string;
	};
	kv: {
		sessions: string;
		todos: string;
		state: string;
	};
	objectStore: {
		files: string;
	};
	workspace: {
		baseDir: string;
		cleanupAfterHours: number;
	};
	git: {
		defaultBranch: string;
		pullOnReuse: boolean;
	};
}

export const DEFAULT_CONFIG: PluginConfig = {
	nats: {
		url: process.env.NATS_URL || "nats://localhost:4222",
		connectTimeout: 5000,
		reconnect: true,
		maxReconnectAttempts: -1,
	},
	subjects: {
		task: "station.coding.task",
	},
	kv: {
		sessions: "opencode-sessions",
		todos: "opencode-todos",
		state: "opencode-state",
	},
	objectStore: {
		files: "opencode-files",
	},
	workspace: {
		baseDir: process.env.OPENCODE_WORKSPACE_DIR || "/opencode/workspaces",
		cleanupAfterHours: 72,
	},
	git: {
		defaultBranch: "main",
		pullOnReuse: true,
	},
};
