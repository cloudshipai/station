package services

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

type mockBackend struct {
	mu              sync.Mutex
	sessions        map[string]*Session
	createCalls     int
	destroyCalls    int
	createError     error
	destroyError    error
	getSessionError error
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		sessions: make(map[string]*Session),
	}
}

func (m *mockBackend) Ping(ctx context.Context) error { return nil }

func (m *mockBackend) CreateSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createCalls++
	if m.createError != nil {
		return nil, m.createError
	}
	session := &Session{
		ID:         generateShortID(),
		Image:      opts.Image,
		Workdir:    opts.Workdir,
		Limits:     opts.Limits,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
	}
	m.sessions[session.ID] = session
	return session, nil
}

func (m *mockBackend) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getSessionError != nil {
		return nil, m.getSessionError
	}
	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (m *mockBackend) DestroySession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destroyCalls++
	if m.destroyError != nil {
		return m.destroyError
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
	return &ExecResult{ExitCode: 0}, nil
}

func (m *mockBackend) ExecAsync(ctx context.Context, sessionID string, req ExecRequest) (*ExecHandle, error) {
	return &ExecHandle{ID: "exec-1"}, nil
}

func (m *mockBackend) ExecWait(ctx context.Context, sessionID, execID string, timeout time.Duration) (*ExecResult, error) {
	return &ExecResult{ExitCode: 0}, nil
}

func (m *mockBackend) ExecRead(ctx context.Context, sessionID, execID string, sinceSeq int, maxChunks int) (*ExecChunks, error) {
	return &ExecChunks{Done: true}, nil
}

func (m *mockBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte, mode os.FileMode) error {
	return nil
}

func (m *mockBackend) ReadFile(ctx context.Context, sessionID, path string, maxBytes int) ([]byte, bool, error) {
	return nil, false, nil
}

func (m *mockBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
	return nil, nil
}

func (m *mockBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
	return nil
}

func TestSessionManager_GetOrCreateSession_CreatesNew(t *testing.T) {
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	session, created, err := manager.GetOrCreateSession(context.Background(), key, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected created=true for new session")
	}
	if session == nil {
		t.Fatal("expected session to be non-nil")
	}
	if backend.createCalls != 1 {
		t.Errorf("expected 1 create call, got %d", backend.createCalls)
	}
}

func TestSessionManager_GetOrCreateSession_ReusesExisting(t *testing.T) {
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}

	session1, created1, err := manager.GetOrCreateSession(context.Background(), key, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created1 {
		t.Error("expected created=true for first call")
	}

	session2, created2, err := manager.GetOrCreateSession(context.Background(), key, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created2 {
		t.Error("expected created=false for second call")
	}
	if session1.ID != session2.ID {
		t.Errorf("expected same session ID, got %s vs %s", session1.ID, session2.ID)
	}
	if backend.createCalls != 1 {
		t.Errorf("expected 1 create call, got %d", backend.createCalls)
	}
}

func TestSessionManager_DifferentKeysCreateDifferentSessions(t *testing.T) {
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key1 := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	key2 := SessionKey{Namespace: "workflow", ID: "run-456", Key: "default"}

	session1, _, _ := manager.GetOrCreateSession(context.Background(), key1, nil)
	session2, _, _ := manager.GetOrCreateSession(context.Background(), key2, nil)

	if session1.ID == session2.ID {
		t.Error("expected different session IDs for different keys")
	}
	if backend.createCalls != 2 {
		t.Errorf("expected 2 create calls, got %d", backend.createCalls)
	}
}

func TestSessionManager_CleanupWorkflow(t *testing.T) {
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key1 := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	key2 := SessionKey{Namespace: "workflow", ID: "run-123", Key: "isolated"}
	key3 := SessionKey{Namespace: "workflow", ID: "run-other", Key: "default"}

	manager.GetOrCreateSession(context.Background(), key1, nil)
	manager.GetOrCreateSession(context.Background(), key2, nil)
	manager.GetOrCreateSession(context.Background(), key3, nil)

	if manager.Count() != 3 {
		t.Fatalf("expected 3 sessions, got %d", manager.Count())
	}

	err := manager.CleanupWorkflow(context.Background(), "run-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manager.Count() != 1 {
		t.Errorf("expected 1 session remaining, got %d", manager.Count())
	}
	if backend.destroyCalls != 2 {
		t.Errorf("expected 2 destroy calls, got %d", backend.destroyCalls)
	}
}

func TestSessionManager_CleanupIdleSessions(t *testing.T) {
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := SessionKey{Namespace: "agent", ID: "run-123", Key: "default"}
	session, _, _ := manager.GetOrCreateSession(context.Background(), key, nil)

	session.LastUsedAt = time.Now().Add(-2 * time.Hour)

	cleaned, err := manager.CleanupIdleSessions(context.Background(), 1*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleaned != 1 {
		t.Errorf("expected 1 cleaned session, got %d", cleaned)
	}
	if manager.Count() != 0 {
		t.Errorf("expected 0 sessions remaining, got %d", manager.Count())
	}
}

func TestResolveSessionKey_WorkflowContext(t *testing.T) {
	key := ResolveSessionKey("wf-123", "agent-456", "")

	if key.Namespace != "workflow" {
		t.Errorf("expected namespace=workflow, got %s", key.Namespace)
	}
	if key.ID != "wf-123" {
		t.Errorf("expected ID=wf-123, got %s", key.ID)
	}
	if key.Key != "default" {
		t.Errorf("expected Key=default, got %s", key.Key)
	}
}

func TestResolveSessionKey_AgentContext(t *testing.T) {
	key := ResolveSessionKey("", "agent-456", "custom")

	if key.Namespace != "agent" {
		t.Errorf("expected namespace=agent, got %s", key.Namespace)
	}
	if key.ID != "agent-456" {
		t.Errorf("expected ID=agent-456, got %s", key.ID)
	}
	if key.Key != "custom" {
		t.Errorf("expected Key=custom, got %s", key.Key)
	}
}

func TestSessionKey_String(t *testing.T) {
	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}
	expected := "workflow:run-123:default"
	if key.String() != expected {
		t.Errorf("expected %q, got %q", expected, key.String())
	}
}

func TestSessionManager_ConcurrentAccess(t *testing.T) {
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := SessionKey{Namespace: "workflow", ID: "run-123", Key: "default"}

	var wg sync.WaitGroup
	sessions := make(chan *Session, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			session, _, err := manager.GetOrCreateSession(context.Background(), key, nil)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			sessions <- session
		}()
	}

	wg.Wait()
	close(sessions)

	var firstID string
	for session := range sessions {
		if firstID == "" {
			firstID = session.ID
		} else if session.ID != firstID {
			t.Errorf("expected all sessions to have same ID, got %s vs %s", firstID, session.ID)
		}
	}

	if backend.createCalls != 1 {
		t.Errorf("expected 1 create call (concurrent safety), got %d", backend.createCalls)
	}
}
