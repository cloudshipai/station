# Station Quick Start Guide

Get up and running with Station in under 5 minutes.

## Prerequisites

- `OPENAI_API_KEY` environment variable set
- Linux, macOS, or Windows with WSL2

## 1. Install Station

```bash
curl -fsSL https://raw.githubusercontent.com/cloudshipai/station/main/install.sh | bash
```

## 2. Bootstrap with Examples

```bash
stn bootstrap --openai
```

This command automatically sets up:
- **OpenAI integration** with gpt-4o model
- **Default environment** with filesystem tools
- **Example agents** ready to use
- **Web automation** tools via Playwright

## 3. Verify Installation

```bash
# Check Station status
stn status

# List available agents
stn agent list

# List environments
stn env list
```

## 4. Run Your First Agent

```bash
# Test the Hello World agent
stn agent run "Hello World Agent" "Introduce yourself and tell me what you can do"
```

## 5. Create Your First Custom Agent

```bash
# Create a new agent interactively
stn agent create
```

Follow the prompts to create a file analysis agent with filesystem tools.

## 6. Monitor Execution

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