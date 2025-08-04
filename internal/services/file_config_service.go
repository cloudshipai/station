package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"io/ioutil"
	"station/internal/db/repositories"
	"station/pkg/config"
	"station/pkg/models"
)

// FileConfigService manages file-based MCP configurations integrated with tool discovery
type FileConfigService struct {
	configManager   config.ConfigManager
	toolDiscovery   *ToolDiscoveryService
	repos          *repositories.Repositories
}

// NewFileConfigService creates a new file-based config service
func NewFileConfigService(
	configManager config.ConfigManager,
	toolDiscovery *ToolDiscoveryService,
	repos *repositories.Repositories,
) *FileConfigService {
	return &FileConfigService{
		configManager:   configManager,
		toolDiscovery:   toolDiscovery,  
		repos:          repos,
	}
}

// CreateOrUpdateTemplate creates or updates a template with variables and triggers tool discovery
func (s *FileConfigService) CreateOrUpdateTemplate(ctx context.Context, envID int64, configName string, template *config.MCPTemplate, variables map[string]interface{}) error {
	log.Printf("Creating/updating template %s in environment %d", configName, envID)
	
	// 1. Save template file
	if err := s.configManager.SaveTemplate(ctx, envID, configName, template); err != nil {
		return fmt.Errorf("failed to save template: %w", err)
	}
	
	// 2. Save variables file (template-specific)
	if len(variables) > 0 {
		if err := s.saveTemplateVariables(ctx, envID, configName, variables); err != nil {
			return fmt.Errorf("failed to save variables: %w", err)
		}
	}
	
	// 3. Calculate hashes for change detection
	templateHash := s.calculateTemplateHash(template.Content)
	variablesHash := s.calculateVariablesHash(variables)
	
	// 4. Update or create file config record
	fileConfig, err := s.updateFileConfigRecord(envID, configName, templateHash, variablesHash)
	if err != nil {
		return fmt.Errorf("failed to update file config record: %w", err)
	}
	
	// 5. Trigger tool discovery with rendered config
	if err := s.discoverAndStoreTools(ctx, fileConfig); err != nil {
		return fmt.Errorf("failed to discover tools: %w", err)
	}
	
	log.Printf("Successfully created/updated template %s", configName)
	return nil
}

// LoadAndRenderConfig loads a template, renders it with variables, and returns the config
func (s *FileConfigService) LoadAndRenderConfig(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	// 1. Load template
	template, err := s.configManager.LoadTemplate(ctx, envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}
	
	// 2. Load template-specific variables
	variables, err := s.loadTemplateVariables(ctx, envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to load variables: %w", err)
	}
	
	// 3. Load global variables and merge
	globalVars, err := s.configManager.LoadVariables(ctx, envID)
	if err != nil {
		log.Printf("Warning: failed to load global variables: %v", err)
		globalVars = make(map[string]interface{})
	}
	
	// 4. Merge variables (template-specific takes precedence)
	finalVars := s.mergeVariables(globalVars, variables)
	
	// 5. Render template
	renderedConfig, err := s.configManager.RenderTemplate(ctx, template, finalVars)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}
	
	return renderedConfig, nil
}

// LoadAndRenderConfigSimple loads and renders config using simple variable substitution (same as load process)
func (s *FileConfigService) LoadAndRenderConfigSimple(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	// 1. Get environment for file paths
	env, err := s.repos.Environments.GetByID(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}

	// 2. Build template file path
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	envDir := filepath.Join(configHome, "station", "environments", env.Name)
	templatePath := filepath.Join(envDir, configName+".json")

	// 3. Read template file
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", templatePath, err)
	}

	// 4. Parse template to find placeholders
	templatePattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := templatePattern.FindAllStringSubmatch(string(templateBytes), -1)
	var placeholders []string
	for _, match := range matches {
		if len(match) > 1 {
			placeholders = append(placeholders, match[1])
		}
	}

	// 5. Resolve variables using same hierarchy as load process
	resolvedVars := s.resolveVariablesFromFileSystemSimple(placeholders, envDir)

	// 6. Replace placeholders with resolved values
	renderedContent := string(templateBytes)
	for placeholder, value := range resolvedVars {
		renderedContent = strings.ReplaceAll(renderedContent, fmt.Sprintf("{{%s}}", placeholder), value)
	}

	// 7. Parse rendered JSON
	var mcpConfig struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal([]byte(renderedContent), &mcpConfig); err != nil {
		return nil, fmt.Errorf("failed to parse rendered config: %w", err)
	}

	// 8. Convert to internal format
	servers := make(map[string]models.MCPServerConfig)
	for name, serverConfig := range mcpConfig.MCPServers {
		servers[name] = models.MCPServerConfig{
			Command: serverConfig.Command,
			Args:    serverConfig.Args,
			Env:     serverConfig.Env,
		}
	}

	return &models.MCPConfigData{
		Name:    configName,
		Servers: servers,
	}, nil
}

// resolveVariablesFromFileSystemSimple resolves variables using the same hierarchy as load process
func (s *FileConfigService) resolveVariablesFromFileSystemSimple(placeholders []string, envDir string) map[string]string {
	values := make(map[string]string)

	// Step 1: Load global variables from variables.yml
	globalVarsPath := filepath.Join(envDir, "variables.yml")
	globalVars := s.loadVariablesFromYAMLSimple(globalVarsPath)
	
	// Step 2: Check environment variables
	envVars := make(map[string]string)
	for _, placeholder := range placeholders {
		if envValue := os.Getenv(placeholder); envValue != "" {
			envVars[placeholder] = envValue
		}
	}

	// Apply resolution hierarchy for each placeholder
	for _, placeholder := range placeholders {
		var value string

		// Priority 1: Global vars (from variables.yml)
		if val, exists := globalVars[placeholder]; exists {
			value = val
		}

		// Priority 2: Environment variables (override global)
		if val, exists := envVars[placeholder]; exists {
			value = val
		}

		if value != "" {
			values[placeholder] = value
		}
	}

	return values
}

// loadVariablesFromYAMLSimple loads variables from a YAML file (simple version)
func (s *FileConfigService) loadVariablesFromYAMLSimple(filePath string) map[string]string {
	variables := make(map[string]string)
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		// File doesn't exist or can't be read - that's okay
		return variables
	}

	// Parse as YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		log.Printf("Warning: Failed to parse %s: %v", filePath, err)
		return variables
	}

	// Convert to string map
	for key, value := range yamlData {
		if strValue, ok := value.(string); ok {
			variables[key] = strValue
		} else {
			// Convert other types to string
			variables[key] = fmt.Sprintf("%v", value)
		}
	}

	return variables
}

// UpdateTemplateVariables updates variables for a specific template and re-renders
func (s *FileConfigService) UpdateTemplateVariables(ctx context.Context, envID int64, configName string, variables map[string]interface{}) error {
	log.Printf("Updating variables for template %s in environment %d", configName, envID)
	
	// 1. Save updated variables
	if err := s.saveTemplateVariables(ctx, envID, configName, variables); err != nil {
		return fmt.Errorf("failed to save variables: %w", err)
	}
	
	// 2. Update variables hash
	variablesHash := s.calculateVariablesHash(variables)
	if err := s.updateVariablesHash(envID, configName, variablesHash); err != nil {
		return fmt.Errorf("failed to update variables hash: %w", err)
	}
	
	// 3. Re-discover tools with new variables
	fileConfig, err := s.getFileConfigRecord(envID, configName)
	if err != nil {
		return fmt.Errorf("failed to get file config record: %w", err)
	}
	
	if err := s.discoverAndStoreTools(ctx, fileConfig); err != nil {
		return fmt.Errorf("failed to re-discover tools: %w", err)
	}
	
	log.Printf("Successfully updated variables for template %s", configName)
	return nil
}

// DiscoverToolsForConfig discovers tools for a specific file-based config
func (s *FileConfigService) DiscoverToolsForConfig(ctx context.Context, envID int64, configName string) (*ToolDiscoveryResult, error) {
	log.Printf("Discovering tools for file config %s in environment %d", configName, envID)
	
	// 1. Load and render config using simple variable resolution (same as load process)
	renderedConfig, err := s.LoadAndRenderConfigSimple(ctx, envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to load and render config: %w", err)
	}
	
	// 2. Get file config record
	fileConfig, err := s.getFileConfigRecord(envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to get file config record: %w", err)
	}
	
	// 3. Clear existing tools for this file config
	if err := s.clearExistingToolsForFileConfig(fileConfig.ID); err != nil {
		return nil, fmt.Errorf("failed to clear existing tools: %w", err)
	}
	
	// 4. Create a temporary MCPConfig for tool discovery
	tempConfig := &models.MCPConfig{
		ID:            fileConfig.ID,
		EnvironmentID: envID,
		ConfigName:    configName,
	}
	
	// 5. Discover tools using the existing tool discovery service
	result, err := s.discoverToolsFromRenderedConfig(tempConfig, renderedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tools: %w", err)
	}
	
	// 6. Update last rendered timestamp
	if err := s.updateLastRenderedTime(fileConfig.ID); err != nil {
		log.Printf("Warning: failed to update last rendered time: %v", err)
	}
	
	return result, nil
}

// ListFileConfigs lists all file-based configs for an environment
func (s *FileConfigService) ListFileConfigs(ctx context.Context, envID int64) ([]config.ConfigInfo, error) {
	// 1. Discover templates from filesystem
	envName, err := s.getEnvironmentName(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment name: %w", err)
	}
	
	templates, err := s.configManager.DiscoverTemplates(ctx, envName)
	if err != nil {
		return nil, fmt.Errorf("failed to discover templates: %w", err)
	}
	
	// 2. Get file config records from database
	fileConfigs, err := s.getFileConfigsByEnvironment(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file config records: %w", err)
	}
	
	// 3. Merge information
	configMap := make(map[string]*config.ConfigInfo)
	
	// Add templates from filesystem
	for _, template := range templates {
		configInfo := config.ConfigInfo{
			Name:        template.Name,
			Type:        config.ConfigTypeFile,
			Path:        template.Path,
			Environment: envName,
		}
		configMap[template.Name] = &configInfo
	}
	
	// Add database information
	for _, fileConfig := range fileConfigs {
		if configInfo, exists := configMap[fileConfig.ConfigName]; exists {
			metadata := map[string]string{
				"template_hash": fileConfig.TemplateHash,
				"variables_hash": fileConfig.VariablesHash,
			}
			if fileConfig.LastLoadedAt != nil {
				metadata["last_loaded"] = fileConfig.LastLoadedAt.Format(time.RFC3339)
			}
			configInfo.Metadata = metadata
		}
	}
	
	// Convert to slice
	var result []config.ConfigInfo
	for _, configInfo := range configMap {
		result = append(result, *configInfo)
	}
	
	return result, nil
}

// GeneratePlaceholders generates .env.example files for GitOps workflow
func (s *FileConfigService) GeneratePlaceholders(ctx context.Context, envID int64, configName string) error {
	// 1. Load template
	template, err := s.configManager.LoadTemplate(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}
	
	// 2. Generate placeholder content
	placeholderContent := s.generatePlaceholderContent(template.Variables)
	
	// 3. Save placeholder file
	envName, err := s.getEnvironmentName(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment name: %w", err)
	}
	
	placeholderPath := s.getPlaceholderPath(envName, configName)
	if err := s.savePlaceholderFile(placeholderPath, placeholderContent); err != nil {
		return fmt.Errorf("failed to save placeholder file: %w", err)
	}
	
	log.Printf("Generated placeholder file: %s", placeholderPath)
	return nil
}

// Private helper methods

func (s *FileConfigService) saveTemplateVariables(ctx context.Context, envID int64, configName string, variables map[string]interface{}) error {
	// This would save template-specific variables to environments/{env}/template-vars/{configName}.env
	// Implementation would use the VariableStore interface
	return nil // TODO: Implement
}

func (s *FileConfigService) loadTemplateVariables(ctx context.Context, envID int64, configName string) (map[string]interface{}, error) {
	envName, err := s.getEnvironmentName(envID)
	if err != nil {
		return make(map[string]interface{}), nil // Return empty if we can't get env name
	}
	
	// Load template-specific variables from proper config directory
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	templateVarsPath := filepath.Join(configHome, "station", "environments", envName, configName+".vars.yml")
	
	// Check if template-specific variables file exists
	if _, err := os.Stat(templateVarsPath); err != nil {
		// File doesn't exist, return empty map (not an error)
		return make(map[string]interface{}), nil
	}
	
	// Load template-specific variables using a simple YAML loader
	variables, err := s.loadYAMLVariables(templateVarsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load template variables from %s: %w", templateVarsPath, err)
	}
	
	return variables, nil
}

func (s *FileConfigService) loadYAMLVariables(filePath string) (map[string]interface{}, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var variables map[string]interface{}
	err = yaml.Unmarshal(data, &variables)
	if err != nil {
		return nil, err
	}
	
	return variables, nil
}

func (s *FileConfigService) mergeVariables(global, templateSpecific map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Start with global variables
	for k, v := range global {
		result[k] = v
	}
	
	// Override with template-specific variables
	for k, v := range templateSpecific {
		result[k] = v
	}
	
	return result
}

func (s *FileConfigService) calculateTemplateHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

func (s *FileConfigService) calculateVariablesHash(variables map[string]interface{}) string {
	// Serialize variables and hash
	content := fmt.Sprintf("%v", variables) // Simple serialization
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash)
}

func (s *FileConfigService) updateFileConfigRecord(envID int64, configName, templateHash, variablesHash string) (*repositories.FileConfigRecord, error) {
	// This would update the file_mcp_configs table
	// TODO: Implement when we have the repository methods
	return &repositories.FileConfigRecord{
		ID:              1,
		EnvironmentID:   envID,
		ConfigName:      configName,
		TemplateHash:    templateHash,
		VariablesHash:   variablesHash,
	}, nil
}

func (s *FileConfigService) getFileConfigRecord(envID int64, configName string) (*repositories.FileConfigRecord, error) {
	// Try to get existing record
	record, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
	if err == nil {
		return record, nil
	}
	
	// Record doesn't exist, create it
	envName, err := s.getEnvironmentName(envID)
	if err != nil {
		envName = "default"
	}
	templatePath := fmt.Sprintf("config/environments/%s/%s.json", envName, configName)
	variablesPath := fmt.Sprintf("config/environments/%s/variables.yml", envName)
	
	record = &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        configName,
		TemplatePath:      templatePath,
		VariablesPath:     variablesPath,
	}
	
	id, err := s.repos.FileMCPConfigs.Create(record)
	if err != nil {
		return nil, fmt.Errorf("failed to create file config record: %w", err)
	}
	
	record.ID = id
	return record, nil
}


func (s *FileConfigService) discoverAndStoreTools(ctx context.Context, fileConfig *repositories.FileConfigRecord) error {
	// This would call the existing tool discovery but link tools to file config
	_, err := s.DiscoverToolsForConfig(ctx, fileConfig.EnvironmentID, fileConfig.ConfigName)
	return err
}

func (s *FileConfigService) discoverToolsFromRenderedConfig(tempConfig *models.MCPConfig, renderedConfig *models.MCPConfigData) (*ToolDiscoveryResult, error) {
	// Use the updated ToolDiscoveryService to handle file config tool discovery
	return s.toolDiscovery.DiscoverToolsFromFileConfig(tempConfig.EnvironmentID, tempConfig.ConfigName, renderedConfig)
}

func (s *FileConfigService) clearExistingToolsForFileConfig(fileConfigID int64) error {
	// Clear tools linked to this file config using the extension methods
	return s.repos.MCPTools.DeleteByFileConfigID(fileConfigID)
}

func (s *FileConfigService) updateLastRenderedTime(fileConfigID int64) error {
	// Update the last_loaded_at timestamp in file_mcp_configs table
	return s.repos.FileMCPConfigs.UpdateLastLoadedAt(fileConfigID)
}

func (s *FileConfigService) updateVariablesHash(envID int64, configName, variablesHash string) error {
	// Get the file config record
	fileConfig, err := s.repos.FileMCPConfigs.GetByEnvironmentAndName(envID, configName)
	if err != nil {
		return fmt.Errorf("failed to get file config: %w", err)
	}
	// Update the variables hash
	return s.repos.FileMCPConfigs.UpdateHashes(fileConfig.ID, fileConfig.TemplateHash, variablesHash, fileConfig.TemplateVarsHash)
}

func (s *FileConfigService) getFileConfigsByEnvironment(envID int64) ([]*repositories.FileConfigRecord, error) {
	// Get all file configs for an environment from the repository
	return s.repos.FileMCPConfigs.ListByEnvironment(envID)
}

func (s *FileConfigService) getEnvironmentName(envID int64) (string, error) {
	env, err := s.repos.Environments.GetByID(envID)
	if err != nil {
		return "", err
	}
	return env.Name, nil
}

func (s *FileConfigService) generatePlaceholderContent(variables []config.TemplateVariable) string {
	content := "# MCP Configuration Variables\n"
	content += "# Copy this file to the appropriate template-vars directory and fill in the values\n\n"
	
	for _, variable := range variables {
		if variable.Secret {
			content += fmt.Sprintf("%s=# %s (required, secret)\n", variable.Name, variable.Description)
		} else {
			defaultVal := ""
			if variable.Default != nil {
				defaultVal = fmt.Sprintf("%v", variable.Default)
			}
			content += fmt.Sprintf("%s=%s # %s\n", variable.Name, defaultVal, variable.Description)
		}
	}
	
	return content
}

func (s *FileConfigService) getPlaceholderPath(envName, configName string) string {
	// This would return the path for placeholder files
	return fmt.Sprintf("placeholders/%s.env.example", configName)
}

func (s *FileConfigService) savePlaceholderFile(path, content string) error {
	// This would save the placeholder file using the filesystem abstraction
	// TODO: Implement using the FileSystem interface
	return nil
}

// FileConfigRecord represents a file-based config record
type FileConfigRecord struct {
	ID              int64
	EnvironmentID   int64
	ConfigName      string
	TemplatePath    string
	VariablesPath   string
	TemplateHash    string
	VariablesHash   string
	LastRenderedAt  time.Time
}