#!/bin/bash
# Cleanup script for Station Lattice Demo
# Stops all running demo stations and cleans up temp files

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE DEMO - CLEANUP                       ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Kill station processes
echo "Stopping station processes..."

# Find and kill any stn serve processes related to the demo
KILLED=0

# By config path pattern
for pid in $(pgrep -f "stn serve.*station-lattice-demo" 2>/dev/null); do
    echo -e "  Killing stn process (PID: $pid)"
    kill "$pid" 2>/dev/null && KILLED=$((KILLED + 1))
done

# By port (orchestrator: 8585, sre: 8586, security: 8587)
for port in 8585 8586 8587; do
    pid=$(lsof -ti ":$port" 2>/dev/null || true)
    if [ -n "$pid" ]; then
        echo -e "  Killing process on port $port (PID: $pid)"
        kill "$pid" 2>/dev/null && KILLED=$((KILLED + 1))
    fi
done

# Kill embedded NATS (port 4222)
pid=$(lsof -ti :4222 2>/dev/null || true)
if [ -n "$pid" ]; then
    echo -e "  Killing NATS process (PID: $pid)"
    kill "$pid" 2>/dev/null && KILLED=$((KILLED + 1))
fi

if [ $KILLED -eq 0 ]; then
    echo -e "${YELLOW}No running station processes found.${NC}"
else
    echo -e "${GREEN}Killed $KILLED process(es).${NC}"
fi

echo ""

# Clean up temp files
echo "Cleaning up temporary files..."

CLEANED=0

# Demo workspace
if [ -d "$HOME/.station-lattice-demo" ]; then
    rm -rf "$HOME/.station-lattice-demo"
    echo "  Removed ~/.station-lattice-demo"
    CLEANED=$((CLEANED + 1))
fi

# Temp workspaces
for dir in /tmp/orchestrator-workspace /tmp/sre-workspace /tmp/security-workspace; do
    if [ -d "$dir" ]; then
        rm -rf "$dir"
        echo "  Removed $dir"
        CLEANED=$((CLEANED + 1))
    fi
done

# Databases
for db in /tmp/orchestrator.db /tmp/sre.db /tmp/security.db; do
    if [ -f "$db" ]; then
        rm -f "$db"
        echo "  Removed $db"
        CLEANED=$((CLEANED + 1))
    fi
done

# NATS store
if [ -d "/tmp/nats-orchestrator" ]; then
    rm -rf /tmp/nats-orchestrator
    echo "  Removed /tmp/nats-orchestrator"
    CLEANED=$((CLEANED + 1))
fi

# Log files
for log in /tmp/orchestrator.log /tmp/sre.log /tmp/security.log; do
    if [ -f "$log" ]; then
        rm -f "$log"
        echo "  Removed $log"
        CLEANED=$((CLEANED + 1))
    fi
done

if [ $CLEANED -eq 0 ]; then
    echo -e "${YELLOW}No temporary files found.${NC}"
else
    echo -e "${GREEN}Cleaned up $CLEANED item(s).${NC}"
fi

echo ""
echo -e "${GREEN}Cleanup complete!${NC}"
echo ""
