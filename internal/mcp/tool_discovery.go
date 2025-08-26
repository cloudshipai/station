package mcp

import (
	"station/internal/db"
	"station/internal/db/repositories"

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


func (t *ToolDiscoveryService) GetDiscoveredTools(configID string) []mcp.Tool {
	return t.discoveredTools[configID]
}