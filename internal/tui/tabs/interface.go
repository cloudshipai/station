package tabs

import (
	"strings"
	
	tea "github.com/charmbracelet/bubbletea"
	"station/internal/db"
)

// ContentComponent defines a simplified interface for tab content components
// Following soft-serve pattern: components only render content, not full screens
type ContentComponent interface {
	// Basic Bubble Tea model interface
	Init() tea.Cmd
	Update(tea.Msg) (ContentComponent, tea.Cmd)
	
	// Content rendering - returns ONLY the content, no headers/navigation
	RenderContent() string
	
	// Tab management
	RefreshData() tea.Cmd
	SetSize(width, height int)
	GetTitle() string
	IsLoading() bool
	
	// Navigation help text for status bar
	GetHelpText() string
}

// TabModel defines the interface that all tab models must implement
// DEPRECATED: Use ContentComponent instead for new components
type TabModel interface {
	// Standard Bubble Tea model interface
	Init() tea.Cmd
	Update(tea.Msg) (TabModel, tea.Cmd)
	View() string
	
	// Additional methods for tab management
	RefreshData() tea.Cmd
	SetSize(width, height int)
	GetTitle() string
	IsLoading() bool
	IsMainView() bool // Check if tab is in main list view (not sub-view)
	
	// Navigation state management
	CanGoBack() bool
	GoBack() tea.Cmd
	GetBreadcrumb() string
}

// BaseTabModel provides common functionality for all tabs
type BaseTabModel struct {
	db          db.Database
	width       int
	height      int
	loading     bool
	error       string
	title       string
	navStack    []string  // Navigation breadcrumb stack
	viewMode    string    // Current view mode (list, detail, edit)
	selectedID  string    // Currently selected item ID
}

// NewBaseTabModel creates a new base tab model
func NewBaseTabModel(database db.Database, title string) BaseTabModel {
	return BaseTabModel{
		db:       database,
		title:    title,
		loading:  false,
		navStack: []string{title}, // Initialize with root title
		viewMode: "list",           // Default to list view
	}
}

// SetSize updates the tab dimensions
func (m *BaseTabModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// GetTitle returns the tab title
func (m BaseTabModel) GetTitle() string {
	return m.title
}

// IsLoading returns loading state
func (m BaseTabModel) IsLoading() bool {
	return m.loading
}

// SetLoading updates loading state
func (m *BaseTabModel) SetLoading(loading bool) {
	m.loading = loading
}

// SetError sets error message
func (m *BaseTabModel) SetError(err string) {
	m.error = err
}

// GetError returns current error
func (m BaseTabModel) GetError() string {
	return m.error
}

// Navigation methods for BaseTabModel
func (m BaseTabModel) CanGoBack() bool {
	return len(m.navStack) > 1
}

func (m *BaseTabModel) GoBack() tea.Cmd {
	if len(m.navStack) > 1 {
		m.navStack = m.navStack[:len(m.navStack)-1]
		m.viewMode = "list"
		m.selectedID = ""
	}
	return nil
}

func (m BaseTabModel) GetBreadcrumb() string {
	if len(m.navStack) <= 1 {
		return m.title
	}
	return strings.Join(m.navStack, " › ")
}

// Navigation helpers
func (m *BaseTabModel) PushNavigation(title string) {
	m.navStack = append(m.navStack, title)
}

func (m *BaseTabModel) SetViewMode(mode string) {
	m.viewMode = mode
}

func (m BaseTabModel) GetViewMode() string {
	return m.viewMode
}

func (m *BaseTabModel) SetSelectedID(id string) {
	m.selectedID = id
}

func (m BaseTabModel) GetSelectedID() string {
	return m.selectedID
}

// IsMainView returns true if the tab is in its main list view
func (m BaseTabModel) IsMainView() bool {
	return m.viewMode == "list" && len(m.navStack) <= 1
}