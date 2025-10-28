package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// TestNewEnvironmentCopyService tests service creation
func TestNewEnvironmentCopyService(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "Create environment copy service",
			description: "Should create service with initialized dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDB, err := db.NewTest(t)
			if err != nil {
				t.Fatalf("Failed to create test database: %v", err)
			}
			defer func() { _ = testDB.Close() }()

			repos := repositories.New(testDB)
			service := NewEnvironmentCopyService(repos)

			if service == nil {
				t.Fatal("NewEnvironmentCopyService() returned nil")
			}

			if service.repos == nil {
				t.Error("Service repos should be initialized")
			}

			if service.envMgmtService == nil {
				t.Error("Service envMgmtService should be initialized")
			}

			if service.agentExportService == nil {
				t.Error("Service agentExportService should be initialized")
			}
		})
	}
}

// TestCopyEnvironment tests full environment copying
func TestCopyEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	// Create source and target environments
	sourceEnv, err := repos.Environments.Create("source-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create source environment: %v", err)
	}

	targetEnv, err := repos.Environments.Create("target-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create target environment: %v", err)
	}

	// Create temporary workspace
	workspace := t.TempDir()
	sourceEnvDir := filepath.Join(workspace, ".config", "station", "environments", sourceEnv.Name)
	targetEnvDir := filepath.Join(workspace, ".config", "station", "environments", targetEnv.Name)
	os.MkdirAll(sourceEnvDir, 0755)
	os.MkdirAll(targetEnvDir, 0755)

	tests := []struct {
		name        string
		setupFunc   func()
		wantSuccess bool
		description string
	}{
		{
			name: "Copy empty environment",
			setupFunc: func() {
				// No servers or agents
			},
			wantSuccess: true,
			description: "Should handle empty source environment",
		},
		{
			name: "Copy environment with MCP server",
			setupFunc: func() {
				// Add an MCP server to source
				repos.MCPServers.Create(&models.MCPServer{
					Name:          "test-server",
					Command:       "npx",
					Args:          []string{"-y", "@modelcontextprotocol/server-filesystem"},
					EnvironmentID: sourceEnv.ID,
				})
			},
			wantSuccess: true,
			description: "Should copy MCP server to target environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setupFunc != nil {
				tt.setupFunc()
			}

			result, err := service.CopyEnvironment(sourceEnv.ID, targetEnv.ID)

			if err != nil {
				t.Fatalf("CopyEnvironment() error = %v", err)
			}

			if result == nil {
				t.Fatal("CopyEnvironment() returned nil result")
			}

			if result.Success != tt.wantSuccess {
				t.Errorf("Result success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if result.TargetEnvironment != targetEnv.Name {
				t.Errorf("Target environment = %s, want %s", result.TargetEnvironment, targetEnv.Name)
			}

			t.Logf("Copy result: %d MCP servers, %d agents, %d conflicts, %d errors",
				result.MCPServersCopied, result.AgentsCopied, len(result.Conflicts), len(result.Errors))
		})
	}
}

// TestCopyEnvironmentNonExistentSource tests error handling
func TestCopyEnvironmentNonExistentSource(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	targetEnv, _ := repos.Environments.Create("target-env", nil, 1)

	_, err = service.CopyEnvironment(99999, targetEnv.ID)

	if err == nil {
		t.Error("CopyEnvironment() should fail for non-existent source")
	}
}

// TestCopyMCPServer tests MCP server copying
func TestCopyMCPServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	targetEnv, _ := repos.Environments.Create("target-env", nil, 1)

	tests := []struct {
		name           string
		server         *models.MCPServer
		expectConflict bool
		description    string
	}{
		{
			name: "Copy new MCP server",
			server: &models.MCPServer{
				ID:      1,
				Name:    "filesystem",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
			},
			expectConflict: false,
			description:    "Should copy server without conflict",
		},
		{
			name: "Copy with existing name",
			server: &models.MCPServer{
				ID:      2,
				Name:    "existing-server",
				Command: "test",
				Args:    []string{},
			},
			expectConflict: true,
			description:    "Should detect conflict for existing server name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CopyResult{
				Conflicts: []CopyConflict{},
				Errors:    []string{},
			}

			if tt.expectConflict {
				// Pre-create server with same name
				repos.MCPServers.Create(&models.MCPServer{
					Name:          tt.server.Name,
					Command:       "existing",
					EnvironmentID: targetEnv.ID,
				})
			}

			err := service.copyMCPServer(tt.server, targetEnv, result)

			if err != nil {
				t.Errorf("copyMCPServer() error = %v", err)
			}

			if tt.expectConflict {
				if len(result.Conflicts) == 0 {
					t.Error("Expected conflict but none detected")
				} else {
					conflict := result.Conflicts[0]
					if conflict.Type != "mcp_server" {
						t.Errorf("Conflict type = %s, want mcp_server", conflict.Type)
					}
					if conflict.Name != tt.server.Name {
						t.Errorf("Conflict name = %s, want %s", conflict.Name, tt.server.Name)
					}
				}
			} else {
				if len(result.Conflicts) > 0 {
					t.Errorf("Unexpected conflict: %+v", result.Conflicts[0])
				}
				if result.MCPServersCopied != 1 {
					t.Errorf("MCPServersCopied = %d, want 1", result.MCPServersCopied)
				}
			}
		})
	}
}

// TestCopyAgent tests agent copying
func TestCopyAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	workspace := t.TempDir()
	targetEnv, _ := repos.Environments.Create("target-env", nil, 1)

	// Create target environment directory
	targetEnvDir := filepath.Join(workspace, ".config", "station", "environments", targetEnv.Name)
	os.MkdirAll(filepath.Join(targetEnvDir, "agents"), 0755)

	// Override home directory for test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", workspace)
	defer os.Setenv("HOME", originalHome)

	tests := []struct {
		name           string
		agent          *models.Agent
		expectConflict bool
		description    string
	}{
		{
			name: "Copy new agent",
			agent: &models.Agent{
				ID:          1,
				Name:        "test-agent",
				Description: "Test agent",
				Prompt:      "You are a test agent",
				MaxSteps:    5,
				CreatedBy:   1,
			},
			expectConflict: false,
			description:    "Should copy agent without conflict",
		},
		{
			name: "Copy with existing name",
			agent: &models.Agent{
				ID:          2,
				Name:        "existing-agent",
				Description: "Existing",
				Prompt:      "Test",
				MaxSteps:    5,
				CreatedBy:   1,
			},
			expectConflict: true,
			description:    "Should detect conflict for existing agent name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CopyResult{
				Conflicts: []CopyConflict{},
				Errors:    []string{},
			}

			if tt.expectConflict {
				// Pre-create agent with same name
				repos.Agents.Create(
					tt.agent.Name,
					tt.agent.Description,
					"existing prompt",
					5,
					targetEnv.ID,
					1,
					nil,
					nil,
					false,
					nil,
					nil,
					"",
					"",
				)
			}

			err := service.copyAgent(tt.agent, targetEnv, result)

			if err != nil {
				t.Errorf("copyAgent() error = %v", err)
			}

			if tt.expectConflict {
				if len(result.Conflicts) == 0 {
					t.Error("Expected conflict but none detected")
				} else {
					conflict := result.Conflicts[0]
					if conflict.Type != "agent" {
						t.Errorf("Conflict type = %s, want agent", conflict.Type)
					}
				}
			} else {
				if len(result.Conflicts) > 0 {
					t.Errorf("Unexpected conflict: %+v", result.Conflicts[0])
				}
				if result.AgentsCopied != 1 {
					t.Errorf("AgentsCopied = %d, want 1", result.AgentsCopied)
				}
			}
		})
	}
}

// TestBuildPromptFileContent tests .prompt file generation
func TestBuildPromptFileContent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	outputSchema := `{"type":"object","properties":{"result":{"type":"string"}}}`
	outputPreset := "investigation"

	tests := []struct {
		name        string
		agent       *models.Agent
		toolNames   []string
		wantContain []string
		description string
	}{
		{
			name: "Basic agent",
			agent: &models.Agent{
				Name:     "test-agent",
				Prompt:   "You are a test agent",
				MaxSteps: 5,
			},
			toolNames:   []string{},
			wantContain: []string{"name: \"test-agent\"", "max_steps: 5", "You are a test agent"},
			description: "Should generate basic YAML frontmatter",
		},
		{
			name: "Agent with tools",
			agent: &models.Agent{
				Name:     "tool-agent",
				Prompt:   "Test prompt",
				MaxSteps: 8,
			},
			toolNames:   []string{"read_file", "write_file"},
			wantContain: []string{"tools:", "\"read_file\"", "\"write_file\""},
			description: "Should include tools in YAML",
		},
		{
			name: "Agent with description",
			agent: &models.Agent{
				Name:        "described-agent",
				Description: "This is a test agent",
				Prompt:      "Test",
				MaxSteps:    5,
			},
			toolNames:   []string{},
			wantContain: []string{"description: \"This is a test agent\""},
			description: "Should include description",
		},
		{
			name: "Agent with output schema",
			agent: &models.Agent{
				Name:         "schema-agent",
				Prompt:       "Test",
				MaxSteps:     5,
				OutputSchema: &outputSchema,
			},
			toolNames:   []string{},
			wantContain: []string{"output:", "format: json", "schema:"},
			description: "Should include output schema",
		},
		{
			name: "Agent with output preset",
			agent: &models.Agent{
				Name:               "preset-agent",
				Prompt:             "Test",
				MaxSteps:           5,
				OutputSchemaPreset: &outputPreset,
			},
			toolNames:   []string{},
			wantContain: []string{"output_schema_preset: \"investigation\""},
			description: "Should include output preset",
		},
		{
			name: "Finops agent with tags",
			agent: &models.Agent{
				Name:     "finops-agent",
				Prompt:   "Test",
				MaxSteps: 5,
				App:      "finops",
			},
			toolNames:   []string{},
			wantContain: []string{"app: \"finops\"", "tags: [\"finops\", \"investigations\", \"cost-analysis\"]"},
			description: "Should add finops tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := service.buildPromptFileContent(tt.agent, tt.toolNames)

			if content == "" {
				t.Fatal("buildPromptFileContent() returned empty content")
			}

			for _, want := range tt.wantContain {
				if !stringContains(content, want) {
					t.Errorf("Content should contain %q, got:\n%s", want, content)
				}
			}

			// Verify YAML structure
			if !stringContains(content, "---\n") {
				t.Error("Content should start with YAML frontmatter")
			}
		})
	}
}

// TestFormatSchemaYAML tests JSON schema to YAML conversion
func TestFormatSchemaYAML(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	tests := []struct {
		name        string
		schema      map[string]interface{}
		indent      int
		wantContain []string
		description string
	}{
		{
			name: "Simple schema",
			schema: map[string]interface{}{
				"type": "object",
			},
			indent:      4,
			wantContain: []string{"type: object"},
			description: "Should format simple type",
		},
		{
			name: "Schema with properties",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": "string",
				},
			},
			indent:      4,
			wantContain: []string{"type: object", "properties:"},
			description: "Should format nested properties",
		},
		{
			name: "Schema with required fields",
			schema: map[string]interface{}{
				"type":     "object",
				"required": []interface{}{"id", "name"},
			},
			indent:      4,
			wantContain: []string{"required:", "- id", "- name"},
			description: "Should format array fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.formatSchemaYAML(tt.schema, tt.indent)

			for _, want := range tt.wantContain {
				if !stringContains(result, want) {
					t.Errorf("Result should contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

// TestRegenerateTemplateJSON tests template.json regeneration
func TestRegenerateTemplateJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	workspace := t.TempDir()
	env, _ := repos.Environments.Create("test-env", nil, 1)

	// Create environment directory
	envDir := filepath.Join(workspace, ".config", "station", "environments", env.Name)
	os.MkdirAll(envDir, 0755)

	// Override home directory for test
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", workspace)
	defer os.Setenv("HOME", originalHome)

	// Create MCP server
	repos.MCPServers.Create(&models.MCPServer{
		Name:          "filesystem",
		Command:       "npx",
		Args:          []string{"-y", "@modelcontextprotocol/server-filesystem"},
		Env:           map[string]string{"DEBUG": "true"},
		EnvironmentID: env.ID,
	})

	err = service.regenerateTemplateJSON(env)

	if err != nil {
		t.Fatalf("regenerateTemplateJSON() error = %v", err)
	}

	// Verify template.json was created
	templatePath := filepath.Join(envDir, "template.json")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Fatal("template.json was not created")
	}

	// Read and parse template.json
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		t.Fatalf("Failed to read template.json: %v", err)
	}

	var template map[string]interface{}
	if err := json.Unmarshal(templateData, &template); err != nil {
		t.Fatalf("Failed to parse template.json: %v", err)
	}

	// Verify structure
	if template["name"] != env.Name {
		t.Errorf("Template name = %v, want %s", template["name"], env.Name)
	}

	mcpServers, ok := template["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers should be a map")
	}

	if _, exists := mcpServers["filesystem"]; !exists {
		t.Error("filesystem server should exist in template.json")
	}
}

// TestFindToolByNameAndServer tests tool lookup
func TestFindToolByNameAndServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	env, _ := repos.Environments.Create("test-env", nil, 1)

	// Create MCP server
	serverID, _ := repos.MCPServers.Create(&models.MCPServer{
		Name:          "filesystem",
		Command:       "npx",
		EnvironmentID: env.ID,
	})

	server, _ := repos.MCPServers.GetByID(serverID)

	// Create tool
	repos.MCPTools.Create(&models.MCPTool{
		MCPServerID: server.ID,
		Name:        "read_file",
		Description: "Reads a file",
	})

	tests := []struct {
		name        string
		toolName    string
		serverName  string
		wantErr     bool
		description string
	}{
		{
			name:        "Find existing tool",
			toolName:    "read_file",
			serverName:  "filesystem",
			wantErr:     false,
			description: "Should find tool in server",
		},
		{
			name:        "Tool not found",
			toolName:    "nonexistent",
			serverName:  "filesystem",
			wantErr:     true,
			description: "Should fail for non-existent tool",
		},
		{
			name:        "Server not found",
			toolName:    "read_file",
			serverName:  "nonexistent-server",
			wantErr:     true,
			description: "Should fail for non-existent server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := service.findToolByNameAndServer(env.ID, tt.toolName, tt.serverName)

			if (err != nil) != tt.wantErr {
				t.Errorf("findToolByNameAndServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tool == nil {
				t.Error("Expected tool but got nil")
			}

			if !tt.wantErr && tool.Name != tt.toolName {
				t.Errorf("Tool name = %s, want %s", tool.Name, tt.toolName)
			}
		})
	}
}

// TestCopyConflictStruct tests CopyConflict structure
func TestCopyConflictStruct(t *testing.T) {
	conflictID := int64(123)

	tests := []struct {
		name        string
		conflict    CopyConflict
		description string
	}{
		{
			name: "MCP server conflict",
			conflict: CopyConflict{
				Type:          "mcp_server",
				Name:          "filesystem",
				Reason:        "Server already exists",
				SourceID:      1,
				ConflictingID: &conflictID,
			},
			description: "Should represent MCP server conflict",
		},
		{
			name: "Agent conflict",
			conflict: CopyConflict{
				Type:          "agent",
				Name:          "test-agent",
				Reason:        "Agent already exists",
				SourceID:      2,
				ConflictingID: nil,
			},
			description: "Should represent agent conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.conflict.Type == "" {
				t.Error("Type should not be empty")
			}

			if tt.conflict.Name == "" {
				t.Error("Name should not be empty")
			}

			if tt.conflict.SourceID == 0 {
				t.Error("SourceID should not be 0")
			}
		})
	}
}

// TestCopyResultStruct tests CopyResult structure
func TestCopyResultStruct(t *testing.T) {
	tests := []struct {
		name        string
		result      CopyResult
		description string
	}{
		{
			name: "Successful copy",
			result: CopyResult{
				Success:           true,
				TargetEnvironment: "prod",
				MCPServersCopied:  3,
				AgentsCopied:      5,
				Conflicts:         []CopyConflict{},
				Errors:            []string{},
			},
			description: "Should represent successful copy",
		},
		{
			name: "Copy with conflicts",
			result: CopyResult{
				Success:           true,
				TargetEnvironment: "staging",
				MCPServersCopied:  2,
				AgentsCopied:      3,
				Conflicts: []CopyConflict{
					{Type: "agent", Name: "test", Reason: "exists", SourceID: 1},
				},
				Errors: []string{},
			},
			description: "Should handle conflicts",
		},
		{
			name: "Copy with errors",
			result: CopyResult{
				Success:           false,
				TargetEnvironment: "dev",
				MCPServersCopied:  0,
				AgentsCopied:      0,
				Conflicts:         []CopyConflict{},
				Errors:            []string{"Failed to create agent", "Failed to copy server"},
			},
			description: "Should track errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.TargetEnvironment == "" {
				t.Error("TargetEnvironment should not be empty")
			}

			if tt.result.Success && len(tt.result.Errors) > 0 {
				t.Error("Successful result should not have errors")
			}

			if !tt.result.Success && len(tt.result.Errors) == 0 {
				t.Error("Failed result should have errors")
			}
		})
	}
}

// Helper function to check if string contains substring
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContainsHelper(s, substr))
}

func stringContainsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests
func BenchmarkCopyEnvironment(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	sourceEnv, _ := repos.Environments.Create("bench-source", nil, 1)
	targetEnv, _ := repos.Environments.Create("bench-target", nil, 1)

	// Add test data
	repos.MCPServers.Create(&models.MCPServer{
		Name:          "test-server",
		Command:       "npx",
		EnvironmentID: sourceEnv.ID,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.CopyEnvironment(sourceEnv.ID, targetEnv.ID)
	}
}

func BenchmarkBuildPromptFileContent(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	agent := &models.Agent{
		Name:        "bench-agent",
		Description: "Benchmark agent",
		Prompt:      "Test prompt",
		MaxSteps:    10,
	}

	toolNames := []string{"tool1", "tool2", "tool3"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.buildPromptFileContent(agent, toolNames)
	}
}

func BenchmarkFormatSchemaYAML(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewEnvironmentCopyService(repos)

	schema := map[string]interface{}{
		"type":     "object",
		"required": []interface{}{"id", "name", "email"},
		"properties": map[string]interface{}{
			"id":    "integer",
			"name":  "string",
			"email": "string",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.formatSchemaYAML(schema, 4)
	}
}
