package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBackend struct {
	sessions     map[string]*CodeSession
	createCount  int
	destroyCount int
	createErr    error
	destroyErr   error
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		sessions: make(map[string]*CodeSession),
	}
}

func (m *mockBackend) CreateSession(ctx context.Context, key SessionKey, cfg SessionConfig) (*CodeSession, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	m.createCount++
	session := &CodeSession{
		ID:          key.String(),
		ContainerID: "container-" + key.String(),
		Key:         key,
		Config:      cfg,
		Status:      SessionStatusReady,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
	}
	m.sessions[key.String()] = session
	return session, nil
}

func (m *mockBackend) DestroySession(ctx context.Context, sessionID string) error {
	if m.destroyErr != nil {
		return m.destroyErr
	}
	m.destroyCount++
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockBackend) GetSession(ctx context.Context, sessionID string) (*CodeSession, error) {
	if s, ok := m.sessions[sessionID]; ok {
		return s, nil
	}
	return nil, nil
}

func (m *mockBackend) Exec(ctx context.Context, sessionID string, req ExecRequest) (*ExecResult, error) {
	return &ExecResult{ExitCode: 0, Stdout: "ok"}, nil
}

func (m *mockBackend) WriteFile(ctx context.Context, sessionID, path string, content []byte) error {
	return nil
}

func (m *mockBackend) ReadFile(ctx context.Context, sessionID, path string) ([]byte, error) {
	return []byte("content"), nil
}

func (m *mockBackend) ListFiles(ctx context.Context, sessionID, path string, recursive bool) ([]FileEntry, error) {
	return nil, nil
}

func (m *mockBackend) DeleteFile(ctx context.Context, sessionID, path string, recursive bool) error {
	return nil
}

var _ SandboxBackend = (*mockBackend)(nil)

func TestSessionKey_String(t *testing.T) {
	tests := []struct {
		name     string
		key      SessionKey
		expected string
	}{
		{
			name:     "workflow scope",
			key:      NewWorkflowSessionKey("wf123"),
			expected: "workflow_wf123_sandbox",
		},
		{
			name:     "agent scope",
			key:      NewAgentSessionKey("agent456"),
			expected: "agent_agent456_sandbox",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.key.String())
		})
	}
}

func TestExecutionContext_SessionKey(t *testing.T) {
	ctx := ExecutionContext{
		WorkflowRunID: "wf123",
		AgentRunID:    "agent456",
	}

	t.Run("workflow scope with workflow ID", func(t *testing.T) {
		key := ctx.SessionKey(SessionScopeWorkflow)
		assert.Equal(t, SessionScopeWorkflow, key.Scope)
		assert.Equal(t, "wf123", key.ScopeID)
	})

	t.Run("agent scope", func(t *testing.T) {
		key := ctx.SessionKey(SessionScopeAgent)
		assert.Equal(t, SessionScopeAgent, key.Scope)
		assert.Equal(t, "agent456", key.ScopeID)
	})

	t.Run("workflow scope without workflow ID falls back to agent", func(t *testing.T) {
		ctxNoWf := ExecutionContext{AgentRunID: "agent789"}
		key := ctxNoWf.SessionKey(SessionScopeWorkflow)
		assert.Equal(t, SessionScopeAgent, key.Scope)
		assert.Equal(t, "agent789", key.ScopeID)
	})
}

func TestSessionManager_GetOrCreateSession(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := NewWorkflowSessionKey("test-workflow")
	cfg := SessionConfig{Runtime: "python"}

	t.Run("creates new session", func(t *testing.T) {
		session, err := manager.GetOrCreateSession(ctx, key, cfg)
		require.NoError(t, err)
		assert.Equal(t, key.String(), session.ID)
		assert.Equal(t, 1, backend.createCount)
		assert.Equal(t, 1, manager.SessionCount())
	})

	t.Run("returns existing session", func(t *testing.T) {
		session, err := manager.GetOrCreateSession(ctx, key, cfg)
		require.NoError(t, err)
		assert.Equal(t, key.String(), session.ID)
		assert.Equal(t, 1, backend.createCount)
	})

	t.Run("creates different session for different key", func(t *testing.T) {
		key2 := NewWorkflowSessionKey("other-workflow")
		session, err := manager.GetOrCreateSession(ctx, key2, cfg)
		require.NoError(t, err)
		assert.Equal(t, key2.String(), session.ID)
		assert.Equal(t, 2, backend.createCount)
		assert.Equal(t, 2, manager.SessionCount())
	})
}

func TestSessionManager_DestroySession(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := NewWorkflowSessionKey("test-workflow")
	cfg := SessionConfig{Runtime: "python"}

	_, err := manager.GetOrCreateSession(ctx, key, cfg)
	require.NoError(t, err)
	assert.Equal(t, 1, manager.SessionCount())

	err = manager.DestroySession(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, 0, manager.SessionCount())
	assert.Equal(t, 1, backend.destroyCount)
}

func TestSessionManager_CleanupWorkflow(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key := NewWorkflowSessionKey("cleanup-wf")
	cfg := SessionConfig{Runtime: "python"}

	_, err := manager.GetOrCreateSession(ctx, key, cfg)
	require.NoError(t, err)

	err = manager.CleanupWorkflow(ctx, "cleanup-wf")
	require.NoError(t, err)
	assert.Equal(t, 0, manager.SessionCount())
}

func TestSessionManager_CleanupIdleSessions(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	key1 := NewWorkflowSessionKey("wf1")
	key2 := NewWorkflowSessionKey("wf2")
	cfg := SessionConfig{Runtime: "python"}

	s1, _ := manager.GetOrCreateSession(ctx, key1, cfg)
	s2, _ := manager.GetOrCreateSession(ctx, key2, cfg)

	s1.LastUsedAt = time.Now().Add(-2 * time.Hour)
	s2.LastUsedAt = time.Now()

	cleaned, err := manager.CleanupIdleSessions(ctx, 1*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, 1, manager.SessionCount())
}

func TestSessionManager_Close(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()
	manager := NewSessionManager(backend)

	cfg := SessionConfig{Runtime: "python"}
	manager.GetOrCreateSession(ctx, NewWorkflowSessionKey("wf1"), cfg)
	manager.GetOrCreateSession(ctx, NewWorkflowSessionKey("wf2"), cfg)
	manager.GetOrCreateSession(ctx, NewWorkflowSessionKey("wf3"), cfg)

	assert.Equal(t, 3, manager.SessionCount())

	err := manager.Close(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, manager.SessionCount())
	assert.Equal(t, 3, backend.destroyCount)
}
