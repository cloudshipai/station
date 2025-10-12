package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	serverPool       *MCPServerPool
	poolingEnabled   bool // Feature flag for connection pooling
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
	// Default to pooling enabled for better performance
	poolingEnabled := getEnvBoolOrDefault("STATION_MCP_POOLING", true)
	
	return &MCPConnectionManager{
		repos:          repos,
		genkitApp:      genkitApp,
		toolCache:      make(map[int64]*EnvironmentToolCache),
		serverPool:     NewMCPServerPool(),
		poolingEnabled: poolingEnabled,
	}
}

// getEnvBoolOrDefault gets a boolean environment variable with a default value
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// getEnvIntOrDefault gets an integer environment variable with a default value
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
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

	// Check if already initialized
	mcm.serverPool.mutex.Lock()
	if mcm.serverPool.initialized {
		mcm.serverPool.mutex.Unlock()
		return nil
	}
	mcm.serverPool.initialized = true
	mcm.serverPool.mutex.Unlock()
	
	logging.Info("Initializing MCP server pool...")
	
	// Get all environments
	environments, err := mcm.repos.Environments.List()
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
	
	// Start unique servers in parallel for faster initialization
	uniqueServers := mcm.deduplicateServers(allServers)
	if err := mcm.startPooledServersParallel(ctx, uniqueServers); err != nil {
		return fmt.Errorf("failed to start pooled servers in parallel: %w", err)
	}
	
	logging.Info("MCP server pool initialized with %d servers", len(uniqueServers))
	return nil
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

// parseFileConfig extracts server configurations from a file config
func (mcm *MCPConnectionManager) parseFileConfig(fileConfig *repositories.FileConfigRecord) map[string]interface{} {
	// Resolve the template path (handles relative paths like "environments/default/coding.json")
	absolutePath := config.ResolvePath(fileConfig.TemplatePath)

	// Read and process the config file
	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		logging.Debug("Failed to read file config %s: %v", fileConfig.ConfigName, err)
		return nil
	}

	// Process template variables using centralized path resolution
	configDir := config.GetConfigRoot()
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
	// Create telemetry span for MCP client creation and tool discovery
	tracer := otel.Tracer("station-mcp")
	ctx, span := tracer.Start(ctx, "mcp.client.create_and_discover_tools",
		trace.WithAttributes(
			attribute.String("mcp.server.name", serverName),
		),
	)
	defer span.End()
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
				Timeout: 180 * time.Second, // 3 minutes for long-running tools like OpenCode
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
		logging.Error(" CRITICAL MCP ERROR: Failed to get tools from server '%s': %v", serverName, err)
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("mcp.tool_discovery.success", false),
			attribute.String("mcp.tool_discovery.error", err.Error()),
		)
		return mcpClient, nil, err // Return client for cleanup even on error
	}
	
	span.SetAttributes(
		attribute.Bool("mcp.tool_discovery.success", true),
		attribute.Int("mcp.tools_discovered", len(serverTools)),
	)
	logging.Info(" MCP SUCCESS: Discovered %d tools from server '%s'", len(serverTools), serverName)
	
	// Log actual tool names returned by MCP server
	logging.Info("Tool names returned by MCP server '%s':", serverName)
	for i, tool := range serverTools {
		toolName := tool.Name()
		logging.Info("   MCP Tool %d: '%s'", i+1, toolName)
	}
	return mcpClient, serverTools, nil
}

// GetEnvironmentMCPTools connects to MCP servers from file configs and gets their tools
// This replaces the large method that was in the old wrapper classes
func (mcm *MCPConnectionManager) GetEnvironmentMCPTools(ctx context.Context, environmentID int64) ([]ai.Tool, []*mcp.GenkitMCPClient, error) {
	if mcm.poolingEnabled {
		return mcm.getPooledEnvironmentMCPTools(ctx, environmentID)
	}

	// Using legacy connection model (deprecated)
	logging.Info("Warning:  Using legacy MCP connection model (deprecated). Enable pooling with STATION_MCP_POOLING=true for better performance.")
	
	// PERFORMANCE: Track legacy MCP connection time
	mcpConnStartTime := time.Now()

	// TEMPORARY FIX: Completely disable caching to fix stdio MCP connection issues
	// Always create fresh connections for each execution

	// Get file configs for this environment
	fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	// Connect to each MCP server from file configs and get their tools in parallel
	
	allTools, allClients := mcm.processFileConfigsParallel(ctx, fileConfigs)

	
	// TEMPORARY FIX: Completely disable caching to fix stdio MCP connection issues
	
	mcpConnDuration := time.Since(mcpConnStartTime)
	logging.Info("MCP_CONN_PERF: Legacy connections completed in %v", mcpConnDuration)
	
	return allTools, allClients, nil
}

// processFileConfig handles a single file config and returns tools and clients
func (mcm *MCPConnectionManager) processFileConfig(ctx context.Context, fileConfig *repositories.FileConfigRecord) ([]ai.Tool, []*mcp.GenkitMCPClient) {
	logging.Info("MCPCONNMGR processFileConfig: Processing file config: %s", fileConfig.ConfigName)
	
	// Resolve the template path (handles relative paths like "environments/default/coding.json")
	absolutePath := config.ResolvePath(fileConfig.TemplatePath)
	logging.Info("MCPCONNMGR processFileConfig: Reading config file: %s", absolutePath)
	
	// Read and process the config file
	rawContent, err := os.ReadFile(absolutePath)
	if err != nil {
		logging.Info("MCPCONNMGR processFileConfig: FAILED to read file config %s from path %s: %v", fileConfig.ConfigName, absolutePath, err)
		return nil, nil
	}
	logging.Info("MCPCONNMGR processFileConfig: Successfully read %d bytes from config file", len(rawContent))

	// Process template variables
	logging.Info("MCPCONNMGR processFileConfig: Processing template variables for config: %s", fileConfig.ConfigName)
	templateService := NewTemplateVariableService(config.GetConfigRoot(), mcm.repos)
	result, err := templateService.ProcessTemplateWithVariables(fileConfig.EnvironmentID, fileConfig.ConfigName, string(rawContent), false)
	if err != nil {
		logging.Info("MCPCONNMGR processFileConfig: FAILED to process template variables for %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}
	logging.Info("MCPCONNMGR processFileConfig: Template processing successful, rendered content length: %d", len(result.RenderedContent))

	// Parse the config
	var rawConfig map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &rawConfig); err != nil {
		logging.Debug("Failed to parse file config %s: %v", fileConfig.ConfigName, err)
		return nil, nil
	}

	// Extract servers
	logging.Info("MCPCONNMGR processFileConfig: Parsing servers from config: %s", fileConfig.ConfigName)
	logging.Info("MCPCONNMGR processFileConfig: Available top-level keys: %v", getMapKeys(rawConfig))
	
	var serversData map[string]interface{}
	if mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
		logging.Info("MCPCONNMGR processFileConfig: Found 'mcpServers' section with %d servers", len(serversData))
	} else if servers, ok := rawConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
		logging.Info("MCPCONNMGR processFileConfig: Found 'servers' section with %d servers", len(serversData))
	} else {
		logging.Info("MCPCONNMGR processFileConfig: NO MCP servers found in config %s - available keys: %v", fileConfig.ConfigName, getMapKeys(rawConfig))
		return nil, nil
	}

	// Process each server in parallel for faster connection setup
	return mcm.processServersParallel(ctx, serversData)
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
	
	// Create MCP client with timeout protection
	var mcpClient *mcp.GenkitMCPClient

	// Add timeout for MCP client creation to prevent freezing
	clientCtx, clientCancel := context.WithTimeout(ctx, 10*time.Second)
	defer clientCancel()

	// Channel to receive client creation result
	type clientResult struct {
		client *mcp.GenkitMCPClient
		err    error
	}
	clientChan := make(chan clientResult, 1)

	// Run client creation in goroutine with timeout
	go func() {
		var client *mcp.GenkitMCPClient
		var err error

		if serverConfig.URL != "" {
			// HTTP-based MCP server
			client, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
				Name:    "_",
				Version: "1.0.0",
				StreamableHTTP: &mcp.StreamableHTTPConfig{
					BaseURL: serverConfig.URL,
					Timeout: 180 * time.Second, // 3 minutes for long-running tools like OpenCode
				},
			})
		} else if serverConfig.Command != "" {
			// Stdio-based MCP server
			var envSlice []string
			for key, value := range serverConfig.Env {
				envSlice = append(envSlice, key+"="+value)
			}

			client, err = mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
				Name:    "_",
				Version: "1.0.0",
				Stdio: &mcp.StdioConfig{
					Command: serverConfig.Command,
					Args:    serverConfig.Args,
					Env:     envSlice,
				},
			})
		} else {
			err = fmt.Errorf("invalid MCP server config - no URL or Command specified")
		}

		clientChan <- clientResult{client: client, err: err}
	}()
	
	// Wait for client creation or timeout
	select {
	case result := <-clientChan:
		mcpClient = result.client
		err = result.err
	case <-clientCtx.Done():
		logging.Error("CRITICAL: CRITICAL: MCP client creation for server '%s' TIMED OUT after 10 seconds", serverName)
		logging.Info("   💀 This indicates the MCP server is not responding or misconfigured")
		logging.Info("   🔧 Check server command: %s %v", serverConfig.Command, serverConfig.Args)
		logging.Info("   ⚠️  All tools from this server will be UNAVAILABLE")
		return nil, nil
	}
	
	if err != nil {
		logging.Error(" CRITICAL: Failed to create MCP client for server '%s': %v", serverName, err)
		logging.Info("   🔧 Check server configuration and ensure the MCP server command is valid")
		logging.Info("   📡 Command: %s %v", serverConfig.Command, serverConfig.Args)
		return nil, nil
	}

	// Add timeout for Mac debugging - MCP GetActiveTools can hang on macOS
	toolCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	
	logging.Info("MCPCONNMGR connectToMCPServer: About to call GetActiveTools for server: %s", serverName)
	serverTools, err := mcpClient.GetActiveTools(toolCtx, mcm.genkitApp)
	
	// NOTE: Connection stays alive for tool execution during Generate() calls
	
	if err != nil {
		// Enhanced error logging for tool discovery failures
		logging.Error(" CRITICAL MCP ERROR: Failed to get tools from server '%s': %v", serverName, err)
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

	logging.Info(" MCP SUCCESS: Discovered %d tools from server '%s'", len(serverTools), serverName)
	debugLogToFile(fmt.Sprintf("SUCCESS: Server '%s' provided %d tools", serverName, len(serverTools)))
	
	for i, tool := range serverTools {
		toolName := tool.Name()
		logging.Info("   🔧 Tool %d: %s", i+1, toolName)
		debugLogToFile(fmt.Sprintf("TOOL DISCOVERED: %s from server %s", toolName, serverName))
	}
	
	if len(serverTools) == 0 {
		logging.Info("Warning:  WARNING: Server '%s' connected successfully but provided ZERO tools", serverName)
		logging.Info("   This may indicate a server configuration issue or empty tool catalog")
		debugLogToFile(fmt.Sprintf("WARNING: Server '%s' connected but provided zero tools", serverName))
	}
	
	return serverTools, mcpClient
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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