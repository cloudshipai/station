package services

import (
	"context"
	"os"
	"testing"
	"time"

	"station/internal/db"
	"station/internal/db/repositories"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// TestNewMCPConnectionManager tests manager creation
func TestNewMCPConnectionManager(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	ctx := context.Background()
	genkitApp := genkit.Init(ctx)

	tests := []struct {
		name          string
		repos         *repositories.Repositories
		genkitApp     *genkit.Genkit
		poolingEnv    string
		expectPooling bool
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

// TestGetAgentToolsForEnvironment tests agent tool creation
func TestGetAgentToolsForEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	genkitApp := genkit.Init(context.Background())
	manager := NewMCPConnectionManager(repos, genkitApp)

	// Create test environment
	env, err := repos.Environments.Create("test-agent-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()

	t.Run("Get agent tools from empty environment", func(t *testing.T) {
		agentTools := manager.getAgentToolsForEnvironment(ctx, env.ID, nil)

		// Should return empty slice for environment with no agents
		if len(agentTools) != 0 {
			t.Errorf("Expected 0 agent tools for empty environment, got %d", len(agentTools))
		}
	})

	t.Run("Get agent tools with agents", func(t *testing.T) {
		// Create test agents
		_, err := repos.Agents.Create("Test Agent 1", "First test agent", "You are test agent 1", 5, env.ID, 1, nil, nil, false, nil, nil, "", "")
		if err != nil {
			t.Fatalf("Failed to create agent 1: %v", err)
		}

		_, err = repos.Agents.Create("Test Agent 2", "Second test agent", "You are test agent 2", 8, env.ID, 1, nil, nil, false, nil, nil, "", "")
		if err != nil {
			t.Fatalf("Failed to create agent 2: %v", err)
		}

		agentTools := manager.getAgentToolsForEnvironment(ctx, env.ID, nil)

		// Should return 2 agent tools
		if len(agentTools) != 2 {
			t.Fatalf("Expected 2 agent tools, got %d", len(agentTools))
		}

		// Verify tool names and properties
		expectedNames := []string{"__agent_test_agent_1", "__agent_test_agent_2"}
		foundNames := make([]string, len(agentTools))

		for i, tool := range agentTools {
			foundNames[i] = tool.Name()

			// Verify tool has proper definition
			def := tool.Definition()
			if def == nil {
				t.Errorf("Tool %s has nil definition", tool.Name())
			}

			// Verify tool can run raw
			result, err := tool.RunRaw(ctx, map[string]interface{}{
				"task": "test task",
			})
			if err != nil {
				t.Errorf("Tool %s failed to run raw: %v", tool.Name(), err)
			}
			if result == nil {
				t.Errorf("Tool %s returned nil result", tool.Name())
			}
		}

		// Check that we got the expected tool names
		for _, expectedName := range expectedNames {
			found := false
			for _, foundName := range foundNames {
				if foundName == expectedName {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected tool name %s not found in results: %v", expectedName, foundNames)
			}
		}

		t.Logf("Successfully created %d agent tools: %v", len(agentTools), foundNames)
	})

	t.Run("Agent tool naming conventions", func(t *testing.T) {
		// Create agent with special characters in name
		_, err := repos.Agents.Create("Special Agent! @#$%", "Agent with special characters", "You are special agent", 3, env.ID, 1, nil, nil, false, nil, nil, "", "")
		if err != nil {
			t.Fatalf("Failed to create agent 3: %v", err)
		}

		agentTools := manager.getAgentToolsForEnvironment(ctx, env.ID, nil)

		// Find the special agent tool
		var specialTool ai.Tool
		for _, tool := range agentTools {
			if tool.Name() == "__agent_special_agent_____" {
				specialTool = tool
				break
			}
		}

		if specialTool == nil {
			t.Errorf("Special agent tool not found in results")
		} else {
			t.Logf("Special agent tool name: %s", specialTool.Name())
		}
	})
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
