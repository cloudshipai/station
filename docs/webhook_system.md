# Station Webhook System

The Station webhook system provides real-time notifications for agent execution events. When agents complete their runs, webhooks can be triggered to notify external systems.

## Features

- **Event-driven notifications**: Get notified when agent runs complete
- **Secure delivery**: HMAC-SHA256 signature verification for security
- **Retry mechanism**: Automatic retry with exponential backoff for failed deliveries
- **Multiple webhooks**: Support for multiple webhook endpoints
- **Custom headers**: Add custom HTTP headers to webhook requests
- **Delivery tracking**: Full audit trail of webhook deliveries

## Quick Start

### 1. Enable Notifications

First, enable webhook notifications globally:

```bash
# Via CLI
stn settings set notifications_enabled true

# Via API
curl -X PUT http://localhost:8080/api/v1/settings/notifications_enabled \
  -H "Content-Type: application/json" \
  -d '{"value": "true", "description": "Enable webhook notifications"}'
```

### 2. Create a Webhook

```bash
# Via CLI
stn webhook create --name "My Webhook" \
  --url "https://example.com/webhook" \
  --secret "my-secret-key" \
  --events "agent_run_completed"

# Via API
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "My Webhook",
    "url": "https://example.com/webhook",
    "events": ["agent_run_completed"],
    "secret": "my-secret-key",
    "timeout_seconds": 30,
    "retry_attempts": 3,
    "enabled": true
  }'
```

### 3. Test Your Webhook

Run an agent to trigger a webhook notification:

```bash
# Create and run an agent
stn agent run <agent-id> "Test task"

# Check webhook deliveries
stn webhook deliveries
```

## Webhook Payload Format

When an agent run completes, Station sends a POST request to your webhook URL with the following JSON payload:

```json
{
  "event": "agent_run_completed",
  "timestamp": "2024-01-15T10:30:45Z",
  "agent": {
    "id": 1,
    "name": "My Agent",
    "description": "Agent description",
    "environment_id": 1,
    "created_at": "2024-01-15T10:00:00Z"
  },
  "run": {
    "id": 123,
    "agent_id": 1,
    "user_id": 1,
    "task": "Original task",
    "final_response": "Agent response",
    "status": "completed",
    "steps_taken": 3,
    "started_at": "2024-01-15T10:30:00Z",
    "completed_at": "2024-01-15T10:30:45Z"
  }
}
```

## HTTP Headers

Station includes several headers with webhook requests:

- `User-Agent`: `Station-Webhook/1.0`
- `Content-Type`: `application/json`
- `X-Station-Event`: Event type (e.g., `agent_run_completed`)
- `X-Station-Timestamp`: ISO 8601 timestamp
- `X-Station-Delivery`: Unique delivery ID
- `X-Station-Signature`: HMAC-SHA256 signature (if secret is configured)

## Signature Verification

If you configure a secret for your webhook, Station will include an HMAC-SHA256 signature in the `X-Station-Signature` header:

```
X-Station-Signature: sha256=<hex-encoded-signature>
```

### Verification Examples

**Go:**
```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
)

func verifySignature(payload []byte, signature, secret string) bool {
    h := hmac.New(sha256.New, []byte(secret))
    h.Write(payload)
    expected := "sha256=" + hex.EncodeToString(h.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expected))
}
```

**Python:**
```python
import hmac
import hashlib

def verify_signature(payload, signature, secret):
    expected = "sha256=" + hmac.new(
        secret.encode('utf-8'),
        payload,
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(signature, expected)
```

**Node.js:**
```javascript
const crypto = require('crypto');

function verifySignature(payload, signature, secret) {
    const expected = 'sha256=' + crypto
        .createHmac('sha256', secret)
        .update(payload)
        .digest('hex');
    return crypto.timingSafeEqual(
        Buffer.from(signature),
        Buffer.from(expected)
    );
}
```

## CLI Commands

### Managing Webhooks

```bash
# List all webhooks
stn webhook list

# Create a webhook
stn webhook create --name "My Webhook" --url "https://example.com/webhook"

# Show webhook details
stn webhook show <webhook-id>

# Enable/disable a webhook
stn webhook enable <webhook-id>
stn webhook disable <webhook-id>

# Delete a webhook
stn webhook delete <webhook-id> --confirm

# View delivery history
stn webhook deliveries --limit 50
```

### Managing Settings

```bash
# List all settings
stn settings list

# Get a specific setting
stn settings get notifications_enabled

# Update a setting
stn settings set notifications_enabled true
```

## API Endpoints

### Webhooks

- `GET /api/v1/webhooks` - List all webhooks
- `POST /api/v1/webhooks` - Create a new webhook
- `GET /api/v1/webhooks/{id}` - Get webhook details
- `PUT /api/v1/webhooks/{id}` - Update a webhook
- `DELETE /api/v1/webhooks/{id}` - Delete a webhook
- `PUT /api/v1/webhooks/{id}/enable` - Enable a webhook
- `PUT /api/v1/webhooks/{id}/disable` - Disable a webhook

### Webhook Deliveries

- `GET /api/v1/webhook-deliveries` - List delivery history
- `GET /api/v1/webhook-deliveries/{id}` - Get delivery details

### Settings

- `GET /api/v1/settings` - List all settings
- `GET /api/v1/settings/{key}` - Get a specific setting
- `PUT /api/v1/settings/{key}` - Update a setting

## Testing

Station includes tools to help you test your webhook system:

### 1. Test Server

Use the included test server to receive and inspect webhook payloads:

```bash
# Start the test server
go run scripts/webhook_test_server.go 8888

# In another terminal, create a webhook pointing to the test server
stn webhook create --name "Test" --url "http://localhost:8888/webhook"

# Run an agent to trigger the webhook
stn agent run <agent-id> "test task"
```

### 2. End-to-End Test

Run the complete test suite:

```bash
# Start Station server
go run cmd/main/*.go serve

# In another terminal, run the test
./scripts/test_webhook_system.sh
```

## Configuration

### Environment Variables

- `STATION_API_KEY`: API key for authentication (if required)
- `WEBHOOK_SECRET`: Secret for signature verification (test server)

### Server Configuration

Webhook settings can be configured globally:

- `notifications_enabled`: Enable/disable webhook notifications
- Default timeout: 30 seconds
- Default retry attempts: 3
- Retry interval: Exponential backoff (1s, 2s, 4s, 8s, ...)

## Troubleshooting

### Common Issues

1. **Webhooks not firing**
   - Check if notifications are enabled: `stn settings get notifications_enabled`
   - Verify webhook is enabled: `stn webhook list`
   - Check agent execution logs

2. **Signature verification failures**
   - Ensure secret matches between webhook config and receiving server
   - Verify payload is read as raw bytes before verification
   - Check for encoding issues

3. **Delivery failures**
   - Check webhook URL is accessible
   - Verify firewall/network settings
   - Review delivery logs: `stn webhook deliveries`

### Debug Mode

Enable debug logging to see detailed webhook processing:

```bash
go run cmd/main/*.go serve --debug
```

## Security Best Practices

1. **Always use HTTPS** for webhook URLs in production
2. **Configure secrets** for signature verification
3. **Validate signatures** on the receiving end
4. **Use allowlists** to restrict webhook destinations if needed
5. **Monitor delivery logs** for suspicious activity
6. **Rotate secrets** periodically

## Examples

### Simple Webhook Receiver (Go)

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

type WebhookPayload struct {
    Event     string      `json:"event"`
    Timestamp string      `json:"timestamp"`
    Agent     interface{} `json:"agent"`
    Run       interface{} `json:"run"`
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", 400)
        return
    }

    var payload WebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        http.Error(w, "Invalid JSON", 400)
        return
    }

    log.Printf("Received %s event at %s", payload.Event, payload.Timestamp)
    
    // Process the webhook...
    
    w.WriteHeader(200)
    w.Write([]byte("OK"))
}

func main() {
    http.HandleFunc("/webhook", webhookHandler)
    log.Println("Webhook server listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Integration with Popular Services

**Slack Webhook:**
```bash
stn webhook create --name "Slack Notifications" \
  --url "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK" \
  --headers "Content-Type=application/json"
```

**Discord Webhook:**
```bash
stn webhook create --name "Discord Notifications" \
  --url "https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN"
```

**Microsoft Teams:**
```bash
stn webhook create --name "Teams Notifications" \
  --url "https://your-tenant.webhook.office.com/webhookb2/YOUR_WEBHOOK_URL"
```