package tabs

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db"
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

type EnvironmentDeletedMsg struct {
	EnvironmentID int64
}

// NewEnvironmentsModel creates a new environments model
func NewEnvironmentsModel(database db.Database) *EnvironmentsModel {
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
		
		// Handle navigation between modes
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
	}

	return m, nil
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
	
	switch msg.String() {
	case "esc":
		m.editMode = EnvModeList
		m.nameInput.Blur()
		m.descInput.Blur()
		m.nameInput.SetValue("")
		m.descInput.SetValue("")
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

	if m.GetError() != "" {
		return styles.ErrorStyle.Render("Error loading environments: " + m.GetError())
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
		// TODO: Load real environments from database
		// For now, return mock data
		desc1 := "Production environment with full access to external services"
		desc2 := "Development environment for testing new features"
		desc3 := "Isolated sandbox environment with restricted access"
		
		environments := []models.Environment{
			{
				ID:          1,
				Name:        "Production",
				Description: &desc1,
				CreatedAt:   time.Now().Add(-time.Hour * 24 * 30),
				UpdatedAt:   time.Now().Add(-time.Hour * 24),
			},
			{
				ID:          2,
				Name:        "Development",
				Description: &desc2,
				CreatedAt:   time.Now().Add(-time.Hour * 24 * 15),
				UpdatedAt:   time.Now().Add(-time.Hour * 12),
			},
			{
				ID:          3,
				Name:        "Sandbox",
				Description: &desc3,
				CreatedAt:   time.Now().Add(-time.Hour * 24 * 7),
				UpdatedAt:   time.Now().Add(-time.Hour * 6),
			},
		}

		return EnvironmentsLoadedMsg{Environments: environments}
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
	// Header with stats
	header := components.RenderSectionHeader(fmt.Sprintf("Environments (%d total)", len(m.environments)))

	// List component
	listView := m.list.View()

	// Help text
	helpText := styles.HelpStyle.Render("• enter: view details • n: new environment • e: edit • d: delete")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listView,
		"",
		helpText,
	)
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
	
	// TODO: Get actual usage statistics from database
	statusInfo := []string{
		"Active agents: 3",
		"Total runs: 156",
		"Last used: 2 hours ago",
		"Status: Active",
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
	helpText := styles.HelpStyle.Render("• tab: switch fields • ctrl+s: save • esc: cancel")
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Save environment command
func (m EnvironmentsModel) saveEnvironment() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually save to database
		// For now, just return success and go back to list
		m.editMode = EnvModeList
		m.nameInput.Blur()
		m.descInput.Blur()
		m.nameInput.SetValue("")
		m.descInput.SetValue("")
		return nil
	})
}

// Delete environment command
func (m EnvironmentsModel) deleteEnvironment(envID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually delete from database
		// For now, just return success
		return EnvironmentDeletedMsg{EnvironmentID: envID}
	})
}

// IsMainView returns true if in main list view
func (m EnvironmentsModel) IsMainView() bool {
	// Use the base implementation for reliable navigation
	return m.BaseTabModel.IsMainView()
}