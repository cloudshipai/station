package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// codingDocumentation contains the comprehensive documentation for OpenCode AI coding backend configuration
const codingDocumentation = `# Station OpenCode Coding Backend Configuration

OpenCode integration enables agents to delegate complex coding tasks to a full-featured AI coding assistant with file system access, git operations, and code execution capabilities.

---

## Quick Start

### Enable Coding for an Agent

` + "```json" + `
{
  "enabled": true,
  "backend": "opencode"
}
` + "```" + `

### With Default Workspace

` + "```json" + `
{
  "enabled": true,
  "backend": "opencode",
  "workspace_path": "/tmp/my-project"
}
` + "```" + `

---

## Configuration Schema

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| enabled | bool | false | Enable coding tools for this agent |
| backend | string | "opencode" | Coding backend to use (currently only "opencode") |
| workspace_path | string | - | Default workspace directory for coding sessions |

---

## Tools Provided

When coding is enabled, the agent receives these tools:

| Tool | Description |
|------|-------------|
| ` + "`coding_open`" + ` | Start a coding session with optional workspace, scope, and scope_id |
| ` + "`code`" + ` | Send a coding task to OpenCode (e.g., "Add health check endpoint") |
| ` + "`coding_close`" + ` | Close the coding session and collect file changes |
| ` + "`coding_commit`" + ` | Commit changes with a message, returns commit hash and stats |
| ` + "`coding_push`" + ` | Push commits to remote repository |

---

## Session Scoping

### Agent Scope (Default)

Each agent run gets its own isolated workspace:

` + "```json" + `
{
  "enabled": true,
  "backend": "opencode"
}
` + "```" + `

### Workflow Scope

Share workspace across multiple workflow steps:

` + "```yaml" + `
# Agent 1 dotprompt
coding:
  enabled: true
  backend: opencode
# When called via coding_open with scope="workflow", scope_id="workflow-run-123"

# Agent 2 in same workflow automatically shares the workspace
` + "```" + `

---

## Examples

### Code Assistant Agent

` + "```json" + `
{
  "enabled": true,
  "backend": "opencode",
  "workspace_path": "/workspace/my-repo"
}
` + "```" + `

Agent prompt:
` + "```" + `
You are a coding assistant. Use coding_open to start a session,
then use the code tool to implement requested features.
Always commit changes with coding_commit before closing.
` + "```" + `

### Multi-Step Workflow

1. **Step 1: Implement Feature**
   - Agent opens workspace with scope="workflow"
   - Implements the requested feature
   - Does NOT close session

2. **Step 2: Write Tests**
   - Agent connects to same workflow-scoped workspace
   - Writes tests for the feature
   - Does NOT close session

3. **Step 3: Review and Commit**
   - Agent reviews changes
   - Commits with descriptive message
   - Closes session with cleanup

---

## Station Config (Global)

Configure OpenCode backend in Station's config.yaml:

` + "```yaml" + `
coding:
  backend: opencode
  opencode:
    url: http://localhost:4096
  max_attempts: 3
  task_timeout_min: 10
  clone_timeout_sec: 300
  push_timeout_sec: 120
  workspace_base_path: /tmp/station-coding
  cleanup_policy: on_session_end  # or "manual", "on_success"
  git:
    token_env: GITHUB_TOKEN       # Read token from env var
    user_name: "Station Bot"
    user_email: "bot@example.com"
` + "```" + `

---

## Git Operations

### Credentials

**Local/stdio mode**: Uses host git credentials by default.

**Container/serve mode**: Requires explicit configuration:

` + "```yaml" + `
coding:
  git:
    token_env: GITHUB_TOKEN  # Environment variable with GitHub token
    # OR
    token: ${GITHUB_TOKEN}   # Direct value with env expansion
` + "```" + `

### Commit Flow

` + "```" + `
1. Agent makes changes via code tool
2. Agent calls coding_commit with message
3. coding_commit returns: commit_hash, files_changed, insertions, deletions
4. Agent calls coding_push (optional)
` + "```" + `

---

## Observability

OpenCode executions are traced with OTEL:

` + "```" + `
station.agent.execute
  └── opencode.task (session_id, workspace, model, cost, tokens)
        ├── opencode.tool.bash
        ├── opencode.tool.read
        ├── opencode.tool.write
        └── ...
` + "```" + `

Captured metrics:
- Model used (e.g., claude-opus-4-5)
- Provider (e.g., anthropic)
- Token usage (input, output, reasoning, cache)
- Execution cost
- Tool calls count

---

## Using in MCP Agent Creation

### Via MCP Tool

` + "```" + `
Tool: create_agent
Parameters:
  name: "code-assistant"
  description: "AI coding assistant with OpenCode"
  prompt: "You are a coding assistant..."
  environment_id: "1"
  coding: '{"enabled": true, "backend": "opencode"}'
` + "```" + `

### Via Dotprompt File

` + "```yaml" + `
---
metadata:
  name: "code-assistant"
  description: "AI coding assistant with OpenCode"
model: gpt-5-mini
max_steps: 10
coding:
  enabled: true
  backend: opencode
  workspace_path: /workspace/my-repo
---

{{role "system"}}
You are a coding assistant. When asked to implement features:
1. Use coding_open to start a session
2. Use code tool to implement the feature
3. Use coding_commit to save changes
4. Use coding_close to end the session

{{role "user"}}
{{userInput}}
` + "```" + `

---

## Prerequisites

1. OpenCode must be running:
` + "```bash" + `
opencode serve --port 4096
` + "```" + `

2. Station config must have OpenCode URL configured (or use default localhost:4096)

3. For private repos, configure git credentials in Station config

---

## Related Resources

- **Sandbox Docs**: ` + "`station://docs/sandbox`" + ` (for compute/code sandbox modes)
- **Workflow DSL**: ` + "`station://docs/workflow-dsl`" + ` (for multi-step workflows)
- **Agent List**: ` + "`station://agents`" + `
`

// handleCodingDocsResource returns the comprehensive OpenCode coding documentation
func (s *Server) handleCodingDocsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     codingDocumentation,
		},
	}, nil
}
