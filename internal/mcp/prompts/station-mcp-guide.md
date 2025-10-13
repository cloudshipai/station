# Station MCP Guide - Proper Usage Workflow

This guide explains the correct way to interact with Station through its MCP interface.

## Core Principle: File-Based Configuration

Station uses a **file-based configuration system** for MCP servers and agents. All configurations live in:
```
~/.config/station/environments/{environment-name}/
├── template.json       # MCP server configurations
├── variables.yml       # Template variables (may contain secrets)
├── agents/            # Agent .prompt files
│   └── agent-name.prompt
└── filesystem.json    # Individual MCP server configs
```

## Correct Workflow for Creating Agents

### Step 1: Check Available Resources First

**ALWAYS start by reading MCP resources to understand what exists:**

```
Read station://environments resource
Read station://mcp-configs resource
Read station://agents resource
```

These resources give you:
- What environments exist
- What MCP servers are configured
- What agents already exist
- What tools are available

### Step 2: Add MCP Server to Environment

Use `mcp__station__add_mcp_server_to_environment` to add MCP servers:

```javascript
{
  "environment_name": "default",
  "server_name": "filesystem",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/workspace"],
  "description": "Filesystem operations for workspace"
}
```

**Important**: When running in Docker mode, use container paths like `/workspace`, NOT host paths.

**DO NOT use `update_raw_mcp_config`** - it bypasses the proper file-based workflow and doesn't create individual server config files.

### Step 3: Restart Server to Load MCP Servers

After adding MCP servers, restart Station to discover tools:
```bash
stn down && stn up --provider openai
```

The server will:
1. Load template.json configurations
2. Connect to each MCP server
3. Discover available tools
4. Store tools in database

### Step 4: List Available Tools

Use `mcp__station__discover_tools` or `mcp__station__list_tools` to see what's available:

```javascript
{
  "environment_id": 1
}
```

This shows all tools discovered from MCP servers like:
- `__read_text_file`
- `__list_directory`
- `__search_files`
- `__directory_tree`
- etc.

### Step 5: Create Agent with Tools

Now create an agent using `mcp__station__create_agent`:

```javascript
{
  "name": "Code Analyzer",
  "description": "Analyzes code structure and explains functions",
  "prompt": "You are a code analysis expert...",
  "environment_id": 1,
  "tool_names": ["__read_text_file", "__search_files", "__directory_tree"],
  "max_steps": 8
}
```

The agent will be assigned the specified tools from the environment's tool pool.

### Step 6: Execute Agent

Use `mcp__station__call_agent` to run the agent:

```javascript
{
  "agent_id": 2,
  "task": "Analyze the main function in cmd/main/main.go"
}
```

### Step 7: Inspect Results

Use `mcp__station__inspect_run` to see execution details:

```javascript
{
  "run_id": 1,
  "verbose": true
}
```

This shows:
- Tool calls made
- Execution steps
- Token usage
- Duration
- Final response

## Common MCP Servers

### Filesystem MCP Server
```javascript
{
  "server_name": "filesystem",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/workspace"]
}
```

Provides: file read/write, directory listing, search operations

### Ship Security Tools MCP Server
```javascript
{
  "server_name": "ship",
  "command": "ship",
  "args": ["mcp", "security", "--stdio"]
}
```

Provides: 300+ security tools (checkov, trivy, gitleaks, semgrep, etc.)

### Brave Search MCP Server
```javascript
{
  "server_name": "brave-search",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-brave-search@latest"]
}
```

Provides: web search capabilities

## What NOT to Do

❌ **Don't use `update_raw_mcp_config` for normal operations**
- This is a low-level tool that bypasses proper workflow
- Use `add_mcp_server_to_environment` instead

❌ **Don't use CLI commands when MCP tools are available**
- Prefer `mcp__station__*` tools over `stn` CLI via Bash
- MCP tools provide structured responses and better error handling

❌ **Don't try to sync environments via CLI in Docker mode**
- The `stn sync` command doesn't work properly in Docker mode
- Server restart is required to reload configurations

❌ **Don't forget Docker container paths**
- Use `/workspace` not `/Users/...` when running in Docker
- Check if you're in Docker mode vs native mode

## Understanding Variables and Security

Station's file-based system supports template variables in `variables.yml`:

```yaml
PROJECT_ROOT: "/workspace"
API_KEY: "secret-key-123"
DATABASE_URL: "postgresql://..."
```

These variables:
- Can be referenced in template.json as `{{ .PROJECT_ROOT }}`
- May contain secrets
- Are resolved at sync time
- Are NOT exposed to LLMs during sync (security feature)

This is why you can't manually trigger sync via MCP - it would expose secrets.

## Resources vs Tools

Station MCP uses MCP specification's distinction:

**Resources (Read-Only)**:
- `station://environments` - List all environments
- `station://agents` - List all agents
- `station://agents/{id}` - Get agent details
- `station://environments/{id}/tools` - List environment tools
- `station://mcp-configs` - List MCP configurations

**Tools (Operations with Side Effects)**:
- `mcp__station__create_agent` - Create new agent
- `mcp__station__add_mcp_server_to_environment` - Add MCP server
- `mcp__station__call_agent` - Execute agent
- `mcp__station__update_agent` - Modify agent
- `mcp__station__delete_agent` - Remove agent

## Troubleshooting

### Agent has no tool calls in run metadata
- This is a known issue similar to the runID=0 bug
- Tool calls are assigned correctly in database but not captured during execution
- Check with: `docker exec station-server sqlite3 /home/station/.config/station/station.db "SELECT * FROM agent_tools WHERE agent_id = X;"`

### MCP server connection timeout
- Check container paths (use `/workspace` not host paths)
- Verify MCP server command is correct
- Check logs: `stn logs --tail 50`

### No tools discovered
- Restart server after adding MCP servers
- Check MCP server configuration in template.json
- Verify MCP server process can start successfully

## Example End-to-End Workflow

```javascript
// 1. Check what exists
Read station://environments

// 2. Add filesystem MCP server
mcp__station__add_mcp_server_to_environment({
  environment_name: "default",
  server_name: "filesystem",
  command: "npx",
  args: ["-y", "@modelcontextprotocol/server-filesystem@latest", "/workspace"]
})

// 3. Restart (via bash)
stn down && stn up --provider openai

// 4. Discover tools
mcp__station__discover_tools({ environment_id: 1 })

// 5. Create agent
mcp__station__create_agent({
  name: "Code Analyzer",
  description: "Analyzes code",
  prompt: "You are a code expert...",
  environment_id: 1,
  tool_names: ["__read_text_file", "__search_files"],
  max_steps: 8
})

// 6. Run agent
mcp__station__call_agent({
  agent_id: 2,
  task: "Analyze main.go"
})

// 7. Inspect results
mcp__station__inspect_run({ run_id: 1, verbose: true })
```

This is the correct, production-ready workflow for working with Station via MCP.
