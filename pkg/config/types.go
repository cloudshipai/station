package config

import (
	"time"
)

// MCPTemplate represents a parsed MCP configuration template
type MCPTemplate struct {
	Name         string             `json:"name"`
	FilePath     string             `json:"file_path"`
	Content      string             `json:"content"`
	Variables    []TemplateVariable `json:"variables"`
	Dependencies []string           `json:"dependencies,omitempty"`
	Metadata     TemplateMetadata   `json:"metadata"`
}

// TemplateVariable defines a variable used in templates
type TemplateVariable struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Required    bool           `json:"required"`
	Default     interface{}    `json:"default,omitempty"`
	Type        VarType        `json:"type"`
	Secret      bool           `json:"secret"` // Don't log/display value
	Validation  *VarValidation `json:"validation,omitempty"`
}

// VarType defines the type of template variable
type VarType string

const (
	VarTypeString  VarType = "string"
	VarTypeNumber  VarType = "number"
	VarTypeBoolean VarType = "boolean"
	VarTypeArray   VarType = "array"
	VarTypeObject  VarType = "object"
)

// VarValidation defines validation rules for template variables
type VarValidation struct {
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
	Pattern   *string  `json:"pattern,omitempty"`
	Enum      []string `json:"enum,omitempty"`
}

// TemplateMetadata contains metadata about the template
type TemplateMetadata struct {
	Version     string            `json:"version"`
	Author      string            `json:"author,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// ParsedTemplate represents a compiled Go template
type ParsedTemplate struct {
	Name      string
	Template  interface{} // *template.Template
	Variables []TemplateVariable
}

// TemplateValidation represents validation results
type TemplateValidation struct {
	Valid        bool                `json:"valid"`
	Errors       []ValidationError   `json:"errors,omitempty"`
	Warnings     []ValidationWarning `json:"warnings,omitempty"`
	Variables    []TemplateVariable  `json:"variables"`
	Dependencies []string            `json:"dependencies,omitempty"`
}

// ValidationError represents a template validation error
type ValidationError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

// ValidationWarning represents a template validation warning
type ValidationWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Line    int    `json:"line,omitempty"`
	Column  int    `json:"column,omitempty"`
}

// TemplateInfo contains basic information about discovered templates
type TemplateInfo struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	HasVars  bool      `json:"has_vars"`
	VarsPath string    `json:"vars_path,omitempty"`
}

// ConfigInfo contains information about a configuration
type ConfigInfo struct {
	Name        string            `json:"name"`
	Type        ConfigType        `json:"type"`
	Path        string            `json:"path,omitempty"`
	Version     int64             `json:"version,omitempty"`
	Environment string            `json:"environment"`
	LastLoaded  *time.Time        `json:"last_loaded,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ConfigType defines the type of configuration
type ConfigType string

const (
	ConfigTypeFile     ConfigType = "file"
	ConfigTypeDatabase ConfigType = "database"
	ConfigTypeHybrid   ConfigType = "hybrid"
)

// VariableScope defines how variables are scoped and resolved
type VariableScope struct {
	// Global variables apply to all templates in an environment
	Global map[string]interface{} `json:"global"`

	// Template-specific variables override global ones
	TemplateSpecific map[string]map[string]interface{} `json:"template_specific"`

	// Environment variables from system env
	Environment map[string]string `json:"environment"`
}

// VariableResolutionStrategy defines how overlapping variables are handled
type VariableResolutionStrategy string

const (
	// Template-specific variables take precedence over global
	StrategyTemplateFirst VariableResolutionStrategy = "template_first"

	// Global variables take precedence over template-specific
	StrategyGlobalFirst VariableResolutionStrategy = "global_first"

	// Require explicit namespacing (GitHub_ApiKey vs AWS_ApiKey)
	StrategyNamespaced VariableResolutionStrategy = "namespaced"

	// Merge strategies with conflict resolution
	StrategyMergeWarn  VariableResolutionStrategy = "merge_warn"
	StrategyMergeError VariableResolutionStrategy = "merge_error"
)

// VariableConfig defines how variables are managed for an environment
type VariableConfig struct {
	Strategy         VariableResolutionStrategy `json:"strategy"`
	GlobalVarsFile   string                     `json:"global_vars_file"`
	TemplateVarsDir  string                     `json:"template_vars_dir"`
	AllowEnvOverride bool                       `json:"allow_env_override"`
	RequireNamespace bool                       `json:"require_namespace"`
}

// RenderContext provides context for template rendering
type RenderContext struct {
	TemplateName string                 `json:"template_name"`
	Environment  string                 `json:"environment"`
	Variables    map[string]interface{} `json:"variables"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Functions    map[string]interface{} `json:"-"` // Template functions
}

// FileConfigOptions configures the file-based config system
type FileConfigOptions struct {
	ConfigDir      string                     `json:"config_dir"`
	VariablesDir   string                     `json:"variables_dir"`
	Strategy       VariableResolutionStrategy `json:"strategy"`
	FileSystem     FileSystem                 `json:"-"`
	AutoCreate     bool                       `json:"auto_create"`
	BackupOnChange bool                       `json:"backup_on_change"`
	ValidateOnLoad bool                       `json:"validate_on_load"`
}
