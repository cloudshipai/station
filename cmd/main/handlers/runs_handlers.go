package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/theme"
	"station/pkg/models"
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

	endpoint, _ := cmd.Flags().GetString("endpoint")
	limit, _ := cmd.Flags().GetInt("limit")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing runs from: " + endpoint))
		return h.listRunsRemote(endpoint, limit)
	} else {
		fmt.Println(styles.Info.Render("🏠 Listing local runs"))
		return h.listRunsLocal(limit)
	}
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

	endpoint, _ := cmd.Flags().GetString("endpoint")
	verbose, _ := cmd.Flags().GetBool("verbose")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🔍 Run Details")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Getting run from: " + endpoint))
		return h.inspectRunRemote(runID, endpoint, verbose)
	} else {
		fmt.Println(styles.Info.Render("🏠 Getting local run"))
		return h.inspectRunLocal(runID, verbose)
	}
}

// Local operations

func (h *RunsHandler) listRunsLocal(limit int) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// Use provided limit or default to 50
	if limit <= 0 {
		limit = 50
	}
	
	runs, err := repos.AgentRuns.ListRecent(int64(limit))
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
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	run, err := repos.AgentRuns.GetByIDWithDetails(runID)
	if err != nil {
		return fmt.Errorf("run with ID %d not found", runID)
	}

	styles := getCLIStyles(h.themeManager)
	statusIcon := h.getStatusIcon(run.Status)
	
	fmt.Printf("Run: %d %s\n", run.ID, statusIcon)
	fmt.Printf("Agent: %s (ID: %d)\n", styles.Success.Render(run.AgentName), run.AgentID)
	fmt.Printf("Status: %s\n", h.colorizeStatus(run.Status))
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
		
		// Tool Calls - Show all in verbose mode
		if run.ToolCalls != nil && len(*run.ToolCalls) > 0 {
			fmt.Printf("\n🔧 Tool Calls (%d):\n", len(*run.ToolCalls))
			for i, toolCall := range *run.ToolCalls {
				toolCallBytes, err := json.MarshalIndent(toolCall, "  ", "  ")
				if err == nil {
					fmt.Printf("  %d. %s\n", i+1, string(toolCallBytes))
				} else {
					fmt.Printf("  %d. %v\n", i+1, toolCall)
				}
			}
		}

		// Execution Steps - Show all in verbose mode
		if run.ExecutionSteps != nil && len(*run.ExecutionSteps) > 0 {
			fmt.Printf("\n📝 Execution Steps (%d):\n", len(*run.ExecutionSteps))
			for i, step := range *run.ExecutionSteps {
				stepBytes, err := json.MarshalIndent(step, "  ", "  ")
				if err == nil {
					fmt.Printf("  %d. %s\n", i+1, string(stepBytes))
				} else {
					fmt.Printf("  %d. %v\n", i+1, step)
				}
			}
		}
		
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
		
		if run.ToolCalls != nil && len(*run.ToolCalls) > 0 {
			fmt.Printf("\nTool Calls (%d):\n", len(*run.ToolCalls))
			for i, toolCall := range *run.ToolCalls {
				if i >= 3 { // Show only first 3 in non-verbose mode
					fmt.Printf("... and %d more (use -v for all)\n", len(*run.ToolCalls)-3)
					break
				}
				fmt.Printf("• %v\n", toolCall)
			}
		}

		// Show execution steps if available (limited)
		if run.ExecutionSteps != nil && len(*run.ExecutionSteps) > 0 {
			fmt.Printf("\nExecution Steps (%d):\n", len(*run.ExecutionSteps))
			for i, step := range *run.ExecutionSteps {
				if i >= 2 { // Show only first 2 in non-verbose mode
					fmt.Printf("... and %d more (use -v for all)\n", len(*run.ExecutionSteps)-2)
					break
				}
				fmt.Printf("• %v\n", step)
			}
		}
	}

	return nil
}

// Remote operations

func (h *RunsHandler) listRunsRemote(endpoint string, limit int) error {
	url := fmt.Sprintf("%s/api/v1/runs", endpoint)
	if limit > 0 {
		url += fmt.Sprintf("?limit=%d", limit)
	}
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Runs  []*models.AgentRunWithDetails `json:"runs"`
		Count int                           `json:"count"`
		Limit int64                         `json:"limit"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("• No runs found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d recent run(s):\n", result.Count)
	for _, run := range result.Runs {
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

func (h *RunsHandler) inspectRunRemote(runID int64, endpoint string, verbose bool) error {
	url := fmt.Sprintf("%s/api/v1/runs/%d", endpoint, runID)
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Run *models.AgentRunWithDetails `json:"run"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	run := result.Run
	styles := getCLIStyles(h.themeManager)
	statusIcon := h.getStatusIcon(run.Status)
	
	fmt.Printf("Run: %d %s\n", run.ID, statusIcon)
	fmt.Printf("Agent: %s (ID: %d)\n", styles.Success.Render(run.AgentName), run.AgentID)
	fmt.Printf("Status: %s\n", h.colorizeStatus(run.Status))
	fmt.Printf("Task: %s\n", run.Task)
	fmt.Printf("Started: %s\n", run.StartedAt.Format("Jan 2, 2006 15:04:05"))
	
	if run.CompletedAt != nil {
		duration := run.CompletedAt.Sub(run.StartedAt)
		fmt.Printf("Completed: %s (Duration: %.1fs)\n", run.CompletedAt.Format("Jan 2, 2006 15:04:05"), duration.Seconds())
	} else {
		fmt.Printf("Status: Running...\n")
	}
	
	fmt.Printf("Steps Taken: %d\n", run.StepsTaken)
	
	if run.FinalResponse != "" {
		fmt.Printf("\nResponse:\n%s\n", run.FinalResponse)
	}

	// Show tool calls if available
	if run.ToolCalls != nil && len(*run.ToolCalls) > 0 {
		fmt.Printf("\nTool Calls (%d):\n", len(*run.ToolCalls))
		for i, toolCall := range *run.ToolCalls {
			if i >= 10 { // Limit to first 10 tool calls
				fmt.Printf("... and %d more\n", len(*run.ToolCalls)-10)
				break
			}
			fmt.Printf("• %v\n", toolCall)
		}
	}

	// Show execution steps if available
	if run.ExecutionSteps != nil && len(*run.ExecutionSteps) > 0 {
		fmt.Printf("\nExecution Steps (%d):\n", len(*run.ExecutionSteps))
		for i, step := range *run.ExecutionSteps {
			if i >= 5 { // Limit to first 5 steps
				fmt.Printf("... and %d more\n", len(*run.ExecutionSteps)-5)
				break
			}
			fmt.Printf("• %v\n", step)
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