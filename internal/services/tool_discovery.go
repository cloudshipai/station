package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"

	"station/internal/db/repositories"
	"station/pkg/models"
)

type ToolDiscoveryService struct {
	repos           *repositories.Repositories
	mcpConfigService *MCPConfigService
}

func NewToolDiscoveryService(repos *repositories.Repositories, mcpConfigService *MCPConfigService) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		repos:           repos,
		mcpConfigService: mcpConfigService,
	}
}

func (s *ToolDiscoveryService) DiscoverTools(environmentID int64) error {
	// Get the latest MCP config for this environment
	config, err := s.repos.MCPConfigs.GetLatest(environmentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no MCP config found for environment %d", environmentID)
		}
		return fmt.Errorf("failed to get MCP config: %v", err)
	}

	// Decrypt the config
	configData, err := s.mcpConfigService.DecryptConfig(config.EncryptedConfig)
	if err != nil {
		return fmt.Errorf("failed to decrypt config: %v", err)
	}

	log.Printf("Starting tool discovery for environment %d with %d servers", environmentID, len(configData.Servers))

	// Clear existing servers and tools for this config version
	if err := s.clearExistingData(config.ID); err != nil {
		log.Printf("Warning: failed to clear existing data: %v", err)
	}

	// Process each server in the config
	for serverName, serverConfig := range configData.Servers {
		log.Printf("Processing server: %s", serverName)
		
		// Store the server in database
		mcpServer := &models.MCPServer{
			MCPConfigID: config.ID,
			Name:        serverName,
			Command:     serverConfig.Command,
			Args:        serverConfig.Args,
			Env:         serverConfig.Env,
		}

		serverID, err := s.repos.MCPServers.Create(mcpServer)
		if err != nil {
			log.Printf("Failed to store server %s: %v", serverName, err)
			continue
		}
		mcpServer.ID = serverID

		// Discover tools from this server
		tools, err := s.discoverToolsFromServer(serverConfig)
		if err != nil {
			log.Printf("Failed to discover tools from server %s: %v", serverName, err)
			continue
		}

		log.Printf("Discovered %d tools from server %s", len(tools), serverName)

		// Store discovered tools
		for _, tool := range tools {
			// Convert the tool schema to JSON
			schemaBytes, err := json.Marshal(tool.InputSchema)
			if err != nil {
				log.Printf("Failed to marshal schema for tool %s: %v", tool.Name, err)
				schemaBytes = []byte(`{"type":"object"}`) // fallback schema
			}
			
			mcpTool := &models.MCPTool{
				MCPServerID: serverID,
				Name:        tool.Name,
				Description: tool.Description,
				Schema:      json.RawMessage(schemaBytes),
			}

			_, toolErr := s.repos.MCPTools.Create(mcpTool)
			if toolErr != nil {
				log.Printf("Failed to store tool %s: %v", tool.Name, toolErr)
			}
		}
	}

	log.Printf("Tool discovery completed for environment %d", environmentID)
	return nil
}

func (s *ToolDiscoveryService) discoverToolsFromServer(serverConfig models.MCPServerConfig) ([]mcp.Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Convert env map to slice of strings
	var envSlice []string
	for key, value := range serverConfig.Env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
	}

	// Create stdio transport for the server
	stdioTransport := transport.NewStdio(serverConfig.Command, envSlice, serverConfig.Args...)

	// Create client
	c := client.NewClient(stdioTransport)

	// Start the client
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start client: %v", err)
	}
	defer c.Close()

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station Tool Discovery",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	serverInfo, err := c.Initialize(ctx, initRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize: %v", err)
	}

	log.Printf("Connected to server: %s (version %s)", 
		serverInfo.ServerInfo.Name, 
		serverInfo.ServerInfo.Version)

	// Check if server supports tools
	if serverInfo.Capabilities.Tools == nil {
		log.Printf("Server does not support tools")
		return []mcp.Tool{}, nil
	}

	// List available tools
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %v", err)
	}

	return toolsResult.Tools, nil
}

func (s *ToolDiscoveryService) clearExistingData(mcpConfigID int64) error {
	// Get all servers for this config
	servers, err := s.repos.MCPServers.GetByConfigID(mcpConfigID)
	if err != nil {
		return err
	}

	// Delete tools for each server
	for _, server := range servers {
		if err := s.repos.MCPTools.DeleteByServerID(server.ID); err != nil {
			log.Printf("Failed to delete tools for server %d: %v", server.ID, err)
		}
	}

	// Delete servers
	return s.repos.MCPServers.DeleteByConfigID(mcpConfigID)
}

// GetToolsByEnvironment returns all tools available in an environment
func (s *ToolDiscoveryService) GetToolsByEnvironment(environmentID int64) ([]*models.MCPTool, error) {
	// Get latest config for environment
	config, err := s.repos.MCPConfigs.GetLatest(environmentID)
	if err != nil {
		return nil, err
	}

	// Get all servers for this config
	servers, err := s.repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		return nil, err
	}

	var allTools []*models.MCPTool
	for _, server := range servers {
		tools, err := s.repos.MCPTools.GetByServerID(server.ID)
		if err != nil {
			log.Printf("Failed to get tools for server %d: %v", server.ID, err)
			continue
		}
		allTools = append(allTools, tools...)
	}

	return allTools, nil
}

// GetToolsByServer returns tools for a specific server
func (s *ToolDiscoveryService) GetToolsByServer(serverID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByServerID(serverID)
}