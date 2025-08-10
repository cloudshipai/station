# Agent Template System

**Complete template lifecycle for AI agents - Station's core innovation**

Station's Agent Template System enables packaging complete AI agent solutions as reusable templates that deploy consistently across environments with GitOps workflows.

## 🎯 System Overview

The Agent Template System provides enterprise-grade template management for AI agents, solving the biggest problems in AI agent deployment: **configuration drift across environments, inability to package complete agent setups, lack of reusable patterns, and complex multi-environment management.**

```
Template Lifecycle:
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   CREATE    │───▶│  VALIDATE   │───▶│  INSTALL    │───▶│   MANAGE    │
│             │    │             │    │             │    │             │
│ Bundle      │    │ Schema +    │    │ Variables + │    │ Database +  │
│ Scaffolding │    │ Dependencies│    │ Resolution  │    │ Status      │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

## 📦 Core Components

### **1. Creator** (`pkg/agent-bundle/creator/`)

**Purpose**: Template scaffolding, export from existing agents

**Key Files**:
- `creator.go`: Main creator implementation with filesystem abstraction
- `creator_test.go`: Comprehensive test coverage (79.2%)

**Operations**:
- `Create(bundlePath, opts)`: Generate new template scaffolding
- `ExportFromAgent(agentID, outputPath, opts)`: Convert existing agent to template
- `AnalyzeDependencies(agentID)`: Extract MCP dependencies from existing agent

**Template Structure Created**:
```
bundle/
├── manifest.json              # Template metadata and dependencies
├── agent.json                 # Agent configuration with {{variables}}
├── variables.schema.json      # Variable validation schema
├── examples/
│   ├── basic-usage.md         # Usage documentation  
│   └── api-integration.json   # API payload examples
└── README.md                  # Template documentation
```

### **2. Validator** (`pkg/agent-bundle/validator/`)

**Purpose**: Comprehensive template validation with actionable suggestions

**Key Files**:
- `validator.go`: Multi-layer validation with detailed error reporting
- `validator_test.go`: 80.3% test coverage with validation scenarios

**Validation Layers**:
1. **Manifest Validation**: Required fields, version constraints, metadata
2. **Agent Config Validation**: Template syntax, required fields, tool references
3. **Variable Schema Validation**: JSON Schema compliance, type checking
4. **Dependency Validation**: MCP bundle availability, tool compatibility
5. **Template Syntax Validation**: Go template parsing, variable resolution

**Validation Result Structure**:
```go
type ValidationResult struct {
    Valid       bool                    // Overall validation status
    Errors      []ValidationError       // Critical errors preventing use
    Warnings    []ValidationWarning     // Non-blocking issues with suggestions
    Statistics  ValidationStatistics    // Bundle complexity metrics
}
```

### **3. Manager** (`pkg/agent-bundle/manager/`)

**Purpose**: Template installation, duplication, and lifecycle management

**Key Files**:
- `manager.go`: Complete installation and management logic
- `manager_test.go`: 70.5% test coverage with installation scenarios

**Core Operations**:
- `Install(bundlePath, environment, variables)`: Deploy template with variable substitution
- `Duplicate(agentID, targetEnv, opts)`: Cross-environment agent deployment
- `GetStatus(agentID)`: Agent health and dependency status
- `List()`: All installed templates with metadata
- `Remove(agentID, opts)`: Safe removal with dependency cleanup
- `Update(bundlePath, agentID, opts)`: Update from newer template versions

**Database Integration**:
```go
// Manager supports both standalone and database-integrated modes
manager := manager.New(fs, validator, resolver)                    // Standalone
manager := manager.NewWithDatabaseAccess(fs, validator, resolver,  // Full database
    repos.Agents, repos.AgentTools, repos.Environments)
```

### **4. Resolver** (`pkg/agent-bundle/resolver/`)

**Purpose**: MCP dependency resolution with conflict detection

**Key Files**:
- `resolver.go`: Real dependency resolution with registry integration

**Resolution Process**:
1. **Bundle Availability**: Check registry for required MCP bundles
2. **Version Constraints**: Validate version compatibility
3. **Conflict Detection**: Identify tool conflicts between bundles
4. **Installation Order**: Determine optimal dependency sequence
5. **Tool Validation**: Environment-specific tool availability

**Resolution Result**:
```go
type ResolutionResult struct {
    Success         bool                    // Overall resolution success
    ResolvedBundles []MCPBundleRef         // Successfully resolved bundles
    MissingBundles  []MCPBundleDependency  // Unavailable dependencies
    Conflicts       []ToolConflict         // Tool conflicts to resolve
    InstallOrder    []string               // Optimal installation sequence
}
```

## 🛠️ CLI Integration

The Agent Template System integrates seamlessly with Station's CLI through extended `stn agent bundle` commands:

### **Template Creation**
```bash
# Create new template bundle
stn agent bundle create ./my-template \
  --name "Database Monitor" \
  --author "Platform Team" \
  --description "PostgreSQL monitoring with alerts"

# Export existing agent as template
stn agent bundle export 5 ./shared-templates/db-monitor \
  --include-deps --analyze-vars
```

### **Template Validation**
```bash
# Comprehensive validation with suggestions
stn agent bundle validate ./my-template

# Example output:
# ✅ Bundle is valid!
# 📊 Statistics: 3 variables (2 required, 1 optional), 2 MCP dependencies
# ⚠️  Warnings: Consider adding examples/ directory for better documentation
```

### **Template Installation**
```bash
# Install with variables file
stn agent bundle install ./my-template \
  --vars-file production.json \
  --env production

# Interactive installation with prompts
stn agent bundle install ./my-template --interactive --env staging

# CLI variables override
stn agent bundle install ./my-template \
  --vars DB_HOST=prod-db.company.com \
  --vars ALERT_THRESHOLD=90 \
  --env production
```

### **Cross-Environment Operations**
```bash
# Duplicate agent across environments
stn agent bundle duplicate 5 staging \
  --name "DB Monitor (Staging)" \
  --vars-file staging-vars.json \
  --interactive
```

## 🌐 API Integration

Full REST API support for automation and CI/CD integration:

### **Template Installation API**
```http
POST /api/v1/agents/templates/install
Content-Type: application/json

{
  "bundle_path": "./templates/database-monitor",
  "environment": "production",
  "variables": {
    "DB_HOST": "prod-db.company.com",
    "ALERT_THRESHOLD": 90,
    "SLACK_WEBHOOK": "https://hooks.slack.com/prod-alerts"
  }
}
```

### **Response Format**
```json
{
  "message": "Agent template installed successfully",
  "agent_id": 12,
  "agent_name": "Database Monitor",
  "environment": "production",
  "tools_installed": 5,
  "mcp_bundles": ["postgresql-tools@1.2.0", "slack-notify@2.1.0"]
}
```

## 🔧 Advanced Features

### **Variable System**
- **Type Preservation**: JSON/YAML variables maintain original types
- **Interactive Prompts**: Masked sensitive input, validation, defaults
- **Hierarchical Resolution**: CLI flags → vars-file → defaults → schema
- **Template Engine**: Full Go template support with conditionals

### **Multi-Environment Support**
- **Environment Isolation**: Separate agent execution contexts
- **Variable Overrides**: Environment-specific configuration
- **Cross-Environment Deployment**: Safe agent duplication
- **Template Rendering**: Environment-aware variable substitution

### **Dependency Management**
- **MCP Bundle Resolution**: Registry integration with version constraints
- **Conflict Detection**: Automatic tool conflict identification
- **Installation Order**: Dependency graph resolution
- **Environment Validation**: Target environment compatibility checking

## 📊 Implementation Details

### **Package Structure**
```go
pkg/agent-bundle/
├── types.go              # Core type definitions and interfaces
├── interfaces.go         # Public interfaces for extensibility
├── creator/
│   ├── creator.go        # Template creation and export
│   └── creator_test.go   # 79.2% test coverage
├── validator/
│   ├── validator.go      # Multi-layer validation
│   └── validator_test.go # 80.3% test coverage  
├── manager/
│   ├── manager.go        # Installation and management
│   └── manager_test.go   # 70.5% test coverage
└── resolver/
    └── resolver.go       # Dependency resolution
```

### **Database Integration**
The Manager component integrates with Station's database layer through repository pattern:

```go
type Manager struct {
    fs           afero.Fs
    validator    Validator
    resolver     DependencyResolver
    
    // Database repositories (optional)
    agentRepo    AgentRepository
    toolRepo     AgentToolRepository  
    envRepo      EnvironmentRepository
}
```

### **Error Handling**
Comprehensive error handling with user-friendly messages:

```go
type ValidationError struct {
    Field       string  // Field that failed validation
    Message     string  // User-friendly error message
    Suggestion  string  // Actionable suggestion for fixing
    Code        string  // Error code for programmatic handling
}
```

## 🎯 Usage Patterns

### **Platform Team Pattern**
```bash
# Create standardized templates
stn agent bundle create ./templates/monitoring-agent --name "Standard Monitor"
stn agent bundle create ./templates/deployment-agent --name "Standard Deployer"

# Validate and version control
git add templates/
git commit -m "Add standardized agent templates"
git push origin main
```

### **Development Team Pattern**
```bash
# Install from shared templates
git pull && stn agent bundle install ./templates/monitoring-agent \
  --vars-file dev-vars.json --env development

# Promote through environments  
stn agent bundle duplicate 1 staging --vars-file staging-vars.json
stn agent bundle duplicate 2 production --vars-file prod-vars.json
```

### **GitOps Integration Pattern**
```yaml
# .github/workflows/deploy-agents.yml
- name: Deploy Agent Templates
  run: |
    stn agent bundle install ./templates/monitoring \
      --vars-file ${{ env.ENVIRONMENT }}-vars.json \
      --env ${{ env.ENVIRONMENT }}
```

## 🚀 Performance & Scale

- **Template Validation**: Sub-second validation for typical templates
- **Variable Resolution**: Efficient Go template engine with caching
- **Database Operations**: sqlc-generated queries for type safety and performance
- **Concurrent Installation**: Parallel MCP dependency resolution
- **Memory Usage**: Streaming template processing for large bundles

---

**The Agent Template System represents Station's core innovation** - solving enterprise AI agent deployment complexity through reusable, version-controlled templates with complete dependency management and multi-environment consistency.

*Implementation Status: ✅ Complete - All PRD requirements delivered*