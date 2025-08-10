package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// registerEnvironmentRoutes registers environment routes
func (h *APIHandlers) registerEnvironmentRoutes(group *gin.RouterGroup) {
	group.GET("", h.listEnvironments)
	group.POST("", h.createEnvironment)
	group.GET("/:env_id", h.getEnvironment)
	group.PUT("/:env_id", h.updateEnvironment)
	group.DELETE("/:env_id", h.deleteEnvironment)
}

// Environment handlers

func (h *APIHandlers) listEnvironments(c *gin.Context) {
	environments, err := h.repos.Environments.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list environments"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"environments": environments,
		"count":        len(environments),
	})
}

func (h *APIHandlers) createEnvironment(c *gin.Context) {
	var req struct {
		Name        string  `json:"name" binding:"required"`
		Description *string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get console user for created_by field
	consoleUser, err := h.repos.Users.GetByUsername("console")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get console user"})
		return
	}
	
	env, err := h.repos.Environments.Create(req.Name, req.Description, consoleUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"environment": env})
}

func (h *APIHandlers) getEnvironment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	env, err := h.repos.Environments.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"environment": env})
}

func (h *APIHandlers) updateEnvironment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.repos.Environments.Update(id, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment updated successfully"})
}

func (h *APIHandlers) deleteEnvironment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	err = h.repos.Environments.Delete(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete environment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment deleted successfully"})
}