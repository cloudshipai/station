# Lattice Lab Tutorial

A hands-on guide to deploying Station Lattice - a distributed agent mesh network.

## Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          STATION LATTICE MESH                                │
│                                                                              │
│   ┌───────────────────┐                                                      │
│   │  stn lattice CLI  │ ─────► Query/Execute agents across the mesh         │
│   └─────────┬─────────┘                                                      │
│             │                                                                │
│             ▼                                                                │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │                     NATS Message Bus                         │           │
│   │   (JetStream for KV store, pub/sub for agent invocation)     │           │
│   └─────────────────────────────────────────────────────────────┘           │
│             │                    │                    │                      │
│             ▼                    ▼                    ▼                      │
│   ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐            │
│   │ Orchestrator    │  │ Compute Station │  │ Utility Station │            │
│   │ Station         │  │                 │  │                 │            │
│   │ ─────────────── │  │ ─────────────── │  │ ─────────────── │            │
│   │ • echo-agent    │  │ • math-agent    │  │ • time-agent    │            │
│   │ • sysinfo-agent │  │                 │  │ • joke-agent    │            │
│   └─────────────────┘  └─────────────────┘  └─────────────────┘            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Mermaid Architecture Diagram

```mermaid
graph TB
    subgraph "Host Machine"
        CLI["stn lattice CLI"]
    end
    
    subgraph "NATS Cluster"
        NATS["NATS Server<br/>JetStream Enabled"]
        KV["KV: stations"]
        KV2["KV: work"]
    end
    
    subgraph "Orchestrator Station"
        O_STN["Station Process"]
        O_ECHO["echo-agent"]
        O_SYS["sysinfo-agent"]
    end
    
    subgraph "Compute Station"
        C_STN["Station Process"]
        C_MATH["math-agent"]
    end
    
    subgraph "Utility Station"
        U_STN["Station Process"]
        U_TIME["time-agent"]
        U_JOKE["joke-agent"]
    end
    
    CLI -->|"nats://..."| NATS
    O_STN -->|"heartbeat"| KV
    C_STN -->|"heartbeat"| KV
    U_STN -->|"heartbeat"| KV
    
    NATS --> O_STN
    NATS --> C_STN
    NATS --> U_STN
    
    O_STN --- O_ECHO
    O_STN --- O_SYS
    C_STN --- C_MATH
    U_STN --- U_TIME
    U_STN --- U_JOKE
```

## Lab Agents

The lab includes 5 test agents distributed across stations:

| Agent | Station | Purpose |
|-------|---------|---------|
| `echo-agent` | Orchestrator | Simple echo for connectivity testing |
| `sysinfo-agent` | Orchestrator | Reports station/lattice status |
| `math-agent` | Compute | Mathematical calculations |
| `time-agent` | Utility | Time and timezone queries |
| `joke-agent` | Utility | Programming humor |

Agent prompts are in `examples/lattice-lab/agents/`.

---

## Lab 1: Quick Start (Local Docker)

The fastest way to experience lattice - runs entirely on your machine with Docker.

### Prerequisites

- Docker and Docker Compose
- Station CLI (`stn`)

### Steps

```bash
cd examples/lattice-lab/e2e-local

# 1. Create your .env file
cp .env.example .env
# Edit .env with your AI provider credentials

# 2. Start NATS + Station
docker compose up -d

# 3. Wait for startup
sleep 10

# 4. List agents
source .env
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" agents

# 5. Execute an agent
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" \
  agent exec echo-agent "Hello from lattice!"

# 6. Cleanup
docker compose down
```

### Architecture

```
┌────────────────────────────────────────────────────────────┐
│                    Your Machine                             │
│                                                             │
│  ┌──────────────────┐       ┌──────────────────────────┐   │
│  │ NATS Container   │       │ Station Container        │   │
│  │ :14222 (client)  │◄──────│ - Lattice client         │   │
│  │ :18222 (monitor) │       │ - Your agents            │   │
│  │ JetStream ON     │       │ - MCP servers            │   │
│  └──────────────────┘       └──────────────────────────┘   │
│           ▲                                                 │
│           │                                                 │
│  ┌────────┴────────┐                                        │
│  │   stn CLI       │                                        │
│  │   lattice cmd   │                                        │
│  └─────────────────┘                                        │
└────────────────────────────────────────────────────────────┘
```

---

## Lab 2: Embedded NATS Orchestrator

Run a station as the lattice orchestrator with embedded NATS server.

### Initialize with Lattice

```bash
# Create a new station workspace as orchestrator
mkdir /tmp/lab2-orchestrator && cd /tmp/lab2-orchestrator

stn init --config . \
  --provider openai \
  --model gpt-4o-mini \
  --lattice-orchestrator \
  --lattice-port 4222 \
  --lattice-token "lab2-secret-token" \
  --lattice-name "orchestrator" \
  --yes
```

### Copy Lab Agents

```bash
cp examples/lattice-lab/agents/echo-agent.prompt environments/default/agents/
cp examples/lattice-lab/agents/sysinfo-agent.prompt environments/default/agents/
```

### Start Orchestrator

```bash
stn serve --config config.yaml --local
```

### Verify

```bash
# Check NATS is running
curl http://localhost:8222/varz | jq '.server_name'

# Check lattice status
stn lattice --nats "nats://lab2-secret-token@localhost:4222" status

# List agents
stn lattice --nats "nats://lab2-secret-token@localhost:4222" agents
```

Expected output:
```
Agents in Lattice (2 total)
============================================
AGENT            STATION        LOCAL
--------------------------------------------
echo-agent       orchestrator   
sysinfo-agent    orchestrator   
```

---

## Lab 3: Add Member Station

Add a second station to the lattice mesh.

### Initialize Member

```bash
# In a new terminal
mkdir /tmp/lab3-member && cd /tmp/lab3-member

stn init --config . \
  --provider openai \
  --model gpt-4o-mini \
  --lattice-url "nats://lab2-secret-token@localhost:4222" \
  --lattice-name "compute-node" \
  --yes
```

### Copy Different Agents

```bash
cp examples/lattice-lab/agents/math-agent.prompt environments/default/agents/
```

### Start Member (Different Ports)

```bash
stn serve --config config.yaml --local --mcp-port 9586 --api-port 9585
```

### Verify Multi-Station

```bash
stn lattice --nats "nats://lab2-secret-token@localhost:4222" agents
```

Expected output:
```
Agents in Lattice (3 total)
============================================
AGENT            STATION        LOCAL
--------------------------------------------
echo-agent       orchestrator   
sysinfo-agent    orchestrator   
math-agent       compute-node   
```

### Execute Cross-Station

```bash
# Execute agent on orchestrator
stn lattice --nats "nats://lab2-secret-token@localhost:4222" \
  agent exec echo-agent "Hello from the mesh!"

# Execute agent on compute-node  
stn lattice --nats "nats://lab2-secret-token@localhost:4222" \
  agent exec math-agent "What is 15% of 340?"
```

---

## Lab 4: NATS Authentication

Secure the lattice with authentication.

### Token Authentication

```yaml
# In orchestrator config.yaml
lattice:
  station_name: "orchestrator"
  orchestrator:
    embedded_nats:
      port: 4222
      http_port: 8222
      auth:
        enabled: true
        token: "my-secure-lattice-token"
```

Members connect with token in URL:
```bash
stn init --lattice-url "nats://my-secure-lattice-token@orchestrator:4222"
```

### User/Password Authentication

For multiple users with different permissions:

```yaml
lattice:
  orchestrator:
    embedded_nats:
      auth:
        enabled: true
        users:
          - user: admin
            password: admin-secret
          - user: reader
            password: reader-secret
```

Members specify credentials:
```bash
stn init --lattice-url "nats://admin:admin-secret@orchestrator:4222"
```

---

## Lab 5: Async Work Distribution

Fire-and-forget agent execution with work tracking.

### Work Flow Diagram

```mermaid
sequenceDiagram
    participant CLI as stn CLI
    participant NATS as NATS JetStream
    participant Station as Station (agent host)
    
    CLI->>NATS: work assign <agent> "task"
    NATS-->>CLI: work_id: abc123
    Note over CLI: Returns immediately
    
    NATS->>Station: Deliver work item
    Station->>Station: Execute agent
    Station->>NATS: Store result in KV
    
    CLI->>NATS: work check abc123
    NATS-->>CLI: status: completed, result: "..."
```

### Commands

```bash
# Assign work (returns immediately)
WORK_ID=$(stn lattice work assign math-agent "Calculate 2^10" | grep "Work ID:" | awk '{print $3}')
echo "Started: $WORK_ID"

# Check status (non-blocking)
stn lattice work check $WORK_ID

# Wait for completion (blocking)
stn lattice work await $WORK_ID
```

### Parallel Execution

```bash
# Start multiple agents in parallel
W1=$(stn lattice work assign echo-agent "ping 1" | grep "Work ID:" | awk '{print $3}')
W2=$(stn lattice work assign math-agent "100 / 4" | grep "Work ID:" | awk '{print $3}')
W3=$(stn lattice work assign time-agent "current time" | grep "Work ID:" | awk '{print $3}')

# Gather results
echo "Echo: $(stn lattice work await $W1)"
echo "Math: $(stn lattice work await $W2)"
echo "Time: $(stn lattice work await $W3)"
```

---

## Lab 6: Multi-Station Routing

Test automatic routing to stations based on agent availability.

### Setup

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Lattice Mesh                                 │
│                                                                      │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐     │
│  │ Orchestrator    │  │ Compute         │  │ Utility         │     │
│  │ :4222           │  │ :9586           │  │ :9687           │     │
│  │                 │  │                 │  │                 │     │
│  │ • echo-agent    │  │ • math-agent    │  │ • time-agent    │     │
│  │ • sysinfo-agent │  │                 │  │ • joke-agent    │     │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘     │
│           │                    │                    │               │
│           └────────────────────┼────────────────────┘               │
│                                │                                     │
│                         NATS Message Bus                            │
└─────────────────────────────────────────────────────────────────────┘
```

### Start Third Station

```bash
mkdir /tmp/lab6-utility && cd /tmp/lab6-utility

stn init --config . \
  --provider openai --model gpt-4o-mini \
  --lattice-url "nats://lab2-secret-token@localhost:4222" \
  --lattice-name "utility-node" \
  --yes

cp examples/lattice-lab/agents/time-agent.prompt environments/default/agents/
cp examples/lattice-lab/agents/joke-agent.prompt environments/default/agents/

stn serve --config config.yaml --local --mcp-port 9687 --api-port 9688
```

### Verify All Agents

```bash
stn lattice --nats "nats://lab2-secret-token@localhost:4222" agents --discover
```

Expected:
```
Agents in Lattice (5 total)
============================================
AGENT            STATION         CAPABILITIES
--------------------------------------------
echo-agent       orchestrator    test, lattice-lab
sysinfo-agent    orchestrator    ops, monitoring
math-agent       compute-node    utility, compute
time-agent       utility-node    utility
joke-agent       utility-node    fun
```

### Execute Across Mesh

```bash
# Each request routes to the correct station
stn lattice agent exec echo-agent "test"        # → orchestrator
stn lattice agent exec math-agent "5 * 5"       # → compute-node  
stn lattice agent exec joke-agent "programming" # → utility-node
```

---

## Lab 7: Dashboard & Observability

Monitor lattice health and agent activity.

### NATS Monitoring

```bash
# Server info
curl http://localhost:8222/varz | jq '{server_name, uptime, connections}'

# Connected clients
curl http://localhost:8222/connz | jq '.connections[] | {name, ip, subscriptions}'

# JetStream status
curl http://localhost:8222/jsz | jq '{streams, consumers, messages}'
```

### Lattice Status Dashboard

```bash
# Full status
stn lattice status --detailed

# Watch mode (updates every 5s)
watch -n 5 'stn lattice agents'
```

### Station Health

```bash
# Per-station health
curl http://localhost:8587/health

# Agent execution metrics
stn lattice station info <station-name>
```

---

## Lab 8: Centralized NATS Deployment

Production-like setup with dedicated NATS server.

### Architecture

```mermaid
graph TB
    subgraph "NATS Server VM (192.168.56.9)"
        NATS["NATS Server<br/>JetStream + Auth"]
    end
    
    subgraph "Orchestrator VM (192.168.56.10)"
        ORch["Station<br/>lattice client mode"]
    end
    
    subgraph "Member VM (192.168.56.11)"
        Mem["Station<br/>lattice client mode"]
    end
    
    NATS ---|":4222"| ORch
    NATS ---|":4222"| Mem
```

### Start VMs

```bash
cd examples/lattice-lab/vagrant
vagrant up nats orchestrator member1
```

### Deploy NATS Server

```bash
cd examples/lattice-lab/ansible-nats
ansible-playbook -i inventory.ini playbook.yml
```

### Deploy Stations

```bash
# Orchestrator (connects to external NATS)
cd examples/lattice-lab/ansible-orchestrator-centralized
ansible-playbook -i inventory.ini playbook.yml

# Member
cd examples/lattice-lab/ansible-member
ansible-playbook -i inventory.ini playbook.yml
```

### Test

```bash
stn lattice --nats nats://192.168.56.9:4222 agents
stn lattice --nats nats://192.168.56.9:4222 agent exec echo-agent "Hello VM mesh!"
```

---

## Lab 9: E2E Local Test (Production Mirror)

Complete E2E test using Docker containers that mirrors production deployment.

See `examples/lattice-lab/e2e-local/README.md` for detailed instructions.

### Key Points

1. **Production-like**: Uses Docker + environment variables (same as `stn deploy --target ansible`)
2. **Repeatable**: `.env.example` shows required variables
3. **No secrets in git**: `.env` is gitignored

```bash
cd examples/lattice-lab/e2e-local
cp .env.example .env
# Edit .env with your credentials
docker compose up -d
stn lattice --nats "nats://${NATS_AUTH_TOKEN}@localhost:14222" agents
```

---

## Configuration Reference

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `STN_LATTICE_NATS_URL` | NATS server URL | - |
| `STN_LATTICE_STATION_NAME` | Station name in mesh | hostname |
| `STN_LATTICE_STATION_ID` | Custom station UUID | auto-generated |
| `STN_LATTICE_AUTH_TOKEN` | Embedded NATS auth token | - |
| `STN_LATTICE_AUTH_ENABLED` | Enable embedded NATS auth | false |
| `STN_LATTICE_NATS_PORT` | Embedded NATS port | 4222 |
| `STN_LATTICE_NATS_HTTP_PORT` | Embedded NATS monitoring | 8222 |
| `STN_LATTICE_PRESENCE_TTL_SEC` | Presence heartbeat TTL | 30 |

### Config File Structure

```yaml
lattice:
  station_name: "my-station"
  station_id: ""  # Auto-generated if empty
  
  nats:
    url: "nats://localhost:4222"
    auth:
      token: ""
      user: ""
      password: ""
      nkey_seed: ""
      creds_file: ""
    tls:
      enabled: false
      cert_file: ""
      key_file: ""
      ca_file: ""
  
  orchestrator:
    embedded_nats:
      port: 4222
      http_port: 8222
      store_dir: ""
      auth:
        enabled: false
        token: ""
        users: []
    registry:
      presence_ttl_sec: 30
    routing:
      timeout_sec: 60

lattice_orchestration: false  # Enable orchestrator mode
lattice_url: ""               # CLI flag alternative
```

### CLI Flags

```bash
# stn init
--lattice-url           NATS URL to join
--lattice-name          Station name
--lattice-orchestrator  Enable orchestrator mode
--lattice-port          Embedded NATS port
--lattice-token         Auth token

# stn serve
--orchestration        Enable orchestrator mode
--lattice              NATS URL to join

# stn lattice
--nats                 NATS connection URL
```

---

## Troubleshooting

### "No responders available"

JetStream not enabled on NATS. Add `-js` flag or enable in config.

### "Connection refused"

1. Check NATS is running: `curl http://localhost:8222/healthz`
2. Verify port is open: `nc -zv localhost 4222`
3. Check auth token matches

### "Station not appearing"

1. Check logs for "Lattice presence heartbeat started"
2. Verify station name is unique
3. Wait 30 seconds for presence TTL

### Agents not loading

1. Check environment directory: `ls environments/default/agents/`
2. Verify agent syntax: `stn agent list`
3. Check logs for "Loading N agents"

---

## Files Reference

```
examples/lattice-lab/
├── README.md                    # Quick reference
├── agents/                      # Lab agent prompts
│   ├── echo-agent.prompt
│   ├── math-agent.prompt
│   ├── time-agent.prompt
│   ├── joke-agent.prompt
│   └── sysinfo-agent.prompt
├── e2e-local/                   # Lab 9: Docker E2E
│   ├── .env.example
│   ├── .gitignore
│   ├── docker-compose.yml
│   └── README.md
├── vagrant/                     # VM definitions
│   └── Vagrantfile
├── ansible-nats/                # Lab 8: Centralized NATS
├── ansible-orchestrator/        # Embedded NATS mode
├── ansible-orchestrator-centralized/  # External NATS mode
└── ansible-member/              # Member station
```
