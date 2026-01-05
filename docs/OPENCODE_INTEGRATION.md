# Coding Backend Integration Guide

Station integrates with AI coding assistants to delegate complex coding tasks. When your Station agent needs to write code, fix bugs, or make changes to a repository, it can hand off the work to a coding backend.

## Supported Backends

| Backend | Command | Description |
|---------|---------|-------------|
| `opencode` | HTTP API | [OpenCode](https://opencode.ai) server (SST's AI coding assistant) |
| `opencode-cli` | CLI subprocess | OpenCode CLI binary |
| `opencode-nats` | NATS messaging | OpenCode via NATS for distributed deployments |
| `claudecode` | CLI subprocess | [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI (Anthropic's official coding agent) |

## Architecture Overview

```
Station Agent                    Coding Backend
┌─────────────┐                 ┌─────────────────┐
│ coding_open │────repo_url────▶│ git clone       │
│             │                 │ create workspace│
├─────────────┤                 ├─────────────────┤
│ code        │────task────────▶│ read/write/edit │
│             │                 │ run commands    │
├─────────────┤                 ├─────────────────┤
│ coding_     │────message─────▶│ git add/commit  │
│ commit      │                 │                 │
├─────────────┤                 ├─────────────────┤
│ coding_push │────────────────▶│ git push        │
├─────────────┤                 ├─────────────────┤
│ coding_close│────────────────▶│ cleanup         │
└─────────────┘                 └─────────────────┘
```

**Key Design**: The coding backend owns ALL workspace and git operations. Station just sends tasks and receives results.

---

## Claude Code Backend

Claude Code is Anthropic's official AI coding agent CLI. It uses your Claude Max/Pro subscription or API key.

### Prerequisites

1. Install Claude Code CLI: https://docs.anthropic.com/en/docs/claude-code
2. Authenticate: `claude login` (for Max/Pro) or set `ANTHROPIC_API_KEY`

### Configuration

```yaml
# ~/.config/station/config.yaml

coding:
  backend: claudecode
  claudecode:
    binary_path: claude          # Path to claude CLI (default: "claude")
    timeout_sec: 300             # Task timeout in seconds (default: 300)
    model: sonnet                # Model: sonnet, opus, haiku (optional)
    max_turns: 10                # Max agentic turns (default: 10)
    allowed_tools:               # Whitelist specific tools (optional)
      - Read
      - Write
      - Bash
      - Glob
      - Grep
    disallowed_tools: []         # Blacklist tools (optional)
  
  # Git credentials (for private repos)
  git:
    token_env: GITHUB_TOKEN
    user_name: "Station Bot"
    user_email: "bot@example.com"
```

### Authentication

Claude Code CLI manages its own authentication, separate from Station:

| Auth Method | Setup |
|-------------|-------|
| Claude Max/Pro | Run `claude login` in terminal |
| API Key | Set `ANTHROPIC_API_KEY` environment variable |

Station's OAuth tokens (from `stn auth anthropic login`) are used for Station's orchestration layer, NOT for Claude Code. This separation allows:
- Using different accounts for orchestration vs coding
- Claude Code inherits host system's claude authentication
- Simpler setup - just run `claude login` once

### Quick Start

```bash
# 1. Ensure claude CLI is authenticated
claude --version
claude login  # if needed

# 2. Configure Station
cat >> ~/.config/station/config.yaml << 'EOF'
coding:
  backend: claudecode
  claudecode:
    timeout_sec: 300
    max_turns: 10
EOF

# 3. Create a coding agent
stn agent create coder \
  --description "Coding assistant using Claude Code" \
  --prompt "You are a coding assistant. Use coding tools to accomplish tasks." \
  --coding '{"enabled":true}' \
  --max-steps 10

# 4. Run it
stn agent run coder "Create a hello.py that prints Hello World" --tail
```

---

## OpenCode Backend

OpenCode is SST's AI coding assistant. It can run as an HTTP server or CLI.

### HTTP Server Mode

```bash
# Start OpenCode server
opencode serve --port 4096

# Or via Docker
docker run -d \
  -p 4096:4096 \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  ghcr.io/sst/opencode:latest \
  opencode serve --hostname 0.0.0.0 --port 4096
```

```yaml
# ~/.config/station/config.yaml
coding:
  backend: opencode
  opencode:
    url: http://localhost:4096
    model: claude-sonnet-4  # optional
```

### CLI Mode

```yaml
# ~/.config/station/config.yaml
coding:
  backend: opencode-cli
  cli:
    binary_path: opencode      # Path to opencode binary
    timeout_sec: 300           # Task timeout
```

### NATS Mode (Distributed)

For multi-tenant or distributed deployments:

```yaml
# ~/.config/station/config.yaml
coding:
  backend: opencode-nats
  nats:
    url: nats://localhost:4222
    creds_file: /path/to/nats.creds  # optional
    subjects:
      task: station.coding.task
      stream: station.coding.stream
      result: station.coding.result
    kv:
      sessions: coding-sessions
      state: coding-state
    object_store: coding-artifacts
```

---

## Git Repository Integration

All coding backends support cloning repositories, creating branches, committing, and pushing changes.

### Setting Up Git Credentials

#### Option 1: Environment Variable (Recommended)

```bash
# Create a GitHub Personal Access Token with 'repo' scope
# https://github.com/settings/tokens

export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

```yaml
# ~/.config/station/config.yaml
coding:
  git:
    token_env: GITHUB_TOKEN      # Reads from this env var
    user_name: "Station Bot"     # For commit author
    user_email: "bot@example.com"
```

#### Option 2: Direct Token (Not Recommended)

```yaml
coding:
  git:
    token: ${GITHUB_TOKEN}       # Supports env var expansion
    # token: ghp_xxxxx           # Or hardcode (avoid in shared configs)
```

#### Option 3: GitHub App Installation Token

For GitHub Actions or automated systems:

```bash
# Generate installation token
export GITHUB_TOKEN=$(gh api \
  -X POST /app/installations/$INSTALLATION_ID/access_tokens \
  --jq '.token')
```

### How Token Injection Works

When cloning or pushing to private repos, Station automatically injects credentials:

```
Original:  https://github.com/org/private-repo.git
Injected:  https://x-access-token:TOKEN@github.com/org/private-repo.git
```

**Security Notes:**
- Tokens are NEVER logged - all output is redacted
- Tokens are passed via HTTPS URL injection (not command line args)
- Each session can have isolated credentials

### Working with Private Repositories

```bash
# Set credentials
export GITHUB_TOKEN=ghp_xxxxxxxxxxxx

# Clone and modify a private repo
stn agent run coder "Clone https://github.com/myorg/private-repo, add a README, commit and push" --tail
```

### Git Operations Reference

| Tool | Description | Example |
|------|-------------|---------|
| `coding_open` | Clone repo or open workspace | `repo_url: "https://github.com/org/repo"` |
| `coding_branch` | Create/switch branches | `branch: "feature/new", create: true` |
| `coding_commit` | Commit changes | `message: "Add feature", add_all: true` |
| `coding_push` | Push to remote | `remote: "origin", set_upstream: true` |

### Example: Full Git Workflow

```bash
stn agent run coder "
Clone https://github.com/myorg/myrepo,
create a branch called 'feature/add-tests',
add unit tests for the auth module,
commit with message 'Add auth unit tests',
and push the branch
" --tail
```

The agent will:
1. `coding_open` - Clone the repo
2. `coding_branch` - Create feature/add-tests
3. `code` - Write the tests
4. `coding_commit` - Commit changes
5. `coding_push` - Push to remote
6. `coding_close` - Cleanup

---

## Tools Reference

### coding_open

Opens a coding session. Optionally clones a git repository.

```json
{
  "repo_url": "https://github.com/org/repo.git",
  "branch": "main",
  "workspace_path": "/path/to/existing/repo",
  "title": "Fix auth bug",
  "scope": "agent",
  "scope_id": "workflow-123"
}
```

**Returns:**
```json
{
  "session_id": "coding_1234567890",
  "workspace_path": "/tmp/station-coding/ws_xxx",
  "repo_cloned": true,
  "managed": true
}
```

### code

Execute a coding task in the session.

```json
{
  "session_id": "coding_1234567890",
  "instruction": "Add a /health endpoint that returns {status: ok}",
  "context": "This is a Flask API",
  "files": ["src/api.py"]
}
```

**Returns:**
```json
{
  "success": true,
  "summary": "Added health endpoint at /health in src/api.py",
  "files_changed": [
    {"path": "src/api.py", "status": "modified"}
  ],
  "trace": {
    "tokens": {"input": 1500, "output": 200},
    "cost": 0.0045,
    "tool_calls": [{"tool": "Write", "output": "..."}]
  }
}
```

### coding_branch

Create or switch git branches.

```json
{
  "session_id": "coding_1234567890",
  "branch": "feature/new-feature",
  "create": true
}
```

### coding_commit

Commit changes in the workspace.

```json
{
  "session_id": "coding_1234567890",
  "message": "Add health check endpoint",
  "add_all": true
}
```

**Returns:**
```json
{
  "success": true,
  "commit_hash": "abc123def456...",
  "files_changed": 2,
  "insertions": 15,
  "deletions": 3
}
```

### coding_push

Push commits to remote.

```json
{
  "session_id": "coding_1234567890",
  "remote": "origin",
  "branch": "feature/health",
  "set_upstream": true
}
```

### coding_close

Close the session and clean up.

```json
{
  "session_id": "coding_1234567890",
  "success": true
}
```

---

## Configuration Reference

### Full Example

```yaml
# ~/.config/station/config.yaml

coding:
  # Backend selection: opencode, opencode-cli, opencode-nats, claudecode
  backend: claudecode
  
  # Claude Code settings
  claudecode:
    binary_path: claude
    timeout_sec: 300
    model: sonnet                # sonnet, opus, haiku
    max_turns: 10
    allowed_tools:
      - Read
      - Write
      - Edit
      - Bash
      - Glob
      - Grep
  
  # OpenCode HTTP settings (when backend: opencode)
  opencode:
    url: http://localhost:4096
    model: claude-sonnet-4
  
  # OpenCode CLI settings (when backend: opencode-cli)
  cli:
    binary_path: opencode
    timeout_sec: 300
  
  # Retry and timeout settings
  max_attempts: 3
  task_timeout_min: 10
  clone_timeout_sec: 300
  push_timeout_sec: 120
  
  # Workspace management
  workspace_base_path: /tmp/station-coding
  cleanup_policy: on_session_end  # on_session_end, on_success, manual
  
  # Git configuration
  git:
    token_env: GITHUB_TOKEN       # Read from env var (recommended)
    # token: ${GITHUB_TOKEN}      # Or direct with expansion
    user_name: "Station Bot"
    user_email: "bot@cloudship.ai"
```

### Agent-Level Config

Enable coding in agent dotprompt:

```yaml
---
model: claude-sonnet-4-20250514
coding:
  enabled: true
---
You are a coding assistant...
```

Or via CLI:

```bash
stn agent create my-coder \
  --description "Coding agent" \
  --prompt "You are a coding assistant." \
  --coding '{"enabled":true}'
```

---

## Workspace Scopes

### Agent Scope (Default)

Workspace is cleaned up after the agent run completes.

```json
{"scope": "agent"}
```

### Workflow Scope

Workspace persists across multiple agent runs in a workflow.

```json
{
  "scope": "workflow",
  "scope_id": "my-workflow-123"
}
```

---

## Observability

### OpenTelemetry Traces

Coding execution is traced:

```
station.agent.execute
  └── claudecode.task              # or opencode.task
        ├── session_id: coding_xxx
        ├── workspace: /tmp/station-coding/...
        ├── cost: 0.0234
        ├── tokens.input: 5000
        ├── tokens.output: 1200
        └── claudecode.tool.Write  # child spans for each tool
              └── tool.name: Write
```

### Result Trace Data

| Field | Description |
|-------|-------------|
| `Trace.SessionID` | Backend session identifier |
| `Trace.Cost` | Execution cost in USD |
| `Trace.Tokens` | Input/output/cache token counts |
| `Trace.Duration` | Total execution time |
| `Trace.ToolCalls` | List of tools the backend used |

---

## Troubleshooting

### Claude Code: Permission Denied

Claude Code runs with `--dangerously-skip-permissions` for non-interactive use. If you see permission errors:

1. Ensure the workspace directory is writable
2. Check claude CLI is authenticated: `claude --version`

### Clone Fails for Private Repo

1. Verify token is set: `echo $GITHUB_TOKEN`
2. Verify config uses `token_env`:
   ```yaml
   git:
     token_env: GITHUB_TOKEN
   ```
3. Check token has `repo` scope (for GitHub)
4. For GitHub Enterprise, ensure the token is authorized for SSO

### Task Timeout

Increase timeout in config:

```yaml
coding:
  claudecode:
    timeout_sec: 600  # 10 minutes
  # or for opencode
  task_timeout_min: 15
```

### Backend Not Found

Ensure the CLI is in PATH:

```bash
# For Claude Code
which claude
claude --version

# For OpenCode
which opencode
opencode --version
```

---

## Backend Comparison

| Feature | Claude Code | OpenCode HTTP | OpenCode CLI |
|---------|-------------|---------------|--------------|
| Setup | `claude login` | Start server | Install binary |
| Auth | Max/Pro or API key | API key | API key |
| Latency | Low (local) | Medium (HTTP) | Low (local) |
| Multi-tenant | No | Yes | No |
| Session Resume | Yes (`--resume`) | Yes | Yes |
| Streaming | Yes | Yes | Yes |
| OTEL Tracing | Yes | Yes | Yes |

Choose **Claude Code** if:
- You have Claude Max/Pro subscription
- You want minimal setup
- You prefer Anthropic's official tooling

Choose **OpenCode** if:
- You need multi-tenant/server deployment
- You want to use different LLM providers
- You need NATS-based distributed architecture
