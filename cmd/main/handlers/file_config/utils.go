package file_config

import (
	"fmt"
	"os"
	"strings"
)

// getConfigDatabasePath gets the database path from config or returns fallback
func getConfigDatabasePath() string {
	// Try to read from the expected config file location
	configPath := os.ExpandEnv("$HOME/.config/station/config.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		// Simple parsing to extract database_url
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "database_url:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	
	// Fallback to local database
	return "station.db"
}

// generateSampleTemplate generates a sample MCP configuration template
func (h *FileConfigHandler) generateSampleTemplate(configName string) string {
	template := `{
  "name": "%s",
  "servers": {
    "%s-server": {
      "command": "node",
      "args": ["/usr/local/lib/node_modules/@modelcontextprotocol/server-github/dist/index.js"],
      "env": {
        "GITHUB_TOKEN": "{{.GithubToken}}",
        "GITHUB_REPO": "{{.GithubRepo}}"
      }
    }
  }
}`
	return fmt.Sprintf(template, configName, configName)
}

// getOrCreateEnvironmentID gets environment ID from database, creating if needed
func (h *FileConfigHandler) getOrCreateEnvironmentID(envName string) (int64, error) {
	// Try to get existing environment
	env, err := h.repos.Environments.GetByName(envName)
	if err == nil {
		return env.ID, nil
	}
	
	// Environment doesn't exist, create it
	description := fmt.Sprintf("Auto-created environment for file-based config: %s", envName)
	env, err = h.repos.Environments.Create(envName, &description, 1) // Default user ID 1
	if err != nil {
		return 0, fmt.Errorf("failed to create environment: %w", err)
	}
	
	return env.ID, nil
}

// validateEnvironmentExists checks if file-based environment directory exists
func (h *FileConfigHandler) validateEnvironmentExists(envName string) error {
	configDir := fmt.Sprintf("./config/environments/%s", envName)
	if _, err := os.Stat(configDir); err != nil {
		return fmt.Errorf("environment '%s' not found at %s", envName, configDir)
	}
	return nil
}