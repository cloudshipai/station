package main

import (
	"fmt"
	"station/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// CLIStyles contains all styled components for the CLI
type CLIStyles struct {
	Title    lipgloss.Style
	Banner   lipgloss.Style
	Success  lipgloss.Style
	Error    lipgloss.Style
	Info     lipgloss.Style
	Focused  lipgloss.Style
	Blurred  lipgloss.Style
	Cursor   lipgloss.Style
	No       lipgloss.Style
	Help     lipgloss.Style
	Form     lipgloss.Style
}

// getCLIStyles returns theme-aware CLI styles
func getCLIStyles(themeManager *theme.ThemeManager) CLIStyles {
	if themeManager == nil {
		// Fallback to hardcoded Tokyo Night styles
		return CLIStyles{
			Title: lipgloss.NewStyle().
				Background(lipgloss.Color("#bb9af7")).
				Foreground(lipgloss.Color("#1a1b26")).
				Bold(true).
				Padding(0, 2).
				MarginBottom(1),
			Banner: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#bb9af7")).
				Padding(1, 2).
				MarginBottom(1),
			Success: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a")).
				Bold(true),
			Error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f7768e")).
				Bold(true),
			Info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7dcfff")),
			Focused: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#bb9af7")).
				Bold(true),
			Blurred: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89")),
			Cursor: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#bb9af7")),
			No: lipgloss.NewStyle(),
			Help: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89")).
				Italic(true),
			Form: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#414868")).
				Padding(1, 2).
				MarginTop(1).
				MarginBottom(1),
		}
	}

	themeStyles := themeManager.GetStyles()
	palette := themeManager.GetPalette()

	return CLIStyles{
		Title: themeStyles.Header.Copy().
			Background(lipgloss.Color(palette.Secondary)).
			Foreground(lipgloss.Color(palette.BackgroundDark)).
			Padding(0, 2).
			MarginBottom(1),
		Banner: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Secondary)).
			Padding(1, 2).
			MarginBottom(1),
		Success: themeStyles.Success,
		Error:   themeStyles.Error,
		Info:    themeStyles.Info,
		Focused: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Secondary)).
			Bold(true),
		Blurred: themeStyles.Muted,
		Cursor: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Secondary)),
		No: lipgloss.NewStyle(),
		Help: themeStyles.Muted.Copy().
			Italic(true),
		Form: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Border)).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1),
	}
}

// showSuccessBanner displays a celebration banner with confetti
func showSuccessBanner(message string, themeManager *theme.ThemeManager) {
	styles := getCLIStyles(themeManager)
	confetti := "🎉✨🎊"
	
	banner := styles.Banner.Render(fmt.Sprintf("%s\n%s\n%s", 
		confetti, 
		styles.Success.Render(message), 
		confetti))
	
	fmt.Println(banner)
}