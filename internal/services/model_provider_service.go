package services

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"station/internal/db/repositories"
	"station/pkg/crypto"
)

// ModelProviderBootService manages model providers with database integration and encryption
type ModelProviderBootService struct {
	repos      *repositories.Repositories
	keyManager *crypto.KeyManager
	configPath string
}

// ModelProviderConfig represents the configuration loaded from file
type ModelProviderConfig struct {
	Providers map[string]*ProviderConfig `json:"providers"`
	Default   string                     `json:"default"`
}

// ProviderConfig represents configuration for a single provider
type ProviderConfig struct {
	Name     string            `json:"name"`
	BaseURL  string            `json:"base_url"`
	APIKey   string            `json:"api_key,omitempty"` // Will be encrypted in DB
	Models   map[string]ModelInfo `json:"models"`
	Headers  map[string]string `json:"headers,omitempty"` // Will be encrypted in DB
	Enabled  bool              `json:"enabled"`
	IsDefault bool             `json:"is_default,omitempty"`
}

// ModelInfo represents information about a specific model
type ModelInfo struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	ContextSize  int64   `json:"context_size"`
	MaxTokens    int64   `json:"max_tokens"`
	SupportsTools bool   `json:"supports_tools"`
	InputCost    float64 `json:"input_cost"`   // per 1M tokens
	OutputCost   float64 `json:"output_cost"`  // per 1M tokens
	Enabled      bool    `json:"enabled"`
}

// NewModelProviderBootService creates a new model provider service
func NewModelProviderBootService(repos *repositories.Repositories, keyManager *crypto.KeyManager) (*ModelProviderBootService, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, ".station", "models.json")
	
	return &ModelProviderBootService{
		repos:      repos,
		keyManager: keyManager,
		configPath: configPath,
	}, nil
}

// LoadAndSyncProvidersOnBoot loads providers from config file and environment, syncs to database
func (s *ModelProviderBootService) LoadAndSyncProvidersOnBoot(ctx context.Context) error {
	// Load configuration from file and environment
	config, err := s.loadConfiguration()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration before proceeding
	if err := s.validateConfiguration(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// TODO: Begin database transaction once repository supports it
	// For now, sync each provider individually
	for providerKey, providerConfig := range config.Providers {
		if err := s.syncProviderToDB(ctx, providerKey, providerConfig); err != nil {
			return fmt.Errorf("failed to sync provider %s: %w", providerKey, err)
		}
	}

	// Set default provider if specified
	if config.Default != "" {
		if err := s.ensureDefaultProvider(ctx, config.Default); err != nil {
			return fmt.Errorf("failed to set default provider: %w", err)
		}
	}

	return nil
}

// loadConfiguration loads configuration from file and environment variables
func (s *ModelProviderBootService) loadConfiguration() (*ModelProviderConfig, error) {
	config := &ModelProviderConfig{
		Providers: make(map[string]*ProviderConfig),
		Default:   "openai", // Default fallback
	}

	// Load from config file if it exists
	if configData, err := os.ReadFile(s.configPath); err == nil {
		if err := json.Unmarshal(configData, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Auto-detect providers from environment variables
	s.detectProvidersFromEnv(config)

	return config, nil
}

// detectProvidersFromEnv automatically detects and configures providers based on environment variables
func (s *ModelProviderBootService) detectProvidersFromEnv(config *ModelProviderConfig) {
	// OpenAI (and OpenAI-compatible providers)
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		if _, exists := config.Providers["openai"]; !exists {
			config.Providers["openai"] = &ProviderConfig{
				Name:      "OpenAI",
				BaseURL:   getEnvWithDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
				APIKey:    apiKey,
				Models:    getOpenAIModelInfo(),
				Enabled:   true,
				IsDefault: true, // Set as default if found in environment
			}
		}
	}

	// Anthropic Claude
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		if _, exists := config.Providers["anthropic"]; !exists {
			config.Providers["anthropic"] = &ProviderConfig{
				Name:    "Anthropic",
				BaseURL: getEnvWithDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
				APIKey:  apiKey,
				Models:  getAnthropicModelInfo(),
				Headers: map[string]string{
					"anthropic-version": "2023-06-01",
				},
				Enabled: true,
			}
		}
	}

	// OpenRouter (OpenAI-compatible)
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		if _, exists := config.Providers["openrouter"]; !exists {
			config.Providers["openrouter"] = &ProviderConfig{
				Name:    "OpenRouter",
				BaseURL: "https://openrouter.ai/api/v1",
				APIKey:  apiKey,
				Models:  getOpenRouterModelInfo(),
				Headers: map[string]string{
					"HTTP-Referer": getEnvWithDefault("OPENROUTER_REFERER", "https://station.local"),
					"X-Title":      getEnvWithDefault("OPENROUTER_TITLE", "Station Agent Platform"),
				},
				Enabled: true,
			}
		}
	}

	// Ollama (local, no API key needed)
	if baseURL := getEnvWithDefault("OLLAMA_BASE_URL", "http://localhost:11434"); baseURL != "" {
		if _, exists := config.Providers["ollama"]; !exists {
			config.Providers["ollama"] = &ProviderConfig{
				Name:    "Ollama",
				BaseURL: baseURL + "/v1", // Ollama uses OpenAI-compatible API at /v1
				APIKey:  "",              // Ollama doesn't require API key
				Models:  getOllamaModelInfo(),
				Enabled: true,
			}
		}
	}
}

// syncProviderToDB syncs a provider configuration to the database with encryption
func (s *ModelProviderBootService) syncProviderToDB(ctx context.Context, providerKey string, config *ProviderConfig) error {
	// Check if provider already exists
	existingProvider, err := s.repos.ModelProviders.GetByName(ctx, providerKey)
	if err == nil {
		// Provider exists, update it
		return s.updateProvider(ctx, existingProvider.ID, config)
	}

	// Provider doesn't exist, create it
	return s.createProvider(ctx, providerKey, config)
}

// createProvider creates a new provider in the database with encrypted sensitive data
func (s *ModelProviderBootService) createProvider(ctx context.Context, providerKey string, config *ProviderConfig) error {
	// Encrypt sensitive data using helper function
	encryptedAPIKey, encryptedHeaders, err := s.encryptSensitiveData(config)
	if err != nil {
		return fmt.Errorf("failed to encrypt sensitive data: %w", err)
	}

	// Create provider in database
	provider, err := s.repos.ModelProviders.Create(ctx, 
		providerKey, config.Name, config.BaseURL, 
		encryptedAPIKey, encryptedHeaders, 
		config.Enabled, config.IsDefault)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create models for this provider
	for modelKey, modelInfo := range config.Models {
		if err := s.createModel(ctx, provider.ID, modelKey, modelInfo); err != nil {
			// TODO: Add rollback mechanism when transaction support is available
			return fmt.Errorf("failed to create model %s: %w", modelKey, err)
		}
	}

	return nil
}

// updateProvider updates an existing provider in the database
func (s *ModelProviderBootService) updateProvider(ctx context.Context, providerID int64, config *ProviderConfig) error {
	// Encrypt sensitive data using helper function
	encryptedAPIKey, encryptedHeaders, err := s.encryptSensitiveData(config)
	if err != nil {
		return fmt.Errorf("failed to encrypt sensitive data: %w", err)
	}

	// Update provider in database
	if err := s.repos.ModelProviders.Update(ctx, providerID, 
		config.Name, config.BaseURL, encryptedAPIKey, 
		encryptedHeaders, config.Enabled); err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}

	// Sync models for this provider
	if err := s.syncModelsForProvider(ctx, providerID, config.Models); err != nil {
		return fmt.Errorf("failed to sync models: %w", err)
	}

	return nil
}

// createModel creates a model record in the database
func (s *ModelProviderBootService) createModel(ctx context.Context, providerID int64, modelKey string, modelInfo ModelInfo) error {
	_, err := s.repos.Models.Create(ctx, providerID, modelKey, modelInfo.Name,
		modelInfo.ContextSize, modelInfo.MaxTokens, modelInfo.SupportsTools,
		modelInfo.InputCost, modelInfo.OutputCost, modelInfo.Enabled)
	return err
}

// ensureDefaultProvider ensures the specified provider is set as default
func (s *ModelProviderBootService) ensureDefaultProvider(ctx context.Context, providerKey string) error {
	provider, err := s.repos.ModelProviders.GetByName(ctx, providerKey)
	if err != nil {
		return fmt.Errorf("default provider %s not found: %w", providerKey, err)
	}

	return s.repos.ModelProviders.SetDefault(ctx, provider.ID)
}

// DecryptProviderSecrets decrypts API key and headers for a provider
func (s *ModelProviderBootService) DecryptProviderSecrets(ctx context.Context, providerID int64) (string, map[string]string, error) {
	provider, err := s.repos.ModelProviders.GetByID(ctx, providerID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get provider: %w", err)
	}

	var apiKey string
	var headers map[string]string

	// Decrypt API key if present
	if provider.ApiKey.Valid && provider.ApiKey.String != "" {
		apiKey, err = s.decryptString(provider.ApiKey.String)
		if err != nil {
			return "", nil, fmt.Errorf("failed to decrypt API key: %w", err)
		}
	}

	// Decrypt headers if present
	if provider.Headers.Valid && provider.Headers.String != "" {
		var headersMap map[string]string
		if err := json.Unmarshal([]byte(provider.Headers.String), &headersMap); err != nil {
			return "", nil, fmt.Errorf("failed to unmarshal headers: %w", err)
		}

		if encryptedValue, exists := headersMap["encrypted"]; exists {
			// Headers are encrypted
			decryptedJSON, err := s.decryptString(encryptedValue)
			if err != nil {
				return "", nil, fmt.Errorf("failed to decrypt headers: %w", err)
			}
			if err := json.Unmarshal([]byte(decryptedJSON), &headers); err != nil {
				return "", nil, fmt.Errorf("failed to unmarshal decrypted headers: %w", err)
			}
		} else {
			// Headers are not encrypted (legacy)
			headers = headersMap
		}
	}

	return apiKey, headers, nil
}

// decryptString decrypts a string in format "keyID:base64EncodedEncryptedData"
func (s *ModelProviderBootService) decryptString(encryptedString string) (string, error) {
	parts := strings.SplitN(encryptedString, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid encrypted string format")
	}

	keyID := parts[0]
	encryptedData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 encrypted data: %w", err)
	}

	decrypted, err := s.keyManager.DecryptWithVersion(encryptedData, keyID)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt data: %w", err)
	}

	return string(decrypted), nil
}

// SaveConfigToFile saves the current database configuration to file
func (s *ModelProviderBootService) SaveConfigToFile(ctx context.Context) error {
	providers, err := s.repos.ModelProviders.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to get providers: %w", err)
	}

	config := &ModelProviderConfig{
		Providers: make(map[string]*ProviderConfig),
	}

	// Get default provider
	defaultProvider, err := s.repos.ModelProviders.GetDefault(ctx)
	if err == nil {
		config.Default = defaultProvider.Name
	}

	// Convert database providers to config format
	for _, provider := range providers {
		apiKey, headers, err := s.DecryptProviderSecrets(ctx, provider.ID)
		if err != nil {
			return fmt.Errorf("failed to decrypt secrets for provider %s: %w", provider.Name, err)
		}

		// Get models for this provider
		models, err := s.repos.Models.ListByProvider(ctx, provider.ID)
		if err != nil {
			return fmt.Errorf("failed to get models for provider %s: %w", provider.Name, err)
		}

		modelInfos := make(map[string]ModelInfo)
		for _, model := range models {
			modelInfos[model.ModelID] = ModelInfo{
				ID:            model.ModelID,
				Name:          model.Name,
				ContextSize:   model.ContextSize,
				MaxTokens:     model.MaxTokens,
				SupportsTools: model.SupportsTools.Bool,
				InputCost:     model.InputCost.Float64,
				OutputCost:    model.OutputCost.Float64,
				Enabled:       model.Enabled.Bool,
			}
		}

		config.Providers[provider.Name] = &ProviderConfig{
			Name:      provider.DisplayName,
			BaseURL:   provider.BaseUrl,
			APIKey:    apiKey, // Will be omitted from JSON due to omitempty tag
			Models:    modelInfos,
			Headers:   headers,
			Enabled:   provider.Enabled.Bool,
			IsDefault: provider.IsDefault.Bool,
		}
	}

	// Ensure directory exists with restrictive permissions
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save configuration (API keys will be omitted due to json tag)
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(s.configPath, data, 0600)
}

// encryptSensitiveData encrypts API key and headers using base64 encoding for safe storage
func (s *ModelProviderBootService) encryptSensitiveData(config *ProviderConfig) (string, map[string]string, error) {
	var encryptedAPIKey string
	var encryptedHeaders map[string]string

	// Encrypt API key if present
	if config.APIKey != "" {
		encrypted, keyID, err := s.keyManager.EncryptWithVersion([]byte(config.APIKey))
		if err != nil {
			return "", nil, fmt.Errorf("failed to encrypt API key: %w", err)
		}
		// Store encrypted data with base64 encoding and key ID
		encryptedAPIKey = fmt.Sprintf("%s:%s", keyID, base64.StdEncoding.EncodeToString(encrypted))
	}

	// Encrypt headers if present
	if len(config.Headers) > 0 {
		headersJSON, err := json.Marshal(config.Headers)
		if err != nil {
			return "", nil, fmt.Errorf("failed to marshal headers: %w", err)
		}
		encrypted, keyID, err := s.keyManager.EncryptWithVersion(headersJSON)
		if err != nil {
			return "", nil, fmt.Errorf("failed to encrypt headers: %w", err)
		}
		// Store as single encrypted string with base64 encoding and key ID
		encryptedHeaders = map[string]string{
			"encrypted": fmt.Sprintf("%s:%s", keyID, base64.StdEncoding.EncodeToString(encrypted)),
		}
	}

	return encryptedAPIKey, encryptedHeaders, nil
}

// validateConfiguration validates provider configurations before database operations
func (s *ModelProviderBootService) validateConfiguration(config *ModelProviderConfig) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	for providerKey, providerConfig := range config.Providers {
		if providerKey == "" {
			return fmt.Errorf("provider key cannot be empty")
		}
		if providerConfig.Name == "" {
			return fmt.Errorf("provider %s: name cannot be empty", providerKey)
		}
		if providerConfig.BaseURL == "" {
			return fmt.Errorf("provider %s: base URL cannot be empty", providerKey)
		}
		
		// Validate URL format
		if _, err := url.Parse(providerConfig.BaseURL); err != nil {
			return fmt.Errorf("provider %s: invalid base URL %s: %w", providerKey, providerConfig.BaseURL, err)
		}

		// Validate models exist
		if len(providerConfig.Models) == 0 {
			return fmt.Errorf("provider %s: must have at least one model", providerKey)
		}

		// Validate each model
		for modelKey, modelInfo := range providerConfig.Models {
			if modelKey == "" || modelInfo.ID == "" {
				return fmt.Errorf("provider %s: model ID cannot be empty", providerKey)
			}
			if modelInfo.Name == "" {
				return fmt.Errorf("provider %s, model %s: name cannot be empty", providerKey, modelKey)
			}
			if modelInfo.ContextSize <= 0 {
				return fmt.Errorf("provider %s, model %s: context size must be positive", providerKey, modelKey)
			}
		}
	}

	return nil
}

// syncModelsForProvider synchronizes models for a provider, handling additions/updates/removals
func (s *ModelProviderBootService) syncModelsForProvider(ctx context.Context, providerID int64, configModels map[string]ModelInfo) error {
	// Get existing models from database
	existingModels, err := s.repos.Models.ListByProvider(ctx, providerID)
	if err != nil {
		return fmt.Errorf("failed to get existing models: %w", err)
	}

	// Create map of existing models for efficient lookup
	existingModelMap := make(map[string]*sql.NullInt64)
	for _, model := range existingModels {
		existingModelMap[model.ModelID] = &sql.NullInt64{Int64: model.ID, Valid: true}
	}

	// Process each model from config
	for modelKey, modelInfo := range configModels {
		if existingID, exists := existingModelMap[modelKey]; exists {
			// Model exists, update it
			if err := s.repos.Models.Update(ctx, existingID.Int64, modelInfo.Name,
				modelInfo.ContextSize, modelInfo.MaxTokens, modelInfo.SupportsTools,
				modelInfo.InputCost, modelInfo.OutputCost, modelInfo.Enabled); err != nil {
				return fmt.Errorf("failed to update model %s: %w", modelKey, err)
			}
			// Remove from existing map to track what's been processed
			delete(existingModelMap, modelKey)
		} else {
			// Model doesn't exist, create it
			if err := s.createModel(ctx, providerID, modelKey, modelInfo); err != nil {
				return fmt.Errorf("failed to create model %s: %w", modelKey, err)
			}
		}
	}

	// Any remaining models in existingModelMap were not in config, so they should be removed
	for modelKey, modelID := range existingModelMap {
		if err := s.repos.Models.Delete(ctx, modelID.Int64); err != nil {
			return fmt.Errorf("failed to delete obsolete model %s: %w", modelKey, err)
		}
	}

	return nil
}

// Helper functions for getting default model configurations

func getOpenAIModelInfo() map[string]ModelInfo {
	return map[string]ModelInfo{
		"gpt-4o": {
			ID: "gpt-4o", Name: "GPT-4 Omni", ContextSize: 128000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 2.50, OutputCost: 10.00, Enabled: true,
		},
		"gpt-4o-mini": {
			ID: "gpt-4o-mini", Name: "GPT-4 Omni Mini", ContextSize: 128000, MaxTokens: 16384,
			SupportsTools: true, InputCost: 0.15, OutputCost: 0.60, Enabled: true,
		},
		"gpt-4-turbo": {
			ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ContextSize: 128000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 10.00, OutputCost: 30.00, Enabled: true,
		},
		"gpt-3.5-turbo": {
			ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ContextSize: 16385, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.50, OutputCost: 1.50, Enabled: true,
		},
	}
}

func getAnthropicModelInfo() map[string]ModelInfo {
	return map[string]ModelInfo{
		"claude-3-5-sonnet-20241022": {
			ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", ContextSize: 200000, MaxTokens: 8192,
			SupportsTools: true, InputCost: 3.00, OutputCost: 15.00, Enabled: true,
		},
		"claude-3-5-haiku-20241022": {
			ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", ContextSize: 200000, MaxTokens: 8192,
			SupportsTools: true, InputCost: 0.25, OutputCost: 1.25, Enabled: true,
		},
		"claude-3-opus-20240229": {
			ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", ContextSize: 200000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 15.00, OutputCost: 75.00, Enabled: true,
		},
	}
}

func getOpenRouterModelInfo() map[string]ModelInfo {
	return map[string]ModelInfo{
		"anthropic/claude-3.5-sonnet": {
			ID: "anthropic/claude-3.5-sonnet", Name: "Claude 3.5 Sonnet (OpenRouter)", ContextSize: 200000, MaxTokens: 8192,
			SupportsTools: true, InputCost: 3.00, OutputCost: 15.00, Enabled: true,
		},
		"openai/gpt-4o": {
			ID: "openai/gpt-4o", Name: "GPT-4 Omni (OpenRouter)", ContextSize: 128000, MaxTokens: 4096,
			SupportsTools: true, InputCost: 2.50, OutputCost: 10.00, Enabled: true,
		},
		"meta-llama/llama-3.1-70b-instruct": {
			ID: "meta-llama/llama-3.1-70b-instruct", Name: "Llama 3.1 70B", ContextSize: 131072, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.59, OutputCost: 0.79, Enabled: true,
		},
	}
}

func getOllamaModelInfo() map[string]ModelInfo {
	return map[string]ModelInfo{
		"llama3.1": {
			ID: "llama3.1", Name: "Llama 3.1", ContextSize: 131072, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.00, OutputCost: 0.00, Enabled: true,
		},
		"codellama": {
			ID: "codellama", Name: "Code Llama", ContextSize: 16384, MaxTokens: 4096,
			SupportsTools: false, InputCost: 0.00, OutputCost: 0.00, Enabled: true,
		},
		"qwen2.5-coder": {
			ID: "qwen2.5-coder", Name: "Qwen 2.5 Coder", ContextSize: 32768, MaxTokens: 4096,
			SupportsTools: true, InputCost: 0.00, OutputCost: 0.00, Enabled: true,
		},
	}
}