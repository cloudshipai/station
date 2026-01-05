package work

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func uniqueID() string {
	return uuid.New().String()[:8]
}

func setupTestServer(t *testing.T) (*nats.Conn, nats.JetStreamContext, func()) {
	t.Helper()

	opts := natstest.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	srv := natstest.RunServer(&opts)

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("connect: %v", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		srv.Shutdown()
		t.Fatalf("jetstream: %v", err)
	}

	cleanup := func() {
		nc.Close()
		srv.Shutdown()
	}

	return nc, js, cleanup
}

func TestNewWorkStore(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	if store.kv == nil {
		t.Error("expected kv to be initialized")
	}
}

func TestNewWorkStore_NilJetStream(t *testing.T) {
	_, err := NewWorkStore(nil, DefaultWorkStoreConfig())
	if err == nil {
		t.Error("expected error for nil JetStream")
	}
}

func TestWorkStore_AssignAndGet(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	record := &WorkRecord{
		WorkID:            "work-123",
		OrchestratorRunID: "run-456",
		SourceStation:     "station-a",
		TargetStation:     "station-b",
		AgentName:         "test-agent",
		Task:              "do something",
	}

	if err := store.Assign(ctx, record); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if record.Status != StatusAssigned {
		t.Errorf("status = %q, want %q", record.Status, StatusAssigned)
	}
	if record.AssignedAt.IsZero() {
		t.Error("expected AssignedAt to be set")
	}

	got, err := store.Get(ctx, "work-123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.WorkID != "work-123" {
		t.Errorf("WorkID = %q, want %q", got.WorkID, "work-123")
	}
	if got.Task != "do something" {
		t.Errorf("Task = %q, want %q", got.Task, "do something")
	}
	if got.Status != StatusAssigned {
		t.Errorf("Status = %q, want %q", got.Status, StatusAssigned)
	}
}

func TestWorkStore_UpdateStatus(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	record := &WorkRecord{
		WorkID:        "work-update-1",
		TargetStation: "station-x",
		AgentName:     "updater",
		Task:          "update test",
	}

	if err := store.Assign(ctx, record); err != nil {
		t.Fatalf("Assign: %v", err)
	}

	if err := store.UpdateStatus(ctx, "work-update-1", StatusAccepted, nil); err != nil {
		t.Fatalf("UpdateStatus to accepted: %v", err)
	}

	got, _ := store.Get(ctx, "work-update-1")
	if got.Status != StatusAccepted {
		t.Errorf("status = %q, want %q", got.Status, StatusAccepted)
	}
	if got.AcceptedAt.IsZero() {
		t.Error("expected AcceptedAt to be set")
	}

	result := &WorkResult{
		Result:     "done successfully",
		DurationMs: 1234.5,
		ToolCalls:  3,
	}
	if err := store.UpdateStatus(ctx, "work-update-1", StatusComplete, result); err != nil {
		t.Fatalf("UpdateStatus to complete: %v", err)
	}

	got, _ = store.Get(ctx, "work-update-1")
	if got.Status != StatusComplete {
		t.Errorf("status = %q, want %q", got.Status, StatusComplete)
	}
	if got.Result != "done successfully" {
		t.Errorf("result = %q, want %q", got.Result, "done successfully")
	}
	if got.DurationMs != 1234.5 {
		t.Errorf("duration = %f, want %f", got.DurationMs, 1234.5)
	}
	if got.ToolCalls != 3 {
		t.Errorf("tool calls = %d, want %d", got.ToolCalls, 3)
	}
	if got.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}
}

func TestWorkStore_GetNotFound(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	_, err = store.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent work")
	}
}

func TestWorkStore_GetHistory(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	workID := fmt.Sprintf("work-history-%s", uniqueID())
	record := &WorkRecord{
		WorkID:    workID,
		AgentName: "history-agent",
		Task:      "history test",
	}

	_ = store.Assign(ctx, record)
	_ = store.UpdateStatus(ctx, workID, StatusAccepted, nil)
	_ = store.UpdateStatus(ctx, workID, StatusComplete, &WorkResult{Result: "done"})

	history, err := store.GetHistory(ctx, workID)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}

	if len(history) < 3 {
		t.Errorf("history length = %d, want >= 3", len(history))
	}

	last := len(history) - 1
	if history[last].Status != StatusComplete {
		t.Errorf("last status = %q, want %q", history[last].Status, StatusComplete)
	}
	if history[last-1].Status != StatusAccepted {
		t.Errorf("second to last status = %q, want %q", history[last-1].Status, StatusAccepted)
	}
	if history[last-2].Status != StatusAssigned {
		t.Errorf("third to last status = %q, want %q", history[last-2].Status, StatusAssigned)
	}
}

func TestWorkStore_GetByOrchestrator(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	runID := "orch-run-123"

	for i := 0; i < 3; i++ {
		record := &WorkRecord{
			WorkID:            "work-orch-" + string(rune('a'+i)),
			OrchestratorRunID: runID,
			AgentName:         "agent",
			Task:              "task",
		}
		_ = store.Assign(ctx, record)
	}

	records, err := store.GetByOrchestrator(ctx, runID)
	if err != nil {
		t.Fatalf("GetByOrchestrator: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("records = %d, want 3", len(records))
	}
}

func TestWorkStore_GetStationActive(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	stationID := "station-active-test"

	_ = store.Assign(ctx, &WorkRecord{
		WorkID:        "active-1",
		TargetStation: stationID,
		AgentName:     "agent1",
		Task:          "task1",
	})
	_ = store.Assign(ctx, &WorkRecord{
		WorkID:        "active-2",
		TargetStation: stationID,
		AgentName:     "agent2",
		Task:          "task2",
	})

	records, err := store.GetStationActive(ctx, stationID)
	if err != nil {
		t.Fatalf("GetStationActive: %v", err)
	}

	if len(records) != 2 {
		t.Errorf("active records = %d, want 2", len(records))
	}

	_ = store.UpdateStatus(ctx, "active-1", StatusComplete, &WorkResult{Result: "done"})

	records, _ = store.GetStationActive(ctx, stationID)
	if len(records) != 1 {
		t.Errorf("active records after complete = %d, want 1", len(records))
	}
}

func TestWorkStore_Watch(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = store.Assign(ctx, &WorkRecord{
		WorkID:    "watch-1",
		AgentName: "watcher",
		Task:      "watch test",
	})

	ch, err := store.Watch(ctx, "watch-1")
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = store.UpdateStatus(ctx, "watch-1", StatusAccepted, nil)
	}()

	select {
	case record := <-ch:
		if record == nil {
			t.Error("expected non-nil record")
			return
		}
	case <-ctx.Done():
		t.Error("watch timed out")
	}
}

func TestWorkStore_Delete(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx := context.Background()
	_ = store.Assign(ctx, &WorkRecord{
		WorkID:    "delete-1",
		AgentName: "deleter",
		Task:      "delete test",
	})

	_, err = store.Get(ctx, "delete-1")
	if err != nil {
		t.Fatalf("Get before delete: %v", err)
	}

	if err := store.Delete(ctx, "delete-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get(ctx, "delete-1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDefaultWorkStoreConfig(t *testing.T) {
	cfg := DefaultWorkStoreConfig()

	if cfg.Replicas != 1 {
		t.Errorf("replicas = %d, want 1", cfg.Replicas)
	}
	if cfg.History != 10 {
		t.Errorf("history = %d, want 10", cfg.History)
	}
	if cfg.TTL != 7*24*time.Hour {
		t.Errorf("ttl = %v, want 7 days", cfg.TTL)
	}
}
