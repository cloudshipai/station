package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"station/internal/auth"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolsServer handles enhanced MCP tools for agent management
type ToolsServer struct {
	repos        *repositories.Repositories
	mcpServer    *server.MCPServer
	agentService services.AgentServiceInterface
	localMode    bool
}

func NewToolsServer(repos *repositories.Repositories, mcpServer *server.MCPServer, agentService services.AgentServiceInterface, localMode bool) *ToolsServer {
	ts := &ToolsServer{
		repos:        repos,
		mcpServer:    mcpServer,
		agentService: agentService,
		localMode:    localMode,
	}
	ts.setupEnhancedTools()
	return ts
}

// setupEnhancedTools adds enhanced create_agent and call_agent tools with scheduling
func (ts *ToolsServer) setupEnhancedTools() {
	// Enhanced create_agent tool with scheduling and tool selection
	enhancedCreateAgent := mcp.NewTool("create_agent_advanced",
		mcp.WithDescription("Create a new AI agent with advanced configuration including tool selection and scheduling"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the agent")),
		mcp.WithString("description", mcp.Required(), mcp.Description("Description of the agent's purpose")),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("System prompt for the agent")),
		mcp.WithString("environment_id", mcp.Required(), mcp.Description("Environment ID where the agent will operate")),
		mcp.WithNumber("max_steps", mcp.Description("Maximum steps for agent execution (default: 5)")),
		mcp.WithArray("tool_ids", mcp.Description("List of specific tool IDs to assign to the agent")),
		mcp.WithArray("tool_names", mcp.Description("List of tool names to assign to the agent")),
		mcp.WithString("schedule", mcp.Description("Optional cron schedule for automatic execution (e.g., '0 9 * * 1-5')")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the agent is enabled for execution (default: true)")),
		mcp.WithObject("metadata", mcp.Description("Additional metadata for the agent")),
	)
	
	ts.mcpServer.AddTool(enhancedCreateAgent, ts.handleCreateAgentAdvanced)
	
	// Enhanced call_agent tool with execution options
	enhancedCallAgent := mcp.NewTool("call_agent_advanced",
		mcp.WithDescription("Execute an AI agent with advanced options and monitoring"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to execute")),
		mcp.WithString("task", mcp.Required(), mcp.Description("Task or input to provide to the agent")),
		mcp.WithBoolean("async", mcp.Description("Execute asynchronously and return run ID (default: false)")),
		mcp.WithNumber("timeout", mcp.Description("Execution timeout in seconds (default: 300)")),
		mcp.WithBoolean("store_run", mcp.Description("Store execution results in runs history (default: true)")),
		mcp.WithObject("context", mcp.Description("Additional context to provide to the agent")),
	)
	
	ts.mcpServer.AddTool(enhancedCallAgent, ts.handleCallAgentAdvanced)
}

// Enhanced tool handlers

func (ts *ToolsServer) handleCreateAgentAdvanced(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only allow admin users to create agents
	if !ts.localMode {
		user, err := auth.GetUserFromHTTPContext(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Authentication required: %v", err)), nil
		}
		
		if !user.IsAdmin {
			return mcp.NewToolResultError("Admin privileges required to create agents"), nil
		}
	}
	
	// Get user for agent creation (in local mode, we'll use a default user or skip user requirement)
	var user *models.User
	var userID int64 = 1 // Default user ID for local mode
	
	if !ts.localMode {
		var err error
		user, err = auth.GetUserFromHTTPContext(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Authentication required: %v", err)), nil
		}
		userID = user.ID
	}
	
	// Extract required parameters
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
		return mcp.NewToolResultError("Invalid environment_id format"), nil
	}
	
	// Extract optional parameters
	maxSteps := int64(request.GetInt("max_steps", 5))
	toolIDs := request.GetIntSlice("tool_ids", []int{})
	toolNames := request.GetStringSlice("tool_names", []string{})
	schedule := request.GetString("schedule", "")
	enabled := request.GetBool("enabled", true)
	
	// Convert tool IDs to strings for the service
	var assignedTools []string
	for _, toolID := range toolIDs {
		assignedTools = append(assignedTools, fmt.Sprintf("%d", toolID))
	}
	assignedTools = append(assignedTools, toolNames...)
	
	// Create agent configuration
	agentConfig := &services.AgentConfig{
		EnvironmentID: environmentID,
		Name:          name,
		Description:   description,
		Prompt:        prompt,
		AssignedTools: assignedTools,
		MaxSteps:      maxSteps,
		CreatedBy:     userID,
	}
	
	// Create the agent
	agent, err := ts.agentService.CreateAgent(ctx, agentConfig)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
	}
	
	// Handle scheduling if provided
	if schedule != "" {
		// TODO: Implement scheduling via cron service
		log.Printf("Scheduling requested for agent %d: %s", agent.ID, schedule)
	}
	
	// Return detailed success response
	result := map[string]interface{}{
		"success": true,
		"agent": map[string]interface{}{
			"id": agent.ID,
			"name": agent.Name,
			"description": agent.Description,
			"environment_id": agent.EnvironmentID,
			"max_steps": agent.MaxSteps,
			"assigned_tools": assignedTools,
			"created_by": agent.CreatedBy,
			"enabled": enabled,
			"schedule": schedule,
		},
		"message": fmt.Sprintf("Agent '%s' created successfully with ID %d", name, agent.ID),
		"timestamp": time.Now(),
	}
	
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (ts *ToolsServer) handleCallAgentAdvanced(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get user for agent execution (required in server mode, optional in local mode)
	var userID int64 = 1 // Default user ID for local mode
	
	if !ts.localMode {
		user, err := auth.GetUserFromHTTPContext(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Authentication required: %v", err)), nil
		}
		userID = user.ID
	}
	
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
	
	// Execute the agent synchronously for now
	response, err := ts.agentService.ExecuteAgent(ctx, agentID, task)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", err)), nil
	}
	
	// Store the run if requested
	var runID int64
	if storeRun {
		// TODO: Store the run in the database
		// This would require extending the agent service or accessing the repository directly
		runID = 0 // Placeholder
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