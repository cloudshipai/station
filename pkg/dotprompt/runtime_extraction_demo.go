package dotprompt

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v3"
)

// RuntimeExtraction demonstrates extracting and using custom frontmatter at runtime
type RuntimeExtraction struct {
	config   DotpromptConfig
	template string
	filePath string
}

// NewRuntimeExtraction creates a new runtime extraction demo
func NewRuntimeExtraction(dotpromptFilePath string) (*RuntimeExtraction, error) {
	content, err := ioutil.ReadFile(dotpromptFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dotprompt file: %w", err)
	}

	// Parse frontmatter and template
	config, template, err := parseDotpromptFile(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse dotprompt: %w", err)
	}

	return &RuntimeExtraction{
		config:   config,
		template: template,
		filePath: dotpromptFilePath,
	}, nil
}

// ExtractCustomField extracts any custom field from frontmatter by path
func (re *RuntimeExtraction) ExtractCustomField(fieldPath string) (interface{}, error) {
	parts := strings.Split(fieldPath, ".")
	current := interface{}(re.config.CustomFields)

	for _, part := range parts {
		if currentMap, ok := current.(map[string]interface{}); ok {
			if value, exists := currentMap[part]; exists {
				current = value
			} else {
				return nil, fmt.Errorf("field path '%s' not found", fieldPath)
			}
		} else {
			return nil, fmt.Errorf("cannot traverse field path '%s' - not a map", fieldPath)
		}
	}

	return current, nil
}

// GetTeamInfo extracts team information from custom frontmatter
func (re *RuntimeExtraction) GetTeamInfo() (map[string]interface{}, error) {
	teamInfo, err := re.ExtractCustomField("team_info")
	if err != nil {
		return nil, err
	}
	
	if teamMap, ok := teamInfo.(map[string]interface{}); ok {
		return teamMap, nil
	}
	
	return nil, fmt.Errorf("team_info is not a map")
}

// GetDeploymentConfig extracts deployment configuration
func (re *RuntimeExtraction) GetDeploymentConfig() (map[string]interface{}, error) {
	deploymentConfig, err := re.ExtractCustomField("deployment_config")
	if err != nil {
		return nil, err
	}
	
	if deploymentMap, ok := deploymentConfig.(map[string]interface{}); ok {
		return deploymentMap, nil
	}
	
	return nil, fmt.Errorf("deployment_config is not a map")
}

// GetBusinessRules extracts business rules and SLA information
func (re *RuntimeExtraction) GetBusinessRules() (map[string]interface{}, error) {
	businessRules, err := re.ExtractCustomField("business_rules")
	if err != nil {
		return nil, err
	}
	
	if businessMap, ok := businessRules.(map[string]interface{}); ok {
		return businessMap, nil
	}
	
	return nil, fmt.Errorf("business_rules is not a map")
}

// GetExecutionProfile extracts execution configuration
func (re *RuntimeExtraction) GetExecutionProfile() (map[string]interface{}, error) {
	executionProfile, err := re.ExtractCustomField("execution_profile")
	if err != nil {
		return nil, err
	}
	
	if executionMap, ok := executionProfile.(map[string]interface{}); ok {
		return executionMap, nil
	}
	
	return nil, fmt.Errorf("execution_profile is not a map")
}

// GetFeatureFlags extracts feature flags
func (re *RuntimeExtraction) GetFeatureFlags() (map[string]interface{}, error) {
	featureFlags, err := re.ExtractCustomField("feature_flags")
	if err != nil {
		return nil, err
	}
	
	if flagsMap, ok := featureFlags.(map[string]interface{}); ok {
		return flagsMap, nil
	}
	
	return nil, fmt.Errorf("feature_flags is not a map")
}

// IsFeatureEnabled checks if a specific feature flag is enabled
func (re *RuntimeExtraction) IsFeatureEnabled(flagName string) (bool, error) {
	flags, err := re.GetFeatureFlags()
	if err != nil {
		return false, err
	}
	
	if value, exists := flags[flagName]; exists {
		if boolValue, ok := value.(bool); ok {
			return boolValue, nil
		}
		return false, fmt.Errorf("feature flag '%s' is not a boolean", flagName)
	}
	
	return false, fmt.Errorf("feature flag '%s' not found", flagName)
}

// GetTemplate returns the template content
func (re *RuntimeExtraction) GetTemplate() string {
	return re.template
}

// GetConfig returns the full dotprompt configuration
func (re *RuntimeExtraction) GetConfig() DotpromptConfig {
	return re.config
}

// GetFilePath returns the file path to the dotprompt file
func (re *RuntimeExtraction) GetFilePath() string {
	return re.filePath
}

// PrintRuntimeInfo prints comprehensive runtime information extracted from frontmatter
func (re *RuntimeExtraction) PrintRuntimeInfo() {
	fmt.Printf("=== Runtime Frontmatter Extraction Demo ===\n\n")
	
	// Standard fields
	fmt.Printf("📋 Standard Agent Info:\n")
	fmt.Printf("  Name: %s\n", re.config.Metadata.Name)
	fmt.Printf("  Model: %s\n", re.config.Model)
	fmt.Printf("  Tools: %v\n", re.config.Tools)
	fmt.Printf("  Max Steps: %d\n\n", re.config.Metadata.MaxSteps)
	
	// Team information
	if teamInfo, err := re.GetTeamInfo(); err == nil {
		fmt.Printf("👥 Team Information:\n")
		fmt.Printf("  Lead: %s\n", teamInfo["lead"])
		fmt.Printf("  Contact: %s\n", teamInfo["contact"])
		fmt.Printf("  Slack: %s\n", teamInfo["slack_channel"])
		if members, ok := teamInfo["members"].([]interface{}); ok {
			fmt.Printf("  Members: %d people\n", len(members))
		}
		fmt.Printf("\n")
	}
	
	// Deployment config
	if deployConfig, err := re.GetDeploymentConfig(); err == nil {
		fmt.Printf("☁️ Deployment Configuration:\n")
		fmt.Printf("  Provider: %s\n", deployConfig["cloud_provider"])
		fmt.Printf("  Region: %s\n", deployConfig["region"])
		fmt.Printf("  Instance: %s\n", deployConfig["instance_type"])
		
		if scaling, ok := deployConfig["scaling"].(map[string]interface{}); ok {
			fmt.Printf("  Scaling: %v-%v instances (target CPU: %v%%)\n", 
				scaling["min_instances"], scaling["max_instances"], scaling["target_cpu"])
		}
		
		if costAlloc, ok := deployConfig["cost_allocation"].(map[string]interface{}); ok {
			fmt.Printf("  Cost: %s - %s (%s)\n", 
				costAlloc["department"], costAlloc["project"], costAlloc["budget_code"])
		}
		fmt.Printf("\n")
	}
	
	// Business rules
	if businessRules, err := re.GetBusinessRules(); err == nil {
		fmt.Printf("📊 Business Rules:\n")
		fmt.Printf("  SLA Target: %s\n", businessRules["sla_target"])
		fmt.Printf("  Max Response Time: %vms\n", businessRules["max_response_time"])
		
		if compliance, ok := businessRules["compliance"].(map[string]interface{}); ok {
			fmt.Printf("  Compliance: %s (%v day retention)\n", 
				compliance["framework"], compliance["data_retention_days"])
		}
		fmt.Printf("\n")
	}
	
	// Execution profile
	if execProfile, err := re.GetExecutionProfile(); err == nil {
		fmt.Printf("⚙️ Execution Profile:\n")
		fmt.Printf("  Timeout: %vs\n", execProfile["timeout_seconds"])
		
		if resourceLimits, ok := execProfile["resource_limits"].(map[string]interface{}); ok {
			fmt.Printf("  Resources: %vMB RAM, %vm CPU\n", 
				resourceLimits["memory_mb"], resourceLimits["cpu_millicores"])
		}
		
		if retryPolicy, ok := execProfile["retry_policy"].(map[string]interface{}); ok {
			fmt.Printf("  Retries: %v attempts (%s backoff)\n", 
				retryPolicy["max_attempts"], retryPolicy["backoff_strategy"])
		}
		fmt.Printf("\n")
	}
	
	// Feature flags
	if flags, err := re.GetFeatureFlags(); err == nil {
		fmt.Printf("🚩 Feature Flags:\n")
		for flagName, flagValue := range flags {
			status := "❌"
			if enabled, ok := flagValue.(bool); ok && enabled {
				status = "✅"
			}
			fmt.Printf("  %s %s\n", status, flagName)
		}
		fmt.Printf("\n")
	}
	
	// Template info
	fmt.Printf("📄 Template:\n")
	fmt.Printf("  Length: %d characters\n", len(re.template))
	fmt.Printf("  Contains custom field references: %v\n", strings.Contains(re.template, "{{team_info."))
	fmt.Printf("  Contains business rule references: %v\n", strings.Contains(re.template, "{{business_rules."))
	fmt.Printf("\n")
}

// parseDotpromptFile parses a dotprompt file and returns config and template
func parseDotpromptFile(content string) (DotpromptConfig, string, error) {
	// Find frontmatter boundaries
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || lines[0] != "---" {
		return DotpromptConfig{}, "", fmt.Errorf("invalid dotprompt format: missing opening frontmatter")
	}
	
	// Find closing frontmatter
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			endIdx = i
			break
		}
	}
	
	if endIdx == -1 {
		return DotpromptConfig{}, "", fmt.Errorf("invalid dotprompt format: missing closing frontmatter")
	}
	
	// Extract YAML and template
	yamlContent := strings.Join(lines[1:endIdx], "\n")
	template := strings.Join(lines[endIdx+1:], "\n")
	
	// Parse YAML
	var config DotpromptConfig
	if err := yaml.Unmarshal([]byte(yamlContent), &config); err != nil {
		return DotpromptConfig{}, "", fmt.Errorf("failed to parse frontmatter YAML: %w", err)
	}
	
	return config, strings.TrimSpace(template), nil
}