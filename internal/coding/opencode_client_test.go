package coding

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenCodeClient_Health(t *testing.T) {
	t.Run("healthy server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/global/health" {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
		}))
		defer server.Close()

		client := NewOpenCodeClient(server.URL)
		err := client.Health(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unhealthy server", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := NewOpenCodeClient(server.URL, WithMaxAttempts(1))
		err := client.Health(context.Background())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestOpenCodeClient_CreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/session" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		dir := r.URL.Query().Get("directory")
		if dir != "/workspaces/test-repo" {
			t.Errorf("unexpected directory: %s", dir)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"id": "session-123"})
	}))
	defer server.Close()

	client := NewOpenCodeClient(server.URL)
	sessionID, err := client.CreateSession(context.Background(), "/workspaces/test-repo", "test session")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionID != "session-123" {
		t.Errorf("sessionID = %q, want %q", sessionID, "session-123")
	}
}

func TestOpenCodeClient_SendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/session/sess-1/message" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		dir := r.URL.Query().Get("directory")
		if dir != "/workspaces/test-repo" {
			t.Errorf("unexpected directory: %s", dir)
		}

		resp := sendMessageResponse{
			Info: messageInfo{
				ID:         "msg-1",
				ModelID:    "claude-sonnet-4-20250514",
				ProviderID: "anthropic",
				Finish:     "stop",
				Cost:       0.015,
			},
			Parts: []messagePart{
				{Type: "text", Text: "Fixed the bug in auth.go"},
			},
		}
		resp.Info.Tokens.Input = 1000
		resp.Info.Tokens.Output = 500
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenCodeClient(server.URL)
	result, err := client.SendMessage(context.Background(), "sess-1", "/workspaces/test-repo", "fix the bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Model != "claude-sonnet-4-20250514" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-sonnet-4-20250514")
	}
	if result.Text != "Fixed the bug in auth.go" {
		t.Errorf("Text = %q, want %q", result.Text, "Fixed the bug in auth.go")
	}
	if result.Tokens.Input != 1000 {
		t.Errorf("Tokens.Input = %d, want %d", result.Tokens.Input, 1000)
	}
}

func TestOpenCodeClient_RetryOnServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]bool{"healthy": true})
	}))
	defer server.Close()

	client := NewOpenCodeClient(server.URL,
		WithMaxAttempts(3),
		WithRetryDelay(10*time.Millisecond, 100*time.Millisecond, 2.0),
	)

	err := client.Health(context.Background())
	if err != nil {
		t.Errorf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestOpenCodeClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewOpenCodeClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := client.Health(ctx)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestOpenCodeClient_Options(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	client := NewOpenCodeClient("http://localhost:4096",
		WithMaxAttempts(5),
		WithRetryDelay(2*time.Second, 60*time.Second, 3.0),
		WithHTTPClient(customClient),
	)

	if client.maxAttempts != 5 {
		t.Errorf("maxAttempts = %d, want 5", client.maxAttempts)
	}
	if client.retryDelay != 2*time.Second {
		t.Errorf("retryDelay = %v, want 2s", client.retryDelay)
	}
	if client.maxDelay != 60*time.Second {
		t.Errorf("maxDelay = %v, want 60s", client.maxDelay)
	}
	if client.multiplier != 3.0 {
		t.Errorf("multiplier = %v, want 3.0", client.multiplier)
	}
	if client.httpClient != customClient {
		t.Error("httpClient not set correctly")
	}
}

func TestOpenCodeClient_ParseToolCalls(t *testing.T) {
	t.Run("single tool call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"info": map[string]interface{}{
					"id":         "msg-1",
					"modelID":    "claude-opus-4-5",
					"providerID": "anthropic",
					"finish":     "stop",
					"cost":       0.025,
					"tokens":     map[string]int{"input": 100, "output": 50},
					"time":       map[string]int64{"created": 1234567890, "completed": 1234567900},
				},
				"parts": []map[string]interface{}{
					{"type": "tool-invocation", "tool": "bash", "input": map[string]interface{}{"command": "git status"}},
					{"type": "tool-result", "output": "On branch main\nnothing to commit"},
					{"type": "text", "text": "The git status shows a clean working tree."},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewOpenCodeClient(server.URL)
		result, err := client.SendMessage(context.Background(), "sess-1", "/workspaces/test", "check git")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.ToolCalls) != 1 {
			t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
		}
		if result.ToolCalls[0].Tool != "bash" {
			t.Errorf("tool = %q, want bash", result.ToolCalls[0].Tool)
		}
		if result.ToolCalls[0].Output != "On branch main\nnothing to commit" {
			t.Errorf("unexpected output: %q", result.ToolCalls[0].Output)
		}
		if result.Text != "The git status shows a clean working tree." {
			t.Errorf("unexpected text: %q", result.Text)
		}
	})

	t.Run("multiple tool calls", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"info": map[string]interface{}{
					"id":         "msg-2",
					"modelID":    "claude-opus-4-5",
					"providerID": "anthropic",
					"finish":     "stop",
					"cost":       0.05,
					"tokens":     map[string]int{"input": 200, "output": 100},
					"time":       map[string]int64{"created": 1234567890, "completed": 1234567920},
				},
				"parts": []map[string]interface{}{
					{"type": "tool-invocation", "tool": "read", "input": map[string]interface{}{"path": "main.go"}},
					{"type": "tool-result", "output": "package main\n\nfunc main() {}"},
					{"type": "tool-invocation", "tool": "write", "input": map[string]interface{}{"path": "main.go", "content": "new content"}},
					{"type": "tool-result", "output": "File written successfully"},
					{"type": "text", "text": "I've updated main.go."},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewOpenCodeClient(server.URL)
		result, err := client.SendMessage(context.Background(), "sess-1", "/workspaces/test", "update main.go")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.ToolCalls) != 2 {
			t.Fatalf("expected 2 tool calls, got %d", len(result.ToolCalls))
		}
		if result.ToolCalls[0].Tool != "read" {
			t.Errorf("first tool = %q, want read", result.ToolCalls[0].Tool)
		}
		if result.ToolCalls[1].Tool != "write" {
			t.Errorf("second tool = %q, want write", result.ToolCalls[1].Tool)
		}
	})

	t.Run("with reasoning", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"info": map[string]interface{}{
					"id":         "msg-3",
					"modelID":    "claude-opus-4-5",
					"providerID": "anthropic",
					"finish":     "stop",
					"cost":       0.03,
					"tokens":     map[string]int{"input": 150, "output": 75, "reasoning": 200},
					"time":       map[string]int64{"created": 1234567890, "completed": 1234567910},
				},
				"parts": []map[string]interface{}{
					{"type": "reasoning", "text": "I need to check the file first."},
					{"type": "reasoning", "text": "Then I'll make the necessary changes."},
					{"type": "tool-invocation", "tool": "bash", "input": map[string]interface{}{"command": "ls"}},
					{"type": "tool-result", "output": "file1.go file2.go"},
					{"type": "text", "text": "Done."},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewOpenCodeClient(server.URL)
		result, err := client.SendMessage(context.Background(), "sess-1", "/workspaces/test", "list files")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Reasoning) != 2 {
			t.Fatalf("expected 2 reasoning entries, got %d", len(result.Reasoning))
		}
		if result.Reasoning[0] != "I need to check the file first." {
			t.Errorf("unexpected reasoning[0]: %q", result.Reasoning[0])
		}
		if result.Tokens.Reasoning != 200 {
			t.Errorf("Tokens.Reasoning = %d, want 200", result.Tokens.Reasoning)
		}
	})

	t.Run("no tool calls", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]interface{}{
				"info": map[string]interface{}{
					"id":         "msg-4",
					"modelID":    "claude-opus-4-5",
					"providerID": "anthropic",
					"finish":     "stop",
					"cost":       0.01,
					"tokens":     map[string]int{"input": 50, "output": 25},
					"time":       map[string]int64{"created": 1234567890, "completed": 1234567895},
				},
				"parts": []map[string]interface{}{
					{"type": "text", "text": "I don't need any tools for this."},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client := NewOpenCodeClient(server.URL)
		result, err := client.SendMessage(context.Background(), "sess-1", "/workspaces/test", "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.ToolCalls) != 0 {
			t.Errorf("expected 0 tool calls, got %d", len(result.ToolCalls))
		}
		if result.Text != "I don't need any tools for this." {
			t.Errorf("unexpected text: %q", result.Text)
		}
	})
}
