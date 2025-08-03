package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// TestFileMCPMigrationValidation tests that the migration to file-based configs is working correctly
func TestFileMCPMigrationValidation(t *testing.T) {
	// Create test database
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()

	err = database.Migrate()
	require.NoError(t, err)

	repos := repositories.New(database)

	t.Run("DatabaseMigrationCompleted", func(t *testing.T) {
		// Verify old tables are gone and new tables exist
		conn := database.Conn()

		// Check that old tables were removed
		oldTables := []string{"mcp_configs", "mcp_configs_backup", "template_variables", "config_migrations", "config_loading_preferences"}
		for _, table := range oldTables {
			var count int
			err := conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 0, count, "Table %s should have been removed by migration", table)
		}

		// Check that new file config table exists
		var count int
		err = conn.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='file_mcp_configs'").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "file_mcp_configs table should exist")

		t.Logf("✅ Database migration validation passed")
		t.Logf("   Removed %d old tables", len(oldTables))
		t.Logf("   Created file_mcp_configs table")
	})

	t.Run("RepositoryStructureUpdated", func(t *testing.T) {
		// Verify that repositories structure no longer has MCPConfigs
		assert.NotNil(t, repos.FileMCPConfigs, "FileMCPConfigs repository should exist")
		assert.NotNil(t, repos.MCPServers, "MCPServers repository should still exist")
		assert.NotNil(t, repos.MCPTools, "MCPTools repository should still exist")

		t.Logf("✅ Repository structure validation passed")
		t.Logf("   FileMCPConfigs repository: ✓")
		t.Logf("   MCPServers repository: ✓")
		t.Logf("   MCPTools repository: ✓")
	})

	t.Run("FileConfigOperations", func(t *testing.T) {
		// Create test user and environment
		user, err := repos.Users.Create("test-user", "test-key", false, nil)
		require.NoError(t, err)

		desc := "Test Environment"
		env, err := repos.Environments.Create("test-env", &desc, user.ID)
		require.NoError(t, err)

		// Test file config record operations
		fileConfig := &repositories.FileConfigRecord{
			EnvironmentID:            env.ID,
			ConfigName:               "migration-test-config",
			TemplatePath:             "/test/config.json",
			VariablesPath:            "/test/variables.yml",
			TemplateSpecificVarsPath: "/test/config.vars.yml",
			TemplateHash:             "hash123",
			VariablesHash:            "vhash456",
			TemplateVarsHash:         "tvhash789",
			Metadata:                 `{"migrated": true}`,
		}

		// Test create
		fileConfigID, err := repos.FileMCPConfigs.Create(fileConfig)
		require.NoError(t, err)
		assert.Greater(t, fileConfigID, int64(0))

		// Test retrieve
		retrieved, err := repos.FileMCPConfigs.GetByEnvironmentAndName(env.ID, "migration-test-config")
		require.NoError(t, err)
		assert.Equal(t, "migration-test-config", retrieved.ConfigName)
		assert.Equal(t, env.ID, retrieved.EnvironmentID)

		// Test list by environment
		configs, err := repos.FileMCPConfigs.ListByEnvironment(env.ID)
		require.NoError(t, err)
		assert.Len(t, configs, 1)
		assert.Equal(t, "migration-test-config", configs[0].ConfigName)

		t.Logf("✅ File config operations validation passed")
		t.Logf("   Created config ID: %d", fileConfigID)
		t.Logf("   Retrieved config: %s", retrieved.ConfigName)
		t.Logf("   Listed configs: %d", len(configs))
	})

	t.Run("ToolDiscoveryServiceUpdated", func(t *testing.T) {
		// Test that ToolDiscoveryService can be created with new signature
		toolDiscovery := NewToolDiscoveryService(repos)
		assert.NotNil(t, toolDiscovery)

		// Create test environment for discovery
		user, err := repos.Users.Create("discovery-user", "test-key", false, nil)
		require.NoError(t, err)

		desc := "Discovery Test Environment"
		env, err := repos.Environments.Create("discovery-env", &desc, user.ID)
		require.NoError(t, err)

		// Test that GetToolsByEnvironment works with file-based configs
		tools, err := toolDiscovery.GetToolsByEnvironment(env.ID)
		require.NoError(t, err)
		// Should be empty for new environment, but shouldn't error

		t.Logf("✅ Tool discovery service validation passed")
		t.Logf("   Service created successfully")
		t.Logf("   GetToolsByEnvironment returned %d tools", len(tools))
	})

	t.Run("ToolFileConfigLinking", func(t *testing.T) {
		// Create test data
		user, err := repos.Users.Create("tool-user", "test-key", false, nil)
		require.NoError(t, err)

		desc := "Tool Test Environment"
		env, err := repos.Environments.Create("tool-env", &desc, user.ID)
		require.NoError(t, err)

		// Create file config record
		fileConfig := &repositories.FileConfigRecord{
			EnvironmentID: env.ID,
			ConfigName:    "tool-test-config",
			TemplatePath:  "/test/tools.json",
		}
		fileConfigID, err := repos.FileMCPConfigs.Create(fileConfig)
		require.NoError(t, err)

		// Create MCP server
		server := &models.MCPServer{
			EnvironmentID: env.ID,
			Name:          "test-server",
			Command:       "echo",
			Args:          []string{"test"},
		}
		serverID, err := repos.MCPServers.Create(server)
		require.NoError(t, err)

		// Create tool linked to file config
		tool := &models.MCPTool{
			MCPServerID: serverID,
			Name:        "test-tool",
			Description: "Test tool for file config linking",
			Schema:      []byte(`{"type": "object"}`),
		}

		toolID, err := repos.MCPTools.CreateWithFileConfig(tool, fileConfigID)
		require.NoError(t, err)

		// Test retrieving tools by file config
		toolsByFileConfig, err := repos.MCPTools.GetByFileConfigID(fileConfigID)
		require.NoError(t, err)
		assert.Len(t, toolsByFileConfig, 1)
		assert.Equal(t, "test-tool", toolsByFileConfig[0].Name)

		// Test hybrid tool retrieval with file config info
		hybridTools, err := repos.MCPTools.GetToolsWithFileConfigInfo(env.ID)
		require.NoError(t, err)
		assert.Len(t, hybridTools, 1)

		t.Logf("✅ Tool file config linking validation passed")
		t.Logf("   Created tool ID: %d", toolID)
		t.Logf("   Tools linked to file config: %d", len(toolsByFileConfig))
		t.Logf("   Hybrid tools query returned: %d", len(hybridTools))
	})

	t.Run("ServiceIntegrationWithFileConfigs", func(t *testing.T) {
		// Test that services can work together with file-based configs
		toolDiscovery := NewToolDiscoveryService(repos)

		// Create test environment
		user, err := repos.Users.Create("integration-user", "test-key", false, nil)
		require.NoError(t, err)

		desc := "Integration Test Environment"
		env, err := repos.Environments.Create("integration-env", &desc, user.ID)
		require.NoError(t, err)

		// Create mock rendered config for tool discovery
		renderedConfig := &models.MCPConfigData{
			Name: "integration-test-config",
			Servers: map[string]models.MCPServerConfig{
				"mock-server": {
					Command: "echo",
					Args:    []string{"mock"},
					Env:     map[string]string{},
				},
			},
		}

		// Test DiscoverToolsFromFileConfig (the new method)
		result, err := toolDiscovery.DiscoverToolsFromFileConfig(env.ID, "integration-test-config", renderedConfig)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, env.ID, result.EnvironmentID)
		assert.Equal(t, "integration-test-config", result.ConfigName)

		t.Logf("✅ Service integration validation passed")
		t.Logf("   Discovery result: Environment %d, Config %s", result.EnvironmentID, result.ConfigName)
		t.Logf("   Success: %v, Servers: %d", result.Success, result.TotalServers)
	})
}

// TestFileMCPServiceCompatibility tests that all service interfaces are compatible after migration
func TestFileMCPServiceCompatibility(t *testing.T) {
	// Create test database
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()

	err = database.Migrate()
	require.NoError(t, err)

	repos := repositories.New(database)

	t.Run("AllServicesCanBeCreated", func(t *testing.T) {
		// Test that all services can be created with the new repository structure
		toolDiscovery := NewToolDiscoveryService(repos)
		assert.NotNil(t, toolDiscovery)

		webhookService := NewWebhookService(repos)
		assert.NotNil(t, webhookService)

		// Test that intelligent agent creator can be created with simplified signature
		intelligentCreator := NewIntelligentAgentCreator(repos, nil)
		assert.NotNil(t, intelligentCreator)

		t.Logf("✅ Service compatibility validation passed")
		t.Logf("   ToolDiscoveryService: ✓")
		t.Logf("   WebhookService: ✓")
		t.Logf("   IntelligentAgentCreator: ✓")
	})

	t.Run("ServiceMethodsStillWork", func(t *testing.T) {
		// Create test environment
		user, err := repos.Users.Create("method-user", "test-key", false, nil)
		require.NoError(t, err)

		desc := "Method Test Environment"
		env, err := repos.Environments.Create("method-env", &desc, user.ID)
		require.NoError(t, err)

		// Test ToolDiscoveryService methods
		toolDiscovery := NewToolDiscoveryService(repos)

		// GetToolsByEnvironment should work (even if empty)
		tools, err := toolDiscovery.GetToolsByEnvironment(env.ID)
		require.NoError(t, err)

		// GetHybridToolsByEnvironment should work
		hybridTools, err := toolDiscovery.GetHybridToolsByEnvironment(env.ID)
		require.NoError(t, err)

		t.Logf("✅ Service method compatibility validation passed")
		t.Logf("   GetToolsByEnvironment: returned %d tools", len(tools))
		t.Logf("   GetHybridToolsByEnvironment: returned %d tools", len(hybridTools))
	})
}