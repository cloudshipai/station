package theme

import "github.com/charmbracelet/lipgloss"

// Theme represents the color scheme for the TUI
type Theme struct {
	// Text colors
	Text       lipgloss.Color
	TextMuted  lipgloss.Color
	
	// Background colors
	Background        lipgloss.Color
	BackgroundElement lipgloss.Color
	BackgroundPanel   lipgloss.Color
	
	// Accent colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color
	
	// Status colors
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	
	// UI elements
	Border lipgloss.Color
}

// DefaultTheme returns the default Station theme
func DefaultTheme() *Theme {
	return &Theme{
		// Text colors - high contrast for readability
		Text:      lipgloss.Color("#E0E0E0"), // Light gray text
		TextMuted: lipgloss.Color("#888888"), // Muted gray
		
		// Background colors - dark theme
		Background:        lipgloss.Color("#1A1A1A"), // Very dark gray
		BackgroundElement: lipgloss.Color("#2D2D30"), // Slightly lighter for elements
		BackgroundPanel:   lipgloss.Color("#252526"), // Panel background
		
		// Accent colors - Station branding inspired
		Primary:   lipgloss.Color("#007ACC"), // Blue primary
		Secondary: lipgloss.Color("#FFA500"), // Orange secondary  
		Accent:    lipgloss.Color("#00D4FF"), // Cyan accent
		
		// Status colors
		Success: lipgloss.Color("#4CAF50"), // Green
		Warning: lipgloss.Color("#FF9800"), // Orange  
		Error:   lipgloss.Color("#F44336"), // Red
		
		// UI elements
		Border: lipgloss.Color("#404040"), // Subtle border
	}
}

// OpenCodeTheme returns a theme similar to OpenCode
func OpenCodeTheme() *Theme {
	return &Theme{
		Text:      lipgloss.Color("#D4D4D4"),
		TextMuted: lipgloss.Color("#6A737D"),
		
		Background:        lipgloss.Color("#0D1117"),
		BackgroundElement: lipgloss.Color("#161B22"),
		BackgroundPanel:   lipgloss.Color("#21262D"),
		
		Primary:   lipgloss.Color("#58A6FF"),
		Secondary: lipgloss.Color("#FF7B72"), 
		Accent:    lipgloss.Color("#A5A5A5"),
		
		Success: lipgloss.Color("#3FB950"),
		Warning: lipgloss.Color("#D29922"),
		Error:   lipgloss.Color("#F85149"),
		
		Border: lipgloss.Color("#30363D"),
	}
}

// LightTheme returns a light theme variant
func LightTheme() *Theme {
	return &Theme{
		Text:      lipgloss.Color("#24292F"),
		TextMuted: lipgloss.Color("#656D76"),
		
		Background:        lipgloss.Color("#FFFFFF"),
		BackgroundElement: lipgloss.Color("#F6F8FA"),
		BackgroundPanel:   lipgloss.Color("#F0F3F6"),
		
		Primary:   lipgloss.Color("#0969DA"),
		Secondary: lipgloss.Color("#BF8700"),
		Accent:    lipgloss.Color("#6F42C1"),
		
		Success: lipgloss.Color("#1A7F37"),
		Warning: lipgloss.Color("#9A6700"),
		Error:   lipgloss.Color("#D1242F"),
		
		Border: lipgloss.Color("#D0D7DE"),
	}
}