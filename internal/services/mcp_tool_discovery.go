package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// performToolDiscovery performs MCP tool discovery for a specific config with proper server-to-tool mapping
func (s *DeclarativeSync) performToolDiscovery(ctx context.Context, envID int64, configName string) (int, error) {
	// Create MCP connection manager for tool discovery
	mcpConnManager := NewMCPConnectionManager(s.repos, nil)

	// Initialize Genkit application (needed for MCP connections)
	genkitApp, err := s.initializeGenkitForSync(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to initialize Genkit for tool discovery: %w", err)
	}
	mcpConnManager.genkitApp = genkitApp

	// Get the specific file config we just registered
	fileConfig, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
	if err != nil {
		return 0, fmt.Errorf("failed to get file config %s: %w", configName, err)
	}

	// Discover tools per server (preserving server-to-tool mapping)
	serverToolMappings, clients, err := s.discoverToolsPerServer(ctx, mcpConnManager, fileConfig)

	// Clean up connections immediately
	defer mcpConnManager.CleanupConnections(clients)

	if err != nil {
		// Tool discovery failed - check if this was a newly created config or an existing one
		logging.Info("   ❌ Tool discovery failed for %s: %v", configName, err)

		// Check if any servers from this config existed before this sync attempt
		existingServers, checkErr := s.repos.MCPServers.GetByEnvironmentID(envID)
		hasExistingServers := false
		if checkErr == nil && fileConfig != nil {
			for _, server := range existingServers {
				if server.FileConfigID != nil && *server.FileConfigID == fileConfig.ID {
					hasExistingServers = true
					break
				}
			}
		}

		if hasExistingServers {
			// This config previously worked - DON'T delete it, just log the error
			logging.Info("   ⚠️  Config '%s' previously worked but now fails to connect", configName)
			logging.Info("   ⚠️  Keeping config file - user may need to fix credentials or server settings")
			logging.Info("   ⚠️  Fix the issue and run 'stn sync' again")
		} else {
			// This is a newly created config that never worked - safe to clean up
			logging.Info("   🧹 New config '%s' failed on first sync - cleaning up...", configName)
			if cleanupErr := s.cleanupBrokenMCPServers(ctx, envID, configName); cleanupErr != nil {
				logging.Info("   ⚠️  Warning: Failed to cleanup broken servers: %v", cleanupErr)
			} else {
				logging.Info("   ✅ Successfully removed broken MCP server configuration")
			}
		}

		return 0, fmt.Errorf("failed to discover tools per server for %s: %w", configName, err)
	}

	// Save discovered tools to database with proper server associations
	totalToolsSaved := 0
	for serverName, tools := range serverToolMappings {
		toolsSaved, err := s.saveToolsForServer(ctx, envID, serverName, tools)
		if err != nil {
			logging.Info("     ❌ Failed to save tools for server %s: %v", serverName, err)
			continue
		}
		logging.Info("     ✅ Saved %d tools for server '%s'", toolsSaved, serverName)
		totalToolsSaved += toolsSaved
	}

	logging.Info("   🔍 Tool discovery completed for %s: %d tools saved across %d servers", configName, totalToolsSaved, len(serverToolMappings))
	return totalToolsSaved, nil
}

// cleanupBrokenMCPServers removes broken MCP servers, their tools, the file config record, and the config file from disk
func (s *DeclarativeSync) cleanupBrokenMCPServers(ctx context.Context, envID int64, configName string) error {
	// Get the file config ID for this config name
	fileConfig, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
	if err != nil {
		return fmt.Errorf("failed to get file config: %w", err)
	}

	// Get all MCP servers for this environment
	servers, err := s.repos.MCPServers.GetByEnvironmentID(envID)
	if err != nil {
		return fmt.Errorf("failed to list servers: %w", err)
	}

	// Delete ONLY servers that belong to this specific config (by FileConfigID)
	deletedCount := 0
	for _, server := range servers {
		// Skip servers that don't belong to this config
		if server.FileConfigID == nil || *server.FileConfigID != fileConfig.ID {
			continue
		}

		logging.Info("     🗑️  Deleting MCP server: %s (ID: %d)", server.Name, server.ID)

		// Delete associated tools first
		if err := s.repos.MCPTools.DeleteByServerID(server.ID); err != nil {
			logging.Info("     ⚠️  Warning: Failed to delete tools for server %s: %v", server.Name, err)
		}

		// Delete the server
		if err := s.repos.MCPServers.Delete(server.ID); err != nil {
			logging.Info("     ⚠️  Warning: Failed to delete server %s: %v", server.Name, err)
		}
		deletedCount++
	}

	// Delete the config file from disk
	configFilePath := s.resolveConfigPath(fileConfig.TemplatePath)
	logging.Info("     🗑️  Deleting broken config file: %s", configFilePath)
	if err := os.Remove(configFilePath); err != nil {
		logging.Info("     ⚠️  Warning: Failed to delete config file %s: %v", configFilePath, err)
	} else {
		logging.Info("     ✅ Deleted config file from disk")
	}

	// Delete the file config record from database
	logging.Info("     🗑️  Deleting file config record: %s (ID: %d)", configName, fileConfig.ID)
	if err := s.repos.FileMCPConfigs.Delete(fileConfig.ID); err != nil {
		logging.Info("     ⚠️  Warning: Failed to delete file config record: %v", err)
	} else {
		logging.Info("     ✅ Deleted file config record from database")
	}

	logging.Info("     ✅ Cleanup complete: deleted %d MCP server(s), config file, and database records for '%s'", deletedCount, configName)
	return nil
}

// discoverToolsPerServer connects to each MCP server individually and returns tools mapped by server name
func (s *DeclarativeSync) discoverToolsPerServer(ctx context.Context, mcpConnManager *MCPConnectionManager, fileConfig *repositories.FileConfigRecord) (map[string][]ai.Tool, []*mcp.GenkitMCPClient, error) {
	// Resolve the template path (handles both relative and absolute paths)
	absolutePath := s.resolveConfigPath(fileConfig.TemplatePath)

	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Process template variables using centralized path resolution
	configDir := config.GetConfigRoot()
	templateService := NewTemplateVariableService(configDir, s.repos)
	result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process template variables: %w", err)
	}

	// Parse the config
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &rawConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Extract servers
	var serversData map[string]interface{}
	if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		return nil, nil, fmt.Errorf("no MCP servers found in config")
	}

	serverToolMappings := make(map[string][]ai.Tool)
	var allClients []*mcp.GenkitMCPClient
	var connectionErrors []string

	// Process each server individually to preserve server-to-tool mapping
	logging.Info("     🔍 Discovering tools from %d servers individually...", len(serversData))
	for serverName, serverConfigRaw := range serversData {
		logging.Info("       🖥️  Connecting to server: %s", serverName)

		tools, client := mcpConnManager.connectToMCPServer(ctx, serverName, serverConfigRaw)
		if client != nil {
			allClients = append(allClients, client)
		}

		if tools != nil && len(tools) > 0 {
			serverToolMappings[serverName] = tools
			logging.Info("       ✅ Discovered %d tools from server '%s'", len(tools), serverName)
			// Log first few tool names for debugging
			for i, tool := range tools {
				if i < 3 { // Show first 3 tools
					logging.Info("         🔧 Tool: %s", tool.Name())
				} else if i == 3 && len(tools) > 3 {
					logging.Info("         🔧 ... and %d more tools", len(tools)-3)
					break
				}
			}
		} else {
			// Connection or tool discovery failed for this server
			errorMsg := fmt.Sprintf("Failed to connect or discover tools from server '%s'", serverName)
			logging.Info("       ❌ %s", errorMsg)
			connectionErrors = append(connectionErrors, errorMsg)
		}
	}

	// If any server failed to connect, return error
	if len(connectionErrors) > 0 {
		return serverToolMappings, allClients, fmt.Errorf("tool discovery failed for %d server(s): %v", len(connectionErrors), connectionErrors)
	}

	return serverToolMappings, allClients, nil
}

// saveToolsForServer saves tools for a specific server (idempotent - preserves IDs when possible)
func (s *DeclarativeSync) saveToolsForServer(ctx context.Context, envID int64, serverName string, tools []ai.Tool) (int, error) {
	// Get the server from database
	server, err := s.repos.MCPServers.GetByNameAndEnvironment(serverName, envID)
	if err != nil {
		return 0, fmt.Errorf("server '%s' not found in database: %w", serverName, err)
	}

	// Get existing tools for this server
	existingTools, err := s.repos.MCPTools.GetByServerID(server.ID)
	if err != nil {
		logging.Info("       ⚠️  Warning: Failed to get existing tools for server %s: %v", serverName, err)
		existingTools = []*models.MCPTool{}
	}

	// Create lookup maps
	existingByName := make(map[string]*models.MCPTool)
	for _, tool := range existingTools {
		existingByName[tool.Name] = tool
	}

	discoveredNames := make(map[string]bool)
	for _, tool := range tools {
		discoveredNames[tool.Name()] = true
	}

	// Track what we'll do
	var toDelete []int64
	var toAdd []ai.Tool
	preserved := 0

	// Find tools to delete (exist in DB but not in MCP server)
	for name, existing := range existingByName {
		if !discoveredNames[name] {
			toDelete = append(toDelete, existing.ID)
		} else {
			preserved++
		}
	}

	// Find tools to add (exist in MCP server but not in DB)
	for _, tool := range tools {
		if _, exists := existingByName[tool.Name()]; !exists {
			toAdd = append(toAdd, tool)
		}
	}

	// Only make changes if needed (idempotent)
	if len(toDelete) == 0 && len(toAdd) == 0 {
		logging.Info("       ✅ Tools already in sync for server '%s' (%d tools)", serverName, preserved)
		return preserved, nil
	}

	// Delete tools that no longer exist
	if len(toDelete) > 0 {
		// Since we don't have individual delete, we need to recreate
		// But only if there are actual deletions needed
		err = s.repos.MCPTools.DeleteByServerID(server.ID)
		if err != nil {
			return 0, fmt.Errorf("failed to clear tools for server %s: %w", serverName, err)
		}

		// Recreate tools we want to keep
		for _, tool := range tools {
			toolModel := &models.MCPTool{
				MCPServerID: server.ID,
				Name:        tool.Name(),
				Description: "",
			}
			_, err = s.repos.MCPTools.Create(toolModel)
			if err != nil {
				logging.Info("         ❌ Failed to save tool '%s': %v", tool.Name(), err)
			}
		}

		logging.Info("       🔧 Tool sync for '%s': recreated %d tools (removed %d obsolete)",
			serverName, len(tools), len(toDelete))
		return len(tools), nil
	}

	// Just add new tools (no deletions needed)
	for _, tool := range toAdd {
		toolModel := &models.MCPTool{
			MCPServerID: server.ID,
			Name:        tool.Name(),
			Description: "",
		}
		_, err = s.repos.MCPTools.Create(toolModel)
		if err != nil {
			logging.Info("         ❌ Failed to save tool '%s': %v", tool.Name(), err)
		}
	}

	logging.Info("       🔧 Tool sync for '%s': added %d new tools, preserved %d existing",
		serverName, len(toAdd), preserved)
	return preserved + len(toAdd), nil
}

// initializeGenkitForSync creates a minimal Genkit app for tool discovery during sync
func (s *DeclarativeSync) initializeGenkitForSync(ctx context.Context) (*genkit.Genkit, error) {
	// Create a minimal Genkit provider for sync operations
	genkitProvider := NewGenKitProvider()
	return genkitProvider.GetApp(ctx)
}