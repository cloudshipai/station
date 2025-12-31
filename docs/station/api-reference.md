# Station API Reference

Station exposes two HTTP servers with different purposes:

| Port | Server | Purpose |
|------|--------|---------|
| **8585** | API Server | Full REST API, Web UI, management |
| **8587** | Dynamic Agent MCP | Webhooks, MCP protocol, external integrations |

## Quick Reference

### Port 8587 - Dynamic Agent MCP (Webhooks & MCP)

Lightweight server for external integrations:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/execute` | POST | Execute agent via webhook |
| `/mcp` | POST | MCP protocol (AI tool integration) |
| `/workflow-approvals/{id}` | GET | Get workflow approval details |
| `/workflow-approvals/{id}/approve` | POST | Approve a workflow step |
| `/workflow-approvals/{id}/reject` | POST | Reject a workflow step |

### Port 8585 - API Server (Full API)

Complete REST API for management and status:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/runs` | GET | List agent runs |
| `/api/v1/runs/:id` | GET | **Get agent run status** |
| `/api/v1/runs/:id/logs` | GET | Get agent run logs |
| `/api/v1/workflows` | GET/POST | List/create workflows |
| `/api/v1/workflow-runs` | POST | **Trigger a workflow** |
| `/api/v1/workflow-runs/:id` | GET | Get workflow run status |
| `/api/v1/workflow-runs/:id/cancel` | POST | Cancel workflow run |
| `/api/v1/workflow-approvals` | GET | List pending approvals |

---

# Port 8587: Dynamic Agent MCP Server

The Dynamic Agent MCP server exposes your Station agents as MCP tools and provides HTTP APIs for agent execution and workflow approvals. It runs on port **8587** by default.

## Overview

| Feature | Description |
|---------|-------------|
| **Port** | 8587 (configurable) |
| **Protocol** | HTTP/1.1, MCP over HTTP |
| **Auth** | Local mode (no auth), Static API key, User API keys, CloudShip OAuth |
| **Agents** | Dynamically loaded from the configured environment |

---

## Health Check

Check if the Dynamic Agent MCP server is running.

**Request:**
```http
GET /health HTTP/1.1
Host: localhost:8587
```

**Response:**
```json
{
  "status": "ok",
  "environment": "default"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always `"ok"` when healthy |
| `environment` | string | The environment agents are loaded from |

**Example:**
```bash
curl http://localhost:8587/health
```

---

## Execute Agent (Webhook)

Trigger agent execution from external systems. See [Webhook Execute](./webhook-execute.md) for complete documentation.

**Request:**
```http
POST /execute HTTP/1.1
Host: localhost:8587
Content-Type: application/json
Authorization: Bearer <token>

{
  "agent_name": "coder",
  "task": "Say hello"
}
```

**Response (202 Accepted):**
```json
{
  "run_id": 1,
  "agent_id": 10,
  "agent_name": "coder",
  "status": "running",
  "message": "Agent execution started"
}
```

**Example:**
```bash
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "Say hello"}'
```

---

## MCP Protocol

The Dynamic Agent MCP server implements the [Model Context Protocol](https://modelcontextprotocol.io/) for AI tool integration. Each agent in your environment is exposed as an MCP tool.

### Connecting MCP Clients

**Claude Desktop / Cursor:**

Add to your MCP config (`.mcp.json` or `claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "station-agents": {
      "url": "http://localhost:8587/mcp",
      "transport": "streamable-http"
    }
  }
}
```

**Programmatic Access:**

Use an MCP client library to connect:

```python
from mcp import ClientSession
from mcp.client.streamable_http import StreamableHTTPTransport

async with ClientSession(
    transport=StreamableHTTPTransport("http://localhost:8587/mcp")
) as session:
    # List available tools (agents)
    tools = await session.list_tools()
    for tool in tools:
        print(f"Agent: {tool.name}")
    
    # Execute an agent
    result = await session.call_tool(
        "agent_coder",
        {"input": "Write a hello world function"}
    )
```

### Tool Naming Convention

Agents are exposed as tools with the `agent_` prefix:

| Agent Name | MCP Tool Name |
|------------|---------------|
| `coder` | `agent_coder` |
| `incident-responder` | `agent_incident-responder` |
| `k8s-investigator` | `agent_k8s-investigator` |

### Tool Schema

Each agent tool accepts:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `input` | string | Yes | The task or prompt to send to the agent |
| `variables` | object | No | Variables for dotprompt template rendering |

**Example tool call:**
```json
{
  "name": "agent_coder",
  "arguments": {
    "input": "Write a Python function that calculates fibonacci",
    "variables": {
      "language": "python",
      "style": "functional"
    }
  }
}
```

---

## Workflow Approvals

External API for approving or rejecting workflow steps that require human intervention.

### Get Approval Details

Retrieve information about a pending workflow approval.

**Request:**
```http
GET /workflow-approvals/{approval_id} HTTP/1.1
Host: localhost:8587
Authorization: Bearer <token>
```

**Response (200 OK):**
```json
{
  "approval": {
    "id": "abc123",
    "workflow_id": 5,
    "step_name": "deploy-production",
    "status": "pending",
    "requested_at": "2025-12-31T13:00:00Z",
    "context": {
      "environment": "production",
      "version": "v1.2.3"
    }
  }
}
```

**Example:**
```bash
curl http://localhost:8587/workflow-approvals/abc123 \
  -H "Authorization: Bearer your-api-key"
```

### Approve Workflow Step

Approve a pending workflow step to allow it to proceed.

**Request:**
```http
POST /workflow-approvals/{approval_id}/approve HTTP/1.1
Host: localhost:8587
Content-Type: application/json
Authorization: Bearer <token>

{
  "comment": "Approved for production deployment"
}
```

**Response (200 OK):**
```json
{
  "approval": {
    "id": "abc123",
    "status": "approved",
    "approved_by": "user@example.com",
    "approved_at": "2025-12-31T13:30:00Z"
  },
  "message": "Workflow step approved"
}
```

**Example:**
```bash
curl -X POST http://localhost:8587/workflow-approvals/abc123/approve \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"comment": "LGTM"}'
```

### Reject Workflow Step

Reject a pending workflow step to stop the workflow.

**Request:**
```http
POST /workflow-approvals/{approval_id}/reject HTTP/1.1
Host: localhost:8587
Content-Type: application/json
Authorization: Bearer <token>

{
  "reason": "Needs security review first"
}
```

**Response (200 OK):**
```json
{
  "approval": {
    "id": "abc123",
    "status": "rejected",
    "rejected_by": "user@example.com",
    "rejected_at": "2025-12-31T13:30:00Z",
    "reason": "Needs security review first"
  },
  "message": "Workflow step rejected"
}
```

**Example:**
```bash
curl -X POST http://localhost:8587/workflow-approvals/abc123/reject \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{"reason": "Missing compliance sign-off"}'
```

### Error Responses

**401 Unauthorized:**
```json
{
  "error": "unauthorized",
  "message": "bearer token required"
}
```

**404 Not Found:**
```json
{
  "error": "not_found",
  "message": "Approval not found: xyz789"
}
```

**400 Bad Request:**
```json
{
  "error": "approval_failed",
  "message": "Approval already processed"
}
```

**503 Service Unavailable:**
```json
{
  "error": "service_unavailable",
  "message": "Workflow service not configured"
}
```

---

## Authentication

The Dynamic Agent MCP server supports multiple authentication methods:

| Method | Priority | Use Case |
|--------|----------|----------|
| Local Mode | Highest | Development - no auth required |
| Static API Key | High | Production webhooks, CI/CD |
| User API Key (`sk-*`) | Medium | Individual user access |
| CloudShip OAuth | Low | Enterprise SSO integration |

### Local Mode

When Station runs with `local_mode: true`, all requests are authenticated automatically:

```bash
# No auth needed
curl http://localhost:8587/health
curl -X POST http://localhost:8587/execute -d '{"agent_name": "coder", "task": "hello"}'
```

### Static API Key

Set via environment variable for production deployments:

```bash
export STN_WEBHOOK_API_KEY="your-secret-key-here"
```

```bash
curl -X POST http://localhost:8587/execute \
  -H "Authorization: Bearer your-secret-key-here" \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "hello"}'
```

### User API Key

Station-issued API keys with `sk-` prefix:

```bash
curl -X POST http://localhost:8587/execute \
  -H "Authorization: Bearer sk-abc123..." \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "hello"}'
```

### CloudShip OAuth

When OAuth is enabled, CloudShip access tokens are accepted:

```bash
curl -X POST http://localhost:8587/execute \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "hello"}'
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STN_WEBHOOK_ENABLED` | `true` | Enable/disable the `/execute` endpoint |
| `STN_WEBHOOK_API_KEY` | (empty) | Static API key for authentication |

### Config File

```yaml
# config.yaml
webhook:
  enabled: true
  api_key: "your-secret-key"

cloudship:
  oauth:
    enabled: false
    # OAuth settings if enabled
```

---

## Deployment Modes

### Development (`stn up <env> --dev`)

All ports exposed for local development:

| Port | Service |
|------|---------|
| 8585 | API Server + Web UI |
| 8586 | MCP Server (for tools) |
| 8587 | Dynamic Agent MCP |

```bash
stn up default --dev
```

### Production (`stn up <env>`)

Only Dynamic Agent MCP port exposed:

| Port | Service |
|------|---------|
| 8587 | Dynamic Agent MCP |

```bash
stn up default
```

This mode is designed for:
- CloudShip integration (agents triggered via NATS)
- Webhook-only deployments
- Headless operation

---

# Port 8585: API Server

The API Server provides the complete REST API for Station management, agent execution status, and workflow orchestration.

## Agent Runs

### List Agent Runs

Get a list of recent agent runs.

**Request:**
```http
GET /api/v1/runs?limit=50&status=completed HTTP/1.1
Host: localhost:8585
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | integer | Max results (default: 50) |
| `status` | string | Filter by status: `completed`, `running`, `failed`, `pending` |

**Response (200 OK):**
```json
{
  "runs": [
    {
      "id": 1,
      "agent_id": 10,
      "agent_name": "coder",
      "task": "Say hello",
      "status": "completed",
      "final_response": "Hello! How can I help you today?",
      "started_at": "2025-12-31T13:29:56Z",
      "completed_at": "2025-12-31T13:29:58Z"
    }
  ],
  "count": 1,
  "total_count": 42,
  "limit": 50
}
```

**Example:**
```bash
curl http://localhost:8585/api/v1/runs?limit=10
```

### Get Agent Run Status

Check the status and result of a specific agent run. Use the `run_id` returned from `/execute`.

**Request:**
```http
GET /api/v1/runs/{run_id} HTTP/1.1
Host: localhost:8585
```

**Response (200 OK):**
```json
{
  "run": {
    "id": 1,
    "agent_id": 10,
    "agent_name": "coder",
    "task": "Say hello",
    "status": "completed",
    "final_response": "Hello! How can I help you today?",
    "steps_taken": 1,
    "tool_calls": [],
    "execution_steps": [...],
    "started_at": "2025-12-31T13:29:56Z",
    "completed_at": "2025-12-31T13:29:58Z",
    "duration_ms": 2000
  }
}
```

| Status | Description |
|--------|-------------|
| `pending` | Run created, not yet started |
| `running` | Agent is executing |
| `completed` | Agent finished successfully |
| `failed` | Agent encountered an error |

**Example - Poll for completion:**
```bash
# Start agent execution
RUN_ID=$(curl -s -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "coder", "task": "Hello"}' | jq -r '.run_id')

# Poll for status
while true; do
  STATUS=$(curl -s http://localhost:8585/api/v1/runs/$RUN_ID | jq -r '.run.status')
  echo "Status: $STATUS"
  if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
    break
  fi
  sleep 1
done

# Get final result
curl http://localhost:8585/api/v1/runs/$RUN_ID | jq '.run.final_response'
```

### Get Agent Run Logs

Get debug logs for a specific run.

**Request:**
```http
GET /api/v1/runs/{run_id}/logs?level=info&limit=100 HTTP/1.1
Host: localhost:8585
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `level` | string | Filter by log level |
| `limit` | integer | Max log entries |

**Response (200 OK):**
```json
{
  "run_id": 1,
  "agent_name": "coder",
  "status": "completed",
  "logs": [
    {"level": "info", "message": "Starting agent execution", "timestamp": "..."},
    {"level": "debug", "message": "LLM response received", "timestamp": "..."}
  ],
  "log_count": 2
}
```

---

## Workflows

### Trigger a Workflow

Start a new workflow run.

**Request:**
```http
POST /api/v1/workflow-runs HTTP/1.1
Host: localhost:8585
Content-Type: application/json

{
  "workflowId": "incident-response",
  "input": {
    "alert_id": "ALT-12345",
    "severity": "high"
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `workflowId` | string | Yes | Workflow definition ID |
| `version` | integer | No | Specific workflow version |
| `environmentId` | integer | No | Environment to run in |
| `input` | object | No | Input data for the workflow |

**Response (201 Created):**
```json
{
  "run": {
    "runId": "abc123-def456",
    "workflowId": "incident-response",
    "status": "running",
    "startedAt": "2025-12-31T14:00:00Z"
  }
}
```

**Example:**
```bash
curl -X POST http://localhost:8585/api/v1/workflow-runs \
  -H "Content-Type: application/json" \
  -d '{
    "workflowId": "incident-response",
    "input": {"alert_id": "ALT-12345", "severity": "high"}
  }'
```

### Get Workflow Run Status

Check the status of a workflow run.

**Request:**
```http
GET /api/v1/workflow-runs/{run_id} HTTP/1.1
Host: localhost:8585
```

**Response (200 OK):**
```json
{
  "run": {
    "runId": "abc123-def456",
    "workflowId": "incident-response",
    "status": "running",
    "currentState": "investigate",
    "startedAt": "2025-12-31T14:00:00Z",
    "input": {"alert_id": "ALT-12345"},
    "output": null,
    "steps": [
      {"state": "start", "status": "completed", "duration": 100},
      {"state": "investigate", "status": "running", "duration": null}
    ]
  }
}
```

| Status | Description |
|--------|-------------|
| `pending` | Workflow created, not started |
| `running` | Workflow is executing |
| `waiting` | Waiting for approval or signal |
| `paused` | Manually paused |
| `completed` | Workflow finished successfully |
| `failed` | Workflow encountered an error |
| `cancelled` | Workflow was cancelled |

**Example:**
```bash
curl http://localhost:8585/api/v1/workflow-runs/abc123-def456
```

### List Workflow Runs

Get a list of workflow runs.

**Request:**
```http
GET /api/v1/workflow-runs?workflowId=incident-response&status=running HTTP/1.1
Host: localhost:8585
```

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `workflowId` | string | Filter by workflow ID |
| `status` | string | Filter by status |
| `limit` | integer | Max results |

### Control Workflow Runs

**Cancel a workflow:**
```bash
curl -X POST http://localhost:8585/api/v1/workflow-runs/abc123/cancel \
  -H "Content-Type: application/json" \
  -d '{"reason": "No longer needed"}'
```

**Pause a workflow:**
```bash
curl -X POST http://localhost:8585/api/v1/workflow-runs/abc123/pause \
  -H "Content-Type: application/json" \
  -d '{"reason": "Waiting for maintenance window"}'
```

**Resume a workflow:**
```bash
curl -X POST http://localhost:8585/api/v1/workflow-runs/abc123/resume
```

**Send a signal:**
```bash
curl -X POST http://localhost:8585/api/v1/workflow-runs/abc123/signal \
  -H "Content-Type: application/json" \
  -d '{"name": "approval_received", "payload": {"approved": true}}'
```

---

## Workflow Approvals (Port 8585)

The API server also provides workflow approval endpoints with additional features.

### List Pending Approvals

```bash
curl http://localhost:8585/api/v1/workflow-approvals
```

### Get Approval with Audit Log

```bash
curl http://localhost:8585/api/v1/workflow-approvals/abc123/audit
```

---

## Complete Integration Example

Here's a complete example showing webhook trigger → status polling → result retrieval:

```bash
#!/bin/bash

# 1. Trigger agent via webhook (port 8587)
echo "Starting agent..."
RESPONSE=$(curl -s -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "incident-responder", "task": "Investigate high CPU on prod-web-01"}')

RUN_ID=$(echo $RESPONSE | jq -r '.run_id')
echo "Run ID: $RUN_ID"

# 2. Poll for completion (port 8585)
echo "Waiting for completion..."
while true; do
  RUN=$(curl -s http://localhost:8585/api/v1/runs/$RUN_ID)
  STATUS=$(echo $RUN | jq -r '.run.status')
  
  case $STATUS in
    "completed")
      echo "Agent completed!"
      echo $RUN | jq '.run.final_response'
      break
      ;;
    "failed")
      echo "Agent failed!"
      echo $RUN | jq '.run'
      exit 1
      ;;
    *)
      echo "Status: $STATUS"
      sleep 2
      ;;
  esac
done
```

---

## Related Documentation

- [Webhook Execute](./webhook-execute.md) - Detailed webhook documentation
- [MCP Tools](./mcp-tools.md) - Station's built-in MCP tools
- [Deployment Modes](./deployment-modes.md) - Container deployment options
- [Workflows](./workflows.md) - Workflow engine documentation
