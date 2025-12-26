package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// sandboxDocumentation contains the comprehensive documentation for agent sandbox configuration
const sandboxDocumentation = `# Station Agent Sandbox Configuration

Sandbox mode enables agents to execute code in isolated Docker containers. This is useful for data processing, scripting, automation, and running untrusted code safely.

---

## Quick Start

### Simple Python Sandbox

` + "```json" + `
{
  "runtime": "python"
}
` + "```" + `

### Python with Packages

` + "```json" + `
{
  "runtime": "python",
  "pip_packages": ["requests", "pandas", "numpy"]
}
` + "```" + `

### Node.js Sandbox

` + "```json" + `
{
  "runtime": "node",
  "npm_packages": ["axios", "lodash"]
}
` + "```" + `

---

## Configuration Schema

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| mode | string | "compute" | Execution mode: "compute" (single execution) or "code" (persistent session) |
| runtime | string | "python" | Runtime environment: "python" or "node" |
| image | string | - | Custom Docker image (overrides runtime) |
| session | string | - | Session ID for "code" mode (enables state persistence) |
| timeout_seconds | int | 30 | Maximum execution time per run |
| max_stdout_bytes | int | 1048576 | Maximum stdout size (1MB default) |
| allow_network | bool | false | Enable network access in sandbox |
| pip_packages | []string | [] | Python packages to install (pip) |
| npm_packages | []string | [] | Node.js packages to install (npm) |
| limits | object | - | Resource limits (see below) |

### Resource Limits

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| cpu_millicores | int | 1000 | CPU limit (1000 = 1 core) |
| memory_mb | int | 512 | Memory limit in MB |
| timeout_seconds | int | 30 | Execution timeout |
| workspace_mb | int | 100 | Disk space limit in MB |

---

## Execution Modes

### Compute Mode (Default)

Single execution with fresh state each time. Ideal for:
- Data processing tasks
- Script execution
- Stateless operations

` + "```json" + `
{
  "mode": "compute",
  "runtime": "python",
  "timeout_seconds": 60
}
` + "```" + `

### Code Mode (Persistent Session)

Maintains state between executions. Ideal for:
- Interactive development
- Iterative data exploration
- Building on previous results

` + "```json" + `
{
  "mode": "code",
  "runtime": "python",
  "session": "data-analysis-session-1"
}
` + "```" + `

**Note:** In code mode, variables and imports persist between runs within the same session.

---

## Examples

### Data Analysis Agent

` + "```json" + `
{
  "runtime": "python",
  "pip_packages": ["pandas", "numpy", "matplotlib"],
  "timeout_seconds": 120,
  "limits": {
    "memory_mb": 1024,
    "cpu_millicores": 2000
  }
}
` + "```" + `

### Web Scraping Agent

` + "```json" + `
{
  "runtime": "python",
  "pip_packages": ["requests", "beautifulsoup4", "lxml"],
  "allow_network": true,
  "timeout_seconds": 60
}
` + "```" + `

### TypeScript/Node Agent

` + "```json" + `
{
  "runtime": "node",
  "npm_packages": ["typescript", "ts-node", "axios"],
  "timeout_seconds": 30
}
` + "```" + `

### Custom Docker Image

` + "```json" + `
{
  "image": "my-registry.io/my-custom-image:v1.0",
  "timeout_seconds": 120,
  "limits": {
    "memory_mb": 2048
  }
}
` + "```" + `

### Minimal Configuration

` + "```json" + `
{"runtime": "python"}
` + "```" + `

---

## Using Sandbox in Agent Creation

### Via MCP Tool

` + "```" + `
Tool: create_agent
Parameters:
  name: "data-processor"
  description: "Processes CSV data with pandas"
  prompt: "You are a data processing agent..."
  environment_id: "1"
  sandbox: '{"runtime": "python", "pip_packages": ["pandas"]}'
` + "```" + `

### Via Dotprompt File

Add sandbox configuration to the YAML frontmatter:

` + "```yaml" + `
---
metadata:
  name: "data-processor"
  description: "Processes CSV data with pandas"
model: gpt-5-mini
max_steps: 5
sandbox:
  runtime: python
  pip_packages:
    - pandas
    - numpy
  timeout_seconds: 60
  limits:
    memory_mb: 1024
---

{{role "system"}}
You are a data processing agent. Use the execute_code tool to run Python code.

{{role "user"}}
{{userInput}}
` + "```" + `

---

## Security Considerations

1. **Network Access**: Disabled by default. Enable with ` + "`allow_network: true`" + ` only when needed.
2. **Resource Limits**: Set appropriate limits to prevent runaway processes.
3. **Timeouts**: Always set reasonable timeouts to prevent hanging executions.
4. **Custom Images**: Only use trusted Docker images from verified registries.
5. **Package Installation**: Be aware that pip/npm packages are installed at runtime and could introduce vulnerabilities.

---

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Package installation fails | Network disabled | Set ` + "`allow_network: true`" + ` |
| Execution timeout | Insufficient time | Increase ` + "`timeout_seconds`" + ` |
| Memory errors | Insufficient memory | Increase ` + "`limits.memory_mb`" + ` |
| File write fails | Disk space limit | Increase ` + "`limits.workspace_mb`" + ` |

### Debugging

1. Check agent execution logs for sandbox-related errors
2. Verify Docker is running and accessible
3. Test with a minimal sandbox config first
4. Gradually add packages and limits

---

## API Reference

### Create Agent with Sandbox

` + "```" + `
Tool: create_agent
Required: name, description, prompt, environment_id
Optional: sandbox (JSON string)
` + "```" + `

### Update Agent Sandbox

` + "```" + `
Tool: update_agent
Required: agent_id
Optional: sandbox (JSON string, set to "{}" to remove)
` + "```" + `

---

## Related Resources

- **Workflow DSL**: ` + "`station://docs/workflow-dsl`" + `
- **Agent List**: ` + "`station://agents`" + `
- **Environment Tools**: ` + "`station://environments/{id}/tools`" + `
`

// handleSandboxDocsResource returns the comprehensive Sandbox documentation
func (s *Server) handleSandboxDocsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "text/markdown",
			Text:     sandboxDocumentation,
		},
	}, nil
}
