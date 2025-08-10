package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// MCPConnectionManager handles MCP server connections and tool discovery lifecycle
type MCPConnectionManager struct {
	repos            *repositories.Repositories
	genkitApp        *genkit.Genkit
	activeMCPClients []*mcp.GenkitMCPClient
}

// NewMCPConnectionManager creates a new MCP connection manager
func NewMCPConnectionManager(repos *repositories.Repositories, genkitApp *genkit.Genkit) *MCPConnectionManager {
	return &MCPConnectionManager{
		repos:     repos,
		genkitApp: genkitApp,
	}
}

// GetEnvironmentMCPTools connects to MCP servers from file configs and gets their tools
// This replaces the large method in IntelligentAgentCreator
func (mcm *MCPConnectionManager) GetEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, []*mcp.GenkitMCPClient, error) {
	// Get file-based MCP configurations for this environment
	environment, err := mcm.repos.Environments.GetByID(environmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get environment %d: %w", environmentID, err)
	}

	logging.Info("Getting MCP tools for environment: %s (ID: %d)", environment.Name, environmentID)

	// Get file configs for this environment
	fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	logging.Debug("Database query returned %d file configs for environment %d", len(fileConfigs), environmentID)

	var allTools []ai.Tool
	var allClients []*mcp.GenkitMCPClient

	// Connect to each MCP server from file configs and get their tools
	for _, fileConfig := range fileConfigs {
		tools, clients := mcm.processFileConfig(ctx, fileConfig)
		allTools = append(allTools, tools...)
		allClients = append(allClients, clients...)
	}

	logging.Debug("Total tools discovered from all file config servers: %d", len(allTools))
	return allTools, allClients, nil
}

// processFileConfig handles a single file config and returns tools and clients
func (mcm *MCPConnectionManager) processFileConfig(ctx context.Context, fileConfig *repositories.FileConfigRecord) ([]ai.Tool, []*mcp.GenkitMCPClient) {
	logging.Debug("Processing file config: %s", fileConfig.ConfigName)
	
	// Make template path absolute
	configDir := os.ExpandEnv("$HOME/.config/station")
	absolutePath := fmt.Sprintf("%s/%s", configDir, fileConfig.TemplatePath)
	
	// Read and process the config file
	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		logging.Debug("Failed to read file config %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}

	// Process template variables
	templateService := NewTemplateVariableService(configDir, mcm.repos)
	result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
	if err != nil {
		logging.Debug("Failed to process template variables for %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}

	// Parse the config
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &rawConfig); err != nil {
		logging.Debug("Failed to parse file config %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}

	// Extract servers
	var serversData map[string]interface{}
	if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		logging.Debug("No MCP servers found in config %s", fileConfig.ConfigName)
		return nil, nil
	}

	var allTools []ai.Tool
	var allClients []*mcp.GenkitMCPClient

	// Process each server
	for serverName, serverConfigRaw := range serversData {
		tools, client := mcm.connectToMCPServer(ctx, serverName, serverConfigRaw)
		if tools != nil {
			allTools = append(allTools, tools...)
		}
		if client != nil {
			allClients = append(allClients, client)
		}
	}

	return allTools, allClients
}

// connectToMCPServer connects to a single MCP server and gets its tools
// Returns tools and client - client context should NOT be canceled until after execution
func (mcm *MCPConnectionManager) connectToMCPServer(ctx context.Context, serverName string, serverConfigRaw interface{}) ([]ai.Tool, *mcp.GenkitMCPClient) {
	// Convert server config
	serverConfigBytes, err := json.Marshal(serverConfigRaw)
	if err != nil {
		logging.Debug("Failed to marshal server config for %s: %v", serverName, err)
		return nil, nil
	}
	
	var serverConfig models.MCPServerConfig
	if err := json.Unmarshal(serverConfigBytes, &serverConfig); err != nil {
		logging.Debug("Failed to unmarshal server config for %s: %v", serverName, err)
		return nil, nil
	}
	
	// Create MCP client based on config type
	var mcpClient *mcp.GenkitMCPClient
	if serverConfig.URL != "" {
		// HTTP-based MCP server
		mcpClient, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
			Name:    "_",
			Version: "1.0.0",
			StreamableHTTP: &mcp.StreamableHTTPConfig{
				BaseURL: serverConfig.URL,
				Timeout: 30 * time.Second,
			},
		})
	} else if serverConfig.Command != "" {
		// Stdio-based MCP server
		var envSlice []string
		for key, value := range serverConfig.Env {
			envSlice = append(envSlice, key+"="+value)
		}
		
		mcpClient, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
			Name:    "_",
			Version: "1.0.0",
			Stdio: &mcp.StdioConfig{
				Command: serverConfig.Command,
				Args:    serverConfig.Args,
				Env:     envSlice,
			},
		})
	} else {
		logging.Debug("Invalid MCP server config for %s", serverName)
		return nil, nil
	}
	
	if err != nil {
		logging.Debug("Failed to create MCP client for %s: %v", serverName, err)
		return nil, nil
	}

	// Use main context directly - don't create timeout context that could break connection
	// The MCP client internally handles timeouts appropriately
	serverTools, err := mcpClient.GetActiveTools(ctx, mcm.genkitApp)
	
	// NOTE: Connection stays alive for tool execution during Generate() calls
	
	if err != nil {
		logging.Debug("Failed to get tools from %s: %v", serverName, err)
		return nil, mcpClient // Return client for cleanup even on error
	}

	logging.Debug("Successfully discovered %d tools from server: %s", len(serverTools), serverName)
	return serverTools, mcpClient
}

// CleanupConnections closes all provided MCP connections
func (mcm *MCPConnectionManager) CleanupConnections(clients []*mcp.GenkitMCPClient) {
	logging.Debug("Cleaning up %d active MCP connections", len(clients))
	for i, client := range clients {
		if client != nil {
			logging.Debug("Disconnecting MCP client %d", i+1)
			client.Disconnect()
		}
	}
}