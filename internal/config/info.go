package config

import (
	"os"
	"path/filepath"
	"strings"
)

// ConfigInfo contains information about configuration paths and status
type ConfigInfo struct {
	ConfigFile     string            `json:"config_file"`
	DatabasePath   string            `json:"database_path"`
	IsLocalMode    bool              `json:"is_local_mode"`
	ConfigExists   bool              `json:"config_exists"`
	DatabaseExists bool              `json:"database_exists"`
	EnvVars        map[string]string `json:"env_vars"`
	ConfigDirs     []string          `json:"config_dirs"`
}

// GetConfigInfo returns comprehensive configuration information
func GetConfigInfo() ConfigInfo {
	info := ConfigInfo{
		EnvVars:    make(map[string]string),
		ConfigDirs: make([]string, 0),
	}

	// Get config file path
	configFile := getConfigFilePath()
	info.ConfigFile = configFile
	info.ConfigExists = fileExists(configFile)

	// Get database path
	databasePath := getDatabasePath()
	info.DatabasePath = databasePath
	info.DatabaseExists = fileExists(databasePath)

	// Detect local mode
	info.IsLocalMode = isLocalMode()

	// Get relevant environment variables
	envVars := []string{
		"STATION_CONFIG",
		"XDG_CONFIG_HOME",
		"HOME",
		"OPENAI_API_KEY",
		"STATION_DATABASE",
		"STATION_API_PORT",
		"STATION_SSH_PORT",
		"STATION_MCP_PORT",
		"STATION_DEBUG",
	}

	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			info.EnvVars[envVar] = value
		}
	}

	// Get configuration directories
	info.ConfigDirs = getConfigDirectories()

	return info
}

// getConfigFilePath returns the path to the configuration file
func getConfigFilePath() string {
	// Check STATION_CONFIG environment variable first
	if configPath := os.Getenv("STATION_CONFIG"); configPath != "" {
		return configPath
	}

	// Use the same directory logic as database
	configDir := getConfigDirectory()
	return filepath.Join(configDir, "config.yaml")
}

// getDatabasePath returns the path to the database file
func getDatabasePath() string {
	// Check STATION_DATABASE environment variable first
	if dbPath := os.Getenv("STATION_DATABASE"); dbPath != "" {
		return dbPath
	}

	// Default to same directory as config file
	configDir := getConfigDirectory()
	return filepath.Join(configDir, "station.db")
}

// getConfigDirectory returns the directory where configuration should be stored
func getConfigDirectory() string {
	// Check XDG_CONFIG_HOME first
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "station")
	}

	// Fallback to ~/.config/station
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".config", "station")
	}

	// Last resort: current directory
	return "."
}

// isLocalMode detects if the application is running in local development mode
func isLocalMode() bool {
	// Check for development indicators
	if os.Getenv("STATION_DEBUG") == "true" {
		return true
	}

	// Check if running from a local build (./station or ./bin/station)
	if executable, err := os.Executable(); err == nil {
		executableName := filepath.Base(executable)
		executableDir := filepath.Dir(executable)

		// Local mode indicators
		if executableName == "station" {
			// Check if in current directory or bin subdirectory
			if strings.HasSuffix(executableDir, "/bin") || executableDir == "." {
				return true
			}
		}
	}

	// Check current working directory for development files
	if fileExists("go.mod") && fileExists("Makefile") {
		return true
	}

	return false
}

// getConfigDirectories returns all potential configuration directories
func getConfigDirectories() []string {
	var dirs []string

	// XDG config directory
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		dirs = append(dirs, filepath.Join(xdgConfigHome, "station"))
	}

	// User config directory
	if homeDir, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(homeDir, ".config", "station"))
		dirs = append(dirs, filepath.Join(homeDir, ".station"))
	}

	// System config directories
	dirs = append(dirs, "/etc/station")
	dirs = append(dirs, "/usr/local/etc/station")

	// Current directory (for local development)
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}

	return dirs
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetLoadedConfigs returns information about which configurations are actually loaded
func GetLoadedConfigs() map[string]interface{} {
	configs := make(map[string]interface{})

	// This would be populated by the actual config loading logic
	// For now, we'll return basic information
	configs["config_file_loaded"] = fileExists(getConfigFilePath())
	configs["database_connected"] = fileExists(getDatabasePath())

	return configs
}

// GetDatabasePath returns the database path (exported for external use)
func GetDatabasePath() string {
	return getDatabasePath()
}

// GetConfigDirectory returns the config directory path (exported for external use)
func GetConfigDirectory() string {
	return getConfigDirectory()
}
