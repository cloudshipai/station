package apps

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db/repositories"
	"station/pkg/models"
)

type UsersAppState int

const (
	StateUsersList UsersAppState = iota
	StateCreateAPIKey
)

type UsersApp struct {
	repos      *repositories.Repositories
	users      []*models.User
	cursor     int
	loading    bool
	err        error
	state      UsersAppState
	newAPIKey  string
	statusMsg  string
}

type usersLoadedMsg []*models.User
type usersErrorMsg error
type apiKeyCreatedMsg string

func NewUsersApp(repos *repositories.Repositories) *UsersApp {
	return &UsersApp{
		repos:   repos,
		loading: true,
		state:   StateUsersList,
	}
}

func (app *UsersApp) Init() tea.Cmd {
	return app.loadUsers
}

func (app *UsersApp) loadUsers() tea.Msg {
	users, err := app.repos.Users.List()
	if err != nil {
		return usersErrorMsg(err)
	}
	return usersLoadedMsg(users)
}

func (app *UsersApp) generateAPIKey(userID int64) tea.Cmd {
	return func() tea.Msg {
		// Generate a random API key
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			return usersErrorMsg(fmt.Errorf("failed to generate API key: %w", err))
		}
		
		apiKey := hex.EncodeToString(keyBytes)
		
		// Update user with new API key
		err := app.repos.Users.UpdateAPIKey(userID, &apiKey)
		if err != nil {
			return usersErrorMsg(fmt.Errorf("failed to save API key: %w", err))
		}
		
		return apiKeyCreatedMsg(apiKey)
	}
}

func (app *UsersApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case usersLoadedMsg:
		app.users = []*models.User(msg)
		app.loading = false
		return app, nil

	case usersErrorMsg:
		app.err = error(msg)
		app.loading = false
		return app, nil

	case apiKeyCreatedMsg:
		app.newAPIKey = string(msg)
		app.state = StateCreateAPIKey
		app.statusMsg = "API key generated successfully!"
		return app, nil

	case tea.KeyMsg:
		if app.loading {
			return app, nil
		}

		switch app.state {
		case StateUsersList:
			return app.updateUsersList(msg)
		case StateCreateAPIKey:
			return app.updateCreateAPIKey(msg)
		}
	}

	return app, nil
}

func (app *UsersApp) updateUsersList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if app.cursor > 0 {
			app.cursor--
		}
	case "down", "j":
		if app.cursor < len(app.users)-1 {
			app.cursor++
		}
	case "r":
		app.loading = true
		return app, app.loadUsers
	case "g":
		if len(app.users) > 0 {
			userID := app.users[app.cursor].ID
			return app, app.generateAPIKey(userID)
		}
	}
	return app, nil
}

func (app *UsersApp) updateCreateAPIKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		app.state = StateUsersList
		app.newAPIKey = ""
		app.statusMsg = ""
		app.loading = true
		return app, app.loadUsers
	}
	return app, nil
}

func (app *UsersApp) View() string {
	if app.loading {
		return "Loading users..."
	}

	if app.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'r' to retry, Esc to go back", app.err)
	}

	switch app.state {
	case StateUsersList:
		return app.viewUsersList()
	case StateCreateAPIKey:
		return app.viewCreateAPIKey()
	}

	return "Unknown state"
}

func (app *UsersApp) viewUsersList() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("👥 Users & API Keys"))
	content.WriteString("\n\n")

	if len(app.users) == 0 {
		content.WriteString("No users found.\n")
	} else {
		for i, user := range app.users {
			style := normalStyle
			if i == app.cursor {
				style = selectedStyle
			}

			adminStatus := ""
			if user.IsAdmin {
				adminStatus = " (Admin)"
			}

			apiKeyStatus := "No API Key"
			if user.APIKey != nil {
				apiKeyStatus = fmt.Sprintf("API Key: %s...", (*user.APIKey)[:8])
			}

			userInfo := fmt.Sprintf("%s%s - %s", user.Username, adminStatus, apiKeyStatus)
			content.WriteString(style.Render(userInfo))
			content.WriteString("\n")
		}
	}

	if app.statusMsg != "" {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render(app.statusMsg))
	}

	content.WriteString("\n\n")
	content.WriteString("Commands: ↑/↓ to navigate, g to generate API key, r to refresh, Esc to go back")

	return content.String()
}

func (app *UsersApp) viewCreateAPIKey() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("🔑 API Key Generated"))
	content.WriteString("\n\n")

	content.WriteString("New API Key (save this securely):\n\n")
	content.WriteString(selectedStyle.Render(app.newAPIKey))
	content.WriteString("\n\n")

	content.WriteString("⚠️  This key will only be shown once. Make sure to copy it now!\n\n")
	content.WriteString("Press Enter or Esc to continue")

	return content.String()
}