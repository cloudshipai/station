package services

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"station/internal/db/repositories"
	"station/pkg/crypto"
	"station/pkg/models"
)

func setupTestDBForServices(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create all necessary tables
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

func TestMCPConfigService_UploadConfig(t *testing.T) {
	db := setupTestDBForServices(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	keyManager := crypto.NewKeyManager(key)
	service := NewMCPConfigService(repos, keyManager)

	// Create an environment first
	env, err := repos.Environments.Create("test-env", nil)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Upload a config
	configData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"test-server": {
				Command: "python",
				Args:    []string{"-m", "test_server"},
				Env:     map[string]string{"TEST": "value"},
			},
		},
	}

	config, err := service.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	if config.ID == 0 {
		t.Error("Expected config ID to be set")
	}

	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}

	if config.EnvironmentID != env.ID {
		t.Errorf("Expected environment ID %d, got %d", env.ID, config.EnvironmentID)
	}

	// Verify servers were created
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to list servers: %v", err)
	}

	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	if servers[0].Name != "test-server" {
		t.Errorf("Expected server name 'test-server', got '%s'", servers[0].Name)
	}
}

func TestMCPConfigService_GetDecryptedConfig(t *testing.T) {
	db := setupTestDBForServices(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	keyManager := crypto.NewKeyManager(key)
	service := NewMCPConfigService(repos, keyManager)

	// Create environment and upload config
	env, err := repos.Environments.Create("test-env", nil)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	originalConfigData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"test-server": {
				Command: "python",
				Args:    []string{"-m", "test_server"},
				Env:     map[string]string{"TEST": "value"},
			},
		},
	}

	config, err := service.UploadConfig(env.ID, originalConfigData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Retrieve and decrypt the config
	decryptedConfig, err := service.GetDecryptedConfig(config.ID)
	if err != nil {
		t.Fatalf("Failed to get decrypted config: %v", err)
	}

	// Verify the decrypted config matches the original
	if len(decryptedConfig.Servers) != 1 {
		t.Errorf("Expected 1 server in decrypted config, got %d", len(decryptedConfig.Servers))
	}

	testServer, exists := decryptedConfig.Servers["test-server"]
	if !exists {
		t.Error("Expected 'test-server' to exist in decrypted config")
	}

	if testServer.Command != "python" {
		t.Errorf("Expected command 'python', got '%s'", testServer.Command)
	}

	if len(testServer.Args) != 2 || testServer.Args[0] != "-m" || testServer.Args[1] != "test_server" {
		t.Errorf("Expected args ['-m', 'test_server'], got %v", testServer.Args)
	}

	if testServer.Env["TEST"] != "value" {
		t.Errorf("Expected env TEST=value, got %v", testServer.Env)
	}
}

func TestMCPConfigService_RotateEncryptionKeys(t *testing.T) {
	db := setupTestDBForServices(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	keyManager := crypto.NewKeyManager(key)
	service := NewMCPConfigService(repos, keyManager)

	// Create environment and upload config
	env, err := repos.Environments.Create("test-env", nil)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	configData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"test-server": {
				Command: "python",
				Args:    []string{"-m", "test_server"},
			},
		},
	}

	originalKeyID := keyManager.GetActiveKey().ID

	config, err := service.UploadConfig(env.ID, configData)
	if err != nil {
		t.Fatalf("Failed to upload config: %v", err)
	}

	// Rotate encryption keys
	err = service.RotateEncryptionKeys()
	if err != nil {
		t.Fatalf("Failed to rotate encryption keys: %v", err)
	}

	// Verify the key changed
	newKeyID := keyManager.GetActiveKey().ID
	if newKeyID == originalKeyID {
		t.Error("Expected active key ID to change after rotation")
	}

	// Verify we can still decrypt the config
	decryptedConfig, err := service.GetDecryptedConfig(config.ID)
	if err != nil {
		t.Fatalf("Failed to decrypt config with new key: %v", err)
	}

	// Verify the decrypted data is still correct
	if len(decryptedConfig.Servers) != 1 {
		t.Errorf("Expected 1 server after key rotation, got %d", len(decryptedConfig.Servers))
	}

	// Verify the config record was updated with the new key ID
	updatedConfig, err := repos.MCPConfigs.GetByID(config.ID)
	if err != nil {
		t.Fatalf("Failed to get updated config: %v", err)
	}

	if updatedConfig.EncryptionKeyID == originalKeyID {
		t.Error("Expected config encryption key ID to be updated after rotation")
	}
}

func TestMCPConfigService_GetKeyManagerStats(t *testing.T) {
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	keyManager := crypto.NewKeyManager(key)
	service := NewMCPConfigService(nil, keyManager)

	// Get initial stats
	stats := service.GetKeyManagerStats()

	totalKeys, ok := stats["total_keys"].(int)
	if !ok || totalKeys != 1 {
		t.Errorf("Expected 1 total key, got %v", stats["total_keys"])
	}

	activeKeyID, ok := stats["active_key_id"].(string)
	if !ok || activeKeyID == "" {
		t.Errorf("Expected active key ID to be set, got %v", stats["active_key_id"])
	}

	// Rotate key and check stats again
	_, err = keyManager.RotateKey()
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	newStats := service.GetKeyManagerStats()

	newTotalKeys, ok := newStats["total_keys"].(int)
	if !ok || newTotalKeys != 2 {
		t.Errorf("Expected 2 total keys after rotation, got %v", newStats["total_keys"])
	}

	newActiveKeyID, ok := newStats["active_key_id"].(string)
	if !ok || newActiveKeyID == activeKeyID {
		t.Error("Expected active key ID to change after rotation")
	}
}

func TestMCPConfigService_FilesystemConfig(t *testing.T) {
	db := setupTestDBForServices(t)
	defer db.Close()

	repos := repositories.New(&mockDB{conn: db})
	
	key, err := crypto.GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	keyManager := crypto.NewKeyManager(key)
	service := NewMCPConfigService(repos, keyManager)

	// Create environment for filesystem config
	env, err := repos.Environments.Create("filesystem-env", stringPtr("Environment for filesystem MCP server"))
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create the filesystem MCP config as provided by the user
	filesystemConfigData := &models.MCPConfigData{
		Servers: map[string]models.MCPServerConfig{
			"filesystem": {
				Command: "npx",
				Args: []string{
					"-y",
					"@modelcontextprotocol/server-filesystem",
					"/Users/username/Desktop",
					"/path/to/other/allowed/dir",
				},
				Env: map[string]string{},
			},
		},
	}

	// Upload the filesystem config
	config, err := service.UploadConfig(env.ID, filesystemConfigData)
	if err != nil {
		t.Fatalf("Failed to upload filesystem config: %v", err)
	}

	if config.ID == 0 {
		t.Error("Expected config ID to be set")
	}

	if config.Version != 1 {
		t.Errorf("Expected version 1, got %d", config.Version)
	}

	if config.EnvironmentID != env.ID {
		t.Errorf("Expected environment ID %d, got %d", env.ID, config.EnvironmentID)
	}

	// Verify the filesystem server was created
	servers, err := repos.MCPServers.GetByConfigID(config.ID)
	if err != nil {
		t.Fatalf("Failed to list servers: %v", err)
	}

	if len(servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(servers))
	}

	if servers[0].Name != "filesystem" {
		t.Errorf("Expected server name 'filesystem', got '%s'", servers[0].Name)
	}

	// Retrieve and verify the decrypted config
	decryptedConfig, err := service.GetDecryptedConfig(config.ID)
	if err != nil {
		t.Fatalf("Failed to get decrypted filesystem config: %v", err)
	}

	// Verify the filesystem server configuration
	filesystemServer, exists := decryptedConfig.Servers["filesystem"]
	if !exists {
		t.Error("Expected 'filesystem' server to exist in decrypted config")
	}

	if filesystemServer.Command != "npx" {
		t.Errorf("Expected command 'npx', got '%s'", filesystemServer.Command)
	}

	expectedArgs := []string{
		"-y",
		"@modelcontextprotocol/server-filesystem",
		"/Users/username/Desktop",
		"/path/to/other/allowed/dir",
	}

	if len(filesystemServer.Args) != len(expectedArgs) {
		t.Errorf("Expected %d args, got %d", len(expectedArgs), len(filesystemServer.Args))
	}

	for i, expectedArg := range expectedArgs {
		if i >= len(filesystemServer.Args) || filesystemServer.Args[i] != expectedArg {
			t.Errorf("Expected arg[%d] to be '%s', got '%s'", i, expectedArg, filesystemServer.Args[i])
		}
	}

	t.Logf("Successfully stored and retrieved filesystem MCP config:")
	t.Logf("  Environment: %s", env.Name)
	t.Logf("  Config ID: %d", config.ID)
	t.Logf("  Server: %s", filesystemServer.Command)
	t.Logf("  Args: %v", filesystemServer.Args)
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}

// mockDB is a simple wrapper to implement the DB interface for tests
type mockDB struct {
	conn *sql.DB
}

func (m *mockDB) Conn() *sql.DB {
	return m.conn
}

func (m *mockDB) Close() error {
	return m.conn.Close()
}

func (m *mockDB) Migrate() error {
	return nil
}

func (m *mockDB) MigrateFromDir(dir string) error {
	return nil
}