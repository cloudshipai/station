# Station Architecture Documentation - Complete Index

Comprehensive documentation of the Station platform architecture with ASCII diagrams and detailed analysis.

## Documentation Files

This architecture documentation consists of three comprehensive markdown files:

### 1. **ARCHITECTURE_DIAGRAMS.md** (2,015 lines)
Complete visual representations of system architecture using ASCII diagrams.

**Contents:**
- High-level system architecture overview
- Agent execution flow (4 paths)
- Service layer architecture (43+ services)
- MCP server subsystem
- Database layer & repositories
- API handler architecture
- CLI command structure
- Data flow patterns (4 major flows)
- Component dependencies map
- Key file locations reference

**Best for:** Understanding how components fit together, visual learners, quick navigation

### 2. **ARCHITECTURE_ANALYSIS.md** (832 lines)
In-depth technical analysis with execution details and design decisions.

**Contents:**
- Executive summary
- Four-layer architecture overview
- Execution paths (CLI, API, MCP, Scheduler)
- Core service architecture breakdown
- MCP server subsystem details
- Database layer explanation
- API endpoint organization
- CLI command hierarchy
- Data flow patterns explained
- Architectural decisions & rationale
- Configuration system
- Security considerations
- Performance optimizations
- Extensibility points
- Known limitations & future work
- Key takeaways

**Best for:** Understanding design decisions, architectural rationale, finding details about specific components

### 3. **COMPONENT_INTERACTIONS.md** (710 lines)
Detailed sequence diagrams showing how components interact.

**Contents:**
- Agent execution sequence diagram
- Environment synchronization sequence
- MCP tool discovery flow
- Scheduled agent execution
- Variable resolution process
- API request processing
- MCP tool handler flow
- Error handling & recovery
- Bundle creation & export
- Component lifecycle diagram

**Best for:** Understanding execution flows, debugging, tracing requests through system

## Quick Navigation

### Finding Information by Topic

**Agent Execution**
- How agents execute: ARCHITECTURE_DIAGRAMS.md § 2 / ARCHITECTURE_ANALYSIS.md § Agent Execution
- Step-by-step sequence: COMPONENT_INTERACTIONS.md § 1
- Code locations: ARCHITECTURE_DIAGRAMS.md § 10

**MCP Integration**
- System overview: ARCHITECTURE_DIAGRAMS.md § 4
- Handler details: ARCHITECTURE_ANALYSIS.md § MCP Server
- Tool discovery: COMPONENT_INTERACTIONS.md § 3, § 6

**Configuration & Environment**
- File-based system: ARCHITECTURE_ANALYSIS.md § File-Based Configuration
- Synchronization flow: ARCHITECTURE_DIAGRAMS.md § 8 / COMPONENT_INTERACTIONS.md § 2
- Variable resolution: ARCHITECTURE_DIAGRAMS.md § 8 / COMPONENT_INTERACTIONS.md § 5

**Database**
- Schema: ARCHITECTURE_DIAGRAMS.md § 5
- Repositories: ARCHITECTURE_ANALYSIS.md § Database Layer
- Data persistence: COMPONENT_INTERACTIONS.md § 10

**API Server**
- Endpoints: ARCHITECTURE_DIAGRAMS.md § 6
- Handler organization: ARCHITECTURE_ANALYSIS.md § API Layer
- Request flow: COMPONENT_INTERACTIONS.md § 7

**CLI Commands**
- Command tree: ARCHITECTURE_DIAGRAMS.md § 7
- Command structure: ARCHITECTURE_ANALYSIS.md § CLI Layer

**Data Flows**
- Environment sync: ARCHITECTURE_DIAGRAMS.md § 8.1
- Agent execution: ARCHITECTURE_DIAGRAMS.md § 8.2
- Tool discovery: ARCHITECTURE_DIAGRAMS.md § 8.3
- Variable resolution: ARCHITECTURE_DIAGRAMS.md § 8.4

**Services**
- Complete list: ARCHITECTURE_DIAGRAMS.md § 3
- Detailed breakdown: ARCHITECTURE_ANALYSIS.md § Core Service Architecture
- Dependencies: ARCHITECTURE_DIAGRAMS.md § 9

**Security**
- Overview: ARCHITECTURE_ANALYSIS.md § Security Considerations
- Configuration handling: ARCHITECTURE_ANALYSIS.md § Configuration System

**Performance**
- Optimizations: ARCHITECTURE_ANALYSIS.md § Performance Optimizations
- Connection pooling: ARCHITECTURE_ANALYSIS.md § Key Architectural Decisions

## Key Concepts Quick Reference

### Four Execution Paths
1. **CLI**: `stn agent run <name> <task>` - Direct local execution
2. **API**: `POST /api/v1/agents/:id/execute` - Network-accessible
3. **MCP**: `mcp_station_agent_run` tool - Integrated with Claude
4. **Scheduler**: Cron-triggered - Background execution

All converge at: **AgentExecutionEngine**

### Four Layers of Architecture
1. **Presentation**: CLI, API, MCP Server
2. **Service**: 43+ focused modules handling business logic
3. **Data Access**: 11 repositories for database interaction
4. **Persistence**: SQLite database + file system

### Core Services (by category)
- **Execution** (3): AgentService, AgentExecutionEngine, GenKitProvider
- **MCP Management** (3): MCPConnectionManager, MCPToolDiscovery, ToolDiscoveryService
- **Configuration** (4): DeclarativeSync, EnvironmentManagementService, AgentFileSync, EnvironmentCopyService
- **Scheduling** (1): SchedulerService
- **Telemetry** (2): TelemetryService, DeploymentContextService

### File-Based Configuration Structure
```
~/.config/station/environments/<env-name>/
├─ template.json      # MCP server definitions (resolved)
├─ variables.yml      # Template variables (Go templates)
└─ agents/
   ├─ Agent1.prompt   # YAML frontmatter + system prompt
   ├─ Agent2.prompt
   └─ ...
```

### Database Core Tables (11)
- users, environments, agents, agent_tools, agent_runs
- mcp_servers, mcp_tools, file_mcp_configs
- model_providers, models, settings

### API Endpoints (60+)
- `/agents` - Agent CRUD & execution
- `/agent-runs` - Execution history
- `/environments` - Environment management
- `/tools` - Tool discovery
- `/mcp-servers` - MCP management
- `/bundles` - Bundle operations
- `/sync` - Environment synchronization
- `/settings` - Configuration
- And more...

### CLI Commands (15+)
- `stn agent` - Agent operations
- `stn env` - Environment management
- `stn sync` - Synchronization
- `stn bundle` - Bundling
- `stn runs` - Execution history
- `stn server` - Start API server
- `stn up` / `stn down` - System lifecycle

## Architecture Principles

### Modular Design
- 43+ focused service modules
- Each module <500 lines for maintainability
- Clear separation of concerns

### Unified Execution
- All entry points converge at AgentExecutionEngine
- Consistent behavior across CLI, API, MCP, Scheduler
- Single source of truth for execution logic

### File + Database
- Source of truth: Files on disk
- Database: Queryable copy for runtime operations
- Bidirectional synchronization
- GitOps-friendly

### Comprehensive Metadata
- Complete execution capture
- Tool calls with parameters
- Execution steps with timing
- Token usage tracking
- Error information

### Clear Interfaces
- AgentServiceInterface for execution
- VariableResolver for custom variable handling
- Repository pattern for data access
- Handler pattern for API/CLI/MCP

## Typical Workflows

### Create & Execute Agent (Quick Start)

1. **Create environment**
   ```bash
   stn env create my-project
   ```

2. **Add agent definition** to `~/.config/station/environments/my-project/agents/MyAgent.prompt`
   ```yaml
   ---
   metadata:
     name: "My Agent"
   model: gpt-4o-mini
   max_steps: 8
   tools:
     - __read_text_file
     - __list_directory
   ---
   {{role "system"}}
   You are an expert analyst...
   ```

3. **Sync environment**
   ```bash
   stn sync my-project
   ```

4. **Execute agent**
   ```bash
   stn agent run MyAgent "Analyze the project"
   ```

5. **Check results**
   ```bash
   stn runs list
   stn runs inspect <run-id> -v
   ```

### Add MCP Server

1. **Edit template.json** in environment directory
   ```json
   {
     "mcpServers": {
       "filesystem": {
         "command": "npx",
         "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/path"]
       }
     }
   }
   ```

2. **Sync to discover tools**
   ```bash
   stn sync <env>
   ```

3. **Tools automatically available** to agents

### Setup Scheduled Execution

1. **Edit agent via API/UI** to enable schedule
   ```json
   {
     "cron_schedule": "0 0 * * *",
     "schedule_enabled": true
   }
   ```

2. **Server runs** `stn server`
3. **Scheduler loads** on startup
4. **Executes** at specified time

## Performance Tips

1. **Enable connection pooling**
   ```bash
   export STATION_MCP_POOLING=true
   stn server
   ```

2. **Reuse environments** for repeated tasks
3. **Use tool filtering** to reduce discovery overhead
4. **Cache agent results** if deterministic

## Debugging Guide

### To understand execution flow
1. Run with verbose logging: `stn agent run <name> <task> --verbose`
2. Check agent run details: `stn runs inspect <run-id> -v`
3. Review tool calls and steps in run metadata

### To troubleshoot sync issues
1. Run dry-run first: `stn sync <env> --dry-run`
2. Check variables resolved: examine `variables.yml`
3. Verify MCP servers: check `template.json` syntax
4. See tool discovery: check `mcp_tools` table

### To debug MCP server issues
1. Check server config: `~/.config/station/environments/<env>/template.json`
2. Verify server can start manually
3. Check variables resolved properly
4. Review MCP server logs

## Extension Points

**Add AI Provider**: Update GenKitProvider
**Add Tool Type**: Create MCP server implementation
**Add Handler**: Implement and register route/command
**Add Telemetry**: Integrate with TelemetryService
**Custom Variables**: Implement VariableResolver interface

## File Locations Cheat Sheet

| Component | Location |
|-----------|----------|
| Agent execution | `/internal/services/agent_execution_engine.go` |
| MCP management | `/internal/services/mcp_*.go` |
| Sync service | `/internal/services/declarative_sync.go` |
| API handlers | `/internal/api/v1/*.go` |
| CLI handlers | `/cmd/main/handlers/` |
| Repositories | `/internal/db/repositories/` |
| Database schema | `/internal/db/schema.sql` |
| MCP server | `/internal/mcp/server.go` |
| Config loading | `/internal/config/config.go` |

## Summary

Station is a **modular, service-oriented AI agent orchestration platform** with:
- ✓ Multiple execution paths (CLI, API, MCP, Scheduler)
- ✓ Unified execution engine
- ✓ File-based GitOps configuration
- ✓ Comprehensive metadata capture
- ✓ Full MCP integration
- ✓ Multi-provider AI model support
- ✓ Clear, maintainable architecture

For questions about specific components, refer to:
1. **Architecture overview**: ARCHITECTURE_DIAGRAMS.md
2. **Design decisions**: ARCHITECTURE_ANALYSIS.md
3. **Execution flows**: COMPONENT_INTERACTIONS.md

Happy architecting!

