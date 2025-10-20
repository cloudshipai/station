package services

import (
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// TestNewMCPServerPool tests pool creation
func TestNewMCPServerPool(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "Create new server pool",
			description: "Should create pool with empty maps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewMCPServerPool()

			if pool == nil {
				t.Fatal("NewMCPServerPool() returned nil")
			}

			if pool.servers == nil {
				t.Error("Pool servers map should be initialized")
			}

			if pool.serverConfigs == nil {
				t.Error("Pool serverConfigs map should be initialized")
			}

			if pool.tools == nil {
				t.Error("Pool tools map should be initialized")
			}

			// Verify maps are empty
			if len(pool.servers) != 0 {
				t.Errorf("Pool servers map should be empty, got %d entries", len(pool.servers))
			}

			if len(pool.serverConfigs) != 0 {
				t.Errorf("Pool serverConfigs map should be empty, got %d entries", len(pool.serverConfigs))
			}

			if len(pool.tools) != 0 {
				t.Errorf("Pool tools map should be empty, got %d entries", len(pool.tools))
			}

			// Verify initialized flag
			if pool.initialized {
				t.Error("Pool initialized flag should be false initially")
			}
		})
	}
}

// TestMCPServerPoolConcurrency tests concurrent pool access
func TestMCPServerPoolConcurrency(t *testing.T) {
	pool := NewMCPServerPool()

	// Test concurrent writes (should not panic)
	t.Run("Concurrent writes", func(t *testing.T) {
		done := make(chan bool)

		// Write goroutine 1
		go func() {
			pool.mutex.Lock()
			pool.servers["test1"] = nil
			pool.mutex.Unlock()
			done <- true
		}()

		// Write goroutine 2
		go func() {
			pool.mutex.Lock()
			pool.servers["test2"] = nil
			pool.mutex.Unlock()
			done <- true
		}()

		// Wait for both goroutines
		<-done
		<-done

		// Verify both writes succeeded
		pool.mutex.RLock()
		count := len(pool.servers)
		pool.mutex.RUnlock()

		if count != 2 {
			t.Errorf("Expected 2 servers in pool, got %d", count)
		}
	})

	// Test concurrent reads (should not panic)
	t.Run("Concurrent reads", func(t *testing.T) {
		done := make(chan bool)

		// Read goroutine 1
		go func() {
			pool.mutex.RLock()
			_ = pool.servers
			pool.mutex.RUnlock()
			done <- true
		}()

		// Read goroutine 2
		go func() {
			pool.mutex.RLock()
			_ = pool.servers
			pool.mutex.RUnlock()
			done <- true
		}()

		// Wait for both goroutines
		<-done
		<-done
	})
}

// TestServerDefinition tests server definition structure
func TestServerDefinition(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		serverName    string
		config        interface{}
		environmentID int64
		description   string
	}{
		{
			name:          "Valid server definition",
			key:           "server1:hash123",
			serverName:    "filesystem",
			config:        map[string]interface{}{"command": "npx"},
			environmentID: 1,
			description:   "Should create valid server definition",
		},
		{
			name:          "Server with empty key",
			key:           "",
			serverName:    "filesystem",
			config:        map[string]interface{}{},
			environmentID: 2,
			description:   "Should allow empty key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := serverDefinition{
				key:           tt.key,
				name:          tt.serverName,
				config:        tt.config,
				environmentID: tt.environmentID,
			}

			if def.key != tt.key {
				t.Errorf("Server key = %s, want %s", def.key, tt.key)
			}

			if def.name != tt.serverName {
				t.Errorf("Server name = %s, want %s", def.name, tt.serverName)
			}

			if def.environmentID != tt.environmentID {
				t.Errorf("Environment ID = %d, want %d", def.environmentID, tt.environmentID)
			}
		})
	}
}

// TestGenerateServerKey tests server key generation
func TestGenerateServerKey(t *testing.T) {
	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}

	tests := []struct {
		name         string
		serverName   string
		serverConfig interface{}
		wantPrefix   string
		description  string
	}{
		{
			name:         "Generate key for simple config",
			serverName:   "filesystem",
			serverConfig: map[string]interface{}{"command": "npx"},
			wantPrefix:   "filesystem:",
			description:  "Should prefix with server name",
		},
		{
			name:         "Generate key for empty config",
			serverName:   "test-server",
			serverConfig: map[string]interface{}{},
			wantPrefix:   "test-server:",
			description:  "Should handle empty config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := mcm.generateServerKey(tt.serverName, tt.serverConfig)

			if key == "" {
				t.Error("generateServerKey() returned empty key")
			}

			// Verify key starts with server name
			if len(key) < len(tt.wantPrefix) {
				t.Errorf("Key too short: %s", key)
				return
			}

			if key[:len(tt.wantPrefix)] != tt.wantPrefix {
				t.Errorf("Key = %s, should start with %s", key, tt.wantPrefix)
			}
		})
	}
}

// TestDeduplicateServers tests server deduplication
func TestDeduplicateServers(t *testing.T) {
	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}

	tests := []struct {
		name        string
		servers     []serverDefinition
		wantCount   int
		description string
	}{
		{
			name:        "Empty server list",
			servers:     []serverDefinition{},
			wantCount:   0,
			description: "Should return empty list",
		},
		{
			name: "No duplicates",
			servers: []serverDefinition{
				{key: "server1", name: "fs1"},
				{key: "server2", name: "fs2"},
			},
			wantCount:   2,
			description: "Should return all servers",
		},
		{
			name: "With duplicates",
			servers: []serverDefinition{
				{key: "server1", name: "fs1"},
				{key: "server1", name: "fs1"}, // Duplicate
				{key: "server2", name: "fs2"},
			},
			wantCount:   2,
			description: "Should remove duplicates",
		},
		{
			name: "All duplicates",
			servers: []serverDefinition{
				{key: "server1", name: "fs1"},
				{key: "server1", name: "fs1"},
				{key: "server1", name: "fs1"},
			},
			wantCount:   1,
			description: "Should return single server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mcm.deduplicateServers(tt.servers)

			if len(result) != tt.wantCount {
				t.Errorf("deduplicateServers() count = %d, want %d", len(result), tt.wantCount)
			}

			// Verify no duplicates in result
			seen := make(map[string]bool)
			for _, server := range result {
				if seen[server.key] {
					t.Errorf("Duplicate server key in result: %s", server.key)
				}
				seen[server.key] = true
			}
		})
	}
}

// TestStartPooledServersParallel tests parallel server startup
func TestStartPooledServersParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}

	tests := []struct {
		name        string
		servers     []serverDefinition
		wantErr     bool
		description string
	}{
		{
			name:        "Empty server list",
			servers:     []serverDefinition{},
			wantErr:     false,
			description: "Should succeed with empty list",
		},
		{
			name: "Invalid server configs",
			servers: []serverDefinition{
				{key: "test1", name: "invalid", config: nil},
			},
			wantErr:     true,
			description: "Should fail with invalid config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mcm.startPooledServersParallel(nil, tt.servers)

			if (err != nil) != tt.wantErr {
				t.Errorf("startPooledServersParallel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMCPServerPoolTools tests pool tool storage
func TestMCPServerPoolTools(t *testing.T) {
	pool := NewMCPServerPool()

	// Mock tools
	mockTools := []ai.Tool{
		// GenKit Tool interface - we can't create real instances without MCP server
		// but we can test the storage mechanism
	}

	tests := []struct {
		name        string
		serverKey   string
		tools       []ai.Tool
		description string
	}{
		{
			name:        "Store empty tools",
			serverKey:   "server1",
			tools:       []ai.Tool{},
			description: "Should store empty tool list",
		},
		{
			name:        "Store tools for server",
			serverKey:   "server2",
			tools:       mockTools,
			description: "Should store tool list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool.mutex.Lock()
			pool.tools[tt.serverKey] = tt.tools
			pool.mutex.Unlock()

			// Verify storage
			pool.mutex.RLock()
			stored, exists := pool.tools[tt.serverKey]
			pool.mutex.RUnlock()

			if !exists {
				t.Errorf("Tools for server %s not stored", tt.serverKey)
			}

			if len(stored) != len(tt.tools) {
				t.Errorf("Stored tools count = %d, want %d", len(stored), len(tt.tools))
			}
		})
	}
}

// TestMCPServerPoolClients tests pool client storage
func TestMCPServerPoolClients(t *testing.T) {
	pool := NewMCPServerPool()

	tests := []struct {
		name        string
		serverKey   string
		client      *mcp.GenkitMCPClient
		description string
	}{
		{
			name:        "Store nil client",
			serverKey:   "server1",
			client:      nil,
			description: "Should allow storing nil client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool.mutex.Lock()
			pool.servers[tt.serverKey] = tt.client
			pool.mutex.Unlock()

			// Verify storage
			pool.mutex.RLock()
			_, exists := pool.servers[tt.serverKey]
			pool.mutex.RUnlock()

			if !exists {
				t.Errorf("Client for server %s not stored", tt.serverKey)
			}
		})
	}
}

// TestMCPServerPoolServerConfigs tests pool config storage
func TestMCPServerPoolServerConfigs(t *testing.T) {
	pool := NewMCPServerPool()

	tests := []struct {
		name        string
		serverKey   string
		config      interface{}
		description string
	}{
		{
			name:        "Store simple config",
			serverKey:   "server1",
			config:      map[string]interface{}{"command": "npx"},
			description: "Should store config map",
		},
		{
			name:        "Store nil config",
			serverKey:   "server2",
			config:      nil,
			description: "Should allow nil config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool.mutex.Lock()
			pool.serverConfigs[tt.serverKey] = tt.config
			pool.mutex.Unlock()

			// Verify storage
			pool.mutex.RLock()
			_, exists := pool.serverConfigs[tt.serverKey]
			pool.mutex.RUnlock()

			if !exists {
				t.Errorf("Config for server %s not stored", tt.serverKey)
			}
		})
	}
}

// TestShutdownServerPool tests pool shutdown
func TestShutdownServerPool(t *testing.T) {
	mcm := &MCPConnectionManager{
		serverPool:     NewMCPServerPool(),
		poolingEnabled: true,
	}

	// Populate pool with test data
	mcm.serverPool.mutex.Lock()
	mcm.serverPool.servers["test1"] = nil
	mcm.serverPool.serverConfigs["test1"] = map[string]interface{}{}
	mcm.serverPool.tools["test1"] = []ai.Tool{}
	mcm.serverPool.mutex.Unlock()

	tests := []struct {
		name           string
		poolingEnabled bool
		description    string
	}{
		{
			name:           "Shutdown with pooling enabled",
			poolingEnabled: true,
			description:    "Should clear all maps",
		},
		{
			name:           "Shutdown with pooling disabled",
			poolingEnabled: false,
			description:    "Should be no-op",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcm.poolingEnabled = tt.poolingEnabled
			mcm.ShutdownServerPool()

			if tt.poolingEnabled {
				// Verify pool is cleared
				mcm.serverPool.mutex.RLock()
				serversCount := len(mcm.serverPool.servers)
				configsCount := len(mcm.serverPool.serverConfigs)
				toolsCount := len(mcm.serverPool.tools)
				mcm.serverPool.mutex.RUnlock()

				if serversCount != 0 {
					t.Errorf("Servers map not cleared, got %d entries", serversCount)
				}

				if configsCount != 0 {
					t.Errorf("ServerConfigs map not cleared, got %d entries", configsCount)
				}

				if toolsCount != 0 {
					t.Errorf("Tools map not cleared, got %d entries", toolsCount)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewMCPServerPool(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewMCPServerPool()
	}
}

func BenchmarkGenerateServerKey(b *testing.B) {
	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}
	config := map[string]interface{}{"command": "npx", "args": []string{"-y", "test"}}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mcm.generateServerKey("filesystem", config)
	}
}

func BenchmarkDeduplicateServers(b *testing.B) {
	mcm := &MCPConnectionManager{
		serverPool: NewMCPServerPool(),
	}

	servers := []serverDefinition{
		{key: "server1", name: "fs1"},
		{key: "server1", name: "fs1"},
		{key: "server2", name: "fs2"},
		{key: "server3", name: "fs3"},
		{key: "server2", name: "fs2"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mcm.deduplicateServers(servers)
	}
}
