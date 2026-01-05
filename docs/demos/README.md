# Station Lattice Demo Lab

This directory contains scripts to demonstrate the Station Lattice distributed agent mesh.

## Overview

The demo sets up a 3-station lattice:
1. **Orchestrator Station** - Hub with embedded NATS, coordinates work
2. **SRE Station** - Leaf node with SRE/DevOps agents
3. **Security Station** - Leaf node with security scanning agents

## Prerequisites

```bash
# Build station CLI
cd /path/to/station
go build -o stn ./cmd/main

# Verify build
./stn --version
```

## Quick Start (Single Terminal)

```bash
# Run the automated demo
./demo-auto.sh
```

## Multi-Terminal Demo (Recommended)

For the full experience, open 4 terminal windows:

### Terminal Layout

```
┌─────────────────────────────┬─────────────────────────────┐
│  Terminal 1: Orchestrator   │  Terminal 2: SRE Station    │
│  ./01-start-orchestrator.sh │  ./02-start-sre-station.sh  │
├─────────────────────────────┼─────────────────────────────┤
│  Terminal 3: Security       │  Terminal 4: User CLI       │
│  ./03-start-security.sh     │  ./04-run-commands.sh       │
└─────────────────────────────┴─────────────────────────────┘
```

### Step-by-Step

1. **Terminal 1** - Start Orchestrator (NATS hub):
   ```bash
   ./01-start-orchestrator.sh
   ```

2. **Terminal 2** - Start SRE Station (wait for orchestrator):
   ```bash
   ./02-start-sre-station.sh
   ```

3. **Terminal 3** - Start Security Station:
   ```bash
   ./03-start-security.sh
   ```

4. **Terminal 4** - Run demo commands:
   ```bash
   ./04-run-commands.sh
   ```

## Demo Scripts

| Script | Description |
|--------|-------------|
| `01-start-orchestrator.sh` | Starts orchestrator with embedded NATS |
| `02-start-sre-station.sh` | Starts SRE station and connects to orchestrator |
| `03-start-security-station.sh` | Starts security station and connects |
| `04-run-commands.sh` | Interactive demo of lattice commands |
| `05-dashboard.sh` | Opens the real-time TUI dashboard |
| `demo-auto.sh` | Automated demo (runs everything in background) |
| `cleanup.sh` | Stops all running stations |

## What You'll See

### 1. Station Discovery
```bash
stn lattice status
# Shows: 3 stations connected, agent counts
```

### 2. Agent Discovery
```bash
stn lattice agents --discover
# Shows: All agents across all stations with capabilities
```

### 3. Async Work Assignment
```bash
stn lattice work assign SecurityScanner "scan /app for vulnerabilities"
# Returns: work_id immediately

stn lattice work await <work_id>
# Shows: Results when complete
```

### 4. Distributed Orchestration (The Main Event!)
```bash
stn lattice run "Analyze security and health of the infrastructure"
# Shows:
#   - Orchestrator receives task
#   - Discovers available agents
#   - Assigns work in parallel to SRE + Security agents
#   - Streams progress updates
#   - Synthesizes final response
```

### 5. Real-Time Dashboard
```bash
stn lattice dashboard
# Shows: TUI with active work, completions, station status
```

## Architecture Diagram

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
│ - Dispatcher  │     │ - LogAnalyzer │     │ - CVELookup   │
│               │     │ - Deployer    │     │ - NetworkAudit│
│ Port: 8585    │     │ Port: 8586    │     │ Port: 8587    │
└───────────────┘     └───────────────┘     └───────────────┘
```

## Troubleshooting

### NATS Connection Failed
```bash
# Check if orchestrator is running
ps aux | grep "stn serve"

# Check NATS port
nc -zv localhost 4222
```

### Agents Not Showing
```bash
# Wait for heartbeat (5 seconds)
sleep 5
stn lattice agents
```

### Work Timeout
```bash
# Increase timeout
stn lattice run --timeout 15m "complex task"
```
