package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	localMode        bool
}

type ToolDiscoveryService struct {
	db              db.Database
	mcpConfigSvc    *services.MCPConfigService
	discoveredTools map[string][]mcp.Tool // configID -> tools
}

func NewToolDiscoveryService(database db.Database, mcpConfigSvc *services.MCPConfigService) *ToolDiscoveryService {
	return &ToolDiscoveryService{
		db:              database,
		mcpConfigSvc:    mcpConfigSvc,
		discoveredTools: make(map[string][]mcp.Tool),
	}
}

func (t *ToolDiscoveryService) DiscoverTools(ctx context.Context, configID int64) error {
	// Get MCP config
	_, err := t.mcpConfigSvc.GetDecryptedConfig(configID)
	if err != nil {
		return fmt.Errorf("failed to get MCP config: %w", err)
	}

	// TODO: Connect to external MCP server and discover tools
	// For now, create mock tools based on config
	configIDStr := fmt.Sprintf("%d", configID)
	mockTools := []mcp.Tool{
		mcp.NewTool(fmt.Sprintf("tool_%s_1", configIDStr),
			mcp.WithDescription(fmt.Sprintf("Tool from config %s", configIDStr)),
		),
		mcp.NewTool(fmt.Sprintf("tool_%s_2", configIDStr),
			mcp.WithDescription(fmt.Sprintf("Another tool from config %s", configIDStr)),
		),
	}

	t.discoveredTools[configIDStr] = mockTools
	log.Printf("Discovered %d tools for config %d", len(mockTools), configID)
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

	toolDiscoverySvc := NewToolDiscoveryService(database, mcpConfigSvc)
	authService := auth.NewAuthService(repos)

	// Create custom HTTP server with conditional authentication middleware
	mux := http.NewServeMux()
	
	var handler http.Handler = mux
	if !localMode {
		// Apply authentication middleware only in server mode
		handler = authService.RequireAuth(mux)
	}
	
	// Create streamable HTTP server wrapping the MCP server
	httpServer := server.NewStreamableHTTPServer(
		mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithLogger(nil), // Use default logger
		server.WithStreamableHTTPServer(&http.Server{
			Handler: handler,
		}),
	)

	server := &Server{
		mcpServer:        mcpServer,
		httpServer:       httpServer,
		db:               database,
		mcpConfigSvc:     mcpConfigSvc,
		toolDiscoverySvc: toolDiscoverySvc,
		agentService:     agentService,
		authService:      authService,
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

	// Add a tool to call tools from discovered MCP servers
	callToolTool := mcp.NewTool("call_mcp_tool",
		mcp.WithDescription("Call a tool from a discovered MCP server"),
		mcp.WithString("config_id",
			mcp.Required(),
			mcp.Description("ID of the MCP configuration"),
		),
		mcp.WithString("tool_name",
			mcp.Required(),
			mcp.Description("Name of the tool to call"),
		),
		mcp.WithObject("arguments",
			mcp.Description("Arguments to pass to the tool"),
		),
	)

	s.mcpServer.AddTool(callToolTool, s.handleCallMCPTool)

	// Add a tool to create AI agents
	createAgentTool := mcp.NewTool("create_agent",
		mcp.WithDescription("Create a new AI agent with specified configuration"),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("Name of the agent"),
		),
		mcp.WithString("description",
			mcp.Required(),
			mcp.Description("Description of the agent"),
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
		mcp.WithArray("assigned_tools",
			mcp.Description("List of tool names to assign to the agent"),
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
}

func (s *Server) handleListMCPConfigs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can list MCP configs
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// TODO: Get list of MCP configs from database
	configs := []string{"config1", "config2", "config3"} // Mock data
	
	return mcp.NewToolResultText(fmt.Sprintf("Available MCP configs: %v", configs)), nil
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

func (s *Server) handleCallMCPTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// In server mode, only admin users can call MCP tools
	if err := s.requireAdminInServerMode(ctx); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	configID, err := request.RequireString("config_id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	toolName, err := request.RequireString("tool_name")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// TODO: Create dynamic MCP client and call the external tool
	// For now, return mock response
	return mcp.NewToolResultText(fmt.Sprintf("Called tool %s from config %s", toolName, configID)), nil
}

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
	assignedTools := request.GetStringSlice("assigned_tools", []string{})

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

	// Return success response with agent details
	result := fmt.Sprintf(`Agent created successfully:
ID: %d
Name: %s
Description: %s
Environment ID: %d
Max Steps: %d
Assigned Tools: %v
Created By: %d`, 
		agent.ID, agent.Name, agent.Description, agent.EnvironmentID, 
		agent.MaxSteps, assignedTools, agent.CreatedBy)

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

func (s *Server) Start(ctx context.Context, port int) error {
	log.Printf("Starting MCP server using streamable HTTP transport on port %d at /mcp", port)
	
	// Start the streamable HTTP server
	go func() {
		addr := fmt.Sprintf(":%d", port)
		if err := s.httpServer.Start(addr); err != nil {
			log.Printf("MCP server error: %v", err)
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	
	// Graceful shutdown
	log.Println("MCP server shutting down...")
	return s.httpServer.Shutdown(context.Background())
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