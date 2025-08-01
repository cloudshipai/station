package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"station/internal/auth"
	"station/internal/services"
)

// registerAgentAdminRoutes registers admin-only agent management routes
func (h *APIHandlers) registerAgentAdminRoutes(group *gin.RouterGroup) {
	group.POST("", h.createAgent)
	group.GET("/:id", h.getAgent)
	group.PUT("/:id", h.updateAgent)
	group.DELETE("/:id", h.deleteAgent)
}

// Agent handlers

func (h *APIHandlers) listAgents(c *gin.Context) {
	agents, err := h.repos.Agents.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list agents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
		"count":  len(agents),
	})
}

func (h *APIHandlers) callAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req struct {
		Task string `json:"task" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Execute agent using the genkit service
	if h.genkitService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent execution service not available"})
		return
	}

	response, err := h.genkitService.ExecuteAgent(c.Request.Context(), agentID, req.Task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to execute agent: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agent_id": agentID,
		"task":     req.Task,
		"response": response.Content,
		"success":  true,
	})
}

func (h *APIHandlers) createAgent(c *gin.Context) {
	var req struct {
		Name          string   `json:"name" binding:"required"`
		Description   string   `json:"description" binding:"required"`
		Prompt        string   `json:"prompt" binding:"required"`
		EnvironmentID int64    `json:"environment_id" binding:"required"`
		MaxSteps      int64    `json:"max_steps"`
		AssignedTools []string `json:"assigned_tools"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user for created_by field
	var createdBy int64 = 1 // Default for local mode
	if !h.localMode {
		user, exists := auth.GetUserFromContext(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}
		createdBy = user.ID
	}

	// Set default max steps if not provided
	if req.MaxSteps == 0 {
		req.MaxSteps = 5
	}

	// Create agent using genkit service
	if h.genkitService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Agent service not available"})
		return
	}

	agentConfig := &services.AgentConfig{
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		Description:   req.Description,
		Prompt:        req.Prompt,
		AssignedTools: req.AssignedTools,
		MaxSteps:      req.MaxSteps,
		CreatedBy:     createdBy,
	}

	agent, err := h.genkitService.CreateAgent(c.Request.Context(), agentConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create agent: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"agent": agent})
}

func (h *APIHandlers) getAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	agent, err := h.repos.Agents.GetByID(agentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"agent": agent})
}

func (h *APIHandlers) updateAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Prompt      string `json:"prompt"`
		MaxSteps    int64  `json:"max_steps"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update agent fields if provided
	if req.Name != "" || req.Description != "" || req.Prompt != "" || req.MaxSteps > 0 {
		err = h.repos.Agents.Update(agentID, req.Name, req.Description, req.Prompt, req.MaxSteps, nil, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agent"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent updated successfully"})
}

func (h *APIHandlers) deleteAgent(c *gin.Context) {
	agentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent ID"})
		return
	}

	err = h.repos.Agents.Delete(agentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete agent"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Agent deleted successfully"})
}