# MCP Proxy Adapter

A comprehensive proxy server adapter that solves the schema conversion hell in MCP tool integration by staying in pure MCP protocol throughout the entire pipeline.

## Architecture Overview

```
┌─────────────────┐    MCP Protocol    ┌─────────────────┐
│                 │◄──────────────────►│                 │
│  Eino Agent     │                    │  MCP Proxy      │
│  (via MCP-Go)   │                    │  Server         │
│                 │                    │                 │
└─────────────────┘                    └─────────────────┘
                                               │
                                               │ Forwards tool calls
                                               ▼
                                    ┌─────────────────┐
                                    │                 │
                                    │  Client Pool    │
                                    │  Manager        │
                                    │                 │
                                    └─────────────────┘
                                               │
                        ┌──────────────────────┼──────────────────────┐
                        │                      │                      │
                        ▼                      ▼                      ▼
            ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
            │                 │   │                 │   │                 │
            │ Real MCP Server │   │ Real MCP Server │   │ Real MCP Server │
            │ (filesystem)    │   │ (database)      │   │ (web scraper)   │
            │                 │   │                 │   │                 │
            └─────────────────┘   └─────────────────┘   └─────────────────┘
```

## Problems Solved

1. **Schema Conversion Hell**: Eliminates JSON Schema ↔ OpenAPI conversion issues
2. **Tool Composition**: Aggregates tools from multiple MCP servers per agent
3. **Selective Access**: Each agent sees only assigned tools
4. **Clean Abstraction**: Eino sees one simple MCP interface
5. **Pure MCP Protocol**: No schema loss or conversion errors

## Core Components

### 1. MCP Proxy Server (`adapter/proxy_server.go`)
- Main orchestrator that creates agent-specific MCP servers
- Registers selected tools from multiple source servers
- Routes tool calls to appropriate backend servers
- Manages agent sessions and tool filtering

### 2. Client Manager (`adapter/client_manager.go`)
- Maintains connection pool to real MCP servers
- Supports stdio, HTTP, and SSE transports
- Handles connection lifecycle and health checking
- Routes tool calls to correct backend server

### 3. Tool Registry (`adapter/tool_registry.go`)
- Maps tool names to their source servers
- Maintains tool schemas and metadata
- Supports filtering by agent permissions
- Thread-safe operations with RWMutex

### 4. Session Manager (`adapter/session_manager.go`)
- Manages agent sessions and tool permissions
- Filters available tools per agent
- Supports dynamic tool updates
- Authorization checking for tool calls

## Key Features

- **Pure MCP Protocol**: No schema conversion, native JSON Schema support
- **Tool Composition**: Mix tools from multiple servers per agent
- **Per-Agent Filtering**: Each agent sees only selected tools
- **Connection Pooling**: Efficient reuse of server connections
- **Health Checking**: Automatic reconnection on failures
- **Thread Safety**: Concurrent access protection
- **Comprehensive Testing**: Mock servers and integration tests

## Usage Example

```go
// Create proxy server for specific agent
agentID := int64(12345)
selectedTools := []string{"read_file", "query_db", "fetch_url"}
config := adapter.ProxyServerConfig{
    Name:        "Agent Proxy",
    Version:     "1.0.0",
    Description: "Proxy for agent tools",
}

proxy := adapter.NewMCPProxyServer(agentID, selectedTools, "production", config)

// Register tools from multiple servers
fsConfig := adapter.MCPServerConfig{
    ID:      "filesystem",
    Name:    "FileSystem Server", 
    Type:    "stdio",
    Command: "npx",
    Args:    []string{"@modelcontextprotocol/server-filesystem", "/path/to/files"},
}

dbConfig := adapter.MCPServerConfig{
    ID:   "database",
    Name: "Database Server",
    Type: "http", 
    URL:  "http://localhost:8080/mcp",
}

proxy.RegisterToolsFromServer(ctx, fsConfig)
proxy.RegisterToolsFromServer(ctx, dbConfig)

// Use the proxy MCP server with Eino
mcpServer := proxy.GetMCPServer()
// Serve via stdio, HTTP, or SSE...
```

## Testing

Comprehensive test suite includes:

- **Unit Tests**: Individual component testing
- **Integration Tests**: Full pipeline testing
- **Mock Servers**: Filesystem, database, web scraper simulators
- **Example Code**: Working demonstration

Run tests:
```bash
go test ./internal/mcp/testing/... -v
```

Run example:
```bash
go run ./cmd/mcp-proxy-example/main.go
```

## Integration with Station

The MCP proxy adapter integrates with Station's agent execution system:

1. **Tool Discovery**: MCP configs → Tool Registry
2. **Agent Creation**: User selects tools → Agent Session
3. **Agent Execution**: Eino uses proxy server → Tool calls routed to real servers
4. **Results**: Pure MCP protocol throughout

## Next Steps

1. **Station Integration**: Replace current Eino schema conversion
2. **Production Serving**: Add stdio/HTTP/SSE serving capabilities  
3. **Performance Optimization**: Connection pooling and caching
4. **Error Handling**: Comprehensive error recovery
5. **Monitoring**: Logging and metrics collection

## Files Structure

```
internal/mcp/
├── adapter/
│   ├── types.go              # Core data structures
│   ├── proxy_server.go       # Main proxy server
│   ├── client_manager.go     # Connection management
│   ├── tool_registry.go      # Tool mapping and filtering
│   └── session_manager.go    # Agent session management
├── testing/
│   ├── mock_server.go        # Mock MCP servers
│   └── integration_test.go   # Comprehensive tests
└── examples/
    └── simple_proxy.go       # Usage demonstration
```

## Benefits

- **Zero Schema Loss**: Tools work exactly as designed
- **Composable**: Mix and match tools from any servers
- **Scalable**: Connection pooling and efficient routing
- **Testable**: Complete isolation for testing
- **Maintainable**: Clean separation of concerns
- **Standards Compliant**: Pure MCP protocol throughout

This architecture completely eliminates the schema conversion problems while providing a clean, scalable solution for MCP tool integration in Station.