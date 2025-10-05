package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"station/internal/config"
	"station/internal/services"
)

// MCP Server request structure - supports both string and object config formats
type MCPServerRequest struct {
	Name        string      `json:"name" binding:"required"`
	Config      interface{} `json:"config" binding:"required"`
	Environment string      `json:"environment" binding:"required"`
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
	group.GET("/:id/config", h.getMCPServerRawConfig)
	group.PUT("/:id/config", h.updateMCPServerRawConfig)
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
		
		// Enhance servers with status information
		serverResponses := make([]map[string]interface{}, len(servers))
		for i, server := range servers {
			// Get tools count for this server
			tools, err := h.repos.MCPTools.GetByServerID(server.ID)
			toolsCount := 0
			if err == nil {
				toolsCount = len(tools)
			}

			serverData := map[string]interface{}{
				"id":             server.ID,
				"name":           server.Name,
				"command":        server.Command,
				"args":           server.Args,
				"env":            server.Env,
				"working_dir":    server.WorkingDir,
				"timeout_seconds": server.TimeoutSeconds,
				"auto_restart":   server.AutoRestart,
				"environment_id": server.EnvironmentID,
				"created_at":     server.CreatedAt,
				"file_config_id": server.FileConfigID,
				"tools_count":    toolsCount,
				"status":         "active",
				"error":          nil,
			}

			// Check for template variable issues
			argsStr := fmt.Sprintf("%v", server.Args)
			if strings.Contains(argsStr, "<no value>") {
				serverData["status"] = "error"
				serverData["error"] = "Template variables not configured. Run 'stn sync' to configure missing variables."
			}

			// Check if server has 0 tools - indicates sync failure or connection error
			if toolsCount == 0 {
				serverData["status"] = "error"
				if serverData["error"] == nil {
					serverData["error"] = "MCP server failed to load tools. Sync may have failed or server connection error."
				}
			}

			serverResponses[i] = serverData
		}
		
		c.JSON(http.StatusOK, gin.H{"servers": serverResponses})
		return
	}
	
	// Get all servers across all environments
	servers, err := h.repos.MCPServers.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch MCP servers"})
		return
	}

	// Enhance servers with status information
	serverResponses := make([]map[string]interface{}, len(servers))
	for i, server := range servers {
		// Get tools count for this server
		tools, err := h.repos.MCPTools.GetByServerID(server.ID)
		toolsCount := 0
		if err == nil {
			toolsCount = len(tools)
		}

		serverData := map[string]interface{}{
			"id":             server.ID,
			"name":           server.Name,
			"command":        server.Command,
			"args":           server.Args,
			"env":            server.Env,
			"working_dir":    server.WorkingDir,
			"timeout_seconds": server.TimeoutSeconds,
			"auto_restart":   server.AutoRestart,
			"environment_id": server.EnvironmentID,
			"created_at":     server.CreatedAt,
			"file_config_id": server.FileConfigID,
			"tools_count":    toolsCount,
			"status":         "active",
			"error":          nil,
		}

		// Check for template variable issues
		argsStr := fmt.Sprintf("%v", server.Args)
		if strings.Contains(argsStr, "<no value>") {
			serverData["status"] = "error"
			serverData["error"] = "Template variables not configured. Run 'stn sync' to configure missing variables."
		}

		// Check if server has 0 tools - indicates sync failure or connection error
		if toolsCount == 0 {
			serverData["status"] = "error"
			if serverData["error"] == nil {
				serverData["error"] = "MCP server failed to load tools. Sync may have failed or server connection error."
			}
		}

		serverResponses[i] = serverData
	}

	c.JSON(http.StatusOK, gin.H{"servers": serverResponses})
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

// createMCPServer creates a new MCP server configuration file using the management service
func (h *APIHandlers) createMCPServer(c *gin.Context) {
	var req MCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// Parse the config to extract MCP server configuration
	var configData map[string]interface{}
	switch cfg := req.Config.(type) {
	case string:
		if err := json.Unmarshal([]byte(cfg), &configData); err != nil {
			c.JSON(http.StatusBadRequest, MCPServerResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid JSON configuration: %v", err),
			})
			return
		}
	case map[string]interface{}:
		configData = cfg
	default:
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "Config must be a JSON string or object",
		})
		return
	}

	// Extract the MCP server configuration from the config
	mcpServersData, exists := configData["mcpServers"]
	if !exists {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "Config must contain 'mcpServers' field",
		})
		return
	}

	mcpServersMap, ok := mcpServersData.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "mcpServers must be an object",
		})
		return
	}

	// Get the server config for the specified server name
	serverConfigData, exists := mcpServersMap[req.Name]
	if !exists {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("No configuration found for server '%s' in mcpServers", req.Name),
		})
		return
	}

	serverConfigMap, ok := serverConfigData.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Configuration for server '%s' must be an object", req.Name),
		})
		return
	}

	// Convert to our MCPServerConfig format
	serverConfig := services.MCPServerConfig{
		Name:        req.Name,
		Description: "",
		Command:     "",
		Args:        []string{},
		Env:         make(map[string]string),
	}

	// Extract description if present
	if desc, ok := serverConfigMap["description"].(string); ok {
		serverConfig.Description = desc
	}

	// Extract command
	if cmd, ok := serverConfigMap["command"].(string); ok {
		serverConfig.Command = cmd
	} else {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "Server configuration must contain 'command' field",
		})
		return
	}

	// Extract args
	if argsData, ok := serverConfigMap["args"].([]interface{}); ok {
		for _, arg := range argsData {
			if argStr, ok := arg.(string); ok {
				serverConfig.Args = append(serverConfig.Args, argStr)
			}
		}
	}

	// Extract env
	if envData, ok := serverConfigMap["env"].(map[string]interface{}); ok {
		for key, value := range envData {
			if valueStr, ok := value.(string); ok {
				serverConfig.Env[key] = valueStr
			}
		}
	}

	// Use the management service to create the server file with template variable conversion
	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.AddMCPServerToEnvironment(req.Environment, req.Name, serverConfig)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   result.Message,
		})
		return
	}

	c.JSON(http.StatusCreated, MCPServerResponse{
		Success:     true,
		Message:     result.Message,
		ServerName:  req.Name,
		Environment: req.Environment,
		FilePath:    fmt.Sprintf("~/.config/station/environments/%s/%s.json", req.Environment, req.Name),
	})
}

// updateMCPServer updates an existing MCP server
func (h *APIHandlers) updateMCPServer(c *gin.Context) {
	// Implementation would go here
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Update MCP server not implemented"})
}

// deleteMCPServer deletes an MCP server from both database and template files
func (h *APIHandlers) deleteMCPServer(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Get the server first to check if it exists
	server, err := h.repos.MCPServers.GetByID(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Get environment information for template cleanup
	environment, err := h.repos.Environments.GetByID(server.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get environment information"})
		return
	}

	// Use MCP management service for comprehensive cleanup (both DB and template)
	mcpService := services.NewMCPServerManagementService(h.repos)

	// First delete from database (tools, file configs, server record)
	if err := h.repos.MCPTools.DeleteByServerID(serverID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP server tools"})
		return
	}

	// Delete file config records and actual config file if they exist
	if server.FileConfigID != nil {
		// Get file config to find the template path
		fileConfig, err := h.repos.FileMCPConfigs.GetByID(*server.FileConfigID)
		if err == nil && fileConfig.TemplatePath != "" {
			// Delete the actual config file from filesystem
			absolutePath := config.ResolvePath(fileConfig.TemplatePath)
			if err := os.Remove(absolutePath); err != nil {
				// Log error but don't fail the entire operation
				fmt.Printf("Warning: Failed to delete config file %s: %v\n", absolutePath, err)
			}
		}

		// Delete the file config record from database
		if err := h.repos.FileMCPConfigs.Delete(*server.FileConfigID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file config"})
			return
		}
	}

	// Delete the server from database
	if err := h.repos.MCPServers.Delete(serverID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete MCP server"})
		return
	}

	// Also attempt to delete from template.json (if it exists there)
	// This is a best-effort cleanup - we don't fail if template cleanup fails
	templateResult := mcpService.DeleteMCPServerFromEnvironment(environment.Name, server.Name)

	response := gin.H{
		"message": fmt.Sprintf("MCP server '%s' deleted successfully", server.Name),
		"server_id": serverID,
		"server_name": server.Name,
		"database_deleted": true,
	}

	// Add template cleanup status if it was attempted
	if templateResult.Success {
		response["template_deleted"] = true
		response["message"] = fmt.Sprintf("MCP server '%s' deleted successfully from both database and template", server.Name)
	} else {
		response["template_deleted"] = false
		response["template_cleanup_note"] = "Template cleanup skipped (server not in template.json)"
	}

	c.JSON(http.StatusOK, response)
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

// getMCPServerRawConfig gets the raw configuration for a specific MCP server
func (h *APIHandlers) getMCPServerRawConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	// Get the MCP server from database
	server, err := h.repos.MCPServers.GetByID(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Get the file config to find the source template file
	if server.FileConfigID == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No file configuration associated with this server"})
		return
	}

	fileConfig, err := h.repos.FileMCPConfigs.GetByID(*server.FileConfigID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file configuration"})
		return
	}

	// Read the template file content using centralized path resolution
	absolutePath := config.ResolvePath(fileConfig.TemplatePath)
	if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Configuration file not found"})
		return
	}

	content, err := os.ReadFile(absolutePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read configuration file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"server_id": serverID,
		"server_name": server.Name,
		"config": string(content),
		"file_path": fileConfig.TemplatePath,
	})
}

// updateMCPServerRawConfig updates the raw configuration for a specific MCP server
func (h *APIHandlers) updateMCPServerRawConfig(c *gin.Context) {
	serverID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid server ID"})
		return
	}

	var req struct {
		Config string `json:"config" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate JSON
	var configData interface{}
	if err := json.Unmarshal([]byte(req.Config), &configData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON configuration"})
		return
	}

	// Get the MCP server from database
	server, err := h.repos.MCPServers.GetByID(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "MCP server not found"})
		return
	}

	// Get the file config to find the source template file
	if server.FileConfigID == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No file configuration associated with this server"})
		return
	}

	fileConfig, err := h.repos.FileMCPConfigs.GetByID(*server.FileConfigID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file configuration"})
		return
	}

	// Write the updated configuration to file using centralized path resolution
	absolutePath := config.ResolvePath(fileConfig.TemplatePath)
	if err := os.WriteFile(absolutePath, []byte(req.Config), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write configuration file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Configuration updated successfully",
		"server_id": serverID,
		"server_name": server.Name,
		"file_path": fileConfig.TemplatePath,
	})
}