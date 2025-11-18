package benchmark

import "encoding/json"

// JaegerTrace represents a complete trace from Jaeger API
// This mirrors the structure from internal/services/jaeger_client.go
type JaegerTrace struct {
	TraceID   string                   `json:"traceID"`
	Spans     []JaegerSpan             `json:"spans"`
	Processes map[string]JaegerProcess `json:"processes,omitempty"`
}

// JaegerSpan represents a single span in a trace
type JaegerSpan struct {
	TraceID       string    `json:"traceID"`
	SpanID        string    `json:"spanID"`
	OperationName string    `json:"operationName"`
	References    []SpanRef `json:"references"`
	StartTime     int64     `json:"startTime"` // Microseconds since epoch
	Duration      int64     `json:"duration"`  // Microseconds
	Tags          []SpanTag `json:"tags"`
	Logs          []SpanLog `json:"logs,omitempty"`
	ProcessID     string    `json:"processID,omitempty"`
}

// SpanRef represents a reference to another span (parent/child relationship)
type SpanRef struct {
	RefType string `json:"refType"` // CHILD_OF or FOLLOWS_FROM
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

// SpanTag represents a key-value tag on a span
type SpanTag struct {
	Key   string      `json:"key"`
	Type  string      `json:"type"` // string, int64, bool, float64
	Value interface{} `json:"value"`
}

// SpanLog represents a log entry in a span
type SpanLog struct {
	Timestamp int64     `json:"timestamp"` // Microseconds
	Fields    []SpanTag `json:"fields"`
}

// JaegerProcess represents process information
type JaegerProcess struct {
	ServiceName string    `json:"serviceName"`
	Tags        []SpanTag `json:"tags"`
}

// ExtractToolCallsFromTrace extracts tool calls with inputs/outputs from Jaeger spans
// Genkit creates spans with genkit:input and genkit:output tags for tool calls
func ExtractToolCallsFromTrace(trace *JaegerTrace) []ToolCall {
	if trace == nil {
		return nil
	}

	var toolCalls []ToolCall
	for _, span := range trace.Spans {
		// Only process spans that represent tool calls (start with __)
		if len(span.OperationName) > 2 && span.OperationName[:2] == "__" {
			toolCall := ToolCall{
				Name: span.OperationName,
			}

			// Extract input and output from Genkit span tags
			for _, tag := range span.Tags {
				switch tag.Key {
				case "genkit:input":
					// Genkit stores input as JSON string or map
					if strVal, ok := tag.Value.(string); ok {
						var params map[string]interface{}
						// Try to parse as JSON
						if err := json.Unmarshal([]byte(strVal), &params); err == nil {
							toolCall.Parameters = params
						}
					} else if mapVal, ok := tag.Value.(map[string]interface{}); ok {
						toolCall.Parameters = mapVal
					}
				case "genkit:output":
					// Genkit output is structured: {"content":[{"type":"text","text":"..."}]}
					// Extract the actual output text/data
					if strVal, ok := tag.Value.(string); ok {
						// Parse the genkit output structure
						var genkitOutput map[string]interface{}
						if err := json.Unmarshal([]byte(strVal), &genkitOutput); err == nil {
							// Extract content array
							if content, ok := genkitOutput["content"].([]interface{}); ok && len(content) > 0 {
								if firstContent, ok := content[0].(map[string]interface{}); ok {
									if text, ok := firstContent["text"].(string); ok {
										toolCall.Output = text
									}
								}
							}
						} else {
							// Fallback to raw string
							toolCall.Output = strVal
						}
					} else if mapVal, ok := tag.Value.(map[string]interface{}); ok {
						// Handle map format
						if content, ok := mapVal["content"].([]interface{}); ok && len(content) > 0 {
							if firstContent, ok := content[0].(map[string]interface{}); ok {
								if text, ok := firstContent["text"].(string); ok {
									toolCall.Output = text
								}
							}
						}
					}
				}
			}

			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls
}

// GetSpanTag retrieves a tag value from a span by key
func GetSpanTag(span *JaegerSpan, key string) (interface{}, bool) {
	for _, tag := range span.Tags {
		if tag.Key == key {
			return tag.Value, true
		}
	}
	return nil, false
}
