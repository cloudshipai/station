#!/bin/bash
# Station Faker PRD Scenarios - Multi-Agent Hierarchy Testing
# Tests easy, medium, and hard DevOps scenarios with faker tools

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🎯 Station Faker PRD Scenarios"
echo "=============================="
echo ""
echo "Testing multi-agent hierarchies with faker tools across three complexity levels:"
echo "  1. EASY: Infrastructure Health Check (2 agents, 2 fakers)"
echo "  2. MEDIUM: Cost Spike Investigation (4 agents, 3 fakers)"
echo "  3. HARD: Critical Incident Response (5 agents, 4 fakers)"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Track results
TOTAL_TESTS=3
PASSED_TESTS=0
FAILED_TESTS=0

# Function to run a scenario
run_scenario() {
    local scenario_name=$1
    local env_name=$2
    local agent_name=$3
    local task=$4
    local difficulty=$5
    
    echo ""
    echo "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo "${YELLOW}Scenario: ${scenario_name} [${difficulty}]${NC}"
    echo "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    
    # Sync environment
    echo "📋 Step 1: Syncing environment '${env_name}'..."
    if stn sync "$env_name" --no-interactive 2>&1 | grep -q "All configurations synced successfully"; then
        echo "${GREEN}✓${NC} Environment synced successfully"
    else
        echo "${RED}✗${NC} Environment sync failed"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
    
    # Run agent with OTEL
    echo ""
    echo "🤖 Step 2: Running agent '${agent_name}'..."
    echo "   Task: ${task}"
    echo ""
    
    RUN_OUTPUT=$(stn agent run "$agent_name" "$task" --env "$env_name" --enable-telemetry --otel-endpoint http://localhost:4318 2>&1)
    RUN_ID=$(echo "$RUN_OUTPUT" | grep "Run ID:" | awk '{print $3}')
    STATUS=$(echo "$RUN_OUTPUT" | grep "Status:" | awk '{print $2}')
    
    echo "$RUN_OUTPUT"
    echo ""
    
    # Check if completed
    if [ "$STATUS" = "completed" ]; then
        echo "${GREEN}✓${NC} Agent completed successfully"
        echo "   Run ID: $RUN_ID"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo "${RED}✗${NC} Agent failed with status: $STATUS"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi
    
    # Show faker sessions
    echo ""
    echo "📊 Step 3: Checking faker sessions..."
    SESSIONS=$(stn faker sessions list 2>&1 | grep -c "Session" || echo "0")
    echo "   Faker sessions created: $SESSIONS"
    
    # Show Jaeger trace link
    echo ""
    echo "🔭 Step 4: OpenTelemetry trace available:"
    echo "   View in Jaeger: ${GREEN}http://localhost:16686${NC}"
    echo "   Service: station"
    echo "   Run ID: $RUN_ID"
    
    echo ""
    echo "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo "${GREEN}✓ Scenario ${scenario_name} PASSED${NC}"
    echo "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    return 0
}

# Scenario 1: EASY - Infrastructure Health Check
run_scenario \
    "Infrastructure Health Check" \
    "scenario-easy-health-check" \
    "Health Check Orchestrator" \
    "Perform a comprehensive infrastructure health check across all systems" \
    "EASY"

sleep 3

# Scenario 2: MEDIUM - Cost Spike Investigation
run_scenario \
    "Cost Spike Investigation" \
    "scenario-medium-cost-investigation" \
    "Cost Investigation Orchestrator" \
    "Investigate the AWS cost spike: identify what caused the 300% cost increase, who made the changes, and provide remediation steps" \
    "MEDIUM"

sleep 3

# Scenario 3: HARD - Critical Incident Response
run_scenario \
    "Critical Incident Response" \
    "scenario-hard-incident-response" \
    "Incident Commander" \
    "Production incident: API is returning 45% errors, database connections exhausted, ECS tasks crashing. Investigate root cause, determine if related to recent deployment, and provide immediate mitigation plan" \
    "HARD"

# Summary
echo ""
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 TEST SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Total Scenarios: $TOTAL_TESTS"
echo "${GREEN}Passed: $PASSED_TESTS${NC}"
if [ $FAILED_TESTS -gt 0 ]; then
    echo "${RED}Failed: $FAILED_TESTS${NC}"
else
    echo "Failed: $FAILED_TESTS"
fi
echo ""

if [ $FAILED_TESTS -eq 0 ]; then
    echo "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo "${GREEN}🎉 ALL PRD SCENARIOS PASSED! 🎉${NC}"
    echo "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "✅ Multi-agent hierarchies with faker tools validated across:"
    echo "   - Easy: 2-agent health monitoring"
    echo "   - Medium: 4-agent cost investigation"
    echo "   - Hard: 5-agent incident response"
    echo ""
    echo "🔭 View all traces in Jaeger: http://localhost:16686"
    exit 0
else
    echo "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo "${RED}❌ SOME SCENARIOS FAILED${NC}"
    echo "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    exit 1
fi
