package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"station/internal/config"
	pkgbenchmark "station/pkg/benchmark"
)

// registerBenchmarkRoutes registers benchmark evaluation routes
func (h *APIHandlers) registerBenchmarkRoutes(group *gin.RouterGroup) {
	group.POST("/:run_id/evaluate", h.evaluateRun)        // Evaluate a run with LLM-as-judge metrics
	group.GET("/:run_id/metrics", h.getRunMetrics)        // Get benchmark metrics for a run
	group.GET("/tasks", h.listBenchmarkTasks)             // List available benchmark tasks
	group.GET("/metrics", h.listRecentBenchmarks)         // List recent benchmark results
	group.POST("/evaluate-bulk", h.evaluateBulk)          // Evaluate multiple runs in parallel
	group.GET("/evaluate-bulk/stream", h.evaluateBulkSSE) // Stream bulk evaluation progress
}

// Benchmark handlers

// evaluateRun evaluates a run using LLM-as-judge metrics
// POST /api/v1/benchmarks/:run_id/evaluate
func (h *APIHandlers) evaluateRun(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	// Verify run exists
	_, err = h.repos.AgentRuns.GetByIDWithDetails(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Run not found"})
		return
	}

	// Load config to create judge
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configuration"})
		return
	}

	// Create judge and analyzer
	judge, err := pkgbenchmark.NewJudge(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create LLM judge", "details": err.Error()})
		return
	}

	analyzer := pkgbenchmark.NewAnalyzer(h.db, judge)

	// Evaluate the run
	result, err := analyzer.EvaluateRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Evaluation failed", "details": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"run_id":             result.RunID,
		"agent_id":           result.AgentID,
		"task":               result.Task,
		"quality_score":      result.QualityScore,
		"production_ready":   result.ProductionReady,
		"recommendation":     result.Recommendation,
		"metrics":            result.Metrics,
		"total_judge_tokens": result.TotalJudgeTokens,
		"total_judge_cost":   result.TotalJudgeCost,
		"evaluation_time_ms": result.EvaluationTimeMS,
	})
}

// getRunMetrics retrieves benchmark metrics for a specific run
// GET /api/v1/benchmarks/:run_id/metrics
func (h *APIHandlers) getRunMetrics(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("run_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	metrics, err := h.repos.BenchmarkMetrics.GetByRunID(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve metrics"})
		return
	}

	if len(metrics) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No benchmark metrics found for this run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"run_id":  runID,
		"metrics": metrics,
		"count":   len(metrics),
	})
}

// listBenchmarkTasks lists all available benchmark tasks
// GET /api/v1/benchmarks/tasks
func (h *APIHandlers) listBenchmarkTasks(c *gin.Context) {
	tasks, err := h.repos.BenchmarkTasks.GetAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list benchmark tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"count": len(tasks),
	})
}

// listRecentBenchmarks lists recent benchmark results across all runs
// GET /api/v1/benchmarks/metrics?limit=50
func (h *APIHandlers) listRecentBenchmarks(c *gin.Context) {
	// Get limit parameter, default to 50
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	metrics, err := h.repos.BenchmarkMetrics.ListRecent(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list benchmark metrics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics": metrics,
		"count":   len(metrics),
		"limit":   limit,
	})
}

// BulkEvaluationRequest represents a request to evaluate multiple runs
type BulkEvaluationRequest struct {
	RunIDs      []int64 `json:"run_ids"`
	Concurrency int     `json:"concurrency"` // Max parallel evaluations, default 10
}

// BulkEvaluationProgress represents progress of a bulk evaluation
type BulkEvaluationProgress struct {
	RunID           int64   `json:"run_id"`
	Status          string  `json:"status"` // pending, evaluating, completed, failed
	QualityScore    float64 `json:"quality_score,omitempty"`
	ProductionReady bool    `json:"production_ready,omitempty"`
	Error           string  `json:"error,omitempty"`
	EvaluatedAt     string  `json:"evaluated_at,omitempty"`
}

// BulkEvaluationResult represents the final result of bulk evaluation
type BulkEvaluationResult struct {
	TotalRuns       int                      `json:"total_runs"`
	Completed       int                      `json:"completed"`
	Failed          int                      `json:"failed"`
	DurationSeconds float64                  `json:"duration_seconds"`
	Results         []BulkEvaluationProgress `json:"results"`
	Summary         BulkEvaluationSummary    `json:"summary"`
}

// BulkEvaluationSummary provides aggregate statistics
type BulkEvaluationSummary struct {
	AvgQualityScore    float64 `json:"avg_quality_score"`
	ProductionReadyPct float64 `json:"production_ready_pct"`
	TotalJudgeTokens   int     `json:"total_judge_tokens"`
	TotalJudgeCost     float64 `json:"total_judge_cost"`
}

// evaluateBulk evaluates multiple runs in parallel using goroutines
// POST /api/v1/benchmarks/evaluate-bulk
// Body: {"run_ids": [1,2,3], "concurrency": 10}
func (h *APIHandlers) evaluateBulk(c *gin.Context) {
	var req BulkEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if len(req.RunIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No run IDs provided"})
		return
	}

	// Default concurrency to 10
	if req.Concurrency <= 0 {
		req.Concurrency = 10
	}

	// Load config and create judge
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load configuration"})
		return
	}

	judge, err := pkgbenchmark.NewJudge(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create LLM judge", "details": err.Error()})
		return
	}

	analyzer := pkgbenchmark.NewAnalyzer(h.db, judge)

	// Track start time
	startTime := time.Now()

	// Create result tracking
	results := make([]BulkEvaluationProgress, len(req.RunIDs))
	for i, runID := range req.RunIDs {
		results[i] = BulkEvaluationProgress{
			RunID:  runID,
			Status: "pending",
		}
	}

	// Use semaphore pattern for concurrency control
	sem := make(chan struct{}, req.Concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect results slice

	// Evaluate each run in parallel
	for i, runID := range req.RunIDs {
		wg.Add(1)
		go func(idx int, id int64) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Update status to evaluating
			mu.Lock()
			results[idx].Status = "evaluating"
			mu.Unlock()

			// Evaluate the run
			result, err := analyzer.EvaluateRun(context.Background(), id)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[idx].Status = "failed"
				results[idx].Error = err.Error()
			} else {
				results[idx].Status = "completed"
				results[idx].QualityScore = result.QualityScore
				results[idx].ProductionReady = result.ProductionReady
				results[idx].EvaluatedAt = time.Now().Format(time.RFC3339)
			}
		}(i, runID)
	}

	// Wait for all evaluations to complete
	wg.Wait()

	duration := time.Since(startTime).Seconds()

	// Calculate summary statistics
	completed := 0
	failed := 0
	totalScore := 0.0
	productionReady := 0

	for _, r := range results {
		if r.Status == "completed" {
			completed++
			totalScore += r.QualityScore
			if r.ProductionReady {
				productionReady++
			}
		} else if r.Status == "failed" {
			failed++
		}
	}

	avgScore := 0.0
	if completed > 0 {
		avgScore = totalScore / float64(completed)
	}

	productionReadyPct := 0.0
	if completed > 0 {
		productionReadyPct = float64(productionReady) / float64(completed) * 100
	}

	c.JSON(http.StatusOK, BulkEvaluationResult{
		TotalRuns:       len(req.RunIDs),
		Completed:       completed,
		Failed:          failed,
		DurationSeconds: duration,
		Results:         results,
		Summary: BulkEvaluationSummary{
			AvgQualityScore:    avgScore,
			ProductionReadyPct: productionReadyPct,
		},
	})
}

// evaluateBulkSSE streams bulk evaluation progress using Server-Sent Events
// GET /api/v1/benchmarks/evaluate-bulk/stream?run_ids=1,2,3&concurrency=10
func (h *APIHandlers) evaluateBulkSSE(c *gin.Context) {
	// Parse run IDs from query parameter
	runIDsStr := c.Query("run_ids")
	if runIDsStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No run_ids provided"})
		return
	}

	var runIDs []int64
	if err := json.Unmarshal([]byte("["+runIDsStr+"]"), &runIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run_ids format", "details": err.Error()})
		return
	}

	if len(runIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No run IDs provided"})
		return
	}

	// Get concurrency parameter
	concurrency := 10
	if concStr := c.Query("concurrency"); concStr != "" {
		if parsed, err := strconv.Atoi(concStr); err == nil && parsed > 0 {
			concurrency = parsed
		}
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Load config and create judge
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\": \"Failed to load configuration\"}\n\n")
		c.Writer.Flush()
		return
	}

	judge, err := pkgbenchmark.NewJudge(cfg)
	if err != nil {
		fmt.Fprintf(c.Writer, "event: error\ndata: {\"error\": \"Failed to create LLM judge\"}\n\n")
		c.Writer.Flush()
		return
	}

	analyzer := pkgbenchmark.NewAnalyzer(h.db, judge)

	// Progress channel
	progressChan := make(chan BulkEvaluationProgress, len(runIDs))

	// Use semaphore for concurrency control
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	// Evaluate each run in parallel
	for _, runID := range runIDs {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Send evaluating status
			progressChan <- BulkEvaluationProgress{
				RunID:  id,
				Status: "evaluating",
			}

			// Evaluate the run
			result, err := analyzer.EvaluateRun(context.Background(), id)

			if err != nil {
				progressChan <- BulkEvaluationProgress{
					RunID:  id,
					Status: "failed",
					Error:  err.Error(),
				}
			} else {
				progressChan <- BulkEvaluationProgress{
					RunID:           id,
					Status:          "completed",
					QualityScore:    result.QualityScore,
					ProductionReady: result.ProductionReady,
					EvaluatedAt:     time.Now().Format(time.RFC3339),
				}
			}
		}(runID)
	}

	// Close progress channel when all evaluations complete
	go func() {
		wg.Wait()
		close(progressChan)
	}()

	// Stream progress events
	for progress := range progressChan {
		data, _ := json.Marshal(progress)
		fmt.Fprintf(c.Writer, "event: progress\ndata: %s\n\n", string(data))
		c.Writer.Flush()
	}

	// Send completion event
	fmt.Fprintf(c.Writer, "event: complete\ndata: {\"status\": \"done\"}\n\n")
	c.Writer.Flush()
}
