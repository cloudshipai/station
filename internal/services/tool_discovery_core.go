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
	repos *repositories.Repositories
}

func NewToolDiscoveryService(repos *repositories.Repositories) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		repos: repos,
	}
}

func (s *ToolDiscoveryService) DiscoverTools(environmentID int64) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: environmentID,
		StartedAt:     time.Now(),
	}

	// This method is deprecated - use DiscoverToolsFromFileConfig instead
	result.AddError(NewToolDiscoveryError(
		ErrorTypeInvalidConfig,
		"",
		"Legacy discovery method deprecated - use file-based configs",
		"",
	))
	result.CompletedAt = time.Now()
	result.Success = false
	return result, nil

}


func (s *ToolDiscoveryService) clearExistingData(mcpConfigID int64) error {
	return s.clearExistingDataTx(nil, mcpConfigID)
}

func (s *ToolDiscoveryService) clearExistingDataTx(tx *sql.Tx, mcpConfigID int64) error {
	// Get all servers for this config
	servers, err := s.repos.MCPServers.GetByEnvironmentID(mcpConfigID)
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
	return s.repos.MCPServers.DeleteByEnvironmentIDTx(tx, mcpConfigID)
}

// GetToolsByEnvironment returns all tools available in an environment
// Now uses file-based configs only
func (s *ToolDiscoveryService) GetToolsByEnvironment(environmentID int64) ([]*models.MCPTool, error) {
	// Get all file configs for the environment
	fileConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, err
	}

	var allTools []*models.MCPTool
	for _, fileConfig := range fileConfigs {
		tools, err := s.repos.MCPTools.GetByFileConfigID(fileConfig.ID)
		if err != nil {
			log.Printf("Failed to get tools for file config %d: %v", fileConfig.ID, err)
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

// DiscoverToolsFromFileConfig discovers tools from a file-based config
func (s *ToolDiscoveryService) DiscoverToolsFromFileConfig(environmentID int64, configName string, renderedConfig *models.MCPConfigData) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: environmentID,
		ConfigName:    configName,
		StartedAt:     time.Now(),
	}
	
	result.TotalServers = len(renderedConfig.Servers)
	
	log.Printf("Starting file config tool discovery for environment %d with %d servers", environmentID, len(renderedConfig.Servers))
	
	// Get or create file config record
	fileConfig, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(environmentID, configName)
	if err != nil {
		result.AddError(NewToolDiscoveryError(
			ErrorTypeDatabase,
			"",
			"Failed to get file config record",
			err.Error(),
		))
		result.CompletedAt = time.Now()
		result.Success = false
		return result, nil
	}
	
	// Clear existing tools for this file config
	if err := s.clearExistingFileConfigData(fileConfig.ID); err != nil {
		log.Printf("Warning: failed to clear existing file config data: %v", err)
		result.AddError(NewToolDiscoveryError(
			ErrorTypeDatabase,
			"",
			"Failed to clear existing file config data",
			err.Error(),
		))
	}
	
	// Process each server in the rendered config
	for serverName, serverConfig := range renderedConfig.Servers {
		log.Printf("Processing file config server: %s", serverName)
		
		// Store the server in database
		mcpServer := &models.MCPServer{
			EnvironmentID: environmentID,
			Name:        serverName,
			Command:     serverConfig.Command,
			Args:        serverConfig.Args,
			Env:         serverConfig.Env,
		}
		
		serverID, err := s.repos.MCPServers.Create(mcpServer)
		if err != nil {
			log.Printf("Failed to store file config server %s: %v", serverName, err)
			result.AddError(NewToolDiscoveryError(
				ErrorTypeDatabase,
				serverName,
				"Failed to store server in database",
				err.Error(),
			))
			continue
		}
		
		// Discover tools from this server using the MCP client
		mcpClient := NewMCPClient()
		tools, err := mcpClient.DiscoverToolsFromServer(serverConfig)
		if err != nil {
			log.Printf("Failed to discover tools from file config server %s: %v", serverName, err)
			
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
		
		log.Printf("Discovered %d tools from file config server %s", len(tools), serverName)
		result.TotalTools += len(tools)
		result.SuccessfulServers++
		
		// Store discovered tools with file config reference
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
			
			// Use the extension method to create tool with file config reference
			_, toolErr := s.repos.MCPTools.CreateWithFileConfig(mcpTool, fileConfig.ID)
			if toolErr != nil {
				log.Printf("Failed to store file config tool %s: %v", tool.Name, toolErr)
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
	
	// Update the file config's last loaded timestamp
	if result.Success {
		if err := s.repos.FileMCPConfigs.UpdateLastLoadedAt(fileConfig.ID); err != nil {
			log.Printf("Warning: failed to update last loaded timestamp: %v", err)
		}
	}
	
	log.Printf("File config tool discovery completed for environment %d. Success: %v, Servers: %d/%d, Tools: %d, Errors: %d", 
		environmentID, result.Success, result.SuccessfulServers, result.TotalServers, result.TotalTools, len(result.Errors))
	
	return result, nil
}

// clearExistingFileConfigData clears existing tools for a file config
func (s *ToolDiscoveryService) clearExistingFileConfigData(fileConfigID int64) error {
	return s.repos.MCPTools.DeleteByFileConfigID(fileConfigID)
}

// GetToolsByFileConfig returns all tools for a specific file config
func (s *ToolDiscoveryService) GetToolsByFileConfig(fileConfigID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByFileConfigID(fileConfigID)
}

// GetHybridToolsByEnvironment returns tools from both database and file configs
func (s *ToolDiscoveryService) GetHybridToolsByEnvironment(environmentID int64) ([]*models.MCPToolWithFileConfig, error) {
	return s.repos.MCPTools.GetToolsWithFileConfigInfo(environmentID)
}