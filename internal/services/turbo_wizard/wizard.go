package turbo_wizard

import (
	tea "github.com/charmbracelet/bubbletea"
	"station/internal/theme"
	"station/pkg/models"
)

// NewTurboWizardModel creates a new TurboTax-style wizard
func NewTurboWizardModel(blocks []MCPServerBlock, themeManager *theme.ThemeManager) *TurboWizardModel {
	return &TurboWizardModel{
		state:          StateShowingBlocks,
		blocks:         blocks,
		selectedBlocks: make([]bool, len(blocks)),
		currentBlock:   0,
		configurations: make([]ServerConfig, 0),
		configStep:     0,
		environments:   []string{"development", "staging", "production"}, // Default environments
		themeManager:   themeManager,
	}
}

// SetEnvironments allows setting custom environments
func (m *TurboWizardModel) SetEnvironments(environments []string) {
	m.environments = environments
}

func (m TurboWizardModel) Init() tea.Cmd {
	return nil
}

func (m TurboWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch m.state {
		case StateShowingBlocks:
			return m.updateBlockSelection(msg)
		case StateConfiguringServer:
			return m.updateServerConfiguration(msg)
		case StateEditingField:
			return m.updateFieldEditor(msg)
		case StateSelectingEnvironment:
			return m.updateEnvironmentSelection(msg)
		case StateReviewConfig:
			return m.updateReviewConfig(msg)
		}
	}

	return m, nil
}

func (m TurboWizardModel) View() string {
	renderer := NewUIRenderer(m.width, m.height, m.themeManager)
	
	switch m.state {
	case StateShowingBlocks:
		return renderer.RenderBlockSelection(&m)
	case StateConfiguringServer:
		return renderer.RenderServerConfiguration(&m)
	case StateEditingField:
		return renderer.RenderFieldEditor(&m)
	case StateSelectingEnvironment:
		return renderer.RenderEnvironmentSelection(&m)
	case StateReviewConfig:
		return renderer.RenderReviewConfig(&m)
	default:
		return "Wizard completed!"
	}
}

// Public methods for external access

func (m *TurboWizardModel) IsCompleted() bool {
	return m.completed
}

func (m *TurboWizardModel) IsCancelled() bool {
	return m.cancelled
}

func (m *TurboWizardModel) GetConfigurations() []ServerConfig {
	return m.configurations
}

func (m *TurboWizardModel) GetSelectedEnvironment() string {
	if m.selectedEnv >= 0 && m.selectedEnv < len(m.environments) {
		return m.environments[m.selectedEnv]
	}
	return "development" // Default
}

func (m *TurboWizardModel) GetFinalMCPConfig() *models.MCPConfigData {
	if !m.completed || len(m.configurations) == 0 {
		return nil
	}

	servers := make(map[string]models.MCPServerConfig)
	for _, config := range m.configurations {
		mcpConfig := models.MCPServerConfig{
			Env: config.Env,
		}

		// Set transport-specific fields
		switch config.Transport {
		case TransportSTDIO:
			mcpConfig.Command = config.Command
			mcpConfig.Args = config.Args
		case TransportDocker:
			mcpConfig.Command = config.Command
			mcpConfig.Args = config.Args
		case TransportHTTP, TransportSSE:
			mcpConfig.URL = config.URL
		}

		servers[config.Name] = mcpConfig
	}

	return &models.MCPConfigData{
		Name:    "turbo-configured-servers",
		Servers: servers,
	}
}

// Helper methods

func (m *TurboWizardModel) getNextSelectedBlock(startFrom int) int {
	for i := startFrom; i < len(m.selectedBlocks); i++ {
		if m.selectedBlocks[i] {
			return i
		}
	}
	return -1
}

func (m *TurboWizardModel) getLastSelectedBlock() int {
	for i := len(m.selectedBlocks) - 1; i >= 0; i-- {
		if m.selectedBlocks[i] {
			return i
		}
	}
	return -1
}

func (m *TurboWizardModel) startConfiguringCurrentBlock() {
	block := m.blocks[m.currentBlock]
	parser := NewConfigParser()
	config := parser.ParseBlockToConfig(block)
	m.currentConfig = config
	m.configStep = 0
}

func (m *TurboWizardModel) getConfigStepCount() int {
	switch m.currentConfig.Transport {
	case TransportSTDIO:
		return 4 // Name, Command, Args, Env
	case TransportDocker:
		return 4 // Name, Image, Mounts, Env
	case TransportHTTP, TransportSSE:
		return 3 // Name, URL, Auth
	default:
		return 2 // Name, basic config
	}
}