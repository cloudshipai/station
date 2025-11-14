# Station Develop Mode - Quick Start

## Overview

`stn develop` launches a Genkit development playground where you can interactively test agents and MCP tools.

## Usage

### Option 1: Manual Two-Step (Recommended)

**Terminal 1: Start Station develop server**
```bash
stn develop --env default
```

This will:
- Load all agents from `~/.config/station/environments/default/agents/*.prompt`
- Connect to all MCP servers defined in `template.json`
- Register all tools as Genkit actions
- Keep the process running as a reflection server

**Terminal 2: Launch Genkit UI**
```bash
genkit start --port 4000
```

Then open: http://localhost:4000

### Option 2: One Command (Integrated)

```bash
genkit start -- stn develop --env default
```

This launches both the Genkit UI and Station develop server together.

## Features

### Multi-Agent Hierarchy Support
- Agents can call other agents as tools using `__agent_*` prefix
- Example: Orchestrator agent calls specialist agents
- All agent-to-agent calls are visible in the Genkit UI

### Live Agent Testing
- Test agents with different inputs
- See step-by-step execution with tool calls
- View token usage and execution time
- Debug failures with detailed logs

### MCP Tool Discovery
- All MCP tools from your environment are available
- Tools are registered as Genkit actions
- Test tools independently before using in agents

## Troubleshooting

### Issue: Panic "name is required"

**Cause**: Invalid `.prompt` file with empty `name` field

**Fix**: Remove or fix the invalid prompt file
```bash
# Find invalid prompts
find ~/.config/station/environments/*/agents -name "*.prompt" -exec grep -l 'name: ""' {} \;

# Remove the problematic file (usually a hidden .prompt file)
rm ~/.config/station/environments/default/agents/.prompt
```

### Issue: Develop hangs on startup

**Cause**: Faker MCP servers taking too long to initialize

**Solution**: 
1. Our timeout fixes (from Nov 10, 2025) should resolve this
2. Use environments without faker for quick testing
3. Ensure target directories exist for filesystem fakers

### Issue: Tools not showing in Genkit UI

**Cause**: MCP server connection failed during startup

**Fix**:
```bash
# Check MCP server status
stn mcp list --environment default

# Verify tools are discovered
stn mcp tools | head -20
```

## Best Practices

1. **Start Simple**: Test with basic agents first
2. **Use Specific Environments**: Create dev-specific environments for testing
3. **Check Logs**: Watch Terminal 1 for connection errors
4. **Iterate Fast**: Edit `.prompt` files and restart `stn develop` to reload

## Examples

### Test a Simple Agent
1. Start: `stn develop --env default`
2. Open Genkit UI: http://localhost:4000
3. Select your agent from the dropdown
4. Enter a test input
5. Run and observe execution

### Test Multi-Agent Workflow
1. Create orchestrator with `__agent_*` tools
2. Start develop mode
3. In Genkit UI, run orchestrator
4. See agent calls to specialists in execution trace

## Related Commands

```bash
# List available agents
stn agent list --env default

# List MCP tools  
stn mcp tools

# View execution history
stn runs list

# Inspect specific run
stn runs inspect <run-id> -v
```

## Architecture Notes

**How it works:**
- `stn develop` runs as a Genkit reflection server
- GenKit UI connects via HTTP to query agents/tools
- All agent executions happen in the Go process
- MCP connections stay alive during the session
- Press Ctrl+C to shutdown cleanly

**vs Production:**
- Develop: Interactive UI, manual testing, live debugging
- Production: Automated execution, scheduled runs, API calls
