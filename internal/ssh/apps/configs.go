package apps

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/crypto"
	"station/pkg/models"
)

type ConfigMode int

const (
	ModeViewing ConfigMode = iota
	ModeEditing
	ModeSelectingEnvironment
)

type ConfigsApp struct {
	repos        *repositories.Repositories
	configs      []*models.MCPConfig
	environments []*models.Environment
	cursor       int
	loading      bool
	err          error
	
	// Text editor state
	mode           ConfigMode
	editorContent  string
	editorCursor   int
	selectedEnvID  int64
	envCursor      int
	configService  *services.MCPConfigService
}

type configsDataMsg struct {
	configs     []*models.MCPConfig
	environments []*models.Environment
}
type configsErrorMsg error
type configSavedMsg struct{}

func NewConfigsApp(repos *repositories.Repositories) *ConfigsApp {
	// Initialize key manager and config service
	keyManager, err := crypto.NewKeyManagerFromEnv()
	var configService *services.MCPConfigService
	if err == nil {
		configService = services.NewMCPConfigService(repos, keyManager)
	}
	
	return &ConfigsApp{
		repos:         repos,
		loading:       true,
		mode:          ModeViewing,
		configService: configService,
	}
}

func (app *ConfigsApp) Init() tea.Cmd {
	return app.loadConfigs
}

func (app *ConfigsApp) loadConfigs() tea.Msg {
	// Load all environments first
	environments, err := app.repos.Environments.List()
	if err != nil {
		return configsErrorMsg(err)
	}

	// Load latest config for each environment
	var configs []*models.MCPConfig
	for _, env := range environments {
		config, err := app.repos.MCPConfigs.GetLatest(env.ID)
		if err != nil {
			// Skip if no config exists for this environment
			continue
		}
		configs = append(configs, config)
	}

	return configsDataMsg{
		configs:     configs,
		environments: environments,
	}
}

func (app *ConfigsApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case configsDataMsg:
		app.configs = msg.configs
		app.environments = msg.environments
		app.loading = false
		return app, nil

	case configsErrorMsg:
		app.err = error(msg)
		app.loading = false
		return app, nil

	case configSavedMsg:
		app.mode = ModeViewing
		app.editorContent = ""
		app.loading = true
		return app, app.loadConfigs

	case tea.KeyMsg:
		if app.loading {
			return app, nil
		}

		return app.handleKeyMsg(msg)
	}

	return app, nil
}

func (app *ConfigsApp) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch app.mode {
	case ModeViewing:
		return app.handleViewingKeys(msg)
	case ModeSelectingEnvironment:
		return app.handleEnvironmentKeys(msg)
	case ModeEditing:
		return app.handleEditorKeys(msg)
	}
	return app, nil
}

func (app *ConfigsApp) handleViewingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if app.cursor > 0 {
			app.cursor--
		}
	case "down", "j":
		if app.cursor < len(app.configs)-1 {
			app.cursor++
		}
	case "r":
		app.loading = true
		return app, app.loadConfigs
	case "n", "N":
		// Start creating new config
		app.mode = ModeSelectingEnvironment
		app.envCursor = 0
	}
	return app, nil
}

func (app *ConfigsApp) handleEnvironmentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if app.envCursor > 0 {
			app.envCursor--
		}
	case "down", "j":
		if app.envCursor < len(app.environments)-1 {
			app.envCursor++
		}
	case "enter":
		if app.envCursor < len(app.environments) {
			app.selectedEnvID = app.environments[app.envCursor].ID
			app.mode = ModeEditing
			// Pre-fill with filesystem config example
			app.editorContent = `{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem",
        "/Users/username/Desktop",
        "/path/to/other/allowed/dir"
      ]
    }
  }
}`
		}
	case "esc":
		app.mode = ModeViewing
	}
	return app, nil
}

func (app *ConfigsApp) handleEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		app.mode = ModeViewing
		app.editorContent = ""
	case "ctrl+s":
		// Save the config
		return app, app.saveConfig
	case "backspace":
		if len(app.editorContent) > 0 {
			app.editorContent = app.editorContent[:len(app.editorContent)-1]
		}
	case "enter":
		app.editorContent += "\n"
	default:
		// Add the character to the content
		if len(msg.String()) == 1 {
			app.editorContent += msg.String()
		}
	}
	return app, nil
}

func (app *ConfigsApp) saveConfig() tea.Msg {
	if app.configService == nil {
		return configsErrorMsg(fmt.Errorf("config service not available"))
	}

	// Parse the JSON config
	var configData struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal([]byte(app.editorContent), &configData); err != nil {
		return configsErrorMsg(fmt.Errorf("invalid JSON: %v", err))
	}

	// Convert to internal format
	mcpConfigData := &models.MCPConfigData{
		Servers: make(map[string]models.MCPServerConfig),
	}

	for name, server := range configData.MCPServers {
		mcpConfigData.Servers[name] = models.MCPServerConfig{
			Command: server.Command,
			Args:    server.Args,
			Env:     server.Env,
		}
	}

	// Save to database
	_, err := app.configService.UploadConfig(app.selectedEnvID, mcpConfigData)
	if err != nil {
		return configsErrorMsg(fmt.Errorf("failed to save config: %v", err))
	}

	return configSavedMsg{}
}

func (app *ConfigsApp) View() string {
	if app.loading {
		return "Loading MCP configurations..."
	}

	if app.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'r' to retry, Esc to go back", app.err)
	}

	switch app.mode {
	case ModeViewing:
		return app.viewConfigsList()
	case ModeSelectingEnvironment:
		return app.viewEnvironmentSelector()
	case ModeEditing:
		return app.viewEditor()
	}

	return ""
}

func (app *ConfigsApp) viewConfigsList() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("⚙️ MCP Configurations"))
	content.WriteString("\n\n")

	if len(app.configs) == 0 {
		content.WriteString("No MCP configurations found.\n")
		content.WriteString("Create your first MCP configuration using the text editor.\n")
	} else {
		// Create a map for quick environment lookup
		envMap := make(map[int64]*models.Environment)
		for _, env := range app.environments {
			envMap[env.ID] = env
		}

		for i, config := range app.configs {
			style := normalStyle
			if i == app.cursor {
				style = selectedStyle
			}

			envName := "Unknown Environment"
			if env, ok := envMap[config.EnvironmentID]; ok {
				envName = env.Name
			}

			configInfo := fmt.Sprintf("%s - Version %d (Encrypted)", 
				envName, config.Version)
			content.WriteString(style.Render(configInfo))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString("Commands: ↑/↓ to navigate, n to create new config, r to refresh, Esc to go back")

	return content.String()
}

func (app *ConfigsApp) viewEnvironmentSelector() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("📝 Select Environment for New Config"))
	content.WriteString("\n\n")

	if len(app.environments) == 0 {
		content.WriteString("No environments found. Create an environment first.\n")
	} else {
		for i, env := range app.environments {
			style := normalStyle
			if i == app.envCursor {
				style = selectedStyle
			}

			desc := "No description"
			if env.Description != nil {
				desc = *env.Description
			}

			envInfo := fmt.Sprintf("%s - %s", env.Name, desc)
			content.WriteString(style.Render(envInfo))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString("Commands: ↑/↓ to navigate, Enter to select, Esc to cancel")

	return content.String()
}

func (app *ConfigsApp) viewEditor() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("📝 MCP Configuration Editor"))
	content.WriteString("\n\n")

	// Show selected environment
	var envName string
	for _, env := range app.environments {
		if env.ID == app.selectedEnvID {
			envName = env.Name
			break
		}
	}
	
	content.WriteString(fmt.Sprintf("Environment: %s\n", selectedStyle.Render(envName)))
	content.WriteString("\n")

	// Show the editor content with a border
	editorLines := strings.Split(app.editorContent, "\n")
	content.WriteString("┌─ Config JSON ──────────────────────────────────────────────────────┐\n")
	
	for i, line := range editorLines {
		if i < 15 { // Limit display to 15 lines
			content.WriteString(fmt.Sprintf("│ %-70s │\n", line))
		}
	}
	
	// Add empty lines if needed
	for i := len(editorLines); i < 15; i++ {
		content.WriteString("│                                                                      │\n")
	}
	
	content.WriteString("└────────────────────────────────────────────────────────────────────┘\n")
	content.WriteString("\n")
	content.WriteString("Commands: Type to edit, Ctrl+S to save, Esc to cancel")

	return content.String()
}