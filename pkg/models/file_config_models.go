package models

import (
	"time"
)

// MCPToolWithFileConfig extends MCPTool with file config information
type MCPToolWithFileConfig struct {
	MCPTool
	ServerName   string     `json:"server_name" db:"server_name"`
	FileConfigID *int64     `json:"file_config_id,omitempty" db:"file_config_id"`
	ConfigName   string     `json:"config_name" db:"config_name"`
	TemplatePath string     `json:"template_path" db:"template_path"`
	LastLoaded   *time.Time `json:"last_loaded,omitempty" db:"last_loaded"`
}

// FileConfigStats represents statistics about file-based configs
type FileConfigStats struct {
	TotalConfigs   int        `json:"total_configs"`
	TotalTemplates int        `json:"total_templates"`
	TotalTools     int        `json:"total_tools"`
	StaleConfigs   int        `json:"stale_configs"`  // Templates changed since last load
	OrphanedTools  int        `json:"orphaned_tools"` // Tools without valid file config
	LastUpdated    *time.Time `json:"last_updated,omitempty"`
}

// FileConfigSummary represents a summary view of file configs
type FileConfigSummary struct {
	ConfigName      string     `json:"config_name"`
	EnvironmentName string     `json:"environment_name"`
	TemplatePath    string     `json:"template_path"`
	HasVariables    bool       `json:"has_variables"`
	VariablesPath   string     `json:"variables_path,omitempty"`
	ToolCount       int        `json:"tool_count"`
	ServerCount     int        `json:"server_count"`
	LastRendered    *time.Time `json:"last_rendered,omitempty"`
	IsStale         bool       `json:"is_stale"`   // Template changed since last render
	HasErrors       bool       `json:"has_errors"` // Last discovery had errors
}

// TemplateVariable represents a variable extracted from a template
type TemplateVariableRecord struct {
	ID              int64     `json:"id" db:"id"`
	FileConfigID    int64     `json:"file_config_id" db:"file_config_id"`
	VariableName    string    `json:"variable_name" db:"variable_name"`
	VariableType    string    `json:"variable_type" db:"variable_type"`
	Required        bool      `json:"required" db:"required"`
	DefaultValue    string    `json:"default_value,omitempty" db:"default_value"`
	Description     string    `json:"description,omitempty" db:"description"`
	Secret          bool      `json:"secret" db:"secret"`
	ValidationRules string    `json:"validation_rules,omitempty" db:"validation_rules"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// ConfigLoadingPreferences represents how configs should be loaded for an environment
type ConfigLoadingPreferences struct {
	ID                         int64     `json:"id" db:"id"`
	EnvironmentID              int64     `json:"environment_id" db:"environment_id"`
	PreferFiles                bool      `json:"prefer_files" db:"prefer_files"`
	EnableFallback             bool      `json:"enable_fallback" db:"enable_fallback"`
	AutoMigrate                bool      `json:"auto_migrate" db:"auto_migrate"`
	ValidateOnLoad             bool      `json:"validate_on_load" db:"validate_on_load"`
	VariableResolutionStrategy string    `json:"variable_resolution_strategy" db:"variable_resolution_strategy"`
	CreatedAt                  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at" db:"updated_at"`
}

// ConfigMigration tracks migration between database and file configs
type ConfigMigration struct {
	ID            int64      `json:"id" db:"id"`
	EnvironmentID int64      `json:"environment_id" db:"environment_id"`
	ConfigName    string     `json:"config_name" db:"config_name"`
	MigrationType string     `json:"migration_type" db:"migration_type"` // db_to_file, file_to_db
	SourceType    string     `json:"source_type" db:"source_type"`       // database, file
	TargetType    string     `json:"target_type" db:"target_type"`       // database, file
	Status        string     `json:"status" db:"status"`                 // pending, completed, failed
	ErrorMessage  string     `json:"error_message,omitempty" db:"error_message"`
	MigratedAt    *time.Time `json:"migrated_at,omitempty" db:"migrated_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// FileConfigChangeEvent represents a change to a file config
type FileConfigChangeEvent struct {
	ConfigName      string    `json:"config_name"`
	EnvironmentID   int64     `json:"environment_id"`
	EnvironmentName string    `json:"environment_name"`
	ChangeType      string    `json:"change_type"` // template_changed, variables_changed, created, deleted
	OldHash         string    `json:"old_hash,omitempty"`
	NewHash         string    `json:"new_hash,omitempty"`
	RequiresReload  bool      `json:"requires_reload"`
	Timestamp       time.Time `json:"timestamp"`
}

// RenderResult represents the result of template rendering
type RenderResult struct {
	ConfigName       string                 `json:"config_name"`
	EnvironmentID    int64                  `json:"environment_id"`
	Success          bool                   `json:"success"`
	RenderedConfig   *MCPConfigData         `json:"rendered_config,omitempty"`
	Variables        map[string]interface{} `json:"variables"`
	MissingVars      []string               `json:"missing_vars,omitempty"`
	ValidationErrors []string               `json:"validation_errors,omitempty"`
	RenderTime       time.Duration          `json:"render_time"`
	Timestamp        time.Time              `json:"timestamp"`
}
