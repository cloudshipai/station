package services

import (
	"fmt"
	"log"

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

func (s *ToolDiscoveryService) GetToolsByEnvironment(environmentID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByEnvironmentID(environmentID)
}

func (s *ToolDiscoveryService) GetToolsByServer(serverID int64) ([]*models.MCPTool, error) {
	return s.repos.MCPTools.GetByServerID(serverID)
}

// All deprecated methods that were using direct SQL queries
func (s *ToolDiscoveryService) DiscoverToolsFromFileConfigNew(environmentID int64, configName string, interactive bool) error {
	log.Printf("Tool discovery deprecated - DeclarativeSync handles this automatically")
	return fmt.Errorf("tool discovery service deprecated - use DeclarativeSync instead")
}

func (s *ToolDiscoveryService) DiscoverToolsFromFileConfig(environmentID int64, configName string, renderedConfig *models.MCPConfigData) error {
	log.Printf("Tool discovery deprecated - DeclarativeSync handles this automatically") 
	return fmt.Errorf("tool discovery service deprecated - use DeclarativeSync instead")
}

func (s *ToolDiscoveryService) GetToolsByFileConfig(fileConfigID int64) ([]*models.MCPTool, error) {
	log.Printf("GetToolsByFileConfig deprecated for file config %d", fileConfigID)
	return nil, fmt.Errorf("not implemented - use DeclarativeSync instead")
}

func (s *ToolDiscoveryService) GetHybridToolsByEnvironment(environmentID int64) ([]*models.MCPToolWithFileConfig, error) {
	log.Printf("GetHybridToolsByEnvironment deprecated for environment %d", environmentID)
	return nil, fmt.Errorf("not implemented - use DeclarativeSync instead")
}