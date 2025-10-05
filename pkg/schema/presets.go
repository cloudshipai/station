package schema

import "encoding/json"

// PresetInfo contains the app/app_type mapping and default schema for a preset
type PresetInfo struct {
	App        string
	AppType    string
	Schema     map[string]interface{}
}

// GetPresetInfo returns the app, app_type, and default schema for a given preset name
func GetPresetInfo(presetName string) (PresetInfo, bool) {
	presets := map[string]PresetInfo{
		// FinOps Presets (5 app_types)
		"finops-inventory": {
			App:     "finops",
			AppType: "inventory",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"resources": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"resource_id":   map[string]interface{}{"type": "string"},
								"resource_type": map[string]interface{}{"type": "string"},
								"cost":          map[string]interface{}{"type": "number"},
								"region":        map[string]interface{}{"type": "string"},
								"tags":          map[string]interface{}{"type": "object"},
							},
							"required": []string{"resource_id", "resource_type", "cost"},
						},
					},
					"total_cost": map[string]interface{}{"type": "number"},
					"currency":   map[string]interface{}{"type": "string"},
					"timestamp":  map[string]interface{}{"type": "string", "format": "date-time"},
				},
				"required": []string{"resources", "total_cost"},
			},
		},
		"finops-investigations": {
			App:     "finops",
			AppType: "investigations",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"finding": map[string]interface{}{"type": "string"},
					"evidence": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"cost_impact":     map[string]interface{}{"type": "number"},
							"affected_resources": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{"type": "string"},
							},
							"time_period": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"start": map[string]interface{}{"type": "string", "format": "date-time"},
									"end":   map[string]interface{}{"type": "string", "format": "date-time"},
								},
							},
						},
					},
					"root_cause":  map[string]interface{}{"type": "string"},
					"confidence":  map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
					"recommended_actions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"finding", "evidence", "confidence"},
			},
		},
		"finops-opportunities": {
			App:     "finops",
			AppType: "opportunities",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"opportunity_type": map[string]interface{}{"type": "string"},
					"description":      map[string]interface{}{"type": "string"},
					"potential_savings": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"amount":   map[string]interface{}{"type": "number"},
							"currency": map[string]interface{}{"type": "string"},
							"period":   map[string]interface{}{"type": "string"},
						},
						"required": []string{"amount", "currency"},
					},
					"affected_resources": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"implementation_effort": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"priority": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high", "critical"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"opportunity_type", "description", "potential_savings", "confidence"},
			},
		},
		"finops-projections": {
			App:     "finops",
			AppType: "projections",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projection_type": map[string]interface{}{"type": "string"},
					"time_horizon": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"start": map[string]interface{}{"type": "string", "format": "date-time"},
							"end":   map[string]interface{}{"type": "string", "format": "date-time"},
						},
						"required": []string{"start", "end"},
					},
					"projected_cost": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"amount":   map[string]interface{}{"type": "number"},
							"currency": map[string]interface{}{"type": "string"},
						},
						"required": []string{"amount", "currency"},
					},
					"confidence_interval": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"lower": map[string]interface{}{"type": "number"},
							"upper": map[string]interface{}{"type": "number"},
						},
					},
					"assumptions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"projection_type", "time_horizon", "projected_cost", "confidence"},
			},
		},
		"finops-events": {
			App:     "finops",
			AppType: "events",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_type": map[string]interface{}{"type": "string"},
					"timestamp":  map[string]interface{}{"type": "string", "format": "date-time"},
					"cost_impact": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"amount":   map[string]interface{}{"type": "number"},
							"currency": map[string]interface{}{"type": "string"},
						},
					},
					"affected_resources": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"change_description": map[string]interface{}{"type": "string"},
					"triggered_by":       map[string]interface{}{"type": "string"},
				},
				"required": []string{"event_type", "timestamp", "change_description"},
			},
		},
		"security-inventory": {
			App:     "security",
			AppType: "inventory",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"vulnerabilities": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"cve_id":      map[string]interface{}{"type": "string"},
								"severity":    map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low", "info"}},
								"affected_resource": map[string]interface{}{"type": "string"},
								"status":      map[string]interface{}{"type": "string"},
								"discovered_at": map[string]interface{}{"type": "string", "format": "date-time"},
							},
							"required": []string{"cve_id", "severity", "affected_resource"},
						},
					},
					"total_count": map[string]interface{}{"type": "integer"},
					"severity_breakdown": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"critical": map[string]interface{}{"type": "integer"},
							"high":     map[string]interface{}{"type": "integer"},
							"medium":   map[string]interface{}{"type": "integer"},
							"low":      map[string]interface{}{"type": "integer"},
						},
					},
					"timestamp": map[string]interface{}{"type": "string", "format": "date-time"},
				},
				"required": []string{"vulnerabilities", "total_count"},
			},
		},
		"security-investigations": {
			App:     "security",
			AppType: "investigations",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"incident_type": map[string]interface{}{"type": "string"},
					"finding":       map[string]interface{}{"type": "string"},
					"evidence": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"indicators_of_compromise": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{"type": "string"},
							},
							"affected_systems": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{"type": "string"},
							},
							"attack_vector": map[string]interface{}{"type": "string"},
							"timeline": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"timestamp": map[string]interface{}{"type": "string", "format": "date-time"},
										"event":     map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"root_cause": map[string]interface{}{"type": "string"},
					"impact":     map[string]interface{}{"type": "string"},
					"remediation_steps": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"incident_type", "finding", "evidence", "confidence"},
			},
		},
		"security-opportunities": {
			App:     "security",
			AppType: "opportunities",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"opportunity_type": map[string]interface{}{"type": "string"},
					"description":      map[string]interface{}{"type": "string"},
					"risk_reduction": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"severity_before": map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low"}},
							"severity_after":  map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low"}},
						},
					},
					"affected_systems": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"implementation_effort": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"priority": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high", "critical"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"opportunity_type", "description", "confidence"},
			},
		},
		"security-projections": {
			App:     "security",
			AppType: "projections",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projection_type": map[string]interface{}{"type": "string"},
					"time_horizon": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"start": map[string]interface{}{"type": "string", "format": "date-time"},
							"end":   map[string]interface{}{"type": "string", "format": "date-time"},
						},
						"required": []string{"start", "end"},
					},
					"risk_score_projection": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"current":   map[string]interface{}{"type": "number"},
							"projected": map[string]interface{}{"type": "number"},
						},
					},
					"trend":       map[string]interface{}{"type": "string", "enum": []string{"increasing", "decreasing", "stable"}},
					"assumptions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"projection_type", "time_horizon", "confidence"},
			},
		},
		"security-events": {
			App:     "security",
			AppType: "events",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_type": map[string]interface{}{"type": "string"},
					"timestamp":  map[string]interface{}{"type": "string", "format": "date-time"},
					"severity":   map[string]interface{}{"type": "string", "enum": []string{"critical", "high", "medium", "low", "info"}},
					"affected_systems": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"description": map[string]interface{}{"type": "string"},
					"source":      map[string]interface{}{"type": "string"},
					"indicators": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
				},
				"required": []string{"event_type", "timestamp", "severity", "description"},
			},
		},
		"deployments-inventory": {
			App:     "deployments",
			AppType: "inventory",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"deployments": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"service_name": map[string]interface{}{"type": "string"},
								"version":      map[string]interface{}{"type": "string"},
								"environment":  map[string]interface{}{"type": "string"},
								"status":       map[string]interface{}{"type": "string", "enum": []string{"running", "stopped", "degraded"}},
								"deployed_at":  map[string]interface{}{"type": "string", "format": "date-time"},
							},
							"required": []string{"service_name", "version", "environment", "status"},
						},
					},
					"total_services": map[string]interface{}{"type": "integer"},
					"timestamp":      map[string]interface{}{"type": "string", "format": "date-time"},
				},
				"required": []string{"deployments", "total_services"},
			},
		},
		"deployments-events": {
			App:     "deployments",
			AppType: "events",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"event_type": map[string]interface{}{"type": "string"},
					"deployment_id": map[string]interface{}{"type": "string"},
					"service_name":  map[string]interface{}{"type": "string"},
					"version": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"previous": map[string]interface{}{"type": "string"},
							"current":  map[string]interface{}{"type": "string"},
						},
					},
					"timestamp": map[string]interface{}{"type": "string", "format": "date-time"},
					"status":    map[string]interface{}{"type": "string", "enum": []string{"pending", "in_progress", "completed", "failed", "rolled_back"}},
					"changes": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"type":        map[string]interface{}{"type": "string"},
								"description": map[string]interface{}{"type": "string"},
								"impact":      map[string]interface{}{"type": "string"},
							},
						},
					},
					"metadata": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"pr_number":  map[string]interface{}{"type": "string"},
							"commit_sha": map[string]interface{}{"type": "string"},
							"author":     map[string]interface{}{"type": "string"},
						},
					},
				},
				"required": []string{"event_type", "service_name", "timestamp", "status"},
			},
		},
		"deployments-investigations": {
			App:     "deployments",
			AppType: "investigations",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"failure_type": map[string]interface{}{"type": "string"},
					"finding":      map[string]interface{}{"type": "string"},
					"evidence": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"deployment_id": map[string]interface{}{"type": "string"},
							"failed_step":   map[string]interface{}{"type": "string"},
							"error_logs": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{"type": "string"},
							},
							"affected_services": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{"type": "string"},
							},
						},
					},
					"root_cause": map[string]interface{}{"type": "string"},
					"remediation_steps": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"failure_type", "finding", "evidence", "confidence"},
			},
		},
		"deployments-opportunities": {
			App:     "deployments",
			AppType: "opportunities",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"opportunity_type": map[string]interface{}{"type": "string"},
					"description":      map[string]interface{}{"type": "string"},
					"improvement_impact": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"metric":      map[string]interface{}{"type": "string"},
							"current":     map[string]interface{}{"type": "number"},
							"potential":   map[string]interface{}{"type": "number"},
							"improvement": map[string]interface{}{"type": "string"},
						},
					},
					"affected_services": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"implementation_effort": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high"},
					},
					"priority": map[string]interface{}{
						"type": "string",
						"enum": []string{"low", "medium", "high", "critical"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"opportunity_type", "description", "confidence"},
			},
		},
		"deployments-projections": {
			App:     "deployments",
			AppType: "projections",
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"projection_type": map[string]interface{}{"type": "string"},
					"time_horizon": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"start": map[string]interface{}{"type": "string", "format": "date-time"},
							"end":   map[string]interface{}{"type": "string", "format": "date-time"},
						},
						"required": []string{"start", "end"},
					},
					"metric_projection": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"metric":    map[string]interface{}{"type": "string"},
							"current":   map[string]interface{}{"type": "number"},
							"projected": map[string]interface{}{"type": "number"},
						},
					},
					"trend":       map[string]interface{}{"type": "string", "enum": []string{"increasing", "decreasing", "stable"}},
					"assumptions": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{"type": "string"},
					},
					"confidence": map[string]interface{}{"type": "number", "minimum": 0, "maximum": 1},
				},
				"required": []string{"projection_type", "time_horizon", "confidence"},
			},
		},
	}

	info, exists := presets[presetName]
	return info, exists
}

// GetPresetSchema returns just the JSON schema for a preset (for backward compatibility)
func GetPresetSchema(presetName string) (string, bool) {
	info, exists := GetPresetInfo(presetName)
	if !exists {
		return "", false
	}

	schemaJSON, err := json.MarshalIndent(info.Schema, "", "  ")
	if err != nil {
		return "", false
	}

	return string(schemaJSON), true
}

// SchemaToJSON converts a schema map to JSON string
func SchemaToJSON(schema map[string]interface{}) string {
	schemaJSON, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return ""
	}
	return string(schemaJSON)
}

// ListPresets returns all available preset names
func ListPresets() []string {
	return []string{
		// FinOps (5 presets)
		"finops-inventory",
		"finops-investigations",
		"finops-opportunities",
		"finops-projections",
		"finops-events",
		// Security (5 presets)
		"security-inventory",
		"security-investigations",
		"security-opportunities",
		"security-projections",
		"security-events",
		// Deployments (5 presets)
		"deployments-inventory",
		"deployments-investigations",
		"deployments-opportunities",
		"deployments-projections",
		"deployments-events",
	}
}

// PresetMetadata contains display information for UI
type PresetMetadata struct {
	Name        string
	App         string
	AppType     string
	Description string
	UseCases    []string
}

// GetPresetMetadata returns user-friendly information about presets for UI display
func GetPresetMetadata() []PresetMetadata {
	return []PresetMetadata{
		// FinOps (5 presets)
		{
			Name:        "finops-inventory",
			App:         "finops",
			AppType:     "inventory",
			Description: "Track current infrastructure resources and costs",
			UseCases:    []string{"Resource inventory", "Cost tracking", "Infrastructure snapshots"},
		},
		{
			Name:        "finops-investigations",
			App:         "finops",
			AppType:     "investigations",
			Description: "Investigate cost spikes and anomalies",
			UseCases:    []string{"Cost spike analysis", "Budget overrun investigation", "Unexpected charges"},
		},
		{
			Name:        "finops-opportunities",
			App:         "finops",
			AppType:     "opportunities",
			Description: "Identify cost optimization opportunities",
			UseCases:    []string{"Cost savings", "Resource rightsizing", "Waste elimination"},
		},
		{
			Name:        "finops-projections",
			App:         "finops",
			AppType:     "projections",
			Description: "Forecast future costs and resource needs",
			UseCases:    []string{"Cost forecasting", "Capacity planning", "Burn rate projection"},
		},
		{
			Name:        "finops-events",
			App:         "finops",
			AppType:     "events",
			Description: "Track cost-impacting changes and events",
			UseCases:    []string{"Resource changes", "Cost anomaly events", "Budget threshold alerts"},
		},
		// Security (5 presets)
		{
			Name:        "security-inventory",
			App:         "security",
			AppType:     "inventory",
			Description: "Catalog current security vulnerabilities and exposure",
			UseCases:    []string{"CVE tracking", "Vulnerability inventory", "Security posture snapshots"},
		},
		{
			Name:        "security-investigations",
			App:         "security",
			AppType:     "investigations",
			Description: "Investigate security incidents and breaches",
			UseCases:    []string{"Incident response", "Breach analysis", "Attack vector investigation"},
		},
		{
			Name:        "security-opportunities",
			App:         "security",
			AppType:     "opportunities",
			Description: "Identify security improvement opportunities",
			UseCases:    []string{"Vulnerability prioritization", "Security hardening", "Risk reduction"},
		},
		{
			Name:        "security-projections",
			App:         "security",
			AppType:     "projections",
			Description: "Project security risk trends and exposure",
			UseCases:    []string{"Risk score forecasting", "Threat trend analysis", "Exposure prediction"},
		},
		{
			Name:        "security-events",
			App:         "security",
			AppType:     "events",
			Description: "Track security incidents and changes",
			UseCases:    []string{"Security incidents", "Configuration changes", "Access events"},
		},
		// Deployments (5 presets)
		{
			Name:        "deployments-inventory",
			App:         "deployments",
			AppType:     "inventory",
			Description: "Catalog currently deployed services and versions",
			UseCases:    []string{"Service catalog", "Version tracking", "Deployment status"},
		},
		{
			Name:        "deployments-investigations",
			App:         "deployments",
			AppType:     "investigations",
			Description: "Investigate deployment failures and issues",
			UseCases:    []string{"Deployment failures", "Rollback analysis", "CI/CD troubleshooting"},
		},
		{
			Name:        "deployments-opportunities",
			App:         "deployments",
			AppType:     "opportunities",
			Description: "Identify deployment process improvements",
			UseCases:    []string{"CI/CD optimization", "Release velocity improvement", "Pipeline efficiency"},
		},
		{
			Name:        "deployments-projections",
			App:         "deployments",
			AppType:     "projections",
			Description: "Project deployment metrics and trends",
			UseCases:    []string{"Release velocity forecasting", "MTTR prediction", "Deployment success rates"},
		},
		{
			Name:        "deployments-events",
			App:         "deployments",
			AppType:     "events",
			Description: "Track deployment events and changes",
			UseCases:    []string{"Deployment tracking", "Change history", "Release timeline"},
		},
	}
}
