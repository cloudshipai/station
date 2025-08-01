package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"station/internal/auth"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	mcpServer        *server.MCPServer
	httpServer       *server.StreamableHTTPServer
	db               db.Database
	mcpConfigSvc     *services.MCPConfigService
	toolDiscoverySvc *ToolDiscoveryService
	agentService     services.AgentServiceInterface
	authService      *auth.AuthService
	repos            *repositories.Repositories
	localMode        bool
}

type ToolDiscoveryService struct {
	db              db.Database
	mcpConfigSvc    *services.MCPConfigService
	repos           *repositories.Repositories
	discoveredTools map[string][]mcp.Tool // configID -> tools
}

func NewToolDiscoveryService(database db.Database, mcpConfigSvc *services.MCPConfigService, repos *repositories.Repositories) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		db:              database,
		mcpConfigSvc:    mcpConfigSvc,
		repos:           repos,
		discoveredTools: make(map[string][]mcp.Tool),
	}
}

func (t *ToolDiscoveryService) DiscoverTools(ctx context.Context, configID int64) error {
	// Get MCP config
	_, err := t.mcpConfigSvc.GetDecryptedConfig(configID)
	if err != nil {
		return fmt.Errorf("failed to get MCP config: %w", err)
	}

	// Get tools from database for this config
	configIDStr := fmt.Sprintf("%d", configID)
	
	// Get config first to get environment info
	config, err := t.repos.MCPConfigs.GetByID(configID)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}
	
	// Get tools for this config's environment
	dbTools, err := t.repos.MCPTools.GetByEnvironmentID(config.EnvironmentID)
	if err != nil {
		log.Printf("Failed to get tools for config %d: %v", configID, err)
		t.discoveredTools[configIDStr] = []mcp.Tool{}
		return nil
	}
	
	// Convert database tools to MCP tools
	var mcpTools []mcp.Tool
	for _, tool := range dbTools {
		mcpTool := mcp.NewTool(tool.Name, mcp.WithDescription(tool.Description))
		mcpTools = append(mcpTools, mcpTool)
	}
	
	t.discoveredTools[configIDStr] = mcpTools
	log.Printf("Discovered %d tools for config %d", len(mcpTools), configID)
	return nil
}

func (t *ToolDiscoveryService) GetDiscoveredTools(configID string) []mcp.Tool {
	return t.discoveredTools[configID]
}

func NewServer(database db.Database, mcpConfigSvc *services.MCPConfigService, agentService services.AgentServiceInterface, repos *repositories.Repositories, localMode bool) *Server {
	// Create MCP server using the official mcp-go library
	mcpServer := server.NewMCPServer(
		"Station MCP Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithRecovery(),
	)

	toolDiscoverySvc := NewToolDiscoveryService(database, mcpConfigSvc, repos)
	authService := auth.NewAuthService(repos)

	// Create streamable HTTP server - simple pattern like the user's example
	httpServer := server.NewStreamableHTTPServer(mcpServer)
	
	log.Printf("MCP Server configured with streamable HTTP transport")

	server := &Server{
		mcpServer:        mcpServer,
		httpServer:       httpServer,
		db:               database,
		mcpConfigSvc:     mcpConfigSvc,
		toolDiscoverySvc: toolDiscoverySvc,
		agentService:     agentService,
		authService:      authService,
		repos:            repos,
		localMode:        localMode,
	}

	// Setup basic MCP tools
	server.setupTools()
	
	// Setup resources
	server.setupResources()
	
	// Setup enhanced MCP tools
	NewToolsServer(repos, mcpServer, agentService, localMode)
	
	return server
}

func (s *Server) setupTools() {
	// Add a tool to get available MCP configurations
	configListTool := mcp.NewTool("list_mcp_configs",
		mcp.WithDescription("List all available MCP configurations"),
	)

	s.mcpServer.AddTool(configListTool, s.handleListMCPConfigs)

	// Add a tool to discover tools from a specific MCP config
	discoverTool := mcp.NewTool("discover_tools",
		mcp.WithDescription("Discover tools from an MCP configuration"),
		mcp.WithString("config_id",
			mcp.Required(),
			mcp.Description("ID of the MCP configuration to discover tools from"),
		),
	)

	s.mcpServer.AddTool(discoverTool, s.handleDiscoverTools)

	// TODO: Add external MCP tool calling when MCP client is implemented
	// For now, we only support Station's native tools

	// Add a tool to create AI agents (consolidated version)
	createAgentTool := mcp.NewTool("create_agent",
		mcp.WithDescription("Create a new AI agent with advanced configuration including tool selection and scheduling"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the agent"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Description of the agent's purpose"),
		),
		mcp.WithString("prompt",
			mcp.Required(),
			mcp.Description("System prompt for the agent"),
		),
		mcp.WithString("environment_id",
			mcp.Required(),
			mcp.Description("Environment ID where the agent will operate"),
		),
		mcp.WithNumber("max_steps",
			mcp.Description("Maximum steps for agent execution (default: 5)"),
		),
		mcp.WithArray("tool_names",
			mcp.Description("List of tool names to assign to the agent (environment-specific)"),
		),
		mcp.WithString("schedule",
			mcp.Description("Optional cron schedule for automatic execution (e.g., '0 9 * * 1-5')"),
		),
		mcp.WithBoolean("enabled",
			mcp.Description("Whether the agent is enabled for execution (default: true)"),
		),
	)

	s.mcpServer.AddTool(createAgentTool, s.handleCreateAgent)

	// Add a tool to execute (call) AI agents
	callAgentTool := mcp.NewTool("call_agent",
		mcp.WithDescription("Execute an AI agent with a given task"),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("ID of the agent to execute"),
		),
		mcp.WithString("task",
			mcp.Required(),
			mcp.Description("Task or input to provide to the agent"),
		),
	)

	s.mcpServer.AddTool(callAgentTool, s.handleCallAgent)

	// Add a tool to delete AI agents
	deleteAgentTool := mcp.NewTool("delete_agent",
		mcp.WithDescription("Delete an AI agent by ID"),
		mcp.WithString("agent_id",
			mcp.Required(),
			mcp.Description("ID of the agent to delete"),
		),
	)

	s.mcpServer.AddTool(deleteAgentTool, s.handleDeleteAgent)

	// Add a tool to list available tools by environment
	listToolsTool := mcp.NewTool("list_tools",
		mcp.WithDescription("List available MCP tools filtered by environment"),
		mcp.WithString("environment_id",
			mcp.Description("Environment ID to filter tools by (optional - shows all if not provided)"),
		),
		mcp.WithBoolean("include_details",
			mcp.Description("Include detailed information about each tool (default: false)"),
		),
	)

	s.mcpServer.AddTool(listToolsTool, s.handleListTools)

	// Add a tool to list available prompts
	listPromptsTool := mcp.NewTool("list_prompts",
		mcp.WithDescription("List available MCP prompts for guided agent creation and management"),
	)

	s.mcpServer.AddTool(listPromptsTool, s.handleListPrompts)

	// Add tools for accessing runs
	listRunsTool := mcp.NewTool("list_runs",
		mcp.WithDescription("List recent agent execution runs"),
		mcp.WithNumber("limit", mcp.Description("Maximum number of runs to return (default: 50)")),
	)

	s.mcpServer.AddTool(listRunsTool, s.handleListRuns)

	getRunTool := mcp.NewTool("get_run",
		mcp.WithDescription("Get detailed information about a specific agent run"),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("ID of the run to retrieve")),
	)

	s.mcpServer.AddTool(getRunTool, s.handleGetRun)

	listAgentRunsTool := mcp.NewTool("list_agent_runs",
		mcp.WithDescription("List all runs for a specific agent"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to get runs for")),
	)

	s.mcpServer.AddTool(listAgentRunsTool, s.handleListAgentRuns)

	// Add environment management tools
	listEnvironmentsTool := mcp.NewTool("list_environments",
		mcp.WithDescription("List all available environments with their tool counts"),
		mcp.WithBoolean("include_tool_counts", mcp.Description("Include number of tools per environment (default: true)")),
	)

	s.mcpServer.AddTool(listEnvironmentsTool, s.handleListEnvironments)

	// Add agent management tools
	listAgentsTool := mcp.NewTool("list_agents",
		mcp.WithDescription("List all agents with their status and configuration"),
		mcp.WithString("environment_id", mcp.Description("Filter agents by environment (optional)")),
		mcp.WithBoolean("include_details", mcp.Description("Include detailed configuration (default: false)")),
	)

	s.mcpServer.AddTool(listAgentsTool, s.handleListAgents)

	getAgentDetailsTool := mcp.NewTool("get_agent_details",
		mcp.WithDescription("Get detailed information about a specific agent including assigned tools"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to get details for")),
	)

	s.mcpServer.AddTool(getAgentDetailsTool, s.handleGetAgentDetails)

	updateAgentTool := mcp.NewTool("update_agent",
		mcp.WithDescription("Update agent configuration including tools, prompt, and schedule"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to update")),
		mcp.WithString("name", mcp.Description("New name for the agent")),
		mcp.WithString("description", mcp.Description("New description for the agent")),
		mcp.WithString("prompt", mcp.Description("New system prompt for the agent")),
		mcp.WithArray("tool_names", mcp.Description("New list of tool names to assign (replaces existing)")),
		mcp.WithArray("add_tools", mcp.Description("Tool names to add to existing tools")),
		mcp.WithArray("remove_tools", mcp.Description("Tool names to remove from existing tools")),
		mcp.WithNumber("max_steps", mcp.Description("New maximum steps for execution")),
		mcp.WithString("schedule", mcp.Description("New cron schedule (empty to disable scheduling)")),
	)

	s.mcpServer.AddTool(updateAgentTool, s.handleUpdateAgent)
}

func (s *Server) setupResources() {
	// Add static resources for read-only data access
	s.setupStaticResources()
	
	// Add dynamic resource templates for parameterized access
	s.setupResourceTemplates()
	
	log.Printf("MCP resources setup complete - read-only data access via Resources, operations via Tools")
}

// setupStaticResources adds static resources for Station data discovery
func (s *Server) setupStaticResources() {
	// Environments list resource
	environmentsResource := mcp.NewResource(
		"station://environments",
		"Station Environments",
		mcp.WithResourceDescription("List all available environments with their configurations"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(environmentsResource, s.handleEnvironmentsResource)

	// Agents list resource
	agentsResource := mcp.NewResource(
		"station://agents",
		"Station Agents",
		mcp.WithResourceDescription("List all available agents with basic information"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(agentsResource, s.handleAgentsResource)

	// MCP configs list resource
	configsResource := mcp.NewResource(
		"station://mcp-configs",
		"MCP Configurations",
		mcp.WithResourceDescription("List all MCP server configurations across environments"),
		mcp.WithMIMEType("application/json"),
	)
	s.mcpServer.AddResource(configsResource, s.handleMCPConfigsResource)
}

// setupResourceTemplates adds dynamic resource templates for parameterized access
func (s *Server) setupResourceTemplates() {
	// Agent details by ID
	agentDetailsTemplate := mcp.NewResourceTemplate(
		"station://agents/{id}",
		"Agent Details",
		mcp.WithTemplateDescription("Detailed information about a specific agent including tools and configuration"),
		mcp.WithTemplateMIMEType("application/json"),
	)
	s.mcpServer.AddResourceTemplate(agentDetailsTemplate, s.handleAgentDetailsResource)

	// Environment tools by environment ID
	environmentToolsTemplate := mcp.NewResourceTemplate(
		"station://environments/{id}/tools",
		"Environment Tools",
		mcp.WithTemplateDescription("List all available MCP tools in a specific environment"),
		mcp.WithTemplateMIMEType("application/json"),
	)
	s.mcpServer.AddResourceTemplate(environmentToolsTemplate, s.handleEnvironmentToolsResource)

	// Agent runs by agent ID
	agentRunsTemplate := mcp.NewResourceTemplate(
		"station://agents/{id}/runs",
		"Agent Runs",
		mcp.WithTemplateDescription("Execution history for a specific agent"),
		mcp.WithTemplateMIMEType("application/json"),
	)
	s.mcpServer.AddResourceTemplate(agentRunsTemplate, s.handleAgentRunsResource)
}

func (s *Server) handleListMCPConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can list MCP configs
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Get actual MCP configs from database
	configs, err := s.repos.MCPConfigs.GetAllLatestConfigs()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get MCP configs: %v", err)), nil
	}
	
	if len(configs) == 0 {
		return mcp.NewToolResultText("No MCP configurations found"), nil
	}
	
	var configNames []string
	for _, config := range configs {
		configNames = append(configNames, fmt.Sprintf("%s (ID: %d, Env: %d)", config.ConfigName, config.ID, config.EnvironmentID))
	}
	
	return mcp.NewToolResultText(fmt.Sprintf("Available MCP configs:\n%s", strings.Join(configNames, "\n"))), nil
}

func (s *Server) handleDiscoverTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can discover tools
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	configIDStr, err := request.RequireString("config_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Convert string to int64
	var configID int64
	if _, err := fmt.Sscanf(configIDStr, "%d", &configID); err != nil {
		return mcp.NewToolResultError("Invalid config ID format"), nil
	}

	// Discover tools from the specified MCP config
	err = s.toolDiscoverySvc.DiscoverTools(ctx, configID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to discover tools: %v", err)), nil
	}

	tools := s.toolDiscoverySvc.GetDiscoveredTools(configIDStr)
	var toolNames []string
	for _, tool := range tools {
		toolNames = append(toolNames, tool.Name)
	}

	return mcp.NewToolResultText(fmt.Sprintf("Discovered tools from config %s: %v", configIDStr, toolNames)), nil
}

// handleCallMCPTool removed - external MCP tool calling not yet implemented

func (s *Server) handleCreateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can create agents
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Get authenticated user from context (or use default for local mode)
	var user *models.User
	var userID int64 = 1 // Default user ID for local mode
	
	if err := s.requireAuthInServerMode(ctx); err == nil {
		var authErr error
		user, authErr = auth.GetUserFromHTTPContext(ctx)
		if authErr == nil {
			userID = user.ID
		}
	}

	// Extract required parameters
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'name' parameter: %v", err)), nil
	}

	description, err := request.RequireString("description")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'description' parameter: %v", err)), nil
	}

	prompt, err := request.RequireString("prompt")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'prompt' parameter: %v", err)), nil
	}

	environmentIDStr, err := request.RequireString("environment_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'environment_id' parameter: %v", err)), nil
	}

	// Convert string parameters to appropriate types
	var environmentID int64
	if _, err := fmt.Sscanf(environmentIDStr, "%d", &environmentID); err != nil {
		return mcp.NewToolResultError("Invalid environment_id format: must be a number"), nil
	}

	// Use the determined user ID
	createdBy := userID

	// Extract optional parameters
	maxSteps := int64(request.GetInt("max_steps", 5))
	assignedTools := request.GetStringSlice("tool_names", []string{})
	schedule := request.GetString("schedule", "")
	enabled := request.GetBool("enabled", true)

	// Create agent configuration
	agentConfig := &services.AgentConfig{
		EnvironmentID: environmentID,
		Name:          name,
		Description:   description,
		Prompt:        prompt,
		AssignedTools: assignedTools,
		MaxSteps:      maxSteps,
		CreatedBy:     createdBy,
	}

	// Create the agent
	agent, err := s.agentService.CreateAgent(ctx, agentConfig)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create agent: %v", err)), nil
	}

	// Handle scheduling if provided
	if schedule != "" {
		log.Printf("Scheduling requested for agent %d: %s", agent.ID, schedule)
		// TODO: Implement scheduling via cron service
	}

	// Return success response with agent details
	result := fmt.Sprintf(`Agent created successfully:
ID: %d
Name: %s
Description: %s
Environment ID: %d
Max Steps: %d
Assigned Tools: %v
Schedule: %s
Enabled: %t
Created By: %d`, 
		agent.ID, agent.Name, agent.Description, agent.EnvironmentID, 
		agent.MaxSteps, assignedTools, schedule, enabled, agent.CreatedBy)

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleCallAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow both admin and regular users
	var userID int64 = 1 // Default user ID for local mode
	
	if err := s.requireAuthInServerMode(ctx); err == nil {
		user, authErr := auth.GetUserFromHTTPContext(ctx)
		if authErr == nil {
			userID = user.ID
		}
	} else if !s.isLocalMode() {
		// In server mode, authentication is required
		return mcp.NewToolResultError("Authentication required"), nil
	}

	// Extract required parameters
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'agent_id' parameter: %v", err)), nil
	}

	task, err := request.RequireString("task")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'task' parameter: %v", err)), nil
	}

	// Convert string parameters to appropriate types
	var agentID int64
	if _, err := fmt.Sscanf(agentIDStr, "%d", &agentID); err != nil {
		return mcp.NewToolResultError("Invalid agent_id format: must be a number"), nil
	}

	// userID is already determined above

	// Execute the agent
	response, err := s.agentService.ExecuteAgent(ctx, agentID, task)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to execute agent: %v", err)), nil
	}

	// TODO: Store the execution result in agent_runs table
	// This would require accessing the repository directly or extending the agent service
	// For now, we'll return the response without storing the run

	// Return success response with agent execution result
	result := fmt.Sprintf(`Agent executed successfully:
Agent ID: %d
Task: %s
Response: %s
User ID: %d`, 
		agentID, task, response.Content, userID)

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleDeleteAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can delete agents
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Extract required parameters
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing or invalid 'agent_id' parameter: %v", err)), nil
	}

	// Convert string parameters to appropriate types
	var agentID int64
	if _, err := fmt.Sscanf(agentIDStr, "%d", &agentID); err != nil {
		return mcp.NewToolResultError("Invalid agent_id format: must be a number"), nil
	}

	// Check if agent exists before attempting to delete
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent with ID %d not found: %v", agentID, err)), nil
	}

	// Delete the agent from the database
	err = s.repos.Agents.Delete(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete agent: %v", err)), nil
	}

	// Return success response with agent details
	result := fmt.Sprintf(`Agent deleted successfully:
ID: %d
Name: %s
Description: %s
Environment ID: %d`, 
		agent.ID, agent.Name, agent.Description, agent.EnvironmentID)

	return mcp.NewToolResultText(result), nil
}

func (s *Server) handleListTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}

	// Extract optional parameters
	environmentIDStr := request.GetString("environment_id", "")
	includeDetails := request.GetBool("include_details", false)

	// Get tools from database
	var toolsWithDetails []*models.MCPToolWithDetails
	var err error

	// Get all tools with details
	toolsWithDetails, err = s.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get tools: %v", err)), nil
	}

	// Filter by environment if specified
	if environmentIDStr != "" {
		var environmentID int64
		if _, err := fmt.Sscanf(environmentIDStr, "%d", &environmentID); err != nil {
			return mcp.NewToolResultError("Invalid environment_id format: must be a number"), nil
		}
		
		// Filter the tools by environment ID
		var filteredTools []*models.MCPToolWithDetails
		for _, tool := range toolsWithDetails {
			if tool.EnvironmentID == environmentID {
				filteredTools = append(filteredTools, tool)
			}
		}
		toolsWithDetails = filteredTools
	}

	if len(toolsWithDetails) == 0 {
		if environmentIDStr != "" {
			return mcp.NewToolResultText(fmt.Sprintf("No tools found in environment %s", environmentIDStr)), nil
		}
		return mcp.NewToolResultText("No tools found"), nil
	}

	// Build response
	var result strings.Builder
	if environmentIDStr != "" {
		result.WriteString(fmt.Sprintf("Tools in environment %s:\n\n", environmentIDStr))
	} else {
		result.WriteString("Available tools across all environments:\n\n")
	}

	// Group tools by environment for better organization
	envToolsMap := make(map[string][]models.MCPToolWithDetails)
	for _, tool := range toolsWithDetails {
		envName := tool.EnvironmentName
		if envName == "" {
			envName = fmt.Sprintf("Environment %d", tool.EnvironmentID)
		}
		envToolsMap[envName] = append(envToolsMap[envName], *tool)
	}

	// Output tools grouped by environment with prefixes
	for envName, tools := range envToolsMap {
		result.WriteString(fmt.Sprintf("📁 %s:\n", envName))
		for _, tool := range tools {
			// Prefix tool name with environment for clarity
			prefixedName := fmt.Sprintf("%s:%s", envName, tool.Name)
			
			if includeDetails {
				result.WriteString(fmt.Sprintf("  • %s\n", prefixedName))
				result.WriteString(fmt.Sprintf("    Description: %s\n", tool.Description))
				result.WriteString(fmt.Sprintf("    Config: %s\n", tool.ConfigName))
				result.WriteString("\n")
			} else {
				result.WriteString(fmt.Sprintf("  • %s - %s\n", prefixedName, tool.Description))
			}
		}
		result.WriteString("\n")
	}

	// Add usage hint
	result.WriteString("💡 Tip: Use tool names with environment prefix (e.g., 'default:search_files') when creating agents to ensure environment-specific tool access.\n")

	return mcp.NewToolResultText(result.String()), nil
}

func (s *Server) handleListPrompts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}

	// Build response with available prompts
	var result strings.Builder
	result.WriteString("📋 Available MCP Prompts:\n\n")
	
	// Document the available prompts (these are defined in tools.go)
	result.WriteString("🎯 **create_comprehensive_agent**\n")
	result.WriteString("   Description: Guide for creating well-structured AI agents using Station's tools and environments\n")
	result.WriteString("   Parameters:\n")
	result.WriteString("   • user_intent (required): What you want to accomplish with this agent\n")
	result.WriteString("   • domain (optional): Area of work (devops, data-science, marketing, etc.)\n")
	result.WriteString("   • schedule_preference (optional): When should this run? (on-demand, daily, weekly, custom cron)\n")
	result.WriteString("\n")
	
	result.WriteString("💡 **How to use prompts:**\n")
	result.WriteString("1. Claude can invoke prompts directly via MCP protocol\n")
	result.WriteString("2. Prompts provide structured guidance for complex tasks\n")
	result.WriteString("3. They include context about available environments and tools\n")
	result.WriteString("4. Ask Claude: 'Please use the create_comprehensive_agent prompt to help me create an agent'\n")
	result.WriteString("\n")
	
	result.WriteString("🔍 **Benefits of using prompts:**\n")
	result.WriteString("• Ensures agents are created with proper tool assignments\n")
	result.WriteString("• Provides environment-specific guidance\n")
	result.WriteString("• Follows Station's best practices\n")
	result.WriteString("• Reduces context poisoning by smart tool selection\n")

	return mcp.NewToolResultText(result.String()), nil
}

func (s *Server) Start(ctx context.Context, port int) error {
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting MCP server using streamable HTTP transport on %s", addr)
	log.Printf("MCP endpoint will be available at http://localhost:%d/mcp", port)
	
	// Start the streamable HTTP server - simple pattern like user's example
	if err := s.httpServer.Start(addr); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}
	
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("MCP server shutting down...")
	
	// Create timeout context if none provided
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
	}
	
	// Shutdown with timeout
	done := make(chan error, 1)
	go func() {
		done <- s.httpServer.Shutdown(ctx)
	}()
	
	select {
	case err := <-done:
		log.Println("MCP server shutdown completed")
		return err
	case <-ctx.Done():
		log.Println("MCP server shutdown timeout - forcing close")
		return ctx.Err()
	}
}

// isLocalMode returns true if the server is running in local mode
func (s *Server) isLocalMode() bool {
	return s.localMode
}

// requireAuthInServerMode checks if authentication is required in server mode
func (s *Server) requireAuthInServerMode(ctx context.Context) error {
	if s.localMode {
		return nil // No auth required in local mode
	}
	
	// In server mode, authentication is required
	_, err := auth.GetUserFromHTTPContext(ctx)
	return err
}

// requireAdminInServerMode checks if the user is an admin in server mode
func (s *Server) requireAdminInServerMode(ctx context.Context) error {
	if s.localMode {
		return nil // No admin check in local mode
	}
	
	// In server mode, check if user is authenticated and is admin
	user, err := auth.GetUserFromHTTPContext(ctx)
	if err != nil {
		return fmt.Errorf("authentication required: %v", err)
	}
	
	if !user.IsAdmin {
		return fmt.Errorf("admin privileges required")
	}
	
	return nil
}

// Runs tool handlers

func (s *Server) handleListRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}
	
	// Get limit parameter
	limit := int64(request.GetInt("limit", 50))
	if limit <= 0 || limit > 200 {
		limit = 50 // Reasonable default
	}
	
	// Get recent runs from database
	runs, err := s.repos.AgentRuns.ListRecent(limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get runs: %v", err)), nil
	}
	
	if len(runs) == 0 {
		return mcp.NewToolResultText("No runs found"), nil
	}
	
	// Format response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Recent agent runs (showing %d):\n\n", len(runs)))
	
	for _, run := range runs {
		status := "🟢"
		switch run.Status {
		case "failed":
			status = "🔴"
		case "running":
			status = "🟡"
		case "pending":
			status = "⏳"
		}
		
		result.WriteString(fmt.Sprintf("%s Run %d: %s\n", status, run.ID, run.AgentName))
		result.WriteString(fmt.Sprintf("   Status: %s\n", run.Status))
		result.WriteString(fmt.Sprintf("   Started: %s\n", run.StartedAt.Format("Jan 2, 2006 15:04")))
		if run.CompletedAt != nil {
			duration := run.CompletedAt.Sub(run.StartedAt)
			result.WriteString(fmt.Sprintf("   Duration: %.1fs\n", duration.Seconds()))
		}
		result.WriteString(fmt.Sprintf("   Task: %s\n", truncateString(run.Task, 80)))
		result.WriteString("\n")
	}
	
	return mcp.NewToolResultText(result.String()), nil
}

func (s *Server) handleGetRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}
	
	// Get run ID parameter
	runIDStr, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}
	
	var runID int64
	if _, err := fmt.Sscanf(runIDStr, "%d", &runID); err != nil {
		return mcp.NewToolResultError("Invalid run_id format: must be a number"), nil
	}
	
	// Get run details from database
	run, err := s.repos.AgentRuns.GetByIDWithDetails(runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Run not found: %v", err)), nil
	}
	
	// Format detailed response
	var result strings.Builder
	status := "🟢"
	switch run.Status {
	case "failed":
		status = "🔴"
	case "running":
		status = "🟡"
	case "pending":
		status = "⏳"
	}
	
	result.WriteString(fmt.Sprintf("%s Run %d Details\n\n", status, run.ID))
	result.WriteString(fmt.Sprintf("Agent: %s (ID: %d)\n", run.AgentName, run.AgentID))
	result.WriteString(fmt.Sprintf("Status: %s\n", run.Status))
	result.WriteString(fmt.Sprintf("Task: %s\n", run.Task))
	result.WriteString(fmt.Sprintf("Started: %s\n", run.StartedAt.Format("Jan 2, 2006 15:04:05")))
	
	if run.CompletedAt != nil {
		duration := run.CompletedAt.Sub(run.StartedAt)
		result.WriteString(fmt.Sprintf("Completed: %s (Duration: %.1fs)\n", run.CompletedAt.Format("Jan 2, 2006 15:04:05"), duration.Seconds()))
	}
	
	result.WriteString(fmt.Sprintf("Steps Taken: %d\n", run.StepsTaken))
	
	if run.FinalResponse != "" {
		result.WriteString(fmt.Sprintf("\nResponse:\n%s\n", run.FinalResponse))
	}
	
	if run.ToolCalls != nil && len(*run.ToolCalls) > 0 {
		result.WriteString(fmt.Sprintf("\nTool Calls (%d):\n", len(*run.ToolCalls)))
		for i, toolCall := range *run.ToolCalls {
			if i >= 5 { // Limit to first 5 tool calls for readability
				result.WriteString(fmt.Sprintf("... and %d more\n", len(*run.ToolCalls)-5))
				break
			}
			result.WriteString(fmt.Sprintf("  %d. %v\n", i+1, toolCall))
		}
	}
	
	return mcp.NewToolResultText(result.String()), nil
}

func (s *Server) handleListAgentRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}
	
	// Get agent ID parameter
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}
	
	var agentID int64
	if _, err := fmt.Sscanf(agentIDStr, "%d", &agentID); err != nil {
		return mcp.NewToolResultError("Invalid agent_id format: must be a number"), nil
	}
	
	// Get runs for the agent from database
	runs, err := s.repos.AgentRuns.ListByAgent(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get runs for agent: %v", err)), nil
	}
	
	if len(runs) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No runs found for agent %d", agentID)), nil
	}
	
	// Format response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Runs for Agent %d (showing %d):\n\n", agentID, len(runs)))
	
	for _, run := range runs {
		status := "🟢"
		switch run.Status {
		case "failed":
			status = "🔴"
		case "running":
			status = "🟡"
		case "pending":
			status = "⏳"
		}
		
		result.WriteString(fmt.Sprintf("%s Run %d: %s\n", status, run.ID, run.Status))
		result.WriteString(fmt.Sprintf("   Started: %s\n", run.StartedAt.Format("Jan 2, 2006 15:04")))
		if run.CompletedAt != nil {
			duration := run.CompletedAt.Sub(run.StartedAt)
			result.WriteString(fmt.Sprintf("   Duration: %.1fs\n", duration.Seconds()))
		}
		result.WriteString(fmt.Sprintf("   Task: %s\n", truncateString(run.Task, 80)))
		result.WriteString("\n")
	}
	
	return mcp.NewToolResultText(result.String()), nil
}

// Helper function for truncating strings
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// Environment management handlers

func (s *Server) handleListEnvironments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}

	includeToolCounts := request.GetBool("include_tool_counts", true)

	// Get all environments from database
	environments, err := s.repos.Environments.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environments: %v", err)), nil
	}

	if len(environments) == 0 {
		return mcp.NewToolResultText("No environments found"), nil
	}

	// Build response
	var result strings.Builder
	result.WriteString("📁 Available Environments:\n\n")

	for _, env := range environments {
		result.WriteString(fmt.Sprintf("🌍 **%s** (ID: %d)\n", env.Name, env.ID))
		if env.Description != nil {
			result.WriteString(fmt.Sprintf("   Description: %s\n", *env.Description))
		}
		result.WriteString(fmt.Sprintf("   Created: %s\n", env.CreatedAt.Format("Jan 2, 2006")))

		if includeToolCounts {
			// Get tool count for this environment
			tools, err := s.repos.MCPTools.GetByEnvironmentID(env.ID)
			if err != nil {
				result.WriteString("   Tools: Error getting count\n")
			} else {
				result.WriteString(fmt.Sprintf("   Tools: %d available\n", len(tools)))
			}
		}
		result.WriteString("\n")
	}

	result.WriteString("💡 Use environment IDs when creating agents or filtering tools\n")

	return mcp.NewToolResultText(result.String()), nil
}

// Agent management handlers

func (s *Server) handleListAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}

	environmentIDStr := request.GetString("environment_id", "")
	includeDetails := request.GetBool("include_details", false)

	// Get agents from database
	var agents []*models.Agent
	var err error

	if environmentIDStr != "" {
		var environmentID int64
		if _, err := fmt.Sscanf(environmentIDStr, "%d", &environmentID); err != nil {
			return mcp.NewToolResultError("Invalid environment_id format: must be a number"), nil
		}
		agents, err = s.repos.Agents.ListByEnvironment(environmentID)
	} else {
		agents, err = s.repos.Agents.List()
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get agents: %v", err)), nil
	}

	if len(agents) == 0 {
		if environmentIDStr != "" {
			return mcp.NewToolResultText(fmt.Sprintf("No agents found in environment %s", environmentIDStr)), nil
		}
		return mcp.NewToolResultText("No agents found"), nil
	}

	// Build response
	var result strings.Builder
	if environmentIDStr != "" {
		result.WriteString(fmt.Sprintf("🤖 Agents in Environment %s:\n\n", environmentIDStr))
	} else {
		result.WriteString(fmt.Sprintf("🤖 All Agents (%d total):\n\n", len(agents)))
	}

	for _, agent := range agents {
		status := "🟢 Active"
		if agent.CronSchedule != nil && *agent.CronSchedule != "" {
			if agent.ScheduleEnabled {
				status = "⏰ Scheduled"
			} else {
				status = "⏸️ Paused"
			}
		}

		result.WriteString(fmt.Sprintf("%s **%s** (ID: %d)\n", status, agent.Name, agent.ID))
		result.WriteString(fmt.Sprintf("   Environment: %d\n", agent.EnvironmentID))
		result.WriteString(fmt.Sprintf("   Description: %s\n", agent.Description))

		if includeDetails {
			result.WriteString(fmt.Sprintf("   Max Steps: %d\n", agent.MaxSteps))
			if agent.CronSchedule != nil && *agent.CronSchedule != "" {
				result.WriteString(fmt.Sprintf("   Schedule: %s\n", *agent.CronSchedule))
			}
			result.WriteString(fmt.Sprintf("   Created: %s\n", agent.CreatedAt.Format("Jan 2, 2006")))
			result.WriteString(fmt.Sprintf("   Prompt: %s\n", truncateString(agent.Prompt, 100)))
		}
		result.WriteString("\n")
	}

	result.WriteString("💡 Use 'get_agent_details' to see full configuration including assigned tools\n")

	return mcp.NewToolResultText(result.String()), nil
}

func (s *Server) handleGetAgentDetails(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, require authentication but allow all users
	if err := s.requireAuthInServerMode(ctx); err != nil && !s.isLocalMode() {
		return mcp.NewToolResultError("Authentication required"), nil
	}

	// Get agent ID parameter
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	var agentID int64
	if _, err := fmt.Sscanf(agentIDStr, "%d", &agentID); err != nil {
		return mcp.NewToolResultError("Invalid agent_id format: must be a number"), nil
	}

	// Get agent from database
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Get assigned tools for this agent
	assignedTools, err := s.repos.AgentTools.List(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get agent tools: %v", err)), nil
	}

	// Get environment details
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environment: %v", err)), nil
	}

	// Build detailed response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("🤖 **%s** (ID: %d) - Detailed Configuration\n\n", agent.Name, agent.ID))

	// Basic info
	result.WriteString("📋 **Basic Information:**\n")
	result.WriteString(fmt.Sprintf("   Description: %s\n", agent.Description))
	result.WriteString(fmt.Sprintf("   Environment: %s (ID: %d)\n", environment.Name, environment.ID))
	result.WriteString(fmt.Sprintf("   Max Steps: %d\n", agent.MaxSteps))
	result.WriteString(fmt.Sprintf("   Created: %s\n", agent.CreatedAt.Format("Jan 2, 2006 15:04")))
	result.WriteString("\n")

	// Schedule info
	result.WriteString("⏰ **Scheduling:**\n")
	if agent.CronSchedule != nil && *agent.CronSchedule != "" {
		status := "Enabled"
		if !agent.ScheduleEnabled {
			status = "Disabled"
		}
		result.WriteString(fmt.Sprintf("   Schedule: %s (%s)\n", *agent.CronSchedule, status))
	} else {
		result.WriteString("   Schedule: On-demand only\n")
	}
	result.WriteString("\n")

	// Tools info
	result.WriteString("🔧 **Assigned Tools:**\n")
	if len(assignedTools) == 0 {
		result.WriteString("   No tools assigned\n")
	} else {
		// Group tools by environment for clarity
		envToolsMap := make(map[int64][]models.AgentTool)
		for _, tool := range assignedTools {
			envToolsMap[tool.EnvironmentID] = append(envToolsMap[tool.EnvironmentID], tool.AgentTool)
		}

		for envID, tools := range envToolsMap {
			env, _ := s.repos.Environments.GetByID(envID)
			envName := fmt.Sprintf("Environment %d", envID)
			if env != nil {
				envName = env.Name
			}

			result.WriteString(fmt.Sprintf("   📁 %s:\n", envName))
			for _, tool := range tools {
				result.WriteString(fmt.Sprintf("     • %s\n", tool.ToolName))
			}
		}
	}
	result.WriteString("\n")

	// System prompt
	result.WriteString("💭 **System Prompt:**\n")
	result.WriteString(fmt.Sprintf("   %s\n", agent.Prompt))
	result.WriteString("\n")

	result.WriteString("💡 Use 'update_agent' to modify configuration or 'call_agent' to execute\n")

	return mcp.NewToolResultText(result.String()), nil
}

func (s *Server) handleUpdateAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can update agents
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get agent ID parameter
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	var agentID int64
	if _, err := fmt.Sscanf(agentIDStr, "%d", &agentID); err != nil {
		return mcp.NewToolResultError("Invalid agent_id format: must be a number"), nil
	}

	// Get existing agent
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Extract optional parameters
	name := request.GetString("name", "")
	description := request.GetString("description", "")
	prompt := request.GetString("prompt", "")
	maxSteps := int64(request.GetInt("max_steps", 0))
	schedule := request.GetString("schedule", "")
	
	toolNames := request.GetStringSlice("tool_names", []string{})
	addTools := request.GetStringSlice("add_tools", []string{})
	removeTools := request.GetStringSlice("remove_tools", []string{})

	// Update basic agent properties if provided
	updateName := name
	if updateName == "" {
		updateName = agent.Name
	}
	
	updateDescription := description
	if updateDescription == "" {
		updateDescription = agent.Description
	}
	
	updatePrompt := prompt
	if updatePrompt == "" {
		updatePrompt = agent.Prompt
	}
	
	updateMaxSteps := maxSteps
	if updateMaxSteps == 0 {
		updateMaxSteps = agent.MaxSteps
	}

	var updateSchedule *string
	if schedule != "" {
		updateSchedule = &schedule
	} else {
		updateSchedule = agent.CronSchedule
	}

	// Update agent in database
	err = s.repos.Agents.Update(agentID, updateName, updateDescription, updatePrompt, updateMaxSteps, updateSchedule, agent.ScheduleEnabled)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update agent: %v", err)), nil
	}

	// Handle tool updates
	var changes []string
	
	if len(toolNames) > 0 {
		// Replace all tools
		// First, remove all existing tools
		existingTools, err := s.repos.AgentTools.List(agentID)
		if err == nil {
			for _, tool := range existingTools {
				s.repos.AgentTools.Remove(agentID, tool.ToolName, tool.EnvironmentID)
			}
		}
		
		// Add new tools
		for _, toolName := range toolNames {
			_, err := s.repos.AgentTools.Add(agentID, toolName, agent.EnvironmentID)
			if err != nil {
				log.Printf("Failed to assign tool %s to agent %d: %v", toolName, agentID, err)
			}
		}
		changes = append(changes, fmt.Sprintf("Replaced all tools with: %v", toolNames))
	}

	if len(addTools) > 0 {
		// Add specific tools
		for _, toolName := range addTools {
			_, err := s.repos.AgentTools.Add(agentID, toolName, agent.EnvironmentID)
			if err != nil {
				log.Printf("Failed to add tool %s to agent %d: %v", toolName, agentID, err)
			}
		}
		changes = append(changes, fmt.Sprintf("Added tools: %v", addTools))
	}

	if len(removeTools) > 0 {
		// Remove specific tools
		for _, toolName := range removeTools {
			err := s.repos.AgentTools.Remove(agentID, toolName, agent.EnvironmentID)
			if err != nil {
				log.Printf("Failed to remove tool %s from agent %d: %v", toolName, agentID, err)
			}
		}
		changes = append(changes, fmt.Sprintf("Removed tools: %v", removeTools))
	}

	// Build response
	var result strings.Builder
	result.WriteString(fmt.Sprintf("✅ Agent %d (%s) updated successfully\n\n", agentID, updateName))
	
	result.WriteString("📝 **Changes Made:**\n")
	if name != "" {
		result.WriteString(fmt.Sprintf("   • Name: %s\n", name))
	}
	if description != "" {
		result.WriteString(fmt.Sprintf("   • Description: %s\n", description))
	}
	if prompt != "" {
		result.WriteString(fmt.Sprintf("   • Prompt: Updated\n"))
	}
	if maxSteps > 0 {
		result.WriteString(fmt.Sprintf("   • Max Steps: %d\n", maxSteps))
	}
	if schedule != "" {
		result.WriteString(fmt.Sprintf("   • Schedule: %s\n", schedule))
	}
	
	for _, change := range changes {
		result.WriteString(fmt.Sprintf("   • %s\n", change))
	}

	result.WriteString("\n💡 Use 'get_agent_details' to see the updated configuration\n")

	return mcp.NewToolResultText(result.String()), nil
}

// Resource handlers for read-only data access

func (s *Server) handleEnvironmentsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	environments, err := s.repos.Environments.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	// Convert to a more readable format for LLM context
	envData := make([]map[string]interface{}, len(environments))
	for i, env := range environments {
		envData[i] = map[string]interface{}{
			"id":          env.ID,
			"name":        env.Name,
			"description": env.Description,
			"created_at":  env.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	result := map[string]interface{}{
		"environments": envData,
		"total_count":  len(environments),
		"description":  "Available Station environments for organizing MCP tools and agents",
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environments data: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

func (s *Server) handleAgentsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	agents, err := s.repos.Agents.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	// Convert to a more readable format for LLM context
	agentData := make([]map[string]interface{}, len(agents))
	for i, agent := range agents {
		schedule := ""
		if agent.CronSchedule != nil {
			schedule = *agent.CronSchedule
		}
		
		agentData[i] = map[string]interface{}{
			"id":             agent.ID,
			"name":           agent.Name,
			"description":    agent.Description,
			"environment_id": agent.EnvironmentID,
			"max_steps":      agent.MaxSteps,
			"schedule":       schedule,
			"enabled":        agent.ScheduleEnabled,
			"created_at":     agent.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	result := map[string]interface{}{
		"agents":      agentData,
		"total_count": len(agents),
		"description": "Available Station agents for automated task execution",
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agents data: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

func (s *Server) handleMCPConfigsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	configs, err := s.repos.MCPConfigs.ListAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP configs: %w", err)
	}

	// Convert to a more readable format for LLM context
	configData := make([]map[string]interface{}, len(configs))
	for i, config := range configs {
		configData[i] = map[string]interface{}{
			"id":             config.ID,
			"config_name":    config.ConfigName,
			"version":        config.Version,
			"environment_id": config.EnvironmentID,
			"created_at":     config.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	result := map[string]interface{}{
		"mcp_configs": configData,
		"total_count": len(configs),
		"description": "MCP server configurations for tool discovery and execution",
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP configs data: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

func (s *Server) handleAgentDetailsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract agent ID from URI using regex
	agentID, err := s.extractIDFromURI(request.Params.URI, `station://agents/(\d+)`)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID in URI: %w", err)
	}

	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	// Get assigned tools
	assignedTools, err := s.repos.AgentTools.List(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent tools: %w", err)
	}

	// Get environment details
	environment, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	// Format tools data
	toolsData := make([]map[string]interface{}, len(assignedTools))
	for i, tool := range assignedTools {
		toolsData[i] = map[string]interface{}{
			"name":        tool.ToolName,
			"description": tool.ToolDescription,
			"server_name": tool.ServerName,
		}
	}

	schedule := ""
	if agent.CronSchedule != nil {
		schedule = *agent.CronSchedule
	}

	result := map[string]interface{}{
		"agent": map[string]interface{}{
			"id":          agent.ID,
			"name":        agent.Name,
			"description": agent.Description,
			"prompt":      agent.Prompt,
			"max_steps":   agent.MaxSteps,
			"schedule":    schedule,
			"enabled":     agent.ScheduleEnabled,
			"created_at":  agent.CreatedAt.Format("2006-01-02 15:04:05"),
		},
		"environment": map[string]interface{}{
			"id":          environment.ID,
			"name":        environment.Name,
			"description": environment.Description,
		},
		"assigned_tools": toolsData,
		"tools_count":    len(assignedTools),
		"description":    fmt.Sprintf("Complete configuration and details for agent '%s'", agent.Name),
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent details: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

func (s *Server) handleEnvironmentToolsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract environment ID from URI using regex
	envID, err := s.extractIDFromURI(request.Params.URI, `station://environments/(\d+)/tools`)
	if err != nil {
		return nil, fmt.Errorf("invalid environment ID in URI: %w", err)
	}

	// Get environment details
	environment, err := s.repos.Environments.GetByID(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment %d: %w", envID, err)
	}

	// Get available tools in this environment
	tools, err := s.repos.MCPTools.GetByEnvironmentID(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools for environment %d: %w", envID, err)
	}

	// Format tools data
	toolsData := make([]map[string]interface{}, len(tools))
	for i, tool := range tools {
		toolsData[i] = map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
			"schema":      string(tool.Schema),
		}
	}

	result := map[string]interface{}{
		"environment": map[string]interface{}{
			"id":          environment.ID,
			"name":        environment.Name,
			"description": environment.Description,
		},
		"tools":       toolsData,
		"tools_count": len(tools),
		"description": fmt.Sprintf("All MCP tools available in the '%s' environment", environment.Name),
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal environment tools: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

func (s *Server) handleAgentRunsResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	// Extract agent ID from URI using regex
	agentID, err := s.extractIDFromURI(request.Params.URI, `station://agents/(\d+)/runs`)
	if err != nil {
		return nil, fmt.Errorf("invalid agent ID in URI: %w", err)
	}

	// Get agent details for context
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent %d: %w", agentID, err)
	}

	// Get recent runs for this agent - use List method with filtering
	allRuns, err := s.repos.AgentRuns.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get runs for agent %d: %w", agentID, err)
	}

	// Filter runs for this specific agent and limit to last 50
	var agentRuns []*models.AgentRun
	for _, run := range allRuns {
		if run.AgentID == agentID {
			agentRuns = append(agentRuns, run)
		}
		if len(agentRuns) >= 50 {
			break
		}
	}

	// Format runs data
	runsData := make([]map[string]interface{}, len(agentRuns))
	for i, run := range agentRuns {
		completedAt := ""
		if run.CompletedAt != nil {
			completedAt = run.CompletedAt.Format("2006-01-02 15:04:05")
		}
		
		runsData[i] = map[string]interface{}{
			"id":           run.ID,
			"status":       run.Status,
			"task":         run.Task,
			"response":     run.FinalResponse,
			"steps_taken":  run.StepsTaken,
			"started_at":   run.StartedAt.Format("2006-01-02 15:04:05"),
			"completed_at": completedAt,
		}
	}

	result := map[string]interface{}{
		"agent": map[string]interface{}{
			"id":   agent.ID,
			"name": agent.Name,
		},
		"runs":        runsData,
		"runs_count":  len(agentRuns),
		"description": fmt.Sprintf("Recent execution history for agent '%s'", agent.Name),
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent runs: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      request.Params.URI,
			MIMEType: "application/json",
			Text:     string(jsonData),
		},
	}, nil
}

// Helper method to extract ID from URI using regex
func (s *Server) extractIDFromURI(uri, pattern string) (int64, error) {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(uri)
	if len(matches) < 2 {
		return 0, fmt.Errorf("no ID found in URI: %s", uri)
	}
	
	id, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ID format: %s", matches[1])
	}
	
	return id, nil
}