# Agent Template System - Implementation Plan

## 🎯 **Overview**

This document outlines the detailed implementation plan for the Agent Template System, building on Station's existing Template Bundle System. The implementation follows a phased approach with comprehensive test coverage and maintains backward compatibility.

## 🏗️ **Architecture Integration**

### **Package Structure**
```
station/
├── pkg/
│   ├── bundle/                    # Existing template bundle system
│   └── agent-bundle/              # New agent bundle system
│       ├── types.go              # Core data structures
│       ├── interfaces.go         # Clean interface definitions
│       ├── creator/              # Bundle creation logic
│       │   ├── creator.go
│       │   └── creator_test.go
│       ├── validator/            # Bundle validation
│       │   ├── validator.go
│       │   └── validator_test.go
│       ├── manager/              # Installation & management
│       │   ├── manager.go
│       │   └── manager_test.go
│       ├── dependency/           # Dependency resolution
│       │   ├── resolver.go
│       │   └── resolver_test.go
│       └── cli/                  # CLI commands
│           ├── commands.go
│           └── commands_test.go
```

### **Integration Points**
- **Template Bundle System**: Reuse registry, validation, and packaging infrastructure
- **Existing Agent System**: Extend current export/import with bundle capabilities  
- **MCP System**: Integrate with file-based configuration and tool discovery
- **CLI Framework**: Follow same patterns as `stn template` commands

## 📋 **Phase 1: Core Agent Bundle System (V1)**

### **Duration:** 2-3 weeks

### **Core Components**

#### **1. Data Types & Interfaces** 
**File:** `pkg/agent-bundle/types.go`
```go
type AgentBundleManifest struct {
    Name              string                    `json:"name"`
    Version           string                    `json:"version"`
    Description       string                    `json:"description"`
    Author            string                    `json:"author"`
    AgentType         string                    `json:"agent_type"`
    RequiredModel     *ModelRequirement        `json:"required_model,omitempty"`
    MCPBundles        []MCPBundleDependency    `json:"mcp_bundles"`
    RequiredTools     []ToolRequirement        `json:"required_tools"`
    RequiredVariables map[string]VariableSpec  `json:"required_variables"`
    Tags              []string                  `json:"tags,omitempty"`
    StationVersion    string                   `json:"station_version"`
    CreatedAt         time.Time                `json:"created_at"`
}

type AgentTemplateConfig struct {
    Name              string                   `json:"name"`
    Description       string                   `json:"description"`
    Prompt            string                   `json:"prompt"`
    MaxSteps          int64                    `json:"max_steps"`
    ScheduleTemplate  *string                  `json:"schedule_template,omitempty"`
    NameTemplate      string                   `json:"name_template"`
    PromptTemplate    string                   `json:"prompt_template"`
    DefaultVars       map[string]interface{}   `json:"default_variables"`
    Version           string                   `json:"version"`
    CreatedAt         time.Time                `json:"created_at"`
    UpdatedAt         time.Time                `json:"updated_at"`
}

type ToolRequirement struct {
    Name         string   `json:"name"`
    ServerName   string   `json:"server_name"`
    MCPBundle    string   `json:"mcp_bundle"`
    Required     bool     `json:"required"`
    Alternatives []string `json:"alternatives,omitempty"`
}

type MCPBundleDependency struct {
    Name        string `json:"name"`
    Version     string `json:"version"`
    Source      string `json:"source"`
    Required    bool   `json:"required"`
    Description string `json:"description"`
}
```

**Test Coverage Target:** 95% (types are critical foundations)

#### **2. Agent Bundle Creator**
**File:** `pkg/agent-bundle/creator/creator.go`

```go
type Creator struct {
    fs       afero.Fs
    registry bundle.BundleRegistry  // Reuse template bundle registry
}

type CreateOptions struct {
    Name         string
    Author       string
    Description  string
    FromAgent    *int64
    Environment  string
    IncludeMCP   bool
    Variables    map[string]interface{}
}

func (c *Creator) Create(path string, opts CreateOptions) error {
    // Create bundle directory structure
    // Generate manifest.json
    // Create agent.json template
    // Generate tools.json from agent dependencies
    // Create variables.schema.json  
    // Generate README.md
    // Create examples/ directory
}

func (c *Creator) ExportFromAgent(agentID int64, path string, opts ExportOptions) error {
    // Query agent from database
    // Analyze agent tool dependencies
    // Resolve MCP bundle dependencies
    // Extract template variables from agent config
    // Generate bundle structure
}
```

**Key Features:**
- Scaffold new agent bundles from templates
- Export existing agents with full dependency analysis
- Generate Go template syntax (`{{ .VAR }}`) in agent configurations
- Auto-detect MCP bundle dependencies from agent tools
- Create comprehensive documentation and examples

**Test Cases:**
- ✅ Create bundle from scratch with all options
- ✅ Export from existing agent with complex tool dependencies
- ✅ Handle missing dependencies gracefully
- ✅ Generate proper Go template syntax
- ✅ Create valid manifest and schema files
- ✅ Error handling for invalid inputs

**Test Coverage Target:** 85%

#### **3. Agent Bundle Validator**
**File:** `pkg/agent-bundle/validator/validator.go`

```go
type Validator struct {
    bundleValidator bundle.BundleValidator  // Reuse template validation
}

func (v *Validator) Validate(bundlePath string) (*AgentValidationResult, error) {
    // Validate bundle structure
    // Validate manifest.json against schema
    // Validate agent.json configuration
    // Check tool requirements against MCP dependencies
    // Validate variable consistency
    // Check MCP bundle dependency availability
}

func (v *Validator) ValidateAgent(config AgentTemplateConfig) error {
    // Validate agent configuration fields
    // Check Go template syntax in templates
    // Validate schedule expressions if present
    // Check prompt template variables
}

func (v *Validator) ValidateToolMappings(tools []ToolRequirement, mcpBundles []MCPBundleDependency) error {
    // Ensure all required tools map to available MCP bundles
    // Check tool name consistency
    // Validate alternative tool mappings
}
```

**Key Features:**
- Comprehensive bundle structure validation
- JSON schema validation for all configuration files
- Tool dependency validation against MCP bundles
- Template variable consistency checking
- MCP bundle availability verification

**Test Cases:**
- ✅ Valid complete agent bundle passes all checks
- ✅ Missing manifest file detected
- ✅ Invalid agent configuration caught
- ✅ Tool mapping inconsistencies identified
- ✅ Missing MCP dependencies flagged
- ✅ Template variable mismatches caught
- ✅ Invalid Go template syntax detected

**Test Coverage Target:** 85%

#### **4. CLI Integration**
**Files:** `pkg/agent-bundle/cli/commands.go`, `cmd/main/commands.go`, `cmd/main/cli.go`

```go
// New CLI commands
func NewAgentBundleCLI(registry BundleRegistry) *AgentBundleCLI

// Commands to implement:
func (cli *AgentBundleCLI) CreateBundle(path string, opts CreateOptions) error
func (cli *AgentBundleCLI) ValidateBundle(path string) (*ValidationSummary, error)
func (cli *AgentBundleCLI) ExportAgent(agentID int64, path string, opts ExportOptions) error
```

**New CLI Commands:**
```bash
stn agent create-bundle <path>     # Create new agent bundle
stn agent export <agent-id> <path> # Export agent to bundle 
stn agent validate <bundle-path>   # Validate agent bundle
stn agent bundle <bundle-path>     # Package agent bundle
```

**Integration with existing:**
- Extend current `stn agent export` with `--bundle` flag
- Add agent bundle commands to main CLI structure
- Reuse styling and output patterns from template bundle system
- Maintain backward compatibility with existing agent export

**Test Coverage Target:** 80%

### **Phase 1 Deliverables**
- [ ] Complete agent bundle data structures
- [ ] Agent bundle creator with export functionality
- [ ] Comprehensive validator with tool dependency checking
- [ ] CLI integration with styled output
- [ ] 85%+ test coverage across all components
- [ ] Documentation and examples
- [ ] Integration with existing agent export/import

### **Phase 1 Success Criteria**
- [ ] Create agent bundle from existing agent in < 30 seconds
- [ ] Export complex agent with 10+ tools successfully
- [ ] Validation identifies all common configuration errors
- [ ] CLI commands follow Station patterns and styling
- [ ] All tests pass with coverage targets met
- [ ] Documentation enables first-time users to succeed

## 📋 **Phase 2: Installation & Duplication (V2)**

### **Duration:** 2-3 weeks

### **Core Components**

#### **1. Dependency Resolver**
**File:** `pkg/agent-bundle/dependency/resolver.go`

```go
type Resolver struct {
    bundleManager   bundle.BundleManager      // Template bundle manager
    mcpService      mcp.FileConfigService    // MCP configuration service
    registries      []bundle.BundleRegistry  // Registry access
}

func (r *Resolver) Resolve(dependencies []MCPBundleDependency, environment string) (*Resolution, error) {
    // Analyze required MCP bundles
    // Check availability in registries
    // Resolve version constraints
    // Plan installation order
}

func (r *Resolver) InstallMCPBundles(bundles []MCPBundleRef, environment string) error {
    // Install MCP template bundles
    // Configure MCP servers in environment  
    // Trigger tool discovery
    // Validate tool availability
}
```

**Key Features:**
- Resolve MCP bundle dependencies with version constraints
- Install missing MCP template bundles automatically
- Handle dependency conflicts and alternatives
- Environment-specific installation and configuration

**Test Coverage Target:** 85%

#### **2. Agent Bundle Manager**
**File:** `pkg/agent-bundle/manager/manager.go`

```go
type Manager struct {
    resolver        *dependency.Resolver
    agentService    services.AgentService
    bundleManager   bundle.BundleManager
}

func (m *Manager) Install(bundlePath string, environment string, vars map[string]interface{}) (*AgentInstallResult, error) {
    // Extract and validate bundle
    // Resolve MCP dependencies
    // Install MCP bundles if missing
    // Process agent template with variables
    // Create agent in database
    // Assign tools to agent
}

func (m *Manager) Duplicate(agentID int64, targetEnv string, opts DuplicateOptions) (*Agent, error) {
    // Export existing agent to temporary bundle
    // Process variables (interactive/file/provided)
    // Install bundle in target environment
    // Create new agent instance
}
```

**Key Features:**
- Complete agent bundle installation with dependency resolution
- Cross-environment agent duplication with variable substitution
- Interactive variable configuration for duplication
- Rollback capability on installation failure

**Test Coverage Target:** 80%

#### **3. Enhanced CLI Commands**
```bash
stn agent install <bundle-path> --environment <env>    # Install agent bundle
stn agent duplicate <agent-id> --target-env <env>     # Duplicate agent
stn agent update <agent-id> --bundle <path>           # Update from bundle  
stn agent remove <agent-id> --cleanup-deps            # Remove with cleanup
```

### **Phase 2 Deliverables**
- [ ] Dependency resolver with MCP bundle integration
- [ ] Agent bundle manager with installation logic
- [ ] Agent duplication across environments
- [ ] Enhanced CLI with installation commands
- [ ] Database integration for bundle tracking
- [ ] Error handling and rollback mechanisms

### **Phase 2 Success Criteria**
- [ ] Install agent bundle with 5 MCP dependencies in < 60 seconds
- [ ] Successfully duplicate agent across environments with variables
- [ ] Handle dependency conflicts gracefully with clear messages
- [ ] Rollback works correctly on installation failures
- [ ] Interactive variable configuration is intuitive

## 📋 **Phase 3: Registry & Publishing (V3)**

### **Duration:** 1-2 weeks

### **Integration with Template Bundle Registry**

Agent bundles will reuse the existing template bundle registry infrastructure with agent-specific extensions:

```go
// Extend existing registry interfaces for agent bundles
type AgentBundleRegistry interface {
    bundle.BundleRegistry  // Inherit template bundle registry
    
    // Agent-specific methods
    ListAgentBundles(filter AgentBundleFilter) ([]AgentBundleRef, error)
    GetAgentBundle(name, version string) (*AgentBundle, error)
    PublishAgentBundle(bundle *AgentBundle) error
}
```

**Key Features:**
- Publish agent bundles to existing registries
- Agent-specific discovery and filtering
- Version management and constraints
- Registry configuration reuse

**New CLI Commands:**
```bash
stn agent publish <bundle-path> --registry <name>     # Publish agent bundle
stn agent list --search <term> --registry <name>     # List available agents
stn agent search <query>                              # Search agent bundles
```

### **Phase 3 Deliverables**
- [ ] Agent bundle registry integration
- [ ] Publishing workflow with validation
- [ ] Discovery and search functionality  
- [ ] Version constraint resolution
- [ ] Registry CLI commands

## 🧪 **Comprehensive Testing Strategy**

### **Unit Testing Standards**

**Test File Structure:**
```go
// Example: pkg/agent-bundle/creator/creator_test.go
func TestCreator_Create(t *testing.T) {
    tests := []struct {
        name    string
        opts    CreateOptions
        want    error
        check   func(t *testing.T, bundlePath string)
    }{
        {
            name: "valid minimal bundle",
            opts: CreateOptions{Name: "test", Author: "tester"},
            check: func(t *testing.T, path string) {
                // Verify manifest exists and is valid
                // Check agent.json structure
                // Validate tools.json
                // Verify schema file
            },
        },
        {
            name: "export from existing agent",
            opts: CreateOptions{FromAgent: &agentID},
            check: func(t *testing.T, path string) {
                // Verify tool mappings are correct
                // Check MCP dependencies extracted
                // Validate template variables
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

**Coverage Targets by Component:**
- **Types & Interfaces:** 95% (critical foundation)
- **Creator:** 85% (complex logic with many edge cases)
- **Validator:** 85% (comprehensive validation scenarios)  
- **Manager:** 80% (integration heavy, mocked dependencies)
- **Dependency Resolver:** 85% (complex dependency logic)
- **CLI:** 75% (UI logic, harder to test comprehensively)

### **Integration Testing**

**End-to-End Workflows:**
```go
func TestE2E_AgentBundleWorkflow(t *testing.T) {
    // Setup: Create test environment with MCP bundles
    // Step 1: Export existing agent to bundle
    // Step 2: Validate bundle
    // Step 3: Package bundle
    // Step 4: Install bundle in new environment
    // Step 5: Verify agent works with tools
    // Cleanup: Remove test environments
}
```

**Integration Scenarios:**
- Agent export → bundle creation → installation cycle
- MCP bundle dependency resolution and installation
- Cross-environment duplication with variable substitution
- Registry publishing and discovery
- Error handling and rollback scenarios

### **Performance Testing**

**Benchmarks:**
```go
func BenchmarkCreator_ExportLargeAgent(b *testing.B) {
    // Agent with 20+ tools, 5+ MCP dependencies
    for i := 0; i < b.N; i++ {
        creator.ExportFromAgent(largeAgentID, path, opts)
    }
}

func BenchmarkManager_InstallBundle(b *testing.B) {
    // Bundle with multiple MCP dependencies
    for i := 0; i < b.N; i++ {
        manager.Install(bundlePath, env, vars)
    }
}
```

**Performance Targets:**
- Bundle creation: < 30 seconds for complex agents
- Bundle validation: < 5 seconds
- Bundle installation: < 60 seconds with dependencies  
- Agent duplication: < 30 seconds

### **Test Data Management**

**Test Fixtures:**
```
testdata/
├── bundles/
│   ├── valid-minimal/           # Minimal valid agent bundle
│   ├── complex-agent/           # Agent with many tools and deps
│   ├── invalid-manifest/        # Bundle with bad manifest
│   └── missing-dependencies/    # Bundle with missing MCP deps
├── agents/
│   ├── simple-agent.json       # Basic agent configuration
│   └── complex-agent.json      # Agent with tools and schedule
└── mcp-bundles/
    ├── filesystem-tools.tar.gz # Test MCP bundle
    └── aws-tools.tar.gz        # Test MCP bundle
```

## 🔧 **Development Workflow**

### **Branch Strategy**
```bash
# Create feature branch for agent templates
git checkout -b feature/agent-template-system

# Phase-based development
git checkout -b phase1/core-agent-bundles
git checkout -b phase2/installation-duplication  
git checkout -b phase3/registry-publishing
```

### **Development Order**
1. **Data Types & Interfaces** - Foundation for all components
2. **Creator** - Core bundle creation functionality
3. **Validator** - Comprehensive validation before other components
4. **CLI Integration** - Basic commands for Phase 1
5. **Dependency Resolver** - Required for installation
6. **Manager** - Installation and duplication logic
7. **Registry Integration** - Publishing and discovery

### **Code Quality Gates**
- [ ] All tests pass with coverage targets met
- [ ] No linting errors or warnings
- [ ] Performance benchmarks meet targets
- [ ] Documentation complete for public APIs
- [ ] Integration tests pass with real dependencies
- [ ] Security review completed for bundle handling

### **Release Process**
1. **Phase 1**: Core functionality with CLI commands
2. **Phase 2**: Installation and duplication features
3. **Phase 3**: Registry integration and publishing
4. **Documentation**: Comprehensive guides and examples
5. **Announcement**: Internal rollout and feedback collection

## 🎯 **Success Metrics**

### **Technical Metrics**
- **Test Coverage**: 85%+ across all components
- **Performance**: All benchmarks meet targets
- **Reliability**: 99%+ successful installations for valid bundles
- **Error Handling**: Clear, actionable error messages for all failure modes

### **User Experience Metrics**
- **Time to First Bundle**: < 10 minutes from agent to packaged bundle
- **Installation Success**: 95%+ success rate for bundle installations
- **Documentation Quality**: Users can complete workflows without support
- **Error Resolution**: Users can resolve 80%+ of errors independently

This implementation plan provides a comprehensive roadmap for building the Agent Template System with high quality, extensive testing, and seamless integration with existing Station systems.