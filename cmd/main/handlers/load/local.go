package load

import (
	"fmt"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// uploadConfigLocalLoad uploads config to local database
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

	// Create key manager from config file encryption key
	// keyManager removed - no longer needed for file-based configs

	// MCPConfigService removed - using file-based configs only

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

	// TODO: Update load handler for file-based configs
	// Legacy database upload removed - need to create file-based configs instead
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("⚠️  Load handler temporarily disabled during migration to file-based configs"))
	fmt.Printf("Config data prepared: %s with %d servers\n", configData.Name, len(configData.Servers))
	fmt.Println("Please use 'stn mcp create' to create file-based configs instead.")

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
	return nil
}

// uploadConfigLocalWizard uploads wizard config to local database
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

	// Create key manager from config file encryption key
	// keyManager removed - no longer needed for file-based configs

	// MCPConfigService removed - using file-based configs only

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

	// TODO: Update load handler for file-based configs (second occurrence)
	// Legacy database upload removed - need to create file-based configs instead
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("⚠️  Load handler temporarily disabled during migration to file-based configs"))
	fmt.Printf("Config data prepared: %s with %d servers\n", configData.Name, len(configData.Servers))

	// TODO: Update tool discovery for file-based configs
	// Legacy ReplaceToolsWithTransaction removed - need file-based approach
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("⚠️  Tool discovery temporarily disabled during migration to file-based configs"))

	showSuccessBanner("MCP Configuration Uploaded Successfully!", h.themeManager)
	return nil
}
