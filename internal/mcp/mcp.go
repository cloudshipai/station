package mcp

import (
	"context"
	"fmt"
	"log"
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