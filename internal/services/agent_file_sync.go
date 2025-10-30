package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"station/internal/logging"
	"station/pkg/models"
	"station/pkg/schema"

	"gopkg.in/yaml.v2"
)

// DotPromptConfig represents the YAML frontmatter in a .prompt file
type DotPromptConfig struct {
	Model        string                 `yaml:"model"`
	Config       map[string]interface{} `yaml:"config"`
	Tools        []string               `yaml:"tools"`
	Metadata     map[string]interface{} `yaml:"metadata"`
	Station      map[string]interface{} `yaml:"station"`
	Input        map[string]interface{} `yaml:"input"`
	Output       map[string]interface{} `yaml:"output"`
	OutputSchema string                 `yaml:"output_schema"` // JSON string format for output schema
	MaxSteps     int64                  `yaml:"max_steps"`     // Top-level max_steps field
	App          string                 `yaml:"app"`           // CloudShip app classification
	AppType      string                 `yaml:"app_type"`      // CloudShip app_type (maps to app_subtype in DB)
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

	// Debug: Show .prompt files found
	fmt.Printf("📋 Found %d agent .prompt files\n", len(promptFiles))

	result.AgentsProcessed = len(promptFiles)

	// Process each .prompt file
	for _, promptFile := range promptFiles {
		agentName := strings.TrimSuffix(filepath.Base(promptFile), ".prompt")

		operation, err := s.syncSingleAgent(ctx, promptFile, agentName, environmentName, options)
		if err != nil {
			result.ValidationErrors++
			errorMsg := fmt.Sprintf("Agent '%s': %v", agentName, err)
			result.ValidationMessages = append(result.ValidationMessages, errorMsg)

			// TEMP DEBUG: Print validation error immediately
			fmt.Printf("❌ VALIDATION ERROR: %s\n", errorMsg)

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
		orphanedCount, err := s.cleanupOrphanedAgents(ctx, agentsDir, environmentName, promptFiles, options)
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

	// First check top-level max_steps field
	if config.MaxSteps > 0 {
		maxSteps = config.MaxSteps
	} else if config.Metadata != nil {
		// Fall back to metadata.max_steps
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

	// Extract app and app_type from top-level fields
	app := config.App
	appType := config.AppType

	// Extract input schema from frontmatter
	inputSchema, err := s.extractInputSchema(config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract input schema: %w", err)
	}

	// Extract output schema from frontmatter
	outputSchema, outputSchemaPreset, err := s.extractOutputSchema(config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract output schema: %w", err)
	}

	// Start transaction for atomic agent creation + tool assignment
	tx, err := s.repos.BeginTx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create agent using individual parameters within transaction
	createdAgent, err := s.repos.Agents.CreateTx(
		tx,
		agentName,
		description,
		promptContent,
		maxSteps,
		env.ID,
		1,                  // createdBy - system user
		inputSchema,        // input_schema - extracted from frontmatter
		nil,                // cronSchedule
		true,               // scheduleEnabled
		outputSchema,       // outputSchema - extracted from dotprompt frontmatter
		outputSchemaPreset, // outputSchemaPreset - extracted from dotprompt frontmatter
		app,                // app - CloudShip app classification
		appType,            // appType - CloudShip app_type classification
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	// Assign tools if specified - all within the same transaction
	if len(config.Tools) > 0 {
		for _, toolName := range config.Tools {
			// Find tool by name in environment
			tool, err := s.repos.MCPTools.FindByNameInEnvironment(env.ID, toolName)
			if err != nil {
				return nil, fmt.Errorf("tool %s not found in environment: %w", toolName, err)
			}

			// Assign tool to agent within transaction
			_, err = s.repos.AgentTools.AddAgentToolTx(tx, createdAgent.ID, tool.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to assign tool %s to agent: %w", toolName, err)
			}
		}
	}

	// Commit transaction - all-or-nothing
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logging.Info("✅ Created agent: %s", agentName)
	return &SyncOperation{
		Type:        OpTypeCreate,
		Target:      agentName,
		Description: "Created agent from .prompt file",
	}, nil
}

// updateAgentFromFile updates an existing agent in the database from a .prompt file
func (s *DeclarativeSync) updateAgentFromFile(ctx context.Context, existingAgent *models.Agent, config *DotPromptConfig, promptContent, checksum string) (*SyncOperation, error) {
	// Extract configuration values with defaults
	maxSteps := existingAgent.MaxSteps // keep existing

	// First check top-level max_steps field
	if config.MaxSteps > 0 {
		maxSteps = config.MaxSteps
	} else if config.Metadata != nil {
		// Fall back to metadata.max_steps
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

	// Extract app and app_type from top-level fields
	app := config.App
	appType := config.AppType

	// Extract input schema from frontmatter
	inputSchema, err := s.extractInputSchema(config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract input schema: %w", err)
	}

	// Extract output schema from frontmatter
	outputSchema, outputSchemaPreset, err := s.extractOutputSchema(config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract output schema: %w", err)
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

	// Check if output schema changed
	currentOutputSchemaStr := ""
	if existingAgent.OutputSchema != nil {
		currentOutputSchemaStr = *existingAgent.OutputSchema
	}
	newOutputSchemaStr := ""
	if outputSchema != nil {
		newOutputSchemaStr = *outputSchema
	}
	if currentOutputSchemaStr != newOutputSchemaStr {
		needsUpdate = true
	}

	// Check if output schema preset changed
	currentOutputPresetStr := ""
	if existingAgent.OutputSchemaPreset != nil {
		currentOutputPresetStr = *existingAgent.OutputSchemaPreset
	}
	newOutputPresetStr := ""
	if outputSchemaPreset != nil {
		newOutputPresetStr = *outputSchemaPreset
	}
	if currentOutputPresetStr != newOutputPresetStr {
		needsUpdate = true
	}

	// ALWAYS sync tool assignments to ensure they match .prompt file
	// This is critical for declarative sync - filesystem is source of truth
	toolsNeedSync := false
	if len(config.Tools) > 0 {
		// Get current tool assignments from database
		currentTools, err := s.repos.AgentTools.ListAgentTools(existingAgent.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current tool assignments: %w", err)
		}

		// Debug first query
		// Also query directly to verify database state
		for _, tool := range currentTools {
			fmt.Printf("     Tool: %s (ID: %d)\n", tool.ToolName, tool.ToolID)
		}

		// Create maps for comparison
		currentToolNames := make(map[string]bool)
		for _, tool := range currentTools {
			currentToolNames[tool.ToolName] = true
		}

		configToolNames := make(map[string]bool)
		for _, toolName := range config.Tools {
			configToolNames[toolName] = true
		}

		// Check if tool sets are different
		if len(currentToolNames) != len(configToolNames) {
			toolsNeedSync = true
		} else {
			// Check if all tools match
			for toolName := range currentToolNames {
				if !configToolNames[toolName] {
					toolsNeedSync = true
					break
				}
			}
			if !toolsNeedSync {
				for toolName := range configToolNames {
					if !currentToolNames[toolName] {
						toolsNeedSync = true
						break
					}
				}
			}
		}
	} else {
		// No tools in config, check if agent currently has tools assigned
		currentTools, err := s.repos.AgentTools.ListAgentTools(existingAgent.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current tool assignments: %w", err)
		}
		if len(currentTools) > 0 {
			toolsNeedSync = true // Need to clear existing tools
		}
	}

	// If nothing needs updating (not even tools), skip
	if !needsUpdate && !toolsNeedSync {
		return &SyncOperation{
			Type:        OpTypeSkip,
			Target:      existingAgent.Name,
			Description: "Agent is up to date",
		}, nil
	}

	// Start transaction for atomic agent update + tool sync
	tx, err := s.repos.BeginTx()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update agent metadata if needed
	if needsUpdate {
		err = s.repos.Agents.UpdateTx(
			tx,
			existingAgent.ID,
			existingAgent.Name,
			description,
			promptContent,
			maxSteps,
			inputSchema,        // input_schema - extracted from frontmatter
			nil,                // cronSchedule
			true,               // scheduleEnabled
			outputSchema,       // outputSchema - extracted from dotprompt frontmatter
			outputSchemaPreset, // outputSchemaPreset - extracted from dotprompt frontmatter
			app,                // app - CloudShip app classification
			appType,            // appType - CloudShip app_type classification
		)
		if err != nil {
			return nil, fmt.Errorf("failed to update agent: %w", err)
		}
	}

	// Sync tool assignments if needed (within same transaction)
	if toolsNeedSync {
		// Get current tool assignments
		currentTools, err := s.repos.AgentTools.ListAgentTools(existingAgent.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current tool assignments: %w", err)
		}

		// Debug: Log current tools
		fmt.Printf("   📊 Agent ID: %d, Current tools in DB for %s: %d tools (err: %v)\n", existingAgent.ID, existingAgent.Name, len(currentTools), err)
		for _, t := range currentTools {
			fmt.Printf("     - %s (ID: %d)\n", t.ToolName, t.ToolID)
		}
		fmt.Printf("   📊 Target tools from config: %d tools\n", len(config.Tools))
		for _, t := range config.Tools {
			fmt.Printf("     - %s\n", t)
		}

		// Create maps for efficient lookup
		currentToolMap := make(map[string]int64) // toolName -> toolID
		for _, tool := range currentTools {
			currentToolMap[tool.ToolName] = tool.ToolID
		}

		configToolSet := make(map[string]bool)
		for _, toolName := range config.Tools {
			configToolSet[toolName] = true
		}

		// Track changes for logging
		toolsAdded := 0
		toolsRemoved := 0

		// Remove tools that are no longer in config
		for toolName, toolID := range currentToolMap {
			if !configToolSet[toolName] {
				err = s.repos.AgentTools.RemoveAgentTool(existingAgent.ID, toolID)
				if err != nil {
					return nil, fmt.Errorf("failed to remove tool %s from agent: %w", toolName, err)
				}
				toolsRemoved++
			}
		}

		// Add tools that are in config but not currently assigned
		for _, toolName := range config.Tools {
			if _, exists := currentToolMap[toolName]; !exists {
				// Find tool by name in environment
				tool, err := s.repos.MCPTools.FindByNameInEnvironment(existingAgent.EnvironmentID, toolName)
				if err != nil {
					// Log warning but continue - tool might be added later when MCP server connects
					fmt.Printf("   ⚠️  Tool %s not found in environment (will retry on next sync)\n", toolName)
					continue
				}

				// Assign tool to agent within transaction
				_, err = s.repos.AgentTools.AddAgentToolTx(tx, existingAgent.ID, tool.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to assign tool %s to agent: %w", toolName, err)
				}
				toolsAdded++
			}
		}

		// Log what changed
		if toolsAdded > 0 || toolsRemoved > 0 {
			fmt.Printf("   🔧 Tool sync: +%d added, -%d removed (total: %d)\n",
				toolsAdded, toolsRemoved, len(config.Tools))
		} else if len(config.Tools) > 0 {
			fmt.Printf("   ✅ Tools already in sync (%d tools)\n", len(config.Tools))
		} else {
			fmt.Printf("   🔧 No tools configured for agent\n")
		}
	}

	// Commit transaction - all-or-nothing
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	updateType := OpTypeUpdate
	if !needsUpdate && toolsNeedSync {
		updateType = OpTypeUpdate // Still an update even if only tools changed
	}

	logging.Info("🔄 Updated agent: %s", existingAgent.Name)
	return &SyncOperation{
		Type:        updateType,
		Target:      existingAgent.Name,
		Description: "Updated agent from .prompt file",
	}, nil
}

// calculateFileChecksum calculates MD5 checksum of a file
func (s *DeclarativeSync) calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// extractInputSchema extracts and validates input schema from dotprompt config
// Supports both Picoschema format and full JSON Schema format
func (s *DeclarativeSync) extractInputSchema(config *DotPromptConfig) (*string, error) {
	if config.Input == nil || config.Input["schema"] == nil {
		return nil, nil
	}

	// Get the raw schema map
	schemaData, exists := config.Input["schema"]
	if !exists {
		return nil, nil
	}

	schemaMap, ok := schemaData.(map[interface{}]interface{})
	if !ok {
		return nil, fmt.Errorf("input.schema must be a map")
	}

	// Check if this is a full JSON Schema (has "type", "properties", "required" keys)
	// vs Picoschema format (field names with type definitions)
	hasType := false
	hasProperties := false
	for key := range schemaMap {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}
		if keyStr == "type" {
			hasType = true
		}
		if keyStr == "properties" {
			hasProperties = true
		}
	}

	// If it looks like full JSON Schema, don't process it as Picoschema
	if hasType && hasProperties {
		// This is a full JSON Schema format (like the bundled agents have)
		// Station automatically provides the userInput parameter, so we don't need
		// to extract custom input schemas from full JSON Schema definitions
		// Return nil to indicate no custom input schema needed
		return nil, nil
	}

	// Otherwise, parse as Picoschema format
	customSchema := make(map[string]*schema.InputVariable)

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
		variable := s.parsePicoschemaField(keyStr, value)
		if variable != nil && variable.Type != "" {
			customSchema[keyStr] = variable
		}
	}

	// If no custom variables, return nil
	if len(customSchema) == 0 {
		return nil, nil
	}

	// Convert to JSON string for storage
	schemaJSON, err := json.Marshal(customSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize input schema: %w", err)
	}

	// Validate the schema using our helper
	helper := schema.NewExportHelper()
	if err := helper.ValidateInputSchema(string(schemaJSON)); err != nil {
		return nil, fmt.Errorf("invalid input schema: %w", err)
	}

	schemaStr := string(schemaJSON)
	return &schemaStr, nil
}

// extractOutputSchema extracts output schema and preset information from dotprompt config
func (s *DeclarativeSync) extractOutputSchema(config *DotPromptConfig) (*string, *string, error) {
	var outputSchema *string
	var outputSchemaPreset *string

	// First check for top-level output_schema field (JSON string format)
	if config.OutputSchema != "" {
		// Trim whitespace and validate JSON
		schemaJSON := strings.TrimSpace(config.OutputSchema)

		// Validate the output schema JSON before storing
		helper := schema.NewExportHelper()
		if err := helper.ValidateOutputSchema(schemaJSON); err != nil {
			return nil, nil, fmt.Errorf("invalid output schema in agent file: %w", err)
		}

		outputSchema = &schemaJSON
		return outputSchema, outputSchemaPreset, nil
	}

	// Fall back to output.schema field (Picoschema YAML format)
	if config.Output == nil {
		return nil, nil, nil
	}

	// Check for schema field (custom output schema in Picoschema YAML format)
	if schemaData, exists := config.Output["schema"]; exists {
		// Convert map[interface{}]interface{} to map[string]interface{} for JSON marshaling
		convertedSchema := convertYAMLMapToJSONMap(schemaData)

		// Convert the YAML schema to JSON string for database storage
		schemaBytes, err := json.Marshal(convertedSchema)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal output schema to JSON: %w", err)
		}
		schemaJSON := string(schemaBytes)

		// DEBUG: Print the converted schema
		fmt.Printf("DEBUG: Converted schema JSON: %s\n", schemaJSON)

		// Validate the output schema JSON before storing
		helper := schema.NewExportHelper()
		if err := helper.ValidateOutputSchema(schemaJSON); err != nil {
			fmt.Printf("DEBUG: Validation failed for schema: %s\n", schemaJSON)
			return nil, nil, fmt.Errorf("invalid output schema in agent file: %w", err)
		}

		outputSchema = &schemaJSON
	}

	// Check for preset field (predefined schema shortcut like "finops")
	if presetData, exists := config.Output["preset"]; exists {
		if presetStr, ok := presetData.(string); ok {
			outputSchemaPreset = &presetStr
		}
	}

	return outputSchema, outputSchemaPreset, nil
}

// parsePicoschemaField parses a single Picoschema field definition
func (s *DeclarativeSync) parsePicoschemaField(fieldName string, value interface{}) *schema.InputVariable {
	switch v := value.(type) {
	case string:
		// Parse Picoschema string format: "string" or "string, description" or "type?(enum, desc): [values]"
		return s.parsePicoschemaString(fieldName, v)
	case []interface{}:
		// Array format for enums: [value1, value2, value3]
		return &schema.InputVariable{
			Type: "string",
			Enum: v,
		}
	case map[interface{}]interface{}:
		// Object format (less common in Picoschema but supported)
		variable := &schema.InputVariable{}

		if typeVal, exists := v["type"]; exists {
			if typeStr, ok := typeVal.(string); ok {
				variable.Type = schema.InputSchemaType(typeStr)
			}
		}
		if descVal, exists := v["description"]; exists {
			if descStr, ok := descVal.(string); ok {
				variable.Description = descStr
			}
		}
		if defaultVal, exists := v["default"]; exists {
			variable.Default = defaultVal
		}
		if enumVal, exists := v["enum"]; exists {
			if enumList, ok := enumVal.([]interface{}); ok {
				variable.Enum = enumList
			}
		}
		if reqVal, exists := v["required"]; exists {
			if reqBool, ok := reqVal.(bool); ok {
				variable.Required = reqBool
			}
		}

		return variable
	}

	return nil
}

// parsePicoschemaString parses Picoschema string definitions
func (s *DeclarativeSync) parsePicoschemaString(fieldName string, definition string) *schema.InputVariable {
	variable := &schema.InputVariable{}

	// Check if field is optional (ends with ?)
	isOptional := strings.HasSuffix(fieldName, "?")
	variable.Required = !isOptional

	// Handle most common case: "type, description"
	if strings.Contains(definition, ",") {
		parts := strings.SplitN(definition, ",", 2)
		variable.Type = schema.InputSchemaType(strings.TrimSpace(parts[0]))
		variable.Description = strings.TrimSpace(parts[1])
		return variable
	}

	// Handle simple case: "type"
	variable.Type = schema.InputSchemaType(strings.TrimSpace(definition))
	return variable
}

// convertYAMLMapToJSONMap recursively converts map[interface{}]interface{} to map[string]interface{}
// This is needed because YAML unmarshaling creates interface{} keys, but JSON requires string keys
func convertYAMLMapToJSONMap(data interface{}) interface{} {
	switch v := data.(type) {
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			// Convert key to string
			keyStr, ok := key.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", key)
			}
			result[keyStr] = convertYAMLMapToJSONMap(value)
		}
		return result
	case map[string]interface{}:
		// Already in correct format, but recursively convert values
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = convertYAMLMapToJSONMap(value)
		}
		return result
	case []interface{}:
		// Recursively convert array elements
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertYAMLMapToJSONMap(item)
		}
		return result
	default:
		// Primitive types (string, int, bool, etc.) pass through unchanged
		return v
	}
}
