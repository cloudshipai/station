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
  endpoint: "lighthouse.cloudshipai.com:50051"
  
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
| `endpoint` | No | `lighthouse.cloudshipai.com:50051` | Lighthouse gRPC endpoint |
| `use_tls` | No | `true` | Use TLS for connection |
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
Connected to CloudShip Lighthouse at lighthouse.cloudshipai.com:50051
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

- Check network connectivity to `lighthouse.cloudshipai.com:50051`
- Verify firewall allows outbound gRPC (port 50051)
- Try with `use_tls: false` for debugging

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

## Security

### TLS

All connections use TLS by default. The station validates the Lighthouse server certificate.

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
