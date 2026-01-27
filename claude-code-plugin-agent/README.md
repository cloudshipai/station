# Station Agent Plugin for Claude Code

This plugin provides a **pre-configured Station subagent** with full MCP integration for Claude Code. The subagent has deep knowledge of Station's 55+ MCP tools and can autonomously manage your AI agent orchestration.

## When to Use This Plugin vs Skills-Only Plugin

| Plugin | Best For |
|--------|----------|
| **`station-agent`** (this plugin) | Power users who want autonomous Station operations via a dedicated subagent |
| **`station`** (skills-only) | Users who prefer lightweight skills that guide the main Claude conversation |

**Choose this plugin if you want:**
- A dedicated Station expert subagent that Claude can delegate to
- Autonomous agent creation, execution, and debugging
- Cleaner main conversation context (Station work happens in subagent)

**Choose the skills-only plugin if you want:**
- Lightweight integration without subagent overhead
- Direct control over Station operations in main conversation
- Smaller context footprint

## Installation

### From GitHub Marketplace

```bash
# Add the Station marketplace
/plugin marketplace add cloudshipai/station

# Install the agent plugin
/plugin install station-agent@cloudshipai-station
```

### Local Installation

```bash
/plugin install ./station/claude-code-plugin-agent
```

## Prerequisites

1. **Station CLI installed**: `stn --version`
2. **Station initialized**: `stn init --provider openai` (or anthropic/gemini)
3. **Start Jaeger for tracing** (recommended): `stn jaeger up`

## What's Included

### Station Operator Subagent

A specialized subagent with:
- Full access to Station's 55+ MCP tools
- Deep knowledge of agent creation patterns (dotprompt format)
- Understanding of multi-agent hierarchies
- Workflow and approval handling capabilities
- Debugging and troubleshooting expertise

### MCP Server

The plugin configures Station as an MCP server (`stn stdio`) with:
- OpenTelemetry tracing to Jaeger (`http://localhost:4318`)
- Access to all Station MCP tools

## Usage

After installation, Claude Code can automatically delegate Station tasks to the subagent:

```
Create a Station agent that monitors Kubernetes pods and alerts on failures
```

```
Debug my last Station agent run - it failed unexpectedly
```

```
Set up a multi-agent team with a coordinator and three specialists
```

Or explicitly invoke the subagent:

```
Use the station-operator subagent to create a new environment for production
```

## Tracing Setup

For full observability, start Jaeger before using Station:

```bash
stn jaeger up
```

Then view traces at: http://localhost:16686

The subagent will remind you about this on first interaction.

## Subagent Capabilities

The `station-operator` subagent can:

| Capability | MCP Tools Used |
|------------|----------------|
| Create/manage agents | `create_agent`, `update_agent`, `delete_agent`, `list_agents` |
| Execute agents | `call_agent`, `list_runs`, `inspect_run` |
| Manage environments | `list_environments`, `create_environment` |
| Configure MCP servers | `add_mcp_server_to_environment`, `discover_tools` |
| Handle workflows | `execute_workflow`, `list_approvals`, `approve_step` |
| Work with bundles | `list_bundles`, `get_bundle` |

## Comparison: Skills vs Agent Plugin

```
station/ (skills-only)           station-agent/ (this plugin)
├── skills/                      ├── agents/
│   ├── station/                 │   └── station-operator.md  <- Subagent definition
│   ├── station-agents/          │
│   ├── station-mcp/             ├── .claude-plugin/
│   └── ...                      │   └── plugin.json          <- MCP config
├── commands/                    │
│   └── ...                      └── README.md
└── .claude-plugin/
    └── plugin.json
```

**Skills-only**: Knowledge injected into main Claude context when relevant
**Agent plugin**: Dedicated subagent with separate context and MCP access

## Structure

```
claude-code-plugin-agent/
├── .claude-plugin/
│   └── plugin.json          # Plugin manifest with MCP config
├── agents/
│   └── station-operator.md  # Station operator subagent
└── README.md
```

## Documentation

- [Station Documentation](https://docs.cloudship.ai/station/overview)
- [Claude Code Plugins](https://code.claude.com/docs/en/plugins)
- [Claude Code Subagents](https://code.claude.com/docs/en/sub-agents)

## License

MIT - See [LICENSE](../LICENSE)
