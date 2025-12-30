package coding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"station/internal/config"
)

func TestE2E_OpenCodeIntegration(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Skipping E2E test. Set OPENCODE_E2E=true to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initGitRepoForE2E(t, workspaceDir)

	cfg := config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: "http://localhost:4096",
		},
		MaxAttempts:    3,
		TaskTimeoutMin: 5,
	}

	backend := NewOpenCodeBackend(cfg)

	t.Run("health_check", func(t *testing.T) {
		if err := backend.Ping(ctx); err != nil {
			t.Fatalf("OpenCode health check failed: %v", err)
		}
		t.Log("OpenCode is healthy")
	})

	var session *Session
	t.Run("create_session", func(t *testing.T) {
		var err error
		session, err = backend.CreateSession(ctx, workspaceDir, "E2E Test Session")
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		t.Logf("Created session: %s (backend: %s)", session.ID, session.BackendSessionID)
	})

	t.Run("execute_coding_task", func(t *testing.T) {
		if session == nil {
			t.Skip("Session not created")
		}

		task := Task{
			Instruction: "Create a Python file called hello.py with a simple print statement that says 'Hello from E2E test'",
			Timeout:     2 * time.Minute,
		}

		result, err := backend.Execute(ctx, session.ID, task)
		if err != nil {
			t.Fatalf("Failed to execute task: %v", err)
		}

		if !result.Success {
			t.Fatalf("Task failed: %s", result.Error)
		}

		t.Logf("Task completed successfully")
		t.Logf("Summary: %s", result.Summary)
		t.Logf("Full result: Success=%v, Error=%s", result.Success, result.Error)

		if result.Trace != nil {
			t.Logf("Model: %s, Provider: %s", result.Trace.Model, result.Trace.Provider)
			t.Logf("Tokens: input=%d, output=%d", result.Trace.Tokens.Input, result.Trace.Tokens.Output)
			t.Logf("Duration: %v", result.Trace.Duration)
			t.Logf("Tool calls: %d", len(result.Trace.ToolCalls))
			for i, tc := range result.Trace.ToolCalls {
				t.Logf("  [%d] %s", i+1, tc.Tool)
			}
			if len(result.Trace.Reasoning) > 0 {
				t.Logf("Reasoning steps: %d", len(result.Trace.Reasoning))
			}
		}
	})

	t.Run("verify_file_created", func(t *testing.T) {
		helloPath := filepath.Join(workspaceDir, "hello.py")
		content, err := os.ReadFile(helloPath)
		if err != nil {
			t.Fatalf("hello.py not found: %v", err)
		}

		t.Logf("File content:\n%s", string(content))

		if len(content) == 0 {
			t.Error("hello.py is empty")
		}
	})

	t.Run("close_session", func(t *testing.T) {
		if session == nil {
			t.Skip("Session not created")
		}

		if err := backend.CloseSession(ctx, session.ID); err != nil {
			t.Fatalf("Failed to close session: %v", err)
		}
		t.Log("Session closed successfully")
	})
}

func TestE2E_GitCommitFlow(t *testing.T) {
	if os.Getenv("OPENCODE_E2E") != "true" {
		t.Skip("Skipping E2E test. Set OPENCODE_E2E=true to run.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initGitRepoForE2E(t, workspaceDir)

	cfg := config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: "http://localhost:4096",
		},
		MaxAttempts:    3,
		TaskTimeoutMin: 5,
	}

	backend := NewOpenCodeBackend(cfg)

	session, err := backend.CreateSession(ctx, workspaceDir, "Git Flow Test")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	defer backend.CloseSession(ctx, session.ID)

	t.Run("create_file_via_opencode", func(t *testing.T) {
		task := Task{
			Instruction: "Create a file called main.go with a simple hello world program",
			Timeout:     2 * time.Minute,
		}

		result, err := backend.Execute(ctx, session.ID, task)
		if err != nil {
			t.Fatalf("Failed to execute task: %v", err)
		}
		if !result.Success {
			t.Fatalf("Task failed: %s", result.Error)
		}
		t.Logf("File created: %s", result.Summary)
	})

	t.Run("git_commit", func(t *testing.T) {
		factory := NewToolFactory(backend)
		commitTool := factory.CreateCommitTool()

		toolCtx := &ai.ToolContext{Context: ctx}
		input := map[string]any{
			"session_id": session.ID,
			"message":    "Add hello world program",
			"add_all":    true,
		}

		result, err := commitTool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		commitResult, ok := result.(GitCommitResult)
		if !ok {
			resultMap, isMap := result.(map[string]any)
			if !isMap {
				t.Fatalf("Expected GitCommitResult or map, got %T", result)
			}
			t.Logf("Commit result: %+v", resultMap)
			return
		}

		t.Logf("Commit: %s", commitResult.CommitHash)
		t.Logf("Files changed: %d, insertions: %d, deletions: %d",
			commitResult.FilesChanged, commitResult.Insertions, commitResult.Deletions)
	})
}

func initGitRepoForE2E(t *testing.T, dir string) {
	t.Helper()

	initCmd := exec.Command("git", "init")
	initCmd.Dir = dir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	configCmd1 := exec.Command("git", "config", "user.email", "test@example.com")
	configCmd1.Dir = dir
	configCmd1.Run()

	configCmd2 := exec.Command("git", "config", "user.name", "Test User")
	configCmd2.Dir = dir
	configCmd2.Run()

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# E2E Test\n"), 0644); err != nil {
		t.Fatalf("Failed to create README: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	if err := addCmd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = dir
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
}
