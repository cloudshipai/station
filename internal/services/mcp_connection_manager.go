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

// MCPServerPool represents a pool of persistent MCP server connections
type MCPServerPool struct {
	servers       map[string]*mcp.GenkitMCPClient // serverKey -> persistent client
	serverConfigs map[string]interface{}          // serverKey -> config for restart
	tools         map[string][]ai.Tool            // serverKey -> cached tools
	mutex         sync.RWMutex
}

// NewMCPServerPool creates a new server pool
func NewMCPServerPool() *MCPServerPool {
	return &MCPServerPool{
		servers:       make(map[string]*mcp.GenkitMCPClient),
		serverConfigs: make(map[string]interface{}),
		tools:         make(map[string][]ai.Tool),
	}
}

// MCPConnectionManager handles MCP server connections and tool discovery lifecycle
type MCPConnectionManager struct {
	repos            *repositories.Repositories
	genkitApp        *genkit.Genkit
	activeMCPClients []*mcp.GenkitMCPClient
	toolCache        map[int64]*EnvironmentToolCache
	cacheMutex       sync.RWMutex
	serverPool       *MCPServerPool
	poolingEnabled   bool // Feature flag for connection pooling
}

// debugLogToFile writes debug messages to a file for investigation
func debugLogToFile(message string) {
	// Use user's home directory for cross-platform compatibility
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return // Silently fail if can't get home dir
	}
	logFile := fmt.Sprintf("%s/.config/station/debug-mcp-sync.log", homeDir)
	
	// Ensure directory exists
	logDir := fmt.Sprintf("%s/.config/station", homeDir)
	os.MkdirAll(logDir, 0755)
	
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return // Silently fail if can't write
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
		repos:          repos,
		genkitApp:      genkitApp,
		toolCache:      make(map[int64]*EnvironmentToolCache),
		serverPool:     NewMCPServerPool(),
		poolingEnabled: false, // Start disabled for surgical rollout
	}
}

// EnableConnectionPooling enables the connection pooling feature
func (mcm *MCPConnectionManager) EnableConnectionPooling() {
	mcm.poolingEnabled = true
	logging.Info("MCP connection pooling enabled")
}

// InitializeServerPool starts all MCP servers for all environments and keeps them alive
func (mcm *MCPConnectionManager) InitializeServerPool(ctx context.Context) error {
	if !mcm.poolingEnabled {
		return nil // Skip if pooling disabled
	}
	
	logging.Info("Initializing MCP server pool...")
	
	// Get all environments
	environments, err := mcm.repos.Environments.ListAll()
	if err != nil {
		return fmt.Errorf("failed to get environments for server pool: %w", err)
	}
	
	var allServers []serverDefinition
	
	// Collect all unique server configurations across environments
	for _, env := range environments {
		fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(env.ID)
		if err != nil {
			logging.Info("Warning: failed to get file configs for environment %d: %v", env.ID, err)
			continue
		}
		
		servers := mcm.extractServerDefinitions(env.ID, fileConfigs)
		allServers = append(allServers, servers...)
	}
	
	// Start unique servers
	uniqueServers := mcm.deduplicateServers(allServers)
	for _, server := range uniqueServers {
		if err := mcm.startPooledServer(ctx, server); err != nil {
			logging.Info("Warning: failed to start pooled server %s: %v", server.key, err)
		}
	}
	
	logging.Info("MCP server pool initialized with %d servers", len(uniqueServers))
	return nil
}

// serverDefinition represents a unique server configuration
type serverDefinition struct {
	key           string      // unique identifier
	name          string      // server name
	config        interface{} // server config
	environmentID int64       // originating environment
}

// extractServerDefinitions extracts server definitions from file configs
func (mcm *MCPConnectionManager) extractServerDefinitions(environmentID int64, fileConfigs []*repositories.FileConfigRecord) []serverDefinition {
	var servers []serverDefinition
	
	for _, fileConfig := range fileConfigs {
		serverConfigs := mcm.parseFileConfig(fileConfig)
		for serverName, serverConfig := range serverConfigs {
			// Create unique key based on server configuration
			serverKey := mcm.generateServerKey(serverName, serverConfig)
			servers = append(servers, serverDefinition{
				key:           serverKey,
				name:          serverName,
				config:        serverConfig,
				environmentID: environmentID,
			})
		}
	}
	
	return servers
}

// generateServerKey creates a unique key for a server configuration
func (mcm *MCPConnectionManager) generateServerKey(serverName string, serverConfig interface{}) string {
	// Simple key generation - could be made more sophisticated
	configBytes, _ := json.Marshal(serverConfig)
	return fmt.Sprintf("%s:%x", serverName, configBytes[:8]) // Use first 8 bytes of config hash
}

// deduplicateServers removes duplicate server configurations
func (mcm *MCPConnectionManager) deduplicateServers(servers []serverDefinition) []serverDefinition {
	seen := make(map[string]bool)
	var unique []serverDefinition
	
	for _, server := range servers {
		if !seen[server.key] {
			seen[server.key] = true
			unique = append(unique, server)
		}
	}
	
	return unique
}

// startPooledServer starts a server and adds it to the pool
func (mcm *MCPConnectionManager) startPooledServer(ctx context.Context, server serverDefinition) error {
	mcm.serverPool.mutex.Lock()
	defer mcm.serverPool.mutex.Unlock()
	
	// Check if already started
	if _, exists := mcm.serverPool.servers[server.key]; exists {
		return nil
	}
	
	logging.Info("Starting pooled MCP server: %s", server.key)
	
	// Create server client (same logic as connectToMCPServer)
	client, tools, err := mcm.createServerClient(ctx, server.name, server.config)
	if err != nil {
		return fmt.Errorf("failed to create server client for %s: %w", server.key, err)
	}
	
	// Store in pool
	mcm.serverPool.servers[server.key] = client
	mcm.serverPool.serverConfigs[server.key] = server.config
	mcm.serverPool.tools[server.key] = tools
	
	logging.Info("✅ Pooled server %s started with %d tools", server.key, len(tools))
	return nil
}

// parseFileConfig extracts server configurations from a file config
func (mcm *MCPConnectionManager) parseFileConfig(fileConfig *repositories.FileConfigRecord) map[string]interface{} {
	// Make template path absolute
	configDir := os.ExpandEnv("$HOME/.config/station")
	absolutePath := fmt.Sprintf("%s/%s", configDir, fileConfig.TemplatePath)
	
	// Read and process the config file
	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		logging.Debug("Failed to read file config %s: %v", fileConfig.ConfigName, err)
		return nil
	}
	
	// Process template variables
	templateService := NewTemplateVariableService(configDir, mcm.repos)
	result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
	if err != nil {
		logging.Debug("Failed to process template variables for %s: %v", fileConfig.ConfigName, err)
		return nil
	}
	
	// Parse the config
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &rawConfig); err != nil {
		logging.Debug("Failed to parse file config %s: %v", fileConfig.ConfigName, err)
		return nil
	}
	
	// Extract servers
	if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
		return mcpServers
	} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
		return servers
	}
	
	return nil
}

// createServerClient creates a new MCP client (extracted from connectToMCPServer)
func (mcm *MCPConnectionManager) createServerClient(ctx context.Context, serverName string, serverConfigRaw interface{}) (*mcp.GenkitMCPClient, []ai.Tool, error) {
	// Convert server config (same logic as connectToMCPServer)
	serverConfigBytes, err := json.Marshal(serverConfigRaw)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal server config: %w", err)
	}
	
	var serverConfig models.MCPServerConfig
	if err := json.Unmarshal(serverConfigBytes, &serverConfig); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal server config: %w", err)
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
		return nil, nil, fmt.Errorf("invalid MCP server config for %s", serverName)
	}
	
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create MCP client: %w", err)
	}
	
	// Get tools with timeout
	toolCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	
	serverTools, err := mcpClient.GetActiveTools(toolCtx, mcm.genkitApp)
	if err != nil {
		logging.Info("❌ CRITICAL MCP ERROR: Failed to get tools from server '%s': %v", serverName, err)
		return mcpClient, nil, err // Return client for cleanup even on error
	}
	
	logging.Info("✅ MCP SUCCESS: Discovered %d tools from server '%s'", len(serverTools), serverName)
	return mcpClient, serverTools, nil
}

// getPooledEnvironmentMCPTools uses the server pool for fast tool access
func (mcm *MCPConnectionManager) getPooledEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, []*mcp.GenkitMCPClient, error) {
	logging.Info("Using pooled MCP connections for environment %d", environmentID)
	
	// Get file configs for this environment
	fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}
	
	var allTools []ai.Tool
	var allClients []*mcp.GenkitMCPClient
	
	// Find matching servers in the pool
	mcm.serverPool.mutex.RLock()
	defer mcm.serverPool.mutex.RUnlock()
	
	for _, fileConfig := range fileConfigs {
		serverConfigs := mcm.parseFileConfig(fileConfig)
		for serverName, serverConfig := range serverConfigs {
			serverKey := mcm.generateServerKey(serverName, serverConfig)
			
			// Check if server exists in pool
			if pooledClient, exists := mcm.serverPool.servers[serverKey]; exists {
				// Use cached tools from pool
				if tools, toolsExist := mcm.serverPool.tools[serverKey]; toolsExist {
					allTools = append(allTools, tools...)
					allClients = append(allClients, pooledClient) // Reuse pooled client
					logging.Info("✅ Using pooled server %s with %d tools", serverKey, len(tools))
				}
			} else {
				// Server not in pool - fallback to creating fresh connection
				logging.Info("⚠️  Server %s not in pool, creating fresh connection", serverKey)
				tools, client := mcm.connectToMCPServer(ctx, serverName, serverConfig)
				if tools != nil {
					allTools = append(allTools, tools...)
				}
				if client != nil {
					allClients = append(allClients, client)
				}
			}
		}
	}
	
	logging.Info("Pooled connection manager returned %d tools and %d clients for environment %d", 
		len(allTools), len(allClients), environmentID)
	
	return allTools, allClients, nil
}

// GetEnvironmentMCPTools connects to MCP servers from file configs and gets their tools
// This replaces the large method in IntelligentAgentCreator
func (mcm *MCPConnectionManager) GetEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, []*mcp.GenkitMCPClient, error) {
	if mcm.poolingEnabled {
		return mcm.getPooledEnvironmentMCPTools(ctx, environmentID)
	}
	
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

	// Add timeout for Mac debugging - MCP GetActiveTools can hang on macOS
	toolCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	
	logging.Info("DEBUG MCPCONNMGR connectToMCPServer: About to call GetActiveTools for server: %s", serverName)
	serverTools, err := mcpClient.GetActiveTools(toolCtx, mcm.genkitApp)
	
	// NOTE: Connection stays alive for tool execution during Generate() calls
	
	if err != nil {
		// Enhanced error logging for tool discovery failures
		logging.Info("❌ CRITICAL MCP ERROR: Failed to get tools from server '%s': %v", serverName, err)
		logging.Info("   🔧 Server command/config may be invalid or server may not be responding")
		logging.Info("   📡 This means NO TOOLS will be available from this server")
		logging.Info("   ⚠️  Agents depending on tools from '%s' will fail during execution", serverName)
		logging.Info("   🔍 Check server configuration and ensure the MCP server starts correctly")
		
		// Log additional debugging info
		debugLogToFile(fmt.Sprintf("CRITICAL: Tool discovery FAILED for server '%s' - Error: %v", serverName, err))
		debugLogToFile(fmt.Sprintf("IMPACT: NO TOOLS from server '%s' will be available in database", serverName))
		debugLogToFile(fmt.Sprintf("ACTION NEEDED: Verify server config and manual test: %s", serverName))
		
		return nil, mcpClient // Return client for cleanup even on error
	}

	logging.Info("✅ MCP SUCCESS: Discovered %d tools from server '%s'", len(serverTools), serverName)
	debugLogToFile(fmt.Sprintf("SUCCESS: Server '%s' provided %d tools", serverName, len(serverTools)))
	
	for i, tool := range serverTools {
		toolName := tool.Name()
		logging.Info("   🔧 Tool %d: %s", i+1, toolName)
		debugLogToFile(fmt.Sprintf("TOOL DISCOVERED: %s from server %s", toolName, serverName))
	}
	
	if len(serverTools) == 0 {
		logging.Info("⚠️  WARNING: Server '%s' connected successfully but provided ZERO tools", serverName)
		logging.Info("   This may indicate a server configuration issue or empty tool catalog")
		debugLogToFile(fmt.Sprintf("WARNING: Server '%s' connected but provided zero tools", serverName))
	}
	
	return serverTools, mcpClient
}

// CleanupConnections closes all provided MCP connections
func (mcm *MCPConnectionManager) CleanupConnections(clients []*mcp.GenkitMCPClient) {
	if mcm.poolingEnabled {
		// For pooled connections, don't disconnect - they stay alive
		logging.Debug("Pooling enabled: keeping %d connections alive", len(clients))
		return
	}
	
	logging.Debug("Cleaning up %d active MCP connections", len(clients))
	for i, client := range clients {
		if client != nil {
			logging.Debug("Disconnecting MCP client %d", i+1)
			client.Disconnect()
		}
	}
}

// ShutdownServerPool gracefully shuts down all pooled servers
func (mcm *MCPConnectionManager) ShutdownServerPool() {
	if !mcm.poolingEnabled {
		return
	}
	
	mcm.serverPool.mutex.Lock()
	defer mcm.serverPool.mutex.Unlock()
	
	logging.Info("Shutting down MCP server pool with %d servers", len(mcm.serverPool.servers))
	
	for serverKey, client := range mcm.serverPool.servers {
		logging.Info("Disconnecting pooled server: %s", serverKey)
		if client != nil {
			client.Disconnect()
		}
	}
	
	// Clear pool
	mcm.serverPool.servers = make(map[string]*mcp.GenkitMCPClient)
	mcm.serverPool.serverConfigs = make(map[string]interface{})
	mcm.serverPool.tools = make(map[string][]ai.Tool)
	
	logging.Info("MCP server pool shutdown complete")
}