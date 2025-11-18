package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"station/pkg/types"
)

// JaegerClient provides HTTP API access to Jaeger for trace querying
type JaegerClient struct {
	baseURL    string // e.g., http://localhost:16686
	httpClient *http.Client
}

// JaegerTrace represents a complete trace from Jaeger API
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

// JaegerResponse represents the API response wrapper
type JaegerResponse struct {
	Data   []JaegerTrace `json:"data"`
	Total  int           `json:"total,omitempty"`
	Limit  int           `json:"limit,omitempty"`
	Offset int           `json:"offset,omitempty"`
	Errors []JaegerError `json:"errors,omitempty"`
}

// JaegerError represents an error from Jaeger API
type JaegerError struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	TraceID string `json:"traceID,omitempty"`
}

// NewJaegerClient creates a new Jaeger HTTP API client
func NewJaegerClient(baseURL string) *JaegerClient {
	// Default to localhost if not specified
	if baseURL == "" {
		baseURL = "http://localhost:16686"
	}

	// Remove trailing slash
	baseURL = strings.TrimSuffix(baseURL, "/")

	return &JaegerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsAvailable checks if Jaeger is accessible
func (jc *JaegerClient) IsAvailable() bool {
	resp, err := jc.httpClient.Get(jc.baseURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// QueryRunTrace queries Jaeger for a trace associated with a specific run ID
func (jc *JaegerClient) QueryRunTrace(runID int64, serviceName string) (*JaegerTrace, error) {
	if serviceName == "" {
		serviceName = "station"
	}

	// Build URL with tag filter - use tag=key:value format (not tags={json})
	queryURL := fmt.Sprintf("%s/api/traces?service=%s&tag=run.id:%d&limit=1",
		jc.baseURL, serviceName, runID)

	// Execute request
	resp, err := jc.httpClient.Get(queryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger for run %d: %w", runID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Jaeger API returned status %d for run %d", resp.StatusCode, runID)
	}

	// Parse response
	var jaegerResp JaegerResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jaeger response: %w", err)
	}

	if len(jaegerResp.Errors) > 0 {
		return nil, fmt.Errorf("Jaeger API errors: %v", jaegerResp.Errors)
	}

	if len(jaegerResp.Data) == 0 {
		return nil, fmt.Errorf("no trace found for run %d", runID)
	}

	return &jaegerResp.Data[0], nil
}

// QueryTraces queries Jaeger for traces matching filters
func (jc *JaegerClient) QueryTraces(service string, operation string, tags map[string]string, limit int) ([]JaegerTrace, error) {
	if service == "" {
		service = "station"
	}
	if limit == 0 {
		limit = 20
	}

	// Build query parameters
	params := url.Values{}
	params.Set("service", service)
	params.Set("limit", fmt.Sprintf("%d", limit))

	if operation != "" {
		params.Set("operation", operation)
	}

	// Add tags as query parameters
	for key, value := range tags {
		params.Set("tags", fmt.Sprintf(`{"key":"%s","value":"%s"}`, key, value))
	}

	// Build URL
	queryURL := fmt.Sprintf("%s/api/traces?%s", jc.baseURL, params.Encode())

	// Execute request
	resp, err := jc.httpClient.Get(queryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Jaeger API returned status %d", resp.StatusCode)
	}

	// Parse response
	var jaegerResp JaegerResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jaeger response: %w", err)
	}

	if len(jaegerResp.Errors) > 0 {
		return nil, fmt.Errorf("Jaeger API errors: %v", jaegerResp.Errors)
	}

	return jaegerResp.Data, nil
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

// GetSpanTagString retrieves a string tag value
func GetSpanTagString(span *JaegerSpan, key string) (string, bool) {
	if val, ok := GetSpanTag(span, key); ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetSpanTagInt64 retrieves an int64 tag value
func GetSpanTagInt64(span *JaegerSpan, key string) (int64, bool) {
	if val, ok := GetSpanTag(span, key); ok {
		switch v := val.(type) {
		case int64:
			return v, true
		case float64:
			return int64(v), true
		case int:
			return int64(v), true
		}
	}
	return 0, false
}

// SpanTree represents a hierarchical tree of spans
type SpanTree struct {
	Span     *JaegerSpan
	Children []*SpanTree
	Depth    int
}

// BuildSpanTree constructs a hierarchical span tree from a flat list of spans
func BuildSpanTree(spans []JaegerSpan) *SpanTree {
	if len(spans) == 0 {
		return nil
	}

	// Build span map for quick lookup
	spanMap := make(map[string]*JaegerSpan)
	for i := range spans {
		spanMap[spans[i].SpanID] = &spans[i]
	}

	// Build parent-child relationships
	childMap := make(map[string][]*JaegerSpan)
	var root *JaegerSpan

	for i := range spans {
		span := &spans[i]
		if len(span.References) == 0 {
			// This is a root span
			root = span
		} else {
			// Find parent reference
			for _, ref := range span.References {
				if ref.RefType == "CHILD_OF" {
					childMap[ref.SpanID] = append(childMap[ref.SpanID], span)
					break
				}
			}
		}
	}

	if root == nil && len(spans) > 0 {
		// No root found, use first span
		root = &spans[0]
	}

	// Build tree recursively
	return buildSpanTreeRecursive(root, childMap, 0)
}

func buildSpanTreeRecursive(span *JaegerSpan, childMap map[string][]*JaegerSpan, depth int) *SpanTree {
	tree := &SpanTree{
		Span:     span,
		Children: []*SpanTree{},
		Depth:    depth,
	}

	// Add children
	if children, ok := childMap[span.SpanID]; ok {
		// Sort children by start time
		sort.Slice(children, func(i, j int) bool {
			return children[i].StartTime < children[j].StartTime
		})

		for _, child := range children {
			childTree := buildSpanTreeRecursive(child, childMap, depth+1)
			tree.Children = append(tree.Children, childTree)
		}
	}

	return tree
}

// ExtractToolSequence extracts the ordered sequence of tool calls from a span tree
func ExtractToolSequence(tree *SpanTree) []types.ToolCallTrace {
	toolCalls := []types.ToolCallTrace{}
	extractToolCallsRecursive(tree, &toolCalls, 1)
	return toolCalls
}

func extractToolCallsRecursive(tree *SpanTree, toolCalls *[]types.ToolCallTrace, step int) int {
	// Check if this span represents a tool call (starts with __ prefix)
	if strings.HasPrefix(tree.Span.OperationName, "__") {
		toolCall := types.ToolCallTrace{
			Step:       step,
			Tool:       tree.Span.OperationName,
			SpanID:     tree.Span.SpanID,
			StartTime:  time.Unix(0, tree.Span.StartTime*1000), // Convert microseconds to nanoseconds
			DurationMs: float64(tree.Span.Duration) / 1000.0,   // Convert microseconds to milliseconds
			Success:    true,                                   // Assume success unless error tag found
		}

		// Extract input/output from tags
		for _, tag := range tree.Span.Tags {
			switch tag.Key {
			case "tool.input":
				if str, ok := tag.Value.(string); ok {
					var input map[string]interface{}
					if err := json.Unmarshal([]byte(str), &input); err == nil {
						toolCall.Input = input
					}
				}
			case "tool.output":
				if str, ok := tag.Value.(string); ok {
					toolCall.Output = str
				}
			case "error":
				if str, ok := tag.Value.(string); ok {
					toolCall.Error = str
					toolCall.Success = false
				}
			case "error.message":
				if str, ok := tag.Value.(string); ok {
					toolCall.Error = str
					toolCall.Success = false
				}
			}
		}

		*toolCalls = append(*toolCalls, toolCall)
		step++
	}

	// Recurse into children
	for _, child := range tree.Children {
		step = extractToolCallsRecursive(child, toolCalls, step)
	}

	return step
}

// CalculateTimingBreakdown calculates timing breakdown from span tree
func CalculateTimingBreakdown(tree *SpanTree) *types.TimingBreakdown {
	breakdown := &types.TimingBreakdown{
		TotalMs: float64(tree.Span.Duration) / 1000.0,
	}

	// Traverse tree and categorize spans
	calculateTimingRecursive(tree, breakdown)

	// Calculate reasoning time (execution - tools - setup - cleanup)
	breakdown.ReasoningMs = breakdown.ExecutionMs - breakdown.ToolsMs

	return breakdown
}

func calculateTimingRecursive(tree *SpanTree, breakdown *types.TimingBreakdown) {
	durationMs := float64(tree.Span.Duration) / 1000.0

	switch {
	case tree.Span.OperationName == "dotprompt.execute":
		breakdown.ExecutionMs = durationMs
	case tree.Span.OperationName == "mcp.server.start":
		breakdown.SetupMs += durationMs
	case strings.HasPrefix(tree.Span.OperationName, "db."):
		if strings.Contains(tree.Span.OperationName, "update") || strings.Contains(tree.Span.OperationName, "completion") {
			breakdown.CleanupMs += durationMs
		} else {
			breakdown.SetupMs += durationMs
		}
	case strings.HasPrefix(tree.Span.OperationName, "__"):
		breakdown.ToolsMs += durationMs
	}

	// Recurse into children
	for _, child := range tree.Children {
		calculateTimingRecursive(child, breakdown)
	}
}
