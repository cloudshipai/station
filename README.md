![Station](./station-logo.png)

# Station - Self-Hosted AI Agent Platform

**Turn your team's tools into AI agents. Deploy securely. Scale reliably.**

🌐 **[Browse Bundle Registry](https://cloudshipai.github.io/registry)** - Discover ready-to-use MCP bundles for Station

📚 **[Documentation](https://cloudshipai.github.io/station)** - Complete Station documentation and guides

---

## What is Station?

Station is a **self-hosted platform** that lets you create AI agents that can use your existing tools, access your systems, and automate your workflows - all running securely in your infrastructure.

**Key Benefits:**
- 🔐 **Self-hosted & secure** - Your data stays in your infrastructure
- 🛠️ **Use existing tools** - Integrate with databases, APIs, monitoring, deployment tools
- 📦 **Template & share agents** - Create bundles that teams can easily deploy
- ⚡ **Lightweight** - Single 45MB binary, runs anywhere
- 🌍 **Multi-environment** - Separate dev/staging/production contexts

## Perfect For

**DevOps & Infrastructure Teams**
- Database monitoring agents with production credentials
- Automated deployment pipelines with rollback capability  
- Infrastructure-as-Code management with Terraform/Kubernetes
- Security scanning and compliance automation

**Development Teams**
- Code review and deployment agents
- Testing automation and quality assurance
- Documentation generation and maintenance
- Project management and workflow automation

**Operations Teams**  
- Incident response and alerting automation
- System monitoring and health checks
- Log analysis and anomaly detection
- Customer support automation with internal system access

## How Station Works

### 1. **Load Your Tools**
Station uses the **MCP (Model Context Protocol)** standard to integrate with your existing tools:

```bash
# Load your team's approved tools
stn load examples/mcps/aws-cli.json          # AWS infrastructure
stn load examples/mcps/database-postgres.json # Database access  
stn load examples/mcps/slack.json            # Team communication
stn sync production                          # Deploy to environment
```

### 2. **Create Smart Agents**
Build agents that can use those tools intelligently:

```bash
# Create an infrastructure monitoring agent
stn agent create \
  --name "Database Monitor" \
  --description "Monitor production database health and alert on issues"

# Test it locally
stn agent run 1 "Check database connection pool and alert if over 80% capacity"
```

### 3. **Template & Deploy**
Package agents into reusable templates for your team:

```bash
# Create template from your working environment
stn template create db-monitor-bundle --env production \
  --name "Database Monitoring Suite" \
  --author "DevOps Team"

# Share with your team
stn template bundle db-monitor-bundle        # Creates .tar.gz
stn template install db-monitor-bundle.tar.gz staging  # Deploy to staging
```

## Getting Started

### 1. **Install Station**
```bash
# Quick install (recommended) 
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash

# Or build from source
git clone https://github.com/cloudshipai/station
cd station && go build -o stn ./cmd/main

# Or download binary from GitHub releases
# https://github.com/cloudshipai/station/releases
```

### 2. **Initialize Your Environment**
```bash
# Set up Station with database and encryption
stn init

# Start with some example tools
stn load examples/mcps/filesystem.json        # File operations
stn load examples/mcps/slack.json             # Team notifications  
stn sync default                              # Deploy to environment
```

### 3. **Create Your First Agent**
```bash
# Create a file management agent
stn agent create \
  --name "File Organizer" \
  --description "Organize and analyze files in project directories"

# Test it out
stn agent run 1 "Analyze the current directory and organize files by type"
```

### 4. **Use the Web Interface** *(Optional)*
```bash
# Launch Station's terminal interface
stn ui

# Or start the full web server
stn serve
# Then connect via SSH: ssh admin@localhost -p 2222
```

## Why Choose Station?

### 🔐 **Security First**
- **Self-hosted** - Your data never leaves your infrastructure
- **Encrypted secrets** - AES encryption for all sensitive configuration
- **Environment isolation** - Complete separation of dev/staging/production
- **Audit logging** - Full tracking of all agent activities

### ⚡ **Developer Friendly** 
- **Single binary** - 45MB download, zero dependencies
- **GitOps ready** - Version control agents like infrastructure code
- **Rich templates** - Share and reuse agent configurations across teams
- **Multiple interfaces** - CLI, web UI, SSH, REST API, MCP server

### 🛠️ **Enterprise Ready**
- **Multi-environment** - Separate contexts for different deployment stages
- **Queue-based execution** - Scalable async processing
- **Webhook integration** - Connect to existing notification systems
- **Database replication** - Production backup with Litestream

## Common Use Cases

### **Infrastructure & DevOps**
```bash
# Database monitoring with production access
stn agent create --name "DB Health Monitor" \
  --description "Monitor connection pools, query performance, disk usage"

# CI/CD pipeline automation
stn agent create --name "Deployment Pipeline" \
  --description "Automated deployments with health checks and rollback"

# Infrastructure as Code management  
stn agent create --name "Terraform Manager" \
  --description "Plan, apply, and manage infrastructure changes"
```

### **Security & Compliance**
```bash
# Vulnerability scanning and alerting
stn agent create --name "Security Scanner" \
  --description "Scan for vulnerabilities and compliance issues"

# Incident response automation
stn agent create --name "Incident Responder" \
  --description "Automated incident detection and initial response"
```

### **Development & Operations**
```bash
# Code review and quality assurance
stn agent create --name "Code Reviewer" \
  --description "Automated code review with team standards"

# Documentation maintenance
stn agent create --name "Doc Generator" \
  --description "Generate and maintain technical documentation"
```

## Available Tools

Station integrates with **any MCP-compatible tool**. Here are some popular categories:

| **Category** | **Examples** |
|--------------|--------------|
| **Infrastructure** | AWS CLI, Kubernetes, Docker, Terraform, SSH |
| **Databases** | PostgreSQL, MySQL, MongoDB, Redis, SQLite |
| **Monitoring** | Prometheus, Grafana, System metrics, Log analysis |
| **Communication** | Slack, PagerDuty, Email, Webhook notifications |
| **Security** | Vault, Certificate management, Access control |
| **Files & Storage** | Local files, S3, Git repositories, Configuration management |

```bash
# Load tools for your agents
stn load examples/mcps/aws-infrastructure.json
stn load examples/mcps/database-monitoring.json  
stn load examples/mcps/team-communication.json
stn sync production
```

### **Template System**

Share agents across your team with Station's template system:

```bash
# Create reusable templates from working environments
stn template create monitoring-suite --env production \
  --name "Database Monitoring Suite" --author "DevOps Team"

# Package and share
stn template bundle monitoring-suite
stn template install monitoring-suite.tar.gz staging

# Discover community templates
stn template list --registry community
```

## Key Commands Reference

### **Agent Management**
```bash
stn agent create --name "Agent Name" --env production
stn agent run <id> "Task description"  
stn agent list --env production
stn agent export <id> ./my-agent-bundle
```

### **Template Operations**  
```bash
stn template create my-bundle --env production
stn template validate my-bundle
stn template bundle my-bundle  
stn template install bundle.tar.gz staging
```

### **Tool & Environment Management**
```bash
stn load config.json                    # Load MCP tools
stn sync production                     # Deploy configurations
stn status                             # Check system health
```

### **Server & Interface Options**
```bash
stn ui                                 # Terminal interface
stn serve                             # Web server + SSH  
stn stdio                             # MCP server mode
```

## Advanced Features

### **MCP Server Integration**

Station can serve as an MCP server for other AI applications:

```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"]
    }
  }
}
```

This provides tools like `call_agent`, `create_agent`, `list_agents` to any MCP-compatible application.

### **Interactive Development Playground**

**NEW**: Station includes a powerful interactive development environment powered by Firebase Genkit:

```bash
genkit start -- stn develop --env dev
```

This launches a complete browser-based development playground where you can:
- **Test agents interactively** with custom task inputs
- **Debug tool calling** with real-time execution traces  
- **Access all MCP tools** from your environment
- **Iterate on prompts** with live reloading
- **Analyze execution flows** with detailed logging

Perfect for developing and testing agents before deployment.

## System Requirements

- **OS:** Linux, macOS, Windows  
- **Memory:** 512MB minimum, 1GB recommended
- **Storage:** 200MB for binary, 1GB+ for agent data
- **Database:** SQLite (development) or PostgreSQL (production)
- **Network:** Outbound HTTPS for AI providers

## Resources

- 📚 **[Documentation](https://cloudshipai.github.io/station)** - Complete guides and tutorials
- 🌐 **[Bundle Registry](https://cloudshipai.github.io/registry)** - Community agent templates
- 🐛 **[Issues](https://github.com/cloudshipai/station/issues)** - Bug reports and feature requests
- 💬 **[Discord](https://discord.gg/station-ai)** - Community support and discussions

## License

**AGPL-3.0** - Free for all use, open source contributions welcome.

---

**Station - Self-Hosted AI Agent Platform**

*Turn your team's tools into AI agents. Deploy securely. Scale reliably.*