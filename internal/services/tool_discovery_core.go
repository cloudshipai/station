package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
	configData, err := s.decryptConfig(config)
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

		// Discover tools from this server using the MCP client
		mcpClient := NewMCPClient()
		tools, err := mcpClient.DiscoverToolsFromServer(serverConfig)
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

// decryptConfig handles both encrypted and unencrypted configs
func (s *ToolDiscoveryService) decryptConfig(config *models.MCPConfig) (*models.MCPConfigData, error) {
	var configData *models.MCPConfigData
	
	if config.EncryptionKeyID == "" {
		// Config is not encrypted, parse directly
		configData = &models.MCPConfigData{}
		if err := json.Unmarshal([]byte(config.ConfigJSON), configData); err != nil {
			return nil, fmt.Errorf("failed to parse unencrypted config: %w", err)
		}
	} else {
		// Config is encrypted, decrypt first
		var err error
		configData, err = s.mcpConfigService.DecryptConfigWithKeyID(config.ConfigJSON, config.EncryptionKeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt config: %w", err)
		}
	}
	
	return configData, nil
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