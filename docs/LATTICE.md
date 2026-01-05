# Station Lattice Architecture

Station Lattice is a NATS-based mesh network that enables multiple Station instances to discover each other, share agent/workflow manifests, and invoke agents remotely.

## Quick Start

```bash
# Terminal 1: Start orchestrator (embedded NATS hub)
stn serve --orchestration

# Terminal 2: Start member station
stn serve --lattice nats://localhost:4222

# Terminal 3: Query the lattice
stn lattice status
stn lattice agents
stn lattice workflows
stn lattice agent exec k8s-admin "List pods"
stn lattice workflow run deploy-app
```

## Operating Modes

| Mode | Command | Description |
|------|---------|-------------|
| Standalone | `stn serve` | Default. No lattice. Current behavior unchanged. |
| Orchestrator | `stn serve --orchestration` | Runs embedded NATS hub on port 4222. |
| Member | `stn serve --lattice <url>` | Connects to an orchestrator. |

## Architecture

```
                    ┌───────────────────────────────────┐
                    │     Orchestrator Station          │
                    │  ┌─────────────────────────────┐  │
                    │  │      Embedded NATS Hub      │  │
                    │  │  - JetStream (persistence)  │  │
                    │  │  - KV (registry, state)     │  │
                    │  │  - Port: 4222               │  │
                    │  └─────────────────────────────┘  │
                    │                                   │
                    │  ┌─────────────────────────────┐  │
                    │  │   Registry & Router         │  │
                    │  │  - Station discovery        │  │
                    │  │  - Agent index              │  │
                    │  │  - Workflow index           │  │
                    │  └─────────────────────────────┘  │
                    └───────────────────────────────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
              ▼                     ▼                     ▼
    ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
    │   Station A     │   │   Station B     │   │   Station C     │
    │   (laptop)      │   │   (k8s-server)  │   │   (aws-server)  │
    │                 │   │                 │   │                 │
    │ Agents:         │   │ Agents:         │   │ Agents:         │
    │  - coder        │   │  - k8s-admin    │   │  - aws-finops   │
    │  - researcher   │   │  - k8s-monitor  │   │  - cost-analyst │
    │                 │   │                 │   │                 │
    │ Workflows:      │   │ Workflows:      │   │ Workflows:      │
    │  - code-review  │   │  - deploy-app   │   │  - cost-report  │
    └─────────────────┘   └─────────────────┘   └─────────────────┘
```

## Components

### LatticeClient (`internal/lattice/client.go`)

NATS connection wrapper with:
- Multiple auth methods (user/pass, token, NKey, creds file)
- TLS support
- Auto-reconnect
- Station ID generation

### EmbeddedServer (`internal/lattice/embedded.go`)

Embedded NATS server for orchestrator mode:
- JetStream enabled with file storage
- HTTP monitoring endpoint (default: 8222)
- Configurable ports and storage

### Registry (`internal/lattice/registry.go`)

KV-based station manifest storage:
- `lattice-stations` bucket: Station manifests
- `lattice-agents` bucket: Agent index
- Watch support for real-time updates

### Presence (`internal/lattice/presence.go`)

Heartbeat and discovery:
- Broadcasts presence every 10 seconds
- Subscribes to announce/goodbye messages
- Auto-registers discovered stations

### ManifestCollector (`internal/lattice/discovery.go`)

Collects local agents and workflows:
- Queries SQLite database
- Extracts capabilities from agent metadata
- Builds complete station manifest

### AgentRouter (`internal/lattice/router.go`)

Routing logic:
- Find agent by name or capability
- Find workflow by ID
- Prefer local agents when available

### Invoker (`internal/lattice/invoker.go`)

Remote execution:
- Handles `lattice.station.{id}.agent.invoke` requests
- Handles `lattice.station.{id}.workflow.run` requests
- NATS request-reply pattern

## NATS Subjects

```
# Station lifecycle
lattice.presence.announce.{station_id}    # Station online
lattice.presence.goodbye.{station_id}     # Station offline
lattice.presence.heartbeat.{station_id}   # Periodic heartbeat

# Agent invocation
lattice.station.{station_id}.agent.invoke
  Request:  { agent_name, agent_id, task, context }
  Response: { status, result, error, duration_ms, tool_calls }

# Workflow invocation
lattice.station.{station_id}.workflow.run
  Request:  { workflow_id, input }
  Response: { status, run_id, result, error }
```

## CLI Commands

### Status
```bash
stn lattice status
```
Shows lattice mode, connection status, and discovered stations.

### Agents
```bash
stn lattice agents                    # List all agents
stn lattice agent exec <name> <task>  # Execute agent
stn lattice agent exec k8s-admin "List pods" --station station-k8s
```

### Workflows
```bash
stn lattice workflows                 # List all workflows
stn lattice workflow run <id>         # Run workflow
stn lattice workflow run deploy-app --station station-k8s
```

## Configuration

### Config File (`config.yaml`)

```yaml
lattice:
  station_id: ""              # Auto-generated UUID if empty
  station_name: "My Station"
  
  nats:
    url: "nats://orchestrator:4222"
    
    auth:
      user: ""
      password: ""
      token: ""
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
      store_dir: ""           # Defaults to ~/.local/share/station/lattice/nats
```

### Environment Variables

```bash
STN_LATTICE_STATION_ID=my-station-id
STN_LATTICE_STATION_NAME="My Station"
STN_LATTICE_NATS_URL=nats://orchestrator:4222
STN_LATTICE_NATS_USER=myuser
STN_LATTICE_NATS_PASSWORD=mypassword
STN_LATTICE_NATS_TOKEN=mytoken
STN_LATTICE_NATS_NKEY_SEED=SUAM...
STN_LATTICE_NATS_CREDS_FILE=/path/to/creds
STN_LATTICE_NATS_TLS_ENABLED=true
STN_LATTICE_ORCHESTRATOR_PORT=4222
STN_LATTICE_ORCHESTRATOR_HTTP_PORT=8222
```

## Remote Agent Invocation Flow

```
Station A (caller)              Orchestrator              Station B (target)
     │                              │                           │
     │ stn lattice agent exec       │                           │
     │ k8s-admin "get pods"         │                           │
     │                              │                           │
     │  1. Connect to lattice       │                           │
     │─────────────────────────────>│                           │
     │                              │                           │
     │  2. Query registry for       │                           │
     │     agent "k8s-admin"        │                           │
     │─────────────────────────────>│                           │
     │                              │                           │
     │  3. Found: station-k8s       │                           │
     │<─────────────────────────────│                           │
     │                              │                           │
     │  4. NATS Request:            │                           │
     │  lattice.station.station-k8s.agent.invoke                │
     │──────────────────────────────────────────────────────────>
     │                              │                           │
     │                              │      5. Execute locally   │
     │                              │         via AgentEngine   │
     │                              │                           │
     │  6. NATS Reply: result       │                           │
     │<──────────────────────────────────────────────────────────
     │                              │                           │
     │  7. Display result           │                           │
```

## Testing

```bash
# Unit tests
go test ./internal/lattice/... -v

# Build verification
go build ./...

# Manual E2E test
# Terminal 1:
stn serve --orchestration

# Terminal 2:
stn serve --lattice nats://localhost:4222

# Terminal 3:
stn lattice status
stn lattice agents
```

## Troubleshooting

### "Not connected to lattice"
Start the station with `--orchestration` or `--lattice <url>`.

### "Registry not initialized"
The orchestrator's NATS server may not be running. Check with:
```bash
curl http://localhost:8222/varz
```

### "Agent not found in lattice"
1. Verify the target station is connected: `stn lattice status`
2. Check if agents are published: `stn lattice agents`
3. Ensure the station published its manifest on connect

### Connection refused
1. Check orchestrator is running
2. Verify NATS port (default 4222) is accessible
3. Check firewall rules

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Core infrastructure (client, embedded, registry, presence) | ✅ Complete |
| 2 | Agent discovery and remote invocation | ✅ Complete |
| 3 | Workflow discovery and remote invocation | ✅ Complete |
| 4 | Task groups and polish | 🔄 Planned |

## Files

```
internal/lattice/
├── client.go       # NATS client with auth/TLS
├── client_test.go  # Client tests
├── embedded.go     # Embedded NATS server
├── embedded_test.go
├── registry.go     # KV-based station registry
├── presence.go     # Heartbeat and discovery
├── discovery.go    # Agent/workflow collection from DB
├── router.go       # Agent/workflow routing
└── invoker.go      # Remote invocation handler

cmd/main/
├── main.go              # --orchestration, --lattice flags
└── lattice_commands.go  # stn lattice subcommands
```
