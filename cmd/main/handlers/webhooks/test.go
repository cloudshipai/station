package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"station/pkg/models"
)

// RunWebhookTest sends a test webhook payload to the specified endpoint
func (h *WebhookHandler) RunWebhookTest(cmd *cobra.Command, args []string) error {
	endpointURL := args[0]
	
	fmt.Printf("🧪 Testing webhook endpoint: %s\n", endpointURL)
	
	// Create a test payload
	testAgent := &models.Agent{
		ID:          1,
		Name:        "Test Agent",
		Description: "Test agent for webhook functionality",
		Prompt:      "You are a test agent used for webhook testing",
		MaxSteps:    5,
	}
	
	testRun := &models.AgentRun{
		ID:            999,
		AgentID:       1,
		UserID:        1,
		Task:          "Test webhook functionality",
		FinalResponse: "Webhook test completed successfully - this is a test payload from Station CLI",
		StepsTaken:    3,
		Status:        "completed",
		StartedAt:     time.Now().Add(-30 * time.Second),
		CompletedAt:   &[]time.Time{time.Now()}[0],
	}
	
	// Create webhook payload
	payload := map[string]interface{}{
		"event":     "agent_run_completed",
		"timestamp": time.Now().Format(time.RFC3339),
		"agent":     testAgent,
		"run":       testRun,
		"settings": map[string]string{
			"source": "cli_test",
			"version": "1.0",
		},
	}
	
	// Marshal payload to JSON
	payloadBytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}
	
	fmt.Printf("📋 Sending payload:\n%s\n\n", string(payloadBytes))
	
	// Create HTTP request
	req, err := http.NewRequest("POST", endpointURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Station-CLI-Test/1.0")
	req.Header.Set("X-Station-Event", "agent_run_completed")
	req.Header.Set("X-Station-Timestamp", time.Now().Format(time.RFC3339))
	req.Header.Set("X-Station-Source", "cli-test")
	
	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	fmt.Printf("🚀 Sending POST request to %s...\n", endpointURL)
	
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("❌ Failed to send request: %v", err)
	}
	defer resp.Body.Close()
	
	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		responseBody = []byte("Failed to read response body")
	}
	
	// Display results
	fmt.Printf("📊 Response Status: %s\n", resp.Status)
	fmt.Printf("📊 Response Headers:\n")
	for key, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
	
	if len(responseBody) > 0 {
		fmt.Printf("📊 Response Body:\n%s\n", string(responseBody))
	}
	
	// Determine success
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("✅ Webhook test successful! Status: %d\n", resp.StatusCode)
	} else {
		fmt.Printf("❌ Webhook test failed! Status: %d\n", resp.StatusCode)
	}
	
	return nil
}