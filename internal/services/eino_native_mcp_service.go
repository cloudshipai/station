package services

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	einomcp "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// EinoNativeMCPService provides Eino tools using native MCP integration (no schema conversion!)
type EinoNativeMCPService struct {
	repos                *repositories.Repositories
	toolDiscoveryService *ToolDiscoveryService
	mcpConfigService     *MCPConfigService
}

// NewEinoNativeMCPService creates a new native MCP service for Eino
func NewEinoNativeMCPService(
	repos *repositories.Repositories,
	toolDiscoveryService *ToolDiscoveryService,
	mcpConfigService *MCPConfigService,
) *EinoNativeMCPService {
	return &EinoNativeMCPService{
		repos:                repos,
		toolDiscoveryService: toolDiscoveryService,
		mcpConfigService:     mcpConfigService,
	}
}

// LoadToolsForAgent loads MCP tools for a specific agent using Eino's native MCP support
func (s *EinoNativeMCPService) LoadToolsForAgent(ctx context.Context, agentID int64) ([]tool.BaseTool, error) {
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
		return []tool.BaseTool{}, nil
	}

	// Use all assigned tools - no filtering (the schema should be handled by Eino native MCP)
	filteredToolNames := assignedToolNames

	// Get all MCP configs for this environment
	mcpConfigs, err := s.repos.MCPConfigs.ListByEnvironment(agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP configs for environment %d: %w", agent.EnvironmentID, err)
	}

	// Collect all tools from all MCP configs
	var allTools []tool.BaseTool

	for _, config := range mcpConfigs {
		// Create MCP client for this config
		mcpClient, err := s.createMCPClientForConfig(config)
		if err != nil {
			log.Printf("Failed to create MCP client for config %s: %v", config.ConfigName, err)
			continue
		}

		// Initialize the client
		if err := s.initializeMCPClient(ctx, mcpClient); err != nil {
			log.Printf("Failed to initialize MCP client for config %s: %v", config.ConfigName, err)
			mcpClient.Close()
			continue
		}

		// Get tools using Eino's native MCP integration (NO SCHEMA CONVERSION!)
		tools, err := einomcp.GetTools(ctx, &einomcp.Config{
			Cli:          mcpClient,
			ToolNameList: filteredToolNames, // Only get the filtered tools assigned to this agent
		})
		if err != nil {
			log.Printf("Failed to get MCP tools for config %s: %v", config.ConfigName, err)
			mcpClient.Close()
			continue
		}

		// Debug: Log tool details to understand what's happening
		for _, tool := range tools {
			toolInfo, err := tool.Info(ctx)
			if err != nil {
				log.Printf("ERROR: Failed to get info for tool: %v", err)
				continue
			}
			log.Printf("DEBUG: Tool loaded: %s - %s", toolInfo.Name, toolInfo.Desc)
		}

		log.Printf("Loaded %d tools from MCP config %s for agent %d", 
			len(tools), config.ConfigName, agentID)

		// Add tools to our collection
		allTools = append(allTools, tools...)

		// Note: We keep the client open for the duration of agent execution
		// TODO: Add proper cleanup mechanism
	}

	log.Printf("Loaded total of %d tools for agent %d using native MCP integration", 
		len(allTools), agentID)

	return allTools, nil
}

// createMCPClientForConfig creates an MCP client for a specific config
func (s *EinoNativeMCPService) createMCPClientForConfig(config *models.MCPConfig) (*client.Client, error) {
	// Decrypt the config JSON
	configData, err := s.mcpConfigService.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt MCP config: %w", err)
	}

	// Find the first server configuration (for now, we assume one server per config)
	if len(configData.Servers) == 0 {
		return nil, fmt.Errorf("no servers found in MCP config %s", config.ConfigName)
	}

	// Take the first server configuration
	var serverConfig models.MCPServerConfig
	var serverName string
	for name, serverCfg := range configData.Servers {
		serverName = name
		serverConfig = serverCfg
		break
	}

	// Create MCP client based on server configuration
	// For now, assume stdio (most common)
	if serverConfig.Command == "" {
		return nil, fmt.Errorf("no command specified for MCP server %s", serverName)
	}

	// Convert environment map to slice
	envs := make([]string, 0, len(serverConfig.Env))
	for key, value := range serverConfig.Env {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}

	// Create stdio MCP client
	mcpClient, err := client.NewStdioMCPClient(
		serverConfig.Command,
		envs,
		serverConfig.Args...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client for server %s: %w", serverName, err)
	}

	log.Printf("Created MCP client for server %s (config %s)", serverName, config.ConfigName)
	return mcpClient, nil
}

// initializeMCPClient initializes an MCP client with the MCP protocol
func (s *EinoNativeMCPService) initializeMCPClient(ctx context.Context, mcpClient *client.Client) error {
	// Initialize
	var initRequest mcp.InitializeRequest
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station Agent Native MCP Client",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err := mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return nil
}

// getAssignedToolNames gets the tool names assigned to an agent
func (s *EinoNativeMCPService) getAssignedToolNames(agentID int64) ([]string, error) {
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

// filterProblematicTools removes tools known to have schemas that cause OpenAI API errors
func (s *EinoNativeMCPService) filterProblematicTools(toolNames []string) []string {
	// List of tools with problematic schemas for OpenAI
	problematicTools := map[string]string{
		"list_allowed_directories": "object schema with empty properties",
		// Add more problematic tools here as we discover them
	}

	filtered := make([]string, 0, len(toolNames))
	for _, toolName := range toolNames {
		if reason, isProblematic := problematicTools[toolName]; isProblematic {
			log.Printf("Filtering out problematic tool '%s': %s", toolName, reason)
			continue
		}
		filtered = append(filtered, toolName)
	}

	return filtered
}