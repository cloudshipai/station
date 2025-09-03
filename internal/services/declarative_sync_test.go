package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

func TestDeclarativeSyncFunctionality(t *testing.T) {
	// Skip if in CI without proper setup
	if os.Getenv("CI") == "true" && os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("Skipping integration test in CI")
	}

	testDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", testDir)

	t.Run("DeclarativeSync_ScanAgentFiles", func(t *testing.T) {
		// Test that DeclarativeSync can scan and process agent files
		envName := "scan-test"
		
		// Create environment structure
		configDir := filepath.Join(testDir, ".config", "station", "environments", envName)
		agentsDir := filepath.Join(configDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		// Create test database
		dbPath := filepath.Join(testDir, "test.db")
		cfg := &config.Config{
			DatabaseURL: dbPath,
		}

		database, err := db.New(cfg.DatabaseURL)
		require.NoError(t, err)
		defer database.Close()

		repos := repositories.New(database)

		// Create environment in database
		env, err := repos.Environments.Create(envName, nil, 1)
		require.NoError(t, err)

		// Create test agent files
		agents := []struct {
			name        string
			description string
			model       string
		}{
			{"scan-agent-1", "First test agent", "gpt-4"},
			{"scan-agent-2", "Second test agent", "gpt-3.5-turbo"},
			{"scan-agent-3", "Third test agent", "gemini-pro"},
		}

		for _, agent := range agents {
			agentContent := `---
metadata:
  name: "` + agent.name + `"
  description: "` + agent.description + `"
model: "` + agent.model + `"
tools: []
---

{{role "system"}}
You are ` + agent.description + `.

{{role "user"}}
{{userInput}}
`
			agentFile := filepath.Join(agentsDir, agent.name+".prompt")
			err = os.WriteFile(agentFile, []byte(agentContent), 0644)
			require.NoError(t, err)
		}

		// Create template.json
		templateContent := `{
  "name": "scan-test",
  "description": "Test environment for scanning",
  "mcpServers": {}
}`
		templateFile := filepath.Join(configDir, "template.json")
		err = os.WriteFile(templateFile, []byte(templateContent), 0644)
		require.NoError(t, err)

		// Test DeclarativeSync
		syncer := NewDeclarativeSync(repos, cfg)
		result, err := syncer.SyncEnvironment(context.Background(), envName, SyncOptions{
			DryRun:      false,
			Validate:    false,
			Interactive: false,
			Verbose:     true,
			Confirm:     false,
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
		
		// Verify agents were processed
		assert.Equal(t, 3, result.AgentsProcessed)
		assert.Equal(t, 3, result.AgentsSynced)

		// Verify agents exist in database
		dbAgents, err := repos.Agents.ListByEnvironment(env.ID)
		require.NoError(t, err)
		assert.Len(t, dbAgents, 3)

		// Verify agent details
		agentsByName := make(map[string]*models.Agent)
		for _, agent := range dbAgents {
			agentsByName[agent.Name] = agent
		}

		for _, expectedAgent := range agents {
			dbAgent, exists := agentsByName[expectedAgent.name]
			require.True(t, exists, "Agent %s should exist in database", expectedAgent.name)
			assert.Equal(t, expectedAgent.description, dbAgent.Description)
			assert.Equal(t, env.ID, dbAgent.EnvironmentID)
		}
	})

	t.Run("DeclarativeSync_DryRun", func(t *testing.T) {
		// Test dry run functionality
		envName := "dryrun-test"
		
		configDir := filepath.Join(testDir, ".config", "station", "environments", envName)
		agentsDir := filepath.Join(configDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		dbPath := filepath.Join(testDir, "dryrun.db")
		cfg := &config.Config{
			DatabaseURL: dbPath,
		}

		database, err := db.New(cfg.DatabaseURL)
		require.NoError(t, err)
		defer database.Close()

		repos := repositories.New(database)

		// Create environment in database
		env, err := repos.Environments.Create(envName, nil, 1)
		require.NoError(t, err)

		// Create agent file
		agentContent := `---
metadata:
  name: "dryrun-agent"
  description: "Agent for dry run testing"
model: "gpt-4"
tools: []
---

{{role "system"}}
You are a dry run test agent.

{{role "user"}}
{{userInput}}
`
		agentFile := filepath.Join(agentsDir, "dryrun-agent.prompt")
		err = os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Create template.json
		templateContent := `{
  "name": "dryrun-test",
  "description": "Dry run test environment",
  "mcpServers": {}
}`
		templateFile := filepath.Join(configDir, "template.json")
		err = os.WriteFile(templateFile, []byte(templateContent), 0644)
		require.NoError(t, err)

		// Test dry run
		syncer := NewDeclarativeSync(repos, cfg)
		result, err := syncer.SyncEnvironment(context.Background(), envName, SyncOptions{
			DryRun:      true, // DRY RUN MODE
			Validate:    false,
			Interactive: false,
			Verbose:     true,
			Confirm:     false,
		})

		require.NoError(t, err)
		assert.NotNil(t, result)
		
		// In dry run, agents should be processed but not actually synced to database
		assert.Equal(t, 1, result.AgentsProcessed)

		// Verify NO agents were actually created in database (dry run)
		dbAgents, err := repos.Agents.ListByEnvironment(env.ID)
		require.NoError(t, err)
		assert.Empty(t, dbAgents, "Dry run should not create agents in database")
	})

	t.Run("DeclarativeSync_ValidationMode", func(t *testing.T) {
		// Test validation-only mode
		envName := "validation-test"
		
		configDir := filepath.Join(testDir, ".config", "station", "environments", envName)
		agentsDir := filepath.Join(configDir, "agents")
		err := os.MkdirAll(agentsDir, 0755)
		require.NoError(t, err)

		dbPath := filepath.Join(testDir, "validation.db")
		cfg := &config.Config{
			DatabaseURL: dbPath,
		}

		database, err := db.New(cfg.DatabaseURL)
		require.NoError(t, err)
		defer database.Close()

		repos := repositories.New(database)

		// Create agent with potential validation issues
		agentContent := `---
metadata:
  name: "validation-agent"
  description: "Agent for validation testing"
model: "gpt-4"
tools:
  - "nonexistent_tool"  # This tool doesn't exist
---

{{role "system"}}
You are a validation test agent.

{{role "user"}}
{{userInput}}
`
		agentFile := filepath.Join(agentsDir, "validation-agent.prompt")
		err = os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Create template.json
		templateContent := `{
  "name": "validation-test",
  "description": "Validation test environment",
  "mcpServers": {}
}`
		templateFile := filepath.Join(configDir, "template.json")
		err = os.WriteFile(templateFile, []byte(templateContent), 0644)
		require.NoError(t, err)

		// Test validation mode
		syncer := NewDeclarativeSync(repos, cfg)
		result, err := syncer.SyncEnvironment(context.Background(), envName, SyncOptions{
			DryRun:      false,
			Validate:    true, // VALIDATION MODE
			Interactive: false,
			Verbose:     true,
			Confirm:     false,
		})

		// Validation should not fail the entire operation
		require.NoError(t, err)
		assert.NotNil(t, result)
		
		// Should process files for validation
		assert.Equal(t, 1, result.AgentsProcessed)
	})
}