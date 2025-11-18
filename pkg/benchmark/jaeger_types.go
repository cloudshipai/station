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

			// Extract input and output from span tags
			for _, tag := range span.Tags {
				switch tag.Key {
				case "tool.input":
					// Store as Parameters (input is stored in Parameters field)
					if strVal, ok := tag.Value.(string); ok {
						var params map[string]interface{}
						// Try to parse as JSON, fallback to raw map
						if err := json.Unmarshal([]byte(strVal), &params); err == nil {
							toolCall.Parameters = params
						}
					} else if mapVal, ok := tag.Value.(map[string]interface{}); ok {
						toolCall.Parameters = mapVal
					}
				case "tool.output":
					toolCall.Output = tag.Value
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
