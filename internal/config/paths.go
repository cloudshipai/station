package config

import (
	"os"
	"path/filepath"
)

// GetConfigRoot returns the Station configuration root directory
// This handles container vs host runtime differences
// In container: /home/station/.config/station (or STATION_CONFIG_DIR if set)
// On host: Uses GetStationConfigDir() which checks workspace/XDG paths
func GetConfigRoot() string {
	// Check if we're running in container
	if os.Getenv("STATION_RUNTIME") == "docker" {
		// Allow override via environment variable
		if configDir := os.Getenv("STATION_CONFIG_DIR"); configDir != "" {
			return configDir
		}
		// Default to station user's home directory
		return "/home/station/.config/station"
	}

	// Use the existing GetStationConfigDir which handles workspace configuration
	return GetStationConfigDir()
}

// GetEnvironmentDir returns the directory for a specific environment
func GetEnvironmentDir(environmentName string) string {
	return filepath.Join(GetConfigRoot(), "environments", environmentName)
}

// GetAgentsDir returns the agents directory for a specific environment
func GetAgentsDir(environmentName string) string {
	return filepath.Join(GetEnvironmentDir(environmentName), "agents")
}

// GetAgentPromptPath returns the path to an agent's prompt file
func GetAgentPromptPath(environmentName, agentName string) string {
	return filepath.Join(GetAgentsDir(environmentName), agentName+".prompt")
}

// GetVariablesPath returns the path to the variables.yml file for an environment
func GetVariablesPath(environmentName string) string {
	return filepath.Join(GetEnvironmentDir(environmentName), "variables.yml")
}

// GetTemplateConfigPath returns the path to a template config file
func GetTemplateConfigPath(environmentName, configName string) string {
	return filepath.Join(GetEnvironmentDir(environmentName), configName+".json")
}

// ResolvePath converts a relative environment path to an absolute path
// Handles paths like "environments/default/cost-explorer.json"
func ResolvePath(path string) string {
	// If it's already absolute, return as-is
	if filepath.IsAbs(path) {
		return path
	}

	// If it starts with "environments/", resolve relative to config dir
	if len(path) > 13 && path[:13] == "environments/" {
		return filepath.Join(GetConfigRoot(), path)
	}

	// Otherwise return as-is
	return path
}
