package services

import (
	"os"
	"strings"
	"testing"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
)

// TestNewAgentExportService tests service creation
func TestNewAgentExportService(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)

	tests := []struct {
		name        string
		repos       *repositories.Repositories
		expectNil   bool
		description string
	}{
		{
			name:        "Valid service creation",
			repos:       repos,
			expectNil:   false,
			description: "Should create service with valid repositories",
		},
		{
			name:        "Service creation with nil repos",
			repos:       nil,
			expectNil:   false,
			description: "Should still create service (may panic on use)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewAgentExportService(tt.repos)

			if tt.expectNil {
				if service != nil {
					t.Errorf("Expected nil service, got %v", service)
				}
			} else {
				if service == nil {
					t.Error("Expected non-nil service")
				} else {
					if service.schemaRegistry == nil {
						t.Error("Schema registry should be initialized")
					}
				}
			}
		})
	}
}

// TestExportAgentAfterSave tests agent export to file-based config
func TestExportAgentAfterSave(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewAgentExportService(repos)

	// Note: Tests use default workspace from config.GetAgentPromptPath

	// Create test environment
	env, err := repos.Environments.Create("test-export-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create test agent
	agentService := NewAgentService(repos)
	agent, err := agentService.CreateAgent(nil, &AgentConfig{
		Name:          "test-export-agent",
		Description:   "Test export agent",
		Prompt:        "You are a test agent.",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	tests := []struct {
		name        string
		agentID     int64
		wantErr     bool
		description string
	}{
		{
			name:        "Export valid agent",
			agentID:     agent.ID,
			wantErr:     false,
			description: "Should export agent to .prompt file",
		},
		{
			name:        "Export non-existent agent",
			agentID:     99999,
			wantErr:     true,
			description: "Should fail for non-existent agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ExportAgentAfterSave(tt.agentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExportAgentAfterSave() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify .prompt file was created
				promptPath := config.GetAgentPromptPath(env.Name, agent.Name)
				if _, err := os.Stat(promptPath); os.IsNotExist(err) {
					t.Errorf("Prompt file not created at %s", promptPath)
				}

				// Read and verify content
				content, err := os.ReadFile(promptPath)
				if err != nil {
					t.Fatalf("Failed to read prompt file: %v", err)
				}

				// Verify YAML frontmatter
				if !strings.HasPrefix(string(content), "---") {
					t.Error("Prompt file should start with YAML frontmatter")
				}

				// Verify agent metadata
				if !strings.Contains(string(content), agent.Name) {
					t.Errorf("Prompt file should contain agent name: %s", agent.Name)
				}

				if !strings.Contains(string(content), agent.Description) {
					t.Errorf("Prompt file should contain agent description: %s", agent.Description)
				}

				// Verify role structure
				if !strings.Contains(string(content), "{{role") {
					t.Error("Prompt file should contain role templates")
				}
			}
		})
	}
}

// TestExportAgentAfterSaveWithMetadata tests export with CloudShip metadata
func TestExportAgentAfterSaveWithMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewAgentExportService(repos)

	// Note: Tests use default workspace from config.GetAgentPromptPath

	// Create test environment
	env, err := repos.Environments.Create("test-metadata-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	// Create test agent
	agentService := NewAgentService(repos)
	agent, err := agentService.CreateAgent(nil, &AgentConfig{
		Name:          "test-metadata-agent",
		Description:   "Test metadata agent",
		Prompt:        "You are a test agent.",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	tests := []struct {
		name        string
		app         string
		appType     string
		wantErr     bool
		description string
	}{
		{
			name:        "Export with CloudShip metadata",
			app:         "test-app",
			appType:     "test-type",
			wantErr:     false,
			description: "Should include app and app_type in frontmatter",
		},
		{
			name:        "Export without metadata",
			app:         "",
			appType:     "",
			wantErr:     false,
			description: "Should export without CloudShip metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ExportAgentAfterSaveWithMetadata(agent.ID, tt.app, tt.appType)

			if (err != nil) != tt.wantErr {
				t.Errorf("ExportAgentAfterSaveWithMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				promptPath := config.GetAgentPromptPath(env.Name, agent.Name)
				content, err := os.ReadFile(promptPath)
				if err != nil {
					t.Fatalf("Failed to read prompt file: %v", err)
				}

				contentStr := string(content)

				// Verify metadata presence
				if tt.app != "" && !strings.Contains(contentStr, "app:") {
					t.Error("Prompt file should contain app metadata")
				}
				if tt.appType != "" && !strings.Contains(contentStr, "app_type:") {
					t.Error("Prompt file should contain app_type metadata")
				}
			}
		})
	}
}

// TestGenerateDotpromptContent tests dotprompt content generation
func TestGenerateDotpromptContent(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewAgentExportService(repos)

	// Create test environment and agent
	env, err := repos.Environments.Create("test-dotprompt-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agentService := NewAgentService(repos)
	agent, err := agentService.CreateAgent(nil, &AgentConfig{
		Name:          "test-dotprompt-agent",
		Description:   "Test dotprompt generation",
		Prompt:        "You are a test agent.",
		MaxSteps:      5,
		EnvironmentID: env.ID,
		CreatedBy:     1,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Get agent tools (should be empty for this test)
	tools, err := repos.AgentTools.ListAgentTools(agent.ID)
	if err != nil {
		t.Fatalf("Failed to get agent tools: %v", err)
	}

	tests := []struct {
		name              string
		app               string
		appType           string
		expectApp         bool
		expectAppType     bool
		expectFrontmatter bool
		description       string
	}{
		{
			name:              "Generate with CloudShip metadata",
			app:               "test-app",
			appType:           "test-type",
			expectApp:         true,
			expectAppType:     true,
			expectFrontmatter: true,
			description:       "Should include app and app_type in generated content",
		},
		{
			name:              "Generate without metadata",
			app:               "",
			appType:           "",
			expectApp:         false,
			expectAppType:     false,
			expectFrontmatter: true,
			description:       "Should generate content without CloudShip metadata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := service.generateDotpromptContent(agent, tools, env.Name, tt.app, tt.appType)

			// Verify frontmatter
			if tt.expectFrontmatter && !strings.HasPrefix(content, "---") {
				t.Error("Content should start with YAML frontmatter")
			}

			// Verify agent metadata
			if !strings.Contains(content, agent.Name) {
				t.Error("Content should contain agent name")
			}

			if !strings.Contains(content, agent.Description) {
				t.Error("Content should contain agent description")
			}

			// Verify CloudShip metadata
			if tt.expectApp != strings.Contains(content, "app:") {
				t.Errorf("Content app metadata presence = %v, want %v", strings.Contains(content, "app:"), tt.expectApp)
			}

			if tt.expectAppType != strings.Contains(content, "app_type:") {
				t.Errorf("Content app_type metadata presence = %v, want %v", strings.Contains(content, "app_type:"), tt.expectAppType)
			}

			// Verify role structure
			if !strings.Contains(content, "{{role") {
				t.Error("Content should contain role templates")
			}
		})
	}
}

// TestSplitLines tests line splitting utility
func TestSplitLines(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
	}{
		{
			name:      "Empty string",
			input:     "",
			wantCount: 0,
		},
		{
			name:      "Single line",
			input:     "test",
			wantCount: 1,
		},
		{
			name:      "Multiple lines",
			input:     "line1\nline2\nline3",
			wantCount: 3,
		},
		{
			name:      "Lines with trailing newline",
			input:     "line1\nline2\n",
			wantCount: 3, // Includes empty string after final \n
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLines(tt.input)

			if len(result) != tt.wantCount {
				t.Errorf("splitLines() count = %d, want %d", len(result), tt.wantCount)
			}
		})
	}
}

// TestIndentLines tests line indentation utility
func TestIndentLines(t *testing.T) {
	service := &AgentExportService{}

	tests := []struct {
		name       string
		input      string
		prefix     string
		wantPrefix string
	}{
		{
			name:       "Indent with spaces",
			input:      "line1\nline2",
			prefix:     "  ",
			wantPrefix: "  line1\n  line2\n",
		},
		{
			name:       "Indent with tabs",
			input:      "line1\nline2",
			prefix:     "\t",
			wantPrefix: "\tline1\n\tline2\n",
		},
		{
			name:       "Empty input",
			input:      "",
			prefix:     "  ",
			wantPrefix: "",
		},
		{
			name:       "Input with empty lines",
			input:      "line1\n\nline3",
			prefix:     "  ",
			wantPrefix: "  line1\n\n  line3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.indentLines(tt.input, tt.prefix)

			if result != tt.wantPrefix {
				t.Errorf("indentLines() = %q, want %q", result, tt.wantPrefix)
			}
		})
	}
}

// TestIndentJSON tests JSON indentation utility
func TestIndentJSON(t *testing.T) {
	service := &AgentExportService{}

	tests := []struct {
		name       string
		input      string
		prefix     string
		wantPrefix string
	}{
		{
			name:       "Indent JSON with spaces",
			input:      `{"key":"value"}`,
			prefix:     "  ",
			wantPrefix: "  {\"key\":\"value\"}\n",
		},
		{
			name:       "Indent multiline JSON",
			input:      "{\n\"key\": \"value\"\n}",
			prefix:     "  ",
			wantPrefix: "  {\n  \"key\": \"value\"\n  }\n",
		},
		{
			name:       "Indent with tabs",
			input:      "line1\nline2",
			prefix:     "\t",
			wantPrefix: "\tline1\n\tline2\n",
		},
		{
			name:       "Empty JSON",
			input:      "",
			prefix:     "  ",
			wantPrefix: "",
		},
		{
			name:       "JSON with empty lines",
			input:      "{\n\n}",
			prefix:     "  ",
			wantPrefix: "  {\n\n  }\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.indentJSON(tt.input, tt.prefix)

			if result != tt.wantPrefix {
				t.Errorf("indentJSON() = %q, want %q", result, tt.wantPrefix)
			}
		})
	}
}

// TestConvertJSONSchemaToYAML tests JSON to YAML conversion
func TestConvertJSONSchemaToYAML(t *testing.T) {
	service := &AgentExportService{}

	tests := []struct {
		name      string
		input     string
		wantEmpty bool
		wantYAML  bool
	}{
		{
			name:      "Valid JSON schema",
			input:     `{"type": "object", "properties": {"test": {"type": "string"}}}`,
			wantEmpty: false,
			wantYAML:  true,
		},
		{
			name:      "Already YAML format",
			input:     "type: object\nproperties:\n  test:\n    type: string",
			wantEmpty: false,
			wantYAML:  true,
		},
		{
			name:      "Invalid JSON",
			input:     `{invalid json`,
			wantEmpty: true,
			wantYAML:  false,
		},
		{
			name:      "Empty string",
			input:     "",
			wantEmpty: false, // Already YAML format (empty)
			wantYAML:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.convertJSONSchemaToYAML(tt.input)

			if tt.wantEmpty && result != "" {
				t.Errorf("convertJSONSchemaToYAML() = %q, want empty", result)
			}

			if !tt.wantEmpty && result == "" && tt.wantYAML {
				t.Error("convertJSONSchemaToYAML() returned empty, want non-empty YAML")
			}
		})
	}
}

// TestExtractCustomVariableNames tests custom variable extraction
func TestExtractCustomVariableNames(t *testing.T) {
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)
	service := NewAgentExportService(repos)

	// Create test environment
	env, err := repos.Environments.Create("test-vars-env", nil, 1)
	if err != nil {
		t.Fatalf("Failed to create environment: %v", err)
	}

	agentService := NewAgentService(repos)

	tests := []struct {
		name         string
		inputSchema  *string
		wantVarCount int
		description  string
	}{
		{
			name:         "Agent with no input schema",
			inputSchema:  nil,
			wantVarCount: 0,
			description:  "Should return empty array for nil schema",
		},
		{
			name:         "Agent with empty input schema",
			inputSchema:  stringPtr(""),
			wantVarCount: 0,
			description:  "Should return empty array for empty schema",
		},
		{
			name:         "Agent with invalid JSON schema",
			inputSchema:  stringPtr(`{invalid json`),
			wantVarCount: 0,
			description:  "Should return empty array for invalid JSON",
		},
		{
			name:         "Agent with custom variables",
			inputSchema:  stringPtr(`{"customVar1":"value1","customVar2":"value2","userInput":"should be excluded"}`),
			wantVarCount: 2,
			description:  "Should extract variables excluding userInput",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := agentService.CreateAgent(nil, &AgentConfig{
				Name:          "test-vars-agent-" + tt.name,
				Description:   "Test variable extraction",
				Prompt:        "Test",
				MaxSteps:      5,
				EnvironmentID: env.ID,
				CreatedBy:     1,
			})
			if err != nil {
				t.Fatalf("Failed to create agent: %v", err)
			}

			// Update input schema if provided
			if tt.inputSchema != nil {
				agent.InputSchema = tt.inputSchema
			}

			varNames := service.extractCustomVariableNames(agent)

			if len(varNames) != tt.wantVarCount {
				t.Errorf("extractCustomVariableNames() count = %d, want %d", len(varNames), tt.wantVarCount)
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewAgentExportService(b *testing.B) {
	testDB, err := db.NewTest(b)
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer func() { _ = testDB.Close() }()

	repos := repositories.New(testDB)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewAgentExportService(repos)
	}
}

func BenchmarkSplitLines(b *testing.B) {
	input := "line1\nline2\nline3\nline4\nline5"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = splitLines(input)
	}
}

func BenchmarkConvertJSONSchemaToYAML(b *testing.B) {
	service := &AgentExportService{}
	input := `{"type": "object", "properties": {"test": {"type": "string"}}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.convertJSONSchemaToYAML(input)
	}
}
