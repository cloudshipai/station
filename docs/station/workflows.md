# Station Workflow Engine

Station's workflow engine enables you to orchestrate multi-step agent tasks with conditional branching, parallel execution, and human approval gates.

## Quick Start

### 1. Create a Workflow

```bash
curl -X POST http://localhost:8585/api/v1/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "workflowId": "incident-response",
    "name": "Incident Response",
    "definition": {
      "id": "incident-response",
      "start": "analyze",
      "states": [
        {
          "id": "analyze",
          "type": "operation",
          "input": {
            "task": "agent.run",
            "agent_id": 1,
            "prompt": "Analyze the incident"
          },
          "transition": "decide"
        },
        {
          "id": "decide",
          "type": "switch",
          "dataPath": "ctx.severity",
          "conditions": [
            {"if": "_value == \"critical\"", "next": "approval"},
            {"if": "_value == \"high\"", "next": "remediate"}
          ],
          "defaultNext": "notify"
        },
        {
          "id": "approval",
          "type": "operation",
          "input": {
            "task": "human.approval",
            "message": "Critical incident requires approval to proceed"
          },
          "transition": "remediate"
        },
        {
          "id": "remediate",
          "type": "operation",
          "input": {
            "task": "agent.run",
            "agent_id": 2,
            "prompt": "Execute remediation"
          },
          "transition": "notify"
        },
        {
          "id": "notify",
          "type": "inject",
          "data": {"status": "complete"},
          "end": true
        }
      ]
    }
  }'
```

### 2. Start a Workflow Run

```bash
curl -X POST http://localhost:8585/api/v1/workflows/incident-response/runs \
  -H "Content-Type: application/json" \
  -d '{"input": {"incident_id": "INC-123"}}'
```

### 3. Monitor Run Progress

```bash
curl http://localhost:8585/api/v1/workflow-runs/{runId}
curl http://localhost:8585/api/v1/workflow-runs/{runId}/steps
```

### 4. Handle Approvals

```bash
# List pending approvals
curl http://localhost:8585/api/v1/workflow-approvals

# Approve
curl -X POST http://localhost:8585/api/v1/workflow-approvals/{approvalId}/approve \
  -H "Content-Type: application/json" \
  -d '{"comment": "Approved for production"}'

# Reject
curl -X POST http://localhost:8585/api/v1/workflow-approvals/{approvalId}/reject \
  -H "Content-Type: application/json" \
  -d '{"reason": "Requires additional review"}'
```

## Workflow Definition

### State Types

| Type | Description | Example Use Case |
|------|-------------|------------------|
| `operation` | Execute an agent or tool | Run analysis agent |
| `inject` / `set` | Inject data into context | Set status flags |
| `switch` | Conditional branching | Route by severity |
| `parallel` | Execute branches concurrently | Run checks in parallel |
| `foreach` | Iterate over array | Process multiple items |

### Operation Tasks

#### Agent Execution

```json
{
  "id": "run-agent",
  "type": "operation",
  "input": {
    "task": "agent.run",
    "agent_id": 1,
    "prompt": "Analyze the data"
  },
  "transition": "next-step"
}
```

#### Human Approval

```json
{
  "id": "approval",
  "type": "operation",
  "input": {
    "task": "human.approval",
    "message": "Approve deployment to production?",
    "timeout_seconds": 3600
  },
  "transition": "deploy"
}
```

### Data Injection

```json
{
  "id": "set-status",
  "type": "inject",
  "data": {
    "status": "processed",
    "timestamp": "2024-01-01T00:00:00Z"
  },
  "resultPath": "ctx.result",
  "transition": "next-step"
}
```

### Conditional Branching

```json
{
  "id": "check-severity",
  "type": "switch",
  "dataPath": "ctx.severity",
  "conditions": [
    {"if": "_value == \"critical\"", "next": "escalate"},
    {"if": "_value == \"high\"", "next": "alert"},
    {"if": "_value == \"medium\"", "next": "log"}
  ],
  "defaultNext": "ignore"
}
```

### Parallel Execution

```json
{
  "id": "parallel-checks",
  "type": "parallel",
  "branches": [
    {
      "name": "security-scan",
      "states": [
        {"id": "scan", "type": "operation", "input": {"task": "agent.run", "agent_id": 1}, "end": true}
      ]
    },
    {
      "name": "compliance-check",
      "states": [
        {"id": "check", "type": "operation", "input": {"task": "agent.run", "agent_id": 2}, "end": true}
      ]
    }
  ],
  "transition": "merge-results"
}
```

### Foreach Loop

```json
{
  "id": "process-items",
  "type": "foreach",
  "itemsPath": "ctx.items",
  "itemName": "item",
  "maxConcurrency": 5,
  "iterator": {
    "states": [
      {"id": "process", "type": "operation", "input": {"task": "agent.run", "agent_id": 1}, "end": true}
    ]
  },
  "transition": "complete"
}
```

## Web UI

Station includes a web UI for managing workflows:

1. **Workflows List** - View all workflow definitions
2. **Workflow Detail** - Visual flow diagram, runs history, definition viewer
3. **Run Detail** - Execution timeline, step outputs, approval actions
4. **Real-time Updates** - SSE streaming for live status

Access the UI at `http://localhost:8585` after starting Station.

## API Reference

### Workflow Definitions

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/workflows` | GET | List all workflows |
| `/api/v1/workflows` | POST | Create workflow |
| `/api/v1/workflows/{id}` | GET | Get workflow |
| `/api/v1/workflows/{id}` | PUT | Update workflow (creates new version) |
| `/api/v1/workflows/{id}` | DELETE | Disable workflow |
| `/api/v1/workflows/validate` | POST | Validate definition |

### Workflow Runs

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/workflow-runs` | GET | List runs (filter by workflow_id, status) |
| `/api/v1/workflow-runs` | POST | Start new run |
| `/api/v1/workflow-runs/{id}` | GET | Get run details |
| `/api/v1/workflow-runs/{id}/steps` | GET | List run steps |
| `/api/v1/workflow-runs/{id}/stream` | GET | SSE stream for real-time updates |
| `/api/v1/workflow-runs/{id}/cancel` | POST | Cancel run |
| `/api/v1/workflow-runs/{id}/pause` | POST | Pause run |
| `/api/v1/workflow-runs/{id}/resume` | POST | Resume run |

### Approvals

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/workflow-approvals` | GET | List pending approvals |
| `/api/v1/workflow-approvals/{id}` | GET | Get approval details |
| `/api/v1/workflow-approvals/{id}/approve` | POST | Approve step |
| `/api/v1/workflow-approvals/{id}/reject` | POST | Reject step |

## Run Statuses

| Status | Description |
|--------|-------------|
| `pending` | Run created, not yet started |
| `running` | Actively executing steps |
| `waiting_approval` | Paused waiting for human approval |
| `paused` | Manually paused |
| `completed` | Successfully finished |
| `failed` | Execution failed |
| `cancelled` | Manually cancelled |

## Expression Syntax

Conditions use a simple expression language:

```
# Comparison
ctx.value == "expected"
ctx.count > 10
ctx.status != "failed"

# With dataPath scoping
_value == "critical"     # _value refers to dataPath value
_value > 100

# Boolean
true
false
```

## Best Practices

1. **Use descriptive state IDs** - Makes debugging easier
2. **Set timeouts for approvals** - Prevent indefinite waits
3. **Include error handling** - Use switch states for error routing
4. **Version your workflows** - Updates create new versions automatically
5. **Test with validation endpoint** - Catch errors before deployment

## MCP Tools

The workflow engine is also available via MCP:

```bash
# Create workflow
station_create_workflow

# Start run
station_start_workflow_run

# Approve/reject
station_approve_workflow_step
station_reject_workflow_step
```

## Troubleshooting

### Run stuck in "pending"

Check if NATS is running and the workflow consumer started:
```bash
# Check Station logs for NATS connection
stn serve --log-level debug
```

### Approval not resuming

Verify the approval was granted and check the run's current_step:
```bash
curl http://localhost:8585/api/v1/workflow-runs/{runId}
```

### Agent execution failing

Check agent exists and is enabled:
```bash
curl http://localhost:8585/api/v1/agents/{agentId}
```
