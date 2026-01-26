package harness

import (
	"context"
	"testing"

	"station/pkg/harness/sandbox"
)

func TestNewAgenticExecutor_WithSandboxOptions(t *testing.T) {
	cfg := &sandbox.Config{
		Mode:    sandbox.ModeHost,
		Timeout: sandbox.DefaultConfig().Timeout,
	}

	executor := NewAgenticExecutor(
		nil,
		nil,
		nil,
		WithSandboxConfig(cfg),
	)

	if executor.sandboxConfig == nil {
		t.Error("sandboxConfig should be set")
	}
	if executor.sandboxConfig.Mode != sandbox.ModeHost {
		t.Errorf("sandboxConfig.Mode = %s, want %s", executor.sandboxConfig.Mode, sandbox.ModeHost)
	}
}

func TestAgenticExecutor_InitializeSandbox_NoConfig(t *testing.T) {
	executor := NewAgenticExecutor(nil, nil, nil)

	err := executor.initializeSandbox(context.Background())
	if err != nil {
		t.Errorf("initializeSandbox should succeed with no config: %v", err)
	}
	if executor.sandbox != nil {
		t.Error("sandbox should be nil when no config provided")
	}
}

func TestAgenticExecutor_InitializeSandbox_WithExistingSandbox(t *testing.T) {
	hostSandbox, err := sandbox.NewHostSandbox(sandbox.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create host sandbox: %v", err)
	}

	executor := NewAgenticExecutor(
		nil,
		nil,
		nil,
		WithSandbox(hostSandbox),
	)

	err = executor.initializeSandbox(context.Background())
	if err != nil {
		t.Errorf("initializeSandbox should succeed with existing sandbox: %v", err)
	}
	if executor.sandbox != hostSandbox {
		t.Error("sandbox should remain unchanged when already provided")
	}
}

func TestAgenticExecutor_InitializeSandbox_CreatesHostSandbox(t *testing.T) {
	cfg := &sandbox.Config{
		Mode:    sandbox.ModeHost,
		Timeout: sandbox.DefaultConfig().Timeout,
	}

	executor := NewAgenticExecutor(
		nil,
		nil,
		nil,
		WithSandboxConfig(cfg),
	)

	err := executor.initializeSandbox(context.Background())
	if err != nil {
		t.Errorf("initializeSandbox failed: %v", err)
	}
	if executor.sandbox == nil {
		t.Error("sandbox should be created")
	}
	if executor.sandbox.ID() == "" {
		t.Error("sandbox ID should not be empty")
	}
}

func TestAgenticExecutor_InitializeSandbox_CreatesDockerSandbox(t *testing.T) {
	cfg := &sandbox.Config{
		Mode:    sandbox.ModeDocker,
		Image:   "ubuntu:22.04",
		Timeout: sandbox.DefaultConfig().Timeout,
	}

	executor := NewAgenticExecutor(
		nil,
		nil,
		nil,
		WithSandboxConfig(cfg),
	)

	err := executor.initializeSandbox(context.Background())
	if err != nil {
		t.Logf("Docker sandbox creation failed (expected if Docker not available): %v", err)
		t.Skip("Skipping Docker test - Docker not available")
	}
	if executor.sandbox == nil {
		t.Error("sandbox should be created")
	}
}

func TestAgenticExecutor_DestroySandbox(t *testing.T) {
	hostSandbox, err := sandbox.NewHostSandbox(sandbox.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create host sandbox: %v", err)
	}

	executor := NewAgenticExecutor(
		nil,
		nil,
		nil,
		WithSandbox(hostSandbox),
	)

	err = executor.destroySandbox(context.Background())
	if err != nil {
		t.Errorf("destroySandbox failed: %v", err)
	}
	if executor.sandbox != nil {
		t.Error("sandbox should be nil after destroy")
	}
}

func TestAgenticExecutor_SandboxGetter(t *testing.T) {
	hostSandbox, err := sandbox.NewHostSandbox(sandbox.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create host sandbox: %v", err)
	}

	executor := NewAgenticExecutor(
		nil,
		nil,
		nil,
		WithSandbox(hostSandbox),
	)

	if executor.Sandbox() != hostSandbox {
		t.Error("Sandbox() should return the configured sandbox")
	}

	executorNoSandbox := NewAgenticExecutor(nil, nil, nil)
	if executorNoSandbox.Sandbox() != nil {
		t.Error("Sandbox() should return nil when no sandbox configured")
	}
}
