package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// ToolsModel represents the tools dashboard tab
type ToolsModel struct {
	BaseTabModel

	// UI components
	list list.Model

	// Data
	tools        []*models.MCPToolWithDetails
	repos        *repositories.Repositories
	selectedEnv  int64

	// State
	selectedIdx   int
	showDetails   bool
	selectedTool  *models.MCPToolWithDetails
	filterText    string // Custom filter text
	isFiltering   bool   // Whether we're in filter mode
}

// ToolItem implements list.Item interface for bubbles list component
type ToolItem struct {
	tool *models.MCPToolWithDetails
}

// Required by list.Item interface
func (i ToolItem) FilterValue() string {
	// Include tool name, description, server name, config name, version, and environment name for fuzzy search
	return strings.Join([]string{
		i.tool.Name,
		i.tool.Description,
		i.tool.ServerName,
		i.tool.ConfigName,
		fmt.Sprintf("v%d", i.tool.ConfigVersion),
		i.tool.EnvironmentName,
	}, " ")
}

// Required by list.DefaultItem interface
func (i ToolItem) Title() string {
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.SuccessStyle.Render("● "),
		styles.BaseStyle.Render(i.tool.Name),
	)
}

func (i ToolItem) Description() string {
	desc := i.tool.Description
	if len(desc) > 60 {
		desc = desc[:60] + "..."
	}
	
	// Create much more concise metadata - just essential info
	configInfo := fmt.Sprintf("%s v%d", i.tool.ConfigName, i.tool.ConfigVersion)
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	return mutedStyle.Render(desc + " • " + configInfo)
}

// Messages for async operations
type ToolsLoadedMsg struct {
	Tools []*models.MCPToolWithDetails
}

type ToolsErrorMsg struct {
	Err error
}

// NewToolsModel creates a new tools model
func NewToolsModel(database db.Database) *ToolsModel {
	repos := repositories.New(database)

	// Create list - styles will be set dynamically in WindowSizeMsg handler
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Available Tools"
	l.Styles.Title = styles.HeaderStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	l.Styles.HelpStyle = styles.HelpStyle
	// Disable built-in filtering to avoid character encoding issues
	l.SetFilteringEnabled(false)

	return &ToolsModel{
		BaseTabModel: NewBaseTabModel(database, "Tools"),
		list:         l,
		repos:        repos,
		selectedEnv:  1, // Default to first environment
		tools:        []*models.MCPToolWithDetails{},
	}
}

// Init initializes the tools tab
func (m ToolsModel) Init() tea.Cmd {
	return m.loadTools()
}

// Update handles messages
func (m *ToolsModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		// Update list size based on the content area dimensions calculated by TUI
		// Reserve space for section header and help text (approximately 4 lines)
		listWidth := msg.Width - 4
		m.list.SetSize(listWidth, msg.Height-4)
		
		// Update delegate styles to use full width for proper selection highlighting
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = styles.GetListItemSelectedStyle(msg.Width)
		delegate.Styles.SelectedDesc = styles.GetListItemSelectedStyle(msg.Width)
		delegate.Styles.NormalTitle = styles.GetListItemStyle(msg.Width)
		delegate.Styles.NormalDesc = styles.GetListItemStyle(msg.Width)
		m.list.SetDelegate(delegate)

	case tea.KeyMsg:
		// Handle filter mode first
		if m.isFiltering {
			switch msg.String() {
			case "esc":
				// Exit filter mode
				m.isFiltering = false
				m.filterText = ""
				m.updateFilteredItems()
				return m, nil
			case "backspace":
				// Remove last character from filter
				if len(m.filterText) > 0 {
					m.filterText = m.filterText[:len(m.filterText)-1]
					m.updateFilteredItems()
				}
				return m, nil
			case "enter":
				// Exit filter mode and accept current filter
				m.isFiltering = false
				return m, nil
			default:
				// Add character to filter (only printable characters)
				if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
					m.filterText += msg.String()
					m.updateFilteredItems()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "enter":
			// Handle tool selection
			if len(m.tools) > 0 {
				if item, ok := m.list.SelectedItem().(ToolItem); ok {
					m.selectedTool = item.tool
					m.showDetails = true
				}
			}
			return m, nil
		case "esc":
			if m.showDetails {
				m.showDetails = false
				return m, nil
			}
		case "/":
			// Enter filter mode
			m.isFiltering = true
			return m, nil
		case "r":
			// Refresh tools
			return m, m.loadTools()
		}

	case ToolsLoadedMsg:
		m.tools = msg.Tools
		m.updateListItems()
		m.SetLoading(false)

	case ToolsErrorMsg:
		m.SetError(msg.Err.Error())
		m.SetLoading(false)

	case MCPToolDiscoveryCompletedMsg:
		// When tool discovery completes in MCP tab, refresh our tools list
		if msg.Success {
			return m, m.loadTools()
		}

	case MCPConfigDeletedMsg:
		// When MCP config is deleted, refresh our tools list to remove orphaned tools
		return m, m.loadTools()
	}

	// Update list component
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// updateFilteredItems filters the tools list based on current filter text
func (m *ToolsModel) updateFilteredItems() {
	if m.filterText == "" {
		// No filter, show all tools
		items := make([]list.Item, len(m.tools))
		for i, tool := range m.tools {
			items[i] = ToolItem{tool: tool}
		}
		m.list.SetItems(items)
		return
	}

	// Filter tools based on text
	var filtered []*models.MCPToolWithDetails
	filterLower := strings.ToLower(m.filterText)
	
	for _, tool := range m.tools {
		if strings.Contains(strings.ToLower(tool.Name), filterLower) ||
		   strings.Contains(strings.ToLower(tool.Description), filterLower) ||
		   strings.Contains(strings.ToLower(tool.ServerName), filterLower) ||
		   strings.Contains(strings.ToLower(tool.ConfigName), filterLower) ||
		   strings.Contains(strings.ToLower(tool.EnvironmentName), filterLower) {
			filtered = append(filtered, tool)
		}
	}
	
	// Update list items
	items := make([]list.Item, len(filtered))
	for i, tool := range filtered {
		items[i] = ToolItem{tool: tool}
	}
	m.list.SetItems(items)
}

// View renders the tools tab
func (m ToolsModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}

	if m.GetError() != "" {
		return styles.ErrorStyle.Render("Error loading tools: " + m.GetError())
	}

	if m.showDetails && m.selectedTool != nil {
		return m.renderToolDetails()
	}

	return m.renderToolsList()
}

// RefreshData reloads tools from database
func (m ToolsModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadTools()
}

// Load tools from database
func (m ToolsModel) loadTools() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		tools, err := m.repos.MCPTools.GetAllWithDetails()
		if err != nil {
			return ToolsErrorMsg{Err: err}
		}
		return ToolsLoadedMsg{Tools: tools}
	})
}

// Update list items from tools data
func (m *ToolsModel) updateListItems() {
	// Use the filtered items method to respect any active filter
	m.updateFilteredItems()
}

// Render tools list view
func (m ToolsModel) renderToolsList() string {
	// Header with stats
	header := components.RenderSectionHeader(fmt.Sprintf("Available Tools (%d total)", len(m.tools)))

	var sections []string
	sections = append(sections, header)

	// Show filter if active
	if m.isFiltering {
		filterLabel := lipgloss.NewStyle().Foreground(styles.Primary).Render("Filter: /" + m.filterText)
		sections = append(sections, filterLabel)
	}

	// List component
	listView := m.list.View()
	sections = append(sections, listView)
	sections = append(sections, "")

	// Help text
	var helpText string
	if m.isFiltering {
		helpText = "• enter: accept filter • esc: clear filter • backspace: delete char"
	} else {
		helpText = "• enter: view details • /: filter • r: refresh tools"
	}
	sections = append(sections, styles.HelpStyle.Render(helpText))

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render tool details view
func (m ToolsModel) renderToolDetails() string {
	if m.selectedTool == nil {
		return styles.ErrorStyle.Render("Tool not found")
	}

	var sections []string

	// Header
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Tool Details: "),
		primaryStyle.Render(m.selectedTool.Name),
	)
	sections = append(sections, header)
	sections = append(sections, "")

	// Basic info
	info := m.renderToolInfo(m.selectedTool)
	sections = append(sections, info)

	// Schema
	schema := m.renderToolSchema(m.selectedTool)
	sections = append(sections, schema)

	// Back instruction
	backText := styles.HelpStyle.Render("Press esc to go back to list")
	sections = append(sections, "")
	sections = append(sections, backText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render tool basic information
func (m ToolsModel) renderToolInfo(tool *models.MCPToolWithDetails) string {
	fields := []string{
		fmt.Sprintf("ID: %d", tool.ID),
		fmt.Sprintf("Config: %s (v%d)", tool.ConfigName, tool.ConfigVersion),
		fmt.Sprintf("Server: %s", tool.ServerName),
		fmt.Sprintf("Environment: %s", tool.EnvironmentName),
		fmt.Sprintf("Description: %s", tool.Description),
	}

	content := strings.Join(fields, "\n")

	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(content)
}

// Render tool schema information
func (m ToolsModel) renderToolSchema(tool *models.MCPToolWithDetails) string {
	// Pretty print the JSON schema
	schemaText := string(tool.Schema)
	if len(schemaText) > 300 {
		schemaText = schemaText[:300] + "..."
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Input Schema:"),
		"",
		lipgloss.NewStyle().Foreground(styles.TextMuted).Render(schemaText),
	)

	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Height(12).
		Padding(1).
		Render(content)
}

// IsMainView returns true if in main list view
func (m ToolsModel) IsMainView() bool {
	return !m.showDetails
}