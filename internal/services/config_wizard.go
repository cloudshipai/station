package services

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"station/pkg/models"
)

// WizardStep represents different steps in the configuration wizard
type WizardStep int

const (
	StepSelectConfig WizardStep = iota
	StepEnvVars
	StepConfirm
	StepComplete
)

// ConfigWizardModel represents the bubbletea model for the MCP configuration wizard
type ConfigWizardModel struct {
	discovery         *MCPServerDiscovery
	currentStep       WizardStep
	selectedConfig    int
	envInputs         []textinput.Model
	envValues         map[string]string
	currentEnvInput   int
	confirmed         bool
	finalConfig       *models.MCPConfigData
	err               error
	
	// UI styling
	titleStyle        lipgloss.Style
	optionStyle       lipgloss.Style
	selectedStyle     lipgloss.Style
	descriptionStyle  lipgloss.Style
	errorStyle        lipgloss.Style
	successStyle      lipgloss.Style
	promptStyle       lipgloss.Style
}

// NewConfigWizardModel creates a new configuration wizard model
func NewConfigWizardModel(discovery *MCPServerDiscovery) *ConfigWizardModel {
	// Initialize environment variable inputs
	var envInputs []textinput.Model
	envValues := make(map[string]string)
	
	for _, env := range discovery.RequiredEnv {
		input := textinput.New()
		input.Placeholder = env.Example
		input.CharLimit = 200
		input.Width = 50
		envInputs = append(envInputs, input)
		envValues[env.Name] = ""
	}
	
	// Focus first input if there are any
	if len(envInputs) > 0 {
		envInputs[0].Focus()
	}

	return &ConfigWizardModel{
		discovery:         discovery,
		currentStep:       StepSelectConfig,
		selectedConfig:    0,
		envInputs:         envInputs,
		envValues:         envValues,
		currentEnvInput:   0,
		
		// UI styles
		titleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7C3AED")).
			Bold(true).
			MarginBottom(1),
		
		optionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#64748B")).
			MarginLeft(2),
		
		selectedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Background(lipgloss.Color("#065F46")).
			Bold(true).
			MarginLeft(2).
			Padding(0, 1),
		
		descriptionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94A3B8")).
			MarginLeft(4).
			Italic(true),
		
		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")).
			Bold(true),
		
		successStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981")).
			Bold(true),
		
		promptStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6")).
			Bold(true).
			MarginTop(1),
	}
}

// Init implements the bubbletea Model interface
func (m *ConfigWizardModel) Init() tea.Cmd {
	return nil
}

// Update implements the bubbletea Model interface
func (m *ConfigWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.currentStep {
		case StepSelectConfig:
			return m.updateConfigSelection(msg)
		case StepEnvVars:
			return m.updateEnvVars(msg)
		case StepConfirm:
			return m.updateConfirm(msg)
		case StepComplete:
			if msg.String() == "enter" || msg.String() == "q" {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

// updateConfigSelection handles config selection step
func (m *ConfigWizardModel) updateConfigSelection(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.selectedConfig > 0 {
			m.selectedConfig--
		}
	case "down", "j":
		if m.selectedConfig < len(m.discovery.Configurations)-1 {
			m.selectedConfig++
		}
	case "enter":
		// Move to environment variables step
		m.currentStep = StepEnvVars
		if len(m.envInputs) == 0 {
			// No env vars needed, go straight to confirm
			m.currentStep = StepConfirm
		}
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// updateEnvVars handles environment variables input step
func (m *ConfigWizardModel) updateEnvVars(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		// Move to next input
		if m.currentEnvInput < len(m.envInputs)-1 {
			m.envInputs[m.currentEnvInput].Blur()
			m.currentEnvInput++
			m.envInputs[m.currentEnvInput].Focus()
		}
	case "shift+tab", "up":
		// Move to previous input
		if m.currentEnvInput > 0 {
			m.envInputs[m.currentEnvInput].Blur()
			m.currentEnvInput--
			m.envInputs[m.currentEnvInput].Focus()
		}
	case "enter":
		// Save current input value and move to confirm step
		envName := m.discovery.RequiredEnv[m.currentEnvInput].Name
		m.envValues[envName] = m.envInputs[m.currentEnvInput].Value()
		
		// Save all input values
		for i, input := range m.envInputs {
			if i < len(m.discovery.RequiredEnv) {
				envName := m.discovery.RequiredEnv[i].Name
				m.envValues[envName] = input.Value()
			}
		}
		
		m.currentStep = StepConfirm
	case "q", "ctrl+c":
		return m, tea.Quit
	default:
		// Update the current input
		var cmd tea.Cmd
		m.envInputs[m.currentEnvInput], cmd = m.envInputs[m.currentEnvInput].Update(msg)
		return m, cmd
	}
	return m, nil
}

// updateConfirm handles confirmation step
func (m *ConfigWizardModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		// Generate final configuration
		m.confirmed = true
		m.generateFinalConfig()
		m.currentStep = StepComplete
	case "n", "N", "q", "ctrl+c":
		return m, tea.Quit
	case "e", "E":
		// Go back to edit environment variables
		m.currentStep = StepEnvVars
		if len(m.envInputs) > 0 {
			m.envInputs[m.currentEnvInput].Focus()
		}
	}
	return m, nil
}

// generateFinalConfig creates the final MCP configuration
func (m *ConfigWizardModel) generateFinalConfig() {
	selectedConfig := m.discovery.Configurations[m.selectedConfig]
	
	// Merge wizard env values with config env values
	finalEnv := make(map[string]string)
	
	// Start with config-defined env vars
	for k, v := range selectedConfig.Env {
		finalEnv[k] = v
	}
	
	// Override with user-provided values
	for k, v := range m.envValues {
		if v != "" {
			finalEnv[k] = v
		}
	}
	
	// Create the MCP server config
	servers := make(map[string]models.MCPServerConfig)
	servers[m.discovery.ServerName] = models.MCPServerConfig{
		Command: selectedConfig.Command,
		Args:    selectedConfig.Args,
		Env:     finalEnv,
	}
	
	m.finalConfig = &models.MCPConfigData{
		Name:    fmt.Sprintf("%s-config", m.discovery.ServerName),
		Servers: servers,
	}
}

// View implements the bubbletea Model interface
func (m *ConfigWizardModel) View() string {
	var s strings.Builder
	
	// Title
	s.WriteString(m.titleStyle.Render("🧙 MCP Server Configuration Wizard"))
	s.WriteString("\n\n")
	
	// Server info
	s.WriteString(fmt.Sprintf("Server: %s\n", m.discovery.ServerName))
	s.WriteString(fmt.Sprintf("Description: %s\n\n", m.discovery.Description))
	
	switch m.currentStep {
	case StepSelectConfig:
		s.WriteString(m.renderConfigSelection())
	case StepEnvVars:
		s.WriteString(m.renderEnvVars())
	case StepConfirm:
		s.WriteString(m.renderConfirm())
	case StepComplete:
		s.WriteString(m.renderComplete())
	}
	
	if m.err != nil {
		s.WriteString("\n")
		s.WriteString(m.errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}
	
	return s.String()
}

// renderConfigSelection renders the configuration selection step
func (m *ConfigWizardModel) renderConfigSelection() string {
	var s strings.Builder
	
	s.WriteString(m.promptStyle.Render("Select a configuration option:"))
	s.WriteString("\n\n")
	
	for i, config := range m.discovery.Configurations {
		if i == m.selectedConfig {
			s.WriteString(m.selectedStyle.Render(fmt.Sprintf("→ %s", config.Name)))
			if config.Recommended {
				s.WriteString(" " + m.successStyle.Render("(Recommended)"))
			}
		} else {
			s.WriteString(m.optionStyle.Render(fmt.Sprintf("  %s", config.Name)))
			if config.Recommended {
				s.WriteString(" " + m.successStyle.Render("(Recommended)"))
			}
		}
		s.WriteString("\n")
		s.WriteString(m.descriptionStyle.Render(config.Description))
		s.WriteString("\n")
		s.WriteString(m.descriptionStyle.Render(fmt.Sprintf("Command: %s %s", config.Command, strings.Join(config.Args, " "))))
		s.WriteString("\n\n")
	}
	
	s.WriteString("Use ↑/↓ to navigate, Enter to select, q to quit\n")
	return s.String()
}

// renderEnvVars renders the environment variables input step
func (m *ConfigWizardModel) renderEnvVars() string {
	var s strings.Builder
	
	s.WriteString(m.promptStyle.Render("Configure environment variables:"))
	s.WriteString("\n\n")
	
	for i, env := range m.discovery.RequiredEnv {
		if i >= len(m.envInputs) {
			break
		}
		
		// Environment variable label
		label := env.Name
		if env.Required {
			label += " " + m.errorStyle.Render("(Required)")
		}
		s.WriteString(label)
		s.WriteString("\n")
		
		// Description
		if env.Description != "" {
			s.WriteString(m.descriptionStyle.Render(env.Description))
			s.WriteString("\n")
		}
		
		// Input field
		if i == m.currentEnvInput {
			s.WriteString("→ ")
		} else {
			s.WriteString("  ")
		}
		s.WriteString(m.envInputs[i].View())
		s.WriteString("\n")
		
		// Example
		if env.Example != "" {
			s.WriteString(m.descriptionStyle.Render(fmt.Sprintf("Example: %s", env.Example)))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}
	
	s.WriteString("Use Tab/Shift+Tab to navigate, Enter to continue, q to quit\n")
	return s.String()
}

// renderConfirm renders the confirmation step
func (m *ConfigWizardModel) renderConfirm() string {
	var s strings.Builder
	
	s.WriteString(m.promptStyle.Render("Configuration Summary:"))
	s.WriteString("\n\n")
	
	// Selected configuration
	selectedConfig := m.discovery.Configurations[m.selectedConfig]
	s.WriteString(fmt.Sprintf("Configuration: %s\n", selectedConfig.Name))
	s.WriteString(fmt.Sprintf("Command: %s %s\n", selectedConfig.Command, strings.Join(selectedConfig.Args, " ")))
	s.WriteString("\n")
	
	// Environment variables
	if len(m.envValues) > 0 {
		s.WriteString("Environment Variables:\n")
		for name, value := range m.envValues {
			if value != "" {
				s.WriteString(fmt.Sprintf("  %s = %s\n", name, value))
			}
		}
		s.WriteString("\n")
	}
	
	s.WriteString(m.promptStyle.Render("Save this configuration? (y/n/e to edit)"))
	return s.String()
}

// renderComplete renders the completion step
func (m *ConfigWizardModel) renderComplete() string {
	var s strings.Builder
	
	s.WriteString(m.successStyle.Render("✅ Configuration Complete!"))
	s.WriteString("\n\n")
	
	if m.finalConfig != nil {
		s.WriteString(fmt.Sprintf("Configuration name: %s\n", m.finalConfig.Name))
		s.WriteString(fmt.Sprintf("Ready to upload to environment\n\n"))
	}
	
	s.WriteString("Press Enter or q to continue")
	return s.String()
}

// GetFinalConfig returns the final configuration if the wizard was completed successfully
func (m *ConfigWizardModel) GetFinalConfig() *models.MCPConfigData {
	return m.finalConfig
}

// IsCompleted returns true if the wizard was completed successfully
func (m *ConfigWizardModel) IsCompleted() bool {
	return m.confirmed && m.finalConfig != nil
}