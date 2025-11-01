# Station Architecture - Comprehensive ASCII Diagrams

This document provides detailed ASCII diagrams of the Station system architecture, showing how all services, APIs, CLI, MCP, and database layers interact.

---

## 1. High-Level System Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          STATION PLATFORM                               │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐  │
│  │   CLI Layer      │    │   API Server     │    │  MCP Server      │  │
│  │  (cmd/main)      │    │   (:8585)        │    │  (stdio)         │  │
│  │                  │    │                  │    │                  │  │
│  │ • agent          │    │ • /api/v1/agents │    │ • Tool handlers  │  │
│  │ • bundle         │    │ • /api/v1/envs   │    │ • Agent handlers │  │
│  │ • env            │    │ • /api/v1/runs   │    │ • Resources      │  │
│  │ • sync           │    │ • /api/v1/mcp    │    │ • Tool Discovery │  │
│  │ • runs           │    │ • /api/v1/tools  │    │ • Prompts        │  │
│  └────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘  │
│           │                       │                        │             │
│           └───────────────────────┼────────────────────────┘             │
│                                   │                                      │
│                        ┌──────────▼──────────┐                          │
│                        │   Service Layer     │                          │
│                        │  (internal/services)│                          │
│                        └──────────┬──────────┘                          │
│                                   │                                      │
│                    ┌──────────────┼──────────────┬─────────────┐        │
│                    │              │              │             │        │
│           ┌────────▼────────┐    │    ┌─────────▼──────┐    │        │
│           │Agent Execution  │    │    │MCP Connection  │    │        │
│           │Engine           │    │    │Manager         │    │        │
│           └────────┬────────┘    │    └─────────┬──────┘    │        │
│                    │             │              │            │        │
│           ┌────────▼────────┐    │    ┌─────────▼──────┐    │        │
│           │GenKit Provider  │    │    │Tool Discovery  │    │        │
│           │(OpenAI/Gemini)  │    │    │Service         │    │        │
│           └─────────────────┘    │    └─────────┬──────┘    │        │
│                                  │              │            │        │
│          ┌────────────────────────┼──────────────┼────────────▼────┐  │
│          │         Environment Management Service               │  │
│          │ (File-based config system + Variable Resolution)     │  │
│          └────────────────────────┬──────────────┬────────────┬───┘  │
│                                   │              │            │       │
│     ┌─────────────────────────────┼──────────────┼────────────┘       │
│     │ ┌──────────────────────────▼──────────────▼─────────┐          │
│     │ │        Database Layer (SQLite)                     │          │
│     │ │  /internal/db & /internal/db/repositories/        │          │
│     │ └──────────────────────────┬──────────────────────┬──┘          │
│     │                            │                      │             │
│     │              ┌─────────────┼──────────┬───────────┘             │
│     │              │             │          │                         │
│     └──────────────┼─────────────┼──────────┼────────────────────┐    │
│                    │             │          │                    │    │
│            ┌───────▼──────┐ ┌───▼───┐ ┌──▼────────┐ ┌──────────▼──┐ │
│            │Environment   │ │Agent  │ │AgentRun   │ │MCP Servers/ │ │
│            │Config Tables │ │Tables │ │Tables     │ │Tools Tables │ │
│            └───────────────┘ └───────┘ └──────────┘ └─────────────┘ │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │         File-Based Configuration Layer                           │  │
│  │  ~/.config/station/environments/<env>/                          │  │
│  │  ├── template.json       (MCP Server definitions)               │  │
│  │  ├── variables.yml       (Template variables)                   │  │
│  │  └── agents/             (Agent .prompt files)                  │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                          │
│  ┌─────────────────────────────────────────────────────────────────┐  │
│  │         External Integration Layer                               │  │
│  │  • GenKit AI Models (OpenAI, Gemini)                            │  │
│  │  • MCP Server Processes (Filesystem, Ship tools, Custom)        │  │
│  │  • Lighthouse (CloudShip data ingestion & telemetry)            │  │
│  │  • OpenTelemetry (Tracing & metrics)                            │  │
│  └─────────────────────────────────────────────────────────────────┘  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Agent Execution Flow - Complete Journey

```
┌──────────────────────────────────────────────────────────────────────┐
│                     AGENT EXECUTION PATHS                             │
└──────────────────────────────────────────────────────────────────────┘

CLI PATH: stn agent run <name> <task>
═══════════════════════════════════════════════════════════════════════

  cmd/main/handlers/agent/execution.go
  └─ RunAgentLocal()
     │
     ├─ Load config
     ├─ Open database connection
     └─ Call: agentExecutionEngine.ExecuteWithOptions()
        │
        └─ AgentExecutionEngine.ExecuteWithOptions()
           │
           ├─ Validate agent exists (nil check)
           │
           ├─ Get environment config
           │
           ├─ Initialize GenKit with AI provider
           │
           ├─ Create MCP Connection Manager
           │  ├─ Load MCP servers from environment
           │  ├─ Initialize each MCP server connection
           │  └─ Discover tools per server (cache enabled)
           │
           ├─ Render dotprompt template (if applicable)
           │
           ├─ Execute agent via GenKit
           │  ├─ Build tool list from available tools
           │  ├─ Invoke AI model with tools
           │  └─ Process tool calls iteratively
           │
           ├─ Capture execution metadata
           │  ├─ Tool calls made
           │  ├─ Execution steps
           │  ├─ Token usage
           │  └─ Duration & completion status
           │
           ├─ Send to Lighthouse (if configured)
           │  └─ CloudShip data ingestion
           │
           ├─ Cleanup MCP connections
           │
           └─ Return AgentExecutionResult


API PATH: POST /api/v1/agents/:id/execute
═══════════════════════════════════════════════════════════════════════

  internal/api/v1/agents.go
  └─ executeAgent() handler
     │
     ├─ Validate authentication
     │
     ├─ Parse task from request body
     │
     ├─ Create AgentService from repositories
     │
     └─ Call: agentService.ExecuteAgent()
        │
        └─ AgentService.ExecuteAgent()
           │
           ├─ Call: executionEngine.ExecuteWithOptions()
           │  (Same as CLI path from here forward)
           │
           └─ Stream results via HTTP response


MCP SERVER PATH: mcp_station_agent_run tool
═══════════════════════════════════════════════════════════════════════

  internal/mcp/execution_handlers.go
  └─ handleMCPAgentRun()
     │
     ├─ Parse MCP tool call arguments
     │
     ├─ Create AgentRun database entry
     │
     ├─ Call: agentService.ExecuteAgentWithRunID()
     │  (Passes proper runID for logging)
     │
     └─ Update agent run completion
        ├─ Final response
        ├─ Tool calls metadata
        └─ Status


SCHEDULER PATH: Cron-triggered execution
═══════════════════════════════════════════════════════════════════════

  internal/services/scheduler.go
  └─ SchedulerService.Start()
     │
     ├─ Load scheduled agents from database
     │
     └─ For each scheduled agent:
        └─ At cron trigger time:
           └─ Call: agentService.ExecuteAgent()
              (Same as AgentService path)


┌─ AgentExecutionEngine Key Responsibilities ─────────────────────────┐
│                                                                      │
│ File: /internal/services/agent_execution_engine.go                 │
│ Lines: 743 lines of core execution logic                           │
│                                                                      │
│ 1. Agent Validation & Loading                                      │
│    • Fetch agent from database                                     │
│    • Validate agent exists and is configured                       │
│    • Get environment and tool assignments                          │
│                                                                      │
│ 2. Environment Setup                                               │
│    • Load environment configuration                                │
│    • Load variables.yml for template processing                    │
│    • Validate environment exists                                   │
│                                                                      │
│ 3. AI Model Configuration                                          │
│    • Initialize GenKit with configured provider                    │
│    • Set up model-specific parameters                              │
│    • Handle model fallbacks if needed                              │
│                                                                      │
│ 4. MCP Server Connection                                           │
│    • Create MCPConnectionManager instance                          │
│    • Connect to each MCP server in environment                     │
│    • Discover available tools per server                           │
│    • Cache tools for performance                                   │
│                                                                      │
│ 5. Tool Preparation                                                │
│    • Build complete tool list from all servers                     │
│    • Apply agent-specific tool assignments                         │
│    • Filter tools based on permissions                             │
│                                                                      │
│ 6. Execution                                                       │
│    • Render dotprompt template with variables                      │
│    • Pass task + tools to GenKit                                   │
│    • Process iterative tool calls                                  │
│    • Enforce max steps limit                                       │
│                                                                      │
│ 7. Metadata Capture                                                │
│    • Record all tool calls with parameters                         │
│    • Capture execution steps                                       │
│    • Track token usage (input/output)                              │
│    • Measure execution duration                                    │
│                                                                      │
│ 8. Data Persistence                                                │
│    • Create AgentRun record                                        │
│    • Save execution steps and metadata                             │
│    • Update completion status                                      │
│    • Store final response                                          │
│                                                                      │
│ 9. Lighthouse Integration (Optional)                               │
│    • Send run data to Lighthouse                                   │
│    • Include app/app_type classification                           │
│    • Enable CloudShip data ingestion                               │
│                                                                      │
│ 10. Cleanup                                                        │
│    • Close all MCP connections                                     │
│    • Release resources                                             │
│    • Stop any running processes                                    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 3. Service Layer Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      CORE SERVICES LAYER                             │
│          /internal/services/ - 43 focused modules                    │
└─────────────────────────────────────────────────────────────────────┘

AGENT EXECUTION SERVICES
═════════════════════════════════════════════════════════════════════

  AgentService (agent_service_impl.go - 586 lines)
  ├─ Creates/manages AgentExecutionEngine
  ├─ Wraps execution with telemetry
  ├─ Interface: AgentServiceInterface
  └─ Methods:
     ├─ ExecuteAgent(agentID, task, variables) → Message
     ├─ ExecuteAgentWithRunID(agentID, task, runID, variables)
     ├─ CreateAgent(config) → Agent
     ├─ UpdateAgent(agentID, config) → Agent
     ├─ DeleteAgent(agentID)
     └─ GetAgent(agentID) → Agent

  AgentExecutionEngine (agent_execution_engine.go - 743 lines)
  ├─ Main execution orchestrator
  ├─ Manages GenKit + MCP lifecycle
  ├─ Coordinates tool discovery
  └─ Methods:
     ├─ Execute(agent, task, runID, userVars)
     ├─ ExecuteWithOptions(agent, task, runID, userVars, skipLighthouse)
     └─ Internal execution flow coordination

  GenKitProvider (genkit_provider.go - 10KB)
  ├─ AI model provider initialization
  ├─ Supports: OpenAI, Gemini, Ollama
  └─ Methods:
     ├─ InitializeGenKit() → *genkit.Genkit
     └─ Provider-specific configuration


MCP MANAGEMENT SERVICES
═════════════════════════════════════════════════════════════════════

  MCPConnectionManager (mcp_connection_manager.go - 20KB)
  ├─ Lifecycle management for MCP connections
  ├─ Tool discovery and caching
  ├─ Connection pooling support
  └─ Key Methods:
     ├─ DiscoverTools(envID, genkitApp)
     ├─ GetCachedTools(envID)
     ├─ CleanupConnections(clients)
     └─ EnableConnectionPooling()

  MCPServerManagementService (mcp_server_management_service.go)
  ├─ MCP server CRUD operations
  ├─ Server registration and lifecycle
  └─ Methods:
     ├─ RegisterServer(envID, config)
     ├─ GetServersByEnvironment(envID)
     └─ DeleteServer(serverID)

  MCPToolDiscovery (mcp_tool_discovery.go - 655 lines)
  ├─ Discovers tools from MCP servers
  ├─ Maps tools to servers
  ├─ Handles tool registration
  └─ Methods:
     ├─ DiscoverToolsPerServer(fileConfig)
     ├─ SaveToolsForServer(envID, serverName, tools)
     └─ GetServerTools(serverID)

  ToolDiscoveryService (internal/services/tool_discovery_service.go)
  ├─ High-level tool discovery orchestration
  ├─ Caching layer
  └─ Methods:
     ├─ DiscoverTools(envID) → []Tool
     ├─ RefreshTools(envID)
     └─ GetToolsByEnvironment(envID)


ENVIRONMENT & CONFIGURATION SERVICES
═════════════════════════════════════════════════════════════════════

  EnvironmentManagementService (environment_management_service.go)
  ├─ Environment CRUD operations
  ├─ File-based configuration management
  ├─ Variables resolution
  └─ Methods:
     ├─ CreateEnvironment(name, description, userID)
     ├─ DeleteEnvironment(name)
     ├─ GetEnvironmentFileConfig(name)
     └─ UpdateEnvironmentFileConfig(name, filename, content)

  DeclarativeSync (declarative_sync.go - 868 lines)
  ├─ Synchronizes file-based configs to database
  ├─ Bidirectional sync (files ↔ database)
  ├─ Template variable processing (Go templates)
  ├─ MCP server registration
  ├─ Agent creation from .prompt files
  └─ Key Methods:
     ├─ SyncEnvironment(ctx, envName, options)
     ├─ SyncAgents(ctx, envID)
     ├─ SyncMCPServers(ctx, envID)
     ├─ performToolDiscovery(ctx, envID, configName)
     └─ discoverToolsPerServer(ctx, mcpConnManager, fileConfig)

  EnvironmentCopyService (environment_copy_service.go)
  ├─ Duplicates environments
  ├─ Clones agents and MCP configs
  └─ Methods:
     ├─ CopyEnvironment(sourceID, targetName)
     └─ CloneAgentsToEnvironment(sourceID, targetEnvID)

  BundleService (bundle_service.go - 881 lines)
  ├─ Environment bundling
  ├─ Creates tar.gz packages
  ├─ Manifest generation
  └─ Methods:
     ├─ CreateBundle(envPath) → []byte
     ├─ generateManifest(sourceDir)
     └─ createTarGz(sourceDir) → []byte


SCHEDULING & AUTOMATION
═════════════════════════════════════════════════════════════════════

  SchedulerService (scheduler.go - 590 lines)
  ├─ Cron-based agent scheduling
  ├─ Persistent schedule storage
  ├─ Schedule management lifecycle
  └─ Key Methods:
     ├─ Start() - loads and activates schedules
     ├─ Stop() - graceful shutdown
     ├─ ScheduleAgent(agentID, cronExpression)
     ├─ UnscheduleAgent(agentID)
     └─ loadScheduledAgents()


TELEMETRY & MONITORING
═════════════════════════════════════════════════════════════════════

  TelemetryService (telemetry_service.go)
  ├─ OpenTelemetry integration
  ├─ Span creation and tracking
  ├─ Metric collection
  └─ Methods:
     ├─ Initialize(ctx)
     ├─ CreateSpan(ctx, name, attributes)
     └─ RecordMetric(name, value)

  DeploymentContextService (deployment_context_service.go - 727 lines)
  ├─ Gathers deployment environment info
  ├─ Kubernetes/Container detection
  ├─ Infrastructure metadata
  └─ Methods:
     ├─ GetDeploymentContext() → DeploymentContext
     └─ detectDeploymentType() → string


DATA & FILE MANAGEMENT
═════════════════════════════════════════════════════════════════════

  AgentFileSync (agent_file_sync.go - 871 lines)
  ├─ Bidirectional agent file synchronization
  ├─ .prompt file parsing
  ├─ Database ↔ Filesystem sync
  └─ Methods:
     ├─ SyncAgentsFromFiles(envID)
     ├─ ExportAgentToFile(agentID, envPath)
     └─ ParseAgentPromptFile(filePath)

  AgentExportService (agent_export_service.go - 9KB)
  ├─ Exports agents for distribution
  ├─ Creates portable agent packages
  └─ Methods:
     ├─ ExportAgent(agentID)
     ├─ ExportMultipleAgents(agentIDs)
     └─ ExportEnvironment(envID)

  SyncCleanup (sync_cleanup.go)
  ├─ Cleanup of sync-related temporary data
  └─ Methods:
     ├─ CleanupFailedSyncs()
     └─ CleanupTempFiles()


SCHEMA & VALIDATION
═════════════════════════════════════════════════════════════════════

  SchemaService (schema.go - 655 lines)
  ├─ JSON schema validation
  ├─ Input/output schema management
  ├─ Schema registry
  └─ Methods:
     ├─ ValidateInputSchema(input, schemaJSON)
     ├─ ValidateOutputSchema(output, schemaJSON)
     └─ GetSchemaRegistry() → SchemaRegistry


PARALLEL PROCESSING
═════════════════════════════════════════════════════════════════════

  MCPParallelProcessing (mcp_parallel_processing.go)
  ├─ Concurrent tool discovery
  ├─ Parallel MCP server initialization
  └─ Methods:
     ├─ DiscoverToolsParallel(servers, tools)
     └─ InitializeServersParallel(configs) → []*MCP
```

---

## 4. MCP Server Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                    MCP SERVER SUBSYSTEM                               │
│          /internal/mcp/ - Handles Model Context Protocol              │
└──────────────────────────────────────────────────────────────────────┘

CORE MCP SERVER
═══════════════════════════════════════════════════════════════════════

  MCP Server (mcp/server.go)
  └─ Main MCP server instance
     ├─ Capabilities:
     │  ├─ Tools (tool discovery & execution)
     │  ├─ Resources (read-only data access)
     │  ├─ Prompts (system prompts)
     │  └─ Tool suggestions
     │
     ├─ Core Components:
     │  ├─ mcpServer: *server.MCPServer (mark3labs/mcp-go)
     │  ├─ httpServer: *server.StreamableHTTPServer
     │  ├─ toolDiscoverySvc: *ToolDiscoveryService
     │  ├─ agentService: AgentServiceInterface
     │  ├─ authService: *auth.AuthService
     │  └─ repos: *repositories.Repositories
     │
     └─ Initialization:
        ├─ setupTools() - Register tool handlers
        ├─ setupResources() - Register resource handlers
        ├─ setupToolSuggestion() - Register prompt suggestions
        └─ NewToolsServer() - Advanced tools server


HANDLER ORGANIZATION
═══════════════════════════════════════════════════════════════════════

  Agent Handlers (agent_handlers.go)
  └─ mcp_station_agent_* tools
     ├─ run: Execute agent with task
     ├─ list: List agents in environment
     ├─ show: Get agent details
     ├─ create: Create new agent
     ├─ update: Modify agent
     └─ delete: Remove agent

  Environment Handlers (environment_handlers.go)
  └─ mcp_station_environment_* tools
     ├─ list: List environments
     ├─ show: Get environment details
     ├─ create: Create environment
     ├─ delete: Remove environment
     └─ sync: Trigger declarative sync

  Execution Handlers (execution_handlers.go)
  └─ mcp_station_execution_* tools
     ├─ queue: Queue agent for execution
     ├─ get_status: Check execution status
     └─ cancel: Stop running agent

  MCP Server Handlers (mcp_server_handlers.go)
  └─ mcp_station_mcp_* tools
     ├─ list: List MCP servers
     ├─ show: Get MCP server details
     ├─ register: Add MCP server config
     └─ tools: List available tools

  Tool Handlers (tool_handlers.go)
  └─ mcp_station_tool_* tools
     ├─ list: List available tools
     ├─ show: Get tool details
     ├─ assign: Assign tool to agent
     └─ discover: Trigger tool discovery

  Resource Handlers (resources_handlers.go)
  └─ Resource (text) providers
     ├─ agents/{id}: Agent details resource
     ├─ environments/{id}: Environment details resource
     ├─ tools/{id}: Tool details resource
     └─ mcp-servers/{id}: MCP server details resource

  Prompt Handlers (prompts_handlers.go)
  └─ Tool suggestions
     ├─ agent-management
     ├─ environment-setup
     ├─ tool-discovery
     └─ sync-workflow

  Export Handlers (export_handlers.go)
  └─ mcp_station_export_* tools
     ├─ agent: Export single agent
     ├─ agents: Export all agents
     └─ environment: Export environment bundle

  Demo Bundle Handlers (demo_bundle_handlers.go)
  └─ mcp_station_demo_* tools
     ├─ list_demos: Show available demos
     └─ load_demo: Load demo environment


TOOL DISCOVERY
═══════════════════════════════════════════════════════════════════════

  ToolDiscoveryService (mcp/tool_discovery.go)
  ├─ Discovers MCP server tools
  ├─ Maps tools to servers
  ├─ Maintains server-tool relationships
  └─ Methods:
     ├─ DiscoverTools(envID, genkitApp)
     ├─ discoverToolsPerServer(fileConfig)
     └─ saveToolsForServer(envID, serverName, tools)

  Tool Suggestion Setup (tool_suggestion_setup.go)
  └─ Configures tool suggestions for MCP
     ├─ Analyzes agent context
     ├─ Recommends relevant tools
     └─ Provides quick-start suggestions


SPECIAL TOOLS
═══════════════════════════════════════════════════════════════════════

  Unified Bundle Tools (unified_bundle_tools.go)
  └─ Tools for bundle management
     ├─ list_bundle_agents
     ├─ show_bundle_agent
     └─ execute_bundle_agent

  Demo Bundle Tools (demo_bundle_tools.go)
  └─ Demo environment capabilities
     ├─ Demo agents
     ├─ Sample workflows
     └─ Learning resources


STATUS & MONITORING
═══════════════════════════════════════════════════════════════════════

  Status Service (status_service.go)
  └─ MCP server health & metrics
     ├─ Connection status
     ├─ Tool availability
     ├─ Performance metrics
     └─ Health checks


LIGHTHOUSE INTEGRATION
═══════════════════════════════════════════════════════════════════════

  Lighthouse Integration (lighthouse_integration.go)
  └─ CloudShip data ingestion
     ├─ Run telemetry forwarding
     ├─ Data classification
     └─ Metrics reporting


RESOURCE SETUP
═══════════════════════════════════════════════════════════════════════

  Resources Setup (resources_setup.go)
  └─ Registers resource URIs
     ├─ Agent resources
     ├─ Environment resources
     ├─ Tool resources
     └─ Metadata resources
```

---

## 5. Database Layer & Repositories

```
┌──────────────────────────────────────────────────────────────────────┐
│                      DATABASE LAYER                                   │
│   /internal/db & /internal/db/repositories/ - Data Persistence       │
└──────────────────────────────────────────────────────────────────────┘

DATABASE INITIALIZATION
═══════════════════════════════════════════════════════════════════════

  DB (internal/db/db.go)
  ├─ SQLite database connection
  ├─ Connection pooling
  ├─ Migration execution
  └─ Methods:
     ├─ New(dbURL) → Database interface
     ├─ Migrate(ctx, schemaSQL)
     └─ Close()

  Queries (internal/db/queries/)
  └─ Auto-generated from sqlc
     ├─ db.go - Main queries interface
     ├─ models.go - Generated model structs
     └─ <entity>.sql.go - CRUD for each entity


REPOSITORY LAYER
═══════════════════════════════════════════════════════════════════════

  Repositories (repositories/base.go)
  └─ Central repository collection
     │
     ├─ Environments *EnvironmentRepo
     │  └─ Methods: Create, List, GetByID, GetByName, Delete
     │
     ├─ Users *UserRepo
     │  └─ Methods: Create, GetByID, GetByUsername, List
     │
     ├─ FileMCPConfigs *FileMCPConfigRepo
     │  └─ Methods: Create, GetByEnvironmentAndName, List, Delete
     │
     ├─ MCPServers *MCPServerRepo
     │  └─ Methods: Create, GetByEnvironmentID, GetByID, Delete
     │
     ├─ MCPTools *MCPToolRepo
     │  └─ Methods: Create, GetByServerID, GetByEnvironmentID, Delete
     │
     ├─ ModelProviders *ModelProviderRepository
     │  └─ Methods: Create, List, GetByID, Delete
     │
     ├─ Models *ModelRepository
     │  └─ Methods: Create, List, GetByID, GetByProvider
     │
     ├─ Agents *AgentRepo
     │  └─ Methods: Create, List, GetByID, GetByName, Update, Delete
     │
     ├─ AgentTools *AgentToolRepo
     │  └─ Methods: Create, GetByAgentID, AddToolToAgent, RemoveTool
     │
     ├─ AgentRuns *AgentRunRepo
     │  └─ Methods: Create, GetByID, List, GetByAgentID, UpdateCompletion
     │
     └─ Settings *SettingsRepo
        └─ Methods: Get, Set, Delete, List


DATA MODELS (Queries-generated)
═══════════════════════════════════════════════════════════════════════

  Agent Run: agent_runs.sql.go
  └─ CREATE run, UPDATE completion, LIST runs, GET run
     ├─ query: INSERT INTO agent_runs
     ├─ query: UPDATE agent_runs SET completed_at, status
     └─ query: SELECT * FROM agent_runs

  Agent Tools: agent_tools.sql.go
  └─ ASSIGN/UNASSIGN tools to agents
     ├─ query: INSERT INTO agent_tools
     └─ query: DELETE FROM agent_tools

  Agents: agents.sql.go
  └─ CRUD operations on agents
     ├─ query: INSERT INTO agents
     ├─ query: UPDATE agents
     ├─ query: DELETE FROM agents
     ├─ query: SELECT * FROM agents
     └─ query: SELECT * FROM agents WHERE name

  MCP Tools: mcp_tools.sql.go
  └─ Tool registry operations
     ├─ query: INSERT INTO mcp_tools
     ├─ query: SELECT * FROM mcp_tools
     └─ query: DELETE FROM mcp_tools

  MCP Servers: mcp_servers.sql.go
  └─ MCP server configuration
     ├─ query: INSERT INTO mcp_servers
     ├─ query: SELECT * FROM mcp_servers
     └─ query: DELETE FROM mcp_servers

  Environments: environments.sql.go
  └─ Environment management
     ├─ query: INSERT INTO environments
     ├─ query: SELECT * FROM environments
     └─ query: DELETE FROM environments

  File MCP Configs: file_mcp_configs.sql.go
  └─ File-based config tracking
     ├─ query: INSERT INTO file_mcp_configs
     ├─ query: SELECT * FROM file_mcp_configs
     └─ query: DELETE FROM file_mcp_configs


SCHEMA TABLES
═══════════════════════════════════════════════════════════════════════

  users
  ├─ id (PK)
  ├─ username
  ├─ email
  └─ created_at

  environments
  ├─ id (PK)
  ├─ name (UNIQUE)
  ├─ description
  ├─ created_by (FK → users)
  └─ created_at

  file_mcp_configs
  ├─ id (PK)
  ├─ environment_id (FK → environments)
  ├─ name
  ├─ file_path
  ├─ config_hash
  └─ synced_at

  mcp_servers
  ├─ id (PK)
  ├─ environment_id (FK → environments)
  ├─ file_config_id (FK → file_mcp_configs)
  ├─ name
  ├─ server_type
  ├─ command
  └─ created_at

  mcp_tools
  ├─ id (PK)
  ├─ mcp_server_id (FK → mcp_servers)
  ├─ name (UNIQUE per server)
  ├─ description
  ├─ input_schema (JSON)
  ├─ tags (JSON)
  └─ created_at

  agents
  ├─ id (PK)
  ├─ environment_id (FK → environments)
  ├─ name
  ├─ description
  ├─ prompt
  ├─ max_steps
  ├─ created_by (FK → users)
  ├─ input_schema (JSON)
  ├─ output_schema (JSON)
  ├─ output_schema_preset
  ├─ model_provider
  ├─ model_id
  ├─ app (CloudShip classification)
  ├─ app_type (CloudShip classification)
  ├─ cron_schedule
  ├─ schedule_enabled
  └─ created_at

  agent_tools
  ├─ agent_id (FK → agents)
  ├─ tool_id (FK → mcp_tools)
  └─ assigned_at

  agent_runs
  ├─ id (PK)
  ├─ agent_id (FK → agents)
  ├─ task
  ├─ status
  ├─ started_at
  ├─ completed_at
  ├─ final_response
  ├─ tool_calls (JSON)
  ├─ execution_steps (JSON)
  ├─ input_tokens
  ├─ output_tokens
  ├─ steps_taken
  └─ metadata (JSON)

  model_providers
  ├─ id (PK)
  ├─ name
  ├─ base_url (optional)
  └─ created_at

  models
  ├─ id (PK)
  ├─ provider_id (FK → model_providers)
  ├─ model_id
  ├─ display_name
  └─ created_at

  settings
  ├─ key (PK)
  ├─ value
  └─ updated_at
```

---

## 6. API Handler Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                      API HANDLERS LAYER                               │
│        /internal/api/v1/ - RESTful API Endpoints (:8585)              │
└──────────────────────────────────────────────────────────────────────┘

API STRUCTURE
═══════════════════════════════════════════════════════════════════════

  APIHandlers (handlers.go - Entry point)
  └─ Central handler for all API operations
     │
     ├─ repos *repositories.Repositories
     ├─ agentService AgentServiceInterface
     ├─ toolDiscoveryService *ToolDiscoveryService
     ├─ telemetryService *TelemetryService
     └─ localMode bool


ROUTE REGISTRATION
═══════════════════════════════════════════════════════════════════════

  RegisterRoutes(gin.RouterGroup)
  └─ Registers all v1 API routes
     │
     ├─ /agents (v1/agents.go)
     │  ├─ GET / - List agents
     │  ├─ POST / - Create agent
     │  ├─ GET /:id - Get agent details
     │  ├─ PUT /:id - Update agent
     │  ├─ DELETE /:id - Delete agent
     │  ├─ GET /:id/details - Get agent with tools
     │  ├─ POST /:id/execute - Execute agent
     │  ├─ GET /:id/prompt - Get agent prompt
     │  └─ PUT /:id/prompt - Update agent prompt
     │
     ├─ /agent-runs (v1/agent_runs.go)
     │  ├─ GET / - List agent runs
     │  └─ GET /:id - Get run details
     │
     ├─ /environments (v1/environments.go)
     │  ├─ GET / - List environments
     │  ├─ POST / - Create environment
     │  ├─ GET /:id - Get environment
     │  ├─ PUT /:id - Update environment
     │  ├─ DELETE /:id - Delete environment
     │  └─ GET /:id/config - Get environment file config
     │
     ├─ /mcp-servers (v1/mcp_servers.go)
     │  ├─ GET / - List MCP servers
     │  ├─ GET /:id - Get MCP server details
     │  └─ GET /:id/tools - Get server's tools
     │
     ├─ /tools (v1/tools.go)
     │  ├─ GET / - List tools (optionally filtered)
     │  └─ GET /:id - Get tool details
     │
     ├─ /mcp-management (v1/mcp_management.go)
     │  ├─ POST /register-server - Register MCP server
     │  ├─ POST /discover-tools - Trigger tool discovery
     │  └─ POST /sync - Trigger environment sync
     │
     ├─ /bundles (v1/bundles.go)
     │  ├─ POST / - Create bundle
     │  ├─ GET / - List bundles
     │  ├─ GET /:name - Get bundle details
     │  └─ POST /install - Install bundle
     │
     ├─ /sync (v1/sync_interactive.go)
     │  ├─ POST / - Trigger sync
     │  ├─ POST /interactive - Interactive sync with variable prompting
     │  └─ POST /resolve-variables - Resolve missing variables
     │
     ├─ /settings (v1/settings.go)
     │  ├─ GET / - List settings
     │  ├─ GET /:key - Get setting value
     │  ├─ POST / - Create/update setting
     │  └─ DELETE /:key - Delete setting
     │
     ├─ /demo-bundles (v1/demo_bundles.go)
     │  ├─ GET / - List available demos
     │  └─ POST /:name/load - Load demo environment
     │
     ├─ /openapi (v1/openapi.go)
     │  └─ POST /convert - Convert OpenAPI spec to MCP
     │
     ├─ /lighthouse (v1/lighthouse.go)
     │  ├─ POST /register - Register with Lighthouse
     │  └─ GET /status - Check Lighthouse status
     │
     └─ /ship (v1/ship.go)
        └─ Ship security tools integration endpoints


AGENT HANDLERS (agents.go)
═══════════════════════════════════════════════════════════════════════

  listAgents(c *gin.Context)
  ├─ Query: environment_id (optional filter)
  ├─ Returns: []Agent with metadata

  createAgent(c *gin.Context)
  ├─ Body: AgentConfig
  ├─ Creates agent in database
  ├─ Returns: Created Agent

  getAgent(c *gin.Context)
  ├─ Path: agent ID
  ├─ Returns: Agent details

  getAgentWithTools(c *gin.Context)
  ├─ Path: agent ID
  ├─ Returns: Agent with assigned tools

  updateAgent(c *gin.Context)
  ├─ Path: agent ID
  ├─ Body: AgentConfig updates
  ├─ Returns: Updated Agent

  deleteAgent(c *gin.Context)
  ├─ Path: agent ID
  ├─ Deletes agent and associations

  executeAgent(c *gin.Context)
  ├─ Path: agent ID
  ├─ Body: { task: "...", variables: {...} }
  ├─ Calls: AgentService.ExecuteAgent()
  ├─ Returns: AgentExecutionResult

  getAgentPrompt(c *gin.Context)
  ├─ Path: agent ID
  ├─ Returns: Agent's prompt text

  updateAgentPrompt(c *gin.Context)
  ├─ Path: agent ID
  ├─ Body: { prompt: "..." }
  └─ Updates agent prompt


AGENT RUN HANDLERS (agent_runs.go)
═══════════════════════════════════════════════════════════════════════

  listAgentRuns(c *gin.Context)
  ├─ Query: agent_id, status, limit, offset
  ├─ Returns: []AgentRun paginated

  getAgentRun(c *gin.Context)
  ├─ Path: run ID
  ├─ Returns: Complete run details with metadata


ENVIRONMENT HANDLERS (environments.go)
═══════════════════════════════════════════════════════════════════════

  listEnvironments(c *gin.Context)
  ├─ Returns: []Environment

  createEnvironment(c *gin.Context)
  ├─ Body: { name, description }
  ├─ Creates: Database entry + file structure
  ├─ Returns: Created Environment

  getEnvironment(c *gin.Context)
  ├─ Path: environment ID
  ├─ Returns: Environment with metadata

  deleteEnvironment(c *gin.Context)
  ├─ Path: environment ID
  ├─ Cleans: Database + file structure

  getEnvironmentConfig(c *gin.Context)
  ├─ Path: environment ID
  ├─ Returns: File-based config (template.json, variables.yml)


TOOL HANDLERS (tools.go)
═══════════════════════════════════════════════════════════════════════

  listTools(c *gin.Context)
  ├─ Query: environment_id, server_id, tags
  ├─ Returns: []Tool filtered

  getTool(c *gin.Context)
  ├─ Path: tool ID
  ├─ Returns: Tool with schema


MCP MANAGEMENT (mcp_management.go)
═══════════════════════════════════════════════════════════════════════

  registerServer(c *gin.Context)
  ├─ Body: MCP server config
  ├─ Creates: File-based config
  ├─ Syncs: To database
  └─ Returns: Registered server

  discoverTools(c *gin.Context)
  ├─ Query: environment_id
  ├─ Triggers: Tool discovery process
  ├─ Returns: Discovery results

  syncEnvironment(c *gin.Context)
  ├─ Query: environment_id, dry_run
  ├─ Triggers: DeclarativeSync.SyncEnvironment()
  └─ Returns: Sync results


SYNC HANDLERS (sync_interactive.go)
═══════════════════════════════════════════════════════════════════════

  syncEnvironment(c *gin.Context)
  ├─ Standard sync operation
  └─ Returns: SyncResult

  interactiveSync(c *gin.Context)
  ├─ UI-integrated variable resolution
  ├─ Form submission handling
  └─ Real-time progress updates

  resolveVariables(c *gin.Context)
  ├─ Variables form submission
  ├─ Updates: variables.yml
  └─ Triggers: Retry sync


BUNDLE HANDLERS (bundles.go)
═══════════════════════════════════════════════════════════════════════

  createBundle(c *gin.Context)
  ├─ Query: environment_id
  ├─ Returns: tar.gz data

  listBundles(c *gin.Context)
  ├─ Returns: Available bundles

  installBundle(c *gin.Context)
  ├─ Body: { bundle_url, name }
  ├─ Downloads: Bundle
  ├─ Extracts: Environment
  └─ Syncs: Configuration


DEMO BUNDLE HANDLERS (demo_bundles.go)
═══════════════════════════════════════════════════════════════════════

  listDemoBundles(c *gin.Context)
  ├─ Returns: Available demo environments

  loadDemoBundle(c *gin.Context)
  ├─ Path: bundle name
  ├─ Creates: Demo environment
  └─ Returns: Environment details


SETTINGS HANDLERS (settings.go)
═══════════════════════════════════════════════════════════════════════

  getSettings(c *gin.Context)
  ├─ Returns: All settings key-value pairs

  getSetting(c *gin.Context)
  ├─ Path: setting key
  ├─ Returns: Setting value

  setSetting(c *gin.Context)
  ├─ Path: setting key
  ├─ Body: { value: "..." }
  └─ Updates: Setting

  deleteSetting(c *gin.Context)
  ├─ Path: setting key
  └─ Deletes: Setting


AUTHENTICATION
═══════════════════════════════════════════════════════════════════════

  All handlers validate:
  ├─ Authorization header (Bearer token)
  ├─ User permissions
  └─ Environment access
```

---

## 7. CLI Command Structure

```
┌──────────────────────────────────────────────────────────────────────┐
│                        CLI LAYER                                      │
│         /cmd/main/ - Command-line Interface (stn)                     │
└──────────────────────────────────────────────────────────────────────┘

MAIN COMMAND TREE
═══════════════════════════════════════════════════════════════════════

  $ stn [command] [subcommand] [args]

  Core Commands:
  ├─ stn agent [subcommand]
  │  ├─ list [--env name]
  │  │  └─ Lists all agents (optionally filtered by environment)
  │  │
  │  ├─ show <name>
  │  │  └─ Shows agent details
  │  │
  │  ├─ run <name> <task> [--tail]
  │  │  ├─ Executes agent locally or via server
  │  │  └─ --tail: Stream live execution logs
  │  │
  │  ├─ create <name> [--env name]
  │  │  └─ Creates new agent interactively
  │  │
  │  ├─ delete <name>
  │  │  └─ Deletes agent
  │  │
  │  └─ export-agents [--env name] [--output-directory path]
  │     └─ Exports agents from environment
  │
  ├─ env [subcommand]
  │  ├─ list
  │  │  └─ Lists all environments
  │  │
  │  ├─ create <name> [--description]
  │  │  └─ Creates new environment
  │  │
  │  ├─ delete <name>
  │  │  └─ Deletes environment
  │  │
  │  ├─ copy <source> <destination>
  │  │  └─ Clones environment
  │  │
  │  └─ show <name>
  │     └─ Shows environment details
  │
  ├─ sync [environment] [--dry-run] [--force] [--verbose]
  │  ├─ Syncs environment from file-based config
  │  ├─ --dry-run: Preview changes without applying
  │  ├─ --force: Skip confirmations
  │  └─ --verbose: Detailed output
  │
  ├─ bundle [subcommand]
  │  ├─ create <environment>
  │  │  └─ Creates tar.gz bundle
  │  │
  │  ├─ install <url>
  │  │  └─ Installs bundle from URL
  │  │
  │  └─ list
  │     └─ Lists available bundles
  │
  ├─ runs [subcommand]
  │  ├─ list [--agent name] [--status] [--limit N]
  │  │  └─ Lists agent execution runs
  │  │
  │  └─ inspect <run-id> [--verbose]
  │     └─ Shows detailed run results
  │
  ├─ server
  │  └─ Starts HTTP API server on port 8585
  │
  ├─ up / down
  │  ├─ up: Starts full Station system
  │  └─ down: Stops Station system
  │
  ├─ init
  │  └─ Initializes Station in project directory
  │
  ├─ status
  │  └─ Shows system status
  │
  └─ config
     ├─ show
     ├─ set <key> <value>
     └─ get <key>


AGENT COMMAND IMPLEMENTATION (agent.go)
═══════════════════════════════════════════════════════════════════════

  Command Definitions:
  ├─ agentCmd - Root agent command
  ├─ agentListCmd - List agents
  ├─ agentShowCmd - Show agent details
  ├─ agentRunCmd - Execute agent
  └─ agentDeleteCmd - Delete agent

  Handlers (handlers/agent/):
  ├─ handlers.go - Main handler structure
  │  └─ AgentHandler{
  │     ├─ db: Database
  │     ├─ telemetryService: TelemetryService
  │     └─ themeManager: ThemeManager
  │     }
  │
  ├─ execution.go - Agent execution
  │  └─ RunAgentRun(agentID, task, tail)
  │     ├─ Finds agent by name or ID
  │     ├─ Calls: agentExecutionEngine.Execute()
  │     ├─ Optionally tails logs
  │     └─ Displays results
  │
  ├─ local.go - Local execution (CLI)
  │  └─ runAgentLocal()
  │     ├─ Connects to database
  │     ├─ Loads agent configuration
  │     ├─ Initializes execution engine
  │     ├─ Executes agent
  │     ├─ Streams output in real-time
  │     └─ Displays formatted results
  │
  └─ utils.go - Helper functions


ENVIRONMENT COMMAND IMPLEMENTATION (env.go not shown, but follows pattern)
═══════════════════════════════════════════════════════════════════════

  Environment operations:
  ├─ Create environment directory structure
  ├─ Initialize database entry
  ├─ Create agents/ subdirectory
  ├─ Write default variables.yml
  ├─ List environments with metadata
  └─ Delete environment (both DB + files)


SYNC COMMAND IMPLEMENTATION (up.go, server.go)
═══════════════════════════════════════════════════════════════════════

  $ stn sync [environment] [--dry-run] [--force]
  │
  └─ Triggers: DeclarativeSync.SyncEnvironment()
     ├─ Load environment from file system
     ├─ Parse agents from .prompt files
     ├─ Parse MCP servers from template.json
     ├─ Resolve variables from variables.yml (Go templates)
     ├─ Validate configurations
     ├─ Create/update database entries
     ├─ Discover tools from MCP servers
     ├─ Create agents
     └─ Display results

  $ stn up [--server] [--stdio]
  │
  ├─ Server mode:
  │  ├─ Start HTTP API server (:8585)
  │  ├─ Start MCP server (stdio)
  │  ├─ Initialize Genkit with AI provider
  │  └─ Load and start schedulers
  │
  └─ Stdio mode (default):
     ├─ Start MCP server on stdio
     └─ All communication via stdin/stdout


RUNS COMMAND IMPLEMENTATION (runs.go)
═══════════════════════════════════════════════════════════════════════

  $ stn runs list [--agent name] [--status status] [--limit N]
  │
  └─ Queries: AgentRunRepo.List()
     └─ Displays: Paginated run list

  $ stn runs inspect <run-id> [-v|--verbose]
  │
  └─ Queries: AgentRunRepo.GetByID()
     ├─ Display: Final response
     ├─ Display: Tool calls (if verbose)
     ├─ Display: Execution steps (if verbose)
     ├─ Display: Token usage (if verbose)
     └─ Display: Metadata (if verbose)


SERVER COMMAND (server.go)
═══════════════════════════════════════════════════════════════════════

  $ stn server [--port 8585] [--debug]
  │
  ├─ Initialize database
  ├─ Load configuration
  ├─ Initialize GenKit with AI provider
  ├─ Create repositories
  ├─ Create API server
  ├─ Register all v1 routes
  ├─ Start HTTP listener
  ├─ Initialize MCP server (stdio)
  ├─ Setup signal handlers
  └─ Graceful shutdown on SIGTERM/SIGINT


UP COMMAND (up.go, up_unix.go, up_windows.go)
═══════════════════════════════════════════════════════════════════════

  $ stn up [--server] [--stdio]
  │
  ├─ Load configuration
  ├─ Initialize database
  ├─ Setup Genkit with AI provider
  ├─ Start scheduler service
  ├─ If --server:
  │  └─ Start HTTP API server
  ├─ If --stdio (default):
  │  └─ Start MCP server on stdio
  └─ Setup graceful shutdown


CLI UTILITIES
═══════════════════════════════════════════════════════════════════════

  handlers/common.go
  └─ Common utilities
     ├─ getThemeManager() → ThemeManager
     ├─ getCLIStyles(themeManager) → CLIStyles
     └─ Output formatting functions

  handlers/common/utils.go
  └─ Utility functions
     ├─ Table formatting
     ├─ Error display
     └─ Progress indicators

  Animation & UI:
  ├─ animation.go - Spinner and progress animations
  ├─ banner.go - ASCII art banner
  ├─ styles.go - CLI color/styling
  └─ provider_setup.go - Provider selection UI


CONFIGURATION (config.go)
═══════════════════════════════════════════════════════════════════════

  $ stn config get <key>
  $ stn config set <key> <value>

  Loaded from: ~/.station/config.yaml or env vars
  Provides: Global Station configuration


MAIN ENTRY POINT (main.go)
═══════════════════════════════════════════════════════════════════════

  main()
  ├─ Initialize root command
  ├─ Parse CLI flags
  ├─ Load telemetry service
  ├─ Execute requested command
  └─ Handle errors
```

---

## 8. Data Flow Patterns

```
┌──────────────────────────────────────────────────────────────────────┐
│                       DATA FLOW PATTERNS                              │
│            How information flows through the system                   │
└──────────────────────────────────────────────────────────────────────┘

ENVIRONMENT SYNCHRONIZATION FLOW
═══════════════════════════════════════════════════════════════════════

File System                 DeclarativeSync               Database
   │                              │                           │
   │ 1. Read files                │                           │
   ├─ template.json ────────────►│                           │
   ├─ variables.yml ────────────►│                           │
   └─ agents/*.prompt ──────────►│                           │
                                  │                           │
                                  │ 2. Parse & validate       │
                                  │                           │
                                  │ 3. Resolve variables      │
                                  │ (Go template engine)      │
                                  │                           │
                                  │ 4. Register MCP servers   │
                                  ├──────────────────────────►│
                                  │ CREATE mcp_servers        │
                                  │ CREATE file_mcp_configs   │
                                  │                           │
                                  │ 5. Create agents          │
                                  ├──────────────────────────►│
                                  │ CREATE agents             │
                                  │ CREATE agent_tools        │
                                  │                           │
                                  │ 6. Discover tools         │
                                  │ (async MCP connections)   │
                                  │                           │
                                  ├──────────────────────────►│
                                  │ CREATE mcp_tools          │
                                  │                           │
                                  │ 7. Return sync result     │
                                  │◄──────────────────────────┤
                                  │                           │


AGENT EXECUTION DATA FLOW
═══════════════════════════════════════════════════════════════════════

Request Source          Service Layer              Execution              Database
   │                        │                          │                     │
   │ 1. Agent request        │                          │                     │
   ├──────────────────────►  │ Load agent config        │                     │
   │                         ├──────────────────────────────────────────────►│
   │                         │                                               │ Query
   │                         │◄──────────────────────────────────────────────┤
   │                         │                                               │
   │                         │ 2. Initialize GenKit                          │
   │                         │ (with AI provider)                            │
   │                         │                                               │
   │                         │ 3. Initialize MCP connections                │
   │                         │ (load MCP servers)                            │
   │                         │                          │                     │
   │                         │                          ├─────────────────────►
   │                         │                          │ Query servers       │
   │                         │                          │ Query tools         │
   │                         │                          │◄─────────────────────
   │                         │                                               │
   │                         │ 4. Discover tools                             │
   │                         │ (from all MCP servers)                        │
   │                         │                                               │
   │                         │ 5. Execute agent                              │
   │                         │ (with GenKit)                                 │
   │                         │                                               │
   │                         ├──────────────────────────────────────────────►│
   │                         │ CREATE agent_run                              │
   │                         │◄──────────────────────────────────────────────┤
   │                         │                         runID                 │
   │                         │                                               │
   │                         │ 6. Execute steps                              │
   │                         │ (call tools, process results)                 │
   │                         │                                               │
   │                         ├──────────────────────────────────────────────►│
   │                         │ UPDATE agent_run                              │
   │                         │ (add execution_steps, tool_calls)             │
   │                         │◄──────────────────────────────────────────────┤
   │                         │                                               │
   │                         │ 7. Completion                                 │
   │                         ├──────────────────────────────────────────────►│
   │                         │ UPDATE agent_run (final)                      │
   │                         │ ├─ final_response                             │
   │                         │ ├─ status                                     │
   │                         │ ├─ completed_at                               │
   │                         │ ├─ input_tokens                               │
   │                         │ └─ output_tokens                              │
   │                         │◄──────────────────────────────────────────────┤
   │                         │                                               │
   │                         │ 8. Send to Lighthouse (optional)              │
   │                         │ (if configured)                               │
   │                         │                                               │
   │ 9. Return result        │                                               │
   │◄─────────────────────────                                               │


MCP TOOL DISCOVERY FLOW
═══════════════════════════════════════════════════════════════════════

File System                  MCP Server              GenKit Lib         Database
   │                            │                        │                 │
   │ 1. template.json           │                        │                 │
   ├──────────────────────────►│                        │                 │
   │ (MCP server definitions)   │                        │                 │
   │                            │                        │                 │
   │                            │ 2. Connect to servers  │                 │
   │                            ├───────────────────────►│                 │
   │                            │                        │                 │
   │                            │ 3. Call tools call     │                 │
   │                            │ (list_tools)           │                 │
   │                            │◄───────────────────────┤                 │
   │                            │ []Tool returned        │                 │
   │                            │                        │                 │
   │                            │ 4. Map tools to server │                 │
   │                            │ Save tool metadata     │                 │
   │                            ├────────────────────────────────────────►│
   │                            │ INSERT mcp_tools                        │
   │                            │◄────────────────────────────────────────┤
   │                            │                        │                 │


VARIABLE RESOLUTION FLOW
═══════════════════════════════════════════════════════════════════════

File System          Declarative Sync      Go Template Engine      Database
   │                        │                       │                   │
   │ 1. template.json       │                       │                   │
   ├──────────────────────►│                       │                   │
   │ (contains {{ .VAR }})  │                       │                   │
   │                        │ 2. Load variables.yml │                   │
   │ variables.yml          │                       │                   │
   ├──────────────────────►│                       │                   │
   │ KEY: value             │                       │                   │
   │                        │ 3. Create variable    │                   │
   │                        │    context            │                   │
   │                        ├──────────────────────►│                   │
   │                        │ (map[string]interface)│                   │
   │                        │                       │                   │
   │                        │ 4. Execute template   │                   │
   │                        │    on config strings  │                   │
   │                        │◄──────────────────────┤                   │
   │                        │ Resolved JSON         │                   │
   │                        │                       │                   │
   │                        │ 5. Save resolved      │                   │
   │                        │    config             │                   │
   │                        ├───────────────────────────────────────────►│
   │                        │                       │ INSERT/UPDATE     │


AGENT CREATION FROM .PROMPT FILE
═══════════════════════════════════════════════════════════════════════

Agent .prompt File        Agent File Sync         Database
   │                            │                    │
   │ 1. agents/MyAgent.prompt   │                    │
   ├───────────────────────────►│                    │
   │ ---                        │                    │
   │ metadata:                  │                    │
   │   name: "My Agent"         │                    │
   │   description: "..."       │ 2. Parse frontmatter
   │ model: gpt-4o              │    (YAML)           │
   │ max_steps: 8               │                    │
   │ tools:                     │                    │
   │   - __tool1                │ 3. Parse body       │
   │   - __tool2                │    (system prompt)  │
   │ ---                        │                    │
   │ {{role "system"}}          │                    │
   │ You are an agent...        │ 4. Create Agent    │
   │                            │    object           │
   │                            ├───────────────────►│
   │                            │ INSERT agents      │
   │                            │◄───────────────────┤
   │                            │ agent_id            │
   │                            │                    │
   │                            │ 5. Assign tools    │
   │                            ├───────────────────►│
   │                            │ INSERT agent_tools │
   │                            │◄───────────────────┤


EXECUTION METADATA CAPTURE
═══════════════════════════════════════════════════════════════════════

Execution Engine          GenKit Output         Database
   │                            │                    │
   │ Start execution            │                    │
   │ startTime = now()          │                    │
   │                            │                    │
   │ Execute agent              │                    │
   ├───────────────────────────►│                    │
   │ (with tools & task)        │                    │
   │                            │                    │
   │ Tool call 1                │                    │
   │◄───────────────────────────┤                    │
   │ { tool, input, output }    │                    │
   │ Record: toolCalls[1]       │                    │
   │ Record: steps[1]           │                    │
   │                            │                    │
   │ Tool call 2                │                    │
   │◄───────────────────────────┤                    │
   │ { tool, input, output }    │                    │
   │ Record: toolCalls[2]       │                    │
   │ Record: steps[2]           │                    │
   │ ...                        │                    │
   │                            │                    │
   │ Final response             │                    │
   │◄───────────────────────────┤                    │
   │ Capture: final_response    │                    │
   │ Capture: token_usage       │                    │
   │ Capture: duration          │                    │
   │                            │                    │
   │ Save execution metadata    │                    │
   ├────────────────────────────────────────────────►│
   │ UPDATE agent_runs:         │                    │
   │  ├─ execution_steps (JSON) │                    │
   │  ├─ tool_calls (JSON)      │                    │
   │  ├─ input_tokens           │                    │
   │  ├─ output_tokens          │                    │
   │  ├─ final_response         │                    │
   │  └─ completed_at           │                    │
   │                            │◄────────────────────
   │                            │ Success             │
```

---

## 9. Component Dependencies Map

```
┌──────────────────────────────────────────────────────────────────────┐
│                   COMPONENT DEPENDENCIES                              │
│        How services depend on and interact with each other           │
└──────────────────────────────────────────────────────────────────────┘

EXECUTION LAYER DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  AgentService
  ├─ depends on: AgentExecutionEngine
  ├─ depends on: TelemetryService
  ├─ depends on: AgentExportService
  ├─ depends on: Repositories (for agent queries)
  └─ interface: AgentServiceInterface (used by: API, MCP, CLI)

  AgentExecutionEngine
  ├─ depends on: GenKitProvider
  ├─ depends on: MCPConnectionManager
  ├─ depends on: TelemetryService (spans)
  ├─ depends on: LighthouseClient (optional)
  ├─ depends on: DeploymentContextService
  ├─ depends on: Repositories
  └─ used by: AgentService, CLI handlers, API handlers, MCP handlers

  GenKitProvider
  ├─ depends on: config.Config
  ├─ depends on: OpenAI SDK (for OpenAI provider)
  ├─ depends on: GoogleAI SDK (for Gemini provider)
  └─ used by: AgentExecutionEngine


MCP & TOOL MANAGEMENT DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  MCPConnectionManager
  ├─ depends on: Repositories
  ├─ depends on: GenKit library (for MCP client)
  ├─ depends on: MCPServerPool (for pooling)
  ├─ uses: EnvironmentToolCache
  └─ used by: AgentExecutionEngine, DeclarativeSync

  MCPToolDiscovery
  ├─ depends on: Repositories
  ├─ depends on: MCPConnectionManager
  ├─ depends on: GenKit library
  └─ used by: DeclarativeSync, ToolDiscoveryService

  ToolDiscoveryService
  ├─ depends on: Repositories
  ├─ depends on: MCPToolDiscovery (indirect via DeclarativeSync)
  ├─ depends on: MCPConnectionManager
  └─ used by: API handlers, AgentExecutionEngine

  MCPServerManagementService
  ├─ depends on: Repositories
  └─ used by: DeclarativeSync, API handlers

  MCP Server (internal/mcp/server.go)
  ├─ depends on: ToolDiscoveryService
  ├─ depends on: AgentServiceInterface
  ├─ depends on: AuthService
  ├─ depends on: AgentExportService
  ├─ depends on: Repositories
  └─ provides: Tool/Resource/Prompt handlers


CONFIGURATION & ENVIRONMENT DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  DeclarativeSync
  ├─ depends on: Repositories
  ├─ depends on: config.Config
  ├─ depends on: MCPToolDiscovery
  ├─ depends on: AgentFileSync
  ├─ depends on: VariableResolver
  ├─ depends on: GenKit (for MCP connections)
  └─ used by: CLI sync command, API sync handler, UI interactive sync

  EnvironmentManagementService
  ├─ depends on: Repositories
  ├─ depends on: config.Config (for paths)
  └─ used by: API handlers, MCP handlers, CLI env command

  AgentFileSync
  ├─ depends on: Repositories
  ├─ depends on: config.Config (for paths)
  └─ used by: DeclarativeSync, CLI handlers

  EnvironmentCopyService
  ├─ depends on: Repositories
  ├─ depends on: config.Config
  └─ used by: API handlers, MCP handlers

  BundleService
  ├─ depends on: config.Config
  ├─ depends on: Repositories (optional)
  ├─ depends on: version.Version
  └─ used by: API bundle handlers, CLI bundle commands


SCHEDULING & AUTOMATION DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  SchedulerService
  ├─ depends on: db.Database
  ├─ depends on: AgentServiceInterface
  ├─ uses: robfig/cron library
  └─ used by: server.go (main), up.go

  DeploymentContextService
  └─ used by: AgentExecutionEngine (for metadata)


MONITORING & TELEMETRY DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  TelemetryService
  ├─ depends on: config.Config
  ├─ depends on: OpenTelemetry SDK
  └─ used by: AgentService, AgentExecutionEngine, API handlers

  LighthouseClient
  ├─ depends on: network (gRPC connection)
  └─ used by: AgentExecutionEngine (optional), MCP handlers


REPOSITORY LAYER DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  Repositories (collection)
  ├─ AgentRepo - queries agents table
  ├─ AgentRunRepo - queries agent_runs table
  ├─ AgentToolRepo - queries agent_tools table
  ├─ EnvironmentRepo - queries environments table
  ├─ MCPServerRepo - queries mcp_servers table
  ├─ MCPToolRepo - queries mcp_tools table
  ├─ FileMCPConfigRepo - queries file_mcp_configs table
  ├─ UserRepo - queries users table
  ├─ ModelProviderRepository - queries model_providers table
  ├─ ModelRepository - queries models table
  └─ SettingsRepo - queries settings table

  All depend on: sql.DB (SQLite connection)
  Used by: All services, API handlers, CLI handlers


API HANDLER DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  APIHandlers
  ├─ depends on: Repositories
  ├─ depends on: AgentServiceInterface
  ├─ depends on: ToolDiscoveryService
  ├─ depends on: TelemetryService
  ├─ depends on: gin.Engine (for HTTP)
  └─ handlers:
     ├─ agents.go → AgentServiceInterface
     ├─ agent_runs.go → AgentRunRepo
     ├─ environments.go → EnvironmentManagementService
     ├─ tools.go → ToolDiscoveryService, MCPToolRepo
     ├─ mcp_management.go → DeclarativeSync
     ├─ bundles.go → BundleService
     ├─ sync_interactive.go → DeclarativeSync
     ├─ settings.go → SettingsRepo
     └─ demo_bundles.go → BundleService


CLI HANDLER DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  CLI Commands (cmd/main/)
  └─ agent.go → handlers/agent/
  └─ env.go → EnvironmentManagementService (indirectly)
  └─ bundle.go → BundleService (indirectly)
  └─ runs.go → AgentRunRepo (indirectly)
  └─ server.go → API Server setup
  └─ up.go → Full system initialization

  Agent Handlers (handlers/agent/)
  ├─ execution.go → AgentExecutionEngine
  ├─ local.go → AgentExecutionEngine, Repositories
  └─ utils.go → Helper functions


INITIALIZATION DEPENDENCIES
═══════════════════════════════════════════════════════════════════════

  Main Entry (main.go)
  ├─ Load Config
  │  └─ depends on: config module
  │
  ├─ Initialize Database
  │  └─ depends on: db module, Config
  │
  ├─ Create Repositories
  │  └─ depends on: Database
  │
  ├─ Initialize GenKit
  │  └─ depends on: Config, GenKit SDK
  │
  ├─ Create Services
  │  ├─ AgentService
  │  ├─ DeclarativeSync
  │  ├─ EnvironmentManagementService
  │  └─ SchedulerService
  │
  ├─ Start Scheduler (if needed)
  │  └─ depends on: SchedulerService
  │
  ├─ Start API Server (if --server)
  │  └─ depends on: APIHandlers, all services
  │
  └─ Start MCP Server
     └─ depends on: MCP Server, all services


CIRCULAR DEPENDENCY HANDLING
═══════════════════════════════════════════════════════════════════════

  AgentService ↔ AgentExecutionEngine
  └─ Solution: AgentService creates and holds AgentExecutionEngine
  └─ AgentExecutionEngine uses AgentServiceInterface (not concrete AgentService)
  └─ Breaks cycle through interface abstraction


  API handlers ↔ Services
  └─ Solution: Clear dependency direction (handlers → services)
  └─ Services don't know about HTTP handlers


  DeclarativeSync ↔ AgentFileSync
  └─ Solution: DeclarativeSync uses AgentFileSync as helper
  └─ Clean separation of concerns
```

---

## 10. Key File Locations Reference

```
┌──────────────────────────────────────────────────────────────────────┐
│                   KEY FILES LOCATION REFERENCE                        │
│              Quick lookup for major components                        │
└──────────────────────────────────────────────────────────────────────┘

AGENT EXECUTION CORE
═══════════════════════════════════════════════════════════════════════
  /internal/services/agent_execution_engine.go           (743 lines)
  /internal/services/agent_service_impl.go               (586 lines)
  /internal/services/agent_service_interface.go          (48 lines)


MCP MANAGEMENT
═══════════════════════════════════════════════════════════════════════
  /internal/services/mcp_connection_manager.go           (20KB)
  /internal/services/mcp_tool_discovery.go               (655 lines)
  /internal/services/mcp_server_management_service.go    (705 lines)
  /internal/mcp/server.go                                (Core MCP server)
  /internal/mcp/tool_discovery.go
  /internal/mcp/*_handlers.go                            (Tool/Resource/Prompt handlers)


AGENT EXECUTION PATHS
═══════════════════════════════════════════════════════════════════════
  /cmd/main/handlers/agent/execution.go                  (CLI execution)
  /cmd/main/handlers/agent/local.go                      (Local execution logic)
  /internal/api/v1/agents.go                             (API execution endpoint)
  /internal/mcp/execution_handlers.go                    (MCP execution tool)


CONFIGURATION & ENVIRONMENT
═══════════════════════════════════════════════════════════════════════
  /internal/services/declarative_sync.go                 (868 lines)
  /internal/services/environment_management_service.go   (245 lines)
  /internal/services/agent_file_sync.go                  (871 lines)
  /internal/services/environment_copy_service.go         (430 lines)
  /internal/services/bundle_service.go                   (881 lines)


DATABASE LAYER
═══════════════════════════════════════════════════════════════════════
  /internal/db/db.go                                     (Database interface)
  /internal/db/schema.sql                                (Schema definition)
  /internal/db/migrations/                               (Migration files)
  /internal/db/repositories/base.go                      (Repository collection)
  /internal/db/repositories/*.go                         (Entity repositories)
  /internal/db/queries/db.go                             (Auto-generated queries)


API HANDLERS
═══════════════════════════════════════════════════════════════════════
  /internal/api/api.go                                   (Server setup)
  /internal/api/v1/handlers.go                           (Handler entry point)
  /internal/api/v1/agents.go                             (Agent handlers)
  /internal/api/v1/agent_runs.go                         (Run handlers)
  /internal/api/v1/environments.go                       (Environment handlers)
  /internal/api/v1/tools.go                              (Tool handlers)
  /internal/api/v1/mcp_management.go                     (MCP management)
  /internal/api/v1/sync_interactive.go                   (Interactive sync)
  /internal/api/v1/bundles.go                            (Bundle endpoints)
  /internal/api/v1/settings.go                           (Settings endpoints)


CLI COMMANDS
═══════════════════════════════════════════════════════════════════════
  /cmd/main/main.go                                      (Entry point)
  /cmd/main/cli.go                                       (Command setup)
  /cmd/main/agent.go                                     (Agent commands)
  /cmd/main/handlers/agent/execution.go                  (Agent execution)
  /cmd/main/handlers/agent/local.go                      (Local run logic)
  /cmd/main/server.go                                    (Server command)
  /cmd/main/up.go                                        (Startup command)
  /cmd/main/runs.go                                      (Runs command)


SCHEDULING & MONITORING
═══════════════════════════════════════════════════════════════════════
  /internal/services/scheduler.go                        (590 lines)
  /internal/services/telemetry_service.go                (Observability)
  /internal/services/deployment_context_service.go       (727 lines)


UI & CONFIGURATION
═══════════════════════════════════════════════════════════════════════
  /internal/config/config.go                             (Config loading)
  /internal/ui/                                          (Embedded UI files)
  /internal/tui/                                         (Terminal UI)


EXTERNAL INTEGRATIONS
═══════════════════════════════════════════════════════════════════════
  /internal/lighthouse/                                  (Lighthouse/CloudShip)
  /internal/auth/                                        (Authentication)
  /internal/logging/                                     (Logging)
  /internal/telemetry/                                   (OpenTelemetry)


MODELS & SCHEMAS
═══════════════════════════════════════════════════════════════════════
  /pkg/models/                                           (Data models)
  /pkg/schema/                                           (JSON schema validation)
  /pkg/dotprompt/                                        (Agent prompt parsing)
  /internal/schemas/                                     (Schema registry)


UTILITIES
═══════════════════════════════════════════════════════════════════════
  /pkg/utils/                                            (General utilities)
  /internal/utils/                                       (Internal utilities)
  /internal/version/                                     (Version info)
```

---

## Summary

This comprehensive architecture documentation covers:

1. **High-Level System** - Complete platform overview with all layers
2. **Agent Execution** - Four execution paths (CLI, API, MCP, Scheduler)
3. **Service Layer** - 43+ focused service modules with clear responsibilities
4. **MCP Architecture** - Model Context Protocol server structure and handlers
5. **Database Layer** - 11 repositories with 30+ tables
6. **API Handlers** - RESTful endpoints across multiple domains
7. **CLI Structure** - Command hierarchy and handler organization
8. **Data Flows** - Synchronization, execution, discovery, and variable resolution
9. **Dependencies** - How components depend on and interact with each other
10. **File Locations** - Quick reference for finding specific components

Key architectural principles:
- Modular services with single responsibilities
- Clean separation between CLI, API, and MCP layers
- File-based configuration with database synchronization
- Comprehensive execution metadata capture
- Support for multiple AI providers and MCP servers
- Graceful error handling and cleanup

