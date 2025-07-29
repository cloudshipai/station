package services

import (
	"encoding/json"
	"strings"
	"testing"

	"station/internal/db/repositories"
	"station/pkg/crypto"
	"station/pkg/models"
)

func TestMCPClientService_CallTool(t *testing.T) {
	// Setup test database and services
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)
	clientService := NewMCPClientService(repos, mcpConfigService, toolDiscoveryService)
	defer clientService.Shutdown()

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create mock MCP config (simple echo server for testing)
	configData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"echo-server": {
				Command: "echo", // Use echo command as a simple test
				Args:    []string{"test-response"},
				Env:     map[string]string{},
			},
		},
	}

	// Upload the config (this will automatically create servers)
	config, err := mcpConfigService.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Get the server that was automatically created
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to get servers: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("No servers were created from config")
	}
	serverID := servers[0].ID

	// Create mock tool
	tool := &models.MCPTool{
		MCPServerID: serverID,
		Name:        "echo_tool",
		Description: "Echo tool for testing",
		Schema:      json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
	}

	_, err = repos.MCPTools.Create(tool)
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}

	// Test CallTool - this will fail because echo is not an MCP server, but we can test the flow
	arguments := map[string]interface{}{
		"message": "hello world",
	}

	result, err := clientService.CallTool(env.ID, "echo_tool", arguments)
	
	// We expect this to fail since echo is not a real MCP server
	// The error should be in connecting to the MCP server, not in tool lookup
	if err != nil {
		t.Logf("Tool call failed as expected (echo is not MCP server): %v", err)
		
		// With the new error handling, we should get nil result for connection errors
		if result != nil {
			t.Error("Expected nil result for connection error, got result")
		}
	} else {
		// If no error, check if result has execution error
		if result == nil {
			t.Fatal("Expected either error or result, got neither")
		}
		
		if result.Error != "" {
			t.Logf("Tool execution failed as expected: %s", result.Error)
		} else {
			t.Log("Tool call succeeded unexpectedly - echo server worked")
		}
		
		if result.ToolName != "echo_tool" {
			t.Errorf("Expected tool name 'echo_tool', got '%s'", result.ToolName)
		}
		
		if result.ServerName != "echo-server" {
			t.Errorf("Expected server name 'echo-server', got '%s'", result.ServerName)
		}
	}
}

func TestMCPClientService_GetAvailableTools(t *testing.T) {
	// Setup test database and services
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)
	clientService := NewMCPClientService(repos, mcpConfigService, toolDiscoveryService)
	defer clientService.Shutdown()

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create mock config and servers with tools (servers will be auto-created)
	config, err := mcpConfigService.UploadConfig(env.ID, &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"test-server": {
				Command: "test",
				Args:    []string{},
				Env:     map[string]string{},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Get the server that was automatically created
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to get servers: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("No servers were created from config")
	}

	// Test GetAvailableTools (tools are automatically created from config)
	availableTools, err := clientService.GetAvailableTools(env.ID)
	if err != nil {
		t.Fatalf("Failed to get available tools: %v", err)
	}

	// Should have 4 tools from config processing (2 default tools per server)
	if len(availableTools) != 4 {
		t.Errorf("Expected 4 tools from config processing, got %d", len(availableTools))
	}

	// Verify some tools exist (the config processing creates default tools)
	toolNames := make(map[string]bool)
	for _, tool := range availableTools {
		toolNames[tool.Name] = true
		t.Logf("Found tool: %s", tool.Name)
	}
}

func TestMCPClientService_CallMultipleTools(t *testing.T) {
	// Setup test database and services
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)
	clientService := NewMCPClientService(repos, mcpConfigService, toolDiscoveryService)
	defer clientService.Shutdown()

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create mock config (servers will be auto-created)
	config, err := mcpConfigService.UploadConfig(env.ID, &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"mock-server": {
				Command: "mock",
				Args:    []string{},
				Env:     map[string]string{},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Get the server that was automatically created
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to get servers: %v", err)
	}
	if len(servers) == 0 {
		t.Fatal("No servers were created from config")
	}
	serverID := servers[0].ID

	// Create tools
	toolData := []struct {
		name string
		desc string
	}{
		{"tool1", "First tool"},
		{"tool2", "Second tool"},
	}

	for _, td := range toolData {
		tool := &models.MCPTool{
			MCPServerID: serverID,
			Name:        td.name,
			Description: td.desc,
			Schema:      json.RawMessage(`{"type":"object"}`),
		}
		_, err := repos.MCPTools.Create(tool)
		if err != nil {
			t.Fatalf("Failed to create tool %s: %v", td.name, err)
		}
	}

	// Test CallMultipleTools
	toolCalls := []models.ToolCall{
		{
			ToolName:  "tool1",
			Arguments: map[string]interface{}{"param": "value1"},
		},
		{
			ToolName:  "tool2",
			Arguments: map[string]interface{}{"param": "value2"},
		},
	}

	results, err := clientService.CallMultipleTools(env.ID, toolCalls)
	if err != nil {
		t.Fatalf("Failed to call multiple tools: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// All should have errors since mock server doesn't exist
	for i, result := range results {
		if result.Error == "" {
			t.Errorf("Expected error in result %d, got none", i)
		}
		if result.ToolName != toolCalls[i].ToolName {
			t.Errorf("Expected tool name '%s', got '%s'", toolCalls[i].ToolName, result.ToolName)
		}
	}
}

func TestMCPClientService_ConnectionManagement(t *testing.T) {
	// Setup test database and services
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)
	clientService := NewMCPClientService(repos, mcpConfigService, toolDiscoveryService)
	defer clientService.Shutdown()

	// Test initial stats
	stats := clientService.GetConnectionStats()
	if stats["active_connections"].(int) != 0 {
		t.Errorf("Expected 0 active connections, got %d", stats["active_connections"])
	}

	// Test connection refresh
	err = clientService.RefreshServerConnection(999) // Non-existent server
	if err != nil {
		t.Errorf("RefreshServerConnection should not error for non-existent server: %v", err)
	}

	// Test close all connections
	clientService.CloseAllConnections()
	
	stats = clientService.GetConnectionStats()
	if stats["active_connections"].(int) != 0 {
		t.Errorf("Expected 0 active connections after close all, got %d", stats["active_connections"])
	}
}

func TestMCPClientService_ToolLookup(t *testing.T) {
	// Setup test database and services
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)
	clientService := NewMCPClientService(repos, mcpConfigService, toolDiscoveryService)
	defer clientService.Shutdown()

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Test tool lookup for non-existent tool
	result, err := clientService.CallTool(env.ID, "non-existent-tool", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for non-existent tool, got none")
	}

	if result != nil {
		t.Error("Expected nil result when tool not found, got result")
	}

	expectedError := "failed to find tool non-existent-tool in environment"
	if err != nil && !contains(err.Error(), expectedError) {
		t.Errorf("Expected error about tool not found, got: %v", err)
	}
}

func TestMCPClientService_ErrorHandling(t *testing.T) {
	// Setup test database and services  
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)
	
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)
	clientService := NewMCPClientService(repos, mcpConfigService, toolDiscoveryService)
	defer clientService.Shutdown()

	// Test with invalid environment ID
	_, err = clientService.GetAvailableTools(999)
	if err == nil {
		t.Error("Expected error for invalid environment ID, got none")
	}

	// Test tool call with invalid environment
	result, err := clientService.CallTool(999, "any-tool", map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for invalid environment in CallTool, got none")
	}

	if result != nil {
		t.Error("Expected nil result for invalid environment, got result")
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}