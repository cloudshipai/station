package mocks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewOpenCostMock() *MockServer {
	server := NewMockServer(
		"opencost",
		"1.0.0",
		"OpenCost mock for Kubernetes cost allocation",
	)

	server.RegisterTool(mcp.Tool{
		Name:        "get_allocation",
		Description: "Get Kubernetes cost allocation by namespace, workload, or labels",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"window": "7d",
			"items": []map[string]interface{}{
				{
					"name":         "production/api-deployment",
					"cpu_cost":     1234.50,
					"memory_cost":  876.30,
					"network_cost": 245.20,
					"total_cost":   2545.50,
				},
			},
			"totals": map[string]interface{}{
				"total_cost": 7186.70,
			},
		}
		return SuccessResult(data)
	})

	return server
}
