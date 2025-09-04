# Station Installation Guide

Station provides multiple installation methods to suit different use cases and environments.

## Quick Installation

The fastest way to get started with Station:

```bash
# Install Station CLI
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

# Bootstrap with OpenAI integration and example agents
stn bootstrap --openai
```

This single command sets up:
- Station CLI (`stn` command)
- OpenAI integration (requires `OPENAI_API_KEY`)
- Example agents and environments
- Basic MCP tools (filesystem, web automation)

## Prerequisites

### System Requirements
- **Operating System**: Linux, macOS, or Windows with WSL2
- **Architecture**: x86_64 or ARM64
- **Memory**: 512MB RAM minimum, 2GB recommended
- **Disk**: 100MB for Station, additional space for agent data
- **Network**: Internet access for MCP tool installation

### Required Environment Variables
```bash
export OPENAI_API_KEY="your-openai-api-key-here"
```

### Optional Dependencies
- **Docker**: For containerized MCP tools
- **Node.js**: For npm-based MCP servers
- **Python**: For Python-based tools and agents

## Installation Methods

### Method 1: Automated Installation (Recommended)

```bash
# Download and run the installer
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

# Verify installation
stn --version
```

The installer automatically:
- Downloads the latest Station binary
- Installs to `~/.local/bin/stn`
- Adds to PATH if needed
- Sets up basic configuration

### Method 2: Manual Installation

1. **Download Binary**:
```bash
# Linux x86_64
wget https://github.com/cloudshipai/station/releases/latest/download/station-linux-amd64
chmod +x station-linux-amd64
mv station-linux-amd64 ~/.local/bin/stn

# macOS
wget https://github.com/cloudshipai/station/releases/latest/download/station-darwin-amd64
chmod +x station-darwin-amd64
mv station-darwin-amd64 ~/.local/bin/stn
```

2. **Add to PATH**:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

### Method 3: Build from Source

```bash
# Clone repository
git clone https://github.com/cloudshipai/station.git
cd station

# Build Station
make build

# Install locally
make local-install
```

## Initial Setup

### 1. Initialize Station

Choose your initialization method:

```bash
# Quick setup with OpenAI
stn init --provider openai --model gpt-4o

# Or manual setup
stn init
```

### 2. Configure API Keys

```bash
# Set OpenAI API key
export OPENAI_API_KEY="your-key-here"

# Or add to your shell profile
echo 'export OPENAI_API_KEY="your-key-here"' >> ~/.bashrc
```

### 3. Verify Setup

```bash
# Check status
stn status

# List available commands
stn --help

# Test with a simple agent run
stn agent list
```

## Configuration Locations

Station stores configuration in standard locations:

```
~/.config/station/           # Main configuration directory
├── config.yaml             # Station configuration
├── database.sqlite         # Agent and execution data
└── environments/           # Multi-environment configs
    ├── default/            # Default environment
    │   ├── template.json   # MCP server configuration
    │   ├── variables.yml   # Environment variables
    │   └── agents/         # Agent definitions (.prompt files)
    └── production/         # Additional environments
        ├── template.json
        └── agents/
```

## Environment Setup

### Development Environment

For development work with multiple environments:

```bash
# Create development environment
stn env create development

# Set up environment variables
cd ~/.config/station/environments/development
echo "PROJECT_ROOT: $(pwd)" > variables.yml
```

### Production Environment

For production deployments:

```bash
# Create production environment
stn env create production

# Configure production variables
cd ~/.config/station/environments/production
cat > variables.yml << EOF
PROJECT_ROOT: /opt/project
LOG_LEVEL: info
ENVIRONMENT: production
EOF
```

## Installing MCP Tools

Station works with MCP (Model Context Protocol) tools:

### Common MCP Servers

```bash
# Filesystem operations
npm install -g @modelcontextprotocol/server-filesystem

# Web automation with Playwright
npm install -g @modelcontextprotocol/server-playwright

# Database operations
npm install -g @modelcontextprotocol/server-postgres
```

### Ship Security Tools

For security-focused workflows:

```bash
# Install Ship CLI (provides 300+ security tools)
curl -sSL https://ship.sh/install | bash

# Verify Ship is available
ship --version
```

## Troubleshooting Installation

### Common Issues

#### Permission Denied
```bash
# Fix PATH issues
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Or install to different location
mkdir -p ~/bin
mv stn ~/bin/
export PATH="$HOME/bin:$PATH"
```

#### Missing Dependencies
```bash
# Install Node.js (for MCP servers)
curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install Docker (for containerized tools)
curl -fsSL https://get.docker.com | sudo sh
```

#### Network Issues
```bash
# Check connectivity
curl -I https://github.com/cloudshipai/station

# Use proxy if needed
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=http://proxy:8080
```

### Logs and Debugging

```bash
# Check Station logs
stn logs

# Enable debug logging
export STATION_LOG_LEVEL=debug
stn status

# Verify configuration
stn config check
```

## Next Steps

After successful installation:

1. **[Quick Start Guide](./QUICKSTART.md)** - Create your first agent
2. **[Agent Creation](./agents/CREATING_AGENTS.md)** - Build custom agents
3. **[MCP Integration](./agents/MCP_INTEGRATION.md)** - Add tools to your agents
4. **[Bundle System](./bundles/BUNDLE_SYSTEM.md)** - Package and share agents

## Updating Station

```bash
# Check for updates
stn version --check

# Update to latest version
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

# Or manual update
wget https://github.com/cloudshipai/station/releases/latest/download/station-linux-amd64
chmod +x station-linux-amd64
mv station-linux-amd64 ~/.local/bin/stn
```