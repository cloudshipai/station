package workflows

import (
	"testing"
)

func TestGenerateStepID_Deterministic(t *testing.T) {
	ctx := StepContext{
		RunID:        "run-123",
		StateName:    "check_pods",
		ForeachIndex: -1,
	}

	id1 := GenerateStepID(ctx)
	id2 := GenerateStepID(ctx)

	if id1 != id2 {
		t.Errorf("GenerateStepID not deterministic: got %s and %s", id1, id2)
	}

	if len(id1) != 16 {
		t.Errorf("Expected 16 char hex string, got %d chars: %s", len(id1), id1)
	}
}

func TestGenerateStepID_DifferentContexts(t *testing.T) {
	base := StepContext{
		RunID:        "run-123",
		StateName:    "check_pods",
		ForeachIndex: -1,
	}

	tests := []struct {
		name string
		ctx  StepContext
	}{
		{"different_run", StepContext{RunID: "run-456", StateName: "check_pods", ForeachIndex: -1}},
		{"different_state", StepContext{RunID: "run-123", StateName: "analyze", ForeachIndex: -1}},
		{"with_branch", StepContext{RunID: "run-123", StateName: "check_pods", BranchPath: []string{"parallel_1", "branch_a"}, ForeachIndex: -1}},
		{"with_foreach", StepContext{RunID: "run-123", StateName: "check_pods", ForeachIndex: 0}},
		{"different_foreach_index", StepContext{RunID: "run-123", StateName: "check_pods", ForeachIndex: 1}},
	}

	baseID := GenerateStepID(base)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := GenerateStepID(tt.ctx)
			if id == baseID {
				t.Errorf("Expected different ID for %s, got same: %s", tt.name, id)
			}
		})
	}
}

func TestGenerateStepID_BranchPath(t *testing.T) {
	ctx1 := StepContext{
		RunID:        "run-123",
		StateName:    "check",
		BranchPath:   []string{"parallel_1", "branch_a"},
		ForeachIndex: -1,
	}

	ctx2 := StepContext{
		RunID:        "run-123",
		StateName:    "check",
		BranchPath:   []string{"parallel_1", "branch_b"},
		ForeachIndex: -1,
	}

	id1 := GenerateStepID(ctx1)
	id2 := GenerateStepID(ctx2)

	if id1 == id2 {
		t.Error("Different branch paths should produce different step IDs")
	}
}

func TestNewStepContext(t *testing.T) {
	ctx := NewStepContext("run-abc", "my_state")

	if ctx.RunID != "run-abc" {
		t.Errorf("Expected RunID 'run-abc', got %s", ctx.RunID)
	}
	if ctx.StateName != "my_state" {
		t.Errorf("Expected StateName 'my_state', got %s", ctx.StateName)
	}
	if ctx.ForeachIndex != -1 {
		t.Errorf("Expected ForeachIndex -1, got %d", ctx.ForeachIndex)
	}
	if len(ctx.BranchPath) != 0 {
		t.Errorf("Expected empty BranchPath, got %v", ctx.BranchPath)
	}
}

func TestStepContext_Fluent(t *testing.T) {
	ctx := NewStepContext("run-1", "state-1").
		WithBranchPath("p1", "b1").
		WithForeachIndex(5)

	if ctx.RunID != "run-1" {
		t.Error("RunID should be preserved")
	}
	if len(ctx.BranchPath) != 2 || ctx.BranchPath[0] != "p1" {
		t.Errorf("BranchPath not set correctly: %v", ctx.BranchPath)
	}
	if ctx.ForeachIndex != 5 {
		t.Errorf("ForeachIndex not set correctly: %d", ctx.ForeachIndex)
	}
}

func TestIdempotencyKey(t *testing.T) {
	key := IdempotencyKey("run-123", "step-abc", 2)
	expected := "run-123:step-abc:2"

	if key != expected {
		t.Errorf("Expected %s, got %s", expected, key)
	}
}

func TestParseIdempotencyKey(t *testing.T) {
	tests := []struct {
		key         string
		wantRunID   string
		wantStepID  string
		wantAttempt int64
		wantOK      bool
	}{
		{"run-123:step-abc:2", "run-123", "step-abc", 2, true},
		{"r:s:0", "r", "s", 0, true},
		{"invalid", "", "", 0, false},
		{"a:b", "", "", 0, false},
		{"a:b:notanumber", "", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			runID, stepID, attempt, ok := ParseIdempotencyKey(tt.key)

			if ok != tt.wantOK {
				t.Errorf("ParseIdempotencyKey(%s) ok = %v, want %v", tt.key, ok, tt.wantOK)
				return
			}

			if !tt.wantOK {
				return
			}

			if runID != tt.wantRunID || stepID != tt.wantStepID || attempt != tt.wantAttempt {
				t.Errorf("ParseIdempotencyKey(%s) = (%s, %s, %d), want (%s, %s, %d)",
					tt.key, runID, stepID, attempt, tt.wantRunID, tt.wantStepID, tt.wantAttempt)
			}
		})
	}
}
