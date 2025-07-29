package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// MCPClientService manages dynamic connections to MCP servers for tool execution
type MCPClientService struct {
	repos               *repositories.Repositories
	mcpConfigService    *MCPConfigService
	toolDiscoveryService *ToolDiscoveryService
	
	// Cache of active clients by server ID
	clientCache map[int64]*mcpClientConnection
	cacheMutex  sync.RWMutex
	
	// Shutdown management
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
}

type mcpClientConnection struct {
	client     *client.Client  
	serverInfo *models.MCPServer
	lastUsed   time.Time
	cancel     context.CancelFunc  // Only store cancel function for cleanup
}

// NewMCPClientService creates a new MCP client service
func NewMCPClientService(repos *repositories.Repositories, mcpConfigService *MCPConfigService, toolDiscoveryService *ToolDiscoveryService) *MCPClientService {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	
	service := &MCPClientService{
		repos:               repos,
		mcpConfigService:    mcpConfigService,
		toolDiscoveryService: toolDiscoveryService,
		clientCache:        make(map[int64]*mcpClientConnection),
		shutdownCtx:        shutdownCtx,
		shutdownCancel:     shutdownCancel,
	}
	
	// Start cleanup routine for stale connections with shutdown support
	go service.cleanupRoutine()
	
	return service
}

// CallTool executes a tool on the appropriate MCP server
func (s *MCPClientService) CallTool(environmentID int64, toolName string, arguments map[string]interface{}) (*models.ToolCall, error) {
	// Find the tool in the environment
	tool, server, err := s.findToolInEnvironment(environmentID, toolName)
	if err != nil {
		return nil, fmt.Errorf("failed to find tool %s in environment %d: %w", toolName, environmentID, err)
	}

	// Get or create client connection for the server
	mcpClient, err := s.getOrCreateClient(server)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server %s: %w", server.Name, err)
	}

	// Execute the tool
	result, err := s.executeTool(mcpClient, tool, arguments)
	if err != nil {
		// Return a ToolCall with the error for non-critical tool execution failures
		return &models.ToolCall{
			ToolName:   toolName,
			ServerName: server.Name,
			Arguments:  arguments,
			Error:      err.Error(),
		}, nil
	}

	return &models.ToolCall{
		ToolName:   toolName,
		ServerName: server.Name,
		Arguments:  arguments,
		Result:     result,
	}, nil
}

// CallMultipleTools executes multiple tools, potentially on different servers
func (s *MCPClientService) CallMultipleTools(environmentID int64, toolCalls []models.ToolCall) ([]models.ToolCall, error) {
	results := make([]models.ToolCall, len(toolCalls))
	
	for i, toolCall := range toolCalls {
		result, err := s.CallTool(environmentID, toolCall.ToolName, toolCall.Arguments)
		if err != nil {
			// For critical errors (tool not found, server connection failed), create error result
			results[i] = models.ToolCall{
				ToolName:   toolCall.ToolName,
				ServerName: toolCall.ServerName,
				Arguments:  toolCall.Arguments,
				Error:      err.Error(),
			}
		} else {
			results[i] = *result
		}
	}
	
	return results, nil
}

// GetAvailableTools returns all tools available in an environment with their schemas
func (s *MCPClientService) GetAvailableTools(environmentID int64) ([]*models.MCPTool, error) {
	return s.toolDiscoveryService.GetToolsByEnvironment(environmentID)
}

// RefreshServerConnection closes and recreates a connection to a specific server
func (s *MCPClientService) RefreshServerConnection(serverID int64) error {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	
	if conn, exists := s.clientCache[serverID]; exists {
		conn.cancel() // Close existing connection
		delete(s.clientCache, serverID)
	}
	
	return nil
}

// CloseAllConnections closes all active MCP client connections
func (s *MCPClientService) CloseAllConnections() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()
	
	for serverID, conn := range s.clientCache {
		conn.cancel()
		delete(s.clientCache, serverID)
	}
}

// Shutdown gracefully shuts down the MCP client service and all connections
func (s *MCPClientService) Shutdown() {
	s.shutdownCancel()  // Signal cleanup routine to stop
	s.CloseAllConnections()
}

// Private methods


func (s *MCPClientService) findToolInEnvironment(environmentID int64, toolName string) (*models.MCPTool, *models.MCPServer, error) {
	// Get all tools in the environment
	tools, err := s.toolDiscoveryService.GetToolsByEnvironment(environmentID)
	if err != nil {
		return nil, nil, err
	}

	// Find the specific tool
	for _, tool := range tools {
		if tool.Name == toolName {
			// Get the server info for this tool
			server, err := s.repos.MCPServers.GetByID(tool.MCPServerID)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get server info: %w", err)
			}
			return tool, server, nil
		}
	}

	return nil, nil, fmt.Errorf("tool %s not found in environment %d", toolName, environmentID)
}

func (s *MCPClientService) getOrCreateClient(server *models.MCPServer) (*client.Client, error) {
	// First, check with read lock if connection exists
	s.cacheMutex.RLock()
	if conn, exists := s.clientCache[server.ID]; exists {
		conn.lastUsed = time.Now()
		s.cacheMutex.RUnlock()
		return conn.client, nil
	}
	s.cacheMutex.RUnlock()

	// No existing connection, need to create one
	// Note: No command validation - admins control MCP configs through encrypted SSH interface

	// Create connection context outside of lock - use background context for long-lived connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	
	// Convert env map to slice outside of lock
	var envSlice []string
	for key, value := range server.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
	}

	// Create and initialize client outside of lock to avoid blocking other operations
	stdioTransport := transport.NewStdio(server.Command, envSlice, server.Args...)
	mcpClient := client.NewClient(stdioTransport)
	
	// Start the client
	if err := mcpClient.Start(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station Agent Tool Executor",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		cancel()
		mcpClient.Close()
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	// Now acquire write lock only for caching the connection
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Double-check that another goroutine didn't create the connection while we were initializing
	if existingConn, exists := s.clientCache[server.ID]; exists {
		// Another goroutine created the connection, close ours and use the existing one
		cancel()
		mcpClient.Close()
		existingConn.lastUsed = time.Now()
		return existingConn.client, nil
	}

	// Cache our new connection (don't store the timeout context, only the cancel function)
	conn := &mcpClientConnection{
		client:     mcpClient,
		serverInfo: server,
		lastUsed:   time.Now(),
		cancel:     cancel,
	}
	s.clientCache[server.ID] = conn

	log.Printf("Created new MCP client connection to server %s", server.Name)
	return mcpClient, nil
}

func (s *MCPClientService) executeTool(mcpClient *client.Client, tool *models.MCPTool, arguments map[string]interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Create the tool call request
	callRequest := mcp.CallToolRequest{}
	callRequest.Params.Name = tool.Name
	callRequest.Params.Arguments = arguments

	// Execute the tool
	result, err := mcpClient.CallTool(ctx, callRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", tool.Name, err)
	}

	// Check if the tool call was successful
	if result.IsError {
		// Extract error message from content if available
		if len(result.Content) > 0 {
			if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
				return nil, fmt.Errorf("tool execution failed: %s", textContent.Text)
			}
		}
		return nil, fmt.Errorf("tool execution failed")
	}

	// Parse the result content
	if len(result.Content) == 0 {
		return nil, nil
	}

	// Try to extract text from the first content item
	if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
		// Try to parse as JSON, fall back to string
		var parsedResult interface{}
		if err := json.Unmarshal([]byte(textContent.Text), &parsedResult); err != nil {
			// If JSON parsing fails, return as string
			return textContent.Text, nil
		}
		return parsedResult, nil
	}

	// If it's not text content, return the raw content
	// This could be image, audio, or other content types
	return result.Content[0], nil
}

func (s *MCPClientService) cleanupRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdownCtx.Done():
			// Service is shutting down, exit cleanup routine
			return
		case <-ticker.C:
			s.cleanupStaleConnections()
		}
	}
}

func (s *MCPClientService) cleanupStaleConnections() {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute) // Close connections unused for 10 minutes
	
	for serverID, conn := range s.clientCache {
		if conn.lastUsed.Before(cutoff) {
			log.Printf("Closing stale MCP connection to server %s", conn.serverInfo.Name)
			conn.cancel()
			delete(s.clientCache, serverID)
		}
	}
}

// GetConnectionStats returns statistics about active connections
func (s *MCPClientService) GetConnectionStats() map[string]interface{} {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	stats := map[string]interface{}{
		"active_connections": len(s.clientCache),
		"servers":           make([]string, 0, len(s.clientCache)),
	}

	servers := stats["servers"].([]string)
	for _, conn := range s.clientCache {
		servers = append(servers, conn.serverInfo.Name)
	}
	stats["servers"] = servers

	return stats
}