package config

import (
	"os"
	"testing"
)

func TestLoad_WithValidEncryptionKey(t *testing.T) {
	// Set up environment
	originalKey := os.Getenv("ENCRYPTION_KEY")
	defer func() {
		if originalKey == "" {
			os.Unsetenv("ENCRYPTION_KEY")
		} else {
			os.Setenv("ENCRYPTION_KEY", originalKey)
		}
	}()

	// Test with 32-byte key
	testKey := "0123456789abcdef0123456789abcdef" // 32 bytes
	os.Setenv("ENCRYPTION_KEY", testKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected config to load successfully, got error: %v", err)
	}

	// Check defaults
	if cfg.DatabaseURL != "station.db" {
		t.Errorf("Expected default database URL to be 'station.db', got %s", cfg.DatabaseURL)
	}
	if cfg.SSHPort != 2222 {
		t.Errorf("Expected default SSH port to be 2222, got %d", cfg.SSHPort)
	}
	if cfg.MCPPort != 3000 {
		t.Errorf("Expected default MCP port to be 3000, got %d", cfg.MCPPort)
	}
	if cfg.APIPort != 8080 {
		t.Errorf("Expected default API port to be 8080, got %d", cfg.APIPort)
	}
}

func TestLoad_WithoutEncryptionKey(t *testing.T) {
	// Ensure no encryption key is set
	originalKey := os.Getenv("ENCRYPTION_KEY")
	os.Unsetenv("ENCRYPTION_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ENCRYPTION_KEY", originalKey)
		}
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error when ENCRYPTION_KEY is not set")
	}

	expectedError := "ENCRYPTION_KEY environment variable is required"
	if err.Error() != expectedError {
		t.Errorf("Expected error message '%s', got '%s'", expectedError, err.Error())
	}
}

func TestLoad_WithInvalidEncryptionKeyLength(t *testing.T) {
	originalKey := os.Getenv("ENCRYPTION_KEY")
	defer func() {
		if originalKey == "" {
			os.Unsetenv("ENCRYPTION_KEY")
		} else {
			os.Setenv("ENCRYPTION_KEY", originalKey)
		}
	}()

	// Test with key that's too short
	os.Setenv("ENCRYPTION_KEY", "tooshort")

	// Config should load successfully - key validation is now done by KeyManager
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Config should load with any ENCRYPTION_KEY set, validation is in KeyManager: %v", err)
	}
	
	if cfg == nil {
		t.Error("Expected config to be returned")
	}
}

func TestLoad_WithEnvironmentVariables(t *testing.T) {
	// Set up test environment
	originalVars := map[string]string{
		"ENCRYPTION_KEY":    os.Getenv("ENCRYPTION_KEY"),
		"DATABASE_URL":      os.Getenv("DATABASE_URL"),
		"SSH_PORT":          os.Getenv("SSH_PORT"),
		"MCP_PORT":          os.Getenv("MCP_PORT"),
		"API_PORT":          os.Getenv("API_PORT"),
		"SSH_HOST_KEY_PATH": os.Getenv("SSH_HOST_KEY_PATH"),
		"ADMIN_USERNAME":    os.Getenv("ADMIN_USERNAME"),
	}
	defer func() {
		for key, value := range originalVars {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}()

	// Set test values
	os.Setenv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	os.Setenv("DATABASE_URL", "test.db")
	os.Setenv("SSH_PORT", "2223")
	os.Setenv("MCP_PORT", "3001")
	os.Setenv("API_PORT", "8081")
	os.Setenv("SSH_HOST_KEY_PATH", "/test/key")
	os.Setenv("ADMIN_USERNAME", "testadmin")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.DatabaseURL != "test.db" {
		t.Errorf("Expected database URL 'test.db', got '%s'", cfg.DatabaseURL)
	}
	if cfg.SSHPort != 2223 {
		t.Errorf("Expected SSH port 2223, got %d", cfg.SSHPort)
	}
	if cfg.MCPPort != 3001 {
		t.Errorf("Expected MCP port 3001, got %d", cfg.MCPPort)
	}
	if cfg.APIPort != 8081 {
		t.Errorf("Expected API port 8081, got %d", cfg.APIPort)
	}
	if cfg.SSHHostKeyPath != "/test/key" {
		t.Errorf("Expected SSH host key path '/test/key', got '%s'", cfg.SSHHostKeyPath)
	}
	if cfg.AdminUsername != "testadmin" {
		t.Errorf("Expected admin username 'testadmin', got '%s'", cfg.AdminUsername)
	}
}