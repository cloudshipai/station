# PRD: Station + OpenCode Integration

**Status**: Phase 1-7 Complete, E2E Verified ✅  
**Author**: Station Team  
**Date**: 2025-12-30  
**Version**: 0.9

---

## Implementation Status

### ✅ Phase 1: Core Coding Package (COMPLETE)

**Files Created:**
| File | Purpose | Tests |
|------|---------|-------|
| `internal/coding/backend.go` | Backend interface definition | N/A |
| `internal/coding/opencode_backend.go` | OpenCode HTTP API implementation | 8 tests |
| `internal/coding/opencode_client.go` | HTTP client with retries | 8 tests |
| `internal/coding/tool.go` | GenKit tool implementations (coding_open, code, coding_close) | 10 tests |
| `internal/coding/types.go` | Session, Result, TokenUsage structs | 5 tests |
| `internal/coding/config.go` | Config parsing | 2 tests |
| `internal/coding/errors.go` | Error definitions | N/A |

**Test Results:** 45 test functions passing in `internal/coding/` (including 2 E2E tests skipped by default)

### ✅ Phase 2: Engine Wiring (COMPLETE)

**Files Created/Modified:**
| File | Changes |
|------|---------|
| `internal/services/coding_tool_factory.go` | Factory wrapper for creating coding tools |
| `internal/services/coding_tool_factory_test.go` | 4 tests passing |
| `internal/services/agent_execution_engine.go` | Added `codingToolFactory` field, `parseCodingConfigFromAgent()` method, tool injection logic |
| `pkg/dotprompt/types.go` | Added `CodingConfig` struct |

**Test Agent Created:**
- `~/.config/station/environments/default/agents/code-assistant.prompt`

**Verification (2025-12-29):**
- ✅ Coding factory initializes: `Coding tool factory initialized with opencode backend`
- ✅ Config parsed from agent dotprompt: `codingConfig=&{Enabled:true Backend:opencode WorkspacePath:}`
- ✅ Tools injected: `Coding tools enabled for agent code-assistant (3 tools: coding_open, code, coding_close)`
- ✅ Tools registered with GenKit: `Registered 3 tools... for agent code-assistant`

**Configuration Required:**

1. Station config (`~/.config/station/config.yaml`):
```yaml
coding:
  backend: opencode
  opencode:
    url: http://localhost:4096
  max_attempts: 3
  task_timeout_min: 10
```

2. Agent dotprompt (e.g., `code-assistant.prompt`):
```yaml
---
model: openai/gpt-4o-mini
coding:
  enabled: true
  backend: opencode
---
You are a coding assistant...
```

3. OpenCode must be running:
```bash
opencode serve --port 4096
```

### ✅ E2E Verification (COMPLETE)

**Date**: 2025-12-29

**Bugs Fixed:**

1. **JSON Parsing Bug** (`opencode_client.go` line 127-141):
   - **Problem**: OpenCode API returns `time` as nested object `{"created": ..., "completed": ...}`, but struct expected `int64`
   - **Fix**: Changed `messageInfo.Time` from `int64` to nested struct

2. **Workspace Path Ignored** (`opencode_backend.go`):
   - **Problem**: OpenCode wrote files to its launch directory instead of session workspace
   - **Fix**: Modified `buildPrompt()` to include workspace path instruction in task prompt

3. **Missing Directory Query Parameter** (`opencode_client.go` line 163) - Fixed 2025-12-30:
   - **Problem**: OpenCode API intermittently returned HTTP 200 with empty body (Content-Length: 0)
   - **Root Cause**: The `/session/{id}/message` endpoint requires `?directory=` query parameter to route requests to the correct project instance
   - **Fix**: Updated `SendMessage()` to include `?directory=` in the URL
   - **Files Changed**:
     - `internal/coding/opencode_client.go` - Added `directory` parameter to `SendMessage()`
     - `internal/coding/opencode_backend.go` - Pass `session.WorkspacePath` to client
     - `internal/coding/opencode_client_test.go` - Updated test to verify directory param

**Test Command:**
```bash
./stn agent run code-assistant "Use coding_open to start a session in /tmp/test-workspace, then use code tool to create hello.py with print Hello World"
```

**Results:**
- ✅ `coding_open` - Created session with correct directory
- ✅ `code` - Executed task, file written to `/tmp/test-workspace/hello.py`
- ✅ `coding_close` - Session closed properly
- ✅ File content verified: `print('Hello World')`

### ✅ Phase 3: Git Operations (COMPLETE)

**Date**: 2025-12-29

**New Tools Added:**
| Tool | Description |
|------|-------------|
| `coding_commit` | Commits staged/all changes with a message. Returns commit hash, files changed, insertions, deletions |
| `coding_push` | Pushes commits to remote. Supports custom remote, branch, and `-u` flag |

**Files Modified:**
| File | Changes |
|------|---------|
| `internal/coding/tool.go` | Added `CreateCommitTool()`, `CreatePushTool()`, `parseGitCommitStats()` |
| `internal/coding/types.go` | Added `GitCommitResult`, `GitPushResult` structs |
| `internal/coding/tool_test.go` | Added 8 new tests including integration test |

**Test Results:** 45 test functions passing in `internal/coding/` (after Phase 6)

**Design Decisions:**
- Git operations execute directly on workspace (not through OpenCode)
- Fast, no AI cost for deterministic commands
- `add_all` defaults to `true` for convenience

### ✅ E2E Integration Tests (COMPLETE)

**Date**: 2025-12-30

**Test File**: `internal/coding/e2e_test.go`

**Tests Added:**
| Test | Description | Duration |
|------|-------------|----------|
| `TestE2E_OpenCodeIntegration` | Full flow: health → session → coding task → verify file → close | ~68s |
| `TestE2E_GitCommitFlow` | Create file via OpenCode → git commit with stats | ~17s |

**Run Command:**
```bash
OPENCODE_E2E=true go test ./internal/coding/... -run TestE2E -v
```

**Verified Behaviors:**
- ✅ Health check returns healthy
- ✅ Session created with correct directory binding
- ✅ Coding tasks execute and create files in workspace
- ✅ File content matches expected output
- ✅ Git commit captures files changed, insertions, deletions
- ✅ Session closes cleanly

**Sample Output:**
```
=== RUN   TestE2E_OpenCodeIntegration/execute_coding_task
    e2e_test.go:73: Task completed successfully
    e2e_test.go:74: Summary: Created `hello.py` with the print statement.
    e2e_test.go:77: Model: claude-opus-4-5, Provider: anthropic
    e2e_test.go:78: Tokens: input=7, output=40
=== RUN   TestE2E_OpenCodeIntegration/verify_file_created
    e2e_test.go:90: File content:
        print('Hello from E2E test')
--- PASS: TestE2E_OpenCodeIntegration (67.95s)
```

### ✅ Phase 4: Observability (COMPLETE)

**Date**: 2025-12-30

**Features Implemented:**

| Feature | Description |
|---------|-------------|
| Tool Call Parsing | Parse `tool-invocation` and `tool-result` parts from OpenCode response |
| Reasoning Extraction | Extract `reasoning` text from extended thinking models |
| OTEL Tracing | `station.coding` tracer with spans for tasks and tool calls |
| Trace Data | ToolCalls and Reasoning populated in Result.Trace |

**Files Modified:**
| File | Changes |
|------|---------|
| `internal/coding/opencode_client.go` | Extended `messagePart` struct, updated `parseMessageResponse` to extract tool calls and reasoning |
| `internal/coding/opencode_backend.go` | Added OTEL tracer, spans for `opencode.task` with child spans for each `opencode.tool.*` |
| `internal/coding/opencode_client_test.go` | Added 4 test cases for tool call parsing (single, multiple, reasoning, none) |

**OTEL Span Structure:**
```
station.agent.execute
  └── opencode.task (session_id, workspace, model, provider, cost, tokens, tool_calls count)
        ├── opencode.tool.bash
        ├── opencode.tool.read
        ├── opencode.tool.write
        └── ...
```

**Span Attributes:**
- `opencode.session_id` - Backend session ID
- `opencode.workspace` - Workspace path
- `opencode.model` - Model used (e.g., `claude-opus-4-5`)
- `opencode.provider` - Provider (e.g., `anthropic`)
- `opencode.cost` - Execution cost in dollars
- `opencode.tokens.input` - Input tokens
- `opencode.tokens.output` - Output tokens
- `opencode.tool_calls` - Number of tool invocations

**Test Results:** 45 test functions passing (43 unit + 2 E2E skipped by default)

**E2E Verification (2025-12-30):**

Confirmed OpenCode writes code and we capture all returned metadata:

```
=== RUN   TestE2E_OpenCodeIntegration/execute_coding_task
    e2e_test.go:77: Model: claude-opus-4-5, Provider: anthropic
    e2e_test.go:78: Tokens: input=7, output=34
    e2e_test.go:79: Duration: 1m4.368616545s
    e2e_test.go:80: Tool calls: 0
    e2e_test.go:85: Reasoning steps: 1
=== RUN   TestE2E_OpenCodeIntegration/verify_file_created
    e2e_test.go:97: File content:
        print('Hello from E2E test')
--- PASS: TestE2E_OpenCodeIntegration (64.53s)
```

**API Discovery:** OpenCode's `/session/{id}/message` endpoint does NOT expose `tool-invocation`/`tool-result` parts in the response. It only returns:
- `step-start` - Task started
- `reasoning` - Extended thinking text (captured in `Trace.Reasoning`)
- `text` - Final summary (captured in `Result.Summary`)
- `step-finish` - Task completed with tokens/cost

Internal tool calls (write, bash, read, etc.) are executed but not exposed in the API response. Our parsing is correct and ready to capture them if OpenCode adds this in the future.

**Data Captured from OpenCode:**

| Field | API Source | Station Field |
|-------|------------|---------------|
| Model | `info.modelID` | `Trace.Model` |
| Provider | `info.providerID` | `Trace.Provider` |
| Cost | `info.cost` | `Trace.Cost` |
| Input tokens | `info.tokens.input` | `Trace.Tokens.Input` |
| Output tokens | `info.tokens.output` | `Trace.Tokens.Output` |
| Reasoning tokens | `info.tokens.reasoning` | `Trace.Tokens.Reasoning` |
| Cache read | `info.tokens.cache.read` | `Trace.Tokens.CacheRead` |
| Cache write | `info.tokens.cache.write` | `Trace.Tokens.CacheWrite` |
| Finish reason | `info.finish` | `Trace.FinishReason` |
| Reasoning text | `parts[type=reasoning].text` | `Trace.Reasoning[]` |
| Summary | `parts[type=text].text` | `Result.Summary` |

### ✅ Phase 5: Workspace Management (COMPLETE)

**Date**: 2025-12-30

**Files Created:**
| File | Purpose |
|------|---------|
| `internal/coding/workspace.go` | Workspace lifecycle manager |
| `internal/coding/workspace_test.go` | 12 test cases covering all functionality |

**Features Implemented:**

| Feature | Description |
|---------|-------------|
| `WorkspaceManager` | Manages workspace lifecycle with thread-safe operations |
| `Workspace` | Tracks workspace state, scope, cleanup policy, git status |
| `CleanupPolicy` | `on_session_end`, `on_success`, `manual` policies |
| `SessionScope` | `agent` (single run) vs `workflow` (persists across steps) |
| `InitGit` | Initializes git repo with Station user config |
| `CollectChanges` | Detects file changes via git status or filesystem walk |
| `CloneRepo` | Clones remote repos with optional branch |
| `GetCommitsSince` | Retrieves commit hashes since a reference |

**Workspace Methods:**

```go
// Create new workspace
ws, err := manager.Create(ctx, ScopeAgent, "session-123")

// Get workspace by ID or scope
ws, err := manager.Get(id)
ws, err := manager.GetByScope(ScopeWorkflow, "workflow-456")

// Git operations
manager.InitGit(ctx, ws)
manager.CloneRepo(ctx, ws, "https://github.com/org/repo.git", "main")

// Collect changes
changes, err := manager.CollectChanges(ctx, ws)  // Returns []FileChange

// Cleanup
manager.CleanupByPolicy(ctx, ws, success)  // Respects policy
manager.CleanupAll(ctx)                     // Force cleanup all
```

**Test Results:** 12 new tests added
- `TestNewWorkspaceManager_Defaults`
- `TestNewWorkspaceManager_WithOptions`
- `TestWorkspaceManager_Create` (2 subtests)
- `TestWorkspaceManager_Get` (2 subtests)
- `TestWorkspaceManager_GetByScope` (3 subtests)
- `TestWorkspaceManager_InitGit` (2 subtests)
- `TestWorkspaceManager_CollectChanges` (3 subtests)
- `TestWorkspaceManager_Cleanup`
- `TestWorkspaceManager_CleanupByPolicy` (3 subtests)
- `TestWorkspaceManager_CleanupAll`
- `TestWorkspaceManager_ListWorkspaces`
- `TestWorkspaceManager_GetCommitsSince` (3 subtests)

### ✅ Phase 6: Tool Integration (COMPLETE)

**Date**: 2025-12-30

**Goal**: Integrate WorkspaceManager into coding tools for automatic workspace lifecycle management.

**Files Modified:**
| File | Changes |
|------|---------|
| `internal/coding/tool.go` | Added WorkspaceManager to ToolFactory, updated coding_open and coding_close tools |
| `internal/coding/tool_test.go` | Added 4 new test functions for workspace integration |

**New Tool Features:**

1. **`coding_open`** with managed workspace:
   - Optional `workspace_path` - if omitted, creates managed workspace automatically
   - New `scope` parameter: "agent" (default) or "workflow"
   - New `scope_id` parameter for workflow identification
   - Returns `workspace_id` and `managed=true` for managed workspaces
   - Auto-initializes git in managed workspaces

2. **`coding_close`** with cleanup:
   - New `workspace_id` parameter to identify managed workspace
   - New `success` parameter for cleanup policy decisions
   - Collects `files_changed` before cleanup
   - Returns `cleaned_up` status based on policy

**ToolFactory Changes:**
```go
// New functional option
factory := NewToolFactory(backend, WithWorkspaceManager(wm))

// Creates managed workspace when path omitted
input := map[string]any{
    "scope": "workflow",
    "scope_id": "workflow-123",
}
```

**Test Results:** 4 new test functions added
- `TestToolFactory_WithWorkspaceManager`
- `TestToolFactory_CreateOpenTool_ManagedWorkspace` (3 subtests)
- `TestToolFactory_CreateOpenTool_NoWorkspaceManager` (1 subtest)
- `TestToolFactory_CreateCloseTool_WithWorkspace` (2 subtests)

**Total Tests:** 45 test functions passing in `internal/coding/`

### ✅ Phase 7: Engine Integration & Git Credentials (COMPLETE)

**Date**: 2025-12-30

**Goal**: Wire WorkspaceManager into the agent execution engine and add git credential management.

**Files Created:**
| File | Purpose |
|------|---------|
| `internal/coding/git_credentials.go` | GitCredentials struct, InjectCredentials, RedactString, RedactURL, RedactError |
| `internal/coding/git_credentials_test.go` | 25+ test cases for credential injection and redaction |

**Files Modified:**
| File | Changes |
|------|---------|
| `internal/config/config.go` | Added `CodingGitConfig` struct with TokenEnvVar, Token, UserName, UserEmail |
| `internal/coding/workspace.go` | Added GitCredentials to WorkspaceManager, updated CloneRepo to inject credentials |
| `internal/coding/tool.go` | Updated CreatePushTool to use GIT_ASKPASS for authenticated push |
| `internal/services/coding_tool_factory.go` | Wired WorkspaceManager with GitCredentials from config |

**Features Implemented:**

| Feature | Description |
|---------|-------------|
| `GitCredentials` | Manages git authentication with token injection |
| `InjectCredentials` | Rewrites HTTPS URLs to include token (`https://x-access-token:TOKEN@github.com/...`) |
| `RedactString` | Removes sensitive data from logs (GitHub tokens, Bearer tokens, API keys) |
| `RedactURL` | URL-aware redaction preserving structure |
| `RedactError` | Wraps errors with redacted messages while preserving type |
| `createGitAskpassScript` | Creates temporary script for git push authentication |

**Credential Flow by Mode:**

| Mode | Behavior |
|------|----------|
| **stdio/CLI** | Uses host git credentials by default (no config needed) |
| **container/`stn serve`** | Requires explicit `git.token_env` or `git.token` in config |

**Configuration Example:**

```yaml
# config.yaml
coding:
  backend: opencode
  opencode:
    url: http://localhost:4096
  workspace_base_path: /tmp/station-coding
  cleanup_policy: on_session_end
  git:
    token_env: GITHUB_TOKEN       # Read token from this env var (recommended)
    # token: ${GITHUB_TOKEN}      # Or specify directly with env expansion
    user_name: "Station Bot"      # Git commit author name
    user_email: "bot@example.com" # Git commit author email
```

**Test Results:** 61+ test functions passing in `internal/coding/`

**Completed Production Hardening:**

| Task | Status | Implementation |
|------|--------|----------------|
| Configurable Timeouts | ✅ Complete | `CloneTimeoutSec`, `PushTimeoutSec` in CodingConfig with context-based timeouts |
| Retry Logic | ✅ Complete | `doWithRetry` with exponential backoff (initial, max, multiplier) |
| Health Monitoring | ✅ Complete | `CheckHealth()` method on CodingToolFactory with 10s timeout |

---

## Executive Summary

Integrate OpenCode (SST's AI coding assistant) as a sandbox backend for Station CLI, enabling Station agents to delegate complex coding tasks to a full-featured AI coding environment with file system access, git operations, and code execution capabilities.

---

## Problem Statement

Station agents currently have two sandbox modes:
1. **Compute Mode** (Dagger): Single-shot script execution, no persistence
2. **Code Mode** (Docker): Persistent container with bash/file tools

Neither provides a **full AI coding assistant** capable of:
- Intelligent code generation and refactoring
- Git operations (clone, commit, push)
- Multi-file project understanding
- Tool use with reasoning

**Gap**: When a Station agent needs to "write code to solve X", it lacks the sophisticated coding capabilities that OpenCode provides.

---

## Proposed Solution

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        Station CLI                               │
│  ┌─────────────┐    ┌──────────────────┐    ┌───────────────┐  │
│  │ Agent       │───▶│ OpenCodeBackend  │───▶│ OpenCode      │  │
│  │ (dotprompt) │    │ (SandboxBackend) │    │ Container     │  │
│  └─────────────┘    └──────────────────┘    │ (HTTP API)    │  │
│                              │               └───────┬───────┘  │
│                              │                       │          │
│                     ┌────────▼────────┐              │          │
│                     │ Workspace       │◀─────────────┘          │
│                     │ (shared volume) │                         │
│                     └─────────────────┘                         │
└─────────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | Responsibility |
|-----------|----------------|
| `OpenCodeBackend` | Implements `SandboxBackend` interface, translates operations to OpenCode API |
| OpenCode Container | Runs `opencode serve`, provides AI coding capabilities |
| Shared Workspace | Volume mount for file exchange between Station and OpenCode |
| Credential Broker | Securely passes git/API credentials to OpenCode |

---

## Detailed Design

### 1. Task Delegation Model

Station agents delegate **high-level coding tasks** to OpenCode, not raw bash commands.

```yaml
# Agent dotprompt with OpenCode sandbox
---
model: openai/gpt-4o
sandbox:
  mode: opencode           # NEW: OpenCode mode
  server_url: http://localhost:4096
  workspace: /workspace/project-x
  git:
    clone_url: https://github.com/org/repo.git
    branch: feature/my-feature
  credentials:
    - GITHUB_TOKEN         # Env var names to pass
    - OPENAI_API_KEY
---
You are a DevOps agent. When asked to write code, delegate to your coding sandbox.

{{userInput}}
```

**Task Example**:
```
Station Agent Task: "Add a health check endpoint to the API"

OpenCode receives: "Add a health check endpoint to the Flask API at /health 
that returns {"status": "ok", "timestamp": <current_time>}. 
Follow existing code patterns in this repo."
```

### 2. Git Operations & Private Repos

#### Credential Flow (Secure)

```
┌──────────┐     ┌─────────────┐     ┌──────────────┐     ┌────────┐
│ Station  │────▶│ Credential  │────▶│ OpenCode     │────▶│ GitHub │
│ Config   │     │ Broker      │     │ Container    │     │        │
└──────────┘     └─────────────┘     └──────────────┘     └────────┘
     │                  │                    │
     │ GITHUB_TOKEN     │ GIT_ASKPASS        │ git clone
     │ (env var)        │ (helper script)    │ (authenticated)
```

#### Option A: Environment Variable Injection (Recommended)

```yaml
# station config.yaml
sandbox:
  opencode:
    enabled: true
    server_url: http://localhost:4096
    credentials:
      # These env vars are passed to OpenCode container
      - name: GITHUB_TOKEN
        source: env           # Read from Station's environment
      - name: GIT_SSH_KEY
        source: file          # Read from file path
        path: ~/.ssh/id_ed25519
```

OpenCode container receives:
```bash
GITHUB_TOKEN=ghp_xxxx
GIT_ASKPASS=/usr/local/bin/git-credential-helper  # Uses GITHUB_TOKEN
```

#### Option B: Git Credential Helper

Station generates a temporary credential helper script:
```bash
#!/bin/bash
# /tmp/git-credential-helper-xxx
echo "protocol=https"
echo "host=github.com"
echo "username=x-access-token"
echo "password=${GITHUB_TOKEN}"
```

Mount into container and set `GIT_ASKPASS`.

#### Option C: SSH Key Mount (For SSH URLs)

```yaml
sandbox:
  opencode:
    ssh_keys:
      - host: github.com
        key_path: ~/.ssh/github_deploy_key
        key_env: GITHUB_SSH_KEY  # Or from env var
```

Container receives mounted key at `/root/.ssh/id_ed25519`.

#### Security Requirements

| Requirement | Implementation |
|-------------|----------------|
| No credentials in logs | Redact tokens from all logged commands |
| No credentials in task prompts | Pass via env vars, not in text |
| Short-lived tokens preferred | Support GitHub App installation tokens |
| Credential isolation | Each session gets unique credential set |

### 3. Observability

#### 3.1 Execution Traces

OpenCode returns structured execution data:

```json
{
  "info": {
    "id": "msg_xxx",
    "modelID": "claude-opus-4-5",
    "cost": 0.0234,
    "tokens": {"input": 5000, "output": 1200}
  },
  "parts": [
    {"type": "step-start"},
    {"type": "tool-invocation", "tool": "bash", "input": "git clone ..."},
    {"type": "tool-result", "output": "Cloning into..."},
    {"type": "tool-invocation", "tool": "write", "input": {"path": "api/health.py"}},
    {"type": "text", "text": "I've added the health endpoint..."},
    {"type": "step-finish", "reason": "stop"}
  ]
}
```

#### 3.2 Station Integration Points

```go
type OpenCodeExecutionResult struct {
    // Core result
    Success     bool   `json:"success"`
    Response    string `json:"response"`
    
    // Observability
    Trace       *OpenCodeTrace `json:"trace"`
    
    // Artifacts
    FilesChanged []FileChange  `json:"files_changed"`
    GitCommits   []GitCommit   `json:"git_commits,omitempty"`
}

type OpenCodeTrace struct {
    SessionID   string          `json:"session_id"`
    MessageID   string          `json:"message_id"`
    Model       string          `json:"model"`
    Provider    string          `json:"provider"`
    Cost        float64         `json:"cost"`
    Tokens      TokenUsage      `json:"tokens"`
    Duration    time.Duration   `json:"duration"`
    ToolCalls   []ToolCall      `json:"tool_calls"`
    Reasoning   []string        `json:"reasoning,omitempty"`
}

type ToolCall struct {
    Tool      string        `json:"tool"`      // "bash", "write", "read", "glob", etc.
    Input     interface{}   `json:"input"`
    Output    string        `json:"output"`
    Duration  time.Duration `json:"duration"`
    ExitCode  int           `json:"exit_code,omitempty"`
}
```

#### 3.3 Telemetry Integration

OpenCode executions emit OTEL spans:

```
station.agent.execute
  └── opencode.task
        ├── opencode.tool.bash (git clone)
        ├── opencode.tool.read (understand existing code)
        ├── opencode.tool.write (create health.py)
        └── opencode.tool.bash (run tests)
```

#### 3.4 Artifact Collection

After task completion, Station collects:

| Artifact | Source | Storage |
|----------|--------|---------|
| Changed files | `git diff` in workspace | Attached to run record |
| Git commits | `git log` | Run metadata |
| Execution trace | OpenCode API response | Run events table |
| Screenshots (if UI) | OpenCode browser tool | S3/blob storage |

### 4. Workspace Management

#### 4.1 Workspace Lifecycle

```
1. Station creates workspace directory
   /tmp/station-opencode/{session_id}/

2. Station clones repo (if configured)
   git clone $REPO_URL /tmp/station-opencode/{session_id}/repo

3. OpenCode receives task with workspace path
   POST /session/{id}/message
   {"parts": [{"type": "text", "text": "..."}]}
   
4. OpenCode operates on workspace
   - Reads/writes files
   - Runs commands
   - Makes git commits

5. Station collects results
   - Changed files
   - Git commits
   - Command outputs

6. Cleanup (based on session scope)
   - "agent": Clean after each agent run
   - "workflow": Persist across workflow steps
```

#### 4.2 File Sync Strategy

**Option A: Shared Volume (Recommended for local)**
```yaml
# docker-compose.yml
services:
  opencode:
    volumes:
      - station-workspaces:/workspaces

# Station mounts same volume
sandbox:
  opencode:
    workspace_base: /workspaces
```

**Option B: API-based Sync (For remote OpenCode)**
```go
// Before task: sync files to OpenCode
func (b *OpenCodeBackend) SyncToRemote(ctx context.Context, sessionID string, files []File) error

// After task: sync files from OpenCode  
func (b *OpenCodeBackend) SyncFromRemote(ctx context.Context, sessionID string) ([]FileChange, error)
```

### 5. Configuration

#### 5.1 Station Config

```yaml
# config.yaml
sandbox:
  opencode:
    enabled: true
    server_url: http://localhost:4096
    
    # Default model for OpenCode tasks
    model:
      provider: anthropic
      model: claude-sonnet-4-20250514
    
    # Workspace settings
    workspace_base: /tmp/station-opencode
    cleanup_policy: on_session_end  # or "manual", "on_success"
    
    # Timeouts
    task_timeout: 10m
    clone_timeout: 5m
    
    # Credentials to pass to OpenCode
    credentials:
      - name: GITHUB_TOKEN
        source: env
      - name: ANTHROPIC_API_KEY
        source: env
      - name: OPENAI_API_KEY
        source: env
    
    # Git defaults
    git:
      user_name: "Station Bot"
      user_email: "station@example.com"
      sign_commits: false
```

#### 5.2 Agent Dotprompt Config

```yaml
---
model: openai/gpt-4o
sandbox:
  mode: opencode
  
  # Override server URL (optional)
  server_url: http://opencode.internal:4096
  
  # Git repo to work on
  git:
    clone_url: git@github.com:myorg/myrepo.git
    branch: main
    shallow: true        # Shallow clone for speed
    sparse_checkout:     # Only checkout specific paths
      - src/
      - tests/
  
  # Additional credentials for this agent
  credentials:
    - NPM_TOKEN
    - AWS_ACCESS_KEY_ID
    - AWS_SECRET_ACCESS_KEY
---
```

### 6. API Design

#### 6.1 OpenCode API Wrapper

```go
type OpenCodeClient struct {
    baseURL    string
    httpClient *http.Client
}

// SendTask sends a coding task and waits for completion
func (c *OpenCodeClient) SendTask(ctx context.Context, req TaskRequest) (*TaskResponse, error)

// SendTaskStream sends a task and streams the response
func (c *OpenCodeClient) SendTaskStream(ctx context.Context, req TaskRequest) (<-chan TaskEvent, error)

// GetSession retrieves session info
func (c *OpenCodeClient) GetSession(ctx context.Context, sessionID string) (*Session, error)

// ListSessions lists all sessions
func (c *OpenCodeClient) ListSessions(ctx context.Context) ([]Session, error)

type TaskRequest struct {
    SessionID  string            `json:"session_id"`
    Task       string            `json:"task"`
    Model      *ModelConfig      `json:"model,omitempty"`
    System     string            `json:"system,omitempty"`
    Tools      map[string]bool   `json:"tools,omitempty"`  // Enable/disable tools
}

type TaskResponse struct {
    Info   MessageInfo `json:"info"`
    Parts  []Part      `json:"parts"`
}
```

#### 6.2 Correct API Format

Based on testing, the correct OpenCode API format:

```go
func (c *OpenCodeClient) SendTask(ctx context.Context, sessionID, task string) (*TaskResponse, error) {
    reqBody := map[string]interface{}{
        "parts": []map[string]interface{}{
            {
                "type": "text",
                "text": task,
            },
        },
    }
    // ... POST to /session/{sessionID}/message
}
```

---

## User Stories

### US-1: Developer delegates coding task to OpenCode

**As a** Station user  
**I want to** have my agent delegate complex coding tasks to OpenCode  
**So that** I get high-quality code changes without manual intervention

**Acceptance Criteria**:
- [ ] Agent can invoke OpenCode via sandbox config
- [ ] OpenCode receives task with full repo context
- [ ] Changes are visible in workspace after completion
- [ ] Execution trace is recorded in Station run

### US-2: Clone and work on private repo

**As a** Station user with private repos  
**I want to** securely pass git credentials to OpenCode  
**So that** it can clone and push to my private repos

**Acceptance Criteria**:
- [ ] GITHUB_TOKEN passed via env var (not in prompts)
- [ ] Git clone works for private repos
- [ ] Git push works with proper auth
- [ ] Credentials never appear in logs or traces

### US-3: Observe OpenCode execution

**As a** Station operator  
**I want to** see detailed traces of OpenCode executions  
**So that** I can debug issues and understand costs

**Acceptance Criteria**:
- [ ] Each tool call is recorded with input/output
- [ ] Token usage and cost tracked per task
- [ ] Execution duration measured
- [ ] Errors captured with context

### US-4: Multi-step workflow with shared workspace

**As a** Station user  
**I want to** run multiple agents that share a workspace  
**So that** one agent can build on another's work

**Acceptance Criteria**:
- [ ] Workspace persists across workflow steps
- [ ] Git commits from step 1 visible in step 2
- [ ] File changes accumulate correctly
- [ ] Cleanup happens only at workflow end

---

## Implementation Phases

### Phase 1: Core Integration (Week 1)
- [ ] Fix OpenCodeBackend API format
- [ ] Implement basic task execution
- [ ] Add credential passing (GITHUB_TOKEN, API keys)
- [ ] Test with simple coding task

### Phase 2: Git Operations (Week 2)
- [ ] Implement git clone before task
- [ ] Support private repos via token auth
- [ ] Collect git commits after task
- [ ] Support SSH keys (optional)

### Phase 3: Observability (Week 2-3)
- [ ] Parse OpenCode response for tool calls
- [ ] Create OpenCodeTrace struct
- [ ] Integrate with Station run events
- [ ] Add OTEL spans

### Phase 4: Workspace Management (Week 3)
- [ ] Implement workspace lifecycle
- [ ] Support workflow-scoped sessions
- [ ] Add file change collection
- [ ] Implement cleanup policies

### Phase 5: Production Hardening (Week 4)
- [ ] Credential redaction in logs
- [ ] Timeout handling
- [ ] Error recovery
- [ ] Documentation

---

## Open Questions

1. **Remote OpenCode**: Should we support OpenCode running on a remote server (not localhost)?
   - Implications for file sync, latency, security

2. **Model Selection**: Should Station control which model OpenCode uses, or let OpenCode decide?
   - Cost implications, capability differences

3. **Tool Restrictions**: Should Station be able to disable certain OpenCode tools (e.g., browser)?
   - Security vs. capability tradeoff

4. **Multi-tenant**: If multiple Station instances share one OpenCode, how to isolate?
   - Session management, workspace isolation

5. **Cost Tracking**: How to attribute OpenCode costs back to Station runs?
   - Token counting, model pricing

---

## Deep Dive: Path Sharing Architecture

### The Problem

Station and OpenCode need to share a filesystem workspace where:
1. Station clones git repos
2. OpenCode reads/writes code
3. Station collects results

### Key Discovery: Directory Query Parameter

OpenCode session creation accepts a `?directory=` query parameter:

```bash
POST /session?directory=/workspaces/my-project
{"title": "coding-task"}

# Response:
{
  "id": "ses_xxx",
  "directory": "/workspaces/my-project",  # OpenCode operates here
  ...
}
```

### Path Sharing Modes

#### Mode 1: Shared Volume Mount (Recommended for Local)

```
┌─────────────────────────────────────────────────────────────────┐
│ Host Machine                                                     │
│                                                                  │
│  /var/station/workspaces/                                        │
│  ├── session-abc/                                                │
│  │   ├── repo/           ◄─── Station clones here               │
│  │   │   ├── src/                                                │
│  │   │   └── tests/                                              │
│  │   └── .git/                                                   │
│  └── session-xyz/                                                │
│                                                                  │
├──────────────────┬──────────────────────────────────────────────┤
│                  │                                               │
│  Station CLI     │    OpenCode Container                         │
│  (host process)  │    docker run -v /var/station/workspaces:     │
│                  │                  /workspaces                  │
│  Sees:           │    Sees:                                      │
│  /var/station/   │    /workspaces/session-abc/repo/              │
│  workspaces/     │                                               │
│                  │                                               │
└──────────────────┴──────────────────────────────────────────────┘
```

**Setup:**
```yaml
# Station config.yaml
sandbox:
  opencode:
    enabled: true
    server_url: http://localhost:4096
    workspace_host_path: /var/station/workspaces    # Path on host
    workspace_container_path: /workspaces            # Path in OpenCode container
```

**Docker run for OpenCode:**
```bash
docker run -d \
  -v /var/station/workspaces:/workspaces \
  -p 4096:4096 \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  ghcr.io/sst/opencode:latest \
  opencode serve --hostname 0.0.0.0 --port 4096
```

**Flow:**
```go
// Station creates workspace
hostPath := "/var/station/workspaces/session-abc"
os.MkdirAll(hostPath, 0755)

// Station clones repo
exec.Command("git", "clone", repoURL, filepath.Join(hostPath, "repo")).Run()

// Station creates OpenCode session with container path
containerPath := "/workspaces/session-abc/repo"
resp := POST("/session?directory=" + url.QueryEscape(containerPath))

// OpenCode operates on /workspaces/session-abc/repo
// Which is the same as /var/station/workspaces/session-abc/repo on host
```

#### Mode 2: Station Also in Container

```yaml
# docker-compose.yml
services:
  station:
    image: station:latest
    volumes:
      - workspaces:/workspaces

  opencode:
    image: ghcr.io/sst/opencode:latest
    volumes:
      - workspaces:/workspaces
    command: opencode serve --hostname 0.0.0.0 --port 4096

volumes:
  workspaces:  # Named volume shared between both
```

Both containers see `/workspaces` - no path translation needed.

#### Mode 3: Remote OpenCode (Different Host)

When OpenCode runs on a different machine, use file sync:

```go
type RemoteOpenCodeBackend struct {
    // Sync files before task
    func (b *RemoteOpenCodeBackend) SyncToRemote(sessionID string, files []File) error {
        // Upload via OpenCode API or rsync
    }
    
    // Sync files after task
    func (b *RemoteOpenCodeBackend) SyncFromRemote(sessionID string) ([]FileChange, error) {
        // Download changed files
    }
}
```

### Path Translation

Station must translate between host paths and container paths:

```go
type PathTranslator struct {
    HostBase      string  // e.g., /var/station/workspaces
    ContainerBase string  // e.g., /workspaces
}

func (t *PathTranslator) ToContainer(hostPath string) string {
    rel, _ := filepath.Rel(t.HostBase, hostPath)
    return filepath.Join(t.ContainerBase, rel)
}

func (t *PathTranslator) ToHost(containerPath string) string {
    rel, _ := filepath.Rel(t.ContainerBase, containerPath)
    return filepath.Join(t.HostBase, rel)
}
```

---

## Deep Dive: Observability

### What We Can Observe

OpenCode returns rich execution data in its response:

```json
{
  "info": {
    "id": "msg_xxx",
    "sessionID": "ses_yyy",
    "modelID": "claude-opus-4-5",
    "providerID": "anthropic",
    "cost": 0.0234,
    "tokens": {
      "input": 5000,
      "output": 1200,
      "reasoning": 500,
      "cache": {"read": 91134, "write": 298}
    },
    "time": {
      "created": 1767029700386,
      "completed": 1767029707632
    },
    "finish": "stop"
  },
  "parts": [
    {"type": "step-start"},
    {"type": "reasoning", "text": "I need to clone the repo first..."},
    {"type": "tool-invocation", "tool": "bash", "input": {"command": "git clone ..."}},
    {"type": "tool-result", "output": "Cloning into 'repo'..."},
    {"type": "tool-invocation", "tool": "write", "input": {"path": "src/health.py", "content": "..."}},
    {"type": "tool-result", "output": "File written successfully"},
    {"type": "text", "text": "I've added the health endpoint at /health..."},
    {"type": "step-finish", "reason": "stop", "cost": 0.0234, "tokens": {...}}
  ]
}
```

### OpenCode Trace Structure

```go
// OpenCodeTrace captures full execution observability
type OpenCodeTrace struct {
    // Identity
    MessageID   string `json:"message_id"`
    SessionID   string `json:"session_id"`
    
    // Model info
    Model       string `json:"model"`        // "claude-opus-4-5"
    Provider    string `json:"provider"`     // "anthropic"
    
    // Cost tracking
    Cost        float64           `json:"cost"`
    Tokens      OpenCodeTokens    `json:"tokens"`
    
    // Timing
    StartTime   time.Time         `json:"start_time"`
    EndTime     time.Time         `json:"end_time"`
    Duration    time.Duration     `json:"duration"`
    
    // Execution details
    Steps       []OpenCodeStep    `json:"steps"`
    ToolCalls   []OpenCodeToolCall `json:"tool_calls"`
    Reasoning   []string          `json:"reasoning,omitempty"`
    
    // Result
    FinalText   string            `json:"final_text"`
    FinishReason string           `json:"finish_reason"`  // "stop", "error", "timeout"
}

type OpenCodeTokens struct {
    Input     int `json:"input"`
    Output    int `json:"output"`
    Reasoning int `json:"reasoning"`
    CacheRead int `json:"cache_read"`
    CacheWrite int `json:"cache_write"`
}

type OpenCodeToolCall struct {
    Tool      string                 `json:"tool"`       // "bash", "write", "read", etc.
    Input     map[string]interface{} `json:"input"`
    Output    string                 `json:"output"`
    ExitCode  int                    `json:"exit_code,omitempty"`
    Duration  time.Duration          `json:"duration"`
    Error     string                 `json:"error,omitempty"`
}
```

### Parsing OpenCode Response

```go
func parseOpenCodeTrace(body []byte) (*OpenCodeTrace, error) {
    var resp struct {
        Info struct {
            ID         string `json:"id"`
            SessionID  string `json:"sessionID"`
            ModelID    string `json:"modelID"`
            ProviderID string `json:"providerID"`
            Cost       float64 `json:"cost"`
            Tokens     struct {
                Input     int `json:"input"`
                Output    int `json:"output"`
                Reasoning int `json:"reasoning"`
                Cache     struct {
                    Read  int `json:"read"`
                    Write int `json:"write"`
                } `json:"cache"`
            } `json:"tokens"`
            Time struct {
                Created   int64 `json:"created"`
                Completed int64 `json:"completed"`
            } `json:"time"`
            Finish string `json:"finish"`
        } `json:"info"`
        Parts []struct {
            Type   string                 `json:"type"`
            Text   string                 `json:"text,omitempty"`
            Tool   string                 `json:"tool,omitempty"`
            Input  map[string]interface{} `json:"input,omitempty"`
            Output string                 `json:"output,omitempty"`
        } `json:"parts"`
    }
    
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, err
    }
    
    trace := &OpenCodeTrace{
        MessageID:    resp.Info.ID,
        SessionID:    resp.Info.SessionID,
        Model:        resp.Info.ModelID,
        Provider:     resp.Info.ProviderID,
        Cost:         resp.Info.Cost,
        StartTime:    time.UnixMilli(resp.Info.Time.Created),
        EndTime:      time.UnixMilli(resp.Info.Time.Completed),
        FinishReason: resp.Info.Finish,
        Tokens: OpenCodeTokens{
            Input:      resp.Info.Tokens.Input,
            Output:     resp.Info.Tokens.Output,
            Reasoning:  resp.Info.Tokens.Reasoning,
            CacheRead:  resp.Info.Tokens.Cache.Read,
            CacheWrite: resp.Info.Tokens.Cache.Write,
        },
    }
    trace.Duration = trace.EndTime.Sub(trace.StartTime)
    
    // Extract tool calls and reasoning
    for _, part := range resp.Parts {
        switch part.Type {
        case "tool-invocation":
            // Start tracking tool call
        case "tool-result":
            trace.ToolCalls = append(trace.ToolCalls, OpenCodeToolCall{
                Tool:   part.Tool,
                Input:  part.Input,
                Output: part.Output,
            })
        case "reasoning":
            trace.Reasoning = append(trace.Reasoning, part.Text)
        case "text":
            trace.FinalText += part.Text
        }
    }
    
    return trace, nil
}
```

### OTEL Span Integration

```go
func (b *OpenCodeBackend) ExecWithTracing(ctx context.Context, sessionID string, task string) (*ExecResult, error) {
    ctx, span := otel.Tracer("station").Start(ctx, "opencode.task")
    defer span.End()
    
    // Execute task
    response, trace, err := b.sendTaskWithTrace(ctx, sessionID, task)
    
    if trace != nil {
        // Set span attributes
        span.SetAttributes(
            attribute.String("opencode.session_id", trace.SessionID),
            attribute.String("opencode.model", trace.Model),
            attribute.String("opencode.provider", trace.Provider),
            attribute.Float64("opencode.cost", trace.Cost),
            attribute.Int("opencode.tokens.input", trace.Tokens.Input),
            attribute.Int("opencode.tokens.output", trace.Tokens.Output),
            attribute.Int("opencode.tool_calls", len(trace.ToolCalls)),
        )
        
        // Create child spans for each tool call
        for _, tc := range trace.ToolCalls {
            _, tcSpan := otel.Tracer("station").Start(ctx, "opencode.tool."+tc.Tool)
            tcSpan.SetAttributes(
                attribute.String("tool.name", tc.Tool),
                attribute.String("tool.output_preview", truncate(tc.Output, 500)),
            )
            tcSpan.End()
        }
    }
    
    return &ExecResult{...}, nil
}
```

### Jaeger Trace View

```
station.agent.execute (10.5s)
├── opencode.task (8.2s)
│   ├── opencode.tool.bash "git clone ..." (2.1s)
│   ├── opencode.tool.read "src/api.py" (0.1s)
│   ├── opencode.tool.write "src/health.py" (0.2s)
│   ├── opencode.tool.bash "pytest tests/" (3.5s)
│   └── opencode.tool.bash "git commit -m ..." (0.3s)
└── station.collect_results (0.5s)
```

### Storing Traces in Station DB

```sql
-- Run events table
INSERT INTO run_events (run_id, seq, type, payload) VALUES (
    'run_123',
    5,
    'opencode_trace',
    '{
        "message_id": "msg_xxx",
        "model": "claude-opus-4-5",
        "cost": 0.0234,
        "duration_ms": 7246,
        "tool_calls": [
            {"tool": "bash", "input": "git clone..."},
            {"tool": "write", "input": {"path": "src/health.py"}}
        ]
    }'
);
```

---

## Phase 8: NATS-Based Communication (Next)

**Status**: Plugin Complete ✅, Station Integration Pending

### 8.1 Architecture Evolution

The Phase 1-7 implementation uses **direct HTTP** from Station to OpenCode. This works but has limitations:
- Requires OpenCode to be directly reachable from Station
- No message persistence if OpenCode is temporarily down
- Limited streaming capabilities
- Tight coupling between Station and OpenCode lifecycle

**New Architecture: NATS Message Bus**

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              NATS JetStream                                   │
│                                                                               │
│   ┌─────────────────────┐    ┌─────────────────────┐    ┌─────────────────┐  │
│   │ station.coding.task │    │ station.coding.stream│   │ station.coding. │  │
│   │ (task requests)     │    │ (execution events)   │   │ result          │  │
│   └─────────────────────┘    └─────────────────────┘   └─────────────────┘  │
│            ▲                          │                        │             │
│            │                          ▼                        ▼             │
├────────────┼──────────────────────────┼────────────────────────┼─────────────┤
│            │                          │                        │             │
│   ┌────────┴────────┐        ┌────────┴────────┐      ┌────────┴────────┐   │
│   │  Station Agent  │        │  Station Agent  │      │  Station Agent  │   │
│   │  (Publisher)    │        │  (Subscriber)   │      │  (Subscriber)   │   │
│   └─────────────────┘        └─────────────────┘      └─────────────────┘   │
│                                                                               │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                    OpenCode Container                                 │   │
│   │   ┌───────────────────────────────────────────────────────────────┐ │   │
│   │   │  @cloudshipai/opencode-plugin                                  │ │   │
│   │   │    ├── TaskHandler      (consumes task, drives OpenCode)      │ │   │
│   │   │    ├── EventPublisher   (streams events, publishes result)    │ │   │
│   │   │    ├── WorkspaceManager (git clone, dirty detection)          │ │   │
│   │   │    └── SessionManager   (KV-backed session persistence)       │ │   │
│   │   └───────────────────────────────────────────────────────────────┘ │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                               │
│   ┌─────────────────────┐    ┌─────────────────────┐                         │
│   │ KV: coding.sessions │    │ KV: coding.state    │                         │
│   │ (session metadata)  │    │ (agent context)     │                         │
│   └─────────────────────┘    └─────────────────────┘                         │
│                                                                               │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │ Object Store: coding.artifacts                                       │   │
│   │ (large files, diffs, snapshots)                                      │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 8.2 OpenCode Plugin (Complete)

The OpenCode plugin (`@cloudshipai/opencode-plugin`) is **published and tested**:

- **npm**: `@cloudshipai/opencode-plugin@0.1.0` (pending NPM_TOKEN)
- **Docker**: `ghcr.io/cloudshipai/opencode-station:0.1.0` ✅

**Plugin Capabilities**:

| Feature | Status | Description |
|---------|--------|-------------|
| NATS Connection | ✅ | Connects to NATS on startup, graceful degradation |
| Task Subscription | ✅ | Consumes from `station.coding.task` |
| Workspace Management | ✅ | Git clone, branch checkout, dirty detection |
| Session Management | ✅ | Create/continue sessions, message counting |
| Stream Events | ✅ | Real-time events to `callback.streamSubject` |
| Result Publishing | ✅ | Final result to `callback.resultSubject` |
| KV Tools | ✅ | `station_kv_get`, `station_kv_set`, `station_session_info` |

**Test Results**: 15 integration tests passing

### 8.3 Station-Side NATS Dispatcher (TODO)

Station needs a **NATS-based dispatcher** to replace direct HTTP calls.

**Files to Create/Modify**:

| File | Changes |
|------|---------|
| `internal/coding/nats_backend.go` | New backend using NATS instead of HTTP |
| `internal/coding/nats_client.go` | NATS connection, pub/sub, KV/Object Store |
| `internal/coding/tool.go` | Update tools to use NATS backend |
| `internal/services/coding_tool_factory.go` | Wire NATS backend based on config |

**Configuration**:

```yaml
# config.yaml
coding:
  backend: opencode-nats  # NEW: NATS-based backend
  nats:
    url: nats://localhost:4222
    subjects:
      task: station.coding.task
      stream: station.coding.stream
      result: station.coding.result
    kv:
      sessions: coding-sessions   # KV bucket for session state
      state: coding-state         # KV bucket for agent context
    object_store: coding-artifacts
  # ... existing opencode config for fallback
```

**Tool Changes**:

The existing `coding_open`, `code`, `coding_close` tools would:

1. **coding_open**: 
   - Publish task to NATS with callback subjects
   - Wait for `session_created` event on stream
   - Return session info from result

2. **code**:
   - Publish task to NATS with session reference
   - Subscribe to stream subject for real-time events
   - Collect result from result subject
   - Return completion with trace data

3. **coding_close**:
   - Publish close request via NATS
   - Collect final workspace state
   - Cleanup local resources

### 8.4 NATS KV Store Usage

**Session State (KV: `coding-sessions`)**:

Stores session metadata for continuation across agent runs.

```typescript
// Key: session name (e.g., "agent-123-session-1")
interface SessionState {
  sessionName: string;
  opencodeID: string;        // OpenCode's internal session ID
  workspaceName: string;
  workspacePath: string;
  git?: {
    url: string;
    branch: string;
    lastCommit: string;
  };
  messageCount: number;
  created: number;           // Unix timestamp
  lastUsed: number;          // Unix timestamp
  metadata?: Record<string, unknown>;
}
```

**Agent Context (KV: `coding-state`)**:

Stores agent-specific context for cross-run persistence.

```typescript
// Key: agent ID + context key (e.g., "finops-analyzer:last-report")
interface AgentContext {
  agentID: string;
  key: string;
  value: unknown;
  updated: number;
  ttl?: number;              // Optional TTL in seconds
}
```

**Station-Side KV Operations**:

```go
// internal/coding/nats_client.go
type NATSCodingClient struct {
    nc       *nats.Conn
    js       nats.JetStreamContext
    sessions nats.KeyValue  // coding-sessions bucket
    state    nats.KeyValue  // coding-state bucket
}

func (c *NATSCodingClient) GetSession(name string) (*SessionState, error)
func (c *NATSCodingClient) SaveSession(session *SessionState) error
func (c *NATSCodingClient) GetContext(agentID, key string) ([]byte, error)
func (c *NATSCodingClient) SetContext(agentID, key string, value []byte, ttl time.Duration) error
```

### 8.5 NATS Object Store Usage

**Object Store: `coding-artifacts`**

For sharing large files between Station and OpenCode:

| Use Case | Object Key Pattern | Direction |
|----------|-------------------|-----------|
| Input files | `input/{taskID}/{filename}` | Station → OpenCode |
| Output diffs | `output/{taskID}/changes.patch` | OpenCode → Station |
| Snapshots | `snapshot/{sessionName}/{timestamp}.tar.gz` | OpenCode → Station |
| Binary artifacts | `artifacts/{taskID}/{filename}` | Bidirectional |

**File Staging Flow**:

```
1. Station prepares large input files
   → Upload to Object Store: input/{taskID}/data.json

2. Station publishes task with file references
   → Task includes: { files: ["input/{taskID}/data.json"] }

3. Plugin downloads files to workspace
   → GET from Object Store → /workspaces/{name}/data.json

4. OpenCode processes task
   → Reads/writes files in workspace

5. Plugin stages output files
   → Collects changes → Upload to Object Store

6. Plugin publishes result with artifact references
   → Result includes: { artifacts: ["output/{taskID}/changes.patch"] }

7. Station retrieves artifacts
   → Download from Object Store → Apply locally
```

**Station-Side Object Store Operations**:

```go
// internal/coding/nats_client.go
type NATSCodingClient struct {
    // ...existing fields...
    artifacts nats.ObjectStore  // coding-artifacts store
}

func (c *NATSCodingClient) StageInputFile(taskID, filename string, data []byte) error
func (c *NATSCodingClient) GetOutputArtifact(taskID, filename string) ([]byte, error)
func (c *NATSCodingClient) ListArtifacts(taskID string) ([]nats.ObjectInfo, error)
func (c *NATSCodingClient) CleanupTask(taskID string) error
```

### 8.6 Streaming Events

Real-time events from OpenCode to Station for observability:

**Event Types**:

| Event | When | Data |
|-------|------|------|
| `session_created` | New session started | `{ sessionName, opencodeID, workspacePath }` |
| `session_reused` | Continuing existing session | `{ sessionName, messageCount }` |
| `workspace_created` | New workspace initialized | `{ workspaceName, path, git? }` |
| `workspace_reused` | Using existing workspace | `{ workspaceName, dirty, lastCommit }` |
| `git_clone` | Cloning repository | `{ url, branch, duration }` |
| `git_pull` | Pulling updates | `{ branch, commits, duration }` |
| `prompt_sent` | Task submitted to OpenCode | `{ promptLength }` |
| `text` | Text response chunk | `{ text }` |
| `thinking` | Reasoning text | `{ text }` |
| `tool_start` | Tool invocation started | `{ tool, input }` |
| `tool_end` | Tool invocation completed | `{ tool, output, duration }` |
| `error` | Error occurred | `{ message, code, recoverable }` |

**Station-Side Event Handling**:

```go
// internal/coding/nats_backend.go
func (b *NATSBackend) streamEvents(ctx context.Context, streamSubject string) <-chan CodingEvent {
    events := make(chan CodingEvent)
    sub, _ := b.client.Subscribe(streamSubject, func(msg *nats.Msg) {
        var event CodingEvent
        json.Unmarshal(msg.Data, &event)
        select {
        case events <- event:
        case <-ctx.Done():
        }
    })
    // ... cleanup on context cancel
    return events
}
```

### 8.7 Implementation Plan

| Phase | Tasks | Effort |
|-------|-------|--------|
| 8.1 | NATS client wrapper for Station | 2 days |
| 8.2 | NATSBackend implementing Backend interface | 2 days |
| 8.3 | Update coding tools to use NATS dispatch | 1 day |
| 8.4 | KV integration for session/context | 1 day |
| 8.5 | Object Store for file staging | 2 days |
| 8.6 | Stream event handling and OTEL integration | 1 day |
| 8.7 | E2E tests with Docker Compose | 1 day |
| 8.8 | Documentation and config examples | 1 day |

**Total**: ~11 days

### 8.8 Configuration Examples

**Minimal Setup (Local Dev)**:

```yaml
# config.yaml
coding:
  backend: opencode-nats
  nats:
    url: nats://localhost:4222
```

**Production Setup**:

```yaml
# config.yaml
coding:
  backend: opencode-nats
  nats:
    url: nats://nats.internal:4222
    creds_file: /etc/station/nats.creds
    subjects:
      task: org.{org_id}.coding.task
      stream: org.{org_id}.coding.stream.{task_id}
      result: org.{org_id}.coding.result.{task_id}
    kv:
      sessions: coding-sessions-{org_id}
      state: coding-state-{org_id}
    object_store: coding-artifacts-{org_id}
  workspace_base_path: /var/station/workspaces
  cleanup_policy: on_session_end
  git:
    token_env: GITHUB_TOKEN
    user_name: "Station Bot"
    user_email: "station@company.com"
```

**Docker Compose (Testing)**:

```yaml
# docker-compose.yaml
services:
  nats:
    image: nats:2.10-alpine
    command: -js -m 8222
    ports:
      - "4222:4222"
      - "8222:8222"

  opencode:
    image: ghcr.io/cloudshipai/opencode-station:0.1.0
    environment:
      - NATS_URL=nats://nats:4222
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    volumes:
      - workspaces:/workspaces
    depends_on:
      - nats

  station:
    image: ghcr.io/cloudshipai/station:latest
    environment:
      - STN_CODING_BACKEND=opencode-nats
      - STN_NATS_URL=nats://nats:4222
    volumes:
      - workspaces:/workspaces
    depends_on:
      - nats
      - opencode

volumes:
  workspaces:
```

---

## Phase 9: Multi-Tenant Isolation (Future)

When running in multi-tenant mode (CloudShip platform):

### 9.1 Subject Namespacing

```
org.{org_id}.coding.task          # Per-org task queue
org.{org_id}.coding.stream.{id}   # Per-task stream (ephemeral)
org.{org_id}.coding.result.{id}   # Per-task result (ephemeral)
```

### 9.2 KV/Object Store Buckets

```
coding-sessions-{org_id}          # Per-org session state
coding-state-{org_id}             # Per-org context
coding-artifacts-{org_id}         # Per-org file storage
```

### 9.3 Workspace Isolation

```
/workspaces/{org_id}/{session_name}/
```

### 9.4 Credential Scoping

Git tokens and API keys are scoped to org-level secrets in CloudShip.

---

## Appendix

### A. OpenCode API Reference

```
GET  /global/health          - Health check
GET  /session                - List sessions
POST /session                - Create session
GET  /session/{id}           - Get session
POST /session/{id}/message   - Send task (main API)
GET  /session/{id}/message   - Get messages
GET  /event                  - SSE for real-time events
```

### B. OpenCode Tools Available

| Tool | Description |
|------|-------------|
| `bash` | Execute shell commands |
| `read` | Read file contents |
| `write` | Write/create files |
| `edit` | Edit existing files |
| `glob` | Find files by pattern |
| `grep` | Search file contents |
| `browser` | Web browsing (if enabled) |
| `todowrite` | Manage task lists |

### C. Related Documents

- Station Sandbox Architecture: `docs/SANDBOX_ARCHITECTURE.md`
- OpenCode Official Docs: https://opencode.ai/docs
