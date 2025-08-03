package apps

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/afero"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/filesystem"
	"station/internal/services"
	"station/internal/template"
	"station/internal/variables"
	"station/pkg/models"
	pkgconfig "station/pkg/config"
)

type ConfigMode int

const (
	ModeViewing ConfigMode = iota
	ModeEditing
	ModeSelectingEnvironment
)

type ConfigsApp struct {
	repos             *repositories.Repositories
	fileConfigService *services.FileConfigService
	configs           []pkgconfig.ConfigInfo
	environments      []*models.Environment
	cursor           int
	loading          bool
	err              error
	
	// Text editor state
	mode           ConfigMode
	editorContent  string
	editorCursor   int
	selectedEnvID  int64
	envCursor      int
}

type configsDataMsg struct {
	configs      []pkgconfig.ConfigInfo
	environments []*models.Environment
}
type configsErrorMsg error
type configSavedMsg struct{}

func NewConfigsApp(repos *repositories.Repositories) *ConfigsApp {
	// Initialize file-based configuration system
	fs := afero.NewOsFs()
	configDir := "./config"
	varsDir := "./config/vars"
	
	// Create file system components
	fileSystem := filesystem.NewConfigFileSystem(fs, configDir, varsDir)
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	// Create file config options
	fileConfigOptions := pkgconfig.FileConfigOptions{
		ConfigDir:       configDir,
		VariablesDir:    varsDir,
		Strategy:        pkgconfig.StrategyTemplateFirst,
		AutoCreate:      true,
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}
	
	// Create file config manager
	fileConfigManager := config.NewFileConfigManager(
		fileSystem,
		templateEngine,
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)
	
	// Initialize tool discovery service
	toolDiscoveryService := services.NewToolDiscoveryService(repos)
	
	// Initialize file config service
	fileConfigService := services.NewFileConfigService(
		fileConfigManager,
		toolDiscoveryService,
		repos,
	)
	
	return &ConfigsApp{
		repos:             repos,
		fileConfigService: fileConfigService,
		configs:          []pkgconfig.ConfigInfo{},
		environments:     []*models.Environment{},
		mode:             ModeViewing,
		selectedEnvID:    0,
	}
}

func (app *ConfigsApp) Init() tea.Cmd {
	return app.loadData
}

func (app *ConfigsApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Handle window resize if needed
		return app, nil
		
	case tea.KeyMsg:
		return app.handleKeys(msg)
		
	case configsDataMsg:
		app.configs = msg.configs
		app.environments = msg.environments
		app.loading = false
		return app, nil
		
	case configsErrorMsg:
		app.err = msg
		app.loading = false
		return app, nil
		
	case configSavedMsg:
		return app, app.loadData
	}
	
	return app, nil
}

func (app *ConfigsApp) handleKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch app.mode {
	case ModeViewing:
		return app.handleViewingKeys(msg)
	case ModeEditing:
		return app.handleEditingKeys(msg)
	case ModeSelectingEnvironment:
		return app.handleEnvironmentKeys(msg)
	}
	return app, nil
}

func (app *ConfigsApp) handleViewingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		return app, tea.Quit
	case "up", "k":
		if app.cursor > 0 {
			app.cursor--
		}
	case "down", "j":
		if app.cursor < len(app.configs)-1 {
			app.cursor++
		}
	case "n":
		app.mode = ModeSelectingEnvironment
		app.envCursor = 0
	case "e":
		if len(app.configs) > 0 {
			app.mode = ModeEditing
			app.editorContent = fmt.Sprintf("Edit config: %s", app.configs[app.cursor].Name)
		}
	case "r":
		return app, app.loadData
	}
	return app, nil
}

func (app *ConfigsApp) handleEditingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		app.mode = ModeViewing
	case "ctrl+s":
		return app, app.saveConfig
	}
	return app, nil
}

func (app *ConfigsApp) handleEnvironmentKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		app.mode = ModeViewing
	case "up", "k":
		if app.envCursor > 0 {
			app.envCursor--
		}
	case "down", "j":
		if app.envCursor < len(app.environments)-1 {
			app.envCursor++
		}
	case "enter":
		if len(app.environments) > 0 {
			app.selectedEnvID = app.environments[app.envCursor].ID
			app.mode = ModeEditing
		}
	}
	return app, nil
}

func (app *ConfigsApp) View() string {
	if app.loading {
		return "Loading configs..."
	}
	
	if app.err != nil {
		return fmt.Sprintf("Error: %v", app.err)
	}
	
	switch app.mode {
	case ModeViewing:
		return app.viewConfigs()
	case ModeEditing:
		return app.viewEditor()
	case ModeSelectingEnvironment:
		return app.viewEnvironments()
	}
	
	return ""
}

func (app *ConfigsApp) viewConfigs() string {
	var lines []string
	lines = append(lines, "📂 File-Based Configurations")
	lines = append(lines, "")
	
	if len(app.configs) == 0 {
		lines = append(lines, "No configurations found")
	} else {
		for i, config := range app.configs {
			prefix := "  "
			if i == app.cursor {
				prefix = "▶ "
			}
			lines = append(lines, fmt.Sprintf("%s%s (%s)", prefix, config.Name, config.Environment))
		}
	}
	
	lines = append(lines, "")
	lines = append(lines, "Controls: ↑/↓ navigate, n new, e edit, r refresh, q quit")
	
	return strings.Join(lines, "\n")
}

func (app *ConfigsApp) viewEditor() string {
	var lines []string
	lines = append(lines, "✏️  Edit Configuration")
	lines = append(lines, "")
	lines = append(lines, app.editorContent)
	lines = append(lines, "")
	lines = append(lines, "Controls: Ctrl+S save, Esc cancel")
	
	return strings.Join(lines, "\n")
}

func (app *ConfigsApp) viewEnvironments() string {
	var lines []string
	lines = append(lines, "🌍 Select Environment")
	lines = append(lines, "")
	
	for i, env := range app.environments {
		prefix := "  "
		if i == app.envCursor {
			prefix = "▶ "
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, env.Name))
	}
	
	lines = append(lines, "")
	lines = append(lines, "Controls: ↑/↓ navigate, Enter select, Esc cancel")
	
	return strings.Join(lines, "\n")
}

func (app *ConfigsApp) loadData() tea.Msg {
	// Load environments
	envs, err := app.repos.Environments.List()
	if err != nil {
		return configsErrorMsg(err)
	}
	
	// Load file configs from all environments
	var allConfigs []pkgconfig.ConfigInfo
	for _, env := range envs {
		configs, err := app.fileConfigService.ListFileConfigs(context.Background(), env.ID)
		if err != nil {
			continue // Skip failed environments
		}
		allConfigs = append(allConfigs, configs...)
	}
	
	return configsDataMsg{
		configs:      allConfigs,
		environments: envs,
	}
}

func (app *ConfigsApp) saveConfig() tea.Msg {
	// Simple save implementation - in a real app this would save the editor content
	return configSavedMsg{}
}