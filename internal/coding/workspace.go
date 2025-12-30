package coding

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type CleanupPolicy string

const (
	CleanupOnSessionEnd CleanupPolicy = "on_session_end"
	CleanupOnSuccess    CleanupPolicy = "on_success"
	CleanupManual       CleanupPolicy = "manual"
)

type SessionScope string

const (
	ScopeAgent    SessionScope = "agent"
	ScopeWorkflow SessionScope = "workflow"
)

type Workspace struct {
	ID             string        `json:"id"`
	Path           string        `json:"path"`
	Scope          SessionScope  `json:"scope"`
	CleanupPolicy  CleanupPolicy `json:"cleanup_policy"`
	CreatedAt      time.Time     `json:"created_at"`
	SessionID      string        `json:"session_id,omitempty"`
	WorkflowID     string        `json:"workflow_id,omitempty"`
	GitInitialized bool          `json:"git_initialized"`
}

type WorkspaceManager struct {
	basePath       string
	cleanupPolicy  CleanupPolicy
	gitCredentials *GitCredentials
	mu             sync.RWMutex
	workspaces     map[string]*Workspace
}

type WorkspaceManagerOption func(*WorkspaceManager)

func WithBasePath(path string) WorkspaceManagerOption {
	return func(m *WorkspaceManager) { m.basePath = path }
}

func WithCleanupPolicy(policy CleanupPolicy) WorkspaceManagerOption {
	return func(m *WorkspaceManager) { m.cleanupPolicy = policy }
}

func WithGitCredentials(creds *GitCredentials) WorkspaceManagerOption {
	return func(m *WorkspaceManager) { m.gitCredentials = creds }
}

func NewWorkspaceManager(opts ...WorkspaceManagerOption) *WorkspaceManager {
	m := &WorkspaceManager{
		basePath:      filepath.Join(os.TempDir(), "station-coding"),
		cleanupPolicy: CleanupOnSessionEnd,
		workspaces:    make(map[string]*Workspace),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *WorkspaceManager) Create(ctx context.Context, scope SessionScope, scopeID string) (*Workspace, error) {
	if err := os.MkdirAll(m.basePath, 0755); err != nil {
		return nil, fmt.Errorf("create base path: %w", err)
	}

	id := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	path := filepath.Join(m.basePath, id)

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	ws := &Workspace{
		ID:            id,
		Path:          path,
		Scope:         scope,
		CleanupPolicy: m.cleanupPolicy,
		CreatedAt:     time.Now(),
	}

	if scope == ScopeAgent {
		ws.SessionID = scopeID
	} else {
		ws.WorkflowID = scopeID
	}

	m.mu.Lock()
	m.workspaces[id] = ws
	m.mu.Unlock()

	return ws, nil
}

func (m *WorkspaceManager) Get(id string) (*Workspace, error) {
	m.mu.RLock()
	ws, ok := m.workspaces[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", id)
	}
	return ws, nil
}

func (m *WorkspaceManager) GetByScope(scope SessionScope, scopeID string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ws := range m.workspaces {
		if ws.Scope != scope {
			continue
		}
		if scope == ScopeAgent && ws.SessionID == scopeID {
			return ws, nil
		}
		if scope == ScopeWorkflow && ws.WorkflowID == scopeID {
			return ws, nil
		}
	}
	return nil, fmt.Errorf("workspace not found for %s: %s", scope, scopeID)
}

func (m *WorkspaceManager) InitGit(ctx context.Context, ws *Workspace) error {
	if ws.GitInitialized {
		return nil
	}

	initCmd := exec.CommandContext(ctx, "git", "init")
	initCmd.Dir = ws.Path
	if err := initCmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	configEmail := exec.CommandContext(ctx, "git", "config", "user.email", "station@localhost")
	configEmail.Dir = ws.Path
	configEmail.Run()

	configName := exec.CommandContext(ctx, "git", "config", "user.name", "Station")
	configName.Dir = ws.Path
	configName.Run()

	m.mu.Lock()
	ws.GitInitialized = true
	m.mu.Unlock()

	return nil
}

func (m *WorkspaceManager) CollectChanges(ctx context.Context, ws *Workspace) ([]FileChange, error) {
	if !ws.GitInitialized {
		return m.collectChangesWithoutGit(ws)
	}

	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = ws.Path
	output, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	var changes []FileChange
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		path := strings.TrimSpace(line[3:])

		var action string
		switch {
		case strings.Contains(status, "A") || strings.Contains(status, "?"):
			action = "created"
		case strings.Contains(status, "M"):
			action = "modified"
		case strings.Contains(status, "D"):
			action = "deleted"
		default:
			action = "modified"
		}

		change := FileChange{
			Path:   path,
			Action: action,
		}

		if action != "deleted" {
			added, removed := m.getFileDiffStats(ctx, ws.Path, path)
			change.LinesAdded = added
			change.LinesRemoved = removed
		}

		changes = append(changes, change)
	}

	return changes, nil
}

func (m *WorkspaceManager) getFileDiffStats(ctx context.Context, workDir, path string) (added, removed int) {
	diffCmd := exec.CommandContext(ctx, "git", "diff", "--numstat", "--", path)
	diffCmd.Dir = workDir
	output, err := diffCmd.Output()
	if err != nil {
		diffCmd = exec.CommandContext(ctx, "git", "diff", "--numstat", "--cached", "--", path)
		diffCmd.Dir = workDir
		output, _ = diffCmd.Output()
	}

	if len(output) == 0 {
		return 0, 0
	}

	parts := strings.Fields(string(output))
	if len(parts) >= 2 {
		added, _ = strconv.Atoi(parts[0])
		removed, _ = strconv.Atoi(parts[1])
	}
	return
}

func (m *WorkspaceManager) collectChangesWithoutGit(ws *Workspace) ([]FileChange, error) {
	var changes []FileChange

	err := filepath.Walk(ws.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(ws.Path, path)
		changes = append(changes, FileChange{
			Path:   relPath,
			Action: "created",
		})
		return nil
	})

	return changes, err
}

func (m *WorkspaceManager) Cleanup(ctx context.Context, ws *Workspace) error {
	m.mu.Lock()
	delete(m.workspaces, ws.ID)
	m.mu.Unlock()

	return os.RemoveAll(ws.Path)
}

func (m *WorkspaceManager) CleanupByPolicy(ctx context.Context, ws *Workspace, success bool) error {
	switch ws.CleanupPolicy {
	case CleanupOnSessionEnd:
		return m.Cleanup(ctx, ws)
	case CleanupOnSuccess:
		if success {
			return m.Cleanup(ctx, ws)
		}
		return nil
	case CleanupManual:
		return nil
	default:
		return m.Cleanup(ctx, ws)
	}
}

func (m *WorkspaceManager) CleanupAll(ctx context.Context) error {
	m.mu.Lock()
	workspaces := make([]*Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		workspaces = append(workspaces, ws)
	}
	m.mu.Unlock()

	var errs []error
	for _, ws := range workspaces {
		if err := m.Cleanup(ctx, ws); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

func (m *WorkspaceManager) CloneRepo(ctx context.Context, ws *Workspace, repoURL, branch string) error {
	cloneURL := repoURL
	if m.gitCredentials != nil && m.gitCredentials.HasToken() {
		cloneURL = m.gitCredentials.InjectCredentials(repoURL)
	}

	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, cloneURL, ".")

	cloneCmd := exec.CommandContext(ctx, "git", args...)
	cloneCmd.Dir = ws.Path

	var stderr bytes.Buffer
	cloneCmd.Stderr = &stderr

	if err := cloneCmd.Run(); err != nil {
		return RedactError(fmt.Errorf("git clone %s: %s", repoURL, stderr.String()))
	}

	m.mu.Lock()
	ws.GitInitialized = true
	m.mu.Unlock()

	return nil
}

func (m *WorkspaceManager) GetGitCredentials() *GitCredentials {
	return m.gitCredentials
}

func (m *WorkspaceManager) GetCommitsSince(ctx context.Context, ws *Workspace, since string) ([]string, error) {
	if !ws.GitInitialized {
		return nil, nil
	}

	args := []string{"log", "--oneline"}
	if since != "" {
		args = append(args, since+"..HEAD")
	}

	logCmd := exec.CommandContext(ctx, "git", args...)
	logCmd.Dir = ws.Path
	output, err := logCmd.Output()
	if err != nil {
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []string
	hashRegex := regexp.MustCompile(`^[a-f0-9]+`)

	for _, line := range lines {
		if line == "" {
			continue
		}
		if match := hashRegex.FindString(line); match != "" {
			commits = append(commits, match)
		}
	}

	return commits, nil
}

func (m *WorkspaceManager) ListWorkspaces() []*Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		result = append(result, ws)
	}
	return result
}
