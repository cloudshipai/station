package intelligent_agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// MCPConnectionManager handles MCP server connections and tool discovery
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

// GetEnvironmentMCPTools connects to the actual MCP servers from file configs and gets their tools
func (mcm *MCPConnectionManager) GetEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, error) {
	// Get file-based MCP configurations for this environment
	environment, err := mcm.repos.Environments.GetByID(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %d: %w", environmentID, err)
	}

	logging.Info("Getting MCP tools for environment: %s (ID: %d)", environment.Name, environmentID)

	// Get file configs for this environment
	logging.Debug("Querying database for file configs with environment ID: %d", environmentID)
	fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	logging.Debug("Database query returned %d file configs for environment %d", len(fileConfigs), environmentID)
	for i, config := range fileConfigs {
		logging.Debug("Config %d: %s (ID: %d, Template: %s)", i+1, config.ConfigName, config.ID, config.TemplatePath)
	}

	var allTools []ai.Tool

	// Connect to each MCP server from file configs and get their tools
	for _, fileConfig := range fileConfigs {
		logging.Debug("Processing file config: %s (ID: %d), template path: %s", fileConfig.ConfigName, fileConfig.ID, fileConfig.TemplatePath)
		
		// Make template path absolute (relative to ~/.config/station/)
		configDir := os.ExpandEnv("$HOME/.config/station")
		absolutePath := fmt.Sprintf("%s/%s", configDir, fileConfig.TemplatePath)
		
		logging.Debug("Reading file config from: %s", absolutePath)
		
		// Read the actual file content from template path
		rawContent, err := os.ReadFile(absolutePath)
		if err != nil {
			logging.Debug("Failed to read file config %s from %s: %v", fileConfig.ConfigName, absolutePath, err)
			continue
		}

		logging.Debug("File config content loaded: %d bytes", len(rawContent))

		// Process template variables using TemplateVariableService
		templateService := services.NewTemplateVariableService(os.ExpandEnv("$HOME/.config/station"), mcm.repos)
		result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
		if err != nil {
			logging.Debug("Failed to process template variables for %s: %v", fileConfig.ConfigName, err)
			continue
		}

		// Use rendered content with variables resolved
		content := result.RenderedContent
		logging.Debug("Template rendered: %d bytes, variables resolved: %v", len(content), result.AllResolved)

		// Parse the file config content to get server configurations
		// The JSON files use "mcpServers" but the struct expects "servers" - handle both
		var rawConfig map[string]interface{}
		if err := json.Unmarshal([]byte(content), &rawConfig); err != nil {
			logging.Debug("Failed to parse file config %s: %v", fileConfig.ConfigName, err)
			continue
		}

		// Extract servers from either "mcpServers" or "servers" field
		var serversData map[string]interface{}
		if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
			serversData = mcpServers
		} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
			serversData = servers
		} else {
			logging.Debug("No 'mcpServers' or 'servers' field found in config %s", fileConfig.ConfigName)
			continue
		}

		logging.Debug("Parsed config data with %d servers", len(serversData))

		// Process each server in the config
		for serverName, serverConfigRaw := range serversData {
			tools, client := mcm.connectToMCPServer(ctx, serverName, serverConfigRaw)
			if client != nil {
				mcm.activeMCPClients = append(mcm.activeMCPClients, client)
			}
			if tools != nil {
				allTools = append(allTools, tools...)
			}
		}
	}

	logging.Debug("Total tools discovered from all file config servers: %d", len(allTools))
	return allTools, nil
}

// connectToMCPServer connects to a single MCP server and gets its tools
func (mcm *MCPConnectionManager) connectToMCPServer(ctx context.Context, serverName string, serverConfigRaw interface{}) ([]ai.Tool, *mcp.GenkitMCPClient) {
	// Convert the server config to proper structure
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
	
	// Determine transport type based on config fields
	var mcpClient *mcp.GenkitMCPClient
	if serverConfig.URL != "" {
		// HTTP-based MCP server
		logging.Debug("Connecting to HTTP MCP server: %s (URL: %s)", serverName, serverConfig.URL)
		mcpClient, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
			Name:    "_", // Minimal discreet prefix
			Version: "1.0.0",
			StreamableHTTP: &mcp.StreamableHTTPConfig{
				BaseURL: serverConfig.URL,
				Timeout: 30 * time.Second, // Add timeout to prevent hanging
			},
		})
	} else if serverConfig.Command != "" {
		// Stdio-based MCP server
		logging.Debug("Connecting to Stdio MCP server: %s (command: %s, args: %v)", serverName, serverConfig.Command, serverConfig.Args)
		
		// Convert env map to slice for Stdio config
		var envSlice []string
		for key, value := range serverConfig.Env {
			envSlice = append(envSlice, key+"="+value)
		}
		
		mcpClient, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
			Name:    "_", // Minimal discreet prefix
			Version: "1.0.0",
			Stdio: &mcp.StdioConfig{
				Command: serverConfig.Command,
				Args:    serverConfig.Args,
				Env:     envSlice,
			},
		})
	} else {
		logging.Debug("Invalid MCP server config for %s: missing both URL and Command fields", serverName)
		return nil, nil
	}
	
	if err != nil {
		logging.Debug("Failed to create MCP client for %s: %v", serverName, err)
		return nil, nil
	}

	// Get tools from this MCP server with timeout and panic recovery
	logging.Debug("Attempting to get tools from MCP server: %s", serverName)
	
	// Create a timeout context for the tool discovery - increased timeout for stdio servers
	timeout := 30 * time.Second
	if serverConfig.Command != "" {
		// Stdio servers (especially uvx-based) need more time for cold start
		timeout = 90 * time.Second
	}
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	
	var serverTools []ai.Tool
	func() {
		// Recover from potential panics in the MCP client
		defer func() {
			if r := recover(); r != nil {
				logging.Debug("Panic recovered while getting tools from %s: %v", serverName, r)
				err = fmt.Errorf("panic in MCP client: %v", r)
			}
		}()
		
		serverTools, err = mcpClient.GetActiveTools(toolCtx, mcm.genkitApp)
	}()
	
	// Cancel immediately after the call returns to prevent resource leaks
	cancel()
	
	if err != nil {
		// Enhanced error logging for timeouts and other failures
		if err == context.DeadlineExceeded {
			envKeys := make([]string, 0, len(serverConfig.Env))
			for k := range serverConfig.Env {
				envKeys = append(envKeys, k)
			}
			logging.Debug("Timeout discovering tools for %s (cmd=%s args=%v envKeys=%v timeout=%v)", 
				serverName, serverConfig.Command, serverConfig.Args, envKeys, timeout)
		} else {
			logging.Debug("Failed to get tools from %s: %v", serverName, err)
		}
		
		if serverConfig.URL != "" {
			logging.Debug("HTTP MCP server details - Name: %s, URL: %s", serverName, serverConfig.URL)
		} else {
			logging.Debug("Stdio MCP server details - Name: %s, Command: %s, Args: %v, Env: %v", 
				serverName, serverConfig.Command, serverConfig.Args, serverConfig.Env)
		}
		
		// Return client even on error so it can be cleaned up later
		return nil, mcpClient
	}

	logging.Debug("Successfully discovered %d tools from server: %s", len(serverTools), serverName)
	for i, tool := range serverTools {
		logging.Debug("  Tool %d: %s", i+1, tool.Name())
	}

	return serverTools, mcpClient
}

// CleanupConnections closes all active MCP connections
func (mcm *MCPConnectionManager) CleanupConnections() {
	logging.Debug("Cleaning up %d active MCP connections", len(mcm.activeMCPClients))
	for i, client := range mcm.activeMCPClients {
		if client != nil {
			logging.Debug("Disconnecting MCP client %d", i+1)
			client.Disconnect()
		}
	}
	mcm.activeMCPClients = nil
}