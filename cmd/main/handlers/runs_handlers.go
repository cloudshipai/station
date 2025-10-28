package handlers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/theme"
)

// RunsHandler handles runs-related CLI commands
type RunsHandler struct {
	themeManager *theme.ThemeManager
}

func NewRunsHandler(themeManager *theme.ThemeManager) *RunsHandler {
	return &RunsHandler{themeManager: themeManager}
}

// RunRunsList lists agent runs
func (h *RunsHandler) RunRunsList(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🏃 Agent Runs")
	fmt.Println(banner)

	limit, _ := cmd.Flags().GetInt("limit")

	fmt.Println(styles.Info.Render("🏠 Listing local runs"))
	return h.listRunsLocal(limit)
}

// RunRunsInspect inspects a specific run
func (h *RunsHandler) RunRunsInspect(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("run ID is required")
	}

	runID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid run ID: %s", args[0])
	}

	verbose, _ := cmd.Flags().GetBool("verbose")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🔍 Run Details")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("🏠 Getting local run"))
	return h.inspectRunLocal(runID, verbose)
}

// Local operations

func (h *RunsHandler) listRunsLocal(limit int) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	
	// Use provided limit or default to 50
	if limit <= 0 {
		limit = 50
	}
	
	runs, err := repos.AgentRuns.ListRecent(context.Background(), int64(limit))
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	if len(runs) == 0 {
		fmt.Println("• No runs found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d recent run(s):\n", len(runs))
	for _, run := range runs {
		statusIcon := h.getStatusIcon(run.Status)
		fmt.Printf("• Run %d: %s %s", run.ID, statusIcon, styles.Success.Render(run.AgentName))
		fmt.Printf(" [%s]", run.StartedAt.Format("Jan 2 15:04"))
		if run.CompletedAt != nil {
			duration := run.CompletedAt.Sub(run.StartedAt)
			fmt.Printf(" (%.1fs)", duration.Seconds())
		}
		fmt.Printf("\n  Task: %s\n", h.truncateString(run.Task, 80))
	}

	return nil
}

func (h *RunsHandler) inspectRunLocal(runID int64, verbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)
	run, err := repos.AgentRuns.GetByIDWithDetails(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("run with ID %d not found", runID)
	}

	styles := getCLIStyles(h.themeManager)
	statusIcon := h.getStatusIcon(run.Status)
	
	fmt.Printf("Run: %d %s\n", run.ID, statusIcon)
	fmt.Printf("Agent: %s (ID: %d)\n", styles.Success.Render(run.AgentName), run.AgentID)
	fmt.Printf("Status: %s\n", h.colorizeStatus(run.Status))
	if run.Error != nil && *run.Error != "" {
		fmt.Printf("Error: %s\n", styles.Error.Render(*run.Error))
	}
	fmt.Printf("Task: %s\n", run.Task)
	fmt.Printf("Started: %s\n", run.StartedAt.Format("Jan 2, 2006 15:04:05"))
	
	if run.CompletedAt != nil {
		duration := run.CompletedAt.Sub(run.StartedAt)
		fmt.Printf("Completed: %s (Duration: %.1fs)\n", run.CompletedAt.Format("Jan 2, 2006 15:04:05"), duration.Seconds())
	} else {
		fmt.Printf("Status: Running...\n")
	}
	
	fmt.Printf("Steps Taken: %d\n", run.StepsTaken)
	
	if verbose {
		// Show comprehensive details in verbose mode
		fmt.Print("\n" + styles.Banner.Render("📊 Detailed Run Information") + "\n")
		
		
		// Agent Information
		fmt.Printf("\n🤖 Agent Details:\n")
		fmt.Printf("• Agent ID: %d\n", run.AgentID)
		fmt.Printf("• Agent Name: %s\n", run.AgentName)
		fmt.Printf("• User: %s\n", run.Username)
		
		// Execution Metadata
		fmt.Printf("\n⚡ Execution Metadata:\n")
		fmt.Printf("• Run ID: %d\n", run.ID)
		fmt.Printf("• Status: %s\n", h.colorizeStatus(run.Status))
		fmt.Printf("• Steps Taken: %d\n", run.StepsTaken)
		fmt.Printf("• Started At: %s\n", run.StartedAt.Format("Jan 2, 2006 15:04:05 MST"))
		if run.CompletedAt != nil {
			duration := run.CompletedAt.Sub(run.StartedAt)
			fmt.Printf("• Completed At: %s\n", run.CompletedAt.Format("Jan 2, 2006 15:04:05 MST"))
			fmt.Printf("• Total Duration: %.2fs\n", duration.Seconds())
		}
		
		// Task Information
		fmt.Printf("\n📋 Task:\n")
		fmt.Printf("%s\n", run.Task)

		// Final Response
		if run.FinalResponse != "" {
			fmt.Printf("\n💬 Final Response:\n")
			fmt.Printf("─────────────────────────────────────────────────\n")
			fmt.Printf("%s\n", run.FinalResponse)
			fmt.Printf("─────────────────────────────────────────────────\n")
		}
	} else {
		// Show limited details in non-verbose mode
		if run.FinalResponse != "" {
			fmt.Printf("\nResponse:\n%s\n", run.FinalResponse)
		}
	}

	return nil
}


// Helper functions

func (h *RunsHandler) getStatusIcon(status string) string {
	switch status {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "running":
		return "🔄"
	case "pending":
		return "⏳"
	default:
		return "❓"
	}
}

func (h *RunsHandler) colorizeStatus(status string) string {
	styles := getCLIStyles(h.themeManager)
	switch status {
	case "completed":
		return styles.Success.Render(status)
	case "failed":
		return styles.Error.Render(status)
	case "running":
		return styles.Info.Render(status)
	default:
		return status
	}
}

func (h *RunsHandler) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}