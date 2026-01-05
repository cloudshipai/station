package work

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			MarginBottom(1)

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	completedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	failedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
)

type Dashboard struct {
	store     *WorkStore
	stationID string

	activeWork  map[string]*WorkRecord
	recentWork  []*WorkRecord
	mu          sync.RWMutex
	maxRecent   int
	lastUpdated time.Time
}

func NewDashboard(store *WorkStore, stationID string) *Dashboard {
	return &Dashboard{
		store:      store,
		stationID:  stationID,
		activeWork: make(map[string]*WorkRecord),
		recentWork: make([]*WorkRecord, 0),
		maxRecent:  10,
	}
}

type workUpdateMsg struct {
	record *WorkRecord
}

type tickMsg time.Time

func (d *Dashboard) Run(ctx context.Context) error {
	p := tea.NewProgram(d.initialModel(ctx), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type dashboardModel struct {
	ctx         context.Context
	dashboard   *Dashboard
	activeWork  map[string]*WorkRecord
	recentWork  []*WorkRecord
	lastUpdated time.Time
	width       int
	height      int
	quitting    bool
}

func (d *Dashboard) initialModel(ctx context.Context) dashboardModel {
	return dashboardModel{
		ctx:         ctx,
		dashboard:   d,
		activeWork:  make(map[string]*WorkRecord),
		recentWork:  make([]*WorkRecord, 0),
		lastUpdated: time.Now(),
		width:       80,
		height:      24,
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.watchWork(),
		m.tick(),
	)
}

func (m dashboardModel) watchWork() tea.Cmd {
	return func() tea.Msg {
		ch, err := m.dashboard.store.WatchAll(m.ctx)
		if err != nil {
			return nil
		}

		for {
			select {
			case record := <-ch:
				if record != nil {
					return workUpdateMsg{record: record}
				}
			case <-m.ctx.Done():
				return nil
			}
		}
	}
}

func (m dashboardModel) tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			return m, m.watchWork()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case workUpdateMsg:
		if msg.record != nil {
			m.handleWorkUpdate(msg.record)
			m.lastUpdated = time.Now()
		}
		return m, m.watchWork()

	case tickMsg:
		return m, m.tick()
	}

	return m, nil
}

func (m *dashboardModel) handleWorkUpdate(record *WorkRecord) {
	switch record.Status {
	case StatusAssigned, StatusAccepted:
		m.activeWork[record.WorkID] = record
	case StatusComplete, StatusFailed, StatusEscalated:
		delete(m.activeWork, record.WorkID)
		m.recentWork = prepend(m.recentWork, record, 10)
	}
}

func prepend(slice []*WorkRecord, record *WorkRecord, maxLen int) []*WorkRecord {
	result := append([]*WorkRecord{record}, slice...)
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}

func (m dashboardModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	title := titleStyle.Render("STATION LATTICE DASHBOARD")
	b.WriteString(title)
	b.WriteString("\n")

	header := fmt.Sprintf("Station: %s | Updated: %s",
		m.dashboard.stationID[:min(8, len(m.dashboard.stationID))],
		m.lastUpdated.Format("15:04:05"))
	b.WriteString(labelStyle.Render(header))
	b.WriteString("\n\n")

	b.WriteString(activeStyle.Render(fmt.Sprintf("ACTIVE WORK (%d)", len(m.activeWork))))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.width-4)))
	b.WriteString("\n")

	if len(m.activeWork) == 0 {
		b.WriteString(labelStyle.Render("  No active work"))
		b.WriteString("\n")
	} else {
		for _, work := range m.activeWork {
			elapsed := time.Since(work.AssignedAt).Round(time.Second)
			line := fmt.Sprintf("  ▶ %s  %s  %s",
				truncate(work.WorkID, 12),
				truncate(work.AgentName, 20),
				elapsed)
			b.WriteString(activeStyle.Render(line))
			b.WriteString("\n")
			task := fmt.Sprintf("    └─ %s", truncate(work.Task, 50))
			b.WriteString(labelStyle.Render(task))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(completedStyle.Render(fmt.Sprintf("RECENT COMPLETIONS (%d)", len(m.recentWork))))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", min(60, m.width-4)))
	b.WriteString("\n")

	if len(m.recentWork) == 0 {
		b.WriteString(labelStyle.Render("  No recent work"))
		b.WriteString("\n")
	} else {
		for _, work := range m.recentWork {
			icon := "✓"
			style := completedStyle
			if work.Status == StatusFailed || work.Status == StatusEscalated {
				icon = "✗"
				style = failedStyle
			}

			ago := time.Since(work.CompletedAt).Round(time.Second)
			duration := time.Duration(work.DurationMs * float64(time.Millisecond)).Round(time.Millisecond)

			line := fmt.Sprintf("  %s %s  %s  %s  %s ago",
				icon,
				truncate(work.WorkID, 12),
				truncate(work.AgentName, 15),
				duration,
				ago)
			b.WriteString(style.Render(line))
			b.WriteString("\n")

			if work.Error != "" {
				errLine := fmt.Sprintf("    └─ Error: %s", truncate(work.Error, 45))
				b.WriteString(failedStyle.Render(errLine))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(labelStyle.Render("[q] quit  [r] refresh"))

	return b.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
