package mcp_agents

import (
	"context"
	"fmt"
	"log"
	"station/internal/db/repositories"
	"station/internal/services"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DynamicAgentServer manages a dynamic MCP server that serves database agents as individual tools
type DynamicAgentServer struct {
	repos           *repositories.Repositories
	agentService    services.AgentServiceInterface
	mcpServer       *server.MCPServer
	httpServer      *server.StreamableHTTPServer
	localMode       bool
	environmentName string
}

// NewDynamicAgentServer creates a new dynamic agent MCP server with environment filtering
func NewDynamicAgentServer(repos *repositories.Repositories, agentService services.AgentServiceInterface, localMode bool, environmentName string) *DynamicAgentServer {
	return &DynamicAgentServer{
		repos:           repos,
		agentService:    agentService,
		localMode:       localMode,
		environmentName: environmentName,
	}
}

// Start starts the dynamic MCP server on the specified port
func (das *DynamicAgentServer) Start(ctx context.Context, port int) error {
	// Create a new MCP server
	das.mcpServer = server.NewMCPServer(
		"Station Dynamic Agents",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Load agents as tools
	if err := das.loadAgentsAsTools(); err != nil {
		return err
	}

	// Start the HTTP server
	das.httpServer = server.NewStreamableHTTPServer(das.mcpServer)
	addr := fmt.Sprintf(":%d", port)
	return das.httpServer.Start(addr)
}

// loadAgentsAsTools loads all agents from the specified environment as MCP tools
func (das *DynamicAgentServer) loadAgentsAsTools() error {
	// Get environment by name
	environment, err := das.repos.Environments.GetByName(das.environmentName)
	if err != nil {
		log.Printf("Failed to find environment '%s': %v", das.environmentName, err)
		return err
	}

	// Get agents from the specified environment
	agents, err := das.repos.Agents.ListByEnvironment(environment.ID)
	if err != nil {
		log.Printf("Failed to load agents from environment '%s': %v", das.environmentName, err)
		return err
	}

	log.Printf("🤖 Loading %d agents from environment '%s' as MCP tools", len(agents), das.environmentName)

	// Register each agent as an MCP tool
	for _, agent := range agents {
		toolName := "agent_" + agent.Name
		log.Printf("  📋 Registering agent '%s' as tool '%s'", agent.Name, toolName)

		// Create tool for this agent using the correct mcp package
		tool := mcp.NewTool(toolName,
			mcp.WithDescription("Execute agent: "+agent.Name),
			mcp.WithString("input", mcp.Required(), mcp.Description("Task or input to provide to the agent")),
			mcp.WithObject("variables", mcp.Description("Variables for dotprompt rendering (optional)")),
		)

		// Set handler for this agent tool
		agentID := agent.ID // capture for closure
		handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract parameters
			input := request.GetString("input", "")

			// Get variables if provided
			variables := make(map[string]interface{})
			if request.Params.Arguments != nil {
				if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
					if variablesArg, ok := argsMap["variables"]; ok {
						if varsMap, ok := variablesArg.(map[string]interface{}); ok {
							variables = varsMap
						}
					}
				}
			}

			// Execute the agent using the agent service
			result, err := das.agentService.ExecuteAgent(ctx, int64(agentID), input, variables)
			if err != nil {
				return mcp.NewToolResultError("Error executing agent: " + err.Error()), nil
			}

			return mcp.NewToolResultText(result.Content), nil
		}

		// Register the tool with the MCP server
		das.mcpServer.AddTool(tool, handler)
	}

	return nil
}

// Shutdown gracefully shuts down the dynamic MCP server
func (das *DynamicAgentServer) Shutdown(ctx context.Context) error {
	if das.httpServer != nil {
		return das.httpServer.Shutdown(ctx)
	}
	return nil
}
