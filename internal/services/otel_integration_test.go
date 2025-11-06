package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOTELExportToJaeger tests that spans are exported to Jaeger
func TestOTELExportToJaeger(t *testing.T) {
	// Check if Jaeger is available
	if !isJaegerAvailable() {
		t.Skip("Jaeger not running at localhost:16686, start with 'make jaeger'")
	}

	ctx := context.Background()

	// Create OTEL telemetry service
	config := &TelemetryConfig{
		Enabled:      true,
		OTLPEndpoint: "localhost:4318",
		ServiceName:  "station-test",
		Environment:  "test",
	}

	ts := NewTelemetryService(config)
	err := ts.Initialize(ctx)
	require.NoError(t, err, "OTEL telemetry initialization should succeed")
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ts.Shutdown(shutdownCtx)
	}()

	// Create a test span
	_, span := ts.StartSpan(ctx, "test-operation")
	span.End()

	// Give Jaeger time to receive and process the span
	time.Sleep(3 * time.Second)

	// Query Jaeger API for our traces
	traces, err := queryJaegerTraces("station-test")
	require.NoError(t, err, "Should be able to query Jaeger API")

	// Verify we got at least one trace
	assert.Greater(t, len(traces), 0, "Should have at least one trace in Jaeger")

	// Verify the trace contains our test operation
	found := false
	for _, trace := range traces {
		for _, span := range trace.Spans {
			if span.OperationName == "test-operation" {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Should find 'test-operation' span in Jaeger traces")
}

// TestOTELSpanHierarchy tests that parent-child span relationships work
func TestOTELSpanHierarchy(t *testing.T) {
	if !isJaegerAvailable() {
		t.Skip("Jaeger not running")
	}

	ctx := context.Background()

	config := &TelemetryConfig{
		Enabled:      true,
		OTLPEndpoint: "localhost:4318",
		ServiceName:  "station-hierarchy-test",
		Environment:  "test",
	}

	ts := NewTelemetryService(config)
	err := ts.Initialize(ctx)
	require.NoError(t, err)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ts.Shutdown(shutdownCtx)
	}()

	// Create parent span
	parentCtx, parentSpan := ts.StartSpan(ctx, "parent-operation")

	// Create child span
	_, childSpan := ts.StartSpan(parentCtx, "child-operation")
	childSpan.End()

	parentSpan.End()

	// Wait for export
	time.Sleep(3 * time.Second)

	// Query traces
	traces, err := queryJaegerTraces("station-hierarchy-test")
	require.NoError(t, err)
	assert.Greater(t, len(traces), 0)

	// Verify both spans exist
	var foundParent, foundChild bool
	for _, trace := range traces {
		for _, span := range trace.Spans {
			if span.OperationName == "parent-operation" {
				foundParent = true
			}
			if span.OperationName == "child-operation" {
				foundChild = true
			}
		}
	}

	assert.True(t, foundParent, "Should find parent span")
	assert.True(t, foundChild, "Should find child span")
}

// Helper function to check if Jaeger is running
func isJaegerAvailable() bool {
	resp, err := http.Get("http://localhost:16686")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// JaegerTrace represents a trace from Jaeger API
type JaegerTrace struct {
	TraceID string        `json:"traceID"`
	Spans   []JaegerSpan  `json:"spans"`
}

// JaegerSpan represents a span from Jaeger API
type JaegerSpan struct {
	TraceID       string `json:"traceID"`
	SpanID        string `json:"spanID"`
	OperationName string `json:"operationName"`
}

// JaegerResponse represents Jaeger API response
type JaegerResponse struct {
	Data []JaegerTrace `json:"data"`
}

// Helper function to query Jaeger for traces
func queryJaegerTraces(serviceName string) ([]JaegerTrace, error) {
	url := fmt.Sprintf("http://localhost:16686/api/traces?service=%s&limit=20", serviceName)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Jaeger API returned status %d", resp.StatusCode)
	}

	var jaegerResp JaegerResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		return nil, fmt.Errorf("failed to decode Jaeger response: %v", err)
	}

	return jaegerResp.Data, nil
}
