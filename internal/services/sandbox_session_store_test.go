package services

import (
	"context"
	"testing"
	"time"
)

func TestInMemorySessionStore_PutAndGet(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	record := &SessionRecord{
		SessionID: "sess-abc",
		Image:     "python:3.11",
		Workdir:   "/workspace",
		Backend:   "fly_machines",
	}

	err := store.Put(ctx, key, record)
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
	if retrieved.SessionID != "sess-abc" {
		t.Errorf("expected SessionID=sess-abc, got %s", retrieved.SessionID)
	}
	if retrieved.Key.String() != key.String() {
		t.Errorf("expected Key=%s, got %s", key.String(), retrieved.Key.String())
	}
}

func TestInMemorySessionStore_GetNonExistent(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-nonexistent", Key: "default"}
	record, err := store.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if record != nil {
		t.Error("expected nil record for nonexistent key")
	}
}

func TestInMemorySessionStore_GetBySessionID(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	record := &SessionRecord{
		SessionID: "fly_abc123",
		Image:     "python:3.11",
	}

	_ = store.Put(ctx, key, record)

	retrieved, err := store.GetBySessionID(ctx, "fly_abc123")
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

func TestInMemorySessionStore_Delete(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	record := &SessionRecord{SessionID: "sess-abc"}
	_ = store.Put(ctx, key, record)

	err := store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	retrieved, _ := store.Get(ctx, key)
	if retrieved != nil {
		t.Error("expected nil after delete")
	}
}

func TestInMemorySessionStore_DeleteByPrefix(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	keys := []SessionKey{
		{Namespace: "workflow", ID: "run-123", Key: "default"},
		{Namespace: "workflow", ID: "run-123", Key: "isolated"},
		{Namespace: "workflow", ID: "run-456", Key: "default"},
	}

	for i, key := range keys {
		record := &SessionRecord{SessionID: string(rune('a' + i))}
		_ = store.Put(ctx, key, record)
	}

	deleted, err := store.DeleteByPrefix(ctx, "workflow:run-123:")
	if err != nil {
		t.Fatalf("DeleteByPrefix failed: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	remaining, _ := store.List(ctx)
	if len(remaining) != 1 {
		t.Errorf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestInMemorySessionStore_List(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		key := SessionKey{Namespace: "workflow", ID: "run-" + string(rune('0'+i)), Key: "default"}
		record := &SessionRecord{SessionID: "sess-" + string(rune('a'+i))}
		_ = store.Put(ctx, key, record)
	}

	records, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

func TestInMemorySessionStore_ListByPrefix(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	keys := []SessionKey{
		{Namespace: "workflow", ID: "run-123", Key: "a"},
		{Namespace: "workflow", ID: "run-123", Key: "b"},
		{Namespace: "agent", ID: "agent-456", Key: "c"},
	}

	for i, key := range keys {
		record := &SessionRecord{SessionID: string(rune('a' + i))}
		_ = store.Put(ctx, key, record)
	}

	records, err := store.ListByPrefix(ctx, "workflow:")
	if err != nil {
		t.Fatalf("ListByPrefix failed: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records with workflow prefix, got %d", len(records))
	}
}

func TestInMemorySessionStore_UpdateLastUsed(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	record := &SessionRecord{SessionID: "sess-abc"}
	_ = store.Put(ctx, key, record)

	initial, _ := store.Get(ctx, key)
	initialTime := initial.LastUsedAt

	time.Sleep(10 * time.Millisecond)

	err := store.UpdateLastUsed(ctx, key)
	if err != nil {
		t.Fatalf("UpdateLastUsed failed: %v", err)
	}

	updated, _ := store.Get(ctx, key)
	if !updated.LastUsedAt.After(initialTime) {
		t.Error("expected LastUsedAt to be updated")
	}
}

func TestInMemorySessionStore_CreatedAtPreserved(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	createdAt := time.Now().Add(-1 * time.Hour)
	record := &SessionRecord{
		SessionID: "sess-abc",
		CreatedAt: createdAt,
	}
	_ = store.Put(ctx, key, record)

	retrieved, _ := store.Get(ctx, key)
	if retrieved.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be preserved")
	}
	if !retrieved.CreatedAt.Equal(createdAt) {
		t.Errorf("expected CreatedAt=%v, got %v", createdAt, retrieved.CreatedAt)
	}
}

func TestSessionManagerWithStore_RecoverFromStore(t *testing.T) {
	backend := newMockBackend()
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	session, _ := backend.CreateSession(ctx, SessionOptions{
		Image:   "python:3.11",
		Workdir: "/workspace",
	})

	record := &SessionRecord{
		SessionID: session.ID,
		Key:       key,
		Image:     "python:3.11",
		Workdir:   "/workspace",
		Backend:   "mock",
		CreatedAt: time.Now(),
	}
	_ = store.Put(ctx, key, record)

	manager := NewSessionManagerWithStore(backend, store)

	recoveredSession, created, err := manager.GetOrCreateSession(ctx, key, nil)
	if err != nil {
		t.Fatalf("GetOrCreateSession failed: %v", err)
	}
	if created {
		t.Error("expected created=false (session recovered from store)")
	}
	if recoveredSession.ID != session.ID {
		t.Errorf("expected session ID %s, got %s", session.ID, recoveredSession.ID)
	}
	if backend.createCalls != 1 {
		t.Errorf("expected 1 create call (initial), got %d", backend.createCalls)
	}
}

func TestSessionManager_RecoverSessions(t *testing.T) {
	backend := newMockBackend()
	store := NewInMemorySessionStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		key := SessionKey{Namespace: "workflow", ID: "run-" + string(rune('0'+i)), Key: "default"}
		session, _ := backend.CreateSession(ctx, SessionOptions{})
		record := &SessionRecord{
			SessionID: session.ID,
			Key:       key,
			Backend:   "mock",
		}
		_ = store.Put(ctx, key, record)
	}

	manager := NewSessionManagerWithStore(backend, store)

	if manager.Count() != 0 {
		t.Errorf("expected 0 sessions before recovery, got %d", manager.Count())
	}

	recovered, err := manager.RecoverSessions(ctx)
	if err != nil {
		t.Fatalf("RecoverSessions failed: %v", err)
	}
	if recovered != 3 {
		t.Errorf("expected 3 recovered, got %d", recovered)
	}
	if manager.Count() != 3 {
		t.Errorf("expected 3 sessions after recovery, got %d", manager.Count())
	}
}

func TestSessionManager_RecoverSessions_PrunesStale(t *testing.T) {
	backend := newMockBackend()
	store := NewInMemorySessionStore()
	ctx := context.Background()

	key1 := SessionKey{Namespace: "workflow", ID: "run-1", Key: "default"}
	session1, _ := backend.CreateSession(ctx, SessionOptions{})
	record1 := &SessionRecord{SessionID: session1.ID, Key: key1}
	_ = store.Put(ctx, key1, record1)

	key2 := SessionKey{Namespace: "workflow", ID: "run-2", Key: "default"}
	record2 := &SessionRecord{SessionID: "nonexistent-session", Key: key2}
	_ = store.Put(ctx, key2, record2)

	manager := NewSessionManagerWithStore(backend, store)

	recovered, err := manager.RecoverSessions(ctx)
	if err != nil {
		t.Fatalf("RecoverSessions failed: %v", err)
	}
	if recovered != 1 {
		t.Errorf("expected 1 recovered (stale should be pruned), got %d", recovered)
	}

	staleRecord, _ := store.Get(ctx, key2)
	if staleRecord != nil {
		t.Error("expected stale record to be deleted from store")
	}
}
