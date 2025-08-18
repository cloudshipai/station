package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MCP Server request structure
type MCPServerRequest struct {
	Name        string `json:"name" binding:"required"`
	Config      string `json:"config" binding:"required"`
	Environment string `json:"environment" binding:"required"`
}

// MCP Server response structure
type MCPServerResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ServerName  string `json:"server_name,omitempty"`
	Environment string `json:"environment,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	Error       string `json:"error,omitempty"`
}

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

// createMCPServer creates a new MCP server configuration file
func (h *APIHandlers) createMCPServer(c *gin.Context) {
	var req MCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Validate JSON config
	var configData interface{}
	if err := json.Unmarshal([]byte(req.Config), &configData); err != nil {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid JSON configuration: %v", err),
		})
		return
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   "Failed to get home directory",
		})
		return
	}

	// Environment directory path
	envPath := filepath.Join(homeDir, ".config", "station", "environments", req.Environment)
	
	// Check if environment directory exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Environment '%s' not found", req.Environment),
		})
		return
	}

	// Create the config file path
	configFileName := fmt.Sprintf("%s.json", req.Name)
	configFilePath := filepath.Join(envPath, configFileName)

	// Check if file already exists
	if _, err := os.Stat(configFilePath); err == nil {
		c.JSON(http.StatusConflict, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("MCP server config '%s' already exists in environment '%s'", req.Name, req.Environment),
		})
		return
	}

	// Write the config to file
	if err := os.WriteFile(configFilePath, []byte(req.Config), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to write config file: %v", err),
		})
		return
	}

	c.JSON(http.StatusCreated, MCPServerResponse{
		Success:     true,
		Message:     fmt.Sprintf("MCP server '%s' created successfully in environment '%s'", req.Name, req.Environment),
		ServerName:  req.Name,
		Environment: req.Environment,
		FilePath:    configFilePath,
	})
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