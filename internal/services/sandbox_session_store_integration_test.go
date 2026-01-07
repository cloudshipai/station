//go:build integration

package services

import (
	"context"
	"testing"
	"time"

	natsserver_test "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func setupTestNATS(t *testing.T) (nats.JetStreamContext, func()) {
	serverOpts := natsserver_test.DefaultTestOptions
	serverOpts.Port = -1
	serverOpts.JetStream = true
	srv := natsserver_test.RunServer(&serverOpts)

	conn, err := nats.Connect(srv.ClientURL())
	if err != nil {
		srv.Shutdown()
		t.Fatalf("failed to connect to NATS: %v", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		srv.Shutdown()
		t.Fatalf("failed to get JetStream: %v", err)
	}

	cleanup := func() {
		conn.Close()
		srv.Shutdown()
	}

	return js, cleanup
}

func TestNATSSessionStore_PutAndGet(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	record := &SessionRecord{
		SessionID: "fly_abc123",
		Image:     "python:3.11",
		Workdir:   "/workspace",
		Backend:   "fly_machines",
		MachineID: "machine-xyz",
		AppName:   "stn-sandbox",
	}

	err = store.Put(ctx, key, record)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	retrieved, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected record, got nil")
	}
	if retrieved.SessionID != "fly_abc123" {
		t.Errorf("expected SessionID=fly_abc123, got %s", retrieved.SessionID)
	}
	if retrieved.MachineID != "machine-xyz" {
		t.Errorf("expected MachineID=machine-xyz, got %s", retrieved.MachineID)
	}
	if retrieved.Key.String() != key.String() {
		t.Errorf("expected Key=%s, got %s", key.String(), retrieved.Key.String())
	}
}

func TestNATSSessionStore_GetBySessionID(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	key := SessionKey{Namespace: "workflow", ID: "run-456", Key: "isolated"}
	record := &SessionRecord{
		SessionID: "fly_def456",
		Image:     "node:18",
	}

	_ = store.Put(ctx, key, record)

	retrieved, err := store.GetBySessionID(ctx, "fly_def456")
	if err != nil {
		t.Fatalf("GetBySessionID failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected record, got nil")
	}
	if retrieved.Key.String() != key.String() {
		t.Errorf("expected Key=%s, got %s", key.String(), retrieved.Key.String())
	}
}

func TestNATSSessionStore_Delete(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	key := SessionKey{Namespace: "workflow", ID: "run-789", Key: "default"}
	record := &SessionRecord{SessionID: "fly_ghi789"}
	_ = store.Put(ctx, key, record)

	err = store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	retrieved, _ := store.Get(ctx, key)
	if retrieved != nil {
		t.Error("expected nil after delete")
	}

	byID, _ := store.GetBySessionID(ctx, "fly_ghi789")
	if byID != nil {
		t.Error("expected session ID index to be deleted")
	}
}

func TestNATSSessionStore_List(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	testPrefix := "list-test-" + time.Now().Format("150405")

	for i := 0; i < 5; i++ {
		key := SessionKey{Namespace: "workflow", ID: testPrefix + "-" + string(rune('0'+i)), Key: "default"}
		record := &SessionRecord{SessionID: "fly_list_" + string(rune('a'+i))}
		_ = store.Put(ctx, key, record)
	}

	records, err := store.ListByPrefix(ctx, testPrefix)
	if err != nil {
		t.Fatalf("ListByPrefix failed: %v", err)
	}
	if len(records) != 5 {
		t.Errorf("expected 5 records, got %d", len(records))
	}
}

func TestNATSSessionStore_ListByPrefix(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	testID := time.Now().Format("150405.000")

	keys := []SessionKey{
		{Namespace: "wfprefix" + testID, ID: "run-1", Key: "a"},
		{Namespace: "wfprefix" + testID, ID: "run-1", Key: "b"},
		{Namespace: "agprefix" + testID, ID: "agent-2", Key: "c"},
	}

	for i, key := range keys {
		record := &SessionRecord{SessionID: "fly_prefix_" + string(rune('a'+i))}
		_ = store.Put(ctx, key, record)
	}

	workflowRecords, err := store.ListByPrefix(ctx, "wfprefix"+testID)
	if err != nil {
		t.Fatalf("ListByPrefix failed: %v", err)
	}
	if len(workflowRecords) != 2 {
		t.Errorf("expected 2 workflow records, got %d", len(workflowRecords))
	}

	agentRecords, _ := store.ListByPrefix(ctx, "agprefix"+testID)
	if len(agentRecords) != 1 {
		t.Errorf("expected 1 agent record, got %d", len(agentRecords))
	}
}

func TestNATSSessionStore_DeleteByPrefix(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	testID := time.Now().Format("150405.000")

	keys := []SessionKey{
		{Namespace: "workflow", ID: "del-" + testID + "-1", Key: "default"},
		{Namespace: "workflow", ID: "del-" + testID + "-1", Key: "isolated"},
		{Namespace: "workflow", ID: "del-" + testID + "-2", Key: "default"},
	}

	for i, key := range keys {
		record := &SessionRecord{SessionID: "fly_del_" + string(rune('a'+i))}
		_ = store.Put(ctx, key, record)
	}

	deleted, err := store.DeleteByPrefix(ctx, "del-"+testID+"-1")
	if err != nil {
		t.Fatalf("DeleteByPrefix failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining, _ := store.ListByPrefix(ctx, "del-"+testID)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestNATSSessionStore_UpdateLastUsed(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	key := SessionKey{Namespace: "workflow", ID: "run-update", Key: "default"}
	record := &SessionRecord{SessionID: "fly_update"}
	_ = store.Put(ctx, key, record)

	initial, _ := store.Get(ctx, key)
	initialTime := initial.LastUsedAt

	time.Sleep(50 * time.Millisecond)

	err = store.UpdateLastUsed(ctx, key)
	if err != nil {
		t.Fatalf("UpdateLastUsed failed: %v", err)
	}

	updated, _ := store.Get(ctx, key)
	if !updated.LastUsedAt.After(initialTime) {
		t.Error("expected LastUsedAt to be updated")
	}
}

func TestNATSSessionStore_PersistenceAcrossInstances(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	ctx := context.Background()
	key := SessionKey{Namespace: "workflow", ID: "run-persist", Key: "default"}

	store1, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store1: %v", err)
	}

	record := &SessionRecord{
		SessionID: "fly_persist_123",
		MachineID: "machine-persist",
		Image:     "python:3.11",
		Workdir:   "/workspace",
		Backend:   "fly_machines",
	}
	_ = store1.Put(ctx, key, record)

	store2, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store2: %v", err)
	}

	retrieved, err := store2.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get from store2 failed: %v", err)
	}
	if retrieved == nil {
		t.Fatal("expected record from store2, got nil")
	}
	if retrieved.SessionID != "fly_persist_123" {
		t.Errorf("expected SessionID=fly_persist_123, got %s", retrieved.SessionID)
	}
	if retrieved.MachineID != "machine-persist" {
		t.Errorf("expected MachineID=machine-persist, got %s", retrieved.MachineID)
	}

	t.Log("SUCCESS: Session record persisted and retrieved across store instances")
}

func TestNATSSessionStore_SimulateRestartRecovery(t *testing.T) {
	js, cleanup := setupTestNATS(t)
	defer cleanup()

	ctx := context.Background()
	backend := newMockBackend()

	store, err := NewNATSSessionStore(js, DefaultSessionStoreConfig())
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	manager1 := NewSessionManagerWithStore(backend, store)

	key := SessionKey{Namespace: "workflow", ID: "wf-restart-test", Key: "default"}
	session, created, err := manager1.GetOrCreateSession(ctx, key, &SessionOptions{
		Image:   "python:3.11",
		Workdir: "/workspace",
	})
	if err != nil {
		t.Fatalf("GetOrCreateSession failed: %v", err)
	}
	if !created {
		t.Error("expected created=true for new session")
	}

	originalSessionID := session.ID
	t.Logf("Created session: %s", originalSessionID)

	manager2 := NewSessionManagerWithStore(backend, store)

	if manager2.Count() != 0 {
		t.Errorf("expected 0 sessions in new manager before recovery, got %d", manager2.Count())
	}

	recoveredSession, created, err := manager2.GetOrCreateSession(ctx, key, nil)
	if err != nil {
		t.Fatalf("GetOrCreateSession after restart failed: %v", err)
	}
	if created {
		t.Error("expected created=false (session should be recovered from NATS KV)")
	}
	if recoveredSession.ID != originalSessionID {
		t.Errorf("expected session ID %s, got %s", originalSessionID, recoveredSession.ID)
	}

	t.Logf("SUCCESS: Session %s recovered after simulated restart", originalSessionID)
}
