package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/crypto"
	"station/pkg/models"
)


// Spinner model for loading states
type spinnerModel struct {
	spinner    spinner.Model
	message    string
	finished   bool
	success    bool
	result     string
	err        error
	
	// Fields for add-server command
	configID   string
	serverName string
	endpoint   string
	env        string
	command    string
	args       []string
	envVars    map[string]string
}

// Interactive form model for MCP Add command
type mcpAddFormModel struct {
	inputs    []textinput.Model
	focused   int
	cancelled bool
	done      bool
	
	// Form data
	configID     string
	serverName   string
	command      string
	args         []string
	envVars      map[string]string
	environment  string
	endpoint     string
}

func newMCPAddForm(endpoint, environment string) *mcpAddFormModel {
	m := &mcpAddFormModel{
		inputs:      make([]textinput.Model, 4), // 4 main fields
		environment: environment,
		endpoint:    endpoint,
		envVars:     make(map[string]string),
	}
	
	// Config ID input
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "my-config or config-123"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 50
	m.inputs[0].Width = 40
	m.inputs[0].Prompt = focusedStyle.Render("▶ ")
	
	// Server Name input
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "filesystem"
	m.inputs[1].CharLimit = 50
	m.inputs[1].Width = 40
	m.inputs[1].Prompt = blurredStyle.Render("▶ ")
	
	// Command input
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "npx"
	m.inputs[2].CharLimit = 100
	m.inputs[2].Width = 40
	m.inputs[2].Prompt = blurredStyle.Render("▶ ")
	
	// Args input
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "-y @modelcontextprotocol/server-filesystem /path"
	m.inputs[3].CharLimit = 200
	m.inputs[3].Width = 40
	m.inputs[3].Prompt = blurredStyle.Render("▶ ")
	
	return m
}

func (m *mcpAddFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *mcpAddFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "enter":
			if m.focused == len(m.inputs)-1 {
				// Last field, submit form
				m.configID = m.inputs[0].Value()
				m.serverName = m.inputs[1].Value()
				m.command = m.inputs[2].Value()
				
				// Parse args
				if argsStr := m.inputs[3].Value(); argsStr != "" {
					m.args = strings.Fields(argsStr)
				}
				
				// Validate required fields
				if m.configID == "" || m.serverName == "" || m.command == "" {
					// Don't submit if required fields are empty
					return m, nil
				}
				
				m.done = true
				return m, tea.Quit
			}
			
			// Move to next field
			m.focused++
			m.updateFocus()
			return m, nil
		case "shift+tab", "up":
			if m.focused > 0 {
				m.focused--
				m.updateFocus()
			}
			return m, nil
		case "tab", "down":
			if m.focused < len(m.inputs)-1 {
				m.focused++
				m.updateFocus()
			}
			return m, nil
		}
	}
	
	// Update the focused input
	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

func (m *mcpAddFormModel) updateFocus() {
	for i := range m.inputs {
		if i == m.focused {
			m.inputs[i].Focus()
			m.inputs[i].Prompt = focusedStyle.Render("▶ ")
			m.inputs[i].TextStyle = focusedStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].Prompt = blurredStyle.Render("▶ ")
			m.inputs[i].TextStyle = noStyle
		}
	}
}

func (m *mcpAddFormModel) View() string {
	if m.done {
		return successStyle.Render("✓ Configuration collected!")
	}
	
	var b strings.Builder
	
	// Title with retro styling
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#bb9af7")).
		Background(lipgloss.Color("#1a1b26")).
		Bold(true).
		Padding(0, 2).
		Render("🎛️  MCP Server Configuration")
	b.WriteString(title + "\n\n")
	
	// Form fields with labels
	fields := []string{
		"Config ID (name or id):",
		"Server Name:",
		"Command:",
		"Arguments:",
	}
	
	for i, field := range fields {
		var style lipgloss.Style
		if i == m.focused {
			style = focusedStyle
		} else {
			style = blurredStyle
		}
		
		b.WriteString(style.Render(field) + "\n")
		b.WriteString(m.inputs[i].View() + "\n\n")
	}
	
	// Help text at bottom
	help := helpStyle.Render("↑/↓: navigate • enter: next/submit • ctrl+c: cancel")
	
	// Validation message
	validation := ""
	if m.inputs[0].Value() == "" || m.inputs[1].Value() == "" || m.inputs[2].Value() == "" {
		validation = errorStyle.Render("⚠ All fields except arguments are required")
	} else {
		validation = successStyle.Render("✓ Ready to submit")
	}
	
	form := formStyle.Render(b.String() + validation + "\n\n" + help)
	
	return form
}

func (m spinnerModel) Init() tea.Cmd {
	// Check if this is an add-server operation
	if m.configID != "" && m.serverName != "" {
		return tea.Batch(m.spinner.Tick, func() tea.Msg {
			result, err := addServerToConfig(m.configID, m.serverName, m.command, m.args, m.envVars, m.env, m.endpoint)
			return finishedMsg{
				success: err == nil,
				result:  result,
				err:     err,
			}
		})
	}
	
	return m.spinner.Tick
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case finishedMsg:
		m.finished = true
		m.success = msg.success
		m.result = msg.result
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.finished {
		if m.success {
			return successStyle.Render("✅ " + m.result)
		} else {
			return errorStyle.Render("❌ " + m.err.Error())
		}
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

type finishedMsg struct {
	success bool
	result  string
	err     error
}

// MCP Config structure for JSON parsing
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
	Servers    map[string]MCPServerConfig `json:"servers"` // Alternative format
}

type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}








// loadStationConfig loads the Station configuration
func loadStationConfig() (*config.Config, error) {
	encryptionKey := viper.GetString("encryption_key")
	if encryptionKey == "" {
		return nil, fmt.Errorf("no encryption key found. Run 'station init' first")
	}

	return &config.Config{
		DatabaseURL:   viper.GetString("database_url"),
		APIPort:       viper.GetInt("api_port"),
		SSHPort:       viper.GetInt("ssh_port"),
		MCPPort:       viper.GetInt("mcp_port"),
	}, nil
}


// getEnvironmentID gets or creates an environment and returns its ID
func getEnvironmentID(endpoint, environment string) (int64, error) {
	// First try to get the environment by name
	url := fmt.Sprintf("%s/api/v1/environments", endpoint)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to list environments: status %d", resp.StatusCode)
	}

	var result struct {
		Environments []*models.Environment `json:"environments"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	// Look for existing environment
	for _, env := range result.Environments {
		if env.Name == environment {
			return env.ID, nil
		}
	}

	// Environment doesn't exist, create it
	payload := map[string]interface{}{
		"name": environment,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	resp, err = http.Post(url, "application/json", bytes.NewBuffer(jsonData))
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

// runMCPList implements the "station mcp list" command
func runMCPList(cmd *cobra.Command, args []string) error {
	banner := bannerStyle.Render("📋 MCP Configurations")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(infoStyle.Render("🏠 Listing local configurations"))
		return listMCPConfigsLocal(environment)
	} else if endpoint != "" {
		fmt.Println(infoStyle.Render("🌐 Listing configurations from: " + endpoint))
		return listMCPConfigsRemote(environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// listMCPConfigsLocal lists MCP configs from local database
func listMCPConfigsLocal(environment string) error {
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
func listMCPConfigsRemote(environment, endpoint string) error {
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
		body, _ := ioutil.ReadAll(resp.Body)
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

// runMCPTools implements the "station mcp tools" command
func runMCPTools(cmd *cobra.Command, args []string) error {
	banner := bannerStyle.Render("🔧 MCP Tools")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	filter, _ := cmd.Flags().GetString("filter")

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(infoStyle.Render("🏠 Listing local tools"))
		return listMCPToolsLocal(environment, filter)
	} else if endpoint != "" {
		fmt.Println(infoStyle.Render("🌐 Listing tools from: " + endpoint))
		return listMCPToolsRemote(environment, filter, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}

// listMCPToolsLocal lists MCP tools from local database
func listMCPToolsLocal(environment, filter string) error {
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
	for _, tool := range tools {
		fmt.Printf("• %s - %s\n", successStyle.Render(tool.Name), tool.Description)
		fmt.Printf("  Server ID: %d\n", tool.MCPServerID)
	}

	return nil
}

// listMCPToolsRemote lists MCP tools from remote API
func listMCPToolsRemote(environment, filter, endpoint string) error {
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
		body, _ := ioutil.ReadAll(resp.Body)
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
	for _, tool := range result.Tools {
		fmt.Printf("• %s - %s\n", successStyle.Render(tool.Name), tool.Description)
		fmt.Printf("  Config: %s v%d | Server: %s\n", 
			tool.ConfigName, tool.ConfigVersion, tool.ServerName)
	}

	return nil
}

// runMCPAdd implements the "station mcp add" command
func runMCPAdd(cmd *cobra.Command, args []string) error {
	// Check if interactive mode is requested
	interactive, _ := cmd.Flags().GetBool("interactive")
	
	if interactive {
		return runMCPAddInteractive(cmd, args)
	}
	
	return runMCPAddFlags(cmd, args)
}

// runMCPAddFlags handles flag-based mode
func runMCPAddFlags(cmd *cobra.Command, args []string) error {
	// Get flags
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configID, _ := cmd.Flags().GetString("config-id")
	serverName, _ := cmd.Flags().GetString("server-name")
	command, _ := cmd.Flags().GetString("command")
	argsSlice, _ := cmd.Flags().GetStringSlice("args")
	envVars, _ := cmd.Flags().GetStringToString("env")

	// Validate required flags
	if configID == "" {
		return fmt.Errorf("--config-id is required")
	}
	if serverName == "" {
		return fmt.Errorf("--server-name is required")
	}
	if command == "" {
		return fmt.Errorf("--command is required")
	}

	// Show banner
	banner := bannerStyle.Render("🔧 Add MCP Server to Configuration")
	fmt.Println(banner)

	// Create spinner model
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))

	model := spinnerModel{
		spinner:    s,
		message:    fmt.Sprintf("Adding server '%s' to configuration '%s'...", serverName, configID),
		configID:   configID,
		serverName: serverName,
		endpoint:   endpoint,
		env:        environment,
		command:    command,
		args:       argsSlice,
		envVars:    envVars,
	}

	// Start the spinner
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run spinner: %w", err)
	}

	// Check results
	final := finalModel.(spinnerModel)
	if final.err != nil {
		fmt.Println(errorStyle.Render("❌ Failed to add server: " + final.err.Error()))
		return final.err
	}

	// Show success banner
	showSuccessBanner(fmt.Sprintf("Server '%s' successfully added to configuration!", serverName))
	fmt.Printf("Result: %s\n", final.result)

	return nil
}

// addServerToConfig adds a single server to an existing MCP configuration
func addServerToConfig(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")
	
	if isLocal {
		fmt.Println(infoStyle.Render("🏠 Running in local mode"))
		return addServerToConfigLocal(configID, serverName, command, args, envVars, environment)
	} else if endpoint != "" {
		fmt.Println(infoStyle.Render("🌐 Connecting to: " + endpoint))
		return addServerToConfigRemote(configID, serverName, command, args, envVars, environment, endpoint)
	}
	
	// Default to local mode
	fmt.Println(infoStyle.Render("🏠 No endpoint specified, using local mode"))
	return addServerToConfigLocal(configID, serverName, command, args, envVars, environment)
}

// addServerToConfigLocal adds server to local configuration
func addServerToConfigLocal(configID, serverName, command string, args []string, envVars map[string]string, environment string) (string, error) {
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
func addServerToConfigRemote(configID, serverName, command string, args []string, envVars map[string]string, environment, endpoint string) (string, error) {
	// This would require a new API endpoint for adding servers to existing configs
	// For now, return an informative message
	return "", fmt.Errorf("remote server addition not yet implemented - use local mode or upload full config")
}

// runMCPAddInteractive handles interactive mode with beautiful forms
func runMCPAddInteractive(cmd *cobra.Command, args []string) error {
	// Show retro banner
	retorBanner := bannerStyle.Render("🎛️  Interactive MCP Server Configuration")
	fmt.Println(retorBanner)
	fmt.Println(infoStyle.Render("Use arrow keys to navigate, Enter to select, Ctrl+C to exit"))
	fmt.Println()

	// Get basic flags that might be pre-set
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	
	// Create the interactive form model
	formModel := newMCPAddForm(endpoint, environment)
	
	// Run the interactive form
	program := tea.NewProgram(formModel, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run interactive form: %w", err)
	}
	
	// Check if user cancelled
	final := finalModel.(*mcpAddFormModel)
	if final.cancelled {
		fmt.Println(infoStyle.Render("Operation cancelled"))
		return nil
	}
	
	// Show completion banner with collected data
	showSuccessBanner("MCP Server Configuration Complete!")
	fmt.Printf("Adding server: %s\n", successStyle.Render(final.serverName))
	fmt.Printf("To config: %s\n", successStyle.Render(final.configID))
	fmt.Printf("Command: %s %v\n", successStyle.Render(final.command), final.args)
	
	// Now execute the actual addition
	result, err := addServerToConfig(final.configID, final.serverName, final.command, final.args, final.envVars, final.environment, final.endpoint)
	if err != nil {
		fmt.Println(errorStyle.Render("❌ Failed to add server: " + err.Error()))
		return err
	}
	
	fmt.Printf("Result: %s\n", result)
	return nil
}