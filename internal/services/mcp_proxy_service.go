package services

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"station/internal/db/repositories"
	"station/internal/mcp/adapter"
	"station/pkg/models"
)

// MCPProxyService manages MCP proxy servers for agent execution
type MCPProxyService struct {
	repos                *repositories.Repositories
	toolDiscoveryService *ToolDiscoveryService
	mcpConfigService     *MCPConfigService
	
	// Proxy server cache - one proxy per agent execution
	mu           sync.RWMutex
	proxyServers map[int64]*adapter.MCPProxyServer // agentID -> proxy server
}

// NewMCPProxyService creates a new MCP proxy service
func NewMCPProxyService(
	repos *repositories.Repositories,
	toolDiscoveryService *ToolDiscoveryService,
	mcpConfigService *MCPConfigService,
) *MCPProxyService {
	return &MCPProxyService{
		repos:                repos,
		toolDiscoveryService: toolDiscoveryService,
		mcpConfigService:     mcpConfigService,
		proxyServers:         make(map[int64]*adapter.MCPProxyServer),
	}
}

// CreateProxyServerForAgent creates a proxy server for a specific agent execution
func (s *MCPProxyService) CreateProxyServerForAgent(ctx context.Context, agentID int64) (*adapter.MCPProxyServer, error) {
	// Get agent configuration
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	// Get assigned tool names for this agent
	assignedToolNames, err := s.getAssignedToolNames(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned tools for agent %d: %w", agentID, err)
	}

	if len(assignedToolNames) == 0 {
		log.Printf("Agent %d has no assigned tools", agentID)
	}

	// Get environment name for session
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %d: %w", agent.EnvironmentID, err)
	}

	// Create proxy server configuration
	proxyConfig := adapter.ProxyServerConfig{
		Name:        fmt.Sprintf("Station Agent %d Proxy", agentID),
		Version:     "1.0.0",
		Description: fmt.Sprintf("MCP proxy server for agent %s", agent.Name),
	}

	// Create the proxy server
	proxy := adapter.NewMCPProxyServer(agentID, assignedToolNames, environment.Name, proxyConfig)

	// Get all MCP configs for this environment to register tools
	mcpConfigs, err := s.repos.MCPConfigs.ListByEnvironment(agent.EnvironmentID)
	if err != nil {
		proxy.Close()
		return nil, fmt.Errorf("failed to get MCP configs for environment %d: %w", agent.EnvironmentID, err)
	}

	// Register tools from each MCP config
	for _, config := range mcpConfigs {
		serverConfig, err := s.convertMCPConfigToAdapterConfig(config)
		if err != nil {
			log.Printf("Failed to convert MCP config %s: %v", config.ConfigName, err)
			continue
		}

		if err := proxy.RegisterToolsFromServer(ctx, serverConfig); err != nil {
			log.Printf("Failed to register tools from MCP config %s: %v", config.ConfigName, err)
			// Continue with other configs - don't fail the entire proxy creation
			continue
		}
	}

	// Cache the proxy server
	s.mu.Lock()
	s.proxyServers[agentID] = proxy
	s.mu.Unlock()

	log.Printf("Created MCP proxy server for agent %d with %d available tools", 
		agentID, len(assignedToolNames))

	return proxy, nil
}

// GetProxyServerForAgent returns the cached proxy server for an agent
func (s *MCPProxyService) GetProxyServerForAgent(agentID int64) (*adapter.MCPProxyServer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	proxy, exists := s.proxyServers[agentID]
	return proxy, exists
}

// CreateEinoMCPClientForAgent creates an Eino MCP client connected to the agent's proxy server
func (s *MCPProxyService) CreateEinoMCPClientForAgent(ctx context.Context, agentID int64) (*client.Client, error) {
	// Get or create proxy server for agent
	proxy, exists := s.GetProxyServerForAgent(agentID)
	if !exists {
		var err error
		proxy, err = s.CreateProxyServerForAgent(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy server for agent %d: %w", agentID, err)
		}
	}

	// Create in-process client connected to the proxy server
	mcpClient, err := client.NewInProcessClient(proxy.GetMCPServer())
	if err != nil {
		return nil, fmt.Errorf("failed to create in-process MCP client for agent %d: %w", agentID, err)
	}

	// Start the client
	if err := mcpClient.Start(ctx); err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("failed to start MCP client for agent %d: %w", agentID, err)
	}

	// Initialize the client
	initRequest := buildMCPInitRequest()
	_, err = mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		mcpClient.Close()
		return nil, fmt.Errorf("failed to initialize MCP client for agent %d: %w", agentID, err)
	}

	log.Printf("Created Eino MCP client for agent %d", agentID)
	return mcpClient, nil
}

// CleanupProxyForAgent removes and closes the proxy server for an agent
func (s *MCPProxyService) CleanupProxyForAgent(agentID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if proxy, exists := s.proxyServers[agentID]; exists {
		proxy.Close()
		delete(s.proxyServers, agentID)
		log.Printf("Cleaned up MCP proxy server for agent %d", agentID)
	}
}

// CleanupAllProxies closes all proxy servers
func (s *MCPProxyService) CleanupAllProxies() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for agentID, proxy := range s.proxyServers {
		proxy.Close()
		log.Printf("Cleaned up MCP proxy server for agent %d", agentID)
	}
	s.proxyServers = make(map[int64]*adapter.MCPProxyServer)
}

// GetProxyStats returns statistics for all active proxy servers
func (s *MCPProxyService) GetProxyStats() map[int64]map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[int64]map[string]interface{})
	for agentID, proxy := range s.proxyServers {
		stats[agentID] = proxy.GetProxyStats()
	}
	return stats
}

// convertMCPConfigToAdapterConfig converts a database MCP config to adapter config
func (s *MCPProxyService) convertMCPConfigToAdapterConfig(config *models.MCPConfig) (adapter.MCPServerConfig, error) {
	// Decrypt the config JSON using the encryption key ID
	configData, err := s.mcpConfigService.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
	if err != nil {
		return adapter.MCPServerConfig{}, fmt.Errorf("failed to decrypt MCP config: %w", err)
	}

	// Find the first server configuration (for now, we assume one server per config)
	// In the future, this could be enhanced to handle multiple servers per config
	var serverConfig models.MCPServerConfig
	var serverName string
	
	if len(configData.Servers) == 0 {
		return adapter.MCPServerConfig{}, fmt.Errorf("no servers found in MCP config %s", config.ConfigName)
	}
	
	// Take the first server configuration
	for name, serverCfg := range configData.Servers {
		serverName = name
		serverConfig = serverCfg
		break
	}

	// Create adapter config from the server configuration
	adapterConfig := adapter.MCPServerConfig{
		ID:          fmt.Sprintf("config_%d_%s", config.ID, serverName),
		Name:        fmt.Sprintf("%s_%s", config.ConfigName, serverName),
		Type:        "stdio", // Default to stdio for now
		Command:     serverConfig.Command,
		Args:        serverConfig.Args,
		Environment: serverConfig.Env,
		Timeout:     30, // Default timeout
	}

	log.Printf("Converted MCP config %s server %s to adapter config (ID: %s)", 
		config.ConfigName, serverName, adapterConfig.ID)

	return adapterConfig, nil
}

// getAssignedToolNames gets the tool names assigned to an agent
func (s *MCPProxyService) getAssignedToolNames(agentID int64) ([]string, error) {
	// Get agent tools with details (includes tool names)
	agentToolsWithDetails, err := s.repos.AgentTools.List(agentID)
	if err != nil {
		return nil, err
	}

	toolNames := make([]string, len(agentToolsWithDetails))
	for i, agentTool := range agentToolsWithDetails {
		toolNames[i] = agentTool.ToolName
	}

	return toolNames, nil
}

// buildMCPInitRequest creates a standard MCP initialization request
func buildMCPInitRequest() mcp.InitializeRequest {
	var initRequest mcp.InitializeRequest
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station Agent MCP Client",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}
	return initRequest
}