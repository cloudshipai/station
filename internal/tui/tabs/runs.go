package tabs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/tui/components"
	"station/internal/tui/styles"
)

// RunsModel represents the agent runs history tab
type RunsModel struct {
	BaseTabModel
	
	// UI components
	table table.Model
	
	// State
	runs         []AgentRunDisplay
	selectedRun  *AgentRunDisplay
	viewMode     RunViewMode
}

type RunViewMode int

const (
	RunModeList RunViewMode = iota
	RunModeDetails
)

type AgentRunDisplay struct {
	ID          string
	AgentName   string
	Status      string
	StartedAt   string
	Duration    string
	User        string
	Error       string
	Output      string
	StepsCount  int
}

// Custom key bindings for runs
type runsKeyMap struct {
	showDetails key.Binding
	refresh     key.Binding
	cancel      key.Binding
}

func newRunsKeyMap() runsKeyMap {
	return runsKeyMap{
		showDetails: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "view details"),
		),
		refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		cancel: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "cancel/stop"),
		),
	}
}

// Messages for async operations
type RunsLoadedMsg struct {
	Runs []AgentRunDisplay
}

type RunCancelledMsg struct {
	RunID string
}

// NewRunsModel creates a new runs model
func NewRunsModel(database db.Database) *RunsModel {
	// Create table with columns
	columns := []table.Column{
		{Title: "ID", Width: 6},
		{Title: "Agent", Width: 20},
		{Title: "Status", Width: 12},
		{Title: "Started", Width: 16},
		{Title: "Duration", Width: 10},
		{Title: "User", Width: 12},
	}
	
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
	)
	
	// Apply styling
	s := table.DefaultStyles()
	s.Header = styles.TableHeaderStyle
	s.Selected = styles.TableSelectedStyle
	t.SetStyles(s)
	
	return &RunsModel{
		BaseTabModel: NewBaseTabModel(database, "Runs"),
		table:        t,
	}
}

// Init initializes the runs tab
func (m RunsModel) Init() tea.Cmd {
	return m.loadRuns()
}

// Update handles messages
func (m *RunsModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 8)
		
	case tea.KeyMsg:
		switch m.viewMode {
		case RunModeList:
			return m.handleListKeys(msg)
		case RunModeDetails:
			return m.handleDetailsKeys(msg)
		}
		
	case RunsLoadedMsg:
		m.runs = msg.Runs
		m.updateTableRows()
		m.SetLoading(false)
		
	case RunCancelledMsg:
		// Update run status to cancelled
		for i, run := range m.runs {
			if run.ID == msg.RunID {
				m.runs[i].Status = "cancelled"
				break
			}
		}
		m.updateTableRows()
	}
	
	// Update table
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)
	
	return m, tea.Batch(cmds...)
}

// Handle key presses in list mode
func (m *RunsModel) handleListKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	keyMap := newRunsKeyMap()
	
	switch {
	case key.Matches(msg, keyMap.showDetails):
		if len(m.runs) > 0 {
			selectedIdx := m.table.Cursor()
			if selectedIdx < len(m.runs) {
				m.selectedRun = &m.runs[selectedIdx]
				m.viewMode = RunModeDetails
			}
		}
		return m, nil
		
	case key.Matches(msg, keyMap.refresh):
		return m, m.RefreshData()
		
	case key.Matches(msg, keyMap.cancel):
		if len(m.runs) > 0 {
			selectedIdx := m.table.Cursor()
			if selectedIdx < len(m.runs) {
				run := m.runs[selectedIdx]
				if run.Status == "running" {
					return m, m.cancelRun(run.ID)
				}
			}
		}
		return m, nil
	}
	
	return m, nil
}

// Handle key presses in details mode
func (m *RunsModel) handleDetailsKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		m.viewMode = RunModeList
		m.selectedRun = nil
		return m, nil
	}
	return m, nil
}

// View renders the runs tab
func (m RunsModel) View() string {
	if m.IsLoading() {
		return components.RenderLoadingIndicator(0)
	}
	
	switch m.viewMode {
	case RunModeDetails:
		return m.renderRunDetails()
	default:
		return m.renderRunsList()
	}
}

// Render runs list view
func (m RunsModel) renderRunsList() string {
	header := components.RenderSectionHeader(fmt.Sprintf("Agent Execution History (%d runs)", len(m.runs)))
	tableView := m.table.View()
	helpText := styles.HelpStyle.Render("• ↑/↓: navigate • enter: view details • r: refresh • d: cancel running")
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tableView,
		"",
		helpText,
	)
}

// Render run details view
func (m RunsModel) renderRunDetails() string {
	if m.selectedRun == nil {
		return styles.ErrorStyle.Render("No run selected")
	}
	
	run := m.selectedRun
	var sections []string
	
	// Header
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Run Details: "),
		primaryStyle.Render(run.ID),
	)
	sections = append(sections, header)
	sections = append(sections, "")
	
	// Basic info
	info := m.renderRunInfo(run)
	sections = append(sections, info)
	
	// Output preview (if available)
	if run.Output != "" {
		output := m.renderRunOutput(run)
		sections = append(sections, output)
	}
	
	// Error info (if failed)
	if run.Status == "failed" && run.Error != "" {
		errorInfo := m.renderRunError(run)
		sections = append(sections, errorInfo)
	}
	
	// Back instruction
	backText := styles.HelpStyle.Render("Press enter or esc to go back to list")
	sections = append(sections, "")
	sections = append(sections, backText)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Render run basic information
func (m RunsModel) renderRunInfo(run *AgentRunDisplay) string {
	fields := []string{
		fmt.Sprintf("Run ID: %s", run.ID),
		fmt.Sprintf("Agent: %s", run.AgentName),
		fmt.Sprintf("Status: %s", run.Status),
		fmt.Sprintf("Started: %s", run.StartedAt),
		fmt.Sprintf("Duration: %s", run.Duration),
		fmt.Sprintf("User: %s", run.User),
		fmt.Sprintf("Steps: %d", run.StepsCount),
	}
	
	content := strings.Join(fields, "\n")
	
	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Padding(1).
		Render(content)
}

// Render run output preview
func (m RunsModel) renderRunOutput(run *AgentRunDisplay) string {
	output := run.Output
	if len(output) > 300 {
		output = output[:300] + "..."
	}
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Output Preview:"),
		"",
		mutedStyle.Render(output),
	)
	
	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Height(10).
		Padding(1).
		Render(content)
}

// Render run error information
func (m RunsModel) renderRunError(run *AgentRunDisplay) string {
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render("Error Details:"),
		"",
		styles.ErrorStyle.Render(run.Error),
	)
	
	return styles.WithBorder(lipgloss.NewStyle()).
		Width(60).
		Height(8).
		Padding(1).
		Render(content)
}

// RefreshData reloads runs from database
func (m RunsModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadRuns()
}

// Load runs from database (stub implementation)
func (m RunsModel) loadRuns() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Load real runs from database
		// Mock data for now
		runs := []AgentRunDisplay{
			{
				ID:          "run-001",
				AgentName:   "Code Reviewer",
				Status:      "completed",
				StartedAt:   "2024-01-15 14:30",
				Duration:    "2m 15s",
				User:        "admin",
				Output:      "Successfully reviewed 15 files. Found 3 minor issues and 1 optimization opportunity.",
				StepsCount:  8,
			},
			{
				ID:          "run-002",
				AgentName:   "Data Analyzer",
				Status:      "running",
				StartedAt:   "2024-01-15 14:35",
				Duration:    "1m 45s",
				User:        "user1",
				Output:      "Processing dataset... Current progress: 67%",
				StepsCount:  5,
			},
			{
				ID:          "run-003",
				AgentName:   "Log Processor",
				Status:      "failed",
				StartedAt:   "2024-01-15 14:20",
				Duration:    "0m 30s",
				User:        "admin",
				Error:       "Connection timeout: Unable to connect to log source after 3 retries",
				StepsCount:  2,
			},
			{
				ID:          "run-004",
				AgentName:   "Security Scanner",
				Status:      "completed",
				StartedAt:   "2024-01-15 14:10",
				Duration:    "5m 22s",
				User:        "user2",
				Output:      "Security scan completed. No critical vulnerabilities found. 2 medium-risk issues detected.",
				StepsCount:  12,
			},
		}
		
		return RunsLoadedMsg{Runs: runs}
	})
}

// Update table rows from runs data
func (m *RunsModel) updateTableRows() {
	rows := make([]table.Row, len(m.runs))
	for i, run := range m.runs {
		rows[i] = table.Row{
			run.ID,
			run.AgentName,
			run.Status,
			run.StartedAt,
			run.Duration,
			run.User,
		}
	}
	m.table.SetRows(rows)
}

// Cancel/stop a running agent
func (m RunsModel) cancelRun(runID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually cancel the run
		// For now, just simulate cancellation
		return RunCancelledMsg{RunID: runID}
	})
}

// IsMainView returns true if in main list view
func (m RunsModel) IsMainView() bool {
	// Use the base implementation for reliable navigation
	return m.BaseTabModel.IsMainView()
}