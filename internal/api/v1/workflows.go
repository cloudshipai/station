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

type workflowDefinitionRequest struct {
	WorkflowID  string          `json:"workflowId" binding:"required"`
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition" binding:"required"`
}

// registerWorkflowRoutes wires workflow definition CRUD endpoints.
func (h *APIHandlers) registerWorkflowRoutes(group *gin.RouterGroup) {
	group.POST("", h.createWorkflow)
	group.POST("/validate", h.validateWorkflow)
	group.GET("", h.listWorkflows)
	group.GET("/:workflowId", h.getWorkflow)
	group.GET("/:workflowId/versions", h.listWorkflowVersions)
	group.GET("/:workflowId/versions/:version", h.getWorkflowVersion)
	group.PUT("/:workflowId", h.updateWorkflow)
	group.DELETE("/:workflowId", h.disableWorkflow)
}

func (h *APIHandlers) createWorkflow(c *gin.Context) {
	var req workflowDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow payload"})
		return
	}
	if len(req.Definition) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "definition is required"})
		return
	}

	record, validation, err := h.workflowService.CreateWorkflow(c.Request.Context(), services.WorkflowDefinitionInput{
		WorkflowID:  req.WorkflowID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
	})
	if errors.Is(err, workflows.ErrValidation) {
		c.JSON(http.StatusBadRequest, gin.H{
			"validation": validation,
			"message":    "Workflow definition failed validation",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create workflow"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"workflow":   record,
		"validation": validation,
	})
}

func (h *APIHandlers) validateWorkflow(c *gin.Context) {
	var req workflowDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow payload"})
		return
	}
	_, validation, err := h.workflowService.ValidateDefinition(req.Definition)
	status := http.StatusOK
	if errors.Is(err, workflows.ErrValidation) {
		status = http.StatusBadRequest
	}

	c.JSON(status, gin.H{"validation": validation})
}

func (h *APIHandlers) updateWorkflow(c *gin.Context) {
	workflowID := c.Param("workflowId")
	var req workflowDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow payload"})
		return
	}
	req.WorkflowID = workflowID

	record, validation, err := h.workflowService.UpdateWorkflow(c.Request.Context(), services.WorkflowDefinitionInput{
		WorkflowID:  workflowID,
		Name:        req.Name,
		Description: req.Description,
		Definition:  req.Definition,
	})
	if errors.Is(err, workflows.ErrValidation) {
		c.JSON(http.StatusBadRequest, gin.H{
			"validation": validation,
			"message":    "Workflow definition failed validation",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update workflow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflow":   record,
		"validation": validation,
	})
}

func (h *APIHandlers) listWorkflows(c *gin.Context) {
	workflows, err := h.workflowService.ListWorkflows(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workflows"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"workflows": workflows,
		"count":     len(workflows),
	})
}

func (h *APIHandlers) getWorkflow(c *gin.Context) {
	workflowID := c.Param("workflowId")
	record, err := h.workflowService.GetWorkflow(c.Request.Context(), workflowID, 0)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workflow": record})
}

func (h *APIHandlers) getWorkflowVersion(c *gin.Context) {
	workflowID := c.Param("workflowId")
	versionStr := c.Param("version")
	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid workflow version"})
		return
	}

	record, err := h.workflowService.GetWorkflow(c.Request.Context(), workflowID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Workflow version not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"workflow": record})
}

func (h *APIHandlers) listWorkflowVersions(c *gin.Context) {
	workflowID := c.Param("workflowId")
	records, err := h.workflowService.ListWorkflowVersions(c.Request.Context(), workflowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workflow versions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"versions": records,
		"count":    len(records),
	})
}

func (h *APIHandlers) disableWorkflow(c *gin.Context) {
	workflowID := c.Param("workflowId")
	if err := h.workflowService.DisableWorkflow(c.Request.Context(), workflowID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disable workflow"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message":     "Workflow disabled",
		"workflow_id": workflowID,
	})
}
