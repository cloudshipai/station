package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// TestNewAgentExecutionEngine tests engine creation
func TestNewAgentExecutionEngine(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)

	tests := []struct {
		name        string
		repos       *repositories.Repositories
		agentSvc    AgentServiceInterface
		expectNil   bool
		description string
	}{
		{
			name:        "Valid engine creation",
			repos:       repos,
			agentSvc:    agentService,
			expectNil:   false,
			description: "Should create engine with valid dependencies",
		},
		{
			name:        "Nil repositories",
			repos:       nil,
			agentSvc:    agentService,
			expectNil:   false,
			description: "Should still create engine (may panic on use)",
		},
		{
			name:        "Nil agent service",
			repos:       repos,
			agentSvc:    nil,
			expectNil:   false,
			description: "Should still create engine (may panic on use)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewAgentExecutionEngine(tt.repos, tt.agentSvc)

			if tt.expectNil {
				if engine != nil {
					t.Errorf("Expected nil engine, got %v", engine)
				}
			} else {
				if engine == nil {
					t.Error("Expected non-nil engine")
				} else {
					// Verify internal components
					if engine.genkitProvider == nil {
						t.Error("GenKit provider should be initialized")
					}
					if engine.mcpConnManager == nil {
						t.Error("MCP connection manager should be initialized")
					}
					if engine.deploymentContextService == nil {
						t.Error("Deployment context service should be initialized")
					}
				}
			}
		})
	}
}

// TestNewAgentExecutionEngineWithLighthouse tests engine creation with Lighthouse
func TestNewAgentExecutionEngineWithLighthouse(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)

	t.Run("Engine with Lighthouse client", func(t *testing.T) {
		engine := NewAgentExecutionEngineWithLighthouse(repos, agentService, nil)

		if engine == nil {
			t.Fatal("Engine should not be nil")
		}

		if engine.lighthouseClient != nil {
			t.Error("Lighthouse client should be nil when nil passed")
		}
	})

	t.Run("Engine without Lighthouse", func(t *testing.T) {
		engine := NewAgentExecutionEngineWithLighthouse(repos, agentService, nil)

		if engine == nil {
			t.Fatal("Engine should not be nil")
		}

		// Verify other components still initialized
		if engine.genkitProvider == nil {
			t.Error("GenKit provider should be initialized")
		}
		if engine.mcpConnManager == nil {
			t.Error("MCP connection manager should be initialized")
		}
	})
}

// TestGetGenkitProvider tests getting the GenKit provider
func TestGetGenkitProvider(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	provider := engine.GetGenkitProvider()

	if provider == nil {
		t.Error("GetGenkitProvider() should return non-nil provider")
	}

	// Should return the same instance
	provider2 := engine.GetGenkitProvider()
	if provider != provider2 {
		t.Error("GetGenkitProvider() should return same instance")
	}
}

// TestConvertToolCalls tests tool call conversion
func TestConvertToolCalls(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	tests := []struct {
		name      string
		toolCalls *models.JSONArray
		wantCount int
		wantErr   bool
	}{
		{
			name:      "Nil tool calls",
			toolCalls: nil,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "Empty tool calls",
			toolCalls: &models.JSONArray{},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.convertToolCalls(tt.toolCalls)

			if len(result) != tt.wantCount {
				t.Errorf("convertToolCalls() count = %d, want %d", len(result), tt.wantCount)
			}
		})
	}
}

// TestConvertExecutionSteps tests execution step conversion
func TestConvertExecutionSteps(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	tests := []struct {
		name      string
		steps     *models.JSONArray
		wantCount int
	}{
		{
			name:      "Nil steps",
			steps:     nil,
			wantCount: 0,
		},
		{
			name:      "Empty steps",
			steps:     &models.JSONArray{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.convertExecutionSteps(tt.steps)

			if len(result) != tt.wantCount {
				t.Errorf("convertExecutionSteps() count = %d, want %d", len(result), tt.wantCount)
			}
		})
	}
}

// TestConvertTokenUsage tests token usage conversion
func TestConvertTokenUsage(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	tests := []struct {
		name  string
		usage map[string]interface{}
		isNil bool
	}{
		{
			name:  "Nil usage",
			usage: nil,
			isNil: true,
		},
		{
			name:  "Empty usage",
			usage: map[string]interface{}{},
			isNil: false,
		},
		{
			name: "Valid usage with input/output tokens",
			usage: map[string]interface{}{
				"inputTokens":  100,
				"outputTokens": 50,
			},
			isNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.convertTokenUsage(tt.usage)

			if tt.isNil {
				if result != nil {
					t.Errorf("convertTokenUsage() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Error("convertTokenUsage() = nil, want non-nil")
				}
			}
		})
	}
}

// TestExecuteAgent tests basic agent execution (will fail without proper setup)
func TestExecuteAgent(t *testing.T) {
	// Skip if no API key (GenKit will panic)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	// Create a minimal agent
	agent := &models.Agent{
		ID:            1,
		Name:          "test-agent",
		Description:   "Test agent",
		Prompt:        "You are a test agent. {{ userInput }}",
		MaxSteps:      1,
		EnvironmentID: 1,
		CreatedBy:     1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	t.Run("Execute with nil agent", func(t *testing.T) {
		// FIXED: ExecuteAgent now returns error instead of panicking
		_, err := engine.ExecuteAgent(ctx, nil, "test task", 1)
		if err == nil {
			t.Error("ExecuteAgent() with nil agent should return error")
		}
		if err != nil && err.Error() != "agent cannot be nil" {
			t.Errorf("ExecuteAgent() error = %v, want 'agent cannot be nil'", err)
		}
	})

	t.Run("Execute with empty task", func(t *testing.T) {
		_, err := engine.ExecuteAgent(ctx, agent, "", 1)
		// May succeed or fail depending on dotprompt handling
		t.Logf("Execute with empty task: err=%v", err)
	})

	t.Run("Execute with invalid runID", func(t *testing.T) {
		_, err := engine.ExecuteAgent(ctx, agent, "test task", 0)
		// May succeed or fail depending on validation
		t.Logf("Execute with invalid runID: err=%v", err)
	})
}

// TestExecuteWithOptions tests execution with options
func TestExecuteWithOptions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	agent := &models.Agent{
		ID:            1,
		Name:          "test-agent",
		Description:   "Test agent",
		Prompt:        "You are a test agent. {{ userInput }}",
		MaxSteps:      1,
		EnvironmentID: 1,
		CreatedBy:     1,
	}

	ctx := context.Background()
	userVars := map[string]interface{}{"key": "value"}

	t.Run("Execute with skipLighthouse=true", func(t *testing.T) {
		_, err := engine.ExecuteWithOptions(ctx, agent, "test", 1, userVars, true)
		// Will likely fail without API key setup
		t.Logf("ExecuteWithOptions error: %v", err)
	})

	t.Run("Execute with skipLighthouse=false", func(t *testing.T) {
		_, err := engine.ExecuteWithOptions(ctx, agent, "test", 1, userVars, false)
		// Will likely fail without API key setup
		t.Logf("ExecuteWithOptions error: %v", err)
	})
}

// TestAgentExecutionResult tests result structure
func TestAgentExecutionResult(t *testing.T) {
	t.Run("Create empty result", func(t *testing.T) {
		result := &AgentExecutionResult{}

		if result.Success {
			t.Error("Empty result should have Success=false")
		}
		if result.Response != "" {
			t.Error("Empty result should have empty Response")
		}
		if result.Duration != 0 {
			t.Error("Empty result should have zero Duration")
		}
	})

	t.Run("Create populated result", func(t *testing.T) {
		result := &AgentExecutionResult{
			Success:    true,
			Response:   "test response",
			Duration:   time.Second,
			ModelName:  "gpt-4o-mini",
			StepsUsed:  3,
			ToolsUsed:  2,
			StepsTaken: 3,
		}

		if !result.Success {
			t.Error("Result should have Success=true")
		}
		if result.Response != "test response" {
			t.Errorf("Response = %q, want %q", result.Response, "test response")
		}
		if result.Duration != time.Second {
			t.Errorf("Duration = %v, want %v", result.Duration, time.Second)
		}
		if result.StepsUsed != 3 {
			t.Errorf("StepsUsed = %d, want 3", result.StepsUsed)
		}
		if result.ToolsUsed != 2 {
			t.Errorf("ToolsUsed = %d, want 2", result.ToolsUsed)
		}
	})
}

// Benchmark tests
func BenchmarkNewAgentExecutionEngine(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewAgentExecutionEngine(repos, agentService)
	}
}

func BenchmarkConvertToolCalls(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	toolCalls := &models.JSONArray{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.convertToolCalls(toolCalls)
	}
}

// TestConvertToAgentRun tests the convertToAgentRun function
func TestConvertToAgentRun(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)
	engine := NewAgentExecutionEngine(repos, agentService)

	outputSchema := `{"type":"object","properties":{"result":{"type":"string"}}}`
	outputPreset := "investigation"

	tests := []struct {
		name              string
		agent             *models.Agent
		task              string
		runID             int64
		result            *AgentExecutionResult
		expectStatus      string
		expectToolCalls   bool
		expectSteps       bool
		description       string
	}{
		{
			name: "Successful run with complete metadata",
			agent: &models.Agent{
				ID:                 1,
				Name:               "test-agent",
				OutputSchema:       &outputSchema,
				OutputSchemaPreset: &outputPreset,
			},
			task:  "Complete a test task",
			runID: 100,
			result: &AgentExecutionResult{
				Success:   true,
				Response:  "Task completed successfully",
				Duration:  time.Second * 5,
				ModelName: "gpt-4o",
				StepsUsed: 3,
				ToolsUsed: 2,
				ToolCalls: &models.JSONArray{
					map[string]interface{}{
						"tool":   "test_tool",
						"result": "success",
					},
				},
				ExecutionSteps: &models.JSONArray{
					map[string]interface{}{
						"step": 1,
						"action": "analyze",
					},
				},
				TokenUsage: map[string]interface{}{
					"input_tokens":  int64(100),
					"output_tokens": int64(50),
				},
			},
			expectStatus:    "completed",
			expectToolCalls: true,
			expectSteps:     true,
			description:     "Should convert successful run with all metadata",
		},
		{
			name: "Failed run",
			agent: &models.Agent{
				ID:   2,
				Name: "failing-agent",
			},
			task:  "Task that failed",
			runID: 101,
			result: &AgentExecutionResult{
				Success:   false,
				Response:  "Task failed due to error",
				Duration:  time.Second * 2,
				ModelName: "gpt-4o-mini",
				StepsUsed: 1,
				ToolsUsed: 0,
			},
			expectStatus:    "failed",
			expectToolCalls: false,
			expectSteps:     false,
			description:     "Should handle failed run status",
		},
		{
			name: "Run with minimal metadata",
			agent: &models.Agent{
				ID:   3,
				Name: "minimal-agent",
			},
			task:  "Minimal task",
			runID: 102,
			result: &AgentExecutionResult{
				Success:   true,
				Response:  "Basic response",
				Duration:  time.Millisecond * 500,
				ModelName: "gpt-3.5-turbo",
				StepsUsed: 1,
				ToolsUsed: 0,
			},
			expectStatus:    "completed",
			expectToolCalls: false,
			expectSteps:     false,
			description:     "Should handle minimal execution result",
		},
		{
			name: "Run with output schema and preset",
			agent: &models.Agent{
				ID:                 4,
				Name:               "schema-agent",
				OutputSchema:       &outputSchema,
				OutputSchemaPreset: &outputPreset,
			},
			task:  "Structured output task",
			runID: 103,
			result: &AgentExecutionResult{
				Success:   true,
				Response:  `{"result": "structured data"}`,
				Duration:  time.Second * 3,
				ModelName: "gpt-4o",
				StepsUsed: 2,
				ToolsUsed: 1,
			},
			expectStatus:    "completed",
			expectToolCalls: false,
			expectSteps:     false,
			description:     "Should include output schema and preset in result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Now()
			agentRun := engine.convertToAgentRun(tt.agent, tt.task, tt.runID, startTime, tt.result)

			// Verify basic fields
			require.NotNil(t, agentRun)
			assert.NotEmpty(t, agentRun.ID, "Run ID should not be empty")
			assert.Equal(t, fmt.Sprintf("agent_%d", tt.agent.ID), agentRun.AgentID)
			assert.Equal(t, tt.agent.Name, agentRun.AgentName)
			assert.Equal(t, tt.task, agentRun.Task)
			assert.Equal(t, tt.result.Response, agentRun.Response)
			assert.Equal(t, tt.expectStatus, agentRun.Status)
			assert.Equal(t, tt.result.ModelName, agentRun.ModelName)
			assert.Equal(t, tt.result.Duration.Milliseconds(), agentRun.DurationMs)

			// Verify timestamps
			assert.Equal(t, startTime, agentRun.StartedAt)
			assert.Equal(t, startTime.Add(tt.result.Duration), agentRun.CompletedAt)

			// Verify metadata
			require.NotNil(t, agentRun.Metadata)
			assert.Equal(t, fmt.Sprintf("%d", tt.result.StepsUsed), agentRun.Metadata["steps_used"])
			assert.Equal(t, fmt.Sprintf("%d", tt.result.ToolsUsed), agentRun.Metadata["tools_used"])
			assert.Equal(t, fmt.Sprintf("%d", tt.runID), agentRun.Metadata["run_id"])
			assert.Equal(t, fmt.Sprintf("%d", tt.runID), agentRun.Metadata["station_run_id"])
			assert.NotEmpty(t, agentRun.Metadata["run_uuid"])

			// Verify tool calls
			if tt.expectToolCalls {
				assert.NotNil(t, agentRun.ToolCalls, "Tool calls should not be nil")
			}

			// Verify execution steps
			if tt.expectSteps {
				assert.NotNil(t, agentRun.ExecutionSteps, "Execution steps should not be nil")
			}

			// Verify output schema fields
			if tt.agent.OutputSchema != nil {
				assert.Equal(t, *tt.agent.OutputSchema, agentRun.OutputSchema)
			} else {
				assert.Empty(t, agentRun.OutputSchema)
			}

			if tt.agent.OutputSchemaPreset != nil {
				assert.Equal(t, *tt.agent.OutputSchemaPreset, agentRun.OutputSchemaPreset)
			} else {
				assert.Empty(t, agentRun.OutputSchemaPreset)
			}

			t.Logf("Converted run: ID=%s, status=%s, duration=%dms",
				agentRun.ID, agentRun.Status, agentRun.DurationMs)
		})
	}
}

// TestSendStructuredDataIfEligible tests the sendStructuredDataIfEligible function
func TestSendStructuredDataIfEligible(t *testing.T) {
	testDB, err := db.NewTest(t)
	require.NoError(t, err)
	defer testDB.Close()

	repos := repositories.New(testDB)
	agentService := NewAgentService(repos)

	outputSchema := `{"type":"object","properties":{"findings":{"type":"array"}}}`
	investigationPreset := "investigation"

	tests := []struct {
		name             string
		agent            *models.Agent
		result           *AgentExecutionResult
		runID            int64
		withLighthouse   bool
		description      string
	}{
		{
			name: "Agent with user-defined schema and app metadata",
			agent: &models.Agent{
				ID:           1,
				Name:         "security-agent",
				OutputSchema: &outputSchema,
			},
			result: &AgentExecutionResult{
				Success:  true,
				Response: `{"findings": []}`,
				App:      "investigations",
				AppType:  "security",
			},
			runID:          100,
			withLighthouse: false,
			description:    "Should handle agent with user-defined schema",
		},
		{
			name: "Agent with preset-based app/app_type",
			agent: &models.Agent{
				ID:                 2,
				Name:               "investigation-agent",
				OutputSchemaPreset: &investigationPreset,
			},
			result: &AgentExecutionResult{
				Success:  true,
				Response: `{"findings": []}`,
			},
			runID:          101,
			withLighthouse: false,
			description:    "Should extract app/app_type from preset",
		},
		{
			name: "Agent without app metadata",
			agent: &models.Agent{
				ID:   3,
				Name: "generic-agent",
			},
			result: &AgentExecutionResult{
				Success:  true,
				Response: "Generic response",
			},
			runID:          102,
			withLighthouse: false,
			description:    "Should skip when no app/app_type found",
		},
		{
			name: "Agent with app metadata but no schema",
			agent: &models.Agent{
				ID:   4,
				Name: "unstructured-agent",
			},
			result: &AgentExecutionResult{
				Success:  true,
				Response: "Unstructured response",
				App:      "investigations",
				AppType:  "security",
			},
			runID:          103,
			withLighthouse: false,
			description:    "Should skip when no output schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var engine *AgentExecutionEngine
			if tt.withLighthouse {
				// Would need mock lighthouse client for full testing
				engine = NewAgentExecutionEngine(repos, agentService)
			} else {
				engine = NewAgentExecutionEngine(repos, agentService)
			}

			contextLabels := map[string]string{
				"environment": "test",
			}

			// This function doesn't return anything, just testing it doesn't panic
			engine.sendStructuredDataIfEligible(tt.agent, tt.result, tt.runID, contextLabels)

			t.Logf("sendStructuredDataIfEligible completed for agent %d (runID: %d)",
				tt.agent.ID, tt.runID)
		})
	}
}
