#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEMO_WORKSPACE="$HOME/.station-lattice-demo/security"
STN="$STATION_ROOT/stn"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE DEMO - SECURITY STATION              ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  This station provides security scanning agents              ║"
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
rm -rf "$DEMO_WORKSPACE" /tmp/security-workspace /tmp/security.db 2>/dev/null || true

mkdir -p "$DEMO_WORKSPACE"
mkdir -p /tmp/security-workspace/environments/default/agents

echo ""
echo "🔧 Initializing Security station..."

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
  station_id: security-station
  station_name: "Security Operations"
  nats:
    url: nats://localhost:4222
EOF

# Update workspace and port in config
sed -i 's|workspace:.*|workspace: /tmp/security-workspace|' "$DEMO_WORKSPACE/config.yaml"
sed -i 's|database_url:.*|database_url: /tmp/security.db|' "$DEMO_WORKSPACE/config.yaml" 2>/dev/null || true
sed -i 's|api_port:.*|api_port: 8587|' "$DEMO_WORKSPACE/config.yaml" 2>/dev/null || true

# Create Security agents (.prompt format)
cat > /tmp/security-workspace/environments/default/agents/VulnScanner.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Scans applications and infrastructure for security vulnerabilities"
  capabilities:
    - security
    - vulnerability-scanning
    - compliance
---
You are a vulnerability scanning agent.

When asked to scan:
- Check for known CVEs
- Identify misconfigurations
- Scan for exposed secrets
- Check dependency vulnerabilities

For this demo, simulate realistic vulnerability scan results.
Report severity levels (Critical, High, Medium, Low).
Include CVE IDs when applicable.
EOF

cat > /tmp/security-workspace/environments/default/agents/CVELookup.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Looks up CVE details and provides remediation guidance"
  capabilities:
    - security
    - cve-research
    - remediation
---
You are a CVE research agent.

When asked about vulnerabilities:
- Provide CVE details (ID, CVSS score, description)
- Explain attack vectors
- Suggest remediation steps
- List affected versions

For this demo, simulate realistic CVE information.
EOF

cat > /tmp/security-workspace/environments/default/agents/NetworkAudit.prompt << 'EOF'
---
model: gpt-4o-mini
metadata:
  description: "Audits network configurations, firewall rules, and exposed services"
  capabilities:
    - security
    - network
    - firewall
    - audit
---
You are a network security audit agent.

When asked to audit:
- Check firewall rules
- Identify open ports
- Review network segmentation
- Detect exposed services

For this demo, simulate realistic network audit findings.
Include specific port numbers and service names.
EOF

echo ""
echo "📁 Workspace: /tmp/security-workspace"
echo "🔧 Config: $DEMO_WORKSPACE/config.yaml"
echo "🤖 Agents: VulnScanner, CVELookup, NetworkAudit"
echo "🔗 Connecting to: nats://localhost:4222"
echo ""
echo "Starting Security station..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

exec $STN serve --config "$DEMO_WORKSPACE/config.yaml" --lattice nats://localhost:4222 --local
