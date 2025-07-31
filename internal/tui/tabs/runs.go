package tabs

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// RunsModel represents the agent runs history tab
type RunsModel struct {
	BaseTabModel
	
	// UI components
	list     list.Model
	viewport viewport.Model
	
	// Data access
	repos *repositories.Repositories
	
	// State
	runs           []AgentRunDisplay
	selectedRun    *AgentRunDisplay
	fullRunData    *models.AgentRunWithDetails  // Full data with tool calls
	viewMode       RunViewMode
	showToolCalls  bool
	outputSelected bool
	expandedScrollOffset int  // Scroll position for expanded output view
}

type RunViewMode int

const (
	RunModeList RunViewMode = iota
	RunModeDetails
	RunModeExpandedOutput
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

// RunItem implements list.Item interface for bubbles list component
type RunItem struct {
	run AgentRunDisplay
}

// Required by list.Item interface
func (i RunItem) FilterValue() string {
	return strings.Join([]string{
		i.run.ID,
		i.run.AgentName,
		i.run.Status,
		i.run.User,
	}, " ")
}

// Required by list.DefaultItem interface
func (i RunItem) Title() string {
	// Status indicator
	statusStyle := styles.BaseStyle
	switch i.run.Status {
	case "completed":
		statusStyle = styles.SuccessStyle
	case "failed":
		statusStyle = styles.ErrorStyle
	case "running":
		statusStyle = styles.BaseStyle.Foreground(lipgloss.Color("11")) // Yellow
	case "cancelled":
		statusStyle = styles.BaseStyle.Foreground(styles.TextMuted)
	}
	
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusStyle.Render("● "),
		styles.BaseStyle.Render(fmt.Sprintf("%s - %s", i.run.ID, i.run.AgentName)),
	)
}

func (i RunItem) Description() string {
	user := i.run.User
	if len(user) > 12 {
		user = user[:12] + "..."
	}
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	return mutedStyle.Render(fmt.Sprintf("%s • %s • %s • %d steps", 
		i.run.Status, i.run.StartedAt, user, i.run.StepsCount))
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

type RunDataLoadedMsg struct {
	RunData *models.AgentRunWithDetails
}

// NewRunsModel creates a new runs model
func NewRunsModel(database db.Database) *RunsModel {
	repos := repositories.New(database)
	
	// Create list - styles will be set dynamically in WindowSizeMsg handler
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Agent Runs"
	l.Styles.Title = styles.HeaderStyle
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(styles.TextMuted)
	l.Styles.HelpStyle = styles.HelpStyle
	
	// Create viewport for expanded output
	vp := viewport.New(80, 20)
	vp.Style = styles.WithBorder(lipgloss.NewStyle()).Padding(1)
	
	return &RunsModel{
		BaseTabModel: NewBaseTabModel(database, "Runs"),
		list:         l,
		viewport:     vp,
		repos:        repos,
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
		// Update list size based on the content area dimensions calculated by TUI
		// Reserve space for section header and help text (approximately 4 lines)
		listWidth := msg.Width - 4
		m.list.SetSize(listWidth, msg.Height-4)
		
		// Update delegate styles to use full width for proper selection highlighting
		// Use the full available width to ensure highlighting spans the entire width
		delegate := list.NewDefaultDelegate()
		delegate.Styles.SelectedTitle = styles.GetListItemSelectedStyle(msg.Width)
		delegate.Styles.SelectedDesc = styles.GetListItemSelectedStyle(msg.Width)
		delegate.Styles.NormalTitle = styles.GetListItemStyle(msg.Width)
		delegate.Styles.NormalDesc = styles.GetListItemStyle(msg.Width)
		m.list.SetDelegate(delegate)
		
	case tea.KeyMsg:
		switch m.viewMode {
		case RunModeList:
			return m.handleListKeys(msg)
		case RunModeDetails:
			return m.handleDetailsKeys(msg)
		case RunModeExpandedOutput:
			// Handle all keys for expanded output mode
			switch msg.String() {
			case "esc":
				m.viewMode = RunModeDetails
				m.expandedScrollOffset = 0 // Reset scroll position
				return m, nil
			case "t":
				// Toggle tool calls display
				m.showToolCalls = !m.showToolCalls
				return m, nil
			case "j", "down":
				// Scroll down
				m.expandedScrollOffset++
				return m, nil
			case "k", "up":
				// Scroll up
				if m.expandedScrollOffset > 0 {
					m.expandedScrollOffset--
				}
				return m, nil
			case "g":
				// Go to top
				m.expandedScrollOffset = 0
				return m, nil
			case "G":
				// Go to bottom (will be adjusted in render function)
				m.expandedScrollOffset = 9999
				return m, nil
			case "ctrl+d":
				// Page down
				m.expandedScrollOffset += 10
				return m, nil
			case "ctrl+u":
				// Page up
				m.expandedScrollOffset -= 10
				if m.expandedScrollOffset < 0 {
					m.expandedScrollOffset = 0
				}
				return m, nil
			}
			return m, nil
		}
		
	case RunsLoadedMsg:
		m.runs = msg.Runs
		m.updateListItems()
		m.SetLoading(false)
		
		// Check if a specific run was pre-selected (e.g., from dashboard navigation)
		if selectedID := m.GetSelectedID(); selectedID != "" {
			for _, run := range m.runs {
				if run.ID == selectedID {
					m.selectedRun = &run
					m.SetViewMode("detail")
					m.SetSelectedID("") // Clear the selected ID after using it
					return m, m.loadFullRunData()
				}
			}
		}
		
	case RunCancelledMsg:
		// Update run status to cancelled
		for i, run := range m.runs {
			if run.ID == msg.RunID {
				m.runs[i].Status = "cancelled"
				break
			}
		}
		m.updateListItems()
		
	case RunDataLoadedMsg:
		// Handle loaded run data
		m.fullRunData = msg.RunData
		// Update viewport content when entering expanded mode
		if m.viewMode == RunModeExpandedOutput {
			m.updateViewportContent()
		}
		
	case RunCreatedMsg:
		// A new run was created, refresh the runs list
		return m, m.loadRuns()
	}
	
	// Update components based on current mode
	var cmd tea.Cmd
	if m.viewMode != RunModeExpandedOutput {
		// Update list when not in expanded output mode (expanded uses manual scrolling)
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}
	
	return m, tea.Batch(cmds...)
}


// Handle key presses in list mode
func (m *RunsModel) handleListKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	keyMap := newRunsKeyMap()
	
	switch {
	case key.Matches(msg, keyMap.showDetails):
		if len(m.runs) > 0 {
			if item, ok := m.list.SelectedItem().(RunItem); ok {
				m.selectedRun = &item.run
				m.viewMode = RunModeDetails
			}
		}
		return m, nil
		
	case key.Matches(msg, keyMap.refresh):
		return m, m.RefreshData()
		
	case key.Matches(msg, keyMap.cancel):
		if len(m.runs) > 0 {
			if item, ok := m.list.SelectedItem().(RunItem); ok {
				if item.run.Status == "running" {
					return m, m.cancelRun(item.run.ID)
				}
			}
		}
		return m, nil
		
	// Add vim-style navigation keys - let list handle them
	case msg.String() == "j" || msg.String() == "down":
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
		
	case msg.String() == "k" || msg.String() == "up":
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
	
	return m, nil
}

// Handle key presses in details mode
func (m *RunsModel) handleDetailsKeys(msg tea.KeyMsg) (TabModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewMode = RunModeList
		m.selectedRun = nil
		m.outputSelected = false
		return m, nil
	case "enter":
		if m.outputSelected {
			// Enter expanded output view
			m.viewMode = RunModeExpandedOutput
			m.expandedScrollOffset = 0  // Reset scroll position
			return m, m.loadFullRunData()
		} else {
			// Go back to list
			m.viewMode = RunModeList
			m.selectedRun = nil
			m.outputSelected = false
			return m, nil
		}
	case "tab":
		// Toggle output selection
		m.outputSelected = !m.outputSelected
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
	case RunModeExpandedOutput:
		return m.renderExpandedOutput()
	default:
		return m.renderRunsList()
	}
}

// Render runs list view
func (m RunsModel) renderRunsList() string {
	header := components.RenderSectionHeader(fmt.Sprintf("Agent Execution History (%d runs)", len(m.runs)))
	listView := m.list.View()
	helpText := styles.HelpStyle.Render("• ↑/↓: navigate • enter: view details • r: refresh • d: cancel running")
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		listView,
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
	var helpText string
	if m.outputSelected {
		helpText = "• enter: expand output • tab: deselect • esc: back to list"
	} else {
		helpText = "• tab: select output • enter/esc: back to list"
	}
	backText := styles.HelpStyle.Render(helpText)
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
	// Create header with selection indicator
	headerText := "Output Preview:"
	if m.outputSelected {
		headerText = "► Output Preview: (press enter to expand)"
	} else {
		headerText = "Output Preview: (press tab to select)"
	}
	
	// Calculate available lines for output content within the box
	// Box height is 10, minus padding (2), border (2), header (1), spacing (1) = 4 lines for content
	availableLines := 4
	
	// Split output into lines and truncate to fit
	output := run.Output
	lines := strings.Split(output, "\n")
	
	if len(lines) > availableLines {
		lines = lines[:availableLines]
		// Add truncation indicator on the last line
		if len(lines) > 0 {
			lines[len(lines)-1] += "..."
		}
	}
	
	truncatedOutput := strings.Join(lines, "\n")
	
	mutedStyle := lipgloss.NewStyle().Foreground(styles.TextMuted)
	
	// Apply selection styling
	outputStyle := mutedStyle
	if m.outputSelected {
		outputStyle = mutedStyle.Background(lipgloss.Color("235"))
	}
	
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		styles.HeaderStyle.Render(headerText),
		"",
		outputStyle.Render(truncatedOutput),
	)
	
	borderStyle := styles.WithBorder(lipgloss.NewStyle())
	if m.outputSelected {
		borderStyle = borderStyle.BorderForeground(styles.Primary)
	}
	
	return borderStyle.
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

// Load runs from database
func (m RunsModel) loadRuns() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Load recent runs from database with agent and user details
		dbRuns, err := m.repos.AgentRuns.ListRecent(50) // Get last 50 runs
		if err != nil {
			// Return empty list on error (could also return error message)
			return RunsLoadedMsg{Runs: []AgentRunDisplay{}}
		}
		
		// Convert database models to display models
		runs := make([]AgentRunDisplay, len(dbRuns))
		for i, dbRun := range dbRuns {
			runs[i] = AgentRunDisplay{
				ID:          fmt.Sprintf("run-%d", dbRun.ID),
				AgentName:   dbRun.AgentName,
				Status:      dbRun.Status,
				StartedAt:   formatTime(dbRun.StartedAt),
				Duration:    calculateDuration(dbRun.StartedAt, dbRun.CompletedAt),
				User:        dbRun.Username,
				Output:      dbRun.FinalResponse,
				StepsCount:  int(dbRun.StepsTaken),
				Error:       "", // Could extract from output if status is failed
			}
		}
		
		return RunsLoadedMsg{Runs: runs}
	})
}

// formatTime formats a time.Time for display
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "Unknown"
	}
	return t.Format("2006-01-02 15:04")
}

// calculateDuration calculates the duration between start and end times
func calculateDuration(startedAt time.Time, completedAt *time.Time) string {
	if startedAt.IsZero() {
		return "Unknown"
	}
	
	endTime := time.Now()
	if completedAt != nil && !completedAt.IsZero() {
		endTime = *completedAt
	}
	
	duration := endTime.Sub(startedAt)
	
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	} else if duration < time.Hour {
		return fmt.Sprintf("%dm %ds", int(duration.Minutes()), int(duration.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
	}
}

// Update list items from runs data
func (m *RunsModel) updateListItems() {
	items := make([]list.Item, len(m.runs))
	for i, run := range m.runs {
		items[i] = RunItem{run: run}
	}
	// Debug: log list items count
	log.Printf("DEBUG: Setting %d items in runs list", len(items))
	m.list.SetItems(items)
}

// Cancel/stop a running agent
func (m RunsModel) cancelRun(runID string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// TODO: Actually cancel the run
		// For now, just simulate cancellation
		return RunCancelledMsg{RunID: runID}
	})
}

// Load full run data with tool calls and execution steps
func (m *RunsModel) loadFullRunData() tea.Cmd {
	if m.selectedRun == nil {
		return nil
	}
	
	return tea.Cmd(func() tea.Msg {
		// Extract run ID from the display ID (e.g., "run-123" -> 123)
		var runID int64
		fmt.Sscanf(m.selectedRun.ID, "run-%d", &runID)
		
		// Get full run data from database
		run, err := m.repos.AgentRuns.GetByIDWithDetails(runID)
		if err != nil {
			return nil
		}
		
		return RunDataLoadedMsg{RunData: run}
	})
}

// updateViewportContent updates the viewport with current content
func (m *RunsModel) updateViewportContent() {
	if m.fullRunData == nil {
		m.viewport.SetContent("Loading run data...\n\nPlease wait while we fetch the complete run information from the database.")
		return
	}
	
	// Build full content without truncation
	var content strings.Builder
	
	content.WriteString("=== Final Response ===\n\n")
	if m.fullRunData.FinalResponse != "" {
		content.WriteString(m.fullRunData.FinalResponse)
	} else {
		content.WriteString("[No response content]")
	}
	
	// Add tool calls info if enabled
	if m.showToolCalls && m.fullRunData.ToolCalls != nil {
		content.WriteString("\n\n=== Tool Calls ===\n\n")
		// Parse tool calls JSON
		var toolCalls []models.ToolCall
		if m.fullRunData.ToolCalls != nil {
			for _, item := range *m.fullRunData.ToolCalls {
				if toolCallData, err := json.Marshal(item); err == nil {
					var toolCall models.ToolCall
					if json.Unmarshal(toolCallData, &toolCall) == nil {
						toolCalls = append(toolCalls, toolCall)
					}
				}
			}
		}
		
		for i, toolCall := range toolCalls {
			content.WriteString(fmt.Sprintf("%d. %s_%s\n", i+1, toolCall.ServerName, toolCall.ToolName))
			if toolCall.Error != "" {
				content.WriteString(fmt.Sprintf("   Error: %s\n", toolCall.Error))
			}
		}
	}
	
	m.viewport.SetContent(content.String())
}

// Render expanded output view using manual scrolling (viewport causes display issues)
func (m *RunsModel) renderExpandedOutput() string {
	if m.selectedRun == nil {
		return styles.ErrorStyle.Render("No run selected")
	}
	
	run := m.selectedRun
	
	// Build header
	primaryStyle := lipgloss.NewStyle().Foreground(styles.Primary).Bold(true)
	header := lipgloss.JoinHorizontal(
		lipgloss.Left,
		styles.HeaderStyle.Render("Full Output: "),
		primaryStyle.Render(run.ID),
	)
	
	// Build content
	var content strings.Builder
	content.WriteString("=== Final Response ===\n\n")
	if m.fullRunData != nil && m.fullRunData.FinalResponse != "" {
		content.WriteString(m.fullRunData.FinalResponse)
	} else {
		content.WriteString("Loading run data...\n\nPlease wait while we fetch the complete run information from the database.")
	}
	
	// Add tool calls info if enabled
	if m.showToolCalls && m.fullRunData != nil && m.fullRunData.ToolCalls != nil {
		content.WriteString("\n\n=== Tool Calls ===\n\n")
		// Parse tool calls JSON (same logic as before)
		var toolCalls []models.ToolCall
		if m.fullRunData.ToolCalls != nil {
			for _, item := range *m.fullRunData.ToolCalls {
				if toolCallData, err := json.Marshal(item); err == nil {
					var toolCall models.ToolCall
					if json.Unmarshal(toolCallData, &toolCall) == nil {
						toolCalls = append(toolCalls, toolCall)
					}
				}
			}
		}
		
		for i, toolCall := range toolCalls {
			content.WriteString(fmt.Sprintf("%d. %s_%s\n", i+1, toolCall.ServerName, toolCall.ToolName))
			if toolCall.Error != "" {
				content.WriteString(fmt.Sprintf("   Error: %s\n", toolCall.Error))
			}
		}
	}
	
	// Build help text
	var helpText string
	if m.showToolCalls {
		helpText = "• j/k/↑↓: scroll • t: hide tool calls • g/G: top/bottom • esc: back to details"
	} else {
		helpText = "• j/k/↑↓: scroll • t: show tool calls • g/G: top/bottom • esc: back to details"
	}
	footer := styles.HelpStyle.Render(helpText)
	
	// Simple vertical layout - manual scrolling implementation (following agents tab pattern)
	fullContent := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"", // spacing
		content.String(),
		"", // spacing  
		footer,
	)
	
	// Manual scrolling implementation
	lines := strings.Split(fullContent, "\n")
	
	// Get available height and calculate scrollable area
	maxHeight := m.height - 2 // Account for minimal padding
	if maxHeight < 5 {
		maxHeight = 5 // Minimum height
	}
	
	// Apply scroll offset
	startLine := m.expandedScrollOffset
	endLine := startLine + maxHeight
	
	if startLine < 0 {
		startLine = 0
		m.expandedScrollOffset = 0
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine >= len(lines) {
		startLine = len(lines) - maxHeight
		if startLine < 0 {
			startLine = 0
		}
		m.expandedScrollOffset = startLine
	}
	
	// Return scrolled content
	visibleLines := lines[startLine:endLine]
	return strings.Join(visibleLines, "\n")
}

// IsMainView returns true if in main list view
func (m RunsModel) IsMainView() bool {
	// Only return true if we're in the main list view
	// This prevents global tab navigation from consuming tab keys in detail views
	return m.viewMode == RunModeList
}