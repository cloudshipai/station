![Station](./image.png)

# Station - AI Agent Template Platform

**Build reusable AI agent templates that work across environments.**

> Early access version v0.2.0 with Agent Template System - production-ready for enterprise deployment

Station solves the biggest problems in AI agent deployment: **configuration drift across environments, inability to package complete agent setups, lack of reusable patterns, and complex multi-environment management.**

## What Station Actually Is

Station is a **template-driven AI agent platform** that lets you:
- **Package complete AI agents** as reusable templates with dependencies
- **Deploy consistently across environments** using GitOps workflows  
- **Share agent patterns** across teams and organizations
- **Manage multi-environment complexity** with variable-driven configuration

Think "Terraform for AI Agents" - but easier.

## Core Value Propositions

### 🎁 **Agent Templates - Package Complete AI Solutions**

**Stop rebuilding agents from scratch. Create once, deploy everywhere.**

Traditional platforms require rebuilding agent configurations manually. Station lets you package complete AI agents as portable templates with all dependencies included.

```bash
# Create a comprehensive agent template
stn agent bundle create ./database-monitor \
  --name "Database Monitor" \
  --author "Platform Team" \
  --description "PostgreSQL monitoring with Slack alerts"

# Template includes:
# ✅ Agent configuration (prompts, tools, settings)
# ✅ MCP dependencies (database + notification tools)
# ✅ Variable schema (DB_HOST, SLACK_WEBHOOK, etc.)
# ✅ Deployment examples and documentation

# Install anywhere with environment-specific variables
stn agent bundle install ./database-monitor --vars-file prod-vars.json --env production
stn agent bundle install ./database-monitor --vars-file dev-vars.json --env development

# Result: Identical agent behavior, environment-specific configuration
```

### 🔄 **Multi-Environment Consistency**

**Deploy the same agent logic across dev/staging/prod with environment-specific variables.**

Stop maintaining separate agent configurations per environment. Use one template with variable substitution.

```bash
# Single template with variables: {{.DB_HOST}}, {{.ALERT_THRESHOLD}}

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

### 📦 **Complete Dependency Management**

**Include MCP tool dependencies in templates. No more "works on my machine".**

Package all required MCP servers and tools with your agent templates. Recipients get everything they need.

```bash
# Template manifest.json includes ALL dependencies:
{
  "name": "web-scraper",
  "mcp_dependencies": [
    {"name": "playwright-tools", "version": "^1.0", "required": true},
    {"name": "http-client", "version": "^2.1", "required": true}
  ],
  "tool_requirements": [
    {"name": "browser_navigate", "server": "playwright-tools", "required": true},
    {"name": "http_post", "server": "http-client", "required": true}
  ]
}

# Installation automatically resolves and installs dependencies
stn agent bundle install ./web-scraper --env production

# Station automatically:
# ✅ Validates all required MCP servers are available
# ✅ Checks tool compatibility and versions
# ✅ Resolves conflicts between different templates
# ✅ Installs missing dependencies

# Result: Zero dependency hell, guaranteed working environments
```

### 🤝 **Enterprise Sharing & Collaboration**

**Share agent templates across teams with GitOps workflows and API deployment.**

Version control agent templates like infrastructure code. Deploy via API for automation.

```bash
# Export existing agent as template
stn agent bundle export 5 ./shared-templates/log-analyzer --include-deps --analyze-vars

# Version control with GitOps
git add shared-templates/log-analyzer/
git commit -m "Add log analyzer template v1.2"
git push origin main

# Other teams install from repository
git pull && stn agent bundle install ./shared-templates/log-analyzer --vars-file our-vars.json

# API deployment for automation
curl -X POST http://station.company.com/api/v1/agents/templates/install \
  -H "Content-Type: application/json" \
  -d '{
    "bundle_path": "./shared-templates/log-analyzer",
    "environment": "production", 
    "variables": {
      "LOG_DIRECTORY": "/var/log/app",
      "ALERT_EMAIL": "ops@company.com"
    }
  }'

# Result: Enterprise-grade sharing, audit trails, automated deployments
```

## Quick Start (5 Minutes)

### 1. Install Station
```bash
# Build from source
git clone https://github.com/anthropics/station
cd station && go build -o stn ./cmd/main

# Binary install (coming soon)
curl -sSL https://getstation.ai/install | bash
```

### 2. Initialize with MCP Tools
```bash
# Initialize Station
stn init

# Load filesystem tools for basic operations
stn load examples/mcps/filesystem.json

# Load additional tools as needed
stn load examples/mcps/github.json
stn load examples/mcps/slack.json
```

### 3. Create Your First Agent Template

```bash
# Create a new template bundle
stn agent bundle create ./my-first-template \
  --name "File Manager" \
  --author "Your Name" \
  --description "Intelligent file management agent"

# Edit the generated files:
# - bundle/manifest.json (metadata and dependencies)
# - bundle/agent.json (agent configuration with {{variables}})
# - bundle/variables.schema.json (variable definitions)

# Validate your template
stn agent bundle validate ./my-first-template

# Install with interactive variable collection
stn agent bundle install ./my-first-template --interactive --env development
```

### 4. Deploy and Share

```bash
# Export existing agent as template
stn agent bundle export 1 ./shared-templates/my-agent --include-deps

# Share via version control
git add shared-templates/
git commit -m "Add reusable agent template"

# Install from shared template
stn agent bundle install ./shared-templates/my-agent --vars-file production-vars.json
```

## Agent Template System Features

Station's **Agent Template System** provides enterprise-grade template management:

### 📋 **Complete Template Lifecycle**
- **Create**: `stn agent bundle create` - Generate template scaffolding
- **Validate**: `stn agent bundle validate` - Comprehensive validation with suggestions  
- **Install**: `stn agent bundle install` - Deploy with variable substitution
- **Export**: `stn agent bundle export` - Convert existing agents to templates
- **Duplicate**: `stn agent bundle duplicate` - Cross-environment deployment

### 🔧 **Advanced Variable System**
- **Type Preservation**: JSON/YAML variables maintain types (strings, numbers, booleans)
- **Interactive Mode**: `--interactive` prompts for missing variables with validation
- **File-Based Variables**: `--vars-file` for repeatable deployments
- **Sensitive Variables**: Masked input for passwords, API keys, secrets
- **Default Values**: Fallback configuration for optional variables
- **Variable Schema**: JSON Schema validation for type safety

### 🌐 **Multi-Environment Excellence**
- **Environment Isolation**: Separate configuration per environment
- **Variable Hierarchies**: CLI flags → vars-file → defaults → schema defaults
- **Template Rendering**: Go template engine with conditional logic
- **Dependency Resolution**: Environment-specific tool availability checking
- **Cross-Environment Deployment**: `stn agent bundle duplicate` across environments

### 🛠️ **Production-Ready APIs**
- **Template Installation**: `POST /api/v1/agents/templates/install`
- **Agent Management**: `POST /api/v1/agents` (direct creation)
- **Agent Execution**: `POST /api/v1/agents/:id/execute`
- **Comprehensive Validation**: Go struct + JSON Schema validation
- **Error Handling**: Detailed error responses with suggestions

## Rich Template Library

Station includes **20+ production-ready MCP server templates** for immediate use:

| Category | Templates | Use Cases |
|----------|-----------|-----------|
| **Development** | GitHub, Git, Docker, SSH | Code management, CI/CD, deployments |
| **Databases** | PostgreSQL, MySQL, SQLite, MongoDB, Redis | Database administration, monitoring |
| **Cloud Services** | AWS CLI, Kubernetes, Terraform | Infrastructure management, deployments |
| **Communication** | Slack, Email, Webhooks | Notifications, alerts, team coordination |
| **Monitoring** | Prometheus, System metrics | Performance monitoring, alerting |
| **File Systems** | Local files, Network storage | File management, data processing |

```bash
# Browse available templates
ls examples/mcps/

# Load multiple tools for complex workflows
stn load examples/mcps/github.json
stn load examples/mcps/aws-cli.json  
stn load examples/mcps/slack.json

# Create agents that use multiple MCP servers
stn agent create --name "DevOps Assistant" --description "GitHub → AWS → Slack workflow automation"
```

## Enterprise Architecture

Station is designed for enterprise deployment with:

### 🔒 **Security & Compliance**
- **Environment Isolation**: Separate agent execution per environment
- **Secret Management**: Encrypted variable storage with rotation
- **Audit Logging**: Complete deployment and execution tracking
- **Access Controls**: Role-based permissions and API authentication

### 📈 **Scalability & Performance**  
- **Database Persistence**: SQLite for development, PostgreSQL for production
- **Queue-Based Execution**: Async agent execution with status tracking
- **Resource Management**: Memory and execution time limits
- **Webhook Integration**: Real-time notifications and integrations

### 🔄 **DevOps Integration**
- **GitOps Workflows**: Version-controlled template management
- **CI/CD Integration**: Automated template validation and deployment
- **API-First Design**: Full automation via REST APIs
- **Multi-Environment**: Dev → Staging → Production promotion workflows

## Template Examples

Station includes comprehensive template examples in `examples/agent-templates/`:

- **[Basic Agent](examples/agent-templates/basic-agent/)** - Simple file management template
- **[Web Scraper](examples/agent-templates/web-scraper/)** - API integration with sensitive variables
- **[Data Processor](examples/agent-templates/data-processor/)** - Complex variable types and validation
- **[API Integration](examples/agent-templates/api-integration/)** - Production-ready enterprise pattern
- **[Multi-Environment](examples/agent-templates/multi-environment/)** - Complete GitOps workflow

Each example includes complete documentation, variable files, API payloads, and deployment scripts.

## Target Users & Use Cases

### **Platform Engineering Teams**
- **Template Standardization**: Create reusable agent patterns across organization
- **Environment Consistency**: Deploy identical logic across dev/staging/prod
- **Dependency Management**: Package complete agent solutions with all requirements

### **DevOps/SRE Teams**  
- **Infrastructure Automation**: Database monitoring, alert management, deployment automation
- **Multi-Environment Management**: Consistent agent behavior across environments
- **GitOps Integration**: Version-controlled agent templates with audit trails

### **Enterprise Development Teams**
- **Agent Sharing**: Reusable templates across projects and teams
- **API Integration**: Automated agent deployment in CI/CD pipelines
- **Security Compliance**: Standardized, auditable agent configurations

### **Managed Service Providers**
- **Client Deployments**: Reusable agent templates across client environments
- **White-Label Solutions**: Customizable agent templates with client-specific variables
- **Operational Excellence**: Standardized monitoring and management patterns

## System Requirements

- **OS:** Linux, macOS, Windows
- **Memory:** 512MB RAM minimum, 2GB recommended for production
- **Storage:** 200MB for binary + templates, 1GB+ for production data
- **Database:** SQLite (included) or PostgreSQL for production
- **Network:** Outbound HTTPS for AI provider APIs and MCP tool communication

## Documentation & Support

- **📚 [Quick Start Guide](docs/QUICKSTART.md)** - Get running in 5 minutes
- **🎁 [Template System Guide](examples/agent-templates/README.md)** - Complete template documentation
- **🏗️ [Architecture Overview](docs/ARCHITECTURE.md)** - System design and components  
- **🔒 [Security Guide](docs/SECURITY.md)** - Enterprise security practices
- **📖 [MCP Templates](examples/mcps/README.md)** - Available MCP server templates

### Community & Support
- **🐛 [Issues](https://github.com/anthropics/station/issues)** - Bug reports and feature requests
- **💬 [Discord](https://discord.gg/station-ai)** - Community discussions
- **🏢 [Enterprise](mailto:enterprise@station.ai)** - Enterprise support and consulting

## License

Station is licensed under **AGPL-3.0** - enabling free use while keeping the platform open source.

**Simple Terms**: Use Station freely for any purpose. If you provide Station as a network service, share your modifications.

📖 **[License Philosophy](LICENSE_RATIONALE.md)** | 📄 **[Full License](LICENSE)**

---

**Station - Build reusable AI agent templates that work across environments.**

*The only platform purpose-built for enterprise AI agent template management and deployment.*