package coding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"station/internal/config"
)

func TestCLIBackend_CreateAndGetSession(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := config.CodingConfig{
		Backend: "opencode-cli",
		CLI: config.CodingCLIConfig{
			BinaryPath: "opencode",
			TimeoutSec: 60,
		},
		WorkspaceBasePath: tmpDir,
	}

	backend := NewCLIBackend(cfg)

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

func TestCLIBackend_GetSession_NotFound(t *testing.T) {
	backend := NewCLIBackend(config.CodingConfig{})

	_, err := backend.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestCLIBackend_CloseSession(t *testing.T) {
	cfg := config.CodingConfig{
		CLI: config.CodingCLIConfig{BinaryPath: "opencode"},
	}
	backend := NewCLIBackend(cfg)

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

func TestCLIBackend_CloseSession_NotFound(t *testing.T) {
	backend := NewCLIBackend(config.CodingConfig{})

	err := backend.CloseSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestCLIBackend_CreateSession_WithExistingSessionID(t *testing.T) {
	cfg := config.CodingConfig{
		CLI: config.CodingCLIConfig{BinaryPath: "opencode"},
	}
	backend := NewCLIBackend(cfg)

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

func TestCLIBackend_Ping(t *testing.T) {
	cfg := config.CodingConfig{
		CLI: config.CodingCLIConfig{BinaryPath: "echo"},
	}
	backend := NewCLIBackend(cfg)

	err := backend.Ping(context.Background())
	if err != nil {
		t.Errorf("Ping failed with 'echo' binary: %v", err)
	}
}

func TestCLIBackend_Ping_NotFound(t *testing.T) {
	cfg := config.CodingConfig{
		CLI: config.CodingCLIConfig{BinaryPath: "/nonexistent/binary"},
	}
	backend := NewCLIBackend(cfg)

	err := backend.Ping(context.Background())
	if err == nil {
		t.Error("expected error for nonexistent binary, got nil")
	}
}
