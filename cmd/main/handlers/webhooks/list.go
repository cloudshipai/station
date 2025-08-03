package webhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// RunWebhookList lists all webhooks
func (h *WebhookHandler) RunWebhookList(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint == "" {
		// Local mode
		return h.runWebhookListLocal()
	}

	// Remote mode
	return h.runWebhookListRemote(endpoint)
}

func (h *WebhookHandler) runWebhookListLocal() error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	webhooks, err := repos.Webhooks.List()
	if err != nil {
		return fmt.Errorf("failed to list webhooks: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("🪝 Webhooks"))
	fmt.Println()

	if len(webhooks) == 0 {
		fmt.Println(styles.Info.Render("No webhooks found"))
		return nil
	}

	for _, webhook := range webhooks {
		status := "✅ Enabled"
		if !webhook.Enabled {
			status = "❌ Disabled"
		}

		fmt.Printf("%s %s (ID: %d)\n", status, styles.Title.Render(webhook.Name), webhook.ID)
		fmt.Printf("  URL: %s\n", webhook.URL)

		// Parse events
		var events []string
		if err := json.Unmarshal([]byte(webhook.Events), &events); err == nil {
			fmt.Printf("  Events: %s\n", strings.Join(events, ", "))
		} else {
			fmt.Printf("  Events: %s\n", webhook.Events)
		}

		fmt.Printf("  Timeout: %ds, Retries: %d\n", webhook.TimeoutSeconds, webhook.RetryAttempts)
		fmt.Println()
	}

	return nil
}

func (h *WebhookHandler) runWebhookListRemote(endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/webhooks", endpoint)
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

	var response struct {
		Webhooks []*models.Webhook `json:"webhooks"`
		Count    int               `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("🪝 Webhooks"))
	fmt.Println()

	if len(response.Webhooks) == 0 {
		fmt.Println(styles.Info.Render("No webhooks found"))
		return nil
	}

	for _, webhook := range response.Webhooks {
		status := "✅ Enabled"
		if !webhook.Enabled {
			status = "❌ Disabled"
		}

		fmt.Printf("%s %s (ID: %d)\n", status, styles.Title.Render(webhook.Name), webhook.ID)
		fmt.Printf("  URL: %s\n", webhook.URL)

		// Parse events
		var events []string
		if err := json.Unmarshal([]byte(webhook.Events), &events); err == nil {
			fmt.Printf("  Events: %s\n", strings.Join(events, ", "))
		} else {
			fmt.Printf("  Events: %s\n", webhook.Events)
		}

		fmt.Printf("  Timeout: %ds, Retries: %d\n", webhook.TimeoutSeconds, webhook.RetryAttempts)
		fmt.Println()
	}

	return nil
}