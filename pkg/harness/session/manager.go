package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Manager struct {
	basePath    string
	lockTimeout time.Duration
}

type Session struct {
	ID         string    `json:"id"`
	Path       string    `json:"path"`
	RepoURL    string    `json:"repo_url,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	TotalRuns  int       `json:"total_runs"`
	IsLocked   bool      `json:"is_locked"`
	LockedBy   string    `json:"locked_by,omitempty"`
	LockedAt   time.Time `json:"locked_at,omitempty"`
}

type LockInfo struct {
	RunID      string    `json:"run_id"`
	PID        int       `json:"pid"`
	Hostname   string    `json:"hostname"`
	AcquiredAt time.Time `json:"acquired_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

type SessionMeta struct {
	ID         string    `json:"id"`
	RepoURL    string    `json:"repo_url,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	TotalRuns  int       `json:"total_runs"`
}

func NewManager(basePath string) *Manager {
	return &Manager{
		basePath:    basePath,
		lockTimeout: 1 * time.Hour,
	}
}

func (m *Manager) SetLockTimeout(d time.Duration) {
	m.lockTimeout = d
}

func (m *Manager) SessionPath(sessionID string) string {
	return filepath.Join(m.basePath, "session", sessionID)
}

func (m *Manager) lockPath(sessionID string) string {
	return filepath.Join(m.SessionPath(sessionID), ".session.lock")
}

func (m *Manager) metaPath(sessionID string) string {
	return filepath.Join(m.SessionPath(sessionID), ".session.meta")
}

func (m *Manager) Create(ctx context.Context, sessionID string, repoURL string, branch string) (*Session, error) {
	sessionPath := m.SessionPath(sessionID)

	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	meta := SessionMeta{
		ID:         sessionID,
		RepoURL:    repoURL,
		Branch:     branch,
		CreatedAt:  time.Now(),
		LastUsedAt: time.Now(),
		TotalRuns:  0,
	}

	if err := m.writeMeta(sessionID, &meta); err != nil {
		return nil, err
	}

	return &Session{
		ID:         sessionID,
		Path:       sessionPath,
		RepoURL:    repoURL,
		Branch:     branch,
		CreatedAt:  meta.CreatedAt,
		LastUsedAt: meta.LastUsedAt,
		TotalRuns:  0,
		IsLocked:   false,
	}, nil
}

func (m *Manager) Get(ctx context.Context, sessionID string) (*Session, error) {
	sessionPath := m.SessionPath(sessionID)

	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	meta, err := m.readMeta(sessionID)
	if err != nil {
		meta = &SessionMeta{
			ID:        sessionID,
			CreatedAt: time.Now(),
		}
	}

	session := &Session{
		ID:         sessionID,
		Path:       sessionPath,
		RepoURL:    meta.RepoURL,
		Branch:     meta.Branch,
		CreatedAt:  meta.CreatedAt,
		LastUsedAt: meta.LastUsedAt,
		TotalRuns:  meta.TotalRuns,
	}

	lockInfo, err := m.getLockInfo(sessionID)
	if err == nil && lockInfo != nil {
		if time.Now().Before(lockInfo.ExpiresAt) && m.isProcessAlive(lockInfo.PID) {
			session.IsLocked = true
			session.LockedBy = lockInfo.RunID
			session.LockedAt = lockInfo.AcquiredAt
		} else {
			_ = os.Remove(m.lockPath(sessionID))
		}
	}

	return session, nil
}

func (m *Manager) List(ctx context.Context) ([]*Session, error) {
	sessionDir := filepath.Join(m.basePath, "session")

	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		return []*Session{}, nil
	}

	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		session, err := m.Get(ctx, entry.Name())
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	session, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.IsLocked {
		return fmt.Errorf("cannot delete locked session (locked by %s)", session.LockedBy)
	}

	return os.RemoveAll(session.Path)
}

func (m *Manager) Cleanup(ctx context.Context, olderThan time.Duration) ([]string, error) {
	sessions, err := m.List(ctx)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-olderThan)
	var deleted []string

	for _, session := range sessions {
		if session.LastUsedAt.Before(cutoff) && !session.IsLocked {
			if err := os.RemoveAll(session.Path); err == nil {
				deleted = append(deleted, session.ID)
			}
		}
	}

	return deleted, nil
}

func (m *Manager) AcquireLock(ctx context.Context, sessionID string, runID string) error {
	session, err := m.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.IsLocked {
		return fmt.Errorf("session already locked by run %s", session.LockedBy)
	}

	hostname, _ := os.Hostname()
	lockInfo := LockInfo{
		RunID:      runID,
		PID:        os.Getpid(),
		Hostname:   hostname,
		AcquiredAt: time.Now(),
		ExpiresAt:  time.Now().Add(m.lockTimeout),
	}

	data, err := json.MarshalIndent(lockInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lock info: %w", err)
	}

	lockPath := m.lockPath(sessionID)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("session lock already exists")
		}
		return fmt.Errorf("failed to create lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(lockPath)
		return fmt.Errorf("failed to write lock file: %w", err)
	}

	meta, _ := m.readMeta(sessionID)
	if meta != nil {
		meta.LastUsedAt = time.Now()
		meta.TotalRuns++
		_ = m.writeMeta(sessionID, meta)
	}

	return nil
}

func (m *Manager) ReleaseLock(ctx context.Context, sessionID string, runID string) error {
	lockInfo, err := m.getLockInfo(sessionID)
	if err != nil {
		return nil
	}

	if lockInfo.RunID != runID {
		return fmt.Errorf("lock held by different run: %s", lockInfo.RunID)
	}

	return os.Remove(m.lockPath(sessionID))
}

func (m *Manager) ForceUnlock(ctx context.Context, sessionID string) error {
	return os.Remove(m.lockPath(sessionID))
}

func (m *Manager) RefreshLock(ctx context.Context, sessionID string, runID string) error {
	lockInfo, err := m.getLockInfo(sessionID)
	if err != nil {
		return fmt.Errorf("no lock to refresh: %w", err)
	}

	if lockInfo.RunID != runID {
		return fmt.Errorf("lock held by different run: %s", lockInfo.RunID)
	}

	lockInfo.ExpiresAt = time.Now().Add(m.lockTimeout)

	data, err := json.MarshalIndent(lockInfo, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.lockPath(sessionID), data, 0644)
}

func (m *Manager) GetOrCreate(ctx context.Context, sessionID string, repoURL string, branch string) (*Session, error) {
	session, err := m.Get(ctx, sessionID)
	if err == nil {
		return session, nil
	}

	return m.Create(ctx, sessionID, repoURL, branch)
}

func (m *Manager) getLockInfo(sessionID string) (*LockInfo, error) {
	data, err := os.ReadFile(m.lockPath(sessionID))
	if err != nil {
		return nil, err
	}

	var lockInfo LockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, err
	}

	return &lockInfo, nil
}

func (m *Manager) readMeta(sessionID string) (*SessionMeta, error) {
	data, err := os.ReadFile(m.metaPath(sessionID))
	if err != nil {
		return nil, err
	}

	var meta SessionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func (m *Manager) writeMeta(sessionID string, meta *SessionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.metaPath(sessionID), data, 0644)
}

func (m *Manager) isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func (m *Manager) DiskUsage(sessionID string) (int64, error) {
	var size int64

	err := filepath.Walk(m.SessionPath(sessionID), func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}
