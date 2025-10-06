package mocks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewCloudQueryMock() *MockServer {
	server := NewMockServer(
		"cloudquery",
		"1.0.0",
		"CloudQuery mock for cloud resource inventory and CUR data",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "query_resources",
		Description: "Query cloud resources across accounts and regions",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"id":                    "i-0a1b2c3d4e5f6g7h8",
				"type":                  "ec2-instance",
				"account":               "123456789012",
				"region":                "us-east-1",
				"monthly_cost_estimate": 280.32,
				"state":                 "running",
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "query_cur_data",
		Description: "Query AWS Cost and Usage Report data",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"date":           "2025-10-01",
				"service":        "Amazon EC2",
				"unblended_cost": 8234.50,
			},
		}
		return SuccessResult(data)
	})

	server.RegisterTool(mcp.Tool{
		Name:        "detect_drift",
		Description: "Detect infrastructure drift and changes",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := []map[string]interface{}{
			{
				"change":      "added",
				"resource_id": "i-9z8y7x6w5v4u3t2s1",
				"type":        "ec2-instance",
				"detected_at": "2025-10-05T14:23:45Z",
			},
		}
		return SuccessResult(data)
	})

	return server
}
