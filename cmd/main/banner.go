package main

import (
	"fmt"
	"station/internal/theme"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Banner command definition
var (
	bannerCmd = &cobra.Command{
		Use:    "banner",
		Short:  "Display the Station ASCII banner",
		Long:   "Display the beautiful Station ASCII art banner",
		RunE:   runBanner,
		Hidden: true, // Hidden command for screenshots
	}
)

func runBanner(cmd *cobra.Command, args []string) error {
	return displayBanner(nil) // Use default theme for banner command
}

// displayBanner renders the Station banner with theme support
func displayBanner(themeManager *theme.ThemeManager) error {
	// Display the beautiful lipgloss Station banner from the TUI splash screen
	asciiArt := []string{
		"  ███████╗████████╗ █████╗ ████████╗██╗ ██████╗ ███╗   ██╗ ",
		"  ██╔════╝╚══██╔══╝██╔══██╗╚══██╔══╝██║██╔═══██╗████╗  ██║ ",
		"  ███████╗   ██║   ███████║   ██║   ██║██║   ██║██╔██╗ ██║ ",
		"  ╚════██║   ██║   ██╔══██║   ██║   ██║██║   ██║██║╚██╗██║ ",
		"  ███████║   ██║   ██║  ██║   ██║   ██║╚██████╔╝██║ ╚████║ ",
		"  ╚══════╝   ╚═╝   ╚═╝  ╚═╝   ╚═╝   ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ ",
	}

	// Get banner colors based on theme
	var colors []lipgloss.Color
	if themeManager != nil {
		palette := themeManager.GetPalette()
		// Create a gradient using theme colors
		colors = []lipgloss.Color{
			lipgloss.Color(palette.Primary),
			lipgloss.Color(palette.Secondary),
			lipgloss.Color(palette.Accent),
			lipgloss.Color(palette.Info),
			lipgloss.Color(palette.Highlight),
			lipgloss.Color(palette.Primary),
		}
	} else {
		// Fallback to Tokyo Night gradient colors
		colors = []lipgloss.Color{
			lipgloss.Color("#00d4ff"), // Electric blue
			lipgloss.Color("#ff0080"), // Holographic pink
			lipgloss.Color("#b400ff"), // Laser purple
			lipgloss.Color("#00ffcc"), // Info cyan
			lipgloss.Color("#00ff88"), // Matrix green
			lipgloss.Color("#00d4ff"), // Electric blue
		}
	}

	// Color the ASCII art with theme-based gradient
	var coloredLines []string
	for i, line := range asciiArt {
		style := lipgloss.NewStyle().
			Foreground(colors[i%len(colors)]).
			Bold(true)
		coloredLines = append(coloredLines, style.Render(line))
	}

	// Print each line
	fmt.Println()
	for _, line := range coloredLines {
		fmt.Println(line)
	}

	// Add subtitle with theme-aware styling
	fmt.Println()
	var subtitleStyle lipgloss.Style
	if themeManager != nil {
		styles := getCLIStyles(themeManager)
		subtitleStyle = styles.Help
	} else {
		subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9090a0")).
			Italic(true)
	}
	subtitle := subtitleStyle.Render("🚂 Easiest way to build secure, intelligent, background, tool agents")
	fmt.Println(subtitle)

	// Add author credit
	var creditStyle lipgloss.Style
	if themeManager != nil {
		palette := themeManager.GetPalette()
		creditStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.TextDim)).
			Faint(true)
	} else {
		creditStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#606070")).
			Faint(true)
	}
	credit := creditStyle.Render("by the CloudshipAI team")
	fmt.Println(credit)
	fmt.Println()

	return nil
}
