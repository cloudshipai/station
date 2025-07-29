package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL     string
	SSHPort         int
	MCPPort         int
	APIPort         int
	SSHHostKeyPath  string
	AdminUsername   string
	Environment     string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:    getEnvOrDefault("DATABASE_URL", "station.db"),
		SSHPort:        getEnvIntOrDefault("SSH_PORT", 2222),
		MCPPort:        getEnvIntOrDefault("MCP_PORT", 3000),
		APIPort:        getEnvIntOrDefault("API_PORT", 8080),
		SSHHostKeyPath: getEnvOrDefault("SSH_HOST_KEY_PATH", "./ssh_host_key"),
		AdminUsername:  getEnvOrDefault("ADMIN_USERNAME", "admin"),
		Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
	}

	// Validate that ENCRYPTION_KEY environment variable exists
	// KeyManager will handle the actual key loading and validation
	if os.Getenv("ENCRYPTION_KEY") == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required")
	}

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}