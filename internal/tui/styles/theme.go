package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Retro terminal theme with blue colors
var (
	// Core colors - retro blue theme
	Primary   = lipgloss.Color("#00BFFF") // Deep sky blue
	Secondary = lipgloss.Color("#4169E1") // Royal blue  
	Accent    = lipgloss.Color("#87CEEB") // Sky blue
	Background = lipgloss.Color("#000B1E") // Very dark blue
	Surface   = lipgloss.Color("#1E3875") // Dark blue
	Text      = lipgloss.Color("#E6F3FF") // Light blue-white
	TextMuted = lipgloss.Color("#8BB8E8") // Muted blue
	Success   = lipgloss.Color("#00FF7F") // Spring green
	Warning   = lipgloss.Color("#FFD700") // Gold
	Error     = lipgloss.Color("#FF4500") // Orange red
	
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
		Background(Surface).
		Foreground(Primary).
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
		Background(Surface).
		Foreground(Primary).
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