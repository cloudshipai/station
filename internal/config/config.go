package config

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/viper"
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
	// Note: Station now uses official GenKit v1.0.1 plugins (custom plugin code preserved)
}

// CloudShipConfig holds CloudShip Lighthouse integration settings
type CloudShipConfig struct {
	Enabled           bool   `yaml:"enabled"`             // Enable CloudShip integration
	RegistrationKey   string `yaml:"registration_key"`    // CloudShip registration key
	Endpoint          string `yaml:"endpoint"`            // Lighthouse gRPC endpoint
	StationID         string `yaml:"station_id"`          // Station ID (auto-generated)
	BundleRegistryURL string `yaml:"bundle_registry_url"` // Bundle registry API URL
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
		TelemetryEnabled: getEnvBoolOrDefault("TELEMETRY_ENABLED", true),     // Default enabled with opt-out
		OTELEndpoint:     getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""), // Default empty (no export)
		Debug:            getEnvBoolOrDefault("STN_DEBUG", false),            // Default to info level
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
	if viper.IsSet("cloudship.station_id") {
		cfg.CloudShip.StationID = viper.GetString("cloudship.station_id")
	}
	if viper.IsSet("cloudship.bundle_registry_url") {
		cfg.CloudShip.BundleRegistryURL = viper.GetString("cloudship.bundle_registry_url")
	}

	return cfg, nil
}

// GetStationConfigDir returns the station configuration directory path
// This respects workspace configuration and falls back to XDG paths
func GetStationConfigDir() string {
	// Check if workspace is configured via viper (CLI override or config file)
	if workspace := viper.GetString("workspace"); workspace != "" {
		return workspace
	}

	// Fall back to XDG config directory
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
