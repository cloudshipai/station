# Webhook Execute Endpoint

Trigger agent execution from external systems via HTTP webhook. The webhook endpoint enables event-driven agent automation, perfect for integrating Station with CI/CD pipelines, alerting systems, monitoring tools, or any system that can make HTTP requests.

## Overview

The webhook endpoint is available on the Dynamic Agent MCP server (port 8587) and provides:

- **Async Execution**: Fire-and-forget pattern - returns immediately with a run ID
- **Flexible Agent Selection**: Trigger by agent name or ID
- **Variable Support**: Pass variables for dotprompt template rendering
- **Multiple Auth Methods**: Static API key, user API keys, or CloudShip OAuth
- **Environment Filtering**: Only agents in the current environment are accessible

## Endpoint Reference

### Execute Agent

**URL:** `POST /execute`

**Port:** 8587 (Dynamic Agent MCP server)

**Content-Type:** `application/json`

#### Request Body

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `agent_name` | string | One of `agent_name` or `agent_id` required | Name of the agent to execute |
| `agent_id` | integer | One of `agent_name` or `agent_id` required | ID of the agent to execute |
| `task` | string | **Required** | The task/prompt to send to the agent |
| `variables` | object | Optional | Variables for dotprompt template rendering |

#### Example Requests

**By Agent Name:**
```bash
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "incident_coordinator",
    "task": "Investigate high latency on the checkout service"
  }'
```

**By Agent ID:**
```bash
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": 21,
    "task": "Check system health"
  }'
```

**With Variables:**
```bash
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "cost_analyzer",
    "task": "Analyze costs for the specified project",
    "variables": {
      "project_id": "prod-123",
      "region": "us-east-1",
      "time_range": "7d"
    }
  }'
```

#### Success Response

**Status:** `202 Accepted`

```json
{
  "run_id": 120,
  "agent_id": 21,
  "agent_name": "incident_coordinator",
  "status": "running",
  "message": "Agent execution started"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `run_id` | integer | Unique identifier for this execution run |
| `agent_id` | integer | ID of the agent being executed |
| `agent_name` | string | Name of the agent being executed |
| `status` | string | Always `"running"` for successful requests |
| `message` | string | Human-readable status message |

#### Error Responses

**400 Bad Request** - Missing required fields:
```json
{
  "error": "bad_request",
  "message": "Either agent_name or agent_id is required"
}
```

```json
{
  "error": "bad_request",
  "message": "task is required"
}
```

**401 Unauthorized** - Authentication failed:
```json
{
  "error": "unauthorized",
  "message": "bearer token required"
}
```

```json
{
  "error": "unauthorized",
  "message": "invalid API key"
}
```

**404 Not Found** - Agent not found:
```json
{
  "error": "not_found",
  "message": "agent 'unknown_agent' not found in environment 'default'"
}
```

**405 Method Not Allowed** - Wrong HTTP method:
```json
{
  "error": "method_not_allowed"
}
```

**503 Service Unavailable** - Webhook disabled:
```json
{
  "error": "service_unavailable",
  "message": "Webhook endpoint is disabled"
}
```

### Health Check

**URL:** `GET /health`

**Response:**
```json
{
  "status": "ok",
  "environment": "default"
}
```

## Authentication

The webhook supports multiple authentication methods, evaluated in order of priority:

### 1. Local Mode (Development)

When Station runs in local mode (`local_mode: true` in config), no authentication is required. All requests are automatically authenticated as the local admin user.

```bash
# No auth needed in local mode
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "my_agent", "task": "Hello"}'
```

### 2. Static API Key (Recommended for Production)

Set a static API key via environment variable. When configured, this is the **only** valid authentication method (user API keys and OAuth are ignored).

```bash
# Set the API key
export STN_WEBHOOK_API_KEY="your-secret-webhook-key"

# Use it in requests
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-secret-webhook-key" \
  -d '{"agent_name": "my_agent", "task": "Hello"}'
```

**Security Best Practices:**
- Use a strong, randomly generated key (32+ characters)
- Store the key in a secrets manager (AWS Secrets Manager, HashiCorp Vault, etc.)
- Rotate keys periodically
- Use HTTPS in production

### 3. User API Key

If no static API key is configured, Station accepts user API keys (prefixed with `sk-`):

```bash
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-user-api-key" \
  -d '{"agent_name": "my_agent", "task": "Hello"}'
```

User API keys are managed in the Station database and can be created via the API or Web UI.

### 4. CloudShip OAuth

When CloudShip OAuth is enabled, the webhook accepts CloudShip access tokens:

```bash
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIs..." \
  -d '{"agent_name": "my_agent", "task": "Hello"}'
```

OAuth tokens are validated against the CloudShip introspection endpoint.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STN_WEBHOOK_ENABLED` | `true` | Enable or disable the webhook endpoint |
| `STN_WEBHOOK_API_KEY` | (empty) | Static API key for authentication. When set, only this key is valid |

### Config File (config.yaml)

```yaml
webhook:
  enabled: true           # Enable the /execute webhook endpoint
  api_key: "your-key"     # Static API key (optional)
```

### Disabling the Webhook

To completely disable the webhook endpoint:

```bash
export STN_WEBHOOK_ENABLED=false
```

When disabled:
- The `/execute` endpoint returns `503 Service Unavailable`
- The `/health` endpoint remains available
- The MCP endpoints (`/`, `/mcp`) remain available

## Integration Examples

### PagerDuty

Trigger incident investigation when alerts fire:

```bash
# PagerDuty webhook configuration
# URL: https://your-station.example.com:8587/execute
# Headers: Authorization: Bearer ${STN_WEBHOOK_API_KEY}
# Body:
{
  "agent_name": "incident_coordinator",
  "task": "PagerDuty Alert: {{event.title}} - Service: {{event.service}} - Severity: {{event.severity}}"
}
```

### GitHub Actions

Run deployment analysis after each deployment:

```yaml
name: Post-Deployment Analysis

on:
  deployment:
    types: [completed]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - name: Trigger deployment analyzer
        run: |
          curl -X POST ${{ secrets.STATION_WEBHOOK_URL }}/execute \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer ${{ secrets.STATION_WEBHOOK_KEY }}" \
            -d '{
              "agent_name": "deployment_analyzer",
              "task": "Analyze deployment for commit ${{ github.sha }} to ${{ github.event.deployment.environment }}",
              "variables": {
                "commit_sha": "${{ github.sha }}",
                "environment": "${{ github.event.deployment.environment }}",
                "repository": "${{ github.repository }}"
              }
            }'
```

### Datadog Webhooks

Trigger cost analysis when budget alerts fire:

```json
{
  "agent_name": "cost_analyzer",
  "task": "Budget alert triggered: $event.title - Current spend: $event.value",
  "variables": {
    "alert_id": "$event.id",
    "threshold": "$event.threshold"
  }
}
```

### Slack Workflows

Create a Slack workflow that triggers Station agents:

```bash
# Slack workflow webhook step
curl -X POST https://your-station:8587/execute \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $STATION_API_KEY" \
  -d '{
    "agent_name": "support_assistant",
    "task": "{{workflow.input.question}}",
    "variables": {
      "user": "{{workflow.user.name}}",
      "channel": "{{workflow.channel.name}}"
    }
  }'
```

### AWS EventBridge

Trigger agents from AWS events:

```json
{
  "source": ["aws.cloudwatch"],
  "detail-type": ["CloudWatch Alarm State Change"],
  "detail": {
    "state": {
      "value": ["ALARM"]
    }
  }
}
```

EventBridge target configuration:
- **Target:** HTTP endpoint
- **URL:** `https://your-station:8587/execute`
- **Auth:** API key in Authorization header

### Prometheus Alertmanager

Configure Alertmanager to trigger Station agents:

```yaml
# alertmanager.yml
receivers:
  - name: station-webhook
    webhook_configs:
      - url: 'https://your-station:8587/execute'
        http_config:
          authorization:
            type: Bearer
            credentials: your-webhook-api-key
        send_resolved: false
```

Custom template for the webhook body:
```yaml
# You'll need a proxy/transformer to format the payload
{
  "agent_name": "alert_investigator",
  "task": "Alert: {{ .GroupLabels.alertname }} - {{ .CommonAnnotations.summary }}"
}
```

## Monitoring Execution Results

The webhook returns immediately with a `run_id`. To check execution results:

### Via Station API

```bash
# Get run details
curl http://localhost:8585/api/v1/runs/120

# List recent runs
curl http://localhost:8585/api/v1/runs?agent_id=21&limit=10
```

### Via MCP Tools

Use the `inspect_run` MCP tool in your AI assistant:

```
"Show me the results of run 120"
```

### Via Web UI

Navigate to `http://localhost:8585/runs/120` to see full execution details including:
- Agent response
- Tool calls made
- Execution timeline
- Token usage

## Async Execution Model

The webhook uses a fire-and-forget pattern:

1. **Request received** - Validates payload and authentication
2. **Run created** - Creates a database record with status `running`
3. **Response sent** - Returns `202 Accepted` with `run_id`
4. **Agent executes** - Runs asynchronously in background goroutine
5. **Run updated** - Updates database with results when complete

This design ensures:
- Fast response times (typically <100ms)
- No timeout issues for long-running agents
- Webhook callers don't need to wait
- Results are always persisted for later retrieval

## Troubleshooting

### Webhook Not Responding

1. Check if Dynamic Agent MCP server is running on port 8587
2. Verify with health check: `curl http://localhost:8587/health`
3. Check Station logs for startup errors

### Authentication Failures

1. **Local mode**: Ensure `local_mode: true` in config
2. **Static API key**: Verify `STN_WEBHOOK_API_KEY` is set correctly
3. **Bearer token**: Ensure `Authorization: Bearer <token>` header format

### Agent Not Found

1. Verify agent exists: `curl http://localhost:8585/api/v1/agents`
2. Check agent is in the correct environment
3. Use exact agent name (case-sensitive)

### Webhook Disabled

If you see `"error": "service_unavailable"`:
1. Check `STN_WEBHOOK_ENABLED` environment variable
2. Ensure it's set to `true` or unset (defaults to true)

## Security Considerations

### Network Security

- **Use HTTPS** in production (terminate TLS at load balancer or reverse proxy)
- **Firewall rules**: Restrict access to trusted IP ranges
- **VPN/Private network**: Consider running Station in a private network

### Authentication

- **Always use authentication** in production
- **Prefer static API key** for service-to-service communication
- **Rotate keys** periodically

### Input Validation

- The webhook validates required fields but does not sanitize task content
- Agent prompts should be designed to handle potentially malicious input
- Consider rate limiting at the load balancer level

### Audit Logging

All webhook executions are logged:
```
Webhook: Started agent execution (Run ID: 120, Agent: incident_coordinator, Task: Investigate...)
Webhook agent execution completed (Run ID: 120, Agent: incident_coordinator)
```

Logs include:
- Run ID for tracing
- Agent name
- Truncated task (first 50 chars)
- Completion status
