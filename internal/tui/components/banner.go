package components

import (
	"strings"
	
	"github.com/charmbracelet/lipgloss"
	"station/internal/tui/styles"
)

// STATION banner with retro ASCII art and multi-colored blue theme
func RenderBanner() string {
	// Retro 80s-style ASCII art for STATION with neon/terminal aesthetics
	bannerText := []string{
		"╔═══════════════════════════════════════════════════════════╗",
		"║   ▄████████    ▄████████    ▄████████    ▄████████  ███    ███",
		"║  ███    ███   ███    ███   ███    ███   ███    ███  ███    ███",  
		"║  ███    █▀    ███    █▀    ███    ███   ███    ███  ███    ███",
		"║ ▄███          ███         ▄███▄▄▄▄██▀  ▄███▄▄▄▄██▀  ███    ███",
		"║▀▀███ ████▄  ▀███████████ ▀▀███▀▀▀▀▀   ▀▀███▀▀▀▀▀    ███    ███",
		"║  ███    ███          ███ ▀███████████ ▀███████████  ███    ███",
		"║  ███    ███    ▄█    ███   ███    ███   ███    ███  ███▄▄▄███",
		"║  ████████▀   ▄████████▀    ███    ███   ███    █▀    ▀▀▀▀▀▀▀",
		"║              ███    ███",
		"╚═══════════════════════════════════════════════════════════╝",
	}
	
	// Retro neon color gradient - cyan to magenta
	colors := []lipgloss.Color{
		lipgloss.Color("#00FFFF"), // Cyan
		lipgloss.Color("#40E0D0"), // Turquoise
		lipgloss.Color("#FF1493"), // Deep pink
		lipgloss.Color("#FF69B4"), // Hot pink
		lipgloss.Color("#00CED1"), // Dark turquoise
		lipgloss.Color("#8A2BE2"), // Blue violet
		lipgloss.Color("#FF00FF"), // Magenta
		lipgloss.Color("#00BFFF"), // Deep sky blue
		lipgloss.Color("#DA70D6"), // Orchid
		lipgloss.Color("#1E90FF"), // Dodger blue
		lipgloss.Color("#C71585"), // Medium violet red
	}
	
	var coloredLines []string
	for i, line := range bannerText {
		color := colors[i%len(colors)]
		style := lipgloss.NewStyle().
			Foreground(color).
			Bold(true)
		coloredLines = append(coloredLines, style.Render(line))
	}
	
	banner := strings.Join(coloredLines, "\n")
	
	// Add retro subtitle with neon glow effect
	subtitle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00FFFF")).
		Background(lipgloss.Color("#000080")).
		Bold(true).
		Align(lipgloss.Center).
		Render("◆◇◆ AI AGENT MANAGEMENT PLATFORM ◆◇◆")
	
	// Combine banner and subtitle
	return lipgloss.JoinVertical(
		lipgloss.Center,
		banner,
		"",
		subtitle,
	)
}

// CloudshipAI branding for bottom right
func RenderBranding() string {
	return lipgloss.NewStyle().
		Foreground(styles.TextMuted).
		Italic(true).
		Render("makers of cloudshipai")
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