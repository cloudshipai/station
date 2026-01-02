package coding

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"station/internal/config"
)

func TestNATSCodingClient_SubjectDefaults(t *testing.T) {
	cfg := config.CodingNATSConfig{}
	c := &NATSCodingClient{cfg: cfg}

	t.Run("taskSubject default", func(t *testing.T) {
		got := c.taskSubject()
		if got != DefaultTaskSubject {
			t.Errorf("taskSubject() = %q, want %q", got, DefaultTaskSubject)
		}
	})

	t.Run("streamSubject default", func(t *testing.T) {
		got := c.streamSubject("task-123")
		expected := DefaultStreamSubject + ".task-123"
		if got != expected {
			t.Errorf("streamSubject() = %q, want %q", got, expected)
		}
	})

	t.Run("resultSubject default", func(t *testing.T) {
		got := c.resultSubject("task-123")
		expected := DefaultResultSubject + ".task-123"
		if got != expected {
			t.Errorf("resultSubject() = %q, want %q", got, expected)
		}
	})

	t.Run("sessionsBucket default", func(t *testing.T) {
		got := c.sessionsBucket()
		if got != DefaultSessionsBucket {
			t.Errorf("sessionsBucket() = %q, want %q", got, DefaultSessionsBucket)
		}
	})

	t.Run("stateBucket default", func(t *testing.T) {
		got := c.stateBucket()
		if got != DefaultStateBucket {
			t.Errorf("stateBucket() = %q, want %q", got, DefaultStateBucket)
		}
	})
}

func TestNATSCodingClient_SubjectCustom(t *testing.T) {
	cfg := config.CodingNATSConfig{
		Subjects: config.CodingNATSSubjects{
			Task:   "custom.task",
			Stream: "custom.stream",
			Result: "custom.result",
		},
		KV: config.CodingNATSKV{
			Sessions: "custom-sessions",
			State:    "custom-state",
		},
	}
	c := &NATSCodingClient{cfg: cfg}

	t.Run("taskSubject custom", func(t *testing.T) {
		got := c.taskSubject()
		if got != "custom.task" {
			t.Errorf("taskSubject() = %q, want %q", got, "custom.task")
		}
	})

	t.Run("streamSubject custom", func(t *testing.T) {
		got := c.streamSubject("task-123")
		if got != "custom.stream.task-123" {
			t.Errorf("streamSubject() = %q, want %q", got, "custom.stream.task-123")
		}
	})

	t.Run("resultSubject custom", func(t *testing.T) {
		got := c.resultSubject("task-123")
		if got != "custom.result.task-123" {
			t.Errorf("resultSubject() = %q, want %q", got, "custom.result.task-123")
		}
	})

	t.Run("sessionsBucket custom", func(t *testing.T) {
		got := c.sessionsBucket()
		if got != "custom-sessions" {
			t.Errorf("sessionsBucket() = %q, want %q", got, "custom-sessions")
		}
	})

	t.Run("stateBucket custom", func(t *testing.T) {
		got := c.stateBucket()
		if got != "custom-state" {
			t.Errorf("stateBucket() = %q, want %q", got, "custom-state")
		}
	})
}

func TestNATSCodingClient_IsConnected(t *testing.T) {
	t.Run("nil connection", func(t *testing.T) {
		c := &NATSCodingClient{}
		if c.IsConnected() {
			t.Error("IsConnected() = true, want false for nil connection")
		}
	})
}

func TestCodingTask_JSON(t *testing.T) {
	task := &CodingTask{
		TaskID: "task-123",
		Session: TaskSession{
			Name:     "session-1",
			Continue: true,
		},
		Workspace: TaskWorkspace{
			Name: "/workspaces/test",
			Git: &TaskGitConfig{
				URL:    "https://github.com/test/repo.git",
				Branch: "main",
				Pull:   true,
			},
		},
		Prompt:  "Fix the authentication bug",
		Agent:   "coder",
		Timeout: 300000,
		Callback: TaskCallback{
			StreamSubject: "station.coding.stream.task-123",
			ResultSubject: "station.coding.result.task-123",
		},
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded CodingTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.TaskID != task.TaskID {
		t.Errorf("TaskID = %q, want %q", decoded.TaskID, task.TaskID)
	}
	if decoded.Session.Name != task.Session.Name {
		t.Errorf("Session.Name = %q, want %q", decoded.Session.Name, task.Session.Name)
	}
	if decoded.Workspace.Git == nil {
		t.Fatal("Workspace.Git = nil, want non-nil")
	}
	if decoded.Workspace.Git.URL != task.Workspace.Git.URL {
		t.Errorf("Workspace.Git.URL = %q, want %q", decoded.Workspace.Git.URL, task.Workspace.Git.URL)
	}
}

func TestCodingStreamEvent_JSON(t *testing.T) {
	event := &CodingStreamEvent{
		TaskID:    "task-123",
		Seq:       1,
		Timestamp: "2025-01-01T12:00:00Z",
		Type:      "tool_start",
		Tool: &StreamEventTool{
			Name:   "bash",
			CallID: "call-1",
			Args:   map[string]interface{}{"command": "git status"},
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded CodingStreamEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.TaskID != event.TaskID {
		t.Errorf("TaskID = %q, want %q", decoded.TaskID, event.TaskID)
	}
	if decoded.Type != event.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, event.Type)
	}
	if decoded.Tool == nil {
		t.Fatal("Tool = nil, want non-nil")
	}
	if decoded.Tool.Name != "bash" {
		t.Errorf("Tool.Name = %q, want %q", decoded.Tool.Name, "bash")
	}
}

func TestCodingResult_JSON(t *testing.T) {
	result := &CodingResult{
		TaskID: "task-123",
		Status: "completed",
		Result: "Successfully fixed the authentication bug",
		Session: ResultSession{
			Name:         "session-1",
			OpencodeID:   "oc-123",
			MessageCount: 5,
		},
		Workspace: ResultWorkspace{
			Name: "workspace-1",
			Path: "/workspaces/test",
			Git: &ResultWorkspaceGit{
				Branch: "main",
				Commit: "abc123",
				Dirty:  false,
			},
		},
		Metrics: ResultMetrics{
			Duration:         15000,
			PromptTokens:     500,
			CompletionTokens: 200,
			ToolCalls:        3,
			StreamEvents:     10,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded CodingResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Status != "completed" {
		t.Errorf("Status = %q, want %q", decoded.Status, "completed")
	}
	if decoded.Session.OpencodeID != "oc-123" {
		t.Errorf("Session.OpencodeID = %q, want %q", decoded.Session.OpencodeID, "oc-123")
	}
	if decoded.Metrics.ToolCalls != 3 {
		t.Errorf("Metrics.ToolCalls = %d, want %d", decoded.Metrics.ToolCalls, 3)
	}
}

func TestSessionState_JSON(t *testing.T) {
	state := &SessionState{
		SessionName:   "session-1",
		OpencodeID:    "oc-123",
		WorkspaceName: "workspace-1",
		WorkspacePath: "/workspaces/test",
		Created:       "2025-01-01T10:00:00Z",
		LastUsed:      "2025-01-01T12:00:00Z",
		MessageCount:  10,
		Git: &SessionStateGit{
			URL:        "https://github.com/test/repo.git",
			Branch:     "main",
			LastCommit: "abc123",
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded SessionState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.SessionName != state.SessionName {
		t.Errorf("SessionName = %q, want %q", decoded.SessionName, state.SessionName)
	}
	if decoded.Git == nil {
		t.Fatal("Git = nil, want non-nil")
	}
	if decoded.Git.LastCommit != "abc123" {
		t.Errorf("Git.LastCommit = %q, want %q", decoded.Git.LastCommit, "abc123")
	}
}

func TestTaskExecution_Channels(t *testing.T) {
	exec := &TaskExecution{
		events: make(chan *CodingStreamEvent, 10),
		result: make(chan *CodingResult, 1),
		done:   make(chan struct{}),
	}

	event := &CodingStreamEvent{TaskID: "task-1", Type: "text"}
	exec.events <- event

	select {
	case got := <-exec.Events():
		if got.TaskID != "task-1" {
			t.Errorf("event TaskID = %q, want %q", got.TaskID, "task-1")
		}
	default:
		t.Error("expected event on Events channel")
	}

	result := &CodingResult{TaskID: "task-1", Status: "completed"}
	exec.result <- result

	select {
	case got := <-exec.Result():
		if got.Status != "completed" {
			t.Errorf("result Status = %q, want %q", got.Status, "completed")
		}
	default:
		t.Error("expected result on Result channel")
	}
}

func TestTaskExecution_Cancel(t *testing.T) {
	exec := &TaskExecution{
		events: make(chan *CodingStreamEvent, 10),
		result: make(chan *CodingResult, 1),
		done:   make(chan struct{}),
	}

	exec.Cancel()

	select {
	case <-exec.Done():
	default:
		t.Error("expected Done channel to be closed after Cancel")
	}

	exec.Cancel()
}

func TestTaskExecution_Wait_Success(t *testing.T) {
	exec := &TaskExecution{
		events: make(chan *CodingStreamEvent, 10),
		result: make(chan *CodingResult, 1),
		done:   make(chan struct{}),
	}

	go func() {
		exec.result <- &CodingResult{TaskID: "task-1", Status: "completed"}
		close(exec.done)
	}()

	ctx := context.Background()
	result, err := exec.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("Status = %q, want %q", result.Status, "completed")
	}
}

func TestTaskExecution_Wait_ContextCancel(t *testing.T) {
	exec := &TaskExecution{
		events: make(chan *CodingStreamEvent, 10),
		result: make(chan *CodingResult, 1),
		done:   make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := exec.Wait(ctx)
	if err == nil {
		t.Error("Wait() error = nil, want context deadline error")
	}
}

func TestDefaultConstants(t *testing.T) {
	if DefaultNATSURL != "nats://localhost:4222" {
		t.Errorf("DefaultNATSURL = %q, want %q", DefaultNATSURL, "nats://localhost:4222")
	}
	if DefaultTaskSubject != "station.coding.task" {
		t.Errorf("DefaultTaskSubject = %q, want %q", DefaultTaskSubject, "station.coding.task")
	}
	if DefaultStreamSubject != "station.coding.stream" {
		t.Errorf("DefaultStreamSubject = %q, want %q", DefaultStreamSubject, "station.coding.stream")
	}
	if DefaultResultSubject != "station.coding.result" {
		t.Errorf("DefaultResultSubject = %q, want %q", DefaultResultSubject, "station.coding.result")
	}
	if DefaultSessionsBucket != "opencode-sessions" {
		t.Errorf("DefaultSessionsBucket = %q, want %q", DefaultSessionsBucket, "opencode-sessions")
	}
	if DefaultStateBucket != "opencode-state" {
		t.Errorf("DefaultStateBucket = %q, want %q", DefaultStateBucket, "opencode-state")
	}
	if DefaultKVTTL != 24*time.Hour*7 {
		t.Errorf("DefaultKVTTL = %v, want %v", DefaultKVTTL, 24*time.Hour*7)
	}
}
