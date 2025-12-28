package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SessionManager manages sandbox sessions with workflow-scoped resolution.
// Sessions are keyed by SessionKey which allows multiple agents in a workflow
// to share the same sandbox environment.
type SessionManager struct {
	backend     SandboxBackend
	sessions    sync.Map
	defaultOpts SessionOptions
	mu          sync.Mutex
}

func NewSessionManager(backend SandboxBackend) *SessionManager {
	return &SessionManager{
		backend:     backend,
		defaultOpts: DefaultSessionOptions(),
	}
}

func (m *SessionManager) SetDefaultOptions(opts SessionOptions) {
	m.defaultOpts = opts
}

// GetOrCreateSession returns an existing session or creates a new one.
// This is the primary method for agents to get a sandbox session.
func (m *SessionManager) GetOrCreateSession(ctx context.Context, key SessionKey, opts *SessionOptions) (*Session, bool, error) {
	keyStr := key.String()

	if existing, ok := m.sessions.Load(keyStr); ok {
		session := existing.(*Session)
		if _, err := m.backend.GetSession(ctx, session.ID); err == nil {
			return session, false, nil
		}
		m.sessions.Delete(keyStr)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sessions.Load(keyStr); ok {
		session := existing.(*Session)
		if _, err := m.backend.GetSession(ctx, session.ID); err == nil {
			return session, false, nil
		}
		m.sessions.Delete(keyStr)
	}

	sessionOpts := m.defaultOpts
	if opts != nil {
		sessionOpts = *opts
	}

	session, err := m.backend.CreateSession(ctx, sessionOpts)
	if err != nil {
		return nil, false, fmt.Errorf("create sandbox session: %w", err)
	}

	session.Key = key
	m.sessions.Store(keyStr, session)

	return session, true, nil
}

// GetSession returns an existing session by key, or nil if not found.
func (m *SessionManager) GetSession(ctx context.Context, key SessionKey) (*Session, error) {
	keyStr := key.String()

	existing, ok := m.sessions.Load(keyStr)
	if !ok {
		return nil, &SandboxError{Op: "GetSession", Err: ErrSessionNotFound}
	}

	session := existing.(*Session)
	if _, err := m.backend.GetSession(ctx, session.ID); err != nil {
		m.sessions.Delete(keyStr)
		return nil, &SandboxError{Op: "GetSession", Session: session.ID, Err: ErrSessionClosed}
	}

	return session, nil
}

// GetSessionByID returns an existing session by ID.
func (m *SessionManager) GetSessionByID(ctx context.Context, sessionID string) (*Session, error) {
	var found *Session
	m.sessions.Range(func(_, v interface{}) bool {
		session := v.(*Session)
		if session.ID == sessionID {
			found = session
			return false
		}
		return true
	})

	if found == nil {
		return nil, &SandboxError{Op: "GetSessionByID", Session: sessionID, Err: ErrSessionNotFound}
	}

	if _, err := m.backend.GetSession(ctx, found.ID); err != nil {
		m.sessions.Delete(found.Key.String())
		return nil, &SandboxError{Op: "GetSessionByID", Session: sessionID, Err: ErrSessionClosed}
	}

	return found, nil
}

// CloseSession destroys a session and removes it from the manager.
func (m *SessionManager) CloseSession(ctx context.Context, sessionID string) error {
	var sessionKey string
	m.sessions.Range(func(k, v interface{}) bool {
		session := v.(*Session)
		if session.ID == sessionID {
			sessionKey = k.(string)
			return false
		}
		return true
	})

	if sessionKey != "" {
		m.sessions.Delete(sessionKey)
	}

	return m.backend.DestroySession(ctx, sessionID)
}

// CleanupWorkflow destroys all sessions belonging to a workflow run.
func (m *SessionManager) CleanupWorkflow(ctx context.Context, workflowRunID string) error {
	prefix := fmt.Sprintf("workflow:%s:", workflowRunID)

	var toDelete []string
	m.sessions.Range(func(k, _ interface{}) bool {
		keyStr := k.(string)
		if strings.HasPrefix(keyStr, prefix) {
			toDelete = append(toDelete, keyStr)
		}
		return true
	})

	var errs []error
	for _, keyStr := range toDelete {
		if v, ok := m.sessions.LoadAndDelete(keyStr); ok {
			session := v.(*Session)
			if err := m.backend.DestroySession(ctx, session.ID); err != nil {
				errs = append(errs, fmt.Errorf("destroy session %s: %w", session.ID, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup workflow %s: %d errors, first: %w", workflowRunID, len(errs), errs[0])
	}

	return nil
}

// CleanupAgentRun destroys all sessions belonging to an agent run.
func (m *SessionManager) CleanupAgentRun(ctx context.Context, agentRunID string) error {
	prefix := fmt.Sprintf("agent:%s:", agentRunID)

	var toDelete []string
	m.sessions.Range(func(k, _ interface{}) bool {
		keyStr := k.(string)
		if strings.HasPrefix(keyStr, prefix) {
			toDelete = append(toDelete, keyStr)
		}
		return true
	})

	var errs []error
	for _, keyStr := range toDelete {
		if v, ok := m.sessions.LoadAndDelete(keyStr); ok {
			session := v.(*Session)
			if err := m.backend.DestroySession(ctx, session.ID); err != nil {
				errs = append(errs, fmt.Errorf("destroy session %s: %w", session.ID, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup agent run %s: %d errors, first: %w", agentRunID, len(errs), errs[0])
	}

	return nil
}

// CleanupIdleSessions destroys sessions that have been idle longer than idleTimeout.
func (m *SessionManager) CleanupIdleSessions(ctx context.Context, idleTimeout time.Duration) (int, error) {
	now := time.Now()
	var toDelete []string

	m.sessions.Range(func(k, v interface{}) bool {
		session := v.(*Session)
		if now.Sub(session.LastUsedAt) > idleTimeout {
			toDelete = append(toDelete, k.(string))
		}
		return true
	})

	cleaned := 0
	var errs []error
	for _, keyStr := range toDelete {
		if v, ok := m.sessions.LoadAndDelete(keyStr); ok {
			session := v.(*Session)
			if err := m.backend.DestroySession(ctx, session.ID); err != nil {
				errs = append(errs, fmt.Errorf("destroy session %s: %w", session.ID, err))
			} else {
				cleaned++
			}
		}
	}

	if len(errs) > 0 {
		return cleaned, fmt.Errorf("cleanup idle sessions: %d errors, first: %w", len(errs), errs[0])
	}

	return cleaned, nil
}

// ListSessions returns all active sessions.
func (m *SessionManager) ListSessions() []*Session {
	var sessions []*Session
	m.sessions.Range(func(_, v interface{}) bool {
		sessions = append(sessions, v.(*Session))
		return true
	})
	return sessions
}

// Count returns the number of active sessions.
func (m *SessionManager) Count() int {
	count := 0
	m.sessions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// ResolveSessionKey determines the appropriate session key based on execution context.
func ResolveSessionKey(workflowRunID, agentRunID, sandboxSessionName string) SessionKey {
	if sandboxSessionName == "" {
		sandboxSessionName = "default"
	}

	if workflowRunID != "" {
		return SessionKey{
			Namespace: "workflow",
			ID:        workflowRunID,
			Key:       sandboxSessionName,
		}
	}

	return SessionKey{
		Namespace: "agent",
		ID:        agentRunID,
		Key:       sandboxSessionName,
	}
}
