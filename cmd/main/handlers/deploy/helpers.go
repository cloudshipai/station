package deploy

import (
	"os"
	"strings"

	"station/internal/auth"
)

// GetEnvVarNameForProvider returns the environment variable name(s) for a given AI provider
func GetEnvVarNameForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "OPENAI_API_KEY or STN_AI_API_KEY"
	case "gemini", "google":
		return "GOOGLE_API_KEY, GEMINI_API_KEY, or STN_AI_API_KEY"
	case "anthropic", "claude":
		return "ANTHROPIC_API_KEY or STN_AI_API_KEY"
	default:
		return "STN_AI_API_KEY"
	}
}

// MaskAPIKey masks an API key for display, showing only the first 8 characters
func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "***"
}

// ParseEnvFile parses an environment file and returns a map of key-value pairs
func ParseEnvFile(envFile string) (map[string]string, error) {
	if envFile == "" {
		return nil, nil
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		return nil, err
	}

	secrets := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.Contains(value, "***MASKED***") || strings.Contains(value, "***GENERATE_NEW***") {
			continue
		}

		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		secrets[key] = value
	}

	return secrets, nil
}

// GenerateEncryptionKey generates a secure encryption key for Station
func GenerateEncryptionKey() (string, error) {
	// Use Station's existing GenerateAPIKey function from auth package
	// This generates a secure random key
	key, err := auth.GenerateAPIKey()
	if err != nil {
		return "", err
	}
	// Remove the "sk-" prefix as this is an encryption key, not an API key
	return key[3:], nil
}

// IsSensitiveKey returns true if the key name suggests it contains sensitive data
func IsSensitiveKey(key string) bool {
	lowered := strings.ToLower(key)
	return strings.Contains(lowered, "key") ||
		strings.Contains(lowered, "token") ||
		strings.Contains(lowered, "secret") ||
		strings.Contains(lowered, "password")
}
