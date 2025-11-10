#!/bin/bash
# Comprehensive E2E Testing for Station Faker with OpenTelemetry
# Tests: Single faker, Dual faker, Multi-agent hierarchy with faker

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4318"
export GENKIT_ENV="prod"

echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  Station Faker E2E Testing Suite with OpenTelemetry${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo ""
echo "📊 OTEL Endpoint: $OTEL_EXPORTER_OTLP_ENDPOINT"
echo "🔭 Jaeger UI: http://localhost:16686"
echo ""

# Function to check if Jaeger is running
check_jaeger() {
    if ! curl -s http://localhost:16686/api/services > /dev/null 2>&1; then
        echo -e "${RED}❌ Jaeger is not running!${NC}"
        echo "Start Jaeger with: docker compose -f docker-compose.otel.yml up -d"
        exit 1
    fi
    echo -e "${GREEN}✅ Jaeger is running${NC}"
}

# Function to sync environment
sync_env() {
    local env_name=$1
    echo -e "${YELLOW}🔄 Syncing environment: $env_name${NC}"
    stn sync "$env_name" -i=false || {
        echo -e "${RED}❌ Failed to sync $env_name${NC}"
        return 1
    }
    echo -e "${GREEN}✅ Synced $env_name${NC}"
}

# Function to run agent and capture run ID
run_agent() {
    local agent_name=$1
    local task=$2
    echo -e "${YELLOW}🤖 Running agent: $agent_name${NC}"
    echo "📝 Task: $task"
    
    stn agent run "$agent_name" "$task" 2>&1 | tee /tmp/agent_run.log
    
    # Extract run ID from output
    local run_id=$(grep "Run ID:" /tmp/agent_run.log | awk '{print $3}')
    echo "$run_id"
}

# Function to check faker sessions
check_faker_sessions() {
    echo -e "${YELLOW}📊 Checking faker sessions...${NC}"
    stn faker sessions list | tail -10
}

# Function to check Jaeger traces
check_traces() {
    local test_name=$1
    echo -e "${YELLOW}🔭 Checking Jaeger traces for: $test_name${NC}"
    
    # Wait a bit for traces to be exported
    sleep 3
    
    # Query Jaeger for station.faker spans
    local traces=$(curl -s "http://localhost:16686/api/traces?service=station&lookback=5m&limit=20" | python3 -m json.tool 2>/dev/null)
    
    local faker_spans=$(echo "$traces" | grep -c "station.faker" || echo "0")
    
    if [ "$faker_spans" -gt 0 ]; then
        echo -e "${GREEN}✅ Found $faker_spans station.faker spans in Jaeger${NC}"
    else
        echo -e "${RED}⚠️  No station.faker spans found yet${NC}"
    fi
    
    # Show trace count
    local trace_count=$(echo "$traces" | grep -c "traceID" || echo "0")
    echo "📈 Total traces in last 5 minutes: $trace_count"
}

# Main test execution
main() {
    check_jaeger
    echo ""
    
    # ================================================================
    # TEST 1: Single Agent with One Faker Tool
    # ================================================================
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  TEST 1: Single Agent with ONE Faker MCP Tool${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    
    sync_env "faker-test-1-single"
    
    run_id_1=$(run_agent "CloudWatch Analyzer" "Analyze the CloudWatch logs and tell me if there are any CPU performance issues")
    
    echo -e "\n${GREEN}✅ Test 1 Complete - Run ID: $run_id_1${NC}"
    check_traces "Test 1 - Single Faker"
    
    echo ""
    read -p "Press Enter to continue to Test 2..."
    
    # ================================================================
    # TEST 2: Single Agent with Two Faker Tools
    # ================================================================
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  TEST 2: Single Agent with TWO Faker MCP Tools${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    
    sync_env "faker-test-2-dual"
    
    run_id_2=$(run_agent "AWS Incident Investigator" "Investigate the recent Lambda service incident - check both CloudWatch logs and cost data to determine what happened and the financial impact")
    
    echo -e "\n${GREEN}✅ Test 2 Complete - Run ID: $run_id_2${NC}"
    check_traces "Test 2 - Dual Faker"
    
    echo ""
    read -p "Press Enter to continue to Test 3..."
    
    # ================================================================
    # TEST 3: Multi-Agent Hierarchy with Faker Tools
    # ================================================================
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  TEST 3: Multi-Agent Hierarchy with Faker Tools${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    
    sync_env "faker-test-3-hierarchy"
    
    run_id_3=$(run_agent "AWS Investigation Orchestrator" "Conduct a comprehensive investigation of the RDS performance and cost issues - coordinate with all specialist agents to get metrics, cost, and audit data")
    
    echo -e "\n${GREEN}✅ Test 3 Complete - Run ID: $run_id_3${NC}"
    check_traces "Test 3 - Multi-Agent Hierarchy"
    
    # ================================================================
    # Summary
    # ================================================================
    echo -e "\n${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  E2E Test Suite Complete!${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    
    echo -e "\n${GREEN}📊 Test Results:${NC}"
    echo "  Test 1 (Single Faker):      Run ID $run_id_1"
    echo "  Test 2 (Dual Faker):        Run ID $run_id_2"
    echo "  Test 3 (Multi-Agent):       Run ID $run_id_3"
    
    echo -e "\n${YELLOW}🔍 Next Steps:${NC}"
    echo "  1. View traces in Jaeger: http://localhost:16686"
    echo "  2. Search for service: 'station'"
    echo "  3. Look for operations containing 'station.faker'"
    echo "  4. Inspect runs: stn runs inspect <run-id> -v"
    echo "  5. Check faker sessions: stn faker sessions list"
    
    check_faker_sessions
}

main "$@"
