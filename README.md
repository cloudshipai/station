![Station](./station-logo.png)

# Station

[![Test Coverage](https://img.shields.io/badge/coverage-52.7%25-yellow?style=flat-square)](./TESTING_PROGRESS.md) [![Go Tests](https://img.shields.io/badge/tests-passing-brightgreen?style=flat-square)](./.github/workflows/ci.yml)

**Open-Source Runtime for Infrastructure Management Agents**

Deploy AI agents on your infrastructure. Keep sensitive data secure. Maintain full control.

[Quick Start](#quick-start) | [Documentation](./docs/station/) | [Examples](./docs/station/examples.md)

---

## Why Station?

AI agents can automate infrastructure management—cost optimization, security compliance, deployments—but most solutions require sharing credentials and sensitive data with third-party platforms.

**Station gives you control:**
- ✅ **Run on your infrastructure** - Deploy agents wherever you need them (AWS, GCP, on-prem, local)
- ✅ **Keep data private** - Agents access your tools directly, no data leaves your environment
- ✅ **Simple agent development** - Declarative dotprompt format, develop and test locally
- ✅ **Fine-grained security** - Control exactly which tools each agent can use (read vs write)
- ✅ **Share and collaborate** - Bundle agents with MCP configs for easy distribution
- ✅ **Open source** - Full transparency, audit the code yourself

[Learn more about Station's architecture →](./docs/station/architecture.md)

---

## How Simple Are Agents?

Here's a complete FinOps agent in dotprompt format:

```yaml
---
metadata:
  name: "AWS Cost Spike Analyzer"
  description: "Detects unusual cost increases and identifies root causes"
model: gpt-4o-mini
max_steps: 5
tools:
  - "__get_cost_and_usage"      # AWS Cost Explorer - read only
  - "__list_cost_allocation_tags"
  - "__get_savings_plans_coverage"
---

{{role "system"}}
You are a FinOps analyst specializing in AWS cost anomaly detection.
Analyze cost trends, identify spikes, and provide actionable recommendations.

{{role "user"}}
{{userInput}}
```

That's it. Station handles:
- MCP tool connections (AWS Cost Explorer, Stripe, Grafana, etc.)
- Template variables for secrets/config (`{{ .AWS_REGION }}`)
- Multi-environment isolation (dev/staging/prod)
- Execution tracking and structured outputs

[See more agent examples →](./docs/station/examples.md)

---

## Quick Start

### 1. Install Station
```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

### 2. Start Station
```bash
# Start Station with your OpenAI API key (automatically configures .mcp.json for Claude Code/Cursor)
stn up --provider openai --api-key sk-your-key-here
```

**More provider options:**
```bash
# OpenAI with specific model
stn up --provider openai --api-key sk-your-key-here --model gpt-4o

# Anthropic Claude
stn up --provider anthropic --api-key sk-ant-...

# Google Gemini
stn up --provider gemini --api-key your-key --model gemini-2.0-flash-exp

# Custom provider (Ollama, etc.)
stn up --provider custom --base-url http://localhost:11434/v1 --model llama3.2

# Alternative: Use environment variable instead of --api-key flag
export OPENAI_API_KEY=sk-your-key-here
stn up --provider openai

# With CloudShip registration for centralized management
stn up --provider openai --api-key sk-your-key-here --cloudshipai-registration-key your-registration-key
```

**Stop Station:**
```bash
stn down
```

**That's it!** Station is now running with:
- ✅ Web UI at `http://localhost:8585` for managing tools, bundles, and builds
- ✅ MCP server at `http://localhost:8586/mcp` configured for Claude Code/Cursor
- ✅ Dynamic Agent MCP at `http://localhost:3030/mcp`
- ✅ `.mcp.json` automatically created for seamless Claude integration

[Full installation guide →](./docs/station/installation.md)

---

## Development Workflow

Station provides a complete agent development workflow using Claude Code or Cursor:

### 1. Add MCP Tools (via UI)
Open the Web UI at `http://localhost:8585`:
- Browse available MCP servers (AWS, Stripe, Grafana, filesystem, security tools)
- Add MCP tools to your environment
- Configure template variables for secrets

### 2. Connect Claude Code/Cursor
Station automatically creates `.mcp.json` when you run `stn up`:
```json
{
  "mcpServers": {
    "station": {
      "type": "http",
      "url": "http://localhost:8586/mcp"
    }
  }
}
```

Restart Claude Code/Cursor to connect to Station.

### 3. Create & Manage Agents (via Claude)
Use Claude Code/Cursor with Station's MCP tools to:
- **Create agents** - Write dotprompt files with agent definitions
- **Run agents** - Execute agents and see results in real-time
- **List agents** - View all agents in your environments
- **Update agents** - Modify agent configs and tools
- **Create environments** - Set up dev/staging/prod isolation
- **Sync environments** - Apply changes and resolve variables

Example interaction with Claude:
```
You: "Create a FinOps agent that analyzes AWS costs using the cost explorer tools"

Claude: [Uses Station MCP tools to create agent with proper dotprompt format]

You: "Run the agent to analyze last month's costs"

Claude: [Executes agent and shows cost analysis results]
```

### 4. Bundle & Deploy (via UI)
Back to the Web UI at `http://localhost:8585`:
- **Create bundles** - Package agents + MCP configs for distribution
- **Share bundles** - Export bundles to share with team
- **Build Docker images** - Create production containers from environments
- **Install bundles** - Import bundles from registry or files

[Agent Development Guide →](./docs/station/agent-development.md) | [Bundling & Distribution →](./docs/station/bundles.md)

---

## CICD Deployments

Run Station agents in your CICD pipelines for automated security scanning, cost analysis, compliance checks, and deployment validation.

**Pre-built agent containers available:**
- `ghcr.io/cloudshipai/station-security:latest` - Infrastructure, container, and code security scanning
- `ghcr.io/cloudshipai/station-finops:latest` - Cost analysis and optimization (coming soon)
- `ghcr.io/cloudshipai/station-compliance:latest` - Policy and compliance validation (coming soon)

### Quick Start: GitHub Actions

```yaml
- uses: cloudshipai/station-action@v1
  with:
    agent: "Infrastructure Security Auditor"
    task: "Scan terraform and kubernetes for security issues"
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

That's it! 3 lines to add AI-powered security scanning to your pipeline.

### Supported CICD Platforms

Station agents work across all major CICD platforms using the same Docker containers:

- **GitHub Actions** - Composite action with 3-line setup
- **GitLab CI** - Docker-based pipeline stages
- **Jenkins** - Docker agent pipelines
- **CircleCI** - Docker executor workflows
- **Argo Workflows** - Kubernetes-native workflows
- **Tekton** - Cloud-native CI/CD pipelines

**Complete deployment templates and guides available in [`deployments/`](./deployments/):**
- Platform-specific configuration examples
- Secret management guides (OpenAI API keys, CloudShip registration)
- Scheduled execution setups (daily cost analysis, compliance scans)
- Multi-agent pipeline orchestration

### Use Cases Beyond Code Scanning

**FinOps & Cost Management:**
```yaml
# Daily cost analysis on schedule
schedule: "0 9 * * *"  # 9 AM daily
agent: "AWS Cost Analyzer"
task: "Analyze yesterday's AWS spend and identify optimization opportunities"
```

**Compliance Monitoring:**
```yaml
# Weekly compliance checks
schedule: "0 0 * * 1"  # Monday midnight
agent: "SOC2 Compliance Auditor"
task: "Verify infrastructure meets SOC2 requirements"
```

**Platform Engineering:**
```yaml
# Post-deployment validation
agent: "Deployment Validator"
task: "Verify deployment health, check metrics, and validate configuration"
```

[Complete CICD Integration Guide →](./deployments/README.md) | [Secrets Management →](./deployments/SECRETS.md)

---

## MCP Tools & Templates

Station uses the Model Context Protocol (MCP) to give agents access to tools—AWS APIs, databases, filesystems, security scanners, and more.

**Fine-grained control over agent capabilities:**
```yaml
tools:
  - "__get_cost_and_usage"          # AWS Cost Explorer - read only
  - "__list_cost_allocation_tags"   # Read cost tags
  - "__read_text_file"              # Filesystem read
  # No write permissions - agent can analyze but not modify
```

**Template variables for secure configuration:**
```json
{
  "mcpServers": {
    "aws-cost-explorer": {
      "command": "mcp-server-aws",
      "env": {
        "AWS_REGION": "{{ .AWS_REGION }}",
        "AWS_PROFILE": "{{ .AWS_PROFILE }}"
      }
    }
  }
}
```

Variables are resolved at runtime from `variables.yml`—never hardcoded in configs.

[MCP Tools Documentation →](./docs/station/mcp-tools.md) | [Template Variables Guide →](./docs/station/templates.md)

---

## OpenAPI MCP Servers

Station can automatically convert OpenAPI/Swagger specifications into MCP servers, making any REST API instantly available as agent tools.

**Turn any OpenAPI spec into MCP tools:**
```json
{
  "name": "Station Management API",
  "description": "Control Station via REST API",
  "mcpServers": {
    "station-api": {
      "command": "stn",
      "args": [
        "openapi-runtime",
        "--spec",
        "environments/{{ .ENVIRONMENT_NAME }}/station-api.openapi.json"
      ]
    }
  },
  "metadata": {
    "openapiSpec": "station-api.openapi.json",
    "variables": {
      "STATION_API_URL": {
        "description": "Station API endpoint URL",
        "default": "http://localhost:8585/api/v1"
      }
    }
  }
}
```

**Template variables in OpenAPI specs:**
```json
{
  "openapi": "3.0.0",
  "servers": [
    {
      "url": "{{ .STATION_API_URL }}",
      "description": "Station API endpoint"
    }
  ]
}
```

Station automatically:
- ✅ **Converts OpenAPI paths to MCP tools** - Each endpoint becomes a callable tool
- ✅ **Processes template variables** - Resolves `{{ .VAR }}` from `variables.yml` and env vars
- ✅ **Supports authentication** - Bearer tokens, API keys, OAuth
- ✅ **Smart tool sync** - Detects OpenAPI spec updates and refreshes tools

**Example: Station Admin Agent**

Create an agent that manages Station itself using the Station API:

```yaml
---
metadata:
  name: "Station Admin"
  description: "Manages Station environments, agents, and MCP servers"
model: gpt-4o-mini
max_steps: 10
tools:
  - "__listEnvironments"    # From station-api OpenAPI spec
  - "__listAgents"
  - "__listMCPServers"
  - "__createAgent"
  - "__executeAgent"
---

{{role "system"}}
You are a Station administrator that helps manage environments, agents, and MCP servers.

Use the Station API tools to:
- List and inspect environments, agents, and MCP servers
- Create new agents from user requirements
- Execute agents and monitor their runs
- Provide comprehensive overviews of the Station deployment

{{role "user"}}
{{userInput}}
```

**Usage:**
```bash
stn agent run station-admin "Show me all environments and their agents"
```

The agent will use the OpenAPI-generated tools to query the Station API and provide a comprehensive overview.

[OpenAPI MCP Documentation →](./docs/station/openapi-mcp-servers.md) | [Station Admin Agent Guide →](./docs/station/station-admin-agent.md)

---

## Zero-Config Deployments

Deploy Station agents to production without manual configuration. Station supports zero-config deployments that automatically:
- Discover cloud credentials and configuration
- Set up MCP tool connections
- Deploy agents with production-ready settings

**Deploy to Docker Compose:**
```bash
# Build environment container
stn build env production

# Deploy with docker-compose
docker-compose up -d
```

Station automatically configures:
- AWS credentials from instance role or environment
- Database connections from service discovery
- MCP servers with template variables resolved

**Supported platforms:**
- Docker / Docker Compose
- AWS ECS
- Kubernetes
- AWS Lambda (coming soon)

[Zero-Config Deployment Guide →](./docs/station/zero-config-deployments.md) | [Docker Compose Examples →](./docs/station/docker-compose-deployments.md)

---

## Use Cases

**FinOps & Cost Optimization:**
- Cost spike detection and root cause analysis
- Reserved instance utilization tracking
- Multi-cloud cost attribution
- COGS analysis for SaaS businesses

**Security & Compliance:**
- Infrastructure security scanning
- Compliance violation detection
- Secret rotation monitoring
- Vulnerability assessments

**Deployment & Operations:**
- Automated deployment validation
- Performance regression detection
- Incident response automation
- Change impact analysis

[See Example Agents →](./docs/station/examples.md)

---

## System Requirements

- **OS:** Linux, macOS, Windows
- **Memory:** 512MB minimum, 1GB recommended
- **Storage:** 200MB for binary, 1GB+ for agent data
- **Network:** Outbound HTTPS for AI providers

---

## Resources

- 📚 **[Documentation](./docs/station/)** - Complete guides and tutorials
- 🐛 **[Issues](https://github.com/cloudshipai/station/issues)** - Bug reports and feature requests
- 💬 **[Discord](https://discord.gg/station-ai)** - Community support

---

## For Contributors

If you're interested in contributing to Station or understanding the internals, comprehensive architecture documentation is available in the [`docs/architecture/`](./docs/architecture/) directory:

- **[Architecture Index](./docs/architecture/ARCHITECTURE_INDEX.md)** - Quick navigation and key concepts reference
- **[Architecture Diagrams](./docs/architecture/ARCHITECTURE_DIAGRAMS.md)** - Complete ASCII diagrams of all major systems and services
- **[Architecture Analysis](./docs/architecture/ARCHITECTURE_ANALYSIS.md)** - Deep dive into design decisions and component organization
- **[Component Interactions](./docs/architecture/COMPONENT_INTERACTIONS.md)** - Detailed sequence diagrams for key workflows

These documents provide a complete understanding of Station's four-layer architecture, 43+ service modules, database schema, API endpoints, and execution flows.

---

## License

**Apache 2.0** - Free for all use, open source contributions welcome.

---

**Station - Open-Source Runtime for Infrastructure Management Agents**

*Deploy AI agents on your infrastructure. Keep data secure. Maintain control.*
