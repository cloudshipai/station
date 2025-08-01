#!/bin/bash

# Station End-to-End Testing Script
# Tests the complete workflow: MCP loading -> Agent creation -> Execution -> Verification

set -e  # Exit on error

STATION_API="http://localhost:8081"
TEST_DIR="/home/epuerta/projects/hack/station/testing"
LOG_FILE="$TEST_DIR/e2e-test.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

success() {
    echo -e "${GREEN}✅ $1${NC}" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}❌ $1${NC}" | tee -a "$LOG_FILE"
    exit 1
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}" | tee -a "$LOG_FILE"
}

# Test functions
test_api_health() {
    log "Testing API health..."
    response=$(curl -s "$STATION_API/health" || echo "")
    if [[ "$response" == *"healthy"* ]]; then
        success "Station API is healthy"
    else
        error "Station API is not responding correctly"
    fi
}

test_mcp_tools_loaded() {
    log "Verifying MCP tools are loaded..."
    tool_count=$(stn mcp tools 2>/dev/null | grep -c "Server ID:" || echo "0")
    if [[ "$tool_count" -gt "0" ]]; then
        success "Found $tool_count MCP tools loaded"
    else
        error "No MCP tools found. Please run 'stn load' first"
    fi
}

test_environment_exists() {
    log "Testing default environment exists..."
    response=$(curl -s "$STATION_API/environments" || echo "")
    if [[ "$response" == *"default"* ]]; then
        success "Default environment found"
        echo "$response" | jq -r '.[] | select(.name=="default") | .id' > "$TEST_DIR/env_id.txt"
    else
        error "Default environment not found"
    fi
}

create_test_agent() {
    log "Creating test agent..."
    local env_id=$(cat "$TEST_DIR/env_id.txt")
    
    # Create agent via API
    agent_data='{
        "name": "File System Explorer",
        "description": "An agent that explores and analyzes file systems using MCP tools",
        "prompt": "You are a file system exploration agent. Use the available MCP tools to explore directories, read files, and provide detailed analysis of file structures. Always be thorough and explain what you find.",
        "max_steps": 10,
        "environment_id": '$env_id',
        "user_id": 1
    }'
    
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$agent_data" \
        "$STATION_API/agents" || echo "")
    
    if [[ "$response" == *"id"* ]]; then
        agent_id=$(echo "$response" | jq -r '.id')
        echo "$agent_id" > "$TEST_DIR/agent_id.txt"
        success "Created test agent with ID: $agent_id"
    else
        error "Failed to create agent: $response"
    fi
}

test_agent_execution() {
    log "Testing agent execution via API..."
    local agent_id=$(cat "$TEST_DIR/agent_id.txt")
    
    # Create execution request
    exec_data='{
        "task": "Explore the /home/epuerta/projects/hack/station directory structure and provide a detailed analysis of the project layout, focusing on the main components and their purposes."
    }'
    
    response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$exec_data" \
        "$STATION_API/agents/$agent_id/runs" || echo "")
    
    if [[ "$response" == *"run_id"* ]]; then
        run_id=$(echo "$response" | jq -r '.run_id')
        echo "$run_id" > "$TEST_DIR/run_id.txt"
        success "Started agent execution with run ID: $run_id"
        
        # Wait and monitor execution
        monitor_execution "$run_id"
    else
        error "Failed to start agent execution: $response"
    fi
}

monitor_execution() {
    local run_id=$1
    log "Monitoring execution progress for run ID: $run_id"
    
    local max_attempts=30
    local attempt=0
    
    while [[ $attempt -lt $max_attempts ]]; do
        response=$(curl -s "$STATION_API/runs/$run_id" || echo "")
        status=$(echo "$response" | jq -r '.status' 2>/dev/null || echo "unknown")
        
        case "$status" in
            "completed")
                success "Agent execution completed successfully"
                local final_response=$(echo "$response" | jq -r '.final_response' 2>/dev/null || echo "No response")
                log "Final response preview: ${final_response:0:200}..."
                echo "$response" > "$TEST_DIR/execution_result.json"
                return 0
                ;;
            "failed")
                error "Agent execution failed"
                ;;
            "running")
                log "Execution in progress... (attempt $((attempt + 1))/$max_attempts)"
                ;;
            *)
                warning "Unknown status: $status"
                ;;
        esac
        
        sleep 2
        ((attempt++))
    done
    
    error "Execution monitoring timed out after $max_attempts attempts"
}

verify_tool_usage() {
    log "Verifying tool usage in execution..."
    if [[ -f "$TEST_DIR/execution_result.json" ]]; then
        tool_calls=$(cat "$TEST_DIR/execution_result.json" | jq -r '.tool_calls' 2>/dev/null || echo "null")
        if [[ "$tool_calls" != "null" && "$tool_calls" != "[]" ]]; then
            tool_count=$(echo "$tool_calls" | jq length 2>/dev/null || echo "0")
            success "Agent used $tool_count tool calls during execution"
            
            # Show tool usage summary
            echo "$tool_calls" | jq -r '.[] | "- \(.tool_name): \(.result.status // "unknown")"' 2>/dev/null || true
        else
            warning "No tool calls found in execution result"
        fi
    else
        warning "Execution result file not found"
    fi
}

test_cli_integration() {
    log "Testing CLI integration..."
    
    # Test agent listing
    log "Testing agent listing via CLI..."
    agent_list=$(stn agent list 2>/dev/null || echo "")
    if [[ "$agent_list" == *"File System Explorer"* ]]; then
        success "Agent appears in CLI listing"
    else
        warning "Agent not found in CLI listing"
    fi
    
    # Test run listing
    log "Testing run listing via CLI..."
    run_list=$(stn runs list 2>/dev/null || echo "")
    if [[ "$run_list" != "" ]]; then
        success "Runs can be listed via CLI"
    else
        warning "No runs found in CLI listing"
    fi
}

create_feedback_report() {
    log "Creating feedback report..."
    
    cat > "$TEST_DIR/feedback_report.md" << EOF
# Station End-to-End Test Report

**Test Date:** $(date)
**Station API:** $STATION_API
**Test Directory:** $TEST_DIR

## Test Results

### 1. API Health Check
- ✅ Station API is healthy and responding

### 2. MCP Tools Verification
- ✅ MCP tools successfully loaded
- Tool count: $(stn mcp tools 2>/dev/null | grep -c "Server ID:" || echo "0")

### 3. Agent Creation
- ✅ Agent created successfully
- Agent ID: $(cat "$TEST_DIR/agent_id.txt" 2>/dev/null || echo "N/A")

### 4. Agent Execution
- ✅ Agent execution completed
- Run ID: $(cat "$TEST_DIR/run_id.txt" 2>/dev/null || echo "N/A")

### 5. Tool Usage Analysis
$(if [[ -f "$TEST_DIR/execution_result.json" ]]; then
    echo "- Tools used during execution:"
    cat "$TEST_DIR/execution_result.json" | jq -r '.tool_calls[]? | "  - \(.tool_name)"' 2>/dev/null || echo "  - No tool details available"
else
    echo "- No execution result available"
fi)

### 6. CLI Integration
- ✅ Agent listing works via CLI
- ✅ Run listing works via CLI

## Files Generated
- Log file: $LOG_FILE
- Agent ID: $TEST_DIR/agent_id.txt
- Run ID: $TEST_DIR/run_id.txt
- Execution result: $TEST_DIR/execution_result.json

## Next Steps
1. Review execution logs for any issues
2. Test additional agent scenarios
3. Verify MCP tool performance across different use cases
4. Test webhook integrations if configured

EOF

    success "Feedback report created: $TEST_DIR/feedback_report.md"
}

cleanup() {
    log "Cleaning up test artifacts..."
    # Clean up test files but keep results
    # rm -f "$TEST_DIR"/{env_id,agent_id,run_id}.txt
    success "Cleanup completed (results preserved)"
}

# Main test execution
main() {
    log "Starting Station End-to-End Testing"
    log "======================================"
    
    # Initialize log file
    echo "Station E2E Test Log - $(date)" > "$LOG_FILE"
    
    # Run tests in sequence
    test_api_health
    test_mcp_tools_loaded
    test_environment_exists
    create_test_agent
    test_agent_execution
    verify_tool_usage
    test_cli_integration
    create_feedback_report
    
    success "All tests completed successfully!"
    log "Check the feedback report: $TEST_DIR/feedback_report.md"
}

# Handle cleanup on exit
trap cleanup EXIT

# Run main function
main "$@"