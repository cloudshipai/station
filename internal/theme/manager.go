package theme

import (
	"context"
	"database/sql"
	"fmt"
	"station/internal/db"
	"station/internal/db/queries"

	"github.com/charmbracelet/lipgloss"
)

// ThemeManager handles theme operations and provides styled components
type ThemeManager struct {
	db      db.Database
	queries *queries.Queries
	current *Theme
}

// Theme represents a complete theme with all color definitions
type Theme struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	IsBuiltIn   bool   `json:"is_built_in"`
	IsDefault   bool   `json:"is_default"`
	Colors      map[string]string `json:"colors"`
}

// ColorPalette defines the color keys used throughout the application
type ColorPalette struct {
	// Background layers
	Background      string
	BackgroundDark  string
	BackgroundLight string
	
	// Primary colors
	Primary     string
	Secondary   string
	Accent      string
	
	// State colors
	Success     string
	Warning     string
	Error       string
	Info        string
	
	// Text colors
	Text        string
	TextMuted   string
	TextDim     string
	
	// Interactive colors
	Border      string
	Highlight   string
	Selected    string
}

// NewThemeManager creates a new theme manager
func NewThemeManager(database db.Database) *ThemeManager {
	queries := queries.New(database.Conn())
	return &ThemeManager{
		db:      database,
		queries: queries,
	}
}

// InitializeBuiltInThemes creates the built-in themes if they don't exist
func (tm *ThemeManager) InitializeBuiltInThemes(ctx context.Context) error {
	themes := getBuiltInThemes()
	
	for _, themeData := range themes {
		// Check if theme already exists
		existing, err := tm.queries.GetThemeByName(ctx, themeData.Name)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to check existing theme: %w", err)
		}
		
		if err == sql.ErrNoRows {
			// Create the theme
			theme, err := tm.queries.CreateTheme(ctx, queries.CreateThemeParams{
				Name:        themeData.Name,
				DisplayName: themeData.DisplayName,
				Description: sql.NullString{String: themeData.Description, Valid: true},
				IsBuiltIn:   sql.NullBool{Bool: true, Valid: true},
				IsDefault:   sql.NullBool{Bool: themeData.IsDefault, Valid: true},
				CreatedBy:   sql.NullInt64{Int64: 1, Valid: true}, // System user
			})
			if err != nil {
				return fmt.Errorf("failed to create theme %s: %w", themeData.Name, err)
			}
			
			// Add colors
			for key, value := range themeData.Colors {
				_, err := tm.queries.CreateThemeColor(ctx, queries.CreateThemeColorParams{
					ThemeID:     theme.ID,
					ColorKey:    key,
					ColorValue:  value,
					Description: sql.NullString{String: fmt.Sprintf("%s color", key), Valid: true},
				})
				if err != nil {
					return fmt.Errorf("failed to create color %s for theme %s: %w", key, themeData.Name, err)
				}
			}
		} else {
			// Update existing theme if it's the default
			if themeData.IsDefault {
				err := tm.queries.SetDefaultTheme(ctx, existing.ID)
				if err != nil {
					return fmt.Errorf("failed to set default theme: %w", err)
				}
			}
		}
	}
	
	return nil
}

// LoadUserTheme loads the theme for a specific user
func (tm *ThemeManager) LoadUserTheme(ctx context.Context, userID int64) error {
	// Try to get user's theme preference
	userTheme, err := tm.queries.GetUserThemeWithColors(ctx, userID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get user theme: %w", err)
	}
	
	// If no user preference, get default theme
	if err == sql.ErrNoRows {
		defaultTheme, err := tm.queries.GetDefaultThemeWithColors(ctx)
		if err != nil {
			return fmt.Errorf("failed to get default theme: %w", err)
		}
		tm.current = buildThemeFromRows(defaultTheme)
	} else {
		tm.current = buildThemeFromUserRows(userTheme)
	}
	
	return nil
}

// LoadDefaultTheme loads the default theme
func (tm *ThemeManager) LoadDefaultTheme(ctx context.Context) error {
	defaultTheme, err := tm.queries.GetDefaultThemeWithColors(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default theme: %w", err)
	}
	
	tm.current = buildThemeFromRows(defaultTheme)
	return nil
}

// GetCurrentTheme returns the currently loaded theme
func (tm *ThemeManager) GetCurrentTheme() *Theme {
	return tm.current
}

// GetColor returns a color value by key
func (tm *ThemeManager) GetColor(key string) string {
	if tm.current == nil {
		return "#ffffff" // Fallback
	}
	
	if color, exists := tm.current.Colors[key]; exists {
		return color
	}
	
	return "#ffffff" // Fallback
}

// GetPalette returns a structured color palette
func (tm *ThemeManager) GetPalette() ColorPalette {
	if tm.current == nil {
		return getDefaultPalette()
	}
	
	return ColorPalette{
		Background:      tm.GetColor("background"),
		BackgroundDark:  tm.GetColor("background_dark"),
		BackgroundLight: tm.GetColor("background_light"),
		Primary:         tm.GetColor("primary"),
		Secondary:       tm.GetColor("secondary"),
		Accent:          tm.GetColor("accent"),
		Success:         tm.GetColor("success"),
		Warning:         tm.GetColor("warning"),
		Error:           tm.GetColor("error"),
		Info:            tm.GetColor("info"),
		Text:            tm.GetColor("text"),
		TextMuted:       tm.GetColor("text_muted"),
		TextDim:         tm.GetColor("text_dim"),
		Border:          tm.GetColor("border"),
		Highlight:       tm.GetColor("highlight"),
		Selected:        tm.GetColor("selected"),
	}
}

// GetStyles returns pre-configured Lipgloss styles
func (tm *ThemeManager) GetStyles() ThemeStyles {
	palette := tm.GetPalette()
	
	return ThemeStyles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(palette.Primary)).
			Background(lipgloss.Color(palette.BackgroundDark)),
		
		Subheader: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(palette.Secondary)),
		
		Text: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Text)),
		
		Muted: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.TextMuted)),
		
		Success: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Success)),
		
		Warning: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Warning)),
		
		Error: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Error)),
		
		Info: lipgloss.NewStyle().
			Foreground(lipgloss.Color(palette.Info)),
		
		Border: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Border)),
		
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(palette.Selected)).
			Foreground(lipgloss.Color(palette.BackgroundDark)),
		
		Highlight: lipgloss.NewStyle().
			Background(lipgloss.Color(palette.Highlight)).
			Foreground(lipgloss.Color(palette.BackgroundDark)),
		
		Container: lipgloss.NewStyle().
			Align(lipgloss.Center).
			Padding(1, 2).
			Background(lipgloss.Color(palette.Background)),
	}
}

// ListThemes returns all available themes
func (tm *ThemeManager) ListThemes(ctx context.Context) ([]queries.Theme, error) {
	return tm.queries.ListThemes(ctx)
}

// GetThemeByName gets a theme by its name
func (tm *ThemeManager) GetThemeByName(ctx context.Context, name string) (queries.Theme, error) {
	return tm.queries.GetThemeByName(ctx, name)
}

// SetDefaultTheme sets a theme as the default
func (tm *ThemeManager) SetDefaultTheme(ctx context.Context, themeID int64) error {
	return tm.queries.SetDefaultTheme(ctx, themeID)
}

// GetThemeColors gets all colors for a theme
func (tm *ThemeManager) GetThemeColors(ctx context.Context, themeID int64) ([]queries.ThemeColor, error) {
	return tm.queries.GetThemeColors(ctx, themeID)
}

// SetCurrentThemeForPreview sets the current theme temporarily (for preview)
func (tm *ThemeManager) SetCurrentThemeForPreview(t *Theme) {
	tm.current = t
}

// ThemeStyles contains pre-configured Lipgloss styles
type ThemeStyles struct {
	Header    lipgloss.Style
	Subheader lipgloss.Style
	Text      lipgloss.Style
	Muted     lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
	Info      lipgloss.Style
	Border    lipgloss.Style
	Selected  lipgloss.Style
	Highlight lipgloss.Style
	Container lipgloss.Style
}