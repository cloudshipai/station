package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/webhooks"
)

// runWebhookList implements the "station webhook list" command
func runWebhookList(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookList(cmd, args)
}

// runWebhookCreate implements the "station webhook create" command
func runWebhookCreate(cmd *cobra.Command, args []string) error {
	// Check if interactive mode is requested
	interactive, _ := cmd.Flags().GetBool("interactive")
	
	if interactive {
		return runWebhookCreateInteractive(cmd, args)
	}
	
	return runWebhookCreateFlags(cmd, args)
}

// runWebhookCreateFlags handles flag-based webhook creation
func runWebhookCreateFlags(cmd *cobra.Command, args []string) error {
	// Get flags
	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	
	// Validate required flags
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	if url == "" {
		return fmt.Errorf("--url is required")
	}

	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookCreate(cmd, args)
}

// runWebhookCreateInteractive handles interactive webhook creation
func runWebhookCreateInteractive(cmd *cobra.Command, args []string) error {
	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("🪝 Interactive Webhook Creation")
	fmt.Println(banner)
	fmt.Println(styles.Info.Render("Use arrow keys to navigate, Enter to select, Ctrl+C to exit"))
	fmt.Println()

	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookCreateInteractive(cmd, args)
}

// runWebhookDelete implements the "station webhook delete" command
func runWebhookDelete(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookDelete(cmd, args)
}

// runWebhookShow implements the "station webhook show" command
func runWebhookShow(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookShow(cmd, args)
}

// runWebhookEnable implements the "station webhook enable" command
func runWebhookEnable(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookEnable(cmd, args)
}

// runWebhookDisable implements the "station webhook disable" command
func runWebhookDisable(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookDisable(cmd, args)
}

// runWebhookDeliveries implements the "station webhook deliveries" command
func runWebhookDeliveries(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunWebhookDeliveries(cmd, args)
}

// runSettingsList implements the "station settings list" command
func runSettingsList(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunSettingsList(cmd, args)
}

// runSettingsGet implements the "station settings get" command
func runSettingsGet(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunSettingsGet(cmd, args)
}

// runSettingsSet implements the "station settings set" command
func runSettingsSet(cmd *cobra.Command, args []string) error {
	webhookHandler := webhooks.NewWebhookHandler(themeManager)
	return webhookHandler.RunSettingsSet(cmd, args)
}