# CloudShip Integration Guide

This guide explains how to connect your Station CLI to CloudShip for centralized management, monitoring, and remote agent execution.

## Overview

Station CLI can operate in two modes:
1. **Standalone Mode** - Run agents locally without CloudShip
2. **Connected Mode** - Connect to CloudShip for centralized management

When connected to CloudShip, your station:
- Appears in the CloudShip web dashboard
- Can be monitored remotely (status, agents, tools)
- Receives remote agent execution requests
- Reports run history and telemetry
- Supports high availability with multiple stations per registration key

## Prerequisites

1. A CloudShip account at [cloudshipai.com](https://cloudshipai.com)
2. Station CLI installed (`go install` or download binary)
3. A registration key from CloudShip

## Getting a Registration Key

1. Log in to CloudShip dashboard
2. Navigate to **Settings > Stations**
3. Click **Create Registration Key**
4. Give it a descriptive name (e.g., "Production Servers", "Dev Team")
5. Copy the registration key (starts with `X5c...` or similar)

## Configuration

### Basic Configuration

Add the `cloudship` section to your `config.yaml`:

```yaml
# Station identity
workspace: /path/to/your/workspace
ai_provider: openai
ai_model: gpt-4o-mini

# CloudShip connection
cloudship:
  enabled: true
  registration_key: "YOUR_REGISTRATION_KEY_HERE"
  
  # Required: Unique station name (must be unique across all stations)
  name: "prod-east-1"
  
  # Optional: Tags for filtering and organization
  tags:
    - production
    - us-east-1
    - kubernetes
```

### Configuration Options

| Option | Required | Default | Description |
|--------|----------|---------|-------------|
| `enabled` | No | `false` | Enable CloudShip integration |
| `registration_key` | Yes | - | Your CloudShip registration key |
| `endpoint` | No | `lighthouse.cloudshipai.com:443` | Lighthouse gRPC endpoint |
| `use_tls` | No | `true` | Use TLS for connection (required for production) |
| `skip_tls_verify` | No | `false` | Skip TLS certificate verification (for local dev only) |
| `name` | Yes | - | Unique station name (user-defined) |
| `tags` | No | `[]` | Tags for filtering/organization |

### Station Name

The `name` field is **required** and must be:
- Unique across all stations in your organization
- Descriptive and meaningful (e.g., `prod-east-1`, `dev-laptop-john`)
- Used to identify this station in the CloudShip dashboard

If you run the same station name from a different machine, it will **replace** the existing connection (useful for failover).

### Tags

Tags help you organize and filter stations:

```yaml
cloudship:
  name: "prod-k8s-cluster-1"
  tags:
    - production
    - kubernetes
    - us-east-1
    - team-platform
```

Tags appear in the CloudShip dashboard and can be used for filtering.

## Running in Connected Mode

### Server Mode (Recommended)

For always-on stations that receive remote commands:

```bash
stn serve --config config.yaml
```

Output:
```
✅ Station is running!
🔧 MCP Server: http://localhost:8586/mcp
🤖 Dynamic Agent MCP: http://localhost:8587/mcp
Successfully registered with CloudShip management channel
V2 auth successful: station_id=9f60a9f7-... name=prod-east-1
```

### Stdio Mode

For integration with MCP clients (Claude Desktop, etc.):

```bash
stn stdio --config config.yaml
```

The station will connect to CloudShip in the background while serving MCP requests.

## Verifying Connection

### From Station CLI

Check the logs for:
```
Connected to CloudShip Lighthouse at lighthouse.cloudshipai.com:443 (TLS)
Successfully registered with CloudShip management channel
V2 auth successful: station_id=... name=prod-east-1 heartbeat_interval=1m0s
```

### From CloudShip Dashboard

1. Go to **Stations** in the dashboard
2. Your station should appear with:
   - Green "Online" status badge
   - Station name and hostname
   - Tags (if configured)
   - Agent and tool counts
   - Last heartbeat time

## High Availability (Multiple Stations)

You can run multiple stations with the same registration key for HA:

```yaml
# Station 1: config-east.yaml
cloudship:
  registration_key: "YOUR_KEY"
  name: "prod-east-1"
  tags: ["production", "us-east-1"]

# Station 2: config-west.yaml  
cloudship:
  registration_key: "YOUR_KEY"
  name: "prod-west-1"
  tags: ["production", "us-west-2"]
```

Both stations will appear in the dashboard and can receive remote commands independently.

### Station Limits

Registration keys have a `max_stations` limit (default: 10). Contact support to increase this limit for large deployments.

## Connection Lifecycle

### Initial Connection

1. Station opens gRPC stream to Lighthouse
2. Station sends `StationAuth` message with name, tags, capabilities
3. Lighthouse validates registration key
4. Lighthouse creates/updates Station record in database
5. Lighthouse returns `AuthResult` with station_id
6. Station is now online and ready for commands

### Heartbeat

- Station sends heartbeat every 60 seconds
- Lighthouse updates `last_heartbeat` timestamp
- If no heartbeat for 5 minutes, station is marked offline

### Reconnection

Station automatically reconnects when:
- Network connection is lost
- Lighthouse restarts
- Stream errors occur

Reconnection uses exponential backoff (1s → 30s max).

### Stream Replacement

If you start a station with the same `name` as an existing online station:
- New connection replaces the old one
- Old station receives disconnect notification
- Useful for failover and upgrades

## Troubleshooting

### "Invalid registration key"

- Verify the key is correct (copy-paste from CloudShip)
- Check the key hasn't been revoked
- Ensure the key belongs to your organization

### "Station limit reached"

- Your registration key has reached `max_stations`
- Remove unused stations or request limit increase

### Connection timeout

- Check network connectivity to `lighthouse.cloudshipai.com:443`
- Verify firewall allows outbound HTTPS/gRPC (port 443)
- For local development with local Lighthouse, use `endpoint: "localhost:50051"` and `use_tls: false`

### Station shows offline in dashboard

- Check station logs for errors
- Verify heartbeat is being sent
- Check if another station with same name replaced it

### Debug logging

Enable debug mode for verbose logging:

```bash
stn serve --config config.yaml --debug
```

Or in config:
```yaml
debug: true
```

## OAuth Authentication for MCP

When you expose your Station's MCP server to external clients (Claude Desktop, Cursor, etc.), you can require CloudShip OAuth authentication. This ensures only authorized CloudShip users in your organization can access your Station's agents.

### Who Can Access Your Station?

With OAuth enabled, users must:
1. **Have a CloudShip account** at [cloudshipai.com](https://cloudshipai.com)
2. **Be a member of your organization** (invited via CloudShip dashboard)
3. **Authenticate via the OAuth flow** when connecting

This means you can share your Station's agents with your team while keeping them secure from unauthorized access.

### Setup Overview

| Step | Who | What |
|------|-----|------|
| 1 | **Station Admin** | Create OAuth App in CloudShip |
| 2 | **Station Admin** | Configure Station with OAuth enabled |
| 3 | **Station Admin** | Invite team members to CloudShip org |
| 4 | **Team Member** | Connect MCP client → Browser login → Access agents |

### How It Works

1. **MCP client connects** to Station without authentication
2. **Station returns 401** with `WWW-Authenticate` header pointing to CloudShip
3. **Client discovers OAuth endpoints** via CloudShip's well-known URLs
4. **Client opens browser** to CloudShip login
5. **User authenticates** and approves access
6. **Client receives token** and reconnects to Station
7. **Station validates token** via CloudShip introspection
8. **MCP request proceeds** with authenticated user context

```
MCP Client                    Station                      CloudShip
    |                           |                             |
    |------ POST /mcp --------->|                             |
    |<----- 401 Unauthorized ---|                             |
    |       WWW-Authenticate:   |                             |
    |       Bearer resource_metadata=                         |
    |       "https://app.cloudshipai.com/.well-known/oauth-protected-resource"
    |                           |                             |
    |------- GET /.well-known/oauth-protected-resource ------>|
    |<------ {"authorization_servers": ["cloudshipai.com"]} --|
    |                           |                             |
    |------- GET /.well-known/oauth-authorization-server ---->|
    |<------ {endpoints, scopes, pkce_methods, ...} ----------|
    |                           |                             |
    |------- GET /oauth/authorize/?... ---------------------->|
    |<------ [Browser: Login + Consent] ----------------------|
    |<------ 302 callback?code=XXX ---------------------------|
    |                           |                             |
    |------- POST /oauth/token/ {code, code_verifier} ------->|
    |<------ {access_token, refresh_token, expires_in} -------|
    |                           |                             |
    |------ POST /mcp --------->|                             |
    |  Authorization: Bearer    |------ POST /oauth/introspect/
    |                           |<------ {active:true, user_id, org_id, email}
    |<----- MCP Response -------|                             |
```

### Enabling OAuth

#### Step 1: Create an OAuth Application in CloudShip

**Option A: Via Django Admin**
1. Log in to CloudShip admin at `/cshipai-admin/`
2. Go to **OAuth2 Provider > Applications**
3. Click **Add Application**
4. Fill in:
   - **Name**: e.g., "Station MCP Client"
   - **Client type**: Public
   - **Authorization grant type**: Authorization code
   - **Redirect URIs**: `http://localhost:8585/oauth/callback` (add your production URL too)
5. Save and note the **Client ID**

**Option B: Via CLI**
```bash
cd cloudshipai/backend
uv run python manage.py create_oauth_app \
  --name "Station MCP Client" \
  --user-email your@email.com \
  --client-type public \
  --redirect-uris "http://localhost:8585/oauth/callback"
```

#### Step 2: Configure Station

Add OAuth settings to your `config.yaml`:

```yaml
cloudship:
  enabled: true
  registration_key: "your-registration-key"
  name: "my-station"
  base_url: "https://app.cloudshipai.com"  # OAuth discovery base
  oauth:
    enabled: true
    client_id: "your-oauth-client-id"      # From Step 1
```

#### Step 3: Invite Team Members to Your Organization

1. Go to CloudShip dashboard
2. Navigate to **Settings > Team**
3. Click **Invite Member**
4. Enter their email address
5. They'll receive an invitation to join your organization

Only users who are members of your organization can authenticate and access your Station's agents.

#### Step 4: Start Station

```bash
stn serve --config config.yaml
# Note: OAuth is only enforced when local_mode is false
```

#### Step 5: Team Members Connect

When a team member configures their MCP client to connect to your Station:

1. **First connection** → Station returns 401 with OAuth discovery URL
2. **Browser opens** → CloudShip login page
3. **User logs in** → With their CloudShip account (must be in your org)
4. **Consent screen** → "Authorize Station MCP Client to access..."
5. **Token issued** → MCP client reconnects with Bearer token
6. **Access granted** → User can now call your Station's agents

### OAuth Configuration Reference

```yaml
cloudship:
  # Base URL for OAuth discovery (well-known endpoints)
  base_url: "https://app.cloudshipai.com"
  
  oauth:
    # Enable OAuth authentication for MCP endpoints
    enabled: true
    
    # OAuth client ID from CloudShip OAuth Apps
    client_id: "your-client-id"
    
    # Optional: Override OAuth endpoints (auto-derived from base_url)
    auth_url: "https://app.cloudshipai.com/oauth/authorize/"
    token_url: "https://app.cloudshipai.com/oauth/token/"
    introspect_url: "https://app.cloudshipai.com/oauth/introspect/"
    
    # Optional: Redirect URI for auth code flow
    redirect_uri: "http://localhost:8585/oauth/callback"
    
    # Optional: OAuth scopes to request
    scopes: "read stations"
```

### MCP Client Configuration

MCP clients automatically discover OAuth configuration from Station's 401 response. No special configuration needed - just point to your Station's MCP endpoint:

**Claude Desktop** (`claude_desktop_config.json`):
```json
{
  "mcpServers": {
    "my-station": {
      "url": "https://my-station.example.com:8587/mcp"
    }
  }
}
```

**Cursor** (`.mcp.json`):
```json
{
  "mcpServers": {
    "my-station": {
      "url": "https://my-station.example.com:8587/mcp"
    }
  }
}
```

> **Port Reference:**
> - **8585** - Station API (REST)
> - **8586** - Standard MCP Server
> - **8587** - Dynamic Agent MCP Server (OAuth-protected when enabled)

When the client first connects, it will receive a 401 and automatically:
1. Discover CloudShip OAuth endpoints
2. Open your browser for login
3. Complete the OAuth flow
4. Reconnect with the access token

### Local API Key Authentication

For local development or scripts, you can also use Station's local API keys (prefix `sk-`):

```bash
# Get API key from Station
curl http://localhost:8585/api/v1/users/me -H "Authorization: Bearer sk-your-local-api-key"

# Use with MCP
curl http://localhost:8586/mcp \
  -H "Authorization: Bearer sk-your-local-api-key" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"tools/list","id":1}'
```

### Token Validation

Station validates OAuth tokens by calling CloudShip's introspection endpoint. The response includes:

```json
{
  "active": true,
  "scope": "read stations",
  "client_id": "...",
  "token_type": "Bearer",
  "exp": 1765128225,
  "user_id": "uuid-of-cloudship-user",
  "email": "user@example.com",
  "org_id": "uuid-of-organization"
}
```

Station caches validated tokens for 5 minutes to reduce introspection calls.

### Security Notes

- **PKCE Required**: CloudShip enforces PKCE (Proof Key for Code Exchange) with S256 for all authorization code flows
- **Token Expiration**: Access tokens expire after 1 hour; use refresh tokens for long-lived sessions
- **Introspection**: Every MCP request validates the token (with caching)
- **Org Scoping**: The `org_id` in the token can be used to scope agent access

### Development Setup

For local development with CloudShip OAuth:

```yaml
cloudship:
  enabled: true
  registration_key: "your-dev-key"
  name: "dev-station"
  endpoint: "localhost:50051"           # Local Lighthouse
  base_url: "http://localhost:8000"     # Local Django
  oauth:
    enabled: true
    client_id: "your-dev-client-id"
    introspect_url: "http://localhost:8000/oauth/introspect/"
```

---

## Security

### TLS

All connections to CloudShip use TLS by default on port 443. The Lighthouse server uses a valid Let's Encrypt certificate managed by Fly.io, so no special certificate configuration is needed.

For local development with a local Lighthouse instance:
```yaml
cloudship:
  endpoint: "localhost:50051"
  use_tls: false  # Local Lighthouse runs without TLS
```

### Registration Key

- Treat registration keys like API keys
- Don't commit them to version control
- Use environment variables in production:

```yaml
cloudship:
  registration_key: ${CLOUDSHIP_REGISTRATION_KEY}
```

### Network

- Station initiates all connections (firewall-friendly)
- No inbound ports required
- All traffic is encrypted

## Environment Variables

| Variable | Description |
|----------|-------------|
| `CLOUDSHIP_REGISTRATION_KEY` | Registration key (alternative to config) |
| `CLOUDSHIP_ENDPOINT` | Lighthouse endpoint override |
| `CLOUDSHIP_STATION_NAME` | Station name override |

## Next Steps

- [Agent Development Guide](./agents/README.md) - Create custom agents
- [MCP Server Integration](./mcp-servers/README.md) - Add MCP tool servers
- [Telemetry Setup](./OTEL_SETUP.md) - Configure OpenTelemetry

## Support

- Documentation: [docs.cloudshipai.com](https://docs.cloudshipai.com)
- GitHub Issues: [github.com/cloudshipai/station](https://github.com/cloudshipai/station/issues)
- Discord: [discord.gg/cloudship](https://discord.gg/cloudship)
