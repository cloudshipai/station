package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config command definitions
var (
	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Manage Station configuration",
		Long:  "View and edit Station configuration settings",
	}

	configShowCmd = &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	}

	configEditCmd = &cobra.Command{
		Use:   "edit",
		Short: "Edit configuration file",
		RunE:  runConfigEdit,
	}
)

func runConfigShow(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no configuration file found. Run 'station init' first")
	}

	fmt.Printf("📋 Station Configuration\n")
	fmt.Printf("======================\n")
	fmt.Printf("Config file: %s\n\n", configFile)
	
	fmt.Printf("Server Ports:\n")
	fmt.Printf("  SSH Port: %d\n", viper.GetInt("ssh_port"))
	fmt.Printf("  MCP Port: %d\n", viper.GetInt("mcp_port"))
	fmt.Printf("  API Port: %d\n", viper.GetInt("api_port"))
	
	fmt.Printf("\nDatabase:\n")
	fmt.Printf("  Database URL: %s\n", viper.GetString("database_url"))
	
	fmt.Printf("\nSSH Configuration:\n")
	fmt.Printf("  Host Key Path: %s\n", viper.GetString("ssh_host_key_path"))
	fmt.Printf("  Admin Username: %s\n", viper.GetString("admin_username"))
	
	fmt.Printf("\nSecurity:\n")
	if viper.GetString("encryption_key") != "" {
		fmt.Printf("  Encryption Key: [CONFIGURED]\n")
	} else {
		fmt.Printf("  Encryption Key: [NOT SET]\n")
	}
	
	fmt.Printf("\nDebug Mode: %v\n", viper.GetBool("debug"))
	fmt.Printf("Telemetry: %v (opt-out via TELEMETRY_ENABLED=false or config file)\n", viper.GetBool("telemetry_enabled"))

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no configuration file found. Run 'station init' first")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano" // fallback editor
	}

	fmt.Printf("Opening config file with %s: %s\n", editor, configFile)
	
	// Execute editor command
	command := fmt.Sprintf("%s %s", editor, configFile)
	return runCommand(command)
}

func runCommand(command string) error {
	// This is a simplified version - in production you'd want proper command execution
	fmt.Printf("Would execute: %s\n", command)
	fmt.Printf("For now, manually edit: %s\n", viper.ConfigFileUsed())
	return nil
}