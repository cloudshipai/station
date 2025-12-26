package dotprompt

import "testing"

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain JSON unchanged",
			input:    `{"service_name":"test","raw_metrics":[]}`,
			expected: `{"service_name":"test","raw_metrics":[]}`,
		},
		{
			name:     "markdown json fence",
			input:    "```json\n{\"service_name\":\"test\"}\n```",
			expected: `{"service_name":"test"}`,
		},
		{
			name:     "markdown json fence with whitespace",
			input:    "  ```json\n{\"value\": 42}\n```  ",
			expected: `{"value": 42}`,
		},
		{
			name:     "plain backtick fence",
			input:    "```\n{\"key\":\"value\"}\n```",
			expected: `{"key":"value"}`,
		},
		{
			name:     "multiline JSON in fence",
			input:    "```json\n{\n  \"a\": 1,\n  \"b\": 2\n}\n```",
			expected: "{\n  \"a\": 1,\n  \"b\": 2\n}",
		},
		{
			name:     "no closing fence",
			input:    "```json\n{\"partial\":true}",
			expected: `{"partial":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanJSONResponse(tt.input)
			if result != tt.expected {
				t.Errorf("cleanJSONResponse() = %q, want %q", result, tt.expected)
			}
		})
	}
}
