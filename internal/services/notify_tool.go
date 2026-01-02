package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"station/internal/config"
	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// NotifyToolFactory creates the notify tool for agents
type NotifyToolFactory struct {
	config     *config.NotifyConfig
	httpClient *http.Client
	tracer     trace.Tracer
}

// NewNotifyToolFactory creates a new notify tool factory
func NewNotifyToolFactory(cfg *config.NotifyConfig) *NotifyToolFactory {
	if cfg == nil || cfg.WebhookURL == "" {
		return nil
	}

	timeout := cfg.TimeoutSeconds
	if timeout <= 0 {
		timeout = 10
	}

	return &NotifyToolFactory{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		tracer: otel.Tracer("station.notify_tool"),
	}
}

// IsEnabled returns true if notify tool is configured
func (f *NotifyToolFactory) IsEnabled() bool {
	return f != nil && f.config != nil && f.config.WebhookURL != ""
}

// ShouldAddTool returns true if notify tool should be added to an agent
func (f *NotifyToolFactory) ShouldAddTool(notifyEnabled bool) bool {
	return f.IsEnabled() && notifyEnabled
}

// NotifyRequest represents the parameters for the notify tool
type NotifyRequest struct {
	Message  string   `json:"message"`
	Title    string   `json:"title,omitempty"`
	Priority string   `json:"priority,omitempty"` // min, low, default, high, urgent (for ntfy)
	Tags     []string `json:"tags,omitempty"`     // emoji tags for ntfy
}

// NotifyResponse represents the result of a notification
type NotifyResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CreateNotifyTool creates the notify tool
func (f *NotifyToolFactory) CreateNotifyTool() ai.Tool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "The notification message content (required)",
			},
			"title": map[string]any{
				"type":        "string",
				"description": "Optional title/subject for the notification",
			},
			"priority": map[string]any{
				"type":        "string",
				"enum":        []string{"min", "low", "default", "high", "urgent"},
				"description": "Notification priority level (default: 'default')",
				"default":     "default",
			},
			"tags": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Tags for the notification (e.g., emoji names like 'white_check_mark', 'warning')",
			},
		},
		"required": []string{"message"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		ctx := toolCtx.Context

		// Start tracing span
		ctx, span := f.tracer.Start(ctx, "notify_tool.send",
			trace.WithAttributes(
				attribute.String("tool.name", "notify"),
			),
		)
		defer span.End()

		inputMap, ok := input.(map[string]any)
		if !ok {
			err := fmt.Errorf("notify: expected map[string]any input, got %T", input)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		req := f.parseRequest(inputMap)

		// Add request attributes to span
		span.SetAttributes(
			attribute.String("notify.title", req.Title),
			attribute.String("notify.priority", req.Priority),
			attribute.Int("notify.message_length", len(req.Message)),
		)

		result, err := f.send(ctx, req)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return &NotifyResponse{
				Success: false,
				Error:   err.Error(),
			}, nil // Return error in response, not as error (tool should not fail)
		}

		span.SetAttributes(
			attribute.Bool("notify.success", result.Success),
			attribute.String("notify.message_id", result.MessageID),
		)
		span.SetStatus(codes.Ok, "notification sent")

		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"notify",
		"Send a notification to the configured webhook (ntfy, Slack, etc.). Use this to alert users about task completion, errors, or important updates.",
		schema,
		toolFunc,
	)
}

// parseRequest extracts NotifyRequest from tool input
func (f *NotifyToolFactory) parseRequest(input map[string]any) NotifyRequest {
	req := NotifyRequest{
		Priority: "default",
	}

	if v, ok := input["message"].(string); ok {
		req.Message = v
	}
	if v, ok := input["title"].(string); ok {
		req.Title = v
	}
	if v, ok := input["priority"].(string); ok && v != "" {
		req.Priority = v
	}
	if v, ok := input["tags"].([]any); ok {
		for _, tag := range v {
			if s, ok := tag.(string); ok {
				req.Tags = append(req.Tags, s)
			}
		}
	}

	return req
}

// send sends the notification to the configured webhook
func (f *NotifyToolFactory) send(ctx context.Context, req NotifyRequest) (*NotifyResponse, error) {
	logging.Debug("[notify] Sending notification: title=%q, priority=%s, message_len=%d, format=%s",
		req.Title, req.Priority, len(req.Message), f.config.Format)

	format := f.config.Format
	if format == "" {
		format = "ntfy"
	}

	switch format {
	case "ntfy":
		return f.sendNtfy(ctx, req)
	case "json":
		return f.sendGenericWebhook(ctx, req)
	case "auto":
		if isNtfyURL(f.config.WebhookURL) {
			return f.sendNtfy(ctx, req)
		}
		return f.sendGenericWebhook(ctx, req)
	default:
		return f.sendNtfy(ctx, req)
	}
}

// sendNtfy sends notification using ntfy.sh format
func (f *NotifyToolFactory) sendNtfy(ctx context.Context, req NotifyRequest) (*NotifyResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.config.WebhookURL, bytes.NewReader([]byte(req.Message)))
	if err != nil {
		return nil, fmt.Errorf("failed to create ntfy request: %w", err)
	}

	// Set ntfy-specific headers
	if req.Title != "" {
		httpReq.Header.Set("Title", req.Title)
	}
	if req.Priority != "" && req.Priority != "default" {
		httpReq.Header.Set("Priority", req.Priority)
	}
	if len(req.Tags) > 0 {
		// Join tags with comma for ntfy
		tags := ""
		for i, tag := range req.Tags {
			if i > 0 {
				tags += ","
			}
			tags += tag
		}
		httpReq.Header.Set("Tags", tags)
	}

	// Add auth if configured
	if f.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+f.config.APIKey)
	}

	httpReq.Header.Set("User-Agent", "Station-NotifyTool/1.0")

	resp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ntfy request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ntfy returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse ntfy response for message ID
	var ntfyResp struct {
		ID string `json:"id"`
	}
	json.Unmarshal(body, &ntfyResp)

	logging.Info("[notify] Notification sent successfully via ntfy: id=%s", ntfyResp.ID)

	return &NotifyResponse{
		Success:   true,
		MessageID: ntfyResp.ID,
	}, nil
}

// sendGenericWebhook sends notification as JSON to a generic webhook
func (f *NotifyToolFactory) sendGenericWebhook(ctx context.Context, req NotifyRequest) (*NotifyResponse, error) {
	payload := map[string]any{
		"message":   req.Message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	if req.Title != "" {
		payload["title"] = req.Title
	}
	if req.Priority != "" {
		payload["priority"] = req.Priority
	}
	if len(req.Tags) > 0 {
		payload["tags"] = req.Tags
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.config.WebhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create webhook request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Station-NotifyTool/1.0")

	// Add auth if configured
	if f.config.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+f.config.APIKey)
	}

	resp, err := f.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	logging.Info("[notify] Notification sent successfully via webhook")

	return &NotifyResponse{
		Success: true,
	}, nil
}

// isNtfyURL checks if the URL is an ntfy.sh endpoint
func isNtfyURL(url string) bool {
	return len(url) > 0 && (
	// Official ntfy.sh
	stringContains(url, "ntfy.sh") ||
		// Self-hosted ntfy
		stringContains(url, "ntfy.") ||
		// Common self-hosted patterns
		stringContains(url, "/ntfy/"))
}

// stringContains is a simple string contains check (named to avoid collision with bundle_service.go)
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// GetNotifyTools returns the notify tool if enabled
func (f *NotifyToolFactory) GetNotifyTools() []ai.Tool {
	if !f.IsEnabled() {
		return nil
	}
	return []ai.Tool{f.CreateNotifyTool()}
}
