#!/bin/bash
# Launch the Station Lattice TUI Dashboard

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
STN="$STATION_ROOT/stn"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE DASHBOARD                            ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  Real-time monitoring of the lattice mesh                    ║"
echo "║  Press 'q' to exit                                           ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check that stn binary exists
if [ ! -f "$STN" ]; then
    echo "Error: stn binary not found at $STN"
    echo "Build it first with: go build -o stn ./cmd/main"
    exit 1
fi

# Check NATS connection
if ! nc -z localhost 4222 2>/dev/null; then
    echo "Error: Cannot connect to NATS on localhost:4222"
    echo "Make sure the orchestrator is running (./01-start-orchestrator.sh)"
    exit 1
fi

echo "Launching dashboard..."
echo ""

exec "$STN" lattice --nats nats://localhost:4222 dashboard
