package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// RunWebhookCreate creates a new webhook using flags
func (h *WebhookHandler) RunWebhookCreate(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	name, _ := cmd.Flags().GetString("name")
	url, _ := cmd.Flags().GetString("url")
	secret, _ := cmd.Flags().GetString("secret")
	events, _ := cmd.Flags().GetStringSlice("events")
	headers, _ := cmd.Flags().GetStringToString("headers")
	timeout, _ := cmd.Flags().GetInt("timeout")
	retries, _ := cmd.Flags().GetInt("retries")

	webhook := &models.Webhook{
		Name:           name,
		URL:            url,
		Secret:         secret,
		Enabled:        true,
		TimeoutSeconds: timeout,
		RetryAttempts:  retries,
		CreatedBy:      1,
	}

	// Convert events to JSON
	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}
	webhook.Events = string(eventsJSON)

	// Convert headers to JSON if provided
	if len(headers) > 0 {
		headersJSON, err := json.Marshal(headers)
		if err != nil {
			return fmt.Errorf("failed to marshal headers: %w", err)
		}
		webhook.Headers = string(headersJSON)
	}

	if endpoint == "" {
		// Local mode
		return h.createWebhookLocal(webhook)
	}

	// Remote mode
	return h.createWebhookRemote(endpoint, webhook)
}

func (h *WebhookHandler) createWebhookLocal(webhook *models.Webhook) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	created, err := repos.Webhooks.Create(webhook)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook created successfully!"))
	fmt.Printf("ID: %d\n", created.ID)
	fmt.Printf("Name: %s\n", created.Name)
	fmt.Printf("URL: %s\n", created.URL)

	return nil
}

func (h *WebhookHandler) createWebhookRemote(endpoint string, webhook *models.Webhook) error {
	// Create request payload
	payload := map[string]interface{}{
		"name":            webhook.Name,
		"url":             webhook.URL,
		"secret":          webhook.Secret,
		"timeout_seconds": webhook.TimeoutSeconds,
		"retry_attempts":  webhook.RetryAttempts,
	}

	// Parse events from JSON string
	var events []string
	if err := json.Unmarshal([]byte(webhook.Events), &events); err == nil {
		payload["events"] = events
	}

	// Parse headers from JSON string if present
	if webhook.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(webhook.Headers), &headers); err == nil {
			payload["headers"] = headers
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/webhooks", endpoint)
	req, err := makeAuthenticatedRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("API error: %s", string(respBody))
	}

	var created models.Webhook
	if err := json.Unmarshal(respBody, &created); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook created successfully!"))
	fmt.Printf("ID: %d\n", created.ID)
	fmt.Printf("Name: %s\n", created.Name)
	fmt.Printf("URL: %s\n", created.URL)

	return nil
}

// RunWebhookCreateInteractive creates a webhook using interactive forms
func (h *WebhookHandler) RunWebhookCreateInteractive(cmd *cobra.Command, args []string) error {
	// TODO: Implement interactive form similar to MCP add form
	return fmt.Errorf("interactive mode not yet implemented")
}