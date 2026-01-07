package config

import (
	"os"
	"testing"
)

func TestLoadSecretsFromBackend_NoBackend(t *testing.T) {
	cfg := &Config{}

	err := cfg.LoadSecretsFromBackend()
	if err != nil {
		t.Errorf("Expected no error when backend is empty, got: %v", err)
	}
}

func TestLoadSecretsFromBackend_UnknownBackend(t *testing.T) {
	cfg := &Config{
		Secrets: SecretsConfig{
			Backend: "unknown-backend",
			Path:    "some/path",
		},
	}

	err := cfg.LoadSecretsFromBackend()
	if err == nil {
		t.Error("Expected error for unknown backend")
	}
}

func TestApplySecretsToConfig(t *testing.T) {
	cfg := &Config{}

	secrets := map[string]string{
		"STN_AI_API_KEY":         "test-api-key",
		"STN_AI_PROVIDER":        "openai",
		"STN_AI_MODEL":           "gpt-4",
		"STN_CLOUDSHIP_KEY":      "test-cloudship-key",
		"STN_CLOUDSHIP_ENDPOINT": "lighthouse.example.com:443",
		"CUSTOM_ENV_VAR":         "custom-value",
	}

	applySecretsToConfig(cfg, secrets)

	if cfg.AIAPIKey != "test-api-key" {
		t.Errorf("Expected AIAPIKey to be 'test-api-key', got: %s", cfg.AIAPIKey)
	}

	if cfg.AIProvider != "openai" {
		t.Errorf("Expected AIProvider to be 'openai', got: %s", cfg.AIProvider)
	}

	if cfg.AIModel != "gpt-4" {
		t.Errorf("Expected AIModel to be 'gpt-4', got: %s", cfg.AIModel)
	}

	if cfg.CloudShip.RegistrationKey != "test-cloudship-key" {
		t.Errorf("Expected CloudShip.RegistrationKey to be 'test-cloudship-key', got: %s", cfg.CloudShip.RegistrationKey)
	}

	if cfg.CloudShip.Endpoint != "lighthouse.example.com:443" {
		t.Errorf("Expected CloudShip.Endpoint to be set, got: %s", cfg.CloudShip.Endpoint)
	}

	if os.Getenv("CUSTOM_ENV_VAR") != "custom-value" {
		t.Errorf("Expected CUSTOM_ENV_VAR env to be set")
	}

	os.Unsetenv("CUSTOM_ENV_VAR")
}

func TestApplySecretsToConfig_FallbackKeys(t *testing.T) {
	cfg := &Config{}

	secrets := map[string]string{
		"OPENAI_API_KEY": "openai-key",
	}

	applySecretsToConfig(cfg, secrets)

	if cfg.AIAPIKey != "openai-key" {
		t.Errorf("Expected AIAPIKey to fall back to OPENAI_API_KEY, got: %s", cfg.AIAPIKey)
	}
}

func TestApplySecretsToConfig_PreferSpecificKey(t *testing.T) {
	cfg := &Config{}

	secrets := map[string]string{
		"STN_AI_API_KEY": "stn-key",
		"OPENAI_API_KEY": "openai-key",
	}

	applySecretsToConfig(cfg, secrets)

	if cfg.AIAPIKey != "stn-key" {
		t.Errorf("Expected AIAPIKey to prefer STN_AI_API_KEY, got: %s", cfg.AIAPIKey)
	}
}

func TestGetSecret(t *testing.T) {
	cfg := &Config{
		Secrets: SecretsConfig{
			LoadedSecrets: map[string]string{
				"MY_SECRET": "secret-value",
			},
		},
	}

	if cfg.GetSecret("MY_SECRET") != "secret-value" {
		t.Error("Expected GetSecret to return loaded secret")
	}

	os.Setenv("ENV_SECRET", "env-value")
	defer os.Unsetenv("ENV_SECRET")

	if cfg.GetSecret("ENV_SECRET") != "env-value" {
		t.Error("Expected GetSecret to fall back to env var")
	}

	if cfg.GetSecret("NONEXISTENT") != "" {
		t.Error("Expected GetSecret to return empty for nonexistent key")
	}
}
