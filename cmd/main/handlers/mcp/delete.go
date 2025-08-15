package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// deleteMCPConfigLocal deletes an MCP configuration from local database
func (h *MCPHandler) deleteMCPConfigLocal(configID, environment string, confirm bool) error {
	// Load Station config
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Find environment
	env, err := repos.Environments.GetByName(environment)
	if err != nil {
		return fmt.Errorf("environment '%s' not found", environment)
	}

	// Find config (try by name first, then by ID)
	var config *repositories.FileConfigRecord
	if configByName, err := repos.FileMCPConfigs.GetByEnvironmentAndName(env.ID, configID); err == nil {
		config = configByName
		log.Printf("Found config by name: %s (ID: %d)", config.ConfigName, config.ID)
	} else {
		// Try parsing as ID
		if id, parseErr := strconv.ParseInt(configID, 10, 64); parseErr == nil {
			if configByID, err := repos.FileMCPConfigs.GetByID(id); err == nil {
				config = configByID
				log.Printf("Found config by ID: %d (Name: %s)", config.ID, config.ConfigName)
			}
		}
	}

	if config == nil {
		return fmt.Errorf("config '%s' not found", configID)
	}

	// Count associated servers and tools that will be cascade deleted
	servers, err := repos.MCPServers.GetByEnvironmentID(env.ID)
	if err != nil {
		return fmt.Errorf("failed to get servers: %w", err)
	}
	
	// Count servers associated with this config
	associatedServers := 0
	totalTools := 0
	for _, server := range servers {
		if server.FileConfigID != nil && *server.FileConfigID == config.ID {
			associatedServers++
			// Count tools for this server
			tools, err := repos.MCPTools.GetByServerID(server.ID)
			if err == nil {
				totalTools += len(tools)
			}
		}
	}

	// Show confirmation prompt if not already confirmed
	if !confirm {
		fmt.Printf("\n⚠️  This will delete:\n")
		fmt.Printf("• Configuration: %s (ID: %d)\n", config.ConfigName, config.ID)
		if associatedServers > 0 {
			fmt.Printf("• %d MCP servers\n", associatedServers)
		}
		if totalTools > 0 {
			fmt.Printf("• %d tools\n", totalTools)
		}
		fmt.Printf("• Template file and variables\n")
		fmt.Print("\nAre you sure? [y/N]: ")
		
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Delete the configuration (cascade delete should handle tools)
	err = repos.FileMCPConfigs.Delete(config.ID)
	if err != nil {
		return fmt.Errorf("failed to delete configuration: %w", err)
	}

	// Also remove template files from filesystem
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	envDir := filepath.Join(configHome, "station", "environments", environment)
	
	// Delete template file
	templatePath := filepath.Join(envDir, config.ConfigName+".json")
	if err := os.Remove(templatePath); err == nil {
		log.Printf("Deleted template file: %s", templatePath)
	}
	
	// Delete template-specific variables if they exist
	templateVarsPath := filepath.Join(envDir, config.ConfigName+".vars.yml")
	if err := os.Remove(templateVarsPath); err == nil {
		log.Printf("Deleted template variables: %s", templateVarsPath)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ %s\n", styles.Success.Render(fmt.Sprintf("Successfully deleted configuration '%s' (ID: %d) and all associated data", 
		config.ConfigName, config.ID)))

	return nil
}

// deleteMCPConfigRemote deletes an MCP configuration from remote API
func (h *MCPHandler) deleteMCPConfigRemote(configID, environment, endpoint string, confirm bool) error {
	// Get environment ID
	envID, err := getEnvironmentID(endpoint, environment)
	if err != nil {
		return fmt.Errorf("failed to get environment ID: %w", err)
	}

	// Parse config ID
	id, err := strconv.ParseInt(configID, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid config ID: %s", configID)
	}

	// Get config details first for confirmation
	url := fmt.Sprintf("%s/api/v1/environments/%d/mcp-configs/%d", endpoint, envID, id)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get config details: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get config details: status %d: %s", resp.StatusCode, string(body))
	}

	var configResult struct {
		Config *models.MCPConfig `json:"config"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&configResult); err != nil {
		return fmt.Errorf("failed to decode config response: %w", err)
	}

	config := configResult.Config

	// Show confirmation prompt if not already confirmed
	if !confirm {
		fmt.Printf("\n⚠️  This will delete:\n")
		fmt.Printf("• Configuration: %s (ID: %d)\n", config.ConfigName, config.ID)
		fmt.Print("• All associated tools and servers\n")
		fmt.Print("\nAre you sure? [y/N]: ")
		
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Delete the configuration
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete configuration: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete configuration: status %d: %s", resp.StatusCode, string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ %s\n", styles.Success.Render(fmt.Sprintf("Successfully deleted configuration '%s' (ID: %d)", 
		config.ConfigName, config.ID)))

	return nil
}