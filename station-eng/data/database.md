# Database Architecture & Patterns

Station uses **SQLite** with **sqlc** for type-safe, high-performance database operations. This document covers the database architecture, patterns, and conventions.

## 🗄️ Database Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        DATABASE LAYER                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │   SQLite    │  │    sqlc     │  │ Migrations  │            │
│  │    File     │  │ Generated   │  │   System    │            │
│  │             │  │   Queries   │  │             │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                │                │                    │
│         └────────────────┼────────────────┘                    │
│                          │                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               REPOSITORY LAYER                          │   │
│  │                                                         │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐   │   │
│  │  │ Agents  │  │  Runs   │  │  MCP    │  │ Users   │   │   │
│  │  │  Repo   │  │  Repo   │  │  Repo   │  │  Repo   │   │   │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 🎯 Why SQLite + sqlc?

### **SQLite Benefits**
- **Embedded**: No separate server process required
- **Zero Config**: Single file database, no setup
- **ACID Compliant**: Full transaction support
- **High Performance**: Excellent for read-heavy workloads
- **Backup Friendly**: Single file = easy backups
- **Cross Platform**: Works everywhere Go works

### **sqlc Benefits**  
- **Type Safety**: Generated Go structs from SQL schema
- **Performance**: No reflection, direct SQL execution
- **SQL First**: Write SQL, get Go code
- **Migration Friendly**: Schema changes = regenerated code
- **No ORM Overhead**: Direct query execution

## 📊 Database Schema Structure

**Core Tables**:
```sql
-- Core entities
agents              # AI agents definitions
agent_runs          # Agent execution history  
environments        # Environment configurations
users               # User accounts

-- MCP Integration  
mcp_configs         # MCP server configurations
mcp_tools           # Available MCP tools
agent_tools         # Agent-to-tool assignments

-- File Configuration (New System)
file_configs        # File-based configurations
file_config_envs    # Environment associations

-- Infrastructure
webhooks            # Webhook configurations
webhook_deliveries  # Webhook delivery tracking
```

**Relationship Overview**:
```
Users 1:N Environments 1:N Agents 1:N AgentRuns
                       │
                       └── N:N AgentTools N:1 MCPTools
```

## 🏗️ Repository Pattern Implementation

### **Repository Structure**
```
internal/db/repositories/
├── repositories.go        # Repository coordinator
├── agents.go             # Agent CRUD operations
├── agent_runs.go         # Agent execution tracking
├── environments.go       # Environment management
├── mcp_configs.go        # MCP configuration management
├── mcp_tools.go          # MCP tool operations
├── agent_tools.go        # Agent-tool associations
├── webhooks.go           # Webhook management
└── users.go              # User management
```

### **Repository Interface Pattern**
```go
// Repository interface definition
type AgentRepository interface {
    Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, cronSchedule *string, scheduleEnabled bool) (*models.Agent, error)
    GetByID(id int64) (*models.Agent, error)
    GetByEnvironment(environmentID int64) ([]*models.Agent, error)
    Update(id int64, updates models.AgentUpdate) (*models.Agent, error)
    Delete(id int64) error
    List(limit, offset int) ([]*models.Agent, error)
}

// Repository implementation
type agentRepository struct {
    db *sql.DB
    queries *db.Queries  // sqlc generated queries
}
```

### **sqlc Query Examples**
```sql
-- queries/agents.sql

-- name: CreateAgent :one
INSERT INTO agents (
    name, description, prompt, max_steps, 
    environment_id, created_by, cron_schedule, 
    schedule_enabled, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, 
    datetime('now'), datetime('now')
) RETURNING *;

-- name: GetAgentByID :one
SELECT * FROM agents WHERE id = ?;

-- name: GetAgentsByEnvironment :many
SELECT * FROM agents 
WHERE environment_id = ? 
ORDER BY created_at DESC;

-- name: UpdateAgent :one
UPDATE agents 
SET name = ?, description = ?, prompt = ?, 
    max_steps = ?, updated_at = datetime('now')
WHERE id = ? 
RETURNING *;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE id = ?;
```

**Generated Go Code** (sqlc output):
```go
// Generated types
type Agent struct {
    ID              int64     `json:"id"`
    Name            string    `json:"name"`
    Description     string    `json:"description"`
    Prompt          string    `json:"prompt"`
    MaxSteps        int64     `json:"max_steps"`
    EnvironmentID   int64     `json:"environment_id"`
    CreatedBy       int64     `json:"created_by"`
    CronSchedule    *string   `json:"cron_schedule"`
    ScheduleEnabled bool      `json:"schedule_enabled"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}

// Generated query methods
func (q *Queries) CreateAgent(ctx context.Context, arg CreateAgentParams) (Agent, error) {
    // Generated implementation
}
```

## 🔄 Migration System

### **Migration Files** (`internal/db/migrations/`)
```
001_initial_schema.sql          # Core tables
002_add_environments.sql        # Environment support
003_mcp_integration.sql         # MCP server integration
...
016_add_file_config_support.sql # Latest: File-based configs
```

### **Migration Pattern**
```sql
-- migrations/016_add_file_config_support.sql

-- +goose Up
CREATE TABLE file_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    environment_id INTEGER NOT NULL,
    config_path TEXT NOT NULL,
    template_path TEXT,
    variables_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (environment_id) REFERENCES environments (id)
);

CREATE INDEX idx_file_configs_environment 
ON file_configs(environment_id);

-- +goose Down
DROP INDEX idx_file_configs_environment;
DROP TABLE file_configs;
```

### **Running Migrations**
```go
// internal/db/migrate.go
func RunMigrations(db *sql.DB) error {
    goose.SetBaseFS(embedMigrations)
    
    if err := goose.SetDialect("sqlite3"); err != nil {
        return err
    }
    
    if err := goose.Up(db, "migrations"); err != nil {
        return err
    }
    
    return nil
}
```

## 🛠️ Development Workflows

### **Adding New Tables**
1. **Create Migration**: Add new `.sql` file in `migrations/`
2. **Update Schema**: Run migration to update `schema.sql`
3. **Write Queries**: Add SQL queries in `queries/{table}.sql`
4. **Generate Code**: Run `sqlc generate` to create Go code
5. **Create Repository**: Implement repository interface
6. **Add to Coordinator**: Wire up in `repositories.go`

### **sqlc Configuration** (`sqlc.yaml`)
```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "internal/db/queries"
    schema: "internal/db/schema.sql"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "database/sql"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: false
```

### **Development Commands**
```bash
# Generate sqlc code after schema/query changes
sqlc generate

# Run migrations (done automatically at startup)
go run cmd/main/main.go migrate

# Reset database (development only)
rm station.db && go run cmd/main/main.go init
```

## 🎨 Query Patterns and Best Practices

### **Standard CRUD Pattern**
```go
func (r *agentRepository) Create(name, description, prompt string, maxSteps, environmentID, createdBy int64, cronSchedule *string, scheduleEnabled bool) (*models.Agent, error) {
    agent, err := r.queries.CreateAgent(context.Background(), db.CreateAgentParams{
        Name:            name,
        Description:     description,
        Prompt:          prompt,
        MaxSteps:        maxSteps,
        EnvironmentID:   environmentID,
        CreatedBy:       createdBy,
        CronSchedule:    cronSchedule,
        ScheduleEnabled: scheduleEnabled,
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to create agent: %w", err)
    }
    
    // Convert sqlc type to domain model
    return &models.Agent{
        ID:              agent.ID,
        Name:            agent.Name,
        Description:     agent.Description,
        // ... other fields
    }, nil
}
```

### **Transaction Pattern**
```go
func (r *agentRepository) CreateWithTools(agent AgentParams, toolIDs []int64) (*models.Agent, error) {
    tx, err := r.db.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    // Create agent
    qtx := r.queries.WithTx(tx)
    createdAgent, err := qtx.CreateAgent(ctx, agent)
    if err != nil {
        return nil, err
    }
    
    // Assign tools
    for _, toolID := range toolIDs {
        _, err = qtx.CreateAgentTool(ctx, db.CreateAgentToolParams{
            AgentID: createdAgent.ID,
            ToolID:  toolID,
        })
        if err != nil {
            return nil, err
        }
    }
    
    return tx.Commit()
}
```

### **Complex Query Pattern**
```sql
-- name: GetAgentExecutionStats :many
SELECT 
    a.id,
    a.name,
    COUNT(ar.id) as total_runs,
    COUNT(CASE WHEN ar.status = 'completed' AND ar.success = true THEN 1 END) as successful_runs,
    AVG(ar.duration_ms) as avg_duration_ms,
    MAX(ar.created_at) as last_run_at
FROM agents a
LEFT JOIN agent_runs ar ON a.id = ar.agent_id
WHERE a.environment_id = ?
GROUP BY a.id, a.name
ORDER BY total_runs DESC;
```

## 📊 Performance Considerations

### **Indexing Strategy**
```sql
-- Environment-based queries (very common)
CREATE INDEX idx_agents_environment_id ON agents(environment_id);
CREATE INDEX idx_agent_runs_agent_id ON agent_runs(agent_id);

-- Status-based filtering
CREATE INDEX idx_agent_runs_status ON agent_runs(status);

-- Time-based queries  
CREATE INDEX idx_agent_runs_created_at ON agent_runs(created_at);

-- Composite indexes for complex queries
CREATE INDEX idx_agents_env_status ON agents(environment_id, enabled);
```

### **Query Optimization**
- **Use LIMIT**: Always paginate large result sets
- **Prepared Statements**: sqlc generates prepared statements automatically
- **Avoid N+1**: Use JOINs or batch queries for related data
- **Connection Pooling**: Single connection for SQLite (no pool needed)

## 🔒 Security and Data Integrity

### **SQL Injection Prevention**
- **sqlc**: Automatic parameter binding prevents injection
- **Prepared Statements**: All queries are prepared statements
- **Type Safety**: Compile-time validation of queries

### **Data Validation**
```go
func (r *agentRepository) Create(params CreateParams) (*Agent, error) {
    // Validate before database operation
    if strings.TrimSpace(params.Name) == "" {
        return nil, errors.New("agent name cannot be empty")
    }
    
    if params.MaxSteps <= 0 {
        return nil, errors.New("max_steps must be positive")
    }
    
    // Proceed with creation
    return r.queries.CreateAgent(ctx, params)
}
```

### **Backup Strategy**
```bash
# SQLite backup (simple file copy when not in use)
cp station.db station.db.backup

# Or use SQLite backup API for live backups
sqlite3 station.db ".backup backup.db"
```

---
*Next: See `repositories.md` for specific repository implementations and patterns*