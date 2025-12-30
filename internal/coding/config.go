package coding

import (
	"time"

	"station/internal/config"
)

func TaskTimeoutFromConfig(cfg config.CodingConfig) time.Duration {
	if cfg.TaskTimeoutMin <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(cfg.TaskTimeoutMin) * time.Minute
}

func MaxAttemptsFromConfig(cfg config.CodingConfig) int {
	if cfg.MaxAttempts <= 0 {
		return 3
	}
	return cfg.MaxAttempts
}

func OpenCodeURLFromConfig(cfg config.CodingConfig) string {
	if cfg.OpenCode.URL == "" {
		return "http://localhost:4096"
	}
	return cfg.OpenCode.URL
}
