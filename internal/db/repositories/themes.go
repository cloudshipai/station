package repositories

import (
	"context"
	"database/sql"
	"station/internal/db/queries"
)

// ThemeRepo handles theme-related database operations
type ThemeRepo struct {
	queries *queries.Queries
}

// NewThemeRepo creates a new theme repository
func NewThemeRepo(db *sql.DB) *ThemeRepo {
	return &ThemeRepo{
		queries: queries.New(db),
	}
}

// CreateTheme creates a new theme
func (r *ThemeRepo) CreateTheme(ctx context.Context, name, displayName, description string, isBuiltIn, isDefault bool, createdBy int64) (queries.Theme, error) {
	return r.queries.CreateTheme(ctx, queries.CreateThemeParams{
		Name:        name,
		DisplayName: displayName,
		Description: sql.NullString{String: description, Valid: description != ""},
		IsBuiltIn:   sql.NullBool{Bool: isBuiltIn, Valid: true},
		IsDefault:   sql.NullBool{Bool: isDefault, Valid: true},
		CreatedBy:   sql.NullInt64{Int64: createdBy, Valid: true},
	})
}

// GetTheme gets a theme by ID
func (r *ThemeRepo) GetTheme(ctx context.Context, id int64) (queries.Theme, error) {
	return r.queries.GetTheme(ctx, id)
}

// GetThemeByName gets a theme by name
func (r *ThemeRepo) GetThemeByName(ctx context.Context, name string) (queries.Theme, error) {
	return r.queries.GetThemeByName(ctx, name)
}

// ListThemes lists all themes
func (r *ThemeRepo) ListThemes(ctx context.Context) ([]queries.Theme, error) {
	return r.queries.ListThemes(ctx)
}

// ListBuiltInThemes lists built-in themes only
func (r *ThemeRepo) ListBuiltInThemes(ctx context.Context) ([]queries.Theme, error) {
	return r.queries.ListBuiltInThemes(ctx)
}

// SetDefaultTheme sets a theme as default
func (r *ThemeRepo) SetDefaultTheme(ctx context.Context, themeID int64) error {
	return r.queries.SetDefaultTheme(ctx, themeID)
}

// DeleteTheme deletes a non-built-in theme
func (r *ThemeRepo) DeleteTheme(ctx context.Context, id int64) error {
	return r.queries.DeleteTheme(ctx, id)
}

// CreateThemeColor creates a theme color
func (r *ThemeRepo) CreateThemeColor(ctx context.Context, themeID int64, colorKey, colorValue, description string) (queries.ThemeColor, error) {
	return r.queries.CreateThemeColor(ctx, queries.CreateThemeColorParams{
		ThemeID:     themeID,
		ColorKey:    colorKey,
		ColorValue:  colorValue,
		Description: sql.NullString{String: description, Valid: description != ""},
	})
}

// GetThemeColors gets all colors for a theme
func (r *ThemeRepo) GetThemeColors(ctx context.Context, themeID int64) ([]queries.ThemeColor, error) {
	return r.queries.GetThemeColors(ctx, themeID)
}

// GetThemeColor gets a specific color for a theme
func (r *ThemeRepo) GetThemeColor(ctx context.Context, themeID int64, colorKey string) (queries.ThemeColor, error) {
	return r.queries.GetThemeColor(ctx, queries.GetThemeColorParams{
		ThemeID:  themeID,
		ColorKey: colorKey,
	})
}

// UpdateThemeColor updates a theme color
func (r *ThemeRepo) UpdateThemeColor(ctx context.Context, themeID int64, colorKey, colorValue, description string) (queries.ThemeColor, error) {
	return r.queries.UpdateThemeColor(ctx, queries.UpdateThemeColorParams{
		ThemeID:     themeID,
		ColorKey:    colorKey,
		ColorValue:  colorValue,
		Description: sql.NullString{String: description, Valid: description != ""},
	})
}

// DeleteThemeColor deletes a theme color
func (r *ThemeRepo) DeleteThemeColor(ctx context.Context, themeID int64, colorKey string) error {
	return r.queries.DeleteThemeColor(ctx, queries.DeleteThemeColorParams{
		ThemeID:  themeID,
		ColorKey: colorKey,
	})
}

// SetUserTheme sets a user's theme preference
func (r *ThemeRepo) SetUserTheme(ctx context.Context, userID, themeID int64) (queries.UserThemePreference, error) {
	return r.queries.SetUserTheme(ctx, queries.SetUserThemeParams{
		UserID:  userID,
		ThemeID: themeID,
	})
}

// GetUserTheme gets a user's theme preference
func (r *ThemeRepo) GetUserTheme(ctx context.Context, userID int64) (queries.GetUserThemeRow, error) {
	return r.queries.GetUserTheme(ctx, userID)
}

// GetUserThemeWithColors gets a user's theme with all colors
func (r *ThemeRepo) GetUserThemeWithColors(ctx context.Context, userID int64) ([]queries.GetUserThemeWithColorsRow, error) {
	return r.queries.GetUserThemeWithColors(ctx, userID)
}

// GetDefaultTheme gets the default theme
func (r *ThemeRepo) GetDefaultTheme(ctx context.Context) (queries.Theme, error) {
	return r.queries.GetDefaultTheme(ctx)
}

// GetDefaultThemeWithColors gets the default theme with all colors
func (r *ThemeRepo) GetDefaultThemeWithColors(ctx context.Context) ([]queries.GetDefaultThemeWithColorsRow, error) {
	return r.queries.GetDefaultThemeWithColors(ctx)
}
