package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"station/internal/config"
)

func getTestOAuthToken(t *testing.T) string {
	os.Setenv("STN_AI_PROVIDER", "anthropic")
	defer os.Unsetenv("STN_AI_PROVIDER")

	// Initialize viper to read from config file BEFORE calling Load()
	if err := config.InitViper(""); err != nil {
		t.Skipf("Cannot initialize viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Skipf("Cannot load config: %v", err)
	}

	if cfg.AIAuthType != "oauth" || cfg.AIOAuthToken == "" {
		t.Skip("No OAuth token available (need Claude Code credentials or config)")
	}

	return cfg.AIOAuthToken
}

func TestAnthropicOAuthMiddleware(t *testing.T) {
	token := getTestOAuthToken(t)

	reqBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 100,
		"system": []map[string]interface{}{
			{"type": "text", "text": "You are Claude Code, Anthropic's official CLI for Claude."},
		},
		"messages": []map[string]string{
			{"role": "user", "content": "Say 'OAuth works!' and nothing else."},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(result.Content) == 0 {
		t.Fatal("No content in response")
	}

	t.Logf("Response: %s", result.Content[0].Text)
	t.Log("OAuth middleware works!")
}

func TestMiddlewareFunction(t *testing.T) {
	token := getTestOAuthToken(t)

	middleware := newAnthropicOAuthMiddleware(token)

	reqBody := map[string]interface{}{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with just: 'Middleware test passed'"},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", "dummy-key-should-be-removed")

	resp, err := middleware(req, func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-api-key") != "" {
			t.Error("x-api-key header should have been removed")
		}
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("Authorization header not set correctly")
		}
		expectedBeta := "oauth-2025-04-20,claude-code-20250219,interleaved-thinking-2025-05-14"
		if r.Header.Get("anthropic-beta") != expectedBeta {
			t.Errorf("anthropic-beta header not set correctly, got: %s", r.Header.Get("anthropic-beta"))
		}

		return http.DefaultClient.Do(r)
	})

	if err != nil {
		t.Fatalf("Middleware failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	t.Logf("Status: %d", resp.StatusCode)
	t.Log("Middleware function works!")
}
