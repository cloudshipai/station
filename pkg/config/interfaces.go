package config

import (
	"context"

	"github.com/spf13/afero"
	"station/pkg/models"
)

// ConfigManager manages file-based MCP configurations with GitOps support
type ConfigManager interface {
	// Template operations
	LoadTemplate(ctx context.Context, envID int64, configName string) (*MCPTemplate, error)
	SaveTemplate(ctx context.Context, envID int64, configName string, template *MCPTemplate) error
	RenderTemplate(ctx context.Context, template *MCPTemplate, variables map[string]interface{}) (*models.MCPConfigData, error)
	ValidateTemplate(ctx context.Context, templatePath string) (*TemplateValidation, error)
	
	// Variable operations
	LoadVariables(ctx context.Context, envID int64) (map[string]interface{}, error)
	SaveVariables(ctx context.Context, envID int64, variables map[string]interface{}) error
	PromptForMissingVariables(ctx context.Context, template *MCPTemplate, existing map[string]interface{}) (map[string]interface{}, error)
	
	// Discovery operations
	DiscoverTemplates(ctx context.Context, envPath string) ([]TemplateInfo, error)
	ExtractTemplateVariables(ctx context.Context, templatePath string) ([]TemplateVariable, error)
	
	// Environment operations
	GetConfigPath(envName, configName string) string
	GetVariablesPath(envName string) string
	EnsureEnvironmentStructure(envName string) error
}

// TemplateEngine handles Go template parsing and rendering
type TemplateEngine interface {
	Parse(ctx context.Context, templateContent string) (*ParsedTemplate, error)
	Render(ctx context.Context, template *ParsedTemplate, variables map[string]interface{}) (string, error)
	ExtractVariables(ctx context.Context, templateContent string) ([]TemplateVariable, error)
	Validate(ctx context.Context, templateContent string) error
}

// VariableStore manages environment variable files
type VariableStore interface {
	Load(ctx context.Context, filePath string) (map[string]interface{}, error)
	Save(ctx context.Context, filePath string, variables map[string]interface{}) error
	Merge(existing, new map[string]interface{}) map[string]interface{}
	Validate(variables map[string]interface{}, required []TemplateVariable) error
}

// FileSystem provides abstracted filesystem operations using afero
type FileSystem interface {
	afero.Fs
	// Config-specific operations
	EnsureConfigDir(envName string) error
	GetConfigPath(envName, configName string) string
	GetVariablesPath(envName string) string
	SetBasePaths(configDir, varsDir string)
}

// LoaderStrategy defines how configs are loaded (file-first, database-first, hybrid)
type LoaderStrategy interface {
	LoadMCPConfig(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error)
	SaveMCPConfig(ctx context.Context, envID int64, configName string, config interface{}) error
	ListConfigs(ctx context.Context, envID int64) ([]ConfigInfo, error)
}