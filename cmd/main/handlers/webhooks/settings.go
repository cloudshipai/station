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