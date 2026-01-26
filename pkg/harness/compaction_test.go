package harness

import (
	"context"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/ai"
)

// mockHistoryBackend implements HistoryBackend for testing
type mockHistoryBackend struct {
	files map[string][]byte
}

func newMockHistoryBackend() *mockHistoryBackend {
	return &mockHistoryBackend{
		files: make(map[string][]byte),
	}
}

func (m *mockHistoryBackend) WriteFile(path string, content []byte) error {
	m.files[path] = content
	return nil
}

func (m *mockHistoryBackend) ReadFile(path string) ([]byte, error) {
	return m.files[path], nil
}

func TestCompactor_TruncateOldArgs(t *testing.T) {
	compactor := NewCompactor(nil, "", CompactionConfig{Enabled: true}, 100000)

	// Create messages with large tool arguments
	messages := []*ai.Message{
		ai.NewUserTextMessage("Hello"),
		{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewToolRequestPart(&ai.ToolRequest{
					Name: "write_file",
					Input: map[string]any{
						"path":    "/some/path.txt",
						"content": "This is a very long content string that should be truncated when it exceeds the maximum allowed length for old tool arguments. " + string(make([]byte, 1000)),
					},
				}),
			},
		},
		ai.NewUserTextMessage("Continue"),
		ai.NewModelTextMessage("Done"),
		ai.NewUserTextMessage("More"),
		ai.NewModelTextMessage("Finished"),
	}

	// Truncate with max length of 100
	truncated := compactor.TruncateOldArgs(messages, 100)

	// Last 4 messages should be unchanged
	if len(truncated) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(truncated))
	}

	// Check that the old write_file tool call was truncated
	if len(truncated[1].Content) > 0 && truncated[1].Content[0].IsToolRequest() {
		input := truncated[1].Content[0].ToolRequest.Input.(map[string]any)
		content := input["content"].(string)
		if len(content) > 200 { // 100 chars + truncation message
			t.Errorf("Content should be truncated, got length %d", len(content))
		}
	}
}

func TestCompactor_ShouldTruncateArgs(t *testing.T) {
	tests := []struct {
		toolName string
		expected bool
	}{
		{"write_file", true},
		{"edit_file", true},
		{"bash", true},
		{"read_file", false},
		{"list_directory", false},
		{"__write_file", true},
		{"unknown_tool", false},
	}

	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			result := shouldTruncateArgs(tt.toolName)
			if result != tt.expected {
				t.Errorf("shouldTruncateArgs(%q) = %v, want %v", tt.toolName, result, tt.expected)
			}
		})
	}
}

func TestCompactor_TruncateInput(t *testing.T) {
	t.Run("truncate long string", func(t *testing.T) {
		input := map[string]any{
			"content": "This is a very long string that will be truncated",
		}
		result := truncateInput(input, 10)
		resultMap := result.(map[string]any)
		content := resultMap["content"].(string)

		// Should start with first 10 chars and contain truncation message
		if !strings.Contains(content, "This is a ") {
			t.Errorf("Truncated content should start with original prefix, got: %s", content)
		}
		if !strings.Contains(content, "[truncated") {
			t.Errorf("Truncated content should contain truncation message, got: %s", content)
		}
	})

	t.Run("short string unchanged", func(t *testing.T) {
		input := map[string]any{
			"content": "short",
		}
		result := truncateInput(input, 100)
		resultMap := result.(map[string]any)
		content := resultMap["content"].(string)

		if content != "short" {
			t.Errorf("Short content should be unchanged, got: %s", content)
		}
	})

	t.Run("nil input", func(t *testing.T) {
		result := truncateInput(nil, 100)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})
}

func TestCompactor_WithHistoryBackend(t *testing.T) {
	backend := newMockHistoryBackend()
	compactor := NewCompactor(nil, "", CompactionConfig{
		Enabled:        true,
		HistoryOffload: true,
	}, 100000)

	compactor.WithHistoryBackend(backend, "/history", "session-123")

	if compactor.historyBackend != backend {
		t.Error("historyBackend not set correctly")
	}
	if compactor.historyPrefix != "/history" {
		t.Errorf("historyPrefix = %q, want /history", compactor.historyPrefix)
	}
	if compactor.sessionID != "session-123" {
		t.Errorf("sessionID = %q, want session-123", compactor.sessionID)
	}
}

func TestCompactor_OffloadHistory(t *testing.T) {
	backend := newMockHistoryBackend()
	compactor := NewCompactor(nil, "", CompactionConfig{
		Enabled:        true,
		HistoryOffload: true,
	}, 100000)
	compactor.WithHistoryBackend(backend, "/history", "test-session")

	messages := []*ai.Message{
		ai.NewUserTextMessage("Hello world"),
		ai.NewModelTextMessage("Hi there!"),
		{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewToolRequestPart(&ai.ToolRequest{
					Name:  "read_file",
					Input: map[string]any{"path": "/test.txt"},
				}),
			},
		},
	}

	filename, err := compactor.offloadHistory(messages)
	if err != nil {
		t.Fatalf("offloadHistory() error = %v", err)
	}

	if filename == "" {
		t.Error("Expected non-empty filename")
	}

	// Check that file was written
	if len(backend.files) == 0 {
		t.Error("No files written to backend")
	}

	// Verify file content contains expected data
	for _, content := range backend.files {
		if len(content) == 0 {
			t.Error("Empty file content")
		}
		// Basic JSON validation
		if content[0] != '{' {
			t.Error("Expected JSON content starting with {")
		}
	}
}

func TestCompactor_OffloadHistory_NoBackend(t *testing.T) {
	compactor := NewCompactor(nil, "", CompactionConfig{
		Enabled:        true,
		HistoryOffload: true,
	}, 100000)

	messages := []*ai.Message{
		ai.NewUserTextMessage("Hello"),
	}

	filename, err := compactor.offloadHistory(messages)
	if err != nil {
		t.Errorf("offloadHistory() error = %v", err)
	}
	if filename != "" {
		t.Errorf("Expected empty filename when no backend, got %q", filename)
	}
}

func TestCompactor_CompactIfNeeded_Disabled(t *testing.T) {
	compactor := NewCompactor(nil, "", CompactionConfig{Enabled: false}, 100000)

	messages := []*ai.Message{
		ai.NewUserTextMessage("Hello"),
	}

	result, compacted, err := compactor.CompactIfNeeded(context.Background(), messages)
	if err != nil {
		t.Fatalf("CompactIfNeeded() error = %v", err)
	}
	if compacted {
		t.Error("Should not compact when disabled")
	}
	if len(result) != len(messages) {
		t.Error("Messages should be unchanged when disabled")
	}
}

func TestCompactor_ShouldCompact(t *testing.T) {
	compactor := NewCompactor(nil, "", CompactionConfig{
		Enabled:   true,
		Threshold: 0.5,
	}, 1000) // Small context window for testing

	tests := []struct {
		name       string
		msgCount   int
		wantCompact bool
	}{
		{"small history", 2, false},
		{"large history", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := make([]*ai.Message, tt.msgCount)
			for i := 0; i < tt.msgCount; i++ {
				messages[i] = ai.NewUserTextMessage("This is message number " + string(rune('A'+i%26)) + " with some content")
			}

			should, _, err := compactor.ShouldCompact(messages)
			if err != nil {
				t.Fatalf("ShouldCompact() error = %v", err)
			}
			if should != tt.wantCompact {
				t.Errorf("ShouldCompact() = %v, want %v", should, tt.wantCompact)
			}
		})
	}
}

func TestSimpleTokenCounter(t *testing.T) {
	counter := &SimpleTokenCounter{}

	messages := []*ai.Message{
		ai.NewUserTextMessage("Hello world this is a test message"),
		ai.NewModelTextMessage("This is the response with more words"),
	}

	count, err := counter.CountTokens(messages)
	if err != nil {
		t.Fatalf("CountTokens() error = %v", err)
	}

	// Simple token counter: words * 4 / 3
	// "Hello world this is a test message" = 7 words
	// "This is the response with more words" = 7 words
	// 14 words * 4 / 3 = 18 tokens (approx)
	if count < 10 || count > 30 {
		t.Errorf("CountTokens() = %d, expected between 10-30", count)
	}
}
