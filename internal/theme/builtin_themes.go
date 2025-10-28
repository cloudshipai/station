package theme

import (
	"station/internal/db/queries"
)

// ThemeData represents theme data for initialization
type ThemeData struct {
	Name        string
	DisplayName string
	Description string
	IsDefault   bool
	Colors      map[string]string
}

// getBuiltInThemes returns all built-in themes with Tokyo Night aesthetic
func getBuiltInThemes() []ThemeData {
	return []ThemeData{
		{
			Name:        "tokyo_midnight_techno",
			DisplayName: "Tokyo Midnight Techno",
			Description: "The ultimate retroish Tokyo midnight techno vibe - Blade Runner meets Akira",
			IsDefault:   true,
			Colors: map[string]string{
				// Foundation
				"background":       "#0a0a0f", // Void black with blue tint
				"background_dark":  "#050507", // Deeper void
				"background_light": "#1a1b26", // Tokyo Night background

				// Neon grid
				"primary":   "#00d4ff", // Electric blue (Tron-like)
				"secondary": "#ff0080", // Holographic pink
				"accent":    "#b400ff", // Laser purple

				// State colors
				"success": "#00ff88", // Matrix green
				"warning": "#ffaa00", // Alert amber
				"error":   "#ff0040", // Critical red
				"info":    "#00ffcc", // Info cyan

				// Text hierarchy
				"text":       "#e0e0ff", // Slightly blue white
				"text_muted": "#9090a0", // Muted purple-gray
				"text_dim":   "#606070", // Faded text

				// Interactive
				"border":    "#2d3436", // Techno gray
				"highlight": "#00ff88", // Matrix green highlight
				"selected":  "#ff0080", // Holographic pink selection
			},
		},
		{
			Name:        "tokyo_night_classic",
			DisplayName: "Tokyo Night Classic",
			Description: "The original Tokyo Night theme - clean and professional",
			IsDefault:   false,
			Colors: map[string]string{
				// Background layers
				"background":       "#1a1b26", // Main background
				"background_dark":  "#16161e", // Darker sections
				"background_light": "#24283b", // Raised elements

				// Primary colors
				"primary":   "#7aa2f7", // Primary blue
				"secondary": "#bb9af7", // Purple
				"accent":    "#73daca", // Cyan

				// State colors
				"success": "#9ece6a", // Green
				"warning": "#e0af68", // Yellow
				"error":   "#f7768e", // Red
				"info":    "#7aa2f7", // Blue

				// Text
				"text":       "#c0caf5", // Main text
				"text_muted": "#787c99", // Secondary text
				"text_dim":   "#565f89", // Dim text

				// Interactive
				"border":    "#414868", // Border
				"highlight": "#7aa2f7", // Highlight
				"selected":  "#364a82", // Selection
			},
		},
		{
			Name:        "cyberpunk_neon",
			DisplayName: "Cyberpunk Neon Fusion",
			Description: "High-intensity cyberpunk with neon fusion aesthetics",
			IsDefault:   false,
			Colors: map[string]string{
				// Dark base
				"background":       "#091833", // Deep blue-black
				"background_dark":  "#0a0047", // Midnight purple
				"background_light": "#133e7c", // Deep accent

				// Neon accents
				"primary":   "#ea00d9", // Hot pink
				"secondary": "#0abdc6", // Electric cyan
				"accent":    "#711c91", // Deep purple

				// State colors
				"success": "#00ff9f", // Matrix green
				"warning": "#fcee0c", // Neon yellow
				"error":   "#ff4499", // Vibrant pink
				"info":    "#00b8ff", // Bright blue

				// Text
				"text":       "#ffffff", // Pure white
				"text_muted": "#b0b0ff", // Light blue-white
				"text_dim":   "#8080a0", // Dim purple-white

				// Interactive
				"border":    "#711c91", // Purple border
				"highlight": "#ea00d9", // Hot pink highlight
				"selected":  "#0abdc6", // Cyan selection
			},
		},
		{
			Name:        "retro_wave",
			DisplayName: "Retro Wave Tokyo",
			Description: "Synthwave-inspired retro Tokyo aesthetics",
			IsDefault:   false,
			Colors: map[string]string{
				// Base colors
				"background":       "#000000", // Pure black
				"background_dark":  "#0a0047", // Deep blue
				"background_light": "#004687", // Steel blue

				// Retro highlights
				"primary":   "#00ffd2", // Bright cyan
				"secondary": "#ff4499", // Vibrant pink
				"accent":    "#bd00ff", // Electric purple

				// State colors
				"success": "#00ff88", // Green
				"warning": "#fcee0c", // Neon yellow
				"error":   "#ff6b6b", // Soft neon orange
				"info":    "#00ffd2", // Cyan

				// Text
				"text":       "#ffffff", // White
				"text_muted": "#cccccc", // Light gray
				"text_dim":   "#888888", // Dim gray

				// Interactive
				"border":    "#bd00ff", // Purple border
				"highlight": "#00ffd2", // Cyan highlight
				"selected":  "#ff4499", // Pink selection
			},
		},
	}
}

// buildThemeFromRows builds a Theme from database rows
func buildThemeFromRows(rows []queries.GetDefaultThemeWithColorsRow) *Theme {
	if len(rows) == 0 {
		return getDefaultTheme()
	}

	firstRow := rows[0]
	theme := &Theme{
		ID:          firstRow.ThemeID,
		Name:        firstRow.ThemeName,
		DisplayName: firstRow.DisplayName,
		Colors:      make(map[string]string),
	}

	if firstRow.ThemeDescription.Valid {
		theme.Description = firstRow.ThemeDescription.String
	}

	if firstRow.IsBuiltIn.Valid {
		theme.IsBuiltIn = firstRow.IsBuiltIn.Bool
	}

	if firstRow.IsDefault.Valid {
		theme.IsDefault = firstRow.IsDefault.Bool
	}

	// Add colors
	for _, row := range rows {
		if row.ColorKey.Valid && row.ColorValue.Valid {
			theme.Colors[row.ColorKey.String] = row.ColorValue.String
		}
	}

	return theme
}

// buildThemeFromUserRows builds a Theme from user theme rows
func buildThemeFromUserRows(rows []queries.GetUserThemeWithColorsRow) *Theme {
	if len(rows) == 0 {
		return getDefaultTheme()
	}

	firstRow := rows[0]
	theme := &Theme{
		ID:          firstRow.ThemeID,
		Name:        firstRow.ThemeName,
		DisplayName: firstRow.DisplayName,
		Colors:      make(map[string]string),
	}

	if firstRow.ThemeDescription.Valid {
		theme.Description = firstRow.ThemeDescription.String
	}

	if firstRow.IsBuiltIn.Valid {
		theme.IsBuiltIn = firstRow.IsBuiltIn.Bool
	}

	if firstRow.IsDefault.Valid {
		theme.IsDefault = firstRow.IsDefault.Bool
	}

	// Add colors
	for _, row := range rows {
		if row.ColorKey.Valid && row.ColorValue.Valid {
			theme.Colors[row.ColorKey.String] = row.ColorValue.String
		}
	}

	return theme
}

// getDefaultTheme returns a fallback theme if database is unavailable
func getDefaultTheme() *Theme {
	return &Theme{
		ID:          1,
		Name:        "tokyo_midnight_techno",
		DisplayName: "Tokyo Midnight Techno",
		Description: "The ultimate retroish Tokyo midnight techno vibe",
		IsBuiltIn:   true,
		IsDefault:   true,
		Colors: map[string]string{
			"background":       "#0a0a0f",
			"background_dark":  "#050507",
			"background_light": "#1a1b26",
			"primary":          "#00d4ff",
			"secondary":        "#ff0080",
			"accent":           "#b400ff",
			"success":          "#00ff88",
			"warning":          "#ffaa00",
			"error":            "#ff0040",
			"info":             "#00ffcc",
			"text":             "#e0e0ff",
			"text_muted":       "#9090a0",
			"text_dim":         "#606070",
			"border":           "#2d3436",
			"highlight":        "#00ff88",
			"selected":         "#ff0080",
		},
	}
}

// getDefaultPalette returns a fallback color palette
func getDefaultPalette() ColorPalette {
	return ColorPalette{
		Background:      "#0a0a0f",
		BackgroundDark:  "#050507",
		BackgroundLight: "#1a1b26",
		Primary:         "#00d4ff",
		Secondary:       "#ff0080",
		Accent:          "#b400ff",
		Success:         "#00ff88",
		Warning:         "#ffaa00",
		Error:           "#ff0040",
		Info:            "#00ffcc",
		Text:            "#e0e0ff",
		TextMuted:       "#9090a0",
		TextDim:         "#606070",
		Border:          "#2d3436",
		Highlight:       "#00ff88",
		Selected:        "#ff0080",
	}
}
