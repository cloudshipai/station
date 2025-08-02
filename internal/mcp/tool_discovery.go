package mcp

import (
	"context"
	"log"

	"station/internal/db"
	"station/internal/db/repositories"
	// "station/internal/services" // Removed - no longer used

	"github.com/mark3labs/mcp-go/mcp"
)

type ToolDiscoveryService struct {
	db              db.Database
	// mcpConfigSvc removed - using file-based configs only
	repos           *repositories.Repositories
	discoveredTools map[string][]mcp.Tool // configID -> tools
}

func NewToolDiscoveryService(database db.Database, repos *repositories.Repositories) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		db:              database,
		repos:           repos,
		discoveredTools: make(map[string][]mcp.Tool),
	}
}

func (t *ToolDiscoveryService) DiscoverTools(ctx context.Context, configID int64) error {
	// This method is deprecated - tool discovery now uses file-based configs
	// See services.ToolDiscoveryService.DiscoverToolsFromFileConfig
	log.Printf("DiscoverTools method deprecated - use file-based config discovery instead")

	return nil
}

func (t *ToolDiscoveryService) GetDiscoveredTools(configID string) []mcp.Tool {
	return t.discoveredTools[configID]
}