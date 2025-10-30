package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewDeclarativeSync tests service creation
func TestNewDeclarativeSync(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{
		Workspace: t.TempDir(),
	}

	tests := []struct {
		name        string
		repos       *repositories.Repositories
		config      *config.Config
		expectNil   bool
		description string
	}{
		{
			name:        "Valid service creation",
			repos:       repos,
			config:      cfg,
			expectNil:   false,
			description: "Should create service with valid dependencies",
		},
		{
			name:        "Nil repositories",
			repos:       nil,
			config:      cfg,
			expectNil:   false,
			description: "Should still create service (may panic on use)",
		},
		{
			name:        "Nil config",
			repos:       repos,
			config:      nil,
			expectNil:   false,
			description: "Should still create service (will use XDG fallback)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDeclarativeSync(tt.repos, tt.config)

			if tt.expectNil {
				if service != nil {
					t.Errorf("Expected nil service, got %v", service)
				}
			} else {
				if service == nil {
					t.Error("Expected non-nil service")
				} else {
					if service.repos != tt.repos {
						t.Error("Service repos not initialized correctly")
					}
					if service.config != tt.config {
						t.Error("Service config not initialized correctly")
					}
				}
			}
		})
	}
}

// TestDeclarativeSyncSetVariableResolver tests custom variable resolver injection
func TestDeclarativeSyncSetVariableResolver(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: t.TempDir()}
	service := NewDeclarativeSync(repos, cfg)

	// Mock resolver function (VariableResolver is a function type)
	mockResolver := func(missingVars []VariableInfo) (map[string]string, error) {
		return map[string]string{"test": "value"}, nil
	}

	service.SetVariableResolver(mockResolver)

	if service.customVariableResolver == nil {
		t.Error("Variable resolver not set")
	}
}

// TestSyncEnvironment tests environment synchronization
func TestSyncEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip if OPENAI_API_KEY is not set or is a dummy test key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" || apiKey == "sk-test-dummy-key-for-ci" {
		t.Skip("Skipping test: requires valid OPENAI_API_KEY for GenKit initialization")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	workspace := t.TempDir()
	cfg := &config.Config{Workspace: workspace}
	service := NewDeclarativeSync(repos, cfg)

	// Create test environment in database
	env, err := repos.Environments.Create("test-sync-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create environment directory structure
	envDir := filepath.Join(workspace, "environments", env.Name)
	agentsDir := filepath.Join(envDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name             string
		environmentName  string
		setupFunc        func()
		wantErr          bool
		expectAgents     int
		expectMCPServers int
		description      string
	}{
		{
			name:             "Sync empty environment",
			environmentName:  env.Name,
			setupFunc:        func() {},
			wantErr:          false,
			expectAgents:     0,
			expectMCPServers: 0,
			description:      "Should handle empty environment without errors",
		},
		{
			name:            "Sync non-existent environment",
			environmentName: "nonexistent-env",
			setupFunc:       func() {},
			wantErr:         true,
			description:     "Should fail for non-existent environment",
		},
		{
			name:            "Sync environment with template.json",
			environmentName: env.Name,
			setupFunc: func() {
				templateJSON := `{
  "name": "test-template",
  "description": "Test template",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest", "/workspace"]
    }
  }
}`
				templatePath := filepath.Join(envDir, "template.json")
				if err := os.WriteFile(templatePath, []byte(templateJSON), 0644); err != nil {
					t.Fatalf("Failed to write template.json: %v", err)
				}
			},
			wantErr:          false,
			expectMCPServers: 1,
			description:      "Should process template.json with MCP servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			options := SyncOptions{
				DryRun:   false,
				Validate: true,
				Verbose:  true,
			}

			result, err := service.SyncEnvironment(ctx, tt.environmentName, options)

			if (err != nil) != tt.wantErr {
				t.Errorf("SyncEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("SyncEnvironment() returned nil result")
					return
				}

				if result.Environment != tt.environmentName {
					t.Errorf("Result environment = %s, want %s", result.Environment, tt.environmentName)
				}

				if result.Duration == 0 {
					t.Error("Result duration should be non-zero")
				}

				t.Logf("Sync result: %d agents processed, %d MCP servers processed, %d errors",
					result.AgentsProcessed, result.MCPServersProcessed, result.ValidationErrors)
			}
		})
	}
}

// TestSyncOptions tests sync option structure
func TestSyncOptions(t *testing.T) {
	tests := []struct {
		name        string
		options     SyncOptions
		description string
	}{
		{
			name: "Default options",
			options: SyncOptions{
				DryRun:      false,
				Validate:    false,
				Force:       false,
				Verbose:     false,
				Interactive: false,
				Confirm:     false,
			},
			description: "All options disabled",
		},
		{
			name: "DryRun enabled",
			options: SyncOptions{
				DryRun:   true,
				Validate: true,
			},
			description: "DryRun mode with validation",
		},
		{
			name: "Force sync",
			options: SyncOptions{
				Force:   true,
				Verbose: true,
			},
			description: "Force sync with verbose output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.options.DryRun && !tt.options.Validate {
				t.Log("Note: DryRun mode typically requires Validate=true")
			}
		})
	}
}

// TestSyncResult tests sync result structure
func TestSyncResult(t *testing.T) {
	t.Run("Create empty result", func(t *testing.T) {
		result := &SyncResult{}

		if result.AgentsProcessed != 0 {
			t.Error("Empty result should have zero agents processed")
		}
		if result.ValidationErrors != 0 {
			t.Error("Empty result should have zero validation errors")
		}
		if result.Duration != 0 {
			t.Error("Empty result should have zero duration")
		}
	})

	t.Run("Create populated result", func(t *testing.T) {
		result := &SyncResult{
			Environment:         "test-env",
			AgentsProcessed:     5,
			AgentsSynced:        4,
			AgentsSkipped:       1,
			ValidationErrors:    0,
			MCPServersProcessed: 2,
			MCPServersConnected: 2,
			Duration:            time.Second * 5,
			Operations: []SyncOperation{
				{Type: OpTypeCreate, Target: "agent1", Description: "Created agent1"},
				{Type: OpTypeUpdate, Target: "agent2", Description: "Updated agent2"},
			},
		}

		if result.Environment != "test-env" {
			t.Errorf("Environment = %s, want test-env", result.Environment)
		}
		if result.AgentsProcessed != 5 {
			t.Errorf("AgentsProcessed = %d, want 5", result.AgentsProcessed)
		}
		if len(result.Operations) != 2 {
			t.Errorf("Operations count = %d, want 2", len(result.Operations))
		}
	})
}

// TestSyncOperation tests sync operation structure
func TestSyncOperation(t *testing.T) {
	tests := []struct {
		name      string
		operation SyncOperation
	}{
		{
			name: "Create operation",
			operation: SyncOperation{
				Type:        OpTypeCreate,
				Target:      "test-agent",
				Description: "Creating new agent",
			},
		},
		{
			name: "Update operation",
			operation: SyncOperation{
				Type:        OpTypeUpdate,
				Target:      "existing-agent",
				Description: "Updating agent configuration",
			},
		},
		{
			name: "Delete operation",
			operation: SyncOperation{
				Type:        OpTypeDelete,
				Target:      "old-agent",
				Description: "Deleting orphaned agent",
			},
		},
		{
			name: "Error operation",
			operation: SyncOperation{
				Type:        OpTypeError,
				Target:      "failed-agent",
				Description: "Failed to sync",
				Error:       os.ErrNotExist,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.operation.Type == "" {
				t.Error("Operation type should not be empty")
			}
			if tt.operation.Target == "" {
				t.Error("Operation target should not be empty")
			}
			if tt.operation.Type == OpTypeError && tt.operation.Error == nil {
				t.Log("Note: Error operation typically has Error field set")
			}
		})
	}
}

// TestSyncOperationType tests operation type constants
func TestSyncOperationType(t *testing.T) {
	tests := []struct {
		name        string
		opType      SyncOperationType
		expectValue string
	}{
		{"Create operation", OpTypeCreate, "create"},
		{"Update operation", OpTypeUpdate, "update"},
		{"Delete operation", OpTypeDelete, "delete"},
		{"Skip operation", OpTypeSkip, "skip"},
		{"Validate operation", OpTypeValidate, "validate"},
		{"Error operation", OpTypeError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.opType) != tt.expectValue {
				t.Errorf("OpType = %s, want %s", tt.opType, tt.expectValue)
			}
		})
	}
}

// TestValidateMCPDependencies tests MCP dependency validation
func TestValidateMCPDependencies(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: t.TempDir()}
	service := NewDeclarativeSync(repos, cfg)

	// Currently a no-op, should always return nil
	err = service.validateMCPDependencies("test-env")
	if err != nil {
		t.Errorf("validateMCPDependencies() error = %v, want nil", err)
	}
}

// TestSyncMCPTemplateFiles tests MCP template file synchronization
func TestSyncMCPTemplateFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	workspace := t.TempDir()
	cfg := &config.Config{Workspace: workspace}
	service := NewDeclarativeSync(repos, cfg)

	// Create environment in database
	env, err := repos.Environments.Create("test-mcp-sync", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	envDir := filepath.Join(workspace, "environments", env.Name)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create env directory: %v", err)
	}

	ctx := context.Background()
	options := SyncOptions{Verbose: true}

	tests := []struct {
		name               string
		setupFunc          func()
		wantErr            bool
		expectServersCount int
		description        string
	}{
		{
			name:               "Empty environment directory",
			setupFunc:          func() {},
			wantErr:            false,
			expectServersCount: 0,
			description:        "Should handle environment with no JSON files",
		},
		{
			name: "Environment with template.json",
			setupFunc: func() {
				templateJSON := `{
  "name": "test-template",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem@latest"]
    }
  }
}`
				if err := os.WriteFile(filepath.Join(envDir, "template.json"), []byte(templateJSON), 0644); err != nil {
					t.Fatalf("Failed to write template.json: %v", err)
				}
			},
			wantErr:            false,
			expectServersCount: 1,
			description:        "Should process template.json file",
		},
		{
			name: "Non-existent directory",
			setupFunc: func() {
				// Don't create directory
			},
			wantErr:            false,
			expectServersCount: 0,
			description:        "Should gracefully handle missing directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment directory
			os.RemoveAll(envDir)

			if tt.setupFunc != nil {
				// Only create directory if test needs it
				if tt.name != "Non-existent directory" {
					os.MkdirAll(envDir, 0755)
				}
				tt.setupFunc()
			}

			result, err := service.syncMCPTemplateFiles(ctx, envDir, env.Name, options)

			if (err != nil) != tt.wantErr {
				t.Errorf("syncMCPTemplateFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				t.Logf("MCP template sync: %d servers processed, %d connected",
					result.MCPServersProcessed, result.MCPServersConnected)
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewDeclarativeSync(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: b.TempDir()}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewDeclarativeSync(repos, cfg)
	}
}

func TestNormalizeConfigPath(t *testing.T) {
	service := &DeclarativeSync{}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "Path with environments in the middle",
			path: "/home/user/.config/station/environments/default/config.json",
			want: "environments/default/config.json",
		},
		{
			name: "Different home directory",
			path: "/root/.config/station/environments/prod/settings.json",
			want: "environments/prod/settings.json",
		},
		{
			name: "Path without environments",
			path: "/some/other/path/file.json",
			want: "/some/other/path/file.json",
		},
		{
			name: "Empty path",
			path: "",
			want: "",
		},
		{
			name: "Only environments directory",
			path: "environments/",
			want: "environments/",
		},
		{
			name: "Environments at start",
			path: "environments/default/config.json",
			want: "environments/default/config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.normalizeConfigPath(tt.path)
			if got != tt.want {
				t.Errorf("normalizeConfigPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func BenchmarkSyncOptions(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SyncOptions{
			DryRun:   true,
			Validate: true,
			Verbose:  true,
		}
	}
}
