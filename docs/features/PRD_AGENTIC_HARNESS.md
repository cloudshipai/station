# PRD: Agentic Harness - Claude Agent SDK-like Execution Engine

**Status**: Phase 6 Complete (Interactive REPL with Session Persistence)
**Author**: Claude/Human Collaboration
**Created**: 2025-01-06
**Last Updated**: 2025-01-26

## Overview

The Agentic Harness is an alternative execution engine for Station agents that provides Claude Agent SDK-like capabilities: manual agentic loop control, doom loop detection, context compaction, git integration, and workspace management.

## Problem Statement

The default Genkit-based execution (`dotprompt.ExecuteAgent`) works well for most agents but has limitations:

1. **No doom loop detection** - Agents can get stuck in repetitive patterns
2. **No context compaction** - Long conversations can exceed context windows
3. **Limited git integration** - No automatic branch/commit management
4. **No workspace isolation** - Agents share the same file system context
5. **No manual loop control** - Cannot customize step behavior or hooks

## Solution

Add `harness: agentic` support to agent dotprompt files, enabling a Claude Agent SDK-like execution model with:

- Manual agentic loop with step-by-step control
- Doom loop detection and prevention
- Context compaction for long-running tasks
- Git integration (auto-branch, auto-commit)
- Workspace management and isolation
- Pre/post tool execution hooks

## Architecture

### Integration Point

The harness integrates at the `AgentExecutionEngine.Execute()` level:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AgentExecutionEngine.Execute()                    │
│                              │                                       │
│         ┌────────────────────┴────────────────────┐                  │
│         ▼                                         ▼                  │
│  harness: "" (default)                    harness: agentic           │
│         │                                         │                  │
│         ▼                                         ▼                  │
│  dotprompt.ExecuteAgent()            executeWithAgenticHarness()     │
│  (Genkit-based execution)             (Manual agentic loop)          │
│                                                   │                  │
│                                                   ▼                  │
│                                        pkg/harness/AgenticExecutor   │
│                                        • Doom loop detection         │
│                                        • Context compaction          │
│                                        • Git integration             │
│                                        • Workspace management        │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Structure

```
pkg/harness/
├── executor.go           # AgenticExecutor - main execution engine
├── config.go             # HarnessConfig, AgentHarnessConfig
├── doom_loop.go          # DoomLoopDetector
├── compaction.go         # Compactor for context management
├── hooks.go              # HookRegistry for pre/post tool hooks
├── tools/
│   ├── registry.go       # ToolRegistry
│   ├── read.go           # File read tool
│   ├── write.go          # File write tool
│   ├── edit.go           # File edit tool
│   ├── bash.go           # Bash command tool
│   ├── glob.go           # File glob tool
│   ├── grep.go           # Content search tool
│   └── git_tools.go      # Git operation tools
├── workspace/
│   └── host.go           # HostWorkspace implementation
├── git/
│   └── manager.go        # GitManager for branch/commit operations
├── prompt/
│   └── builder.go        # PromptBuilder for system prompts
└── trace/
    └── tracer.go         # OpenTelemetry tracing integration
```

## Usage

### Agent Configuration

Add `harness: agentic` to the agent's dotprompt frontmatter:

```yaml
---
model: anthropic/claude-sonnet-4-20250514
harness: agentic
harness_config:
  max_steps: 50
  doom_loop_threshold: 3
  timeout: 10m
tools:
  - read_file
  - write_file
  - bash
---
You are a code analysis agent...
```

### Global Configuration

Configure harness defaults in `config.yaml`. Running `stn init` sets these defaults automatically:

```yaml
harness:
  workspace:
    path: ./workspace
    mode: host  # or "sandbox"
  compaction:
    enabled: true
    threshold: 0.85       # Compact at 85% of context window
    protect_tokens: 40000 # Keep last N tokens from compaction
  git:
    auto_branch: true
    branch_prefix: agent/
    auto_commit: false
    require_approval: true
    workflow_branch_strategy: shared
  nats:
    enabled: true
    kv_bucket: harness-state
    object_bucket: harness-files
    max_file_size: 100MB
    ttl: 24h
  permissions:
    external_directory: deny
    # bash permissions use code defaults (see DefaultHarnessConfig)
```

The `stn init` command also creates the workspace directory at `~/.config/station/workspace/`.

## Key Features

### 1. Doom Loop Detection

Detects when an agent is stuck in a repetitive pattern:

```go
type DoomLoopDetector struct {
    threshold  int               // Number of similar calls to trigger
    history    []ToolCallRecord  // Recent tool calls
    maxHistory int               // Maximum history to keep
}

// Triggers when same tool+input seen N times consecutively
func (d *DoomLoopDetector) IsInLoop() bool
```

**Configuration**:
```yaml
harness_config:
  doom_loop_threshold: 3  # Trigger after 3 identical calls
```

### 2. Context Compaction

Automatically summarizes conversation history when approaching context limits:

```go
type Compactor struct {
    genkitApp     *genkit.Genkit
    modelName     string
    config        CompactionConfig
    contextWindow int
}

// Returns compacted history when token count exceeds threshold
func (c *Compactor) CompactIfNeeded(ctx context.Context, history []*ai.Message) ([]*ai.Message, bool, error)
```

**Configuration**:
```yaml
harness:
  compaction:
    enabled: true
    threshold: 0.8        # 80% of context window
    protect_tokens: 2000  # Protect recent tokens
```

### 3. Git Integration

Automatic branch creation and commits:

```go
type GitManager interface {
    CreateBranch(ctx context.Context, task string, agentID string) (string, error)
    Commit(ctx context.Context, message string) (string, error)
    Push(ctx context.Context) error
    GetCurrentBranch(ctx context.Context) (string, error)
}
```

**Configuration**:
```yaml
harness:
  git:
    auto_branch: true
    branch_prefix: agent/
    auto_commit: true
```

### 4. Built-in Tools

The harness provides built-in tools that work independently of MCP:

| Tool | Description |
|------|-------------|
| `read` | Read file contents |
| `write` | Write file contents |
| `edit` | Edit file with string replacement |
| `bash` | Execute bash commands |
| `glob` | Find files by pattern |
| `grep` | Search file contents |
| `git_status` | Get git status |
| `git_diff` | Get git diff |
| `git_log` | Get git log |

### 5. Hook System

Pre/post tool execution hooks for:
- Permission checks
- Audit logging
- Doom loop detection
- Custom validation

```go
type HookResult string
const (
    HookContinue  HookResult = "continue"
    HookBlock     HookResult = "block"
    HookInterrupt HookResult = "interrupt"
)

type PreToolHook func(ctx context.Context, req *ai.ToolRequest) (HookResult, string)
type PostToolHook func(ctx context.Context, req *ai.ToolRequest, result interface{})
```

## Workflow Integration

The harness works with Station's workflow system. Agents with `harness: agentic` are executed through `executeWithAgenticHarness()` when triggered by workflows:

```yaml
# workflow.yaml
id: code-review
states:
  - name: analyze
    type: agent
    agent: code-analyzer  # Has harness: agentic in dotprompt
    transition: report
```

## Multi-Agent Handoff

The `pkg/harness/nats/LatticeAdapter` bridges harness agents with Station's existing lattice infrastructure:

```go
// LatticeAdapter uses existing WorkStore for state persistence
adapter, err := nats.NewLatticeAdapter(js, nats.LatticeAdapterConfig{
    StationID:       stationID,
    WorkStoreConfig: work.DefaultWorkStoreConfig(),
    FileStoreConfig: nats.DefaultFileStoreConfig(),
})

// Create work record for harness execution
record, err := adapter.CreateHarnessWork(ctx, nats.CreateHarnessWorkInput{
    WorkID:            workID,
    WorkflowRunID:     workflowRunID,
    StepID:            stepID,
    AgentID:           agentID,
    AgentName:         agentName,
    Task:              task,
    OrchestratorRunID: orchestratorRunID,
})

// Upload/download files between workflow steps
adapter.UploadOutputFile(ctx, workID, localPath)
adapter.DownloadPreviousStepFiles(ctx, orchestratorRunID, localDir)
```

The adapter stores workflow metadata (WorkflowRunID, StepID, GitBranch) in the `Context` map of `work.WorkRecord`, maintaining compatibility with the existing lattice infrastructure.

## Testing

### Unit Tests
```bash
go test ./pkg/harness/... -v
```

### Integration Tests (with real LLM)
```bash
HARNESS_E2E_TEST=1 go test ./pkg/harness/... -tags=integration -run TestAgenticExecutor -v
```

### Multi-Agent E2E Test
```bash
HARNESS_E2E_TEST=1 go test ./pkg/harness/... -tags=integration -run TestMultiAgentWorkflow -v -timeout 10m
```

## Success Criteria

- [x] Harness execution mode selectable via `harness: agentic` frontmatter
- [x] Doom loop detection prevents infinite loops
- [x] Built-in tools work independently of MCP
- [x] Git integration creates branches and commits
- [x] Workspace isolation prevents file conflicts
- [x] Context compaction tested with real LLM (E2E test with 1000 token window)
- [x] Multi-agent handoff uses existing lattice infrastructure (LatticeAdapter in pkg/harness/nats/)
- [x] E2E test with workflow simulation passes (TestAgenticExecutor_E2E_WorkflowSimulation)
- [x] `stn init` provides sensible harness defaults

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Doom loop false positives | Configurable threshold, tool+input hashing |
| Compaction loses context | Protected token window, summarization prompt |
| Git conflicts | Branch per agent/task, require approval option |
| MCP tool conflicts | Built-in tools have distinct names |

## Timeline

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1-8 | Complete | Core harness, tools, doom loop, workspace, git |
| Phase 9 | Complete | NATS integration via LatticeAdapter (uses work.WorkStore) |
| Phase 10 | Complete | Multi-agent E2E test with real LLM |
| Phase 11 | Complete | Compaction E2E test with configurable token window |
| Phase 12 | Complete | Workflow simulation E2E test (multi-agent pipeline) |

## E2E Test Results (2025-01-06)

All tests pass with Claude Opus 4.5:

| Test | Duration | Tokens | Description |
|------|----------|--------|-------------|
| `TestAgenticExecutor_E2E_RealLLM` | ~7s | ~2,100 | Basic file create/read |
| `TestAgenticExecutor_E2E_MultiStep` | ~11s | ~2,700 | Directory + JSON config |
| `TestAgenticExecutor_E2E_Compaction` | ~10s | ~2,500 | Compaction config wired |
| `TestAgenticExecutor_E2E_WorkflowSimulation` | ~13s | ~4,400 | Multi-agent workflow |

Run tests:
```bash
HARNESS_E2E_TEST=1 go test ./pkg/harness/... -v -run "E2E" -timeout 5m
```

## Phase 2: Workspace Isolation & Session Persistence (IN PROGRESS)

### Problem Statement

The current harness uses a single shared workspace path for all executions. This causes issues:

1. **Concurrent runs conflict** - Two agents writing to same files
2. **No continuation** - Can't resume work from previous run
3. **No repo targeting** - Can't tell agent "work on this specific repo"
4. **Workflow vs standalone** - Different isolation needs

### Invocation Sources

Agents can be invoked from multiple sources, each with different workspace needs:

| Source | Identity Provider | Example |
|--------|------------------|---------|
| CLI | User provides `--session` or `--repo` | `stn call coder --session proj-123` |
| Workflow | Engine provides `workflow_run_id` | Steps share workspace automatically |
| Agent handoff | Parent context propagates | Child inherits parent's session |
| Schedule | Config specifies | `schedule.session_id: "nightly-scan"` |
| Event/webhook | Payload contains | `{"session_id": "pr-456", "repo": "..."}` |
| CloudShip | Remote orchestration | Dispatch includes repo URL |

### Proposed Design

```
┌─────────────────────────────────────────────────────────────────┐
│                    RUNTIME PARAMETERS                            │
│  (passed at invocation time, not config)                        │
├─────────────────────────────────────────────────────────────────┤
│  session_id     - User-provided, persists across runs           │
│  workflow_run_id - From workflow engine context                 │
│  agent_run_id   - Auto-generated per execution                  │
│  git_source     - {url, branch, ref} to clone                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    WORKSPACE RESOLVER                            │
│                                                                  │
│  Resolution Priority:                                            │
│  1. session_id provided     → workspace/session/{session_id}/   │
│  2. workflow_run_id present → workspace/workflow/{wf_run_id}/   │
│  3. fallback               → workspace/run/{agent_run_id}/      │
│                                                                  │
│  Git Source Handling:                                            │
│  - If git_source provided AND workspace empty → clone repo      │
│  - If git_source provided AND workspace exists → fetch/checkout │
│  - If no git_source → use workspace as-is                       │
└─────────────────────────────────────────────────────────────────┘
```

### Configuration

**Global defaults (config.yaml):**
```yaml
harness:
  workspace:
    path: ./workspace
    mode: host
    isolation_mode: per_workflow  # shared | per_run | per_workflow
    cleanup_on_complete: false
    cleanup_after: 24h
```

**Runtime parameters (passed at invocation):**
```go
type ExecutionContext struct {
    SessionID     string          // User-provided, highest priority
    WorkflowRunID string          // From workflow context
    AgentRunID    string          // Auto-generated
    AgentName     string          // For branch naming
    Task          string          // For branch naming
    GitSource     *GitSourceConfig // Optional repo to clone
}
```

**CLI usage:**
```bash
# Fresh isolated run (default)
stn call my-agent --task "analyze code"

# Continue from session
stn call my-agent --task "continue refactoring" --session "refactor-auth"

# Work on specific repo
stn call my-agent --task "review PR" --repo "https://github.com/org/repo" --ref "pr-123"

# Combine: specific repo with persistent session
stn call my-agent --task "fix tests" --repo "git@github.com:org/repo" --session "fix-tests-sprint-42"
```

**Workflow usage:**
```yaml
id: code-review-pipeline
input:
  repo_url: string
  session_id: string  # Optional: persist across workflow runs

states:
  - name: clone
    type: agent
    agent: repo-setup
    input:
      git_source:
        url: "${repo_url}"
        depth: 1
      session_id: "${session_id}"  # All steps share this
    
  - name: analyze
    type: agent
    agent: code-analyzer
    # Inherits session from workflow context
```

### Design Decisions

#### 1. Collaboration Modes

Two modes for how agents interact within a session:

```
SEQUENTIAL MODE (default)
━━━━━━━━━━━━━━━━━━━━━━━━━
Agent A ──commits──► Agent B ──commits──► Agent C
              same branch (pick up where left off)

Use: Pipeline where each step builds on previous
Example: analyze → fix → test → document


PARALLEL MODE (explicit)
━━━━━━━━━━━━━━━━━━━━━━━━━
         ┌── Agent A (branch: agent/session/frontend)
base ────┼── Agent B (branch: agent/session/backend)
         └── Agent C (branch: agent/session/docs)
                        │
                        ▼
                   merge step

Use: Independent work that gets combined later
Example: parallel feature development, multiple reviewers
```

**Configuration:**
```yaml
harness:
  git:
    collaboration: sequential  # sequential | parallel
    working_branch: ""         # Empty = current branch
```

#### 2. Concurrent Access

**Decision: Mode-dependent behavior**

| Mode | Same session, same time | Behavior |
|------|------------------------|----------|
| Sequential | Second caller blocked | "Session locked by run X" |
| Parallel | Allowed | Each gets own branch |

For sequential mode, we use advisory file locks on the session directory.

#### 3. Session Lifecycle

**Decision: Explicit management with warnings**

```bash
stn session list                      # Shows sessions + last-used time
stn session delete proj-123           # Explicit cleanup  
stn session cleanup --older-than 7d   # Bulk cleanup
```

Sessions persist until explicitly deleted. Station warns about stale sessions.

#### 4. Workflow Configuration

**Workflow-level git settings:**
```yaml
id: code-review-pipeline
git:
  collaboration: sequential  # Default for all steps
  working_branch: feature/review-${input.pr_number}

states:
  - name: analyze
    type: agent
    agent: analyzer
    # Uses workflow git settings (sequential)
    
  - name: parallel_checks
    type: parallel
    git:
      collaboration: parallel  # Override for this section
    branches:
      - agent: security-scanner
      - agent: perf-analyzer
    transition: merge
    
  - name: merge
    type: agent
    agent: merge-bot
    # Back to sequential (workflow default)
```

#### 5. Mental Model Summary

```
SESSION = Named, persistent workspace with git repo
├── Has a WORKING BRANCH (the main line of work)
├── Tracks which RUN currently holds the lock (sequential mode)
└── Persists until explicitly deleted

SEQUENTIAL COLLABORATION
├── Agents work on same branch, same files
├── Each sees previous agent's commits  
├── "Pick up where left off" semantics
├── Only ONE active run at a time (locked)
└── Default for workflows

PARALLEL COLLABORATION  
├── Each agent gets own branch from working branch
├── Multiple concurrent runs OK
├── Requires merge step to combine
└── Must be explicitly configured

RUN = Single agent execution
├── In sequential: acquires session lock, works on working branch
├── In parallel: creates/uses own branch
└── On completion: commits changes, releases lock (if held)
```

### Implementation Plan

| Phase | Description | Status |
|-------|-------------|--------|
| 2.1 | Add `ExecutionContext` to harness config | ✅ Done |
| 2.2 | Implement `ResolveWorkspacePath()` | ✅ Done |
| 2.3 | Add `git/cloner.go` for repo cloning | ✅ Done |
| 2.4 | Add session lock manager for sequential mode | ✅ Done |
| 2.5 | Wire into `executeWithAgenticHarness()` | ✅ Done |
| 2.6 | Add real-time streaming with full identifiers | ✅ Done |
| 2.7 | Session management commands (`stn session list/delete/cleanup/unlock/info`) | ✅ Done |
| 2.8 | Add CLI flags (`--session`, `--repo`, `--collaboration`) | Pending |
| 2.9 | Add workflow git/collaboration settings parsing | Pending |
| 2.10 | E2E tests for sequential continuation | Pending |
| 2.11 | E2E tests for parallel branches + merge | Pending |

### Success Criteria (Phase 2)

- [ ] **Sequential mode**: Agent B sees Agent A's commits on same branch
- [ ] **Parallel mode**: Each agent gets isolated branch, merge step combines
- [ ] **Session locking**: Sequential mode blocks concurrent runs on same session
- [ ] **Session persistence**: `--session` flag continues from previous run
- [ ] **Git repo cloning**: `--repo` flag clones repo into session workspace
- [ ] **Workflow integration**: `git.collaboration` setting propagates to steps
- [ ] **CLI commands**: `stn session list/delete/cleanup` work correctly
- [ ] **Lock recovery**: Stale locks (crashed runs) auto-expire after timeout
- [x] **Real-time streaming**: Events streamed with full ownership identifiers

## Phase 2.6: Real-Time Streaming (COMPLETE)

### Overview

The streaming system enables real-time visibility into agent execution, similar to Claude Code/OpenCode. Events are published to NATS and can be consumed by Lighthouse for forwarding to CloudShip platform.

### Architecture

```
AgenticExecutor.runLoop()
    │
    ├─► run_start ──────────► StreamEvent
    │
    ├─► For each step:
    │   ├─► tool_start ─────► StreamEvent
    │   ├─► tool_result ────► StreamEvent
    │   └─► step_complete ──► StreamEvent
    │
    └─► run_complete ───────► StreamEvent
            │
            ▼
    NATS: station.{station_id}.run.{run_uuid}.stream
            │
            ▼
    Lighthouse StreamConsumer (future)
            │
            ▼
    CloudShip Platform → WebSocket → Browser UI
```

### Stream Event Structure

Every event contains full ownership identifiers for correlation:

```go
type Event struct {
    // Ownership identifiers - who does this stream belong to?
    StationRunID  string    // Local DB ID (e.g., "123")
    RunUUID       string    // CloudShip's unique run identifier
    WorkflowRunID string    // Workflow run ID (when in workflow context)
    SessionID     string    // Session ID for workspace persistence
    AgentID       string    // Agent ID (e.g., "agent-1")
    AgentName     string    // Human-readable agent name
    StationID     string    // Station identifier
    
    // Event metadata
    Seq           int64     // Sequence number within the stream
    Timestamp     time.Time // Event timestamp
    Type          EventType // Event type
    Data          any       // Event payload
}
```

### Event Types

| Type | Data Structure | When Emitted |
|------|----------------|--------------|
| `run_start` | `RunStartData{AgentID, AgentName, Task, MaxSteps}` | Execution begins |
| `tool_start` | `ToolStartData{ToolName, ToolID, Input}` | Before tool execution |
| `tool_result` | `ToolResultData{ToolName, ToolID, Output, DurationMs, Error}` | After tool execution |
| `step_complete` | `StepCompleteData{StepNumber, TotalTokens, InputTokens, OutputTokens, FinishReason}` | After each LLM step |
| `run_complete` | `RunCompleteData{Success, TotalSteps, TotalTokens, DurationMs, FinishReason, Error}` | Execution ends |
| `token` | `TokenData{Content, Done}` | Streaming tokens (future) |
| `thinking` | `ThinkingData{Content}` | Extended thinking content (future) |
| `error` | `{error: string}` | Error occurred |

### Correlation Matrix

Different systems use different identifiers to correlate streams:

| System | Uses | Purpose |
|--------|------|---------|
| CloudShip Platform | `run_uuid` | Correlate with their runs table |
| Station Local DB | `station_run_id` | Correlate with agent_runs table |
| Workflows | `workflow_run_id` | Group steps in same workflow |
| Sessions | `session_id` | Resume work in same workspace |
| Lighthouse | `station_id` + `run_uuid` | Route to correct station |
| NATS Subject | `station_id` + `run_uuid` | Message routing |

### NATS Subject Pattern

```
station.{station_id}.run.{run_uuid}.stream
```

Subscription patterns:
- All streams from a station: `station.my-station.run.*.stream`
- Specific run: `station.my-station.run.abc-123.stream`
- All stations: `station.*.run.*.stream`

### Configuration

```yaml
harness:
  streaming:
    enabled: true
```

### Publisher Implementations

| Publisher | Use Case |
|-----------|----------|
| `ChannelPublisher` | Local/testing - publishes to Go channel |
| `NATSPublisher` | Production - publishes to NATS (with optional JetStream) |
| `MultiPublisher` | Combine multiple publishers |
| `NoOpPublisher` | Disabled streaming |

### Component Structure

```
pkg/harness/stream/
├── publisher.go       # Event types, StreamContext, Publisher interface
└── nats_publisher.go  # NATS/JetStream publisher implementation
```

### Usage in Executor

```go
// ExecuteOptions includes all stream identifiers
execOpts := harness.ExecuteOptions{
    StationRunID:  fmt.Sprintf("%d", agent.ID),
    RunUUID:       runUUID,
    WorkflowRunID: workflowRunID,
    SessionID:     workflowRunID,
    AgentName:     agent.Name,
    StationID:     stationID,
}

// Publisher is configured via option
executor := harness.NewAgenticExecutor(
    genkitApp,
    harnessConfig,
    agentHarnessConfig,
    harness.WithStreamPublisher(natsPublisher),
)

// Execute with streaming
result, err := executor.ExecuteWithOptions(ctx, agentID, task, tools, execOpts)
```

### Example Event Sequence

```json
{"station_run_id":"123","run_uuid":"abc-def","workflow_run_id":"wf-001","agent_id":"5","agent_name":"code-analyzer","station_id":"stn-xyz","seq":1,"type":"run_start","data":{"agent_id":"5","agent_name":"code-analyzer","task":"Analyze main.go","max_steps":50}}

{"station_run_id":"123","run_uuid":"abc-def","workflow_run_id":"wf-001","agent_id":"5","agent_name":"code-analyzer","station_id":"stn-xyz","seq":2,"type":"tool_start","data":{"tool_name":"read","tool_id":"read-123","input":{"path":"main.go"}}}

{"station_run_id":"123","run_uuid":"abc-def","workflow_run_id":"wf-001","agent_id":"5","agent_name":"code-analyzer","station_id":"stn-xyz","seq":3,"type":"tool_result","data":{"tool_name":"read","tool_id":"read-123","output":"package main...","duration_ms":5}}

{"station_run_id":"123","run_uuid":"abc-def","workflow_run_id":"wf-001","agent_id":"5","agent_name":"code-analyzer","station_id":"stn-xyz","seq":4,"type":"step_complete","data":{"step_number":1,"total_tokens":1500,"input_tokens":1000,"output_tokens":500,"finish_reason":"tool_use"}}

{"station_run_id":"123","run_uuid":"abc-def","workflow_run_id":"wf-001","agent_id":"5","agent_name":"code-analyzer","station_id":"stn-xyz","seq":5,"type":"run_complete","data":{"success":true,"total_steps":3,"total_tokens":4500,"duration_ms":12000,"finish_reason":"agent_done"}}
```

### Future Work

- [ ] Lighthouse StreamConsumer to forward events to CloudShip
- [ ] Token-by-token streaming for real-time UI updates
- [ ] Extended thinking content streaming
- [ ] WebSocket adapter for direct browser connections
- [ ] Stream replay from NATS JetStream

## Phase 3: Sandbox Isolation Strategies (COMPLETE)

### Problem Statement

The current harness executes code directly on the host system. This presents security risks:

1. **Untrusted code execution** - Agent-generated bash commands run with user privileges
2. **File system access** - Agents can read/write anywhere the user has access
3. **Network access** - Agents can make arbitrary network requests
4. **Resource exhaustion** - No limits on CPU, memory, or disk usage
5. **Persistence** - Changes persist on host even after errors

### Sandbox Strategies

| Strategy | Isolation Level | Startup Time | Use Case | Status |
|----------|-----------------|--------------|----------|--------|
| **Host** | None | Instant | Development, trusted agents | ✅ Complete |
| **Docker** | Container | ~1-2s | Production, untrusted code | ✅ Complete |
| **E2B** | Cloud sandbox | ~2-3s | Serverless, managed infra | ✅ Complete |
| **Firecracker** | microVM | ~200ms | High security, multi-tenant | Planned |
| **gVisor** | Kernel sandbox | ~500ms | Balance of security/performance | Planned |
| **WASM** | Process sandbox | ~50ms | Lightweight, fast iteration | Planned |

### Proposed Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    SANDBOX CONFIGURATION                          │
│  (per-agent or global config)                                    │
├─────────────────────────────────────────────────────────────────┤
│  sandbox:                                                        │
│    mode: docker | firecracker | gvisor | wasm | host             │
│    image: station-sandbox:latest                                 │
│    resources:                                                    │
│      cpu: 2                                                      │
│      memory: 4Gi                                                 │
│      disk: 10Gi                                                  │
│    network:                                                      │
│      enabled: false                                              │
│      allowed_hosts: [github.com, api.openai.com]                 │
│    filesystem:                                                   │
│      read_only: [/etc, /usr]                                     │
│      read_write: [/workspace]                                    │
│      denied: [/etc/passwd, ~/.ssh]                               │
│    timeout: 30m                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    SANDBOX MANAGER                                │
│                                                                  │
│  Interface:                                                       │
│  - Create(config) → SandboxInstance                              │
│  - Execute(instance, command) → Result                           │
│  - ReadFile(instance, path) → Content                            │
│  - WriteFile(instance, path, content) → Error                    │
│  - Destroy(instance) → Error                                     │
│                                                                  │
│  Implementations:                                                 │
│  - HostSandbox (passthrough, no isolation)                       │
│  - DockerSandbox (container-based)                               │
│  - FirecrackerSandbox (microVM-based)                            │
│  - WASMSandbox (WebAssembly-based)                               │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Plan

| Phase | Description | Priority | Status |
|-------|-------------|----------|--------|
| 3.1 | Define `Sandbox` interface and `SandboxConfig` | High | ✅ Complete |
| 3.2 | Implement `HostSandbox` (current behavior, passthrough) | High | ✅ Complete |
| 3.3 | Implement `DockerSandbox` with resource limits | High | ✅ Complete |
| 3.4 | Implement `E2BSandbox` with cloud execution | High | ✅ Complete |
| 3.5 | Add `StreamingSandbox` interface with real-time output | High | ✅ Complete |
| 3.6 | Implement `RuntimeResolver` for auto-detection | High | ✅ Complete |
| 3.7 | Wire sandbox into `AgenticExecutor` lifecycle | High | ✅ Complete |
| 3.8 | Add network filtering (allowed hosts list) | Medium | Pending |
| 3.9 | Add filesystem ACLs (read-only, read-write, denied paths) | Medium | Pending |
| 3.10 | Implement `FirecrackerSandbox` for high-security use cases | Low | Pending |
| 3.11 | Implement `WASMSandbox` for lightweight isolation | Low | Pending |
| 3.12 | Add sandbox metrics (CPU, memory, I/O usage) | Medium | Pending |
| 3.13 | Add sandbox cleanup/garbage collection | Medium | Pending |
| 3.14 | E2E tests for each sandbox mode | High | ✅ Complete (Host, Docker, E2B) |

### Docker Sandbox Design

```go
type DockerSandbox struct {
    containerID string
    config      SandboxConfig
    client      *docker.Client
}

// Creates container with:
// - Read-only root filesystem
// - Mounted workspace volume
// - Resource limits (CPU, memory)
// - No network (unless explicitly allowed)
// - Dropped capabilities
func (s *DockerSandbox) Create(ctx context.Context) error

// Executes command via docker exec
func (s *DockerSandbox) Execute(ctx context.Context, cmd string) (string, error)

// Copies file from container
func (s *DockerSandbox) ReadFile(ctx context.Context, path string) ([]byte, error)

// Copies file into container
func (s *DockerSandbox) WriteFile(ctx context.Context, path string, content []byte) error
```

### Configuration Examples

**Development (no isolation):**
```yaml
harness:
  sandbox:
    mode: host
```

**Production (Docker isolation):**
```yaml
harness:
  sandbox:
    mode: docker
    image: station-sandbox:latest
    resources:
      cpu: 2
      memory: 4Gi
    network:
      enabled: false
    timeout: 30m
```

**High-security (Firecracker microVM):**
```yaml
harness:
  sandbox:
    mode: firecracker
    kernel: /var/lib/firecracker/vmlinux
    rootfs: /var/lib/firecracker/rootfs.ext4
    resources:
      vcpu: 2
      memory: 2048
    network:
      enabled: true
      allowed_hosts:
        - "*.github.com"
        - "api.openai.com"
```

### Success Criteria (Phase 3)

- [x] **Host mode**: Current behavior preserved, zero overhead
- [x] **Docker mode**: Agents execute in isolated containers
- [x] **E2B mode**: Agents execute in cloud sandboxes with streaming
- [x] **Streaming support**: Real-time stdout/stderr via `StreamingSandbox` interface
- [x] **RuntimeResolver**: Auto-detects host capabilities, falls back to configured isolation
- [x] **Executor integration**: Sandbox initialized in setup(), destroyed in cleanup()
- [x] **Resource limits**: CPU/memory/disk limits enforced (Docker, E2B)
- [ ] **Network filtering**: Only allowed hosts reachable (future)
- [ ] **Filesystem ACLs**: Denied paths inaccessible (future)
- [x] **Cleanup**: Sandbox destroyed on completion/timeout
- [ ] **Metrics**: Resource usage tracked per execution (future)
- [x] **E2E tests**: Host, Docker, E2B sandbox modes tested (25 tests pass)

### Component Structure (Phase 3)

```
pkg/harness/sandbox/
├── sandbox.go            # Sandbox interface, Config, Factory, RuntimeResolver
├── host.go               # HostSandbox - passthrough execution
├── docker.go             # DockerSandbox - container isolation
├── e2b.go                # E2BSandbox - cloud sandbox with streaming
├── sandbox_test.go       # Unit tests (13 tests)
├── e2b_test.go           # E2B unit tests (12 tests)
└── e2b/
    ├── client.go         # REST client for E2B API
    ├── streaming.go      # gRPC/Connect streaming client
    └── proto/
        ├── process.proto # E2B envd process service definition
        └── gen/          # Generated protobuf and Connect code
```

### Key Interfaces

```go
// Sandbox provides isolated execution environment
type Sandbox interface {
    ID() string
    Exec(ctx context.Context, command string, args ...string) (string, error)
    ReadFile(ctx context.Context, path string) ([]byte, error)
    WriteFile(ctx context.Context, path string, content []byte) error
    Destroy(ctx context.Context) error
}

// StreamingSandbox adds real-time output streaming
type StreamingSandbox interface {
    Sandbox
    ExecStream(ctx context.Context, command string, args []string, opts ExecOptions) (ProcessHandle, error)
    ListProcesses(ctx context.Context) ([]ProcessInfo, error)
    KillProcess(ctx context.Context, pid int) error
}

// RuntimeResolver auto-detects sandbox mode based on host capabilities
type RuntimeResolver struct {
    FallbackBackend IsolationBackend // docker | e2b
}

func (r *RuntimeResolver) Resolve(cfg Config, reqs RuntimeRequirements) (Config, error)
```

### Runtime Resolution Flow

```
┌─────────────────────────────────────────────────────────────┐
│                    User Configuration                        │
│  isolation_backend: docker | e2b    ← User's explicit choice │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    RuntimeResolver                           │
│  if cfg.Mode == ModeAuto:                                    │
│    if host has required commands → ModeHost                  │
│    else → use configured isolation_backend                   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Factory.Create(cfg)                       │
│  ModeHost → HostSandbox (passthrough)                        │
│  ModeDocker → DockerSandbox (container)                      │
│  ModeE2B → E2BSandbox (cloud)                               │
└─────────────────────────────────────────────────────────────┘
```

**Key principle:** Same agent config works everywhere. Station adapts based on environment.

## Phase 4: Session Persistence & REPL Support (COMPLETE)

### Problem Statement

The harness executes agents statelessly - each execution starts fresh with no memory of previous interactions. For REPL-style workflows, users need:

1. **Message history persistence** - Continue conversations across multiple inputs
2. **Sandbox persistence** - Keep the same files/environment between interactions
3. **Session lifecycle management** - Start, pause, resume, and end sessions

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│  REPL Session Flow:                                              │
├─────────────────────────────────────────────────────────────────┤
│  1. StartSession(sessionID)                                      │
│     ├── GetOrCreate session workspace                            │
│     ├── AcquireLock (prevent concurrent access)                  │
│     ├── Initialize workspace directory                           │
│     └── Create sandbox (keep alive for session duration)         │
├─────────────────────────────────────────────────────────────────┤
│  2. Execute(task) - repeat for each user input                   │
│     ├── Load history from .history.json                          │
│     ├── Create AgenticExecutor with loaded history               │
│     ├── Run task                                                 │
│     ├── Append new messages to history                           │
│     └── Save history to disk                                     │
├─────────────────────────────────────────────────────────────────┤
│  3. EndSession()                                                 │
│     ├── Destroy sandbox (or keep for future resume)              │
│     └── ReleaseLock                                              │
└─────────────────────────────────────────────────────────────────┘
```

### Session Storage

Files stored per session:
```
~/.config/station/workspace/session/{session-id}/
├── .session.meta    # Session metadata (created_at, total_runs)
├── .session.lock    # Lock file (PID, expires_at)
├── .history.json    # Message history (all user/assistant messages)
└── (workspace files created by agent)
```

### Key Components

#### HistoryStore (`pkg/harness/session/history_store.go`)

Persists message history for sessions enabling REPL-style conversations:

```go
type HistoryStore struct {
    basePath string
    mu       sync.RWMutex
}

type StoredMessage struct {
    Role      string                 `json:"role"`
    Content   string                 `json:"content"`
    ToolCalls []StoredToolCall       `json:"tool_calls,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
    Timestamp time.Time              `json:"timestamp"`
}

type SessionHistory struct {
    SessionID   string          `json:"session_id"`
    Messages    []StoredMessage `json:"messages"`
    TotalTokens int             `json:"total_tokens"`
    CreatedAt   time.Time       `json:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

// Load loads message history for a session
func (h *HistoryStore) Load(sessionID string) (*SessionHistory, error)

// Save persists message history for a session
func (h *HistoryStore) Save(sessionID string, history *SessionHistory) error

// Append adds messages to a session's history
func (h *HistoryStore) Append(sessionID string, messages []StoredMessage) error

// ToAIMessages converts stored messages to ai.Message format
func (h *SessionHistory) ToAIMessages() []*ai.Message
```

#### SessionExecutor (`pkg/harness/session/session_executor.go`)

Wraps AgenticExecutor with session persistence:

```go
type SessionExecutor struct {
    genkitApp      *genkit.Genkit
    sessionManager *Manager
    historyStore   *HistoryStore
    // ... config fields

    // Active session state
    currentSessionID string
    currentSession   *Session
    currentWorkspace harness.WorkspaceManager
    currentSandbox   sandbox.Sandbox
}

// StartSession starts or resumes a session
func (se *SessionExecutor) StartSession(ctx context.Context, sessionID string, repoURL string, branch string) error

// Execute runs a task within the current session, maintaining history
func (se *SessionExecutor) Execute(ctx context.Context, task string) (*harness.ExecutionResult, error)

// EndSession ends the current session, cleaning up resources
func (se *SessionExecutor) EndSession(ctx context.Context) error
```

#### AgenticExecutor History Support

The AgenticExecutor now supports initial history injection:

```go
// WithInitialHistory sets the initial message history for session persistence
func WithInitialHistory(history []*ai.Message) ExecutorOption

// ExecutionResult now includes history for persistence
type ExecutionResult struct {
    // ... existing fields
    History []*ai.Message `json:"-"` // Full message history for session persistence
}
```

### Usage Example

```go
// Create session executor
executor := session.NewSessionExecutor(genkitApp, session.SessionExecutorConfig{
    BasePath:     "~/.config/station/workspace",
    ModelName:    "openai/gpt-4o-mini",
    SystemPrompt: "You are a helpful coding assistant.",
})

// Start session
if err := executor.StartSession(ctx, "my-coding-session", "", ""); err != nil {
    log.Fatal(err)
}
defer executor.EndSession(ctx)

// First interaction
result1, _ := executor.Execute(ctx, "Create a hello.py file that prints 'Hello World'")

// Second interaction - agent has full context of previous work
result2, _ := executor.Execute(ctx, "Now add a function that takes a name parameter")

// Third interaction - continues with full history
result3, _ := executor.Execute(ctx, "Add tests for the function")
```

### CLI Integration (Future)

```bash
# Start REPL session
stn repl --session "my-project" --agent "coder"

# Inside REPL:
> Create a REST API with user CRUD operations
# Agent creates files, you see the output
> Add authentication middleware
# Agent continues, aware of previous work
> exit

# Later, resume the session
stn repl --session "my-project" --agent "coder"
# Agent has full context of all previous work
```

### Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| 4.1 | Define `StoredMessage` and `SessionHistory` types | ✅ Complete |
| 4.2 | Implement `HistoryStore` for JSON persistence | ✅ Complete |
| 4.3 | Add `WithInitialHistory` option to `AgenticExecutor` | ✅ Complete |
| 4.4 | Return `History` in `ExecutionResult` | ✅ Complete |
| 4.5 | Implement `SessionExecutor` wrapper | ✅ Complete |
| 4.6 | Add session persistence tests | ✅ Complete |
| 4.7 | Add CLI REPL command (`stn repl`) | Pending |
| 4.8 | Add session list/delete CLI commands | Pending |

### Success Criteria (Phase 4)

- [x] **HistoryStore**: Can save/load/append message history to disk
- [x] **SessionExecutor**: Wraps AgenticExecutor with session lifecycle
- [x] **Initial history injection**: `WithInitialHistory` option works
- [x] **History in result**: ExecutionResult includes full history
- [x] **Unit tests**: All session persistence tests pass
- [ ] **CLI REPL**: `stn repl` command for interactive sessions (future)
- [ ] **E2E test**: Multi-turn conversation with real LLM (future)

### Component Structure (Phase 4)

```
pkg/harness/session/
├── manager.go              # Session lifecycle management
├── history_store.go        # Message history persistence (NEW)
├── session_executor.go     # Session-aware executor wrapper (NEW)
└── session_executor_test.go # Session persistence tests (NEW)
```

### Test Results (2025-01-26)

All session tests pass:

| Test | Description | Status |
|------|-------------|--------|
| `TestHistoryStore_SaveLoad` | Save and load history | ✅ PASS |
| `TestHistoryStore_Append` | Append messages across calls | ✅ PASS |
| `TestHistoryStore_Clear` | Clear history | ✅ PASS |
| `TestSessionManager_Integration` | Full session lifecycle | ✅ PASS |

Run tests:
```bash
go test ./pkg/harness/session/... -v -count=1
```

---

## Phase 7: Developer Experience (DX) Improvements

### Problem Statement

The current harness has powerful capabilities but poor developer ergonomics:

1. **No dedicated CLI commands** - Users must know `--harness-config` JSON
2. **No scaffolding** - No templates for common agent patterns
3. **No REPL mode** - Can't develop agents interactively
4. **Limited debugging** - Hard to inspect runs and tool calls
5. **Confusing entrypoints** - Multiple ways to trigger, unclear when to use each

### Proposed CLI Commands

#### 1. `stn harness init` - Scaffold New Harness Agent

```bash
# Interactive mode (guided prompts)
stn harness init

# One-liner with options
stn harness init code-reviewer \
  --template coding \
  --sandbox docker \
  --image python:3.11 \
  --max-steps 50

# Templates available:
# - coding: read, write, edit, bash, glob, grep
# - sre: read, bash, grep + prometheus tools
# - security: read, glob, grep + security scanning
# - data: read, write, bash + pandas/sql
```

**Output:**
- Creates `.prompt` file with proper frontmatter
- Syncs environment automatically
- Prints usage instructions

#### 2. `stn harness run` - Execute Harness Agent

```bash
# Basic execution
stn harness run code-reviewer "Review auth module"

# With workspace (clones repo)
stn harness run code-reviewer "Fix tests" --repo https://github.com/org/repo

# With session persistence
stn harness run code-reviewer "Continue refactoring" --session my-session

# With streaming output
stn harness run code-reviewer "Analyze code" --stream

# With variables
stn harness run scanner "Scan {{path}}" --var path=./src --var depth=3
```

#### 3. `stn harness repl` - Interactive Development Mode

```bash
stn harness repl code-reviewer

╭──────────────────────────────────────────────────╮
│ Harness REPL: code-reviewer                      │
│ Session: ses_abc123 | Steps: 0/50 | Tokens: 0    │
│ Type /help for commands, Ctrl+C to exit          │
╰──────────────────────────────────────────────────╯

[code-reviewer] > Read the main.py file

[Step 1/50] Executing: read
✓ read main.py (245 lines)

[code-reviewer] > Find security issues

[Step 2/50] Executing: grep
✓ grep "password|secret|key" (3 matches)

[Step 3/50] Reasoning...
I found 3 potential security issues:
1. Hardcoded password on line 42
...

[code-reviewer] > /save
✓ Session saved: ses_abc123

[code-reviewer] > /export ./report.md
✓ Exported conversation to ./report.md

[code-reviewer] > /exit
```

**REPL Commands:**
| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/tools` | List available tools |
| `/history` | Show conversation history |
| `/save` | Save session checkpoint |
| `/load <id>` | Load previous session |
| `/export <path>` | Export conversation |
| `/clear` | Clear current context |
| `/exit` | Exit REPL |

#### 4. `stn harness inspect` - Debug/Inspect Runs

```bash
# List recent harness runs
stn harness runs
ID          AGENT           STATUS    STEPS   TOKENS  DURATION
run_abc123  code-reviewer   success   12      4,521   2m34s
run_def456  code-reviewer   error     5       1,203   45s

# Inspect a run
stn harness inspect run_abc123

╭─ Run: run_abc123 ─────────────────────────────────╮
│ Agent:    code-reviewer                           │
│ Status:   success                                 │
│ Steps:    12/50                                   │
│ Tokens:   4,521 (in: 1,200 out: 3,321)           │
│ Duration: 2m34s                                   │
│ Session:  ses_xyz789                              │
│ Sandbox:  docker (python:3.11)                    │
╰───────────────────────────────────────────────────╯

Tool Calls:
  1. [0.2s] read main.py → 245 lines
  2. [0.1s] grep "TODO|FIXME" → 3 matches
  3. [0.3s] edit main.py → 1 replacement
  4. [0.8s] bash pytest → exit 0
  ...

# Stream logs in real-time
stn harness logs run_abc123 --follow

# Export run data
stn harness export run_abc123 --format json > run.json
```

### Implementation Plan

| Task | Priority | Estimate |
|------|----------|----------|
| `stn harness init` with templates | High | 1 day |
| `stn harness run` with flags | High | 1 day |
| `stn harness inspect` / `runs` | High | 0.5 day |
| `stn harness repl` interactive mode | Medium | 2 days |
| Templates (coding, sre, security) | Medium | 1 day |

---

## Phase 8: Entrypoints, Triggers & Artifacts

### Overview

Harness agents need clear entrypoints (how they're triggered) and exitpoints (how results get out). This phase defines the complete lifecycle.

### Entrypoints (Triggers)

```
┌─────────────────────────────────────────────────────────────────┐
│                        ENTRYPOINTS                               │
│                 (How harness agents get triggered)               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │     CLI     │  │   MCP/API   │  │   Workflow  │              │
│  │  One-shot   │  │   Request   │  │    Step     │              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         │                │                │                      │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌──────┴──────┐              │
│  │    Cron     │  │   Webhook   │  │   Event     │              │
│  │  Scheduled  │  │   HTTP      │  │  NATS/PubSub│              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         │                │                │                      │
│         └────────────────┴────────────────┘                      │
│                          │                                       │
│                          ▼                                       │
│              ┌───────────────────────┐                          │
│              │   HarnessEntrypoint   │                          │
│              │   (unified handler)    │                          │
│              └───────────────────────┘                          │
└─────────────────────────────────────────────────────────────────┘
```

#### 1. CLI Entrypoint (Interactive)

```bash
# One-shot execution
stn harness run <agent> "<task>"

# Interactive REPL
stn harness repl <agent>

# Resume session
stn harness run <agent> --session <session-id>
```

#### 2. MCP/API Entrypoint (Programmatic)

```go
// MCP tool: call_harness_agent
{
  "agent": "code-reviewer",
  "task": "Review PR #123",
  "session_id": "pr-123-review",  // Optional
  "workspace": {
    "repo": "https://github.com/org/repo",
    "ref": "pull/123/head"
  },
  "timeout": "30m"
}

// HTTP API: POST /api/v1/harness/run
{
  "agent_id": 42,
  "task": "...",
  "session_id": "...",
  "variables": {...}
}
```

#### 3. Cron Entrypoint (Scheduled)

```bash
# Set up scheduled harness agent
stn harness schedule code-scanner \
  --cron "0 2 * * *" \
  --task "Scan codebase for security issues" \
  --session "nightly-scan" \
  --repo https://github.com/org/repo

# Cron config in agent
---
name: code-scanner
harness: agentic
schedule:
  cron: "0 2 * * *"
  task: "Scan codebase for security issues"
  session: "nightly-scan"
  workspace:
    repo: "${REPO_URL}"
---
```

#### 4. Webhook Entrypoint (Event-Driven)

```bash
# Register webhook trigger
stn harness webhook create code-reviewer \
  --path /hooks/pr-review \
  --secret "${WEBHOOK_SECRET}"

# Incoming webhook payload:
POST /hooks/pr-review
{
  "action": "opened",
  "pull_request": {
    "number": 123,
    "head": {"ref": "feature-branch"}
  },
  "repository": {
    "clone_url": "https://github.com/org/repo"
  }
}

# Webhook config in agent
---
name: pr-reviewer
harness: agentic
webhook:
  path: /hooks/pr-review
  events: ["pull_request.opened", "pull_request.synchronize"]
  task_template: "Review PR #{{.pull_request.number}}"
  workspace:
    repo: "{{.repository.clone_url}}"
    ref: "{{.pull_request.head.ref}}"
---
```

#### 5. Event Entrypoint (NATS/PubSub)

```yaml
# Agent subscribes to events
---
name: incident-responder
harness: agentic
subscribe:
  - subject: "alerts.pagerduty.triggered"
    task_template: "Investigate alert: {{.incident.title}}"
  - subject: "monitoring.anomaly.detected"
    task_template: "Analyze anomaly in {{.service}}"
---

# Event published
{
  "subject": "alerts.pagerduty.triggered",
  "data": {
    "incident": {
      "id": "INC-123",
      "title": "High CPU on prod-web-01"
    }
  }
}
```

#### 6. Workflow Entrypoint (Pipeline Step)

```yaml
id: incident-response-pipeline
states:
  - name: investigate
    type: harness_agent        # Dedicated harness state type
    agent: incident-investigator
    harness:
      max_steps: 100
      timeout: 1h
      sandbox:
        mode: docker
        network: true          # Need network for kubectl
    input:
      incident_id: "{{ctx.incident_id}}"
    resultPath: investigation
    next: remediate
```

### Exitpoints (Results & Artifacts)

```
┌─────────────────────────────────────────────────────────────────┐
│                        EXITPOINTS                                │
│                (How results get out of harness)                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│              ┌───────────────────────┐                          │
│              │   HarnessExitpoint    │                          │
│              │   (result handler)    │                          │
│              └───────────┬───────────┘                          │
│                          │                                       │
│     ┌────────────────────┼────────────────────┐                 │
│     │                    │                    │                 │
│     ▼                    ▼                    ▼                 │
│  ┌──────────┐     ┌──────────┐       ┌──────────┐              │
│  │ Response │     │ Artifact │       │  Event   │              │
│  │  (sync)  │     │  Upload  │       │  Publish │              │
│  └──────────┘     └──────────┘       └──────────┘              │
│                                                                  │
│  - Final text     - Files (workspace)  - Completion event       │
│  - Structured     - Logs               - Webhook callback       │
│    output         - Screenshots        - CloudShip sync         │
│                   - Reports                                      │
└─────────────────────────────────────────────────────────────────┘
```

### Artifact Handling

#### Current State

| Mode | Artifact Storage | Persistence | Cross-Step Access |
|------|------------------|-------------|-------------------|
| Host | Local filesystem | Session dir | Yes (same session) |
| Docker | Docker volumes | Named volume | Yes (volume mount) |
| E2B | E2B filesystem | Sandbox lifetime | No (ephemeral) |

#### Artifact Types

```go
type ArtifactType string

const (
    ArtifactTypeFile       ArtifactType = "file"        // Created/modified files
    ArtifactTypeLog        ArtifactType = "log"         // Execution logs
    ArtifactTypeReport     ArtifactType = "report"      // Generated reports
    ArtifactTypeScreenshot ArtifactType = "screenshot"  // Browser captures
    ArtifactTypeData       ArtifactType = "data"        // Structured data output
)

type Artifact struct {
    ID          string       `json:"id"`
    Type        ArtifactType `json:"type"`
    Name        string       `json:"name"`
    Path        string       `json:"path"`        // Local path in workspace
    Size        int64        `json:"size"`
    ContentType string       `json:"content_type"`
    CreatedAt   time.Time    `json:"created_at"`

    // Storage location
    StorageKey  string       `json:"storage_key"` // NATS Object Store key
    TTL         time.Duration `json:"ttl"`

    // Lineage
    SessionID   string       `json:"session_id"`
    RunID       string       `json:"run_id"`
    StepID      string       `json:"step_id,omitempty"`
}
```

#### Artifact Lifecycle

```
┌─────────────────────────────────────────────────────────────────┐
│                    ARTIFACT LIFECYCLE                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. CREATION (during execution)                                  │
│     ├─ write tool → local file in workspace                     │
│     ├─ bash tool → stdout/stderr captured                       │
│     └─ explicit artifact_save tool → designated artifact        │
│                                                                  │
│  2. CAPTURE (post-execution)                                     │
│     ├─ Scan workspace for new/modified files                    │
│     ├─ Filter by .harnessignore patterns                        │
│     └─ Create Artifact records with metadata                    │
│                                                                  │
│  3. UPLOAD (to storage)                                          │
│     ├─ Local: Keep in workspace, index in SQLite                │
│     ├─ NATS: Upload to Object Store with TTL                    │
│     └─ CloudShip: Sync to management channel                    │
│                                                                  │
│  4. ACCESS (retrieval)                                           │
│     ├─ Same session: Direct filesystem access                   │
│     ├─ Next workflow step: Download from Object Store           │
│     └─ CloudShip UI: Fetch via management channel               │
│                                                                  │
│  5. CLEANUP (TTL expiration)                                     │
│     ├─ Object Store: Auto-expire after TTL (default 24h)        │
│     ├─ Workspace: Optional cleanup on session end               │
│     └─ CloudShip: Retention policy per organization             │
└─────────────────────────────────────────────────────────────────┘
```

#### Artifact Configuration

```yaml
harness:
  artifacts:
    # Auto-capture settings
    capture:
      enabled: true
      include:
        - "**/*.py"
        - "**/*.md"
        - "**/*.json"
        - "**/*.log"
      exclude:
        - "node_modules/**"
        - ".git/**"
        - "*.tmp"
      max_file_size: 10MB

    # Storage settings
    storage:
      mode: nats              # local | nats | cloudship
      ttl: 24h                # Time-to-live for stored artifacts
      max_total_size: 100MB   # Per-run artifact limit

    # Sync to CloudShip
    cloudship:
      sync_enabled: true      # Push artifacts to management channel
      sync_reports: true      # Sync generated reports
      sync_logs: false        # Don't sync verbose logs
```

### Session Isolation

```
┌─────────────────────────────────────────────────────────────────┐
│                    SESSION ISOLATION                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  SESSION: ses_abc123                                             │
│  ├── workspace/                                                  │
│  │   ├── src/                   # Cloned repo or working files  │
│  │   │   ├── main.py                                            │
│  │   │   └── utils.py                                           │
│  │   └── .harness/              # Harness metadata               │
│  │       ├── tasks.json         # Task tracking                  │
│  │       ├── history.json       # Conversation history           │
│  │       └── artifacts.json     # Artifact index                 │
│  │                                                               │
│  ├── runs/                      # Run-specific data              │
│  │   ├── run_001/                                                │
│  │   │   ├── logs/                                               │
│  │   │   └── output/                                             │
│  │   └── run_002/                                                │
│  │       ├── logs/                                               │
│  │       └── output/                                             │
│  │                                                               │
│  └── .session.meta              # Session metadata               │
│                                                                  │
│  ISOLATION GUARANTEES:                                           │
│  • Each session has dedicated workspace                          │
│  • Concurrent runs on same session are serialized (lock)         │
│  • Different sessions are fully isolated                         │
│  • Sandbox mode adds OS-level isolation (Docker/E2B)             │
└─────────────────────────────────────────────────────────────────┘
```

### CloudShip Integration

When connected to CloudShip via management channel:

```
┌─────────────────────────────────────────────────────────────────┐
│                    CLOUDSHIP INTEGRATION                         │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  STATION (Customer Infrastructure)                               │
│  ├── Receives WorkAssignment via NATS                           │
│  │   {                                                           │
│  │     "work_id": "work_123",                                   │
│  │     "orchestrator_run_id": "uuid-from-cloudship",            │
│  │     "agent_id": "42",                                        │
│  │     "task": "Review code",                                   │
│  │     "workspace": { "repo": "...", "ref": "..." }             │
│  │   }                                                           │
│  │                                                               │
│  ├── Creates session: ses_{work_id}                             │
│  ├── Clones repo into session workspace                         │
│  ├── Executes harness agent                                      │
│  │                                                               │
│  ├── Artifacts uploaded to NATS Object Store                    │
│  │   Key: runs/{orchestrator_run_id}/output/{filename}          │
│  │                                                               │
│  └── Sends WorkResponse via NATS                                │
│      {                                                           │
│        "work_id": "work_123",                                   │
│        "status": "completed",                                   │
│        "result": "Found 3 issues...",                           │
│        "artifacts": [                                            │
│          { "key": "runs/.../report.md", "size": 1234 }          │
│        ],                                                        │
│        "duration_ms": 45000,                                    │
│        "tokens": 4521                                           │
│      }                                                           │
│                                                                  │
│  CLOUDSHIP (Management Plane)                                    │
│  ├── Tracks run via orchestrator_run_id                         │
│  ├── Can poll for artifacts from Object Store                   │
│  ├── Displays results in UI                                     │
│  └── Applies retention policies                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Plan

| Task | Priority | Estimate | Phase |
|------|----------|----------|-------|
| `stn harness init` command | High | 1 day | 7 |
| `stn harness run` command | High | 1 day | 7 |
| `stn harness inspect` command | High | 0.5 day | 7 |
| Harness agent templates | High | 1 day | 7 |
| `stn harness repl` command | Medium | 2 days | 7 |
| Webhook entrypoint | Medium | 1 day | 8 |
| Event subscription | Medium | 1 day | 8 |
| Artifact capture system | High | 1 day | 8 |
| Artifact sync to CloudShip | Medium | 1 day | 8 |
| `.harnessignore` support | Low | 0.5 day | 8 |

---

## References

- `internal/services/agent_execution_engine.go` - Integration point
- `pkg/harness/` - Harness implementation
- `pkg/harness/session/` - Session management
- `pkg/harness/nats/` - NATS integration and artifact storage
- `internal/workflows/runtime/` - Workflow execution system
- `internal/lattice/work/` - Existing NATS/state infrastructure
- `internal/services/sandbox_session_manager.go` - Existing session pattern
- `internal/cloudship/client.go` - CloudShip integration
