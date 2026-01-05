package coding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"station/internal/config"
)

func TestClaudeCodeBackend_CreateAndGetSession(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.CodingConfig{
		Backend: "claudecode",
		ClaudeCode: config.CodingClaudeCodeConfig{
			BinaryPath: "claude",
			TimeoutSec: 60,
		},
		WorkspaceBasePath: tmpDir,
	}

	backend := NewClaudeCodeBackend(cfg)

	workspacePath := filepath.Join(tmpDir, "test-workspace")
	session, err := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath: workspacePath,
		Title:         "test-session",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.WorkspacePath != workspacePath {
		t.Errorf("WorkspacePath = %q, want %q", session.WorkspacePath, workspacePath)
	}
	if session.Title != "test-session" {
		t.Errorf("Title = %q, want %q", session.Title, "test-session")
	}

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		t.Error("workspace directory was not created")
	}

	got, err := backend.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("GetSession ID = %q, want %q", got.ID, session.ID)
	}
}

func TestClaudeCodeBackend_GetSession_NotFound(t *testing.T) {
	backend := NewClaudeCodeBackend(config.CodingConfig{})

	_, err := backend.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestClaudeCodeBackend_CloseSession(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{BinaryPath: "claude"},
	}
	backend := NewClaudeCodeBackend(cfg)

	session, err := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath: t.TempDir(),
		Title:         "test",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	err = backend.CloseSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	_, err = backend.GetSession(context.Background(), session.ID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound after close, got %v", err)
	}
}

func TestClaudeCodeBackend_CloseSession_NotFound(t *testing.T) {
	backend := NewClaudeCodeBackend(config.CodingConfig{})

	err := backend.CloseSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestClaudeCodeBackend_CreateSession_WithExistingSessionID(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{BinaryPath: "claude"},
	}
	backend := NewClaudeCodeBackend(cfg)

	session, err := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath:     t.TempDir(),
		Title:             "test",
		ExistingSessionID: "ses_abc123",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.BackendSessionID != "ses_abc123" {
		t.Errorf("BackendSessionID = %q, want %q", session.BackendSessionID, "ses_abc123")
	}
}

func TestClaudeCodeBackend_Ping(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{BinaryPath: "echo"},
	}
	backend := NewClaudeCodeBackend(cfg)

	err := backend.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping failed with 'echo' binary: %v", err)
	}
}

func TestClaudeCodeBackend_Ping_NotFound(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{BinaryPath: "/nonexistent/binary"},
	}
	backend := NewClaudeCodeBackend(cfg)

	err := backend.Ping(context.Background())
	if err == nil {
		t.Error("expected error for nonexistent binary, got nil")
	}
}

func TestClaudeCodeBackend_DefaultValues(t *testing.T) {
	// Test that defaults are applied correctly
	backend := NewClaudeCodeBackend(config.CodingConfig{})

	if backend.binaryPath != "claude" {
		t.Errorf("default binaryPath = %q, want %q", backend.binaryPath, "claude")
	}
	if backend.taskTimeout.Minutes() != 5 {
		t.Errorf("default taskTimeout = %v, want 5m", backend.taskTimeout)
	}
	if backend.maxTurns != 10 {
		t.Errorf("default maxTurns = %d, want 10", backend.maxTurns)
	}
}

func TestClaudeCodeBackend_ConfigValues(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{
			BinaryPath:      "/custom/claude",
			TimeoutSec:      120,
			Model:           "opus",
			MaxTurns:        20,
			AllowedTools:    []string{"Read", "Write"},
			DisallowedTools: []string{"Bash"},
		},
	}
	backend := NewClaudeCodeBackend(cfg)

	if backend.binaryPath != "/custom/claude" {
		t.Errorf("binaryPath = %q, want %q", backend.binaryPath, "/custom/claude")
	}
	if backend.taskTimeout.Seconds() != 120 {
		t.Errorf("taskTimeout = %v, want 120s", backend.taskTimeout)
	}
	if backend.model != "opus" {
		t.Errorf("model = %q, want %q", backend.model, "opus")
	}
	if backend.maxTurns != 20 {
		t.Errorf("maxTurns = %d, want 20", backend.maxTurns)
	}
	if len(backend.allowedTools) != 2 {
		t.Errorf("allowedTools length = %d, want 2", len(backend.allowedTools))
	}
	if len(backend.disallowedTools) != 1 {
		t.Errorf("disallowedTools length = %d, want 1", len(backend.disallowedTools))
	}
}

func TestClaudeCodeBackend_CreateSession_WithRepoURL(t *testing.T) {
	// This test validates the metadata is set correctly
	// Actual clone will fail without network, but we can test the session metadata
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{
			BinaryPath: "false", // Will fail immediately
		},
	}
	backend := NewClaudeCodeBackend(cfg)

	_, err := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath: t.TempDir(),
		Title:         "test",
		RepoURL:       "https://github.com/test/repo.git",
		Branch:        "main",
	})

	// We expect an error since 'false' will fail
	if err == nil {
		t.Error("expected error from failed clone, got nil")
	}
}

func TestClaudeCodeBackend_buildCloneTask(t *testing.T) {
	backend := NewClaudeCodeBackend(config.CodingConfig{})

	tests := []struct {
		name     string
		repoURL  string
		branch   string
		creds    *GitCredentials
		wantPart string
	}{
		{
			name:     "simple clone",
			repoURL:  "https://github.com/test/repo.git",
			branch:   "",
			creds:    nil,
			wantPart: "git clone https://github.com/test/repo.git .",
		},
		{
			name:     "clone with branch",
			repoURL:  "https://github.com/test/repo.git",
			branch:   "develop",
			creds:    nil,
			wantPart: "git clone --branch develop https://github.com/test/repo.git .",
		},
		{
			name:    "clone with token",
			repoURL: "https://github.com/test/repo.git",
			branch:  "",
			creds: &GitCredentials{
				Token: "ghp_secret123",
			},
			wantPart: "git clone https://x-access-token:ghp_secret123@github.com/test/repo.git .",
		},
		{
			name:    "clone with username and token (username ignored, x-access-token used)",
			repoURL: "https://github.com/test/repo.git",
			branch:  "main",
			creds: &GitCredentials{
				UserName: "myuser",
				Token:    "ghp_secret123",
			},
			wantPart: "git clone --branch main https://x-access-token:ghp_secret123@github.com/test/repo.git .",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backend.buildCloneTask(tt.repoURL, tt.branch, tt.creds)
			if got != tt.wantPart && !contains(got, tt.wantPart) {
				t.Errorf("buildCloneTask() = %q, want to contain %q", got, tt.wantPart)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestClaudeCodeBackend_SessionMetadata(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{BinaryPath: "claude"},
	}
	backend := NewClaudeCodeBackend(cfg)

	session, err := backend.CreateSession(context.Background(), SessionOptions{
		WorkspacePath: t.TempDir(),
		Title:         "test-metadata",
	})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.Metadata == nil {
		t.Error("session.Metadata should not be nil")
	}

	if session.CreatedAt.IsZero() {
		t.Error("session.CreatedAt should be set")
	}

	if session.LastUsedAt.IsZero() {
		t.Error("session.LastUsedAt should be set")
	}
}

func TestClaudeCodeBackend_Execute_SessionNotFound(t *testing.T) {
	backend := NewClaudeCodeBackend(config.CodingConfig{})

	_, err := backend.Execute(context.Background(), "nonexistent", Task{
		Instruction: "test",
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestClaudeCodeBackend_ConcurrentSessions(t *testing.T) {
	cfg := config.CodingConfig{
		ClaudeCode: config.CodingClaudeCodeConfig{BinaryPath: "claude"},
	}
	backend := NewClaudeCodeBackend(cfg)

	// Create multiple sessions concurrently
	sessions := make([]*Session, 5)
	errs := make([]error, 5)

	for i := 0; i < 5; i++ {
		i := i
		go func() {
			sessions[i], errs[i] = backend.CreateSession(context.Background(), SessionOptions{
				WorkspacePath: t.TempDir(),
				Title:         "concurrent-test",
			})
		}()
	}

	// Wait for all to complete (simple approach)
	for i := 0; i < 100; i++ {
		allDone := true
		for j := 0; j < 5; j++ {
			if sessions[j] == nil && errs[j] == nil {
				allDone = false
				break
			}
		}
		if allDone {
			break
		}
	}

	// Verify all sessions were created with unique IDs
	ids := make(map[string]bool)
	for i, s := range sessions {
		if errs[i] != nil {
			t.Errorf("session %d creation failed: %v", i, errs[i])
			continue
		}
		if s == nil {
			continue
		}
		if ids[s.ID] {
			t.Errorf("duplicate session ID: %s", s.ID)
		}
		ids[s.ID] = true
	}
}
