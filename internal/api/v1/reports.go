package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"station/internal/config"
	"station/internal/db/queries"
	"station/internal/services"
)

// registerReportRoutes registers report routes
func (h *APIHandlers) registerReportRoutes(group *gin.RouterGroup) {
	group.GET("", h.listReports)                  // List all reports
	group.GET("/:id", h.getReport)                // Get report details
	group.POST("", h.createReport)                // Create a new report
	group.POST("/:id/generate", h.generateReport) // Generate a report
	group.DELETE("/:id", h.deleteReport)          // Delete a report
}

// CreateReportRequest is the request body for creating a report
type CreateReportRequest struct {
	Name          string                 `json:"name" binding:"required"`
	Description   string                 `json:"description"`
	EnvironmentID int64                  `json:"environment_id" binding:"required"`
	TeamCriteria  map[string]interface{} `json:"team_criteria" binding:"required"`
	AgentCriteria map[string]interface{} `json:"agent_criteria"`
	JudgeModel    string                 `json:"judge_model"`
}

// listReports lists all reports, optionally filtered by environment
func (h *APIHandlers) listReports(c *gin.Context) {
	ctx := c.Request.Context()

	// Check for environment filter
	envIDStr := c.Query("environment_id")

	var reports []queries.Report
	var err error

	if envIDStr != "" {
		envID, parseErr := strconv.ParseInt(envIDStr, 10, 64)
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment_id"})
			return
		}

		reports, err = h.repos.Reports.ListByEnvironment(ctx, envID)
	} else {
		// List all reports with pagination
		limit := int64(100)
		if limitStr := c.Query("limit"); limitStr != "" {
			if parsedLimit, err := strconv.ParseInt(limitStr, 10, 64); err == nil && parsedLimit > 0 {
				limit = parsedLimit
			}
		}

		offset := int64(0)
		if offsetStr := c.Query("offset"); offsetStr != "" {
			if parsedOffset, err := strconv.ParseInt(offsetStr, 10, 64); err == nil && parsedOffset >= 0 {
				offset = parsedOffset
			}
		}

		reports, err = h.repos.Reports.ListReports(ctx, queries.ListReportsParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"count":   len(reports),
	})
}

// getReport gets a single report with all details
func (h *APIHandlers) getReport(c *gin.Context) {
	ctx := c.Request.Context()

	reportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid report ID"})
		return
	}

	// Get report
	report, err := h.repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	// Get agent details
	agentDetails, err := h.repos.Reports.GetAgentReportDetails(ctx, reportID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get agent details"})
		return
	}

	// Get environment info
	env, err := h.repos.Environments.GetByID(report.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"report":        report,
		"agent_details": agentDetails,
		"environment":   env,
	})
}

// createReport creates a new report
func (h *APIHandlers) createReport(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify environment exists
	_, err := h.repos.Environments.GetByID(req.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Marshal team criteria to JSON
	teamCriteriaJSON, err := json.Marshal(req.TeamCriteria)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team_criteria format"})
		return
	}

	// Marshal agent criteria if provided
	var agentCriteriaJSON sql.NullString
	if req.AgentCriteria != nil {
		agentBytes, err := json.Marshal(req.AgentCriteria)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent_criteria format"})
			return
		}
		agentCriteriaJSON = sql.NullString{String: string(agentBytes), Valid: true}
	}

	// Set judge model - use Station's global AI model if not explicitly provided
	var judgeModel sql.NullString
	if req.JudgeModel != "" {
		judgeModel = sql.NullString{String: req.JudgeModel, Valid: true}
	} else {
		// Use Station's global AI configuration
		cfg, err := config.Load()
		if err == nil && cfg.AIModel != "" {
			judgeModel = sql.NullString{String: cfg.AIModel, Valid: true}
		} else {
			// Fallback only if config loading fails
			judgeModel = sql.NullString{String: "gpt-4o-mini", Valid: true}
		}
	}

	// Create report
	report, err := h.repos.Reports.CreateReport(ctx, queries.CreateReportParams{
		Name:          req.Name,
		Description:   sql.NullString{String: req.Description, Valid: req.Description != ""},
		EnvironmentID: req.EnvironmentID,
		TeamCriteria:  string(teamCriteriaJSON),
		AgentCriteria: agentCriteriaJSON,
		JudgeModel:    judgeModel,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create report"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"report":  report,
		"message": "Report created successfully",
	})
}

// generateReport triggers report generation
func (h *APIHandlers) generateReport(c *gin.Context) {
	ctx := c.Request.Context()

	reportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid report ID"})
		return
	}

	// Get report
	report, err := h.repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	// Check if report is already completed or in progress
	if report.Status == "completed" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Report already completed"})
		return
	}

	if report.Status == "generating_team" || report.Status == "generating_agents" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Report generation already in progress"})
		return
	}

	// Create report generator
	reportGenerator := services.NewReportGenerator(h.repos, h.db, nil)

	// Start generation in background with independent context
	go func() {
		// Use background context since request context will be cancelled
		_ = reportGenerator.GenerateReport(context.Background(), reportID)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":   "Report generation started",
		"report_id": reportID,
		"status":    "generating",
	})
}

// deleteReport deletes a report
func (h *APIHandlers) deleteReport(c *gin.Context) {
	ctx := c.Request.Context()

	reportID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid report ID"})
		return
	}

	// Check if report exists
	_, err = h.repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Report not found"})
		return
	}

	// Delete report
	err = h.repos.Reports.DeleteReport(ctx, reportID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete report"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Report deleted successfully",
	})
}
