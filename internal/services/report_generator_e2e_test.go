package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"station/internal/db"
	"station/internal/db/queries"
	"station/internal/db/repositories"
	"station/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReportsRepositoryE2E tests the reports repository CRUD operations
func TestReportsRepositoryE2E(t *testing.T) {
	ctx := context.Background()

	// Setup test database
	testDB := setupTestDatabase(t)
	defer testDB.Close()

	repos := repositories.New(testDB)

	// Create test environment
	desc := "Test Environment for Reports"
	env, err := repos.Environments.Create("test-reports-env", &desc, 1)
	require.NoError(t, err, "Failed to create test environment")
	t.Logf("✅ Created test environment: %d", env.ID)

	// Create test user
	user, err := repos.Users.Create("test-user", "ssh-key-123", false, nil)
	require.NoError(t, err, "Failed to create test user")
	t.Logf("✅ Created test user: %d", user.ID)

	// Create test agents
	agent1 := createTestAgent(t, ctx, repos, env.ID, user.ID, "cost-analyzer", "Analyzes AWS costs")
	agent2 := createTestAgent(t, ctx, repos, env.ID, user.ID, "security-scanner", "Scans for security issues")
	agent3 := createTestAgent(t, ctx, repos, env.ID, user.ID, "performance-monitor", "Monitors performance")
	t.Logf("✅ Created 3 test agents")

	// Create test runs for each agent
	createTestRuns(t, ctx, repos, agent1.ID, user.ID, 5) // 5 successful runs
	createTestRuns(t, ctx, repos, agent2.ID, user.ID, 3) // 3 successful runs
	createFailedRun(t, ctx, repos, agent3.ID, user.ID)   // 1 failed run
	createTestRuns(t, ctx, repos, agent3.ID, user.ID, 2) // 2 successful runs
	t.Logf("✅ Created test runs (11 total)")

	// Test 1: Create Report
	t.Run("Create Report", func(t *testing.T) {
		teamCriteria := map[string]interface{}{
			"goal": "Evaluate overall environment performance and reliability",
			"criteria": map[string]interface{}{
				"reliability": map[string]interface{}{
					"weight":      0.4,
					"description": "How reliable are the agents?",
					"threshold":   7.0,
				},
				"performance": map[string]interface{}{
					"weight":      0.3,
					"description": "How fast and efficient are the agents?",
					"threshold":   6.0,
				},
				"completeness": map[string]interface{}{
					"weight":      0.3,
					"description": "Do agents provide complete answers?",
					"threshold":   7.0,
				},
			},
		}
		teamCriteriaJSON, _ := json.Marshal(teamCriteria)

		report, err := repos.Reports.CreateReport(ctx, queries.CreateReportParams{
			Name:          "Test Environment Evaluation",
			Description:   sql.NullString{String: "E2E test of report generation", Valid: true},
			EnvironmentID: env.ID,
			TeamCriteria:  string(teamCriteriaJSON),
			AgentCriteria: sql.NullString{},
			JudgeModel:    sql.NullString{String: "gpt-4o-mini", Valid: true},
		})
		require.NoError(t, err, "Failed to create report")

		assert.Equal(t, "pending", report.Status)
		assert.Equal(t, int64(0), report.Progress.Int64)
		t.Logf("✅ Created report: %d with status=%s", report.ID, report.Status)
	})

	// Test 2: Update Report Status
	t.Run("Update Report Status", func(t *testing.T) {
		teamCriteriaJSON, _ := json.Marshal(map[string]interface{}{"goal": "test"})

		report, err := repos.Reports.CreateReport(ctx, queries.CreateReportParams{
			Name:          "Status Test Report",
			EnvironmentID: env.ID,
			TeamCriteria:  string(teamCriteriaJSON),
			JudgeModel:    sql.NullString{String: "gpt-4o-mini", Valid: true},
		})
		require.NoError(t, err)

		// Update status
		err = repos.Reports.UpdateStatus(ctx, queries.UpdateReportStatusParams{
			ID:          report.ID,
			Status:      "generating_team",
			Progress:    sql.NullInt64{Int64: 25, Valid: true},
			CurrentStep: sql.NullString{String: "Evaluating team performance", Valid: true},
		})
		require.NoError(t, err)

		// Verify update
		updated, err := repos.Reports.GetByID(ctx, report.ID)
		require.NoError(t, err)
		assert.Equal(t, "generating_team", updated.Status)
		assert.Equal(t, int64(25), updated.Progress.Int64)
		t.Logf("✅ Updated report status to: %s (progress=%d%%)", updated.Status, updated.Progress.Int64)
	})

	// Test 3: Create Agent Report Details
	t.Run("Agent Report Details", func(t *testing.T) {
		teamCriteriaJSON, _ := json.Marshal(map[string]interface{}{"goal": "test"})

		report, err := repos.Reports.CreateReport(ctx, queries.CreateReportParams{
			Name:          "Agent Details Test",
			EnvironmentID: env.ID,
			TeamCriteria:  string(teamCriteriaJSON),
			JudgeModel:    sql.NullString{String: "gpt-4o-mini", Valid: true},
		})
		require.NoError(t, err)

		// Create agent report detail
		criteriaScores := map[string]interface{}{
			"accuracy": map[string]interface{}{
				"score":     8.5,
				"reasoning": "Agent provides accurate results",
			},
		}
		criteriaScoresJSON, _ := json.Marshal(criteriaScores)

		detail, err := repos.Reports.CreateAgentReportDetail(ctx, queries.CreateAgentReportDetailParams{
			ReportID:           report.ID,
			AgentID:            agent1.ID,
			AgentName:          agent1.Name,
			Score:              8.5,
			Passed:             true,
			Reasoning:          sql.NullString{String: "Excellent performance", Valid: true},
			CriteriaScores:     sql.NullString{String: string(criteriaScoresJSON), Valid: true},
			RunsAnalyzed:       sql.NullInt64{Int64: 5, Valid: true},
			RunIds:             sql.NullString{String: "1,2,3,4,5", Valid: true},
			AvgDurationSeconds: sql.NullFloat64{Float64: 2.5, Valid: true},
			AvgTokens:          sql.NullInt64{Int64: 500, Valid: true},
			AvgCost:            sql.NullFloat64{Float64: 0.002, Valid: true},
			SuccessRate:        sql.NullFloat64{Float64: 1.0, Valid: true},
			Strengths:          sql.NullString{String: `["Fast","Accurate"]`, Valid: true},
			Weaknesses:         sql.NullString{String: `["Needs context"]`, Valid: true},
			Recommendations:    sql.NullString{String: `["Add history"]`, Valid: true},
			TelemetrySummary:   sql.NullString{},
		})
		require.NoError(t, err)
		assert.Equal(t, agent1.ID, detail.AgentID)
		assert.Equal(t, 8.5, detail.Score)
		t.Logf("✅ Created agent detail: agent=%s score=%.1f", detail.AgentName, detail.Score)
	})

	// Test 4: Complete Report
	t.Run("Complete Report", func(t *testing.T) {
		teamCriteriaJSON, _ := json.Marshal(map[string]interface{}{"goal": "test"})

		report, err := repos.Reports.CreateReport(ctx, queries.CreateReportParams{
			Name:          "Complete Test Report",
			EnvironmentID: env.ID,
			TeamCriteria:  string(teamCriteriaJSON),
			JudgeModel:    sql.NullString{String: "gpt-4o-mini", Valid: true},
		})
		require.NoError(t, err)

		// Complete report
		agentReportsJSON, _ := json.Marshal(map[string]interface{}{
			"1": map[string]interface{}{"score": 8.5, "summary": "Good"},
		})

		err = repos.Reports.CompleteReport(ctx, queries.CompleteReportParams{
			ID:                        report.ID,
			GenerationDurationSeconds: sql.NullFloat64{Float64: 15.5, Valid: true},
			TotalRunsAnalyzed:         sql.NullInt64{Int64: 11, Valid: true},
			TotalAgentsAnalyzed:       sql.NullInt64{Int64: 3, Valid: true},
			TotalLlmTokens:            sql.NullInt64{Int64: 5000, Valid: true},
			TotalLlmCost:              sql.NullFloat64{Float64: 0.015, Valid: true},
			AgentReports:              sql.NullString{String: string(agentReportsJSON), Valid: true},
		})
		require.NoError(t, err)

		// Verify completion
		completed, err := repos.Reports.GetByID(ctx, report.ID)
		require.NoError(t, err)
		assert.Equal(t, "completed", completed.Status)
		assert.Equal(t, int64(100), completed.Progress.Int64)
		assert.Equal(t, int64(11), completed.TotalRunsAnalyzed.Int64)
		t.Logf("✅ Completed report: runs=%d agents=%d duration=%.1fs",
			completed.TotalRunsAnalyzed.Int64,
			completed.TotalAgentsAnalyzed.Int64,
			completed.GenerationDurationSeconds.Float64)
	})

	// Test 5: GetRecentRunsByAgent
	t.Run("GetRecentRunsByAgent", func(t *testing.T) {
		recentRuns, err := repos.AgentRuns.GetRecentByAgent(ctx, agent1.ID, 3)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(recentRuns), 3)
		assert.GreaterOrEqual(t, len(recentRuns), 1)
		t.Logf("✅ GetRecentByAgent returned %d runs (limit=3)", len(recentRuns))
	})

	// Test 6: GetByEnvironment
	t.Run("GetByEnvironment", func(t *testing.T) {
		agents, err := repos.Agents.GetByEnvironment(ctx, env.ID)
		require.NoError(t, err)
		assert.Equal(t, 3, len(agents))
		t.Logf("✅ GetByEnvironment returned %d agents", len(agents))
	})
}

// Helper functions

func setupTestDatabase(t *testing.T) db.Database {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_reports.db")

	database, err := db.New(dbPath)
	require.NoError(t, err, "Failed to create test database")

	// Run migrations
	err = database.Migrate()
	require.NoError(t, err, "Failed to run migrations")

	t.Logf("✅ Test database created: %s", dbPath)
	return database
}

func createTestAgent(t *testing.T, ctx context.Context, repos *repositories.Repositories, envID, userID int64, name, description string) *models.Agent {
	agent, err := repos.Agents.Create(
		name,
		description,
		"You are a helpful agent",
		5, // maxSteps
		envID,
		userID,
		nil,   // inputSchema
		nil,   // cronSchedule
		false, // scheduleEnabled
		nil,   // outputSchema
		nil,   // outputSchemaPreset
		"",    // app
		"",    // appType
	)
	require.NoError(t, err, "Failed to create test agent: %s", name)
	return agent
}

func createTestRuns(t *testing.T, ctx context.Context, repos *repositories.Repositories, agentID, userID int64, count int) {
	for i := 0; i < count; i++ {
		_, err := repos.AgentRuns.Create(ctx, agentID, userID,
			"test task",
			"test response",
			3,   // stepsTaken
			nil, // toolCalls
			nil, // executionSteps
			"completed",
			nil, // completedAt
		)
		require.NoError(t, err, "Failed to create test run")
	}
}

func createFailedRun(t *testing.T, ctx context.Context, repos *repositories.Repositories, agentID, userID int64) {
	run, err := repos.AgentRuns.Create(ctx, agentID, userID,
		"test task",
		"",
		0,
		nil,
		nil,
		"failed",
		nil,
	)
	require.NoError(t, err, "Failed to create failed run")

	// Update to failed status
	err = repos.AgentRuns.UpdateStatus(ctx, run.ID, "failed")
	require.NoError(t, err, "Failed to update run status")
}
