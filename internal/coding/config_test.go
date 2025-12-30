package coding

import (
	"testing"
	"time"

	"station/internal/config"
)

func TestTaskTimeoutFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.CodingConfig
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			cfg:      config.CodingConfig{TaskTimeoutMin: 0},
			expected: 10 * time.Minute,
		},
		{
			name:     "negative uses default",
			cfg:      config.CodingConfig{TaskTimeoutMin: -1},
			expected: 10 * time.Minute,
		},
		{
			name:     "positive value",
			cfg:      config.CodingConfig{TaskTimeoutMin: 5},
			expected: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TaskTimeoutFromConfig(tt.cfg)
			if got != tt.expected {
				t.Errorf("TaskTimeoutFromConfig() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMaxAttemptsFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.CodingConfig
		expected int
	}{
		{
			name:     "zero uses default",
			cfg:      config.CodingConfig{MaxAttempts: 0},
			expected: 3,
		},
		{
			name:     "negative uses default",
			cfg:      config.CodingConfig{MaxAttempts: -1},
			expected: 3,
		},
		{
			name:     "positive value",
			cfg:      config.CodingConfig{MaxAttempts: 5},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaxAttemptsFromConfig(tt.cfg)
			if got != tt.expected {
				t.Errorf("MaxAttemptsFromConfig() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestOpenCodeURLFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.CodingConfig
		expected string
	}{
		{
			name:     "empty uses default",
			cfg:      config.CodingConfig{},
			expected: "http://localhost:4096",
		},
		{
			name: "custom url",
			cfg: config.CodingConfig{
				OpenCode: config.CodingOpenCodeConfig{URL: "http://opencode:4096"},
			},
			expected: "http://opencode:4096",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := OpenCodeURLFromConfig(tt.cfg)
			if got != tt.expected {
				t.Errorf("OpenCodeURLFromConfig() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCloneTimeoutFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.CodingConfig
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			cfg:      config.CodingConfig{CloneTimeoutSec: 0},
			expected: 5 * time.Minute,
		},
		{
			name:     "negative uses default",
			cfg:      config.CodingConfig{CloneTimeoutSec: -1},
			expected: 5 * time.Minute,
		},
		{
			name:     "positive value in seconds",
			cfg:      config.CodingConfig{CloneTimeoutSec: 120},
			expected: 120 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CloneTimeoutFromConfig(tt.cfg)
			if got != tt.expected {
				t.Errorf("CloneTimeoutFromConfig() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPushTimeoutFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.CodingConfig
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			cfg:      config.CodingConfig{PushTimeoutSec: 0},
			expected: 2 * time.Minute,
		},
		{
			name:     "negative uses default",
			cfg:      config.CodingConfig{PushTimeoutSec: -1},
			expected: 2 * time.Minute,
		},
		{
			name:     "positive value in seconds",
			cfg:      config.CodingConfig{PushTimeoutSec: 60},
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PushTimeoutFromConfig(tt.cfg)
			if got != tt.expected {
				t.Errorf("PushTimeoutFromConfig() = %v, want %v", got, tt.expected)
			}
		})
	}
}
