package benchmark

import (
	"context"
	"fmt"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/telemetry"
	"station/internal/theme"
	pkgbenchmark "station/pkg/benchmark"
)

// BenchmarkHandler handles benchmark-related CLI commands
type BenchmarkHandler struct {
	themeManager     *theme.ThemeManager
	telemetryService *telemetry.TelemetryService
}

func NewBenchmarkHandler(themeManager *theme.ThemeManager, telemetryService *telemetry.TelemetryService) *BenchmarkHandler {
	return &BenchmarkHandler{
		themeManager:     themeManager,
		telemetryService: telemetryService,
	}
}

// RunBenchmarkEvaluate evaluates an agent run using LLM-as-judge metrics
func (h *BenchmarkHandler) RunBenchmarkEvaluate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("run ID is required")
	}

	runID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid run ID: %s", args[0])
	}

	verbose, _ := cmd.Flags().GetBool("verbose")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🎯 Benchmark Evaluation")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("📊 Evaluating run using LLM-as-judge metrics..."))
	return h.evaluateRunLocal(runID, verbose)
}

// RunBenchmarkList lists benchmark results
func (h *BenchmarkHandler) RunBenchmarkList(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📈 Benchmark Results")
	fmt.Println(banner)

	if len(args) > 0 {
		// Show detailed results for specific run
		runID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid run ID: %s", args[0])
		}
		return h.showBenchmarkResultLocal(runID)
	}

	// List all recent benchmarks
	return h.listBenchmarksLocal()
}

// RunBenchmarkTasks lists available benchmark tasks
func (h *BenchmarkHandler) RunBenchmarkTasks(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("📋 Benchmark Tasks")
	fmt.Println(banner)

	return h.listBenchmarkTasksLocal()
}

// Local operations

func (h *BenchmarkHandler) evaluateRunLocal(runID int64, verbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Create judge and analyzer
	judge, err := pkgbenchmark.NewJudge(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LLM judge: %w", err)
	}

	analyzer := pkgbenchmark.NewAnalyzer(database.Conn(), judge)

	// Evaluate the run
	styles := getCLIStyles(h.themeManager)
	fmt.Printf("🔍 Loading run %d...\n", runID)

	result, err := analyzer.EvaluateRun(context.Background(), runID)
	if err != nil {
		return fmt.Errorf("evaluation failed: %w", err)
	}

	// Display results
	fmt.Printf("\n%s\n", styles.Banner.Render("📊 Evaluation Results"))
	fmt.Printf("\n🎯 Overall Score: %s/10\n", h.colorizeScore(result.QualityScore))

	readinessIcon := "✅"
	readinessColor := styles.Success
	if !result.ProductionReady {
		readinessIcon = "⚠️"
		readinessColor = styles.Error
	}
	fmt.Printf("🚀 Production Ready: %s %s\n", readinessIcon, readinessColor.Render(fmt.Sprintf("%v", result.ProductionReady)))

	fmt.Printf("\n📈 Metric Scores:\n")
	for name, metric := range result.Metrics {
		passIcon := "✅"
		if !metric.Passed {
			passIcon = "❌"
		}
		fmt.Printf("  %s %s: %.2f (threshold: %.2f)\n", passIcon, name, metric.Score, metric.Threshold)
		if verbose && metric.Reason != "" {
			fmt.Printf("     💡 %s\n", metric.Reason)
		}
	}

	fmt.Printf("\n💾 Results saved to database (run_id: %d)\n", runID)

	return nil
}

func (h *BenchmarkHandler) showBenchmarkResultLocal(runID int64) error {
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

	// Get benchmark metrics for this run
	metrics, err := repos.BenchmarkMetrics.GetByRunID(context.Background(), runID)
	if err != nil || len(metrics) == 0 {
		return fmt.Errorf("no benchmark results found for run %d", runID)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("📊 Benchmark Results for Run %d\n\n", runID)

	for _, metric := range metrics {
		passIcon := "✅"
		passColor := styles.Success
		if !metric.Passed {
			passIcon = "❌"
			passColor = styles.Error
		}

		fmt.Printf("%s %s: %s\n", passIcon, metric.MetricName, passColor.Render(fmt.Sprintf("%.2f", metric.Score)))
		fmt.Printf("   Threshold: %.2f\n", metric.Threshold)
		if metric.Reason != "" {
			fmt.Printf("   💡 %s\n", metric.Reason)
		}
		fmt.Println()
	}

	return nil
}

func (h *BenchmarkHandler) listBenchmarksLocal() error {
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

	// Get recent benchmarked runs
	runs, err := repos.AgentRuns.ListRecent(context.Background(), 50)
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	benchmarkedCount := 0

	for _, run := range runs {
		// Check if this run has benchmark results
		metrics, err := repos.BenchmarkMetrics.GetByRunID(context.Background(), run.ID)
		if err != nil || len(metrics) == 0 {
			continue
		}

		benchmarkedCount++

		// Calculate average score
		var totalScore float64
		passCount := 0
		for _, metric := range metrics {
			totalScore += metric.Score
			if metric.Passed {
				passCount++
			}
		}
		avgScore := totalScore / float64(len(metrics))

		scoreColor := styles.Success
		if avgScore < 0.7 {
			scoreColor = styles.Error
		} else if avgScore < 0.85 {
			scoreColor = styles.Info
		}

		fmt.Printf("• Run %d: %s (avg: %s, %d/%d passed)\n",
			run.ID,
			run.AgentName,
			scoreColor.Render(fmt.Sprintf("%.2f", avgScore)),
			passCount,
			len(metrics))
		fmt.Printf("  %s\n", h.truncateString(run.Task, 80))
	}

	if benchmarkedCount == 0 {
		fmt.Println("• No benchmark results found")
		fmt.Println("\n💡 Use 'stn benchmark evaluate <run-id>' to evaluate a run")
	} else {
		fmt.Printf("\nTotal: %d benchmarked runs\n", benchmarkedCount)
	}

	return nil
}

func (h *BenchmarkHandler) listBenchmarkTasksLocal() error {
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
	tasks, err := repos.BenchmarkTasks.GetAll(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list benchmark tasks: %w", err)
	}

	if len(tasks) == 0 {
		fmt.Println("• No benchmark tasks found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d benchmark task(s):\n\n", len(tasks))

	for _, task := range tasks {
		fmt.Printf("📋 %s\n", styles.Success.Render(task.Name))
		fmt.Printf("   Category: %s\n", task.Category)
		fmt.Printf("   Description: %s\n", task.Description)

		if task.ExpectedOutputExample != "" {
			fmt.Printf("   Expected Output: %s\n", h.truncateString(task.ExpectedOutputExample, 80))
		}

		if task.EvaluationCriteria != "" {
			fmt.Printf("   Criteria: %s\n", h.truncateString(task.EvaluationCriteria, 80))
		}

		fmt.Println()
	}

	return nil
}

// Helper functions

func (h *BenchmarkHandler) colorizeScore(score float64) string {
	styles := getCLIStyles(h.themeManager)
	scoreStr := fmt.Sprintf("%.1f", score)

	if score >= 8.5 {
		return styles.Success.Render(scoreStr)
	} else if score >= 7.0 {
		return styles.Info.Render(scoreStr)
	}
	return styles.Error.Render(scoreStr)
}

func (h *BenchmarkHandler) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

type CLIStyles struct {
	Banner  lipgloss.Style
	Success lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style
}

// getCLIStyles returns theme-aware CLI styles
func getCLIStyles(themeManager *theme.ThemeManager) CLIStyles {
	if themeManager == nil {
		// Fallback to hardcoded Tokyo Night styles
		return CLIStyles{
			Banner: lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#bb9af7")).
				Padding(1, 2).
				MarginBottom(1),
			Success: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#9ece6a")).
				Bold(true),
			Error: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#f7768e")).
				Bold(true),
			Info: lipgloss.NewStyle().
				Foreground(lipgloss.Color("#7dcfff")),
		}
	}

	themeStyles := themeManager.GetStyles()
	palette := themeManager.GetPalette()

	return CLIStyles{
		Banner: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(palette.Secondary)).
			Padding(1, 2).
			MarginBottom(1),
		Success: themeStyles.Success,
		Error:   themeStyles.Error,
		Info:    themeStyles.Info,
	}
}
