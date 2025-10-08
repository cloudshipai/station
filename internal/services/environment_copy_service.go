package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"station/internal/db/repositories"
	"station/internal/services/file_config"
	"station/pkg/logging"
	"station/pkg/models"
)

// EnvironmentCopyService handles copying environments with conflict detection
type EnvironmentCopyService struct {
	repos            *repositories.Repositories
	envMgmtService   *EnvironmentManagementService
	fileConfigWriter *file_config.FileConfigWriter
}

// NewEnvironmentCopyService creates a new environment copy service
func NewEnvironmentCopyService(repos *repositories.Repositories) *EnvironmentCopyService {
	return &EnvironmentCopyService{
		repos:            repos,
		envMgmtService:   NewEnvironmentManagementService(repos),
		fileConfigWriter: file_config.NewFileConfigWriter(),
	}
}

// CopyConflict represents a conflict during environment copy
type CopyConflict struct {
	Type          string `json:"type"` // "mcp_server" or "agent"
	Name          string `json:"name"`
	Reason        string `json:"reason"`
	SourceID      int64  `json:"source_id"`
	ConflictingID *int64 `json:"conflicting_id,omitempty"` // ID of existing item in target env
}

// CopyResult represents the result of copying an environment
type CopyResult struct {
	Success           bool            `json:"success"`
	TargetEnvironment string          `json:"target_environment"`
	MCPServersCopied  int             `json:"mcp_servers_copied"`
	AgentsCopied      int             `json:"agents_copied"`
	Conflicts         []CopyConflict  `json:"conflicts"`
	Errors            []string        `json:"errors"`
}

// CopyEnvironment copies agents and MCP servers from source to target environment
// Returns conflict information for items that couldn't be copied
func (s *EnvironmentCopyService) CopyEnvironment(sourceEnvID, targetEnvID int64) (*CopyResult, error) {
	result := &CopyResult{
		Conflicts: []CopyConflict{},
		Errors:    []string{},
	}

	// Get source and target environments
	sourceEnv, err := s.repos.Environments.GetByID(sourceEnvID)
	if err != nil {
		return nil, fmt.Errorf("source environment not found: %w", err)
	}

	targetEnv, err := s.repos.Environments.GetByID(targetEnvID)
	if err != nil {
		return nil, fmt.Errorf("target environment not found: %w", err)
	}

	result.TargetEnvironment = targetEnv.Name

	logging.Info("Starting environment copy from '%s' to '%s'", sourceEnv.Name, targetEnv.Name)

	// Step 1: Copy MCP servers and detect conflicts
	mcpServers, err := s.repos.MCPServers.GetByEnvironment(sourceEnvID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to fetch MCP servers: %v", err))
	} else {
		for _, mcpServer := range mcpServers {
			err := s.copyMCPServer(mcpServer, targetEnv, result)
			if err != nil {
				logging.Error("Failed to copy MCP server '%s': %v", mcpServer.Name, err)
				result.Errors = append(result.Errors, fmt.Sprintf("MCP server '%s': %v", mcpServer.Name, err))
			}
		}
	}

	// Step 2: Copy agents and detect conflicts
	agents, err := s.repos.Agents.ListByEnvironment(sourceEnvID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to fetch agents: %v", err))
	} else {
		for _, agent := range agents {
			err := s.copyAgent(&agent, targetEnv, result)
			if err != nil {
				logging.Error("Failed to copy agent '%s': %v", agent.Name, err)
				result.Errors = append(result.Errors, fmt.Sprintf("Agent '%s': %v", agent.Name, err))
			}
		}
	}

	// Step 3: Regenerate template.json for target environment
	if err := s.regenerateTemplateJSON(targetEnv); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to regenerate template.json: %v", err))
	}

	result.Success = len(result.Errors) == 0
	logging.Info("Environment copy completed: %d MCP servers, %d agents copied, %d conflicts, %d errors",
		result.MCPServersCopied, result.AgentsCopied, len(result.Conflicts), len(result.Errors))

	return result, nil
}

// copyMCPServer copies a single MCP server to target environment
func (s *EnvironmentCopyService) copyMCPServer(source *models.MCPServer, targetEnv *models.Environment, result *CopyResult) error {
	// Check for name conflict
	existing, err := s.repos.MCPServers.GetByNameAndEnvironment(source.Name, targetEnv.ID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error checking for existing MCP server: %w", err)
	}

	if existing != nil {
		// Conflict detected
		result.Conflicts = append(result.Conflicts, CopyConflict{
			Type:          "mcp_server",
			Name:          source.Name,
			Reason:        fmt.Sprintf("MCP server '%s' already exists in target environment", source.Name),
			SourceID:      source.ID,
			ConflictingID: &existing.ID,
		})
		logging.Debug("MCP server conflict: '%s' already exists in '%s'", source.Name, targetEnv.Name)
		return nil // Not an error, just a conflict
	}

	// No conflict, create the MCP server
	newMCP := &models.MCPServer{
		Name:           source.Name,
		Command:        source.Command,
		Args:           source.Args,
		Env:            source.Env,
		WorkingDir:     source.WorkingDir,
		TimeoutSeconds: source.TimeoutSeconds,
		AutoRestart:    source.AutoRestart,
		EnvironmentID:  targetEnv.ID,
	}

	createdMCP, err := s.repos.MCPServers.Create(newMCP)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	logging.Info("Copied MCP server '%s' to environment '%s' (ID: %d)", source.Name, targetEnv.Name, createdMCP.ID)
	result.MCPServersCopied++
	return nil
}

// copyAgent copies a single agent to target environment
func (s *EnvironmentCopyService) copyAgent(source *models.Agent, targetEnv *models.Environment, result *CopyResult) error {
	// Check for name conflict
	existing, err := s.repos.Agents.GetByNameAndEnvironment(source.Name, targetEnv.ID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("error checking for existing agent: %w", err)
	}

	if existing != nil {
		// Conflict detected
		result.Conflicts = append(result.Conflicts, CopyConflict{
			Type:          "agent",
			Name:          source.Name,
			Reason:        fmt.Sprintf("Agent '%s' already exists in target environment", source.Name),
			SourceID:      source.ID,
			ConflictingID: &existing.ID,
		})
		logging.Debug("Agent conflict: '%s' already exists in '%s'", source.Name, targetEnv.Name)
		return nil // Not an error, just a conflict
	}

	// No conflict, create the agent
	newAgent := &models.Agent{
		Name:               source.Name,
		Description:        source.Description,
		Prompt:             source.Prompt,
		Model:              source.Model,
		MaxSteps:           source.MaxSteps,
		EnvironmentID:      targetEnv.ID,
		Enabled:            source.Enabled,
		Schedule:           source.Schedule,
		InputSchema:        source.InputSchema,
		OutputSchema:       source.OutputSchema,
		OutputSchemaPreset: source.OutputSchemaPreset,
		App:                source.App,
		AppType:            source.AppType,
		CreatedBy:          source.CreatedBy,
	}

	createdAgent, err := s.repos.Agents.Create(newAgent)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Copy agent-tool relationships
	sourceTools, err := s.repos.AgentTools.GetToolsForAgent(source.ID)
	if err != nil {
		logging.Error("Failed to fetch tools for agent '%s': %v", source.Name, err)
	} else {
		for _, tool := range sourceTools {
			// Find corresponding tool in target environment by name
			targetTool, err := s.findToolInEnvironment(tool.Name, targetEnv.ID)
			if err != nil {
				logging.Debug("Tool '%s' not found in target environment, skipping", tool.Name)
				continue
			}

			if err := s.repos.AgentTools.AssignToolToAgent(createdAgent.ID, targetTool.ID); err != nil {
				logging.Error("Failed to assign tool '%s' to agent '%s': %v", tool.Name, createdAgent.Name, err)
			}
		}
	}

	// Generate .prompt file for the agent
	if err := s.generateAgentPromptFile(createdAgent, targetEnv); err != nil {
		logging.Error("Failed to generate .prompt file for agent '%s': %v", createdAgent.Name, err)
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create .prompt file for '%s': %v", createdAgent.Name, err))
	}

	logging.Info("Copied agent '%s' to environment '%s' (ID: %d)", source.Name, targetEnv.Name, createdAgent.ID)
	result.AgentsCopied++
	return nil
}

// findToolInEnvironment finds a tool by name in target environment
func (s *EnvironmentCopyService) findToolInEnvironment(toolName string, environmentID int64) (*models.Tool, error) {
	// Get all MCP servers in target environment
	mcpServers, err := s.repos.MCPServers.GetByEnvironment(environmentID)
	if err != nil {
		return nil, err
	}

	// Search for tool across all MCP servers
	for _, mcpServer := range mcpServers {
		tools, err := s.repos.MCPTools.GetByMCPServer(mcpServer.ID)
		if err != nil {
			continue
		}

		for _, tool := range tools {
			if tool.Name == toolName {
				return &tool, nil
			}
		}
	}

	return nil, fmt.Errorf("tool '%s' not found in environment", toolName)
}

// generateAgentPromptFile creates the .prompt file for an agent in the target environment
func (s *EnvironmentCopyService) generateAgentPromptFile(agent *models.Agent, env *models.Environment) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	agentsDir := filepath.Join(homeDir, ".config", "station", "environments", env.Name, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	promptFilePath := filepath.Join(agentsDir, fmt.Sprintf("%s.prompt", agent.Name))

	// Get agent tools
	tools, err := s.repos.AgentTools.GetToolsForAgent(agent.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch agent tools: %w", err)
	}

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	// Build .prompt file content
	content := s.buildPromptFileContent(agent, toolNames)

	if err := os.WriteFile(promptFilePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write .prompt file: %w", err)
	}

	logging.Debug("Created .prompt file for agent '%s' at %s", agent.Name, promptFilePath)
	return nil
}

// buildPromptFileContent constructs the .prompt file YAML format
func (s *EnvironmentCopyService) buildPromptFileContent(agent *models.Agent, toolNames []string) string {
	var sb strings.Builder

	// YAML frontmatter
	sb.WriteString("---\n")
	sb.WriteString("metadata:\n")
	sb.WriteString(fmt.Sprintf("  name: \"%s\"\n", agent.Name))
	if agent.Description != nil && *agent.Description != "" {
		sb.WriteString(fmt.Sprintf("  description: \"%s\"\n", *agent.Description))
	}
	sb.WriteString(fmt.Sprintf("model: %s\n", agent.Model))
	sb.WriteString(fmt.Sprintf("max_steps: %d\n", agent.MaxSteps))

	if len(toolNames) > 0 {
		sb.WriteString("tools:\n")
		for _, toolName := range toolNames {
			sb.WriteString(fmt.Sprintf("  - \"%s\"\n", toolName))
		}
	}

	sb.WriteString("---\n\n")

	// Agent prompt
	sb.WriteString(agent.Prompt)
	sb.WriteString("\n")

	return sb.String()
}

// regenerateTemplateJSON rebuilds the template.json file for target environment
func (s *EnvironmentCopyService) regenerateTemplateJSON(env *models.Environment) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	envDir := filepath.Join(homeDir, ".config", "station", "environments", env.Name)
	templatePath := filepath.Join(envDir, "template.json")

	// Get all MCP servers for this environment
	mcpServers, err := s.repos.MCPServers.GetByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch MCP servers: %w", err)
	}

	// Build template.json structure
	template := map[string]interface{}{
		"name":        env.Name,
		"description": env.Description,
		"mcpServers":  make(map[string]interface{}),
	}

	for _, mcp := range mcpServers {
		mcpConfig := map[string]interface{}{
			"command": mcp.Command,
		}

		// Parse args if present
		if mcp.Args != nil && *mcp.Args != "" {
			var args []string
			if err := json.Unmarshal([]byte(*mcp.Args), &args); err == nil {
				mcpConfig["args"] = args
			}
		}

		// Parse env if present
		if mcp.Env != nil && *mcp.Env != "" {
			var envVars map[string]string
			if err := json.Unmarshal([]byte(*mcp.Env), &envVars); err == nil {
				mcpConfig["env"] = envVars
			}
		}

		template["mcpServers"].(map[string]interface{})[mcp.Name] = mcpConfig
	}

	// Write template.json
	templateJSON, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template.json: %w", err)
	}

	if err := os.WriteFile(templatePath, templateJSON, 0644); err != nil {
		return fmt.Errorf("failed to write template.json: %w", err)
	}

	logging.Info("Regenerated template.json for environment '%s'", env.Name)
	return nil
}
