package config

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"station/internal/db/repositories"
	"station/pkg/config"
	"station/pkg/models"
)

// FileConfigManager implements the ConfigManager interface
type FileConfigManager struct {
	fs           config.FileSystem
	templateEng  config.TemplateEngine
	variableStore config.VariableStore
	options      config.FileConfigOptions
	envRepo      *repositories.EnvironmentRepo
}

// NewFileConfigManager creates a new file-based configuration manager
func NewFileConfigManager(fs config.FileSystem, templateEng config.TemplateEngine, variableStore config.VariableStore, opts config.FileConfigOptions, envRepo *repositories.EnvironmentRepo) *FileConfigManager {
	return &FileConfigManager{
		fs:           fs,
		templateEng:  templateEng,
		variableStore: variableStore,
		options:      opts,
		envRepo:      envRepo,
	}
}

// LoadTemplate loads and parses a template file
func (m *FileConfigManager) LoadTemplate(ctx context.Context, envID int64, configName string) (*config.MCPTemplate, error) {
	// Get environment name from envID (would need to fetch from DB)
	envName := m.getEnvironmentName(envID) // Helper method needed
	
	templatePath := m.fs.GetConfigPath(envName, configName)
	
	// Check if file exists
	exists, err := afero.Exists(m.fs, templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check template existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templatePath)
	}
	
	// Read template content
	content, err := afero.ReadFile(m.fs, templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}
	
	// Extract variables from template
	variables, err := m.templateEng.ExtractVariables(ctx, string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to extract variables: %w", err)
	}
	
	// Get file info for metadata
	info, err := m.fs.Stat(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	
	return &config.MCPTemplate{
		Name:      configName,
		FilePath:  templatePath,
		Content:   string(content),
		Variables: variables,
		Metadata: config.TemplateMetadata{
			CreatedAt: info.ModTime(),
			UpdatedAt: info.ModTime(),
		},
	}, nil
}

// LoadVariables loads variables with support for template-specific overrides
func (m *FileConfigManager) LoadVariables(ctx context.Context, envID int64) (map[string]interface{}, error) {
	envName := m.getEnvironmentName(envID)
	
	scope := &config.VariableScope{
		Global:           make(map[string]interface{}),
		TemplateSpecific: make(map[string]map[string]interface{}),
		Environment:      make(map[string]string),
	}
	
	// Load global variables
	globalVarsPath := m.fs.GetVariablesPath(envName)
	if exists, _ := afero.Exists(m.fs, globalVarsPath); exists {
		globalVars, err := m.variableStore.Load(ctx, globalVarsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load global variables: %w", err)
		}
		scope.Global = globalVars
	}
	
	// Load template-specific variables
	templateVarsDir := filepath.Join(filepath.Dir(globalVarsPath), "template-vars")
	if exists, _ := afero.Exists(m.fs, templateVarsDir); exists {
		err := m.loadTemplateSpecificVariables(ctx, templateVarsDir, scope)
		if err != nil {
			return nil, fmt.Errorf("failed to load template-specific variables: %w", err)
		}
	}
	
	// Merge variables according to strategy
	return m.mergeVariables(scope), nil
}

// LoadTemplateVariables loads variables specific to a template
func (m *FileConfigManager) LoadTemplateVariables(ctx context.Context, envID int64, templateName string) (map[string]interface{}, error) {
	envName := m.getEnvironmentName(envID)
	
	// Load global variables first
	variables, err := m.LoadVariables(ctx, envID)
	if err != nil {
		return nil, err
	}
	
	// Load template-specific variables
	templateVarsPath := m.getTemplateVariablesPath(envName, templateName)
	if exists, _ := afero.Exists(m.fs, templateVarsPath); exists {
		templateVars, err := m.variableStore.Load(ctx, templateVarsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load template variables for %s: %w", templateName, err)
		}
		
		// Merge with strategy
		variables = m.mergeWithStrategy(variables, templateVars, templateName)
	}
	
	return variables, nil
}

// RenderTemplate renders a template with resolved variables
func (m *FileConfigManager) RenderTemplate(ctx context.Context, template *config.MCPTemplate, variables map[string]interface{}) (*models.MCPConfigData, error) {
	// Parse the template
	parsed, err := m.templateEng.Parse(ctx, template.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	
	// Create render context
	renderCtx := &config.RenderContext{
		TemplateName: template.Name,
		Variables:    variables,
	}
	
	// Render the template
	rendered, err := m.templateEng.Render(ctx, parsed, renderCtx.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}
	
	// Parse the rendered JSON back to MCPConfigData
	configData, err := m.parseRenderedConfig(rendered)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rendered config: %w", err)
	}
	
	return configData, nil
}

// Helper methods

func (m *FileConfigManager) loadTemplateSpecificVariables(ctx context.Context, templateVarsDir string, scope *config.VariableScope) error {
	// List all .env files in template vars directory
	files, err := afero.ReadDir(m.fs, templateVarsDir)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".env") {
			continue
		}
		
		// Extract template name from filename (e.g., "github-tools.env" -> "github-tools")
		templateName := strings.TrimSuffix(file.Name(), ".env")
		varsPath := filepath.Join(templateVarsDir, file.Name())
		
		templateVars, err := m.variableStore.Load(ctx, varsPath)
		if err != nil {
			return fmt.Errorf("failed to load variables for template %s: %w", templateName, err)
		}
		
		scope.TemplateSpecific[templateName] = templateVars
	}
	
	return nil
}

func (m *FileConfigManager) mergeVariables(scope *config.VariableScope) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Start with global variables
	for k, v := range scope.Global {
		result[k] = v
	}
	
	// Apply strategy-based merging
	switch m.options.Strategy {
	case config.StrategyTemplateFirst:
		// Template-specific overrides global (handled per template)
		return result
		
	case config.StrategyGlobalFirst:
		// Global takes precedence (already done above)
		return result
		
	case config.StrategyNamespaced:
		// Require namespaced variables (validate during load)
		return m.validateNamespacedVariables(result)
		
	default:
		return result
	}
}

func (m *FileConfigManager) mergeWithStrategy(global, templateSpecific map[string]interface{}, templateName string) map[string]interface{} {
	switch m.options.Strategy {
	case config.StrategyTemplateFirst:
		// Template-specific overrides global
		result := make(map[string]interface{})
		for k, v := range global {
			result[k] = v
		}
		for k, v := range templateSpecific {
			result[k] = v
		}
		return result
		
	case config.StrategyGlobalFirst:
		// Global takes precedence over template-specific
		result := make(map[string]interface{})
		for k, v := range templateSpecific {
			result[k] = v
		}
		for k, v := range global {
			result[k] = v
		}
		return result
		
	default:
		return global
	}
}

func (m *FileConfigManager) getTemplateVariablesPath(envName, templateName string) string {
	envDir := filepath.Join(m.options.ConfigDir, "environments", envName)
	return filepath.Join(envDir, "template-vars", templateName+".env")
}

func (m *FileConfigManager) validateNamespacedVariables(variables map[string]interface{}) map[string]interface{} {
	// Implementation for validating namespaced variables
	// This would check that variables follow naming conventions like GitHub_ApiKey, AWS_ApiKey
	return variables
}

func (m *FileConfigManager) parseRenderedConfig(rendered string) (*models.MCPConfigData, error) {
	// Parse the rendered JSON string back to MCPConfigData struct
	var configData models.MCPConfigData
	if err := json.Unmarshal([]byte(rendered), &configData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rendered config: %w", err)
	}
	
	return &configData, nil
}

func (m *FileConfigManager) getEnvironmentName(envID int64) string {
	// If no envRepo provided, return default
	if m.envRepo == nil {
		return "default"
	}
	
	// Fetch the environment name from the database using envID
	env, err := m.envRepo.GetByID(envID)
	if err != nil {
		// If we can't find the environment, return default
		return "default"
	}
	return env.Name
}

// SaveTemplate saves a template to the filesystem
func (m *FileConfigManager) SaveTemplate(ctx context.Context, envID int64, configName string, template *config.MCPTemplate) error {
	envName := m.getEnvironmentName(envID)
	templatePath := m.fs.GetConfigPath(envName, configName)
	
	// Ensure directory exists
	if err := m.fs.EnsureConfigDir(envName); err != nil {
		return fmt.Errorf("failed to ensure config directory: %w", err)
	}
	
	// Write template content
	if err := afero.WriteFile(m.fs, templatePath, []byte(template.Content), 0644); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}
	
	return nil
}

// SaveVariables saves variables to the filesystem
func (m *FileConfigManager) SaveVariables(ctx context.Context, envID int64, variables map[string]interface{}) error {
	envName := m.getEnvironmentName(envID)
	varsPath := m.fs.GetVariablesPath(envName)
	
	return m.variableStore.Save(ctx, varsPath, variables)
}

// ValidateTemplate validates a template at the given path
func (m *FileConfigManager) ValidateTemplate(ctx context.Context, templatePath string) (*config.TemplateValidation, error) {
	// Read template content
	content, err := afero.ReadFile(m.fs, templatePath)
	if err != nil {
		return &config.TemplateValidation{
			Valid: false,
			Errors: []config.ValidationError{{
				Type:    "file_error",
				Message: fmt.Sprintf("failed to read template: %v", err),
			}},
		}, nil
	}
	
	// Validate template syntax
	if err := m.templateEng.Validate(ctx, string(content)); err != nil {
		return &config.TemplateValidation{
			Valid: false,
			Errors: []config.ValidationError{{
				Type:    "syntax_error",
				Message: err.Error(),
			}},
		}, nil
	}
	
	// Extract variables for validation
	variables, err := m.templateEng.ExtractVariables(ctx, string(content))
	if err != nil {
		return &config.TemplateValidation{
			Valid: false,
			Errors: []config.ValidationError{{
				Type:    "variable_extraction_error",
				Message: err.Error(),
			}},
		}, nil
	}
	
	return &config.TemplateValidation{
		Valid:     true,
		Variables: variables,
	}, nil
}

// DiscoverTemplates discovers all templates in an environment
func (m *FileConfigManager) DiscoverTemplates(ctx context.Context, envPath string) ([]config.TemplateInfo, error) {
	configDir := filepath.Join(m.options.ConfigDir, "environments", envPath, "templates")
	
	// Check if directory exists
	exists, err := afero.Exists(m.fs, configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to check config directory: %w", err)
	}
	if !exists {
		return []config.TemplateInfo{}, nil
	}
	
	// Read directory
	files, err := afero.ReadDir(m.fs, configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}
	
	var templates []config.TemplateInfo
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		
		templateName := strings.TrimSuffix(file.Name(), ".json")
		templatePath := filepath.Join(configDir, file.Name())
		
		// Check for variables file
		varsPath := m.getTemplateVariablesPath(envPath, templateName)
		hasVars, _ := afero.Exists(m.fs, varsPath)
		
		templates = append(templates, config.TemplateInfo{
			Name:     templateName,
			Path:     templatePath,
			Size:     file.Size(),
			ModTime:  file.ModTime(),
			HasVars:  hasVars,
			VarsPath: varsPath,
		})
	}
	
	return templates, nil
}

// ExtractTemplateVariables extracts variables from a template file
func (m *FileConfigManager) ExtractTemplateVariables(ctx context.Context, templatePath string) ([]config.TemplateVariable, error) {
	content, err := afero.ReadFile(m.fs, templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template: %w", err)
	}
	
	return m.templateEng.ExtractVariables(ctx, string(content))
}

// PromptForMissingVariables prompts for missing template variables
func (m *FileConfigManager) PromptForMissingVariables(ctx context.Context, template *config.MCPTemplate, existing map[string]interface{}) (map[string]interface{}, error) {
	// This would provide interactive prompts for missing variables
	// For now, just return the existing variables
	return existing, fmt.Errorf("interactive variable prompting not implemented")
}

// GetConfigPath returns the path for a config file
func (m *FileConfigManager) GetConfigPath(envName, configName string) string {
	return m.fs.GetConfigPath(envName, configName)
}

// GetVariablesPath returns the path for variables file
func (m *FileConfigManager) GetVariablesPath(envName string) string {
	return m.fs.GetVariablesPath(envName)
}

// EnsureEnvironmentStructure ensures the directory structure exists for an environment
func (m *FileConfigManager) EnsureEnvironmentStructure(envName string) error {
	return m.fs.EnsureConfigDir(envName)
}