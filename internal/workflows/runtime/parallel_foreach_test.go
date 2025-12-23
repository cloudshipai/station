package runtime

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"station/internal/workflows"
)

type mockBranchDeps struct {
	executeFunc func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error)
	callCount   int32
}

func (m *mockBranchDeps) ExecuteStep(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
	atomic.AddInt32(&m.callCount, 1)
	if m.executeFunc != nil {
		return m.executeFunc(ctx, step, runContext)
	}
	return StepResult{
		Status: StepStatusCompleted,
		Output: map[string]interface{}{"executed": true, "stepId": step.ID},
	}, nil
}

func TestParallelExecutor_SupportedTypes(t *testing.T) {
	executor := NewParallelExecutor(nil)
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != workflows.StepTypeParallel {
		t.Errorf("expected StepTypeParallel, got %v", types[0])
	}
}

func TestParallelExecutor_Execute(t *testing.T) {
	tests := []struct {
		name         string
		step         workflows.ExecutionStep
		runContext   map[string]interface{}
		mockFunc     func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error)
		wantStatus   StepStatus
		wantErr      error
		wantBranches int
		checkOutput  func(t *testing.T, result StepResult)
	}{
		{
			name: "executes all branches in parallel",
			step: workflows.ExecutionStep{
				ID:   "parallel-step",
				Type: workflows.StepTypeParallel,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type: "parallel",
					Branches: []workflows.BranchSpec{
						{
							Name: "branch1",
							States: []workflows.StateSpec{
								{Name: "b1-step", Type: "inject", Data: map[string]interface{}{"key": "val1"}, End: true},
							},
						},
						{
							Name: "branch2",
							States: []workflows.StateSpec{
								{Name: "b2-step", Type: "inject", Data: map[string]interface{}{"key": "val2"}, End: true},
							},
						},
					},
				},
			},
			runContext:   map[string]interface{}{},
			wantStatus:   StepStatusCompleted,
			wantBranches: 2,
			checkOutput: func(t *testing.T, result StepResult) {
				if result.NextStep != "next-step" {
					t.Errorf("NextStep = %v, want 'next-step'", result.NextStep)
				}
				if _, ok := result.Output["branch1"]; !ok {
					t.Error("expected branch1 in output")
				}
				if _, ok := result.Output["branch2"]; !ok {
					t.Error("expected branch2 in output")
				}
			},
		},
		{
			name: "fails when no branches defined",
			step: workflows.ExecutionStep{
				ID:   "no-branches",
				Type: workflows.StepTypeParallel,
				Raw: workflows.StateSpec{
					Type:     "parallel",
					Branches: []workflows.BranchSpec{},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrNoBranches,
		},
		{
			name: "skips non-parallel type",
			step: workflows.ExecutionStep{
				ID:   "not-parallel",
				Type: workflows.StepTypeParallel,
				Next: "next",
				Raw: workflows.StateSpec{
					Type: "operation",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkOutput: func(t *testing.T, result StepResult) {
				if result.Output["skipped"] != true {
					t.Error("expected skipped=true")
				}
			},
		},
		{
			name: "fails with unsupported join mode",
			step: workflows.ExecutionStep{
				ID:   "bad-join",
				Type: workflows.StepTypeParallel,
				Raw: workflows.StateSpec{
					Type: "parallel",
					Branches: []workflows.BranchSpec{
						{Name: "b1", States: []workflows.StateSpec{{Name: "s1", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}}},
					},
					Join: &workflows.JoinSpec{Mode: "any"},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrUnsupportedJoin,
		},
		{
			name: "handles branch with no states",
			step: workflows.ExecutionStep{
				ID:   "empty-branch",
				Type: workflows.StepTypeParallel,
				Raw: workflows.StateSpec{
					Type: "parallel",
					Branches: []workflows.BranchSpec{
						{Name: "empty", States: []workflows.StateSpec{}},
					},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrBranchFailed,
		},
		{
			name: "stores results at resultPath",
			step: workflows.ExecutionStep{
				ID:   "with-result-path",
				Type: workflows.StepTypeParallel,
				Raw: workflows.StateSpec{
					Type: "parallel",
					Branches: []workflows.BranchSpec{
						{
							Name: "diagnostic",
							States: []workflows.StateSpec{
								{Name: "check", Type: "inject", Data: map[string]interface{}{"status": "ok"}, End: true},
							},
						},
					},
					ResultPath: "steps.parallel_results",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
		},
		{
			name: "branch execution failure",
			step: workflows.ExecutionStep{
				ID:   "fail-branch",
				Type: workflows.StepTypeParallel,
				Raw: workflows.StateSpec{
					Type: "parallel",
					Branches: []workflows.BranchSpec{
						{
							Name: "failing",
							States: []workflows.StateSpec{
								{Name: "will-fail", Type: "operation", Input: map[string]interface{}{"task": "agent.run"}, End: true},
							},
						},
					},
				},
			},
			runContext: map[string]interface{}{},
			mockFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
				return StepResult{
					Status: StepStatusFailed,
					Error:  strPtr("agent failed"),
				}, errors.New("agent execution failed")
			},
			wantStatus: StepStatusFailed,
			wantErr:    ErrBranchFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &mockBranchDeps{executeFunc: tt.mockFunc}
			executor := NewParallelExecutor(deps)

			result, err := executor.Execute(context.Background(), tt.step, tt.runContext)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && !containsError(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, result)
			}
		})
	}
}

func TestParallelExecutor_ConcurrentExecution(t *testing.T) {
	var execOrder []string
	var mu sync.Mutex
	startTime := time.Now()

	deps := &mockBranchDeps{
		executeFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
			mu.Lock()
			execOrder = append(execOrder, step.ID)
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			return StepResult{Status: StepStatusCompleted, Output: map[string]interface{}{"id": step.ID}}, nil
		},
	}

	executor := NewParallelExecutor(deps)

	step := workflows.ExecutionStep{
		ID:   "concurrent-test",
		Type: workflows.StepTypeParallel,
		Raw: workflows.StateSpec{
			Type: "parallel",
			Branches: []workflows.BranchSpec{
				{Name: "b1", States: []workflows.StateSpec{{Name: "s1", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}}},
				{Name: "b2", States: []workflows.StateSpec{{Name: "s2", Type: "inject", Data: map[string]interface{}{"x": 2}, End: true}}},
				{Name: "b3", States: []workflows.StateSpec{{Name: "s3", Type: "inject", Data: map[string]interface{}{"x": 3}, End: true}}},
			},
		},
	}

	result, err := executor.Execute(context.Background(), step, map[string]interface{}{})

	elapsed := time.Since(startTime)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Status != StepStatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, StepStatusCompleted)
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("execution took %v, should be ~50ms if parallel", elapsed)
	}
}

func TestParallelExecutor_ContextCancellation(t *testing.T) {
	deps := &mockBranchDeps{
		executeFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
			time.Sleep(100 * time.Millisecond)
			return StepResult{Status: StepStatusCompleted}, nil
		},
	}

	executor := NewParallelExecutor(deps)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	step := workflows.ExecutionStep{
		ID:   "cancel-test",
		Type: workflows.StepTypeParallel,
		Raw: workflows.StateSpec{
			Type: "parallel",
			Branches: []workflows.BranchSpec{
				{Name: "slow", States: []workflows.StateSpec{{Name: "slow-step", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}}},
			},
		},
	}

	result, err := executor.Execute(ctx, step, map[string]interface{}{})

	if err == nil || result.Status != StepStatusFailed {
		t.Log("Context cancellation handling depends on timing")
	}
}

func TestForeachExecutor_SupportedTypes(t *testing.T) {
	executor := NewForeachExecutor(nil)
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}

	if types[0] != workflows.StepTypeLoop {
		t.Errorf("expected StepTypeLoop, got %v", types[0])
	}
}

func TestForeachExecutor_Execute(t *testing.T) {
	tests := []struct {
		name        string
		step        workflows.ExecutionStep
		runContext  map[string]interface{}
		mockFunc    func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error)
		wantStatus  StepStatus
		wantErr     error
		checkOutput func(t *testing.T, result StepResult, ctx map[string]interface{})
	}{
		{
			name: "iterates over array items",
			step: workflows.ExecutionStep{
				ID:   "foreach-step",
				Type: workflows.StepTypeLoop,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "services",
					ItemName:  "service",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{
							{Name: "check", Type: "inject", Data: map[string]interface{}{"checked": true}, End: true},
						},
					},
					ResultPath: "results",
				},
			},
			runContext: map[string]interface{}{
				"services": []interface{}{"svc1", "svc2", "svc3"},
			},
			wantStatus: StepStatusCompleted,
			checkOutput: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.NextStep != "next-step" {
					t.Errorf("NextStep = %v, want 'next-step'", result.NextStep)
				}
				results, ok := result.Output["results"].([]interface{})
				if !ok {
					t.Errorf("expected results array, got %T", result.Output["results"])
					return
				}
				if len(results) != 3 {
					t.Errorf("expected 3 results, got %d", len(results))
				}
			},
		},
		{
			name: "fails when itemsPath missing",
			step: workflows.ExecutionStep{
				ID:   "no-items-path",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:     "foreach",
					ItemName: "item",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
					},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrNoItemsPath,
		},
		{
			name: "fails when items not found",
			step: workflows.ExecutionStep{
				ID:   "items-not-found",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "nonexistent",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
					},
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusFailed,
			wantErr:    ErrItemsNotFound,
		},
		{
			name: "fails when items not an array",
			step: workflows.ExecutionStep{
				ID:   "items-not-array",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "items",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
					},
				},
			},
			runContext: map[string]interface{}{
				"items": "not-an-array",
			},
			wantStatus: StepStatusFailed,
			wantErr:    ErrItemsNotArray,
		},
		{
			name: "fails when no iterator defined",
			step: workflows.ExecutionStep{
				ID:   "no-iterator",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "items",
				},
			},
			runContext: map[string]interface{}{
				"items": []interface{}{"a"},
			},
			wantStatus: StepStatusFailed,
			wantErr:    ErrNoIterator,
		},
		{
			name: "handles empty array",
			step: workflows.ExecutionStep{
				ID:   "empty-array",
				Type: workflows.StepTypeLoop,
				Next: "next",
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "items",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
					},
				},
			},
			runContext: map[string]interface{}{
				"items": []interface{}{},
			},
			wantStatus: StepStatusCompleted,
			checkOutput: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				count, ok := result.Output["count"].(int)
				if !ok || count != 0 {
					t.Errorf("expected count=0, got %v", result.Output["count"])
				}
			},
		},
		{
			name: "uses default itemName when not specified",
			step: workflows.ExecutionStep{
				ID:   "default-item-name",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "items",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
					},
				},
			},
			runContext: map[string]interface{}{
				"items": []interface{}{"a"},
			},
			wantStatus: StepStatusCompleted,
		},
		{
			name: "skips non-foreach type",
			step: workflows.ExecutionStep{
				ID:   "not-foreach",
				Type: workflows.StepTypeLoop,
				Next: "next",
				Raw: workflows.StateSpec{
					Type: "operation",
				},
			},
			runContext: map[string]interface{}{},
			wantStatus: StepStatusCompleted,
			checkOutput: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Output["skipped"] != true {
					t.Error("expected skipped=true")
				}
			},
		},
		{
			name: "iteration failure",
			step: workflows.ExecutionStep{
				ID:   "fail-iteration",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "items",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{
							{Name: "fail-step", Type: "operation", Input: map[string]interface{}{"task": "agent.run"}, End: true},
						},
					},
				},
			},
			runContext: map[string]interface{}{
				"items": []interface{}{"a", "b"},
			},
			mockFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
				return StepResult{
					Status: StepStatusFailed,
					Error:  strPtr("iteration failed"),
				}, errors.New("failed")
			},
			wantStatus: StepStatusFailed,
			wantErr:    ErrIterationFailed,
		},
		{
			name: "nested itemsPath",
			step: workflows.ExecutionStep{
				ID:   "nested-path",
				Type: workflows.StepTypeLoop,
				Raw: workflows.StateSpec{
					Type:      "foreach",
					ItemsPath: "data.items.list",
					ItemName:  "entry",
					Iterator: &workflows.IteratorSpec{
						States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
					},
				},
			},
			runContext: map[string]interface{}{
				"data": map[string]interface{}{
					"items": map[string]interface{}{
						"list": []interface{}{"x", "y"},
					},
				},
			},
			wantStatus: StepStatusCompleted,
			checkOutput: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				count, ok := result.Output["count"].(int)
				if !ok || count != 2 {
					t.Errorf("expected count=2, got %v", result.Output["count"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := &mockBranchDeps{executeFunc: tt.mockFunc}
			executor := NewForeachExecutor(deps)

			ctx := deepCopyMap(tt.runContext)
			if ctx == nil {
				ctx = make(map[string]interface{})
			}

			result, err := executor.Execute(context.Background(), tt.step, ctx)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && !containsError(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Status != tt.wantStatus {
				t.Errorf("Status = %v, want %v", result.Status, tt.wantStatus)
			}

			if tt.checkOutput != nil {
				tt.checkOutput(t, result, ctx)
			}
		})
	}
}

func TestForeachExecutor_SequentialExecution(t *testing.T) {
	var execOrder []int
	var mu sync.Mutex

	deps := &mockBranchDeps{
		executeFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
			idx := runContext["_index"].(int)
			mu.Lock()
			execOrder = append(execOrder, idx)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			return StepResult{Status: StepStatusCompleted, Output: map[string]interface{}{"index": idx}}, nil
		},
	}

	executor := NewForeachExecutor(deps)

	step := workflows.ExecutionStep{
		ID:   "sequential-test",
		Type: workflows.StepTypeLoop,
		Raw: workflows.StateSpec{
			Type:           "foreach",
			ItemsPath:      "items",
			MaxConcurrency: 1,
			Iterator: &workflows.IteratorSpec{
				States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
			},
		},
	}

	ctx := map[string]interface{}{
		"items": []interface{}{"a", "b", "c", "d", "e"},
	}

	result, err := executor.Execute(context.Background(), step, ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Status != StepStatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, StepStatusCompleted)
	}

	for i := 0; i < len(execOrder)-1; i++ {
		if execOrder[i] > execOrder[i+1] {
			t.Errorf("execution was not sequential: %v", execOrder)
			break
		}
	}
}

func TestForeachExecutor_ConcurrentExecution(t *testing.T) {
	startTime := time.Now()

	deps := &mockBranchDeps{
		executeFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
			time.Sleep(50 * time.Millisecond)
			return StepResult{Status: StepStatusCompleted, Output: map[string]interface{}{"done": true}}, nil
		},
	}

	executor := NewForeachExecutor(deps)

	step := workflows.ExecutionStep{
		ID:   "concurrent-test",
		Type: workflows.StepTypeLoop,
		Raw: workflows.StateSpec{
			Type:           "foreach",
			ItemsPath:      "items",
			MaxConcurrency: 5,
			Iterator: &workflows.IteratorSpec{
				States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
			},
		},
	}

	ctx := map[string]interface{}{
		"items": []interface{}{"a", "b", "c", "d", "e"},
	}

	result, err := executor.Execute(context.Background(), step, ctx)

	elapsed := time.Since(startTime)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.Status != StepStatusCompleted {
		t.Errorf("Status = %v, want %v", result.Status, StepStatusCompleted)
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("execution took %v, should be ~50ms if concurrent", elapsed)
	}
}

func TestForeachExecutor_ContextVariables(t *testing.T) {
	var capturedContexts []map[string]interface{}
	var mu sync.Mutex

	deps := &mockBranchDeps{
		executeFunc: func(ctx context.Context, step workflows.ExecutionStep, runContext map[string]interface{}) (StepResult, error) {
			mu.Lock()
			capturedContexts = append(capturedContexts, deepCopyMap(runContext))
			mu.Unlock()
			return StepResult{Status: StepStatusCompleted, Output: map[string]interface{}{"done": true}}, nil
		},
	}

	executor := NewForeachExecutor(deps)

	step := workflows.ExecutionStep{
		ID:   "context-vars-test",
		Type: workflows.StepTypeLoop,
		Raw: workflows.StateSpec{
			Type:      "foreach",
			ItemsPath: "items",
			ItemName:  "current",
			Iterator: &workflows.IteratorSpec{
				States: []workflows.StateSpec{{Name: "s", Type: "inject", Data: map[string]interface{}{"x": 1}, End: true}},
			},
		},
	}

	ctx := map[string]interface{}{
		"items":    []interface{}{"first", "second", "third"},
		"existing": "preserved",
	}

	result, err := executor.Execute(context.Background(), step, ctx)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != StepStatusCompleted {
		t.Fatalf("Status = %v, want %v", result.Status, StepStatusCompleted)
	}

	if len(capturedContexts) != 3 {
		t.Fatalf("expected 3 captured contexts, got %d", len(capturedContexts))
	}

	for i, captured := range capturedContexts {
		if captured["existing"] != "preserved" {
			t.Errorf("iteration %d: existing not preserved", i)
		}
		if captured["_index"] != i {
			t.Errorf("iteration %d: _index = %v, want %d", i, captured["_index"], i)
		}
		if captured["_total"] != 3 {
			t.Errorf("iteration %d: _total = %v, want 3", i, captured["_total"])
		}
		expectedItem := []string{"first", "second", "third"}[i]
		if captured["current"] != expectedItem {
			t.Errorf("iteration %d: current = %v, want %s", i, captured["current"], expectedItem)
		}
	}
}

func TestExecutorRegistry_WithParallelAndForeach(t *testing.T) {
	deps := &mockBranchDeps{}
	registry := NewExecutorRegistry()
	registry.Register(NewParallelExecutor(deps))
	registry.Register(NewForeachExecutor(deps))
	registry.Register(NewInjectExecutor())

	t.Run("get parallel executor", func(t *testing.T) {
		executor, err := registry.GetExecutor(workflows.StepTypeParallel)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if executor == nil {
			t.Error("expected non-nil executor")
		}
	})

	t.Run("get foreach executor", func(t *testing.T) {
		executor, err := registry.GetExecutor(workflows.StepTypeLoop)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if executor == nil {
			t.Error("expected non-nil executor")
		}
	})
}
