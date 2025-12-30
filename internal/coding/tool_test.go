package coding

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
)

type mockBackend struct {
	pingErr       error
	createSession *Session
	createErr     error
	getSession    *Session
	getErr        error
	closeErr      error
	executeResult *Result
	executeErr    error
	lastTask      Task
	lastSessionID string
	lastWorkspace string
}

func (m *mockBackend) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockBackend) CreateSession(ctx context.Context, workspacePath, title string) (*Session, error) {
	m.lastWorkspace = workspacePath
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createSession != nil {
		return m.createSession, nil
	}
	return &Session{
		ID:            "test-session-1",
		WorkspacePath: workspacePath,
		Title:         title,
		CreatedAt:     time.Now(),
	}, nil
}

func (m *mockBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	m.lastSessionID = sessionID
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.getSession, nil
}

func (m *mockBackend) CloseSession(ctx context.Context, sessionID string) error {
	m.lastSessionID = sessionID
	return m.closeErr
}

func (m *mockBackend) Execute(ctx context.Context, sessionID string, task Task) (*Result, error) {
	m.lastSessionID = sessionID
	m.lastTask = task
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return m.executeResult, nil
}

func TestToolFactory_CreateOpenTool(t *testing.T) {
	mock := &mockBackend{}
	factory := NewToolFactory(mock)
	tool := factory.CreateOpenTool()

	if tool.Name() != "coding_open" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "coding_open")
	}

	t.Run("success", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"workspace_path": "/workspaces/my-repo",
			"title":          "Fix bugs",
		}

		result, err := tool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map output, got %T", result)
		}

		if output["session_id"] == "" {
			t.Error("expected non-empty session_id")
		}
		if output["workspace_path"] != "/workspaces/my-repo" {
			t.Errorf("workspace_path = %q, want %q", output["workspace_path"], "/workspaces/my-repo")
		}
	})

	t.Run("missing workspace_path", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing workspace_path")
		}
	})

	t.Run("backend error", func(t *testing.T) {
		mock.createErr = errors.New("connection refused")
		defer func() { mock.createErr = nil }()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{"workspace_path": "/ws"}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error from backend")
		}
	})
}

func TestToolFactory_CreateCodeTool(t *testing.T) {
	mock := &mockBackend{
		executeResult: &Result{
			Success: true,
			Summary: "Fixed the null pointer exception",
			FilesChanged: []FileChange{
				{Path: "auth.go", Action: "modified", LinesAdded: 5, LinesRemoved: 2},
			},
			Trace: &Trace{
				Tokens: TokenUsage{Input: 1000, Output: 500},
				Cost:   0.015,
			},
		},
	}
	factory := NewToolFactory(mock)
	tool := factory.CreateCodeTool()

	if tool.Name() != "code" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "code")
	}

	t.Run("success", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id":  "sess-1",
			"instruction": "Fix the null pointer exception in auth.go",
			"context":     "Users report crashes on login",
			"files":       []any{"auth.go", "user.go"},
		}

		result, err := tool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map output, got %T", result)
		}

		if output["success"] != true {
			t.Error("expected success = true")
		}
		if output["summary"] != "Fixed the null pointer exception" {
			t.Errorf("summary = %q, want %q", output["summary"], "Fixed the null pointer exception")
		}

		if mock.lastTask.Instruction != "Fix the null pointer exception in auth.go" {
			t.Errorf("task instruction not passed correctly")
		}
		if mock.lastTask.Context != "Users report crashes on login" {
			t.Errorf("task context not passed correctly")
		}
		if len(mock.lastTask.Files) != 2 {
			t.Errorf("task files not passed correctly")
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{"instruction": "do something"}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing session_id")
		}
	})

	t.Run("missing instruction", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{"session_id": "sess-1"}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing instruction")
		}
	})

	t.Run("backend error", func(t *testing.T) {
		mock.executeErr = errors.New("timeout")
		defer func() { mock.executeErr = nil }()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id":  "sess-1",
			"instruction": "do something",
		}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error from backend")
		}
	})
}

func TestToolFactory_CreateCloseTool(t *testing.T) {
	mock := &mockBackend{}
	factory := NewToolFactory(mock)
	tool := factory.CreateCloseTool()

	if tool.Name() != "coding_close" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "coding_close")
	}

	t.Run("success", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{"session_id": "sess-1"}

		result, err := tool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map output, got %T", result)
		}

		if output["ok"] != true {
			t.Error("expected ok = true")
		}
		if mock.lastSessionID != "sess-1" {
			t.Errorf("session_id not passed correctly")
		}
	})

	t.Run("missing session_id", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing session_id")
		}
	})

	t.Run("backend error", func(t *testing.T) {
		mock.closeErr = ErrSessionNotFound
		defer func() { mock.closeErr = nil }()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{"session_id": "nonexistent"}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error from backend")
		}
	})
}

func TestToolFactory_CreateAllTools(t *testing.T) {
	mock := &mockBackend{}
	factory := NewToolFactory(mock)
	tools := factory.CreateAllTools()

	if len(tools) != 5 {
		t.Errorf("CreateAllTools() returned %d tools, want 5", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name()] = true
	}

	expected := []string{"coding_open", "code", "coding_close", "coding_commit", "coding_push"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestToolFactory_CreateCommitTool(t *testing.T) {
	t.Run("missing_session_id", func(t *testing.T) {
		mock := &mockBackend{}
		factory := NewToolFactory(mock)
		tool := factory.CreateCommitTool()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"message": "test commit",
		}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing session_id")
		}
	})

	t.Run("missing_message", func(t *testing.T) {
		mock := &mockBackend{}
		factory := NewToolFactory(mock)
		tool := factory.CreateCommitTool()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id": "test-session",
		}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing message")
		}
	})

	t.Run("session_not_found", func(t *testing.T) {
		mock := &mockBackend{
			getErr: errors.New("session not found"),
		}
		factory := NewToolFactory(mock)
		tool := factory.CreateCommitTool()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id": "nonexistent",
			"message":    "test commit",
		}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for session not found")
		}
	})
}

func TestToolFactory_CreatePushTool(t *testing.T) {
	t.Run("missing_session_id", func(t *testing.T) {
		mock := &mockBackend{}
		factory := NewToolFactory(mock)
		tool := factory.CreatePushTool()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for missing session_id")
		}
	})

	t.Run("session_not_found", func(t *testing.T) {
		mock := &mockBackend{
			getErr: errors.New("session not found"),
		}
		factory := NewToolFactory(mock)
		tool := factory.CreatePushTool()

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id": "nonexistent",
		}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error for session not found")
		}
	})
}

func TestToolFactory_CreateCommitTool_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	tmpDir := t.TempDir()

	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	configCmd1 := exec.Command("git", "config", "user.email", "test@example.com")
	configCmd1.Dir = tmpDir
	configCmd1.Run()

	configCmd2 := exec.Command("git", "config", "user.name", "Test User")
	configCmd2.Dir = tmpDir
	configCmd2.Run()

	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	session := &Session{
		ID:            "test-session",
		WorkspacePath: tmpDir,
	}
	mock := &mockBackend{
		getSession: session,
	}
	factory := NewToolFactory(mock)
	tool := factory.CreateCommitTool()

	toolCtx := &ai.ToolContext{Context: context.Background()}
	input := map[string]any{
		"session_id": "test-session",
		"message":    "Initial commit",
		"add_all":    true,
	}

	result, err := tool.RunRaw(toolCtx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output, ok := result.(GitCommitResult)
	if !ok {
		resultMap, isMap := result.(map[string]any)
		if !isMap {
			t.Fatalf("expected GitCommitResult or map, got %T", result)
		}
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v (error: %v)", resultMap["success"], resultMap["error"])
		}
		if resultMap["commit_hash"] == "" || resultMap["commit_hash"] == nil {
			t.Error("expected non-empty commit_hash")
		}
		return
	}

	if !output.Success {
		t.Errorf("expected success, got error: %s", output.Error)
	}
	if output.CommitHash == "" {
		t.Error("expected non-empty commit hash")
	}
	if output.Message != "Initial commit" {
		t.Errorf("message = %q, want %q", output.Message, "Initial commit")
	}
}

func TestParseGitCommitStats(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		wantFiles      int
		wantInsertions int
		wantDeletions  int
	}{
		{
			name:           "typical_commit",
			output:         " 2 files changed, 15 insertions(+), 3 deletions(-)\n",
			wantFiles:      2,
			wantInsertions: 15,
			wantDeletions:  3,
		},
		{
			name:           "insertions_only",
			output:         " 1 file changed, 10 insertions(+)\n",
			wantFiles:      1,
			wantInsertions: 10,
			wantDeletions:  0,
		},
		{
			name:           "deletions_only",
			output:         " 3 files changed, 5 deletions(-)\n",
			wantFiles:      3,
			wantInsertions: 0,
			wantDeletions:  5,
		},
		{
			name:           "no_stats",
			output:         "nothing to commit\n",
			wantFiles:      0,
			wantInsertions: 0,
			wantDeletions:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, insertions, deletions := parseGitCommitStats(tt.output)
			if files != tt.wantFiles {
				t.Errorf("files = %d, want %d", files, tt.wantFiles)
			}
			if insertions != tt.wantInsertions {
				t.Errorf("insertions = %d, want %d", insertions, tt.wantInsertions)
			}
			if deletions != tt.wantDeletions {
				t.Errorf("deletions = %d, want %d", deletions, tt.wantDeletions)
			}
		})
	}
}

func TestToolFactory_WithWorkspaceManager(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-wm-tools")
	defer os.RemoveAll(basePath)

	wm := NewWorkspaceManager(WithBasePath(basePath))
	mock := &mockBackend{}
	factory := NewToolFactory(mock, WithWorkspaceManager(wm))

	if factory.workspaceManager != wm {
		t.Error("expected workspace manager to be set")
	}
}

func TestToolFactory_CreateOpenTool_ManagedWorkspace(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-open-managed")
	defer os.RemoveAll(basePath)

	wm := NewWorkspaceManager(WithBasePath(basePath))
	mock := &mockBackend{}
	factory := NewToolFactory(mock, WithWorkspaceManager(wm))
	tool := factory.CreateOpenTool()

	t.Run("creates_managed_workspace_when_path_omitted", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"title": "managed session",
		}

		result, err := tool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map output, got %T", result)
		}

		if output["managed"] != true {
			t.Error("expected managed=true for auto-created workspace")
		}
		if output["workspace_id"] == "" || output["workspace_id"] == nil {
			t.Error("expected non-empty workspace_id for managed workspace")
		}
		if output["workspace_path"] == "" || output["workspace_path"] == nil {
			t.Error("expected non-empty workspace_path")
		}

		wsPath := output["workspace_path"].(string)
		gitDir := filepath.Join(wsPath, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			t.Error("expected .git directory in managed workspace")
		}
	})

	t.Run("uses_workflow_scope", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"scope":    "workflow",
			"scope_id": "workflow-123",
		}

		result, err := tool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := result.(map[string]any)
		wsID := output["workspace_id"].(string)

		ws, err := wm.Get(wsID)
		if err != nil {
			t.Fatalf("workspace not found: %v", err)
		}
		if ws.Scope != ScopeWorkflow {
			t.Errorf("expected workflow scope, got %s", ws.Scope)
		}
		if ws.WorkflowID != "workflow-123" {
			t.Errorf("expected workflow ID workflow-123, got %s", ws.WorkflowID)
		}
	})

	t.Run("uses_explicit_path_when_provided", func(t *testing.T) {
		explicitPath := filepath.Join(basePath, "explicit-workspace")
		os.MkdirAll(explicitPath, 0755)

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"workspace_path": explicitPath,
		}

		result, err := tool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := result.(map[string]any)
		if output["managed"] == true {
			t.Error("expected managed=false when explicit path provided")
		}
		if output["workspace_id"] != "" && output["workspace_id"] != nil {
			t.Error("expected empty workspace_id for non-managed workspace")
		}
		if output["workspace_path"] != explicitPath {
			t.Errorf("expected workspace_path=%s, got %v", explicitPath, output["workspace_path"])
		}
	})
}

func TestToolFactory_CreateOpenTool_NoWorkspaceManager(t *testing.T) {
	mock := &mockBackend{}
	factory := NewToolFactory(mock)
	tool := factory.CreateOpenTool()

	t.Run("requires_path_without_workspace_manager", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{}

		_, err := tool.RunRaw(toolCtx, input)
		if err == nil {
			t.Error("expected error when workspace_path omitted and no WorkspaceManager")
		}
		if !errors.Is(err, nil) && err.Error() != "coding_open: workspace_path is required (no WorkspaceManager configured)" {
			t.Logf("got expected error type: %v", err)
		}
	})
}

func TestToolFactory_CreateCloseTool_WithWorkspace(t *testing.T) {
	basePath := filepath.Join(os.TempDir(), "station-test-close-managed")
	defer os.RemoveAll(basePath)

	wm := NewWorkspaceManager(WithBasePath(basePath), WithCleanupPolicy(CleanupOnSuccess))
	mock := &mockBackend{}
	factory := NewToolFactory(mock, WithWorkspaceManager(wm))

	ctx := context.Background()
	ws, _ := wm.Create(ctx, ScopeAgent, "test-session")
	wm.InitGit(ctx, ws)

	testFile := filepath.Join(ws.Path, "changed.txt")
	os.WriteFile(testFile, []byte("content\n"), 0644)

	closeTool := factory.CreateCloseTool()

	t.Run("collects_changes_and_cleans_on_success", func(t *testing.T) {
		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id":   "test-session",
			"workspace_id": ws.ID,
			"success":      true,
		}

		result, err := closeTool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := result.(map[string]any)
		if output["ok"] != true {
			t.Error("expected ok=true")
		}

		filesChanged, ok := output["files_changed"].([]FileChange)
		if !ok {
			t.Logf("files_changed type: %T", output["files_changed"])
		}
		if len(filesChanged) == 0 && output["files_changed"] != nil {
			rawChanges, _ := output["files_changed"].([]any)
			if len(rawChanges) == 0 {
				t.Error("expected files_changed to contain changed.txt")
			}
		}

		if output["cleaned_up"] != true {
			t.Error("expected cleaned_up=true for CleanupOnSuccess with success=true")
		}

		if _, err := os.Stat(ws.Path); !os.IsNotExist(err) {
			t.Error("expected workspace to be cleaned up")
		}
	})

	t.Run("preserves_workspace_on_failure", func(t *testing.T) {
		ws2, _ := wm.Create(ctx, ScopeAgent, "test-session-2")
		os.WriteFile(filepath.Join(ws2.Path, "file.txt"), []byte("data"), 0644)

		toolCtx := &ai.ToolContext{Context: context.Background()}
		input := map[string]any{
			"session_id":   "test-session-2",
			"workspace_id": ws2.ID,
			"success":      false,
		}

		result, err := closeTool.RunRaw(toolCtx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := result.(map[string]any)
		if output["cleaned_up"] == true {
			t.Error("expected cleaned_up=false for CleanupOnSuccess with success=false")
		}

		if _, err := os.Stat(ws2.Path); os.IsNotExist(err) {
			t.Error("expected workspace to be preserved on failure")
		}
	})
}
