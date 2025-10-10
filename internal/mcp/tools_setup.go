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
		mcp.WithString("input_schema", mcp.Description("JSON schema for custom input variables (optional)")),
		mcp.WithString("output_schema", mcp.Description("JSON schema for output format (optional)")),
		mcp.WithString("output_schema_preset", mcp.Description("Predefined schema preset (e.g., 'finops') - alternative to output_schema")),
		mcp.WithString("app", mcp.Description("CloudShip data ingestion app classification (optional, must be provided with app_type)")),
		mcp.WithString("app_type", mcp.Description("CloudShip data ingestion app_type classification (optional, must be provided with app)")),
	)
	s.mcpServer.AddTool(createAgentTool, s.handleCreateAgent)

	// Update agent tool
	updateAgentTool := mcp.NewTool("update_agent",
		mcp.WithDescription("Update an existing agent's configuration. Note: To manage tools, use add_tool/remove_tool instead."),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to update")),
		mcp.WithString("name", mcp.Description("New name for the agent")),
		mcp.WithString("description", mcp.Description("New description for the agent")),
		mcp.WithString("prompt", mcp.Description("New system prompt for the agent")),
		mcp.WithNumber("max_steps", mcp.Description("New maximum steps for the agent")),
		mcp.WithBoolean("enabled", mcp.Description("Whether the agent should be enabled")),
		mcp.WithString("output_schema", mcp.Description("JSON schema for output format (optional)")),
		mcp.WithString("output_schema_preset", mcp.Description("Predefined schema preset (e.g., 'finops') - alternative to output_schema")),
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
		mcp.WithObject("variables", mcp.Description("Variables for dotprompt rendering (e.g., {\"my_folder\": \"/tmp\", \"target_file\": \"log.txt\"})")),
	)
	s.mcpServer.AddTool(callAgentTool, s.handleCallAgent)

	// Agent schema discovery tool
	getAgentSchemaTool := mcp.NewTool("get_agent_schema",
		mcp.WithDescription("Get input schema and available variables for an agent's dotprompt template"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to get schema for")),
	)
	s.mcpServer.AddTool(getAgentSchemaTool, s.handleGetAgentSchema)

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
		mcp.WithDescription("List available MCP tools with pagination support"),
		mcp.WithString("environment_id", mcp.Description("Filter by environment ID")),
		mcp.WithString("search", mcp.Description("Search term to filter tools")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of tools to return (default: 50)")),
		mcp.WithNumber("offset", mcp.Description("Number of tools to skip for pagination (default: 0)")),
	)
	s.mcpServer.AddTool(listToolsTool, s.handleListTools)

	listPromptsTool := mcp.NewTool("list_prompts",
		mcp.WithDescription("List available MCP prompts"),
		mcp.WithString("category", mcp.Description("Filter by prompt category")),
	)
	s.mcpServer.AddTool(listPromptsTool, s.handleListPrompts)

	getPromptTool := mcp.NewTool("get_prompt",
		mcp.WithDescription("Get the content of a specific MCP prompt"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the prompt to retrieve")),
	)
	s.mcpServer.AddTool(getPromptTool, s.handleGetPrompt)

	// Agent and environment listing
	listEnvironmentsTool := mcp.NewTool("list_environments",
		mcp.WithDescription("List all available environments"),
	)
	s.mcpServer.AddTool(listEnvironmentsTool, s.handleListEnvironments)

	listAgentsTool := mcp.NewTool("list_agents",
		mcp.WithDescription("List all agents with pagination support"),
		mcp.WithString("environment_id", mcp.Description("Filter by environment ID")),
		mcp.WithBoolean("enabled_only", mcp.Description("Show only enabled agents (default: false)")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of agents to return (default: 50)")),
		mcp.WithNumber("offset", mcp.Description("Number of agents to skip for pagination (default: 0)")),
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

	exportAgentsTool := mcp.NewTool("export_agents",
		mcp.WithDescription("Export all agents in an environment to dotprompt format"),
		mcp.WithString("environment_id", mcp.Description("Environment ID to export agents from (defaults to all environments)")),
		mcp.WithString("output_directory", mcp.Description("Directory to export agents to (defaults to current environment's agents directory)")),
		mcp.WithBoolean("enabled_only", mcp.Description("Export only enabled agents (default: false)")),
	)
	s.mcpServer.AddTool(exportAgentsTool, s.handleExportAgents)

	// Agent run management tools
	listRunsTool := mcp.NewTool("list_runs",
		mcp.WithDescription("List agent execution runs with pagination support"),
		mcp.WithString("agent_id", mcp.Description("Filter by specific agent ID")),
		mcp.WithString("status", mcp.Description("Filter by run status (success, error, running)")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of runs to return (default: 50)")),
		mcp.WithNumber("offset", mcp.Description("Number of runs to skip for pagination (default: 0)")),
	)
	s.mcpServer.AddTool(listRunsTool, s.handleListRuns)

	inspectRunTool := mcp.NewTool("inspect_run",
		mcp.WithDescription("Get detailed information about a specific agent run"),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("ID of the run to inspect")),
		mcp.WithBoolean("verbose", mcp.Description("Include detailed tool calls and execution steps (default: true)")),
	)
	s.mcpServer.AddTool(inspectRunTool, s.handleInspectRun)

	// Environment management tools
	createEnvironmentTool := mcp.NewTool("create_environment",
		mcp.WithDescription("Create a new environment with file-based configuration"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the environment")),
		mcp.WithString("description", mcp.Description("Description of the environment")),
	)
	s.mcpServer.AddTool(createEnvironmentTool, s.handleCreateEnvironment)

	deleteEnvironmentTool := mcp.NewTool("delete_environment",
		mcp.WithDescription("Delete an environment and all its associated data"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the environment to delete")),
		mcp.WithBoolean("confirm", mcp.Required(), mcp.Description("Confirmation flag - must be true to proceed")),
	)
	s.mcpServer.AddTool(deleteEnvironmentTool, s.handleDeleteEnvironment)

	// Unified Bundle Management Tool (API-compatible)
	createBundleFromEnvTool := mcp.NewTool("create_bundle_from_environment",
		mcp.WithDescription("Create an API-compatible bundle (.tar.gz) from a Station environment. The bundle can be installed via the Station Bundle API or UI."),
		mcp.WithString("environmentName", mcp.Required(), mcp.Description("Name of the environment to bundle (e.g., 'default', 'production')")),
		mcp.WithString("outputPath", mcp.Description("Output path for the bundle file (optional, defaults to <environment>.tar.gz)")),
	)
	s.mcpServer.AddTool(createBundleFromEnvTool, s.handleCreateBundleFromEnvironment)

	// MCP Server Management Tools
	listMCPServersTool := mcp.NewTool("list_mcp_servers_for_environment",
		mcp.WithDescription("List all MCP servers configured for an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
	)
	s.mcpServer.AddTool(listMCPServersTool, s.handleListMCPServersForEnvironment)

	addMCPServerTool := mcp.NewTool("add_mcp_server_to_environment",
		mcp.WithDescription("Add an MCP server to an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
		mcp.WithString("server_name", mcp.Required(), mcp.Description("Name of the MCP server")),
		mcp.WithString("command", mcp.Required(), mcp.Description("Command to execute the MCP server")),
		mcp.WithString("description", mcp.Description("Description of the MCP server")),
		mcp.WithArray("args", mcp.Description("Command line arguments for the MCP server"), mcp.WithStringItems()),
		mcp.WithObject("env", mcp.Description("Environment variables for the MCP server")),
	)
	s.mcpServer.AddTool(addMCPServerTool, s.handleAddMCPServerToEnvironment)

	updateMCPServerTool := mcp.NewTool("update_mcp_server_in_environment",
		mcp.WithDescription("Update an MCP server configuration in an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
		mcp.WithString("server_name", mcp.Required(), mcp.Description("Name of the MCP server to update")),
		mcp.WithString("command", mcp.Required(), mcp.Description("Command to execute the MCP server")),
		mcp.WithString("description", mcp.Description("Description of the MCP server")),
		mcp.WithArray("args", mcp.Description("Command line arguments for the MCP server"), mcp.WithStringItems()),
		mcp.WithObject("env", mcp.Description("Environment variables for the MCP server")),
	)
	s.mcpServer.AddTool(updateMCPServerTool, s.handleUpdateMCPServerInEnvironment)

	deleteMCPServerTool := mcp.NewTool("delete_mcp_server_from_environment",
		mcp.WithDescription("Delete an MCP server from an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
		mcp.WithString("server_name", mcp.Required(), mcp.Description("Name of the MCP server to delete")),
	)
	s.mcpServer.AddTool(deleteMCPServerTool, s.handleDeleteMCPServerFromEnvironment)

	// Raw MCP Config Management Tools
	getRawMCPConfigTool := mcp.NewTool("get_raw_mcp_config",
		mcp.WithDescription("Get the raw template.json content for an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
	)
	s.mcpServer.AddTool(getRawMCPConfigTool, s.handleGetRawMCPConfig)

	updateRawMCPConfigTool := mcp.NewTool("update_raw_mcp_config",
		mcp.WithDescription("Update the raw template.json content for an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Raw JSON content for template.json")),
	)
	s.mcpServer.AddTool(updateRawMCPConfigTool, s.handleUpdateRawMCPConfig)

	// Environment File Config Management Tools
	getEnvFileConfigTool := mcp.NewTool("get_environment_file_config",
		mcp.WithDescription("Get all file-based configuration for an environment"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
	)
	s.mcpServer.AddTool(getEnvFileConfigTool, s.handleGetEnvironmentFileConfig)

	updateEnvFileConfigTool := mcp.NewTool("update_environment_file_config",
		mcp.WithDescription("Update a specific file in environment configuration"),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment")),
		mcp.WithString("filename", mcp.Required(), mcp.Description("Name of the file to update (variables.yml or template.json)")),
		mcp.WithString("content", mcp.Required(), mcp.Description("Content to write to the file")),
	)
	s.mcpServer.AddTool(updateEnvFileConfigTool, s.handleUpdateEnvironmentFileConfig)

	// Demo Bundle Tools
	listDemoBundlesTool := mcp.NewTool("list_demo_bundles",
		mcp.WithDescription("List all available embedded demo bundles for trying Station features"),
	)
	s.mcpServer.AddTool(listDemoBundlesTool, s.handleListDemoBundles)

	installDemoBundleTool := mcp.NewTool("install_demo_bundle",
		mcp.WithDescription("Install an embedded demo bundle to a new environment"),
		mcp.WithString("bundle_id", mcp.Required(), mcp.Description("ID of the demo bundle to install (e.g., 'finops-demo')")),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name for the new environment where demo will be installed")),
	)
	s.mcpServer.AddTool(installDemoBundleTool, s.handleInstallDemoBundle)

	log.Printf("MCP tools setup complete - %d tools registered", 33)
}