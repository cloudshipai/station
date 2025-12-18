package v1

import (
	"encoding/json"
	"fmt"
	"log"
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
				"id":              server.ID,
				"name":            server.Name,
				"command":         server.Command,
				"args":            server.Args,
				"env":             server.Env,
				"working_dir":     server.WorkingDir,
				"timeout_seconds": server.TimeoutSeconds,
				"auto_restart":    server.AutoRestart,
				"environment_id":  server.EnvironmentID,
				"created_at":      server.CreatedAt,
				"file_config_id":  server.FileConfigID,
				"tools_count":     toolsCount,
				"status":          "active",
				"error":           nil,
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
			"id":              server.ID,
			"name":            server.Name,
			"command":         server.Command,
			"args":            server.Args,
			"env":             server.Env,
			"working_dir":     server.WorkingDir,
			"timeout_seconds": server.TimeoutSeconds,
			"auto_restart":    server.AutoRestart,
			"environment_id":  server.EnvironmentID,
			"created_at":      server.CreatedAt,
			"file_config_id":  server.FileConfigID,
			"tools_count":     toolsCount,
			"status":          "active",
			"error":           nil,
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
		log.Printf("[ERROR] Invalid create MCP server request: %v", err)
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	log.Printf("[INFO] Creating MCP server name=%s environment=%s", req.Name, req.Environment)

	// Parse the config to extract MCP server configuration
	var configData map[string]interface{}
	switch cfg := req.Config.(type) {
	case string:
		if err := json.Unmarshal([]byte(cfg), &configData); err != nil {
			log.Printf("[ERROR] Invalid JSON configuration: %v", err)
			c.JSON(http.StatusBadRequest, MCPServerResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid JSON configuration: %v", err),
			})
			return
		}
	case map[string]interface{}:
		configData = cfg
	default:
		log.Printf("[ERROR] Config must be JSON string or object, got type: %T", cfg)
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "Config must be a JSON string or object",
		})
		return
	}

	// Extract the MCP server configuration from the config
	mcpServersData, exists := configData["mcpServers"]
	if !exists {
		log.Printf("[ERROR] Config missing mcpServers field")
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "Config must contain 'mcpServers' field",
		})
		return
	}

	mcpServersMap, ok := mcpServersData.(map[string]interface{})
	if !ok {
		log.Printf("[ERROR] mcpServers is not an object, got type: %T", mcpServersData)
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "mcpServers must be an object",
		})
		return
	}

	// Log available server keys
	serverKeys := make([]string, 0, len(mcpServersMap))
	for key := range mcpServersMap {
		serverKeys = append(serverKeys, key)
	}
	log.Printf("[INFO] Found %d MCP server configs: %v (requested: %s)", len(mcpServersMap), serverKeys, req.Name)

	// Support both single-server and multi-server configs
	// First try to find a config matching the server name
	serverConfigData, exists := mcpServersMap[req.Name]

	// If not found, check if there's only one server config and use that
	if !exists {
		if len(mcpServersMap) == 1 {
			// Use the single server config regardless of key name
			for key, config := range mcpServersMap {
				serverConfigData = config
				exists = true
				log.Printf("[INFO] Using single server config key=%s (requested: %s)", key, req.Name)
				break
			}
		}
	}

	// If still not found, return error with helpful message
	if !exists {
		log.Printf("[ERROR] No matching server config found for %s, available: %v", req.Name, serverKeys)
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("No configuration found for server '%s' in mcpServers. Available keys: %v", req.Name, serverKeys),
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

	// Extract command OR url (one is required)
	hasCommand := false
	hasURL := false

	if cmd, ok := serverConfigMap["command"].(string); ok && cmd != "" {
		serverConfig.Command = cmd
		hasCommand = true
	}

	if urlVal, ok := serverConfigMap["url"].(string); ok && urlVal != "" {
		serverConfig.URL = urlVal
		hasURL = true
	}

	if !hasCommand && !hasURL {
		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   "Server configuration must contain either 'command' or 'url' field",
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
	log.Printf("[INFO] Creating server config file: name=%s command=%s args=%d", req.Name, serverConfig.Command, len(serverConfig.Args))
	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.AddMCPServerToEnvironment(req.Environment, req.Name, serverConfig)

	if !result.Success {
		log.Printf("[ERROR] Failed to create server config file: %s", result.Message)
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   result.Message,
		})
		return
	}
	log.Printf("[INFO] Server config file created successfully: %s", result.Message)

	// Load config for variable detection and sync
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[ERROR] Failed to load config: %v", err)
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   "Failed to load configuration",
		})
		return
	}

	// Read the created template file to check for Go template variables
	// Use centralized config path helper
	templatePath := config.GetTemplateConfigPath(req.Environment, req.Name)
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		log.Printf("[ERROR] Failed to read template file for variable detection: %v", err)
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   "Failed to read template file for validation",
		})
		return
	}

	// Try to render template with empty variables to detect if variables are needed
	// This uses Go template engine's missingkey=error to properly detect template variables
	templateService := services.NewTemplateVariableService(cfg.Workspace, h.repos)
	hasVariables := templateService.HasTemplateVariables(string(templateContent))

	if hasVariables {
		log.Printf("[INFO] Config contains template variables (detected via Go template parsing), returning VARIABLES_NEEDED")
		c.JSON(http.StatusCreated, MCPServerResponse{
			Success:     true,
			Message:     "MCP server configuration created. Variables detected - please configure them via sync.",
			ServerName:  req.Name,
			Environment: req.Environment,
			FilePath:    fmt.Sprintf("environments/%s/%s.json", req.Environment, req.Name),
			Error:       "VARIABLES_NEEDED", // Special marker for frontend to open sync modal
		})
		return
	}

	// No template variables detected - run sync immediately to validate and connect
	log.Printf("[INFO] No template variables detected, running sync to validate config")
	log.Printf("[INFO] Starting sync for environment: %s", req.Environment)
	syncService := services.NewDeclarativeSync(h.repos, cfg)
	syncOptions := services.SyncOptions{
		DryRun:      false,
		Validate:    true,
		Interactive: false, // Non-interactive since no variables expected
		Verbose:     true,
		Confirm:     false,
	}

	// Run the sync
	ctx := c.Request.Context()
	syncResult, syncErr := syncService.SyncEnvironment(ctx, req.Environment, syncOptions)

	if syncErr != nil {
		// Sync failed - clean up and return error
		log.Printf("[ERROR] Sync failed for %s: %v", req.Name, syncErr)
		filePath := fmt.Sprintf("~/.config/station/environments/%s/%s.json", req.Environment, req.Name)
		absolutePath := config.ResolvePath(filePath)
		os.Remove(absolutePath) // Clean up the file

		c.JSON(http.StatusBadRequest, MCPServerResponse{
			Success: false,
			Error:   fmt.Sprintf("MCP server configuration validation failed: %v", syncErr),
		})
		return
	}
	log.Printf("[INFO] Sync completed: %d servers connected", syncResult.MCPServersConnected)

	// Check if the server actually got created and has tools
	// First get the environment ID
	log.Printf("[INFO] Validating server creation for environment: %s", req.Environment)
	environment, err := h.repos.Environments.GetByName(req.Environment)
	if err != nil {
		log.Printf("[ERROR] Failed to get environment %s: %v", req.Environment, err)
		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   "Failed to get environment",
		})
		return
	}

	// Find the server that was just created
	var createdServerID int64
	servers, _ := h.repos.MCPServers.GetByEnvironmentID(environment.ID)
	log.Printf("[INFO] Searching for server %s in environment %d (%d servers)", req.Name, environment.ID, len(servers))
	for _, server := range servers {
		if server.Name == req.Name {
			createdServerID = server.ID
			log.Printf("[INFO] Found server in database: ID=%d name=%s", createdServerID, server.Name)
			break
		}
	}

	if createdServerID == 0 {
		// Server wasn't created in DB, sync may have failed
		log.Printf("[ERROR] Server %s not found in database after sync", req.Name)
		filePath := fmt.Sprintf("~/.config/station/environments/%s/%s.json", req.Environment, req.Name)
		absolutePath := config.ResolvePath(filePath)
		os.Remove(absolutePath) // Clean up the file

		c.JSON(http.StatusInternalServerError, MCPServerResponse{
			Success: false,
			Error:   "MCP server was not created in database after sync. Configuration may be invalid.",
		})
		return
	}

	// Check if server has tools (indicates successful connection)
	tools, _ := h.repos.MCPTools.GetByServerID(createdServerID)
	log.Printf("[INFO] Server %d has %d tools", createdServerID, len(tools))

	if len(tools) == 0 {
		// No tools discovered - server likely failed to connect
		// Note: We don't remove the server here as it might be a valid config
		// that just needs variables configured
		log.Printf("[WARN] Server %s (ID=%d) created but no tools discovered", req.Name, createdServerID)
		c.JSON(http.StatusCreated, MCPServerResponse{
			Success:     true,
			Message:     fmt.Sprintf("MCP server created but no tools discovered. Server may need configuration or may have failed to connect. Found %d MCP servers connected during sync.", syncResult.MCPServersConnected),
			ServerName:  req.Name,
			Environment: req.Environment,
			FilePath:    fmt.Sprintf("~/.config/station/environments/%s/%s.json", req.Environment, req.Name),
		})
		return
	}

	log.Printf("[INFO] MCP server %s created successfully: ID=%d tools=%d", req.Name, createdServerID, len(tools))
	c.JSON(http.StatusCreated, MCPServerResponse{
		Success:     true,
		Message:     fmt.Sprintf("MCP server created successfully with %d tools discovered!", len(tools)),
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
		"message":          fmt.Sprintf("MCP server '%s' deleted successfully", server.Name),
		"server_id":        serverID,
		"server_name":      server.Name,
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
		"server_id":   serverID,
		"server_name": server.Name,
		"config":      string(content),
		"file_path":   fileConfig.TemplatePath,
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
		"message":     "Configuration updated successfully",
		"server_id":   serverID,
		"server_name": server.Name,
		"file_path":   fileConfig.TemplatePath,
	})
}
