// Package harness provides an agentic execution harness for Station.
// It implements a Claude Agent SDK-like execution model with manual loop control,
// pre/post tool hooks, doom loop detection, and context compaction.
package harness

import (
	"time"
)

// HarnessConfig is the top-level configuration for the agentic harness.
// It is loaded from stn.yaml under the "harness" key.
type HarnessConfig struct {
	Workspace   WorkspaceConfig   `yaml:"workspace" json:"workspace"`
	Compaction  CompactionConfig  `yaml:"compaction" json:"compaction"`
	Git         GitConfig         `yaml:"git" json:"git"`
	NATS        NATSConfig        `yaml:"nats" json:"nats"`
	Permissions PermissionsConfig `yaml:"permissions" json:"permissions"`
}

// WorkspaceConfig defines workspace settings.
type WorkspaceConfig struct {
	// Path is the workspace directory path (relative to station root).
	// Default: "./workspace"
	Path string `yaml:"path" json:"path"`

	// Mode is the workspace isolation mode.
	// Options: "host" (raw filesystem), "sandbox" (Docker), "agentfs" (SQLite)
	// Default: "host"
	Mode string `yaml:"mode" json:"mode"`

	// IsolationMode controls how workspaces are isolated for concurrent runs.
	// Options:
	//   - "shared": All runs share the base workspace (NOT RECOMMENDED for concurrent)
	//   - "per_run": Each agent run gets its own subdirectory (basePath/<run_id>/)
	//   - "per_workflow": Workflow steps share workspace, isolated between workflows
	// Default: "per_workflow"
	IsolationMode string `yaml:"isolation_mode" json:"isolation_mode"`

	// CleanupOnComplete removes the workspace after successful completion.
	// Default: false
	CleanupOnComplete bool `yaml:"cleanup_on_complete" json:"cleanup_on_complete"`

	// CleanupAfter specifies duration after which to cleanup completed workspaces.
	// Only applies when CleanupOnComplete is false.
	// Default: "24h"
	CleanupAfter string `yaml:"cleanup_after" json:"cleanup_after"`
}

// GitSourceConfig defines settings for cloning a git repo into the workspace.
// This allows agents to work on specific repositories.
type GitSourceConfig struct {
	// URL is the git repository URL to clone.
	// Supports HTTPS and SSH URLs.
	// Example: "https://github.com/org/repo.git" or "git@github.com:org/repo.git"
	URL string `yaml:"url" json:"url"`

	// Branch is the branch to checkout after cloning.
	// Default: repository's default branch (usually "main")
	Branch string `yaml:"branch" json:"branch"`

	// Ref is a specific commit SHA, tag, or ref to checkout.
	// Takes precedence over Branch if both are specified.
	Ref string `yaml:"ref" json:"ref"`

	// Depth for shallow clone (0 = full clone).
	// Default: 1 (shallow clone for faster setup)
	Depth int `yaml:"depth" json:"depth"`

	// Submodules controls whether to initialize submodules.
	// Default: false
	Submodules bool `yaml:"submodules" json:"submodules"`

	// SSHKeyPath is the path to SSH key for private repos.
	// If empty, uses default SSH agent.
	SSHKeyPath string `yaml:"ssh_key_path" json:"ssh_key_path"`
}

// ExecutionContext provides runtime context for workspace isolation.
// This is passed at execution time, not from config.
type ExecutionContext struct {
	// SessionID is a user-provided identifier for workspace persistence across runs.
	// If provided, takes precedence over WorkflowRunID and AgentRunID.
	// Use this to continue work from a previous run.
	SessionID string
	// WorkflowRunID is the workflow run ID (for per_workflow isolation).
	WorkflowRunID string
	// AgentRunID is the agent run ID (for per_run isolation).
	AgentRunID string
	// AgentName for branch naming.
	AgentName string
	// Task description for branch naming.
	Task string
}

// CompactionConfig defines context compaction settings.
type CompactionConfig struct {
	// Enabled enables automatic context compaction.
	// Default: true
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Threshold is the percentage of context window that triggers compaction.
	// Default: 0.85 (85%)
	Threshold float64 `yaml:"threshold" json:"threshold"`

	// ProtectTokens is the number of recent tokens to keep protected from pruning.
	// Default: 40000
	ProtectTokens int `yaml:"protect_tokens" json:"protect_tokens"`
}

// GitConfig defines git workflow settings.
type GitConfig struct {
	// AutoBranch enables automatic branch creation for each task.
	// Default: true
	AutoBranch bool `yaml:"auto_branch" json:"auto_branch"`

	// BranchPrefix is the prefix for agent-created branches.
	// Default: "agent/"
	BranchPrefix string `yaml:"branch_prefix" json:"branch_prefix"`

	// AutoCommit enables automatic commit on task completion.
	// Default: false
	AutoCommit bool `yaml:"auto_commit" json:"auto_commit"`

	// RequireApproval requires human approval before push/PR.
	// Default: true
	RequireApproval bool `yaml:"require_approval" json:"require_approval"`

	// WorkflowBranchStrategy defines how branches are managed in workflows.
	// Options: "shared" (all agents on same branch), "per_agent" (new branch per agent)
	// Default: "shared"
	WorkflowBranchStrategy string `yaml:"workflow_branch_strategy" json:"workflow_branch_strategy"`
}

// NATSConfig defines NATS integration settings for inter-agent communication.
type NATSConfig struct {
	// Enabled enables NATS integration.
	// Default: true
	Enabled bool `yaml:"enabled" json:"enabled"`

	// KVBucket is the NATS KV bucket name for state.
	// Default: "harness-state"
	KVBucket string `yaml:"kv_bucket" json:"kv_bucket"`

	// ObjectBucket is the NATS Object Store bucket name for files.
	// Default: "harness-files"
	ObjectBucket string `yaml:"object_bucket" json:"object_bucket"`

	// MaxFileSize is the maximum file size for object store.
	// Default: "100MB"
	MaxFileSize string `yaml:"max_file_size" json:"max_file_size"`

	// TTL is the time-to-live for stored artifacts.
	// Default: "24h"
	TTL string `yaml:"ttl" json:"ttl"`
}

// PermissionsConfig defines command permissions (OpenCode-style).
type PermissionsConfig struct {
	// ExternalDirectory controls access to paths outside workspace.
	// Options: "deny", "ask", "allow"
	// Default: "deny"
	ExternalDirectory string `yaml:"external_directory" json:"external_directory"`

	// Bash is a map of command patterns to actions.
	// Actions: "allow", "deny", "ask"
	// Patterns support wildcards: "*" matches any sequence
	// Example: {"*": "allow", "rm -rf *": "deny", "git push *": "ask"}
	Bash map[string]string `yaml:"bash" json:"bash"`
}

// AgentHarnessConfig is per-agent configuration that overrides global settings.
type AgentHarnessConfig struct {
	// MaxSteps is the maximum number of agentic loop iterations.
	// Default: 50
	MaxSteps int `yaml:"max_steps" json:"max_steps"`

	// DoomLoopThreshold is the number of identical consecutive tool calls
	// before triggering doom loop detection.
	// Default: 3
	DoomLoopThreshold int `yaml:"doom_loop_threshold" json:"doom_loop_threshold"`

	// Timeout is the maximum execution time for the entire task.
	// Default: 30m
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// DefaultHarnessConfig returns the default harness configuration.
func DefaultHarnessConfig() *HarnessConfig {
	return &HarnessConfig{
		Workspace: WorkspaceConfig{
			Path:              "./workspace",
			Mode:              "host",
			IsolationMode:     "per_workflow",
			CleanupOnComplete: false,
			CleanupAfter:      "24h",
		},
		Compaction: CompactionConfig{
			Enabled:       true,
			Threshold:     0.85,
			ProtectTokens: 40000,
		},
		Git: GitConfig{
			AutoBranch:             true,
			BranchPrefix:           "agent/",
			AutoCommit:             false,
			RequireApproval:        true,
			WorkflowBranchStrategy: "shared",
		},
		NATS: NATSConfig{
			Enabled:      true,
			KVBucket:     "harness-state",
			ObjectBucket: "harness-files",
			MaxFileSize:  "100MB",
			TTL:          "24h",
		},
		Permissions: PermissionsConfig{
			ExternalDirectory: "deny",
			Bash: map[string]string{
				"*":                 "allow",
				"rm -rf *":          "deny",
				"rm -r *":           "deny",
				"rm -f /*":          "deny",
				"chmod 777 *":       "deny",
				"chmod -R 777 *":    "deny",
				"curl * | sh":       "deny",
				"curl * | bash":     "deny",
				"wget * | sh":       "deny",
				"wget * | bash":     "deny",
				"find * -delete*":   "deny",
				"find * -exec rm *": "deny",
				"git push --force*": "deny",
				"git reset --hard*": "deny",
				"git push *":        "ask",
				"git commit *":      "ask",
			},
		},
	}
}

// DefaultAgentHarnessConfig returns the default per-agent configuration.
func DefaultAgentHarnessConfig() *AgentHarnessConfig {
	return &AgentHarnessConfig{
		MaxSteps:          50,
		DoomLoopThreshold: 3,
		Timeout:           30 * time.Minute,
	}
}

// Merge merges agent-specific config with global config.
func (c *AgentHarnessConfig) Merge(global *HarnessConfig) *AgentHarnessConfig {
	merged := *c
	if merged.MaxSteps == 0 {
		merged.MaxSteps = DefaultAgentHarnessConfig().MaxSteps
	}
	if merged.DoomLoopThreshold == 0 {
		merged.DoomLoopThreshold = DefaultAgentHarnessConfig().DoomLoopThreshold
	}
	if merged.Timeout == 0 {
		merged.Timeout = DefaultAgentHarnessConfig().Timeout
	}
	return &merged
}

// ResolveWorkspacePath determines the workspace path based on isolation mode and execution context.
func ResolveWorkspacePath(basePath string, isolationMode string, execCtx *ExecutionContext) string {
	if execCtx == nil {
		return basePath
	}

	switch isolationMode {
	case "per_workflow":
		if execCtx.WorkflowRunID != "" {
			return basePath + "/workflow/" + execCtx.WorkflowRunID
		}
		if execCtx.AgentRunID != "" {
			return basePath + "/agent/" + execCtx.AgentRunID
		}
	case "per_run":
		if execCtx.AgentRunID != "" {
			return basePath + "/run/" + execCtx.AgentRunID
		}
	case "shared":
		return basePath
	}

	return basePath
}
