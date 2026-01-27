package deploy

import (
	"fmt"
	"strings"
)

// BuildAllSecrets builds all secrets for deployment to any target
func BuildAllSecrets(aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, runtimeSecrets *RuntimeSecretsConfig) map[string]string {
	secrets := make(map[string]string)

	if runtimeSecrets != nil && runtimeSecrets.Backend != "" {
		secrets["STN_SECRETS_BACKEND"] = runtimeSecrets.Backend
		if runtimeSecrets.Path != "" {
			secrets["STN_SECRETS_PATH"] = runtimeSecrets.Path
		}
	}

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		secrets["STN_CLOUDSHIP_ENABLED"] = "true"
		secrets["STN_CLOUDSHIP_KEY"] = cloudShipConfig.RegistrationKey
		if cloudShipConfig.Name != "" {
			secrets["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			secrets["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
		if cloudShipConfig.UseTLS {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "true"
		}
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	for k, v := range externalSecrets {
		secrets[k] = v
	}

	return secrets
}

// BuildFlySecrets builds secrets for Fly.io deployment
func BuildFlySecrets(aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig) (map[string]string, error) {
	secrets := make(map[string]string)

	encryptionKey, err := GenerateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	secrets["STATION_ENCRYPTION_KEY"] = encryptionKey

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		case "groq":
			secrets["GROQ_API_KEY"] = aiConfig.APIKey
		case "openrouter":
			secrets["OPENROUTER_API_KEY"] = aiConfig.APIKey
		case "together":
			secrets["TOGETHER_API_KEY"] = aiConfig.APIKey
		case "fireworks":
			secrets["FIREWORKS_API_KEY"] = aiConfig.APIKey
		case "ollama":
			secrets["OLLAMA_BASE_URL"] = "http://localhost:11434"
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		secrets["STN_CLOUDSHIP_ENABLED"] = "true"
		secrets["STN_CLOUDSHIP_KEY"] = cloudShipConfig.RegistrationKey
		if cloudShipConfig.Name != "" {
			secrets["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			secrets["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
		if cloudShipConfig.UseTLS {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "true"
		} else {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "false"
		}
	}

	telemetryConfig := DetectTelemetryConfig()
	if telemetryConfig != nil {
		isLocalEndpoint := telemetryConfig.Endpoint == "" ||
			strings.Contains(telemetryConfig.Endpoint, "localhost") ||
			strings.Contains(telemetryConfig.Endpoint, "127.0.0.1")

		if isLocalEndpoint {
			secrets["STN_TELEMETRY_PROVIDER"] = "cloudship"
			secrets["STN_TELEMETRY_ENDPOINT"] = "https://telemetry.cloudshipai.com/v1/traces"
		} else {
			secrets["STN_TELEMETRY_PROVIDER"] = telemetryConfig.Provider
			secrets["STN_TELEMETRY_ENDPOINT"] = telemetryConfig.Endpoint
		}
		if telemetryConfig.Enabled {
			secrets["STN_TELEMETRY_ENABLED"] = "true"
		}
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	secrets["STATION_MCP_POOLING"] = "true"

	return secrets, nil
}

// BuildBundleFlySecrets builds secrets for Fly.io bundle deployment
func BuildBundleFlySecrets(bundleID string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig) (map[string]string, error) {
	secrets := make(map[string]string)

	secrets["STN_BUNDLE_ID"] = bundleID

	encryptionKey, err := GenerateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	secrets["STATION_ENCRYPTION_KEY"] = encryptionKey

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		secrets["STN_CLOUDSHIP_ENABLED"] = "true"
		secrets["STN_CLOUDSHIP_KEY"] = cloudShipConfig.RegistrationKey
		if cloudShipConfig.Name != "" {
			secrets["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			secrets["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
		if cloudShipConfig.UseTLS {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "true"
		}
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	secrets["STATION_MCP_POOLING"] = "true"

	return secrets, nil
}

// BuildOpenCodeSecrets builds secrets for OpenCode sidecar deployment
func BuildOpenCodeSecrets(aiConfig *DeploymentAIConfig) map[string]string {
	secrets := make(map[string]string)

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model
	secrets["OPENCODE_AUTO_APPROVE"] = "true"

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		}
	}

	return secrets
}
