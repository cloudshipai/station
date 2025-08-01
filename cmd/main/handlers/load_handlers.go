package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/firebase/genkit/go/genkit"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	tea "github.com/charmbracelet/bubbletea"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/services/turbo_wizard"
	"station/internal/theme"
	"station/pkg/crypto"
	"station/pkg/models"
)

// LoadMCPConfig configuration structure for load command
type LoadMCPConfig struct {
	Name        string                        `json:"name,omitempty"`
	Description string                        `json:"description,omitempty"`
	MCPServers  map[string]LoadMCPServerConfig `json:"mcpServers"`
	Templates   map[string]TemplateField      `json:"templates,omitempty"`
}

type LoadMCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type TemplateField struct {
	Description string `json:"description"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
	Default     string `json:"default,omitempty"`
	Help        string `json:"help,omitempty"`
}

// LoadHandler handles the "stn load" command
type LoadHandler struct {
	themeManager *theme.ThemeManager
}

func NewLoadHandler(themeManager *theme.ThemeManager) *LoadHandler {
	return &LoadHandler{themeManager: themeManager}
}

func (h *LoadHandler) RunLoad(cmd *cobra.Command, args []string) error {
	banner := getCLIStyles(h.themeManager).Banner.Render("📂 Loading MCP Configuration")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configName, _ := cmd.Flags().GetString("config-name")

	var configFile string
	var found bool

	// Check if we have a direct README URL as argument
	if len(args) > 0 && isDirectReadmeURL(args[0]) {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("📄 README URL detected, starting TurboTax-style flow..."))
		return h.runTurboTaxMCPFlow(args[0], environment, endpoint)
	}

	// Check if we have a GitHub URL as argument (legacy flow)
	if len(args) > 0 && isGitHubURL(args[0]) {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 GitHub URL detected, starting discovery flow..."))
		return h.runGitHubDiscoveryFlow(args[0], environment, endpoint)
	}

	// Check if we have a direct file argument
	if len(args) > 0 {
		if _, err := os.Stat(args[0]); err == nil {
			configFile = args[0]
			found = true
		} else {
			return fmt.Errorf("file not found: %s", args[0])
		}
	} else {
		// Look for MCP configuration file in current directory
		configFiles := []string{"mcp.json", ".mcp.json"}
		
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
	}

	fmt.Printf("📄 Found config file: %s\n", configFile)

	// Read and parse MCP config
	data, err := os.ReadFile(configFile)
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

	// Check if this is a template configuration and handle it
	if hasTemplates, missingValues := h.detectTemplates(&mcpConfig); hasTemplates {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🧩 Template configuration detected"))
		
		// Show credential form for missing values
		processedConfig, err := h.processTemplateConfig(&mcpConfig, missingValues)
		if err != nil {
			return fmt.Errorf("failed to process template: %w", err)
		}
		
		if processedConfig == nil {
			fmt.Println(getCLIStyles(h.themeManager).Info.Render("Template configuration cancelled"))
			return nil
		}
		
		// Use the processed config
		mcpConfig = *processedConfig
	}

	// Use filename as default config name if not provided
	if configName == "" {
		if mcpConfig.Name != "" {
			configName = mcpConfig.Name
		} else {
			configName = filepath.Base(configFile)
			if ext := filepath.Ext(configName); ext != "" {
				configName = configName[:len(configName)-len(ext)]
			}
		}
	}

	// Add unique ID suffix to prevent duplicates
	configName = h.generateUniqueConfigName(configName)

	fmt.Printf("📝 Config name: %s\n", configName)
	fmt.Printf("🌍 Environment: %s\n", environment)

	// Determine if we're in local mode - check config first, then endpoint flag
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Running in local mode"))
		return h.uploadConfigLocalLoad(mcpConfig, configName, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Connecting to: " + endpoint))
		return h.uploadConfigRemoteLoad(mcpConfig, configName, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

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
	
	// Create key manager from config file encryption key
	keyManager, err := createKeyManagerFromConfig()
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

	fmt.Println(getCLIStyles(h.themeManager).Success.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d", 
		savedConfig.ConfigName, savedConfig.Version)))
	
	// Start tool discovery
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 Starting tool discovery..."))
	toolDiscoveryService := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	
	_, err = toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, configName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
	} else {
		fmt.Println(getCLIStyles(h.themeManager).Success.Render("✅ Tool discovery completed"))
	}

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
	return nil
}

// uploadConfigRemoteLoad uploads config to remote API
func (h *LoadHandler) uploadConfigRemoteLoad(mcpConfig LoadMCPConfig, configName, environment, endpoint string) error {
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload config: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Config *models.MCPConfig `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Println(getCLIStyles(h.themeManager).Success.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d", 
		result.Config.ConfigName, result.Config.Version)))

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 Tool discovery started in background"))

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
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
		body, _ := io.ReadAll(resp.Body)
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
func (h *LoadHandler) uploadGeneratedConfig(configData *models.MCPConfigData, environment, endpoint string) error {

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Uploading to local database..."))
		return h.uploadConfigLocalWizard(configData, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Uploading to: " + endpoint))
		return h.uploadConfigRemoteWizard(configData, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
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
	
	// Create key manager from config file encryption key
	keyManager, err := createKeyManagerFromConfig()
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

	fmt.Println(getCLIStyles(h.themeManager).Success.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d", 
		savedConfig.ConfigName, savedConfig.Version)))
	
	// Start tool discovery
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 Starting tool discovery..."))
	toolDiscoveryService := services.NewToolDiscoveryService(repos, mcpConfigSvc)
	
	_, err = toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, configData.Name)
	if err != nil {
		fmt.Printf("⚠️  Warning: Tool discovery failed: %v\n", err)
	} else {
		fmt.Println(getCLIStyles(h.themeManager).Success.Render("✅ Tool discovery completed"))
	}

	showSuccessBanner("MCP Configuration Uploaded Successfully!", h.themeManager)
	return nil
}

// uploadConfigRemoteWizard uploads wizard config to remote API
func (h *LoadHandler) uploadConfigRemoteWizard(configData *models.MCPConfigData, environment, endpoint string) error {
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload config: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Config *models.MCPConfig `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Println(getCLIStyles(h.themeManager).Success.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d", 
		result.Config.ConfigName, result.Config.Version)))

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 Tool discovery started in background"))

	showSuccessBanner("MCP Configuration Uploaded Successfully!", h.themeManager)
	return nil
}

// isGitHubURL checks if the provided URL is a GitHub repository URL
func isGitHubURL(url string) bool {
	return strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "http://github.com/")
}

// isDirectReadmeURL checks if the provided URL is a direct README URL
func isDirectReadmeURL(url string) bool {
	return strings.Contains(url, "README.md") && 
		   (strings.HasPrefix(url, "https://raw.githubusercontent.com/") || 
		    strings.HasPrefix(url, "https://github.com/") ||
		    strings.HasPrefix(url, "http://"))
}

// convertToRawGitHubURL converts GitHub blob/tree URLs to raw content URLs
func convertToRawGitHubURL(url string) string {
	// Convert GitHub blob URLs to raw URLs
	// https://github.com/user/repo/blob/branch/path -> https://raw.githubusercontent.com/user/repo/branch/path
	if strings.Contains(url, "github.com") && strings.Contains(url, "/blob/") {
		// Replace github.com with raw.githubusercontent.com and remove /blob/
		url = strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		url = strings.Replace(url, "/blob/", "/", 1)
	}
	
	// Also handle /tree/ URLs (though less common for README files)
	if strings.Contains(url, "github.com") && strings.Contains(url, "/tree/") {
		url = strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		url = strings.Replace(url, "/tree/", "/", 1)
	}
	
	return url
}

// runTurboTaxMCPFlow handles the TurboTax-style MCP configuration flow
func (h *LoadHandler) runTurboTaxMCPFlow(readmeURL, environment, endpoint string) error {
	// Convert GitHub blob URLs to raw URLs
	readmeURL = convertToRawGitHubURL(readmeURL)
	fmt.Printf("📄 Analyzing README file: %s\n", readmeURL)
	
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

	// Initialize OpenAI plugin for AI model access
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("⚠️  OPENAI_API_KEY not set, using fallback parser..."))
		return h.runFallbackMCPExtraction(readmeURL, environment, endpoint)
	}
	
	openaiPlugin := &oai.OpenAI{APIKey: openaiAPIKey}
	
	// Initialize Genkit with OpenAI plugin for AI analysis
	genkitApp, err := genkit.Init(context.Background(), genkit.WithPlugins(openaiPlugin))
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	// Initialize GitHub discovery service
	discoveryService := services.NewGitHubDiscoveryService(genkitApp, openaiPlugin)
	
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🤖 Extracting MCP server blocks from README..."))
	
	// Extract all MCP server blocks from the README
	blocks, err := discoveryService.DiscoverMCPServerBlocks(context.Background(), readmeURL)
	if err != nil {
		return fmt.Errorf("failed to extract MCP server blocks: %w", err)
	}

	if len(blocks) == 0 {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("No MCP server configurations found in the README"))
		return nil
	}

	fmt.Printf("✅ Found %d MCP server configuration(s)\n", len(blocks))
	
	// Launch TurboTax-style wizard
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙 Launching TurboTax-style configuration wizard..."))
	
	// Check if we're in a TTY environment
	if _, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err != nil {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("📋 Non-TTY environment detected, showing configuration preview:"))
		
		// Show what configurations were found
		for i, block := range blocks {
			fmt.Printf("\n%d. %s - %s\n", i+1, block.ServerName, block.Description)
			fmt.Printf("   Configuration: %s\n", block.RawBlock)
		}
		
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("\n✨ In a terminal environment, this would launch an interactive TurboTax-style wizard!"))
		return nil
	}
	
	wizard := services.NewTurboWizardModel(blocks)
	p := tea.NewProgram(wizard, tea.WithAltScreen())
	
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run TurboTax wizard: %w", err)
	}
	
	// Check if wizard was completed successfully
	// The actual type returned is *turbo_wizard.TurboWizardModel
	wizardModel, ok := finalModel.(*turbo_wizard.TurboWizardModel)
	if !ok {
		fmt.Printf("Debug: received model type: %T\n", finalModel)
		return fmt.Errorf("unexpected model type from wizard: got %T, expected *turbo_wizard.TurboWizardModel", finalModel)
	}
	
	if wizardModel.IsCancelled() {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard cancelled"))
		return nil
	}
	
	if !wizardModel.IsCompleted() {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard not completed"))
		return nil
	}
	
	// Get the final configuration
	finalConfig := wizardModel.GetFinalMCPConfig()
	if finalConfig == nil {
		return fmt.Errorf("no configuration generated from wizard")
	}
	
	fmt.Printf("✅ Configuration generated with %d server(s)\n", len(finalConfig.Servers))
	
	// Upload the configuration
	return h.uploadGeneratedConfig(finalConfig, environment, endpoint)
}

// runGitHubDiscoveryFlow handles the GitHub MCP server discovery flow
func (h *LoadHandler) runGitHubDiscoveryFlow(githubURL, environment, endpoint string) error {
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

	// Initialize OpenAI plugin for AI model access
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable is required for GitHub discovery")
	}
	
	openaiPlugin := &oai.OpenAI{APIKey: openaiAPIKey}
	
	// Initialize Genkit with OpenAI plugin for AI analysis
	genkitApp, err := genkit.Init(context.Background(), genkit.WithPlugins(openaiPlugin))
	if err != nil {
		return fmt.Errorf("failed to initialize Genkit: %w", err)
	}
	
	// Initialize GitHub discovery service
	discoveryService := services.NewGitHubDiscoveryService(genkitApp, openaiPlugin)
	
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🤖 Starting AI analysis of repository..."))
	
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
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙 Launching configuration wizard..."))
	
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
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard cancelled"))
		return nil
	}
	
	// Get the final configuration
	finalConfig := wizardModel.GetFinalConfig()
	if finalConfig == nil {
		return fmt.Errorf("no configuration generated from wizard")
	}
	
	fmt.Printf("✅ Configuration generated: %s\n", finalConfig.Name)
	
	// Upload the configuration
	return h.uploadGeneratedConfig(finalConfig, environment, endpoint)
}

// createKeyManagerFromConfig creates a key manager using the encryption key from config file
func createKeyManagerFromConfig() (*crypto.KeyManager, error) {
	// Get encryption key from viper (config file)
	encryptionKey := viper.GetString("encryption_key")
	return crypto.NewKeyManagerFromConfig(encryptionKey)
}

// runFallbackMCPExtraction handles MCP extraction without AI when OPENAI_API_KEY is not available
func (h *LoadHandler) runFallbackMCPExtraction(readmeURL, environment, endpoint string) error {
	fmt.Printf("🔍 Fetching README content from: %s\n", readmeURL)
	
	// Fetch README content directly
	resp, err := http.Get(readmeURL)
	if err != nil {
		return fmt.Errorf("failed to fetch README: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP error %d fetching README", resp.StatusCode)
	}
	
	// Read content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read README content: %w", err)
	}
	
	content := string(body)
	
	// Parse MCP server blocks using simple pattern matching
	blocks := extractMCPBlocksFromContent(content)
	
	if len(blocks) == 0 {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("No MCP server configurations found in the README"))
		return nil
	}
	
	fmt.Printf("✅ Found %d MCP server configuration(s)\n", len(blocks))
	
	// Launch TurboTax-style wizard
	fmt.Println("\n" + getCLIStyles(h.themeManager).Info.Render("🧙 Launching TurboTax-style configuration wizard..."))
	
	config, selectedEnv, err := services.RunTurboWizardWithTheme(blocks, []string{"development", "staging", "production"}, h.themeManager)
	if err != nil {
		return fmt.Errorf("failed to run TurboTax wizard: %w", err)
	}
	
	if config == nil {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("Configuration wizard cancelled"))
		return nil
	}
	
	// Use selected environment or provided environment
	if environment == "" {
		environment = selectedEnv
	}
	
	fmt.Printf("✅ Configuration generated with %d server(s) for %s environment\n", len(config.Servers), environment)
	
	// Upload the configuration (implement this function if needed)
	return h.uploadGeneratedConfig(config, environment, endpoint)
}

// extractMCPBlocksFromContent extracts MCP server blocks using simple pattern matching
func extractMCPBlocksFromContent(content string) []services.MCPServerBlock {
	blocks := []services.MCPServerBlock{}
	
	// Look for JSON blocks containing mcpServers
	jsonBlockPattern := regexp.MustCompile(`\{[^{}]*"mcpServers"[^{}]*\{[^{}]*\}[^{}]*\}`)
	matches := jsonBlockPattern.FindAllString(content, -1)
	
	for i, match := range matches {
		// Try to parse the JSON
		var configData map[string]interface{}
		if err := json.Unmarshal([]byte(match), &configData); err != nil {
			continue
		}
		
		// Extract server configurations
		if mcpServers, ok := configData["mcpServers"].(map[string]interface{}); ok {
			for serverName, serverConfig := range mcpServers {
				if serverMap, ok := serverConfig.(map[string]interface{}); ok {
					description := fmt.Sprintf("MCP server configuration #%d", i+1)
					
					// Try to determine description from server config
					if command, hasCommand := serverMap["command"].(string); hasCommand {
						description = fmt.Sprintf("STDIO server using %s", command)
					} else if url, hasURL := serverMap["url"].(string); hasURL {
						description = fmt.Sprintf("HTTP server at %s", url)
					}
					
					blocks = append(blocks, services.MCPServerBlock{
						ServerName:  serverName,
						Description: description,
						RawBlock:    match,
					})
				}
			}
		}
	}
	
	return blocks
}

// detectTemplates checks if the configuration has template placeholders
func (h *LoadHandler) detectTemplates(config *LoadMCPConfig) (bool, []string) {
	var missingValues []string
	hasTemplates := false
	
	// Check if there's a templates section
	if len(config.Templates) > 0 {
		hasTemplates = true
	}
	
	// Scan all environment variables for template placeholders
	templatePattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	
	for _, serverConfig := range config.MCPServers {
		for key, value := range serverConfig.Env {
			matches := templatePattern.FindAllStringSubmatch(value, -1)
			for _, match := range matches {
				if len(match) > 1 {
					placeholder := match[1]
					hasTemplates = true
					
					// Check if we have a template definition for this placeholder
					if _, exists := config.Templates[placeholder]; exists {
						missingValues = append(missingValues, placeholder)
					} else {
						// Create a basic template for unknown placeholders
						if config.Templates == nil {
							config.Templates = make(map[string]TemplateField)
						}
						config.Templates[placeholder] = TemplateField{
							Description: fmt.Sprintf("Value for %s in %s", placeholder, key),
							Type:        "string",
							Required:    true,
						}
						missingValues = append(missingValues, placeholder)
					}
				}
			}
		}
	}
	
	return hasTemplates, missingValues
}

// processTemplateConfig shows credential forms and processes templates
func (h *LoadHandler) processTemplateConfig(config *LoadMCPConfig, missingValues []string) (*LoadMCPConfig, error) {
	if len(missingValues) == 0 {
		return config, nil
	}
	
	fmt.Printf("🔑 Configuration requires %d credential(s):\n", len(missingValues))
	
	// Collect values from user
	values := make(map[string]string)
	
	for _, placeholder := range missingValues {
		template := config.Templates[placeholder]
		
		fmt.Printf("\n📝 %s\n", template.Description)
		if template.Help != "" {
			fmt.Printf("💡 %s\n", template.Help)
		}
		
		var value string
		if template.Default != "" {
			fmt.Printf("Enter value (default: %s): ", template.Default)
		} else if template.Required {
			fmt.Printf("Enter value (required): ")
		} else {
			fmt.Printf("Enter value (optional): ")
		}
		
		// Read input
		var input string
		if _, err := fmt.Scanln(&input); err != nil && template.Required {
			return nil, fmt.Errorf("input required for %s", placeholder)
		}
		
		if input == "" && template.Default != "" {
			value = template.Default
		} else if input == "" && template.Required {
			return nil, fmt.Errorf("value required for %s", placeholder)
		} else {
			value = input
		}
		
		values[placeholder] = value
		
		if template.Sensitive {
			fmt.Printf("✅ Secured credential for %s\n", placeholder)
		} else {
			fmt.Printf("✅ Set %s = %s\n", placeholder, value)
		}
	}
	
	// Process templates by replacing placeholders
	processedConfig := *config
	
	for serverName, serverConfig := range processedConfig.MCPServers {
		for envKey, envValue := range serverConfig.Env {
			processedValue := envValue
			for placeholder, value := range values {
				processedValue = strings.ReplaceAll(processedValue, fmt.Sprintf("{{%s}}", placeholder), value)
			}
			serverConfig.Env[envKey] = processedValue
		}
		processedConfig.MCPServers[serverName] = serverConfig
	}
	
	return &processedConfig, nil
}

// generateUniqueConfigName adds a timestamp suffix to prevent duplicates
func (h *LoadHandler) generateUniqueConfigName(baseName string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", baseName, timestamp)
}

// Helper functions are now in common.go