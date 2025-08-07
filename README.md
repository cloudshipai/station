![Station](./image.png)

# Station - GitOps for AI Agents

**Deploy AI agents as reliably as infrastructure - with declarative configuration, secret management, and team collaboration.**

> Production-ready v0.2.0 - Self-hosted platform for enterprise AI agent deployment

Station solves the **agent configuration hell** that prevents teams from deploying AI agents reliably across environments. No more "works on my machine" - deploy agents with the same reliability as infrastructure code.

## What Station Actually Is

Station enables the complete AI agent lifecycle from development to production:

- **🔬 Develop Locally** - Quick agent creation and testing with zero setup overhead
- **🤖 Orchestrate Complexity** - Sub-agents calling other agents for multi-step workflows  
- **🚀 Deploy Securely** - GitOps workflows with encrypted secret management
- **👥 Share Patterns** - Version-controlled agent templates across teams

**Think "Terraform for AI Agents" - but easier.**

## The Problem Station Solves

**Agent Configuration Hell:**
- Agents work locally but fail in production due to missing dependencies
- No way to securely manage MCP credentials and API keys across environments  
- Teams can't share working agent setups - everyone rebuilds from scratch
- Configuration drift between dev/staging/prod environments causes unpredictable behavior

**Station's Solution:**
**Declarative agent deployment with GitOps workflows and encrypted secret management.**

## Core Value Propositions

### 🔐 **Secure Internal Agent Deployment**

**Deploy agents that access internal systems with enterprise-grade security.**

Traditional platforms can't handle internal agents that need database credentials, API keys, and system access. Station provides encrypted secret management for production-ready internal automation.

```bash
# Declarative agent template with encrypted secrets
agents/database-monitor/
├── agent.json              # Agent configuration with {{variables}}
├── manifest.json           # MCP dependencies and tool requirements
├── variables.schema.json   # Variable validation schema
└── secrets/
    ├── production.enc      # Encrypted prod DB credentials
    └── staging.enc         # Encrypted staging credentials

# Secure deployment with full audit trail
stn agent bundle install ./database-monitor \
  --env production \
  --vars-file secrets/production.enc

# Result: Verifiable, auditable, repeatable agent deployment
```

**Use Cases:**
- **Database monitoring** with production DB credentials
- **Infrastructure agents** accessing internal APIs and secrets
- **Security scanning** with elevated system access  
- **Deployment automation** with CI/CD system credentials

### 🌍 **Environment Consistency**

**Deploy identical agent logic across dev/staging/prod with environment-specific configuration.**

Eliminate configuration drift with declarative templates. One agent definition works everywhere with only variable differences.

```bash
# Single template, multiple environments
# Development variables (dev-vars.json):
{
  "DB_HOST": "localhost:5432",
  "ALERT_THRESHOLD": 50,
  "SLACK_WEBHOOK": "https://hooks.slack.com/dev-alerts"
}

# Production variables (prod-vars.json):  
{
  "DB_HOST": "prod-db.company.com:5432", 
  "ALERT_THRESHOLD": 90,
  "SLACK_WEBHOOK": "https://hooks.slack.com/prod-alerts"
}

# Deploy identical logic with environment differences
stn agent bundle install ./database-monitor --vars-file dev-vars.json --env dev
stn agent bundle install ./database-monitor --vars-file prod-vars.json --env prod

# Result: 100% configuration consistency, zero environment drift
```

### 👥 **Team Collaboration & GitOps**

**Share agent templates across teams with version-controlled workflows.**

Version control complete agent configurations like infrastructure code. Enable enterprise-wide agent standardization and collaboration.

```bash
# Export working agent as template
stn agent bundle export 5 ./shared-templates/log-analyzer --include-deps

# Version control with GitOps
git add shared-templates/log-analyzer/
git commit -m "Add log analyzer agent v1.2 - supports JSON parsing" 
git push origin main

# Other teams install from repository
git pull && stn agent bundle install ./shared-templates/log-analyzer \
  --vars-file our-environment.json --env staging

# API deployment for CI/CD automation
curl -X POST http://station.company.com/api/v1/agents/templates/install \
  -H "Content-Type: application/json" \
  -d '{
    "bundle_path": "./shared-templates/log-analyzer",
    "environment": "production",
    "variables": {
      "LOG_DIRECTORY": "/var/log/applications",
      "ALERT_EMAIL": "platform-team@company.com"
    }
  }'

# Result: Enterprise-wide agent standardization with full audit trails
```

## Quick Start (5 Minutes)

### 1. Install Station
```bash
# Build from source
git clone https://github.com/cloudshipai/station
cd station && go build -o stn ./cmd/main

# Binary install (coming soon)
curl -sSL https://getstation.ai/install | bash
```

### 2. Initialize & Load Tools
```bash
# Initialize with encrypted key management
stn init

# Load MCP tools for agent development
stn load examples/mcps/filesystem.json
stn load examples/mcps/database-sqlite.json
```

### 3. Develop Locally
```bash
# Quick local development
stn agent create \
  --name "System Monitor" \
  --description "Monitor system resources and alert on issues"

# Test locally
stn agent run 1 "Check disk usage and memory consumption"

# View execution history
stn runs list
```

### 4. Deploy with GitOps
```bash
# Export as template for sharing
stn agent bundle export 1 ./templates/system-monitor --analyze-vars

# Version control
git add templates/ && git commit -m "Add system monitor agent"

# Deploy to production with encrypted secrets
stn agent bundle install ./templates/system-monitor \
  --vars-file secrets/production.enc --env production
```

## Platform Features

### 🔬 **Local Development Experience**
- **Zero-Setup Development** - Single binary with embedded database
- **Interactive Agent Creation** - AI-assisted configuration and tool selection
- **Real-Time Testing** - Immediate feedback with execution tailing
- **Sub-Agent Orchestration** - Agents calling other agents for complex workflows

### 🚀 **Production Deployment**
- **Declarative Configuration** - Complete agent definitions in version control
- **Encrypted Secret Management** - Secure credential storage with rotation support
- **Multi-Environment Isolation** - Separate execution contexts per environment
- **Dependency Resolution** - Automatic MCP tool validation and conflict detection

### 🛠️ **Enterprise Operations**
- **GitOps Workflows** - Standard git-based deployment pipelines
- **Audit & Compliance** - Complete deployment tracking and history
- **API Automation** - Full REST API for CI/CD integration
- **Team Management** - Role-based access with environment permissions

### 🌐 **Self-Hosted Architecture**
- **Complete Data Sovereignty** - No external dependencies beyond AI providers
- **Multi-Modal Access** - CLI, SSH/TUI, REST API, MCP server interfaces
- **Lightweight Runtime** - Single 45MB binary with SQLite database
- **Production Ready** - Queue-based execution, webhook notifications, monitoring

## Agent Template System

Station's template system enables packaging complete agent solutions as portable bundles:

### **Template Structure**
```
database-monitor/
├── bundle/
│   ├── manifest.json         # Metadata and MCP dependencies
│   ├── agent.json           # Agent config with {{variables}}
│   └── variables.schema.json # Variable validation
├── variables/
│   ├── development.json     # Dev environment values
│   ├── staging.yml         # Staging environment values  
│   └── production.enc      # Encrypted prod secrets
├── examples/
│   ├── api-install.json    # API deployment examples
│   └── basic-usage.md      # Documentation
└── README.md               # Template documentation
```

### **Template Lifecycle**
- **`stn agent bundle create`** - Generate template scaffolding
- **`stn agent bundle validate`** - Comprehensive validation with suggestions
- **`stn agent bundle install`** - Deploy with variable substitution
- **`stn agent bundle duplicate`** - Cross-environment deployment
- **`stn agent bundle export`** - Convert existing agents to templates

## MCP Tool Library

Station includes **20+ production-ready MCP server templates**:

| Category | Templates | Use Cases |
|----------|-----------|-----------|
| **Development** | GitHub, Git, Docker, SSH | Code management, CI/CD, deployments |
| **Databases** | PostgreSQL, MySQL, SQLite, MongoDB | Database administration, monitoring |
| **Cloud Services** | AWS CLI, Kubernetes, Terraform | Infrastructure management, deployments |
| **Communication** | Slack, Email, Webhooks | Notifications, alerts, team coordination |
| **Monitoring** | Prometheus, System metrics | Performance monitoring, alerting |
| **File Systems** | Local files, Network storage | File management, data processing |

```bash
# Load multiple tools for complex workflows
stn load examples/mcps/github.json
stn load examples/mcps/aws-cli.json  
stn load examples/mcps/slack.json

# Create agents that coordinate multiple MCP servers
stn agent create --name "DevOps Pipeline" \
  --description "GitHub → AWS → Slack deployment workflow"
```

## Target Users & Use Cases

### **Platform Engineering Teams**
- **Internal Tool Automation** - Agents for monitoring, deployments, maintenance
- **Multi-Environment Management** - Consistent agent behavior across environments
- **Secret Management** - Secure credential handling for internal system access

### **DevOps/SRE Teams**  
- **Infrastructure Monitoring** - Database, server, and application health checks
- **Incident Response** - Automated diagnostics and alert management
- **Deployment Automation** - CI/CD integration with proper credential management

### **Enterprise Development Teams**
- **Shared Agent Libraries** - Reusable templates across projects and teams
- **Compliance & Audit** - Version-controlled agent configurations with tracking
- **Security-First Development** - Encrypted secrets and controlled deployments

## Architecture & Security

### **Self-Hosted Security**
- **Data Sovereignty** - Complete control over agent configurations and secrets
- **Encrypted Storage** - AES encryption for sensitive variables and credentials
- **Environment Isolation** - Separate execution contexts prevent cross-contamination
- **Audit Logging** - Complete tracking of agent deployments and executions

### **Production Architecture**
- **Queue-Based Execution** - Asynchronous agent processing with worker pools
- **Multi-Modal Interface** - CLI, SSH/TUI, REST API, MCP server access
- **Database Persistence** - SQLite for development, PostgreSQL for production
- **Webhook Integration** - Real-time notifications and CI/CD system integration

## System Requirements

- **OS:** Linux, macOS, Windows
- **Memory:** 512MB RAM minimum, 2GB recommended for production
- **Storage:** 200MB for binary + templates, 1GB+ for production data
- **Database:** SQLite (included) or PostgreSQL for production scale
- **Network:** Outbound HTTPS for AI provider APIs and MCP communication

## Documentation & Support

- **📚 [Quick Start Guide](docs/QUICKSTART.md)** - Complete setup walkthrough
- **🎁 [Template Examples](examples/agent-templates/README.md)** - Production-ready patterns
- **🏗️ [Architecture Overview](docs/ARCHITECTURE.md)** - System design and security
- **🔒 [Security Guide](docs/SECURITY.md)** - Enterprise security practices
- **📖 [MCP Templates](examples/mcps/README.md)** - Available tool integrations

### Community & Support
- **🐛 [Issues](https://github.com/cloudshipai/station/issues)** - Bug reports and feature requests
- **💬 [Discord](https://discord.gg/station-ai)** - Community discussions and support
- **🏢 [Enterprise](mailto:enterprise@station.ai)** - Enterprise deployment and consulting

## License

Station is licensed under **AGPL-3.0** - enabling free use while keeping the platform open source.

**Simple Terms**: Use Station freely for any purpose. If you provide Station as a network service, share your modifications.

📖 **[License Philosophy](LICENSE_RATIONALE.md)** | 📄 **[Full License](LICENSE)**

---

**Station - GitOps for AI Agents**

*Deploy AI agents as reliably as infrastructure - with declarative configuration, secret management, and team collaboration.*