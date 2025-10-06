package mocks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewGrafanaMock() *MockServer {
	server := NewMockServer(
		"grafana",
		"1.0.0",
		"Grafana/Prometheus mock for metrics and usage data",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "query_range",
		Description: "Query time-series metrics over a range",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"query": "container_cpu_usage_seconds_total",
			"series": []map[string]interface{}{
				{
					"metric": map[string]string{"namespace": "production", "pod": "api-deployment-abc123"},
					"values": [][]interface{}{{1727740800, "15.3"}, {1727744400, "18.7"}},
				},
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "query_egress_metrics",
		Description: "Query network egress/data transfer metrics",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"service":              "api-gateway",
				"bytes_per_day":        1234567890,
				"week_over_week_change": 15.3,
			},
		}
		return SuccessResult(data)
	})

	return server
}

func NewGitHubMock() *MockServer {
	server := NewMockServer(
		"github",
		"1.0.0",
		"GitHub mock for PRs, commits, and deployments",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "list_prs",
		Description: "List recent pull requests",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"id":        12345,
				"number":    789,
				"title":     "Add new caching layer to API",
				"state":     "merged",
				"merged_at": "2025-10-03T14:23:45Z",
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "list_deployments",
		Description: "List recent deployments",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"id":          "deploy-123",
				"sha":         "abc123def456",
				"environment": "production",
				"created_at":  "2025-10-03T14:30:00Z",
				"status":      "success",
			},
		}
		return SuccessResult(data)
	})

	return server
}

func NewPagerDutyMock() *MockServer {
	server := NewMockServer(
		"pagerduty",
		"1.0.0",
		"PagerDuty mock for incident data",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "list_incidents",
		Description: "List recent incidents",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"id":               "INC-12345",
				"service":          "api-service",
				"status":           "resolved",
				"severity":         "high",
				"started_at":       "2025-10-03T14:25:00Z",
				"duration_minutes": 77,
			},
		}
		return SuccessResult(data)
	})

	return server
}

func NewStripeMock() *MockServer {
	server := NewMockServer(
		"stripe",
		"1.0.0",
		"Stripe mock for revenue and subscription data",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "get_revenue_metrics",
		Description: "Get MRR, ARR, and customer metrics",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"period":           "2025-10",
			"mrr":              245000.00,
			"arr":              2940000.00,
			"active_customers": 1247,
		}
		return SuccessResult(data)
	})

	return server
}
