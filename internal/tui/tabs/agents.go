package tabs

import (
	"fmt"
	"strings"
	"time"
	
	"github.com/charmbracelet/bubbles/key"
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

// AgentsModel represents the agents management tab
type AgentsModel struct {
	BaseTabModel
	
	// UI components
	list         list.Model
	
	// Create form components
	nameInput    textinput.Model
	descInput    textinput.Model
	promptArea   textarea.Model
	
	// Data access
	repos        *repositories.Repositories
	
	// State
	agents        []models.Agent
	selectedAgent *models.Agent
	
	// Create form state
	environments    []models.Environment
	availableTools  []models.MCPTool
	selectedEnvID   int64
	selectedToolIDs []int64
	focusedField    AgentFormField
	toolCursor      int  // Track which tool is currently highlighted
}

type AgentFormField int

const (
	AgentFieldName AgentFormField = iota
	AgentFieldDesc
	AgentFieldEnvironment
	AgentFieldPrompt
	AgentFieldTools
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

type AgentsEnvironmentsLoadedMsg struct {
	Environments []models.Environment
}

type AgentsToolsLoadedMsg struct {
	Tools []models.MCPTool
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
	
	// Create form inputs
	nameInput := textinput.New()
	nameInput.Placeholder = "Agent name"
	nameInput.Width = 40
	
	descInput := textinput.New()
	descInput.Placeholder = "Agent description"
	descInput.Width = 60
	
	promptArea := textarea.New()
	promptArea.Placeholder = "Enter system prompt for the agent..."
	promptArea.SetWidth(60)
	promptArea.SetHeight(4)
	
	return &AgentsModel{
		BaseTabModel: NewBaseTabModel(database, "Agents"),
		list:         l,
		repos:        repos,
		nameInput:    nameInput,
		descInput:    descInput,
		promptArea:   promptArea,
		focusedField: AgentFieldName,
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
		if m.GetViewMode() == "create" {
			return m.handleCreateFormKeys(msg)
		} else if m.GetViewMode() == "list" {
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
		
	case AgentsEnvironmentsLoadedMsg:
		m.environments = msg.Environments
		if len(m.environments) > 0 {
			m.selectedEnvID = m.environments[0].ID
		}
		// Load tools after environments are loaded
		return m, m.loadTools()
		
	case AgentsToolsLoadedMsg:
		m.availableTools = msg.Tools
		
	case AgentCreatedMsg:
		// Add new agent to the list
		m.agents = append(m.agents, msg.Agent)
		m.updateListItems()
		// Return to list view
		m.SetViewMode("list")
		m.GoBack()
		m.resetCreateForm()
		return m, tea.Printf("Agent '%s' created successfully", msg.Agent.Name)
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
	
	// Name field
	nameLabel := "Name:"
	if m.focusedField == AgentFieldName {
		nameLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Name:")
	}
	nameSection := lipgloss.JoinVertical(lipgloss.Left, nameLabel, m.nameInput.View())
	sections = append(sections, nameSection)
	sections = append(sections, "")
	
	// Description field
	descLabel := "Description:"
	if m.focusedField == AgentFieldDesc {
		descLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Description:")
	}
	descSection := lipgloss.JoinVertical(lipgloss.Left, descLabel, m.descInput.View())
	sections = append(sections, descSection)
	sections = append(sections, "")
	
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
	sections = append(sections, envSection)
	sections = append(sections, "")
	
	// System prompt
	promptLabel := "System Prompt:"
	if m.focusedField == AgentFieldPrompt {
		promptLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("System Prompt:")
	}
	promptSection := lipgloss.JoinVertical(lipgloss.Left, promptLabel, m.promptArea.View())
	sections = append(sections, promptSection)
	sections = append(sections, "")
	
	// Tools selection
	toolsLabel := "Available Tools:"
	if m.focusedField == AgentFieldTools {
		toolsLabel = lipgloss.NewStyle().Foreground(styles.Primary).Render("Available Tools:")
	}
	toolsSection := m.renderToolsSelection(toolsLabel)
	sections = append(sections, toolsSection)
	sections = append(sections, "")
	
	// Help text
	helpText := styles.HelpStyle.Render("• tab: next field • ↑/↓: navigate • space: select tools • ctrl+s: save • esc: cancel")
	sections = append(sections, helpText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render tools selection section
func (m AgentsModel) renderToolsSelection(label string) string {
	var toolsList []string
	toolsList = append(toolsList, label)
	toolsList = append(toolsList, "")
	
	if len(m.availableTools) == 0 {
		toolsList = append(toolsList, styles.BaseStyle.Render("No tools available"))
	} else {
		// Show first few tools with selection status
		maxShow := 5
		for i, tool := range m.availableTools {
			if i >= maxShow {
				remaining := len(m.availableTools) - maxShow
				toolsList = append(toolsList, styles.BaseStyle.Render(fmt.Sprintf("... and %d more tools", remaining)))
				break
			}
			
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
			
			toolLine := fmt.Sprintf("%s%s - %s", prefix, tool.Name, tool.Description)
			if len(toolLine) > 50 {
				toolLine = toolLine[:50] + "..."
			}
			
			// Highlight current cursor position when in tools field
			if m.focusedField == AgentFieldTools && i == m.toolCursor {
				toolsList = append(toolsList, styles.ListItemSelectedStyle.Render("▶ "+toolLine))
			} else {
				toolsList = append(toolsList, styles.BaseStyle.Render("  "+toolLine))
			}
		}
	}
	
	return lipgloss.JoinVertical(lipgloss.Left, toolsList...)
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
		var allTools []*models.MCPTool
		
		// Get tools from all environments
		for _, env := range m.environments {
			tools, err := m.repos.MCPTools.GetByEnvironmentID(env.ID)
			if err != nil {
				continue // Skip this environment on error
			}
			allTools = append(allTools, tools...)
		}
		
		// Convert from []*models.MCPTool to []models.MCPTool
		var mcpTools []models.MCPTool
		for _, tool := range allTools {
			mcpTools = append(mcpTools, *tool)
		}
		
		return AgentsToolsLoadedMsg{Tools: mcpTools}
	})
}

// Reset create form to initial state
func (m *AgentsModel) resetCreateForm() {
	m.nameInput.SetValue("")
	m.descInput.SetValue("")
	m.promptArea.SetValue("")
	m.selectedToolIDs = []int64{}
	m.focusedField = AgentFieldName
	m.toolCursor = 0
	if len(m.environments) > 0 {
		m.selectedEnvID = m.environments[0].ID
	}
}

// Handle key events in create form
func (m *AgentsModel) handleCreateFormKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg.String() {
	case "esc":
		// Cancel creation and go back to list
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
			// Navigate tool selection up
			if len(m.availableTools) > 0 && m.toolCursor > 0 {
				m.toolCursor--
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
			// Navigate tool selection down
			if len(m.availableTools) > 0 && m.toolCursor < len(m.availableTools)-1 {
				m.toolCursor++
			}
			return m, nil
		}
		
	case " ":
		if m.focusedField == AgentFieldTools {
			// Toggle tool selection
			if len(m.availableTools) > 0 && m.toolCursor < len(m.availableTools) {
				toolID := m.availableTools[m.toolCursor].ID
				
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
	}
	
	// Move to next field
	m.focusedField = (m.focusedField + 1) % 5
	
	// Focus new field
	switch m.focusedField {
	case AgentFieldName:
		m.nameInput.Focus()
	case AgentFieldDesc:
		m.descInput.Focus()
	case AgentFieldPrompt:
		m.promptArea.Focus()
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
		
		// Create agent in database
		agent, err := m.repos.Agents.Create(name, description, prompt, 10, m.selectedEnvID, 1) // Default to user ID 1
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