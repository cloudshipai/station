package services

import (
	"os"
	"strings"
	"testing"
)

// TestNewDeploymentContextService tests service creation
func TestNewDeploymentContextService(t *testing.T) {
	service := NewDeploymentContextService()

	if service == nil {
		t.Fatal("NewDeploymentContextService() returned nil")
	}
}

// TestGatherContextForMode tests context gathering for different modes
func TestGatherContextForMode(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name         string
		mode         string
		envVars      map[string]string
		expectSource string
		description  string
	}{
		{
			name:         "CLI mode",
			mode:         "cli",
			envVars:      map[string]string{},
			expectSource: "analytics",
			description:  "Should gather CLI context",
		},
		{
			name:         "Stdio mode",
			mode:         "stdio",
			envVars:      map[string]string{},
			expectSource: "analytics",
			description:  "Should gather stdio context",
		},
		{
			name:         "Serve mode",
			mode:         "serve",
			envVars:      map[string]string{},
			expectSource: "analytics",
			description:  "Should gather server context",
		},
		{
			name:         "CLI mode with GitHub Actions",
			mode:         "cli",
			envVars:      map[string]string{"GITHUB_ACTIONS": "true"},
			expectSource: "analytics",
			description:  "Should detect GitHub Actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			context := service.GatherContextForMode(tt.mode)

			if context == nil {
				t.Fatal("GatherContextForMode() returned nil")
			}

			if context.Mode != tt.mode {
				t.Errorf("Mode = %s, want %s", context.Mode, tt.mode)
			}

			if context.Source != tt.expectSource {
				t.Errorf("Source = %s, want %s", context.Source, tt.expectSource)
			}

			if context.StationVersion == "" {
				t.Error("StationVersion should not be empty")
			}
		})
	}
}

// TestDeploymentContextToLabelsMap tests label map conversion
func TestDeploymentContextToLabelsMap(t *testing.T) {
	tests := []struct {
		name        string
		context     *DeploymentContext
		expectKeys  []string
		description string
	}{
		{
			name: "Minimal context",
			context: &DeploymentContext{
				Mode:            "cli",
				Source:          "analytics",
				CommandLine:     "stn agent run test",
				WorkingDirectory: "/workspace",
				StationVersion:  "v0.11.0",
			},
			expectKeys:  []string{"mode", "source", "command_line", "working_dir", "station_version"},
			description: "Should include only required fields",
		},
		{
			name: "Full context with git",
			context: &DeploymentContext{
				Mode:             "cli",
				Source:           "analytics",
				CommandLine:      "stn agent run test",
				WorkingDirectory: "/workspace",
				StationVersion:   "v0.11.0",
				GitBranch:        "main",
				GitCommit:        "abc123",
			},
			expectKeys:  []string{"mode", "source", "git_branch", "git_commit"},
			description: "Should include git fields when present",
		},
		{
			name: "Full context with CI",
			context: &DeploymentContext{
				Mode:             "cli",
				Source:           "analytics",
				CommandLine:      "stn agent run test",
				WorkingDirectory: "/workspace",
				StationVersion:   "v0.11.0",
				CIProvider:       "github_actions",
				Repository:       "user/repo",
				Workflow:         "CI",
			},
			expectKeys:  []string{"mode", "source", "ci_provider", "repository", "workflow"},
			description: "Should include CI fields when present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := tt.context.ToLabelsMap()

			if labels == nil {
				t.Fatal("ToLabelsMap() returned nil")
			}

			// Verify expected keys are present
			for _, key := range tt.expectKeys {
				if _, exists := labels[key]; !exists {
					t.Errorf("Expected key %s not found in labels", key)
				}
			}
		})
	}
}

// TestGetCommandLine tests command line extraction
func TestGetCommandLine(t *testing.T) {
	service := NewDeploymentContextService()

	cmdLine := service.getCommandLine()

	if cmdLine == "" {
		t.Error("getCommandLine() returned empty string")
	}

	// Should contain at least the test binary name
	if !strings.Contains(cmdLine, "test") {
		t.Logf("Command line: %s", cmdLine)
	}
}

// TestGetCurrentWorkingDir tests working directory extraction
func TestGetCurrentWorkingDir(t *testing.T) {
	service := NewDeploymentContextService()

	wd := service.getCurrentWorkingDir()

	if wd == "" {
		t.Error("getCurrentWorkingDir() returned empty string")
	}

	// Verify it's an absolute path
	if !strings.HasPrefix(wd, "/") {
		t.Errorf("Working directory should be absolute path, got: %s", wd)
	}
}

// TestGetStationVersion tests version extraction
func TestGetStationVersion(t *testing.T) {
	service := NewDeploymentContextService()

	version := service.getStationVersion()

	if version == "" {
		t.Error("getStationVersion() returned empty string")
	}

	if !strings.HasPrefix(version, "v") {
		t.Errorf("Version should start with 'v', got: %s", version)
	}
}

// TestGetGitBranch tests git branch detection
func TestGetGitBranch(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name       string
		envVar     string
		value      string
		wantBranch string
	}{
		{
			name:       "GitHub Actions branch",
			envVar:     "GITHUB_REF_NAME",
			value:      "main",
			wantBranch: "main",
		},
		{
			name:       "GitLab CI branch",
			envVar:     "CI_COMMIT_REF_NAME",
			value:      "develop",
			wantBranch: "develop",
		},
		{
			name:       "Generic GIT_BRANCH",
			envVar:     "GIT_BRANCH",
			value:      "feature/test",
			wantBranch: "feature/test",
		},
		{
			name:       "No environment variable",
			envVar:     "",
			value:      "",
			wantBranch: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("GITHUB_REF_NAME")
			os.Unsetenv("CI_COMMIT_REF_NAME")
			os.Unsetenv("GIT_BRANCH")

			// Set test environment variable
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.value)
				defer os.Unsetenv(tt.envVar)
			}

			branch := service.getGitBranch()

			if branch != tt.wantBranch {
				t.Errorf("getGitBranch() = %s, want %s", branch, tt.wantBranch)
			}
		})
	}
}

// TestGetGitCommit tests git commit detection
func TestGetGitCommit(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name       string
		envVar     string
		value      string
		wantCommit string
	}{
		{
			name:       "GitHub Actions commit",
			envVar:     "GITHUB_SHA",
			value:      "abc123",
			wantCommit: "abc123",
		},
		{
			name:       "GitLab CI commit",
			envVar:     "CI_COMMIT_SHA",
			value:      "def456",
			wantCommit: "def456",
		},
		{
			name:       "Generic GIT_COMMIT",
			envVar:     "GIT_COMMIT",
			value:      "xyz789",
			wantCommit: "xyz789",
		},
		{
			name:       "No environment variable",
			envVar:     "",
			value:      "",
			wantCommit: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("GITHUB_SHA")
			os.Unsetenv("CI_COMMIT_SHA")
			os.Unsetenv("GIT_COMMIT")

			// Set test environment variable
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.value)
				defer os.Unsetenv(tt.envVar)
			}

			commit := service.getGitCommit()

			if commit != tt.wantCommit {
				t.Errorf("getGitCommit() = %s, want %s", commit, tt.wantCommit)
			}
		})
	}
}

// TestGetCIProvider tests CI provider detection
func TestGetCIProvider(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name         string
		envVars      map[string]string
		wantProvider string
	}{
		{
			name:         "GitHub Actions",
			envVars:      map[string]string{"GITHUB_ACTIONS": "true"},
			wantProvider: "github_actions",
		},
		{
			name:         "GitLab CI",
			envVars:      map[string]string{"GITLAB_CI": "true"},
			wantProvider: "gitlab_ci",
		},
		{
			name:         "Jenkins",
			envVars:      map[string]string{"JENKINS_URL": "http://jenkins.example.com"},
			wantProvider: "jenkins",
		},
		{
			name:         "Travis CI",
			envVars:      map[string]string{"TRAVIS": "true"},
			wantProvider: "travis",
		},
		{
			name:         "CircleCI",
			envVars:      map[string]string{"CIRCLECI": "true"},
			wantProvider: "circleci",
		},
		{
			name:         "Unknown CI",
			envVars:      map[string]string{"CI": "true"},
			wantProvider: "unknown_ci",
		},
		{
			name:         "No CI environment",
			envVars:      map[string]string{},
			wantProvider: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("GITHUB_ACTIONS")
			os.Unsetenv("GITLAB_CI")
			os.Unsetenv("JENKINS_URL")
			os.Unsetenv("TRAVIS")
			os.Unsetenv("CIRCLECI")
			os.Unsetenv("CI")

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			provider := service.getCIProvider()

			if provider != tt.wantProvider {
				t.Errorf("getCIProvider() = %s, want %s", provider, tt.wantProvider)
			}
		})
	}
}

// TestGetRepositoryName tests repository name detection
func TestGetRepositoryName(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name     string
		envVar   string
		value    string
		wantRepo string
	}{
		{
			name:     "GitHub repository",
			envVar:   "GITHUB_REPOSITORY",
			value:    "user/repo",
			wantRepo: "user/repo",
		},
		{
			name:     "GitLab repository",
			envVar:   "CI_PROJECT_PATH",
			value:    "group/project",
			wantRepo: "group/project",
		},
		{
			name:     "Travis repository",
			envVar:   "TRAVIS_REPO_SLUG",
			value:    "owner/repo",
			wantRepo: "owner/repo",
		},
		{
			name:     "No environment variable",
			envVar:   "",
			value:    "",
			wantRepo: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("GITHUB_REPOSITORY")
			os.Unsetenv("CI_PROJECT_PATH")
			os.Unsetenv("TRAVIS_REPO_SLUG")

			// Set test environment variable
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.value)
				defer os.Unsetenv(tt.envVar)
			}

			repo := service.getRepositoryName()

			if repo != tt.wantRepo {
				t.Errorf("getRepositoryName() = %s, want %s", repo, tt.wantRepo)
			}
		})
	}
}

// TestGetWorkflowName tests workflow name detection
func TestGetWorkflowName(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name         string
		envVar       string
		value        string
		wantWorkflow string
	}{
		{
			name:         "GitHub workflow",
			envVar:       "GITHUB_WORKFLOW",
			value:        "CI",
			wantWorkflow: "CI",
		},
		{
			name:         "GitLab pipeline",
			envVar:       "CI_PIPELINE_NAME",
			value:        "build-and-test",
			wantWorkflow: "build-and-test",
		},
		{
			name:         "Jenkins job",
			envVar:       "JOB_NAME",
			value:        "deploy-prod",
			wantWorkflow: "deploy-prod",
		},
		{
			name:         "Travis job",
			envVar:       "TRAVIS_JOB_NAME",
			value:        "test",
			wantWorkflow: "test",
		},
		{
			name:         "No environment variable",
			envVar:       "",
			value:        "",
			wantWorkflow: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("GITHUB_WORKFLOW")
			os.Unsetenv("CI_PIPELINE_NAME")
			os.Unsetenv("JOB_NAME")
			os.Unsetenv("TRAVIS_JOB_NAME")

			// Set test environment variable
			if tt.envVar != "" {
				os.Setenv(tt.envVar, tt.value)
				defer os.Unsetenv(tt.envVar)
			}

			workflow := service.getWorkflowName()

			if workflow != tt.wantWorkflow {
				t.Errorf("getWorkflowName() = %s, want %s", workflow, tt.wantWorkflow)
			}
		})
	}
}

// TestGetStdioEnvironment tests stdio environment gathering
func TestGetStdioEnvironment(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name        string
		mcpClient   string
		expectEmpty bool
	}{
		{
			name:        "With MCP client",
			mcpClient:   "vscode",
			expectEmpty: false,
		},
		{
			name:        "Without MCP client",
			mcpClient:   "",
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Unsetenv("MCP_CLIENT")

			if tt.mcpClient != "" {
				os.Setenv("MCP_CLIENT", tt.mcpClient)
				defer os.Unsetenv("MCP_CLIENT")
			}

			env := service.getStdioEnvironment()

			if tt.expectEmpty && len(env) != 0 {
				t.Errorf("Expected empty environment, got %d entries", len(env))
			}

			if !tt.expectEmpty && len(env) == 0 {
				t.Error("Expected non-empty environment")
			}

			if tt.mcpClient != "" {
				if client, exists := env["mcp_client"]; !exists {
					t.Error("Expected mcp_client in environment")
				} else if client != tt.mcpClient {
					t.Errorf("mcp_client = %s, want %s", client, tt.mcpClient)
				}
			}
		})
	}
}

// TestGetServerEnvironment tests server environment gathering
func TestGetServerEnvironment(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name        string
		envVars     map[string]string
		expectKeys  []string
		description string
	}{
		{
			name: "Kubernetes environment",
			envVars: map[string]string{
				"KUBERNETES_NAMESPACE": "production",
				"HOSTNAME":             "station-pod-123",
				"CLUSTER_NAME":         "prod-cluster",
			},
			expectKeys:  []string{"k8s_namespace", "pod_name", "cluster"},
			description: "Should detect Kubernetes deployment",
		},
		{
			name:        "No Kubernetes environment",
			envVars:     map[string]string{},
			expectKeys:  []string{},
			description: "Should return empty environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("KUBERNETES_NAMESPACE")
			os.Unsetenv("HOSTNAME")
			os.Unsetenv("CLUSTER_NAME")

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			env := service.getServerEnvironment()

			// Verify expected keys
			for _, key := range tt.expectKeys {
				if _, exists := env[key]; !exists {
					t.Errorf("Expected key %s not found in environment", key)
				}
			}
		})
	}
}

// TestGetCIEnvironment tests CI environment gathering
func TestGetCIEnvironment(t *testing.T) {
	service := NewDeploymentContextService()

	tests := []struct {
		name        string
		envVars     map[string]string
		expectKeys  []string
		description string
	}{
		{
			name: "GitHub Actions environment",
			envVars: map[string]string{
				"GITHUB_ACTIONS":     "true",
				"GITHUB_WORKFLOW":    "CI",
				"GITHUB_REPOSITORY":  "user/repo",
				"GITHUB_REF":         "refs/heads/main",
				"GITHUB_SHA":         "abc123",
			},
			expectKeys:  []string{"GITHUB_ACTIONS", "GITHUB_WORKFLOW", "GITHUB_REPOSITORY"},
			description: "Should collect GitHub Actions variables",
		},
		{
			name: "GitLab CI environment",
			envVars: map[string]string{
				"GITLAB_CI":        "true",
				"CI_PROJECT_PATH":  "group/project",
				"CI_PIPELINE_NAME": "build",
			},
			expectKeys:  []string{"GITLAB_CI", "CI_PROJECT_PATH", "CI_PIPELINE_NAME"},
			description: "Should collect GitLab CI variables",
		},
		{
			name:        "No CI environment",
			envVars:     map[string]string{},
			expectKeys:  []string{},
			description: "Should return empty environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			ciVars := []string{
				"GITHUB_ACTIONS", "GITHUB_WORKFLOW", "GITHUB_REPOSITORY",
				"GITHUB_REF", "GITHUB_SHA", "RUNNER_OS", "CI",
				"GITLAB_CI", "CI_PROJECT_PATH", "CI_PIPELINE_NAME",
				"JENKINS_URL", "JOB_NAME", "BUILD_NUMBER",
				"TRAVIS", "TRAVIS_REPO_SLUG", "TRAVIS_JOB_NAME",
				"CIRCLECI", "CIRCLE_PROJECT_REPONAME", "CIRCLE_JOB",
			}

			for _, key := range ciVars {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			env := service.getCIEnvironment()

			// Verify expected keys
			for _, key := range tt.expectKeys {
				if _, exists := env[key]; !exists {
					t.Errorf("Expected key %s not found in environment", key)
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewDeploymentContextService(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewDeploymentContextService()
	}
}

func BenchmarkGatherContextForMode(b *testing.B) {
	service := NewDeploymentContextService()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = service.GatherContextForMode("cli")
	}
}

func BenchmarkToLabelsMap(b *testing.B) {
	context := &DeploymentContext{
		Mode:             "cli",
		Source:           "analytics",
		CommandLine:      "stn agent run test",
		WorkingDirectory: "/workspace",
		GitBranch:        "main",
		GitCommit:        "abc123",
		StationVersion:   "v0.11.0",
		CIProvider:       "github_actions",
		Repository:       "user/repo",
		Workflow:         "CI",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = context.ToLabelsMap()
	}
}
