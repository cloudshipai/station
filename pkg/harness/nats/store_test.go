package nats

import (
	"testing"
	"time"
)

func TestDefaultStoreConfig(t *testing.T) {
	config := DefaultStoreConfig()

	if config.KVBucket != DefaultKVBucket {
		t.Errorf("expected KVBucket %q, got %q", DefaultKVBucket, config.KVBucket)
	}

	if config.ObjectBucket != DefaultObjectBucket {
		t.Errorf("expected ObjectBucket %q, got %q", DefaultObjectBucket, config.ObjectBucket)
	}

	if config.MaxFileSize != 100*1024*1024 {
		t.Errorf("expected MaxFileSize 100MB, got %d", config.MaxFileSize)
	}

	if config.TTL != DefaultStateTTL {
		t.Errorf("expected TTL %v, got %v", DefaultStateTTL, config.TTL)
	}
}

func TestRunStateKey(t *testing.T) {
	key := runStateKey("run-123")
	expected := "run.run-123.state"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestWorkflowContextKey(t *testing.T) {
	key := workflowContextKey("wf-456")
	expected := "workflow.wf-456.context"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestRunFileKey(t *testing.T) {
	key := runFileKey("run-123", "output.txt")
	expected := "run/run-123/output/output.txt"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestSharedFileKey(t *testing.T) {
	key := sharedFileKey("myfile.json")
	expected := "shared/myfile.json"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

func TestRunState(t *testing.T) {
	now := time.Now()
	state := RunState{
		RunID:     "run-123",
		AgentID:   "agent-1",
		AgentName: "coder",
		Status:    "running",
		StartedAt: now,
		Task:      "Fix the bug",
	}

	if state.RunID != "run-123" {
		t.Errorf("expected RunID %q, got %q", "run-123", state.RunID)
	}

	if state.Status != "running" {
		t.Errorf("expected Status %q, got %q", "running", state.Status)
	}
}

func TestWorkflowContext(t *testing.T) {
	wctx := WorkflowContext{
		WorkflowID:    "wf-1",
		WorkflowRunID: "wfr-123",
		StartedAt:     time.Now(),
		GitBranch:     "feature/test",
		Steps:         []WorkflowStepSummary{},
		SharedData:    map[string]interface{}{"key": "value"},
	}

	if wctx.WorkflowID != "wf-1" {
		t.Errorf("expected WorkflowID %q, got %q", "wf-1", wctx.WorkflowID)
	}

	if wctx.GitBranch != "feature/test" {
		t.Errorf("expected GitBranch %q, got %q", "feature/test", wctx.GitBranch)
	}

	if wctx.SharedData["key"] != "value" {
		t.Errorf("expected SharedData key to be 'value'")
	}
}

func TestWorkflowStepSummary(t *testing.T) {
	now := time.Now()
	step := WorkflowStepSummary{
		StepName:      "analyze",
		AgentName:     "analyzer",
		RunID:         "run-456",
		Status:        "completed",
		StartedAt:     now,
		Summary:       "Analyzed the code",
		FilesModified: []string{"main.go", "util.go"},
		Commits:       []string{"abc123"},
	}

	if step.StepName != "analyze" {
		t.Errorf("expected StepName %q, got %q", "analyze", step.StepName)
	}

	if len(step.FilesModified) != 2 {
		t.Errorf("expected 2 files modified, got %d", len(step.FilesModified))
	}

	if len(step.Commits) != 1 {
		t.Errorf("expected 1 commit, got %d", len(step.Commits))
	}
}

func TestFileMetadata(t *testing.T) {
	now := time.Now()
	meta := FileMetadata{
		Key:         "test/file.txt",
		Size:        1024,
		ContentType: "text/plain",
		Checksum:    "abc123",
		CreatedAt:   now,
		Metadata:    map[string]string{"custom": "value"},
	}

	if meta.Key != "test/file.txt" {
		t.Errorf("expected Key %q, got %q", "test/file.txt", meta.Key)
	}

	if meta.Size != 1024 {
		t.Errorf("expected Size 1024, got %d", meta.Size)
	}

	if meta.Metadata["custom"] != "value" {
		t.Errorf("expected Metadata custom to be 'value'")
	}
}

func TestPutFileOptions(t *testing.T) {
	opts := PutFileOptions{
		ContentType: "application/json",
		Description: "Test file",
		TTL:         24 * time.Hour,
		Metadata:    map[string]string{"source": "test"},
	}

	if opts.ContentType != "application/json" {
		t.Errorf("expected ContentType %q, got %q", "application/json", opts.ContentType)
	}

	if opts.TTL != 24*time.Hour {
		t.Errorf("expected TTL 24h, got %v", opts.TTL)
	}
}
