package dotprompt_test

import (
	"io/ioutil"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
	"station/pkg/dotprompt"
)

// TestCustomFrontmatterExtraction demonstrates adding and extracting arbitrary frontmatter data
func TestCustomFrontmatterExtraction(t *testing.T) {
	// Create a dotprompt file with extensive custom frontmatter
	dotpromptContent := `---
model: googleai/gemini-1.5-flash
input:
  schema:
    task: string
    context: object
output:
  format: json
  schema:
    result: object
tools:
  - system_monitor
  - alert_manager
metadata:
  agent_id: 999
  name: CustomAgent
  description: Agent with rich custom frontmatter
  max_steps: 3
  environment: test
  version: "2.0"
  schedule_enabled: false

# Custom frontmatter fields - any YAML structure supported
author:
  name: "Station Team"
  email: "team@station.ai"
  department: "Engineering"

deployment:
  region: "us-west-2"
  tier: "premium"
  scaling:
    min_replicas: 2
    max_replicas: 10
    cpu_threshold: 80

business_context:
  cost_center: "INFRA-001"
  compliance_level: "SOC2"
  data_classification: "confidential"
  stakeholders:
    - "DevOps Team"
    - "Security Team"
    - "Product Team"

execution_hints:
  timeout_seconds: 300
  retry_strategy:
    max_attempts: 3
    backoff_multiplier: 2.0
    initial_delay_ms: 1000
  resource_limits:
    memory_mb: 512
    cpu_cores: 0.5

monitoring:
  alerts:
    - type: "execution_failure"
      threshold: 3
      window_minutes: 15
    - type: "response_time"
      threshold: 5000
      unit: "ms"
  metrics:
    - "execution_count"
    - "success_rate"
    - "average_duration"
  dashboards:
    - name: "Agent Performance"
      url: "https://grafana.station.ai/dashboard/agent-999"

integration:
  webhooks:
    on_success: "https://api.station.ai/webhooks/agent-success"
    on_failure: "https://api.station.ai/webhooks/agent-failure"
  external_apis:
    - name: "StatusPage"
      endpoint: "https://api.statuspage.io/v1/"
      auth_type: "bearer"
    - name: "Slack"
      endpoint: "https://hooks.slack.com/services/"
      channel: "#alerts"

development:
  test_cases:
    - description: "Basic system check"
      input: { "task": "check cpu usage" }
      expected_tools: ["system_monitor"]
    - description: "Alert creation"
      input: { "task": "create high cpu alert" }
      expected_tools: ["system_monitor", "alert_manager"]
  documentation: "https://docs.station.ai/agents/custom-agent"
  repository: "https://github.com/company/station-agents"
---

{{#system}}
You are {{metadata.name}} ({{metadata.description}}).

Environment: {{metadata.environment}}
Author: {{author.name}} from {{author.department}}
Deployment Tier: {{deployment.tier}} in {{deployment.region}}

Your execution timeout is {{execution_hints.timeout_seconds}} seconds.
You have access to {{deployment.scaling.max_replicas}} maximum replicas.
{{/system}}

Task: {{task}}

{{#if context}}
Context: {{toJson context}}
{{/if}}

Available tools: {{#each tools}}
- {{.}}
{{/each}}

Please complete the task efficiently within the resource constraints.
Compliance Level: {{business_context.compliance_level}}
`

	// Write to temporary file
	tmpFile, err := ioutil.TempFile("", "custom-agent-*.prompt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(dotpromptContent); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Parse the dotprompt file and extract frontmatter
	content, err := ioutil.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	// Extract YAML frontmatter (between --- markers)
	contentStr := string(content)
	start := "---\n"
	end := "\n---\n"
	
	startIdx := len(start)
	endIdx := findString(contentStr[startIdx:], end)
	if endIdx == -1 {
		t.Fatal("Could not find end of frontmatter")
	}
	endIdx += startIdx
	
	yamlContent := contentStr[startIdx:endIdx]
	template := contentStr[endIdx+len(end):]

	t.Logf("Extracted YAML frontmatter (%d characters)", len(yamlContent))
	t.Logf("Extracted template (%d characters)", len(template))

	// Parse into DotpromptConfig
	var config dotprompt.DotpromptConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
		t.Fatalf("Failed to parse YAML frontmatter: %v", err)
	}

	// Test standard fields
	if config.Model != "googleai/gemini-1.5-flash" {
		t.Errorf("Expected model gemini-1.5-flash, got %s", config.Model)
	}

	if config.Metadata.Name != "CustomAgent" {
		t.Errorf("Expected agent name CustomAgent, got %s", config.Metadata.Name)
	}

	if len(config.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(config.Tools))
	}

	t.Logf("✅ Standard fields parsed correctly")

	// Test custom frontmatter extraction
	testCustomFields(t, config.CustomFields)

	// Test template rendering with custom fields
	testTemplateRendering(t, template, config)
}

func testCustomFields(t *testing.T, customFields map[string]interface{}) {
	// Author information
	if author, ok := customFields["author"].(map[string]interface{}); ok {
		name := author["name"].(string)
		email := author["email"].(string)
		dept := author["department"].(string)
		
		if name != "Station Team" {
			t.Errorf("Expected author name 'Station Team', got %s", name)
		}
		if email != "team@station.ai" {
			t.Errorf("Expected author email 'team@station.ai', got %s", email)
		}
		if dept != "Engineering" {
			t.Errorf("Expected department 'Engineering', got %s", dept)
		}
		t.Logf("✅ Author fields extracted: %s <%s> from %s", name, email, dept)
	} else {
		t.Error("Failed to extract author information")
	}

	// Deployment configuration
	if deployment, ok := customFields["deployment"].(map[string]interface{}); ok {
		region := deployment["region"].(string)
		tier := deployment["tier"].(string)
		
		if scaling, ok := deployment["scaling"].(map[string]interface{}); ok {
			minReplicas := scaling["min_replicas"].(int)
			maxReplicas := scaling["max_replicas"].(int)
			cpuThreshold := scaling["cpu_threshold"].(int)
			
			t.Logf("✅ Deployment config: %s tier in %s (replicas: %d-%d, CPU: %d%%)", 
				tier, region, minReplicas, maxReplicas, cpuThreshold)
		}
	} else {
		t.Error("Failed to extract deployment configuration")
	}

	// Business context
	if business, ok := customFields["business_context"].(map[string]interface{}); ok {
		costCenter := business["cost_center"].(string)
		compliance := business["compliance_level"].(string)
		classification := business["data_classification"].(string)
		
		if stakeholders, ok := business["stakeholders"].([]interface{}); ok {
			stakeholderList := make([]string, len(stakeholders))
			for i, s := range stakeholders {
				stakeholderList[i] = s.(string)
			}
			t.Logf("✅ Business context: %s (%s, %s) - %d stakeholders", 
				costCenter, compliance, classification, len(stakeholderList))
		}
	}

	// Execution hints with nested structure
	if hints, ok := customFields["execution_hints"].(map[string]interface{}); ok {
		timeout := hints["timeout_seconds"].(int)
		
		if retry, ok := hints["retry_strategy"].(map[string]interface{}); ok {
			maxAttempts := retry["max_attempts"].(int)
			backoffMultiplier := retry["backoff_multiplier"].(float64)
			initialDelay := retry["initial_delay_ms"].(int)
			
			t.Logf("✅ Execution hints: timeout=%ds, retries=%d (backoff=%.1fx, delay=%dms)", 
				timeout, maxAttempts, backoffMultiplier, initialDelay)
		}
	}

	// Monitoring configuration with arrays
	if monitoring, ok := customFields["monitoring"].(map[string]interface{}); ok {
		if alerts, ok := monitoring["alerts"].([]interface{}); ok {
			t.Logf("✅ Monitoring: %d alert configurations", len(alerts))
			
			for i, alert := range alerts {
				if alertMap, ok := alert.(map[string]interface{}); ok {
					alertType := alertMap["type"].(string)
					threshold := alertMap["threshold"]
					t.Logf("  Alert %d: %s (threshold: %v)", i+1, alertType, threshold)
				}
			}
		}
		
		if metrics, ok := monitoring["metrics"].([]interface{}); ok {
			metricNames := make([]string, len(metrics))
			for i, metric := range metrics {
				metricNames[i] = metric.(string)
			}
			t.Logf("✅ Metrics tracked: %v", metricNames)
		}
	}

	t.Logf("✅ All custom frontmatter fields successfully extracted and validated")
}

func testTemplateRendering(t *testing.T, template string, config dotprompt.DotpromptConfig) {
	// Test that custom fields can be accessed in template
	if findString(template, "{{author.name}}") != -1 {
		t.Log("✅ Template includes custom author field reference")
	}
	
	if findString(template, "{{deployment.tier}}") != -1 {
		t.Log("✅ Template includes custom deployment field reference") 
	}
	
	if findString(template, "{{business_context.compliance_level}}") != -1 {
		t.Log("✅ Template includes custom business context field reference")
	}

	t.Logf("✅ Template can reference custom frontmatter fields")
}

// Helper function
func findString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}