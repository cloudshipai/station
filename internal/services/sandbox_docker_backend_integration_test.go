//go:build integration

package services

import (
	"context"
	"testing"
	"time"
)

func skipIfNoDocker(t *testing.T) *DockerBackend {
	t.Helper()

	cfg := DefaultCodeModeConfig()
	cfg.Enabled = true

	backend, err := NewDockerBackend(cfg)
	if err != nil {
		t.Skipf("Docker not available: %v", err)
	}

	if err := backend.Ping(context.Background()); err != nil {
		t.Skipf("Docker not responding: %v", err)
	}

	t.Cleanup(func() {
		backend.Close()
	})

	return backend
}

func TestDockerBackend_Integration_CreateDestroySession(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
		Limits: ResourceLimits{
			CPUMillicores: 1000,
			MemoryMB:      512,
		},
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.ID == "" {
		t.Error("expected session ID to be set")
	}
	if session.ContainerID == "" {
		t.Error("expected container ID to be set")
	}

	retrieved, err := backend.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if retrieved.ID != session.ID {
		t.Errorf("expected same session ID, got %s vs %s", session.ID, retrieved.ID)
	}

	if err := backend.DestroySession(ctx, session.ID); err != nil {
		t.Fatalf("DestroySession failed: %v", err)
	}

	_, err = backend.GetSession(ctx, session.ID)
	if err == nil {
		t.Error("expected GetSession to fail after destroy")
	}
}

func TestDockerBackend_Integration_Exec(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	result, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"echo", "hello world"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "hello world\n" {
		t.Errorf("expected stdout 'hello world\\n', got %q", result.Stdout)
	}
}

func TestDockerBackend_Integration_ExecPython(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	result, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"python", "-c", "print(2 + 2)"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "4\n" {
		t.Errorf("expected stdout '4\\n', got %q", result.Stdout)
	}
}

func TestDockerBackend_Integration_FileOperations(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	content := []byte("print('Hello from test')\n")
	if err := backend.WriteFile(ctx, session.ID, "test.py", content, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	readContent, truncated, err := backend.ReadFile(ctx, session.ID, "test.py", 1024)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if truncated {
		t.Error("expected file not to be truncated")
	}
	if string(readContent) != string(content) {
		t.Errorf("expected %q, got %q", string(content), string(readContent))
	}

	entries, err := backend.ListFiles(ctx, session.ID, ".", false)
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	found := false
	for _, entry := range entries {
		if entry.Path == "test.py" {
			found = true
			if entry.Type != "file" {
				t.Errorf("expected type 'file', got %s", entry.Type)
			}
		}
	}
	if !found {
		t.Error("expected to find test.py in listing")
	}

	result, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"python", "test.py"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "Hello from test\n" {
		t.Errorf("expected 'Hello from test\\n', got %q", result.Stdout)
	}

	if err := backend.DeleteFile(ctx, session.ID, "test.py", false); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	_, _, err = backend.ReadFile(ctx, session.ID, "test.py", 1024)
	if err == nil {
		t.Error("expected ReadFile to fail after delete")
	}
}

func TestDockerBackend_Integration_ExecTimeout(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	start := time.Now()
	result, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"sleep", "30"},
		TimeoutSeconds: 2,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code due to timeout")
	}

	if elapsed > 10*time.Second {
		t.Errorf("expected execution to timeout around 2s, took %v", elapsed)
	}
}

func TestDockerBackend_Integration_ExecAsync(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	handle, err := backend.ExecAsync(ctx, session.ID, ExecRequest{
		Cmd:            []string{"python", "-c", "import time; print('start'); time.sleep(1); print('end')"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("ExecAsync failed: %v", err)
	}

	if handle.ID == "" {
		t.Error("expected exec ID to be set")
	}

	result, err := backend.ExecWait(ctx, session.ID, handle.ID, 30*time.Second)
	if err != nil {
		t.Fatalf("ExecWait failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "start\nend\n" {
		t.Errorf("expected 'start\\nend\\n', got %q", result.Stdout)
	}
}

func TestDockerBackend_Integration_WorkspacePersistence(t *testing.T) {
	backend := skipIfNoDocker(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	result1, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"sh", "-c", "echo 'hello' > /workspace/data.txt"},
		TimeoutSeconds: 30,
	})
	if err != nil || result1.ExitCode != 0 {
		t.Fatalf("first exec failed: %v (exit %d)", err, result1.ExitCode)
	}

	result2, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"cat", "/workspace/data.txt"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("second exec failed: %v", err)
	}
	if result2.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result2.ExitCode)
	}
	if result2.Stdout != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result2.Stdout)
	}
}
