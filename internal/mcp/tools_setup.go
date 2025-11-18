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
		mcp.WithString("output_schema_preset", mcp.Description("Predefined schema preset (e.g., 'finops-inventory', 'security-investigations') - alternative to output_schema. Available presets: finops (inventory, investigations, opportunities, projections, events), security (inventory, investigations, opportunities, projections, events), deployments (inventory, investigations, opportunities, projections, events)")),
		mcp.WithString("app", mcp.Description("CloudShip data ingestion app classification for structured data routing (optional, must be provided with app_type). Valid values: 'finops', 'security', 'deployments'. Auto-populated from preset if not provided. Requires output_schema or output_schema_preset to enable structured data ingestion.")),
		mcp.WithString("app_type", mcp.Description("CloudShip data ingestion app_type classification for data categorization (optional, must be provided with app). Valid values: 'inventory', 'investigations', 'opportunities', 'projections', 'events'. Auto-populated from preset if not provided. Defines the type of operational data this agent generates.")),
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
		mcp.WithDescription("Add a tool to an agent. For MCP tools from servers, use tool name directly (e.g., '__get_cost_and_usage'). For agent tools, use format '__agent_<agent-name>' (e.g., '__agent_finops-analyst' to add the finops-analyst agent as a callable tool)."),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the tool to add")),
	)
	s.mcpServer.AddTool(addToolTool, s.handleAddTool)

	addAgentTool := mcp.NewTool("add_agent_as_tool",
		mcp.WithDescription("Add another agent as a callable tool to create multi-agent hierarchies. The child agent will be available as '__agent_<name>' tool."),
		mcp.WithString("parent_agent_id", mcp.Required(), mcp.Description("ID of the parent agent that will call the child")),
		mcp.WithString("child_agent_id", mcp.Required(), mcp.Description("ID of the child agent to add as a tool")),
	)
	s.mcpServer.AddTool(addAgentTool, s.handleAddAgentAsTool)

	removeAgentTool := mcp.NewTool("remove_agent_as_tool",
		mcp.WithDescription("Remove a child agent from a parent agent's callable tools, breaking the multi-agent hierarchy link."),
		mcp.WithString("parent_agent_id", mcp.Required(), mcp.Description("ID of the parent agent")),
		mcp.WithString("child_agent_id", mcp.Required(), mcp.Description("ID of the child agent to remove")),
	)
	s.mcpServer.AddTool(removeAgentTool, s.handleRemoveAgentAsTool)

	removeToolTool := mcp.NewTool("remove_tool",
		mcp.WithDescription("Remove a tool from an agent"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
		mcp.WithString("tool_name", mcp.Required(), mcp.Description("Name of the tool to remove")),
	)
	s.mcpServer.AddTool(removeToolTool, s.handleRemoveTool)

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

	// Faker MCP Server Creation Tool
	fakerCreateStandaloneTool := mcp.NewTool("faker_create_standalone",
		mcp.WithDescription("Create a standalone faker with custom tools and AI-generated data. Define your own tool schemas and let AI generate realistic responses based on your goal/instruction. Uses Station's global AI configuration (STN_AI_PROVIDER, STN_AI_MODEL)."),
		mcp.WithString("environment_name", mcp.Required(), mcp.Description("Name of the environment where faker will be created")),
		mcp.WithString("faker_name", mcp.Required(), mcp.Description("Name for the faker (e.g., 'prometheus-metrics', 'datadog-apm')")),
		mcp.WithString("description", mcp.Required(), mcp.Description("Description of what this faker provides")),
		mcp.WithString("goal", mcp.Required(), mcp.Description("Goal/instruction for AI data generation (e.g., 'Generate realistic Prometheus metrics for microservices')")),
		mcp.WithString("tools", mcp.Description("JSON array of tool definitions with schemas (see faker-config.schema.json). If not provided, AI will suggest tools based on goal.")),
		mcp.WithBoolean("persist", mcp.Description("Persist faker configuration to template.json for long-term use (default: true)")),
		mcp.WithBoolean("auto_sync", mcp.Description("Automatically sync environment after creating faker (default: true)")),
		mcp.WithBoolean("debug", mcp.Description("Enable debug logging for faker (default: false)")),
	)
	s.mcpServer.AddTool(fakerCreateStandaloneTool, s.handleFakerCreateStandalone)

	// Raw MCP Config Management Tools
	// Benchmark Tools
	evaluateBenchmarkTool := mcp.NewTool("evaluate_benchmark",
		mcp.WithDescription("Evaluate an agent run asynchronously using LLM-as-judge metrics. Returns a task ID to check status later."),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("ID of the completed agent run to evaluate")),
	)
	s.mcpServer.AddTool(evaluateBenchmarkTool, s.handleEvaluateBenchmark)

	getBenchmarkStatusTool := mcp.NewTool("get_benchmark_status",
		mcp.WithDescription("Check the status of a benchmark evaluation task"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("ID of the benchmark task")),
	)
	s.mcpServer.AddTool(getBenchmarkStatusTool, s.handleGetBenchmarkStatus)

	listBenchmarkResultsTool := mcp.NewTool("list_benchmark_results",
		mcp.WithDescription("List benchmark evaluation results"),
		mcp.WithString("run_id", mcp.Description("Filter by specific run ID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of results to return (default: 10)")),
	)
	s.mcpServer.AddTool(listBenchmarkResultsTool, s.handleListBenchmarkResults)

	// Report Management Tools
	createReportTool := mcp.NewTool("create_report",
		mcp.WithDescription("Create a new report to evaluate how well the agent team achieves its business purpose"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Name of the report")),
		mcp.WithString("description", mcp.Description("Description of the report")),
		mcp.WithString("environment_id", mcp.Required(), mcp.Description("Environment ID to evaluate")),
		mcp.WithString("team_criteria", mcp.Required(), mcp.Description("JSON defining team's business goals and success criteria. Measure against the PURPOSE of this agent team. Examples: FinOps team → cost_reduction, forecasting_accuracy; DevOps team → deployment_insights, failure_prediction; Security team → vulnerability_detection, compliance_coverage. Format: {\"goal\": \"team's purpose\", \"criteria\": {\"business_metric\": {\"weight\": 0.4, \"description\": \"what success looks like\", \"threshold\": 0.8}}}")),
		mcp.WithString("agent_criteria", mcp.Description("JSON defining how individual agents contribute to team goals. Examples: cost analyzer → savings_identified, execution_cost; PR reviewer → bugs_caught, review_speed, false_positive_rate. Measures agent's VALUE vs LABOR COST. Same format as team_criteria (optional)")),
		mcp.WithString("filter_model", mcp.Description("Filter agent runs by model name (e.g., 'openai/gpt-4o-mini', 'openai/gpt-4o'). Use list_models tool to see available models. Allows comparing performance across different models.")),
	)
	s.mcpServer.AddTool(createReportTool, s.handleCreateReport)

	generateReportTool := mcp.NewTool("generate_report",
		mcp.WithDescription("Generate a report by running benchmarks and LLM-as-judge evaluation on all agents"),
		mcp.WithString("report_id", mcp.Required(), mcp.Description("ID of the report to generate")),
	)
	s.mcpServer.AddTool(generateReportTool, s.handleGenerateReport)

	listReportsTool := mcp.NewTool("list_reports",
		mcp.WithDescription("List all reports"),
		mcp.WithString("environment_id", mcp.Description("Filter by environment ID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of reports to return (default: 50)")),
		mcp.WithNumber("offset", mcp.Description("Number of reports to skip for pagination (default: 0)")),
	)
	s.mcpServer.AddTool(listReportsTool, s.handleListReports)

	getReportTool := mcp.NewTool("get_report",
		mcp.WithDescription("Get detailed information about a specific report"),
		mcp.WithString("report_id", mcp.Required(), mcp.Description("ID of the report")),
	)
	s.mcpServer.AddTool(getReportTool, s.handleGetReport)

	// Model Filtering Tools
	listRunsByModelTool := mcp.NewTool("list_runs_by_model",
		mcp.WithDescription("List agent runs filtered by AI model name with pagination"),
		mcp.WithString("model_name", mcp.Required(), mcp.Description("Model name to filter by (e.g., 'openai/gpt-4o-mini')")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of runs to return (default: 50)")),
		mcp.WithNumber("offset", mcp.Description("Number of runs to skip for pagination (default: 0)")),
	)
	s.mcpServer.AddTool(listRunsByModelTool, s.handleListRunsByModel)

	// Schedule Management Tools
	setScheduleTool := mcp.NewTool("set_schedule",
		mcp.WithDescription("Configure an agent to run on a schedule with specified input variables"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to schedule")),
		mcp.WithString("cron_schedule", mcp.Required(), mcp.Description("Cron expression (e.g., '0 0 * * *' for daily at midnight)")),
		mcp.WithString("schedule_variables", mcp.Description("JSON object with input variables for scheduled runs")),
		mcp.WithBoolean("enabled", mcp.Description("Enable/disable the schedule (default: true)")),
	)
	s.mcpServer.AddTool(setScheduleTool, s.handleSetSchedule)

	removeScheduleTool := mcp.NewTool("remove_schedule",
		mcp.WithDescription("Remove/disable an agent's schedule configuration"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
	)
	s.mcpServer.AddTool(removeScheduleTool, s.handleRemoveSchedule)

	getScheduleTool := mcp.NewTool("get_schedule",
		mcp.WithDescription("Get an agent's current schedule configuration"),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent")),
	)
	s.mcpServer.AddTool(getScheduleTool, s.handleGetSchedule)

	// Batch Execution and Testing Tools
	batchExecuteAgentsTool := mcp.NewTool("batch_execute_agents",
		mcp.WithDescription("Execute multiple agents concurrently for testing and evaluation. Creates run results and traces for analysis."),
		mcp.WithString("tasks", mcp.Required(), mcp.Description("JSON array of execution tasks. Format: [{\"agent_id\": 1, \"task\": \"analyze logs\", \"variables\": {...}}]")),
		mcp.WithNumber("max_concurrent", mcp.Description("Maximum concurrent executions (default: 5, max: 20)")),
		mcp.WithNumber("iterations", mcp.Description("Number of times to execute each task (default: 1, max: 100)")),
		mcp.WithBoolean("store_runs", mcp.Description("Store execution results in database (default: true)")),
	)
	s.mcpServer.AddTool(batchExecuteAgentsTool, s.handleBatchExecuteAgents)

	// Dataset Export Tool
	exportDatasetTool := mcp.NewTool("export_dataset",
		mcp.WithDescription("Export agent runs and execution traces to Genkit-compatible JSON format for external evaluation and analysis."),
		mcp.WithString("filter_model", mcp.Description("Filter runs by AI model name (e.g., 'openai/gpt-4o-mini')")),
		mcp.WithString("filter_agent_id", mcp.Description("Filter runs by specific agent ID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of runs to export (default: 100)")),
		mcp.WithNumber("offset", mcp.Description("Number of runs to skip (default: 0)")),
		mcp.WithString("output_dir", mcp.Description("Output directory for dataset file (default: ./evals/)")),
	)
	s.mcpServer.AddTool(exportDatasetTool, s.handleExportDataset)

	// Complete Async Testing Pipeline
	generateAndTestAgentTool := mcp.NewTool("generate_and_test_agent",
		mcp.WithDescription("Generate test scenarios and execute comprehensive agent testing pipeline with full trace capture. Runs asynchronously and returns task ID for progress monitoring. Results saved to timestamped datasets/ directory in agent's environment workspace."),
		mcp.WithString("agent_id", mcp.Required(), mcp.Description("ID of the agent to test")),
		mcp.WithNumber("scenario_count", mcp.Description("Number of test scenarios to generate (default: 100)")),
		mcp.WithNumber("max_concurrent", mcp.Description("Maximum concurrent executions (default: 10)")),
		mcp.WithString("variation_strategy", mcp.Description("Scenario variation strategy: 'comprehensive', 'edge_cases', 'common' (default: 'comprehensive')")),
		mcp.WithString("jaeger_url", mcp.Description("Jaeger URL for trace collection (default: http://localhost:16686)")),
	)
	s.mcpServer.AddTool(generateAndTestAgentTool, s.handleGenerateAndTestAgent)

	log.Printf("MCP tools setup complete - %d tools registered", 38)
}
