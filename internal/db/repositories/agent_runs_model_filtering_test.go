package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"station/internal/db"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDBForModelFiltering(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		if err := testDB.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}
	}

	return testDB.Conn(), cleanup
}

func createTestAgent(t *testing.T, agentRepo *AgentRepo, name string) int64 {
	t.Helper()

	agent, err := agentRepo.Create(
		name,
		"Test agent for model filtering",
		"System prompt for testing",
		5,     // maxSteps
		1,     // environmentID
		1,     // createdBy
		nil,   // inputSchema
		nil,   // cronSchedule
		false, // scheduleEnabled
		nil,   // outputSchema
		nil,   // outputSchemaPreset
		"",    // app
		"",    // appType
	)
	if err != nil {
		t.Fatalf("Failed to create test agent: %v", err)
	}
	return agent.ID
}

func createTestRunWithModel(t *testing.T, runRepo *AgentRunRepo, agentID, userID int64, modelName string, status string) int64 {
	t.Helper()

	ctx := context.Background()
	completedAt := time.Now()
	tokens := int64(100)
	duration := 1.5
	toolsUsed := int64(2)

	run, err := runRepo.CreateWithMetadata(
		ctx,
		agentID,
		userID,
		"Test task for "+modelName,
		"Test response",
		1,
		nil,
		nil,
		status,
		&completedAt,
		&tokens,
		&tokens,
		&tokens,
		&duration,
		&modelName,
		&toolsUsed,
		nil,
	)
	if err != nil {
		t.Fatalf("Failed to create test run: %v", err)
	}
	return run.ID
}

func TestGetRecentByAgentAndModel(t *testing.T) {
	testDB, cleanup := setupTestDBForModelFiltering(t)
	defer cleanup()

	agentRepo := NewAgentRepo(testDB)
	runRepo := NewAgentRunRepo(testDB)
	userRepo := NewUserRepo(testDB)

	// Create test user
	user, err := userRepo.Create("testuser", "test-public-key", false, nil)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test agent
	agentID := createTestAgent(t, agentRepo, "Model Filter Agent")

	// Create runs with different models
	gpt4oMiniRuns := []int64{
		createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o-mini", "completed"),
		createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o-mini", "completed"),
		createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o-mini", "completed"),
	}

	gpt4oRuns := []int64{
		createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o", "completed"),
		createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o", "completed"),
	}

	claudeRun := createTestRunWithModel(t, runRepo, agentID, user.ID, "anthropic/claude-3-opus", "completed")

	tests := []struct {
		name          string
		modelName     string
		limit         int64
		expectedCount int
		expectedIDs   []int64
	}{
		{
			name:          "Filter by gpt-4o-mini",
			modelName:     "openai/gpt-4o-mini",
			limit:         10,
			expectedCount: 3,
			expectedIDs:   gpt4oMiniRuns,
		},
		{
			name:          "Filter by gpt-4o",
			modelName:     "openai/gpt-4o",
			limit:         10,
			expectedCount: 2,
			expectedIDs:   gpt4oRuns,
		},
		{
			name:          "Filter by claude",
			modelName:     "anthropic/claude-3-opus",
			limit:         10,
			expectedCount: 1,
			expectedIDs:   []int64{claudeRun},
		},
		{
			name:          "Filter with limit",
			modelName:     "openai/gpt-4o-mini",
			limit:         2,
			expectedCount: 2,
			expectedIDs:   nil, // Don't check IDs, just count
		},
		{
			name:          "Non-existent model",
			modelName:     "openai/gpt-5",
			limit:         10,
			expectedCount: 0,
			expectedIDs:   []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs, err := runRepo.GetRecentByAgentAndModel(context.Background(), agentID, tt.modelName, tt.limit)
			if err != nil {
				t.Fatalf("GetRecentByAgentAndModel failed: %v", err)
			}

			if len(runs) != tt.expectedCount {
				t.Errorf("Expected %d runs, got %d", tt.expectedCount, len(runs))
			}

			// Verify all runs have the correct model
			for _, run := range runs {
				if run.ModelName.String != tt.modelName {
					t.Errorf("Expected model %s, got %s", tt.modelName, run.ModelName.String)
				}
			}

			// If expected IDs are provided, verify they match (order may differ due to ORDER BY)
			if tt.expectedIDs != nil && len(tt.expectedIDs) > 0 {
				gotIDs := make(map[int64]bool)
				for _, run := range runs {
					gotIDs[run.ID] = true
				}

				for _, expectedID := range tt.expectedIDs {
					if !gotIDs[expectedID] {
						t.Errorf("Expected run ID %d not found in results", expectedID)
					}
				}
			}
		})
	}
}

func TestListByModel(t *testing.T) {
	testDB, cleanup := setupTestDBForModelFiltering(t)
	defer cleanup()

	agentRepo := NewAgentRepo(testDB)
	runRepo := NewAgentRunRepo(testDB)
	userRepo := NewUserRepo(testDB)

	// Create test user
	user, err := userRepo.Create("testuser", "test-public-key", false, nil)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create multiple agents
	agent1ID := createTestAgent(t, agentRepo, "Agent 1")
	agent2ID := createTestAgent(t, agentRepo, "Agent 2")

	// Create runs across different agents with same model
	createTestRunWithModel(t, runRepo, agent1ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent1ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent2ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent1ID, user.ID, "openai/gpt-4o", "completed")

	tests := []struct {
		name          string
		modelName     string
		limit         int64
		offset        int64
		expectedCount int
	}{
		{
			name:          "List all gpt-4o-mini runs",
			modelName:     "openai/gpt-4o-mini",
			limit:         10,
			offset:        0,
			expectedCount: 3,
		},
		{
			name:          "List with pagination",
			modelName:     "openai/gpt-4o-mini",
			limit:         2,
			offset:        0,
			expectedCount: 2,
		},
		{
			name:          "List with offset",
			modelName:     "openai/gpt-4o-mini",
			limit:         10,
			offset:        2,
			expectedCount: 1,
		},
		{
			name:          "List gpt-4o runs",
			modelName:     "openai/gpt-4o",
			limit:         10,
			offset:        0,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs, err := runRepo.ListByModel(context.Background(), tt.modelName, tt.limit, tt.offset)
			if err != nil {
				t.Fatalf("ListByModel failed: %v", err)
			}

			if len(runs) != tt.expectedCount {
				t.Errorf("Expected %d runs, got %d", tt.expectedCount, len(runs))
			}

			// Verify all runs have the correct model and include agent details
			for _, run := range runs {
				if run.ModelName == nil || *run.ModelName != tt.modelName {
					t.Errorf("Expected model %s, got %v", tt.modelName, run.ModelName)
				}
				if run.AgentName == "" {
					t.Error("Expected AgentName to be populated")
				}
				if run.Username == "" {
					t.Error("Expected Username to be populated")
				}
			}
		})
	}
}

func TestListDistinctModels(t *testing.T) {
	testDB, cleanup := setupTestDBForModelFiltering(t)
	defer cleanup()

	agentRepo := NewAgentRepo(testDB)
	runRepo := NewAgentRunRepo(testDB)
	userRepo := NewUserRepo(testDB)

	// Create test user
	user, err := userRepo.Create("testuser", "test-public-key", false, nil)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test agent
	agentID := createTestAgent(t, agentRepo, "Multi-Model Agent")

	// Create runs with different models
	createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o", "completed")
	createTestRunWithModel(t, runRepo, agentID, user.ID, "openai/gpt-4o", "completed")
	createTestRunWithModel(t, runRepo, agentID, user.ID, "anthropic/claude-3-opus", "completed")

	// Get distinct models
	models, err := runRepo.ListDistinctModels(context.Background())
	if err != nil {
		t.Fatalf("ListDistinctModels failed: %v", err)
	}

	// Verify counts
	expectedModels := map[string]int{
		"openai/gpt-4o-mini":      3,
		"openai/gpt-4o":           2,
		"anthropic/claude-3-opus": 1,
	}

	if len(models) != len(expectedModels) {
		t.Errorf("Expected %d distinct models, got %d", len(expectedModels), len(models))
	}

	for modelName, expectedCount := range expectedModels {
		count, exists := models[modelName]
		if !exists {
			t.Errorf("Expected model %s not found in results", modelName)
			continue
		}
		if count != expectedCount {
			t.Errorf("For model %s: expected count %d, got %d", modelName, expectedCount, count)
		}
	}
}

func TestListDistinctModels_EmptyDatabase(t *testing.T) {
	testDB, cleanup := setupTestDBForModelFiltering(t)
	defer cleanup()

	runRepo := NewAgentRunRepo(testDB)

	// Get distinct models from empty database
	models, err := runRepo.ListDistinctModels(context.Background())
	if err != nil {
		t.Fatalf("ListDistinctModels failed: %v", err)
	}

	if len(models) != 0 {
		t.Errorf("Expected 0 models in empty database, got %d", len(models))
	}
}

func TestGetRecentByAgentAndModel_MultipleAgents(t *testing.T) {
	testDB, cleanup := setupTestDBForModelFiltering(t)
	defer cleanup()

	agentRepo := NewAgentRepo(testDB)
	runRepo := NewAgentRunRepo(testDB)
	userRepo := NewUserRepo(testDB)

	// Create test user
	user, err := userRepo.Create("testuser", "test-public-key", false, nil)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create two agents
	agent1ID := createTestAgent(t, agentRepo, "Agent 1")
	agent2ID := createTestAgent(t, agentRepo, "Agent 2")

	// Create runs for both agents with same model
	createTestRunWithModel(t, runRepo, agent1ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent1ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent2ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent2ID, user.ID, "openai/gpt-4o-mini", "completed")
	createTestRunWithModel(t, runRepo, agent2ID, user.ID, "openai/gpt-4o-mini", "completed")

	// Get runs for agent1 only
	runs, err := runRepo.GetRecentByAgentAndModel(context.Background(), agent1ID, "openai/gpt-4o-mini", 10)
	if err != nil {
		t.Fatalf("GetRecentByAgentAndModel failed: %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("Expected 2 runs for agent1, got %d", len(runs))
	}

	// Verify all runs belong to agent1
	for _, run := range runs {
		if run.AgentID != agent1ID {
			t.Errorf("Expected run for agent %d, got run for agent %d", agent1ID, run.AgentID)
		}
	}

	// Get runs for agent2
	runs, err = runRepo.GetRecentByAgentAndModel(context.Background(), agent2ID, "openai/gpt-4o-mini", 10)
	if err != nil {
		t.Fatalf("GetRecentByAgentAndModel failed: %v", err)
	}

	if len(runs) != 3 {
		t.Errorf("Expected 3 runs for agent2, got %d", len(runs))
	}
}
