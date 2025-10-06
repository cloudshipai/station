package mocks

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// AWSCostExplorerMock provides a mock AWS Cost Explorer MCP server
// Returns realistic fake cost data for FinOps demo scenarios
type AWSCostExplorerMock struct {
	*MockServer
}

// NewAWSCostExplorerMock creates a new AWS Cost Explorer mock server
func NewAWSCostExplorerMock() *AWSCostExplorerMock {
	mock := &AWSCostExplorerMock{
		MockServer: NewMockServer(
			"aws-cost-explorer-mock",
			"1.0.0",
			"Mock AWS Cost Explorer for FinOps demos - returns realistic fake cost data",
		),
	}

	// Register tools
	mock.registerTools()

	return mock
}

func (m *AWSCostExplorerMock) registerTools() {
	// Get Cost and Usage
	m.RegisterTool(mcp.Tool{
		Name:        "get_cost_and_usage",
		Description: "Get AWS cost and usage data for a specific time period",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"time_period": map[string]interface{}{
					"type":        "object",
					"description": "Time period for cost data (start and end dates)",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{
							"type":        "string",
							"description": "Start date (YYYY-MM-DD)",
						},
						"end": map[string]interface{}{
							"type":        "string",
							"description": "End date (YYYY-MM-DD)",
						},
					},
					"required": []string{"start", "end"},
				},
				"granularity": map[string]interface{}{
					"type":        "string",
					"description": "Granularity of cost data (DAILY, MONTHLY, HOURLY)",
					"enum":        []string{"DAILY", "MONTHLY", "HOURLY"},
				},
				"metrics": map[string]interface{}{
					"type":        "array",
					"description": "Cost metrics to retrieve",
					"items": map[string]interface{}{
						"type": "string",
						"enum": []string{"BlendedCost", "UnblendedCost", "UsageQuantity"},
					},
				},
			},
			Required: []string{"time_period", "granularity", "metrics"},
		},
	}, m.handleGetCostAndUsage)

	// Get Cost Forecast
	m.RegisterTool(mcp.Tool{
		Name:        "get_cost_forecast",
		Description: "Get AWS cost forecast for future periods",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"time_period": map[string]interface{}{
					"type":        "object",
					"description": "Time period for forecast",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{
							"type": "string",
						},
						"end": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"start", "end"},
				},
				"metric": map[string]interface{}{
					"type": "string",
					"enum": []string{"BLENDED_COST", "UNBLENDED_COST"},
				},
			},
			Required: []string{"time_period", "metric"},
		},
	}, m.handleGetCostForecast)

	// Get Cost Anomalies
	m.RegisterTool(mcp.Tool{
		Name:        "get_cost_anomalies",
		Description: "Detect cost anomalies and spikes",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"time_period": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{
							"type": "string",
						},
						"end": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"start", "end"},
				},
			},
			Required: []string{"time_period"},
		},
	}, m.handleGetCostAnomalies)
}

func (m *AWSCostExplorerMock) handleGetCostAndUsage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Generate realistic cost data
	services := []string{"Amazon EC2", "Amazon S3", "Amazon RDS", "AWS Lambda", "Amazon CloudFront", "Amazon DynamoDB"}

	var resultsByTime []map[string]interface{}

	// Generate 7 days of cost data
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")

		var groups []map[string]interface{}
		for _, service := range services {
			cost := rand.Float64() * 1000
			groups = append(groups, map[string]interface{}{
				"Keys": []string{service},
				"Metrics": map[string]interface{}{
					"BlendedCost": map[string]interface{}{
						"Amount": fmt.Sprintf("%.2f", cost),
						"Unit":   "USD",
					},
				},
			})
		}

		resultsByTime = append(resultsByTime, map[string]interface{}{
			"TimePeriod": map[string]string{
				"Start": date,
				"End":   date,
			},
			"Total": map[string]interface{}{
				"BlendedCost": map[string]interface{}{
					"Amount": fmt.Sprintf("%.2f", rand.Float64()*5000+2000),
					"Unit":   "USD",
				},
			},
			"Groups": groups,
		})
	}

	response := map[string]interface{}{
		"ResultsByTime": resultsByTime,
		"DimensionValueAttributes": []interface{}{},
	}

	return SuccessResult(response)
}

func (m *AWSCostExplorerMock) handleGetCostForecast(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Generate forecast data
	var forecastResults []map[string]interface{}

	baseCost := rand.Float64()*5000 + 3000
	for i := 0; i < 30; i++ {
		date := time.Now().AddDate(0, 0, i).Format("2006-01-02")

		// Simulate slight upward trend with variance
		variance := (rand.Float64() - 0.5) * 500
		trend := float64(i) * 20
		forecastCost := baseCost + trend + variance

		forecastResults = append(forecastResults, map[string]interface{}{
			"TimePeriod": map[string]string{
				"Start": date,
				"End":   date,
			},
			"MeanValue": fmt.Sprintf("%.2f", forecastCost),
			"PredictionIntervalLowerBound": fmt.Sprintf("%.2f", forecastCost*0.85),
			"PredictionIntervalUpperBound": fmt.Sprintf("%.2f", forecastCost*1.15),
		})
	}

	response := map[string]interface{}{
		"Total": map[string]interface{}{
			"Amount": fmt.Sprintf("%.2f", baseCost*30),
			"Unit":   "USD",
		},
		"ForecastResultsByTime": forecastResults,
	}

	return SuccessResult(response)
}

func (m *AWSCostExplorerMock) handleGetCostAnomalies(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Generate 2-3 realistic anomalies
	anomalies := []map[string]interface{}{
		{
			"AnomalyId":    "anomaly-1",
			"AnomalyScore": map[string]interface{}{
				"MaxScore":     98.5,
				"CurrentScore": 95.2,
			},
			"Impact": map[string]interface{}{
				"MaxImpact":     1500.00,
				"TotalImpact":   1250.00,
				"TotalExpectedSpend": 3200.00,
				"TotalActualSpend":   4450.00,
			},
			"MonitorArn": "arn:aws:ce::123456789012:anomalymonitor/monitor-1",
			"AnomalyStartDate": time.Now().AddDate(0, 0, -2).Format("2006-01-02"),
			"AnomalyEndDate":   time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
			"DimensionValue":   "Amazon EC2",
			"RootCauses": []map[string]interface{}{
				{
					"Service": "Amazon EC2",
					"Region":  "us-east-1",
					"UsageType": "BoxUsage:t3.large",
				},
			},
			"Feedback": "UNCLASSIFIED",
		},
		{
			"AnomalyId":    "anomaly-2",
			"AnomalyScore": map[string]interface{}{
				"MaxScore":     75.3,
				"CurrentScore": 70.1,
			},
			"Impact": map[string]interface{}{
				"MaxImpact":     800.00,
				"TotalImpact":   650.00,
				"TotalExpectedSpend": 1200.00,
				"TotalActualSpend":   1850.00,
			},
			"MonitorArn": "arn:aws:ce::123456789012:anomalymonitor/monitor-1",
			"AnomalyStartDate": time.Now().AddDate(0, 0, -5).Format("2006-01-02"),
			"AnomalyEndDate":   time.Now().AddDate(0, 0, -4).Format("2006-01-02"),
			"DimensionValue":   "Amazon S3",
			"RootCauses": []map[string]interface{}{
				{
					"Service": "Amazon S3",
					"Region":  "us-west-2",
					"UsageType": "DataTransfer-Out-Bytes",
				},
			},
			"Feedback": "UNCLASSIFIED",
		},
	}

	response := map[string]interface{}{
		"Anomalies": anomalies,
	}

	return SuccessResult(response)
}
