package adapter

import (
	"context"
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPProxyServer is the main proxy server that aggregates tools from multiple MCP servers
type MCPProxyServer struct {
	server        *server.MCPServer
	clientManager *ClientManager
	toolRegistry  *ToolRegistry
	sessionMgr    *SessionManager
	config        ProxyServerConfig
	agentID       int64 // The agent this proxy serves
}

// NewMCPProxyServer creates a new MCP proxy server for a specific agent
func NewMCPProxyServer(agentID int64, selectedTools []string, environment string, config ProxyServerConfig) *MCPProxyServer {
	// Create the core MCP server
	mcpServer := server.NewMCPServer(
		config.Name,
		config.Version,
		server.WithToolCapabilities(true),
		server.WithRecovery(), // Add panic recovery
	)
	
	// Create managers
	clientManager := NewClientManager()
	toolRegistry := NewToolRegistry()
	sessionMgr := NewSessionManager()
	
	// Create agent session
	sessionMgr.CreateAgentSession(agentID, selectedTools, environment)
	
	// Create proxy server
	proxy := &MCPProxyServer{
		server:        mcpServer,
		clientManager: clientManager,
		toolRegistry:  toolRegistry,
		sessionMgr:    sessionMgr,
		config:        config,
		agentID:       agentID,
	}
	
	// Set up dynamic tool listing
	proxy.setupDynamicToolListing()
	
	return proxy
}

// setupDynamicToolListing configures the server to dynamically list tools
func (p *MCPProxyServer) setupDynamicToolListing() {
	// Override the tool listing to return only agent-specific tools
	// Note: This would require extending mcp-go server to support dynamic tool listing
	// For now, we'll register tools as they become available
}

// RegisterToolsFromServer registers all tools from a source MCP server
func (p *MCPProxyServer) RegisterToolsFromServer(ctx context.Context, serverConfig MCPServerConfig) error {
	// Add server to client manager
	if err := p.clientManager.AddServer(serverConfig); err != nil {
		return fmt.Errorf("failed to add server config: %w", err)
	}
	
	// Connect to the server
	if err := p.clientManager.ConnectToServer(ctx, serverConfig.ID); err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", serverConfig.ID, err)
	}
	
	// Get tools from the server
	tools, err := p.clientManager.ListToolsFromServer(ctx, serverConfig.ID)
	if err != nil {
		return fmt.Errorf("failed to list tools from server %s: %w", serverConfig.ID, err)
	}
	
	// Register tools in registry
	p.toolRegistry.RegisterTools(serverConfig.ID, tools)
	
	// Filter tools for this agent and register them with the MCP server
	filteredTools, err := p.sessionMgr.FilterToolsForAgent(p.agentID, tools)
	if err != nil {
		return fmt.Errorf("failed to filter tools for agent: %w", err)
	}
	
	// Register each filtered tool with the MCP server
	for _, tool := range filteredTools {
		p.server.AddTool(tool, p.createToolHandler(tool.Name))
	}
	
	log.Printf("Registered %d/%d tools from server %s for agent %d", 
		len(filteredTools), len(tools), serverConfig.Name, p.agentID)
	
	return nil
}

// createToolHandler creates a handler function for a specific tool
func (p *MCPProxyServer) createToolHandler(toolName string) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return p.handleToolCall(ctx, toolName, request)
	}
}

// handleToolCall handles tool calls by routing them to the appropriate source server
func (p *MCPProxyServer) handleToolCall(ctx context.Context, toolName string, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if agent is allowed to use this tool
	if !p.sessionMgr.IsToolAllowedForAgent(p.agentID, toolName) {
		return mcp.NewToolResultError(fmt.Sprintf("Agent %d is not authorized to use tool %s", p.agentID, toolName)), nil
	}
	
	// Get tool mapping to find source server
	mapping, err := p.toolRegistry.GetToolMapping(toolName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool %s not found: %v", toolName, err)), nil
	}
	
	// Forward the tool call to the source server
	result, err := p.clientManager.CallTool(ctx, mapping.ServerID, toolName, request.Params.Arguments)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool call failed: %v", err)), nil
	}
	
	log.Printf("Agent %d successfully called tool %s on server %s", p.agentID, toolName, mapping.ServerID)
	
	return result, nil
}

// RegisterMultipleServers registers tools from multiple MCP servers
func (p *MCPProxyServer) RegisterMultipleServers(ctx context.Context, serverConfigs []MCPServerConfig) error {
	var lastError error
	
	for _, config := range serverConfigs {
		if err := p.RegisterToolsFromServer(ctx, config); err != nil {
			log.Printf("Failed to register tools from server %s: %v", config.Name, err)
			lastError = err
		}
	}
	
	return lastError
}

// GetMCPServer returns the underlying MCP server for serving
func (p *MCPProxyServer) GetMCPServer() *server.MCPServer {
	return p.server
}

// GetAvailableToolCount returns the number of tools available to the agent
func (p *MCPProxyServer) GetAvailableToolCount() (int, error) {
	allTools := p.toolRegistry.GetAllTools()
	filteredTools, err := p.sessionMgr.FilterToolsForAgent(p.agentID, allTools)
	if err != nil {
		return 0, err
	}
	
	return len(filteredTools), nil
}

// UpdateAgentTools updates the tools available to the agent
func (p *MCPProxyServer) UpdateAgentTools(ctx context.Context, newSelectedTools []string) error {
	// Update session
	if err := p.sessionMgr.UpdateAgentTools(p.agentID, newSelectedTools); err != nil {
		return fmt.Errorf("failed to update agent tools: %w", err)
	}
	
	// TODO: This would require rebuilding the MCP server with new tools
	// For now, we log the change - in a full implementation we'd need to:
	// 1. Clear existing tool handlers
	// 2. Re-register tools based on new selection
	// 3. Update the server's tool list
	
	log.Printf("Agent %d tool selection updated to %d tools", p.agentID, len(newSelectedTools))
	return nil
}

// EnsureConnections ensures all source servers are connected
func (p *MCPProxyServer) EnsureConnections(ctx context.Context) error {
	return p.clientManager.EnsureConnections(ctx)
}

// Close shuts down the proxy server
func (p *MCPProxyServer) Close() error {
	log.Printf("Shutting down MCP proxy server for agent %d", p.agentID)
	
	// Disconnect from all source servers
	if err := p.clientManager.DisconnectAll(); err != nil {
		log.Printf("Error disconnecting from servers: %v", err)
	}
	
	// Remove agent session
	p.sessionMgr.RemoveAgentSession(p.agentID)
	
	return nil
}

// GetProxyStats returns statistics about the proxy server
func (p *MCPProxyServer) GetProxyStats() map[string]interface{} {
	connectedServers := p.clientManager.GetConnectedServers()
	totalTools := p.toolRegistry.GetToolCount()
	availableTools, _ := p.GetAvailableToolCount()
	
	return map[string]interface{}{
		"agent_id":          p.agentID,
		"connected_servers": len(connectedServers),
		"server_ids":        connectedServers,
		"total_tools":       totalTools,
		"available_tools":   availableTools,
		"sessions":          p.sessionMgr.GetSessionCount(),
	}
}