package webhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// RunWebhookShow shows webhook details
func (h *WebhookHandler) RunWebhookShow(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("webhook ID is required")
	}

	webhookID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook ID: %s", args[0])
	}

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint == "" {
		// Local mode
		return h.showWebhookLocal(webhookID)
	}

	// Remote mode
	return h.showWebhookRemote(endpoint, webhookID)
}

func (h *WebhookHandler) showWebhookLocal(webhookID int64) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	webhook, err := repos.Webhooks.GetByID(webhookID)
	if err != nil {
		return fmt.Errorf("failed to get webhook: %w", err)
	}

	h.displayWebhookDetails(webhook)
	return nil
}

func (h *WebhookHandler) showWebhookRemote(endpoint string, webhookID int64) error {
	url := fmt.Sprintf("%s/api/v1/webhooks/%d", endpoint, webhookID)
	req, err := makeAuthenticatedRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(body))
	}

	var webhook models.Webhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	h.displayWebhookDetails(&webhook)
	return nil
}

func (h *WebhookHandler) displayWebhookDetails(webhook *models.Webhook) {
	styles := getCLIStyles(h.themeManager)

	status := "✅ Enabled"
	if !webhook.Enabled {
		status = "❌ Disabled"
	}

	fmt.Println(styles.Banner.Render(fmt.Sprintf("🪝 Webhook: %s", webhook.Name)))
	fmt.Println()
	fmt.Printf("ID: %d\n", webhook.ID)
	fmt.Printf("Status: %s\n", status)
	fmt.Printf("URL: %s\n", webhook.URL)

	if webhook.Secret != "" {
		fmt.Printf("Secret: %s\n", styles.Info.Render("[configured]"))
	} else {
		fmt.Printf("Secret: %s\n", styles.Info.Render("[none]"))
	}

	// Parse and display events
	var events []string
	if err := json.Unmarshal([]byte(webhook.Events), &events); err == nil {
		fmt.Printf("Events: %s\n", strings.Join(events, ", "))
	} else {
		fmt.Printf("Events: %s\n", webhook.Events)
	}

	// Parse and display headers if present
	if webhook.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(webhook.Headers), &headers); err == nil && len(headers) > 0 {
			fmt.Println("Custom Headers:")
			for key, value := range headers {
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}

	fmt.Printf("Timeout: %ds\n", webhook.TimeoutSeconds)
	fmt.Printf("Retry Attempts: %d\n", webhook.RetryAttempts)
	fmt.Printf("Created: %s\n", webhook.CreatedAt.Format("2006-01-02 15:04:05"))
}