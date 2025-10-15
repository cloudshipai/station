# MCP Server Templates

Pre-configured MCP server templates for quick installation. Each file contains a ready-to-use MCP server configuration that can be added to your Station environment.

## Usage

Copy the JSON content from any server template and add it to your environment's `template.json` file, or use `stn mcp add` to configure it interactively.

### Example: Adding Memory Server

1. **Via template.json**:
```bash
# Copy the mcpServers content from memory.json to your environment's template.json
cat mcp-servers/memory.json
# Then edit ~/.config/station/environments/your-env/template.json
```

2. **Via sync** (after adding to template.json):
```bash
stn sync your-env
```

## Available Servers

### memory.json
Persistent memory and context management for Claude Code sessions. Store and recall information across conversations and projects.

**Tools provided**:
- Store entities and relations
- Retrieve context-aware information
- Maintain conversation history
- Search knowledge graph

**Use cases**:
- Multi-session projects
- Long-running agent tasks
- Knowledge accumulation
- Context preservation

## Contributing

To add a new MCP server template:

1. Create a new `.json` file in this directory
2. Follow the structure:
```json
{
  "mcpServers": {
    "server-name": {
      "description": "Clear description of what this server does",
      "command": "command-to-run",
      "args": ["arg1", "arg2"]
    }
  }
}
```
3. Update this README with server details and use cases
