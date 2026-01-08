package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"station/cmd/main/handlers"
	"station/internal/config"
	"station/internal/db"
	"station/internal/logging"
	"station/internal/services"
	"station/internal/telemetry"
	"station/internal/theme"
	"station/internal/version"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile              string
	enableOTEL           bool   // Global flag to enable OTEL telemetry
	otelEndpoint         string // OTEL endpoint override
	themeManager         *theme.ThemeManager
	telemetryService     *telemetry.TelemetryService // PostHog analytics
	otelTelemetryService *services.TelemetryService  // OTEL distributed tracing
	rootCmd              = &cobra.Command{
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
	cobra.OnInitialize(initOTELTelemetry)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/station/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&enableOTEL, "enable-telemetry", false, "Enable OpenTelemetry distributed tracing (exports to Jaeger)")
	rootCmd.PersistentFlags().StringVar(&otelEndpoint, "otel-endpoint", "", "OpenTelemetry OTLP endpoint (default: http://localhost:4318)")

	// Add subcommands
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(deployCmd)
	initDeploySubcommands()
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(bundleCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(runsCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(benchmarkCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(developCmd)
	rootCmd.AddCommand(blastoffCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(mockCmd)
	rootCmd.AddCommand(fakerCmd)
	rootCmd.AddCommand(handlers.NewJaegerCmd())
	rootCmd.AddCommand(handlers.NewOpenCodeCmd())
	rootCmd.AddCommand(workflowCmd)
	rootCmd.AddCommand(providerCmd)
	rootCmd.AddCommand(filesCmd)
	rootCmd.AddCommand(latticeCmd)
	rootCmd.AddCommand(githubCmd)
	rootCmd.AddCommand(secretsCmd)

	filesCmd.AddCommand(filesUploadCmd)
	filesCmd.AddCommand(filesDownloadCmd)
	filesCmd.AddCommand(filesListCmd)
	filesCmd.AddCommand(filesDeleteCmd)
	filesCmd.AddCommand(filesInfoCmd)

	// Legacy file-config handlers removed - use 'stn sync' instead

	initConfigCmd()

	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpToolsCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpCmd.AddCommand(mcpAddOpenapiCmd)
	mcpCmd.AddCommand(mcpDeleteCmd)
	mcpCmd.AddCommand(mcpStatusCmd)

	// Unified bundle command replaces the old template system
	// bundleCmd is standalone and doesn't need subcommands

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
	agentCmd.AddCommand(agentRunCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentUpdateCmd)
	agentCmd.AddCommand(agentDeleteCmd)

	reportCmd.AddCommand(reportCreateCmd)
	reportCmd.AddCommand(reportGenerateCmd)
	reportCmd.AddCommand(reportListCmd)
	reportCmd.AddCommand(reportShowCmd)

	runsCmd.AddCommand(runsListCmd)
	runsCmd.AddCommand(runsInspectCmd)

	benchmarkCmd.AddCommand(benchmarkEvaluateCmd)
	benchmarkCmd.AddCommand(benchmarkListCmd)
	benchmarkCmd.AddCommand(benchmarkTasksCmd)

	workflowCmd.AddCommand(workflowListCmd)
	workflowCmd.AddCommand(workflowShowCmd)
	workflowCmd.AddCommand(workflowRunCmd)
	workflowCmd.AddCommand(workflowRunsCmd)
	workflowCmd.AddCommand(workflowInspectCmd)
	workflowCmd.AddCommand(workflowDebugExpressionCmd)
	workflowCmd.AddCommand(workflowExportCmd)
	workflowCmd.AddCommand(workflowDeleteCmd)
	workflowCmd.AddCommand(workflowValidateCmd)
	workflowCmd.AddCommand(workflowApprovalsCmd)
	workflowApprovalsCmd.AddCommand(workflowApprovalsListCmd)
	workflowApprovalsCmd.AddCommand(workflowApprovalsApproveCmd)
	workflowApprovalsCmd.AddCommand(workflowApprovalsRejectCmd)

	settingsCmd.AddCommand(settingsListCmd)
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)

	// Init command flags
	initCmd.Flags().Bool("replicate", false, "Set up Litestream database replication for production deployments")
	initCmd.Flags().StringP("config", "c", "", "Path to configuration file (sets workspace to config file's directory)")
	initCmd.Flags().Bool("ship", false, "Bootstrap with ship CLI MCP integration for filesystem access")
	initCmd.Flags().String("provider", "", "AI provider (openai, gemini, custom) - if not set, shows interactive selection")
	initCmd.Flags().String("model", "", "AI model name - if not set, shows interactive selection based on provider")
	initCmd.Flags().String("api-key", "", "API key for AI provider (alternative to environment variables)")
	initCmd.Flags().String("base-url", "", "Base URL for OpenAI-compatible endpoints (e.g., http://localhost:11434/v1 for Ollama)")
	initCmd.Flags().BoolP("yes", "y", false, "Use defaults without interactive prompts")
	initCmd.Flags().String("cloudship-key", "", "CloudShip registration key for station management and monitoring")
	initCmd.Flags().String("cloudship-endpoint", "lighthouse.cloudship.ai:443", "CloudShip Lighthouse gRPC endpoint")
	initCmd.Flags().String("otel-endpoint", "", "OpenTelemetry OTLP endpoint for telemetry export (e.g., http://localhost:4318)")
	initCmd.Flags().Bool("telemetry", false, "Enable telemetry collection and export (default: false)")
	initCmd.Flags().String("lattice-url", "", "NATS URL to join a lattice mesh (e.g., nats://orchestrator:4222)")
	initCmd.Flags().String("lattice-name", "", "Station name in the lattice mesh (defaults to hostname)")
	initCmd.Flags().Bool("lattice-orchestrator", false, "Run as lattice orchestrator with embedded NATS server")
	initCmd.Flags().Int("lattice-port", 4222, "NATS port for embedded orchestrator (default: 4222)")
	initCmd.Flags().String("lattice-token", "", "Authentication token for lattice NATS (for both client and embedded server)")

	// Serve command flags
	serveCmd.Flags().Int("ssh-port", 2222, "SSH server port")
	serveCmd.Flags().Int("mcp-port", 8586, "MCP server port")
	serveCmd.Flags().Int("api-port", 8585, "API server port")
	serveCmd.Flags().String("database", "station.db", "Database file path")
	serveCmd.Flags().Bool("debug", false, "Enable debug logging")
	serveCmd.Flags().Bool("local", false, "Run in local mode (single user, no authentication)")
	serveCmd.Flags().Bool("dev", false, "Enable development mode with GenKit reflection server (default: disabled)")
	serveCmd.Flags().Bool("orchestration", false, "Run as lattice orchestrator with embedded NATS hub (enables Station-to-Station mesh)")
	serveCmd.Flags().String("lattice", "", "Connect to lattice orchestrator NATS URL (e.g., nats://orchestrator:4222)")

	// MCP Add command flags
	mcpAddCmd.Flags().StringP("environment", "e", "default", "Environment to add configuration to")
	mcpAddCmd.Flags().StringP("command", "c", "", "Command to execute the MCP server (required unless -i)")
	mcpAddCmd.Flags().StringSliceP("args", "a", nil, "Command arguments (comma-separated or multiple -a flags)")
	mcpAddCmd.Flags().StringToStringP("env", "E", nil, "Environment variables (KEY=VALUE, can use {{.VAR}} for templates)")
	mcpAddCmd.Flags().StringP("description", "d", "", "Description of the MCP server")
	mcpAddCmd.Flags().BoolP("interactive", "i", false, "Open editor for interactive configuration")
	mcpAddCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")

	mcpAddOpenapiCmd.Flags().StringP("environment", "e", "default", "Environment to add the OpenAPI spec to")
	mcpAddOpenapiCmd.Flags().StringP("url", "u", "", "URL to download OpenAPI spec from")
	mcpAddOpenapiCmd.Flags().StringP("file", "f", "", "Local file path to OpenAPI spec")

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
	syncCmd.Flags().Bool("browser", false, "Open browser for secure variable input (useful for LLM agents)")

	// Bootstrap command flags
	bootstrapCmd.Flags().Bool("openai", false, "Bootstrap with OpenAI provider (runs stn init --ship --provider openai --model gpt-5)")

	// Develop command flags
	developCmd.Flags().String("env", "default", "Environment to load agents and MCP configs from")
	developCmd.Flags().Int("port", 4000, "Port to run the Genkit development server on")
	developCmd.Flags().String("ai-model", "", "AI model to use (overrides Station config)")
	developCmd.Flags().String("ai-provider", "", "AI provider to use (gemini, openai, anthropic)")
	developCmd.Flags().Bool("verbose", false, "Enable verbose logging for debugging")
	developCmd.Flags().Bool("auto-ui", true, "Auto-launch GenKit Developer UI (default: true)")
	syncCmd.Flags().BoolP("verbose", "v", false, "Verbose output showing all operations")

	// Deploy command flags
	deployCmd.Flags().String("target", "fly", "Deployment target (fly, kubernetes, ansible, cloudflare)")
	deployCmd.Flags().String("region", "ord", "Deployment region (e.g., ord, syd, fra)")
	deployCmd.Flags().String("sleep-after", "10m", "Cloudflare: sleep container after inactivity (e.g., 10m, 1h, 24h)")
	deployCmd.Flags().Bool("auto-stop", false, "Enable auto-stop/suspend when idle (default: always-on for persistent Lighthouse connection)")
	deployCmd.Flags().String("instance-type", "basic", "Cloudflare: container size (lite, basic, standard-1 to standard-4)")
	deployCmd.Flags().Bool("destroy", false, "Tear down the deployment instead of deploying")
	deployCmd.Flags().Bool("with-opencode", false, "Deploy OpenCode coding backend alongside Station (Fly.io only)")
	deployCmd.Flags().Bool("with-sandbox", false, "Enable Fly Machines sandbox backend for code execution (Fly.io only)")
	deployCmd.Flags().String("secrets-backend", "", "Runtime secrets backend - container fetches on startup (aws-secretsmanager, aws-ssm, vault, gcp-secretmanager, sops)")
	deployCmd.Flags().String("secrets-path", "", "Path for runtime secrets backend (e.g., station/prod for AWS SM, secret/data/station/prod for Vault)")
	deployCmd.Flags().String("namespace", "", "Kubernetes namespace (default: default)")
	deployCmd.Flags().String("context", "", "Kubernetes context to use")
	deployCmd.Flags().String("output-dir", "", "Output directory for generated configs")
	deployCmd.Flags().Bool("dry-run", false, "Generate configs only, don't deploy")
	deployCmd.Flags().String("bundle-id", "", "CloudShip bundle ID to deploy (uses base image, no local environment needed)")
	deployCmd.Flags().String("bundle", "", "Local bundle file (.tar.gz) to deploy - installs to temp environment then deploys")
	deployCmd.Flags().String("name", "", "Custom name for the deployed app (default: station-<env> or cloudshipai.name from config)")
	deployCmd.Flags().StringSlice("hosts", nil, "Ansible: target hosts (user@host or host format, can specify multiple)")
	deployCmd.Flags().String("ssh-key", "", "Ansible: SSH private key path")
	deployCmd.Flags().String("ssh-user", "root", "Ansible: SSH user (default: root)")
	deployCmd.Flags().String("env-file", "", "Load secrets from .env file (use with 'stn deploy export-vars' to generate template)")

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
	agentRunCmd.Flags().String("coding-session", "", "Existing OpenCode session ID to continue (for coding_open tool)")
	agentDeleteCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	agentDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion without prompt")

	agentCreateCmd.Flags().StringP("prompt", "p", "", "System prompt for the agent (required)")
	agentCreateCmd.Flags().StringP("description", "d", "", "Agent description (required)")
	agentCreateCmd.Flags().StringP("environment", "e", "default", "Environment to create agent in")
	agentCreateCmd.Flags().Int("max-steps", 5, "Maximum execution steps")
	agentCreateCmd.Flags().String("tools", "", "Comma-separated list of tool names to assign")
	agentCreateCmd.Flags().String("output-schema", "", "JSON schema for structured output")
	agentCreateCmd.Flags().String("output-schema-preset", "", "Predefined schema preset (e.g., 'finops')")
	agentCreateCmd.Flags().String("input-schema", "", "JSON schema for input variables")
	agentCreateCmd.Flags().String("app", "", "CloudShip app classification for data ingestion")
	agentCreateCmd.Flags().String("app-type", "", "CloudShip app_type classification for data ingestion")
	agentCreateCmd.Flags().String("memory-topic", "", "CloudShip memory topic key for context injection")
	agentCreateCmd.Flags().Int("memory-max-tokens", 0, "Max tokens for memory context (default: 2000)")
	agentCreateCmd.Flags().String("sandbox", "", "Sandbox config JSON (e.g., '{\"enabled\":true,\"image\":\"python:3.11\"}')")
	agentCreateCmd.Flags().String("coding", "", "Coding config JSON (e.g., '{\"enabled\":true,\"backend\":\"opencode\"}')")
	agentCreateCmd.Flags().Bool("notify", false, "Enable notifications for this agent")

	agentUpdateCmd.Flags().StringP("environment", "e", "default", "Environment the agent is in")
	agentUpdateCmd.Flags().StringP("prompt", "p", "", "New system prompt")
	agentUpdateCmd.Flags().StringP("description", "d", "", "New description")
	agentUpdateCmd.Flags().Int("max-steps", 0, "New max steps (0 = unchanged)")
	agentUpdateCmd.Flags().String("tools", "", "New comma-separated tool list")
	agentUpdateCmd.Flags().String("output-schema", "", "New JSON schema for structured output")
	agentUpdateCmd.Flags().String("output-schema-preset", "", "Predefined schema preset (e.g., 'finops')")
	agentUpdateCmd.Flags().String("app", "", "CloudShip app classification")
	agentUpdateCmd.Flags().String("app-type", "", "CloudShip app_type classification")
	agentUpdateCmd.Flags().String("memory-topic", "", "CloudShip memory topic key")
	agentUpdateCmd.Flags().Int("memory-max-tokens", 0, "Max tokens for memory context")
	agentUpdateCmd.Flags().String("sandbox", "", "Sandbox config JSON")
	agentUpdateCmd.Flags().String("coding", "", "Coding config JSON")
	agentUpdateCmd.Flags().Bool("notify", false, "Enable notifications")

	// Runs command flags
	runsListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	runsListCmd.Flags().Int("limit", 50, "Maximum number of runs to display")
	runsInspectCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	runsInspectCmd.Flags().BoolP("verbose", "v", false, "Show detailed run information including tool calls, execution steps, and metadata")

	// Benchmark command flags
	benchmarkEvaluateCmd.Flags().BoolP("verbose", "v", false, "Show detailed metric analysis and evidence")

	// Workflow command flags
	workflowShowCmd.Flags().Int64("version", 0, "Specific workflow version to show (0 for latest)")
	workflowShowCmd.Flags().BoolP("verbose", "v", false, "Show full workflow definition")
	workflowRunCmd.Flags().String("input", "", "Input JSON for the workflow")
	workflowRunCmd.Flags().Int64("version", 0, "Specific workflow version to run (0 for latest)")
	workflowRunCmd.Flags().Bool("wait", false, "Wait for workflow to complete")
	workflowRunCmd.Flags().Duration("timeout", 5*time.Minute, "Timeout when waiting for completion")
	workflowRunsCmd.Flags().Int64("limit", 20, "Maximum number of runs to show")
	workflowRunsCmd.Flags().Int64("offset", 0, "Number of runs to skip for pagination")
	workflowRunsCmd.Flags().String("status", "", "Filter by status (running, completed, failed)")
	workflowInspectCmd.Flags().BoolP("verbose", "v", false, "Show detailed step output")
	workflowDebugExpressionCmd.Flags().String("context", "", "JSON context for evaluation")
	workflowDebugExpressionCmd.Flags().String("run-id", "", "Load context from a specific run ID")
	workflowDebugExpressionCmd.Flags().String("data-path", "$", "JSONPath to extract data before evaluation")
	workflowExportCmd.Flags().Int64("version", 0, "Specific workflow version to export (0 for latest)")
	workflowExportCmd.Flags().StringP("environment", "e", "default", "Environment to export to")
	workflowExportCmd.Flags().StringP("output", "o", "", "Output file path (default: environment's workflows directory)")
	workflowDeleteCmd.Flags().BoolP("all", "a", false, "Delete all workflows")
	workflowDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	workflowDeleteCmd.Flags().StringP("environment", "e", "default", "Environment to delete workflow files from")
	workflowDeleteCmd.Flags().Bool("keep-file", false, "Keep the workflow file (only delete from database)")
	workflowValidateCmd.Flags().String("format", "text", "Output format: text or json")
	workflowApprovalsListCmd.Flags().BoolP("all", "a", false, "Show all approvals, not just pending")
	workflowApprovalsApproveCmd.Flags().StringP("comment", "c", "", "Optional comment for the approval")
	workflowApprovalsRejectCmd.Flags().StringP("reason", "r", "", "Reason for rejection")

	// Report command flags
	reportCreateCmd.Flags().StringP("environment", "e", "", "Environment name (required)")
	reportCreateCmd.Flags().StringP("name", "n", "", "Report name (required)")
	reportCreateCmd.Flags().StringP("description", "d", "", "Report description")
	reportGenerateCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	reportListCmd.Flags().StringP("environment", "e", "", "Filter reports by environment name")
	reportListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	reportShowCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")

	// Settings command flags
	settingsListCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	settingsGetCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	settingsSetCmd.Flags().String("endpoint", "", "Station API endpoint (default: use local mode)")
	settingsSetCmd.Flags().String("description", "", "Description for the setting")

	filesUploadCmd.Flags().String("station", "", "Station API endpoint (use HTTP API instead of local NATS)")
	filesUploadCmd.Flags().String("key", "", "Custom file key (default: auto-generated)")
	filesUploadCmd.Flags().String("ttl", "", "Time-to-live for the file (e.g., 24h, 7d)")
	filesDownloadCmd.Flags().String("station", "", "Station API endpoint (use HTTP API instead of local NATS)")
	filesDownloadCmd.Flags().StringP("output", "o", "", "Output file path (default: derived from key)")
	filesListCmd.Flags().String("station", "", "Station API endpoint (use HTTP API instead of local NATS)")
	filesListCmd.Flags().String("prefix", "", "Filter files by key prefix")
	filesListCmd.Flags().Bool("json", false, "Output in JSON format")
	filesDeleteCmd.Flags().String("station", "", "Station API endpoint (use HTTP API instead of local NATS)")
	filesDeleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	filesInfoCmd.Flags().String("station", "", "Station API endpoint (use HTTP API instead of local NATS)")

	// Bind flags to viper
	viper.BindPFlag("ssh_port", serveCmd.Flags().Lookup("ssh-port"))
	viper.BindPFlag("mcp_port", serveCmd.Flags().Lookup("mcp-port"))
	viper.BindPFlag("api_port", serveCmd.Flags().Lookup("api-port"))
	viper.BindPFlag("database_url", serveCmd.Flags().Lookup("database"))
	viper.BindPFlag("debug", serveCmd.Flags().Lookup("debug"))
	viper.BindPFlag("local_mode", serveCmd.Flags().Lookup("local"))
	viper.BindPFlag("dev_mode", serveCmd.Flags().Lookup("dev"))
	viper.BindPFlag("lattice_orchestration", serveCmd.Flags().Lookup("orchestration"))
	viper.BindPFlag("lattice_url", serveCmd.Flags().Lookup("lattice"))

	// Set default values
	viper.SetDefault("telemetry_enabled", false)
	// AI provider defaults are handled in config.Load() to respect environment variables
	// Don't set viper defaults here as they override env vars
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

	// Explicitly bind critical environment variables that may not be in config file
	// This is required for Docker deployments where config is minimal
	viper.BindEnv("encryption_key", "STATION_ENCRYPTION_KEY")

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

func initOTELTelemetry() {
	// Load config to check telemetry settings
	cfg, err := config.Load()
	if err != nil {
		logging.Debug("Failed to load config for OTEL telemetry: %v", err)
		return
	}

	// Check if telemetry is enabled (priority: CLI flag > config file)
	telemetryEnabled := enableOTEL || cfg.Telemetry.Enabled || cfg.TelemetryEnabled
	if !telemetryEnabled {
		logging.Debug("OTEL telemetry disabled (use --enable-telemetry flag or set telemetry.enabled: true in config)")
		return
	}

	// Build CloudShip info for telemetry
	// Use station name from config for resource attributes
	var cloudShipInfo *services.CloudShipInfo
	if cfg.CloudShip.RegistrationKey != "" || cfg.CloudShip.Name != "" {
		cloudShipInfo = &services.CloudShipInfo{
			RegistrationKey: cfg.CloudShip.RegistrationKey,
			StationName:     cfg.CloudShip.Name,      // Use configured station name
			StationID:       cfg.CloudShip.StationID, // Use configured station ID (if any)
			// OrgID populated when connected to CloudShip (not in config)
		}
	}

	// Build telemetry config from the new struct
	otelConfig := services.NewTelemetryConfigFromConfig(&cfg.Telemetry, cloudShipInfo)

	// CLI flag overrides take precedence
	if otelEndpoint != "" {
		otelConfig.Endpoint = otelEndpoint
		otelConfig.Provider = config.TelemetryProviderOTLP
	}

	// Environment variable override (for backward compatibility)
	if envEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); envEndpoint != "" && otelEndpoint == "" {
		otelConfig.Endpoint = envEndpoint
		otelConfig.Provider = config.TelemetryProviderOTLP
	}

	// Log the provider being used
	logging.Info("🔭 OTEL telemetry enabled - provider=%s, endpoint=%s", otelConfig.Provider, otelConfig.Endpoint)

	// Initialize OTEL telemetry service
	otelTelemetryService = services.NewTelemetryService(otelConfig)
	err = otelTelemetryService.Initialize(context.Background())
	if err != nil {
		logging.Info("Failed to initialize OTEL telemetry: %v", err)
		otelTelemetryService = nil
		return
	}

	logging.Debug("OTEL telemetry initialized successfully (provider: %s, endpoint: %s, service: %s)", otelConfig.Provider, otelConfig.Endpoint, otelConfig.ServiceName)
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

	// Cleanup OTEL telemetry
	if otelTelemetryService != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelTelemetryService.Shutdown(ctx); err != nil {
			logging.Debug("Failed to shutdown OTEL telemetry: %v", err)
		}
	}

	// CloudShip cleanup is now handled by individual command contexts
}
