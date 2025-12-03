package lighthouse

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"station/internal/logging"
)

// MemoryAPIClient makes direct HTTP calls to CloudShip's memory API.
// This is used in CLI mode where there's no persistent management channel.
type MemoryAPIClient struct {
	apiURL          string
	registrationKey string
	httpClient      *http.Client
	timeout         time.Duration
}

// MemoryContextRequest is the request payload for the memory context API
type MemoryContextRequest struct {
	TopicKey        string `json:"topic_key"`
	RegistrationKey string `json:"registration_key"`
	MaxTokens       int    `json:"max_tokens"`
}

// MemoryContextResponse is the response from the memory context API
type MemoryContextResponse struct {
	Success      bool   `json:"success"`
	Context      string `json:"context"`       // The actual memory content
	TokenCount   int    `json:"token_count"`
	TopicKey     string `json:"topic_key"`
	LastUpdated  string `json:"last_updated,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NewMemoryAPIClient creates a new memory API client for direct HTTP calls.
func NewMemoryAPIClient(apiURL, registrationKey string, timeout time.Duration) *MemoryAPIClient {
	if timeout == 0 {
		timeout = 2 * time.Second // Default 2 second timeout per PRD
	}
	return &MemoryAPIClient{
		apiURL:          apiURL,
		registrationKey: registrationKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// IsConnected returns true if the client is configured (has API URL).
func (mac *MemoryAPIClient) IsConnected() bool {
	return mac.apiURL != "" && mac.registrationKey != ""
}

// GetMemoryContext fetches memory context directly from the CloudShip Django API.
// This is used in CLI mode where there's no management channel.
//
// Returns empty string on error (never fails agent run due to memory issues)
func (mac *MemoryAPIClient) GetMemoryContext(ctx context.Context, topicKey string, maxTokens int) (string, error) {
	if !mac.IsConnected() {
		return "", fmt.Errorf("memory API client not configured")
	}

	if topicKey == "" {
		return "", fmt.Errorf("topic key is required")
	}

	// Build request
	reqBody := MemoryContextRequest{
		TopicKey:        topicKey,
		RegistrationKey: mac.registrationKey,
		MaxTokens:       maxTokens,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make HTTP request to Django internal memory API
	url := fmt.Sprintf("%s/api/v1/memory/internal/context/", mac.apiURL)
	logging.Debug("[MEMORY API] Making request to %s for topic '%s'", url, topicKey)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := mac.httpClient.Do(req)
	if err != nil {
		logging.Debug("[MEMORY API] Request failed: %v", err)
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logging.Debug("[MEMORY API] Request returned status %d: %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var memResp MemoryContextResponse
	if err := json.Unmarshal(body, &memResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if memResp.ErrorMessage != "" {
		return "", fmt.Errorf("API error: %s", memResp.ErrorMessage)
	}

	if !memResp.Success {
		return "", fmt.Errorf("API returned success=false")
	}

	logging.Info("[MEMORY API] Successfully fetched %d tokens of memory for topic '%s'", memResp.TokenCount, topicKey)
	return memResp.Context, nil
}
