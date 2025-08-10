package services

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"station/pkg/models"
)

func TestWebhookService_WebhookHandlesEvent(t *testing.T) {
	service := &WebhookService{}

	testCases := []struct {
		name      string
		events    string
		eventType string
		expected  bool
	}{
		{
			name:      "empty events",
			events:    "",
			eventType: "agent_run_completed",
			expected:  false,
		},
		{
			name:      "single event match",
			events:    "agent_run_completed",
			eventType: "agent_run_completed",
			expected:  true,
		},
		{
			name:      "single event no match",
			events:    "agent_run_completed",
			eventType: "agent_created",
			expected:  false,
		},
		{
			name:      "JSON array match",
			events:    `["agent_run_completed", "agent_created"]`,
			eventType: "agent_run_completed",
			expected:  true,
		},
		{
			name:      "JSON array no match",
			events:    `["agent_created", "agent_deleted"]`,
			eventType: "agent_run_completed",
			expected:  false,
		},
		{
			name:      "wildcard match",
			events:    `["*"]`,
			eventType: "agent_run_completed",
			expected:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			webhook := &models.Webhook{
				Events: tc.events,
			}

			result := service.webhookHandlesEvent(webhook, tc.eventType)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v for events %s and eventType %s", tc.expected, result, tc.events, tc.eventType)
			}
		})
	}
}

func TestWebhookService_CalculateRetryDelay(t *testing.T) {
	service := &WebhookService{}

	testCases := []struct {
		attempt  int64
		expected time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{10, 1024 * time.Second}, // 2^10 = 1024 seconds
		{13, time.Hour}, // Should be capped at 1 hour
	}

	for _, tc := range testCases {
		result := service.calculateRetryDelay(tc.attempt)
		if result != tc.expected {
			t.Errorf("For attempt %d, expected %v, got %v", tc.attempt, tc.expected, result)
		}
	}
}

func TestWebhookService_GenerateSignature(t *testing.T) {
	service := &WebhookService{}

	payload := []byte(`{"test": "data"}`)
	secret := "test-secret"

	signature := service.generateSignature(payload, secret)

	if !strings.HasPrefix(signature, "sha256=") {
		t.Errorf("Expected signature to start with 'sha256=', got %s", signature)
	}

	// Verify signature is consistent
	signature2 := service.generateSignature(payload, secret)
	if signature != signature2 {
		t.Errorf("Expected consistent signatures, got %s and %s", signature, signature2)
	}

	// Verify different payload produces different signature
	signature3 := service.generateSignature([]byte(`{"test": "different"}`), secret)
	if signature == signature3 {
		t.Error("Expected different signatures for different payloads")
	}
}

func TestWebhookPayload_JSON(t *testing.T) {
	payload := &WebhookPayload{
		Event:     "agent_run_completed",
		Timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		Agent: &models.Agent{
			ID:          1,
			Name:        "test-agent",
			Description: "Test agent",
		},
		Run: &models.AgentRun{
			ID:            1,
			AgentID:       1,
			Task:          "test task",
			FinalResponse: "test response",
			Status:        "completed",
		},
	}

	// Test JSON marshaling
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled WebhookPayload
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	// Verify key fields
	if unmarshaled.Event != payload.Event {
		t.Errorf("Expected event %s, got %s", payload.Event, unmarshaled.Event)
	}

	if unmarshaled.Agent.ID != payload.Agent.ID {
		t.Errorf("Expected agent ID %d, got %d", payload.Agent.ID, unmarshaled.Agent.ID)
	}

	if unmarshaled.Run.ID != payload.Run.ID {
		t.Errorf("Expected run ID %d, got %d", payload.Run.ID, unmarshaled.Run.ID)
	}
}

// Integration tests would require database setup, so we keep these as unit tests
// for the core functionality that doesn't depend on repositories