package services

import (
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// TestNewToolDiscoveryService tests service creation
func TestNewToolDiscoveryService(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "Create tool discovery service",
			description: "Should create service with initialized repos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB, err := db.NewTest(t)
			if err != nil {
				t.Fatalf("Failed to create test database: %v", err)
			}
			defer func() { _ = testDB.Close() }()

			repos := repositories.New(testDB)
			service := NewToolDiscoveryService(repos)

			if service == nil {
				t.Fatal("NewToolDiscoveryService() returned nil")
			}

			if service.repos == nil {
				t.Error("Service repos should be initialized")
			}
		})
	}
}

// TestGetToolsByEnvironment tests retrieving tools by environment
func TestGetToolsByEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	// Create test environment
	env, err := repos.Environments.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create MCP server
	serverID, err := repos.MCPServers.Create(&models.MCPServer{
		Name:          "test-server",
		Command:       "npx",
		EnvironmentID: env.ID,
	})
	if err != nil {
		t.Fatalf("Failed to create MCP server: %v", err)
	}

	server, _ := repos.MCPServers.GetByID(serverID)

	// Create tools
	repos.MCPTools.Create(&models.MCPTool{
		MCPServerID: server.ID,
		Name:        "read_file",
		Description: "Reads a file",
	})

	repos.MCPTools.Create(&models.MCPTool{
		MCPServerID: server.ID,
		Name:        "write_file",
		Description: "Writes a file",
	})

	tests := []struct {
		name          string
		environmentID int64
		wantCount     int
		wantErr       bool
		description   string
	}{
		{
			name:          "Get tools for environment",
			environmentID: env.ID,
			wantCount:     2,
			wantErr:       false,
			description:   "Should return all tools in environment",
		},
		{
			name:          "Get tools for non-existent environment",
			environmentID: 99999,
			wantCount:     0,
			wantErr:       false,
			description:   "Should return empty list for non-existent environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := service.GetToolsByEnvironment(tt.environmentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetToolsByEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(tools) != tt.wantCount {
				t.Errorf("GetToolsByEnvironment() returned %d tools, want %d", len(tools), tt.wantCount)
			}

			t.Logf("Retrieved %d tools for environment %d", len(tools), tt.environmentID)
		})
	}
}

// TestGetToolsByServer tests retrieving tools by server
func TestGetToolsByServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	// Create test environment
	env, _ := repos.Environments.Create("test-env", nil, 1)

	// Create MCP servers
	server1ID, _ := repos.MCPServers.Create(&models.MCPServer{
		Name:          "server1",
		Command:       "npx",
		EnvironmentID: env.ID,
	})

	server2ID, _ := repos.MCPServers.Create(&models.MCPServer{
		Name:          "server2",
		Command:       "npx",
		EnvironmentID: env.ID,
	})

	server1, _ := repos.MCPServers.GetByID(server1ID)
	server2, _ := repos.MCPServers.GetByID(server2ID)

	// Create tools for server1
	repos.MCPTools.Create(&models.MCPTool{
		MCPServerID: server1.ID,
		Name:        "tool1",
		Description: "Tool 1",
	})

	repos.MCPTools.Create(&models.MCPTool{
		MCPServerID: server1.ID,
		Name:        "tool2",
		Description: "Tool 2",
	})

	// Create tools for server2
	repos.MCPTools.Create(&models.MCPTool{
		MCPServerID: server2.ID,
		Name:        "tool3",
		Description: "Tool 3",
	})

	tests := []struct {
		name        string
		serverID    int64
		wantCount   int
		wantErr     bool
		description string
	}{
		{
			name:        "Get tools for server 1",
			serverID:    server1.ID,
			wantCount:   2,
			wantErr:     false,
			description: "Should return 2 tools for server 1",
		},
		{
			name:        "Get tools for server 2",
			serverID:    server2.ID,
			wantCount:   1,
			wantErr:     false,
			description: "Should return 1 tool for server 2",
		},
		{
			name:        "Get tools for non-existent server",
			serverID:    99999,
			wantCount:   0,
			wantErr:     false,
			description: "Should return empty list for non-existent server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := service.GetToolsByServer(tt.serverID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetToolsByServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(tools) != tt.wantCount {
				t.Errorf("GetToolsByServer() returned %d tools, want %d", len(tools), tt.wantCount)
			}

			t.Logf("Retrieved %d tools for server %d", len(tools), tt.serverID)
		})
	}
}

// TestGetHybridToolsByEnvironment tests hybrid tool retrieval
func TestGetHybridToolsByEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	tests := []struct {
		name          string
		environmentID int64
		wantErr       bool
		description   string
	}{
		{
			name:          "Get hybrid tools for environment",
			environmentID: 1,
			wantErr:       false,
			description:   "Should return empty list (not yet implemented)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := service.GetHybridToolsByEnvironment(tt.environmentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetHybridToolsByEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Currently returns empty list
			if tools == nil {
				t.Error("GetHybridToolsByEnvironment() should return non-nil slice")
			}

			t.Logf("Retrieved %d hybrid tools for environment %d", len(tools), tt.environmentID)
		})
	}
}

// TestDeprecatedDiscoverToolsFromFileConfig tests deprecated method
func TestDeprecatedDiscoverToolsFromFileConfig(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	err = service.DiscoverToolsFromFileConfig(1, "test-config", nil)

	if err == nil {
		t.Error("DiscoverToolsFromFileConfig() should return error for deprecated method")
	}

	errMsg := err.Error()
	if !stringContains(errMsg, "deprecated") {
		t.Errorf("Error should mention 'deprecated', got: %s", errMsg)
	}

	if !stringContains(errMsg, "DeclarativeSync") {
		t.Errorf("Error should mention 'DeclarativeSync', got: %s", errMsg)
	}
}

// TestDeprecatedDiscoverToolsFromFileConfigNew tests deprecated method
func TestDeprecatedDiscoverToolsFromFileConfigNew(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	err = service.DiscoverToolsFromFileConfigNew(1, "test-config", false)

	if err == nil {
		t.Error("DiscoverToolsFromFileConfigNew() should return error for deprecated method")
	}

	errMsg := err.Error()
	if !stringContains(errMsg, "deprecated") {
		t.Errorf("Error should mention 'deprecated', got: %s", errMsg)
	}
}

// TestDeprecatedGetToolsByFileConfig tests deprecated method
func TestDeprecatedGetToolsByFileConfig(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	tools, err := service.GetToolsByFileConfig(1)

	if err == nil {
		t.Error("GetToolsByFileConfig() should return error for deprecated method")
	}

	if tools != nil {
		t.Error("GetToolsByFileConfig() should return nil tools for deprecated method")
	}

	errMsg := err.Error()
	if !stringContains(errMsg, "deprecated") {
		t.Errorf("Error should mention 'deprecated', got: %s", errMsg)
	}
}

// TestGetToolsByEnvironmentEmpty tests empty environment
func TestGetToolsByEnvironmentEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	// Create environment with no tools
	env, _ := repos.Environments.Create("empty-env", nil, 1)

	tools, err := service.GetToolsByEnvironment(env.ID)

	if err != nil {
		t.Errorf("GetToolsByEnvironment() error = %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("GetToolsByEnvironment() should return 0 tools for empty environment, got %d", len(tools))
	}
}

// TestGetToolsByServerEmpty tests empty server
func TestGetToolsByServerEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	// Create server with no tools
	env, _ := repos.Environments.Create("test-env", nil, 1)
	serverID, _ := repos.MCPServers.Create(&models.MCPServer{
		Name:          "empty-server",
		Command:       "npx",
		EnvironmentID: env.ID,
	})

	server, _ := repos.MCPServers.GetByID(serverID)

	tools, err := service.GetToolsByServer(server.ID)

	if err != nil {
		t.Errorf("GetToolsByServer() error = %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("GetToolsByServer() should return 0 tools for empty server, got %d", len(tools))
	}
}

// Benchmark tests
func BenchmarkGetToolsByEnvironment(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	env, _ := repos.Environments.Create("bench-env", nil, 1)
	serverID, _ := repos.MCPServers.Create(&models.MCPServer{
		Name:          "bench-server",
		Command:       "npx",
		EnvironmentID: env.ID,
	})

	server, _ := repos.MCPServers.GetByID(serverID)

	// Create multiple tools
	for i := 0; i < 10; i++ {
		repos.MCPTools.Create(&models.MCPTool{
			MCPServerID: server.ID,
			Name:        "tool",
			Description: "Benchmark tool",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetToolsByEnvironment(env.ID)
	}
}

func BenchmarkGetToolsByServer(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	env, _ := repos.Environments.Create("bench-env", nil, 1)
	serverID, _ := repos.MCPServers.Create(&models.MCPServer{
		Name:          "bench-server",
		Command:       "npx",
		EnvironmentID: env.ID,
	})

	server, _ := repos.MCPServers.GetByID(serverID)

	// Create multiple tools
	for i := 0; i < 10; i++ {
		repos.MCPTools.Create(&models.MCPTool{
			MCPServerID: server.ID,
			Name:        "tool",
			Description: "Benchmark tool",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetToolsByServer(server.ID)
	}
}

func BenchmarkGetHybridToolsByEnvironment(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewToolDiscoveryService(repos)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetHybridToolsByEnvironment(1)
	}
}
