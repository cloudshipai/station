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

//go:embed finops-demo.tar.gz
var finopsDemoData []byte

//go:embed finops-investigations.tar.gz
var finopsInvestigationsData []byte

//go:embed finops-opportunities.tar.gz
var finopsOpportunitiesData []byte

//go:embed finops-projections.tar.gz
var finopsProjectionsData []byte

//go:embed finops-inventory.tar.gz
var finopsInventoryData []byte

//go:embed finops-events.tar.gz
var finopsEventsData []byte

//go:embed security-investigations.tar.gz
var securityInvestigationsData []byte

//go:embed security-opportunities.tar.gz
var securityOpportunitiesData []byte

//go:embed reliability-investigations.tar.gz
var reliabilityInvestigationsData []byte

//go:embed deployments-opportunities.tar.gz
var deploymentsOpportunitiesData []byte

// GetDemoBundles returns all available embedded demo bundles
func GetDemoBundles() []DemoBundle {
	return []DemoBundle{
		{
			ID:          "finops-demo",
			Name:        "FinOps Cost Investigation Demo",
			Description: "Interactive demo showcasing AWS cost analysis agents with mock Cost Explorer data. Includes cost spike investigation and forecasting agents.",
			Category:    "FinOps",
			Data:        finopsDemoData,
		},
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
			ID:          "finops-events",
			Name:        "FinOps Cost Events",
			Description: "Cost-impacting event tracking agents. Correlates deployments with costs, tracks incident costs, logs infrastructure changes, analyzes scaling events, predicts PR cost impacts, and logs anomaly events.",
			Category:    "FinOps",
			Data:        finopsEventsData,
		},
		{
			ID:          "security-investigations",
			Name:        "Security Threat Investigations",
			Description: "Security incident root cause analysis agents. Investigates GuardDuty threats, public exposure, vulnerability exploitability, secrets leaks, data exfiltration, privilege escalation, and runtime incidents.",
			Category:    "Security",
			Data:        securityInvestigationsData,
		},
		{
			ID:          "security-opportunities",
			Name:        "Security Hardening",
			Description: "Security posture improvement agents. Provides CIS hardening recommendations, IAM least privilege advice, patch prioritization, secret hygiene coaching, network security optimization, and runtime policy advice.",
			Category:    "Security",
			Data:        securityOpportunitiesData,
		},
		{
			ID:          "reliability-investigations",
			Name:        "Reliability Incident Analysis",
			Description: "SRE root cause analysis agents. Investigates latency spikes, error bursts, saturation issues, SLO breaches, cascade failures, and capacity exhaustion.",
			Category:    "Reliability",
			Data:        reliabilityInvestigationsData,
		},
		{
			ID:          "deployments-opportunities",
			Name:        "Deployment Optimization",
			Description: "CI/CD pipeline improvement agents. Optimizes build caches, test parallelization, deployment frequency, rollback safety, artifact sizes, and DORA metrics.",
			Category:    "Deployments",
			Data:        deploymentsOpportunitiesData,
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
