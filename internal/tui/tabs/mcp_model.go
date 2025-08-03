package tabs

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/afero"
	
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/filesystem"
	"station/internal/services"
	"station/internal/template"
	"station/internal/variables"
	"station/pkg/models"
	pkgconfig "station/pkg/config"
)

// MCPModel represents the MCP servers configuration tab
type MCPModel struct {
	BaseTabModel
	
	// UI components
	configEditor textarea.Model
	nameInput    textinput.Model
	environmentList list.Model
	versionList     list.Model
	
	// Data
	envs         []*models.Environment
	repos        *repositories.Repositories
	database     db.Database  // Need this to create services
	
	// Services - file-based configuration system
	fileConfigService   *services.FileConfigService
	toolDiscoverySvc    *services.ToolDiscoveryService
	genkitService       services.AgentServiceInterface
	
	// State
	mode         MCPMode
	configs      []MCPConfigDisplay
	configVersions []MCPConfigDisplay // All versions of current config
	selectedIdx  int
	showEditor   bool
	selectedEnvID int64
	focusedField MCPField
	selectedVersionIdx int
	currentConfigName string // Track which config we're editing
	
	// Change detection for auto-save
	originalName   string // Original name when entering edit mode
	originalConfig string // Original config JSON when entering edit mode
}

type MCPField int

const (
	MCPFieldName MCPField = iota
	MCPFieldEnvironment
	MCPFieldVersions
	MCPFieldConfig
)

type MCPMode int

const (
	MCPModeList MCPMode = iota
	MCPModeEdit
)

type MCPConfigDisplay struct {
	ID            int64
	Name          string
	Version       int64
	Updated       string
	Size          string
	ConfigJSON    string // Store the actual JSON content
	EnvironmentID int64
	EnvironmentName string
	ToolStatus    ToolExtractionStatus
	ToolCount     int
}

type ToolExtractionStatus int

const (
	ToolStatusUnknown ToolExtractionStatus = iota
	ToolStatusSuccess
	ToolStatusFailed
	ToolStatusPartial
)

// VersionItem implements list.Item for version selection
type VersionItem struct {
	config MCPConfigDisplay
}

func (i VersionItem) FilterValue() string {
	return fmt.Sprintf("v%d %s", i.config.Version, i.config.Updated)
}

func (i VersionItem) Title() string {
	return fmt.Sprintf("v%d", i.config.Version)
}

func (i VersionItem) Description() string {
	return fmt.Sprintf("Updated %s • Size %s", i.config.Updated, i.config.Size)
}

// NewMCPModel creates a new MCP model with file-based configuration
func NewMCPModel(database db.Database, genkitService services.AgentServiceInterface) *MCPModel {
	repos := repositories.New(database)
	
	// Create textarea for config editing - scrollable
	ta := textarea.New()
	ta.Placeholder = "Paste your MCP server configuration here (JSON format)..."
	ta.SetWidth(60) // Will be adjusted dynamically in renderEditor
	ta.SetHeight(5)  // Will be adjusted dynamically in renderEditor
	
	// Create text input for name
	ti := textinput.New()
	ti.Placeholder = "Configuration name"
	ti.Width = 30
	
	// Create environment list
	envList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 4)
	envList.SetShowHelp(false)
	envList.SetShowStatusBar(false)
	envList.SetShowTitle(false)
	
	// Create version list
	versionList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 6)
	versionList.SetShowHelp(false)
	versionList.SetShowStatusBar(false)
	versionList.SetShowTitle(false)
	envList.SetFilteringEnabled(false)
	versionList.SetFilteringEnabled(false)
	
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
	
	// Initialize tool discovery service (updated to use file-based system)
	toolDiscoverySvc := services.NewToolDiscoveryService(repos)
	
	// Initialize file config service
	fileConfigService := services.NewFileConfigService(
		fileConfigManager,
		toolDiscoverySvc,
		repos,
	)
	
	m := &MCPModel{
		BaseTabModel:        NewBaseTabModel(database, "MCP Servers"),
		configEditor:        ta,
		nameInput:           ti,
		environmentList:     envList,
		versionList:         versionList,
		repos:               repos,
		database:            database,
		fileConfigService:   fileConfigService,
		toolDiscoverySvc:    toolDiscoverySvc,
		genkitService:       genkitService,
		mode:                MCPModeList,
		configs:             []MCPConfigDisplay{},
		configVersions:      []MCPConfigDisplay{},
		selectedIdx:         0,
		showEditor:          false,
		selectedEnvID:       0, // Will be set when environments load
		focusedField:        MCPFieldName,
		selectedVersionIdx:  0,
		currentConfigName:   "",
		originalName:        "",
		originalConfig:      "",
	}
	
	return m
}

// Init initializes the MCP tab
func (m MCPModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadConfigs(),
		m.loadEnvironments(),
		m.configEditor.Cursor.BlinkCmd(),
	)
}

// RefreshData reloads MCP configs from database
func (m *MCPModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadConfigs()
}

// IsMainView returns true if in main list view
func (m MCPModel) IsMainView() bool {
	return m.mode == MCPModeList
}