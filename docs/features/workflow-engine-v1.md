# PRD: CloudShip Station - Workflow Orchestration Engine (V1)

> **Status**: Draft  
> **Created**: 2025-12-23  
> **Based on**: PR #83 (`origin/codex/add-durable-workflow-engine-to-station`)

## 1) Overview

### Architecture Context

Station has a layered architecture:

```
┌─────────────────────────────────────────────────────────────────┐
│                    WORKFLOW LAYER (Highest)                      │
│  - Open Serverless DSL definitions                               │
│  - NATS JetStream for durability, replay, steps, logs           │
│  - Schema validation: agent output → next agent input            │
│  - Executors: agent.run, human.approval                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                 AGENTS-AS-TOOLS LAYER (Middle)                   │
│  - Agents can call other agents as tools                         │
│  - Hierarchical agent composition                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                   PRIMITIVES LAYER (Foundation)                  │
│  - MCP Servers (tools, secrets via MCP mechanism)               │
│  - Agents (input/output schemas, dotprompt)                      │
│  - Sandbox (Dagger compute - GenKit native tool)                │
└─────────────────────────────────────────────────────────────────┘
```

**Key principle**: Workflows orchestrate agents. Agents do the work (via MCP tools, sandbox, etc.). Workflows don't directly call MCP tools or sandbox - that's the agents' job.

### Problem Statement

Station today can run individual AI agents and operational tools, but real-world DevOps work is rarely a single command. Teams need **repeatable, durable, multi-step procedures** with branching, parallelism, human approvals, and strong auditability.

PR #83 introduces foundational workflow scaffolding (DSL translation, SQLite persistence, NATS JetStream durable messaging). This PRD defines what it takes to complete that into a **production-ready** Workflow Orchestration Engine (WOE) for Station.

### Goals (V1)

1. **Durable workflows**: Workflow runs survive Station crashes/restarts and resume from the last confirmed step/state.
2. **Core state types**: Implement a minimal but powerful set:
   - `operation` (executes a single action)
   - `switch` (conditional routing via Starlark expressions)
   - `parallel` (fan-out/fan-in)
   - `inject` (mutate context with provided data)
   - `foreach` (iterate over a list)
3. **V1 Executors**:
   - `agent.run` → runs Station agent via AgentExecutionEngine
   - `human.approval` → approval gates with timeout
4. **Schema validation**: Validate that agent output schemas match next agent's input schemas for type-safe data flow.
5. **Human-in-loop controls**: Approval gates, pause/resume, signal delivery, and timeouts.
6. **Observability**: End-to-end tracing for workflow runs and per-step spans; durable run history for audit and debugging.
7. **Deployment compatibility**: Works in Docker Compose and common cloud platforms (ECS, GCP, Fly.io), with embedded or external NATS.

### Non-goals (V1)

- Full Serverless Workflow spec compliance (V1 is "inspired by" and compatible-by-design where feasible).
- `http.call` executor (agents can do HTTP via MCP tools).
- `sandbox.execute` executor (agents can use sandbox via GenKit tool).
- Secrets management (MCP mechanism handles secrets at primitives layer).
- Graphical workflow designer UI (API-first; UI can be layered later).
- Exactly-once execution guarantees (V1 uses **at-least-once** with idempotency).
- Distributed multi-node workflow scheduling with leader election.

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Expression language | **Starlark** | Good Go support, safe, deterministic |
| Database | **Same SQLite DB Station uses** | Single source of truth, simpler ops |
| Secrets | **MCP mechanism** | Already handled at primitives layer |
| V1 Executors | **agent.run + human.approval** | Workflows orchestrate agents, agents do work |

---

## 2) User Stories

### Primary Users

- **SRE/DevOps Engineer**: wants reliable incident playbooks and operational procedures.
- **Platform Engineer**: wants standardized workflows (deploy, rollback, rotate secrets).
- **Security Engineer**: wants gated workflows (approval required) with full audit logs.
- **Automation system**: triggers workflows via API/webhook.

### Stories

1. **Run a playbook durably**  
   As an SRE, I can start a workflow run (e.g., "diagnose API latency"), and if Station restarts mid-run, it resumes automatically without losing progress.

2. **Branch based on conditions**  
   As an operator, I can define switch logic like "if error_rate > 5% run mitigation else run reporting" using Starlark expressions.

3. **Parallelize checks**  
   As an SRE, I can run multiple diagnostic agents in parallel (e.g., check pods, logs, DB health) and proceed when all complete.

4. **Invoke agents as steps**  
   As an operator, I can call a Station agent (LLM-backed) inside a workflow and pass outputs to later steps.

5. **Schema-validated data flow**  
   As an agent author, I define input/output schemas for my agents, and the workflow engine validates that data flows correctly between steps.

6. **Approval gates**  
   As a security engineer, I can require human approval before disruptive actions (restart service, rotate secrets, delete infra).

7. **Auditing and replay**  
   As an org admin, I can view run history showing every step input/output, approvals, signals, timestamps, and who approved.

---

## 3) Technical Design

### 3.1 Workflow Definition Schema (Serverless Workflow-Inspired)

**Top-level fields (V1)**

```yaml
id: incident-triage           # stable workflow identifier
version: "1.0"
name: "Incident Triage Workflow"
description: "Automated incident diagnosis and remediation"
inputSchema:                   # JSON Schema for workflow input validation
  type: object
  properties:
    namespace:
      type: string
    service:
      type: string
  required: [namespace, service]
start: diagnostics             # name of starting state
states: [...]                  # state definitions
```

**Execution context model**

- `context` is a JSON object for the run, initialized from workflow input.
- Each state reads from `context` and writes results back via `resultPath`.
- V1 uses dot-path notation (`"steps.checkPods.result"`) for context access.

### 3.2 State Types (V1)

#### A) `operation`

Executes exactly one action using a built-in executor.

```yaml
- name: check_pods
  type: operation
  action: agent.run
  input:
    agent: "kubernetes-triage"
    task: "Check pods in namespace {{ ctx.namespace }}"
  timeoutSeconds: 300
  retry:
    maxAttempts: 3
    backoffSeconds: 5
  resultPath: "steps.checkPods"
  next: analyze_results
```

Fields:
- `action`: `"agent.run"` | `"human.approval"`
- `input`: payload mapped from context (supports `{{ ctx.path }}` templating)
- `timeoutSeconds`: step timeout
- `retry`: `{ maxAttempts, backoffSeconds }`
- `resultPath`: where to store result in context
- `next` / `end`: transition control

#### B) `switch`

Routes based on Starlark conditions.

```yaml
- name: decide_action
  type: switch
  dataPath: steps.analysis      # context path to evaluate
  conditions:
    - if: "result.error_rate > 0.05"
      next: request_approval
    - if: "result.status == 'degraded'"
      next: run_diagnostics
  defaultNext: report_ok
```

**Starlark integration**: Conditions are evaluated using [Starlark](https://github.com/google/starlark-go), a safe Python-like language with good Go support.

#### C) `parallel`

Fan-out to multiple branches and join.

```yaml
- name: run_diagnostics
  type: parallel
  branches:
    - name: check_pods
      states:
        - name: pod_check
          type: operation
          action: agent.run
          input:
            agent: "k8s-pod-checker"
            task: "Check pods in {{ ctx.namespace }}"
          resultPath: "branches.pods"
          end: true
    - name: check_logs
      states:
        - name: log_check
          type: operation
          action: agent.run
          input:
            agent: "log-analyzer"
            task: "Analyze logs for {{ ctx.service }}"
          resultPath: "branches.logs"
          end: true
  join:
    mode: all                  # wait for all branches (V1: all only)
  resultPath: "steps.diagnostics"
  next: analyze
```

#### D) `inject`

Writes static data into context.

```yaml
- name: set_thresholds
  type: inject
  data:
    thresholds:
      errorRate: 0.05
      latencyP99: 500
  resultPath: "ctx.config"
  next: diagnostics
```

#### E) `foreach`

Iterate over a list in context.

```yaml
- name: check_all_services
  type: foreach
  itemsPath: ctx.services       # dot-path to array
  itemName: service             # variable name for current item
  maxConcurrency: 1             # V1: sequential (1), future: bounded concurrency
  iterator:
    start: health_check
    states:
      - name: health_check
        type: operation
        action: agent.run
        input:
          agent: "health-checker"
          task: "Check health of {{ service.name }}"
        resultPath: "result"
        end: true
  resultPath: "steps.healthResults"
  next: summarize
```

### 3.3 Step Executors (V1)

#### `agent.run` → AgentExecutionEngine

Calls Station's existing AgentExecutionEngine with an agent identifier and task/input.

**Input schema**:
```yaml
action: agent.run
input:
  agent: "kubernetes-triage"        # agent name/id
  task: "Check pods in namespace production"
  variables:                        # optional: template variables for agent
    namespace: "production"
  timeoutSeconds: 300
```

**Output**: Agent's structured output (if output schema defined) or text result.

**Schema validation**: Before executing, workflow engine validates:
1. Agent exists in environment
2. Input matches agent's `inputSchema` (if defined)
3. After execution, output matches agent's `outputSchema` (if defined)
4. If next step has an agent with `inputSchema`, validate compatibility

**Idempotency**:
- Store `agent_run_id` in step record.
- If re-executed with same StepID+Attempt, return previously stored output.

#### `human.approval` → Approval Gates

Blocks workflow until human approves or rejects.

**Input schema**:
```yaml
action: human.approval
input:
  message: "Approve deployment to production?"
  summaryPath: "steps.plan"         # context path to show in approval UI
  approvers:                        # optional: restrict who can approve
    - "team:platform"
    - "user:alice@example.com"
  timeoutSeconds: 3600              # 1 hour timeout
```

**Behavior**:
- Step transitions to `WAITING_APPROVAL`
- Creates approval record in SQLite
- On approve: engine enqueues resume task
- On reject: run transitions to `CANCELLED` or `FAILED`
- On timeout: step transitions to `TIMED_OUT`

### 3.4 Schema Validation for Data Flow

Agents can define input and output schemas in their dotprompt frontmatter:

```yaml
---
model: openai/gpt-4o
metadata:
  name: kubernetes-triage
input:
  schema:
    type: object
    properties:
      namespace:
        type: string
      service:
        type: string
    required: [namespace]
output:
  schema:
    type: object
    properties:
      status:
        type: string
        enum: [healthy, degraded, critical]
      pods:
        type: array
        items:
          type: object
          properties:
            name: { type: string }
            ready: { type: boolean }
---
```

**Workflow engine validates**:
1. **Pre-execution**: Input data matches agent's input schema
2. **Post-execution**: Output data matches agent's output schema
3. **Flow validation**: At workflow load time, warn if agent A's output schema is incompatible with agent B's input schema

This enables **type-safe workflows** where data flows correctly between steps.

### 3.5 NATS JetStream Integration

**SQLite is the source of truth** for workflow state; JetStream is the **durable queue/event bus** for:
- Scheduling step execution
- Delivering signals
- Publishing run/step events
- Resuming processing after restart

**Streams (V1)**:

| Stream | Purpose | Subjects |
|--------|---------|----------|
| `WORKFLOW_TASKS` | Work queue | `wf.task.run.<runID>`, `wf.task.step.<runID>.<stepID>`, `wf.task.resume.<runID>` |
| `WORKFLOW_EVENTS` | Event log | `wf.event.run.<runID>`, `wf.event.step.<runID>.<stepID>` |

**Message headers**:
- `traceparent`: W3C trace context for OpenTelemetry propagation
- `idempotency-key`: `runID + stepID + attempt` for safe retries

**Crash recovery**:
1. If Station crashes mid-step, message is not acked → redelivered
2. Executor checks SQLite: "is this step already completed?"
3. If yes, ack and skip; if no, execute

### 3.6 State Machine

**Run lifecycle statuses (V1)**:
- `PENDING` → created, not yet started
- `RUNNING` → actively executing states
- `WAITING_APPROVAL` → blocked on approval
- `WAITING_SIGNAL` → blocked awaiting external signal
- `PAUSED` → operator-paused
- `COMPLETED` → end reached successfully
- `FAILED` → unrecoverable error
- `CANCELLED` → operator/system cancelled
- `TIMED_OUT` → run exceeded timeout

**Step lifecycle statuses (V1)**:
- `PENDING`, `RUNNING`, `WAITING_APPROVAL`, `SUCCEEDED`, `FAILED`, `CANCELLED`, `TIMED_OUT`

**Deterministic step identity**:
```
step_id = hash(run_id + state_name + branch_path + foreach_index)
```

---

## 4) API Design

### 4.1 Workflow Definition Endpoints

```
GET    /api/v1/workflows                    # list definitions
POST   /api/v1/workflows                    # create definition
GET    /api/v1/workflows/{id}               # get definition
PUT    /api/v1/workflows/{id}               # update (creates new version)
DELETE /api/v1/workflows/{id}               # delete (restricted if runs exist)
```

### 4.2 Run Management Endpoints

```
POST   /api/v1/workflow-runs                # start run
GET    /api/v1/workflow-runs/{runId}        # run status + context
GET    /api/v1/workflow-runs/{runId}/events # event timeline (paged)
POST   /api/v1/workflow-runs/{runId}/pause
POST   /api/v1/workflow-runs/{runId}/resume
POST   /api/v1/workflow-runs/{runId}/cancel
POST   /api/v1/workflow-runs/{runId}/signals # deliver signal
```

### 4.3 Approval Endpoints

```
GET    /api/v1/workflow-runs/{runId}/approvals              # list pending/decided
POST   /api/v1/workflow-runs/{runId}/approvals/{id}/approve
POST   /api/v1/workflow-runs/{runId}/approvals/{id}/reject
```

### 4.4 Real-time Updates (V1: SSE)

```
GET    /api/v1/workflow-runs/{runId}/stream  # Server-Sent Events
```

---

## 5) Observability

### 5.1 Tracing (OpenTelemetry)

**Span hierarchy**:
```
WorkflowRun (workflow_id, run_id)
├── StateTransition (state_name, state_type)
│   └── StepExecute (action, step_id, attempt)
│       └── AgentExecute (agent_name, agent_run_id)
```

Trace context propagated via NATS message headers (`traceparent`).

### 5.2 Structured Logging

Each step execution logs:
- `run_id`, `step_id`, `attempt`, `executor`, `duration_ms`, `status`
- `error_type` (timeout, retry_exhausted, validation, agent_error)

### 5.3 Run History and Audit

SQLite records an immutable event trail in `workflow_run_events`:
- Run created/started/completed/failed
- State entered/exited
- Step started/succeeded/failed
- Approval requested/approved/rejected (with actor identity)
- Signals received
- Pause/resume/cancel actions

---

## 6) Deployment Considerations

### 6.1 Embedded vs External NATS

| Mode | Use Case | Durability |
|------|----------|------------|
| Embedded NATS | Dev, single-node | Volume-dependent |
| External NATS | Production, HA | JetStream clustering |

**Configuration**:
```bash
STATION_NATS_EMBEDDED=true|false
STATION_NATS_URL=nats://nats:4222
STATION_NATS_JETSTREAM_ENABLED=true
```

### 6.2 Database

Uses the same SQLite database as Station (configured via `STATION_DATABASE_URL`). No separate database needed.

**Tables added** (from PR #83 migration `040_add_workflow_engine.sql`):
- `workflow_definitions`
- `workflow_runs`
- `workflow_run_steps`
- `workflow_run_events` (audit trail)
- `workflow_approvals`

### 6.3 Scaling (V1)

V1 targets single-replica deployment. Multiple replicas can process tasks via JetStream consumer groups, but correctness relies on:
- Durable step claiming
- Idempotent execution
- SQLite as source of truth

### 6.4 File-Based Workflow Definitions

Workflow definitions follow the same file-based configuration pattern as agents (`.prompt` files) and MCP servers. This enables:
- Version control for workflow definitions
- Bundle packaging and distribution
- GitOps workflows for workflow management

**Directory Structure**:
```
environments/
└── default/
    ├── agents/
    │   └── *.prompt
    ├── mcp-servers/
    │   └── *.json
    └── workflows/           # NEW: Workflow definitions
        ├── incident-triage.workflow.yaml
        ├── deploy-pipeline.workflow.yaml
        └── security-scan.workflow.yaml
```

**File Format**: `.workflow.yaml` or `.workflow.json`

```yaml
# incident-triage.workflow.yaml
id: incident-triage
version: "1.0"
name: "Incident Triage Workflow"
description: "Automated incident diagnosis with approval gate"
inputSchema:
  type: object
  properties:
    namespace: { type: string }
    service: { type: string }
  required: [namespace, service]
start: diagnostics
states:
  - name: diagnostics
    type: operation
    action: agent.run
    input:
      agent: "kubernetes-triage"
      task: "Check pods in {{ ctx.namespace }}"
    next: analyze
  # ... more states
```

**Sync Behavior**:
- `stn sync` discovers `.workflow.yaml` / `.workflow.json` files in `workflows/` directory
- Files are validated against Open Serverless Workflow schema (Station profile)
- Valid workflows are registered in SQLite with version tracking
- Deleted files are marked as `disabled` (not hard deleted, preserving run history)

**Bundle Integration**:
```
bundles/
└── sre-playbooks/
    ├── template.json
    ├── agents/
    │   └── kubernetes-triage.prompt
    ├── mcp-servers/
    │   └── kubectl.json
    └── workflows/              # Workflows included in bundle
        ├── incident-triage.workflow.yaml
        └── capacity-planning.workflow.yaml
```

---

## 7) Web UI

### 7.1 Workflows Page

A dedicated Workflows page in the Station web UI, similar to the existing Agents page.

**Navigation**: Add "Workflows" entry to sidenav under the existing sections.

```
┌─────────────────────────────────────────────────────────────────┐
│  Station                                                         │
├─────────────────────────────────────────────────────────────────┤
│  📊 Dashboard                                                    │
│  🤖 Agents                                                       │
│  🔧 Tools                                                        │
│  📋 Workflows  ← NEW                                             │
│  ⚙️  Settings                                                    │
└─────────────────────────────────────────────────────────────────┘
```

**Workflows List Page** (`/workflows`):

| Column | Description |
|--------|-------------|
| Name | Workflow name with link to detail |
| Version | Current active version |
| Status | `active` / `disabled` |
| Last Run | Timestamp of most recent run |
| Success Rate | % of successful runs (last 30 days) |
| Actions | Run, Edit, Disable |

**Features**:
- Filter by status, search by name
- Quick "Run" button to start a new run with input modal
- Sync status indicator (file vs DB state)

### 7.2 Workflow Detail Page

**Tabs**:

1. **Overview Tab**
   - Workflow metadata (name, description, version)
   - Visual state diagram (mermaid or custom SVG)
   - Input schema documentation
   - Quick stats (total runs, success rate, avg duration)

2. **Runs Tab**
   - Paginated list of workflow runs
   - Filter by status (running, completed, failed, cancelled)
   - Click to open run detail drawer/page
   - Real-time status updates via SSE

3. **Definition Tab**
   - YAML/JSON viewer with syntax highlighting
   - Read-only in V1 (edits via file system)
   - Validation status indicator

4. **Versions Tab**
   - Version history with timestamps
   - Diff viewer between versions
   - Rollback action (creates new version from old)

### 7.3 Run Detail Page/Drawer

**Sections**:

1. **Header**
   - Run ID, status badge, duration
   - Action buttons: Pause, Resume, Cancel
   - Approval actions (if waiting)

2. **Timeline View**
   - Vertical timeline of steps executed
   - Each step shows: name, status, duration, input/output toggle
   - Expandable step details with full payload
   - Approval steps show approver, decision, timestamp

3. **Context Inspector**
   - Current workflow context (JSON tree view)
   - Shows data flow between steps
   - Highlight changes per step

4. **Events Log**
   - Chronological event stream
   - Filter by event type
   - Export to JSON

### 7.4 React Components

```
static_src/js/Pages/Workflows/
├── Index.tsx              # Workflow list page
├── Detail.tsx             # Workflow detail with tabs
├── RunDetail.tsx          # Run detail page/drawer
└── components/
    ├── WorkflowCard.tsx           # List item card
    ├── WorkflowDiagram.tsx        # Visual state diagram
    ├── RunTimeline.tsx            # Step execution timeline
    ├── StepDetail.tsx             # Expandable step view
    ├── ContextInspector.tsx       # JSON context viewer
    ├── ApprovalCard.tsx           # Pending approval UI
    ├── RunInputModal.tsx          # Start run with input
    └── WorkflowStatusBadge.tsx    # Status indicator
```

### 7.5 API Integration

The UI consumes existing workflow APIs:

| Page | API Endpoints |
|------|---------------|
| List | `GET /api/v1/workflows` |
| Detail | `GET /api/v1/workflows/{id}` |
| Runs | `GET /api/v1/workflow-runs?workflowId={id}` |
| Run Detail | `GET /api/v1/workflow-runs/{runId}` |
| Start Run | `POST /api/v1/workflow-runs` |
| Approvals | `GET /api/v1/workflow-approvals` |
| Real-time | `GET /api/v1/workflow-runs/{runId}/stream` (SSE) |

---

## 8) Implementation Plan

### Phase 0 - Align & Harden PR #83 Foundations (1-4h) ✅ COMPLETE

- [x] Confirm schema migration stability (`040_add_workflow_engine.sql`)
- [x] Add `workflow_run_events` table for audit trail
- [x] Add `workflow_approvals` table
- [x] Confirm NATS engine uses explicit acks, durable consumers
  - **Gap identified**: Current engine uses `conn.Subscribe()` (basic NATS), not JetStream durable consumer
  - **Fix planned for Phase 1**: Convert to `js.Subscribe()` with `Durable`, `AckExplicit`, `DeliverAll`
- [x] Map PR #83 gaps to this PRD

**Deliverables**: 
- Updated migration `040_add_workflow_engine.sql` with `workflow_run_events` and `workflow_approvals` tables
- SQLC queries: `workflow_run_events.sql`, `workflow_approvals.sql`
- Schema updated in `internal/db/schema.sql`

**Files changed**:
- `internal/db/migrations/040_add_workflow_engine.sql`
- `internal/db/schema.sql`
- `internal/db/queries/workflow_run_events.sql` (new)
- `internal/db/queries/workflow_approvals.sql` (new)

### Phase 1 - Core Runtime: State Machine + Persistence (1-2d) ✅ COMPLETE

- [x] Implement deterministic `step_id` generation
  - Created `internal/workflows/stepid.go` with hash-based step ID
  - `GenerateStepID(runID, stateName, branchPath, foreachIndex)` 
  - `StepContext` builder with fluent API
  - `IdempotencyKey()` for deduplication
- [x] JetStream durable consumer with explicit acks
  - Updated `internal/workflows/runtime/nats_engine.go`
  - Uses `js.Subscribe()` with `Durable`, `AckExplicit`, `ManualAck`
  - Added `ConsumerName` to options
- [x] Implement idempotent step execution guard
  - Added `GetWorkflowRunStep` and `IsStepCompleted` SQLC queries
  - Added `Get()` and `IsCompleted()` repository methods
- [x] Ensure all transitions persist before enqueueing next task
- [x] Add event table writes for audit trail (`InsertWorkflowRunEvent`)
  - Created `WorkflowRunEventRepo` with Insert, GetNextSeq, ListByRun, ListByType
  - Created `WorkflowApprovalRepo` with Create, Get, ListByRun, ListPending, Approve, Reject, TimeoutExpired
  - Updated `WorkflowService.emitRunEvent()` to write to DB before NATS publish
  - Added event emission at all lifecycle points (StartRun, CancelRun, SignalRun, PauseRun, CompleteRun, RecordStepStart, RecordStepUpdate)

**Files implemented**:
- `internal/workflows/stepid.go` - Deterministic step ID generation
- `internal/workflows/stepid_test.go` - Tests (all passing)
- `internal/workflows/runtime/nats_engine.go` - JetStream durable consumer
- `internal/workflows/runtime/options.go` - Added `ConsumerName` field
- `internal/db/queries/workflow_run_steps.sql` - Idempotency queries
- `internal/db/queries/workflow_run_events.sql` - Event audit trail queries
- `internal/db/queries/workflow_approvals.sql` - Approval queries
- `internal/db/repositories/workflows.go` - Repository methods (WorkflowRunEventRepo, WorkflowApprovalRepo)
- `internal/db/repositories/base.go` - Added new repos to Repositories struct
- `internal/services/workflow_service.go` - Updated emitRunEvent for DB persistence
- `pkg/models/workflow.go` - Added WorkflowRunEvent, WorkflowApproval models and constants

**Deliverables**: Crash-recovery integration tests

### Phase 2 - Executor: agent.run (1-2d) ✅ COMPLETE

- [x] Build executor registry + interfaces
  - Created `StepExecutor` interface with `Execute()` and `SupportedTypes()` methods
  - Created `ExecutorRegistry` with `Register()`, `GetExecutor()`, and `Execute()` methods
- [x] Implement `agent.run` calling AgentExecutionEngine
  - Created `AgentRunExecutor` with dependency injection interface
  - Handles agent_id parsing (float64, int64, int, json.Number)
  - Merges workflow context variables with step input variables
- [ ] Add schema validation (input/output) - deferred to Phase 3
- [x] Store agent_run_id for idempotency (via step record)

**Files implemented**:
- `internal/workflows/runtime/executor.go` - StepExecutor interface, ExecutorRegistry, AgentRunExecutor
- `internal/workflows/runtime/executor_test.go` - Comprehensive tests (all passing)

**Deliverables**: E2E test: workflow calls agent, stores result

### Phase 3 - Executor: human.approval + Signals (1-2d)

- [ ] Implement `human.approval` executor
- [ ] Add approval records + APIs
- [ ] Implement `WAITING_SIGNAL` semantics
- [ ] Implement timeouts for approvals/signals

**Deliverables**: Workflow example: plan → approval → execute

### Phase 4 - Starlark Switch + Inject (1d)

- [ ] Integrate Starlark for `switch` conditions
- [ ] Implement `inject` state type
- [ ] Expand validator for V1 constraints

**Deliverables**: Workflow with conditional branching

### Phase 5 - Parallel + Foreach (2-3d)

- [ ] Implement parallel branch execution and join
- [ ] Implement foreach (sequential, maxConcurrency=1)
- [ ] Branch context isolation and merge

**Deliverables**: Parallel diagnostics workflow

### Phase 6 - File-Based Workflow Definitions (1-2d)

- [ ] Add `workflows/` directory support in environment structure
- [ ] Implement workflow file discovery (`.workflow.yaml`, `.workflow.json`)
- [ ] Add workflow sync logic to `stn sync` command
- [ ] Integrate workflows into bundle packaging
- [ ] Add file-based workflow validation on sync

**Deliverables**: Workflows loadable from files, included in bundles

### Phase 7 - Web UI: Workflows Page (2-3d)

- [ ] Add "Workflows" to sidenav navigation
- [ ] Implement Workflows list page (`/workflows`)
- [ ] Implement Workflow detail page with tabs (Overview, Runs, Definition, Versions)
- [ ] Add Run detail drawer/page with timeline view
- [ ] Implement "Start Run" modal with input form
- [ ] Add real-time run updates via SSE

**Deliverables**: Full workflows UI parity with Agents page

### Phase 8 - Observability + Docs (1-2d)

- [ ] OpenTelemetry spans + NATS trace propagation
- [ ] Add run/step metrics
- [ ] Deployment guides (Compose, ECS, GCP, Fly.io)
- [ ] Embedded vs external NATS guidance

---

## 10) Success Metrics

### Reliability

- **Crash recovery**: 99%+ of test runs resume correctly after forced restart
- **No lost progress**: No run transitions backwards or repeats succeeded steps

### Capability

- All V1 state types working: operation, switch, parallel, inject, foreach
- Both executors working: agent.run, human.approval
- Schema validation catching mismatched agent inputs/outputs
- File-based workflow definitions loadable via `stn sync`
- Workflows includable in bundles
- Full web UI for workflow management (list, detail, runs, approvals)

### Performance (V1 targets)

- Sustain 50+ concurrent runs on single Station instance
- JetStream consumer lag remains bounded

### Operability

- Run timeline reconstructable from SQLite events (100% fidelity)
- Traces show end-to-end run with step spans (95%+ runs)

---

## 11) Open Questions

1. **Starlark sandbox limits**: What CPU/memory limits for expression evaluation?
2. **Approval identity**: What auth system is authoritative (local users, CloudShip identity, OIDC)?
3. **Event retention**: How long to retain run events in SQLite? Compaction strategy?
4. **Agent cancellation**: What's the cancellation contract for AgentExecutionEngine?

---

## Appendix: Complete Workflow Example

```yaml
id: incident-triage
version: "1.0"
name: "Incident Triage Workflow"
description: "Automated incident diagnosis with approval gate"
inputSchema:
  type: object
  properties:
    namespace: { type: string }
    service: { type: string }
  required: [namespace, service]
start: inject_defaults

states:
  - name: inject_defaults
    type: inject
    data:
      thresholds:
        errorRate: 0.05
    resultPath: "ctx.config"
    next: diagnostics

  - name: diagnostics
    type: parallel
    branches:
      - name: pods
        states:
          - name: check_pods
            type: operation
            action: agent.run
            input:
              agent: "k8s-pod-checker"
              task: "Check pods in {{ ctx.namespace }}"
            resultPath: "result"
            end: true
      - name: errors
        states:
          - name: check_errors
            type: operation
            action: agent.run
            input:
              agent: "slo-analyzer"
              task: "Compute error rate for {{ ctx.service }}"
            resultPath: "result"
            end: true
    resultPath: "steps.diagnostics"
    next: decide

  - name: decide
    type: switch
    dataPath: steps.diagnostics.errors.result
    conditions:
      - if: "error_rate > ctx.config.thresholds.errorRate"
        next: request_approval
    defaultNext: report_ok

  - name: request_approval
    type: operation
    action: human.approval
    input:
      message: "Error rate high ({{ steps.diagnostics.errors.result.error_rate }}). Approve mitigation?"
      summaryPath: "steps.diagnostics"
    timeoutSeconds: 3600
    next: mitigate

  - name: mitigate
    type: operation
    action: agent.run
    input:
      agent: "k8s-remediator"
      task: "Restart unhealthy pods in {{ ctx.namespace }}"
    resultPath: "steps.mitigation"
    next: notify

  - name: notify
    type: operation
    action: agent.run
    input:
      agent: "notification-sender"
      task: "Send incident summary to Slack"
      variables:
        channel: "#incidents"
        summary: "{{ steps.mitigation.result }}"
    end: true

  - name: report_ok
    type: operation
    action: agent.run
    input:
      agent: "notification-sender"
      task: "Send all-clear to Slack"
      variables:
        channel: "#incidents"
        message: "No mitigation required for {{ ctx.service }}"
    end: true
```

---

*Created: 2025-12-23*  
*Based on: PR #83 workflow scaffolding*
