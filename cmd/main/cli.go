package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
)

// runMCPList implements the "station mcp list" command
func runMCPList(cmd *cobra.Command, args []string) error {
	mcpHandler := handlers.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPList(cmd, args)
}

// runMCPTools implements the "station mcp tools" command
func runMCPTools(cmd *cobra.Command, args []string) error {
	mcpHandler := handlers.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPTools(cmd, args)
}

// runMCPDelete implements the "station mcp delete" command
func runMCPDelete(cmd *cobra.Command, args []string) error {
	mcpHandler := handlers.NewMCPHandler(themeManager)
	return mcpHandler.RunMCPDelete(cmd, args)
}

// runMCPAdd implements the "station mcp add" command
func runMCPAdd(cmd *cobra.Command, args []string) error {
	// Check if interactive mode is requested
	interactive, _ := cmd.Flags().GetBool("interactive")
	
	if interactive {
		return runMCPAddInteractive(cmd, args)
	}
	
	return runMCPAddFlags(cmd, args)
}

// runMCPAddFlags handles flag-based mode
func runMCPAddFlags(cmd *cobra.Command, args []string) error {
	// Get flags
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	configID, _ := cmd.Flags().GetString("config-id")
	serverName, _ := cmd.Flags().GetString("server-name")
	command, _ := cmd.Flags().GetString("command")
	argsSlice, _ := cmd.Flags().GetStringSlice("args")
	envVars, _ := cmd.Flags().GetStringToString("env")

	// Validate required flags
	if configID == "" {
		return fmt.Errorf("--config-id is required")
	}
	if serverName == "" {
		return fmt.Errorf("--server-name is required")
	}
	if command == "" {
		return fmt.Errorf("--command is required")
	}

	// Show banner
	styles := getCLIStyles(themeManager)
	banner := styles.Banner.Render("🔧 Add MCP Server to Configuration")
	fmt.Println(banner)

	// Create spinner model with server configuration
	model := handlers.NewSpinnerModelWithServerConfig(
		fmt.Sprintf("Adding server '%s' to configuration '%s'...", serverName, configID),
		configID, serverName, command, argsSlice, envVars, environment, endpoint, themeManager)

	// Start the spinner
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run spinner: %w", err)
	}

	// Check results
	final := finalModel.(handlers.SpinnerModel)
	if final.GetError() != nil {
		fmt.Println(getCLIStyles(themeManager).Error.Render("❌ Failed to add server: " + final.GetError().Error()))
		return final.GetError()
	}

	// Show success banner
	showSuccessBanner(fmt.Sprintf("Server '%s' successfully added to configuration!", serverName), themeManager)
	fmt.Printf("Result: %s\n", final.GetResult())

	return nil
}

// runMCPAddInteractive handles interactive mode with beautiful forms
func runMCPAddInteractive(cmd *cobra.Command, args []string) error {
	// Show retro banner
	retroBanner := getCLIStyles(themeManager).Banner.Render("🎛️  Interactive MCP Server Configuration")
	fmt.Println(retroBanner)
	fmt.Println(getCLIStyles(themeManager).Info.Render("Use arrow keys to navigate, Enter to select, Ctrl+C to exit"))
	fmt.Println()

	// Get basic flags that might be pre-set
	endpoint, _ := cmd.Flags().GetString("endpoint")
	environment, _ := cmd.Flags().GetString("environment")
	
	// Create the interactive form model
	formModel := handlers.NewMCPAddForm(endpoint, environment, themeManager)
	
	// Run the interactive form
	program := tea.NewProgram(formModel, tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run interactive form: %w", err)
	}
	
	// Check if user cancelled
	final := finalModel.(*handlers.MCPAddFormModel)
	if final.IsCancelled() {
		fmt.Println(getCLIStyles(themeManager).Info.Render("Operation cancelled"))
		return nil
	}
	
	// Show completion banner with collected data
	showSuccessBanner("MCP Server Configuration Complete!", themeManager)
	fmt.Printf("Adding server: %s\n", getCLIStyles(themeManager).Success.Render(final.GetServerName()))
	fmt.Printf("To config: %s\n", getCLIStyles(themeManager).Success.Render(final.GetConfigID()))
	fmt.Printf("Command: %s %v\n", getCLIStyles(themeManager).Success.Render(final.GetCommand()), final.GetArgs())
	
	// Now execute the actual addition
	mcpHandler := handlers.NewMCPHandler(themeManager)
	result, err := mcpHandler.AddServerToConfig(final.GetConfigID(), final.GetServerName(), final.GetCommand(), final.GetArgs(), final.GetEnvVars(), final.GetEnvironment(), final.GetEndpoint())
	if err != nil {
		fmt.Println(getCLIStyles(themeManager).Error.Render("❌ Failed to add server: " + err.Error()))
		return err
	}
	
	fmt.Printf("Result: %s\n", result)
	return nil
}