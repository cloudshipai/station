package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"station/internal/services"
	"station/pkg/models"
	"station/pkg/schema"

	"github.com/mark3labs/mcp-go/mcp"
)

// Agent Management Handlers
// Handles agent CRUD operations: create, update, delete, get details, list, schema

func (s *Server) handleCreateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	description, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'description' parameter: %v", err)), nil
	}

	prompt, err := request.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'prompt' parameter: %v", err)), nil
	}

	environmentIDStr, err := request.RequireString("environment_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_id' parameter: %v", err)), nil
	}

	environmentID, err := strconv.ParseInt(environmentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", err)), nil
	}

	// Extract optional parameters
	maxSteps := request.GetInt("max_steps", 5) // Default to 5 if not provided

	// Extract and validate input_schema if provided
	var inputSchema *string
	helper := schema.NewExportHelper() // Create helper for schema validation
	if inputSchemaParam := request.GetString("input_schema", ""); inputSchemaParam != "" {
		// Validate the schema JSON before storing
		if err := helper.ValidateInputSchema(inputSchemaParam); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid input_schema JSON: %v", err)), nil
		}
		inputSchema = &inputSchemaParam
	}

	// Extract output schema parameters
	var outputSchema *string
	var outputSchemaPreset *string

	if outputSchemaParam := request.GetString("output_schema", ""); outputSchemaParam != "" {
		// Validate output schema before using it
		if err := helper.ValidateOutputSchema(outputSchemaParam); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid output schema: %v", err)), nil
		}
		outputSchema = &outputSchemaParam
	}

	if outputPresetParam := request.GetString("output_schema_preset", ""); outputPresetParam != "" {
		outputSchemaPreset = &outputPresetParam
	}

	// Extract optional app and app_type parameters for CloudShip data ingestion classification
	app := request.GetString("app", "")
	appType := request.GetString("app_type", "")

	// Auto-populate app/app_type for known presets if not explicitly provided
	if app == "" && appType == "" && outputSchemaPreset != nil && *outputSchemaPreset != "" {
		switch *outputSchemaPreset {
		case "finops":
			app = "finops"
			appType = "cost-analysis"
		// Add more presets as they're created
		}
	}

	// Validate app and app_type: both must be provided together or both empty
	if (app == "" && appType != "") || (app != "" && appType == "") {
		return mcp.NewToolResultError("app and app_type must both be provided together or both omitted"), nil
	}

	// If app and app_type are provided, require output_schema or preset
	if app != "" && appType != "" && (outputSchema == nil || *outputSchema == "") && (outputSchemaPreset == nil || *outputSchemaPreset == "") {
		return mcp.NewToolResultError("app and app_type parameters require output_schema or output_schema_preset to be provided (structured output needed for data ingestion)"), nil
	}

	// Extract tool_names array if provided
	var toolNames []string
	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if toolNamesArg, exists := argsMap["tool_names"]; exists {
				if toolNamesArray, ok := toolNamesArg.([]interface{}); ok {
					for _, toolName := range toolNamesArray {
						if str, ok := toolName.(string); ok {
							toolNames = append(toolNames, str)
						}
					}
				}
			}
		}
	}

	// Create the agent using unified service layer
	config := &services.AgentConfig{
		EnvironmentID:      environmentID,
		Name:               name,
		Description:        description,
		Prompt:             prompt,
		AssignedTools:      toolNames,
		MaxSteps:           int64(maxSteps),
		CreatedBy:          1, // Console user
		InputSchema:        inputSchema,
		OutputSchema:       outputSchema,
		OutputSchemaPreset: outputSchemaPreset,
		App:                app,
		AppType:            appType,
	}

	createdAgent, err := s.agentService.CreateAgent(ctx, config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":             createdAgent.ID,
			"name":           createdAgent.Name,
			"description":    createdAgent.Description,
			"max_steps":      createdAgent.MaxSteps,
			"environment_id": createdAgent.EnvironmentID,
		},
		"message": fmt.Sprintf("Agent '%s' created successfully with max_steps=%d in environment_id=%d", name, createdAgent.MaxSteps, createdAgent.EnvironmentID),
	}

	// Automatically export agent to file-based config after successful DB save and tool assignment
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSaveWithMetadata(createdAgent.ID, app, appType); err != nil {
			// Log the error but don't fail the request - the agent was successfully created in DB
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Agent created but export failed: %v. Use 'stn agent export %s' to export manually.", err, name)
		}
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetAgentSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract agent ID parameter
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError("Invalid agent_id format"), nil
	}

	// Get the agent
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Build response with agent schema information
	response := map[string]interface{}{
		"agent_id":   agentID,
		"agent_name": agent.Name,
		"has_schema": false,
		"schema":     nil,
		"variables":  []string{},
	}

	// Always include userInput as it's automatically available
	response["variables"] = []string{"userInput"}

	// Check if agent has custom input schema
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		response["has_schema"] = true

		// Parse the stored JSON schema
		var customSchema map[string]interface{}
		if err := json.Unmarshal([]byte(*agent.InputSchema), &customSchema); err == nil {
			response["schema"] = customSchema

			// Add custom variable names to variables list
			variables := []string{"userInput"}
			for varName := range customSchema {
				variables = append(variables, varName)
			}
			response["variables"] = variables
		}
	}

	// Return schema as JSON
	schemaJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal schema: %v", err)), nil
	}

	return mcp.NewToolResultText(string(schemaJSON)), nil
}

func (s *Server) handleDeleteAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent before deletion
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Delete the agent using AgentService for proper file cleanup
	agentService := services.NewAgentService(s.repos)
	err = agentService.DeleteAgent(ctx, agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Agent '%s' deleted successfully", agent.Name),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleUpdateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get existing agent
	existingAgent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Extract optional parameters, preserving existing values if not provided
	name := request.GetString("name", existingAgent.Name)
	description := request.GetString("description", existingAgent.Description)
	prompt := request.GetString("prompt", existingAgent.Prompt)
	maxSteps := int64(request.GetInt("max_steps", int(existingAgent.MaxSteps)))

	// Handle output schema parameters
	var outputSchema *string
	var outputSchemaPreset *string

	helper := schema.NewExportHelper() // Create helper for schema validation
	if outputSchemaParam := request.GetString("output_schema", ""); outputSchemaParam != "" {
		// Validate output schema before using it
		if err := helper.ValidateOutputSchema(outputSchemaParam); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid output schema: %v", err)), nil
		}
		outputSchema = &outputSchemaParam
	} else if existingAgent.OutputSchema != nil {
		outputSchema = existingAgent.OutputSchema
	}

	if outputSchemaPresetParam := request.GetString("output_schema_preset", ""); outputSchemaPresetParam != "" {
		outputSchemaPreset = &outputSchemaPresetParam
		// Clear output_schema if preset is provided
		outputSchema = nil
	} else if existingAgent.OutputSchemaPreset != nil {
		outputSchemaPreset = existingAgent.OutputSchemaPreset
	}

	// Update the agent
	err = s.repos.Agents.Update(
		agentID,
		name,
		description,
		prompt,
		maxSteps,
		existingAgent.InputSchema,     // Keep existing input schema for now
		existingAgent.CronSchedule,    // Keep existing schedule
		existingAgent.ScheduleEnabled, // Keep existing schedule setting
		outputSchema,
		outputSchemaPreset,
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update agent: %v", err)), nil
	}

	// Export agent to filesystem after successful update
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Add export error info to response for user awareness
			response := map[string]interface{}{
				"success": true,
				"agent": map[string]interface{}{
					"id":          agentID,
					"name":        name,
					"description": description,
					"max_steps":   maxSteps,
				},
				"message":        fmt.Sprintf("Agent '%s' updated successfully", name),
				"export_warning": fmt.Sprintf("Agent updated but export failed: %v. Use 'stn agent export %s' to export manually.", err, name),
			}
			resultJSON, _ := json.MarshalIndent(response, "", "  ")
			return mcp.NewToolResultText(string(resultJSON)), nil
		}
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":          agentID,
			"name":        name,
			"description": description,
			"max_steps":   maxSteps,
		},
		"message": fmt.Sprintf("Agent '%s' updated successfully", name),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract pagination parameters
	limit := request.GetInt("limit", 50)
	offset := request.GetInt("offset", 0)

	// Extract optional filters
	environmentID := request.GetString("environment_id", "")
	enabledOnly := request.GetBool("enabled_only", false)

	agents, err := s.repos.Agents.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %v", err)), nil
	}

	// Apply environment filter if provided
	if environmentID != "" {
		envID, err := strconv.ParseInt(environmentID, 10, 64)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", err)), nil
		}
		filteredAgents := make([]*models.Agent, 0)
		for _, agent := range agents {
			if agent.EnvironmentID == envID {
				filteredAgents = append(filteredAgents, agent)
			}
		}
		agents = filteredAgents
	}

	// Apply enabled filter if provided
	if enabledOnly {
		filteredAgents := make([]*models.Agent, 0)
		for _, agent := range agents {
			// For now, consider all agents as enabled unless explicitly disabled
			// This can be enhanced when agent enabled/disabled status is implemented
			filteredAgents = append(filteredAgents, agent)
		}
		agents = filteredAgents
	}

	totalCount := len(agents)

	// Apply pagination
	start := offset
	if start > totalCount {
		start = totalCount
	}

	end := start + limit
	if end > totalCount {
		end = totalCount
	}

	paginatedAgents := agents[start:end]

	response := map[string]interface{}{
		"success": true,
		"agents":  paginatedAgents,
		"count":   len(paginatedAgents),
		"pagination": map[string]interface{}{
			"count":       len(paginatedAgents),
			"total":       totalCount,
			"limit":       limit,
			"offset":      offset,
			"has_more":    end < totalCount,
			"next_offset": end,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetAgentDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Get environment
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		environment = &models.Environment{Name: "Unknown"}
	}

	// Get assigned tools
	agentTools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		agentTools = []*models.AgentToolWithDetails{}
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":          agent.ID,
			"name":        agent.Name,
			"description": agent.Description,
			"prompt":      agent.Prompt,
			"max_steps":   agent.MaxSteps,
		},
		"environment": map[string]interface{}{
			"id":   environment.ID,
			"name": environment.Name,
		},
		"tools":       agentTools,
		"tools_count": len(agentTools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleUpdateAgentPrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	prompt, err := request.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'prompt' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get existing agent to verify it exists
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Update the agent prompt
	err = s.repos.Agents.UpdatePrompt(agentID, prompt)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update agent prompt: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully updated prompt for agent '%s'", agent.Name),
		"agent": map[string]interface{}{
			"id":     agent.ID,
			"name":   agent.Name,
			"prompt": prompt,
		},
	}

	// Automatically export agent to file-based config after successful DB update
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Agent updated but export failed: %v. Use 'stn agent export %s' to export manually.", err, agent.Name)
		}
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}