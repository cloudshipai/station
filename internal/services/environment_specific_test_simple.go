package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/telemetry"
)

func TestEnvironmentSpecificAgents_Simple(t *testing.T) {
	// Create test database
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()

	err = database.Migrate()
	require.NoError(t, err)

	// Create repositories
	repos := repositories.New(database)

	// Create test data directly with proper signatures
	user, err := repos.Users.Create("testuser", "test-public-key", false, nil)
	require.NoError(t, err)

	desc := "Test Environment"
	env, err := repos.Environments.Create("test-env", &desc, user.ID)
	require.NoError(t, err)

	// Create GenkitService with minimal dependencies for testing
	telemetryService := telemetry.NewTelemetryService(false)
	genkitService := &GenkitService{
		agentRepo:        repos.Agents,
		agentRunRepo:     repos.AgentRuns,
		agentToolRepo:    repos.AgentTools,
		mcpToolRepo:      repos.MCPTools,
		environmentRepo:  repos.Environments,
		telemetryService: telemetryService,
	}

	t.Run("ListAgentsByEnvironment works correctly", func(t *testing.T) {
		// Should initially be empty
		agents, err := genkitService.ListAgentsByEnvironment(context.Background(), env.ID)
		require.NoError(t, err)
		assert.Len(t, agents, 0)
	})

	t.Run("CreateAgent stores agent in correct environment", func(t *testing.T) {
		config := &AgentConfig{
			EnvironmentID: env.ID,
			Name:          "test-agent",
			Description:   "Test agent",
			Prompt:        "You are a test agent",
			MaxSteps:      5,
			CreatedBy:     user.ID,
		}

		agent, err := genkitService.CreateAgent(context.Background(), config)
		require.NoError(t, err)
		assert.Equal(t, env.ID, agent.EnvironmentID)
		assert.Equal(t, "test-agent", agent.Name)

		// Verify it shows up in environment listing
		agents, err := genkitService.ListAgentsByEnvironment(context.Background(), env.ID)
		require.NoError(t, err)
		assert.Len(t, agents, 1)
		assert.Equal(t, agent.ID, agents[0].ID)
	})
}