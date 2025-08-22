package services

import (
	"context"
	"fmt"
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

	// Update tracked configuration
	gp.currentProvider = strings.ToLower(cfg.AIProvider)
	gp.currentAPIKey = cfg.AIAPIKey
	gp.currentBaseURL = cfg.AIBaseURL

	// Initialize Genkit with the configured AI provider
	logging.Info("Initializing GenKit with provider: %s, model: %s", cfg.AIProvider, cfg.AIModel)
	
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
		
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(stationOpenAI))
		
	case "googlegenai", "gemini":
		logging.Debug("Setting up Google AI plugin with model: %s", cfg.AIModel)
		
		// Let GoogleAI plugin automatically pick up GEMINI_API_KEY or GOOGLE_API_KEY from environment
		// This matches the official GenKit examples approach
		geminiPlugin := &googlegenai.GoogleAI{}
		
		genkitApp, err = genkit.Init(ctx, genkit.WithPlugins(geminiPlugin))
		
	case "ollama":
		return fmt.Errorf("Ollama provider not yet supported in modular architecture")
		
	default:
		return fmt.Errorf("unsupported AI provider: %s\n\n"+
			"Supported providers:\n"+
			"  • openai: OpenAI GPT models (gpt-4, gpt-3.5-turbo, etc)\n"+
			"  • googlegenai: Google Gemini models (gemini-pro, etc)\n"+
			"  • ollama: Local Ollama models (llama3, mistral, etc)\n\n"+
			"For OpenAI-compatible providers, use 'openai' with custom STN_AI_BASE_URL", 
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