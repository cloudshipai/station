package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNewTemplateVariableService tests service initialization
func TestNewTemplateVariableService(t *testing.T) {
	tmpDir := t.TempDir()

	service := NewTemplateVariableService(tmpDir, nil)

	if service == nil {
		t.Fatal("NewTemplateVariableService returned nil")
	}

	if service.configDir != tmpDir {
		t.Errorf("configDir = %s, want %s", service.configDir, tmpDir)
	}
}

// TestSetVariableResolver tests setting custom resolver
func TestSetVariableResolver(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	called := false
	resolver := func(missingVars []VariableInfo) (map[string]string, error) {
		called = true
		return map[string]string{}, nil
	}

	service.SetVariableResolver(resolver)

	if service.variableResolver == nil {
		t.Error("variableResolver not set")
	}

	// Test that resolver can be called
	_, _ = service.variableResolver([]VariableInfo{})
	if !called {
		t.Error("variableResolver was not called")
	}
}

// TestHasTemplateVariables tests variable detection
func TestHasTemplateVariables(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	tests := []struct {
		name     string
		template string
		want     bool
	}{
		{
			name:     "Template with simple variable",
			template: `{"url": "{{ .API_URL }}"}`,
			want:     true,
		},
		{
			name:     "Template with multiple variables",
			template: `{"url": "{{ .API_URL }}", "key": "{{ .API_KEY }}"}`,
			want:     true,
		},
		{
			name:     "Template without variables",
			template: `{"url": "http://example.com"}`,
			want:     false,
		},
		{
			name:     "Template with conditional",
			template: `{{ if .DEBUG }}debug{{ else }}prod{{ end }}`,
			want:     true,
		},
		{
			name:     "Template with range",
			template: `{{ range .ITEMS }}{{ . }}{{ end }}`,
			want:     true,
		},
		{
			name:     "Empty template",
			template: ``,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.HasTemplateVariables(tt.template)
			if got != tt.want {
				t.Errorf("HasTemplateVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRenderTemplate tests template rendering
func TestRenderTemplate(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	tests := []struct {
		name      string
		template  string
		variables map[string]string
		want      string
		wantErr   bool
	}{
		{
			name:     "Simple variable substitution",
			template: `{"url": "{{ .API_URL }}"}`,
			variables: map[string]string{
				"API_URL": "http://api.example.com",
			},
			want:    `{"url": "http://api.example.com"}`,
			wantErr: false,
		},
		{
			name:     "Multiple variables",
			template: `{"url": "{{ .URL }}", "key": "{{ .KEY }}"}`,
			variables: map[string]string{
				"URL": "http://example.com",
				"KEY": "secret",
			},
			want:    `{"url": "http://example.com", "key": "secret"}`,
			wantErr: false,
		},
		{
			name:     "Missing variable",
			template: `{"url": "{{ .MISSING }}"}`,
			variables: map[string]string{
				"OTHER": "value",
			},
			want:    "",
			wantErr: true,
		},
		{
			name:      "No variables needed",
			template:  `{"url": "http://example.com"}`,
			variables: map[string]string{},
			want:      `{"url": "http://example.com"}`,
			wantErr:   false,
		},
		{
			name:      "Empty template",
			template:  "",
			variables: map[string]string{},
			want:      "",
			wantErr:   false,
		},
		{
			name:     "Template with conditional",
			template: `{{ if .DEBUG }}debug{{ else }}prod{{ end }}`,
			variables: map[string]string{
				"DEBUG": "true",
			},
			want:    "debug",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.renderTemplate(tt.template, tt.variables)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("renderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestIsSystemEnvVar tests system variable filtering
func TestIsSystemEnvVar(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	tests := []struct {
		name string
		key  string
		want bool
	}{
		// System variables that should be filtered
		{"PATH variable", "PATH", true},
		{"HOME variable", "HOME", true},
		{"USER variable", "USER", true},
		{"PWD variable", "PWD", true},
		{"SHELL variable", "SHELL", true},
		{"TERM variable", "TERM", true},
		{"LANG variable", "LANG", true},
		{"GOPATH variable", "GOPATH", true},
		{"GOROOT variable", "GOROOT", true},
		{"GO variables", "GOTOOLDIR", true},
		{"Underscore prefix", "_", true},
		{"Internal SHLVL", "SHLVL", true},

		// User variables that should NOT be filtered
		{"Custom API_URL", "API_URL", false},
		{"Custom DATABASE_URL", "DATABASE_URL", false},
		{"Custom API_KEY", "API_KEY", false},
		{"Custom SERVICE_NAME", "SERVICE_NAME", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.isSystemEnvVar(tt.key)
			if got != tt.want {
				t.Errorf("isSystemEnvVar(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestIsSecretVariable tests secret variable detection
func TestIsSecretVariable(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	tests := []struct {
		name string
		key  string
		want bool
	}{
		// Should be marked as secrets
		{"API_KEY", "API_KEY", true},
		{"SECRET", "SECRET", true},
		{"PASSWORD", "PASSWORD", true},
		{"TOKEN", "TOKEN", true},
		{"CREDENTIALS", "CREDENTIALS", true},
		{"PRIVATE_KEY", "PRIVATE_KEY", true},
		{"AWS_SECRET_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY", true},

		// Should NOT be marked as secrets
		{"API_URL", "API_URL", false},
		{"DATABASE_HOST", "DATABASE_HOST", false},
		{"PORT", "PORT", false},
		{"ENVIRONMENT", "ENVIRONMENT", false},
		{"LOG_LEVEL", "LOG_LEVEL", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.isSecretVariable(tt.key)
			if got != tt.want {
				t.Errorf("isSecretVariable(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// TestLoadEnvironmentVariables tests loading variables from file
func TestLoadEnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTemplateVariableService(tmpDir, nil)

	// Create environment directory with variables.yml
	envDir := filepath.Join(tmpDir, "environments", "test-env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create env dir: %v", err)
	}

	variablesFile := filepath.Join(envDir, "variables.yml")
	variablesContent := `API_URL: http://api.example.com
API_KEY: secret123
PORT: 8080
DEBUG: true
`

	if err := os.WriteFile(variablesFile, []byte(variablesContent), 0644); err != nil {
		t.Fatalf("Failed to write variables.yml: %v", err)
	}

	tests := []struct {
		name     string
		envName  string
		wantVars map[string]string
		wantErr  bool
	}{
		{
			name:    "Valid environment with variables",
			envName: "test-env",
			wantVars: map[string]string{
				"API_URL": "http://api.example.com",
				"API_KEY": "secret123",
				"PORT":    "8080",
				"DEBUG":   "true",
			},
			wantErr: false,
		},
		{
			name:     "Non-existent environment",
			envName:  "nonexistent",
			wantVars: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.loadEnvironmentVariables(tt.envName)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadEnvironmentVariables() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				for key, wantVal := range tt.wantVars {
					gotVal, ok := got[key]
					if !ok {
						t.Errorf("loadEnvironmentVariables() missing key %q", key)
					}
					if gotVal != wantVal {
						t.Errorf("loadEnvironmentVariables()[%q] = %q, want %q", key, gotVal, wantVal)
					}
				}
			}
		})
	}
}

// TestExtractMissingVariableFromError tests error parsing
func TestExtractMissingVariableFromError(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "Standard missing variable error",
			err:  service.renderTemplateErr("template: :1:3: executing \"\" at <.MISSING>: map has no entry for key \"MISSING\""),
			want: "MISSING",
		},
		{
			name: "Different variable name",
			err:  service.renderTemplateErr("template: :1:10: executing \"\" at <.API_KEY>: map has no entry for key \"API_KEY\""),
			want: "API_KEY",
		},
		{
			name: "Non-template error",
			err:  service.renderTemplateErr("some other error"),
			want: "",
		},
		{
			name: "Empty error",
			err:  service.renderTemplateErr(""),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.extractMissingVariableFromError(tt.err)
			if got != tt.want {
				t.Errorf("extractMissingVariableFromError() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Helper to create error from string for testing
func (tvs *TemplateVariableService) renderTemplateErr(errStr string) error {
	if errStr == "" {
		return nil
	}
	// Return error with the provided message string
	return &templateError{msg: errStr}
}

type templateError struct {
	msg string
}

func (e *templateError) Error() string {
	return e.msg
}

// TestVariableResolutionEdgeCases tests edge cases
func TestVariableResolutionEdgeCases(t *testing.T) {
	service := NewTemplateVariableService("", nil)

	tests := []struct {
		name     string
		template string
		setup    func(*testing.T)
		wantErr  bool
	}{
		{
			name:     "Template with special characters in value",
			template: `{"url": "{{ .URL }}"}`,
			setup: func(t *testing.T) {
				os.Setenv("URL", "http://example.com/path?key=value&foo=bar")
			},
			wantErr: false,
		},
		{
			name:     "Template with quotes in value",
			template: `{"message": "{{ .MSG }}"}`,
			setup: func(t *testing.T) {
				os.Setenv("MSG", `He said "hello"`)
			},
			wantErr: false,
		},
		{
			name:     "Template with newlines",
			template: "Line 1: {{ .VAR1 }}\nLine 2: {{ .VAR2 }}",
			setup: func(t *testing.T) {
				os.Setenv("VAR1", "value1")
				os.Setenv("VAR2", "value2")
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setup != nil {
				tt.setup(t)
				defer func() {
					// Cleanup env vars
					os.Unsetenv("URL")
					os.Unsetenv("MSG")
					os.Unsetenv("VAR1")
					os.Unsetenv("VAR2")
				}()
			}

			// Load environment variables for test
			vars := make(map[string]string)
			for _, envPair := range os.Environ() {
				parts := strings.SplitN(envPair, "=", 2)
				if len(parts) == 2 && !service.isSystemEnvVar(parts[0]) {
					vars[parts[0]] = parts[1]
				}
			}

			_, err := service.renderTemplate(tt.template, vars)
			if (err != nil) != tt.wantErr {
				t.Errorf("renderTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestVariableOverridePrecedence tests variable precedence
func TestVariableOverridePrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	service := NewTemplateVariableService(tmpDir, nil)

	// Create environment with variables.yml
	envDir := filepath.Join(tmpDir, "environments", "test-env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("Failed to create env dir: %v", err)
	}

	variablesFile := filepath.Join(envDir, "variables.yml")
	variablesContent := `TEST_VAR: from_file
ANOTHER_VAR: only_in_file
`

	if err := os.WriteFile(variablesFile, []byte(variablesContent), 0644); err != nil {
		t.Fatalf("Failed to write variables.yml: %v", err)
	}

	// Set environment variable that should override file
	os.Setenv("TEST_VAR", "from_env")
	defer os.Unsetenv("TEST_VAR")

	vars, err := service.loadEnvironmentVariables("test-env")
	if err != nil {
		t.Fatalf("loadEnvironmentVariables() failed: %v", err)
	}

	// Load environment variables
	for _, envPair := range os.Environ() {
		parts := strings.SplitN(envPair, "=", 2)
		if len(parts) == 2 && !service.isSystemEnvVar(parts[0]) {
			vars[parts[0]] = parts[1]
		}
	}

	// Check precedence: environment variable should override file
	if vars["TEST_VAR"] != "from_env" {
		t.Errorf("TEST_VAR = %q, want %q (env should override file)", vars["TEST_VAR"], "from_env")
	}

	// Check file-only variable still exists
	if vars["ANOTHER_VAR"] != "only_in_file" {
		t.Errorf("ANOTHER_VAR = %q, want %q", vars["ANOTHER_VAR"], "only_in_file")
	}
}

// Benchmark tests
func BenchmarkRenderTemplate(b *testing.B) {
	service := NewTemplateVariableService("", nil)
	template := `{"url": "{{ .API_URL }}", "key": "{{ .API_KEY }}", "region": "{{ .REGION }}"}`
	vars := map[string]string{
		"API_URL": "http://api.example.com",
		"API_KEY": "secret123",
		"REGION":  "us-east-1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = service.renderTemplate(template, vars)
	}
}

func BenchmarkHasTemplateVariables(b *testing.B) {
	service := NewTemplateVariableService("", nil)
	template := `{"url": "{{ .API_URL }}", "key": "{{ .API_KEY }}"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.HasTemplateVariables(template)
	}
}
