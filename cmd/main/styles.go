package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

// CLI styles using Tokyo Night theme
var (
	titleStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#bb9af7")).
		Foreground(lipgloss.Color("#1a1b26")).
		Bold(true).
		Padding(0, 2).
		MarginBottom(1)

	bannerStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#bb9af7")).
		Padding(1, 2).
		MarginBottom(1)

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9ece6a")).
		Bold(true)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#f7768e")).
		Bold(true)

	infoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7dcfff"))

	// Interactive form styles - Tokyo Night theme
	focusedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#bb9af7")).
		Bold(true)
	
	blurredStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89"))
	
	cursorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#bb9af7"))
	
	noStyle = lipgloss.NewStyle()
	
	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#565f89")).
		Italic(true)
	
	formStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#414868")).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)
)

// showSuccessBanner displays a celebration banner with confetti
func showSuccessBanner(message string) {
	confetti := "🎉✨🎊"
	
	banner := bannerStyle.Render(fmt.Sprintf("%s\n%s\n%s", 
		confetti, 
		successStyle.Render(message), 
		confetti))
	
	fmt.Println(banner)
}