package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// registerAgentRunRoutes registers agent run routes
func (h *APIHandlers) registerAgentRunRoutes(group *gin.RouterGroup) {
	group.GET("", h.listRuns)                        // Users can list runs
	group.GET("/:id", h.getRun)                      // Users can get run details
	group.GET("/:id/logs", h.getRunLogs)             // Users can get run debug logs
	group.GET("/agent/:agent_id", h.listRunsByAgent) // Users can list runs by agent
	group.DELETE("/:id", h.deleteRun)                // Delete a single run
	group.DELETE("", h.deleteRuns)                   // Delete runs (bulk by IDs, by status, or all)
}

// Agent runs handlers

func (h *APIHandlers) listRuns(c *gin.Context) {
	// Get limit parameter, default to 50
	limit := int64(50)
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get optional status filter
	status := c.Query("status")

	var runs interface{}
	var totalCount int64
	var err error

	if status != "" {
		// Validate status
		validStatuses := map[string]bool{"completed": true, "running": true, "failed": true, "pending": true}
		if !validStatuses[status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be one of: completed, running, failed, pending"})
			return
		}
		runs, err = h.repos.AgentRuns.ListRecentByStatus(c.Request.Context(), status, limit)
		if err == nil {
			totalCount, _ = h.repos.AgentRuns.CountByStatus(c.Request.Context(), status)
		}
	} else {
		runs, err = h.repos.AgentRuns.ListRecent(c.Request.Context(), limit)
		if err == nil {
			totalCount, _ = h.repos.AgentRuns.Count(c.Request.Context())
		}
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runs"})
		return
	}

	// Get count from runs slice
	var count int
	switch r := runs.(type) {
	case []*interface{}:
		count = len(r)
	default:
		// Use reflection-free approach - runs is always a slice
		if runsSlice, ok := runs.([]*struct{}); ok {
			count = len(runsSlice)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":        runs,
		"count":       count,
		"total_count": totalCount,
		"limit":       limit,
		"status":      status,
	})
}

func (h *APIHandlers) getRun(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	run, err := h.repos.AgentRuns.GetByIDWithDetails(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Run not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run})
}

func (h *APIHandlers) listRunsByAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("agent_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	runs, err := h.repos.AgentRuns.ListByAgent(c.Request.Context(), agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runs for agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":     runs,
		"count":    len(runs),
		"agent_id": agentID,
	})
}

func (h *APIHandlers) getRunLogs(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	run, err := h.repos.AgentRuns.GetByIDWithDetails(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Run not found"})
		return
	}

	// Extract debug logs
	debugLogs := []interface{}{}
	if run.DebugLogs != nil {
		debugLogs = *run.DebugLogs
	}

	// Apply filters if provided
	level := c.Query("level")
	if level != "" {
		filtered := []interface{}{}
		for _, logEntry := range debugLogs {
			if logMap, ok := logEntry.(map[string]interface{}); ok {
				if logLevel, exists := logMap["level"]; exists && logLevel == level {
					filtered = append(filtered, logEntry)
				}
			}
		}
		debugLogs = filtered
	}

	// Limit results if requested
	limit := c.Query("limit")
	if limit != "" {
		if limitNum, err := strconv.Atoi(limit); err == nil && limitNum > 0 && limitNum < len(debugLogs) {
			debugLogs = debugLogs[:limitNum]
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"run_id":     runID,
		"logs":       debugLogs,
		"log_count":  len(debugLogs),
		"agent_name": run.AgentName,
		"status":     run.Status,
		"started_at": run.StartedAt,
	})
}

// deleteRun deletes a single run by ID
func (h *APIHandlers) deleteRun(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	// Check if run exists
	_, err = h.repos.AgentRuns.GetByID(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Run not found"})
		return
	}

	err = h.repos.AgentRuns.Delete(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Run deleted successfully",
		"run_id":  runID,
	})
}

// DeleteRunsRequest represents the request body for bulk delete operations
type DeleteRunsRequest struct {
	IDs    []int64 `json:"ids"`    // Specific IDs to delete
	Status string  `json:"status"` // Delete all runs with this status
	All    bool    `json:"all"`    // Delete all runs
}

// deleteRuns handles bulk delete operations
func (h *APIHandlers) deleteRuns(c *gin.Context) {
	var req DeleteRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var deletedCount int64
	var err error

	// Priority: All > Status > IDs
	if req.All {
		// Get count before deletion
		deletedCount, _ = h.repos.AgentRuns.Count(c.Request.Context())
		err = h.repos.AgentRuns.DeleteAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete all runs"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       "All runs deleted successfully",
			"deleted_count": deletedCount,
		})
		return
	}

	if req.Status != "" {
		// Validate status
		validStatuses := map[string]bool{"completed": true, "running": true, "failed": true, "pending": true}
		if !validStatuses[req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be one of: completed, running, failed, pending"})
			return
		}

		// Get count before deletion
		deletedCount, _ = h.repos.AgentRuns.CountByStatus(c.Request.Context(), req.Status)
		err = h.repos.AgentRuns.DeleteByStatus(c.Request.Context(), req.Status)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete runs by status"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       "Runs deleted successfully",
			"status":        req.Status,
			"deleted_count": deletedCount,
		})
		return
	}

	if len(req.IDs) > 0 {
		// Filter out invalid IDs by checking existence
		validIDs := make([]int64, 0, len(req.IDs))
		for _, id := range req.IDs {
			if _, err := h.repos.AgentRuns.GetByID(c.Request.Context(), id); err == nil {
				validIDs = append(validIDs, id)
			}
		}

		if len(validIDs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "No valid runs found with provided IDs"})
			return
		}

		err = h.repos.AgentRuns.DeleteByIDs(c.Request.Context(), validIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete runs"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"message":       "Runs deleted successfully",
			"deleted_count": len(validIDs),
			"requested":     len(req.IDs),
		})
		return
	}

	// No valid delete criteria provided
	c.JSON(http.StatusBadRequest, gin.H{"error": "Must provide 'ids', 'status', or 'all' parameter"})
}
