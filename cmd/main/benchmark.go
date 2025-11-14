package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers/benchmark"
)

// Benchmark command definitions
var (
	benchmarkCmd = &cobra.Command{
		Use:   "benchmark",
		Short: "Benchmark and evaluate agent quality",
		Long:  "Evaluate agent runs using LLM-as-judge metrics for quality assessment",
	}

	benchmarkEvaluateCmd = &cobra.Command{
		Use:   "evaluate <run-id>",
		Short: "Evaluate a run",
		Long: `Evaluate an agent run using LLM-as-judge metrics.

This command analyzes a completed agent run using multiple quality metrics:
• Task Completion (0.0-1.0): How well the task was completed
• Answer Relevancy (0.0-1.0): How relevant the output is to the task
• Hallucination (0.0-1.0, lower is better): Rate of contradictions
• Toxicity (0.0-1.0, lower is better): Offensive/harmful content rate
• Faithfulness (0.0-1.0): Consistency with tool outputs

Results are stored in the database and include:
• Quality score (0-10 scale)
• Production readiness assessment
• Detailed metric breakdowns with evidence`,
		Example: `  stn benchmark evaluate 42           # Evaluate run ID 42
  stn benchmark evaluate 42 --verbose  # Show detailed metric analysis`,
		Args: cobra.ExactArgs(1),
		RunE: runBenchmarkEvaluate,
	}

	benchmarkListCmd = &cobra.Command{
		Use:   "list [run-id]",
		Short: "List benchmark results",
		Long: `List benchmark evaluation results.

With no arguments, shows all recent benchmarks.
With a run ID, shows detailed results for that specific run.`,
		Example: `  stn benchmark list       # List all benchmarks
  stn benchmark list 42    # Show detailed results for run 42`,
		RunE: runBenchmarkList,
	}

	benchmarkTasksCmd = &cobra.Command{
		Use:   "tasks",
		Short: "List benchmark tasks",
		Long:  "List all available benchmark tasks with their criteria and thresholds",
		RunE:  runBenchmarkTasks,
	}
)

// runBenchmarkEvaluate evaluates an agent run
func runBenchmarkEvaluate(cmd *cobra.Command, args []string) error {
	benchmarkHandler := benchmark.NewBenchmarkHandler(nil, telemetryService)
	return benchmarkHandler.RunBenchmarkEvaluate(cmd, args)
}

// runBenchmarkList lists benchmark results
func runBenchmarkList(cmd *cobra.Command, args []string) error {
	benchmarkHandler := benchmark.NewBenchmarkHandler(nil, telemetryService)
	return benchmarkHandler.RunBenchmarkList(cmd, args)
}

// runBenchmarkTasks lists benchmark tasks
func runBenchmarkTasks(cmd *cobra.Command, args []string) error {
	benchmarkHandler := benchmark.NewBenchmarkHandler(nil, telemetryService)
	return benchmarkHandler.RunBenchmarkTasks(cmd, args)
}
