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

func (s *ToolDiscoveryService) DiscoverTools(environmentID int64) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: environmentID,
		StartedAt:     time.Now(),
	}

	// Get the latest MCP config for this environment
	config, err := s.repos.MCPConfigs.GetLatest(environmentID)
	if err != nil {
		if err == sql.ErrNoRows {
			result.AddError(NewToolDiscoveryError(
				ErrorTypeInvalidConfig,
				"",
				fmt.Sprintf("No MCP config found for environment %d", environmentID),
				"",
			))
			result.CompletedAt = time.Now()
			result.Success = false
			return result, nil
		}
		result.AddError(NewToolDiscoveryError(
			ErrorTypeDatabase,
			"",
			"Failed to get MCP config",
			err.Error(),
		))
		result.CompletedAt = time.Now()
		result.Success = false
		return result, nil
	}

	result.ConfigID = config.ID

	// Decrypt the config (handle both encrypted and unencrypted configs)
	var configData *models.MCPConfigData
	
	if config.EncryptionKeyID == "" {
		// Config is not encrypted, parse directly
		configData = &models.MCPConfigData{}
		if err := json.Unmarshal([]byte(config.ConfigJSON), configData); err != nil {
			result.AddError(NewToolDiscoveryError(
				ErrorTypeInvalidConfig,
				"",
				"Failed to parse unencrypted config",
				err.Error(),
			))
			result.CompletedAt = time.Now()
			result.Success = false
			return result, nil
		}
	} else {
		// Config is encrypted, decrypt first
		var err error
		configData, err = s.mcpConfigService.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
		if err != nil {
			result.AddError(NewToolDiscoveryError(
				ErrorTypeDecryption,
				"",
				"Failed to decrypt config",
				err.Error(),
			))
			result.CompletedAt = time.Now()
			result.Success = false
			return result, nil
		}
	}

	result.ConfigName = configData.Name
	result.TotalServers = len(configData.Servers)

	log.Printf("Starting tool discovery for environment %d with %d servers", environmentID, len(configData.Servers))

	// Clear existing servers and tools for this config version
	if err := s.clearExistingData(config.ID); err != nil {
		log.Printf("Warning: failed to clear existing data: %v", err)
		result.AddError(NewToolDiscoveryError(
			ErrorTypeDatabase,
			"",
			"Failed to clear existing data",
			err.Error(),
		))
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
			result.AddError(NewToolDiscoveryError(
				ErrorTypeDatabase,
				serverName,
				"Failed to store server in database",
				err.Error(),
			))
			continue
		}
		mcpServer.ID = serverID

		// Discover tools from this server
		tools, err := s.discoverToolsFromServer(serverConfig)
		if err != nil {
			log.Printf("Failed to discover tools from server %s: %v", serverName, err)
			
			// Determine error type based on error message
			errorType := ErrorTypeConnection
			if err.Error() == "context deadline exceeded" {
				errorType = ErrorTypeTimeout
			} else if err.Error() == "failed to start client" {
				errorType = ErrorTypeServerStart
			}
			
			result.AddError(NewToolDiscoveryError(
				errorType,
				serverName,
				"Failed to discover tools from server",
				err.Error(),
			))
			continue
		}

		log.Printf("Discovered %d tools from server %s", len(tools), serverName)
		result.TotalTools += len(tools)
		result.SuccessfulServers++

		// Store discovered tools
		for _, tool := range tools {
			// Convert the tool schema to JSON
			schemaBytes, err := json.Marshal(tool.InputSchema)
			if err != nil {
				log.Printf("Failed to marshal schema for tool %s: %v", tool.Name, err)
				result.AddError(NewToolDiscoveryError(
					ErrorTypeToolParsing,
					serverName,
					fmt.Sprintf("Failed to marshal schema for tool %s", tool.Name),
					err.Error(),
				))
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
				result.AddError(NewToolDiscoveryError(
					ErrorTypeDatabase,
					serverName,
					fmt.Sprintf("Failed to store tool %s", tool.Name),
					toolErr.Error(),
				))
			}
		}
	}

	result.CompletedAt = time.Now()
	result.Success = !result.HasErrors() || result.SuccessfulServers > 0

	log.Printf("Tool discovery completed for environment %d. Success: %v, Servers: %d/%d, Tools: %d, Errors: %d", 
		environmentID, result.Success, result.SuccessfulServers, result.TotalServers, result.TotalTools, len(result.Errors))
	
	return result, nil
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
	return s.clearExistingDataTx(nil, mcpConfigID)
}

func (s *ToolDiscoveryService) clearExistingDataTx(tx *sql.Tx, mcpConfigID int64) error {
	// Get all servers for this config
	servers, err := s.repos.MCPServers.GetByConfigID(mcpConfigID)
	if err != nil {
		return err
	}

	// Delete tools for each server
	for _, server := range servers {
		if err := s.repos.MCPTools.DeleteByServerIDTx(tx, server.ID); err != nil {
			log.Printf("Failed to delete tools for server %d: %v", server.ID, err)
		}
	}

	// Delete servers
	return s.repos.MCPServers.DeleteByConfigIDTx(tx, mcpConfigID)
}

// ReplaceToolsWithTransaction handles the complete tool replacement workflow
// It replaces all tools for a given config name with tools from the latest version
func (s *ToolDiscoveryService) ReplaceToolsWithTransaction(environmentID int64, configName string) (*ToolDiscoveryResult, error) {
	// Get the latest config for this named config
	latestConfig, err := s.repos.MCPConfigs.GetLatestByName(environmentID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest config for %s: %w", configName, err)
	}

	// Start a database transaction
	tx, err := s.repos.BeginTx()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback() // This will be a no-op if we commit successfully

	// Step 1: Get all existing tools for this config name across all versions
	// We need to remove these from agent associations first
	oldTools, err := s.getToolsByConfigName(environmentID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing tools: %w", err)
	}

	// Step 2: Remove all agent-tool associations for the old tools
	if len(oldTools) > 0 {
		oldToolIDs := make([]int64, len(oldTools))
		for i, tool := range oldTools {
			oldToolIDs[i] = tool.ID
		}
		
		if err := s.repos.AgentTools.RemoveByToolIDsTx(tx, oldToolIDs); err != nil {
			return nil, fmt.Errorf("failed to remove agent-tool associations: %w", err)
		}
		log.Printf("Removed %d agent-tool associations for config %s", len(oldToolIDs), configName)
	}

	// Step 3: Clear existing servers and tools for the latest config
	if err := s.clearExistingDataTx(tx, latestConfig.ID); err != nil {
		return nil, fmt.Errorf("failed to clear existing data: %w", err)
	}

	// Step 4: Discover new tools from the latest config
	result, err := s.discoverToolsForConfig(latestConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tools: %w", err)
	}

	// Step 5: Commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully replaced tools for config %s in environment %d", configName, environmentID)
	return result, nil
}

// getToolsByConfigName gets all tools for a given config name across all versions
func (s *ToolDiscoveryService) getToolsByConfigName(environmentID int64, configName string) ([]*models.MCPTool, error) {
	// Get all configs with this name
	configs, err := s.repos.MCPConfigs.ListByConfigName(environmentID, configName)
	if err != nil {
		return nil, err
	}

	var allTools []*models.MCPTool
	for _, config := range configs {
		servers, err := s.repos.MCPServers.GetByConfigID(config.ID)
		if err != nil {
			log.Printf("Failed to get servers for config %d: %v", config.ID, err)
			continue
		}

		for _, server := range servers {
			tools, err := s.repos.MCPTools.GetByServerID(server.ID)
			if err != nil {
				log.Printf("Failed to get tools for server %d: %v", server.ID, err)
				continue
			}
			allTools = append(allTools, tools...)
		}
	}

	return allTools, nil
}

// discoverToolsForConfig discovers tools for a specific config (used within transactions)
func (s *ToolDiscoveryService) discoverToolsForConfig(config *models.MCPConfig) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: config.EnvironmentID,
		ConfigID:      config.ID,
		ConfigName:    config.ConfigName,
		StartedAt:     time.Now(),
	}

	// Decrypt the config data
	var configData *models.MCPConfigData
	
	if config.EncryptionKeyID == "" {
		// Config is not encrypted, parse directly
		configData = &models.MCPConfigData{}
		if err := json.Unmarshal([]byte(config.ConfigJSON), configData); err != nil {
			result.AddError(NewToolDiscoveryError(
				ErrorTypeInvalidConfig,
				"",
				"Failed to parse unencrypted config",
				err.Error(),
			))
			result.CompletedAt = time.Now()
			result.Success = false
			return result, nil
		}
	} else {
		// Config is encrypted, decrypt first
		var err error
		configData, err = s.mcpConfigService.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
		if err != nil {
			result.AddError(NewToolDiscoveryError(
				ErrorTypeDecryption,
				"",
				"Failed to decrypt config",
				err.Error(),
			))
			result.CompletedAt = time.Now()
			result.Success = false
			return result, nil
		}
	}

	result.TotalServers = len(configData.Servers)

	// Process each server in the config
	for serverName, serverConfig := range configData.Servers {
		log.Printf("Processing server: %s for config %s", serverName, config.ConfigName)
		
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
			result.AddError(NewToolDiscoveryError(
				ErrorTypeDatabase,
				serverName,
				"Failed to store server in database",
				err.Error(),
			))
			continue
		}
		mcpServer.ID = serverID

		// Discover tools from this server
		tools, err := s.discoverToolsFromServer(serverConfig)
		if err != nil {
			// Determine error type based on error message
			errorType := ErrorTypeConnection
			if err.Error() == "context deadline exceeded" {
				errorType = ErrorTypeTimeout
			} else if err.Error() == "failed to start client" {
				errorType = ErrorTypeServerStart
			}
			
			result.AddError(NewToolDiscoveryError(
				errorType,
				serverName,
				"Failed to discover tools from server",
				err.Error(),
			))
			continue
		}

		log.Printf("Discovered %d tools from server %s", len(tools), serverName)
		result.TotalTools += len(tools)
		result.SuccessfulServers++

		// Store discovered tools
		for _, tool := range tools {
			// Convert the tool schema to JSON
			schemaBytes, err := json.Marshal(tool.InputSchema)
			if err != nil {
				result.AddError(NewToolDiscoveryError(
					ErrorTypeToolParsing,
					serverName,
					fmt.Sprintf("Failed to marshal schema for tool %s", tool.Name),
					err.Error(),
				))
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
				result.AddError(NewToolDiscoveryError(
					ErrorTypeDatabase,
					serverName,
					fmt.Sprintf("Failed to store tool %s", tool.Name),
					toolErr.Error(),
				))
			}
		}
	}

	result.CompletedAt = time.Now()
	result.Success = !result.HasErrors() || result.SuccessfulServers > 0

	log.Printf("Tool discovery completed for config %s. Success: %v, Servers: %d/%d, Tools: %d, Errors: %d", 
		config.ConfigName, result.Success, result.SuccessfulServers, result.TotalServers, result.TotalTools, len(result.Errors))
	
	return result, nil
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