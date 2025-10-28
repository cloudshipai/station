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

	// get_today_date - Get current date (awslabs MCP tool)
	m.RegisterTool(mcp.Tool{
		Name:        "get_today_date",
		Description: "Get the current date and month to determine relevant data when answering last month",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		now := time.Now()
		data := map[string]interface{}{
			"current_date":  now.Format("2006-01-02"),
			"current_month": now.Format("2006-01"),
			"current_year":  now.Year(),
			"timestamp":     now.Unix(),
		}
		return SuccessResult(data)
	})

	// get_dimension_values - Get available dimension values (awslabs MCP tool)
	m.RegisterTool(mcp.Tool{
		Name:        "get_dimension_values",
		Description: "Get available values for a specific dimension (e.g., SERVICE, REGION)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"dimension": map[string]interface{}{
					"type":        "string",
					"description": "Dimension name (SERVICE, REGION, LINKED_ACCOUNT, etc.)",
				},
			},
			Required: []string{"dimension"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"dimension": "SERVICE",
			"values": []string{
				"Amazon EC2",
				"Amazon S3",
				"Amazon RDS",
				"AWS Lambda",
				"Amazon CloudFront",
				"Amazon DynamoDB",
				"Amazon ECS",
				"Amazon EKS",
				"Amazon ElastiCache",
			},
		}
		return SuccessResult(data)
	})

	// get_tag_values - Get available tag values (awslabs MCP tool)
	m.RegisterTool(mcp.Tool{
		Name:        "get_tag_values",
		Description: "Get available values for a specific tag key",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"tag_key": map[string]interface{}{
					"type":        "string",
					"description": "Tag key name",
				},
			},
			Required: []string{"tag_key"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"tag_key": "Environment",
			"values": []string{
				"production",
				"staging",
				"development",
				"test",
			},
		}
		return SuccessResult(data)
	})

	// get_cost_and_usage_comparisons - Compare costs between periods (awslabs MCP tool)
	m.RegisterTool(mcp.Tool{
		Name:        "get_cost_and_usage_comparisons",
		Description: "Compare costs between two time periods to identify changes and trends",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"current_period": map[string]interface{}{
					"type":        "object",
					"description": "Current time period",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{"type": "string"},
						"end":   map[string]interface{}{"type": "string"},
					},
				},
				"previous_period": map[string]interface{}{
					"type":        "object",
					"description": "Previous time period for comparison",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{"type": "string"},
						"end":   map[string]interface{}{"type": "string"},
					},
				},
			},
			Required: []string{"current_period", "previous_period"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"current_period_cost":  15234.50,
			"previous_period_cost": 12890.25,
			"difference":           2344.25,
			"percent_change":       18.18,
			"trend":                "increasing",
			"comparison_by_service": []map[string]interface{}{
				{
					"service":        "Amazon EC2",
					"current_cost":   5200.00,
					"previous_cost":  4800.00,
					"change_percent": 8.33,
				},
				{
					"service":        "Amazon S3",
					"current_cost":   3500.00,
					"previous_cost":  2900.00,
					"change_percent": 20.69,
				},
			},
		}
		return SuccessResult(data)
	})

	// get_cost_comparison_drivers - Analyze cost change drivers (awslabs MCP tool)
	m.RegisterTool(mcp.Tool{
		Name:        "get_cost_comparison_drivers",
		Description: "Analyze what drove cost changes between periods (top 10 most significant drivers)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"current_period": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{"type": "string"},
						"end":   map[string]interface{}{"type": "string"},
					},
				},
				"previous_period": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"start": map[string]interface{}{"type": "string"},
						"end":   map[string]interface{}{"type": "string"},
					},
				},
			},
			Required: []string{"current_period", "previous_period"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"top_drivers": []map[string]interface{}{
				{
					"rank":           1,
					"service":        "Amazon EC2",
					"region":         "us-east-1",
					"cost_increase":  1200.50,
					"percent_impact": 51.2,
					"reason":         "Increased instance usage in production",
				},
				{
					"rank":           2,
					"service":        "Amazon S3",
					"region":         "us-west-2",
					"cost_increase":  600.00,
					"percent_impact": 25.6,
					"reason":         "Storage growth and data transfer costs",
				},
				{
					"rank":           3,
					"service":        "AWS Lambda",
					"region":         "eu-west-1",
					"cost_increase":  380.25,
					"percent_impact": 16.2,
					"reason":         "Increased function invocations",
				},
			},
			"total_change": 2344.25,
		}
		return SuccessResult(data)
	})
}

func (m *AWSCostExplorerMock) handleGetCostAndUsage(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Generate realistic cost data matching AWS Cost Explorer API response format
	services := []string{"Amazon EC2", "Amazon S3", "Amazon RDS", "AWS Lambda", "Amazon CloudFront", "Amazon DynamoDB"}

	var resultsByTime []map[string]interface{}

	// Generate 7 days of cost data
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, -i).Format("2006-01-02")
		nextDate := time.Now().AddDate(0, 0, -i+1).Format("2006-01-02")

		var groups []map[string]interface{}
		var totalBlendedCost float64
		var totalUnblendedCost float64
		var totalUsageQuantity float64

		for _, service := range services {
			blendedCost := rand.Float64() * 1000
			unblendedCost := blendedCost + (rand.Float64()*50 - 25) // Slight variation
			usageQty := rand.Float64() * 100000

			totalBlendedCost += blendedCost
			totalUnblendedCost += unblendedCost
			totalUsageQuantity += usageQty

			groups = append(groups, map[string]interface{}{
				"Keys": []string{service},
				"Metrics": map[string]interface{}{
					"BlendedCost": map[string]interface{}{
						"Amount": fmt.Sprintf("%.10f", blendedCost),
						"Unit":   "USD",
					},
					"UnblendedCost": map[string]interface{}{
						"Amount": fmt.Sprintf("%.10f", unblendedCost),
						"Unit":   "USD",
					},
					"UsageQuantity": map[string]interface{}{
						"Amount": fmt.Sprintf("%.10f", usageQty),
						"Unit":   "N/A",
					},
				},
			})
		}

		resultsByTime = append(resultsByTime, map[string]interface{}{
			"Estimated": false,
			"TimePeriod": map[string]string{
				"Start": date,
				"End":   nextDate,
			},
			"Total": map[string]interface{}{
				"BlendedCost": map[string]interface{}{
					"Amount": fmt.Sprintf("%.10f", totalBlendedCost),
					"Unit":   "USD",
				},
				"UnblendedCost": map[string]interface{}{
					"Amount": fmt.Sprintf("%.10f", totalUnblendedCost),
					"Unit":   "USD",
				},
				"UsageQuantity": map[string]interface{}{
					"Amount": fmt.Sprintf("%.10f", totalUsageQuantity),
					"Unit":   "N/A",
				},
			},
			"Groups": groups,
		})
	}

	response := map[string]interface{}{
		"GroupDefinitions": []map[string]interface{}{
			{
				"Type": "DIMENSION",
				"Key":  "SERVICE",
			},
		},
		"ResultsByTime":            resultsByTime,
		"DimensionValueAttributes": []interface{}{},
	}

	return SuccessResult(response)
}

func (m *AWSCostExplorerMock) handleGetCostForecast(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Generate forecast data matching AWS Cost Explorer API format
	var forecastResults []map[string]interface{}

	baseCost := rand.Float64()*5000 + 3000
	totalForecast := 0.0

	for i := 0; i < 30; i++ {
		date := time.Now().AddDate(0, 0, i).Format("2006-01-02")
		nextDate := time.Now().AddDate(0, 0, i+1).Format("2006-01-02")

		// Simulate slight upward trend with variance
		variance := (rand.Float64() - 0.5) * 500
		trend := float64(i) * 20
		forecastCost := baseCost + trend + variance
		totalForecast += forecastCost

		forecastResults = append(forecastResults, map[string]interface{}{
			"TimePeriod": map[string]string{
				"Start": date,
				"End":   nextDate,
			},
			"MeanValue":                    fmt.Sprintf("%.10f", forecastCost),
			"PredictionIntervalLowerBound": fmt.Sprintf("%.10f", forecastCost*0.85),
			"PredictionIntervalUpperBound": fmt.Sprintf("%.10f", forecastCost*1.15),
		})
	}

	response := map[string]interface{}{
		"Total": map[string]interface{}{
			"Amount": fmt.Sprintf("%.10f", totalForecast),
			"Unit":   "USD",
		},
		"ForecastResultsByTime": forecastResults,
	}

	return SuccessResult(response)
}

func (m *AWSCostExplorerMock) handleGetCostAnomalies(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Generate 2-3 realistic anomalies matching AWS Cost Explorer API format
	anomalies := []map[string]interface{}{
		{
			"AnomalyId": "anomaly-1234567890abcdef",
			"AnomalyScore": map[string]interface{}{
				"MaxScore":     98.5,
				"CurrentScore": 95.2,
			},
			"Impact": map[string]interface{}{
				"MaxImpact":             1500.00,
				"TotalImpact":           1250.00,
				"TotalExpectedSpend":    3200.00,
				"TotalActualSpend":      4450.00,
				"TotalImpactPercentage": 39.06,
			},
			"MonitorArn":       "arn:aws:ce::123456789012:anomalymonitor/monitor-1",
			"AnomalyStartDate": time.Now().AddDate(0, 0, -2).Format("2006-01-02"),
			"AnomalyEndDate":   time.Now().AddDate(0, 0, -1).Format("2006-01-02"),
			"DimensionValue":   "Amazon EC2",
			"RootCauses": []map[string]interface{}{
				{
					"Service":           "Amazon EC2",
					"Region":            "us-east-1",
					"UsageType":         "BoxUsage:t3.large",
					"LinkedAccount":     "123456789012",
					"LinkedAccountName": "Production Account",
					"Impact": map[string]interface{}{
						"Contribution": 92.5,
					},
				},
			},
			"Feedback": "UNCLASSIFIED",
		},
		{
			"AnomalyId": "anomaly-9876543210fedcba",
			"AnomalyScore": map[string]interface{}{
				"MaxScore":     75.3,
				"CurrentScore": 70.1,
			},
			"Impact": map[string]interface{}{
				"MaxImpact":             800.00,
				"TotalImpact":           650.00,
				"TotalExpectedSpend":    1200.00,
				"TotalActualSpend":      1850.00,
				"TotalImpactPercentage": 54.17,
			},
			"MonitorArn":       "arn:aws:ce::123456789012:anomalymonitor/monitor-1",
			"AnomalyStartDate": time.Now().AddDate(0, 0, -5).Format("2006-01-02"),
			"AnomalyEndDate":   time.Now().AddDate(0, 0, -4).Format("2006-01-02"),
			"DimensionValue":   "Amazon S3",
			"RootCauses": []map[string]interface{}{
				{
					"Service":           "Amazon S3",
					"Region":            "us-west-2",
					"UsageType":         "DataTransfer-Out-Bytes",
					"LinkedAccount":     "123456789012",
					"LinkedAccountName": "Production Account",
					"Impact": map[string]interface{}{
						"Contribution": 88.3,
					},
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
