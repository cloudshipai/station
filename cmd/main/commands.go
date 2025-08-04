package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	
	"station/internal/db"
	"station/internal/db/repositories"
)

// Command definitions
var (
	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "Start the Station server",
		Long:  "Start all Station services: SSH admin interface, MCP server, and REST API",
		RunE:  runServe,
	}

	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize Station configuration",
		Long:  "Generate encryption keys and create configuration files in XDG config directory",
		RunE:  runInit,
	}

	loadCmd = &cobra.Command{
		Use:   "load [file|url]",
		Short: "Load MCP configuration with intelligent template processing",
		Long: `Load and process MCP configurations with multiple modes:

• No args: Open interactive editor for pasting MCP template configuration
• GitHub URL: Extract configuration from README and use TurboTax wizard  
• File path: Load configuration from specified file
• File path + --detect: Use AI to detect placeholders and generate forms
• -e/--editor: Open editor to paste template, then detect and generate forms

Interactive Editor Features:
• Paste any MCP configuration template into the editor
• AI automatically detects template variables ({{VAR}}, YOUR_KEY, <path>, etc.)
• Interactive form to fill in variable values securely
• Saves to file-based configuration system in specified environment

Examples:
  stn load                                    # Open interactive editor (default env)
  stn load --env production                   # Open editor, save to production env
  stn load config.json --detect              # Load with AI placeholder detection
  stn load -e --env staging                  # Open editor for staging environment
  stn load https://github.com/user/mcp-repo  # GitHub discovery with wizard`,
		RunE: runLoad,
	}

	mcpAddCmd = &cobra.Command{
		Use:   "add",
		Short: "Add a single MCP server to an existing configuration",
		Long:  "Add a single MCP server to an existing configuration by specifying config ID or name",
		RunE:  runMCPAdd,
	}

	blastoffCmd = &cobra.Command{
		Use:    "blastoff",
		Short:  "🚀 Epic retro station blastoff animation",
		Long:   "Watch an amazing retro ASCII animation of Station blasting off into space!",
		RunE:   runBlastoff,
		Hidden: true, // Hidden easter egg command
	}

	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Launch Station TUI interface",
		Long:  "Launch the Station terminal user interface directly without SSH",
		RunE:  runUI,
	}

	mcpCmd = &cobra.Command{
		Use:   "mcp",
		Short: "MCP management commands",
		Long:  "Manage MCP configurations and tools",
	}

	mcpListCmd = &cobra.Command{
		Use:   "list",
		Short: "List MCP configurations",
		Long:  "List all MCP configurations for the specified environment",
		RunE:  runMCPList,
	}

	mcpToolsCmd = &cobra.Command{
		Use:   "tools",
		Short: "List available MCP tools",
		Long:  "List all available MCP tools, optionally filtered",
		RunE:  runMCPTools,
	}

	mcpDeleteCmd = &cobra.Command{
		Use:   "delete <config-id>",
		Short: "Delete an MCP configuration",
		Long:  "Delete an MCP configuration and all associated tools by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runMCPDelete,
	}

	mcpSyncCmd = &cobra.Command{
		Use:   "sync <environment>",
		Short: "Sync file-based configs to database",
		Long:  "Declaratively sync all file-based MCP configs for an environment to the database, removing orphaned agent tools",
		Args:  cobra.ExactArgs(1),
		RunE:  runMCPSync,
	}

	mcpStatusCmd = &cobra.Command{
		Use:   "status [environment]",
		Short: "Show MCP configuration status",
		Long:  "Display validation status table showing agents, registered MCP configs, and their sync status",
		RunE:  runMCPStatus,
	}

	// Webhook commands
	webhookCmd = &cobra.Command{
		Use:   "webhook",
		Short: "Webhook management commands",
		Long:  "Manage webhook endpoints and delivery settings",
	}

	webhookListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all webhooks",
		Long:  "Display all registered webhook endpoints with their status",
		RunE:  runWebhookList,
	}

	webhookCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new webhook",
		Long:  "Create a new webhook endpoint for receiving notifications",
		RunE:  runWebhookCreate,
	}

	webhookDeleteCmd = &cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Delete a webhook",
		Long:  "Delete a webhook endpoint by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhookDelete,
	}

	webhookShowCmd = &cobra.Command{
		Use:   "show <webhook-id>",
		Short: "Show webhook details",
		Long:  "Display detailed information about a specific webhook",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhookShow,
	}

	webhookEnableCmd = &cobra.Command{
		Use:   "enable <webhook-id>",
		Short: "Enable a webhook",
		Long:  "Enable a webhook to start receiving notifications",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhookEnable,
	}

	webhookDisableCmd = &cobra.Command{
		Use:   "disable <webhook-id>",
		Short: "Disable a webhook",
		Long:  "Disable a webhook to stop receiving notifications",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhookDisable,
	}

	webhookDeliveriesCmd = &cobra.Command{
		Use:   "deliveries [webhook-id]",
		Short: "Show webhook deliveries",
		Long:  "Display webhook delivery history. Optionally filter by webhook ID",
		RunE:  runWebhookDeliveries,
	}

	webhookTestCmd = &cobra.Command{
		Use:   "test <endpoint-url>",
		Short: "Test webhook by sending POST to endpoint",
		Long:  "Send a test agent_run_completed webhook payload to the specified endpoint URL",
		Args:  cobra.ExactArgs(1),
		RunE:  runWebhookTest,
	}

	// Settings commands
	settingsCmd = &cobra.Command{
		Use:   "settings",
		Short: "Settings management commands",
		Long:  "Manage system settings and configurations",
	}

	settingsListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all settings",
		Long:  "Display all system settings and their values",
		RunE:  runSettingsList,
	}

	settingsGetCmd = &cobra.Command{
		Use:   "get <key>",
		Short: "Get a setting value",
		Long:  "Get the value of a specific setting",
		Args:  cobra.ExactArgs(1),
		RunE:  runSettingsGet,
	}

	settingsSetCmd = &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a setting value",
		Long:  "Set the value of a specific setting",
		Args:  cobra.ExactArgs(2),
		RunE:  runSettingsSet,
	}
)

func runServe(cmd *cobra.Command, args []string) error {
	// Check if configuration exists
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Configuration not found. Please run 'station init' first.\n")
		fmt.Printf("Expected config file: %s\n", configFile)
		return fmt.Errorf("configuration not initialized")
	}

	// Validate encryption key
	encryptionKey := viper.GetString("encryption_key")
	if encryptionKey == "" {
		return fmt.Errorf("encryption key not found in configuration. Please run 'station init' to generate keys")
	}

	fmt.Printf("🚀 Starting Station...\n")
	fmt.Printf("SSH Port: %d\n", viper.GetInt("ssh_port"))
	fmt.Printf("MCP Port: %d\n", viper.GetInt("mcp_port"))
	fmt.Printf("API Port: %d\n", viper.GetInt("api_port"))
	fmt.Printf("Database: %s\n", viper.GetString("database_url"))
	
	// Set environment variables for the main application to use
	os.Setenv("ENCRYPTION_KEY", encryptionKey)
	os.Setenv("SSH_PORT", fmt.Sprintf("%d", viper.GetInt("ssh_port")))
	os.Setenv("MCP_PORT", fmt.Sprintf("%d", viper.GetInt("mcp_port")))
	os.Setenv("API_PORT", fmt.Sprintf("%d", viper.GetInt("api_port")))
	os.Setenv("DATABASE_URL", viper.GetString("database_url"))
	if viper.GetBool("debug") {
		os.Setenv("STATION_DEBUG", "true")
	}

	// Import and run the main server code
	return runMainServer()
}

func runInit(cmd *cobra.Command, args []string) error {
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")

	fmt.Printf("🔧 Initializing Station configuration...\n")
	fmt.Printf("Config directory: %s\n", configDir)

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate encryption key
	fmt.Printf("🔐 Generating encryption key...\n")
	key, encryptionKey, err := generateEncryptionKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	_ = key // Use the raw key if needed elsewhere

	// Set default configuration
	viper.Set("encryption_key", encryptionKey)
	viper.Set("ssh_port", 2222)
	viper.Set("mcp_port", 3000)
	viper.Set("api_port", 8080)
	viper.Set("database_url", filepath.Join(configDir, "station.db"))
	viper.Set("ssh_host_key_path", "./ssh_host_key")
	viper.Set("admin_username", "admin")
	viper.Set("debug", false)
	viper.Set("local_mode", true) // Default to local mode

	// Write configuration file
	viper.SetConfigFile(configFile)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Initialize database and run migrations
	fmt.Printf("🗄️  Initializing database...\n")
	databasePath := filepath.Join(configDir, "station.db")
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Initialize default environment and file-based config structure
	fmt.Printf("🌍 Setting up default environment...\n")
	if err := initDefaultEnvironment(database); err != nil {
		return fmt.Errorf("failed to initialize default environment: %w", err)
	}

	fmt.Printf("✅ Configuration initialized successfully!\n")
	fmt.Printf("📁 Config file: %s\n", configFile)
	fmt.Printf("🗄️  Database: %s\n", databasePath)
	fmt.Printf("🔑 Encryption key generated and saved securely\n")
	fmt.Printf("📁 File config structure: %s\n", filepath.Join(configDir, "environments", "default"))
	fmt.Printf("\n🚀 You can now run 'station serve' to launch the server\n")
	fmt.Printf("🔗 Connect via SSH: ssh admin@localhost -p 2222\n")
	fmt.Printf("\n📖 Next steps:\n")
	fmt.Printf("   • Run 'stn mcp init' to create sample configurations\n")
	fmt.Printf("   • Run 'stn mcp env list' to see your environments\n")

	return nil
}

// initDefaultEnvironment creates the default environment and file structure
func initDefaultEnvironment(database db.Database) error {
	repos := repositories.New(database)
	
	// Create default environment if it doesn't exist
	defaultEnv, err := repos.Environments.GetByName("default")
	if err != nil {
		// Environment doesn't exist, create it
		description := "Default environment for development and testing"
		defaultEnv, err = repos.Environments.Create("default", &description, 1) // Default user ID 1
		if err != nil {
			return fmt.Errorf("failed to create default environment: %w", err)
		}
		fmt.Printf("   ✅ Created default environment (ID: %d)\n", defaultEnv.ID)
	} else {
		fmt.Printf("   ℹ️  Default environment already exists (ID: %d)\n", defaultEnv.ID)
	}
	
	// Create file config directory structure in XDG config dir
	xdgConfigDir := getXDGConfigDir()
	configDir := filepath.Join(xdgConfigDir, "environments", "default")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Create variables directory
	varsDir := filepath.Join(xdgConfigDir, "vars")
	if err := os.MkdirAll(varsDir, 0755); err != nil {
		return fmt.Errorf("failed to create variables directory: %w", err)
	}
	
	fmt.Printf("   📁 Created file config directories\n")
	
	return nil
}