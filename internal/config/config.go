package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var (
	// loadedConfig stores the loaded configuration for use by path helpers
	loadedConfig *Config
)

type Config struct {
	DatabaseURL    string
	SSHPort        int
	MCPPort        int
	APIPort        int
	SSHHostKeyPath string
	AdminUsername  string
	Environment    string
	Debug          bool // Debug mode enables verbose logging
	// Workspace Configuration
	Workspace string // Custom workspace path (overrides XDG paths)
	// AI Provider Configuration
	AIProvider string // openai, ollama, gemini
	AIAPIKey   string // The API key for the AI provider
	AIModel    string // Model name (e.g., gpt-4o, llama3, gemini-pro)
	AIBaseURL  string // Base URL for OpenAI-compatible endpoints (Ollama, etc)
	// CloudShip Integration
	CloudShip CloudShipConfig
	// Telemetry Configuration (distributed tracing)
	Telemetry TelemetryConfig
	// Faker Templates (for local development)
	FakerTemplates map[string]FakerTemplate
	// Note: Station now uses official GenKit v1.0.1 plugins (custom plugin code preserved)

	// Legacy fields (deprecated, use Telemetry struct instead)
	TelemetryEnabled bool   // Deprecated: use Telemetry.Enabled
	OTELEndpoint     string // Deprecated: use Telemetry.Endpoint
	JaegerQueryURL   string // Deprecated: use Telemetry.JaegerQueryURL
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
	Enabled            bool     `yaml:"enabled"`             // Enable CloudShip integration
	RegistrationKey    string   `yaml:"registration_key"`    // CloudShip registration key
	Endpoint           string   `yaml:"endpoint"`            // Lighthouse gRPC endpoint
	UseTLS             bool     `yaml:"use_tls"`             // Use TLS for gRPC connection (default: false)
	InsecureSkipVerify bool     `yaml:"skip_tls_verify"`     // Skip TLS certificate verification (for self-signed certs)
	StationID          string   `yaml:"station_id"`          // Station ID (auto-generated, legacy v1)
	Name               string   `yaml:"name"`                // Station name (required for v2, unique across org)
	Tags               []string `yaml:"tags"`                // User-defined tags for filtering ["production", "us-east-1"]
	BundleRegistryURL  string   `yaml:"bundle_registry_url"` // Bundle registry API URL
	APIURL             string   `yaml:"api_url"`             // CloudShip Django API URL (for direct API calls)
	APIKey             string   `yaml:"api_key"`             // CloudShip API key for authentication
	BaseURL            string   `yaml:"base_url"`            // CloudShip base URL for OAuth discovery (default: https://app.cloudshipai.com)
	// OAuth settings for MCP authentication via CloudShip
	OAuth OAuthConfig `yaml:"oauth"` // OAuth configuration
}

// OAuthConfig holds OAuth settings for CloudShip authentication
type OAuthConfig struct {
	Enabled       bool   `yaml:"enabled"`        // Enable OAuth authentication for MCP
	ClientID      string `yaml:"client_id"`      // OAuth client ID from CloudShip
	AuthURL       string `yaml:"auth_url"`       // CloudShip OAuth authorization URL
	TokenURL      string `yaml:"token_url"`      // CloudShip OAuth token URL
	IntrospectURL string `yaml:"introspect_url"` // CloudShip OAuth introspect URL
	RedirectURI   string `yaml:"redirect_uri"`   // OAuth redirect URI (for auth code flow)
	Scopes        string `yaml:"scopes"`         // OAuth scopes (space-separated)
}

// TelemetryProvider defines the type of telemetry backend
type TelemetryProvider string

const (
	// TelemetryProviderNone disables telemetry export
	TelemetryProviderNone TelemetryProvider = "none"
	// TelemetryProviderJaeger uses local Jaeger (no auth, http://localhost:4318)
	TelemetryProviderJaeger TelemetryProvider = "jaeger"
	// TelemetryProviderOTLP uses a custom OTLP endpoint with optional auth
	TelemetryProviderOTLP TelemetryProvider = "otlp"
	// TelemetryProviderCloudShip uses CloudShip's managed telemetry (telemetry.cloudshipai.com)
	TelemetryProviderCloudShip TelemetryProvider = "cloudship"

	// CloudShipTelemetryEndpoint is the managed OTLP endpoint for CloudShip users
	CloudShipTelemetryEndpoint = "https://telemetry.cloudshipai.com/v1/traces"
	// LocalJaegerEndpoint is the default local Jaeger OTLP endpoint
	LocalJaegerEndpoint = "http://localhost:4318"
	// LocalJaegerQueryURL is the default local Jaeger UI URL
	LocalJaegerQueryURL = "http://localhost:16686"
)

// TelemetryConfig holds configuration for distributed tracing
type TelemetryConfig struct {
	// Enabled controls whether telemetry is active (default: true)
	Enabled bool `yaml:"enabled"`

	// Provider determines the telemetry backend: "none", "jaeger", "otlp", "cloudship"
	// - "none": Disable trace export
	// - "jaeger": Local Jaeger at http://localhost:4318 (no auth)
	// - "otlp": Custom OTLP endpoint (set Endpoint and optional Headers)
	// - "cloudship": CloudShip managed telemetry (uses registration key for auth)
	Provider TelemetryProvider `yaml:"provider"`

	// Endpoint is the OTLP endpoint URL (used for "otlp" and "jaeger" providers)
	// Examples:
	//   - http://localhost:4318 (local Jaeger)
	//   - https://otel-collector.example.com:4318 (custom OTLP)
	// For "cloudship" provider, this is automatically set to telemetry.cloudshipai.com
	Endpoint string `yaml:"endpoint"`

	// Headers are custom HTTP headers to send with OTLP requests (for "otlp" provider)
	// Example: {"Authorization": "Bearer my-token"}
	Headers map[string]string `yaml:"headers"`

	// JaegerQueryURL is the Jaeger Query API endpoint for fetching traces (UI/API)
	// Default: http://localhost:16686
	JaegerQueryURL string `yaml:"jaeger_query_url"`

	// ServiceName overrides the default service name in traces (default: "station")
	ServiceName string `yaml:"service_name"`

	// Environment tag added to all traces (default: "development")
	Environment string `yaml:"environment"`

	// SampleRate controls trace sampling (0.0 to 1.0, default: 1.0 for dev, 0.1 for prod)
	SampleRate float64 `yaml:"sample_rate"`
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

	// Read config file FIRST (lowest priority)
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintf(os.Stderr, "[CONFIG] Using config file: %s\n", viper.ConfigFileUsed())
	}

	// Environment variables take HIGHEST priority - they override config file values
	// We support multiple prefixes: STN_, STATION_, and unprefixed for common vars
	viper.AutomaticEnv()

	// Bind all STN_* and STATION_* environment variables to their config keys
	// This ensures env vars ALWAYS override config file values
	bindEnvVars()

	return nil
}

// bindEnvVars explicitly binds environment variables to viper config keys
// This ensures environment variables take precedence over config file values
func bindEnvVars() {
	// Core config
	viper.BindEnv("encryption_key", "STATION_ENCRYPTION_KEY", "STN_ENCRYPTION_KEY")
	viper.BindEnv("database_url", "DATABASE_URL", "STATION_DATABASE", "STN_DATABASE_URL")
	viper.BindEnv("mcp_port", "STATION_MCP_PORT", "STN_MCP_PORT", "MCP_PORT")
	viper.BindEnv("api_port", "STATION_API_PORT", "STN_API_PORT", "API_PORT")
	viper.BindEnv("ssh_port", "STATION_SSH_PORT", "STN_SSH_PORT", "SSH_PORT")
	viper.BindEnv("admin_username", "STATION_ADMIN_USERNAME", "STN_ADMIN_USERNAME")
	viper.BindEnv("debug", "STATION_DEBUG", "STN_DEBUG")
	viper.BindEnv("local_mode", "STATION_LOCAL_MODE", "STN_LOCAL_MODE")
	viper.BindEnv("dev_mode", "STN_DEV_MODE", "STATION_DEV_MODE")

	// AI Provider config
	viper.BindEnv("ai_provider", "STN_AI_PROVIDER", "STATION_AI_PROVIDER")
	viper.BindEnv("ai_model", "STN_AI_MODEL", "STATION_AI_MODEL")
	viper.BindEnv("ai_base_url", "STN_AI_BASE_URL", "STATION_AI_BASE_URL")

	// CloudShip config - these are critical for container deployments
	viper.BindEnv("cloudship.enabled", "STN_CLOUDSHIP_ENABLED")
	viper.BindEnv("cloudship.registration_key", "STN_CLOUDSHIP_KEY")
	viper.BindEnv("cloudship.endpoint", "STN_CLOUDSHIP_ENDPOINT")
	viper.BindEnv("cloudship.name", "STN_CLOUDSHIP_NAME")
	viper.BindEnv("cloudship.station_id", "STN_CLOUDSHIP_STATION_ID")
	viper.BindEnv("cloudship.base_url", "STN_CLOUDSHIP_BASE_URL")
	viper.BindEnv("cloudship.api_url", "STN_CLOUDSHIP_API_URL")
	viper.BindEnv("cloudship.bundle_registry_url", "STN_CLOUDSHIP_BUNDLE_REGISTRY_URL")
	viper.BindEnv("cloudship.use_tls", "STN_CLOUDSHIP_USE_TLS")

	// CloudShip OAuth config
	viper.BindEnv("cloudship.oauth.enabled", "STN_CLOUDSHIP_OAUTH_ENABLED")
	viper.BindEnv("cloudship.oauth.client_id", "STN_CLOUDSHIP_OAUTH_CLIENT_ID")
	viper.BindEnv("cloudship.oauth.auth_url", "STN_CLOUDSHIP_OAUTH_AUTH_URL")
	viper.BindEnv("cloudship.oauth.token_url", "STN_CLOUDSHIP_OAUTH_TOKEN_URL")
	viper.BindEnv("cloudship.oauth.introspect_url", "STN_CLOUDSHIP_OAUTH_INTROSPECT_URL")
	viper.BindEnv("cloudship.oauth.redirect_uri", "STN_CLOUDSHIP_OAUTH_REDIRECT_URI")
	viper.BindEnv("cloudship.oauth.scopes", "STN_CLOUDSHIP_OAUTH_SCOPES")

	// Telemetry config
	viper.BindEnv("telemetry_enabled", "STN_TELEMETRY_ENABLED", "STATION_TELEMETRY_ENABLED")
	viper.BindEnv("telemetry.enabled", "STN_TELEMETRY_ENABLED", "STATION_TELEMETRY_ENABLED")
	viper.BindEnv("telemetry.provider", "STN_TELEMETRY_PROVIDER")
	viper.BindEnv("telemetry.endpoint", "STN_TELEMETRY_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT")
	viper.BindEnv("telemetry.service_name", "STN_TELEMETRY_SERVICE_NAME", "OTEL_SERVICE_NAME")
	viper.BindEnv("telemetry.environment", "STN_TELEMETRY_ENVIRONMENT")
	viper.BindEnv("telemetry.sample_rate", "STN_TELEMETRY_SAMPLE_RATE")
	viper.BindEnv("otel_endpoint", "OTEL_EXPORTER_OTLP_ENDPOINT", "STN_OTEL_ENDPOINT")
	viper.BindEnv("jaeger_query_url", "STN_JAEGER_QUERY_URL", "JAEGER_QUERY_URL")
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    getEnvOrDefault("DATABASE_URL", GetDatabasePath()),
		SSHPort:        getEnvIntOrDefault("SSH_PORT", 2222),
		MCPPort:        getEnvIntOrDefault("MCP_PORT", 8586),
		APIPort:        getEnvIntOrDefault("API_PORT", 8585),
		SSHHostKeyPath: getEnvOrDefault("SSH_HOST_KEY_PATH", "./ssh_host_key"),
		AdminUsername:  getEnvOrDefault("ADMIN_USERNAME", "admin"),
		Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
		Debug:          getEnvBoolOrDefault("STN_DEBUG", false), // Default to info level
		// Workspace Configuration
		Workspace: getEnvOrDefault("STATION_WORKSPACE", ""), // Custom workspace path
		// AI Provider Configuration with STN_ prefix and sane defaults
		AIProvider: getEnvOrDefault("STN_AI_PROVIDER", "openai"), // Default to OpenAI
		AIAPIKey:   getAIAPIKey(),                                // Smart fallback for API keys
		AIModel:    getAIModelDefault(),                          // Provider-specific defaults
		AIBaseURL:  getEnvOrDefault("STN_AI_BASE_URL", ""),       // Empty means use provider default
		// CloudShip Integration (disabled by default)
		// When enabled, connects to Lighthouse via TLS on port 443 (managed by Fly.io)
		CloudShip: CloudShipConfig{
			Enabled:           getEnvBoolOrDefault("STN_CLOUDSHIP_ENABLED", false),
			RegistrationKey:   getEnvOrDefault("STN_CLOUDSHIP_KEY", ""),
			Endpoint:          getEnvOrDefault("STN_CLOUDSHIP_ENDPOINT", "lighthouse.cloudshipai.com:443"),
			UseTLS:            getEnvBoolOrDefault("STN_CLOUDSHIP_USE_TLS", true), // TLS enabled by default for production
			StationID:         getEnvOrDefault("STN_CLOUDSHIP_STATION_ID", ""),
			Name:              getEnvOrDefault("STN_CLOUDSHIP_NAME", ""), // v2: user-defined station name
			BundleRegistryURL: getEnvOrDefault("STN_CLOUDSHIP_BUNDLE_REGISTRY_URL", "https://api.cloudshipai.com"),
			BaseURL:           getEnvOrDefault("STN_CLOUDSHIP_BASE_URL", "https://app.cloudshipai.com"),
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
		// Telemetry Configuration - defaults for local Jaeger development
		// Note: ApplyTelemetryDefaults() should be called after Load() to apply smart defaults
		// based on runtime mode (stdio vs serve) and CloudShip connection status
		Telemetry: TelemetryConfig{
			Enabled:        true,                    // Default enabled
			Provider:       TelemetryProviderJaeger, // Default to local Jaeger (may be overridden)
			Endpoint:       LocalJaegerEndpoint,     // Default OTLP endpoint
			JaegerQueryURL: LocalJaegerQueryURL,     // Default Jaeger Query UI
			ServiceName:    "station",               // Default service name
			Environment:    "development",           // Default environment
			SampleRate:     1.0,                     // Sample everything in dev
			Headers:        make(map[string]string),
		},
		// Legacy fields for backward compatibility
		TelemetryEnabled: true,
		OTELEndpoint:     LocalJaegerEndpoint,
		JaegerQueryURL:   LocalJaegerQueryURL,
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
	// TLS configuration
	if viper.IsSet("cloudship.use_tls") {
		cfg.CloudShip.UseTLS = viper.GetBool("cloudship.use_tls")
	}
	if viper.IsSet("cloudship.skip_tls_verify") {
		cfg.CloudShip.InsecureSkipVerify = viper.GetBool("cloudship.skip_tls_verify")
	}
	if viper.IsSet("cloudship.station_id") {
		cfg.CloudShip.StationID = viper.GetString("cloudship.station_id")
	}
	if viper.IsSet("cloudship.name") {
		cfg.CloudShip.Name = viper.GetString("cloudship.name")
	}
	if viper.IsSet("cloudship.tags") {
		cfg.CloudShip.Tags = viper.GetStringSlice("cloudship.tags")
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
	if viper.IsSet("cloudship.base_url") {
		cfg.CloudShip.BaseURL = viper.GetString("cloudship.base_url")
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

	// Telemetry configuration overrides from config file
	if viper.IsSet("telemetry.enabled") {
		cfg.Telemetry.Enabled = viper.GetBool("telemetry.enabled")
	}
	if viper.IsSet("telemetry.provider") {
		cfg.Telemetry.Provider = TelemetryProvider(viper.GetString("telemetry.provider"))
	}
	if viper.IsSet("telemetry.endpoint") {
		cfg.Telemetry.Endpoint = viper.GetString("telemetry.endpoint")
	}
	if viper.IsSet("telemetry.headers") {
		cfg.Telemetry.Headers = viper.GetStringMapString("telemetry.headers")
	}
	if viper.IsSet("telemetry.jaeger_query_url") {
		cfg.Telemetry.JaegerQueryURL = viper.GetString("telemetry.jaeger_query_url")
	}
	if viper.IsSet("telemetry.service_name") {
		cfg.Telemetry.ServiceName = viper.GetString("telemetry.service_name")
	}
	if viper.IsSet("telemetry.environment") {
		cfg.Telemetry.Environment = viper.GetString("telemetry.environment")
	}
	if viper.IsSet("telemetry.sample_rate") {
		cfg.Telemetry.SampleRate = viper.GetFloat64("telemetry.sample_rate")
	}

	// Legacy config support: migrate old fields to new Telemetry struct
	if viper.IsSet("telemetry_enabled") {
		cfg.TelemetryEnabled = viper.GetBool("telemetry_enabled")
		cfg.Telemetry.Enabled = cfg.TelemetryEnabled
	}
	if viper.IsSet("otel_endpoint") {
		cfg.OTELEndpoint = viper.GetString("otel_endpoint")
		// Auto-detect provider from endpoint
		endpoint := cfg.OTELEndpoint
		if strings.Contains(endpoint, "telemetry.cloudshipai.com") {
			cfg.Telemetry.Provider = TelemetryProviderCloudShip
		} else if strings.Contains(endpoint, "localhost") || strings.Contains(endpoint, "127.0.0.1") {
			cfg.Telemetry.Provider = TelemetryProviderJaeger
		} else {
			cfg.Telemetry.Provider = TelemetryProviderOTLP
		}
		cfg.Telemetry.Endpoint = endpoint
	}
	if viper.IsSet("jaeger_query_url") {
		cfg.JaegerQueryURL = viper.GetString("jaeger_query_url")
		cfg.Telemetry.JaegerQueryURL = cfg.JaegerQueryURL
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

	// Telemetry environment variable overrides (take precedence over config file)
	if envEnabled := os.Getenv("STN_TELEMETRY_ENABLED"); envEnabled != "" {
		if boolValue, err := strconv.ParseBool(envEnabled); err == nil {
			cfg.Telemetry.Enabled = boolValue
		}
	}
	if envProvider := os.Getenv("STN_TELEMETRY_PROVIDER"); envProvider != "" {
		cfg.Telemetry.Provider = TelemetryProvider(envProvider)
	}
	if envEndpoint := os.Getenv("STN_TELEMETRY_ENDPOINT"); envEndpoint != "" {
		cfg.Telemetry.Endpoint = envEndpoint
	}
	if envJaegerURL := os.Getenv("STN_TELEMETRY_JAEGER_QUERY_URL"); envJaegerURL != "" {
		cfg.Telemetry.JaegerQueryURL = envJaegerURL
	}
	if envServiceName := os.Getenv("STN_TELEMETRY_SERVICE_NAME"); envServiceName != "" {
		cfg.Telemetry.ServiceName = envServiceName
	}
	if envEnvironment := os.Getenv("STN_TELEMETRY_ENVIRONMENT"); envEnvironment != "" {
		cfg.Telemetry.Environment = envEnvironment
	}
	if envSampleRate := os.Getenv("STN_TELEMETRY_SAMPLE_RATE"); envSampleRate != "" {
		if floatValue, err := strconv.ParseFloat(envSampleRate, 64); err == nil {
			cfg.Telemetry.SampleRate = floatValue
		}
	}

	// Store loaded config for use by path helpers
	loadedConfig = cfg

	return cfg, nil
}

// ApplyTelemetryDefaults applies smart defaults for telemetry based on runtime mode.
//
// Logic:
//   - stdio mode: Default to local Jaeger (localhost:4318) for development
//   - serve mode with CloudShip registration key: AUTO-ENABLE telemetry and use CloudShip endpoint
//   - serve mode without CloudShip: Default to local Jaeger (if telemetry enabled)
//
// This should be called after Load() in the command handlers.
func (cfg *Config) ApplyTelemetryDefaults(isStdioMode bool) {
	// Check if user has explicitly set an endpoint (via config file or env var)
	userSetEndpoint := viper.IsSet("telemetry.endpoint") || os.Getenv("STN_TELEMETRY_ENDPOINT") != ""
	userSetEnabled := os.Getenv("STN_TELEMETRY_ENABLED") != ""

	// Check for CloudShip integration
	hasCloudShipKey := cfg.CloudShip.Enabled && cfg.CloudShip.RegistrationKey != ""

	if isStdioMode {
		// stdio mode: use local Jaeger for development (if telemetry enabled)
		if cfg.Telemetry.Enabled && !userSetEndpoint {
			cfg.Telemetry.Provider = TelemetryProviderJaeger
			cfg.Telemetry.Endpoint = LocalJaegerEndpoint
			cfg.Telemetry.JaegerQueryURL = LocalJaegerQueryURL
			cfg.Telemetry.Environment = "development"
			cfg.Telemetry.SampleRate = 1.0 // Sample everything in dev
		}
	} else {
		// serve mode
		if hasCloudShipKey {
			// CloudShip connected = AUTO-ENABLE telemetry unless user explicitly disabled it
			if !userSetEnabled {
				cfg.Telemetry.Enabled = true
				cfg.TelemetryEnabled = true
			}

			// Use CloudShip telemetry endpoint unless user specified a custom one
			if cfg.Telemetry.Enabled && !userSetEndpoint {
				cfg.Telemetry.Provider = TelemetryProviderCloudShip
				cfg.Telemetry.Endpoint = CloudShipTelemetryEndpoint
				cfg.Telemetry.Environment = "production"
				cfg.Telemetry.SampleRate = 1.0 // Sample everything, CloudShip handles storage
			}
		} else if cfg.Telemetry.Enabled && !userSetEndpoint {
			// No CloudShip, no custom endpoint = default to local Jaeger
			cfg.Telemetry.Provider = TelemetryProviderJaeger
			cfg.Telemetry.Endpoint = LocalJaegerEndpoint
			cfg.Telemetry.JaegerQueryURL = LocalJaegerQueryURL
		}
	}
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
			Model:       "gpt-5-mini",
		},
		"gcp-finops": {
			Name:        "GCP FinOps",
			Description: "GCP cloud billing and cost optimization tools",
			Instruction: "Generate comprehensive GCP Cloud Billing and Cost Management API tools for FinOps investigations. Include tools for: 1) Querying billing data and export tables (query_billing_export, get_billing_account, list_projects_billing), 2) Analyzing cost trends and anomalies (analyze_cost_spike, get_cost_forecast, compare_period_costs, detect_cost_anomalies), 3) Resource cost attribution (get_service_costs, get_project_costs, get_sku_costs, get_label_costs), 4) Budget and alert management (list_budgets, get_budget_status, get_budget_alerts, analyze_budget_variance), 5) Recommendations and optimization (get_cost_recommendations, list_idle_resources, get_commitment_analysis, analyze_sustained_use_discount), 6) Cost allocation and reporting (get_cost_breakdown_by_service, get_cost_by_region, get_cost_by_label, generate_cost_report). Each tool should accept parameters like project_id, billing_account_id, time_range (start_date, end_date), granularity (daily, weekly, monthly), filters (service, sku, region, labels), and aggregation options.",
			Model:       "gpt-5-mini",
		},
		"azure-finops": {
			Name:        "Azure FinOps",
			Description: "Azure cost management and optimization tools",
			Instruction: "Generate comprehensive Azure Cost Management API tools for FinOps investigations. Include tools for: 1) Cost queries (get_cost_and_usage, get_cost_forecast, query_cost_management), 2) Cost anomaly detection (detect_cost_anomalies, get_anomaly_alerts), 3) Resource-level analysis (get_resource_costs, get_subscription_costs, get_resource_group_costs), 4) Budget management (list_budgets, get_budget_alerts, analyze_budget_variance), 5) Recommendations (get_advisor_recommendations, get_rightsizing_recommendations, get_reserved_instance_recommendations), 6) Cost allocation (get_cost_by_tag, get_cost_by_department, get_cost_by_service). Tools should work with Azure Cost Management REST API patterns and return realistic Azure billing data.",
			Model:       "gpt-5-mini",
		},
		"datadog-monitoring": {
			Name:        "Datadog Monitoring",
			Description: "Datadog metrics, logs, and monitoring tools",
			Instruction: "Generate Datadog monitoring API tools for DevOps and observability. Include tools for: 1) Metrics queries (query_metrics, get_metric_metadata, list_active_metrics), 2) Log analysis (search_logs, get_log_aggregates, analyze_log_patterns), 3) APM traces (search_traces, get_service_performance, analyze_trace_latency), 4) Monitors and alerts (list_monitors, get_monitor_status, get_alert_history), 5) Dashboards (get_dashboard_data, query_dashboard_widgets), 6) Infrastructure monitoring (get_host_metrics, get_container_metrics, get_process_metrics). Tools should return realistic Datadog API responses with time-series data, log entries, and monitoring insights.",
			Model:       "gpt-5-mini",
		},
		"stripe-payments": {
			Name:        "Stripe Payments",
			Description: "Stripe payment and subscription API tools",
			Instruction: "Generate Stripe payment API tools for payment processing and subscription management. Include tools for: 1) Payment operations (create_payment_intent, capture_payment, refund_payment, list_payments), 2) Customer management (create_customer, update_customer, list_customers, get_customer_payment_methods), 3) Subscription handling (create_subscription, update_subscription, cancel_subscription, list_subscriptions), 4) Invoice operations (create_invoice, finalize_invoice, list_invoices, get_invoice_status), 5) Product and pricing (list_products, get_product_details, list_prices, create_price), 6) Payment analytics (get_payment_analytics, get_mrr_metrics, get_churn_analysis). Tools should return realistic Stripe API response formats with proper object structures.",
			Model:       "gpt-5-mini",
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

func getEnvFloatOrDefault(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
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
		// Any model string is passed through to OpenAI API via GenKit
		// GPT-5 family: gpt-5.2, gpt-5.1, gpt-5, gpt-5-mini, gpt-5-nano
		// GPT-4 family: gpt-4.1, gpt-4o, gpt-4o-mini, gpt-4-turbo
		// Reasoning: o4-mini, o3, o3-mini
		return "gpt-5-mini" // Fast and cost-effective default
	case "gemini":
		return "gemini-2.5-flash" // Google's latest fast model
	case "cloudflare":
		return "@cf/openai/gpt-oss-120b" // Cloudflare Workers AI GPT-OSS model
	case "ollama":
		return "llama3" // Popular local model
	default:
		return "gpt-5-mini" // Safe, cost-effective fallback
	}
}

// GetSupportedOpenAIModels returns example OpenAI models for UI suggestions
// Note: Any model string is accepted - these are just common examples
func GetSupportedOpenAIModels() []string {
	return []string{
		"gpt-5-mini",  // Default - fast and cost-effective
		"gpt-5.2",     // Latest flagship
		"gpt-4o-mini", // Previous generation
	}
}

// GetRecommendedOpenAIModels returns the default model for OpenAI
// Note: Any model string is accepted - this is just the default suggestion
func GetRecommendedOpenAIModels() map[string]string {
	return map[string]string{
		"cost_effective": "gpt-5-mini",
	}
}
