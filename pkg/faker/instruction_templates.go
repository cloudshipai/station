package faker

import "fmt"

// PredefinedInstructionTemplates provides common scenarios for AI enrichment
type PredefinedInstructionTemplates struct {
	templates map[string]string
}

// NewPredefinedInstructionTemplates creates a new templates instance
func NewPredefinedInstructionTemplates() *PredefinedInstructionTemplates {
	templates := map[string]string{
		// Monitoring & Observability
		"monitoring-high-alert": "Generate alert-heavy monitoring data with high error rates, critical warnings, urgent status indicators, and elevated resource utilization. Use values that would trigger monitoring alerts and pages.",
		"monitoring-healthy": "Generate healthy monitoring data with normal error rates, stable performance metrics, green status indicators, and optimal resource utilization. Show a well-functioning system.",
		"monitoring-mixed": "Generate realistic monitoring data with mixed status indicators, moderate error rates, some performance degradation, and both healthy and problematic components.",
		
		// Financial Services
		"financial-transactions": "Generate realistic financial transaction data with proper monetary amounts (2 decimal places), valid account numbers, appropriate timestamps, plausible merchant names, and realistic transaction categories.",
		"financial-trading": "Generate realistic trading data with proper stock symbols, accurate price movements (2-4 decimal places), realistic volumes, appropriate timestamps, and plausible market conditions.",
		"financial-budgeting": "Generate realistic budget data with proper allocation percentages, variance analysis, spending categories, quarterly breakdowns, and plausible financial metrics.",
		
		// Healthcare
		"healthcare-patient": "Generate realistic medical patient data with proper patient IDs, valid vital signs ranges (HR 60-100, BP 90-140/60-90), appropriate medical terminology, plausible clinical information, and realistic timestamps.",
		"healthcare-laboratory": "Generate realistic laboratory test results with proper medical units, valid reference ranges, appropriate test names, plausible values, and accurate clinical terminology.",
		"healthcare-medication": "Generate realistic medication data with proper drug names, accurate dosages, appropriate schedules, plausible administration routes, and realistic timing information.",
		
		// E-commerce & Retail
		"ecommerce-orders": "Generate realistic e-commerce order data with proper order IDs, plausible product names, realistic quantities, appropriate pricing, valid shipping addresses, and realistic timestamps.",
		"ecommerce-inventory": "Generate realistic inventory data with proper SKU numbers, plausible stock levels, appropriate reorder points, realistic warehouse locations, and accurate product categories.",
		"ecommerce-customer": "Generate realistic customer data with proper customer IDs, plausible shopping behavior, realistic order history, appropriate segmentation, and valid contact information.",
		
		// DevOps & Infrastructure
		"devops-deployment": "Generate realistic deployment data with proper version numbers, plausible deployment durations, appropriate environment names, realistic rollback scenarios, and accurate status indicators.",
		"devops-performance": "Generate realistic performance data with proper response times, plausible throughput metrics, appropriate error rates, realistic resource utilization, and accurate service dependencies.",
		"devops-security": "Generate realistic security data with proper threat levels, plausible vulnerability descriptions, appropriate risk scores, realistic incident timelines, and accurate mitigation actions.",
		
		// IoT & Sensors
		"iot-environmental": "Generate realistic environmental sensor data with proper temperature ranges (-20°C to 50°C), appropriate humidity levels (0-100%), plausible air quality indices, accurate timestamps, realistic sensor locations, and proper sensor calibration data.",
		"iot-industrial": "Generate realistic industrial IoT data with proper machine metrics, appropriate production rates, plausible maintenance indicators, accurate sensor readings, and realistic operational status.",
		"iot-smart-home": "Generate realistic smart home data with proper device states, appropriate energy consumption levels, plausible automation triggers, accurate user preferences, and realistic usage patterns.",
		
		// Human Resources
		"hr-employee": "Generate realistic employee data with proper employee IDs, plausible job titles, appropriate salary ranges, realistic department assignments, and valid work history.",
		"hr-recruitment": "Generate realistic recruitment data with proper candidate IDs, plausible skill assessments, appropriate interview scores, realistic application timelines, and accurate hiring decisions.",
		"hr-performance": "Generate realistic performance data with proper review cycles, plausible rating scales, appropriate goal completion metrics, realistic feedback comments, and accurate development plans.",
		
		// Logistics & Supply Chain
		"logistics-shipping": "Generate realistic shipping data with proper tracking numbers, plausible route information, appropriate delivery timestamps, realistic carrier details, and accurate package dimensions.",
		"logistics-warehouse": "Generate realistic warehouse data with proper location codes, plausible inventory movements, appropriate picking times, realistic storage utilization, and accurate order fulfillment metrics.",
		"logistics-fleet": "Generate realistic fleet data with proper vehicle IDs, plausible fuel consumption, appropriate maintenance schedules, realistic route optimization, and accurate driver performance.",
		
		// Education & Learning
		"education-student": "Generate realistic student data with proper student IDs, plausible grade distributions, appropriate attendance records, realistic course enrollments, and accurate academic performance.",
		"education-course": "Generate realistic course data with proper course codes, plausible enrollment numbers, appropriate completion rates, realistic assessment scores, and accurate learning outcomes.",
		"education-learning": "Generate realistic learning analytics with proper engagement metrics, plausible completion times, appropriate knowledge assessments, realistic skill progression, and accurate learning paths.",
	}

	return &PredefinedInstructionTemplates{templates: templates}
}

// GetTemplate returns a predefined instruction template by name
func (t *PredefinedInstructionTemplates) GetTemplate(name string) (string, error) {
	template, exists := t.templates[name]
	if !exists {
		return "", fmt.Errorf("template '%s' not found", name)
	}
	return template, nil
}

// ListTemplates returns all available template names
func (t *PredefinedInstructionTemplates) ListTemplates() []string {
	names := make([]string, 0, len(t.templates))
	for name := range t.templates {
		names = append(names, name)
	}
	return names
}

// GetTemplateByCategory returns templates filtered by category
func (t *PredefinedInstructionTemplates) GetTemplateByCategory(category string) map[string]string {
	result := make(map[string]string)
	categoryPrefix := category + "-"
	
	for name, template := range t.templates {
		if len(name) > len(categoryPrefix) && name[:len(categoryPrefix)] == categoryPrefix {
			result[name] = template
		}
	}
	
	return result
}

// GetCategories returns all available categories
func (t *PredefinedInstructionTemplates) GetCategories() []string {
	categories := make(map[string]bool)
	
	for name := range t.templates {
		if dashIndex := findDashIndex(name); dashIndex != -1 {
			category := name[:dashIndex]
			categories[category] = true
		}
	}
	
	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}
	return result
}

// findDashIndex finds the first dash in a string
func findDashIndex(s string) int {
	for i, char := range s {
		if char == '-' {
			return i
		}
	}
	return -1
}