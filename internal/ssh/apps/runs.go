package apps

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db/repositories"
	"station/pkg/models"
)

type RunsApp struct {
	repos   *repositories.Repositories
	runs    []*models.AgentRunWithDetails
	cursor  int
	loading bool
	err     error
}

type runsLoadedMsg []*models.AgentRunWithDetails
type runsErrorMsg error

func NewRunsApp(repos *repositories.Repositories) *RunsApp {
	return &RunsApp{
		repos:   repos,
		loading: true,
	}
}

func (app *RunsApp) Init() tea.Cmd {
	return app.loadRuns
}

func (app *RunsApp) loadRuns() tea.Msg {
	runs, err := app.repos.AgentRuns.ListRecent(context.Background(), 20) // Get latest 20 runs
	if err != nil {
		return runsErrorMsg(err)
	}
	return runsLoadedMsg(runs)
}

func (app *RunsApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case runsLoadedMsg:
		app.runs = []*models.AgentRunWithDetails(msg)
		app.loading = false
		return app, nil

	case runsErrorMsg:
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
			if app.cursor < len(app.runs)-1 {
				app.cursor++
			}
		case "r":
			app.loading = true
			return app, app.loadRuns
		}
	}

	return app, nil
}

func (app *RunsApp) View() string {
	if app.loading {
		return "Loading agent runs..."
	}

	if app.err != nil {
		return fmt.Sprintf("Error loading runs: %v\n\nPress 'r' to retry, Esc to go back", app.err)
	}

	var content strings.Builder
	content.WriteString(titleStyle.Render("📊 Agent Runs"))
	content.WriteString("\n\n")

	if len(app.runs) == 0 {
		content.WriteString("No agent runs found.\n")
	} else {
		for i, run := range app.runs {
			style := normalStyle
			if i == app.cursor {
				style = selectedStyle
			}

			statusStyle := lipgloss.NewStyle()
			switch run.Status {
			case "completed":
				statusStyle = statusStyle.Foreground(lipgloss.Color("46")) // Green
			case "failed":
				statusStyle = statusStyle.Foreground(lipgloss.Color("196")) // Red
			case "timeout":
				statusStyle = statusStyle.Foreground(lipgloss.Color("214")) // Orange
			default:
				statusStyle = statusStyle.Foreground(lipgloss.Color("240")) // Gray
			}

			duration := "Running"
			if run.CompletedAt != nil {
				duration = run.CompletedAt.Sub(run.StartedAt).Truncate(time.Second).String()
			}

			runInfo := fmt.Sprintf("%s by %s - %s (%s) - %d steps", 
				run.AgentName, run.Username, 
				statusStyle.Render(run.Status), 
				duration, run.StepsTaken)
			
			content.WriteString(style.Render(runInfo))
			content.WriteString("\n")

			// Show task preview if selected
			if i == app.cursor {
				taskPreview := run.Task
				if len(taskPreview) > 60 {
					taskPreview = taskPreview[:60] + "..."
				}
				content.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("240")).
					MarginLeft(2).
					Render("Task: " + taskPreview))
				content.WriteString("\n")
			}
		}
	}

	content.WriteString("\n")
	content.WriteString("Commands: ↑/↓ to navigate, r to refresh, Esc to go back")

	return content.String()
}