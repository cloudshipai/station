# @cloudshipai/opencode-plugin

OpenCode plugin for Station integration via NATS.

## Overview

This plugin enables Station agents and workflows to delegate coding tasks to OpenCode. It:

1. Connects to NATS on OpenCode startup (graceful degradation if unavailable)
2. Consumes coding tasks from Station via NATS subjects
3. Manages workspaces and git operations on OpenCode's filesystem
4. Streams execution events back to Station for observability
5. Provides custom tools for KV/Object Store interaction
6. Maintains session state in NATS KV for cross-call persistence

## Installation

```bash
# In your opencode.json
{
  "plugins": ["@cloudshipai/opencode-plugin"]
}
```

## Configuration

Environment variables:

- `NATS_URL` - NATS server URL (default: `nats://localhost:4222`)
- `OPENCODE_WORKSPACE_DIR` - Base directory for workspaces (default: `/opencode/workspaces`)

## Message Protocol

### Task (Station -> Plugin)

Published to `station.coding.task`:

```typescript
interface CodingTask {
  taskID: string;
  session: {
    name: string;
    continue?: boolean;
  };
  workspace: {
    name: string;
    git?: {
      url: string;
      branch?: string;
      token?: string;
    };
  };
  prompt: string;
  agent?: string;
  callback: {
    streamSubject: string;
    resultSubject: string;
  };
}
```

### Stream Events (Plugin -> Station)

Published to `callback.streamSubject`:

- `session_created` / `session_reused`
- `workspace_created` / `workspace_reused`
- `git_clone` / `git_pull`
- `prompt_sent`
- `text` / `thinking`
- `tool_start` / `tool_end`
- `error`

### Result (Plugin -> Station)

Published to `callback.resultSubject`:

```typescript
interface CodingResult {
  taskID: string;
  status: "completed" | "error" | "timeout";
  result?: string;
  error?: string;
  session: { name: string; opencodeID: string; messageCount: number };
  workspace: { name: string; path: string; git?: { branch: string; commit: string; dirty: boolean } };
  metrics: { duration: number; toolCalls: number; streamEvents: number };
}
```

## Custom Tools

The plugin registers these tools for agents:

- `station_kv_get` - Get value from NATS KV
- `station_kv_set` - Set value in NATS KV
- `station_session_info` - Get session metadata

## Development

```bash
cd opencode-plugin
bun install
bun run typecheck
bun run build
```

## Architecture

```
Station Container                    OpenCode Container
+------------------+                +---------------------------+
|  Agent/Workflow  |                |  @cloudshipai/opencode-plugin  |
|                  |   NATS         |                           |
|  coding_tool()  -+--------------->|  TaskHandler              |
|                  |                |    |                      |
|  <stream events--+-<--------------|    +-> WorkspaceManager   |
|  <final result---+-<--------------|    +-> SessionManager     |
+------------------+                +---------------------------+
```
