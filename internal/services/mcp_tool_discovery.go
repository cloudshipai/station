package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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
	if err != nil {
		return 0, fmt.Errorf("failed to discover tools per server for %s: %w", configName, err)
	}
	
	// Clean up connections immediately
	defer mcpConnManager.CleanupConnections(clients)
	
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

// discoverToolsPerServer connects to each MCP server individually and returns tools mapped by server name
func (s *DeclarativeSync) discoverToolsPerServer(ctx context.Context, mcpConnManager *MCPConnectionManager, fileConfig *repositories.FileConfigRecord) (map[string][]ai.Tool, []*mcp.GenkitMCPClient, error) {
	// Read and process the config file (similar to processFileConfig but with individual server processing)
	// TemplatePath is already an absolute path, don't concatenate it again
	absolutePath := fileConfig.TemplatePath
	
	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Process template variables
	configDir := os.ExpandEnv("$HOME/.config/station")
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
			logging.Info("       ⚠️  No tools discovered from server '%s'", serverName)
		}
	}

	return serverToolMappings, allClients, nil
}

// saveToolsForServer saves tools for a specific server, ensuring they're assigned to the correct server ID
func (s *DeclarativeSync) saveToolsForServer(ctx context.Context, envID int64, serverName string, tools []ai.Tool) (int, error) {
	// Get the server from database
	server, err := s.repos.MCPServers.GetByNameAndEnvironment(serverName, envID)
	if err != nil {
		return 0, fmt.Errorf("server '%s' not found in database: %w", serverName, err)
	}

	// Clear existing tools for this server (declarative sync approach)
	err = s.repos.MCPTools.DeleteByServerID(server.ID)
	if err != nil {
		logging.Info("       ⚠️  Warning: Failed to clear existing tools for server %s: %v", serverName, err)
	} else {
		logging.Info("       🧹 Cleared existing tools for server '%s' (ID: %d)", serverName, server.ID)
	}

	// Save each tool to the correct server
	toolsSaved := 0
	for _, tool := range tools {
		toolName := tool.Name()
		
		// Create tool model
		toolModel := &models.MCPTool{
			MCPServerID: server.ID,
			Name:        toolName,
			Description: "", // Genkit AI tools don't expose description directly
		}
		
		// Save tool to database
		_, err = s.repos.MCPTools.Create(toolModel)
		if err != nil {
			logging.Info("         ❌ Failed to save tool '%s' to server '%s': %v", toolName, serverName, err)
			continue
		}
		toolsSaved++
	}
	
	return toolsSaved, nil
}

// initializeGenkitForSync creates a minimal Genkit app for tool discovery during sync
func (s *DeclarativeSync) initializeGenkitForSync(ctx context.Context) (*genkit.Genkit, error) {
	// Create a minimal Genkit provider for sync operations
	genkitProvider := NewGenKitProvider()
	return genkitProvider.GetApp(ctx)
}