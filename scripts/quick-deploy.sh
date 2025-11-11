#!/bin/bash

# Station Quick Deploy Script
# One-command deployment of Station with bundles
# Usage: ./quick-deploy.sh [options]

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
DEPLOY_MODE="docker-compose"  # docker-compose, docker, kubernetes, local
BUNDLES_DIR="./bundles"
ENV_NAME="default"
PROVIDER="openai"
MODEL="gpt-4o-mini"
PORT="8585"
MCP_PORT="8586"
AUTO_OPEN_UI=true

# Banner
print_banner() {
    echo -e "${BLUE}"
    cat << "EOF"
╔═══════════════════════════════════════╗
║   🚀 Station Quick Deploy             ║
║   Zero to Running in 60 Seconds       ║
╚═══════════════════════════════════════╝
EOF
    echo -e "${NC}"
}

log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
    exit 1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    case "$DEPLOY_MODE" in
        docker-compose)
            if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
                log_error "Docker Compose not found. Install: https://docs.docker.com/compose/install/"
            fi
            ;;
        docker)
            if ! command -v docker &> /dev/null; then
                log_error "Docker not found. Install: https://docs.docker.com/get-docker/"
            fi
            ;;
        kubernetes)
            if ! command -v kubectl &> /dev/null; then
                log_error "kubectl not found. Install: https://kubernetes.io/docs/tasks/tools/"
            fi
            ;;
        local)
            if ! command -v stn &> /dev/null; then
                log_error "Station CLI not found. Run: curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash"
            fi
            ;;
    esac
    
    log_success "Prerequisites OK"
}

# Check for API key
check_api_key() {
    log_info "Checking AI provider configuration..."
    
    case "$PROVIDER" in
        openai)
            if [ -z "$OPENAI_API_KEY" ]; then
                echo ""
                log_warning "OPENAI_API_KEY not set"
                read -p "Enter your OpenAI API key: " -r OPENAI_API_KEY
                export OPENAI_API_KEY
            fi
            ;;
        anthropic)
            if [ -z "$ANTHROPIC_API_KEY" ]; then
                echo ""
                log_warning "ANTHROPIC_API_KEY not set"
                read -p "Enter your Anthropic API key: " -r ANTHROPIC_API_KEY
                export ANTHROPIC_API_KEY
            fi
            ;;
        gemini)
            if [ -z "$GOOGLE_API_KEY" ]; then
                echo ""
                log_warning "GOOGLE_API_KEY not set"
                read -p "Enter your Google API key: " -r GOOGLE_API_KEY
                export GOOGLE_API_KEY
            fi
            ;;
    esac
    
    log_success "API key configured"
}

# Setup bundles directory
setup_bundles() {
    log_info "Setting up bundles directory..."
    
    if [ ! -d "$BUNDLES_DIR" ]; then
        mkdir -p "$BUNDLES_DIR"
        log_info "Created bundles directory: $BUNDLES_DIR"
        log_info "You can add .tar.gz bundle files to this directory"
    else
        local bundle_count=$(ls -1 "$BUNDLES_DIR"/*.tar.gz 2>/dev/null | wc -l)
        if [ "$bundle_count" -gt 0 ]; then
            log_success "Found $bundle_count bundle(s) in $BUNDLES_DIR"
        else
            log_info "No bundles found. You can add .tar.gz files to $BUNDLES_DIR"
        fi
    fi
}

# Deploy with Docker Compose
deploy_docker_compose() {
    log_info "Deploying with Docker Compose..."
    
    # Create docker-compose.yml
    cat > docker-compose.yml << EOF
version: '3.8'

services:
  station:
    image: ghcr.io/cloudshipai/station:latest
    container_name: station-quick-deploy
    ports:
      - "${PORT}:8585"
      - "${MCP_PORT}:8586"
    environment:
      - OPENAI_API_KEY=\${OPENAI_API_KEY}
      - ANTHROPIC_API_KEY=\${ANTHROPIC_API_KEY}
      - GOOGLE_API_KEY=\${GOOGLE_API_KEY}
      - AWS_ACCESS_KEY_ID=\${AWS_ACCESS_KEY_ID}
      - AWS_SECRET_ACCESS_KEY=\${AWS_SECRET_ACCESS_KEY}
      - AWS_REGION=\${AWS_REGION:-us-east-1}
    volumes:
      - ./bundles:/bundles:ro
      - station-data:/root/.config/station
    command: >
      sh -c "
        echo '🚀 Station Quick Deploy Starting...' &&
        
        if [ ! -f /root/.config/station/config.yaml ]; then
          echo '📦 Initializing Station...' &&
          stn init --provider ${PROVIDER} --model ${MODEL} --yes
        fi &&
        
        if [ -d /bundles ] && [ \"\\\$(ls -A /bundles/*.tar.gz 2>/dev/null)\" ]; then
          echo '📦 Installing bundles...' &&
          for bundle in /bundles/*.tar.gz; do
            bundle_name=\\\$(basename \"\\\$bundle\" .tar.gz) &&
            echo \"  ✓ Installing: \\\$bundle_name\" &&
            stn bundle install \"\\\$bundle\" \"\\\$bundle_name\" &&
            stn sync \"\\\$bundle_name\" -i=false || true
          done &&
          echo '✅ Bundles installed!'
        fi &&
        
        echo '🌐 Starting Station server...' &&
        echo '   Web UI: http://localhost:${PORT}' &&
        echo '   MCP Server: http://localhost:${MCP_PORT}/mcp' &&
        stn serve
      "
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8585/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  station-data:
    driver: local
EOF
    
    log_success "Created docker-compose.yml"
    
    # Start services
    log_info "Starting Station..."
    if command -v docker-compose &> /dev/null; then
        docker-compose up -d
    else
        docker compose up -d
    fi
    
    log_success "Station is starting!"
    
    # Wait for health check
    log_info "Waiting for Station to be ready..."
    local max_wait=60
    local waited=0
    while [ $waited -lt $max_wait ]; do
        if curl -sf "http://localhost:${PORT}/health" > /dev/null 2>&1; then
            log_success "Station is ready!"
            break
        fi
        sleep 2
        waited=$((waited + 2))
        echo -n "."
    done
    echo ""
    
    if [ $waited -ge $max_wait ]; then
        log_warning "Station is starting but health check hasn't passed yet"
        log_info "Check status with: docker-compose logs -f"
    fi
}

# Deploy with plain Docker
deploy_docker() {
    log_info "Deploying with Docker..."
    
    docker run -d \
        --name station-quick-deploy \
        -p "${PORT}:8585" \
        -p "${MCP_PORT}:8586" \
        -e "OPENAI_API_KEY=${OPENAI_API_KEY}" \
        -e "ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}" \
        -e "GOOGLE_API_KEY=${GOOGLE_API_KEY}" \
        -v "$(pwd)/bundles:/bundles:ro" \
        -v "station-data:/root/.config/station" \
        ghcr.io/cloudshipai/station:latest \
        sh -c "stn init --provider ${PROVIDER} --model ${MODEL} --yes && stn serve"
    
    log_success "Station container started!"
}

# Deploy locally
deploy_local() {
    log_info "Starting Station locally..."
    
    # Check if already running
    if pgrep -f "stn (up|serve)" > /dev/null; then
        log_warning "Station is already running. Stop it with: stn down"
        exit 0
    fi
    
    # Start Station with stn up (which handles initialization and serves)
    # Note: stn up doesn't support custom ports, uses defaults 8585/8586
    log_info "Running: stn up --provider ${PROVIDER} --model ${MODEL}"
    stn up --provider "${PROVIDER}" --model "${MODEL}" &
    
    local pid=$!
    log_success "Station started (PID: $pid)"
    log_info "Stop with: stn down"
    
    if [ "$PORT" != "8585" ] || [ "$MCP_PORT" != "8586" ]; then
        log_warning "Custom ports not supported in local mode. Using defaults: 8585, 8586"
    fi
}

# Print deployment summary
print_summary() {
    echo ""
    echo -e "${PURPLE}╔═══════════════════════════════════════╗${NC}"
    echo -e "${PURPLE}║   🎉 Deployment Complete!             ║${NC}"
    echo -e "${PURPLE}╚═══════════════════════════════════════╝${NC}"
    echo ""
    echo -e "${CYAN}📍 Access Points:${NC}"
    echo "  • Web UI:      http://localhost:${PORT}"
    echo "  • MCP Server:  http://localhost:${MCP_PORT}/mcp"
    echo ""
    echo -e "${CYAN}🔧 Quick Commands:${NC}"
    
    case "$DEPLOY_MODE" in
        docker-compose)
            echo "  • View logs:   docker-compose logs -f"
            echo "  • Stop:        docker-compose down"
            echo "  • Restart:     docker-compose restart"
            echo "  • Shell:       docker-compose exec station sh"
            ;;
        docker)
            echo "  • View logs:   docker logs -f station-quick-deploy"
            echo "  • Stop:        docker stop station-quick-deploy"
            echo "  • Restart:     docker restart station-quick-deploy"
            echo "  • Shell:       docker exec -it station-quick-deploy sh"
            ;;
        local)
            echo "  • Stop:        stn down"
            echo "  • Status:      stn status"
            echo "  • Logs:        Check terminal output"
            ;;
    esac
    
    echo ""
    echo -e "${CYAN}📚 Next Steps:${NC}"
    echo "  1. Open Web UI and explore available tools"
    echo "  2. Install bundles from https://registry.station.dev"
    echo "  3. Create your first agent"
    echo "  4. Run agents from Claude Code/Cursor"
    echo ""
    
    if [ "$AUTO_OPEN_UI" = true ] && command -v open &> /dev/null; then
        log_info "Opening Web UI in browser..."
        sleep 2
        open "http://localhost:${PORT}" 2>/dev/null || true
    elif [ "$AUTO_OPEN_UI" = true ] && command -v xdg-open &> /dev/null; then
        log_info "Opening Web UI in browser..."
        sleep 2
        xdg-open "http://localhost:${PORT}" 2>/dev/null || true
    fi
    
    echo -e "${GREEN}Happy building with Station! 🚂${NC}"
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --mode)
                DEPLOY_MODE="$2"
                shift 2
                ;;
            --provider)
                PROVIDER="$2"
                shift 2
                ;;
            --model)
                MODEL="$2"
                shift 2
                ;;
            --port)
                PORT="$2"
                shift 2
                ;;
            --mcp-port)
                MCP_PORT="$2"
                shift 2
                ;;
            --bundles-dir)
                BUNDLES_DIR="$2"
                shift 2
                ;;
            --no-open)
                AUTO_OPEN_UI=false
                shift
                ;;
            --help)
                cat << EOF
Station Quick Deploy

Usage: $0 [options]

Options:
  --mode MODE          Deployment mode: docker-compose (default), docker, kubernetes, local
  --provider PROVIDER  AI provider: openai (default), anthropic, gemini
  --model MODEL        AI model (default: gpt-4o-mini)
  --port PORT          Web UI port (default: 8585)
  --mcp-port PORT      MCP server port (default: 8586)
  --bundles-dir DIR    Bundles directory (default: ./bundles)
  --no-open            Don't auto-open browser
  --help               Show this help

Environment Variables:
  OPENAI_API_KEY       OpenAI API key
  ANTHROPIC_API_KEY    Anthropic API key
  GOOGLE_API_KEY       Google API key
  AWS_ACCESS_KEY_ID    AWS credentials (optional)
  AWS_SECRET_ACCESS_KEY

Examples:
  # Quick start with Docker Compose (recommended)
  ./quick-deploy.sh

  # Deploy locally
  ./quick-deploy.sh --mode local

  # Use Anthropic Claude
  ./quick-deploy.sh --provider anthropic --model claude-3-sonnet

  # Custom ports
  ./quick-deploy.sh --port 9000 --mcp-port 9001
EOF
                exit 0
                ;;
            *)
                log_error "Unknown option: $1. Use --help for usage."
                ;;
        esac
    done
}

# Main
main() {
    print_banner
    parse_args "$@"
    check_prerequisites
    check_api_key
    setup_bundles
    
    case "$DEPLOY_MODE" in
        docker-compose)
            deploy_docker_compose
            ;;
        docker)
            deploy_docker
            ;;
        local)
            deploy_local
            ;;
        *)
            log_error "Unsupported deployment mode: $DEPLOY_MODE"
            ;;
    esac
    
    print_summary
}

# Handle Ctrl+C
trap 'echo -e "\n${RED}Deployment interrupted by user.${NC}"; exit 1' INT

main "$@"
