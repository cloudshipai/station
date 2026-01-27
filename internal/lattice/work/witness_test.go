package work

import (
	"context"
	"testing"
	"time"
)

func TestWitnessConfig_Defaults(t *testing.T) {
	cfg := DefaultWitnessConfig()

	if cfg.CheckInterval != 30*time.Second {
		t.Errorf("expected CheckInterval 30s, got %v", cfg.CheckInterval)
	}
	if cfg.StuckThreshold != 5*time.Minute {
		t.Errorf("expected StuckThreshold 5m, got %v", cfg.StuckThreshold)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", cfg.MaxRetries)
	}
	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
}

type mockWitnessHandler struct {
	stuckCalls     []string
	escalatedCalls []string
	returnAction   WitnessAction
}

func (h *mockWitnessHandler) OnStuckWork(record *WorkRecord, retryCount int) WitnessAction {
	h.stuckCalls = append(h.stuckCalls, record.WorkID)
	return h.returnAction
}

func (h *mockWitnessHandler) OnWorkEscalated(record *WorkRecord) {
	h.escalatedCalls = append(h.escalatedCalls, record.WorkID)
}

func TestWitness_DisabledByDefault(t *testing.T) {
	cfg := WitnessConfig{Enabled: false}
	w := NewWitness(nil, cfg, nil)

	err := w.Start(context.Background())
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}

	stats := w.GetStats()
	if stats.Running {
		t.Error("witness should not be running when disabled")
	}
}

func TestWitness_StartStop(t *testing.T) {
	cfg := DefaultWitnessConfig()
	cfg.CheckInterval = 100 * time.Millisecond
	w := NewWitness(nil, cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start witness: %v", err)
	}

	stats := w.GetStats()
	if !stats.Running {
		t.Error("witness should be running after start")
	}

	w.Stop()
	stats = w.GetStats()
	if stats.Running {
		t.Error("witness should not be running after stop")
	}
}

func TestWitness_IsStuck(t *testing.T) {
	cfg := DefaultWitnessConfig()
	cfg.StuckThreshold = 1 * time.Minute
	w := NewWitness(nil, cfg, nil)

	now := time.Now()

	tests := []struct {
		name     string
		record   *WorkRecord
		expected bool
	}{
		{
			name: "assigned recently",
			record: &WorkRecord{
				Status:     StatusAssigned,
				AssignedAt: now.Add(-30 * time.Second),
			},
			expected: false,
		},
		{
			name: "assigned long ago",
			record: &WorkRecord{
				Status:     StatusAssigned,
				AssignedAt: now.Add(-2 * time.Minute),
			},
			expected: true,
		},
		{
			name: "accepted recently",
			record: &WorkRecord{
				Status:     StatusAccepted,
				AssignedAt: now.Add(-5 * time.Minute),
				AcceptedAt: now.Add(-30 * time.Second),
			},
			expected: false,
		},
		{
			name: "accepted long ago",
			record: &WorkRecord{
				Status:     StatusAccepted,
				AssignedAt: now.Add(-10 * time.Minute),
				AcceptedAt: now.Add(-2 * time.Minute),
			},
			expected: true,
		},
		{
			name: "completed work",
			record: &WorkRecord{
				Status:      StatusComplete,
				AssignedAt:  now.Add(-10 * time.Minute),
				CompletedAt: now.Add(-5 * time.Minute),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := w.isStuck(tt.record, now)
			if result != tt.expected {
				t.Errorf("expected isStuck=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDefaultWitnessHandler(t *testing.T) {
	var escalatedWork *WorkRecord
	handler := NewDefaultWitnessHandler(func(r *WorkRecord) {
		escalatedWork = r
	})

	action := handler.OnStuckWork(&WorkRecord{WorkID: "test"}, 0)
	if action != WitnessActionRetry {
		t.Errorf("expected WitnessActionRetry, got %v", action)
	}

	record := &WorkRecord{WorkID: "escalated-work"}
	handler.OnWorkEscalated(record)
	if escalatedWork == nil || escalatedWork.WorkID != "escalated-work" {
		t.Error("OnWorkEscalated callback not called correctly")
	}
}

func TestWitness_Stats(t *testing.T) {
	cfg := DefaultWitnessConfig()
	w := NewWitness(nil, cfg, nil)

	stats := w.GetStats()
	if stats.TrackedWorkItems != 0 {
		t.Errorf("expected 0 tracked items, got %d", stats.TrackedWorkItems)
	}
	if stats.StuckWorkItems != 0 {
		t.Errorf("expected 0 stuck items, got %d", stats.StuckWorkItems)
	}
}
