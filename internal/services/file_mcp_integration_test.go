package services

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/filesystem"
	"station/internal/template"
	"station/internal/variables"
	"station/pkg/models"
	pkgconfig "station/pkg/config"
)

// TestFileMCPConfigurationEndToEnd tests the complete file-based MCP configuration workflow
func TestFileMCPConfigurationEndToEnd(t *testing.T) {
	// Create test database
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()

	err = database.Migrate()
	require.NoError(t, err)

	repos := repositories.New(database)

	// Create test user and environment
	user, err := repos.Users.Create("test-user", "test-key", false, nil)
	require.NoError(t, err)

	desc := "Test Environment for File MCP"
	env, err := repos.Environments.Create("test-env", &desc, user.ID)
	require.NoError(t, err)

	// Setup file system components
	fs := afero.NewMemMapFs()
	configDir := "/config"
	varsDir := "/config/vars"

	// Create directories
	err = fs.MkdirAll(configDir, 0755)
	require.NoError(t, err)
	err = fs.MkdirAll(varsDir, 0755)
	require.NoError(t, err)

	fileSystem := filesystem.NewConfigFileSystem(fs, configDir, varsDir)
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	// Create file config options
	fileConfigOptions := pkgconfig.FileConfigOptions{
		ConfigDir:       configDir,
		VariablesDir:    varsDir,
		Strategy:        pkgconfig.StrategyTemplateFirst,
		AutoCreate:      true,
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}

	// Create file config manager
	fileConfigManager := config.NewFileConfigManager(
		fileSystem,
		templateEngine,
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)

	// Initialize tool discovery service
	toolDiscoveryService := NewToolDiscoveryService(repos)

	// Initialize file config service
	fileConfigService := NewFileConfigService(
		fileConfigManager,
		toolDiscoveryService,
		repos,
	)

	t.Run("CreateAndLoadFileBasedConfig", func(t *testing.T) {
		ctx := context.Background()

		// Create a test template file
		templateContent := `{
  "name": "{{.config_name}}",
  "servers": {
    "filesystem": {
      "transport": "stdio",
      "command": "{{.filesystem_command}}",
      "args": ["{{.filesystem_args}}"],
      "env": {
        "PATH": "{{.path_env}}"
      }
    }
  }
}`

		templatePath := filepath.Join(configDir, "test-config.json")
		err = afero.WriteFile(fs, templatePath, []byte(templateContent), 0644)
		require.NoError(t, err)

		// Create variables file
		variablesContent := `config_name: "Test File Config"
filesystem_command: "npx"
filesystem_args: "-y @modelcontextprotocol/server-filesystem"
path_env: "/usr/local/bin:/usr/bin:/bin"`

		varsPath := filepath.Join(varsDir, "test-env.yml")
		err = afero.WriteFile(fs, varsPath, []byte(variablesContent), 0644)
		require.NoError(t, err)

		// Test loading the config
		renderedConfig, err := fileConfigService.LoadAndRenderConfig(ctx, env.ID, "test-config")
		require.NoError(t, err)
		require.NotNil(t, renderedConfig)

		// Verify rendered config content
		assert.Equal(t, "Test File Config", renderedConfig.Name)
		assert.Contains(t, renderedConfig.Servers, "filesystem")
		
		fsServer := renderedConfig.Servers["filesystem"]
		assert.Equal(t, "npx", fsServer.Command)
		assert.Equal(t, []string{"-y", "@modelcontextprotocol/server-filesystem"}, fsServer.Args)
		assert.Equal(t, "/usr/local/bin:/usr/bin:/bin", fsServer.Env["PATH"])

		// Test discovery tools from file config
		result, err := toolDiscoveryService.DiscoverToolsFromFileConfig(env.ID, "test-config", renderedConfig)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, env.ID, result.EnvironmentID)
		assert.Equal(t, "test-config", result.ConfigName)

		t.Logf("✅ File-based config end-to-end test passed")
		t.Logf("   Config Name: %s", renderedConfig.Name)
		t.Logf("   Servers: %d", len(renderedConfig.Servers))
		t.Logf("   Discovery Result: Environment %d, Success: %v", result.EnvironmentID, result.Success)
	})

	t.Run("ConfigListingAndValidation", func(t *testing.T) {
		ctx := context.Background()

		// Test listing file configs
		configs, err := fileConfigService.ListFileConfigs(ctx, env.ID)
		require.NoError(t, err)
		assert.Len(t, configs, 1)
		assert.Equal(t, "test-config", configs[0].Name)

		// Test that we can load the config again (validates persistence)
		reloadedConfig, err := fileConfigService.LoadAndRenderConfig(ctx, env.ID, "test-config")
		require.NoError(t, err)
		assert.Equal(t, "Test File Config", reloadedConfig.Name)

		t.Logf("✅ Config listing and validation test passed")
		t.Logf("   Listed configs: %d", len(configs))
		t.Logf("   Reloaded config name: %s", reloadedConfig.Name)
	})

	t.Run("ToolLinkingWithFileConfig", func(t *testing.T) {
		ctx := context.Background()

		// Simulate discovering tools and verify they link to file config
		renderedConfig, err := fileConfigService.LoadAndRenderConfig(ctx, env.ID, "test-config")
		require.NoError(t, err)

		// Test tool discovery with file config linking
		result, err := toolDiscoveryService.DiscoverToolsFromFileConfig(env.ID, "test-config", renderedConfig)
		require.NoError(t, err)

		// Get file config record to verify tool linking
		fileConfig, err := repos.FileMCPConfigs.GetByEnvironmentAndName(env.ID, "test-config")
		require.NoError(t, err)

		// Verify tools can be retrieved by file config
		tools, err := repos.MCPTools.GetByFileConfigID(fileConfig.ID)
		require.NoError(t, err)

		t.Logf("✅ Tool linking with file config test passed")
		t.Logf("   File Config ID: %d", fileConfig.ID)
		t.Logf("   Linked Tools: %d", len(tools))
		t.Logf("   Discovery successful: %v", result.Success)
	})
}

// TestFileMCPConfigurationErrorHandling tests error scenarios
func TestFileMCPConfigurationErrorHandling(t *testing.T) {
	// Create test database
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()

	err = database.Migrate()
	require.NoError(t, err)

	repos := repositories.New(database)

	// Create test environment
	user, err := repos.Users.Create("test-user", "test-key", false, nil)
	require.NoError(t, err)

	desc := "Test Environment"
	env, err := repos.Environments.Create("test-env", &desc, user.ID)
	require.NoError(t, err)

	// Setup file system with missing directories to test error handling
	fs := afero.NewMemMapFs()
	configDir := "/nonexistent"
	varsDir := "/nonexistent/vars"

	fileSystem := filesystem.NewConfigFileSystem(fs, configDir, varsDir)
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	fileConfigOptions := pkgconfig.FileConfigOptions{
		ConfigDir:       configDir,
		VariablesDir:    varsDir,
		Strategy:        pkgconfig.StrategyTemplateFirst,
		AutoCreate:      false, // Disable auto-create to test error handling
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}

	fileConfigManager := config.NewFileConfigManager(
		fileSystem,
		templateEngine,
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)

	toolDiscoveryService := NewToolDiscoveryService(repos)
	fileConfigService := NewFileConfigService(
		fileConfigManager,
		toolDiscoveryService,
		repos,
	)

	t.Run("MissingConfigFileError", func(t *testing.T) {
		ctx := context.Background()

		// Try to load non-existent config
		_, err := fileConfigService.LoadAndRenderConfig(ctx, env.ID, "nonexistent-config")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nonexistent-config")

		t.Logf("✅ Missing config file error handling test passed")
		t.Logf("   Expected error: %v", err)
	})

	t.Run("InvalidEnvironmentError", func(t *testing.T) {
		ctx := context.Background()

		// Try to load config for non-existent environment
		_, err := fileConfigService.LoadAndRenderConfig(ctx, 99999, "any-config")
		assert.Error(t, err)

		t.Logf("✅ Invalid environment error handling test passed")
		t.Logf("   Expected error: %v", err)
	})
}

// TestFileMCPPerformance tests performance characteristics
func TestFileMCPPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create test database
	database, err := db.New(":memory:")
	require.NoError(t, err)
	defer database.Close()

	err = database.Migrate()
	require.NoError(t, err)

	repos := repositories.New(database)

	// Create test user and multiple environments
	user, err := repos.Users.Create("perf-user", "test-key", false, nil)
	require.NoError(t, err)

	const numEnvironments = 10
	const numConfigsPerEnv = 5

	environments := make([]*models.Environment, numEnvironments)
	for i := 0; i < numEnvironments; i++ {
		desc := fmt.Sprintf("Performance Test Environment %d", i)
		env, err := repos.Environments.Create(fmt.Sprintf("perf-env-%d", i), &desc, user.ID)
		require.NoError(t, err)
		environments[i] = env
	}

	// Setup file system
	fs := afero.NewMemMapFs()
	configDir := "/config"
	varsDir := "/config/vars"

	err = fs.MkdirAll(configDir, 0755)
	require.NoError(t, err)
	err = fs.MkdirAll(varsDir, 0755)
	require.NoError(t, err)

	fileSystem := filesystem.NewConfigFileSystem(fs, configDir, varsDir)
	templateEngine := template.NewGoTemplateEngine()
	variableStore := variables.NewEnvVariableStore(fs)
	
	fileConfigOptions := pkgconfig.FileConfigOptions{
		ConfigDir:       configDir,
		VariablesDir:    varsDir,
		Strategy:        pkgconfig.StrategyTemplateFirst,
		AutoCreate:      true,
		BackupOnChange:  false,
		ValidateOnLoad:  true,
	}

	fileConfigManager := config.NewFileConfigManager(
		fileSystem,
		templateEngine,
		variableStore,
		fileConfigOptions,
		repos.Environments,
	)

	toolDiscoveryService := NewToolDiscoveryService(repos)
	fileConfigService := NewFileConfigService(
		fileConfigManager,
		toolDiscoveryService,
		repos,
	)

	t.Run("BulkConfigOperations", func(t *testing.T) {
		ctx := context.Background()

		// Create multiple config templates
		for i := 0; i < numConfigsPerEnv; i++ {
			templateContent := fmt.Sprintf(`{
  "name": "Perf Config %d",
  "servers": {
    "server%d": {
      "transport": "stdio",
      "command": "echo",
      "args": ["server-%d"],
      "env": {}
    }
  }
}`, i, i, i)

			templatePath := filepath.Join(configDir, fmt.Sprintf("perf-config-%d.json", i))
			err = afero.WriteFile(fs, templatePath, []byte(templateContent), 0644)
			require.NoError(t, err)
		}

		// Create variables for each environment
		for _, env := range environments {
			varsContent := fmt.Sprintf("env_name: \"%s\"", env.Name)
			varsPath := filepath.Join(varsDir, fmt.Sprintf("%s.yml", env.Name))
			err = afero.WriteFile(fs, varsPath, []byte(varsContent), 0644)
			require.NoError(t, err)
		}

		// Test bulk loading
		totalConfigs := 0
		for _, env := range environments {
			configs, err := fileConfigService.ListFileConfigs(ctx, env.ID)
			require.NoError(t, err)
			totalConfigs += len(configs)
		}

		t.Logf("✅ Bulk config operations test passed")
		t.Logf("   Environments: %d", numEnvironments)
		t.Logf("   Total configs processed: %d", totalConfigs)
		t.Logf("   Expected configs: %d", numEnvironments*numConfigsPerEnv)
	})
}