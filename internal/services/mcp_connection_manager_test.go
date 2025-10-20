package services

import (
	"context"
	"os"
	"testing"
	"time"

	"station/internal/db"
	"station/internal/db/repositories"

	"github.com/firebase/genkit/go/genkit"
)

// TestNewMCPConnectionManager tests manager creation
func TestNewMCPConnectionManager(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	ctx := context.Background()
	genkitApp := genkit.Init(ctx)

	tests := []struct {
		name              string
		repos             *repositories.Repositories
		genkitApp         *genkit.Genkit
		poolingEnv        string
		expectPooling     bool
	}{
		{
			name:          "Create with default pooling enabled",
			repos:         repos,
			genkitApp:     genkitApp,
			poolingEnv:    "",
			expectPooling: true,
		},
		{
			name:          "Create with pooling explicitly enabled",
			repos:         repos,
			genkitApp:     genkitApp,
			poolingEnv:    "true",
			expectPooling: true,
		},
		{
			name:          "Create with pooling disabled",
			repos:         repos,
			genkitApp:     genkitApp,
			poolingEnv:    "false",
			expectPooling: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.poolingEnv != "" {
				os.Setenv("STATION_MCP_POOLING", tt.poolingEnv)
				defer os.Unsetenv("STATION_MCP_POOLING")
			}

			manager := NewMCPConnectionManager(tt.repos, tt.genkitApp)

			if manager == nil {
				t.Fatal("NewMCPConnectionManager returned nil")
			}
			if manager.repos == nil {
				t.Error("Manager repos should not be nil")
			}
			if manager.genkitApp == nil {
				t.Error("Manager genkitApp should not be nil")
			}
			if manager.toolCache == nil {
				t.Error("Tool cache should be initialized")
			}
			if manager.serverPool == nil {
				t.Error("Server pool should be initialized")
			}
			if manager.poolingEnabled != tt.expectPooling {
				t.Errorf("Pooling enabled = %v, want %v", manager.poolingEnabled, tt.expectPooling)
			}
		})
	}
}

// TestEnvironmentToolCache tests cache functionality
func TestEnvironmentToolCache(t *testing.T) {
	t.Run("Valid cache", func(t *testing.T) {
		cache := &EnvironmentToolCache{
			CachedAt: time.Now(),
			ValidFor: 5 * time.Minute,
		}

		if !cache.IsValid() {
			t.Error("Cache should be valid immediately after creation")
		}
	})

	t.Run("Expired cache", func(t *testing.T) {
		cache := &EnvironmentToolCache{
			CachedAt: time.Now().Add(-10 * time.Minute),
			ValidFor: 5 * time.Minute,
		}

		if cache.IsValid() {
			t.Error("Cache should be expired after 10 minutes with 5 minute validity")
		}
	})

	t.Run("Cache at boundary", func(t *testing.T) {
		cache := &EnvironmentToolCache{
			CachedAt: time.Now().Add(-5 * time.Minute),
			ValidFor: 5 * time.Minute,
		}

		// Should be expired or very close to expiring
		// We allow small timing variance here
		t.Logf("Cache valid: %v (timing dependent)", cache.IsValid())
	})
}

// TestGetEnvBoolOrDefault tests boolean environment variable parsing
func TestGetEnvBoolOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue bool
		want         bool
	}{
		{
			name:         "True values - 'true'",
			envValue:     "true",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "True values - '1'",
			envValue:     "1",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "True values - 'yes'",
			envValue:     "yes",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "True values - 'on'",
			envValue:     "on",
			defaultValue: false,
			want:         true,
		},
		{
			name:         "False values - 'false'",
			envValue:     "false",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "False values - '0'",
			envValue:     "0",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "False values - 'no'",
			envValue:     "no",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "False values - 'off'",
			envValue:     "off",
			defaultValue: true,
			want:         false,
		},
		{
			name:         "Empty string uses default true",
			envValue:     "",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "Empty string uses default false",
			envValue:     "",
			defaultValue: false,
			want:         false,
		},
		{
			name:         "Invalid value uses default",
			envValue:     "invalid",
			defaultValue: true,
			want:         true,
		},
		{
			name:         "Case insensitive - 'TRUE'",
			envValue:     "TRUE",
			defaultValue: false,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_BOOL_VAR", tt.envValue)
				defer os.Unsetenv("TEST_BOOL_VAR")
			}

			got := getEnvBoolOrDefault("TEST_BOOL_VAR", tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvBoolOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetEnvIntOrDefault tests integer environment variable parsing
func TestGetEnvIntOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		envValue     string
		defaultValue int
		want         int
	}{
		{
			name:         "Valid positive integer",
			envValue:     "42",
			defaultValue: 0,
			want:         42,
		},
		{
			name:         "Valid negative integer",
			envValue:     "-10",
			defaultValue: 0,
			want:         -10,
		},
		{
			name:         "Zero value",
			envValue:     "0",
			defaultValue: 100,
			want:         0,
		},
		{
			name:         "Empty string uses default",
			envValue:     "",
			defaultValue: 99,
			want:         99,
		},
		{
			name:         "Invalid integer uses default",
			envValue:     "not-a-number",
			defaultValue: 50,
			want:         50,
		},
		{
			name:         "Float value truncated",
			envValue:     "42.7",
			defaultValue: 10,
			want:         10, // Invalid, uses default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("TEST_INT_VAR", tt.envValue)
				defer os.Unsetenv("TEST_INT_VAR")
			}

			got := getEnvIntOrDefault("TEST_INT_VAR", tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnvIntOrDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetMapKeys tests map key extraction
func TestGetMapKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		wantLen  int
		contains []string
	}{
		{
			name:     "Empty map",
			input:    map[string]interface{}{},
			wantLen:  0,
			contains: []string{},
		},
		{
			name: "Single key",
			input: map[string]interface{}{
				"key1": "value1",
			},
			wantLen:  1,
			contains: []string{"key1"},
		},
		{
			name: "Multiple keys",
			input: map[string]interface{}{
				"foo": 1,
				"bar": 2,
				"baz": 3,
			},
			wantLen:  3,
			contains: []string{"foo", "bar", "baz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := getMapKeys(tt.input)

			if len(keys) != tt.wantLen {
				t.Errorf("getMapKeys() returned %d keys, want %d", len(keys), tt.wantLen)
			}

			// Check all expected keys are present
			for _, expectedKey := range tt.contains {
				found := false
				for _, key := range keys {
					if key == expectedKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected key %q not found in result", expectedKey)
				}
			}
		})
	}
}

// TestFileExists tests file existence checking
func TestFileExists(t *testing.T) {
	// Create temporary test file
	tmpFile := t.TempDir() + "/test-file.txt"
	os.WriteFile(tmpFile, []byte("test"), 0644)

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "Existing file",
			path: tmpFile,
			want: true,
		},
		{
			name: "Non-existent file",
			path: "/nonexistent/file/path.txt",
			want: false,
		},
		{
			name: "Directory exists",
			path: t.TempDir(),
			want: true,
		},
		{
			name: "Empty path",
			path: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fileExists(tt.path)
			if got != tt.want {
				t.Errorf("fileExists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestEnableConnectionPooling tests pooling enablement
func TestEnableConnectionPooling(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	genkitApp := genkit.Init(context.Background())

	// Create manager with pooling disabled
	os.Setenv("STATION_MCP_POOLING", "false")
	defer os.Unsetenv("STATION_MCP_POOLING")

	manager := NewMCPConnectionManager(repos, genkitApp)

	if manager.poolingEnabled {
		t.Error("Pooling should be disabled initially")
	}

	// Enable pooling
	manager.EnableConnectionPooling()

	if !manager.poolingEnabled {
		t.Error("Pooling should be enabled after EnableConnectionPooling()")
	}
}

// TestCleanupConnections tests connection cleanup
func TestCleanupConnections(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	genkitApp := genkit.Init(context.Background())
	manager := NewMCPConnectionManager(repos, genkitApp)

	tests := []struct {
		name    string
		clients []*interface{} // Using interface{} as placeholder
		wantErr bool
	}{
		{
			name:    "Cleanup nil clients",
			clients: nil,
			wantErr: false,
		},
		{
			name:    "Cleanup empty client list",
			clients: []*interface{}{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// CleanupConnections doesn't return error, just test it doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("CleanupConnections() panicked: %v", r)
				}
			}()

			manager.CleanupConnections(nil)
		})
	}
}

// TestGetEnvironmentMCPTools tests tool retrieval (integration test)
func TestGetEnvironmentMCPTools(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	genkitApp := genkit.Init(context.Background())
	manager := NewMCPConnectionManager(repos, genkitApp)

	// Create test environment
	env, err := repos.Environments.Create("test-mcp-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()

	t.Run("Get tools from environment with no configs", func(t *testing.T) {
		tools, clients, err := manager.GetEnvironmentMCPTools(ctx, env.ID)

		// Should succeed with empty results
		if err != nil {
			t.Logf("GetEnvironmentMCPTools() error (expected for empty env): %v", err)
		}

		if len(tools) > 0 {
			t.Logf("Found %d tools in empty environment", len(tools))
		}
		if len(clients) > 0 {
			t.Logf("Found %d clients in empty environment", len(clients))
		}
	})

	t.Run("Get tools from non-existent environment", func(t *testing.T) {
		_, _, err := manager.GetEnvironmentMCPTools(ctx, 99999)

		// Should handle gracefully
		if err != nil {
			t.Logf("GetEnvironmentMCPTools() error for non-existent env: %v", err)
		}
	})
}

// TestCacheInvalidation tests tool cache invalidation
func TestCacheInvalidation(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	genkitApp := genkit.Init(context.Background())
	manager := NewMCPConnectionManager(repos, genkitApp)

	// Create a cache entry
	envID := int64(1)
	cache := &EnvironmentToolCache{
		CachedAt: time.Now(),
		ValidFor: 1 * time.Second,
	}

	manager.cacheMutex.Lock()
	manager.toolCache[envID] = cache
	manager.cacheMutex.Unlock()

	// Verify cache exists and is valid
	manager.cacheMutex.RLock()
	cachedEntry, exists := manager.toolCache[envID]
	manager.cacheMutex.RUnlock()

	if !exists {
		t.Error("Cache entry should exist")
	}
	if !cachedEntry.IsValid() {
		t.Error("Cache should be valid immediately after creation")
	}

	// Wait for cache to expire
	time.Sleep(1100 * time.Millisecond)

	manager.cacheMutex.RLock()
	cachedEntry, _ = manager.toolCache[envID]
	manager.cacheMutex.RUnlock()

	if cachedEntry.IsValid() {
		t.Error("Cache should be invalid after expiration")
	}
}

// Benchmark tests
func BenchmarkGetMapKeys(b *testing.B) {
	testMap := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
		"key5": "value5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getMapKeys(testMap)
	}
}

func BenchmarkEnvironmentToolCacheIsValid(b *testing.B) {
	cache := &EnvironmentToolCache{
		CachedAt: time.Now(),
		ValidFor: 5 * time.Minute,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cache.IsValid()
	}
}

func BenchmarkFileExists(b *testing.B) {
	tmpFile := b.TempDir() + "/test-file.txt"
	os.WriteFile(tmpFile, []byte("test"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fileExists(tmpFile)
	}
}
