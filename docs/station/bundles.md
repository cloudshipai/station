# Bundles

Package and share agents with MCP configurations.

## What are Bundles?

Bundles package everything needed to run agents:
- Agent definitions (`.prompt` files)
- MCP server configurations (`template.json`)
- Variable definitions (`variables.yml`)

## Creating Bundles

```bash
# Export environment with agents + MCP configs
stn bundle create my-finops-agents

# Generates: my-finops-agents.tar.gz
```

## Installing Bundles

```bash
# Install bundle on any Station instance
stn bundle install my-finops-agents.tar.gz

# Agents are immediately available
stn agent list
```

## Bundle Structure

[Content to be added]

## Sharing Bundles

[Content to be added]

## Registry Integration

[Content to be added - link to bundle registry]
