# Station Runtime Internals

This document covers the internal architecture of Station's runtime systems for developers contributing to Station or debugging complex issues.

## Table of Contents

- [Container Architecture (`stn up`)](#container-architecture-stn-up)
- [Workflow Engine & NATS Messaging](#workflow-engine--nats-messaging)
- [Sandbox Execution](#sandbox-execution)

---

## Container Architecture (`stn up`)

### Overview

The `stn up` command orchestrates a complete Station environment in Docker. It handles image building, volume management, bundle installation, and service startup.

### Architecture Diagram

```mermaid
graph TB
    subgraph "Host Machine"
        CLI[stn up command]
        DockerD[Docker Daemon]
        HostConfig[~/.config/station/config.yaml]
        Workspace[User Workspace]
    end
    
    subgraph "Docker Volumes"
        ConfigVol[station-config volume]
        CacheVol[station-cache volume]
    end
    
    subgraph "station-server Container"
        subgraph "Services"
            API[API Server :8585]
            MCP[MCP Server :8586]
            AgentMCP[Agent MCP :8587]
            Scheduler[Cron Scheduler]
        end
        
        subgraph "Core Components"
            DB[(SQLite Database)]
            GenKit[GenKit AI Engine]
            DecSync[DeclarativeSync]
        end
        
        subgraph "Runtime"
            NATS[Embedded NATS]
            WorkflowEngine[Workflow Engine]
            SandboxMgr[Sandbox Manager]
        end
    end
    
    subgraph "Sidecar Containers"
        Jaeger[Jaeger :16686]
        SandboxC[Sandbox Containers]
    end
    
    CLI -->|1. Check Docker| DockerD
    CLI -->|2. Import config| HostConfig
    HostConfig -->|copied to| ConfigVol
    CLI -->|3. Create container| DockerD
    
    DockerD -->|mount| ConfigVol
    DockerD -->|mount| CacheVol
    DockerD -->|mount| Workspace
    
    API --> DB
    MCP --> GenKit
    AgentMCP --> GenKit
    WorkflowEngine --> NATS
    SandboxMgr -->|Docker API| SandboxC
    
    API -->|traces| Jaeger
    MCP -->|traces| Jaeger
```

### Startup Sequence

Detailed initialization flow when `stn serve` runs inside the container:

```mermaid
sequenceDiagram
    participant CLI as stn up
    participant Docker
    participant Container as station-server
    participant Serve as stn serve
    participant DB as SQLite
    participant NATS as Embedded NATS
    participant GenKit
    participant API as API Server
    
    CLI->>Docker: docker run station-server
    Docker->>Container: Start container
    Container->>Serve: Execute stn serve
    
    rect rgb(240, 248, 255)
        Note over Serve,DB: Database Initialization
        Serve->>DB: Open/Create database
        Serve->>DB: Run migrations
        Serve->>DB: Ensure default environment
    end
    
    rect rgb(255, 248, 240)
        Note over Serve,GenKit: AI Provider Setup
        Serve->>GenKit: Initialize with provider config
        GenKit-->>Serve: AI client ready
    end
    
    rect rgb(240, 255, 240)
        Note over Serve,NATS: Workflow Engine Setup
        Serve->>NATS: Start embedded NATS server
        NATS-->>Serve: NATS ready
        Serve->>Serve: Create WorkflowEngine
        Serve->>Serve: Create WorkflowService
        Serve->>Serve: Start WorkflowConsumer
    end
    
    rect rgb(255, 240, 255)
        Note over Serve,API: DeclarativeSync
        Serve->>DB: Scan environments/*/mcp-configs/*.json
        Serve->>DB: Connect MCP servers, discover tools
        Serve->>DB: Scan environments/*/agents/*.prompt
        Serve->>DB: Parse prompts, create agent records
    end
    
    Serve->>API: Start API server :8585
    Serve->>API: Start MCP server :8586
    Serve->>API: Start Agent MCP :8587
    Serve->>API: Start scheduler service
    
    API-->>CLI: Ready for connections
```

### Shared Workflow Engine Pattern

A critical architectural detail: the container uses a **single shared workflow engine** for both HTTP API and internal operations. This ensures messages published via HTTP API are processed by the same consumer:

```mermaid
graph LR
    subgraph "server.go (Container Entry)"
        Init[Initialize Components]
        Engine[WorkflowEngine]
        Service[WorkflowService]
        Consumer[WorkflowConsumer]
    end
    
    subgraph "API Handlers"
        HTTP[HTTP POST /workflow-runs]
        Handlers[APIHandlers]
    end
    
    subgraph "NATS Messaging"
        Queue[workflow.run.*.step.*.schedule]
    end
    
    Init -->|creates| Engine
    Init -->|creates with engine| Service
    Init -->|creates| Consumer
    Init -->|passes components| Handlers
    
    HTTP -->|uses shared| Service
    Service -->|publishes to| Engine
    Engine -->|writes to| Queue
    Consumer -->|reads from| Queue
    Consumer -->|executes steps| Handlers
```

> **Note:** Prior to commit `a6878261`, HTTP API handlers created their own separate engine, causing workflows started via API to be stuck in "pending" status.

### Volume Data Layout

Understanding what's stored in the `station-config` volume:

```
/home/station/.config/station/
├── config.yaml                    # Main configuration
├── station.db                     # SQLite database
├── environments/
│   ├── default/
│   │   ├── agents/                # .prompt files
│   │   │   ├── my-agent.prompt
│   │   │   └── coordinator.prompt
│   │   ├── mcp-configs/           # MCP server configs
│   │   │   ├── github.json
│   │   │   └── slack.json
│   │   ├── workflows/             # Workflow definitions
│   │   │   └── incident-rca.workflow.yaml
│   │   └── variables.yml          # Environment variables
│   └── production/
│       └── ...
└── bundles/                       # Installed bundles
    └── finops-v1.0.0/
```

---

## Workflow Engine & NATS Messaging

### NATS Architecture Overview

Station uses an embedded NATS JetStream server for durable, reliable workflow execution. All workflow steps are published as messages and processed by a consumer pool.

```mermaid
graph TB
    subgraph "Workflow Execution Entry Points"
        HTTP[HTTP POST /workflow-runs]
        CLI[stn workflow run]
        MCP[MCP start_workflow_run]
    end
    
    subgraph "Workflow Service"
        WS[WorkflowService]
        WE[WorkflowEngine]
    end
    
    subgraph "Embedded NATS JetStream"
        Stream[(WORKFLOWS Stream)]
        Subject1[workflow.run.{run_id}.step.{step_id}.schedule]
    end
    
    subgraph "Consumer Pool"
        Consumer[WorkflowConsumer]
        W1[Worker 1]
        W2[Worker 2]
        W3[Worker 3]
        WN[Worker N...]
    end
    
    subgraph "Step Executors"
        Agent[AgentExecutor]
        Switch[SwitchExecutor]
        Transform[TransformExecutor]
        Parallel[ParallelExecutor]
        Foreach[ForeachExecutor]
        Approval[HumanApprovalExecutor]
    end
    
    HTTP --> WS
    CLI --> WS
    MCP --> WS
    
    WS --> WE
    WE -->|publish| Stream
    Stream -->|subscribe| Consumer
    
    Consumer --> W1
    Consumer --> W2
    Consumer --> W3
    Consumer --> WN
    
    W1 --> Agent
    W2 --> Switch
    W3 --> Transform
    W1 --> Parallel
    W2 --> Foreach
    W3 --> Approval
```

### Message Flow: Step-by-Step

When a workflow runs, each step follows this message pattern:

```mermaid
sequenceDiagram
    participant Client as HTTP/CLI/MCP
    participant WS as WorkflowService
    participant Engine as NATSEngine
    participant Stream as JetStream
    participant Consumer as WorkflowConsumer
    participant Worker as Worker Pool
    participant Executor as Step Executor
    participant DB as SQLite
    
    Client->>WS: StartWorkflowRun(workflow_id, input)
    WS->>DB: Create workflow_run record (status=pending)
    WS->>Engine: ScheduleStep(run_id, first_step_id)
    
    rect rgb(240, 248, 255)
        Note over Engine,Stream: NATS Publishing
        Engine->>Stream: Publish to workflow.run.{run_id}.step.{step_id}.schedule
        Engine->>Engine: Include trace context in message headers
    end
    
    Stream-->>Consumer: Pull message (durable consumer)
    Consumer->>Worker: Dispatch to available worker
    
    rect rgb(255, 248, 240)
        Note over Worker,Executor: Step Execution
        Worker->>DB: Update step status = running
        Worker->>Executor: Execute step (agent/switch/transform/etc)
        Executor->>Executor: Run agent, evaluate expression, etc.
        Executor-->>Worker: Step result
        Worker->>DB: Update step status = completed, store output
    end
    
    rect rgb(240, 255, 240)
        Note over Worker,Engine: Next Step Scheduling
        Worker->>Worker: Determine next step from transitions
        alt Has next step
            Worker->>Engine: ScheduleStep(run_id, next_step_id)
            Engine->>Stream: Publish next step message
        else Workflow complete
            Worker->>DB: Update workflow_run status = completed
        end
    end
    
    Stream-->>Consumer: Next step message...
    Note over Consumer: Cycle continues until workflow ends
```

### Subject Naming Convention

NATS subjects follow a hierarchical pattern for routing and filtering:

| Subject Pattern | Description | Example |
|-----------------|-------------|---------|
| `workflow.run.{run_id}.step.{step_id}.schedule` | Schedule a step for execution | `workflow.run.abc123.step.analyze.schedule` |
| `workflow.run.{run_id}.cancel` | Cancel a running workflow | `workflow.run.abc123.cancel` |
| `workflow.approval.{approval_id}` | Approval-related messages | `workflow.approval.xyz789` |

### Consumer Configuration

The workflow consumer uses JetStream's durable consumer for reliability:

```mermaid
graph LR
    subgraph "JetStream Configuration"
        Stream[WORKFLOWS Stream<br/>Retention: WorkQueue<br/>MaxAge: 24h]
        Consumer[station-workflow Consumer<br/>AckPolicy: Explicit<br/>MaxAckPending: 1000<br/>MaxDeliver: 3]
    end
    
    subgraph "Worker Pool"
        Pool[10 Concurrent Workers]
        Timeout[Per-step timeout: 5m]
        Retry[Retry on failure: 3x]
    end
    
    Stream --> Consumer
    Consumer --> Pool
```

**Key Configuration:**
- **AckPolicy: Explicit** - Messages must be explicitly acknowledged after processing
- **MaxAckPending: 1000** - Up to 1000 messages can be in-flight
- **MaxDeliver: 3** - Failed messages retry up to 3 times before dead-lettering
- **WorkQueue Retention** - Messages removed after acknowledgment

### Parallel and Foreach Execution

Parallel and foreach states spawn multiple sub-messages:

```mermaid
sequenceDiagram
    participant Consumer
    participant Parallel as ParallelExecutor
    participant Engine as NATSEngine
    participant Stream as JetStream
    
    Consumer->>Parallel: Execute parallel state
    
    rect rgb(240, 248, 255)
        Note over Parallel,Stream: Spawn Branch Messages
        Parallel->>Engine: ScheduleStep(branch1.first_step)
        Parallel->>Engine: ScheduleStep(branch2.first_step)
        Parallel->>Engine: ScheduleStep(branch3.first_step)
        Engine->>Stream: Publish 3 messages (concurrent)
    end
    
    Note over Consumer,Stream: All 3 branches execute in parallel
    
    rect rgb(255, 248, 240)
        Note over Parallel: Join Point
        Parallel->>Parallel: Wait for all branches
        Parallel->>Parallel: Aggregate results
    end
    
    Parallel-->>Consumer: Combined branch outputs
```

### Trace Context Propagation

OpenTelemetry trace context is preserved across NATS messages:

```mermaid
graph LR
    subgraph "Parent Span"
        HTTP[HTTP Request<br/>trace_id: abc123]
        WS[WorkflowService]
    end
    
    subgraph "NATS Message"
        Headers[Message Headers<br/>traceparent: 00-abc123-...<br/>tracestate: ...]
    end
    
    subgraph "Child Span"
        Consumer[Consumer]
        Step[Step Execution<br/>trace_id: abc123<br/>parent: workflow span]
    end
    
    HTTP --> WS
    WS -->|inject context| Headers
    Headers -->|extract context| Consumer
    Consumer --> Step
```

This enables end-to-end distributed tracing in Jaeger, showing the complete workflow execution path including all step transitions.

### Error Handling and Recovery

```mermaid
stateDiagram-v2
    [*] --> Pending: Create workflow run
    Pending --> Running: First step scheduled
    
    Running --> Running: Step completes, next step scheduled
    Running --> Paused: Pause requested or approval pending
    Running --> Failed: Step error (after retries)
    Running --> Completed: Final step completes
    
    Paused --> Running: Resume or approval granted
    Paused --> Cancelled: Cancel requested
    
    Failed --> [*]
    Completed --> [*]
    Cancelled --> [*]
```

### Stale Run Recovery

On startup, the workflow consumer recovers pending runs that were interrupted:

```go
// Pseudo-code for recovery logic
runs := db.GetWorkflowRuns(status: "running", olderThan: 4h)
for _, run := range runs {
    if run.Age > staleThreshold {
        log.Warn("Skipping stale run", run.ID)
        continue
    }
    // Resume from last completed step
    lastStep := db.GetLastCompletedStep(run.ID)
    engine.ScheduleStep(run.ID, lastStep.NextStep)
}
```

> **Note:** Runs older than 4 hours are considered stale and skipped during recovery. This prevents infinite retries of broken workflows.

---

## Sandbox Execution

### Architecture Overview

Station supports two sandbox backends that manage isolated execution environments:

```mermaid
graph TB
    subgraph "Station Process"
        Agent[Agent Execution]
        SandboxMgr[Sandbox Manager]
        ToolReg[Tool Registry]
    end
    
    subgraph "Compute Mode (Dagger)"
        Dagger[Dagger Engine]
        DaggerC1[Ephemeral Container 1]
        DaggerC2[Ephemeral Container 2]
    end
    
    subgraph "Code Mode (Docker)"
        DockerAPI[Docker API]
        CodeC1[Persistent Container 1<br/>Session: workflow-abc]
        CodeC2[Persistent Container 2<br/>Session: workflow-xyz]
    end
    
    subgraph "NATS JetStream"
        ObjStore[(Object Store<br/>File Staging)]
    end
    
    Agent --> SandboxMgr
    SandboxMgr --> ToolReg
    
    ToolReg -->|sandbox_run| Dagger
    Dagger --> DaggerC1
    Dagger --> DaggerC2
    
    ToolReg -->|sandbox_open/exec| DockerAPI
    DockerAPI --> CodeC1
    DockerAPI --> CodeC2
    
    SandboxMgr <-->|stage/publish| ObjStore
```

### Compute Mode: Ephemeral Execution

Compute mode uses Dagger for ephemeral, single-call code execution:

```mermaid
sequenceDiagram
    participant Agent
    participant Tool as sandbox_run Tool
    participant Dagger as Dagger Engine
    participant Container as Ephemeral Container
    
    Agent->>Tool: sandbox_run(code, runtime="python")
    
    rect rgb(240, 248, 255)
        Note over Tool,Dagger: Container Setup
        Tool->>Dagger: Create container from python:3.11-slim
        Dagger->>Container: Start container
        
        opt Has pip_packages
            Tool->>Container: pip install packages
        end
        
        opt Has files parameter
            Tool->>Container: Write input files
        end
    end
    
    rect rgb(255, 248, 240)
        Note over Tool,Container: Execution
        Tool->>Container: Execute code
        Container-->>Tool: stdout, stderr, exit_code
    end
    
    rect rgb(240, 255, 240)
        Note over Tool,Container: Cleanup
        Tool->>Dagger: Destroy container
        Dagger->>Container: Remove
    end
    
    Tool-->>Agent: Execution result
```

**Key Characteristics:**
- Container created fresh for each `sandbox_run` call
- No state persists between calls
- Fast for simple computations
- Automatic cleanup

### Code Mode: Persistent Sessions

Code mode uses Docker directly for persistent, session-based sandboxes:

```mermaid
sequenceDiagram
    participant Agent
    participant Open as sandbox_open
    participant Exec as sandbox_exec
    participant FS as sandbox_fs_*
    participant Docker as Docker API
    participant Container as Persistent Container
    participant Store as NATS Object Store
    
    Agent->>Open: sandbox_open()
    Open->>Docker: docker create ubuntu:22.04
    Docker->>Container: Create and start
    Docker-->>Open: Container ID
    Open-->>Agent: session_id
    
    Agent->>FS: sandbox_stage_file(file_key, dest)
    FS->>Store: Get file content
    Store-->>FS: File bytes
    FS->>Container: docker cp to /workspace/dest
    FS-->>Agent: Success
    
    Agent->>Exec: sandbox_exec("pip install pandas")
    Exec->>Container: docker exec pip install pandas
    Container-->>Exec: stdout/stderr
    Exec-->>Agent: Result
    
    Agent->>Exec: sandbox_exec("python analyze.py")
    Exec->>Container: docker exec python analyze.py
    Container-->>Exec: stdout/stderr
    Exec-->>Agent: Result
    
    Agent->>FS: sandbox_publish_file(source)
    FS->>Container: docker cp from /workspace/source
    Container-->>FS: File bytes
    FS->>Store: Store file
    Store-->>FS: file_key
    FS-->>Agent: file_key
    
    Note over Agent,Container: Session ends (workflow complete or explicit close)
    
    Agent->>Open: sandbox_close(session_id)
    Open->>Docker: docker rm -f container
    Docker-->>Open: Removed
```

### Session Scoping: Workflow vs Agent

The `session` configuration determines container lifecycle:

```mermaid
graph TB
    subgraph "Workflow Scope (session: workflow)"
        WF[Workflow: code-qa-fix]
        A1[Agent 1: coder<br/>sandbox_fs_write main.py]
        A2[Agent 2: qa-engineer<br/>sandbox_exec pytest]
        A3[Agent 3: coder<br/>sandbox_exec fix]
        SC1[Shared Container<br/>Files persist across agents]
        
        WF --> A1
        WF --> A2
        WF --> A3
        A1 --> SC1
        A2 --> SC1
        A3 --> SC1
    end
    
    subgraph "Agent Scope (session: agent)"
        WF2[Workflow: parallel-analysis]
        B1[Agent 1: analyzer<br/>Own container]
        B2[Agent 2: validator<br/>Own container]
        IC1[Container 1<br/>Isolated]
        IC2[Container 2<br/>Isolated]
        
        WF2 --> B1
        WF2 --> B2
        B1 --> IC1
        B2 --> IC2
    end
```

**Workflow Scope:**
- Single container shared across all agents in workflow
- Files written by Agent 1 are visible to Agent 2
- Container destroyed when workflow completes
- Ideal for: Build → Test → Fix pipelines

**Agent Scope:**
- Fresh container for each agent execution
- Complete isolation between agents
- Container destroyed when agent completes
- Ideal for: Parallel independent tasks

### File Staging Architecture

Large files are staged through NATS Object Store to avoid passing binary data through LLM context:

```mermaid
graph LR
    subgraph "Upload Flow"
        CLI[stn files upload data.csv]
        Upload[Upload Handler]
        Chunk[Chunk into parts]
        Store[(NATS Object Store<br/>files/f_abc123)]
    end
    
    subgraph "Stage Flow"
        Agent[Agent]
        Stage[sandbox_stage_file]
        Fetch[Fetch from store]
        Container[Sandbox Container<br/>/workspace/data.csv]
    end
    
    subgraph "Publish Flow"
        Read[sandbox_publish_file]
        Extract[Read from container]
        StoreOut[(NATS Object Store<br/>files/f_xyz789)]
        Download[stn files download]
    end
    
    CLI --> Upload --> Chunk --> Store
    Agent --> Stage --> Fetch
    Store --> Fetch --> Container
    Container --> Extract --> StoreOut
    StoreOut --> Download
```

### Resource Limits and Security

```mermaid
graph TB
    subgraph "Security Boundaries"
        subgraph "Station Container"
            SandboxMgr[Sandbox Manager]
            DockerSock[/var/run/docker.sock]
        end
        
        subgraph "Sandbox Container (Isolated)"
            NoPriv[No --privileged]
            NoSock[No Docker socket]
            NetOff[Network disabled by default]
            User[Non-root user]
            Limits[Resource limits enforced]
        end
    end
    
    SandboxMgr -->|creates| NoPriv
    DockerSock -.->|NOT mounted| NoSock
    
    subgraph "Default Limits"
        CPU[CPU: 2 cores]
        Mem[Memory: 2GB]
        Disk[Disk: 10GB]
        Time[Timeout: 5 minutes]
        Files[Max files: 100]
        FileSize[Max file size: 10MB]
    end
    
    Limits --> CPU
    Limits --> Mem
    Limits --> Disk
    Limits --> Time
    Limits --> Files
    Limits --> FileSize
```

**Security Features:**
- Unprivileged containers (no `--privileged` flag)
- No Docker socket access from within sandbox
- Network disabled by default (`allow_network: false`)
- Non-root user execution
- Resource limits (CPU, memory, disk, time)
- File count and size limits

> **Warning:** When `allow_network: true` is set, the sandbox can reach external services. Only enable this when necessary and ensure your agents handle untrusted network data safely.

### Container Image Selection

```mermaid
graph TD
    Config[Agent Sandbox Config]
    
    Config -->|runtime: linux| Ubuntu[ubuntu:22.04]
    Config -->|runtime: python| Python[python:3.11-slim]
    Config -->|runtime: node| Node[node:20-slim]
    Config -->|image: custom/image:tag| Custom[custom/image:tag]
    
    subgraph "Pre-installed Tools"
        Ubuntu -->|apt-get| UTools[build-essential, curl, git, vim]
        Python -->|pip| PTools[pip, wheel, setuptools]
        Node -->|npm| NTools[npm, yarn, pnpm]
    end
```

---

## Related Documentation

- [Architecture Index](./ARCHITECTURE_INDEX.md) - Quick navigation and key concepts
- [Architecture Diagrams](./ARCHITECTURE_DIAGRAMS.md) - Complete ASCII diagrams
- [Component Interactions](./COMPONENT_INTERACTIONS.md) - Detailed sequence diagrams
