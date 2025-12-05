package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"
)

var (
	// loadedConfig stores the loaded configuration for use by path helpers
	loadedConfig *Config
)

type Config struct {
	DatabaseURL      string
	SSHPort          int
	MCPPort          int
	APIPort          int
	SSHHostKeyPath   string
	AdminUsername    string
	Environment      string
	TelemetryEnabled bool
	OTELEndpoint     string // OpenTelemetry OTLP endpoint for exporting traces
	JaegerQueryURL   string // Jaeger Query API endpoint for fetching traces
	Debug            bool   // Debug mode enables verbose logging
	// Workspace Configuration
	Workspace string // Custom workspace path (overrides XDG paths)
	// AI Provider Configuration
	AIProvider string // openai, ollama, gemini
	AIAPIKey   string // The API key for the AI provider
	AIModel    string // Model name (e.g., gpt-4o, llama3, gemini-pro)
	AIBaseURL  string // Base URL for OpenAI-compatible endpoints (Ollama, etc)
	// CloudShip Integration
	CloudShip CloudShipConfig
	// Faker Templates (for local development)
	FakerTemplates map[string]FakerTemplate
	// Note: Station now uses official GenKit v1.0.1 plugins (custom plugin code preserved)
}

// FakerTemplate defines a reusable faker configuration
type FakerTemplate struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Instruction string `yaml:"instruction"`
	Model       string `yaml:"model"`
}

// CloudShipConfig holds CloudShip Lighthouse integration settings
type CloudShipConfig struct {
	Enabled           bool   `yaml:"enabled"`             // Enable CloudShip integration
	RegistrationKey   string `yaml:"registration_key"`    // CloudShip registration key
	Endpoint          string `yaml:"endpoint"`            // Lighthouse gRPC endpoint
	StationID         string `yaml:"station_id"`          // Station ID (auto-generated)
	BundleRegistryURL string `yaml:"bundle_registry_url"` // Bundle registry API URL
	APIURL            string `yaml:"api_url"`             // CloudShip Django API URL (for direct API calls)
	APIKey            string `yaml:"api_key"`             // CloudShip API key for authentication
	// OAuth settings for MCP authentication via CloudShip
	OAuth OAuthConfig `yaml:"oauth"` // OAuth configuration
}

// OAuthConfig holds OAuth settings for CloudShip authentication
type OAuthConfig struct {
	Enabled      bool   `yaml:"enabled"`       // Enable OAuth authentication for MCP
	ClientID     string `yaml:"client_id"`     // OAuth client ID from CloudShip
	AuthURL      string `yaml:"auth_url"`      // CloudShip OAuth authorization URL
	TokenURL     string `yaml:"token_url"`     // CloudShip OAuth token URL
	IntrospectURL string `yaml:"introspect_url"` // CloudShip OAuth introspect URL
	RedirectURI  string `yaml:"redirect_uri"`  // OAuth redirect URI (for auth code flow)
	Scopes       string `yaml:"scopes"`        // OAuth scopes (space-separated)
}

// InitViper initializes viper to read config from the correct location
// This must be called before Load() to ensure proper config file discovery
func InitViper(cfgFile string) error {
	if cfgFile != "" {
		// Use explicitly provided config file
		viper.SetConfigFile(cfgFile)
	} else {
		// Check for config file using the same logic as main CLI:
		// 1. Check current working directory first
		// 2. Fall back to XDG config directory (~/.config/station)
		cwd, err := os.Getwd()
		if err == nil {
			// Try current directory first
			if _, err := os.Stat(filepath.Join(cwd, "config.yaml")); err == nil {
				viper.AddConfigPath(cwd)
			}
		}

		// Also check XDG config directory
		configDir := GetStationConfigDir()
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Read environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("STATION")
	viper.BindEnv("encryption_key", "STATION_ENCRYPTION_KEY")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "[CONFIG] Using config file: %s\n", viper.ConfigFileUsed())
	}

	return nil
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      getEnvOrDefault("DATABASE_URL", GetDatabasePath()),
		SSHPort:          getEnvIntOrDefault("SSH_PORT", 2222),
		MCPPort:          getEnvIntOrDefault("MCP_PORT", 8586),
		APIPort:          getEnvIntOrDefault("API_PORT", 8585),
		SSHHostKeyPath:   getEnvOrDefault("SSH_HOST_KEY_PATH", "./ssh_host_key"),
		AdminUsername:    getEnvOrDefault("ADMIN_USERNAME", "admin"),
		Environment:      getEnvOrDefault("ENVIRONMENT", "development"),
		TelemetryEnabled: getEnvBoolOrDefault("TELEMETRY_ENABLED", true),                          // Default enabled with opt-out
		OTELEndpoint:     getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318"), // Default to local Jaeger
		JaegerQueryURL:   getEnvOrDefault("JAEGER_QUERY_URL", "http://localhost:16686"),           // Default to local Jaeger Query
		Debug:            getEnvBoolOrDefault("STN_DEBUG", false),                                 // Default to info level
		// Workspace Configuration
		Workspace: getEnvOrDefault("STATION_WORKSPACE", ""), // Custom workspace path
		// AI Provider Configuration with STN_ prefix and sane defaults
		AIProvider: getEnvOrDefault("STN_AI_PROVIDER", "openai"), // Default to OpenAI
		AIAPIKey:   getAIAPIKey(),                                // Smart fallback for API keys
		AIModel:    getAIModelDefault(),                          // Provider-specific defaults
		AIBaseURL:  getEnvOrDefault("STN_AI_BASE_URL", ""),       // Empty means use provider default
		// CloudShip Integration (disabled by default)
		CloudShip: CloudShipConfig{
			Enabled:           getEnvBoolOrDefault("STN_CLOUDSHIP_ENABLED", false),
			RegistrationKey:   getEnvOrDefault("STN_CLOUDSHIP_KEY", ""),
			Endpoint:          getEnvOrDefault("STN_CLOUDSHIP_ENDPOINT", "lighthouse.cloudshipai.com:50051"),
			StationID:         getEnvOrDefault("STN_CLOUDSHIP_STATION_ID", ""),
			BundleRegistryURL: getEnvOrDefault("STN_CLOUDSHIP_BUNDLE_REGISTRY_URL", "https://api.cloudshipai.com"),
			OAuth: OAuthConfig{
				Enabled:       getEnvBoolOrDefault("STN_CLOUDSHIP_OAUTH_ENABLED", false),
				ClientID:      getEnvOrDefault("STN_CLOUDSHIP_OAUTH_CLIENT_ID", ""),
				AuthURL:       getEnvOrDefault("STN_CLOUDSHIP_OAUTH_AUTH_URL", "https://app.cloudshipai.com/oauth/authorize/"),
				TokenURL:      getEnvOrDefault("STN_CLOUDSHIP_OAUTH_TOKEN_URL", "https://app.cloudshipai.com/oauth/token/"),
				IntrospectURL: getEnvOrDefault("STN_CLOUDSHIP_OAUTH_INTROSPECT_URL", "https://app.cloudshipai.com/oauth/introspect/"),
				RedirectURI:   getEnvOrDefault("STN_CLOUDSHIP_OAUTH_REDIRECT_URI", "http://localhost:8585/oauth/callback"),
				Scopes:        getEnvOrDefault("STN_CLOUDSHIP_OAUTH_SCOPES", "read stations"),
			},
		},
	}

	// Override with values from config file (if available) using Viper
	if viper.IsSet("database_url") {
		cfg.DatabaseURL = viper.GetString("database_url")
	}
	if viper.IsSet("ssh_port") {
		cfg.SSHPort = viper.GetInt("ssh_port")
	}
	if viper.IsSet("mcp_port") {
		cfg.MCPPort = viper.GetInt("mcp_port")
	}
	if viper.IsSet("api_port") {
		cfg.APIPort = viper.GetInt("api_port")
	}
	if viper.IsSet("admin_username") {
		cfg.AdminUsername = viper.GetString("admin_username")
	}
	if viper.IsSet("telemetry_enabled") {
		cfg.TelemetryEnabled = viper.GetBool("telemetry_enabled")
	}
	if viper.IsSet("otel_endpoint") {
		cfg.OTELEndpoint = viper.GetString("otel_endpoint")
	}
	if viper.IsSet("jaeger_query_url") {
		cfg.JaegerQueryURL = viper.GetString("jaeger_query_url")
	}
	if viper.IsSet("debug") {
		cfg.Debug = viper.GetBool("debug")
	}
	if viper.IsSet("ai_provider") {
		cfg.AIProvider = viper.GetString("ai_provider")
	}
	if viper.IsSet("ai_model") {
		cfg.AIModel = viper.GetString("ai_model")
	}
	if viper.IsSet("ai_api_key") {
		rawKey := viper.GetString("ai_api_key")
		// Expand environment variables like ${OPENAI_API_KEY}, but preserve direct values
		expandedKey := os.ExpandEnv(rawKey)
		// If expansion resulted in empty string but original had ${}, keep original (missing env var)
		if expandedKey == "" && rawKey != "" && (len(rawKey) > 3 && rawKey[0] == '$' && rawKey[1] == '{') {
			cfg.AIAPIKey = rawKey // Keep unexpanded form to show error later
		} else {
			cfg.AIAPIKey = expandedKey
		}
	}
	if viper.IsSet("ai_base_url") {
		cfg.AIBaseURL = viper.GetString("ai_base_url")
	}
	if viper.IsSet("workspace") {
		cfg.Workspace = viper.GetString("workspace")
	}

	// CloudShip configuration overrides from config file
	if viper.IsSet("cloudship.enabled") {
		cfg.CloudShip.Enabled = viper.GetBool("cloudship.enabled")
	}
	if viper.IsSet("cloudship.registration_key") {
		cfg.CloudShip.RegistrationKey = viper.GetString("cloudship.registration_key")
	}
	if viper.IsSet("cloudship.endpoint") {
		cfg.CloudShip.Endpoint = viper.GetString("cloudship.endpoint")
	}
	// Also check for lighthouse_url (legacy/alternative config key)
	if viper.IsSet("cloudship.lighthouse_url") {
		cfg.CloudShip.Endpoint = viper.GetString("cloudship.lighthouse_url")
	}
	if viper.IsSet("cloudship.station_id") {
		cfg.CloudShip.StationID = viper.GetString("cloudship.station_id")
	}
	if viper.IsSet("cloudship.bundle_registry_url") {
		cfg.CloudShip.BundleRegistryURL = viper.GetString("cloudship.bundle_registry_url")
	}
	if viper.IsSet("cloudship.api_url") {
		cfg.CloudShip.APIURL = viper.GetString("cloudship.api_url")
	}
	if viper.IsSet("cloudship.api_key") {
		cfg.CloudShip.APIKey = viper.GetString("cloudship.api_key")
	}
	// OAuth configuration overrides
	if viper.IsSet("cloudship.oauth.enabled") {
		cfg.CloudShip.OAuth.Enabled = viper.GetBool("cloudship.oauth.enabled")
	}
	if viper.IsSet("cloudship.oauth.client_id") {
		cfg.CloudShip.OAuth.ClientID = viper.GetString("cloudship.oauth.client_id")
	}
	if viper.IsSet("cloudship.oauth.auth_url") {
		cfg.CloudShip.OAuth.AuthURL = viper.GetString("cloudship.oauth.auth_url")
	}
	if viper.IsSet("cloudship.oauth.token_url") {
		cfg.CloudShip.OAuth.TokenURL = viper.GetString("cloudship.oauth.token_url")
	}
	if viper.IsSet("cloudship.oauth.introspect_url") {
		cfg.CloudShip.OAuth.IntrospectURL = viper.GetString("cloudship.oauth.introspect_url")
	}
	if viper.IsSet("cloudship.oauth.redirect_uri") {
		cfg.CloudShip.OAuth.RedirectURI = viper.GetString("cloudship.oauth.redirect_uri")
	}
	if viper.IsSet("cloudship.oauth.scopes") {
		cfg.CloudShip.OAuth.Scopes = viper.GetString("cloudship.oauth.scopes")
	}

	// Load faker templates from config file
	cfg.FakerTemplates = loadFakerTemplates()

	// Environment variable overrides (take precedence over config file)
	// CRITICAL: STATION_DATABASE must override everything (including viper config)
	// This ensures faker subprocesses with STATION_DATABASE env var use the correct database
	if stationDB := os.Getenv("STATION_DATABASE"); stationDB != "" {
		cfg.DatabaseURL = stationDB
	}

	if envProvider := os.Getenv("STN_AI_PROVIDER"); envProvider != "" {
		cfg.AIProvider = envProvider
	}
	if envModel := os.Getenv("STN_AI_MODEL"); envModel != "" {
		cfg.AIModel = envModel
	}
	if envBaseURL := os.Getenv("STN_AI_BASE_URL"); envBaseURL != "" {
		cfg.AIBaseURL = envBaseURL
	}

	// Store loaded config for use by path helpers
	loadedConfig = cfg

	return cfg, nil
}

// loadFakerTemplates loads faker templates from config file and merges with built-in templates
func loadFakerTemplates() map[string]FakerTemplate {
	templates := getBuiltInFakerTemplates()

	// Load custom templates from config file if present
	if viper.IsSet("faker_templates") {
		var customTemplates map[string]FakerTemplate
		if err := viper.UnmarshalKey("faker_templates", &customTemplates); err == nil {
			// Merge custom templates (override built-in if same key)
			for key, template := range customTemplates {
				templates[key] = template
			}
		}
	}

	return templates
}

// getBuiltInFakerTemplates returns the default faker templates
func getBuiltInFakerTemplates() map[string]FakerTemplate {
	return map[string]FakerTemplate{
		"aws-finops": {
			Name:        "AWS FinOps",
			Description: "Complete AWS cost management and optimization tools",
			Instruction: "Generate comprehensive AWS Cost Explorer and Billing API tools for FinOps investigations. Include tools for: 1) Cost queries (get_cost_and_usage, get_cost_forecast, get_cost_categories, get_dimension_values), 2) Cost anomaly detection (get_anomalies, get_anomaly_monitors, get_anomaly_subscriptions, detect_cost_spikes), 3) Service-level cost analysis (get_ec2_costs, get_rds_costs, get_s3_costs, get_lambda_costs, get_cloudfront_costs, get_data_transfer_costs), 4) Reserved Instances and Savings Plans (get_ri_utilization, get_ri_coverage, get_savings_plans_utilization, get_ri_recommendations, get_savings_plans_purchase_recommendation), 5) Cost allocation and tagging (get_cost_by_tag, get_cost_by_account, get_cost_by_region, get_untagged_resources, validate_cost_allocation_tags), 6) Budget management (list_budgets, get_budget_performance, get_budget_forecast, analyze_budget_variance, get_budget_alerts). Tools should accept parameters like time_period (start/end dates), granularity (daily/monthly), filters (service, region, tag, account), group_by dimensions, and return realistic AWS Cost Explorer JSON responses with detailed cost breakdowns, usage quantities, and trending data.",
			Model:       "gpt-4o-mini",
		},
		"gcp-finops": {
			Name:        "GCP FinOps",
			Description: "GCP cloud billing and cost optimization tools",
			Instruction: "Generate comprehensive GCP Cloud Billing and Cost Management API tools for FinOps investigations. Include tools for: 1) Querying billing data and export tables (query_billing_export, get_billing_account, list_projects_billing), 2) Analyzing cost trends and anomalies (analyze_cost_spike, get_cost_forecast, compare_period_costs, detect_cost_anomalies), 3) Resource cost attribution (get_service_costs, get_project_costs, get_sku_costs, get_label_costs), 4) Budget and alert management (list_budgets, get_budget_status, get_budget_alerts, analyze_budget_variance), 5) Recommendations and optimization (get_cost_recommendations, list_idle_resources, get_commitment_analysis, analyze_sustained_use_discount), 6) Cost allocation and reporting (get_cost_breakdown_by_service, get_cost_by_region, get_cost_by_label, generate_cost_report). Each tool should accept parameters like project_id, billing_account_id, time_range (start_date, end_date), granularity (daily, weekly, monthly), filters (service, sku, region, labels), and aggregation options.",
			Model:       "gpt-4o-mini",
		},
		"azure-finops": {
			Name:        "Azure FinOps",
			Description: "Azure cost management and optimization tools",
			Instruction: "Generate comprehensive Azure Cost Management API tools for FinOps investigations. Include tools for: 1) Cost queries (get_cost_and_usage, get_cost_forecast, query_cost_management), 2) Cost anomaly detection (detect_cost_anomalies, get_anomaly_alerts), 3) Resource-level analysis (get_resource_costs, get_subscription_costs, get_resource_group_costs), 4) Budget management (list_budgets, get_budget_alerts, analyze_budget_variance), 5) Recommendations (get_advisor_recommendations, get_rightsizing_recommendations, get_reserved_instance_recommendations), 6) Cost allocation (get_cost_by_tag, get_cost_by_department, get_cost_by_service). Tools should work with Azure Cost Management REST API patterns and return realistic Azure billing data.",
			Model:       "gpt-4o-mini",
		},
		"datadog-monitoring": {
			Name:        "Datadog Monitoring",
			Description: "Datadog metrics, logs, and monitoring tools",
			Instruction: "Generate Datadog monitoring API tools for DevOps and observability. Include tools for: 1) Metrics queries (query_metrics, get_metric_metadata, list_active_metrics), 2) Log analysis (search_logs, get_log_aggregates, analyze_log_patterns), 3) APM traces (search_traces, get_service_performance, analyze_trace_latency), 4) Monitors and alerts (list_monitors, get_monitor_status, get_alert_history), 5) Dashboards (get_dashboard_data, query_dashboard_widgets), 6) Infrastructure monitoring (get_host_metrics, get_container_metrics, get_process_metrics). Tools should return realistic Datadog API responses with time-series data, log entries, and monitoring insights.",
			Model:       "gpt-4o-mini",
		},
		"stripe-payments": {
			Name:        "Stripe Payments",
			Description: "Stripe payment and subscription API tools",
			Instruction: "Generate Stripe payment API tools for payment processing and subscription management. Include tools for: 1) Payment operations (create_payment_intent, capture_payment, refund_payment, list_payments), 2) Customer management (create_customer, update_customer, list_customers, get_customer_payment_methods), 3) Subscription handling (create_subscription, update_subscription, cancel_subscription, list_subscriptions), 4) Invoice operations (create_invoice, finalize_invoice, list_invoices, get_invoice_status), 5) Product and pricing (list_products, get_product_details, list_prices, create_price), 6) Payment analytics (get_payment_analytics, get_mrr_metrics, get_churn_analysis). Tools should return realistic Stripe API response formats with proper object structures.",
			Model:       "gpt-4o-mini",
		},
	}
}

// GetStationConfigDir returns the station configuration directory path
// This respects workspace configuration and falls back to XDG paths
func GetStationConfigDir() string {
	// CRITICAL: Use loaded config's workspace if available (most reliable)
	// This ensures we use the actual loaded config, not viper which may not be initialized yet
	if loadedConfig != nil && loadedConfig.Workspace != "" {
		return loadedConfig.Workspace
	}

	// Fallback 1: Check if workspace is configured via viper (CLI override or config file)
	if workspace := viper.GetString("workspace"); workspace != "" {
		return workspace
	}

	// Fallback 2: Use XDG config directory
	return getXDGConfigDir()
}

// getXDGConfigDir returns the XDG config directory for station
func getXDGConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		// Respect HOME environment variable (important for tests)
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			var err error
			homeDir, err = os.UserHomeDir()
			if err != nil {
				return filepath.Join(os.TempDir(), ".config", "station") // Fallback
			}
		}
		configHome = filepath.Join(homeDir, ".config")
	}
	return filepath.Join(configHome, "station")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getAIAPIKey provides smart fallback for AI API keys with backward compatibility
func getAIAPIKey() string {
	// Try STN-prefixed key first
	if key := os.Getenv("STN_AI_API_KEY"); key != "" {
		return key
	}

	// Try generic AI_API_KEY
	if key := os.Getenv("AI_API_KEY"); key != "" {
		return key
	}

	// Fall back to provider-specific keys for backward compatibility
	provider := getEnvOrDefault("STN_AI_PROVIDER", "openai")
	switch provider {
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "gemini":
		if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
			return key
		}
		return os.Getenv("GEMINI_API_KEY")
	case "cloudflare":
		// Cloudflare Workers AI uses CF_TOKEN for authentication
		if key := os.Getenv("CF_TOKEN"); key != "" {
			return key
		}
		if key := os.Getenv("CLOUDFLARE_API_KEY"); key != "" {
			return key
		}
		return os.Getenv("CLOUDFLARE_API_TOKEN")
	case "ollama":
		// Ollama typically doesn't need API keys for local instances
		return ""
	default:
		return ""
	}
}

// getAIModelDefault provides provider-specific model defaults
func getAIModelDefault() string {
	// Check for explicit model configuration first
	if model := os.Getenv("STN_AI_MODEL"); model != "" {
		return model
	}
	if model := os.Getenv("AI_MODEL"); model != "" {
		return model
	}

	// Provide sane defaults based on provider
	provider := getEnvOrDefault("STN_AI_PROVIDER", "openai")
	switch provider {
	case "openai":
		// Official GenKit v1.0.1 OpenAI plugin supported models:
		// Latest: gpt-4.1, gpt-4.1-mini, gpt-4.1-nano, gpt-4.5-preview
		// Production: gpt-4o, gpt-4o-mini, gpt-4-turbo, gpt-4, gpt-3.5-turbo
		// Reasoning: o3-mini, o1, o1-preview, o1-mini
		return "gpt-4o-mini" // Fast and cost-effective default
	case "gemini":
		return "gemini-pro" // Google's main model
	case "cloudflare":
		return "@cf/openai/gpt-oss-120b" // Cloudflare Workers AI GPT-OSS model
	case "ollama":
		return "llama3" // Popular local model
	default:
		return "gpt-4o-mini" // Safe, cost-effective fallback
	}
}

// GetSupportedOpenAIModels returns the list of models supported by the official GenKit v1.0.1 OpenAI plugin
func GetSupportedOpenAIModels() []string {
	return []string{
		// Latest Models
		"gpt-4.1",
		"gpt-4.1-mini",
		"gpt-4.1-nano",
		"gpt-4.5-preview",
		// Production Models
		"gpt-4o",
		"gpt-4o-mini",
		"gpt-4-turbo",
		"gpt-4",
		"gpt-3.5-turbo",
		// Reasoning Models
		"o3-mini",
		"o1",
		"o1-preview",
		"o1-mini",
	}
}

// GetRecommendedOpenAIModels returns recommended models for different use cases
func GetRecommendedOpenAIModels() map[string]string {
	return map[string]string{
		"cost_effective": "gpt-4o-mini",
		"balanced":       "gpt-4o",
		"latest":         "gpt-4.1",
		"reasoning":      "o1-mini",
	}
}
