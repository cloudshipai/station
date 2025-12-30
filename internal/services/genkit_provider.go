package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"station/internal/config"
	"station/internal/genkit/anthropic_oauth"
	"station/internal/logging"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/openai/openai-go/option"
)

// GenKitProvider manages GenKit initialization and AI provider configuration
type GenKitProvider struct {
	mu              sync.RWMutex // Protect concurrent access to provider config fields
	genkitApp       *genkit.Genkit
	currentProvider string         // Track current AI provider to detect changes
	currentAPIKey   string         // Track current API key to detect changes
	currentBaseURL  string         // Track current base URL to detect changes
	openaiPlugin    *openai.OpenAI // Official GenKit v1.0.1 OpenAI plugin
	// Note: Station custom plugin code preserved in internal/genkit/ but not used in production
}

// NewGenKitProvider creates a new GenKit provider manager
func NewGenKitProvider() *GenKitProvider {
	return &GenKitProvider{}
}

// findAvailablePort finds an available port starting from the given port number
func findAvailablePort(startPort int) (int, error) {
	for port := startPort; port < startPort+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue // Port is not available, try next
		}
		ln.Close() // Close the listener, port is available
		return port, nil
	}
	return 0, fmt.Errorf("no available port found in range %d-%d", startPort, startPort+99)
}

// ensureGenkitReflectionPort disables the GenKit reflection server entirely
// This prevents port conflicts when running multiple Station instances
func (gp *GenKitProvider) ensureGenkitReflectionPort() error {
	currentEnv := os.Getenv("GENKIT_ENV")
	logging.Debug("GenKit environment check - current GENKIT_ENV: '%s'", currentEnv)

	// Check if GENKIT_ENV is already set
	if currentEnv != "" {
		logging.Debug("GENKIT_ENV already set to '%s', not overriding", currentEnv)
		return nil // Already configured, don't override user setting
	}

	// Disable the GenKit reflection server entirely by setting GENKIT_ENV to prod
	// The reflection server only starts when GENKIT_ENV=dev
	os.Setenv("GENKIT_ENV", "prod")
	logging.Debug("GenKit environment set to prod to disable reflection server (was empty)")

	return nil
}

// GetApp returns the initialized GenKit app, initializing if needed
func (gp *GenKitProvider) GetApp(ctx context.Context) (*genkit.Genkit, error) {
	if err := gp.Initialize(ctx); err != nil {
		return nil, err
	}
	return gp.genkitApp, nil
}

// Initialize initializes GenKit with the configured AI provider and telemetry
func (gp *GenKitProvider) Initialize(ctx context.Context) error {
	// Load Station configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Lock for reading current configuration
	gp.mu.RLock()
	configChanged := gp.currentProvider != strings.ToLower(cfg.AIProvider) ||
		gp.currentAPIKey != cfg.AIAPIKey ||
		gp.currentBaseURL != cfg.AIBaseURL
	alreadyInitialized := gp.genkitApp != nil
	gp.mu.RUnlock()

	// If already initialized with same config, return early
	if alreadyInitialized && !configChanged {
		return nil
	}

	// Lock for writing (initialization or reinitialization)
	gp.mu.Lock()
	defer gp.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine might have initialized)
	if gp.genkitApp != nil && !configChanged {
		return nil
	}

	// Configuration changed or first initialization - reinitialize GenKit
	if configChanged && gp.genkitApp != nil {
		logging.Info("AI provider configuration changed from %s to %s, reinitializing GenKit...",
			gp.currentProvider, strings.ToLower(cfg.AIProvider))
		// Note: GenKit doesn't provide a clean shutdown method, so we'll just replace the instance
		gp.genkitApp = nil
	}

	// Ensure GenKit reflection server has an available port before initialization
	// This prevents port conflicts when running multiple Station instances
	if err := gp.ensureGenkitReflectionPort(); err != nil {
		return fmt.Errorf("failed to configure GenKit reflection port: %w", err)
	}

	// Update tracked configuration
	gp.currentProvider = strings.ToLower(cfg.AIProvider)
	gp.currentAPIKey = cfg.AIAPIKey
	gp.currentBaseURL = cfg.AIBaseURL

	// Auto-detect provider based on model name if needed
	detectedProvider := detectProviderFromModel(cfg.AIModel, cfg.AIProvider)
	if detectedProvider != cfg.AIProvider {
		logging.Info("Auto-detected provider '%s' for model '%s' (overriding configured provider '%s')",
			detectedProvider, cfg.AIModel, cfg.AIProvider)
		cfg.AIProvider = detectedProvider
	}

	// Initialize Genkit with the configured AI provider
	logging.Info("Initializing GenKit with provider: %s, model: %s", cfg.AIProvider, cfg.AIModel)
	logging.Info("[CONFIG DEBUG] AIAuthType='%s', AIOAuthToken length=%d", cfg.AIAuthType, len(cfg.AIOAuthToken))

	// Disable telemetry by default to prevent "traces export" connection errors
	// Only enable if explicitly requested via environment variable
	if os.Getenv("GENKIT_ENABLE_TELEMETRY") == "" && !cfg.TelemetryEnabled {
		os.Setenv("OTEL_SDK_DISABLED", "true")
		// Do NOT override GENKIT_ENV here - let ensureGenkitReflectionPort handle it
		logging.Debug("Telemetry disabled by default (set GENKIT_ENABLE_TELEMETRY=true to enable)")
	}

	var genkitApp *genkit.Genkit
	switch strings.ToLower(cfg.AIProvider) {
	case "openai":
		logging.Debug("Setting up official GenKit v1.0.1 OpenAI plugin with model: %s", cfg.AIModel)

		// Build request options for official plugin
		var opts []option.RequestOption
		if cfg.AIBaseURL != "" {
			logging.Debug("Using custom OpenAI base URL with official plugin: %s", cfg.AIBaseURL)
			opts = append(opts, option.WithBaseURL(cfg.AIBaseURL))
		}

		// Use official GenKit v1.0.1 OpenAI plugin (includes our tool_call_id fixes upstream)
		openaiPlugin := &openai.OpenAI{
			APIKey: cfg.AIAPIKey,
			Opts:   opts,
		}

		// Store reference for potential future use
		gp.openaiPlugin = openaiPlugin

		// Create prompt directory for dotprompt support
		promptDir := "/tmp/station-prompts"
		_ = os.MkdirAll(promptDir, 0755)

		logging.Debug("About to call genkit.Init for official OpenAI plugin with prompt directory support, GENKIT_ENV='%s'", os.Getenv("GENKIT_ENV"))
		genkitApp = genkit.Init(ctx,
			genkit.WithPlugins(openaiPlugin),
			genkit.WithPromptDir(promptDir))
		err = nil // GenKit v1.0.1 Init doesn't return error

	case "googlegenai", "gemini":
		logging.Debug("Setting up Google AI plugin with model: %s", cfg.AIModel)

		// Create prompt directory for dotprompt support
		promptDir := "/tmp/station-prompts"
		_ = os.MkdirAll(promptDir, 0755)

		// Let GoogleAI plugin automatically pick up GEMINI_API_KEY or GOOGLE_API_KEY from environment
		// This matches the official GenKit examples approach
		geminiPlugin := &googlegenai.GoogleAI{}

		genkitApp = genkit.Init(ctx,
			genkit.WithPlugins(geminiPlugin),
			genkit.WithPromptDir(promptDir))
		err = nil // GenKit v1.0.1 Init doesn't return error

	case "anthropic":
		logging.Debug("Setting up Anthropic plugin with model: %s, auth_type: %s", cfg.AIModel, cfg.AIAuthType)

		promptDir := "/tmp/station-prompts"
		_ = os.MkdirAll(promptDir, 0755)

		// Always use our custom native Anthropic plugin for FULL tool support
		// The GenKit compat_oai/anthropic plugin explicitly disables tools because
		// "Anthropic supports tool use, but it's not compatible with the OpenAI API"
		var anthropicPlugin *anthropic_oauth.AnthropicOAuth

		if cfg.AIAuthType == "oauth" && cfg.AIOAuthToken != "" {
			logging.Info("Using native Anthropic plugin with OAuth authentication (full tool support)")
			anthropicPlugin = &anthropic_oauth.AnthropicOAuth{
				OAuthToken: cfg.AIOAuthToken,
			}
		} else if cfg.AIAPIKey != "" {
			logging.Info("Using native Anthropic plugin with API key authentication (full tool support)")
			anthropicPlugin = &anthropic_oauth.AnthropicOAuth{
				APIKey: cfg.AIAPIKey,
			}
		} else {
			return fmt.Errorf("Anthropic provider requires either OAuth token (ai_oauth_token) or API key (ANTHROPIC_API_KEY)")
		}

		logging.Debug("Initializing GenKit with native Anthropic plugin...")
		genkitApp = genkit.Init(ctx,
			genkit.WithPlugins(anthropicPlugin),
			genkit.WithPromptDir(promptDir))
		err = nil

	default:
		return fmt.Errorf("unsupported AI provider: %s\n\n"+
			"Station automatically detects providers based on model names:\n"+
			"  • gemini-*: Routes to Google Gemini provider\n"+
			"  • claude-*: Routes to Anthropic provider\n"+
			"  • gpt-*, llama*, etc: Routes to OpenAI-compatible provider\n\n"+
			"Supported configurations:\n"+
			"  • OpenAI models: Use any gpt-* model name with OPENAI_API_KEY\n"+
			"  • Anthropic models: Use any claude-* model name with ANTHROPIC_API_KEY\n"+
			"  • Gemini models: Use any gemini-* model name with GEMINI_API_KEY or GOOGLE_API_KEY\n"+
			"  • OpenAI-compatible APIs: Use any model name with ai_base_url configured\n"+
			"    Examples: Ollama, Together AI, etc.\n\n"+
			"Set ai_base_url in config.yml or use --base-url with 'stn init' for custom endpoints.",
			cfg.AIProvider)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize GenKit with provider %s: %w", cfg.AIProvider, err)
	}

	gp.genkitApp = genkitApp
	return nil
}

// SetOpenAILogCallback sets the logging callback for the OpenAI plugin (if using OpenAI provider)
func (gp *GenKitProvider) SetOpenAILogCallback(callback func(map[string]interface{})) {
	// Official GenKit plugin may not support custom logging callbacks
	// This functionality may need to be implemented differently or via GenKit's built-in logging
	logging.Debug("Logging callback not yet implemented for official GenKit OpenAI plugin")
}

// GetPluginInfo returns information about the currently configured plugin
func (gp *GenKitProvider) GetPluginInfo() map[string]interface{} {
	return map[string]interface{}{
		"provider":      gp.currentProvider,
		"plugin_type":   "Official GenKit v1.0.1 OpenAI Plugin",
		"plugin_status": gp.openaiPlugin != nil,
		"note":          "Station custom plugin code preserved but not used in production",
	}
}

func detectProviderFromModel(modelName, configuredProvider string) string {
	modelLower := strings.ToLower(modelName)

	if strings.HasPrefix(modelLower, "gemini") {
		return "gemini"
	}

	if strings.HasPrefix(modelLower, "claude") {
		return "anthropic"
	}

	return configuredProvider
}
