package v1

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"station/pkg/models"
)

// registerToolsRoutes registers tools routes
func (h *APIHandlers) registerToolsRoutes(group *gin.RouterGroup) {
	group.GET("", h.listTools)
}

// Tools handlers

func (h *APIHandlers) listTools(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get filter parameter
	filter := c.Query("filter")

	tools, err := h.repos.MCPTools.GetByEnvironmentID(envID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tools"})
		return
	}

	// Convert to MCPToolWithDetails - for now just create a simple version
	toolsWithDetails := make([]*models.MCPToolWithDetails, len(tools))
	for i, tool := range tools {
		toolsWithDetails[i] = &models.MCPToolWithDetails{
			MCPTool:           *tool,
			ConfigName:        "unknown", // Would need to join with config
			ConfigVersion:     1,         // Would need to join with config
			ServerName:        "unknown", // Would need to join with server
			EnvironmentName:   "unknown", // Would need to join with environment
		}
	}

	// Apply filter if provided
	if filter != "" {
		filteredTools := make([]*models.MCPToolWithDetails, 0)
		filterLower := strings.ToLower(filter)
		
		for _, tool := range toolsWithDetails {
			if strings.Contains(strings.ToLower(tool.Name), filterLower) ||
				strings.Contains(strings.ToLower(tool.Description), filterLower) {
				filteredTools = append(filteredTools, tool)
			}
		}
		toolsWithDetails = filteredTools
	}

	c.JSON(http.StatusOK, gin.H{
		"tools": toolsWithDetails,
		"count": len(toolsWithDetails),
		"filter": filter,
	})
}