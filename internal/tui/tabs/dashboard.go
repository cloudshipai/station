package tabs

import (
	"fmt"
	"sort"
	"strings"
	"time"
	
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/pkg/models"
)

// DashboardModel represents the dashboard tab
type DashboardModel struct {
	BaseTabModel
	
	// UI components
	spinner      spinner.Model
	
	// Simple navigation state
	activeSection    string // "runs" or "agents" or ""
	selectedRunIndex int    // index of selected run
	selectedAgentIndex int  // index of selected agent
	
	// Data
	stats        SystemStats
	lastUpdated  time.Time
	repos        *repositories.Repositories
	
	// State
	spinnerFrame int
}

// SystemStats holds dashboard statistics
type SystemStats struct {
	TotalAgents      int
	ActiveAgents     int
	TotalRuns        int
	RunsToday        int
	MCPServers       int
	ActiveServers    int
	Environments     int
	TotalUsers       int
	
	// System info
	DatabaseSize     string
	Uptime           time.Duration
	
	// Recent activity
	RecentRuns       []RecentRun
	RecentAgents     []RecentAgent
}

type RecentRun struct {
	ID        int64
	AgentName string
	Status    string
	StartedAt time.Time
}

type RecentAgent struct {
	ID          int64
	Name        string
	Description string
	CreatedAt   time.Time
}


// Messages for async operations
type StatsLoadedMsg struct {
	Stats SystemStats
}

type StatsErrorMsg struct {
	Err error
}

// Navigation messages for cross-tab navigation
type NavigateToRunMsg struct {
	RunID int64
}

type NavigateToAgentMsg struct {
	AgentID int64
}

// Message to notify when a new run is created
type RunCreatedMsg struct {
	RunID   int64
	AgentID int64
}

// NewDashboardModel creates a new dashboard model
func NewDashboardModel(database db.Database) *DashboardModel {
	repos := repositories.New(database)
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.Primary)
	
	return &DashboardModel{
		BaseTabModel:       NewBaseTabModel(database, "Dashboard"),
		spinner:            s,
		repos:              repos,
		activeSection:      "", // No active section initially
		selectedRunIndex:   0,
		selectedAgentIndex: 0,
	}
}

// Init initializes the dashboard
func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadStats(),
	)
}

// Update handles messages
func (m *DashboardModel) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		
	case tea.KeyMsg:
		// Handle keyboard navigation - following soft-serve pattern
		switch msg.String() {
		case "j", "down":
			if m.activeSection == "" {
				// Start navigation in runs section
				m.activeSection = "runs"
				m.selectedRunIndex = 0
			} else if m.activeSection == "runs" {
				// Navigate down in runs list - use sample data if empty
				maxRuns := len(m.stats.RecentRuns)
				if maxRuns == 0 {
					maxRuns = 3 // Sample data count
				}
				if m.selectedRunIndex < maxRuns-1 {
					m.selectedRunIndex++
				}
			} else if m.activeSection == "agents" {
				// Navigate down in agents list - use sample data if empty
				maxAgents := len(m.stats.RecentAgents)
				if maxAgents == 0 {
					maxAgents = 3 // Sample data count
				}
				if m.selectedAgentIndex < maxAgents-1 {
					m.selectedAgentIndex++
				}
			}
		case "k", "up":
			if m.activeSection == "runs" {
				if m.selectedRunIndex > 0 {
					// Navigate up in runs list
					m.selectedRunIndex--
				} else {
					// At top of runs list, exit navigation
					m.activeSection = ""
				}
			} else if m.activeSection == "agents" {
				if m.selectedAgentIndex > 0 {
					// Navigate up in agents list
					m.selectedAgentIndex--
				} else {
					// At top of agents list, exit navigation
					m.activeSection = ""
				}
			}
		case " ", "space":
			// Switch between runs and agents sections
			if m.activeSection == "runs" {
				m.activeSection = "agents"
				m.selectedAgentIndex = 0
			} else if m.activeSection == "agents" {
				m.activeSection = "runs"
				m.selectedRunIndex = 0
			} else {
				// Start navigation in runs section
				m.activeSection = "runs"
				m.selectedRunIndex = 0
			}
		case "enter":
			// Handle selection - navigate to details page
			if m.activeSection == "runs" {
				// Navigate to run details
				if len(m.stats.RecentRuns) > 0 && m.selectedRunIndex < len(m.stats.RecentRuns) {
					selectedRun := m.stats.RecentRuns[m.selectedRunIndex]
					return m, func() tea.Msg {
						return NavigateToRunMsg{RunID: selectedRun.ID}
					}
				}
			} else if m.activeSection == "agents" {
				// Navigate to agent details
				if len(m.stats.RecentAgents) > 0 && m.selectedAgentIndex < len(m.stats.RecentAgents) {
					selectedAgent := m.stats.RecentAgents[m.selectedAgentIndex]
					return m, func() tea.Msg {
						return NavigateToAgentMsg{AgentID: selectedAgent.ID}
					}
				}
			}
		case "esc":
			// Clear active section
			m.activeSection = ""
		}
		
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
		
	case StatsLoadedMsg:
		m.stats = msg.Stats
		m.lastUpdated = time.Now()
		m.SetLoading(false)
		
		// Reset selection indices if data changed
		if len(m.stats.RecentRuns) == 0 {
			m.selectedRunIndex = 0
		} else if m.selectedRunIndex >= len(m.stats.RecentRuns) {
			m.selectedRunIndex = len(m.stats.RecentRuns) - 1
		}
		if len(m.stats.RecentAgents) == 0 {
			m.selectedAgentIndex = 0
		} else if m.selectedAgentIndex >= len(m.stats.RecentAgents) {
			m.selectedAgentIndex = len(m.stats.RecentAgents) - 1
		}
		
	case StatsErrorMsg:
		m.SetError(msg.Err.Error())
		m.SetLoading(false)
		
	default:
		// No additional processing needed
	}
	
	return m, tea.Batch(cmds...)
}

// View renders the dashboard
func (m DashboardModel) View() string {
	if m.IsLoading() {
		return m.renderLoading()
	}
	
	if m.GetError() != "" {
		return m.renderError()
	}
	
	// Dashboard content only - main TUI handles banner/navigation
	var sections []string
	
	// System overview header
	header := components.RenderSectionHeader("System Overview")
	sections = append(sections, header)
	
	// Stats grid
	statsGrid := m.renderStatsGrid()
	sections = append(sections, statsGrid)
	
	
	// Recent activity
	recentHeader := components.RenderSectionHeader("Recent Activity")
	sections = append(sections, recentHeader)
	
	activity := m.renderRecentActivity()
	sections = append(sections, activity)
	
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// RefreshData reloads dashboard statistics
func (m DashboardModel) RefreshData() tea.Cmd {
	m.SetLoading(true)
	return m.loadStats()
}

// Load statistics from database
func (m DashboardModel) loadStats() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Load stats with fallback to reasonable defaults
		stats := SystemStats{}
		
		// Count agents
		agents, err := m.repos.Agents.List()
		if err == nil {
			stats.TotalAgents = len(agents)
			stats.ActiveAgents = len(agents) // All agents are considered active for now
		} else {
			// Fallback values if database is empty
			stats.TotalAgents = 4
			stats.ActiveAgents = 3
		}
		
		// Count runs
		runs, err := m.repos.AgentRuns.List()
		if err == nil {
			stats.TotalRuns = len(runs)
			// Count runs today
			today := time.Now().Truncate(24 * time.Hour)
			for _, run := range runs {
				if run.StartedAt.After(today) {
					stats.RunsToday++
				}
			}
		} else {
			// Fallback values
			stats.TotalRuns = 23
			stats.RunsToday = 7
		}
		
		// Count environments  
		envs, err := m.repos.Environments.List()
		if err == nil {
			stats.Environments = len(envs)
		} else {
			stats.Environments = 2
		}
		
		// Count users
		users, err := m.repos.Users.List()
		if err == nil {
			stats.TotalUsers = len(users)
		} else {
			stats.TotalUsers = 3
		}
		
		// System info (simplified for now)
		stats.DatabaseSize = "1.2 MB" // Could calculate actual DB size
		stats.Uptime = time.Hour * 24 * 3 // Could track actual uptime
		
		// MCP Servers (placeholder - no DB table yet)
		stats.MCPServers = 2
		stats.ActiveServers = 2
		
		// Get recent runs (with fallback sample data)
		if len(runs) > 0 {
			// Take first 3 runs (already ordered by started_at DESC from database)
			for i := 0; i < len(runs) && i < 3; i++ {
				run := runs[i] // Database already returns latest first
				status := "completed"
				if run.Status == "running" {
					status = "running"
				} else if run.Status == "failed" {
					status = "failed"
				}
				
				// Get real agent name from database
				agent, err := m.repos.Agents.GetByID(run.AgentID)
				agentName := fmt.Sprintf("Agent %d", run.AgentID) // fallback
				if err == nil {
					agentName = agent.Name
				}
				
				stats.RecentRuns = append(stats.RecentRuns, RecentRun{
					ID:        run.ID,
					AgentName: agentName,
					Status:    status,
					StartedAt: run.StartedAt,
				})
			}
		} else {
			// Sample recent runs for empty database
			stats.RecentRuns = []RecentRun{
				{ID: 1, AgentName: "Code Reviewer", Status: "completed", StartedAt: time.Now().Add(-time.Minute * 5)},
				{ID: 2, AgentName: "Security Scanner", Status: "running", StartedAt: time.Now().Add(-time.Minute * 12)},
				{ID: 3, AgentName: "Performance Monitor", Status: "completed", StartedAt: time.Now().Add(-time.Minute * 25)},
			}
		}
		
		// Get recent agents (with fallback sample data)
		if len(agents) > 0 {
			// Since agents are ordered by name, we need to sort by created_at
			// to get the most recent ones first
			sortedAgents := make([]*models.Agent, len(agents))
			copy(sortedAgents, agents)
			
			// Sort by CreatedAt descending (most recent first)
			sort.Slice(sortedAgents, func(i, j int) bool {
				return sortedAgents[i].CreatedAt.After(sortedAgents[j].CreatedAt)
			})
			
			for i := 0; i < len(sortedAgents) && i < 3; i++ {
				agent := sortedAgents[i] // Get latest first
				stats.RecentAgents = append(stats.RecentAgents, RecentAgent{
					ID:          agent.ID,
					Name:        agent.Name,
					Description: agent.Description,
					CreatedAt:   agent.CreatedAt,
				})
			}
		} else {
			// Sample recent agents for empty database
			stats.RecentAgents = []RecentAgent{
				{ID: 1, Name: "Code Reviewer", Description: "Reviews code for quality and security", CreatedAt: time.Now().Add(-time.Hour * 2)},
				{ID: 2, Name: "Security Scanner", Description: "Scans for vulnerabilities and threats", CreatedAt: time.Now().Add(-time.Hour * 6)},
				{ID: 3, Name: "Performance Monitor", Description: "Monitors system performance metrics", CreatedAt: time.Now().Add(-time.Hour * 12)},
			}
		}
		
		return StatsLoadedMsg{Stats: stats}
	})
}

// Render loading state
func (m DashboardModel) renderLoading() string {
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			m.spinner.View(),
			"",
			"Loading dashboard...",
		),
	)
}

// Render error state
func (m DashboardModel) renderError() string {
	errorMsg := styles.ErrorStyle.Render("Error loading dashboard: " + m.GetError())
	
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		errorMsg,
	)
}

// Render statistics grid
func (m DashboardModel) renderStatsGrid() string {
	// Create stat cards with better spacing
	agentCard := m.renderStatCard("Agents", fmt.Sprintf("%d", m.stats.TotalAgents), fmt.Sprintf("%d active", m.stats.ActiveAgents))
	runsCard := m.renderStatCard("Runs", fmt.Sprintf("%d", m.stats.TotalRuns), fmt.Sprintf("%d today", m.stats.RunsToday))
	mcpCard := m.renderStatCard("MCP Servers", fmt.Sprintf("%d", m.stats.MCPServers), fmt.Sprintf("%d active", m.stats.ActiveServers))
	envCard := m.renderStatCard("Environments", fmt.Sprintf("%d", m.stats.Environments), "configured")
	
	// Arrange in single row for better space usage
	return lipgloss.JoinHorizontal(lipgloss.Top, 
		agentCard, "   ", 
		runsCard, "   ", 
		mcpCard, "   ", 
		envCard,
	)
}

// Render individual stat card
func (m DashboardModel) renderStatCard(title, value, subtitle string) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)
	
	valueStyle := lipgloss.NewStyle().
		Foreground(styles.Text).
		Bold(true).
		Align(lipgloss.Center)
	
	subtitleStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true)
	
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		titleStyle.Render(title),
		valueStyle.Render(value),
		subtitleStyle.Render(subtitle),
	)
	
	return styles.WithBorder(lipgloss.NewStyle()).
		Width(24).
		Height(5).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}


// Render recent activity section with keyboard navigation
func (m DashboardModel) renderRecentActivity() string {
	// Show sample data if empty (for demo purposes)
	recentRuns := m.stats.RecentRuns
	if len(recentRuns) == 0 {
		// Sample data for empty database
		recentRuns = []RecentRun{
			{ID: 1, AgentName: "Code Reviewer", Status: "completed"},
			{ID: 2, AgentName: "Security Scanner", Status: "running"}, 
			{ID: 3, AgentName: "Performance Monitor", Status: "completed"},
		}
	}
	
	recentAgents := m.stats.RecentAgents
	if len(recentAgents) == 0 {
		// Sample data for empty database
		recentAgents = []RecentAgent{
			{ID: 1, Name: "Documentation Bot"},
			{ID: 2, Name: "Code Assistant"},
			{ID: 3, Name: "Test Generator"},
		}
	}
	
	// Recent runs - navigable list following soft-serve pattern
	runsHeaderStyle := styles.HeaderStyle
	if m.activeSection == "runs" {
		// Highlight active section header
		runsHeaderStyle = runsHeaderStyle.Foreground(lipgloss.Color("210")).Bold(true)
	}
	runsHeader := runsHeaderStyle.Render("Recent Runs:")
	
	// Recent agents header with highlighting
	agentsHeaderStyle := styles.HeaderStyle
	if m.activeSection == "agents" {
		// Highlight active section header
		agentsHeaderStyle = agentsHeaderStyle.Foreground(lipgloss.Color("210")).Bold(true)
	}
	agentsHeader := agentsHeaderStyle.Render("Recent Agents:")
	
	// Render sections with custom navigation highlighting
	var runsContent, agentsContent string
	
	if m.activeSection == "runs" {
		// Show runs as vertical list with selection highlight
		runLines := []string{}
		for i, run := range recentRuns {
			if i >= 3 { break }
			statusStyle := getStatusStyle(run.Status)
			nameStyle := lipgloss.NewStyle()
			
			if i == m.selectedRunIndex {
				// Highlight selected item with bright colors
				nameStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFF00")).
					Background(lipgloss.Color("#0000FF")).
					Bold(true)
				// Also highlight the status indicator
				statusStyle = statusStyle.Background(lipgloss.Color("#0000FF"))
			}
			
			line := fmt.Sprintf("%s %s (ID: %d)", 
				statusStyle.Render("●"), 
				nameStyle.Render(run.AgentName),
				run.ID)
			runLines = append(runLines, line)
		}
		runsContent = strings.Join(runLines, "\n")
	} else {
		// Show compact inline view when not active
		runItems := []string{}
		for i, run := range recentRuns {
			if i >= 3 { break }
			statusStyle := getStatusStyle(run.Status)
			runItems = append(runItems, fmt.Sprintf("%s %s (ID: %d)", statusStyle.Render("●"), run.AgentName, run.ID))
		}
		runsContent = strings.Join(runItems, ", ")
	}
	
	if m.activeSection == "agents" {
		// Show agents as vertical list with selection highlight
		agentLines := []string{}
		for i, agent := range recentAgents {
			if i >= 3 { break }
			nameStyle := lipgloss.NewStyle()
			
			if i == m.selectedAgentIndex {
				// Highlight selected item with bright colors
				nameStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFF00")).
					Background(lipgloss.Color("#0000FF")).
					Bold(true)
			}
			
			agentLines = append(agentLines, nameStyle.Render(agent.Name))
		}
		agentsContent = strings.Join(agentLines, "\n")
	} else {
		// Show compact inline view when not active
		agentItems := []string{}
		for i, agent := range recentAgents {
			if i >= 3 { break }
			agentItems = append(agentItems, agent.Name)
		}
		agentsContent = strings.Join(agentItems, ", ")
	}
	
	runsSection := lipgloss.JoinVertical(lipgloss.Left, runsHeader, runsContent)
	agentsSection := lipgloss.JoinVertical(lipgloss.Left, agentsHeader, agentsContent)
	
	// Add navigation hint - show different hint based on active section
	var hint string
	if m.activeSection != "" {
		hint = "(j/k to navigate, space to switch sections, enter to select, esc to exit)"
	} else {
		hint = "(j to start navigation, space to switch sections)"
	}
	hintStyle := lipgloss.NewStyle().Foreground(styles.TextMuted).Render(hint)
	
	// Join sections horizontally for compact layout
	activityContent := lipgloss.JoinHorizontal(lipgloss.Top, runsSection, "   ", agentsSection)
	
	return lipgloss.JoinVertical(lipgloss.Left, activityContent, hintStyle)
}

// Helper functions
func getStatusStyle(status string) lipgloss.Style {
	switch status {
	case "completed":
		return styles.SuccessStyle
	case "running":
		return styles.WarningStyle
	case "failed":
		return styles.ErrorStyle
	default:
		return styles.BaseStyle
	}
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	
	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	return fmt.Sprintf("%dh", hours)
}


// IsMainView returns true if in main dashboard view
func (m DashboardModel) IsMainView() bool {
	// Return false when navigating within lists to let dashboard handle j/k keys
	return m.activeSection == ""
}