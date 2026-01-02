![Station](./station-logo.png)

# Station - AI Agent Orchestration Platform

[![Test Coverage](https://img.shields.io/badge/coverage-52.7%25-yellow?style=flat-square)](./TESTING_PROGRESS.md) [![Go Tests](https://img.shields.io/badge/tests-passing-brightgreen?style=flat-square)](./.github/workflows/ci.yml)

**Build, test, and deploy intelligent agent teams. Self-hosted. Git-backed. Production-ready.**

[Quick Start](#quick-start) | [Real Example](#real-example-sre-incident-response-team) | [Deploy](#deploy-to-production) | [Documentation](./docs/station/)

---

## Why Station?

Build multi-agent systems that coordinate like real teams. Test with realistic scenarios. Deploy on your infrastructure.

**Station gives you:**
- ✅ **Multi-Agent Teams** - Coordinate specialist agents under orchestrators
- ✅ **Built-in Evaluation** - LLM-as-judge tests every agent automatically  
- ✅ **Git-Backed Workflow** - Version control agents like code
- ✅ **One-Command Deploy** - Push to production with `stn deploy`
- ✅ **Full Observability** - Jaeger traces for every execution
- ✅ **Self-Hosted** - Your data, your infrastructure, your control

---

## Quick Start (2 minutes)

### Prerequisites

- **Docker** - Required for Jaeger (traces and observability)
- **AI Provider** - Choose one:
  - Claude Max/Pro subscription (no API billing)
  - `OPENAI_API_KEY` - OpenAI (gpt-4o-mini, gpt-4o, etc.)
  - `GEMINI_API_KEY` - Google Gemini

### 1. Install Station

```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

### 2. Initialize Station

Choose your AI provider:

<details>
<summary><b>Claude Max/Pro Subscription (Recommended)</b></summary>

Use your existing Claude Max or Claude Pro subscription - no API billing required.

```bash
# Initialize with Anthropic
stn init --provider anthropic --ship

# Authenticate with your Claude subscription
stn auth anthropic login
```

This opens your browser to authorize Station. After authorizing, paste the code and select your model:

```
✅ Successfully authenticated with Anthropic!

   You're using your Claude Max/Pro subscription.
   Station will automatically refresh tokens as needed.

Select a model for your Claude Max/Pro subscription:

* [1] Claude Opus 4.5 - Most capable model
  [2] Claude Opus 4
  [3] Claude Sonnet 4.5 - Balanced performance
  [4] Claude Sonnet 4 - Fast and efficient
  [5] Claude Haiku 4.5 - Fastest model
```

</details>

<details>
<summary><b>OpenAI (API Key)</b></summary>

```bash
export OPENAI_API_KEY="sk-..."
stn init --provider openai --ship  # defaults to gpt-4o-mini
```

</details>

<details>
<summary><b>Google Gemini (API Key)</b></summary>

```bash
export GEMINI_API_KEY="..."
stn init --provider gemini --ship
```

</details>

This sets up:
- ✅ Your chosen AI provider
- ✅ [Ship CLI](https://github.com/cloudshipai/ship) for filesystem MCP tools
- ✅ Configuration at `~/.config/station/config.yaml`

### 3. Start Jaeger (Tracing)

Start the Jaeger tracing backend for observability:

```bash
stn jaeger up
```

This starts Jaeger UI at [http://localhost:16686](http://localhost:16686) for viewing agent execution traces.

### 4. Connect Your MCP Client

Choose your editor and add Station:

<details>
<summary><b>Claude Code CLI</b></summary>

```bash
claude mcp add station -e OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 --scope user -- stn stdio
```

Verify with `claude mcp list`.

</details>

<details>
<summary><b>OpenCode</b></summary>

Add to `opencode.jsonc`:
```jsonc
{
  "mcp": {
    "station": {
      "enabled": true,
      "type": "local",
      "command": ["stn", "stdio"],
      "environment": {
        "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
      }
    }
  }
}
```

</details>

<details>
<summary><b>Cursor</b></summary>

Add to `.cursor/mcp.json` in your project (or `~/.cursor/mcp.json` for global):
```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"],
      "env": {
        "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
      }
    }
  }
}
```

</details>

<details>
<summary><b>Claude Desktop</b></summary>

| OS | Config Path |
|-----|------|
| macOS | `~/Library/Application Support/Claude/claude_desktop_config.json` |
| Windows | `%APPDATA%\Claude\claude_desktop_config.json` |
| Linux | `~/.config/Claude/claude_desktop_config.json` |

```json
{
  "mcpServers": {
    "station": {
      "command": "stn",
      "args": ["stdio"],
      "env": {
        "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
      }
    }
  }
}
```

</details>

**Optional GitOps:** Point to a Git-backed workspace:
```json
"command": ["stn", "--config", "/path/to/my-agents/config.yaml", "stdio"]
```

### 5. Start Building

Restart your editor. Station provides:
- ✅ **Web UI** at `http://localhost:8585` for configuration
- ✅ **Jaeger UI** at `http://localhost:16686` for traces
- ✅ **41 MCP tools** available in your AI assistant

**Try your first command:**
```
"Show me all Station MCP tools available"
```

<details>
<summary><b>Interactive Onboarding Guide (3-5 min tutorial)</b></summary>

Copy this prompt into your AI assistant for a hands-on tour:

```
You are my Station onboarding guide. Walk me through an interactive hands-on tutorial.

RULES:
1. Create a todo list to track progress through each section
2. At each section, STOP and let me engage before continuing
3. Use Station MCP tools to demonstrate - don't just explain, DO IT
4. Keep it fun and celebrate wins!

THE JOURNEY:

## 1. Hello World Agent
- Create a "hello-world" agent that greets users and tells a joke
- Call the agent and show the result
[STOP for me to try it]

## 2. Faker Tools & MCP Templates
- Explain Faker tools (AI-generated mock data for safe development)
- Note: Real MCP tools are added via Station UI or template.json
- Explain MCP templates - they keep credentials safe when deploying
- Create a "prometheus-metrics" faker for realistic metrics
[STOP to see the faker]

## 3. DevOps Investigation Agent
- Create a "metrics-investigator" agent using our prometheus faker
- Call it: "Check for performance issues in the last hour"
[STOP to review the investigation]

## 4. Multi-Agent Hierarchy
- Create an "incident-coordinator" that delegates to:
  - metrics-investigator (existing)
  - logs-investigator (new - create a logs faker)
- Show hierarchy structure in the .prompt file
- Call coordinator: "Investigate why the API is slow"
[STOP to see delegation]

## 5. Inspecting Runs
- Use inspect_run to show detailed execution
- Explain: tool calls, delegations, timing
[STOP to explore]

## 6. Workflow with Human-in-the-Loop
- Create a workflow: investigate → switch on severity → human_approval if high → report
- Make it complex (switch/parallel), not sequential
- Start the workflow
[STOP for me to approve/reject]

## 7. Evaluation & Reporting
- Run evals with evaluate_benchmark
- Generate a performance report
[STOP to review]

## 8. Grand Finale
- Direct me to http://localhost:8585 (Station UI)
- Quick tour: Agents, MCP servers, Runs, Workflows
- Celebrate!

## 9. Want More? (Optional)
Briefly explain these advanced features (no demo needed):
- **Schedules**: Cron-based agent scheduling
- **Sandboxes**: Isolated code execution (Python/Node/Bash)
- **Notify Webhooks**: Send alerts to Slack, ntfy, Discord
- **Bundles**: Package and share agent teams
- **Deploy**: `stn deploy` to Fly.io, Docker, K8s
- **CloudShip**: Centralized management and team OAuth

Start now with Section 1!
```

</details>

---

## Running Station with `stn up`

The easiest way to run Station is with `stn up` - a single command that starts Station in a Docker container with everything configured.

### Primary Use Case: Running Bundles

`stn up` is designed to make it trivial to run agent bundles from your CloudShip account or the community:

```bash
# Run a bundle from CloudShip (by ID or name)
stn up --bundle finops-cost-analyzer

# Run a bundle from URL
stn up --bundle https://example.com/my-bundle.tar.gz

# Run a local bundle file
stn up --bundle ./my-custom-agents.tar.gz
```

This is the recommended way for most users to get started - just pick a bundle and go.

### Secondary Use Case: Testing Local Configurations

Developers can also use `stn up` to test their local agent configurations in an isolated container environment:

```bash
# Test your local workspace in a container
stn up --workspace ~/my-agents

# Test with a specific environment
stn up --environment production

# Test with development tools enabled
stn up --develop
```

This lets you validate that your agents work correctly in the same containerized environment they'll run in production.

### Quick Commands

```bash
# Start Station (interactive setup on first run)
stn up

# Start with specific AI provider
stn up --provider openai --model gpt-4o

# Check status
stn status

# View logs
stn logs -f

# Stop Station
stn down

# Stop and remove all data (fresh start)
stn down --remove-volume
```

### What `stn up` Provides

| Service | Port | Description |
|---------|------|-------------|
| Web UI | 8585 | Configuration interface |
| MCP Server | 8586 | Main MCP endpoint for tools |
| Agent MCP | 8587 | Dynamic agent execution |
| Jaeger UI | 16686 | Distributed tracing |

**[See Container Lifecycle](./docs/station/container-lifecycle.md) for detailed architecture.**

---

## AI Provider Authentication

Station supports multiple authentication methods for AI providers.

### API Keys (Default)

The simplest way to authenticate - set your API key as an environment variable:

```bash
# OpenAI
export OPENAI_API_KEY="sk-..."

# Google Gemini
export GEMINI_API_KEY="..."

# Anthropic (API billing)
export ANTHROPIC_API_KEY="sk-ant-api03-..."
```

### Anthropic OAuth (Claude Max/Pro Subscription)

Use your Claude Max or Claude Pro subscription instead of pay-per-token API billing. This is the same authentication method used by Claude Code CLI.

**Setup:**
```bash
# Authenticate with your Claude subscription
stn auth anthropic login

# This will:
# 1. Open your browser to claude.ai
# 2. You authorize Station
# 3. Paste the code back to the terminal
# 4. Tokens are stored in your config.yaml
```

**Configure Station to use Anthropic:**
```yaml
# config.yaml
ai_provider: anthropic
ai_model: claude-sonnet-4-20250514
```

**Check status:**
```bash
stn auth anthropic status
# ✅ Authenticated with Anthropic
#    Token expires: 2025-01-15T10:30:00Z (23h remaining)
```

**Logout:**
```bash
stn auth anthropic logout
```

**Authentication Priority:**
| Priority | Method | Description |
|----------|--------|-------------|
| 1 | `STN_AI_AUTH_TYPE=api_key` | Force API key mode (override) |
| 2 | Station OAuth tokens | From `stn auth anthropic login` |
| 3 | Claude Code credentials | From `~/.claude/.credentials.json` |
| 4 | `ANTHROPIC_API_KEY` env var | Standard API key |

**Remote Deployment:**

For deployed Station instances (Docker, K8s, Fly.io), use API keys:

```bash
# In production, use API key authentication
docker run \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  -e STN_AI_PROVIDER=anthropic \
  station:latest
```

OAuth tokens are designed for local/interactive use. For production deployments, API keys provide simpler credential management.

---

## How You Interface: MCP-Driven Platform

**Station is driven entirely through MCP tools in your AI assistant.** No complex CLI commands or web forms - just natural language requests that use the 41 available MCP tools.

### Available MCP Tools

**Agent Management (11 tools):**
- `create_agent` - Create new agents with prompts and tools
- `update_agent` - Modify agent configuration
- `update_agent_prompt` - Update agent system prompt
- `delete_agent` - Remove an agent
- `list_agents` - List all agents (with filters)
- `get_agent_details` - Get full agent configuration
- `get_agent_schema` - Get agent's input schema
- `add_tool` - Add MCP tool to agent
- `remove_tool` - Remove tool from agent
- `add_agent_as_tool` - Create multi-agent hierarchies
- `remove_agent_as_tool` - Break agent hierarchy links

**Agent Execution (4 tools):**
- `call_agent` - Execute an agent with task
- `list_runs` - List agent execution history
- `inspect_run` - Get detailed run information
- `list_runs_by_model` - Filter runs by AI model

**Evaluation & Testing (6 tools):**
- `generate_and_test_agent` - Generate test scenarios and run agent
- `batch_execute_agents` - Run multiple agents in parallel
- `evaluate_benchmark` - Run LLM-as-judge evaluation
- `evaluate_dataset` - Evaluate entire dataset
- `export_dataset` - Export runs for analysis
- `list_benchmark_results` - List evaluation results
- `get_benchmark_status` - Check evaluation status

**Reports & Analytics (3 tools):**
- `create_report` - Create team performance report
- `generate_report` - Run benchmarks and generate report
- `list_reports` - List all reports
- `get_report` - Get report details

**Environment Management (2 tools):**
- `create_environment` - Create new environment
- `delete_environment` - Delete environment
- `list_environments` - List all environments

**MCP Server Management (5 tools):**
- `add_mcp_server_to_environment` - Add MCP server config
- `update_mcp_server_in_environment` - Update MCP server
- `delete_mcp_server_from_environment` - Remove MCP server
- `list_mcp_servers_for_environment` - List configured servers
- `list_mcp_configs` - List all MCP configurations

**Tool Discovery (2 tools):**
- `discover_tools` - Discover tools from MCP servers
- `list_tools` - List available tools

**Scheduling (3 tools):**
- `set_schedule` - Schedule agent with cron expression
- `remove_schedule` - Remove agent schedule
- `get_schedule` - Get agent schedule details

**Bundles (1 tool):**
- `create_bundle_from_environment` - Package environment as bundle

**Faker System (1 tool):**
- `faker_create_standalone` - Create AI-powered mock data server

**Examples of MCP-driven interaction:**
```
You: "Create a logs analysis agent that uses Datadog and Elasticsearch"

Claude: [Using create_agent tool...]
✅ Created logs_investigator agent with tools: __logs_query, __search_query
```

```
You: "Run the incident coordinator on the API timeout issue"

Claude: [Using call_agent tool with agent_id=21...]
[Shows full incident investigation with multi-agent delegation]
```

```
You: "Generate a performance report for my SRE team"

Claude: [Using generate_report tool...]
✅ Report completed: Team Score 7.5/10
```

**View all tools in your AI assistant:**
- Type "Show me all Station MCP tools" to see complete list with descriptions
- Tools appear with prefix `opencode-station_` in OpenCode
- Full documentation available via `get_agent_schema` and tool descriptions

---

## Real Example: SRE Incident Response Team

We built a complete SRE incident response team with 9 specialized agents that achieved a **7.5/10 performance score** in automated testing.

### The Team Structure

```
incident_coordinator (Orchestrator)
    ├── logs_investigator      - Analyzes error patterns in application logs
    ├── metrics_investigator   - Identifies performance spikes and anomalies
    ├── traces_investigator    - Examines distributed trace slowdowns
    ├── change_detective       - Correlates with recent deployments
    ├── infra_sre             - Checks K8s, AWS, and infrastructure
    ├── saas_dependency_analyst - Monitors external service outages
    ├── runbook_recommender   - Finds relevant documentation
    └── scribe                - Generates incident reports
```

### In Action: Production Incident Response

**You say:** "The checkout service is returning 503 errors and customers can't complete purchases"

**Incident Coordinator responds:**
```
I'll coordinate our SRE team to investigate this critical checkout service issue.

INITIAL ASSESSMENT:
- Service: checkout-service  
- Error: HTTP 503 (Service Unavailable)
- Impact: Customers unable to complete purchases
- Severity: SEV1 - Revenue impacting

Delegating to specialist agents...

[LOGS INVESTIGATION - @logs_investigator]
✅ Found error spike starting at 14:32 UTC
- Pattern: "connection pool exhausted" 
- Frequency: 847 errors/minute
- Stack trace: database connection timeout

[METRICS ANALYSIS - @metrics_investigator]
✅ Database connection pool at 100% utilization
- Normal: 40-60 connections
- Current: 200 connections (maxed out)
- Response time: 5000ms (normal: 200ms)

[DEPLOYMENT CHECK - @change_detective]
✅ Recent deployment detected:
- Service: checkout-service v2.3.0
- Deployed: 14:15 UTC (17 minutes before incident)
- Change: Added batch order processing

ROOT CAUSE IDENTIFIED:
New batch processing feature holding database connections 
longer than expected, exhausting the connection pool.

RECOMMENDED ACTIONS:
1. IMMEDIATE: Increase connection pool to 400 (kubectl patch)
2. ROLLBACK: Revert to v2.2.0 if pool increase insufficient
3. LONG-TERM: Implement connection pooling in batch processor
```

**[Screenshot needed: Web UI showing the incident coordinator agent run with delegations to specialist agents]**

### How Agents Are Built

Agents are simple `.prompt` files using GenKit's dotprompt format:

**Coordinator Agent** (delegates to other agents):
```yaml
---
metadata:
  name: "incident_coordinator"
  description: "Orchestrates specialist agents to investigate production incidents"
model: gpt-4o-mini
max_steps: 20
agents:
  - "logs_investigator"         # Delegate to logs specialist
  - "metrics_investigator"      # Delegate to metrics specialist
  - "traces_investigator"       # Delegate to traces specialist
  - "change_detective"          # Check recent changes
  - "infra_sre"                # Infrastructure analysis
  - "saas_dependency_analyst"  # External dependencies
  - "runbook_recommender"      # Find solutions
  - "scribe"                   # Generate reports
---

{{role "system"}}
You are an expert SRE incident coordinator. When presented with a production issue:

1. Assess severity and business impact
2. Delegate to relevant specialist agents using __agent_<name> tools
3. Synthesize findings into root cause
4. Provide clear remediation steps

{{role "user"}}
{{userInput}}
```

**Specialist Agent** (uses MCP tools):
```yaml
---
metadata:
  name: "logs_investigator"
  description: "Deep dive into logs to identify error patterns"
model: gpt-4o-mini
max_steps: 8
tools:
  - "__logs_query"      # Query Datadog/Elasticsearch logs
  - "__search_query"    # Search for patterns
---

{{role "system"}}
You analyze application logs to find root causes of incidents.
Focus on error patterns, stack traces, and anomalies.

{{role "user"}}
{{userInput}}
```

### How You Interface: MCP Tools in Action

Station provides 41 MCP tools directly in your AI assistant. Here's how you use them:

**Creating the team:**
```
You: Create an incident response team with coordinator and specialist agents

Claude: I'll create the SRE incident response team for you.

[Using tool: create_agent with name="incident_coordinator"...]
✅ Created incident_coordinator agent

[Using tool: create_agent with name="logs_investigator"...]
✅ Created logs_investigator agent

[Using tool: add_agent_as_tool with parent="incident_coordinator", child="logs_investigator"...]
✅ Added logs_investigator as tool for incident_coordinator

[Continues for all 8 specialists...]

Your SRE incident response team is ready! The coordinator can now delegate to all specialists.
```

**Running investigations:**
```
You: There's high memory usage on the payment service

Claude: I'll have the incident coordinator investigate this.

[Using tool: call_agent with agent_id="21", task="Investigate high memory usage on payment service"...]

[Response shows full investigation with delegations to specialists]
```

**[Screenshot needed: Claude/Cursor interface showing MCP tools being used]**

### The Stack: Real Infrastructure Tools + Simulated Data

Each specialist has access to production-grade tool integrations:
- **Datadog** - Metrics, APM, logs (via faker)
- **AWS CloudWatch** - Infrastructure monitoring (via faker)
- **Kubernetes** - Cluster diagnostics (via faker)
- **GitHub** - Deployment history (via faker)
- **Elasticsearch** - Log aggregation (via faker)
- **Grafana** - Metric dashboards (via faker)
- **StatusPage** - Service status (via faker)

The Faker system generates realistic mock data during development:

```yaml
datadog:
  command: stn
  args: ["faker", "--ai-instruction", "Generate production incident data: high CPU, memory leaks, error spikes"]
```

This lets you build and test without production credentials.

**[Screenshot needed: Faker generating realistic Datadog metrics]**

### Performance: LLM-as-Judge Evaluation

Station automatically tested this team against 100+ production scenarios:

**Team Performance: 7.5/10**
- ✅ **Multi-agent coordination**: 8.5/10 - Excellent delegation
- ✅ **Tool utilization**: 8.0/10 - Effective use of all tools
- ✅ **Root cause analysis**: 7.5/10 - Identifies issues accurately
- ⚠️ **Resolution speed**: 7.0/10 - Room for improvement
- ⚠️ **Communication clarity**: 6.5/10 - Could be more concise

**[Screenshot needed: Web UI showing team performance report with 7.5/10 score]**

---

## Deploy to Production

### One-Command Cloud Deploy

Deploy your agent team to Fly.io and expose agents as consumable MCP tools:

```bash
# Deploy the SRE team
stn deploy station-sre --target fly

✅ Building Docker image with agents
✅ Deploying to Fly.io (ord region)
✅ Configuring secrets from variables.yml
✅ Starting MCP server on port 3030

Your agents are live at:
https://station-sre.fly.dev:3030
```

**What you get:**
- **MCP Endpoint**: All 9 SRE agents exposed as MCP tools
- **Agent Tools**: Each agent becomes `__agent_<name>` tool
- **Secure Access**: Authentication via deploy token
- **Auto-Scaling**: Fly.io scales based on demand
- **Global CDN**: Deploy to regions worldwide

### Connect Deployed Agents to Your AI Assistant

Your deployed agents are now accessible as MCP tools from Claude, Cursor, or OpenCode:

**Claude Desktop / Cursor configuration:**
```json
{
  "mcpServers": {
    "station-sre-production": {
      "url": "https://station-sre.fly.dev:3030/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_DEPLOY_TOKEN"
      }
    }
  }
}
```

**Available tools after connection:**
```
__agent_incident_coordinator      - Orchestrates incident response
__agent_logs_investigator         - Analyzes error patterns
__agent_metrics_investigator      - Identifies performance spikes
__agent_traces_investigator       - Examines distributed traces
__agent_change_detective          - Correlates with deployments
__agent_infra_sre                 - Checks K8s/AWS infrastructure
__agent_saas_dependency_analyst   - Monitors external services
__agent_runbook_recommender       - Finds relevant docs
__agent_scribe                    - Generates incident reports
```

**Now you can call your agents from anywhere:**
```
You: "Investigate the API timeout issue using my SRE team"

Claude: [Calling __agent_incident_coordinator...]
[Full incident investigation with multi-agent delegation]
```

---

### Build for Self-Hosted Infrastructure

Create Docker images to run on your own infrastructure:

**Step 1: Build the image**
```bash
# Build with your environment embedded
stn build env station-sre --skip-sync

# Output: station-sre:latest Docker image
```

**Step 2: Run with your environment variables**
```bash
docker run -d \
  -p 3030:3030 \
  -e OPENAI_API_KEY=$OPENAI_API_KEY \
  -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY \
  -e PROJECT_ROOT=/workspace \
  -e AWS_REGION=us-east-1 \
  station-sre:latest
```

**Environment Variables at Runtime:**
- **AI Provider Keys**: `OPENAI_API_KEY`, `GEMINI_API_KEY`, etc.
- **Cloud Credentials**: `AWS_*`, `GCP_*`, `AZURE_*` credentials
- **Template Variables**: Any `{{ .VARIABLE }}` from your configs
- **MCP Server Config**: Database URLs, API endpoints, etc.

**Deploy anywhere:**
- **Kubernetes** - Standard deployment with ConfigMaps/Secrets
- **AWS ECS/Fargate** - Task definition with environment variables
- **Google Cloud Run** - One-click deploy with secrets
- **Azure Container Instances** - ARM templates
- **Docker Compose** - Multi-container orchestration
- **Your own servers** - Any Docker-capable host

**Example: Kubernetes Deployment**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station-sre
spec:
  replicas: 2
  template:
    spec:
      containers:
      - name: station
        image: your-registry/station-sre:latest
        ports:
        - containerPort: 3030
        env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: station-secrets
              key: openai-api-key
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: secret-access-key
        - name: PROJECT_ROOT
          value: "/workspace"
        - name: AWS_REGION
          value: "us-east-1"
---
apiVersion: v1
kind: Service
metadata:
  name: station-sre
spec:
  type: LoadBalancer
  ports:
  - port: 3030
    targetPort: 3030
  selector:
    app: station-sre
```

**Connect to your self-hosted MCP endpoint:**
```json
{
  "mcpServers": {
    "station-sre-production": {
      "url": "https://your-domain.com:3030/mcp",
      "headers": {
        "Authorization": "Bearer YOUR_TOKEN"
      }
    }
  }
}
```

---

### Advanced Deployment Options

**Custom AI Provider Configuration:**
```bash
# Build with specific model configuration
stn build env station-sre \
  --provider openai \
  --model gpt-4o-mini

# Or use environment variables at runtime
docker run -e STN_AI_PROVIDER=gemini \
           -e GEMINI_API_KEY=$GEMINI_API_KEY \
           station-sre:latest
```

**Multiple Regions:**
```bash
# Deploy to multiple Fly.io regions
stn deploy station-sre --target fly --region ord  # Chicago
stn deploy station-sre --target fly --region syd  # Sydney
stn deploy station-sre --target fly --region fra  # Frankfurt
```

**Health Checks:**
```bash
# Check MCP endpoint health
curl https://station-sre.fly.dev:3030/health

# Response
{
  "status": "healthy",
  "agents": 9,
  "mcp_servers": 3,
  "uptime": "2h 15m 30s"
}
```

### Bundle and Share

Package your agent team for distribution:

```bash
# Create a bundle from environment
stn bundle create station-sre

# Creates station-sre.tar.gz

# Share with your team or install elsewhere
stn bundle install station-sre.tar.gz
```

**[Screenshot needed: Web UI showing bundle in registry]**

### Schedule Agents for Automation

Run agents on a schedule for continuous monitoring:

```yaml
# Set up daily cost analysis
"Set a daily schedule for the cost analyzer agent to run at 9 AM"

# Schedule incident checks every 5 minutes
"Schedule the incident coordinator to check system health every 5 minutes"

# Weekly compliance audit
"Set up weekly compliance checks on Mondays at midnight"
```

Station uses cron expressions with second precision:
- `0 */5 * * * *` - Every 5 minutes
- `0 0 9 * * *` - Daily at 9 AM
- `0 0 0 * * 1` - Weekly on Monday midnight

**View scheduled agents in Web UI:**

**[Screenshot needed: Web UI showing scheduled agents with cron expressions]**

Scheduled agents run automatically and store results in the runs history.

### Event-Triggered Execution (Webhooks)

Trigger agent execution from external systems via HTTP webhook. Perfect for integrating with CI/CD pipelines, alerting systems, or any automation that can make HTTP requests.

**Endpoint:** `POST http://localhost:8587/execute`

```bash
# Trigger by agent name
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_name": "incident_coordinator", "task": "Investigate the API timeout alert"}'

# Trigger by agent ID
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{"agent_id": 21, "task": "Check system health"}'

# With variables for template rendering
curl -X POST http://localhost:8587/execute \
  -H "Content-Type: application/json" \
  -d '{
    "agent_name": "cost_analyzer",
    "task": "Analyze costs for project",
    "variables": {"project_id": "prod-123", "region": "us-east-1"}
  }'
```

**Response (202 Accepted):**
```json
{
  "run_id": 120,
  "agent_id": 21,
  "agent_name": "incident_coordinator",
  "status": "running",
  "message": "Agent execution started"
}
```

**Integration Examples:**

*PagerDuty Webhook:*
```bash
# Auto-investigate when PagerDuty alert fires
curl -X POST https://your-station:8587/execute \
  -H "Authorization: Bearer $STN_WEBHOOK_API_KEY" \
  -d '{"agent_name": "incident_coordinator", "task": "PagerDuty alert: {{alert.title}}"}'
```

*GitHub Actions:*
```yaml
- name: Run deployment analyzer
  run: |
    curl -X POST ${{ secrets.STATION_URL }}/execute \
      -H "Authorization: Bearer ${{ secrets.STATION_API_KEY }}" \
      -d '{"agent_name": "deployment_analyzer", "task": "Analyze deployment ${{ github.sha }}"}'
```

**Authentication:**
- **Local mode:** No authentication required
- **Production:** Set `STN_WEBHOOK_API_KEY` environment variable for static API key auth
- **OAuth:** Uses CloudShip OAuth when enabled

**Configuration:**
```bash
# Enable/disable webhook (default: enabled)
export STN_WEBHOOK_ENABLED=true

# Set static API key for authentication
export STN_WEBHOOK_API_KEY="your-secret-key"
```

[Webhook API Reference →](./docs/station/webhook-execute.md)

---

## What Makes Station Special

### Declarative Agent Definition
Simple `.prompt` files define intelligent behavior:

```yaml
---
metadata:
  name: "metrics_investigator"
  description: "Analyze performance metrics and identify anomalies"
model: gpt-4o-mini
max_steps: 8
tools:
  - "__get_metrics"           # Datadog metrics API
  - "__query_time_series"     # Grafana queries
  - "__get_dashboards"        # Dashboard snapshots
  - "__list_alerts"           # Active alerts
---

{{role "system"}}
You investigate performance issues by analyzing metrics and time series data.
Focus on: CPU, memory, latency, error rates, and throughput.

{{role "user"}}
{{userInput}}
```

### GitOps Workflow
Version control your entire agent infrastructure:

```bash
my-agents/
├── config.yaml              # Station configuration
├── environments/
│   ├── production/
│   │   ├── agents/         # Production agents
│   │   ├── template.json   # MCP server configs
│   │   └── variables.yml   # Secrets and config
│   └── development/
│       ├── agents/         # Dev agents
│       ├── template.json
│       └── variables.yml
└── reports/                # Performance evaluations
```

### Built-in Observability
Every execution automatically traced:

**[Screenshot needed: Jaeger showing multi-agent trace]**

```
incident_coordinator (18.2s)
├─ assess_severity (0.5s)
├─ delegate_logs_investigator (4.1s)
│  └─ __get_logs (3.2s)
├─ delegate_metrics_investigator (3.8s)
│  └─ __query_time_series (2.9s)
├─ delegate_change_detective (2.4s)
│  └─ __get_recent_deployments (1.8s)
└─ synthesize_findings (1.2s)
```

### Template Variables for Security
Never hardcode credentials:

```json
{
  "mcpServers": {
    "aws": {
      "command": "aws-mcp",
      "env": {
        "AWS_REGION": "{{ .AWS_REGION }}",
        "AWS_PROFILE": "{{ .AWS_PROFILE }}"
      }
    }
  }
}
```

Variables resolved from `variables.yml` or environment.

### Production-Grade Integrations
Connect to your actual infrastructure tools:
- **Cloud**: AWS, GCP, Azure via official SDKs
- **Monitoring**: Datadog, New Relic, Grafana
- **Incidents**: PagerDuty, Opsgenie, VictorOps
- **Kubernetes**: Direct cluster access
- **Databases**: PostgreSQL, MySQL, MongoDB
- **CI/CD**: Jenkins, GitHub Actions, GitLab

### Sandbox: Isolated Code Execution

Agents can execute Python, Node.js, or Bash code in isolated Docker containers:

**Compute Mode** - Ephemeral per-call (default):
```yaml
---
metadata:
  name: "data-processor"
sandbox: python    # or: node, bash
---
Use the sandbox_run tool to process data with Python.
```

**Code Mode** - Persistent session across workflow steps:
```yaml
---
metadata:
  name: "code-developer"
sandbox:
  mode: code
  session: workflow  # Share container across agents in workflow
---
Use sandbox_open, sandbox_exec, sandbox_fs_write to develop iteratively.
```

**Why Sandbox?**
| Without Sandbox | With Sandbox |
|-----------------|--------------|
| LLM calculates (often wrong) | Python computes correctly |
| Large JSON in context (slow) | Python parses efficiently |
| Host execution (security risk) | Isolated container (safe) |

**Enabling Sandbox:**
```bash
# Compute mode (ephemeral per-call)
export STATION_SANDBOX_ENABLED=true

# Code mode (persistent sessions - requires Docker)
export STATION_SANDBOX_ENABLED=true
export STATION_SANDBOX_CODE_MODE_ENABLED=true
```

[Sandbox Documentation →](./docs/station/sandbox.md)

---

## Try It Yourself

Ready to build your own agent team? Here's how:

### 1. Create Your Team

Ask your AI assistant:
```
"Create an incident response team like the SRE example with coordinator and specialist agents"
```

Station will:
- Create the multi-agent hierarchy
- Assign appropriate tools to each specialist
- Set up the coordinator to delegate tasks
- Configure realistic mock data for testing

### 2. Test with Real Scenarios

```
"The API gateway is timing out and affecting all services"
```

Watch as your coordinator:
- Assesses the situation
- Delegates to relevant specialists
- Gathers data from multiple sources
- Provides root cause analysis
- Recommends specific fixes

### 3. Evaluate Performance

```
"Generate a benchmark report for my incident response team"
```

Get detailed metrics on:
- Multi-agent coordination effectiveness
- Tool utilization patterns
- Response accuracy
- Communication clarity
- Areas for improvement

### 4. Deploy When Ready

```bash
stn deploy my-team --target fly
```

Your agents are now available as a production MCP endpoint.

---

## OpenAPI MCP Servers (Experimental)

Station can automatically convert OpenAPI/Swagger specifications into MCP servers, making any REST API instantly available as agent tools.

> ⚠️ **Experimental Feature** - OpenAPI to MCP conversion is currently in beta.

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

## Observability & Distributed Tracing

Station includes built-in OpenTelemetry (OTEL) support for complete execution observability:

**What Gets Traced:**
- **Agent Executions**: Complete timeline from start to finish
- **LLM Calls**: Every OpenAI/Anthropic/Gemini API call with latency
- **MCP Tool Usage**: Individual tool calls to AWS, Stripe, GitHub, etc.
- **Database Operations**: Query performance and data access patterns
- **GenKit Native Spans**: Dotprompt execution, generation flow, model interactions

**Quick Start with Jaeger:**
```bash
# Start Jaeger locally
make jaeger

# Configure Station
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
stn serve

# Run agent and view traces
stn agent run my-agent "Analyze costs"
open http://localhost:16686
```

**Team Integration Examples:**
- **Jaeger** - Open source tracing (local development)
- **Grafana Tempo** - Scalable distributed tracing
- **Datadog APM** - Full-stack observability platform
- **Honeycomb** - Advanced trace analysis with BubbleUp
- **New Relic** - Application performance monitoring
- **AWS X-Ray** - AWS-native distributed tracing

**Span Details Captured:**
```
aws-cost-spike-analyzer (18.2s)
├─ generate (17ms)
│  ├─ openai/gpt-4o-mini (11ms) - "Analyze cost data"
│  └─ __get_cost_anomalies (0ms) - AWS Cost Explorer
├─ generate (11ms)
│  └─ __get_cost_and_usage_comparisons (0ms)
└─ db.agent_runs.create (0.1ms)
```

**Configuration:**
```bash
# Environment variable (recommended)
export OTEL_EXPORTER_OTLP_ENDPOINT=http://your-collector:4318

# Or config file
otel_endpoint: "http://your-collector:4318"
```

[Complete OTEL Setup Guide →](./docs/OTEL_SETUP.md) - Includes Jaeger, Tempo, Datadog, Honeycomb, AWS X-Ray, New Relic, Azure Monitor examples

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

## CloudShip Integration

Connect your Station to [CloudShip](https://cloudshipai.com) for centralized management, OAuth authentication, and team collaboration.

### Why CloudShip?

- **Centralized Management** - Manage multiple Stations from a single dashboard
- **OAuth Authentication** - Secure MCP access with CloudShip user accounts
- **Team Collaboration** - Share agents with your organization members
- **Audit Trail** - Track all Station connections and executions

### Who Can Access Your Station?

With CloudShip OAuth enabled, only users who:
1. Have a **CloudShip account**
2. Are **members of your organization**
3. Successfully **authenticate via OAuth**

...can access your Station's agents through MCP. This lets you share powerful agents with your team while keeping them secure.

### Quick Setup

1. **Get a Registration Key** from your CloudShip dashboard at `Settings > Stations`

2. **Configure your Station** (`config.yaml`):
   ```yaml
   cloudship:
     enabled: true
     registration_key: "your-registration-key"
     name: "my-station"           # Unique name for this station
     tags: ["production", "us-east-1"]
   ```

3. **Start Station** - It will automatically connect to CloudShip:
   ```bash
   stn serve
   # Output: Successfully registered with CloudShip management channel
   ```

### OAuth Authentication for MCP

When CloudShip OAuth is enabled, MCP clients (Claude Desktop, Cursor, etc.) authenticate through CloudShip before accessing your Station's agents.

**Setup (Station Admin):**
1. Create an OAuth App in CloudShip (Settings > OAuth Apps)
2. Configure Station with `oauth.enabled: true` and `oauth.client_id`
3. Invite team members to your CloudShip organization

**Usage (Team Members):**
1. Point MCP client to your Station's Dynamic Agent MCP URL (port 8587)
2. Browser opens for CloudShip login
3. Approve access → Done! Now you can use the agents.

**How it works:**

```
MCP Client                    Station                      CloudShip
    |                           |                             |
    |------ POST /mcp --------->|                             |
    |<----- 401 Unauthorized ---|                             |
    |       WWW-Authenticate:   |                             |
    |       Bearer resource_metadata="..."                    |
    |                           |                             |
    |------- [OAuth Discovery] ------------------------------>|
    |<------ [Authorization Server Metadata] -----------------|
    |                           |                             |
    |------- [Browser Login] -------------------------------->|
    |<------ [Authorization Code] ----------------------------|
    |                           |                             |
    |------- [Token Exchange] ------------------------------->|
    |<------ [Access Token] ----------------------------------|
    |                           |                             |
    |------ POST /mcp --------->|                             |
    |  Authorization: Bearer    |------ Validate Token ------>|
    |                           |<------ {active: true} ------|
    |<----- MCP Response -------|                             |
```

**Enable OAuth** (`config.yaml`):
```yaml
cloudship:
  enabled: true
  registration_key: "your-key"
  name: "my-station"
  oauth:
    enabled: true
    client_id: "your-oauth-client-id"  # From CloudShip OAuth Apps
```

**MCP Client Configuration** (Claude Desktop / Cursor):
```json
{
  "mcpServers": {
    "my-station": {
      "url": "https://my-station.example.com:8587/mcp"
    }
  }
}
```

> **Note:** Port 8587 is the Dynamic Agent MCP server. Port 8586 is the standard MCP server.

When the MCP client connects, it will:
1. Receive a 401 with OAuth discovery URL
2. Open CloudShip login in your browser
3. After authentication, automatically retry with the access token

### Configuration Reference

```yaml
cloudship:
  # Enable CloudShip integration
  enabled: true
  
  # Registration key from CloudShip dashboard
  registration_key: "sk-..."
  
  # Unique station name (required for multi-station support)
  name: "production-us-east"
  
  # Tags for filtering and organization
  tags: ["production", "us-east-1", "sre-team"]
  
  # CloudShip endpoints (defaults shown - usually no need to change)
  endpoint: "lighthouse.cloudshipai.com:443"  # TLS-secured gRPC endpoint
  use_tls: true                               # TLS enabled by default
  base_url: "https://app.cloudshipai.com"
  
  # OAuth settings for MCP authentication
  oauth:
    enabled: false                    # Enable OAuth for MCP
    client_id: ""                     # OAuth client ID from CloudShip
    # These are auto-configured from base_url:
    # auth_url: "https://app.cloudshipai.com/oauth/authorize/"
    # token_url: "https://app.cloudshipai.com/oauth/token/"
    # introspect_url: "https://app.cloudshipai.com/oauth/introspect/"
```

### Development Setup

For local development with a local Lighthouse instance:

```yaml
cloudship:
  enabled: true
  registration_key: "your-dev-key"
  name: "dev-station"
  endpoint: "localhost:50051"           # Local Lighthouse (no TLS)
  use_tls: false                        # Disable TLS for local development
  base_url: "http://localhost:8000"     # Local Django
  oauth:
    enabled: true
    client_id: "your-dev-client-id"
    introspect_url: "http://localhost:8000/oauth/introspect/"
```

For connecting to **production CloudShip** during development (recommended):

```yaml
cloudship:
  enabled: true
  registration_key: "your-registration-key"
  name: "dev-station"
  # Uses defaults: endpoint=lighthouse.cloudshipai.com:443, use_tls=true
```

### Security Notes

- **Registration keys** should be kept secret - they authorize Station connections
- **OAuth tokens** are validated on every MCP request via CloudShip introspection
- **PKCE** is required for all OAuth flows (S256 code challenge)
- Station caches validated tokens for 5 minutes to reduce introspection calls

---

## Database Persistence & Replication

Station uses SQLite by default, with support for cloud databases and continuous backup for production deployments.

### Local Development (Default)
```bash
# Station uses local SQLite file
stn stdio
```
Perfect for local development, zero configuration required.

### Cloud Database (libsql)
For multi-instance deployments or team collaboration, use a libsql-compatible cloud database:

```bash
# Connect to cloud database
export DATABASE_URL="libsql://your-db.example.com?authToken=your-token"
stn stdio
```

**Benefits:**
- State persists across multiple deployments
- Team collaboration with shared database
- Multi-region replication
- Automatic backups

### Continuous Backup (Litestream)
For single-instance production deployments with disaster recovery:

```bash
# Docker deployment with automatic S3 backup
docker run \
  -e LITESTREAM_S3_BUCKET=my-backups \
  -e LITESTREAM_S3_ACCESS_KEY_ID=xxx \
  -e LITESTREAM_S3_SECRET_ACCESS_KEY=yyy \
  ghcr.io/cloudshipai/station:production
```

**Benefits:**
- Continuous replication to S3/GCS/Azure
- Automatic restore on startup
- Point-in-time recovery
- Zero data loss on server failures

[Database Replication Guide →](./docs/station/DATABASE_REPLICATION.md)

---

## GitOps Workflow

Version control your agent configurations, MCP templates, and variables in Git:

```bash
# Create a Git repository for your Station config
mkdir my-station-config
cd my-station-config

# Initialize Station in this directory
export STATION_WORKSPACE=$(pwd)
stn init

# Your agents are now in ./environments/default/agents/
# Commit to Git and share with your team!
git init
git add .
git commit -m "Initial Station configuration"
```

**Team Workflow:**
```bash
# Clone team repository
git clone git@github.com:your-team/station-config.git
cd station-config

# Run Station with this workspace
export STATION_WORKSPACE=$(pwd)
stn stdio
```

All agent `.prompt` files, MCP `template.json` configs, and `variables.yml` are version-controlled and reviewable in Pull Requests.

[GitOps Workflow Guide →](./docs/station/GITOPS_WORKFLOW.md)

---

## System Requirements

- **OS:** Linux, macOS, Windows
- **Memory:** 512MB minimum, 1GB recommended
- **Storage:** 200MB for binary, 1GB+ for agent data
- **Network:** Outbound HTTPS for AI providers

---

## Mission

**Make it easy for engineering teams to build and deploy infrastructure agents on their own terms.**

Station puts you in control:
- **Self-hosted** - Your data stays on your infrastructure
- **Git-backed** - Version control everything like code
- **Production-ready** - Deploy confidently with built-in evaluation
- **Team-owned** - No vendor lock-in, no data sharing

We believe teams should own their agentic automation, from development to production.

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

**Station - AI Agent Orchestration Platform**

*Build, test, and deploy intelligent agent teams. Self-hosted. Git-backed. Production-ready.*
