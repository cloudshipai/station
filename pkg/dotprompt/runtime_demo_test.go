package dotprompt_test

import (
	"testing"

	"station/pkg/dotprompt"
)

// TestRuntimeExtractionDemo demonstrates live extraction of custom frontmatter at runtime
func TestRuntimeExtractionDemo(t *testing.T) {
	// Load the demo agent with rich custom frontmatter
	extractor, err := dotprompt.NewRuntimeExtraction("../../demo-agent-with-custom-frontmatter.prompt")
	if err != nil {
		t.Fatalf("Failed to load demo agent: %v", err)
	}

	t.Logf("✅ Successfully loaded dotprompt file with custom frontmatter")

	// Test extraction of nested team information
	teamInfo, err := extractor.GetTeamInfo()
	if err != nil {
		t.Fatalf("Failed to extract team info: %v", err)
	}

	expectedLead := "Alice Johnson"
	if lead := teamInfo["lead"].(string); lead != expectedLead {
		t.Errorf("Expected team lead '%s', got '%s'", expectedLead, lead)
	}

	expectedContact := "team-alpha@company.com"
	if contact := teamInfo["contact"].(string); contact != expectedContact {
		t.Errorf("Expected contact '%s', got '%s'", expectedContact, contact)
	}

	if members, ok := teamInfo["members"].([]interface{}); ok {
		if len(members) != 3 {
			t.Errorf("Expected 3 team members, got %d", len(members))
		}
		t.Logf("✅ Team info extracted: %s leads %d members, contact: %s", 
			teamInfo["lead"], len(members), teamInfo["contact"])
	}

	// Test extraction of deployment configuration
	deployConfig, err := extractor.GetDeploymentConfig()
	if err != nil {
		t.Fatalf("Failed to extract deployment config: %v", err)
	}

	expectedProvider := "AWS"
	if provider := deployConfig["cloud_provider"].(string); provider != expectedProvider {
		t.Errorf("Expected cloud provider '%s', got '%s'", expectedProvider, provider)
	}

	expectedRegion := "us-west-2"
	if region := deployConfig["region"].(string); region != expectedRegion {
		t.Errorf("Expected region '%s', got '%s'", expectedRegion, region)
	}

	// Test nested scaling configuration
	if scaling, ok := deployConfig["scaling"].(map[string]interface{}); ok {
		minInstances := scaling["min_instances"].(int)
		maxInstances := scaling["max_instances"].(int)
		targetCPU := scaling["target_cpu"].(int)
		
		if minInstances != 2 || maxInstances != 8 || targetCPU != 70 {
			t.Errorf("Expected scaling 2-8 instances with 70%% CPU, got %d-%d with %d%% CPU", 
				minInstances, maxInstances, targetCPU)
		}
		
		t.Logf("✅ Deployment config extracted: %s %s in %s, scaling %d-%d instances", 
			deployConfig["cloud_provider"], deployConfig["instance_type"], 
			deployConfig["region"], minInstances, maxInstances)
	}

	// Test business rules extraction
	businessRules, err := extractor.GetBusinessRules()
	if err != nil {
		t.Fatalf("Failed to extract business rules: %v", err)
	}

	expectedSLA := "99.9%"
	if sla := businessRules["sla_target"].(string); sla != expectedSLA {
		t.Errorf("Expected SLA target '%s', got '%s'", expectedSLA, sla)
	}

	expectedMaxResponseTime := 5000
	if maxResponseTime := businessRules["max_response_time"].(int); maxResponseTime != expectedMaxResponseTime {
		t.Errorf("Expected max response time %dms, got %dms", expectedMaxResponseTime, maxResponseTime)
	}

	// Test nested compliance configuration
	if compliance, ok := businessRules["compliance"].(map[string]interface{}); ok {
		framework := compliance["framework"].(string)
		retentionDays := compliance["data_retention_days"].(int)
		auditTrail := compliance["audit_trail"].(bool)
		
		if framework != "SOC2" || retentionDays != 365 || !auditTrail {
			t.Errorf("Expected SOC2 compliance with 365 day retention and audit trail, got %s with %d days, audit: %v", 
				framework, retentionDays, auditTrail)
		}
		
		t.Logf("✅ Business rules extracted: SLA %s, max response %dms, compliance %s", 
			businessRules["sla_target"], businessRules["max_response_time"], framework)
	}

	// Test feature flags extraction using convenience method
	advancedAnalytics, err := extractor.IsFeatureEnabled("enable_advanced_analytics")
	if err != nil {
		t.Fatalf("Failed to check advanced analytics feature: %v", err)
	}
	if !advancedAnalytics {
		t.Error("Expected advanced analytics to be enabled")
	}

	mlPredictions, err := extractor.IsFeatureEnabled("use_ml_predictions")
	if err != nil {
		t.Fatalf("Failed to check ML predictions feature: %v", err)
	}
	if mlPredictions {
		t.Error("Expected ML predictions to be disabled")
	}

	detailedNotifications, err := extractor.IsFeatureEnabled("send_detailed_notifications")
	if err != nil {
		t.Fatalf("Failed to check detailed notifications feature: %v", err)
	}
	if !detailedNotifications {
		t.Error("Expected detailed notifications to be enabled")
	}

	t.Logf("✅ Feature flags extracted: analytics=%v, ml=%v, notifications=%v", 
		advancedAnalytics, mlPredictions, detailedNotifications)

	// Test execution profile with nested retry policy
	execProfile, err := extractor.GetExecutionProfile()
	if err != nil {
		t.Fatalf("Failed to extract execution profile: %v", err)
	}

	expectedTimeout := 120
	if timeout := execProfile["timeout_seconds"].(int); timeout != expectedTimeout {
		t.Errorf("Expected timeout %ds, got %ds", expectedTimeout, timeout)
	}

	if retryPolicy, ok := execProfile["retry_policy"].(map[string]interface{}); ok {
		maxAttempts := retryPolicy["max_attempts"].(int)
		backoffStrategy := retryPolicy["backoff_strategy"].(string)
		baseDelay := retryPolicy["base_delay_ms"].(int)
		
		if maxAttempts != 3 || backoffStrategy != "exponential" || baseDelay != 500 {
			t.Errorf("Expected 3 attempts with exponential backoff and 500ms base delay, got %d attempts with %s backoff and %dms delay", 
				maxAttempts, backoffStrategy, baseDelay)
		}
		
		t.Logf("✅ Execution profile extracted: timeout=%ds, retries=%d (%s backoff)", 
			expectedTimeout, maxAttempts, backoffStrategy)
	}

	// Test arbitrary field path extraction
	cloudProvider, err := extractor.ExtractCustomField("deployment_config.cloud_provider")
	if err != nil {
		t.Fatalf("Failed to extract nested field: %v", err)
	}
	if cloudProvider.(string) != "AWS" {
		t.Errorf("Expected AWS via field path extraction, got %s", cloudProvider)
	}

	budgetCode, err := extractor.ExtractCustomField("deployment_config.cost_allocation.budget_code")
	if err != nil {
		t.Fatalf("Failed to extract deeply nested field: %v", err)
	}
	if budgetCode.(string) != "ENG-2024-Q4" {
		t.Errorf("Expected ENG-2024-Q4 via field path extraction, got %s", budgetCode)
	}

	t.Logf("✅ Arbitrary field path extraction works: cloud=%s, budget=%s", cloudProvider, budgetCode)

	// Test template access
	template := extractor.GetTemplate()
	if len(template) == 0 {
		t.Error("Template should not be empty")
	}

	// Verify template contains custom field references
	if !containsString(template, "{{team_info.lead}}") {
		t.Error("Template should contain team info reference")
	}
	if !containsString(template, "{{business_rules.sla_target}}") {
		t.Error("Template should contain business rules reference")
	}
	if !containsString(template, "{{deployment_config.cloud_provider}}") {
		t.Error("Template should contain deployment config reference")
	}

	t.Logf("✅ Template extracted (%d chars) with custom field references", len(template))

	// Print comprehensive runtime information
	t.Log("=== Full Runtime Extraction Demo ===")
	extractor.PrintRuntimeInfo()

	t.Log("✅ All custom frontmatter successfully extracted and validated at runtime!")
}

// Helper function
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}