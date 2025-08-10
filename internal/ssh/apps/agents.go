package apps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"station/internal/db/repositories"
	"station/pkg/models"
)

type AgentsApp struct {
	repos   *repositories.Repositories
	agents  []*models.Agent
	cursor  int
	loading bool
	err     error
}

type agentsLoadedMsg []*models.Agent
type agentsErrorMsg error

func NewAgentsApp(repos *repositories.Repositories) *AgentsApp {
	return &AgentsApp{
		repos:   repos,
		loading: true,
	}
}

func (app *AgentsApp) Init() tea.Cmd {
	return app.loadAgents
}

func (app *AgentsApp) loadAgents() tea.Msg {
	agents, err := app.repos.Agents.List()
	if err != nil {
		return agentsErrorMsg(err)
	}
	return agentsLoadedMsg(agents)
}

func (app *AgentsApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case agentsLoadedMsg:
		app.agents = []*models.Agent(msg)
		app.loading = false
		return app, nil

	case agentsErrorMsg:
		app.err = error(msg)
		app.loading = false
		return app, nil

	case tea.KeyMsg:
		if app.loading {
			return app, nil
		}

		switch msg.String() {
		case "up", "k":
			if app.cursor > 0 {
				app.cursor--
			}
		case "down", "j":
			if app.cursor < len(app.agents)-1 {
				app.cursor++
			}
		case "r":
			app.loading = true
			return app, app.loadAgents
		}
	}

	return app, nil
}

func (app *AgentsApp) View() string {
	if app.loading {
		return "Loading agents..."
	}

	if app.err != nil {
		return fmt.Sprintf("Error loading agents: %v\n\nPress 'r' to retry, Esc to go back", app.err)
	}

	var content strings.Builder
	content.WriteString(titleStyle.Render("🤖 Agents"))
	content.WriteString("\n\n")

	if len(app.agents) == 0 {
		content.WriteString("No agents found.\n")
	} else {
		for i, agent := range app.agents {
			style := normalStyle
			if i == app.cursor {
				style = selectedStyle
			}

			agentInfo := fmt.Sprintf("%s - %s (Max Steps: %d)", 
				agent.Name, agent.Description, agent.MaxSteps)
			content.WriteString(style.Render(agentInfo))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString("Commands: ↑/↓ to navigate, r to refresh, Esc to go back")

	return content.String()
}