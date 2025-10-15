# Architecture

Station is an open-source runtime for executing AI agents on your own infrastructure with full control over data and tools.

## Overview

Station is designed around a core principle: **trust through control**. Instead of sending your credentials and data to third-party platforms, Station runs entirely on your infrastructure while providing enterprise-grade agent orchestration capabilities.

### Core Design Goals

**1. Security First**
- Agents run in isolated environments with explicit tool access
- MCP servers connect directly to your infrastructure (AWS, GCP, databases)
- Credentials never leave your environment
- Fine-grained tool permissions (read vs write operations)

**2. GitOps-Ready Configuration**
- All configuration stored as files (not in external databases)
- Go template variables for environment-specific deployments
- Version control friendly (commit agent definitions with your code)
- Reproducible deployments across dev/staging/prod

**3. Multi-Environment Isolation**
- Separate tool configurations per environment (dev uses dev AWS, prod uses prod AWS)
- Independent agent definitions and variable resolution
- Safe experimentation without affecting production

**4. Multiple Deployment Modes**
- **Server Mode** (`stn up`): Full-featured with web UI, API, and MCP servers
- **Stdio Mode** (`stn stdio`): Claude Desktop/Cursor integration via stdio protocol
- **Standalone Binary**: Single executable with no external dependencies
- **Docker/Kubernetes**: Containerized deployments with zero-config credential discovery

## Components

### 1. Agent Execution Engine

Station's agent engine is built on Firebase GenKit with custom extensions for MCP tool integration.

**Key Features:**
- **Dotprompt Format**: Declarative YAML-based agent definitions
- **Multi-Step Reasoning**: Agents can take multiple tool calls to complete tasks (configurable via `max_steps`)
- **Streaming Execution**: Real-time progress updates during agent runs
- **Run Tracking**: Complete execution metadata (tool calls, tokens used, duration)

**How It Works:**
```
User Request → Agent Selection → GenKit Executor
  ↓
Tool Resolution (from MCP servers) → Agent Execution
  ↓
Tool Calls → Results → Agent Reasoning → More Tools or Final Answer
  ↓
Structured Output (with metadata)
```

### 2. MCP Server Integration

Station uses the Model Context Protocol (MCP) to give agents access to tools without hardcoding integrations.

**Architecture:**
- MCP servers run as child processes managed by Station
- Each server provides tools via JSON-RPC protocol
- Station aggregates tools from multiple servers
- Tools are namespaced by server (`__filesystem__read_file`, `__aws__get_cost_and_usage`)

**Benefits:**
- **Ecosystem**: Use any MCP server (AWS, Stripe, Grafana, filesystem, security tools)
- **Isolation**: Servers can't access each other's data
- **Composability**: Mix and match tools from different servers per agent

**Connection Lifecycle:**
```
stn up → Parse template.json → Start MCP Servers
  ↓
Discover Tools → Register in GenKit → Agent Creation
  ↓
Agent Execution → Tool Calls via MCP → Results
  ↓
stn down → Graceful MCP Shutdown
```

### 3. Template Variable System

Station uses Go templates to inject environment-specific configuration at runtime.

**Why Templates?**
- Never commit secrets to git (`{{ .AWS_ACCESS_KEY }}` instead of actual keys)
- Same agent definition works across environments
- Deploy-time configuration resolution
- Support for nested templates in complex configs

**Variable Resolution:**
```
Agent Load → Parse .prompt file → Find {{ .VARIABLES }}
  ↓
Load variables.yml → Resolve values → MCP Server Config
  ↓
Start Servers with Resolved Config → Agent Execution
```

**Example:**
```json
{
  "mcpServers": {
    "aws": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-aws"],
      "env": {
        "AWS_REGION": "{{ .AWS_REGION }}",
        "AWS_PROFILE": "{{ .AWS_PROFILE }}"
      }
    }
  }
}
```

At runtime, `{{ .AWS_REGION }}` is replaced with the value from `variables.yml`.

### 4. Multi-Environment Management

Environments provide complete isolation for different deployment contexts.

**File Structure:**
```
~/.config/station/environments/
├── dev/
│   ├── agents/           # Agent .prompt files
│   ├── template.json     # MCP server configs
│   └── variables.yml     # Dev-specific variables
├── staging/
│   ├── agents/
│   ├── template.json
│   └── variables.yml     # Staging-specific variables
└── prod/
    ├── agents/
    ├── template.json
    └── variables.yml       # Prod-specific variables
```

**Environment Switching:**
```bash
# Run agent in dev environment
stn agent run "Cost Analyzer" "analyze EC2 costs" --env dev

# Same agent, prod environment (different AWS account)
stn agent run "Cost Analyzer" "analyze EC2 costs" --env prod
```

## Data Flow

### Agent Execution Flow

```
1. User Submits Task
   ↓
2. Load Agent Definition (.prompt file)
   ↓
3. Load Environment Config (template.json + variables.yml)
   ↓
4. Start MCP Servers (if not running)
   ↓
5. Create GenKit Agent with MCP Tools
   ↓
6. Execute Agent (streaming)
   ↓
7. Agent Calls Tools → MCP Servers → Infrastructure (AWS, DB, etc.)
   ↓
8. Agent Processes Results → More Tools or Final Answer
   ↓
9. Save Run Metadata (tool calls, tokens, duration)
   ↓
10. Return Structured Output
```

### MCP Tool Call Flow

```
Agent Decides to Call Tool
   ↓
Station Looks Up Tool by Name
   ↓
Find MCP Server for Tool
   ↓
Send JSON-RPC Request to Server
   ↓
Server Executes (reads AWS API, database, filesystem, etc.)
   ↓
Return Results to Agent
   ↓
Agent Continues Reasoning
```

## Security Model

### Principle of Least Privilege

Agents only get access to tools explicitly listed in their .prompt file:

```yaml
tools:
  - "__get_cost_and_usage"      # Read-only AWS Cost Explorer
  - "__list_cost_allocation_tags"  # Read cost tags
  # No write permissions - agent can analyze but not modify
```

### Credential Isolation

**Development:**
- Credentials stored in `variables.yml` (gitignored)
- Template variables prevent accidental secret commits
- MCP servers inherit credentials from environment

**Production:**
- Zero-config credential discovery (IAM roles, service accounts)
- No credentials in configuration files
- Automatic credential rotation support

### Audit Trail

Every agent execution is tracked:
- Which agent ran
- Which tools were called with what parameters
- Full execution steps and reasoning
- Token usage and costs
- Execution duration
- Success/failure status

Query runs:
```bash
stn runs list --agent "Cost Analyzer"
stn runs inspect <run-id> --verbose
```

## Why These Design Decisions?

### Why File-Based Configuration?

**GitOps Workflow**: Configuration stored alongside code, version controlled, reviewable via PRs.

**Reproducibility**: Same configuration deploys identically across environments.

**No External Dependencies**: No database required for configuration (though SQLite used for run tracking).

### Why Go Templates?

**Industry Standard**: Familiar to anyone using Helm, Terraform, or other DevOps tools.

**Powerful**: Supports conditionals, loops, functions for complex configs.

**Safe**: Variables are resolved at runtime, never stored in plain text.

### Why Multiple Deployment Modes?

**Flexibility**: Use Station however it fits your workflow:
- `stn up` for full-featured local development
- `stn stdio` for Claude Desktop/Cursor integration
- `stn serve` for production API deployments
- Docker for containerized environments

**No Lock-In**: Switch between modes without changing agent definitions.

### Why MCP Instead of Direct Integrations?

**Ecosystem**: Leverage hundreds of existing MCP servers instead of building integrations.

**Community**: MCP is an open standard with growing adoption.

**Composability**: Mix tools from different MCP servers in a single agent.

**Security**: Fine-grained tool permissions (select exactly which tools each agent gets).

## Next Steps

- [Installation Guide](./installation.md) - Get Station running
- [Agent Development](./agent-development.md) - Write your first agent
- [MCP Tools](./mcp-tools.md) - Understand tool integration
- [Examples](./examples.md) - Real-world agent examples
