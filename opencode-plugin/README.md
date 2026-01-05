# Station OpenCode Plugin

OpenCode skills for [Station](https://github.com/cloudshipai/station) - self-hosted AI agent orchestration platform.

## Installation

### Option 1: Project-Local (Recommended)

Copy to your project root - OpenCode auto-discovers skills at startup:

```bash
# From Station repo
cp -r opencode-plugin/.opencode /path/to/your/project/

# Or if you cloned Station
cp -r station/opencode-plugin/.opencode .
```

### Option 2: Global Installation

Install globally so the skill is available in all projects:

```bash
# Linux/macOS
mkdir -p ~/.config/opencode/skill
cp opencode-plugin/.opencode/skill/station.md ~/.config/opencode/skill/

# Or copy entire skill directory
cp -r opencode-plugin/.opencode ~/.config/opencode/
```

### Option 3: MCP Server Only

If you just want the MCP tools without the skill:

Add to `~/.config/opencode/config.json` or project `opencode.json`:

```json
{
  "mcp": {
    "station": {
      "command": ["stn", "stdio"],
      "environment": {
        "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
      }
    }
  }
}
```

## Skills Included

| Skill | Description |
|-------|-------------|
| `station.md` | Comprehensive Station CLI reference including agents, workflows, MCP, deployment, and benchmarking |

## CLI-First Approach

This plugin guides OpenCode to prefer `stn` CLI for:
- File operations and setup
- Configuration syncing
- Deployment and container management
- Benchmarking and evaluation

MCP tools are preferred for:
- Programmatic agent execution during conversations
- Querying run results and agent metadata
- Dynamic tool discovery

## Structure

```
.opencode/
└── skill/
    └── station.md          # Combined Station skill
```

## Quick Reference

```bash
stn init --provider openai --ship    # Initialize
stn agent list/run/show/delete       # Agent management
stn mcp add/list/tools/status        # MCP configuration
stn workflow list/run/approvals      # Workflows
stn bundle install/create/share      # Bundles
stn sync <env> --browser             # Sync with secure input
stn serve                            # Start web UI (:8585)
stn up/down                          # Docker container mode
stn deploy <env> --target fly        # Deploy to Fly.io
stn benchmark evaluate <run-id>      # Evaluate agent quality
stn report create/generate/show      # Performance reports
stn jaeger up                        # Start tracing (:16686)
```

## Verify Installation

After copying, restart OpenCode. The skill should be available:

```
# Ask OpenCode to use the Station skill
"Load the station skill and show me how to create an agent"
```

## Related

- [Claude Code Plugin](../claude-code-plugin/) - Plugin with skills and slash commands for Claude Code
- [Station Documentation](https://docs.cloudshipai.com/station/overview)
- [Station GitHub](https://github.com/cloudshipai/station)
