package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

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

	if environment != "" {
		// List configs for specific environment
		return h.listConfigsForEnvironment(repos, environment)
	}

	// List configs for all environments, grouped by environment
	return h.listConfigsAllEnvironments(repos)
}

// listConfigsForEnvironment lists configs for a specific environment
func (h *MCPHandler) listConfigsForEnvironment(repos *repositories.Repositories, environment string) error {
	// Find environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found", environment)
	}

	// List file-based configs
	configs, err := repos.FileMCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		return fmt.Errorf("failed to list configurations: %w", err)
	}

	if len(configs) == 0 {
		fmt.Printf("• No configurations found in environment '%s'\n", environment)
		return nil
	}

	fmt.Printf("Found %d configuration(s) in environment '%s':\n\n", len(configs), environment)
	h.printConfigTable(configs, false) // false = don't show environment column
	return nil
}

// listConfigsAllEnvironments lists configs for all environments, grouped by environment
func (h *MCPHandler) listConfigsAllEnvironments(repos *repositories.Repositories) error {
	// Get all environments
	environments, err := repos.Environments.List()
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	totalConfigs := 0
	var allConfigs []*repositories.FileConfigRecord
	var configsWithEnv []struct {
		Config *repositories.FileConfigRecord
		EnvName string
	}

	// Collect configs from all environments
	for _, env := range environments {
		configs, err := repos.FileMCPConfigs.ListByEnvironment(env.ID)
		if err != nil {
			fmt.Printf("Warning: failed to list configs for environment '%s': %v\n", env.Name, err)
			continue
		}
		
		for _, config := range configs {
			allConfigs = append(allConfigs, config)
			configsWithEnv = append(configsWithEnv, struct {
				Config *repositories.FileConfigRecord
				EnvName string
			}{config, env.Name})
		}
		totalConfigs += len(configs)
	}

	if totalConfigs == 0 {
		fmt.Println("• No configurations found in any environment")
		return nil
	}

	fmt.Printf("Found %d configuration(s) across all environments:\n\n", totalConfigs)
	h.printConfigTableWithEnvironments(configsWithEnv)
	return nil
}

// printConfigTable prints a table of configs without environment column
func (h *MCPHandler) printConfigTable(configs []*repositories.FileConfigRecord, showEnv bool) {
	if showEnv {
		fmt.Printf("┌──────────────────────────────────────────────────────────────────────────────────┐\n")
		fmt.Printf("│ %-4s │ %-35s │ %-8s │ %-12s │ %-14s │\n", 
			"ID", "Configuration Name", "Version", "Environment", "Created")
		fmt.Printf("├──────────────────────────────────────────────────────────────────────────────────┤\n")
	} else {
		fmt.Printf("┌──────────────────────────────────────────────────────────────────────┐\n")
		fmt.Printf("│ %-4s │ %-40s │ %-8s │ %-14s │\n", 
			"ID", "Configuration Name", "Version", "Created")
		fmt.Printf("├──────────────────────────────────────────────────────────────────────┤\n")
	}
	
	// Print each config
	for _, config := range configs {
		if showEnv {
			// This version won't be used since we have a separate method for env display
		} else {
			fmt.Printf("│ %-4d │ %-40s │ %-9s │ %-14s │\n", 
				config.ID, 
				truncateString(config.ConfigName, 40),
				"file-based", 
				config.CreatedAt.Format("Jan 2 15:04"))
		}
	}
	
	if showEnv {
		fmt.Printf("└──────────────────────────────────────────────────────────────────────────────────┘\n")
	} else {
		fmt.Printf("└──────────────────────────────────────────────────────────────────────┘\n")
	}
}

// printConfigTableWithEnvironments prints configs grouped by environment
func (h *MCPHandler) printConfigTableWithEnvironments(configsWithEnv []struct {
	Config *repositories.FileConfigRecord
	EnvName string
}) {
	fmt.Printf("┌──────────────────────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│ %-4s │ %-35s │ %-8s │ %-12s │ %-14s │\n", 
		"ID", "Configuration Name", "Version", "Environment", "Created")
	fmt.Printf("├──────────────────────────────────────────────────────────────────────────────────┤\n")
	
	// Print each config with environment
	for _, item := range configsWithEnv {
		config := item.Config
		envName := item.EnvName
		fmt.Printf("│ %-4d │ %-35s │ %-9s │ %-12s │ %-14s │\n", 
			config.ID, 
			truncateString(config.ConfigName, 35),
			"file-based",
			truncateString(envName, 12),
			config.CreatedAt.Format("Jan 2 15:04"))
	}
	
	fmt.Printf("└──────────────────────────────────────────────────────────────────────────────────┘\n")
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

	// Find environment using hybrid approach (supports file-based environments)
	envID, err := h.getOrCreateEnvironmentID(repos, environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", environment, err)
	}

	// List tools
	tools, err := repos.MCPTools.GetByEnvironmentID(envID)
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

	fmt.Printf("Found %d tool(s):\n\n", len(tools))
	styles := getCLIStyles(h.themeManager)
	
	// Group tools by server
	serverTools := make(map[int64][]*models.MCPTool)
	for _, tool := range tools {
		serverTools[tool.MCPServerID] = append(serverTools[tool.MCPServerID], tool)
	}
	
	// Display tools grouped by server
	for serverID, toolList := range serverTools {
		// Get server details
		server, err := repos.MCPServers.GetByID(serverID)
		if err != nil {
			fmt.Printf("🔧 Server ID %d (Unknown)\n", serverID)
		} else {
			fmt.Printf("🔧 %s (Server ID: %d)\n", styles.Info.Render(server.Name), serverID)
		}
		
		// Display tools for this server
		for _, tool := range toolList {
			fmt.Printf("  • %s - %s\n", styles.Success.Render(tool.Name), tool.Description)
		}
		fmt.Println() // Empty line between servers
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