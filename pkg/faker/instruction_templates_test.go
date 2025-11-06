package faker

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInstructionTemplates tests the predefined instruction templates
func TestInstructionTemplates(t *testing.T) {
	templates := NewPredefinedInstructionTemplates()

	// Test getting a specific template
	instruction, err := templates.GetTemplate("monitoring-high-alert")
	require.NoError(t, err)
	assert.Contains(t, instruction, "alert-heavy monitoring data")
	assert.Contains(t, instruction, "high error rates")

	// Test getting non-existent template
	_, err = templates.GetTemplate("non-existent-template")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test listing all templates
	allTemplates := templates.ListTemplates()
	assert.Greater(t, len(allTemplates), 0)
	assert.Contains(t, allTemplates, "monitoring-high-alert")
	assert.Contains(t, allTemplates, "financial-transactions")
	assert.Contains(t, allTemplates, "healthcare-patient")

	// Test getting categories
	categories := templates.GetCategories()
	assert.Greater(t, len(categories), 0)
	assert.Contains(t, categories, "monitoring")
	assert.Contains(t, categories, "financial")
	assert.Contains(t, categories, "healthcare")

	// Test getting templates by category
	monitoringTemplates := templates.GetTemplateByCategory("monitoring")
	assert.Contains(t, monitoringTemplates, "monitoring-high-alert")
	assert.Contains(t, monitoringTemplates, "monitoring-healthy")
	assert.Contains(t, monitoringTemplates, "monitoring-mixed")

	// Verify category filtering works
	for name := range monitoringTemplates {
		assert.True(t, strings.HasPrefix(name, "monitoring-"))
	}
}

// TestTemplateContentQuality tests the quality and specificity of templates
func TestTemplateContentQuality(t *testing.T) {
	templates := NewPredefinedInstructionTemplates()

	testCases := []struct {
		templateName string
		expectedKeywords []string
		description string
	}{
		{
			templateName: "monitoring-high-alert",
			expectedKeywords: []string{"alert-heavy", "high error rates", "critical warnings", "urgent status"},
			description: "High alert monitoring should contain alert-specific keywords",
		},
		{
			templateName: "financial-transactions",
			expectedKeywords: []string{"monetary amounts", "account numbers", "timestamps", "merchant"},
			description: "Financial transactions should contain money-specific keywords",
		},
		{
			templateName: "healthcare-patient",
			expectedKeywords: []string{"patient IDs", "vital signs", "medical terminology", "clinical"},
			description: "Healthcare patient should contain medical-specific keywords",
		},
		{
			templateName: "ecommerce-orders",
			expectedKeywords: []string{"order IDs", "product names", "quantities", "shipping"},
			description: "E-commerce orders should contain retail-specific keywords",
		},
		{
			templateName: "devops-deployment",
			expectedKeywords: []string{"version numbers", "deployment", "environment", "rollback"},
			description: "DevOps deployment should contain deployment-specific keywords",
		},
		{
			templateName: "iot-environmental",
			expectedKeywords: []string{"temperature", "humidity", "air quality", "sensor"},
			description: "IoT environmental should contain sensor-specific keywords",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.templateName, func(t *testing.T) {
			instruction, err := templates.GetTemplate(tc.templateName)
			require.NoError(t, err, "Failed to get template: %s", tc.templateName)

			// Check that expected keywords are present
			for _, keyword := range tc.expectedKeywords {
				assert.Contains(t, instruction, keyword, 
					"Template '%s' should contain keyword '%s' for %s", 
					tc.templateName, keyword, tc.description)
			}

			// Check that instruction is sufficiently detailed
			assert.Greater(t, len(instruction), 50, 
				"Template '%s' should be sufficiently detailed", tc.templateName)
		})
	}
}

// TestTemplateCategories tests the category organization
func TestTemplateCategories(t *testing.T) {
	templates := NewPredefinedInstructionTemplates()

	categories := templates.GetCategories()
	
	// Expected major categories
	expectedCategories := []string{
		"monitoring", "financial", "healthcare", "ecommerce", 
		"devops", "iot", "hr", "logistics", "education",
	}

	for _, expected := range expectedCategories {
		assert.Contains(t, categories, expected, 
			"Should have category: %s", expected)
	}

	// Test that each category has templates
	for _, category := range categories {
		categoryTemplates := templates.GetTemplateByCategory(category)
		assert.Greater(t, len(categoryTemplates), 0, 
			"Category '%s' should have at least one template", category)

		// Verify all templates in category have correct prefix
		for templateName := range categoryTemplates {
			assert.True(t, strings.HasPrefix(templateName, category+"-"), 
				"Template '%s' should have category prefix '%s-'", 
				templateName, category)
		}
	}
}

// TestTemplateInstructionsForAI tests that templates are AI-friendly
func TestTemplateInstructionsForAI(t *testing.T) {
	templates := NewPredefinedInstructionTemplates()

	allTemplates := templates.ListTemplates()
	
	for _, templateName := range allTemplates {
		t.Run(templateName, func(t *testing.T) {
			instruction, err := templates.GetTemplate(templateName)
			require.NoError(t, err)

			// AI-friendly characteristics
			assertions := []struct {
				description string
				check func(string) bool
			}{
				{
					"Should be specific and actionable", 
					func(s string) bool { return len(s) > 30 },
				},
				{
					"Should mention realistic data", 
					func(s string) bool { return strings.Contains(s, "realistic") || strings.Contains(s, "proper") || strings.Contains(s, "valid") },
				},
				{
					"Should specify data characteristics", 
					func(s string) bool { 
						return strings.Contains(s, "ranges") || strings.Contains(s, "patterns") || 
							   strings.Contains(s, "format") || strings.Contains(s, "structure") 
					},
				},
				{
					"Should avoid ambiguous instructions", 
					func(s string) bool { return !strings.Contains(s, "some data") && !strings.Contains(s, "random stuff") },
				},
			}

			for _, assertion := range assertions {
				assert.True(t, assertion.check(instruction), 
					"Template '%s' failed: %s\nInstruction: %s", 
					templateName, assertion.description, instruction)
			}
		})
	}
}