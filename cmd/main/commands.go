package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"station/internal/auth"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/internal/services"
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
		Long: `Initialize Station with configuration files and interactive AI provider setup.

By default, sets up Station for local development with SQLite database and guides you through
selecting an AI provider (OpenAI, Gemini, or custom) with an interactive interface.

Features:
• Interactive AI provider and model selection with beautiful TUI
• Environment variable detection (OPENAI_API_KEY, GEMINI_API_KEY)
• Support for custom providers (Anthropic, Ollama, local models)
• Optional database replication setup with Litestream
• Ship CLI integration for instant filesystem MCP tools

Use --provider and --model flags to skip interactive setup.
Use --yes flag to use sensible defaults without any prompts.
Use --replicate flag to also configure Litestream for database replication to cloud storage.
Use --ship flag to bootstrap with ship CLI MCP integration for filesystem access.`,
		RunE: runInit,
	}

	mcpAddCmd = &cobra.Command{
		Use:   "add [config-name]",
		Short: "Add a new MCP configuration via editor",
		Long: `Add a new MCP configuration by opening your default editor.

This command opens your default text editor (vim, nano, code, etc.) where you can:
1. Paste an MCP configuration from documentation or examples
2. Use template variables that will be detected automatically
3. Save and close to create the configuration file

The configuration will be saved as <config-name>.json in your current environment.
When you run 'stn sync', you'll be prompted for any template variables found.

Examples:
  stn mcp add filesystem          # Creates filesystem.json via editor
  stn mcp add --env prod database # Creates database.json in prod environment`,
		RunE: runMCPAdd,
	}

	blastoffCmd = &cobra.Command{
		Use:    "blastoff",
		Short:  "🚀 Epic retro station blastoff animation",
		Long:   "Watch an amazing retro ASCII animation of Station blasting off into space!",
		RunE:   runBlastoff,
		Hidden: true, // Hidden easter egg command
	}

	bootstrapCmd = &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap Station with pre-configured agents and tools",
		Long: `Bootstrap Station with a complete setup including agents and MCP tools.

This command runs a full quickstart setup:
• Runs stn init --ship --provider openai --model gpt-5
• Creates hello world agent (no tools)
• Creates playwright agent with @playwright/mcp integration
• Installs devops-security-bundle from registry
• Syncs all environments

Perfect for getting started quickly with a fully configured Station instance.`,
		RunE: runBootstrap,
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

	mcpStatusCmd = &cobra.Command{
		Use:   "status [environment]",
		Short: "Show MCP configuration status",
		Long:  "Display validation status table showing agents, registered MCP configs, and their sync status",
		RunE:  runMCPStatus,
	}

	// Template bundle commands
	templateCmd = &cobra.Command{
		Use:   "template",
		Short: "Template bundle management commands",
		Long:  "Manage template bundles for quick MCP server configuration deployment",
	}

	templateCreateCmd = &cobra.Command{
		Use:   "create <path>",
		Short: "Create a new template bundle",
		Long:  "Create a new template bundle with scaffolding for MCP server configurations, or create from an existing environment",
		Example: `  stn template create my-bundle                    # Create empty bundle
  stn template create my-bundle --env default        # Create from default environment
  stn template create my-bundle --env production     # Create from production environment`,
		Args: cobra.ExactArgs(1),
		RunE: runTemplateCreate,
	}

	templateValidateCmd = &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate a template bundle",
		Long:  "Validate template bundle structure and check variable consistency between template and schema",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateValidate,
	}

	templateBundleCmd = &cobra.Command{
		Use:   "bundle <path>",
		Short: "Package a template bundle for distribution",
		Long:  "Create a distributable .tar.gz package from a validated template bundle",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateBundle,
	}

	templatePublishCmd = &cobra.Command{
		Use:   "publish <bundle-path>",
		Short: "Publish a template bundle to a registry",
		Long:  "Package and publish a template bundle to a specified registry",
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplatePublish,
	}

	templateInstallCmd = &cobra.Command{
		Use:   "install <bundle-name>[@version] [environment]",
		Short: "Install a template bundle from a registry",
		Long:  "Download and install a template bundle from a configured registry into the specified environment (defaults to 'default')",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runTemplateInstall,
	}

	templateListCmd = &cobra.Command{
		Use:   "list",
		Short: "List available template bundles",
		Long:  "List template bundles from configured registries",
		RunE:  runTemplateList,
	}

	templateRegistryCmd = &cobra.Command{
		Use:   "registry",
		Short: "Manage template registries",
		Long:  "Add, remove, and list configured template registries",
	}

	templateRegistryAddCmd = &cobra.Command{
		Use:   "add <name> <url>",
		Short: "Add a new template registry",
		Long:  "Add a new template registry endpoint",
		Args:  cobra.ExactArgs(2),
		RunE:  runTemplateRegistryAdd,
	}

	templateRegistryListCmd = &cobra.Command{
		Use:   "list",
		Short: "List configured registries",
		Long:  "List all configured template registries",
		RunE:  runTemplateRegistryList,
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

	// Top-level sync command for all configurations
	syncCmd = &cobra.Command{
		Use:   "sync [environment]",
		Short: "Sync all file-based configurations",
		Long: `Declaratively synchronize all file-based configurations to the database.
This includes agents (.prompt files), MCP configurations, and environment settings.`,
		Example: `  stn sync                    # Sync all environments (interactive)
  stn sync production         # Sync specific environment  
  stn sync --dry-run          # Show what would change
  stn sync --validate         # Validate configurations only
  stn sync --no-interactive   # Skip prompting for missing variables`,
		RunE: runSync,
	}

	// Development playground command
	developCmd = &cobra.Command{
		Use:   "develop",
		Short: "Launch Genkit development playground with Station agents and tools",
		Long: `Start a Genkit development server with all agents and MCP tools loaded for interactive testing.

This command creates a development environment where you can:
• Test and iterate on your agents in the Genkit UI
• Access all MCP tools from your environment
• Use dotprompt templates with live reloading
• Debug agent execution with full tool calling support

The development server loads all file-based configurations from the specified environment
and makes them available in the Genkit developer UI for interactive testing.`,
		Example: `  stn develop                    # Start playground with default environment
  stn develop --env production   # Start playground with production environment
  stn develop --port 4000        # Start on custom port
  stn develop --ai-model gemini-2.5-flash      # Override AI model`,
		RunE: runDevelop,
	}
)

func runServe(cmd *cobra.Command, args []string) error {
	// Set GenKit environment based on --dev flag
	devMode, _ := cmd.Flags().GetBool("dev")
	if !devMode && os.Getenv("GENKIT_ENV") == "" {
		os.Setenv("GENKIT_ENV", "prod") // Disable reflection server by default
	}

	// Check if configuration exists
	configDir := getWorkspacePath()
	configFile := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Configuration not found. Please run 'station init' first.\n")
		fmt.Printf("Expected config file: %s\n", configFile)
		return fmt.Errorf("configuration not initialized")
	}

	// Validate encryption key - check config first, then environment variables
	encryptionKey := viper.GetString("encryption_key")
	if encryptionKey == "" {
		// Check for environment variable as fallback (for Docker containers)
		encryptionKey = os.Getenv("STATION_ENCRYPTION_KEY")
		if encryptionKey == "" {
			return fmt.Errorf("encryption key not found in configuration. Please run 'station init' to generate keys or set STATION_ENCRYPTION_KEY environment variable")
		}
		// Set the encryption key in viper for other components to use
		viper.Set("encryption_key", encryptionKey)
	}

	// Load other essential configuration from environment variables if not in config
	if envProvider := os.Getenv("STATION_AI_PROVIDER"); envProvider != "" && viper.GetString("ai_provider") == "" {
		viper.Set("ai_provider", envProvider)
	}
	if envModel := os.Getenv("STATION_AI_MODEL"); envModel != "" && viper.GetString("ai_model") == "" {
		viper.Set("ai_model", envModel)
	}
	if envAPIPort := os.Getenv("STATION_API_PORT"); envAPIPort != "" && viper.GetInt("api_port") == 0 {
		viper.Set("api_port", envAPIPort)
	}
	if envMCPPort := os.Getenv("STATION_MCP_PORT"); envMCPPort != "" && viper.GetInt("mcp_port") == 0 {
		viper.Set("mcp_port", envMCPPort)
	}
	if envSSHPort := os.Getenv("STATION_SSH_PORT"); envSSHPort != "" && viper.GetInt("ssh_port") == 0 {
		viper.Set("ssh_port", envSSHPort)
	}
	if envLocalMode := os.Getenv("STATION_LOCAL_MODE"); envLocalMode != "" && !viper.IsSet("local_mode") {
		viper.Set("local_mode", envLocalMode == "true")
	}
	if envDebug := os.Getenv("STATION_DEBUG"); envDebug != "" && !viper.IsSet("debug") {
		viper.Set("debug", envDebug == "true")
	}
	if envTelemetry := os.Getenv("STATION_TELEMETRY_ENABLED"); envTelemetry != "" && !viper.IsSet("telemetry_enabled") {
		viper.Set("telemetry_enabled", envTelemetry == "true")
	}
	if envAdminUsername := os.Getenv("STATION_ADMIN_USERNAME"); envAdminUsername != "" && viper.GetString("admin_username") == "" {
		viper.Set("admin_username", envAdminUsername)
	}

	// CloudShip configuration via environment variables for Docker runtime
	if envCloudShipEnabled := os.Getenv("STN_CLOUDSHIP_ENABLED"); envCloudShipEnabled != "" && !viper.IsSet("cloudship.enabled") {
		viper.Set("cloudship.enabled", envCloudShipEnabled == "true")
	}
	if envCloudShipKey := os.Getenv("STN_CLOUDSHIP_KEY"); envCloudShipKey != "" && viper.GetString("cloudship.registration_key") == "" {
		viper.Set("cloudship.registration_key", envCloudShipKey)
	}
	if envCloudShipEndpoint := os.Getenv("STN_CLOUDSHIP_ENDPOINT"); envCloudShipEndpoint != "" && viper.GetString("cloudship.endpoint") == "" {
		viper.Set("cloudship.endpoint", envCloudShipEndpoint)
	}
	if envCloudShipStationID := os.Getenv("STN_CLOUDSHIP_STATION_ID"); envCloudShipStationID != "" && viper.GetString("cloudship.station_id") == "" {
		viper.Set("cloudship.station_id", envCloudShipStationID)
	}

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
	// Check if custom config file path is provided
	configPath, _ := cmd.Flags().GetString("config")
	if configPath != "" {
		// Set workspace to the directory containing the config file
		workspaceDir := filepath.Dir(filepath.Clean(configPath))
		workspaceDir, _ = filepath.Abs(workspaceDir)

		// Set the workspace in viper for the session
		viper.Set("workspace", workspaceDir)

		// Also set database path relative to workspace
		databasePath := filepath.Join(workspaceDir, "station.db")
		viper.Set("database_url", databasePath)
	}

	// Config file always stays in secure default location
	configDir := getXDGConfigDir() // Use XDG, not workspace
	configFile := filepath.Join(configDir, "config.yaml")

	// Workspace directory for content (environments, bundles, etc.)
	workspaceDir := getWorkspacePath()

	// Check if replication setup is requested
	replicationSetup, _ := cmd.Flags().GetBool("replicate")

	// Check if ship setup is requested
	shipSetup, _ := cmd.Flags().GetBool("ship")

	// Check CloudShip AI configuration
	cloudshipaiKey, _ := cmd.Flags().GetString("cloudshipai")
	cloudshipaiEndpoint, _ := cmd.Flags().GetString("cloudshipai_endpoint")

	// Check environment variable if flag not provided
	if cloudshipaiKey == "" {
		cloudshipaiKey = os.Getenv("CLOUDSHIPAI_REGISTRATION_KEY")
	}

	// Check OTEL configuration
	otelEndpoint, _ := cmd.Flags().GetString("otel-endpoint")
	telemetryEnabled, _ := cmd.Flags().GetBool("telemetry")

	// Check environment variable if flag not provided
	if otelEndpoint == "" {
		otelEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}

	// Check existing config to avoid interactive mode
	_, err := os.Stat(configFile)
	configExists := err == nil

	fmt.Printf("🔧 Initializing Station configuration...\n")
	fmt.Printf("Config file: %s\n", configFile)
	if workspaceDir != configDir {
		fmt.Printf("Workspace directory: %s\n", workspaceDir)
	}

	if configExists {
		fmt.Printf("⚠️  Configuration already exists - skipping interactive setup\n")
	}

	if replicationSetup {
		fmt.Printf("🔄 Replication mode: Setting up Litestream for database replication\n")
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create workspace directory if different from config directory
	if workspaceDir != configDir {
		if err := os.MkdirAll(workspaceDir, 0755); err != nil {
			return fmt.Errorf("failed to create workspace directory: %w", err)
		}
	}

	// Provider and model setup
	var providerConfig *ProviderConfig
	provider, _ := cmd.Flags().GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	apiKey, _ := cmd.Flags().GetString("api-key")
	baseURL, _ := cmd.Flags().GetString("base-url")
	useDefaults, _ := cmd.Flags().GetBool("yes")

	// If --api-key flag is provided, set it as environment variable for this session
	if apiKey != "" {
		os.Setenv("STN_AI_API_KEY", apiKey)
	}

	if !configExists && !useDefaults && provider == "" {
		// Interactive provider setup
		providerConfig, err = setupProviderInteractively()
		if err != nil {
			defaultProvider, defaultModel := getDefaultProvider()
			fmt.Printf("⚠️  Provider setup failed: %v\n", err)
			fmt.Printf("💡 Using defaults: provider=%s, model=%s\n", defaultProvider, defaultModel)
			providerConfig = &ProviderConfig{Provider: defaultProvider, Model: defaultModel, BaseURL: baseURL}
		}
	} else {
		// Use flags, defaults, or skip if config exists
		if provider == "" {
			provider, _ = getDefaultProvider()
		}
		if model == "" {
			if provider == "openai" {
				recommended := config.GetRecommendedOpenAIModels()
				model = recommended["cost_effective"]
			} else {
				model = "gemini-2.5-flash"
			}
		}
		providerConfig = &ProviderConfig{Provider: provider, Model: model, BaseURL: baseURL}

		if !configExists {
			fmt.Printf("🤖 Using AI provider: %s\n", providerConfig.Provider)
			fmt.Printf("   Model: %s\n", providerConfig.Model)
			if providerConfig.BaseURL != "" {
				fmt.Printf("   Base URL: %s\n", providerConfig.BaseURL)
			}
		}
	}

	// Set default configuration
	viper.Set("ssh_port", 2222)
	viper.Set("mcp_port", 8586)
	viper.Set("api_port", 8585)
	viper.Set("ssh_host_key_path", "./ssh_host_key")
	viper.Set("admin_username", "admin")
	viper.Set("debug", false)
	viper.Set("local_mode", true) // Default to local mode

	// Set AI provider configuration
	if !configExists {
		viper.Set("ai_provider", providerConfig.Provider)
		viper.Set("ai_model", providerConfig.Model)
		if providerConfig.BaseURL != "" {
			viper.Set("ai_base_url", providerConfig.BaseURL)
		}
	}

	// Set workspace and database paths
	// Check if database URL is already set via environment variable
	if existingDatabaseURL := os.Getenv("STATION_DATABASE_URL"); existingDatabaseURL != "" {
		viper.Set("database_url", existingDatabaseURL)
	} else if workspaceDir != configDir {
		// Custom workspace: database goes to workspace, workspace setting saved
		viper.Set("workspace", workspaceDir)
		viper.Set("database_url", filepath.Join(workspaceDir, "station.db"))
	} else {
		// Default: database in config directory, no workspace setting needed
		viper.Set("database_url", filepath.Join(configDir, "station.db"))
	}

	// Set CloudShip AI configuration if provided
	if cloudshipaiKey != "" {
		viper.Set("cloudshipai.registration_key", cloudshipaiKey)
		viper.Set("cloudshipai.endpoint", cloudshipaiEndpoint)
		viper.Set("cloudshipai.enabled", true)
		fmt.Printf("🌩️  CloudShip AI integration enabled\n")
		fmt.Printf("   Endpoint: %s\n", cloudshipaiEndpoint)
	}

	// Set OTEL configuration
	viper.Set("telemetry_enabled", telemetryEnabled)
	if otelEndpoint != "" {
		viper.Set("otel_endpoint", otelEndpoint)
		fmt.Printf("📊 OpenTelemetry integration enabled\n")
		fmt.Printf("   OTLP Endpoint: %s\n", otelEndpoint)
	}

	// Generate encryption key if not already present
	if viper.GetString("encryption_key") == "" {
		encryptionKey, err := auth.GenerateAPIKey()
		if err != nil {
			return fmt.Errorf("failed to generate encryption key: %w", err)
		}
		// Remove the "sk-" prefix as this is an encryption key, not an API key
		encryptionKey = encryptionKey[3:] // Remove "sk-" prefix
		viper.Set("encryption_key", encryptionKey)
		fmt.Printf("🔑 Encryption key generated and saved securely\n")
	}

	// Write configuration file
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Initialize database and run migrations
	fmt.Printf("🗄️  Initializing database...\n")
	databasePath := viper.GetString("database_url")
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = database.Close() }()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Initialize default environment and file-based config structure
	fmt.Printf("🌍 Setting up default environment...\n")
	if err := initDefaultEnvironment(database); err != nil {
		return fmt.Errorf("failed to initialize default environment: %w", err)
	}

	// Set up ship CLI MCP integration if requested
	shipSetupSucceeded := false
	if shipSetup {
		fmt.Printf("🚢 Setting up ship CLI MCP integration...\n")
		if err := setupShipIntegration(workspaceDir); err != nil {
			logging.Info("⚠️  Ship CLI setup failed: %v", err)
			logging.Info("💡 Alternative: Use 'stn mcp add filesystem' to manually configure filesystem tools")
			logging.Info("   Or install ship CLI manually: curl -fsSL https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash")
			// Don't return error, continue with normal init
		} else {
			shipSetupSucceeded = true
		}
	}

	// Set up Litestream replication configuration if requested
	if replicationSetup {
		if err := setupReplicationConfiguration(configDir); err != nil {
			return fmt.Errorf("failed to set up replication configuration: %w", err)
		}
	}

	fmt.Printf("\n🎉 Station initialized successfully!\n\n")
	fmt.Printf("📁 Config file: %s\n", configFile)
	fmt.Printf("🗄️  Database: %s\n", viper.GetString("database_url"))
	fmt.Printf("📁 File config structure: %s\n", filepath.Join(workspaceDir, "environments", "default"))

	if !configExists {
		fmt.Printf("🤖 AI Provider: %s\n", providerConfig.Provider)
		fmt.Printf("🧠 Model: %s\n", providerConfig.Model)
		if providerConfig.BaseURL != "" {
			fmt.Printf("🔗 Base URL: %s\n", providerConfig.BaseURL)
		}
	}

	// Ship setup completed (no sync needed since no MCP configs created)
	if shipSetupSucceeded {
		logging.Info("🚢 Ship CLI installed and ready for MCP integration")
		logging.Info("💡 Use 'stn bootstrap --openai' for quick start with agents and tools")
	}

	if replicationSetup {
		fmt.Printf("🔄 Replication setup: Litestream configuration created at %s\n", filepath.Join(configDir, "litestream.yml"))
		fmt.Printf("🐳 Docker files: Check examples/deployments/ for production deployment\n")
		fmt.Printf("\n🌟 You can now run 'station serve' locally or deploy with replication\n")
		fmt.Printf("🔗 Local: ssh admin@localhost -p 2222\n")
		fmt.Printf("🚢 Production: docker-compose -f examples/deployments/docker-compose/docker-compose.production.yml up\n")
		fmt.Printf("\n📖 Replication setup steps:\n")
		fmt.Printf("   • Edit %s to configure your cloud storage\n", filepath.Join(configDir, "litestream.yml"))
		fmt.Printf("   • Set LITESTREAM_S3_BUCKET and credentials in deployment environment\n")
		fmt.Printf("   • See docs/GITOPS-DEPLOYMENT.md for complete guide\n")
	} else {
		fmt.Printf("🔗 Connect via SSH: ssh admin@localhost -p 2222\n")
		fmt.Printf("\n📖 Next steps:\n")
		fmt.Printf("   • Run 'stn up' to start Station and connect Claude Code/Cursor\n")
		fmt.Printf("   • Visit http://localhost:8585 to add MCP tools and manage bundles\n")
		fmt.Printf("   • Use 'stn mcp list' to see your MCP configurations\n")
		fmt.Printf("   • Create and manage agents via Claude Code/Cursor using Station's MCP tools\n")
	}

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
	xdgConfigDir := getWorkspacePath()
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

// setupReplicationConfiguration creates Litestream configuration and deployment examples
func setupReplicationConfiguration(configDir string) error {
	fmt.Printf("🔄 Setting up Litestream replication configuration...\n")

	// Create Litestream configuration with clear user guidance
	litestreamConfigContent := `# Litestream Database Replication Configuration
# Configure where and how Station's SQLite database is replicated
# 
# REQUIRED: Set environment variables in your deployment:
#   LITESTREAM_S3_BUCKET=your-backup-bucket-name
#   LITESTREAM_S3_ACCESS_KEY_ID=your-access-key
#   LITESTREAM_S3_SECRET_ACCESS_KEY=your-secret-key
#   LITESTREAM_S3_REGION=us-east-1 (optional, defaults to us-east-1)

dbs:
  - path: /data/station.db
    replicas:
      # Primary: AWS S3 (recommended for production)
      - type: s3
        bucket: ${LITESTREAM_S3_BUCKET}
        path: station-db
        region: ${LITESTREAM_S3_REGION:-us-east-1}
        access-key-id: ${LITESTREAM_S3_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_S3_SECRET_ACCESS_KEY}
        sync-interval: 10s    # How often to sync changes
        retention: 24h        # How long to keep old snapshots
        
      # Alternative: Google Cloud Storage
      # - type: gcs
      #   bucket: ${LITESTREAM_GCS_BUCKET}
      #   path: station-db
      #   service-account-json-path: /secrets/gcs-service-account.json
      #   sync-interval: 10s
      #   retention: 24h
        
      # Alternative: Azure Blob Storage  
      # - type: abs
      #   bucket: ${LITESTREAM_ABS_BUCKET}
      #   path: station-db
      #   account-name: ${LITESTREAM_ABS_ACCOUNT_NAME}
      #   account-key: ${LITESTREAM_ABS_ACCOUNT_KEY}
      #   sync-interval: 10s
      #   retention: 24h
        
      # Development: Local file backup (not for production)
      # - type: file
      #   path: /backup/station-db-backup
      #   sync-interval: 30s
      #   retention: 168h  # 7 days`

	litestreamConfigPath := filepath.Join(configDir, "litestream.yml")
	if err := os.WriteFile(litestreamConfigPath, []byte(litestreamConfigContent), 0644); err != nil {
		return fmt.Errorf("failed to create litestream.yml: %w", err)
	}
	fmt.Printf("   ✅ Created %s\n", litestreamConfigPath)

	// Create .env.example for Docker Compose
	envExampleContent := `# Litestream Configuration for GitOps Deployments
LITESTREAM_S3_BUCKET=your-station-backups
LITESTREAM_S3_REGION=us-east-1
LITESTREAM_S3_ACCESS_KEY_ID=your-access-key
LITESTREAM_S3_SECRET_ACCESS_KEY=your-secret-key

# Station Configuration
STATION_ENV=production
PORT=8585

# AI Provider APIs
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key

# Optional: Additional MCP server credentials
# GITHUB_TOKEN=ghp_your-github-token
`

	envExamplePath := filepath.Join(configDir, ".env.example")
	if err := os.WriteFile(envExamplePath, []byte(envExampleContent), 0644); err != nil {
		return fmt.Errorf("failed to create .env.example: %w", err)
	}
	fmt.Printf("   ✅ Created %s\n", envExamplePath)

	// Create gitops directory with basic structure
	gitopsDir := filepath.Join(configDir, "gitops")
	if err := os.MkdirAll(gitopsDir, 0755); err != nil {
		return fmt.Errorf("failed to create gitops directory: %w", err)
	}

	// Create basic agent template structure
	agentTemplateDir := filepath.Join(gitopsDir, "agent-templates", "sample-agent")
	if err := os.MkdirAll(agentTemplateDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent template directory: %w", err)
	}

	// Create README with GitOps instructions
	readmeContent := `# Station GitOps Setup

This directory contains your Station GitOps configuration created by \` + "`" + `stn init --gitops\` + "`" + `.

## Quick Start

1. **Set up cloud storage credentials:**
   \` + "`" + `\` + "`" + `\` + "`" + `bash
   cp .env.example .env
   # Edit .env with your S3/GCS/Azure credentials
   \` + "`" + `\` + "`" + `\` + "`" + `

2. **Deploy locally with Litestream:**
   \` + "`" + `\` + "`" + `\` + "`" + `bash
   docker-compose -f examples/deployments/docker-compose/docker-compose.production.yml up
   \` + "`" + `\` + "`" + `\` + "`" + `

3. **Deploy to Kubernetes:**
   \` + "`" + `\` + "`" + `\` + "`" + `bash
   kubectl apply -f examples/deployments/kubernetes/station-deployment.yml
   \` + "`" + `\` + "`" + `\` + "`" + `

## Files Created

- \` + "`" + `litestream.yml\` + "`" + ` - Database replication configuration
- \` + "`" + `.env.example\` + "`" + ` - Environment variables template
- \` + "`" + `gitops/\` + "`" + ` - Directory structure for agent templates

## Documentation

See \` + "`" + `docs/GITOPS-DEPLOYMENT.md\` + "`" + ` for the complete GitOps deployment guide.`

	readmePath := filepath.Join(configDir, "README-GITOPS.md")
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create GitOps README: %w", err)
	}
	fmt.Printf("   ✅ Created %s\n", readmePath)
	fmt.Printf("   📁 Created GitOps directory structure at %s\n", gitopsDir)

	return nil
}

// runSync handles the top-level sync command
func runSync(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name is required")
	}

	environment := args[0]
	return runSyncForEnvironment(environment)
}

// setupShipIntegration installs latest ship CLI only (no MCP configuration)
func setupShipIntegration(workspaceDir string) error {
	// Always install latest ship CLI
	logging.Info("   📦 Installing latest ship CLI...")
	if err := installShipCLI(); err != nil {
		return fmt.Errorf("failed to install ship CLI: %w", err)
	}

	logging.Info("   ✅ Ship CLI installed")
	return nil
}

// installShipCLI installs the ship CLI using the official installer
func installShipCLI() error {
	// Add timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", "curl -fsSL https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("ship CLI installation timed out after 2 minutes. Please install manually or check your internet connection")
	}

	return err
}

// runSyncForEnvironment runs sync for a specific environment using DeclarativeSync service
func runSyncForEnvironment(environment string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	// Create sync service
	syncer := services.NewDeclarativeSync(repos, cfg)

	fmt.Printf("Starting sync for environment: %s\n", environment)

	// Sync the specific environment
	result, err := syncer.SyncEnvironment(context.Background(), environment, services.SyncOptions{
		DryRun:      false,
		Validate:    false,
		Interactive: true,
		Verbose:     false,
		Confirm:     false,
	})

	if err != nil {
		return fmt.Errorf("sync failed for environment %s: %w", environment, err)
	}

	// Display results
	fmt.Printf("\nSync completed for environment: %s\n", environment)
	fmt.Printf("  Agents: %d processed, %d synced\n", result.AgentsProcessed, result.AgentsSynced)
	fmt.Printf("  MCP Servers: %d processed, %d connected\n", result.MCPServersProcessed, result.MCPServersConnected)

	if result.ValidationErrors > 0 {
		fmt.Printf("  ⚠️  Validation Errors: %d\n", result.ValidationErrors)
		return fmt.Errorf("sync completed with %d validation errors", result.ValidationErrors)
	} else {
		fmt.Printf("  ✅ All configurations synced successfully\n")
	}

	return nil
}

// runBootstrap implements the bootstrap command with complete Station setup
func runBootstrap(cmd *cobra.Command, args []string) error {
	openaiSetup, _ := cmd.Flags().GetBool("openai")
	if !openaiSetup {
		return fmt.Errorf("bootstrap requires a provider flag. Use --openai for OpenAI setup")
	}

	workspaceDir := getWorkspacePath()


	// Step 1: Run stn init --ship --provider openai --model gpt-5
	fmt.Printf("📦 Step 1/6: Initializing Station with OpenAI and Ship CLI...\n")
	initCmd := exec.Command("stn", "init", "--ship", "--provider", "openai", "--model", "gpt-5", "--yes")
	initCmd.Stdout = os.Stdout
	initCmd.Stderr = os.Stderr
	if err := initCmd.Run(); err != nil {
		return fmt.Errorf("failed to run stn init: %w", err)
	}
	fmt.Printf("✅ Station initialized with OpenAI and Ship CLI\n\n")

	// Step 2: Create hello world agent (no tools)
	fmt.Printf("🤖 Step 2/6: Creating Hello World agent...\n")
	if err := createHelloWorldAgent(workspaceDir); err != nil {
		return fmt.Errorf("failed to create hello world agent: %w", err)
	}
	fmt.Printf("✅ Hello World agent created\n\n")

	// Step 3: Create playwright MCP config
	fmt.Printf("🎭 Step 3/6: Setting up Playwright MCP integration...\n")
	if err := createPlaywrightMCP(workspaceDir); err != nil {
		return fmt.Errorf("failed to create playwright MCP config: %w", err)
	}
	fmt.Printf("✅ Playwright MCP integration configured\n\n")

	// Step 4: Create playwright agent
	fmt.Printf("🤖 Step 4/6: Creating Playwright agent...\n")
	if err := createPlaywrightAgent(workspaceDir); err != nil {
		return fmt.Errorf("failed to create playwright agent: %w", err)
	}
	fmt.Printf("✅ Playwright agent created\n\n")

	// Step 5: Install security bundle
	fmt.Printf("🔒 Step 5/6: Installing DevOps Security Bundle...\n")
	bundleCmd := exec.Command("stn", "bundle", "install", "https://github.com/cloudshipai/registry/releases/latest/download/devops-security-bundle.tar.gz", "security")
	bundleCmd.Stdout = os.Stdout
	bundleCmd.Stderr = os.Stderr
	if err := bundleCmd.Run(); err != nil {
		return fmt.Errorf("failed to install security bundle: %w", err)
	}
	fmt.Printf("✅ DevOps Security Bundle installed\n\n")

	// Step 6: Sync all environments
	fmt.Printf("🔄 Step 6/6: Syncing all environments...\n")

	// Sync default environment
	syncDefaultCmd := exec.Command("stn", "sync", "default")
	syncDefaultCmd.Stdout = os.Stdout
	syncDefaultCmd.Stderr = os.Stderr
	if err := syncDefaultCmd.Run(); err != nil {
		return fmt.Errorf("failed to sync default environment: %w", err)
	}

	// Sync security environment
	syncSecurityCmd := exec.Command("stn", "sync", "security")
	syncSecurityCmd.Stdout = os.Stdout
	syncSecurityCmd.Stderr = os.Stderr
	if err := syncSecurityCmd.Run(); err != nil {
		return fmt.Errorf("failed to sync security environment: %w", err)
	}

	fmt.Printf("✅ All environments synced\n\n")

	fmt.Printf("🎉 Bootstrap complete! Your Station is ready with:\n")
	fmt.Printf("   • OpenAI integration (gpt-5)\n")
	fmt.Printf("   • Ship CLI filesystem tools\n")
	fmt.Printf("   • Hello World agent (default env - basic tasks)\n")
	fmt.Printf("   • Playwright agent (default env - web automation)\n")
	fmt.Printf("   • DevOps Security Bundle (security env - comprehensive security tools)\n\n")
	fmt.Printf("   • Run 'stn serve' to start Station\n")
	fmt.Printf("   • Connect via SSH: ssh admin@localhost -p 2222\n")
	fmt.Printf("   • Or run 'stn stdio' for MCP integration\n")

	return nil
}

// createHelloWorldAgent creates a simple hello world agent with no tools
func createHelloWorldAgent(workspaceDir string) error {
	agentDir := filepath.Join(workspaceDir, "environments", "default", "agents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	agentContent := `---
metadata:
  name: "Hello World Agent"
  description: "A friendly agent for basic tasks and greetings"
  tags: ["basic", "hello-world", "simple"]
model: gpt-5
max_steps: 3
tools: []
---

{{role "system"}}
You are a helpful and friendly Hello World agent. You specialize in:
• Greeting users warmly
• Providing simple explanations
• Creating basic code examples
• Helping with introductory tasks

Keep your responses concise and helpful. Always maintain a positive, encouraging tone.

{{role "user"}}
{{userInput}}
`

	agentPath := filepath.Join(agentDir, "Hello World Agent.prompt")
	return os.WriteFile(agentPath, []byte(agentContent), 0644)
}

// createPlaywrightMCP creates the playwright MCP configuration
func createPlaywrightMCP(workspaceDir string) error {
	defaultEnvDir := filepath.Join(workspaceDir, "environments", "default")
	mcpConfigPath := filepath.Join(defaultEnvDir, "playwright.json")

	playwrightConfig := `{
  "mcpServers": {
    "playwright": {
      "command": "npx",
      "args": [
        "@playwright/mcp@latest"
      ]
    }
  }
}`

	return os.WriteFile(mcpConfigPath, []byte(playwrightConfig), 0644)
}

// createPlaywrightAgent creates an agent that uses playwright tools
func createPlaywrightAgent(workspaceDir string) error {
	agentDir := filepath.Join(workspaceDir, "environments", "default", "agents")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agents directory: %w", err)
	}

	agentContent := `---
metadata:
  name: "Playwright Agent"
  description: "Web automation agent using Playwright for browser interactions, testing, and scraping"
  tags: ["web", "automation", "testing", "playwright", "browser"]
model: gpt-5
max_steps: 8
tools:
  - "__browser_take_screenshot"
  - "__browser_navigate" 
  - "__browser_click"
  - "__browser_fill_form"
  - "__browser_type"
  - "__browser_wait_for"
  - "__browser_evaluate"
  - "__browser_snapshot"
  - "__browser_hover"
  - "__browser_drag"
  - "__browser_tabs"
  - "__browser_console_messages"
---

{{role "system"}}
You are an expert web automation agent that uses Playwright to interact with web pages. You excel at:

• **Web Navigation**: Opening pages, following links, handling navigation
• **Element Interaction**: Clicking buttons, filling forms, typing text
• **Data Extraction**: Scraping content, taking screenshots, getting page text
• **Web Testing**: Verifying page elements, testing user workflows
• **Browser Automation**: Handling dynamic content, waiting for elements

**Key Capabilities:**
- Take screenshots of web pages for visual verification
- Navigate to any URL and interact with page elements
- Fill out forms and submit data
- Extract text content and data from pages
- Wait for elements to load before interacting
- Execute JavaScript in the browser context

Always explain what you're doing with the web page and provide clear feedback about the results of your actions.

{{role "user"}}
{{userInput}}
`

	agentPath := filepath.Join(agentDir, "Playwright Agent.prompt")
	return os.WriteFile(agentPath, []byte(agentContent), 0644)
}

// Settings command functions

func runSettingsList(cmd *cobra.Command, args []string) error {
	// Get database path from config
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getWorkspacePath()
		databasePath = filepath.Join(configDir, "station.db")
	}

	// Initialize database connection
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize repositories
	repos := repositories.New(database)

	// Get all settings
	settings, err := repos.Settings.GetAll()
	if err != nil {
		return fmt.Errorf("failed to list settings: %w", err)
	}

	if len(settings) == 0 {
		fmt.Println("No settings found.")
		return nil
	}

	fmt.Printf("Settings (%d total):\n\n", len(settings))
	for _, setting := range settings {
		fmt.Printf("Key: %s\n", setting.Key)
		fmt.Printf("Value: %s\n", setting.Value)
		if setting.Description != nil {
			fmt.Printf("Description: %s\n", *setting.Description)
		}
		fmt.Printf("Updated: %s\n", setting.UpdatedAt.Format(time.RFC3339))
		fmt.Println()
	}

	return nil
}

func runSettingsGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("setting key is required")
	}

	key := args[0]

	// Get database path from config
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getWorkspacePath()
		databasePath = filepath.Join(configDir, "station.db")
	}

	// Initialize database connection
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize repositories
	repos := repositories.New(database)

	// Get the setting
	setting, err := repos.Settings.GetByKey(key)
	if err != nil {
		return fmt.Errorf("setting '%s' not found", key)
	}

	fmt.Printf("Key: %s\n", setting.Key)
	fmt.Printf("Value: %s\n", setting.Value)
	if setting.Description != nil {
		fmt.Printf("Description: %s\n", *setting.Description)
	}
	fmt.Printf("Created: %s\n", setting.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", setting.UpdatedAt.Format(time.RFC3339))

	return nil
}

func runSettingsSet(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("setting key and value are required")
	}

	key := args[0]
	value := args[1]
	description, _ := cmd.Flags().GetString("description")

	// Get database path from config
	databasePath := viper.GetString("database_url")
	if databasePath == "" {
		configDir := getWorkspacePath()
		databasePath = filepath.Join(configDir, "station.db")
	}

	// Initialize database connection
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Initialize repositories
	repos := repositories.New(database)

	// Set the setting
	err = repos.Settings.Set(key, value, description)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	fmt.Printf("Setting '%s' has been set to '%s'\n", key, value)
	if description != "" {
		fmt.Printf("Description: %s\n", description)
	}

	return nil
}
