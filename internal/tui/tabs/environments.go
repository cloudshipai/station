package tabs

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// EnvironmentsModel represents the environments management tab
type EnvironmentsModel struct {
	BaseTabModel

	// UI components
	list          list.Model
	nameInput     textinput.Model
	descInput     textinput.Model

	// Data access
	repos         *repositories.Repositories

	// State
	environments  []models.Environment
	selectedID    int64
	showDetails   bool
	showCreate    bool
	editMode      EnvironmentEditMode
}

type EnvironmentEditMode int

const (
	EnvModeList EnvironmentEditMode = iota
	EnvModeDetails
	EnvModeCreate
	EnvModeEdit
)

// EnvironmentStats holds statistics for an environment
type EnvironmentStats struct {
	ActiveAgents int
	TotalRuns    int
	LastUsed     string
	Status       string
}

// EnvironmentItem implements list.Item interface for bubbles list component
type EnvironmentItem struct {
	env models.Environment
}

// Required by list.Item interface
func (i EnvironmentItem) FilterValue() string {
	return i.env.Name
}

// Required by list.DefaultItem interface
func (i EnvironmentItem) Title() string {
	status := "●"
	statusStyle := styles.SuccessStyle

	// Show status based on whether environment has active agents
	// For now, show all as active
	
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusStyle.Render(status+" "),
		styles.BaseStyle.Render(i.env.Name),
	)
}

func (i EnvironmentItem) Description() string {
	createdAt := i.env.CreatedAt.Format("Jan 2, 2006")
	updatedAt := i.env.UpdatedAt.Format("Jan 2, 2006")
	
	desc := ""
	if i.env.Description != nil {
		desc = *i.env.Description
		if len(desc) > 40 {
			desc = desc[:40] + "..."
		}
	}

	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		mutedStyle.Render(desc+" • "),
		mutedStyle.Render("Created: "+createdAt+" • "),
		mutedStyle.Render("Updated: "+updatedAt),
	)
}

// Custom key bindings for environments list
type environmentsKeyMap struct {
	list.KeyMap
	showDetails   key.Binding
	createEnv     key.Binding
	deleteEnv     key.Binding
	editEnv       key.Binding
}

func newEnvironmentsKeyMap() environmentsKeyMap {
	listKeys := list.DefaultKeyMap()

	return environmentsKeyMap{
		KeyMap: listKeys,
		showDetails: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "view details"),
		),
		createEnv: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new environment"),
		),
		deleteEnv: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		editEnv: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
	}
}

// Messages for async operations
type EnvironmentsLoadedMsg struct {
	Environments []models.Environment
}

type EnvironmentsErrorMsg struct {
	Err error
}

type EnvironmentCreatedMsg struct {
	Environment models.Environment
}

type EnvironmentUpdatedMsg struct {
	EnvironmentID int64
}

type EnvironmentDeletedMsg struct {
	EnvironmentID int64
}

// NewEnvironmentsModel creates a new environments model
func NewEnvironmentsModel(database db.Database) *EnvironmentsModel {
	repos := repositories.New(database)
	
	// Create list with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = styles.ListItemSelectedStyle
	delegate.Styles.SelectedDesc = styles.ListItemSelectedStyle
	delegate.Styles.NormalTitle = styles.ListItemStyle
	delegate.Styles.NormalDesc = styles.ListItemStyle

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Station Environments"
	l.Styles.Title = styles.HeaderStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	l.Styles.HelpStyle = styles.HelpStyle

	// Set custom key bindings
	keyMap := newEnvironmentsKeyMap()
	l.KeyMap = keyMap.KeyMap

	// Create input fields for creation/editing
	nameInput := textinput.New()
	nameInput.Placeholder = "Environment name"
	nameInput.Width = 40

	descInput := textinput.New()
	descInput.Placeholder = "Environment description"
	descInput.Width = 60

	return &EnvironmentsModel{
		BaseTabModel: NewBaseTabModel(database, "Environments"),
		list:         l,
		nameInput:    nameInput,
		descInput:    descInput,
		repos:        repos,
		editMode:     EnvModeList,
	}
}

// Init initializes the environments tab
func (m EnvironmentsModel) Init() tea.Cmd {
	return m.loadEnvironments()
}

// Update handles messages
func (m *EnvironmentsModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.list.SetSize(msg.Width-4, msg.Height-8) // Account for borders and header

	case tea.KeyMsg:
		// Clear error message on any key press when there's an error
		if m.GetError() != "" {
			m.SetError("")
			// Still process the key press normally after clearing error
		}
		
		// Handle navigation between modes
		log.Printf("DEBUG: Environment received key '%s', current editMode: %d", msg.String(), m.editMode)
		switch m.editMode {
		case EnvModeList:
			return m.handleListKeys(msg)
		case EnvModeDetails:
			return m.handleDetailsKeys(msg)
		case EnvModeCreate, EnvModeEdit:
			return m.handleCreateKeys(msg)
		}

	case EnvironmentsLoadedMsg:
		m.environments = msg.Environments
		m.updateListItems()
		m.SetLoading(false)

	case EnvironmentsErrorMsg:
		m.SetError(msg.Err.Error())
		m.SetLoading(false)

	case EnvironmentCreatedMsg:
		// Add new environment to list and reload from database to get fresh data
		log.Printf("DEBUG: Environment created, reloading list")
		m.editMode = EnvModeList // Return to list mode after creation
		return m, m.loadEnvironments()

	case EnvironmentUpdatedMsg:
		// Environment updated, reload from database to get fresh data
		log.Printf("DEBUG: Environment updated, reloading list")
		m.editMode = EnvModeList // Return to list mode after update
		return m, m.loadEnvironments()

	case EnvironmentDeletedMsg:
		// Remove deleted environment from list
		for i, env := range m.environments {
			if env.ID == msg.EnvironmentID {
				m.environments = append(m.environments[:i], m.environments[i+1:]...)
				break
			}
		}
		m.updateListItems()
		m.editMode = EnvModeList // Return to list mode after deletion
		m.showDetails = false
	}

	// Update list component
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Input updates are handled in handleCreateKeys() to avoid double processing

	return m, tea.Batch(cmds...)
}

// Handle key presses in list mode
func (m *EnvironmentsModel) handleListKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch {
	case key.Matches(msg, m.getKeyMap().showDetails):
		if len(m.environments) > 0 {
			m.editMode = EnvModeDetails
			if item, ok := m.list.SelectedItem().(EnvironmentItem); ok {
				m.selectedID = item.env.ID
			}
		}
		return m, nil

	case key.Matches(msg, m.getKeyMap().createEnv):
		log.Printf("DEBUG: Entering EnvModeCreate, setting editMode to %d", int(EnvModeCreate))
		m.editMode = EnvModeCreate
		m.nameInput.Focus()
		return m, nil

	case key.Matches(msg, m.getKeyMap().deleteEnv):
		if len(m.environments) > 0 {
			if item, ok := m.list.SelectedItem().(EnvironmentItem); ok {
				return m, m.deleteEnvironment(item.env.ID)
			}
		}
		return m, nil

	case key.Matches(msg, m.getKeyMap().editEnv):
		if len(m.environments) > 0 {
			if item, ok := m.list.SelectedItem().(EnvironmentItem); ok {
				m.editMode = EnvModeEdit
				m.selectedID = item.env.ID
				// Pre-populate fields with current values
				m.nameInput.SetValue(item.env.Name)
				if item.env.Description != nil {
					m.descInput.SetValue(*item.env.Description)
				} else {
					m.descInput.SetValue("")
				}
				m.nameInput.Focus()
			}
		}
		return m, nil
	default:
		// Let the list component handle unhandled keys (like j/k navigation)
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}

// Handle key presses in details mode
func (m *EnvironmentsModel) handleDetailsKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.editMode = EnvModeList
		return m, nil
	case "d":
		// Delete the currently selected environment
		if m.selectedID > 0 {
			return m, m.deleteEnvironment(m.selectedID)
		}
		return m, nil
	case "e":
		// Edit the currently selected environment
		if m.selectedID > 0 {
			// Find the environment and switch to edit mode
			for _, env := range m.environments {
				if env.ID == m.selectedID {
					m.editMode = EnvModeEdit
					m.nameInput.SetValue(env.Name)
					if env.Description != nil {
						m.descInput.SetValue(*env.Description)
					} else {
						m.descInput.SetValue("")
					}
					m.nameInput.Focus()
					break
				}
			}
		}
		return m, nil
	}
	return m, nil
}

// Handle key presses in create mode
func (m *EnvironmentsModel) handleCreateKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	// Debug logging to see what keys are being received
	log.Printf("DEBUG: Environment handleCreateKeys received key: '%s', editMode: %d", msg.String(), m.editMode)
	
	switch msg.String() {
	case "esc":
		// Auto-save if there's content in the form fields
		nameValue := m.nameInput.Value()
		descValue := m.descInput.Value()
		
		log.Printf("DEBUG: ESC pressed - nameValue: '%s', descValue: '%s'", nameValue, descValue)
		
		if nameValue != "" || descValue != "" {
			log.Printf("DEBUG: Content found, calling saveEnvironment()")
			// Save the environment before exiting
			cmd := m.saveEnvironment()
			m.editMode = EnvModeList
			m.nameInput.Blur()
			m.descInput.Blur()
			m.nameInput.SetValue("")
			m.descInput.SetValue("")
			log.Printf("DEBUG: Returning from ESC with save command")
			return m, cmd
		}
		
		log.Printf("DEBUG: No content to save, just exiting")
		// No content to save, just exit
		m.editMode = EnvModeList
		m.nameInput.Blur()
		m.descInput.Blur()
		m.nameInput.SetValue("")
		m.descInput.SetValue("")
		log.Printf("DEBUG: Returning from ESC without save")
		return m, nil

	case "tab":
		if m.nameInput.Focused() {
			m.nameInput.Blur()
			m.descInput.Focus()
		} else {
			m.descInput.Blur()
			m.nameInput.Focus()
		}
		return m, nil

	case "ctrl+s":
		// Save environment
		return m, m.saveEnvironment()
	}
	
	// Always update input components when in create/edit mode
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	cmds = append(cmds, cmd)
	
	m.descInput, cmd = m.descInput.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

// View renders the environments tab
func (m EnvironmentsModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}

	switch m.editMode {
	case EnvModeDetails:
		return m.renderEnvironmentDetails()
	case EnvModeCreate, EnvModeEdit:
		return m.renderEnvironmentForm()
	default:
		return m.renderEnvironmentsList()
	}
}

// RefreshData reloads environments from database
func (m EnvironmentsModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadEnvironments()
}

// Get key map for custom bindings
func (m EnvironmentsModel) getKeyMap() environmentsKeyMap {
	return newEnvironmentsKeyMap()
}

// Load environments from database
func (m EnvironmentsModel) loadEnvironments() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Loading environments from database...")
		
		// Get environment repository from database
		environments, err := m.repos.Environments.List()
		if err != nil {
			log.Printf("ERROR: Failed to load environments: %v", err)
			return EnvironmentsErrorMsg{Err: err}
		}
		
		log.Printf("DEBUG: Loaded %d environments from database", len(environments))
		
		// Convert from []*models.Environment to []models.Environment
		envSlice := make([]models.Environment, len(environments))
		for i, env := range environments {
			envSlice[i] = *env
		}

		return EnvironmentsLoadedMsg{Environments: envSlice}
	})
}

// Update list items from environments data
func (m *EnvironmentsModel) updateListItems() {
	items := make([]list.Item, len(m.environments))
	for i, env := range m.environments {
		items[i] = EnvironmentItem{env: env}
	}
	m.list.SetItems(items)
}

// Render environments list view
func (m EnvironmentsModel) renderEnvironmentsList() string {
	var sections []string

	// Header with stats
	header := components.RenderSectionHeader(fmt.Sprintf("Environments (%d total)", len(m.environments)))
	sections = append(sections, header)

	// Show error message if there is one
	if m.GetError() != "" {
		errorMsg := styles.ErrorStyle.Render("⚠ " + m.GetError())
		sections = append(sections, errorMsg)
		sections = append(sections, "")
	}

	// List component
	listView := m.list.View()
	sections = append(sections, listView)

	// Help text
	helpText := styles.HelpStyle.Render("• enter: view details • n: new environment • e: edit • d: delete")
	if m.GetError() != "" {
		helpText = styles.HelpStyle.Render("• enter: view details • n: new environment • e: edit • d: delete • any key: dismiss error")
	}
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render environment details view
func (m EnvironmentsModel) renderEnvironmentDetails() string {
	var env *models.Environment
	for _, e := range m.environments {
		if e.ID == m.selectedID {
			env = &e
			break
		}
	}

	if env == nil {
		return styles.ErrorStyle.Render("Environment not found")
	}

	var sections []string

	// Header
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Environment Details: "),
		primaryStyle.Render(env.Name),
	)
	sections = append(sections, header)
	sections = append(sections, "")

	// Basic info
	info := m.renderEnvironmentInfo(env)
	sections = append(sections, info)

	// Environment status
	status := m.renderEnvironmentStatus(env)
	sections = append(sections, status)

	// Available actions
	actionsText := styles.HelpStyle.Render("• enter/esc: back to list • e: edit • d: delete environment")
	sections = append(sections, "")
	sections = append(sections, actionsText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render environment basic information
func (m EnvironmentsModel) renderEnvironmentInfo(env *models.Environment) string {
	desc := "No description"
	if env.Description != nil {
		desc = *env.Description
	}
	
	fields := []string{
		fmt.Sprintf("ID: %d", env.ID),
		fmt.Sprintf("Description: %s", desc),
		fmt.Sprintf("Created: %s", env.CreatedAt.Format("Jan 2, 2006 15:04")),
		fmt.Sprintf("Updated: %s", env.UpdatedAt.Format("Jan 2, 2006 15:04")),
	}

	content := strings.Join(fields, "\n")

	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(content)
}

// Render environment status/usage info
func (m EnvironmentsModel) renderEnvironmentStatus(env *models.Environment) string {
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	
	// Get actual usage statistics from database
	stats := m.getEnvironmentStats(env.ID)
	statusInfo := []string{
		fmt.Sprintf("Active agents: %d", stats.ActiveAgents),
		fmt.Sprintf("Total runs: %d", stats.TotalRuns),
		fmt.Sprintf("Last used: %s", stats.LastUsed),
		fmt.Sprintf("Status: %s", stats.Status),
	}
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Environment Status:"),
		"",
		mutedStyle.Render(strings.Join(statusInfo, "\n")),
	)

	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Height(8).
		Padding(1).
		Render(content)
}

// Render environment creation/edit form
func (m EnvironmentsModel) renderEnvironmentForm() string {
	var sections []string

	// Header
	title := "Create New Environment"
	if m.editMode == EnvModeEdit {
		title = "Edit Environment"
	}
	header := components.RenderSectionHeader(title)
	sections = append(sections, header)

	// Name input
	nameSection := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Name:"),
		m.nameInput.View(),
	)
	sections = append(sections, nameSection)
	sections = append(sections, "")

	// Description input
	descSection := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Description:"),
		m.descInput.View(),
	)
	sections = append(sections, descSection)

	// Help text
	helpText := styles.HelpStyle.Render("• tab: switch fields • ctrl+s: save • esc: auto-save & exit")
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Save environment command
func (m EnvironmentsModel) saveEnvironment() tea.Cmd {
	log.Printf("DEBUG: saveEnvironment() called")
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: saveEnvironment() tea.Cmd executing")
		
		nameValue := m.nameInput.Value()
		descValue := m.descInput.Value()
		
		log.Printf("DEBUG: Saving environment - name: '%s', desc: '%s'", nameValue, descValue)
		
		if nameValue == "" {
			log.Printf("ERROR: Cannot save environment with empty name")
			return tea.Printf("❌ Environment name cannot be empty")
		}
		
		var description *string
		if descValue != "" {
			description = &descValue
		}
		
		// Check if we're editing an existing environment or creating a new one
		if m.editMode == EnvModeEdit && m.selectedID > 0 {
			// Update existing environment
			log.Printf("DEBUG: Updating existing environment with ID: %d, name: '%s', desc: '%s'", m.selectedID, nameValue, func() string {
				if description != nil {
					return *description
				}
				return "nil"
			}())
			err := m.repos.Environments.Update(m.selectedID, nameValue, description)
			if err != nil {
				log.Printf("ERROR: Failed to update environment: %v", err)
				return tea.Printf("❌ Failed to update environment: %v", err)
			}
			
			log.Printf("DEBUG: Environment updated successfully with ID: %d", m.selectedID)
			
			// Return success message for update
			return EnvironmentUpdatedMsg{EnvironmentID: m.selectedID}
		} else {
			// Create new environment
			log.Printf("DEBUG: Creating new environment")
			env, err := m.repos.Environments.Create(nameValue, description)
			if err != nil {
				log.Printf("ERROR: Failed to create environment: %v", err)
				return tea.Printf("❌ Failed to create environment: %v", err)
			}
			
			log.Printf("DEBUG: Environment created successfully with ID: %d", env.ID)
			
			// Return success message for create
			return EnvironmentCreatedMsg{Environment: *env}
		}
	})
}

// Delete environment command
func (m EnvironmentsModel) deleteEnvironment(envID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		log.Printf("DEBUG: Attempting to delete environment with ID: %d", envID)
		
		// Actually delete from database
		err := m.repos.Environments.Delete(envID)
		if err != nil {
			log.Printf("ERROR: Failed to delete environment: %v", err)
			return EnvironmentsErrorMsg{Err: err}
		}
		
		log.Printf("DEBUG: Environment deleted successfully from database with ID: %d", envID)
		return EnvironmentDeletedMsg{EnvironmentID: envID}
	})
}

// IsMainView returns true if in main list view
func (m EnvironmentsModel) IsMainView() bool {
	// Only return true when we're actually in the main list view
	// This prevents the main TUI from intercepting tab keys when in forms
	return m.editMode == EnvModeList
}

// getEnvironmentStats retrieves real statistics for an environment from the database
func (m EnvironmentsModel) getEnvironmentStats(envID int64) EnvironmentStats {
	// Get agents for this environment
	agents, err := m.repos.Agents.ListByEnvironment(envID)
	if err != nil {
		log.Printf("Error getting agents for environment %d: %v", envID, err)
		agents = []*models.Agent{}
	}

	// Count total runs for all agents in this environment
	totalRuns := 0
	var lastRunTime time.Time
	var hasLastRunTime bool
	
	for _, agent := range agents {
		runs, err := m.repos.AgentRuns.ListByAgent(agent.ID)
		if err != nil {
			log.Printf("Error getting runs for agent %d: %v", agent.ID, err)
			continue
		}
		totalRuns += len(runs)
		
		// Find the most recent run time
		for _, run := range runs {
			if !hasLastRunTime || run.StartedAt.After(lastRunTime) {
				lastRunTime = run.StartedAt
				hasLastRunTime = true
			}
		}
	}

	// Format last used time
	lastUsed := "Never"
	if hasLastRunTime {
		duration := time.Since(lastRunTime)
		if duration < time.Hour {
			lastUsed = fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
		} else if duration < 24*time.Hour {
			lastUsed = fmt.Sprintf("%d hours ago", int(duration.Hours()))
		} else {
			lastUsed = fmt.Sprintf("%d days ago", int(duration.Hours()/24))
		}
	}

	// Determine status based on recent activity
	status := "Inactive"
	if len(agents) > 0 {
		if hasLastRunTime && time.Since(lastRunTime) < 24*time.Hour {
			status = "Active"
		} else {
			status = "Idle"
		}
	}

	return EnvironmentStats{
		ActiveAgents: len(agents),
		TotalRuns:    totalRuns,
		LastUsed:     lastUsed,
		Status:       status,
	}
}