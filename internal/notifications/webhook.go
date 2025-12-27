package notifications

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"station/internal/config"
)

type ApprovalWebhookPayload struct {
	Event          string   `json:"event"`
	ApprovalID     string   `json:"approval_id"`
	WorkflowID     string   `json:"workflow_id"`
	WorkflowName   string   `json:"workflow_name"`
	RunID          string   `json:"run_id"`
	StepName       string   `json:"step_name"`
	Message        string   `json:"message"`
	Approvers      []string `json:"approvers"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	CreatedAt      string   `json:"created_at"`
	ApproveURL     string   `json:"approve_url"`
	RejectURL      string   `json:"reject_url"`
	ViewURL        string   `json:"view_url"`
}

type WebhookNotifier struct {
	webhookURL   string
	timeout      time.Duration
	baseURL      string
	httpClient   *http.Client
	auditService *AuditService
}

func NewWebhookNotifier(cfg *config.Config, db *sql.DB) *WebhookNotifier {
	if cfg == nil || cfg.Notifications.ApprovalWebhookURL == "" {
		return nil
	}

	timeout := cfg.Notifications.ApprovalWebhookTimeout
	if timeout <= 0 {
		timeout = 10
	}

	mcpPort := cfg.MCPPort
	if mcpPort == 0 {
		mcpPort = 8586
	}
	dynamicAgentPort := mcpPort + 1

	var auditSvc *AuditService
	if db != nil {
		auditSvc = NewAuditService(db)
	}

	return &WebhookNotifier{
		webhookURL:   cfg.Notifications.ApprovalWebhookURL,
		timeout:      time.Duration(timeout) * time.Second,
		baseURL:      fmt.Sprintf("http://localhost:%d", dynamicAgentPort),
		auditService: auditSvc,
		httpClient: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (w *WebhookNotifier) SetBaseURL(baseURL string) {
	w.baseURL = baseURL
}

func (w *WebhookNotifier) IsEnabled() bool {
	return w != nil && w.webhookURL != ""
}

func (w *WebhookNotifier) NotifyApprovalRequired(ctx context.Context, payload ApprovalWebhookPayload) error {
	if !w.IsEnabled() {
		return nil
	}

	payload.Event = "approval.requested"
	payload.ApproveURL = fmt.Sprintf("%s/workflow-approvals/%s/approve", w.baseURL, payload.ApprovalID)
	payload.RejectURL = fmt.Sprintf("%s/workflow-approvals/%s/reject", w.baseURL, payload.ApprovalID)

	return w.sendWithRetry(ctx, payload, 3)
}

func (w *WebhookNotifier) sendWithRetry(ctx context.Context, payload ApprovalWebhookPayload, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := w.send(ctx, payload, attempt)
		if err == nil {
			log.Printf("[WebhookNotifier] Successfully sent approval webhook (approval_id=%s, attempt=%d)", payload.ApprovalID, attempt)
			return nil
		}

		lastErr = err
		log.Printf("[WebhookNotifier] Webhook attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}

	log.Printf("[WebhookNotifier] All %d webhook attempts failed for approval_id=%s", maxRetries, payload.ApprovalID)
	return lastErr
}

func (w *WebhookNotifier) send(ctx context.Context, payload ApprovalWebhookPayload, attempt int) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.webhookURL, bytes.NewReader(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Station-Webhook/1.0")

	startTime := time.Now()
	resp, err := w.httpClient.Do(req)
	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		if w.auditService != nil {
			_ = w.auditService.LogWebhookFailure(ctx, payload.ApprovalID, w.webhookURL, err.Error(), 0, durationMs, attempt)
		}
		return fmt.Errorf("request failed after %dms: %w", durationMs, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	respBodyStr := string(respBody)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if w.auditService != nil {
			_ = w.auditService.LogWebhookFailure(ctx, payload.ApprovalID, w.webhookURL, fmt.Sprintf("HTTP %d", resp.StatusCode), resp.StatusCode, durationMs, attempt)
		}
		return fmt.Errorf("webhook returned status %d after %dms", resp.StatusCode, durationMs)
	}

	if w.auditService != nil {
		_ = w.auditService.LogWebhookSuccess(ctx, payload.ApprovalID, w.webhookURL, resp.StatusCode, respBodyStr, durationMs, attempt)
	}

	return nil
}
