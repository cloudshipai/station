package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type SessionManager struct {
	backend     SandboxBackend
	store       SessionStore
	sessions    sync.Map
	defaultOpts SessionOptions
	mu          sync.Mutex
}

func NewSessionManager(backend SandboxBackend) *SessionManager {
	return &SessionManager{
		backend:     backend,
		store:       NewInMemorySessionStore(),
		defaultOpts: DefaultSessionOptions(),
	}
}

func NewSessionManagerWithStore(backend SandboxBackend, store SessionStore) *SessionManager {
	return &SessionManager{
		backend:     backend,
		store:       store,
		defaultOpts: DefaultSessionOptions(),
	}
}

func (m *SessionManager) SetDefaultOptions(opts SessionOptions) {
	m.defaultOpts = opts
}

func (m *SessionManager) SetStore(store SessionStore) {
	m.store = store
}

func (m *SessionManager) GetOrCreateSession(ctx context.Context, key SessionKey, opts *SessionOptions) (*Session, bool, error) {
	keyStr := key.String()

	if existing, ok := m.sessions.Load(keyStr); ok {
		session := existing.(*Session)
		if _, err := m.backend.GetSession(ctx, session.ID); err == nil {
			_ = m.store.UpdateLastUsed(ctx, key)
			return session, false, nil
		}
		m.sessions.Delete(keyStr)
		_ = m.store.Delete(ctx, key)
	}

	if m.store != nil {
		record, err := m.store.Get(ctx, key)
		if err == nil && record != nil {
			if _, err := m.backend.GetSession(ctx, record.SessionID); err == nil {
				session := &Session{
					ID:         record.SessionID,
					Key:        key,
					Image:      record.Image,
					Workdir:    record.Workdir,
					Env:        record.Env,
					Limits:     record.Limits,
					CreatedAt:  record.CreatedAt,
					LastUsedAt: time.Now(),
				}
				m.sessions.Store(keyStr, session)
				_ = m.store.UpdateLastUsed(ctx, key)
				return session, false, nil
			}
			_ = m.store.Delete(ctx, key)
		}
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

	if m.store != nil {
		record := &SessionRecord{
			SessionID:  session.ID,
			Key:        key,
			Image:      session.Image,
			Workdir:    session.Workdir,
			Env:        session.Env,
			Limits:     session.Limits,
			CreatedAt:  session.CreatedAt,
			LastUsedAt: time.Now(),
		}

		if flySession, ok := m.backend.(*FlyMachinesBackend); ok {
			_ = flySession
			record.Backend = "fly_machines"
		} else {
			record.Backend = "docker"
		}

		_ = m.store.Put(ctx, key, record)
	}

	return session, true, nil
}

func (m *SessionManager) GetSession(ctx context.Context, key SessionKey) (*Session, error) {
	keyStr := key.String()

	existing, ok := m.sessions.Load(keyStr)
	if !ok {
		if m.store != nil {
			record, err := m.store.Get(ctx, key)
			if err == nil && record != nil {
				if _, err := m.backend.GetSession(ctx, record.SessionID); err == nil {
					session := &Session{
						ID:         record.SessionID,
						Key:        key,
						Image:      record.Image,
						Workdir:    record.Workdir,
						Env:        record.Env,
						Limits:     record.Limits,
						CreatedAt:  record.CreatedAt,
						LastUsedAt: time.Now(),
					}
					m.sessions.Store(keyStr, session)
					return session, nil
				}
				_ = m.store.Delete(ctx, key)
			}
		}
		return nil, &SandboxError{Op: "GetSession", Err: ErrSessionNotFound}
	}

	session := existing.(*Session)
	if _, err := m.backend.GetSession(ctx, session.ID); err != nil {
		m.sessions.Delete(keyStr)
		_ = m.store.Delete(ctx, key)
		return nil, &SandboxError{Op: "GetSession", Session: session.ID, Err: ErrSessionClosed}
	}

	return session, nil
}

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

	if found == nil && m.store != nil {
		record, err := m.store.GetBySessionID(ctx, sessionID)
		if err == nil && record != nil {
			if _, err := m.backend.GetSession(ctx, record.SessionID); err == nil {
				found = &Session{
					ID:         record.SessionID,
					Key:        record.Key,
					Image:      record.Image,
					Workdir:    record.Workdir,
					Env:        record.Env,
					Limits:     record.Limits,
					CreatedAt:  record.CreatedAt,
					LastUsedAt: time.Now(),
				}
				m.sessions.Store(record.Key.String(), found)
			} else {
				_ = m.store.Delete(ctx, record.Key)
			}
		}
	}

	if found == nil {
		return nil, &SandboxError{Op: "GetSessionByID", Session: sessionID, Err: ErrSessionNotFound}
	}

	if _, err := m.backend.GetSession(ctx, found.ID); err != nil {
		m.sessions.Delete(found.Key.String())
		_ = m.store.Delete(ctx, found.Key)
		return nil, &SandboxError{Op: "GetSessionByID", Session: sessionID, Err: ErrSessionClosed}
	}

	return found, nil
}

func (m *SessionManager) CloseSession(ctx context.Context, sessionID string) error {
	var sessionKey string
	var sessionKeyObj SessionKey
	m.sessions.Range(func(k, v interface{}) bool {
		session := v.(*Session)
		if session.ID == sessionID {
			sessionKey = k.(string)
			sessionKeyObj = session.Key
			return false
		}
		return true
	})

	if sessionKey != "" {
		m.sessions.Delete(sessionKey)
		_ = m.store.Delete(ctx, sessionKeyObj)
	} else if m.store != nil {
		record, _ := m.store.GetBySessionID(ctx, sessionID)
		if record != nil {
			_ = m.store.Delete(ctx, record.Key)
		}
	}

	return m.backend.DestroySession(ctx, sessionID)
}

func (m *SessionManager) CleanupWorkflow(ctx context.Context, workflowRunID string) error {
	prefix := fmt.Sprintf("workflow:%s:", workflowRunID)

	var toDelete []string
	var toDeleteKeys []SessionKey
	m.sessions.Range(func(k, v interface{}) bool {
		keyStr := k.(string)
		if strings.HasPrefix(keyStr, prefix) {
			toDelete = append(toDelete, keyStr)
			toDeleteKeys = append(toDeleteKeys, v.(*Session).Key)
		}
		return true
	})

	var errs []error
	for i, keyStr := range toDelete {
		if v, ok := m.sessions.LoadAndDelete(keyStr); ok {
			session := v.(*Session)
			if err := m.backend.DestroySession(ctx, session.ID); err != nil {
				errs = append(errs, fmt.Errorf("destroy session %s: %w", session.ID, err))
			}
			_ = m.store.Delete(ctx, toDeleteKeys[i])
		}
	}

	if m.store != nil {
		_, _ = m.store.DeleteByPrefix(ctx, prefix)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup workflow %s: %d errors, first: %w", workflowRunID, len(errs), errs[0])
	}

	return nil
}

func (m *SessionManager) CleanupAgentRun(ctx context.Context, agentRunID string) error {
	prefix := fmt.Sprintf("agent:%s:", agentRunID)

	var toDelete []string
	var toDeleteKeys []SessionKey
	m.sessions.Range(func(k, v interface{}) bool {
		keyStr := k.(string)
		if strings.HasPrefix(keyStr, prefix) {
			toDelete = append(toDelete, keyStr)
			toDeleteKeys = append(toDeleteKeys, v.(*Session).Key)
		}
		return true
	})

	var errs []error
	for i, keyStr := range toDelete {
		if v, ok := m.sessions.LoadAndDelete(keyStr); ok {
			session := v.(*Session)
			if err := m.backend.DestroySession(ctx, session.ID); err != nil {
				errs = append(errs, fmt.Errorf("destroy session %s: %w", session.ID, err))
			}
			_ = m.store.Delete(ctx, toDeleteKeys[i])
		}
	}

	if m.store != nil {
		_, _ = m.store.DeleteByPrefix(ctx, prefix)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup agent run %s: %d errors, first: %w", agentRunID, len(errs), errs[0])
	}

	return nil
}

func (m *SessionManager) CleanupIdleSessions(ctx context.Context, idleTimeout time.Duration) (int, error) {
	now := time.Now()
	var toDelete []string
	var toDeleteKeys []SessionKey

	m.sessions.Range(func(k, v interface{}) bool {
		session := v.(*Session)
		if now.Sub(session.LastUsedAt) > idleTimeout {
			toDelete = append(toDelete, k.(string))
			toDeleteKeys = append(toDeleteKeys, session.Key)
		}
		return true
	})

	cleaned := 0
	var errs []error
	for i, keyStr := range toDelete {
		if v, ok := m.sessions.LoadAndDelete(keyStr); ok {
			session := v.(*Session)
			if err := m.backend.DestroySession(ctx, session.ID); err != nil {
				errs = append(errs, fmt.Errorf("destroy session %s: %w", session.ID, err))
			} else {
				cleaned++
			}
			_ = m.store.Delete(ctx, toDeleteKeys[i])
		}
	}

	if len(errs) > 0 {
		return cleaned, fmt.Errorf("cleanup idle sessions: %d errors, first: %w", len(errs), errs[0])
	}

	return cleaned, nil
}

func (m *SessionManager) RecoverSessions(ctx context.Context) (int, error) {
	if m.store == nil {
		return 0, nil
	}

	records, err := m.store.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("list stored sessions: %w", err)
	}

	recovered := 0
	for _, record := range records {
		if _, err := m.backend.GetSession(ctx, record.SessionID); err != nil {
			_ = m.store.Delete(ctx, record.Key)
			continue
		}

		session := &Session{
			ID:         record.SessionID,
			Key:        record.Key,
			Image:      record.Image,
			Workdir:    record.Workdir,
			Env:        record.Env,
			Limits:     record.Limits,
			CreatedAt:  record.CreatedAt,
			LastUsedAt: record.LastUsedAt,
		}
		m.sessions.Store(record.Key.String(), session)
		recovered++
	}

	return recovered, nil
}

func (m *SessionManager) ListSessions() []*Session {
	var sessions []*Session
	m.sessions.Range(func(_, v interface{}) bool {
		sessions = append(sessions, v.(*Session))
		return true
	})
	return sessions
}

func (m *SessionManager) Count() int {
	count := 0
	m.sessions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

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
