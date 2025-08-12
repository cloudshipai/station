package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/pkg/models"
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

	fmt.Printf("Starting declarative sync for environment: %s\n", environmentName)

	// 1. Validate environment exists in database
	_, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' not found: %w", environmentName, err)
	}

	// 2. Determine paths for this environment  
	envDir := filepath.Join("environments", environmentName)
	agentsDir := filepath.Join(envDir, "agents")

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

	// 4. Sync MCP template files (JSON files with potential variables)
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

	result.Duration = time.Since(startTime)
	
	fmt.Printf("Completed sync for environment %s: %d agents processed, %d errors\n", 
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
		fmt.Printf("Debug: Agents directory does not exist: %s\n", agentsDir)
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
	// 1. Basic file validation
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("prompt file not found: %w", err)
	}

	// 3. Calculate file checksum
	checksum, err := s.calculateFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// 4. If dry-run, just report what would be done
	if options.DryRun {
		return &SyncOperation{
			Type:        OpTypeCreate,
			Target:      agentName,
			Description: "Would sync agent from .prompt file",
		}, nil
	}

	// 5. For now, just report successful validation
	fmt.Printf("Info: Validated agent file: %s (checksum: %s)\n", agentName, checksum[:8])

	return &SyncOperation{
		Type:        OpTypeSkip,
		Target:      agentName,
		Description: "Agent validated successfully",
	}, nil
}

// validateMCPDependencies validates that all MCP dependencies are available
func (s *DeclarativeSync) validateMCPDependencies(environmentName string) error {
	// For now, skip complex validation to avoid circular imports
	// TODO: Implement proper MCP dependency validation
	fmt.Printf("Debug: Skipping MCP dependency validation for environment: %s\n", environmentName)
	return nil
}

// createAgentFromFile creates a new agent in the database from a .prompt file
func (s *DeclarativeSync) createAgentFromFile(ctx context.Context, filePath, environmentName, checksum string) error {
	// TODO: Implement agent creation once SQLC is working
	fmt.Printf("Info: Would create agent from file '%s' in environment '%s'\n", filePath, environmentName)
	return nil
}

// updateAgentFromFile updates an existing agent in the database from a .prompt file
func (s *DeclarativeSync) updateAgentFromFile(ctx context.Context, agentName, environmentName, checksum string) error {
	// TODO: Implement agent update once SQLC is working
	fmt.Printf("Info: Would update agent '%s' in environment '%s'\n", agentName, environmentName)
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

	// Process each JSON template file
	for _, jsonFile := range jsonFiles {
		configName := strings.TrimSuffix(filepath.Base(jsonFile), ".json")
		
		fmt.Printf("Processing MCP template: %s\n", configName)
		
		// Read the template file
		templateContent, err := os.ReadFile(jsonFile)
		if err != nil {
			fmt.Printf("Warning: Failed to read template file %s: %v\n", jsonFile, err)
			result.ValidationErrors++
			continue
		}

		// Get environment from database
		env, err := s.repos.Environments.GetByName(environmentName)
		if err != nil {
			fmt.Printf("Warning: Failed to get environment %s: %v\n", environmentName, err)
			result.ValidationErrors++
			continue
		}

		// Process template with variables using the fixed template service
		configDir := filepath.Dir(filepath.Dir(envDir)) // Go up to station config dir
		templateService := NewTemplateVariableService(configDir, s.repos)
		
		templateResult, err := templateService.ProcessTemplateWithVariables(env.ID, configName, string(templateContent), false)
		if err != nil {
			fmt.Printf("Warning: Failed to process template variables for %s: %v\n", configName, err)
			result.ValidationErrors++
			continue
		}

		// Parse the rendered JSON to extract MCP server configurations
		var mcpConfig map[string]interface{}
		if err := json.Unmarshal([]byte(templateResult.RenderedContent), &mcpConfig); err != nil {
			fmt.Printf("Warning: Failed to parse rendered template %s: %v\n", configName, err)
			result.ValidationErrors++
			continue
		}

		// Extract and sync MCP servers from the template
		if err := s.syncMCPServersFromTemplate(ctx, mcpConfig, env.ID, configName, options); err != nil {
			fmt.Printf("Warning: Failed to sync MCP servers from template %s: %v\n", configName, err)
			result.ValidationErrors++
			continue
		}

		result.MCPServersConnected++
		fmt.Printf("Successfully synced template: %s\n", configName)
	}

	return result, nil
}

// syncMCPServersFromTemplate extracts MCP servers from a rendered template and updates the database
func (s *DeclarativeSync) syncMCPServersFromTemplate(ctx context.Context, mcpConfig map[string]interface{}, envID int64, configName string, options SyncOptions) error {
	// Extract MCP servers section
	var serversData map[string]interface{}
	if mcpServers, ok := mcpConfig["mcpServers"].(map[string]interface{}); ok {
		serversData = mcpServers
	} else if servers, ok := mcpConfig["servers"].(map[string]interface{}); ok {
		serversData = servers
	} else {
		// No MCP servers in this template - that's OK
		return nil
	}

	// Process each server configuration
	for serverName, serverConfigRaw := range serversData {
		if options.DryRun {
			fmt.Printf("  [DRY RUN] Would sync MCP server: %s\n", serverName)
			continue
		}

		// Convert server config to proper format
		serverConfigBytes, err := json.Marshal(serverConfigRaw)
		if err != nil {
			return fmt.Errorf("failed to marshal server config for %s: %w", serverName, err)
		}
		
		var serverConfig map[string]interface{}
		if err := json.Unmarshal(serverConfigBytes, &serverConfig); err != nil {
			return fmt.Errorf("failed to unmarshal server config for %s: %w", serverName, err)
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

		// Check if server already exists
		existingServer, err := s.repos.MCPServers.GetByNameAndEnvironment(serverName, envID)
		if err != nil {
			// Server doesn't exist, create new one
			newServer := &models.MCPServer{
				Name:          serverName,
				Command:       command,
				Args:          args,
				Env:           env,
				EnvironmentID: envID,
			}
			_, err = s.repos.MCPServers.Create(newServer)
			if err != nil {
				return fmt.Errorf("failed to create MCP server %s: %w", serverName, err)
			}
			fmt.Printf("  Created MCP server: %s\n", serverName)
		} else {
			// Server exists, update it
			existingServer.Command = command
			existingServer.Args = args
			existingServer.Env = env
			
			err = s.repos.MCPServers.Update(existingServer)
			if err != nil {
				return fmt.Errorf("failed to update MCP server %s: %w", serverName, err)
			}
			fmt.Printf("  Updated MCP server: %s with new args: %v\n", serverName, args)
		}
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