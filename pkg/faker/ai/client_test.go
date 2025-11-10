package ai

import (
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
	cfg := &config.Config{
		AIProvider: "gemini",
		AIModel:    "gemini-pro",
		AIAPIKey:   "test-key",
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
	assert.Equal(t, "openai/gpt-4o-mini", modelName)
}

func TestGetDefaultModel(t *testing.T) {
	tests := []struct {
		provider string
		expected string
	}{
		{"openai", "gpt-4o-mini"},
		{"gemini", "gemini-1.5-flash"},
		{"googlegenai", "gemini-1.5-flash"},
		{"unknown", "gpt-4o-mini"}, // Fallback
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			model := GetDefaultModel(tt.provider)
			assert.Equal(t, tt.expected, model)
		})
	}
}
