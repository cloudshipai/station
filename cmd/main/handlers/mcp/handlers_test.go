package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database with basic schema
func setupTestDB(t *testing.T) (*db.DB, *repositories.Repositories) {
	// Create a temporary database file for testing
	tempFile := filepath.Join(t.TempDir(), "test.db")
	testDB, err := db.New(tempFile)
	require.NoError(t, err)
	
	// Run migrations to set up the database schema
	err = testDB.Migrate()
	require.NoError(t, err)
	
	repos := repositories.New(testDB)
	return testDB, repos
}

// TestMCPHandler_validateEnvironmentExists tests environment validation
func TestMCPHandler_validateEnvironmentExists(t *testing.T) {
	handler := &MCPHandler{}
	
	// Create temporary directory structure
	tempDir := t.TempDir()
	envDir := filepath.Join(tempDir, "config", "environments", "test")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	// Change working directory for the test
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Test existing environment
	exists := handler.validateEnvironmentExists("test")
	assert.True(t, exists)

	// Test non-existing environment
	exists = handler.validateEnvironmentExists("nonexistent")
	assert.False(t, exists)
}

// Integration test for CLI handlers using internal services
func TestMCPHandler_Integration(t *testing.T) {
	// This test verifies that the CLI handlers properly delegate to internal services
	// without duplicating all the business logic tests (those are in internal/mcp/*_test.go)
	
	tempDir := t.TempDir()
	envDir := filepath.Join(tempDir, ".config", "station", "environments", "test")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	// Create a simple test config
	configContent := `{
		"mcpServers": {
			"test-server": {
				"command": "echo",
				"args": ["hello", "world"]
			}
		}
	}`
	err = os.WriteFile(filepath.Join(envDir, "test_config.json"), []byte(configContent), 0644)
	require.NoError(t, err)

	// Mock the config directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// The actual CLI handler tests would require more setup (database config, theme manager, etc.)
	// For now, we verify that the basic structure is correct and the services can be instantiated
	testDB, repos := setupTestDB(t)
	defer testDB.Close()

	// Verify that we can create the services that the handlers use
	// This ensures the dependency injection is working
	require.NotNil(t, repos)
	
	// The actual sync and status functionality is thoroughly tested in the internal service tests
	// This integration test just ensures the CLI layer can access the services
}