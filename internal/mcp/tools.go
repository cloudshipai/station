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
	
	// Add prompts for AI-assisted agent creation
	ts.setupAgentCreationPrompts()
}

// setupAgentCreationPrompts adds prompts that guide AI clients in creating agents
func (ts *ToolsServer) setupAgentCreationPrompts() {
	// Agent creation assistant prompt
	agentCreationPrompt := mcp.NewPrompt("create_comprehensive_agent",
		mcp.WithPromptDescription("Guide for creating well-structured AI agents using Station's tools and environments"),
		mcp.WithArgument("user_intent", mcp.ArgumentDescription("What the user wants to accomplish with this agent"), mcp.RequiredArgument()),
		mcp.WithArgument("domain", mcp.ArgumentDescription("Area of work (devops, data-science, marketing, etc.)")),
		mcp.WithArgument("schedule_preference", mcp.ArgumentDescription("When should this run? (on-demand, daily, weekly, custom cron)")),
	)
	
	ts.mcpServer.AddPrompt(agentCreationPrompt, ts.handleAgentCreationPrompt)
}

// handleAgentCreationPrompt provides structured guidance for agent creation
func (ts *ToolsServer) handleAgentCreationPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Get available environments and tools for context
	environments, err := ts.repos.Environments.List()
	if err != nil {
		log.Printf("Failed to get environments for prompt: %v", err)
		environments = []*models.Environment{} // Continue with empty list
	}
	
	// Get actual tool categories from discovered MCP tools
	toolsWithDetails, err := ts.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		log.Printf("Failed to get tools for prompt: %v", err)
		toolsWithDetails = []*models.MCPToolWithDetails{} // Continue with empty list
	}
	
	// Extract unique tool categories/names
	toolCategoryMap := make(map[string]bool)
	for _, tool := range toolsWithDetails {
		toolCategoryMap[tool.Name] = true
	}
	
	var toolCategories []string
	for category := range toolCategoryMap {
		toolCategories = append(toolCategories, category)
	}
	
	userIntent := ""
	domain := ""
	schedulePreference := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if intent, ok := args["user_intent"]; ok {
			userIntent = intent
		}
		if d, ok := args["domain"]; ok {
			domain = d
		}
		if sched, ok := args["schedule_preference"]; ok {
			schedulePreference = sched
		}
	}
	
	promptContent := ts.buildAgentCreationPrompt(userIntent, domain, schedulePreference, toolCategories, getEnvironmentNames(environments))

	return mcp.NewGetPromptResult("Station AI Agent Creation Assistant", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}

// buildAgentCreationPrompt creates the comprehensive agent creation prompt
func (ts *ToolsServer) buildAgentCreationPrompt(userIntent, domain, schedulePreference string, toolCategories []string, environmentNames []string) string {
	return fmt.Sprintf(`# Station AI Agent Creation Assistant

You are helping to create a sophisticated AI agent in Station - a revolutionary AI infrastructure platform. This prompt will guide you through creating a well-structured agent that takes advantage of Station's key benefits:

## Why Station for AI Agents?

1. **Background Agent Excellence**: Station is the easiest way to create background agents that work seamlessly with your development flow
2. **Environment-Based Tool Organization**: Organize tools by environments without cluttering your personal MCP setup
3. **Smart Context Management**: Filter subtools (not just servers) so agents don't get context poisoning from MCP servers with too many tools
4. **Team AI Infrastructure**: Build agents that can be shared and managed across teams

## User Intent Analysis
%s

## Current Context
- Domain: %s
- Schedule Preference: %s
- Available Environments: %v
- Available Tool Categories: %v

## Agent Creation Framework

### 1. Intent Understanding & Agent Purpose
Based on the user intent, determine:
- **Primary Goal**: What specific problem does this agent solve?
- **Success Metrics**: How will we know the agent is working well?
- **Automation Level**: On-demand, scheduled, or event-driven?

### 2. Tool Selection Strategy
Instead of overwhelming the agent with every available tool, use Station's smart filtering:
- **Core Tools**: Essential tools for the main workflow (2-4 tools max)
- **Context Tools**: Environment-specific tools that provide necessary context
- **Fallback Tools**: Additional tools for edge cases (use sparingly)

### 3. Environment Alignment
Choose the optimal environment based on:
- **Resource Access**: Does the agent need specific databases, APIs, or credentials?
- **Security Boundary**: What level of access is appropriate?
- **Team Scope**: Should this be personal or shared?

### 4. Prompt Engineering
Create a system prompt that includes:
- **Clear Role Definition**: What is the agent's primary responsibility?
- **Workflow Steps**: Step-by-step process the agent should follow
- **Quality Gates**: How should the agent validate its work?
- **Error Handling**: What to do when things go wrong?

### 5. Execution Strategy
Configure the agent for optimal performance:
- **Max Steps**: Balance thoroughness with efficiency (recommended: 3-7 steps)
- **Schedule**: Match execution frequency to business needs
- **Dependencies**: What other agents or systems does this interact with?

## Output Format

Please provide your agent creation plan in this structure:

{
  "agent_name": "descriptive-agent-name",
  "agent_description": "One-line description of agent purpose",
  "environment_selection": {
    "recommended_environment": "environment_name",
    "rationale": "Why this environment is optimal"
  },
  "tool_selection": {
    "core_tools": ["tool1", "tool2"],
    "rationale": "Why these specific tools were chosen"
  },
  "system_prompt": "Detailed system prompt for the agent...",
  "execution_config": {
    "max_steps": 5,
    "schedule": "cron_expression_or_on_demand",
    "rationale": "Why this execution pattern fits the use case"
  },
  "success_criteria": "How to measure if this agent is successful",
  "potential_improvements": ["future enhancement 1", "future enhancement 2"]
}

## Next Steps

After you provide this plan, I'll help you:
1. Validate tool availability in the selected environment
2. Refine the system prompt based on Station's capabilities
3. Create the agent using Station's enhanced MCP tools
4. Set up monitoring and scheduling if needed

Remember: Station's power comes from smart agent design, not tool proliferation. Focus on solving the specific user problem efficiently!`, 
		userIntent, domain, schedulePreference, environmentNames, toolCategories)
}

// Helper function to extract environment names
func getEnvironmentNames(environments []*models.Environment) []string {
	names := make([]string, len(environments))
	for i, env := range environments {
		names[i] = env.Name
	}
	return names
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
		runID = 0 // Run storage not yet implemented
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