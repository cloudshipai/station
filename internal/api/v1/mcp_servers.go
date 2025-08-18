package v1

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// registerMCPServerRoutes registers MCP server routes
func (h *APIHandlers) registerMCPServerRoutes(group *gin.RouterGroup) {
	group.GET("", h.listMCPServers)
	group.GET("/:id", h.getMCPServer)
	group.GET("/:id/tools", h.getMCPServerTools)
	group.POST("", h.createMCPServer)
	group.PUT("/:id", h.updateMCPServer)
	group.DELETE("/:id", h.deleteMCPServer)
}

// listMCPServers lists all MCP servers, optionally filtered by environment
func (h *APIHandlers) listMCPServers(c *gin.Context) {
	environmentIDStr := c.Query("environment_id")
	
	if environmentIDStr != "" {
		// Filter by environment
		environmentID, err := strconv.ParseInt(environmentIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment_id"})
			return
		}
		
		servers, err := h.repos.MCPServers.GetByEnvironmentID(environmentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch MCP servers"})
			return
		}
		
		c.JSON(http.StatusOK, servers)
		return
	}
	
	// For now, return empty list since we don't have a "get all" method
	// This could be implemented if needed
	c.JSON(http.StatusOK, []interface{}{})
}

// getMCPServer gets a specific MCP server by ID
func (h *APIHandlers) getMCPServer(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	
	server, err := h.repos.MCPServers.GetByID(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}
	
	c.JSON(http.StatusOK, server)
}

// createMCPServer creates a new MCP server
func (h *APIHandlers) createMCPServer(c *gin.Context) {
	// Implementation would go here
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Create MCP server not implemented"})
}

// updateMCPServer updates an existing MCP server
func (h *APIHandlers) updateMCPServer(c *gin.Context) {
	// Implementation would go here
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update MCP server not implemented"})
}

// deleteMCPServer deletes an MCP server
func (h *APIHandlers) deleteMCPServer(c *gin.Context) {
	// Implementation would go here
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Delete MCP server not implemented"})
}

// getMCPServerTools gets all tools for a specific MCP server
func (h *APIHandlers) getMCPServerTools(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}
	
	tools, err := h.repos.MCPTools.GetByServerID(serverID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tools"})
		return
	}
	
	c.JSON(http.StatusOK, tools)
}