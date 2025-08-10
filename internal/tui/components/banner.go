package components

import (
	"strings"
	
	"github.com/charmbracelet/lipgloss"
	"station/internal/tui/styles"
)

// STATION banner - compact single line version with Tokyo Night colors
func RenderBanner() string {
	// Compact banner with Tokyo Night gradient
	stationText := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7aa2f7")). // Tokyo Night blue
		Bold(true).
		Render("STATION")
	
	broadcast := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#bb9af7")). // Tokyo Night purple
		Render("◆◇◆")
	
	platform := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7dcfff")). // Tokyo Night cyan
		Render("AI AGENT MANAGEMENT PLATFORM")
	
	// Combine into single line with spacing
	banner := lipgloss.JoinHorizontal(
		lipgloss.Left,
		stationText,
		" ",
		broadcast,
		" ",
		platform,
		" ",
		broadcast,
	)
	
	// Center the banner
	return lipgloss.NewStyle().
		Align(lipgloss.Center).
		Render(banner)
}

// Simple branding text for status bar
func RenderBranding() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")). // Tokyo Night muted
		Italic(true).
		Render("v1.0.0")
}

// Version info
func RenderVersion() string {
	return lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Render("v1.0.0")
}

// System status indicator
func RenderSystemStatus(isHealthy bool) string {
	var style lipgloss.Style
	var text string
	
	if isHealthy {
		style = styles.SuccessStyle
		text = "● ONLINE"
	} else {
		style = styles.ErrorStyle  
		text = "● OFFLINE"
	}
	
	return style.Render(text)
}

// Animated loading indicator for retro feel
func RenderLoadingIndicator(frame int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinner := frames[frame%len(frames)]
	
	return lipgloss.NewStyle().
		Foreground(styles.Primary).
		Render(spinner + " Loading...")
}

// Header decoration for sections
func RenderSectionHeader(title string) string {
	// Create retro-style section divider
	line := strings.Repeat("─", 50)
	
	headerStyle := lipgloss.NewStyle().
		Foreground(styles.Primary).
		Bold(true)
	
	lineStyle := lipgloss.NewStyle().
		Foreground(styles.TextMuted)
	
	return lipgloss.JoinVertical(
		lipgloss.Left,
		lineStyle.Render(line),
		headerStyle.Render("▶ "+title),
		lineStyle.Render(line),
		"",
	)
}