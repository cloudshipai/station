# Station Component Interactions - Detailed Diagrams

Detailed interaction sequences and component communication patterns.

---

## 1. Agent Execution Sequence Diagram

```
┌─────────┐        ┌──────────────────┐      ┌─────────────────┐      ┌────────────────┐
│ CLI/API │        │ AgentService     │      │ AgentExecution  │      │ Database       │
│ Request │        │                  │      │ Engine          │      │               │
└────┬────┘        └──────────────────┘      └─────────────────┘      └────────────────┘
     │                                                │                        │
     │ 1. ExecuteAgent(agentID, task)               │                        │
     ├───────────────────────────────────────────►│                        │
     │                                              │                        │
     │                                              │ 2. Load agent config   │
     │                                              ├───────────────────────►│
     │                                              │ Query agents table     │
     │                                              │◄───────────────────────┤
     │                                              │ Agent object           │
     │                                              │                        │
     │                                              │ 3. Get environment     │
     │                                              ├───────────────────────►│
     │                                              │ Query environments     │
     │                                              │◄───────────────────────┤
     │                                              │ Environment details    │
     │                                              │                        │
     │                                              │ 4. Get MCP servers    │
     │                                              ├───────────────────────►│
     │                                              │ Query mcp_servers      │
     │                                              │◄───────────────────────┤
     │                                              │ Server configs         │
     │                                              │                        │
     │                                              │ 5. Initialize GenKit   │
     │                                              │ with AI provider       │
     │                                              │                        │
     │                                              │ 6. Create MCPConnMgr   │
     │                                              │ Connect to servers     │
     │                                              │ Discover tools (cache) │
     │                                              │                        │
     │                                              │ 7. Create agent_run    │
     │                                              ├───────────────────────►│
     │                                              │ INSERT agent_runs      │
     │                                              │◄───────────────────────┤
     │                                              │ runID                  │
     │                                              │                        │
     │                                              │ 8. Execute with GenKit │
     │                                              │ (iterate tools)        │
     │                                              │ Tool 1: list_files     │
     │                                              │ Tool 2: read_file      │
     │                                              │ ... (max_steps limit)  │
     │                                              │                        │
     │                                              │ 9. Update agent_run    │
     │                                              ├───────────────────────►│
     │                                              │ UPDATE with metadata:  │
     │                                              │ ├─ execution_steps     │
     │                                              │ ├─ tool_calls          │
     │                                              │ ├─ final_response      │
     │                                              │ ├─ input_tokens        │
     │                                              │ ├─ output_tokens       │
     │                                              │ ├─ steps_taken         │
     │                                              │ └─ completed_at        │
     │                                              │◄───────────────────────┤
     │                                              │                        │
     │                                              │ 10. Cleanup            │
     │                                              │ Close MCP connections  │
     │                                              │ Release resources      │
     │                                              │                        │
     │ 11. Return AgentExecutionResult            │                        │
     │◄──────────────────────────────────────────┤                        │
     │ { success, response, toolCalls, ... }     │                        │
```

---

## 2. Environment Synchronization Sequence

```
┌──────────────┐     ┌──────────────────┐     ┌────────────────┐     ┌────────────────┐
│ File System  │     │ DeclarativeSync  │     │ MCPTool        │     │ Database       │
│              │     │                  │     │ Discovery      │     │               │
└────┬─────────┘     └──────────────────┘     └────────────────┘     └────────────────┘
     │                                              │                        │
     │ 1. Read environment files                   │                        │
     ├─────────────────────────────────────────►│                        │
     │ • template.json (MCP servers)              │                        │
     │ • variables.yml (template vars)            │                        │
     │ • agents/*.prompt (agent definitions)      │                        │
     │                                              │                        │
     │                                              │ 2. Parse configs       │
     │                                              │ Parse YAML, JSON       │
     │                                              │                        │
     │                                              │ 3. Resolve variables   │
     │                                              │ Load variables.yml     │
     │                                              │ Apply Go templates     │
     │                                              │ Replace {{ .VAR }}     │
     │                                              │                        │
     │                                              │ 4. Register servers    │
     │                                              ├───────────────────────►│
     │                                              │ INSERT mcp_servers     │
     │                                              │ INSERT file_configs    │
     │                                              │◄───────────────────────┤
     │                                              │ server IDs             │
     │                                              │                        │
     │                                              │ 5. Create agents       │
     │                                              ├───────────────────────►│
     │                                              │ INSERT agents          │
     │                                              │ INSERT agent_tools     │
     │                                              │◄───────────────────────┤
     │                                              │ agent IDs              │
     │                                              │                        │
     │                                              │ 6. Discover tools      │
     │                                              ├──────────────────────►│
     │                                              │ Connect to servers     │
     │                                              │ Call tools endpoint    │
     │                                              │◄──────────────────────┤
     │                                              │ []Tool per server      │
     │                                              │                        │
     │                                              │ 7. Save tools          │
     │                                              ├───────────────────────►│
     │                                              │ INSERT mcp_tools       │
     │                                              │◄───────────────────────┤
     │                                              │                        │
     │                                              │ 8. Cleanup connections │
     │                                              │ Close MCP clients      │
     │                                              │                        │
     │ 9. Return SyncResult                      │                        │
     │◄──────────────────────────────────────────┤                        │
     │ { agents: 5, tools: 47, status: ok }      │                        │
```

---

## 3. MCP Tool Discovery Flow

```
┌──────────────────────┐     ┌──────────────────┐     ┌─────────────┐     ┌────────────┐
│ File Config          │     │ MCPConnection    │     │ GenKit MCP  │     │ Database   │
│ (template.json)      │     │ Manager          │     │ Client      │     │           │
└────┬─────────────────┘     └──────────────────┘     └─────────────┘     └────────────┘
     │                                │                     │                   │
     │ 1. MCP server definition      │                     │                   │
     ├─────────────────────────────►│                     │                   │
     │ {                             │                     │                   │
     │   "filesystem": {             │                     │                   │
     │     "command": "npx",         │                     │                   │
     │     "args": [...]            │                     │                   │
     │   }                           │                     │                   │
     │ }                             │                     │                   │
     │                               │ 2. Parse config     │                   │
     │                               │                     │                   │
     │                               │ 3. Create connection│                   │
     │                               ├────────────────────►│                   │
     │                               │                     │ Initialize        │
     │                               │                     │ stdio/http        │
     │                               │                     │ transport         │
     │                               │                     │                   │
     │                               │◄────────────────────┤                   │
     │                               │ Connection ready    │                   │
     │                               │                     │                   │
     │                               │ 4. Call list_tools  │                   │
     │                               │ (MCP tool)          │                   │
     │                               ├────────────────────►│                   │
     │                               │                     │ Execute tools     │
     │                               │                     │ resource          │
     │                               │◄────────────────────┤                   │
     │                               │ []ToolDefinition    │                   │
     │                               │ {                   │                   │
     │                               │   name: "read_file",│                   │
     │                               │   description: "...",
     │                               │   inputSchema: {...}│
     │                               │ }                   │                   │
     │                               │                     │                   │
     │                               │ 5. Map tools to srv │                   │
     │                               │ (this server)       │                   │
     │                               │ Save metadata       │                   │
     │                               ├───────────────────────────────────────►│
     │                               │                     │ INSERT mcp_tools  │
     │                               │                     │ (server_id ref)   │
     │                               │                     │◄───────────────────
     │                               │                     │                   │
     │                               │ 6. Cache tools      │                   │
     │                               │ (per environment)   │                   │
     │                               │ Set TTL             │                   │
     │                               │                     │                   │
     │                               │ 7. Connection ready │                   │
     │                               │ for agent exec      │                   │
```

---

## 4. Scheduled Agent Execution

```
┌──────────────┐     ┌─────────────────┐     ┌───────────────┐     ┌────────────┐
│ Cron Trigger │     │ SchedulerService│     │ AgentService  │     │ Database   │
│ (robfig/cron)│     │                 │     │               │     │           │
└────┬─────────┘     └─────────────────┘     └───────────────┘     └────────────┘
     │                                              │                   │
     │ 1. SchedulerService.Start()                │                   │
     │ (on application startup)                   │                   │
     ├──────────────────────────────────────────►│                   │
     │                                              │                   │
     │                                              │ 2. Load scheduled │
     │                                              │ agents from DB    │
     │                                              ├──────────────────►│
     │                                              │ SELECT agents     │
     │                                              │ WHERE sched_enabled
     │                                              │◄──────────────────┤
     │                                              │ [Agent with cron] │
     │                                              │                   │
     │                                              │ 3. Register cron  │
     │                                              │ tasks with cron   │
     │                                              │ library           │
     │                                              │                   │
     │ 4. [Cron trigger time]                    │                   │
     │ "0 0 * * * " (midnight daily)              │                   │
     ├──────────────────────────────────────────►│                   │
     │ Cron.Run(entryID)                          │                   │
     │                                              │                   │
     │                                              │ 5. Execute agent  │
     │                                              │ (no special flags) │
     │                                              │ Uses default task │
     │                                              │                   │
     │                                              │ [Execution flow]  │
     │                                              │ (same as #1)      │
     │                                              │                   │
     │                                              │ 6. Create run     │
     │                                              ├──────────────────►│
     │                                              │ INSERT agent_runs │
     │                                              │                   │
     │                                              │ 7. Update with    │
     │                                              │ metadata          │
     │                                              ├──────────────────►│
     │                                              │ UPDATE agent_runs │
     │                                              │ (same as #1)      │
     │                                              │                   │
     │ [Next scheduled trigger]                  │                   │
     ├──────────────────────────────────────────►│                   │
     │ (cron library handles timing)              │                   │
```

---

## 5. Variable Resolution Process

```
┌──────────────────┐     ┌─────────────────┐     ┌──────────────────┐     ┌─────────┐
│ File System      │     │ Declarative     │     │ Go Template      │     │Database │
│ template.json    │     │ Sync            │     │ Engine           │     │        │
└────┬─────────────┘     └─────────────────┘     └──────────────────┘     └─────────┘
     │                                                │                        │
     │ 1. Config with variables                     │                        │
     │ {                                            │                        │
     │   "command": "{{ .PYTHON_BIN }}",            │                        │
     │   "args": ["{{ .SCRIPT_PATH }}"]             │                        │
     │ }                                            │                        │
     ├─────────────────────────────────────────►│                        │
     │                                              │ 2. Load variables.yml   │
     │ variables.yml:                               │                        │
     │ PYTHON_BIN: "/usr/bin/python3"              │                        │
     │ SCRIPT_PATH: "/home/user/scripts/run.py"    │                        │
     │                                              │                        │
     │                                              │ 3. Create template      │
     │                                              │ context map             │
     │                                              │ {                       │
     │                                              │   "PYTHON_BIN": "...",  │
     │                                              │   "SCRIPT_PATH": "..."  │
     │                                              │ }                       │
     │                                              │                        │
     │                                              │ 4. Execute template    │
     │                                              ├────────────────────►│
     │                                              │ Parse {{ .PYTHON_BIN}}  │
     │                                              │ Lookup in context      │
     │                                              │◄────────────────────┤
     │                                              │ Resolved: "/usr/bin/..."
     │                                              │                        │
     │                                              │ 5. Build resolved cfg  │
     │                                              │ {                      │
     │                                              │   "command": "/usr/bin/"
     │                                              │   "args": ["/home/..."] │
     │                                              │ }                      │
     │                                              │                        │
     │                                              │ 6. Save to database    │
     │                                              ├───────────────────────►│
     │                                              │ INSERT mcp_servers     │
     │                                              │ (with resolved config) │
     │                                              │◄───────────────────────┤
```

---

## 6. API Request Processing

```
Client                    Gin Router           APIHandler            Services
  │                            │                    │                    │
  │ 1. POST /api/v1/agents/:id/execute               │                    │
  │    { task: "...", variables: {...} }            │                    │
  ├───────────────────────────────────────────────►│                    │
  │                                                  │ 2. Extract params     │
  │                                                  │ Get agent ID from URL │
  │                                                  │ Parse body JSON       │
  │                                                  │                       │
  │                                                  │ 3. Validate request   │
  │                                                  │ Check auth header     │
  │                                                  │ Check permissions     │
  │                                                  │                       │
  │                                                  │ 4. Call AgentService  │
  │                                                  ├──────────────────────►│
  │                                                  │ ExecuteAgent(id, task,│
  │                                                  │ variables)            │
  │                                                  │                       │
  │                                                  │ [Execution]           │
  │                                                  │ (see diagram #1)      │
  │                                                  │                       │
  │                                                  │◄──────────────────────┤
  │                                                  │ AgentExecutionResult  │
  │                                                  │                       │
  │                                                  │ 5. Build response     │
  │                                                  │ HTTP status: 200      │
  │                                                  │ JSON body with result │
  │                                                  │                       │
  │ 6. HTTP 200 OK                                 │                       │
  │ {                                              │                       │
  │   "success": true,                             │                       │
  │   "response": "...",                           │                       │
  │   "tool_calls": [...],                         │                       │
  │   "input_tokens": 1234,                        │                       │
  │   "output_tokens": 567                         │                       │
  │ }                                              │                       │
  │◄──────────────────────────────────────────────┤                       │
```

---

## 7. MCP Tool Handler Flow

```
Claude/MCP Client        MCP Server            Handler             AgentService
      │                       │                    │                    │
      │ 1. mcp_station_agent_run                   │                    │
      │    {                                       │                    │
      │      agentID: 5,                           │                    │
      │      task: "Scan project"                  │                    │
      │    }                                       │                    │
      ├──────────────────────────────────────────►│                    │
      │                                              │ 2. Parse tool args    │
      │                                              │                       │
      │                                              │ 3. Lookup agent       │
      │                                              │ Query database        │
      │                                              │                       │
      │                                              │ 4. Create agent_run   │
      │                                              │ INSERT database entry │
      │                                              │ Get runID             │
      │                                              │                       │
      │                                              │ 5. Execute with runID │
      │                                              ├──────────────────────►│
      │                                              │ ExecuteAgentWithRunID(│
      │                                              │   agentID, task,      │
      │                                              │   runID, variables)   │
      │                                              │                       │
      │                                              │ [Execution]           │
      │                                              │ (see diagram #1)      │
      │                                              │                       │
      │                                              │◄──────────────────────┤
      │                                              │ AgentExecutionResult  │
      │                                              │                       │
      │                                              │ 6. Update agent_run   │
      │                                              │ completion            │
      │                                              │ (already done by      │
      │                                              │  ExecuteAgent)        │
      │                                              │                       │
      │ 7. Return tool result                      │                       │
      │    {                                       │                       │
      │      success: true,                        │                       │
      │      response: "Execution complete",       │                       │
      │      runID: 12345,                         │                       │
      │      toolCalls: [...]                      │                       │
      │    }                                       │                       │
      │◄──────────────────────────────────────────┤                       │
      │                                              │                       │
      │ 8. Query resource for details               │                       │
      │    agent-runs/12345                        │                       │
      ├──────────────────────────────────────────►│                       │
      │                                              │ 9. Read resource      │
      │                                              │ Query database        │
      │                                              │ Format as text        │
      │                                              │                       │
      │ 10. Resource data                          │                       │
      │     (execution details)                    │                       │
      │◄──────────────────────────────────────────┤                       │
```

---

## 8. Error Handling & Recovery

```
Component             Failure Point         Handler               Recovery
    │                      │                   │                     │
    ├─ MCP server doesn't start
    │  (e.g., invalid command)                │                     │
    └──────────────────────────────────────►│                     │
                                               │ Catch error         │
                                               │ Check if config ok  │
                                               │                     │
                                               │ If new config:      │
                                               ├────────────────────►│
                                               │ Cleanup broken      │
                                               │ server from DB      │
                                               │◄────────────────────┤
                                               │                     │
                                               │ If existing config: │
                                               │ Log error           │
                                               │ Suggest user fix    │
                                               │ Keep config         │
                                               │                     │

    ├─ Variable not resolved
    │  (missing from variables.yml)           │                     │
    └──────────────────────────────────────►│                     │
                                               │ In interactive mode:│
                                               │ Show form to user   │
                                               │ Accept input        │
                                               │ Save to variables   │
                                               │ Retry sync          │
                                               │                     │
                                               │ In CLI mode:        │
                                               │ Return error        │
                                               │ Show missing vars   │
                                               │                     │

    ├─ Agent tool discovery fails
    │  (MCP server unreachable)               │                     │
    └──────────────────────────────────────►│                     │
                                               │ Log warning         │
                                               │ Keep existing tools │
                                               │ Continue execution  │
                                               │ (with cached tools) │
                                               │                     │

    ├─ Agent execution max steps exceeded
    │                                          │                     │
    └──────────────────────────────────────►│                     │
                                               │ Stop iteration      │
                                               │ Mark as incomplete  │
                                               │ Save partial results│
                                               │ Return status       │
                                               │                     │

    ├─ Database connection lost
    │                                          │                     │
    └──────────────────────────────────────►│                     │
                                               │ Retry connection    │
                                               │ (with backoff)      │
                                               │ Return error if     │
                                               │ all retries fail    │
                                               │                     │
```

---

## 9. Bundle Creation & Export

```
Environment             BundleService         File System           Output
    │                        │                    │                    │
    │ 1. CreateBundle        │                    │                    │
    │ (envPath)              │                    │                    │
    ├──────────────────────►│                    │                    │
    │                        │ 2. Scan env dir   │                    │
    │                        ├──────────────────►│                    │
    │                        │ Read:              │                    │
    │                        │ • template.json    │                    │
    │                        │ • variables.yml    │                    │
    │                        │ • agents/*.prompt  │                    │
    │                        │◄──────────────────┤                    │
    │                        │                    │                    │
    │                        │ 3. Generate        │                    │
    │                        │ manifest           │                    │
    │                        │ (metadata about    │                    │
    │                        │  bundle)           │                    │
    │                        │                    │                    │
    │                        │ 4. Create tar.gz   │                    │
    │                        │ Package files      │                    │
    │                        │                    │                    │
    │                        │ 5. Return bundle   │                    │
    │                        │ ([]byte)           │                    │
    │◄──────────────────────┤                    │                    │
    │ tar.gz data            │                    │ 6. Write to file          │
    │                        │                    │ (if saving)      ├────────►│
    │                        │                    │                  │ bundle │
    │                        │                    │                  │ .tar.gz│
    │                        │                    │                  │       │
    │ [Later: Installation]  │                    │                  │       │
    │ 7. Unpack tar.gz       │                    │                  │       │
    │                        │                    │◄─────────────────┤       │
    │                        │                    │ Extract files    │       │
    │                        │                    │                  │       │
    │ 8. Create environment  │                    │                  │       │
    │ from bundle            │                    │                  │       │
    │                        │ 9. Setup env dir   │                  │       │
    │                        ├──────────────────►│                  │       │
    │                        │ Copy files         │                  │       │
    │                        │ Create dirs        │                  │       │
    │                        │◄──────────────────┤                  │       │
    │                        │                    │                  │       │
    │ 10. Sync new env       │                    │                  │       │
    │ (DeclarativeSync)      │                    │                  │       │
    │                        │ [Same as diagram #2│                  │       │
    │                        │  (sync)]           │                  │       │
    │                        │                    │                  │       │
    │ 11. Done               │                    │                  │       │
    │ Environment ready      │                    │                  │       │
```

---

## 10. Component Lifecycle Diagram

```
┌──────────────────────────────────────────────────────────────────┐
│                    STATION SYSTEM LIFECYCLE                       │
└──────────────────────────────────────────────────────────────────┘

APPLICATION STARTUP
═══════════════════════════════════════════════════════════════════

  main.go (entry point)
    ↓
  Load configuration
    ├─ Environment variables
    ├─ Config file (~/.station/config.yaml)
    └─ Defaults
    ↓
  Initialize database
    ├─ Open SQLite connection
    ├─ Run migrations
    └─ Create Repositories
    ↓
  Initialize Genkit
    ├─ Load AI provider (OpenAI/Gemini/Ollama)
    ├─ Create genkit.Genkit instance
    └─ Register model
    ↓
  Create Services
    ├─ AgentService
    ├─ DeclarativeSync
    ├─ EnvironmentManagementService
    ├─ SchedulerService
    ├─ TelemetryService
    └─ Others
    ↓
  (If --server flag):
    ├─ Start HTTP API server (:8585)
    └─ Register all v1 routes
  (Else):
    └─ Start MCP server (stdio)
    ↓
  Start scheduler (if enabled)
    ├─ Load scheduled agents from DB
    ├─ Register cron tasks
    └─ Start cron scheduler
    ↓
  Setup signal handlers
    ├─ SIGTERM → graceful shutdown
    └─ SIGINT (Ctrl+C) → graceful shutdown
    ↓
  [READY] - System running, accepting requests


REQUEST PROCESSING LIFECYCLE
═══════════════════════════════════════════════════════════════════

  Agent Execution Request (CLI/API/MCP/Scheduler)
    ↓
  Load agent configuration
    ├─ Query agents table
    └─ Get assigned tools
    ↓
  Initialize execution context
    ├─ Load environment config
    ├─ Load variables.yml
    └─ Resolve template variables
    ↓
  Setup AI provider
    ├─ Initialize GenKit
    └─ Load model
    ↓
  Setup MCP connections
    ├─ Read MCP server configs
    ├─ Start/connect to each server
    └─ Discover tools
    ↓
  Execute agent
    ├─ Render prompt template
    ├─ Pass task to GenKit
    ├─ Iterate tool calls (up to max_steps)
    │  ├─ GenKit selects tool
    │  ├─ Execute tool
    │  ├─ Process output
    │  └─ Feed back to GenKit
    └─ Collect final response
    ↓
  Capture execution metadata
    ├─ Tool calls made
    ├─ Execution steps
    ├─ Token usage
    └─ Timing information
    ↓
  Persist to database
    ├─ Create agent_run record
    ├─ Save execution_steps JSON
    ├─ Save tool_calls JSON
    └─ Update completion status
    ↓
  Optional: Send to Lighthouse
    └─ CloudShip data ingestion
    ↓
  Cleanup
    ├─ Close MCP connections
    ├─ Release resources
    └─ Stop any timers
    ↓
  Return result


GRACEFUL SHUTDOWN
═══════════════════════════════════════════════════════════════════

  Receive SIGTERM/SIGINT
    ↓
  Stop accepting new requests
    ↓
  Wait for in-flight requests (5s timeout)
    ├─ Allow running agents to complete
    ├─ Save any pending metadata
    └─ Timeout → force close
    ↓
  Shutdown services
    ├─ Stop scheduler (500ms timeout)
    ├─ Close database connections
    ├─ Close MCP connections
    └─ Release telemetry
    ↓
  Shutdown API server (1s timeout)
    ├─ Close listening socket
    ├─ Graceful HTTP shutdown
    └─ Force close if timeout
    ↓
  Exit process (code 0 for success)


ENVIRONMENT LIFECYCLE
═══════════════════════════════════════════════════════════════════

  CREATE environment
    ├─ Create database entry
    ├─ Create directory structure
    │  ├─ ~/.config/station/environments/<name>/
    │  ├─ agents/ subdirectory
    │  └─ variables.yml file
    └─ Ready for configuration
    ↓
  CONFIGURE environment
    ├─ User creates template.json (MCP servers)
    ├─ User creates agents/*.prompt files
    └─ User edits variables.yml
    ↓
  SYNC environment
    ├─ DeclarativeSync reads files
    ├─ Registers MCP servers in DB
    ├─ Creates agents in DB
    ├─ Discovers tools
    └─ Agents ready to execute
    ↓
  USE environment
    ├─ Execute agents
    ├─ Query run history
    └─ Modify configurations
    ↓
  DELETE environment
    ├─ Remove database entries
    │  └─ Cascade deletes agents, runs, tools
    └─ Remove directory structure
       └─ All files, agents, configs deleted
```

---

## Summary

These detailed interaction diagrams show:

1. **Sequential execution flows** - Step-by-step interactions
2. **Asynchronous operations** - Parallel processing where applicable
3. **Error handling paths** - Recovery mechanisms
4. **Data transformation** - How data flows through the system
5. **Lifecycle management** - Component initialization and cleanup

Key characteristics:
- **Clear responsibilities** - Each component has defined role
- **Well-defined boundaries** - Components interact through interfaces
- **Error resilience** - Graceful degradation on failures
- **Resource management** - Proper cleanup and initialization
- **Observability** - Metadata capture at each step

