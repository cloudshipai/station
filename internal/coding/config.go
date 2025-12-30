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

func CloneTimeoutFromConfig(cfg config.CodingConfig) time.Duration {
	if cfg.CloneTimeoutSec <= 0 {
		return 5 * time.Minute // default 5 minutes for clone operations
	}
	return time.Duration(cfg.CloneTimeoutSec) * time.Second
}

func PushTimeoutFromConfig(cfg config.CodingConfig) time.Duration {
	if cfg.PushTimeoutSec <= 0 {
		return 2 * time.Minute // default 2 minutes for push operations
	}
	return time.Duration(cfg.PushTimeoutSec) * time.Second
}
