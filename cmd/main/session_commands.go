package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"station/internal/config"
	"station/pkg/harness/session"

	"github.com/spf13/cobra"
)

var (
	sessionCmd = &cobra.Command{
		Use:   "session",
		Short: "Manage harness workspace sessions",
		Long:  `Manage persistent workspace sessions for agentic harness executions.`,
	}

	sessionListCmd = &cobra.Command{
		Use:   "list",
		Short: "List all sessions",
		RunE:  runSessionList,
	}

	sessionInfoCmd = &cobra.Command{
		Use:   "info <session-id>",
		Short: "Show session details",
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionInfo,
	}

	sessionDeleteCmd = &cobra.Command{
		Use:   "delete <session-id>",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionDelete,
	}

	sessionCleanupCmd = &cobra.Command{
		Use:   "cleanup",
		Short: "Remove stale sessions",
		RunE:  runSessionCleanup,
	}

	sessionUnlockCmd = &cobra.Command{
		Use:   "unlock <session-id>",
		Short: "Force unlock a session",
		Args:  cobra.ExactArgs(1),
		RunE:  runSessionUnlock,
	}
)

var (
	cleanupOlderThan string
	cleanupDryRun    bool
	deleteForce      bool
)

func init() {
	sessionCleanupCmd.Flags().StringVar(&cleanupOlderThan, "older-than", "7d", "Delete sessions older than duration (e.g., 7d, 24h)")
	sessionCleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would be deleted without actually deleting")

	sessionDeleteCmd.Flags().BoolVar(&deleteForce, "force", false, "Force delete even if session is locked")

	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionInfoCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionCleanupCmd)
	sessionCmd.AddCommand(sessionUnlockCmd)
}

func getSessionManager() *session.Manager {
	cfg := config.GetLoadedConfig()
	basePath := "./workspace"
	if cfg != nil && cfg.Harness.Workspace.Path != "" {
		basePath = cfg.Harness.Workspace.Path
	}
	if !filepath.IsAbs(basePath) {
		if cfg != nil && cfg.Workspace != "" {
			basePath = filepath.Join(cfg.Workspace, basePath)
		}
	}
	return session.NewManager(basePath)
}

func runSessionList(cmd *cobra.Command, args []string) error {
	mgr := getSessionManager()
	ctx := context.Background()

	sessions, err := mgr.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION ID\tSTATUS\tLAST USED\tRUNS\tREPO\tSIZE")
	fmt.Fprintln(w, "----------\t------\t---------\t----\t----\t----")

	for _, s := range sessions {
		status := "ready"
		if s.IsLocked {
			status = fmt.Sprintf("locked (%s)", s.LockedBy)
		}

		lastUsed := formatSessionDuration(time.Since(s.LastUsedAt))
		repo := s.RepoURL
		if len(repo) > 40 {
			repo = "..." + repo[len(repo)-37:]
		}
		if repo == "" {
			repo = "-"
		}

		size, _ := mgr.DiskUsage(s.ID)
		sizeStr := formatSessionSize(size)

		fmt.Fprintf(w, "%s\t%s\t%s ago\t%d\t%s\t%s\n",
			s.ID, status, lastUsed, s.TotalRuns, repo, sizeStr)
	}
	w.Flush()

	return nil
}

func runSessionInfo(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	mgr := getSessionManager()
	ctx := context.Background()

	s, err := mgr.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	size, _ := mgr.DiskUsage(s.ID)

	fmt.Printf("Session: %s\n", s.ID)
	fmt.Printf("Path: %s\n", s.Path)
	fmt.Printf("Created: %s\n", s.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Last Used: %s (%s ago)\n", s.LastUsedAt.Format(time.RFC3339), formatSessionDuration(time.Since(s.LastUsedAt)))
	fmt.Printf("Total Runs: %d\n", s.TotalRuns)
	fmt.Printf("Disk Usage: %s\n", formatSessionSize(size))

	if s.RepoURL != "" {
		fmt.Printf("Repository: %s\n", s.RepoURL)
	}
	if s.Branch != "" {
		fmt.Printf("Branch: %s\n", s.Branch)
	}

	if s.IsLocked {
		fmt.Printf("Status: LOCKED\n")
		fmt.Printf("Locked By: %s\n", s.LockedBy)
		fmt.Printf("Locked At: %s\n", s.LockedAt.Format(time.RFC3339))
	} else {
		fmt.Printf("Status: Ready\n")
	}

	return nil
}

func runSessionDelete(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	mgr := getSessionManager()
	ctx := context.Background()

	s, err := mgr.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if s.IsLocked && !deleteForce {
		return fmt.Errorf("session is locked by %s (use --force to override)", s.LockedBy)
	}

	if s.IsLocked && deleteForce {
		if err := mgr.ForceUnlock(ctx, sessionID); err != nil {
			return fmt.Errorf("failed to force unlock: %w", err)
		}
	}

	if err := mgr.Delete(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	fmt.Printf("Deleted session: %s\n", sessionID)
	return nil
}

func runSessionCleanup(cmd *cobra.Command, args []string) error {
	duration, err := parseDuration(cleanupOlderThan)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	mgr := getSessionManager()
	ctx := context.Background()

	sessions, err := mgr.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	cutoff := time.Now().Add(-duration)
	var toDelete []*session.Session

	for _, s := range sessions {
		if s.LastUsedAt.Before(cutoff) && !s.IsLocked {
			toDelete = append(toDelete, s)
		}
	}

	if len(toDelete) == 0 {
		fmt.Println("No sessions to clean up.")
		return nil
	}

	if cleanupDryRun {
		fmt.Printf("Would delete %d session(s):\n", len(toDelete))
		for _, s := range toDelete {
			size, _ := mgr.DiskUsage(s.ID)
			fmt.Printf("  - %s (last used: %s, size: %s)\n",
				s.ID, formatSessionDuration(time.Since(s.LastUsedAt)), formatSessionSize(size))
		}
		return nil
	}

	deleted, err := mgr.Cleanup(ctx, duration)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	fmt.Printf("Deleted %d session(s):\n", len(deleted))
	for _, id := range deleted {
		fmt.Printf("  - %s\n", id)
	}

	return nil
}

func runSessionUnlock(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	mgr := getSessionManager()
	ctx := context.Background()

	s, err := mgr.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if !s.IsLocked {
		fmt.Printf("Session %s is not locked.\n", sessionID)
		return nil
	}

	if err := mgr.ForceUnlock(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to unlock session: %w", err)
	}

	fmt.Printf("Unlocked session: %s (was locked by %s)\n", sessionID, s.LockedBy)
	return nil
}

func formatSessionDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

func formatSessionSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, fmt.Errorf("invalid days: %s", days)
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
