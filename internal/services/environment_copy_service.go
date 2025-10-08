package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/models"
)

// EnvironmentCopyService handles copying environments with conflict detection
type EnvironmentCopyService struct {
	repos          *repositories.Repositories
	envMgmtService *EnvironmentManagementService
}

// NewEnvironmentCopyService creates a new environment copy service
func NewEnvironmentCopyService(repos *repositories.Repositories) *EnvironmentCopyService {
	return &EnvironmentCopyService{
		repos:          repos,
		envMgmtService: NewEnvironmentManagementService(repos),
	}
}

// CopyConflict represents a conflict during environment copy
type CopyConflict struct {
	Type          string `json:"type"` // "mcp_server" or "agent"
	Name          string `json:"name"`
	Reason        string `json:"reason"`
	SourceID      int64  `json:"source_id"`
	ConflictingID *int64 `json:"conflicting_id,omitempty"`
}

// CopyResult represents the result of copying an environment
type CopyResult struct {
	Success           bool           `json:"success"`
	TargetEnvironment string         `json:"target_environment"`
	MCPServersCopied  int            `json:"mcp_servers_copied"`
	AgentsCopied      int            `json:"agents_copied"`
	Conflicts         []CopyConflict `json:"conflicts"`
	Errors            []string       `json:"errors"`
}

// CopyEnvironment copies agents and MCP servers from source to target environment
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

	// Step 1: Copy MCP servers
	mcpServers, err := s.repos.MCPServers.GetByEnvironmentID(sourceEnvID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to fetch MCP servers: %v", err))
	} else {
		for _, mcpServer := range mcpServers {
			if err := s.copyMCPServer(mcpServer, targetEnv, result); err != nil {
				logging.Error("Failed to copy MCP server '%s': %v", mcpServer.Name, err)
				result.Errors = append(result.Errors, fmt.Sprintf("MCP server '%s': %v", mcpServer.Name, err))
			}
		}
	}

	// Step 2: Copy agents
	agents, err := s.repos.Agents.ListByEnvironment(sourceEnvID)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to fetch agents: %v", err))
	} else {
		for _, agent := range agents {
			if err := s.copyAgent(agent, targetEnv, result); err != nil {
				logging.Error("Failed to copy agent '%s': %v", agent.Name, err)
				result.Errors = append(result.Errors, fmt.Sprintf("Agent '%s': %v", agent.Name, err))
			}
		}
	}

	// Step 3: Regenerate template.json
	if err := s.regenerateTemplateJSON(targetEnv); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to regenerate template.json: %v", err))
	}

	result.Success = len(result.Errors) == 0
	logging.Info("Copy completed: %d MCP servers, %d agents, %d conflicts, %d errors",
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

	// Create the MCP server
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

	createdID, err := s.repos.MCPServers.Create(newMCP)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	logging.Info("Copied MCP server '%s' to environment '%s' (ID: %d)", source.Name, targetEnv.Name, createdID)
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

	// Create the agent using the Create method signature
	createdAgent, err := s.repos.Agents.Create(
		source.Name,
		source.Description,
		source.Prompt,
		source.MaxSteps,
		targetEnv.ID,
		source.CreatedBy,
		source.InputSchema,
		source.CronSchedule,
		source.ScheduleEnabled,
		source.OutputSchema,
		source.OutputSchemaPreset,
		source.App,
		source.AppType,
	)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Copy agent-tool relationships
	sourceTools, err := s.repos.AgentTools.ListAgentTools(source.ID)
	if err != nil {
		logging.Error("Failed to fetch tools for agent '%s': %v", source.Name, err)
	} else {
		for _, toolDetail := range sourceTools {
			// Find corresponding tool in target environment by name
			targetTool, err := s.repos.MCPTools.FindByNameInEnvironment(targetEnv.ID, toolDetail.ToolName)
			if err != nil {
				logging.Debug("Tool '%s' not found in target environment, skipping", toolDetail.ToolName)
				continue
			}

			if _, err := s.repos.AgentTools.AddAgentTool(createdAgent.ID, targetTool.ID); err != nil {
				logging.Error("Failed to assign tool '%s' to agent '%s': %v", toolDetail.ToolName, createdAgent.Name, err)
			}
		}
	}

	// Generate .prompt file
	if err := s.generateAgentPromptFile(createdAgent, targetEnv); err != nil {
		logging.Error("Failed to generate .prompt file for agent '%s': %v", createdAgent.Name, err)
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to create .prompt file for '%s': %v", createdAgent.Name, err))
	}

	logging.Info("Copied agent '%s' to environment '%s' (ID: %d)", source.Name, targetEnv.Name, createdAgent.ID)
	result.AgentsCopied++
	return nil
}


// generateAgentPromptFile creates the .prompt file for an agent
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
	tools, err := s.repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch agent tools: %w", err)
	}

	toolNames := make([]string, len(tools))
	for i, toolDetail := range tools {
		toolNames[i] = toolDetail.ToolName
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
	if agent.Description != "" {
		sb.WriteString(fmt.Sprintf("  description: \"%s\"\n", agent.Description))
	}
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
	mcpServers, err := s.repos.MCPServers.GetByEnvironmentID(env.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch MCP servers: %w", err)
	}

	// Build template.json structure
	template := map[string]interface{}{
		"name":       env.Name,
		"mcpServers": make(map[string]interface{}),
	}

	if env.Description != nil {
		template["description"] = *env.Description
	}

	for _, mcp := range mcpServers {
		mcpConfig := map[string]interface{}{
			"command": mcp.Command,
		}

		if len(mcp.Args) > 0 {
			mcpConfig["args"] = mcp.Args
		}

		if len(mcp.Env) > 0 {
			mcpConfig["env"] = mcp.Env
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

// AssignToolsFromSource assigns tools to agents in target environment by matching tool names from source environment
// This should be called after running sync on the target environment to ensure tools are discovered
func (s *EnvironmentCopyService) AssignToolsFromSource(targetEnvID, sourceEnvID int64) (int, error) {
	logging.Info("Starting tool assignment from source environment %d to target environment %d", sourceEnvID, targetEnvID)

	// Get all agents from source environment
	sourceAgents, err := s.repos.Agents.ListByEnvironment(sourceEnvID)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch source agents: %w", err)
	}

	totalAssigned := 0

	// For each source agent, find matching target agent and assign tools
	for _, sourceAgent := range sourceAgents {
		// Find matching agent in target environment by name
		targetAgent, err := s.repos.Agents.GetByNameAndEnvironment(sourceAgent.Name, targetEnvID)
		if err != nil {
			// Agent doesn't exist in target, skip
			logging.Debug("Agent '%s' not found in target environment, skipping tool assignment", sourceAgent.Name)
			continue
		}

		// Get tools assigned to source agent
		sourceTools, err := s.repos.AgentTools.ListAgentTools(sourceAgent.ID)
		if err != nil {
			logging.Error("Failed to fetch tools for source agent '%s': %v", sourceAgent.Name, err)
			continue
		}

		// Assign each tool to target agent by matching tool name and MCP server name
		for _, sourceTool := range sourceTools {
			// Find matching tool in target environment by name and server name
			targetTool, err := s.findToolByNameAndServer(targetEnvID, sourceTool.ToolName, sourceTool.ServerName)
			if err != nil {
				logging.Debug("Tool '%s' from server '%s' not found in target environment, skipping", sourceTool.ToolName, sourceTool.ServerName)
				continue
			}

			// Check if tool is already assigned to avoid duplicates
			existingTools, err := s.repos.AgentTools.ListAgentTools(targetAgent.ID)
			if err != nil {
				logging.Error("Failed to check existing tools for agent '%s': %v", targetAgent.Name, err)
				continue
			}

			alreadyAssigned := false
			for _, existing := range existingTools {
				if existing.ToolName == sourceTool.ToolName && existing.ServerName == sourceTool.ServerName {
					alreadyAssigned = true
					break
				}
			}

			if alreadyAssigned {
				logging.Debug("Tool '%s' already assigned to agent '%s', skipping", sourceTool.ToolName, targetAgent.Name)
				continue
			}

			// Assign tool to target agent
			if _, err := s.repos.AgentTools.AddAgentTool(targetAgent.ID, targetTool.ID); err != nil {
				logging.Error("Failed to assign tool '%s' to agent '%s': %v", sourceTool.ToolName, targetAgent.Name, err)
				continue
			}

			logging.Debug("Assigned tool '%s' to agent '%s'", sourceTool.ToolName, targetAgent.Name)
			totalAssigned++
		}
	}

	logging.Info("Tool assignment completed: %d tools assigned", totalAssigned)
	return totalAssigned, nil
}

// findToolByNameAndServer finds a tool in the target environment by matching tool name and MCP server name
func (s *EnvironmentCopyService) findToolByNameAndServer(environmentID int64, toolName, serverName string) (*models.MCPTool, error) {
	// Get all MCP servers in target environment
	mcpServers, err := s.repos.MCPServers.GetByEnvironmentID(environmentID)
	if err != nil {
		return nil, err
	}

	// Find the matching MCP server by name
	var targetServer *models.MCPServer
	for _, server := range mcpServers {
		if server.Name == serverName {
			targetServer = server
			break
		}
	}

	if targetServer == nil {
		return nil, fmt.Errorf("MCP server '%s' not found in target environment", serverName)
	}

	// Get tools from the target server
	tools, err := s.repos.MCPTools.GetByServerID(targetServer.ID)
	if err != nil {
		return nil, err
	}

	// Find matching tool by name
	for _, tool := range tools {
		if tool.Name == toolName {
			return tool, nil
		}
	}

	return nil, fmt.Errorf("tool '%s' not found in server '%s'", toolName, serverName)
}
