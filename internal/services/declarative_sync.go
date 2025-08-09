package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/dotprompt"
)

// DeclarativeSync handles synchronization between file-based configs and database
type DeclarativeSync struct {
	repos  *repositories.Repositories
	config *config.Config
}

// SyncOptions controls sync behavior
type SyncOptions struct {
	DryRun   bool
	Validate bool
	Force    bool
	Verbose  bool
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

// SyncEnvironment synchronizes a specific environment
func (s *DeclarativeSync) SyncEnvironment(ctx context.Context, environmentName string, options SyncOptions) (*SyncResult, error) {
	startTime := time.Now()
	
	result := &SyncResult{
		Environment:        environmentName,
		Operations:         []SyncOperation{},
		ValidationMessages: []string{},
	}

	logging.Info("Starting declarative sync for environment: %s", environmentName)

	// 1. Validate environment exists in database
	env, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' not found: %w", environmentName, err)
	}

	// 2. Determine paths for this environment  
	envDir := filepath.Join("environments", environmentName)
	agentsDir := filepath.Join(envDir, "agents")
	mcpConfigPath := filepath.Join(envDir, "mcp-config.yaml")

	// 3. Sync agents from .prompt files
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

	// 4. Sync MCP configurations (if file exists)
	if _, err := os.Stat(mcpConfigPath); err == nil {
		mcpResult, err := s.syncMCPConfig(ctx, mcpConfigPath, environmentName, options)
		if err != nil {
			logging.Warn("Failed to sync MCP config for %s: %v", environmentName, err)
			result.ValidationErrors++
			result.ValidationMessages = append(result.ValidationMessages, 
				fmt.Sprintf("MCP config sync failed: %v", err))
		} else {
			result.MCPServersProcessed = mcpResult.MCPServersProcessed
			result.MCPServersConnected = mcpResult.MCPServersConnected
			result.Operations = append(result.Operations, mcpResult.Operations...)
		}
	}

	result.Duration = time.Since(startTime)
	
	logging.Info("Completed sync for environment %s: %d agents processed, %d errors", 
		environmentName, result.AgentsProcessed, result.ValidationErrors)

	return result, nil
}

// syncAgents handles synchronization of agent .prompt files
func (s *DeclarativeSync) syncAgents(ctx context.Context, agentsDir, environmentName string, options SyncOptions) (*SyncResult, error) {
	result := &SyncResult{
		Environment:        environmentName,
		Operations:         []SyncOperation{},
		ValidationMessages: []string{},
	}

	// Check if agents directory exists
	if _, err := os.Stat(agentsDir); os.IsNotExist(err) {
		logging.Debug("Agents directory does not exist: %s", agentsDir)
		return result, nil
	}

	// Find all .prompt files
	promptFiles, err := filepath.Glob(filepath.Join(agentsDir, "*.prompt"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan agent files: %w", err)
	}

	result.AgentsProcessed = len(promptFiles)

	// Process each .prompt file
	for _, promptFile := range promptFiles {
		agentName := strings.TrimSuffix(filepath.Base(promptFile), ".prompt")
		
		operation, err := s.syncSingleAgent(ctx, promptFile, agentName, environmentName, options)
		if err != nil {
			result.ValidationErrors++
			result.ValidationMessages = append(result.ValidationMessages, 
				fmt.Sprintf("Agent '%s': %v", agentName, err))
			
			result.Operations = append(result.Operations, SyncOperation{
				Type:        OpTypeError,
				Target:      agentName,
				Description: fmt.Sprintf("Failed to sync agent: %v", err),
				Error:       err,
			})
			continue
		}

		result.Operations = append(result.Operations, *operation)
		
		switch operation.Type {
		case OpTypeCreate, OpTypeUpdate:
			result.AgentsSynced++
		case OpTypeSkip:
			result.AgentsSkipped++
		}
	}

	return result, nil
}

// syncSingleAgent synchronizes a single agent .prompt file
func (s *DeclarativeSync) syncSingleAgent(ctx context.Context, filePath, agentName, environmentName string, options SyncOptions) (*SyncOperation, error) {
	// 1. Parse the .prompt file
	extractor, err := dotprompt.NewRuntimeExtraction(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse .prompt file: %w", err)
	}

	config := extractor.GetConfig()

	// 2. Validate agent name matches filename
	if config.Metadata.Name != agentName {
		return nil, fmt.Errorf("agent name '%s' in file doesn't match filename '%s'", 
			config.Metadata.Name, agentName)
	}

	// 3. Validate MCP dependencies
	if err := s.validateMCPDependencies(extractor, environmentName); err != nil {
		return nil, fmt.Errorf("MCP dependency validation failed: %w", err)
	}

	// 4. Calculate file checksum
	checksum, err := s.calculateFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// 5. Check if agent exists in database
	existingAgent, err := s.repos.Agents.GetByName(agentName, environmentName)
	needsUpdate := false
	isNew := false

	if err != nil {
		// Agent doesn't exist - need to create
		isNew = true
		needsUpdate = true
	} else {
		// Agent exists - check if file changed
		if !options.Force && existingAgent.ChecksumMD5 == checksum {
			// No changes needed
			return &SyncOperation{
				Type:        OpTypeSkip,
				Target:      agentName,
				Description: fmt.Sprintf("No changes (checksum match)"),
			}, nil
		}
		needsUpdate = true
	}

	// 6. If dry-run, just report what would be done
	if options.DryRun {
		opType := OpTypeUpdate
		description := "Would update agent"
		if isNew {
			opType = OpTypeCreate
			description = "Would create new agent"
		}
		
		return &SyncOperation{
			Type:        opType,
			Target:      agentName,
			Description: description,
		}, nil
	}

	// 7. Perform database update
	if needsUpdate {
		if isNew {
			// Create new agent
			err = s.createAgentFromFile(ctx, extractor, filePath, environmentName, checksum)
			if err != nil {
				return nil, fmt.Errorf("failed to create agent: %w", err)
			}

			return &SyncOperation{
				Type:        OpTypeCreate,
				Target:      agentName,
				Description: "Created new agent from .prompt file",
			}, nil
		} else {
			// Update existing agent
			err = s.updateAgentFromFile(ctx, extractor, agentName, environmentName, checksum)
			if err != nil {
				return nil, fmt.Errorf("failed to update agent: %w", err)
			}

			return &SyncOperation{
				Type:        OpTypeUpdate,
				Target:      agentName,
				Description: "Updated agent from .prompt file",
			}, nil
		}
	}

	return &SyncOperation{
		Type:        OpTypeSkip,
		Target:      agentName,
		Description: "No changes needed",
	}, nil
}

// validateMCPDependencies validates that all MCP dependencies are available
func (s *DeclarativeSync) validateMCPDependencies(extractor *dotprompt.RuntimeExtraction, environmentName string) error {
	// Extract MCP dependencies from frontmatter
	mcpDeps, err := extractor.ExtractCustomField("station.mcp_dependencies")
	if err != nil || mcpDeps == nil {
		// No dependencies defined - this is OK
		return nil
	}

	depsMap, ok := mcpDeps.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid MCP dependencies format - expected map")
	}

	// Validate each MCP dependency
	for mcpConfigName, depData := range depsMap {
		depMap, ok := depData.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid dependency format for MCP config '%s'", mcpConfigName)
		}

		// Get assigned tools
		assignedToolsInterface, exists := depMap["assigned_tools"]
		if !exists {
			continue // No tools assigned
		}

		assignedToolsList, ok := assignedToolsInterface.([]interface{})
		if !ok {
			return fmt.Errorf("assigned_tools must be a list for MCP config '%s'", mcpConfigName)
		}

		// Convert to string slice
		assignedTools := make([]string, len(assignedToolsList))
		for i, toolInterface := range assignedToolsList {
			toolName, ok := toolInterface.(string)
			if !ok {
				return fmt.Errorf("tool name must be string in MCP config '%s'", mcpConfigName)
			}
			assignedTools[i] = toolName
		}

		// Validate tools exist (this would need to check against actual MCP servers)
		// For now, we'll do basic validation that the config is well-formed
		logging.Debug("Validated MCP config '%s' with %d assigned tools", mcpConfigName, len(assignedTools))
	}

	return nil
}

// createAgentFromFile creates a new agent in the database from a .prompt file
func (s *DeclarativeSync) createAgentFromFile(ctx context.Context, extractor *dotprompt.RuntimeExtraction, filePath, environmentName, checksum string) error {
	config := extractor.GetConfig()
	
	// This would use the new SQLC-generated methods once available
	// For now, using a placeholder approach
	logging.Info("Creating agent '%s' in environment '%s'", config.Metadata.Name, environmentName)
	
	// TODO: Replace with actual SQLC CreateAgent call
	// agent, err := s.repos.Agents.CreateAgent(ctx, db.CreateAgentParams{
	//     Name:            config.Metadata.Name,
	//     DisplayName:     config.Metadata.Name,
	//     Description:     config.Metadata.Description,
	//     FilePath:        filePath,
	//     EnvironmentName: environmentName,
	//     ChecksumMd5:     checksum,
	// })
	
	return nil
}

// updateAgentFromFile updates an existing agent in the database from a .prompt file
func (s *DeclarativeSync) updateAgentFromFile(ctx context.Context, extractor *dotprompt.RuntimeExtraction, agentName, environmentName, checksum string) error {
	config := extractor.GetConfig()
	
	logging.Info("Updating agent '%s' in environment '%s'", agentName, environmentName)
	
	// TODO: Replace with actual SQLC UpdateAgentFromFile call
	// err := s.repos.Agents.UpdateAgentFromFile(ctx, db.UpdateAgentFromFileParams{
	//     DisplayName:     config.Metadata.Name,
	//     Description:     config.Metadata.Description,
	//     ChecksumMd5:     checksum,
	//     Name:            agentName,
	//     EnvironmentName: environmentName,
	// })
	
	return nil
}

// syncMCPConfig handles MCP configuration synchronization
func (s *DeclarativeSync) syncMCPConfig(ctx context.Context, configPath, environmentName string, options SyncOptions) (*SyncResult, error) {
	result := &SyncResult{
		Environment: environmentName,
		Operations:  []SyncOperation{},
	}

	logging.Debug("Syncing MCP config from: %s", configPath)

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

// calculateFileChecksum calculates MD5 checksum of a file
func (s *DeclarativeSync) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
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

// Placeholder methods for database operations
func (repos *repositories.Repositories) GetByName(name, environment string) (*AgentRecord, error) {
	// TODO: Implement with SQLC generated code
	return nil, fmt.Errorf("agent not found") // Placeholder
}

func (repos *repositories.Repositories) CreateAgent(ctx context.Context, agent *AgentRecord) error {
	// TODO: Implement with SQLC generated code
	return nil
}

func (repos *repositories.Repositories) UpdateAgent(ctx context.Context, agent *AgentRecord) error {
	// TODO: Implement with SQLC generated code
	return nil
}