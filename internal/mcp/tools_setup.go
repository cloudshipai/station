package mcp

import (
	"log"

	"github.com/mark3labs/mcp-go/mcp"
)

// setupTools initializes all MCP tools for operations with side effects
func (s *Server) setupTools() {
	// Agent management tools (CRUD operations)
	createAgentTool := mcp.NewTool("create_agent",
		mcp.WithDescription("Create a new AI agent with specified configuration"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the agent")),
		mcp.WithString("description", mcp.Required(), mcp.Description("Description of what the agent does")),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("System prompt for the agent")),
		mcp.WithString("environment_id", mcp.Required(), mcp.Description("Environment ID where the agent will run")),
		mcp.WithNumber("max_steps", mcp.Description("Maximum steps the agent can take (default: 5)")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the agent is enabled (default: true)")),
		mcp.WithArray("tool_names", mcp.Description("List of tool names to assign to the agent"), mcp.WithStringItems()),
	)
	s.mcpServer.AddTool(createAgentTool, s.handleCreateAgent)

	// Update agent tool
	updateAgentTool := mcp.NewTool("update_agent",
		mcp.WithDescription("Update an existing agent's configuration"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to update")),
		mcp.WithString("name", mcp.Description("New name for the agent")),
		mcp.WithString("description", mcp.Description("New description for the agent")),
		mcp.WithString("prompt", mcp.Description("New system prompt for the agent")),
		mcp.WithNumber("max_steps", mcp.Description("New maximum steps for the agent")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the agent should be enabled")),
		mcp.WithArray("tool_names", mcp.Description("New list of tool names to assign to the agent"), mcp.WithStringItems()),
	)
	s.mcpServer.AddTool(updateAgentTool, s.handleUpdateAgent)

	// Agent execution tool with advanced options
	callAgentTool := mcp.NewTool("call_agent",
		mcp.WithDescription("Execute an AI agent with advanced options and monitoring"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to execute")),
		mcp.WithString("task", mcp.Required(), mcp.Description("Task or input to provide to the agent")),
		mcp.WithBoolean("async", mcp.Description("Execute asynchronously and return run ID (default: false)")),
		mcp.WithNumber("timeout", mcp.Description("Execution timeout in seconds (default: 300)")),
		mcp.WithBoolean("store_run", mcp.Description("Store execution results in runs history (default: true)")),
		mcp.WithObject("context", mcp.Description("Additional context to provide to the agent")),
	)
	s.mcpServer.AddTool(callAgentTool, s.handleCallAgent)

	// Agent deletion tool
	deleteAgentTool := mcp.NewTool("delete_agent",
		mcp.WithDescription("Delete an AI agent"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to delete")),
	)
	s.mcpServer.AddTool(deleteAgentTool, s.handleDeleteAgent)

	// Tool and configuration management
	discoverToolsTool := mcp.NewTool("discover_tools",
		mcp.WithDescription("Discover available MCP tools from configurations"),
		mcp.WithString("config_id", mcp.Description("Specific MCP config ID to discover tools from")),
		mcp.WithString("environment_id", mcp.Description("Environment ID to filter tools")),
	)
	s.mcpServer.AddTool(discoverToolsTool, s.handleDiscoverTools)

	// List operations
	listMCPConfigsTool := mcp.NewTool("list_mcp_configs",
		mcp.WithDescription("List all MCP configurations"),
		mcp.WithString("environment_id", mcp.Description("Filter by environment ID")),
	)
	s.mcpServer.AddTool(listMCPConfigsTool, s.handleListMCPConfigs)

	listToolsTool := mcp.NewTool("list_tools",
		mcp.WithDescription("List available MCP tools"),
		mcp.WithString("environment_id", mcp.Description("Filter by environment ID")),
		mcp.WithString("search", mcp.Description("Search term to filter tools")),
	)
	s.mcpServer.AddTool(listToolsTool, s.handleListTools)

	listPromptsTool := mcp.NewTool("list_prompts",
		mcp.WithDescription("List available MCP prompts"),
		mcp.WithString("category", mcp.Description("Filter by prompt category")),
	)
	s.mcpServer.AddTool(listPromptsTool, s.handleListPrompts)

	// Agent and environment listing
	listEnvironmentsTool := mcp.NewTool("list_environments",
		mcp.WithDescription("List all available environments"),
	)
	s.mcpServer.AddTool(listEnvironmentsTool, s.handleListEnvironments)

	listAgentsTool := mcp.NewTool("list_agents",
		mcp.WithDescription("List all agents"),
		mcp.WithString("environment_id", mcp.Description("Filter by environment ID")),
		mcp.WithBoolean("enabled_only", mcp.Description("Show only enabled agents (default: false)")),
	)
	s.mcpServer.AddTool(listAgentsTool, s.handleListAgents)

	getAgentDetailsTool := mcp.NewTool("get_agent_details",
		mcp.WithDescription("Get detailed information about a specific agent"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
	)
	s.mcpServer.AddTool(getAgentDetailsTool, s.handleGetAgentDetails)

	// Agent management tools for fine-grained control
	updateAgentPromptTool := mcp.NewTool("update_agent_prompt",
		mcp.WithDescription("Update an agent's system prompt"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to update")),
		mcp.WithString("prompt", mcp.Required(), mcp.Description("New system prompt for the agent")),
	)
	s.mcpServer.AddTool(updateAgentPromptTool, s.handleUpdateAgentPrompt)

	addToolTool := mcp.NewTool("add_tool",
		mcp.WithDescription("Add a tool to an agent"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the tool to add")),
	)
	s.mcpServer.AddTool(addToolTool, s.handleAddTool)

	removeToolTool := mcp.NewTool("remove_tool",
		mcp.WithDescription("Remove a tool from an agent"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the tool to remove")),
	)
	s.mcpServer.AddTool(removeToolTool, s.handleRemoveTool)

	exportAgentTool := mcp.NewTool("export_agent",
		mcp.WithDescription("Export agent configuration to dotprompt format"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to export")),
		mcp.WithString("output_path", mcp.Description("Optional output file path (defaults to agent name)")),
	)
	s.mcpServer.AddTool(exportAgentTool, s.handleExportAgent)

	log.Printf("MCP tools setup complete - %d tools registered", 15)
}