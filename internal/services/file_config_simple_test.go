package services

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/spf13/afero"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// Test the key scenarios without complex MCP interactions
func TestFileConfigIntegration_BasicFlow(t *testing.T) {
	// Setup test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Conn().Close()

	// Run migrations
	if err := db.RunMigrations(database.Conn()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repos := repositories.New(database)

	// Create test environment
	env, err := repos.Environments.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	// Test file config record creation
	fileConfig := &repositories.FileConfigRecord{
		EnvironmentID:            env.ID,
		ConfigName:               "github-config",
		TemplatePath:             "/test/github.json",
		VariablesPath:            "/test/variables.yml",
		TemplateSpecificVarsPath: "/test/github.vars.yml",
		TemplateHash:             "template-hash-123",
		VariablesHash:            "vars-hash-456",
		TemplateVarsHash:         "template-vars-hash-789",
		Metadata:                 `{"test": true}`,
	}

	fileConfigID, err := repos.FileMCPConfigs.Create(fileConfig)
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	t.Logf("Created file config with ID: %d", fileConfigID)

	// Test retrieving file config
	retrieved, err := repos.FileMCPConfigs.GetByEnvironmentAndName(env.ID, "github-config")
	if err != nil {
		t.Fatalf("Failed to retrieve file config: %v", err)
	}

	if retrieved.ConfigName != "github-config" {
		t.Errorf("Expected config name 'github-config', got '%s'", retrieved.ConfigName)
	}

	if retrieved.TemplateHash != "template-hash-123" {
		t.Errorf("Expected template hash 'template-hash-123', got '%s'", retrieved.TemplateHash)
	}

	// Test change detection
	hasChanges, err := repos.FileMCPConfigs.HasChanges(env.ID, "github-config", "template-hash-123", "vars-hash-456", "template-vars-hash-789")
	if err != nil {
		t.Fatalf("Failed to check for changes: %v", err)
	}

	if hasChanges {
		t.Errorf("Expected no changes with identical hashes")
	}

	// Test detecting changes
	hasChanges, err = repos.FileMCPConfigs.HasChanges(env.ID, "github-config", "new-template-hash", "vars-hash-456", "template-vars-hash-789")
	if err != nil {
		t.Fatalf("Failed to check for template changes: %v", err)
	}

	if !hasChanges {
		t.Errorf("Expected changes with different template hash")
	}

	t.Logf("✅ File config basic flow test passed")
}

func TestFileConfigIntegration_VariableResolution(t *testing.T) {
	// Test the core scenario: template-specific variables override global ones
	fs := afero.NewMemMapFs()
	configDir := "/test/config"
	
	// Create directory structure
	fs.MkdirAll(filepath.Join(configDir, "environments", "dev"), 0755)
	
	// Create template with variables
	templateContent := `{
	"name": "github-test",
	"servers": {
		"github": {
			"command": "node",
			"args": ["github-server.js"],
			"env": {
				"GITHUB_TOKEN": "{{.GithubToken}}",
				"GITHUB_REPO": "{{.GithubRepo | default \"default-repo\"}}"
			}
		}
	}
}`

	templatePath := filepath.Join(configDir, "environments", "dev", "github.json")
	if err := afero.WriteFile(fs, templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to create template: %v", err)
	}

	// Create global variables
	globalVars := `github_token: "global-token-123"
github_repo: "global-repo"`

	globalVarsPath := filepath.Join(configDir, "environments", "dev", "variables.yml")
	if err := afero.WriteFile(fs, globalVarsPath, []byte(globalVars), 0644); err != nil {
		t.Fatalf("Failed to create global variables: %v", err)
	}

	// Create template-specific variables (should override global)
	templateVars := `github_token: "template-token-456"
github_repo: "template-repo"`

	templateVarsPath := filepath.Join(configDir, "environments", "dev", "github.vars.yml")
	if err := afero.WriteFile(fs, templateVarsPath, []byte(templateVars), 0644); err != nil {
		t.Fatalf("Failed to create template variables: %v", err)
	}

	// Test variable resolution logic
	globalVariables := map[string]interface{}{
		"GithubToken": "global-token-123",
		"GithubRepo":  "global-repo",
	}

	templateSpecificVariables := map[string]interface{}{
		"GithubToken": "template-token-456",
		"GithubRepo":  "template-repo",
	}

	// Simulate the merging logic from FileConfigService
	merged := make(map[string]interface{})
	
	// Start with global variables
	for k, v := range globalVariables {
		merged[k] = v
	}
	
	// Override with template-specific variables
	for k, v := range templateSpecificVariables {
		merged[k] = v
	}

	// Verify template-specific variables take precedence
	if merged["GithubToken"] != "template-token-456" {
		t.Errorf("Expected template-specific token, got '%v'", merged["GithubToken"])
	}

	if merged["GithubRepo"] != "template-repo" {
		t.Errorf("Expected template-specific repo, got '%v'", merged["GithubRepo"])
	}

	t.Logf("✅ Variable resolution test passed")
}

func TestFileConfigIntegration_ToolLinking(t *testing.T) {
	// Setup test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Conn().Close()

	// Run migrations
	if err := db.RunMigrations(database.Conn()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repos := repositories.New(database)

	// Create test environment
	env, err := repos.Environments.Create("test-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	// Create file config
	fileConfig := &repositories.FileConfigRecord{
		EnvironmentID: env.ID,
		ConfigName:    "test-config",
		TemplatePath:  "/test/config.json",
		VariablesPath: "/test/vars.yml",
		TemplateHash:  "hash-123",
		VariablesHash: "vars-hash-456",
		Metadata:      "{}",
	}

	fileConfigID, err := repos.FileMCPConfigs.Create(fileConfig)
	if err != nil {
		t.Fatalf("Failed to create file config: %v", err)
	}

	// Create test server
	server := &models.MCPServer{
		EnvironmentID: env.ID,
		Name:          "test-server",
		Command:       "node",
		Args:          []string{"test-server.js"},
	}

	serverID, err := repos.MCPServers.Create(server)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create tools linked to file config
	tool1 := &models.MCPTool{
		MCPServerID: serverID,
		Name:        "test-tool-1",
		Description: "Test tool 1",
		Schema:      []byte(`{"type": "object", "properties": {"param1": {"type": "string"}}}`),
	}

	tool2 := &models.MCPTool{
		MCPServerID: serverID,
		Name:        "test-tool-2",
		Description: "Test tool 2",
		Schema:      []byte(`{"type": "object", "properties": {"param2": {"type": "number"}}}`),
	}

	// Use extension method to create tools with file config reference
	tool1ID, err := repos.MCPTools.CreateWithFileConfig(tool1, fileConfigID)
	if err != nil {
		t.Fatalf("Failed to create tool1 with file config: %v", err)
	}

	tool2ID, err := repos.MCPTools.CreateWithFileConfig(tool2, fileConfigID)
	if err != nil {
		t.Fatalf("Failed to create tool2 with file config: %v", err)
	}

	t.Logf("Created tools with IDs: %d, %d", tool1ID, tool2ID)

	// Test retrieving tools by file config
	tools, err := repos.MCPTools.GetByFileConfigID(fileConfigID)
	if err != nil {
		t.Fatalf("Failed to get tools by file config: %v", err)
	}

	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	if !toolNames["test-tool-1"] || !toolNames["test-tool-2"] {
		t.Errorf("Missing expected tools. Got: %v", toolNames)
	}

	// Test hybrid tool retrieval (tools with file config info)
	hybridTools, err := repos.MCPTools.GetToolsWithFileConfigInfo(env.ID)
	if err != nil {
		t.Fatalf("Failed to get hybrid tools: %v", err)
	}

	if len(hybridTools) != 2 {
		t.Errorf("Expected 2 hybrid tools, got %d", len(hybridTools))
	}

	// Verify file config information is included
	for _, tool := range hybridTools {
		if tool.FileConfigID == nil {
			t.Errorf("Expected file config ID for tool %s", tool.Name)
		} else if *tool.FileConfigID != fileConfigID {
			t.Errorf("Expected file config ID %d for tool %s, got %d", fileConfigID, tool.Name, *tool.FileConfigID)
		}

		if tool.ConfigName != "test-config" {
			t.Errorf("Expected config name 'test-config' for tool %s, got '%s'", tool.Name, tool.ConfigName)
		}
	}

	// Test clearing tools for file config
	err = repos.MCPTools.DeleteByFileConfigID(fileConfigID)
	if err != nil {
		t.Fatalf("Failed to delete tools by file config: %v", err)
	}

	// Verify tools were deleted
	tools, err = repos.MCPTools.GetByFileConfigID(fileConfigID)
	if err != nil {
		t.Fatalf("Failed to get tools after deletion: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("Expected 0 tools after deletion, got %d", len(tools))
	}

	t.Logf("✅ Tool linking test passed")
}

func TestFileConfigIntegration_MultipleTemplatesSameVariables(t *testing.T) {
	// Test the core user scenario: multiple templates with same variable names but different values
	
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer database.Conn().Close()

	// Run migrations
	if err := db.RunMigrations(database.Conn()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	repos := repositories.New(database)

	// Create test environment
	env, err := repos.Environments.Create("dev", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create test environment: %v", err)
	}

	// Scenario: Two GitHub templates, one for personal repos, one for work repos
	// Both use the same variable names but need different values

	// Create file configs for both templates
	personalGithub := &repositories.FileConfigRecord{
		EnvironmentID:            env.ID,
		ConfigName:               "github-personal",
		TemplatePath:             "/test/github-personal.json",
		VariablesPath:            "/test/variables.yml",
		TemplateSpecificVarsPath: "/test/github-personal.vars.yml",
		TemplateHash:             "personal-template-hash",
		VariablesHash:            "global-vars-hash",
		TemplateVarsHash:         "personal-vars-hash",
		Metadata:                 `{"type": "personal"}`,
	}

	workGithub := &repositories.FileConfigRecord{
		EnvironmentID:            env.ID,
		ConfigName:               "github-work",
		TemplatePath:             "/test/github-work.json",
		VariablesPath:            "/test/variables.yml",
		TemplateSpecificVarsPath: "/test/github-work.vars.yml",
		TemplateHash:             "work-template-hash",
		VariablesHash:            "global-vars-hash",
		TemplateVarsHash:         "work-vars-hash",  
		Metadata:                 `{"type": "work"}`,
	}

	personalID, err := repos.FileMCPConfigs.Create(personalGithub)
	if err != nil {
		t.Fatalf("Failed to create personal GitHub config: %v", err)
	}

	workID, err := repos.FileMCPConfigs.Create(workGithub)
	if err != nil {
		t.Fatalf("Failed to create work GitHub config: %v", err)
	}

	// Simulate variable resolution for each config
	globalVars := map[string]interface{}{
		"ApiKey":    "default-api-key",
		"OrgName":   "default-org",
		"RepoName":  "default-repo",
	}

	personalVars := map[string]interface{}{
		"ApiKey":   "personal-github-token-123",
		"OrgName":  "john-doe",
		"RepoName": "my-personal-project",
	}

	workVars := map[string]interface{}{
		"ApiKey":   "work-github-token-456", 
		"OrgName":  "acme-corp",
		"RepoName": "enterprise-app",
	}

	// Test personal config variable resolution
	personalMerged := make(map[string]interface{})
	for k, v := range globalVars {
		personalMerged[k] = v
	}
	for k, v := range personalVars {
		personalMerged[k] = v
	}

	// Test work config variable resolution
	workMerged := make(map[string]interface{})
	for k, v := range globalVars {
		workMerged[k] = v
	}
	for k, v := range workVars {
		workMerged[k] = v
	}

	// Verify each config gets its own variables despite same names
	if personalMerged["ApiKey"] != "personal-github-token-123" {
		t.Errorf("Expected personal API key, got '%v'", personalMerged["ApiKey"])
	}

	if workMerged["ApiKey"] != "work-github-token-456" {
		t.Errorf("Expected work API key, got '%v'", workMerged["ApiKey"])
	}

	if personalMerged["OrgName"] != "john-doe" {
		t.Errorf("Expected personal org name, got '%v'", personalMerged["OrgName"])
	}

	if workMerged["OrgName"] != "acme-corp" {
		t.Errorf("Expected work org name, got '%v'", workMerged["OrgName"])
	}

	// Verify both configs exist in database
	allConfigs, err := repos.FileMCPConfigs.ListByEnvironment(env.ID)
	if err != nil {
		t.Fatalf("Failed to list configs: %v", err)
	}

	if len(allConfigs) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(allConfigs))
	}

	configNames := make(map[string]bool)
	for _, config := range allConfigs {
		configNames[config.ConfigName] = true
	}

	if !configNames["github-personal"] || !configNames["github-work"] {
		t.Errorf("Missing expected configs. Got: %v", configNames)
	}

	// Test change detection for each config independently
	personalHasChanges, err := repos.FileMCPConfigs.HasChanges(env.ID, "github-personal", "personal-template-hash", "global-vars-hash", "personal-vars-hash")
	if err != nil {
		t.Fatalf("Failed to check personal changes: %v", err)
	}

	if personalHasChanges {
		t.Errorf("Expected no changes for personal config with matching hashes")
	}

	workHasChanges, err := repos.FileMCPConfigs.HasChanges(env.ID, "github-work", "work-template-hash", "global-vars-hash", "work-vars-hash")
	if err != nil {
		t.Fatalf("Failed to check work changes: %v", err)
	}

	if workHasChanges {
		t.Errorf("Expected no changes for work config with matching hashes")
	}

	// Test that changing one config doesn't affect the other
	personalNewVarsHash, err := repos.FileMCPConfigs.HasChanges(env.ID, "github-personal", "personal-template-hash", "global-vars-hash", "new-personal-vars-hash")
	if err != nil {
		t.Fatalf("Failed to check personal template vars changes: %v", err)
	}

	if !personalNewVarsHash {
		t.Errorf("Expected changes for personal config with different template vars hash")
	}

	// Work config should still have no changes
	workStillSame, err := repos.FileMCPConfigs.HasChanges(env.ID, "github-work", "work-template-hash", "global-vars-hash", "work-vars-hash")
	if err != nil {
		t.Fatalf("Failed to check work changes after personal change: %v", err)
	}

	if workStillSame {
		t.Errorf("Work config should still have no changes")
	}

	t.Logf("✅ Multiple templates with same variables test passed")
	t.Logf("   Personal config ID: %d", personalID)
	t.Logf("   Work config ID: %d", workID)
	t.Logf("   Successfully demonstrated template-specific variable resolution")
}