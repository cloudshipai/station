package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"station/internal/services"
	"station/internal/workflows"
)

type startWorkflowRunRequest struct {
	WorkflowID string          `json:"workflowId" binding:"required"`
	Version    *int64          `json:"version,omitempty"`
	Input      json.RawMessage `json:"input"`
	Options    json.RawMessage `json:"options"`
}

type cancelWorkflowRunRequest struct {
	Reason string `json:"reason"`
}

type signalWorkflowRunRequest struct {
	Name    string          `json:"name" binding:"required"`
	Payload json.RawMessage `json:"payload"`
}

// registerWorkflowRunRoutes wires workflow run operations.
func (h *APIHandlers) registerWorkflowRunRoutes(group *gin.RouterGroup) {
	group.POST("", h.startWorkflowRun)
	group.GET("", h.listWorkflowRuns)
	group.GET("/:runId", h.getWorkflowRun)
	group.GET("/:runId/steps", h.listWorkflowRunSteps)
	group.POST("/:runId/cancel", h.cancelWorkflowRun)
	group.POST("/:runId/signal", h.signalWorkflowRun)
}

func (h *APIHandlers) startWorkflowRun(c *gin.Context) {
	var req startWorkflowRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow run payload"})
		return
	}

	version := int64(0)
	if req.Version != nil {
		version = *req.Version
	}

	run, validation, err := h.workflowService.StartRun(c.Request.Context(), services.StartWorkflowRunRequest{
		WorkflowID: req.WorkflowID,
		Version:    version,
		Input:      req.Input,
		Options:    req.Options,
	})
	if errors.Is(err, workflows.ErrValidation) {
		c.JSON(http.StatusBadRequest, gin.H{
			"validation": validation,
			"message":    "Workflow definition failed validation for execution",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start workflow run"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"run":        run,
		"validation": validation,
	})
}

func (h *APIHandlers) getWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")
	run, err := h.workflowService.GetRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow run not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run})
}

func (h *APIHandlers) listWorkflowRuns(c *gin.Context) {
	workflowID := c.Query("workflowId")
	status := c.Query("status")

	limit := int64(50)
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	runs, err := h.workflowService.ListRuns(c.Request.Context(), workflowID, status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workflow runs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":    runs,
		"count":   len(runs),
		"limit":   limit,
		"filters": gin.H{"workflowId": workflowID, "status": status},
	})
}

func (h *APIHandlers) cancelWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")
	var req cancelWorkflowRunRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" { // empty body allowed
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid cancel payload"})
		return
	}

	run, err := h.workflowService.CancelRun(c.Request.Context(), runID, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel workflow run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"run":     run,
		"message": "Workflow run canceled",
	})
}

func (h *APIHandlers) signalWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")
	var req signalWorkflowRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signal payload"})
		return
	}

	run, err := h.workflowService.SignalRun(c.Request.Context(), services.SignalWorkflowRunRequest{
		RunID:   runID,
		Name:    req.Name,
		Payload: req.Payload,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to signal workflow run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"run":     run,
		"message": "Signal recorded",
	})
}

func (h *APIHandlers) listWorkflowRunSteps(c *gin.Context) {
	runID := c.Param("runId")
	steps, err := h.workflowService.ListSteps(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workflow run steps"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"steps": steps,
		"count": len(steps),
	})
}
