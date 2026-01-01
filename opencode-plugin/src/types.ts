/**
 * Types for CloudShip Station <-> OpenCode communication via NATS
 */

// ============================================================================
// Task Message (Station -> Plugin)
// ============================================================================

export interface CodingTask {
  /** Unique task ID for correlation */
  taskID: string;

  /** Session management */
  session: {
    /** Logical session name (e.g., "wf-abc-main") */
    name: string;
    /** Continue existing session if exists (default: true) */
    continue?: boolean;
  };

  /** Workspace management */
  workspace: {
    /** Workspace directory name */
    name: string;
    /** Git repository configuration */
    git?: GitConfig;
  };

  /** The coding task/prompt */
  prompt: string;

  /** OpenCode agent to use (default: "build") */
  agent?: string;

  /** Model override */
  model?: {
    providerID: string;
    modelID: string;
  };

  /** Task timeout in milliseconds */
  timeout?: number;

  /** Callback subjects for responses */
  callback: {
    /** Subject for streaming events */
    streamSubject: string;
    /** Subject for final result */
    resultSubject: string;
  };
}

export interface GitConfig {
  /** Repository URL (https or ssh) */
  url: string;
  /** Branch to checkout */
  branch?: string;
  /** Specific commit or tag */
  ref?: string;
  /** GitHub/GitLab token for https URLs */
  token?: string;
  /** KV key containing token */
  tokenFromKV?: string;
  /** Pull latest before task (default: true) */
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
  /** Sequence number for ordering */
  seq: number;
  /** ISO timestamp */
  timestamp: string;
  type: StreamEventType;

  /** Content for text, thinking, error events */
  content?: string;

  /** Tool info for tool_start, tool_end events */
  tool?: {
    name: string;
    callID: string;
    args?: Record<string, unknown>;
    output?: string;
    duration?: number;
  };

  /** Git info for git_* events */
  git?: {
    url?: string;
    branch?: string;
    commit?: string;
  };

  /** Session info for session_* events */
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

  /** Final text response (on success) */
  result?: string;

  /** Error message (on error) */
  error?: string;
  errorType?: string;

  /** Session metadata */
  session: {
    name: string;
    opencodeID: string;
    messageCount: number;
  };

  /** Workspace metadata */
  workspace: {
    name: string;
    path: string;
    git?: {
      branch: string;
      commit: string;
      dirty: boolean;
    };
  };

  /** Execution metrics */
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
    maxReconnectAttempts: -1, // infinite
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
