# Station Quick Start Guide

Get up and running with Station in under 5 minutes.

## Prerequisites

- **AI Provider** - Choose one:
  - `CLOUDSHIPAI_REGISTRATION_KEY` or `STN_CLOUDSHIP_KEY` (Recommended)
  - `OPENAI_API_KEY` environment variable
  - `ANTHROPIC_API_KEY` or `GEMINI_API_KEY`
- Linux, macOS, or Windows with WSL2

## 1. Install Station

```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

## 2. Initialize Station

```bash
# CloudShip AI (recommended - auto-detected when key is set)
export CLOUDSHIPAI_REGISTRATION_KEY="csk-..."
stn init --ship

# Or for OpenAI:
# export OPENAI_API_KEY="sk-..."
# stn init --provider openai --ship
```

This command automatically sets up:
- **CloudShip AI** (or your chosen provider) integration
- **Default environment** with filesystem tools
- **Ship CLI** for additional MCP tools

## 3. Configure in Your AI Editor

Add Station to your MCP settings. Choose your editor:

**Claude Code:**
```bash
claude mcp add --transport stdio station -e OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 -- stn stdio
```

**Claude Desktop / Cursor:**

Add to your MCP config file:
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

See [MCP Integration Guide](./agents/MCP_INTEGRATION.md) for config file locations.

## 4. Install Editor Plugins (Optional)

Get enhanced skills, slash commands, and documentation:

**Claude Code:**
```bash
/plugin marketplace add cloudshipai/station
/plugin install station@cloudshipai-station
```

**OpenCode:**
```bash
# Copy skill to your project
cp -r station/opencode-plugin/.opencode .
```

## 5. Verify Installation

```bash
# Check Station status
stn status

# List available agents
stn agent list

# List environments
stn env list
```

## 6. Run Your First Agent

```bash
# Test the Hello World agent
stn agent run "Hello World Agent" "Introduce yourself and tell me what you can do"
```

## 7. Create Your First Custom Agent

```bash
# Create a new agent interactively
stn agent create
```

Follow the prompts to create a file analysis agent with filesystem tools.

## 8. Monitor Execution

```bash
# List recent runs
stn runs list

# Get detailed execution info  
stn runs inspect <run-id> -v

# Follow real-time execution
stn agent run "My Agent" "task" --tail
```

## Next Steps

- **[Agent Creation Guide](./agents/CREATING_AGENTS.md)** - Build advanced agents
- **[MCP Integration](./agents/MCP_INTEGRATION.md)** - Add powerful tools
- **[Bundle System](./bundles/BUNDLE_SYSTEM.md)** - Package and share agents