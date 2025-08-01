package services

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"station/pkg/models"
)

// TurboWizardState represents the current state of the wizard
type TurboWizardState int

const (
	StateShowingBlocks TurboWizardState = iota
	StateConfiguringServer
	StateReviewConfig
	StateCompleted
)

// TurboWizardModel implements the TurboTax-style MCP configuration wizard
type TurboWizardModel struct {
	state           TurboWizardState
	blocks          []MCPServerBlock
	selectedBlocks  []bool           // Which blocks user wants to configure
	currentBlock    int              // Current block being shown/configured
	configurations  []ServerConfig   // Completed configurations
	currentConfig   *ServerConfig    // Currently being configured
	configStep      int              // Current step in server configuration
	completed       bool
	cancelled       bool
	width           int
	height          int
}

// ServerConfig represents a configured MCP server
type ServerConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	RawBlock    string            `json:"rawBlock"`
	Description string            `json:"description"`
}

// NewTurboWizardModel creates a new TurboTax-style wizard
func NewTurboWizardModel(blocks []MCPServerBlock) *TurboWizardModel {
	return &TurboWizardModel{
		state:          StateShowingBlocks,
		blocks:         blocks,
		selectedBlocks: make([]bool, len(blocks)),
		currentBlock:   0,
		configurations: make([]ServerConfig, 0),
		configStep:     0,
	}
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
		case StateReviewConfig:
			return m.updateReviewConfig(msg)
		}
	}

	return m, nil
}

func (m TurboWizardModel) updateBlockSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.currentBlock > 0 {
			m.currentBlock--
		}
	case "down", "j":
		if m.currentBlock < len(m.blocks)-1 {
			m.currentBlock++
		}
	case " ", "enter": // Toggle selection
		m.selectedBlocks[m.currentBlock] = !m.selectedBlocks[m.currentBlock]
	case "n": // Next step
		// Check if any blocks are selected
		hasSelected := false
		for _, selected := range m.selectedBlocks {
			if selected {
				hasSelected = true
				break
			}
		}
		if hasSelected {
			m.state = StateConfiguringServer
			m.currentBlock = m.getNextSelectedBlock(0)
			m.startConfiguringCurrentBlock()
		}
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m TurboWizardModel) updateServerConfiguration(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y": // Accept current value and move to next
		m.configStep++
		if m.configStep >= m.getConfigStepCount() {
			// Configuration complete for this server
			m.configurations = append(m.configurations, *m.currentConfig)
			
			// Move to next selected block
			nextBlock := m.getNextSelectedBlock(m.currentBlock + 1)
			if nextBlock == -1 {
				// All blocks configured, go to review
				m.state = StateReviewConfig
			} else {
				m.currentBlock = nextBlock
				m.startConfiguringCurrentBlock()
			}
		}
	case "e": // Edit current value
		// For now, just accept - editing would require more complex input handling
		m.configStep++
		if m.configStep >= m.getConfigStepCount() {
			m.configurations = append(m.configurations, *m.currentConfig)
			
			nextBlock := m.getNextSelectedBlock(m.currentBlock + 1)
			if nextBlock == -1 {
				m.state = StateReviewConfig
			} else {
				m.currentBlock = nextBlock
				m.startConfiguringCurrentBlock()
			}
		}
	case "b": // Go back
		if m.configStep > 0 {
			m.configStep--
		} else {
			// Go back to block selection
			m.state = StateShowingBlocks
		}
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m TurboWizardModel) updateReviewConfig(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter": // Accept and complete
		m.state = StateCompleted
		m.completed = true
		return m, tea.Quit
	case "b": // Go back to configuration
		if len(m.configurations) > 0 {
			// Remove last configuration and go back to configuring it
			m.configurations = m.configurations[:len(m.configurations)-1]
			m.state = StateConfiguringServer
			m.currentBlock = m.getLastSelectedBlock()
			m.startConfiguringCurrentBlock()
		} else {
			m.state = StateShowingBlocks
		}
	case "q", "esc":
		m.cancelled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m TurboWizardModel) View() string {
	switch m.state {
	case StateShowingBlocks:
		return m.renderBlockSelection()
	case StateConfiguringServer:
		return m.renderServerConfiguration()
	case StateReviewConfig:
		return m.renderReviewConfig()
	default:
		return "Wizard completed!"
	}
}

func (m TurboWizardModel) renderBlockSelection() string {
	var b strings.Builder
	
	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(headerStyle.Render("🧙 MCP Server Configuration Wizard"))
	b.WriteString("\n\n")
	
	b.WriteString("Found MCP server configurations in the README. Select which ones you want to configure:\n\n")
	
	// Show blocks
	for i, block := range m.blocks {
		cursor := " "
		if i == m.currentBlock {
			cursor = ">"
		}
		
		checkbox := "☐"
		if m.selectedBlocks[i] {
			checkbox = "☑"
		}
		
		style := lipgloss.NewStyle()
		if i == m.currentBlock {
			style = style.Background(lipgloss.Color("240"))
		}
		
		line := fmt.Sprintf("%s %s %s - %s", cursor, checkbox, block.ServerName, block.Description)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
		
		// Show snippet of raw block for current item
		if i == m.currentBlock {
			snippetStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).MarginLeft(4)
			snippet := block.RawBlock
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			b.WriteString(snippetStyle.Render("Preview: " + snippet))
			b.WriteString("\n")
		}
	}
	
	// Instructions
	b.WriteString("\n")
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	b.WriteString(instructionStyle.Render("Controls: ↑/↓ navigate, SPACE toggle selection, N next, Q quit"))
	
	return b.String()
}

func (m TurboWizardModel) renderServerConfiguration() string {
	var b strings.Builder
	
	// Header
	block := m.blocks[m.currentBlock]
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(headerStyle.Render(fmt.Sprintf("🔧 Configuring: %s", block.ServerName)))
	b.WriteString("\n\n")
	
	b.WriteString(fmt.Sprintf("Description: %s\n\n", block.Description))
	
	// Show the current configuration step
	currentStyle := lipgloss.NewStyle().Background(lipgloss.Color("240"))
	
	steps := []string{"Server Name", "Command", "Arguments", "Environment Variables"}
	
	b.WriteString("Configuration Steps:\n")
	for i, step := range steps {
		cursor := " "
		if i == m.configStep {
			cursor = ">"
		}
		
		style := lipgloss.NewStyle()
		if i == m.configStep {
			style = currentStyle
		} else if i < m.configStep {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // Green for completed
		}
		
		line := fmt.Sprintf("%s %d. %s", cursor, i+1, step)
		b.WriteString(style.Render(line))
		b.WriteString("\n")
	}
	
	b.WriteString("\n")
	
	// Show current step details
	switch m.configStep {
	case 0: // Server Name
		b.WriteString(fmt.Sprintf("Server Name: %s\n", m.currentConfig.Name))
		b.WriteString("This will be used as the key in your MCP configuration.\n")
	case 1: // Command
		b.WriteString(fmt.Sprintf("Command: %s\n", m.currentConfig.Command))
		b.WriteString("The executable command to run this MCP server.\n")
	case 2: // Arguments
		if len(m.currentConfig.Args) > 0 {
			b.WriteString(fmt.Sprintf("Arguments: %v\n", m.currentConfig.Args))
		} else {
			b.WriteString("Arguments: (none)\n")
		}
		b.WriteString("Command line arguments passed to the server.\n")
	case 3: // Environment Variables
		if len(m.currentConfig.Env) > 0 {
			b.WriteString("Environment Variables:\n")
			for k, v := range m.currentConfig.Env {
				b.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
			}
		} else {
			b.WriteString("Environment Variables: (none)\n")
		}
		b.WriteString("Environment variables required by the server.\n")
	}
	
	b.WriteString("\n")
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	b.WriteString(instructionStyle.Render("Controls: Y accept, E edit, B back, Q quit"))
	
	return b.String()
}

func (m TurboWizardModel) renderReviewConfig() string {
	var b strings.Builder
	
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	b.WriteString(headerStyle.Render("📋 Review Configuration"))
	b.WriteString("\n\n")
	
	b.WriteString(fmt.Sprintf("You have configured %d MCP server(s):\n\n", len(m.configurations)))
	
	for i, config := range m.configurations {
		configStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).MarginBottom(1)
		
		var configText strings.Builder
		configText.WriteString(fmt.Sprintf("Server %d: %s\n", i+1, config.Name))
		configText.WriteString(fmt.Sprintf("Command: %s\n", config.Command))
		if len(config.Args) > 0 {
			configText.WriteString(fmt.Sprintf("Args: %v\n", config.Args))
		}
		if len(config.Env) > 0 {
			configText.WriteString("Environment:\n")
			for k, v := range config.Env {
				configText.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
			}
		}
		
		b.WriteString(configStyle.Render(configText.String()))
		b.WriteString("\n")
	}
	
	instructionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	b.WriteString(instructionStyle.Render("Controls: Y accept and continue, B go back, Q quit"))
	
	return b.String()
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
	
	// Parse the raw block to extract configuration
	config := &ServerConfig{
		Name:        block.ServerName,
		Description: block.Description,
		RawBlock:    block.RawBlock,
		Env:         make(map[string]string),
	}
	
	// Try to parse the JSON block to extract defaults
	var parsedBlock map[string]interface{}
	if err := json.Unmarshal([]byte(block.RawBlock), &parsedBlock); err == nil {
		if command, ok := parsedBlock["command"].(string); ok {
			config.Command = command
		}
		if args, ok := parsedBlock["args"].([]interface{}); ok {
			config.Args = make([]string, len(args))
			for i, arg := range args {
				if argStr, ok := arg.(string); ok {
					config.Args[i] = argStr
				}
			}
		}
		if env, ok := parsedBlock["env"].(map[string]interface{}); ok {
			for k, v := range env {
				if vStr, ok := v.(string); ok {
					config.Env[k] = vStr
				}
			}
		}
	}
	
	m.currentConfig = config
	m.configStep = 0
}

func (m *TurboWizardModel) getConfigStepCount() int {
	return 4 // Name, Command, Args, Env
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

func (m *TurboWizardModel) GetFinalMCPConfig() *models.MCPConfigData {
	if !m.completed || len(m.configurations) == 0 {
		return nil
	}
	
	servers := make(map[string]models.MCPServerConfig)
	for _, config := range m.configurations {
		servers[config.Name] = models.MCPServerConfig{
			Command: config.Command,
			Args:    config.Args,
			Env:     config.Env,
		}
	}
	
	return &models.MCPConfigData{
		Name:    "mcp-configuration", // Default name
		Servers: servers,
	}
}