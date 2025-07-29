package tabs

import (
	"fmt"
	"strings"
	"time"
	
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// AgentsModel represents the agents management tab
type AgentsModel struct {
	BaseTabModel
	
	// UI components
	list         list.Model
	
	// Data access
	repos        *repositories.Repositories
	
	// State
	agents        []models.Agent
	selectedAgent *models.Agent
}

// AgentItem implements list.Item interface for bubbles list component
type AgentItem struct {
	agent models.Agent
}

// Required by list.Item interface
func (i AgentItem) FilterValue() string {
	return i.agent.Name
}

// Required by list.DefaultItem interface
func (i AgentItem) Title() string {
	status := "●"
	statusStyle := styles.SuccessStyle
	
	// TODO: Get actual agent status from database
	// For now, show all as active
	
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusStyle.Render(status+" "),
		styles.BaseStyle.Render(i.agent.Name),
	)
}

func (i AgentItem) Description() string {
	createdAt := i.agent.CreatedAt.Format("Jan 2, 2006")
	env := fmt.Sprintf("Env: %d", i.agent.EnvironmentID)
	steps := fmt.Sprintf("Max steps: %d", i.agent.MaxSteps)
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		mutedStyle.Render(i.agent.Description+" • "),
		mutedStyle.Render(createdAt+" • "),
		mutedStyle.Render(env+" • "),
		mutedStyle.Render(steps),
	)
}

// Custom key bindings for agents list
type agentsKeyMap struct {
	list.KeyMap
	showDetails  key.Binding
	createAgent  key.Binding
	deleteAgent  key.Binding
	runAgent     key.Binding
}

func newAgentsKeyMap() agentsKeyMap {
	listKeys := list.DefaultKeyMap()
	
	return agentsKeyMap{
		KeyMap: listKeys,
		showDetails: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "view details"),
		),
		createAgent: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new agent"),
		),
		deleteAgent: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		runAgent: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "run agent"),
		),
	}
}

// Messages for async operations
type AgentsLoadedMsg struct {
	Agents []models.Agent
}

type AgentsErrorMsg struct {
	Err error
}

type AgentCreatedMsg struct {
	Agent models.Agent
}

type AgentDeletedMsg struct {
	AgentID int64
}

// NewAgentsModel creates a new agents model
func NewAgentsModel(database db.Database) *AgentsModel {
	repos := repositories.New(database)
	// Create list with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = styles.ListItemSelectedStyle
	delegate.Styles.SelectedDesc = styles.ListItemSelectedStyle
	delegate.Styles.NormalTitle = styles.ListItemStyle
	delegate.Styles.NormalDesc = styles.ListItemStyle
	
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Station Agents"
	l.Styles.Title = styles.HeaderStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	l.Styles.HelpStyle = styles.HelpStyle
	
	// Set custom key bindings
	keyMap := newAgentsKeyMap()
	l.KeyMap = keyMap.KeyMap
	
	return &AgentsModel{
		BaseTabModel: NewBaseTabModel(database, "Agents"),
		list:         l,
		repos:        repos,
	}
}

// Init initializes the agents tab
func (m AgentsModel) Init() tea.Cmd {
	return m.loadAgents()
}

// Update handles messages
func (m *AgentsModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.list.SetSize(msg.Width-4, msg.Height-8) // Account for borders and header
		
	case tea.KeyMsg:
		
		// Handle back navigation first
		switch msg.String() {
		case "esc", "backspace":
			if m.CanGoBack() {
				m.GoBack()
				m.selectedAgent = nil
				return m, nil
			}
		}
		
		// Handle view-specific keys with simpler string matching
		if m.GetViewMode() == "list" {
			switch msg.String() {
			case "enter", " ":
				// Show agent details
				if len(m.agents) > 0 {
					if item, ok := m.list.SelectedItem().(AgentItem); ok {
						m.selectedAgent = &item.agent
						m.PushNavigation(item.agent.Name)
						m.SetViewMode("detail")
					}
				}
				return m, nil
				
			case "n":
				// Create new agent - show create form
				m.PushNavigation("Create Agent")
				m.SetViewMode("create")
				return m, nil
				
			case "d":
				// Delete agent
				if len(m.agents) == 0 {
					return m, tea.Printf("No agents to delete")
				}
				if item, ok := m.list.SelectedItem().(AgentItem); ok {
					return m, m.deleteAgent(item.agent.ID)
				}
				return m, nil
				
			case "r":
				// Run agent
				if len(m.agents) == 0 {
					return m, tea.Printf("No agents to run")
				}
				if item, ok := m.list.SelectedItem().(AgentItem); ok {
					return m, tea.Printf("Running agent: %s", item.agent.Name)
				}
				return m, nil
			}
			
			// List updates handled at end of Update method to avoid duplication
		}
		
	case AgentsLoadedMsg:
		m.agents = msg.Agents
		m.updateListItems()
		m.SetLoading(false)
		
	case AgentsErrorMsg:
		m.SetError(msg.Err.Error())
		m.SetLoading(false)
		
	case AgentDeletedMsg:
		// Remove deleted agent from list
		for i, agent := range m.agents {
			if agent.ID == msg.AgentID {
				m.agents = append(m.agents[:i], m.agents[i+1:]...)
				break
			}
		}
		m.updateListItems()
		m.SetViewMode("list")
		m.selectedAgent = nil
	}
	
	// Update list component
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

// View renders the agents tab
func (m AgentsModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}
	
	if m.GetError() != "" {
		return styles.ErrorStyle.Render("Error loading agents: " + m.GetError())
	}
	
	var content string
	switch m.GetViewMode() {
	case "detail":
		content = m.renderAgentDetails()
	case "create":
		content = m.renderCreateAgentForm()
	default:
		content = m.renderAgentsList()
	}
	
	// Add navigation hint for sub-views
	if m.GetViewMode() != "list" {
		navHint := styles.HelpStyle.Render("Press ESC to go back")
		content = lipgloss.JoinVertical(lipgloss.Left, navHint, "", content)
	}
	
	return content
}

// RefreshData reloads agents from database
func (m AgentsModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadAgents()
}

// Get key map for custom bindings
func (m AgentsModel) getKeyMap() agentsKeyMap {
	// This is a bit of a hack to get the custom key map
	// In a real implementation, you'd store this in the model
	return newAgentsKeyMap()
}

// Load agents from database
func (m AgentsModel) loadAgents() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Try to load real agents from database first
		agents, err := m.repos.Agents.List()
		if err != nil || len(agents) == 0 {
			// Fallback to sample data if database is empty
			sampleAgents := []*models.Agent{
				{
					ID:            1,
					Name:          "Code Reviewer",
					Description:   "Reviews code for quality and security issues",
					Prompt:        "You are a code reviewer...",
					MaxSteps:      10,
					EnvironmentID: 1,
					CreatedBy:     1,
					CreatedAt:     time.Now().Add(-time.Hour * 24),
					UpdatedAt:     time.Now().Add(-time.Hour * 12),
				},
				{
					ID:            2,
					Name:          "Security Scanner",
					Description:   "Scans for security vulnerabilities",
					Prompt:        "You are a security expert...",
					MaxSteps:      12,
					EnvironmentID: 1,
					CreatedBy:     1,
					CreatedAt:     time.Now().Add(-time.Hour * 48),
					UpdatedAt:     time.Now().Add(-time.Hour * 24),
				},
				{
					ID:            3,
					Name:          "Performance Monitor",
					Description:   "Monitors system performance metrics",
					Prompt:        "You are a performance monitoring expert...",
					MaxSteps:      8,
					EnvironmentID: 1,
					CreatedBy:     1,
					CreatedAt:     time.Now().Add(-time.Hour * 72),
					UpdatedAt:     time.Now().Add(-time.Hour * 36),
				},
			}
			
			// Convert to the expected format
			var convertedAgents []models.Agent
			for _, agent := range sampleAgents {
				convertedAgents = append(convertedAgents, *agent)
			}
			
			return AgentsLoadedMsg{Agents: convertedAgents}
		} else {
			// Convert from []*models.Agent to []models.Agent
			var convertedAgents []models.Agent
			for _, agent := range agents {
				convertedAgents = append(convertedAgents, *agent)
			}
			return AgentsLoadedMsg{Agents: convertedAgents}
		}
	})
}

// Update list items from agents data
func (m *AgentsModel) updateListItems() {
	items := make([]list.Item, len(m.agents))
	for i, agent := range m.agents {
		items[i] = AgentItem{agent: agent}
	}
	m.list.SetItems(items)
}

// Render agents list view
func (m AgentsModel) renderAgentsList() string {
	// Navigation breadcrumbs
	breadcrumb := m.renderBreadcrumb()
	
	// Header with stats
	header := components.RenderSectionHeader(fmt.Sprintf("Agents (%d total)", len(m.agents)))
	
	// List component
	listView := m.list.View()
	
	// Help text
	helpText := styles.HelpStyle.Render("• enter: view details • n: new agent • d: delete • r: run agent")
	
	var sections []string
	if breadcrumb != "" {
		sections = append(sections, breadcrumb)
	}
	sections = append(sections, header, listView, "", helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render breadcrumb navigation
func (m AgentsModel) renderBreadcrumb() string {
	if !m.CanGoBack() {
		return ""
	}
	
	breadcrumbText := m.GetBreadcrumb()
	backText := " (Press ESC to go back)"
	
	breadcrumbStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Padding(0, 1)
	
	backStyle := lipgloss.NewStyle().
		Foreground(styles.Secondary).
		Italic(true)
	
	content := breadcrumbStyle.Render(breadcrumbText) + backStyle.Render(backText)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		content,
		"",
	)
}

// Render agent details view
func (m AgentsModel) renderAgentDetails() string {
	if m.selectedAgent == nil {
		return styles.ErrorStyle.Render("No agent selected")
	}
	
	agent := m.selectedAgent
	
	// Agent details layout
	var sections []string
	
	// Header
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Agent Details: "),
		primaryStyle.Render(agent.Name),
	)
	sections = append(sections, header)
	sections = append(sections, "")
	
	// Basic info
	info := m.renderAgentInfo(agent)
	sections = append(sections, info)
	
	// Prompt preview
	prompt := m.renderPromptPreview(agent)
	sections = append(sections, prompt)
	
	// Actions
	actions := m.renderAgentActions()
	sections = append(sections, actions)
	
	// Back instruction
	backText := styles.HelpStyle.Render("Press ESC to go back to list")
	sections = append(sections, "")
	sections = append(sections, backText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render agent basic information
func (m AgentsModel) renderAgentInfo(agent *models.Agent) string {
	fields := []string{
		fmt.Sprintf("ID: %d", agent.ID),
		fmt.Sprintf("Description: %s", agent.Description),
		fmt.Sprintf("Environment ID: %d", agent.EnvironmentID),
		fmt.Sprintf("Max Steps: %d", agent.MaxSteps),
		fmt.Sprintf("Created: %s", agent.CreatedAt.Format("Jan 2, 2006 15:04")),
		fmt.Sprintf("Updated: %s", agent.UpdatedAt.Format("Jan 2, 2006 15:04")),
		fmt.Sprintf("Created By: User %d", agent.CreatedBy),
	}
	
	content := strings.Join(fields, "\n")
	
	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(content)
}

// Render prompt preview
func (m AgentsModel) renderPromptPreview(agent *models.Agent) string {
	prompt := agent.Prompt
	if len(prompt) > 200 {
		prompt = prompt[:200] + "..."
	}
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("System Prompt:"),
		"",
		mutedStyle.Render(prompt),
	)
	
	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Height(8).
		Padding(1).
		Render(content)
}

// Render agent action buttons
func (m AgentsModel) renderAgentActions() string {
	runBtn := styles.ButtonActiveStyle.Render("Run Agent")
	editBtn := styles.ButtonStyle.Render("Edit")
	deleteBtn := styles.ErrorStyle.Render("Delete")
	
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		runBtn,
		editBtn,
		deleteBtn,
	)
}

// Render create agent form
func (m AgentsModel) renderCreateAgentForm() string {
	var sections []string
	
	// Header
	header := components.RenderSectionHeader("Create New Agent")
	sections = append(sections, header)
	sections = append(sections, "")
	
	// Form placeholder
	formContent := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Agent Configuration:"),
		"",
		styles.BaseStyle.Render("• Name: [Input field coming soon]"),
		styles.BaseStyle.Render("• Description: [Input field coming soon]"),
		styles.BaseStyle.Render("• Environment: [Dropdown coming soon]"),
		styles.BaseStyle.Render("• System Prompt: [Textarea coming soon]"),
		"",
		lipgloss.NewStyle().Foreground(styles.TextMuted).Render("This feature is under development."),
	)
	
	sections = append(sections, styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(2).
		Render(formContent))
	
	// Help text
	helpText := styles.HelpStyle.Render("• ESC: go back to list")
	sections = append(sections, "")
	sections = append(sections, helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Navigation interface implementation
func (m AgentsModel) CanGoBack() bool {
	return m.BaseTabModel.CanGoBack()
}

func (m *AgentsModel) GoBack() tea.Cmd {
	return m.BaseTabModel.GoBack()
}

func (m AgentsModel) GetBreadcrumb() string {
	return m.BaseTabModel.GetBreadcrumb()
}

// Delete agent command
func (m AgentsModel) deleteAgent(agentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually delete from database
		// For now, just return success
		return AgentDeletedMsg{AgentID: agentID}
	})
}

// IsMainView returns true if in main agents list view
func (m AgentsModel) IsMainView() bool {
	// Use the base implementation for reliable navigation
	return m.BaseTabModel.IsMainView()
}