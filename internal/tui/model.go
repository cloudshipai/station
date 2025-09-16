package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
)

type Model struct {
	db            *db.DB
	repos         *repositories.Repositories
	genkitService services.AgentServiceInterface
	content       string
	width         int
	height        int
}

func NewChatModel(database *db.DB, repos *repositories.Repositories, genkitService services.AgentServiceInterface) *Model {
	return &Model{
		db:            database,
		repos:         repos,
		genkitService: genkitService,
		content:       "Welcome to Station SSH Interface\n\nPress 'q' to quit",
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m *Model) View() string {
	style := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#04B575"))

	content := fmt.Sprintf("%s\n\nTerminal size: %dx%d", m.content, m.width, m.height)
	
	return style.Render(content)
}