package tabs

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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
	nameInput         textinput.Model
	descInput         textinput.Model
	promptArea        textarea.Model
	scheduleInput     textinput.Model
	scheduleEnabled   bool
	
	// Data access
	repos          *repositories.Repositories
	
	// State
	agents        []models.Agent
	selectedAgent *models.Agent
	
	// Create form state
	environments    []models.Environment
	availableTools  []models.MCPToolWithDetails  // Changed to include environment/config info
	selectedEnvIDs  []int64  // Changed to support multiple environments
	selectedToolIDs []int64
	focusedField    AgentFormField
	toolCursor      int    // Track which tool is currently highlighted
	toolsOffset     int    // Track scroll position in tools list
	toolsFilter     string // Filter text for tools
	isFiltering     bool   // Whether we're in filter mode
	envCursor       int    // Track which environment is currently highlighted in multi-select
	envOffset       int    // Track scroll position in environments list
	
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
	AgentFieldEnvironments  // Changed to plural for multi-selection
	AgentFieldScheduleEnabled
	AgentFieldPrompt
	AgentFieldTools
	AgentFieldSchedule
)

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
func NewAgentsModel(database db.Database) *AgentsModel {
	repos := repositories.New(database)
	// Create list - styles will be set dynamically in WindowSizeMsg handler
	delegate := list.NewDefaultDelegate()
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
		// Update viewport size for details view (also use content area)
		m.detailsViewport.Width = m.width - 4
		m.detailsViewport.Height = m.height - 4
		
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
				// Refresh available tools to ensure we have the latest data
				return m, m.loadTools()
				
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
				
			// Add vim-style navigation keys - let list handle them
			case "j":
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(tea.KeyMsg{Type: tea.KeyDown})
				return m, cmd
				
			case "k":
				var cmd tea.Cmd
				m.list, cmd = m.list.Update(tea.KeyMsg{Type: tea.KeyUp})
				return m, cmd
			}
			
			// List updates handled at end of Update method to avoid duplication
		}
		
	case AgentsLoadedMsg:
		m.agents = msg.Agents
		m.updateListItems()
		m.SetLoading(false)
		
		// Check if a specific agent was pre-selected (e.g., from dashboard navigation)
		if selectedID := m.GetSelectedID(); selectedID != "" {
			for _, agent := range m.agents {
				if fmt.Sprintf("%d", agent.ID) == selectedID {
					m.selectedAgent = &agent
					m.PushNavigation(agent.Name)
					m.SetViewMode("detail")
					m.actionButtonIndex = 0  // Start with first button selected
					m.SetSelectedID("") // Clear the selected ID after using it
					break
				}
			}
		}
		
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
			// Select first environment by default for new agents
		m.selectedEnvIDs = []int64{m.environments[0].ID}
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

// RefreshData reloads agents and tools from database
func (m AgentsModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return tea.Batch(
		m.loadAgents(),
		m.loadTools(),
	)
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

// IsMainView returns true if in main agents list view
func (m AgentsModel) IsMainView() bool {
	// Use the base implementation for reliable navigation
	return m.BaseTabModel.IsMainView()
}