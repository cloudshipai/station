# MCP Server Component

Station's **Model Context Protocol (MCP) server** provides AI systems with management tools for creating, executing, and managing agents. This is how external AI systems (like Claude Code) interact with Station programmatically.

## 🏗️ MCP Server Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    STATION MCP SERVER                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │    Tools    │  │  Resources  │  │   Prompts   │            │
│  │             │  │             │  │             │            │
│  │ - create_   │  │ - agents    │  │ - agent_    │            │
│  │   agent     │  │ - envs      │  │   creation  │            │
│  │ - call_     │  │ - configs   │  │ - tool_     │            │
│  │   agent     │  │ - runs      │  │   suggest   │            │
│  │ - 11+ more  │  │             │  │             │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                │                │                    │
│         └────────────────┼────────────────┘                    │
│                          │                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               MCP PROTOCOL LAYER                        │   │
│  │                                                         │   │
│  │  JSON-RPC 2.0 │ Tool Calls │ Resource Access │ etc.   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          │                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                STATION CORE                             │   │
│  │                                                         │   │
│  │  Services │ Repositories │ Database │ File System      │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 🎯 Connection Modes

### **1. Stdio Mode** (Primary)
```bash
# Start Station MCP server
./stn stdio

# AI systems connect via stdin/stdout
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "create_agent",
    "arguments": {
      "name": "Test Agent",
      "description": "Test agent description"
    }
  }
}
```

### **2. HTTP Mode** (Alternative)  
```bash
# Start with HTTP transport (port 3000)
./stn serve --mcp-port 3000

# AI systems connect via HTTP
POST http://localhost:3000/mcp
Content-Type: application/json

{
  "jsonrpc": "2.0", 
  "method": "tools/call",
  "params": {...}
}
```

## 🛠️ Available Tools (11+ Management Tools)

### **Agent Management Tools**
| Tool | Purpose | Parameters | Return |
|------|---------|------------|--------|
| `create_agent` | Create new AI agent | name, description, prompt, environment_id, max_steps, tool_names | Agent object with ID |
| `call_agent` | Execute agent with task | agent_id, task, async, timeout, store_run | Execution result |
| `update_agent` | Modify agent config | agent_id, name, description, prompt, max_steps | Updated agent |
| `delete_agent` | Remove agent | agent_id | Success confirmation |
| `get_agent_details` | Get agent info | agent_id | Complete agent details |
| `list_agents` | List all agents | enabled_only, environment_id | Array of agents |

### **Environment Management Tools**
| Tool | Purpose | Parameters | Return |
|------|---------|------------|--------|
| `list_environments` | List all environments | none | Array of environments |
| `list_tools` | List available MCP tools | environment_id, search | Array of tools |
| `discover_tools` | Intelligent tool discovery | config_id, environment_id | Tool analysis |

### **Configuration Management Tools**
| Tool | Purpose | Parameters | Return |
|------|---------|------------|--------|
| `list_mcp_configs` | List MCP configurations | environment_id | Array of configs |
| `suggest_agent_config` | AI-powered agent setup | user_request, domain | Suggested configuration |

## 📊 MCP Resources (Read-Only Data)

Station exposes read-only data through MCP resources:

```
station://agents              # List all agents
station://agents/{id}         # Specific agent details  
station://environments        # List environments
station://environments/{id}/tools  # Tools in environment
station://agents/{id}/runs    # Agent execution history
station://mcp-configs         # MCP server configurations
```

**Usage Example**:
```json
{
  "jsonrpc": "2.0",
  "method": "resources/read",
  "params": {
    "uri": "station://agents"
  }
}
```

## 🎨 Tool Implementation Pattern

### **Tool Definition** (`internal/mcp/tools_setup.go`)
```go
func (s *Server) setupTools() {
    // Define tool with parameters
    createAgentTool := mcp.NewTool("create_agent",
        mcp.WithDescription("Create a new AI agent"),
        mcp.WithString("name", mcp.Required(), mcp.Description("Agent name")),
        mcp.WithString("description", mcp.Required(), mcp.Description("Agent description")),
        mcp.WithString("prompt", mcp.Required(), mcp.Description("Agent system prompt")),
        mcp.WithString("environment_id", mcp.Description("Environment ID (default: 1)")),
        mcp.WithNumber("max_steps", mcp.Description("Maximum execution steps (default: 5)")),
        mcp.WithArray("tool_names", mcp.Description("Array of tool names to assign")),
    )
    
    // Register tool with handler
    s.mcpServer.AddTool(createAgentTool, s.handleCreateAgent)
}
```

### **Tool Handler** (`internal/mcp/handlers_fixed.go`)
```go
func (s *Server) handleCreateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. Extract and validate parameters
    name, err := request.RequireString("name")
    if err != nil {
        return mcp.NewToolResultError("Missing name parameter"), nil
    }
    
    description, err := request.RequireString("description")  
    if err != nil {
        return mcp.NewToolResultError("Missing description parameter"), nil
    }
    
    // 2. Process optional parameters with defaults
    environmentID := request.GetInt("environment_id", 1)
    maxSteps := request.GetInt("max_steps", 5)
    
    // 3. Extract tool assignment array
    var toolNames []string
    if request.Params.Arguments != nil {
        if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
            if toolNamesArg, ok := argsMap["tool_names"]; ok {
                if toolNamesArray, ok := toolNamesArg.([]interface{}); ok {
                    for _, toolName := range toolNamesArray {
                        if str, ok := toolName.(string); ok {
                            toolNames = append(toolNames, str)
                        }
                    }
                }
            }
        }
    }
    
    // 4. Create agent via repository
    createdAgent, err := s.repos.Agents.Create(name, description, prompt, int64(maxSteps), environmentID, 1, nil, true)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
    }
    
    // 5. Assign tools to agent
    var assignedTools []string
    var skippedTools []string
    if len(toolNames) > 0 {
        for _, toolName := range toolNames {
            tool, err := s.repos.MCPTools.FindByNameInEnvironment(environmentID, toolName)
            if err != nil {
                skippedTools = append(skippedTools, fmt.Sprintf("%s (not found)", toolName))
                continue
            }
            
            _, err = s.repos.AgentTools.AddAgentTool(createdAgent.ID, tool.ID)
            if err != nil {
                skippedTools = append(skippedTools, fmt.Sprintf("%s (failed: %v)", toolName, err))
                continue
            }
            
            assignedTools = append(assignedTools, toolName)
        }
    }
    
    // 6. Build comprehensive response
    response := map[string]interface{}{
        "success": true,
        "agent": map[string]interface{}{
            "id":             createdAgent.ID,
            "name":           createdAgent.Name,
            "description":    createdAgent.Description,
            "max_steps":      createdAgent.MaxSteps,
            "environment_id": createdAgent.EnvironmentID,
        },
        "message": fmt.Sprintf("Agent '%s' created successfully", name),
    }
    
    // 7. Add tool assignment status
    if len(toolNames) > 0 {
        toolAssignment := map[string]interface{}{
            "requested_tools": toolNames,
            "assigned_tools":  assignedTools,
            "assigned_count":  len(assignedTools),
        }
        
        if len(skippedTools) > 0 {
            toolAssignment["skipped_tools"] = skippedTools
            toolAssignment["status"] = "partial"
        } else {
            toolAssignment["status"] = "success"
        }
        
        response["tool_assignment"] = toolAssignment
    }
    
    // 8. Return formatted JSON response
    resultJSON, _ := json.MarshalIndent(response, "", "  ")
    return mcp.NewToolResultText(string(resultJSON)), nil
}
```

## 🚀 Server Startup & Configuration

### **Server Initialization** (`internal/mcp/server.go`)
```go
type Server struct {
    mcpServer *server.MCPServer
    repos     *repositories.Repositories
}

func NewServer(repos *repositories.Repositories) *Server {
    mcpServer := server.NewMCPServer()
    
    s := &Server{
        mcpServer: mcpServer,
        repos:     repos,
    }
    
    // Setup all tools and resources
    s.setupTools()
    s.setupResources()
    s.setupPrompts()
    
    return s
}
```

### **Stdio Mode Startup** (`cmd/main/stdio.go`)
```go
func runStdio(cmd *cobra.Command, args []string) error {
    // 1. Load configuration
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    
    // 2. Initialize database
    database, err := db.New(cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("failed to connect to database: %w", err)
    }
    defer database.Close()
    
    // 3. Setup repositories
    repos := repositories.NewRepositories(database)
    
    // 4. Create and start MCP server
    mcpServer := mcp.NewServer(repos)
    
    // 5. Run stdio transport
    return mcpServer.RunStdio()
}
```

## 🎭 AI Integration Examples

### **Claude Code Integration**
```python
# In Claude Code environment, Station MCP tools are available as:

# Create an agent
result = mcp__station__create_agent(
    name="Log Analyzer", 
    description="Analyzes application logs for errors",
    prompt="You are a log analysis specialist...",
    environment_id="1",
    max_steps=10,
    tool_names=["filesystem", "grep", "search"]
)

# Execute the agent  
execution = mcp__station__call_agent(
    agent_id=result["agent"]["id"],
    task="Analyze the logs in /var/log/app.log for any error patterns",
    async=False,
    timeout=300,
    store_run=True
)
```

### **Custom AI System Integration**
```javascript
// Connect to Station MCP server
const mcp = new MCPClient({
  transport: new StdioTransport('./stn stdio')
});

// Create agent
const agent = await mcp.callTool('create_agent', {
  name: 'Code Reviewer',
  description: 'Reviews code for best practices',
  prompt: 'You are an expert code reviewer...',
  tool_names: ['filesystem', 'git', 'grep']
});

// Execute agent
const result = await mcp.callTool('call_agent', {
  agent_id: agent.agent.id,
  task: 'Review the code in ./src/ directory'
});
```

## 🔧 Development Guidelines

### **Adding New Tools**
1. **Define tool** in `tools_setup.go` with parameters and description
2. **Implement handler** in `handlers_fixed.go` following the pattern
3. **Add validation** for required and optional parameters  
4. **Create comprehensive response** with success/error details
5. **Test tool** with MCP client or Claude Code

### **Tool Design Principles**
- **Single responsibility**: Each tool does one thing well
- **Comprehensive responses**: Include success status, data, and error details
- **Parameter validation**: Always validate inputs before processing
- **Error handling**: Provide clear error messages with context
- **Documentation**: Clear descriptions for AI systems to understand

### **Resource Design Principles**
- **Read-only**: Resources provide data access, not modification
- **Consistent URIs**: Use `station://` scheme with logical paths
- **Structured data**: Return JSON with consistent field names
- **Efficient queries**: Optimize database queries for resource access

---
*The MCP server is Station's primary interface for AI integration - it's how external AI systems understand and control Station's agent management capabilities.*