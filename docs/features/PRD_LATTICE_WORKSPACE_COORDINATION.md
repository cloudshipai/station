# PRD: Lattice Workspace Coordination

## Status: Proposed (Nice-to-Have)

This feature builds on top of the core Station Lattice (PRD_STATION_LATTICE.md). The core lattice provides agent discovery, async work assignment, and distributed run tracking. This PRD describes an **optional enhancement** for coordinating shared workspace access when multiple agents collaborate on code.

---

## Overview

Lattice Workspace Coordination enables multiple AI coding agents (potentially on different stations) to collaborate on shared codebases without conflicts. Rather than implementing a distributed filesystem, this feature leverages **Git as the coordination mechanism** with the lattice tracking **who is working on what**.

### Key Insight: Git IS the Distributed Filesystem

Each station maintains its own git clone. The lattice doesn't share files directly—it coordinates access:
- Lattice tracks: `{repo, branch, assigned_station, assigned_agent, status}`
- Agents use existing `coding_open(repo_url, branch)` to get workspace
- Write locks prevent concurrent writes to same branch
- Git push/pull is the sync mechanism between stations
- Handoff signals notify next agent when work is ready

### Why Nice-to-Have?

The core lattice already provides significant value:
- Async work dispatch across stations
- Agent discovery with schema awareness  
- Distributed run tracking with UUIDs
- Real-time observability via KV watches

Workspace coordination is an **enhancement for complex multi-agent coding workflows** where agents need to:
1. Collaborate on the same repository
2. Hand off work to each other sequentially
3. Work on parallel branches that eventually merge

Most use cases work fine with each agent operating in isolation on assigned tasks.

---

## Problem Statement

### Current State

When an orchestrator assigns coding work to multiple agents:
1. Each agent gets work via `assign_work(agent="CodingAgent", task="implement feature X")`
2. Agent uses `coding_open(repo_url="...", branch="feature-x")` to get workspace
3. Agent makes changes, commits, pushes
4. **No coordination** if another agent is also working on the same branch

**Conflicts can occur when:**
- Two agents assigned to same branch concurrently
- Agent B starts before Agent A pushes
- Merge conflicts from uncoordinated parallel work
- Agent C needs to continue where Agent A left off (handoff)

### Desired State

```
Orchestrator: "Implement authentication system"
  │
  ├─► assign_work(agent="ArchitectAgent", task="design auth schema")
  │     └─ writes to: main-repo, branch: feature/auth, status: WRITING
  │     └─ completes → status: READY_FOR_HANDOFF
  │
  └─► assign_work(agent="ImplementerAgent", task="implement auth from design")
        └─ waits for: feature/auth to be READY_FOR_HANDOFF
        └─ git pull (gets ArchitectAgent's work)
        └─ status: WRITING
        └─ completes → pushes, status: COMPLETE
```

The lattice coordinates **access**, not file contents. Git handles actual synchronization.

---

## Architecture Design

### Workspace Assignment Model

```go
// internal/lattice/workspace/assignment.go

type WorkspaceAssignment struct {
    AssignmentID   string    `json:"assignment_id"`   // UUID
    WorkID         string    `json:"work_id"`         // Links to WorkAssignment
    
    // Repository info
    RepoURL        string    `json:"repo_url"`        // Git remote URL
    Branch         string    `json:"branch"`          // Branch being worked on
    BaseBranch     string    `json:"base_branch"`     // Branch to merge into (e.g., main)
    
    // Assignment
    AssignedStation string   `json:"assigned_station"`
    AssignedAgent   string   `json:"assigned_agent"`
    AssignedAt      time.Time `json:"assigned_at"`
    
    // State
    Status         string    `json:"status"`          // See status enum below
    
    // Handoff chain
    PreviousAssignment string `json:"previous_assignment,omitempty"` // For handoffs
    NextAssignment     string `json:"next_assignment,omitempty"`     // Planned successor
    
    // Metadata
    LastCommitSHA  string    `json:"last_commit_sha,omitempty"`
    LastPushAt     time.Time `json:"last_push_at,omitempty"`
}

// Status values
const (
    StatusPending       = "PENDING"         // Assigned but not yet started
    StatusWriting       = "WRITING"         // Agent has write lock, actively working
    StatusPushed        = "PUSHED"          // Changes pushed, still owns branch
    StatusReadyHandoff  = "READY_HANDOFF"   // Ready for next agent to take over
    StatusComplete      = "COMPLETE"        // Work done, branch can be merged
    StatusAbandoned     = "ABANDONED"       // Agent failed/timed out
)
```

### NATS KV Schema

Workspace state stored in JetStream KV (same pattern as work state in Phase 5C):

```go
// KV Bucket: lattice-workspace
// TTL: 7 days (matches work state)

// Key patterns:
//   workspace:{assignment_id}     → Full WorkspaceAssignment JSON
//   branch:{repo_hash}:{branch}   → Current assignment_id (write lock)
//   repo:{repo_hash}:branches     → []string of active branches
```

### Write Lock Mechanism

Only one agent can write to a branch at a time:

```go
// internal/lattice/workspace/lock.go

type WorkspaceLock struct {
    kv nats.KeyValue
}

// AcquireLock attempts to get write lock on a branch
// Returns error if branch is already locked by another agent
func (l *WorkspaceLock) AcquireLock(ctx context.Context, assignment *WorkspaceAssignment) error {
    key := fmt.Sprintf("branch:%s:%s", hashRepo(assignment.RepoURL), assignment.Branch)
    
    // Try to create (fails if exists)
    _, err := l.kv.Create(key, []byte(assignment.AssignmentID))
    if err != nil {
        // Check who has it
        entry, _ := l.kv.Get(key)
        if entry != nil {
            existingID := string(entry.Value())
            return &BranchLockedError{
                Branch:       assignment.Branch,
                LockedBy:     existingID,
                RequestedBy:  assignment.AssignmentID,
            }
        }
        return err
    }
    
    return nil
}

// ReleaseLock releases write lock (called on handoff or completion)
func (l *WorkspaceLock) ReleaseLock(ctx context.Context, assignment *WorkspaceAssignment) error {
    key := fmt.Sprintf("branch:%s:%s", hashRepo(assignment.RepoURL), assignment.Branch)
    return l.kv.Delete(key)
}

// TransferLock atomically transfers lock to next agent (handoff)
func (l *WorkspaceLock) TransferLock(ctx context.Context, from, to *WorkspaceAssignment) error {
    key := fmt.Sprintf("branch:%s:%s", hashRepo(from.RepoURL), from.Branch)
    
    entry, err := l.kv.Get(key)
    if err != nil {
        return err
    }
    
    // Verify current owner
    if string(entry.Value()) != from.AssignmentID {
        return fmt.Errorf("lock not owned by %s", from.AssignmentID)
    }
    
    // Atomic update to new owner
    _, err = l.kv.Update(key, []byte(to.AssignmentID), entry.Revision())
    return err
}
```

### Integration with `coding_open`

The existing `coding_open` tool is extended with optional lattice integration:

```go
// internal/coding/tool.go (modified)

type CodingOpenInput struct {
    RepoURL     string `json:"repo_url"`
    Branch      string `json:"branch"`
    
    // NEW: Optional lattice integration
    WorkID      string `json:"work_id,omitempty"`       // Links to lattice work assignment
    RequireLock bool   `json:"require_lock,omitempty"`  // Require write lock before opening
}

func (t *CodingTool) Open(ctx context.Context, input CodingOpenInput) (*CodingSession, error) {
    // If lattice integration enabled and work_id provided
    if t.latticeEnabled && input.WorkID != "" {
        // Create workspace assignment
        assignment := &WorkspaceAssignment{
            AssignmentID:    uuid.NewString(),
            WorkID:          input.WorkID,
            RepoURL:         input.RepoURL,
            Branch:          input.Branch,
            AssignedStation: t.stationID,
            AssignedAgent:   GetAgentFromContext(ctx),
            Status:          StatusPending,
        }
        
        // Acquire write lock if required
        if input.RequireLock {
            if err := t.workspaceLock.AcquireLock(ctx, assignment); err != nil {
                return nil, fmt.Errorf("cannot acquire workspace lock: %w", err)
            }
        }
        
        // Store assignment in KV
        if err := t.workspaceStore.Save(assignment); err != nil {
            return nil, err
        }
        
        // Update status to WRITING
        assignment.Status = StatusWriting
        t.workspaceStore.UpdateStatus(assignment.AssignmentID, StatusWriting)
    }
    
    // Continue with normal coding_open logic...
    return t.openWorkspace(ctx, input)
}
```

### Handoff Workflow

When Agent A completes and Agent B needs to continue:

```go
// internal/lattice/workspace/handoff.go

type HandoffRequest struct {
    FromAssignmentID string `json:"from_assignment_id"`
    ToAgentName      string `json:"to_agent_name"`
    ToStation        string `json:"to_station,omitempty"` // Empty = any station with agent
    Message          string `json:"message,omitempty"`    // Context for next agent
}

func (s *WorkspaceService) InitiateHandoff(ctx context.Context, req *HandoffRequest) error {
    // 1. Get current assignment
    from, err := s.store.Get(req.FromAssignmentID)
    if err != nil {
        return err
    }
    
    // 2. Ensure current agent pushed their changes
    if from.Status != StatusPushed {
        return fmt.Errorf("must push changes before handoff (current status: %s)", from.Status)
    }
    
    // 3. Update status to ready for handoff
    from.Status = StatusReadyHandoff
    s.store.UpdateStatus(from.AssignmentID, StatusReadyHandoff)
    
    // 4. Publish handoff event (next agent's station will pick this up)
    handoffEvent := &HandoffEvent{
        WorkspaceAssignment: from,
        ToAgentName:         req.ToAgentName,
        ToStation:           req.ToStation,
        Message:             req.Message,
    }
    
    subject := fmt.Sprintf("lattice.%s.workspace.handoff", s.latticeID)
    return s.nats.Publish(subject, handoffEvent)
}
```

### Genkit Tools for Workspace Coordination

```go
// internal/services/workspace_tools.go

// workspace_status - Check current branch status
func CreateWorkspaceStatusTool(service *WorkspaceService) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "workspace_status",
        `Check the current status of a branch across the lattice.
        
Returns who (if anyone) is currently working on the branch,
and whether it's available for you to work on.`,
        WorkspaceStatusInputSchema,
        func(ctx context.Context, input WorkspaceStatusInput) (*WorkspaceStatusOutput, error) {
            status, err := service.GetBranchStatus(ctx, input.RepoURL, input.Branch)
            if err != nil {
                return nil, err
            }
            
            return &WorkspaceStatusOutput{
                Branch:          input.Branch,
                Status:          status.Status,
                AssignedTo:      status.AssignedAgent,
                AssignedStation: status.AssignedStation,
                LastCommit:      status.LastCommitSHA,
                Available:       status.Status == "" || status.Status == StatusComplete,
            }, nil
        },
    )
}

// workspace_handoff - Hand off branch to another agent
func CreateWorkspaceHandoffTool(service *WorkspaceService) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "workspace_handoff",
        `Hand off your current branch to another agent.
        
Use this when:
- You've completed your part of the work
- Another agent needs to continue from where you left off
- You're transferring responsibility for a branch

IMPORTANT: You must push your changes before handing off.`,
        WorkspaceHandoffInputSchema,
        func(ctx context.Context, input WorkspaceHandoffInput) (*WorkspaceHandoffOutput, error) {
            workCtx := GetWorkContext(ctx)
            
            err := service.InitiateHandoff(ctx, &HandoffRequest{
                FromAssignmentID: workCtx.WorkspaceAssignmentID,
                ToAgentName:      input.ToAgentName,
                ToStation:        input.ToStation,
                Message:          input.Message,
            })
            if err != nil {
                return nil, err
            }
            
            return &WorkspaceHandoffOutput{
                Success:     true,
                Branch:      input.Branch,
                HandedOffTo: input.ToAgentName,
                Message:     "Handoff initiated. Next agent will be notified.",
            }, nil
        },
    )
}

// workspace_await_handoff - Wait for a branch to be handed off to you
func CreateWorkspaceAwaitHandoffTool(service *WorkspaceService) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "workspace_await_handoff",
        `Wait for another agent to hand off a branch to you.
        
Use this when you know another agent is working on a branch
and you need to continue after they finish.`,
        WorkspaceAwaitHandoffInputSchema,
        func(ctx context.Context, input WorkspaceAwaitHandoffInput) (*WorkspaceAwaitHandoffOutput, error) {
            timeout := parseDuration(input.Timeout, 5*time.Minute)
            ctx, cancel := context.WithTimeout(ctx, timeout)
            defer cancel()
            
            assignment, err := service.AwaitHandoff(ctx, input.RepoURL, input.Branch)
            if err != nil {
                return nil, err
            }
            
            return &WorkspaceAwaitHandoffOutput{
                Branch:        input.Branch,
                FromAgent:     assignment.AssignedAgent,
                FromStation:   assignment.AssignedStation,
                LastCommitSHA: assignment.LastCommitSHA,
                Message:       assignment.HandoffMessage,
                Ready:         true,
            }, nil
        },
    )
}
```

---

## Visual: Multi-Agent Coding Workflow

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                              ORCHESTRATOR STATION                                    │
│                                                                                      │
│  User: "Build a REST API with authentication"                                       │
│                                                                                      │
│  ┌────────────────────────────────────────────────────────────────────────────────┐ │
│  │  Orchestrator Agent                                                             │ │
│  │                                                                                 │ │
│  │  1. assign_work(agent="ArchitectAgent", task="Design API schema and auth flow") │ │
│  │  2. assign_work(agent="BackendAgent", task="Implement API endpoints")           │ │
│  │     └─ depends_on: ArchitectAgent handoff                                       │ │
│  │  3. assign_work(agent="TestAgent", task="Write API tests")                      │ │
│  │     └─ depends_on: BackendAgent handoff                                         │ │
│  └────────────────────────────────────────────────────────────────────────────────┘ │
│                │                    │                    │                           │
└────────────────┼────────────────────┼────────────────────┼───────────────────────────┘
                 │                    │                    │
                 ▼                    │                    │
┌────────────────────────────────────┐│                    │
│  ARCHITECT STATION                 ││                    │
│                                    ││                    │
│  ┌──────────────────────────────┐  ││                    │
│  │ ArchitectAgent               │  ││                    │
│  │                              │  ││                    │
│  │ 1. coding_open(              │  ││                    │
│  │      repo="api-repo",        │  ││                    │
│  │      branch="feature/auth",  │  ││                    │
│  │      work_id="...",          │  ││                    │
│  │      require_lock=true)      │  ││                    │
│  │    → KV: branch locked       │  ││                    │
│  │    → Status: WRITING         │  ││                    │
│  │                              │  ││                    │
│  │ 2. code(file="schema.sql")   │  ││                    │
│  │    code(file="auth_flow.md") │  ││                    │
│  │                              │  ││                    │
│  │ 3. coding_close()            │  ││                    │
│  │    → git commit & push       │  ││                    │
│  │    → Status: PUSHED          │  ││                    │
│  │                              │  ││                    │
│  │ 4. workspace_handoff(        │  ││                    │
│  │      to_agent="BackendAgent",│  ││                    │
│  │      message="Schema ready") │  ││                    │
│  │    → Status: READY_HANDOFF   │  ││                    │
│  │    → Lock transferred        │  ││                    │
│  └──────────────────────────────┘  ││                    │
│                                    ││                    │
└────────────────────────────────────┘│                    │
                                      ▼                    │
                 ┌────────────────────────────────────────┐│
                 │  BACKEND STATION                       ││
                 │                                        ││
                 │  ┌──────────────────────────────────┐  ││
                 │  │ BackendAgent                     │  ││
                 │  │                                  │  ││
                 │  │ 1. workspace_await_handoff(      │  ││
                 │  │      branch="feature/auth")      │  ││
                 │  │    → Receives handoff event      │  ││
                 │  │                                  │  ││
                 │  │ 2. coding_open(                  │  ││
                 │  │      repo="api-repo",            │  ││
                 │  │      branch="feature/auth")      │  ││
                 │  │    → git pull (gets schema)      │  ││
                 │  │    → Status: WRITING             │  ││
                 │  │                                  │  ││
                 │  │ 3. code(file="handlers.go")      │  ││
                 │  │    code(file="middleware.go")    │  ││
                 │  │                                  │  ││
                 │  │ 4. coding_close()                │  ││
                 │  │    → git commit & push           │  ││
                 │  │                                  │  ││
                 │  │ 5. workspace_handoff(            │  ││
                 │  │      to_agent="TestAgent")       │  ││
                 │  └──────────────────────────────────┘  ││
                 │                                        ││
                 └────────────────────────────────────────┘│
                                                          ▼
                                      ┌────────────────────────────────────────┐
                                      │  TEST STATION                          │
                                      │                                        │
                                      │  ┌──────────────────────────────────┐  │
                                      │  │ TestAgent                        │  │
                                      │  │                                  │  │
                                      │  │ 1. workspace_await_handoff(...)  │  │
                                      │  │                                  │  │
                                      │  │ 2. coding_open(...)              │  │
                                      │  │    → git pull (gets impl)        │  │
                                      │  │                                  │  │
                                      │  │ 3. code(file="api_test.go")      │  │
                                      │  │                                  │  │
                                      │  │ 4. coding_close()                │  │
                                      │  │    → Status: COMPLETE            │  │
                                      │  │    → Branch ready for merge      │  │
                                      │  └──────────────────────────────────┘  │
                                      │                                        │
                                      └────────────────────────────────────────┘
```

---

## Configuration

```yaml
# config.yaml
lattice:
  workspace:
    enabled: true                    # Enable workspace coordination
    require_lock_by_default: false   # Require lock on all coding_open calls
    lock_timeout: 30m                # Auto-release lock after timeout
    handoff_timeout: 5m              # Max wait for handoff
    
    # KV settings (uses same JetStream as work state)
    kv:
      bucket: "lattice-workspace"
      replicas: 3
      history: 10
      ttl: 168h                      # 7 days
```

---

## Behavior Matrix

| Scenario | Behavior |
|----------|----------|
| `coding_open` without `work_id` | Normal behavior, no lattice tracking |
| `coding_open` with `work_id`, no lock | Opens workspace, tracks in KV, no lock |
| `coding_open` with `require_lock=true` | Acquires lock first, fails if locked |
| Branch locked by another agent | `BranchLockedError` with lock holder info |
| `workspace_handoff` before push | Error: "must push changes first" |
| `workspace_handoff` success | Lock transferred, next agent notified |
| `workspace_await_handoff` timeout | Error with last known state |
| Agent crashes with lock | Lock auto-released after `lock_timeout` |
| Read-only access to locked branch | Allowed (git clone/pull always works) |

---

## Integration Points

### With Existing Lattice Components

| Component | Integration |
|-----------|-------------|
| `WorkDispatcher` (Phase 5A.3) | Work assignments can include workspace context |
| `WorkStore` (Phase 5C) | Workspace uses same KV pattern |
| `OrchestratorContext` (Phase 5B) | Workspace assignments linked via `work_id` |
| `coding_open` tool | Extended with optional lattice params |

### No New Filesystem Primitives

This design explicitly avoids:
- FUSE mounts or virtual filesystems
- AgentFS or similar distributed FS solutions
- Shared network drives
- Real-time file sync

Git already solved distributed file collaboration. The lattice just coordinates access.

---

## CLI Commands

```bash
# List all active workspace assignments
stn lattice workspace list

# Check status of a specific branch
stn lattice workspace status --repo <url> --branch <branch>

# Force-release a lock (admin operation)
stn lattice workspace unlock --repo <url> --branch <branch>

# View handoff history for a branch
stn lattice workspace history --repo <url> --branch <branch>
```

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Lock acquisition latency | < 10ms |
| Handoff notification latency | < 100ms |
| Lock release on timeout | 100% reliability |
| Concurrent branch operations | No conflicts |
| Git sync accuracy | 100% (git handles this) |

---

## Implementation Order

1. **WorkspaceAssignment model** and KV storage
2. **WriteLock mechanism** with KV-backed locking
3. **Extend `coding_open`** with optional lattice params
4. **Handoff workflow** with NATS events
5. **Genkit tools**: `workspace_status`, `workspace_handoff`, `workspace_await_handoff`
6. **CLI commands** for workspace management
7. **E2E tests**: Multi-agent handoff chain

---

## Open Questions

1. **Merge conflicts**: What happens if Agent B's work conflicts with concurrent changes to base branch? (Likely: agent needs to resolve, or escalate to orchestrator)

2. **Parallel branches**: Should lattice support multiple agents on different branches of same repo? (Likely yes, locks are per-branch)

3. **Large repos**: Clone time can be significant. Should stations cache common repos? (Likely yes, but out of scope for this PRD)

4. **Submodules**: How to handle repos with submodules? (Likely: treat as implementation detail of `coding_open`)

5. **Lock granularity**: Branch-level locks are coarse. Do we need file-level? (Likely no - Git handles file-level conflicts)

---

## Why NOT AgentFS / Distributed FS

We explored AgentFS (Turso's SQLite-backed FUSE filesystem) and similar approaches. They were rejected because:

| Approach | Problem |
|----------|---------|
| **AgentFS** | Single-writer model, FUSE mounting overhead, operational complexity |
| **NFS/Network FS** | Latency, single point of failure, not agent-aware |
| **Syncthing/rsync** | Conflict resolution is hard, not transactional |
| **Custom VFS** | Massive engineering effort, reinventing Git |

**Git already exists.** It handles:
- Distributed storage
- Conflict detection and resolution
- History and rollback
- Authentication and access control
- Branch isolation

The lattice just needs to answer: "Who is working on what?" Git handles the rest.

---

## References

- `PRD_STATION_LATTICE.md` - Core lattice architecture
- `internal/coding/tool.go` - Current `coding_open` implementation
- `internal/lattice/work/` - Work assignment (Phase 5A.3)
- `internal/lattice/work/store.go` - KV-backed work state (Phase 5C)
- `pkg/models/models.go` - CodingSession model
