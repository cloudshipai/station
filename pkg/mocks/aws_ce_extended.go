package mocks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewAWSCostExplorerExtendedMock() *MockServer {
	server := NewMockServer(
		"aws-cost-explorer-extended",
		"1.0.0",
		"Extended AWS Cost Explorer mock with advanced cost analysis tools",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "get_cost_and_usage_comparisons",
		Description: "Compare costs across different time periods",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"current_period": map[string]interface{}{
				"start":      "2025-10-01",
				"end":        "2025-10-31",
				"total_cost": 157234.50,
			},
			"comparison_period": map[string]interface{}{
				"start":      "2025-09-01",
				"end":        "2025-09-30",
				"total_cost": 142891.20,
			},
			"delta": map[string]interface{}{
				"absolute": 14343.30,
				"percent":  10.04,
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "get_cost_drivers",
		Description: "Identify primary cost drivers and contributors",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{"service": "Amazon EC2", "cost": 8234.50, "percentage": 52.3, "trend": "increasing"},
			{"service": "Amazon RDS", "cost": 3421.80, "percentage": 21.8, "trend": "stable"},
		}
		return SuccessResult(data)
	})

	return server
}

func NewAWSComputeOptimizerMock() *MockServer {
	server := NewMockServer(
		"aws-compute-optimizer",
		"1.0.0",
		"AWS Compute Optimizer mock for rightsizing recommendations",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "get_ec2_rightsizing_recommendations",
		Description: "Get EC2 instance rightsizing recommendations",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"instance_id":     "i-1234567890abcdef0",
				"current_type":    "m5.2xlarge",
				"recommended":     "m5.xlarge",
				"monthly_savings": 280.50,
				"cpu_utilization": 25.3,
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "get_rds_rightsizing_recommendations",
		Description: "Get RDS database rightsizing recommendations",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"db_instance_id":  "prod-mysql-01",
				"current_class":   "db.r5.4xlarge",
				"recommended":     "db.r5.2xlarge",
				"monthly_savings": 650.25,
			},
		}
		return SuccessResult(data)
	})

	return server
}

func NewAWSBillingMock() *MockServer {
	server := NewMockServer(
		"aws-billing",
		"1.0.0",
		"AWS Billing mock for budgets, RIs, and Savings Plans",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "list_budgets",
		Description: "List all AWS budgets",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{"name": "Monthly Engineering Budget", "amount": 50000, "actual": 48234.50, "percentage": 96.5},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "get_budget_performance",
		Description: "Get budget performance and variance",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"budget_name": "Monthly Engineering Budget",
			"amount":      50000,
			"actual":      48234.50,
			"forecasted":  51200,
			"variance":    -765.50,
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "list_reserved_instances",
		Description: "List active Reserved Instances",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"id":              "ri-1234567890abcdef0",
				"instance_type":   "m5.2xlarge",
				"count":           5,
				"expiration_date": "2026-03-15",
				"term":            "1-year",
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "list_savings_plans",
		Description: "List active Savings Plans",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"id":                "sp-1234567890abcdef0",
				"type":              "Compute Savings Plan",
				"hourly_commitment": 5.50,
				"expiration_date":   "2026-06-01",
				"utilization":       94.5,
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "get_reserved_instance_recommendations",
		Description: "Get Reserved Instance purchase recommendations",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"instance_type":     "m5.xlarge",
				"recommended_count": 3,
				"estimated_savings": 450.25,
				"term":              "1-year",
				"payment_option":    "No Upfront",
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "get_savings_plans_recommendations",
		Description: "Get Savings Plans purchase recommendations",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"type":              "Compute Savings Plan",
				"hourly_commitment": 3.50,
				"estimated_savings": 1250.75,
				"term":              "1-year",
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "get_commitment_utilization",
		Description: "Get Reserved Instance and Savings Plan utilization",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"reserved_instances": map[string]interface{}{
				"utilization": 96.5,
				"coverage":    78.2,
			},
			"savings_plans": map[string]interface{}{
				"utilization": 94.5,
				"coverage":    82.1,
			},
		}
		return SuccessResult(data)
	})

	return server
}
