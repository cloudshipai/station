package services

import (
	"testing"

	"station/internal/db/repositories"
	"station/pkg/crypto"
	"station/pkg/models"

	_ "modernc.org/sqlite"
)

func TestToolReplacement_Basic(t *testing.T) {
	// Setup test database
	db := setupTestDBForServices(t)
	defer db.Close()

	// Create repositories
	repos := repositories.New(&mockDB{conn: db})

	// Create services
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test environment"))
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	// Test config data - using a simple echo server that will fail tool discovery
	// but will allow us to test the transaction workflow
	configData1 := &models.MCPConfigData{
		Name: "test-config",
		Servers: map[string]models.MCPServerConfig{
			"echo-server": {
				Command: "echo",
				Args:    []string{"hello"},
				Env:     map[string]string{},
			},
		},
	}

	// Upload first version of config
	savedConfig1, err := mcpConfigService.UploadConfig(env.ID, configData1)
	if err != nil {
		t.Fatalf("Failed to upload first config version: %v", err)
	}

	t.Logf("Uploaded config version 1: ID=%d, Name=%s, Version=%d", 
		savedConfig1.ID, savedConfig1.ConfigName, savedConfig1.Version)

	// Verify the config name is set correctly
	if savedConfig1.ConfigName != "test-config" {
		t.Errorf("Expected config name 'test-config', got '%s'", savedConfig1.ConfigName)
	}

	// Verify version is 1
	if savedConfig1.Version != 1 {
		t.Errorf("Expected version 1, got %d", savedConfig1.Version)
	}

	// Run tool replacement for the first config
	result1, err := toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, "test-config")
	if err != nil {
		t.Fatalf("Failed to replace tools for first config: %v", err)
	}

	t.Logf("First tool replacement result: Success=%v, Servers=%d/%d, Tools=%d, Errors=%d", 
		result1.Success, result1.SuccessfulServers, result1.TotalServers, result1.TotalTools, len(result1.Errors))

	// The echo server should fail tool discovery, but transaction should still work
	if result1.TotalServers != 1 {
		t.Errorf("Expected 1 server, got %d", result1.TotalServers)
	}

	// Upload second version with different server
	configData2 := &models.MCPConfigData{
		Name: "test-config", // Same name, new version
		Servers: map[string]models.MCPServerConfig{
			"different-server": {
				Command: "pwd",
				Args:    []string{},
				Env:     map[string]string{},
			},
		},
	}

	// Upload second version of config
	savedConfig2, err := mcpConfigService.UploadConfig(env.ID, configData2)
	if err != nil {
		t.Fatalf("Failed to upload second config version: %v", err)
	}

	t.Logf("Uploaded config version 2: ID=%d, Name=%s, Version=%d", 
		savedConfig2.ID, savedConfig2.ConfigName, savedConfig2.Version)

	// Verify version numbers
	if savedConfig2.Version != 2 {
		t.Errorf("Expected config version 2, got %d", savedConfig2.Version)
	}

	if savedConfig1.ConfigName != savedConfig2.ConfigName {
		t.Errorf("Config names should match: %s != %s", savedConfig1.ConfigName, savedConfig2.ConfigName)
	}

	// Run tool replacement for the second config
	result2, err := toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, "test-config")
	if err != nil {
		t.Fatalf("Failed to replace tools for second config: %v", err)
	}

	t.Logf("Second tool replacement result: Success=%v, Servers=%d/%d, Tools=%d, Errors=%d", 
		result2.Success, result2.SuccessfulServers, result2.TotalServers, result2.TotalTools, len(result2.Errors))

	// Verify that only the latest config version is being used
	latestConfig, err := repos.MCPConfigs.GetLatestByName(env.ID, "test-config")
	if err != nil {
		t.Fatalf("Failed to get latest config: %v", err)
	}

	if latestConfig.ID != savedConfig2.ID {
		t.Errorf("Latest config should be version 2, got config ID %d (expected %d)", 
			latestConfig.ID, savedConfig2.ID)
	}

	// Verify that servers are associated with the latest config only
	servers, err := repos.MCPServers.GetByConfigID(latestConfig.ID)
	if err != nil {
		t.Fatalf("Failed to get servers for latest config: %v", err)
	}

	if len(servers) != 1 {
		t.Errorf("Expected 1 server for latest config, got %d", len(servers))
	}

	if len(servers) > 0 && servers[0].Name != "different-server" {
		t.Errorf("Expected server name 'different-server', got '%s'", servers[0].Name)
	}

	// Verify old config still has its servers (they remain for historical purposes)
	// The tool replacement only affects the latest config version
	oldServers, err := repos.MCPServers.GetByConfigID(savedConfig1.ID)
	if err != nil {
		t.Fatalf("Failed to get servers for old config: %v", err)
	}

	if len(oldServers) != 1 {
		t.Errorf("Expected 1 server for old config (preserved for history), got %d", len(oldServers))
	}

	if len(oldServers) > 0 && oldServers[0].Name != "echo-server" {
		t.Errorf("Expected old server name 'echo-server', got '%s'", oldServers[0].Name)
	}

	t.Log("Tool replacement transaction test completed successfully")
}