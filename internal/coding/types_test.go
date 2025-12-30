package coding

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTokenUsage_Total(t *testing.T) {
	tests := []struct {
		name     string
		usage    TokenUsage
		expected int
	}{
		{
			name:     "all zeros",
			usage:    TokenUsage{},
			expected: 0,
		},
		{
			name:     "input only",
			usage:    TokenUsage{Input: 100},
			expected: 100,
		},
		{
			name:     "input and output",
			usage:    TokenUsage{Input: 100, Output: 50},
			expected: 150,
		},
		{
			name:     "with reasoning",
			usage:    TokenUsage{Input: 100, Output: 50, Reasoning: 25},
			expected: 175,
		},
		{
			name:     "cache fields not included in total",
			usage:    TokenUsage{Input: 100, Output: 50, CacheRead: 1000, CacheWrite: 500},
			expected: 150,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.usage.Total(); got != tt.expected {
				t.Errorf("Total() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSession_JSONRoundTrip(t *testing.T) {
	session := Session{
		ID:               "test-123",
		BackendSessionID: "oc-abc",
		WorkspacePath:    "/workspaces/my-repo",
		Title:            "Fix auth bug",
		CreatedAt:        time.Now().Truncate(time.Second),
		LastUsedAt:       time.Now().Truncate(time.Second),
		Metadata:         map[string]string{"workflow_id": "wf-1"},
	}

	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Session
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != session.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, session.ID)
	}
	if decoded.BackendSessionID != session.BackendSessionID {
		t.Errorf("BackendSessionID = %q, want %q", decoded.BackendSessionID, session.BackendSessionID)
	}
	if decoded.WorkspacePath != session.WorkspacePath {
		t.Errorf("WorkspacePath = %q, want %q", decoded.WorkspacePath, session.WorkspacePath)
	}
}

func TestResult_JSONRoundTrip(t *testing.T) {
	result := Result{
		Success: true,
		Summary: "Fixed the null pointer exception in auth.go",
		FilesChanged: []FileChange{
			{Path: "auth.go", Action: "modified", LinesAdded: 5, LinesRemoved: 2},
			{Path: "auth_test.go", Action: "created", LinesAdded: 20},
		},
		Trace: &Trace{
			Model:    "claude-sonnet-4-20250514",
			Provider: "anthropic",
			Cost:     0.015,
			Tokens:   TokenUsage{Input: 1000, Output: 500},
			Duration: 5 * time.Second,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Result
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Success != result.Success {
		t.Errorf("Success = %v, want %v", decoded.Success, result.Success)
	}
	if len(decoded.FilesChanged) != len(result.FilesChanged) {
		t.Errorf("FilesChanged len = %d, want %d", len(decoded.FilesChanged), len(result.FilesChanged))
	}
	if decoded.Trace == nil {
		t.Error("Trace is nil, expected non-nil")
	} else if decoded.Trace.Model != result.Trace.Model {
		t.Errorf("Trace.Model = %q, want %q", decoded.Trace.Model, result.Trace.Model)
	}
}

func TestTask_Defaults(t *testing.T) {
	task := Task{
		Instruction: "Fix the bug",
	}

	if task.Context != "" {
		t.Errorf("Context = %q, want empty", task.Context)
	}
	if task.Files != nil {
		t.Errorf("Files = %v, want nil", task.Files)
	}
	if task.Timeout != 0 {
		t.Errorf("Timeout = %v, want 0", task.Timeout)
	}
}
