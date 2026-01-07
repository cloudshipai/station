package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHostSandbox_Create(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		Mode:          ModeHost,
		WorkspacePath: filepath.Join(tmpDir, "workspace"),
	}

	sb, err := NewHostSandbox(cfg)
	if err != nil {
		t.Fatalf("NewHostSandbox failed: %v", err)
	}

	ctx := context.Background()
	if err := sb.Create(ctx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if _, err := os.Stat(cfg.WorkspacePath); os.IsNotExist(err) {
		t.Error("workspace directory was not created")
	}
}

func TestHostSandbox_Exec(t *testing.T) {
	sb, _ := NewHostSandbox(Config{Mode: ModeHost})

	ctx := context.Background()
	result, err := sb.Exec(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	if result.Stdout != "hello\n" {
		t.Errorf("expected stdout 'hello\\n', got %q", result.Stdout)
	}
}

func TestHostSandbox_ExecTimeout(t *testing.T) {
	sb, _ := NewHostSandbox(Config{
		Mode:    ModeHost,
		Timeout: 100 * time.Millisecond,
	})

	ctx := context.Background()
	result, err := sb.Exec(ctx, "sleep", "10")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if !result.Killed {
		t.Error("expected process to be killed")
	}

	if result.KillReason != "timeout" {
		t.Errorf("expected kill reason 'timeout', got %q", result.KillReason)
	}
}

func TestHostSandbox_FileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	sb, _ := NewHostSandbox(Config{
		Mode:          ModeHost,
		WorkspacePath: tmpDir,
	})

	ctx := context.Background()

	content := []byte("test content")
	if err := sb.WriteFile(ctx, "test.txt", content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	exists, err := sb.FileExists(ctx, "test.txt")
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if !exists {
		t.Error("file should exist after write")
	}

	readContent, err := sb.ReadFile(ctx, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", readContent, content)
	}

	files, err := sb.ListFiles(ctx, ".")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	found := false
	for _, f := range files {
		if f.Name == "test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("test.txt not found in file listing")
	}

	if err := sb.DeleteFile(ctx, "test.txt"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	exists, _ = sb.FileExists(ctx, "test.txt")
	if exists {
		t.Error("file should not exist after delete")
	}
}

func TestHostSandbox_CopyInOut(t *testing.T) {
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")

	sb, _ := NewHostSandbox(Config{
		Mode:          ModeHost,
		WorkspacePath: workspaceDir,
	})

	ctx := context.Background()
	sb.Create(ctx)

	hostFile := filepath.Join(tmpDir, "source.txt")
	content := []byte("copy test")
	os.WriteFile(hostFile, content, 0644)

	if err := sb.CopyIn(ctx, hostFile, "copied.txt"); err != nil {
		t.Fatalf("CopyIn failed: %v", err)
	}

	exists, _ := sb.FileExists(ctx, "copied.txt")
	if !exists {
		t.Error("copied file should exist in sandbox")
	}

	destFile := filepath.Join(tmpDir, "dest.txt")
	if err := sb.CopyOut(ctx, "copied.txt", destFile); err != nil {
		t.Fatalf("CopyOut failed: %v", err)
	}

	destContent, err := os.ReadFile(destFile)
	if err != nil {
		t.Fatalf("failed to read dest file: %v", err)
	}
	if string(destContent) != string(content) {
		t.Errorf("content mismatch after copy out")
	}
}

func TestHostSandbox_Environment(t *testing.T) {
	sb, _ := NewHostSandbox(Config{
		Mode: ModeHost,
		Environment: map[string]string{
			"TEST_VAR": "test_value",
		},
	})

	ctx := context.Background()
	result, err := sb.Exec(ctx, "sh", "-c", "echo $TEST_VAR")
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.Stdout != "test_value\n" {
		t.Errorf("expected 'test_value\\n', got %q", result.Stdout)
	}
}

func TestFactory_CreateHost(t *testing.T) {
	factory := NewFactory(DefaultConfig())

	sb, err := factory.Create(Config{Mode: ModeHost})
	if err != nil {
		t.Fatalf("Factory.Create failed: %v", err)
	}

	if sb.ID() == "" {
		t.Error("sandbox should have an ID")
	}

	cfg := sb.Config()
	if cfg.Mode != ModeHost {
		t.Errorf("expected mode %s, got %s", ModeHost, cfg.Mode)
	}
}

func TestFactory_DefaultMode(t *testing.T) {
	factory := NewFactory(DefaultConfig())

	sb, err := factory.Create(Config{})
	if err != nil {
		t.Fatalf("Factory.Create failed: %v", err)
	}

	cfg := sb.Config()
	if cfg.Mode != ModeHost {
		t.Errorf("expected default mode %s, got %s", ModeHost, cfg.Mode)
	}
}

func TestFactory_MergeConfig(t *testing.T) {
	factory := NewFactory(Config{
		Mode:    ModeHost,
		Timeout: 5 * time.Minute,
		Resources: ResourceConfig{
			CPU:    4,
			Memory: "8Gi",
		},
	})

	sb, _ := factory.Create(Config{
		Resources: ResourceConfig{
			CPU: 2,
		},
	})

	cfg := sb.Config()

	if cfg.Timeout != 5*time.Minute {
		t.Errorf("timeout should inherit default: got %v", cfg.Timeout)
	}

	if cfg.Resources.CPU != 2 {
		t.Errorf("CPU should be overridden: got %v", cfg.Resources.CPU)
	}

	if cfg.Resources.Memory != "8Gi" {
		t.Errorf("Memory should inherit default: got %v", cfg.Resources.Memory)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Mode != ModeHost {
		t.Errorf("expected default mode %s", ModeHost)
	}

	if cfg.Timeout != 30*time.Minute {
		t.Errorf("expected default timeout 30m, got %v", cfg.Timeout)
	}

	if cfg.Resources.CPU != 2 {
		t.Errorf("expected default CPU 2, got %v", cfg.Resources.CPU)
	}

	if cfg.Network.Enabled {
		t.Error("network should be disabled by default")
	}
}
