package tabs

import (
	"fmt"
	"strings"
	"time"
	
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// AgentsModel represents the agents management tab
type AgentsModel struct {
	BaseTabModel
	
	// UI components
	list         list.Model
	
	// Create form components
	nameInput         textinput.Model
	descInput         textinput.Model
	promptArea        textarea.Model
	scheduleInput     textinput.Model
	scheduleEnabled   bool
	
	// Data access
	repos          *repositories.Repositories
	executionQueue *services.ExecutionQueueService
	
	// State
	agents        []models.Agent
	selectedAgent *models.Agent
	
	// Create form state
	environments    []models.Environment
	availableTools  []models.MCPToolWithDetails  // Changed to include environment/config info
	selectedEnvID   int64
	selectedToolIDs []int64
	focusedField    AgentFormField
	toolCursor      int    // Track which tool is currently highlighted
	toolsOffset     int    // Track scroll position in tools list
	toolsFilter     string // Filter text for tools
	isFiltering     bool   // Whether we're in filter mode
	
	// Detail view state
	actionButtonIndex int  // Track which action button is selected (0=Run, 1=Edit, 2=Delete)
	assignedTools     []models.AgentToolWithDetails  // Tools assigned to selected agent
	detailsScrollOffset int // Manual scroll offset for agent details
	detailsViewport   viewport.Model  // Viewport for scrollable agent details
}

type AgentFormField int

const (
	AgentFieldName AgentFormField = iota
	AgentFieldDesc
	AgentFieldEnvironment
	AgentFieldPrompt
	AgentFieldTools
	AgentFieldSchedule
	AgentFieldScheduleEnabled
)

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

type AgentUpdatedMsg struct {
	Agent models.Agent
}

type AgentsEnvironmentsLoadedMsg struct {
	Environments []models.Environment
}

type AgentsToolsLoadedMsg struct {
	Tools []models.MCPToolWithDetails
}

type AgentToolsLoadedMsg struct {
	AgentID int64
	Tools   []models.AgentToolWithDetails
}

// NewAgentsModel creates a new agents model
func NewAgentsModel(database db.Database, executionQueue *services.ExecutionQueueService) *AgentsModel {
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
	
	// Create form inputs
	nameInput := textinput.New()
	nameInput.Placeholder = "Agent name"
	nameInput.Width = 40
	
	descInput := textinput.New()
	descInput.Placeholder = "Agent description"
	descInput.Width = 60
	
	promptArea := textarea.New()
	promptArea.Placeholder = "Enter system prompt for the agent..."
	promptArea.SetWidth(80)  // Wider for right column
	promptArea.SetHeight(12) // Much taller for better editing
	
	scheduleInput := textinput.New()
	scheduleInput.Placeholder = "* * * * * (cron expression, e.g., 0 0 * * * for daily at midnight)"
	scheduleInput.Width = 60
	
	// Initialize viewport for details view
	detailsViewport := viewport.New(80, 20)
	detailsViewport.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.TextMuted)
	
	return &AgentsModel{
		BaseTabModel:    NewBaseTabModel(database, "Agents"),
		list:            l,
		repos:           repos,
		executionQueue:  executionQueue,
		nameInput:       nameInput,
		descInput:       descInput,
		promptArea:      promptArea,
		scheduleInput:   scheduleInput,
		scheduleEnabled: false,
		focusedField:    AgentFieldName,
		detailsViewport: detailsViewport,
	}
}

// Init initializes the agents tab
func (m AgentsModel) Init() tea.Cmd {
	return tea.Batch(
		m.loadAgents(),
		m.loadEnvironments(),
		// Note: tools will be loaded after environments are loaded
	)
}

// Update handles messages
func (m *AgentsModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.list.SetSize(msg.Width-4, msg.Height-8) // Account for borders and header
		// Update viewport size for details view
		m.detailsViewport.Width = msg.Width - 4
		m.detailsViewport.Height = msg.Height - 8
		
	case tea.KeyMsg:
		
		// Handle back navigation first (but only for ESC, not backspace in forms)
		switch msg.String() {
		case "esc":
			// Only handle ESC for navigation if we're NOT in a form
			if m.CanGoBack() && m.GetViewMode() != "create" && m.GetViewMode() != "edit" {
				m.GoBack()
				m.selectedAgent = nil
				m.actionButtonIndex = 0  // Reset button selection
				return m, nil
			}
		}
		
		// Handle view-specific keys with simpler string matching
		if m.GetViewMode() == "create" {
			return m.handleCreateFormKeys(msg)
		} else if m.GetViewMode() == "edit" {
			return m.handleEditFormKeys(msg)
		} else if m.GetViewMode() == "detail" {
			return m.handleDetailViewKeys(msg)
		} else if m.GetViewMode() == "list" {
			switch msg.String() {
			case "enter", " ":
				// Show agent details
				if len(m.agents) > 0 {
					if item, ok := m.list.SelectedItem().(AgentItem); ok {
						m.selectedAgent = &item.agent
						m.PushNavigation(item.agent.Name)
						m.SetViewMode("detail")
						m.actionButtonIndex = 0  // Start with first button selected
						m.detailsScrollOffset = 0  // Reset scroll position
						return m, m.loadAgentTools(item.agent.ID)  // Load assigned tools
					}
				}
				return m, nil
				
			case "n":
				// Create new agent - show create form
				m.PushNavigation("Create Agent")
				m.SetViewMode("create")
				// Reset form and focus first field
				m.resetCreateForm()
				m.nameInput.Focus()
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
					return m, m.runAgent(item.agent)
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
		
	case AgentsEnvironmentsLoadedMsg:
		m.environments = msg.Environments
		if len(m.environments) > 0 {
			m.selectedEnvID = m.environments[0].ID
		}
		// Load tools after environments are loaded
		return m, m.loadTools()
		
	case AgentsToolsLoadedMsg:
		m.availableTools = msg.Tools
		
	case AgentToolsLoadedMsg:
		if m.selectedAgent != nil && m.selectedAgent.ID == msg.AgentID {
			m.assignedTools = msg.Tools
		}
		
	case AgentCreatedMsg:
		// Add new agent to the list
		m.agents = append(m.agents, msg.Agent)
		m.updateListItems()
		// Return to list view
		m.SetViewMode("list")
		m.GoBack()
		m.resetCreateForm()
		return m, tea.Printf("Agent '%s' created successfully", msg.Agent.Name)
		
	case AgentUpdatedMsg:
		// Update agent in the list
		for i, agent := range m.agents {
			if agent.ID == msg.Agent.ID {
				m.agents[i] = msg.Agent
				break
			}
		}
		m.updateListItems()
		// Update selected agent and return to detail view
		m.selectedAgent = &msg.Agent
		m.SetViewMode("detail")
		m.GoBack()
		// Reload assigned tools since they may have changed
		return m, tea.Batch(
			tea.Printf("Agent '%s' updated successfully", msg.Agent.Name),
			m.loadAgentTools(msg.Agent.ID),
		)
	}
	
	// Update list component
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	
	// Update viewport component when in detail view
	if m.GetViewMode() == "detail" {
		m.detailsViewport, cmd = m.detailsViewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	
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
	case "edit":
		content = m.renderEditAgentForm()
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
func (m *AgentsModel) renderAgentDetails() string {
	if m.selectedAgent == nil {
		return styles.ErrorStyle.Render("No agent selected")
	}
	
	agent := m.selectedAgent
	
	// Simple single-column layout
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
	
	// System prompt
	prompt := m.renderFullPrompt(agent)
	sections = append(sections, prompt)
	
	// Assigned tools
	assignedTools := m.renderAssignedTools()
	sections = append(sections, assignedTools)
	
	// Actions
	actions := m.renderAgentActions()
	sections = append(sections, actions)
	
	// Help instructions
	helpText := styles.HelpStyle.Render("• ↑/↓ or j/k: scroll • ←/→ or h/l: navigate buttons • enter: execute • r: run • d: delete • esc: back")
	backText := styles.HelpStyle.Render("Press ESC to go back to list")
	sections = append(sections, "")
	sections = append(sections, helpText)
	sections = append(sections, backText)
	
	// Simple vertical layout - return directly (viewport causes display issues)
	fullContent := lipgloss.JoinVertical(lipgloss.Left, sections...)
	
	// Manual scrolling implementation
	lines := strings.Split(fullContent, "\n")
	
	// Get terminal height and calculate available space for content
	maxHeight := m.height - 4 // Account for borders and header
	if maxHeight < 10 {
		maxHeight = 10 // Minimum height
	}
	
	// Apply scroll offset
	startLine := m.detailsScrollOffset
	endLine := startLine + maxHeight
	
	if startLine < 0 {
		startLine = 0
		m.detailsScrollOffset = 0
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine >= len(lines) {
		startLine = len(lines) - maxHeight
		if startLine < 0 {
			startLine = 0
		}
		m.detailsScrollOffset = startLine
	}
	
	// Get visible lines
	visibleLines := lines[startLine:endLine]
	return strings.Join(visibleLines, "\n")
}

// Render agent basic information
func (m AgentsModel) renderAgentInfo(agent *models.Agent) string {
	// Compact info display like in the list view
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	
	// First line: Description
	descLine := agent.Description
	if descLine == "" {
		descLine = "No description"
	}
	
	// Second line: compact details with bullet separators
	createdAt := agent.CreatedAt.Format("Jan 2, 2006")
	detailsLine := lipgloss.JoinHorizontal(
		lipgloss.Left,
		mutedStyle.Render(fmt.Sprintf("ID: %d • ", agent.ID)),
		mutedStyle.Render(fmt.Sprintf("Env: %d • ", agent.EnvironmentID)),
		mutedStyle.Render(fmt.Sprintf("Max steps: %d • ", agent.MaxSteps)),
		mutedStyle.Render(fmt.Sprintf("Created: %s", createdAt)),
	)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		descLine,
		detailsLine,
		"", // Add some spacing
	)
}

// Render prompt preview
func (m AgentsModel) renderPromptPreview(agent *models.Agent) string {
	prompt := agent.Prompt
	
	// Truncate very long prompts for preview
	maxLength := 500
	if len(prompt) > maxLength {
		prompt = prompt[:maxLength] + "..."
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
		Padding(1).
		Render(content)
}

// Render full system prompt for right column in details view
func (m AgentsModel) renderFullPrompt(agent *models.Agent) string {
	prompt := agent.Prompt
	if prompt == "" {
		prompt = "No system prompt configured"
	}
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("System Prompt:"),
		mutedStyle.Render(prompt),
		"", // Add spacing after prompt
	)
	
	return content
}

// Render assigned tools section
func (m AgentsModel) renderAssignedTools() string {
	var toolsList []string
	
	toolsList = append(toolsList, styles.HeaderStyle.Render("Assigned Tools:"))
	toolsList = append(toolsList, "")
	
	if len(m.assignedTools) == 0 {
		toolsList = append(toolsList, styles.BaseStyle.Render("No tools assigned"))
	} else {
		for _, tool := range m.assignedTools {
			// Get environment name from tool's server info
			envInfo := fmt.Sprintf("(Server: %s)", tool.ServerName)
			
			toolLine := fmt.Sprintf("• %s - %s", tool.ToolName, tool.ToolDescription)
			if len(toolLine) > 50 {
				toolLine = toolLine[:50] + "..."
			}
			
			mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
			content := lipgloss.JoinVertical(
				lipgloss.Left,
				styles.BaseStyle.Render(toolLine),
				mutedStyle.Render("  "+envInfo),
			)
			toolsList = append(toolsList, content)
		}
	}
	
	content := lipgloss.JoinVertical(lipgloss.Left, toolsList...)
	
	return content
}

// Render agent action buttons
func (m AgentsModel) renderAgentActions() string {
	// Style buttons based on selection
	var runBtn, editBtn, deleteBtn string
	
	if m.actionButtonIndex == 0 {
		runBtn = styles.ButtonActiveStyle.Render("▶ Run Agent")
	} else {
		runBtn = styles.ButtonStyle.Render("Run Agent")
	}
	
	if m.actionButtonIndex == 1 {
		editBtn = styles.ButtonActiveStyle.Render("▶ Edit")
	} else {
		editBtn = styles.ButtonStyle.Render("Edit")
	}
	
	if m.actionButtonIndex == 2 {
		deleteBtn = styles.ButtonActiveStyle.Render("▶ Delete")
	} else {
		deleteBtn = styles.ErrorStyle.Render("Delete")
	}
	
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		runBtn, "  ",
		editBtn, "  ",
		deleteBtn,
	)
}

// Render create agent form with two-column layout
func (m AgentsModel) renderCreateAgentForm() string {
	// Header
	header := components.RenderSectionHeader("Create New Agent")
	
	// LEFT COLUMN - Basic fields and tools
	var leftColumn []string
	
	// Name field
	nameLabel := "Name:"
	if m.focusedField == AgentFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	nameSection := lipgloss.JoinVertical(lipgloss.Left, nameLabel, m.nameInput.View())
	leftColumn = append(leftColumn, nameSection)
	leftColumn = append(leftColumn, "")
	
	// Description field
	descLabel := "Description:"
	if m.focusedField == AgentFieldDesc {
		descLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Description:")
	}
	descSection := lipgloss.JoinVertical(lipgloss.Left, descLabel, m.descInput.View())
	leftColumn = append(leftColumn, descSection)
	leftColumn = append(leftColumn, "")
	
	// Environment selection
	envLabel := "Environment:"
	if m.focusedField == AgentFieldEnvironment {
		envLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Environment:")
	}
	envName := "No environments available"
	if len(m.environments) > 0 {
		for _, env := range m.environments {
			if env.ID == m.selectedEnvID {
				envName = env.Name
				break
			}
		}
	}
	envSection := lipgloss.JoinVertical(lipgloss.Left, envLabel, styles.BaseStyle.Render("▶ "+envName))
	leftColumn = append(leftColumn, envSection)
	leftColumn = append(leftColumn, "")
	
	// Schedule enabled checkbox
	scheduleEnabledLabel := "Schedule Enabled:"
	if m.focusedField == AgentFieldScheduleEnabled {
		scheduleEnabledLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Schedule Enabled:")
	}
	scheduleEnabledValue := "☐ No"
	if m.scheduleEnabled {
		scheduleEnabledValue = "☑ Yes"
	}
	scheduleEnabledSection := lipgloss.JoinVertical(lipgloss.Left, scheduleEnabledLabel, styles.BaseStyle.Render("▶ "+scheduleEnabledValue))
	leftColumn = append(leftColumn, scheduleEnabledSection)
	leftColumn = append(leftColumn, "")
	
	// Cron schedule input (only show if scheduling is enabled)
	if m.scheduleEnabled {
		scheduleLabel := "Cron Schedule:"
		if m.focusedField == AgentFieldSchedule {
			scheduleLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Cron Schedule:")
		}
		scheduleSection := lipgloss.JoinVertical(lipgloss.Left, scheduleLabel, m.scheduleInput.View())
		leftColumn = append(leftColumn, scheduleSection)
		leftColumn = append(leftColumn, "")
	}
	
	// Tools selection
	toolsLabel := "Available Tools:"
	if m.focusedField == AgentFieldTools {
		toolsLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Available Tools:")
	}
	toolsSection := m.renderToolsSelection(toolsLabel)
	leftColumn = append(leftColumn, toolsSection)
	
	// RIGHT COLUMN - System prompt (takes up majority of space)
	var rightColumn []string
	
	// System prompt
	promptLabel := "System Prompt:"
	if m.focusedField == AgentFieldPrompt {
		promptLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("System Prompt:")
	}
	promptSection := lipgloss.JoinVertical(lipgloss.Left, promptLabel, m.promptArea.View())
	rightColumn = append(rightColumn, promptSection)
	
	// Combine columns
	leftColumnContent := lipgloss.JoinVertical(lipgloss.Left, leftColumn...)
	rightColumnContent := lipgloss.JoinVertical(lipgloss.Left, rightColumn...)
	
	// Style columns with appropriate widths
	leftColumnStyled := lipgloss.NewStyle().
		Width(50).  // Left column: ~50 chars
		Render(leftColumnContent)
	
	rightColumnStyled := lipgloss.NewStyle().
		Width(85).  // Right column: ~85 chars (majority of space)
		Render(rightColumnContent)
	
	// Join columns horizontally
	columnsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumnStyled,
		"  ", // Small gap between columns
		rightColumnStyled,
	)
	
	// Help text
	helpText := styles.HelpStyle.Render("• tab: next field • ↑/↓: navigate • space: select tools • ctrl+s: save • esc: auto-save & exit")
	
	// Combine all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		columnsContent,
		"",
		helpText,
	)
}

// Render edit agent form with two-column layout (same as create but with different header)
func (m AgentsModel) renderEditAgentForm() string {
	// Header
	header := components.RenderSectionHeader("Edit Agent")
	
	// LEFT COLUMN - Basic fields and tools
	var leftColumn []string
	
	// Name field
	nameLabel := "Name:"
	if m.focusedField == AgentFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	nameSection := lipgloss.JoinVertical(lipgloss.Left, nameLabel, m.nameInput.View())
	leftColumn = append(leftColumn, nameSection)
	leftColumn = append(leftColumn, "")
	
	// Description field
	descLabel := "Description:"
	if m.focusedField == AgentFieldDesc {
		descLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Description:")
	}
	descSection := lipgloss.JoinVertical(lipgloss.Left, descLabel, m.descInput.View())
	leftColumn = append(leftColumn, descSection)
	leftColumn = append(leftColumn, "")
	
	// Environment selection
	envLabel := "Environment:"
	if m.focusedField == AgentFieldEnvironment {
		envLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Environment:")
	}
	envName := "No environments available"
	if len(m.environments) > 0 {
		for _, env := range m.environments {
			if env.ID == m.selectedEnvID {
				envName = env.Name
				break
			}
		}
	}
	envSection := lipgloss.JoinVertical(lipgloss.Left, envLabel, styles.BaseStyle.Render("▶ "+envName))
	leftColumn = append(leftColumn, envSection)
	leftColumn = append(leftColumn, "")
	
	// Schedule enabled checkbox
	scheduleEnabledLabel := "Schedule Enabled:"
	if m.focusedField == AgentFieldScheduleEnabled {
		scheduleEnabledLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Schedule Enabled:")
	}
	scheduleEnabledValue := "☐ No"
	if m.scheduleEnabled {
		scheduleEnabledValue = "☑ Yes"
	}
	scheduleEnabledSection := lipgloss.JoinVertical(lipgloss.Left, scheduleEnabledLabel, styles.BaseStyle.Render("▶ "+scheduleEnabledValue))
	leftColumn = append(leftColumn, scheduleEnabledSection)
	leftColumn = append(leftColumn, "")
	
	// Cron schedule input (only show if scheduling is enabled)
	if m.scheduleEnabled {
		scheduleLabel := "Cron Schedule:"
		if m.focusedField == AgentFieldSchedule {
			scheduleLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Cron Schedule:")
		}
		scheduleSection := lipgloss.JoinVertical(lipgloss.Left, scheduleLabel, m.scheduleInput.View())
		leftColumn = append(leftColumn, scheduleSection)
		leftColumn = append(leftColumn, "")
	}
	
	// Tools selection
	toolsLabel := "Available Tools:"
	if m.focusedField == AgentFieldTools {
		toolsLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Available Tools:")
	}
	toolsSection := m.renderToolsSelection(toolsLabel)
	leftColumn = append(leftColumn, toolsSection)
	
	// RIGHT COLUMN - System prompt (takes up majority of space)
	var rightColumn []string
	
	// System prompt
	promptLabel := "System Prompt:"
	if m.focusedField == AgentFieldPrompt {
		promptLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("System Prompt:")
	}
	promptSection := lipgloss.JoinVertical(lipgloss.Left, promptLabel, m.promptArea.View())
	rightColumn = append(rightColumn, promptSection)
	
	// Combine columns
	leftColumnContent := lipgloss.JoinVertical(lipgloss.Left, leftColumn...)
	rightColumnContent := lipgloss.JoinVertical(lipgloss.Left, rightColumn...)
	
	// Style columns with appropriate widths
	leftColumnStyled := lipgloss.NewStyle().
		Width(50).  // Left column: ~50 chars
		Render(leftColumnContent)
	
	rightColumnStyled := lipgloss.NewStyle().
		Width(85).  // Right column: ~85 chars (majority of space)
		Render(rightColumnContent)
	
	// Join columns horizontally
	columnsContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumnStyled,
		"  ", // Small gap between columns
		rightColumnStyled,
	)
	
	// Help text (different for edit)
	helpText := styles.HelpStyle.Render("• tab: next field • ↑/↓: navigate • space: select tools • ctrl+s: update • esc: auto-save & exit")
	
	// Combine all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		"",
		columnsContent,
		"",
		helpText,
	)
}

// Render tools selection section with scrolling, filtering, and environment info
func (m AgentsModel) renderToolsSelection(label string) string {
	var toolsList []string
	toolsList = append(toolsList, label)
	toolsList = append(toolsList, "")
	
	// Show filter input if in filtering mode
	if m.isFiltering {
		filterLabel := lipgloss.NewStyle().Foreground(styles.Primary).Render("Filter: /" + m.toolsFilter)
		toolsList = append(toolsList, filterLabel)
		toolsList = append(toolsList, "")
	}
	
	if len(m.availableTools) == 0 {
		toolsList = append(toolsList, styles.BaseStyle.Render("No tools available"))
	} else {
		// Filter tools based on search text
		filteredTools := m.getFilteredTools()
		
		if len(filteredTools) == 0 {
			toolsList = append(toolsList, styles.BaseStyle.Render("No tools match filter"))
		} else {
			// Calculate visible range with scrolling
			maxShow := 4  // Reduced to fit in left column properly
			start := m.toolsOffset
			end := start + maxShow
			if end > len(filteredTools) {
				end = len(filteredTools)
			}
			
			// Show scroll indicator at top if needed
			if start > 0 {
				toolsList = append(toolsList, styles.BaseStyle.Render("↑ ... more tools above"))
			}
			
			// Show visible tools
			for i := start; i < end; i++ {
				tool := filteredTools[i]
				
				// Check if tool is selected
				selected := false
				for _, selectedID := range m.selectedToolIDs {
					if selectedID == tool.ID {
						selected = true
						break
					}
				}
				
				prefix := "☐ "
				if selected {
					prefix = "☑ "
				}
				
				// Enhanced display with MCP server, environment, and config info
				serverInfo := fmt.Sprintf("server: %s", tool.ServerName)
				envInfo := fmt.Sprintf("env: %s", tool.EnvironmentName)
				configInfo := fmt.Sprintf("config: %s v%d", tool.ConfigName, tool.ConfigVersion)
				
				toolLine := fmt.Sprintf("%s%s", prefix, tool.Name)
				
				// Highlight current cursor position when in tools field
				// m.toolCursor is the absolute index in filteredTools, i is also absolute index
				isCursorHere := m.focusedField == AgentFieldTools && m.toolCursor == i
				
				if isCursorHere {
					// Full highlighting for selected item - apply style to entire lines with single arrow
					line1 := fmt.Sprintf("▶ %s - %s", toolLine, serverInfo)
					line2 := fmt.Sprintf("     %s | %s", envInfo, configInfo)
					toolsList = append(toolsList, styles.ListItemSelectedStyle.Width(48).Render(line1))
					toolsList = append(toolsList, styles.ListItemSelectedStyle.Width(48).Render(line2))
				} else {
					// Normal display with muted colors for metadata
					line1 := fmt.Sprintf("  %s - %s", toolLine, serverInfo)
					line2 := fmt.Sprintf("     %s | %s", styles.BaseStyle.Foreground(styles.TextMuted).Render(envInfo), styles.BaseStyle.Foreground(styles.TextMuted).Render(configInfo))
					toolsList = append(toolsList, styles.BaseStyle.Render(line1))
					toolsList = append(toolsList, styles.BaseStyle.Render(line2))
				}
			}
			
			// Show scroll indicator at bottom if needed
			if end < len(filteredTools) {
				remaining := len(filteredTools) - end
				toolsList = append(toolsList, styles.BaseStyle.Render(fmt.Sprintf("↓ ... %d more tools below", remaining)))
			}
		}
	}
	
	// Add filter help text
	if !m.isFiltering {
		filterHelp := styles.HelpStyle.Render("Press / to filter tools")
		toolsList = append(toolsList, "")
		toolsList = append(toolsList, filterHelp)
	} else {
		filterHelp := styles.HelpStyle.Render("Press ESC to clear filter")
		toolsList = append(toolsList, "")
		toolsList = append(toolsList, filterHelp)
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, toolsList...)
}

// Get filtered tools based on selected environment and current filter text
func (m AgentsModel) getFilteredTools() []models.MCPToolWithDetails {
	// First filter by selected environment
	var envFilteredTools []models.MCPToolWithDetails
	for _, tool := range m.availableTools {
		if tool.EnvironmentID == m.selectedEnvID {
			envFilteredTools = append(envFilteredTools, tool)
		}
	}
	
	// Then filter by search text if any
	if m.toolsFilter == "" {
		return envFilteredTools
	}
	
	var filtered []models.MCPToolWithDetails
	filterLower := strings.ToLower(m.toolsFilter)
	
	for _, tool := range envFilteredTools {
		// Search in tool name, description, environment name, and config name
		if strings.Contains(strings.ToLower(tool.Name), filterLower) ||
		   strings.Contains(strings.ToLower(tool.Description), filterLower) ||
		   strings.Contains(strings.ToLower(tool.EnvironmentName), filterLower) ||
		   strings.Contains(strings.ToLower(tool.ConfigName), filterLower) {
			filtered = append(filtered, tool)
		}
	}
	
	return filtered
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
		// Delete agent from database
		if err := m.repos.Agents.Delete(agentID); err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to delete agent: %w", err)}
		}
		return AgentDeletedMsg{AgentID: agentID}
	})
}

// IsMainView returns true if in main agents list view
func (m AgentsModel) IsMainView() bool {
	// Use the base implementation for reliable navigation
	return m.BaseTabModel.IsMainView()
}

// Load environments from database
func (m AgentsModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		envs, err := m.repos.Environments.List()
		if err != nil {
			// Return empty slice on error
			return EnvironmentsLoadedMsg{Environments: []models.Environment{}}
		}
		
		// Convert from []*models.Environment to []models.Environment
		var environments []models.Environment
		for _, env := range envs {
			environments = append(environments, *env)
		}
		
		return AgentsEnvironmentsLoadedMsg{Environments: environments}
	})
}

// Load available tools from database
func (m AgentsModel) loadTools() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Get all tools with details (includes environment and config names)
		toolsWithDetails, err := m.repos.MCPTools.GetAllWithDetails()
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to load tools: %w", err)}
		}
		
		// Convert from []*models.MCPToolWithDetails to []models.MCPToolWithDetails
		var mcpTools []models.MCPToolWithDetails
		for _, tool := range toolsWithDetails {
			mcpTools = append(mcpTools, *tool)
		}
		
		return AgentsToolsLoadedMsg{Tools: mcpTools}
	})
}

// Load tools assigned to a specific agent
func (m AgentsModel) loadAgentTools(agentID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		tools, err := m.repos.AgentTools.List(agentID)
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to load agent tools: %w", err)}
		}
		
		// Convert from []*models.AgentToolWithDetails to []models.AgentToolWithDetails
		var agentTools []models.AgentToolWithDetails
		for _, tool := range tools {
			agentTools = append(agentTools, *tool)
		}
		
		return AgentToolsLoadedMsg{AgentID: agentID, Tools: agentTools}
	})
}

// Reset create form to initial state
func (m *AgentsModel) resetCreateForm() {
	m.nameInput.SetValue("")
	m.descInput.SetValue("")
	m.promptArea.SetValue("")
	m.scheduleInput.SetValue("")
	m.scheduleEnabled = false
	m.selectedToolIDs = []int64{}
	m.focusedField = AgentFieldName
	m.toolCursor = 0
	if len(m.environments) > 0 {
		m.selectedEnvID = m.environments[0].ID
	}
}

// Populate edit form with current agent data
func (m *AgentsModel) populateEditForm() {
	if m.selectedAgent == nil {
		return
	}
	
	m.nameInput.SetValue(m.selectedAgent.Name)
	m.descInput.SetValue(m.selectedAgent.Description)
	m.promptArea.SetValue(m.selectedAgent.Prompt)
	
	// Set schedule fields
	if m.selectedAgent.CronSchedule != nil {
		m.scheduleInput.SetValue(*m.selectedAgent.CronSchedule)
	} else {
		m.scheduleInput.SetValue("")
	}
	m.scheduleEnabled = m.selectedAgent.ScheduleEnabled
	
	// Set environment ID, but default to first available if agent's environment doesn't exist
	m.selectedEnvID = m.selectedAgent.EnvironmentID
	validEnv := false
	for _, env := range m.environments {
		if env.ID == m.selectedAgent.EnvironmentID {
			validEnv = true
			break
		}
	}
	if !validEnv && len(m.environments) > 0 {
		// Agent's environment doesn't exist, default to first available environment
		m.selectedEnvID = m.environments[0].ID
	}
	
	m.focusedField = AgentFieldName
	m.toolCursor = 0
	
	// Populate selected tools from assigned tools
	m.selectedToolIDs = []int64{}
	for _, tool := range m.assignedTools {
		m.selectedToolIDs = append(m.selectedToolIDs, tool.ToolID)
	}
}

// Handle key events in create form
func (m *AgentsModel) handleCreateFormKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Handle filter mode when in tools section first
	if m.focusedField == AgentFieldTools && m.isFiltering {
		switch msg.String() {
		case "esc":
			// Exit filter mode
			m.isFiltering = false
			m.toolsFilter = ""
			m.toolCursor = 0 // Reset cursor when exiting filter
			return m, nil
		case "backspace":
			// Remove last character from filter
			if len(m.toolsFilter) > 0 {
				m.toolsFilter = m.toolsFilter[:len(m.toolsFilter)-1]
				m.toolCursor = 0 // Reset cursor when filter changes
			}
			return m, nil
		default:
			// Add character to filter (only printable characters)
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.toolsFilter += msg.String()
				m.toolCursor = 0 // Reset cursor when filter changes
				return m, nil
			}
		}
	}
	
	switch msg.String() {
	case "esc":
		// Auto-save if form has content, then go back to list
		nameValue := m.nameInput.Value()
		descValue := m.descInput.Value()
		
		if nameValue != "" || descValue != "" {
			// Save the agent before exiting
			cmd := m.createAgent()
			m.SetViewMode("list")
			m.GoBack()
			m.resetCreateForm()
			return m, cmd
		}
		
		// No content to save, just cancel and go back
		m.SetViewMode("list")
		m.GoBack()
		m.resetCreateForm()
		return m, nil
		
	case "tab":
		// Cycle through form fields
		m.cycleFocusedField()
		return m, nil
		
	case "ctrl+s":
		// Save agent
		return m, m.createAgent()
		
	case "up", "k":
		if m.focusedField == AgentFieldEnvironment {
			// Navigate environment selection
			for i, env := range m.environments {
				if env.ID == m.selectedEnvID && i > 0 {
					m.selectedEnvID = m.environments[i-1].ID
					break
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection up (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor > 0 {
				m.toolCursor--
				// Update scroll offset if cursor goes above visible area
				if m.toolCursor < m.toolsOffset {
					m.toolsOffset = m.toolCursor
				}
			}
			return m, nil
		}
		
	case "down", "j":
		if m.focusedField == AgentFieldEnvironment {
			// Navigate environment selection
			for i, env := range m.environments {
				if env.ID == m.selectedEnvID && i < len(m.environments)-1 {
					m.selectedEnvID = m.environments[i+1].ID
					break
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection down (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools)-1 {
				m.toolCursor++
				// Update scroll offset if cursor goes below visible area
				maxShow := 4  // Must match the display maxShow
				if m.toolCursor >= m.toolsOffset + maxShow {
					m.toolsOffset = m.toolCursor - maxShow + 1
				}
			}
			return m, nil
		}
		
	case " ", "enter":
		if m.focusedField == AgentFieldScheduleEnabled {
			// Toggle schedule enabled
			m.scheduleEnabled = !m.scheduleEnabled
			// Clear schedule input if disabling
			if !m.scheduleEnabled {
				m.scheduleInput.SetValue("")
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Toggle tool selection (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools) {
				toolID := filteredTools[m.toolCursor].ID
				
				// Check if tool is already selected
				found := false
				for i, selectedID := range m.selectedToolIDs {
					if selectedID == toolID {
						// Remove from selection
						m.selectedToolIDs = append(m.selectedToolIDs[:i], m.selectedToolIDs[i+1:]...)
						found = true
						break
					}
				}
				
				// If not found, add to selection
				if !found {
					m.selectedToolIDs = append(m.selectedToolIDs, toolID)
				}
			}
			return m, nil
		}
		
	case "/":
		if m.focusedField == AgentFieldTools && !m.isFiltering {
			// Enter filter mode
			m.isFiltering = true
			m.toolsFilter = ""
			return m, nil
		}
	}
	
	// Update focused input component
	var cmd tea.Cmd
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldDesc:
		m.descInput, cmd = m.descInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldPrompt:
		m.promptArea, cmd = m.promptArea.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldSchedule:
		m.scheduleInput, cmd = m.scheduleInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// Handle key events in detail view
func (m *AgentsModel) handleDetailViewKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		// Scroll up in details view (simple manual scrolling)
		if m.detailsScrollOffset > 0 {
			m.detailsScrollOffset--
		}
		return m, nil
		
	case "down", "j":
		// Scroll down in details view (simple manual scrolling)
		m.detailsScrollOffset++
		return m, nil
		
	case "pgup", "b":
		// Scroll up one page (half screen)
		maxHeight := m.height - 4
		if maxHeight < 10 {
			maxHeight = 10
		}
		pageSize := maxHeight / 2
		m.detailsScrollOffset -= pageSize
		if m.detailsScrollOffset < 0 {
			m.detailsScrollOffset = 0
		}
		return m, nil
		
	case "pgdown", "f":
		// Scroll down one page (half screen)
		maxHeight := m.height - 4
		if maxHeight < 10 {
			maxHeight = 10
		}
		pageSize := maxHeight / 2
		m.detailsScrollOffset += pageSize
		return m, nil
		
	case "g":
		// Go to top
		m.detailsScrollOffset = 0
		return m, nil
		
	case "G":
		// Go to bottom (will be clamped in render function)
		m.detailsScrollOffset = 9999
		return m, nil
		
	case "left", "h":
		// Navigate left through action buttons
		if m.actionButtonIndex > 0 {
			m.actionButtonIndex--
		}
		return m, nil
		
	case "right", "l":
		// Navigate right through action buttons
		if m.actionButtonIndex < 2 {  // 0=Run, 1=Edit, 2=Delete
			m.actionButtonIndex++
		}
		return m, nil
		
	case "enter", " ":
		// Execute selected action
		if m.selectedAgent == nil {
			return m, nil
		}
		
		switch m.actionButtonIndex {
		case 0: // Run Agent
			return m, m.runAgent(*m.selectedAgent)
		case 1: // Edit
			// Switch to edit mode - populate form with current agent data
			m.PushNavigation("Edit Agent")
			m.SetViewMode("edit")
			m.populateEditForm()
			m.nameInput.Focus()
			return m, nil
		case 2: // Delete
			return m, m.deleteAgent(m.selectedAgent.ID)
		}
		return m, nil
		
	case "r":
		// Quick run with 'r' key
		if m.selectedAgent != nil {
			return m, m.runAgent(*m.selectedAgent)
		}
		return m, nil
		
	case "d":
		// Quick delete with 'd' key
		if m.selectedAgent != nil {
			return m, m.deleteAgent(m.selectedAgent.ID)
		}
		return m, nil
		
	case "esc":
		// Go back to list view
		if m.CanGoBack() {
			m.GoBack()
			m.selectedAgent = nil
			m.actionButtonIndex = 0
		}
		return m, nil
	}
	
	return m, nil
}

// Handle key events in edit form (same as create form but saves updates)
func (m *AgentsModel) handleEditFormKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Handle filter mode when in tools section first
	if m.focusedField == AgentFieldTools && m.isFiltering {
		switch msg.String() {
		case "esc":
			// Exit filter mode
			m.isFiltering = false
			m.toolsFilter = ""
			m.toolCursor = 0 // Reset cursor when exiting filter
			return m, nil
		case "backspace":
			// Remove last character from filter
			if len(m.toolsFilter) > 0 {
				m.toolsFilter = m.toolsFilter[:len(m.toolsFilter)-1]
				m.toolCursor = 0 // Reset cursor when filter changes
			}
			return m, nil
		default:
			// Add character to filter (only printable characters)
			if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.toolsFilter += msg.String()
				m.toolCursor = 0 // Reset cursor when filter changes
				return m, nil
			}
		}
	}
	
	switch msg.String() {
	case "esc":
		// Auto-save changes if form has been modified, then go back to detail view
		nameValue := m.nameInput.Value()
		descValue := m.descInput.Value()
		
		if nameValue != "" || descValue != "" {
			// Save the updated agent before exiting 
			cmd := m.updateAgent()
			m.SetViewMode("detail")
			m.GoBack()
			return m, cmd
		}
		
		// No changes to save, just go back
		m.SetViewMode("detail")
		m.GoBack()
		return m, nil
		
	case "tab":
		// Cycle through form fields
		m.cycleFocusedField()
		return m, nil
		
	case "ctrl+s":
		// Save agent updates
		return m, m.updateAgent()
		
	case "up", "k":
		if m.focusedField == AgentFieldEnvironment {
			// Navigate environment selection
			for i, env := range m.environments {
				if env.ID == m.selectedEnvID && i > 0 {
					m.selectedEnvID = m.environments[i-1].ID
					break
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection up (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor > 0 {
				m.toolCursor--
				// Update scroll offset if cursor goes above visible area
				if m.toolCursor < m.toolsOffset {
					m.toolsOffset = m.toolCursor
				}
			}
			return m, nil
		}
		
	case "down", "j":
		if m.focusedField == AgentFieldEnvironment {
			// Navigate environment selection
			for i, env := range m.environments {
				if env.ID == m.selectedEnvID && i < len(m.environments)-1 {
					m.selectedEnvID = m.environments[i+1].ID
					break
				}
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Navigate tool selection down (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools)-1 {
				m.toolCursor++
				// Update scroll offset if cursor goes below visible area
				maxShow := 4  // Must match the display maxShow
				if m.toolCursor >= m.toolsOffset + maxShow {
					m.toolsOffset = m.toolCursor - maxShow + 1
				}
			}
			return m, nil
		}
		
	case " ", "enter":
		if m.focusedField == AgentFieldScheduleEnabled {
			// Toggle schedule enabled
			m.scheduleEnabled = !m.scheduleEnabled
			// Clear schedule input if disabling
			if !m.scheduleEnabled {
				m.scheduleInput.SetValue("")
			}
			return m, nil
		} else if m.focusedField == AgentFieldTools {
			// Toggle tool selection (work with filtered tools)
			filteredTools := m.getFilteredTools()
			if len(filteredTools) > 0 && m.toolCursor < len(filteredTools) {
				toolID := filteredTools[m.toolCursor].ID
				
				// Check if tool is already selected
				found := false
				for i, selectedID := range m.selectedToolIDs {
					if selectedID == toolID {
						// Remove from selection
						m.selectedToolIDs = append(m.selectedToolIDs[:i], m.selectedToolIDs[i+1:]...)
						found = true
						break
					}
				}
				
				// If not found, add to selection
				if !found {
					m.selectedToolIDs = append(m.selectedToolIDs, toolID)
				}
			}
			return m, nil
		}
		
	case "/":
		if m.focusedField == AgentFieldTools && !m.isFiltering {
			// Enter filter mode
			m.isFiltering = true
			m.toolsFilter = ""
			return m, nil
		}
	}
	
	// Update focused input component
	var cmd tea.Cmd
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput, cmd = m.nameInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldDesc:
		m.descInput, cmd = m.descInput.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldPrompt:
		m.promptArea, cmd = m.promptArea.Update(msg)
		cmds = append(cmds, cmd)
	case AgentFieldSchedule:
		m.scheduleInput, cmd = m.scheduleInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}

// Cycle through form fields
func (m *AgentsModel) cycleFocusedField() {
	// Blur current field
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput.Blur()
	case AgentFieldDesc:
		m.descInput.Blur()
	case AgentFieldPrompt:
		m.promptArea.Blur()
	case AgentFieldSchedule:
		m.scheduleInput.Blur()
	}
	
	// Move to next field (skip schedule field if scheduling is disabled)
	for {
		m.focusedField = (m.focusedField + 1) % 7
		// Skip schedule field if scheduling is disabled
		if m.focusedField == AgentFieldSchedule && !m.scheduleEnabled {
			continue
		}
		break
	}
	
	// Focus new field
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput.Focus()
	case AgentFieldDesc:
		m.descInput.Focus()
	case AgentFieldPrompt:
		m.promptArea.Focus()
	case AgentFieldSchedule:
		m.scheduleInput.Focus()
	}
}

// Create agent command
func (m AgentsModel) createAgent() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Validate inputs
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name is required")}
		}
		if len(name) > 100 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name too long (max 100 characters)")}
		}
		
		description := strings.TrimSpace(m.descInput.Value())
		if description == "" {
			description = "No description provided"
		}
		if len(description) > 500 {
			return AgentsErrorMsg{Err: fmt.Errorf("description too long (max 500 characters)")}
		}
		
		prompt := strings.TrimSpace(m.promptArea.Value())
		if prompt == "" {
			prompt = "You are a helpful AI assistant."
		}
		if len(prompt) > 5000 {
			return AgentsErrorMsg{Err: fmt.Errorf("system prompt too long (max 5000 characters)")}
		}
		
		// Validate environment exists
		validEnv := false
		for _, env := range m.environments {
			if env.ID == m.selectedEnvID {
				validEnv = true
				break
			}
		}
		if !validEnv {
			return AgentsErrorMsg{Err: fmt.Errorf("selected environment does not exist")}
		}
		
		// Validate and prepare schedule fields
		var cronSchedule *string
		if m.scheduleEnabled {
			schedule := strings.TrimSpace(m.scheduleInput.Value())
			if schedule != "" {
				// TODO: Add cron expression validation here if needed
				cronSchedule = &schedule
			}
		}
		
		// Create agent in database
		agent, err := m.repos.Agents.Create(name, description, prompt, 10, m.selectedEnvID, 1, cronSchedule, m.scheduleEnabled) // Default to user ID 1
		if err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to create agent: %w", err)}
		}
		
		// Associate selected tools with agent
		var failedTools []int64
		for _, toolID := range m.selectedToolIDs {
			if _, err := m.repos.AgentTools.Add(agent.ID, toolID); err != nil {
				failedTools = append(failedTools, toolID)
			}
		}
		
		// Show warning if some tools failed to associate
		if len(failedTools) > 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent created but %d tools failed to associate", len(failedTools))}
		}
		
		return AgentCreatedMsg{Agent: *agent}
	})
}

// Update agent command
func (m AgentsModel) updateAgent() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if m.selectedAgent == nil {
			return AgentsErrorMsg{Err: fmt.Errorf("no agent selected for update")}
		}
		
		// Validate inputs (same as create)
		name := strings.TrimSpace(m.nameInput.Value())
		if name == "" {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name is required")}
		}
		if len(name) > 100 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent name too long (max 100 characters)")}
		}
		
		description := strings.TrimSpace(m.descInput.Value())
		if description == "" {
			description = "No description provided"
		}
		if len(description) > 500 {
			return AgentsErrorMsg{Err: fmt.Errorf("description too long (max 500 characters)")}
		}
		
		prompt := strings.TrimSpace(m.promptArea.Value())
		if prompt == "" {
			prompt = "You are a helpful AI assistant."
		}
		if len(prompt) > 5000 {
			return AgentsErrorMsg{Err: fmt.Errorf("system prompt too long (max 5000 characters)")}
		}
		
		// Validate environment exists
		validEnv := false
		for _, env := range m.environments {
			if env.ID == m.selectedEnvID {
				validEnv = true
				break
			}
		}
		if !validEnv {
			return AgentsErrorMsg{Err: fmt.Errorf("selected environment does not exist")}
		}
		
		// Validate and prepare schedule fields
		var cronSchedule *string
		if m.scheduleEnabled {
			schedule := strings.TrimSpace(m.scheduleInput.Value())
			if schedule != "" {
				// TODO: Add cron expression validation here if needed
				cronSchedule = &schedule
			}
		}
		
		// Update agent in database
		if err := m.repos.Agents.Update(m.selectedAgent.ID, name, description, prompt, m.selectedAgent.MaxSteps, cronSchedule, m.scheduleEnabled); err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to update agent: %w", err)}
		}
		
		// Create updated agent model for return
		updatedAgent := &models.Agent{
			ID:              m.selectedAgent.ID,
			Name:            name,
			Description:     description,
			Prompt:          prompt,
			MaxSteps:        m.selectedAgent.MaxSteps,
			EnvironmentID:   m.selectedEnvID,
			CreatedBy:       m.selectedAgent.CreatedBy,
			CronSchedule:    cronSchedule,
			IsScheduled:     cronSchedule != nil && *cronSchedule != "" && m.scheduleEnabled,
			ScheduleEnabled: m.scheduleEnabled,
			CreatedAt:       m.selectedAgent.CreatedAt,
		}
		
		// Clear existing tool associations
		if err := m.repos.AgentTools.Clear(m.selectedAgent.ID); err != nil {
			return AgentsErrorMsg{Err: fmt.Errorf("failed to clear agent tools: %w", err)}
		}
		
		// Associate selected tools with agent
		var failedTools []int64
		for _, toolID := range m.selectedToolIDs {
			if _, err := m.repos.AgentTools.Add(m.selectedAgent.ID, toolID); err != nil {
				failedTools = append(failedTools, toolID)
			}
		}
		
		// Show warning if some tools failed to associate
		if len(failedTools) > 0 {
			return AgentsErrorMsg{Err: fmt.Errorf("agent updated but %d tools failed to associate", len(failedTools))}
		}
		
		return AgentUpdatedMsg{Agent: *updatedAgent}
	})
}

// runAgent queues an agent for execution using the execution queue service
func (m *AgentsModel) runAgent(agent models.Agent) tea.Cmd {
	if m.executionQueue == nil {
		return tea.Printf("❌ Execution queue service not available")
	}
	
	// Use a default task prompt for manual agent execution
	task := "Execute agent manually from TUI"
	if agent.Prompt != "" {
		task = agent.Prompt
	}
	
	// Create metadata to indicate this was a manual execution
	metadata := map[string]interface{}{
		"source":       "manual_tui",
		"triggered_at": time.Now(),
	}
	
	// For manual executions via SSH console, get the console user ID
	// TODO: Get actual user ID from session when authentication is implemented
	consoleUser, err := m.repos.Users.GetByUsername("console")
	if err != nil {
		// Fallback to system user if console user not found
		consoleUser, err = m.repos.Users.GetByUsername("system")
		if err != nil {
			return tea.Printf("❌ Could not find console or system user for execution tracking")
		}
	}
	
	// Queue the execution
	if err := m.executionQueue.QueueExecution(agent.ID, consoleUser.ID, task, metadata); err != nil {
		return tea.Printf("❌ Failed to queue agent execution: %v", err)
	}
	
	return tea.Printf("🚀 Agent '%s' queued for execution - check Runs tab for progress", agent.Name)
}