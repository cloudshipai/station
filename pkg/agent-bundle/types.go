package agent_bundle

import (
	"time"
)

// AgentBundleManifest defines the metadata and structure of an agent bundle
type AgentBundleManifest struct {
	Name              string                    `json:"name" yaml:"name"`
	Version           string                    `json:"version" yaml:"version"`
	Description       string                    `json:"description" yaml:"description"`
	Author            string                    `json:"author" yaml:"author"`
	
	// Agent-specific metadata
	AgentType         string                    `json:"agent_type" yaml:"agent_type"`                   // "task", "scheduled", "interactive"
	SupportedSchedules []string                `json:"supported_schedules,omitempty" yaml:"supported_schedules,omitempty"` // cron patterns, intervals
	RequiredModel     *ModelRequirement        `json:"required_model,omitempty" yaml:"required_model,omitempty"`      // model constraints
	
	// Dependencies
	MCPBundles        []MCPBundleDependency    `json:"mcp_bundles" yaml:"mcp_bundles"`
	RequiredTools     []ToolRequirement        `json:"required_tools" yaml:"required_tools"`
	
	// Variables
	RequiredVariables map[string]VariableSpec  `json:"required_variables,omitempty" yaml:"required_variables,omitempty"`
	
	// Metadata
	Tags              []string                  `json:"tags,omitempty" yaml:"tags,omitempty"`
	License           string                   `json:"license,omitempty" yaml:"license,omitempty"`
	Homepage          string                   `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	StationVersion    string                   `json:"station_version" yaml:"station_version"`
	CreatedAt         time.Time                `json:"created_at" yaml:"created_at"`
}

// AgentTemplateConfig represents the agent configuration with template variables
type AgentTemplateConfig struct {
	Name              string                   `json:"name" yaml:"name"`
	Description       string                   `json:"description" yaml:"description"`
	Prompt            string                   `json:"prompt" yaml:"prompt"`
	MaxSteps          int64                    `json:"max_steps" yaml:"max_steps"`
	
	// Templated fields with variables
	ScheduleTemplate  *string                  `json:"schedule_template,omitempty" yaml:"schedule_template,omitempty"`    // "0 {{ .HOUR }} * * *"
	NameTemplate      string                   `json:"name_template" yaml:"name_template"`        // "{{ .ENVIRONMENT }}-analyzer"
	PromptTemplate    string                   `json:"prompt_template" yaml:"prompt_template"`      // prompt with {{ .CONTEXT }} vars
	
	// Environment and deployment
	SupportedEnvs     []string                 `json:"supported_environments,omitempty" yaml:"supported_environments,omitempty"`
	DefaultVars       map[string]interface{}   `json:"default_variables,omitempty" yaml:"default_variables,omitempty"`
	
	// Metadata  
	Version           string                   `json:"version" yaml:"version"`
	CreatedAt         time.Time                `json:"created_at" yaml:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at" yaml:"updated_at"`
}

// MCPBundleDependency represents a dependency on an MCP template bundle
type MCPBundleDependency struct {
	Name            string `json:"name" yaml:"name"`
	Version         string `json:"version" yaml:"version"`         // semver constraint
	Source          string `json:"source" yaml:"source"`          // registry, local, url
	Required        bool   `json:"required" yaml:"required"`
	Description     string `json:"description,omitempty" yaml:"description,omitempty"`
}

// ToolRequirement represents a tool that the agent requires
type ToolRequirement struct {
	Name         string   `json:"name" yaml:"name"`
	ServerName   string   `json:"server_name" yaml:"server_name"`
	MCPBundle    string   `json:"mcp_bundle" yaml:"mcp_bundle"`      // which bundle provides this tool
	Required     bool     `json:"required" yaml:"required"`
	Alternatives []string `json:"alternatives,omitempty" yaml:"alternatives,omitempty"`    // alternative tool names
}

// ModelRequirement represents constraints on the AI model
type ModelRequirement struct {
	Provider      string `json:"provider,omitempty" yaml:"provider,omitempty"`           // "openai", "anthropic", etc.
	MinContextSize int64 `json:"min_context_size,omitempty" yaml:"min_context_size,omitempty"`
	RequiresTools  bool  `json:"requires_tools,omitempty" yaml:"requires_tools,omitempty"`
	Models        []string `json:"models,omitempty" yaml:"models,omitempty"`            // specific model IDs
}

// VariableSpec defines the specification for a template variable
type VariableSpec struct {
	Type        string      `json:"type" yaml:"type"`               // "string", "number", "boolean", "secret"
	Description string      `json:"description" yaml:"description"`
	Required    bool        `json:"required" yaml:"required"`
	Default     interface{} `json:"default,omitempty" yaml:"default,omitempty"`
	Pattern     string      `json:"pattern,omitempty" yaml:"pattern,omitempty"`     // regex pattern for validation
	Enum        []string    `json:"enum,omitempty" yaml:"enum,omitempty"`         // allowed values
	Sensitive   bool        `json:"sensitive,omitempty" yaml:"sensitive,omitempty"` // should be encrypted
}

// AgentBundle represents a complete agent bundle with all its components
type AgentBundle struct {
	Manifest        *AgentBundleManifest  `json:"manifest"`
	AgentConfig     *AgentTemplateConfig  `json:"agent_config"`
	ToolMappings    []ToolRequirement     `json:"tool_mappings"`
	Path            string                `json:"path"`
	Size            int64                 `json:"size"`
	PackagedPath    string                `json:"packaged_path,omitempty"`
}

// CreateOptions defines options for creating agent bundles
type CreateOptions struct {
	Name         string
	Author       string
	Description  string
	FromAgent    *int64                    // Export from existing agent
	Environment  string                    // Source environment for export
	IncludeMCP   bool                     // Include MCP bundle dependencies
	Variables    map[string]interface{}    // Default variable values
	AgentType    string                   // "task", "scheduled", "interactive"
	Tags         []string                 // Bundle tags
}

// ExportOptions defines options for exporting agents to bundles
type ExportOptions struct {
	IncludeDependencies bool
	IncludeExamples    bool
	VariableAnalysis   bool
	OutputFormat       string // "bundle", "tar.gz"
}

// ValidationResult represents the result of bundle validation
type ValidationResult struct {
	Valid            bool                    `json:"valid"`
	Errors           []ValidationError       `json:"errors,omitempty"`
	Warnings         []ValidationWarning     `json:"warnings,omitempty"`
	ManifestValid    bool                   `json:"manifest_valid"`
	AgentConfigValid bool                   `json:"agent_config_valid"`
	ToolsValid       bool                   `json:"tools_valid"`
	DependenciesValid bool                  `json:"dependencies_valid"`
	VariablesValid   bool                   `json:"variables_valid"`
	Statistics       ValidationStatistics   `json:"statistics"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Field       string `json:"field,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Type        string `json:"type"`
	Message     string `json:"message"`
	Field       string `json:"field,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ValidationStatistics provides statistics about the bundle
type ValidationStatistics struct {
	TotalVariables    int `json:"total_variables"`
	RequiredVariables int `json:"required_variables"`
	OptionalVariables int `json:"optional_variables"`
	MCPDependencies   int `json:"mcp_dependencies"`
	RequiredTools     int `json:"required_tools"`
	OptionalTools     int `json:"optional_tools"`
}

// PackageResult represents the result of packaging an agent bundle
type PackageResult struct {
	Success     bool   `json:"success"`
	OutputPath  string `json:"output_path"`
	Size        int64  `json:"size"`
	Checksum    string `json:"checksum"`
	Error       string `json:"error,omitempty"`
}

// InstallResult represents the result of installing an agent bundle
type InstallResult struct {
	Success        bool     `json:"success"`
	AgentID        int64    `json:"agent_id"`
	AgentName      string   `json:"agent_name"`
	Environment    string   `json:"environment"`
	ToolsInstalled int      `json:"tools_installed"`
	MCPBundles     []string `json:"mcp_bundles"`
	Variables      map[string]interface{} `json:"variables"`
	Error          string   `json:"error,omitempty"`
}

// DependencyAnalysis represents the analysis of agent dependencies
type DependencyAnalysis struct {
	RequiredMCPBundles []MCPBundleDependency `json:"required_mcp_bundles"`
	RequiredTools      []ToolRequirement     `json:"required_tools"`
	ResolvedTools      []ResolvedTool        `json:"resolved_tools"`
	MissingTools       []string              `json:"missing_tools"`
	ConflictingTools   []ToolConflict        `json:"conflicting_tools"`
}

// ResolvedTool represents a tool that has been resolved to a specific MCP bundle
type ResolvedTool struct {
	Name       string `json:"name"`
	ServerName string `json:"server_name"`
	MCPBundle  string `json:"mcp_bundle"`
	Version    string `json:"version"`
	Source     string `json:"source"`
}

// ToolConflict represents a conflict between tool requirements
type ToolConflict struct {
	ToolName        string   `json:"tool_name"`
	ConflictingBundles []string `json:"conflicting_bundles"`
	Resolution      string   `json:"resolution"`
}

// Multi-Tenant Types (for the sub-PRD)

// AgentClientVariable represents a client-specific variable (encrypted in database)
type AgentClientVariable struct {
	ID               int64     `json:"id" db:"id"`
	TemplateAgentID  int64     `json:"template_agent_id" db:"template_agent_id"`
	ClientID         string    `json:"client_id" db:"client_id"`
	VariableName     string    `json:"variable_name" db:"variable_name"`
	EncryptedValue   string    `json:"encrypted_value" db:"encrypted_value"`  // AES-256 encrypted
	VariableType     string    `json:"variable_type" db:"variable_type"`      // "agent", "mcp", "system"
	Description      string    `json:"description" db:"description"`
	IsRequired       bool      `json:"is_required" db:"is_required"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// DuplicateOptions defines options for duplicating agents across environments
type DuplicateOptions struct {
	Name         string
	Variables    map[string]interface{}
	VariablesFile string
	Interactive  bool
	ClientID     *string // For multi-tenant templates
}

// AgentTemplateUpdate represents updates to be applied to a template and its instances
type AgentTemplateUpdate struct {
	Prompt        *string                `json:"prompt,omitempty"`
	MaxSteps      *int64                 `json:"max_steps,omitempty"`
	AddMCPBundles []string               `json:"add_mcp_bundles,omitempty"`
	RemoveMCPBundles []string            `json:"remove_mcp_bundles,omitempty"`
	AddTools      []ToolRequirement      `json:"add_tools,omitempty"`
	RemoveTools   []string               `json:"remove_tools,omitempty"`
	UpdateVariables map[string]VariableSpec `json:"update_variables,omitempty"`
	Version       string                 `json:"version"`
}