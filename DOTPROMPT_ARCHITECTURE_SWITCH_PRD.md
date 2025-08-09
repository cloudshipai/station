# **PRD: Dotprompt Architecture Switch**
## **Station Agent System Redesign**

### **Executive Summary**
Redesign Station's agent system to use dotprompt files as the primary interface with name-based addressing, simplified CLI commands, and declarative MCP synchronization. This eliminates ID-based workflows and makes Station more developer-friendly.

---

## **🎯 Core Changes**

### **1. Agent Addressing: ID → Name**
```bash
# OLD: Cryptic ID-based commands
stn agent run 13 "check system status"
stn agent show 13
stn agent delete 13

# NEW: Intuitive name-based commands  
stn agent run monitoring-agent "check system status"
stn agent show monitoring-agent
stn agent delete monitoring-agent
```

### **2. Execution Method: Database → Dotprompt**
```bash
# OLD: Multiple execution paths
stn agent run 13 "task"           # Database execution
stn agent run-dotprompt 13 "task" # Dotprompt execution

# NEW: Single dotprompt execution path
stn agent run monitoring-agent "task"  # Always uses .prompt file
```

### **3. Configuration: Bundles → Single Files**
```bash
# OLD: Complex bundle management
stn template install monitoring-bundle
stn agent bundle install monitoring-bundle.tar.gz

# NEW: Simple .prompt file management
stn template install monitoring-agent.prompt
stn agent install monitoring-agent.prompt
```

---

## **📋 Database Schema Changes**

### **Agent Table Redesign (SQLite + SQLC)**

```sql
-- migrations/001_agent_name_primary_key.sql
CREATE TABLE agents_new (
    name TEXT PRIMARY KEY,                    -- Primary key changed from ID to name
    display_name TEXT NOT NULL,               -- Human-readable name
    description TEXT NOT NULL,
    file_path TEXT NOT NULL,                  -- Path to .prompt file
    environment_name TEXT NOT NULL,           -- FK to environments.name
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_executed_at TIMESTAMP,
    execution_count INTEGER DEFAULT 0,
    checksum_md5 TEXT,                        -- File integrity check
    status TEXT DEFAULT 'active',             -- active, archived, error
    
    FOREIGN KEY (environment_name) REFERENCES environments (name),
    UNIQUE (name, environment_name)           -- Unique within environment
);

-- Drop old table after migration
DROP TABLE agents;
ALTER TABLE agents_new RENAME TO agents;

-- Update related tables
CREATE TABLE agent_runs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name TEXT NOT NULL,                 -- Changed from agent_id
    environment_name TEXT NOT NULL,
    task TEXT NOT NULL,
    status TEXT NOT NULL,
    result TEXT,
    duration_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (agent_name, environment_name) REFERENCES agents (name, environment_name)
);

-- Agent tools assignment (simplified)
CREATE TABLE agent_tools_new (
    agent_name TEXT NOT NULL,
    environment_name TEXT NOT NULL,
    mcp_config_name TEXT NOT NULL,            -- MCP config providing the tool
    tool_name TEXT NOT NULL,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (agent_name, environment_name, tool_name),
    FOREIGN KEY (agent_name, environment_name) REFERENCES agents (name, environment_name)
);
```

### **SQLC Queries**

```sql
-- queries/agents.sql

-- name: GetAgent :one
SELECT * FROM agents WHERE name = ? AND environment_name = ?;

-- name: GetAgentByPath :one  
SELECT * FROM agents WHERE file_path = ?;

-- name: ListAgents :many
SELECT * FROM agents WHERE environment_name = ? ORDER BY name;

-- name: CreateAgent :one
INSERT INTO agents (name, display_name, description, file_path, environment_name)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateAgentFromFile :exec
UPDATE agents 
SET display_name = ?, description = ?, checksum_md5 = ?, updated_at = CURRENT_TIMESTAMP
WHERE name = ? AND environment_name = ?;

-- name: DeleteAgent :exec
DELETE FROM agents WHERE name = ? AND environment_name = ?;

-- name: GetAgentTools :many
SELECT mcp_config_name, tool_name FROM agent_tools 
WHERE agent_name = ? AND environment_name = ?;

-- name: SyncAgentTools :exec
-- This will be called during mcp sync to update tool assignments
```

---

## **🔧 Command Design**

### **Simplified CLI Commands**

```bash
# AGENT MANAGEMENT
stn agent list [environment]                    # List all agents
stn agent show monitoring-agent [--env prod]    # Show agent details
stn agent run monitoring-agent "task" [--env prod]  # Execute agent
stn agent delete monitoring-agent [--env prod]  # Delete agent
stn agent edit monitoring-agent [--env prod]    # Open .prompt file in editor

# TEMPLATE/INSTALLATION  
stn agent install monitoring-agent.prompt [--env prod]     # Install .prompt file
stn agent export monitoring-agent [--env prod] [--output file.prompt]  # Export to .prompt

# MCP SYNCHRONIZATION (Key New Command)
stn mcp sync [--dry-run] [--validate] [--env prod]         # Sync all file configs to DB

# ENVIRONMENT MANAGEMENT
stn env list                                     # List environments
stn env create staging                           # Create environment
stn env sync staging                             # Sync environment configs
```

### **Command Examples**

```bash
# Install agent from template
curl -o /tmp/monitoring.prompt https://templates.station.ai/monitoring.prompt
stn agent install /tmp/monitoring.prompt --env production

# Run agent with natural naming
stn agent run monitoring-agent "check database connections" --env production

# Edit agent in place
stn agent edit monitoring-agent --env staging
# Opens: agents/staging/monitoring-agent.prompt in $EDITOR

# Sync all file-based configs
stn mcp sync --validate --env production
```

---

## **📁 File System Organization**

### **Directory Structure**

```
/workspace/
├── environments/
│   ├── default/
│   │   ├── mcp-config.yaml                 # MCP server definitions
│   │   └── agents/
│   │       ├── monitoring-agent.prompt     # Agent definitions
│   │       ├── scanner-agent.prompt
│   │       └── alerting-agent.prompt
│   ├── staging/
│   │   ├── mcp-config.yaml
│   │   └── agents/
│   │       ├── monitoring-agent.prompt     # Staging version
│   │       └── test-agent.prompt
│   └── production/
│       ├── mcp-config.yaml
│       └── agents/
│           ├── monitoring-agent.prompt     # Production version
│           ├── business-critical-agent.prompt
│           └── compliance-agent.prompt
├── templates/
│   ├── monitoring-agent.prompt             # Template library
│   ├── data-processing-agent.prompt
│   └── alerting-agent.prompt
└── .station/
    ├── config.yaml                         # Station configuration
    └── cache/                               # Runtime cache
```

### **Agent File Format (Enhanced)**

```yaml
---
# Standard dotprompt fields
model: googleai/gemini-1.5-flash
input:
  schema: {task: string, context: object, parameters: object}
output:
  format: json
  schema: {result: object, metadata: object}

# Station metadata
metadata:
  name: monitoring-agent                     # Must match filename
  display_name: "System Monitoring Agent"
  description: "Monitors system health and performance metrics"
  environment: production
  max_steps: 5
  version: "2.1.0"
  author: "DevOps Team"
  tags: ["monitoring", "system", "alerts"]

# MCP Dependencies (replaces complex tool assignment)
station:
  mcp_dependencies:
    grafana-prod:
      server_name: "mcp-grafana"
      assigned_tools:
        - __find_error_pattern_logs
        - __create_incident  
        - __find_slow_requests
        - __add_activity_to_incident
      config_version: "v1-88"
    
    system-monitor:
      server_name: "mcp-system"
      assigned_tools:
        - __get_cpu_usage
        - __check_disk_space
      config_version: "v2-12"

  execution_profile:
    timeout_seconds: 300
    retry_attempts: 3
    resource_limits:
      memory_mb: 512
      cpu_millicores: 500

# Business/Enterprise metadata (unlimited custom fields)
team_info:
  lead: "Alice Johnson"
  contact: "devops@company.com"
  slack_channel: "#system-alerts"

business_rules:
  sla_target: "99.9%"
  max_response_time: 5000
  compliance_framework: "SOC2"

deployment_config:
  cloud_provider: "AWS"
  region: "us-west-2"
  cost_center: "INFRA-001"
---

{{#system}}
You are {{metadata.display_name}} ({{metadata.name}}).
Environment: {{metadata.environment}}
Team Contact: {{team_info.contact}} | Slack: {{team_info.slack_channel}}
SLA Target: {{business_rules.sla_target}}

You have access to tools from:
{{#each station.mcp_dependencies}}
- {{server_name}}: {{assigned_tools.length}} tools
{{/each}}

{{#if business_rules.compliance_framework}}
⚠️  COMPLIANCE: This agent operates under {{business_rules.compliance_framework}} requirements.
All actions must be logged and auditable.
{{/if}}
{{/system}}

**Task**: {{task}}

{{#if context}}
**Context**: {{toJson context}}
{{/if}}

**Available Tools**:
{{#each station.mcp_dependencies}}
From {{server_name}}:
{{#each assigned_tools}}
- {{.}}
{{/each}}

{{/each}}

Please complete this task within {{business_rules.max_response_time}}ms SLA.
Team escalation: {{team_info.contact}}
```

---

## **🔄 MCP Sync Implementation**

### **Declarative Synchronization**

The `stn mcp sync` command becomes the cornerstone of the system:

```go
// cmd/main/handlers/mcp/sync.go
func syncMCPConfigs(cmd *cobra.Command, args []string) error {
    environment := viper.GetString("environment")
    dryRun := viper.GetBool("dry-run") 
    validate := viper.GetBool("validate")
    
    syncer := mcp.NewDeclarativeSync(repos, environment)
    
    if dryRun {
        return syncer.PlanSync()
    }
    
    if validate {
        if err := syncer.ValidateConfigs(); err != nil {
            return err
        }
    }
    
    return syncer.ExecuteSync()
}

// internal/services/mcp_declarative_sync.go
type DeclarativeSync struct {
    repos       *repositories.Repositories
    environment string
}

func (s *DeclarativeSync) ExecuteSync() error {
    log.Printf("🔄 Starting MCP sync for environment: %s", s.environment)
    
    // 1. Scan file system for configs
    mcpConfigPath := fmt.Sprintf("environments/%s/mcp-config.yaml", s.environment)
    agentDir := fmt.Sprintf("environments/%s/agents/", s.environment)
    
    // 2. Parse MCP configuration
    mcpConfig, err := s.parseMCPConfig(mcpConfigPath)
    if err != nil {
        return fmt.Errorf("failed to parse MCP config: %w", err)
    }
    
    // 3. Scan agent .prompt files
    agentFiles, err := filepath.Glob(filepath.Join(agentDir, "*.prompt"))
    if err != nil {
        return err
    }
    
    // 4. Process each agent
    for _, agentFile := range agentFiles {
        if err := s.syncAgentFromFile(agentFile); err != nil {
            log.Printf("❌ Failed to sync %s: %v", agentFile, err)
            continue
        }
        log.Printf("✅ Synced %s", filepath.Base(agentFile))
    }
    
    // 5. Update MCP server connections
    return s.syncMCPServers(mcpConfig)
}

func (s *DeclarativeSync) syncAgentFromFile(filePath string) error {
    // Parse .prompt file
    extractor, err := dotprompt.NewRuntimeExtraction(filePath)
    if err != nil {
        return err
    }
    
    config := extractor.GetConfig()
    agentName := config.Metadata.Name
    
    // Validate agent name matches filename
    expectedName := strings.TrimSuffix(filepath.Base(filePath), ".prompt")
    if agentName != expectedName {
        return fmt.Errorf("agent name '%s' doesn't match filename '%s'", agentName, expectedName)
    }
    
    // Extract MCP dependencies
    mcpDeps, err := extractor.ExtractCustomField("station.mcp_dependencies")
    if err != nil {
        return fmt.Errorf("failed to extract MCP dependencies: %w", err)
    }
    
    // Validate all tools exist
    if err := s.validateMCPDependencies(mcpDeps); err != nil {
        return err
    }
    
    // Calculate file checksum
    checksum, err := s.calculateChecksum(filePath)
    if err != nil {
        return err
    }
    
    // Upsert agent in database
    params := db.CreateAgentParams{
        Name:            agentName,
        DisplayName:     config.Metadata.Name, // Using display_name from metadata
        Description:     config.Metadata.Description,
        FilePath:        filePath,
        EnvironmentName: s.environment,
        ChecksumMd5:     checksum,
    }
    
    // Check if agent exists
    existing, err := s.repos.Agents.GetAgent(context.Background(), db.GetAgentParams{
        Name:            agentName,
        EnvironmentName: s.environment,
    })
    
    if err != nil {
        // Create new agent
        _, err = s.repos.Agents.CreateAgent(context.Background(), params)
        if err != nil {
            return fmt.Errorf("failed to create agent: %w", err)
        }
    } else {
        // Update existing agent if file changed
        if existing.ChecksumMd5 != checksum {
            err = s.repos.Agents.UpdateAgentFromFile(context.Background(), db.UpdateAgentFromFileParams{
                DisplayName:     params.DisplayName,
                Description:     params.Description,
                ChecksumMd5:     params.ChecksumMd5,
                Name:            agentName,
                EnvironmentName: s.environment,
            })
            if err != nil {
                return fmt.Errorf("failed to update agent: %w", err)
            }
        }
    }
    
    // Sync tool assignments
    return s.syncAgentTools(agentName, mcpDeps)
}

func (s *DeclarativeSync) validateMCPDependencies(deps interface{}) error {
    if deps == nil {
        return nil // No dependencies
    }
    
    depsMap, ok := deps.(map[string]interface{})
    if !ok {
        return fmt.Errorf("invalid MCP dependencies format")
    }
    
    for mcpConfigName, depData := range depsMap {
        depMap := depData.(map[string]interface{})
        assignedTools := depMap["assigned_tools"].([]interface{})
        
        // Check MCP config exists
        mcpConfig, err := s.getMCPConfig(mcpConfigName)
        if err != nil {
            return fmt.Errorf("MCP config '%s' not found: %w", mcpConfigName, err)
        }
        
        // Get available tools from MCP server
        availableTools, err := s.getAvailableToolsForConfig(mcpConfig)
        if err != nil {
            return fmt.Errorf("failed to get tools for MCP config '%s': %w", mcpConfigName, err)
        }
        
        // Validate each assigned tool
        for _, toolInterface := range assignedTools {
            toolName := toolInterface.(string)
            if !contains(availableTools, toolName) {
                return fmt.Errorf("tool '%s' not available in MCP config '%s'\nAvailable tools: %v", 
                    toolName, mcpConfigName, availableTools)
            }
        }
        
        log.Printf("✅ Validated MCP config '%s': %d tools", mcpConfigName, len(assignedTools))
    }
    
    return nil
}
```

### **Sync Output Example**

```bash
$ stn mcp sync --validate --env production

🔄 Starting MCP sync for environment: production

📂 Scanning configuration files...
  └── environments/production/mcp-config.yaml ✅
  └── environments/production/agents/ (3 files found)

🤖 Processing agents...
  ✅ monitoring-agent.prompt
     └── grafana-prod: 4 tools validated
     └── system-monitor: 2 tools validated
  ✅ business-critical-agent.prompt  
     └── grafana-prod: 3 tools validated
     └── compliance-audit: 5 tools validated
  ❌ test-agent.prompt
     └── ERROR: Tool '__nonexistent_tool' not found in MCP config 'grafana-prod'
     └── Available tools: [__find_error_pattern_logs, __create_incident, __find_slow_requests, __add_activity_to_incident]

🔧 Updating database...
  └── Created: business-critical-agent (new)
  └── Updated: monitoring-agent (checksum changed)
  └── Skipped: test-agent (validation errors)

📊 Summary:
  ✅ 2 agents synced successfully
  ❌ 1 agent has validation errors
  🔧 3 MCP configs processed
  📁 Database updated with 2 agent records
  ⏱️  Sync completed in 1.2s
```

---

## **📦 Template & Bundle System Redesign**

### **Simplified Template Management**

```bash
# TEMPLATE INSTALLATION (replaces bundles)
stn template install https://templates.station.ai/monitoring-agent.prompt
stn template install ./local-agent.prompt --name custom-monitoring

# TEMPLATE LIBRARY
stn template list                           # List available templates
stn template show monitoring-agent          # Show template details
stn template search "monitoring"            # Search templates

# AGENT INSTALLATION FROM TEMPLATES
stn agent install monitoring-agent --from-template --env production
# Equivalent to:
# 1. Download monitoring-agent.prompt template
# 2. Customize for environment
# 3. Save to environments/production/agents/monitoring-agent.prompt
# 4. Run mcp sync to update database
```

### **Environment Bundle Management**

Instead of complex bundles, use simple directory operations:

```bash
# ENVIRONMENT SETUP
stn env create staging --from production    # Copy all configs from production
stn env export production --output prod-backup.tar.gz
stn env import staging-configs.tar.gz --env staging

# BULK AGENT OPERATIONS  
stn agent install-all ./agent-templates/ --env production
# Installs all .prompt files from directory

stn agent export-all --env production --output ./backup/
# Exports all agents to directory
```

---

## **🔧 Runtime Configuration Parsing**

### **Agent Execution Flow**

```go
// internal/services/dotprompt_agent_executor.go
type DotpromptAgentExecutor struct {
    repos           *repositories.Repositories
    mcpConnManager  *MCPConnectionManager
    genkitExecutor  *dotprompt.GenkitExecutor
}

func (e *DotpromptAgentExecutor) ExecuteAgent(ctx context.Context, agentName, environment, task string) (*AgentExecutionResult, error) {
    // 1. Get agent metadata from database
    agent, err := e.repos.Agents.GetAgent(ctx, db.GetAgentParams{
        Name:            agentName,
        EnvironmentName: environment,
    })
    if err != nil {
        return nil, fmt.Errorf("agent '%s' not found in environment '%s': %w", agentName, environment, err)
    }
    
    // 2. Load .prompt file
    extractor, err := dotprompt.NewRuntimeExtraction(agent.FilePath)
    if err != nil {
        return nil, fmt.Errorf("failed to load agent file %s: %w", agent.FilePath, err)
    }
    
    // 3. Verify file integrity
    currentChecksum, _ := calculateChecksum(agent.FilePath)
    if currentChecksum != agent.ChecksumMd5 {
        return nil, fmt.Errorf("agent file %s has been modified (run 'stn mcp sync' to update)", agent.FilePath)
    }
    
    // 4. Extract MCP dependencies and resolve tools
    mcpDeps, err := extractor.ExtractCustomField("station.mcp_dependencies")
    if err != nil {
        return nil, fmt.Errorf("failed to extract MCP dependencies: %w", err)
    }
    
    toolMappings, mcpClients, err := e.resolveMCPTools(ctx, mcpDeps)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve MCP tools: %w", err)
    }
    defer e.cleanupMCPClients(mcpClients)
    
    // 5. Extract business context from frontmatter
    executionContext := e.buildExecutionContext(extractor)
    
    // 6. Execute with rich context
    request := dotprompt.ExecutionRequest{
        Task:              task,
        Context:           executionContext,
        Parameters:        map[string]interface{}{},
        Config:            extractor.GetConfig().Config,
    }
    
    startTime := time.Now()
    result, err := e.genkitExecutor.ExecuteAgentWithDotpromptTemplate(ctx, 
        extractor.GetConfig(), extractor.GetTemplate(), request, toolMappings)
    
    if err != nil {
        return nil, err
    }
    
    // 7. Record execution in database
    _, err = e.repos.AgentRuns.CreateAgentRun(ctx, db.CreateAgentRunParams{
        AgentName:       agentName,
        EnvironmentName: environment,
        Task:            task,
        Status:          getStatusFromResult(result),
        Result:          result.Response,
        DurationMs:      int64(time.Since(startTime).Milliseconds()),
    })
    
    return result, err
}

func (e *DotpromptAgentExecutor) buildExecutionContext(extractor *dotprompt.RuntimeExtraction) map[string]interface{} {
    context := map[string]interface{}{
        "execution_mode": "dotprompt",
    }
    
    // Add business context from frontmatter
    if teamInfo, err := extractor.GetTeamInfo(); err == nil {
        context["team_info"] = teamInfo
    }
    
    if businessRules, err := extractor.GetBusinessRules(); err == nil {
        context["business_rules"] = businessRules
    }
    
    if execProfile, err := extractor.GetExecutionProfile(); err == nil {
        context["execution_profile"] = execProfile
    }
    
    return context
}
```

---

## **📋 Implementation Plan**

### **Phase 1: Foundation (Week 1)**

**Day 1: Database Migration**
- [ ] Create SQLC schema with name-based primary keys
- [ ] Generate Go code with `sqlc generate`  
- [ ] Create migration scripts for existing data
- [ ] Test migration with sample data

**Day 2: File System Setup**
- [ ] Create environment-based directory structure
- [ ] Export all existing agents to .prompt files
- [ ] Implement file checksum validation
- [ ] Basic file→database sync logic

**Day 3: CLI Commands**  
- [ ] Update agent commands to use names instead of IDs
- [ ] Implement `stn mcp sync` command
- [ ] Update `stn agent run` to use dotprompt execution
- [ ] Add validation and error handling

**Day 4-5: MCP Dependency System**
- [ ] Implement MCP dependency parsing from frontmatter
- [ ] Add tool validation during sync
- [ ] Create dependency mapping visualization
- [ ] Test with existing MCP configurations

### **Phase 2: Enhanced Features (Week 2)**

**Day 1: Template System**  
- [ ] Redesign template installation to use .prompt files
- [ ] Implement template library management
- [ ] Create template→agent installation flow

**Day 2-3: Advanced Sync Features**
- [ ] Add dry-run and validation modes to sync
- [ ] Implement conflict resolution for file changes
- [ ] Add rollback capabilities
- [ ] Create sync status reporting

**Day 4-5: Quality of Life**
- [ ] Add agent editing commands (`stn agent edit`)
- [ ] Implement environment cloning
- [ ] Add bulk operations for agents
- [ ] Create comprehensive error messages and help

### **Phase 3: Production Readiness**

**Day 1-2: Testing & Validation**
- [ ] Comprehensive test suite for name-based operations
- [ ] Integration tests for MCP sync
- [ ] Performance testing with large agent catalogs
- [ ] Migration testing with production data

**Day 3: Documentation & Training**
- [ ] Update all documentation for new CLI commands
- [ ] Create migration guide for existing users
- [ ] Record demo videos of new workflows
- [ ] Update help text and examples

**Day 4-5: Deployment**
- [ ] Create feature flag for gradual rollout
- [ ] Deploy to staging environment
- [ ] Migrate sample agents and test thoroughly
- [ ] Production deployment with monitoring

---

## **🚀 Git Branch Strategy**

```bash
# Create feature branch for this major change
git checkout -b feature/dotprompt-architecture-switch

# Implement in logical commits
git add migrations/001_agent_name_primary_key.sql
git commit -m "feat: migrate agents table to name-based primary key"

git add internal/db/queries/agents.sql
git commit -m "feat: add SQLC queries for name-based agent operations"

git add cmd/main/handlers/mcp/sync.go  
git commit -m "feat: implement declarative MCP sync command"

git add cmd/main/handlers/agent/*.go
git commit -m "feat: update agent CLI commands to use names instead of IDs"

git add internal/services/dotprompt_agent_executor.go
git commit -m "feat: implement dotprompt-based agent execution"

# Create PR for review
git push origin feature/dotprompt-architecture-switch
```

---

## **📊 Success Metrics**

### **Developer Experience**
- [ ] **Command Simplicity**: 50% reduction in command complexity
- [ ] **Memorability**: Developers can execute agents by name without docs
- [ ] **Error Clarity**: Self-explanatory error messages with suggestions

### **System Reliability**  
- [ ] **Sync Accuracy**: 100% consistency between files and database
- [ ] **Validation Coverage**: All invalid configurations caught during sync
- [ ] **Rollback Capability**: Ability to revert any configuration change

### **Operational Efficiency**
- [ ] **Deployment Speed**: 75% faster agent deployment (no database schema changes)
- [ ] **Configuration Drift**: Eliminated via declarative sync
- [ ] **Auditability**: Full Git history for all agent changes

---

## **🎯 Summary**

This architecture switch delivers:

1. **🏷️ Natural Naming**: `stn agent run monitoring-agent` instead of cryptic IDs
2. **📁 File-First**: .prompt files as single source of truth
3. **🔄 Declarative Sync**: `stn mcp sync` keeps everything consistent  
4. **📦 Simplified Templates**: Single .prompt files replace complex bundles
5. **🗺️ Dependency Mapping**: Clear visibility of agent→MCP→tool relationships
6. **⚡ Developer Velocity**: Intuitive commands, Git workflows, rich metadata

**Result**: Station becomes significantly more developer-friendly while maintaining all enterprise features through unlimited custom frontmatter.

<function_calls>
<invoke name="TodoWrite">
<parameter name="todos">[{"content": "Create comprehensive PRD for dotprompt architecture switch", "status": "completed", "id": "11"}, {"content": "Design agent name-based primary key system", "status": "completed", "id": "12"}, {"content": "Plan CLI command simplification", "status": "completed", "id": "13"}, {"content": "Design declarative MCP sync system", "status": "completed", "id": "14"}, {"content": "Plan bundle system replacement", "status": "completed", "id": "15"}]