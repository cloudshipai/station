package mocks

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// DatadogMock provides a mock Datadog MCP server
// Returns realistic metrics, logs, and monitor data for observability demos
type DatadogMock struct {
	*MockServer
}

// NewDatadogMock creates a new Datadog mock server
func NewDatadogMock() *DatadogMock {
	mock := &DatadogMock{
		MockServer: NewMockServer(
			"datadog",
			"1.0.0",
			"Mock Datadog MCP server for metrics, logs, monitors, and APM traces",
		),
	}

	// Register tools
	mock.registerTools()

	return mock
}

func (m *DatadogMock) registerTools() {
	// metrics.query - Query Datadog metrics
	m.RegisterTool(mcp.Tool{
		Name:        "metrics.query",
		Description: "Query Datadog metrics with aggregations and filters (supports avg, sum, min, max)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Datadog metric query (e.g., 'avg:aws.lambda.invocations{service:checkout}')",
				},
				"from": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds)",
				},
				"to": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds)",
				},
			},
			Required: []string{"query", "from", "to"},
		},
	}, m.handleMetricsQuery)

	// logs.query - Query Datadog logs
	m.RegisterTool(mcp.Tool{
		Name:        "logs.query",
		Description: "Query Datadog logs with filters and aggregations",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Datadog log search query (e.g., 'service:api status:error')",
				},
				"from": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"to": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of logs to return (default: 100)",
					"default":     100,
				},
			},
			Required: []string{"query", "from", "to"},
		},
	}, m.handleLogsQuery)

	// monitors.search - Search and list Datadog monitors
	m.RegisterTool(mcp.Tool{
		Name:        "monitors.search",
		Description: "Search Datadog monitors by name, tags, or status",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query (e.g., 'status:Alert', 'tag:service:api')",
				},
				"monitor_tags": map[string]interface{}{
					"type":        "array",
					"description": "Filter by monitor tags",
					"items": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}, m.handleMonitorsSearch)

	// apm.query - Query APM traces
	m.RegisterTool(mcp.Tool{
		Name:        "apm.query",
		Description: "Query Datadog APM traces for latency and error analysis",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"service": map[string]interface{}{
					"type":        "string",
					"description": "Service name",
				},
				"env": map[string]interface{}{
					"type":        "string",
					"description": "Environment (production, staging, etc.)",
				},
				"from": map[string]interface{}{
					"type":        "integer",
					"description": "Start timestamp (Unix seconds)",
				},
				"to": map[string]interface{}{
					"type":        "integer",
					"description": "End timestamp (Unix seconds)",
				},
			},
			Required: []string{"service", "from", "to"},
		},
	}, m.handleAPMQuery)
}

func (m *DatadogMock) handleMetricsQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now()
	var series []map[string]interface{}

	// Generate hourly datapoints for the last 24 hours
	for i := 0; i < 24; i++ {
		timestamp := now.Add(-time.Duration(i)*time.Hour).Unix() * 1000 // milliseconds
		value := 100.0 + rand.Float64()*200.0 + float64(i)*5.0          // Trend upward with noise

		series = append(series, map[string]interface{}{
			"pointlist": [][]float64{
				{float64(timestamp), value},
			},
			"scope":  "service:checkout,env:production",
			"metric": "aws.lambda.invocations",
			"unit": []interface{}{
				map[string]interface{}{
					"family":       "count",
					"scale_factor": 1.0,
					"name":         "invocation",
					"short_name":   "invoc",
					"plural":       "invocations",
					"id":           89,
				},
			},
		})
	}

	response := map[string]interface{}{
		"status":       "ok",
		"res_type":     "time_series",
		"resp_version": 1,
		"query":        "avg:aws.lambda.invocations{service:checkout}",
		"from_date":    now.Add(-24*time.Hour).Unix() * 1000,
		"to_date":      now.Unix() * 1000,
		"series":       series,
	}

	return SuccessResult(response)
}

func (m *DatadogMock) handleLogsQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now()
	var logs []map[string]interface{}

	errorMessages := []string{
		"Lambda function timeout after 30 seconds",
		"DynamoDB ProvisionedThroughputExceededException: Rate exceeded for table payments",
		"API Gateway 502 Bad Gateway - upstream connect error",
		"Cold start latency exceeded threshold: 2.3s",
		"Payment processor returned 503 Service Unavailable",
	}

	// Generate 20 log entries
	for i := 0; i < 20; i++ {
		timestamp := now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339Nano)
		status := "error"
		if i%3 == 0 {
			status = "warn"
		}

		logs = append(logs, map[string]interface{}{
			"id": fmt.Sprintf("log-%d", i),
			"content": map[string]interface{}{
				"timestamp": timestamp,
				"host":      "lambda-prod-us-east-1",
				"service":   "checkout-service",
				"status":    status,
				"message":   errorMessages[i%len(errorMessages)],
				"tags": []string{
					"env:production",
					"service:checkout-service",
					"aws_account:123456789012",
					"function_name:checkout-handler",
				},
				"attributes": map[string]interface{}{
					"aws.lambda.request_id": fmt.Sprintf("req-%d", i),
					"duration":              1500 + rand.Intn(1000),
					"memory_used_mb":        128 + rand.Intn(256),
				},
			},
		})
	}

	response := map[string]interface{}{
		"logs": logs,
		"meta": map[string]interface{}{
			"status": "done",
			"page": map[string]interface{}{
				"after": "next_page_token_xyz",
			},
		},
	}

	return SuccessResult(response)
}

func (m *DatadogMock) handleMonitorsSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now()

	monitors := []map[string]interface{}{
		{
			"id":      12345678,
			"name":    "High Lambda Error Rate",
			"type":    "metric alert",
			"query":   "avg(last_5m):sum:aws.lambda.errors{service:checkout} by {function_name}.as_count() > 50",
			"message": "@slack-alerts Lambda errors spiking on {{function_name.name}}",
			"tags":    []string{"service:checkout", "severity:high", "team:backend"},
			"options": map[string]interface{}{
				"thresholds": map[string]interface{}{
					"critical": 50.0,
					"warning":  25.0,
				},
				"notify_no_data": false,
				"notify_audit":   false,
			},
			"overall_state": "Alert",
			"created":       now.Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			"modified":      now.Add(-2 * time.Hour).Format(time.RFC3339),
			"creator": map[string]interface{}{
				"email": "sre@example.com",
				"name":  "SRE Team",
			},
		},
		{
			"id":            12345679,
			"name":          "Lambda Cold Start Latency",
			"type":          "metric alert",
			"query":         "avg(last_10m):avg:aws.lambda.duration.cold_start{service:checkout} > 2000",
			"message":       "@slack-alerts Cold start latency high on checkout service",
			"tags":          []string{"service:checkout", "severity:medium", "team:backend"},
			"overall_state": "OK",
			"created":       now.Add(-60 * 24 * time.Hour).Format(time.RFC3339),
			"modified":      now.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
		},
		{
			"id":            12345680,
			"name":          "DynamoDB Throttling",
			"type":          "metric alert",
			"query":         "avg(last_5m):sum:aws.dynamodb.user_errors{tablename:payments,operation:putitem} > 10",
			"message":       "@pagerduty DynamoDB throttling detected on payments table",
			"tags":          []string{"service:checkout", "severity:critical", "team:backend"},
			"overall_state": "Alert",
			"created":       now.Add(-90 * 24 * time.Hour).Format(time.RFC3339),
			"modified":      now.Add(-1 * time.Hour).Format(time.RFC3339),
		},
	}

	response := map[string]interface{}{
		"monitors": monitors,
		"metadata": map[string]interface{}{
			"total_count": len(monitors),
			"page_count":  1,
		},
	}

	return SuccessResult(response)
}

func (m *DatadogMock) handleAPMQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now()

	// Generate APM spans
	spans := []map[string]interface{}{
		{
			"trace_id": "1234567890abcdef",
			"span_id":  "abcdef1234567890",
			"service":  "checkout-service",
			"name":     "aws.lambda",
			"resource": "POST /checkout",
			"start":    now.Add(-5 * time.Minute).UnixNano(),
			"duration": 2500000000, // 2.5s in nanoseconds
			"error":    1,
			"meta": map[string]interface{}{
				"env":                   "production",
				"aws.lambda.request_id": "req-abc-123",
				"error.type":            "TimeoutError",
				"error.message":         "Lambda timeout after 30 seconds",
				"error.stack":           "at processPayment (index.js:45:10)\nat handler (index.js:23:5)",
			},
			"metrics": map[string]interface{}{
				"_dd.measured":   1,
				"memory_used_mb": 256,
			},
		},
		{
			"trace_id": "fedcba0987654321",
			"span_id":  "0987654321fedcba",
			"service":  "checkout-service",
			"name":     "dynamodb.query",
			"resource": "payments.getitem",
			"start":    now.Add(-3 * time.Minute).UnixNano(),
			"duration": 45000000, // 45ms in nanoseconds
			"error":    0,
			"meta": map[string]interface{}{
				"env":                     "production",
				"aws.dynamodb.table_name": "payments",
				"aws.dynamodb.operation":  "GetItem",
			},
		},
	}

	response := map[string]interface{}{
		"data": spans,
		"meta": map[string]interface{}{
			"total_count": len(spans),
		},
	}

	return SuccessResult(response)
}
