# Station Development Guide

**Practical guide for working with Station codebase - based on actual code analysis**

## 🚀 Getting Started

### **"We're working on agent handlers next"**
**Read these first:**
1. `components/handlers/README.md` - Handler patterns and structure
2. `conventions/rules.md` - Development rules and patterns
3. `components/handlers/agent.md` - Specific agent handler details

**Key files to understand:**
- `cmd/main/handlers/agent/handlers.go` - Main agent handler coordinator
- `internal/db/repositories/agents.go` - Agent data operations
- `internal/db/queries/agents.sql` - Agent SQL queries (sqlc)

### **"We need to understand the MCP layer"**
**Read these first:**
1. `components/mcp-server/README.md` - Complete MCP server overview
2. `architecture/overview.md` - How MCP fits in overall system

**Key files to examine:**
- `internal/mcp/tools_setup.go` - Tool definitions and registration
- `internal/mcp/handlers_fixed.go` - Tool implementation handlers
- `internal/mcp/server.go` - MCP server initialization

### **"Database schema changes needed"**
**Read these first:**
1. `data/database.md` - Database architecture and sqlc patterns
2. `conventions/rules.md` - Database rules and conventions

**Development workflow:**
1. **Migration**: Add new `.sql` file in `internal/db/migrations/`
2. **Queries**: Update queries in `internal/db/queries/{table}.sql`
3. **Generate**: Run `sqlc generate` to update Go code
4. **Repository**: Update repository in `internal/db/repositories/`

## 🛠️ Common Development Tasks

### **Adding a New MCP Tool**

**1. Define the Tool** (`internal/mcp/tools_setup.go`):
```go
func (s *Server) setupTools() {
    // Add your tool definition
    myTool := mcp.NewTool("my_new_tool",
        mcp.WithDescription("Description of what the tool does"),
        mcp.WithString("param1", mcp.Required(), mcp.Description("Required parameter")),
        mcp.WithNumber("param2", mcp.Description("Optional parameter with default")),
    )
    s.mcpServer.AddTool(myTool, s.handleMyNewTool)
}
```

**2. Implement Handler** (`internal/mcp/handlers_fixed.go`):
```go
func (s *Server) handleMyNewTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // 1. Extract and validate parameters
    param1, err := request.RequireString("param1")
    if err != nil {
        return mcp.NewToolResultError("Missing param1"), nil
    }
    
    param2 := request.GetInt("param2", 10) // Default value
    
    // 2. Perform business logic via services/repositories
    result, err := s.repos.SomeRepo.DoOperation(param1, param2)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Operation failed: %v", err)), nil
    }
    
    // 3. Return structured response
    response := map[string]interface{}{
        "success": true,
        "data": result,
        "message": "Operation completed successfully",
    }
    
    resultJSON, _ := json.MarshalIndent(response, "", "  ")
    return mcp.NewToolResultText(string(resultJSON)), nil
}
```

### **Adding a New Database Table**

**1. Create Migration** (`internal/db/migrations/017_new_table.sql`):
```sql
-- +goose Up
CREATE TABLE my_new_table (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_my_new_table_name ON my_new_table(name);

-- +goose Down
DROP INDEX idx_my_new_table_name;
DROP TABLE my_new_table;
```

**2. Add Queries** (`internal/db/queries/my_new_table.sql`):
```sql
-- name: CreateMyNewTable :one
INSERT INTO my_new_table (name, description)
VALUES (?, ?)
RETURNING *;

-- name: GetMyNewTable :one
SELECT * FROM my_new_table WHERE id = ?;

-- name: ListMyNewTables :many
SELECT * FROM my_new_table ORDER BY name;

-- name: UpdateMyNewTable :exec
UPDATE my_new_table SET name = ?, description = ? WHERE id = ?;

-- name: DeleteMyNewTable :exec
DELETE FROM my_new_table WHERE id = ?;
```

**3. Generate Code**:
```bash
sqlc generate
```

**4. Create Repository** (`internal/db/repositories/my_new_table.go`):
```go
package repositories

import (
    "context"
    "database/sql"
    "station/internal/db/queries"
    "station/pkg/models"
)

type MyNewTableRepo struct {
    db      *sql.DB
    queries *queries.Queries
}

func NewMyNewTableRepo(db *sql.DB) *MyNewTableRepo {
    return &MyNewTableRepo{
        db:      db,
        queries: queries.New(db),
    }
}

func (r *MyNewTableRepo) Create(name, description string) (*models.MyNewTable, error) {
    result, err := r.queries.CreateMyNewTable(context.Background(), queries.CreateMyNewTableParams{
        Name:        name,
        Description: sql.NullString{String: description, Valid: description != ""},
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to create: %w", err)
    }
    
    return convertFromSQLc(result), nil
}

// Add other CRUD methods...
```

### **Adding a New CLI Command**

**1. Define Command** (`cmd/main/commands.go`):
```go
var myNewCmd = &cobra.Command{
    Use:   "mynew <subcommand>",
    Short: "Manage my new feature",
    Long:  "Detailed description of the new feature",
}

var myNewCreateCmd = &cobra.Command{
    Use:   "create <name>",
    Short: "Create new item",
    Args:  cobra.ExactArgs(1),
    RunE:  runMyNewCreate,
}
```

**2. Register Command** (`cmd/main/main.go`):
```go
func init() {
    // Add to root command
    rootCmd.AddCommand(myNewCmd)
    
    // Add subcommands
    myNewCmd.AddCommand(myNewCreateCmd)
    
    // Add flags
    myNewCreateCmd.Flags().String("description", "", "Description of the item")
}
```

**3. Implement Handler** (`cmd/main/mynew.go`):
```go
func runMyNewCreate(cmd *cobra.Command, args []string) error {
    name := args[0]
    description, _ := cmd.Flags().GetString("description")
    
    // Load configuration
    cfg, err := config.Load()
    if err != nil {
        return fmt.Errorf("failed to load config: %w", err)
    }
    
    // Connect to database
    database, err := db.New(cfg.DatabaseURL)
    if err != nil {
        return fmt.Errorf("database connection failed: %w", err)
    }
    defer database.Close()
    
    // Create repository
    repo := repositories.NewMyNewTableRepo(database)
    
    // Perform operation
    item, err := repo.Create(name, description)
    if err != nil {
        return fmt.Errorf("creation failed: %w", err)
    }
    
    // Display result
    fmt.Printf("✅ Created item: %s (ID: %d)\n", item.Name, item.ID)
    return nil
}
```

## 🔧 Development Workflow

### **Daily Development Process**
1. **Pull latest changes**: `git pull origin main`
2. **Run tests**: `make test` (if available)
3. **Make changes**: Follow existing patterns
4. **Generate code**: `sqlc generate` (if database changes)
5. **Test locally**: Build and test your changes
6. **Commit**: Use conventional commit messages

### **Code Analysis Workflow**
```bash
# Understand component structure
tree cmd/main/handlers/agent/

# Find related files
find . -name "*agent*" -type f

# Examine patterns in existing code
grep -r "func.*Agent" cmd/main/handlers/

# Check database schema
cat internal/db/schema.sql | grep -A 10 -B 2 "agents"

# Understand sqlc queries
cat internal/db/queries/agents.sql
```

### **Testing Changes**
```bash
# Build the binary
make build

# Test version (should work after recent fixes)
./bin/stn --version

# Test your new functionality
./bin/stn mynew create "test-item" --description "Test description"

# Check database state (if needed)
sqlite3 station.db "SELECT * FROM my_new_table;"
```

## 🐛 Debugging Common Issues

### **"sqlc generate fails"**
- **Check**: `sqlc.yaml` configuration is correct
- **Verify**: SQL syntax in `.sql` files is valid
- **Ensure**: Schema is up to date with migrations

### **"Handler not found"**
- **Check**: Handler is registered in `cmd/main/main.go`
- **Verify**: Function name matches command definition
- **Ensure**: Import statements include handler package

### **"Database operation fails"**
- **Check**: Migration has been run
- **Verify**: sqlc code is generated after schema changes
- **Ensure**: Repository is using correct queries package

### **"MCP tool not working"**
- **Check**: Tool is registered in `tools_setup.go`
- **Verify**: Handler function exists and is wired correctly
- **Ensure**: Parameters match tool definition
- **Test**: Use `./stn stdio` and test with MCP client

## 📋 Code Review Checklist

### **Before Submitting Changes**
- [ ] **Follow existing patterns**: Code style matches surrounding code
- [ ] **No hardcoded values**: Use configuration where appropriate
- [ ] **Error handling**: Proper error handling with context
- [ ] **Input validation**: Validate all user inputs
- [ ] **Documentation**: Update relevant docs if behavior changes
- [ ] **Tests pass**: Ensure tests still pass (if test suite exists)

### **Database Changes**
- [ ] **Migration included**: New migration file for schema changes
- [ ] **Queries updated**: Relevant `.sql` query files updated
- [ ] **Code generated**: `sqlc generate` run after changes
- [ ] **Repository updated**: Repository methods match new schema
- [ ] **Indexes considered**: Add indexes for performance if needed

### **MCP Tool Changes**
- [ ] **Tool definition**: Proper parameter definitions with descriptions
- [ ] **Handler implementation**: Complete error handling and validation
- [ ] **Response format**: Consistent JSON response structure
- [ ] **Documentation**: Tool purpose and usage documented

## 🎯 Team Conventions

### **Communication Patterns**
- **"Working on X component"**: Read the component docs first
- **"Found an issue"**: Check existing patterns before proposing changes
- **"Adding new feature"**: Analyze similar existing features first
- **"Database changes needed"**: Follow migration → queries → generate → repository pattern

### **Code Standards**
- **Consistency**: Match existing code style and patterns
- **Clarity**: Use descriptive names and clear structure
- **Efficiency**: Consider performance implications
- **Security**: Validate inputs and handle secrets properly

---
*This guide reflects the actual codebase structure and patterns as of Station v0.1.0. Update as the codebase evolves.*