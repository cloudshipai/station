package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"station/internal/workflows"
)

func TestCronExecutor_SupportedTypes(t *testing.T) {
	executor := NewCronExecutor()
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}
	if types[0] != workflows.StepTypeCron {
		t.Errorf("expected StepTypeCron, got %v", types[0])
	}
}

func TestCronExecutor_Execute(t *testing.T) {
	executor := NewCronExecutor()

	tests := []struct {
		name        string
		step        workflows.ExecutionStep
		runContext  map[string]interface{}
		checkResult func(t *testing.T, result StepResult, ctx map[string]interface{})
	}{
		{
			name: "injects input into context",
			step: workflows.ExecutionStep{
				ID:   "cron-step",
				Type: workflows.StepTypeCron,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type: "cron",
					Cron: "0 9 * * *",
					Input: map[string]interface{}{
						"namespace": "production",
						"services":  []string{"api", "web"},
					},
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusCompleted {
					t.Errorf("expected completed status, got %v", result.Status)
				}
				if ctx["namespace"] != "production" {
					t.Errorf("expected namespace=production, got %v", ctx["namespace"])
				}
				if ctx["_cronTriggeredAt"] == nil {
					t.Error("expected _cronTriggeredAt to be set")
				}
				if ctx["_cronExpression"] != "0 9 * * *" {
					t.Errorf("expected _cronExpression='0 9 * * *', got %v", ctx["_cronExpression"])
				}
				if result.NextStep != "next-step" {
					t.Errorf("expected next-step, got %s", result.NextStep)
				}
			},
		},
		{
			name: "works with empty input",
			step: workflows.ExecutionStep{
				ID:   "cron-empty",
				Type: workflows.StepTypeCron,
				End:  true,
				Raw: workflows.StateSpec{
					Type: "cron",
					Cron: "*/15 * * * *",
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusCompleted {
					t.Errorf("expected completed status, got %v", result.Status)
				}
				if !result.End {
					t.Error("expected end=true")
				}
			},
		},
		{
			name: "sets timezone in context",
			step: workflows.ExecutionStep{
				ID:   "cron-tz",
				Type: workflows.StepTypeCron,
				Next: "next",
				Raw: workflows.StateSpec{
					Type:     "cron",
					Cron:     "0 9 * * *",
					Timezone: "America/Chicago",
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if ctx["_cronTimezone"] != "America/Chicago" {
					t.Errorf("expected timezone=America/Chicago, got %v", ctx["_cronTimezone"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tt.step, tt.runContext)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			tt.checkResult(t, result, tt.runContext)
		})
	}
}

func TestTimerExecutor_SupportedTypes(t *testing.T) {
	executor := NewTimerExecutor()
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}
	if types[0] != workflows.StepTypeTimer {
		t.Errorf("expected StepTypeTimer, got %v", types[0])
	}
}

func TestTimerExecutor_Execute(t *testing.T) {
	executor := NewTimerExecutor()

	tests := []struct {
		name        string
		step        workflows.ExecutionStep
		runContext  map[string]interface{}
		wantErr     bool
		errType     error
		checkResult func(t *testing.T, result StepResult, ctx map[string]interface{})
	}{
		{
			name: "valid duration from Duration field",
			step: workflows.ExecutionStep{
				ID:   "timer-step",
				Type: workflows.StepTypeTimer,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type:     "timer",
					Duration: "5m",
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusWaitingTimer {
					t.Errorf("expected waiting_timer status, got %v", result.Status)
				}
				if ctx["_timerDuration"] != "5m" {
					t.Errorf("expected _timerDuration='5m', got %v", ctx["_timerDuration"])
				}
				if ctx["_timerResumeAt"] == nil {
					t.Error("expected _timerResumeAt to be set")
				}
			},
		},
		{
			name: "valid duration from input",
			step: workflows.ExecutionStep{
				ID:   "timer-input",
				Type: workflows.StepTypeTimer,
				Next: "next",
				Raw: workflows.StateSpec{
					Type: "timer",
					Input: map[string]interface{}{
						"duration": "30s",
					},
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusWaitingTimer {
					t.Errorf("expected waiting_timer status, got %v", result.Status)
				}
				if ctx["_timerDuration"] != "30s" {
					t.Errorf("expected _timerDuration='30s', got %v", ctx["_timerDuration"])
				}
			},
		},
		{
			name: "missing duration",
			step: workflows.ExecutionStep{
				ID:   "timer-missing",
				Type: workflows.StepTypeTimer,
				Raw: workflows.StateSpec{
					Type: "timer",
				},
			},
			runContext: map[string]interface{}{},
			wantErr:    true,
			errType:    ErrInvalidDuration,
		},
		{
			name: "invalid duration format",
			step: workflows.ExecutionStep{
				ID:   "timer-invalid",
				Type: workflows.StepTypeTimer,
				Raw: workflows.StateSpec{
					Type:     "timer",
					Duration: "5 minutes",
				},
			},
			runContext: map[string]interface{}{},
			wantErr:    true,
			errType:    ErrInvalidDuration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tt.step, tt.runContext)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if !errors.Is(err, tt.errType) {
					t.Errorf("expected error %v, got %v", tt.errType, err)
				}
				if result.Status != StepStatusFailed {
					t.Errorf("expected failed status, got %v", result.Status)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			tt.checkResult(t, result, tt.runContext)
		})
	}
}

func TestTimerExecutor_CheckTimerComplete(t *testing.T) {
	executor := NewTimerExecutor()

	t.Run("timer not complete", func(t *testing.T) {
		futureTime := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
		ctx := map[string]interface{}{
			"_timerResumeAt": futureTime,
		}

		complete, err := executor.CheckTimerComplete(context.Background(), ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if complete {
			t.Error("expected timer to not be complete")
		}
	})

	t.Run("timer complete", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
		ctx := map[string]interface{}{
			"_timerResumeAt": pastTime,
		}

		complete, err := executor.CheckTimerComplete(context.Background(), ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !complete {
			t.Error("expected timer to be complete")
		}
	})

	t.Run("no timer set", func(t *testing.T) {
		ctx := map[string]interface{}{}

		complete, err := executor.CheckTimerComplete(context.Background(), ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if complete {
			t.Error("expected timer to not be complete when not set")
		}
	})
}

func TestTryCatchExecutor_SupportedTypes(t *testing.T) {
	registry := NewExecutorRegistry()
	executor := NewTryCatchExecutor(registry)
	types := executor.SupportedTypes()

	if len(types) != 1 {
		t.Errorf("expected 1 supported type, got %d", len(types))
	}
	if types[0] != workflows.StepTypeTryCatch {
		t.Errorf("expected StepTypeTryCatch, got %v", types[0])
	}
}

func TestTryCatchExecutor_Execute(t *testing.T) {
	registry := NewExecutorRegistry()
	registry.Register(NewInjectExecutor())

	executor := NewTryCatchExecutor(registry)

	tests := []struct {
		name        string
		step        workflows.ExecutionStep
		runContext  map[string]interface{}
		checkResult func(t *testing.T, result StepResult, ctx map[string]interface{})
	}{
		{
			name: "try block succeeds",
			step: workflows.ExecutionStep{
				ID:   "try-success",
				Type: workflows.StepTypeTryCatch,
				Next: "next-step",
				Raw: workflows.StateSpec{
					Type: "try",
					Try: &workflows.IteratorSpec{
						Start: "inject-data",
						States: []workflows.StateSpec{
							{
								Name: "inject-data",
								Type: "inject",
								Data: map[string]interface{}{
									"result": "success",
								},
								End: true,
							},
						},
					},
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusCompleted {
					t.Errorf("expected completed status, got %v", result.Status)
				}
				if result.Output["block"] != "try" {
					t.Errorf("expected block=try, got %v", result.Output["block"])
				}
				if result.NextStep != "next-step" {
					t.Errorf("expected next-step, got %s", result.NextStep)
				}
			},
		},
		{
			name: "empty try block",
			step: workflows.ExecutionStep{
				ID:   "try-empty",
				Type: workflows.StepTypeTryCatch,
				Next: "next",
				Raw: workflows.StateSpec{
					Type: "try",
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusCompleted {
					t.Errorf("expected completed status, got %v", result.Status)
				}
				if result.Output["skipped"] != true {
					t.Error("expected skipped=true for empty try block")
				}
			},
		},
		{
			name: "finally block runs after success",
			step: workflows.ExecutionStep{
				ID:   "try-finally",
				Type: workflows.StepTypeTryCatch,
				Next: "next",
				Raw: workflows.StateSpec{
					Type: "try",
					Try: &workflows.IteratorSpec{
						States: []workflows.StateSpec{
							{
								Name: "step1",
								Type: "inject",
								Data: map[string]interface{}{"try_ran": true},
								End:  true,
							},
						},
					},
					Finally: &workflows.IteratorSpec{
						States: []workflows.StateSpec{
							{
								Name: "cleanup",
								Type: "inject",
								Data: map[string]interface{}{"cleaned_up": true},
								End:  true,
							},
						},
					},
				},
			},
			runContext: map[string]interface{}{},
			checkResult: func(t *testing.T, result StepResult, ctx map[string]interface{}) {
				if result.Status != StepStatusCompleted {
					t.Errorf("expected completed status, got %v", result.Status)
				}
				if result.Output["finally_output"] == nil {
					t.Error("expected finally_output to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(context.Background(), tt.step, tt.runContext)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			tt.checkResult(t, result, tt.runContext)
		})
	}
}
