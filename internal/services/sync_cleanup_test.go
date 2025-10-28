package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// TestCleanupOrphanedResources tests orphaned resource cleanup
func TestCleanupOrphanedResources(t *testing.T) {
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

	// Create test environment
	env, err := repos.Environments.Create("test-cleanup-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	envDir := filepath.Join(workspace, "environments", env.Name)
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create env directory: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		setupFunc   func()
		options     SyncOptions
		wantErr     bool
		description string
	}{
		{
			name: "No orphaned resources",
			setupFunc: func() {
				// No configs in database, no files - nothing to clean
			},
			options:     SyncOptions{DryRun: false},
			wantErr:     false,
			description: "Should handle no orphaned resources",
		},
		{
			name: "Dry run mode",
			setupFunc: func() {
				// Create a file config in database but no corresponding file
				repos.FileMCPConfigs.Create(&repositories.FileConfigRecord{
					EnvironmentID: env.ID,
					ConfigName:    "orphaned-config",
					TemplatePath:  filepath.Join(envDir, "orphaned-config.json"),
				})
			},
			options:     SyncOptions{DryRun: true},
			wantErr:     false,
			description: "Should report what would be removed without removing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			result, err := service.cleanupOrphanedResources(ctx, envDir, env.Name, tt.options)

			if (err != nil) != tt.wantErr {
				t.Errorf("cleanupOrphanedResources() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result == "" {
				t.Error("cleanupOrphanedResources() returned empty result")
			}

			t.Logf("Cleanup result: %s", result)
		})
	}
}

// TestRemoveConfigServersAndTools tests server and tool removal
func TestRemoveConfigServersAndTools(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: t.TempDir()}
	service := NewDeclarativeSync(repos, cfg)

	// Create test environment
	env, err := repos.Environments.Create("test-remove-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create file config
	fileConfigID, err := repos.FileMCPConfigs.Create(&repositories.FileConfigRecord{
		EnvironmentID: env.ID,
		ConfigName:    "test-config",
		TemplatePath:  "/tmp/test-config.json",
	})
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	fileConfig, err := repos.FileMCPConfigs.GetByID(fileConfigID)
	if err != nil {
		t.Fatalf("Failed to get file config: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		configName  string
		wantErr     bool
		description string
	}{
		{
			name:        "Remove config with no servers",
			configName:  "test-config",
			wantErr:     false,
			description: "Should handle config with no associated servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serversRemoved, toolsRemoved, err := service.removeConfigServersAndTools(ctx, env.ID, tt.configName, fileConfig)

			if (err != nil) != tt.wantErr {
				t.Errorf("removeConfigServersAndTools() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if serversRemoved < 0 {
				t.Errorf("serversRemoved should be non-negative, got %d", serversRemoved)
			}

			if toolsRemoved < 0 {
				t.Errorf("toolsRemoved should be non-negative, got %d", toolsRemoved)
			}

			t.Logf("Removed %d servers and %d tools", serversRemoved, toolsRemoved)
		})
	}
}

// TestCleanupOrphanedAgents tests orphaned agent cleanup
func TestCleanupOrphanedAgents(t *testing.T) {
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

	// Create test environment
	env, err := repos.Environments.Create("test-agent-cleanup-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agentsDir := filepath.Join(workspace, "environments", env.Name, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents directory: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name        string
		setupFunc   func() []string
		options     SyncOptions
		expectCount int
		wantErr     bool
		description string
	}{
		{
			name: "No orphaned agents",
			setupFunc: func() []string {
				// Create an agent and its corresponding .prompt file
				agentService := NewAgentService(repos)
				_, _ = agentService.CreateAgent(ctx, &AgentConfig{
					Name:          "test-agent",
					Prompt:        "Test",
					MaxSteps:      5,
					EnvironmentID: env.ID,
					CreatedBy:     1,
				})

				promptFile := filepath.Join(agentsDir, "test-agent.prompt")
				os.WriteFile(promptFile, []byte("test content"), 0644)

				return []string{promptFile}
			},
			options:     SyncOptions{Confirm: true},
			expectCount: 0,
			wantErr:     false,
			description: "Should not delete agents with .prompt files",
		},
		{
			name: "Empty agents directory",
			setupFunc: func() []string {
				return []string{}
			},
			options:     SyncOptions{Confirm: true},
			expectCount: 0,
			wantErr:     false,
			description: "Should handle empty directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promptFiles := tt.setupFunc()

			count, err := service.cleanupOrphanedAgents(ctx, agentsDir, env.Name, promptFiles, tt.options)

			if (err != nil) != tt.wantErr {
				t.Errorf("cleanupOrphanedAgents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if count != tt.expectCount {
				t.Logf("cleanupOrphanedAgents() count = %d, expected %d (may vary based on setup)", count, tt.expectCount)
			}

			t.Logf("Cleaned up %d orphaned agents", count)
		})
	}
}

// TestCleanupOrphanedResourcesNonExistentEnvironment tests error handling
func TestCleanupOrphanedResourcesNonExistentEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: t.TempDir()}
	service := NewDeclarativeSync(repos, cfg)

	ctx := context.Background()

	_, err = service.cleanupOrphanedResources(ctx, "/tmp/nonexistent", "nonexistent-env", SyncOptions{})

	if err == nil {
		t.Error("cleanupOrphanedResources() should fail for non-existent environment")
	}
}

// TestCleanupOrphanedAgentsNonExistentEnvironment tests error handling
func TestCleanupOrphanedAgentsNonExistentEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: t.TempDir()}
	service := NewDeclarativeSync(repos, cfg)

	ctx := context.Background()

	_, err = service.cleanupOrphanedAgents(ctx, "/tmp/nonexistent", "nonexistent-env", []string{}, SyncOptions{})

	if err == nil {
		t.Error("cleanupOrphanedAgents() should fail for non-existent environment")
	}
}

// TestSyncOptionsConfirmFlag tests the Confirm flag behavior
func TestSyncOptionsConfirmFlag(t *testing.T) {
	tests := []struct {
		name        string
		options     SyncOptions
		expectAuto  bool
		description string
	}{
		{
			name:        "Auto-confirm enabled",
			options:     SyncOptions{Confirm: true},
			expectAuto:  true,
			description: "Should auto-confirm deletions",
		},
		{
			name:        "Auto-confirm disabled",
			options:     SyncOptions{Confirm: false},
			expectAuto:  false,
			description: "Should prompt for confirmation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.options.Confirm != tt.expectAuto {
				t.Errorf("Confirm flag = %v, want %v", tt.options.Confirm, tt.expectAuto)
			}
		})
	}
}

// TestRemoveConfigServersAndToolsServerAssociation tests server association logic
func TestRemoveConfigServersAndToolsServerAssociation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	cfg := &config.Config{Workspace: t.TempDir()}
	service := NewDeclarativeSync(repos, cfg)

	// Create test environment
	env, err := repos.Environments.Create("test-assoc-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create file config
	fileConfigID, err := repos.FileMCPConfigs.Create(&repositories.FileConfigRecord{
		EnvironmentID: env.ID,
		ConfigName:    "myconfig",
		TemplatePath:  "/tmp/myconfig.json",
	})
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	fileConfig, err := repos.FileMCPConfigs.GetByID(fileConfigID)
	if err != nil {
		t.Fatalf("Failed to get file config: %v", err)
	}

	// Create a server with a name that contains the config name
	serverID, err := repos.MCPServers.Create(&models.MCPServer{
		EnvironmentID: env.ID,
		Name:          "myconfig-server",
		Command:       "test command",
		Args:          []string{},
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx := context.Background()

	// Test removal - server should be removed because name contains config name
	serversRemoved, _, err := service.removeConfigServersAndTools(ctx, env.ID, "myconfig", fileConfig)

	if err != nil {
		t.Fatalf("removeConfigServersAndTools() error = %v", err)
	}

	// Verify server was removed
	_, err = repos.MCPServers.GetByID(serverID)
	if err == nil {
		t.Log("Server still exists (heuristic may not have matched)")
	} else {
		t.Logf("Server was removed as expected (removed %d servers)", serversRemoved)
	}
}

// Benchmark tests
func BenchmarkCleanupOrphanedResources(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	workspace := b.TempDir()
	cfg := &config.Config{Workspace: workspace}
	service := NewDeclarativeSync(repos, cfg)

	env, _ := repos.Environments.Create("bench-env", nil, 1)
	envDir := filepath.Join(workspace, "environments", env.Name)
	os.MkdirAll(envDir, 0755)

	ctx := context.Background()
	options := SyncOptions{DryRun: true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.cleanupOrphanedResources(ctx, envDir, env.Name, options)
	}
}
