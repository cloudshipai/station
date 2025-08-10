# Handler Components Overview

Station uses a **handler pattern** to process requests from different interfaces (CLI, SSH, API, MCP). Each handler type has specific responsibilities and follows consistent patterns.

## 📁 Handler Directory Structure

```
cmd/main/handlers/
├── agent/                 # Agent management handlers
│   ├── handlers.go       # Main handler coordinator
│   ├── local.go         # Local environment operations  
│   └── remote.go        # Remote environment operations
├── file_config/          # File-based configuration handlers
│   ├── handlers.go      # Config operation coordinator
│   ├── create.go        # Config creation
│   ├── delete.go        # Config deletion
│   ├── list.go          # Config listing
│   ├── update.go        # Config updates
│   └── env_*.go         # Environment-specific operations
├── load/                 # MCP configuration loading handlers
│   ├── handlers.go      # Load operation coordinator  
│   ├── local.go         # Local MCP config loading
│   └── remote.go        # Remote MCP config loading
├── mcp/                  # MCP server management handlers
│   ├── handlers.go      # MCP operation coordinator
│   └── utils.go         # MCP utility functions
├── webhooks/             # Webhook management handlers
│   ├── handlers.go      # Webhook coordinator
│   └── test.go          # Webhook testing functionality
└── common/               # Shared handler utilities
    └── common.go        # Common handler functions
```

## 🎯 Handler Types and Responsibilities

### **Agent Handlers** (`agent/`)
**Purpose**: Manage AI agent lifecycle operations + Agent Template System

```
Agent + Template Operations Flow:
┌─────────────┐    ┌─────────────┐    ┌─────────────────┐    ┌─────────────┐
│ CLI Command │───▶│   Handler   │───▶│  Agent/Bundle   │───▶│   Service   │
│ stn agent   │    │             │    │     Manager     │    │             │
│ create/run/ │    │ - Validate  │    │                 │    │ - Business  │
│ bundle/*    │    │ - Route     │    │ - Templates     │    │   Logic     │
└─────────────┘    └─────────────┘    │ - Variables     │    │ - Database  │
                                     │ - Dependencies  │    │ - Templates │
                                     └─────────────────┘    └─────────────┘
```

**Key Files**:
- `handlers.go`: Command routing, validation, and Agent Bundle System integration
- `local.go`: Local environment agent operations + template management
- `remote.go`: Remote environment agent operations + API integration

**Agent Bundle Commands**:
- `RunAgentBundleCreate`: Create new agent templates with scaffolding
- `RunAgentBundleValidate`: Comprehensive template validation  
- `RunAgentBundleInstall`: Install templates with variable resolution
- `RunAgentBundleExport`: Convert existing agents to templates
- `RunAgentBundleDuplicate`: Cross-environment agent deployment

**Pattern**: Local/Remote duality + Agent Template System integration

### **File Config Handlers** (`file_config/`)
**Purpose**: Manage file-based MCP configurations

```
Config Management Flow:
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ File System │───▶│  Handlers   │───▶│ File Config │
│             │    │             │    │  Service    │
│ - Templates │    │ - Validate  │    │             │
│ - Variables │    │ - Process   │    │ - Template  │
│ - Configs   │    │ - Transform │    │   Engine    │
└─────────────┘    └─────────────┘    └─────────────┘
```

**Key Operations**:
- `create.go`: New configuration creation with templates
- `list.go`: Configuration discovery and listing
- `update.go`: Configuration modification and versioning
- `env_*.go`: Environment-specific configuration management

### **Load Handlers** (`load/`)
**Purpose**: Load and process MCP server configurations

```
MCP Loading Flow:
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Source    │───▶│  Handlers   │───▶│ MCP Service │
│             │    │             │    │             │
│ - JSON/YAML │    │ - Parse     │    │ - Server    │
│ - Templates │    │ - Resolve   │    │   Start     │
│ - Variables │    │ - Validate  │    │ - Tool      │
└─────────────┘    └─────────────┘    │   Discovery │
                                      └─────────────┘
```

**Special Features**:
- Interactive template variable resolution
- AI-powered placeholder detection  
- Environment-specific variable management

### **MCP Handlers** (`mcp/`)
**Purpose**: Handle MCP server lifecycle and tool management

**Operations**:
- Server startup and shutdown
- Tool discovery and registration
- Connection health monitoring
- Configuration validation

### **Webhook Handlers** (`webhooks/`)
**Purpose**: Manage webhook notifications and testing

**Features**:
- Webhook endpoint management
- Test payload generation and sending
- Delivery tracking and retry logic

## 🔄 Common Handler Patterns

### **1. Local/Remote Pattern**
Most handlers implement both local and remote variants:

```go
// Local operation - direct database/file access
func (h *AgentHandler) agentCreateLocal(cmd *cobra.Command, args []string) error {
    // Direct local operations
    database, err := db.New(cfg.DatabaseURL)
    // ... perform local operations
}

// Remote operation - API call to remote Station instance
func (h *AgentHandler) agentCreateRemote(cmd *cobra.Command, args []string) error {
    // Remote API calls
    client := api.NewClient(remoteURL)
    // ... perform remote operations
}
```

**Why**: Allows Station to manage both local agents and remote Station instances

### **2. Configuration Loading Pattern**
All handlers follow consistent config loading:

```go
func loadStationConfig() (*config.Config, error) {
    // 1. Load from environment variables
    // 2. Override with config file values
    // 3. Validate required fields
    // 4. Return validated config
}
```

### **3. Error Handling Pattern**
Consistent error handling and user feedback:

```go
func (h *Handler) operation(cmd *cobra.Command, args []string) error {
    // Validate inputs
    if err := validateInputs(args); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // Perform operation
    result, err := h.service.DoOperation(params)
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    
    // Format and display results
    h.displayResults(result)
    return nil
}
```

### **4. Service Injection Pattern**
Handlers receive services through dependency injection:

```go
type AgentHandler struct {
    agentService    services.AgentService
    configService   services.ConfigService
    themeManager    *theme.ThemeManager
}

func NewAgentHandler(services ...) *AgentHandler {
    return &AgentHandler{
        agentService:  services.Agent,
        configService: services.Config,
        // ... other dependencies
    }
}
```

## 🎨 Handler Conventions

### **File Naming**
- `handlers.go`: Main coordinator and command registration
- `local.go`: Local environment operations
- `remote.go`: Remote environment operations  
- `{operation}.go`: Specific operation implementations

### **Function Naming**
- `run{Component}{Operation}`: Main command handlers
- `{component}{Operation}Local`: Local variant implementations
- `{component}{Operation}Remote`: Remote variant implementations

### **Error Messages**
- User-friendly error messages with context
- Technical details in debug logs only
- Consistent formatting across handlers

## 🔧 Development Guidelines

### **Adding New Handlers**
1. Create handler struct with required dependencies
2. Implement both local and remote variants (if applicable)
3. Add proper input validation
4. Follow error handling patterns
5. Add to command registration in main.go

### **Modifying Existing Handlers**
1. Maintain backward compatibility
2. Update both local and remote variants
3. Add appropriate tests
4. Update command help text if needed

### **Handler Testing**
- Unit tests for handler logic
- Integration tests for service interactions
- Mock external dependencies
- Test both success and error paths

---
*Next: Read specific handler documentation for implementation details*