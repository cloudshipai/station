package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"
)

// DeclarativeSync handles synchronization between file-based configs and database
type DeclarativeSync struct {
	repos                   *repositories.Repositories
	config                  *config.Config
	customVariableResolver  VariableResolver // Custom resolver for UI integration
}

// SyncOptions controls sync behavior
type SyncOptions struct {
	DryRun      bool
	Validate    bool
	Force       bool
	Verbose     bool
	Interactive bool
	Confirm     bool
}

// SyncResult contains results of a sync operation
type SyncResult struct {
	Environment             string
	AgentsProcessed         int
	AgentsSynced            int
	AgentsSkipped           int
	ValidationErrors        int
	ValidationMessages      []string
	MCPServersProcessed     int
	MCPServersConnected     int
	Operations              []SyncOperation
	Duration                time.Duration
}

// SyncOperation represents a single sync operation
type SyncOperation struct {
	Type        SyncOperationType
	Target      string // agent name, mcp config, etc.
	Description string
	Error       error
}

// SyncOperationType represents the type of sync operation
type SyncOperationType string

const (
	OpTypeCreate   SyncOperationType = "create"
	OpTypeUpdate   SyncOperationType = "update"
	OpTypeDelete   SyncOperationType = "delete"
	OpTypeSkip     SyncOperationType = "skip"
	OpTypeValidate SyncOperationType = "validate"
	OpTypeError    SyncOperationType = "error"
)

// NewDeclarativeSync creates a new declarative sync service
func NewDeclarativeSync(repos *repositories.Repositories, config *config.Config) *DeclarativeSync {
	return &DeclarativeSync{
		repos:  repos,
		config: config,
	}
}

// SetVariableResolver sets a custom variable resolver for UI integration
func (s *DeclarativeSync) SetVariableResolver(resolver VariableResolver) {
	s.customVariableResolver = resolver
}

// SyncEnvironment synchronizes a specific environment
func (s *DeclarativeSync) SyncEnvironment(ctx context.Context, environmentName string, options SyncOptions) (*SyncResult, error) {
	startTime := time.Now()
	
	result := &SyncResult{
		Environment:        environmentName,
		Operations:         []SyncOperation{},
		ValidationMessages: []string{},
	}

	fmt.Printf("Starting declarative sync for environment: %s\n", environmentName)

	// 1. Validate environment exists in database
	_, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' not found: %w", environmentName, err)
	}

	// 2. Determine paths for this environment
	// Get the workspace directory from config (e.g., /Users/jaredwolff/.config/station)
	var workspaceDir string
	if s.config.Workspace != "" {
		workspaceDir = s.config.Workspace
	} else {
		// Fallback to XDG config directory  
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			homeDir, _ := os.UserHomeDir()
			configHome = filepath.Join(homeDir, ".config")
		}
		workspaceDir = filepath.Join(configHome, "station")
	}
	
	envDir := filepath.Join(workspaceDir, "environments", environmentName)
	agentsDir := filepath.Join(envDir, "agents")

	// 3. Sync MCP template files FIRST (JSON files with potential variables)
	// This must happen before agent sync so tools have stable IDs
	mcpResult, err := s.syncMCPTemplateFiles(ctx, envDir, environmentName, options)
	if err != nil {
		fmt.Printf("Warning: Failed to sync MCP templates for %s: %v\n", environmentName, err)
		result.ValidationErrors++
		result.ValidationMessages = append(result.ValidationMessages, 
			fmt.Sprintf("MCP template sync failed: %v", err))
	} else {
		result.MCPServersProcessed = mcpResult.MCPServersProcessed
		result.MCPServersConnected = mcpResult.MCPServersConnected
		result.Operations = append(result.Operations, mcpResult.Operations...)
	}

	// 4. Sync agents from .prompt files AFTER MCP tools are stable
	agentResult, err := s.syncAgents(ctx, agentsDir, environmentName, options)
	if err != nil {
		return nil, fmt.Errorf("failed to sync agents: %w", err)
	}

	// Merge agent results
	result.AgentsProcessed = agentResult.AgentsProcessed
	result.AgentsSynced = agentResult.AgentsSynced
	result.AgentsSkipped = agentResult.AgentsSkipped
	result.ValidationErrors += agentResult.ValidationErrors
	result.ValidationMessages = append(result.ValidationMessages, agentResult.ValidationMessages...)
	result.Operations = append(result.Operations, agentResult.Operations...)

	// 5. Cleanup orphaned configs, servers, and tools (declarative sync)
	cleanupResult, err := s.cleanupOrphanedResources(ctx, envDir, environmentName, options)
	if err != nil {
		fmt.Printf("Warning: Failed to cleanup orphaned resources for %s: %v\n", environmentName, err)
		result.ValidationErrors++
		result.ValidationMessages = append(result.ValidationMessages, 
			fmt.Sprintf("Cleanup failed: %v", err))
	} else {
		fmt.Printf("🧹 Cleanup completed: %s\n", cleanupResult)
	}

	result.Duration = time.Since(startTime)
	
	fmt.Printf("Completed sync for environment %s: %d agents processed, %d errors\n", 
		environmentName, result.AgentsProcessed, result.ValidationErrors)

	return result, nil
}

// validateMCPDependencies validates that all MCP dependencies are available
func (s *DeclarativeSync) validateMCPDependencies(environmentName string) error {
	// For now, skip complex validation to avoid circular imports
	// TODO: Implement proper MCP dependency validation
	fmt.Printf("Debug: Skipping MCP dependency validation for environment: %s\n", environmentName)
	return nil
}

// syncMCPConfig handles MCP configuration synchronization
func (s *DeclarativeSync) syncMCPConfig(ctx context.Context, configPath, environmentName string, options SyncOptions) (*SyncResult, error) {
	result := &SyncResult{
		Environment: environmentName,
		Operations:  []SyncOperation{},
	}

	fmt.Printf("Debug: Syncing MCP config from: %s\n", configPath)

	// TODO: Implement MCP config parsing and synchronization
	// This would parse the mcp-config.yaml file and update MCP servers in the database

	result.MCPServersProcessed = 1 // Placeholder
	result.Operations = append(result.Operations, SyncOperation{
		Type:        OpTypeSkip,
		Target:      "mcp-config",
		Description: "MCP config sync not yet implemented",
	})

	return result, nil
}

// syncMCPTemplateFiles processes individual JSON template files in the environment directory
func (s *DeclarativeSync) syncMCPTemplateFiles(ctx context.Context, envDir, environmentName string, options SyncOptions) (*SyncResult, error) {
	result := &SyncResult{
		Environment:        environmentName,
		Operations:         []SyncOperation{},
		ValidationMessages: []string{},
	}

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		fmt.Printf("Debug: Environment directory does not exist: %s\n", envDir)
		return result, nil
	}

	// Find all .json files (excluding agent .prompt files)
	jsonFiles, err := filepath.Glob(filepath.Join(envDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan JSON template files: %w", err)
	}

	result.MCPServersProcessed = len(jsonFiles)

	// Create template service once for this environment and reuse it
	// Recalculate workspace directory to ensure it's in scope
	var templateWorkspaceDir string
	if s.config.Workspace != "" {
		templateWorkspaceDir = s.config.Workspace
	} else {
		// Fallback to XDG config directory  
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			homeDir, _ := os.UserHomeDir()
			configHome = filepath.Join(homeDir, ".config")
		}
		templateWorkspaceDir = filepath.Join(configHome, "station")
	}
	templateService := NewTemplateVariableService(templateWorkspaceDir, s.repos)
	// Inject custom variable resolver if available
	if s.customVariableResolver != nil {
		templateService.SetVariableResolver(s.customVariableResolver)
	}

	// Process JSON template files in parallel for faster MCP server validation
	if len(jsonFiles) > 0 {
		parallelResult, err := s.processJSONTemplatesParallel(ctx, jsonFiles, environmentName, templateService, options)
		if err != nil {
			return nil, fmt.Errorf("parallel template processing failed: %w", err)
		}
		
		// Merge parallel results
		result.ValidationErrors += parallelResult.ValidationErrors
		result.MCPServersConnected += parallelResult.MCPServersConnected
		result.ValidationMessages = append(result.ValidationMessages, parallelResult.ValidationMessages...)
		result.Operations = append(result.Operations, parallelResult.Operations...)
	}

	return result, nil
}

// processJSONTemplatesParallel processes multiple JSON templates in parallel
func (s *DeclarativeSync) processJSONTemplatesParallel(ctx context.Context, jsonFiles []string, environmentName string, templateService *TemplateVariableService, options SyncOptions) (*SyncResult, error) {
	// Create worker pool with configurable concurrency
	maxWorkers := getEnvIntOrDefault("STATION_SYNC_TEMPLATE_WORKERS", 3) // Default: 3 workers
	if len(jsonFiles) < maxWorkers {
		maxWorkers = len(jsonFiles)
	}
	
	fmt.Printf("Processing %d MCP templates in parallel with %d workers\n", len(jsonFiles), maxWorkers)
	
	// Channel to send template jobs to workers
	type templateJob struct {
		jsonFile   string
		configName string
	}
	jobChan := make(chan templateJob, len(jsonFiles))
	
	// Channel to collect results (reuse defined templateResult type)
	resultChan := make(chan templateResult, len(jsonFiles))
	
	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobChan {
				fmt.Printf("Worker %d processing template: %s\n", workerID, job.configName)
				result := s.processTemplateJob(ctx, job.jsonFile, job.configName, environmentName, templateService, options)
				resultChan <- result
			}
		}(i)
	}
	
	// Send all template jobs to workers
	for _, jsonFile := range jsonFiles {
		configName := filepath.Base(jsonFile)
		configName = configName[:len(configName)-len(filepath.Ext(configName))] // Remove extension
		jobChan <- templateJob{
			jsonFile:   jsonFile,
			configName: configName,
		}
	}
	close(jobChan)
	
	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	var aggregatedResult = &SyncResult{
		Environment:        environmentName,
		Operations:         []SyncOperation{},
		ValidationMessages: []string{},
	}
	successCount := 0
	
	for result := range resultChan {
		if result.error != nil {
			fmt.Printf("Template %s processing failed: %v\n", result.configName, result.error)
			aggregatedResult.ValidationErrors++
			aggregatedResult.ValidationMessages = append(aggregatedResult.ValidationMessages, 
				fmt.Sprintf("Template %s: %v", result.configName, result.error))
		} else {
			successCount++
			fmt.Printf("✅ Template %s processed: %d MCP servers\n", result.configName, result.mcpServersCount)
		}
		
		// Aggregate results
		aggregatedResult.ValidationErrors += result.validationErrors
		aggregatedResult.MCPServersConnected += result.mcpServersCount
		aggregatedResult.ValidationMessages = append(aggregatedResult.ValidationMessages, result.validationMessages...)
		aggregatedResult.Operations = append(aggregatedResult.Operations, result.operations...)
	}
	
	fmt.Printf("Parallel template processing completed: %d templates, %d successful\n", 
		len(jsonFiles), successCount)
	
	return aggregatedResult, nil
}

// templateResult holds the result of processing a single template
type templateResult struct {
	configName         string
	validationErrors   int
	validationMessages []string
	mcpServersCount    int
	operations         []SyncOperation
	error              error
}

// processTemplateJob processes a single template file job
func (s *DeclarativeSync) processTemplateJob(ctx context.Context, jsonFile, configName, environmentName string, templateService *TemplateVariableService, options SyncOptions) templateResult {
	result := templateResult{
		configName:         configName,
		validationMessages: []string{},
		operations:         []SyncOperation{},
	}
	
	// Read the template file
	templateContent, err := os.ReadFile(jsonFile)
	if err != nil {
		result.error = fmt.Errorf("failed to read template file: %w", err)
		return result
	}

	// Get environment from database
	env, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		result.error = fmt.Errorf("failed to get environment: %w", err)
		return result
	}

	// Process template with variables using the shared template service
	templateResult, err := templateService.ProcessTemplateWithVariables(env.ID, configName, string(templateContent), options.Interactive)
	if err != nil {
		result.error = fmt.Errorf("failed to process template variables: %w", err)
		return result
	}

	// Parse the rendered JSON to extract MCP server configurations
	var mcpConfig map[string]interface{}
	if err := json.Unmarshal([]byte(templateResult.RenderedContent), &mcpConfig); err != nil {
		result.error = fmt.Errorf("failed to parse rendered template: %w", err)
		return result
	}

	// Calculate environment directory for file config registration
	var workspaceDir string
	if s.config.Workspace != "" {
		workspaceDir = s.config.Workspace
	} else {
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			homeDir, _ := os.UserHomeDir()
			configHome = filepath.Join(homeDir, ".config")
		}
		workspaceDir = filepath.Join(configHome, "station")
	}
	envDir := filepath.Join(workspaceDir, "environments", environmentName)

	// 1. Register/update the file config in database
	err = s.registerOrUpdateFileConfig(ctx, env.ID, configName, jsonFile, envDir, templateResult, options)
	if err != nil {
		result.error = fmt.Errorf("failed to register file config: %w", err)
		return result
	}

	// 2. Extract and sync MCP servers from the template
	serversCount, err := s.syncMCPServersFromTemplate(ctx, mcpConfig, env.ID, configName, options)
	if err != nil {
		result.error = fmt.Errorf("failed to sync MCP servers: %w", err)
		return result
	}

	// 3. Perform tool discovery for the servers that were created/updated
	if serversCount > 0 && !options.DryRun {
		toolsDiscovered, err := s.performToolDiscovery(ctx, env.ID, configName)
		if err != nil {
			// Don't treat tool discovery failure as fatal - servers are still synced
			result.validationMessages = append(result.validationMessages, 
				fmt.Sprintf("Template %s: Tool discovery failed - %v", configName, err))
		} else {
			fmt.Printf("   🔧 Discovered %d tools from MCP servers\n", toolsDiscovered)
		}
		result.mcpServersCount = serversCount
	} else if serversCount > 0 {
		result.mcpServersCount = serversCount
	}

	result.operations = append(result.operations, SyncOperation{
		Type:        OpTypeValidate,
		Target:      configName,
		Description: fmt.Sprintf("Template %s processed with %d MCP servers", configName, serversCount),
	})

	return result
}


// syncMCPServersFromTemplate extracts MCP servers from a rendered template and updates the database
// Returns the number of servers successfully synced
func (s *DeclarativeSync) syncMCPServersFromTemplate(ctx context.Context, mcpConfig map[string]interface{}, envID int64, configName string, options SyncOptions) (int, error) {
	// Extract MCP servers section
	var serversData map[string]interface{}
	if mcpServers, ok := mcpConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := mcpConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		// No MCP servers in this template - that's OK (config-only file)
		return 0, nil
	}

	logging.Info("    Processing %d MCP servers from config...\n", len(serversData))
	successCount := 0
	
	// Process each server configuration
	for serverName, serverConfigRaw := range serversData {
		if options.DryRun {
			fmt.Printf("  [DRY RUN] Would sync MCP server: %s\n", serverName)
			continue
		}

		logging.Info("       Processing server: %s\n", serverName)
		
		// Convert server config to proper format
		serverConfigBytes, err := json.Marshal(serverConfigRaw)
		if err != nil {
			fmt.Printf("     ❌ Failed to marshal config for server %s: %v\n", serverName, err)
			fmt.Printf("        Raw config: %+v\n", serverConfigRaw)
			return successCount, fmt.Errorf("failed to marshal server config for %s: %w", serverName, err)
		}
		
		var serverConfig map[string]interface{}
		if err := json.Unmarshal(serverConfigBytes, &serverConfig); err != nil {
			fmt.Printf("     ❌ Failed to unmarshal config for server %s: %v\n", serverName, err)
			fmt.Printf("        JSON: %s\n", string(serverConfigBytes))
			return successCount, fmt.Errorf("failed to unmarshal server config for %s: %w", serverName, err)
		}

		// Extract server properties
		command, _ := serverConfig["command"].(string)
		args := []string{}
		if argsRaw, ok := serverConfig["args"].([]interface{}); ok {
			for _, arg := range argsRaw {
				if argStr, ok := arg.(string); ok {
					args = append(args, argStr)
				}
			}
		}

		env := map[string]string{}
		if envRaw, ok := serverConfig["env"].(map[string]interface{}); ok {
			for key, value := range envRaw {
				if valueStr, ok := value.(string); ok {
					env[key] = valueStr
				}
			}
		}

		// Validate required server config
		if command == "" {
			fmt.Printf("     ❌ Server %s missing required 'command' field\n", serverName)
			fmt.Printf("        Config: %+v\n", serverConfig)
			return successCount, fmt.Errorf("server %s missing required 'command' field", serverName)
		}
		
		fmt.Printf("        Command: %s %v\n", command, args)
		if len(env) > 0 {
			fmt.Printf("        Environment: %+v\n", env)
		}

		// Get the file config ID for proper cascade deletion
		fileConfig, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
		if err != nil {
			return successCount, fmt.Errorf("failed to get file config for %s: %w", configName, err)
		}

		// Check if server already exists
		existingServer, err := s.repos.MCPServers.GetByNameAndEnvironment(serverName, envID)
		if err != nil {
			// Server doesn't exist, create new one
			logging.Info("      Creating new MCP server: %s\n", serverName)
			newServer := &models.MCPServer{
				Name:          serverName,
				Command:       command,
				Args:          args,
				Env:           env,
				EnvironmentID: envID,
				FileConfigID:  &fileConfig.ID,
			}
			_, err = s.repos.MCPServers.Create(newServer)
			if err != nil {
				fmt.Printf("     ❌ DATABASE ERROR: Failed to create server %s: %v\n", serverName, err)
				fmt.Printf("        This server will NOT be available for tool discovery\n")
				return successCount, fmt.Errorf("failed to create MCP server %s: %w", serverName, err)
			}
			fmt.Printf("     ✅ Created MCP server: %s\n", serverName)
		} else {
			// Server exists, update it
			fmt.Printf("     🔄 Updating existing MCP server: %s\n", serverName)
			existingServer.Command = command
			existingServer.Args = args
			existingServer.Env = env
			existingServer.FileConfigID = &fileConfig.ID
			
			err = s.repos.MCPServers.Update(existingServer)
			if err != nil {
				fmt.Printf("     ❌ DATABASE ERROR: Failed to update server %s: %v\n", serverName, err)
				fmt.Printf("        Server config changes will NOT be reflected\n")
				return successCount, fmt.Errorf("failed to update MCP server %s: %w", serverName, err)
			}
			fmt.Printf("     ✅ Updated MCP server: %s\n", serverName)
		}
		
		successCount++
	}

	fmt.Printf("   ✅ Successfully synced %d/%d MCP servers from template\n", successCount, len(serversData))
	return successCount, nil
}

// registerOrUpdateFileConfig registers or updates a file config in the database
func (s *DeclarativeSync) registerOrUpdateFileConfig(ctx context.Context, envID int64, configName, jsonFile, envDir string, templateResult *VariableResolutionResult, options SyncOptions) error {
	// Calculate relative path from workspace
	var workspaceDir string
	if s.config.Workspace != "" {
		workspaceDir = s.config.Workspace
	} else {
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			homeDir, _ := os.UserHomeDir()
			configHome = filepath.Join(homeDir, ".config")
		}
		workspaceDir = filepath.Join(configHome, "station")
	}
	
	// Make template path relative to workspace
	relativePath, err := filepath.Rel(workspaceDir, jsonFile)
	if err != nil {
		// Fallback to absolute path if relative calculation fails
		relativePath = jsonFile
	}
	
	// Check if file config already exists
	existingConfig, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
	if err != nil {
		// Config doesn't exist, create new one
		fileConfig := &repositories.FileConfigRecord{
			EnvironmentID:            envID,
			ConfigName:               configName,
			TemplatePath:             relativePath,
			VariablesPath:            "variables.yml", // Standard variables file
			TemplateSpecificVarsPath: "",
			LastLoadedAt:             &time.Time{}, // Set to current time
			TemplateHash:             "", // Will be calculated if needed
			VariablesHash:            "",
			TemplateVarsHash:         "",
			Metadata:                 "{}",
		}
		now := time.Now()
		fileConfig.LastLoadedAt = &now
		
		_, err = s.repos.FileMCPConfigs.Create(fileConfig)
		if err != nil {
			return fmt.Errorf("failed to create file config record: %w", err)
		}
		logging.Info("    Registered new file config: %s\n", configName)
	} else {
		// Config exists, update it
		err = s.repos.FileMCPConfigs.UpdateLastLoadedAt(existingConfig.ID)
		if err != nil {
			return fmt.Errorf("failed to update file config timestamp: %w", err)
		}
		logging.Info("    Updated file config: %s\n", configName)
	}
	
	return nil
}

// Helper type for database operations (until SQLC is working)
type AgentRecord struct {
	Name            string
	DisplayName     string
	Description     string
	FilePath        string
	EnvironmentName string
	ChecksumMD5     string
}

