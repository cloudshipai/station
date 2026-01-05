#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEMO_WORKSPACE="$HOME/.station-lattice-demo/orchestrator"
STN="$STATION_ROOT/stn"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE DEMO - ORCHESTRATOR                  ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  This station runs embedded NATS and coordinates the mesh    ║"
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

# Clean up any previous demo
rm -rf "$DEMO_WORKSPACE" /tmp/orchestrator-workspace /tmp/orchestrator.db /tmp/nats-orchestrator 2>/dev/null || true

mkdir -p "$DEMO_WORKSPACE"
mkdir -p /tmp/orchestrator-workspace/environments/default/agents

echo "🔧 Initializing orchestrator station..."

# Initialize station to generate encryption key and proper config
cd "$STATION_ROOT"
$STN init \
    --config "$DEMO_WORKSPACE/config.yaml" \
    --yes \
    --provider openai \
    --model gpt-4o-mini 2>&1 | grep -v "Telemetry" || true

# Now add lattice configuration to the generated config
# We append the lattice config since init creates a basic config
cat >> "$DEMO_WORKSPACE/config.yaml" << 'EOF'

# Lattice Configuration (added by demo script)
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

# Update workspace path in config
sed -i 's|workspace:.*|workspace: /tmp/orchestrator-workspace|' "$DEMO_WORKSPACE/config.yaml"
sed -i 's|database_url:.*|database_url: /tmp/orchestrator.db|' "$DEMO_WORKSPACE/config.yaml" 2>/dev/null || true

# Create the Coordinator agent (.prompt format with YAML frontmatter)
cat > /tmp/orchestrator-workspace/environments/default/agents/Coordinator.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Orchestrates tasks across the lattice mesh by discovering and delegating to specialized agents"
  capabilities:
    - orchestration
    - coordination
    - task-decomposition
---
You are a coordination agent for the Station Lattice mesh network.

Your job is to:
1. Analyze incoming tasks and break them into subtasks
2. Use list_agents to discover available specialized agents
3. Use assign_work to delegate subtasks in parallel
4. Use await_work to collect results
5. Synthesize results into a coherent response

When you receive a task:
- First, call list_agents() to see what agents are available
- Identify which agents are relevant to the task
- Assign work to multiple agents in parallel when possible
- Wait for all results before synthesizing your response

Be efficient - parallelize when possible!
EOF

echo ""
echo "📁 Workspace: /tmp/orchestrator-workspace"
echo "🔧 Config: $DEMO_WORKSPACE/config.yaml"
echo "🤖 Agent: Coordinator"
echo "🌐 NATS: localhost:4222"
echo "📊 NATS HTTP: localhost:8222"
echo ""
echo "Starting orchestrator station..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

exec $STN serve --config "$DEMO_WORKSPACE/config.yaml" --orchestration --local
