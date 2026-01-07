# Station Lattice Architecture

Station Lattice is a NATS-based mesh network that enables multiple Station instances to discover each other, share agent/workflow manifests, and invoke agents remotely.

## Table of Contents

- [Why Station Lattice?](#why-station-lattice)
- [Quick Start](#quick-start)
- [Demo Lab](#demo-lab)
- [Lattice Lab Tutorial](#lattice-lab-tutorial) ← Real VM deployment guide
- [Operating Modes](#operating-modes)
- [Architecture](#architecture)
- [Monitoring & Dashboards](#monitoring--dashboards)
- [Components](#components)
- [API Reference](#api-reference)
- [NATS Subjects](#nats-subjects)
- [CLI Commands](#cli-commands)
- [Configuration](#configuration)
- [Remote Invocation Flow](#remote-agent-invocation-flow)
- [Testing](#testing)
- [Troubleshooting](#troubleshooting)
- [Implementation Status](#implementation-status)
- [Next Steps (Phase 4)](#next-steps-phase-4)

## Why Station Lattice?

### The Problem

In traditional setups, AI agents run in isolation. A security team has their vulnerability scanners, SRE has their Kubernetes monitors, and DevOps has their deployment tools - all separate, unable to collaborate.

When you need a comprehensive infrastructure review, you manually:
1. Run the security scanner
2. Copy results somewhere
3. Run the K8s health check
4. Copy those results
5. Run the network audit
6. Manually synthesize everything

This is slow, error-prone, and doesn't scale.

### The Solution: Distributed Agent Orchestration

Station Lattice creates a **mesh network of AI agents** that can:

| Capability | Benefit |
|------------|---------|
| **Automatic Discovery** | Agents across stations find each other automatically |
| **Parallel Execution** | Orchestrator delegates to multiple agents simultaneously |
| **Cross-Team Collaboration** | Security, SRE, and DevOps agents work together on complex tasks |
| **Unified Interface** | Single command triggers work across your entire infrastructure |
| **Distributed Run Tracking** | Full observability of parent-child task relationships |

### Real-World Example

Instead of manually coordinating tools, you run:

```bash
stn lattice run "Perform a comprehensive security and health assessment"
```

The Coordinator agent automatically:
1. Discovers all available agents (K8sHealthChecker, VulnScanner, NetworkAudit, etc.)
2. Decomposes the task into subtasks
3. Assigns work to specialized agents **in parallel**
4. Collects and synthesizes results into a coherent report

**Result**: What took 30+ minutes of manual work now takes seconds.

## Quick Start

```bash
# Terminal 1: Start orchestrator (embedded NATS hub)
stn serve --orchestration

# Terminal 2: Start member station
stn serve --lattice nats://localhost:4222

# Terminal 3: Query the lattice (use --nats for CLI-only access)
stn lattice --nats nats://localhost:4222 status
stn lattice --nats nats://localhost:4222 agents
stn lattice --nats nats://localhost:4222 agent exec k8s-admin "List pods"
stn lattice --nats nats://localhost:4222 run "Analyze my infrastructure"
```

## Demo Lab

We provide ready-to-run demo scripts that set up a 3-station lattice with specialized agents.

### Demo Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    EMBEDDED NATS (port 4222)                     │
│                    JetStream KV: lattice-work                    │
└─────────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│  Orchestrator │     │  SRE Station  │     │   Security    │
│   Station     │     │               │     │   Station     │
│               │     │               │     │               │
│ Agents:       │     │ Agents:       │     │ Agents:       │
│ - Coordinator │     │ - K8sHealth   │     │ - VulnScanner │
│               │     │ - LogAnalyzer │     │ - CVELookup   │
│               │     │ - Deployer    │     │ - NetworkAudit│
│ Port: 8585    │     │ Port: 8586    │     │ Port: 8587    │
└───────────────┘     └───────────────┘     └───────────────┘
```

### Option 1: Automated Demo (Single Terminal)

```bash
cd docs/demos
./demo-auto.sh
```

This starts all 3 stations, runs demo commands, and shows results.

### Option 2: Multi-Terminal Demo (Recommended)

For the full interactive experience:

```
┌─────────────────────────────┬─────────────────────────────┐
│  Terminal 1: Orchestrator   │  Terminal 2: SRE Station    │
│  ./01-start-orchestrator.sh │  ./02-start-sre-station.sh  │
├─────────────────────────────┼─────────────────────────────┤
│  Terminal 3: Security       │  Terminal 4: User CLI       │
│  ./03-start-security.sh     │  ./04-run-commands.sh       │
└─────────────────────────────┴─────────────────────────────┘
```

### Demo Commands

Once stations are running, try these commands:

```bash
# Check lattice connectivity
stn lattice --nats nats://localhost:4222 status

# Discover all agents across the mesh
stn lattice --nats nats://localhost:4222 agents --discover

# Execute a specific agent directly
stn lattice --nats nats://localhost:4222 agent exec K8sHealthChecker "Check pod health"

# Assign async work
stn lattice --nats nats://localhost:4222 work assign VulnScanner "Scan /app"
stn lattice --nats nats://localhost:4222 work await <work_id>

# THE MAIN EVENT: Distributed orchestration
stn lattice --nats nats://localhost:4222 run "Analyze security and health of the infrastructure"

# Watch real-time work in TUI dashboard
stn lattice --nats nats://localhost:4222 dashboard
```

### Cleanup

```bash
./docs/demos/cleanup.sh
```

## Lattice Lab Tutorial

For a complete hands-on tutorial deploying Station Lattice across real VMs using Vagrant and Ansible, 
see **[LATTICE_LAB.md](./LATTICE_LAB.md)**.

The tutorial covers:
- Setting up VMs with Vagrant (libvirt provider)
- Deploying orchestrator with embedded NATS
- Deploying member stations with agent bundles
- Testing agent discovery and remote execution
- Configuration gotchas and troubleshooting

Example files are available in `examples/lattice-lab/`.

### Demo Scripts Reference

| Script | Purpose |
|--------|---------|
| `01-start-orchestrator.sh` | Starts orchestrator with embedded NATS on port 4222 |
| `02-start-sre-station.sh` | Starts SRE station with K8s/Log/Deploy agents |
| `03-start-security-station.sh` | Starts Security station with Vuln/CVE/Network agents |
| `04-run-commands.sh` | Interactive walkthrough of all lattice commands |
| `05-dashboard.sh` | Quick launcher for the TUI dashboard |
| `demo-auto.sh` | Automated demo (starts everything, runs commands) |
| `cleanup.sh` | Stops all stations and removes temp files |

## Operating Modes

| Mode | Command | Description |
|------|---------|-------------|
| Standalone | `stn serve` | Default. No lattice. Current behavior unchanged. |
| Orchestrator | `stn serve --orchestration` | Runs embedded NATS hub on port 4222. |
| Member | `stn serve --lattice <url>` | Connects to an orchestrator. |

## Monitoring & Dashboards

Station Lattice provides multiple ways to observe what's happening across your agent mesh.

### 1. TUI Dashboard (Real-Time Work Monitoring)

The built-in terminal dashboard shows live work status across all stations:

```bash
stn lattice --nats nats://localhost:4222 dashboard
```

Features:
- Active work items with status
- Recently completed work
- Station health indicators
- Press `q` to exit

### 2. NATS HTTP Monitoring

The embedded NATS server exposes monitoring endpoints on port 8222:

| Endpoint | Description |
|----------|-------------|
| `http://localhost:8222/varz` | Server stats (connections, memory, messages) |
| `http://localhost:8222/connz` | Active connections |
| `http://localhost:8222/jsz` | JetStream statistics |
| `http://localhost:8222/subsz` | Subscription details |

Example:
```bash
# Server overview
curl http://localhost:8222/varz | jq .

# Active connections
curl http://localhost:8222/connz | jq '.connections[] | {name, ip, subscriptions}'

# JetStream streams (includes lattice-work KV)
curl http://localhost:8222/jsz?streams=true | jq .
```

### 3. OpenTelemetry Tracing (Jaeger)

For distributed tracing across stations:

```bash
# Start with telemetry enabled
stn serve --orchestration --enable-telemetry --otel-endpoint http://localhost:4318

# View traces in Jaeger UI
open http://localhost:16686
```

Traces include:
- Agent invocations with parent-child relationships
- Work assignment and completion spans
- Cross-station request flows

### 4. Station Logs

Each station logs lattice activity:

```bash
# Watch all station logs
tail -f /tmp/orchestrator.log /tmp/sre.log /tmp/security.log

# Filter for lattice events
grep -E "(lattice|agent|work)" /tmp/sre.log
```

Key log patterns:
- `✅ Connected to lattice NATS` - Station joined mesh
- `Station registered with N agents` - Agents published
- `[invoker] Listening for agent invocations` - Ready for work
- `[presence] Station joined:` - New station discovered

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

### Component Interaction

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Client     │────▶│   Registry   │────▶│   Router     │
│  (NATS conn) │     │  (KV store)  │     │ (find agent) │
└──────────────┘     └──────────────┘     └──────────────┘
       │                    ▲                    │
       │                    │                    │
       ▼                    │                    ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Presence    │────▶│  Discovery   │     │   Invoker    │
│ (heartbeat)  │     │ (collect DB) │     │(remote exec) │
└──────────────┘     └──────────────┘     └──────────────┘
```

## Components

### LatticeClient (`internal/lattice/client.go`)

NATS connection wrapper with:
- Multiple auth methods (user/pass, token, NKey, creds file)
- TLS support
- Auto-reconnect with configurable backoff
- Station ID generation (UUID if not provided)

```go
// Create and connect
client, _ := lattice.NewClient(cfg.Lattice)
client.Connect()
defer client.Close()

// Basic operations
client.Publish(subject, data)
client.Subscribe(subject, handler)
client.Request(subject, data, timeout)

// Getters
client.StationID()    // UUID or configured ID
client.StationName()  // Human-readable name
client.IsConnected()  // Connection status
client.Conn()         // Raw NATS connection
client.JetStream()    // JetStream context
```

### EmbeddedServer (`internal/lattice/embedded.go`)

Embedded NATS server for orchestrator mode:
- JetStream enabled with file storage
- HTTP monitoring endpoint (default: 8222)
- Configurable ports and storage location
- Auto-creates data directory

```go
server := lattice.NewEmbeddedServer(cfg.Lattice.Orchestrator.EmbeddedNATS)
server.Start()
defer server.Shutdown()

server.ClientURL()      // "nats://127.0.0.1:4222"
server.MonitoringURL()  // "http://127.0.0.1:8222"
server.IsRunning()      // true/false
```

### Registry (`internal/lattice/registry.go`)

KV-based station manifest storage:
- `lattice-stations` bucket: Station manifests (JSON)
- `lattice-agents` bucket: Agent index for fast lookup
- Watch support for real-time updates
- Thread-safe with RWMutex

```go
registry := lattice.NewRegistry(client)
registry.Initialize(ctx)

// Station operations
registry.RegisterStation(ctx, manifest)
registry.UnregisterStation(ctx, stationID)
registry.GetStation(ctx, stationID)
registry.ListStations(ctx)
registry.UpdateStationStatus(ctx, stationID, status)

// Agent operations
registry.FindAgentsByCapability(ctx, capability)

// Real-time updates
ch, _ := registry.WatchStations(ctx)
for manifest := range ch {
    // Handle station update
}
```

### Presence (`internal/lattice/presence.go`)

Heartbeat and discovery:
- Broadcasts presence every 10 seconds (configurable)
- Subscribes to announce/goodbye/heartbeat messages
- Auto-registers discovered stations in registry

```go
presence := lattice.NewPresence(client, registry, manifest, 10)
presence.Start(ctx)
defer presence.Stop()

// Update manifest (triggers re-announce)
presence.UpdateManifest(newManifest)
```

### ManifestCollector (`internal/lattice/discovery.go`)

Collects local agents and workflows from SQLite database:
- Queries agents table via sqlc-generated queries
- Queries workflows table for active/enabled workflows
- Extracts capabilities from agent metadata (app, app_subtype)

```go
collector := lattice.NewManifestCollector(db)

// Collect all
manifest, _ := collector.CollectFullManifest(ctx, stationID, stationName)

// Individual collections
agents, _ := collector.CollectAgents(ctx)
workflows, _ := collector.CollectWorkflows(ctx)

// Lookups
agent, _ := collector.GetAgentByID(ctx, "123")
agent, _ := collector.GetAgentByName(ctx, "k8s-admin")
workflow, _ := collector.GetWorkflowByID(ctx, "deploy-app")
```

### AgentRouter (`internal/lattice/router.go`)

Routing logic for finding agents and workflows:
- Searches across all online stations
- Prefers local agents when available
- Supports lookup by name or capability

```go
router := lattice.NewAgentRouter(registry, stationID)

// Agent routing
locations, _ := router.FindAgentByName(ctx, "k8s-admin")
locations, _ := router.FindAgentByCapability(ctx, "kubernetes")
best, _ := router.FindBestAgent(ctx, "k8s-admin", "")
allAgents, _ := router.ListAllAgents(ctx)

// Workflow routing
locations, _ := router.FindWorkflowByID(ctx, "deploy-app")
best, _ := router.FindBestWorkflow(ctx, "deploy-app")
allWorkflows, _ := router.ListAllWorkflows(ctx)
```

### Invoker (`internal/lattice/invoker.go`)

Remote execution via NATS request-reply:
- Handles incoming agent/workflow invocations
- Sends requests to remote stations
- 5-minute timeout for agents, 10-minute for workflows

```go
invoker := lattice.NewInvoker(client, stationID, executor)
invoker.Start(ctx)  // Start listening for requests
defer invoker.Stop()

// Remote invocation
req := lattice.InvokeAgentRequest{AgentName: "k8s-admin", Task: "List pods"}
response, _ := invoker.InvokeRemoteAgent(ctx, targetStationID, req)

wfReq := lattice.RunWorkflowRequest{WorkflowID: "deploy-app"}
wfResponse, _ := invoker.InvokeRemoteWorkflow(ctx, targetStationID, wfReq)
```

## API Reference

### Data Types

```go
// Station manifest stored in registry
type StationManifest struct {
    StationID   string         `json:"station_id"`
    StationName string         `json:"station_name"`
    Agents      []AgentInfo    `json:"agents"`
    Workflows   []WorkflowInfo `json:"workflows"`
    LastSeen    time.Time      `json:"last_seen"`
    Status      StationStatus  `json:"status"`  // "online" | "offline"
}

// Agent information
type AgentInfo struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Capabilities []string `json:"capabilities"`
}

// Workflow information
type WorkflowInfo struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
}

// Agent location from router
type AgentLocation struct {
    StationID   string
    StationName string
    AgentID     string
    AgentName   string
    IsLocal     bool
}

// Workflow location from router
type WorkflowLocation struct {
    StationID    string
    StationName  string
    WorkflowID   string
    WorkflowName string
    Description  string
    IsLocal      bool
}

// Agent invocation request
type InvokeAgentRequest struct {
    AgentID   string            `json:"agent_id,omitempty"`
    AgentName string            `json:"agent_name,omitempty"`
    Task      string            `json:"task"`
    Context   map[string]string `json:"context,omitempty"`
}

// Agent invocation response
type InvokeAgentResponse struct {
    Status     string  `json:"status"`      // "success" | "error"
    Result     string  `json:"result"`
    Error      string  `json:"error,omitempty"`
    DurationMs float64 `json:"duration_ms"`
    ToolCalls  int     `json:"tool_calls"`
    StationID  string  `json:"station_id"`
}

// Workflow invocation request
type RunWorkflowRequest struct {
    WorkflowID string            `json:"workflow_id"`
    Input      map[string]string `json:"input,omitempty"`
}

// Workflow invocation response
type RunWorkflowResponse struct {
    Status    string `json:"status"`
    RunID     string `json:"run_id,omitempty"`
    Result    string `json:"result,omitempty"`
    Error     string `json:"error,omitempty"`
    StationID string `json:"station_id"`
}

// Presence message types
type PresenceMessage struct {
    StationID   string           `json:"station_id"`
    StationName string           `json:"station_name"`
    Type        PresenceType     `json:"type"`  // "heartbeat" | "announce" | "goodbye"
    Timestamp   time.Time        `json:"timestamp"`
    Manifest    *StationManifest `json:"manifest,omitempty"`
}
```

### Interfaces

```go
// AgentExecutor must be implemented to handle local agent execution
type AgentExecutor interface {
    ExecuteAgentByID(ctx context.Context, agentID string, task string) (result string, toolCalls int, err error)
    ExecuteAgentByName(ctx context.Context, agentName string, task string) (result string, toolCalls int, err error)
}
```

## NATS Subjects

```
# Station lifecycle
lattice.presence.announce            # Station online (includes manifest)
lattice.presence.goodbye             # Station offline
lattice.presence.heartbeat           # Periodic heartbeat

# Agent invocation (request-reply)
lattice.station.{station_id}.agent.invoke
  Request:  InvokeAgentRequest
  Response: InvokeAgentResponse

# Workflow invocation (request-reply)
lattice.station.{station_id}.workflow.run
  Request:  RunWorkflowRequest
  Response: RunWorkflowResponse
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
    reconnect_wait_sec: 2     # Seconds between reconnect attempts
    max_reconnects: -1        # -1 = unlimited
    
    auth:
      user: ""
      password: ""
      token: ""
      nkey_seed: ""
      nkey_file: ""
      creds_file: ""
    
    tls:
      enabled: false
      cert_file: ""
      key_file: ""
      ca_file: ""
      skip_verify: false
  
  orchestrator:
    embedded_nats:
      port: 4222
      http_port: 8222
      store_dir: ""           # Defaults to ~/.local/share/station/lattice/nats
```

### Environment Variables

#### Core Lattice Mode Activation

These env vars are used in Docker/container deployments and config files to activate lattice mode:

| Variable | Description | Example |
|----------|-------------|---------|
| `STN_LATTICE_NATS_URL` | NATS URL to connect as member | `nats://192.168.56.10:4222` |
| `STN_LATTICE_STATION_NAME` | Station name in the lattice | `posthog-member` |
| `STN_LATTICE_STATION_ID` | Custom station ID (auto-generated if empty) | `my-station-id` |

> **Note**: To activate lattice mode via config file, set `lattice_url` at the root level (not nested).
> The env var `STN_LATTICE_NATS_URL` maps to `lattice.nats.url` in config, but the serve command 
> checks `lattice_url` directly. For container deployments, set `lattice_url` in your config.yaml.

#### Orchestrator Mode (Embedded NATS)

| Variable | Description | Default |
|----------|-------------|---------|
| `STN_LATTICE_NATS_PORT` | Embedded NATS client port | `4222` |
| `STN_LATTICE_NATS_HTTP_PORT` | Embedded NATS monitoring port | `8222` |
| `STN_LATTICE_NATS_STORE_DIR` | JetStream storage directory | `~/.local/share/station/lattice/nats` |
| `STN_LATTICE_PRESENCE_TTL_SEC` | Station heartbeat TTL | `30` |
| `STN_LATTICE_ROUTING_TIMEOUT_SEC` | Agent routing timeout | `60` |

#### NATS Authentication

| Variable | Description |
|----------|-------------|
| `STN_LATTICE_NATS_USER` | NATS username |
| `STN_LATTICE_NATS_PASSWORD` | NATS password |
| `STN_LATTICE_NATS_TOKEN` | NATS token auth |
| `STN_LATTICE_NATS_NKEY_SEED` | NKey seed for auth |
| `STN_LATTICE_NATS_NKEY_FILE` | Path to NKey file |
| `STN_LATTICE_NATS_CREDS_FILE` | Path to credentials file |

#### NATS TLS

| Variable | Description |
|----------|-------------|
| `STN_LATTICE_NATS_TLS_ENABLED` | Enable TLS (`true`/`false`) |
| `STN_LATTICE_NATS_TLS_CERT_FILE` | Client certificate path |
| `STN_LATTICE_NATS_TLS_KEY_FILE` | Client key path |
| `STN_LATTICE_NATS_TLS_CA_FILE` | CA certificate path |

#### Example: Docker Compose Environment

```yaml
environment:
  # Activate lattice member mode
  - STN_LATTICE_NATS_URL=nats://192.168.56.10:4222
  - STN_LATTICE_STATION_NAME=my-station
  
  # Or for orchestrator mode
  - STN_LATTICE_NATS_PORT=4222
  - STN_LATTICE_NATS_HTTP_PORT=8222
```

> **Important**: For container deployments where you can't pass CLI flags, you must also set 
> `lattice_url` in the config.yaml file. See [Lattice Lab Tutorial](./LATTICE_LAB.md) for a 
> complete example.

## Remote Agent Invocation Flow

```
Station A (caller)              Orchestrator              Station B (target)
     │                              │                           │
     │ stn lattice agent exec       │                           │
     │ k8s-admin "get pods"         │                           │
     │                              │                           │
     │  1. Connect to lattice       │                           │
     │─────────────────────────────▶│                           │
     │                              │                           │
     │  2. Query registry for       │                           │
     │     agent "k8s-admin"        │                           │
     │─────────────────────────────▶│                           │
     │                              │                           │
     │  3. Found: station-k8s       │                           │
     │◀─────────────────────────────│                           │
     │                              │                           │
     │  4. NATS Request:            │                           │
     │  lattice.station.station-k8s.agent.invoke                │
     │──────────────────────────────────────────────────────────▶
     │                              │                           │
     │                              │      5. Execute locally   │
     │                              │         via AgentExecutor │
     │                              │                           │
     │  6. NATS Reply: result       │                           │
     │◀──────────────────────────────────────────────────────────
     │                              │                           │
     │  7. Display result           │                           │
```

## Testing

```bash
# Unit tests
go test ./internal/lattice/... -v

# Build verification
go build ./...

# Integration test (requires no external dependencies)
go test ./internal/lattice/integration_test.go -v

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

### NATS authentication errors
1. Verify auth configuration matches between orchestrator and members
2. Check credentials file permissions
3. Ensure NKey seeds are valid

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Core infrastructure (client, embedded, registry, presence) | ✅ Complete |
| 2 | Agent discovery and remote invocation | ✅ Complete |
| 3 | Workflow discovery and remote invocation | ✅ Complete |
| 4 | Integration into `stn serve` + task groups | 🔄 Planned |

## Next Steps (Phase 4)

### 4.1 Wire Agent Execution to Invoker

Currently the `Invoker` receives a `nil` executor. Need to create an adapter:

```go
// internal/lattice/executor_adapter.go
type ExecutorAdapter struct {
    engine *services.AgentExecutionEngine
}

func (e *ExecutorAdapter) ExecuteAgentByID(ctx context.Context, agentID, task string) (string, int, error) {
    // Convert to RunCreateParams and execute via engine
}

func (e *ExecutorAdapter) ExecuteAgentByName(ctx context.Context, agentName, task string) (string, int, error) {
    // Look up agent by name, then execute
}
```

### 4.2 Integrate Lattice into `stn serve`

Modify `cmd/main/main.go` serve command to:

1. **If `--orchestration`**:
   - Start embedded NATS server
   - Connect client to localhost NATS
   - Collect and publish manifest
   - Start presence heartbeat
   - Start invoker listener

2. **If `--lattice <url>`**:
   - Connect client to provided URL
   - Collect and publish manifest
   - Start presence heartbeat
   - Start invoker listener

### 4.3 Task Groups (from PRD)

Coordinated multi-task tracking across stations:

```bash
stn lattice taskgroup create "Deploy and verify"
stn lattice taskgroup add-task <group-id> --station station-a --workflow deploy
stn lattice taskgroup add-task <group-id> --station station-b --agent verifier "Check deployment"
stn lattice taskgroup run <group-id>
stn lattice taskgroup status <group-id>
```

## Files

```
internal/lattice/
├── client.go           # NATS client with auth/TLS
├── client_test.go      # Client unit tests
├── embedded.go         # Embedded NATS server
├── embedded_test.go    # Embedded server tests
├── registry.go         # KV-based station registry
├── presence.go         # Heartbeat and discovery
├── discovery.go        # Agent/workflow collection from DB
├── router.go           # Agent/workflow routing
├── invoker.go          # Remote invocation handler
└── integration_test.go # Full integration test

cmd/main/
├── main.go              # --orchestration, --lattice flags
└── lattice_commands.go  # stn lattice subcommands

docs/
├── LATTICE.md           # This documentation
└── LATTICE_PROGRESS.md  # Implementation progress tracking
```
