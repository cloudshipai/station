package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Tokyo Night theme with proper color palette
var (
	// Core colors - Official Tokyo Night theme
	Primary   = lipgloss.Color("#7aa2f7") // Tokyo Night blue
	Secondary = lipgloss.Color("#bb9af7") // Tokyo Night purple  
	Accent    = lipgloss.Color("#7dcfff") // Tokyo Night cyan
	Background = lipgloss.Color("#1a1b26") // Tokyo Night dark background
	Surface   = lipgloss.Color("#24283b") // Tokyo Night surface
	Text      = lipgloss.Color("#a9b1d6") // Tokyo Night main foreground
	TextMuted = lipgloss.Color("#565f89") // Tokyo Night comment
	Success   = lipgloss.Color("#9ece6a") // Tokyo Night green
	Warning   = lipgloss.Color("#e0af68") // Tokyo Night yellow
	Error     = lipgloss.Color("#f7768e") // Tokyo Night red
	
	// Base styles
	BaseStyle = lipgloss.NewStyle().
		Foreground(Text).
		Background(Background)
	
	// Tab styles
	InactiveTab = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(TextMuted).
		Padding(0, 2).
		Foreground(TextMuted)
	
	ActiveTab = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Background(Surface).
		Padding(0, 2).
		Foreground(Primary).
		Bold(true)
	
	// Content area
	ContentStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Background(Background).
		Padding(1, 2).
		Margin(1, 0)
	
	// Header styles
	HeaderStyle = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		Underline(true)
	
	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
		Background(Surface).
		Foreground(Text).
		Padding(0, 1)
	
	StatusKeyStyle = lipgloss.NewStyle().
		Background(Primary).
		Foreground(Background).
		Padding(0, 1).
		Bold(true)
	
	// List item styles
	ListItemStyle = lipgloss.NewStyle().
		Foreground(Text).
		Padding(0, 2)
	
	ListItemSelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#bb9af7")). // Tokyo Night purple for selection
		Foreground(lipgloss.Color("#1a1b26")). // Dark background for contrast
		Padding(0, 2).
		Bold(true)
	
	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
		Background(Surface).
		Foreground(Primary).
		Bold(true).
		Padding(0, 1)
	
	TableCellStyle = lipgloss.NewStyle().
		Foreground(Text).
		Padding(0, 1)
	
	TableSelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#bb9af7")). // Tokyo Night purple for selection
		Foreground(lipgloss.Color("#1a1b26")). // Dark background for contrast
		Padding(0, 1).
		Bold(true)
	
	// Form styles
	InputStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(TextMuted).
		Foreground(Text).
		Padding(0, 1)
	
	InputFocusedStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Foreground(Primary).
		Padding(0, 1)
	
	// Button styles
	ButtonStyle = lipgloss.NewStyle().
		Background(Surface).
		Foreground(Text).
		Padding(0, 2).
		Margin(0, 1)
	
	ButtonActiveStyle = lipgloss.NewStyle().
		Background(Primary).
		Foreground(Background).
		Padding(0, 2).
		Margin(0, 1).
		Bold(true)
	
	// Help styles
	HelpStyle = lipgloss.NewStyle().
		Foreground(TextMuted).
		Italic(true)
	
	// Dialog styles
	DialogStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Background(Background).
		Padding(1, 2)
	
	// Error styles
	ErrorStyle = lipgloss.NewStyle().
		Foreground(Error).
		Bold(true)
	
	SuccessStyle = lipgloss.NewStyle().
		Foreground(Success).
		Bold(true)
	
	WarningStyle = lipgloss.NewStyle().
		Foreground(Warning).
		Bold(true)
)

// Helper functions for common styling patterns
func WithBorder(style lipgloss.Style) lipgloss.Style {
	return style.
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary)
}

func WithPadding(style lipgloss.Style, vertical, horizontal int) lipgloss.Style {
	return style.Padding(vertical, horizontal)
}

func WithMargin(style lipgloss.Style, vertical, horizontal int) lipgloss.Style {
	return style.Margin(vertical, horizontal)
}