package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"station/internal/config"
)

// DetectAIConfig detects AI configuration from Station config
func DetectAIConfig() (*DeploymentAIConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load Station config: %w", err)
	}

	aiConfig := &DeploymentAIConfig{
		Provider:          cfg.AIProvider,
		Model:             cfg.AIModel,
		APIKey:            cfg.AIAPIKey,
		AuthType:          cfg.AIAuthType,
		OAuthToken:        cfg.AIOAuthToken,
		OAuthRefreshToken: cfg.AIOAuthRefreshToken,
		OAuthExpiresAt:    cfg.AIOAuthExpiresAt,
	}

	if aiConfig.AuthType == "oauth" {
		if aiConfig.OAuthRefreshToken == "" {
			return nil, fmt.Errorf("OAuth auth type but no refresh token found in config")
		}

		// Refresh the OAuth token before deploying to ensure it's valid
		fmt.Printf("🔄 Refreshing OAuth token for deployment...\n")
		newToken, newRefresh, newExpires, err := config.RefreshOAuthToken(aiConfig.OAuthRefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh OAuth token: %w", err)
		}

		aiConfig.OAuthToken = newToken
		if newRefresh != "" {
			aiConfig.OAuthRefreshToken = newRefresh
		}
		aiConfig.OAuthExpiresAt = newExpires
		fmt.Printf("   ✓ OAuth token refreshed (expires in %d hours)\n", (newExpires-CurrentTimeMs())/1000/3600)

		return aiConfig, nil
	}

	if aiConfig.APIKey == "" {
		return nil, fmt.Errorf(
			"no API key found for provider '%s'\nSet %s environment variable",
			aiConfig.Provider,
			GetEnvVarNameForProvider(aiConfig.Provider),
		)
	}

	return aiConfig, nil
}

// CurrentTimeMs returns current time in milliseconds
func CurrentTimeMs() int64 {
	return time.Now().UnixMilli()
}

// DetectCloudShipConfig detects CloudShip configuration from Station config
func DetectCloudShipConfig() *DeploymentCloudShipConfig {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	if !cfg.CloudShip.Enabled {
		return nil
	}

	if cfg.CloudShip.RegistrationKey == "" {
		return nil
	}

	return &DeploymentCloudShipConfig{
		Enabled:         cfg.CloudShip.Enabled,
		RegistrationKey: cfg.CloudShip.RegistrationKey,
		Name:            cfg.CloudShip.Name,
		Endpoint:        cfg.CloudShip.Endpoint,
		UseTLS:          cfg.CloudShip.UseTLS,
	}
}

// DetectTelemetryConfig detects telemetry configuration from Station config
func DetectTelemetryConfig() *DeploymentTelemetryConfig {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	return &DeploymentTelemetryConfig{
		Enabled:  cfg.Telemetry.Enabled,
		Provider: string(cfg.Telemetry.Provider),
		Endpoint: cfg.Telemetry.Endpoint,
	}
}

// LoadEnvironmentConfig loads the environment directory and parses its contents
func LoadEnvironmentConfig(envName string) (*EnvironmentConfig, error) {
	// Get Station config directory
	stationDir := config.GetStationConfigDir()
	envPath := filepath.Join(stationDir, "environments", envName)

	// Check if environment exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("environment '%s' not found at %s", envName, envPath)
	}

	envConfig := &EnvironmentConfig{
		Name:      envName,
		Path:      envPath,
		Variables: make(map[string]string),
		Agents:    []string{},
	}

	// Load variables.yml
	variablesPath := filepath.Join(envPath, "variables.yml")
	if _, err := os.Stat(variablesPath); err == nil {
		data, err := os.ReadFile(variablesPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables.yml: %w", err)
		}

		var variables map[string]interface{}
		if err := yaml.Unmarshal(data, &variables); err != nil {
			return nil, fmt.Errorf("failed to parse variables.yml: %w", err)
		}

		// Convert to string map
		for k, v := range variables {
			envConfig.Variables[k] = fmt.Sprintf("%v", v)
		}
	}

	// Load template.json (optional)
	templatePath := filepath.Join(envPath, "template.json")
	if _, err := os.Stat(templatePath); err == nil {
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read template.json: %w", err)
		}

		if err := yaml.Unmarshal(data, &envConfig.Template); err != nil {
			return nil, fmt.Errorf("failed to parse template.json: %w", err)
		}
	}

	// List agents
	agentsPath := filepath.Join(envPath, "agents")
	if entries, err := os.ReadDir(agentsPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".prompt") {
				envConfig.Agents = append(envConfig.Agents, entry.Name())
			}
		}
	}

	return envConfig, nil
}

// ResolveAppName determines the app name based on custom name, bundle ID, or environment name
func ResolveAppName(customName, bundleID, envName string) string {
	if customName != "" {
		return customName
	}
	cfg, err := config.Load()
	if err == nil && cfg.CloudShip.Name != "" {
		return cfg.CloudShip.Name
	}
	if envName != "" {
		return fmt.Sprintf("station-%s", envName)
	}
	if len(bundleID) >= 8 {
		return fmt.Sprintf("station-%s", bundleID[:8])
	}
	return fmt.Sprintf("station-%s", bundleID)
}

// GetAppName returns the app name for a given environment
func GetAppName(envName string) string {
	return ResolveAppName("", "", envName)
}
