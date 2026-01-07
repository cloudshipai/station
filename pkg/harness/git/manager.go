package git

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Manager struct {
	workspacePath  string
	branchPrefix   string
	currentBranch  string
	originalBranch string
}

type ManagerOption func(*Manager)

func WithBranchPrefix(prefix string) ManagerOption {
	return func(m *Manager) {
		m.branchPrefix = prefix
	}
}

func NewManager(workspacePath string, opts ...ManagerOption) *Manager {
	m := &Manager{
		workspacePath: workspacePath,
		branchPrefix:  "agent/",
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func (m *Manager) CreateBranch(ctx context.Context, task string, agentID string) (string, error) {
	original, err := m.GetCurrentBranch(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	m.originalBranch = original

	slug := slugify(task)
	if len(slug) > 30 {
		slug = slug[:30]
	}

	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("%s%s-%s-%s", m.branchPrefix, slug, timestamp, agentID)

	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create branch: %s - %w", string(output), err)
	}

	m.currentBranch = branchName
	return branchName, nil
}

func (m *Manager) SwitchToBranch(ctx context.Context, branchName string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", branchName)
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch branch: %s - %w", string(output), err)
	}

	m.currentBranch = branchName
	return nil
}

func (m *Manager) Commit(ctx context.Context, message string) (string, error) {
	addCmd := exec.CommandContext(ctx, "git", "add", "-A")
	addCmd.Dir = m.workspacePath

	if output, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to stage changes: %s - %w", string(output), err)
	}

	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	statusCmd.Dir = m.workspacePath

	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to check status: %w", err)
	}

	if len(strings.TrimSpace(string(statusOutput))) == 0 {
		return "", nil
	}

	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message)
	commitCmd.Dir = m.workspacePath

	if output, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to commit: %s - %w", string(output), err)
	}

	shaCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	shaCmd.Dir = m.workspacePath

	shaOutput, err := shaCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get commit SHA: %w", err)
	}

	return strings.TrimSpace(string(shaOutput)), nil
}

func (m *Manager) Push(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "push", "-u", "origin", m.currentBranch)
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push: %s - %w", string(output), err)
	}

	return nil
}

func (m *Manager) GetCurrentBranch(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %s - %w", string(output), err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (m *Manager) GetStatus(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--short")
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %s - %w", string(output), err)
	}

	return string(output), nil
}

func (m *Manager) GetDiff(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %s - %w", string(output), err)
	}

	return string(output), nil
}

func (m *Manager) RestoreOriginalBranch(ctx context.Context) error {
	if m.originalBranch == "" {
		return nil
	}

	return m.SwitchToBranch(ctx, m.originalBranch)
}

func (m *Manager) HasUncommittedChanges(ctx context.Context) (bool, error) {
	status, err := m.GetStatus(ctx)
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(status)) > 0, nil
}

func (m *Manager) IsGitRepo(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	cmd.Dir = m.workspacePath

	return cmd.Run() == nil
}

func (m *Manager) GetRecentCommits(ctx context.Context, count int) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("-%d", count), "--oneline")
	cmd.Dir = m.workspacePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %s - %w", string(output), err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}, nil
	}

	return lines, nil
}

func slugify(s string) string {
	s = strings.ToLower(s)

	reg := regexp.MustCompile(`[^a-z0-9\s-]`)
	s = reg.ReplaceAllString(s, "")

	reg = regexp.MustCompile(`[\s_]+`)
	s = reg.ReplaceAllString(s, "-")

	reg = regexp.MustCompile(`-+`)
	s = reg.ReplaceAllString(s, "-")

	s = strings.Trim(s, "-")

	return s
}
