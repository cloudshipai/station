# Station Development Conventions & Rules

**Engineering rules and patterns based on actual codebase analysis**

## 🗄️ Database Conventions

### **RULE: Always use sqlc for database operations**
- **No hand-written SQL in Go code** - All queries must be in `.sql` files
- **Location**: `internal/db/queries/*.sql`
- **Package**: Generated code goes to `internal/db/queries` package
- **Command**: `sqlc generate` after schema/query changes

**Example Query File** (`internal/db/queries/agents.sql`):
```sql
-- name: CreateAgent :one
INSERT INTO agents (name, description, prompt, max_steps, environment_id, created_by, cron_schedule, is_scheduled, schedule_enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAgent :one
SELECT * FROM agents WHERE id = ?;

-- name: ListAgentsByEnvironment :many
SELECT * FROM agents WHERE environment_id = ? ORDER BY name;
```

**Generated Usage**:
```go
// Use generated queries, never raw SQL
agent, err := repo.queries.CreateAgent(ctx, queries.CreateAgentParams{
    Name:            name,
    Description:     description,
    // ... other params
})
```

### **RULE: Repository Pattern Implementation**
**Structure**: `internal/db/repositories/{entity}.go`
```go
type AgentRepo struct {
    db      *sql.DB
    queries *queries.Queries  // Always use sqlc queries
}

func NewAgentRepo(db *sql.DB) *AgentRepo {
    return &AgentRepo{
        db:      db,
        queries: queries.New(db),
    }
}
```

### **RULE: SQLite is the database**
- **Single file database**: `station.db` in project root
- **No connection pooling needed** - SQLite handles single connections
- **Migrations**: Use goose with embedded migrations
- **Location**: `internal/db/migrations/*.sql`

## 📁 File-Based Configuration Rules

### **RULE: File-based configs, not database configs**
- **Templates**: JSON files with `{{.VARIABLE}}` placeholders
- **Variables**: YAML files with environment-specific values  
- **Location**: `~/.config/station/config/environments/{env}/`
- **No database storage** - Files are source of truth

**Directory Structure**:
```
~/.config/station/config/environments/
├── default/
│   ├── filesystem.json      # MCP template
│   └── template-vars/
│       └── filesystem.yml   # Variables for template
├── production/
└── development/
```

### **RULE: Template Variable Resolution**
- **Go templates**: Use `{{.VAR_NAME}}` syntax
- **Environment isolation**: Variables never shared between environments
- **Security**: Sensitive variables encrypted at rest
- **Detection**: Auto-detect variables in templates

**Example Template** (`filesystem.json`):
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "{{.ALLOWED_PATHS}}"]
    }
  }
}
```

**Variables File** (`template-vars/filesystem.yml`):
```yaml
ALLOWED_PATHS: "/home/user/projects"
```

## 🎭 Handler Pattern Rules

### **RULE: Handler Organization**
- **By domain**: `cmd/main/handlers/{domain}/` 
- **Main coordinator**: `handlers.go` in each domain
- **Local/Remote split**: When applicable, separate local and remote operations

**Current Structure** (Verified):
```
cmd/main/handlers/
├── agent/handlers.go          # Agent operations coordinator
├── file_config/handlers.go    # File config operations  
├── load/handlers.go           # MCP loading operations
├── mcp/handlers.go            # MCP server management
├── webhooks/handlers.go       # Webhook operations
└── common/common.go           # Shared utilities
```

### **RULE: Handler Constructor Pattern**
```go
type AgentHandler struct {
    themeManager *theme.ThemeManager
}

func NewAgentHandler(themeManager *theme.ThemeManager) *AgentHandler {
    return &AgentHandler{themeManager: themeManager}
}
```

### **RULE: Command Handler Naming**
- **Command handlers**: `Run{Entity}{Action}` (e.g., `RunAgentList`)
- **Helper functions**: `{entity}{Action}Local` / `{entity}{Action}Remote`
- **Validation**: Consistent input validation patterns

## 🧩 Service Layer Rules

### **RULE: Service Dependency Injection**
```go
type FileConfigService struct {
    configManager   config.ConfigManager
    toolDiscovery   *ToolDiscoveryService
    repos          *repositories.Repositories
}

func NewFileConfigService(
    configManager config.ConfigManager,
    toolDiscovery *ToolDiscoveryService,
    repos *repositories.Repositories,
) *FileConfigService {
    return &FileConfigService{
        configManager: configManager,
        toolDiscovery: toolDiscovery,
        repos:        repos,
    }
}
```

### **RULE: Service Interface Definitions**
- **Location**: `internal/services/` 
- **Pattern**: Define interfaces for testability
- **Implementation**: Separate implementation files

## 🎨 Code Style Rules

### **RULE: No Comments Unless Requested**
- **Self-documenting code**: Use clear variable and function names
- **Only add comments** when explicitly asked by user
- **Package comments**: Required for public packages only

### **RULE: Error Handling Pattern**
```go
func (h *Handler) operation(params) error {
    // Validate inputs first
    if err := validateInputs(params); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    // Perform operation
    result, err := h.service.DoOperation(params)
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    
    // Success path
    return nil
}
```

### **RULE: Import Organization**
```go
import (
    // Standard library
    "context"
    "fmt"
    
    // Third party
    "github.com/spf13/cobra"
    "gopkg.in/yaml.v3"
    
    // Internal packages
    "station/internal/config"
    "station/pkg/models"
)
```

## 🚀 MCP Integration Rules

### **RULE: MCP Server Management**
- **Location**: `internal/mcp/`
- **Tools**: Define in `tools_setup.go`
- **Handlers**: Implement in `handlers_fixed.go`
- **Pattern**: Use mark3labs/mcp-go library

### **RULE: Tool Registration**
```go
// Always use this pattern for MCP tools
tool := mcp.NewTool("tool_name",
    mcp.WithDescription("Tool description"),
    mcp.WithString("param_name", mcp.Required(), mcp.Description("Parameter description")),
)
server.AddTool(tool, handlerFunction)
```

## 🔧 Configuration Loading Rules

### **RULE: Configuration Priority Order**
1. **Command line flags** (highest priority)
2. **Environment variables** 
3. **Config file values**
4. **Default values** (lowest priority)

### **RULE: Configuration File Locations**
- **Main config**: `~/.config/station/config.yaml`
- **MCP configs**: `~/.config/station/config/environments/{env}/`
- **Variables**: `template-vars/` subdirectories

## 🧪 Testing Rules

### **RULE: Test File Naming**
- **Unit tests**: `{filename}_test.go` 
- **Integration tests**: `{filename}_integration_test.go`
- **Test files**: Same package as code being tested

### **RULE: Test Organization**
```go
func TestFunctionName(t *testing.T) {
    // Arrange
    setup := createTestSetup()
    
    // Act  
    result, err := functionUnderTest(params)
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

## 🔒 Security Rules

### **RULE: Secret Management**
- **Never commit secrets** to version control
- **Encryption**: Use Station's crypto package for sensitive data
- **File permissions**: 600 for variable files, 644 for templates
- **Environment isolation**: Secrets never shared between environments

### **RULE: Input Validation**
- **Always validate** user inputs at handler level
- **Sanitize paths** before file operations
- **Validate JSON/YAML** before processing
- **Check permissions** before file access

## 📦 Dependency Management Rules

### **RULE: Go Modules**
- **go.mod**: Keep dependencies minimal and up to date
- **Vendor**: Don't commit vendor directory
- **Version pinning**: Pin versions for reproducible builds

### **RULE: Internal Package Structure**
```
internal/              # Private application code
├── api/              # HTTP API layer
├── config/           # Configuration management
├── db/               # Database operations
├── mcp/              # MCP server implementation
├── services/         # Business logic
└── telemetry/        # Usage analytics

pkg/                  # Public packages (can be imported)
├── config/           # Configuration types
├── models/           # Domain models
└── crypto/           # Cryptographic utilities
```

## 🎯 Development Workflow Rules

### **RULE: Making Changes**
1. **Read existing code** to understand patterns
2. **Check dependencies** - never assume libraries exist
3. **Follow existing conventions** in the codebase
4. **Test locally** before committing
5. **Update documentation** if behavior changes

### **RULE: Adding New Features**
1. **Analyze existing similar features** first
2. **Use existing libraries** and utilities
3. **Follow established patterns** (handlers, services, repos)
4. **Add appropriate tests**
5. **Update relevant documentation**

---
*These rules are derived from actual codebase analysis and ensure consistency across Station development.*