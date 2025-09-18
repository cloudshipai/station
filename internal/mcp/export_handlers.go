package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"station/pkg/models"
	"station/pkg/schema"

	"github.com/mark3labs/mcp-go/mcp"
)

// Export Handlers
// Handles agent export operations: export single agent, export all agents

func (s *Server) handleExportAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get optional output path
	outputPath := request.GetString("output_path", "")

	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Get environment info
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Environment not found: %v", err)), nil
	}

	// Get agent tools
	tools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get agent tools: %v", err)), nil
	}

	// Generate dotprompt content
	dotpromptContent := s.generateDotpromptContent(agent, tools, environment.Name)

	// Determine output file path like CLI does
	if outputPath == "" {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			var homeErr error
			homeDir, homeErr = os.UserHomeDir()
			if homeErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to get user home directory: %v", homeErr)), nil
			}
		}
		outputPath = fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environment.Name, agent.Name)
	}

	// Ensure directory exists
	agentsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agents directory: %v", err)), nil
	}

	// Write .prompt file to filesystem like CLI does
	if err := os.WriteFile(outputPath, []byte(dotpromptContent), 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write .prompt file: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully exported agent '%s' to dotprompt file", agent.Name),
		"agent": map[string]interface{}{
			"id":          agent.ID,
			"name":        agent.Name,
			"environment": environment.Name,
		},
		"export": map[string]interface{}{
			"filepath": outputPath,
			"format":   "dotprompt",
			"written":  true,
		},
		"tools_count": len(tools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleExportAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract optional parameters
	enabledOnly := request.GetBool("enabled_only", false)
	environmentIDStr := request.GetString("environment_id", "")
	outputDirectory := request.GetString("output_directory", "")

	// Parse environment ID if provided
	var environmentID *int64
	if environmentIDStr != "" {
		parsedID, err := strconv.ParseInt(environmentIDStr, 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", err)), nil
		}
		environmentID = &parsedID
	}

	// Get agents to export
	dbAgents, err := s.repos.Agents.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %v", err)), nil
	}

	// Filter agents
	var filteredAgents []*models.Agent
	for _, agent := range dbAgents {
		// Filter by environment if specified
		if environmentID != nil && agent.EnvironmentID != *environmentID {
			continue
		}
		// Filter by enabled status if specified
		if enabledOnly && !agent.ScheduleEnabled {
			continue
		}
		filteredAgents = append(filteredAgents, agent)
	}

	if len(filteredAgents) == 0 {
		return mcp.NewToolResultError("No agents found to export"), nil
	}

	// Export all agents
	exportResults := make([]map[string]interface{}, 0)
	var exportErrors []string

	for _, agent := range filteredAgents {
		// Get environment info
		environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to get environment for agent '%s': %v", agent.Name, err)
			exportErrors = append(exportErrors, errorMsg)
			continue
		}

		// Get agent tools
		tools, err := s.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			errorMsg := fmt.Sprintf("Failed to get tools for agent '%s': %v", agent.Name, err)
			exportErrors = append(exportErrors, errorMsg)
			continue
		}

		// Generate dotprompt content
		dotpromptContent := s.generateDotpromptContent(agent, tools, environment.Name)

		// Determine output file path
		var exportPath string
		if outputDirectory == "" {
			homeDir := os.Getenv("HOME")
			if homeDir == "" {
				var homeErr error
				homeDir, homeErr = os.UserHomeDir()
				if homeErr != nil {
					errorMsg := fmt.Sprintf("Failed to get home directory for agent '%s': %v", agent.Name, homeErr)
					exportErrors = append(exportErrors, errorMsg)
					continue
				}
			}
			exportPath = fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environment.Name, agent.Name)
		} else {
			exportPath = filepath.Join(outputDirectory, fmt.Sprintf("%s.prompt", agent.Name))
		}

		// Ensure directory exists
		agentsDir := filepath.Dir(exportPath)
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			errorMsg := fmt.Sprintf("Failed to create directory for agent '%s': %v", agent.Name, err)
			exportErrors = append(exportErrors, errorMsg)
			continue
		}

		// Write .prompt file
		if err := os.WriteFile(exportPath, []byte(dotpromptContent), 0644); err != nil {
			errorMsg := fmt.Sprintf("Failed to write .prompt file for agent '%s': %v", agent.Name, err)
			exportErrors = append(exportErrors, errorMsg)
			continue
		}

		exportResults = append(exportResults, map[string]interface{}{
			"agent_id":    agent.ID,
			"agent_name":  agent.Name,
			"export_path": exportPath,
			"success":     true,
		})
	}

	// Prepare response
	response := map[string]interface{}{
		"success":         len(exportErrors) == 0,
		"exported_count":  len(exportResults),
		"total_agents":    len(filteredAgents),
		"export_results":  exportResults,
		"output_directory": outputDirectory,
	}

	if len(exportErrors) > 0 {
		response["errors"] = exportErrors
		response["error_count"] = len(exportErrors)
		response["message"] = fmt.Sprintf("Exported %d of %d agents with %d errors", len(exportResults), len(filteredAgents), len(exportErrors))
	} else {
		response["message"] = fmt.Sprintf("Successfully exported %d agents", len(exportResults))
	}

	// Add environment filter info if used
	if environmentID != nil {
		response["environment_filter"] = environmentIDStr
	}

	if enabledOnly {
		response["enabled_only"] = true
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// generateDotpromptContent generates the .prompt file content for an agent using multi-role format
func (s *Server) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environment string) string {
	var content strings.Builder

	// Get configured model from Station config, fallback to default
	modelName := "gemini-2.5-flash" // default fallback
	if s.config != nil && s.config.AIModel != "" {
		modelName = s.config.AIModel
	}

	// YAML frontmatter with multi-role support
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("model: \"%s\"\n", modelName))

	// Input schema with merged custom and default variables
	schemaHelper := schema.NewExportHelper()
	inputSchemaSection, err := schemaHelper.GenerateInputSchemaSection(agent)
	if err != nil {
		// Fallback to default if custom schema is invalid
		content.WriteString("input:\n")
		content.WriteString("  userInput:\n")
		content.WriteString("    type: string\n")
		content.WriteString("    description: \"The user's input or task description\"\n")
	} else {
		content.WriteString(inputSchemaSection)
	}

	// Output schema section
	if agent.OutputSchema != nil && *agent.OutputSchema != "" {
		content.WriteString("output:\n")
		content.WriteString("  schema: |\n")
		content.WriteString("    " + *agent.OutputSchema + "\n")
	}

	// Max steps
	if agent.MaxSteps > 0 {
		content.WriteString(fmt.Sprintf("max_steps: %d\n", agent.MaxSteps))
	}

	// Tools list
	if len(tools) > 0 {
		content.WriteString("tools:\n")
		for _, tool := range tools {
			content.WriteString(fmt.Sprintf("  - \"%s\"\n", tool.ToolName))
		}
	}

	content.WriteString("---\n\n")

	// Multi-role prompt content with system role
	content.WriteString("{{role \"system\"}}\n")
	content.WriteString(agent.Prompt)
	content.WriteString("\n\n")

	// User role with variable substitution
	content.WriteString("{{role \"user\"}}\n")
	content.WriteString("{{userInput}}")

	return content.String()
}