# Station Workflow Testing Playbook

**Version**: 1.0  
**Last Updated**: 2025-12-25  
**Purpose**: Comprehensive guide for manual and automated testing of Station workflows

---

## Quick Start

### Prerequisites

```bash
# 1. Install UI and build binary
cd station
make local-install-ui

# 2. Start API server (separate terminal)
export STN_DEV_MODE=true
stn serve

# 3. Start UI dev server (separate terminal)  
cd ui
npm run dev

# 4. Verify services
curl http://localhost:8585/api/v1/version      # API
curl http://localhost:5173                       # UI (Vite dev)
```

### Service Ports

| Service | Port | Purpose |
|---------|------|---------|
| Station API | 8585 | REST API, WebSocket |
| UI (Vite Dev) | 5173 | Frontend development |
| NATS | 4222 | Message queue (embedded) |
| NATS Monitor | 8222 | NATS dashboard |

---

## Testing Interfaces

### 1. API Testing (curl)

```bash
# List workflows
curl -s http://localhost:8585/api/v1/workflows | jq '.workflows[] | {id, name, status}'

# List agents
curl -s http://localhost:8585/api/v1/agents | jq '.agents[] | {id, name}'

# Create a workflow
curl -X POST http://localhost:8585/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d @workflow.json

# Start a workflow run
curl -X POST http://localhost:8585/api/v1/workflows/{workflowId}/runs \
  -H "Content-Type: application/json" \
  -d '{"input": {"namespace": "production"}}'

# Get run status
curl -s http://localhost:8585/api/v1/workflow-runs/{runId} | jq '.run.status'

# Get run steps
curl -s http://localhost:8585/api/v1/workflow-runs/{runId}/steps | jq '.steps'

# Stream run updates (SSE)
curl -N http://localhost:8585/api/v1/workflow-runs/{runId}/stream
```

### 2. CLI Testing (stn)

```bash
# List workflows
stn workflow list

# Get workflow details
stn workflow get <workflow-id>

# Run workflow
stn workflow run <workflow-id> --input '{"namespace": "production"}'

# Watch run
stn workflow run <workflow-id> --tail

# List runs
stn workflow runs <workflow-id>

# Get run status
stn workflow run-status <run-id>
```

### 3. UI Testing (Browser)

Navigate to http://localhost:5173 and test:
- [ ] Workflows list page
- [ ] Workflow detail view
- [ ] Create/Edit workflow
- [ ] Run workflow
- [ ] View run execution
- [ ] Step visualization
- [ ] Real-time updates

---

## Workflow Step Types Reference

### All Supported Step Types

| Type | Purpose | Key Properties |
|------|---------|----------------|
| `operation` | Execute action (agent.run, human.approval) | `input`, `action`, `timeoutSeconds`, `retry` |
| `switch` | Conditional routing | `dataPath`, `conditions`, `defaultNext` |
| `parallel` | Fan-out/fan-in | `branches`, `join.mode` |
| `inject` | Add data to context | `data`, `resultPath` |
| `foreach` | Iterate over list | `itemsPath`, `itemName`, `iterator` |
| `cron` | Scheduled trigger | `cron`, `timezone`, `enabled` |
| `timer` | Delay execution | `duration` ("30s", "5m", "1h") |
| `try_catch` | Error handling | `try`, `catch`, `finally` |

### Available Executors

| Action | Purpose |
|--------|---------|
| `agent.run` | Run Station agent |
| `human.approval` | Wait for human approval |

---

## Test Scenarios

### Scenario 1: Basic Sequential Workflow

**Goal**: Test simple step-by-step execution

```yaml
id: test-sequential
name: "Sequential Test"
start: step1
states:
  - id: step1
    type: inject
    data: { value: 1 }
    transition: step2
  - id: step2
    type: inject
    data: { value: 2 }
    transition: step3
  - id: step3
    type: inject
    data: { value: 3 }
    end: true
```

**Verify**:
- [ ] All steps execute in order
- [ ] Context accumulates data
- [ ] Run completes with status "completed"

---

### Scenario 2: Parallel Execution

**Goal**: Test parallel branch fan-out/fan-in

```yaml
id: test-parallel
name: "Parallel Test"
start: gather
states:
  - id: gather
    type: parallel
    branches:
      - name: branch_a
        states:
          - { id: a1, type: inject, data: { a: true }, end: true }
      - name: branch_b
        states:
          - { id: b1, type: inject, data: { b: true }, end: true }
    join: { mode: all }
    transition: done
  - id: done
    type: inject
    data: { complete: true }
    end: true
```

**Verify**:
- [ ] Both branches execute (can be concurrent)
- [ ] Join waits for all branches
- [ ] Results from both branches in context
- [ ] Final step executes after join

---

### Scenario 3: Conditional Routing (Switch)

**Goal**: Test switch statement with Starlark conditions

```yaml
id: test-switch
name: "Switch Test"
start: init
states:
  - id: init
    type: inject
    data: { error_rate: 0.08 }
    transition: decide
  - id: decide
    type: switch
    dataPath: ctx
    conditions:
      - if: "error_rate > 0.05"
        next: high_alert
      - if: "error_rate > 0.01"
        next: warning
    defaultNext: ok
  - id: high_alert
    type: inject
    data: { action: "page_oncall" }
    end: true
  - id: warning
    type: inject
    data: { action: "log_warning" }
    end: true
  - id: ok
    type: inject
    data: { action: "all_good" }
    end: true
```

**Verify**:
- [ ] Correct branch taken based on condition
- [ ] defaultNext works when no conditions match
- [ ] Starlark expressions evaluate correctly

---

### Scenario 4: Foreach Loop

**Goal**: Test iteration over list

```yaml
id: test-foreach
name: "Foreach Test"
start: init
states:
  - id: init
    type: inject
    data:
      services: ["api", "web", "worker"]
    transition: check_each
  - id: check_each
    type: foreach
    itemsPath: ctx.services
    itemName: service
    iterator:
      start: check
      states:
        - id: check
          type: inject
          data: { checked: "{{ service }}" }
          end: true
    transition: done
  - id: done
    type: inject
    data: { complete: true }
    end: true
```

**Verify**:
- [ ] Loop executes for each item
- [ ] Results collected for all iterations
- [ ] Final step executes after loop

---

### Scenario 5: Agent Execution

**Goal**: Test actual agent invocation

```yaml
id: test-agent
name: "Agent Test"
start: run_agent
states:
  - id: run_agent
    type: operation
    input:
      task: agent.run
      agent: "k8s-investigator"
      agent_task: "Check pod status in default namespace"
    end: true
```

**Verify**:
- [ ] Agent resolved by name
- [ ] Agent executes with task
- [ ] Agent output stored in context
- [ ] Error handling if agent fails

---

### Scenario 6: Human Approval Gate

**Goal**: Test approval workflow

```yaml
id: test-approval
name: "Approval Test"  
start: request
states:
  - id: request
    type: operation
    input:
      task: human.approval
      message: "Approve this action?"
    transition: approved
  - id: approved
    type: inject
    data: { approved: true }
    end: true
```

**Verify**:
- [ ] Run pauses at approval step
- [ ] Approval request created
- [ ] POST to approve endpoint works
- [ ] Run resumes after approval

---

### Scenario 7: Cron Scheduling

**Goal**: Test scheduled workflow triggers

```yaml
id: test-cron
name: "Cron Test"
start: schedule
states:
  - id: schedule
    type: cron
    cron: "* * * * *"  # Every minute (for testing)
    enabled: true
    input:
      triggered_at: "{{ now }}"
    transition: log_trigger
  - id: log_trigger
    type: inject
    data: { executed: true }
    end: true
```

**Verify**:
- [ ] Scheduler picks up workflow
- [ ] Run created on schedule
- [ ] Input injected correctly
- [ ] Manual runs still work

---

### Scenario 8: Timer/Delay

**Goal**: Test timed delays

```yaml
id: test-timer
name: "Timer Test"
start: step1
states:
  - id: step1
    type: inject
    data: { started: true }
    transition: wait
  - id: wait
    type: timer
    duration: "10s"
    transition: done
  - id: done
    type: inject
    data: { complete: true }
    end: true
```

**Verify**:
- [ ] Run pauses at timer step
- [ ] Resumes after duration
- [ ] Duration is durable (survives restart)

---

### Scenario 9: Try/Catch Error Handling

**Goal**: Test error recovery

```yaml
id: test-try-catch
name: "Try Catch Test"
start: risky
states:
  - id: risky
    type: try_catch
    try:
      start: might_fail
      states:
        - id: might_fail
          type: operation
          input:
            task: agent.run
            agent: "nonexistent-agent"  # Will fail
            agent_task: "This will error"
          end: true
    catch:
      start: recover
      states:
        - id: recover
          type: inject
          data: { recovered: true, error: "{{ ctx._errorMessage }}" }
          end: true
    transition: done
  - id: done
    type: inject
    data: { complete: true }
    end: true
```

**Verify**:
- [ ] Try block executes
- [ ] On error, catch block executes
- [ ] Error context available (_error, _errorMessage)
- [ ] Workflow continues after catch

---

## Observation Points

### 1. Server Logs

```bash
# Tail server logs
tail -f /tmp/station-server.log

# Key patterns to watch:
# - "NATS Engine: Publishing step"
# - "Workflow consumer: executing step"
# - "Workflow consumer: run completed"
# - Any error messages
```

### 2. NATS Monitoring

```bash
# Open NATS dashboard
open http://localhost:8222

# View streams
curl http://localhost:8222/streaming/channelsz?subs=1
```

### 3. Database Inspection

```bash
# Query workflow runs
sqlite3 station.db "SELECT run_id, workflow_id, status, current_step FROM workflow_runs ORDER BY created_at DESC LIMIT 10;"

# Query run steps
sqlite3 station.db "SELECT run_id, step_id, status, started_at, completed_at FROM workflow_run_steps WHERE run_id = '<run-id>';"
```

### 4. UI Verification

For each workflow run:
- [ ] Run appears in list immediately
- [ ] Status updates in real-time
- [ ] Steps shown with proper state
- [ ] Flow diagram highlights current step
- [ ] Completed runs show full history

---

## Bug Patterns & Fixes

### Pattern: UNIQUE Constraint Failure

**Symptom**: `UNIQUE constraint failed: workflow_run_steps.run_id, workflow_run_steps.step_id`

**Cause**: NATS message redelivery after restart

**Fix**: Make RecordStepStart idempotent - check for existing step before insert

### Pattern: Missing Run Context

**Symptom**: `sql: no rows in result set` for run context

**Cause**: Stale NATS messages referencing deleted runs

**Fix**: Consumer should skip gracefully when run not found

### Pattern: Step Not Executing

**Symptom**: Run stuck at pending step

**Check**:
1. Is NATS consumer running? Check logs
2. Is message in queue? Check NATS dashboard
3. Is step record created? Check database

---

## Test Execution Checklist

### Before Testing

- [ ] Server running (`pgrep -f "stn serve"`)
- [ ] UI dev server running (`pgrep -f "vite"`)
- [ ] Database accessible (`ls station.db`)
- [ ] Logs capturing (`tail -f /tmp/station-server.log`)

### After Testing

- [ ] All test workflows created
- [ ] Each workflow run verified
- [ ] Logs reviewed for errors
- [ ] UI displays correctly
- [ ] Database state consistent

---

## Realistic DevOps Workflow Templates

### 1. Daily Health Check (Cron + Parallel + Switch)

Uses: cron, parallel, switch, agent.run

### 2. Incident Response (Sequential + Approval + Try/Catch)

Uses: operation, human.approval, try_catch, agent.run

### 3. Multi-Service Deployment (Foreach + Parallel + Approval)

Uses: foreach, parallel, human.approval, agent.run

### 4. Alert Investigation Pipeline (Switch + Parallel + Timer)

Uses: inject, switch, parallel, timer, agent.run

See `docs/workflows/devops-examples/` for full definitions.

---

## Cleanup Commands

```bash
# Stop services
pkill -f "stn serve"
pkill -f "vite"

# Clear workflow runs (for fresh testing)
sqlite3 station.db "DELETE FROM workflow_run_steps;"
sqlite3 station.db "DELETE FROM workflow_runs;"

# Purge NATS streams (if needed)
# Connect to NATS and purge workflow streams
```

---

**End of Playbook**
