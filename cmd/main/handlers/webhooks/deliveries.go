package webhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// RunWebhookDeliveries shows webhook deliveries
func (h *WebhookHandler) RunWebhookDeliveries(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint == "" {
		// Local mode
		if len(args) > 0 {
			webhookID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid webhook ID: %s", args[0])
			}
			return h.showWebhookDeliveriesLocal(webhookID, limit)
		}
		return h.showAllDeliveriesLocal(limit)
	}

	// Remote mode
	if len(args) > 0 {
		webhookID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid webhook ID: %s", args[0])
		}
		return h.showWebhookDeliveriesRemote(endpoint, webhookID, limit)
	}
	return h.showAllDeliveriesRemote(endpoint, limit)
}

func (h *WebhookHandler) showWebhookDeliveriesLocal(webhookID int64, limit int) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	deliveries, err := repos.WebhookDeliveries.ListByWebhook(webhookID, limit)
	if err != nil {
		return fmt.Errorf("failed to get deliveries: %w", err)
	}

	h.displayDeliveries(deliveries, fmt.Sprintf("Deliveries for Webhook %d", webhookID))
	return nil
}

func (h *WebhookHandler) showAllDeliveriesLocal(limit int) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	deliveries, err := repos.WebhookDeliveries.List(limit)
	if err != nil {
		return fmt.Errorf("failed to get deliveries: %w", err)
	}

	h.displayDeliveries(deliveries, "All Webhook Deliveries")
	return nil
}

func (h *WebhookHandler) showWebhookDeliveriesRemote(endpoint string, webhookID int64, limit int) error {
	url := fmt.Sprintf("%s/api/v1/webhooks/%d/deliveries?limit=%d", endpoint, webhookID, limit)
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
		Deliveries []*models.WebhookDelivery `json:"deliveries"`
		Count      int                       `json:"count"`
		WebhookID  int64                     `json:"webhook_id"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	h.displayDeliveries(response.Deliveries, fmt.Sprintf("Deliveries for Webhook %d", response.WebhookID))
	return nil
}

func (h *WebhookHandler) showAllDeliveriesRemote(endpoint string, limit int) error {
	url := fmt.Sprintf("%s/api/v1/webhook-deliveries?limit=%d", endpoint, limit)
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
		Deliveries []*models.WebhookDelivery `json:"deliveries"`
		Count      int                       `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	h.displayDeliveries(response.Deliveries, "All Webhook Deliveries")
	return nil
}

func (h *WebhookHandler) displayDeliveries(deliveries []*models.WebhookDelivery, title string) {
	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("📨 " + title))
	fmt.Println()

	if len(deliveries) == 0 {
		fmt.Println(styles.Info.Render("No deliveries found"))
		return
	}

	for _, delivery := range deliveries {
		statusIcon := "🟢"
		switch delivery.Status {
		case "failed":
			statusIcon = "🔴"
		case "pending":
			statusIcon = "🟡"
		}

		fmt.Printf("%s Delivery %d - %s\n", statusIcon, delivery.ID, styles.Title.Render(delivery.EventType))
		fmt.Printf("  Webhook ID: %d\n", delivery.WebhookID)
		fmt.Printf("  Status: %s\n", delivery.Status)
		fmt.Printf("  Attempts: %d\n", delivery.AttemptCount)
		fmt.Printf("  Created: %s\n", delivery.CreatedAt.Format("2006-01-02 15:04:05"))

		if delivery.HTTPStatusCode != nil {
			fmt.Printf("  HTTP Status: %d\n", *delivery.HTTPStatusCode)
		}

		if delivery.ErrorMessage != nil {
			fmt.Printf("  Error: %s\n", styles.Error.Render(*delivery.ErrorMessage))
		}

		if delivery.DeliveredAt != nil {
			fmt.Printf("  Delivered: %s\n", delivery.DeliveredAt.Format("2006-01-02 15:04:05"))
		}

		fmt.Println()
	}
}