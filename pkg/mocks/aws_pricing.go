package mocks

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// AWSPricingMock provides a mock AWS Pricing MCP server
// Returns realistic pricing data for EC2, RDS, Lambda, and other AWS services
type AWSPricingMock struct {
	*MockServer
}

// NewAWSPricingMock creates a new AWS Pricing mock server
func NewAWSPricingMock() *AWSPricingMock {
	mock := &AWSPricingMock{
		MockServer: NewMockServer(
			"aws-pricing",
			"1.0.0",
			"Mock AWS Pricing API for rightsizing and cost optimization calculations",
		),
	}

	// Register tools
	mock.registerTools()

	return mock
}

func (m *AWSPricingMock) registerTools() {
	// query_products - Query AWS service pricing
	m.RegisterTool(mcp.Tool{
		Name:        "query_products",
		Description: "Query AWS Pricing API for product pricing information (EC2, RDS, Lambda, etc.)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"service_code": map[string]interface{}{
					"type":        "string",
					"description": "AWS service code (AmazonEC2, AmazonRDS, AWSLambda, AmazonS3, etc.)",
				},
				"filters": map[string]interface{}{
					"type":        "array",
					"description": "Filter criteria (instanceType, location, operatingSystem, etc.)",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"field": map[string]string{
								"type": "string",
							},
							"value": map[string]string{
								"type": "string",
							},
						},
					},
				},
			},
			Required: []string{"service_code"},
		},
	}, m.handleQueryProducts)

	// get_price_dimensions - Get pricing dimensions for a product
	m.RegisterTool(mcp.Tool{
		Name:        "get_price_dimensions",
		Description: "Get detailed pricing dimensions and on-demand rates for AWS products",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"service_code": map[string]interface{}{
					"type":        "string",
					"description": "AWS service code",
				},
				"instance_type": map[string]interface{}{
					"type":        "string",
					"description": "Instance type (e.g., t3.large, db.r5.xlarge)",
				},
				"region": map[string]interface{}{
					"type":        "string",
					"description": "AWS region (us-east-1, eu-west-1, etc.)",
				},
			},
			Required: []string{"service_code", "instance_type", "region"},
		},
	}, m.handleGetPriceDimensions)
}

func (m *AWSPricingMock) handleQueryProducts(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Return realistic EC2 instance pricing
	priceList := []map[string]interface{}{
		{
			"product": map[string]interface{}{
				"productFamily": "Compute Instance",
				"attributes": map[string]interface{}{
					"servicecode":      "AmazonEC2",
					"location":         "US East (N. Virginia)",
					"locationType":     "AWS Region",
					"instanceType":     "t3.medium",
					"currentGeneration": "Yes",
					"vcpu":             "2",
					"memory":           "4 GiB",
					"storage":          "EBS only",
					"networkPerformance": "Up to 5 Gigabit",
					"operatingSystem":  "Linux",
					"tenancy":          "Shared",
				},
			},
			"serviceCode": "AmazonEC2",
			"terms": map[string]interface{}{
				"OnDemand": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"hourly": map[string]interface{}{
							"unit":           "Hrs",
							"pricePerUnit": map[string]string{
								"USD": "0.0416",
							},
							"description": "USD 0.0416 per On Demand Linux t3.medium Instance Hour",
						},
					},
				},
			},
		},
		{
			"product": map[string]interface{}{
				"productFamily": "Compute Instance",
				"attributes": map[string]interface{}{
					"servicecode":      "AmazonEC2",
					"location":         "US East (N. Virginia)",
					"instanceType":     "t3.large",
					"vcpu":             "2",
					"memory":           "8 GiB",
					"operatingSystem":  "Linux",
				},
			},
			"terms": map[string]interface{}{
				"OnDemand": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"hourly": map[string]interface{}{
							"unit":           "Hrs",
							"pricePerUnit": map[string]string{
								"USD": "0.0832",
							},
						},
					},
				},
			},
		},
		{
			"product": map[string]interface{}{
				"productFamily": "Compute Instance",
				"attributes": map[string]interface{}{
					"servicecode":      "AmazonEC2",
					"location":         "US East (N. Virginia)",
					"instanceType":     "t3.xlarge",
					"vcpu":             "4",
					"memory":           "16 GiB",
					"operatingSystem":  "Linux",
				},
			},
			"terms": map[string]interface{}{
				"OnDemand": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"hourly": map[string]interface{}{
							"unit":           "Hrs",
							"pricePerUnit": map[string]string{
								"USD": "0.1664",
							},
						},
					},
				},
			},
		},
		{
			"product": map[string]interface{}{
				"productFamily": "Compute Instance",
				"attributes": map[string]interface{}{
					"servicecode":      "AmazonEC2",
					"location":         "US East (N. Virginia)",
					"instanceType":     "m5.large",
					"vcpu":             "2",
					"memory":           "8 GiB",
					"operatingSystem":  "Linux",
				},
			},
			"terms": map[string]interface{}{
				"OnDemand": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"hourly": map[string]interface{}{
							"unit":           "Hrs",
							"pricePerUnit": map[string]string{
								"USD": "0.096",
							},
						},
					},
				},
			},
		},
		{
			"product": map[string]interface{}{
				"productFamily": "Compute Instance",
				"attributes": map[string]interface{}{
					"servicecode":      "AmazonEC2",
					"location":         "US East (N. Virginia)",
					"instanceType":     "m5.xlarge",
					"vcpu":             "4",
					"memory":           "16 GiB",
					"operatingSystem":  "Linux",
				},
			},
			"terms": map[string]interface{}{
				"OnDemand": map[string]interface{}{
					"priceDimensions": map[string]interface{}{
						"hourly": map[string]interface{}{
							"unit":           "Hrs",
							"pricePerUnit": map[string]string{
								"USD": "0.192",
							},
						},
					},
				},
			},
		},
	}

	response := map[string]interface{}{
		"PriceList": priceList,
		"NextToken": "",
	}

	return SuccessResult(response)
}

func (m *AWSPricingMock) handleGetPriceDimensions(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Return detailed pricing for a specific instance type
	instancePricing := map[string]map[string]interface{}{
		"t3.medium": {
			"hourly_usd": 0.0416,
			"monthly_usd": 30.40,
			"vcpu": 2,
			"memory_gib": 4,
			"storage": "EBS only",
			"network_performance": "Up to 5 Gigabit",
		},
		"t3.large": {
			"hourly_usd": 0.0832,
			"monthly_usd": 60.80,
			"vcpu": 2,
			"memory_gib": 8,
			"storage": "EBS only",
			"network_performance": "Up to 5 Gigabit",
		},
		"t3.xlarge": {
			"hourly_usd": 0.1664,
			"monthly_usd": 121.60,
			"vcpu": 4,
			"memory_gib": 16,
			"storage": "EBS only",
			"network_performance": "Up to 5 Gigabit",
		},
		"m5.large": {
			"hourly_usd": 0.096,
			"monthly_usd": 70.08,
			"vcpu": 2,
			"memory_gib": 8,
			"storage": "EBS only",
			"network_performance": "Up to 10 Gigabit",
		},
		"m5.xlarge": {
			"hourly_usd": 0.192,
			"monthly_usd": 140.16,
			"vcpu": 4,
			"memory_gib": 16,
			"storage": "EBS only",
			"network_performance": "Up to 10 Gigabit",
		},
		"c5.large": {
			"hourly_usd": 0.085,
			"monthly_usd": 62.05,
			"vcpu": 2,
			"memory_gib": 4,
			"storage": "EBS only",
			"network_performance": "Up to 10 Gigabit",
		},
		"r5.large": {
			"hourly_usd": 0.126,
			"monthly_usd": 91.98,
			"vcpu": 2,
			"memory_gib": 16,
			"storage": "EBS only",
			"network_performance": "Up to 10 Gigabit",
		},
	}

	// Get instance type from request (simplified - in real implementation parse from request.Params)
	instanceType := "t3.large" // default
	pricing, exists := instancePricing[instanceType]
	if !exists {
		pricing = instancePricing["t3.large"]
	}

	response := map[string]interface{}{
		"instance_type": instanceType,
		"region": "us-east-1",
		"pricing": pricing,
		"currency": "USD",
		"price_per_hour": fmt.Sprintf("%.4f", pricing["hourly_usd"]),
		"price_per_month": fmt.Sprintf("%.2f", pricing["monthly_usd"]),
		"specifications": map[string]interface{}{
			"vcpu": pricing["vcpu"],
			"memory_gib": pricing["memory_gib"],
			"storage": pricing["storage"],
			"network_performance": pricing["network_performance"],
		},
	}

	return SuccessResult(response)
}
