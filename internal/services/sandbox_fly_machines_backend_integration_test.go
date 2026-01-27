//go:build integration && flymachines

package services

import (
	"context"
	"os"
	"testing"
	"time"
)

func skipIfNoFlyToken(t *testing.T) *FlyMachinesBackend {
	t.Helper()

	if os.Getenv("RUN_FLY_INTEGRATION_TESTS") != "true" {
		t.Skip("RUN_FLY_INTEGRATION_TESTS not set to 'true', skipping Fly Machines integration tests")
	}

	apiToken := os.Getenv("FLY_API_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("FLY_API_KEY")
	}
	if apiToken == "" {
		t.Skip("FLY_API_TOKEN or FLY_API_KEY not set, skipping Fly Machines integration tests")
	}

	orgSlug := os.Getenv("FLY_ORG")
	if orgSlug == "" {
		t.Skip("FLY_ORG not set, skipping Fly Machines integration tests")
	}

	cfg := DefaultFlyMachinesConfig()
	cfg.Enabled = true
	cfg.APIToken = apiToken
	cfg.OrgSlug = orgSlug
	cfg.AppPrefix = "stn-sandbox-test"
	cfg.DefaultImage = "python:3.11-slim"

	backend, err := NewFlyMachinesBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create Fly Machines backend: %v", err)
	}

	if err := backend.Ping(context.Background()); err != nil {
		t.Fatalf("Fly Machines backend ping failed: %v", err)
	}

	t.Cleanup(func() {
		backend.Close()
	})

	return backend
}

func TestFlyMachinesBackend_Integration_CreateDestroySession(t *testing.T) {
	backend := skipIfNoFlyToken(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
		Limits: ResourceLimits{
			CPUMillicores: 1000,
			MemoryMB:      256,
		},
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Logf("Created session %s with machine %s", session.ID, session.ContainerID)

	if session.ID == "" {
		t.Error("expected session ID to be set")
	}
	if session.ContainerID == "" {
		t.Error("expected container/machine ID to be set")
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
	t.Log("Session destroyed successfully")

	_, err = backend.GetSession(ctx, session.ID)
	if err == nil {
		t.Error("expected GetSession to fail after destroy")
	}
}

func TestFlyMachinesBackend_Integration_Exec(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
	t.Logf("Created session %s", session.ID)

	result, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"echo", "hello world"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	expected := "hello world\n"
	if result.Stdout != expected {
		t.Errorf("expected stdout %q, got %q", expected, result.Stdout)
	}
}

func TestFlyMachinesBackend_Integration_ExecPython(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
		Cmd:            []string{"python3", "-c", "print(2 + 2)"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	t.Logf("Result: exit=%d, stdout=%q, stderr=%q", result.ExitCode, result.Stdout, result.Stderr)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", result.ExitCode, result.Stderr)
	}
	if result.Stdout != "4\n" {
		t.Errorf("expected stdout '4\\n', got %q", result.Stdout)
	}
}

func TestFlyMachinesBackend_Integration_FileOperations(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
	t.Log("File written successfully")

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
		if entry.Path == "test.py" || entry.Path == "./test.py" {
			found = true
			if entry.Type != "file" {
				t.Errorf("expected type 'file', got %s", entry.Type)
			}
		}
	}
	if !found {
		t.Errorf("expected to find test.py in listing, got: %+v", entries)
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

	deletedContent, _, readErr := backend.ReadFile(ctx, session.ID, "test.py", 1024)
	if readErr == nil {
		t.Logf("ReadFile after delete returned no error, content=%q", string(deletedContent))
		t.Error("expected ReadFile to fail after delete")
	}
}

func TestFlyMachinesBackend_Integration_ExecTimeout(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
		TimeoutSeconds: 3,
	})
	elapsed := time.Since(start)

	if err == nil && result.ExitCode == 0 {
		t.Error("expected timeout or non-zero exit code")
	}

	if elapsed > 30*time.Second {
		t.Errorf("expected execution to timeout around 3s, took %v", elapsed)
	}
	t.Logf("Timeout test completed in %v", elapsed)
}

func TestFlyMachinesBackend_Integration_ExecAsync(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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

func TestFlyMachinesBackend_Integration_WorkspacePersistence(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
		t.Fatalf("first exec failed: %v (exit %d, stderr: %s)", err, result1.ExitCode, result1.Stderr)
	}

	result2, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"cat", "/workspace/data.txt"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("second exec failed: %v", err)
	}
	if result2.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", result2.ExitCode, result2.Stderr)
	}
	if result2.Stdout != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result2.Stdout)
	}
}

func TestFlyMachinesBackend_Integration_MultipleExecs(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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

	for i := 1; i <= 5; i++ {
		result, err := backend.Exec(ctx, session.ID, ExecRequest{
			Cmd:            []string{"python", "-c", "import os; print(os.getpid())"},
			TimeoutSeconds: 30,
		})
		if err != nil {
			t.Fatalf("Exec %d failed: %v", i, err)
		}
		if result.ExitCode != 0 {
			t.Errorf("Exec %d: expected exit code 0, got %d", i, result.ExitCode)
		}
		t.Logf("Exec %d: PID output: %s", i, result.Stdout)
	}
}

func TestFlyMachinesBackend_Integration_LargeOutput(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
		Cmd:            []string{"python", "-c", "print('x' * 10000)"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	expectedLen := 10001
	if len(result.Stdout) < expectedLen {
		t.Errorf("expected stdout length >= %d, got %d", expectedLen, len(result.Stdout))
	}
}

func TestFlyMachinesBackend_Integration_EnvVars(t *testing.T) {
	backend := skipIfNoFlyToken(t)

	ctx := context.Background()
	opts := SessionOptions{
		Image:   "python:3.11-slim",
		Workdir: "/workspace",
		Env: map[string]string{
			"MY_VAR":     "test_value",
			"ANOTHER":    "hello",
			"PYTHONPATH": "/custom/path",
		},
	}

	session, err := backend.CreateSession(ctx, opts)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	t.Cleanup(func() {
		backend.DestroySession(ctx, session.ID)
	})

	result, err := backend.Exec(ctx, session.ID, ExecRequest{
		Cmd:            []string{"python", "-c", "import os; print(os.environ.get('MY_VAR', 'not_found'))"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout != "test_value\n" {
		t.Errorf("expected 'test_value\\n', got %q", result.Stdout)
	}
}

func TestFlyMachinesBackend_Integration_NonZeroExitCode(t *testing.T) {
	backend := skipIfNoFlyToken(t)

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
		Cmd:            []string{"python", "-c", "import sys; print('error message', file=sys.stderr); sys.exit(42)"},
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}

	if result.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %d", result.ExitCode)
	}
	if result.Stderr == "" {
		t.Error("expected stderr to contain error message")
	}
}
