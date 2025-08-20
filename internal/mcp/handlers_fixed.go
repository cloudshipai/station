package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"station/internal/services"
	"station/pkg/models"
	"station/pkg/schema"

	"github.com/mark3labs/mcp-go/mcp"
)

// Simplified handlers that work with the current repository interfaces

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
	
	// Extract input_schema if provided
	var inputSchema *string
	if inputSchemaStr := request.GetString("input_schema", ""); inputSchemaStr != "" {
		inputSchema = &inputSchemaStr
	}
	
	// Extract tool_names array if provided
	var toolNames []string
	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if toolNamesArg, ok := argsMap["tool_names"]; ok {
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

	// Create the agent using repository with updated signature
	// Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, inputSchema *string, cronSchedule *string, scheduleEnabled bool)
	createdAgent, err := s.repos.Agents.Create(name, description, prompt, int64(maxSteps), environmentID, 1, inputSchema, nil, true)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
	}

	// Assign tools to the agent
	var assignedTools []string
	var skippedTools []string
	
	if len(toolNames) > 0 {
		// Assign specific tools if provided
		for _, toolName := range toolNames {
			// Find tool by name in the agent's environment
			tool, err := s.repos.MCPTools.FindByNameInEnvironment(environmentID, toolName)
			if err != nil {
				skippedTools = append(skippedTools, fmt.Sprintf("%s (not found)", toolName))
				continue
			}
			
			// Assign tool to agent
			_, err = s.repos.AgentTools.AddAgentTool(createdAgent.ID, tool.ID)
			if err != nil {
				skippedTools = append(skippedTools, fmt.Sprintf("%s (failed: %v)", toolName, err))
				continue
			}
			
			assignedTools = append(assignedTools, toolName)
		}
	} else {
		// If no specific tools provided, assign all available tools in the environment
		allTools, err := s.repos.MCPTools.GetByEnvironmentID(environmentID)
		if err == nil {
			for _, tool := range allTools {
				// Assign tool to agent
				_, err = s.repos.AgentTools.AddAgentTool(createdAgent.ID, tool.ID)
				if err != nil {
					skippedTools = append(skippedTools, fmt.Sprintf("%s (failed: %v)", tool.Name, err))
					continue
				}
				assignedTools = append(assignedTools, tool.Name)
			}
		}
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
	
	// Add tool assignment status to response
	if len(toolNames) > 0 {
		toolAssignment := map[string]interface{}{
			"requested_tools": toolNames,
			"assigned_tools":  assignedTools,
			"assigned_count":  len(assignedTools),
		}
		
		if len(skippedTools) > 0 {
			toolAssignment["skipped_tools"] = skippedTools
			toolAssignment["skipped_count"] = len(skippedTools)
		}
		
		if len(assignedTools) == len(toolNames) {
			toolAssignment["status"] = "success"
		} else if len(assignedTools) > 0 {
			toolAssignment["status"] = "partial"
		} else {
			toolAssignment["status"] = "failed"
		}
		
		response["tool_assignment"] = toolAssignment
		
		// Update message to include tool assignment info
		response["message"] = fmt.Sprintf("Agent '%s' created successfully with max_steps=%d in environment_id=%d. Tools assigned: %d/%d", 
			name, createdAgent.MaxSteps, createdAgent.EnvironmentID, len(assignedTools), len(toolNames))
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleCallAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Validate required dependencies
	if s.repos == nil {
		return mcp.NewToolResultError("Server repositories not initialized"), nil
	}
	
	// Get user for agent execution (default user ID for local mode)
	var userID int64 = 1
	
	// Extract required parameters
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}
	
	task, err := request.RequireString("task")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'task' parameter: %v", err)), nil
	}
	
	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError("Invalid agent_id format"), nil
	}
	
	// Extract optional parameters
	async := request.GetBool("async", false)
	timeout := request.GetInt("timeout", 300)
	storeRun := request.GetBool("store_run", true)
	
	// Extract variables for dotprompt rendering
	var userVariables map[string]interface{}
	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if variablesArg, ok := argsMap["variables"]; ok {
				if variables, ok := variablesArg.(map[string]interface{}); ok {
					userVariables = variables
				}
			}
		}
	}
	if userVariables == nil {
		userVariables = make(map[string]interface{}) // Default to empty map
	}
	
	// Use execution queue for proper tracing and storage
	var runID int64
	var response *services.Message
	var execErr error
	
	if s.executionQueue != nil {
		// Prepare metadata with user variables for dotprompt rendering
		metadata := map[string]interface{}{
			"source": "mcp",
			"user_variables": userVariables,
		}
		
		// Queue the execution for detailed tracing
		runID, execErr = s.executionQueue.QueueExecution(agentID, userID, task, metadata)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to queue agent execution: %v", execErr)), nil
		}
		
		// Wait for execution to complete and get the result
		// For synchronous execution, we need to poll the database
		// or implement a blocking queue mechanism
		time.Sleep(100 * time.Millisecond) // Small delay to let queue start
		
		// Poll for completion with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
		
	pollLoop:
		for {
			select {
			case <-timeoutCtx.Done():
				return mcp.NewToolResultError("Agent execution timed out"), nil
			default:
				// Check if execution is complete
				if s.repos.AgentRuns == nil {
					return mcp.NewToolResultError("Agent runs repository not available"), nil
				}
				run, checkErr := s.repos.AgentRuns.GetByID(runID)
				if checkErr != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to check execution status: %v", checkErr)), nil
				}
				
				if run.Status == "completed" {
					response = &services.Message{Content: run.FinalResponse}
					break pollLoop
				} else if run.Status == "failed" {
					return mcp.NewToolResultError(fmt.Sprintf("Agent execution failed: %s", run.FinalResponse)), nil
				}
				
				// Wait before polling again
				time.Sleep(500 * time.Millisecond)
			}
		}
	} else {
		// Fallback to direct execution
		if s.agentService == nil {
			return mcp.NewToolResultError("Agent service not available for direct execution"), nil
		}
		
		response, execErr = s.agentService.ExecuteAgent(ctx, agentID, task, userVariables)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", execErr)), nil
		}
		
		if response == nil {
			return mcp.NewToolResultError("Agent execution returned nil response"), nil
		}
		
		if storeRun {
			// TODO: Store the run in the database for direct execution
			runID = 0 // Run storage not yet implemented for direct execution
		}
	}
	
	// Return detailed response
	result := map[string]interface{}{
		"success": true,
		"execution": map[string]interface{}{
			"agent_id": agentID,
			"task": task,
			"response": response.Content,
			"user_id": userID,
			"run_id": runID,
			"async": async,
			"timeout": timeout,
			"stored": storeRun,
		},
		"timestamp": time.Now(),
	}
	
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
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
		"agent_id":    agentID,
		"agent_name":  agent.Name,
		"has_schema":  false,
		"schema":      nil,
		"variables":   []string{},
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

	// Extract optional parameters with defaults from existing agent
	name := request.GetString("name", existingAgent.Name)
	description := request.GetString("description", existingAgent.Description)
	prompt := request.GetString("prompt", existingAgent.Prompt)
	
	// Handle max_steps
	maxSteps := existingAgent.MaxSteps
	if maxStepsParam := request.GetString("max_steps", ""); maxStepsParam != "" {
		if parsed, parseErr := strconv.ParseInt(maxStepsParam, 10, 64); parseErr == nil {
			maxSteps = parsed
		}
	}

	// Handle input schema (preserve existing if not provided)
	var inputSchema *string
	if existingAgent.InputSchema != nil {
		inputSchema = existingAgent.InputSchema
	}
	if schemaParam := request.GetString("input_schema", ""); schemaParam != "" {
		inputSchema = &schemaParam
	}

	// Handle schedule fields (preserve existing if not provided)
	var cronSchedule *string
	scheduleEnabled := false
	if existingAgent.CronSchedule != nil {
		cronSchedule = existingAgent.CronSchedule
		scheduleEnabled = existingAgent.ScheduleEnabled
	}
	if scheduleParam := request.GetString("cron_schedule", ""); scheduleParam != "" {
		cronSchedule = &scheduleParam
		scheduleEnabled = true
	}

	// Perform the update
	err = s.repos.Agents.Update(agentID, name, description, prompt, maxSteps, inputSchema, cronSchedule, scheduleEnabled)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update agent: %v", err)), nil
	}

	// Get the updated agent for response
	updatedAgent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve updated agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":          updatedAgent.ID,
			"name":        updatedAgent.Name,
			"description": updatedAgent.Description,
			"max_steps":   updatedAgent.MaxSteps,
		},
		"message": fmt.Sprintf("Successfully updated agent '%s'", updatedAgent.Name),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleDiscoverTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get all available tools
	tools, err := s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to discover tools: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"tools":   tools,
		"count":   len(tools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListMCPConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// File-based configs: Get all file configs across all environments
	environments, err := s.repos.Environments.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list environments: %v", err)), nil
	}
	
	var allConfigs []interface{}
	for _, env := range environments {
		fileConfigs, err := s.repos.FileMCPConfigs.ListByEnvironment(env.ID)
		if err != nil {
			continue // Skip environments with no configs
		}
		for _, fc := range fileConfigs {
			allConfigs = append(allConfigs, map[string]interface{}{
				"id":             fc.ID,
				"name":           fc.ConfigName,
				"environment_id": fc.EnvironmentID,
				"path":           fc.TemplatePath,
				"type":           "file",
				"last_loaded":    fc.LastLoadedAt,
			})
		}
	}

	response := map[string]interface{}{
		"success": true,
		"configs": allConfigs,
		"count":   len(allConfigs),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract pagination parameters
	limit := request.GetInt("limit", 50)
	offset := request.GetInt("offset", 0)
	
	// Extract optional filters
	environmentID := request.GetString("environment_id", "")
	search := request.GetString("search", "")

	tools, err := s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list tools: %v", err)), nil
	}

	// Apply search filter if provided
	if search != "" {
		filteredTools := make([]*models.MCPToolWithDetails, 0)
		searchLower := strings.ToLower(search)
		for _, tool := range tools {
			if strings.Contains(strings.ToLower(tool.Name), searchLower) ||
				strings.Contains(strings.ToLower(tool.Description), searchLower) {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
	}

	// Apply environment filter if provided
	if environmentID != "" {
		filteredTools := make([]*models.MCPToolWithDetails, 0)
		for _, tool := range tools {
			if fmt.Sprintf("%d", tool.EnvironmentID) == environmentID {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
	}

	totalCount := len(tools)

	// Apply pagination
	start := offset
	if start > totalCount {
		start = totalCount
	}
	
	end := start + limit
	if end > totalCount {
		end = totalCount
	}

	paginatedTools := tools[start:end]

	response := map[string]interface{}{
		"success": true,
		"tools":   paginatedTools,
		"pagination": map[string]interface{}{
			"count":        len(paginatedTools),
			"total":        totalCount,
			"limit":        limit,
			"offset":       offset,
			"has_more":     end < totalCount,
			"next_offset":  end,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListPrompts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prompts := []map[string]interface{}{
		{"name": "create_comprehensive_agent", "description": "Guide for creating well-structured AI agents"},
		{"name": "create_logs_analysis_agent", "description": "Guide for AWS logs analysis agents"},
		{"name": "create_devops_monitor_agent", "description": "Guide for DevOps monitoring agents"},
		{"name": "create_security_scan_agent", "description": "Guide for security scanning agents"},
		{"name": "create_data_processing_agent", "description": "Guide for data processing agents"},
		{"name": "export_agents_guide", "description": "Guide for exporting agents to .prompt files"},
		{"name": "agent_export_reminder", "description": "Important reminder about exporting agents after creation to save them to disk"},
		{"name": "station_mcp_tools_guide", "description": "Comprehensive guide to using Station's MCP tools"},
		{"name": "input_schema_guide", "description": "Guide for using custom input schemas with agents"},
	}

	response := map[string]interface{}{
		"success": true,
		"prompts": prompts,
		"count":   len(prompts),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetPrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	promptName, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	var promptContent string
	var found bool

	switch promptName {
	case "export_agents_guide":
		promptContent = "# Agent Export Guide\n\nStation provides multiple ways to export agents to .prompt files for version control and deployment.\n\n## Export Single Agent\n\nUse the export_agent tool to export a specific agent:\n\n```json\n{\n  \"tool\": \"export_agent\",\n  \"parameters\": {\n    \"agent_id\": \"42\",\n    \"output_path\": \"/path/to/agent.prompt\"  // Optional\n  }\n}\n```\n\n## Export All Agents\n\nUse the export_agents tool to export multiple agents at once:\n\n```json\n{\n  \"tool\": \"export_agents\",\n  \"parameters\": {\n    \"environment_id\": \"1\",              // Optional: filter by environment\n    \"output_directory\": \"/path/to/dir\", // Optional: custom output directory\n    \"enabled_only\": true                // Optional: only export enabled agents\n  }\n}\n```\n\n## Export Format\n\nExported agents are saved as .prompt files with YAML frontmatter:\n\n```yaml\n---\nmodel: \"gemini-2.0-flash-exp\"\nconfig:\n  temperature: 0.3\n  max_tokens: 2000\nmetadata:\n  name: \"Agent Name\"\n  description: \"Agent description\"\n  version: \"1.0.0\"\ntools:\n  - \"tool_name_1\"\n  - \"tool_name_2\"\nstation:\n  execution_metadata:\n    max_steps: 5\n    environment: \"production\"\n    agent_id: 42\n---\n\nYour agent prompt content goes here...\n```\n\n## Benefits\n\n- **Version Control**: Track agent changes in git\n- **Deployment**: Deploy agents across environments\n- **Backup**: Preserve agent configurations\n- **Sharing**: Share agents with team members"
		found = true

	case "agent_export_reminder":
		promptContent = "# 📤 Important: Agent Export Required\n\n## Critical Post-Creation Step\n\nAfter creating an agent using Station's MCP tools, you **MUST export the agent to disk** to ensure it's permanently saved and accessible for future use.\n\n## Why Export is Required\n\n🔄 **In-Memory Creation**: When you create an agent via MCP tools, it's stored in Station's database but not yet saved as a .prompt file\n💾 **Disk Persistence**: Exporting creates the .prompt file that can be version-controlled, shared, and deployed\n📋 **Backup & Recovery**: .prompt files serve as the source of truth for agent configurations\n🔧 **Development Workflow**: Exported agents can be edited, tested, and iterated upon\n\n## How to Export After Creation\n\n### CLI Export Command\n```bash\nstn agent export <agent-name> <environment-name>\n```\n\n### MCP Export Tool\nUse the station MCP server's export functionality:\n```\nmcp__stn__export_agent tool with agent_id parameter\n```\n\n## What Gets Exported\n\nThe export process creates a complete .prompt file containing:\n- ✅ Agent metadata (name, description, version)\n- ✅ System prompt and configuration\n- ✅ Input schema (including custom fields)\n- ✅ Execution settings (max_steps, environment)\n- ✅ Model configuration and parameters\n\n## Export Location\n\nAgents are exported to:\n```\n~/.config/station/environments/<environment>/agents/<agent-name>.prompt\n```\n\n## Best Practices\n\n1. **Immediate Export**: Export immediately after creation\n2. **Version Control**: Add .prompt files to your git repository\n3. **Environment Consistency**: Use consistent naming across environments\n4. **Documentation**: Include clear descriptions in your agents\n\n## Example Workflow\n\n```bash\n# 1. Create agent via MCP\n# (agent now exists in Station database)\n\n# 2. Export to disk immediately\nstn agent export my-new-agent production\n\n# 3. Verify the file was created\nls ~/.config/station/environments/production/agents/my-new-agent.prompt\n\n# 4. Add to version control\ngit add ~/.config/station/environments/production/agents/my-new-agent.prompt\ngit commit -m \"Add my-new-agent configuration\"\n```\n\n## ⚠️ Remember: No Export = Potential Data Loss\n\nWithout export, your agent exists only in the database and may not survive:\n- Database migrations or resets\n- Environment changes\n- System reinstallation\n- Configuration synchronization issues\n\n**Always export your agents after creation!**"
		found = true

	case "station_mcp_tools_guide":
		promptContent = "# Station MCP Tools Guide\n\nStation provides 20 MCP tools for complete agent lifecycle management through Claude Desktop, Claude Code, or any MCP client.\n\n## Key Tools\n\n- **create_agent**: Create new AI agents\n- **list_agents**: List all agents with filters\n- **call_agent**: Execute agents with advanced options\n- **export_agent**: Export single agent to .prompt file\n- **export_agents**: Export multiple agents at once\n- **list_tools**: List available MCP tools with pagination\n- **list_runs**: List agent execution runs\n- **inspect_run**: Get detailed run information\n\n## Usage Example\n\n```json\n{\n  \"tool\": \"create_agent\",\n  \"parameters\": {\n    \"name\": \"Database Monitor\",\n    \"description\": \"Monitors database health\",\n    \"prompt\": \"You are a database monitoring specialist...\",\n    \"environment_id\": \"1\",\n    \"tool_names\": [\"postgres_query\", \"slack_notify\"]\n  }\n}\n```\n\n## Best Practices\n\n1. Start with simple prompts and add complexity gradually\n2. Only assign tools the agent actually needs\n3. Use different environments for dev/staging/prod\n4. Export agents regularly for version control\n5. Use pagination for large datasets"
		found = true
	case "input_schema_guide":
		promptContent = "# Input Schema Guide\n\nStation agents support custom input schemas to define structured input beyond the default `userInput` parameter. This enables rich, validated input data for complex agent workflows.\n\n## Default Schema\n\nEvery agent automatically has a `userInput` field:\n\n```yaml\ninput:\n  schema:\n    userInput: string  # Always available\n```\n\n## Custom Input Schema\n\nDefine additional input variables when creating an agent:\n\n```json\n{\n  \"tool\": \"create_agent\",\n  \"parameters\": {\n    \"name\": \"Deploy Agent\",\n    \"description\": \"Deploys applications to environments\",\n    \"prompt\": \"You deploy applications using the provided parameters.\",\n    \"environment_id\": \"1\",\n    \"input_schema\": \"{\\\"projectPath\\\": {\\\"type\\\": \\\"string\\\", \\\"description\\\": \\\"Path to project directory\\\"}, \\\"environment\\\": {\\\"type\\\": \\\"string\\\", \\\"enum\\\": [\\\"dev\\\", \\\"staging\\\", \\\"prod\\\"], \\\"description\\\": \\\"Target environment\\\"}, \\\"enableDebug\\\": {\\\"type\\\": \\\"boolean\\\", \\\"description\\\": \\\"Enable debug mode\\\"}}\"\n  }\n}\n```\n\n## Schema Types\n\n- **string**: Text values\n- **number**: Numeric values\n- **boolean**: true/false values\n- **array**: List of values\n- **object**: Nested objects\n\n## Schema Properties\n\n- **type**: Variable type (required)\n- **description**: Human-readable description\n- **enum**: Allowed values list\n- **default**: Default value if not provided\n- **required**: Whether field is mandatory\n\n## Exported Format\n\nAgents with custom schemas export with merged input definitions:\n\n```yaml\ninput:\n  schema:\n    userInput: string\n    projectPath: string\n    environment: string\n    enableDebug: boolean\n```\n\n## Call Agent with Variables\n\nWhen calling agents, the `variables` parameter provides custom input data:\n\n```json\n{\n  \"tool\": \"call_agent\",\n  \"parameters\": {\n    \"agent_id\": \"42\",\n    \"task\": \"Deploy the application\",\n    \"variables\": {\n      \"projectPath\": \"/home/user/myapp\",\n      \"environment\": \"staging\",\n      \"enableDebug\": true\n    }\n  }\n}\n```\n\n## Template Usage\n\nIn agent prompts, reference custom variables:\n\n```text\nDeploy the application from {{projectPath}} to {{environment}}.\n{{#if enableDebug}}Enable debug logging.{{/if}}\nUser instructions: {{userInput}}\n```\n\n## Benefits\n\n- **Structured Input**: Type-safe parameter passing\n- **Validation**: Automatic input validation\n- **Reusability**: Consistent agent interfaces\n- **Documentation**: Self-documenting agent APIs\n- **Flexibility**: Support complex workflows"
		found = true

	default:
		// For other existing prompts, return placeholder content
		switch promptName {
		case "create_comprehensive_agent":
			promptContent = "Guide for creating well-structured AI agents with proper tool assignments and configuration."
			found = true
		case "create_logs_analysis_agent":
			promptContent = "Guide for creating agents that analyze AWS CloudWatch logs and application logs."
			found = true
		case "create_devops_monitor_agent":
			promptContent = "Guide for creating DevOps monitoring agents for infrastructure and deployment monitoring."
			found = true
		case "create_security_scan_agent":
			promptContent = "Guide for creating security scanning agents for vulnerability detection and compliance."
			found = true
		case "create_data_processing_agent":
			promptContent = "Guide for creating data processing agents for ETL operations and data transformation."
			found = true
		}
	}

	if !found {
		return mcp.NewToolResultError(fmt.Sprintf("Prompt '%s' not found", promptName)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"prompt": map[string]interface{}{
			"name":    promptName,
			"content": promptContent,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListEnvironments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environments, err := s.repos.Environments.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list environments: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":      true,
		"environments": environments,
		"count":        len(environments),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agents, err := s.repos.Agents.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"agents":  agents,
		"count":   len(agents),
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

// New agent management handlers

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

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleAddTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	toolName, err := request.RequireString("tool_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'tool_name' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent to verify it exists and get environment
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Find tool by name in the agent's environment
	tool, err := s.repos.MCPTools.FindByNameInEnvironment(agent.EnvironmentID, toolName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in environment: %v", toolName, err)), nil
	}

	// Check if tool is already assigned
	existingTools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err == nil {
		for _, existingTool := range existingTools {
			if existingTool.ToolName == toolName {
				return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' is already assigned to agent '%s'", toolName, agent.Name)), nil
			}
		}
	}

	// Add tool to agent
	_, err = s.repos.AgentTools.AddAgentTool(agentID, tool.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to add tool to agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully added tool '%s' to agent '%s'", toolName, agent.Name),
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"tool": map[string]interface{}{
			"name": toolName,
			"id":   tool.ID,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleRemoveTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	toolName, err := request.RequireString("tool_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'tool_name' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", err)), nil
	}

	// Get agent to verify it exists
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Find tool by name in the agent's environment
	tool, err := s.repos.MCPTools.FindByNameInEnvironment(agent.EnvironmentID, toolName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Tool '%s' not found in environment: %v", toolName, err)), nil
	}

	// Remove tool from agent
	err = s.repos.AgentTools.RemoveAgentTool(agentID, tool.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove tool from agent: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Successfully removed tool '%s' from agent '%s'", toolName, agent.Name),
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"tool": map[string]interface{}{
			"name": toolName,
			"id":   tool.ID,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

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
			"filepath":    outputPath,
			"format":      "dotprompt",
			"written":     true,
		},
		"tools_count": len(tools),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// generateDotpromptContent generates the .prompt file content for an agent using multi-role format
func (s *Server) generateDotpromptContent(agent *models.Agent, tools []*models.AgentToolWithDetails, environment string) string {
	var content strings.Builder

	// Get configured model from Station config, fallback to default
	modelName := "gemini-1.5-flash" // default fallback
	if s.config != nil && s.config.AIModel != "" {
		modelName = s.config.AIModel
	}

	// YAML frontmatter with multi-role support
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("model: \"%s\"\n", modelName))
	content.WriteString("config:\n")
	content.WriteString("  temperature: 0.3\n")
	content.WriteString("  max_tokens: 2000\n")
	
	// Input schema with merged custom and default variables
	schemaHelper := schema.NewExportHelper()
	inputSchemaSection, err := schemaHelper.GenerateInputSchemaSection(agent)
	if err != nil {
		// Log the error but continue with default schema to avoid breaking the export
		content.WriteString("input:\n  schema:\n    userInput: string\n")
	} else {
		content.WriteString(inputSchemaSection)
	}
	
	// Add default output schema for GenKit UI compatibility
	content.WriteString("output:\n")
	content.WriteString("  schema:\n")
	content.WriteString("    response: string\n")
	
	content.WriteString("metadata:\n")
	content.WriteString(fmt.Sprintf("  name: \"%s\"\n", agent.Name))
	if agent.Description != "" {
		content.WriteString(fmt.Sprintf("  description: \"%s\"\n", agent.Description))
	}
	content.WriteString("  version: \"1.0.0\"\n")

	// Tools section
	if len(tools) > 0 {
		content.WriteString("tools:\n")
		for _, tool := range tools {
			content.WriteString(fmt.Sprintf("  - \"%s\"\n", tool.ToolName))
		}
	}

	// Station metadata
	content.WriteString("station:\n")
	content.WriteString("  execution_metadata:\n")
	if agent.MaxSteps > 0 {
		content.WriteString(fmt.Sprintf("    max_steps: %d\n", agent.MaxSteps))
	}
	content.WriteString(fmt.Sprintf("    environment: \"%s\"\n", environment))
	content.WriteString(fmt.Sprintf("    agent_id: %d\n", agent.ID))
	content.WriteString(fmt.Sprintf("    created_at: \"%s\"\n", agent.CreatedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("    updated_at: \"%s\"\n", agent.UpdatedAt.Format(time.RFC3339)))
	content.WriteString("---\n\n")

	// Multi-role prompt content
	// Check if agent prompt is already multi-role
	if s.isMultiRolePrompt(agent.Prompt) {
		// Already multi-role, use as-is
		content.WriteString(agent.Prompt)
	} else {
		// Convert single prompt to multi-role format
		content.WriteString("{{role \"system\"}}\n")
		content.WriteString(agent.Prompt)
		content.WriteString("\n\n{{role \"user\"}}\n")
		content.WriteString("{{userInput}}")
	}
	content.WriteString("\n")

	return content.String()
}

// isMultiRolePrompt checks if a prompt already contains role directives
func (s *Server) isMultiRolePrompt(prompt string) bool {
	return strings.Contains(prompt, "{{role \"") || strings.Contains(prompt, "{{role '")
}

func (s *Server) handleExportAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get optional parameters
	environmentIDStr := request.GetString("environment_id", "")
	outputDirectory := request.GetString("output_directory", "")
	enabledOnly := request.GetBool("enabled_only", false)
	
	var agents []*models.Agent
	var err error
	
	// Get agents based on environment filter
	if environmentIDStr != "" {
		environmentID, parseErr := strconv.ParseInt(environmentIDStr, 10, 64)
		if parseErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", parseErr)), nil
		}
		agents, err = s.repos.Agents.ListByEnvironment(environmentID)
	} else {
		agents, err = s.repos.Agents.List()
	}
	
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list agents: %v", err)), nil
	}
	
	// Filter enabled agents if requested
	if enabledOnly {
		var enabledAgents []*models.Agent
		for _, agent := range agents {
			if agent.ScheduleEnabled {
				enabledAgents = append(enabledAgents, agent)
			}
		}
		agents = enabledAgents
	}
	
	if len(agents) == 0 {
		return mcp.NewToolResultText("No agents found to export"), nil
	}
	
	// Export each agent
	var exportedAgents []map[string]interface{}
	var failedExports []map[string]interface{}
	
	for _, agent := range agents {
		// Get environment info
		environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
		if err != nil {
			failedExports = append(failedExports, map[string]interface{}{
				"agent_id": agent.ID,
				"name":     agent.Name,
				"error":    fmt.Sprintf("Environment not found: %v", err),
			})
			continue
		}
		
		// Get agent tools
		tools, err := s.repos.AgentTools.ListAgentTools(agent.ID)
		if err != nil {
			failedExports = append(failedExports, map[string]interface{}{
				"agent_id": agent.ID,
				"name":     agent.Name,
				"error":    fmt.Sprintf("Failed to get agent tools: %v", err),
			})
			continue
		}
		
		// Generate dotprompt content
		dotpromptContent := s.generateDotpromptContent(agent, tools, environment.Name)
		
		// Determine output file path
		var outputPath string
		if outputDirectory != "" {
			outputPath = fmt.Sprintf("%s/%s.prompt", outputDirectory, agent.Name)
		} else {
			homeDir := os.Getenv("HOME")
			if homeDir == "" {
				var homeErr error
				homeDir, homeErr = os.UserHomeDir()
				if homeErr != nil {
					failedExports = append(failedExports, map[string]interface{}{
						"agent_id": agent.ID,
						"name":     agent.Name,
						"error":    fmt.Sprintf("Failed to get user home directory: %v", homeErr),
					})
					continue
				}
			}
			outputPath = fmt.Sprintf("%s/.config/station/environments/%s/agents/%s.prompt", homeDir, environment.Name, agent.Name)
		}
		
		// Ensure directory exists
		agentsDir := filepath.Dir(outputPath)
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			failedExports = append(failedExports, map[string]interface{}{
				"agent_id": agent.ID,
				"name":     agent.Name,
				"error":    fmt.Sprintf("Failed to create agents directory: %v", err),
			})
			continue
		}
		
		// Write .prompt file to filesystem
		if err := os.WriteFile(outputPath, []byte(dotpromptContent), 0644); err != nil {
			failedExports = append(failedExports, map[string]interface{}{
				"agent_id": agent.ID,
				"name":     agent.Name,
				"error":    fmt.Sprintf("Failed to write .prompt file: %v", err),
			})
			continue
		}
		
		exportedAgents = append(exportedAgents, map[string]interface{}{
			"agent_id":    agent.ID,
			"name":        agent.Name,
			"environment": environment.Name,
			"filepath":    outputPath,
			"tools_count": len(tools),
		})
	}
	
	response := map[string]interface{}{
		"success":          true,
		"message":          fmt.Sprintf("Exported %d agents successfully", len(exportedAgents)),
		"exported_count":   len(exportedAgents),
		"failed_count":     len(failedExports),
		"exported_agents":  exportedAgents,
	}
	
	if len(failedExports) > 0 {
		response["failed_exports"] = failedExports
	}
	
	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract pagination parameters
	limit := request.GetInt("limit", 50)
	offset := request.GetInt("offset", 0)
	
	// Extract optional filters
	agentIDStr := request.GetString("agent_id", "")
	status := request.GetString("status", "")

	// Get all runs - we'll filter and paginate manually since we need filtering capability
	var allRuns []*models.AgentRunWithDetails
	var err error

	if agentIDStr != "" {
		// Filter by specific agent
		agentID, parseErr := strconv.ParseInt(agentIDStr, 10, 64)
		if parseErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", parseErr)), nil
		}
		
		// Get basic runs for this agent, then convert to detailed format
		basicRuns, err := s.repos.AgentRuns.ListByAgent(agentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list runs for agent: %v", err)), nil
		}
		
		// Convert to detailed format
		allRuns = make([]*models.AgentRunWithDetails, len(basicRuns))
		for i, run := range basicRuns {
			allRuns[i] = &models.AgentRunWithDetails{
				AgentRun:   *run,
				AgentName:  "Unknown", // Could be enhanced to fetch agent name
				Username:  "Unknown", // Could be enhanced to fetch user email
			}
		}
	} else {
		// Get recent runs (no agent filter)
		allRuns, err = s.repos.AgentRuns.ListRecent(1000) // Get large number, then filter/paginate
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list runs: %v", err)), nil
		}
	}

	// Apply status filter if provided
	if status != "" {
		filteredRuns := make([]*models.AgentRunWithDetails, 0)
		for _, run := range allRuns {
			if strings.ToLower(run.Status) == strings.ToLower(status) {
				filteredRuns = append(filteredRuns, run)
			}
		}
		allRuns = filteredRuns
	}

	totalCount := len(allRuns)

	// Apply pagination
	start := offset
	if start > totalCount {
		start = totalCount
	}
	
	end := start + limit
	if end > totalCount {
		end = totalCount
	}

	paginatedRuns := allRuns[start:end]

	response := map[string]interface{}{
		"success": true,
		"runs":    paginatedRuns,
		"pagination": map[string]interface{}{
			"count":        len(paginatedRuns),
			"total":        totalCount,
			"limit":        limit,
			"offset":       offset,
			"has_more":     end < totalCount,
			"next_offset":  end,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleInspectRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runIDStr, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid run_id format: %v", err)), nil
	}

	verbose := request.GetBool("verbose", true)

	// Get detailed run information
	run, err := s.repos.AgentRuns.GetByIDWithDetails(runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get run details: %v", err)), nil
	}

	if run == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Run with ID %d not found", runID)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"run":     run,
	}

	// Add detailed information if verbose is true
	if verbose {
		response["detailed"] = map[string]interface{}{
			"has_tool_calls":      run.ToolCalls != nil,
			"has_execution_steps": run.ExecutionSteps != nil,
			"tool_calls_count":    0,
			"execution_steps_count": 0,
		}

		if run.ToolCalls != nil {
			// Count tool calls if available
			toolCalls := *run.ToolCalls
			response["detailed"].(map[string]interface{})["tool_calls_count"] = len(toolCalls)
		}

		if run.ExecutionSteps != nil {
			// Count execution steps if available  
			execSteps := *run.ExecutionSteps
			response["detailed"].(map[string]interface{})["execution_steps_count"] = len(execSteps)
		}
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleCreateEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	description := request.GetString("description", "")
	
	// Get console user for created_by field
	consoleUser, err := s.repos.Users.GetByUsername("console")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get console user: %v", err)), nil
	}

	// Create database entry
	var desc *string
	if description != "" {
		desc = &description
	}

	env, err := s.repos.Environments.Create(name, desc, consoleUser.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create environment: %v", err)), nil
	}

	// Create file-based environment directory structure
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get user home directory: %v", err)), nil
	}

	envDir := filepath.Join(homeDir, ".config", "station", "environments", name)
	agentsDir := filepath.Join(envDir, "agents")

	// Create environment directory structure
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		// Try to cleanup database entry if directory creation fails
		s.repos.Environments.Delete(env.ID)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create environment directory: %v", err)), nil
	}

	// Create default variables.yml file
	variablesPath := filepath.Join(envDir, "variables.yml")
	defaultVariables := fmt.Sprintf("# Environment variables for %s\n# Add your template variables here\n# Example:\n# DATABASE_URL: \"your-database-url\"\n# API_KEY: \"your-api-key\"\n", name)
	
	if err := os.WriteFile(variablesPath, []byte(defaultVariables), 0644); err != nil {
		// Try to cleanup if variables file creation fails
		os.RemoveAll(envDir)
		s.repos.Environments.Delete(env.ID)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create variables.yml: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":         true,
		"environment":     env,
		"directory_path":  envDir,
		"variables_path":  variablesPath,
		"message":         fmt.Sprintf("Environment '%s' created successfully", name),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleDeleteEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	confirm, err := request.RequireBool("confirm")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'confirm' parameter: %v", err)), nil
	}

	if !confirm {
		return mcp.NewToolResultError("Confirmation required: set 'confirm' to true to proceed"), nil
	}

	// Get environment by name
	env, err := s.repos.Environments.GetByName(name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Environment '%s' not found: %v", name, err)), nil
	}

	// Prevent deletion of default environment
	if env.Name == "default" {
		return mcp.NewToolResultError("Cannot delete the default environment"), nil
	}

	// Delete file-based configuration first
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get user home directory: %v", err)), nil
	}

	envDir := filepath.Join(homeDir, ".config", "station", "environments", name)
	
	// Remove environment directory and all contents
	var fileCleanupError error
	if _, err := os.Stat(envDir); err == nil {
		fileCleanupError = os.RemoveAll(envDir)
	}

	// Delete database entries (this also deletes associated agents, runs, etc. via foreign key constraints)
	dbDeleteError := s.repos.Environments.Delete(env.ID)

	// Prepare response with cleanup status
	response := map[string]interface{}{
		"success":     dbDeleteError == nil,
		"environment": env.Name,
		"database_deleted": dbDeleteError == nil,
		"files_deleted":    fileCleanupError == nil,
	}

	if dbDeleteError != nil {
		response["database_error"] = dbDeleteError.Error()
	}

	if fileCleanupError != nil {
		response["file_cleanup_error"] = fileCleanupError.Error()
	}

	if dbDeleteError == nil && fileCleanupError == nil {
		response["message"] = fmt.Sprintf("Environment '%s' deleted successfully", name)
	} else if dbDeleteError == nil {
		response["message"] = fmt.Sprintf("Environment '%s' deleted from database, but file cleanup failed", name)
	} else {
		response["message"] = fmt.Sprintf("Failed to delete environment '%s'", name)
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

