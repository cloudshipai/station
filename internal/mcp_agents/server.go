package mcp_agents

import (
	"context"
	"fmt"
	"log"

	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
	"station/pkg/schema"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// DynamicAgentServer serves agents from the database as individual MCP tools
type DynamicAgentServer struct {
	repos           *repositories.Repositories
	agentService    services.AgentServiceInterface
	mcpServer       *server.MCPServer
	httpServer      *server.StreamableHTTPServer
	localMode       bool
	environmentName string
}

// NewDynamicAgentServer creates a new dynamic agent MCP server
func NewDynamicAgentServer(repos *repositories.Repositories, agentService services.AgentServiceInterface, localMode bool, environmentName string) *DynamicAgentServer {
	mcpServer := server.NewMCPServer("station-agents", "1.0.0")
	httpServer := server.NewStreamableHTTPServer(mcpServer)
	
	das := &DynamicAgentServer{
		repos:           repos,
		agentService:    agentService,
		mcpServer:       mcpServer,
		httpServer:      httpServer,
		localMode:       localMode,
		environmentName: environmentName,
	}
	
	// Load agents and register as tools
	if err := das.loadAgentsAsTools(); err != nil {
		log.Printf("Warning: Failed to load agents as tools: %v", err)
	}
	
	return das
}

// Start starts the dynamic agent MCP server on port 3031
func (das *DynamicAgentServer) Start(ctx context.Context) error {
	addr := ":3031"
	log.Printf("Starting Dynamic Agents MCP server on %s", addr)
	log.Printf("Dynamic Agents MCP endpoint available at http://localhost:3031/mcp")
	
	if err := das.httpServer.Start(addr); err != nil {
		return fmt.Errorf("Dynamic Agents MCP server error: %w", err)
	}
	
	return nil
}

// loadAgentsAsTools queries the database for agents and registers each as an MCP tool
func (das *DynamicAgentServer) loadAgentsAsTools() error {
	// Get environment by name
	environment, err := das.repos.Environments.GetByName(das.environmentName)
	if err != nil {
		return fmt.Errorf("failed to find environment '%s': %w", das.environmentName, err)
	}
	
	// Get agents from the specified environment
	agents, err := das.repos.Agents.ListByEnvironment(environment.ID)
	if err != nil {
		return fmt.Errorf("failed to list agents for environment '%s': %w", das.environmentName, err)
	}
	
	log.Printf("Loading %d agents from environment '%s' as MCP tools", len(agents), das.environmentName)
	
	for _, agent := range agents {
		if err := das.registerAgentAsTool(agent); err != nil {
			log.Printf("Warning: Failed to register agent %s as tool: %v", agent.Name, err)
			continue
		}
		log.Printf("✅ Registered agent '%s' as MCP tool", agent.Name)
	}
	
	return nil
}

// registerAgentAsTool registers a single agent as an MCP tool
func (das *DynamicAgentServer) registerAgentAsTool(agent *models.Agent) error {
	// Create tool parameters based on agent's input schema
	var toolParams []mcp.ToolOption
	toolParams = append(toolParams, mcp.WithDescription(agent.Description))
	
	// Handle input schema
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		// Parse the JSON schema and add parameters
		variables, err := schema.ExtractVariablesFromSchema(*agent.InputSchema)
		if err != nil {
			log.Printf("Warning: Failed to extract variables from schema for agent %s: %v", agent.Name, err)
			// Fall back to basic userInput
			toolParams = append(toolParams, mcp.WithString("userInput", mcp.Required(), mcp.Description("The main task for the agent")))
		} else {
			// Add each schema variable as a tool parameter
			for _, variable := range variables {
				if variable == "userInput" {
					toolParams = append(toolParams, mcp.WithString("userInput", mcp.Required(), mcp.Description("The main task for the agent")))
				} else {
					// For now, treat all other variables as optional strings
					// TODO: Parse the schema more deeply to get types, required fields, enums, etc.
					toolParams = append(toolParams, mcp.WithString(variable, mcp.Description(fmt.Sprintf("Custom parameter: %s", variable))))
				}
			}
		}
	} else {
		// Default to just userInput for agents without custom schema
		toolParams = append(toolParams, mcp.WithString("userInput", mcp.Required(), mcp.Description("The main task for the agent")))
	}
	
	// Create the tool
	tool := mcp.NewTool(agent.Name, toolParams...)
	
	// Register the tool with a handler that executes the agent
	das.mcpServer.AddTool(tool, das.createAgentHandler(agent))
	
	return nil
}

// createAgentHandler creates a handler function for executing a specific agent
func (das *DynamicAgentServer) createAgentHandler(agent *models.Agent) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	agentName := agent.Name // Capture the agent name for lookup
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Look up the agent by name to get fresh data
		currentAgent, err := das.repos.Agents.GetByName(agentName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Agent '%s' not found: %v", agentName, err)), nil
		}
		
		// Extract userInput (required)
		userInput, err := request.RequireString("userInput")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Missing required parameter 'userInput': %v", err)), nil
		}
		
		// Extract custom variables if agent has input schema
		var userVariables map[string]interface{}
		
		if currentAgent.InputSchema != nil && *currentAgent.InputSchema != "" {
			userVariables = make(map[string]interface{})
			
			// Extract all schema variables
			variables, err := schema.ExtractVariablesFromSchema(*currentAgent.InputSchema)
			if err != nil {
				log.Printf("Warning: Failed to extract variables for agent %s: %v", currentAgent.Name, err)
			} else {
				// Get values for each variable from the request
				for _, variable := range variables {
					if variable == "userInput" {
						// userInput is handled separately above
						continue
					}
					
					// Get the parameter value (will be empty string if not provided)
					if value := request.GetString(variable, ""); value != "" {
						userVariables[variable] = value
					}
				}
			}
		}
		
		log.Printf("Executing agent '%s' (ID: %d) with userInput: %s, variables: %+v", currentAgent.Name, currentAgent.ID, userInput, userVariables)
		
		// Execute the agent using the existing service layer
		response, err := das.agentService.ExecuteAgent(ctx, currentAgent.ID, userInput, userVariables)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Agent execution failed: %v", err)), nil
		}
		
		if response == nil {
			return mcp.NewToolResultError("Agent execution returned no response"), nil
		}
		
		// Return the agent's response
		result := map[string]interface{}{
			"success":      true,
			"agent_name":   currentAgent.Name,
			"agent_id":     currentAgent.ID,
			"response":     response.Content,
			"user_input":   userInput,
		}
		
		if len(userVariables) > 0 {
			result["variables"] = userVariables
		}
		
		return mcp.NewToolResultText(response.Content), nil
	}
}

// RefreshAgents reloads agents from database and updates the available tools
func (das *DynamicAgentServer) RefreshAgents() error {
	// TODO: Implement tool removal and re-registration
	// For now, this would require restarting the server
	log.Printf("RefreshAgents called - server restart required for now")
	return nil
}