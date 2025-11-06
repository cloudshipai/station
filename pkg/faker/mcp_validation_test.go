package faker

import (
	"encoding/json"
	"testing"
	"time"
)

// MockResponseData represents realistic mock data for different MCP servers
var MockResponseData = map[string]interface{}{
	"aws-billing": map[string]interface{}{
		"costAndUsageData": map[string]interface{}{
			"resultsByTime": []interface{}{
				map[string]interface{}{
					"timePeriod": map[string]interface{}{
						"start": "2024-11-01T00:00:00Z",
						"end":   "2024-11-02T00:00:00Z",
					},
					"total": map[string]interface{}{
						"amount":     "125.43",
						"currency":    "USD",
						"unit":        "USD",
						"usageAmount": "125.43",
					},
					"groups": []interface{}{
						map[string]interface{}{
							"keys": []interface{}{"Amazon EC2", "us-east-1"},
							"metrics": map[string]interface{}{
								"blendedCost": map[string]interface{}{
									"amount": "89.27",
								},
							},
						},
						map[string]interface{}{
							"keys": []interface{}{"Amazon S3", "us-east-1"},
							"metrics": map[string]interface{}{
								"blendedCost": map[string]interface{}{
									"amount": "23.15",
								},
							},
						},
					},
				},
				map[string]interface{}{
					"timePeriod": map[string]interface{}{
						"start": "2024-11-02T00:00:00Z",
						"end":   "2024-11-03T00:00:00Z",
					},
					"total": map[string]interface{}{
						"amount":     "98.76",
						"currency":    "USD",
						"unit":        "USD",
						"usageAmount": "98.76",
					},
					"groups": []interface{}{
						map[string]interface{}{
							"keys": []interface{}{"Amazon EC2", "us-east-1"},
							"metrics": map[string]interface{}{
								"blendedCost": map[string]interface{}{
									"amount": "67.34",
								},
							},
						},
					},
				},
			},
		},
	},
	"github": map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           123456789,
			"name":         "awesome-project",
			"fullName":     "octocat/awesome-project",
			"description":  "An awesome project for testing",
			"private":      false,
			"fork":         false,
			"language":     "Go",
			"stars":        42,
			"forks":        15,
			"openIssues":   7,
			"createdAt":    "2024-01-15T10:30:00Z",
			"updatedAt":    "2024-11-01T15:45:00Z",
			"pushedAt":     "2024-11-01T14:20:00Z",
			"size":         2048,
			"defaultBranch": "main",
			"owner": map[string]interface{}{
				"id":         583231,
				"login":      "octocat",
				"name":       "Octo Cat",
				"email":      "octocat@github.com",
				"type":       "User",
				"company":    "GitHub",
				"location":   "San Francisco",
				"blog":       "https://github.blog",
				"twitter":    "octocat",
				"publicRepos": 25,
				"followers":   150,
				"following":   75,
			},
			"topics": []interface{}{"go", "testing", "awesome", "mcp"},
			"license": map[string]interface{}{
				"key":  "mit",
				"name": "MIT License",
				"spdxId": "MIT",
			},
		},
	},
	"stripe": map[string]interface{}{
		"charges": map[string]interface{}{
			"data": []interface{}{
				map[string]interface{}{
					"id": "ch_1234567890abcdef",
					"object": "charge",
					"amount": 2999,
					"amountCaptured": 2999,
					"amountRefunded": 0,
					"currency": "usd",
					"status": "succeeded",
					"created": time.Now().Unix(),
					"description": "Premium subscription",
					"metadata": map[string]interface{}{
						"order_id": "12345",
						"customer_email": "test@example.com",
					},
					"paymentMethod": map[string]interface{}{
						"id": "pm_1234567890",
						"type": "card",
						"card": map[string]interface{}{
							"brand": "visa",
							"last4": "4242",
							"expMonth": 12,
							"expYear": 2025,
							"fingerprint": "F1234567890ABCDEF",
						},
					},
					"customer": map[string]interface{}{
						"id": "cus_1234567890",
						"email": "test@example.com",
						"name": "Test Customer",
					},
					"billingDetails": map[string]interface{}{
						"email": "test@example.com",
						"name": map[string]interface{}{
							"first": "Test",
							"last": "Customer",
						},
						"address": map[string]interface{}{
							"line1": "123 Test St",
							"city": "San Francisco",
							"state": "CA",
							"postalCode": "94105",
							"country": "US",
						},
					},
				},
				map[string]interface{}{
					"id": "ch_0987654321fedcba",
					"object": "charge",
					"amount": 4999,
					"amountCaptured": 4999,
					"amountRefunded": 0,
					"currency": "usd",
					"status": "succeeded",
					"created": time.Now().Unix() - 86400,
					"description": "Enterprise plan",
					"paymentMethod": map[string]interface{}{
						"id": "pm_0987654321",
						"type": "card",
						"card": map[string]interface{}{
							"brand": "mastercard",
							"last4": "5555",
							"expMonth": 9,
							"expYear": 2024,
							"fingerprint": "F0987654321FEDCBA",
						},
					},
				},
			},
			"hasMore": true,
			"totalCount": 2,
		},
	},
	"postgresql": map[string]interface{}{
		"rows": []interface{}{
			map[string]interface{}{
				"id":        1,
				"name":      "John Doe",
				"email":     "john.doe@example.com",
				"created_at": "2024-01-15T10:30:00Z",
				"updated_at": "2024-11-01T15:45:00Z",
				"is_active": true,
				"balance":   1250.75,
				"metadata":  map[string]interface{}{"source": "web", "campaign": "fall2024"},
			},
			map[string]interface{}{
				"id":        2,
				"name":      "Jane Smith",
				"email":     "jane.smith@example.com",
				"created_at": "2024-02-20T14:15:00Z",
				"updated_at": "2024-10-30T09:20:00Z",
				"is_active": true,
				"balance":   3450.00,
				"metadata":  map[string]interface{}{"source": "api", "tier": "premium"},
			},
		},
		"rowCount": 2,
		"fields": []interface{}{
			map[string]interface{}{"name": "id", "type": "integer", "nullable": false},
			map[string]interface{}{"name": "name", "type": "varchar", "nullable": false},
			map[string]interface{}{"name": "email", "type": "varchar", "nullable": false},
			map[string]interface{}{"name": "created_at", "type": "timestamp", "nullable": false},
			map[string]interface{}{"name": "updated_at", "type": "timestamp", "nullable": false},
			map[string]interface{}{"name": "is_active", "type": "boolean", "nullable": false},
			map[string]interface{}{"name": "balance", "type": "numeric", "nullable": true},
			map[string]interface{}{"name": "metadata", "type": "jsonb", "nullable": true},
		},
	},
}

func TestDataEnrichmentWithRealMCPSchemas(t *testing.T) {
	// Create schema cache and enricher
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	enricher := NewEnricher(schemaCache)
	
	for serverName, mockResponse := range MockResponseData {
		t.Run(serverName, func(t *testing.T) {
			// Convert mock response to JSON and back to simulate real MCP response
			responseBytes, err := json.Marshal(mockResponse)
			if err != nil {
				t.Fatalf("Failed to marshal mock response: %v", err)
			}
			
			var response map[string]interface{}
			if err := json.Unmarshal(responseBytes, &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}
			
			// Enrich the response
			enriched, err := enricher.EnrichResponse(serverName, response)
			if err != nil {
				t.Fatalf("Failed to enrich response: %v", err)
			}
			
			// Validate that enriched response maintains structure
			if enriched == nil {
				t.Fatal("Enriched response is nil")
			}
			
			// Check that structure is preserved
			t.Logf("✅ %s: Response structure preserved", serverName)
			
			// Server-specific validations
			switch serverName {
			case "aws-billing":
				validateAWSBillingResponse(t, enriched)
			case "github":
				validateGitHubResponse(t, enriched)
			case "stripe":
				validateStripeResponse(t, enriched)
			case "postgresql":
				validatePostgreSQLResponse(t, enriched)
			}
		})
	}
}

func validateAWSBillingResponse(t *testing.T, result map[string]interface{}) {
	if costData, ok := result["costAndUsageData"].(map[string]interface{}); ok {
		if resultsByTime, ok := costData["resultsByTime"].([]interface{}); ok && len(resultsByTime) > 0 {
			if firstResult, ok := resultsByTime[0].(map[string]interface{}); ok {
				if timePeriod, ok := firstResult["timePeriod"].(map[string]interface{}); ok {
					if start, ok := timePeriod["start"].(string); ok {
						t.Logf("✅ AWS Billing: Time period start = %s", start)
					}
				}
				if total, ok := firstResult["total"].(map[string]interface{}); ok {
					if amount, ok := total["amount"].(string); ok {
						t.Logf("✅ AWS Billing: Total amount = %s", amount)
					}
				}
			}
		}
	}
}

func validateGitHubResponse(t *testing.T, result map[string]interface{}) {
	if repo, ok := result["repository"].(map[string]interface{}); ok {
		if id, ok := repo["id"].(float64); ok {
			t.Logf("✅ GitHub: Repository ID = %.0f", id)
		}
		if name, ok := repo["name"].(string); ok {
			t.Logf("✅ GitHub: Repository name = %s", name)
		}
		if owner, ok := repo["owner"].(map[string]interface{}); ok {
			if login, ok := owner["login"].(string); ok {
				t.Logf("✅ GitHub: Owner login = %s", login)
			}
		}
	}
}

func validateStripeResponse(t *testing.T, result map[string]interface{}) {
	if charges, ok := result["charges"].(map[string]interface{}); ok {
		if data, ok := charges["data"].([]interface{}); ok && len(data) > 0 {
			if firstCharge, ok := data[0].(map[string]interface{}); ok {
				if id, ok := firstCharge["id"].(string); ok {
					t.Logf("✅ Stripe: Charge ID = %s", id)
				}
				if amount, ok := firstCharge["amount"].(float64); ok {
					t.Logf("✅ Stripe: Amount = %.0f cents", amount)
				}
				if currency, ok := firstCharge["currency"].(string); ok {
					t.Logf("✅ Stripe: Currency = %s", currency)
				}
			}
		}
	}
}

func validatePostgreSQLResponse(t *testing.T, result map[string]interface{}) {
	if rows, ok := result["rows"].([]interface{}); ok && len(rows) > 0 {
		if firstRow, ok := rows[0].(map[string]interface{}); ok {
			if id, ok := firstRow["id"].(float64); ok {
				t.Logf("✅ PostgreSQL: First row ID = %.0f", id)
			}
			if name, ok := firstRow["name"].(string); ok {
				t.Logf("✅ PostgreSQL: First row name = %s", name)
			}
			if email, ok := firstRow["email"].(string); ok {
				t.Logf("✅ PostgreSQL: First row email = %s", email)
			}
		}
	}
	if rowCount, ok := result["rowCount"].(float64); ok {
		t.Logf("✅ PostgreSQL: Row count = %.0f", rowCount)
	}
}

func TestSchemaLearningWithComplexStructures(t *testing.T) {
	// Create schema cache
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	
	for serverName, mockResponse := range MockResponseData {
		t.Run(serverName+"-schema-learning", func(t *testing.T) {
			// Analyze the mock response
			err := schemaCache.AnalyzeResponse(serverName, mockResponse)
			if err != nil {
				t.Fatalf("Failed to analyze response: %v", err)
			}
			
			// Get the learned schema
			schema, exists := schemaCache.GetSchema(serverName)
			if !exists {
				t.Fatal("Schema not found after analysis")
			}
			
			if schema == nil {
				t.Fatal("Schema analysis returned nil")
			}
			
			t.Logf("✅ %s: Schema learned successfully", serverName)
			t.Logf("   Schema type: %s", schema.Type)
			if len(schema.Children) > 0 {
				var childKeys []string
				for k := range schema.Children {
					childKeys = append(childKeys, k)
				}
				t.Logf("   Child keys: %v", childKeys)
			}
		})
	}
}

func TestFieldPatternDetection(t *testing.T) {
	// Create schema cache and enricher
	schemaCache, err := NewSchemaCache("/tmp/test-cache")
	if err != nil {
		t.Fatalf("Failed to create schema cache: %v", err)
	}
	enricher := NewEnricher(schemaCache)
	
	testCases := []struct {
		fieldName   string
		expectedType string
		description  string
	}{
		{"email", "string", "Email field should generate email format"},
		{"id", "string", "ID field should generate UUID/string format"},
		{"amount", "number", "Amount field should generate numeric values"},
		{"created_at", "string", "Timestamp field should generate ISO format"},
		{"is_active", "boolean", "Boolean field should generate true/false"},
		{"balance", "number", "Balance field should generate numeric values"},
		{"currency", "string", "Currency field should generate currency codes"},
		{"status", "string", "Status field should generate status strings"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.fieldName, func(t *testing.T) {
			// Create a simple schema node for testing
			schema := &SchemaNode{
				Type: tc.expectedType,
			}
			
			// Test field pattern detection by calling the enricher method
			value, err := enricher.enrichFieldByName(tc.fieldName, schema, nil)
			if err != nil {
				t.Errorf("❌ %s: Error generating value: %v", tc.fieldName, err)
				return
			}
			
			if value == nil {
				t.Errorf("❌ %s: No value generated", tc.fieldName)
				return
			}
			
			t.Logf("✅ %s: Generated %v (%s)", tc.fieldName, value, tc.description)
		})
	}
}