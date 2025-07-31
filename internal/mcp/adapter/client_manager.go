package adapter

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// ClientManager manages connections to real MCP servers
type ClientManager struct {
	mu           sync.RWMutex
	clients      map[string]*client.Client      // server_id -> client
	configs      map[string]MCPServerConfig     // server_id -> config
	initialized  map[string]bool                // server_id -> initialization status
}

// NewClientManager creates a new client manager
func NewClientManager() *ClientManager {
	return &ClientManager{
		clients:     make(map[string]*client.Client),
		configs:     make(map[string]MCPServerConfig),
		initialized: make(map[string]bool),
	}
}

// AddServer adds a server configuration
func (cm *ClientManager) AddServer(config MCPServerConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if _, exists := cm.configs[config.ID]; exists {
		return fmt.Errorf("server with ID %s already exists", config.ID)
	}
	
	cm.configs[config.ID] = config
	cm.initialized[config.ID] = false
	
	return nil
}

// ConnectToServer establishes connection to a server
func (cm *ClientManager) ConnectToServer(ctx context.Context, serverID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	config, exists := cm.configs[serverID]
	if !exists {
		return fmt.Errorf("server config for %s not found", serverID)
	}
	
	// Check if already connected
	if cm.initialized[serverID] {
		return nil
	}
	
	// Create transport based on type
	var transportLayer transport.Interface
	var err error
	
	switch config.Type {
	case "stdio":
		// Convert environment map to slice format if needed
		var envSlice []string
		for key, value := range config.Environment {
			envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
		}
		transportLayer = transport.NewStdio(config.Command, envSlice, config.Args...)
	case "http":
		transportLayer, err = transport.NewStreamableHTTP(config.URL)
		if err != nil {
			return fmt.Errorf("failed to create HTTP transport for %s: %w", serverID, err)
		}
	case "sse":
		transportLayer, err = transport.NewSSE(config.URL)
		if err != nil {
			return fmt.Errorf("failed to create SSE transport for %s: %w", serverID, err)
		}
	default:
		return fmt.Errorf("unsupported transport type %s for server %s", config.Type, serverID)
	}
	
	// Create client
	mcpClient := client.NewClient(transportLayer)
	
	// Set timeout context
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}
	
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Start client
	if err := mcpClient.Start(ctxWithTimeout); err != nil {
		return fmt.Errorf("failed to start client for %s: %w", serverID, err)
	}
	
	// Initialize client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station MCP Proxy",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}
	
	_, err = mcpClient.Initialize(ctxWithTimeout, initRequest)
	if err != nil {
		mcpClient.Close()
		return fmt.Errorf("failed to initialize client for %s: %w", serverID, err)
	}
	
	// Store client and mark as initialized
	cm.clients[serverID] = mcpClient
	cm.initialized[serverID] = true
	
	log.Printf("Successfully connected to MCP server: %s (%s)", config.Name, serverID)
	return nil
}

// ConnectToAllServers connects to all configured servers
func (cm *ClientManager) ConnectToAllServers(ctx context.Context) error {
	cm.mu.RLock()
	serverIDs := make([]string, 0, len(cm.configs))
	for serverID := range cm.configs {
		serverIDs = append(serverIDs, serverID)
	}
	cm.mu.RUnlock()
	
	var lastError error
	for _, serverID := range serverIDs {
		if err := cm.ConnectToServer(ctx, serverID); err != nil {
			log.Printf("Failed to connect to server %s: %v", serverID, err)
			lastError = err
		}
	}
	
	return lastError
}

// GetClient returns the client for a server
func (cm *ClientManager) GetClient(serverID string) (*client.Client, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	mcpClient, exists := cm.clients[serverID]
	if !exists || !cm.initialized[serverID] {
		return nil, fmt.Errorf("client for server %s not initialized", serverID)
	}
	
	return mcpClient, nil
}

// CallTool routes a tool call to the appropriate server
func (cm *ClientManager) CallTool(ctx context.Context, serverID, toolName string, arguments any) (*mcp.CallToolResult, error) {
	mcpClient, err := cm.GetClient(serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for server %s: %w", serverID, err)
	}
	
	// Create tool call request
	request := mcp.CallToolRequest{}
	request.Params.Name = toolName
	request.Params.Arguments = arguments
	
	// Call the tool
	result, err := mcpClient.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("tool call failed for %s on server %s: %w", toolName, serverID, err)
	}
	
	return result, nil
}

// ListToolsFromServer gets all tools from a specific server
func (cm *ClientManager) ListToolsFromServer(ctx context.Context, serverID string) ([]mcp.Tool, error) {
	mcpClient, err := cm.GetClient(serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get client for server %s: %w", serverID, err)
	}
	
	// List tools
	request := mcp.ListToolsRequest{}
	result, err := mcpClient.ListTools(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from server %s: %w", serverID, err)
	}
	
	return result.Tools, nil
}

// EnsureConnections checks and reconnects to servers if needed
func (cm *ClientManager) EnsureConnections(ctx context.Context) error {
	cm.mu.RLock()
	serverIDs := make([]string, 0, len(cm.configs))
	for serverID := range cm.configs {
		if !cm.initialized[serverID] {
			serverIDs = append(serverIDs, serverID)
		}
	}
	cm.mu.RUnlock()
	
	// Reconnect to disconnected servers
	for _, serverID := range serverIDs {
		if err := cm.ConnectToServer(ctx, serverID); err != nil {
			log.Printf("Failed to reconnect to server %s: %v", serverID, err)
		}
	}
	
	return nil
}

// DisconnectServer disconnects from a specific server
func (cm *ClientManager) DisconnectServer(serverID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if mcpClient, exists := cm.clients[serverID]; exists {
		if err := mcpClient.Close(); err != nil {
			log.Printf("Error closing client for server %s: %v", serverID, err)
		}
		delete(cm.clients, serverID)
	}
	
	cm.initialized[serverID] = false
	
	log.Printf("Disconnected from MCP server: %s", serverID)
	return nil
}

// DisconnectAll disconnects from all servers
func (cm *ClientManager) DisconnectAll() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for serverID, mcpClient := range cm.clients {
		if err := mcpClient.Close(); err != nil {
			log.Printf("Error closing client for server %s: %v", serverID, err)
		}
		cm.initialized[serverID] = false
	}
	
	// Clear all clients
	cm.clients = make(map[string]*client.Client)
	
	log.Printf("Disconnected from all MCP servers")
	return nil
}

// GetConnectedServers returns list of connected server IDs
func (cm *ClientManager) GetConnectedServers() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	var connected []string
	for serverID, initialized := range cm.initialized {
		if initialized {
			connected = append(connected, serverID)
		}
	}
	
	return connected
}

// GetServerConfig returns the configuration for a server
func (cm *ClientManager) GetServerConfig(serverID string) (MCPServerConfig, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	config, exists := cm.configs[serverID]
	if !exists {
		return MCPServerConfig{}, fmt.Errorf("server config for %s not found", serverID)
	}
	
	return config, nil
}