package services

import (
	"database/sql"
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

func (s *ToolDiscoveryService) clearExistingData(mcpConfigID int64) error {
	return s.clearExistingDataTx(nil, mcpConfigID)
}

func (s *ToolDiscoveryService) clearExistingDataTx(tx *sql.Tx, mcpConfigID int64) error {
	// Clear existing MCP servers and tools for this config
	var err error
	if tx != nil {
		_, err = tx.Exec(`
			DELETE FROM mcp_tools 
			WHERE server_id IN (
				SELECT id FROM mcp_servers WHERE file_config_id = ?
			)
		`, mcpConfigID)
		if err == nil {
			_, err = tx.Exec(`DELETE FROM mcp_servers WHERE file_config_id = ?`, mcpConfigID)
		}
	} else {
		// Use the repository methods if no transaction
		// This is a simplified approach - in practice you'd want proper cleanup
		log.Printf("TODO: Implement proper cleanup for config %d", mcpConfigID)
	}
	return err
}

func (s *ToolDiscoveryService) GetToolsByEnvironment(environmentID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByEnvironmentID(environmentID)
}

func (s *ToolDiscoveryService) GetToolsByServer(serverID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByServerID(serverID)
}

// DiscoverToolsFromFileConfigNew - DEPRECATED in favor of DeclarativeSync
func (s *ToolDiscoveryService) DiscoverToolsFromFileConfigNew(environmentID int64, configName string, interactive bool) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: environmentID,
		ConfigName:    configName,
		StartedAt:     time.Now(),
	}

	log.Printf("Tool discovery deprecated - DeclarativeSync handles this automatically")

	// Return error indicating this is deprecated
	result.AddError(NewToolDiscoveryError(
		ErrorTypeTemplateRendering,
		"",
		"Tool discovery service deprecated - use DeclarativeSync instead",
		"FileConfigService was deprecated in favor of DeclarativeSync for unified sync operations",
	))
	result.CompletedAt = time.Now()
	result.Success = false
	return result, nil
}

// DEPRECATED: Use DeclarativeSync instead
func (s *ToolDiscoveryService) DiscoverToolsFromFileConfig(environmentID int64, configName string, renderedConfig *models.MCPConfigData) (*ToolDiscoveryResult, error) {
	result := &ToolDiscoveryResult{
		EnvironmentID: environmentID,
		ConfigName:    configName,
		StartedAt:     time.Now(),
	}

	// Return error indicating this is deprecated
	result.AddError(NewToolDiscoveryError(
		ErrorTypeTemplateRendering,
		"",
		"Tool discovery service deprecated - use DeclarativeSync instead",
		"This method was deprecated in favor of DeclarativeSync for unified sync operations",
	))
	result.CompletedAt = time.Now()
	result.Success = false
	return result, nil
}

func (s *ToolDiscoveryService) clearExistingFileConfigData(fileConfigID int64) error {
	log.Printf("TODO: Implement clearExistingFileConfigData for file config %d", fileConfigID)
	return nil
}

func (s *ToolDiscoveryService) GetToolsByFileConfig(fileConfigID int64) ([]*models.MCPTool, error) {
	log.Printf("TODO: Implement GetToolsByFileConfig for file config %d", fileConfigID)
	return nil, fmt.Errorf("not implemented - use DeclarativeSync instead")
}

func (s *ToolDiscoveryService) GetHybridToolsByEnvironment(environmentID int64) ([]*models.MCPToolWithFileConfig, error) {
	log.Printf("TODO: Implement GetHybridToolsByEnvironment for environment %d", environmentID)
	return nil, fmt.Errorf("not implemented - use DeclarativeSync instead")
}