package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"
	
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// ConfigSyncer handles MCP configuration synchronization between filesystem and database
type ConfigSyncer struct {
	repos *repositories.Repositories
}

// NewConfigSyncer creates a new configuration syncer
func NewConfigSyncer(repos *repositories.Repositories) *ConfigSyncer {
	return &ConfigSyncer{
		repos: repos,
	}
}

// SyncOptions contains options for sync operations
type SyncOptions struct {
	DryRun bool
	Force  bool
}

// SyncResult contains the results of a sync operation
type SyncResult struct {
	SyncedConfigs        []string
	RemovedConfigs       []string
	SyncErrors           []SyncError
	OrphanedToolsRemoved int
	AffectedAgents       []string
}

// SyncError represents an error that occurred during sync
type SyncError struct {
	ConfigName string
	Error      error
}

// FileConfig represents a configuration file discovered on the filesystem
type FileConfig struct {
	ConfigName   string
	TemplatePath string
	LastLoadedAt *time.Time
}

// DiscoverConfigs scans the filesystem for JSON config files in the environment directory
func (s *ConfigSyncer) DiscoverConfigs(environment string) ([]*FileConfig, error) {
	// Get config directory path
	configDir := os.ExpandEnv("$HOME/.config/station")
	envDir := filepath.Join(configDir, "environments", environment)
	
	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return []*FileConfig{}, nil
	}
	
	// Read all files in environment directory
	files, err := os.ReadDir(envDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read environment directory %s: %w", envDir, err)
	}
	
	var configs []*FileConfig
	for _, file := range files {
		// Skip non-JSON files and variables.yml
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") || 
		   file.Name() == "variables.yml" {
			continue
		}
		
		// Get file info
		fileInfo, err := file.Info()
		if err != nil {
			continue
		}
		
		// Extract config name from filename (remove .json extension and timestamp suffix if present)
		configName := strings.TrimSuffix(file.Name(), ".json")
		
		// Create a FileConfig structure for filesystem files
		modTime := fileInfo.ModTime()
		config := &FileConfig{
			ConfigName:   configName,
			TemplatePath: filepath.Join("environments", environment, file.Name()),
			LastLoadedAt: &modTime,
		}
		
		configs = append(configs, config)
	}
	
	return configs, nil
}

// LoadConfig loads a config file from filesystem, processes templates, and registers servers/tools
func (s *ConfigSyncer) LoadConfig(envID int64, environment, configName string, fileConfig *FileConfig) error {
	// Get config directory path
	configDir := os.ExpandEnv("$HOME/.config/station")
	configPath := filepath.Join(configDir, fileConfig.TemplatePath)
	
	// Read the config file
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Process template variables
	templateService := services.NewTemplateVariableService(configDir, s.repos)
	result, err := templateService.ProcessTemplateWithVariables(envID, configName, string(rawContent), false)
	if err != nil {
		return fmt.Errorf("failed to process template variables: %w", err)
	}
	
	// Check if all variables were resolved
	if !result.AllResolved {
		missingVars := make([]string, 0, len(result.MissingVars))
		for _, missingVar := range result.MissingVars {
			missingVars = append(missingVars, missingVar.Name)
		}
		return fmt.Errorf("missing template variables: %v. Please update %s/environments/%s/variables.yml", 
			missingVars, configDir, environment)
	}
	
	// Parse the rendered JSON
	var configData map[string]interface{}
	if err := json.Unmarshal([]byte(result.RenderedContent), &configData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	
	// Extract MCP servers from config
	var serversData map[string]interface{}
	if mcpServers, ok := configData["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := configData["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		return fmt.Errorf("no 'mcpServers' or 'servers' field found in config")
	}
	
	// Create or update file config record
	now := time.Now()
	var fileConfigID int64
	
	// Check if config already exists
	existingConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(envID)
	if err != nil {
		return fmt.Errorf("failed to check existing configs: %w", err)
	}
	
	var existingConfig *repositories.FileConfigRecord
	for _, existing := range existingConfigs {
		if existing.ConfigName == configName {
			existingConfig = existing
			break
		}
	}
	
	if existingConfig != nil {
		// Update existing config
		fileConfigID = existingConfig.ID
		err = s.repos.FileMCPConfigs.UpdateLastLoadedAt(fileConfigID)
		if err != nil {
			return fmt.Errorf("failed to update config timestamp: %w", err)
		}
	} else {
		// Create new config
		fileConfigID, err = s.repos.FileMCPConfigs.Create(&repositories.FileConfigRecord{
			EnvironmentID: envID,
			ConfigName:    configName,
			TemplatePath:  fileConfig.TemplatePath,
			LastLoadedAt:  &now,
		})
		if err != nil {
			return fmt.Errorf("failed to create file config record: %w", err)
		}
	}
	
	// Create MCP servers and discover tools
	for serverName, serverConfig := range serversData {
		serverConfigMap, ok := serverConfig.(map[string]interface{})
		if !ok {
			continue
		}
		
		// Create MCP server record
		server := &models.MCPServer{
			EnvironmentID: envID,
			Name:          serverName,
			FileConfigID:  &fileConfigID,
		}
		
		// Extract server configuration
		if command, ok := serverConfigMap["command"].(string); ok {
			server.Command = command
		}
		if url, ok := serverConfigMap["url"].(string); ok {
			server.Command = url // Store URL in command field for HTTP servers
		}
		if argsInterface, ok := serverConfigMap["args"]; ok {
			if argsArray, ok := argsInterface.([]interface{}); ok {
				args := make([]string, len(argsArray))
				for i, arg := range argsArray {
					if argStr, ok := arg.(string); ok {
						args[i] = argStr
					}
				}
				server.Args = args
			}
		}
		if envInterface, ok := serverConfigMap["env"]; ok {
			if envMap, ok := envInterface.(map[string]interface{}); ok {
				env := make(map[string]string)
				for k, v := range envMap {
					if vStr, ok := v.(string); ok {
						env[k] = vStr
					}
				}
				server.Env = env
			}
		}
		
		// Create server (or try to find existing one)
		serverID, err := s.repos.MCPServers.Create(server)
		if err != nil {
			return fmt.Errorf("failed to create/update MCP server %s: %w", serverName, err)
		}
		
		// Discover and register tools for this server
		err = s.discoverToolsForServer(serverID, serverName, serverConfigMap, result.RenderedContent)
		if err != nil {
			// Don't fail the entire sync for tool discovery errors, just log them
			logging.Debug("Tool discovery failed for server %s: %v", serverName, err)
		}
	}
	
	return nil
}

// RemoveOrphanedAgentTools removes tools from agents when their associated config is deleted
func (s *ConfigSyncer) RemoveOrphanedAgentTools(agents []*models.Agent, configID int64, envID int64) (int, error) {
	var removedCount int
	
	// Get all tools associated with servers from this config
	orphanedServers, err := s.repos.MCPServers.GetByEnvironmentID(envID)
	if err != nil {
		return 0, fmt.Errorf("failed to get servers: %w", err)
	}
	
	var orphanedTools []*models.MCPTool
	for _, server := range orphanedServers {
		tools, err := s.repos.MCPTools.GetByServerID(server.ID)
		if err != nil {
			continue
		}
		orphanedTools = append(orphanedTools, tools...)
	}
	
	// Remove tool assignments from agents and delete tools
	for _, agent := range agents {
		agentTools, err := s.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			continue
		}
		
		var toolsToRemove []int64
		for _, agentTool := range agentTools {
			for _, orphanedTool := range orphanedTools {
				if agentTool.ToolID == orphanedTool.ID {
					toolsToRemove = append(toolsToRemove, orphanedTool.ID)
					break
				}
			}
		}
		
		// Remove tool assignments
		for _, toolID := range toolsToRemove {
			err = s.repos.AgentTools.RemoveAgentTool(agent.ID, toolID)
			if err != nil {
				continue
			}
			removedCount++
		}
		
		// Log agent health event if tools were removed
		if len(toolsToRemove) > 0 {
			impact := s.DetermineImpactLevel(len(toolsToRemove))
			// In a real implementation, this would go to a proper logging system
			fmt.Printf("📋 Agent Health Event: Agent %d - tool_removed (orphaned_config) - Removed %d tools from deleted config - Impact: %s\n", 
				agent.ID, len(toolsToRemove), impact)
		}
	}
	
	// Delete the orphaned tools and servers
	for _, server := range orphanedServers {
		// Delete tools by server ID first
		_ = s.repos.MCPTools.DeleteByServerID(server.ID)
		// Then delete the server
		_ = s.repos.MCPServers.Delete(server.ID)
	}
	
	return removedCount, nil
}

// DetermineImpactLevel determines the impact level based on number of tools removed
func (s *ConfigSyncer) DetermineImpactLevel(toolsRemoved int) string {
	if toolsRemoved >= 5 {
		return "high"
	} else if toolsRemoved >= 2 {
		return "medium"
	}
	return "low"
}

// Sync performs the complete sync operation
func (s *ConfigSyncer) Sync(environment string, envID int64, options SyncOptions) (*SyncResult, error) {
	result := &SyncResult{}
	
	// Get current database state
	dbConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list database configs: %w", err)
	}
	
	// Discover actual config files from filesystem
	fileConfigs, err := s.DiscoverConfigs(environment)
	if err != nil {
		return nil, fmt.Errorf("failed to discover config files: %w", err)
	}
	
	// Get all agents in this environment
	agents, err := s.repos.Agents.ListByEnvironment(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	
	// Create maps for comparison
	fileConfigMap := make(map[string]*FileConfig)
	dbConfigMap := make(map[string]*repositories.FileConfigRecord)
	
	for _, fileConfig := range fileConfigs {
		fileConfigMap[fileConfig.ConfigName] = fileConfig
	}
	
	for _, dbConfig := range dbConfigs {
		dbConfigMap[dbConfig.ConfigName] = dbConfig
	}
	
	// Find configs that exist in filesystem but not in database (new configs to sync)
	for _, fileConfig := range fileConfigs {
		dbConfig, existsInDB := dbConfigMap[fileConfig.ConfigName]
		
		if !existsInDB || options.Force {
			// New config or force sync requested
			result.SyncedConfigs = append(result.SyncedConfigs, fileConfig.ConfigName)
		} else if dbConfig.LastLoadedAt != nil && !dbConfig.LastLoadedAt.IsZero() && fileConfig.LastLoadedAt.After(*dbConfig.LastLoadedAt) {
			// File modified after last sync
			result.SyncedConfigs = append(result.SyncedConfigs, fileConfig.ConfigName)
		}
	}
	
	// Find configs that exist in DB but not in filesystem (orphaned configs to remove)
	for _, dbConfig := range dbConfigs {
		if _, existsInFiles := fileConfigMap[dbConfig.ConfigName]; !existsInFiles {
			result.RemovedConfigs = append(result.RemovedConfigs, dbConfig.ConfigName)
		}
	}
	
	if options.DryRun {
		return result, nil
	}
	
	// Load new/updated configs
	for _, configName := range result.SyncedConfigs {
		err := s.LoadConfig(envID, environment, configName, fileConfigMap[configName])
		if err != nil {
			result.SyncErrors = append(result.SyncErrors, SyncError{
				ConfigName: configName,
				Error:      err,
			})
		}
	}
	
	// Remove orphaned configs and clean up agent tools
	for _, configName := range result.RemovedConfigs {
		// Find and remove from database
		var configToRemove *repositories.FileConfigRecord
		for _, dbConfig := range dbConfigs {
			if dbConfig.ConfigName == configName {
				configToRemove = dbConfig
				break
			}
		}
		
		if configToRemove != nil {
			// Remove agent tools that reference this config
			toolsRemoved, err := s.RemoveOrphanedAgentTools(agents, configToRemove.ID, envID)
			if err != nil {
				result.SyncErrors = append(result.SyncErrors, SyncError{
					ConfigName: configName,
					Error:      fmt.Errorf("failed to clean up agent tools: %w", err),
				})
				continue
			}
			result.OrphanedToolsRemoved += toolsRemoved
			
			// Remove the file config
			err = s.repos.FileMCPConfigs.Delete(configToRemove.ID)
			if err != nil {
				result.SyncErrors = append(result.SyncErrors, SyncError{
					ConfigName: configName,
					Error:      fmt.Errorf("failed to remove config: %w", err),
				})
			}
		}
	}
	
	return result, nil
}

// discoverToolsForServer discovers and registers tools for a specific MCP server using pure mcp-go client
func (s *ConfigSyncer) discoverToolsForServer(serverID int64, serverName string, serverConfig map[string]interface{}, renderedContent string) error {
	logging.Debug("Starting real tool discovery for server: %s", serverName)
	
	// Parse the server configuration to determine connection type
	var serverConfigStruct models.MCPServerConfig
	serverConfigBytes, err := json.Marshal(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal server config: %w", err)
	}
	
	if err := json.Unmarshal(serverConfigBytes, &serverConfigStruct); err != nil {
		return fmt.Errorf("failed to unmarshal server config: %w", err)
	}
	
	// Create proper mcp-go client based on server configuration
	var mcpClient *client.Client
	var clientTransport transport.Interface
	
	if serverConfigStruct.URL != "" {
		// HTTP-based MCP server
		logging.Debug("Connecting to HTTP MCP server: %s (URL: %s)", serverName, serverConfigStruct.URL)
		httpTransport, err := transport.NewStreamableHTTP(serverConfigStruct.URL)
		if err != nil {
			return fmt.Errorf("failed to create HTTP transport for %s: %w", serverName, err)
		}
		clientTransport = httpTransport
	} else if serverConfigStruct.Command != "" {
		// Stdio-based MCP server
		logging.Debug("Connecting to Stdio MCP server: %s (command: %s, args: %v)", serverName, serverConfigStruct.Command, serverConfigStruct.Args)
		
		// Convert env map to environment variables for the command
		var envVars []string
		for key, value := range serverConfigStruct.Env {
			envVars = append(envVars, key+"="+value)
		}
		
		// Create stdio transport with environment
		stdioTransport := transport.NewStdio(serverConfigStruct.Command, envVars, serverConfigStruct.Args...)
		clientTransport = stdioTransport
	} else {
		return fmt.Errorf("invalid MCP server config for %s: missing both URL and Command fields", serverName)
	}

	// Create the MCP client
	mcpClient = client.NewClient(clientTransport)

	// Always close the client after discovery to prevent subprocess leaks
	defer func() {
		if mcpClient != nil {
			mcpClient.Close()
		}
	}()

	// Start the client with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err = mcpClient.Start(ctx)
	if err != nil {
		logging.Debug("Failed to start MCP client for %s: %v", serverName, err)
		return fmt.Errorf("failed to start MCP client for %s: %w", serverName, err)
	}

	// Initialize the MCP client
	logging.Debug("Initializing MCP client for %s", serverName)
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Station MCP Sync",
		Version: "1.0.0",
	}
	initRequest.Params.Capabilities = mcp.ClientCapabilities{}

	_, err = mcpClient.Initialize(ctx, initRequest)
	if err != nil {
		logging.Debug("Failed to initialize MCP client for %s: %v", serverName, err)
		return fmt.Errorf("failed to initialize MCP client for %s: %w", serverName, err)
	}

	// List available tools from the server
	logging.Debug("Fetching available tools from %s", serverName)
	toolsRequest := mcp.ListToolsRequest{}
	toolsResult, err := mcpClient.ListTools(ctx, toolsRequest)
	if err != nil {
		logging.Debug("Failed to list tools from %s: %v", serverName, err)
		return fmt.Errorf("failed to list tools from %s: %w", serverName, err)
	}

	logging.Debug("Discovered %d real tools from %s", len(toolsResult.Tools), serverName)
	
	// Store each real tool in the database with its actual name from the server
	for i, tool := range toolsResult.Tools {
		logging.Debug("  Tool %d: %s - %s", i+1, tool.Name, tool.Description)
		
		// Create database record for this real tool
		mcpTool := &models.MCPTool{
			MCPServerID: serverID,
			Name:        "__" + tool.Name,                          // Add double underscore to match GenKit runtime
			Description: tool.Description,                          // Use actual description from server
		}
		
		_, err = s.repos.MCPTools.Create(mcpTool)
		if err != nil {
			logging.Debug("Failed to store tool %s: %v", tool.Name, err)
			continue
		}
	}
	
	logging.Debug("Completed real tool discovery for server: %s", serverName)
	return nil
}

