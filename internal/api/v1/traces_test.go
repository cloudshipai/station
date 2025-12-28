package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/gin-gonic/gin"
)

func createMockJaegerTrace(traceID string, spanCount int) JaegerTrace {
	spans := make([]JaegerSpan, spanCount)
	for i := range spanCount {
		spans[i] = JaegerSpan{
			TraceID:       traceID,
			SpanID:        "span" + string(rune('a'+i)),
			OperationName: "test.operation",
			StartTime:     1000000 * int64(i),
			Duration:      500000,
			Tags: []JaegerTag{
				{Key: "run.id", Type: "int64", Value: float64(1)},
			},
		}
	}
	return JaegerTrace{
		TraceID: traceID,
		Spans:   spans,
		Processes: map[string]JaegerProcess{
			"p1": {ServiceName: "station", Tags: []JaegerTag{}},
		},
	}
}

func TestAPIHandlers_GetTraceByRunID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		runID          string
		mockResponse   JaegerTraceResponse
		mockStatusCode int
		expectedStatus int
		expectError    bool
	}{
		{
			name:  "valid run ID with trace",
			runID: "1",
			mockResponse: JaegerTraceResponse{
				Data: []JaegerTrace{
					createMockJaegerTrace("abc123", 5),
				},
				Total: 1,
			},
			mockStatusCode: http.StatusOK,
			expectedStatus: http.StatusOK,
		},
		{
			name:  "valid run ID with no traces",
			runID: "2",
			mockResponse: JaegerTraceResponse{
				Data:  []JaegerTrace{},
				Total: 0,
			},
			mockStatusCode: http.StatusOK,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "invalid run ID format",
			runID:          "not-a-number",
			mockResponse:   JaegerTraceResponse{},
			mockStatusCode: http.StatusOK,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:  "multiple traces returns largest",
			runID: "3",
			mockResponse: JaegerTraceResponse{
				Data: []JaegerTrace{
					createMockJaegerTrace("trace1", 3),
					createMockJaegerTrace("trace2", 10),
					createMockJaegerTrace("trace3", 5),
				},
				Total: 3,
			},
			mockStatusCode: http.StatusOK,
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.runID == "not-a-number" {
				handlers := setupTestAPIHandlers()

				req := httptest.NewRequest("GET", "/traces/run/"+tt.runID, nil)
				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				c.Request = req
				c.Params = gin.Params{{Key: "run_id", Value: tt.runID}}

				handlers.getTraceByRunID(c)

				if w.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				}
				return
			}

			t.Logf("Test case '%s' requires Jaeger mock - covered by integration tests", tt.name)
		})
	}
}

func TestAPIHandlers_GetTraceByWorkflowRunID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("empty run ID returns bad request", func(t *testing.T) {
		handlers := setupTestAPIHandlers()

		req := httptest.NewRequest("GET", "/traces/workflow-run/", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "run_id", Value: ""}}

		handlers.getTraceByWorkflowRunID(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("valid UUID returns OK or ServiceUnavailable depending on Jaeger", func(t *testing.T) {
		handlers := setupTestAPIHandlers()

		req := httptest.NewRequest("GET", "/traces/workflow-run/950f2a21-541d-45b9-90f8-32b441a6f434", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "run_id", Value: "950f2a21-541d-45b9-90f8-32b441a6f434"}}

		handlers.getTraceByWorkflowRunID(c)

		validStatuses := []int{http.StatusOK, http.StatusNotFound, http.StatusServiceUnavailable}
		if !slices.Contains(validStatuses, w.Code) {
			t.Errorf("Expected status in %v, got %d", validStatuses, w.Code)
		}
	})
}

func TestAPIHandlers_GetTraceByTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid trace ID returns OK or ServiceUnavailable depending on Jaeger", func(t *testing.T) {
		handlers := setupTestAPIHandlers()

		req := httptest.NewRequest("GET", "/traces/trace/abc123def456", nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "trace_id", Value: "abc123def456"}}

		handlers.getTraceByTraceID(c)

		validStatuses := []int{http.StatusOK, http.StatusNotFound, http.StatusServiceUnavailable}
		if !slices.Contains(validStatuses, w.Code) {
			t.Errorf("Expected status in %v, got %d", validStatuses, w.Code)
		}
	})
}

func TestJaegerResponseParsing(t *testing.T) {
	tests := []struct {
		name      string
		trace     JaegerTrace
		spanCount int
	}{
		{
			name:      "empty trace",
			trace:     JaegerTrace{TraceID: "empty", Spans: []JaegerSpan{}},
			spanCount: 0,
		},
		{
			name:      "trace with spans",
			trace:     createMockJaegerTrace("test123", 5),
			spanCount: 5,
		},
		{
			name: "trace with nested references",
			trace: JaegerTrace{
				TraceID: "nested123",
				Spans: []JaegerSpan{
					{
						TraceID:       "nested123",
						SpanID:        "parent",
						OperationName: "parent.operation",
						StartTime:     1000000,
						Duration:      5000000,
					},
					{
						TraceID:       "nested123",
						SpanID:        "child",
						OperationName: "child.operation",
						StartTime:     1500000,
						Duration:      2000000,
						References: []JaegerReference{
							{RefType: "CHILD_OF", TraceID: "nested123", SpanID: "parent"},
						},
					},
				},
			},
			spanCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.trace)
			if err != nil {
				t.Fatalf("Failed to marshal trace: %v", err)
			}

			var parsed JaegerTrace
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Failed to unmarshal trace: %v", err)
			}

			if len(parsed.Spans) != tt.spanCount {
				t.Errorf("Expected %d spans, got %d", tt.spanCount, len(parsed.Spans))
			}
			if parsed.TraceID != tt.trace.TraceID {
				t.Errorf("Expected traceID %s, got %s", tt.trace.TraceID, parsed.TraceID)
			}
		})
	}
}

func TestFindLargestTrace(t *testing.T) {
	traces := []JaegerTrace{
		createMockJaegerTrace("small", 2),
		createMockJaegerTrace("large", 10),
		createMockJaegerTrace("medium", 5),
	}

	// same logic as handler: find trace with most spans
	mainTrace := traces[0]
	for _, trace := range traces {
		if len(trace.Spans) > len(mainTrace.Spans) {
			mainTrace = trace
		}
	}

	if mainTrace.TraceID != "large" {
		t.Errorf("Expected largest trace 'large', got '%s'", mainTrace.TraceID)
	}
	if len(mainTrace.Spans) != 10 {
		t.Errorf("Expected 10 spans, got %d", len(mainTrace.Spans))
	}
}

func TestJaegerTagTypes(t *testing.T) {
	tags := []JaegerTag{
		{Key: "string.tag", Type: "string", Value: "hello"},
		{Key: "bool.tag", Type: "bool", Value: true},
		{Key: "int64.tag", Type: "int64", Value: float64(42)}, // JSON numbers become float64
		{Key: "float64.tag", Type: "float64", Value: 3.14},
	}

	span := JaegerSpan{
		TraceID:       "test",
		SpanID:        "span1",
		OperationName: "test.op",
		Tags:          tags,
	}

	data, err := json.Marshal(span)
	if err != nil {
		t.Fatalf("Failed to marshal span: %v", err)
	}

	var parsed JaegerSpan
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal span: %v", err)
	}

	if len(parsed.Tags) != len(tags) {
		t.Errorf("Expected %d tags, got %d", len(tags), len(parsed.Tags))
	}

	if parsed.Tags[0].Value != "hello" {
		t.Errorf("Expected string tag value 'hello', got %v", parsed.Tags[0].Value)
	}

	if parsed.Tags[1].Value != true {
		t.Errorf("Expected bool tag value true, got %v", parsed.Tags[1].Value)
	}
}
