package coding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNewWorkspaceManager_Defaults(t *testing.T) {
	m := NewWorkspaceManager()

	if m.basePath == "" {
		t.Error("expected basePath to be set")
	}
	if m.cleanupPolicy != CleanupOnSessionEnd {
		t.Errorf("expected default cleanup policy to be on_session_end, got %s", m.cleanupPolicy)
	}
	if m.workspaces == nil {
		t.Error("expected workspaces map to be initialized")
	}
}

func TestNewWorkspaceManager_WithOptions(t *testing.T) {
	customPath := "/tmp/custom-workspace"
	m := NewWorkspaceManager(
		WithBasePath(customPath),
		WithCleanupPolicy(CleanupManual),
	)

	if m.basePath != customPath {
		t.Errorf("expected basePath %s, got %s", customPath, m.basePath)
	}
	if m.cleanupPolicy != CleanupManual {
		t.Errorf("expected cleanup policy manual, got %s", m.cleanupPolicy)
	}
}

func TestWorkspaceManager_Create(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-create")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	t.Run("creates agent-scoped workspace", func(t *testing.T) {
		ws, err := m.Create(ctx, ScopeAgent, "session-123")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if ws.ID == "" {
			t.Error("expected workspace ID to be set")
		}
		if ws.Path == "" {
			t.Error("expected workspace path to be set")
		}
		if ws.Scope != ScopeAgent {
			t.Errorf("expected scope agent, got %s", ws.Scope)
		}
		if ws.SessionID != "session-123" {
			t.Errorf("expected session ID session-123, got %s", ws.SessionID)
		}
		if ws.WorkflowID != "" {
			t.Error("expected workflow ID to be empty for agent scope")
		}
		if ws.CreatedAt.IsZero() {
			t.Error("expected created_at to be set")
		}

		// Verify directory exists
		if _, err := os.Stat(ws.Path); os.IsNotExist(err) {
			t.Error("workspace directory was not created")
		}
	})

	t.Run("creates workflow-scoped workspace", func(t *testing.T) {
		ws, err := m.Create(ctx, ScopeWorkflow, "workflow-456")
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}

		if ws.Scope != ScopeWorkflow {
			t.Errorf("expected scope workflow, got %s", ws.Scope)
		}
		if ws.WorkflowID != "workflow-456" {
			t.Errorf("expected workflow ID workflow-456, got %s", ws.WorkflowID)
		}
		if ws.SessionID != "" {
			t.Error("expected session ID to be empty for workflow scope")
		}
	})
}

func TestWorkspaceManager_Get(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-get")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	ws, _ := m.Create(ctx, ScopeAgent, "session-123")

	t.Run("returns existing workspace", func(t *testing.T) {
		found, err := m.Get(ws.ID)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if found.ID != ws.ID {
			t.Errorf("expected ID %s, got %s", ws.ID, found.ID)
		}
	})

	t.Run("returns error for non-existent workspace", func(t *testing.T) {
		_, err := m.Get("non-existent")
		if err == nil {
			t.Error("expected error for non-existent workspace")
		}
	})
}

func TestWorkspaceManager_GetByScope(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-scope")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	agentWs, _ := m.Create(ctx, ScopeAgent, "session-abc")
	workflowWs, _ := m.Create(ctx, ScopeWorkflow, "workflow-xyz")

	t.Run("finds agent-scoped workspace", func(t *testing.T) {
		found, err := m.GetByScope(ScopeAgent, "session-abc")
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}
		if found.ID != agentWs.ID {
			t.Errorf("expected ID %s, got %s", agentWs.ID, found.ID)
		}
	})

	t.Run("finds workflow-scoped workspace", func(t *testing.T) {
		found, err := m.GetByScope(ScopeWorkflow, "workflow-xyz")
		if err != nil {
			t.Fatalf("GetByScope failed: %v", err)
		}
		if found.ID != workflowWs.ID {
			t.Errorf("expected ID %s, got %s", workflowWs.ID, found.ID)
		}
	})

	t.Run("returns error for non-existent scope", func(t *testing.T) {
		_, err := m.GetByScope(ScopeAgent, "non-existent")
		if err == nil {
			t.Error("expected error for non-existent scope")
		}
	})
}

func TestWorkspaceManager_InitGit(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-git")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	ws, _ := m.Create(ctx, ScopeAgent, "session-123")

	t.Run("initializes git repo", func(t *testing.T) {
		err := m.InitGit(ctx, ws)
		if err != nil {
			t.Fatalf("InitGit failed: %v", err)
		}

		if !ws.GitInitialized {
			t.Error("expected GitInitialized to be true")
		}

		// Verify .git directory exists
		gitDir := filepath.Join(ws.Path, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			t.Error(".git directory was not created")
		}
	})

	t.Run("is idempotent", func(t *testing.T) {
		err := m.InitGit(ctx, ws)
		if err != nil {
			t.Fatalf("second InitGit failed: %v", err)
		}
	})
}

func TestWorkspaceManager_CollectChanges(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-changes")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	t.Run("collects changes with git", func(t *testing.T) {
		ws, _ := m.Create(ctx, ScopeAgent, "session-git")
		m.InitGit(ctx, ws)

		// Create a new file
		testFile := filepath.Join(ws.Path, "hello.txt")
		os.WriteFile(testFile, []byte("hello world\n"), 0644)

		changes, err := m.CollectChanges(ctx, ws)
		if err != nil {
			t.Fatalf("CollectChanges failed: %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		if changes[0].Path != "hello.txt" {
			t.Errorf("expected path hello.txt, got %s", changes[0].Path)
		}
		if changes[0].Action != "created" {
			t.Errorf("expected action created, got %s", changes[0].Action)
		}
	})

	t.Run("collects changes without git", func(t *testing.T) {
		ws, _ := m.Create(ctx, ScopeAgent, "session-no-git")

		// Create files without git
		testFile := filepath.Join(ws.Path, "test.py")
		os.WriteFile(testFile, []byte("print('hello')\n"), 0644)

		changes, err := m.CollectChanges(ctx, ws)
		if err != nil {
			t.Fatalf("CollectChanges failed: %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		if changes[0].Path != "test.py" {
			t.Errorf("expected path test.py, got %s", changes[0].Path)
		}
	})

	t.Run("detects modified files", func(t *testing.T) {
		ws, _ := m.Create(ctx, ScopeAgent, "session-modify")
		m.InitGit(ctx, ws)

		// Create and commit a file
		testFile := filepath.Join(ws.Path, "existing.txt")
		os.WriteFile(testFile, []byte("original\n"), 0644)

		// Stage and commit
		runGit(t, ws.Path, "add", ".")
		runGit(t, ws.Path, "commit", "-m", "initial")

		// Modify the file
		os.WriteFile(testFile, []byte("modified\n"), 0644)

		changes, err := m.CollectChanges(ctx, ws)
		if err != nil {
			t.Fatalf("CollectChanges failed: %v", err)
		}

		if len(changes) != 1 {
			t.Fatalf("expected 1 change, got %d", len(changes))
		}

		if changes[0].Action != "modified" {
			t.Errorf("expected action modified, got %s", changes[0].Action)
		}
	})
}

func TestWorkspaceManager_Cleanup(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-cleanup")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	ws, _ := m.Create(ctx, ScopeAgent, "session-123")
	wsPath := ws.Path
	wsID := ws.ID

	err := m.Cleanup(ctx, ws)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify directory removed
	if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
		t.Error("workspace directory should be removed")
	}

	// Verify workspace removed from manager
	_, err = m.Get(wsID)
	if err == nil {
		t.Error("workspace should be removed from manager")
	}
}

func TestWorkspaceManager_CleanupByPolicy(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-policy")
	defer os.RemoveAll(basePath)

	ctx := context.Background()

	t.Run("CleanupOnSessionEnd always cleans up", func(t *testing.T) {
		m := NewWorkspaceManager(
			WithBasePath(filepath.Join(basePath, "session-end")),
			WithCleanupPolicy(CleanupOnSessionEnd),
		)

		ws, _ := m.Create(ctx, ScopeAgent, "session-1")
		wsPath := ws.Path

		m.CleanupByPolicy(ctx, ws, false)

		if _, err := os.Stat(wsPath); !os.IsNotExist(err) {
			t.Error("CleanupOnSessionEnd should remove workspace regardless of success")
		}
	})

	t.Run("CleanupOnSuccess only cleans on success", func(t *testing.T) {
		m := NewWorkspaceManager(
			WithBasePath(filepath.Join(basePath, "on-success")),
			WithCleanupPolicy(CleanupOnSuccess),
		)

		// Test failure case - should NOT clean up
		wsFailure, _ := m.Create(ctx, ScopeAgent, "session-fail")
		wsFailurePath := wsFailure.Path

		m.CleanupByPolicy(ctx, wsFailure, false)

		if _, err := os.Stat(wsFailurePath); os.IsNotExist(err) {
			t.Error("CleanupOnSuccess should NOT remove workspace on failure")
		}

		// Test success case - should clean up
		wsSuccess, _ := m.Create(ctx, ScopeAgent, "session-success")
		wsSuccessPath := wsSuccess.Path

		m.CleanupByPolicy(ctx, wsSuccess, true)

		if _, err := os.Stat(wsSuccessPath); !os.IsNotExist(err) {
			t.Error("CleanupOnSuccess should remove workspace on success")
		}
	})

	t.Run("CleanupManual never cleans", func(t *testing.T) {
		m := NewWorkspaceManager(
			WithBasePath(filepath.Join(basePath, "manual")),
			WithCleanupPolicy(CleanupManual),
		)

		ws, _ := m.Create(ctx, ScopeAgent, "session-manual")
		wsPath := ws.Path

		m.CleanupByPolicy(ctx, ws, true)

		if _, err := os.Stat(wsPath); os.IsNotExist(err) {
			t.Error("CleanupManual should NOT remove workspace")
		}
	})
}

func TestWorkspaceManager_CleanupAll(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-all")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	// Create multiple workspaces
	ws1, _ := m.Create(ctx, ScopeAgent, "session-1")
	ws2, _ := m.Create(ctx, ScopeAgent, "session-2")
	ws3, _ := m.Create(ctx, ScopeWorkflow, "workflow-1")

	paths := []string{ws1.Path, ws2.Path, ws3.Path}

	err := m.CleanupAll(ctx)
	if err != nil {
		t.Fatalf("CleanupAll failed: %v", err)
	}

	// Verify all directories removed
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("workspace %s should be removed", path)
		}
	}

	// Verify manager is empty
	if len(m.ListWorkspaces()) != 0 {
		t.Error("all workspaces should be removed from manager")
	}
}

func TestWorkspaceManager_ListWorkspaces(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-list")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	// Start empty
	if len(m.ListWorkspaces()) != 0 {
		t.Error("expected empty list initially")
	}

	// Create workspaces
	m.Create(ctx, ScopeAgent, "session-1")
	m.Create(ctx, ScopeAgent, "session-2")
	m.Create(ctx, ScopeWorkflow, "workflow-1")

	list := m.ListWorkspaces()
	if len(list) != 3 {
		t.Errorf("expected 3 workspaces, got %d", len(list))
	}
}

func TestWorkspaceManager_GetCommitsSince(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-ws-commits")
	defer os.RemoveAll(basePath)

	m := NewWorkspaceManager(WithBasePath(basePath))
	ctx := context.Background()

	ws, _ := m.Create(ctx, ScopeAgent, "session-123")
	m.InitGit(ctx, ws)

	// Create first commit
	file1 := filepath.Join(ws.Path, "file1.txt")
	os.WriteFile(file1, []byte("content1\n"), 0644)
	runGit(t, ws.Path, "add", ".")
	runGit(t, ws.Path, "commit", "-m", "commit 1")

	// Get first commit hash
	firstCommit := getGitHead(t, ws.Path)

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	// Create second commit
	file2 := filepath.Join(ws.Path, "file2.txt")
	os.WriteFile(file2, []byte("content2\n"), 0644)
	runGit(t, ws.Path, "add", ".")
	runGit(t, ws.Path, "commit", "-m", "commit 2")

	t.Run("returns all commits when since is empty", func(t *testing.T) {
		commits, err := m.GetCommitsSince(ctx, ws, "")
		if err != nil {
			t.Fatalf("GetCommitsSince failed: %v", err)
		}
		if len(commits) < 2 {
			t.Errorf("expected at least 2 commits, got %d", len(commits))
		}
	})

	t.Run("returns commits since specified hash", func(t *testing.T) {
		commits, err := m.GetCommitsSince(ctx, ws, firstCommit)
		if err != nil {
			t.Fatalf("GetCommitsSince failed: %v", err)
		}
		if len(commits) != 1 {
			t.Errorf("expected 1 commit since first, got %d", len(commits))
		}
	})

	t.Run("returns nil for non-git workspace", func(t *testing.T) {
		wsNoGit, _ := m.Create(ctx, ScopeAgent, "session-no-git")
		commits, err := m.GetCommitsSince(ctx, wsNoGit, "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if commits != nil {
			t.Errorf("expected nil commits for non-git workspace")
		}
	})
}

// Helper functions

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}

func getGitHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse failed: %v", err)
	}
	return string(out[:len(out)-1]) // trim newline
}
