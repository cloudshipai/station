# Troubleshooting Station

Common issues and solutions when using Station.

## MCP Server Issues

### Genkit Schema Conversion Panic

**Symptom**: Agent execution fails with panic message:
```
panic: interface conversion: interface {} is nil, not string
...
github.com/firebase/genkit/go/plugins/googlegenai.toGeminiSchema
```

**Cause**: Some MCP servers (like AWS Cost Explorer) provide tools with nil or malformed schemas that break Genkit's schema conversion for Gemini.

**Diagnosis**: Run the following to check for problematic tools:
```bash
stn mcp tools --filter <server-name>
```

Look for tools with empty descriptions (`-`) or missing schema information.

**Solution**: This is an issue with the MCP server, not Station. Options:
1. Contact the MCP server maintainer to fix schema definitions
2. Use a different MCP server for the same functionality
3. Remove the problematic MCP server configuration

**Example**: The `awslabs.cost-explorer-mcp-server` is known to have this issue with tools like `__get_cost_and_usage` having nil schemas.

### MCP Connection Issues

**Symptom**: "Connection closed" or "Not connected" errors

**Solutions**:
1. Check MCP server configuration: `stn mcp list`
2. Restart Station: `stn status` then restart the process
3. Verify MCP server is running and accessible
4. Check network connectivity if using remote MCP servers

## Agent Issues

### Agent Export Requirements

**Important**: After creating agents via MCP tools, you must export them to save to disk:

```bash
# Export via CLI
stn agent export <agent-name> <environment-name>

# Or use MCP tool
mcp__stn__export_agent with agent_id parameter
```

Without export, agents exist only in the database and may not survive system changes.

## Performance Issues

### Slow Shutdown (SSH/MCP)

**Symptom**: Station takes 1+ minutes to shutdown gracefully

**Known Issue**: Long shutdown times with MCP connections and SSH servers

**Workaround**: Force kill if needed: `pkill -f stn`

**Investigation Needed**: This appears to be related to hanging MCP connections or database locks during cleanup.