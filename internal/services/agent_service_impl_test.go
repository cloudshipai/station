package services

import (
	"context"
	"os"
	"testing"
	"time"

	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewAgentService tests service creation
func TestNewAgentService(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)

	t.Run("Create service without lighthouse", func(t *testing.T) {
		service := NewAgentService(repos)

		if service == nil {
			t.Fatal("NewAgentService returned nil")
		}
		if service.repos == nil {
			t.Error("Service repos should not be nil")
		}
		if service.executionEngine == nil {
			t.Error("Execution engine should be initialized")
		}
		if service.telemetry == nil {
			t.Error("Telemetry service should be initialized")
		}
		if service.exportService == nil {
			t.Error("Export service should be initialized")
		}
	})

	t.Run("Create service with nil lighthouse", func(t *testing.T) {
		service := NewAgentService(repos, nil)

		if service == nil {
			t.Fatal("NewAgentService returned nil")
		}
		if service.executionEngine == nil {
			t.Error("Execution engine should be initialized even with nil lighthouse")
		}
	})
}

// TestNewAgentServiceWithLighthouse tests service creation with Lighthouse
func TestNewAgentServiceWithLighthouse(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)

	t.Run("Create service with nil lighthouse client", func(t *testing.T) {
		service := NewAgentServiceWithLighthouse(repos, nil)

		if service == nil {
			t.Fatal("NewAgentServiceWithLighthouse returned nil")
		}
		if service.repos == nil {
			t.Error("Service repos should not be nil")
		}
		if service.executionEngine == nil {
			t.Error("Execution engine should be initialized")
		}
	})
}

// TestGetExecutionEngine tests getting execution engine
func TestGetExecutionEngine(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	engine := service.GetExecutionEngine()

	if engine == nil {
		t.Error("GetExecutionEngine() should return non-nil engine")
	}

	// Should return same instance
	engine2 := service.GetExecutionEngine()
	if engine != engine2 {
		t.Error("GetExecutionEngine() should return same instance")
	}
}

// TestCreateAgent tests agent creation
func TestCreateAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment first
	env, err := repos.Environments.Create("test-env", stringPtr("Test environment"), 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr bool
	}{
		{
			name: "Create valid agent",
			config: &AgentConfig{
				Name:          "test-agent-1",
				Description:   "Test agent",
				Prompt:        "You are a test agent",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			},
			wantErr: false,
		},
		{
			name: "Create agent with empty name",
			config: &AgentConfig{
				Name:          "",
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			},
			wantErr: true, // FIXED: CreateAgent now validates empty names
		},
		{
			name: "Create agent with invalid environment",
			config: &AgentConfig{
				Name:          "test-agent-2",
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: 99999,
				CreatedBy:     1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := service.CreateAgent(ctx, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateAgent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if agent == nil {
					t.Error("CreateAgent() returned nil agent")
				}
				if agent.Name != tt.config.Name {
					t.Errorf("Agent name = %s, want %s", agent.Name, tt.config.Name)
				}
				if agent.MaxSteps != tt.config.MaxSteps {
					t.Errorf("Agent MaxSteps = %d, want %d", agent.MaxSteps, tt.config.MaxSteps)
				}
			}
		})
	}
}

// TestGetAgent tests retrieving agents
func TestGetAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment and agent
	env, err := repos.Environments.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, &AgentConfig{
		Name:          "test-get-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	tests := []struct {
		name    string
		agentID int64
		wantErr bool
	}{
		{
			name:    "Get existing agent",
			agentID: agent.ID,
			wantErr: false,
		},
		{
			name:    "Get non-existent agent",
			agentID: 99999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.GetAgent(ctx, tt.agentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetAgent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("GetAgent() returned nil")
				}
				if result.ID != tt.agentID {
					t.Errorf("Agent ID = %d, want %d", result.ID, tt.agentID)
				}
			}
		})
	}
}

// TestListAgentsByEnvironment tests listing agents
func TestListAgentsByEnvironment(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment
	env, err := repos.Environments.Create("test-list-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()

	// Create multiple agents
	for i := 0; i < 3; i++ {
		_, err := service.CreateAgent(ctx, &AgentConfig{
			Name:          "test-agent-" + string(rune('A'+i)),
			Prompt:        "Test",
			MaxSteps:      5,
			EnvironmentID: env.ID,
			CreatedBy:     1,
		})
		if err != nil {
			t.Fatalf("Failed to create test agent %d: %v", i, err)
		}
	}

	tests := []struct {
		name          string
		environmentID int64
		minExpected   int
		wantErr       bool
	}{
		{
			name:          "List agents in populated environment",
			environmentID: env.ID,
			minExpected:   3,
			wantErr:       false,
		},
		{
			name:          "List agents in non-existent environment",
			environmentID: 99999,
			minExpected:   0,
			wantErr:       false, // Should return empty list, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agents, err := service.ListAgentsByEnvironment(ctx, tt.environmentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListAgentsByEnvironment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(agents) < tt.minExpected {
					t.Errorf("ListAgentsByEnvironment() returned %d agents, want at least %d", len(agents), tt.minExpected)
				}
			}
		})
	}
}

// TestUpdateAgent tests agent updates
func TestUpdateAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment and agent
	env, err := repos.Environments.Create("test-update-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, &AgentConfig{
		Name:          "test-update-agent",
		Prompt:        "Original prompt",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	tests := []struct {
		name    string
		agentID int64
		config  *AgentConfig
		wantErr bool
	}{
		{
			name:    "Update agent prompt",
			agentID: agent.ID,
			config: &AgentConfig{
				Name:          agent.Name,
				Prompt:        "Updated prompt",
				MaxSteps:      agent.MaxSteps,
				EnvironmentID: agent.EnvironmentID,
				CreatedBy:     agent.CreatedBy,
			},
			wantErr: false,
		},
		{
			name:    "Update non-existent agent",
			agentID: 99999,
			config: &AgentConfig{
				Name:          "test",
				Prompt:        "test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated, err := service.UpdateAgent(ctx, tt.agentID, tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateAgent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if updated == nil {
					t.Error("UpdateAgent() returned nil")
				}
				if updated.Prompt != tt.config.Prompt {
					t.Errorf("Updated prompt = %s, want %s", updated.Prompt, tt.config.Prompt)
				}
			}
		})
	}
}

// TestUpdateAgentPrompt tests prompt updates
func TestUpdateAgentPrompt(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment and agent
	env, err := repos.Environments.Create("test-prompt-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, &AgentConfig{
		Name:          "test-prompt-agent",
		Prompt:        "Original",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	newPrompt := "Updated prompt content"
	err = service.UpdateAgentPrompt(ctx, agent.ID, newPrompt)

	if err != nil {
		t.Errorf("UpdateAgentPrompt() error = %v", err)
	}

	// Verify update
	updated, err := service.GetAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("Failed to get updated agent: %v", err)
	}

	if updated.Prompt != newPrompt {
		t.Errorf("Prompt after update = %s, want %s", updated.Prompt, newPrompt)
	}
}

// TestDeleteAgent tests agent deletion
func TestDeleteAgent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment and agent
	env, err := repos.Environments.Create("test-delete-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, &AgentConfig{
		Name:          "test-delete-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	tests := []struct {
		name    string
		agentID int64
		wantErr bool
	}{
		{
			name:    "Delete existing agent",
			agentID: agent.ID,
			wantErr: false,
		},
		{
			name:    "Delete non-existent agent",
			agentID: 99999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.DeleteAgent(ctx, tt.agentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("DeleteAgent() error = %v, wantErr %v", err, tt.wantErr)
			}

			// For successful deletion, verify agent is gone
			if !tt.wantErr {
				_, err := service.GetAgent(ctx, tt.agentID)
				if err == nil {
					t.Error("Agent should not exist after deletion")
				}
			}
		})
	}
}

// TestExecuteAgent tests agent execution (integration test)
func TestExecuteAgentIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Skip in CI environments (no OpenAI API key available)
	if os.Getenv("CI") != "" {
		t.Skip("Skipping agent execution test in CI environment (requires OpenAI API key)")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	// Create environment and agent
	env, err := repos.Environments.Create("test-exec-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, &AgentConfig{
		Name:          "test-exec-agent",
		Prompt:        "You are a test agent. {{ userInput }}",
		MaxSteps:      1,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}

	t.Run("Execute agent with valid task", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// This will likely fail without proper API key setup, but tests the execution path
		msg, err := service.ExecuteAgent(ctx, agent.ID, "test task", nil)

		// Log result regardless of success/failure
		if err != nil {
			t.Logf("ExecuteAgent() failed (expected without API key): %v", err)
		}
		if msg != nil {
			t.Logf("ExecuteAgent() returned message: %+v", msg)
		}
	})

	t.Run("Execute non-existent agent", func(t *testing.T) {
		_, err := service.ExecuteAgent(context.Background(), 99999, "test", nil)
		if err == nil {
			t.Error("ExecuteAgent() should return error for non-existent agent")
		}
	})
}

// Benchmark tests
func BenchmarkCreateAgent(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	env, err := repos.Environments.Create("bench-env", nil, 1)
	if err != nil {
		b.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	config := &AgentConfig{
		Name:          "bench-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Name = "bench-agent-" + string(rune('A'+i%26))
		_, _ = service.CreateAgent(ctx, config)
	}
}

func BenchmarkGetAgent(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	service := NewAgentService(repos)

	env, err := repos.Environments.Create("bench-env", nil, 1)
	if err != nil {
		b.Fatalf("Failed to create environment: %v", err)
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, &AgentConfig{
		Name:          "bench-agent",
		Prompt:        "Test",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		b.Fatalf("Failed to create agent: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.GetAgent(ctx, agent.ID)
	}
}
