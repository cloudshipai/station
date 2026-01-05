# PRD: Station Lattice - Distributed Agent Mesh

## Overview

Station Lattice enables multiple Station instances to form a distributed mesh for coordinated agent execution. The system provides:

1. **Orchestrator/Leaf Architecture** - User-facing station coordinates, leaf stations provide agents
2. **Async Work Assignment** - Work is assigned to agents asynchronously, not called synchronously
3. **Distributed Run Tracking** - UUID-based run IDs that correlate execution across stations
4. **Streaming Results** - Results stream back via NATS, not as RPC responses
5. **Full Observability** - OTEL traces span the entire distributed execution

### Key Difference from RPC

**WRONG (Synchronous RPC):**
```
Agent A: result = call_agent("SecurityScanner", "scan for vulns")  # BLOCKS waiting
```

**RIGHT (Async Work Assignment):**
```
Orchestrator: assign_work(station="security-station", agent="scanner", task="scan")
              # Returns immediately, work is queued
Scanner:      # Picks up work, executes autonomously
              # Streams results back via NATS
Orchestrator: # Receives results asynchronously, continues coordination
```

## Status

### Completed (Phases 1-4)
- [x] Core NATS client and embedded server
- [x] Station discovery and presence (heartbeats)
- [x] Agent/workflow registry across stations
- [x] Remote invocation via NATS request/reply
- [x] Server integration with `stn serve --lattice`
- [x] OTEL telemetry for Invoker, Registry, Presence
- [x] Configurable NATS ports
- [x] E2E tests passing with traces in Jaeger

### Pending (Phases 5-6)
- [ ] **Phase 5A.1**: Agent Discovery & Schema Awareness
- [ ] **Phase 5A.2**: Unique Agent Names in Lattice
- [ ] **Phase 5A.3**: Async Work Assignment
- [ ] **Phase 5B**: Distributed Run Tracking with UUID
- [ ] **Phase 5C**: Integration & CLI Commands
- [ ] Phase 6: Production hardening and multi-region support

---

## Phase 5A: Async Work Assignment Architecture

### Problem Statement

Currently, the lattice can register stations and their agents, but there's no way for an orchestrating agent to assign work to agents on leaf stations and receive results.

**Current State:**
- CLI commands (`stn lattice agent exec`) run as separate processes without lattice connection
- `InvokeRemoteAgent()` uses synchronous RPC pattern
- No way to stream results back from distributed execution

**Desired State:**
- **Orchestrator Station** receives user requests and coordinates work
- **Leaf Stations** have agents that execute assigned work autonomously
- Work is **assigned** asynchronously, not called synchronously
- Results **stream back** via NATS, enabling real-time observability
- Works in both standalone (single station) and lattice (multi-station) modes

### Architecture Design

Work assignment is **asynchronous** - the orchestrator assigns work and continues coordinating while agents execute autonomously. Agents pick up work immediately and execute without blocking the caller.

#### Work Assignment Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         ORCHESTRATOR STATION                                │
│                                                                             │
│   User: "Analyze security of my infrastructure"                            │
│         │                                                                   │
│         ▼                                                                   │
│   ┌───────────────────────────────────────────────────────────────────┐    │
│   │  Orchestrator Agent                                                │    │
│   │                                                                    │    │
│   │  1. Discovers available agents: list_available_agents()           │    │
│   │     → [VulnScanner@security-station, NetworkAudit@local, ...]     │    │
│   │                                                                    │    │
│   │  2. Assigns work (non-blocking):                                  │    │
│   │     work_id_1 = assign_work(agent="VulnScanner", task="scan")     │    │
│   │     work_id_2 = assign_work(agent="NetworkAudit", task="audit")   │    │
│   │     # Returns immediately with work_ids!                           │    │
│   │                                                                    │    │
│   │  3. Continues orchestrating... assigns more work...               │    │
│   │                                                                    │    │
│   │  4. Collects results:                                             │    │
│   │     result_1 = await_work(work_id_1)  # or poll via check_work    │    │
│   │     result_2 = await_work(work_id_2)                              │    │
│   │                                                                    │    │
│   │  5. Synthesizes and responds to user                              │    │
│   └───────────────────────────────────────────────────────────────────┘    │
│                    │                           │                            │
│        ┌───────────┘                           └───────────┐               │
│        │ NATS: WORK_ASSIGNED                               │               │
│        ▼                                                   ▼               │
└────────┼───────────────────────────────────────────────────┼───────────────┘
         │                                                   │
         │ (local)                                           │ (remote via NATS)
         ▼                                                   ▼
┌─────────────────────────────┐           ┌─────────────────────────────────┐
│  LOCAL AGENT EXECUTOR       │           │  LEAF STATION (security-station) │
│  ┌───────────────────────┐  │           │                                   │
│  │ NetworkAudit Agent    │  │           │  ┌───────────────────────────┐   │
│  │                       │  │           │  │ Hook (Work Queue)         │   │
│  │ • Picks up work       │  │           │  │                           │   │
│  │ • Executes            │  │           │  │ ← WORK_ASSIGNED received  │   │
│  │ • Streams progress    │  │           │  │ → Agent picks up work     │   │
│  │ • Sends WORK_COMPLETE │  │           │  └───────────────────────────┘   │
│  └───────────────────────┘  │           │              │                    │
└─────────────────────────────┘           │              ▼                    │
                                          │  ┌───────────────────────────┐   │
                                          │  │ VulnScanner Agent         │   │
                                          │  │                           │   │
                                          │  │ • Executes autonomously   │   │
                                          │  │ • Sends WORK_PROGRESS     │   │
                                          │  │ • Sends WORK_COMPLETE     │   │
                                          │  │ (or WORK_FAILED/ESCALATE) │   │
                                          │  └───────────────────────────┘   │
                                          │              │                    │
                                          │              │ NATS: WORK_*       │
                                          │              ▼                    │
                                          └──────────────┼────────────────────┘
                                                         │
                                    ┌────────────────────┘
                                    │
                                    ▼ (streams back to orchestrator)
                        ┌─────────────────────────────────┐
                        │  Results arrive asynchronously   │
                        │  at orchestrator station         │
                        └─────────────────────────────────┘
```

#### NATS Message Types

Work lifecycle messages for async coordination:

```go
// internal/lattice/work/messages.go

// Work lifecycle message types
const (
    // Orchestrator → Leaf Station
    MsgWorkAssigned    = "WORK_ASSIGNED"     // Work hooked to agent
    MsgWorkCancelled   = "WORK_CANCELLED"    // Work cancelled before completion
    
    // Leaf Station → Orchestrator (responses)
    MsgWorkAccepted    = "WORK_ACCEPTED"     // Agent picked up work from hook
    MsgWorkProgress    = "WORK_PROGRESS"     // Streaming progress update
    MsgWorkComplete    = "WORK_COMPLETE"     // Agent finished successfully
    MsgWorkFailed      = "WORK_FAILED"       // Agent execution failed
    MsgWorkEscalate    = "WORK_ESCALATE"     // Agent needs help, escalates to orchestrator
)

// WorkAssignment - sent when orchestrator assigns work
type WorkAssignment struct {
    WorkID            string            `json:"work_id"`         // UUID for this work unit
    OrchestratorRunID string            `json:"orchestrator_run_id"`
    ParentWorkID      string            `json:"parent_work_id,omitempty"`
    
    // Target
    TargetStation     string            `json:"target_station,omitempty"` // empty = local
    AgentName         string            `json:"agent_name"`
    
    // Task
    Task              string            `json:"task"`
    Context           map[string]any    `json:"context,omitempty"`
    
    // Metadata
    AssignedAt        time.Time         `json:"assigned_at"`
    Timeout           time.Duration     `json:"timeout,omitempty"`
    Priority          int               `json:"priority,omitempty"`       // Higher = more urgent
    
    // Tracing
    TraceID           string            `json:"trace_id,omitempty"`
    SpanID            string            `json:"span_id,omitempty"`
}

// WorkResponse - sent by agent during/after execution
type WorkResponse struct {
    WorkID            string            `json:"work_id"`
    OrchestratorRunID string            `json:"orchestrator_run_id"`
    Type              string            `json:"type"`            // MsgWork* constant
    
    // Result (for COMPLETE/FAILED)
    Result            string            `json:"result,omitempty"`
    Error             string            `json:"error,omitempty"`
    
    // Progress (for PROGRESS)
    ProgressPct       int               `json:"progress_pct,omitempty"`   // 0-100
    ProgressMsg       string            `json:"progress_msg,omitempty"`
    
    // Escalation (for ESCALATE)
    EscalationReason  string            `json:"escalation_reason,omitempty"`
    EscalationContext map[string]any    `json:"escalation_context,omitempty"`
    
    // Metadata
    StationID         string            `json:"station_id"`
    LocalRunID        int64             `json:"local_run_id,omitempty"`
    DurationMs        float64           `json:"duration_ms,omitempty"`
    Timestamp         time.Time         `json:"timestamp"`
}
```

#### NATS Subject Convention

```
# Work assignment (Orchestrator → Leaf)
lattice.{lattice_id}.station.{station_id}.work.assign

# Work responses (Leaf → Orchestrator, via reply subject)
lattice.{lattice_id}.work.{work_id}.response

# Broadcast work status (for observability)
lattice.{lattice_id}.work.{work_id}.status
```

#### Component Design

##### 1. WorkDispatcher (Orchestrator's work assignment)

```go
// internal/lattice/work/dispatcher.go

type WorkDispatcher struct {
    client       *nats.Conn
    registry     *Registry
    localExec    *ExecutorAdapter
    pendingWork  sync.Map           // work_id -> *PendingWork
    stationID    string
}

type PendingWork struct {
    Assignment   *WorkAssignment
    ResultChan   chan *WorkResponse  // Buffered channel for results
    ProgressChan chan *WorkResponse  // Unbuffered for streaming
    Done         chan struct{}
}

// AssignWork dispatches work and returns immediately with work_id
func (d *WorkDispatcher) AssignWork(ctx context.Context, assignment *WorkAssignment) (string, error) {
    ctx, span := tracer.Start(ctx, "WorkDispatcher.AssignWork")
    defer span.End()
    
    // Generate work ID if not provided
    if assignment.WorkID == "" {
        assignment.WorkID = uuid.NewString()
    }
    assignment.AssignedAt = time.Now()
    
    // Track pending work
    pending := &PendingWork{
        Assignment:   assignment,
        ResultChan:   make(chan *WorkResponse, 1),
        ProgressChan: make(chan *WorkResponse, 10),
        Done:         make(chan struct{}),
    }
    d.pendingWork.Store(assignment.WorkID, pending)
    
    // Route: local or remote?
    if assignment.TargetStation == "" || assignment.TargetStation == d.stationID {
        // Local execution - still async via goroutine
        go d.executeLocal(ctx, assignment, pending)
    } else {
        // Remote execution via NATS
        if err := d.publishWorkAssignment(ctx, assignment); err != nil {
            d.pendingWork.Delete(assignment.WorkID)
            return "", fmt.Errorf("failed to assign work: %w", err)
        }
        // Subscribe to responses
        go d.subscribeToWorkResponses(ctx, assignment.WorkID, pending)
    }
    
    return assignment.WorkID, nil
}

// AwaitWork blocks until work completes or context cancelled
func (d *WorkDispatcher) AwaitWork(ctx context.Context, workID string) (*WorkResponse, error) {
    val, ok := d.pendingWork.Load(workID)
    if !ok {
        return nil, fmt.Errorf("work %s not found", workID)
    }
    pending := val.(*PendingWork)
    
    select {
    case result := <-pending.ResultChan:
        return result, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-time.After(pending.Assignment.Timeout):
        return nil, fmt.Errorf("work %s timed out", workID)
    }
}

// StreamProgress returns channel for progress updates
func (d *WorkDispatcher) StreamProgress(workID string) (<-chan *WorkResponse, error) {
    val, ok := d.pendingWork.Load(workID)
    if !ok {
        return nil, fmt.Errorf("work %s not found", workID)
    }
    return val.(*PendingWork).ProgressChan, nil
}

// CheckWork returns current status without blocking
func (d *WorkDispatcher) CheckWork(workID string) (*WorkStatus, error) {
    val, ok := d.pendingWork.Load(workID)
    if !ok {
        return nil, fmt.Errorf("work %s not found", workID)
    }
    pending := val.(*PendingWork)
    
    select {
    case result := <-pending.ResultChan:
        // Put it back for AwaitWork
        pending.ResultChan <- result
        return &WorkStatus{
            WorkID:   workID,
            Status:   result.Type,
            Result:   result,
        }, nil
    default:
        return &WorkStatus{
            WorkID: workID,
            Status: "PENDING",
        }, nil
    }
}
```

##### 2. WorkHook (Leaf Station's work receiver)

Work arrives via NATS subscription and agents pick it up immediately:

```go
// internal/lattice/work/hook.go

type WorkHook struct {
    client       *nats.Conn
    executor     *ExecutorAdapter
    stationID    string
    workQueue    chan *WorkAssignment  // Buffered work queue
}

func (h *WorkHook) Start(ctx context.Context) error {
    // Subscribe to work assignments for this station
    subject := fmt.Sprintf("lattice.*.station.%s.work.assign", h.stationID)
    
    _, err := h.client.Subscribe(subject, func(msg *nats.Msg) {
        var assignment WorkAssignment
        if err := json.Unmarshal(msg.Data, &assignment); err != nil {
            log.Printf("Invalid work assignment: %v", err)
            return
        }
        
        // Pick up work immediately and execute
        go h.executeWork(ctx, &assignment, msg.Reply)
    })
    
    return err
}

func (h *WorkHook) executeWork(ctx context.Context, assignment *WorkAssignment, replySubject string) {
    // Send WORK_ACCEPTED
    h.sendResponse(replySubject, &WorkResponse{
        WorkID:    assignment.WorkID,
        Type:      MsgWorkAccepted,
        StationID: h.stationID,
        Timestamp: time.Now(),
    })
    
    // Execute the agent
    result, localRunID, err := h.executor.ExecuteAgentByName(
        ctx,
        assignment.AgentName,
        assignment.Task,
        assignment.OrchestratorRunID,
        assignment.WorkID,
    )
    
    // Send result
    response := &WorkResponse{
        WorkID:            assignment.WorkID,
        OrchestratorRunID: assignment.OrchestratorRunID,
        StationID:         h.stationID,
        LocalRunID:        localRunID,
        Timestamp:         time.Now(),
    }
    
    if err != nil {
        response.Type = MsgWorkFailed
        response.Error = err.Error()
    } else {
        response.Type = MsgWorkComplete
        response.Result = result
    }
    
    h.sendResponse(replySubject, response)
}
```

##### 3. Genkit Tools: Async Work Pattern

Three tools for the async workflow:

```go
// internal/services/builtin_tools.go

// assign_work - Non-blocking work assignment
func CreateAssignWorkTool(dispatcher *WorkDispatcher) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "assign_work",
        `Assign work to an agent. Returns immediately with a work_id.
        
The agent will execute autonomously. Use await_work or check_work to get results.
Works for both local and remote agents.`,
        AssignWorkInputSchema,
        func(ctx context.Context, input AssignWorkInput) (*AssignWorkOutput, error) {
            orchCtx := GetOrchestratorContext(ctx)
            
            assignment := &WorkAssignment{
                OrchestratorRunID: orchCtx.RunID,
                ParentWorkID:      orchCtx.WorkID,
                AgentName:         input.AgentName,
                Task:              input.Task,
                Context:           input.Context,
                Priority:          input.Priority,
                Timeout:           parseDuration(input.Timeout, 5*time.Minute),
                TraceID:           orchCtx.TraceID,
            }
            
            workID, err := dispatcher.AssignWork(ctx, assignment)
            if err != nil {
                return nil, err
            }
            
            return &AssignWorkOutput{
                WorkID:    workID,
                AgentName: input.AgentName,
                Status:    "ASSIGNED",
            }, nil
        },
    )
}

type AssignWorkInput struct {
    AgentName string         `json:"agent_name" jsonschema:"description=Name of the agent to assign work to"`
    Task      string         `json:"task" jsonschema:"description=Task description for the agent"`
    Context   map[string]any `json:"context,omitempty" jsonschema:"description=Additional context for the agent"`
    Priority  int            `json:"priority,omitempty" jsonschema:"description=Priority (higher=more urgent)"`
    Timeout   string         `json:"timeout,omitempty" jsonschema:"description=Max wait time (e.g. '5m', '30s')"`
}

type AssignWorkOutput struct {
    WorkID    string `json:"work_id"`
    AgentName string `json:"agent_name"`
    Status    string `json:"status"`
}

// await_work - Block until work completes
func CreateAwaitWorkTool(dispatcher *WorkDispatcher) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "await_work",
        `Wait for assigned work to complete and return the result.
        
Use this when you need the result before continuing. For multiple parallel tasks,
assign all work first, then await each result.`,
        AwaitWorkInputSchema,
        func(ctx context.Context, input AwaitWorkInput) (*WorkResultOutput, error) {
            timeout := parseDuration(input.Timeout, 5*time.Minute)
            ctx, cancel := context.WithTimeout(ctx, timeout)
            defer cancel()
            
            response, err := dispatcher.AwaitWork(ctx, input.WorkID)
            if err != nil {
                return nil, err
            }
            
            return &WorkResultOutput{
                WorkID:    response.WorkID,
                Status:    response.Type,
                Result:    response.Result,
                Error:     response.Error,
                Duration:  response.DurationMs,
            }, nil
        },
    )
}

// check_work - Non-blocking status check
func CreateCheckWorkTool(dispatcher *WorkDispatcher) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "check_work",
        `Check the status of assigned work without blocking.
        
Returns current status (PENDING, ACCEPTED, COMPLETE, FAILED).
Use this to poll for completion if you don't want to block.`,
        CheckWorkInputSchema,
        func(ctx context.Context, input CheckWorkInput) (*WorkStatusOutput, error) {
            status, err := dispatcher.CheckWork(input.WorkID)
            if err != nil {
                return nil, err
            }
            
            output := &WorkStatusOutput{
                WorkID: status.WorkID,
                Status: status.Status,
            }
            
            if status.Result != nil {
                output.Result = status.Result.Result
                output.Error = status.Result.Error
            }
            
            return output, nil
        },
    )
}
```

##### 4. Parallel Work Assignment Pattern

Agents can assign multiple tasks in parallel:

```go
// Example: Orchestrator agent prompt usage

// Agent system prompt:
`You are an orchestrator agent. To parallelize work:

1. Assign multiple tasks without waiting:
   work1 = assign_work(agent="VulnScanner", task="scan servers")
   work2 = assign_work(agent="NetworkAudit", task="check firewall")
   work3 = assign_work(agent="LogAnalyzer", task="find anomalies")

2. Then collect results:
   result1 = await_work(work1.work_id)
   result2 = await_work(work2.work_id)
   result3 = await_work(work3.work_id)

3. Synthesize and respond

This runs all three agents in parallel across the lattice!`
```

##### 5. Work Escalation

When an agent needs help:

```go
// Agent can escalate back to orchestrator
func CreateEscalateWorkTool(hook *WorkHook) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "escalate",
        `Escalate current work back to orchestrator when you cannot complete it.
        
Use when:
- Task is outside your capabilities
- You need information you don't have access to
- An error occurred that requires human/orchestrator intervention`,
        EscalateInputSchema,
        func(ctx context.Context, input EscalateInput) (string, error) {
            workCtx := GetWorkContext(ctx)
            
            hook.sendResponse(workCtx.ReplySubject, &WorkResponse{
                WorkID:            workCtx.WorkID,
                Type:              MsgWorkEscalate,
                EscalationReason:  input.Reason,
                EscalationContext: input.Context,
                Timestamp:         time.Now(),
            })
            
            return "Work escalated to orchestrator", nil
        },
    )
}
```

### Configuration

```yaml
# config.yaml
lattice:
  enabled: true
  orchestrator:
    embedded_nats:
      enabled: true
      port: 4222
  
  # Work assignment settings
  work:
    default_timeout: 5m        # Default work timeout
    max_parallel: 10           # Max concurrent work assignments
    queue_size: 100            # Work hook queue buffer size
    progress_interval: 5s      # How often to send progress updates
    
  # Retry settings
  retry:
    enabled: true
    max_attempts: 3
    backoff: exponential       # linear, exponential, fixed
    initial_delay: 1s
```

### Behavior Matrix

| Scenario | Behavior |
|----------|----------|
| `assign_work` local agent | Executes in goroutine, streams results |
| `assign_work` remote agent | Publishes WORK_ASSIGNED via NATS |
| `await_work` before complete | Blocks until WORK_COMPLETE/FAILED |
| `await_work` after complete | Returns immediately with result |
| `check_work` while pending | Returns `{status: "PENDING"}` |
| Agent timeout | Returns WORK_FAILED with timeout error |
| Agent escalates | Returns WORK_ESCALATE with context |
| Network partition | Timeout after configured duration |

### Comparison: Synchronous vs Asynchronous

| Aspect | Synchronous (OLD) | Asynchronous (NEW) |
|--------|-------------------|-------------------|
| Pattern | `result = call_agent()` | `id = assign_work(); result = await_work(id)` |
| Blocking | Yes, caller waits | No, caller continues |
| Parallelism | Sequential by default | Parallel by default |
| Progress | None until complete | Streaming progress |
| Timeout handling | Hard timeout, then fail | Configurable, with status checks |
| Escalation | Not supported | WORK_ESCALATE message |
| Design pattern | ❌ RPC-style | ✅ Async message-passing |

---

## Phase 5A.1: Agent Discovery & Schema Awareness

### Problem Statement

When an agent uses `assign_work`, how does it know:
1. What agents are available in the lattice?
2. What input schema each agent expects?
3. What capabilities each agent has?

Without this information, LLMs cannot make intelligent decisions about which agent to assign work to or how to format the request.

### Solution: Discovery Tools

Provide companion tools that agents can use to discover available agents and their schemas.

#### 1. `list_available_agents` Tool

Returns all agents available (local + lattice) with metadata:

```go
// internal/services/builtin_tools.go

type AgentInfo struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Location     string   `json:"location"`      // "local" or station name
    Capabilities []string `json:"capabilities"`
    InputSchema  string   `json:"input_schema"`  // JSON Schema
    OutputSchema string   `json:"output_schema"` // JSON Schema (if defined)
}

func CreateListAgentsTool(executor *LatticeAwareAgentExecutor) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "list_available_agents",
        "List all agents available in the lattice with their capabilities and input schemas.",
        ListAgentsInputSchema,
        func(ctx context.Context, input ListAgentsInput) ([]AgentInfo, error) {
            var agents []AgentInfo
            
            // 1. Get local agents
            localAgents, _ := executor.repos.Agents.ListAll()
            for _, a := range localAgents {
                agents = append(agents, AgentInfo{
                    Name:         a.Name,
                    Description:  a.Description,
                    Location:     "local",
                    Capabilities: parseCapabilities(a),
                    InputSchema:  a.InputSchema,
                    OutputSchema: a.OutputSchema,
                })
            }
            
            // 2. Get remote agents from lattice registry
            if executor.isLatticeConnected() {
                remoteAgents, _ := executor.router.ListAllAgents(ctx)
                for _, loc := range remoteAgents {
                    if !loc.IsLocal {
                        // Fetch schema from remote station
                        schema, _ := executor.fetchRemoteAgentSchema(ctx, loc)
                        agents = append(agents, AgentInfo{
                            Name:         loc.AgentName,
                            Description:  schema.Description,
                            Location:     loc.StationName,
                            Capabilities: schema.Capabilities,
                            InputSchema:  schema.InputSchema,
                            OutputSchema: schema.OutputSchema,
                        })
                    }
                }
            }
            
            // Filter by capability if specified
            if input.Capability != "" {
                agents = filterByCapability(agents, input.Capability)
            }
            
            return agents, nil
        },
    )
}

type ListAgentsInput struct {
    Capability string `json:"capability,omitempty" jsonschema:"description=Filter agents by capability (e.g., 'security', 'kubernetes', 'database')"`
}
```

#### 2. `get_agent_schema` Tool

Get detailed schema for a specific agent:

```go
func CreateGetAgentSchemaTool(executor *LatticeAwareAgentExecutor) *ai.Tool {
    return ai.NewToolWithInputSchema(
        "get_agent_schema",
        "Get the detailed input/output schema for a specific agent.",
        GetAgentSchemaInputSchema,
        func(ctx context.Context, input GetAgentSchemaInput) (*AgentSchemaResponse, error) {
            // Try local first
            agent, err := executor.repos.Agents.GetByNameGlobal(input.AgentName)
            if err == nil {
                return &AgentSchemaResponse{
                    Name:         agent.Name,
                    Description:  agent.Description,
                    InputSchema:  agent.InputSchema,
                    OutputSchema: agent.OutputSchema,
                    Examples:     agent.Examples, // Example invocations
                }, nil
            }
            
            // Try lattice
            if executor.isLatticeConnected() {
                location, err := executor.router.FindBestAgent(ctx, input.AgentName, "")
                if err == nil {
                    return executor.fetchRemoteAgentSchema(ctx, location)
                }
            }
            
            return nil, fmt.Errorf("agent '%s' not found", input.AgentName)
        },
    )
}
```

#### 3. Enhanced Registry with Schema Sync

When stations register, include full agent schemas:

```go
// internal/lattice/registry.go

type AgentRegistration struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Capabilities []string `json:"capabilities"`
    InputSchema  string   `json:"input_schema"`   // JSON Schema
    OutputSchema string   `json:"output_schema"`  // JSON Schema
    Examples     []string `json:"examples"`       // Example tasks
}

type StationRegistration struct {
    StationID   string              `json:"station_id"`
    StationName string              `json:"station_name"`
    Agents      []AgentRegistration `json:"agents"`
    Workflows   []WorkflowInfo      `json:"workflows"`
    // ...
}
```

#### 4. Dynamic Tool Description

The `assign_work` tool description is dynamically generated to include available agents:

```go
func CreateAssignWorkToolWithDescription(dispatcher *WorkDispatcher, registry *Registry) *ai.Tool {
    // Build dynamic description with available agents
    description := buildAssignWorkDescription(registry)
    
    return ai.NewToolWithInputSchema(
        "assign_work",
        description, // Includes list of available agents
        AssignWorkInputSchema,
        // ... (implementation shown in Phase 5A.3)
    )
}

func buildAssignWorkDescription(registry *Registry) string {
    var sb strings.Builder
    sb.WriteString("Assign work to an agent. Returns immediately with work_id. Available agents:\n\n")
    
    // Local agents
    localAgents, _ := registry.ListLocalAgents()
    for _, a := range localAgents {
        sb.WriteString(fmt.Sprintf("- **%s** (local): %s\n", a.Name, a.Description))
    }
    
    // Remote agents (if lattice connected)
    remoteAgents, _ := registry.ListRemoteAgents()
    for _, a := range remoteAgents {
        sb.WriteString(fmt.Sprintf("- **%s** (@%s): %s\n", 
            a.Name, a.StationName, a.Description))
    }
    
    sb.WriteString("\nUse list_available_agents for full details and schemas.")
    sb.WriteString("\nUse await_work(work_id) to get results after assignment.")
    return sb.String()
}
```

### Agent Prompt Integration

Agents that orchestrate other agents should have system prompts that explain the async work tools:

```yaml
# orchestrator-agent.prompt
---
model: gpt-4o
tools:
  - list_available_agents
  - get_agent_schema
  - assign_work
  - await_work
  - check_work
---

You are an orchestrator agent that coordinates work across multiple specialized agents.

## Available Tools

### Discovery Tools
1. **list_available_agents** - Discover what agents are available
   - Use this first to see what agents you can delegate to
   - Filter by capability if you know what type of agent you need

2. **get_agent_schema** - Get detailed input schema for an agent
   - Use before assigning work to understand expected inputs
   - Check for required fields and data types

### Work Assignment Tools (Async)
3. **assign_work** - Assign work to an agent (NON-BLOCKING)
   - Returns immediately with a work_id
   - Agent executes autonomously in background
   - Use for parallel execution of multiple agents

4. **await_work** - Wait for assigned work to complete (BLOCKING)
   - Blocks until the agent finishes
   - Returns the result when complete

5. **check_work** - Check work status without blocking (NON-BLOCKING)
   - Poll for status (PENDING, ACCEPTED, COMPLETE, FAILED)
   - Use when you need to do other work while waiting

## Workflow

### Sequential (simple tasks):
1. Discover agents: list_available_agents()
2. Check schema if needed: get_agent_schema("AgentName")
3. Assign and wait: work = assign_work(...); result = await_work(work.work_id)

### Parallel (complex tasks - PREFERRED):
1. Discover agents: list_available_agents()
2. Assign ALL work first (non-blocking):
   work1 = assign_work(agent="VulnScanner", task="scan")
   work2 = assign_work(agent="NetworkAudit", task="audit")
   work3 = assign_work(agent="LogAnalyzer", task="analyze")
3. Then collect results:
   result1 = await_work(work1.work_id)
   result2 = await_work(work2.work_id)
   result3 = await_work(work3.work_id)
4. Synthesize and respond

This parallel pattern runs all agents concurrently, significantly reducing total time.
```

---

## Phase 5A.2: Unique Agent Names in Lattice

### Problem Statement

If two stations both have an agent named "SecurityScanner", what happens when someone calls `assign_work(agent="SecurityScanner", ...)`?

Options:
1. **Allow duplicates** - Confusing, non-deterministic routing
2. **Qualified names** - `station-a/SecurityScanner` - Verbose, breaks abstraction
3. **Enforce uniqueness** - Registration fails if name already exists in lattice

### Decision: Enforce Lattice-Wide Unique Agent Names

**Rationale:**
- Agents are the "API" of the lattice - names should be stable identifiers
- Duplicate names create confusion and non-deterministic behavior
- Qualified names leak infrastructure details into agent logic
- Uniqueness is easy to enforce at registration time

### Implementation

#### 1. Registration Validation

```go
// internal/lattice/registry.go

func (r *Registry) RegisterStation(ctx context.Context, station StationRegistration) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Check for agent name conflicts
    for _, newAgent := range station.Agents {
        if conflict := r.findAgentNameConflict(newAgent.Name, station.StationID); conflict != nil {
            return &AgentNameConflictError{
                AgentName:        newAgent.Name,
                ExistingStation:  conflict.StationName,
                AttemptedStation: station.StationName,
            }
        }
    }
    
    // Proceed with registration
    r.stations[station.StationID] = &station
    return nil
}

func (r *Registry) findAgentNameConflict(agentName, excludeStationID string) *StationRegistration {
    for stationID, station := range r.stations {
        if stationID == excludeStationID {
            continue
        }
        for _, agent := range station.Agents {
            if agent.Name == agentName {
                return station
            }
        }
    }
    return nil
}

type AgentNameConflictError struct {
    AgentName        string
    ExistingStation  string
    AttemptedStation string
}

func (e *AgentNameConflictError) Error() string {
    return fmt.Sprintf(
        "agent name '%s' already registered by station '%s', cannot register from '%s'",
        e.AgentName, e.ExistingStation, e.AttemptedStation,
    )
}
```

#### 2. Station Startup Behavior

```go
// internal/lattice/presence.go

func (p *Presence) Start(ctx context.Context) error {
    // Register station with lattice
    err := p.registry.RegisterStation(ctx, p.buildRegistration())
    
    if err != nil {
        if conflictErr, ok := err.(*AgentNameConflictError); ok {
            // Log warning but continue without the conflicting agent
            log.Printf("⚠️  Agent name conflict: %s", conflictErr.Error())
            log.Printf("   Station will join lattice without agent '%s'", conflictErr.AgentName)
            
            // Re-register without conflicting agent
            registration := p.buildRegistrationExcluding(conflictErr.AgentName)
            return p.registry.RegisterStation(ctx, registration)
        }
        return err
    }
    
    return nil
}
```

#### 3. CLI Feedback

```bash
$ stn serve --lattice
🔗 Connecting to lattice orchestrator at nats://orchestrator:4222...
✅ Connected to lattice
⚠️  Agent 'SecurityScanner' already exists in lattice (station: security-station)
   → This agent will NOT be available via lattice from this station
   → Rename agent or use different lattice to avoid conflict
✅ Registered 5/6 agents with lattice
✅ Station 'my-station' online in lattice
```

#### 4. Agent Naming Conventions

Recommend namespacing in documentation:

```markdown
## Agent Naming Best Practices

To avoid conflicts in multi-station lattices, use descriptive, namespaced names:

**Good:**
- `sre-kubernetes-health-checker`
- `security-vuln-scanner-nist`
- `devops-ci-runner`

**Avoid:**
- `scanner` (too generic)
- `agent1` (meaningless)
- `helper` (will conflict)

**Pattern:** `{team}-{domain}-{function}`
```

#### 5. Conflict Resolution Options

If conflict occurs, user can:

1. **Rename agent** - Change `name` in agent definition
2. **Use different lattice** - Connect to separate lattice network
3. **Accept local-only** - Agent works locally, not exposed to lattice
4. **Coordinate with team** - Agree on naming across stations

### Behavior Summary

| Scenario | Behavior |
|----------|----------|
| First station registers "Scanner" | Success |
| Second station tries "Scanner" | Warning, agent excluded from lattice |
| Second station's "Scanner" | Works locally, invisible to lattice |
| assign_work(agent="Scanner") from any station | Routes to first station |
| Local call on second station | Uses local "Scanner" (local-first) |

---

## Phase 5B: Distributed Run Tracking with UUID

### Problem Statement

Currently, each Station uses SQLite auto-increment IDs for agent runs. In a distributed lattice:
- Run ID `42` on Station A has no relation to Run ID `42` on Station B
- Cannot correlate a distributed execution flow
- No way to track parent-child relationships across stations
- OTEL traces exist but lack application-level correlation

**Current State:**
```
Station A: Run #42 calls remote agent on Station B
Station B: Creates Run #127 (no link to #42)
           └── How do we know these are related?
```

**Desired State:**
```
Orchestrator generates: 550e8400-e29b-41d4-a716-446655440000 (UUID)
Station A: Run #42, orchestrator_run_id="550e8400-...", parent=null (root)
    └── Calls remote agent
Station B: Run #127, orchestrator_run_id="550e8400-...-1", parent="550e8400-..."
    └── Calls another agent
Station B: Run #128, orchestrator_run_id="550e8400-...-1-1", parent="550e8400-...-1"
    └── Full lineage preserved with UUID hierarchy
```

### Architecture Design

#### New Fields for agent_runs Table

```sql
-- Migration: 048_add_orchestrator_run_tracking.sql

ALTER TABLE agent_runs ADD COLUMN orchestrator_run_id TEXT;
ALTER TABLE agent_runs ADD COLUMN parent_orchestrator_run_id TEXT;
ALTER TABLE agent_runs ADD COLUMN originating_station_id TEXT;
ALTER TABLE agent_runs ADD COLUMN trace_id TEXT;    -- OTEL trace ID for correlation
ALTER TABLE agent_runs ADD COLUMN work_id TEXT;     -- Lattice work ID (links to WorkAssignment)

CREATE INDEX idx_agent_runs_orchestrator_run_id ON agent_runs(orchestrator_run_id);
CREATE INDEX idx_agent_runs_parent_orchestrator_run_id ON agent_runs(parent_orchestrator_run_id);
CREATE INDEX idx_agent_runs_work_id ON agent_runs(work_id);
```

#### Orchestrator Run ID Format

```
Format: {uuid}[-{child_index}]

Examples:
  550e8400-e29b-41d4-a716-446655440000           # Root run (initiated by user)
  550e8400-e29b-41d4-a716-446655440000-1         # First child
  550e8400-e29b-41d4-a716-446655440000-2         # Second child
  550e8400-e29b-41d4-a716-446655440000-2-1       # Child of second child
```

Using UUID v4 (random) or UUID v7 (time-ordered):
- Universally unique across all stations
- No coordination needed between stations
- UUID v7 preferred for time-ordering if available
- 36 character string (with hyphens)

```go
// internal/lattice/context.go

import "github.com/google/uuid"

func GenerateOrchestratorRunID() string {
    // Use UUID v7 if available (time-ordered), otherwise v4
    return uuid.NewString()
}

func (c *OrchestratorContext) GenerateChildRunID(childIndex int) string {
    return fmt.Sprintf("%s-%d", c.RootRunID, childIndex)
}
```

#### Context Propagation

##### 1. OrchestratorContext

```go
// internal/lattice/context.go

type OrchestratorContext struct {
    RunID              string    // This execution's orchestrator run ID (UUID or UUID-N)
    ParentRunID        string    // Parent's orchestrator run ID (empty if root)
    RootRunID          string    // Original root run ID (always a UUID)
    OriginatingStation string    // Station that initiated the root run
    Depth              int       // Nesting depth (0 = root)
    ChildIndex         int       // Index among siblings (for generating child IDs)
    TraceID            string    // OTEL trace ID for correlation
    WorkID             string    // Current work ID (links to WorkAssignment)
}

// Generate new orchestrator run ID for child execution
func (c *OrchestratorContext) NewChildContext(childIndex int) *OrchestratorContext {
    childRunID := fmt.Sprintf("%s-%d", c.RootRunID, childIndex)
    return &OrchestratorContext{
        RunID:              childRunID,
        ParentRunID:        c.RunID,
        RootRunID:          c.RootRunID,
        OriginatingStation: c.OriginatingStation,
        Depth:              c.Depth + 1,
        ChildIndex:         childIndex,
        TraceID:            c.TraceID,
    }
}

// Create root context for new orchestrated execution
func NewRootOrchestratorContext(stationID, traceID string) *OrchestratorContext {
    rootRunID := uuid.NewString() // UUID v4
    return &OrchestratorContext{
        RunID:              rootRunID,
        ParentRunID:        "",
        RootRunID:          rootRunID,
        OriginatingStation: stationID,
        Depth:              0,
        ChildIndex:         0,
        TraceID:            traceID,
    }
}

// Context key for propagation
type orchestratorContextKey struct{}

func WithOrchestratorContext(ctx context.Context, oc *OrchestratorContext) context.Context {
    return context.WithValue(ctx, orchestratorContextKey{}, oc)
}

func GetOrchestratorContext(ctx context.Context) *OrchestratorContext {
    if oc, ok := ctx.Value(orchestratorContextKey{}).(*OrchestratorContext); ok {
        return oc
    }
    return nil
}
```

##### 2. Work Messages Already Include Orchestrator Context

The `WorkAssignment` and `WorkResponse` types (defined in Phase 5A.3) already include orchestrator tracking fields:

```go
// From internal/lattice/work/messages.go (see Phase 5A.3)

type WorkAssignment struct {
    WorkID            string            `json:"work_id"`         // UUID for this work unit
    OrchestratorRunID string            `json:"orchestrator_run_id"`
    ParentWorkID      string            `json:"parent_work_id,omitempty"`
    
    // ... other fields ...
    
    // Tracing (propagated through entire chain)
    TraceID           string            `json:"trace_id,omitempty"`
    SpanID            string            `json:"span_id,omitempty"`
}

type WorkResponse struct {
    WorkID            string            `json:"work_id"`
    OrchestratorRunID string            `json:"orchestrator_run_id"`
    
    // ... other fields ...
    
    LocalRunID        int64             `json:"local_run_id,omitempty"`  // Station's SQLite ID
}
```

The orchestrator context flows through the async work assignment chain:
1. Root work assignment includes new `OrchestratorRunID` (UUID)
2. Each child work assignment includes `ParentWorkID` linking to parent
3. All responses include both `WorkID` and `OrchestratorRunID` for correlation

##### 3. Run Creation with Orchestrator Context

```go
// internal/lattice/executor_adapter.go

func (e *ExecutorAdapter) ExecuteAgentByID(
    ctx context.Context,
    agentID string,
    task string,
    orchCtx *OrchestratorContext,
) (string, int, error) {
    
    id, err := strconv.ParseInt(agentID, 10, 64)
    if err != nil {
        return "", 0, fmt.Errorf("invalid agent ID '%s': %w", agentID, err)
    }

    userID := int64(1)
    
    // Create run with orchestrator tracking
    agentRun, err := e.repos.AgentRuns.CreateWithOrchestratorContext(
        ctx,
        id,
        userID,
        task,
        orchCtx.RunID,
        orchCtx.ParentRunID,
        orchCtx.OriginatingStation,
        orchCtx.TraceID,
    )
    if err != nil {
        return "", 0, fmt.Errorf("failed to create agent run: %w", err)
    }

    // Propagate context to child executions
    ctx = WithOrchestratorContext(ctx, orchCtx)
    
    result, err := e.agentService.ExecuteAgentWithRunID(ctx, id, task, agentRun.ID, nil)
    // ... rest of execution
}
```

#### Visual: Distributed Run Tracking (Async)

```
User Request: "Analyze security of my infrastructure"
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Station A (Orchestrator)                                           │
│  ┌────────────────────────────────────────────────────────────┐    │
│  │ Run #42                                                     │    │
│  │ orchestrator_run_id: "550e8400-e29b-41d4-a716-446655440000"│    │
│  │ parent_orchestrator_run_id: null (ROOT)                    │    │
│  │ originating_station_id: "station-a"                        │    │
│  │ trace_id: "abc123def456789"                                │    │
│  │                                                             │    │
│  │ Agent: SecurityCoordinator                                  │    │
│  │ Task: "Analyze security of my infrastructure"              │    │
│  └────────────────────────────────────────────────────────────┘    │
│                              │                                      │
│              ┌───────────────┴───────────────┐                     │
│              ▼                               ▼                      │
│  work1 = assign_work(           work2 = assign_work(               │
│    agent="NetworkScanner",        agent="VulnScanner",             │
│    task="audit network")          task="scan for vulns")          │
│         (local)                         (remote)                    │
│              │                               │                      │
│              │  ← executes async             │ WORK_ASSIGNED        │
│              ▼                               ▼ via NATS             │
└──────────────┼───────────────────────────────┼──────────────────────┘
               │                               │
┌──────────────────────────────┐               │
│  Station A (Local Execution) │               │
│  ┌─────────────────────────┐ │               │
│  │ Run #43                 │ │               │
│  │ work_id: "work-uuid-1"  │ │               │
│  │ orch_run: "550e8400..-1"│ │               │
│  │ parent: "550e8400..."   │ │               │
│  │ origin: "station-a"     │ │               │
│  │ trace: "abc123def456789"│ │               │
│  │                         │ │               │
│  │ → Sends WORK_COMPLETE   │ │               │
│  └─────────────────────────┘ │               │
└──────────────────────────────┘               │
                                               ▼
                              ┌─────────────────────────────────────┐
                              │  Station B (Leaf Station)           │
                              │  ┌────────────────────────────────┐ │
                              │  │ Run #127                       │ │
                              │  │ work_id: "work-uuid-2"         │ │
                              │  │ orch_run: "550e8400...-2"      │ │
                              │  │ parent: "550e8400..."          │ │
                              │  │ origin: "station-a"            │ │
                              │  │ trace: "abc123def456789"       │ │
                              │  │                                │ │
                              │  │ Agent: VulnScanner             │ │
                              │  │ (Picks up from hook, executes) │ │
                              │  └────────────────────────────────┘ │
                              │                │                    │
                              │                ▼                    │
                              │   assign_work(agent="CVELookup")    │
                              │        (local to Station B)         │
                              │                │                    │
                              │  ┌────────────────────────────────┐ │
                              │  │ Run #128                       │ │
                              │  │ work_id: "work-uuid-2-1"       │ │
                              │  │ orch_run: "550e8400...-2-1"    │ │
                              │  │ parent: "550e8400...-2"        │ │
                              │  │ origin: "station-a"            │ │
                              │  │ trace: "abc123def456789"       │ │
                              │  │                                │ │
                              │  │ → Sends WORK_COMPLETE          │ │
                              │  └────────────────────────────────┘ │
                              │                                     │
                              │  Both results stream back to        │
                              │  via WORK_COMPLETE messages         │
                              └─────────────────────────────────────┘
```

#### Query Examples

```sql
-- Get full execution tree for an orchestrator run (single station)
SELECT 
    orchestrator_run_id,
    parent_orchestrator_run_id,
    originating_station_id,
    agent_id,
    task,
    status,
    started_at,
    completed_at,
    duration_seconds
FROM agent_runs
WHERE orchestrator_run_id LIKE '550e8400-e29b-41d4-a716-446655440000%'
ORDER BY orchestrator_run_id;

-- Get root run and all children
SELECT * FROM agent_runs
WHERE orchestrator_run_id = '550e8400-e29b-41d4-a716-446655440000'  -- root
   OR parent_orchestrator_run_id LIKE '550e8400-e29b-41d4-a716-446655440000%';

-- Get all runs from a distributed execution (across all stations)
-- This requires querying each station or a central aggregator
-- Each station stores its portion of the execution tree
```

#### OTEL Trace Correlation

The `trace_id` field links to OTEL:

```go
func (e *ExecutorAdapter) ExecuteAgentByID(ctx context.Context, ...) {
    // Extract OTEL trace ID from context
    span := trace.SpanFromContext(ctx)
    traceID := span.SpanContext().TraceID().String()
    
    orchCtx := &OrchestratorContext{
        RunID:   generateOrchestratorRunID(),
        TraceID: traceID,
        // ...
    }
    
    // Now OTEL traces and orchestrator runs are correlated
    // Query Jaeger by trace_id, then find all related runs in DB
}
```

### Migration Path

1. **Add new columns** (nullable) - no breaking changes
2. **Update run creation** to populate orchestrator fields when available
3. **Existing runs** have null orchestrator fields (standalone/legacy)
4. **New lattice runs** have full orchestrator context

---

## Implementation Order

### Phase 5A.1: Agent Discovery & Schema
1. Extend `AgentRegistration` struct with full schema info
2. Update registry sync to include schemas
3. Create `list_available_agents` Genkit tool
4. Create `get_agent_schema` Genkit tool
5. Test: Agents can discover and introspect remote agents

### Phase 5A.2: Unique Agent Names
1. Add name conflict detection to `Registry.RegisterStation()`
2. Create `AgentNameConflictError` type
3. Update `Presence.Start()` to handle conflicts gracefully
4. Add CLI warnings for name conflicts
5. Test: Second station with duplicate name gets warning

### Phase 5A.3: Async Work Assignment
1. Define NATS message types in `internal/lattice/work/messages.go`:
   - `WORK_ASSIGNED`, `WORK_ACCEPTED`, `WORK_PROGRESS`
   - `WORK_COMPLETE`, `WORK_FAILED`, `WORK_ESCALATE`
2. Create `WorkDispatcher` in `internal/lattice/work/dispatcher.go`:
   - `AssignWork()` - non-blocking work assignment
   - `AwaitWork()` - blocking wait for completion
   - `CheckWork()` - non-blocking status check
   - `StreamProgress()` - channel for progress updates
3. Create `WorkHook` in `internal/lattice/work/hook.go`:
   - Subscribe to `lattice.*.station.{station_id}.work.assign`
   - Pick up work immediately and execute
   - Execute agent and stream responses
4. Create Genkit tools:
   - `assign_work` - assign work, returns work_id immediately
   - `await_work` - block until work completes
   - `check_work` - poll status without blocking
   - `escalate` - agent escalates back to orchestrator
5. Integrate into `AgentExecutionEngine`:
   - Inject work tools when lattice enabled
   - Wire WorkDispatcher and WorkHook
6. Add configuration for work assignment settings
7. E2E tests:
   - Single station: assign_work → await_work cycle
   - Multi-station: assign remote work, receive results
   - Parallel work: assign 3 agents, await all results
   - Escalation: agent escalates, orchestrator handles

### Phase 5B: Distributed Run Tracking with UUID
1. Create migration `048_add_orchestrator_run_tracking.sql`:
   - `orchestrator_run_id` (TEXT) - UUID or UUID-N format
   - `parent_orchestrator_run_id` (TEXT) - parent's run ID
   - `originating_station_id` (TEXT) - station that started root
   - `trace_id` (TEXT) - OTEL trace correlation
   - `work_id` (TEXT) - lattice work ID reference
2. Update sqlc queries for new fields
3. Create `OrchestratorContext` with UUID generation:
   - `NewRootOrchestratorContext()` - for user-initiated runs
   - `NewChildContext()` - for work assigned to other agents
4. Update `WorkAssignment` message with orchestrator fields
5. Update `ExecutorAdapter` to propagate orchestrator context
6. Add OTEL trace ID correlation
7. E2E test: Verify UUID-based run IDs flow through distributed async execution

### Phase 5C: Integration & CLI
1. Combine all features end-to-end
2. Add CLI command: `stn lattice runs --uuid <id>` to query distributed runs
3. Add CLI command: `stn lattice agents` to list all agents with schemas
4. Add observability dashboard recommendations
5. Performance testing with deep call chains (5+ levels)

---

## Success Metrics

| Metric | Target |
|--------|--------|
| Local-first routing latency | < 1ms overhead |
| Remote invocation latency | < 50ms + network RTT |
| Orchestrator context propagation | 100% of lattice calls |
| OTEL trace correlation | All runs linked to traces |
| E2E test: 3-station call chain | Pass with full lineage |

---

## Open Questions

1. **Central Aggregator**: Should there be a central service to aggregate runs across all stations for unified querying?
2. **UUID Version**: Use UUID v4 (random) or UUID v7 (time-ordered) for root run IDs?
3. **Timeout Handling**: How to handle partial execution when remote station times out?
4. **Replay/Recovery**: Can we replay a distributed execution from orchestrator context?
5. **Schema Caching**: How long to cache remote agent schemas? Invalidation strategy?
6. **Name Reservation**: Should there be a way to "reserve" agent names before deployment?

---

## References

- `internal/lattice/invoker.go` - Current remote invocation
- `internal/lattice/router.go` - Agent/workflow routing
- `internal/lattice/executor_adapter.go` - Local execution adapter
- `internal/services/agent_execution_engine.go` - Tool loading and execution
- `internal/db/repositories/agent_runs.go` - Current run tracking
