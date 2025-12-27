# Workflow Webhooks & Human-in-the-Loop Approvals

This guide covers webhook notifications for workflow approval steps, enabling human-in-the-loop automation with external integrations like Slack, PagerDuty, or custom systems.

## Table of Contents

1. [Overview](#overview)
2. [Configuration](#configuration)
3. [Webhook Payloads](#webhook-payloads)
4. [Approval API](#approval-api)
5. [Audit Logging](#audit-logging)
6. [Integration Examples](#integration-examples)
7. [Troubleshooting](#troubleshooting)

---

## Overview

When a workflow reaches a `human_approval` step, Station can notify external systems via webhooks. This enables:

- **Slack notifications** with approve/reject buttons
- **PagerDuty escalations** for critical decisions
- **Custom dashboards** showing pending approvals
- **Mobile apps** for on-the-go approvals

### Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        APPROVAL WEBHOOK FLOW                                 │
└─────────────────────────────────────────────────────────────────────────────┘

     ┌──────────────┐
     │   Workflow   │
     │   Running    │
     └──────┬───────┘
            │
            ▼
┌───────────────────────┐
│   human_approval      │
│       step            │
└───────────┬───────────┘
            │
            ▼
┌───────────────────────┐     ┌─────────────────────────────────────┐
│  Create Approval      │     │                                     │
│  Record in DB         │     │         WEBHOOK NOTIFICATION        │
└───────────┬───────────┘     │                                     │
            │                 │  POST https://hooks.slack.com/...   │
            ├────────────────▶│                                     │
            │                 │  {                                  │
            │                 │    "event": "approval.requested",   │
            │                 │    "approval_id": "appr-abc123",    │
            │                 │    "message": "Approve deploy?",    │
            │                 │    "approve_url": "http://...",     │
            │                 │    "reject_url": "http://..."       │
            │                 │  }                                  │
            │                 │                                     │
            │                 └─────────────────────────────────────┘
            │
            ▼
┌───────────────────────┐
│   WORKFLOW PAUSED     │◄───────────────────────────────┐
│   (waiting_approval)  │                                │
└───────────┬───────────┘                                │
            │                                            │
            │         Human reviews in Slack/Dashboard   │
            │                                            │
            ▼                                            │
     ┌──────────────┐                                    │
     │   Decision   │                                    │
     └──────┬───────┘                                    │
            │                                            │
    ┌───────┴───────┐                                    │
    │               │                                    │
    ▼               ▼                                    │
┌────────┐    ┌──────────┐                              │
│APPROVE │    │  REJECT  │                              │
└───┬────┘    └────┬─────┘                              │
    │              │                                    │
    │              │       API Call                     │
    │              ├───────────────────────────────────►│
    │              │  POST /workflow-approvals/{id}/reject
    │              │                                    │
    │              │                                    │
    │    API Call  │                                    │
    ├──────────────┼───────────────────────────────────►│
    │              │  POST /workflow-approvals/{id}/approve
    │              │
    ▼              ▼
┌────────────────────────┐
│   Workflow Continues   │
│   or Fails             │
└────────────────────────┘
```

---

## Configuration

### Enabling Webhooks

Configure webhook notifications in your Station config file (`config.yaml`):

```yaml
# Station config.yaml
api_port: 8587

# Webhook configuration for approval notifications
webhooks:
  approval:
    # Webhook URL to notify when approval is requested
    url: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXX"
    
    # Optional: Custom headers (e.g., for authentication)
    headers:
      Authorization: "Bearer your-token-here"
      X-Custom-Header: "custom-value"
    
    # Optional: Timeout for webhook delivery (default: 30s)
    timeout: 10s
    
    # Optional: Retry configuration
    retries: 3
    retry_delay: 5s
```

### Environment Variables

You can also configure via environment variables:

```bash
export STN_WEBHOOK_APPROVAL_URL="https://hooks.slack.com/services/..."
export STN_WEBHOOK_APPROVAL_TIMEOUT="10s"
```

---

## Webhook Payloads

### Approval Requested Event

When a workflow reaches a `human_approval` step, Station sends:

```json
{
  "event": "approval.requested",
  "approval_id": "appr-abc123def456",
  "workflow_id": "security-review",
  "run_id": "run-xyz789",
  "step_name": "request_approval",
  "message": "Approve deployment of api-service v2.3.0 to production?",
  "metadata": {
    "service": "api-service",
    "version": "2.3.0",
    "environment": "production"
  },
  "approve_url": "http://localhost:8587/workflow-approvals/appr-abc123def456/approve",
  "reject_url": "http://localhost:8587/workflow-approvals/appr-abc123def456/reject",
  "created_at": "2025-01-15T10:30:00Z",
  "timeout_at": "2025-01-15T11:00:00Z"
}
```

### Payload Fields

| Field | Type | Description |
|-------|------|-------------|
| `event` | string | Always `"approval.requested"` |
| `approval_id` | string | Unique ID for this approval request |
| `workflow_id` | string | ID of the workflow definition |
| `run_id` | string | ID of this workflow run instance |
| `step_name` | string | Name of the approval step in the workflow |
| `message` | string | Human-readable message (supports template variables) |
| `metadata` | object | Custom metadata from the workflow step |
| `approve_url` | string | URL to POST for approval |
| `reject_url` | string | URL to POST for rejection |
| `created_at` | string | ISO 8601 timestamp when approval was requested |
| `timeout_at` | string | ISO 8601 timestamp when approval will auto-expire (if timeout set) |

---

## Approval API

### Approve a Request

```bash
POST /workflow-approvals/{approval_id}/approve
Content-Type: application/json

{
  "approved_by": "alice@example.com",
  "comment": "Looks good, approved for deployment"
}
```

**Response (200 OK):**
```json
{
  "approval_id": "appr-abc123def456",
  "status": "approved",
  "approved_by": "alice@example.com",
  "approved_at": "2025-01-15T10:35:00Z"
}
```

### Reject a Request

```bash
POST /workflow-approvals/{approval_id}/reject
Content-Type: application/json

{
  "rejected_by": "bob@example.com",
  "reason": "Missing security review, please add before deploying"
}
```

**Response (200 OK):**
```json
{
  "approval_id": "appr-abc123def456",
  "status": "rejected",
  "rejected_by": "bob@example.com",
  "rejected_at": "2025-01-15T10:35:00Z",
  "reason": "Missing security review, please add before deploying"
}
```

### Get Approval Status

```bash
GET /workflow-approvals/{approval_id}
```

**Response (200 OK):**
```json
{
  "approval_id": "appr-abc123def456",
  "workflow_id": "security-review",
  "run_id": "run-xyz789",
  "step_name": "request_approval",
  "status": "pending",
  "message": "Approve deployment?",
  "created_at": "2025-01-15T10:30:00Z",
  "timeout_at": "2025-01-15T11:00:00Z"
}
```

### List Pending Approvals

```bash
GET /workflow-approvals?status=pending
```

**Response (200 OK):**
```json
{
  "approvals": [
    {
      "approval_id": "appr-abc123",
      "workflow_id": "deploy-prod",
      "status": "pending",
      "message": "Approve production deployment?",
      "created_at": "2025-01-15T10:30:00Z"
    },
    {
      "approval_id": "appr-def456",
      "workflow_id": "security-review",
      "status": "pending",
      "message": "Approve IP block action?",
      "created_at": "2025-01-15T10:25:00Z"
    }
  ],
  "total": 2
}
```

### CLI Commands

Station CLI also supports approval management:

```bash
# List pending approvals
stn workflow approvals list --status pending

# Approve a request
stn workflow approvals approve appr-abc123 --by "alice@example.com" --comment "LGTM"

# Reject a request
stn workflow approvals reject appr-abc123 --by "bob@example.com" --reason "Needs review"

# Get approval details
stn workflow approvals get appr-abc123
```

---

## Audit Logging

All webhook deliveries and approval decisions are logged for compliance and debugging.

### Webhook Delivery Logs

Every webhook attempt is recorded with:

- Request payload sent
- Response status code
- Response body (truncated)
- Duration in milliseconds
- Error message (if failed)

### Get Audit Logs for an Approval

```bash
GET /workflow-approvals/{approval_id}/audit
```

**Response (200 OK):**
```json
{
  "logs": [
    {
      "log_id": "log-001",
      "approval_id": "appr-abc123def456",
      "event_type": "webhook.attempt",
      "webhook_url": "https://hooks.slack.com/services/...",
      "created_at": "2025-01-15T10:30:00.100Z"
    },
    {
      "log_id": "log-002",
      "approval_id": "appr-abc123def456",
      "event_type": "webhook.success",
      "webhook_url": "https://hooks.slack.com/services/...",
      "response_status": 200,
      "duration_ms": 150,
      "created_at": "2025-01-15T10:30:00.250Z"
    },
    {
      "log_id": "log-003",
      "approval_id": "appr-abc123def456",
      "event_type": "approval.approved",
      "metadata": {
        "approved_by": "alice@example.com",
        "comment": "Looks good"
      },
      "created_at": "2025-01-15T10:35:00Z"
    }
  ]
}
```

### Audit Event Types

| Event Type | Description |
|------------|-------------|
| `webhook.attempt` | Webhook delivery started |
| `webhook.success` | Webhook delivered successfully (2xx response) |
| `webhook.failure` | Webhook delivery failed (non-2xx or network error) |
| `approval.approved` | Approval was granted |
| `approval.rejected` | Approval was denied |
| `approval.timeout` | Approval expired without decision |

### Audit Log Schema

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         notification_logs TABLE                              │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  log_id            TEXT PRIMARY KEY                                          │
│  approval_id       TEXT NOT NULL (FK → workflow_approvals)                   │
│  event_type        TEXT NOT NULL                                             │
│  webhook_url       TEXT                                                      │
│  request_payload   TEXT (JSON)                                               │
│  response_status   INTEGER                                                   │
│  response_body     TEXT (truncated to 1KB)                                   │
│  error_message     TEXT                                                      │
│  duration_ms       INTEGER                                                   │
│  metadata          TEXT (JSON)                                               │
│  created_at        DATETIME DEFAULT CURRENT_TIMESTAMP                        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Integration Examples

### Slack Integration

#### Using Slack Incoming Webhooks

1. Create a Slack App and enable Incoming Webhooks
2. Copy the webhook URL
3. Configure in Station:

```yaml
webhooks:
  approval:
    url: "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXX"
```

#### Slack Message Format

The webhook payload can be formatted for Slack's Block Kit:

```json
{
  "blocks": [
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*Approval Required*\n\nApprove deployment of `api-service v2.3.0` to production?"
      }
    },
    {
      "type": "actions",
      "elements": [
        {
          "type": "button",
          "text": { "type": "plain_text", "text": "Approve" },
          "style": "primary",
          "url": "http://station:8587/workflow-approvals/appr-abc123/approve"
        },
        {
          "type": "button",
          "text": { "type": "plain_text", "text": "Reject" },
          "style": "danger",
          "url": "http://station:8587/workflow-approvals/appr-abc123/reject"
        }
      ]
    }
  ]
}
```

### Custom Integration (Express.js Example)

Build a custom approval dashboard:

```javascript
const express = require('express');
const app = express();

// Receive webhook from Station
app.post('/webhooks/station-approval', express.json(), (req, res) => {
  const { event, approval_id, message, approve_url, reject_url } = req.body;
  
  if (event === 'approval.requested') {
    // Store in database, send email, push notification, etc.
    console.log(`New approval: ${approval_id}`);
    console.log(`Message: ${message}`);
    
    // Notify your team
    notifyTeam({ approval_id, message, approve_url, reject_url });
  }
  
  res.status(200).json({ received: true });
});

// Dashboard endpoint to approve
app.post('/approve/:id', async (req, res) => {
  const approvalId = req.params.id;
  const user = req.user.email;
  
  // Call Station API
  const response = await fetch(
    `http://station:8587/workflow-approvals/${approvalId}/approve`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ approved_by: user })
    }
  );
  
  res.redirect('/dashboard');
});

app.listen(3000);
```

### PagerDuty Integration

Configure a webhook transformer for PagerDuty Events API v2:

```yaml
webhooks:
  approval:
    url: "https://events.pagerduty.com/v2/enqueue"
    headers:
      Content-Type: "application/json"
    # Transform payload to PagerDuty format
    transform: |
      {
        "routing_key": "YOUR_INTEGRATION_KEY",
        "event_action": "trigger",
        "payload": {
          "summary": message,
          "severity": "warning",
          "source": "station-workflow",
          "custom_details": {
            "approval_id": approval_id,
            "workflow_id": workflow_id,
            "approve_url": approve_url,
            "reject_url": reject_url
          }
        }
      }
```

---

## Troubleshooting

### Webhook Not Received

1. **Check webhook URL is correct:**
   ```bash
   curl -X POST "YOUR_WEBHOOK_URL" \
     -H "Content-Type: application/json" \
     -d '{"test": true}'
   ```

2. **Check Station logs:**
   ```bash
   STN_LOG_LEVEL=debug stn serve 2>&1 | grep -i webhook
   ```

3. **Check audit logs:**
   ```bash
   curl http://localhost:8587/workflow-approvals/appr-xxx/audit
   ```

### Webhook Returns Non-2xx

Check the audit log for the response body:

```sql
SELECT 
  event_type, 
  response_status, 
  response_body, 
  error_message 
FROM notification_logs 
WHERE approval_id = 'appr-xxx' 
ORDER BY created_at DESC;
```

### Approval URL Not Working

1. **Verify Station is accessible** from where approvals happen:
   ```bash
   curl http://your-station-host:8587/health
   ```

2. **Check approval hasn't expired:**
   ```bash
   stn workflow approvals get appr-xxx
   ```

3. **Check approval wasn't already decided:**
   ```bash
   GET /workflow-approvals/appr-xxx
   # status should be "pending"
   ```

### Workflow Stuck After Approval

1. **Check workflow run status:**
   ```bash
   stn workflow runs get run-xyz --steps
   ```

2. **Check NATS is running:**
   ```bash
   stn status
   ```

3. **Check workflow consumer logs:**
   ```bash
   STN_LOG_LEVEL=debug stn serve 2>&1 | grep -i "workflow\|approval"
   ```

---

## Security Considerations

### Webhook Authentication

Always use HTTPS for webhook URLs. Add authentication headers:

```yaml
webhooks:
  approval:
    url: "https://your-system.example.com/webhooks/station"
    headers:
      Authorization: "Bearer ${WEBHOOK_SECRET}"
      X-Webhook-Signature: "${WEBHOOK_SIGNATURE}"
```

### API Authentication

The approval API endpoints should be protected. Consider:

1. **Network isolation** - Run Station on internal network
2. **API tokens** - Use bearer token authentication
3. **mTLS** - Mutual TLS for service-to-service auth

### Audit Retention

Configure log retention for compliance:

```yaml
audit:
  retention_days: 90  # Keep audit logs for 90 days
```

---

## Next Steps

- See [Workflow Authoring Guide](./workflow-authoring-guide.md) for complete workflow syntax
- Check [Workflow Engine Architecture](../features/workflow-engine-v1.md) for internals
- Read [API Reference](./api-reference.md) for full endpoint documentation
