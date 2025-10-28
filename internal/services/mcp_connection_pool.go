package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// MCPServerPool represents a pool of persistent MCP server connections
type MCPServerPool struct {
	servers       map[string]*mcp.GenkitMCPClient // serverKey -> persistent client
	serverConfigs map[string]interface{}          // serverKey -> config for restart
	tools         map[string][]ai.Tool            // serverKey -> cached tools
	mutex         sync.RWMutex
	initialized   bool // prevent multiple initializations
}

// NewMCPServerPool creates a new server pool
func NewMCPServerPool() *MCPServerPool {
	return &MCPServerPool{
		servers:       make(map[string]*mcp.GenkitMCPClient),
		serverConfigs: make(map[string]interface{}),
		tools:         make(map[string][]ai.Tool),
	}
}

// serverDefinition represents a unique server configuration
type serverDefinition struct {
	key           string      // unique identifier
	name          string      // server name
	config        interface{} // server config
	environmentID int64       // originating environment
}

// startPooledServersParallel starts multiple servers in parallel for faster pool initialization
func (mcm *MCPConnectionManager) startPooledServersParallel(ctx context.Context, servers []serverDefinition) error {
	if len(servers) == 0 {
		return nil
	}

	// Create worker pool with configurable concurrency limit
	maxWorkers := getEnvIntOrDefault("STATION_MCP_POOL_WORKERS", 5) // Default: 5 workers
	if len(servers) < maxWorkers {
		maxWorkers = len(servers)
	}

	// Channel to send servers to workers
	serverChan := make(chan serverDefinition, len(servers))

	// Channel to collect results
	type serverResult struct {
		server serverDefinition
		err    error
	}
	resultChan := make(chan serverResult, len(servers))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for server := range serverChan {
				logging.Debug("Worker %d starting pooled server: %s", workerID, server.key)
				err := mcm.startPooledServer(ctx, server)
				resultChan <- serverResult{server: server, err: err}
			}
		}(i)
	}

	// Send all servers to workers
	for _, server := range servers {
		serverChan <- server
	}
	close(serverChan)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and track any errors
	var errorServers []string
	successCount := 0

	for result := range resultChan {
		if result.err != nil {
			logging.Info("Warning: failed to start pooled server %s: %v", result.server.key, result.err)
			errorServers = append(errorServers, result.server.key)
		} else {
			successCount++
		}
	}

	logging.Info("Parallel server startup completed: %d successful, %d failed", successCount, len(errorServers))

	if len(errorServers) > 0 {
		logging.Info("Failed servers: %v", errorServers)
	}

	// Return error if no servers started successfully
	if successCount == 0 && len(servers) > 0 {
		return fmt.Errorf("failed to start any pooled servers (%d failures)", len(errorServers))
	}

	return nil
}

// startPooledServer starts a server and adds it to the pool
func (mcm *MCPConnectionManager) startPooledServer(ctx context.Context, server serverDefinition) error {
	// Create telemetry span for MCP server startup
	tracer := otel.Tracer("station-mcp")
	ctx, span := tracer.Start(ctx, "mcp.server.start",
		trace.WithAttributes(
			attribute.String("mcp.server.name", server.name),
			attribute.String("mcp.server.key", server.key),
		),
	)
	defer span.End()

	mcm.serverPool.mutex.Lock()
	defer mcm.serverPool.mutex.Unlock()

	// Check if already started
	if _, exists := mcm.serverPool.servers[server.key]; exists {
		span.SetAttributes(attribute.Bool("mcp.server.already_running", true))
		return nil
	}

	logging.Info("Starting pooled MCP server: %s", server.key)

	// Create server client (same logic as connectToMCPServer)
	client, tools, err := mcm.createServerClient(ctx, server.name, server.config)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("mcp.server.success", false),
			attribute.String("mcp.server.error", err.Error()),
		)
		return fmt.Errorf("failed to create server client for %s: %w", server.key, err)
	}

	// Store in pool
	mcm.serverPool.servers[server.key] = client
	mcm.serverPool.serverConfigs[server.key] = server.config
	mcm.serverPool.tools[server.key] = tools

	span.SetAttributes(
		attribute.Bool("mcp.server.success", true),
		attribute.Int("mcp.server.tools_count", len(tools)),
	)

	logging.Info(" Pooled server %s started with %d tools", server.key, len(tools))
	return nil
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
					logging.Info(" Using pooled server %s with %d tools", serverKey, len(tools))
				}
			} else {
				// Server not in pool - fallback to creating fresh connection
				logging.Info("Warning:  Server %s not in pool, creating fresh connection", serverKey)
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
