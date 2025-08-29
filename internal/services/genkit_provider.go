package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"station/internal/config"
	stationGenkit "station/internal/genkit"
	"station/internal/logging"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

// GenKitProvider manages GenKit initialization and AI provider configuration
type GenKitProvider struct {
	genkitApp       *genkit.Genkit
	currentProvider string // Track current AI provider to detect changes
	currentAPIKey   string // Track current API key to detect changes
	currentBaseURL  string // Track current base URL to detect changes
	openaiPlugin    *stationGenkit.StationOpenAI // Keep reference to OpenAI plugin for logging callbacks
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

	// Check if configuration has changed
	configChanged := gp.currentProvider != strings.ToLower(cfg.AIProvider) ||
		gp.currentAPIKey != cfg.AIAPIKey ||
		gp.currentBaseURL != cfg.AIBaseURL

	// If already initialized with same config, return early
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
		logging.Debug("Setting up Station's fixed OpenAI plugin with model: %s", cfg.AIModel)
		
		// Use Station's fixed OpenAI plugin that handles tool_call_id properly
		stationOpenAI := &stationGenkit.StationOpenAI{
			APIKey: cfg.AIAPIKey,
		}
		
		// Set base URL if provided for OpenAI-compatible APIs
		if cfg.AIBaseURL != "" {
			stationOpenAI.BaseURL = cfg.AIBaseURL
			logging.Debug("Using custom OpenAI base URL: %s", cfg.AIBaseURL)
		}
		
		// Store reference to plugin for logging callbacks
		gp.openaiPlugin = stationOpenAI
		
		logging.Debug("About to call genkit.Init for OpenAI with GENKIT_ENV='%s'", os.Getenv("GENKIT_ENV"))
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(stationOpenAI))
		
	case "googlegenai", "gemini":
		logging.Debug("Setting up Google AI plugin with model: %s", cfg.AIModel)
		
		// Let GoogleAI plugin automatically pick up GEMINI_API_KEY or GOOGLE_API_KEY from environment
		// This matches the official GenKit examples approach
		geminiPlugin := &googlegenai.GoogleAI{}
		
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(geminiPlugin))
		
	default:
		return fmt.Errorf("unsupported AI provider: %s\n\n"+
			"Station automatically detects providers based on model names:\n"+
			"  • gemini-*: Routes to Google Gemini provider\n"+
			"  • gpt-*, claude-*, llama*, etc: Routes to OpenAI-compatible provider\n\n"+
			"Supported configurations:\n"+
			"  • OpenAI models: Use any gpt-* model name with OPENAI_API_KEY\n"+
			"  • Gemini models: Use any gemini-* model name with GEMINI_API_KEY or GOOGLE_API_KEY\n"+
			"  • OpenAI-compatible APIs: Use any model name with ai_base_url configured\n"+
			"    Examples: Anthropic, Ollama, Together AI, etc.\n\n"+
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
	if gp.openaiPlugin != nil {
		gp.openaiPlugin.SetLogCallback(callback)
	}
}

// detectProviderFromModel auto-detects the provider based on model name
func detectProviderFromModel(modelName, configuredProvider string) string {
	modelLower := strings.ToLower(modelName)
	
	// If model starts with gemini-, it's definitely a Gemini model
	if strings.HasPrefix(modelLower, "gemini") {
		return "gemini"
	}
	
	// Known Gemini models
	geminiModels := []string{
		"gemini-pro", "gemini-pro-vision", "gemini-1.5-pro", "gemini-1.5-flash",
		"gemini-2.0-flash", "gemini-2.5-flash", "gemini-2.5-pro",
	}
	for _, geminiModel := range geminiModels {
		if modelLower == geminiModel {
			return "gemini"
		}
	}
	
	// For all other models (gpt-*, claude-*, llama*, etc.), use OpenAI-compatible
	// This allows Station to work with:
	// - OpenAI models (gpt-4, gpt-3.5-turbo, etc.)
	// - Anthropic models via OpenAI-compatible API (claude-3-sonnet, etc.)
	// - Local models via Ollama (llama3, mistral, etc.)
	// - Any other OpenAI-compatible endpoint
	return "openai"
}