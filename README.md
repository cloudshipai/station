![Station](./image.png)

# Station - Lightweight Runtime for Deployable Sub-Agents

**A secure, self-hosted platform for building and deploying intelligent sub-agents.**

🌐 **[Browse Bundle Registry](https://cloudshipai.github.io/registry)** - Discover ready-to-use MCP bundles for Station

> Lightweight runtime for deployable sub-agents that need to access internal systems, run deep in your infrastructure, and integrate seamlessly with your existing deployment processes.

Station is purpose-built for **deployable sub-agents** - the intelligent automation you need for infrastructure monitoring, deployment pipelines, security scanning, and day-to-day tasks that require secure access to internal systems.

## Why Station Exists

When you need agents for internal work, you need more than application-focused agent platforms. You need:

- **Secure Internal Access** - Agents that can safely handle database credentials, API keys, and system-level access
- **Versionable Deployment** - Deployable agents that integrate with your existing deployment pipelines  
- **Team-Approved Tools** - Easy way to use and share the tools your team builds and approves
- **Low Footprint Runtime** - Lightweight system that blends into your infrastructure without overhead

**Station provides exactly this** - a lightweight, secure runtime specifically designed for deployable sub-agents.

## Core Value: Secure Deployable Sub-Agent Runtime

### 🔧 **Purpose-Built for Internal Tasks**

Unlike application-focused agent platforms, Station is designed for deployable sub-agents that need to:

- Access internal databases with production credentials
- Monitor infrastructure and alert on issues
- Automate deployment pipelines with CI/CD system access
- Perform security scans with elevated permissions
- Handle incident response with system-level tools

### 🔐 **Security by Design**

- **Self-Hosted** - Complete data sovereignty, no external dependencies beyond AI providers
- **Encrypted Secrets** - AES encryption for credentials and sensitive configuration
- **Environment Isolation** - Separate execution contexts for dev/staging/prod
- **Audit Trail** - Complete tracking of agent deployments and executions

### ⚡ **Lightweight & Integrated**

- **Single 45MB Binary** - No complex infrastructure or dependencies
- **SQLite Database** - Zero-setup local development, PostgreSQL for production
- **GitOps Ready** - Version-controlled agent configurations like infrastructure code
- **Existing Toolchain** - Uses your team's approved MCP tools and integrations

## Quick Start

### 1. Install Station
```bash
# Build from source
git clone https://github.com/cloudshipai/station
cd station && go build -o stn ./cmd/main

# Binary install (coming soon)
curl -sSL https://getstation.ai/install | bash
```

### 2. Initialize & Configure Tools
```bash
# Initialize runtime
stn init

# Load operational tools (two-step process)
stn load examples/mcps/aws-cli.json
stn load examples/mcps/database-postgres.json
stn load examples/mcps/slack.json

# Sync configurations to keep tools up-to-date
stn sync production
```

### 3. Create Deployable Sub-Agents
```bash
# Create a database monitoring agent
stn agent create \
  --name "Database Monitor" \
  --description "Monitor production database health and alert on issues"

# Create an infrastructure deployment agent  
stn agent create \
  --name "Deploy Pipeline" \
  --description "Automated deployment pipeline with rollback capability"

# Test locally
stn agent run 1 "Check database connection pool and alert if over 80% capacity"
```

### 4. Deploy with Version Control
```bash
# Export agents as versioned templates
stn agent export 1 ./ops-agents/db-monitor
stn agent export 2 ./ops-agents/deploy-pipeline

# Version control like infrastructure code
git add ops-agents/
git commit -m "Add production database monitoring agent"
git push origin main

# Deploy to production with encrypted secrets
stn template install ./ops-agents/db-monitor production \
  --vars-file secrets/production.enc
```

## Deployable Sub-Agent Use Cases

### **Infrastructure Monitoring**
```bash
# Database health monitoring with production credentials
stn agent create --name "DB Health Monitor" \
  --description "Monitor connection pools, query performance, disk usage"

# System resource monitoring across environments  
stn agent create --name "Resource Monitor" \
  --description "Monitor CPU, memory, disk across development and production"
```

### **Deployment Automation**
```bash
# CI/CD pipeline integration
stn agent create --name "Deployment Pipeline" \
  --description "Automated deployments with health checks and rollback"

# Infrastructure as Code management
stn agent create --name "Terraform Manager" \
  --description "Plan, apply, and manage infrastructure changes"
```

### **Security Operations**
```bash
# Vulnerability scanning and alerting
stn agent create --name "Security Scanner" \
  --description "Scan for vulnerabilities and compliance issues"

# Access monitoring and incident response
stn agent create --name "Incident Responder" \
  --description "Automated incident detection and initial response"
```

## MCP Tool Integration

Station includes **20+ production-ready tools for sub-agents**:

| **Operations** | **Tools** |
|----------------|-----------|
| **Infrastructure** | AWS CLI, Kubernetes, Docker, Terraform, SSH |
| **Databases** | PostgreSQL, MySQL, MongoDB, Redis, SQLite |
| **Monitoring** | Prometheus, Grafana, System metrics, Log analysis |
| **Communication** | Slack, PagerDuty, Email, Webhook notifications |
| **Security** | Vault, Certificate management, Access control |
| **Files** | Local files, Network storage, Configuration management |

```bash
# Load toolchains for sub-agents (two-step process)
stn load examples/mcps/infrastructure-suite.json
stn sync production  # Keep configurations in sync

# Load monitoring and alerting tools
stn load examples/mcps/monitoring-stack.json  
stn sync production

# Load security and compliance tools
stn load examples/mcps/security-tools.json
stn sync production
```

## Agent Templates for Teams

### **Template Structure**
```
ops-agents/database-monitor/
├── bundle/
│   ├── manifest.json         # MCP dependencies and metadata
│   ├── agent.json           # Agent configuration with variables
│   └── variables.schema.json # Variable validation schema
├── variables/
│   ├── development.json     # Dev environment values
│   ├── staging.json        # Staging environment values  
│   └── production.enc      # Encrypted production secrets
└── README.md               # Agent documentation
```

### **Team Workflow**
```bash
# Export working sub-agents
stn agent export 3 ./team-agents/db-monitor --analyze-vars

# Share across team with version control
git add team-agents/ && git commit -m "Add database monitoring agent v1.0"

# Team members deploy with their environment variables
stn template install ./team-agents/db-monitor staging \
  --vars-file ~/.station/secrets/staging.enc

# Invoke and trigger remote sub-agents via API or MCP
curl -X POST https://station.company.com/api/v1/agents/3/execute \
  -d '{"task": "Check database health and alert if issues found"}'

# Or trigger via MCP from other agents
stn agent run 5 "Coordinate with database monitor agent to run health check"
```

## Architecture

### **Lightweight Runtime**
- **45MB Binary** - Single executable with embedded SQLite
- **Low Memory** - 512MB minimum, optimized for container deployment
- **Multi-Access** - CLI, SSH/TUI, REST API, MCP server interfaces

### **Production Ready**
- **Queue-Based Execution** - Asynchronous processing with worker pools
- **Webhook Integration** - Real-time notifications to existing systems
- **Database Replication** - Litestream integration for production backup
- **Environment Isolation** - Complete separation of dev/staging/prod contexts
- **Enhanced AI Providers** - Custom OpenAI plugin with multi-turn conversation fixes

### **Security Architecture**
- **Self-Hosted** - No external dependencies beyond AI provider APIs
- **Encrypted Storage** - AES encryption for secrets and sensitive variables
- **Audit Logging** - Complete deployment and execution tracking
- **Access Controls** - Role-based permissions with environment boundaries

## System Requirements

- **OS:** Linux, macOS, Windows
- **Memory:** 512MB RAM minimum, 1GB recommended
- **Storage:** 200MB for runtime, 1GB+ for production agent data
- **Database:** SQLite (development) or PostgreSQL (production)
- **Network:** Outbound HTTPS for AI providers and tool integrations

## Documentation

- **[Database Replication](docs/DATABASE-REPLICATION.md)** - Production backup with Litestream
- **[MCP Tool Library](examples/mcps/README.md)** - Available tool integrations
- **[Agent Templates](examples/agent-templates/README.md)** - Sub-agent patterns and examples

## Community

- **[Issues](https://github.com/cloudshipai/station/issues)** - Bug reports and feature requests
- **[Discord](https://discord.gg/station-ai)** - Community support and discussions

## License

**AGPL-3.0** - Free for all use, open source contributions welcome.

---

**Station - Lightweight Runtime for Deployable Sub-Agents**

*Secure, versionable, deployable sub-agents for your infrastructure.*