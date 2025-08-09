# Stdio Transport Pipe Management Issue

## Issue Summary

**Error**: `transport error: failed to write request: write |1: file already closed`

**Impact**: MCP servers using stdio transport (including Docker containers) fail during tool execution, preventing agents from successfully using MCP tools.

## Root Cause Analysis

The stdio transport layer in Station has a pipe lifecycle management issue where:

1. **Process Spawning**: Station spawns MCP server subprocess with stdin/stdout pipes
2. **Tool Discovery**: Initial communication works (tools are discovered successfully)
3. **Tool Execution**: During actual tool calls, the subprocess closes its pipes prematurely
4. **Write Failure**: Station attempts to write to closed pipe, causing transport error

## Reproduction Steps

1. Configure any MCP server with stdio transport (Docker or direct command)
2. Create agent with tool access
3. Run agent with task that uses MCP tools
4. Observe transport error during tool execution

## Evidence

### Successful Tool Discovery
```
✅ MCP connection test successful - discovered 174 tools
Agent execution using 94 tools (filtered from 174 available in environment)
```

### Failed Tool Execution
```
ERROR Raw MCP server error error="transport error: failed to write request: write |1: file already closed"
ERROR MCP tool call failed tool=list_stargazers error="transport error: failed to write request: write |1: file already closed"
GenKit Generate error: tool "__list_stargazers" failed: error calling tool __list_stargazers: failed to call tool list_stargazers: transport error: failed to write request: write |1: file already closed
```

## Affected Components

- **File**: `internal/services/mcp_connection_manager.go`
- **File**: `internal/services/tool_discovery_client.go`
- **Transport**: `github.com/mark3labs/mcp-go` stdio transport
- **Scope**: All stdio-based MCP servers (Docker, npx, python, etc.)

## Technical Analysis

### Connection Lifecycle
1. **Discovery Phase**: Works correctly ✅
2. **Execution Phase**: Pipes close prematurely ❌

### Timing Issue
- Initial connection establishes successfully
- Tool discovery completes without errors
- Long-running connection to subprocess fails during execution

### Process Management
- Subprocess may be timing out
- Pipe cleanup happening too early
- Signal handling issues between Station and MCP server process

## Proposed Solution

### 1. Connection Pooling & Reuse
Instead of single long-running connection, use connection pooling:

```go
type MCPConnectionPool struct {
    connections map[string]*MCPConnection
    maxConnections int
    connectionTimeout time.Duration
}
```

### 2. Pipe Lifecycle Management
Improve pipe cleanup and error handling:

```go
func (mcm *MCPConnectionManager) executeToolCall(toolName string, args map[string]interface{}) error {
    // Create fresh connection for each tool call
    conn, err := mcm.createConnection()
    if err != nil {
        return err
    }
    defer conn.Close()
    
    // Execute with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    return conn.ExecuteTool(ctx, toolName, args)
}
```

### 3. Retry Logic
Add retry mechanism for transient failures:

```go
func (mcm *MCPConnectionManager) executeWithRetry(toolName string, args map[string]interface{}) error {
    for attempts := 0; attempts < 3; attempts++ {
        err := mcm.executeToolCall(toolName, args)
        if err == nil {
            return nil
        }
        
        if isTransientError(err) {
            time.Sleep(time.Duration(attempts) * time.Second)
            continue
        }
        
        return err
    }
    return errors.New("max retry attempts exceeded")
}
```

## Testing Strategy

### Unit Tests
- Test pipe lifecycle management
- Test connection pooling
- Test timeout handling

### Integration Tests
- Test with various MCP servers (Docker, Python, Node.js)
- Test concurrent tool executions
- Test long-running agent sessions

### End-to-End Tests
- GitHub MCP server integration
- Multi-step agent workflows
- Error recovery scenarios

## Implementation Plan

1. **Phase 1**: Connection pooling and improved lifecycle management
2. **Phase 2**: Retry logic and error handling
3. **Phase 3**: Performance optimization and monitoring
4. **Phase 4**: Comprehensive testing and validation

## Expected Outcomes

- ✅ Reliable MCP tool execution
- ✅ Stable stdio transport connections
- ✅ Better error handling and recovery
- ✅ Improved agent workflow reliability

## Related Files to Modify

- `internal/services/mcp_connection_manager.go`
- `internal/services/tool_discovery_client.go`
- `internal/services/agent_execution_engine.go`
- Add tests in `testing/` directory

## Verification Steps

1. Configure GitHub MCP server with stdio transport
2. Create agent with GitHub tools
3. Execute multi-step workflow using GitHub MCP tools
4. Verify no transport errors occur
5. Test with concurrent agent executions