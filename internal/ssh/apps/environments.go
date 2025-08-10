package apps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"station/internal/db/repositories"
	"station/pkg/models"
)

type EnvironmentsApp struct {
	repos        *repositories.Repositories
	environments []*models.Environment
	cursor       int
	loading      bool
	err          error
}

type environmentsLoadedMsg []*models.Environment
type environmentsErrorMsg error

func NewEnvironmentsApp(repos *repositories.Repositories) *EnvironmentsApp {
	return &EnvironmentsApp{
		repos:   repos,
		loading: true,
	}
}

func (app *EnvironmentsApp) Init() tea.Cmd {
	return app.loadEnvironments
}

func (app *EnvironmentsApp) loadEnvironments() tea.Msg {
	environments, err := app.repos.Environments.List()
	if err != nil {
		return environmentsErrorMsg(err)
	}
	return environmentsLoadedMsg(environments)
}

func (app *EnvironmentsApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case environmentsLoadedMsg:
		app.environments = []*models.Environment(msg)
		app.loading = false
		return app, nil

	case environmentsErrorMsg:
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
			if app.cursor < len(app.environments)-1 {
				app.cursor++
			}
		case "r":
			app.loading = true
			return app, app.loadEnvironments
		}
	}

	return app, nil
}

func (app *EnvironmentsApp) View() string {
	if app.loading {
		return "Loading environments..."
	}

	if app.err != nil {
		return fmt.Sprintf("Error loading environments: %v\n\nPress 'r' to retry, Esc to go back", app.err)
	}

	var content strings.Builder
	content.WriteString(titleStyle.Render("🌍 Environments"))
	content.WriteString("\n\n")

	if len(app.environments) == 0 {
		content.WriteString("No environments found.\n")
	} else {
		for i, env := range app.environments {
			style := normalStyle
			if i == app.cursor {
				style = selectedStyle
			}

			desc := "No description"
			if env.Description != nil {
				desc = *env.Description
			}

			envInfo := fmt.Sprintf("%s - %s", env.Name, desc)
			content.WriteString(style.Render(envInfo))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString("Commands: ↑/↓ to navigate, r to refresh, Esc to go back")

	return content.String()
}