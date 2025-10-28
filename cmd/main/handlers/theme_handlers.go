package handlers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"station/internal/theme"
)

// ThemeHandler handles theme-related commands
type ThemeHandler struct {
	themeManager *theme.ThemeManager
}

func NewThemeHandler(themeManager *theme.ThemeManager) *ThemeHandler {
	return &ThemeHandler{themeManager: themeManager}
}

// Theme list item for interactive selection
type themeItem struct {
	name        string
	displayName string
	description string
	isDefault   bool
	isBuiltIn   bool
}

func (t themeItem) FilterValue() string { return t.name }
func (t themeItem) Title() string {
	title := t.displayName
	if t.isDefault {
		title += " (current)"
	}
	return title
}
func (t themeItem) Description() string { return t.description }

// Interactive theme selector model
type themeSelectModel struct {
	list     list.Model
	choice   string
	quitting bool
	manager  *theme.ThemeManager
}

func (m themeSelectModel) Init() tea.Cmd {
	return nil
}

func (m themeSelectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(themeItem)
			if ok {
				m.choice = i.name
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m themeSelectModel) View() string {
	if m.choice != "" {
		styles := getCLIStyles(m.manager)
		return styles.Success.Render(fmt.Sprintf("✨ Selected theme: %s", m.choice))
	}
	if m.quitting {
		return "Theme selection cancelled.\n"
	}

	styles := getCLIStyles(m.manager)
	header := styles.Banner.Render("🎨 Theme Selection")
	return header + "\n\n" + m.list.View()
}

// RunThemeList lists all available themes
func (h *ThemeHandler) RunThemeList(cmd *cobra.Command, args []string) error {
	if h.themeManager == nil {
		return fmt.Errorf("theme system not available (database not found)")
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("🎨 Available Themes"))
	fmt.Println()

	ctx := context.Background()
	themes, err := h.themeManager.ListThemes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list themes: %w", err)
	}

	current := h.themeManager.GetCurrentTheme()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	for _, t := range themes {
		name := t.Name
		desc := ""
		if t.Description.Valid {
			desc = t.Description.String
		}

		// Style theme name
		nameStyle := styles.Info
		if name == currentName {
			nameStyle = styles.Success
			name += " (current)"
		}

		fmt.Printf("• %s - %s\n", nameStyle.Render(name), desc)
		fmt.Printf("  ID: %d", t.ID)
		if t.IsBuiltIn.Valid && t.IsBuiltIn.Bool {
			fmt.Printf(" | Built-in")
		}
		if t.IsDefault.Valid && t.IsDefault.Bool {
			fmt.Printf(" | Default")
		}
		fmt.Println()
		fmt.Println()
	}

	return nil
}

// RunThemeSet sets the active theme for the current user
func (h *ThemeHandler) RunThemeSet(cmd *cobra.Command, args []string) error {
	if h.themeManager == nil {
		return fmt.Errorf("theme system not available (database not found)")
	}

	themeName := args[0]
	styles := getCLIStyles(h.themeManager)

	ctx := context.Background()

	// Find theme by name
	selectedTheme, err := h.themeManager.GetThemeByName(ctx, themeName)
	if err != nil {
		return fmt.Errorf("theme '%s' not found", themeName)
	}

	// For CLI, we'll set it as the default theme since we don't have user management here
	// In a real application, you'd set it for the specific user
	err = h.themeManager.SetDefaultTheme(ctx, selectedTheme.ID)
	if err != nil {
		return fmt.Errorf("failed to set theme: %w", err)
	}

	// Reload the theme manager
	err = h.themeManager.LoadDefaultTheme(ctx)
	if err != nil {
		return fmt.Errorf("failed to reload theme: %w", err)
	}

	fmt.Println(styles.Success.Render(fmt.Sprintf("✨ Theme set to: %s", selectedTheme.DisplayName)))
	return nil
}

// RunThemePreview shows a preview of a theme
func (h *ThemeHandler) RunThemePreview(cmd *cobra.Command, args []string) error {
	if h.themeManager == nil {
		return fmt.Errorf("theme system not available (database not found)")
	}

	ctx := context.Background()
	var previewTheme theme.Theme

	if len(args) > 0 {
		// Preview specific theme
		themeName := args[0]
		dbTheme, err := h.themeManager.GetThemeByName(ctx, themeName)
		if err != nil {
			return fmt.Errorf("theme '%s' not found", themeName)
		}

		// Get theme colors
		colors, err := h.themeManager.GetThemeColors(ctx, dbTheme.ID)
		if err != nil {
			return fmt.Errorf("failed to get theme colors: %w", err)
		}

		previewTheme = theme.Theme{
			ID:          dbTheme.ID,
			Name:        dbTheme.Name,
			DisplayName: dbTheme.DisplayName,
			Colors:      make(map[string]string),
		}

		if dbTheme.Description.Valid {
			previewTheme.Description = dbTheme.Description.String
		}

		for _, color := range colors {
			previewTheme.Colors[color.ColorKey] = color.ColorValue
		}
	} else {
		// Preview current theme
		current := h.themeManager.GetCurrentTheme()
		if current == nil {
			return fmt.Errorf("no current theme set")
		}
		previewTheme = *current
	}

	// Create temporary theme manager for preview
	tempManager := h.themeManager
	tempManager.SetCurrentThemeForPreview(&previewTheme)

	// Show preview
	previewStyles := tempManager.GetStyles()
	palette := tempManager.GetPalette()

	fmt.Println(previewStyles.Header.Render(fmt.Sprintf("🎨 Theme Preview: %s", previewTheme.DisplayName)))
	fmt.Println()
	fmt.Println(previewTheme.Description)
	fmt.Println()

	// Color palette preview
	fmt.Println(previewStyles.Subheader.Render("Color Palette:"))
	fmt.Printf("• Primary: %s\n", colorBlock(palette.Primary, "Primary"))
	fmt.Printf("• Secondary: %s\n", colorBlock(palette.Secondary, "Secondary"))
	fmt.Printf("• Accent: %s\n", colorBlock(palette.Accent, "Accent"))
	fmt.Printf("• Success: %s\n", colorBlock(palette.Success, "Success"))
	fmt.Printf("• Warning: %s\n", colorBlock(palette.Warning, "Warning"))
	fmt.Printf("• Error: %s\n", colorBlock(palette.Error, "Error"))
	fmt.Printf("• Info: %s\n", colorBlock(palette.Info, "Info"))
	fmt.Println()

	// Component preview
	fmt.Println(previewStyles.Subheader.Render("Component Styles:"))
	fmt.Println(previewStyles.Success.Render("✅ Success message"))
	fmt.Println(previewStyles.Warning.Render("⚠️  Warning message"))
	fmt.Println(previewStyles.Error.Render("❌ Error message"))
	fmt.Println(previewStyles.Info.Render("ℹ️  Info message"))
	fmt.Println()

	banner := previewStyles.Border.Render("🚀 Station Banner Example")
	fmt.Println(banner)

	return nil
}

// RunThemeSelect runs interactive theme selection
func (h *ThemeHandler) RunThemeSelect(cmd *cobra.Command, args []string) error {
	if h.themeManager == nil {
		return fmt.Errorf("theme system not available (database not found)")
	}

	ctx := context.Background()
	themes, err := h.themeManager.ListThemes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list themes: %w", err)
	}

	current := h.themeManager.GetCurrentTheme()
	currentName := ""
	if current != nil {
		currentName = current.Name
	}

	// Create theme items
	items := make([]list.Item, len(themes))
	for i, t := range themes {
		desc := "Custom theme"
		if t.Description.Valid {
			desc = t.Description.String
		}
		if t.IsBuiltIn.Valid && t.IsBuiltIn.Bool {
			desc += " (built-in)"
		}

		items[i] = themeItem{
			name:        t.Name,
			displayName: t.DisplayName,
			description: desc,
			isDefault:   t.Name == currentName,
			isBuiltIn:   t.IsBuiltIn.Valid && t.IsBuiltIn.Bool,
		}
	}

	const defaultWidth = 20
	const listHeight = 14

	l := list.New(items, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = "Choose a theme"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	m := themeSelectModel{list: l, manager: h.themeManager}

	program := tea.NewProgram(m)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	// Apply the selected theme
	if final, ok := finalModel.(themeSelectModel); ok && final.choice != "" {
		return h.RunThemeSet(cmd, []string{final.choice})
	}

	return nil
}

// Helper function to create colored blocks for preview
func colorBlock(color, label string) string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(color)).
		Foreground(lipgloss.Color("#ffffff")).
		Padding(0, 1)
	return style.Render(label)
}
