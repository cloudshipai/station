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
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EnvironmentToolCache caches tools and clients for an environment
type EnvironmentToolCache struct {
	Tools    []ai.Tool
	Clients  []*mcp.GenkitMCPClient
	CachedAt time.Time
	ValidFor time.Duration
}

// AgentToolCache caches agent tools for performance
type AgentToolCache struct {
	Tools    []ai.Tool
	CachedAt time.Time
	ValidFor time.Duration
}

// IsValid checks if the cached agent tools are still valid
func (cache *AgentToolCache) IsValid() bool {
	return time.Since(cache.CachedAt) < cache.ValidFor
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
	agentToolCache   map[string]*AgentToolCache // Cache for agent tools with string keys
	cacheMutex       sync.RWMutex
	serverPool       *MCPServerPool
	poolingEnabled   bool                  // Feature flag for connection pooling
	agentService     AgentServiceInterface // For agent tool integration
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
		agentToolCache: make(map[string]*AgentToolCache),
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

// InitializeServerPool starts MCP servers for a specific environment only (performance optimization)
// CRITICAL FIX: Only initialize servers for the current environment, not ALL environments
// This prevents 54-second initialization overhead when only 1-2 servers are needed
func (mcm *MCPConnectionManager) InitializeServerPool(ctx context.Context, environmentID int64) error {
	if !mcm.poolingEnabled {
		return nil // Skip if pooling disabled
	}

	// Check if this environment's servers are already initialized
	mcm.serverPool.mutex.Lock()
	poolKey := fmt.Sprintf("env_%d", environmentID)
	if mcm.serverPool.initializedEnvs == nil {
		mcm.serverPool.initializedEnvs = make(map[string]bool)
	}
	if mcm.serverPool.initializedEnvs[poolKey] {
		mcm.serverPool.mutex.Unlock()
		logging.Debug("Server pool for environment %d already initialized", environmentID)
		return nil
	}
	mcm.serverPool.initializedEnvs[poolKey] = true
	mcm.serverPool.mutex.Unlock()

	logging.Info("Initializing MCP server pool for environment %d...", environmentID)

	// Only get servers for THIS environment (not all environments!)
	fileConfigs, err := mcm.repos.FileMCPConfigs.ListByEnvironment(environmentID)
	if err != nil {
		return fmt.Errorf("failed to get file configs for environment %d: %w", environmentID, err)
	}

	servers := mcm.extractServerDefinitions(environmentID, fileConfigs)

	// Start unique servers in parallel for faster initialization
	uniqueServers := mcm.deduplicateServers(servers)
	if err := mcm.startPooledServersParallel(ctx, uniqueServers); err != nil {
		return fmt.Errorf("failed to start pooled servers in parallel: %w", err)
	}

	logging.Info("MCP server pool initialized for environment %d with %d servers", environmentID, len(uniqueServers))
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

	// Add agent tools to the mix
	agentTools := mcm.getAgentToolsForEnvironment(ctx, environmentID, mcm.agentService)
	allTools = append(allTools, agentTools...)

	// TEMPORARY FIX: Completely disable caching to fix stdio MCP connection issues

	mcpConnDuration := time.Since(mcpConnStartTime)
	logging.Info("MCP_CONN_PERF: Legacy connections completed in %v", mcpConnDuration)

	return allTools, allClients, nil
}

// getAgentToolsForEnvironment creates AgentToolWrapper instances for all agents in an environment
// with caching for improved performance
func (mcm *MCPConnectionManager) getAgentToolsForEnvironment(ctx context.Context, environmentID int64, agentService AgentServiceInterface) []ai.Tool {
	// Check cache first for agent tools
	cacheKey := fmt.Sprintf("agents_%d", environmentID)
	mcm.cacheMutex.RLock()
	if cachedTools, exists := mcm.agentToolCache[cacheKey]; exists && cachedTools.IsValid() {
		mcm.cacheMutex.RUnlock()
		logging.Debug("Using cached agent tools for environment %d (age: %v)", environmentID, time.Since(cachedTools.CachedAt))
		return cachedTools.Tools
	}
	mcm.cacheMutex.RUnlock()

	// Get all agents for this environment
	var agents []*models.Agent
	var err error

	// Handle case where agentService provides agent listing (e.g., for testing)
	if agentService != nil {
		if lister, ok := agentService.(interface {
			ListAgentsByEnvironment(context.Context, int64) ([]*models.Agent, error)
		}); ok {
			agents, err = lister.ListAgentsByEnvironment(ctx, environmentID)
		} else if mcm.repos != nil && mcm.repos.Agents != nil {
			agents, err = mcm.repos.Agents.ListByEnvironment(environmentID)
		} else {
			logging.Debug("No agent source available for environment %d", environmentID)
			return nil
		}
	} else if mcm.repos != nil && mcm.repos.Agents != nil {
		agents, err = mcm.repos.Agents.ListByEnvironment(environmentID)
	} else {
		logging.Debug("No repos available to get agents for environment %d", environmentID)
		return nil
	}

	if err != nil {
		logging.Error("Failed to get agents for environment %d: %v", environmentID, err)
		return nil
	}

	var agentTools []ai.Tool
	for _, agent := range agents {
		// Create tool function that wraps the agent execution
		agentID := agent.ID
		agentName := agent.Name
		agentDesc := agent.Description

		// Format tool name with __ prefix and lowercase - normalize ALL special characters to underscores
		// This must match the tool name format used in agent .prompt files
		normalizedName := strings.ToLower(agentName)
		// Replace all special characters with underscores (same as AgentAsTool.Name())
		replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
		for _, char := range replacements {
			normalizedName = strings.ReplaceAll(normalizedName, char, "_")
		}
		// Remove multiple consecutive underscores
		for strings.Contains(normalizedName, "__") {
			normalizedName = strings.ReplaceAll(normalizedName, "__", "_")
		}
		// Trim leading/trailing underscores
		normalizedName = strings.Trim(normalizedName, "_")

		toolName := fmt.Sprintf("__agent_%s", normalizedName)

		// Create the tool function that executes the agent
		toolFunc := func(toolCtx *ai.ToolContext, input interface{}) (interface{}, error) {
			// Extract the task from input
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid input format for agent tool: expected map[string]interface{}, got %T", input)
			}

			task, ok := inputMap["task"].(string)
			if !ok {
				return nil, fmt.Errorf("missing required 'task' parameter for agent tool")
			}

			// Get parent run ID from context for hierarchical tracking
			parentRunID := GetParentRunIDFromContext(toolCtx.Context)

			// CRITICAL FIX: Create database connection with retry logic for concurrent access
			// This prevents SQLite locking issues when multiple child agents execute simultaneously
			cfg, err := config.Load()
			if err != nil {
				return nil, fmt.Errorf("failed to load config for child agent: %w", err)
			}

			// Retry run creation with exponential backoff to handle concurrent writes
			var newRunID int64
			maxRetries := 3
			baseDelay := 100 * time.Millisecond
			defaultUserID := int64(1)

			for attempt := 0; attempt < maxRetries; attempt++ {
				// Create fresh database connection for each attempt
				childDB, err := db.New(cfg.DatabaseURL)
				if err != nil {
					if attempt == maxRetries-1 {
						return nil, fmt.Errorf("failed to create database connection after %d attempts: %w", maxRetries, err)
					}
					time.Sleep(baseDelay * time.Duration(1<<uint(attempt)))
					continue
				}

				childRepos := repositories.New(childDB)

				// Create run with retry
				var newRun *models.AgentRun
				if parentRunID != nil {
					logging.Info("Agent tool %s: Creating child run with parent run ID %d (attempt %d/%d)", toolName, *parentRunID, attempt+1, maxRetries)
					newRun, err = childRepos.AgentRuns.CreateWithMetadata(
						toolCtx.Context,
						agentID,
						defaultUserID,
						task,
						"",
						0, nil, nil,
						"running",
						nil, nil, nil, nil, nil, nil, nil,
						parentRunID,
					)
				} else {
					logging.Info("Agent tool %s: Creating root run (attempt %d/%d)", toolName, attempt+1, maxRetries)
					newRun, err = childRepos.AgentRuns.Create(
						toolCtx.Context,
						agentID,
						defaultUserID,
						task,
						"", 0, nil, nil,
						"running",
						nil,
					)
				}

				childDB.Close() // Close immediately after run creation

				if err == nil {
					newRunID = newRun.ID
					if parentRunID != nil {
						logging.Info("Agent tool %s: Created child run ID %d (parent: %d)", toolName, newRunID, *parentRunID)
					} else {
						logging.Info("Agent tool %s: Created root run ID %d", toolName, newRunID)
					}
					break // Success!
				}

				// If this was the last attempt, return error
				if attempt == maxRetries-1 {
					return nil, fmt.Errorf("failed to create child run after %d attempts: %w", maxRetries, err)
				}

				// Exponential backoff before retry
				delay := baseDelay * time.Duration(1<<uint(attempt))
				logging.Info("Agent tool %s: Retrying run creation after %v (attempt %d/%d)", toolName, delay, attempt+1, maxRetries)
				time.Sleep(delay)
			}

			// Add the NEW run ID to context as parent for nested calls
			execCtx := WithParentRunID(toolCtx.Context, newRunID)

			// Execute the agent using the EXISTING agentService (which has MCP connections)
			// The agentService will use its own repos for execution but we pass the run ID we created
			result, err := agentService.ExecuteAgentWithRunID(execCtx, agentID, task, newRunID, nil)

			// CRITICAL: Debug logging to understand what we got back
			if result != nil {
				logging.Info("Agent tool %s: Got result - Content length: %d, Extra fields: %d", toolName, len(result.Content), len(result.Extra))
				if len(result.Content) > 0 {
					preview := result.Content
					if len(preview) > 100 {
						preview = preview[:100] + "..."
					}
					logging.Info("Agent tool %s: Result content preview: %s", toolName, preview)
				}
			} else {
				logging.Info("Agent tool %s: Result is nil!", toolName)
			}

			// CRITICAL: Update the child run with completion status and result
			// This ensures child agent responses are saved in the database with parent tracking
			completedAt := time.Now()
			status := "completed"
			var errorMsg *string

			if err != nil {
				status = "failed"
				errStr := err.Error()
				errorMsg = &errStr
				logging.Error("Agent tool %s: Child agent execution failed: %v", toolName, err)
			}

			// Extract metadata from result if successful
			var durationSeconds *float64
			var inputTokens, outputTokens, totalTokens *int64
			var modelName *string
			var toolsUsed *int64
			var toolCalls, executionSteps *models.JSONArray
			stepsTaken := int64(0)
			finalResponse := ""

			// Extract response content (always present in result.Content)
			if result != nil {
				finalResponse = result.Content
				logging.Info("Agent tool %s: Saving child response (length: %d)", toolName, len(finalResponse))
			}

			// Extract additional metadata from Extra if available
			if result != nil && result.Extra != nil {
				logging.Debug("🔍 Agent tool %s: Extracting metadata from Extra (keys: %v)", toolName, func() []string {
					keys := make([]string, 0, len(result.Extra))
					for k := range result.Extra {
						keys = append(keys, k)
					}
					return keys
				}())

				if dur, ok := result.Extra["duration"].(float64); ok {
					durationSeconds = &dur
				}

				// Try to extract tool_calls
				if tCalls, ok := result.Extra["tool_calls"].(*models.JSONArray); ok {
					toolCalls = tCalls
					logging.Debug("🔍 Agent tool %s: Extracted tool_calls (count: %d)", toolName, len(*toolCalls))
				} else if result.Extra["tool_calls"] != nil {
					logging.Debug("🔍 Agent tool %s: tool_calls present but wrong type: %T", toolName, result.Extra["tool_calls"])
				}

				// Try to extract execution_steps
				if eSteps, ok := result.Extra["execution_steps"].(*models.JSONArray); ok {
					executionSteps = eSteps
					logging.Debug("🔍 Agent tool %s: Extracted execution_steps (count: %d)", toolName, len(*executionSteps))
				} else if result.Extra["execution_steps"] != nil {
					logging.Debug("🔍 Agent tool %s: execution_steps present but wrong type: %T", toolName, result.Extra["execution_steps"])
				}
				if steps, ok := result.Extra["steps_taken"].(int); ok {
					stepsTaken = int64(steps)
				} else if steps, ok := result.Extra["steps_taken"].(int64); ok {
					stepsTaken = steps
				}
				if model, ok := result.Extra["model_name"].(string); ok {
					modelName = &model
				}
				if tools, ok := result.Extra["tools_used"].(int); ok {
					t := int64(tools)
					toolsUsed = &t
				} else if tools, ok := result.Extra["tools_used"].(int64); ok {
					toolsUsed = &tools
				}

				// Extract token usage
				if tokenUsage, ok := result.Extra["token_usage"].(map[string]interface{}); ok {
					if inTok, ok := tokenUsage["input_tokens"].(int64); ok {
						inputTokens = &inTok
					} else if inTok, ok := tokenUsage["input_tokens"].(int); ok {
						t := int64(inTok)
						inputTokens = &t
					}
					if outTok, ok := tokenUsage["output_tokens"].(int64); ok {
						outputTokens = &outTok
					} else if outTok, ok := tokenUsage["output_tokens"].(int); ok {
						t := int64(outTok)
						outputTokens = &t
					}
					if totTok, ok := tokenUsage["total_tokens"].(int64); ok {
						totalTokens = &totTok
					} else if totTok, ok := tokenUsage["total_tokens"].(int); ok {
						t := int64(totTok)
						totalTokens = &t
					}
				}
			}

			// Update the child run with completion data using a fresh database connection
			// This prevents locking issues with parent's connection
			updateErr := func() error {
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				updateDB, err := db.New(cfg.DatabaseURL)
				if err != nil {
					return fmt.Errorf("failed to create database connection: %w", err)
				}
				defer updateDB.Close()

				updateRepos := repositories.New(updateDB)
				return updateRepos.AgentRuns.UpdateCompletionWithMetadata(
					toolCtx.Context,
					newRunID,
					finalResponse,
					stepsTaken,
					toolCalls,
					executionSteps,
					status,
					&completedAt,
					inputTokens,
					outputTokens,
					totalTokens,
					durationSeconds,
					modelName,
					toolsUsed,
					errorMsg,
				)
			}()

			if updateErr != nil {
				logging.Error("Agent tool %s: Failed to update child run %d completion: %v", toolName, newRunID, updateErr)
				// Don't fail the execution - we already have the result
			} else {
				logging.Info("Agent tool %s: Updated child run %d with status=%s, response_len=%d", toolName, newRunID, status, len(finalResponse))
			}

			// Return error if execution failed
			if err != nil {
				return nil, fmt.Errorf("agent execution failed: %w", err)
			}

			if result == nil {
				return "", nil
			}

			return result.Content, nil
		}

		// Define input schema for the agent tool
		inputSchema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "The task to execute with this agent",
				},
			},
			"required": []string{"task"},
		}

		// Create a proper GenKit tool using NewToolWithInputSchema
		// This ensures the tool is properly registered and discoverable
		tool := ai.NewToolWithInputSchema(
			toolName,
			agentDesc,
			inputSchema,
			toolFunc,
		)

		logging.Debug("Created GenKit tool for agent '%s' as '%s'", agentName, toolName)
		agentTools = append(agentTools, tool)
	}

	// Cache the agent tools for future use
	mcm.cacheMutex.Lock()
	mcm.agentToolCache[cacheKey] = &AgentToolCache{
		Tools:    agentTools,
		CachedAt: time.Now(),
		ValidFor: 5 * time.Minute, // Cache for 5 minutes - agents don't change frequently
	}
	mcm.cacheMutex.Unlock()

	logging.Info("Created and cached %d agent tools for environment %d", len(agentTools), environmentID)
	return agentTools
}

// AgentAsTool implements ai.Tool interface to make agents callable as tools
// DEPRECATED: Production code now uses ai.NewToolWithInputSchema in getAgentToolsForEnvironment
// This struct is kept for test compatibility
type AgentAsTool struct {
	agentID       int64
	agentName     string
	description   string
	repos         *repositories.Repositories
	genkitApp     *genkit.Genkit
	environmentID int64
	agentService  AgentServiceInterface
	parentRunID   *int64 // Track parent run for hierarchical execution
}

// Name returns the tool name (prefixed with __ to avoid conflicts)
func (a *AgentAsTool) Name() string {
	// Convert to lowercase and replace spaces and special characters with underscores
	name := strings.ToLower(a.agentName)
	// Replace spaces and common special characters with underscores
	replacements := []string{" ", "-", ".", "!", "@", "#", "$", "%", "^", "&", "*", "(", ")", "+", "=", "[", "]", "{", "}", "|", "\\", ":", ";", "\"", "'", "<", ">", ",", "?", "/"}
	for _, char := range replacements {
		name = strings.ReplaceAll(name, char, "_")
	}
	// Remove multiple consecutive underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}
	// Trim leading/trailing underscores
	name = strings.Trim(name, "_")
	return fmt.Sprintf("__agent_%s", name)
}

// Definition returns the tool definition for models
func (a *AgentAsTool) Definition() *ai.ToolDefinition {
	return &ai.ToolDefinition{
		Name:        a.Name(),
		Description: a.description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"task": map[string]interface{}{
					"type":        "string",
					"description": "The task to execute with this agent",
				},
			},
			"required": []string{"task"},
		},
		OutputSchema: map[string]interface{}{
			"type": "string",
		},
	}
}

// RunRaw runs this tool using the provided raw input
func (a *AgentAsTool) RunRaw(ctx context.Context, input any) (any, error) {
	// Enhanced input validation with detailed error messages
	if input == nil {
		return nil, fmt.Errorf("agent tool %s received nil input - expected map with 'task' field", a.Name())
	}

	// Extract the task from input
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid input format for agent tool %s: expected map[string]interface{}, got %T", a.Name(), input)
	}

	task, ok := inputMap["task"].(string)
	if !ok {
		// Provide helpful error message based on what's actually in the input
		if taskVal, exists := inputMap["task"]; exists {
			return nil, fmt.Errorf("invalid 'task' parameter for agent tool %s: expected string, got %T (value: %v)", a.Name(), taskVal, taskVal)
		} else {
			return nil, fmt.Errorf("missing required 'task' parameter for agent tool %s: input map keys are %v", a.Name(), getMapKeys(inputMap))
		}
	}

	// Validate task content
	if strings.TrimSpace(task) == "" {
		return nil, fmt.Errorf("empty 'task' parameter for agent tool %s: task cannot be empty or whitespace only", a.Name())
	}

	if len(task) > 10000 {
		return nil, fmt.Errorf("task parameter too long for agent tool %s: length %d exceeds maximum of 10000 characters", a.Name(), len(task))
	}

	// Execute the agent using the agent service
	if a.agentService == nil {
		logging.Error("AgentAsTool: Agent service is nil for tool %s (agent_id=%d, env_id=%d)", a.Name(), a.agentID, a.environmentID)
		return nil, fmt.Errorf("agent service not available for agent tool %s (agent_id=%d, env_id=%d): this indicates a configuration issue", a.agentName, a.agentID, a.environmentID)
	}

	// Get the parent run ID from context to enable proper hierarchical tracking
	parentRunID := GetParentRunIDFromContext(ctx)

	// Create a NEW run for this child agent execution with proper parent tracking
	// This is critical: we need a NEW run ID for the child, with parent_run_id set
	// Use user ID 1 (console user) as default for agent-initiated runs
	defaultUserID := int64(1)
	var newRunID int64
	var createErr error

	if parentRunID != nil {
		logging.Info("AgentAsTool: Creating child run for agent %s with parent run ID %d", a.agentName, *parentRunID)
		// Create run with parent_run_id set for hierarchical tracking
		newRun, err := a.repos.AgentRuns.CreateWithMetadata(
			ctx,
			a.agentID,
			defaultUserID, // Use console user for agent-initiated runs
			task,
			"",       // finalResponse - will be updated after execution
			0,        // stepsTaken - will be updated
			nil, nil, // toolCalls, executionSteps - will be updated
			"running",     // initial status
			nil,           // completedAt - will be set when done
			nil, nil, nil, // tokens - will be updated
			nil,         // duration - will be updated
			nil,         // modelName - will be updated
			nil,         // toolsUsed - will be updated
			parentRunID, // CRITICAL: Set the parent run ID
		)
		if err != nil {
			logging.Error("AgentAsTool: Failed to create child run for agent %s: %v", a.agentName, err)
			createErr = fmt.Errorf("failed to create child run: %w", err)
		} else {
			newRunID = newRun.ID
			logging.Info("AgentAsTool: Created child run ID %d for agent %s (parent: %d)", newRunID, a.agentName, *parentRunID)
		}
	} else {
		logging.Info("AgentAsTool: Creating root run for agent %s (no parent)", a.agentName)
		// Create run without parent (root execution)
		newRun, err := a.repos.AgentRuns.Create(
			ctx,
			a.agentID,
			defaultUserID, // Use console user for agent-initiated runs
			task,
			"",       // finalResponse
			0,        // stepsTaken
			nil, nil, // toolCalls, executionSteps
			"running",
			nil, // completedAt
		)
		if err != nil {
			logging.Error("AgentAsTool: Failed to create root run for agent %s: %v", a.agentName, err)
			createErr = fmt.Errorf("failed to create run: %w", err)
		} else {
			newRunID = newRun.ID
			logging.Info("AgentAsTool: Created root run ID %d for agent %s", newRunID, a.agentName)
		}
	}

	if createErr != nil {
		return nil, createErr
	}

	// Add the NEW run ID to context as the parent for any further nested calls
	execCtx := WithParentRunID(ctx, newRunID)

	// Execute the agent with the provided task (with configurable timeout protection)
	// Get timeout from context or use default (10 minutes for nested agents)
	timeout := 10 * time.Minute // Shorter timeout for nested agents
	if deadline, ok := ctx.Deadline(); ok {
		remainingTime := time.Until(deadline)
		if remainingTime > 0 && remainingTime < timeout {
			timeout = remainingTime - 5*time.Second // Leave 5 seconds buffer
		}
	}

	execCtx, cancel := context.WithTimeout(execCtx, timeout)
	defer cancel()

	// Execute with the NEW run ID (not the parent's!)
	result, err := a.agentService.ExecuteAgentWithRunID(execCtx, a.agentID, task, newRunID, nil)
	if err != nil {
		// Enhanced error context for better debugging
		logging.Error("AgentAsTool execution failed: agent=%s (ID: %d), parent_run_id=%v, task_length=%d, error: %v",
			a.agentName, a.agentID, parentRunID, len(task), err)

		// Create structured error with context
		errorContext := map[string]interface{}{
			"agent_name":     a.agentName,
			"agent_id":       a.agentID,
			"parent_run_id":  parentRunID,
			"task_length":    len(task),
			"environment_id": a.environmentID,
		}

		// Check if this is a timeout or specific error type
		if strings.Contains(err.Error(), "timeout") || execCtx.Err() == context.DeadlineExceeded {
			errorContext["error_type"] = "timeout"
			errorContext["timeout_duration"] = timeout.String()
			logging.Error("Agent execution timed out after %v: agent=%s (ID: %d), parent_run_id=%v, task_length=%d",
				timeout, a.agentName, a.agentID, parentRunID, len(task))
			return nil, fmt.Errorf("agent execution timed out after %v: agent %s (ID: %d) failed to complete within the allowed time. Consider increasing timeout or breaking task into smaller chunks: %w",
				timeout, a.agentName, a.agentID, err)
		} else if strings.Contains(err.Error(), "rate limit") {
			errorContext["error_type"] = "rate_limit"
			return nil, fmt.Errorf("rate limit exceeded while executing agent %s (ID: %d): %w", a.agentName, a.agentID, err)
		} else if strings.Contains(err.Error(), "connection") {
			errorContext["error_type"] = "connection"
			return nil, fmt.Errorf("connection error while executing agent %s (ID: %d): %w", a.agentName, a.agentID, err)
		}

		// Generic execution error with context
		return nil, fmt.Errorf("agent execution failed: agent %s (ID: %d, parent_run_id=%v, env_id=%d): %w",
			a.agentName, a.agentID, parentRunID, a.environmentID, err)
	}

	// Return the result text
	if result == nil {
		logging.Info("AgentAsTool: Agent %s (ID: %d) returned nil result, returning empty string", a.agentName, a.agentID)
		return "", nil
	}

	// Validate result content
	if result.Content == "" {
		logging.Debug("AgentAsTool: Agent %s (ID: %d) returned empty content", a.agentName, a.agentID)
		return "", nil
	}

	// Log successful execution for debugging
	logging.Debug("AgentAsTool: Successfully executed agent %s (ID: %d) with task length %d, returning result length %d",
		a.agentName, a.agentID, len(task), len(result.Content))

	return result.Content, nil
}

// Respond constructs a Part with a ToolResponse for a given interrupted tool request
func (a *AgentAsTool) Respond(toolReq *ai.Part, outputData any, opts *ai.RespondOptions) *ai.Part {
	return ai.NewResponseForToolRequest(toolReq, outputData)
}

// Restart constructs a Part with a new ToolRequest to re-trigger a tool
func (a *AgentAsTool) Restart(toolReq *ai.Part, opts *ai.RestartOptions) *ai.Part {
	// Extract the original input from the tool request
	if toolReq.ToolRequest != nil {
		return ai.NewToolRequestPart(&ai.ToolRequest{
			Name:  a.Name(),
			Input: toolReq.ToolRequest.Input,
		})
	}
	// Fallback: create a new tool request with empty input
	return ai.NewToolRequestPart(&ai.ToolRequest{
		Name:  a.Name(),
		Input: map[string]interface{}{},
	})
}

// Register registers the tool with the given registry
func (a *AgentAsTool) Register(r api.Registry) {
	// Implementation depends on the registry interface
	// For now, this is a no-op as tools are registered through other mechanisms
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
	clientCtx, clientCancel := context.WithTimeout(ctx, 30*time.Second)
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
		logging.Error("CRITICAL: CRITICAL: MCP client creation for server '%s' TIMED OUT after 30 seconds", serverName)
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
