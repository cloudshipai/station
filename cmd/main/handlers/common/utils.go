package common

import (
	"fmt"
	"os"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"station/internal/config"
	"station/internal/theme"
)

// CLIStyles contains theme-aware styles for CLI output
type CLIStyles struct {
	Banner  lipgloss.Style
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
	Muted   lipgloss.Style
}

// GetCLIStyles returns theme-aware CLI styles
func GetCLIStyles(themeManager *theme.ThemeManager) CLIStyles {
	if themeManager == nil {
		// Fallback to hardcoded styles
		return CLIStyles{
			Banner: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#bb9af7")).
				Bold(true).
				MarginBottom(1),
			Success: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a")),
			Error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f7768e")),
			Warning: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#e0af68")),
			Info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7dcfff")),
			Muted: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#565f89")),
		}
	}

	palette := themeManager.GetPalette()
	themeStyles := themeManager.GetStyles()

	return CLIStyles{
		Banner: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Primary)).
			Bold(true).
			MarginBottom(1),
		Success: themeStyles.Success,
		Error:   themeStyles.Error,
		Warning: themeStyles.Warning,
		Info:    themeStyles.Info,
		Muted:   themeStyles.Muted,
	}
}

// LoadStationConfig loads the Station configuration using the main config.Load() function
func LoadStationConfig() (*config.Config, error) {
	return config.Load()
}

// GetAPIKeyFromEnv retrieves API key from environment variables
func GetAPIKeyFromEnv() string {
	if key := os.Getenv("STATION_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("API_KEY"); key != "" {
		return key
	}
	return ""
}

// TruncateString truncates a string to the specified length
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ParseIDFromString safely parses an ID from a string
func ParseIDFromString(idStr string) (int64, error) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid ID format: %v", err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("ID must be positive")
	}
	return id, nil
}

// FormatBytes formats bytes into human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Contains checks if a string slice contains a value
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetStationConfigRoot returns the Station configuration root directory
func GetStationConfigRoot() (string, error) {
	return config.GetStationConfigDir(), nil
}

// GetEnvironmentIDByName retrieves environment ID by name
func GetEnvironmentIDByName(endpoint, environment string) (int64, error) {
	// This is a placeholder - in a real implementation, this would make an API call
	// or query the database to get the environment ID by name
	return 0, fmt.Errorf("environment lookup not implemented")
}

// GetBoolFlag safely gets a boolean flag value
func GetBoolFlag(value interface{}) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}

// GetStringFlag safely gets a string flag value
func GetStringFlag(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

// GetIntFlag safely gets an integer flag value
func GetIntFlag(value interface{}) int {
	if i, ok := value.(int); ok {
		return i
	}
	return 0
}
