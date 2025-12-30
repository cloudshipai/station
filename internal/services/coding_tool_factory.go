package services

import (
	"station/internal/coding"
	"station/internal/config"
	"station/internal/logging"
	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

type CodingToolFactory struct {
	backend coding.Backend
	enabled bool
}

func NewCodingToolFactory(cfg config.CodingConfig) *CodingToolFactory {
	if cfg.Backend == "" || cfg.Backend != "opencode" {
		logging.Debug("Coding backend not configured or not opencode, coding tools disabled")
		return &CodingToolFactory{enabled: false}
	}

	backend := coding.NewOpenCodeBackend(cfg)

	logging.Info("Coding tool factory initialized with OpenCode backend (URL: %s)", cfg.OpenCode.URL)

	return &CodingToolFactory{
		backend: backend,
		enabled: true,
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

	toolFactory := coding.NewToolFactory(f.backend)
	return toolFactory.CreateAllTools()
}

func (f *CodingToolFactory) GetBackend() coding.Backend {
	return f.backend
}
