package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"station/internal/services"
	"station/internal/workflows"
)

type startWorkflowRunRequest struct {
	WorkflowID    string          `json:"workflowId" binding:"required"`
	Version       *int64          `json:"version,omitempty"`
	EnvironmentID *int64          `json:"environmentId,omitempty"`
	Input         json.RawMessage `json:"input"`
	Options       json.RawMessage `json:"options"`
}

type cancelWorkflowRunRequest struct {
	Reason string `json:"reason"`
}

type signalWorkflowRunRequest struct {
	Name    string          `json:"name" binding:"required"`
	Payload json.RawMessage `json:"payload"`
}

type pauseWorkflowRunRequest struct {
	Reason string `json:"reason"`
}

type completeWorkflowRunRequest struct {
	Result  json.RawMessage `json:"result"`
	Summary string          `json:"summary"`
}

type approveWorkflowStepRequest struct {
	Comment string `json:"comment"`
}

type rejectWorkflowStepRequest struct {
	Reason string `json:"reason"`
}

type deleteWorkflowRunsRequest struct {
	RunIDs     []string `json:"runIds"`
	WorkflowID string   `json:"workflowId"`
	Status     string   `json:"status"`
	All        bool     `json:"all"`
}

func (h *APIHandlers) registerWorkflowRunRoutes(group *gin.RouterGroup) {
	group.POST("", h.startWorkflowRun)
	group.GET("", h.listWorkflowRuns)
	group.DELETE("", h.deleteWorkflowRuns)
	group.GET("/:runId", h.getWorkflowRun)
	group.GET("/:runId/stream", h.streamWorkflowRun)
	group.GET("/:runId/steps", h.listWorkflowRunSteps)
	group.GET("/:runId/approvals", h.listWorkflowRunApprovals)
	group.POST("/:runId/cancel", h.cancelWorkflowRun)
	group.POST("/:runId/signal", h.signalWorkflowRun)
	group.POST("/:runId/pause", h.pauseWorkflowRun)
	group.POST("/:runId/resume", h.resumeWorkflowRun)
	group.POST("/:runId/complete", h.completeWorkflowRun)
}

func (h *APIHandlers) registerWorkflowApprovalRoutes(group *gin.RouterGroup) {
	group.GET("", h.listPendingApprovals)
	group.GET("/:approvalId", h.getApproval)
	group.POST("/:approvalId/approve", h.approveWorkflowStep)
	group.POST("/:approvalId/reject", h.rejectWorkflowStep)
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

	environmentID := int64(0)
	if req.EnvironmentID != nil {
		environmentID = *req.EnvironmentID
	}

	run, validation, err := h.workflowService.StartRun(c.Request.Context(), services.StartWorkflowRunRequest{
		WorkflowID:    req.WorkflowID,
		Version:       version,
		EnvironmentID: environmentID,
		Input:         req.Input,
		Options:       req.Options,
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

func (h *APIHandlers) startWorkflowRunNested(c *gin.Context) {
	workflowID := c.Param("workflowId")

	var req struct {
		Version       *int64          `json:"version,omitempty"`
		EnvironmentID *int64          `json:"environmentId,omitempty"`
		Input         json.RawMessage `json:"input"`
		Options       json.RawMessage `json:"options"`
	}
	_ = c.ShouldBindJSON(&req)

	version := int64(0)
	if req.Version != nil {
		version = *req.Version
	}

	environmentID := int64(0)
	if req.EnvironmentID != nil {
		environmentID = *req.EnvironmentID
	}

	run, validation, err := h.workflowService.StartRun(c.Request.Context(), services.StartWorkflowRunRequest{
		WorkflowID:    workflowID,
		Version:       version,
		EnvironmentID: environmentID,
		Input:         req.Input,
		Options:       req.Options,
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
		"run":     run,
		"message": "Workflow run started",
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
	workflowID := c.Query("workflow_id")
	if workflowID == "" {
		workflowID = c.Query("workflowId")
	}
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

func (h *APIHandlers) deleteWorkflowRuns(c *gin.Context) {
	var req deleteWorkflowRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid delete payload"})
		return
	}

	if !req.All && len(req.RunIDs) == 0 && req.WorkflowID == "" && req.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Must specify runIds, workflowId, status, or all=true"})
		return
	}

	deleted, err := h.workflowService.DeleteRuns(c.Request.Context(), services.DeleteRunsRequest{
		RunIDs:     req.RunIDs,
		WorkflowID: req.WorkflowID,
		Status:     req.Status,
		All:        req.All,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete workflow runs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deleted": deleted,
		"message": "Workflow runs deleted",
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

func (h *APIHandlers) pauseWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")
	var req pauseWorkflowRunRequest
	_ = c.ShouldBindJSON(&req)

	run, err := h.workflowService.PauseRun(c.Request.Context(), runID, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pause workflow run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run, "message": "Workflow run paused"})
}

func (h *APIHandlers) resumeWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")
	var req signalWorkflowRunRequest
	_ = c.ShouldBindJSON(&req)

	run, err := h.workflowService.ResumeRun(c.Request.Context(), runID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resume workflow run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run, "message": "Workflow run resumed"})
}

func (h *APIHandlers) completeWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")
	var req completeWorkflowRunRequest
	_ = c.ShouldBindJSON(&req)

	run, err := h.workflowService.CompleteRun(c.Request.Context(), runID, req.Result, req.Summary)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete workflow run"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run, "message": "Workflow run completed"})
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

func (h *APIHandlers) listWorkflowRunApprovals(c *gin.Context) {
	runID := c.Param("runId")
	approvals, err := h.workflowService.ListApprovalsByRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list approvals"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"approvals": approvals,
		"count":     len(approvals),
	})
}

func (h *APIHandlers) listPendingApprovals(c *gin.Context) {
	runID := c.Query("run_id")
	if runID == "" {
		runID = c.Query("runId")
	}
	limit := int64(50)
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	approvals, err := h.workflowService.ListPendingApprovals(c.Request.Context(), runID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pending approvals"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"approvals": approvals,
		"count":     len(approvals),
		"limit":     limit,
	})
}

func (h *APIHandlers) getApproval(c *gin.Context) {
	approvalID := c.Param("approvalId")
	approval, err := h.workflowService.GetApproval(c.Request.Context(), approvalID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Approval not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"approval": approval})
}

func (h *APIHandlers) approveWorkflowStep(c *gin.Context) {
	approvalID := c.Param("approvalId")
	approverID := c.GetHeader("X-Approver-ID")
	if approverID == "" {
		approverID = "api-user"
	}

	var req approveWorkflowStepRequest
	_ = c.ShouldBindJSON(&req)

	approval, err := h.workflowService.ApproveWorkflowStep(c.Request.Context(), services.ApproveWorkflowStepRequest{
		ApprovalID: approvalID,
		ApproverID: approverID,
		Comment:    req.Comment,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"approval": approval,
		"message":  "Workflow step approved",
	})
}

func (h *APIHandlers) rejectWorkflowStep(c *gin.Context) {
	approvalID := c.Param("approvalId")
	rejecterID := c.GetHeader("X-Approver-ID")
	if rejecterID == "" {
		rejecterID = "api-user"
	}

	var req rejectWorkflowStepRequest
	_ = c.ShouldBindJSON(&req)

	approval, err := h.workflowService.RejectWorkflowStep(c.Request.Context(), services.RejectWorkflowStepRequest{
		ApprovalID: approvalID,
		RejecterID: rejecterID,
		Reason:     req.Reason,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"approval": approval,
		"message":  "Workflow step rejected",
	})
}

func (h *APIHandlers) streamWorkflowRun(c *gin.Context) {
	runID := c.Param("runId")

	run, err := h.workflowService.GetRun(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow run not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	clientGone := c.Request.Context().Done()

	runData, _ := json.Marshal(gin.H{"type": "run_update", "run": run})
	c.SSEvent("message", string(runData))
	c.Writer.Flush()

	steps, _ := h.workflowService.ListSteps(c.Request.Context(), runID)
	for _, step := range steps {
		stepData, _ := json.Marshal(gin.H{"type": "step_update", "step": step})
		c.SSEvent("message", string(stepData))
	}
	c.Writer.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastRunStatus := run.Status
	lastStepCount := len(steps)

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			currentRun, err := h.workflowService.GetRun(c.Request.Context(), runID)
			if err != nil {
				continue
			}

			stepChanged := (currentRun.CurrentStep == nil) != (run.CurrentStep == nil) ||
				(currentRun.CurrentStep != nil && run.CurrentStep != nil && *currentRun.CurrentStep != *run.CurrentStep)

			if currentRun.Status != lastRunStatus || stepChanged {
				runData, _ := json.Marshal(gin.H{"type": "run_update", "run": currentRun})
				c.SSEvent("message", string(runData))
				c.Writer.Flush()
				lastRunStatus = currentRun.Status
				run = currentRun
			}

			currentSteps, _ := h.workflowService.ListSteps(c.Request.Context(), runID)
			if len(currentSteps) != lastStepCount {
				for _, step := range currentSteps[lastStepCount:] {
					stepData, _ := json.Marshal(gin.H{"type": "step_update", "step": step})
					c.SSEvent("message", string(stepData))
				}
				c.Writer.Flush()
				lastStepCount = len(currentSteps)
			}

			if currentRun.Status == "completed" || currentRun.Status == "failed" || currentRun.Status == "cancelled" {
				return
			}
		}
	}
}
