package ai

import (
	"os"
	"testing"

	"station/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient_InvalidConfig(t *testing.T) {
	_, err := NewClient(nil, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config cannot be nil")
}

func TestNewClient_UnsupportedProvider(t *testing.T) {
	cfg := &config.Config{
		AIProvider: "unsupported",
		AIAPIKey:   "test-key",
	}

	_, err := NewClient(cfg, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported AI provider")
}

func TestGetModelName_OpenAI(t *testing.T) {
	cfg := &config.Config{
		AIProvider: "openai",
		AIModel:    "gpt-4",
		AIAPIKey:   "test-key",
	}

	client, err := NewClient(cfg, false)
	require.NoError(t, err)

	modelName := client.GetModelName()
	assert.Equal(t, "openai/gpt-4", modelName)
}

func TestGetModelName_Gemini(t *testing.T) {
	// Skip if no real Gemini API key is set - the genkit library panics without it
	if os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("Skipping Gemini test: GEMINI_API_KEY or GOOGLE_API_KEY not set")
	}

	cfg := &config.Config{
		AIProvider: "gemini",
		AIModel:    "gemini-pro",
		AIAPIKey:   os.Getenv("GEMINI_API_KEY"),
	}
	if cfg.AIAPIKey == "" {
		cfg.AIAPIKey = os.Getenv("GOOGLE_API_KEY")
	}

	client, err := NewClient(cfg, false)
	require.NoError(t, err)

	modelName := client.GetModelName()
	assert.Equal(t, "googleai/gemini-pro", modelName)
}

func TestGetModelName_DefaultModel(t *testing.T) {
	cfg := &config.Config{
		AIProvider: "openai",
		AIModel:    "", // Empty model should use default
		AIAPIKey:   "test-key",
	}

	client, err := NewClient(cfg, false)
	require.NoError(t, err)

	modelName := client.GetModelName()
	assert.Equal(t, "openai/gpt-5-mini", modelName)
}

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{"openai", "gpt-5-mini"},
		{"gemini", "gemini-1.5-flash"},
		{"googlegenai", "gemini-1.5-flash"},
		{"unknown", "gpt-5-mini"}, // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			model := GetDefaultModel(tt.provider)
			assert.Equal(t, tt.expected, model)
		})
	}
}
