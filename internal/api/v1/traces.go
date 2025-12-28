package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"station/internal/config"

	"github.com/gin-gonic/gin"
)

// JaegerSpan represents a span from Jaeger's trace response
type JaegerSpan struct {
	TraceID       string            `json:"traceID"`
	SpanID        string            `json:"spanID"`
	OperationName string            `json:"operationName"`
	References    []JaegerReference `json:"references"`
	StartTime     int64             `json:"startTime"` // microseconds
	Duration      int64             `json:"duration"`  // microseconds
	Tags          []JaegerTag       `json:"tags"`
	Logs          []JaegerLog       `json:"logs"`
	ProcessID     string            `json:"processID"`
	Warnings      []string          `json:"warnings"`
}

type JaegerReference struct {
	RefType string `json:"refType"` // CHILD_OF, FOLLOWS_FROM
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

type JaegerTag struct {
	Key   string      `json:"key"`
	Type  string      `json:"type"` // string, bool, int64, float64
	Value interface{} `json:"value"`
}

type JaegerLog struct {
	Timestamp int64       `json:"timestamp"` // microseconds
	Fields    []JaegerTag `json:"fields"`
}

type JaegerProcess struct {
	ServiceName string      `json:"serviceName"`
	Tags        []JaegerTag `json:"tags"`
}

type JaegerTrace struct {
	TraceID   string                   `json:"traceID"`
	Spans     []JaegerSpan             `json:"spans"`
	Processes map[string]JaegerProcess `json:"processes"`
	Warnings  []string                 `json:"warnings"`
}

type JaegerTraceResponse struct {
	Data   []JaegerTrace `json:"data"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	Errors []string      `json:"errors"`
}

// registerTracesRoutes registers trace query routes
func (h *APIHandlers) registerTracesRoutes(router *gin.RouterGroup) {
	tracesGroup := router.Group("/traces")
	tracesGroup.GET("/run/:run_id", h.getTraceByRunID)
	tracesGroup.GET("/workflow-run/:run_id", h.getTraceByWorkflowRunID)
	tracesGroup.GET("/trace/:trace_id", h.getTraceByTraceID)
}

// getTraceByRunID fetches traces for a specific agent run from Jaeger
func (h *APIHandlers) getTraceByRunID(c *gin.Context) {
	runIDStr := c.Param("run_id")
	runID, err := strconv.ParseInt(runIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	// Query Jaeger using run.id tag
	// No need to fetch run from DB, we'll query Jaeger directly
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	// Query Jaeger by tag: run.id=<run_id>
	// Note: Use tag=key:value format, not tags={json} format
	jaegerURL := fmt.Sprintf("%s/api/traces?service=station&tag=run.id:%d&limit=10",
		cfg.JaegerQueryURL, runID)

	resp, err := http.Get(jaegerURL)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "Failed to query Jaeger",
			"details":    err.Error(),
			"jaeger_url": cfg.JaegerQueryURL,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{
			"error":  "Jaeger query failed",
			"status": resp.StatusCode,
			"body":   string(body),
		})
		return
	}

	// Parse Jaeger response
	var jaegerResp JaegerTraceResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse Jaeger response",
			"details": err.Error(),
		})
		return
	}

	// Return trace data
	if len(jaegerResp.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "No traces found for this run",
			"run_id":     runID,
			"suggestion": "Traces may not be available yet or telemetry is disabled",
		})
		return
	}

	// Find the trace with the most spans (this is typically the main execution trace)
	mainTrace := jaegerResp.Data[0]
	for _, trace := range jaegerResp.Data {
		if len(trace.Spans) > len(mainTrace.Spans) {
			mainTrace = trace
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"run_id": runID,
		"trace":  mainTrace,
	})
}

// getTraceByWorkflowRunID fetches traces for a specific workflow run from Jaeger
func (h *APIHandlers) getTraceByWorkflowRunID(c *gin.Context) {
	runID := c.Param("run_id")
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow run ID"})
		return
	}

	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	// Query Jaeger by tag: workflow.run_id=<run_id>
	// Note: Use tag=key:value format, not tags={json} format
	jaegerURL := fmt.Sprintf("%s/api/traces?service=station&tag=workflow.run_id:%s&limit=10",
		cfg.JaegerQueryURL, runID)

	resp, err := http.Get(jaegerURL)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":      "Failed to query Jaeger",
			"details":    err.Error(),
			"jaeger_url": cfg.JaegerQueryURL,
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{
			"error":  "Jaeger query failed",
			"status": resp.StatusCode,
			"body":   string(body),
		})
		return
	}

	// Parse Jaeger response
	var jaegerResp JaegerTraceResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse Jaeger response",
			"details": err.Error(),
		})
		return
	}

	// Return trace data
	if len(jaegerResp.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":      "No traces found for this workflow run",
			"run_id":     runID,
			"suggestion": "Traces may not be available yet or telemetry is disabled",
		})
		return
	}

	// Find the trace with the most spans (this is typically the main execution trace)
	mainTrace := jaegerResp.Data[0]
	for _, trace := range jaegerResp.Data {
		if len(trace.Spans) > len(mainTrace.Spans) {
			mainTrace = trace
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"run_id": runID,
		"trace":  mainTrace,
	})
}

// getTraceByTraceID fetches a specific trace from Jaeger by trace ID
func (h *APIHandlers) getTraceByTraceID(c *gin.Context) {
	traceID := c.Param("trace_id")

	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	// Query Jaeger directly by trace ID
	jaegerURL := fmt.Sprintf("%s/api/traces/%s", cfg.JaegerQueryURL, traceID)

	resp, err := http.Get(jaegerURL)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Failed to query Jaeger",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.JSON(resp.StatusCode, gin.H{
			"error":  "Jaeger query failed",
			"status": resp.StatusCode,
			"body":   string(body),
		})
		return
	}

	// Parse Jaeger response
	var jaegerResp JaegerTraceResponse
	if err := json.NewDecoder(resp.Body).Decode(&jaegerResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to parse Jaeger response",
			"details": err.Error(),
		})
		return
	}

	if len(jaegerResp.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error":    "Trace not found",
			"trace_id": traceID,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trace_id": traceID,
		"trace":    jaegerResp.Data[0],
	})
}
