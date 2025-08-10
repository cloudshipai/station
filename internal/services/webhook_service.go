package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"station/internal/db/repositories"
	"station/pkg/models"
)

type WebhookService struct {
	repos      *repositories.Repositories
	httpClient *http.Client
}

type WebhookPayload struct {
	Event     string             `json:"event"`
	Timestamp time.Time          `json:"timestamp"`
	Agent     *models.Agent      `json:"agent"`
	Run       *models.AgentRun   `json:"run"`
	Settings  map[string]string  `json:"settings,omitempty"`
}

type WebhookDeliveryResult struct {
	WebhookID      int64
	Success        bool
	HTTPStatusCode int
	ResponseBody   string
	ResponseHeaders map[string]string
	Error          error
}

func NewWebhookService(repos *repositories.Repositories) *WebhookService {
	return &WebhookService{
		repos: repos,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NotifyAgentRunCompleted sends webhook notifications for completed agent runs
func (w *WebhookService) NotifyAgentRunCompleted(ctx context.Context, agentRun *models.AgentRun, agent *models.Agent) error {
	// Check if notifications are enabled
	enabled, err := w.areNotificationsEnabled(ctx)
	if err != nil {
		log.Printf("Failed to check notification settings: %v", err)
		return err
	}
	
	if !enabled {
		log.Printf("Notifications disabled, skipping webhook for agent run %d", agentRun.ID)
		return nil
	}

	// Get enabled webhooks
	webhooks, err := w.repos.Webhooks.ListEnabled()
	if err != nil {
		log.Printf("Failed to get enabled webhooks: %v", err)
		return err
	}

	if len(webhooks) == 0 {
		log.Printf("No webhooks registered, skipping notification for agent run %d", agentRun.ID)
		return nil
	}

	// Create payload
	payload := &WebhookPayload{
		Event:     "agent_run_completed",
		Timestamp: time.Now(),
		Agent:     agent,
		Run:       agentRun,
	}

	// Send to all enabled webhooks
	for _, webhook := range webhooks {
		// Check if this webhook handles this event type
		if !w.webhookHandlesEvent(webhook, "agent_run_completed") {
			continue
		}

		// Create delivery record
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Failed to marshal webhook payload: %v", err)
			continue
		}

		delivery, err := w.repos.WebhookDeliveries.Create(&models.WebhookDelivery{
			WebhookID:    webhook.ID,
			EventType:    "agent_run_completed",
			Payload:      string(payloadBytes),
			Status:       "pending",
			AttemptCount: 0,
		})
		if err != nil {
			log.Printf("Failed to create webhook delivery record: %v", err)
			continue
		}

		// Send webhook asynchronously
		go w.deliverWebhook(ctx, webhook, delivery, payload)
	}

	return nil
}

// deliverWebhook sends a webhook and updates the delivery record
func (w *WebhookService) deliverWebhook(ctx context.Context, webhook *models.Webhook, delivery *models.WebhookDelivery, payload *WebhookPayload) {
	// Prepare request
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		w.markDeliveryFailed(delivery.ID, fmt.Sprintf("Failed to marshal payload: %v", err))
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		w.markDeliveryFailed(delivery.ID, fmt.Sprintf("Failed to create request: %v", err))
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Station-Webhook/1.0")
	req.Header.Set("X-Station-Event", payload.Event)
	req.Header.Set("X-Station-Timestamp", payload.Timestamp.Format(time.RFC3339))
	req.Header.Set("X-Station-Delivery", strconv.FormatInt(delivery.ID, 10))

	// Add custom headers from webhook config
	if webhook.Headers != "" {
		var customHeaders map[string]string
		if err := json.Unmarshal([]byte(webhook.Headers), &customHeaders); err == nil {
			for key, value := range customHeaders {
				req.Header.Set(key, value)
			}
		}
	}

	// Add signature if secret is provided
	if webhook.Secret != "" {
		signature := w.generateSignature(payloadBytes, webhook.Secret)
		req.Header.Set("X-Station-Signature", signature)
	}

	// Send request with timeout from webhook config
	client := &http.Client{
		Timeout: time.Duration(webhook.TimeoutSeconds) * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		w.handleDeliveryError(delivery, webhook, err)
		return
	}
	defer resp.Body.Close()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		responseBody = []byte("Failed to read response body")
	}

	// Collect response headers
	responseHeaders := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			responseHeaders[key] = values[0]
		}
	}

	// Update delivery status
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.markDeliverySuccess(delivery.ID, resp.StatusCode, string(responseBody), responseHeaders)
		log.Printf("Webhook delivered successfully to %s (delivery %d)", webhook.URL, delivery.ID)
	} else {
		w.handleDeliveryError(delivery, webhook, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody)))
	}
}

// handleDeliveryError handles webhook delivery errors and schedules retries
func (w *WebhookService) handleDeliveryError(delivery *models.WebhookDelivery, webhook *models.Webhook, err error) {
	newAttemptCount := delivery.AttemptCount + 1
	
	if newAttemptCount >= webhook.RetryAttempts {
		// Max retries reached, mark as failed
		w.markDeliveryFailed(delivery.ID, fmt.Sprintf("Max retries reached: %v", err))
		log.Printf("Webhook delivery %d failed after %d attempts: %v", delivery.ID, newAttemptCount, err)
	} else {
		// Schedule retry with exponential backoff
		nextRetry := time.Now().Add(w.calculateRetryDelay(int64(newAttemptCount)))
		err := w.repos.WebhookDeliveries.UpdateForRetry(delivery.ID, nextRetry, 0, "", "", err.Error())
		if err != nil {
			log.Printf("Failed to schedule webhook retry: %v", err)
		} else {
			log.Printf("Scheduled webhook retry %d/%d for delivery %d at %v", newAttemptCount, webhook.RetryAttempts, delivery.ID, nextRetry)
		}
	}
}

// calculateRetryDelay calculates exponential backoff delay
func (w *WebhookService) calculateRetryDelay(attempt int64) time.Duration {
	// Exponential backoff: 2^attempt seconds, capped at 1 hour
	if attempt > 12 { // 2^12 = 4096 seconds > 1 hour
		return time.Hour
	}
	delay := time.Duration(1<<uint(attempt)) * time.Second
	if delay > time.Hour {
		delay = time.Hour
	}
	return delay
}

// ProcessRetries processes failed webhook deliveries that are ready for retry
func (w *WebhookService) ProcessRetries(ctx context.Context) error {
	deliveries, err := w.repos.WebhookDeliveries.ListFailedForRetry()
	if err != nil {
		return fmt.Errorf("failed to get failed deliveries: %w", err)
	}

	for _, delivery := range deliveries {
		webhook, err := w.repos.Webhooks.GetByID(delivery.WebhookID)
		if err != nil {
			log.Printf("Failed to get webhook %d for retry: %v", delivery.WebhookID, err)
			continue
		}

		if !webhook.Enabled {
			log.Printf("Skipping retry for disabled webhook %d", webhook.ID)
			continue
		}

		// Parse the original payload
		var payload WebhookPayload
		if err := json.Unmarshal([]byte(delivery.Payload), &payload); err != nil {
			log.Printf("Failed to parse payload for delivery %d: %v", delivery.ID, err)
			continue
		}

		// Retry delivery
		go w.deliverWebhook(ctx, webhook, delivery, &payload)
	}

	return nil
}

// webhookHandlesEvent checks if a webhook is configured to handle a specific event
func (w *WebhookService) webhookHandlesEvent(webhook *models.Webhook, eventType string) bool {
	if webhook.Events == "" {
		return false
	}

	var events []string
	if err := json.Unmarshal([]byte(webhook.Events), &events); err != nil {
		// If it's not JSON, treat as a single event string
		return webhook.Events == eventType
	}

	for _, event := range events {
		if event == eventType || event == "*" {
			return true
		}
	}

	return false
}

// generateSignature generates HMAC-SHA256 signature for webhook payload
func (w *WebhookService) generateSignature(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}

// areNotificationsEnabled checks if notifications are globally enabled
func (w *WebhookService) areNotificationsEnabled(ctx context.Context) (bool, error) {
	setting, err := w.repos.Settings.GetByKey("notifications_enabled")
	if err != nil {
		// Default to true if setting doesn't exist
		return true, nil
	}

	return setting.Value == "true", nil
}

// markDeliverySuccess marks a delivery as successful
func (w *WebhookService) markDeliverySuccess(deliveryID int64, statusCode int, responseBody string, responseHeaders map[string]string) {
	headersJSON := ""
	if len(responseHeaders) > 0 {
		if bytes, err := json.Marshal(responseHeaders); err == nil {
			headersJSON = string(bytes)
		}
	}

	err := w.repos.WebhookDeliveries.MarkSuccess(deliveryID, statusCode, responseBody, headersJSON)
	if err != nil {
		log.Printf("Failed to mark delivery %d as successful: %v", deliveryID, err)
	}
}

// markDeliveryFailed marks a delivery as permanently failed
func (w *WebhookService) markDeliveryFailed(deliveryID int64, errorMessage string) {
	err := w.repos.WebhookDeliveries.MarkFailed(deliveryID, errorMessage)
	if err != nil {
		log.Printf("Failed to mark delivery %d as failed: %v", deliveryID, err)
	}
}

// CleanupOldDeliveries removes old successful and failed deliveries
func (w *WebhookService) CleanupOldDeliveries(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)
	return w.repos.WebhookDeliveries.DeleteOldDeliveries(cutoff)
}