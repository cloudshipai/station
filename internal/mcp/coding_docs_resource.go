package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

const codingDocumentation = `# Station Coding Backend

Enables agents to delegate coding tasks to AI coding assistants (clone repos, write code, commit, push).

## Backends

| Backend | Description | Requirement |
|---------|-------------|-------------|
| ` + "`claudecode`" + ` | Claude Code CLI | ` + "`claude`" + ` in PATH |
| ` + "`opencode-cli`" + ` | OpenCode CLI (default) | ` + "`opencode`" + ` in PATH |
| ` + "`opencode`" + ` | OpenCode HTTP API | Server at localhost:4096 |
| ` + "`opencode-nats`" + ` | NATS-based OpenCode | NATS + OpenCode |

## Enable for Agent

` + "```json" + `
{"enabled": true, "backend": "claudecode"}
` + "```" + `

## Tools Provided

| Tool | Description |
|------|-------------|
| ` + "`coding_open`" + ` | Start coding session (clone repo, set workspace) |
| ` + "`code`" + ` | Send task to coding backend |
| ` + "`coding_close`" + ` | Close session |
| ` + "`coding_commit`" + ` | Commit changes |
| ` + "`coding_push`" + ` | Push to remote |

## CLI Configuration

` + "```bash" + `
stn config set coding.backend claudecode
stn config set coding.claudecode.model sonnet
stn config set coding.claudecode.timeout_sec 300
` + "```" + `

## Create Agent with Coding

` + "```bash" + `
stn agent create code-fixer \
  --prompt "You fix code issues" \
  --description "Code fixer" \
  --coding '{"enabled": true, "backend": "claudecode"}'
` + "```" + `
`

func (s *Server) handleCodingDocsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     codingDocumentation,
		},
	}, nil
}
