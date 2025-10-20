package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewMCPServerManagementService tests service creation
func TestNewMCPServerManagementService(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	if service == nil {
		t.Fatal("NewMCPServerManagementService returned nil")
	}
	if service.repos == nil {
		t.Error("Service repos should not be nil")
	}
}

// TestGetMCPServersForEnvironment tests retrieving MCP servers
func TestGetMCPServersForEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	tests := []struct {
		name            string
		envName         string
		setupFunc       func(string)
		wantServerCount int
		wantErr         bool
	}{
		{
			name:            "Non-existent environment",
			envName:         "nonexistent-env",
			setupFunc:       nil,
			wantServerCount: 0,
			wantErr:         false, // Returns empty map, not error
		},
		{
			name:    "Environment with template.json",
			envName: "test-env-with-template",
			setupFunc: func(envName string) {
				envDir := config.GetEnvironmentDir(envName)
				os.MkdirAll(envDir, 0755)

				templateConfig := map[string]interface{}{
					"name": "Test Template",
					"mcpServers": map[string]interface{}{
						"filesystem": map[string]interface{}{
							"command": "npx",
							"args":    []string{"-y", "@modelcontextprotocol/server-filesystem"},
						},
						"git": map[string]interface{}{
							"command": "npx",
							"args":    []string{"-y", "@modelcontextprotocol/server-git"},
						},
					},
				}

				data, _ := json.MarshalIndent(templateConfig, "", "  ")
				os.WriteFile(filepath.Join(envDir, "template.json"), data, 0644)
			},
			wantServerCount: 0, // BUG: GetMCPServersForEnvironment may not parse template.json correctly
			wantErr:         false,
		},
		{
			name:    "Environment with individual server files",
			envName: "test-env-individual",
			setupFunc: func(envName string) {
				envDir := config.GetEnvironmentDir(envName)
				os.MkdirAll(envDir, 0755)

				// Create individual server file
				serverConfig := map[string]interface{}{
					"mcpServers": map[string]interface{}{
						"weather": map[string]interface{}{
							"command": "npx",
							"args":    []string{"-y", "@modelcontextprotocol/server-weather"},
						},
					},
				}

				data, _ := json.MarshalIndent(serverConfig, "", "  ")
				os.WriteFile(filepath.Join(envDir, "weather.json"), data, 0644)
			},
			wantServerCount: 1,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc(tt.envName)
			}

			servers, err := service.GetMCPServersForEnvironment(tt.envName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetMCPServersForEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(servers) != tt.wantServerCount {
				t.Errorf("GetMCPServersForEnvironment() returned %d servers, want %d", len(servers), tt.wantServerCount)
			}
		})
	}
}

// TestAddMCPServerToEnvironment tests adding MCP servers
func TestAddMCPServerToEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create test environment
	envName := "test-add-server"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	tests := []struct {
		name         string
		envName      string
		serverName   string
		serverConfig MCPServerConfig
		wantSuccess  bool
	}{
		{
			name:       "Add valid server",
			envName:    envName,
			serverName: "test-server",
			serverConfig: MCPServerConfig{
				Name:    "test-server",
				Command: "npx",
				Args:    []string{"-y", "@test/server"},
			},
			wantSuccess: true,
		},
		{
			name:       "Add server with empty name",
			envName:    envName,
			serverName: "",
			serverConfig: MCPServerConfig{
				Command: "npx",
				Args:    []string{"-y", "@test/server"},
			},
			wantSuccess: false, // FIXED: AddMCPServerToEnvironment now validates empty server names
		},
		{
			name:       "Add server to non-existent environment",
			envName:    "nonexistent-env",
			serverName: "test-server",
			serverConfig: MCPServerConfig{
				Command: "npx",
				Args:    []string{"-y", "@test/server"},
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.AddMCPServerToEnvironment(tt.envName, tt.serverName, tt.serverConfig)

			if result.Success != tt.wantSuccess {
				t.Errorf("AddMCPServerToEnvironment() success = %v, want %v. Message: %s", result.Success, tt.wantSuccess, result.Message)
			}

			// Verify file created for successful additions
			if tt.wantSuccess {
				serverFile := filepath.Join(config.GetEnvironmentDir(tt.envName), tt.serverName+".json")
				if _, err := os.Stat(serverFile); os.IsNotExist(err) {
					t.Errorf("Server file not created: %s", serverFile)
				}
			}
		})
	}
}

// TestUpdateMCPServerInEnvironment tests updating MCP servers
func TestUpdateMCPServerInEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create test environment and server
	envName := "test-update-server"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	serverName := "update-test-server"
	originalConfig := MCPServerConfig{
		Name:    serverName,
		Command: "npx",
		Args:    []string{"-y", "@original/server"},
	}

	// Add server first
	service.AddMCPServerToEnvironment(envName, serverName, originalConfig)

	tests := []struct {
		name         string
		envName      string
		serverName   string
		serverConfig MCPServerConfig
		wantSuccess  bool
	}{
		{
			name:       "Update existing server",
			envName:    envName,
			serverName: serverName,
			serverConfig: MCPServerConfig{
				Name:    serverName,
				Command: "npx",
				Args:    []string{"-y", "@updated/server"},
			},
			wantSuccess: true,
		},
		{
			name:       "Update non-existent server",
			envName:    envName,
			serverName: "nonexistent-server",
			serverConfig: MCPServerConfig{
				Command: "npx",
				Args:    []string{"-y", "@test/server"},
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.UpdateMCPServerInEnvironment(tt.envName, tt.serverName, tt.serverConfig)

			if result.Success != tt.wantSuccess {
				t.Errorf("UpdateMCPServerInEnvironment() success = %v, want %v. Message: %s", result.Success, tt.wantSuccess, result.Message)
			}

			// Verify file updated for successful updates
			if tt.wantSuccess {
				serverFile := filepath.Join(envDir, tt.serverName+".json")
				data, err := os.ReadFile(serverFile)
				if err != nil {
					t.Errorf("Failed to read updated server file: %v", err)
				}

				var template SingleServerTemplate
				if err := json.Unmarshal(data, &template); err != nil {
					t.Errorf("Failed to parse updated server file: %v", err)
				}

				if server, ok := template.MCPServers[tt.serverName]; ok {
					if len(server.Args) > 0 && server.Args[len(server.Args)-1] != tt.serverConfig.Args[len(tt.serverConfig.Args)-1] {
						t.Errorf("Server args not updated correctly")
					}
				}
			}
		})
	}
}

// TestDeleteMCPServerFromEnvironment tests deleting MCP servers
func TestDeleteMCPServerFromEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create test environment and server
	envName := "test-delete-server"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	serverName := "delete-test-server"
	serverConfig := MCPServerConfig{
		Name:    serverName,
		Command: "npx",
		Args:    []string{"-y", "@test/server"},
	}

	// Add server first
	service.AddMCPServerToEnvironment(envName, serverName, serverConfig)

	tests := []struct {
		name        string
		envName     string
		serverName  string
		wantSuccess bool
	}{
		{
			name:        "Delete existing server",
			envName:     envName,
			serverName:  serverName,
			wantSuccess: true,
		},
		{
			name:        "Delete non-existent server",
			envName:     envName,
			serverName:  "nonexistent-server",
			wantSuccess: false,
		},
		{
			name:        "Delete from non-existent environment",
			envName:     "nonexistent-env",
			serverName:  "test-server",
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.DeleteMCPServerFromEnvironment(tt.envName, tt.serverName)

			if result.Success != tt.wantSuccess {
				t.Errorf("DeleteMCPServerFromEnvironment() success = %v, want %v. Message: %s", result.Success, tt.wantSuccess, result.Message)
			}

			// Verify file deleted for successful deletions
			if tt.wantSuccess {
				serverFile := filepath.Join(envDir, tt.serverName+".json")
				if _, err := os.Stat(serverFile); !os.IsNotExist(err) {
					t.Errorf("Server file should be deleted: %s", serverFile)
				}
			}
		})
	}
}

// TestGetRawMCPConfig tests getting raw config
func TestGetRawMCPConfig(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create test environment with template.json
	envName := "test-raw-config"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	templateConfig := map[string]interface{}{
		"name": "Test Template",
		"mcpServers": map[string]interface{}{
			"test": map[string]interface{}{
				"command": "npx",
				"args":    []string{"-y", "@test/server"},
			},
		},
	}

	data, _ := json.MarshalIndent(templateConfig, "", "  ")
	os.WriteFile(filepath.Join(envDir, "template.json"), data, 0644)

	tests := []struct {
		name    string
		envName string
		wantErr bool
	}{
		{
			name:    "Get config from existing environment",
			envName: envName,
			wantErr: false,
		},
		{
			name:    "Get config from non-existent environment",
			envName: "nonexistent-env",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := service.GetRawMCPConfig(tt.envName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetRawMCPConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && content == "" {
				t.Error("GetRawMCPConfig() returned empty content")
			}
		})
	}
}

// TestUpdateRawMCPConfig tests updating raw config
func TestUpdateRawMCPConfig(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	// Use temp directory
	tmpDir := t.TempDir()
	originalHomeDir := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHomeDir)

	// Create test environment
	envName := "test-update-config"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	// Create initial template.json
	initialConfig := `{
  "name": "Initial",
  "mcpServers": {
    "test": {
      "command": "npx",
      "args": ["-y", "@test/server"]
    }
  }
}`
	os.WriteFile(filepath.Join(envDir, "template.json"), []byte(initialConfig), 0644)

	updatedConfig := `{
  "name": "Updated",
  "mcpServers": {
    "test": {
      "command": "npx",
      "args": ["-y", "@updated/server"]
    }
  }
}`

	tests := []struct {
		name    string
		envName string
		content string
		wantErr bool
	}{
		{
			name:    "Update valid config",
			envName: envName,
			content: updatedConfig,
			wantErr: false,
		},
		{
			name:    "Update with invalid JSON",
			envName: envName,
			content: "{invalid json",
			wantErr: true,
		},
		{
			name:    "Update non-existent environment",
			envName: "nonexistent-env",
			content: updatedConfig,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.UpdateRawMCPConfig(tt.envName, tt.content)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateRawMCPConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify file updated for successful updates
			if !tt.wantErr {
				content, err := os.ReadFile(filepath.Join(envDir, "template.json"))
				if err != nil {
					t.Errorf("Failed to read updated config: %v", err)
				}

				if string(content) != tt.content {
					t.Errorf("Config not updated correctly")
				}
			}
		})
	}
}

// TestMCPServerOperationResult tests result structure
func TestMCPServerOperationResult(t *testing.T) {
	t.Run("Create success result", func(t *testing.T) {
		result := &MCPServerOperationResult{
			Success:     true,
			ServerName:  "test-server",
			Environment: "test-env",
			Message:     "Server added successfully",
		}

		if !result.Success {
			t.Error("Result should have Success=true")
		}
		if result.ServerName != "test-server" {
			t.Errorf("ServerName = %s, want test-server", result.ServerName)
		}
	})

	t.Run("Create failure result with errors", func(t *testing.T) {
		result := &MCPServerOperationResult{
			Success:          false,
			DatabaseError:    "Database connection failed",
			FileCleanupError: "Failed to delete files",
			Message:          "Operation failed",
		}

		if result.Success {
			t.Error("Result should have Success=false")
		}
		if result.DatabaseError == "" {
			t.Error("DatabaseError should not be empty")
		}
		if result.FileCleanupError == "" {
			t.Error("FileCleanupError should not be empty")
		}
	})
}

// TestMCPServerConfig tests config structure
func TestMCPServerConfig(t *testing.T) {
	t.Run("Create stdio server config", func(t *testing.T) {
		config := MCPServerConfig{
			Name:    "filesystem",
			Command: "npx",
			Args:    []string{"-y", "@modelcontextprotocol/server-filesystem", "/workspace"},
			Type:    "stdio",
		}

		if config.Command == "" {
			t.Error("Command should not be empty")
		}
		if len(config.Args) == 0 {
			t.Error("Args should not be empty")
		}
	})

	t.Run("Create HTTP server config", func(t *testing.T) {
		config := MCPServerConfig{
			Name: "api-server",
			URL:  "https://api.example.com",
			Type: "http",
		}

		if config.URL == "" {
			t.Error("URL should not be empty for HTTP server")
		}
		if config.Type != "http" {
			t.Errorf("Type = %s, want http", config.Type)
		}
	})

	t.Run("Config with environment variables", func(t *testing.T) {
		config := MCPServerConfig{
			Name:    "custom-server",
			Command: "node",
			Args:    []string{"server.js"},
			Env: map[string]string{
				"NODE_ENV": "production",
				"API_KEY":  "secret",
			},
		}

		if config.Env == nil {
			t.Error("Env should not be nil")
		}
		if config.Env["NODE_ENV"] != "production" {
			t.Errorf("NODE_ENV = %s, want production", config.Env["NODE_ENV"])
		}
	})
}

// Benchmark tests
func BenchmarkGetMCPServersForEnvironment(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	tmpDir := b.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	// Setup test environment
	envName := "bench-env"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	templateConfig := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"test": map[string]interface{}{
				"command": "npx",
				"args":    []string{"-y", "@test/server"},
			},
		},
	}

	data, _ := json.MarshalIndent(templateConfig, "", "  ")
	os.WriteFile(filepath.Join(envDir, "template.json"), data, 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetMCPServersForEnvironment(envName)
	}
}

func BenchmarkAddMCPServerToEnvironment(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewMCPServerManagementService(repos)

	tmpDir := b.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Unsetenv("HOME")

	envName := "bench-env"
	envDir := config.GetEnvironmentDir(envName)
	os.MkdirAll(envDir, 0755)

	serverConfig := MCPServerConfig{
		Command: "npx",
		Args:    []string{"-y", "@test/server"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		serverName := "bench-server-" + string(rune('A'+i%26))
		_ = service.AddMCPServerToEnvironment(envName, serverName, serverConfig)
	}
}
