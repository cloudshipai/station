# PRD: CloudShip Station - Workflow Orchestration Engine (V1)

> **Status**: Draft  
> **Created**: 2025-12-23  
> **Updated**: 2025-12-24  
> **Based on**: PR #83 (`origin/codex/add-durable-workflow-engine-to-station`)
> **Current Phase**: Phase 10 - Cron State Executor (In Progress)

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
   - `cron` (scheduled start state)
   - `timer` (delayed execution)
   - `try_catch` (error handling with fallback)
3. **V1 Executors**:
   - `agent.run` → runs Station agent via AgentExecutionEngine
   - `human.approval` → approval gates with timeout
4. **Global agent resolution**: Agents referenced by name (globally unique), enabling bundle portability without ID mapping.
5. **Schema validation**: Validate that agent output schemas match next agent's input schemas for type-safe data flow.
6. **Human-in-loop controls**: Approval gates, pause/resume, signal delivery, and timeouts.
7. **Observability**: End-to-end tracing for workflow runs and per-step spans; durable run history for audit and debugging.
8. **Deployment compatibility**: Works in Docker Compose and common cloud platforms (ECS, GCP, Fly.io), with embedded or external NATS.

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

#### F) `cron`

Schedule-triggered start state. When used as the `start` state, the workflow runs automatically on schedule.

```yaml
id: daily-health-check
name: "Daily Health Check"
start: schedule                 # Start with cron state
states:
  - name: schedule
    type: cron
    cron: "0 9 * * *"           # Every day at 9 AM
    timezone: "America/Chicago"  # Optional, defaults to UTC
    enabled: true                # Can be disabled without removing
    input:                       # Injected into context on each run
      namespace: "production"
      services: ["api", "web", "worker"]
    next: check_services        # First "real" state after trigger
    
  - name: check_services
    type: operation
    action: agent.run
    input:
      agent: "health-checker"
      task: "Check services: {{ ctx.services }}"
    end: true
```

**Cron expression format** (standard 5-field):
```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6, Sun = 0)
│ │ │ │ │
* * * * *
```

**Common schedules**:
```yaml
cron: "*/15 * * * *"          # Every 15 minutes
cron: "0 * * * *"             # Every hour
cron: "0 9 * * *"             # Daily at 9 AM
cron: "0 9 * * 1-5"           # Weekdays at 9 AM
cron: "0 0 1 * *"             # Monthly on 1st at midnight
```

**Behavior**:
- Workflow with `cron` start state is registered with scheduler on sync
- Background scheduler checks every minute for due workflows
- On trigger: creates run with `input` injected into context
- Immediately transitions to `next` state
- Manual runs still work: `POST /api/v1/workflow-runs` skips cron state

**Management**:
- `enabled: false` stops scheduled runs without deleting workflow
- Deleting workflow removes from scheduler
- `stn sync` updates schedule from file changes

**Database**: Cron state metadata stored in `workflow_definitions.definition` JSON. 
Scheduler tracks in `workflow_schedules` table:
- `workflow_id`, `cron_expression`, `timezone`, `enabled`
- `last_run_at`, `next_run_at`

#### G) `timer`

Delay execution for a specified duration. Useful for rate limiting, cooldowns, or scheduled delays.

```yaml
- name: wait_for_cooldown
  type: timer
  duration: "5m"                # Duration: "30s", "5m", "1h", "24h"
  next: retry_check
```

**Duration formats**:
- Seconds: `"30s"`, `"120s"`
- Minutes: `"5m"`, `"15m"`
- Hours: `"1h"`, `"24h"`
- Combined: `"1h30m"`, `"2h15m30s"`

**Behavior**:
- Step transitions to `WAITING_TIMER`
- Timer is durable (survives Station restart via NATS delayed delivery)
- On completion: engine enqueues next step

#### H) `try_catch`

Error handling with fallback states. Wraps states that may fail with recovery logic.

```yaml
- name: safe_deploy
  type: try_catch
  try:
    start: deploy_service
    states:
      - name: deploy_service
        type: operation
        action: agent.run
        input:
          agent: "deployer"
          task: "Deploy {{ ctx.service }} to production"
        resultPath: "deploy_result"
        end: true
  catch:
    start: rollback
    states:
      - name: rollback
        type: operation
        action: agent.run
        input:
          agent: "deployer"
          task: "Rollback {{ ctx.service }} to previous version"
        resultPath: "rollback_result"
        end: true
  finally:                      # Optional: always runs
    start: notify
    states:
      - name: notify
        type: operation
        action: agent.run
        input:
          agent: "notifier"
          task: "Send deployment status to Slack"
        end: true
  resultPath: "steps.deployment"
  next: verify
```

**Behavior**:
- Executes `try` states normally
- On any error in `try`: captures error, executes `catch` states
- `finally` always executes (success or failure)
- Error details available in context: `ctx._error`, `ctx._errorStep`, `ctx._errorMessage`

**Error context**:
```json
{
  "_error": true,
  "_errorStep": "deploy_service",
  "_errorMessage": "Agent execution failed: timeout",
  "_errorType": "TIMEOUT"
}
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

### 3.4 Global Agent Resolution (Bundle-Portable)

Agents in workflows are referenced **by name globally**, enabling bundle portability across Station instances.

**Why global agent resolution**:
- **Bundle portability**: Workflows shared via bundles should work on any Station without ID mapping
- **Environment independence**: Agent IDs are auto-generated and differ between Station instances
- **GitOps compatibility**: Workflow definitions reference stable names, not instance-specific IDs
- **No ID mismatch**: When importing a bundle, environment IDs won't line up—global names always work

**Workflow definition syntax**:
```yaml
- name: check_pods
  type: operation
  input:
    task: agent.run
    agent: "kubernetes-triage"    # Agent name (global lookup)
    task: "Check pods in {{ ctx.namespace }}"
```

**Resolution rules**:
1. Agent lookup: `GetAgentByName(name)` - searches across ALL environments
2. If multiple agents with same name exist, use priority:
   - a) Agent in "default" environment (most common case)
   - b) First agent found (alphabetically by environment name)
   - c) Explicit environment override in step: `agent: "kubernetes-triage@production"`
3. If agent not found: validation error at definition time, graceful failure at runtime

**Environment override syntax** (optional):
```yaml
agent: "kubernetes-triage"              # Global lookup
agent: "kubernetes-triage@production"   # Explicit environment
```

**Bundle portability guarantee**:
- Workflows in bundles reference agents by name only
- When bundle is installed, agent names are resolved at runtime
- No environment ID mapping needed during bundle import/export

### 3.5 Schema Validation for Data Flow

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

**Workflow engine validates at three levels**:

#### Level 1: Static Validation (at workflow create/update)

When a workflow is created or updated, the validator checks:

1. **Agent existence**: All referenced agents exist in the environment
2. **Schema compatibility**: For sequential steps A → B where both are `agent.run`:
   - If A has `output_schema` and B has `input_schema`, validate compatibility
   - Compatibility rule: A's output must satisfy B's input requirements (superset OK)
   - Missing schemas are allowed (no validation performed)

```
Step A (agent: "pod-checker")     Step B (agent: "log-analyzer")
output_schema: {                  input_schema: {
  pods: [{name, status}],           pods: [{name}],     ← OK: A provides more
  timestamp: string                 filters: [string]   ← WARNING: B expects field A doesn't provide
}                                 }
```

**Validation result**:
- `errors`: Block workflow creation (agent not found, critical schema mismatch)
- `warnings`: Allow creation but surface issues (optional field missing, type coercion needed)

#### Level 2: Pre-execution Validation (at step start)

Before executing an agent step:
1. Extract input data from workflow context (via `resultPath` of previous step)
2. Validate against agent's `input_schema` if defined
3. On validation failure: fail step with clear error, do NOT execute agent

#### Level 3: Post-execution Validation (at step completion)

After agent execution:
1. Validate output against agent's `output_schema` if defined
2. On validation failure: log warning but proceed (don't fail the workflow)
3. Store validation result in step record for debugging

#### Graceful NATS Error Handling

Schema mismatches at runtime are handled gracefully through the NATS message flow:

```
┌─────────────────────────────────────────────────────────────────┐
│  NATS Step Execution Flow with Schema Validation                │
├─────────────────────────────────────────────────────────────────┤
│  1. Consumer receives step task                                  │
│  2. Load step definition + agent metadata                        │
│  3. Extract input from workflow context                          │
│  4. IF agent.input_schema exists:                                │
│     ├─ Validate input against schema                             │
│     ├─ ON FAILURE: Record step as FAILED with validation error   │
│     │              Ack message (don't retry)                     │
│     │              Emit error event to WORKFLOW_EVENTS           │
│     │              Continue to next step if error handling set   │
│     └─ ON SUCCESS: Proceed to execution                          │
│  5. Execute agent                                                │
│  6. IF agent.output_schema exists:                               │
│     ├─ Validate output against schema                            │
│     ├─ ON FAILURE: Log warning, store validation result          │
│     │              Proceed anyway (output may still be useful)   │
│     └─ ON SUCCESS: Store validated output                        │
│  7. Write output to workflow context at resultPath               │
│  8. Schedule next step                                           │
└─────────────────────────────────────────────────────────────────┘
```

**Error categories**:
- `SCHEMA_VALIDATION_INPUT`: Input doesn't match agent's input_schema
- `SCHEMA_VALIDATION_OUTPUT`: Output doesn't match agent's output_schema (warning only)
- `SCHEMA_COMPATIBILITY`: Static validation found incompatible schemas between steps

This enables **type-safe workflows** where data flows correctly between steps, while gracefully handling edge cases without crashing the entire workflow.

### 3.6 NATS JetStream Integration

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

### 3.7 State Machine

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

### Phase 3 - Executor: human.approval + Signals (1-2d) ✅ COMPLETE

- [x] Implement `human.approval` executor
- [x] Add approval records + APIs
- [x] Implement `WAITING_SIGNAL` semantics
- [x] Implement timeouts for approvals/signals

**Deliverables**: Workflow example: plan → approval → execute

### Phase 4 - Starlark Switch + Inject (1d) ✅ COMPLETE

- [x] Integrate Starlark for `switch` conditions
- [x] Implement `inject` state type
- [x] Add GetNestedValue/SetNestedValue for context access
- [x] Add comprehensive tests

**Deliverables**: Workflow with conditional branching

### Phase 5 - Parallel + Foreach (2-3d) ✅ COMPLETE

- [x] Implement parallel branch execution and join
- [x] Implement foreach (sequential and concurrent with maxConcurrency)
- [x] Branch context isolation and merge
- [x] Add comprehensive tests

**Deliverables**: Parallel diagnostics workflow

### Phase 6 - File-Based Workflow Definitions (1-2d) ✅ COMPLETE

- [x] Add `workflows/` directory support in environment structure (`config/paths.go`)
- [x] Implement workflow file discovery (`.workflow.yaml`, `.workflow.yml`, `.workflow.json`)
- [x] Create workflow loader (`internal/workflows/loader.go`)
- [x] Add `SyncWorkflowFiles` to workflow service
- [x] Integrate workflow sync into declarative sync (`stn sync`)
- [x] Add file-based workflow validation on sync
- [ ] Integrate workflows into bundle packaging (deferred to Phase 6.5)

**Deliverables**: Workflows loadable from files via `stn sync`

**Directory Structure**:
```
environments/
└── default/
    ├── agents/
    │   └── *.prompt
    ├── mcp-servers/
    │   └── *.json
    └── workflows/
        ├── incident-triage.workflow.yaml
        └── deploy-pipeline.workflow.json
```

### Phase 6.5 - Workflow Consumer & Execution Wiring (1d) ✅ COMPLETE

- [x] Create WorkflowServiceAdapter implementing WorkflowRunUpdater, StepRecorder, StepProvider
- [x] Wire WorkflowConsumer startup in API handlers when NATS engine enabled
- [x] Update scheduleNextStep to fetch full step definition from execution plan
- [x] Enable NATS and embedded NATS by default (no env vars needed)
- [x] Register inject, switch, agent, and human approval executors in consumer startup
- [x] Create approvalExecutorAdapter bridging ApprovalExecutorDeps interface to repositories
- [x] Implement approval resume mechanism (schedule next step after approval granted)
- [x] Implement approval rejection mechanism (fail run when rejected)
- [x] Modify HumanApprovalExecutor to get runID from runContext (single executor instance)

**Files implemented**:
- `internal/workflows/runtime/adapter.go` - WorkflowServiceAdapter
- `internal/workflows/runtime/consumer.go` - WorkflowConsumer with StepProvider interface, adds `_runID` to runContext
- `internal/workflows/runtime/executor.go` - HumanApprovalExecutor reads runID from `runContext["_runID"]`
- `internal/api/v1/base.go` - Consumer startup wiring with approvalExecutorAdapter
- `internal/services/workflow_service.go` - resumeAfterApproval() and failAfterRejection() methods

**Deliverables**: Workflows execute automatically when runs are started, including human approval steps

### Phase 7 - Web UI: Workflows Page (2-3d) ✅ COMPLETE

- [x] Add "Workflows" to sidenav navigation
- [x] Implement Workflows list page (`/workflows`)
- [x] Implement Workflow detail page with tabs (Overview, Runs, Definition, Versions)
- [x] Add Run detail drawer/page with timeline view
- [x] Implement "Start Run" modal with Monaco JSON editor
- [x] Add real-time run updates via SSE
- [x] Add React Flow visualization with ELK layout

**Deliverables**: Full workflows UI parity with Agents page

### Phase 8 - Agent Name Resolution + Schema Validation (2-3d)

Implement type-safe workflows with agent name resolution and schema validation.

#### 8.1 Agent Name Resolution

- [ ] Update `AgentRunExecutor` to resolve agents by name instead of ID
- [ ] Add `environment_id` to workflow execution context
- [ ] Create `AgentResolver` interface for testability
- [ ] Update workflow definition schema to use `agent: "name"` syntax

**Files to modify/create**:
- `internal/workflows/runtime/executor.go` - Update AgentRunExecutor
- `internal/workflows/runtime/agent_resolver.go` - New: AgentResolver interface
- `internal/workflows/types.go` - Add environment_id to execution context

#### 8.2 Workflow Validator

- [ ] Create `WorkflowValidator` for static validation at create/update time
- [ ] Validate all referenced agents exist in environment
- [ ] Build step transition graph for schema validation
- [ ] Return structured `ValidationResult` with errors and warnings

**Files to create**:
- `internal/workflows/validator.go` - WorkflowValidator implementation
- `internal/workflows/validator_test.go` - Comprehensive tests

#### 8.3 Schema Compatibility Checker

- [ ] Create `SchemaChecker` for JSON Schema compatibility validation
- [ ] Implement superset validation (output satisfies input requirements)
- [ ] Handle optional vs required fields
- [ ] Generate human-readable compatibility reports

**Files to create**:
- `internal/workflows/schema_checker.go` - SchemaChecker implementation
- `internal/workflows/schema_checker_test.go` - Tests

#### 8.4 Runtime Schema Validation

- [ ] Add pre-execution input validation in step consumer
- [ ] Add post-execution output validation (warning only)
- [ ] Emit schema validation events to WORKFLOW_EVENTS
- [ ] Graceful error handling without crashing workflow

**Files to modify**:
- `internal/workflows/runtime/consumer.go` - Add validation hooks
- `internal/workflows/runtime/executor.go` - Add validation to AgentRunExecutor

#### 8.5 API Integration

- [ ] Add validation endpoint: `POST /api/v1/workflows/validate`
- [ ] Return validation results on workflow create/update
- [ ] Surface validation warnings in UI

**Deliverables**: Type-safe workflows with clear validation errors

---

## 9) Test Plan: Agent Name Resolution + Schema Validation

### 9.1 Unit Tests

#### Agent Resolution Tests (`internal/workflows/runtime/executor_test.go`)

| Test Case | Input | Expected |
|-----------|-------|----------|
| `TestAgentRunExecutor_ResolveByName_Success` | `agent: "kubernetes-triage"`, env has agent | Agent resolved, execution proceeds |
| `TestAgentRunExecutor_ResolveByName_NotFound` | `agent: "nonexistent"` | Error: `agent not found in environment` |
| `TestAgentRunExecutor_ResolveByName_WrongEnvironment` | Agent exists in different env | Error: `agent not found in environment` |
| `TestAgentRunExecutor_ResolveByName_EmptyName` | `agent: ""` | Error: `agent name is required` |
| `TestAgentRunExecutor_BackwardsCompatibility` | `agent_id: 123` (legacy) | Works with deprecation warning |

#### Workflow Validator Tests (`internal/workflows/validator_test.go`)

| Test Case | Input | Expected |
|-----------|-------|----------|
| `TestValidator_AllAgentsExist` | Workflow with 3 valid agents | No errors |
| `TestValidator_AgentNotFound` | Workflow references missing agent | Error with agent name and step |
| `TestValidator_MultipleAgentsNotFound` | 2 missing agents | 2 errors, one per agent |
| `TestValidator_EmptyWorkflow` | No states | Error: `workflow has no states` |
| `TestValidator_InvalidStartState` | `start` references nonexistent state | Error: `start state not found` |
| `TestValidator_CircularTransition` | A → B → A | Warning: `circular transition detected` |
| `TestValidator_UnreachableState` | State with no incoming transition | Warning: `unreachable state` |

#### Schema Compatibility Tests (`internal/workflows/schema_checker_test.go`)

| Test Case | Output Schema | Input Schema | Expected |
|-----------|--------------|--------------|----------|
| `TestSchema_ExactMatch` | `{pods: []}` | `{pods: []}` | Compatible |
| `TestSchema_OutputSuperset` | `{pods: [], timestamp: ""}` | `{pods: []}` | Compatible |
| `TestSchema_MissingRequiredField` | `{pods: []}` | `{pods: [], filters: []}` (required) | Incompatible |
| `TestSchema_MissingOptionalField` | `{pods: []}` | `{pods: [], filters?: []}` | Compatible with warning |
| `TestSchema_TypeMismatch` | `{count: "10"}` | `{count: integer}` | Incompatible |
| `TestSchema_NestedObjectMatch` | `{result: {status: ""}}` | `{result: {status: ""}}` | Compatible |
| `TestSchema_ArrayItemMatch` | `{pods: [{name: ""}]}` | `{pods: [{name: ""}]}` | Compatible |
| `TestSchema_NoSchemas` | None | None | Compatible (no validation) |
| `TestSchema_OnlyOutputSchema` | `{pods: []}` | None | Compatible (no validation) |
| `TestSchema_OnlyInputSchema` | None | `{pods: []}` | Warning: cannot validate |

#### Runtime Validation Tests (`internal/workflows/runtime/consumer_test.go`)

| Test Case | Scenario | Expected |
|-----------|----------|----------|
| `TestRuntime_InputValidation_Pass` | Input matches schema | Step executes |
| `TestRuntime_InputValidation_Fail` | Input missing required field | Step FAILED, clear error message |
| `TestRuntime_InputValidation_NoSchema` | Agent has no input_schema | Step executes (no validation) |
| `TestRuntime_OutputValidation_Pass` | Output matches schema | Step completes normally |
| `TestRuntime_OutputValidation_Fail` | Output missing field | Warning logged, step still completes |
| `TestRuntime_OutputValidation_NoSchema` | Agent has no output_schema | Step completes (no validation) |

### 9.2 Integration Tests

#### End-to-End Workflow Tests (`internal/workflows/runtime/e2e_test.go`)

| Test Case | Scenario | Expected |
|-----------|----------|----------|
| `TestE2E_ThreeAgentPipeline_SchemaValid` | A → B → C, all schemas compatible | Workflow completes successfully |
| `TestE2E_ThreeAgentPipeline_SchemaMismatch` | A outputs X, B expects Y | Static validation error at creation |
| `TestE2E_RuntimeSchemaMismatch` | Agent returns unexpected output | Warning logged, workflow continues |
| `TestE2E_AgentNotFoundAtRuntime` | Agent deleted after workflow created | Step fails with clear error |
| `TestE2E_ParallelBranches_SchemaValidation` | Parallel branches with different schemas | Each branch validated independently |
| `TestE2E_ForeachIteration_SchemaValidation` | Foreach with schema-validated agent | Each iteration validated |

#### API Tests (`internal/api/v1/workflows_test.go`)

| Test Case | Endpoint | Expected |
|-----------|----------|----------|
| `TestAPI_CreateWorkflow_Valid` | `POST /workflows` with valid def | 201, workflow created |
| `TestAPI_CreateWorkflow_AgentNotFound` | `POST /workflows` with bad agent | 400, validation errors returned |
| `TestAPI_CreateWorkflow_SchemaIncompatible` | `POST /workflows` with mismatched schemas | 400 or 201 with warnings |
| `TestAPI_ValidateWorkflow` | `POST /workflows/validate` | 200, validation result |
| `TestAPI_UpdateWorkflow_BreaksSchema` | `PUT /workflows/{id}` | Validation errors if incompatible |

### 9.3 Test Fixtures

#### Sample Agents (for tests)

```yaml
# Agent: pod-checker (has output_schema)
name: pod-checker
input_schema:
  type: object
  properties:
    namespace: { type: string }
  required: [namespace]
output_schema:
  type: object
  properties:
    pods:
      type: array
      items:
        type: object
        properties:
          name: { type: string }
          status: { type: string }
    timestamp: { type: string }
  required: [pods]

# Agent: log-analyzer (has input_schema matching pod-checker output)
name: log-analyzer
input_schema:
  type: object
  properties:
    pods:
      type: array
      items:
        type: object
        properties:
          name: { type: string }
  required: [pods]
output_schema:
  type: object
  properties:
    analysis: { type: string }
    severity: { type: string, enum: [low, medium, high, critical] }

# Agent: notifier (no schemas - accepts anything)
name: notifier
# No input_schema or output_schema defined
```

#### Sample Workflows (for tests)

```yaml
# Valid workflow - schemas compatible
id: valid-pipeline
states:
  - id: check
    type: operation
    input:
      task: agent.run
      agent: pod-checker
    resultPath: steps.check
    transition: analyze
  - id: analyze
    type: operation
    input:
      task: agent.run
      agent: log-analyzer
      # Input comes from steps.check.pods - matches input_schema
    transition: notify
  - id: notify
    type: operation
    input:
      task: agent.run
      agent: notifier
    end: true

# Invalid workflow - agent not found
id: invalid-agent
states:
  - id: check
    type: operation
    input:
      task: agent.run
      agent: nonexistent-agent  # Does not exist
    end: true

# Warning workflow - schema mismatch (optional field missing)
id: warning-pipeline
states:
  - id: step1
    type: operation
    input:
      task: agent.run
      agent: minimal-output-agent  # outputs: {result: ""}
    transition: step2
  - id: step2
    type: operation
    input:
      task: agent.run
      agent: expects-more-agent   # expects: {result: "", metadata?: {}}
    end: true
```

### 9.4 Test Execution Commands

```bash
# Run all workflow tests
go test ./internal/workflows/... -v

# Run specific test suites
go test ./internal/workflows/runtime/... -v -run TestAgentRunExecutor
go test ./internal/workflows/... -v -run TestValidator
go test ./internal/workflows/... -v -run TestSchema

# Run integration tests
go test ./internal/workflows/runtime/... -v -run TestE2E

# Run with coverage
go test ./internal/workflows/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### 9.5 E2E Test Plan (Manual Verification)

This section documents end-to-end testing using real workflows with actual agents. Test workflows are located in `examples/workflows/`.

#### Prerequisites

1. Station server running with agents registered
2. At least these agents available: `k8s-investigator`, `aws-log-analyzer`, `grafana-analyst`, `root-cause-analyzer`, `k8s-deployment-checker`, `alert-investigator`
3. NATS enabled (embedded or external)

#### Test Matrix

| # | Test | Workflow File | State Types | Expected Outcome |
|---|------|---------------|-------------|------------------|
| 1 | Basic Inject + Switch | `e2e-switch-routing.workflow.yaml` | inject, switch, operation | Injects alert data, routes to `high_severity_response` branch, runs 2 agents |
| 2 | Parallel Branches | `e2e-parallel-diagnostics.workflow.yaml` | inject, parallel, operation | Runs 3 agents concurrently, merges results, runs analyzer |
| 3 | Foreach Concurrent | `e2e-foreach-services.workflow.yaml` | inject, foreach, operation | Iterates 3 services with maxConcurrency=2, runs summarizer |
| 4 | Cron Triggered | `e2e-cron-with-agents.workflow.yaml` | cron, inject, operation | Triggers every 3 minutes, runs 3 sequential agents |
| 5 | Human Approval | (create ad-hoc) | operation, human.approval | Blocks on approval, resumes after approve/reject |
| 6 | Complex Multi-Step | (combine above) | All state types | Full workflow with all features |

#### Test 1: Inject + Switch Routing

**Workflow**: `examples/workflows/e2e-switch-routing.workflow.yaml`

**What it tests**:
- `inject` state: Sets alert data with severity="high"
- `switch` state: Routes based on `alert.severity` value
- `operation` state: Runs `alert-investigator` and `root-cause-analyzer` agents

**Steps**:
```bash
# 1. Sync workflow to Station
stn sync

# 2. Start workflow run via API or UI
curl -X POST http://localhost:8585/api/v1/workflow-runs \
  -H "Content-Type: application/json" \
  -d '{"workflow_id": "e2e-switch-routing"}'

# 3. Watch run progress
curl http://localhost:8585/api/v1/workflow-runs/{runId}
```

**Expected outcome**:
- Run starts in `inject_alert` state
- Injects `{severity: "high", ...}` into context
- Switch routes to `high_severity_response` (not critical, medium, or low)
- Runs `alert-investigator` agent
- Transitions to `analyze_high`
- Runs `root-cause-analyzer` agent
- Run completes with status `COMPLETED`

**Verification**:
```bash
# Check run status
curl http://localhost:8585/api/v1/workflow-runs/{runId} | jq '.status'
# Expected: "COMPLETED"

# Check steps executed
curl http://localhost:8585/api/v1/workflow-runs/{runId}/steps | jq '.[].name'
# Expected: ["inject_alert", "route_by_severity", "high_severity_response", "analyze_high"]
```

#### Test 2: Parallel Branches

**Workflow**: `examples/workflows/e2e-parallel-diagnostics.workflow.yaml`

**What it tests**:
- `parallel` state: Runs 3 branches concurrently
- Branch isolation: Each branch has independent context
- Join: Waits for all branches before proceeding
- Result merging: Combined results available in `steps.gather_diagnostics`

**Steps**:
```bash
# Start run
curl -X POST http://localhost:8585/api/v1/workflow-runs \
  -H "Content-Type: application/json" \
  -d '{"workflow_id": "e2e-parallel-diagnostics"}'
```

**Expected outcome**:
- `init` injects context with namespace, service, time_range
- `gather_diagnostics` starts 3 branches in parallel:
  - Branch `kubernetes`: runs `k8s-investigator`
  - Branch `aws_logs`: runs `aws-log-analyzer`
  - Branch `grafana_metrics`: runs `grafana-analyst`
- All 3 branches complete (order may vary)
- Join waits for all branches
- `analyze_results` runs `root-cause-analyzer` with combined data
- Run completes

**Verification**:
- Check logs for concurrent agent execution (overlapping timestamps)
- All 4 agents should have executed
- Final context should contain results from all branches

#### Test 3: Foreach Concurrent Execution

**Workflow**: `examples/workflows/e2e-foreach-services.workflow.yaml`

**What it tests**:
- `foreach` state: Iterates over 3 services
- `maxConcurrency: 2`: Runs 2 iterations in parallel at a time
- Item variable: `{{ service.name }}`, `{{ service.namespace }}`
- Thread-safe prompt loading: Validates our race condition fix

**Steps**:
```bash
curl -X POST http://localhost:8585/api/v1/workflow-runs \
  -H "Content-Type: application/json" \
  -d '{"workflow_id": "e2e-foreach-services"}'
```

**Expected outcome**:
- `init_services` injects 3 service objects
- `check_each_service` iterates with `maxConcurrency: 2`:
  - First 2 services run in parallel
  - Third service runs after one completes
- Each iteration runs `k8s-deployment-checker` agent
- `summarize` runs `root-cause-analyzer` agent
- **NO PANIC**: Concurrent prompt loading should work (validates our fix)

**Verification**:
```bash
# Check for panic in logs (should be none)
grep -i "panic" /tmp/station.log

# Check for our fix working
grep "Reusing already-registered prompt" /tmp/station.log
# Should see entries for k8s-deployment-checker being reused
```

#### Test 4: Cron-Triggered Execution

**Workflow**: `examples/workflows/e2e-cron-with-agents.workflow.yaml`

**What it tests**:
- `cron` state: Scheduled execution every 3 minutes
- Cron context injection: `_cronTriggeredAt`, `_cronExpression`
- Sequential agent execution after trigger

**Steps**:
```bash
# 1. Sync workflow (registers cron schedule)
stn sync

# 2. Check schedule registered
curl http://localhost:8585/api/v1/workflow-schedules | jq '.[] | select(.workflow_id == "e2e-cron-with-agents")'

# 3. Wait for next trigger (up to 3 minutes) or check logs
tail -f /tmp/station.log | grep "WorkflowScheduler"
```

**Expected outcome**:
- Workflow registered in `workflow_schedules` table
- Background scheduler triggers every 3 minutes
- Each trigger creates a new run with:
  - `_cronTriggeredAt` in context
  - `_cronExpression` and `_cronTimezone` metadata
- Run executes: `inject_config` → `gather_metrics` → `check_kubernetes` → `analyze_health`
- Run completes automatically

**Verification**:
```bash
# Check for triggered runs
curl http://localhost:8585/api/v1/workflow-runs?workflow_id=e2e-cron-with-agents | jq '.[0]'

# Check context has cron metadata
curl http://localhost:8585/api/v1/workflow-runs/{runId} | jq '.context._cronTriggeredAt'
```

#### Test 5: Human Approval Gate

**What it tests**:
- `human.approval` action: Blocks workflow for approval
- Timeout handling: Approval expires after timeout
- Resume: Workflow continues after approval granted

**Ad-hoc workflow** (create via API):
```bash
curl -X POST http://localhost:8585/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_id": "e2e-approval-test",
    "name": "E2E Approval Test",
    "definition": {
      "id": "e2e-approval-test",
      "version": "1.0",
      "start": "prepare",
      "states": [
        {
          "name": "prepare",
          "type": "inject",
          "data": {"action": "deploy", "env": "production"},
          "next": "request_approval"
        },
        {
          "name": "request_approval",
          "type": "operation",
          "action": "human.approval",
          "input": {
            "message": "Approve deployment to production?",
            "timeoutSeconds": 300
          },
          "next": "execute"
        },
        {
          "name": "execute",
          "type": "inject",
          "data": {"status": "deployed"},
          "end": true
        }
      ]
    }
  }'
```

**Expected outcome**:
1. Run starts, executes `prepare`
2. `request_approval` creates approval record, run status → `WAITING_APPROVAL`
3. On approve: run resumes, executes `execute`, completes
4. On reject: run fails with `CANCELLED` or `FAILED`
5. On timeout: run fails with `TIMED_OUT`

#### Test 6: Load Test (10+ Concurrent Runs)

**What it tests**:
- Concurrent workflow execution
- NATS consumer handling
- SQLite contention
- Memory/goroutine usage

**Steps**:
```bash
# Start 15 concurrent runs
for i in {1..15}; do
  curl -X POST http://localhost:8585/api/v1/workflow-runs \
    -H "Content-Type: application/json" \
    -d '{"workflow_id": "e2e-parallel-diagnostics"}' &
done
wait

# Monitor
watch -n 1 'curl -s http://localhost:8585/api/v1/workflow-runs?status=RUNNING | jq length'
```

**Expected outcome**:
- All 15 runs start without error
- Runs execute concurrently
- All runs complete eventually
- No panics, no deadlocks
- Memory usage stays bounded

#### UI Verification (Playwright)

| Test | Steps | Expected |
|------|-------|----------|
| Workflow List | Navigate to `/workflows` | See all synced workflows |
| Workflow Detail | Click workflow | See tabs: Overview, Runs, Definition |
| Start Run | Click "Run" button, enter input | Run created, redirected to run detail |
| Run Timeline | View run detail | Steps displayed in order with status |
| Real-time Updates | Start run, watch | Status updates via SSE without refresh |
| Approval UI | View waiting run | Shows approval button, can approve/reject |

---

### Phase 9 - Global Agent Resolution (1d)

Implement bundle-portable agent resolution by name.

- [ ] Update `AgentRunExecutor` to use global agent lookup
- [ ] Implement `GetAgentByName(name)` repository method (searches all environments)
- [ ] Add environment override syntax: `agent: "name@environment"`
- [ ] Update workflow validator to use global lookup
- [ ] Add tests for multi-environment agent resolution

**Files to modify/create**:
- `internal/db/repositories/agents.go` - Add `GetByName()` method
- `internal/workflows/runtime/executor.go` - Update `resolveAgent()`
- `internal/workflows/validator.go` - Update agent existence check

### Phase 10 - Cron State Executor (1-2d) - IN PROGRESS

Implement `cron` state type for scheduled workflow execution.

- [x] Create `workflow_schedules` table (migration `041_add_workflow_schedules.sql`)
- [x] Create `WorkflowScheduleRepo` with CRUD operations
- [x] Implement schedule parsing with `robfig/cron/v3`
- [x] Create `WorkflowSchedulerService` with background ticker (checks every minute)
- [x] Wire up scheduler in `server.go` (start/stop lifecycle)
- [x] Inject scheduler into `DeclarativeSync` for registration on sync
- [x] Add `RegisterCronSchedules` to `WorkflowService`
- [x] Add `RegisterWorkflowSchedule` to scheduler (finds cron start state, registers)
- [x] On trigger: create run via `WorkflowService.StartRun`, inject cron state's `input`
- [x] Add `enabled` toggle support via `SetScheduleEnabled`
- [x] Add cron state fields to `StateSpec` (Cron, Timezone, Enabled)
- [ ] Create `CronExecutor` implementing `StepExecutor` interface (for runtime skipping)
- [ ] Add UI indicator for scheduled workflows
- [ ] Add tests

**Files created/modified**:
- `internal/db/migrations/041_add_workflow_schedules.sql` - Migration for workflow_schedules table
- `internal/db/queries/workflow_schedules.sql` - SQLC queries for schedules
- `internal/db/repositories/workflows.go` - WorkflowScheduleRepo implementation
- `internal/services/workflow_scheduler_service.go` - Background scheduler service
- `internal/services/workflow_service.go` - Added RegisterCronSchedules method
- `internal/services/declarative_sync.go` - Added SetWorkflowScheduler, updated syncWorkflows
- `internal/workflows/types.go` - Added Cron, Timezone, Enabled fields to StateSpec
- `cmd/main/server.go` - Wire up WorkflowSchedulerService lifecycle

**Architecture**:
```
┌─────────────────────────────────────────────────────────────────┐
│                      Station Startup                              │
├─────────────────────────────────────────────────────────────────┤
│  1. server.go creates WorkflowService                            │
│  2. server.go creates WorkflowSchedulerService                   │
│  3. server.go calls workflowSchedulerSvc.Start(ctx)              │
│  4. DeclarativeSync.syncWorkflows() registers cron schedules     │
│     └─ WorkflowService.RegisterCronSchedules(scheduler)          │
│         └─ scheduler.RegisterWorkflowSchedule(def, version)      │
│  5. Background ticker checks every minute for due schedules       │
│     └─ scheduler.checkAndTrigger(ctx)                            │
│         └─ WorkflowService.StartRun() for each due schedule      │
│  6. On shutdown: workflowSchedulerSvc.Stop()                     │
└─────────────────────────────────────────────────────────────────┘
```

**Database Schema** (`workflow_schedules`):
```sql
CREATE TABLE workflow_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workflow_id TEXT NOT NULL,
    workflow_version INTEGER NOT NULL,
    cron_expression TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    enabled INTEGER NOT NULL DEFAULT 1,
    input TEXT,
    last_run_at DATETIME,
    next_run_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workflow_id, workflow_version)
);
```

**E2E Testing Method**:

To verify cron workflow functionality end-to-end:

```bash
# 1. Build station with UI
cd /path/to/station
make local-install-ui

# 2. Start station server in dev mode
export STN_DEV_MODE=true
stn serve

# 3. In another terminal, start the UI dev server
cd /path/to/station/ui
npm run dev
```

**API Verification** (localhost:8585):

```bash
# Create a cron workflow via API
curl -X POST http://localhost:8585/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflow_id": "cron-test",
    "name": "Cron Test Workflow",
    "definition": {
      "id": "cron-test",
      "version": "1.0",
      "name": "Cron Test",
      "start": "schedule",
      "states": [
        {
          "name": "schedule",
          "type": "cron",
          "cron": "*/1 * * * *",
          "timezone": "UTC",
          "enabled": true,
          "input": {"test": "value"},
          "next": "do_work"
        },
        {
          "name": "do_work",
          "type": "inject",
          "data": {"message": "Cron triggered"},
          "end": true
        }
      ]
    }
  }'

# Check workflow schedules table
curl http://localhost:8585/api/v1/workflow-schedules

# List workflow runs (check for cron-triggered runs)
curl http://localhost:8585/api/v1/workflow-runs?workflow_id=cron-test
```

**Playwright Browser Verification**:

1. Navigate to `http://localhost:5173/workflows` (UI dev server)
2. Verify cron workflow appears in list with schedule indicator
3. Check workflow detail page shows next scheduled run time
4. Wait for cron trigger (1 minute) and verify new run appears
5. Check run detail shows `_cronTriggeredAt` in context

**Expected Behavior**:
- Workflow with cron start state registers in `workflow_schedules` table on sync
- Background scheduler checks every minute for due schedules
- When `next_run_at <= now`, scheduler triggers `WorkflowService.StartRun()`
- Run context contains `_cronTriggeredAt`, `_cronExpression`, `_cronTimezone`
- `last_run_at` and `next_run_at` updated after each trigger

### Phase 11 - Tool Step Executor ~~(1d)~~ REMOVED

~~Implement direct MCP tool invocation step type.~~

**Status**: REMOVED - Tool step type was removed because ensuring tool signature compatibility would be a maintenance burden. Agents can call any MCP tools they need internally, making direct tool steps redundant.

### Phase 12 - Timer Step Executor (0.5d)

Implement delayed execution step type.

- [ ] Create `TimerExecutor` implementing `StepExecutor` interface
- [ ] Parse duration strings ("5m", "1h30m")
- [ ] Use NATS delayed delivery or background timer
- [ ] Add `WAITING_TIMER` step status
- [ ] Add tests

**Files to create/modify**:
- `internal/workflows/runtime/timer_executor.go` - New executor
- `pkg/models/workflow.go` - Add `StepStatusWaitingTimer`

### Phase 13 - TryCatch Step Executor (1d)

Implement error handling with try/catch/finally semantics.

- [ ] Create `TryCatchExecutor` implementing `StepExecutor` interface
- [ ] Execute `try` states, capture errors
- [ ] On error: set `_error` context, execute `catch` states
- [ ] Always execute `finally` states
- [ ] Add tests for error propagation

**Files to create/modify**:
- `internal/workflows/runtime/trycatch_executor.go` - New executor
- `internal/workflows/types.go` - Add `TryBlock`, `CatchBlock`, `FinallyBlock` to StateSpec

### Phase 14 - Observability + Docs (1-2d)

- [ ] OpenTelemetry spans + NATS trace propagation
- [ ] Add run/step metrics
- [ ] Deployment guides (Compose, ECS, GCP, Fly.io)
- [ ] Embedded vs external NATS guidance

---

## 10) Success Metrics (updated)

### Reliability

- **Crash recovery**: 99%+ of test runs resume correctly after forced restart
- **No lost progress**: No run transitions backwards or repeats succeeded steps

### Capability

- All V1 state types working: operation, switch, parallel, inject, foreach
- Both executors working: agent.run, human.approval
- **Agent name resolution**: Agents referenced by name (unique per environment), not ID
- **Schema validation**: 
  - Static validation at workflow create/update time
  - Runtime input validation before agent execution
  - Runtime output validation after agent execution (warning only)
  - Clear error messages for schema mismatches
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
