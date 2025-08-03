package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/theme"
	"station/pkg/models"
)

// WebhookHandler handles webhook-related CLI commands
type WebhookHandler struct {
	themeManager *theme.ThemeManager
}

func NewWebhookHandler(themeManager *theme.ThemeManager) *WebhookHandler {
	return &WebhookHandler{themeManager: themeManager}
}

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

// Settings handlers

// RunSettingsList lists all settings
func (h *WebhookHandler) RunSettingsList(cmd *cobra.Command, args []string) error {
	endpoint, _ := cmd.Flags().GetString("endpoint")
	
	if endpoint == "" {
		// Local mode
		return h.runSettingsListLocal()
	}
	
	// Remote mode
	return h.runSettingsListRemote(endpoint)
}

func (h *WebhookHandler) runSettingsListLocal() error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	settings, err := repos.Settings.GetAll()
	if err != nil {
		return fmt.Errorf("failed to list settings: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("⚙️ Settings"))
	fmt.Println()

	if len(settings) == 0 {
		fmt.Println(styles.Info.Render("No settings found"))
		return nil
	}

	for _, setting := range settings {
		fmt.Printf("%s: %s\n", styles.Title.Render(setting.Key), setting.Value)
		if setting.Description != nil {
			fmt.Printf("  %s\n", styles.Info.Render(*setting.Description))
		}
		fmt.Println()
	}

	return nil
}

func (h *WebhookHandler) runSettingsListRemote(endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/settings", endpoint)
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
		Settings []*models.Setting `json:"settings"`
		Count    int               `json:"count"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Banner.Render("⚙️ Settings"))
	fmt.Println()

	if len(response.Settings) == 0 {
		fmt.Println(styles.Info.Render("No settings found"))
		return nil
	}

	for _, setting := range response.Settings {
		fmt.Printf("%s: %s\n", styles.Title.Render(setting.Key), setting.Value)
		if setting.Description != nil {
			fmt.Printf("  %s\n", styles.Info.Render(*setting.Description))
		}
		fmt.Println()
	}

	return nil
}

// RunSettingsGet gets a specific setting
func (h *WebhookHandler) RunSettingsGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("setting key is required")
	}

	key := args[0]
	endpoint, _ := cmd.Flags().GetString("endpoint")
	
	if endpoint == "" {
		// Local mode
		return h.getSettingLocal(key)
	}
	
	// Remote mode
	return h.getSettingRemote(endpoint, key)
}

func (h *WebhookHandler) getSettingLocal(key string) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	setting, err := repos.Settings.GetByKey(key)
	if err != nil {
		return fmt.Errorf("setting not found: %s", key)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("%s: %s\n", styles.Title.Render(setting.Key), setting.Value)
	if setting.Description != nil {
		fmt.Printf("Description: %s\n", *setting.Description)
	}

	return nil
}

func (h *WebhookHandler) getSettingRemote(endpoint, key string) error {
	url := fmt.Sprintf("%s/api/v1/settings/%s", endpoint, key)
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

	var setting models.Setting
	if err := json.Unmarshal(body, &setting); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("%s: %s\n", styles.Title.Render(setting.Key), setting.Value)
	if setting.Description != nil {
		fmt.Printf("Description: %s\n", *setting.Description)
	}

	return nil
}

// RunSettingsSet sets a setting value
func (h *WebhookHandler) RunSettingsSet(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("setting key and value are required")
	}

	key := args[0]
	value := args[1]
	description, _ := cmd.Flags().GetString("description")
	endpoint, _ := cmd.Flags().GetString("endpoint")
	
	if endpoint == "" {
		// Local mode
		return h.setSettingLocal(key, value, description)
	}
	
	// Remote mode
	return h.setSettingRemote(endpoint, key, value, description)
}

func (h *WebhookHandler) setSettingLocal(key, value, description string) error {
	databasePath := getDatabasePath()
	database, err := db.New(databasePath)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	err = repos.Settings.Set(key, value, description)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Setting updated successfully!"))
	fmt.Printf("%s: %s\n", key, value)

	return nil
}

func (h *WebhookHandler) setSettingRemote(endpoint, key, value, description string) error {
	payload := map[string]string{
		"value":       value,
		"description": description,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/settings/%s", endpoint, key)
	req, err := makeAuthenticatedRequest("PUT", url, bytes.NewReader(body))
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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API error: %s", string(respBody))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render("✅ Setting updated successfully!"))
	fmt.Printf("%s: %s\n", key, value)

	return nil
}

// getDatabasePath returns the database path for local mode
func getDatabasePath() string {
	dbPath := os.Getenv("STATION_DATABASE_URL")
	if dbPath == "" {
		dbPath = "station.db"
	}
	return dbPath
}