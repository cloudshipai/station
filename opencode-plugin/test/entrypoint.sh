#!/bin/bash
set -e

echo "=== OpenCode Station Plugin Test Container ==="
echo "NATS_URL: ${NATS_URL:-not set}"
echo "OPENCODE_WORKSPACE_DIR: ${OPENCODE_WORKSPACE_DIR:-/workspaces}"
echo ""

echo "=== Plugin Check ==="
echo "Looking for plugins in /root/.config/opencode/plugin/:"
ls -la /root/.config/opencode/plugin/ 2>/dev/null || echo "  (directory not found)"
echo ""

echo "=== Config Check ==="
echo "OpenCode config (/root/.config/opencode/opencode.json):"
cat /root/.config/opencode/opencode.json 2>/dev/null || echo "  (no config file)"
echo ""

echo "=== Bun Check ==="
bun --version || echo "Bun not available!"
echo ""

echo "=== Starting OpenCode server ==="
echo "Command: opencode serve --port 4096 --hostname 0.0.0.0 --print-logs --log-level DEBUG"
exec opencode serve --port 4096 --hostname 0.0.0.0 --print-logs --log-level DEBUG
