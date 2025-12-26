# Sandbox V2: Code Mode

## Overview

Code Mode extends Station's sandbox capabilities from ephemeral compute containers (V1/Dagger) to persistent, workflow-scoped **full Linux environments**. This enables agents to work like human developers: run any shell commands, install packages, compile code, iterate on projects, and maintain state across multiple tool calls.

## Motivation

### V1 (Compute Mode) Limitations
- Each `sandbox_run` creates a fresh container
- No file persistence between calls
- All code must be passed inline
- Cannot iteratively build/debug code

### V2 (Code Mode) Benefits
- Persistent container per workflow
- Files persist across agent turns
- Multiple tools for fine-grained control
- Supports iterative development patterns

## Design

### Two Modes, Same Frontmatter

```yaml
# V1: Compute Mode (default, backward compatible)
sandbox: python
# or
sandbox:
  runtime: python
  mode: compute  # explicit, same as omitting mode

# V2: Code Mode
sandbox:
  runtime: python
  mode: code
  session: workflow  # share across workflow steps
```

### Mode Comparison

| Aspect | Compute Mode (V1) | Code Mode (V2) |
|--------|-------------------|----------------|
| Backend | Dagger | Docker |
| Lifecycle | Ephemeral per-call | Persistent per-workflow |
| Tool | `sandbox_run` | `sandbox_open`, `sandbox_exec`, `sandbox_fs_*` |
| Files | Via `files` param | Persist in container |
| Use Case | Data processing, calculations | Code development, iterative work |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Agent Execution Engine                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  sandbox:                                                        │
│    mode: compute  ──────────►  SandboxToolFactory                │
│                                     │                            │
│                                     ▼                            │
│                              sandbox_run (Dagger)                │
│                                                                  │
│  sandbox:                                                        │
│    mode: code     ──────────►  CodeModeToolFactory               │
│                                     │                            │
│                                     ▼                            │
│                         ┌───────────────────┐                    │
│                         │  SessionManager    │                   │
│                         │  (workflow-scoped) │                   │
│                         └─────────┬─────────┘                    │
│                                   │                              │
│                                   ▼                              │
│                         ┌───────────────────┐                    │
│                         │  DockerBackend    │                    │
│                         │  (container ops)  │                    │
│                         └───────────────────┘                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Code Mode Tools

### `sandbox_open`
Get or create a persistent Linux sandbox session.

```json
{
  "name": "sandbox_open",
  "input": {
    "image": "linux"  // shortcuts: "python", "node", "linux", or any Docker image
  },
  "output": {
    "session_id": "wf_abc123_sandbox",
    "image": "ubuntu:22.04",
    "workdir": "/work",
    "status": "ready"
  }
}
```

Image shortcuts:
- `linux` / `bash` / `shell` → `ubuntu:22.04` (default)
- `python` → `python:3.11-slim`
- `node` → `node:20-slim`
- Any Docker image directly: `golang:1.22`, `rust:1.75`, `alpine:3.19`

### `sandbox_exec`
Execute any shell command in the Linux sandbox. Full shell access - install packages, compile code, run scripts, etc.

```json
{
  "name": "sandbox_exec",
  "input": {
    "command": "apt update && apt install -y curl",
    "timeout_seconds": 60,
    "workdir": "/work"  // optional
  },
  "output": {
    "exit_code": 0,
    "stdout": "...",
    "stderr": "",
    "duration_ms": 1500
  }
}
```

Examples:
- `apt update && apt install -y build-essential`
- `pip install pandas numpy`
- `gcc -o app main.c && ./app`
- `curl -s https://api.example.com/data | jq .`
- `npm install && npm run build`

### `sandbox_fs_write`
Write a file to the sandbox.

```json
{
  "name": "sandbox_fs_write",
  "input": {
    "path": "main.py",
    "content": "print('Hello, World!')",
    "encoding": "text"  // or "base64"
  },
  "output": {
    "path": "/work/main.py",
    "size_bytes": 24
  }
}
```

### `sandbox_fs_read`
Read a file from the sandbox.

```json
{
  "name": "sandbox_fs_read",
  "input": {
    "path": "main.py",
    "encoding": "text"  // or "base64"
  },
  "output": {
    "path": "/work/main.py",
    "content": "print('Hello, World!')",
    "size_bytes": 24
  }
}
```

### `sandbox_fs_list`
List directory contents.

```json
{
  "name": "sandbox_fs_list",
  "input": {
    "path": ".",
    "recursive": false
  },
  "output": {
    "entries": [
      {"name": "main.py", "type": "file", "size_bytes": 24},
      {"name": "data", "type": "directory"}
    ]
  }
}
```

### `sandbox_fs_delete`
Delete a file or directory.

```json
{
  "name": "sandbox_fs_delete",
  "input": {
    "path": "temp.txt",
    "recursive": false
  },
  "output": {
    "deleted": true,
    "path": "/work/temp.txt"
  }
}
```

### `sandbox_close`
Explicitly close a sandbox session (optional, auto-cleaned on workflow end).

```json
{
  "name": "sandbox_close",
  "input": {},
  "output": {
    "closed": true,
    "session_id": "wf_abc123_sandbox"
  }
}
```

## Session Management

### Session Scoping

Sessions are scoped to workflows to enable multi-step development:

```
Workflow Run: wf_abc123
├── Step 1: code-writer agent
│   └── sandbox_open → creates session
│   └── sandbox_fs_write → writes code
├── Step 2: tester agent  
│   └── sandbox_open → reuses same session
│   └── sandbox_exec → runs tests
├── Step 3: debugger agent
│   └── sandbox_open → same session, files still there
│   └── sandbox_fs_read → reads test output
└── Workflow completes → session auto-destroyed
```

### Session Key Format

```
SessionKey = "{scope_type}_{scope_id}_sandbox"

Examples:
- Workflow scope: "workflow_wf_abc123_sandbox"  
- Agent scope:    "agent_run_456_sandbox"
```

### Lifecycle

1. **Creation**: First `sandbox_open` in a workflow creates the container
2. **Reuse**: Subsequent `sandbox_open` calls return existing session
3. **Cleanup**: Container destroyed when workflow completes
4. **Idle Timeout**: Optional cleanup of abandoned sessions (configurable)

## Configuration

### Agent Frontmatter

```yaml
---
model: openai/gpt-4o
metadata:
  name: "Code Developer"
sandbox:
  mode: code
  runtime: linux    # default, or "python", "node", or any Docker image
  session: workflow  # or "agent" for per-agent-run isolation
  limits:
    timeout_seconds: 300
    max_file_size_bytes: 10485760  # 10MB
    max_files: 100
---

You are a developer with access to a full Linux sandbox. You can install packages,
compile code, run scripts, and execute any shell commands.
```

### Environment Variables

```bash
# Enable code mode (requires Docker)
STATION_SANDBOX_CODE_MODE_ENABLED=true

# Docker configuration
DOCKER_HOST=unix:///var/run/docker.sock

# Session cleanup
SANDBOX_SESSION_IDLE_TIMEOUT=30m
SANDBOX_SESSION_MAX_LIFETIME=4h
```

## Implementation

### File Structure

```
internal/services/
├── sandbox_service.go           # V1 Dagger (unchanged)
├── sandbox_tool.go              # V1 tool factory (updated)
├── sandbox_code_types.go        # Shared types for V2
├── sandbox_backend.go           # SandboxBackend interface
├── sandbox_docker_backend.go    # Docker implementation
├── sandbox_session_manager.go   # Session lifecycle
├── sandbox_code_tools.go        # V2 GenKit tools
└── sandbox_code_tools_test.go   # Tests
```

### Backend Interface

```go
type SandboxBackend interface {
    CreateSession(ctx context.Context, cfg SessionConfig) (*CodeSession, error)
    DestroySession(ctx context.Context, sessionID string) error
    
    Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error)
    
    WriteFile(ctx context.Context, sessionID, path string, content []byte) error
    ReadFile(ctx context.Context, sessionID, path string) ([]byte, error)
    ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error)
    DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error
}
```

### Docker Backend

Uses Docker SDK (`github.com/docker/docker/client`) to:
- Create long-running containers with `tail -f /dev/null`
- Execute commands via `docker exec`
- Copy files via tar streams
- Clean up via `docker rm -f`

### Execution Context

Tools receive workflow/agent context to resolve sessions:

```go
type ExecutionContext struct {
    WorkflowRunID string  // Set when running in workflow
    AgentRunID    string  // Always set
    AgentName     string
    Environment   string
}
```

## Security

### Container Isolation
- Unprivileged containers (no `--privileged`)
- No Docker socket mount
- Read-only root filesystem (optional)
- Resource limits (CPU, memory, disk)

### Network Policy
- Network disabled by default
- Enable via `allow_network: true`
- No access to host network

### File Limits
- Max file size: 10MB default
- Max total files: 100 default  
- Blocked paths: `/etc/passwd`, `/root`, etc.

## Testing

### Unit Tests
```bash
go test ./internal/services/... -run "SessionManager" -v
```

### Integration Tests (require Docker)
```bash
go test -tags=integration ./internal/services/... -run "DockerBackend" -v
```

### E2E Test
```bash
# Start station with sandbox enabled
STATION_SANDBOX_CODE_MODE_ENABLED=true stn serve

# Run test agent
stn agent run "Python Developer" "Write a hello world program and run it"
```

## Migration

### Backward Compatibility
- Existing `sandbox: python` or `sandbox: {runtime: python}` continues to use V1
- V2 requires explicit `mode: code`
- No changes to V1 tool schema

### Gradual Rollout
1. Deploy Docker backend alongside Dagger
2. Enable for specific agents via `mode: code`
3. Monitor session lifecycle and resource usage
4. Expand to more agents as validated

## Future Enhancements

- **containerd backend**: For Kubernetes environments without Docker
- **Snapshot/restore**: Save and restore session state
- **Pre-built images**: Custom images with pre-installed packages
- **Shared volumes**: Mount host directories for data access
- **GPU support**: CUDA containers for ML workloads
