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
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(bundleCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(runsCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(developCmd)
	rootCmd.AddCommand(blastoffCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(mockCmd)

	// Legacy file-config handlers removed - use 'stn sync' instead

	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)

	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpDeleteCmd)
	mcpCmd.AddCommand(mcpStatusCmd)

	// Unified bundle command replaces the old template system
	// bundleCmd is standalone and doesn't need subcommands

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
	agentCmd.AddCommand(agentRunCmd)
	agentCmd.AddCommand(agentDeleteCmd)

	runsCmd.AddCommand(runsListCmd)
	runsCmd.AddCommand(runsInspectCmd)

	settingsCmd.AddCommand(settingsListCmd)
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)

	// Init command flags
	initCmd.Flags().Bool("replicate", false, "Set up Litestream database replication for production deployments")
	initCmd.Flags().StringP("config", "c", "", "Path to configuration file (sets workspace to config file's directory)")
	initCmd.Flags().Bool("ship", false, "Bootstrap with ship CLI MCP integration for filesystem access")
	initCmd.Flags().String("provider", "", "AI provider (openai, gemini, custom) - if not set, shows interactive selection")
	initCmd.Flags().String("model", "", "AI model name - if not set, shows interactive selection based on provider")
	initCmd.Flags().String("base-url", "", "Base URL for OpenAI-compatible endpoints (e.g., http://localhost:11434/v1 for Ollama)")
	initCmd.Flags().BoolP("yes", "y", false, "Use defaults without interactive prompts")
	initCmd.Flags().String("cloudship-key", "", "CloudShip registration key for station management and monitoring")
	initCmd.Flags().String("cloudship-endpoint", "lighthouse.cloudship.ai:443", "CloudShip Lighthouse gRPC endpoint")
	initCmd.Flags().String("otel-endpoint", "", "OpenTelemetry OTLP endpoint for telemetry export (e.g., http://localhost:4318)")
	initCmd.Flags().Bool("telemetry", false, "Enable telemetry collection and export (default: false)")

	// Serve command flags
	serveCmd.Flags().Int("ssh-port", 2222, "SSH server port")
	serveCmd.Flags().Int("mcp-port", 8586, "MCP server port")
	serveCmd.Flags().Int("api-port", 8585, "API server port")
	serveCmd.Flags().String("database", "station.db", "Database file path")
	serveCmd.Flags().Bool("debug", false, "Enable debug logging")
	serveCmd.Flags().Bool("local", false, "Run in local mode (single user, no authentication)")
	serveCmd.Flags().Bool("dev", false, "Enable development mode with GenKit reflection server (default: disabled)")

	// MCP Add command flags
	mcpAddCmd.Flags().StringP("environment", "e", "default", "Environment to add configuration to")
	mcpAddCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")

	// MCP command flags
	mcpListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpListCmd.Flags().String("environment", "", "Environment to list configs from (default: all environments)")

	mcpToolsCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpToolsCmd.Flags().String("environment", "default", "Environment to list tools from")
	mcpToolsCmd.Flags().String("filter", "", "Filter tools by name or description")

	mcpDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpDeleteCmd.Flags().String("environment", "default", "Environment to delete from")
	mcpDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")

	// Sync command flags (top-level)
	syncCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	syncCmd.Flags().Bool("dry-run", false, "Show what would be synced without making changes")
	syncCmd.Flags().Bool("validate", false, "Validate configurations only without syncing")
	syncCmd.Flags().BoolP("interactive", "i", true, "Prompt for missing variables (default: true)")

	// Bootstrap command flags
	bootstrapCmd.Flags().Bool("openai", false, "Bootstrap with OpenAI provider (runs stn init --ship --provider openai --model gpt-5)")

	// Develop command flags
	developCmd.Flags().String("env", "default", "Environment to load agents and MCP configs from")
	developCmd.Flags().Int("port", 4000, "Port to run the Genkit development server on")
	developCmd.Flags().String("ai-model", "", "AI model to use (overrides Station config)")
	developCmd.Flags().String("ai-provider", "", "AI provider to use (gemini, openai, anthropic)")
	developCmd.Flags().Bool("verbose", false, "Enable verbose logging for debugging")
	syncCmd.Flags().BoolP("verbose", "v", false, "Verbose output showing all operations")

	mcpStatusCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	mcpStatusCmd.Flags().String("environment", "default", "Environment to check status for (default shows all)")

	// Template command flags
	templateCreateCmd.Flags().String("name", "", "Bundle name (defaults to directory name)")
	templateCreateCmd.Flags().String("author", "", "Bundle author")
	templateCreateCmd.Flags().String("description", "", "Bundle description")
	templateCreateCmd.Flags().String("env", "", "Create bundle from existing environment (scans MCP configs, agents, and variables)")

	templateBundleCmd.Flags().String("output", "", "Output path for package (defaults to bundle-name.tar.gz)")
	templateBundleCmd.Flags().Bool("validate", true, "Validate bundle before packaging")

	templatePublishCmd.Flags().String("registry", "default", "Registry to publish to")
	templatePublishCmd.Flags().Bool("skip-validation", false, "Skip validation before publishing")

	templateInstallCmd.Flags().String("registry", "", "Registry to install from (defaults to searching all)")
	templateInstallCmd.Flags().Bool("force", false, "Force reinstallation if bundle already exists")

	templateListCmd.Flags().String("registry", "", "Filter by registry name")
	templateListCmd.Flags().String("search", "", "Search term for bundle names/descriptions")

	// Agent command flags
	agentListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentListCmd.Flags().String("env", "", "Filter agents by environment name or ID")
	agentShowCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentShowCmd.Flags().String("env", "default", "Environment name for the agent")
	agentRunCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentRunCmd.Flags().String("env", "default", "Environment name for the agent")
	agentRunCmd.Flags().Bool("tail", false, "Follow the agent execution with real-time output")
	agentDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")

	// Runs command flags
	runsListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	runsListCmd.Flags().Int("limit", 50, "Maximum number of runs to display")
	runsInspectCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	runsInspectCmd.Flags().BoolP("verbose", "v", false, "Show detailed run information including tool calls, execution steps, and metadata")

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
	viper.SetDefault("telemetry_enabled", false)
	viper.SetDefault("ai_provider", "gemini")
	viper.SetDefault("ai_model", "gemini-2.5-flash")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Use XDG config directory
		configDir := getWorkspacePath()
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

		// CloudShip integration is now handled by the Lighthouse client
		// in individual command contexts (stdio, serve, CLI modes)
	}
}

func initTheme() {
	// Try to initialize theme manager with database
	// For CLI commands, we'll use fallback themes if database is not available
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getWorkspacePath()
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
	// Check for explicit STATION_CONFIG_DIR override first
	// This should be the complete path including 'station' suffix
	if configDir := os.Getenv("STATION_CONFIG_DIR"); configDir != "" {
		return configDir
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, "station")
}

func getWorkspacePath() string {
	// Check for explicit STATION_CONFIG_DIR override first
	// This is needed for container environments
	if configDir := os.Getenv("STATION_CONFIG_DIR"); configDir != "" {
		return configDir
	}

	// Check if workspace is configured via viper
	if workspace := viper.GetString("workspace"); workspace != "" {
		return workspace
	}

	// Fall back to XDG path for backward compatibility
	return getXDGConfigDir()
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

	// CloudShip cleanup is now handled by individual command contexts
}
