![Station](./station-logo.png)

# Station

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
# Set your OpenAI API key
export OPENAI_API_KEY=sk-your-key-here

# Start Station (automatically configures .mcp.json for Claude Code/Cursor)
stn up --provider openai
```

**More provider options:**
```bash
# OpenAI with specific model
stn up --provider openai --model gpt-4o

# Anthropic Claude
stn up --provider anthropic --api-key sk-ant-...

# Google Gemini
stn up --provider gemini --api-key your-key --model gemini-2.0-flash-exp

# Custom provider (Ollama, etc.)
stn up --provider custom --base-url http://localhost:11434/v1 --model llama3.2

# With CloudShip registration for centralized management
stn up --provider openai --cloudshipai-registration-key your-registration-key
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

## License

**Apache 2.0** - Free for all use, open source contributions welcome.

---

**Station - Open-Source Runtime for Infrastructure Management Agents**

*Deploy AI agents on your infrastructure. Keep data secure. Maintain control.*
