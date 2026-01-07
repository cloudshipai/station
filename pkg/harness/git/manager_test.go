package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir
}

func TestManager_IsGitRepo(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	if !m.IsGitRepo(ctx) {
		t.Error("should detect git repo")
	}

	nonGitDir := t.TempDir()
	m2 := NewManager(nonGitDir)

	if m2.IsGitRepo(ctx) {
		t.Error("should not detect git repo in non-git directory")
	}
}

func TestManager_GetCurrentBranch(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	branch, err := m.GetCurrentBranch(ctx)
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}

	if branch != "main" && branch != "master" {
		t.Errorf("GetCurrentBranch returned %q, want main or master", branch)
	}
}

func TestManager_CreateBranch(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir, WithBranchPrefix("test/"))

	branchName, err := m.CreateBranch(ctx, "Fix the bug", "agent-1")
	if err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	if !strings.HasPrefix(branchName, "test/fix-the-bug-") {
		t.Errorf("branch name %q doesn't have expected prefix", branchName)
	}

	if !strings.HasSuffix(branchName, "-agent-1") {
		t.Errorf("branch name %q doesn't have expected suffix", branchName)
	}

	currentBranch, _ := m.GetCurrentBranch(ctx)
	if currentBranch != branchName {
		t.Errorf("current branch is %q, want %q", currentBranch, branchName)
	}
}

func TestManager_Commit(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "newfile.txt"), []byte("new content"), 0644)

	sha, err := m.Commit(ctx, "Add new file")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if sha == "" {
		t.Error("commit should return SHA")
	}

	if len(sha) != 40 {
		t.Errorf("SHA length = %d, want 40", len(sha))
	}
}

func TestManager_CommitNoChanges(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	sha, err := m.Commit(ctx, "No changes")
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	if sha != "" {
		t.Error("commit with no changes should return empty SHA")
	}
}

func TestManager_HasUncommittedChanges(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	hasChanges, err := m.HasUncommittedChanges(ctx)
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}
	if hasChanges {
		t.Error("should have no uncommitted changes initially")
	}

	os.WriteFile(filepath.Join(tmpDir, "modified.txt"), []byte("content"), 0644)

	hasChanges, err = m.HasUncommittedChanges(ctx)
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}
	if !hasChanges {
		t.Error("should have uncommitted changes after creating file")
	}
}

func TestManager_GetStatus(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "newfile.txt"), []byte("content"), 0644)

	status, err := m.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !strings.Contains(status, "newfile.txt") {
		t.Errorf("status should contain newfile.txt: %q", status)
	}
}

func TestManager_GetRecentCommits(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	commits, err := m.GetRecentCommits(ctx, 5)
	if err != nil {
		t.Fatalf("GetRecentCommits failed: %v", err)
	}

	if len(commits) != 1 {
		t.Errorf("expected 1 commit, got %d", len(commits))
	}

	if !strings.Contains(commits[0], "Initial commit") {
		t.Errorf("commit message should contain 'Initial commit': %q", commits[0])
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fix the bug", "fix-the-bug"},
		{"Add new feature!", "add-new-feature"},
		{"Update README.md", "update-readmemd"},
		{"  spaces  around  ", "spaces-around"},
		{"UPPERCASE text", "uppercase-text"},
		{"multiple---dashes", "multiple-dashes"},
		{"special@#$chars", "specialchars"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestManager_RestoreOriginalBranch(t *testing.T) {
	tmpDir := setupGitRepo(t)
	ctx := context.Background()

	m := NewManager(tmpDir)

	originalBranch, _ := m.GetCurrentBranch(ctx)

	m.CreateBranch(ctx, "Feature branch", "agent-1")

	currentBranch, _ := m.GetCurrentBranch(ctx)
	if currentBranch == originalBranch {
		t.Error("should be on new branch")
	}

	if err := m.RestoreOriginalBranch(ctx); err != nil {
		t.Fatalf("RestoreOriginalBranch failed: %v", err)
	}

	currentBranch, _ = m.GetCurrentBranch(ctx)
	if currentBranch != originalBranch {
		t.Errorf("should be back on original branch %q, got %q", originalBranch, currentBranch)
	}
}
