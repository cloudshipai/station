# PRD: Station Sandbox V2 - Code Mode (Persistent Sessions)

> **Status**: Draft  
> **Created**: 2025-12-26  
> **Layer**: Primitives (Foundation)  
> **Depends On**: Sandbox V1 (Compute Mode) - COMPLETE

## 1) Overview

### Evolution from V1

Station Sandbox V1 provides **ephemeral compute** - each `sandbox_run` call creates a fresh container, executes code, and destroys it. This is perfect for one-shot transformations.

V2 adds **Code Mode** - persistent sandbox sessions where agents can iteratively write, run, and debug code across multiple tool calls, with the filesystem preserved throughout.

### Two Modes, One API Surface

| Mode | Use Case | Session | Lifecycle | Tools |
|------|----------|---------|-----------|-------|
| **Compute Mode** (V1) | One-shot transforms, calculations, parsing | Ephemeral per-call | Create → Execute → Destroy | `sandbox_run` |
| **Code Mode** (V2) | Iterative development, write→run→debug cycles | Persistent session | Open → [Exec/FS ops]* → Close | `sandbox_open`, `sandbox_exec`, `sandbox_fs_*`, `sandbox_close` |

### Problem Statement

For complex coding tasks, agents need to:
1. Write code to files
2. Execute and see errors
3. Fix the code
4. Re-run until working
5. Build on previous work (files persist)

With V1's ephemeral model, each execution loses all previous state. Agents must re-inject all files on every call, making iterative development impractical.

### Goals (V2)

1. Add **persistent sandbox sessions** with workspace that survives across tool calls
2. Expose **file operations** as first-class tools (`sandbox_fs_write`, `sandbox_fs_read`, etc.)
3. Support **workflow-scoped sessions** - all agents in a workflow share the same sandbox
4. Enable **resource limits per-agent** via frontmatter
5. Abstract container backend to support **Docker (dev)** and optionally **containerd (prod)**

### Non-goals (V2)

1. Snapshots/checkpoints (V3)
2. SSE/WebSocket streaming (V3)
3. Network access with egress rules (use pre-baked images for now)
4. Multi-node distributed execution
5. Interactive REPL/debugging

### Inspiration: AutoGen's CodeExecutor

AutoGen's `DockerCommandLineCodeExecutor` provides a good reference:

```python
# AutoGen pattern - persistent container across executions
class DockerCommandLineCodeExecutor(CodeExecutor):
    async def start(self) -> None: ...
    async def stop(self) -> None: ...
    async def execute_code_blocks(self, code_blocks, cancellation_token) -> CodeResult: ...
    
# Container stays running, work_dir persists
async with DockerCommandLineCodeExecutor(
    image="python:3-slim",
    work_dir="/workspace",
    timeout=60,
) as executor:
    result1 = await executor.execute_code_blocks([CodeBlock(code="...", language="python")])
    result2 = await executor.execute_code_blocks([CodeBlock(code="...", language="python")])
    # Files from result1 are still there for result2
```

Our design follows similar patterns but exposes finer-grained tools for LLM agents.

---

## 2) Architecture

### 2.1 Session Scoping

**Key insight**: Within a workflow run, all agents should share the same sandbox session by default.

```
Workflow: "debug-and-fix"
├── Step 1: analyzer-agent (code mode) → writes /workspace/analysis.json
├── Step 2: fixer-agent (code mode) → reads analysis.json, writes /workspace/fix.py  
├── Step 3: tester-agent (code mode) → runs fix.py, writes /workspace/results.txt
└── Loop back to Step 1 if tests fail → ALL FILES STILL THERE
```

**Session Resolution Logic**:

```go
type SessionKey struct {
    Namespace string // "workflow", "agent", or "user" (future)
    ID        string // workflow_run_id, agent_run_id, etc.
    Key       string // session name within namespace (default: "default")
}

func resolveSandboxSession(ctx ExecutionContext) SessionKey {
    // If in a workflow, use workflow-scoped session
    if ctx.WorkflowRunID != "" {
        return SessionKey{
            Namespace: "workflow",
            ID:        ctx.WorkflowRunID,
            Key:       ctx.SandboxSessionKey, // from frontmatter or "default"
        }
    }
    
    // Standalone agent run - ephemeral session tied to run
    return SessionKey{
        Namespace: "agent", 
        ID:        ctx.AgentRunID,
        Key:       "default",
    }
}
```

### 2.2 Session Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                     Workflow Run Lifecycle                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  workflow.start(run_id)                                         │
│       │                                                          │
│       ▼                                                          │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  Sandbox Session: workflow:{run_id}:default              │    │
│  │  Container: running, /workspace bind-mounted             │    │
│  │                                                          │    │
│  │  Step 1: analyzer-agent                                  │    │
│  │    └── sandbox_open() → sbx_abc                         │    │
│  │    └── sandbox_fs_write("analysis.json", ...)           │    │
│  │    └── sandbox_exec(["python", "analyze.py"])           │    │
│  │                                                          │    │
│  │  Step 2: fixer-agent                                     │    │
│  │    └── sandbox_open() → sbx_abc (same session!)         │    │
│  │    └── sandbox_fs_read("analysis.json") ✓               │    │
│  │    └── sandbox_fs_write("fix.py", ...)                  │    │
│  │    └── sandbox_exec(["python", "fix.py"])               │    │
│  │                                                          │    │
│  │  Step 3: tester-agent                                    │    │
│  │    └── sandbox_open() → sbx_abc (same session!)         │    │
│  │    └── sandbox_exec(["pytest"]) → fails                 │    │
│  │                                                          │    │
│  │  Loop back to Step 1...                                  │    │
│  │    └── ALL FILES STILL THERE ✓                          │    │
│  │                                                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│       │                                                          │
│       ▼                                                          │
│  workflow.complete(run_id)                                      │
│       │                                                          │
│       ▼                                                          │
│  Session cleanup (configurable retention)                       │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.3 Backend Abstraction

```go
// SandboxBackend abstracts container runtime (Docker, containerd, etc.)
type SandboxBackend interface {
    // Session lifecycle
    CreateSession(ctx context.Context, opts SessionOptions) (*Session, error)
    GetSession(ctx context.Context, id string) (*Session, error)
    DestroySession(ctx context.Context, id string) error
    
    // Execution
    Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error)
    ExecWait(ctx context.Context, sessionID, execID string, timeout time.Duration) (*ExecResult, error)
    ExecRead(ctx context.Context, sessionID, execID string, sinceSeq int) (*ExecChunks, error)
    
    // Filesystem
    WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error
    ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, error)
    ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error)
    DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error
}

type SessionOptions struct {
    Image           string
    Workdir         string            // default: /workspace
    Env             map[string]string
    Limits          ResourceLimits
    NetworkEnabled  bool
}

type ResourceLimits struct {
    CPUMillicores  int           // e.g., 1000 = 1 CPU
    MemoryMB       int           // e.g., 2048 = 2GB
    TimeoutSeconds int           // Session idle timeout
    WorkspaceMB    int           // Max workspace size
}
```

**Implementations**:
1. `DockerBackend` - uses Docker API (V2 default)
2. `ContainerdBackend` - direct containerd (V3, for tighter K8s integration)

### 2.4 Deployment Support

| Platform | Backend | Notes |
|----------|---------|-------|
| Local dev / Docker Compose | DockerBackend | Socket mount or DinD |
| Kubernetes | DockerBackend (DinD sidecar) | Or ContainerdBackend in V3 |
| ECS on EC2 | DockerBackend | Socket mount |
| ECS Fargate | DockerBackend (sidecar) | No socket access |
| Fly.io | DockerBackend | Socket mount |

---

## 3) Frontmatter Schema

### 3.1 Compute Mode (V1 - unchanged)

```yaml
# Simple: one-shot sandbox
sandbox: python

# Or with options:
sandbox:
  runtime: python
  timeout_seconds: 300
```

### 3.2 Code Mode (V2 - new)

```yaml
sandbox:
  # Mode selection
  mode: code                    # "compute" (default) or "code"
  
  # Runtime config
  runtime: python               # python, node, bash
  image: "python:3.11-slim"     # Override default image
  
  # Session behavior (code mode only)
  session: default              # "default" (inherit workflow), "isolated", or custom name
  
  # Resource limits (per-agent)
  limits:
    cpu_millicores: 1000        # 1 CPU
    memory_mb: 2048             # 2GB RAM
    timeout_seconds: 900        # 15 min session idle timeout
    workspace_mb: 500           # 500MB workspace
  
  # Network (disabled by default, use pre-baked images for deps)
  network:
    enabled: false
```

### 3.3 Go Struct

```go
// pkg/dotprompt/types.go
type SandboxConfig struct {
    // Mode: "compute" (default, V1) or "code" (V2)
    Mode    string `yaml:"mode,omitempty"`
    
    // Runtime: python, node, bash
    Runtime string `yaml:"runtime,omitempty"`
    
    // Image override
    Image   string `yaml:"image,omitempty"`
    
    // Session name (code mode only): "default", "isolated", or custom
    Session string `yaml:"session,omitempty"`
    
    // Resource limits
    Limits  *SandboxLimits `yaml:"limits,omitempty"`
    
    // Network config
    Network *SandboxNetwork `yaml:"network,omitempty"`
    
    // V1 compat fields
    TimeoutSeconds int      `yaml:"timeout_seconds,omitempty"`
    AllowNetwork   bool     `yaml:"allow_network,omitempty"`
    PipPackages    []string `yaml:"pip_packages,omitempty"`
    NpmPackages    []string `yaml:"npm_packages,omitempty"`
}

type SandboxLimits struct {
    CPUMillicores  int `yaml:"cpu_millicores,omitempty"`
    MemoryMB       int `yaml:"memory_mb,omitempty"`
    TimeoutSeconds int `yaml:"timeout_seconds,omitempty"`
    WorkspaceMB    int `yaml:"workspace_mb,omitempty"`
}

type SandboxNetwork struct {
    Enabled bool `yaml:"enabled,omitempty"`
}
```

---

## 4) Tool Schemas (GenKit Native)

### 4.1 Compute Mode Tools (V1 - unchanged)

```go
// sandbox_run - one-shot execution (existing)
type SandboxRunInput struct {
    Runtime        string            `json:"runtime,omitempty"`
    Code           string            `json:"code"`
    Args           []string          `json:"args,omitempty"`
    Env            map[string]string `json:"env,omitempty"`
    Files          map[string]string `json:"files,omitempty"`
    TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
}

type SandboxRunOutput struct {
    OK         bool   `json:"ok"`
    Runtime    string `json:"runtime"`
    ExitCode   int    `json:"exit_code"`
    DurationMs int64  `json:"duration_ms"`
    Stdout     string `json:"stdout"`
    Stderr     string `json:"stderr"`
    Error      string `json:"error,omitempty"`
}
```

### 4.2 Code Mode Tools (V2 - new)

#### `sandbox_open` - Create or resume session

```go
type SandboxOpenInput struct {
    // Usually auto-populated from execution context
    SessionKey string `json:"session_key,omitempty"` // Override session name
}

type SandboxOpenOutput struct {
    SandboxID string `json:"sandbox_id"`
    Image     string `json:"image"`
    Workdir   string `json:"workdir"`
    Created   bool   `json:"created"` // true if new, false if resumed existing
}
```

**Behavior**: Idempotent - returns existing session if one exists for the resolved session key.

#### `sandbox_exec` - Run command in session

```go
type SandboxExecInput struct {
    SandboxID      string            `json:"sandbox_id"`
    Cmd            []string          `json:"cmd"`              // e.g., ["python", "main.py"]
    Cwd            string            `json:"cwd,omitempty"`    // Working dir, default /workspace
    Env            map[string]string `json:"env,omitempty"`
    TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
    Stream         bool              `json:"stream,omitempty"` // If true, returns exec_id for polling
}

type SandboxExecOutput struct {
    ExecID   string `json:"exec_id"`
    ExitCode int    `json:"exit_code,omitempty"` // Only if stream=false
    Stdout   string `json:"stdout,omitempty"`    // Only if stream=false
    Stderr   string `json:"stderr,omitempty"`    // Only if stream=false  
    Status   string `json:"status"`              // "completed" or "running"
}
```

#### `sandbox_exec_read` - Poll for output chunks

```go
type SandboxExecReadInput struct {
    SandboxID string `json:"sandbox_id"`
    ExecID    string `json:"exec_id"`
    SinceSeq  int    `json:"since_seq,omitempty"`
    MaxChunks int    `json:"max_chunks,omitempty"`
}

type SandboxExecReadOutput struct {
    Chunks []OutputChunk `json:"chunks"`
    Done   bool          `json:"done"`
}

type OutputChunk struct {
    Seq    int    `json:"seq"`
    Stream string `json:"stream"` // "stdout" or "stderr"
    Text   string `json:"text"`
}
```

#### `sandbox_exec_wait` - Wait for completion

```go
type SandboxExecWaitInput struct {
    SandboxID      string `json:"sandbox_id"`
    ExecID         string `json:"exec_id"`
    TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type SandboxExecWaitOutput struct {
    Done     bool `json:"done"`
    ExitCode int  `json:"exit_code,omitempty"`
}
```

#### `sandbox_fs_write` - Write file

```go
type SandboxFsWriteInput struct {
    SandboxID string `json:"sandbox_id"`
    Path      string `json:"path"`              // Relative to /workspace
    Contents  string `json:"contents,omitempty"` // Plain text (convenience)
    ContentsB64 string `json:"contents_b64,omitempty"` // Base64 for binary
    Mode      string `json:"mode,omitempty"`    // "0644" default
    Overwrite bool   `json:"overwrite,omitempty"`
}

type SandboxFsWriteOutput struct {
    OK   bool   `json:"ok"`
    Path string `json:"path"` // Absolute path written
}
```

#### `sandbox_fs_read` - Read file

```go
type SandboxFsReadInput struct {
    SandboxID string `json:"sandbox_id"`
    Path      string `json:"path"`
    MaxBytes  int    `json:"max_bytes,omitempty"` // Default 256KB
}

type SandboxFsReadOutput struct {
    Contents    string `json:"contents,omitempty"`     // Plain text if valid UTF-8
    ContentsB64 string `json:"contents_b64,omitempty"` // Base64 if binary
    Truncated   bool   `json:"truncated"`
    SizeBytes   int64  `json:"size_bytes"`
}
```

#### `sandbox_fs_list` - List directory

```go
type SandboxFsListInput struct {
    SandboxID string `json:"sandbox_id"`
    Path      string `json:"path"`              // Default "." (workspace root)
    Recursive bool   `json:"recursive,omitempty"`
}

type SandboxFsListOutput struct {
    Entries []FileEntry `json:"entries"`
}

type FileEntry struct {
    Path      string `json:"path"`
    Type      string `json:"type"` // "file" or "dir"
    Size      int64  `json:"size"`
    Mode      string `json:"mode"`
    MtimeUnix int64  `json:"mtime_unix"`
}
```

#### `sandbox_fs_delete` - Delete file/directory

```go
type SandboxFsDeleteInput struct {
    SandboxID string `json:"sandbox_id"`
    Path      string `json:"path"`
    Recursive bool   `json:"recursive,omitempty"` // Required for directories
}

type SandboxFsDeleteOutput struct {
    OK      bool   `json:"ok"`
    Deleted string `json:"deleted"` // Path that was deleted
}
```

#### `sandbox_close` - Explicitly close session (optional)

```go
type SandboxCloseInput struct {
    SandboxID       string `json:"sandbox_id"`
    DeleteWorkspace bool   `json:"delete_workspace,omitempty"`
}

type SandboxCloseOutput struct {
    OK bool `json:"ok"`
}
```

**Note**: Sessions auto-cleanup when workflow/agent run completes. This tool is optional for explicit cleanup.

---

## 5) Tool Injection Logic

```go
func (e *AgentExecutionEngine) injectSandboxTools(agent *Agent, tools []ai.Tool) []ai.Tool {
    sandboxCfg := parseSandboxConfig(agent)
    if sandboxCfg == nil {
        return tools
    }
    
    mode := sandboxCfg.Mode
    if mode == "" {
        mode = "compute" // Default to V1 behavior
    }
    
    switch mode {
    case "compute":
        // V1: Single sandbox_run tool
        if e.sandboxToolFactory.ShouldAddTool(sandboxCfg) {
            tools = append(tools, e.sandboxToolFactory.CreateComputeTool(sandboxCfg))
        }
        
    case "code":
        // V2: Full code mode tool suite
        if e.sandboxToolFactory.ShouldAddTool(sandboxCfg) {
            tools = append(tools,
                e.sandboxToolFactory.CreateOpenTool(sandboxCfg),
                e.sandboxToolFactory.CreateExecTool(sandboxCfg),
                e.sandboxToolFactory.CreateExecReadTool(sandboxCfg),
                e.sandboxToolFactory.CreateExecWaitTool(sandboxCfg),
                e.sandboxToolFactory.CreateFsWriteTool(sandboxCfg),
                e.sandboxToolFactory.CreateFsReadTool(sandboxCfg),
                e.sandboxToolFactory.CreateFsListTool(sandboxCfg),
                e.sandboxToolFactory.CreateFsDeleteTool(sandboxCfg),
                e.sandboxToolFactory.CreateCloseTool(sandboxCfg),
            )
        }
    }
    
    return tools
}
```

---

## 6) Session Manager

```go
// internal/services/sandbox_session_manager.go

type SessionManager struct {
    backend  SandboxBackend
    sessions sync.Map // map[string]*Session (sessionKey -> Session)
    mu       sync.Mutex
}

func (m *SessionManager) GetOrCreateSession(ctx context.Context, key SessionKey, opts SessionOptions) (*Session, error) {
    keyStr := key.String() // "workflow:run_123:default"
    
    // Check if session exists
    if s, ok := m.sessions.Load(keyStr); ok {
        return s.(*Session), nil
    }
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Double-check after acquiring lock
    if s, ok := m.sessions.Load(keyStr); ok {
        return s.(*Session), nil
    }
    
    // Create new session
    session, err := m.backend.CreateSession(ctx, opts)
    if err != nil {
        return nil, fmt.Errorf("failed to create sandbox session: %w", err)
    }
    
    session.Key = key
    m.sessions.Store(keyStr, session)
    
    return session, nil
}

func (m *SessionManager) CleanupForWorkflow(ctx context.Context, workflowRunID string) error {
    prefix := fmt.Sprintf("workflow:%s:", workflowRunID)
    
    var toDelete []string
    m.sessions.Range(func(k, v interface{}) bool {
        if strings.HasPrefix(k.(string), prefix) {
            toDelete = append(toDelete, k.(string))
        }
        return true
    })
    
    for _, key := range toDelete {
        if s, ok := m.sessions.LoadAndDelete(key); ok {
            session := s.(*Session)
            if err := m.backend.DestroySession(ctx, session.ID); err != nil {
                log.Printf("Warning: failed to cleanup session %s: %v", key, err)
            }
        }
    }
    
    return nil
}
```

---

## 7) Docker Backend Implementation

```go
// internal/services/sandbox_docker_backend.go

type DockerBackend struct {
    client       *docker.Client
    workspaceDir string // Base directory for workspace bind mounts
}

func (b *DockerBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    // Generate unique session ID
    sessionID := fmt.Sprintf("sbx_%s", generateID())
    
    // Create workspace directory on host
    workspacePath := filepath.Join(b.workspaceDir, sessionID)
    if err := os.MkdirAll(workspacePath, 0755); err != nil {
        return nil, fmt.Errorf("failed to create workspace: %w", err)
    }
    
    // Create container config
    config := &container.Config{
        Image:      opts.Image,
        WorkingDir: opts.Workdir,
        Env:        mapToEnvSlice(opts.Env),
        Tty:        false,
        OpenStdin:  true,
        StdinOnce:  false,
    }
    
    // Host config with bind mount and resource limits
    hostConfig := &container.HostConfig{
        Binds: []string{
            fmt.Sprintf("%s:%s", workspacePath, opts.Workdir),
        },
        Resources: container.Resources{
            NanoCPUs: int64(opts.Limits.CPUMillicores) * 1e6, // millicores to nanocores
            Memory:   int64(opts.Limits.MemoryMB) * 1024 * 1024,
        },
        NetworkMode: "none", // Default: no network
        AutoRemove:  false,  // We manage lifecycle
    }
    
    if opts.NetworkEnabled {
        hostConfig.NetworkMode = "bridge"
    }
    
    // Create container
    resp, err := b.client.ContainerCreate(ctx, config, hostConfig, nil, nil, sessionID)
    if err != nil {
        return nil, fmt.Errorf("failed to create container: %w", err)
    }
    
    // Start container
    if err := b.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
        return nil, fmt.Errorf("failed to start container: %w", err)
    }
    
    return &Session{
        ID:            sessionID,
        ContainerID:   resp.ID,
        Image:         opts.Image,
        Workdir:       opts.Workdir,
        WorkspacePath: workspacePath,
        CreatedAt:     time.Now(),
    }, nil
}

func (b *DockerBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error) {
    session, err := b.getSession(sessionID)
    if err != nil {
        return nil, err
    }
    
    execConfig := container.ExecOptions{
        Cmd:          req.Cmd,
        WorkingDir:   req.Cwd,
        Env:          mapToEnvSlice(req.Env),
        AttachStdout: true,
        AttachStderr: true,
    }
    
    execResp, err := b.client.ContainerExecCreate(ctx, session.ContainerID, execConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create exec: %w", err)
    }
    
    // Start exec and capture output
    attachResp, err := b.client.ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to attach exec: %w", err)
    }
    
    execHandle := &ExecHandle{
        ID:         execResp.ID,
        SessionID:  sessionID,
        AttachResp: attachResp,
        Output:     &OutputBuffer{},
        Done:       make(chan struct{}),
    }
    
    // Start background goroutine to collect output
    go execHandle.collectOutput(ctx, req.TimeoutSeconds)
    
    return execHandle, nil
}

// ... other methods (WriteFile, ReadFile, etc.) use docker cp or exec
```

---

## 8) Implementation Plan

### Phase 1: Core Infrastructure (1-2 weeks)

- [ ] `SandboxBackend` interface definition
- [ ] `DockerBackend` implementation (CreateSession, DestroySession, Exec, ExecWait)
- [ ] `SessionManager` with workflow-scoped resolution
- [ ] Session store (in-memory, keyed by SessionKey)
- [ ] Unit tests for backend and session manager

**Files**:
- `internal/services/sandbox_backend.go` - Interface
- `internal/services/sandbox_docker_backend.go` - Docker implementation
- `internal/services/sandbox_session_manager.go` - Session lifecycle
- `internal/services/sandbox_types.go` - Shared types

### Phase 2: Code Mode Tools (1 week)

- [ ] `sandbox_open` tool
- [ ] `sandbox_exec` tool (non-streaming first)
- [ ] `sandbox_exec_read` + `sandbox_exec_wait` for streaming
- [ ] `sandbox_fs_write`, `sandbox_fs_read`, `sandbox_fs_list`, `sandbox_fs_delete`
- [ ] `sandbox_close` tool
- [ ] Tool injection based on `mode: code` frontmatter
- [ ] Integration tests

**Files**:
- `internal/services/sandbox_code_tools.go` - Code mode tool factories
- `internal/services/sandbox_tool.go` - Update to support both modes

### Phase 3: Workflow Integration (1 week)

- [ ] Pass `WorkflowRunID` through execution context to tools
- [ ] Session resolution in tool handlers
- [ ] Cleanup hook on workflow completion
- [ ] E2E test: multi-step workflow with shared sandbox

**Files**:
- `internal/workflow/executor.go` - Add sandbox context
- `internal/services/sandbox_code_tools.go` - Use context for session resolution

### Phase 4: Resource Limits + Polish (3-5 days)

- [ ] CPU/memory limits via Docker API
- [ ] Workspace size limits (quota or periodic check)
- [ ] Idle timeout enforcement (background cleanup)
- [ ] Frontmatter parsing for `limits` block
- [ ] Observability: spans for sandbox operations
- [ ] Documentation

---

## 9) Security Considerations

### 9.1 Isolation

- Each session runs in a separate container
- Default: no network access (`NetworkMode: "none"`)
- Workspace is bind-mounted, isolated per session
- No access to host filesystem outside workspace

### 9.2 Resource Limits

- CPU/memory enforced via cgroups (Docker)
- Execution timeout per command
- Workspace size limit (configurable)
- Output truncation to prevent memory exhaustion

### 9.3 Image Policy

- Only allowed images can be used (allowlist)
- Pre-baked images for dependencies (no runtime `pip install`)
- Future: signed image verification

### 9.4 Cleanup

- Sessions auto-cleanup on workflow/agent completion
- Idle timeout for orphaned sessions
- Workspace directories cleaned up with session

---

## 10) Success Metrics

### Functional

- Agents can write code, run it, see errors, fix, and re-run across multiple tool calls
- Files persist within workflow run, cleaned up after
- Multi-agent workflows share sandbox state correctly

### Operational

- 100% of sandbox sessions are cleaned up within 5 minutes of workflow completion
- Resource limits enforced (no container exceeds configured memory)
- Tool spans appear in Jaeger traces

### Adoption

- Agents using code mode show higher task success rates for complex coding tasks
- Reduced token usage vs. re-injecting all files on each call

---

## 11) Future Work (V3+)

1. **Snapshots**: Checkpoint workspace for replay/restore
2. **SSE/WebSocket streaming**: Real-time output to UI
3. **Network with egress rules**: Controlled outbound access
4. **ContainerdBackend**: Direct containerd for K8s environments
5. **Multi-session workflows**: Named sessions for parallel environments
6. **Image building**: Build custom images from workflow steps

---

## Appendix A: Example Agent Using Code Mode

```yaml
---
model: openai/gpt-4o
metadata:
  name: "Python Developer"
  description: "Writes and debugs Python code iteratively"
sandbox:
  mode: code
  runtime: python
  image: "python:3.11-slim"
  limits:
    cpu_millicores: 1000
    memory_mb: 2048
    timeout_seconds: 600
    workspace_mb: 500
---

You are a Python developer agent. You have access to a persistent sandbox environment.

Available tools:
- sandbox_open: Get access to your sandbox (call this first)
- sandbox_fs_write: Write files to the workspace
- sandbox_fs_read: Read files from the workspace  
- sandbox_fs_list: List files in the workspace
- sandbox_exec: Run commands in the sandbox

Workflow:
1. Call sandbox_open to get your sandbox_id
2. Write your code using sandbox_fs_write
3. Run it using sandbox_exec with cmd: ["python", "your_script.py"]
4. If there are errors, read the output, fix the code, and try again
5. Files persist between calls - you can build on previous work

{{role "user"}}
{{userInput}}
```

---

## Appendix B: Example Workflow Using Shared Sandbox

```yaml
id: code-review-and-fix
name: "Code Review and Fix"
description: "Reviews code, identifies issues, fixes them, and verifies"

steps:
  - id: analyze
    type: agent
    agent: code-analyzer
    # Uses sandbox session "workflow:{run_id}:default"
    
  - id: fix
    type: agent  
    agent: code-fixer
    # Same sandbox - sees analyzer's output files
    
  - id: test
    type: agent
    agent: test-runner
    # Same sandbox - runs tests on fixed code
    
  - id: check-results
    type: switch
    expression: "steps.test.output.all_passed"
    transitions:
      - condition: "true"
        next: done
      - condition: "false"  
        next: fix  # Loop back - sandbox state preserved!

  - id: done
    type: end
```

---

## Appendix C: AutoGen Implementation Patterns (Reference)

Detailed patterns extracted from AutoGen's `DockerCommandLineCodeExecutor` for Go implementation:

### C.1 Container Lifecycle

```python
# AutoGen: Persistent container with context manager
class DockerCommandLineCodeExecutor:
    async def start(self) -> None:
        # 1. Create workspace directory (temp if not provided)
        self._work_dir = work_dir or tempfile.TemporaryDirectory()
        
        # 2. Create container with bind mount
        self._container = self._docker_client.containers.run(
            image=self._image,
            entrypoint="/bin/sh",
            tty=True,
            stdin_open=True,
            auto_remove=self._auto_remove,
            volumes={str(self.bind_dir): {"bind": "/workspace", "mode": "rw"}},
            working_dir="/workspace",
            detach=True,
        )
        self._running = True
        
    async def stop(self) -> None:
        if self._container:
            self._container.stop()
            self._container.remove()
        self._running = False
```

**Go Translation:**
```go
func (b *DockerBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
    // Create workspace on host
    workspacePath := filepath.Join(b.workspaceDir, sessionID)
    os.MkdirAll(workspacePath, 0755)
    
    // Create container
    resp, _ := b.client.ContainerCreate(ctx, &container.Config{
        Image:      opts.Image,
        WorkingDir: "/workspace",
        Tty:        true,
        OpenStdin:  true,
    }, &container.HostConfig{
        Binds: []string{workspacePath + ":/workspace:rw"},
        AutoRemove: false, // We manage lifecycle
    }, nil, nil, sessionID)
    
    // Start container
    b.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
    
    return &Session{ID: sessionID, ContainerID: resp.ID}, nil
}
```

### C.2 Code Execution Pattern

```python
# AutoGen: File-based execution with timeout command
async def _execute_code_dont_check_setup(self, code_blocks, cancellation_token):
    outputs = []
    exit_code = 0
    
    for code_block in code_blocks:
        lang = code_block.language.lower()
        code = code_block.code
        
        # 1. Write code to file
        filename = f"tmp_code_{hash(code)}.{_LANG_TO_EXTENSION[lang]}"
        code_path = self._work_dir / filename
        code_path.write_text(code)
        
        # 2. Build command with timeout wrapper
        command = ["timeout", str(self._timeout)]
        command.extend(_get_execution_command(lang, filename))
        # e.g., ["timeout", "60", "python", "tmp_code_xxx.py"]
        
        # 3. Execute in container
        result = await asyncio.to_thread(
            self._container.exec_run,
            command,
            workdir="/workspace",
        )
        
        # 4. Collect output
        outputs.append(result.output.decode("utf-8"))
        exit_code = result.exit_code
        
        # 5. Fail-fast on error
        if exit_code != 0:
            break
    
    return CommandLineCodeResult(
        exit_code=exit_code,
        output="".join(outputs),
    )
```

**Go Translation:**
```go
func (b *DockerBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
    session := b.getSession(sessionID)
    
    // Build command with timeout
    cmd := append([]string{"timeout", fmt.Sprint(req.TimeoutSeconds)}, req.Cmd...)
    
    // Create exec
    execConfig := types.ExecConfig{
        Cmd:          cmd,
        WorkingDir:   req.Cwd,
        AttachStdout: true,
        AttachStderr: true,
    }
    execResp, _ := b.client.ContainerExecCreate(ctx, session.ContainerID, execConfig)
    
    // Attach and run
    attachResp, _ := b.client.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{})
    defer attachResp.Close()
    
    // Read output
    output, _ := io.ReadAll(attachResp.Reader)
    
    // Get exit code
    inspect, _ := b.client.ContainerExecInspect(ctx, execResp.ID)
    
    return &ExecResult{
        ExitCode: inspect.ExitCode,
        Output:   string(output),
    }, nil
}
```

### C.3 Cancellation with Process Kill

```python
# AutoGen: Cancel running command via pkill
async def _execute_command(self, command, cancellation_token):
    exec_task = asyncio.create_task(
        asyncio.to_thread(self._container.exec_run, command)
    )
    cancellation_token.link_future(exec_task)
    
    try:
        result = await exec_task
        return result.output.decode(), result.exit_code
    except asyncio.CancelledError:
        # Kill the process in background
        self._container.exec_run(["pkill", "-f", " ".join(command)])
        return "Cancelled", 1
```

**Go Translation:**
```go
func (b *DockerBackend) execWithCancel(ctx context.Context, containerID string, cmd []string) (string, int, error) {
    execID, _ := b.client.ContainerExecCreate(ctx, containerID, types.ExecConfig{Cmd: cmd})
    resp, _ := b.client.ContainerExecAttach(ctx, execID, types.ExecStartCheck{})
    
    outputCh := make(chan []byte, 1)
    go func() {
        out, _ := io.ReadAll(resp.Reader)
        outputCh <- out
    }()
    
    select {
    case output := <-outputCh:
        inspect, _ := b.client.ContainerExecInspect(ctx, execID)
        return string(output), inspect.ExitCode, nil
    case <-ctx.Done():
        // Kill the process
        b.client.ContainerExecCreate(context.Background(), containerID, 
            types.ExecConfig{Cmd: []string{"pkill", "-f", strings.Join(cmd, " ")}})
        return "Cancelled", 1, ctx.Err()
    }
}
```

### C.4 Configuration Defaults

```python
# AutoGen: Pydantic config with sensible defaults
class DockerCommandLineCodeExecutorConfig(BaseModel):
    image: str = "python:3-slim"
    container_name: Optional[str] = None
    timeout: int = 60
    work_dir: Optional[str] = None
    bind_dir: Optional[str] = None
    auto_remove: bool = True
    stop_container: bool = True
    extra_volumes: Dict[str, Dict[str, str]] = {}
    extra_hosts: Dict[str, str] = {}
    init_command: Optional[str] = None
```

### C.5 Key Takeaways

1. **Persistent containers** - Container stays running between executions
2. **File-based execution** - Write code to file, then execute (not stdin piping)
3. **Timeout via `timeout` command** - More reliable than Docker API timeout
4. **Bind mount pattern** - Host directory mounted to `/workspace`
5. **Fail-fast semantics** - Stop on first non-zero exit code
6. **Cancellation via pkill** - Kill process tree on context cancel
7. **Separate work_dir/bind_dir** - Supports running inside containers

---

*Created: 2025-12-26*
*Based on: AutoGen DockerCommandLineCodeExecutor patterns*
*AutoGen Source: https://github.com/microsoft/autogen/blob/main/python/packages/autogen-ext/src/autogen_ext/code_executors/docker/_docker_code_executor.py*
