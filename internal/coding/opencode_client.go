package coding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OpenCodeClient struct {
	baseURL     string
	httpClient  *http.Client
	maxAttempts int
	retryDelay  time.Duration
	maxDelay    time.Duration
	multiplier  float64
}

type OpenCodeClientOption func(*OpenCodeClient)

func WithMaxAttempts(n int) OpenCodeClientOption {
	return func(c *OpenCodeClient) { c.maxAttempts = n }
}

func WithRetryDelay(initial, max time.Duration, multiplier float64) OpenCodeClientOption {
	return func(c *OpenCodeClient) {
		c.retryDelay = initial
		c.maxDelay = max
		c.multiplier = multiplier
	}
}

func WithHTTPClient(client *http.Client) OpenCodeClientOption {
	return func(c *OpenCodeClient) { c.httpClient = client }
}

func NewOpenCodeClient(baseURL string, opts ...OpenCodeClientOption) *OpenCodeClient {
	c := &OpenCodeClient{
		baseURL:     baseURL,
		httpClient:  &http.Client{Timeout: 10 * time.Minute},
		maxAttempts: 3,
		retryDelay:  time.Second,
		maxDelay:    30 * time.Second,
		multiplier:  2.0,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *OpenCodeClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/global/health", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unhealthy: status %d, body: %s", resp.StatusCode, body)
	}

	return nil
}

type createSessionRequest struct {
	Title string `json:"title,omitempty"`
}

type createSessionResponse struct {
	ID string `json:"id"`
}

func (c *OpenCodeClient) CreateSession(ctx context.Context, directory, title string) (string, error) {
	reqBody := createSessionRequest{Title: title}
	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/session?directory=%s", c.baseURL, directory)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session failed: status %d, body: %s", resp.StatusCode, respBody)
	}

	var result createSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return result.ID, nil
}

type sendMessageRequest struct {
	Parts []messagePart `json:"parts"`
}

type messagePart struct {
	Type   string                 `json:"type"`
	Text   string                 `json:"text,omitempty"`
	Tool   string                 `json:"tool,omitempty"`
	Input  map[string]interface{} `json:"input,omitempty"`
	Output string                 `json:"output,omitempty"`
	Reason string                 `json:"reason,omitempty"`
}

type sendMessageResponse struct {
	Info  messageInfo   `json:"info"`
	Parts []messagePart `json:"parts"`
}

type messageInfo struct {
	ID         string `json:"id"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
	Finish     string `json:"finish"`
	Time       struct {
		Created   int64 `json:"created"`
		Completed int64 `json:"completed"`
	} `json:"time"`
	Tokens struct {
		Input      int `json:"input"`
		Output     int `json:"output"`
		Reasoning  int `json:"reasoning"`
		CacheRead  int `json:"cacheRead"`
		CacheWrite int `json:"cacheWrite"`
	} `json:"tokens"`
	Cost float64 `json:"cost"`
}

type MessageResponse struct {
	ID           string
	Model        string
	Provider     string
	FinishReason string
	Text         string
	Tokens       TokenUsage
	Cost         float64
	ToolCalls    []ToolCall
	Reasoning    []string
}

func (c *OpenCodeClient) SendMessage(ctx context.Context, sessionID, directory, text string) (*MessageResponse, error) {
	reqBody := sendMessageRequest{
		Parts: []messagePart{{Type: "text", Text: text}},
	}
	body, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("%s/session/%s/message?directory=%s", c.baseURL, sessionID, directory)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("send message failed: status %d, body: %s", resp.StatusCode, respBody)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if len(respBody) == 0 {
		return nil, fmt.Errorf("empty response body (status: %d, content-length: %s)",
			resp.StatusCode, resp.Header.Get("Content-Length"))
	}

	return c.parseMessageResponse(respBody)
}

func (c *OpenCodeClient) parseMessageResponse(body []byte) (*MessageResponse, error) {
	var raw sendMessageResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result := &MessageResponse{
		ID:           raw.Info.ID,
		Model:        raw.Info.ModelID,
		Provider:     raw.Info.ProviderID,
		FinishReason: raw.Info.Finish,
		Cost:         raw.Info.Cost,
		Tokens: TokenUsage{
			Input:      raw.Info.Tokens.Input,
			Output:     raw.Info.Tokens.Output,
			Reasoning:  raw.Info.Tokens.Reasoning,
			CacheRead:  raw.Info.Tokens.CacheRead,
			CacheWrite: raw.Info.Tokens.CacheWrite,
		},
	}

	var pendingToolCall *ToolCall
	var reasoning []string

	for _, part := range raw.Parts {
		switch part.Type {
		case "text":
			if result.Text != "" {
				result.Text += "\n"
			}
			result.Text += part.Text

		case "reasoning":
			if part.Text != "" {
				reasoning = append(reasoning, part.Text)
			}

		case "tool-invocation":
			if pendingToolCall != nil {
				result.ToolCalls = append(result.ToolCalls, *pendingToolCall)
			}
			pendingToolCall = &ToolCall{
				Tool:  part.Tool,
				Input: part.Input,
			}

		case "tool-result":
			if pendingToolCall != nil {
				pendingToolCall.Output = part.Output
				result.ToolCalls = append(result.ToolCalls, *pendingToolCall)
				pendingToolCall = nil
			}
		}
	}

	if pendingToolCall != nil {
		result.ToolCalls = append(result.ToolCalls, *pendingToolCall)
	}

	result.Reasoning = reasoning
	return result, nil
}

func (c *OpenCodeClient) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	delay := c.retryDelay

	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}

		resp, err := c.httpClient.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else {
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("server error: status %d, body: %s", resp.StatusCode, respBody)
		}

		if attempt < c.maxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			delay = time.Duration(float64(delay) * c.multiplier)
			if delay > c.maxDelay {
				delay = c.maxDelay
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
