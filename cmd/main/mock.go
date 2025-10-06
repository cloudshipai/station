package main

import (
	"fmt"
	"os"

	"station/pkg/mocks"

	"github.com/spf13/cobra"
)

var mockCmd = &cobra.Command{
	Use:   "mock [tool]",
	Short: "Run a mock MCP server for demos",
	Long: `Run a mock MCP server that returns realistic fake data for demonstration purposes.

Available mock tools:
  aws-cost-explorer          - Mock AWS Cost Explorer (cost data, forecasts, anomalies)
  aws-cost-explorer-extended - Extended AWS CE (comparisons, drivers)
  aws-compute-optimizer      - Mock AWS Compute Optimizer (rightsizing)
  aws-billing                - Mock AWS Billing (budgets, SPs, RIs)
  opencost                   - Mock OpenCost (Kubernetes cost allocation)
  cloudquery                 - Mock CloudQuery (resource inventory, CUR data)
  grafana                    - Mock Grafana/Prometheus (metrics, usage)
  github                     - Mock GitHub (PRs, deployments, commits)
  pagerduty                  - Mock PagerDuty (incidents)
  stripe                     - Mock Stripe (revenue, subscriptions)

Mock servers are designed for the Live Demo feature and return realistic but fake data.`,
	Args: cobra.ExactArgs(1),
	Run:  runMock,
}

func runMock(cmd *cobra.Command, args []string) {
	toolName := args[0]

	var server interface {
		Serve() error
	}

	switch toolName {
	case "aws-cost-explorer":
		server = mocks.NewAWSCostExplorerMock()
	case "aws-cost-explorer-extended":
		server = mocks.NewAWSCostExplorerExtendedMock()
	case "aws-compute-optimizer":
		server = mocks.NewAWSComputeOptimizerMock()
	case "aws-billing":
		server = mocks.NewAWSBillingMock()
	case "opencost":
		server = mocks.NewOpenCostMock()
	case "cloudquery":
		server = mocks.NewCloudQueryMock()
	case "grafana":
		server = mocks.NewGrafanaMock()
	case "github":
		server = mocks.NewGitHubMock()
	case "pagerduty":
		server = mocks.NewPagerDutyMock()
	case "stripe":
		server = mocks.NewStripeMock()
	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown mock tool '%s'\n\n", toolName)
		fmt.Fprintln(os.Stderr, "Available mock tools:")
		fmt.Fprintln(os.Stderr, "  aws-cost-explorer          - Mock AWS Cost Explorer")
		fmt.Fprintln(os.Stderr, "  aws-cost-explorer-extended - Extended AWS CE")
		fmt.Fprintln(os.Stderr, "  aws-compute-optimizer      - Mock Compute Optimizer")
		fmt.Fprintln(os.Stderr, "  aws-billing                - Mock AWS Billing")
		fmt.Fprintln(os.Stderr, "  opencost                   - Mock OpenCost")
		fmt.Fprintln(os.Stderr, "  cloudquery                 - Mock CloudQuery")
		fmt.Fprintln(os.Stderr, "  grafana                    - Mock Grafana/Prometheus")
		fmt.Fprintln(os.Stderr, "  github                     - Mock GitHub")
		fmt.Fprintln(os.Stderr, "  pagerduty                  - Mock PagerDuty")
		fmt.Fprintln(os.Stderr, "  stripe                     - Mock Stripe")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Starting mock MCP server: %s\n", toolName)
	fmt.Fprintln(os.Stderr, "Server is running in stdio mode...")

	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running mock server: %v\n", err)
		os.Exit(1)
	}
}
