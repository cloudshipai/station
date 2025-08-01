package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/crypto"
	"station/pkg/models"
	"station/internal/theme"
)

// MCPHandler handles MCP-related commands
type MCPHandler struct {
	themeManager *theme.ThemeManager
}

func NewMCPHandler(themeManager *theme.ThemeManager) *MCPHandler {
	return &MCPHandler{themeManager: themeManager}
}

// RunMCPList implements the "station mcp list" command
func (h *MCPHandler) RunMCPList(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📋 MCP Configurations")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Listing local configurations"))
		return h.listMCPConfigsLocal(environment)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing configurations from: " + endpoint))
		return h.listMCPConfigsRemote(environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// listMCPConfigsLocal lists MCP configs from local database
func (h *MCPHandler) listMCPConfigsLocal(environment string) error {
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

	// Find environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found", environment)
	}

	// List configs
	configs, err := repos.MCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list configurations: %w", err)
	}

	if len(configs) == 0 {
		fmt.Println("• No configurations found")
		return nil
	}

	fmt.Printf("Found %d configuration(s):\n", len(configs))
	for _, config := range configs {
		fmt.Printf("• %s v%d (ID: %d) - %s\n", 
			config.ConfigName, config.Version, config.ID, 
			config.CreatedAt.Format("Jan 2, 2006 15:04"))
	}

	return nil
}

// listMCPConfigsRemote lists MCP configs from remote API
func (h *MCPHandler) listMCPConfigsRemote(environment, endpoint string) error {
	// Get environment ID
	envID, err := getEnvironmentID(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get environment ID: %w", err)
	}

	// List configs
	url := fmt.Sprintf("%s/api/v1/environments/%d/mcp-configs", endpoint, envID)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to list configurations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list configurations: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Configs []*models.MCPConfig `json:"configs"`
		Count   int                 `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("• No configurations found")
		return nil
	}

	fmt.Printf("Found %d configuration(s):\n", result.Count)
	for _, config := range result.Configs {
		fmt.Printf("• %s v%d (ID: %d) - %s\n", 
			config.ConfigName, config.Version, config.ID, 
			config.CreatedAt.Format("Jan 2, 2006 15:04"))
	}

	return nil
}

// RunMCPTools implements the "station mcp tools" command
func (h *MCPHandler) RunMCPTools(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🔧 MCP Tools")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	filter, _ := cmd.Flags().GetString("filter")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(styles.Info.Render("🏠 Listing local tools"))
		return h.listMCPToolsLocal(environment, filter)
	} else if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing tools from: " + endpoint))
		return h.listMCPToolsRemote(environment, filter, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// listMCPToolsLocal lists MCP tools from local database
func (h *MCPHandler) listMCPToolsLocal(environment, filter string) error {
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

	// Find environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found", environment)
	}

	// List tools
	tools, err := repos.MCPTools.GetByEnvironmentID(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Apply filter if provided
	if filter != "" {
		filteredTools := make([]*models.MCPTool, 0)
		filterLower := strings.ToLower(filter)
		
		for _, tool := range tools {
			if strings.Contains(strings.ToLower(tool.Name), filterLower) ||
				strings.Contains(strings.ToLower(tool.Description), filterLower) {
				filteredTools = append(filteredTools, tool)
			}
		}
		tools = filteredTools
		fmt.Printf("Filter: %s\n", filter)
	}

	if len(tools) == 0 {
		fmt.Println("• No tools found")
		return nil
	}

	fmt.Printf("Found %d tool(s):\n", len(tools))
	styles := getCLIStyles(h.themeManager)
	for _, tool := range tools {
		fmt.Printf("• %s - %s\n", styles.Success.Render(tool.Name), tool.Description)
		fmt.Printf("  Server ID: %d\n", tool.MCPServerID)
	}

	return nil
}

// listMCPToolsRemote lists MCP tools from remote API
func (h *MCPHandler) listMCPToolsRemote(environment, filter, endpoint string) error {
	// Get environment ID
	envID, err := getEnvironmentID(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get environment ID: %w", err)
	}

	// Build URL with filter
	url := fmt.Sprintf("%s/api/v1/environments/%d/tools", endpoint, envID)
	if filter != "" {
		url += "?filter=" + filter
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list tools: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Tools  []*models.MCPToolWithDetails `json:"tools"`
		Count  int                          `json:"count"`
		Filter string                       `json:"filter"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Filter != "" {
		fmt.Printf("Filter: %s\n", result.Filter)
	}

	if result.Count == 0 {
		fmt.Println("• No tools found")
		return nil
	}

	fmt.Printf("Found %d tool(s):\n", result.Count)
	styles := getCLIStyles(h.themeManager)
	for _, tool := range result.Tools {
		fmt.Printf("• %s - %s\n", styles.Success.Render(tool.Name), tool.Description)
		fmt.Printf("  Config: %s v%d | Server: %s\n", 
			tool.ConfigName, tool.ConfigVersion, tool.ServerName)
	}

	return nil
}

// AddServerToConfig adds a single server to an existing MCP configuration (public method)
func (h *MCPHandler) AddServerToConfig(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	return h.addServerToConfig(configID, serverName, command, args, envVars, environment, endpoint)
}

// addServerToConfig adds a single server to an existing MCP configuration
func (h *MCPHandler) addServerToConfig(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Running in local mode"))
		return h.addServerToConfigLocal(configID, serverName, command, args, envVars, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Connecting to: " + endpoint))
		return h.addServerToConfigRemote(configID, serverName, command, args, envVars, environment, endpoint)
	}
	
	// Default to local mode
	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 No endpoint specified, using local mode"))
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
	mcpConfigService := services.NewMCPConfigService(repos, keyManager)

	// Find environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return "", fmt.Errorf("environment '%s' not found", environment)
	}

	// Find config (try by name first, then by ID)
	var config *models.MCPConfig
	if configByName, err := repos.MCPConfigs.GetLatestByName(env.ID, configID); err == nil {
		config = configByName
	} else {
		// Try parsing as ID
		if id, parseErr := strconv.ParseInt(configID, 10, 64); parseErr == nil {
			if configByID, err := repos.MCPConfigs.GetByID(id); err == nil {
				config = configByID
			}
		}
	}

	if config == nil {
		return "", fmt.Errorf("config '%s' not found", configID)
	}

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
}

// addServerToConfigRemote adds server to remote configuration
func (h *MCPHandler) addServerToConfigRemote(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	// This would require a new API endpoint for adding servers to existing configs
	// For now, return an informative message
	return "", fmt.Errorf("remote server addition not yet implemented - use local mode or upload full config")
}