package embedded

import (
	_ "embed"
)

// DemoBundle represents an embedded demo bundle
type DemoBundle struct {
	ID          string
	Name        string
	Description string
	Category    string
	Data        []byte
}

//go:embed finops-investigations.tar.gz
var finopsInvestigationsData []byte

//go:embed finops-opportunities.tar.gz
var finopsOpportunitiesData []byte

//go:embed finops-projections.tar.gz
var finopsProjectionsData []byte

//go:embed finops-inventory.tar.gz
var finopsInventoryData []byte

//go:embed security-investigations.tar.gz
var securityInvestigationsData []byte

//go:embed reliability-investigations.tar.gz
var reliabilityInvestigationsData []byte

//go:embed aws-k8s-grafana.tar.gz
var awsK8sGrafanaData []byte

//go:embed aws-stripe-cogs.tar.gz
var awsStripeCogsData []byte

//go:embed gcp-gke-bigquery-grafana.tar.gz
var gcpGkeBigqueryGrafanaData []byte

//go:embed aws-serverless-datadog.tar.gz
var awsServerlessDatadogData []byte

// GetDemoBundles returns all available embedded demo bundles
func GetDemoBundles() []DemoBundle {
	return []DemoBundle{
		{
			ID:          "finops-investigations",
			Name:        "FinOps Cost Investigations",
			Description: "Comprehensive cost spike root cause analysis agents. Investigates AWS anomalies, service unit cost changes, storage/egress spikes, K8s drift, third-party SaaS costs, and multi-cloud anomalies.",
			Category:    "FinOps",
			Data:        finopsInvestigationsData,
		},
		{
			ID:          "finops-opportunities",
			Name:        "FinOps Cost Optimization",
			Description: "Cost savings and optimization agents. Identifies EC2 rightsizing, RDS optimization, Reserved Instance opportunities, K8s resource optimization, orphaned resources, and budget optimization.",
			Category:    "FinOps",
			Data:        finopsOpportunitiesData,
		},
		{
			ID:          "finops-projections",
			Name:        "FinOps Cost Forecasting",
			Description: "Financial planning and projection agents. Generates monthly forecasts, quarterly budgets, growth impact models, K8s capacity projections, commitment expiration planning, and seasonal variance predictions.",
			Category:    "FinOps",
			Data:        finopsProjectionsData,
		},
		{
			ID:          "finops-inventory",
			Name:        "FinOps Resource Inventory",
			Description: "Resource tracking and cost attribution agents. Catalogs multi-cloud resources, K8s workloads, commitment portfolios, cost center aggregation, budget vs actual tracking, and infrastructure drift detection.",
			Category:    "FinOps",
			Data:        finopsInventoryData,
		},
		{
			ID:          "security-investigations",
			Name:        "Security Threat Investigations",
			Description: "Security threat analysis and vulnerability assessment agents. Analyzes AWS GuardDuty findings for active threats and assesses CVE exploitability using AWS Inspector with EPSS and CISA KEV integration.",
			Category:    "Security",
			Data:        securityInvestigationsData,
		},
		{
			ID:          "reliability-investigations",
			Name:        "Reliability Incident Analysis",
			Description: "SRE incident response and performance regression detection agents. Performs root cause analysis using CloudWatch metrics and X-Ray traces, correlates with deployments, and provides data-driven remediation guidance.",
			Category:    "Reliability",
			Data:        reliabilityInvestigationsData,
		},
		{
			ID:          "aws-k8s-grafana",
			Name:        "AWS + K8s + Grafana FinOps",
			Description: "Complete FinOps suite for AWS EKS with Grafana observability. Includes cost spike RCA, EC2/container rightsizing, monthly forecasting, resource inventory tracking, and billing event correlation.",
			Category:    "FinOps",
			Data:        awsK8sGrafanaData,
		},
		{
			ID:          "aws-stripe-cogs",
			Name:        "AWS + Stripe COGS Analytics",
			Description: "Financial operations agents for AWS + Stripe SaaS platforms. Analyzes payment processing fees, calculates COGS by customer cohort, optimizes payment routing, and tracks infrastructure cost attribution.",
			Category:    "FinOps",
			Data:        awsStripeCogsData,
		},
		{
			ID:          "gcp-gke-bigquery-grafana",
			Name:        "GCP + BigQuery + Grafana FinOps",
			Description: "GCP cost analysis using BigQuery billing exports and GKE metrics. Performs MoM billing delta RCA, correlates with GKE scaling events, and provides monthly cost forecasting with confidence intervals.",
			Category:    "FinOps",
			Data:        gcpGkeBigqueryGrafanaData,
		},
		{
			ID:          "aws-serverless-datadog",
			Name:        "AWS Serverless + Datadog FinOps",
			Description: "Serverless cost optimization using Datadog telemetry. Analyzes Lambda spend patterns, identifies memory/concurrency rightsizing opportunities, detects cold start cost drivers, and forecasts based on invocation trends.",
			Category:    "FinOps",
			Data:        awsServerlessDatadogData,
		},
	}
}

// GetDemoBundle returns a specific demo bundle by ID
func GetDemoBundle(id string) *DemoBundle {
	bundles := GetDemoBundles()
	for _, bundle := range bundles {
		if bundle.ID == id {
			return &bundle
		}
	}
	return nil
}
