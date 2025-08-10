package apps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db/repositories"
)

type DashboardState int

const (
	StateMainMenu DashboardState = iota
	StateEnvironments
	StateUsers
	StateAgents
	StateConfigs
	StateRuns
)

type Dashboard struct {
	repos    *repositories.Repositories
	username string
	state    DashboardState
	cursor   int
	width    int
	height   int

	// Sub-apps
	environmentsApp *EnvironmentsApp
	usersApp        *UsersApp
	agentsApp       *AgentsApp
	configsApp      *ConfigsApp
	runsApp         *RunsApp
}

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("62")).
			Background(lipgloss.Color("235")).
			Padding(0, 1).
			MarginBottom(1)

	menuStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Padding(0, 1)
)

func NewDashboard(repos *repositories.Repositories, username string) *Dashboard {
	return &Dashboard{
		repos:    repos,
		username: username,
		state:    StateMainMenu,
		cursor:   0,
	}
}

func (d *Dashboard) Init() tea.Cmd {
	return nil
}

func (d *Dashboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		return d, nil

	case tea.KeyMsg:
		switch d.state {
		case StateMainMenu:
			return d.updateMainMenu(msg)
		case StateEnvironments:
			return d.updateEnvironments(msg)
		case StateUsers:
			return d.updateUsers(msg)
		case StateAgents:
			return d.updateAgents(msg)
		case StateConfigs:
			return d.updateConfigs(msg)
		case StateRuns:
			return d.updateRuns(msg)
		}
	}

	return d, nil
}

func (d *Dashboard) updateMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return d, tea.Quit
	case "up", "k":
		if d.cursor > 0 {
			d.cursor--
		}
	case "down", "j":
		if d.cursor < 4 {
			d.cursor++
		}
	case "enter", " ":
		switch d.cursor {
		case 0:
			d.state = StateEnvironments
			if d.environmentsApp == nil {
				d.environmentsApp = NewEnvironmentsApp(d.repos)
			}
		case 1:
			d.state = StateUsers
			if d.usersApp == nil {
				d.usersApp = NewUsersApp(d.repos)
			}
		case 2:
			d.state = StateAgents
			if d.agentsApp == nil {
				d.agentsApp = NewAgentsApp(d.repos)
			}
		case 3:
			d.state = StateConfigs
			if d.configsApp == nil {
				d.configsApp = NewConfigsApp(d.repos)
			}
		case 4:
			d.state = StateRuns
			if d.runsApp == nil {
				d.runsApp = NewRunsApp(d.repos)
			}
		}
	}
	return d, nil
}

func (d *Dashboard) updateEnvironments(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		d.state = StateMainMenu
		return d, nil
	}
	
	if d.environmentsApp != nil {
		updatedApp, cmd := d.environmentsApp.Update(msg)
		d.environmentsApp = updatedApp.(*EnvironmentsApp)
		return d, cmd
	}
	return d, nil
}

func (d *Dashboard) updateUsers(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		d.state = StateMainMenu
		return d, nil
	}
	
	if d.usersApp != nil {
		updatedApp, cmd := d.usersApp.Update(msg)
		d.usersApp = updatedApp.(*UsersApp)
		return d, cmd
	}
	return d, nil
}

func (d *Dashboard) updateAgents(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		d.state = StateMainMenu
		return d, nil
	}
	
	if d.agentsApp != nil {
		updatedApp, cmd := d.agentsApp.Update(msg)
		d.agentsApp = updatedApp.(*AgentsApp)
		return d, cmd
	}
	return d, nil
}

func (d *Dashboard) updateConfigs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		d.state = StateMainMenu
		return d, nil
	}
	
	if d.configsApp != nil {
		updatedApp, cmd := d.configsApp.Update(msg)
		d.configsApp = updatedApp.(*ConfigsApp)
		return d, cmd
	}
	return d, nil
}

func (d *Dashboard) updateRuns(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		d.state = StateMainMenu
		return d, nil
	}
	
	if d.runsApp != nil {
		updatedApp, cmd := d.runsApp.Update(msg)
		d.runsApp = updatedApp.(*RunsApp)
		return d, cmd
	}
	return d, nil
}

func (d *Dashboard) View() string {
	switch d.state {
	case StateMainMenu:
		return d.viewMainMenu()
	case StateEnvironments:
		if d.environmentsApp != nil {
			return d.environmentsApp.View()
		}
	case StateUsers:
		if d.usersApp != nil {
			return d.usersApp.View()
		}
	case StateAgents:
		if d.agentsApp != nil {
			return d.agentsApp.View()
		}
	case StateConfigs:
		if d.configsApp != nil {
			return d.configsApp.View()
		}
	case StateRuns:
		if d.runsApp != nil {
			return d.runsApp.View()
		}
	}
	return "Loading..."
}

func (d *Dashboard) viewMainMenu() string {
	title := titleStyle.Render(fmt.Sprintf("Station Admin Dashboard - %s", d.username))
	
	menuItems := []string{
		"🌍 Environments",
		"👥 Users & API Keys", 
		"🤖 Agents",
		"⚙️  MCP Configurations",
		"📊 Agent Runs",
	}

	var menuContent strings.Builder
	for i, item := range menuItems {
		if i == d.cursor {
			menuContent.WriteString(selectedStyle.Render("> " + item))
		} else {
			menuContent.WriteString(normalStyle.Render("  " + item))
		}
		menuContent.WriteString("\n")
	}

	menu := menuStyle.Render(menuContent.String())
	
	help := "\nNavigation: ↑/↓ or j/k to move, Enter to select, q to quit"
	
	return lipgloss.JoinVertical(lipgloss.Left, title, menu, help)
}