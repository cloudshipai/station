package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	
	"station/internal/db"
	"station/internal/tui/components"
	"station/internal/tui/styles"
	"station/internal/tui/tabs"
)

// LayoutDimensions holds calculated layout dimensions
type LayoutDimensions struct {
	// Terminal dimensions
	TerminalWidth  int
	TerminalHeight int
	
	// Container dimensions (centered, max width)
	ContainerWidth int
	
	// Component heights (calculated dynamically)
	BannerHeight    int
	TabBarHeight    int
	StatusBarHeight int
	
	// Content area dimensions
	ContentWidth  int
	ContentHeight int
	
	// Layout decisions
	UseCompactBanner bool
}

// Main TUI model for Station admin interface
type Model struct {
	// Core data
	db db.Database
	
	// UI state
	width       int
	height      int
	activeTab   int
	tabs        []string
	
	// Tab models - each tab is its own bubbletea model
	dashboardModel tabs.TabModel
	agentsModel    tabs.TabModel
	runsModel      tabs.TabModel
	mcpModel       tabs.TabModel
	envModel       tabs.TabModel
	settingsModel  tabs.TabModel
	
	// UI components
	help     help.Model
	keyMap   KeyMap
	
	// State
	initialized bool
	loading     bool
	error       string
}

// KeyMap defines keyboard shortcuts
type KeyMap struct {
	NextTab     key.Binding
	PrevTab     key.Binding
	Quit        key.Binding
	Help        key.Binding
	Refresh     key.Binding
}

// Default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		NextTab: key.NewBinding(
			key.WithKeys("tab", "right", "l"),
			key.WithHelp("tab/right/l", "next tab"),
		),
		PrevTab: key.NewBinding(
			key.WithKeys("shift+tab", "left", "h"),
			key.WithHelp("shift+tab/left/h", "prev tab"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("ctrl+r", "refresh"),
		),
	}
}

// NewModel creates a new TUI model
func NewModel(database db.Database) *Model {
	
	return &Model{
		db:        database,
		tabs:      []string{"Dashboard", "Agents", "Runs", "MCP Servers", "Environments", "Settings"},
		activeTab: 0,
		keyMap:    DefaultKeyMap(),
		help:      help.New(),
		loading:   true,
	}
}

// Init initializes the TUI
func (m *Model) Init() tea.Cmd {
	
	return tea.Batch(
		m.initializeTabModels(),
		tea.EnterAltScreen,
	)
}

// Initialize all tab models
func (m *Model) initializeTabModels() tea.Cmd {
	var cmds []tea.Cmd
	
	// Initialize each tab model
	m.dashboardModel = tabs.NewDashboardModel(m.db)
	m.agentsModel = tabs.NewAgentsModel(m.db)
	m.runsModel = tabs.NewRunsModel(m.db)
	m.mcpModel = tabs.NewMCPModel(m.db)
	m.envModel = tabs.NewEnvironmentsModel(m.db)
	m.settingsModel = tabs.NewSettingsModel(m.db)
	
	// Get initialization commands from each model
	cmds = append(cmds, m.dashboardModel.Init())
	cmds = append(cmds, m.agentsModel.Init())
	cmds = append(cmds, m.runsModel.Init())
	cmds = append(cmds, m.mcpModel.Init())
	cmds = append(cmds, m.envModel.Init())
	cmds = append(cmds, m.settingsModel.Init())
	
	m.initialized = true
	m.loading = false
	
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		// Calculate dimensions using dynamic layout system
		layout := m.calculateLayout(msg.Width, msg.Height)
		
		// Update all tab models with calculated sizing
		m.dashboardModel.SetSize(layout.ContentWidth, layout.ContentHeight)
		m.agentsModel.SetSize(layout.ContentWidth, layout.ContentHeight)
		m.runsModel.SetSize(layout.ContentWidth, layout.ContentHeight)
		m.mcpModel.SetSize(layout.ContentWidth, layout.ContentHeight)
		m.envModel.SetSize(layout.ContentWidth, layout.ContentHeight)
		m.settingsModel.SetSize(layout.ContentWidth, layout.ContentHeight)
		
		// Forward the message to tab models
		m.dashboardModel, _ = m.dashboardModel.Update(msg)
		m.agentsModel, _ = m.agentsModel.Update(msg)
		m.runsModel, _ = m.runsModel.Update(msg)
		m.mcpModel, _ = m.mcpModel.Update(msg)
		m.envModel, _ = m.envModel.Update(msg)
		m.settingsModel, _ = m.settingsModel.Update(msg)
		
		return m, nil
		
	case tea.KeyMsg:
		
		// Handle global keys first - but check conditions before consuming keys
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			return m, tea.Quit
			
		case key.Matches(msg, m.keyMap.Refresh):
			return m, m.refreshActiveTab()
			
		case key.Matches(msg, m.keyMap.NextTab) && m.getActiveTabModel().IsMainView():
			// Only consume tab key if we're actually going to use it for tab navigation
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			return m, nil
			
		case key.Matches(msg, m.keyMap.PrevTab) && m.getActiveTabModel().IsMainView():
			// Only consume shift+tab key if we're actually going to use it for tab navigation
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			return m, nil
			
		default:
			
			// Forward unhandled keys to active tab
			var cmd tea.Cmd
			switch m.activeTab {
			case 0: // Dashboard
				m.dashboardModel, cmd = m.dashboardModel.Update(msg)
			case 1: // Agents
				m.agentsModel, cmd = m.agentsModel.Update(msg)
			case 2: // Runs
				m.runsModel, cmd = m.runsModel.Update(msg)
			case 3: // MCP Servers
				m.mcpModel, cmd = m.mcpModel.Update(msg)
			case 4: // Environments
				m.envModel, cmd = m.envModel.Update(msg)
			case 5: // Settings
				m.settingsModel, cmd = m.settingsModel.Update(msg)
			}
			return m, cmd
		}
	}
	
	return m, tea.Batch(cmds...)
}

// View renders the TUI - following soft-serve pattern: main model owns entire screen
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Waiting for terminal size..."
	}
	
	// Create banner with blue neon styling
	banner := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Background(lipgloss.Color("#000080")).
		Bold(true).
		Width(m.width).
		Align(lipgloss.Center).
		Render("STATION ◆◇◆ AI AGENT MANAGEMENT PLATFORM ◆◇◆")
	
	// Create tab navigation
	tabNames := []string{"Dashboard", "Agents", "Runs", "MCP Servers", "Environments", "Settings"}
	var tabItems []string
	for i, name := range tabNames {
		if i == m.activeTab {
			tabItems = append(tabItems, "● "+name)
		} else {
			tabItems = append(tabItems, "○ "+name)  
		}
	}
	
	navigation := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4169E1")).
		Width(m.width).
		Align(lipgloss.Center).
		Render(strings.Join(tabItems, "  "))
	
	// Separator line
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4169E1")).
		Render(strings.Repeat("─", m.width))
	
	// Status bar
	statusBar := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Width(m.width).
		Render("Press ? for help • Tab/Shift+Tab to navigate • q to quit")
	
	if m.loading {
		return lipgloss.JoinVertical(lipgloss.Left, banner, navigation, separator, "Loading...", statusBar)
	}
	
	if !m.initialized {
		return lipgloss.JoinVertical(lipgloss.Left, banner, navigation, separator, "Initializing...", statusBar)
	}
	
	// Get content from active tab 
	content := m.renderActiveTabContent()
	
	// CRITICAL: Calculate exact available height
	// Terminal height minus: banner(1) + navigation(1) + separator(1) + status(1) + border(2) + padding(2) = 8
	contentHeight := m.height - 8
	if contentHeight < 3 {
		contentHeight = 3 // absolute minimum
	}
	
	// Truncate content if it's too long to fit
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentHeight {
		contentLines = contentLines[:contentHeight]
		content = strings.Join(contentLines, "\n")
	}
	
	// Style content with EXACT height constraint
	styledContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#4169E1")).
		Padding(1, 2).
		Width(m.width - 4).
		Height(contentHeight).
		Render(content)
	
	// Join all sections
	return lipgloss.JoinVertical(lipgloss.Left,
		banner,
		navigation,
		separator, 
		styledContent,
		statusBar,
	)
}

// calculateLayout determines the optimal layout based on terminal size
func (m Model) calculateLayout(width, height int) LayoutDimensions {
	layout := LayoutDimensions{
		TerminalWidth:  width,
		TerminalHeight: height,
	}
	
	// Simple approach: use full width, calculate content area precisely
	layout.ContainerWidth = width
	layout.ContentWidth = width - 4 // Account for content container borders and padding
	
	// Define component heights precisely
	layout.BannerHeight = 1    // Single line banner
	layout.TabBarHeight = 3    // Tab buttons + separator line  
	layout.StatusBarHeight = 1 // Status bar at bottom
	
	// Use Zen's recommended pattern: subtract fixed sections from total height
	// Account for content container padding (2) + border (2) = 4 total overhead
	contentStylingOverhead := 4
	layout.ContentHeight = height - layout.BannerHeight - layout.TabBarHeight - layout.StatusBarHeight - contentStylingOverhead
	
	// Ensure we don't go negative
	if layout.ContentHeight < 5 {
		layout.ContentHeight = 5
	}
	
	layout.UseCompactBanner = true
	
	return layout
}

// Helper function for min (Go 1.21+)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Render banner with STATION logo
func (m Model) renderBanner() string {
	banner := components.RenderBanner()
	
	// Add padding and center
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Margin(1, 0).
		Render(banner)
}

// Render medium banner for medium-sized terminals
func (m Model) renderMediumBanner() string {
	// Simplified ASCII art
	bannerLines := []string{
		"███████╗████████╗ █████╗ ████████╗██╗ ██████╗ ███╗   ██╗",
		"███████║   ██║   ███████║   ██║   ██║██║   ██║██╔██╗ ██║",
		"╚══════╝   ╚═╝   ╚═╝  ╚═╝   ╚═╝   ╚═╝ ╚═════╝ ╚═╝  ╚═══╝",
	}
	
	// Color the ASCII art
	var coloredLines []string
	colors := []lipgloss.Color{
		lipgloss.Color("#4169E1"), // Royal blue
		lipgloss.Color("#00BFFF"), // Deep sky blue  
		lipgloss.Color("#87CEEB"), // Sky blue
	}
	
	for i, line := range bannerLines {
		style := lipgloss.NewStyle().
			Foreground(colors[i]).
			Bold(true)
		coloredLines = append(coloredLines, style.Render(line))
	}
	
	banner := strings.Join(coloredLines, "\n")
	
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Margin(0, 0, 1, 0).
		Render(banner)
}

// Render compact banner that fits with navigation
func (m Model) renderCompactBanner() string {
	// Retro single-line STATION text with neon styling
	stationText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Background(lipgloss.Color("#000080")).
		Bold(true).
		Render(" STATION ")
	
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF1493")).
		Bold(true).
		Render("◆◇◆ AI AGENT MANAGEMENT PLATFORM ◆◇◆")
	
	// Combine in a horizontal layout with retro spacing
	compactBanner := lipgloss.JoinHorizontal(
		lipgloss.Left,
		stationText,
		lipgloss.NewStyle().Render(" "),
		subtitle,
	)
	
	// Center with no margin to save space
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(compactBanner)
}

// Render minimal banner for very small terminals
func (m Model) renderMinimalBanner() string {
	// Just "STATION" text
	stationText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4169E1")).
		Bold(true).
		Render("STATION")
	
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(stationText)
}

// Render tab navigation bar
func (m Model) renderTabs() string {
	var renderedTabs []string
	
	for i, tabName := range m.tabs {
		var style lipgloss.Style
		if i == m.activeTab {
			style = styles.ActiveTab
		} else {
			style = styles.InactiveTab
		}
		
		// Add tab indicators
		tabContent := tabName
		if i == m.activeTab {
			tabContent = "● " + tabName
		} else {
			tabContent = "○ " + tabName
		}
		
		renderedTabs = append(renderedTabs, style.Render(tabContent))
	}
	
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	
	// Center the tab bar
	centeredTabBar := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(tabBar)
	
	// Add separator line
	separator := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Render(strings.Repeat("─", m.width))
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		centeredTabBar,
		separator,
	)
}

// Render active tab content - just the content, no styling wrapper
func (m Model) renderActiveTabContent() string {
	switch m.activeTab {
	case 0: // Dashboard
		return m.dashboardModel.View()
	case 1: // Agents
		return m.agentsModel.View()
	case 2: // Runs
		return m.runsModel.View()
	case 3: // MCP Servers
		return m.mcpModel.View()
	case 4: // Environments
		return m.envModel.View()
	case 5: // Settings
		return m.settingsModel.View()
	default:
		return "Invalid tab selected"
	}
}


// Render status bar
func (m Model) renderStatusBar() string {
	// Left side: help text
	helpText := m.help.ShortHelpView([]key.Binding{
		m.keyMap.NextTab,
		m.keyMap.PrevTab,
		m.keyMap.Refresh,
		m.keyMap.Help,
		m.keyMap.Quit,
	})
	
	// Right side: branding
	branding := components.RenderBranding()
	
	// Calculate spacing
	helpWidth := lipgloss.Width(helpText)
	brandingWidth := lipgloss.Width(branding)
	spacerWidth := m.width - helpWidth - brandingWidth - 4
	if spacerWidth < 0 {
		spacerWidth = 0
	}
	spacer := strings.Repeat(" ", spacerWidth)
	
	statusContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		helpText,
		spacer,
		branding,
	)
	
	return styles.StatusBarStyle.
		Width(m.width).
		Render(statusContent)
}

// Render loading screen
func (m Model) renderLoading() string {
	loading := components.RenderLoadingIndicator(0)
	
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		loading,
	)
}

// Refresh active tab data
func (m Model) refreshActiveTab() tea.Cmd {
	switch m.activeTab {
	case 0:
		return m.dashboardModel.RefreshData()
	case 1:
		return m.agentsModel.RefreshData()
	case 2:
		return m.runsModel.RefreshData()
	case 3:
		return m.mcpModel.RefreshData()
	case 4:
		return m.envModel.RefreshData()
	case 5:
		return m.settingsModel.RefreshData()
	default:
		return nil
	}
}

// getActiveTabModel returns the currently active tab model
func (m Model) getActiveTabModel() tabs.TabModel {
	switch m.activeTab {
	case 0: // Dashboard
		return m.dashboardModel
	case 1: // Agents
		return m.agentsModel
	case 2: // Runs
		return m.runsModel
	case 3: // MCP Servers
		return m.mcpModel
	case 4: // Environments
		return m.envModel
	case 5: // Settings
		return m.settingsModel
	default:
		return m.dashboardModel
	}
}

// Program creates a new Bubble Tea program
func NewProgram(database db.Database) *tea.Program {
	model := NewModel(database)
	return tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
}