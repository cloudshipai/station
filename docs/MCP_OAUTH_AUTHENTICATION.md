# MCP OAuth Authentication

Station supports CloudShip OAuth authentication for the Dynamic Agent MCP endpoint, allowing external MCP clients (like Claude Desktop, Cursor, or custom integrations) to securely execute Station agents using CloudShip-issued OAuth tokens.

## Overview

Station exposes two MCP endpoints:

| Endpoint | Port | Purpose | Authentication |
|----------|------|---------|----------------|
| Management MCP | 8586 | Local Station management (tools, resources, sync) | None (local only) |
| Dynamic Agent MCP | 8587 | External agent execution | CloudShip OAuth or local API key |

**Only the Dynamic Agent MCP endpoint (8587) supports OAuth authentication.** This is the endpoint that external MCP clients connect to for executing Station agents.

## How It Works

```
MCP Client (Claude, Cursor, etc.)
    |
    | Authorization: Bearer <cloudship_oauth_token>
    v
Station Dynamic Agent MCP (port 8587)
    |
    v
Auth Context Function
    |
    +-- 1. Check for Authorization header
    |
    +-- 2. Try local API key (sk-* prefix)
    |
    +-- 3. If not local key, call CloudShip introspect
    |       POST https://app.cloudshipai.com/oauth/introspect/
    |       token=<bearer_token>
    |
    +-- 4. CloudShip returns: {active, user_id, org_id, email, scope}
    |
    +-- 5. Inject user context into request
    v
MCP Tool Handler (agent execution)
```

## Configuration

### Environment Variables

```bash
# Enable CloudShip OAuth for MCP
export STN_CLOUDSHIP_OAUTH_ENABLED=true

# OAuth client ID from CloudShip
export STN_CLOUDSHIP_OAUTH_CLIENT_ID="your-client-id"

# CloudShip introspect endpoint
export STN_CLOUDSHIP_OAUTH_INTROSPECT_URL="https://app.cloudshipai.com/oauth/introspect/"
```

### Config File (config.yaml)

```yaml
cloudship:
  oauth:
    enabled: true
    client_id: "your-client-id"
    introspect_url: "https://app.cloudshipai.com/oauth/introspect/"
```

## Getting an OAuth Token

OAuth tokens are issued by CloudShip. Users authenticate with CloudShip and receive an access token that can be used to authenticate with Station MCP.

### For Development/Testing

1. Create an OAuth application in CloudShip Admin
2. Generate a test access token via the CloudShip API or Admin UI
3. Use the token in your MCP client configuration

### For Production

Users will authenticate via the CloudShip OAuth flow (authorization code with PKCE) and receive tokens automatically.

## MCP Client Configuration

### Claude Desktop (claude_desktop_config.json)

```json
{
  "mcpServers": {
    "station-agents": {
      "url": "http://your-station:8587/mcp",
      "transport": "streamable-http",
      "headers": {
        "Authorization": "Bearer YOUR_CLOUDSHIP_OAUTH_TOKEN"
      }
    }
  }
}
```

### Using the MCP SDK (Node.js)

```javascript
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StreamableHTTPClientTransport } from "@modelcontextprotocol/sdk/client/streamableHttp.js";

const transport = new StreamableHTTPClientTransport(
  new URL("http://localhost:8587/mcp"),
  {
    requestInit: {
      headers: {
        "Authorization": `Bearer ${OAUTH_TOKEN}`
      }
    }
  }
);

const client = new Client({
  name: "my-mcp-client",
  version: "1.0.0",
});

await client.connect(transport);

// List available agent tools
const tools = await client.listTools();

// Execute an agent
const result = await client.callTool({
  name: "agent_My_Agent_Name",
  arguments: { input: "Hello, execute this task" }
});
```

## Authentication Flow

### With OAuth Token

1. Client sends request with `Authorization: Bearer <token>`
2. Station extracts token from header
3. Station calls CloudShip introspect endpoint
4. CloudShip validates token and returns claims:
   ```json
   {
     "active": true,
     "user_id": "uuid",
     "org_id": "uuid", 
     "email": "user@example.com",
     "scope": "read stations"
   }
   ```
5. Station creates user context and allows request

### With Local API Key

1. Client sends request with `Authorization: Bearer sk-xxxxx`
2. Station recognizes `sk-` prefix as local API key
3. Station validates key against local database
4. If valid, request proceeds with local user context

### Without Authentication

If no `Authorization` header is provided, the request proceeds without user context. Individual tools may reject unauthenticated requests.

## Security Considerations

1. **Token Caching**: Station caches validated tokens for 5 minutes to reduce introspect calls
2. **TLS**: In production, always use HTTPS for the MCP endpoint
3. **Scopes**: CloudShip tokens include scopes that can be used for fine-grained access control
4. **Org Isolation**: The `org_id` claim ensures users only access resources within their organization

## Troubleshooting

### Check OAuth is Enabled

Look for this log message on Station startup:
```
Dynamic Agent MCP OAuth authentication enabled
```

### Verify Token Validation

Successful OAuth authentication logs:
```
Dynamic Agent MCP auth: authenticated via CloudShip OAuth (user: user@example.com, org: uuid)
```

### Test Token Manually

```bash
# Test token with CloudShip introspect
curl -X POST https://app.cloudshipai.com/oauth/introspect/ \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "token=YOUR_TOKEN"
```

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| "Invalid session ID" | Using POST without establishing session | Use proper MCP client that handles SSE sessions |
| No auth log messages | OAuth not enabled | Check `STN_CLOUDSHIP_OAUTH_ENABLED=true` |
| Token rejected | Token expired or invalid | Get fresh token from CloudShip |
| Connection refused | Station not running | Start Station with `stn serve` |

## Related Documentation

- [CloudShip OAuth Provider Guide](../../../cloudshipai/backend/docs/OAUTH_PROVIDER_GUIDE.md)
- [Station Configuration](./GETTING_STARTED.md)
- [MCP Protocol Specification](https://modelcontextprotocol.io/)
