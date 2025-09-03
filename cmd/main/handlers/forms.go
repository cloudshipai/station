package handlers

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"station/cmd/main/handlers/mcp"
	"station/internal/theme"
)

// Spinner model for loading states
type SpinnerModel struct {
	spinner    spinner.Model
	message    string
	finished   bool
	success    bool
	result     string
	err        error
	
	// Fields for add-server command
	configID   string
	serverName string
	env        string
	command    string
	args       []string
	envVars    map[string]string
	
	themeManager *theme.ThemeManager
}

// Interactive form model for MCP Add command
type MCPAddFormModel struct {
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
	
	themeManager *theme.ThemeManager
}

func NewMCPAddForm(environment string, themeManager *theme.ThemeManager) *MCPAddFormModel {
	m := &MCPAddFormModel{
		inputs:       make([]textinput.Model, 4), // 4 main fields
		environment:  environment,
		envVars:      make(map[string]string),
		themeManager: themeManager,
	}
	
	styles := getCLIStyles(themeManager)
	
	// Config ID input
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "my-config or config-123"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 50
	m.inputs[0].Width = 40
	m.inputs[0].Prompt = styles.Focused.Render("▶ ")
	
	// Server Name input
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "filesystem"
	m.inputs[1].CharLimit = 50
	m.inputs[1].Width = 40
	m.inputs[1].Prompt = styles.Blurred.Render("▶ ")
	
	// Command input
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "npx"
	m.inputs[2].CharLimit = 100
	m.inputs[2].Width = 40
	m.inputs[2].Prompt = styles.Blurred.Render("▶ ")
	
	// Args input
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "-y @modelcontextprotocol/server-filesystem /path"
	m.inputs[3].CharLimit = 200
	m.inputs[3].Width = 40
	m.inputs[3].Prompt = styles.Blurred.Render("▶ ")
	
	return m
}

func (m *MCPAddFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *MCPAddFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *MCPAddFormModel) updateFocus() {
	styles := getCLIStyles(m.themeManager)
	for i := range m.inputs {
		if i == m.focused {
			m.inputs[i].Focus()
			m.inputs[i].Prompt = styles.Focused.Render("▶ ")
			m.inputs[i].TextStyle = styles.Focused
		} else {
			m.inputs[i].Blur()
			m.inputs[i].Prompt = styles.Blurred.Render("▶ ")
			m.inputs[i].TextStyle = styles.No
		}
	}
}

func (m *MCPAddFormModel) View() string {
	styles := getCLIStyles(m.themeManager)
	if m.done {
		return styles.Success.Render("✓ Configuration collected!")
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
			style = styles.Focused
		} else {
			style = styles.Blurred
		}
		
		b.WriteString(style.Render(field) + "\n")
		b.WriteString(m.inputs[i].View() + "\n\n")
	}
	
	// Help text at bottom
	help := styles.Help.Render("↑/↓: navigate • enter: next/submit • ctrl+c: cancel")
	
	// Validation message
	validation := ""
	if m.inputs[0].Value() == "" || m.inputs[1].Value() == "" || m.inputs[2].Value() == "" {
		validation = styles.Error.Render("⚠ All fields except arguments are required")
	} else {
		validation = styles.Success.Render("✓ Ready to submit")
	}
	
	form := styles.Form.Render(b.String() + validation + "\n\n" + help)
	
	return form
}

func NewSpinnerModel(message string, themeManager *theme.ThemeManager) SpinnerModel {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))

	return SpinnerModel{
		spinner:      s,
		message:      message,
		themeManager: themeManager,
	}
}

func NewSpinnerModelWithServerConfig(message, configID, serverName, command string, args []string, envVars map[string]string, env string, themeManager *theme.ThemeManager) SpinnerModel {
	s := spinner.New()
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))

	return SpinnerModel{
		spinner:      s,
		message:      message,
		configID:     configID,
		serverName:   serverName,
		env:          env,
		command:      command,
		args:         args,
		envVars:      envVars,
		themeManager: themeManager,
	}
}

func (m SpinnerModel) Init() tea.Cmd {
	// Check if this is an add-server operation
	if m.configID != "" && m.serverName != "" {
		return tea.Batch(m.spinner.Tick, func() tea.Msg {
			handler := mcp.NewMCPHandler(m.themeManager)
			result, err := handler.AddServerToConfig(m.configID, m.serverName, m.command, m.args, m.envVars, m.env)
			return FinishedMsg{
				success: err == nil,
				result:  result,
				err:     err,
			}
		})
	}
	
	return m.spinner.Tick
}

func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case FinishedMsg:
		m.finished = true
		m.success = msg.success
		m.result = msg.result
		m.err = msg.err
		return m, tea.Quit
	}
	return m, nil
}

func (m SpinnerModel) View() string {
	if m.finished {
		styles := getCLIStyles(m.themeManager)
		if m.success {
			return styles.Success.Render("✅ " + m.result)
		} else {
			return styles.Error.Render("❌ " + m.err.Error())
		}
	}
	return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
}

type FinishedMsg struct {
	success bool
	result  string
	err     error
}

// Getter methods for MCPAddFormModel
func (m *MCPAddFormModel) GetConfigID() string     { return m.configID }
func (m *MCPAddFormModel) GetServerName() string   { return m.serverName }
func (m *MCPAddFormModel) GetCommand() string      { return m.command }
func (m *MCPAddFormModel) GetArgs() []string       { return m.args }
func (m *MCPAddFormModel) GetEnvVars() map[string]string { return m.envVars }
func (m *MCPAddFormModel) GetEnvironment() string  { return m.environment }
func (m *MCPAddFormModel) IsCancelled() bool       { return m.cancelled }

// Getter methods for SpinnerModel
func (m *SpinnerModel) GetResult() string { return m.result }
func (m *SpinnerModel) GetError() error   { return m.err }
func (m *SpinnerModel) IsFinished() bool  { return m.finished }
func (m *SpinnerModel) IsSuccess() bool   { return m.success }