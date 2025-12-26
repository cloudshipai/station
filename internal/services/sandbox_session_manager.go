package services

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SessionManager struct {
	backend  SandboxBackend
	sessions map[string]*CodeSession
	mu       sync.RWMutex
}

func NewSessionManager(backend SandboxBackend) *SessionManager {
	return &SessionManager{
		backend:  backend,
		sessions: make(map[string]*CodeSession),
	}
}

func (m *SessionManager) GetOrCreateSession(ctx context.Context, key SessionKey, cfg SessionConfig) (*CodeSession, error) {
	keyStr := key.String()

	m.mu.RLock()
	session, exists := m.sessions[keyStr]
	m.mu.RUnlock()

	if exists && session.Status == SessionStatusReady {
		session.LastUsedAt = time.Now()
		return session, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if session, exists = m.sessions[keyStr]; exists && session.Status == SessionStatusReady {
		session.LastUsedAt = time.Now()
		return session, nil
	}

	newSession, err := m.backend.CreateSession(ctx, key, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	m.sessions[keyStr] = newSession
	return newSession, nil
}

func (m *SessionManager) GetSession(key SessionKey) (*CodeSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[key.String()]
	if exists && session.Status == SessionStatusReady {
		return session, true
	}
	return nil, false
}

func (m *SessionManager) DestroySession(ctx context.Context, key SessionKey) error {
	keyStr := key.String()

	m.mu.Lock()
	session, exists := m.sessions[keyStr]
	if !exists {
		m.mu.Unlock()
		return nil
	}
	delete(m.sessions, keyStr)
	m.mu.Unlock()

	if session.ContainerID != "" {
		return m.backend.DestroySession(ctx, session.ContainerID)
	}
	return nil
}

func (m *SessionManager) CleanupWorkflow(ctx context.Context, workflowRunID string) error {
	key := NewWorkflowSessionKey(workflowRunID)
	return m.DestroySession(ctx, key)
}

func (m *SessionManager) CleanupAgent(ctx context.Context, agentRunID string) error {
	key := NewAgentSessionKey(agentRunID)
	return m.DestroySession(ctx, key)
}

func (m *SessionManager) CleanupIdleSessions(ctx context.Context, maxIdle time.Duration) (int, error) {
	m.mu.Lock()
	var toCleanup []SessionKey
	now := time.Now()

	for keyStr, session := range m.sessions {
		if now.Sub(session.LastUsedAt) > maxIdle {
			toCleanup = append(toCleanup, session.Key)
			delete(m.sessions, keyStr)
		}
	}
	m.mu.Unlock()

	var cleanupErrors []error
	for _, key := range toCleanup {
		if err := m.backend.DestroySession(ctx, key.String()); err != nil {
			cleanupErrors = append(cleanupErrors, err)
		}
	}

	if len(cleanupErrors) > 0 {
		return len(toCleanup), fmt.Errorf("cleanup had %d errors", len(cleanupErrors))
	}
	return len(toCleanup), nil
}

func (m *SessionManager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *SessionManager) Close(ctx context.Context) error {
	m.mu.Lock()
	sessions := make([]*CodeSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.sessions = make(map[string]*CodeSession)
	m.mu.Unlock()

	var errs []error
	for _, s := range sessions {
		if err := m.backend.DestroySession(ctx, s.ContainerID); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to destroy %d sessions", len(errs))
	}
	return nil
}
