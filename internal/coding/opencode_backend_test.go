package coding

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"station/internal/config"
)

func TestOpenCodeBackend_CreateAndGetSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": "oc-session-1"})
	}))
	defer server.Close()

	cfg := config.CodingConfig{
		Backend: "opencode",
		OpenCode: config.CodingOpenCodeConfig{
			URL: server.URL,
		},
		MaxAttempts:    1,
		TaskTimeoutMin: 1,
	}

	backend := NewOpenCodeBackend(cfg)

	session, err := backend.CreateSession(context.Background(), "/workspaces/test", "test")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.BackendSessionID != "oc-session-1" {
		t.Errorf("BackendSessionID = %q, want %q", session.BackendSessionID, "oc-session-1")
	}
	if session.WorkspacePath != "/workspaces/test" {
		t.Errorf("WorkspacePath = %q, want %q", session.WorkspacePath, "/workspaces/test")
	}

	got, err := backend.GetSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}
	if got.ID != session.ID {
		t.Errorf("GetSession ID = %q, want %q", got.ID, session.ID)
	}
}

func TestOpenCodeBackend_GetSession_NotFound(t *testing.T) {
	backend := NewOpenCodeBackend(config.CodingConfig{})

	_, err := backend.GetSession(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestOpenCodeBackend_CloseSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": "oc-1"})
	}))
	defer server.Close()

	cfg := config.CodingConfig{
		OpenCode: config.CodingOpenCodeConfig{URL: server.URL},
	}
	backend := NewOpenCodeBackend(cfg)

	session, _ := backend.CreateSession(context.Background(), "/ws", "test")

	err := backend.CloseSession(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("CloseSession failed: %v", err)
	}

	_, err = backend.GetSession(context.Background(), session.ID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound after close, got %v", err)
	}
}

func TestOpenCodeBackend_Execute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" {
			json.NewEncoder(w).Encode(map[string]string{"id": "oc-1"})
			return
		}

		resp := sendMessageResponse{
			Info: messageInfo{
				ID:         "msg-1",
				ModelID:    "claude-sonnet-4-20250514",
				ProviderID: "anthropic",
				Finish:     "stop",
				Cost:       0.01,
			},
			Parts: []messagePart{{Type: "text", Text: "Fixed the null pointer in auth.go by adding a nil check."}},
		}
		resp.Info.Tokens.Input = 500
		resp.Info.Tokens.Output = 200
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := config.CodingConfig{
		OpenCode:       config.CodingOpenCodeConfig{URL: server.URL},
		TaskTimeoutMin: 1,
	}
	backend := NewOpenCodeBackend(cfg)

	session, _ := backend.CreateSession(context.Background(), "/ws", "test")

	result, err := backend.Execute(context.Background(), session.ID, Task{
		Instruction: "Fix the null pointer exception",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Error("expected Success = true")
	}
	if result.Trace == nil {
		t.Fatal("expected Trace to be set")
	}
	if result.Trace.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Trace.Model = %q, want %q", result.Trace.Model, "claude-sonnet-4-20250514")
	}
	if result.Trace.Tokens.Input != 500 {
		t.Errorf("Trace.Tokens.Input = %d, want %d", result.Trace.Tokens.Input, 500)
	}
}

func TestOpenCodeBackend_Execute_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/session" {
			json.NewEncoder(w).Encode(map[string]string{"id": "oc-1"})
			return
		}
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	cfg := config.CodingConfig{
		OpenCode:       config.CodingOpenCodeConfig{URL: server.URL},
		TaskTimeoutMin: 1,
	}
	backend := NewOpenCodeBackend(cfg)
	backend.taskTimeout = 100 * time.Millisecond

	session, _ := backend.CreateSession(context.Background(), "/ws", "test")

	result, err := backend.Execute(context.Background(), session.ID, Task{
		Instruction: "Do something slow",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected Success = false due to timeout")
	}
	if result.Error != "task timed out" {
		t.Errorf("Error = %q, want %q", result.Error, "task timed out")
	}
}

func TestOpenCodeBackend_Ping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
	}))
	defer server.Close()

	cfg := config.CodingConfig{
		OpenCode: config.CodingOpenCodeConfig{URL: server.URL},
	}
	backend := NewOpenCodeBackend(cfg)

	if err := backend.Ping(context.Background()); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestOpenCodeBackend_BuildPrompt(t *testing.T) {
	backend := &OpenCodeBackend{}
	workspacePath := "/test/workspace"

	tests := []struct {
		name     string
		task     Task
		expected string
	}{
		{
			name:     "instruction only",
			task:     Task{Instruction: "Fix the bug"},
			expected: "IMPORTANT: Work in directory: /test/workspace\nAll file operations must use this path.\n\nFix the bug",
		},
		{
			name:     "with context",
			task:     Task{Instruction: "Fix the bug", Context: "Users report crashes"},
			expected: "IMPORTANT: Work in directory: /test/workspace\nAll file operations must use this path.\n\nContext: Users report crashes\n\nTask: Fix the bug",
		},
		{
			name:     "with files",
			task:     Task{Instruction: "Fix the bug", Files: []string{"auth.go", "user.go"}},
			expected: "IMPORTANT: Work in directory: /test/workspace\nAll file operations must use this path.\n\nFix the bug\n\nFocus on these files: [auth.go user.go]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backend.buildPrompt(tt.task, workspacePath)
			if got != tt.expected {
				t.Errorf("buildPrompt() = %q, want %q", got, tt.expected)
			}
		})
	}
}
