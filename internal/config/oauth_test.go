package config

import (
	"os"
	"testing"
)

func TestGetClaudeCodeCredentials(t *testing.T) {
	creds := getClaudeCodeCredentials()

	if creds == nil {
		t.Skip("No valid Claude Code credentials found at ~/.claude/.credentials.json (token may be expired)")
	}

	if creds.AuthType != "oauth" {
		t.Errorf("Expected auth_type 'oauth', got '%s'", creds.AuthType)
	}

	if creds.AccessToken == "" {
		t.Error("Expected non-empty access token")
	}

	t.Logf("Found OAuth token: %s...%s", creds.AccessToken[:20], creds.AccessToken[len(creds.AccessToken)-10:])
}

func TestGetAIAuthTypeAnthropic(t *testing.T) {
	os.Setenv("STN_AI_PROVIDER", "anthropic")
	defer os.Unsetenv("STN_AI_PROVIDER")

	authType := getAIAuthType()
	t.Logf("AI Auth Type: %s", authType)

	if authType != "oauth" && authType != "api_key" {
		t.Errorf("Expected 'oauth' or 'api_key', got '%s'", authType)
	}
}

func TestGetAIOAuthTokenAnthropic(t *testing.T) {
	os.Setenv("STN_AI_PROVIDER", "anthropic")
	defer os.Unsetenv("STN_AI_PROVIDER")

	token := getAIOAuthToken()

	if token == "" {
		t.Skip("No OAuth token available")
	}

	t.Logf("OAuth token found: %s...%s", token[:20], token[len(token)-10:])
}
