package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"station/internal/deployment"
	"station/internal/deployment/secrets"
)

func (cfg *Config) LoadSecretsFromBackend() error {
	if cfg.Secrets.Backend == "" {
		return nil
	}

	log.Printf("🔐 Loading secrets from backend: %s (path: %s)", cfg.Secrets.Backend, cfg.Secrets.Path)

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

	log.Printf("✅ Loaded %d secrets from %s", len(secrets), cfg.Secrets.Backend)

	return nil
}

func applySecretsToConfig(cfg *Config, secrets map[string]string) {
	var injectedKeys []string

	if v, ok := secrets["STN_AI_API_KEY"]; ok && v != "" {
		cfg.AIAPIKey = v
		injectedKeys = append(injectedKeys, "STN_AI_API_KEY→config")
	}
	if v, ok := secrets["OPENAI_API_KEY"]; ok && v != "" && cfg.AIAPIKey == "" {
		cfg.AIAPIKey = v
		injectedKeys = append(injectedKeys, "OPENAI_API_KEY→config")
	}
	if v, ok := secrets["ANTHROPIC_API_KEY"]; ok && v != "" && cfg.AIAPIKey == "" {
		cfg.AIAPIKey = v
		injectedKeys = append(injectedKeys, "ANTHROPIC_API_KEY→config")
	}
	if v, ok := secrets["GOOGLE_API_KEY"]; ok && v != "" && cfg.AIAPIKey == "" {
		cfg.AIAPIKey = v
		injectedKeys = append(injectedKeys, "GOOGLE_API_KEY→config")
	}

	if v, ok := secrets["STN_AI_PROVIDER"]; ok && v != "" && cfg.AIProvider == "" {
		cfg.AIProvider = v
		injectedKeys = append(injectedKeys, "STN_AI_PROVIDER→config")
	}
	if v, ok := secrets["STN_AI_MODEL"]; ok && v != "" && cfg.AIModel == "" {
		cfg.AIModel = v
		injectedKeys = append(injectedKeys, "STN_AI_MODEL→config")
	}
	if v, ok := secrets["STN_AI_BASE_URL"]; ok && v != "" && cfg.AIBaseURL == "" {
		cfg.AIBaseURL = v
		injectedKeys = append(injectedKeys, "STN_AI_BASE_URL→config")
	}

	if v, ok := secrets["STN_CLOUDSHIP_KEY"]; ok && v != "" {
		cfg.CloudShip.RegistrationKey = v
		injectedKeys = append(injectedKeys, "STN_CLOUDSHIP_KEY→config")
	}
	if v, ok := secrets["CLOUDSHIP_REGISTRATION_KEY"]; ok && v != "" && cfg.CloudShip.RegistrationKey == "" {
		cfg.CloudShip.RegistrationKey = v
		injectedKeys = append(injectedKeys, "CLOUDSHIP_REGISTRATION_KEY→config")
	}
	if v, ok := secrets["STN_CLOUDSHIP_ENDPOINT"]; ok && v != "" && cfg.CloudShip.Endpoint == "" {
		cfg.CloudShip.Endpoint = v
		injectedKeys = append(injectedKeys, "STN_CLOUDSHIP_ENDPOINT→config")
	}
	if v, ok := secrets["STN_CLOUDSHIP_NAME"]; ok && v != "" && cfg.CloudShip.Name == "" {
		cfg.CloudShip.Name = v
		injectedKeys = append(injectedKeys, "STN_CLOUDSHIP_NAME→config")
	}

	for k, v := range secrets {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
			injectedKeys = append(injectedKeys, k+"→env")
		}
	}

	if len(injectedKeys) > 0 {
		log.Printf("   Injected: %v", injectedKeys)
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
