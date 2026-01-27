package prompt

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"station/pkg/harness/memory"
	"station/pkg/harness/skills"
)

// AgentConfig represents a parsed agent dotprompt with extended harness configuration
type AgentConfig struct {
	// Standard Genkit/Station dotprompt fields
	Model    string         `yaml:"model"`
	Metadata AgentMetadata  `yaml:"metadata"`
	Tools    []string       `yaml:"tools,omitempty"`
	Sandbox  *SandboxConfig `yaml:"sandbox,omitempty"`

	// Extended harness configuration
	Harness   *HarnessConfig   `yaml:"harness,omitempty"`
	Workspace *WorkspaceConfig `yaml:"workspace,omitempty"`
	Skills    *skills.SkillsConfig `yaml:"skills,omitempty"`
	Memory    *memory.MemoryConfig `yaml:"memory,omitempty"`

	// The prompt content (system + user messages)
	PromptContent string `yaml:"-"`
}

// AgentMetadata contains agent metadata
type AgentMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	MaxSteps    int    `yaml:"max_steps,omitempty"`
}

// HarnessConfig configures the agentic harness behavior
type HarnessConfig struct {
	Enabled    bool              `yaml:"enabled,omitempty"`
	MaxSteps   int               `yaml:"max_steps,omitempty"`
	Timeout    string            `yaml:"timeout,omitempty"` // e.g., "30m", "1h"
	DoomLoop   *DoomLoopConfig   `yaml:"doom_loop,omitempty"`
	Compaction *CompactionConfig `yaml:"compaction,omitempty"`
	Progress   *ProgressConfig   `yaml:"progress,omitempty"`
	Heartbeat  *HeartbeatConfig  `yaml:"heartbeat,omitempty"`
}

// HeartbeatConfig configures periodic agent heartbeats
type HeartbeatConfig struct {
	Enabled     bool               `yaml:"enabled,omitempty"`
	Every       string             `yaml:"every,omitempty"`        // e.g., "30m", "1h"
	ActiveHours *ActiveHoursConfig `yaml:"active_hours,omitempty"`
	Session     string             `yaml:"session,omitempty"` // "main" (default) or "isolated"
	Notify      *NotifyConfig      `yaml:"notify,omitempty"`
}

// ActiveHoursConfig defines when heartbeats should run
type ActiveHoursConfig struct {
	Start    string `yaml:"start,omitempty"`    // "08:00" (24h format)
	End      string `yaml:"end,omitempty"`      // "20:00" (24h format)
	Timezone string `yaml:"timezone,omitempty"` // "local", "UTC", or IANA timezone
}

// NotifyConfig configures heartbeat notifications
type NotifyConfig struct {
	Channel string `yaml:"channel,omitempty"` // "webhook"
	URL     string `yaml:"url,omitempty"`     // Webhook URL
}

// DoomLoopConfig configures doom loop detection
type DoomLoopConfig struct {
	Threshold int    `yaml:"threshold,omitempty"` // Default: 3
	Action    string `yaml:"action,omitempty"`    // "summarize", "pause", "fail"
}

// CompactionConfig configures context compaction
type CompactionConfig struct {
	Enabled        bool    `yaml:"enabled,omitempty"`
	Threshold      float64 `yaml:"threshold,omitempty"`       // Default: 0.85
	ProtectTokens  int     `yaml:"protect_tokens,omitempty"`  // Default: 40000
	HistoryOffload bool    `yaml:"history_offload,omitempty"` // Save to NATS before summarizing
	MemoryFlush    bool    `yaml:"memory_flush,omitempty"`    // Auto-flush memory before compaction
}

// ProgressConfig configures progress tracking
type ProgressConfig struct {
	File         string `yaml:"file,omitempty"`          // e.g., "progress.md"
	SyncInterval string `yaml:"sync_interval,omitempty"` // e.g., "5s"
}

// WorkspaceConfig configures the working directory
type WorkspaceConfig struct {
	Mode    string `yaml:"mode,omitempty"`    // "host" or "sandbox"
	Scope   string `yaml:"scope,omitempty"`   // "session" or "workflow"
	Cleanup string `yaml:"cleanup,omitempty"` // "always", "on_success", "never"
}

// SandboxConfig configures sandbox isolation
type SandboxConfig struct {
	Mode      string            `yaml:"mode,omitempty"` // "host", "docker", "e2b", "modal", "runloop", "daytona"
	Image     string            `yaml:"image,omitempty"`
	Timeout   string            `yaml:"timeout,omitempty"`
	Resources *ResourceConfig   `yaml:"resources,omitempty"`
	Network   *NetworkConfig    `yaml:"network,omitempty"`
}

// ResourceConfig defines resource limits
type ResourceConfig struct {
	CPU    string `yaml:"cpu,omitempty"`    // e.g., "2"
	Memory string `yaml:"memory,omitempty"` // e.g., "4g"
}

// NetworkConfig defines network access
type NetworkConfig struct {
	Enabled      bool     `yaml:"enabled,omitempty"`
	AllowedHosts []string `yaml:"allowed_hosts,omitempty"`
}

// LoadAgentConfig loads and parses an agent dotprompt file with extended harness fields
func LoadAgentConfig(path string) (*AgentConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent file: %w", err)
	}

	return ParseAgentConfig(string(content))
}

// ParseAgentConfig parses agent config from dotprompt content string
func ParseAgentConfig(content string) (*AgentConfig, error) {
	// Split frontmatter and prompt content
	frontmatter, promptContent, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Parse frontmatter
	var config AgentConfig
	if err := yaml.Unmarshal([]byte(frontmatter), &config); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	config.PromptContent = promptContent
	return &config, nil
}

// splitFrontmatter separates YAML frontmatter from prompt content
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	if !strings.HasPrefix(content, "---") {
		return "", content, nil // No frontmatter
	}

	// Find the closing ---
	rest := content[3:]
	parts := strings.SplitN(rest, "---", 2)
	if len(parts) < 2 {
		return "", "", fmt.Errorf("frontmatter not closed")
	}

	frontmatter = strings.TrimSpace(parts[0])
	body = strings.TrimSpace(parts[1])
	return frontmatter, body, nil
}

// IsAgenticHarness returns true if this agent uses the agentic harness
func (c *AgentConfig) IsAgenticHarness() bool {
	return c.Harness != nil && c.Harness.Enabled
}

// GetMaxSteps returns the effective max steps (harness > metadata > default)
func (c *AgentConfig) GetMaxSteps() int {
	if c.Harness != nil && c.Harness.MaxSteps > 0 {
		return c.Harness.MaxSteps
	}
	if c.Metadata.MaxSteps > 0 {
		return c.Metadata.MaxSteps
	}
	return 50 // Default
}

// GetTimeout returns the timeout as a Duration
func (c *AgentConfig) GetTimeout() time.Duration {
	if c.Harness == nil || c.Harness.Timeout == "" {
		return 30 * time.Minute // Default
	}

	d, err := time.ParseDuration(c.Harness.Timeout)
	if err != nil {
		return 30 * time.Minute
	}
	return d
}

// GetDoomLoopThreshold returns the doom loop threshold
func (c *AgentConfig) GetDoomLoopThreshold() int {
	if c.Harness != nil && c.Harness.DoomLoop != nil && c.Harness.DoomLoop.Threshold > 0 {
		return c.Harness.DoomLoop.Threshold
	}
	return 3 // Default
}

// GetCompactionThreshold returns the compaction threshold
func (c *AgentConfig) GetCompactionThreshold() float64 {
	if c.Harness != nil && c.Harness.Compaction != nil && c.Harness.Compaction.Threshold > 0 {
		return c.Harness.Compaction.Threshold
	}
	return 0.85 // Default
}

// GetProtectTokens returns the number of tokens to protect from compaction
func (c *AgentConfig) GetProtectTokens() int {
	if c.Harness != nil && c.Harness.Compaction != nil && c.Harness.Compaction.ProtectTokens > 0 {
		return c.Harness.Compaction.ProtectTokens
	}
	return 40000 // Default
}

// IsCompactionEnabled returns whether compaction is enabled
func (c *AgentConfig) IsCompactionEnabled() bool {
	if c.Harness == nil || c.Harness.Compaction == nil {
		return true // Enabled by default for harness agents
	}
	return c.Harness.Compaction.Enabled
}

// IsHistoryOffloadEnabled returns whether history offload is enabled
func (c *AgentConfig) IsHistoryOffloadEnabled() bool {
	if c.Harness == nil || c.Harness.Compaction == nil {
		return false // Disabled by default
	}
	return c.Harness.Compaction.HistoryOffload
}

// GetSkillSources returns skill sources, using defaults if not specified
func (c *AgentConfig) GetSkillSources(envPath string) []string {
	if c.Skills != nil && len(c.Skills.Sources) > 0 {
		return c.Skills.Sources
	}
	return skills.DefaultSkillSources(envPath)
}

// GetMemorySources returns memory sources, using defaults if not specified
func (c *AgentConfig) GetMemorySources(envPath string) []string {
	if c.Memory != nil && len(c.Memory.Sources) > 0 {
		return c.Memory.Sources
	}
	return memory.DefaultMemorySources(envPath)
}

// GetSandboxMode returns the sandbox mode (defaults to "host")
func (c *AgentConfig) GetSandboxMode() string {
	if c.Sandbox != nil && c.Sandbox.Mode != "" {
		return c.Sandbox.Mode
	}
	return "host"
}

// IsHeartbeatEnabled returns whether heartbeat is enabled
func (c *AgentConfig) IsHeartbeatEnabled() bool {
	return c.Harness != nil && c.Harness.Heartbeat != nil && c.Harness.Heartbeat.Enabled
}

// GetHeartbeatInterval returns the heartbeat interval as a Duration
func (c *AgentConfig) GetHeartbeatInterval() time.Duration {
	if c.Harness == nil || c.Harness.Heartbeat == nil || c.Harness.Heartbeat.Every == "" {
		return 30 * time.Minute // Default
	}

	d, err := time.ParseDuration(c.Harness.Heartbeat.Every)
	if err != nil {
		return 30 * time.Minute
	}
	return d
}

// GetHeartbeatSession returns the heartbeat session mode ("main" or "isolated")
func (c *AgentConfig) GetHeartbeatSession() string {
	if c.Harness == nil || c.Harness.Heartbeat == nil || c.Harness.Heartbeat.Session == "" {
		return "main" // Default
	}
	return c.Harness.Heartbeat.Session
}

// IsMemoryFlushEnabled returns whether memory flush before compaction is enabled
func (c *AgentConfig) IsMemoryFlushEnabled() bool {
	if c.Harness == nil || c.Harness.Compaction == nil {
		return false // Disabled by default
	}
	return c.Harness.Compaction.MemoryFlush
}
