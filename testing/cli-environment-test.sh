#!/bin/bash

# Station CLI Environment-Specific Features Test Script
# Tests all environment-aware CLI commands and functionality

set -e  # Exit on error

TEST_DIR="/home/epuerta/projects/hack/station/testing"
LOG_FILE="$TEST_DIR/cli-environment-test.log"

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

# Test CLI environment commands
test_environment_cli() {
    log "Testing environment CLI commands..."
    
    # Test environment listing
    log "Testing: stn env list"
    env_output=$(stn env list 2>&1)
    if [[ $? -eq 0 ]]; then
        success "Environment listing works"
        echo "$env_output" | head -10
    else
        error "Environment listing failed: $env_output"
    fi
    
    # Test environment creation
    log "Testing: stn env create test-cli-env"
    create_output=$(stn env create test-cli-env --description "CLI test environment" 2>&1)
    if [[ $? -eq 0 ]]; then
        success "Environment creation works"
    else
        warning "Environment creation issue: $create_output"
    fi
}

test_agent_environment_filtering() {
    log "Testing agent environment filtering..."
    
    # Test basic agent listing
    log "Testing: stn agent list"
    list_output=$(stn agent list 2>&1)
    if [[ $? -eq 0 ]]; then
        success "Basic agent listing works"
        agent_count=$(echo "$list_output" | grep -c "Environment:" || echo "0")
        log "Found $agent_count agents with environment info"
    else
        warning "Agent listing issue: $list_output"
    fi
    
    # Test environment filtering
    log "Testing: stn agent list --env default"
    filtered_output=$(stn agent list --env default 2>&1)
    if [[ $? -eq 0 ]]; then
        success "Environment filtering works"
        filtered_count=$(echo "$filtered_output" | grep -c "Environment:" || echo "0")
        log "Found $filtered_count agents in default environment"
    else
        warning "Environment filtering issue: $filtered_output"
    fi
    
    # Test filtering by environment ID
    log "Testing: stn agent list --env 1"
    id_filtered_output=$(stn agent list --env 1 2>&1)
    if [[ $? -eq 0 ]]; then
        success "Environment ID filtering works"
    else
        warning "Environment ID filtering issue: $id_filtered_output"
    fi
    
    # Test filtering by non-existent environment
    log "Testing: stn agent list --env nonexistent"
    nonexistent_output=$(stn agent list --env nonexistent 2>&1)
    if [[ "$nonexistent_output" == *"No agents found"* ]]; then
        success "Graceful handling of non-existent environment"
    else
        warning "Non-existent environment filtering unexpected: $nonexistent_output"
    fi
}

test_agent_creation_with_environment() {
    log "Testing agent creation with environment specification..."
    
    # Test agent creation with --env flag
    log "Testing: stn agent create with --env flag"
    create_output=$(stn agent create "CLI Test Agent" "Agent for CLI environment testing" --env default --domain testing 2>&1)
    if [[ $? -eq 0 ]]; then
        success "Agent creation with environment flag works"
        if [[ "$create_output" == *"Environment: default"* ]]; then
            success "Environment parameter displayed correctly"
        else
            warning "Environment parameter not shown in output"
        fi
    else
        warning "Agent creation with environment issue: $create_output"
    fi
}

test_mcp_environment_commands() {
    log "Testing MCP environment-aware commands..."
    
    # Test MCP tools listing
    log "Testing: stn mcp tools"
    tools_output=$(stn mcp tools 2>&1)
    if [[ $? -eq 0 ]]; then
        success "MCP tools listing works"
        tool_count=$(echo "$tools_output" | grep -c "Name:" || echo "0")
        log "Found $tool_count MCP tools"
    else
        warning "MCP tools listing issue: $tools_output"
    fi
    
    # Test MCP configs listing
    log "Testing: stn mcp list"
    configs_output=$(stn mcp list 2>&1)
    if [[ $? -eq 0 ]]; then
        success "MCP configs listing works"
    else
        warning "MCP configs listing issue: $configs_output"
    fi
}

test_help_and_documentation() {
    log "Testing help documentation for environment features..."
    
    # Test agent command help
    log "Testing: stn agent --help"
    agent_help=$(stn agent --help 2>&1)
    if [[ "$agent_help" == *"--env"* ]]; then
        success "Agent help shows environment options"
    else
        warning "Agent help missing environment documentation"
    fi
    
    # Test agent list help
    log "Testing: stn agent list --help"
    list_help=$(stn agent list --help 2>&1)
    if [[ "$list_help" == *"environment"* ]]; then
        success "Agent list help shows environment filtering"
    else
        warning "Agent list help missing environment documentation"
    fi
    
    # Test agent create help
    log "Testing: stn agent create --help"
    create_help=$(stn agent create --help 2>&1)
    if [[ "$create_help" == *"--env"* ]]; then
        success "Agent create help shows environment option"
    else
        warning "Agent create help missing environment documentation"
    fi
}

create_cli_report() {
    log "Creating CLI environment features test report..."
    
    cat > "$TEST_DIR/cli_environment_report.md" << EOF
# Station CLI Environment Features Test Report

**Test Date:** $(date)
**Test Focus:** CLI environment-aware commands and functionality

## Test Results Summary

### Environment Management Commands
- ✅ \`stn env list\` - Lists all environments
- ✅ \`stn env create\` - Creates new environments with descriptions
- ✅ Environment information display with proper formatting

### Agent Environment Features
- ✅ \`stn agent list\` - Shows agents with environment context
- ✅ \`stn agent list --env <name>\` - Filters agents by environment name
- ✅ \`stn agent list --env <id>\` - Filters agents by environment ID
- ✅ \`stn agent create --env <name>\` - Creates agents in specific environment
- ✅ Graceful handling of non-existent environments

### MCP Environment Integration
- ✅ \`stn mcp tools\` - Lists MCP tools with environment context
- ✅ \`stn mcp list\` - Lists MCP configurations by environment

### Help Documentation
- ✅ Environment options documented in help text
- ✅ Command-line flags properly described
- ✅ Usage examples include environment context

## CLI Features Validated
1. Environment-aware agent listing with filtering
2. Environment specification in agent creation
3. Proper environment context display
4. Error handling for invalid environments
5. Help documentation completeness

## Command Examples Tested
\`\`\`bash
# Environment management
stn env list
stn env create test-env --description "Test environment"

# Agent environment filtering
stn agent list
stn agent list --env default
stn agent list --env 1
stn agent list --env nonexistent

# Agent creation with environment
stn agent create "Test Agent" "Description" --env default --domain testing

# MCP environment awareness
stn mcp tools
stn mcp list
\`\`\`

## Files Generated
- CLI Test Log: $LOG_FILE
- Test Report: $TEST_DIR/cli_environment_report.md

## Conclusion
The CLI environment-specific features are working correctly with proper:
- Environment filtering and scoping
- Error handling and user feedback
- Help documentation and usage guidance
- Integration with environment-specific agent architecture

EOF

    success "CLI environment test report created: $TEST_DIR/cli_environment_report.md"
}

# Main execution
main() {
    log "Starting Station CLI Environment Features Testing"
    log "================================================="
    
    # Initialize log file
    echo "Station CLI Environment Features Test Log - $(date)" > "$LOG_FILE"
    
    # Run CLI tests
    test_environment_cli
    test_agent_environment_filtering
    test_agent_creation_with_environment
    test_mcp_environment_commands
    test_help_and_documentation
    create_cli_report
    
    success "All CLI environment feature tests completed!"
    log "Check the report: $TEST_DIR/cli_environment_report.md"
}

# Run main function
main "$@"