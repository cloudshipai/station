package faker

import (
	"encoding/json"
	"fmt"
)

// DemonstrateAIvsBasic shows the difference between AI and basic enrichment
func DemonstrateAIvsBasic() {
	fmt.Println("🤖 Station AI-Powered Faker Demo")
	fmt.Println("==================================")
	
	// Sample GitHub repository schema
	sampleData := map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           123456789,
			"name":         "awesome-project",
			"fullName":     "octocat/awesome-project",
			"description":  "An awesome project for testing",
			"private":      false,
			"language":     "Go",
			"stars":        42,
			"forks":        15,
			"openIssues":   7,
			"createdAt":    "2024-01-15T10:30:00Z",
			"updatedAt":    "2024-11-01T15:45:00Z",
			"owner": map[string]interface{}{
				"id":         583231,
				"login":      "octocat",
				"name":       "Octo Cat",
				"email":      "octocat@github.com",
				"company":    "GitHub",
				"location":   "San Francisco",
				"blog":       "https://github.blog",
				"followers":   150,
				"following":   75,
			},
		},
	}

	fmt.Println("\n📋 Original Schema (GitHub Repository):")
	originalJSON, _ := json.MarshalIndent(sampleData, "", "  ")
	fmt.Println(string(originalJSON))

	// Demonstrate basic faker (current implementation)
	fmt.Println("\n🎲 Basic Faker (Current - Generates 'Slop'):")
	basicResult := generateBasicFakerResult()
	basicJSON, _ := json.MarshalIndent(basicResult, "", "  ")
	fmt.Println(string(basicJSON))

	// Demonstrate AI faker (proposed implementation)
	fmt.Println("\n🤖 AI Faker (Proposed - Contextually Appropriate):")
	scenarios := []struct {
		name        string
		instruction string
		result      map[string]interface{}
	}{
		{
			name:        "Standard Repository",
			instruction: "Generate realistic GitHub repository data with proper repository names, accurate programming languages, plausible star counts.",
			result:      generateAIStandardResult(),
		},
		{
			name:        "High-Activity Repository", 
			instruction: "Generate data for a highly active GitHub repository with many stars, forks, and issues. Use popular programming languages.",
			result:      generateAIHighActivityResult(),
		},
		{
			name:        "Enterprise Repository",
			instruction: "Generate enterprise GitHub repository data with corporate naming conventions, business-focused programming languages.",
			result:      generateAIEnterpriseResult(),
		},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n--- %s ---\n", scenario.name)
		fmt.Printf("Instruction: %s\n", scenario.instruction)
		aiJSON, _ := json.MarshalIndent(scenario.result, "", "  ")
		fmt.Println(string(aiJSON))
	}

	// Show user experience
	fmt.Println("\n👤 User Experience Comparison:")
	fmt.Println("================================")
	demonstrateUserExperience()
}

// generateBasicFakerResult shows current basic faker output
func generateBasicFakerResult() map[string]interface{} {
	return map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           "6b1db778-1124-463d-a155-7420f4270e8f",
			"name":         "Jeremie Ortiz",
			"fullName":     "Shyanne Runolfsdottir",
			"description":  "211.41.218.11",
			"private":      false,
			"language":     "laborum",
			"stars":        28,
			"forks":        56,
			"openIssues":   59,
			"createdAt":    "1920-04-25T06:21:48Z",
			"updatedAt":    "1950-08-18T16:02:03Z",
			"owner": map[string]interface{}{
				"id":         "563dd20c-6e9e-425f-8216-d31c68206106",
				"login":      "odio",
				"name":       "Kayden Rutherford",
				"email":      "wilburnstracke@schinner.io",
				"company":    "et",
				"location":   "Et perspiciatis deleniti et accusamus.",
				"blog":       "http://www.productsticky.info/virtual",
				"followers":   86,
				"following":   24,
			},
		},
	}
}

// generateAIStandardResult shows AI faker for standard repository
func generateAIStandardResult() map[string]interface{} {
	return map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           987654321,
			"name":         "react-dashboard",
			"fullName":     "techcompany/react-dashboard",
			"description":  "A modern React dashboard component with TypeScript support and real-time data visualization.",
			"private":      false,
			"language":     "TypeScript",
			"stars":        1250,
			"forks":        342,
			"openIssues":   18,
			"createdAt":    "2023-06-12T14:30:00Z",
			"updatedAt":    "2024-10-28T09:15:00Z",
			"owner": map[string]interface{}{
				"id":         12345678,
				"login":      "techcompany",
				"name":       "Tech Company Inc",
				"email":      "opensource@techcompany.com",
				"company":    "Tech Company Inc",
				"location":   "San Francisco, CA",
				"blog":       "https://techcompany.com/blog",
				"followers":   2500,
				"following":   180,
			},
		},
	}
}

// generateAIHighActivityResult shows AI faker for high-activity repository
func generateAIHighActivityResult() map[string]interface{} {
	return map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           555666777,
			"name":         "tensorflow",
			"fullName":     "tensorflow/tensorflow",
			"description":  "An end-to-end open source platform for machine learning with comprehensive ecosystem and extensive community support.",
			"private":      false,
			"language":     "C++",
			"stars":        185000,
			"forks":        74200,
			"openIssues":   3420,
			"createdAt":    "2015-11-09T02:15:00Z",
			"updatedAt":    "2024-11-01T16:45:00Z",
			"owner": map[string]interface{}{
				"id":         149393,
				"login":      "tensorflow",
				"name":       "TensorFlow",
				"email":      "tensorflow@google.com",
				"company":    "Google",
				"location":   "Mountain View, CA",
				"blog":       "https://tensorflow.org",
				"followers":   5200,
				"following":   45,
			},
		},
	}
}

// generateAIEnterpriseResult shows AI faker for enterprise repository
func generateAIEnterpriseResult() map[string]interface{} {
	return map[string]interface{}{
		"repository": map[string]interface{}{
			"id":           111222333,
			"name":         "payment-service",
			"fullName":     "acme-corp/payment-service",
			"description":  "Enterprise payment processing service with PCI compliance, high availability, and comprehensive audit logging.",
			"private":      true,
			"language":     "Java",
			"stars":        145,
			"forks":        28,
			"openIssues":   7,
			"createdAt":    "2022-03-15T11:20:00Z",
			"updatedAt":    "2024-10-30T13:10:00Z",
			"owner": map[string]interface{}{
				"id":         98765432,
				"login":      "acme-corp",
				"name":       "ACME Corporation",
				"email":      "dev.team@acme.com",
				"company":    "ACME Corporation",
				"location":   "New York, NY",
				"blog":       "https://acme.com/tech-blog",
				"followers":   320,
				"following":   85,
			},
		},
	}
}

// demonstrateUserExperience shows how users would interact with the system
func demonstrateUserExperience() {
	fmt.Println("\n🚀 Getting Started (Developer Workflow):")
	fmt.Println("==========================================")
	
	fmt.Println("\n1️⃣ List Available Templates:")
	fmt.Println("   $ stn faker --ai-template list --command echo")
	fmt.Println("   📋 Shows 27 templates across 9 categories")
	
	fmt.Println("\n2️⃣ Basic Usage (Template-based):")
	fmt.Println("   $ stn faker \\")
	fmt.Println("     --command 'npx' \\")
	fmt.Println("     --args '-y,@datadog/mcp-server-datadog' \\")
	fmt.Println("     --ai-enabled \\")
	fmt.Println("     --ai-template 'monitoring-high-alert'")
	fmt.Println("   🎯 Instant contextual data for monitoring scenarios")
	
	fmt.Println("\n3️⃣ Custom Instructions:")
	fmt.Println("   $ stn faker \\")
	fmt.Println("     --command 'npx' \\")
	fmt.Println("     --args '-y,@stripe/mcp-server-stripe' \\")
	fmt.Println("     --ai-enabled \\")
	fmt.Println("     --ai-instruction 'Generate realistic fintech transaction data with proper compliance patterns'")
	fmt.Println("   ✏️ Fully customized data generation")
	
	fmt.Println("\n4️⃣ Environment Configuration:")
	fmt.Println("   $ export GOOGLE_GENAI_API_KEY=your-api-key")
	fmt.Println("   $ stn faker --ai-enabled --command 'npx' --args '-y,@github/mcp-server-github'")
	fmt.Println("   🔧 Environment-based configuration")
	
	fmt.Println("\n📦 Template.json Integration:")
	fmt.Println("==============================")
	templateExample := `{
  "mcpServers": {
    "datadog-monitoring": {
      "command": "stn",
      "args": [
        "faker",
        "--command", "npx",
        "--args", "-y,@datadog/mcp-server-datadog",
        "--ai-enabled",
        "--ai-template", "monitoring-high-alert"
      ]
    }
  }
}`
	fmt.Println(templateExample)
	
	fmt.Println("\n🎭 Use Case Scenarios:")
	fmt.Println("=====================")
	
	scenarios := []struct {
		title       string
		description string
		template    string
		experience  string
	}{
		{
			title:       "🔒 Security Testing",
			description: "Test agents against realistic security alerts",
			template:    "monitoring-high-alert",
			experience:  "Agents see critical alerts, high error rates, urgent warnings - perfect for testing incident response workflows",
		},
		{
			title:       "💰 Financial Services",
			description: "Validate fintech agents with proper transaction data",
			template:    "financial-transactions",
			experience:  "Realistic monetary amounts, proper account formats, plausible merchant names - no more 'voluptas' transactions",
		},
		{
			title:       "🏥 Healthcare Systems",
			description: "Medical AI agents need clinically accurate data",
			template:    "healthcare-patient",
			experience:  "Valid vital signs ranges, proper medical terminology, realistic patient IDs - HIPAA-compliant testing data",
		},
		{
			title:       "🛒 E-commerce Testing",
			description: "Retail automation agents need realistic order data",
			template:    "ecommerce-orders",
			experience:  "Proper order IDs, realistic product names, valid shipping addresses, plausible pricing",
		},
	}
	
	for _, scenario := range scenarios {
		fmt.Printf("\n%s\n", scenario.title)
		fmt.Printf("   %s\n", scenario.description)
		fmt.Printf("   Template: %s\n", scenario.template)
		fmt.Printf("   UX: %s\n", scenario.experience)
	}
	
	fmt.Println("\n🎯 Key Benefits:")
	fmt.Println("================")
	fmt.Println("✅ Eliminates 'slop' - no more random Latin words")
	fmt.Println("✅ Contextually appropriate - data matches domain")
	fmt.Println("✅ Schema-correct - maintains exact API structure")
	fmt.Println("✅ Easy to use - template-based or custom instructions")
	fmt.Println("✅ Graceful fallback - basic faker if AI fails")
	fmt.Println("✅ Cost-effective - cached responses, smart prompting")
	
	fmt.Println("\n🔮 Future Enhancements:")
	fmt.Println("======================")
	fmt.Println("📊 Analytics dashboard for template usage")
	fmt.Println("🎨 More domain-specific templates (gaming, social media, etc.)")
	fmt.Println("🔄 Template versioning and updates")
	fmt.Println("🌍 Multi-language support for international data")
	fmt.Println("⚡ Performance optimizations and response caching")
}