package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"station/cmd/main/handlers/common"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/queries"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/internal/telemetry"
	"station/internal/theme"
)

// ReportHandler handles report-related CLI commands
type ReportHandler struct {
	themeManager     *theme.ThemeManager
	telemetryService *telemetry.TelemetryService
}

func NewReportHandler(themeManager *theme.ThemeManager, telemetryService *telemetry.TelemetryService) *ReportHandler {
	return &ReportHandler{
		themeManager:     themeManager,
		telemetryService: telemetryService,
	}
}

// RunReportCreate creates a new report
func (h *ReportHandler) RunReportCreate(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📊 Create Report")
	fmt.Println(banner)

	envName, _ := cmd.Flags().GetString("environment")
	reportName, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")

	if envName == "" {
		return fmt.Errorf("--environment flag is required")
	}

	if reportName == "" {
		return fmt.Errorf("--name flag is required")
	}

	err := h.createReport(envName, reportName, description)

	// Track telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("report", "create", err == nil, time.Since(startTime).Milliseconds())
	}

	return err
}

// RunReportGenerate generates a report
func (h *ReportHandler) RunReportGenerate(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("⚙️  Generate Report")
	fmt.Println(banner)

	if len(args) != 1 {
		return fmt.Errorf("usage: stn report generate <report_id>")
	}

	reportID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid report ID: %v", err)
	}

	err = h.generateReport(reportID)

	// Track telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("report", "generate", err == nil, time.Since(startTime).Milliseconds())
	}

	return err
}

// RunReportList lists all reports
func (h *ReportHandler) RunReportList(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📋 Reports")
	fmt.Println(banner)

	envName, _ := cmd.Flags().GetString("environment")
	err := h.listReports(envName)

	// Track telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("report", "list", err == nil, time.Since(startTime).Milliseconds())
	}

	return err
}

// RunReportShow shows report details
func (h *ReportHandler) RunReportShow(cmd *cobra.Command, args []string) error {
	startTime := time.Now()
	styles := common.GetCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📊 Report Details")
	fmt.Println(banner)

	if len(args) != 1 {
		return fmt.Errorf("usage: stn report show <report_id>")
	}

	reportID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid report ID: %v", err)
	}

	err = h.showReport(reportID)

	// Track telemetry
	if h.telemetryService != nil {
		h.telemetryService.TrackCLICommand("report", "show", err == nil, time.Since(startTime).Milliseconds())
	}

	return err
}

// createReport creates a new report
func (h *ReportHandler) createReport(envName, reportName, description string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Find environment
	env, err := repos.Environments.GetByName(envName)
	if err != nil {
		return fmt.Errorf("environment not found: %v", err)
	}

	// Create default team criteria
	teamCriteria := services.TeamCriteria{
		Goal: "Evaluate the overall performance and quality of all agents in the environment",
		Criteria: map[string]services.EvaluationCriterion{
			"effectiveness": {
				Weight:      0.4,
				Description: "How well agents accomplish their intended tasks",
				Threshold:   7.0,
			},
			"reliability": {
				Weight:      0.3,
				Description: "Consistency and success rate of agent executions",
				Threshold:   8.0,
			},
			"efficiency": {
				Weight:      0.3,
				Description: "Resource usage and execution speed",
				Threshold:   7.0,
			},
		},
	}

	teamCriteriaJSON, _ := json.Marshal(teamCriteria)

	descSQL := sql.NullString{String: description, Valid: description != ""}

	// Create report
	report, err := repos.Reports.CreateReport(context.Background(), queries.CreateReportParams{
		Name:          reportName,
		Description:   descSQL,
		EnvironmentID: env.ID,
		TeamCriteria:  string(teamCriteriaJSON),
		AgentCriteria: sql.NullString{Valid: false},
		JudgeModel:    sql.NullString{String: "gpt-5-mini", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to create report: %v", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	fmt.Println(styles.Success.Render(fmt.Sprintf("✅ Report created successfully (ID: %d)", report.ID)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("   Environment: %s", envName)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("   Name: %s", reportName)))
	fmt.Println()
	fmt.Println(styles.Info.Render(fmt.Sprintf("Run: stn report generate %d", report.ID)))

	return nil
}

// generateReport generates a report
func (h *ReportHandler) generateReport(reportID int64) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Get report
	report, err := repos.Reports.GetByID(context.Background(), reportID)
	if err != nil {
		return fmt.Errorf("report not found: %v", err)
	}

	// Check status
	if report.Status == "completed" {
		return fmt.Errorf("report already completed")
	}

	if report.Status == "generating_team" || report.Status == "generating_agents" {
		return fmt.Errorf("report generation already in progress")
	}

	// Create report generator service
	reportGenerator := services.NewReportGenerator(repos, database.Conn(), nil) // Uses default config

	// Generate report
	styles := common.GetCLIStyles(h.themeManager)
	fmt.Println(styles.Info.Render(fmt.Sprintf("🔄 Generating report %d...", reportID)))
	fmt.Println()

	err = reportGenerator.GenerateReport(context.Background(), reportID)
	if err != nil {
		return fmt.Errorf("failed to generate report: %v", err)
	}

	// Get updated report
	report, err = repos.Reports.GetByID(context.Background(), reportID)
	if err != nil {
		return fmt.Errorf("failed to fetch updated report: %v", err)
	}

	fmt.Println()
	fmt.Println(styles.Success.Render("✅ Report generation completed!"))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("   Team Score: %.1f/10", report.TeamScore.Float64)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("   Agents Analyzed: %d", report.TotalAgentsAnalyzed.Int64)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("   Runs Analyzed: %d", report.TotalRunsAnalyzed.Int64)))
	fmt.Println(styles.Muted.Render(fmt.Sprintf("   Duration: %.2fs", report.GenerationDurationSeconds.Float64)))
	fmt.Println()
	fmt.Println(styles.Info.Render(fmt.Sprintf("View details: stn report show %d", reportID)))

	return nil
}

// listReports lists all reports (simplified version)
func (h *ReportHandler) listReports(envName string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	ctx := context.Background()

	var reports []queries.Report
	if envName != "" {
		// Filter by environment
		env, err := repos.Environments.GetByName(envName)
		if err != nil {
			return fmt.Errorf("environment not found: %v", err)
		}
		reports, err = repos.Reports.ListByEnvironment(ctx, env.ID)
		if err != nil {
			return fmt.Errorf("failed to list reports: %v", err)
		}
	} else {
		// List all reports
		reports, err = repos.Reports.ListReports(ctx, queries.ListReportsParams{
			Limit:  100,
			Offset: 0,
		})
		if err != nil {
			return fmt.Errorf("failed to list reports: %v", err)
		}
	}

	if len(reports) == 0 {
		fmt.Println("No reports found.")
		fmt.Println()
		fmt.Println("Create a report with: stn report create --env <environment> --name <name>")
		return nil
	}

	styles := common.GetCLIStyles(h.themeManager)
	fmt.Println()

	for _, report := range reports {
		// Get environment name
		env, _ := repos.Environments.GetByID(report.EnvironmentID)
		envNameDisplay := "unknown"
		if env != nil {
			envNameDisplay = env.Name
		}

		statusIcon := getStatusIcon(report.Status)
		fmt.Printf("%s %s %s\n",
			styles.Muted.Render(fmt.Sprintf("[%d]", report.ID)),
			statusIcon,
			report.Name)
		fmt.Printf("    %s\n", styles.Muted.Render(fmt.Sprintf("Env: %s | Status: %s", envNameDisplay, report.Status)))

		if report.Status == "completed" {
			fmt.Printf("    %s\n", styles.Success.Render(fmt.Sprintf("Score: %.1f/10 | Agents: %d | Runs: %d",
				report.TeamScore.Float64, report.TotalAgentsAnalyzed.Int64, report.TotalRunsAnalyzed.Int64)))
		} else if report.Status == "generating_team" || report.Status == "generating_agents" {
			fmt.Printf("    %s\n", styles.Info.Render(fmt.Sprintf("Progress: %d%% | %s",
				report.Progress.Int64, report.CurrentStep.String)))
		}
		fmt.Println()
	}

	return nil
}

// showReport shows detailed report information (simplified version)
func (h *ReportHandler) showReport(reportID int64) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load station config: %v", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	ctx := context.Background()

	// Get report
	report, err := repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		return fmt.Errorf("report not found: %v", err)
	}

	// Get environment
	env, err := repos.Environments.GetByID(report.EnvironmentID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %v", err)
	}

	styles := common.GetCLIStyles(h.themeManager)
	fmt.Println()
	fmt.Printf("%s\n", report.Name)
	fmt.Println(styles.Muted.Render(fmt.Sprintf("ID: %d | Environment: %s", report.ID, env.Name)))
	fmt.Println()

	// Status
	statusIcon := getStatusIcon(report.Status)
	fmt.Printf("%s Status: %s\n", statusIcon, report.Status)

	if report.Status == "generating_team" || report.Status == "generating_agents" {
		fmt.Println(styles.Info.Render(fmt.Sprintf("   Progress: %d%%", report.Progress.Int64)))
		if report.CurrentStep.Valid {
			fmt.Println(styles.Muted.Render(fmt.Sprintf("   Current Step: %s", report.CurrentStep.String)))
		}
	}
	fmt.Println()

	if report.Status == "completed" {
		// Team-level results
		fmt.Println("📊 Team Score")
		fmt.Println(styles.Success.Render(fmt.Sprintf("   Overall Score: %.1f/10", report.TeamScore.Float64)))
		fmt.Println()

		// Executive Summary
		if report.ExecutiveSummary.Valid && report.ExecutiveSummary.String != "" {
			fmt.Println("📝 Executive Summary")
			fmt.Println(report.ExecutiveSummary.String)
			fmt.Println()
		}

		// Agent Details
		agentDetails, err := repos.Reports.GetAgentReportDetails(ctx, reportID)
		if err == nil && len(agentDetails) > 0 {
			fmt.Printf("🤖 Agent Performance (%d agents)\n", len(agentDetails))
			fmt.Println()

			for _, detail := range agentDetails {
				passIcon := "✅"
				if !detail.Passed {
					passIcon = "❌"
				}

				fmt.Printf("%s %s - Score: %.1f/10\n",
					passIcon,
					detail.AgentName,
					detail.Score)

				if detail.RunsAnalyzed.Valid {
					fmt.Println(styles.Muted.Render(fmt.Sprintf("   Runs: %d | Success Rate: %.1f%% | Avg Duration: %.2fs",
						detail.RunsAnalyzed.Int64,
						detail.SuccessRate.Float64*100,
						detail.AvgDurationSeconds.Float64)))
				}

				if detail.Reasoning.Valid {
					fmt.Println(styles.Muted.Render(fmt.Sprintf("   %s", detail.Reasoning.String)))
				}
				fmt.Println()
			}
		}

		// Metadata
		fmt.Println("📈 Report Metadata")
		fmt.Println(styles.Muted.Render(fmt.Sprintf("   Total Runs Analyzed: %d", report.TotalRunsAnalyzed.Int64)))
		fmt.Println(styles.Muted.Render(fmt.Sprintf("   Total Agents Analyzed: %d", report.TotalAgentsAnalyzed.Int64)))
		fmt.Println(styles.Muted.Render(fmt.Sprintf("   Generation Duration: %.2fs", report.GenerationDurationSeconds.Float64)))
		if report.JudgeModel.Valid {
			fmt.Println(styles.Muted.Render(fmt.Sprintf("   LLM Judge Model: %s", report.JudgeModel.String)))
		}
		if report.TotalLlmTokens.Valid {
			fmt.Println(styles.Muted.Render(fmt.Sprintf("   Total LLM Tokens: %d", report.TotalLlmTokens.Int64)))
		}
		if report.TotalLlmCost.Valid {
			fmt.Println(styles.Muted.Render(fmt.Sprintf("   Total LLM Cost: $%.4f", report.TotalLlmCost.Float64)))
		}
		fmt.Println()
	}

	if report.Status == "failed" {
		fmt.Println(styles.Error.Render("❌ Report generation failed"))
		if report.ErrorMessage.Valid {
			fmt.Println(styles.Muted.Render(fmt.Sprintf("   Error: %s", report.ErrorMessage.String)))
		}
		fmt.Println()
	}

	return nil
}

func getStatusIcon(status string) string {
	switch status {
	case "pending":
		return "⏳"
	case "generating_team":
		return "🔄"
	case "generating_agents":
		return "🔄"
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	default:
		return "❓"
	}
}
