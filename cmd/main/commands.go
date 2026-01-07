package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"station/cmd/main/handlers"
	"station/internal/auth"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/internal/services"
	"station/internal/templates"
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
• Lattice mesh network for multi-station agent orchestration

Use --provider and --model flags to skip interactive setup.
Use --yes flag to use sensible defaults without any prompts.
Use --replicate flag to also configure Litestream for database replication to cloud storage.
Use --ship flag to bootstrap with ship CLI MCP integration for filesystem access.

Lattice Configuration:
  --lattice-url         Join an existing lattice mesh (e.g., nats://orchestrator:4222)
  --lattice-orchestrator  Run as orchestrator with embedded NATS server
  --lattice-name        Station name in the mesh (default: hostname)
  --lattice-token       Authentication token for NATS`,
		RunE: runInit,
	}

	mcpAddCmd = &cobra.Command{
		Use:   "add <server-name>",
		Short: "Add an MCP server configuration",
		Long: `Add an MCP server configuration to an environment.

OVERVIEW:
  Creates a JSON configuration file for an MCP (Model Context Protocol) server.
  The server becomes available to AI agents after running 'stn sync'.

MODES:
  Non-interactive (default): Pass all configuration via flags. Best for scripting and AI agents.
  Interactive (-i):          Opens your $EDITOR to edit a template. Best for complex configs.

OUTPUT:
  Creates: ~/.config/station/environments/<env>/<server-name>.json
  
TEMPLATE VARIABLES:
  Use {{.VAR_NAME}} syntax in --env values for secrets that should be prompted at sync time.
  Example: --env "API_KEY={{.MY_API_KEY}}"
  
  When you run 'stn sync --browser', users enter these values securely in a browser form.
  This keeps secrets OUT of CLI history and AI agent context.

COMMON MCP SERVERS:
  Filesystem:  --command npx --args "-y,@modelcontextprotocol/server-filesystem,/path"
  GitHub:      --command npx --args "-y,@modelcontextprotocol/server-github" --env "GITHUB_TOKEN={{.GITHUB_TOKEN}}"
  Slack:       --command npx --args "-y,@anthropic/mcp-server-slack" --env "SLACK_TOKEN={{.SLACK_TOKEN}}"
  PostgreSQL:  --command npx --args "-y,@modelcontextprotocol/server-postgres" --env "DATABASE_URL={{.DATABASE_URL}}"
  Playwright:  --command npx --args "-y,@anthropic/mcp-server-playwright"

WORKFLOW:
  1. Add server:    stn mcp add <name> --command <cmd> --args <args>
  2. Sync env:      stn sync <env> --browser   # Enter any template variables
  3. Use tools:     AI agents can now use the MCP server's tools

EXAMPLES:
  # Add filesystem MCP server (no secrets needed)
  stn mcp add filesystem --command npx --args "-y,@modelcontextprotocol/server-filesystem,/home/user/projects"

  # Add GitHub MCP server with token template (prompted at sync)
  stn mcp add github \
    --command npx \
    --args "-y,@modelcontextprotocol/server-github" \
    --env "GITHUB_TOKEN={{.GITHUB_TOKEN}}" \
    --description "GitHub API integration for repo management"

  # Add to production environment
  stn mcp add database --env prod \
    --command npx \
    --args "-y,@modelcontextprotocol/server-postgres" \
    --env "DATABASE_URL={{.PROD_DATABASE_URL}}"

  # Interactive mode - opens editor with template
  stn mcp add complex-server -i

  # After adding, sync to activate (browser mode for secrets)
  stn sync default --browser`,
		Args: cobra.ExactArgs(1),
		RunE: runMCPAdd,
	}

	mcpAddOpenapiCmd = &cobra.Command{
		Use:   "add-openapi <name>",
		Short: "Add an OpenAPI spec as an MCP server",
		Long: `Add an OpenAPI specification file as an MCP server to an environment.

OVERVIEW:
  Downloads or copies an OpenAPI spec and creates an MCP server that exposes
  each API operation as a tool. This is the easiest way to add API integrations.

SOURCES:
  --url <url>     Download OpenAPI spec from a URL
  --file <path>   Copy OpenAPI spec from a local file

OUTPUT:
  Creates: ~/.config/station/environments/<env>/<name>.openapi.json
  
  After running 'stn sync', each API operation becomes an available tool.

TEMPLATE VARIABLES:
  OpenAPI specs can use Go template variables for secrets:
  Example in spec: "Authorization": "Bearer {{.API_KEY}}"
  
  Set variables in ~/.config/station/environments/<env>/variables.yml

EXAMPLES:
  # Add from URL
  stn mcp add-openapi petstore --url https://petstore3.swagger.io/api/v3/openapi.json

  # Add from local file
  stn mcp add-openapi myapi --file ./api-spec.json

  # Add to specific environment
  stn mcp add-openapi github-api --url https://raw.githubusercontent.com/.../openapi.json -e production

  # After adding, sync to discover tools
  stn sync default`,
		Args: cobra.ExactArgs(1),
		RunE: runMCPAddOpenapi,
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

	syncCmd = &cobra.Command{
		Use:   "sync [environment]",
		Short: "Sync all file-based configurations",
		Long: `Declaratively synchronize all file-based configurations to the database.

OVERVIEW:
  Syncs agents (.prompt files), MCP server configurations (.json), and environment
  settings from the filesystem to the Station database. After sync, AI agents can
  use all configured MCP tools.

WHAT GETS SYNCED:
  ~/.config/station/environments/<env>/
  ├── *.prompt          → Agent definitions (system prompts, tools, schemas)
  ├── *.json            → MCP server configurations
  └── variables.yml     → Template variable values (created by sync)

TEMPLATE VARIABLES:
  MCP configs can use {{.VAR_NAME}} placeholders for secrets/config values.
  During sync, you'll be prompted to enter values for any missing variables.
  
  --browser MODE (RECOMMENDED FOR AI AGENTS):
    Opens a browser window where users enter secrets directly.
    Secrets NEVER appear in terminal output or AI agent context.
    The CLI polls for completion and continues automatically.

  --interactive MODE (default):
    Prompts for variables in the terminal.
    Use --no-interactive to fail on missing variables instead.

SECURITY:
  The --browser flag is designed for AI agents (Claude, GPT, etc.) that should
  NOT see secrets in their context. The user enters secrets in their browser,
  keeping them out of the AI conversation entirely.

WORKFLOW:
  1. Add MCP servers:  stn mcp add github --command npx --args "..." --env "TOKEN={{.TOKEN}}"
  2. Sync with vars:   stn sync default --browser   # User enters TOKEN in browser
  3. Tools available:  AI agents can now call GitHub MCP tools

EXAMPLES:
  # Sync default environment (prompts in terminal for missing vars)
  stn sync default

  # Sync with browser-based variable input (RECOMMENDED for AI agents)
  stn sync default --browser

  # Sync production environment
  stn sync production --browser

  # Preview what would change without making changes
  stn sync default --dry-run

  # Validate configurations only (no database writes)
  stn sync default --validate

  # Fail if any variables are missing (for CI/CD)
  stn sync default --no-interactive

  # Verbose output showing all operations
  stn sync default -v`,
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

	deployCmd = &cobra.Command{
		Use:   "deploy [environment-name]",
		Short: "Deploy Station environment to cloud platform",
		Long: `Deploy a Station environment to a cloud platform with agents and MCP tools.

Deploy using either:
  1. Local environment: stn deploy <environment-name>
  2. CloudShip bundle:  stn deploy --bundle-id <uuid>
  3. Local bundle file: stn deploy --bundle ./my-bundle.tar.gz

Supported targets:
  fly        - Fly.io (uses fly secrets set, persistent storage)
  kubernetes - Kubernetes (generates manifests, use --secrets-backend for runtime secrets)
  ansible    - Ansible (generates playbook for Docker + SSH deployment)
  cloudflare - [EXPERIMENTAL] Cloudflare Containers

Runtime secrets (--secrets-backend):
  Container fetches secrets at startup from:
  - aws-secretsmanager  - AWS Secrets Manager
  - aws-ssm             - AWS SSM Parameter Store
  - vault               - HashiCorp Vault
  - gcp-secretmanager   - Google Secret Manager
  - sops                - SOPS encrypted file

Use 'stn deploy export-vars' to see what secrets are needed for deployment.

The deployed instance exposes agents via MCP for public access.`,
		Example: `  # Bundle-based deploy (no local environment needed)
  stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a --target k8s
  stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a --target fly
  stn deploy --bundle-id e26b414a-f076-4135-927f-810bc1dc892a --name my-station --target ansible

  # Local bundle file deploy
  stn deploy --bundle ./my-bundle.tar.gz --target fly
  stn deploy --bundle ./my-bundle.tar.gz --target k8s --name my-station

  # Fly.io
  stn deploy my-env --target fly                    # Deploy to Fly.io (always-on)
  stn deploy my-env --target fly --auto-stop        # Enable auto-stop when idle
  stn deploy my-env --target fly --destroy          # Tear down deployment

  # Kubernetes
  stn deploy my-env --target kubernetes             # Generate K8s manifests
  stn deploy my-env --target k8s --namespace prod   # Deploy to namespace
  stn deploy my-env --target k8s --dry-run          # Preview only
  stn deploy my-env --target k8s --context my-ctx   # Use specific context

  # Ansible (SSH + Docker)
  stn deploy my-env --target ansible --dry-run      # Generate playbook
  stn deploy my-env --target ansible                # Run playbook

  # Runtime secrets (container fetches from backend at startup)
  stn deploy --bundle-id xxx --target k8s --secrets-backend vault --secrets-path secret/data/station/prod
  stn deploy --bundle-id xxx --target k8s --secrets-backend aws-secretsmanager --secrets-path station/prod

  # See what secrets are needed
  stn deploy export-vars default                           # For environment
  stn deploy export-vars --bundle-id xxx                   # For bundle

  # Generate secrets template for CI/CD
  stn deploy export-vars default --format env > secrets.env`,
		Args: cobra.MaximumNArgs(1),
		RunE: runDeploy,
	}
)

func runServe(cmd *cobra.Command, args []string) error {
	// Set GenKit environment based on --dev flag
	devMode, _ := cmd.Flags().GetBool("dev")
	if !devMode && os.Getenv("GENKIT_ENV") == "" {
		os.Setenv("GENKIT_ENV", "prod") // Disable reflection server by default
	}

	// Check if configuration exists
	// Use environment variable directly to avoid viper initialization issues
	configDir := os.Getenv("STATION_CONFIG_DIR")
	if configDir == "" {
		configDir = os.Getenv("XDG_CONFIG_HOME")
		if configDir != "" {
			configDir = filepath.Join(configDir, "station")
		} else {
			configDir = filepath.Join(os.Getenv("HOME"), ".config", "station")
		}
	}
	configFile := filepath.Join(configDir, "config.yaml")

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Auto-initialize if we detect API keys or bundle ID (Docker/container deployments)
		hasAPIKey := os.Getenv("OPENAI_API_KEY") != "" ||
			os.Getenv("ANTHROPIC_API_KEY") != "" ||
			os.Getenv("GEMINI_API_KEY") != "" ||
			os.Getenv("GOOGLE_API_KEY") != "" ||
			os.Getenv("AI_API_KEY") != "" ||
			os.Getenv("STN_API_KEY") != "" ||
			os.Getenv("STN_AI_API_KEY") != "" ||
			os.Getenv("STN_AI_OAUTH_TOKEN") != ""

		hasBundleID := os.Getenv("STN_BUNDLE_ID") != ""

		if hasAPIKey {
			logging.Info("🔧 Auto-initializing Station from environment variables...")

			// Auto-create config from environment variables
			if err := autoInitializeConfig(configFile, configDir); err != nil {
				return fmt.Errorf("auto-initialization failed: %w", err)
			}

			logging.Info("✅ Configuration auto-initialized successfully")
		} else if hasBundleID {
			// Bundle ID is set but no API key - provide helpful error
			fmt.Printf("Error: STN_BUNDLE_ID is set but no AI provider API key found.\n")
			fmt.Printf("Please provide an API key for the AI provider your bundle uses:\n")
			fmt.Printf("  docker run -e STN_BUNDLE_ID=... -e OPENAI_API_KEY=sk-... station:latest\n")
			fmt.Printf("  docker run -e STN_BUNDLE_ID=... -e ANTHROPIC_API_KEY=... station:latest\n")
			return fmt.Errorf("STN_BUNDLE_ID requires an AI provider API key")
		} else {
			fmt.Printf("Configuration not found. Please run 'station init' first.\n")
			fmt.Printf("Expected config file: %s\n", configFile)
			fmt.Printf("\nFor container deployments, provide an API key via environment:\n")
			fmt.Printf("  docker run -e OPENAI_API_KEY=sk-... station:latest\n")
			return fmt.Errorf("configuration not initialized")
		}
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
	if envCloudShipName := os.Getenv("STN_CLOUDSHIP_NAME"); envCloudShipName != "" && viper.GetString("cloudship.name") == "" {
		viper.Set("cloudship.name", envCloudShipName)
	}
	if envCloudShipBaseURL := os.Getenv("STN_CLOUDSHIP_BASE_URL"); envCloudShipBaseURL != "" && viper.GetString("cloudship.base_url") == "" {
		viper.Set("cloudship.base_url", envCloudShipBaseURL)
	}

	// CloudShip OAuth configuration via environment variables
	if envOAuthEnabled := os.Getenv("STN_CLOUDSHIP_OAUTH_ENABLED"); envOAuthEnabled != "" && !viper.IsSet("cloudship.oauth.enabled") {
		viper.Set("cloudship.oauth.enabled", envOAuthEnabled == "true")
	}
	if envOAuthClientID := os.Getenv("STN_CLOUDSHIP_OAUTH_CLIENT_ID"); envOAuthClientID != "" && viper.GetString("cloudship.oauth.client_id") == "" {
		viper.Set("cloudship.oauth.client_id", envOAuthClientID)
	}
	if envOAuthAuthURL := os.Getenv("STN_CLOUDSHIP_OAUTH_AUTH_URL"); envOAuthAuthURL != "" && viper.GetString("cloudship.oauth.auth_url") == "" {
		viper.Set("cloudship.oauth.auth_url", envOAuthAuthURL)
	}
	if envOAuthTokenURL := os.Getenv("STN_CLOUDSHIP_OAUTH_TOKEN_URL"); envOAuthTokenURL != "" && viper.GetString("cloudship.oauth.token_url") == "" {
		viper.Set("cloudship.oauth.token_url", envOAuthTokenURL)
	}
	if envOAuthIntrospectURL := os.Getenv("STN_CLOUDSHIP_OAUTH_INTROSPECT_URL"); envOAuthIntrospectURL != "" && viper.GetString("cloudship.oauth.introspect_url") == "" {
		viper.Set("cloudship.oauth.introspect_url", envOAuthIntrospectURL)
	}
	if envOAuthRedirectURI := os.Getenv("STN_CLOUDSHIP_OAUTH_REDIRECT_URI"); envOAuthRedirectURI != "" && viper.GetString("cloudship.oauth.redirect_uri") == "" {
		viper.Set("cloudship.oauth.redirect_uri", envOAuthRedirectURI)
	}
	if envOAuthScopes := os.Getenv("STN_CLOUDSHIP_OAUTH_SCOPES"); envOAuthScopes != "" && viper.GetString("cloudship.oauth.scopes") == "" {
		viper.Set("cloudship.oauth.scopes", envOAuthScopes)
	}

	fmt.Printf("MCP Port: %d\n", viper.GetInt("mcp_port"))
	fmt.Printf("API Port: %d\n", viper.GetInt("api_port"))
	fmt.Printf("Database: %s\n", viper.GetString("database_url"))

	// Set environment variables for the main application to use
	os.Setenv("ENCRYPTION_KEY", encryptionKey)
	os.Setenv("MCP_PORT", fmt.Sprintf("%d", viper.GetInt("mcp_port")))
	os.Setenv("API_PORT", fmt.Sprintf("%d", viper.GetInt("api_port")))
	os.Setenv("DATABASE_URL", viper.GetString("database_url"))
	if viper.GetBool("debug") {
		os.Setenv("STATION_DEBUG", "true")
	}

	// Import and run the main server code
	return runMainServer()
}

// autoInitializeConfig creates a minimal Station config from environment variables
// This allows Docker containers to start without pre-baked config.yaml files
//
// IMPORTANT: This only writes the MINIMAL required config (encryption_key, database_url).
// All other settings are read directly from environment variables at runtime via config.Load().
// This means ANY env var supported by config.Load() will work without changes here.
func autoInitializeConfig(configFile, configDir string) error {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate encryption key - this MUST be persisted to disk
	encryptionKey, err := auth.GenerateAPIKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	// Remove the "sk-" prefix as this is an encryption key, not an API key
	encryptionKey = encryptionKey[3:]

	// Get database path
	databasePath := os.Getenv("DATABASE_URL")
	if databasePath == "" {
		databasePath = filepath.Join(configDir, "station.db")
	}

	// Only set the minimal required values that MUST be persisted
	// Everything else comes from env vars via config.Load()
	viper.Set("encryption_key", encryptionKey)
	viper.Set("database_url", databasePath)

	// Write minimal config file
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logging.Info("📝 Created config file: %s", configFile)
	logging.Info("🗄️  Database: %s", databasePath)
	logging.Info("📋 All other settings will be read from environment variables")

	return nil
}

// Helper functions for auto-init
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if custom config file path is provided
	configPath, _ := cmd.Flags().GetString("config")
	var configFile string
	var configDir string

	var workspaceDir string

	if configPath != "" {
		// Handle both file paths and directories
		absPath, _ := filepath.Abs(configPath)

		// If it's a directory or just ".", append config.yaml
		if stat, err := os.Stat(absPath); err == nil && stat.IsDir() {
			configFile = filepath.Join(absPath, "config.yaml")
		} else if configPath == "." || configPath == "./" {
			cwd, _ := os.Getwd()
			configFile = filepath.Join(cwd, "config.yaml")
		} else {
			configFile = absPath
		}

		workspaceDir = filepath.Dir(configFile)

		// Set the workspace in viper for the session
		viper.Set("workspace", workspaceDir)

		// Also set database path relative to workspace
		databasePath := filepath.Join(workspaceDir, "station.db")
		viper.Set("database_url", databasePath)

		configDir = workspaceDir
	} else {
		// No custom config - use secure default XDG location
		configDir = getXDGConfigDir()
		configFile = filepath.Join(configDir, "config.yaml")

		// Workspace directory for content (environments, bundles, etc.)
		workspaceDir = getWorkspacePath()
	}

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

	// Check existing config to avoid interactive mode and load existing values
	_, err := os.Stat(configFile)
	configExists := err == nil

	// If config exists, load it into viper FIRST to preserve existing values
	// (e.g., otel_endpoint with host.docker.internal from stn up --bundle workflow)
	if configExists {
		viper.SetConfigFile(configFile)
		viper.SetConfigType("yaml")
		_ = viper.ReadInConfig() // Ignore error, we'll use defaults if it fails
	}

	// Check OTEL configuration (priority: CLI flag > env var > existing config > default)
	otelEndpoint, _ := cmd.Flags().GetString("otel-endpoint")
	// Default telemetry to TRUE (we ship with Jaeger integration)
	telemetryEnabled := true
	if cmd.Flags().Changed("telemetry") {
		telemetryEnabled, _ = cmd.Flags().GetBool("telemetry")
	}

	// Check environment variable if flag not provided
	if otelEndpoint == "" {
		otelEndpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	// Check existing config value (for stn up --bundle workflow where config is pre-copied)
	if otelEndpoint == "" && viper.GetString("otel_endpoint") != "" {
		otelEndpoint = viper.GetString("otel_endpoint")
	}
	// Default to local Jaeger endpoint if not specified
	if otelEndpoint == "" {
		otelEndpoint = "http://localhost:4318"
	}

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
			} else if provider == "cloudshipai" {
				model = "cloudship/llama-3.1-70b"
			} else {
				model = "gemini-2.5-flash"
			}
		}

		// If cloudshipai is selected via flag, ensure we have authentication
		if provider == "cloudshipai" && !configExists {
			if err := ensureCloudShipAuth(); err != nil {
				return fmt.Errorf("CloudShip AI authentication required: %w", err)
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
	// Only set local_mode if not already configured (preserve value from config file or env var)
	if !viper.IsSet("local_mode") {
		viper.Set("local_mode", true) // Default to local mode
	}

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

	// Set Lattice configuration
	latticeURL, _ := cmd.Flags().GetString("lattice-url")
	latticeName, _ := cmd.Flags().GetString("lattice-name")
	latticeOrchestrator, _ := cmd.Flags().GetBool("lattice-orchestrator")
	latticePort, _ := cmd.Flags().GetInt("lattice-port")
	latticeToken, _ := cmd.Flags().GetString("lattice-token")

	// Check environment variables as fallback
	if latticeURL == "" {
		latticeURL = os.Getenv("STN_LATTICE_NATS_URL")
	}
	if latticeName == "" {
		latticeName = os.Getenv("STN_LATTICE_STATION_NAME")
	}
	if latticeToken == "" {
		latticeToken = os.Getenv("STN_LATTICE_AUTH_TOKEN")
	}

	// Default station name to hostname
	if latticeName == "" {
		if hostname, err := os.Hostname(); err == nil {
			latticeName = hostname
		} else {
			latticeName = "station"
		}
	}

	if latticeOrchestrator || latticeURL != "" {
		viper.Set("lattice.station_name", latticeName)

		if latticeOrchestrator {
			viper.Set("lattice_orchestration", true)
			viper.Set("lattice.orchestrator.embedded_nats.port", latticePort)
			viper.Set("lattice.orchestrator.embedded_nats.http_port", latticePort+4000)
			if latticeToken != "" {
				viper.Set("lattice.orchestrator.embedded_nats.auth.enabled", true)
				viper.Set("lattice.orchestrator.embedded_nats.auth.token", latticeToken)
			}
			fmt.Printf("🔗 Lattice orchestrator mode enabled\n")
			fmt.Printf("   NATS Port: %d\n", latticePort)
			fmt.Printf("   Station Name: %s\n", latticeName)
		} else if latticeURL != "" {
			viper.Set("lattice.nats.url", latticeURL)
			if latticeToken != "" {
				viper.Set("lattice.nats.auth.token", latticeToken)
			}
			fmt.Printf("🔗 Lattice client mode enabled\n")
			fmt.Printf("   NATS URL: %s\n", latticeURL)
			fmt.Printf("   Station Name: %s\n", latticeName)
		}
	}

	// Set harness defaults (agentic execution harness)
	if !viper.IsSet("harness.workspace.path") {
		viper.Set("harness.workspace.path", "./workspace")
	}
	if !viper.IsSet("harness.workspace.mode") {
		viper.Set("harness.workspace.mode", "host")
	}
	if !viper.IsSet("harness.compaction.enabled") {
		viper.Set("harness.compaction.enabled", true)
	}
	if !viper.IsSet("harness.compaction.threshold") {
		viper.Set("harness.compaction.threshold", "0.85")
	}
	if !viper.IsSet("harness.compaction.protect_tokens") {
		viper.Set("harness.compaction.protect_tokens", 40000)
	}
	if !viper.IsSet("harness.git.auto_branch") {
		viper.Set("harness.git.auto_branch", true)
	}
	if !viper.IsSet("harness.git.branch_prefix") {
		viper.Set("harness.git.branch_prefix", "agent/")
	}
	if !viper.IsSet("harness.git.auto_commit") {
		viper.Set("harness.git.auto_commit", false)
	}
	if !viper.IsSet("harness.git.require_approval") {
		viper.Set("harness.git.require_approval", true)
	}
	if !viper.IsSet("harness.git.workflow_branch_strategy") {
		viper.Set("harness.git.workflow_branch_strategy", "shared")
	}
	if !viper.IsSet("harness.nats.enabled") {
		viper.Set("harness.nats.enabled", true)
	}
	if !viper.IsSet("harness.nats.kv_bucket") {
		viper.Set("harness.nats.kv_bucket", "harness-state")
	}
	if !viper.IsSet("harness.nats.object_bucket") {
		viper.Set("harness.nats.object_bucket", "harness-files")
	}
	if !viper.IsSet("harness.nats.max_file_size") {
		viper.Set("harness.nats.max_file_size", "100MB")
	}
	if !viper.IsSet("harness.nats.ttl") {
		viper.Set("harness.nats.ttl", "24h")
	}
	if !viper.IsSet("harness.permissions.external_directory") {
		viper.Set("harness.permissions.external_directory", "deny")
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

	// Create harness workspace directory
	harnessWorkspaceDir := filepath.Join(configDir, "workspace")
	if err := os.MkdirAll(harnessWorkspaceDir, 0755); err != nil {
		fmt.Printf("   Warning: Failed to create workspace directory: %v\n", err)
	} else {
		fmt.Printf("   Created harness workspace directory\n")
	}

	// Generate .gitignore for workspace if using custom config path
	if configPath != "" {
		gitignorePath := filepath.Join(configDir, ".gitignore")
		if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
			gitignoreContent := `# Station runtime files - DO NOT COMMIT
station.db
station.db-shm
station.db-wal

# Harness workspace (agent execution artifacts)
workspace/

# Runtime variable resolution
vars/

# SSH host keys (regenerated per deployment)
ssh_host_key
ssh_host_key.pub

# Local environment overrides
*.local.yaml
.env.local

# Note: config.yaml contains generated encryption_key
# Each engineer/deployment should generate their own via 'stn init'
`
			if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
				fmt.Printf("⚠️  Warning: Failed to create .gitignore: %v\n", err)
			} else {
				fmt.Printf("📝 Created .gitignore for workspace\n")
			}
		}

		// Bootstrap GitHub Actions workflows
		if err := bootstrapGitHubWorkflows(configDir); err != nil {
			fmt.Printf("⚠️  Warning: Failed to create GitHub Actions workflows: %v\n", err)
		} else {
			fmt.Printf("🔄 Created GitHub Actions workflows in .github/workflows/\n")
		}
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

	// Create Jaeger docker-compose file for optional telemetry
	if _, err := handlers.EnsureJaegerComposeFile(); err != nil {
		logging.Info("⚠️  Failed to create Jaeger docker-compose: %v", err)
	}

	// Create OpenCode docker-compose file for AI coding sandbox
	if _, err := handlers.EnsureOpenCodeComposeFile(); err != nil {
		logging.Info("⚠️  Failed to create OpenCode docker-compose: %v", err)
	}

	fmt.Printf("\n🎉 Station initialized successfully!\n\n")
	fmt.Printf("📁 Config file: %s\n", configFile)
	fmt.Printf("🗄️  Database: %s\n", viper.GetString("database_url"))
	fmt.Printf("📁 File config structure: %s\n", filepath.Join(workspaceDir, "environments", "default"))

	// Show GitHub workflows info if created
	if configPath != "" {
		fmt.Printf("🔄 GitHub Actions workflows: %s\n", filepath.Join(configDir, ".github", "workflows"))
	}

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
		fmt.Printf("\n🔍 Telemetry (optional):\n")
		fmt.Printf("   • Run 'stn jaeger up' to start Jaeger for distributed tracing\n")
		fmt.Printf("   • View traces at http://localhost:16686\n")

		// Show GitHub Actions info if workflows were created
		if configPath != "" {
			fmt.Printf("\n🚀 CI/CD Ready:\n")
			fmt.Printf("   • Workflows created in .github/workflows/\n")
			fmt.Printf("   • Push to GitHub and use Actions tab to build bundles or container images\n")
			fmt.Printf("   • Trigger manually: gh workflow run build-env-image.yml -f environment_name=default -f image_tag=v1.0.0\n")
		}
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

func runSync(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name is required")
	}

	environment := args[0]
	browserMode, _ := cmd.Flags().GetBool("browser")

	if browserMode {
		return runSyncWithBrowser(environment)
	}

	return runSyncForEnvironment(environment)
}

func runSyncWithBrowser(environment string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.LoadSecretsFromBackend(); err != nil {
		return fmt.Errorf("failed to load secrets from backend: %w", err)
	}

	browserSync := services.NewBrowserSyncService(cfg.APIPort)
	_, err = browserSync.SyncWithBrowser(context.Background(), environment)
	return err
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
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.LoadSecretsFromBackend(); err != nil {
		return fmt.Errorf("failed to load secrets from backend: %w", err)
	}

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

func runDeploy(cmd *cobra.Command, args []string) error {
	var envName string
	if len(args) > 0 {
		envName = args[0]
	}

	target, _ := cmd.Flags().GetString("target")
	region, _ := cmd.Flags().GetString("region")
	sleepAfter, _ := cmd.Flags().GetString("sleep-after")
	autoStop, _ := cmd.Flags().GetBool("auto-stop")
	instanceType, _ := cmd.Flags().GetString("instance-type")
	destroy, _ := cmd.Flags().GetBool("destroy")
	withOpenCode, _ := cmd.Flags().GetBool("with-opencode")
	withSandbox, _ := cmd.Flags().GetBool("with-sandbox")
	namespace, _ := cmd.Flags().GetString("namespace")
	k8sContext, _ := cmd.Flags().GetString("context")
	outputDir, _ := cmd.Flags().GetString("output-dir")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	bundleID, _ := cmd.Flags().GetString("bundle-id")
	bundlePath, _ := cmd.Flags().GetString("bundle")
	appName, _ := cmd.Flags().GetString("name")
	hosts, _ := cmd.Flags().GetStringSlice("hosts")
	sshKey, _ := cmd.Flags().GetString("ssh-key")
	sshUser, _ := cmd.Flags().GetString("ssh-user")
	envFile, _ := cmd.Flags().GetString("env-file")
	secretsBackend, _ := cmd.Flags().GetString("secrets-backend")
	secretsPath, _ := cmd.Flags().GetString("secrets-path")

	if envName == "" && bundleID == "" && bundlePath == "" {
		return fmt.Errorf("either environment name, --bundle-id, or --bundle is required\n\nUsage:\n  stn deploy <environment>          Deploy local environment\n  stn deploy --bundle-id <uuid>     Deploy CloudShip bundle\n  stn deploy --bundle ./file.tar.gz Deploy local bundle file")
	}

	exclusiveCount := 0
	if envName != "" {
		exclusiveCount++
	}
	if bundleID != "" {
		exclusiveCount++
	}
	if bundlePath != "" {
		exclusiveCount++
	}
	if exclusiveCount > 1 {
		return fmt.Errorf("cannot specify multiple of: environment name, --bundle-id, --bundle")
	}

	if bundlePath != "" {
		return deployLocalBundle(cmd, bundlePath, target, region, sleepAfter, instanceType, destroy, autoStop, withOpenCode, withSandbox, namespace, k8sContext, outputDir, dryRun, appName, hosts, sshKey, sshUser, envFile, secretsBackend, secretsPath)
	}

	if !autoStop {
		sleepAfter = "168h"
	}

	ctx := context.Background()
	return handlers.HandleDeploy(ctx, envName, target, region, sleepAfter, instanceType, destroy, autoStop, withOpenCode, withSandbox, namespace, k8sContext, outputDir, dryRun, bundleID, appName, hosts, sshKey, sshUser, "", envFile, secretsBackend, secretsPath)
}

func resolveDeployEnvName(customName string, cfg *config.Config, bundlePath string) string {
	if customName != "" {
		return customName
	}
	if cfg != nil && cfg.CloudShip.Name != "" {
		return cfg.CloudShip.Name
	}
	bundleBaseName := filepath.Base(bundlePath)
	for _, ext := range []string{".tar.gz", ".tgz", ".tar", ".gz"} {
		if len(bundleBaseName) > len(ext) && bundleBaseName[len(bundleBaseName)-len(ext):] == ext {
			bundleBaseName = bundleBaseName[:len(bundleBaseName)-len(ext)]
			break
		}
	}
	if bundleBaseName == "" {
		return fmt.Sprintf("deploy-%d", time.Now().Unix())
	}
	return bundleBaseName
}

func deployLocalBundle(cmd *cobra.Command, bundlePath, target, region, sleepAfter, instanceType string, destroy, autoStop, withOpenCode, withSandbox bool, namespace, k8sContext, outputDir string, dryRun bool, appName string, hosts []string, sshKey, sshUser, envFile, secretsBackend, secretsPath string) error {
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		return fmt.Errorf("bundle file not found: %s", bundlePath)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	envName := resolveDeployEnvName(appName, cfg, bundlePath)

	normalizedTarget := strings.ToLower(target)
	skipLocalInstall := normalizedTarget == "kubernetes" || normalizedTarget == "k8s" || normalizedTarget == "ansible"

	// Fly.io doesn't support local bundle file deployment - it needs environment with baked-in agents
	if !skipLocalInstall && (normalizedTarget == "fly" || normalizedTarget == "flyio" || normalizedTarget == "fly.io") {
		return fmt.Errorf("fly target does not support --bundle (local file).\n\nOptions:\n  1. Use --bundle-id for CloudShip bundles: stn deploy --bundle-id <uuid> --target fly\n  2. Install bundle locally first and deploy environment: stn bundle install %s my-env && stn deploy my-env --target fly\n  3. Use Kubernetes or Ansible which copy the bundle file: stn deploy --bundle %s --target k8s", bundlePath, bundlePath)
	}

	if skipLocalInstall {
		fmt.Printf("📦 Using bundle file directly (no local installation): %s\n", bundlePath)
		fmt.Printf("   Environment name: %s\n", envName)
	} else {
		fmt.Printf("📦 Installing bundle to environment: %s\n", envName)

		database, err := db.New(cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}
		defer func() { _ = database.Close() }()

		repos := repositories.New(database)
		bundleService := services.NewBundleServiceWithRepos(repos)

		result, err := bundleService.InstallBundleWithOptions(bundlePath, envName, false)
		if err != nil || !result.Success {
			errorMsg := result.Error
			if errorMsg == "" && err != nil {
				errorMsg = err.Error()
			}
			return fmt.Errorf("bundle installation failed: %s", errorMsg)
		}

		fmt.Printf("✅ Bundle installed: %d agents, %d MCP configs\n", result.InstalledAgents, result.InstalledMCPs)
	}

	if !autoStop {
		sleepAfter = "168h"
	}

	ctx := context.Background()
	return handlers.HandleDeploy(ctx, envName, target, region, sleepAfter, instanceType, destroy, autoStop, withOpenCode, withSandbox, namespace, k8sContext, outputDir, dryRun, "", appName, hosts, sshKey, sshUser, bundlePath, envFile, secretsBackend, secretsPath)
}

// bootstrapGitHubWorkflows creates GitHub Actions workflow files in .github/workflows/
func bootstrapGitHubWorkflows(workspaceDir string) error {
	workflowsDir := filepath.Join(workspaceDir, ".github", "workflows")

	// Create .github/workflows directory
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .github/workflows directory: %w", err)
	}

	// Write build-bundle.yml
	bundleWorkflowPath := filepath.Join(workflowsDir, "build-bundle.yml")
	if _, err := os.Stat(bundleWorkflowPath); os.IsNotExist(err) {
		if err := os.WriteFile(bundleWorkflowPath, []byte(templates.BuildBundleWorkflow), 0644); err != nil {
			return fmt.Errorf("failed to write build-bundle.yml: %w", err)
		}
	}

	// Write build-env-image.yml
	envImageWorkflowPath := filepath.Join(workflowsDir, "build-env-image.yml")
	if _, err := os.Stat(envImageWorkflowPath); os.IsNotExist(err) {
		if err := os.WriteFile(envImageWorkflowPath, []byte(templates.BuildEnvImageWorkflow), 0644); err != nil {
			return fmt.Errorf("failed to write build-env-image.yml: %w", err)
		}
	}

	// Write build-image-from-bundle.yml
	bundleImageWorkflowPath := filepath.Join(workflowsDir, "build-image-from-bundle.yml")
	if _, err := os.Stat(bundleImageWorkflowPath); os.IsNotExist(err) {
		if err := os.WriteFile(bundleImageWorkflowPath, []byte(templates.BuildImageFromBundleWorkflow), 0644); err != nil {
			return fmt.Errorf("failed to write build-image-from-bundle.yml: %w", err)
		}
	}

	return nil
}
