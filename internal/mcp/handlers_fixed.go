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

	"station/internal/logging"
	"station/internal/services"
	"station/pkg/models"
	"station/pkg/schema"

	"github.com/mark3labs/mcp-go/mcp"
)

// BundleEnvironmentRequest represents the input for creating a bundle from an environment
type BundleEnvironmentRequest struct {
	EnvironmentName string `json:"environmentName"`
	OutputPath      string `json:"outputPath,omitempty"`
}

// Simplified handlers that work with the current repository interfaces

// extractInt64FromTokenUsage safely extracts int64 from various numeric types in token usage
// (Same helper function as CLI uses)
func extractInt64FromTokenUsage(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case int64:
		return &v
	case int:
		val := int64(v)
		return &val
	case int32:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	case float32:
		val := int64(v)
		return &val
	default:
		return nil
	}
}

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
	if inputSchemaParam := request.GetString("input_schema", ""); inputSchemaParam != "" {
		// Validate the schema JSON before storing
		helper := schema.NewExportHelper()
		if err := helper.ValidateInputSchema(inputSchemaParam); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid input_schema JSON: %v", err)), nil
		}
		inputSchema = &inputSchemaParam
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
		EnvironmentID: environmentID,
		Name:          name,
		Description:   description,
		Prompt:        prompt,
		AssignedTools: toolNames,
		MaxSteps:      int64(maxSteps),
		CreatedBy:     1, // Console user
		InputSchema:   inputSchema,
	}
	
	createdAgent, err := s.agentService.CreateAgent(ctx, config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
	}

	// Tool assignment is handled by AgentService.CreateAgent
	// No duplicate tool assignment logic needed here

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
	
	// Tool assignment details are handled by the AgentService
	// Simple response without duplicate tool assignment tracking

	// Automatically export agent to file-based config after successful DB save and tool assignment
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(createdAgent.ID); err != nil {
			// Log the error but don't fail the request - the agent was successfully created in DB
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Agent created but export failed: %v. Use 'stn agent export %s' to export manually.", err, name)
		}
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
	
	// Create metadata for execution
	metadata := map[string]interface{}{
		"source": "mcp",
		"user_variables": userVariables,
	}
	
	if storeRun {
		// Create agent run first to get a proper run ID
		run, err := s.repos.AgentRuns.Create(ctx, agentID, userID, task, "", 0, nil, nil, "running", nil)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent run: %v", err)), nil
		}
		runID = run.ID
		
		// Get agent details for unified execution flow
		agent, err := s.repos.Agents.GetByID(agentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
		}
		
		// Create concrete agent service to access execution engine (same as CLI)
		concreteAgentService := services.NewAgentService(s.repos)
		
		// Use the same unified execution flow as CLI 
		result, execErr := concreteAgentService.GetExecutionEngine().ExecuteAgentViaStdioMCPWithVariables(ctx, agent, task, runID, userVariables)
		if execErr != nil {
			// Update run as failed (same as CLI)
			completedAt := time.Now()
			errorMsg := fmt.Sprintf("MCP execution failed: %v", execErr)
			updateErr := s.repos.AgentRuns.UpdateCompletionWithMetadata(
				ctx, runID, errorMsg, 0, nil, nil, "failed", &completedAt,
				nil, nil, nil, nil, nil, nil,
			)
			if updateErr != nil {
				logging.Info("Warning: Failed to update failed run %d: %v", runID, updateErr)
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", execErr)), nil
		}
		
		// Update run as completed with full metadata (same as CLI)
		completedAt := time.Now()
		durationSeconds := result.Duration.Seconds()
		
		// Extract token usage from result using exact same logic as CLI
		var inputTokens, outputTokens, totalTokens *int64
		var toolsUsed *int64
		
		if result.TokenUsage != nil {
			// Use same field names and extraction logic as CLI
			if inputVal := extractInt64FromTokenUsage(result.TokenUsage["input_tokens"]); inputVal != nil {
				inputTokens = inputVal
			}
			if outputVal := extractInt64FromTokenUsage(result.TokenUsage["output_tokens"]); outputVal != nil {
				outputTokens = outputVal
			}
			if totalVal := extractInt64FromTokenUsage(result.TokenUsage["total_tokens"]); totalVal != nil {
				totalTokens = totalVal
			}
		}
		
		if result.StepsUsed > 0 {
			toolsUsedVal := int64(result.StepsUsed) // Using StepsUsed as proxy for tools used
			toolsUsed = &toolsUsedVal
		}
		
		// Determine status based on execution result success (same as CLI)
		status := "completed"
		if !result.Success {
			status = "failed"
		}
		
		// Update database with complete metadata (same as CLI)
		err = s.repos.AgentRuns.UpdateCompletionWithMetadata(
			ctx,
			runID,
			result.Response,        // final_response
			result.StepsTaken,      // steps_taken
			result.ToolCalls,       // tool_calls  
			result.ExecutionSteps,  // execution_steps
			status,                // status - now respects result.Success
			&completedAt,          // completed_at
			inputTokens,           // input_tokens
			outputTokens,          // output_tokens
			totalTokens,           // total_tokens
			&durationSeconds,      // duration_seconds
			&result.ModelName,     // model_name
			toolsUsed,            // tools_used
		)
		if err != nil {
			logging.Info("Warning: Failed to update run %d completion metadata: %v", runID, err)
		}
		
		// Convert AgentExecutionResult to Message for response
		response = &services.Message{
			Content: result.Response,
		}
	} else {
		// Execute without run storage using simplified flow
		response, execErr = s.agentService.ExecuteAgentWithRunID(ctx, agentID, task, 0, metadata)
		if execErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", execErr)), nil
		}
	}
	
	if response == nil {
		return mcp.NewToolResultError("Agent execution returned nil response"), nil
	}
	
	// Note: When storeRun=true, ExecuteAgentViaStdioMCPWithVariables automatically 
	// updates the run record with completion status, metadata, tool calls, etc.
	// No manual database update needed.
	
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

	// For now, return success with current agent data
	response := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id":          existingAgent.ID,
			"name":        existingAgent.Name,
			"description": existingAgent.Description,
		},
		"message": "Agent update functionality pending - repository signature mismatch",
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
		promptContent = "# Agent Export Guide\n\nStation provides multiple ways to export agents to .prompt files for version control and deployment.\n\n## Export Single Agent\n\nUse the export_agent tool to export a specific agent:\n\n```json\n{\n  \"tool\": \"export_agent\",\n  \"parameters\": {\n    \"agent_id\": \"42\",\n    \"output_path\": \"/path/to/agent.prompt\"  // Optional\n  }\n}\n```\n\n## Export All Agents\n\nUse the export_agents tool to export multiple agents at once:\n\n```json\n{\n  \"tool\": \"export_agents\",\n  \"parameters\": {\n    \"environment_id\": \"1\",              // Optional: filter by environment\n    \"output_directory\": \"/path/to/dir\", // Optional: custom output directory\n    \"enabled_only\": true                // Optional: only export enabled agents\n  }\n}\n```\n\n## Export Format\n\nExported agents are saved as .prompt files with YAML frontmatter:\n\n```yaml\n---\nmodel: \"gemini-2.0-flash-exp\"\nmetadata:\n  name: \"Agent Name\"\n  description: \"Agent description\"\n  version: \"1.0.0\"\ntools:\n  - \"tool_name_1\"\n  - \"tool_name_2\"\nstation:\n  execution_metadata:\n    max_steps: 5\n    environment: \"production\"\n    agent_id: 42\n---\n\nYour agent prompt content goes here...\n```\n\n## Benefits\n\n- **Version Control**: Track agent changes in git\n- **Deployment**: Deploy agents across environments\n- **Backup**: Preserve agent configurations\n- **Sharing**: Share agents with team members"
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

	// Auto-export agent to keep file config in sync (Database → Config)
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Tool added but export failed: %v. Use 'stn agent export %s' to export manually.", err, agent.Name)
		}
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

	// Auto-export agent to keep file config in sync (Database → Config)
	if s.agentExportService != nil {
		if err := s.agentExportService.ExportAgentAfterSave(agentID); err != nil {
			// Add export error info to response for user awareness
			response["export_warning"] = fmt.Sprintf("Tool removed but export failed: %v. Use 'stn agent export %s' to export manually.", err, agent.Name)
		}
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
	// config section removed - temperature not supported by gpt-5 model
	
	// Input schema with merged custom and default variables
	schemaHelper := schema.NewExportHelper()
	inputSchemaSection, err := schemaHelper.GenerateInputSchemaSection(agent)
	if err != nil {
		// Fallback to default if custom schema is invalid
		content.WriteString("input:\n")
		content.WriteString("  schema:\n")
		content.WriteString("    userInput: string\n")
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
		basicRuns, err := s.repos.AgentRuns.ListByAgent(context.Background(), agentID)
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
		allRuns, err = s.repos.AgentRuns.ListRecent(context.Background(), 1000) // Get large number, then filter/paginate
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
	run, err := s.repos.AgentRuns.GetByIDWithDetails(context.Background(), runID)
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

// Unified Bundle Management Handlers

// handleCreateBundleFromEnvironment creates an API-compatible bundle from an environment
func (s *Server) handleCreateBundleFromEnvironment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameters
	environmentName, err := req.RequireString("environmentName")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environmentName' parameter: %v", err)), nil
	}

	// Extract optional parameters
	outputPath := req.GetString("outputPath", "")
	
	bundleReq := BundleEnvironmentRequest{
		EnvironmentName: environmentName,
		OutputPath:      outputPath,
	}

	response, err := s.bundleHandler.CreateBundle(ctx, bundleReq)
	if err != nil {
		return mcp.NewToolResultText(fmt.Sprintf("Error creating bundle: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

