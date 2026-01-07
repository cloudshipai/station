package config

import (
	"context"
	"fmt"
	"os"
	"time"

	"station/internal/deployment"
	"station/internal/deployment/secrets"
)

func (cfg *Config) LoadSecretsFromBackend() error {
	if cfg.Secrets.Backend == "" {
		return nil
	}

	provider, ok := deployment.GetSecretProvider(cfg.Secrets.Backend)
	if !ok {
		return fmt.Errorf("unknown secrets backend: %s (supported: aws-secretsmanager, aws-ssm, vault, gcp-secretmanager, sops)", cfg.Secrets.Backend)
	}

	switch p := provider.(type) {
	case *secrets.VaultProvider:
		if cfg.Secrets.VaultAddr != "" {
			p.WithAddr(cfg.Secrets.VaultAddr)
		}
		if cfg.Secrets.VaultToken != "" {
			p.WithToken(cfg.Secrets.VaultToken)
		}
	case *secrets.AWSSecretsManagerProvider:
		if cfg.Secrets.Region != "" {
			p.WithRegion(cfg.Secrets.Region)
		}
	case *secrets.AWSSSMProvider:
		if cfg.Secrets.Region != "" {
			p.WithRegion(cfg.Secrets.Region)
		}
	case *secrets.GCPSecretManagerProvider:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := provider.Validate(ctx); err != nil {
		return fmt.Errorf("secrets backend validation failed: %w", err)
	}

	secrets, err := provider.GetSecrets(ctx, cfg.Secrets.Path)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets from %s: %w", cfg.Secrets.Backend, err)
	}

	cfg.Secrets.LoadedSecrets = secrets
	cfg.Secrets.Loaded = true

	applySecretsToConfig(cfg, secrets)

	return nil
}

func applySecretsToConfig(cfg *Config, secrets map[string]string) {
	if v, ok := secrets["STN_AI_API_KEY"]; ok && v != "" {
		cfg.AIAPIKey = v
	}
	if v, ok := secrets["OPENAI_API_KEY"]; ok && v != "" && cfg.AIAPIKey == "" {
		cfg.AIAPIKey = v
	}
	if v, ok := secrets["ANTHROPIC_API_KEY"]; ok && v != "" && cfg.AIAPIKey == "" {
		cfg.AIAPIKey = v
	}
	if v, ok := secrets["GOOGLE_API_KEY"]; ok && v != "" && cfg.AIAPIKey == "" {
		cfg.AIAPIKey = v
	}

	if v, ok := secrets["STN_CLOUDSHIP_KEY"]; ok && v != "" {
		cfg.CloudShip.RegistrationKey = v
	}
	if v, ok := secrets["CLOUDSHIP_REGISTRATION_KEY"]; ok && v != "" && cfg.CloudShip.RegistrationKey == "" {
		cfg.CloudShip.RegistrationKey = v
	}

	for k, v := range secrets {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func (cfg *Config) GetSecret(key string) string {
	if cfg.Secrets.LoadedSecrets != nil {
		if v, ok := cfg.Secrets.LoadedSecrets[key]; ok {
			return v
		}
	}
	return os.Getenv(key)
}
