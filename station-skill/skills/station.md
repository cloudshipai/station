# Station CLI

Station is a self-hosted AI agent orchestration platform. You interact with it via the `stn` CLI or MCP tools (41+ available via `stn stdio`).

## When to Use CLI vs MCP Tools

| Task | Use CLI | Use MCP Tool |
|------|---------|--------------|
| Create/edit agent files | `stn agent create`, edit `.prompt` files | - |
| Run an agent | `stn agent run <name> "<task>"` | `call_agent` |
| List agents/environments | `stn agent list`, `stn env list` | `list_agents`, `list_environments` |
| Add MCP servers | `stn mcp add <name>` | `add_mcp_server_to_environment` |
| Sync configurations | `stn sync <env>` | - |
| Install bundles | `stn bundle install <url>` | - |
| Inspect runs | `stn runs list` | `inspect_run`, `list_runs` |
| Deploy | `stn deploy <env>` | - |
| Start services | `stn serve`, `stn jaeger up` | - |

**Rule of thumb**: CLI for setup, file operations, deployment. MCP tools for programmatic execution and queries within conversations.

## Quick Reference

### Initialization

```bash
# Initialize Station with AI provider
stn init --provider openai --ship       # OpenAI with Ship filesystem tools
stn init --provider anthropic --ship    # Anthropic (requires OAuth: stn auth anthropic login)
stn init --provider gemini --ship       # Google Gemini

# Start Jaeger for observability
stn jaeger up                           # View traces at http://localhost:16686
```

### Agent Management

```bash
# CREATE agent via CLI (recommended)
stn agent create <name> --prompt "<prompt>" --description "<desc>" [options]

# Required flags:
#   --prompt, -p        System prompt (required)
#   --description, -d   Description (required)
# Optional flags:
#   --environment, -e   Environment (default: "default")
#   --max-steps         Max execution steps (default: 5)
#   --tools             Comma-separated tool names
#   --input-schema      JSON schema for input variables
#   --output-schema     JSON schema for structured output
#   --output-schema-preset  Predefined schema (e.g., 'finops')
# CloudShip integration:
#   --app               CloudShip app classification
#   --app-type          CloudShip app_type classification
#   --memory-topic      CloudShip memory topic key
#   --memory-max-tokens Max tokens for memory context
# Advanced:
#   --sandbox           Sandbox config JSON
#   --coding            Coding config JSON
#   --notify            Enable notifications

# Examples:
stn agent create my-agent \
  --prompt "You are a helpful assistant" \
  --description "General purpose assistant"

stn agent create sre-helper \
  --prompt "You are an SRE assistant" \
  --description "SRE support with memory" \
  --memory-topic "sre-incidents" \
  --memory-max-tokens 4000

# LIST agents
stn agent list                          # All agents in default environment
stn agent list --env production         # Agents in specific environment

# SHOW agent details
stn agent show <agent-name>
stn agent show my-agent --env production

# RUN an agent
stn agent run <name> "<task>"           # Execute with task
stn agent run my-agent "task" --tail    # Follow output in real-time
stn agent run my-agent "task" --env prod

# UPDATE agent via CLI (all flags optional)
stn agent update <name> [options]
# Uses same flags as create (all optional - only provided values updated)

# Examples:
stn agent update my-agent --prompt "New prompt" --max-steps 15
stn agent update my-agent --memory-topic "project-context"
stn agent update my-agent --notify

# Alternative: Edit .prompt file directly, then sync
nano ~/.config/station/environments/default/my-agent.prompt
stn sync default

# DELETE agent
stn agent delete <name>
stn agent delete my-agent --confirm     # Skip confirmation
```

### Environment Management

```bash
# Sync file configurations to database
stn sync default                        # Sync default environment
stn sync default --browser              # Secure input for secrets (recommended for AI)
stn sync default --dry-run              # Preview changes
```

### MCP Server Configuration

```bash
# Add MCP server
stn mcp add <name> --command <cmd> --args "<args>"

# Examples
stn mcp add filesystem --command npx --args "-y,@modelcontextprotocol/server-filesystem,/path"
stn mcp add github --command npx --args "-y,@modelcontextprotocol/server-github" --env "GITHUB_TOKEN={{.TOKEN}}"

# Add OpenAPI spec as MCP server
stn mcp add-openapi petstore --url https://petstore3.swagger.io/api/v3/openapi.json

# List and manage
stn mcp list                            # List configurations
stn mcp tools                           # List available tools
stn mcp status                          # Show sync status
```

### Bundle Management

```bash
# Install bundle from URL or CloudShip
stn bundle install <url-or-id> <environment>

# Create bundle from environment
stn bundle create <environment>

# Share bundle to CloudShip
stn bundle share <environment>
```

### Workflow Management

```bash
# CREATE: Write YAML file (no CLI create command)
mkdir -p ~/.config/station/environments/default/workflows
cat > ~/.config/station/environments/default/workflows/my-workflow.yaml << 'EOF'
name: my-workflow
description: Example workflow
initial_state: start
states:
  start:
    type: agent
    agent: my-agent
    transitions:
      - next_state: complete
  complete:
    type: terminal
EOF
stn sync default                        # Required after creating/editing

# LIST workflows
stn workflow list
stn workflow list --env production

# SHOW workflow details
stn workflow show <workflow-id>
stn workflow show <workflow-id> --verbose

# VALIDATE workflow
stn workflow validate my-workflow

# RUN workflow
stn workflow run <name>
stn workflow run <name> --input '{"key": "value"}'

# LIST workflow runs
stn workflow runs
stn workflow runs --status running

# INSPECT a run
stn workflow inspect <run-id>

# MANAGE approvals (human-in-the-loop)
stn workflow approvals list
stn workflow approvals approve <approval-id>
stn workflow approvals reject <approval-id> --reason "Not authorized"

# EXPORT workflow to file
stn workflow export <workflow-id> --output workflow.yaml

# UPDATE: Edit YAML file directly, then sync
nano ~/.config/station/environments/default/workflows/my-workflow.yaml
stn sync default

# DELETE workflow
stn workflow delete <workflow-id>
stn workflow delete --all --force       # Delete all workflows
```

### Server & Deployment

```bash
# Start Station server (web UI at :8585)
stn serve

# Docker container mode
stn up                                  # Interactive setup
stn up default --yes                    # Use defaults
stn status                              # Check container status
stn logs -f                             # Follow logs
stn down                                # Stop container

# Deploy to cloud
stn deploy <environment> --target fly   # Deploy to Fly.io
stn deploy production --target fly --region syd
```

### Runs & Inspection

```bash
# List recent runs
stn runs list
stn runs list --limit 20

# Inspect run details
stn runs inspect <run-id>
stn runs inspect <run-id> --verbose     # Full details with tool calls
```

### Benchmarking & Reports

```bash
# BENCHMARK: Evaluate agent runs with LLM-as-judge
stn benchmark evaluate <run-id>
stn benchmark evaluate <run-id> --verbose  # Detailed metrics
stn benchmark list                         # List all evaluations
stn benchmark list <run-id>                # Details for specific run
stn benchmark tasks                        # List available benchmark tasks

# REPORTS: Environment-wide evaluation
stn report create --name "review" --environment default
stn report create -n "audit" -e prod -d "Security audit"
stn report list
stn report list --environment production
stn report generate <report-id>
stn report show <report-id>
```

## File Structure

Station stores configurations at `~/.config/station/`:

```
~/.config/station/
├── config.yaml                 # Main configuration
├── station.db                  # SQLite database
└── environments/
    └── default/
        ├── *.prompt            # Agent definitions
        ├── *.json              # MCP server configurations
        └── variables.yml       # Template variable values
```

## Agent File Format (dotprompt)

Agents are `.prompt` files with YAML frontmatter:

```yaml
---
metadata:
  name: "my-agent"
  description: "What this agent does"
model: gpt-4o-mini
max_steps: 8
tools:
  - "__tool_name"              # MCP tools prefixed with __
agents:
  - "child-agent-name"         # Child agents (become __agent_* tools)
---
{{role "system"}}
You are a helpful agent that [purpose].

{{role "user"}}
{{userInput}}
```

## Common Workflows

### 1. Create New Agent

```bash
# Option 1: CLI (recommended)
stn agent create my-agent \
  --prompt "You are a helpful agent." \
  --description "Description here"

# Option 2: File + sync
cat > ~/.config/station/environments/default/my-agent.prompt << 'EOF'
---
metadata:
  name: "my-agent"
  description: "Description here"
model: gpt-4o-mini
max_steps: 5
tools: []
---
{{role "system"}}
You are a helpful agent.

{{role "user"}}
{{userInput}}
EOF

# Sync to database (only needed for Option 2)
stn sync default

# Run it
stn agent run my-agent "Hello, what can you do?"
```

### 2. Add External Tools

```bash
# Add GitHub MCP server with template variable
stn mcp add github \
  --command npx \
  --args "-y,@modelcontextprotocol/server-github" \
  --env "GITHUB_TOKEN={{.GITHUB_TOKEN}}"

# Sync (will prompt for GITHUB_TOKEN)
stn sync default --browser

# Now agents can use __github_* tools
```

### 3. Deploy to Production

```bash
# Prepare environment
stn sync production --browser

# Test locally
stn agent run my-agent "test task" --env production

# Deploy to Fly.io
stn deploy production --target fly --region ord
```

### 4. GitHub Actions CI/CD

Run agents in GitHub Actions workflows using [cloudshipai/station-action](https://github.com/cloudshipai/station-action):

```yaml
# .github/workflows/ai-review.yml
name: AI Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4

      - uses: cloudshipai/station-action@v1
        with:
          agent: 'Code Reviewer'
          task: |
            Review the changes in this PR. Focus on:
            - Security vulnerabilities
            - Performance issues
            - Code quality
          comment-pr: 'true'
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

**Action Inputs:**

| Input | Description | Default |
|-------|-------------|---------|
| `agent` | Agent name to run (required) | - |
| `task` | Task description (required) | - |
| `provider` | AI provider (openai, anthropic, gemini, ollama) | `openai` |
| `model` | Model override | Provider default |
| `environment` | Local environment name | `default` |
| `bundle-url` | Bundle URL to download | - |
| `timeout` | Execution timeout (seconds) | `300` |
| `max-steps` | Maximum agent steps | `50` |
| `comment-pr` | Post result as PR comment | `false` |

**Provider Examples:**

```yaml
# Anthropic
- uses: cloudshipai/station-action@v1
  with:
    agent: 'My Agent'
    task: 'Analyze the codebase'
    provider: 'anthropic'
    model: 'claude-3-5-sonnet-20241022'
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

# Google Gemini
- uses: cloudshipai/station-action@v1
  with:
    agent: 'My Agent'
    task: 'Analyze the codebase'
    provider: 'gemini'
    model: 'gemini-2.0-flash-exp'
  env:
    GOOGLE_API_KEY: ${{ secrets.GOOGLE_API_KEY }}
```

**Build & Release Actions:**

```yaml
# Build bundle from environment
- uses: cloudshipai/station/.github/actions/build-bundle@main
  with:
    environment: 'production'
    version: '1.0.0'

# Build and push Docker image
- uses: cloudshipai/station/.github/actions/build-image@main
  with:
    source-type: 'environment'
    environment: 'production'
    image-name: 'my-org/station-production'
    push: 'true'
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}

# Setup Station CLI only
- uses: cloudshipai/station/.github/actions/setup-station@main
  with:
    version: 'latest'
  env:
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

Full docs: https://docs.cloudshipai.com/station/github-actions

## Troubleshooting

### Agent not finding tools
```bash
stn sync <environment>          # Resync configurations
stn mcp tools                   # Verify tools are loaded
```

### View execution traces
```bash
stn jaeger up                   # Start Jaeger
# Open http://localhost:16686
# Search for service: station
```
