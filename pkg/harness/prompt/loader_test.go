package prompt

import (
	"testing"
	"time"

	"station/pkg/harness/memory"
	"station/pkg/harness/skills"
)

func TestParseAgentConfig_Basic(t *testing.T) {
	content := `---
model: gpt-4o-mini
metadata:
  name: test-agent
  description: A test agent
  max_steps: 25
tools:
  - bash
  - read_file
---

You are a test agent.`

	config, err := ParseAgentConfig(content)
	if err != nil {
		t.Fatalf("ParseAgentConfig() error = %v", err)
	}

	if config.Model != "gpt-4o-mini" {
		t.Errorf("Model = %q, want gpt-4o-mini", config.Model)
	}
	if config.Metadata.Name != "test-agent" {
		t.Errorf("Name = %q, want test-agent", config.Metadata.Name)
	}
	if len(config.Tools) != 2 {
		t.Errorf("Tools count = %d, want 2", len(config.Tools))
	}
	if config.PromptContent != "You are a test agent." {
		t.Errorf("PromptContent = %q", config.PromptContent)
	}
}

func TestParseAgentConfig_WithHarness(t *testing.T) {
	content := `---
model: openai/gpt-4o
metadata:
  name: harness-agent
  description: An agentic harness agent
harness:
  enabled: true
  max_steps: 100
  timeout: 60m
  doom_loop:
    threshold: 5
    action: summarize
  compaction:
    enabled: true
    threshold: 0.85
    protect_tokens: 50000
    history_offload: true
---

{{role "system"}}
You are a powerful agent.`

	config, err := ParseAgentConfig(content)
	if err != nil {
		t.Fatalf("ParseAgentConfig() error = %v", err)
	}

	if !config.IsAgenticHarness() {
		t.Error("should be agentic harness")
	}

	if config.GetMaxSteps() != 100 {
		t.Errorf("MaxSteps = %d, want 100", config.GetMaxSteps())
	}

	if config.GetTimeout() != 60*time.Minute {
		t.Errorf("Timeout = %v, want 60m", config.GetTimeout())
	}

	if config.GetDoomLoopThreshold() != 5 {
		t.Errorf("DoomLoopThreshold = %d, want 5", config.GetDoomLoopThreshold())
	}

	if !config.IsCompactionEnabled() {
		t.Error("compaction should be enabled")
	}

	if config.GetCompactionThreshold() != 0.85 {
		t.Errorf("CompactionThreshold = %f, want 0.85", config.GetCompactionThreshold())
	}

	if config.GetProtectTokens() != 50000 {
		t.Errorf("ProtectTokens = %d, want 50000", config.GetProtectTokens())
	}

	if !config.IsHistoryOffloadEnabled() {
		t.Error("history offload should be enabled")
	}
}

func TestParseAgentConfig_WithSkillsAndMemory(t *testing.T) {
	content := `---
model: gpt-4o-mini
metadata:
  name: skilled-agent
harness:
  enabled: true
skills:
  sources:
    - /custom/skills
    - .station/skills
memory:
  sources:
    - ~/.config/station/AGENTS.md
    - .station/AGENTS.md
---

Agent prompt here.`

	config, err := ParseAgentConfig(content)
	if err != nil {
		t.Fatalf("ParseAgentConfig() error = %v", err)
	}

	if len(config.Skills.Sources) != 2 {
		t.Errorf("Skills sources = %d, want 2", len(config.Skills.Sources))
	}

	if len(config.Memory.Sources) != 2 {
		t.Errorf("Memory sources = %d, want 2", len(config.Memory.Sources))
	}

	skillSources := config.GetSkillSources("/env")
	if skillSources[0] != "/custom/skills" {
		t.Errorf("First skill source = %q, want /custom/skills", skillSources[0])
	}
}

func TestParseAgentConfig_WithSandbox(t *testing.T) {
	content := `---
model: gpt-4o-mini
metadata:
  name: sandboxed-agent
sandbox:
  mode: docker
  image: python:3.12-slim
  timeout: 30m
  resources:
    cpu: "2"
    memory: 4g
  network:
    enabled: true
    allowed_hosts:
      - "*.github.com"
      - "pypi.org"
---

Agent prompt.`

	config, err := ParseAgentConfig(content)
	if err != nil {
		t.Fatalf("ParseAgentConfig() error = %v", err)
	}

	if config.GetSandboxMode() != "docker" {
		t.Errorf("SandboxMode = %q, want docker", config.GetSandboxMode())
	}

	if config.Sandbox.Image != "python:3.12-slim" {
		t.Errorf("Image = %q, want python:3.12-slim", config.Sandbox.Image)
	}

	if config.Sandbox.Resources.CPU != "2" {
		t.Errorf("CPU = %q, want 2", config.Sandbox.Resources.CPU)
	}

	if !config.Sandbox.Network.Enabled {
		t.Error("network should be enabled")
	}

	if len(config.Sandbox.Network.AllowedHosts) != 2 {
		t.Errorf("AllowedHosts count = %d, want 2", len(config.Sandbox.Network.AllowedHosts))
	}
}

func TestParseAgentConfig_NoFrontmatter(t *testing.T) {
	content := "Just a prompt without frontmatter."

	config, err := ParseAgentConfig(content)
	if err != nil {
		t.Fatalf("ParseAgentConfig() error = %v", err)
	}

	if config.PromptContent != content {
		t.Errorf("PromptContent = %q, want original content", config.PromptContent)
	}
}

func TestParseAgentConfig_UnclosedFrontmatter(t *testing.T) {
	content := "---\nmodel: test"

	_, err := ParseAgentConfig(content)
	if err == nil {
		t.Error("should error on unclosed frontmatter")
	}
}

func TestAgentConfig_Defaults(t *testing.T) {
	config := &AgentConfig{}

	if config.IsAgenticHarness() {
		t.Error("should not be harness by default")
	}

	if config.GetMaxSteps() != 50 {
		t.Errorf("default MaxSteps = %d, want 50", config.GetMaxSteps())
	}

	if config.GetTimeout() != 30*time.Minute {
		t.Errorf("default Timeout = %v, want 30m", config.GetTimeout())
	}

	if config.GetDoomLoopThreshold() != 3 {
		t.Errorf("default DoomLoopThreshold = %d, want 3", config.GetDoomLoopThreshold())
	}

	if config.GetCompactionThreshold() != 0.85 {
		t.Errorf("default CompactionThreshold = %f, want 0.85", config.GetCompactionThreshold())
	}

	if config.GetProtectTokens() != 40000 {
		t.Errorf("default ProtectTokens = %d, want 40000", config.GetProtectTokens())
	}

	if config.GetSandboxMode() != "host" {
		t.Errorf("default SandboxMode = %q, want host", config.GetSandboxMode())
	}
}

func TestAgentConfig_GetSkillSources_CustomVsDefault(t *testing.T) {
	// With custom sources
	config := &AgentConfig{
		Skills: &skills.SkillsConfig{
			Sources: []string{"/custom/path"},
		},
	}

	sources := config.GetSkillSources("/env/default")
	if len(sources) != 1 || sources[0] != "/custom/path" {
		t.Error("should return custom sources when specified")
	}

	// Without custom sources - should get defaults
	config2 := &AgentConfig{}
	sources2 := config2.GetSkillSources("/env/default")
	if len(sources2) < 2 {
		t.Error("should return default sources when not specified")
	}
}

func TestAgentConfig_GetMemorySources_CustomVsDefault(t *testing.T) {
	// With custom sources
	config := &AgentConfig{
		Memory: &memory.MemoryConfig{
			Sources: []string{"/custom/AGENTS.md"},
		},
	}

	sources := config.GetMemorySources("/env/default")
	if len(sources) != 1 || sources[0] != "/custom/AGENTS.md" {
		t.Error("should return custom sources when specified")
	}

	// Without custom sources - should get defaults
	config2 := &AgentConfig{}
	sources2 := config2.GetMemorySources("/env/default")
	if len(sources2) < 2 {
		t.Error("should return default sources when not specified")
	}
}
