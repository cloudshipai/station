package tabs

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/atotto/clipboard"
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

// UsersModel represents the users management tab
type UsersModel struct {
	BaseTabModel

	// Data access
	repos *repositories.Repositories

	// UI components
	userList      list.Model
	userNameInput textinput.Model
	
	// State
	viewMode         UsersViewMode
	users            []models.User
	selectedUser     *models.User
	showCreateUser   bool
	isCreatingAdmin  bool
	newUserAPIKey    string // Store generated API key for one-time display
	copyFeedback     string // Store copy feedback message
	createFormFocus  int    // 0 = username input, 1 = role selection
}

type UsersViewMode int

const (
	UsersViewList UsersViewMode = iota
	UsersViewCreate
	UsersViewDetails
)

// UsersUserItem implements list.Item interface for bubbles list component
type UsersUserItem struct {
	user models.User
}

func (i UsersUserItem) FilterValue() string { return i.user.Username }
func (i UsersUserItem) Title() string       { return i.user.Username }
func (i UsersUserItem) Description() string {
	role := "User"
	if i.user.IsAdmin {
		role = "Admin"
	}
	apiKeyStatus := "No API Key" 
	if i.user.APIKey != nil {
		// Show first 6 and last 6 characters of API key
		key := *i.user.APIKey
		if len(key) > 12 {
			apiKeyStatus = fmt.Sprintf("API Key: %s...%s", key[:6], key[len(key)-6:])
		} else {
			apiKeyStatus = "API Key: ********"
		}
	}
	return fmt.Sprintf("%s • Created: %s • %s", role, i.user.CreatedAt.Format("2006-01-02"), apiKeyStatus)
}

// Messages for async operations
type UsersTabLoadedMsg struct {
	Users []models.User
}

type UsersTabUserCreatedMsg struct {
	User   models.User
	APIKey string
}

type UsersTabUserDeletedMsg struct {
	UserID int64
}

type UsersErrorMsg struct {
	Err error
}

type ClipboardCopiedMsg struct {
	Content string
}

type clearCopyFeedbackMsg struct{}

// NewUsersModel creates a new users model
func NewUsersModel(database db.Database) *UsersModel {
	// Create user list
	userList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	userList.Title = "System Users"
	userList.SetShowStatusBar(false)
	userList.SetFilteringEnabled(true)

	// Create user name input for user creation
	userNameInput := textinput.New()
	userNameInput.Placeholder = "Enter username"
	userNameInput.Width = 30

	// Get repositories
	repos := repositories.New(database)

	return &UsersModel{
		BaseTabModel:  NewBaseTabModel(database, "Users"),
		repos:         repos,
		userList:      userList,
		userNameInput: userNameInput,
		viewMode:      UsersViewList,
	}
}

// Init initializes the users tab
func (m UsersModel) Init() tea.Cmd {
	return m.loadUsers()
}

// Update handles messages
func (m *UsersModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		listWidth := msg.Width - 4
		m.userList.SetSize(listWidth, msg.Height-10)
		
		// Update delegate styles to use full width for proper selection highlighting
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = styles.GetListItemSelectedStyle(msg.Width)
		delegate.Styles.SelectedDesc = styles.GetListItemSelectedStyle(msg.Width)
		delegate.Styles.NormalTitle = styles.GetListItemStyle(msg.Width)
		delegate.Styles.NormalDesc = styles.GetListItemStyle(msg.Width)
		m.userList.SetDelegate(delegate)

	case tea.KeyMsg:
		// Clear error message on any key press when there's an error
		if m.GetError() != "" {
			m.SetError("")
			// Still process the key press normally after clearing error
		}
		
		// For create view, handle text input first, then navigation
		if m.viewMode == UsersViewCreate {
			return m.handleCreateUserKeysNew(msg)
		}
		return m.handleKeyPress(msg)

	case UsersTabLoadedMsg:
		m.users = msg.Users
		m.updateUserList()
		m.SetLoading(false)

	case UsersTabUserCreatedMsg:
		m.newUserAPIKey = msg.APIKey
		m.showCreateUser = false
		m.viewMode = UsersViewDetails
		m.selectedUser = &msg.User
		return m, m.loadUsers() // Refresh user list

	case UsersTabUserDeletedMsg:
		m.selectedUser = nil
		m.viewMode = UsersViewList
		return m, m.loadUsers() // Refresh user list

	case UsersErrorMsg:
		m.SetError(msg.Err.Error())
		m.SetLoading(false)

	case ClipboardCopiedMsg:
		m.copyFeedback = "✅ API Key copied to clipboard!"
		// Clear feedback after a delay
		return m, tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
			return clearCopyFeedbackMsg{}
		})

	case clearCopyFeedbackMsg:
		m.copyFeedback = ""
	}

	// Update components based on current view mode
	switch m.viewMode {
	case UsersViewList:
		var cmd tea.Cmd
		m.userList, cmd = m.userList.Update(msg)
		cmds = append(cmds, cmd)
	case UsersViewCreate:
		// Text input update is handled directly in handleCreateUserKeysNew
		// No additional update needed here
	}

	return m, tea.Batch(cmds...)
}

// handleKeyPress handles keyboard input for different view modes
func (m *UsersModel) handleKeyPress(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch m.viewMode {
	case UsersViewList:
		return m.handleUserListKeys(msg)
	case UsersViewCreate:
		return m.handleCreateUserKeys(msg)
	case UsersViewDetails:
		return m.handleUserDetailsKeys(msg)
	}
	return m, nil
}

// handleUserListKeys handles keys for user list view
func (m *UsersModel) handleUserListKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "n":
		// Create new user
		m.viewMode = UsersViewCreate
		m.showCreateUser = true
		m.isCreatingAdmin = true // Default to admin
		m.createFormFocus = 0     // Start with username field focused
		m.userNameInput.SetValue("")
		m.userNameInput.Focus()
		return m, nil

	case "enter":
		// View user details
		if item, ok := m.userList.SelectedItem().(UsersUserItem); ok {
			m.selectedUser = &item.user
			m.viewMode = UsersViewDetails
			m.newUserAPIKey = "" // Clear any previous API key
		}
		return m, nil

	case "d":
		// Delete user
		if item, ok := m.userList.SelectedItem().(UsersUserItem); ok {
			return m, m.deleteUser(item.user.ID)
		}
		return m, nil
	
	default:
		// Forward unhandled keys to the user list for navigation (j/k, arrows, etc.)
		var cmd tea.Cmd
		m.userList, cmd = m.userList.Update(msg)
		return m, cmd
	}
}

// handleCreateUserKeys handles keys for user creation view
func (m *UsersModel) handleCreateUserKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	// Only handle specific navigation keys, let everything else pass through
	switch msg.String() {
	case "esc":
		// Cancel user creation
		m.viewMode = UsersViewList
		m.showCreateUser = false
		m.userNameInput.Blur()
		m.createFormFocus = 0
		return m, nil

	case "enter":
		// Create user only if we have a username
		if m.userNameInput.Value() != "" {
			return m, m.createUser(m.userNameInput.Value(), m.isCreatingAdmin)
		}
		return m, nil

	case "tab", "down":
		// Navigate between form fields
		if m.createFormFocus == 0 {
			m.createFormFocus = 1
			m.userNameInput.Blur()
		} else {
			m.createFormFocus = 0
			m.userNameInput.Focus()
		}
		return m, nil

	case "shift+tab", "up":
		// Navigate backwards between form fields
		if m.createFormFocus == 1 {
			m.createFormFocus = 0
			m.userNameInput.Focus()
		} else {
			m.createFormFocus = 1
			m.userNameInput.Blur()
		}
		return m, nil
	}

	// Handle role-specific keys when role field is focused
	if m.createFormFocus == 1 {
		switch msg.String() {
		case "left", "right", " ":
			// Toggle role when role field is focused
			m.isCreatingAdmin = !m.isCreatingAdmin
			return m, nil
		}
		// Don't handle any other keys when role is focused
		return m, nil
	}

	// If we get here, let the text input handle the key (this happens in Update method)
	return m, nil
}

// handleCreateUserKeysNew handles keys for user creation with proper text input handling
func (m *UsersModel) handleCreateUserKeysNew(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle special navigation keys first (these should NOT go to text input)
	switch msg.String() {
	case "esc":
		// Cancel user creation
		m.viewMode = UsersViewList
		m.showCreateUser = false
		m.userNameInput.Blur()
		m.createFormFocus = 0
		return m, nil

	case "enter":
		// Create user only if we have a username
		if m.userNameInput.Value() != "" {
			return m, m.createUser(m.userNameInput.Value(), m.isCreatingAdmin)
		}
		return m, nil

	case "tab", "down":
		// Navigate between form fields
		if m.createFormFocus == 0 {
			m.createFormFocus = 1
			m.userNameInput.Blur()
		} else {
			m.createFormFocus = 0
			m.userNameInput.Focus()
		}
		return m, nil

	case "shift+tab", "up":
		// Navigate backwards between form fields
		if m.createFormFocus == 1 {
			m.createFormFocus = 0
			m.userNameInput.Focus()
		} else {
			m.createFormFocus = 1
			m.userNameInput.Blur()
		}
		return m, nil
	}

	// Handle role-specific keys when role field is focused
	if m.createFormFocus == 1 {
		switch msg.String() {
		case "left", "right", " ":
			// Toggle role when role field is focused
			m.isCreatingAdmin = !m.isCreatingAdmin
			return m, nil
		}
		// Don't send other keys to text input when role is focused
		return m, nil
	}

	// For username field (createFormFocus == 0), let text input handle the key
	if m.createFormFocus == 0 {
		var cmd tea.Cmd
		m.userNameInput, cmd = m.userNameInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleUserDetailsKeys handles keys for user details view
func (m *UsersModel) handleUserDetailsKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "c":
		// Copy API key (if available)
		if m.newUserAPIKey != "" {
			return m, m.copyToClipboard(m.newUserAPIKey)
		}
		return m, nil

	case "r":
		// Regenerate API key
		if m.selectedUser != nil {
			return m, m.regenerateAPIKey(m.selectedUser.ID)
		}
		return m, nil

	case "esc":
		// Go back to user list
		m.viewMode = UsersViewList
		m.newUserAPIKey = "" // Clear API key when leaving details
		return m, nil
	}
	return m, nil
}

// View renders the users tab
func (m UsersModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}

	switch m.viewMode {
	case UsersViewList:
		return m.renderUsersList()
	case UsersViewCreate:
		return m.renderCreateUser()
	case UsersViewDetails:
		return m.renderUserDetails()
	default:
		return m.renderUsersList()
	}
}

// RefreshData reloads users from database
func (m UsersModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadUsers()
}

// loadUsers loads users from database
func (m UsersModel) loadUsers() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		users, err := m.repos.Users.List()
		if err != nil {
			log.Printf("Error loading users: %v", err)
			return UsersErrorMsg{Err: err}
		}

		// Convert from []*models.User to []models.User
		userSlice := make([]models.User, len(users))
		for i, user := range users {
			userSlice[i] = *user
		}

		return UsersTabLoadedMsg{Users: userSlice}
	})
}

// updateUserList updates the user list component
func (m *UsersModel) updateUserList() {
	items := make([]list.Item, len(m.users))
	for i, user := range m.users {
		items[i] = UsersUserItem{user: user}
	}
	m.userList.SetItems(items)
}

// generateAPIKey generates a secure API key
func generateUserAPIKey() string {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		log.Printf("Error generating API key: %v", err)
		// Fallback to timestamp-based key (not recommended for production)
		return fmt.Sprintf("sk_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("sk_%s", hex.EncodeToString(bytes))
}

// createUser creates a new user with API key
func (m UsersModel) createUser(username string, isAdmin bool) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Generate API key
		apiKey := generateUserAPIKey()
		
		// Create user with placeholder public key
		publicKey := "ui-generated-" + apiKey
		user, err := m.repos.Users.Create(username, publicKey, isAdmin, &apiKey)
		if err != nil {
			log.Printf("Error creating user: %v", err)
			return UsersErrorMsg{Err: err}
		}

		return UsersTabUserCreatedMsg{User: *user, APIKey: apiKey}
	})
}

// deleteUser deletes a user
func (m UsersModel) deleteUser(userID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := m.repos.Users.Delete(userID)
		if err != nil {
			log.Printf("Error deleting user: %v", err)
			return UsersErrorMsg{Err: err}
		}

		return UsersTabUserDeletedMsg{UserID: userID}
	})
}

// regenerateAPIKey generates a new API key for a user
func (m UsersModel) regenerateAPIKey(userID int64) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Generate new API key
		apiKey := generateUserAPIKey()
		
		err := m.repos.Users.UpdateAPIKey(userID, &apiKey)
		if err != nil {
			log.Printf("Error updating API key: %v", err)
			return UsersErrorMsg{Err: err}
		}

		// Load the updated user
		user, err := m.repos.Users.GetByID(userID)
		if err != nil {
			log.Printf("Error loading updated user: %v", err)
			return UsersErrorMsg{Err: err}
		}

		return UsersTabUserCreatedMsg{User: *user, APIKey: apiKey}
	})
}

// copyToClipboard copies the given text to the system clipboard
func (m UsersModel) copyToClipboard(text string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := clipboard.WriteAll(text)
		if err != nil {
			log.Printf("Failed to copy to clipboard: %v", err)
			return UsersErrorMsg{Err: fmt.Errorf("failed to copy to clipboard: %v", err)}
		}
		return ClipboardCopiedMsg{Content: text}
	})
}

// renderUsersList renders the users list view
func (m UsersModel) renderUsersList() string {
	var sections []string

	// Header
	header := components.RenderSectionHeader("User Management")
	sections = append(sections, header)

	// Show error message if there is one
	if m.GetError() != "" {
		errorMsg := styles.ErrorStyle.Render("⚠ " + m.GetError())
		sections = append(sections, errorMsg)
		sections = append(sections, "")
	} else {
		sections = append(sections, "")
	}

	// User list
	userListView := m.userList.View()
	sections = append(sections, userListView)

	// Help text
	helpText := styles.HelpStyle.Render("• ↑↓: navigate • n: new user • enter: details • d: delete")
	if m.GetError() != "" {
		helpText = styles.HelpStyle.Render("• ↑↓: navigate • n: new user • enter: details • d: delete • any key: dismiss error")
	}
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderCreateUser renders the create user view
func (m UsersModel) renderCreateUser() string {
	var sections []string

	// Header
	header := components.RenderSectionHeader("Create New User")
	sections = append(sections, header)
	sections = append(sections, "")

	// Username input
	var usernameLabel string
	if m.createFormFocus == 0 {
		usernameLabel = styles.HeaderStyle.Render("► Username:")
	} else {
		usernameLabel = lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Username:")
	}
	sections = append(sections, usernameLabel)
	sections = append(sections, m.userNameInput.View())
	sections = append(sections, "")

	// Role selection
	var roleLabel string
	if m.createFormFocus == 1 {
		roleLabel = styles.HeaderStyle.Render("► Role:")
	} else {
		roleLabel = lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Role:")
	}
	
	var roleText string
	if m.createFormFocus == 1 {
		// Show focused styling when role field is selected
		if m.isCreatingAdmin {
			roleText = lipgloss.NewStyle().
				Foreground(styles.Primary).
				Background(lipgloss.Color("#414868")).
				Render(" Admin ") + " / " + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("User")
		} else {
			roleText = lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Admin") + " / " + 
				lipgloss.NewStyle().
				Foreground(styles.Primary).
				Background(lipgloss.Color("#414868")).
				Render(" User ")
		}
	} else {
		// Show normal styling when not focused
		if m.isCreatingAdmin {
			roleText = styles.SuccessStyle.Render("Admin") + " / " + lipgloss.NewStyle().Foreground(styles.TextMuted).Render("User")
		} else {
			roleText = lipgloss.NewStyle().Foreground(styles.TextMuted).Render("Admin") + " / " + styles.SuccessStyle.Render("User")
		}
	}
	sections = append(sections, roleLabel)
	sections = append(sections, roleText)
	sections = append(sections, "")

	// Info box
	infoText := "• Admin users can access admin API routes and see admin MCP tools\n• Regular users have limited access\n• API key will be generated automatically"
	infoBox := styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(infoText)
	sections = append(sections, infoBox)

	// Help text
	helpText := styles.HelpStyle.Render("• tab/↑↓: navigate fields • ←→: toggle role (when selected) • enter: create • esc: cancel")
	sections = append(sections, "")
	sections = append(sections, helpText)

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderUserDetails renders the user details view
func (m UsersModel) renderUserDetails() string {
	if m.selectedUser == nil {
		return "No user selected"
	}

	var sections []string

	// Header
	header := components.RenderSectionHeader(fmt.Sprintf("User Details: %s", m.selectedUser.Username))
	sections = append(sections, header)
	sections = append(sections, "")

	// User info
	userInfo := []string{
		fmt.Sprintf("ID: %d", m.selectedUser.ID),
		fmt.Sprintf("Username: %s", m.selectedUser.Username),
		fmt.Sprintf("Role: %s", func() string {
			if m.selectedUser.IsAdmin {
				return "Admin"
			}
			return "User"
		}()),
		fmt.Sprintf("Created: %s", m.selectedUser.CreatedAt.Format("2006-01-02 15:04:05")),
		fmt.Sprintf("Updated: %s", m.selectedUser.UpdatedAt.Format("2006-01-02 15:04:05")),
	}

	userInfoBox := styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(strings.Join(userInfo, "\n"))
	sections = append(sections, userInfoBox)
	sections = append(sections, "")

	// API Key section
	apiKeyHeader := styles.HeaderStyle.Render("API Key")
	sections = append(sections, apiKeyHeader)
	sections = append(sections, "")

	var apiKeyContent string
	if m.newUserAPIKey != "" {
		// Show full API key (one-time display)
		apiKeyContent = fmt.Sprintf("🔑 New API Key (copy now, it won't be shown again):\n\n%s", 
			styles.SuccessStyle.Render(m.newUserAPIKey))
	} else if m.selectedUser.APIKey != nil {
		// Show truncated API key
		key := *m.selectedUser.APIKey
		if len(key) > 12 {
			truncated := fmt.Sprintf("%s...%s", key[:6], key[len(key)-6:])
			apiKeyContent = fmt.Sprintf("Current API Key: %s", truncated)
		} else {
			apiKeyContent = "Current API Key: ********"
		}
	} else {
		apiKeyContent = "No API Key assigned"
	}

	apiKeyBox := styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(apiKeyContent)
	sections = append(sections, apiKeyBox)

	// Help text
	var helpText string
	if m.newUserAPIKey != "" {
		helpText = styles.HelpStyle.Render("• c: copy key • r: regenerate • esc: back")
	} else {
		helpText = styles.HelpStyle.Render("• r: regenerate API key • esc: back")
	}
	sections = append(sections, "")
	sections = append(sections, helpText)

	// Show copy feedback if available
	if m.copyFeedback != "" {
		sections = append(sections, "")
		sections = append(sections, styles.SuccessStyle.Render(m.copyFeedback))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// IsMainView returns true if in main view
func (m UsersModel) IsMainView() bool {
	return m.viewMode == UsersViewList
}

// Navigation methods
func (m UsersModel) CanGoBack() bool {
	return m.viewMode != UsersViewList
}

func (m *UsersModel) GoBack() tea.Cmd {
	if m.viewMode != UsersViewList {
		m.viewMode = UsersViewList
		m.newUserAPIKey = ""
		m.selectedUser = nil
		m.showCreateUser = false
		m.userNameInput.Blur()
	}
	return nil
}

func (m UsersModel) GetBreadcrumb() string {
	switch m.viewMode {
	case UsersViewList:
		return "Users"
	case UsersViewCreate:
		return "Users › Create User"
	case UsersViewDetails:
		if m.selectedUser != nil {
			return fmt.Sprintf("Users › %s", m.selectedUser.Username)
		}
		return "Users › Details"
	default:
		return "Users"
	}
}