#!/bin/bash
# Station CLI Deployment Script
# Environment: {{.EnvironmentName}}
# Docker Image: {{.DockerImage}}

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Station CLI Deployment${NC}"
echo -e "${BLUE}Environment: {{.EnvironmentName}}${NC}"
echo -e "${BLUE}Docker Image: {{.DockerImage}}${NC}"
echo ""

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker is not installed. Please install Docker first.${NC}"
    exit 1
fi

# Pull the Station Docker image
echo -e "${YELLOW}📦 Pulling Station Docker image...${NC}"
docker pull {{.DockerImage}}

# Stop and remove existing container if it exists
if docker ps -a | grep -q station-{{.EnvironmentName}}; then
    echo -e "${YELLOW}🛑 Stopping existing container...${NC}"
    docker stop station-{{.EnvironmentName}} || true
    docker rm station-{{.EnvironmentName}} || true
fi

# Start Station container
echo -e "${YELLOW}🏃 Starting Station container...${NC}"
docker run -d \
  --name station-{{.EnvironmentName}} \
  -p 8585:8585 \
  -e OPENAI_API_KEY="${OPENAI_API_KEY}" \
  -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
  -v "$(pwd)":/workspace \
  {{.DockerImage}}

# Wait for Station to be ready
echo -e "${YELLOW}⏳ Waiting for Station to start...${NC}"
timeout 30 sh -c 'until docker exec station-{{.EnvironmentName}} stn status > /dev/null 2>&1; do sleep 1; done' || {
    echo -e "${RED}❌ Station failed to start within 30 seconds${NC}"
    docker logs station-{{.EnvironmentName}}
    exit 1
}

echo -e "${GREEN}✅ Station is ready!${NC}"
echo ""

# List available agents
echo -e "${BLUE}🤖 Available Agents:${NC}"
docker exec station-{{.EnvironmentName}} stn agent list --env {{.EnvironmentName}}
echo ""

# Print usage instructions
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}Station CLI Usage${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "${BLUE}Run an agent:${NC}"
echo -e "  docker exec station-{{.EnvironmentName}} stn agent run \"Agent Name\" \"Task description\" --env {{.EnvironmentName}} --tail"
echo ""
echo -e "${BLUE}List agents:${NC}"
echo -e "  docker exec station-{{.EnvironmentName}} stn agent list --env {{.EnvironmentName}}"
echo ""
echo -e "${BLUE}Check Station status:${NC}"
echo -e "  docker exec station-{{.EnvironmentName}} stn status"
echo ""
echo -e "${BLUE}View logs:${NC}"
echo -e "  docker logs -f station-{{.EnvironmentName}}"
echo ""
echo -e "${BLUE}Stop Station:${NC}"
echo -e "  docker stop station-{{.EnvironmentName}}"
echo ""
echo -e "${BLUE}Access Station UI:${NC}"
echo -e "  http://localhost:8585"
echo ""

# ============================================================================
# Example Agent Executions
# ============================================================================
#
# Security Scan:
#   docker exec station-{{.EnvironmentName}} stn agent run \
#     "Security Scanner" \
#     "Scan this project for security vulnerabilities and compliance issues" \
#     --env {{.EnvironmentName}} \
#     --tail
#
# Code Review:
#   docker exec station-{{.EnvironmentName}} stn agent run \
#     "Code Reviewer" \
#     "Review the recent code changes and provide feedback" \
#     --env {{.EnvironmentName}} \
#     --tail
#
# Test Generation:
#   docker exec station-{{.EnvironmentName}} stn agent run \
#     "Test Generator" \
#     "Generate unit tests for the auth.go file" \
#     --env {{.EnvironmentName}} \
#     --tail
#
# Documentation:
#   docker exec station-{{.EnvironmentName}} stn agent run \
#     "Documentation Writer" \
#     "Generate API documentation for the REST endpoints" \
#     --env {{.EnvironmentName}} \
#     --tail
#
# ============================================================================
# Environment Variables
# ============================================================================
#
# Required:
#   OPENAI_API_KEY       - Your OpenAI API key
#   ANTHROPIC_API_KEY    - Your Anthropic API key (if using Claude)
#
# Optional:
#   STATION_DEBUG=true   - Enable debug logging
#   STATION_PORT=8585    - Change API port
#
# ============================================================================
# Advanced Usage
# ============================================================================
#
# Mount additional volumes:
#   docker run -d \
#     --name station-{{.EnvironmentName}} \
#     -v "$(pwd)":/workspace \
#     -v ~/.ssh:/root/.ssh:ro \
#     -v /var/run/docker.sock:/var/run/docker.sock \
#     {{.DockerImage}}
#
# Run with specific environment file:
#   docker run -d \
#     --name station-{{.EnvironmentName}} \
#     --env-file .env.station \
#     {{.DockerImage}}
#
# Interactive shell access:
#   docker exec -it station-{{.EnvironmentName}} /bin/bash
#
# Execute agent with custom endpoint:
#   docker exec station-{{.EnvironmentName}} stn agent run \
#     "Agent Name" \
#     "Task" \
#     --env {{.EnvironmentName}} \
#     --endpoint http://station-api:8585
