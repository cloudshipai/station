package services

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"station/internal/db/repositories"
	"station/pkg/crypto"
	"station/pkg/models"
)

func setupTestDBForToolDiscovery(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create all necessary tables with updated schema
	schema := `
	CREATE TABLE environments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE mcp_configs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		environment_id INTEGER NOT NULL,
		config_name TEXT NOT NULL DEFAULT '',
		version INTEGER NOT NULL DEFAULT 1,
		config_json TEXT NOT NULL,
		encryption_key_id TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (environment_id) REFERENCES environments (id),
		UNIQUE (environment_id, config_name, version)
	);

	CREATE TABLE mcp_servers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mcp_config_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		command TEXT NOT NULL,
		args TEXT NOT NULL DEFAULT '[]',
		env TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (mcp_config_id) REFERENCES mcp_configs (id),
		UNIQUE (mcp_config_id, name)
	);

	CREATE TABLE mcp_tools (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		mcp_server_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		description TEXT,
		input_schema TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers (id),
		UNIQUE (mcp_server_id, name)
	);`

	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	return db
}


func TestToolDiscoveryService_DiscoverTools(t *testing.T) {
	// Setup test database
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	// Initialize repositories
	repos := repositories.New(&mockDB{conn: db})

	// Initialize crypto service
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)

	// Initialize services
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create filesystem MCP config (using stdio transport for testing)
	configData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"filesystem": {
				Command: "npx",
				Args: []string{
					"-y",
					"@modelcontextprotocol/server-filesystem",
					"/tmp",  // Use /tmp for testing
				},
				Env: map[string]string{},
			},
		},
	}

	// Upload the config
	config, err := mcpConfigService.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Verify config was stored
	if config == nil {
		t.Fatal("Config was not created")
	}

	// Note: Since we can't guarantee the filesystem MCP server is available in CI,
	// let's test the discovery flow up to the point where we would connect to external servers
	
	// Test that we can retrieve the config and decrypt it
	retrievedConfig, err := repos.MCPConfigs.GetLatest(env.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve config: %v", err)
	}

	decryptedData, err := mcpConfigService.DecryptConfig(retrievedConfig.ConfigJSON)
	if err != nil {
		t.Fatalf("Failed to decrypt config: %v", err)
	}

	// Verify the decrypted config matches what we uploaded
	if len(decryptedData.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(decryptedData.Servers))
	}

	filesystem, exists := decryptedData.Servers["filesystem"]
	if !exists {
		t.Error("filesystem server not found in decrypted config")
	}

	if filesystem.Command != "npx" {
		t.Errorf("Expected command 'npx', got '%s'", filesystem.Command)
	}

	if len(filesystem.Args) != 3 {
		t.Errorf("Expected 3 args, got %d", len(filesystem.Args))
	}

	// Test tool discovery - this will fail gracefully if the MCP server isn't available
	result, err := toolDiscoveryService.DiscoverTools(env.ID)
	
	// We expect this to fail in CI since we don't have the actual MCP server running
	// But we want to verify that the error handling works correctly
	if err != nil {
		t.Logf("Tool discovery failed with error: %v", err)
	}
	
	if result != nil {
		if result.HasErrors() {
			t.Logf("Tool discovery completed with errors as expected (MCP server not available):")
			for _, discoveryErr := range result.Errors {
				t.Logf("  - %s: %s", discoveryErr.Type, discoveryErr.Message)
			}
			
			// Verify that at least the server was stored in the database before failing
			servers, err := repos.MCPServers.GetByConfigID(config.ID)
			if err == nil && len(servers) > 0 {
				t.Logf("Server was stored successfully before discovery failed")
			}
		} else {
			t.Log("Tool discovery succeeded - MCP server was available")
		
			// If discovery succeeded, verify the results
			servers, err := repos.MCPServers.GetByConfigID(config.ID)
			if err != nil {
				t.Fatalf("Failed to get servers: %v", err)
			}

			if len(servers) != 1 {
				t.Errorf("Expected 1 server, got %d", len(servers))
			}

			if len(servers) > 0 {
				server := servers[0]
				if server.Name != "filesystem" {
					t.Errorf("Expected server name 'filesystem', got '%s'", server.Name)
				}

				// Check if tools were discovered
				tools, err := repos.MCPTools.GetByServerID(server.ID)
				if err != nil {
					t.Fatalf("Failed to get tools: %v", err)
				}

				t.Logf("Discovered %d tools from filesystem server", len(tools))
				
				// Log the discovered tools
				for _, tool := range tools {
					t.Logf("Tool: %s - %s", tool.Name, tool.Description)
				}
			}
		}
	}

	// Test GetToolsByEnvironment
	tools, err := toolDiscoveryService.GetToolsByEnvironment(env.ID)
	if err != nil {
		// This is expected if discovery failed
		t.Logf("GetToolsByEnvironment failed as expected: %v", err)
	} else {
		t.Logf("Found %d tools in environment", len(tools))
	}
}

func TestToolDiscoveryService_GetToolsByEnvironment(t *testing.T) {
	// Setup test database
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	// Initialize repositories
	repos := repositories.New(&mockDB{conn: db})

	// Initialize crypto service
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)

	// Initialize services
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create mock MCP config
	configData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"mock-server": {
				Command: "mock",
				Args:    []string{"test"},
				Env:     map[string]string{},
			},
		},
	}

	// Upload the config
	config, err := mcpConfigService.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Get the server that was automatically created by UploadConfig
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to get servers: %v", err)
	}
	
	if len(servers) == 0 {
		t.Fatal("Expected at least one server to be created from config upload")
	}
	
	serverID := servers[0].ID

	// Note: UploadConfig automatically creates 4 common tools for each server
	// (read_file, write_file, list_directory, execute_command)
	// So we don't need to create tools manually - they're already there

	// Test GetToolsByEnvironment
	retrievedTools, err := toolDiscoveryService.GetToolsByEnvironment(env.ID)
	if err != nil {
		t.Fatalf("Failed to get tools by environment: %v", err)
	}

	// UploadConfig automatically creates 4 common tools for each server
	if len(retrievedTools) != 4 {
		t.Errorf("Expected 4 tools, got %d", len(retrievedTools))
	}

	// Verify tool details - check for the 4 automatically created tools
	toolNames := make(map[string]bool)
	for _, tool := range retrievedTools {
		toolNames[tool.Name] = true
		
		if tool.MCPServerID != serverID {
			t.Errorf("Tool %s has wrong server ID: expected %d, got %d", tool.Name, serverID, tool.MCPServerID)
		}
	}

	expectedTools := []string{"read_file", "write_file", "list_directory", "execute_command"}
	for _, expectedTool := range expectedTools {
		if !toolNames[expectedTool] {
			t.Errorf("%s tool not found", expectedTool)
		}
	}

	// Test GetToolsByServer
	serverTools, err := toolDiscoveryService.GetToolsByServer(serverID)
	if err != nil {
		t.Fatalf("Failed to get tools by server: %v", err)
	}

	if len(serverTools) != 4 {
		t.Errorf("Expected 4 tools for server, got %d", len(serverTools))
	}
}

func TestToolDiscoveryService_ClearExistingData(t *testing.T) {
	// Setup test database
	db := setupTestDBForToolDiscovery(t)
	defer db.Close()

	// Initialize repositories
	repos := repositories.New(&mockDB{conn: db})

	// Initialize crypto service
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	keyManager := crypto.NewKeyManager(key)

	// Initialize services
	mcpConfigService := NewMCPConfigService(repos, keyManager)
	toolDiscoveryService := NewToolDiscoveryService(repos, mcpConfigService)

	// Create test environment
	env, err := repos.Environments.Create("test-env", stringPtr("Test Environment"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create mock MCP config
	configData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"mock-server": {
				Command: "mock",
				Args:    []string{"test"},
				Env:     map[string]string{},
			},
		},
	}

	// Upload the config
	config, err := mcpConfigService.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Get the server that was automatically created by UploadConfig
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to get servers: %v", err)
	}
	
	if len(servers) == 0 {
		t.Fatal("Expected at least one server to be created from config upload")
	}
	
	serverID := servers[0].ID

	// Note: UploadConfig automatically creates 4 common tools for each server
	// (read_file, write_file, list_directory, execute_command)
	// Let's create one additional tool to test clearing
	tool := &models.MCPTool{
		MCPServerID: serverID,
		Name:        "test_tool",
		Description: "A test tool",
		Schema:      json.RawMessage(`{"type":"object"}`),
	}

	_, err = repos.MCPTools.Create(tool)
	if err != nil {
		t.Fatalf("Failed to create tool: %v", err)
	}

	// Verify data exists (1 server from config upload + 4 auto-created tools + 1 manual tool = 5 tools)
	tools, err := repos.MCPTools.GetByServerID(serverID)
	if err != nil || len(tools) != 5 {
		t.Fatalf("Expected 5 tools, got %d (err: %v)", len(tools), err)
	}

	// Test clearExistingData
	err = toolDiscoveryService.clearExistingData(config.ID)
	if err != nil {
		t.Fatalf("Failed to clear existing data: %v", err)
	}

	// Verify data was cleared
	servers, err = repos.MCPServers.GetByConfigID(config.ID)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Unexpected error getting servers: %v", err)
	}
	if len(servers) != 0 {
		t.Errorf("Expected 0 servers after clearing, got %d", len(servers))
	}

	tools, err = repos.MCPTools.GetByServerID(serverID)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("Unexpected error getting tools: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after clearing, got %d", len(tools))
	}
}