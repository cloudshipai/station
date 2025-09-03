package mcp

import (
	"fmt"
	"strconv"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/crypto"
)

// addServerToConfig adds a single server to an existing MCP configuration
func (h *MCPHandler) addServerToConfig(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Running in local mode"))
	return h.addServerToConfigLocal(configID, serverName, command, args, envVars, environment)
}

// addServerToConfigLocal adds server to local configuration
func (h *MCPHandler) addServerToConfigLocal(configID, serverName, command string, args []string, envVars map[string]string, environment string) (string, error) {
	// Load Station config
	cfg, err := loadStationConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load Station config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	keyManager, err := crypto.NewKeyManagerFromEnv()
	if err != nil {
		return "", fmt.Errorf("failed to initialize key manager: %w", err)
	}
	// TODO: Replace with file-based config service
	// mcpConfigService := services.NewMCPConfigService(repos, keyManager)

	// Find environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return "", fmt.Errorf("environment '%s' not found", environment)
	}

	// Find config (try by name first, then by ID)
	var config *repositories.FileConfigRecord
	if configByName, err := repos.FileMCPConfigs.GetByEnvironmentAndName(env.ID, configID); err == nil {
		config = configByName
	} else {
		// Try parsing as ID
		if id, parseErr := strconv.ParseInt(configID, 10, 64); parseErr == nil {
			if configByID, err := repos.FileMCPConfigs.GetByID(id); err == nil {
				config = configByID
			}
		}
	}

	if config == nil {
		return "", fmt.Errorf("config '%s' not found", configID)
	}

	// NOTE: The following code references MCPConfigService which has compilation errors
	// Commenting out until the service is properly implemented for file-based configs
	
	/* 
	// Get and decrypt existing config
	configData, err := mcpConfigService.GetDecryptedConfig(config.ID)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt existing config: %w", err)
	}

	// Add new server to the config
	if configData.Servers == nil {
		configData.Servers = make(map[string]models.MCPServerConfig)
	}
	
	configData.Servers[serverName] = models.MCPServerConfig{
		Command: command,
		Args:    args,
		Env:     envVars,
	}

	// Upload updated config (creates new version)
	newConfig, err := mcpConfigService.UploadConfig(env.ID, configData)
	if err != nil {
		return "", fmt.Errorf("failed to save updated config: %w", err)
	}

	return fmt.Sprintf("Added server '%s' to config '%s' (new version: %d)", 
		serverName, config.ConfigName, newConfig.Version), nil
	*/
	
	// Temporary implementation to avoid compilation errors
	_ = keyManager // suppress unused variable warning
	return fmt.Sprintf("Server '%s' addition to config '%s' - feature temporarily disabled due to MCPConfigService refactoring", 
		serverName, config.ConfigName), nil
}

