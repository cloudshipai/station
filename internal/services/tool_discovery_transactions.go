package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"station/pkg/models"
)

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
		oldToolNames := make([]string, len(oldTools))
		for i, tool := range oldTools {
			oldToolNames[i] = tool.Name
		}
		
		if err := s.repos.AgentTools.RemoveByToolNames(oldToolNames); err != nil {
			return nil, fmt.Errorf("failed to remove agent-tool associations: %w", err)
		}
		log.Printf("Removed %d agent-tool associations for config %s", len(oldToolNames), configName)
	}

	// Step 3: Clear existing servers and tools for ALL versions of this config name
	allConfigs, err := s.repos.MCPConfigs.ListByConfigName(environmentID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to get all config versions: %w", err)
	}
	
	for _, config := range allConfigs {
		if err := s.clearExistingDataTx(tx, config.ID); err != nil {
			log.Printf("Failed to clear data for config version %d: %v", config.ID, err)
			// Continue with other configs rather than failing completely
		}
	}
	
	// Step 3.5: Remove old config versions, keeping only the latest
	if len(allConfigs) > 1 {
		for _, config := range allConfigs {
			if config.ID != latestConfig.ID {
				if err := s.repos.MCPConfigs.DeleteTx(tx, config.ID); err != nil {
					log.Printf("Failed to delete old config version %d: %v", config.ID, err)
				} else {
					log.Printf("Deleted old config version %d (v%d) for config %s", config.ID, config.Version, configName)
				}
			}
		}
	}

	// Step 4: Discover new tools from the latest config
	result, err := s.discoverToolsForConfigTx(tx, latestConfig)
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
		mcpClient := NewMCPClient()
		tools, err := mcpClient.DiscoverToolsFromServer(serverConfig)
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

// discoverToolsForConfigTx is the transaction-aware version of discoverToolsForConfig
func (s *ToolDiscoveryService) discoverToolsForConfigTx(tx *sql.Tx, config *models.MCPConfig) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: config.EnvironmentID,
		ConfigID:      config.ID,
		ConfigName:    config.ConfigName,
		StartedAt:     time.Now(),
	}

	// Decrypt the config data
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

	result.TotalServers = len(configData.Servers)

	// Process each server in the config
	for serverName, serverConfig := range configData.Servers {
		log.Printf("Processing server: %s for config %s", serverName, config.ConfigName)
		
		// Store the server in database using transaction
		mcpServer := &models.MCPServer{
			MCPConfigID: config.ID,
			Name:        serverName,
			Command:     serverConfig.Command,
			Args:        serverConfig.Args,
			Env:         serverConfig.Env,
		}

		serverID, err := s.repos.MCPServers.CreateTx(tx, mcpServer)
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
		mcpClient := NewMCPClient()
		tools, err := mcpClient.DiscoverToolsFromServer(serverConfig)
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

		result.SuccessfulServers++

		// Store each discovered tool in database using transaction
		for _, tool := range tools {
			// Serialize the schema to JSON
			schemaBytes, err := json.Marshal(tool.InputSchema)
			if err != nil {
				log.Printf("Failed to marshal schema for tool %s: %v", tool.Name, err)
				schemaBytes = []byte("{}")
			}
			
			mcpTool := &models.MCPTool{
				MCPServerID: serverID,
				Name:        tool.Name,
				Description: tool.Description,
				Schema:      json.RawMessage(schemaBytes),
			}

			_, toolErr := s.repos.MCPTools.CreateTx(tx, mcpTool)
			if toolErr != nil {
				result.AddError(NewToolDiscoveryError(
					ErrorTypeDatabase,
					serverName,
					fmt.Sprintf("Failed to store tool %s", tool.Name),
					toolErr.Error(),
				))
			} else {
				result.TotalTools++
			}
		}
	}

	result.CompletedAt = time.Now()
	result.Success = !result.HasErrors() || result.SuccessfulServers > 0

	log.Printf("Tool discovery completed for config %s. Success: %v, Servers: %d/%d, Tools: %d, Errors: %d", 
		config.ConfigName, result.Success, result.SuccessfulServers, result.TotalServers, result.TotalTools, len(result.Errors))
	
	return result, nil
}