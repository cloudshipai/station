package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// EnvironmentToolCache caches tools and clients for an environment
type EnvironmentToolCache struct {
	Tools       []ai.Tool
	Clients     []*mcp.GenkitMCPClient
	CachedAt    time.Time
	ValidFor    time.Duration
}

// IsValid checks if the cached tools are still valid
func (cache *EnvironmentToolCache) IsValid() bool {
	return time.Since(cache.CachedAt) < cache.ValidFor
}

// MCPConnectionManager handles MCP server connections and tool discovery lifecycle
type MCPConnectionManager struct {
	repos            *repositories.Repositories
	genkitApp        *genkit.Genkit
	activeMCPClients []*mcp.GenkitMCPClient
	toolCache        map[int64]*EnvironmentToolCache
	cacheMutex       sync.RWMutex
}

// debugLogToFile writes debug messages to a file for investigation
func debugLogToFile(message string) {
	logFile := "/home/epuerta/projects/hack/station/debug-mcp-connection.log"
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}

// getMapKeys returns the keys of a map for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// NewMCPConnectionManager creates a new MCP connection manager
func NewMCPConnectionManager(repos *repositories.Repositories, genkitApp *genkit.Genkit) *MCPConnectionManager {
	return &MCPConnectionManager{
		repos:     repos,
		genkitApp: genkitApp,
		toolCache: make(map[int64]*EnvironmentToolCache),
	}
}

// GetEnvironmentMCPTools connects to MCP servers from file configs and gets their tools
// This replaces the large method in IntelligentAgentCreator
func (mcm *MCPConnectionManager) GetEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, []*mcp.GenkitMCPClient, error) {
	// TEMPORARY FIX: Completely disable caching to fix stdio MCP connection issues
	// Always create fresh connections for each execution
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: CACHE COMPLETELY DISABLED - creating fresh connections")

	// Get file-based MCP configurations for this environment
	environment, err := mcm.repos.Environments.GetByID(environmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get environment %d: %w", environmentID, err)
	}

	msg := fmt.Sprintf("Getting MCP tools for environment: %s (ID: %d)", environment.Name, environmentID)
	logging.Info("DEBUG MCPCONNMGR: %s", msg)
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + msg)

	// Get file configs for this environment
	fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		msg := fmt.Sprintf("FAILED to get file configs for environment %d: %v", environmentID, err)
		logging.Info("DEBUG MCPCONNMGR: %s", msg)
		debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + msg)
		return nil, nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	msg2 := fmt.Sprintf("Database query returned %d file configs for environment %d", len(fileConfigs), environmentID)
	logging.Info("DEBUG MCPCONNMGR: %s", msg2)
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + msg2)
	
	for i, fc := range fileConfigs {
		fcMsg := fmt.Sprintf("File config %d: name='%s', path='%s', env_id=%d", i, fc.ConfigName, fc.TemplatePath, fc.EnvironmentID)
		logging.Info("DEBUG MCPCONNMGR: %s", fcMsg)
		debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + fcMsg)
	}

	var allTools []ai.Tool
	var allClients []*mcp.GenkitMCPClient

	// Connect to each MCP server from file configs and get their tools
	processMsg := fmt.Sprintf("Processing %d file configs for tool discovery", len(fileConfigs))
	logging.Info("DEBUG MCPCONNMGR: %s", processMsg)
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + processMsg)
	
	for i, fileConfig := range fileConfigs {
		fcProcessMsg := fmt.Sprintf("Processing file config %d: %s", i+1, fileConfig.ConfigName)
		logging.Info("DEBUG MCPCONNMGR: %s", fcProcessMsg)
		debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + fcProcessMsg)
		
		tools, clients := mcm.processFileConfig(ctx, fileConfig)
		
		fcResultMsg := fmt.Sprintf("File config %d returned %d tools and %d clients", i+1, len(tools), len(clients))
		logging.Info("DEBUG MCPCONNMGR: %s", fcResultMsg)
		debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + fcResultMsg)
		
		allTools = append(allTools, tools...)
		allClients = append(allClients, clients...)
	}

	totalToolsMsg := fmt.Sprintf("Total tools discovered from all file config servers: %d", len(allTools))
	totalClientsMsg := fmt.Sprintf("Total clients created: %d", len(allClients))
	logging.Info("DEBUG MCPCONNMGR: %s", totalToolsMsg)
	logging.Info("DEBUG MCPCONNMGR: %s", totalClientsMsg)
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + totalToolsMsg)
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: " + totalClientsMsg)
	
	// TEMPORARY FIX: Completely disable caching to fix stdio MCP connection issues
	debugLogToFile("MCPCONNMGR GetEnvironmentMCPTools: NOT CACHING - fresh connections every time")
	
	return allTools, allClients, nil
}

// processFileConfig handles a single file config and returns tools and clients
func (mcm *MCPConnectionManager) processFileConfig(ctx context.Context, fileConfig *repositories.FileConfigRecord) ([]ai.Tool, []*mcp.GenkitMCPClient) {
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Processing file config: %s", fileConfig.ConfigName)
	
	// Make template path absolute
	configDir := os.ExpandEnv("$HOME/.config/station")
	absolutePath := fmt.Sprintf("%s/%s", configDir, fileConfig.TemplatePath)
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Reading config file: %s", absolutePath)
	
	// Read and process the config file
	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		logging.Info("DEBUG MCPCONNMGR processFileConfig: FAILED to read file config %s from path %s: %v", fileConfig.ConfigName, absolutePath, err)
		return nil, nil
	}
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Successfully read %d bytes from config file", len(rawContent))

	// Process template variables
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Processing template variables for config: %s", fileConfig.ConfigName)
	templateService := NewTemplateVariableService(configDir, mcm.repos)
	result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
	if err != nil {
		logging.Info("DEBUG MCPCONNMGR processFileConfig: FAILED to process template variables for %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Template processing successful, rendered content length: %d", len(result.RenderedContent))

	// Parse the config
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &rawConfig); err != nil {
		logging.Debug("Failed to parse file config %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}

	// Extract servers
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Parsing servers from config: %s", fileConfig.ConfigName)
	logging.Info("DEBUG MCPCONNMGR processFileConfig: Available top-level keys: %v", getMapKeys(rawConfig))
	
	var serversData map[string]interface{}
	if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
		logging.Info("DEBUG MCPCONNMGR processFileConfig: Found 'mcpServers' section with %d servers", len(serversData))
	} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
		logging.Info("DEBUG MCPCONNMGR processFileConfig: Found 'servers' section with %d servers", len(serversData))
	} else {
		logging.Info("DEBUG MCPCONNMGR processFileConfig: NO MCP servers found in config %s - available keys: %v", fileConfig.ConfigName, getMapKeys(rawConfig))
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
	logging.Info("DEBUG MCPCONNMGR connectToMCPServer: About to call GetActiveTools for server: %s", serverName)
	serverTools, err := mcpClient.GetActiveTools(ctx, mcm.genkitApp)
	
	// NOTE: Connection stays alive for tool execution during Generate() calls
	
	if err != nil {
		logging.Info("DEBUG MCPCONNMGR connectToMCPServer: FAILED to get tools from %s: %v", serverName, err)
		return nil, mcpClient // Return client for cleanup even on error
	}

	logging.Info("DEBUG MCPCONNMGR connectToMCPServer: Successfully discovered %d tools from server: %s", len(serverTools), serverName)
	for i, tool := range serverTools {
		logging.Info("DEBUG MCPCONNMGR connectToMCPServer: Tool %d from %s: %s", i+1, serverName, tool.Name())
	}
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