# Station System Layers

**Detailed mapping of Station's layered architecture with actual code references**

## 🏗️ Layer Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    PRESENTATION LAYER                           │
│                                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │     CLI     │ │     SSH     │ │  REST API   │ │ MCP Server  ││
│  │ cmd/main/   │ │internal/ssh/│ │internal/api/│ │internal/mcp/││
│  │  :8080      │ │   :2222     │ │   :8080     │ │ :3000/stdio ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
            │             │             │            │
┌─────────────────────────────────────────────────────────────────┐
│                    HANDLER LAYER                                │
│                                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Agent     │ │ File Config │ │     Load    │ │  Webhooks   ││
│  │ Handlers    │ │  Handlers   │ │  Handlers   │ │  Handlers   ││
│  │ :43 files   │ │ :17 files   │ │  :6 files   │ │  :8 files   ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
            │             │             │            │
┌─────────────────────────────────────────────────────────────────┐
│                     SERVICE LAYER                               │
│                                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │    Agent    │ │ File Config │ │     Tool    │ │   Webhook   ││
│  │   Service   │ │   Service   │ │ Discovery   │ │   Service   ││
│  │             │ │             │ │   Service   │ │             ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
            │             │             │            │
┌─────────────────────────────────────────────────────────────────┐
│                  REPOSITORY LAYER                               │
│                                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Agents    │ │ Agent Runs  │ │ MCP Tools   │ │ Environments││
│  │    Repo     │ │    Repo     │ │    Repo     │ │    Repo     ││
│  │             │ │             │ │             │ │             ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
            │             │             │            │
┌─────────────────────────────────────────────────────────────────┐
│                      DATA LAYER                                 │
│                                                                 │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   SQLite    │ │    sqlc     │ │ File System │ │  Templates  ││
│  │ station.db  │ │  Generated  │ │   Configs   │ │ & Variables ││
│  │             │ │   Queries   │ │             │ │             ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## 🎯 Layer Responsibilities & Code Locations

### **1. Presentation Layer**
**Responsibility**: Handle user interactions and external integrations

#### **CLI Interface** (`cmd/main/`)
```
Entry Points:
├── main.go              # Root command and initialization
├── commands.go          # Command definitions  
├── agent.go             # Agent command setup
├── load.go              # Load command setup
├── server.go            # Server command setup
└── stdio.go             # MCP stdio mode
```

**Key Pattern**: Cobra commands with handler delegation
```go
var agentCmd = &cobra.Command{
    Use:   "agent",
    Short: "Manage agents",
    RunE:  handlers.NewAgentHandler(themeManager).RunAgentList,
}
```

#### **SSH Server** (`internal/ssh/`)
```
SSH Components:
├── ssh.go               # SSH server implementation
└── apps/                # TUI applications
    ├── agents.go        # Agent management TUI
    ├── configs.go       # Config management TUI
    └── dashboard.go     # Main dashboard TUI
```

#### **REST API** (`internal/api/`)
```
API Structure:
├── api.go               # API server setup
└── v1/                  # API version 1
    ├── agents.go        # Agent CRUD endpoints
    ├── runs.go          # Agent run endpoints
    └── environments.go  # Environment endpoints
```

#### **MCP Server** (`internal/mcp/`)
```
MCP Components:
├── server.go            # MCP server initialization
├── tools_setup.go       # Tool definitions (11+ tools)
├── handlers_fixed.go    # Tool implementations
├── resources_setup.go   # Resource definitions
└── resources_handlers.go # Resource implementations
```

### **2. Handler Layer**
**Responsibility**: Process requests, validate input, coordinate business logic

#### **Handler Organization** (`cmd/main/handlers/`)
```
Handler Modules:
├── agent/               # Agent lifecycle management
│   ├── handlers.go      # Main coordinator
│   ├── local.go         # Local operations (12 functions)
│   └── remote.go        # Remote operations (12 functions)
├── file_config/         # File-based configuration
│   ├── handlers.go      # Config coordinator
│   ├── create.go        # Config creation
│   ├── list.go          # Config listing
│   ├── update.go        # Config updates
│   └── env_*.go         # Environment operations
├── load/                # MCP server loading
│   ├── handlers.go      # Load coordinator
│   ├── local.go         # Local MCP loading
│   └── remote.go        # Remote MCP loading
├── mcp/                 # MCP management
├── webhooks/            # Webhook management
└── common/              # Shared utilities
```

**Handler Pattern**:
```go
type AgentHandler struct {
    themeManager *theme.ThemeManager
}

func (h *AgentHandler) RunAgentCreate(cmd *cobra.Command, args []string) error {
    // 1. Validate input
    // 2. Load configuration  
    // 3. Route to local/remote
    // 4. Format response
}
```

### **3. Service Layer**
**Responsibility**: Business logic, orchestration, cross-cutting concerns

#### **Core Services** (`internal/services/`)
```
Service Components:
├── agent_service_impl.go          # Agent lifecycle business logic
├── agent_service_interface.go     # Agent service contract
├── file_config_service.go         # File configuration management
├── tool_discovery_service.go      # MCP tool discovery
├── webhook_service.go             # Webhook notifications
├── intelligent_agent_creator.go   # AI-powered agent creation
├── intelligent_placeholder_analyzer.go # Template variable detection
└── execution_queue.go             # Agent execution queue
```

**Service Pattern**:
```go
type FileConfigService struct {
    configManager   config.ConfigManager
    toolDiscovery   *ToolDiscoveryService
    repos          *repositories.Repositories
}

func (s *FileConfigService) CreateTemplate(ctx context.Context, envID int64, template *config.MCPTemplate) error {
    // Business logic implementation
}
```

### **4. Repository Layer**
**Responsibility**: Data access, CRUD operations, query execution

#### **Repository Structure** (`internal/db/repositories/`)
```
Repository Files:
├── repositories.go      # Repository coordinator
├── agents.go           # Agent data operations
├── agent_runs.go       # Agent execution records
├── agent_tools.go      # Agent-tool associations
├── mcp_configs.go      # MCP configuration storage
├── mcp_tools.go        # MCP tool definitions
├── environments.go     # Environment management
└── webhooks.go         # Webhook configuration
```

**Repository Pattern**:
```go
type AgentRepo struct {
    db      *sql.DB
    queries *queries.Queries  // sqlc generated
}

func (r *AgentRepo) Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, cronSchedule *string, scheduleEnabled bool) (*models.Agent, error) {
    // Use sqlc generated queries only
    result, err := r.queries.CreateAgent(context.Background(), queries.CreateAgentParams{...})
    return convertFromSQLc(result), err
}
```

### **5. Data Layer**
**Responsibility**: Data persistence, query execution, schema management

#### **Database Components** (`internal/db/`)
```
Database Structure:
├── db.go                # Database connection
├── migrate.go           # Migration runner
├── schema.sql           # Complete schema
├── interface.go         # Database interfaces
├── migrations/          # Schema migrations
│   ├── 001_initial_schema.sql
│   ├── 002_add_environments.sql
│   └── ...016_add_file_config_support.sql
└── queries/             # sqlc queries and generated code
    ├── agents.sql       # Agent queries
    ├── agents.sql.go    # Generated Go code
    ├── mcp_tools.sql    # MCP tool queries
    └── *.sql.go         # All generated code
```

**sqlc Configuration** (`sqlc.yaml`):
```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "internal/db/queries"
    schema: "internal/db/schema.sql"
    gen:
      go:
        package: "queries"
        out: "internal/db/queries"
        emit_json_tags: true
```

## 🔄 Data Flow Through Layers

### **Example: Agent Creation Flow**
```
1. CLI Command        │ User: stn agent create --name "Test"
   cmd/main/agent.go  │ 
                      │
2. Handler Layer      │ cmd/main/handlers/agent/handlers.go
                      │ ├─ Validate parameters
                      │ ├─ Load Station configuration
                      │ └─ Route to local handler
                      │
3. Service Layer      │ internal/services/agent_service_impl.go
                      │ ├─ Apply business rules
                      │ ├─ Set defaults (max_steps, environment)
                      │ └─ Call repository
                      │
4. Repository Layer   │ internal/db/repositories/agents.go
                      │ ├─ Use sqlc generated queries
                      │ ├─ Execute database transaction
                      │ └─ Return domain model
                      │
5. Data Layer         │ internal/db/queries/agents.sql.go
                      │ ├─ INSERT INTO agents (...)
                      │ ├─ SQLite execution
                      │ └─ Return result row
```

### **Cross-Layer Principles**

#### **Dependency Direction**
- **Downward Only**: Each layer only depends on layers below it
- **Interface Boundaries**: Services depend on repository interfaces, not implementations
- **No Skip**: Handlers don't directly access repositories (go through services)

#### **Error Propagation**
```go
// Error flows up through layers with context
func (h *Handler) CreateAgent(...) error {
    agent, err := h.service.CreateAgent(...)
    if err != nil {
        return fmt.Errorf("handler: agent creation failed: %w", err)
    }
    return nil
}

func (s *Service) CreateAgent(...) error {
    agent, err := s.repo.Create(...)
    if err != nil {
        return fmt.Errorf("service: repository operation failed: %w", err)
    }
    return nil
}
```

#### **Data Transformation**
- **Domain Models**: `pkg/models/` for business entities
- **Database Models**: `internal/db/queries/` for sqlc generated types
- **API Models**: HTTP request/response structures
- **Conversion**: Each layer converts between its preferred formats

## 🎯 Development Implications

### **Adding New Features**
1. **Start with Data**: Design schema, add migration
2. **Repository**: Create queries and repository methods
3. **Service**: Implement business logic
4. **Handler**: Add request processing
5. **Presentation**: Wire up to CLI/API/MCP/SSH

### **Modifying Existing Features**
1. **Identify Layer**: Determine which layer needs changes
2. **Check Dependencies**: Understand what depends on current behavior
3. **Follow Patterns**: Use existing patterns in the same layer
4. **Test Boundaries**: Ensure layer contracts are maintained

### **Debugging Issues**
1. **Trace Flow**: Follow request from presentation → data
2. **Layer Isolation**: Test each layer independently
3. **Check Boundaries**: Verify data conversion between layers
4. **Log Context**: Add logging with layer context

---
*This layer mapping reflects the actual Station codebase architecture as verified through code analysis.*