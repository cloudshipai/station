//go:build integration

// Package nats provides NATS JetStream integration tests using the lattice embedded server.
// These tests verify that the harness NATS store works correctly with real NATS infrastructure.
//
// Run with: go test ./pkg/harness/nats/... -tags=integration -v
package nats

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/lattice"
)

// getFreePort finds an available TCP port for testing.
func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

// testEnv holds the test environment with embedded NATS.
type testEnv struct {
	server *lattice.EmbeddedServer
	client *lattice.Client
	store  *Store
}

// setupTestEnv creates a test environment with embedded NATS server.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	port := getFreePort(t)
	httpPort := getFreePort(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := lattice.NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded NATS server: %v", err)
	}

	clientCfg := config.LatticeConfig{
		StationID:   "test-harness",
		StationName: "Test Harness Station",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client, err := lattice.NewClient(clientCfg)
	if err != nil {
		server.Shutdown()
		t.Fatalf("Failed to create lattice client: %v", err)
	}

	if err := client.Connect(); err != nil {
		server.Shutdown()
		t.Fatalf("Failed to connect to NATS: %v", err)
	}

	// Create harness store using the lattice client's NATS connection
	store, err := NewStore(client.Conn(), DefaultStoreConfig())
	if err != nil {
		client.Close()
		server.Shutdown()
		t.Fatalf("Failed to create harness store: %v", err)
	}

	return &testEnv{
		server: server,
		client: client,
		store:  store,
	}
}

// cleanup tears down the test environment.
func (e *testEnv) cleanup() {
	if e.store != nil {
		e.store.Close()
	}
	if e.client != nil {
		e.client.Close()
	}
	if e.server != nil {
		e.server.Shutdown()
	}
}

// =============================================================================
// KV Store Tests
// =============================================================================

func TestIntegration_KV_BasicOperations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	t.Run("SetAndGetState", func(t *testing.T) {
		key := "test.key.1"
		value := []byte("hello world")

		// Set state
		err := env.store.SetState(ctx, key, value)
		if err != nil {
			t.Fatalf("SetState failed: %v", err)
		}

		// Get state
		got, err := env.store.GetState(ctx, key)
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}

		if !bytes.Equal(got, value) {
			t.Errorf("GetState returned wrong value: got %q, want %q", got, value)
		}
	})

	t.Run("GetNonExistentKey", func(t *testing.T) {
		got, err := env.store.GetState(ctx, "nonexistent.key")
		if err != nil {
			t.Fatalf("GetState failed: %v", err)
		}

		if got != nil {
			t.Errorf("Expected nil for non-existent key, got %q", got)
		}
	})

	t.Run("DeleteState", func(t *testing.T) {
		key := "test.delete.key"
		value := []byte("to be deleted")

		// Set state
		err := env.store.SetState(ctx, key, value)
		if err != nil {
			t.Fatalf("SetState failed: %v", err)
		}

		// Delete state
		err = env.store.DeleteState(ctx, key)
		if err != nil {
			t.Fatalf("DeleteState failed: %v", err)
		}

		// Verify deletion
		got, err := env.store.GetState(ctx, key)
		if err != nil {
			t.Fatalf("GetState after delete failed: %v", err)
		}

		if got != nil {
			t.Errorf("Expected nil after delete, got %q", got)
		}
	})

	t.Run("SetAndGetJSON", func(t *testing.T) {
		key := "test.json.key"
		original := RunState{
			RunID:     "run-123",
			AgentID:   "agent-1",
			AgentName: "test-agent",
			Status:    "running",
			Task:      "Do something",
			StartedAt: time.Now().Truncate(time.Second),
		}

		// Set JSON
		err := env.store.SetJSON(ctx, key, original)
		if err != nil {
			t.Fatalf("SetJSON failed: %v", err)
		}

		// Get JSON
		var got RunState
		err = env.store.GetJSON(ctx, key, &got)
		if err != nil {
			t.Fatalf("GetJSON failed: %v", err)
		}

		if got.RunID != original.RunID {
			t.Errorf("RunID mismatch: got %q, want %q", got.RunID, original.RunID)
		}
		if got.AgentName != original.AgentName {
			t.Errorf("AgentName mismatch: got %q, want %q", got.AgentName, original.AgentName)
		}
		if got.Status != original.Status {
			t.Errorf("Status mismatch: got %q, want %q", got.Status, original.Status)
		}
	})
}

func TestIntegration_KV_RunState(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	t.Run("SetAndGetRunState", func(t *testing.T) {
		state := &RunState{
			RunID:      "run-456",
			AgentID:    "agent-2",
			AgentName:  "coder",
			WorkflowID: "wf-1",
			StepName:   "analyze",
			Status:     "running",
			StartedAt:  time.Now(),
			Task:       "Analyze the codebase",
			GitBranch:  "feature/test",
		}

		err := env.store.SetRunState(ctx, state)
		if err != nil {
			t.Fatalf("SetRunState failed: %v", err)
		}

		got, err := env.store.GetRunState(ctx, state.RunID)
		if err != nil {
			t.Fatalf("GetRunState failed: %v", err)
		}

		if got == nil {
			t.Fatal("GetRunState returned nil")
		}

		if got.RunID != state.RunID {
			t.Errorf("RunID mismatch: got %q, want %q", got.RunID, state.RunID)
		}
		if got.WorkflowID != state.WorkflowID {
			t.Errorf("WorkflowID mismatch: got %q, want %q", got.WorkflowID, state.WorkflowID)
		}
		if got.GitBranch != state.GitBranch {
			t.Errorf("GitBranch mismatch: got %q, want %q", got.GitBranch, state.GitBranch)
		}
	})

	t.Run("UpdateRunState", func(t *testing.T) {
		state := &RunState{
			RunID:     "run-789",
			AgentName: "reviewer",
			Status:    "running",
			StartedAt: time.Now(),
			Task:      "Review code",
		}

		// Create initial state
		err := env.store.SetRunState(ctx, state)
		if err != nil {
			t.Fatalf("SetRunState failed: %v", err)
		}

		// Update state
		now := time.Now()
		state.Status = "completed"
		state.Result = "Code looks good!"
		state.CompletedAt = &now

		err = env.store.SetRunState(ctx, state)
		if err != nil {
			t.Fatalf("SetRunState (update) failed: %v", err)
		}

		// Verify update
		got, err := env.store.GetRunState(ctx, state.RunID)
		if err != nil {
			t.Fatalf("GetRunState failed: %v", err)
		}

		if got.Status != "completed" {
			t.Errorf("Status not updated: got %q, want %q", got.Status, "completed")
		}
		if got.Result != "Code looks good!" {
			t.Errorf("Result not updated: got %q", got.Result)
		}
		if got.CompletedAt == nil {
			t.Error("CompletedAt not set")
		}
	})
}

func TestIntegration_KV_WorkflowContext(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	t.Run("SetAndGetWorkflowContext", func(t *testing.T) {
		wctx := &WorkflowContext{
			WorkflowID:    "wf-analyze-fix",
			WorkflowRunID: "wfr-001",
			StartedAt:     time.Now(),
			GitBranch:     "agent/wf-analyze-fix",
			Steps:         []WorkflowStepSummary{},
			SharedData: map[string]interface{}{
				"target_file": "main.go",
				"priority":    "high",
			},
		}

		err := env.store.SetWorkflowContext(ctx, wctx)
		if err != nil {
			t.Fatalf("SetWorkflowContext failed: %v", err)
		}

		got, err := env.store.GetWorkflowContext(ctx, wctx.WorkflowRunID)
		if err != nil {
			t.Fatalf("GetWorkflowContext failed: %v", err)
		}

		if got == nil {
			t.Fatal("GetWorkflowContext returned nil")
		}

		if got.WorkflowID != wctx.WorkflowID {
			t.Errorf("WorkflowID mismatch: got %q, want %q", got.WorkflowID, wctx.WorkflowID)
		}
		if got.GitBranch != wctx.GitBranch {
			t.Errorf("GitBranch mismatch: got %q, want %q", got.GitBranch, wctx.GitBranch)
		}
		if got.SharedData["target_file"] != "main.go" {
			t.Errorf("SharedData mismatch: got %v", got.SharedData)
		}
	})

	t.Run("AddWorkflowStep", func(t *testing.T) {
		wctx := &WorkflowContext{
			WorkflowID:    "wf-multi-step",
			WorkflowRunID: "wfr-002",
			StartedAt:     time.Now(),
			GitBranch:     "agent/multi-step",
			Steps:         []WorkflowStepSummary{},
		}

		err := env.store.SetWorkflowContext(ctx, wctx)
		if err != nil {
			t.Fatalf("SetWorkflowContext failed: %v", err)
		}

		// Add first step
		now := time.Now()
		step1 := WorkflowStepSummary{
			StepName:      "analyze",
			AgentName:     "analyzer",
			RunID:         "run-step1",
			Status:        "completed",
			StartedAt:     now,
			CompletedAt:   &now,
			Summary:       "Found 3 issues",
			FilesModified: []string{"main.go", "utils.go"},
			Commits:       []string{"abc123"},
		}

		err = env.store.AddWorkflowStep(ctx, wctx.WorkflowRunID, step1)
		if err != nil {
			t.Fatalf("AddWorkflowStep failed: %v", err)
		}

		// Add second step
		step2 := WorkflowStepSummary{
			StepName:  "fix",
			AgentName: "fixer",
			RunID:     "run-step2",
			Status:    "completed",
			StartedAt: now,
			Summary:   "Fixed all issues",
		}

		err = env.store.AddWorkflowStep(ctx, wctx.WorkflowRunID, step2)
		if err != nil {
			t.Fatalf("AddWorkflowStep (second) failed: %v", err)
		}

		// Verify steps
		got, err := env.store.GetWorkflowContext(ctx, wctx.WorkflowRunID)
		if err != nil {
			t.Fatalf("GetWorkflowContext failed: %v", err)
		}

		if len(got.Steps) != 2 {
			t.Fatalf("Expected 2 steps, got %d", len(got.Steps))
		}

		if got.Steps[0].StepName != "analyze" {
			t.Errorf("First step name mismatch: got %q", got.Steps[0].StepName)
		}
		if got.Steps[1].StepName != "fix" {
			t.Errorf("Second step name mismatch: got %q", got.Steps[1].StepName)
		}
	})
}

// =============================================================================
// ObjectStore Tests
// =============================================================================

func TestIntegration_ObjectStore_BasicOperations(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	t.Run("PutAndGetFile", func(t *testing.T) {
		content := []byte("Hello from the harness ObjectStore!")
		key := "test/hello.txt"

		meta, err := env.store.PutFile(ctx, key, bytes.NewReader(content), PutFileOptions{
			ContentType: "text/plain",
			Description: "Test file",
		})
		if err != nil {
			t.Fatalf("PutFile failed: %v", err)
		}

		if meta.Key != key {
			t.Errorf("Key mismatch: got %q, want %q", meta.Key, key)
		}
		if meta.Size != int64(len(content)) {
			t.Errorf("Size mismatch: got %d, want %d", meta.Size, len(content))
		}

		// Get file
		reader, gotMeta, err := env.store.GetFile(ctx, key)
		if err != nil {
			t.Fatalf("GetFile failed: %v", err)
		}
		defer reader.Close()

		gotContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(gotContent, content) {
			t.Errorf("Content mismatch: got %q, want %q", gotContent, content)
		}

		if gotMeta.Size != int64(len(content)) {
			t.Errorf("Metadata size mismatch: got %d, want %d", gotMeta.Size, len(content))
		}
	})

	t.Run("GetNonExistentFile", func(t *testing.T) {
		reader, meta, err := env.store.GetFile(ctx, "nonexistent/file.txt")
		if err != nil {
			t.Fatalf("GetFile failed: %v", err)
		}

		if reader != nil || meta != nil {
			t.Error("Expected nil for non-existent file")
			if reader != nil {
				reader.Close()
			}
		}
	})

	t.Run("DeleteFile", func(t *testing.T) {
		content := []byte("Delete me")
		key := "test/delete-me.txt"

		_, err := env.store.PutFile(ctx, key, bytes.NewReader(content), PutFileOptions{})
		if err != nil {
			t.Fatalf("PutFile failed: %v", err)
		}

		err = env.store.DeleteFile(ctx, key)
		if err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}

		reader, _, err := env.store.GetFile(ctx, key)
		if err != nil {
			t.Fatalf("GetFile after delete failed: %v", err)
		}

		if reader != nil {
			reader.Close()
			t.Error("Expected nil after delete")
		}
	})

	t.Run("ListFiles", func(t *testing.T) {
		// Create several files with a common prefix
		prefix := "listtest/"
		files := []string{"a.txt", "b.txt", "c.txt"}

		for _, f := range files {
			key := prefix + f
			_, err := env.store.PutFile(ctx, key, bytes.NewReader([]byte(f)), PutFileOptions{})
			if err != nil {
				t.Fatalf("PutFile %s failed: %v", key, err)
			}
		}

		// List files
		list, err := env.store.ListFiles(ctx, prefix)
		if err != nil {
			t.Fatalf("ListFiles failed: %v", err)
		}

		if len(list) < len(files) {
			t.Errorf("Expected at least %d files, got %d", len(files), len(list))
		}
	})
}

func TestIntegration_ObjectStore_RunFiles(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	runID := "run-output-test"

	t.Run("PutAndGetRunFile", func(t *testing.T) {
		content := []byte(`{"analysis": "done", "issues": 3}`)
		filename := "analysis.json"

		meta, err := env.store.PutRunFile(ctx, runID, filename, bytes.NewReader(content), PutFileOptions{
			ContentType: "application/json",
			Metadata: map[string]string{
				"step": "analyze",
			},
		})
		if err != nil {
			t.Fatalf("PutRunFile failed: %v", err)
		}

		expectedKey := fmt.Sprintf("run/%s/output/%s", runID, filename)
		if meta.Key != expectedKey {
			t.Errorf("Key mismatch: got %q, want %q", meta.Key, expectedKey)
		}

		// Get run file
		reader, _, err := env.store.GetRunFile(ctx, runID, filename)
		if err != nil {
			t.Fatalf("GetRunFile failed: %v", err)
		}
		defer reader.Close()

		gotContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(gotContent, content) {
			t.Errorf("Content mismatch: got %q, want %q", gotContent, content)
		}
	})

	t.Run("ListRunFiles", func(t *testing.T) {
		// Add more files to the run
		files := []string{"output1.txt", "output2.txt", "results.json"}
		for _, f := range files {
			_, err := env.store.PutRunFile(ctx, runID, f, bytes.NewReader([]byte(f)), PutFileOptions{})
			if err != nil {
				t.Fatalf("PutRunFile %s failed: %v", f, err)
			}
		}

		list, err := env.store.ListRunFiles(ctx, runID)
		if err != nil {
			t.Fatalf("ListRunFiles failed: %v", err)
		}

		// Should have at least the files we created (plus analysis.json from previous test)
		if len(list) < len(files) {
			t.Errorf("Expected at least %d files, got %d", len(files), len(list))
		}
	})
}

func TestIntegration_ObjectStore_SharedFiles(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	t.Run("PutAndGetSharedFile", func(t *testing.T) {
		content := []byte("Shared configuration data")
		key := "config/shared.yaml"

		meta, err := env.store.PutSharedFile(ctx, key, bytes.NewReader(content), PutFileOptions{
			ContentType: "application/yaml",
			TTL:         24 * time.Hour,
		})
		if err != nil {
			t.Fatalf("PutSharedFile failed: %v", err)
		}

		expectedKey := "shared/" + key
		if meta.Key != expectedKey {
			t.Errorf("Key mismatch: got %q, want %q", meta.Key, expectedKey)
		}

		// Get shared file
		reader, _, err := env.store.GetSharedFile(ctx, key)
		if err != nil {
			t.Fatalf("GetSharedFile failed: %v", err)
		}
		defer reader.Close()

		gotContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("ReadAll failed: %v", err)
		}

		if !bytes.Equal(gotContent, content) {
			t.Errorf("Content mismatch: got %q, want %q", gotContent, content)
		}
	})
}

// =============================================================================
// Workflow Handoff Tests
// =============================================================================

func TestIntegration_WorkflowHandoff(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()
	handoff := NewHandoffManager(env.store)

	t.Run("FullWorkflowHandoff", func(t *testing.T) {
		// Step 1: Start workflow
		wctx, err := handoff.StartWorkflow(ctx, StartWorkflowInput{
			WorkflowID:    "wf-code-review",
			WorkflowRunID: "wfr-handoff-test",
			GitBranch:     "agent/code-review",
			SharedData: map[string]interface{}{
				"target_pr": "PR-123",
				"repo":      "myorg/myrepo",
			},
		})
		if err != nil {
			t.Fatalf("StartWorkflow failed: %v", err)
		}

		if wctx.WorkflowID != "wf-code-review" {
			t.Errorf("WorkflowID mismatch: got %q", wctx.WorkflowID)
		}

		// Step 2: Start first step (analyzer)
		runState, gotWctx, err := handoff.StartStep(ctx, StartStepInput{
			WorkflowRunID: wctx.WorkflowRunID,
			StepName:      "analyze",
			AgentName:     "code-analyzer",
			RunID:         "run-analyze-1",
			Task:          "Analyze PR-123 for issues",
		})
		if err != nil {
			t.Fatalf("StartStep failed: %v", err)
		}

		if runState.Status != "running" {
			t.Errorf("RunState status mismatch: got %q", runState.Status)
		}
		if gotWctx.SharedData["target_pr"] != "PR-123" {
			t.Errorf("SharedData not preserved: got %v", gotWctx.SharedData)
		}

		// Step 3: Complete first step
		err = handoff.CompleteStep(ctx, CompleteStepInput{
			RunID:         runState.RunID,
			WorkflowRunID: wctx.WorkflowRunID,
			Status:        "completed",
			Result:        "Found 2 issues: unused variable, missing error check",
			Summary:       "Code analysis complete",
			FilesModified: []string{"main.go"},
			Commits:       []string{"def456"},
		})
		if err != nil {
			t.Fatalf("CompleteStep failed: %v", err)
		}

		// Step 4: Get previous step context (for next agent)
		prevCtx, err := handoff.GetPreviousStepContext(ctx, wctx.WorkflowRunID)
		if err != nil {
			t.Fatalf("GetPreviousStepContext failed: %v", err)
		}

		if prevCtx == nil {
			t.Fatal("GetPreviousStepContext returned nil")
		}

		if prevCtx.StepName != "analyze" {
			t.Errorf("Previous step name mismatch: got %q", prevCtx.StepName)
		}
		if prevCtx.Summary != "Code analysis complete" {
			t.Errorf("Previous step summary mismatch: got %q", prevCtx.Summary)
		}

		// Step 5: Update shared data
		err = handoff.UpdateSharedData(ctx, wctx.WorkflowRunID,
			SharedDataUpdate{Key: "issues_found", Value: 2},
			SharedDataUpdate{Key: "analyzer_result", Value: "needs_fix"},
		)
		if err != nil {
			t.Fatalf("UpdateSharedData failed: %v", err)
		}

		// Step 6: Get shared data
		issuesFound, err := handoff.GetSharedData(ctx, wctx.WorkflowRunID, "issues_found")
		if err != nil {
			t.Fatalf("GetSharedData failed: %v", err)
		}

		// Note: JSON unmarshaling converts numbers to float64
		if issuesFound != float64(2) {
			t.Errorf("SharedData issues_found mismatch: got %v (type: %T)", issuesFound, issuesFound)
		}

		// Step 7: Start second step (fixer)
		runState2, _, err := handoff.StartStep(ctx, StartStepInput{
			WorkflowRunID: wctx.WorkflowRunID,
			StepName:      "fix",
			AgentName:     "code-fixer",
			RunID:         "run-fix-1",
			Task:          "Fix the 2 issues found by analyzer",
		})
		if err != nil {
			t.Fatalf("StartStep (fixer) failed: %v", err)
		}

		// Step 8: Complete second step
		err = handoff.CompleteStep(ctx, CompleteStepInput{
			RunID:         runState2.RunID,
			WorkflowRunID: wctx.WorkflowRunID,
			Status:        "completed",
			Result:        "Fixed all issues",
			Summary:       "Code fixes applied",
			FilesModified: []string{"main.go", "utils.go"},
			Commits:       []string{"ghi789"},
		})
		if err != nil {
			t.Fatalf("CompleteStep (fixer) failed: %v", err)
		}

		// Verify final workflow state
		finalWctx, err := env.store.GetWorkflowContext(ctx, wctx.WorkflowRunID)
		if err != nil {
			t.Fatalf("GetWorkflowContext failed: %v", err)
		}

		if len(finalWctx.Steps) != 2 {
			t.Fatalf("Expected 2 steps, got %d", len(finalWctx.Steps))
		}

		if finalWctx.Steps[0].StepName != "analyze" {
			t.Errorf("First step name mismatch: got %q", finalWctx.Steps[0].StepName)
		}
		if finalWctx.Steps[1].StepName != "fix" {
			t.Errorf("Second step name mismatch: got %q", finalWctx.Steps[1].StepName)
		}

		t.Logf("Workflow completed successfully with %d steps", len(finalWctx.Steps))
	})
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestIntegration_ConcurrentAccess(t *testing.T) {
	env := setupTestEnv(t)
	defer env.cleanup()

	ctx := context.Background()

	t.Run("ConcurrentKVWrites", func(t *testing.T) {
		numGoroutines := 10
		numWritesPerGoroutine := 20

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*numWritesPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < numWritesPerGoroutine; j++ {
					key := fmt.Sprintf("concurrent.%d.%d", goroutineID, j)
					value := []byte(fmt.Sprintf("value-%d-%d", goroutineID, j))

					if err := env.store.SetState(ctx, key, value); err != nil {
						errors <- fmt.Errorf("goroutine %d, write %d: %w", goroutineID, j, err)
						return
					}

					got, err := env.store.GetState(ctx, key)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d, read %d: %w", goroutineID, j, err)
						return
					}

					if !bytes.Equal(got, value) {
						errors <- fmt.Errorf("goroutine %d, mismatch %d: got %q, want %q", goroutineID, j, got, value)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		var errList []error
		for err := range errors {
			errList = append(errList, err)
		}

		if len(errList) > 0 {
			t.Fatalf("Concurrent access errors: %v", errList)
		}

		t.Logf("Successfully completed %d concurrent writes", numGoroutines*numWritesPerGoroutine)
	})

	t.Run("ConcurrentFileUploads", func(t *testing.T) {
		numGoroutines := 5
		numFilesPerGoroutine := 10

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*numFilesPerGoroutine)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < numFilesPerGoroutine; j++ {
					key := fmt.Sprintf("concurrent-files/%d/file-%d.txt", goroutineID, j)
					content := []byte(fmt.Sprintf("Content from goroutine %d, file %d", goroutineID, j))

					_, err := env.store.PutFile(ctx, key, bytes.NewReader(content), PutFileOptions{})
					if err != nil {
						errors <- fmt.Errorf("goroutine %d, upload %d: %w", goroutineID, j, err)
						return
					}

					reader, _, err := env.store.GetFile(ctx, key)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d, download %d: %w", goroutineID, j, err)
						return
					}

					got, err := io.ReadAll(reader)
					reader.Close()
					if err != nil {
						errors <- fmt.Errorf("goroutine %d, read %d: %w", goroutineID, j, err)
						return
					}

					if !bytes.Equal(got, content) {
						errors <- fmt.Errorf("goroutine %d, mismatch %d", goroutineID, j)
						return
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		var errList []error
		for err := range errors {
			errList = append(errList, err)
		}

		if len(errList) > 0 {
			t.Fatalf("Concurrent file upload errors: %v", errList)
		}

		t.Logf("Successfully completed %d concurrent file uploads", numGoroutines*numFilesPerGoroutine)
	})
}

// =============================================================================
// Multi-Station Simulation
// =============================================================================

func TestIntegration_MultiStationWorkflow(t *testing.T) {
	// Use shared embedded server
	port := getFreePort(t)
	httpPort := getFreePort(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := lattice.NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded NATS server: %v", err)
	}
	defer server.Shutdown()

	// Create two stations (simulating distributed agents)
	createStation := func(name string) (*lattice.Client, *Store, *HandoffManager) {
		clientCfg := config.LatticeConfig{
			StationID:   name,
			StationName: name + " Station",
			NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
		}

		client, err := lattice.NewClient(clientCfg)
		if err != nil {
			t.Fatalf("Failed to create client for %s: %v", name, err)
		}

		if err := client.Connect(); err != nil {
			t.Fatalf("Failed to connect %s: %v", name, err)
		}

		store, err := NewStore(client.Conn(), DefaultStoreConfig())
		if err != nil {
			client.Close()
			t.Fatalf("Failed to create store for %s: %v", name, err)
		}

		return client, store, NewHandoffManager(store)
	}

	// Station A: Analyzer
	clientA, storeA, handoffA := createStation("station-a")
	defer storeA.Close()
	defer clientA.Close()

	// Station B: Fixer
	clientB, storeB, handoffB := createStation("station-b")
	defer storeB.Close()
	defer clientB.Close()

	ctx := context.Background()

	t.Run("CrossStationWorkflowHandoff", func(t *testing.T) {
		workflowRunID := "wfr-cross-station"

		_, err := handoffA.StartWorkflow(ctx, StartWorkflowInput{
			WorkflowID:    "wf-distributed",
			WorkflowRunID: workflowRunID,
			GitBranch:     "agent/distributed-fix",
			SharedData: map[string]interface{}{
				"initiated_by": "station-a",
			},
		})
		if err != nil {
			t.Fatalf("Station A StartWorkflow failed: %v", err)
		}

		// Station A executes first step
		runStateA, _, err := handoffA.StartStep(ctx, StartStepInput{
			WorkflowRunID: workflowRunID,
			StepName:      "analyze",
			AgentName:     "analyzer-a",
			RunID:         "run-a-1",
			Task:          "Analyze codebase",
		})
		if err != nil {
			t.Fatalf("Station A StartStep failed: %v", err)
		}

		// Station A completes step and uploads output file
		outputContent := []byte(`{"bugs": ["null-pointer", "race-condition"], "severity": "high"}`)
		_, err = storeA.PutRunFile(ctx, runStateA.RunID, "analysis-report.json", bytes.NewReader(outputContent), PutFileOptions{
			ContentType: "application/json",
		})
		if err != nil {
			t.Fatalf("Station A PutRunFile failed: %v", err)
		}

		err = handoffA.CompleteStep(ctx, CompleteStepInput{
			RunID:         runStateA.RunID,
			WorkflowRunID: workflowRunID,
			Status:        "completed",
			Result:        "Found 2 critical bugs",
			Summary:       "Analysis complete",
			FilesModified: []string{},
			Commits:       []string{},
		})
		if err != nil {
			t.Fatalf("Station A CompleteStep failed: %v", err)
		}

		t.Log("Station A completed analysis step")

		// Station B picks up the workflow (simulating handoff)
		// Station B can read the workflow context because it's stored in shared NATS KV
		prevCtx, err := handoffB.GetPreviousStepContext(ctx, workflowRunID)
		if err != nil {
			t.Fatalf("Station B GetPreviousStepContext failed: %v", err)
		}

		if prevCtx == nil {
			t.Fatal("Station B could not read previous step context from Station A")
		}

		if prevCtx.StepName != "analyze" {
			t.Errorf("Station B saw wrong previous step: got %q", prevCtx.StepName)
		}

		t.Logf("Station B received handoff: previous step=%q, summary=%q", prevCtx.StepName, prevCtx.Summary)

		// Station B downloads output from Station A
		reader, _, err := storeB.GetRunFile(ctx, runStateA.RunID, "analysis-report.json")
		if err != nil {
			t.Fatalf("Station B GetRunFile failed: %v", err)
		}

		downloadedContent, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("Station B ReadAll failed: %v", err)
		}

		if !bytes.Equal(downloadedContent, outputContent) {
			t.Errorf("Station B received wrong content: got %q, want %q", downloadedContent, outputContent)
		}

		t.Log("Station B downloaded analysis report from Station A")

		// Station B executes fix step
		runStateB, _, err := handoffB.StartStep(ctx, StartStepInput{
			WorkflowRunID: workflowRunID,
			StepName:      "fix",
			AgentName:     "fixer-b",
			RunID:         "run-b-1",
			Task:          "Fix the 2 bugs found by analyzer",
		})
		if err != nil {
			t.Fatalf("Station B StartStep failed: %v", err)
		}

		err = handoffB.CompleteStep(ctx, CompleteStepInput{
			RunID:         runStateB.RunID,
			WorkflowRunID: workflowRunID,
			Status:        "completed",
			Result:        "All bugs fixed",
			Summary:       "Fixes applied",
			FilesModified: []string{"main.go", "handler.go"},
			Commits:       []string{"fix123", "fix456"},
		})
		if err != nil {
			t.Fatalf("Station B CompleteStep failed: %v", err)
		}

		t.Log("Station B completed fix step")

		// Verify final state (readable from either station)
		finalCtx, err := storeA.GetWorkflowContext(ctx, workflowRunID)
		if err != nil {
			t.Fatalf("Final GetWorkflowContext failed: %v", err)
		}

		if len(finalCtx.Steps) != 2 {
			t.Fatalf("Expected 2 steps, got %d", len(finalCtx.Steps))
		}

		if finalCtx.Steps[0].AgentName != "analyzer-a" {
			t.Errorf("First step agent mismatch: got %q", finalCtx.Steps[0].AgentName)
		}
		if finalCtx.Steps[1].AgentName != "fixer-b" {
			t.Errorf("Second step agent mismatch: got %q", finalCtx.Steps[1].AgentName)
		}

		t.Logf("Cross-station workflow completed: %d steps across 2 stations", len(finalCtx.Steps))
		t.Logf("  Step 1: %s by %s on %s", finalCtx.Steps[0].StepName, finalCtx.Steps[0].AgentName, "station-a")
		t.Logf("  Step 2: %s by %s on %s", finalCtx.Steps[1].StepName, finalCtx.Steps[1].AgentName, "station-b")
	})
}
