package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/firebase/genkit/go/genkit"
	tea "github.com/charmbracelet/bubbletea"
	
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

// runLoad implements the "stn load" command
func runLoad(cmd *cobra.Command, args []string) error {
	banner := bannerStyle.Render("📂 Loading MCP Configuration")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configName, _ := cmd.Flags().GetString("config-name")

	// Check if we have a GitHub URL as argument
	if len(args) > 0 && isGitHubURL(args[0]) {
		fmt.Println(infoStyle.Render("🔍 GitHub URL detected, starting discovery flow..."))
		return runGitHubDiscoveryFlow(args[0], environment, endpoint)
	}

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

// uploadGeneratedConfig uploads a configuration generated by the wizard
func uploadGeneratedConfig(configData *models.MCPConfigData, environment, endpoint string) error {

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(infoStyle.Render("🏠 Uploading to local database..."))
		return uploadConfigLocalWizard(configData, environment)
	} else if endpoint != "" {
		fmt.Println(infoStyle.Render("🌐 Uploading to: " + endpoint))
		return uploadConfigRemoteWizard(configData, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// uploadConfigLocalWizard uploads wizard config to local database
func uploadConfigLocalWizard(configData *models.MCPConfigData, environment string) error {
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
	
	_, err = toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, configData.Name)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
	} else {
		fmt.Println(successStyle.Render("✅ Tool discovery completed"))
	}

	showSuccessBanner("MCP Configuration Uploaded Successfully!")
	return nil
}

// uploadConfigRemoteWizard uploads wizard config to remote API
func uploadConfigRemoteWizard(configData *models.MCPConfigData, environment, endpoint string) error {
	// Get or create environment
	envID, err := getOrCreateEnvironmentRemote(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get/create environment: %w", err)
	}

	uploadRequest := struct {
		Name    string                         `json:"name"`
		Servers map[string]models.MCPServerConfig `json:"servers"`
	}{
		Name:    configData.Name,
		Servers: configData.Servers,
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

	showSuccessBanner("MCP Configuration Uploaded Successfully!")
	return nil
}

// isGitHubURL checks if the provided URL is a GitHub repository URL
func isGitHubURL(url string) bool {
	return strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "http://github.com/")
}

// runGitHubDiscoveryFlow handles the GitHub MCP server discovery flow
func runGitHubDiscoveryFlow(githubURL, environment, endpoint string) error {
	fmt.Printf("🔍 Analyzing GitHub repository: %s\n", githubURL)
	
	// Initialize Genkit service for discovery
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	// For discovery, we just need a basic Genkit app with web search
	genkitApp, err := genkit.Init(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	// Initialize GitHub discovery service and register web search tool
	discoveryService := services.NewGitHubDiscoveryService(genkitApp)
	webSearchTool := discoveryService.WebSearchTool()
	
	// Register the tool with Genkit (tool is already created with the genkit app instance)
	_ = webSearchTool // Tool is registered during creation
	
	fmt.Println(infoStyle.Render("🤖 Starting AI analysis of repository..."))
	
	// Discover MCP server configuration
	discovery, err := discoveryService.DiscoverMCPServer(context.Background(), githubURL)
	if err != nil {
		return fmt.Errorf("failed to discover MCP server configuration: %w", err)
	}

	fmt.Printf("✅ Discovered MCP server: %s\n", discovery.ServerName)
	fmt.Printf("📄 Description: %s\n", discovery.Description)
	fmt.Printf("🔧 Found %d configuration option(s)\n", len(discovery.Configurations))
	
	if len(discovery.RequiredEnv) > 0 {
		fmt.Printf("🔑 Requires %d environment variable(s)\n", len(discovery.RequiredEnv))
	}

	// Launch interactive configuration wizard
	fmt.Println("\n" + infoStyle.Render("🧙 Launching configuration wizard..."))
	
	wizard := services.NewConfigWizardModel(discovery)
	p := tea.NewProgram(wizard, tea.WithAltScreen())
	
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run configuration wizard: %w", err)
	}
	
	// Check if wizard was completed successfully
	wizardModel, ok := finalModel.(*services.ConfigWizardModel)
	if !ok {
		return fmt.Errorf("unexpected model type from wizard")
	}
	
	if !wizardModel.IsCompleted() {
		fmt.Println(infoStyle.Render("Configuration wizard cancelled"))
		return nil
	}
	
	// Get the final configuration
	finalConfig := wizardModel.GetFinalConfig()
	if finalConfig == nil {
		return fmt.Errorf("no configuration generated from wizard")
	}
	
	fmt.Printf("✅ Configuration generated: %s\n", finalConfig.Name)
	
	// Upload the configuration
	return uploadGeneratedConfig(finalConfig, environment, endpoint)
}