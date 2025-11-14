package ai

// GetDefaultModel returns the default model for a given provider
func GetDefaultModel(provider string) string {
	defaults := map[string]string{
		"openai":      "gpt-4o-mini",
		"gemini":      "gemini-1.5-flash",
		"googlegenai": "gemini-1.5-flash",
	}

	if model, ok := defaults[provider]; ok {
		return model
	}

	return "gpt-4o-mini" // Fallback default
}
