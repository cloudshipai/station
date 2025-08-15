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
	
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/logging"
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
		Long: `Initialize Station with configuration files and optional database replication.

By default, sets up Station for local development with SQLite database.
Use --replicate flag to also configure Litestream for database replication to cloud storage.
Use --ship flag to bootstrap with ship CLI MCP integration for filesystem access.`,
		RunE:  runInit,
	}

	loadCmd = &cobra.Command{
		Use:   "load [file|url]",
		Short: "Load MCP configuration with intelligent template processing",
		Long: `Load and process MCP configurations with automatic discovery:

• No args: Auto-discover mcp.json or .mcp.json in current directory, or open interactive editor
• GitHub URL: Extract configuration from README and use TurboTax wizard  
• File path: Load configuration from specified file
• File path + --detect: Use AI to detect placeholders and generate forms
• -e/--editor: Open editor to paste template, then detect and generate forms

Auto-Discovery:
• Looks for: mcp.json, .mcp.json, mcp-config.json, .mcp-config.json
• Loads to default environment unless --env specified
• Falls back to interactive editor if no files found

Interactive Editor Features:
• Paste any MCP configuration template into the editor
• AI automatically detects template variables ({{VAR}}, YOUR_KEY, <path>, etc.)
• Interactive form to fill in variable values securely
• Saves to file-based configuration system in specified environment

Examples:
  stn load                                    # Auto-discover config file (default env)
  stn load --env production                   # Auto-discover, load to production env
  stn load config.json --detect              # Load specific file with AI detection
  stn load -e --env staging                  # Open editor for staging environment
  stn load https://github.com/user/mcp-repo  # GitHub discovery with wizard`,
		RunE: runLoad,
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
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateCreate,
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

	// Top-level sync command for all configurations
	syncCmd = &cobra.Command{
		Use:   "sync [environment]",
		Short: "Sync all file-based configurations",
		Long:  `Declaratively synchronize all file-based configurations to the database.
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
	// Check if configuration exists
	configDir := getWorkspacePath()
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
	configDir := getXDGConfigDir()  // Use XDG, not workspace
	configFile := filepath.Join(configDir, "config.yaml")
	
	// Workspace directory for content (environments, bundles, etc.)
	workspaceDir := getWorkspacePath()
	
	// Check if replication setup is requested
	replicationSetup, _ := cmd.Flags().GetBool("replicate")
	
	// Check if ship setup is requested
	shipSetup, _ := cmd.Flags().GetBool("ship")

	fmt.Printf("🔧 Initializing Station configuration...\n")
	fmt.Printf("Config file: %s\n", configFile)
	if workspaceDir != configDir {
		fmt.Printf("Workspace directory: %s\n", workspaceDir)
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
	viper.Set("ssh_host_key_path", "./ssh_host_key")
	viper.Set("admin_username", "admin")
	viper.Set("debug", false)
	viper.Set("local_mode", true) // Default to local mode
	
	// Set workspace and database paths
	if workspaceDir != configDir {
		// Custom workspace: database goes to workspace, workspace setting saved
		viper.Set("workspace", workspaceDir)
		viper.Set("database_url", filepath.Join(workspaceDir, "station.db"))
	} else {
		// Default: database in config directory, no workspace setting needed
		viper.Set("database_url", filepath.Join(configDir, "station.db"))
	}

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

	fmt.Printf("✅ Configuration initialized successfully!\n")
	fmt.Printf("📁 Config file: %s\n", configFile)
	fmt.Printf("🗄️  Database: %s\n", viper.GetString("database_url"))
	fmt.Printf("🔑 Encryption key generated and saved securely\n")
	fmt.Printf("📁 File config structure: %s\n", filepath.Join(workspaceDir, "environments", "default"))
	
	// Run ship sync after everything is set up (only if ship setup succeeded)
	if shipSetupSucceeded {
		logging.Info("🚢 Ship integration: Filesystem MCP tools configured")
		logging.Info("🔄 Syncing MCP tools...")
		if err := runSyncForEnvironment("default"); err != nil {
			logging.Info("⚠️  Warning: Failed to sync MCP tools: %v", err)
			logging.Info("   Run 'stn sync default' manually to complete setup")
		} else {
			logging.Info("✅ Ship MCP tools synced successfully")
		}
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
		fmt.Printf("\n🚀 You can now run 'station serve' to launch the server\n")
		fmt.Printf("🔗 Connect via SSH: ssh admin@localhost -p 2222\n")
		fmt.Printf("\n📖 Next steps:\n")
		if !shipSetup {
			fmt.Printf("   • Run 'stn mcp init' to create sample configurations\n")
			fmt.Printf("   • Run 'stn init --ship' to bootstrap with ship CLI MCP integration\n")
		}
		fmt.Printf("   • Run 'stn mcp env list' to see your environments\n")
		fmt.Printf("   • Use 'stn init --replicate' for database replication setup\n")
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
PORT=8080

# AI Provider APIs
OPENAI_API_KEY=sk-your-openai-key
ANTHROPIC_API_KEY=sk-ant-your-anthropic-key

# Optional: Additional MCP server credentials
# GITHUB_TOKEN=ghp_your-github-token
# SLACK_WEBHOOK_URL=https://hooks.slack.com/your-webhook`

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
	// If no environment specified, default to "default"
	if len(args) == 0 {
		args = []string{"default"}
	}
	
	// Use the existing MCP sync functionality but make it more general
	// For now, delegate to runMCPSync but we can expand this later
	return runMCPSync(cmd, args)
}

// setupShipIntegration installs latest ship CLI and sets up MCP configuration
func setupShipIntegration(workspaceDir string) error {
	// Always install latest ship CLI
	logging.Info("   📦 Installing latest ship CLI...")
	if err := installShipCLI(); err != nil {
		return fmt.Errorf("failed to install ship CLI: %w", err)
	}
	
	// Create filesystem MCP configuration in default environment
	defaultEnvDir := filepath.Join(workspaceDir, "environments", "default")
	mcpConfigPath := filepath.Join(defaultEnvDir, "mcp.json")
	
	shipMCPConfig := `{
  "mcpServers": {
    "station": {
      "command": "ship",
      "args": ["mcp", "filesystem"]
    }
  }
}`
	
	if err := os.WriteFile(mcpConfigPath, []byte(shipMCPConfig), 0644); err != nil {
		return fmt.Errorf("failed to create ship MCP configuration: %w", err)
	}
	
	// Create variables.yml file for template variables
	variablesPath := filepath.Join(defaultEnvDir, "variables.yml")
	defaultVariables := `# Environment variables for ship MCP integration  
# Add any template variables needed for your ship MCP server here
# Example:
# ROOT_PATH: "/home/user/projects"
# ALLOWED_DIRECTORIES: "/tmp,/home/user/workspace"
`
	
	if err := os.WriteFile(variablesPath, []byte(defaultVariables), 0644); err != nil {
		return fmt.Errorf("failed to create variables.yml: %w", err)
	}
	
	logging.Info("   ✅ Ship MCP integration configured")
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

// runSyncForEnvironment runs sync for a specific environment
func runSyncForEnvironment(environment string) error {
	// Reload viper configuration to ensure database path is set correctly
	configDir := getXDGConfigDir()
	configFile := filepath.Join(configDir, "config.yaml")
	viper.SetConfigFile(configFile)
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config for sync: %w", err)
	}
	
	// Create a mock command to pass to runMCPSync
	cmd := &cobra.Command{}
	args := []string{environment}
	
	return runMCPSync(cmd, args)
}