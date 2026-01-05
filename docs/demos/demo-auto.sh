#!/bin/bash
# Automated Station Lattice Demo
# Runs everything in a single terminal for quick demonstrations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
STN="$STATION_ROOT/stn"
DEMO_WORKSPACE="$HOME/.station-lattice-demo"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# PID tracking
PIDS=()

cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            kill "$pid" 2>/dev/null || true
        fi
    done
    # Also cleanup any remaining station processes
    pkill -f "stn serve.*station-lattice-demo" 2>/dev/null || true
    # Kill by port
    for port in 8585 8586 8587 4222; do
        pid=$(lsof -ti ":$port" 2>/dev/null || true)
        if [ -n "$pid" ]; then
            kill "$pid" 2>/dev/null || true
        fi
    done
    echo -e "${GREEN}Cleanup complete.${NC}"
}

trap cleanup EXIT INT TERM

print_step() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  STEP $1: $2${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

run_command() {
    echo -e "${YELLOW}$ $1${NC}"
    echo ""
    eval "$1"
}

init_station() {
    local name=$1
    local config_path=$2
    local workspace=$3
    local port=$4
    
    mkdir -p "$(dirname $config_path)"
    mkdir -p "$workspace/environments/default/agents"
    
    # Initialize with stn init
    $STN init \
        --config "$config_path" \
        --yes \
        --provider openai \
        --model gpt-4o-mini 2>&1 | grep -v "Telemetry" | grep -v "📊" || true
    
    # Update workspace in config
    sed -i "s|workspace:.*|workspace: $workspace|" "$config_path"
    if [ -n "$port" ]; then
        # Add or update api_port
        if grep -q "api_port:" "$config_path"; then
            sed -i "s|api_port:.*|api_port: $port|" "$config_path"
        else
            echo "api_port: $port" >> "$config_path"
        fi
    fi
}

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE - AUTOMATED DEMO                     ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  This script starts all stations and runs the demo           ║"
echo "║  Press Ctrl+C at any time to stop                            ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check that stn binary exists
if [ ! -f "$STN" ]; then
    echo -e "${RED}Error: stn binary not found at $STN${NC}"
    echo "Build it first with:"
    echo "  cd $STATION_ROOT"
    echo "  go build -o stn ./cmd/main"
    exit 1
fi

# Check for OpenAI API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo -e "${YELLOW}Warning: OPENAI_API_KEY not set. Agents won't be able to use LLM.${NC}"
    echo "Set it with: export OPENAI_API_KEY=your-key"
    echo ""
fi

# Clean up previous demo
rm -rf "$DEMO_WORKSPACE" /tmp/orchestrator-workspace /tmp/sre-workspace /tmp/security-workspace 2>/dev/null || true
rm -f /tmp/orchestrator.db /tmp/sre.db /tmp/security.db 2>/dev/null || true
rm -rf /tmp/nats-orchestrator 2>/dev/null || true

# ==============================================================================
# STEP 1: Start Orchestrator Station
# ==============================================================================
print_step "1" "Starting Orchestrator Station (embedded NATS)"

init_station "orchestrator" "$DEMO_WORKSPACE/orchestrator/config.yaml" "/tmp/orchestrator-workspace" "8585"

# Add lattice orchestrator config
cat >> "$DEMO_WORKSPACE/orchestrator/config.yaml" << 'EOF'

lattice:
  enabled: true
  station_id: orchestrator-station
  station_name: "Orchestrator Hub"
  orchestrator:
    embedded_nats:
      enabled: true
      port: 4222
      http_port: 8222
      store_dir: /tmp/nats-orchestrator
EOF

# Create orchestrator agent
cat > /tmp/orchestrator-workspace/environments/default/agents/Coordinator.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Orchestrates tasks across the lattice mesh"
  capabilities:
    - orchestration
    - coordination
---
You are a coordination agent for the Station Lattice mesh network.
Use list_agents to discover agents, assign_work to delegate, await_work to collect results.
EOF

echo "Starting orchestrator in background..."
cd "$STATION_ROOT"
$STN serve --config "$DEMO_WORKSPACE/orchestrator/config.yaml" --orchestration --local > /tmp/orchestrator.log 2>&1 &
PIDS+=($!)
echo -e "${GREEN}Orchestrator started (PID: ${PIDS[-1]})${NC}"

# Wait for NATS to be ready
echo "Waiting for NATS to be ready..."
for i in {1..30}; do
    if nc -z localhost 4222 2>/dev/null; then
        echo -e "${GREEN}NATS is ready!${NC}"
        break
    fi
    sleep 1
    echo "  Waiting... ($i/30)"
done

if ! nc -z localhost 4222 2>/dev/null; then
    echo -e "${RED}Error: NATS did not start. Check /tmp/orchestrator.log${NC}"
    cat /tmp/orchestrator.log | tail -20
    exit 1
fi

# ==============================================================================
# STEP 2: Start SRE Station
# ==============================================================================
print_step "2" "Starting SRE Station"

init_station "sre" "$DEMO_WORKSPACE/sre/config.yaml" "/tmp/sre-workspace" "8586"

# Add lattice member config
cat >> "$DEMO_WORKSPACE/sre/config.yaml" << 'EOF'

lattice:
  enabled: true
  station_id: sre-station
  station_name: "SRE Operations"
  nats:
    url: nats://localhost:4222
EOF

# Create SRE agents
cat > /tmp/sre-workspace/environments/default/agents/K8sHealthChecker.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Checks Kubernetes cluster health"
  capabilities:
    - kubernetes
    - monitoring
---
You are a Kubernetes health agent. Simulate realistic K8s health reports.
EOF

cat > /tmp/sre-workspace/environments/default/agents/LogAnalyzer.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Analyzes application logs"
  capabilities:
    - logging
    - analysis
---
You are a log analysis agent. Simulate realistic log analysis findings.
EOF

echo "Starting SRE station in background..."
$STN serve --config "$DEMO_WORKSPACE/sre/config.yaml" --lattice nats://localhost:4222 --local > /tmp/sre.log 2>&1 &
PIDS+=($!)
echo -e "${GREEN}SRE station started (PID: ${PIDS[-1]})${NC}"

# ==============================================================================
# STEP 3: Start Security Station
# ==============================================================================
print_step "3" "Starting Security Station"

init_station "security" "$DEMO_WORKSPACE/security/config.yaml" "/tmp/security-workspace" "8587"

# Add lattice member config
cat >> "$DEMO_WORKSPACE/security/config.yaml" << 'EOF'

lattice:
  enabled: true
  station_id: security-station
  station_name: "Security Operations"
  nats:
    url: nats://localhost:4222
EOF

# Create Security agents
cat > /tmp/security-workspace/environments/default/agents/VulnScanner.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Scans for security vulnerabilities"
  capabilities:
    - security
    - vulnerability-scanning
---
You are a vulnerability scanner. Simulate realistic vuln scan results with CVE IDs.
EOF

cat > /tmp/security-workspace/environments/default/agents/NetworkAudit.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Audits network configurations"
  capabilities:
    - security
    - network
    - audit
---
You are a network audit agent. Simulate realistic network audit findings.
EOF

echo "Starting Security station in background..."
$STN serve --config "$DEMO_WORKSPACE/security/config.yaml" --lattice nats://localhost:4222 --local > /tmp/security.log 2>&1 &
PIDS+=($!)
echo -e "${GREEN}Security station started (PID: ${PIDS[-1]})${NC}"

# Wait for stations to register
echo ""
echo "Waiting for stations to register with the lattice..."
sleep 5

# ==============================================================================
# STEP 4: Demo Commands
# ==============================================================================
print_step "4" "Running Demo Commands"

NATS_URL="nats://localhost:4222"
STN_LATTICE="$STN lattice --nats $NATS_URL"

echo -e "${BLUE}4.1 Check Lattice Status${NC}"
echo ""
run_command "$STN_LATTICE status"
sleep 2

echo ""
echo -e "${BLUE}4.2 Discover All Agents${NC}"
echo ""
run_command "$STN_LATTICE agents --discover"
sleep 2

echo ""
echo -e "${BLUE}4.3 Direct Agent Execution (if OpenAI key is set)${NC}"
echo ""
if [ -n "$OPENAI_API_KEY" ]; then
    run_command "$STN_LATTICE agent exec K8sHealthChecker 'Check pod health in production'"
else
    echo -e "${YELLOW}Skipping - OPENAI_API_KEY not set${NC}"
fi

# ==============================================================================
# STEP 5: Summary
# ==============================================================================
print_step "5" "Demo Complete"

echo "All stations are running:"
echo ""
echo "  Orchestrator: http://localhost:8585 (PID: ${PIDS[0]})"
echo "  SRE Station:  http://localhost:8586 (PID: ${PIDS[1]})"
echo "  Security:     http://localhost:8587 (PID: ${PIDS[2]})"
echo ""
echo "Logs available at:"
echo "  /tmp/orchestrator.log"
echo "  /tmp/sre.log"
echo "  /tmp/security.log"
echo ""
echo "You can now:"
echo "  - Run more commands: $STN_LATTICE <command>"
echo "  - Open dashboard: $STN_LATTICE dashboard"
echo "  - Check logs: tail -f /tmp/*.log"
echo ""
echo -e "${YELLOW}Press Ctrl+C to stop all stations and exit.${NC}"
echo ""

# Keep running until interrupted
wait
