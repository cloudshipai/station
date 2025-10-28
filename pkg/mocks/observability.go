package mocks

import (
	"context"
	"time"

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
				"service":               "api-gateway",
				"bytes_per_day":         1234567890,
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
		"Mock Stripe MCP server matching official Stripe MCP tool signatures",
	)

	// get_stripe_account_info - Retrieve account information
	server.RegisterTool(mcp.Tool{
		Name:        "get_stripe_account_info",
		Description: "Retrieve information about the Stripe account",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":     "acct_1234567890",
			"object": "account",
			"business_profile": map[string]interface{}{
				"name":          "Demo Company Inc",
				"support_email": "support@democompany.com",
				"url":           "https://democompany.com",
			},
			"charges_enabled":   true,
			"country":           "US",
			"default_currency":  "usd",
			"details_submitted": true,
			"email":             "admin@democompany.com",
			"payouts_enabled":   true,
			"type":              "standard",
		}
		return SuccessResult(data)
	})

	// retrieve_balance - Get account balance
	server.RegisterTool(mcp.Tool{
		Name:        "retrieve_balance",
		Description: "Retrieve the balance for the Stripe account",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "balance",
			"livemode": false,
			"available": []map[string]interface{}{
				{
					"amount":   245000,
					"currency": "usd",
					"source_types": map[string]interface{}{
						"card": 245000,
					},
				},
			},
			"pending": []map[string]interface{}{
				{
					"amount":   12500,
					"currency": "usd",
					"source_types": map[string]interface{}{
						"card": 12500,
					},
				},
			},
		}
		return SuccessResult(data)
	})

	// list_customers - List customers
	server.RegisterTool(mcp.Tool{
		Name:        "list_customers",
		Description: "List customers with pagination support",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Number of customers to return (max 100)",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/customers",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":       "cus_ABC123",
					"object":   "customer",
					"email":    "customer1@example.com",
					"name":     "John Doe",
					"balance":  0,
					"created":  1704067200,
					"currency": "usd",
					"livemode": false,
					"metadata": map[string]interface{}{},
				},
				{
					"id":       "cus_DEF456",
					"object":   "customer",
					"email":    "customer2@example.com",
					"name":     "Jane Smith",
					"balance":  -500,
					"created":  1704070800,
					"currency": "usd",
					"livemode": false,
					"metadata": map[string]interface{}{},
				},
			},
		}
		return SuccessResult(data)
	})

	// list_invoices - List invoices
	server.RegisterTool(mcp.Tool{
		Name:        "list_invoices",
		Description: "List invoices with pagination support",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Number of invoices to return (max 100)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"description": "Filter by status: draft, open, paid, uncollectible, void",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/invoices",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":                "in_1234567890",
					"object":            "invoice",
					"amount_due":        5000,
					"amount_paid":       5000,
					"customer":          "cus_ABC123",
					"customer_email":    "customer1@example.com",
					"customer_name":     "John Doe",
					"currency":          "usd",
					"status":            "paid",
					"total":             5000,
					"created":           1704067200,
					"collection_method": "charge_automatically",
				},
				{
					"id":                "in_0987654321",
					"object":            "invoice",
					"amount_due":        12500,
					"amount_paid":       0,
					"customer":          "cus_DEF456",
					"customer_email":    "customer2@example.com",
					"customer_name":     "Jane Smith",
					"currency":          "usd",
					"status":            "open",
					"total":             12500,
					"created":           1704070800,
					"collection_method": "send_invoice",
				},
			},
		}
		return SuccessResult(data)
	})

	// list_subscriptions - List subscriptions
	server.RegisterTool(mcp.Tool{
		Name:        "list_subscriptions",
		Description: "List active subscriptions",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Number of subscriptions to return (max 100)",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/subscriptions",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":                   "sub_ABC123",
					"object":               "subscription",
					"customer":             "cus_ABC123",
					"status":               "active",
					"current_period_end":   1735689600,
					"current_period_start": 1733011200,
					"created":              1704067200,
					"currency":             "usd",
				},
			},
		}
		return SuccessResult(data)
	})

	// list_products - List products
	server.RegisterTool(mcp.Tool{
		Name:        "list_products",
		Description: "List products from the catalog",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/products",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":          "prod_ABC123",
					"object":      "product",
					"name":        "Professional Plan",
					"description": "Full-featured professional subscription",
					"active":      true,
					"created":     1704067200,
					"livemode":    false,
				},
			},
		}
		return SuccessResult(data)
	})

	// list_prices - List prices
	server.RegisterTool(mcp.Tool{
		Name:        "list_prices",
		Description: "List prices for products",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/prices",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":          "price_ABC123",
					"object":      "price",
					"product":     "prod_ABC123",
					"unit_amount": 4900,
					"currency":    "usd",
					"recurring": map[string]interface{}{
						"interval":       "month",
						"interval_count": 1,
					},
					"active":   true,
					"livemode": false,
				},
			},
		}
		return SuccessResult(data)
	})

	// create_coupon - Create a new coupon
	server.RegisterTool(mcp.Tool{
		Name:        "create_coupon",
		Description: "Create a new coupon for discounts",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"duration": map[string]interface{}{
					"type":        "string",
					"description": "Duration: forever, once, repeating",
				},
				"percent_off": map[string]interface{}{
					"type":        "number",
					"description": "Percentage discount (0-100)",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":             "SUMMER25",
			"object":         "coupon",
			"percent_off":    25.0,
			"duration":       "once",
			"created":        time.Now().Unix(),
			"livemode":       false,
			"valid":          true,
			"times_redeemed": 0,
			"metadata":       map[string]interface{}{},
		}
		return SuccessResult(data)
	})

	// list_coupons - List all coupons
	server.RegisterTool(mcp.Tool{
		Name:        "list_coupons",
		Description: "List all coupons",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/coupons",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":             "SUMMER25",
					"object":         "coupon",
					"percent_off":    25.0,
					"duration":       "once",
					"valid":          true,
					"times_redeemed": 142,
				},
			},
		}
		return SuccessResult(data)
	})

	// create_customer - Create a new customer
	server.RegisterTool(mcp.Tool{
		Name:        "create_customer",
		Description: "Create a new customer",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"email": map[string]interface{}{
					"type":        "string",
					"description": "Customer email address",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Customer full name",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":       "cus_NEW123",
			"object":   "customer",
			"email":    "newcustomer@example.com",
			"name":     "New Customer",
			"balance":  0,
			"created":  time.Now().Unix(),
			"currency": "usd",
			"livemode": false,
			"metadata": map[string]interface{}{},
		}
		return SuccessResult(data)
	})

	// list_disputes - List disputes
	server.RegisterTool(mcp.Tool{
		Name:        "list_disputes",
		Description: "List payment disputes",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/disputes",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":       "dp_1ABC123",
					"object":   "dispute",
					"amount":   2500,
					"currency": "usd",
					"status":   "needs_response",
					"reason":   "fraudulent",
					"created":  time.Now().Add(-72 * time.Hour).Unix(),
				},
			},
		}
		return SuccessResult(data)
	})

	// update_dispute - Update dispute information
	server.RegisterTool(mcp.Tool{
		Name:        "update_dispute",
		Description: "Update a dispute with evidence or metadata",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"dispute_id": map[string]interface{}{
					"type":        "string",
					"description": "Dispute ID to update",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":     "dp_1ABC123",
			"object": "dispute",
			"status": "under_review",
			"evidence": map[string]interface{}{
				"customer_name":          "John Doe",
				"customer_email_address": "john@example.com",
				"billing_address":        "123 Main St",
				"receipt":                "rcpt_123",
			},
		}
		return SuccessResult(data)
	})

	// create_invoice - Create a new invoice
	server.RegisterTool(mcp.Tool{
		Name:        "create_invoice",
		Description: "Create a new invoice for a customer",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"customer": map[string]interface{}{
					"type":        "string",
					"description": "Customer ID",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":                "in_NEW123",
			"object":            "invoice",
			"customer":          "cus_ABC123",
			"status":            "draft",
			"amount_due":        0,
			"amount_paid":       0,
			"currency":          "usd",
			"created":           time.Now().Unix(),
			"collection_method": "charge_automatically",
		}
		return SuccessResult(data)
	})

	// create_invoice_item - Add line item to invoice
	server.RegisterTool(mcp.Tool{
		Name:        "create_invoice_item",
		Description: "Add a line item to an invoice",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"customer": map[string]interface{}{
					"type":        "string",
					"description": "Customer ID",
				},
				"amount": map[string]interface{}{
					"type":        "number",
					"description": "Amount in cents",
				},
				"currency": map[string]interface{}{
					"type":        "string",
					"description": "Currency code",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":          "ii_ABC123",
			"object":      "invoiceitem",
			"customer":    "cus_ABC123",
			"amount":      5000,
			"currency":    "usd",
			"description": "Professional services",
			"created":     time.Now().Unix(),
		}
		return SuccessResult(data)
	})

	// finalize_invoice - Finalize a draft invoice
	server.RegisterTool(mcp.Tool{
		Name:        "finalize_invoice",
		Description: "Finalize a draft invoice to make it payable",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"invoice_id": map[string]interface{}{
					"type":        "string",
					"description": "Invoice ID to finalize",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":         "in_NEW123",
			"object":     "invoice",
			"status":     "open",
			"amount_due": 5000,
			"created":    time.Now().Unix(),
		}
		return SuccessResult(data)
	})

	// create_payment_link - Create payment link
	server.RegisterTool(mcp.Tool{
		Name:        "create_payment_link",
		Description: "Create a shareable payment link",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"line_items": map[string]interface{}{
					"type":        "array",
					"description": "Line items for payment",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":     "plink_ABC123",
			"object": "payment_link",
			"active": true,
			"url":    "https://buy.stripe.com/test_abc123",
		}
		return SuccessResult(data)
	})

	// list_payment_intents - List payment intents
	server.RegisterTool(mcp.Tool{
		Name:        "list_payment_intents",
		Description: "List payment intents",
		InputSchema: mcp.ToolInputSchema{Type: "object", Properties: map[string]interface{}{}},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "list",
			"url":      "/v1/payment_intents",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":                   "pi_ABC123",
					"object":               "payment_intent",
					"amount":               5000,
					"currency":             "usd",
					"status":               "succeeded",
					"customer":             "cus_ABC123",
					"payment_method_types": []string{"card"},
					"created":              time.Now().Add(-24 * time.Hour).Unix(),
				},
			},
		}
		return SuccessResult(data)
	})

	// create_price - Create a new price
	server.RegisterTool(mcp.Tool{
		Name:        "create_price",
		Description: "Create a new price for a product",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"product": map[string]interface{}{
					"type":        "string",
					"description": "Product ID",
				},
				"unit_amount": map[string]interface{}{
					"type":        "number",
					"description": "Price in cents",
				},
				"currency": map[string]interface{}{
					"type":        "string",
					"description": "Currency code",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":          "price_NEW123",
			"object":      "price",
			"product":     "prod_ABC123",
			"unit_amount": 9900,
			"currency":    "usd",
			"active":      true,
			"created":     time.Now().Unix(),
		}
		return SuccessResult(data)
	})

	// create_product - Create a new product
	server.RegisterTool(mcp.Tool{
		Name:        "create_product",
		Description: "Create a new product",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Product name",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":          "prod_NEW123",
			"object":      "product",
			"name":        "New Product",
			"description": "Product description",
			"active":      true,
			"created":     time.Now().Unix(),
		}
		return SuccessResult(data)
	})

	// create_refund - Create refund for payment
	server.RegisterTool(mcp.Tool{
		Name:        "create_refund",
		Description: "Refund a payment",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"payment_intent": map[string]interface{}{
					"type":        "string",
					"description": "Payment intent ID to refund",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":             "re_ABC123",
			"object":         "refund",
			"amount":         5000,
			"currency":       "usd",
			"payment_intent": "pi_ABC123",
			"status":         "succeeded",
			"created":        time.Now().Unix(),
		}
		return SuccessResult(data)
	})

	// cancel_subscription - Cancel subscription
	server.RegisterTool(mcp.Tool{
		Name:        "cancel_subscription",
		Description: "Cancel a customer subscription",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"subscription_id": map[string]interface{}{
					"type":        "string",
					"description": "Subscription ID to cancel",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":          "sub_ABC123",
			"object":      "subscription",
			"status":      "canceled",
			"canceled_at": time.Now().Unix(),
			"ended_at":    time.Now().Unix(),
		}
		return SuccessResult(data)
	})

	// update_subscription - Update subscription
	server.RegisterTool(mcp.Tool{
		Name:        "update_subscription",
		Description: "Update subscription details",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"subscription_id": map[string]interface{}{
					"type":        "string",
					"description": "Subscription ID to update",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":     "sub_ABC123",
			"object": "subscription",
			"status": "active",
			"metadata": map[string]interface{}{
				"updated": "true",
			},
		}
		return SuccessResult(data)
	})

	// search_stripe_resources - Search across resources
	server.RegisterTool(mcp.Tool{
		Name:        "search_stripe_resources",
		Description: "Search across Stripe resources (customers, invoices, etc.)",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Resource type to search",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"object":   "search_result",
			"url":      "/v1/search",
			"has_more": false,
			"data": []map[string]interface{}{
				{
					"id":     "cus_ABC123",
					"object": "customer",
					"email":  "customer@example.com",
				},
			},
		}
		return SuccessResult(data)
	})

	// fetch_stripe_resources - Fetch resource by ID
	server.RegisterTool(mcp.Tool{
		Name:        "fetch_stripe_resources",
		Description: "Fetch a specific Stripe resource by ID",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"resource_id": map[string]interface{}{
					"type":        "string",
					"description": "Resource ID to fetch",
				},
				"resource_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of resource",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"id":     "cus_ABC123",
			"object": "customer",
			"email":  "customer@example.com",
			"name":   "John Doe",
		}
		return SuccessResult(data)
	})

	// search_stripe_documentation - Search Stripe docs
	server.RegisterTool(mcp.Tool{
		Name:        "search_stripe_documentation",
		Description: "Search Stripe API documentation",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Documentation search query",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"title":   "Creating Customers",
					"url":     "https://docs.stripe.com/api/customers/create",
					"snippet": "Create a customer object to track payments...",
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}

func NewCloudWatchMock() *MockServer {
	server := NewMockServer(
		"aws-cloudwatch",
		"1.0.0",
		"Mock AWS CloudWatch MCP server matching awslabs implementation",
	)

	// get_metric_data - Retrieve detailed CloudWatch metric data (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "get_metric_data",
		Description: "Retrieves detailed CloudWatch metric data with timestamps and values",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"metric_name": map[string]interface{}{
					"type":        "string",
					"description": "CloudWatch metric name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "CloudWatch namespace (e.g., AWS/EC2, AWS/Lambda)",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
			},
			Required: []string{"metric_name", "namespace", "start_time", "end_time"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		now := time.Now()
		data := map[string]interface{}{
			"MetricDataResults": []map[string]interface{}{
				{
					"Id":         "m1",
					"Label":      "CPUUtilization",
					"StatusCode": "Complete",
					"Timestamps": []int64{
						now.Add(-20 * time.Minute).Unix(),
						now.Add(-15 * time.Minute).Unix(),
						now.Add(-10 * time.Minute).Unix(),
						now.Add(-5 * time.Minute).Unix(),
					},
					"Values": []float64{45.2, 67.8, 89.3, 52.1},
				},
			},
			"Messages":  []interface{}{},
			"NextToken": "",
		}
		return SuccessResult(data)
	})

	// get_metric_metadata - Retrieve comprehensive metadata about a specific metric (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "get_metric_metadata",
		Description: "Retrieves comprehensive metadata about a specific CloudWatch metric",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"metric_name": map[string]interface{}{
					"type":        "string",
					"description": "Metric name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Namespace",
				},
			},
			Required: []string{"metric_name", "namespace"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"MetricName": "CPUUtilization",
			"Namespace":  "AWS/EC2",
			"Dimensions": []map[string]interface{}{
				{"Name": "InstanceId", "Value": "i-1234567890abcdef0"},
			},
			"Statistics": []string{"Average", "Sum", "Maximum", "Minimum", "SampleCount"},
			"Unit":       "Percent",
		}
		return SuccessResult(data)
	})

	// get_recommended_metric_alarms - Gets recommended alarms for a CloudWatch metric (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "get_recommended_metric_alarms",
		Description: "Gets recommended alarms for a CloudWatch metric based on best practices",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"metric_name": map[string]interface{}{
					"type":        "string",
					"description": "Metric name",
				},
				"namespace": map[string]interface{}{
					"type":        "string",
					"description": "Namespace",
				},
			},
			Required: []string{"metric_name", "namespace"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"RecommendedAlarms": []map[string]interface{}{
				{
					"AlarmName":          "HighCPUUtilization",
					"MetricName":         "CPUUtilization",
					"Threshold":          80.0,
					"ComparisonOperator": "GreaterThanThreshold",
					"EvaluationPeriods":  2,
					"Period":             300,
					"Statistic":          "Average",
					"Description":        "Alert when CPU utilization exceeds 80%",
				},
			},
		}
		return SuccessResult(data)
	})

	// get_active_alarms - Identifies currently active CloudWatch alarms across the account (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "get_active_alarms",
		Description: "Identifies currently active CloudWatch alarms across the account",
		InputSchema: mcp.ToolInputSchema{
			Type:       "object",
			Properties: map[string]interface{}{},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		now := time.Now()
		data := map[string]interface{}{
			"MetricAlarms": []map[string]interface{}{
				{
					"AlarmName":                          "HighCPUAlarm",
					"AlarmArn":                           "arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPUAlarm",
					"StateValue":                         "ALARM",
					"StateReason":                        "Threshold Crossed: 1 datapoint [85.0] was greater than the threshold (80.0).",
					"StateUpdatedTimestamp":              now.Format(time.RFC3339),
					"MetricName":                         "CPUUtilization",
					"Namespace":                          "AWS/EC2",
					"Statistic":                          "Average",
					"Dimensions":                         []map[string]interface{}{{"Name": "InstanceId", "Value": "i-1234567890abcdef0"}},
					"Period":                             300,
					"EvaluationPeriods":                  2,
					"Threshold":                          80.0,
					"ComparisonOperator":                 "GreaterThanThreshold",
					"ActionsEnabled":                     true,
					"AlarmActions":                       []string{"arn:aws:sns:us-east-1:123456789012:alert-topic"},
					"AlarmConfigurationUpdatedTimestamp": now.Add(-24 * time.Hour).Format(time.RFC3339),
				},
			},
			"CompositeAlarms": []interface{}{},
			"NextToken":       "",
		}
		return SuccessResult(data)
	})

	// get_alarm_history - Retrieves historical state changes and patterns for a given CloudWatch alarm (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "get_alarm_history",
		Description: "Retrieves historical state changes and patterns for a given CloudWatch alarm",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"alarm_name": map[string]interface{}{
					"type":        "string",
					"description": "Alarm name",
				},
			},
			Required: []string{"alarm_name"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		now := time.Now()
		data := map[string]interface{}{
			"AlarmHistoryItems": []map[string]interface{}{
				{
					"AlarmName":       "HighCPUAlarm",
					"Timestamp":       now.Add(-2 * time.Hour).Format(time.RFC3339),
					"HistoryItemType": "StateUpdate",
					"HistorySummary":  "Alarm updated from OK to ALARM",
					"HistoryData":     "{\"oldState\":{\"stateValue\":\"OK\"},\"newState\":{\"stateValue\":\"ALARM\",\"stateReason\":\"Threshold Crossed\"}}",
				},
				{
					"AlarmName":       "HighCPUAlarm",
					"Timestamp":       now.Add(-24 * time.Hour).Format(time.RFC3339),
					"HistoryItemType": "StateUpdate",
					"HistorySummary":  "Alarm updated from ALARM to OK",
					"HistoryData":     "{\"oldState\":{\"stateValue\":\"ALARM\"},\"newState\":{\"stateValue\":\"OK\",\"stateReason\":\"Threshold no longer breached\"}}",
				},
			},
			"NextToken": "",
		}
		return SuccessResult(data)
	})

	// describe_log_groups - Finds metadata about CloudWatch log groups (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "describe_log_groups",
		Description: "Finds metadata about CloudWatch log groups",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"log_group_name_prefix": map[string]interface{}{
					"type":        "string",
					"description": "Log group name prefix to filter (optional)",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		now := time.Now()
		data := map[string]interface{}{
			"logGroups": []map[string]interface{}{
				{
					"logGroupName":    "/aws/lambda/checkout-service",
					"creationTime":    now.Add(-30*24*time.Hour).Unix() * 1000,
					"retentionInDays": 7,
					"storedBytes":     1048576000,
					"arn":             "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/checkout-service",
				},
				{
					"logGroupName":    "/aws/lambda/payment-processor",
					"creationTime":    now.Add(-60*24*time.Hour).Unix() * 1000,
					"retentionInDays": 14,
					"storedBytes":     524288000,
					"arn":             "arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/payment-processor",
				},
			},
			"nextToken": "",
		}
		return SuccessResult(data)
	})

	// analyze_log_group - Analyzes CloudWatch logs for anomalies, message patterns, and error patterns (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "analyze_log_group",
		Description: "Analyzes CloudWatch logs for anomalies, message patterns, and error patterns",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"log_group": map[string]interface{}{
					"type":        "string",
					"description": "Log group name",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
			},
			Required: []string{"log_group", "start_time", "end_time"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"log_group": "/aws/lambda/checkout-service",
			"analysis": map[string]interface{}{
				"total_events":  15234,
				"error_count":   247,
				"warning_count": 1523,
				"error_rate":    0.0162,
				"top_error_patterns": []map[string]interface{}{
					{"pattern": "Database connection timeout", "occurrences": 89, "percentage": 36.0},
					{"pattern": "Payment gateway returned 503", "occurrences": 152, "percentage": 61.5},
					{"pattern": "Lambda timeout", "occurrences": 6, "percentage": 2.4},
				},
				"anomalies": []map[string]interface{}{
					{
						"timestamp":    "2025-10-06T22:15:00Z",
						"anomaly_type": "ERROR_SPIKE",
						"description":  "Error rate increased from 0.3% to 1.6% (533% increase)",
						"severity":     "HIGH",
					},
				},
			},
		}
		return SuccessResult(data)
	})

	// execute_log_insights_query - Executes CloudWatch Logs insights query (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "execute_log_insights_query",
		Description: "Executes CloudWatch Logs insights query and returns queryId",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"log_group": map[string]interface{}{
					"type":        "string",
					"description": "Log group name",
				},
				"query_string": map[string]interface{}{
					"type":        "string",
					"description": "Logs Insights query string",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time (ISO 8601)",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "End time (ISO 8601)",
				},
			},
			Required: []string{"log_group", "query_string", "start_time", "end_time"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"queryId": "abcd1234-ef56-7890-gh12-ijklmnop3456",
		}
		return SuccessResult(data)
	})

	// get_logs_insight_query_results - Retrieves results of an executed Logs Insights query (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "get_logs_insight_query_results",
		Description: "Retrieves results of an executed Logs Insights query",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query_id": map[string]interface{}{
					"type":        "string",
					"description": "Query ID from execute_log_insights_query",
				},
			},
			Required: []string{"query_id"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"status": "Complete",
			"results": [][]map[string]interface{}{
				{
					{"field": "@timestamp", "value": "2025-10-06T22:15:30.123Z"},
					{"field": "@message", "value": "ERROR: Database connection timeout"},
					{"field": "level", "value": "ERROR"},
				},
				{
					{"field": "@timestamp", "value": "2025-10-06T22:15:31.456Z"},
					{"field": "@message", "value": "ERROR: Payment gateway returned 503"},
					{"field": "level", "value": "ERROR"},
				},
			},
			"statistics": map[string]interface{}{
				"recordsMatched": 247,
				"recordsScanned": 15234,
				"bytesScanned":   10485760,
			},
			"queryLanguage": "CWL",
		}
		return SuccessResult(data)
	})

	// cancel_logs_insight_query - Cancels an in-progress CloudWatch logs insights query (awslabs MCP tool)
	server.RegisterTool(mcp.Tool{
		Name:        "cancel_logs_insight_query",
		Description: "Cancels an in-progress CloudWatch logs insights query",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"query_id": map[string]interface{}{
					"type":        "string",
					"description": "Query ID to cancel",
				},
			},
			Required: []string{"query_id"},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"success": true,
		}
		return SuccessResult(data)
	})

	return server
}

func NewXRayMock() *MockServer {
	server := NewMockServer(
		"aws-xray",
		"1.0.0",
		"Mock AWS X-Ray for distributed tracing and service maps",
	)

	// get_xray_traces - Get X-Ray traces
	server.RegisterTool(mcp.Tool{
		Name:        "get_xray_traces",
		Description: "Get X-Ray traces to identify slow operations, bottlenecks, and error traces",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time for trace query (ISO 8601)",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "End time for trace query (ISO 8601)",
				},
				"filter_expression": map[string]interface{}{
					"type":        "string",
					"description": "X-Ray filter expression (e.g., error = true)",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"traces": []map[string]interface{}{
				{
					"trace_id": "1-67024d2a-abcdef1234567890",
					"duration": 2.547,
					"segments": []map[string]interface{}{
						{
							"id":         "segment-api-gateway",
							"name":       "API Gateway",
							"start_time": 1727846714.123,
							"end_time":   1727846716.670,
							"duration":   2.547,
							"http": map[string]interface{}{
								"request": map[string]interface{}{
									"url":    "https://api.example.com/checkout",
									"method": "POST",
								},
								"response": map[string]interface{}{
									"status": 500,
								},
							},
						},
						{
							"id":         "segment-lambda",
							"name":       "checkout-service",
							"parent_id":  "segment-api-gateway",
							"start_time": 1727846714.234,
							"end_time":   1727846716.556,
							"duration":   2.322,
							"error":      true,
							"fault":      false,
							"cause": map[string]interface{}{
								"exception_type": "DatabaseConnectionTimeout",
								"message":        "Connection pool exhausted after 30s",
								"stack": []string{
									"at processPayment (index.js:45:10)",
									"at handler (index.js:23:5)",
								},
							},
						},
						{
							"id":         "segment-rds",
							"name":       "payments-db",
							"parent_id":  "segment-lambda",
							"start_time": 1727846714.345,
							"end_time":   1727846716.450,
							"duration":   2.105,
							"error":      true,
							"fault":      true,
							"sql": map[string]interface{}{
								"query":          "SELECT * FROM payments WHERE order_id = ?",
								"connection_url": "payments-db.cluster-abc123.us-east-1.rds.amazonaws.com:5432",
							},
						},
					},
					"response_time": 2.547,
					"http": map[string]interface{}{
						"status": 500,
					},
					"error": true,
				},
				{
					"trace_id": "1-67024d3b-1234567890abcdef",
					"duration": 0.156,
					"segments": []map[string]interface{}{
						{
							"id":         "segment-alb",
							"name":       "Application Load Balancer",
							"start_time": 1727846731.100,
							"end_time":   1727846731.256,
							"duration":   0.156,
							"http": map[string]interface{}{
								"request": map[string]interface{}{
									"url":    "https://api.example.com/health",
									"method": "GET",
								},
								"response": map[string]interface{}{
									"status": 200,
								},
							},
						},
					},
					"response_time": 0.156,
					"http": map[string]interface{}{
						"status": 200,
					},
				},
			},
			"trace_summary": map[string]interface{}{
				"total_traces":     2,
				"error_traces":     1,
				"fault_traces":     1,
				"average_duration": 1.3515,
				"p99_duration":     2.547,
			},
		}
		return SuccessResult(data)
	})

	// analyze_xray_service_graph - Analyze X-Ray service graph
	server.RegisterTool(mcp.Tool{
		Name:        "analyze_xray_service_graph",
		Description: "Analyze X-Ray service graph to identify service dependencies and performance bottlenecks",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time for analysis (ISO 8601)",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "End time for analysis (ISO 8601)",
				},
			},
		},
	}, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		data := map[string]interface{}{
			"services": []map[string]interface{}{
				{
					"name":       "api-gateway",
					"type":       "AWS::ApiGateway",
					"account_id": "123456789012",
					"state":      "active",
					"start_time": "2025-10-06T20:00:00Z",
					"end_time":   "2025-10-06T23:00:00Z",
					"summary_statistics": map[string]interface{}{
						"total_count":           15234,
						"ok_count":              14987,
						"error_count":           247,
						"fault_count":           0,
						"total_response_time":   1897.45,
						"average_response_time": 0.124,
					},
					"edges": []map[string]interface{}{
						{
							"destination_name":      "checkout-service",
							"request_count":         15234,
							"error_count":           247,
							"fault_count":           0,
							"average_response_time": 0.245,
						},
					},
				},
				{
					"name":       "checkout-service",
					"type":       "AWS::Lambda::Function",
					"account_id": "123456789012",
					"state":      "active",
					"start_time": "2025-10-06T20:00:00Z",
					"end_time":   "2025-10-06T23:00:00Z",
					"summary_statistics": map[string]interface{}{
						"total_count":           15234,
						"ok_count":              14987,
						"error_count":           247,
						"fault_count":           89,
						"total_response_time":   3734.33,
						"average_response_time": 0.245,
					},
					"edges": []map[string]interface{}{
						{
							"destination_name":      "payments-db",
							"request_count":         15234,
							"error_count":           247,
							"fault_count":           89,
							"average_response_time": 0.187,
						},
						{
							"destination_name":      "payment-gateway-api",
							"request_count":         14987,
							"error_count":           152,
							"fault_count":           0,
							"average_response_time": 0.356,
						},
					},
				},
				{
					"name":       "payments-db",
					"type":       "AWS::RDS::DBInstance",
					"account_id": "123456789012",
					"state":      "degraded",
					"start_time": "2025-10-06T20:00:00Z",
					"end_time":   "2025-10-06T23:00:00Z",
					"summary_statistics": map[string]interface{}{
						"total_count":           15234,
						"ok_count":              14987,
						"error_count":           247,
						"fault_count":           89,
						"total_response_time":   2849.46,
						"average_response_time": 0.187,
					},
				},
			},
			"anomalies": []map[string]interface{}{
				{
					"service_name": "payments-db",
					"anomaly_type": "HIGH_ERROR_RATE",
					"description":  "Error rate increased from 0.3% to 1.6% (533% increase)",
					"severity":     "HIGH",
					"detected_at":  "2025-10-06T22:00:00Z",
				},
				{
					"service_name": "payments-db",
					"anomaly_type": "INCREASED_LATENCY",
					"description":  "P99 latency increased from 150ms to 2100ms (1400% increase)",
					"severity":     "CRITICAL",
					"detected_at":  "2025-10-06T22:10:00Z",
				},
			},
		}
		return SuccessResult(data)
	})

	return server
}
