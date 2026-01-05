package work

import (
	"context"
	"strings"
	"testing"
	"time"

	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func TestNewDashboard(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	dashboard := NewDashboard(store, "test-station-123")

	if dashboard.store != store {
		t.Error("store not set correctly")
	}
	if dashboard.stationID != "test-station-123" {
		t.Errorf("stationID = %q, want %q", dashboard.stationID, "test-station-123")
	}
	if dashboard.activeWork == nil {
		t.Error("activeWork map not initialized")
	}
	if dashboard.recentWork == nil {
		t.Error("recentWork slice not initialized")
	}
	if dashboard.maxRecent != 10 {
		t.Errorf("maxRecent = %d, want 10", dashboard.maxRecent)
	}
}

func TestDashboardModel_Init(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	dashboard := NewDashboard(store, "test-station")
	ctx := context.Background()
	model := dashboard.initialModel(ctx)

	if model.dashboard != dashboard {
		t.Error("dashboard reference not set")
	}
	if model.activeWork == nil {
		t.Error("activeWork not initialized")
	}
	if model.width != 80 {
		t.Errorf("default width = %d, want 80", model.width)
	}
	if model.height != 24 {
		t.Errorf("default height = %d, want 24", model.height)
	}
}

func TestDashboardModel_HandleWorkUpdate(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	dashboard := NewDashboard(store, "test-station")
	ctx := context.Background()
	model := dashboard.initialModel(ctx)

	// Test adding active work
	assignedRecord := &WorkRecord{
		WorkID:    "work-1",
		AgentName: "test-agent",
		Task:      "do something",
		Status:    StatusAssigned,
	}
	model.handleWorkUpdate(assignedRecord)

	if len(model.activeWork) != 1 {
		t.Errorf("activeWork count = %d, want 1", len(model.activeWork))
	}
	if model.activeWork["work-1"] == nil {
		t.Error("work-1 not in activeWork")
	}

	// Test accepted also stays in active
	acceptedRecord := &WorkRecord{
		WorkID:    "work-2",
		AgentName: "test-agent",
		Task:      "do more",
		Status:    StatusAccepted,
	}
	model.handleWorkUpdate(acceptedRecord)

	if len(model.activeWork) != 2 {
		t.Errorf("activeWork count = %d, want 2", len(model.activeWork))
	}

	// Test completion moves to recent
	completedRecord := &WorkRecord{
		WorkID:      "work-1",
		AgentName:   "test-agent",
		Task:        "do something",
		Status:      StatusComplete,
		CompletedAt: time.Now(),
	}
	model.handleWorkUpdate(completedRecord)

	if len(model.activeWork) != 1 {
		t.Errorf("activeWork after complete = %d, want 1", len(model.activeWork))
	}
	if len(model.recentWork) != 1 {
		t.Errorf("recentWork count = %d, want 1", len(model.recentWork))
	}

	// Test failed also goes to recent
	failedRecord := &WorkRecord{
		WorkID:      "work-2",
		AgentName:   "test-agent",
		Task:        "do more",
		Status:      StatusFailed,
		Error:       "something went wrong",
		CompletedAt: time.Now(),
	}
	model.handleWorkUpdate(failedRecord)

	if len(model.activeWork) != 0 {
		t.Errorf("activeWork after failed = %d, want 0", len(model.activeWork))
	}
	if len(model.recentWork) != 2 {
		t.Errorf("recentWork count = %d, want 2", len(model.recentWork))
	}

	// Verify most recent is first
	if model.recentWork[0].WorkID != "work-2" {
		t.Errorf("most recent work = %q, want work-2", model.recentWork[0].WorkID)
	}
}

func TestDashboardModel_View(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	dashboard := NewDashboard(store, "test-station-abc123")
	ctx := context.Background()
	model := dashboard.initialModel(ctx)

	// Test empty view
	view := model.View()
	if view == "" {
		t.Error("expected non-empty view")
	}

	// Check for expected elements
	expectedStrings := []string{
		"STATION LATTICE DASHBOARD",
		"test-sta", // truncated station ID
		"ACTIVE WORK",
		"RECENT COMPLETIONS",
		"No active work",
		"No recent work",
		"[q] quit",
	}

	for _, s := range expectedStrings {
		if !contains(view, s) {
			t.Errorf("view missing expected string: %q", s)
		}
	}

	// Add some work and verify it appears
	model.activeWork["work-xyz"] = &WorkRecord{
		WorkID:     "work-xyz",
		AgentName:  "my-agent",
		Task:       "important task",
		Status:     StatusAssigned,
		AssignedAt: time.Now(),
	}

	view = model.View()
	if !contains(view, "work-xyz") {
		t.Error("view missing active work ID")
	}
	if !contains(view, "my-agent") {
		t.Error("view missing agent name")
	}

	// Add recent work
	model.recentWork = append(model.recentWork, &WorkRecord{
		WorkID:      "work-done",
		AgentName:   "completed-agent",
		Task:        "finished task",
		Status:      StatusComplete,
		CompletedAt: time.Now(),
		DurationMs:  1500,
	})

	view = model.View()
	if !contains(view, "work-done") {
		t.Error("view missing completed work ID")
	}
}

func TestDashboardModel_ViewQuitting(t *testing.T) {
	_, js, cleanup := setupTestServer(t)
	defer cleanup()

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	dashboard := NewDashboard(store, "test-station")
	ctx := context.Background()
	model := dashboard.initialModel(ctx)
	model.quitting = true

	view := model.View()
	if view != "" {
		t.Errorf("quitting view = %q, want empty", view)
	}
}

func TestPrepend(t *testing.T) {
	// Test basic prepend
	slice := []*WorkRecord{
		{WorkID: "a"},
		{WorkID: "b"},
	}
	result := prepend(slice, &WorkRecord{WorkID: "c"}, 10)

	if len(result) != 3 {
		t.Errorf("len = %d, want 3", len(result))
	}
	if result[0].WorkID != "c" {
		t.Errorf("first = %q, want c", result[0].WorkID)
	}

	// Test max length
	slice = make([]*WorkRecord, 10)
	for i := range slice {
		slice[i] = &WorkRecord{WorkID: string(rune('a' + i))}
	}
	result = prepend(slice, &WorkRecord{WorkID: "new"}, 10)

	if len(result) != 10 {
		t.Errorf("len after max = %d, want 10", len(result))
	}
	if result[0].WorkID != "new" {
		t.Errorf("first after max = %q, want new", result[0].WorkID)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"ab", 5, "ab"},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
	if min(5, 5) != 5 {
		t.Error("min(5, 5) should be 5")
	}
}

func TestDashboard_WatchAllIntegration(t *testing.T) {
	opts := natstest.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	srv := natstest.RunServer(&opts)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	expectedWorkID := "e2e-watchall-" + uniqueID()

	ch, err := store.WatchAll(ctx)
	if err != nil {
		t.Fatalf("WatchAll: %v", err)
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = store.Assign(ctx, &WorkRecord{
			WorkID:    expectedWorkID,
			AgentName: "e2e-agent",
			Task:      "e2e task",
		})
	}()

	for {
		select {
		case record := <-ch:
			if record == nil {
				continue
			}
			if record.WorkID == expectedWorkID {
				if record.Status != StatusAssigned {
					t.Errorf("status = %q, want %q", record.Status, StatusAssigned)
				}
				return
			}
		case <-ctx.Done():
			t.Error("WatchAll timed out waiting for update")
			return
		}
	}
}

func TestDashboard_FullWorkflowSimulation(t *testing.T) {
	opts := natstest.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	srv := natstest.RunServer(&opts)
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	store, err := NewWorkStore(js, DefaultWorkStoreConfig())
	if err != nil {
		t.Fatalf("NewWorkStore: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dashboard := NewDashboard(store, "e2e-station")
	model := dashboard.initialModel(ctx)

	ch, err := store.WatchAll(ctx)
	if err != nil {
		t.Fatalf("WatchAll: %v", err)
	}

	workID := "e2e-lifecycle-" + uniqueID()
	agentName := "lifecycle-agent-" + uniqueID()

	_ = store.Assign(ctx, &WorkRecord{
		WorkID:    workID,
		AgentName: agentName,
		Task:      "lifecycle test",
	})

	waitForWork := func(expectedStatus string) *WorkRecord {
		for {
			select {
			case record := <-ch:
				if record == nil {
					continue
				}
				if record.WorkID == workID {
					return record
				}
			case <-ctx.Done():
				t.Fatalf("timeout waiting for status %s", expectedStatus)
				return nil
			}
		}
	}

	record := waitForWork(StatusAssigned)
	model.handleWorkUpdate(record)
	if model.activeWork[workID] == nil {
		t.Error("after assign: work should be in activeWork")
	}

	_ = store.UpdateStatus(ctx, workID, StatusAccepted, nil)
	record = waitForWork(StatusAccepted)
	model.handleWorkUpdate(record)
	if model.activeWork[workID] == nil {
		t.Error("after accept: work should still be in activeWork")
	}
	if model.activeWork[workID].Status != StatusAccepted {
		t.Errorf("status = %q, want %q", model.activeWork[workID].Status, StatusAccepted)
	}

	_ = store.UpdateStatus(ctx, workID, StatusComplete, &WorkResult{
		Result:     "success",
		DurationMs: 500,
	})
	record = waitForWork(StatusComplete)
	model.handleWorkUpdate(record)
	if model.activeWork[workID] != nil {
		t.Error("after complete: work should not be in activeWork")
	}
	if len(model.recentWork) == 0 {
		t.Error("after complete: recentWork should have entry")
	} else if model.recentWork[0].Result != "success" {
		t.Errorf("result = %q, want success", model.recentWork[0].Result)
	}

	view := model.View()
	if view == "" {
		t.Error("view should not be empty")
	}
	if len(model.recentWork) == 0 || model.recentWork[0].AgentName == "" {
		t.Error("recent work agent name should be set")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
