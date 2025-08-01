package mcp

import (
	"context"
	"log"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"

	"github.com/mark3labs/mcp-go/mcp"
)

type ToolDiscoveryService struct {
	db              db.Database
	mcpConfigSvc    *services.MCPConfigService
	repos           *repositories.Repositories
	discoveredTools map[string][]mcp.Tool // configID -> tools
}

func NewToolDiscoveryService(database db.Database, mcpConfigSvc *services.MCPConfigService, repos *repositories.Repositories) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		db:              database,
		mcpConfigSvc:    mcpConfigSvc,
		repos:           repos,
		discoveredTools: make(map[string][]mcp.Tool),
	}
}

func (t *ToolDiscoveryService) DiscoverTools(ctx context.Context, configID int64) error {
	// Get the MCP configuration
	config, err := t.repos.MCPConfigs.GetByID(configID)
	if err != nil {
		return err
	}

	// Decrypt the configuration to get server details
	decryptedConfig, err := t.mcpConfigSvc.GetDecryptedConfig(configID)
	if err != nil {
		return err
	}

	log.Printf("Tool discovery for config %d: %s", configID, config.ConfigName)
	log.Printf("Found %d MCP servers in config", len(decryptedConfig.Servers))

	// TODO: Implement actual tool discovery by connecting to MCP servers
	// For now, this is a placeholder that logs the discovery process
	
	// Store discovered tools
	configIDStr := string(rune(configID))
	t.discoveredTools[configIDStr] = []mcp.Tool{} // Empty for now

	return nil
}

func (t *ToolDiscoveryService) GetDiscoveredTools(configID string) []mcp.Tool {
	return t.discoveredTools[configID]
}