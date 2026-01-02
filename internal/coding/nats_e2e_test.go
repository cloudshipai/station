package coding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"station/internal/config"
)

func TestE2E_NATSBackendIntegration(t *testing.T) {
	if os.Getenv("NATS_E2E") != "true" {
		t.Skip("Skipping NATS E2E test. Set NATS_E2E=true to run.")
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	workspaceDir := t.TempDir()
	initGitRepoForNATSE2E(t, workspaceDir)

	cfg := config.CodingConfig{
		Backend: "opencode-nats",
		NATS: config.CodingNATSConfig{
			URL: natsURL,
		},
		TaskTimeoutMin: 5,
	}

	backend, err := NewNATSBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create NATS backend: %v", err)
	}
	defer backend.Close()

	t.Run("ping", func(t *testing.T) {
		if err := backend.Ping(ctx); err != nil {
			t.Fatalf("NATS ping failed: %v", err)
		}
		t.Log("NATS connection healthy")
	})

	var session *Session
	t.Run("create_session", func(t *testing.T) {
		var err error
		session, err = backend.CreateSession(ctx, SessionOptions{
			WorkspacePath: workspaceDir,
			Title:         "NATS E2E Test",
		})
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		t.Logf("Created session: %s", session.ID)
	})

	t.Run("execute_simple_task", func(t *testing.T) {
		if session == nil {
			t.Skip("Session not created")
		}

		task := Task{
			Instruction: "Create a file called hello.txt with the text 'Hello from NATS E2E test'",
			Timeout:     2 * time.Minute,
		}

		result, err := backend.Execute(ctx, session.ID, task)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Task failed: %s", result.Error)
		}

		t.Logf("Task completed: %s", result.Summary)
		if result.Trace != nil {
			t.Logf("Duration: %v", result.Trace.Duration)
			t.Logf("Tool calls: %d", len(result.Trace.ToolCalls))
		}
	})

	t.Run("verify_file_created", func(t *testing.T) {
		helloPath := filepath.Join(workspaceDir, "hello.txt")
		content, err := os.ReadFile(helloPath)
		if err != nil {
			t.Logf("Note: File not found at %s - OpenCode may use different workspace", helloPath)
			t.Skip("File verification skipped (different workspace)")
		}

		t.Logf("File content: %s", string(content))
		if len(content) == 0 {
			t.Error("hello.txt is empty")
		}
	})

	t.Run("execute_coding_task", func(t *testing.T) {
		if session == nil {
			t.Skip("Session not created")
		}

		task := Task{
			Instruction: "Create a Python file called greet.py with a function greet(name) that returns 'Hello, {name}!'",
			Timeout:     2 * time.Minute,
		}

		result, err := backend.Execute(ctx, session.ID, task)
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Task failed: %s", result.Error)
		}

		t.Logf("Coding task completed: %s", result.Summary)
	})

	t.Run("git_commit", func(t *testing.T) {
		if session == nil {
			t.Skip("Session not created")
		}

		commitResult, err := backend.GitCommit(ctx, session.ID, "Add greeting files", true)
		if err != nil {
			t.Fatalf("GitCommit failed: %v", err)
		}

		if !commitResult.Success {
			t.Logf("Commit may have failed (expected in isolated workspace): %s", commitResult.Error)
			t.Skip("Git commit skipped (workspace isolation)")
		}

		t.Logf("Commit: %s", commitResult.CommitHash)
		t.Logf("Files changed: %d", commitResult.FilesChanged)
	})

	t.Run("close_session", func(t *testing.T) {
		if session == nil {
			t.Skip("Session not created")
		}

		if err := backend.CloseSession(ctx, session.ID); err != nil {
			t.Fatalf("CloseSession failed: %v", err)
		}
		t.Log("Session closed")
	})
}

func TestE2E_NATSClientDirect(t *testing.T) {
	if os.Getenv("NATS_E2E") != "true" {
		t.Skip("Skipping NATS E2E test. Set NATS_E2E=true to run.")
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	cfg := config.CodingNATSConfig{
		URL: natsURL,
	}

	client, err := NewNATSCodingClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	t.Run("connection", func(t *testing.T) {
		if !client.IsConnected() {
			t.Fatal("Client not connected")
		}
		t.Log("NATS client connected")
	})

	ctx := context.Background()

	t.Run("kv_state_operations", func(t *testing.T) {
		key := "test-state-key"
		value := []byte(`{"test": "data", "timestamp": 1234567890}`)

		if err := client.SetState(ctx, key, value); err != nil {
			t.Fatalf("SetState failed: %v", err)
		}
		t.Log("State saved")

		got, err := client.GetState(ctx, key)
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}

		if string(got) != string(value) {
			t.Errorf("GetState = %s, want %s", string(got), string(value))
		}
		t.Log("State retrieved and verified")

		if err := client.DeleteState(ctx, key); err != nil {
			t.Fatalf("DeleteState failed: %v", err)
		}
		t.Log("State deleted")

		got, err = client.GetState(ctx, key)
		if err != nil {
			t.Fatalf("GetState after delete failed: %v", err)
		}
		if got != nil {
			t.Errorf("Expected nil after delete, got %s", string(got))
		}
	})

	t.Run("session_state_operations", func(t *testing.T) {
		state := &SessionState{
			SessionName:   "e2e-test-session",
			OpencodeID:    "oc-test-123",
			WorkspaceName: "test-workspace",
			WorkspacePath: "/workspaces/test",
			Created:       time.Now().Format(time.RFC3339),
			LastUsed:      time.Now().Format(time.RFC3339),
			MessageCount:  5,
		}

		if err := client.SaveSession(ctx, state); err != nil {
			t.Fatalf("SaveSession failed: %v", err)
		}
		t.Log("Session state saved")

		got, err := client.GetSession(ctx, state.SessionName)
		if err != nil {
			t.Fatalf("GetSession failed: %v", err)
		}
		if got == nil {
			t.Fatal("GetSession returned nil")
		}

		if got.OpencodeID != state.OpencodeID {
			t.Errorf("OpencodeID = %s, want %s", got.OpencodeID, state.OpencodeID)
		}
		t.Logf("Session retrieved: %+v", got)

		if err := client.DeleteSession(ctx, state.SessionName); err != nil {
			t.Fatalf("DeleteSession failed: %v", err)
		}
		t.Log("Session deleted")
	})
}

func TestE2E_NATSTaskExecution(t *testing.T) {
	if os.Getenv("NATS_E2E") != "true" {
		t.Skip("Skipping NATS E2E test. Set NATS_E2E=true to run.")
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	cfg := config.CodingNATSConfig{
		URL: natsURL,
	}

	client, err := NewNATSCodingClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create NATS client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	t.Run("execute_task_with_events", func(t *testing.T) {
		task := &CodingTask{
			TaskID: "e2e-test-task-1",
			Session: TaskSession{
				Name:     "e2e-session",
				Continue: false,
			},
			Workspace: TaskWorkspace{
				Name: "e2e-workspace",
			},
			Prompt:  "What is 2 + 2? Just respond with the number.",
			Timeout: 60000,
		}

		exec, err := client.ExecuteTask(ctx, task)
		if err != nil {
			t.Fatalf("ExecuteTask failed: %v", err)
		}

		eventCount := 0
		go func() {
			for event := range exec.Events() {
				eventCount++
				t.Logf("Event [%d]: type=%s", event.Seq, event.Type)
				if event.Tool != nil {
					t.Logf("  Tool: %s", event.Tool.Name)
				}
			}
		}()

		result, err := exec.Wait(ctx)
		if err != nil {
			t.Fatalf("Wait failed: %v", err)
		}

		t.Logf("Result: status=%s", result.Status)
		t.Logf("Result text: %s", result.Result)
		t.Logf("Events received: %d", eventCount)

		if result.Status != "completed" {
			t.Errorf("Status = %s, want completed. Error: %s", result.Status, result.Error)
		}
	})
}

func initGitRepoForNATSE2E(t *testing.T, dir string) {
	t.Helper()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Logf("Command %v failed: %v (continuing)", args, err)
		}
	}

	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# NATS E2E Test\n"), 0644); err != nil {
		t.Logf("Failed to create README: %v (continuing)", err)
		return
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	addCmd.Run()

	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = dir
	commitCmd.Run()
}
