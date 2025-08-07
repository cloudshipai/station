package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"station/internal/config"
	"station/internal/db"
	"station/internal/logging"
	"station/internal/telemetry"
	"station/internal/theme"
	"station/internal/version"
	"station/cmd/main/handlers/file_config"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile          string
	themeManager     *theme.ThemeManager
	telemetryService *telemetry.TelemetryService
	rootCmd          = &cobra.Command{
		Use:   "stn",
		Short: "Station - AI Agent Management Platform",
		Long: `Station is a secure, self-hosted platform for managing AI agents with MCP tool integration.
It provides a retro terminal interface for system administration and agent management.`,
		Version: version.GetVersionString(),
	}
)

func init() {
	cobra.OnInitialize(initConfig)
	cobra.OnInitialize(initLogging)
	cobra.OnInitialize(initTheme)
	cobra.OnInitialize(initTelemetry)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/station/config.yaml)")
	
	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(keyCmd)
	rootCmd.AddCommand(loadCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(runsCmd)
	rootCmd.AddCommand(webhookCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(blastoffCmd)
	rootCmd.AddCommand(bannerCmd)
	
	// Initialize file config handler and integrate with mcp commands
	fileConfigHandler := file_config.NewFileConfigHandler()
	fileConfigHandler.RegisterMCPCommands(mcpCmd)
	
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(themeCmd)
	
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeSetCmd)
	themeCmd.AddCommand(themePreviewCmd)
	themeCmd.AddCommand(themeSelectCmd)
	
	keyCmd.AddCommand(keyGenerateCmd)
	keyCmd.AddCommand(keySetCmd)
	keyCmd.AddCommand(keyRotateCmd)
	keyCmd.AddCommand(keyStatusCmd)
	keyCmd.AddCommand(keyFinishRotationCmd)

	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpDeleteCmd)
	mcpCmd.AddCommand(mcpSyncCmd)
	mcpCmd.AddCommand(mcpStatusCmd)
	
	
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
	agentCmd.AddCommand(agentRunCmd)
	agentCmd.AddCommand(agentDeleteCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentExportCmd)
	agentCmd.AddCommand(agentImportCmd)
	
	runsCmd.AddCommand(runsListCmd)
	runsCmd.AddCommand(runsInspectCmd)
	
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookCreateCmd)
	webhookCmd.AddCommand(webhookDeleteCmd)
	webhookCmd.AddCommand(webhookShowCmd)
	webhookCmd.AddCommand(webhookEnableCmd)
	webhookCmd.AddCommand(webhookDisableCmd)
	webhookCmd.AddCommand(webhookDeliveriesCmd)
	webhookCmd.AddCommand(webhookTestCmd)
	
	settingsCmd.AddCommand(settingsListCmd)
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)
	
	// Serve command flags
	serveCmd.Flags().Int("ssh-port", 2222, "SSH server port")
	serveCmd.Flags().Int("mcp-port", 3000, "MCP server port") 
	serveCmd.Flags().Int("api-port", 8080, "API server port")
	serveCmd.Flags().String("database", "station.db", "Database file path")
	serveCmd.Flags().Bool("debug", false, "Enable debug logging")
	serveCmd.Flags().Bool("local", false, "Run in local mode (single user, no authentication)")
	
	// Load command flags
	loadCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	loadCmd.Flags().String("environment", "default", "Environment name to upload to")
	loadCmd.Flags().String("env", "", "Environment name (creates if doesn't exist, overrides --environment)")
	loadCmd.Flags().String("config-name", "", "Name for the MCP configuration")
	loadCmd.Flags().Bool("detect", false, "Use AI to intelligently detect and generate forms for placeholders")
	loadCmd.Flags().BoolP("editor", "e", false, "Open editor to paste template configuration")
	
	// MCP Add command flags
	mcpAddCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpAddCmd.Flags().String("environment", "default", "Environment name")
	mcpAddCmd.Flags().String("config-id", "", "Config ID to add server to (format: id or configname)")
	mcpAddCmd.Flags().String("server-name", "", "Name for the new MCP server")
	mcpAddCmd.Flags().String("command", "", "Command to run the MCP server")
	mcpAddCmd.Flags().StringSlice("args", []string{}, "Arguments for the MCP server command")
	mcpAddCmd.Flags().StringToString("env", map[string]string{}, "Environment variables (key=value)")
	mcpAddCmd.Flags().BoolP("interactive", "i", false, "Interactive mode with forms")
	
	// MCP command flags
	mcpListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpListCmd.Flags().String("environment", "", "Environment to list configs from (default: all environments)")
	
	mcpToolsCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpToolsCmd.Flags().String("environment", "default", "Environment to list tools from")
	mcpToolsCmd.Flags().String("filter", "", "Filter tools by name or description")
	
	mcpDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpDeleteCmd.Flags().String("environment", "default", "Environment to delete from")
	mcpDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")
	
	mcpSyncCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpSyncCmd.Flags().Bool("dry-run", false, "Show what would be synced without making changes")
	mcpSyncCmd.Flags().Bool("force", false, "Force sync even if no changes detected")
	
	mcpStatusCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpStatusCmd.Flags().String("environment", "default", "Environment to check status for (default shows all)")
	
	
	// Agent command flags
	agentListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentListCmd.Flags().String("env", "", "Filter agents by environment name or ID")
	agentShowCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentRunCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentRunCmd.Flags().Bool("tail", false, "Follow the agent execution with real-time output")
	agentDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")
	agentCreateCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentCreateCmd.Flags().String("env", "default", "Environment name or ID for the agent")
	agentCreateCmd.Flags().String("domain", "", "Domain context for the agent (e.g., devops, data-science)")
	agentCreateCmd.Flags().String("schedule", "on-demand", "Schedule preference (on-demand, daily, weekly, custom)")
	
	// Runs command flags
	runsListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	runsListCmd.Flags().Int("limit", 50, "Maximum number of runs to display")
	runsInspectCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	runsInspectCmd.Flags().BoolP("verbose", "v", false, "Show detailed run information including tool calls, execution steps, and metadata")
	
	// Webhook command flags
	webhookListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookCreateCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookCreateCmd.Flags().String("name", "", "Webhook name (required)")
	webhookCreateCmd.Flags().String("url", "", "Webhook URL (required)")
	webhookCreateCmd.Flags().String("secret", "", "Webhook secret for signature validation")
	webhookCreateCmd.Flags().StringSlice("events", []string{"agent_run_completed"}, "Events to subscribe to")
	webhookCreateCmd.Flags().StringToString("headers", map[string]string{}, "Custom headers (key=value)")
	webhookCreateCmd.Flags().Int("timeout", 30, "Timeout in seconds")
	webhookCreateCmd.Flags().Int("retries", 3, "Number of retry attempts")
	webhookCreateCmd.Flags().BoolP("interactive", "i", false, "Interactive mode with forms")
	webhookDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")
	webhookShowCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookEnableCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookDisableCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookDeliveriesCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	webhookDeliveriesCmd.Flags().Int("limit", 50, "Maximum number of deliveries to display")
	
	// Settings command flags
	settingsListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	settingsGetCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	settingsSetCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	settingsSetCmd.Flags().String("description", "", "Description for the setting")
	
	// Bind flags to viper
	viper.BindPFlag("ssh_port", serveCmd.Flags().Lookup("ssh-port"))
	viper.BindPFlag("mcp_port", serveCmd.Flags().Lookup("mcp-port"))
	viper.BindPFlag("api_port", serveCmd.Flags().Lookup("api-port"))
	viper.BindPFlag("database_url", serveCmd.Flags().Lookup("database"))
	viper.BindPFlag("debug", serveCmd.Flags().Lookup("debug"))
	viper.BindPFlag("local_mode", serveCmd.Flags().Lookup("local"))
	
	// Set default values
	viper.SetDefault("telemetry_enabled", true)
	viper.SetDefault("ai_provider", "gemini")
	viper.SetDefault("ai_model", "gemini-2.0-flash-exp")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Use XDG config directory
		configDir := getXDGConfigDir()
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("STATION")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		fmt.Printf("Using config file: %s\n", viper.ConfigFileUsed())
	}
}

func initTheme() {
	// Try to initialize theme manager with database
	// For CLI commands, we'll use fallback themes if database is not available
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getXDGConfigDir()
		databasePath = filepath.Join(configDir, "station.db")
	}
	
	// Check if database file exists and is accessible
	if _, err := os.Stat(databasePath); err == nil {
		// Database exists, try to connect
		if database, err := db.New(databasePath); err == nil {
			themeManager = theme.NewThemeManager(database)
			// Try to initialize built-in themes and load default theme
			ctx := context.Background()
			themeManager.InitializeBuiltInThemes(ctx)
			themeManager.LoadDefaultTheme(ctx)
		}
	}
	
	// If themeManager is still nil, commands will use fallback themes
}

func initLogging() {
	// Load config to check debug settings
	cfg, err := config.Load()
	if err != nil {
		// If config fails to load, default to info level (debug disabled)
		logging.Initialize(false)
		return
	}
	
	// Initialize logging based on config
	logging.Initialize(cfg.Debug)
}

func initTelemetry() {
	// Load config to check telemetry settings
	cfg, err := config.Load()
	if err != nil {
		// If config fails to load, default to telemetry disabled for safety
		telemetryService = telemetry.NewTelemetryService(false)
		return
	}
	
	// Initialize telemetry service based on config
	telemetryService = telemetry.NewTelemetryService(cfg.TelemetryEnabled)
}

func getXDGConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, "station")
}

func main() {
	// Track CLI execution
	startTime := time.Now()
	var commandName, subcommandName string
	success := true
	
	// Capture command info
	if len(os.Args) > 1 {
		commandName = os.Args[1]
		if len(os.Args) > 2 {
			subcommandName = os.Args[2]
		}
	}
	
	if err := rootCmd.Execute(); err != nil {
		success = false
		fmt.Printf("Error: %v\n", err)
		
		// Track error
		if telemetryService != nil {
			telemetryService.TrackError("cli_execution", err.Error(), map[string]interface{}{
				"command":    commandName,
				"subcommand": subcommandName,
			})
		}
		
		os.Exit(1)
	}
	
	// Track successful command execution
	if telemetryService != nil {
		duration := time.Since(startTime).Milliseconds()
		telemetryService.TrackCLICommand(commandName, subcommandName, success, duration)
		// Flush to ensure events are sent immediately
		telemetryService.Flush()
	}
	
	// Cleanup telemetry
	if telemetryService != nil {
		telemetryService.Close()
	}
}