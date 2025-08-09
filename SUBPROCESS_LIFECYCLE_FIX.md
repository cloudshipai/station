# Subprocess Lifecycle Management Fix for Stdio Transport

## Root Cause Analysis

The "write |1: file already closed" error occurs because **subprocess-based MCP servers are terminating mid-execution** while the Station agent is still running, causing the stdio pipes to close.

## The Connection Lifecycle (As Designed)

```
1. Agent Execution Starts
2. MCP Connections Established (tools discovered successfully) ✅  
3. Connections stored in activeMCPClients
4. Agent runs for potentially minutes with multiple tool calls
5. Subprocess terminates unexpectedly ❌ (ROOT CAUSE)
6. Station tries to use tools via closed pipes ❌
7. "transport error: failed to write request: write |1: file already closed"
8. Agent execution completes
9. defer cleanup runs
```

## Why Subprocesses Are Terminating

### Docker Containers
- **Resource limits**: Container may hit memory/CPU limits
- **Timeout behavior**: Docker may terminate inactive containers
- **Signal handling**: Container not properly handling keep-alive

### Python/Node.js Processes  
- **Process timeouts**: MCP server may have internal timeouts
- **Garbage collection**: Process cleanup may close stdio prematurely
- **Error handling**: Subprocess may exit on first error

### OS-Level Issues
- **Pipe buffer limits**: Large data transfers may overflow buffers
- **Process monitoring**: OS may kill unresponsive processes

## Proposed Solutions

### 1. Subprocess Health Monitoring

Add health checks to detect when subprocess dies:

```go
// In mcp_connection_manager.go
func (mcm *MCPConnectionManager) monitorConnection(client *mcp.GenkitMCPClient, serverName string) {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        defer ticker.Stop()
        
        for range ticker.C {
            if !mcm.isConnectionHealthy(client) {
                logging.Warn("MCP connection to %s became unhealthy, attempting reconnection", serverName)
                // Implement reconnection logic
            }
        }
    }()
}
```

### 2. Subprocess Keep-Alive

For Docker containers, add keep-alive flags:

```json
{
  "mcpServers": {
    "github": {
      "command": "docker",
      "args": [
        "run", "-i", "--rm",
        "--init",  // Proper signal handling
        "--memory=512m", // Explicit memory limit
        "--cpus=0.5",    // CPU limits
        "-e", "GITHUB_PERSONAL_ACCESS_TOKEN={{.GITHUB_PERSONAL_ACCESS_TOKEN}}",
        "ghcr.io/github/github-mcp-server"
      ]
    }
  }
}
```

### 3. Graceful Reconnection

If subprocess dies, reconnect transparently:

```go
func (mcm *MCPConnectionManager) executeWithReconnection(toolName string, client *mcp.GenkitMCPClient, config *MCPServerConfig) error {
    err := client.ExecuteTool(toolName, args)
    
    if isConnectionError(err) {
        logging.Info("Connection lost, attempting reconnection for %s", config.Name)
        
        // Disconnect old client
        client.Disconnect()
        
        // Create new client
        newClient, err := mcm.recreateConnection(config)
        if err != nil {
            return fmt.Errorf("reconnection failed: %w", err)
        }
        
        // Replace in active clients
        mcm.replaceClient(client, newClient)
        
        // Retry with new connection
        return newClient.ExecuteTool(toolName, args)
    }
    
    return err
}
```

### 4. Robust Container Configuration

For Docker-based MCP servers:

```bash
# Add to MCP server container
docker run -i --rm \
  --init \                          # Proper PID 1 handling
  --memory=512m \                   # Memory limit
  --cpus=0.5 \                      # CPU limit  
  --oom-kill-disable=false \        # Handle OOM gracefully
  --restart=no \                    # Don't auto-restart
  --health-cmd="echo 'healthy'" \   # Health check
  --health-interval=30s \           # Health check frequency
  -e GITHUB_PERSONAL_ACCESS_TOKEN \
  ghcr.io/github/github-mcp-server
```

## Implementation Strategy

### Phase 1: Connection Health Monitoring
- Add subprocess monitoring to detect when processes die
- Implement health checks using simple JSON-RPC pings
- Add logging for connection state changes

### Phase 2: Graceful Reconnection  
- Implement transparent reconnection when subprocess dies
- Update active client references during reconnection
- Add retry logic for transient failures

### Phase 3: Robust Container Management
- Update Docker configurations with proper resource limits
- Add health checks and timeout configurations
- Implement proper signal handling

## Files to Modify

1. **`internal/services/mcp_connection_manager.go`**
   - Add connection health monitoring
   - Implement reconnection logic
   - Add subprocess lifecycle management

2. **GitHub MCP Configuration**
   - Update Docker run parameters
   - Add resource limits and health checks
   - Improve signal handling

3. **Error Handling**
   - Better detection of connection vs. other errors
   - Graceful degradation when connections fail
   - Comprehensive logging for debugging

## Expected Outcome

- ✅ Subprocess remains alive throughout agent execution
- ✅ Transparent reconnection if subprocess dies
- ✅ No more "write |1: file already closed" errors
- ✅ Reliable long-running agent workflows