package load

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/viper"
	"station/pkg/models"
)

// uploadConfigRemoteLoad uploads config to remote API
func (h *LoadHandler) uploadConfigRemoteLoad(mcpConfig LoadMCPConfig, configName, environment, endpoint string) error {
	// Get or create environment
	envID, err := getOrCreateEnvironmentRemote(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get/create environment: %w", err)
	}

	// Convert to API format
	servers := make(map[string]models.MCPServerConfig)
	for name, serverConfig := range mcpConfig.MCPServers {
		servers[name] = models.MCPServerConfig{
			Command: serverConfig.Command,
			Args:    serverConfig.Args,
			Env:     serverConfig.Env,
		}
	}

	uploadRequest := struct {
		Name    string                            `json:"name"`
		Servers map[string]models.MCPServerConfig `json:"servers"`
	}{
		Name:    configName,
		Servers: servers,
	}

	jsonData, err := json.Marshal(uploadRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Upload config
	url := fmt.Sprintf("%s/api/v1/environments/%d/mcp-configs", endpoint, envID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload config: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Config *models.MCPConfig `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Println(getCLIStyles(h.themeManager).Success.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d",
		result.Config.ConfigName, result.Config.Version)))

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 Tool discovery started in background"))

	showSuccessBanner("MCP Configuration Loaded Successfully!", h.themeManager)
	return nil
}

// getOrCreateEnvironmentRemote gets or creates an environment via remote API
func getOrCreateEnvironmentRemote(endpoint, envName string) (int64, error) {
	// Try to get existing environment
	envID, err := getEnvironmentID(endpoint, envName)
	if err == nil {
		return envID, nil
	}

	// Environment doesn't exist, create it
	createRequest := struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}{
		Name:        envName,
		Description: &[]string{fmt.Sprintf("Environment for %s", envName)}[0],
	}

	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		return 0, err
	}

	url := fmt.Sprintf("%s/api/v1/environments", endpoint)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to create environment: status %d: %s", resp.StatusCode, string(body))
	}

	var createResult struct {
		Environment *models.Environment `json:"environment"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&createResult); err != nil {
		return 0, err
	}

	return createResult.Environment.ID, nil
}

// uploadConfigRemoteWizard uploads wizard config to remote API
func (h *LoadHandler) uploadConfigRemoteWizard(configData *models.MCPConfigData, environment, endpoint string) error {
	// Get or create environment
	envID, err := getOrCreateEnvironmentRemote(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get/create environment: %w", err)
	}

	uploadRequest := struct {
		Name    string                            `json:"name"`
		Servers map[string]models.MCPServerConfig `json:"servers"`
	}{
		Name:    configData.Name,
		Servers: configData.Servers,
	}

	jsonData, err := json.Marshal(uploadRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Upload config
	url := fmt.Sprintf("%s/api/v1/environments/%d/mcp-configs", endpoint, envID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to upload config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to upload config: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Config *models.MCPConfig `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Println(getCLIStyles(h.themeManager).Success.Render(fmt.Sprintf("✅ Successfully uploaded config: %s v%d",
		result.Config.ConfigName, result.Config.Version)))

	fmt.Println(getCLIStyles(h.themeManager).Info.Render("🔍 Tool discovery started in background"))

	showSuccessBanner("MCP Configuration Uploaded Successfully!", h.themeManager)
	return nil
}

// uploadGeneratedConfig uploads a configuration generated by the wizard
func (h *LoadHandler) uploadGeneratedConfig(configData *models.MCPConfigData, environment, endpoint string) error {

	// Determine if we're in local mode
	isLocal := endpoint == "" && viper.GetBool("local_mode")

	if isLocal {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🏠 Uploading to local database..."))
		return h.uploadConfigLocalWizard(configData, environment)
	} else if endpoint != "" {
		fmt.Println(getCLIStyles(h.themeManager).Info.Render("🌐 Uploading to: " + endpoint))
		return h.uploadConfigRemoteWizard(configData, environment, endpoint)
	} else {
		return fmt.Errorf("no endpoint specified and local_mode is false in config. Use --endpoint flag or enable local_mode in config")
	}
}
