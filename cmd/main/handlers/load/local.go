package load

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
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
	// This function is deprecated - use createFileBasedConfigTemplate instead
	// For backwards compatibility, convert to standard format
	standardConfig := map[string]interface{}{
		"mcpServers": mcpConfig.MCPServers,
	}

	// Get environment name for file paths
	env, err := repos.Environments.GetByID(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Use proper config directory and standardize naming to snake_case
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	
	// Convert config name to snake_case for cleaner file names
	cleanConfigName := strings.ReplaceAll(strings.ToLower(configName), " ", "_")
	cleanConfigName = strings.ReplaceAll(cleanConfigName, "-", "_")
	
	// Create proper paths
	envDir := filepath.Join(configHome, "station", "environments", env.Name)
	templatePath := filepath.Join(envDir, cleanConfigName+".json")
	
	// Ensure environment directory exists
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Save template file to filesystem in standard MCP format
	templateBytes, err := json.MarshalIndent(standardConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template data: %w", err)
	}
	
	if err := os.WriteFile(templatePath, templateBytes, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	
	fmt.Printf("✅ Saved template file: %s\n", templatePath)

	// Create file config record in database with relative paths for portability
	relativeTemplatePath := filepath.Join("environments", env.Name, cleanConfigName+".json")
	relativeVariablesPath := filepath.Join("environments", env.Name, "variables.yml")
	
	fileConfigRecord := &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        cleanConfigName, // Use cleaned name
		TemplatePath:      relativeTemplatePath,
		VariablesPath:     relativeVariablesPath,
		TemplateHash:      h.calculateConfigHashFromBytes(templateBytes),
		VariablesHash:     "",
	}

	_, err = repos.FileMCPConfigs.Create(fileConfigRecord)
	if err != nil {
		return fmt.Errorf("failed to create file config record: %w", err)
	}

	// Create a simple tool discovery service and discover tools
	toolDiscovery := services.NewToolDiscoveryService(repos)

	// Convert back to internal format for tool discovery
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

// calculateConfigHashFromBytes calculates a simple hash from bytes
func (h *LoadHandler) calculateConfigHashFromBytes(bytes []byte) string {
	return fmt.Sprintf("%x", len(bytes)) // Simple hash based on length
}

// createFileBasedConfigFromData creates a file-based config from MCPConfigData
func (h *LoadHandler) createFileBasedConfigFromData(envID int64, configData *models.MCPConfigData, repos *repositories.Repositories) error {
	// Get environment name for file paths
	env, err := repos.Environments.GetByID(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Convert internal format to standard MCP format
	mcpServers := make(map[string]interface{})
	for name, serverConfig := range configData.Servers {
		mcpServers[name] = map[string]interface{}{
			"command": serverConfig.Command,
			"args":    serverConfig.Args,
			"env":     serverConfig.Env,
		}
	}
	
	standardConfig := map[string]interface{}{
		"mcpServers": mcpServers,
	}

	// Use proper config directory and standardize naming to snake_case
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	
	// Convert config name to snake_case for cleaner file names
	cleanConfigName := strings.ReplaceAll(strings.ToLower(configData.Name), " ", "_")
	cleanConfigName = strings.ReplaceAll(cleanConfigName, "-", "_")
	
	// Create proper paths
	envDir := filepath.Join(configHome, "station", "environments", env.Name)
	templatePath := filepath.Join(envDir, cleanConfigName+".json")
	
	// Ensure environment directory exists
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Save template file to filesystem in standard MCP format
	templateBytes, err := json.MarshalIndent(standardConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template data: %w", err)
	}
	
	if err := os.WriteFile(templatePath, templateBytes, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	
	fmt.Printf("✅ Saved template file: %s\n", templatePath)

	// Create file config record in database with relative paths for portability
	relativeTemplatePath := filepath.Join("environments", env.Name, cleanConfigName+".json")
	relativeVariablesPath := filepath.Join("environments", env.Name, "variables.yml")
	
	fileConfigRecord := &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        cleanConfigName, // Use cleaned name
		TemplatePath:      relativeTemplatePath,
		VariablesPath:     relativeVariablesPath,
		TemplateHash:      h.calculateConfigHashFromBytes(templateBytes),
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

// uploadConfigLocalLoadTemplate creates file-based config saving original template but using processed config for tool discovery
func (h *LoadHandler) uploadConfigLocalLoadTemplate(originalConfig, processedConfig LoadMCPConfig, configName, environment string) error {
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

	// Save original template and variables separately, then use processed config for tool discovery
	err = h.createFileBasedConfigTemplateWithVariables(env.ID, configName, originalConfig, processedConfig, make(map[string]string), repos)
	if err != nil {
		return fmt.Errorf("failed to create file-based config: %w", err)
	}

	fmt.Printf("✅ Created file-based config: %s in environment %s\n", configName, environment)
	fmt.Printf("🔧 Discovered tools from %d MCP servers\n", len(processedConfig.MCPServers))

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
	return nil
}

// uploadConfigLocalLoadTemplateWithVariables creates file-based config saving template and variables separately
func (h *LoadHandler) uploadConfigLocalLoadTemplateWithVariables(originalConfig, processedConfig LoadMCPConfig, resolvedVariables map[string]string, configName, environment string) error {
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

	// Save original template and variables separately, then use processed config for tool discovery
	err = h.createFileBasedConfigTemplateWithVariables(env.ID, configName, originalConfig, processedConfig, resolvedVariables, repos)
	if err != nil {
		return fmt.Errorf("failed to create file-based config: %w", err)
	}

	fmt.Printf("✅ Created file-based config: %s in environment %s\n", configName, environment)
	fmt.Printf("🔧 Discovered tools from %d MCP servers\n", len(processedConfig.MCPServers))

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
	return nil
}

// createFileBasedConfigTemplate saves original template with placeholders but uses processed config for tool discovery
func (h *LoadHandler) createFileBasedConfigTemplate(envID int64, configName string, originalConfig, processedConfig LoadMCPConfig, repos *repositories.Repositories) error {
	// Save original template with placeholders for the template system
	templateConfig := map[string]interface{}{
		"mcpServers": originalConfig.MCPServers,
	}

	// Get environment name for file paths
	env, err := repos.Environments.GetByID(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Use proper config directory and standardize naming to snake_case
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	
	// Convert config name to snake_case for cleaner file names
	cleanConfigName := strings.ReplaceAll(strings.ToLower(configName), " ", "_")
	cleanConfigName = strings.ReplaceAll(cleanConfigName, "-", "_")
	
	// Create proper paths
	envDir := filepath.Join(configHome, "station", "environments", env.Name)
	templatePath := filepath.Join(envDir, cleanConfigName+".json")
	
	// Ensure environment directory exists
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Save template file with placeholders
	templateBytes, err := json.MarshalIndent(templateConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template data: %w", err)
	}
	
	if err := os.WriteFile(templatePath, templateBytes, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	
	fmt.Printf("✅ Saved template file: %s\n", templatePath)

	// Create file config record in database with relative paths for portability
	relativeTemplatePath := filepath.Join("environments", env.Name, cleanConfigName+".json")
	relativeVariablesPath := filepath.Join("environments", env.Name, "variables.yml")
	
	fileConfigRecord := &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        cleanConfigName, // Use cleaned name
		TemplatePath:      relativeTemplatePath,
		VariablesPath:     relativeVariablesPath,
		TemplateHash:      h.calculateConfigHashFromBytes(templateBytes),
		VariablesHash:     "",
	}

	_, err = repos.FileMCPConfigs.Create(fileConfigRecord)
	if err != nil {
		return fmt.Errorf("failed to create file config record: %w", err)
	}

	// Create a simple tool discovery service and discover tools using processed config
	toolDiscovery := services.NewToolDiscoveryService(repos)

	// Convert processed config to internal format for tool discovery
	servers := make(map[string]models.MCPServerConfig)
	for name, serverConfig := range processedConfig.MCPServers {
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
	
	// Use the existing tool discovery with the rendered config
	_, err = toolDiscovery.DiscoverToolsFromFileConfig(envID, configName, configData)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
		// Don't fail the entire operation if tool discovery fails
	}

	return nil
}
// createFileBasedConfigTemplateWithVariables saves template with placeholders and variables separately
func (h *LoadHandler) createFileBasedConfigTemplateWithVariables(envID int64, configName string, originalConfig, processedConfig LoadMCPConfig, resolvedVariables map[string]string, repos *repositories.Repositories) error {
	// Save original template with placeholders for the template system
	templateConfig := map[string]interface{}{
		"mcpServers": originalConfig.MCPServers,
	}

	// Get environment name for file paths
	env, err := repos.Environments.GetByID(envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Use proper config directory and standardize naming to snake_case
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	
	// Convert config name to snake_case for cleaner file names
	cleanConfigName := strings.ReplaceAll(strings.ToLower(configName), " ", "_")
	cleanConfigName = strings.ReplaceAll(cleanConfigName, "-", "_")
	
	// Create proper paths
	envDir := filepath.Join(configHome, "station", "environments", env.Name) 
	templatePath := filepath.Join(envDir, cleanConfigName+".json")
	variablesPath := filepath.Join(envDir, "variables.yml")
	
	// Ensure environment directory exists
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Save template file with placeholders
	templateBytes, err := json.MarshalIndent(templateConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template data: %w", err)
	}
	
	if err := os.WriteFile(templatePath, templateBytes, 0644); err != nil {
		return fmt.Errorf("failed to write template file: %w", err)
	}
	
	fmt.Printf("✅ Saved template file: %s\n", templatePath)

	// Save or update variables file
	if len(resolvedVariables) > 0 {
		err = h.saveVariablesToFile(variablesPath, resolvedVariables)
		if err != nil {
			return fmt.Errorf("failed to save variables: %w", err)
		}
		fmt.Printf("✅ Saved variables to: %s\n", variablesPath)
	}

	// Create file config record in database with relative paths for portability
	relativeTemplatePath := filepath.Join("environments", env.Name, cleanConfigName+".json")
	relativeVariablesPath := filepath.Join("environments", env.Name, "variables.yml")
	
	fileConfigRecord := &repositories.FileConfigRecord{
		EnvironmentID:     envID,
		ConfigName:        cleanConfigName, // Use cleaned name
		TemplatePath:      relativeTemplatePath,
		VariablesPath:     relativeVariablesPath,
		TemplateHash:      h.calculateConfigHashFromBytes(templateBytes),
		VariablesHash:     "",
	}

	_, err = repos.FileMCPConfigs.Create(fileConfigRecord)
	if err != nil {
		return fmt.Errorf("failed to create file config record: %w", err)
	}

	// Create a simple tool discovery service and discover tools using processed config
	toolDiscovery := services.NewToolDiscoveryService(repos)

	// Convert processed config to internal format for tool discovery
	servers := make(map[string]models.MCPServerConfig)
	for name, serverConfig := range processedConfig.MCPServers {
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
	
	// Use the existing tool discovery with the rendered config
	_, err = toolDiscovery.DiscoverToolsFromFileConfig(envID, configName, configData)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
		// Don't fail the entire operation if tool discovery fails
	}

	return nil
}

// saveVariablesToFile saves variables to a YAML file, merging with existing variables
func (h *LoadHandler) saveVariablesToFile(filePath string, newVariables map[string]string) error {
	// Load existing variables if file exists
	existingVariables := make(map[string]interface{})
	if data, err := os.ReadFile(filePath); err == nil {
		yaml.Unmarshal(data, &existingVariables)
	}

	// Merge new variables with existing ones (new variables take precedence)
	for key, value := range newVariables {
		existingVariables[key] = value
	}

	// Write updated variables back to file
	data, err := yaml.Marshal(existingVariables)
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write variables file: %w", err)
	}

	return nil
}