package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDB creates a test database with basic schema
func setupTestDB(t *testing.T) (*db.DB, *repositories.Repositories) {
	// Create a temporary database file for testing
	tempFile := filepath.Join(t.TempDir(), "test.db")
	testDB, err := db.New(tempFile)
	require.NoError(t, err)
	
	// Run migrations to set up the database schema
	err = testDB.Migrate()
	require.NoError(t, err)
	
	repos := repositories.New(testDB)
	return testDB, repos
}

func TestMCPHandler_discoverConfigFiles(t *testing.T) {
	tests := []struct {
		name        string
		setupFiles  map[string]string // filename -> content
		environment string
		expectCount int
		expectNames []string
	}{
		{
			name: "discover_multiple_json_configs",
			setupFiles: map[string]string{
				"config1.json": `{"mcpServers": {"test1": {"command": "echo"}}}`,
				"config2.json": `{"mcpServers": {"test2": {"command": "ls"}}}`,
				"variables.yml": `TEST_VAR: test_value`,
				"readme.txt":    `This should be ignored`,
			},
			environment: "test",
			expectCount: 2,
			expectNames: []string{"config1", "config2"},
		},
		{
			name: "discover_configs_with_timestamps",
			setupFiles: map[string]string{
				"filesystem_20250805_151454.json": `{"mcpServers": {"fs": {"command": "npx"}}}`,
				"aws_knowledge_20250806_120000.json": `{"mcpServers": {"aws": {"url": "https://api.aws"}}}`,
			},
			environment: "test",
			expectCount: 2,
			expectNames: []string{"filesystem_20250805_151454", "aws_knowledge_20250806_120000"},
		},
		{
			name:        "empty_directory",
			setupFiles:  map[string]string{},
			environment: "empty",
			expectCount: 0,
			expectNames: []string{},
		},
		{
			name: "mixed_file_types",
			setupFiles: map[string]string{
				"config.json":     `{"mcpServers": {"test": {"command": "echo"}}}`,
				"config.yaml":     `servers: test`,
				"config.txt":      `not a config`,
				"subdir/test.json": `should be ignored`,
			},
			environment: "test",
			expectCount: 1,
			expectNames: []string{"config"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory structure
			tempDir := t.TempDir()
			envDir := filepath.Join(tempDir, ".config", "station", "environments", tt.environment)
			err := os.MkdirAll(envDir, 0755)
			require.NoError(t, err)

			// Create test files
			for filename, content := range tt.setupFiles {
				filePath := filepath.Join(envDir, filename)
				// Create subdirectory if needed
				if dir := filepath.Dir(filePath); dir != envDir {
					err := os.MkdirAll(dir, 0755)
					require.NoError(t, err)
				}
				err := os.WriteFile(filePath, []byte(content), 0644)
				require.NoError(t, err)
			}

			// Mock the config directory
			originalHome := os.Getenv("HOME")
			os.Setenv("HOME", tempDir)
			defer os.Setenv("HOME", originalHome)

			// Create handler and test
			handler := &MCPHandler{}
			configs, err := handler.discoverConfigFiles(tt.environment)

			require.NoError(t, err)
			assert.Len(t, configs, tt.expectCount)

			// Check config names
			actualNames := make([]string, len(configs))
			for i, config := range configs {
				actualNames[i] = config.ConfigName
			}
			assert.ElementsMatch(t, tt.expectNames, actualNames)

			// Verify template paths are set correctly
			for _, config := range configs {
				expectedPath := filepath.Join("environments", tt.environment, config.ConfigName+".json")
				assert.Equal(t, expectedPath, config.TemplatePath)
				assert.NotNil(t, config.LastLoadedAt)
			}
		})
	}
}

func TestMCPHandler_loadConfigFromFilesystem(t *testing.T) {
	tests := []struct {
		name           string
		configContent  string
		variables      map[string]string
		expectError    bool
		errorContains  string
		expectServers  []string
	}{
		{
			name: "valid_config_with_variables",
			configContent: `{
				"mcpServers": {
					"filesystem": {
						"command": "npx",
						"args": ["-y", "@modelcontextprotocol/server-filesystem", "{{.ALLOWED_PATHS}}"],
						"env": {"DEBUG": "{{.DEBUG_MODE}}"}
					}
				}
			}`,
			variables: map[string]string{
				"ALLOWED_PATHS": "/home/test",
				"DEBUG_MODE":    "true",
			},
			expectError:   false,
			expectServers: []string{"filesystem"},
		},
		{
			name: "missing_template_variables",
			configContent: `{
				"mcpServers": {
					"test-server": {
						"command": "echo",
						"args": ["{{.MISSING_VAR}}", "{{.ANOTHER_MISSING}}"],
						"env": {"API_KEY": "{{.UNDEFINED_KEY}}"}
					}
				}
			}`,
			variables:     map[string]string{},
			expectError:   true,
			errorContains: "missing template variables: [MISSING_VAR ANOTHER_MISSING UNDEFINED_KEY]",
		},
		{
			name: "invalid_json",
			configContent: `{
				"mcpServers": {
					"test": {
						"command": "echo"
					// missing closing brace
			}`,
			variables:     map[string]string{},
			expectError:   true,
			errorContains: "failed to parse JSON",
		},
		{
			name: "no_mcp_servers_field",
			configContent: `{
				"someOtherField": {
					"test": {"command": "echo"}
				}
			}`,
			variables:     map[string]string{},
			expectError:   true,
			errorContains: "no 'mcpServers' or 'servers' field found",
		},
		{
			name: "http_server_config",
			configContent: `{
				"mcpServers": {
					"aws-knowledge": {
						"url": "https://knowledge-mcp.global.api.aws"
					}
				}
			}`,
			variables:     map[string]string{},
			expectError:   false,
			expectServers: []string{"aws-knowledge"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory and files
			tempDir := t.TempDir()
			envDir := filepath.Join(tempDir, ".config", "station", "environments", "test")
			err := os.MkdirAll(envDir, 0755)
			require.NoError(t, err)

			configFile := filepath.Join(envDir, "test_config.json")
			err = os.WriteFile(configFile, []byte(tt.configContent), 0644)
			require.NoError(t, err)

			// Create variables.yml if variables provided
			if len(tt.variables) > 0 {
				variablesContent := ""
				for key, value := range tt.variables {
					variablesContent += key + ": " + value + "\n"
				}
				variablesFile := filepath.Join(envDir, "variables.yml")
				err = os.WriteFile(variablesFile, []byte(variablesContent), 0644)
				require.NoError(t, err)
			}

			// Mock the config directory
			originalHome := os.Getenv("HOME")
			os.Setenv("HOME", tempDir)
			defer os.Setenv("HOME", originalHome)

			// Create test database with schema
			testDB, repos := setupTestDB(t)
			defer testDB.Close()

			// Create test environment
			env, err := repos.Environments.Create("test", nil, 1)
			require.NoError(t, err)
			envID := env.ID

			// Create file config record
			fileConfig := &repositories.FileConfigRecord{
				ConfigName:   "test_config",
				TemplatePath: "environments/test/test_config.json",
			}

			// Test the function
			handler := &MCPHandler{}
			err = handler.loadConfigFromFilesystem(repos, envID, "test", "test_config", fileConfig)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)

				// Verify servers were created
				servers, err := repos.MCPServers.GetByEnvironmentID(envID)
				require.NoError(t, err)
				assert.Len(t, servers, len(tt.expectServers))

				serverNames := make([]string, len(servers))
				for i, server := range servers {
					serverNames[i] = server.Name
				}
				assert.ElementsMatch(t, tt.expectServers, serverNames)

				// Verify file config was created/updated
				fileConfigs, err := repos.FileMCPConfigs.ListByEnvironment(envID)
				require.NoError(t, err)
				assert.Len(t, fileConfigs, 1)
				assert.Equal(t, "test_config", fileConfigs[0].ConfigName)
				assert.NotNil(t, fileConfigs[0].LastLoadedAt)
			}
		})
	}
}

func TestMCPHandler_removeOrphanedAgentTools(t *testing.T) {
	// Create test database with schema
	testDB, repos := setupTestDB(t)
	defer testDB.Close()

	// Create test environment
	env, err := repos.Environments.Create("test", nil, 1)
	require.NoError(t, err)
	envID := env.ID

	// Create test agents
	agent1, err := repos.Agents.Create("Test Agent 1", "Test", "Test", 
		5, envID, 1, nil, true)
	require.NoError(t, err)

	agent2, err := repos.Agents.Create("Test Agent 2", "Test", "Test", 
		3, envID, 1, nil, true)
	require.NoError(t, err)

	// Create test MCP server
	server := &models.MCPServer{
		Name: "test-server", Command: "echo", EnvironmentID: envID,
	}
	serverID, err := repos.MCPServers.Create(server)
	require.NoError(t, err)

	// Create test tools
	tool1 := &models.MCPTool{
		MCPServerID: serverID, Name: "test_tool_1", Description: "Test Tool 1",
	}
	tool1ID, err := repos.MCPTools.Create(tool1)
	require.NoError(t, err)

	tool2 := &models.MCPTool{
		MCPServerID: serverID, Name: "test_tool_2", Description: "Test Tool 2",
	}
	tool2ID, err := repos.MCPTools.Create(tool2)
	require.NoError(t, err)

	// Assign tools to agents
	_, err = repos.AgentTools.AddAgentTool(agent1.ID, tool1ID)
	require.NoError(t, err)
	_, err = repos.AgentTools.AddAgentTool(agent1.ID, tool2ID)
	require.NoError(t, err)
	_, err = repos.AgentTools.AddAgentTool(agent2.ID, tool1ID)
	require.NoError(t, err)

	// Create file config
	fileConfig := &repositories.FileConfigRecord{
		EnvironmentID: envID, ConfigName: "test-config", TemplatePath: "test.json",
	}
	configID, err := repos.FileMCPConfigs.Create(fileConfig)
	require.NoError(t, err)

	// Get agents for the test
	agents, err := repos.Agents.ListByEnvironment(envID)
	require.NoError(t, err)

	// Test orphaned tool removal
	handler := &MCPHandler{}
	removedCount, err := handler.removeOrphanedAgentTools(repos, agents, configID)
	require.NoError(t, err)

	// Should have removed 3 tool assignments (2 from agent1, 1 from agent2)
	assert.Equal(t, 3, removedCount)

	// Verify tools were removed from agents
	agent1Tools, err := repos.AgentTools.ListAgentTools(agent1.ID)
	require.NoError(t, err)
	assert.Empty(t, agent1Tools)

	agent2Tools, err := repos.AgentTools.ListAgentTools(agent2.ID)
	require.NoError(t, err)
	assert.Empty(t, agent2Tools)

	// Verify tools were deleted from database
	tools, err := repos.MCPTools.GetByServerID(serverID)
	require.NoError(t, err)
	assert.Empty(t, tools)

	// Verify server was deleted
	servers, err := repos.MCPServers.GetByEnvironmentID(envID)
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestMCPHandler_determineImpactLevel(t *testing.T) {
	tests := []struct {
		name         string
		toolsRemoved int
		expectedImpact string
	}{
		{"high_impact", 5, "high"},
		{"high_impact_many", 10, "high"},
		{"medium_impact_low", 2, "medium"},
		{"medium_impact_high", 4, "medium"},
		{"low_impact", 1, "low"},
		{"low_impact_zero", 0, "low"},
	}

	handler := &MCPHandler{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact := handler.determineImpactLevel(tt.toolsRemoved)
			assert.Equal(t, tt.expectedImpact, impact)
		})
	}
}

// Integration test for the full sync process
func TestMCPHandler_syncMCPConfigsLocal_Integration(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()
	envDir := filepath.Join(tempDir, ".config", "station", "environments", "test")
	err := os.MkdirAll(envDir, 0755)
	require.NoError(t, err)

	// Create test config files
	config1 := `{
		"mcpServers": {
			"test-server-1": {
				"command": "echo",
				"args": ["hello", "{{.GREETING}}"]
			}
		}
	}`
	err = os.WriteFile(filepath.Join(envDir, "config1.json"), []byte(config1), 0644)
	require.NoError(t, err)

	config2 := `{
		"mcpServers": {
			"test-server-2": {
				"command": "ls",
				"args": ["-la"]
			}
		}
	}`
	err = os.WriteFile(filepath.Join(envDir, "config2.json"), []byte(config2), 0644)
	require.NoError(t, err)

	// Create variables file
	variables := "GREETING: world\n"
	err = os.WriteFile(filepath.Join(envDir, "variables.yml"), []byte(variables), 0644)
	require.NoError(t, err)

	// Mock the config directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create test database with schema
	testDB, repos := setupTestDB(t)
	defer testDB.Close()

	// Create test environment
	env, err := repos.Environments.Create("test", nil, 1)
	require.NoError(t, err)
	envID := env.ID

	// Note: For integration tests, we use the test database directly

	// Create theme manager mock
	handler := &MCPHandler{}

	// Test sync (this would normally be called through CLI)
	// We'll test the core functionality through the individual functions
	
	// Test file discovery
	configs, err := handler.discoverConfigFiles("test")
	require.NoError(t, err)
	assert.Len(t, configs, 2)

	configNames := make([]string, len(configs))
	for i, config := range configs {
		configNames[i] = config.ConfigName
	}
	assert.ElementsMatch(t, []string{"config1", "config2"}, configNames)

	// Test config loading
	for _, config := range configs {
		err = handler.loadConfigFromFilesystem(repos, envID, "test", config.ConfigName, config)
		require.NoError(t, err)
	}

	// Verify servers were created
	servers, err := repos.MCPServers.GetByEnvironmentID(envID)
	require.NoError(t, err)
	assert.Len(t, servers, 2)

	serverNames := make([]string, len(servers))
	for i, server := range servers {
		serverNames[i] = server.Name
	}
	assert.ElementsMatch(t, []string{"test-server-1", "test-server-2"}, serverNames)

	// Verify file configs were created
	fileConfigs, err := repos.FileMCPConfigs.ListByEnvironment(envID)
	require.NoError(t, err)
	assert.Len(t, fileConfigs, 2)

	// Test orphaned config removal by removing a file
	err = os.Remove(filepath.Join(envDir, "config1.json"))
	require.NoError(t, err)

	// Re-discover configs
	newConfigs, err := handler.discoverConfigFiles("test")
	require.NoError(t, err)
	assert.Len(t, newConfigs, 1)
	assert.Equal(t, "config2", newConfigs[0].ConfigName)

	// This demonstrates that the sync would detect the orphaned config
	// and remove it from the database
}

// Note: StationConfig and loadStationConfig are defined in common.go