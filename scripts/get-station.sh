#!/bin/bash

# One-Line Station Installer & Launcher
# Usage: curl -fsSL https://get.station.dev | bash

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

cat << "EOF"
╔═══════════════════════════════════════╗
║   🚂 Station - One-Line Deploy       ║
║   Install & Run in 60 Seconds        ║
╚═══════════════════════════════════════╝
EOF

echo ""
echo -e "${BLUE}Step 1/3: Installing Station CLI...${NC}"
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

echo ""
echo -e "${BLUE}Step 2/3: Configuring Station...${NC}"

# Check for API key
if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$GOOGLE_API_KEY" ]; then
    echo ""
    echo -e "${CYAN}Choose your AI provider:${NC}"
    echo "  1) OpenAI (recommended)"
    echo "  2) Anthropic Claude"
    echo "  3) Google Gemini"
    echo "  4) Custom/Ollama"
    read -p "Selection [1]: " provider_choice
    provider_choice=${provider_choice:-1}
    
    case $provider_choice in
        1)
            read -p "Enter OpenAI API key: " OPENAI_API_KEY
            export OPENAI_API_KEY
            PROVIDER="openai"
            MODEL="gpt-4o-mini"
            ;;
        2)
            read -p "Enter Anthropic API key: " ANTHROPIC_API_KEY
            export ANTHROPIC_API_KEY
            PROVIDER="anthropic"
            MODEL="claude-3-sonnet"
            ;;
        3)
            read -p "Enter Google API key: " GOOGLE_API_KEY
            export GOOGLE_API_KEY
            PROVIDER="gemini"
            MODEL="gemini-2.0-flash-exp"
            ;;
        4)
            read -p "Enter base URL (e.g., http://localhost:11434/v1): " BASE_URL
            read -p "Enter model name: " MODEL
            PROVIDER="openai"
            ;;
    esac
else
    # Auto-detect provider from env
    if [ -n "$OPENAI_API_KEY" ]; then
        PROVIDER="openai"
        MODEL="gpt-4o-mini"
    elif [ -n "$ANTHROPIC_API_KEY" ]; then
        PROVIDER="anthropic"
        MODEL="claude-3-sonnet"
    elif [ -n "$GOOGLE_API_KEY" ]; then
        PROVIDER="gemini"
        MODEL="gemini-2.0-flash-exp"
    fi
fi

echo ""
echo -e "${BLUE}Step 3/3: Starting Station...${NC}"

# Add to PATH if needed
export PATH="$HOME/.local/bin:$PATH"

# Start Station
if [ -n "$BASE_URL" ]; then
    stn up --provider "$PROVIDER" --base-url "$BASE_URL" --model "$MODEL" &
else
    stn up --provider "$PROVIDER" --model "$MODEL" &
fi

# Wait for startup
sleep 5

echo ""
echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   ✅ Station is Running!              ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════╝${NC}"
echo ""
echo -e "${CYAN}📍 Access Points:${NC}"
echo "  • Web UI:      http://localhost:8585"
echo "  • MCP Server:  http://localhost:8586/mcp"
echo ""
echo -e "${CYAN}🔧 Quick Commands:${NC}"
echo "  • Stop:        stn down"
echo "  • Status:      stn status"
echo "  • List agents: stn agent list"
echo ""
echo -e "${CYAN}📚 Next Steps:${NC}"
echo "  1. Open http://localhost:8585 in your browser"
echo "  2. Add MCP tools via the UI"
echo "  3. Create your first agent"
echo "  4. Install bundles from https://registry.station.dev"
echo ""

# Try to open browser
if command -v open &> /dev/null; then
    sleep 2
    open "http://localhost:8585" 2>/dev/null || true
elif command -v xdg-open &> /dev/null; then
    sleep 2
    xdg-open "http://localhost:8585" 2>/dev/null || true
fi

echo -e "${GREEN}Happy building with Station! 🚂${NC}"
