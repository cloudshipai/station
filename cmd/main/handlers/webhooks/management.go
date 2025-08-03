package webhooks

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
)

// RunWebhookDelete deletes a webhook
func (h *WebhookHandler) RunWebhookDelete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("webhook ID is required")
	}

	webhookID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid webhook ID: %s", args[0])
	}

	confirm, _ := cmd.Flags().GetBool("confirm")
	if !confirm {
		fmt.Printf("Are you sure you want to delete webhook %d? (y/N): ", webhookID)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint == "" {
		// Local mode
		return h.deleteWebhookLocal(webhookID)
	}

	// Remote mode
	return h.deleteWebhookRemote(endpoint, webhookID)
}

func (h *WebhookHandler) deleteWebhookLocal(webhookID int64) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	err = repos.Webhooks.Delete(webhookID)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook deleted successfully!"))

	return nil
}

func (h *WebhookHandler) deleteWebhookRemote(endpoint string, webhookID int64) error {
	url := fmt.Sprintf("%s/api/v1/webhooks/%d", endpoint, webhookID)
	req, err := makeAuthenticatedRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook deleted successfully!"))

	return nil
}

// RunWebhookEnable enables a webhook
func (h *WebhookHandler) RunWebhookEnable(cmd *cobra.Command, args []string) error {
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
		return h.enableWebhookLocal(webhookID)
	}

	// Remote mode
	return h.enableWebhookRemote(endpoint, webhookID)
}

func (h *WebhookHandler) enableWebhookLocal(webhookID int64) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	err = repos.Webhooks.Enable(webhookID)
	if err != nil {
		return fmt.Errorf("failed to enable webhook: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook enabled successfully!"))

	return nil
}

func (h *WebhookHandler) enableWebhookRemote(endpoint string, webhookID int64) error {
	url := fmt.Sprintf("%s/api/v1/webhooks/%d/enable", endpoint, webhookID)
	req, err := makeAuthenticatedRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook enabled successfully!"))

	return nil
}

// RunWebhookDisable disables a webhook
func (h *WebhookHandler) RunWebhookDisable(cmd *cobra.Command, args []string) error {
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
		return h.disableWebhookLocal(webhookID)
	}

	// Remote mode
	return h.disableWebhookRemote(endpoint, webhookID)
}

func (h *WebhookHandler) disableWebhookLocal(webhookID int64) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	err = repos.Webhooks.Disable(webhookID)
	if err != nil {
		return fmt.Errorf("failed to disable webhook: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook disabled successfully!"))

	return nil
}

func (h *WebhookHandler) disableWebhookRemote(endpoint string, webhookID int64) error {
	url := fmt.Sprintf("%s/api/v1/webhooks/%d/disable", endpoint, webhookID)
	req, err := makeAuthenticatedRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: %s", string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Webhook disabled successfully!"))

	return nil
}