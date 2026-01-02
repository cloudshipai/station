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

## Test Architecture

The integration tests run OpenCode in a **containerized environment** with our plugin installed:

```
┌─────────────────────────────────────────────────────────────┐
│                         HOST MACHINE                        │
│                                                             │
│  ┌──────────────────┐                                       │
│  │  Test Runner     │   (bun run test/*.ts)                 │
│  │  (Host)          │                                       │
│  └────────┬─────────┘                                       │
│           │ NATS @ localhost:4222                           │
├───────────┼─────────────────────────────────────────────────┤
│           │           DOCKER NETWORK                        │
│           ▼                                                 │
│  ┌─────────────────┐        ┌─────────────────────────────┐ │
│  │  test-nats-1    │◄──────►│  test-opencode-1            │ │
│  │  (NATS Server)  │        │  ┌───────────────────────┐  │ │
│  │  :4222, :8222   │        │  │ OpenCode Server       │  │ │
│  └─────────────────┘        │  │ + station-plugin.js   │  │ │
│                             │  │   ├── TaskHandler     │  │ │
│                             │  │   ├── WorkspaceManager│  │ │
│                             │  │   └── SessionManager  │  │ │
│                             │  └───────────────────────┘  │ │
│                             │  /workspaces/ (volume)      │ │
│                             └─────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Test Categories

| Test File | What It Tests | Status |
|-----------|---------------|--------|
| `smoke-test.ts` | Basic NATS connectivity, plugin loading, simple prompt | ✅ Passing |
| `git-workspace-test.ts` | Git clone, branch checkout, workspace reuse | ⚠️ Partial (non-git works, git has session race) |
| `session-test.ts` | Session creation, continuation, message counting | ✅ Implemented |
| `full-integration-test.ts` | Complete workflow with git + session + tools | ✅ Implemented |

**Current Status (Jan 2026)**:
- Plugin loading: ✅ Working (tools registered correctly)
- Non-git workspaces: ✅ Working (Test 4 passes)
- Git workspaces: ⚠️ Session race condition after clone (5s timeout hit)

### Running Tests

```bash
# Start test infrastructure
cd opencode-plugin/test
docker compose up -d

# Wait for OpenCode to be ready
sleep 5

# Run individual tests
NATS_URL=nats://localhost:4222 bun run smoke-test.ts
NATS_URL=nats://localhost:4222 bun run git-workspace-test.ts
NATS_URL=nats://localhost:4222 bun run session-test.ts

# Run full test suite
NATS_URL=nats://localhost:4222 bun run test-all.ts

# Cleanup
docker compose down -v
```

### Test Coverage Matrix

| Feature | Plugin Code | Test Coverage |
|---------|-------------|---------------|
| NATS connection | `NATSClient.connect()` | ✅ smoke-test |
| Task subscription | `NATSClient.subscribe()` | ✅ smoke-test |
| Workspace creation | `WorkspaceManager.resolve()` | ✅ git-workspace-test |
| Git clone | `WorkspaceManager.gitClone()` | ✅ git-workspace-test |
| Git pull on reuse | `WorkspaceManager.gitPull()` | ✅ git-workspace-test |
| Git dirty detection | `WorkspaceManager.isGitDirty()` | ✅ git-workspace-test |
| Branch checkout | `gitClone()` with branch | ✅ git-workspace-test |
| Token injection | `injectCredentials()` | ⚠️ Requires real private repo |
| Session creation | `SessionManager.resolve()` | ✅ session-test |
| Session continuation | `resolve()` with continue=true | ✅ session-test |
| Message counting | `incrementMessageCount()` | ✅ session-test |
| Stream events | `EventPublisher` | ✅ All tests |
| Result publishing | `EventPublisher.result()` | ✅ All tests |
| KV tools | `station_kv_get/set` | ✅ full-integration-test |
| Error handling | `TaskHandler.handle()` catch | ✅ full-integration-test |

### Docker Configuration

The test environment uses:

- **Dockerfile.opencode**: Alpine-based OpenCode with Bun, git, and plugin installed
- **docker-compose.yaml**: NATS + OpenCode services with proper networking
- **entrypoint.sh**: Debug output + server start with `--hostname 0.0.0.0`
- **opencode.json**: Plugin path configuration for containerized environment

### Critical Test Environment Details

#### Port Configuration

```yaml
# docker-compose.yaml
ports:
  - "4097:4096"  # Host 4097 -> Container 4096 (avoids conflict with host OpenCode)
```

**IMPORTANT**: Use port `4097` when testing from the host machine to avoid conflicts with any OpenCode instance running on the host (which uses default port 4096).

```bash
# From host machine
curl http://localhost:4097/experimental/tool/ids?directory=/workspaces

# From inside container (docker exec)
curl http://localhost:4096/experimental/tool/ids?directory=/workspaces
```

#### Plugin Path Configuration

The `opencode.json` **MUST** use `file://` prefix for local plugins:

```json
{
  "plugin": [
    "file:///root/.config/opencode/plugin/station-plugin.js"
  ]
}
```

**Why**: OpenCode's plugin loader (`plugin/index.ts`) requires `file://` prefix for local files, otherwise it tries to install them as npm packages and fails silently.

#### Verifying Plugin Loading

```bash
# Check plugin is configured
docker exec test-opencode-1 curl -s "http://localhost:4096/config?directory=/workspaces" | jq '.plugin'

# Check custom tools are registered
docker exec test-opencode-1 curl -s "http://localhost:4096/experimental/tool/ids?directory=/workspaces" | jq .
# Should include: station_kv_get, station_kv_set, station_session_info

# Check container logs for plugin loading
docker logs test-opencode-1 2>&1 | grep -i plugin
```

#### Known Issues

1. **OpenCode Session Race Condition**: Sessions may not be immediately queryable after creation. The plugin includes `waitForSessionReady()` with 5 second timeout.

2. **Duplicate Plugin Registration**: If plugin appears twice in config array, tools will be registered twice (harmless but indicates config issue).

3. **Host OpenCode Conflict**: If you have OpenCode running on the host (port 4096), curl commands from host will hit the wrong instance. Always use port 4097 or `docker exec`.

#### Quick Troubleshooting

```bash
# Rebuild container with fresh plugin
cd test && docker compose build opencode --no-cache && docker compose up -d

# Full restart
cd test && docker compose down && docker compose up -d

# Check if host OpenCode is running (causes port conflicts)
lsof -i :4096

# View OpenCode logs in real-time
docker logs -f test-opencode-1
```
