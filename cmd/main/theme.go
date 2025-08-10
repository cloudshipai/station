package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
)

// Theme command definitions
var (
	themeCmd = &cobra.Command{
		Use:   "theme",
		Short: "Manage UI themes",
		Long:  "List, preview, and set UI themes for Station",
	}

	themeListCmd = &cobra.Command{
		Use:   "list",
		Short: "List available themes",
		RunE:  runThemeList,
	}

	themeSetCmd = &cobra.Command{
		Use:   "set [theme-name]",
		Short: "Set the active theme",
		Args:  cobra.ExactArgs(1),
		RunE:  runThemeSet,
	}

	themePreviewCmd = &cobra.Command{
		Use:   "preview [theme-name]",
		Short: "Preview a theme",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runThemePreview,
	}

	themeSelectCmd = &cobra.Command{
		Use:   "select",
		Short: "Interactive theme selector",
		RunE:  runThemeSelect,
	}
)

// runThemeList lists all available themes
func runThemeList(cmd *cobra.Command, args []string) error {
	themeHandler := handlers.NewThemeHandler(themeManager)
	return themeHandler.RunThemeList(cmd, args)
}

// runThemeSet sets the active theme for the current user
func runThemeSet(cmd *cobra.Command, args []string) error {
	themeHandler := handlers.NewThemeHandler(themeManager)
	return themeHandler.RunThemeSet(cmd, args)
}

// runThemePreview shows a preview of a theme
func runThemePreview(cmd *cobra.Command, args []string) error {
	themeHandler := handlers.NewThemeHandler(themeManager)
	return themeHandler.RunThemePreview(cmd, args)
}

// runThemeSelect runs interactive theme selection
func runThemeSelect(cmd *cobra.Command, args []string) error {
	themeHandler := handlers.NewThemeHandler(themeManager)
	return themeHandler.RunThemeSelect(cmd, args)
}