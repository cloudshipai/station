#!/bin/bash

# Simple Station Testing Script
# Tests the CLI workflow: MCP tools -> Agent creation -> Execution

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log() { echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"; }
success() { echo -e "${GREEN}✅ $1${NC}"; }
error() { echo -e "${RED}❌ $1${NC}"; exit 1; }

log "Starting simple Station test"

# Test 1: Verify MCP tools are loaded
log "Checking MCP tools..."
tool_count=$(stn mcp tools | grep -c "Server ID:" || echo "0")
if [[ "$tool_count" -gt "0" ]]; then
    success "Found $tool_count MCP tools"
else
    error "No MCP tools found"
fi

# Test 2: List environments
log "Checking environments..."
stn env list || error "Failed to list environments"
success "Environment listing works"

# Test 3: Create a test agent
log "Creating test agent..."
agent_output=$(stn agent create \
    --name "File Explorer Test" \
    --description "Test agent for exploring files" \
    --prompt "You are a file exploration agent. Use MCP tools to explore and analyze file structures." \
    --max-steps 5 \
    --environment default 2>&1 || echo "FAILED")

if [[ "$agent_output" == *"FAILED"* || "$agent_output" == *"error"* ]]; then
    error "Failed to create agent: $agent_output"
else
    success "Agent created successfully"
fi

# Test 4: List agents
log "Listing agents..."
stn agent list || error "Failed to list agents"
success "Agent listing works"

log "Simple test completed successfully!"
echo
echo "Next steps:"
echo "1. Start station server: stn serve"
echo "2. Use SSH: ssh admin@localhost -p 2223"
echo "3. Or use UI: stn ui"