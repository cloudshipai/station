package services

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"station/internal/coding"
	"station/internal/config"
	"station/internal/logging"
	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

type CodingToolFactory struct {
	backend          coding.Backend
	workspaceManager *coding.WorkspaceManager
	enabled          bool
}

func NewCodingToolFactory(cfg config.CodingConfig) *CodingToolFactory {
	var backend coding.Backend
	var err error

	switch cfg.Backend {
	case "opencode":
		backend = coding.NewOpenCodeBackend(cfg)
	case "opencode-nats":
		backend, err = coding.NewNATSBackend(cfg)
		if err != nil {
			logging.Error("Failed to create NATS backend: %v, coding tools disabled", err)
			return &CodingToolFactory{enabled: false}
		}
	case "opencode-cli":
		backend = coding.NewCLIBackend(cfg)
	case "claudecode":
		backend = coding.NewClaudeCodeBackend(cfg)
	default:
		logging.Debug("Coding backend not configured or unsupported (%s), coding tools disabled", cfg.Backend)
		return &CodingToolFactory{enabled: false}
	}

	basePath := cfg.WorkspaceBasePath
	if basePath == "" {
		if cfg.Backend == "opencode" || cfg.Backend == "opencode-nats" {
			basePath = "/workspaces/station-coding"
		} else if _, err := os.Stat("/workspaces"); err == nil {
			basePath = "/workspaces/station-coding"
		} else {
			basePath = filepath.Join(os.TempDir(), "station-coding")
		}
	}

	cleanupPolicy := coding.CleanupOnSessionEnd
	if cfg.CleanupPolicy == "on_success" {
		cleanupPolicy = coding.CleanupOnSuccess
	} else if cfg.CleanupPolicy == "manual" {
		cleanupPolicy = coding.CleanupManual
	}

	var gitCreds *coding.GitCredentials
	if cfg.Git.TokenEnvVar != "" || cfg.Git.Token != "" {
		gitCreds = coding.NewGitCredentials(cfg.Git.Token, cfg.Git.TokenEnvVar)
		if gitCreds.HasToken() {
			logging.Debug("Git credentials configured for coding operations")
		}
	}

	cloneTimeout := coding.CloneTimeoutFromConfig(cfg)
	pushTimeout := coding.PushTimeoutFromConfig(cfg)

	workspaceManager := coding.NewWorkspaceManager(
		coding.WithBasePath(basePath),
		coding.WithCleanupPolicy(cleanupPolicy),
		coding.WithGitCredentials(gitCreds),
		coding.WithCloneTimeout(cloneTimeout),
		coding.WithPushTimeout(pushTimeout),
	)

	switch cfg.Backend {
	case "opencode-nats":
		logging.Info("Coding tool factory initialized with NATS backend (NATS: %s, workspace: %s)", cfg.NATS.URL, basePath)
	case "opencode-cli":
		binaryPath := cfg.CLI.BinaryPath
		if binaryPath == "" {
			binaryPath = "opencode"
		}
		logging.Info("Coding tool factory initialized with CLI backend (binary: %s, workspace: %s)", binaryPath, basePath)
	case "claudecode":
		binaryPath := cfg.ClaudeCode.BinaryPath
		if binaryPath == "" {
			binaryPath = "claude"
		}
		logging.Info("Coding tool factory initialized with Claude Code backend (binary: %s, workspace: %s)", binaryPath, basePath)
	default:
		logging.Info("Coding tool factory initialized with OpenCode backend (URL: %s, workspace: %s)", cfg.OpenCode.URL, basePath)
	}

	return &CodingToolFactory{
		backend:          backend,
		workspaceManager: workspaceManager,
		enabled:          true,
	}
}

func (f *CodingToolFactory) IsEnabled() bool {
	return f.enabled && f.backend != nil
}

func (f *CodingToolFactory) ShouldAddTools(codingCfg *dotprompt.CodingConfig) bool {
	if codingCfg == nil || !codingCfg.Enabled {
		return false
	}
	return f.IsEnabled()
}

func (f *CodingToolFactory) GetCodingTools(codingCfg *dotprompt.CodingConfig, execCtx ExecutionContext) []ai.Tool {
	if !f.ShouldAddTools(codingCfg) {
		return nil
	}

	codingExecCtx := coding.ExecutionContext{
		WorkflowRunID: execCtx.WorkflowRunID,
		AgentRunID:    execCtx.AgentRunID,
	}
	toolFactory := coding.NewToolFactory(f.backend, coding.WithWorkspaceManager(f.workspaceManager), coding.WithExecutionContext(codingExecCtx))
	return toolFactory.CreateAllTools()
}

func (f *CodingToolFactory) GetWorkspaceManager() *coding.WorkspaceManager {
	return f.workspaceManager
}

func (f *CodingToolFactory) GetBackend() coding.Backend {
	return f.backend
}

func (f *CodingToolFactory) CheckHealth(ctx context.Context) error {
	if !f.IsEnabled() {
		return nil
	}
	healthCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return f.backend.Ping(healthCtx)
}

func (f *CodingToolFactory) CleanupWorkflowWorkspace(ctx context.Context, workflowRunID string) {
	if !f.IsEnabled() || f.workspaceManager == nil {
		return
	}
	ws, err := f.workspaceManager.GetByScope(coding.ScopeWorkflow, workflowRunID)
	if err != nil {
		return
	}
	logging.Info("Cleaning up coding workspace for workflow %s (workspace: %s)", workflowRunID, ws.ID)
	f.workspaceManager.Cleanup(ctx, ws)
}
