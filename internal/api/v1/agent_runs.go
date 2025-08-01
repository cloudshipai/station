package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// registerAgentRunRoutes registers agent run routes
func (h *APIHandlers) registerAgentRunRoutes(group *gin.RouterGroup) {
	group.GET("", h.listRuns)              // Users can list runs
	group.GET("/:id", h.getRun)            // Users can get run details
	group.GET("/agent/:agent_id", h.listRunsByAgent) // Users can list runs by agent
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

	runs, err := h.repos.AgentRuns.ListRecent(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":  runs,
		"count": len(runs),
		"limit": limit,
	})
}

func (h *APIHandlers) getRun(c *gin.Context) {
	runID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid run ID"})
		return
	}

	run, err := h.repos.AgentRuns.GetByIDWithDetails(runID)
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

	runs, err := h.repos.AgentRuns.ListByAgent(agentID)
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