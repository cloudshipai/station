package services

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"station/internal/logging"
	"station/pkg/models"
	
	"gopkg.in/yaml.v2"
)

// DotPromptConfig represents the YAML frontmatter in a .prompt file
type DotPromptConfig struct {
	Model       string                 `yaml:"model"`
	Config      map[string]interface{} `yaml:"config"`
	Tools       []string               `yaml:"tools"`
	Metadata    map[string]interface{} `yaml:"metadata"`
	Station     map[string]interface{} `yaml:"station"`
	Input       map[string]interface{} `yaml:"input"`
	Output      map[string]interface{} `yaml:"output"`
}

// extractInputSchema extracts and validates input schema from dotprompt config using Picoschema parsing
func (s *DeclarativeSync) extractInputSchema(config *DotPromptConfig) (*string, error) {
	if config.Input == nil || config.Input["schema"] == nil {
		return nil, nil
	}
	
	// Get the raw schema map (Picoschema format)
	schemaData, exists := config.Input["schema"]
	if !exists {
		return nil, nil
	}
	
	schemaMap, ok := schemaData.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("input.schema must be a map")
	}
	
	// TODO: Update to use JSON Schema format instead of custom InputVariable format
	// For now, we'll skip custom schema parsing during sync
	_ = schemaMap // Suppress unused variable warning
	
	for key, value := range schemaMap {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}
		
		// Skip userInput as it's automatically provided
		if keyStr == "userInput" {
			continue
		}
		
		// Parse Picoschema format - value is a string like "string, test description"
		// Schema parsing disabled during JSON Schema migration
		_ = s.parsePicoschemaField(keyStr, value)
	}
	
	// Schema sync disabled during JSON Schema migration
	return nil, fmt.Errorf("custom schema sync temporarily disabled - needs JSON Schema format update")
}

// parsePicoschemaField parses a single Picoschema field definition
// TODO: Update to use JSON Schema format
func (s *DeclarativeSync) parsePicoschemaField(fieldName string, value interface{}) interface{} {
	// Temporarily disabled - needs update to JSON Schema format
	_ = fieldName
	_ = value
	return nil
}

// parsePicoschemaString parses Picoschema string definitions
func (s *DeclarativeSync) parsePicoschemaString(fieldName string, definition string) interface{} {
	// TODO: Update to JSON Schema format
	_ = fieldName
	_ = definition
	return nil
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
		logging.Info("Debug: Agents directory does not exist: %s", agentsDir)
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

	// Cleanup orphaned agents (declarative: filesystem is source of truth)
	if !options.DryRun {
		orphanedCount, err := s.cleanupOrphanedAgents(ctx, agentsDir, environmentName, promptFiles)
		if err != nil {
			logging.Info("Warning: Failed to cleanup orphaned agents: %v", err)
		} else if orphanedCount > 0 {
			logging.Info("🧹 Removed %d orphaned agent(s) from database", orphanedCount)
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

	// 2. Read and parse .prompt file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read prompt file: %w", err)
	}

	config, promptContent, err := s.parseDotPrompt(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt file: %w", err)
	}

	// 3. Calculate file checksum
	checksum, err := s.calculateFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// 4. Get environment
	env, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' not found: %w", environmentName, err)
	}

	// 5. Check if agent already exists in database
	existingAgent, err := s.findAgentByName(agentName, env.ID)
	if err != nil && err.Error() != "agent not found" {
		return nil, fmt.Errorf("failed to check existing agent: %w", err)
	}

	// 6. If dry-run, just report what would be done
	if options.DryRun {
		if existingAgent != nil {
			return &SyncOperation{
				Type:        OpTypeUpdate,
				Target:      agentName,
				Description: "Would update agent from .prompt file",
			}, nil
		}
		return &SyncOperation{
			Type:        OpTypeCreate,
			Target:      agentName,
			Description: "Would create agent from .prompt file",
		}, nil
	}

	// 7. Create or update agent
	if existingAgent != nil {
		// Update existing agent
		return s.updateAgentFromFile(ctx, existingAgent, config, promptContent, checksum)
	} else {
		// Create new agent
		return s.createAgentFromFile(ctx, filePath, agentName, environmentName, config, promptContent, checksum)
	}
}

// parseDotPrompt parses a .prompt file with YAML frontmatter and prompt content
func (s *DeclarativeSync) parseDotPrompt(content string) (*DotPromptConfig, string, error) {
	// Split on the first occurrence of "---" after the initial "---"
	parts := strings.Split(content, "---")
	if len(parts) < 3 {
		// No frontmatter, treat entire content as prompt
		return &DotPromptConfig{}, content, nil
	}

	// Extract YAML frontmatter (first part after initial ---)
	yamlContent := strings.TrimSpace(parts[1])
	
	// Extract prompt content (everything after second ---)
	promptContent := strings.TrimSpace(strings.Join(parts[2:], "---"))

	// Parse YAML frontmatter
	var config DotPromptConfig
	if yamlContent != "" {
		if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
			return nil, "", fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}
	}

	return &config, promptContent, nil
}

// findAgentByName finds an agent by name in the specified environment
func (s *DeclarativeSync) findAgentByName(agentName string, environmentID int64) (*models.Agent, error) {
	agents, err := s.repos.Agents.ListByEnvironment(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	for _, agent := range agents {
		if agent.Name == agentName {
			return agent, nil
		}
	}

	return nil, fmt.Errorf("agent not found")
}

// createAgentFromFile creates a new agent in the database from a .prompt file
func (s *DeclarativeSync) createAgentFromFile(ctx context.Context, filePath, agentName, environmentName string, config *DotPromptConfig, promptContent, checksum string) (*SyncOperation, error) {
	env, err := s.repos.Environments.GetByName(environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' not found: %w", environmentName, err)
	}

	// Extract configuration values with defaults
	maxSteps := int64(5) // default
	if config.Metadata != nil {
		if steps, ok := config.Metadata["max_steps"]; ok {
			switch v := steps.(type) {
			case int:
				maxSteps = int64(v)
			case int64:
				maxSteps = v
			case string:
				if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
					maxSteps = parsed
				}
			}
		}
	}

	description := ""
	if config.Metadata != nil {
		if desc, ok := config.Metadata["description"].(string); ok {
			description = desc
		}
	}

	// Extract input schema from frontmatter
	inputSchema, err := s.extractInputSchema(config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract input schema: %w", err)
	}

	// Create agent using individual parameters
	createdAgent, err := s.repos.Agents.Create(
		agentName,
		description,
		promptContent,
		maxSteps,
		env.ID,
		1, // createdBy - system user
		inputSchema, // input_schema - extracted from frontmatter
		nil, // cronSchedule
		true, // scheduleEnabled
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Assign tools if specified
	if len(config.Tools) > 0 {
		for _, toolName := range config.Tools {
			// Find tool by name in environment
			tool, err := s.repos.MCPTools.FindByNameInEnvironment(env.ID, toolName)
			if err != nil {
				logging.Info("Warning: Tool %s not found in environment: %v", toolName, err)
				continue
			}

			// Assign tool to agent
			_, err = s.repos.AgentTools.AddAgentTool(createdAgent.ID, tool.ID)
			if err != nil {
				logging.Info("Warning: Failed to assign tool %s to agent: %v", toolName, err)
			}
		}
	}

	logging.Info("✅ Created agent: %s", agentName)
	return &SyncOperation{
		Type:        OpTypeCreate,
		Target:      agentName,
		Description: fmt.Sprintf("Created agent from .prompt file"),
	}, nil
}

// updateAgentFromFile updates an existing agent in the database from a .prompt file
func (s *DeclarativeSync) updateAgentFromFile(ctx context.Context, existingAgent *models.Agent, config *DotPromptConfig, promptContent, checksum string) (*SyncOperation, error) {
	// Extract configuration values with defaults
	maxSteps := existingAgent.MaxSteps // keep existing
	if config.Metadata != nil {
		if steps, ok := config.Metadata["max_steps"]; ok {
			switch v := steps.(type) {
			case int:
				maxSteps = int64(v)
			case int64:
				maxSteps = v
			case string:
				if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
					maxSteps = parsed
				}
			}
		}
	}

	description := existingAgent.Description // keep existing
	if config.Metadata != nil {
		if desc, ok := config.Metadata["description"].(string); ok {
			description = desc
		}
	}

	// Extract input schema from frontmatter
	inputSchema, err := s.extractInputSchema(config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract input schema: %w", err)
	}

	// Check if anything actually changed
	needsUpdate := false
	if existingAgent.Prompt != promptContent {
		needsUpdate = true
	}
	if existingAgent.MaxSteps != maxSteps {
		needsUpdate = true
	}
	if existingAgent.Description != description {
		needsUpdate = true
	}
	
	// Check if input schema changed
	currentSchemaStr := ""
	if existingAgent.InputSchema != nil {
		currentSchemaStr = *existingAgent.InputSchema
	}
	newSchemaStr := ""
	if inputSchema != nil {
		newSchemaStr = *inputSchema
	}
	if currentSchemaStr != newSchemaStr {
		needsUpdate = true
	}

	if !needsUpdate {
		return &SyncOperation{
			Type:        OpTypeSkip,
			Target:      existingAgent.Name,
			Description: "Agent is up to date",
		}, nil
	}

	// Update agent using individual parameters
	err = s.repos.Agents.Update(
		existingAgent.ID,
		existingAgent.Name,
		description,
		promptContent,
		maxSteps,
		inputSchema, // input_schema - extracted from frontmatter
		nil, // cronSchedule
		true, // scheduleEnabled
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	// Update tool assignments if specified
	if len(config.Tools) > 0 {
		// Clear existing assignments
		err = s.repos.AgentTools.Clear(existingAgent.ID)
		if err != nil {
			logging.Info("Warning: Failed to clear existing tool assignments: %v", err)
		}

		// Assign new tools
		for _, toolName := range config.Tools {
			// Find tool by name in environment
			tool, err := s.repos.MCPTools.FindByNameInEnvironment(existingAgent.EnvironmentID, toolName)
			if err != nil {
				logging.Info("Warning: Tool %s not found in environment: %v", toolName, err)
				continue
			}

			// Assign tool to agent
			_, err = s.repos.AgentTools.AddAgentTool(existingAgent.ID, tool.ID)
			if err != nil {
				logging.Info("Warning: Failed to assign tool %s to agent: %v", toolName, err)
			}
		}
	}

	logging.Info("🔄 Updated agent: %s", existingAgent.Name)
	return &SyncOperation{
		Type:        OpTypeUpdate,
		Target:      existingAgent.Name,
		Description: fmt.Sprintf("Updated agent from .prompt file"),
	}, nil
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