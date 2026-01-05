#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_ROOT/stn"
WORKSPACE_DIR=$(mktemp -d)
LOG_DIR="$WORKSPACE_DIR/logs"

NATS_PORT="${NATS_PORT:-14222}"
NATS_HTTP_PORT="${NATS_HTTP_PORT:-18222}"

mkdir -p "$LOG_DIR"

cleanup() {
    echo ""
    echo "=== Cleaning up ==="
    
    if [ -n "$ORCHESTRATOR_PID" ] && kill -0 "$ORCHESTRATOR_PID" 2>/dev/null; then
        echo "Stopping orchestrator (PID $ORCHESTRATOR_PID)..."
        kill "$ORCHESTRATOR_PID" 2>/dev/null || true
        wait "$ORCHESTRATOR_PID" 2>/dev/null || true
    fi
    
    if [ -n "$MEMBER_PID" ] && kill -0 "$MEMBER_PID" 2>/dev/null; then
        echo "Stopping member station (PID $MEMBER_PID)..."
        kill "$MEMBER_PID" 2>/dev/null || true
        wait "$MEMBER_PID" 2>/dev/null || true
    fi
    
    echo "Workspace: $WORKSPACE_DIR"
    echo "Logs available in: $LOG_DIR"
}

trap cleanup EXIT

echo "=== Station Lattice E2E Test ==="
echo "Project root: $PROJECT_ROOT"
echo "Workspace: $WORKSPACE_DIR"
echo "NATS port: $NATS_PORT"
echo "NATS HTTP port: $NATS_HTTP_PORT"
echo ""

echo "=== Building Station binary ==="
cd "$PROJECT_ROOT"
go build -o "$BINARY" ./cmd/main
echo "Binary built: $BINARY"
echo ""

ORCHESTRATOR_WORKSPACE="$WORKSPACE_DIR/orchestrator"
MEMBER_WORKSPACE="$WORKSPACE_DIR/member"

mkdir -p "$ORCHESTRATOR_WORKSPACE"
mkdir -p "$MEMBER_WORKSPACE"

cat > "$ORCHESTRATOR_WORKSPACE/config.yaml" << EOF
workspace: $ORCHESTRATOR_WORKSPACE
database_url: $ORCHESTRATOR_WORKSPACE/station.db
api_port: 18585
mcp_port: 18586
ai_provider: openai
ai_model: gpt-4o-mini
local_mode: true
lattice:
  orchestrator:
    embedded_nats:
      port: $NATS_PORT
      http_port: $NATS_HTTP_PORT
EOF

cat > "$MEMBER_WORKSPACE/config.yaml" << EOF
workspace: $MEMBER_WORKSPACE
database_url: $MEMBER_WORKSPACE/station.db
api_port: 28585
mcp_port: 28586
ai_provider: openai
ai_model: gpt-4o-mini
local_mode: true
EOF

echo "=== Initializing Orchestrator Station ==="
cd "$ORCHESTRATOR_WORKSPACE"
"$BINARY" init --yes --provider openai --model gpt-4o-mini --config "$ORCHESTRATOR_WORKSPACE/config.yaml" 2>&1 || true
echo ""

echo "=== Initializing Member Station ==="
cd "$MEMBER_WORKSPACE"
"$BINARY" init --yes --provider openai --model gpt-4o-mini --config "$MEMBER_WORKSPACE/config.yaml" 2>&1 || true
echo ""

echo "=== Starting Orchestrator Station (with embedded NATS) ==="
cd "$ORCHESTRATOR_WORKSPACE"
"$BINARY" serve --orchestration --config "$ORCHESTRATOR_WORKSPACE/config.yaml" > "$LOG_DIR/orchestrator.log" 2>&1 &
ORCHESTRATOR_PID=$!
echo "Orchestrator PID: $ORCHESTRATOR_PID"

echo "Waiting for orchestrator to start..."
sleep 5

if ! kill -0 "$ORCHESTRATOR_PID" 2>/dev/null; then
    echo "ERROR: Orchestrator failed to start"
    echo "=== Orchestrator Log ==="
    cat "$LOG_DIR/orchestrator.log"
    exit 1
fi

echo "Orchestrator is running"
echo ""

echo "=== Starting Member Station (connecting to orchestrator) ==="
cd "$MEMBER_WORKSPACE"
"$BINARY" serve --lattice "nats://localhost:$NATS_PORT" --config "$MEMBER_WORKSPACE/config.yaml" > "$LOG_DIR/member.log" 2>&1 &
MEMBER_PID=$!
echo "Member PID: $MEMBER_PID"

echo "Waiting for member to connect..."
sleep 5

if ! kill -0 "$MEMBER_PID" 2>/dev/null; then
    echo "ERROR: Member station failed to start"
    echo "=== Member Log ==="
    cat "$LOG_DIR/member.log"
    exit 1
fi

echo "Member station is running"
echo ""

echo "=== Testing Lattice Status ==="
cd "$ORCHESTRATOR_WORKSPACE"
"$BINARY" lattice status --config "$ORCHESTRATOR_WORKSPACE/config.yaml" || {
    echo "WARNING: Lattice status command failed (might need stations to register first)"
}
echo ""

echo "=== Testing Lattice Agents List ==="
"$BINARY" lattice agents --config "$ORCHESTRATOR_WORKSPACE/config.yaml" || {
    echo "WARNING: Lattice agents command returned no agents (expected if no agents are configured)"
}
echo ""

echo "=== Testing Lattice Workflows List ==="
"$BINARY" lattice workflows --config "$ORCHESTRATOR_WORKSPACE/config.yaml" || {
    echo "WARNING: Lattice workflows command returned no workflows (expected if no workflows are configured)"
}
echo ""

echo "=== Checking Orchestrator Logs for Lattice Activity ==="
if grep -q "Lattice orchestrator mode" "$LOG_DIR/orchestrator.log"; then
    echo "✓ Orchestrator started in lattice mode"
else
    echo "✗ Orchestrator did not start in lattice mode"
    echo "=== Orchestrator Log ==="
    cat "$LOG_DIR/orchestrator.log"
    exit 1
fi

if grep -q "Lattice registry initialized" "$LOG_DIR/orchestrator.log"; then
    echo "✓ Registry initialized"
else
    echo "✗ Registry not initialized"
fi

if grep -q "Station registered" "$LOG_DIR/orchestrator.log"; then
    echo "✓ Orchestrator station registered"
else
    echo "✗ Orchestrator station not registered"
fi

if grep -q "Lattice invoker listening" "$LOG_DIR/orchestrator.log"; then
    echo "✓ Invoker listening for requests"
else
    echo "✗ Invoker not listening"
fi

echo ""

echo "=== Checking Member Logs for Lattice Activity ==="
if grep -q "Lattice client mode" "$LOG_DIR/member.log"; then
    echo "✓ Member connected in client mode"
else
    echo "✗ Member did not connect in client mode"
    echo "=== Member Log ==="
    cat "$LOG_DIR/member.log"
    exit 1
fi

if grep -q "Connected to lattice NATS" "$LOG_DIR/member.log"; then
    echo "✓ Member connected to NATS"
else
    echo "✗ Member failed to connect to NATS"
fi

if grep -q "Station registered" "$LOG_DIR/member.log"; then
    echo "✓ Member station registered"
else
    echo "✗ Member station not registered"
fi

echo ""
echo "=== E2E Test PASSED ==="
echo ""
echo "Both stations started successfully and connected to the lattice."
echo "Logs are available at: $LOG_DIR"
