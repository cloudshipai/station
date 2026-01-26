# Harness Architecture Analysis

**Date**: 2025-01-07  
**Purpose**: Understand how sandbox integration fits into the broader harness ecosystem

## Executive Summary

The sandbox work in `pkg/harness/sandbox/` is **architecturally isolated** and **not wired into the actual execution path**. While we built comprehensive sandbox interfaces (Host, Docker, E2B), the core execution engine (`executeWithAgenticHarness`) doesn't use them.

### Critical Gaps Identified

| Gap | Impact | Priority |
|-----|--------|----------|
| `executeWithAgenticHarness` doesn't use sandbox | Tools execute directly on host | **HIGH** |
| ToolRegistry created without sandbox | All tools bypass sandbox isolation | **HIGH** |
| No sandbox config in harness frontmatter | Can't enable per-agent | **MEDIUM** |
| Session manager exists but unused | Workspace isolation incomplete | **MEDIUM** |
| Existing `SandboxService` in `internal/services/` | Potential duplication | **LOW** |

---

## Current Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         STATION ARCHITECTURE                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Entry Points:                                                               │
│  ├── CLI: stn call <agent>                                                   │
│  ├── API: /api/agents/:id/execute                                            │
│  ├── Workflow Engine: internal/workflows/runtime/                            │
│  └── Lattice: ExecutorAdapter (remote execution)                             │
│                                                                              │
│                              │                                               │
│                              ▼                                               │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │              AgentExecutionEngine.ExecuteWithOptions()                 │  │
│  │                                                                        │  │
│  │  1. Load harness frontmatter from .prompt file                         │  │
│  │  2. Check: if harnessMode == "agentic" → executeWithAgenticHarness()  │  │
│  │  3. Otherwise → GenKitExecutor.ExecuteAgent() (dotprompt default)     │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                              │                                               │
│              ┌───────────────┴───────────────┐                               │
│              ▼                               ▼                               │
│  ┌─────────────────────────┐    ┌─────────────────────────┐                 │
│  │   Agentic Harness       │    │   Dotprompt (Default)   │                 │
│  │   (pkg/harness/)        │    │   (pkg/dotprompt/)      │                 │
│  │                         │    │                         │                 │
│  │   • Manual loop control │    │   • Genkit's default    │                 │
│  │   • Doom loop detection │    │   • Simple execution    │                 │
│  │   • Context compaction  │    │   • No special features │                 │
│  │   • Git integration     │    │                         │                 │
│  │   • Streaming events    │    │                         │                 │
│  └─────────────────────────┘    └─────────────────────────┘                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## The Problem: Sandbox Not Wired

### Current `executeWithAgenticHarness()` (lines 1391-1512)

```go
func (aee *AgentExecutionEngine) executeWithAgenticHarness(...) {
    // 1. Load config
    harnessConfig := convertConfigToHarnessConfig(loadedConfig.Harness)
    
    // 2. Create workspace (host only!)
    ws := workspace.NewHostWorkspace(workspacePath)  // ❌ No sandbox option
    
    // 3. Create tool registry WITHOUT sandbox
    toolRegistry := tools.NewToolRegistry(genkitApp, workspacePath)  // ❌ Missing sandbox!
    toolRegistry.RegisterBuiltinTools()
    
    // 4. Create executor (has sandbox fields but never used)
    executorOpts := []harness.ExecutorOption{
        harness.WithWorkspace(ws),
        harness.WithModelName(modelName),
        // ❌ NO WithSandbox() or WithSandboxConfig()!
    }
    
    agenticExecutor := harness.NewAgenticExecutor(...)
    // agenticExecutor.sandbox is nil
    // Tools execute directly on host
}
```

### What Should Happen

```go
func (aee *AgentExecutionEngine) executeWithAgenticHarness(...) {
    // 1. Parse sandbox config from frontmatter
    sandboxConfig := parseSandboxConfig(harnessFM)  // NEW
    
    // 2. Create RuntimeResolver
    resolver := sandbox.NewRuntimeResolver(sandbox.IsolationDocker)  // or E2B
    
    // 3. Create sandbox based on config
    sb, err := createSandbox(ctx, sandboxConfig, resolver)  // NEW
    
    // 4. Create tool registry WITH sandbox
    toolRegistry := tools.NewToolRegistryWithSandbox(genkitApp, workspacePath, sb)  // ✅
    
    // 5. Create executor WITH sandbox
    executorOpts := []harness.ExecutorOption{
        harness.WithSandbox(sb),                    // ✅
        harness.WithSandboxConfig(sandboxConfig),   // ✅
        harness.WithRuntimeResolver(resolver),      // ✅
    }
}
```

---

## Component Inventory

### What We Built (pkg/harness/)

| Component | Location | Status | Purpose |
|-----------|----------|--------|---------|
| **Sandbox Interface** | `sandbox/sandbox.go` | ✅ Complete | Abstract sandbox interface |
| **HostSandbox** | `sandbox/host.go` | ✅ Complete | Passthrough (no isolation) |
| **DockerSandbox** | `sandbox/docker.go` | ✅ Complete | Container isolation |
| **E2BSandbox** | `sandbox/e2b.go` | ✅ Complete | Cloud sandbox |
| **StreamingSandbox** | `sandbox/sandbox.go` | ✅ Complete | Real-time output |
| **RuntimeResolver** | `sandbox/sandbox.go` | ✅ Complete | Auto-detect mode |
| **Factory** | `sandbox/sandbox.go` | ✅ Complete | Create sandboxes |
| **ToolRegistry w/Sandbox** | `tools/registry.go` | ✅ Complete | Tools use sandbox |
| **Bash Tool w/Sandbox** | `tools/bash.go` | ✅ Complete | Executes via sandbox |
| **AgenticExecutor Sandbox Fields** | `executor.go` | ✅ Complete | Fields exist |
| **Executor Sandbox Lifecycle** | `executor.go` | ✅ Complete | setup/cleanup wired |

### What's Missing (Integration)

| Component | Location | Status | Required Action |
|-----------|----------|--------|-----------------|
| **Sandbox frontmatter parsing** | `agent_execution_engine.go` | ❌ Missing | Parse `sandbox:` from .prompt |
| **Sandbox creation in engine** | `agent_execution_engine.go` | ❌ Missing | Create sandbox before tools |
| **Pass sandbox to ToolRegistry** | `agent_execution_engine.go` | ❌ Missing | Use `NewToolRegistryWithSandbox` |
| **Pass sandbox to executor** | `agent_execution_engine.go` | ❌ Missing | Use `WithSandbox()` option |
| **Session → Workspace → Sandbox** | `session/manager.go` | ❌ Disconnected | Wire session to sandbox |

---

## Existing Station Sandbox System

Station already has a sandbox system in `internal/services/`:

```
internal/services/
├── sandbox_service.go           # Core service (Dagger-based)
├── sandbox_docker_backend.go    # Docker backend for persistent sessions
└── sandbox_opencode_backend.go  # OpenCode integration
```

### Comparison

| Feature | Existing (`internal/services/`) | New (`pkg/harness/sandbox/`) |
|---------|--------------------------------|------------------------------|
| **Engine** | Dagger | Direct Docker/E2B API |
| **Purpose** | Code execution tool | Agent execution isolation |
| **Scope** | Single code snippet | Entire agent run |
| **Streaming** | No | Yes (ExecStream) |
| **Auto-detect** | No | Yes (RuntimeResolver) |
| **E2B Support** | No | Yes |

### Recommendation

The `pkg/harness/sandbox/` is better suited for agent-level isolation because:
1. Designed for entire agent runs, not single code executions
2. Streaming support for real-time tool output
3. RuntimeResolver for environment adaptation
4. E2B support for serverless deployment

---

## Deployment Constraint Analysis

### Why Sandbox Was Needed

Station runs in containers via `stn up`:

```dockerfile
# Dockerfile - Station container
FROM ubuntu:22.04
# Contains: git, python, node, docker CLI
# But NOT: arbitrary dev tools the agent might need
```

**Problem**: When Station runs in a container, agents have limited tools:
- Container has basic tools (git, python, node)
- Agent might need tools not in container (rust, terraform, kubectl)
- Can't install tools at runtime (container is immutable)

**Solution Options**:

| Option | How It Works | Pros | Cons |
|--------|--------------|------|------|
| **Host Mode** | Execute on host | Full access | No isolation |
| **Docker Mode** | Spawn sibling containers | Custom images | Docker-in-Docker complexity |
| **E2B Mode** | Cloud sandbox | No local Docker needed | Latency, cost |

### RuntimeResolver Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    DEPLOYMENT SCENARIO                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  SCENARIO 1: Developer laptop                                    │
│  ├── Host has all tools (python, node, etc.)                     │
│  └── RuntimeResolver → ModeHost (no overhead)                    │
│                                                                  │
│  SCENARIO 2: Station container (stn up)                          │
│  ├── Container has limited tools                                 │
│  ├── Agent needs tools not in container                          │
│  ├── isolation_backend: docker configured                        │
│  └── RuntimeResolver → ModeDocker (spawn sibling container)      │
│                                                                  │
│  SCENARIO 3: Serverless/CloudShip deployment                     │
│  ├── No Docker available                                         │
│  ├── isolation_backend: e2b configured                           │
│  └── RuntimeResolver → ModeE2B (cloud sandbox)                   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Session / Workspace / Sandbox Relationship

### Current State (Disconnected)

```
Session (pkg/harness/session/)
├── Manages: workspace paths, locks, metadata
├── NOT connected to: sandbox
└── Used by: nothing currently

Workspace (pkg/harness/workspace/)
├── Manages: file operations within a path
├── NOT connected to: sandbox
└── Used by: executeWithAgenticHarness (HostWorkspace only)

Sandbox (pkg/harness/sandbox/)
├── Manages: isolated execution environment
├── NOT connected to: session or workspace
└── Used by: nothing currently
```

### Desired State (Connected)

```
Session
├── Creates/manages workspace path
├── Provides repo URL for cloning
└── Locks workspace during execution
        │
        ▼
Workspace Resolver
├── Resolves workspace path from session
├── Clones repo if git_source provided
└── Returns workspace path
        │
        ▼
Sandbox
├── Receives workspace path as mount point
├── Mounts workspace into container (Docker mode)
├── Uploads workspace to cloud (E2B mode)
└── Tools execute inside sandbox with workspace access
        │
        ▼
Tools
├── Read/write files via sandbox.ReadFile/WriteFile
├── Execute commands via sandbox.Exec
└── Changes visible in workspace after completion
```

---

## NATS / Lattice Integration

### Current Integration Points

```
pkg/harness/nats/
├── lattice_adapter.go    # Uses existing work.WorkStore
├── store.go              # NATS KV + Object Store
├── handoff.go            # Multi-agent workflow coordination
└── nats_publisher.go     # Streaming events to NATS

pkg/harness/stream/
└── nats_publisher.go     # Event streaming
```

### How Sandbox Affects Lattice

Sandbox doesn't change lattice architecture, but:
1. **Work records** should track sandbox mode used
2. **File sharing** may need to extract from sandbox to Object Store
3. **Streaming** should include sandbox events (container start/stop)

---

## Integration Roadmap

### Phase 3.5: Wire Sandbox into Execution (HIGH PRIORITY)

```
1. Modify executeWithAgenticHarness():
   - Parse sandbox config from frontmatter
   - Create sandbox via Factory
   - Pass to ToolRegistry
   - Pass to AgenticExecutor

2. Add frontmatter support:
   sandbox:
     mode: auto | host | docker | e2b
     image: station-sandbox:latest  # For docker mode
     timeout: 30m
   
3. Add config.yaml support:
   harness:
     sandbox:
       default_mode: auto
       isolation_backend: docker  # Fallback for auto mode
       e2b_api_key: ${E2B_API_KEY}
```

### Phase 3.6: Session → Workspace → Sandbox (MEDIUM PRIORITY)

```
1. Wire SessionManager into executeWithAgenticHarness()
2. Resolve workspace from session
3. Clone repo if git_source provided
4. Create sandbox with workspace mounted
5. Lock session during execution
```

### Phase 3.7: NATS Sandbox Events (LOW PRIORITY)

```
1. Add sandbox_start/sandbox_stop events to stream
2. Track sandbox mode in work records
3. Extract artifacts from sandbox to Object Store
```

---

## Test Status

### Tests That Pass (Isolated)

```bash
go test ./pkg/harness/sandbox/... -v  # 25 tests ✅
go test ./pkg/harness/... -v          # All pass ✅
```

### Tests That Would Fail (If Integration Existed)

The sandbox tests pass because they test the sandbox in isolation. If we wire it into `executeWithAgenticHarness()`, we'd need:

1. **E2E tests with sandbox** - Currently skip sandbox entirely
2. **Docker-in-Docker tests** - Need DinD test infrastructure
3. **E2B integration tests** - Need E2B API key and network access

---

## Recommendations

### Immediate (This Session)

1. **Don't** try to fully wire sandbox into agent_execution_engine.go
2. **Do** document the integration plan clearly
3. **Do** ensure pkg/harness/sandbox tests continue to pass

### Next Session

1. Create `parseHarnessSandboxConfig()` function
2. Add sandbox frontmatter schema
3. Wire sandbox into `executeWithAgenticHarness()`
4. Add integration test with Docker sandbox

### Future

1. Wire session manager
2. Add E2B integration test
3. Add sandbox events to NATS stream

---

## Files Reference

| File | Purpose | Lines |
|------|---------|-------|
| `internal/services/agent_execution_engine.go` | Main entry, needs sandbox wiring | 1391-1512 |
| `pkg/harness/executor.go` | Has sandbox fields, lifecycle wired | ~600 |
| `pkg/harness/tools/registry.go` | Has `NewToolRegistryWithSandbox` | ~90 |
| `pkg/harness/tools/bash.go` | Uses sandbox if provided | ~190 |
| `pkg/harness/sandbox/sandbox.go` | Interface + Factory + Resolver | ~310 |
| `pkg/harness/session/manager.go` | Session management (unused) | ~360 |
| `pkg/harness/nats/lattice_adapter.go` | NATS integration | ~530 |
