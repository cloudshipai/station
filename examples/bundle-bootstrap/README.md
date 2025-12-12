# Bundle Bootstrap Example

Deploy a Station with pre-configured agents from a CloudShip bundle.

## Prerequisites

- Docker and Docker Compose
- CloudShip account with a registration key
- A bundle ID from CloudShip (create one at app.cloudshipai.com/webapp/bundles/)
- `station-server:latest` image built locally (`make rebuild-all` from station root)

## Quick Start

1. Copy the example env file:
   ```bash
   cp .env.example .env
   ```

2. Edit `.env` with your values:
   ```bash
   STN_BUNDLE_ID=your-bundle-uuid
   STN_CLOUDSHIP_KEY=your-registration-key
   OPENAI_API_KEY=your-openai-key
   ```

3. Start the station:
   ```bash
   docker compose up -d
   ```

4. Check logs:
   ```bash
   docker compose logs -f
   ```

## What Happens

1. Station starts with `station-server:latest` image (includes bundle agents)
2. Connects to CloudShip via the registration key
3. Registers with the specified station name
4. Loads agents from the bundle into the default environment
5. Exposes agents as MCP tools

## Ports

| Port | Service |
|------|---------|
| 8585 | API/UI (dev mode) |
| 8586 | MCP Server |
| 8587 | Dynamic Agent MCP |

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `STN_BUNDLE_ID` | Yes | Bundle UUID from CloudShip |
| `STN_CLOUDSHIP_KEY` | Yes | Registration key for authentication |
| `STN_CLOUDSHIP_NAME` | No | Station name (default: bootstrap) |
| `STN_CLOUDSHIP_ENDPOINT` | No | Lighthouse endpoint (default: lighthouse.cloudshipai.com:443) |
| `OPENAI_API_KEY` | Yes | OpenAI API key for agent execution |
| `STN_DEV_MODE` | No | Enable API access (default: true) |
| `STN_TELEMETRY_ENABLED` | No | Enable distributed tracing (default: true) |
| `STN_TELEMETRY_PROVIDER` | No | Telemetry backend: `cloudship`, `jaeger`, `otlp` (default: jaeger) |

## Telemetry & Tracing

To send execution traces to CloudShip (forwarded to Grafana Tempo), add these environment variables:

```yaml
environment:
  - STN_TELEMETRY_ENABLED=true
  - STN_TELEMETRY_PROVIDER=cloudship
```

This enables:
- Distributed tracing for all agent executions
- Trace visualization in Grafana Tempo
- Performance monitoring and debugging

### Trace Flow
```
Station → telemetry.cloudshipai.com → Grafana Tempo
```

### Query Traces via Tempo API
```bash
curl -s -u "$GRAFANA_CLOUD_USER:$GRAFANA_CLOUD_API_KEY" \
  "https://tempo-prod-26-prod-us-east-2.grafana.net/tempo/api/search?limit=20"
```

### Trace Attributes
Each trace includes:
- `agent.name` - Agent being executed
- `agent.id` - Agent database ID
- `run.uuid` - CloudShip run ID
- `cloudship.station_name` - Station name
- `execution.model` - AI model used (e.g., openai/gpt-4o-mini)
- `execution.duration_seconds` - Total execution time
- `execution.success` - Whether execution succeeded

## Verify Station is Online

1. Check CloudShip UI: https://app.cloudshipai.com/webapp/stations/
2. Your station should appear with the configured agents
3. Click "Connect" to interact with agents

## Troubleshooting

**Station not appearing online:**
- Ensure only one Lighthouse instance is running (check Fly.io)
- Verify registration key is valid and has available station slots

**No agents loaded:**
- Check that the bundle exists and has agents
- Verify `STN_BUNDLE_ID` matches a valid bundle UUID

**Connection errors:**
- Check `STN_CLOUDSHIP_ENDPOINT` is correct
- Ensure TLS is working (default endpoint uses port 443 with TLS)
