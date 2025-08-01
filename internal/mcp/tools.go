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

// setupEnhancedTools adds enhanced call_agent tools and prompts
func (ts *ToolsServer) setupEnhancedTools() {
	// Note: create_agent is now consolidated in the main MCP server (mcp.go)
	
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
	
	// Add specialized prompts for common use cases 
	ts.setupSpecializedPrompts()
}

// setupSpecializedPrompts adds prompts for common agent creation use cases
func (ts *ToolsServer) setupSpecializedPrompts() {
	// AWS logs analysis agent prompt
	logsAnalysisPrompt := mcp.NewPrompt("create_logs_analysis_agent",
		mcp.WithPromptDescription("Guide for creating an agent that searches and analyzes AWS logs for urgent issues"),
		mcp.WithArgument("log_sources", mcp.ArgumentDescription("AWS log sources to analyze (CloudWatch, S3, etc.)")),
		mcp.WithArgument("urgency_criteria", mcp.ArgumentDescription("What makes a log entry urgent or high priority")),
		mcp.WithArgument("analysis_depth", mcp.ArgumentDescription("Level of analysis needed (summary, detailed, root-cause)")),
	)
	
	ts.mcpServer.AddPrompt(logsAnalysisPrompt, ts.handleLogsAnalysisPrompt)
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

After you provide this plan, I'll:
1. Present the complete agent configuration for your review
2. Ask for your explicit confirmation before creating the agent
3. Validate tool availability in the selected environment
4. Create the agent using Station's enhanced MCP tools
5. Set up monitoring and scheduling if needed

## ⚠️ Important: User Confirmation Required

**I will NOT create any agent without your explicit approval.** After analyzing your requirements and presenting the plan above, I will:

1. **Show you the complete agent details** including name, description, system prompt, environment, tools, and configuration
2. **Ask: "Do you want me to create this agent with these exact specifications?"**
3. **Wait for your "yes" or confirmation** before proceeding with agent creation
4. **Allow you to modify** any aspect of the agent before creation

This ensures you have full control over what agents are created in your Station environment.

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

// Note: handleCreateAgentAdvanced has been consolidated into the main MCP server (mcp.go)

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

// handleLogsAnalysisPrompt provides specialized guidance for AWS logs analysis agent creation
func (ts *ToolsServer) handleLogsAnalysisPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	logSources := ""
	urgencyCriteria := ""
	analysisDepth := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if sources, ok := args["log_sources"]; ok {
			logSources = sources
		}
		if criteria, ok := args["urgency_criteria"]; ok {
			urgencyCriteria = criteria
		}
		if depth, ok := args["analysis_depth"]; ok {
			analysisDepth = depth
		}
	}
	
	promptContent := fmt.Sprintf(`# AWS Logs Analysis Agent Creation Guide

## Specialized Agent Configuration

**Agent Purpose**: Intelligent AWS log analysis and urgent issue prioritization

### Recommended Configuration

**Agent Name**: "aws-logs-analyzer"
**Description**: "Analyzes AWS logs across multiple sources and prioritizes urgent issues for immediate attention"

### Tool Requirements for AWS Logs Analysis

**Essential Tools** (select from available MCP tools):
- aws-cli or aws-sdk tools for accessing CloudWatch, S3, ECS logs
- search or grep tools for pattern matching in log files
- json parsing tools for structured log analysis
- file operations for log file access
- notification tools for urgent issue alerts

### System Prompt Template

You are an AWS Logs Analysis Agent specialized in searching and prioritizing urgent issues across AWS infrastructure.

Your Mission: 
1. Search specified AWS log sources: %s
2. Identify urgent issues based on: %s
3. Provide analysis depth: %s

Workflow:
1. Log Collection: Access logs from CloudWatch, S3 buckets, ECS containers, Lambda functions
2. Pattern Recognition: Look for error patterns, performance anomalies, security alerts
3. Urgency Classification: 
   - CRITICAL: Service outages, security breaches, data loss
   - HIGH: Performance degradation, failed deployments, resource exhaustion
   - MEDIUM: Warnings, deprecated API usage, configuration issues
   - LOW: Info messages, debug traces, routine operations

4. Prioritization: Rank issues by business impact and time sensitivity
5. Summary Report: Provide actionable insights with timestamps and affected resources

Output Format:
- CRITICAL issues (immediate action required)
- HIGH priority issues (address within hours)
- Summary of patterns and trends
- Recommended next steps

Error Handling: If log access fails, report the specific AWS service/region and suggest troubleshooting steps.

### Execution Configuration
- Max Steps: 7 (allows thorough log analysis across multiple sources)
- Schedule: On-demand (for immediate analysis) or hourly (for continuous monitoring)
- Environment: Production environment with AWS credentials

### Success Criteria
- Identifies critical issues within 5 minutes of execution
- Provides clear priority ranking of all discovered issues
- Includes specific log entries and timestamps for evidence
- Suggests concrete remediation steps

Ready to create this agent? Use the 'create_agent' tool with these specifications.
`, logSources, urgencyCriteria, analysisDepth)

	return mcp.NewGetPromptResult("AWS Logs Analysis Agent Creator", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}