#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEMO_WORKSPACE="$HOME/.station-lattice-demo/sre"
STN="$STATION_ROOT/stn"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE DEMO - SRE STATION                   ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  This station provides SRE/DevOps agents to the lattice      ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check that stn binary exists
if [ ! -f "$STN" ]; then
    echo "Error: stn binary not found at $STN"
    echo "Build it first with: cd $STATION_ROOT && go build -o stn ./cmd/main"
    exit 1
fi

# Check for OpenAI API key
if [ -z "$OPENAI_API_KEY" ]; then
    echo "Warning: OPENAI_API_KEY not set. Agents won't be able to use LLM."
    echo "Set it with: export OPENAI_API_KEY=your-key"
    echo ""
fi

# Wait for orchestrator NATS to be ready
echo "Waiting for orchestrator NATS to be ready..."
for i in {1..30}; do
    if nc -z localhost 4222 2>/dev/null; then
        echo "✅ NATS is available!"
        break
    fi
    echo "   Waiting... ($i/30)"
    sleep 1
done

if ! nc -z localhost 4222 2>/dev/null; then
    echo "❌ Could not connect to NATS on localhost:4222"
    echo "   Make sure the orchestrator is running first:"
    echo "   ./01-start-orchestrator.sh"
    exit 1
fi

# Clean up any previous demo
rm -rf "$DEMO_WORKSPACE" /tmp/sre-workspace /tmp/sre.db 2>/dev/null || true

mkdir -p "$DEMO_WORKSPACE"
mkdir -p /tmp/sre-workspace/environments/default/agents

echo ""
echo "🔧 Initializing SRE station..."

# Initialize station to generate encryption key and proper config
cd "$STATION_ROOT"
$STN init \
    --config "$DEMO_WORKSPACE/config.yaml" \
    --yes \
    --provider openai \
    --model gpt-4o-mini 2>&1 | grep -v "Telemetry" || true

# Add lattice configuration
cat >> "$DEMO_WORKSPACE/config.yaml" << 'EOF'

# Lattice Configuration (added by demo script)
lattice:
  enabled: true
  station_id: sre-station
  station_name: "SRE Operations"
  nats:
    url: nats://localhost:4222
EOF

# Update workspace and port in config
sed -i 's|workspace:.*|workspace: /tmp/sre-workspace|' "$DEMO_WORKSPACE/config.yaml"
sed -i 's|database_url:.*|database_url: /tmp/sre.db|' "$DEMO_WORKSPACE/config.yaml" 2>/dev/null || true
sed -i 's|api_port:.*|api_port: 8586|' "$DEMO_WORKSPACE/config.yaml" 2>/dev/null || true

# Create SRE agents (.prompt format)
cat > /tmp/sre-workspace/environments/default/agents/K8sHealthChecker.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Checks Kubernetes cluster health, pod status, and resource utilization"
  capabilities:
    - kubernetes
    - monitoring
    - health-check
---
You are a Kubernetes health monitoring agent.

When asked to check health:
- Report on pod status (Running, Pending, Failed)
- Check resource utilization (CPU, Memory)
- Identify any pods in CrashLoopBackOff
- Note any pending deployments

For this demo, simulate realistic responses as if connected to a cluster.
Include specific pod names and namespaces in your reports.
EOF

cat > /tmp/sre-workspace/environments/default/agents/LogAnalyzer.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Analyzes application logs for errors, patterns, and anomalies"
  capabilities:
    - logging
    - analysis
    - troubleshooting
---
You are a log analysis agent.

When asked to analyze logs:
- Look for ERROR and WARN level messages
- Identify patterns and recurring issues
- Correlate timestamps with incidents
- Suggest root causes

For this demo, simulate realistic log analysis findings.
EOF

cat > /tmp/sre-workspace/environments/default/agents/Deployer.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Manages deployments, rollouts, and rollbacks"
  capabilities:
    - deployment
    - rollout
    - devops
---
You are a deployment management agent.

You can:
- Deploy new versions
- Monitor rollout progress
- Perform rollbacks
- Manage canary deployments

For this demo, simulate deployment operations.
Report deployment status and progress percentage.
EOF

echo ""
echo "📁 Workspace: /tmp/sre-workspace"
echo "🔧 Config: $DEMO_WORKSPACE/config.yaml"
echo "🤖 Agents: K8sHealthChecker, LogAnalyzer, Deployer"
echo "🔗 Connecting to: nats://localhost:4222"
echo ""
echo "Starting SRE station..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

exec $STN serve --config "$DEMO_WORKSPACE/config.yaml" --lattice nats://localhost:4222 --local
