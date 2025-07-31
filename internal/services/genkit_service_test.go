package services

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"station/internal/db/repositories"
)

func TestGenkitService_Integration(t *testing.T) {
	// Skip if no OpenAI API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()

	// Initialize test database
	testDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Create basic test tables (simplified for testing)
	_, err = testDB.Exec(`
		CREATE TABLE agents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			prompt TEXT,
			max_steps INTEGER,
			environment_id INTEGER,
			created_by INTEGER,
			cron_schedule TEXT,
			is_scheduled BOOLEAN,
			schedule_enabled BOOLEAN,
			last_scheduled_run TEXT,
			next_scheduled_run TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE agent_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id INTEGER,
			user_id INTEGER,
			task TEXT,
			final_response TEXT,
			steps_taken INTEGER,
			tool_calls TEXT,
			execution_steps TEXT,
			status TEXT,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);
		
		CREATE TABLE mcp_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			environment_id INTEGER,
			config_name TEXT,
			version INTEGER,
			config_json TEXT,
			encryption_key_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	require.NoError(t, err)

	// Initialize repositories
	agentRepo := repositories.NewAgentRepo(testDB)
	agentRunRepo := repositories.NewAgentRunRepo(testDB)
	mcpConfigRepo := repositories.NewMCPConfigRepo(testDB)

	// Initialize Genkit with OpenAI plugin
	openaiPlugin := &oai.OpenAI{APIKey: apiKey}
	g, err := genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
	require.NoError(t, err)

	// Create Genkit service
	genkitService := NewGenkitService(g, openaiPlugin, agentRepo, agentRunRepo, mcpConfigRepo, 1)

	// Initialize MCP
	err = genkitService.InitializeMCP(ctx)
	require.NoError(t, err)

	t.Run("ExecuteAgent_BasicTask", func(t *testing.T) {
		// Create a test agent
		agent, err := agentRepo.Create(
			"test-genkit-agent",
			"A test agent for Genkit integration",
			"You are a helpful assistant. Keep responses brief and accurate.",
			10, // maxSteps
			1,  // environmentID
			1,  // createdBy
			nil, // cronSchedule
			false, // scheduleEnabled
		)
		require.NoError(t, err)

		// Execute the agent
		run, err := genkitService.ExecuteAgent(ctx, agent.ID, 1, "What is the capital of France?")
		require.NoError(t, err)

		// Verify the run
		assert.Equal(t, agent.ID, run.AgentID)
		assert.Equal(t, int64(1), run.UserID)
		assert.Equal(t, "What is the capital of France?", run.Task)
		assert.Equal(t, "completed", run.Status)
		assert.NotEmpty(t, run.FinalResponse)
		assert.Contains(t, run.FinalResponse, "Paris")
		assert.NotNil(t, run.CompletedAt)
		assert.Equal(t, int64(1), run.StepsTaken)

		t.Logf("Agent response: %s", run.FinalResponse)
	})

	t.Run("ExecuteAgent_WithSystemPrompt", func(t *testing.T) {
		// Create an agent with a specific system prompt
		agent, err := agentRepo.Create(
			"math-genkit-agent",
			"A math-focused agent",
			"You are a mathematics tutor. Always provide step-by-step solutions and show your work clearly.",
			10, // maxSteps
			1,  // environmentID
			1,  // createdBy
			nil, // cronSchedule
			false, // scheduleEnabled
		)
		require.NoError(t, err)

		// Execute the agent with a math problem
		run, err := genkitService.ExecuteAgent(ctx, agent.ID, 1, "Calculate 25 * 16")
		require.NoError(t, err)

		// Verify the run
		assert.Equal(t, "completed", run.Status)
		assert.NotEmpty(t, run.FinalResponse)
		assert.Contains(t, run.FinalResponse, "400")

		t.Logf("Math agent response: %s", run.FinalResponse)
	})

	t.Run("ExecuteAgent_InvalidAgent", func(t *testing.T) {
		// Try to execute non-existent agent
		run, err := genkitService.ExecuteAgent(ctx, 99999, 1, "Test task")
		assert.Error(t, err)
		assert.Nil(t, run)
		assert.Contains(t, err.Error(), "failed to get agent")
	})

	t.Run("GetAgentRun", func(t *testing.T) {
		// Create and execute an agent to get a run
		agent, err := agentRepo.Create(
			"retrieval-test-agent",
			"Agent for testing run retrieval",
			"You are a helpful assistant.",
			10, // maxSteps
			1,  // environmentID
			1,  // createdBy
			nil, // cronSchedule
			false, // scheduleEnabled
		)
		require.NoError(t, err)

		// Execute agent
		run, err := genkitService.ExecuteAgent(ctx, agent.ID, 1, "Hello, world!")
		require.NoError(t, err)

		// Retrieve the run
		retrievedRun, err := genkitService.GetAgentRun(run.ID)
		require.NoError(t, err)

		assert.Equal(t, run.ID, retrievedRun.ID)
		assert.Equal(t, run.AgentID, retrievedRun.AgentID)
		assert.Equal(t, run.UserID, retrievedRun.UserID)
		assert.Equal(t, run.Task, retrievedRun.Task)
		assert.Equal(t, run.Status, retrievedRun.Status)
	})

	t.Run("GetAgentRunsForAgent", func(t *testing.T) {
		// Create an agent
		agent, err := agentRepo.Create(
			"multi-run-agent",
			"Agent for testing multiple runs",
			"You are a helpful assistant.",
			10, // maxSteps
			1,  // environmentID
			1,  // createdBy
			nil, // cronSchedule
			false, // scheduleEnabled
		)
		require.NoError(t, err)

		// Execute agent multiple times
		tasks := []string{"Task 1", "Task 2", "Task 3"}
		var runIDs []int64
		
		for _, task := range tasks {
			run, err := genkitService.ExecuteAgent(ctx, agent.ID, 1, task)
			require.NoError(t, err)
			runIDs = append(runIDs, run.ID)
		}

		// Get runs for this agent
		runs, err := genkitService.GetAgentRunsForAgent(agent.ID, 10)
		require.NoError(t, err)

		assert.Equal(t, 3, len(runs))
		
		// All runs should be for this agent
		for _, run := range runs {
			assert.Equal(t, agent.ID, run.AgentID)
		}
	})

	t.Run("AddMCPServer", func(t *testing.T) {
		// Test adding an MCP server dynamically
		err := genkitService.AddMCPServer(ctx, "test-server", mcp.MCPClientOptions{
			Name:     "test-mcp-server",
			Version:  "1.0.0",
			Disabled: true, // Disable so we don't need actual server
			Stdio: &mcp.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		})
		
		require.NoError(t, err)

		// Get available tools (should be empty since server is disabled)
		tools, err := genkitService.GetAvailableTools(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(tools))
	})

	t.Run("Close", func(t *testing.T) {
		err := genkitService.Close(ctx)
		assert.NoError(t, err)
	})
}

func TestGenkitService_WithoutAPI(t *testing.T) {
	ctx := context.Background()

	// Initialize test database  
	testDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer testDB.Close()

	// Create basic test tables (simplified for testing)
	_, err = testDB.Exec(`
		CREATE TABLE agents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			prompt TEXT,
			max_steps INTEGER,
			environment_id INTEGER,
			created_by INTEGER,
			cron_schedule TEXT,
			is_scheduled BOOLEAN,
			schedule_enabled BOOLEAN,
			last_scheduled_run TEXT,
			next_scheduled_run TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE agent_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id INTEGER,
			user_id INTEGER,
			task TEXT,
			final_response TEXT,
			steps_taken INTEGER,
			tool_calls TEXT,
			execution_steps TEXT,
			status TEXT,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		);
		
		CREATE TABLE mcp_configs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			environment_id INTEGER,
			config_name TEXT,
			version INTEGER,
			config_json TEXT,
			encryption_key_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	require.NoError(t, err)

	// Initialize repositories
	agentRepo := repositories.NewAgentRepo(testDB)
	agentRunRepo := repositories.NewAgentRunRepo(testDB)
	mcpConfigRepo := repositories.NewMCPConfigRepo(testDB)

	// Initialize Genkit without real API key
	openaiPlugin := &oai.OpenAI{APIKey: "test-key"}
	g, err := genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
	require.NoError(t, err)

	// Create Genkit service
	genkitService := NewGenkitService(g, openaiPlugin, agentRepo, agentRunRepo, mcpConfigRepo, 1)

	t.Run("InitializeMCP", func(t *testing.T) {
		err := genkitService.InitializeMCP(ctx)
		assert.NoError(t, err)
	})

	t.Run("GetAvailableTools_Empty", func(t *testing.T) {
		tools, err := genkitService.GetAvailableTools(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(tools))
	})

	t.Run("ExecuteAgent_NoAPIKey", func(t *testing.T) {
		// Create a test agent
		agent, err := agentRepo.Create(
			"no-api-agent",
			"Agent for testing without API key",
			"You are a helpful assistant.",
			10, // maxSteps
			1,  // environmentID
			1,  // createdBy
			nil, // cronSchedule
			false, // scheduleEnabled
		)
		require.NoError(t, err)

		// This should fail due to invalid API key
		run, err := genkitService.ExecuteAgent(ctx, agent.ID, 1, "Test task")
		
		// The run should be created but marked as failed
		// Our service returns the original error when execution fails
		assert.Error(t, err)
		assert.NotNil(t, run) // Run should still be created
		assert.Equal(t, "failed", run.Status)
		assert.Contains(t, run.FinalResponse, "Error:")
	})
}