package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers/report"
)

// Report command definitions
var (
	reportCmd = &cobra.Command{
		Use:   "report",
		Short: "Manage reports",
		Long:  "Create, generate, list, and view environment-wide agent performance reports",
	}

	reportCreateCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new report",
		Long:  "Create a new report for an environment with team evaluation criteria",
		RunE:  runReportCreate,
	}

	reportGenerateCmd = &cobra.Command{
		Use:   "generate <report_id>",
		Short: "Generate a report",
		Long:  "Generate a report by running LLM-as-judge evaluation on all agents in the environment",
		Args:  cobra.ExactArgs(1),
		RunE:  runReportGenerate,
	}

	reportListCmd = &cobra.Command{
		Use:   "list",
		Short: "List reports",
		Long:  "List all reports, optionally filtered by environment",
		RunE:  runReportList,
	}

	reportShowCmd = &cobra.Command{
		Use:   "show <report_id>",
		Short: "Show report details",
		Long:  "Show detailed information about a report including team score, agent performance, and LLM evaluation results",
		Args:  cobra.ExactArgs(1),
		RunE:  runReportShow,
	}
)

// runReportCreate creates a new report
func runReportCreate(cmd *cobra.Command, args []string) error {
	reportHandler := report.NewReportHandler(nil, telemetryService)
	return reportHandler.RunReportCreate(cmd, args)
}

// runReportGenerate generates a report
func runReportGenerate(cmd *cobra.Command, args []string) error {
	reportHandler := report.NewReportHandler(nil, telemetryService)
	return reportHandler.RunReportGenerate(cmd, args)
}

// runReportList lists reports
func runReportList(cmd *cobra.Command, args []string) error {
	reportHandler := report.NewReportHandler(nil, telemetryService)
	return reportHandler.RunReportList(cmd, args)
}

// runReportShow shows report details
func runReportShow(cmd *cobra.Command, args []string) error {
	reportHandler := report.NewReportHandler(nil, telemetryService)
	return reportHandler.RunReportShow(cmd, args)
}
