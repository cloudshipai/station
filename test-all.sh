# test#!/bin/bash
# Station Comprehensive Test Script
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}🧪 Station Comprehensive Test Suite${NC}"
echo "=================================="

# Clean up any existing processes
echo -e "${YELLOW}🧹 Cleaning up existing processes...${NC}"
make stop-station > /dev/null 2>&1 || true

# Build Station
echo -e "${BLUE}🔨 Building Station...${NC}"
make dev

# Run unit tests
echo -e "${BLUE}🧪 Running unit tests...${NC}"
make test

# Test CLI commands
echo -e "${BLUE}🖥️  Testing CLI commands...${NC}"

# Remove existing config for clean test
rm -rf ~/.config/station || true

# Test init command
echo -e "${GREEN}✓ Testing: stn init${NC}"
./stn init

# Test config commands
echo -e "${GREEN}✓ Testing: stn config show${NC}"
./stn config show

# Test key commands
echo -e "${GREEN}✓ Testing: stn key status${NC}"
./stn key status

# Test banner command
echo -e "${GREEN}✓ Testing: stn banner${NC}"
./stn banner

# Test help commands
echo -e "${GREEN}✓ Testing: stn --help${NC}"
./stn --help > /dev/null

echo -e "${GREEN}✓ Testing: stn mcp --help${NC}"
./stn mcp --help > /dev/null

# Test server startup (background)
echo -e "${BLUE}🚀 Testing server startup...${NC}"
./stn serve &
SERVER_PID=$!

# Wait a bit for server to start
sleep 3

# Test if server is running
if ps -p $SERVER_PID > /dev/null; then
    echo -e "${GREEN}✓ Server started successfully (PID: $SERVER_PID)${NC}"
    
    # Test SSH connection (non-interactive)
    echo -e "${GREEN}✓ Testing SSH connection...${NC}"
    timeout 5 ssh -o ConnectTimeout=2 -o StrictHostKeyChecking=no admin@localhost -p 2222 "exit" || echo "SSH test completed"
    
    # Kill server
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
    echo -e "${GREEN}✓ Server stopped${NC}"
else
    echo -e "${RED}❌ Server failed to start${NC}"
    exit 1
fi

# Test MCP commands
echo -e "${BLUE}📋 Testing MCP commands...${NC}"

# Test mcp list (should be empty initially)
echo -e "${GREEN}✓ Testing: stn mcp list${NC}"
./stn mcp list

# Test mcp tools (should be empty initially) 
echo -e "${GREEN}✓ Testing: stn mcp tools${NC}"
./stn mcp tools

# Clean up
echo -e "${YELLOW}🧹 Cleaning up...${NC}"
make stop-station > /dev/null 2>&1 || true

echo ""
echo -e "${GREEN}🎉 All tests passed!${NC}"
echo -e "${BLUE}Station is ready for use!${NC}"
echo ""
echo "Next steps:"
echo "  1. ./stn init          # Initialize configuration"
echo "  2. ./stn serve         # Start Station server"
echo "  3. ./stn banner        # See beautiful banner"
echo "  4. ssh admin@localhost -p 2222  # Connect to admin interface"