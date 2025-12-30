package services

import (
	"os"
	"path/filepath"

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
	if cfg.Backend == "" || cfg.Backend != "opencode" {
		logging.Debug("Coding backend not configured or not opencode, coding tools disabled")
		return &CodingToolFactory{enabled: false}
	}

	backend := coding.NewOpenCodeBackend(cfg)

	basePath := cfg.WorkspaceBasePath
	if basePath == "" {
		basePath = filepath.Join(os.TempDir(), "station-coding")
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

	workspaceManager := coding.NewWorkspaceManager(
		coding.WithBasePath(basePath),
		coding.WithCleanupPolicy(cleanupPolicy),
		coding.WithGitCredentials(gitCreds),
	)

	logging.Info("Coding tool factory initialized with OpenCode backend (URL: %s, workspace: %s)", cfg.OpenCode.URL, basePath)

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

func (f *CodingToolFactory) GetCodingTools(codingCfg *dotprompt.CodingConfig) []ai.Tool {
	if !f.ShouldAddTools(codingCfg) {
		return nil
	}

	toolFactory := coding.NewToolFactory(f.backend, coding.WithWorkspaceManager(f.workspaceManager))
	return toolFactory.CreateAllTools()
}

func (f *CodingToolFactory) GetWorkspaceManager() *coding.WorkspaceManager {
	return f.workspaceManager
}

func (f *CodingToolFactory) GetBackend() coding.Backend {
	return f.backend
}
