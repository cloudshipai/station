package load

import (
	"encoding/json"
	"fmt"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"
)

// uploadConfigLocalLoad creates file-based config using FileConfigService
func (h *LoadHandler) uploadConfigLocalLoad(mcpConfig LoadMCPConfig, configName, environment string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}

	// Find or create environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		// Create environment if it doesn't exist
		description := fmt.Sprintf("Environment for %s", environment)
		env, err = repos.Environments.Create(environment, &description, consoleUser.ID)
		if err != nil {
			return fmt.Errorf("failed to create environment: %w", err)
		}
		fmt.Printf("✅ Created environment: %s (ID: %d)\n", environment, env.ID)
	}

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("📁 Creating file-based configuration..."))

	// Convert MCP config to internal format and save as file-based config
	err = h.createFileBasedConfig(env.ID, configName, mcpConfig, repos)
	if err != nil {
		return fmt.Errorf("failed to create file-based config: %w", err)
	}

	fmt.Printf("✅ Created file-based config: %s in environment %s\n", configName, environment)
	fmt.Printf("🔧 Discovered tools from %d MCP servers\n", len(mcpConfig.MCPServers))

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
	return nil
}


// createFileBasedConfig creates a file-based config record and triggers tool discovery
func (h *LoadHandler) createFileBasedConfig(envID int64, configName string, mcpConfig LoadMCPConfig, repos *repositories.Repositories) error {
	// Convert MCP config to internal format
	servers := make(map[string]models.MCPServerConfig)
	for name, serverConfig := range mcpConfig.MCPServers {
		servers[name] = models.MCPServerConfig{
			Command: serverConfig.Command,
			Args:    serverConfig.Args,
			Env:     serverConfig.Env,
		}
	}

	configData := &models.MCPConfigData{
		Name:    configName,
		Servers: servers,
	}

	// Get environment name for file paths
	env, err := repos.Environments.GetByID(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Create file config record in database
	templatePath := fmt.Sprintf("config/environments/%s/%s.json", env.Name, configName)
	variablesPath := fmt.Sprintf("config/environments/%s/variables.yml", env.Name)
	
	fileConfigRecord := &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        configName,
		TemplatePath:      templatePath,
		VariablesPath:     variablesPath,
		TemplateHash:      h.calculateConfigHash(configData),
		VariablesHash:     "",
	}

	_, err = repos.FileMCPConfigs.Create(fileConfigRecord)
	if err != nil {
		return fmt.Errorf("failed to create file config record: %w", err)
	}

	// Create a simple tool discovery service and discover tools
	toolDiscovery := services.NewToolDiscoveryService(repos)

	// Use the existing tool discovery with the rendered config
	_, err = toolDiscovery.DiscoverToolsFromFileConfig(envID, configName, configData)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
		// Don't fail the entire operation if tool discovery fails
	}

	return nil
}

// calculateConfigHash calculates a simple hash for the config
func (h *LoadHandler) calculateConfigHash(config *models.MCPConfigData) string {
	configBytes, _ := json.Marshal(config)
	return fmt.Sprintf("%x", len(configBytes)) // Simple hash based on length
}

// createFileBasedConfigFromData creates a file-based config from MCPConfigData
func (h *LoadHandler) createFileBasedConfigFromData(envID int64, configData *models.MCPConfigData, repos *repositories.Repositories) error {
	// Get environment name for file paths
	env, err := repos.Environments.GetByID(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Create file config record in database
	templatePath := fmt.Sprintf("config/environments/%s/%s.json", env.Name, configData.Name)
	variablesPath := fmt.Sprintf("config/environments/%s/variables.yml", env.Name)
	
	fileConfigRecord := &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        configData.Name,
		TemplatePath:      templatePath,
		VariablesPath:     variablesPath,
		TemplateHash:      h.calculateConfigHash(configData),
		VariablesHash:     "",
	}

	_, err = repos.FileMCPConfigs.Create(fileConfigRecord)
	if err != nil {
		return fmt.Errorf("failed to create file config record: %w", err)
	}

	// Create a simple tool discovery service and discover tools
	toolDiscovery := services.NewToolDiscoveryService(repos)
	
	// Use the existing tool discovery with the rendered config
	_, err = toolDiscovery.DiscoverToolsFromFileConfig(envID, configData.Name, configData)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
		// Don't fail the entire operation if tool discovery fails
	}

	return nil
}

// uploadConfigLocalWizard creates file-based config from wizard data using FileConfigService
func (h *LoadHandler) uploadConfigLocalWizard(configData *models.MCPConfigData, environment string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}

	// Find or create environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		// Create environment if it doesn't exist
		description := fmt.Sprintf("Environment for %s", environment)
		env, err = repos.Environments.Create(environment, &description, consoleUser.ID)
		if err != nil {
			return fmt.Errorf("failed to create environment: %w", err)
		}
		fmt.Printf("✅ Created environment: %s (ID: %d)\n", environment, env.ID)
	}

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("📁 Creating file-based configuration from wizard..."))

	// Create file-based config directly
	err = h.createFileBasedConfigFromData(env.ID, configData, repos)
	if err != nil {
		return fmt.Errorf("failed to create file-based config: %w", err)
	}

	fmt.Printf("✅ Created file-based config: %s in environment %s\n", configData.Name, environment)
	fmt.Printf("🔧 Discovered tools from %d MCP servers\n", len(configData.Servers))

	showSuccessBanner("MCP Configuration Uploaded Successfully!", h.themeManager)
	return nil
}
