# Lab 9: Local E2E Lattice Test

Run a complete lattice setup locally with NATS in Docker and Station running natively.

## Prerequisites

- Docker and Docker Compose
- Station CLI (`stn`) built locally
- OpenAI API key (for agent execution)

## Quick Start

```bash
# 1. Start NATS with JetStream
docker compose up -d

# 2. Verify NATS is running
curl http://localhost:18222/varz | jq '.server_name'

# 3. Create .env from template
cp .env.example .env
# Edit .env and set your OPENAI_API_KEY

# 4. Set up workspace with lab agents
source .env
mkdir -p /tmp/e2e-lab/environments/default/agents
cp ../agents/*.prompt /tmp/e2e-lab/environments/default/agents/

# 5. Create station config
ENCRYPTION_KEY=$(openssl rand -base64 32)
cat > /tmp/e2e-lab-config.yaml << EOF
workspace: /tmp/e2e-lab
ai_provider: openai
ai_model: gpt-4o-mini
api_port: 18585
mcp_port: 18586
local_mode: true
encryption_key: "${ENCRYPTION_KEY}"

lattice:
  station_name: e2e-lab-station
  nats:
    url: "nats://${NATS_AUTH_TOKEN}@localhost:14222"
EOF

# 6. Start station
export OPENAI_API_KEY=your-key-here
stn serve --config /tmp/e2e-lab-config.yaml

# (In another terminal)

# 7. Test lattice connectivity
source .env
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" status
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" agents

# 8. Execute agents
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" \
  agent exec echo-agent "Hello from E2E test!"

stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" \
  agent exec math-agent "Calculate 25 * 17"

stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" \
  agent exec joke-agent "Tell me a programming joke"

# 9. Cleanup
docker compose down
# Stop station with Ctrl+C
rm -rf /tmp/e2e-lab /tmp/e2e-lab-config.yaml
```

## How It Works

```
+-------------------------------------------------------------------+
|                         Your Machine                               |
+-------------------------------------------------------------------+
|                                                                    |
|  +------------------+         +--------------------------------+   |
|  | NATS Container   |<------->| Station (native)               |   |
|  | :14222 (client)  |         | - Lattice client               |   |
|  | :18222 (monitor) |         | - Lab agents                   |   |
|  +------------------+         | - /tmp/e2e-lab workspace       |   |
|         ^                     +--------------------------------+   |
|         |                                                          |
|  +------+-------+                                                  |
|  | stn CLI      |                                                  |
|  | lattice      |                                                  |
|  | commands     |                                                  |
|  +--------------+                                                  |
|                                                                    |
+-------------------------------------------------------------------+
```

## Workspace Structure

The station expects agents in a specific directory structure:

```
/tmp/e2e-lab/
+-- environments/
    +-- default/
        +-- agents/
            +-- echo-agent.prompt
            +-- math-agent.prompt
            +-- joke-agent.prompt
            +-- time-agent.prompt
            +-- sysinfo-agent.prompt
```

## Lab Agents

| Agent | Purpose |
|-------|---------|
| `echo-agent` | Simple echo for connectivity testing |
| `math-agent` | Mathematical calculations |
| `joke-agent` | Programming humor |
| `time-agent` | Time and timezone info |
| `sysinfo-agent` | Station/lattice status |

## Files

| File | Purpose | Git Status |
|------|---------|------------|
| `.env.example` | Template with required variables | Committed |
| `.env` | Your actual secrets | **Gitignored** |
| `docker-compose.yml` | NATS container config | Committed |

## Port Mappings

| Port | Service | Description |
|------|---------|-------------|
| 14222 | NATS client | Station connects here |
| 18222 | NATS monitoring | Health checks and metrics |
| 18585 | Station API | REST API (optional) |
| 18586 | Station MCP | MCP server (optional) |

## Troubleshooting

### Station can't connect to NATS

```bash
# Check NATS is healthy
docker compose ps
curl http://localhost:18222/connz

# Verify auth token matches
source .env
echo $NATS_AUTH_TOKEN
```

### "No responders available" error

This usually means stale registrations. Clean up and restart:

```bash
source .env

# List current stations
nats --server "nats://${NATS_AUTH_TOKEN}@localhost:14222" kv ls lattice-stations

# Delete stale entries
nats --server "nats://${NATS_AUTH_TOKEN}@localhost:14222" kv rm lattice-stations <old-id> -f

# Restart station
```

### Agent not found in lattice

```bash
# Check station registered correctly
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" status

# Verify agents are loaded
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" agents

# Check station logs for sync errors
```

### Agent execution fails

```bash
# Check OpenAI API key
echo $OPENAI_API_KEY

# Verify agent prompt file exists
ls /tmp/e2e-lab/environments/default/agents/

# Check station logs for errors
```

## Expected Output

```
$ stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" agent exec echo-agent "Hello!"

[routing to e2e-lab-station]

Received your message.
Hello!
ECHO FROM E2E

Execution completed in 1.13s (via 18362231-4535-4138-a110-463918316931)
```
