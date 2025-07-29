package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/tui/components"
	"station/internal/tui/styles"
)

// MCPModel represents the MCP servers configuration tab
type MCPModel struct {
	BaseTabModel
	
	// UI components
	configEditor textarea.Model
	nameInput    textinput.Model
	
	// State
	mode         MCPMode
	configs      []MCPConfigDisplay
	selectedIdx  int
	showEditor   bool
}

type MCPMode int

const (
	MCPModeList MCPMode = iota
	MCPModeEdit
)

type MCPConfigDisplay struct {
	ID      int64
	Name    string
	Version int64
	Updated string
	Size    string
}

// NewMCPModel creates a new MCP model
func NewMCPModel(database db.Database) *MCPModel {
	// Create textarea for config editing
	ta := textarea.New()
	ta.Placeholder = "Paste your MCP server configuration here (JSON format)..."
	ta.SetWidth(60)
	ta.SetHeight(15)
	
	// Create text input for name
	ti := textinput.New()
	ti.Placeholder = "Configuration name"
	ti.Width = 30
	
	return &MCPModel{
		BaseTabModel: NewBaseTabModel(database, "MCP Servers"),
		configEditor: ta,
		nameInput:    ti,
		mode:         MCPModeList,
		showEditor:   false,
	}
}

// Init initializes the MCP tab
func (m MCPModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadConfigs(),
		m.configEditor.Cursor.BlinkCmd(),
	)
}

// Update handles messages
func (m *MCPModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.configEditor.SetWidth(msg.Width - 10)
		
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			if m.mode == MCPModeList {
				m.mode = MCPModeEdit
				m.showEditor = true
				m.nameInput.Focus()
				m.nameInput.SetValue("")
				m.configEditor.SetValue("")
				return m, nil
			}
		case "enter":
			if m.mode == MCPModeList && len(m.configs) > 0 {
				// Edit selected config
				m.mode = MCPModeEdit
				m.showEditor = true
				m.nameInput.Focus()
				// Pre-populate with selected config data
				selected := m.configs[m.selectedIdx]
				m.nameInput.SetValue(selected.Name)
				m.configEditor.SetValue("{\n  \"name\": \"" + selected.Name + "\",\n  \"version\": \"" + fmt.Sprintf("%d", selected.Version) + "\"\n}")
				return m, nil
			}
		case "d":
			if m.mode == MCPModeList && len(m.configs) > 0 {
				// Delete selected config
				return m, m.deleteConfig(m.configs[m.selectedIdx].ID)
			}
		case "up", "k":
			if m.mode == MCPModeList && len(m.configs) > 0 {
				m.selectedIdx = (m.selectedIdx - 1 + len(m.configs)) % len(m.configs)
				return m, nil
			}
		case "down", "j":
			if m.mode == MCPModeList && len(m.configs) > 0 {
				m.selectedIdx = (m.selectedIdx + 1) % len(m.configs)
				return m, nil
			}
		case "esc":
			if m.mode == MCPModeEdit {
				m.mode = MCPModeList
				m.showEditor = false
				m.nameInput.Blur()
				m.configEditor.Blur()
				return m, nil
			}
		case "tab", "right":
			if m.mode == MCPModeEdit && m.showEditor {
				if m.nameInput.Focused() {
					m.nameInput.Blur()
					m.configEditor.Focus()
				} else {
					m.configEditor.Blur()
					m.nameInput.Focus()
				}
				return m, nil
			}
		case "ctrl+s":
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		}
	}
	
	// Update components when editor is shown
	if m.showEditor {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		cmds = append(cmds, cmd)
		
		m.configEditor, cmd = m.configEditor.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// View renders the MCP tab
func (m MCPModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}
	
	if m.showEditor {
		return m.renderEditor()
	}
	
	return m.renderConfigList()
}

// RefreshData reloads MCP configs from database
func (m MCPModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadConfigs()
}

// Render configuration list
func (m MCPModel) renderConfigList() string {
	// Show breadcrumb if available (though main view doesn't need it)
	breadcrumb := ""
	if !m.IsMainView() {
		breadcrumb = "MCP Servers › Configuration List"
	}
	
	header := components.RenderSectionHeader("MCP Server Configurations")
	
	// Config list
	var configItems []string
	configItems = append(configItems, styles.HeaderStyle.Render("Available Configurations:"))
	configItems = append(configItems, "")
	
	for i, config := range m.configs {
		prefix := "  "
		if i == m.selectedIdx {
			prefix = "▶ "
		}
		configLine := fmt.Sprintf("%s (v%d) - Updated %s - Size %s", 
			config.Name, config.Version, config.Updated, config.Size)
		configItems = append(configItems, prefix+configLine)
	}
	
	configList := strings.Join(configItems, "\n")
	
	helpText := styles.HelpStyle.Render("• ↑/↓: navigate • n: new config • enter: edit • d: delete")
	
	var sections []string
	if breadcrumb != "" {
		sections = append(sections, breadcrumb, "")
	}
	sections = append(sections, header, "", configList, "", "", helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render configuration editor
func (m MCPModel) renderEditor() string {
	var sections []string
	
	// Navigation breadcrumb
	breadcrumb := "MCP Servers › Configuration Editor"
	sections = append(sections, breadcrumb, "")
	
	// Header
	header := components.RenderSectionHeader("MCP Configuration Editor")
	sections = append(sections, header)
	
	// Name input
	nameSection := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Configuration Name:"),
		m.nameInput.View(),
	)
	sections = append(sections, nameSection)
	sections = append(sections, "")
	
	// Config editor
	editorSection := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Configuration (JSON):"),
		styles.WithBorder(lipgloss.NewStyle()).
			Padding(1).
			Render(m.configEditor.View()),
	)
	sections = append(sections, editorSection)
	
	// Help text
	helpText := styles.HelpStyle.Render("• tab/→: switch fields • ctrl+s: save • esc: cancel")
	sections = append(sections, "")
	sections = append(sections, helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Load configurations from database (stub)
func (m MCPModel) loadConfigs() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Load real configs from database
		// Mock data for now
		m.configs = []MCPConfigDisplay{
			{ID: 1, Name: "filesystem-tools", Version: 120, Updated: "2 hours ago", Size: "1.2MB"},
			{ID: 2, Name: "browser-automation", Version: 210, Updated: "1 day ago", Size: "2.1MB"},
			{ID: 3, Name: "database-connector", Version: 103, Updated: "3 days ago", Size: "0.8MB"},
		}
		m.SetLoading(false)
		return nil
	})
}

// Save configuration (stub)
func (m MCPModel) saveConfig() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Save config to database
		// For now, just simulate save and return to list
		m.mode = MCPModeList
		m.showEditor = false
		m.nameInput.Blur()
		m.configEditor.Blur()
		return tea.Printf("Configuration saved: %s", m.nameInput.Value())
	})
}

// Delete configuration (stub)
func (m MCPModel) deleteConfig(configID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually delete from database
		// For now, just remove from list
		for i, config := range m.configs {
			if config.ID == configID {
				m.configs = append(m.configs[:i], m.configs[i+1:]...)
				if m.selectedIdx >= len(m.configs) && len(m.configs) > 0 {
					m.selectedIdx = len(m.configs) - 1
				}
				break
			}
		}
		return tea.Printf("Configuration deleted")
	})
}

// IsMainView returns true if in main list view
func (m MCPModel) IsMainView() bool {
	return m.mode == MCPModeList
}