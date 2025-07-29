package services

import (
	"testing"

	"station/internal/db/repositories"
	"station/pkg/crypto"
	"station/pkg/models"

	_ "modernc.org/sqlite"
)

func TestTransactionPropagation(t *testing.T) {
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

	// Test config data with a server that will be saved but fail tool discovery
	configData := &models.MCPConfigData{
		Name: "tx-test-config",
		Servers: map[string]models.MCPServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"hello"},
				Env:     map[string]string{},
			},
		},
	}

	// Upload config
	savedConfig, err := mcpConfigService.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	t.Logf("Uploaded config: ID=%d, Name=%s, Version=%d", 
		savedConfig.ID, savedConfig.ConfigName, savedConfig.Version)

	// Now we need to manually insert a server and tools to test transaction cleanup
	// This simulates having existing data that should be cleaned up

	// Create a server manually
	server := &models.MCPServer{
		MCPConfigID: savedConfig.ID,
		Name:        "manual-server",
		Command:     "test-command",
		Args:        []string{"arg1", "arg2"},
		Env:         map[string]string{"TEST": "value"},
	}

	serverID, err := repos.MCPServers.Create(server)
	if err != nil {
		t.Fatalf("Failed to create manual server: %v", err)
	}

	t.Logf("Created manual server with ID: %d", serverID)

	// Create some tools for this server
	tool1 := &models.MCPTool{
		MCPServerID: serverID,
		Name:        "test_tool_1",
		Description: "Test tool 1",
		Schema:      []byte(`{"type": "object"}`),
	}

	tool1ID, err := repos.MCPTools.Create(tool1)
	if err != nil {
		t.Fatalf("Failed to create tool 1: %v", err)
	}

	tool2 := &models.MCPTool{
		MCPServerID: serverID,
		Name:        "test_tool_2", 
		Description: "Test tool 2",
		Schema:      []byte(`{"type": "object"}`),
	}

	tool2ID, err := repos.MCPTools.Create(tool2)
	if err != nil {
		t.Fatalf("Failed to create tool 2: %v", err)
	}

	t.Logf("Created tools with IDs: %d, %d", tool1ID, tool2ID)

	// Verify initial state before transaction
	initialServers, err := repos.MCPServers.GetByConfigID(savedConfig.ID)
	if err != nil {
		t.Fatalf("Failed to get initial servers: %v", err)
	}

	if len(initialServers) != 1 {
		t.Errorf("Expected 1 initial server, got %d", len(initialServers))
	}

	initialTools, err := repos.MCPTools.GetByServerID(serverID)
	if err != nil {
		t.Fatalf("Failed to get initial tools: %v", err)
	}

	if len(initialTools) != 2 {
		t.Errorf("Expected 2 initial tools, got %d", len(initialTools))
	}

	// Test the transaction propagation by calling ReplaceToolsWithTransaction
	// This should clean up existing data atomically
	result, err := toolDiscoveryService.ReplaceToolsWithTransaction(env.ID, "tx-test-config")
	if err != nil {
		t.Fatalf("Failed to replace tools: %v", err)
	}

	t.Logf("Tool replacement result: Success=%v, Servers=%d/%d, Tools=%d, Errors=%d", 
		result.Success, result.SuccessfulServers, result.TotalServers, result.TotalTools, len(result.Errors))

	// Verify that transaction properly cleaned up data
	finalServers, err := repos.MCPServers.GetByConfigID(savedConfig.ID)
	if err != nil {
		t.Fatalf("Failed to get final servers: %v", err)
	}

	t.Logf("Final server count: %d", len(finalServers))

	// The transaction should have cleared the existing server and tools
	// Since the echo command fails discovery, no new servers should be created
	if len(finalServers) != 0 {
		t.Errorf("Expected 0 final servers after transaction cleanup, got %d", len(finalServers))
	}

	// Verify that tools were also cleaned up
	finalTools, err := repos.MCPTools.GetByServerID(serverID)
	if err != nil {
		// This is expected - the server should be deleted, so tools query might fail
		t.Logf("Tools query failed as expected after server deletion: %v", err)
	} else if len(finalTools) != 0 {
		t.Errorf("Expected 0 final tools after cleanup, got %d", len(finalTools))
	}

	t.Log("Transaction propagation test completed successfully - all data properly cleaned up within transaction")
}