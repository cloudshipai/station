# Station Plugin for Claude Code

This plugin integrates [Station](https://github.com/cloudshipai/station) with Claude Code, providing access to 55+ MCP tools for AI agent orchestration.

## Installation

### Option 1: From GitHub (Recommended)

```bash
# Add the Station marketplace
/plugin marketplace add cloudshipai/station

# Install the plugin
/plugin install station@cloudshipai-station
```

### Option 2: Local Installation

If you have Station cloned locally:

```bash
# Install directly from path
/plugin install ./station/claude-code-plugin
```

### Option 3: Manual MCP Only

If you just want the MCP tools without skills/commands:

```bash
claude mcp add station -- stn stdio
```

## Prerequisites

- Station CLI installed (`stn --version`)
- Station initialized (`stn init`)

## What's Included

### MCP Server

The plugin configures Station as an MCP server, giving Claude Code access to:

- **Agent Management**: Create, list, update, delete agents
- **Execution**: Run agents, view execution history, inspect runs
- **Workflows**: Create and manage state machine workflows
- **Environments**: Manage environments and configurations
- **Bundles**: Work with agent bundles

### Slash Commands

| Command | Purpose |
|---------|---------|
| `/station` | Core Station concepts and MCP tools |
| `/station-agent` | Create and manage AI agents |
| `/station-workflow` | Build multi-step workflows |
| `/station-bundle` | Package and distribute bundles |

### Skills

The plugin includes focused skills that teach Claude Code and OpenCode how to use Station effectively:

| Skill | Purpose |
|-------|---------|
| `station` | Core CLI commands, when to use CLI vs MCP, file structure |
| `station-agents` | Creating agents with dotprompt format, multi-agent hierarchies |
| `station-workflows` | State machine workflows with human-in-the-loop |
| `station-mcp` | Adding MCP servers, faker configuration, tool management |
| `station-deploy` | Docker containers, Fly.io deployment, cloud operations |
| `station-benchmark` | LLM-as-judge evaluation, quality metrics, performance reports |

**CLI-first approach**: Skills guide you to prefer CLI for file operations and setup, MCP tools for programmatic execution within conversations.

Skills work with both Claude Code (`.claude/skills/`) and OpenCode (reads `.claude/skills/` or `.opencode/skill/`).

## Usage

After installation, ask Claude Code to work with Station:

```
Create a Station agent that monitors Kubernetes pods
```

```
List my Station agents and run the first one
```

```
/station-workflow
Create a deployment approval workflow
```

## Manual MCP Configuration

If you prefer manual setup, add to `.mcp.json`:

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

> **Note**: The `OTEL_EXPORTER_OTLP_ENDPOINT` enables tracing via Jaeger. Start Jaeger with `stn jaeger up` to view traces at http://localhost:16686.

## Documentation

- [Claude Code Plugin Guide](https://docs.cloudship.ai/station/claude-code)
- [Station Documentation](https://docs.cloudship.ai/station/overview)
- [MCP Tools Reference](https://docs.cloudship.ai/station/mcp-tools)

## Structure

```
claude-code-plugin/
├── .claude-plugin/
│   └── plugin.json          # Plugin manifest with MCP config
├── commands/
│   ├── station.md           # /station command
│   ├── station-agent.md     # /station-agent command
│   ├── station-workflow.md
│   └── station-bundle.md
├── skills/
│   ├── station/
│   │   └── SKILL.md         # Core CLI skill
│   ├── station-agents/
│   │   └── SKILL.md         # Agent creation skill
│   ├── station-workflows/
│   │   └── SKILL.md         # Workflow skill
│   ├── station-mcp/
│   │   └── SKILL.md         # MCP configuration skill
│   ├── station-deploy/
│   │   └── SKILL.md         # Deployment skill
│   └── station-benchmark/
│       └── SKILL.md         # Evaluation skill
└── README.md
```

## License

MIT - See [LICENSE](../LICENSE)
