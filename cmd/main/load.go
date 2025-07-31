package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/crypto"
	"station/pkg/models"
)

// LoadMCPConfig configuration structure for load command
type LoadMCPConfig struct {
	MCPServers map[string]LoadMCPServerConfig `json:"mcpServers"`
}

type LoadMCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// runLoad implements the "station load" command
func runLoad(cmd *cobra.Command, args []string) error {
	banner := bannerStyle.Render("📂 Loading MCP Configuration")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configName, _ := cmd.Flags().GetString("config-name")

	// Look for MCP configuration file
	configFiles := []string{"mcp.json", ".mcp.json"}
	var configFile string
	var found bool

	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			configFile = file
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no MCP configuration file found. Looking for: %s", configFiles)
	}

	fmt.Printf("📄 Found config file: %s\n", configFile)

	// Read and parse MCP config
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var mcpConfig LoadMCPConfig
	if err := json.Unmarshal(data, &mcpConfig); err != nil {
		return fmt.Errorf("failed to parse MCP config: %w", err)
	}

	if len(mcpConfig.MCPServers) == 0 {
		return fmt.Errorf("no MCP servers found in configuration")
	}

	fmt.Printf("🔧 Found %d MCP server(s)\n", len(mcpConfig.MCPServers))

	// Use filename as default config name if not provided
	if configName == "" {
		configName = filepath.Base(configFile)
		if ext := filepath.Ext(configName); ext != "" {
			configName = configName[:len(configName)-len(ext)]
		}
	}

	fmt.Printf("📝 Config name: %s\n", configName)
	fmt.Printf("🌍 Environment: %s\n", environment)

	// Determine if we're in local mode - check config first, then endpoint flag
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(infoStyle.Render("🏠 Running in local mode"))
		return uploadConfigLocalLoad(mcpConfig, configName, environment)
	} else if endpoint != "" {
		fmt.Println(infoStyle.Render("🌐 Connecting to: " + endpoint))
		return uploadConfigRemoteLoad(mcpConfig, configName, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// uploadConfigLocalLoad uploads config to local database
func uploadConfigLocalLoad(mcpConfig LoadMCPConfig, configName, environment string) error {
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
	keyManager, err := crypto.NewKeyManagerFromEnv()
	if err != nil {
		return fmt.Errorf("failed to initialize key manager: %w", err)
	}

	mcpConfigSvc := services.NewMCPConfigService(repos, keyManager)

	// Find or create environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		// Create environment if it doesn't exist
		description := fmt.Sprintf("Environment for %s", environment)
		env, err = repos.Environments.Create(environment, &description)
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

	// Upload config
	savedConfig, err := mcpConfigSvc.UploadConfig(env.ID, configData)
	if err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d", 
		savedConfig.ConfigName, savedConfig.Version)))
	
	// Start tool discovery
	fmt.Println(infoStyle.Render("🔍 Starting tool discovery..."))
	toolDiscoveryService := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	
	_, err = toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, configName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
	} else {
		fmt.Println(successStyle.Render("✅ Tool discovery completed"))
	}

	showSuccessBanner("MCP Configuration Loaded Successfully!")
	return nil
}

// uploadConfigRemoteLoad uploads config to remote API
func uploadConfigRemoteLoad(mcpConfig LoadMCPConfig, configName, environment, endpoint string) error {
	// Get or create environment
	envID, err := getOrCreateEnvironmentRemote(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get/create environment: %w", err)
	}

	// Convert to API format
	servers := make(map[string]models.MCPServerConfig)
	for name, serverConfig := range mcpConfig.MCPServers {
		servers[name] = models.MCPServerConfig{
			Command: serverConfig.Command,
			Args:    serverConfig.Args,
			Env:     serverConfig.Env,
		}
	}

	uploadRequest := struct {
		Name    string                         `json:"name"`
		Servers map[string]models.MCPServerConfig `json:"servers"`
	}{
		Name:    configName,
		Servers: servers,
	}

	jsonData, err := json.Marshal(uploadRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Upload config
	url := fmt.Sprintf("%s/api/v1/environments/%d/mcp-configs", endpoint, envID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload config: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Config *models.MCPConfig `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Println(successStyle.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d", 
		result.Config.ConfigName, result.Config.Version)))

	fmt.Println(infoStyle.Render("🔍 Tool discovery started in background"))

	showSuccessBanner("MCP Configuration Loaded Successfully!")
	return nil
}

// getOrCreateEnvironmentRemote gets or creates an environment via remote API
func getOrCreateEnvironmentRemote(endpoint, envName string) (int64, error) {
	// Try to get existing environment
	envID, err := getEnvironmentID(endpoint, envName)
	if err == nil {
		return envID, nil
	}

	// Environment doesn't exist, create it
	createRequest := struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}{
		Name:        envName,
		Description: &[]string{fmt.Sprintf("Environment for %s", envName)}[0],
	}

	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/api/v1/environments", endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to create environment: status %d: %s", resp.StatusCode, string(body))
	}

	var createResult struct {
		Environment *models.Environment `json:"environment"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&createResult); err != nil {
		return 0, err
	}

	return createResult.Environment.ID, nil
}