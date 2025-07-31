# Station (stn) 🚂

**Station** is a revolutionary AI infrastructure platform that makes MCP (Model Context Protocol) servers as easy to discover, configure, and deploy as Docker containers. Think of it as the "package manager for AI agents" - transforming complex MCP server setup from hours of configuration into a 30-second guided wizard.

## 🎯 Why Station?

**The Problem**: MCP servers are powerful but painful to set up. Each repository has different installation methods, environment variables, and configuration requirements. Teams waste hours reading documentation, debugging configs, and managing scattered MCP server deployments.

**Station's Solution**: AI-powered discovery + guided wizards + seamless deployment. Point Station at any GitHub MCP server repository, and it automatically analyzes the code, presents configuration options, and sets up everything for you.

```bash
# Instead of reading docs, copying configs, debugging environment variables...
stn load https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem

# Station analyzes the repo, shows you options, guides setup, and deploys automatically
# ✨ From GitHub URL to running MCP server in 30 seconds
```

## 🚀 Value in 5 Minutes (3 Steps)

### Step 1: Initialize Station (1 minute)
```bash
# Download and initialize Station
curl -sSL https://get-station.dev | bash  # (hypothetical installer)
cd ~/my-project
stn init  # Creates config, generates encryption keys
```

### Step 2: Discover & Deploy MCP Server (2 minutes)
```bash
# AI-powered discovery from any GitHub repo
stn load https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem

# 🧙 Interactive wizard guides you through:
# - Configuration options (NPX, Docker, local build)  
# - Environment variables (with examples and validation)
# - Automatic deployment and tool discovery
```

### Step 3: Use Your MCP Server (2 minutes)
```bash
# Access admin interface
ssh admin@localhost -p 2222

# Or integrate with any MCP client
# Station exposes all discovered tools through MCP protocol
# Your filesystem tools are now available to Claude, Cody, etc.
```

**Result**: You now have a fully configured MCP server with filesystem tools, automatically discovered and deployed from a GitHub URL. No documentation reading, no configuration debugging, no manual setup.

## 🏠 Local Mode vs 🌐 Remote Mode

Station supports two deployment modes to fit different use cases:

### 🏠 Local Mode (Developer/Personal)
**Perfect for**: Solo developers, experimentation, local AI workflows

```bash
# Everything runs locally on your machine
stn init                    # Local database, local SSH
stn serve --local          # Starts all services locally
ssh admin@localhost -p 2222  # Admin interface
```

**Features**:
- ✅ Single-user operation
- ✅ Local SQLite database  
- ✅ No authentication required
- ✅ Perfect for development and testing
- ✅ All data stays on your machine

### 🌐 Remote Mode (Team/Enterprise)
**Perfect for**: Teams, production deployments, multi-user environments

```bash
# Deploy Station as a shared service
stn serve --remote --host 0.0.0.0 --port 8080
```

**Features**:
- ✅ Multi-user authentication with API keys
- ✅ Role-based access control (admin vs user)
- ✅ SSH authentication via system users
- ✅ Shared MCP server configurations
- ✅ Team collaboration and environment isolation
- ✅ Production-ready deployment

**Usage Examples**:

```bash
# Local development
stn load https://github.com/awesome/mcp-server  # Deploy locally

# Remote deployment  
stn load https://github.com/awesome/mcp-server --endpoint https://station.company.com

# Team member usage
stn load --endpoint https://station.company.com  # Uses team configurations
```

## 🛠️ Setup Examples

### Local Development Setup
```bash
# Quick local setup for development
stn init
echo 'local_mode: true' >> ~/.config/station/config.yaml
stn serve
```

### Team/Enterprise Setup
```bash
# Server setup
stn init --production
stn serve --remote --host 0.0.0.0

# Team member setup
stn config set endpoint https://your-station-server.com
stn config set api_key your-api-key-here
```

### MCP Server Discovery Examples
```bash
# Popular MCP servers - just paste GitHub URLs
stn load https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem
stn load https://github.com/awslabs/mcp/tree/main/src/cfn-mcp-server  
stn load https://github.com/kocierik/mcp-nomad

# Station automatically:
# 1. Analyzes repository structure and documentation
# 2. Extracts configuration options (NPX, Docker, local build)
# 3. Identifies required environment variables
# 4. Presents guided wizard for setup
# 5. Deploys and enables tools automatically
```

## 🏗️ Architecture

Station bridges MCP clients and MCP servers with intelligent orchestration:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   MCP Clients   │────│   Station Hub   │────│  GitHub Repos   │
│ (Claude, Cody)  │    │  (stn serve)    │    │ (MCP Servers)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                               │
                    ┌─────────────────┐
                    │  AI Discovery   │  🧠 Analyzes repos
                    │   & Wizard      │     Guides setup
                    └─────────────────┘
                               │
                    ┌─────────────────┐    ┌─────────────────┐
                    │  Deployed MCP   │    │  Admin Control  │
                    │    Servers      │    │  (SSH + Web)    │
                    └─────────────────┘    └─────────────────┘
```

### Key Innovation: AI-Powered MCP Discovery

Station uses AI to understand GitHub repositories and automatically extract:
- **Server capabilities** (what tools it provides)
- **Installation methods** (NPX, Docker, local build)
- **Environment requirements** (API keys, endpoints, configs)
- **Best practices** (recommended setups, common patterns)

This eliminates the manual work of reading documentation and figuring out configurations.

## 🎯 Core Features

- **🧙 AI-Powered Discovery**: Analyze any GitHub MCP server repository automatically
- **📋 Guided Configuration**: Interactive wizards with validation and examples
- **🚀 One-Command Deploy**: From GitHub URL to running server in seconds
- **🔒 Enterprise Security**: API keys, encryption, role-based access control
- **🌍 Multi-Environment**: Isolated configs for dev/staging/prod
- **📡 Dual-Mode SSH**: Local admin access + remote user authentication
- **🔧 Universal MCP Hub**: Connects any MCP client to any MCP server

## 🛡️ Security & Authentication

### Local Mode Security
- No authentication required (single-user)
- Local SSH access only
- Data encrypted at rest with generated keys

### Remote Mode Security  
- **API Key Authentication**: Each user gets secure API keys
- **Role-Based Access**: Admin vs user permissions
- **SSH Integration**: Authenticates against system users  
- **Encrypted Storage**: All sensitive data encrypted with NaCl
- **Audit Logging**: Track all operations and access

## 🚀 Installation & Quick Start

## 🤖 Revolutionary AI Agent Platform

Station isn't just an MCP server manager - it's the **easiest way to create background agents** that revolutionize how you work with AI:

### 🏆 Why Station for AI Agents?

**1. Background Agent Excellence**
- Create agents that run automatically on schedules
- Perfect for monitoring, data processing, content generation
- Seamless integration with your development workflow

**2. Environment-Based Tool Organization**  
- Organize tools by environments (dev/staging/prod)
- No more cluttered personal MCP configurations
- Clean separation of concerns for different projects

**3. Smart Context Management**
- Filter subtools, not just servers  
- Agents get exactly the tools they need
- No context poisoning from MCP servers with hundreds of tools

**4. Team AI Infrastructure**
- Share agents across teams
- Centralized management and monitoring
- Production-ready deployment and scaling

### 🧙 AI-Assisted Agent Creation

Station includes MCP prompts that guide your main AI (Claude, etc.) to create well-structured agents:

```bash
# Connect your Claude/AI client to Station's MCP server
# Then use the create_comprehensive_agent prompt:

"I need an agent that monitors our GitHub issues and 
creates daily summaries for the team Slack channel"

# Station's AI prompt helps Claude understand:
# - Which tools are needed (GitHub API, Slack, scheduling)
# - Optimal environment setup
# - Smart filtering to avoid tool overload  
# - Proper error handling and validation
# - Scheduling and automation patterns
```

**Example Agent Creation Flow:**
1. **Intent**: "Monitor website uptime and send alerts"
2. **Station Analysis**: Recommends HTTP monitoring tools + Slack notifications
3. **Smart Filtering**: Only assigns essential tools (http-client, slack-api) 
4. **Environment**: Selects monitoring environment with proper credentials
5. **Deployment**: Creates scheduled agent with error handling

### Option 1: Quick Install (Recommended)
```bash
# Install Station
curl -sSL https://get-station.dev | bash
stn init

# Discover and deploy MCP server
stn load https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem

# Start using immediately
ssh admin@localhost -p 2222
```

### Option 2: Build from Source
```bash
git clone https://github.com/your-org/station.git
cd station
make build
./stn init
```

## 📚 Commands Reference

### Core Commands
```bash
stn init                    # Initialize Station configuration
stn serve                   # Start Station services (local mode)
stn serve --remote          # Start in remote/team mode
stn load <github-url>       # Discover and deploy MCP server
stn load                    # Load from local mcp.json/.mcp.json
```

### Configuration
```bash
stn config show             # View current configuration
stn config set key value    # Update configuration
stn key generate            # Generate new encryption key
```

### Environment Management
```bash
stn env create <name>       # Create new environment
stn env list                # List environments
stn env switch <name>       # Switch active environment
```

## 🤝 SSH Integration (Remote Mode)

In remote mode, Station integrates with your system's SSH configuration for user authentication:

```bash
# Server reads from system SSH config
# Users authenticate with their system credentials
ssh user@station-server -p 2222

# Supports:
# - SSH key authentication
# - System user validation  
# - Host-based authentication
# - All standard SSH auth methods
```

Station's SSH server in remote mode leverages the host system's SSH configuration, making it seamless to integrate with existing user management and authentication systems.

## 🔧 Troubleshooting

### Common Issues

**"No MCP configuration found"**
```bash
# Ensure you have mcp.json or .mcp.json in current directory
# Or use GitHub URL discovery instead
stn load https://github.com/some/mcp-server
```

**"GitHub analysis failed"**
```bash
# Check internet connection and GitHub URL format
# Ensure repository contains MCP server code
```

**"SSH connection refused"**
```bash
# Check Station is running and SSH port is correct
stn serve  # Start Station if not running
ssh admin@localhost -p 2222  # Default SSH port
```

## 📄 License

AGPL-3.0 - Open source with copyleft provisions for service deployments.

## 🌟 Contributing

Station is built for the community. Contributions welcome!

1. Fork and create feature branch
2. Add tests and documentation  
3. Submit pull request

---

**Station** - Making MCP servers as easy as `npm install` • Built with ❤️ for the AI community