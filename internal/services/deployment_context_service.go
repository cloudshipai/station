package services

import (
	"os"
	"strings"
)

// DeploymentContextService handles gathering deployment context information
// This service is responsible for detecting CI/CD environments and collecting
// relevant metadata for agent execution tracking.
type DeploymentContextService struct{}

// NewDeploymentContextService creates a new deployment context service
func NewDeploymentContextService() *DeploymentContextService {
	return &DeploymentContextService{}
}

// DeploymentContext represents the context in which Station is running
type DeploymentContext struct {
	Mode             string            `json:"mode"`
	Source           string            `json:"source"`
	CommandLine      string            `json:"command_line"`
	WorkingDirectory string            `json:"working_directory"`
	GitBranch        string            `json:"git_branch,omitempty"`
	GitCommit        string            `json:"git_commit,omitempty"`
	StationVersion   string            `json:"station_version"`
	CIProvider       string            `json:"ci_provider,omitempty"`
	Repository       string            `json:"repository,omitempty"`
	Workflow         string            `json:"workflow,omitempty"`
	Environment      map[string]string `json:"environment,omitempty"`
}

// GatherContextForMode collects deployment context based on the execution mode
func (dcs *DeploymentContextService) GatherContextForMode(mode string) *DeploymentContext {
	context := &DeploymentContext{
		Mode:             mode,
		CommandLine:      dcs.getCommandLine(),
		WorkingDirectory: dcs.getCurrentWorkingDir(),
		GitBranch:        dcs.getGitBranch(),
		GitCommit:        dcs.getGitCommit(),
		StationVersion:   dcs.getStationVersion(),
	}

	switch mode {
	case "stdio":
		context.Source = "analytics"
		context.Environment = dcs.getStdioEnvironment()
	case "serve":
		context.Source = "analytics"
		context.Environment = dcs.getServerEnvironment()
	case "cli":
		context.Source = "analytics"
		context.CIProvider = dcs.getCIProvider()
		context.Repository = dcs.getRepositoryName()
		context.Workflow = dcs.getWorkflowName()
		context.Environment = dcs.getCIEnvironment()
	}

	return context
}

// ToLabelsMap converts deployment context to a labels map for Lighthouse
func (dc *DeploymentContext) ToLabelsMap() map[string]string {
	labels := map[string]string{
		"mode":            dc.Mode,
		"source":          dc.Source,
		"command_line":    dc.CommandLine,
		"working_dir":     dc.WorkingDirectory,
		"station_version": dc.StationVersion,
	}

	// Add optional fields only if they have values
	if dc.GitBranch != "" {
		labels["git_branch"] = dc.GitBranch
	}
	if dc.GitCommit != "" {
		labels["git_commit"] = dc.GitCommit
	}
	if dc.CIProvider != "" {
		labels["ci_provider"] = dc.CIProvider
	}
	if dc.Repository != "" {
		labels["repository"] = dc.Repository
	}
	if dc.Workflow != "" {
		labels["workflow"] = dc.Workflow
	}

	return labels
}

// Private helper methods for gathering context

func (dcs *DeploymentContextService) getCommandLine() string {
	return strings.Join(os.Args, " ")
}

func (dcs *DeploymentContextService) getCurrentWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

func (dcs *DeploymentContextService) getStationVersion() string {
	// TODO: Import version package to get actual version
	return "v0.11.0"
}

// Git context methods
func (dcs *DeploymentContextService) getGitBranch() string {
	// Try environment variables first (CI/CD)
	if branch := os.Getenv("GITHUB_REF_NAME"); branch != "" {
		return branch
	}
	if branch := os.Getenv("CI_COMMIT_REF_NAME"); branch != "" {
		return branch
	}
	if branch := os.Getenv("GIT_BRANCH"); branch != "" {
		return branch
	}

	// TODO: Could add git command execution as fallback
	// exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	return ""
}

func (dcs *DeploymentContextService) getGitCommit() string {
	// Try environment variables first (CI/CD)
	if commit := os.Getenv("GITHUB_SHA"); commit != "" {
		return commit
	}
	if commit := os.Getenv("CI_COMMIT_SHA"); commit != "" {
		return commit
	}
	if commit := os.Getenv("GIT_COMMIT"); commit != "" {
		return commit
	}

	// TODO: Could add git command execution as fallback
	// exec.Command("git", "rev-parse", "HEAD")
	return ""
}

// CI/CD detection methods
func (dcs *DeploymentContextService) getCIProvider() string {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		return "github_actions"
	}
	if os.Getenv("GITLAB_CI") == "true" {
		return "gitlab_ci"
	}
	if os.Getenv("JENKINS_URL") != "" {
		return "jenkins"
	}
	if os.Getenv("TRAVIS") == "true" {
		return "travis"
	}
	if os.Getenv("CIRCLECI") == "true" {
		return "circleci"
	}
	if os.Getenv("CI") == "true" {
		return "unknown_ci"
	}
	return ""
}

func (dcs *DeploymentContextService) getRepositoryName() string {
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		return repo
	}
	if repo := os.Getenv("CI_PROJECT_PATH"); repo != "" {
		return repo
	}
	if repo := os.Getenv("TRAVIS_REPO_SLUG"); repo != "" {
		return repo
	}
	return ""
}

func (dcs *DeploymentContextService) getWorkflowName() string {
	if workflow := os.Getenv("GITHUB_WORKFLOW"); workflow != "" {
		return workflow
	}
	if pipeline := os.Getenv("CI_PIPELINE_NAME"); pipeline != "" {
		return pipeline
	}
	if job := os.Getenv("JOB_NAME"); job != "" {
		return job
	}
	if job := os.Getenv("TRAVIS_JOB_NAME"); job != "" {
		return job
	}
	return ""
}

// Environment-specific context gathering
func (dcs *DeploymentContextService) getStdioEnvironment() map[string]string {
	env := make(map[string]string)

	// Collect MCP client information
	if client := os.Getenv("MCP_CLIENT"); client != "" {
		env["mcp_client"] = client
	}

	return env
}

func (dcs *DeploymentContextService) getServerEnvironment() map[string]string {
	env := make(map[string]string)

	// Collect server deployment information
	if namespace := os.Getenv("KUBERNETES_NAMESPACE"); namespace != "" {
		env["k8s_namespace"] = namespace
	}
	if pod := os.Getenv("HOSTNAME"); pod != "" {
		env["pod_name"] = pod
	}
	if cluster := os.Getenv("CLUSTER_NAME"); cluster != "" {
		env["cluster"] = cluster
	}

	return env
}

func (dcs *DeploymentContextService) getCIEnvironment() map[string]string {
	env := make(map[string]string)

	// Collect CI/CD specific environment variables
	relevantKeys := []string{
		"GITHUB_ACTIONS", "GITHUB_WORKFLOW", "GITHUB_REPOSITORY",
		"GITHUB_REF", "GITHUB_SHA", "RUNNER_OS", "CI",
		"GITLAB_CI", "CI_PROJECT_PATH", "CI_PIPELINE_NAME",
		"JENKINS_URL", "JOB_NAME", "BUILD_NUMBER",
		"TRAVIS", "TRAVIS_REPO_SLUG", "TRAVIS_JOB_NAME",
		"CIRCLECI", "CIRCLE_PROJECT_REPONAME", "CIRCLE_JOB",
	}

	for _, key := range relevantKeys {
		if value := os.Getenv(key); value != "" {
			env[key] = value
		}
	}

	return env
}
