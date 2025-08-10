# Agent Template System - Product Requirements Document

## 🎯 **Vision Statement**

Enable users to package, share, and deploy complete AI agent configurations with their MCP dependencies as reusable, declarative templates. This system builds on the existing Template Bundle System to provide agent-specific workflows while maintaining seamless integration with MCP server configurations.

## 📋 **Executive Summary**

The Agent Template System extends Station's Template Bundle System to support complete agent configurations, including their MCP tool dependencies. Users can export agents as declarative templates, duplicate agents across environments with new variables, and publish agent bundles to registries for team sharing.

### **Key Problems Solved:**
- **Agent Portability**: Move successful agents between environments/projects
- **Team Collaboration**: Share proven agent configurations across teams  
- **Configuration Drift**: Maintain consistent agent setups through declarative templates
- **Environment Scaling**: Rapidly deploy agents to new environments with different variables
- **Dependency Management**: Ensure agents have their required MCP tools and configurations

## 🏗️ **System Architecture Overview**

### **Agent Template Structure**
```
my-agent-bundle/
├── manifest.json              # Agent bundle metadata & dependencies
├── agent.json                 # Agent configuration with {{ .VAR }} placeholders
├── tools.json                 # Required tool mappings and MCP server references  
├── variables.schema.json      # JSON schema for agent-specific variables
├── dependencies/              # MCP template bundle dependencies
│   ├── aws-tools-bundle.tar.gz   # Required MCP bundle
│   └── github-tools-bundle.tar.gz # Required MCP bundle  
├── README.md                  # Usage documentation
└── examples/                  # Example configurations
    ├── development.yml        # Dev environment variables
    ├── staging.yml           # Staging environment variables
    └── production.yml        # Production environment variables
```

### **Relationship Architecture**

```mermaid
graph TD
    A[Agent Bundle] --> B[Agent Config]
    A --> C[Tool Requirements]
    A --> D[MCP Dependencies]
    A --> E[Variable Schema]
    
    B --> F[{{ .AGENT_NAME }}]
    B --> G[{{ .MAX_STEPS }}] 
    B --> H[{{ .SCHEDULE }}]
    
    C --> I[Required Tool Names]
    C --> J[MCP Server Mappings]
    
    D --> K[MCP Bundle References]
    D --> L[Template Bundle Dependencies]
    
    E --> M[Agent Variables]
    E --> N[Environment Variables]
    E --> O[MCP Variables]
```

## 🎨 **User Experience Workflows**

### **1. Export Existing Agent**
```bash
# Export agent with all dependencies
stn agent export my-analyzer --environment production --output ./my-analyzer-bundle
✅ Exported agent: my-analyzer
📦 MCP dependencies: 3 bundles
🔧 Tools mapped: 12 tools
📁 Bundle created: ./my-analyzer-bundle/

# Package for distribution  
stn template bundle ./my-analyzer-bundle
✅ Agent bundle packaged: my-analyzer-v1.0.0.tar.gz (15.2 MB)
```

### **2. Install Agent Bundle**  
```bash
# Install agent bundle with dependencies
stn agent install my-analyzer-v1.0.0.tar.gz --environment staging

🔍 Analyzing bundle dependencies...
📦 Installing MCP bundles: aws-tools, github-tools, filesystem-tools
🔧 Discovering tools from MCP servers...
🤖 Creating agent: my-analyzer
✅ Agent bundle installed successfully

Agent ID: 42
Tools available: 12
Environment: staging
```

### **3. Duplicate Agent Across Environments**
```bash
# Duplicate existing agent to new environment with different variables
stn agent duplicate 5 --target-environment production --name "Production Analyzer"

📋 Select variable configuration:
   1) Use existing variables
   2) Prompt for new variables  
   3) Load from file: production.yml

Choice: 3
🔧 Loading variables from production.yml...
🤖 Creating agent in production environment...
✅ Agent duplicated successfully

Original Agent ID: 5 (staging)
New Agent ID: 78 (production)
```

### **4. Create Agent Bundle from Scratch**
```bash
# Create new agent bundle template
stn agent create-bundle ./my-new-agent --name "Log Analyzer" --author "DevOps Team"

✅ Agent bundle scaffolded: ./my-new-agent/
📝 Next steps:
   1. Edit agent.json with your agent configuration
   2. Update tools.json with required tool mappings
   3. Add MCP bundle dependencies to dependencies/
   4. Update variables.schema.json for agent-specific variables
   5. Run 'stn agent validate ./my-new-agent' to test your bundle
```

### **5. Publish Agent Bundle**
```bash  
# Publish to registry
stn agent publish ./my-agent-bundle --registry company-private

🔍 Validating agent bundle...
📦 Packaging bundle with dependencies...
🚀 Publishing to registry: company-private
✅ Published: log-analyzer@v1.2.0

Download URL: https://registry.company.com/agents/log-analyzer/v1.2.0
Bundle size: 15.2 MB
Dependencies: 3 MCP bundles
```

## 🔧 **Core Components & Features**

### **1. Agent Bundle Creator**
- **Scaffolding**: Generate bundle structure from existing agents or templates
- **Dependency Analysis**: Automatically detect required MCP bundles and tools
- **Variable Extraction**: Extract template variables from agent configurations  
- **Documentation**: Auto-generate README with usage examples

**Interface:**
```go
type AgentBundleCreator interface {
    Create(path string, opts AgentCreateOptions) error
    ExportFromAgent(agentID int64, path string, opts ExportOptions) error
    AnalyzeDependencies(agentID int64) (*DependencyAnalysis, error)
}

type AgentCreateOptions struct {
    Name         string
    Author       string
    Description  string
    FromAgent    *int64                    // Export from existing agent
    Environment  string                    // Source environment for export
    IncludeMCP   bool                     // Include MCP bundle dependencies
    Variables    map[string]interface{}    // Default variable values
}
```

### **2. Agent Bundle Validator**
- **Configuration Validation**: Validate agent.json structure and required fields
- **Tool Validation**: Ensure all required tools exist in referenced MCP bundles
- **Dependency Validation**: Verify MCP bundle dependencies are valid and accessible
- **Variable Validation**: Check variable schema consistency across agent and MCP configs

**Interface:**
```go
type AgentBundleValidator interface {
    Validate(bundlePath string) (*AgentValidationResult, error)
    ValidateAgent(agentConfig AgentConfig) error
    ValidateToolMappings(tools []ToolMapping, mcpBundles []MCPBundle) error
    ValidateDependencies(dependencies []BundleDependency) error
}
```

### **3. Agent Bundle Manager**
- **Installation**: Install agent bundles with MCP dependency resolution
- **Duplication**: Clone agents across environments with variable substitution
- **Updates**: Update installed agent bundles while preserving local customizations
- **Removal**: Clean removal of agents and orphaned dependencies

**Interface:**
```go
type AgentBundleManager interface {
    Install(bundlePath string, environment string, vars map[string]interface{}) (*AgentInstallResult, error)
    Duplicate(agentID int64, targetEnv string, opts DuplicateOptions) (*Agent, error)
    Update(bundlePath string, agentID int64, opts UpdateOptions) error
    Remove(agentID int64, opts RemoveOptions) error
    List() ([]AgentBundle, error)
}

type DuplicateOptions struct {
    Name         string
    Variables    map[string]interface{}
    VariablesFile string
    Interactive  bool
}
```

### **4. Dependency Resolution System**
- **MCP Bundle Resolution**: Automatically install required MCP template bundles
- **Tool Mapping**: Map agent tool requirements to MCP server tools
- **Version Compatibility**: Handle MCP bundle version constraints
- **Environment Isolation**: Ensure agents work in target environments

**Interface:**  
```go
type DependencyResolver interface {
    Resolve(dependencies []BundleDependency, environment string) (*Resolution, error)
    InstallMCPBundles(bundles []MCPBundleRef, environment string) error
    ValidateToolAvailability(tools []ToolRequirement, environment string) error
}
```

## 📊 **Data Models**

### **Agent Bundle Manifest**
```go
type AgentBundleManifest struct {
    Name              string                    `json:"name"`
    Version           string                    `json:"version"`
    Description       string                    `json:"description"`
    Author            string                    `json:"author"`
    
    // Agent-specific metadata
    AgentType         string                    `json:"agent_type"`          // "task", "scheduled", "interactive"
    SupportedSchedules []string                `json:"supported_schedules"`  // cron patterns, intervals
    RequiredModel     *ModelRequirement        `json:"required_model"`      // model constraints
    
    // Dependencies
    MCPBundles        []MCPBundleDependency    `json:"mcp_bundles"`
    RequiredTools     []ToolRequirement        `json:"required_tools"`
    
    // Variables
    RequiredVariables map[string]VariableSpec  `json:"required_variables"`
    
    // Metadata
    Tags              []string                  `json:"tags"`
    License           string                   `json:"license"`
    Homepage          string                   `json:"homepage"`
    StationVersion    string                   `json:"station_version"`
    CreatedAt         time.Time                `json:"created_at"`
}

type MCPBundleDependency struct {
    Name            string `json:"name"`
    Version         string `json:"version"`         // semver constraint
    Source          string `json:"source"`          // registry, local, url
    Required        bool   `json:"required"`
    Description     string `json:"description"`
}

type ToolRequirement struct {
    Name            string   `json:"name"`
    ServerName      string   `json:"server_name"`
    MCPBundle       string   `json:"mcp_bundle"`      // which bundle provides this tool
    Required        bool     `json:"required"`
    Alternatives    []string `json:"alternatives"`    // alternative tool names
}
```

### **Agent Template Configuration**
```go
type AgentTemplateConfig struct {
    Name              string            `json:"name"`
    Description       string            `json:"description"`
    Prompt            string            `json:"prompt"`
    MaxSteps          int64             `json:"max_steps"`
    
    // Templated fields with variables
    ScheduleTemplate  *string           `json:"schedule_template"`    // "0 {{ .HOUR }} * * *"
    NameTemplate      string            `json:"name_template"`        // "{{ .ENVIRONMENT }}-analyzer"
    PromptTemplate    string            `json:"prompt_template"`      // prompt with {{ .CONTEXT }} vars
    
    // Environment and deployment
    SupportedEnvs     []string          `json:"supported_environments"`
    DefaultVars       map[string]interface{} `json:"default_variables"`
    
    // Metadata  
    Version           string            `json:"version"`
    CreatedAt         time.Time         `json:"created_at"`
    UpdatedAt         time.Time         `json:"updated_at"`
}
```

## 🔄 **Integration with Existing Systems**

### **Template Bundle System Integration**
- **Dependency Management**: Agent bundles reference MCP template bundles as dependencies
- **Registry Sharing**: Agent bundles published to same registry system as MCP bundles
- **Variable Resolution**: Hierarchical variable resolution (agent vars → MCP vars → environment vars)
- **Validation Pipeline**: Reuse template validation infrastructure for agent bundles

### **Agent Management Integration**  
- **Enhanced Export**: Extend existing `stn agent export` to create full bundles
- **Import Evolution**: Evolve `stn agent import` to handle bundle dependencies
- **CLI Consistency**: Follow same patterns as `stn template` commands
- **Database Integration**: Maintain existing agent storage with bundle metadata

### **MCP System Integration**
- **Tool Discovery**: Leverage existing `stn mcp sync` for tool discovery after bundle install
- **Configuration Management**: Use file-based MCP configs for dependency installation
- **Environment Isolation**: Respect environment boundaries for tool access

## 🚀 **Implementation Phases**

### **Phase 1: Core Agent Bundle System (V1)**
**Duration: 2-3 weeks**

**Features:**
- Agent bundle creation from existing agents
- Agent configuration templating with variables
- Tool requirement mapping and validation
- Basic dependency resolution for MCP bundles
- CLI commands: `create-bundle`, `validate`, `bundle`

**Deliverables:**
- `pkg/agent-bundle/` package with core interfaces
- Agent bundle creator with 80%+ test coverage
- Agent bundle validator with comprehensive checks
- CLI integration following template bundle patterns

### **Phase 2: Installation & Duplication (V2)**  
**Duration: 2-3 weeks**

**Features:**
- Agent bundle installation with MCP dependency resolution  
- Agent duplication across environments
- Interactive variable configuration
- Update and removal workflows
- CLI commands: `install`, `duplicate`, `update`, `remove`

**Deliverables:**
- Agent bundle manager with installation logic
- Dependency resolver for MCP bundles
- Environment duplication with variable substitution
- Database integration for bundle tracking

### **Phase 3: Registry & Publishing (V3)**
**Duration: 1-2 weeks**

**Features:**
- Agent bundle publishing to registries
- Bundle discovery and listing
- Version management and constraints
- Registry configuration management
- CLI commands: `publish`, `list`, `registry`

**Deliverables:**
- Registry integration for agent bundles
- Publishing workflow with validation
- Discovery and search functionality
- Version constraint resolution

### **Phase 4: Advanced Features (V4)**
**Duration: 2-3 weeks**

**Features:**
- Bundle composition (combining multiple agent bundles)
- Environment-specific optimizations
- Bundle signing and security
- GitOps integration for CI/CD
- Advanced scheduling and trigger patterns

**Deliverables:**
- Composite bundle support
- Security and signing infrastructure  
- GitOps workflow examples
- Advanced scheduling features

## 🎯 **Success Metrics**

### **Developer Experience**
- **Bundle Creation**: < 5 minutes from agent to published bundle
- **Installation Speed**: < 30 seconds for typical agent bundle with dependencies
- **Error Resolution**: Clear error messages with actionable suggestions
- **Documentation**: Comprehensive examples and tutorials

### **System Reliability**
- **Test Coverage**: 85%+ across all agent bundle components
- **Validation Accuracy**: 99%+ successful installations for valid bundles
- **Dependency Resolution**: 100% success rate for compatible MCP bundles
- **Environment Isolation**: Zero cross-environment tool leakage

### **Adoption Metrics**
- **Bundle Creation**: 50+ agent bundles created in first month
- **Community Sharing**: 10+ publicly shared agent bundles
- **Cross-Environment Usage**: 80% of agents deployed to multiple environments
- **Team Collaboration**: 5+ teams sharing agent bundles internally

## ⚡ **Technical Requirements**

### **Performance**
- **Bundle Creation**: < 10 seconds for complex agents with multiple MCP dependencies
- **Installation**: < 60 seconds including MCP bundle installation and tool discovery
- **Validation**: < 5 seconds for typical agent bundle
- **Registry Operations**: < 3 seconds for bundle listing and search

### **Scalability**  
- **Bundle Size**: Support up to 100MB agent bundles (including MCP dependencies)
- **Dependencies**: Support up to 20 MCP bundle dependencies per agent
- **Concurrent Operations**: Handle 10+ simultaneous bundle operations
- **Registry Scale**: Support 1000+ agent bundles per registry

### **Security**
- **Input Validation**: Comprehensive validation of all user inputs and bundle contents
- **Path Security**: Protection against path traversal in bundle extraction
- **Variable Security**: Safe template rendering without code injection
- **Registry Security**: Authentication and authorization for private registries

## 🧪 **Testing Strategy**

### **Unit Testing (Target: 85%+ Coverage)**
- Agent bundle creator with mock file systems
- Validator with comprehensive test cases for valid/invalid configurations
- Manager with database mocking for installation/duplication workflows  
- Dependency resolver with various MCP bundle scenarios

### **Integration Testing**
- End-to-end agent bundle workflows (create → package → install)
- MCP bundle integration with actual template bundles
- Cross-environment agent duplication with variable substitution
- Registry integration with HTTP mock servers

### **Performance Testing**
- Bundle creation performance with large agents and many tools
- Installation performance with multiple MCP dependencies
- Concurrent operation handling
- Memory usage during bundle processing

### **User Acceptance Testing**
- Developer workflow testing with real-world scenarios
- Team collaboration workflows
- Error handling and recovery scenarios
- Documentation and example validation

## 🔮 **Future Considerations**

### **Advanced Agent Features**
- **Multi-Agent Bundles**: Bundles containing multiple coordinated agents
- **Agent Orchestration**: Workflow patterns between related agents
- **Dynamic Tool Discovery**: Runtime tool discovery and adaptation
- **Agent Marketplace**: Public registry with ratings and reviews

### **Enterprise Features**
- **RBAC Integration**: Role-based access control for agent bundles
- **Audit Logging**: Comprehensive tracking of bundle operations
- **Enterprise Registry**: Private, scalable registry infrastructure
- **Compliance**: SOC2, HIPAA compliance for sensitive agent bundles

### **AI/ML Features**
- **Smart Recommendations**: Suggest MCP bundles based on agent requirements
- **Performance Optimization**: AI-driven agent configuration optimization
- **Anomaly Detection**: Detect unusual agent behavior after bundle updates
- **Auto-Documentation**: AI-generated documentation for agent bundles

## ✅ **Definition of Done**

### **Phase 1 (V1) Complete When:**
- [ ] All agent bundle creator tests pass with 85%+ coverage
- [ ] Agent bundle validator handles all edge cases with clear error messages
- [ ] CLI commands integrated and working with styled output  
- [ ] Documentation complete with examples and tutorials
- [ ] End-to-end workflow tested: export agent → create bundle → validate

### **Phase 2 (V2) Complete When:**
- [ ] Agent bundle installation works with MCP dependency resolution
- [ ] Agent duplication across environments with variable substitution
- [ ] All integration tests pass with real MCP bundles
- [ ] Performance requirements met for installation and duplication
- [ ] Error handling and rollback scenarios tested

### **Phase 3 (V3) Complete When:**  
- [ ] Agent bundles can be published to registries
- [ ] Bundle discovery and search functionality working
- [ ] Version management and constraints properly handled
- [ ] Registry integration tests pass with mock and real servers
- [ ] Security validation complete for all registry operations

This PRD provides a comprehensive roadmap for implementing the Agent Template System as a natural extension of Station's existing Template Bundle System, ensuring seamless integration while providing powerful new capabilities for agent management and sharing.