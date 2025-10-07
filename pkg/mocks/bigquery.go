package mocks

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// BigQueryMock provides a mock BigQuery MCP server
// Returns realistic billing export data for GCP FinOps scenarios
type BigQueryMock struct {
	*MockServer
}

// NewBigQueryMock creates a new BigQuery mock server
func NewBigQueryMock() *BigQueryMock {
	mock := &BigQueryMock{
		MockServer: NewMockServer(
			"bigquery",
			"1.0.0",
			"Mock BigQuery MCP server for GCP billing export queries and analytics",
		),
	}

	// Register tools
	mock.registerTools()

	return mock
}

func (m *BigQueryMock) registerTools() {
	// execute-query - Execute BigQuery SQL and return results
	m.RegisterTool(mcp.Tool{
		Name:        "execute-query",
		Description: "Execute a BigQuery SQL query and return results (supports billing export queries, aggregations, and analytics)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "BigQuery Standard SQL query",
				},
				"project_id": map[string]interface{}{
					"type":        "string",
					"description": "GCP project ID (optional)",
				},
				"use_legacy_sql": map[string]interface{}{
					"type":        "boolean",
					"description": "Use legacy SQL syntax (default: false)",
					"default":     false,
				},
			},
			Required: []string{"query"},
		},
	}, m.handleExecuteQuery)
}

func (m *BigQueryMock) handleExecuteQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse query to determine what kind of data to return
	// For now, return realistic GCP billing export data

	now := time.Now()
	services := []string{"Compute Engine", "Cloud Storage", "BigQuery", "Cloud SQL", "Kubernetes Engine", "Cloud Functions", "Cloud CDN"}
	projects := []string{"prod-api", "prod-workers", "staging", "analytics"}
	skus := map[string][]string{
		"Compute Engine": {"N1 Predefined Instance Core running in Americas", "N1 Predefined Instance Ram running in Americas", "Storage PD Capacity"},
		"Cloud Storage":  {"Standard Storage US Multi-region", "Network Egress"},
		"BigQuery":       {"Analysis", "Storage"},
		"Cloud SQL":      {"MySQL: Database instance core hour", "MySQL: Database instance RAM"},
		"Kubernetes Engine": {"Cluster management fee", "Small cluster in Americas"},
	}

	var rows []map[string]interface{}

	// Generate 30 days of billing data
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")

		for _, service := range services {
			for _, project := range projects {
				// Not every service runs in every project
				if rand.Float64() < 0.3 {
					continue
				}

				serviceSKUs := skus[service]
				if len(serviceSKUs) == 0 {
					serviceSKUs = []string{service + " - Default SKU"}
				}

				for _, sku := range serviceSKUs {
					cost := rand.Float64() * 500
					usage := rand.Float64() * 1000

					rows = append(rows, map[string]interface{}{
						"usage_date":   date,
						"service":      service,
						"sku":          sku,
						"project_id":   project,
						"project_name": project,
						"cost":         fmt.Sprintf("%.2f", cost),
						"currency":     "USD",
						"usage_amount": fmt.Sprintf("%.4f", usage),
						"usage_unit":   "hours",
						"labels": map[string]string{
							"environment": map[string]string{"prod-api": "production", "prod-workers": "production", "staging": "staging", "analytics": "production"}[project],
							"team":        map[string]string{"prod-api": "backend", "prod-workers": "backend", "staging": "engineering", "analytics": "data"}[project],
						},
					})
				}
			}
		}
	}

	// Return BigQuery result format
	response := map[string]interface{}{
		"kind": "bigquery#queryResponse",
		"schema": map[string]interface{}{
			"fields": []map[string]interface{}{
				{"name": "usage_date", "type": "DATE", "mode": "NULLABLE"},
				{"name": "service", "type": "STRING", "mode": "NULLABLE"},
				{"name": "sku", "type": "STRING", "mode": "NULLABLE"},
				{"name": "project_id", "type": "STRING", "mode": "NULLABLE"},
				{"name": "project_name", "type": "STRING", "mode": "NULLABLE"},
				{"name": "cost", "type": "FLOAT", "mode": "NULLABLE"},
				{"name": "currency", "type": "STRING", "mode": "NULLABLE"},
				{"name": "usage_amount", "type": "FLOAT", "mode": "NULLABLE"},
				{"name": "usage_unit", "type": "STRING", "mode": "NULLABLE"},
				{"name": "labels", "type": "RECORD", "mode": "REPEATED"},
			},
		},
		"rows":            rows,
		"totalRows":       fmt.Sprintf("%d", len(rows)),
		"jobComplete":     true,
		"cacheHit":        false,
		"totalBytesProcessed": fmt.Sprintf("%d", len(rows)*1024),
	}

	return SuccessResult(response)
}
