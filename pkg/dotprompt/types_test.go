package dotprompt

import (
	"testing"

	"gopkg.in/yaml.v2"
)

func TestSandboxConfigUnmarshalYAML_String(t *testing.T) {
	yamlData := `sandbox: python`

	type testConfig struct {
		Sandbox *SandboxConfig `yaml:"sandbox,omitempty"`
	}

	var cfg testConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.Sandbox == nil {
		t.Fatal("expected sandbox to be non-nil")
	}
	if cfg.Sandbox.Runtime != "python" {
		t.Errorf("expected runtime 'python', got %q", cfg.Sandbox.Runtime)
	}
}

func TestSandboxConfigUnmarshalYAML_Object(t *testing.T) {
	yamlData := `
sandbox:
  runtime: node
  timeout_seconds: 300
  pip_packages:
    - pandas
    - numpy
`

	type testConfig struct {
		Sandbox *SandboxConfig `yaml:"sandbox,omitempty"`
	}

	var cfg testConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.Sandbox == nil {
		t.Fatal("expected sandbox to be non-nil")
	}
	if cfg.Sandbox.Runtime != "node" {
		t.Errorf("expected runtime 'node', got %q", cfg.Sandbox.Runtime)
	}
	if cfg.Sandbox.TimeoutSeconds != 300 {
		t.Errorf("expected timeout 300, got %d", cfg.Sandbox.TimeoutSeconds)
	}
	if len(cfg.Sandbox.PipPackages) != 2 {
		t.Errorf("expected 2 pip packages, got %d", len(cfg.Sandbox.PipPackages))
	}
}

func TestSandboxConfigUnmarshalYAML_Nil(t *testing.T) {
	yamlData := `model: openai/gpt-4o`

	type testConfig struct {
		Model   string         `yaml:"model"`
		Sandbox *SandboxConfig `yaml:"sandbox,omitempty"`
	}

	var cfg testConfig
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.Sandbox != nil {
		t.Error("expected sandbox to be nil when not specified")
	}
}
