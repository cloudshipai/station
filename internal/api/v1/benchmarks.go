package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"station/internal/config"
	pkgbenchmark "station/pkg/benchmark"
)

// registerBenchmarkRoutes registers benchmark evaluation routes
func (h *APIHandlers) registerBenchmarkRoutes(group *gin.RouterGroup) {
	group.POST("/:run_id/evaluate", h.evaluateRun) // Evaluate a run with LLM-as-judge metrics
	group.GET("/:run_id/metrics", h.getRunMetrics) // Get benchmark metrics for a run
	group.GET("/tasks", h.listBenchmarkTasks)      // List available benchmark tasks
	group.GET("/metrics", h.listRecentBenchmarks)  // List recent benchmark results
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

	analyzer := pkgbenchmark.NewAnalyzer(h.repos.BeginTx, judge)

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
