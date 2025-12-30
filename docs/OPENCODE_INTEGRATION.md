# OpenCode Integration Guide

Station integrates with [OpenCode](https://opencode.ai) (SST's AI coding assistant) to delegate complex coding tasks. When your Station agent needs to write code, fix bugs, or make changes to a repository, it can hand off the work to OpenCode.

## Architecture Overview

```
Station Agent                    OpenCode
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

**Key Design**: OpenCode owns ALL workspace and git operations. Station just sends tasks and receives results. This enables:
- Multi-tenant OpenCode (one instance serving many Stations)
- Remote Station (no filesystem access to OpenCode needed)
- Git-as-transport for code changes

## Quick Start

### 1. Start OpenCode

```bash
# Option A: Run OpenCode locally
opencode serve --port 4096

# Option B: Run in Docker
docker run -d \
  -p 4096:4096 \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  ghcr.io/sst/opencode:latest \
  opencode serve --hostname 0.0.0.0 --port 4096
```

### 2. Configure Station

Add to your `~/.config/station/config.yaml`:

```yaml
coding:
  backend: opencode
  opencode:
    url: http://localhost:4096
  
  # Timeouts
  task_timeout_min: 10        # Max time for coding tasks
  clone_timeout_sec: 300      # Max time for git clone
  push_timeout_sec: 120       # Max time for git push
  
  # Git credentials (for private repos)
  git:
    token_env: GITHUB_TOKEN   # Read token from this env var
    user_name: "Station Bot"
    user_email: "bot@example.com"
```

### 3. Create a Coding Agent

Create `~/.config/station/environments/default/agents/coder.prompt`:

```yaml
---
model: openai/gpt-4o-mini
coding:
  enabled: true
  backend: opencode
---
You are a coding assistant. When asked to write code or make changes to a repository:

1. Use `coding_open` with the repo_url to clone the repository
2. Use `code` to make the requested changes
3. Use `coding_commit` to commit the changes
4. Use `coding_push` to push to the remote
5. Use `coding_close` to clean up

Always explain what changes you made.

{{userInput}}
```

### 4. Run the Agent

```bash
# Clone a repo and make changes
stn agent run coder "Clone https://github.com/myorg/myrepo and add a health check endpoint at /health"

# Work on a local directory (explicit workspace)
stn agent run coder "Open workspace /path/to/repo and fix the bug in auth.go"
```

## Tools Reference

### coding_open

Opens a coding session. Optionally clones a git repository.

```json
{
  "repo_url": "https://github.com/org/repo.git",  // OpenCode clones this
  "branch": "main",                                // Optional: branch to checkout
  "workspace_path": "/path/to/existing/repo",     // Optional: use existing directory
  "title": "Fix auth bug",                        // Optional: session title
  "scope": "agent",                               // "agent" or "workflow"
  "scope_id": "workflow-123"                      // For workflow scope
}
```

**Returns:**
```json
{
  "session_id": "coding_1234567890",
  "workspace_path": "/tmp/opencode-workspace-xxx",
  "repo_cloned": true,
  "managed": true
}
```

### code

Execute a coding task in the session.

```json
{
  "session_id": "coding_1234567890",  // Required: from coding_open
  "instruction": "Add a /health endpoint that returns {status: ok}",
  "context": "This is a Flask API",   // Optional: additional context
  "files": ["src/api.py"]             // Optional: files to focus on
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
  "tokens_used": 1500,
  "cost": 0.0045
}
```

### coding_commit

Commit changes in the workspace.

```json
{
  "session_id": "coding_1234567890",
  "message": "Add health check endpoint",
  "add_all": true  // Default: true. Runs git add -A before commit
}
```

**Returns:**
```json
{
  "success": true,
  "commit_hash": "abc123def456...",
  "message": "Add health check endpoint",
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
  "remote": "origin",        // Default: origin
  "branch": "feature/health", // Optional: defaults to current branch
  "set_upstream": true       // Add -u flag
}
```

**Returns:**
```json
{
  "success": true,
  "remote": "origin",
  "branch": "feature/health",
  "message": "Pushed to origin/feature/health"
}
```

### coding_close

Close the session and clean up.

```json
{
  "session_id": "coding_1234567890",
  "workspace_id": "ws_xxx",  // Optional: for managed workspaces
  "success": true            // Affects cleanup policy
}
```

## Configuration Reference

### Full Config Example

```yaml
# ~/.config/station/config.yaml

coding:
  # Backend selection
  backend: opencode           # Currently only "opencode" supported
  
  # OpenCode connection
  opencode:
    url: http://localhost:4096
  
  # Retry settings
  max_attempts: 3             # Retry failed API calls
  
  # Timeouts
  task_timeout_min: 10        # Max time for coding tasks (minutes)
  clone_timeout_sec: 300      # Max time for git clone (seconds)
  push_timeout_sec: 120       # Max time for git push (seconds)
  
  # Workspace management
  workspace_base_path: /tmp/station-coding
  cleanup_policy: on_session_end  # "on_session_end", "on_success", "manual"
  
  # Git configuration
  git:
    # Authentication (choose one)
    token_env: GITHUB_TOKEN     # Read from environment variable (recommended)
    # token: ghp_xxxx           # Or hardcode (not recommended)
    
    # Commit author
    user_name: "Station Bot"
    user_email: "bot@cloudship.ai"
```

### Agent-Level Config

Override settings per agent in the dotprompt:

```yaml
---
model: openai/gpt-4o
coding:
  enabled: true
  backend: opencode
  # Agent-specific overrides (future)
---
```

## Private Repositories

### GitHub Token Authentication

1. Create a personal access token with `repo` scope
2. Set the environment variable:
   ```bash
   export GITHUB_TOKEN=ghp_xxxxxxxxxxxx
   ```
3. Configure Station to use it:
   ```yaml
   coding:
     git:
       token_env: GITHUB_TOKEN
   ```

Station automatically injects the token when cloning/pushing:
```
https://x-access-token:TOKEN@github.com/org/repo.git
```

### Security Notes

- Tokens are **never logged** - all output is redacted
- Tokens are passed via HTTPS URL injection (not command line)
- Each session can have isolated credentials

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

Use `GetByScope` to retrieve existing workspace:
```go
ws, err := manager.GetByScope(ScopeWorkflow, "my-workflow-123")
```

## Observability

### Traces

OpenCode execution is traced via OpenTelemetry:

```
station.agent.execute
  └── opencode.task
        ├── session_id: coding_xxx
        ├── model: claude-opus-4-5
        ├── provider: anthropic
        ├── cost: 0.0234
        ├── tokens.input: 5000
        ├── tokens.output: 1200
        └── tool_calls: 5
```

### Result Data

Each coding task returns:

| Field | Description |
|-------|-------------|
| `Trace.Model` | Model used (e.g., claude-opus-4-5) |
| `Trace.Provider` | Provider (anthropic, openai) |
| `Trace.Cost` | Execution cost in USD |
| `Trace.Tokens` | Input/output/cache token counts |
| `Trace.Duration` | Total execution time |
| `Trace.ToolCalls` | List of tools OpenCode used |
| `Trace.Reasoning` | Extended thinking text (if model supports) |

## Troubleshooting

### OpenCode Not Responding

```bash
# Check health
curl http://localhost:4096/global/health

# Check if sessions exist
curl http://localhost:4096/session
```

### Clone Fails for Private Repo

1. Verify token is set: `echo $GITHUB_TOKEN`
2. Verify config uses `token_env`: 
   ```yaml
   git:
     token_env: GITHUB_TOKEN
   ```
3. Check token has `repo` scope

### Task Timeout

Increase timeout in config:
```yaml
coding:
  task_timeout_min: 30  # Increase from default 10
```

### Empty Response from OpenCode

Ensure the `directory` parameter matches the session workspace. This is handled automatically by Station.

## E2E Testing

Run integration tests (requires OpenCode running):

```bash
# Start OpenCode first
opencode serve --port 4096

# Run E2E tests
OPENCODE_E2E=true go test ./internal/coding/... -run TestE2E -v
```

## What's Next

### Workflow Integration (Coming Soon)

Multi-step coding tasks sharing workspace:

```yaml
workflow:
  - agent: analyzer
    task: "Analyze codebase structure"
  - agent: coder
    task: "Implement changes based on analysis"
    workspace_from: analyzer  # Share workspace
  - agent: reviewer
    task: "Review and test changes"
    workspace_from: coder
```

### PR Creation (Coming Soon)

Automatic pull request creation after push:

```json
{
  "tool": "coding_create_pr",
  "input": {
    "session_id": "coding_xxx",
    "title": "Add health check endpoint",
    "body": "Adds /health endpoint for monitoring"
  }
}
```
