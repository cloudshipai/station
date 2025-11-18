package services

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to check if Jaeger is running
func isJaegerAvailable() bool {
	resp, err := http.Get("http://localhost:16686")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func TestNewJaegerClient(t *testing.T) {
	// Test with custom URL
	client := NewJaegerClient("http://jaeger.example.com:16686")
	assert.Equal(t, "http://jaeger.example.com:16686", client.baseURL)

	// Test with empty URL (should default to localhost)
	client = NewJaegerClient("")
	assert.Equal(t, "http://localhost:16686", client.baseURL)

	// Test with trailing slash removal
	client = NewJaegerClient("http://localhost:16686/")
	assert.Equal(t, "http://localhost:16686", client.baseURL)
}

func TestJaegerClient_IsAvailable(t *testing.T) {
	if !isJaegerAvailable() {
		t.Skip("Jaeger not running")
	}

	client := NewJaegerClient("")
	assert.True(t, client.IsAvailable(), "Jaeger should be available")

	// Test with invalid URL
	badClient := NewJaegerClient("http://localhost:99999")
	assert.False(t, badClient.IsAvailable(), "Invalid Jaeger URL should not be available")
}

func TestJaegerClient_QueryTraces(t *testing.T) {
	if !isJaegerAvailable() {
		t.Skip("Jaeger not running")
	}

	client := NewJaegerClient("")

	// Query for station service traces
	traces, err := client.QueryTraces("station", "", nil, 5)
	require.NoError(t, err, "Should be able to query traces")

	// We might not have traces, but the query should succeed
	assert.GreaterOrEqual(t, len(traces), 0, "Should return trace array (possibly empty)")

	// If we have traces, verify structure
	if len(traces) > 0 {
		trace := traces[0]
		assert.NotEmpty(t, trace.TraceID, "Trace should have ID")
		assert.NotNil(t, trace.Spans, "Trace should have spans")
	}
}

func TestGetSpanTag(t *testing.T) {
	span := &JaegerSpan{
		Tags: []SpanTag{
			{Key: "string_tag", Type: "string", Value: "hello"},
			{Key: "int_tag", Type: "int64", Value: int64(42)},
			{Key: "bool_tag", Type: "bool", Value: true},
		},
	}

	// Test existing tags
	val, ok := GetSpanTag(span, "string_tag")
	assert.True(t, ok)
	assert.Equal(t, "hello", val)

	val, ok = GetSpanTag(span, "int_tag")
	assert.True(t, ok)
	assert.Equal(t, int64(42), val)

	val, ok = GetSpanTag(span, "bool_tag")
	assert.True(t, ok)
	assert.Equal(t, true, val)

	// Test non-existent tag
	_, ok = GetSpanTag(span, "missing_tag")
	assert.False(t, ok)
}

func TestGetSpanTagString(t *testing.T) {
	span := &JaegerSpan{
		Tags: []SpanTag{
			{Key: "name", Type: "string", Value: "test-agent"},
			{Key: "count", Type: "int64", Value: int64(5)},
		},
	}

	// Test string tag
	str, ok := GetSpanTagString(span, "name")
	assert.True(t, ok)
	assert.Equal(t, "test-agent", str)

	// Test non-string tag
	_, ok = GetSpanTagString(span, "count")
	assert.False(t, ok)

	// Test missing tag
	_, ok = GetSpanTagString(span, "missing")
	assert.False(t, ok)
}

func TestGetSpanTagInt64(t *testing.T) {
	span := &JaegerSpan{
		Tags: []SpanTag{
			{Key: "run_id", Type: "int64", Value: int64(12847)},
			{Key: "count_float", Type: "float64", Value: float64(42.0)},
			{Key: "count_int", Type: "int", Value: int(99)},
			{Key: "name", Type: "string", Value: "test"},
		},
	}

	// Test int64 tag
	val, ok := GetSpanTagInt64(span, "run_id")
	assert.True(t, ok)
	assert.Equal(t, int64(12847), val)

	// Test float64 conversion
	val, ok = GetSpanTagInt64(span, "count_float")
	assert.True(t, ok)
	assert.Equal(t, int64(42), val)

	// Test int conversion
	val, ok = GetSpanTagInt64(span, "count_int")
	assert.True(t, ok)
	assert.Equal(t, int64(99), val)

	// Test non-numeric tag
	_, ok = GetSpanTagInt64(span, "name")
	assert.False(t, ok)
}

func TestBuildSpanTree(t *testing.T) {
	// Create a simple parent-child span relationship
	spans := []JaegerSpan{
		{
			SpanID:        "span1",
			OperationName: "parent-operation",
			References:    []SpanRef{}, // Root span
			StartTime:     1000,
			Duration:      5000,
		},
		{
			SpanID:        "span2",
			OperationName: "child-operation",
			References: []SpanRef{
				{RefType: "CHILD_OF", SpanID: "span1"},
			},
			StartTime: 2000,
			Duration:  2000,
		},
		{
			SpanID:        "span3",
			OperationName: "another-child",
			References: []SpanRef{
				{RefType: "CHILD_OF", SpanID: "span1"},
			},
			StartTime: 4000,
			Duration:  500,
		},
	}

	tree := BuildSpanTree(spans)

	require.NotNil(t, tree, "Should build span tree")
	assert.Equal(t, "parent-operation", tree.Span.OperationName)
	assert.Equal(t, 0, tree.Depth)
	assert.Len(t, tree.Children, 2, "Parent should have 2 children")

	// Children should be sorted by start time
	assert.Equal(t, "child-operation", tree.Children[0].Span.OperationName)
	assert.Equal(t, "another-child", tree.Children[1].Span.OperationName)
	assert.Equal(t, 1, tree.Children[0].Depth)
	assert.Equal(t, 1, tree.Children[1].Depth)
}

func TestBuildSpanTree_EmptySpans(t *testing.T) {
	tree := BuildSpanTree([]JaegerSpan{})
	assert.Nil(t, tree, "Empty spans should return nil tree")
}

func TestExtractToolSequence(t *testing.T) {
	// Create span tree with tool calls
	spans := []JaegerSpan{
		{
			SpanID:        "root",
			OperationName: "agent.execute",
			References:    []SpanRef{},
			StartTime:     1000000,
			Duration:      10000,
		},
		{
			SpanID:        "tool1",
			OperationName: "__read_file",
			References:    []SpanRef{{RefType: "CHILD_OF", SpanID: "root"}},
			StartTime:     2000000,
			Duration:      500,
			Tags: []SpanTag{
				{Key: "tool.input", Type: "string", Value: `{"path": "/test.txt"}`},
				{Key: "tool.output", Type: "string", Value: "file contents"},
			},
		},
		{
			SpanID:        "tool2",
			OperationName: "__search_files",
			References:    []SpanRef{{RefType: "CHILD_OF", SpanID: "root"}},
			StartTime:     3000000,
			Duration:      1200,
			Tags: []SpanTag{
				{Key: "tool.input", Type: "string", Value: `{"pattern": "*.go"}`},
				{Key: "error", Type: "string", Value: "search failed"},
			},
		},
	}

	tree := BuildSpanTree(spans)
	toolSequence := ExtractToolSequence(tree)

	require.Len(t, toolSequence, 2, "Should extract 2 tool calls")

	// First tool call
	assert.Equal(t, 1, toolSequence[0].Step)
	assert.Equal(t, "__read_file", toolSequence[0].Tool)
	assert.Equal(t, float64(0.5), toolSequence[0].DurationMs)
	assert.True(t, toolSequence[0].Success)
	assert.Equal(t, "/test.txt", toolSequence[0].Input["path"])
	assert.Equal(t, "file contents", toolSequence[0].Output)

	// Second tool call (failed)
	assert.Equal(t, 2, toolSequence[1].Step)
	assert.Equal(t, "__search_files", toolSequence[1].Tool)
	assert.Equal(t, float64(1.2), toolSequence[1].DurationMs)
	assert.False(t, toolSequence[1].Success)
	assert.Equal(t, "search failed", toolSequence[1].Error)
}

func TestCalculateTimingBreakdown(t *testing.T) {
	// Create span tree with different operation types
	spans := []JaegerSpan{
		{
			SpanID:        "root",
			OperationName: "agent.execute",
			References:    []SpanRef{},
			StartTime:     0,
			Duration:      10000000, // 10 seconds
		},
		{
			SpanID:        "setup",
			OperationName: "mcp.server.start",
			References:    []SpanRef{{RefType: "CHILD_OF", SpanID: "root"}},
			Duration:      500000, // 500ms
		},
		{
			SpanID:        "exec",
			OperationName: "dotprompt.execute",
			References:    []SpanRef{{RefType: "CHILD_OF", SpanID: "root"}},
			Duration:      8000000, // 8 seconds
		},
		{
			SpanID:        "tool",
			OperationName: "__read_file",
			References:    []SpanRef{{RefType: "CHILD_OF", SpanID: "exec"}},
			Duration:      100000, // 100ms
		},
		{
			SpanID:        "cleanup",
			OperationName: "db.agent_runs.update_completion",
			References:    []SpanRef{{RefType: "CHILD_OF", SpanID: "root"}},
			Duration:      50000, // 50ms
		},
	}

	tree := BuildSpanTree(spans)
	breakdown := CalculateTimingBreakdown(tree)

	assert.Equal(t, float64(10000), breakdown.TotalMs)
	assert.Equal(t, float64(500), breakdown.SetupMs)
	assert.Equal(t, float64(8000), breakdown.ExecutionMs)
	assert.Equal(t, float64(100), breakdown.ToolsMs)
	assert.Equal(t, float64(50), breakdown.CleanupMs)
	assert.Equal(t, float64(7900), breakdown.ReasoningMs) // Execution - tools
}
