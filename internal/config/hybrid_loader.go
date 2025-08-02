package config

import (
	"context"
	"fmt"
	"log"

	"station/internal/db/repositories"
	pkgconfig "station/pkg/config"
	"station/pkg/models"
)

// HybridLoadStrategy implements a hybrid file-first, database-fallback loading strategy
type HybridLoadStrategy struct {
	fileManager   pkgconfig.ConfigManager
	dbConfigRepo  *repositories.MCPConfigRepo
	dbEnvRepo     *repositories.EnvironmentRepo
	preferFiles   bool
	enableFallback bool
}

// NewHybridLoadStrategy creates a new hybrid loading strategy
func NewHybridLoadStrategy(
	fileManager pkgconfig.ConfigManager,
	dbConfigRepo *repositories.MCPConfigRepo,
	dbEnvRepo *repositories.EnvironmentRepo,
	opts HybridLoadOptions,
) *HybridLoadStrategy {
	return &HybridLoadStrategy{
		fileManager:   fileManager,
		dbConfigRepo:  dbConfigRepo,
		dbEnvRepo:     dbEnvRepo,
		preferFiles:   opts.PreferFiles,
		enableFallback: opts.EnableFallback,
	}
}

// HybridLoadOptions configures the hybrid loading behavior
type HybridLoadOptions struct {
	PreferFiles     bool // If true, try files first, then database
	EnableFallback  bool // If true, fallback to alternative if primary fails
	MigrateOnLoad   bool // If true, migrate database configs to files on load
	ValidateOnLoad  bool // If true, validate loaded configs
}

// LoadMCPConfig loads an MCP config using the hybrid strategy
func (h *HybridLoadStrategy) LoadMCPConfig(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	if h.preferFiles {
		return h.loadFileFirst(ctx, envID, configName)
	}
	return h.loadDatabaseFirst(ctx, envID, configName)
}

// SaveMCPConfig saves an MCP config using the hybrid strategy
func (h *HybridLoadStrategy) SaveMCPConfig(ctx context.Context, envID int64, configName string, config interface{}) error {
	var errors []error
	
	// Try to save to both file and database systems
	if h.preferFiles {
		// Save to files first
		if err := h.saveToFiles(ctx, envID, configName, config); err != nil {
			errors = append(errors, fmt.Errorf("file save failed: %w", err))
		}
		
		// Also save to database for backward compatibility (if enabled)
		if h.enableFallback {
			if err := h.saveToDatabase(ctx, envID, configName, config); err != nil {
				log.Printf("Warning: database save failed: %v", err)
			}
		}
	} else {
		// Save to database first
		if err := h.saveToDatabase(ctx, envID, configName, config); err != nil {
			errors = append(errors, fmt.Errorf("database save failed: %w", err))
		}
		
		// Also save to files (if enabled)
		if h.enableFallback {
			if err := h.saveToFiles(ctx, envID, configName, config); err != nil {
				log.Printf("Warning: file save failed: %v", err)
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("hybrid save failed: %v", errors)
	}
	
	return nil
}

// ListConfigs lists configs from both sources and merges the results
func (h *HybridLoadStrategy) ListConfigs(ctx context.Context, envID int64) ([]pkgconfig.ConfigInfo, error) {
	var allConfigs []pkgconfig.ConfigInfo
	configMap := make(map[string]*pkgconfig.ConfigInfo)
	
	// Get environment name for file operations
	env, err := h.dbEnvRepo.GetByID(envID)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment: %w", err)
	}
	
	// List file-based configs
	fileTemplates, err := h.fileManager.DiscoverTemplates(ctx, env.Name)
	if err != nil {
		log.Printf("Warning: failed to discover file templates: %v", err)
	} else {
		for _, template := range fileTemplates {
			configInfo := pkgconfig.ConfigInfo{
				Name:        template.Name,
				Type:        pkgconfig.ConfigTypeFile,
				Path:        template.Path,
				Environment: env.Name,
				LastLoaded:  nil, // Would need to track this
			}
			configMap[template.Name] = &configInfo
		}
	}
	
	// List database configs
	dbConfigs, err := h.dbConfigRepo.GetLatestConfigs(envID)
	if err != nil {
		log.Printf("Warning: failed to get database configs: %v", err)
	} else {
		for _, dbConfig := range dbConfigs {
			if existing, exists := configMap[dbConfig.ConfigName]; exists {
				// Mark as hybrid if exists in both
				existing.Type = pkgconfig.ConfigTypeHybrid
				if existing.Metadata == nil {
					existing.Metadata = make(map[string]string)
				}
				existing.Metadata["database_version"] = fmt.Sprintf("%d", dbConfig.Version)
			} else {
				// Database-only config
				configInfo := pkgconfig.ConfigInfo{
					Name:        dbConfig.ConfigName,
					Type:        pkgconfig.ConfigTypeDatabase,
					Version:     dbConfig.Version,
					Environment: env.Name,
					LastLoaded:  &dbConfig.UpdatedAt,
				}
				configMap[dbConfig.ConfigName] = &configInfo
			}
		}
	}
	
	// Convert map to slice
	for _, configInfo := range configMap {
		allConfigs = append(allConfigs, *configInfo)
	}
	
	return allConfigs, nil
}

// Private methods

func (h *HybridLoadStrategy) loadFileFirst(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	// Try file-based config first
	if template, err := h.loadFromFiles(ctx, envID, configName); err == nil {
		return template, nil
	} else {
		log.Printf("File-based config load failed for %s: %v", configName, err)
	}
	
	// Fallback to database if enabled
	if h.enableFallback {
		if dbConfig, err := h.loadFromDatabase(ctx, envID, configName); err == nil {
			log.Printf("Fell back to database config for %s", configName)
			return dbConfig, nil
		} else {
			log.Printf("Database fallback also failed for %s: %v", configName, err)
		}
	}
	
	return nil, fmt.Errorf("config %s not found in files or database", configName)
}

func (h *HybridLoadStrategy) loadDatabaseFirst(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	// Try database first
	if dbConfig, err := h.loadFromDatabase(ctx, envID, configName); err == nil {
		return dbConfig, nil
	} else {
		log.Printf("Database config load failed for %s: %v", configName, err)
	}
	
	// Fallback to files if enabled
	if h.enableFallback {
		if template, err := h.loadFromFiles(ctx, envID, configName); err == nil {
			log.Printf("Fell back to file-based config for %s", configName)
			return template, nil
		} else {
			log.Printf("File fallback also failed for %s: %v", configName, err)
		}
	}
	
	return nil, fmt.Errorf("config %s not found in database or files", configName)
}

func (h *HybridLoadStrategy) loadFromFiles(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	// Load template
	template, err := h.fileManager.LoadTemplate(ctx, envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}
	
	// Load template-specific variables
	variables, err := h.loadTemplateVariables(ctx, envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to load variables: %w", err)
	}
	
	// Render template
	renderedConfig, err := h.fileManager.RenderTemplate(ctx, template, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}
	
	return renderedConfig, nil
}

func (h *HybridLoadStrategy) loadFromDatabase(ctx context.Context, envID int64, configName string) (*models.MCPConfigData, error) {
	// Get the latest config from database
	_, err := h.dbConfigRepo.GetLatestByName(envID, configName)
	if err != nil {
		return nil, fmt.Errorf("failed to get database config: %w", err)
	}
	
	// Parse the config JSON
	// This would need to decrypt and parse the JSON from dbConfig.ConfigJSON
	// For now, return a placeholder
	return &models.MCPConfigData{
		Name: configName,
		Servers: make(map[string]models.MCPServerConfig),
	}, nil
}

func (h *HybridLoadStrategy) loadTemplateVariables(ctx context.Context, envID int64, templateName string) (map[string]interface{}, error) {
	// This would use the ConfigManager to load template-specific variables
	// For now, load global variables
	return h.fileManager.LoadVariables(ctx, envID)
}

func (h *HybridLoadStrategy) saveToFiles(ctx context.Context, envID int64, configName string, config interface{}) error {
	// This would save the config as a template and update variables
	// Implementation depends on the specific config format
	return fmt.Errorf("file save not implemented yet")
}

func (h *HybridLoadStrategy) saveToDatabase(ctx context.Context, envID int64, configName string, config interface{}) error {
	// This would save to the existing database system
	// Implementation depends on the specific config format and encryption
	return fmt.Errorf("database save not implemented yet")
}

// Migration helpers

// MigrateConfigToFiles migrates a database config to file-based template
func (h *HybridLoadStrategy) MigrateConfigToFiles(ctx context.Context, envID int64, configName string) error {
	// Load from database
	dbConfig, err := h.loadFromDatabase(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("failed to load database config: %w", err)
	}
	
	// Convert to template format
	template, variables, err := h.convertConfigToTemplate(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to convert to template: %w", err)
	}
	
	// Save template
	if err := h.fileManager.SaveTemplate(ctx, envID, configName, template); err != nil {
		return fmt.Errorf("failed to save template: %w", err)
	}
	
	// Save variables  
	if err := h.fileManager.SaveVariables(ctx, envID, variables); err != nil {
		return fmt.Errorf("failed to save variables: %w", err)
	}
	
	log.Printf("Successfully migrated config %s to file-based template", configName)
	return nil
}

// MigrateConfigToDatabase migrates a file-based config to database
func (h *HybridLoadStrategy) MigrateConfigToDatabase(ctx context.Context, envID int64, configName string) error {
	// Load and render file-based config
	fileConfig, err := h.loadFromFiles(ctx, envID, configName)
	if err != nil {
		return fmt.Errorf("failed to load file config: %w", err)
	}
	
	// Save to database
	if err := h.saveToDatabase(ctx, envID, configName, fileConfig); err != nil {
		return fmt.Errorf("failed to save to database: %w", err)
	}
	
	log.Printf("Successfully migrated config %s to database", configName)
	return nil
}

func (h *HybridLoadStrategy) convertConfigToTemplate(config *models.MCPConfigData) (*pkgconfig.MCPTemplate, map[string]interface{}, error) {
	// This would analyze the config and extract variables to create a template
	// For now, return placeholders
	template := &pkgconfig.MCPTemplate{
		Name:    config.Name,
		Content: "{}",
	}
	variables := make(map[string]interface{})
	
	return template, variables, nil
}

// Ensure we implement the interface
var _ pkgconfig.LoaderStrategy = (*HybridLoadStrategy)(nil)