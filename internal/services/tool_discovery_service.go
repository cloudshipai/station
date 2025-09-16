package services

import (
	"fmt"
	"log"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// ToolDiscoveryService provides access to discovered MCP tools
// This service focuses only on reading tools that have been discovered by DeclarativeSync
type ToolDiscoveryService struct {
	repos *repositories.Repositories
}

func NewToolDiscoveryService(repos *repositories.Repositories) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		repos: repos,
	}
}

// GetToolsByEnvironment returns tools for a specific environment
func (s *ToolDiscoveryService) GetToolsByEnvironment(environmentID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByEnvironmentID(environmentID)
}

// GetToolsByServer returns tools for a specific server
func (s *ToolDiscoveryService) GetToolsByServer(serverID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByServerID(serverID)
}

// GetHybridToolsByEnvironment returns tools with server info for an environment
func (s *ToolDiscoveryService) GetHybridToolsByEnvironment(environmentID int64) ([]*models.MCPToolWithFileConfig, error) {
	// This method is used by lighthouse service to get tools with server information
	// For now, return an empty list with a log message indicating tools should be discovered by DeclarativeSync
	log.Printf("GetHybridToolsByEnvironment called for environment %d - tools should be discovered via DeclarativeSync", environmentID)
	
	// TODO: Implement proper hybrid tool retrieval by joining MCP tools with their server configs
	// This would require querying both MCPTools and FileMCPConfigs repositories
	return []*models.MCPToolWithFileConfig{}, nil
}

// Deprecated methods that return errors pointing to DeclarativeSync
func (s *ToolDiscoveryService) DiscoverToolsFromFileConfig(environmentID int64, configName string, renderedConfig *models.MCPConfigData) error {
	return fmt.Errorf("tool discovery deprecated - use DeclarativeSync.SyncEnvironment() instead")
}

func (s *ToolDiscoveryService) DiscoverToolsFromFileConfigNew(environmentID int64, configName string, interactive bool) error {
	return fmt.Errorf("tool discovery deprecated - use DeclarativeSync.SyncEnvironment() instead")
}

func (s *ToolDiscoveryService) GetToolsByFileConfig(fileConfigID int64) ([]*models.MCPTool, error) {
	return nil, fmt.Errorf("deprecated - use DeclarativeSync instead")
}