#!/bin/bash
# Interactive demo commands for Station Lattice
# Run this in Terminal 4 after starting all stations

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATION_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
STN="$STATION_ROOT/stn"
NATS_URL="nats://localhost:4222"
STN_LATTICE="$STN lattice --nats $NATS_URL"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

print_header() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

print_command() {
    echo -e "${YELLOW}$ $1${NC}"
}

wait_for_enter() {
    echo ""
    echo -e "${GREEN}Press ENTER to continue...${NC}"
    read -r
}

run_command() {
    print_command "$1"
    echo ""
    eval "$1"
}

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║         STATION LATTICE DEMO - INTERACTIVE COMMANDS          ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  This script walks through the lattice demo commands         ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check that stn binary exists
if [ ! -f "$STN" ]; then
    echo -e "${RED}Error: stn binary not found at $STN${NC}"
    echo "Build it first with: go build -o stn ./cmd/main"
    exit 1
fi

# Check NATS connection
echo "Checking connection to lattice..."
if ! nc -z localhost 4222 2>/dev/null; then
    echo -e "${RED}Error: Cannot connect to NATS on localhost:4222${NC}"
    echo "Make sure the orchestrator is running (./01-start-orchestrator.sh)"
    exit 1
fi
echo -e "${GREEN}Connected to NATS!${NC}"

wait_for_enter

# ==============================================================================
# DEMO 1: Lattice Status
# ==============================================================================
print_header "DEMO 1: Check Lattice Status"

echo "First, let's check the overall status of our lattice mesh."
echo "This shows connected stations, NATS status, and agent counts."
echo ""

run_command "$STN_LATTICE status"

wait_for_enter

# ==============================================================================
# DEMO 2: Agent Discovery
# ==============================================================================
print_header "DEMO 2: Discover All Agents"

echo "Now let's discover all agents available across the lattice."
echo "This queries all connected stations and lists their agents."
echo ""

run_command "$STN_LATTICE agents --discover"

wait_for_enter

# ==============================================================================
# DEMO 3: Direct Agent Execution
# ==============================================================================
print_header "DEMO 3: Direct Agent Execution"

echo "We can execute a specific agent directly."
echo "Let's ask the K8sHealthChecker to check our cluster."
echo ""

run_command "$STN_LATTICE agent exec K8sHealthChecker 'Check the health of pods in the production namespace'"

wait_for_enter

# ==============================================================================
# DEMO 4: Async Work Assignment
# ==============================================================================
print_header "DEMO 4: Async Work Assignment"

echo "For long-running tasks, we can assign work asynchronously."
echo "This returns a work_id immediately while the work runs in background."
echo ""

print_command "$STN_LATTICE work assign VulnScanner 'Scan the application at /app for vulnerabilities'"
echo ""

WORK_OUTPUT=$($STN_LATTICE work assign VulnScanner "Scan the application at /app for vulnerabilities" 2>&1)
echo "$WORK_OUTPUT"

# Extract work_id from output (assumes format like "Work ID: abc123")
WORK_ID=$(echo "$WORK_OUTPUT" | grep -oP 'Work ID: \K[a-zA-Z0-9-]+' || echo "")

if [ -n "$WORK_ID" ]; then
    echo ""
    echo -e "${BLUE}Work assigned! ID: $WORK_ID${NC}"
    echo ""
    echo "Now let's check on the work status..."
    wait_for_enter
    
    run_command "$STN_LATTICE work check $WORK_ID"
    
    echo ""
    echo "And wait for the result..."
    wait_for_enter
    
    run_command "$STN_LATTICE work await $WORK_ID --timeout 60s"
else
    echo ""
    echo -e "${YELLOW}Note: Could not extract work_id from output.${NC}"
    echo "In a real demo, you would use: stn lattice work await <work_id>"
fi

wait_for_enter

# ==============================================================================
# DEMO 5: Distributed Orchestration (THE MAIN EVENT)
# ==============================================================================
print_header "DEMO 5: Distributed Orchestration (THE MAIN EVENT!)"

echo "This is where it gets exciting!"
echo ""
echo "We'll submit a complex task to the lattice. The Coordinator agent will:"
echo "  1. Analyze the task"
echo "  2. Discover available specialized agents"
echo "  3. Decompose the task into subtasks"
echo "  4. Assign work to multiple agents IN PARALLEL"
echo "  5. Collect results and synthesize a response"
echo ""
echo "Watch the other terminal windows to see agents activating!"
echo ""

wait_for_enter

echo -e "${CYAN}Submitting task to lattice...${NC}"
echo ""

run_command "$STN_LATTICE run 'Perform a comprehensive security and health assessment of our infrastructure. Check Kubernetes pod health, scan for vulnerabilities, and audit our network configuration.'"

wait_for_enter

# ==============================================================================
# DEMO 6: TUI Dashboard
# ==============================================================================
print_header "DEMO 6: Real-Time Dashboard"

echo "For continuous monitoring, launch the TUI dashboard."
echo "It shows live work status, completions, and station health."
echo ""
echo "Launch it with:"
echo -e "${YELLOW}$ $STN_LATTICE dashboard${NC}"
echo ""
echo "(Press 'q' to exit the dashboard)"
echo ""

echo -e "${GREEN}Want to launch the dashboard? (y/n)${NC}"
read -r LAUNCH_DASHBOARD

if [[ "$LAUNCH_DASHBOARD" == "y" || "$LAUNCH_DASHBOARD" == "Y" ]]; then
    run_command "$STN_LATTICE dashboard"
fi

# ==============================================================================
# CONCLUSION
# ==============================================================================
print_header "Demo Complete!"

echo "You've seen the Station Lattice in action:"
echo ""
echo "  [x] Lattice status and connectivity"
echo "  [x] Agent discovery across stations"
echo "  [x] Direct agent execution"
echo "  [x] Async work assignment and polling"
echo "  [x] Distributed orchestration with parallel execution"
echo "  [x] Real-time dashboard monitoring"
echo ""
echo "Key commands to remember:"
echo ""
echo -e "  ${YELLOW}stn lattice status${NC}              - Check lattice connectivity"
echo -e "  ${YELLOW}stn lattice agents --discover${NC}   - Discover all agents"
echo -e "  ${YELLOW}stn lattice agent exec${NC}          - Execute specific agent"
echo -e "  ${YELLOW}stn lattice work assign/await${NC}   - Async work operations"
echo -e "  ${YELLOW}stn lattice run${NC}                 - Submit to orchestrator"
echo -e "  ${YELLOW}stn lattice dashboard${NC}           - Real-time TUI"
echo ""
echo "To clean up, run: ./cleanup.sh"
echo ""
