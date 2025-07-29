package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// MCPModel represents the MCP servers configuration tab
type MCPModel struct {
	BaseTabModel
	
	// UI components
	configEditor textarea.Model
	nameInput    textinput.Model
	environmentList list.Model
	
	// Data
	envs         []*models.Environment
	repos        *repositories.Repositories
	
	// State
	mode         MCPMode
	configs      []MCPConfigDisplay
	selectedIdx  int
	showEditor   bool
	selectedEnvID int64
	focusedField MCPField
}

type MCPField int

const (
	MCPFieldName MCPField = iota
	MCPFieldEnvironment
	MCPFieldConfig
)

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
	envList.SetFilteringEnabled(false)
	
	m := &MCPModel{
		BaseTabModel:    NewBaseTabModel(database, "MCP Servers"),
		configEditor:    ta,
		nameInput:       ti,
		environmentList: envList,
		repos:           repos,
		mode:            MCPModeList,
		configs:         []MCPConfigDisplay{},
		selectedIdx:     0,
		showEditor:      false,
		selectedEnvID:   1, // Default to first environment
		focusedField:    MCPFieldName,
	}
	
	// Load environments
	m.loadEnvironments()
	
	return m
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
		case "tab":
			if m.mode == MCPModeEdit {
				// Cycle through fields
				switch m.focusedField {
				case MCPFieldName:
					m.nameInput.Blur()
					m.focusedField = MCPFieldEnvironment
				case MCPFieldEnvironment:
					m.focusedField = MCPFieldConfig
					m.configEditor.Focus()
				case MCPFieldConfig:
					m.configEditor.Blur()
					m.focusedField = MCPFieldName
					m.nameInput.Focus()
				}
				return m, nil
			}
		case "esc":
			if m.mode == MCPModeEdit {
				m.mode = MCPModeList
				m.showEditor = false
				m.nameInput.Blur()
				m.configEditor.Blur()
				m.focusedField = MCPFieldName
				return m, nil
			}
		case "right":
			if m.mode == MCPModeEdit && m.showEditor {
				// Handled by the main tab case above
				return m, nil
			}
		case "ctrl+s":
			if m.mode == MCPModeEdit {
				return m, m.saveConfig()
			}
		}
		
		// Handle input for components based on focused field
		if m.mode == MCPModeEdit {
			switch m.focusedField {
			case MCPFieldName:
				var cmd tea.Cmd
				m.nameInput, cmd = m.nameInput.Update(msg)
				cmds = append(cmds, cmd)
			case MCPFieldEnvironment:
				var cmd tea.Cmd
				m.environmentList, cmd = m.environmentList.Update(msg)
				cmds = append(cmds, cmd)
				// Update selected environment ID
				if selectedItem := m.environmentList.SelectedItem(); selectedItem != nil {
					if envItem, ok := selectedItem.(EnvironmentItem); ok {
						m.selectedEnvID = envItem.env.ID
					}
				}
			case MCPFieldConfig:
				var cmd tea.Cmd
				m.configEditor, cmd = m.configEditor.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}
	
	// Update components when editor is shown - handled by focused field logic above
	// This section is now handled in the switch statement based on focusedField
	
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
	nameStyle := styles.HeaderStyle
	if m.focusedField == MCPFieldName {
		nameStyle = nameStyle.Foreground(styles.Primary)
	}
	nameSection := lipgloss.JoinVertical(
		lipgloss.Left,
		nameStyle.Render("Configuration Name:"),
		m.nameInput.View(),
	)
	sections = append(sections, nameSection)
	sections = append(sections, "")
	
	// Environment selection with constrained height
	envStyle := styles.HeaderStyle
	if m.focusedField == MCPFieldEnvironment {
		envStyle = envStyle.Foreground(styles.Primary)
	}
	
	// Constrain environment list size
	m.environmentList.SetSize(m.width-8, 3)
	
	envSection := lipgloss.JoinVertical(
		lipgloss.Left,
		envStyle.Render("Environment:"),
		styles.WithBorder(lipgloss.NewStyle()).
			Padding(0, 1).
			Height(3).
			Width(m.width - 6).
			Render(m.environmentList.View()),
	)
	sections = append(sections, envSection)
	sections = append(sections, "")
	
	// Config editor with precise height calculation
	// Components that take up space:
	// - Breadcrumb: 1 line + 1 empty = 2
	// - Header: 1 line = 1  
	// - Name section: 1 header + 1 input + 1 empty = 3
	// - Environment section: 1 header + 3 list (bordered) + 1 empty = 7 
	// - Config header: 1 line = 1
	// - Help text: 1 empty + 1 help = 2
	// - Config border padding: 2
	// Total overhead: 18 lines
	usedHeight := 18
	availableHeight := m.height - usedHeight
	if availableHeight < 3 {
		availableHeight = 3 // Absolute minimum for usability
	}
	
	// Set editor size to fit remaining space
	m.configEditor.SetWidth(m.width - 8) // Account for border + padding
	m.configEditor.SetHeight(availableHeight)
	
	configStyle := styles.HeaderStyle
	if m.focusedField == MCPFieldConfig {
		configStyle = configStyle.Foreground(styles.Primary)
	}
	
	editorSection := lipgloss.JoinVertical(
		lipgloss.Left,
		configStyle.Render("Configuration (JSON):"),
		styles.WithBorder(lipgloss.NewStyle()).
			Padding(1).
			Height(availableHeight).
			Width(m.width - 4).
			Render(m.configEditor.View()),
	)
	sections = append(sections, editorSection)
	
	// Help text with updated instructions
	helpText := styles.HelpStyle.Render("• tab: switch fields (name→env→config) • ↑↓: navigate env • ctrl+s: save • esc: cancel")
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

// Load environments from database
func (m *MCPModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		envs, err := m.repos.Environments.List()
		if err != nil {
			return err
		}
		
		m.envs = envs
		
		// Convert to list items
		items := make([]list.Item, len(envs))
		for i, env := range envs {
			items[i] = EnvironmentItem{env: *env}
		}
		m.environmentList.SetItems(items)
		
		// Set default selection to first environment
		if len(envs) > 0 {
			m.selectedEnvID = envs[0].ID
		}
		
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