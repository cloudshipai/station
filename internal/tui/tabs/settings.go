package tabs

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db"
	"station/internal/tui/components"
	"station/internal/tui/styles"
)

// SettingsModel represents the system settings tab
type SettingsModel struct {
	BaseTabModel

	// UI components
	inputs        []textinput.Model
	focusedInput  int
	
	// State
	settings      map[string]string
	editMode      bool
	settingFields []SettingField
}


type SettingField struct {
	Key         string
	Label       string
	Value       string
	Placeholder string
	Sensitive   bool
	ReadOnly    bool
	Category    string
}


// Messages for async operations
type SettingsLoadedMsg struct {
	Settings map[string]string
}

type SettingsErrorMsg struct {
	Err error
}

type SettingsSavedMsg struct{}

// NewSettingsModel creates a new settings model
func NewSettingsModel(database db.Database) *SettingsModel {
	// Create input fields for common settings
	var inputs []textinput.Model

	settingFields := []SettingField{
		// System Information (Read-Only)
		{"version", "Station Version", "v1.0", "Current Station version", false, true, "System"},
		{"build_time", "Build Time", "2024-01-15 12:00:00", "When this build was created", false, true, "System"},
		{"db_path", "Database Path", "./station.db", "Path to SQLite database", false, true, "System"},
		{"uptime", "System Uptime", "2 days, 14 hours", "How long the system has been running", false, true, "System"},
		
		// Server Configuration (Editable)
		{"log_level", "Log Level", "info", "Logging level (debug, info, warn, error)", false, false, "Server"},
		{"max_agents", "Max Concurrent Agents", "10", "Maximum concurrent agent executions", false, false, "Server"},
		{"session_timeout", "Session Timeout", "24h", "User session timeout duration", false, false, "Server"},
		{"debug_mode", "Debug Mode", "false", "Enable debug logging and features", false, false, "Server"},
	}

	for _, field := range settingFields {
		input := textinput.New()
		input.Placeholder = field.Placeholder
		input.Width = 50
		input.SetValue(field.Value)
		
		if field.Sensitive {
			input.EchoMode = textinput.EchoPassword
		}
		
		if field.ReadOnly {
			input.Blur() // Make sure it's not focused
			// We'll handle read-only display in the View method
		}
		
		inputs = append(inputs, input)
	}

	// Focus first input
	if len(inputs) > 0 {
		inputs[0].Focus()
	}

	return &SettingsModel{
		BaseTabModel:  NewBaseTabModel(database, "Settings"),
		inputs:        inputs,
		focusedInput:  0,
		settings:      make(map[string]string),
		settingFields: settingFields,
	}
}

// Init initializes the settings tab
func (m SettingsModel) Init() tea.Cmd {
	return m.loadSettings()
}

// Update handles messages
func (m *SettingsModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case SettingsLoadedMsg:
		m.settings = msg.Settings
		m.populateInputs()
		m.SetLoading(false)

	case SettingsErrorMsg:
		m.SetError(msg.Err.Error())
		m.SetLoading(false)

	case SettingsSavedMsg:
		// Settings saved successfully
		m.editMode = false
	}

	// Update focused input for system settings
	if len(m.inputs) > 0 && m.focusedInput < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusedInput], cmd = m.inputs[m.focusedInput].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input
func (m *SettingsModel) handleKeyPress(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	return m.handleSystemKeys(msg)
}

// handleSystemKeys handles keys for system settings view
func (m *SettingsModel) handleSystemKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		// Move to next editable input
		m.inputs[m.focusedInput].Blur()
		for i := m.focusedInput + 1; i < len(m.inputs); i++ {
			if !m.settingFields[i].ReadOnly {
				m.focusedInput = i
				m.inputs[m.focusedInput].Focus()
				break
			}
		}
		return m, nil

	case "shift+tab", "up":
		// Move to previous editable input
		m.inputs[m.focusedInput].Blur()
		for i := m.focusedInput - 1; i >= 0; i-- {
			if !m.settingFields[i].ReadOnly {
				m.focusedInput = i
				m.inputs[m.focusedInput].Focus()
				break
			}
		}
		return m, nil

	case "ctrl+s":
		// Save settings
		return m, m.saveSettings()

	case "ctrl+r":
		// Reset to defaults
		return m, m.resetSettings()

	case "esc":
		// Auto-save settings if any have changed, then exit edit mode
		cmd := m.saveSettings()
		m.editMode = false
		return m, cmd
	}
	return m, nil
}


// View renders the settings tab
func (m SettingsModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}

	if m.GetError() != "" {
		return styles.ErrorStyle.Render("Error: " + m.GetError())
	}

	return m.renderSystemSettings()
}

// RefreshData reloads settings from database
func (m SettingsModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadSettings()
}

// Load settings from database
func (m SettingsModel) loadSettings() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Load real settings from database
		// For now, return mock data
		settings := map[string]string{
			"server_host":     "localhost",
			"server_port":     "2222",
			"db_path":         "./station.db",
			"log_level":       "info",
			"max_agents":      "10",
			"session_timeout": "24h",
		}

		return SettingsLoadedMsg{Settings: settings}
	})
}


// Populate input fields with loaded settings
func (m *SettingsModel) populateInputs() {
	settingKeys := []string{
		"server_host",
		"server_port",
		"db_path",
		"log_level",
		"max_agents",
		"session_timeout",
	}

	for i, key := range settingKeys {
		if i < len(m.inputs) {
			if value, exists := m.settings[key]; exists {
				m.inputs[i].SetValue(value)
			}
		}
	}
}

// Render settings form
func (m SettingsModel) renderSettingsForm() string {
	var sections []string

	// Header
	header := components.RenderSectionHeader("System Settings")
	sections = append(sections, header)
	sections = append(sections, "")

	// Settings form
	form := m.renderForm()
	sections = append(sections, form)

	// System info section
	sysInfo := m.renderSystemInfo()
	sections = append(sections, sysInfo)

	// Help text
	helpText := styles.HelpStyle.Render("• tab/↑↓: navigate • ctrl+s: save • ctrl+r: reset • esc: auto-save & exit")
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render settings form fields
func (m SettingsModel) renderForm() string {
	var formSections []string

	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	readOnlyStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Italic(true)
	
	// Group fields by category
	categories := make(map[string][]int)
	for i, field := range m.settingFields {
		categories[field.Category] = append(categories[field.Category], i)
	}
	
	// Render each category
	for _, category := range []string{"System", "Server"} {
		if fieldIndices, exists := categories[category]; exists {
			// Category header
			categoryHeader := styles.HeaderStyle.Render(category + " Configuration")
			formSections = append(formSections, categoryHeader)
			formSections = append(formSections, "")
			
			// Fields in this category
			for _, i := range fieldIndices {
				field := m.settingFields[i]
				input := m.inputs[i]
				
				// Field label
				label := field.Label + ":"
				if field.ReadOnly {
					label = readOnlyStyle.Render(label + " (read-only)")
				} else if i == m.focusedInput {
					label = styles.HeaderStyle.Render(label)
				} else {
					label = mutedStyle.Render(label)
				}

				// Field input/display
				var inputView string
				if field.ReadOnly {
					// Display as read-only text
					inputView = readOnlyStyle.Render(field.Value)
				} else {
					// Display as editable input
					inputView = input.View()
				}
				
				fieldSection := lipgloss.JoinVertical(
					lipgloss.Left,
					label,
					inputView,
				)

				formSections = append(formSections, fieldSection)
			}
			
			formSections = append(formSections, "") // Space between categories
		}
	}

	form := strings.Join(formSections, "\n")

	return styles.WithBorder(lipgloss.NewStyle()).
		Width(70).
		Padding(1).
		Render(form)
}

// Render system information
func (m SettingsModel) renderSystemInfo() string {
	info := []string{
		"Station Admin Interface v1.0",
		"Built with Bubble Tea and Lipgloss",
		"Database: SQLite",
		"Authentication: SSH Keys + API Tokens",
		"",
		"Status: System running normally",
		"Uptime: 2 days, 14 hours",
		"Connected users: 3",
	}

	content := strings.Join(info, "\n")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		styles.HeaderStyle.Render("System Information"),
		"",
		styles.WithBorder(lipgloss.NewStyle()).
			Width(60).
			Padding(1).
			Render(content),
	)
}

// renderSystemSettings renders the system settings view
func (m SettingsModel) renderSystemSettings() string {
	var sections []string
	
	// Header
	header := components.RenderSectionHeader("System Settings")
	sections = append(sections, header)
	sections = append(sections, "")

	// Settings form
	form := m.renderForm()
	sections = append(sections, form)

	// System info section
	sysInfo := m.renderSystemInfo()
	sections = append(sections, sysInfo)

	// Help text
	helpText := styles.HelpStyle.Render("• tab/↑↓: navigate • ctrl+s: save • ctrl+r: reset • esc: auto-save & exit")
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}


// Save settings command
func (m SettingsModel) saveSettings() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually save settings to database
		// For now, just return success
		return SettingsSavedMsg{}
	})
}

// Reset settings to defaults command
func (m SettingsModel) resetSettings() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Reset inputs to default values
		defaults := map[string]string{
			"server_host":     "localhost",
			"server_port":     "2222",
			"db_path":         "./station.db",
			"log_level":       "info",
			"max_agents":      "10",
			"session_timeout": "24h",
		}

		return SettingsLoadedMsg{Settings: defaults}
	})
}

// IsMainView returns true if in main view (always true for settings)
func (m SettingsModel) IsMainView() bool {
	return true
}

// Navigation methods (unused for simple settings but required by interface)
func (m SettingsModel) CanGoBack() bool {
	return false
}

func (m *SettingsModel) GoBack() tea.Cmd {
	return nil
}

func (m SettingsModel) GetBreadcrumb() string {
	return "Settings"
}