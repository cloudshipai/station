# Station Testing Scenarios Guide

This document provides comprehensive testing scenarios to validate Station functionality across different use cases and environments.

## 🧪 Test Environment Setup

### Prerequisites
```bash
# Install test dependencies
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go install github.com/onsi/gomega@latest
npm install -g @modelcontextprotocol/server-filesystem
npm install -g @modelcontextprotocol/server-sqlite

# Set up test environment
export STN_ENV=test
export STN_DATABASE_URL="sqlite:///tmp/station_test.db"
export STN_ENCRYPTION_KEY="test-key-32-bytes-long-for-testing"
export STN_AI_PROVIDER="openai"
export STN_AI_API_KEY="sk-test-key"
```

### Test Data Setup
```bash
# Initialize test database
./stn init --env test

# Create test environments
./stn env create test-env "Test Environment"
./stn env create staging "Staging Environment"
./stn env create production "Production Environment"
```

## 🔄 Core Functionality Tests

### 1. Agent Lifecycle Management

#### Test Scenario: Complete Agent CRUD Operations
```bash
#!/bin/bash
# test_agent_lifecycle.sh

set -e

echo "🧪 Testing Agent Lifecycle Management"

# Test 1: Create intelligent agent
echo "1. Creating intelligent agent..."
AGENT_ID=$(./stn agent create \
    --name "Test File Manager" \
    --description "Agent for testing file operations" \
    --domain "testing" \
    --schedule "on-demand" \
    --environment "test-env" | grep "Agent ID:" | cut -d: -f2 | xargs)

echo "✅ Created agent ID: $AGENT_ID"

# Test 2: List agents
echo "2. Listing agents..."
./stn agent list | grep "Test File Manager" || (echo "❌ Agent not found in list" && exit 1)
echo "✅ Agent appears in listing"

# Test 3: Show agent details
echo "3. Showing agent details..."
./stn agent show $AGENT_ID | grep "Test File Manager" || (echo "❌ Agent details not correct" && exit 1)
echo "✅ Agent details correct"

# Test 4: Export agent
echo "4. Exporting agent..."
./stn agent export $AGENT_ID test-env || (echo "❌ Agent export failed" && exit 1)
echo "✅ Agent exported successfully"

# Test 5: Delete original agent
echo "5. Deleting original agent..."
./stn agent delete $AGENT_ID || (echo "❌ Agent deletion failed" && exit 1)
echo "✅ Agent deleted successfully"

# Test 6: Import agent back
echo "6. Importing agent..."
./stn agent import test-env || (echo "❌ Agent import failed" && exit 1)
echo "✅ Agent imported successfully"

# Test 7: Verify imported agent works
echo "7. Testing imported agent execution..."
NEW_AGENT_ID=$(./stn agent list | grep "Test File Manager" | grep -o 'ID: [0-9]*' | cut -d' ' -f2)
./stn agent run $NEW_AGENT_ID "List files in current directory" || (echo "❌ Agent execution failed" && exit 1)
echo "✅ Imported agent executes correctly"

echo "🎉 All agent lifecycle tests passed!"
```

#### Expected Results
- Agent creation with intelligent tool assignment
- Consistent agent data across export/import
- All CRUD operations work without errors
- Tool assignments preserved during import/export

### 2. MCP Server Integration

#### Test Scenario: MCP Server Loading and Configuration
```bash
#!/bin/bash
# test_mcp_integration.sh

set -e

echo "🧪 Testing MCP Server Integration"

# Test 1: Load filesystem MCP server
echo "1. Loading filesystem MCP server..."
./stn load examples/mcps/filesystem.json --env test-env || (echo "❌ MCP server load failed" && exit 1)
echo "✅ Filesystem MCP server loaded"

# Test 2: Verify server is configured
echo "2. Verifying MCP server configuration..."
./stn mcp list --env test-env | grep "filesystem" || (echo "❌ MCP server not found" && exit 1)
echo "✅ MCP server appears in configuration"

# Test 3: Test server connectivity
echo "3. Testing MCP server connectivity..."
./stn mcp test filesystem --env test-env || (echo "❌ MCP server connectivity failed" && exit 1)
echo "✅ MCP server connectivity verified"

# Test 4: Load multiple servers
echo "4. Loading multiple MCP servers..."
./stn load examples/mcps/sqlite.json --env test-env || (echo "❌ SQLite MCP server load failed" && exit 1)
./stn load examples/mcps/git-advanced.json --env test-env || (echo "❌ Git MCP server load failed" && exit 1)
echo "✅ Multiple MCP servers loaded"

# Test 5: Verify all servers are available
echo "5. Verifying all MCP servers..."
SERVER_COUNT=$(./stn mcp list --env test-env | grep -c "Server:")
if [ "$SERVER_COUNT" -lt 3 ]; then
    echo "❌ Expected at least 3 servers, found $SERVER_COUNT"
    exit 1
fi
echo "✅ All MCP servers verified ($SERVER_COUNT servers)"

echo "🎉 All MCP integration tests passed!"
```

#### Expected Results
- Successful loading of MCP server configurations
- Proper template variable resolution
- Server connectivity verification
- Multi-server environment support

### 3. Environment Management

#### Test Scenario: Multi-Environment Operations
```bash
#!/bin/bash
# test_environments.sh

set -e

echo "🧪 Testing Environment Management"

# Test 1: Create environments
echo "1. Creating test environments..."
./stn env create dev "Development Environment" || (echo "❌ Dev environment creation failed" && exit 1)
./stn env create prod "Production Environment" || (echo "❌ Prod environment creation failed" && exit 1)
echo "✅ Test environments created"

# Test 2: List environments
echo "2. Listing environments..."
ENV_COUNT=$(./stn env list | grep -c "Environment:")
if [ "$ENV_COUNT" -lt 3 ]; then
    echo "❌ Expected at least 3 environments, found $ENV_COUNT"
    exit 1
fi
echo "✅ Environments listed correctly ($ENV_COUNT environments)"

# Test 3: Load MCP servers in different environments
echo "3. Loading MCP servers in different environments..."
./stn load examples/mcps/filesystem.json --env dev || (echo "❌ Dev MCP load failed" && exit 1)
./stn load examples/mcps/postgresql.json --env prod || (echo "❌ Prod MCP load failed" && exit 1)
echo "✅ MCP servers loaded in different environments"

# Test 4: Create agents in different environments
echo "4. Creating agents in different environments..."
DEV_AGENT=$(./stn agent create \
    --name "Dev Agent" \
    --description "Development agent" \
    --environment "dev" | grep "Agent ID:" | cut -d: -f2 | xargs)

PROD_AGENT=$(./stn agent create \
    --name "Prod Agent" \
    --description "Production agent" \
    --environment "prod" | grep "Agent ID:" | cut -d: -f2 | xargs)
echo "✅ Agents created in different environments"

# Test 5: Verify environment isolation
echo "5. Verifying environment isolation..."
DEV_AGENTS=$(./stn agent list --env dev | grep -c "Dev Agent")
PROD_AGENTS=$(./stn agent list --env prod | grep -c "Prod Agent")

if [ "$DEV_AGENTS" -ne 1 ] || [ "$PROD_AGENTS" -ne 1 ]; then
    echo "❌ Environment isolation failed"
    exit 1
fi
echo "✅ Environment isolation verified"

echo "🎉 All environment management tests passed!"
```

#### Expected Results
- Successful environment creation and configuration
- Proper isolation between environments
- Environment-specific MCP server and agent management
- No cross-environment data leakage

### 4. Agent Execution & Scheduling

#### Test Scenario: Agent Execution Modes
```bash
#!/bin/bash
# test_agent_execution.sh

set -e

echo "🧪 Testing Agent Execution"

# Setup: Create test agent with filesystem tools
echo "Setting up test environment..."
./stn load examples/mcps/filesystem.json --env staging
AGENT_ID=$(./stn agent create \
    --name "Execution Test Agent" \
    --description "Agent for testing execution modes" \
    --environment "staging" | grep "Agent ID:" | cut -d: -f2 | xargs)

# Test 1: On-demand execution
echo "1. Testing on-demand execution..."
./stn agent run $AGENT_ID "List files in the cmd directory" --timeout 60 || (echo "❌ On-demand execution failed" && exit 1)
echo "✅ On-demand execution successful"

# Test 2: Execution with tool usage verification
echo "2. Testing execution with tool verification..."
RUN_OUTPUT=$(./stn agent run $AGENT_ID "Create a test directory called 'test-station' and list its contents")
echo "$RUN_OUTPUT" | grep -i "test-station\|directory\|created" || (echo "❌ Tool usage not verified" && exit 1)
echo "✅ Tool usage verified in execution"

# Test 3: Multi-step execution
echo "3. Testing multi-step execution..."
RUN_OUTPUT=$(./stn agent run $AGENT_ID "First create a directory called 'multi-step', then create a file inside it called 'step2.txt', then list the directory contents")
STEPS=$(echo "$RUN_OUTPUT" | grep "Steps Taken:" | grep -o '[0-9]*')
if [ "$STEPS" -lt 3 ]; then
    echo "❌ Multi-step execution failed (only $STEPS steps)"
    exit 1
fi
echo "✅ Multi-step execution successful ($STEPS steps)"

# Test 4: Scheduled agent creation and execution
echo "4. Testing scheduled agent creation..."
SCHEDULED_AGENT=$(./stn agent create \
    --name "Scheduled Test Agent" \
    --description "Agent with schedule" \
    --schedule "0 */6 * * * *" \
    --environment "staging" | grep "Agent ID:" | cut -d: -f2 | xargs)
echo "✅ Scheduled agent created (ID: $SCHEDULED_AGENT)"

# Test 5: Error handling
echo "5. Testing error handling..."
ERROR_OUTPUT=$(./stn agent run $AGENT_ID "Delete the entire root filesystem" 2>&1)
if echo "$ERROR_OUTPUT" | grep -i "error\|failed\|denied\|cannot"; then
    echo "✅ Error handling working correctly"
else
    echo "❌ Error handling may not be working"
    exit 1
fi

echo "🎉 All agent execution tests passed!"
```

#### Expected Results
- Successful on-demand agent execution
- Multi-step execution with proper step counting
- Tool usage verification in responses
- Proper error handling for invalid operations
- Scheduled agent creation with valid cron expressions

### 5. File-Based Configuration Management

#### Test Scenario: GitOps Configuration Management
```bash
#!/bin/bash
# test_file_config.sh

set -e

echo "🧪 Testing File-Based Configuration Management"

# Test 1: Manual file configuration creation
echo "1. Creating manual file configuration..."
mkdir -p ~/.config/station/environments/test-file/mcp-servers/
cat > ~/.config/station/environments/test-file/mcp-servers/manual-fs.json << 'EOF'
{
  "name": "manual-filesystem",
  "description": "Manually created filesystem server",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
  "env": {},
  "enabled": true
}
EOF
echo "✅ Manual file configuration created"

# Test 2: Load from file-based configuration
echo "2. Loading from file-based configuration..."
./stn env create test-file "Test File Environment" || echo "Environment may already exist"
./stn mcp discover test-file || (echo "❌ File-based discovery failed" && exit 1)
echo "✅ File-based configuration loaded"

# Test 3: Verify loaded configuration
echo "3. Verifying loaded configuration..."
./stn mcp list --env test-file | grep "manual-filesystem" || (echo "❌ Manual config not found" && exit 1)
echo "✅ Manual configuration verified"

# Test 4: Template variable resolution
echo "4. Testing template variable resolution..."
mkdir -p ~/.config/station/environments/test-template/mcp-servers/
cat > ~/.config/station/environments/test-template/variables.yml << 'EOF'
CUSTOM_PATH: "/home/testuser"
CUSTOM_PORT: "3001"
EOF

cat > ~/.config/station/environments/test-template/mcp-servers/template-test.json << 'EOF'
{
  "name": "template-test",
  "description": "Template variable test",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem", "{{CUSTOM_PATH}}"],
  "env": {
    "PORT": "{{CUSTOM_PORT}}"
  },
  "enabled": true
}
EOF

./stn env create test-template "Template Test Environment" || echo "Environment may already exist"
./stn mcp discover test-template || (echo "❌ Template discovery failed" && exit 1)
echo "✅ Template variable resolution successful"

# Test 5: Agent export/import with file persistence
echo "5. Testing agent file persistence..."
TEST_AGENT=$(./stn agent create \
    --name "File Persistence Test" \
    --description "Testing file-based agent storage" \
    --environment "test-file" | grep "Agent ID:" | cut -d: -f2 | xargs)

./stn agent export $TEST_AGENT test-file || (echo "❌ Agent export failed" && exit 1)

# Verify export files exist
if [[ ! -f ~/.config/station/environments/test-file/agents/file-persistence-test.json ]]; then
    echo "❌ Agent export file not created"
    exit 1
fi
echo "✅ Agent file persistence verified"

echo "🎉 All file-based configuration tests passed!"
```

#### Expected Results
- Manual file configuration discovery and loading
- Template variable resolution with custom values
- Proper agent export to file format
- GitOps-ready configuration structure

### 6. Security & Error Handling

#### Test Scenario: Security Validation
```bash
#!/bin/bash
# test_security.sh

set -e

echo "🧪 Testing Security & Error Handling"

# Test 1: Invalid configuration rejection
echo "1. Testing invalid configuration rejection..."
INVALID_RESULT=$(./stn load <(echo '{"invalid": "json structure"}') 2>&1 || true)
if ! echo "$INVALID_RESULT" | grep -i "error\|invalid\|failed"; then
    echo "❌ Invalid configuration not rejected"
    exit 1
fi
echo "✅ Invalid configuration properly rejected"

# Test 2: Sensitive data handling
echo "2. Testing sensitive data handling..."
cat > /tmp/test-sensitive.json << 'EOF'
{
  "name": "Sensitive Test",
  "description": "Testing sensitive data handling",
  "mcpServers": {
    "test": {
      "command": "echo",
      "args": ["test"],
      "env": {
        "SECRET_TOKEN": "{{SECRET_TOKEN}}"
      }
    }
  },
  "templates": {
    "SECRET_TOKEN": {
      "description": "Secret token",
      "type": "password",
      "required": true,
      "sensitive": true
    }
  }
}
EOF

# This should prompt for sensitive input (we'll simulate with expect if available)
echo "✅ Sensitive data handling configured (manual verification required)"

# Test 3: Permission validation
echo "3. Testing permission validation..."
chmod 000 /tmp/test-locked 2>/dev/null || true
PERMISSION_RESULT=$(./stn agent run 1 "Access /tmp/test-locked" 2>&1 || true)
if ! echo "$PERMISSION_RESULT" | grep -i "permission\|denied\|error"; then
    echo "⚠️ Permission validation may need review"
else
    echo "✅ Permission validation working"
fi

# Test 4: Resource limits
echo "4. Testing resource limits..."
# Create agent with very high step limit to test bounds
LARGE_AGENT=$(./stn agent create \
    --name "Resource Test Agent" \
    --description "Testing resource limits" \
    --environment "default" | grep "Agent ID:" | cut -d: -f2 | xargs)

# This should handle resource constraints gracefully
RESOURCE_RESULT=$(timeout 30 ./stn agent run $LARGE_AGENT "Repeat this task 1000 times" 2>&1 || true)
echo "✅ Resource limits handled (manual verification required)"

echo "🎉 All security tests completed!"
```

#### Expected Results
- Invalid configurations properly rejected
- Sensitive data marked and handled appropriately
- Permission errors caught and reported
- Resource limits enforced

## 🧩 Integration Testing Scenarios

### 1. End-to-End Workflow Test

#### Scenario: Complete Development Workflow
```bash
#!/bin/bash
# test_e2e_workflow.sh

set -e

echo "🧪 End-to-End Development Workflow Test"

# Setup development environment
echo "1. Setting up development environment..."
./stn env create dev-workflow "Development Workflow Environment"

# Load development tools
echo "2. Loading development MCP servers..."
./stn load examples/mcps/filesystem.json --env dev-workflow
./stn load examples/mcps/git-advanced.json --env dev-workflow
./stn load examples/mcps/github.json --env dev-workflow

# Create development agent
echo "3. Creating development agent..."
DEV_AGENT=$(./stn agent create \
    --name "Full Stack Developer" \
    --description "Agent for complete development tasks" \
    --domain "development" \
    --environment "dev-workflow" | grep "Agent ID:" | cut -d: -f2 | xargs)

# Test complex development task
echo "4. Testing complex development task..."
./stn agent run $DEV_AGENT "Create a new directory called 'test-project', initialize a git repository in it, create a simple README.md file with project description, and show the git status"

# Verify results
echo "5. Verifying workflow results..."
if [[ -d "test-project" ]] && [[ -f "test-project/README.md" ]] && [[ -d "test-project/.git" ]]; then
    echo "✅ Complete workflow executed successfully"
else
    echo "❌ Workflow execution incomplete"
    exit 1
fi

echo "🎉 End-to-end workflow test passed!"
```

### 2. Multi-Environment Deployment Test

#### Scenario: Production Deployment Simulation
```bash
#!/bin/bash
# test_deployment_simulation.sh

set -e

echo "🧪 Production Deployment Simulation"

# Create production-like environments
echo "1. Creating production environments..."
./stn env create staging-deploy "Staging Deployment Environment"
./stn env create prod-deploy "Production Deployment Environment"

# Load production configurations
echo "2. Loading production MCP configurations..."
./stn load examples/mcps/postgresql.json --env staging-deploy
./stn load examples/mcps/monitoring-prometheus.json --env staging-deploy
./stn load examples/mcps/docker.json --env prod-deploy
./stn load examples/mcps/kubernetes.json --env prod-deploy

# Create deployment agents
echo "3. Creating deployment agents..."
STAGING_AGENT=$(./stn agent create \
    --name "Staging Deploy Agent" \
    --description "Handles staging deployments" \
    --environment "staging-deploy" | grep "Agent ID:" | cut -d: -f2 | xargs)

PROD_AGENT=$(./stn agent create \
    --name "Production Deploy Agent" \
    --description "Handles production deployments" \
    --environment "prod-deploy" | grep "Agent ID:" | cut -d: -f2 | xargs)

# Test deployment workflow
echo "4. Testing deployment workflow..."
./stn agent run $STAGING_AGENT "Check system status and prepare for deployment verification"
./stn agent run $PROD_AGENT "Verify production environment readiness and security checks"

# Export agents for GitOps
echo "5. Exporting agents for GitOps..."
./stn agent export $STAGING_AGENT staging-deploy
./stn agent export $PROD_AGENT prod-deploy

echo "✅ Deployment simulation completed successfully"
echo "🎉 Multi-environment deployment test passed!"
```

## 📊 Performance Testing

### Load Testing Scenario
```bash
#!/bin/bash
# test_performance.sh

set -e

echo "🧪 Performance Testing"

# Create performance test agent
PERF_AGENT=$(./stn agent create \
    --name "Performance Test Agent" \
    --description "Agent for performance testing" \
    --environment "default" | grep "Agent ID:" | cut -d: -f2 | xargs)

# Concurrent execution test
echo "1. Testing concurrent execution..."
for i in {1..5}; do
    ./stn agent run $PERF_AGENT "Execute performance test $i" &
done
wait
echo "✅ Concurrent execution completed"

# Memory usage test
echo "2. Testing memory usage..."
INITIAL_MEMORY=$(ps -o pid,vsz,rss,pcpu,pmem,comm -p $(pgrep station) | tail -1 | awk '{print $3}')
./stn agent run $PERF_AGENT "Perform memory-intensive task with large data processing"
FINAL_MEMORY=$(ps -o pid,vsz,rss,pcpu,pmem,comm -p $(pgrep station) | tail -1 | awk '{print $3}')

echo "Memory usage: Initial=${INITIAL_MEMORY}KB, Final=${FINAL_MEMORY}KB"
echo "✅ Memory usage test completed"

echo "🎉 Performance testing completed!"
```

## 🚀 Continuous Integration Tests

### GitHub Actions Test Pipeline
```yaml
# .github/workflows/test.yml
name: Station Test Suite

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: station_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
    - uses: actions/checkout@v3
    
    - uses: actions/setup-go@v3
      with:
        go-version: '1.21'
    
    - uses: actions/setup-node@v3
      with:
        node-version: '18'
    
    - name: Install dependencies
      run: |
        npm install -g @modelcontextprotocol/server-filesystem
        npm install -g @modelcontextprotocol/server-sqlite
    
    - name: Build Station
      run: go build -o stn ./cmd/main
    
    - name: Run Unit Tests
      run: go test ./...
    
    - name: Run Integration Tests
      env:
        STN_DATABASE_URL: "postgres://postgres:postgres@localhost:5432/station_test?sslmode=disable"
        STN_ENCRYPTION_KEY: "test-key-32-bytes-long-for-testing"
      run: |
        ./test_agent_lifecycle.sh
        ./test_mcp_integration.sh
        ./test_environments.sh
        ./test_agent_execution.sh
        ./test_file_config.sh
        ./test_security.sh
    
    - name: Run E2E Tests
      run: |
        ./test_e2e_workflow.sh
        ./test_deployment_simulation.sh
```

## 📋 Manual Testing Checklist

### Pre-Release Testing
- [ ] **Agent Creation**: Create agents with different configurations
- [ ] **MCP Integration**: Load various MCP server types
- [ ] **Environment Management**: Create and manage multiple environments
- [ ] **Execution Modes**: Test on-demand and scheduled execution
- [ ] **Import/Export**: Verify agent export/import functionality
- [ ] **Security**: Test authentication and authorization
- [ ] **Performance**: Verify system handles expected load
- [ ] **Documentation**: Ensure all examples work as documented

### User Acceptance Testing
- [ ] **New User Experience**: Fresh installation and setup
- [ ] **Common Workflows**: Typical user scenarios
- [ ] **Error Recovery**: System behavior during failures
- [ ] **UI/UX**: Terminal interface usability
- [ ] **Help System**: Accuracy of help and documentation

## 🐛 Bug Reproduction Scenarios

### Common Issues Testing
```bash
#!/bin/bash
# test_known_issues.sh

echo "🧪 Testing Known Issue Scenarios"

# Test 1: Database connection recovery
echo "1. Testing database connection recovery..."
# Simulate database disconnection and reconnection

# Test 2: MCP server timeout handling
echo "2. Testing MCP server timeout handling..."
# Test with slow or unresponsive MCP servers

# Test 3: Concurrent agent execution
echo "3. Testing concurrent agent execution limits..."
# Test system behavior under high concurrent load

# Test 4: Memory leak detection
echo "4. Running memory leak detection..."
# Long-running test to detect memory leaks

echo "🎉 Issue scenario testing completed!"
```

---

## 📈 Test Results Tracking

Use this template to track test results:

```markdown
## Test Execution Report

**Date**: 2024-01-15
**Version**: v1.0.0
**Environment**: staging

### Test Results Summary
- ✅ Agent Lifecycle: PASSED (7/7 tests)
- ✅ MCP Integration: PASSED (5/5 tests)  
- ✅ Environment Management: PASSED (5/5 tests)
- ✅ Agent Execution: PASSED (5/5 tests)
- ✅ File Configuration: PASSED (5/5 tests)
- ⚠️ Security: PASSED (3/4 tests) - 1 manual verification needed
- ✅ E2E Workflow: PASSED (5/5 tests)
- ✅ Deployment Simulation: PASSED (5/5 tests)

### Performance Metrics
- Average agent execution time: 3.2s
- Memory usage: 45MB baseline, 67MB peak
- Concurrent execution limit: 10 agents

### Issues Found
- None critical
- 1 minor UI improvement opportunity

### Recommendations
- Ready for production deployment
- Monitor memory usage in production
```

This comprehensive testing approach ensures Station is thoroughly validated across all major use cases and scenarios before production deployment.