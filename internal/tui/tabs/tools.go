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
	selectedIdx  int
	showDetails  bool
	selectedTool *models.MCPToolWithDetails
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
	if len(desc) > 30 {
		desc = desc[:30] + "..."
	}
	
	// Create config, server and environment info
	configInfo := fmt.Sprintf("Config: %s v%d", i.tool.ConfigName, i.tool.ConfigVersion)
	serverInfo := fmt.Sprintf("Server: %s", i.tool.ServerName)
	envInfo := fmt.Sprintf("Env: %s", i.tool.EnvironmentName)
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		mutedStyle.Render(desc),
		mutedStyle.Render(" • "),
		mutedStyle.Render(configInfo),
		mutedStyle.Render(" • "),
		mutedStyle.Render(serverInfo),
		mutedStyle.Render(" • "),
		mutedStyle.Render(envInfo),
	)
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

	// Create list with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = styles.ListItemSelectedStyle
	delegate.Styles.SelectedDesc = styles.ListItemSelectedStyle
	delegate.Styles.NormalTitle = styles.ListItemStyle
	delegate.Styles.NormalDesc = styles.ListItemStyle

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Available Tools"
	l.Styles.Title = styles.HeaderStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	l.Styles.HelpStyle = styles.HelpStyle

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
		m.list.SetSize(msg.Width-4, msg.Height-8) // Account for borders and header

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
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
	}

	// Update list component
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
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
	items := make([]list.Item, len(m.tools))
	for i, tool := range m.tools {
		items[i] = ToolItem{tool: tool}
	}
	m.list.SetItems(items)
}

// Render tools list view
func (m ToolsModel) renderToolsList() string {
	// Header with stats
	header := components.RenderSectionHeader(fmt.Sprintf("Available Tools (%d total)", len(m.tools)))

	// List component
	listView := m.list.View()

	// Help text
	helpText := styles.HelpStyle.Render("• enter: view details • r: refresh tools • Environment: default")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listView,
		"",
		helpText,
	)
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