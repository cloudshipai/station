package template

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"station/pkg/config"
)

func TestGoTemplateEngine_ExtractVariables(t *testing.T) {
	engine := NewGoTemplateEngine()
	ctx := context.Background()

	tests := []struct {
		name         string
		template     string
		expectedVars []string
		expectSecret []string
	}{
		{
			name:         "simple variables",
			template:     `{"server": "{{.ServerName}}", "key": "{{.ApiKey}}"}`,
			expectedVars: []string{"ServerName", "ApiKey"},
			expectSecret: []string{"ApiKey"},
		},
		{
			name:         "nested variables",
			template:     `{"github": {"token": "{{.GitHub.Token}}", "repo": "{{.GitHub.Repository}}"}}`,
			expectedVars: []string{"GitHub.Token", "GitHub.Repository"},
			expectSecret: []string{"GitHub.Token"},
		},
		{
			name:         "variables with functions",
			template:     `{"name": "{{.Name | default \"test\"}}", "secret": "{{.Secret | required \"Secret is required\"}}"}`,
			expectedVars: []string{"Name", "Secret"},
			expectSecret: []string{"Secret"},
		},
		{
			name:         "complex template",
			template: `{
				"mcpServers": {
					"{{.ServerName}}": {
						"command": "{{.Command}}",
						"args": {{.Args | toJSON}},
						"env": {
							"API_TOKEN": "{{.ApiToken}}",
							"PASSWORD": "{{.DatabasePassword}}",
							"DEBUG": "{{.Debug | default false}}"
						}
					}
				}
			}`,
			expectedVars: []string{"ServerName", "Command", "Args", "ApiToken", "DatabasePassword", "Debug"},
			expectSecret: []string{"ApiToken", "DatabasePassword"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			variables, err := engine.ExtractVariables(ctx, tt.template)
			if err != nil {
				t.Fatalf("ExtractVariables() error = %v", err)
			}

			// Check if all expected variables are found
			varMap := make(map[string]config.TemplateVariable)
			for _, v := range variables {
				varMap[v.Name] = v
			}

			for _, expectedVar := range tt.expectedVars {
				if _, found := varMap[expectedVar]; !found {
					t.Errorf("Expected variable %s not found", expectedVar)
				}
			}

			// Check secret detection
			for _, expectedSecret := range tt.expectSecret {
				if variable, found := varMap[expectedSecret]; found {
					if !variable.Secret {
						t.Errorf("Variable %s should be marked as secret", expectedSecret)
					}
				}
			}
		})
	}
}

func TestGoTemplateEngine_ParseAndRender(t *testing.T) {
	engine := NewGoTemplateEngine()
	ctx := context.Background()

	tests := []struct {
		name      string
		template  string
		variables map[string]interface{}
		expected  string
		shouldErr bool
	}{
		{
			name:     "simple substitution",
			template: `{"server": "{{.ServerName}}", "port": {{.Port}}}`,
			variables: map[string]interface{}{
				"ServerName": "test-server",
				"Port":       8080,
			},
			expected: `{"server": "test-server", "port": 8080}`,
		},
		{
			name:     "with default values",
			template: `{"debug": {{.Debug | default false}}, "timeout": {{.Timeout | default 30}}}`,
			variables: map[string]interface{}{
				"Debug": true,
			},
			expected: `{"debug": true, "timeout": 30}`,
		},
		{
			name:     "array handling",
			template: `{"args": {{.Args | toJSON}}}`,
			variables: map[string]interface{}{
				"Args": []string{"--verbose", "--config", "/etc/app.conf"},
			},
			expected: `{"args": ["--verbose","--config","/etc/app.conf"]}`,
		},
		{
			name:     "nested object access",
			template: `{"token": "{{.GitHub.Token}}", "repo": "{{.GitHub.Repository}}"}`,
			variables: map[string]interface{}{
				"GitHub": map[string]interface{}{
					"Token":      "ghp_token123",
					"Repository": "owner/repo",
				},
			},
			expected: `{"token": "ghp_token123", "repo": "owner/repo"}`,
		},
		{
			name:     "required field missing",
			template: `{"key": "{{required \"API key is required\" .ApiKey}}"}`,
			variables: map[string]interface{}{
				"SomeOtherField": "value",
			},
			shouldErr: true,
		},
		{
			name:     "string functions",
			template: `{"upper": "{{.Name | toUpper}}", "lower": "{{.Name | toLower}}"}`,
			variables: map[string]interface{}{
				"Name": "TestServer",
			},
			expected: `{"upper": "TESTSERVER", "lower": "testserver"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse template
			parsed, err := engine.Parse(ctx, tt.template)
			if err != nil {
				if !tt.shouldErr {
					t.Fatalf("Parse() error = %v, wantErr %v", err, tt.shouldErr)
				}
				return
			}

			// Render template
			result, err := engine.Render(ctx, parsed, tt.variables)
			if err != nil {
				if !tt.shouldErr {
					t.Fatalf("Render() error = %v, wantErr %v", err, tt.shouldErr)
				}
				return
			}

			if tt.shouldErr {
				t.Fatalf("Expected error but got result: %s", result)
			}

			// Normalize whitespace for comparison
			result = normalizeJSON(result)
			expected := normalizeJSON(tt.expected)

			if result != expected {
				t.Errorf("Render() = %v, want %v", result, expected)
			}
		})
	}
}

func TestGoTemplateEngine_Validate(t *testing.T) {
	engine := NewGoTemplateEngine()
	ctx := context.Background()

	tests := []struct {
		name        string
		template    string
		shouldError bool
	}{
		{
			name: "valid template",
			template: `{
				"mcpServers": {
					"{{.ServerName}}": {
						"command": "{{.Command}}",
						"env": {"API_KEY": "{{.ApiKey}}"}
					}
				}
			}`,
			shouldError: false,
		},
		{
			name:        "invalid Go template syntax",
			template:    `{"server": "{{.ServerName"}`,
			shouldError: true,
		},
		{
			name:        "invalid JSON structure after rendering",
			template:    `{"server": {{.ServerName}}, "port": {{.Port}}}`, // Missing quotes around ServerName
			shouldError: true,
		},
		{
			name: "valid template with functions",
			template: `{
				"server": "{{.ServerName | default \"default-server\"}}",
				"args": {{.Args | toJSON}},
				"debug": {{.Debug | default false}}
			}`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.Validate(ctx, tt.template)
			if (err != nil) != tt.shouldError {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.shouldError)
			}
		})
	}
}

func TestSecretDetection(t *testing.T) {
	engine := NewGoTemplateEngine()

	tests := []struct {
		varName  string
		isSecret bool
	}{
		{"ApiKey", true},
		{"api_key", true},
		{"Password", true},
		{"Secret", true},
		{"Token", true},
		{"AuthToken", true},
		{"PrivateKey", true},
		{"Credential", true},
		
		{"ServerName", false},
		{"Port", false},
		{"Debug", false},
		{"Timeout", false},
		{"Region", false},
	}

	for _, tt := range tests {
		t.Run(tt.varName, func(t *testing.T) {
			result := engine.isSecretVariable(tt.varName)
			if result != tt.isSecret {
				t.Errorf("isSecretVariable(%s) = %v, want %v", tt.varName, result, tt.isSecret)
			}
		})
	}
}

func TestMCPConfigTemplateWorkflow(t *testing.T) {
	// Test the complete workflow for MCP configuration templates
	engine := NewGoTemplateEngine()
	ctx := context.Background()

	// Test template that represents a real MCP configuration
	mcpTemplate := `{
  "mcpServers": {
    "{{.GitHub.ServerName | default \"github\"}}": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{required \"GitHub token is required\" .GitHub.Token}}",
        "GITHUB_REPOSITORY": "{{.GitHub.Repository | default \"owner/repo\"}}"
      }
    },
    "{{.AWS.ServerName | default \"aws-tools\"}}": {
      "command": "aws-mcp-server",
      "args": ["--region", "{{.AWS.Region | default \"us-east-1\"}}"],
      "env": {
        "AWS_ACCESS_KEY_ID": "{{required \"AWS access key is required\" .AWS.AccessKey}}",
        "AWS_SECRET_ACCESS_KEY": "{{required \"AWS secret key is required\" .AWS.SecretKey}}",
        "AWS_DEFAULT_REGION": "{{.AWS.Region | default \"us-east-1\"}}"
      }
    }
  }
}`

	// Variables that would come from .env files
	variables := map[string]interface{}{
		"GitHub": map[string]interface{}{
			"Token":      "ghp_test_token_xxxxxxxxxx",
			"Repository": "myorg/myrepo",
		},
		"AWS": map[string]interface{}{
			"AccessKey": "AKIATEST123EXAMPLE",
			"SecretKey": "test_secret_key_xxxxxxxxxx",
			"Region":    "us-west-2",
		},
	}

	// Test variable extraction
	extractedVars, err := engine.ExtractVariables(ctx, mcpTemplate)
	if err != nil {
		t.Fatalf("ExtractVariables() error = %v", err)
	}

	expectedVars := []string{
		"GitHub.ServerName", "GitHub.Token", "GitHub.Repository", 
		"AWS.ServerName", "AWS.AccessKey", "AWS.SecretKey", "AWS.Region",
	}

	varMap := make(map[string]bool)
	for _, v := range extractedVars {
		varMap[v.Name] = true
	}

	for _, expectedVar := range expectedVars {
		if !varMap[expectedVar] {
			t.Errorf("Expected variable %s not found in extracted variables", expectedVar)
		}
	}

	// Test template parsing
	parsed, err := engine.Parse(ctx, mcpTemplate)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	// Test template rendering
	rendered, err := engine.Render(ctx, parsed, variables)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Verify the rendered result is valid JSON
	var result interface{}
	if err := json.Unmarshal([]byte(rendered), &result); err != nil {
		t.Fatalf("Rendered template is not valid JSON: %v", err)
	}

	// Verify specific values were substituted correctly
	if !contains(rendered, "ghp_test_token_xxxxxxxxxx") {
		t.Error("GitHub token not substituted correctly")
	}
	if !contains(rendered, "AKIATEST123EXAMPLE") {
		t.Error("AWS access key not substituted correctly")
	}
	if !contains(rendered, "us-west-2") {
		t.Error("AWS region not substituted correctly")
	}

	t.Logf("Successfully rendered MCP config template:\n%s", rendered)
}

// Helper functions

func normalizeJSON(s string) string {
	// Remove all whitespace to compare JSON content
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "\n", "")
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}