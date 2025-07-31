package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino-ext/components/model/openai"
)

// ModelProvider represents a model provider configuration
type ModelProvider struct {
	Name     string            `json:"name"`
	BaseURL  string            `json:"base_url"`
	APIKey   string            `json:"api_key,omitempty"` // omitted from JSON if empty for security
	Models   map[string]Model  `json:"models"`
	Headers  map[string]string `json:"headers,omitempty"`
	Enabled  bool              `json:"enabled"`
}

// Model represents a specific model configuration
type Model struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ContextSize  int     `json:"context_size"`
	MaxTokens    int     `json:"max_tokens"`
	SupportsTools bool   `json:"supports_tools"`
	InputCost    float64 `json:"input_cost"`   // per 1M tokens
	OutputCost   float64 `json:"output_cost"`  // per 1M tokens
}

// ModelConfig represents the entire model configuration
type ModelConfig struct {
	Providers map[string]*ModelProvider `json:"providers"`
	Default   string                    `json:"default"` // default provider
}

// ModelProviderService manages model providers and creates chat models
type ModelProviderService struct {
	config *ModelConfig
	configPath string
}

// NewModelProviderService creates a new model provider service
func NewModelProviderService() *ModelProviderService {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".station", "models.json")
	
	service := &ModelProviderService{
		configPath: configPath,
	}
	
	// Load configuration
	service.loadConfig()
	
	return service
}

// loadConfig loads model configuration from file and environment variables
func (s *ModelProviderService) loadConfig() {
	// Default configuration
	s.config = &ModelConfig{
		Providers: make(map[string]*ModelProvider),
		Default:   "openai",
	}
	
	// Load from config file if it exists
	if configData, err := os.ReadFile(s.configPath); err == nil {
		json.Unmarshal(configData, s.config)
	}
	
	// Auto-detect providers from environment variables
	s.detectProvidersFromEnv()
}

// detectProvidersFromEnv automatically detects and configures providers based on environment variables
func (s *ModelProviderService) detectProvidersFromEnv() {
	// OpenAI (and OpenAI-compatible providers)
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		if _, exists := s.config.Providers["openai"]; !exists {
			s.config.Providers["openai"] = &ModelProvider{
				Name:    "OpenAI",
				BaseURL: getEnvWithDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
				APIKey:  apiKey,
				Models:  getOpenAIModels(),
				Enabled: true,
			}
		}
	}
	
	// Anthropic Claude
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		if _, exists := s.config.Providers["anthropic"]; !exists {
			s.config.Providers["anthropic"] = &ModelProvider{
				Name:    "Anthropic",
				BaseURL: getEnvWithDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
				APIKey:  apiKey,
				Models:  getAnthropicModels(),
				Headers: map[string]string{
					"anthropic-version": "2023-06-01",
				},
				Enabled: true,
			}
		}
	}
	
	// OpenRouter (OpenAI-compatible)
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		if _, exists := s.config.Providers["openrouter"]; !exists {
			s.config.Providers["openrouter"] = &ModelProvider{
				Name:    "OpenRouter",
				BaseURL: "https://openrouter.ai/api/v1",
				APIKey:  apiKey,
				Models:  getOpenRouterModels(),
				Headers: map[string]string{
					"HTTP-Referer": getEnvWithDefault("OPENROUTER_REFERER", "https://station.local"),
					"X-Title": getEnvWithDefault("OPENROUTER_TITLE", "Station Agent Platform"),
				},
				Enabled: true,
			}
		}
	}
	
	// Ollama (local, no API key needed)
	if baseURL := getEnvWithDefault("OLLAMA_BASE_URL", "http://localhost:11434"); baseURL != "" {
		if _, exists := s.config.Providers["ollama"]; !exists {
			s.config.Providers["ollama"] = &ModelProvider{
				Name:    "Ollama",
				BaseURL: baseURL + "/v1", // Ollama uses OpenAI-compatible API at /v1
				APIKey:  "", // Ollama doesn't require API key
				Models:  getOllamaModels(),
				Enabled: true,
			}
		}
	}
}

// CreateChatModel creates a chat model for the given provider and model ID using proper Eino implementations
func (s *ModelProviderService) CreateChatModel(providerName, modelID string) (model.ToolCallingChatModel, error) {
	provider, exists := s.config.Providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}
	
	if !provider.Enabled {
		return nil, fmt.Errorf("provider %s is disabled", providerName)
	}
	
	modelConfig, exists := provider.Models[modelID]
	if !exists {
		return nil, fmt.Errorf("model %s not found in provider %s", modelID, providerName)
	}
	
	ctx := context.Background()
	
	// Create the appropriate chat model based on provider using Eino implementations
	switch providerName {
	case "openai", "openrouter":
		// Use Eino's OpenAI implementation
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			BaseURL: provider.BaseURL,
			APIKey:  provider.APIKey,
			Model:   modelConfig.ID,
		})
	case "anthropic":
		// For now, return an error as we need Anthropic implementation from eino-ext
		return nil, fmt.Errorf("Anthropic provider not yet implemented with Eino - use OpenAI for now")
	case "ollama":
		// Ollama uses OpenAI-compatible API, so use the OpenAI implementation
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			BaseURL: provider.BaseURL,
			APIKey:  provider.APIKey, // Empty for Ollama
			Model:   modelConfig.ID,
		})
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}

// CreateDefaultChatModel creates a chat model using the default provider and a suitable model
func (s *ModelProviderService) CreateDefaultChatModel() (model.ToolCallingChatModel, error) {
	if s.config.Default == "" {
		return nil, fmt.Errorf("no default provider configured")
	}
	
	provider := s.config.Providers[s.config.Default]
	if provider == nil {
		return nil, fmt.Errorf("default provider %s not found", s.config.Default)
	}
	
	// Find a suitable model (prefer one that supports tools)
	var bestModel string
	for modelID, modelConfig := range provider.Models {
		if modelConfig.SupportsTools {
			bestModel = modelID
			break
		}
		if bestModel == "" {
			bestModel = modelID
		}
	}
	
	if bestModel == "" {
		return nil, fmt.Errorf("no models available in provider %s", s.config.Default)
	}
	
	return s.CreateChatModel(s.config.Default, bestModel)
}

// GetProviders returns all configured providers
func (s *ModelProviderService) GetProviders() map[string]*ModelProvider {
	return s.config.Providers
}

// SaveConfig saves the current configuration to file
func (s *ModelProviderService) SaveConfig() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0755); err != nil {
		return err
	}
	
	// Save configuration (API keys will be omitted due to json tag)
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(s.configPath, data, 0600)
}

// Helper functions for getting default model configurations

func getOpenAIModels() map[string]Model {
	return map[string]Model{
		"gpt-4o": {
			ID: "gpt-4o", Name: "GPT-4 Omni", ContextSize: 128000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 2.50, OutputCost: 10.00,
		},
		"gpt-4o-mini": {
			ID: "gpt-4o-mini", Name: "GPT-4 Omni Mini", ContextSize: 128000, MaxTokens: 16384,
			SupportsTools: true, InputCost: 0.15, OutputCost: 0.60,
		},
		"gpt-4-turbo": {
			ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ContextSize: 128000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 10.00, OutputCost: 30.00,
		},
		"gpt-3.5-turbo": {
			ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ContextSize: 16385, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.50, OutputCost: 1.50,
		},
	}
}

func getAnthropicModels() map[string]Model {
	return map[string]Model{
		"claude-3-5-sonnet-20241022": {
			ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextSize: 200000, MaxTokens: 8192,
			SupportsTools: true, InputCost: 3.00, OutputCost: 15.00,
		},
		"claude-3-5-haiku-20241022": {
			ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextSize: 200000, MaxTokens: 8192,
			SupportsTools: true, InputCost: 0.25, OutputCost: 1.25,
		},
		"claude-3-opus-20240229": {
			ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ContextSize: 200000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 15.00, OutputCost: 75.00,
		},
	}
}

func getOpenRouterModels() map[string]Model {
	return map[string]Model{
		"anthropic/claude-3.5-sonnet": {
			ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet (OpenRouter)", ContextSize: 200000, MaxTokens: 8192,
			SupportsTools: true, InputCost: 3.00, OutputCost: 15.00,
		},
		"openai/gpt-4o": {
			ID: "openai/gpt-4o", Name: "GPT-4 Omni (OpenRouter)", ContextSize: 128000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 2.50, OutputCost: 10.00,
		},
		"meta-llama/llama-3.1-70b-instruct": {
			ID: "meta-llama/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", ContextSize: 131072, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.59, OutputCost: 0.79,
		},
	}
}

func getOllamaModels() map[string]Model {
	// Default Ollama models - users can customize this in their config
	return map[string]Model{
		"llama3.1": {
			ID: "llama3.1", Name: "Llama 3.1", ContextSize: 131072, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.00, OutputCost: 0.00, // Local, no cost
		},
		"codellama": {
			ID: "codellama", Name: "Code Llama", ContextSize: 16384, MaxTokens: 4096,
			SupportsTools: false, InputCost: 0.00, OutputCost: 0.00,
		},
		"qwen2.5-coder": {
			ID: "qwen2.5-coder", Name: "Qwen 2.5 Coder", ContextSize: 32768, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.00, OutputCost: 0.00,
		},
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}