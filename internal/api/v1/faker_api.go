package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"station/internal/config"
	"station/internal/services"

	"github.com/gin-gonic/gin"
)

// registerFakerRoutes registers faker management routes
func (h *APIHandlers) registerFakerRoutes(envGroup *gin.RouterGroup) {
	fakerGroup := envGroup.Group("/:env_id/fakers")
	fakerGroup.POST("", h.createFaker)
	fakerGroup.GET("", h.listFakers)
	fakerGroup.GET("/:faker_name", h.getFakerDetails)
	fakerGroup.DELETE("/:faker_name", h.deleteFaker)
}

// createFaker creates a new standalone faker configuration
func (h *APIHandlers) createFaker(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Instruction string `json:"instruction"`
		Template    string `json:"template"`
		Model       string `json:"model"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Must provide either instruction or template
	if req.Instruction == "" && req.Template == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either 'instruction' or 'template' must be provided"})
		return
	}

	// Get environment
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Load config for templates and defaults
	cfg, err := config.Load()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load config"})
		return
	}

	// Determine instruction (from template or custom)
	var instruction string
	var aiModel string

	if req.Template != "" {
		// Use template
		if template, exists := cfg.FakerTemplates[req.Template]; exists {
			instruction = template.Instruction
			aiModel = template.Model
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Template '%s' not found", req.Template)})
			return
		}
	} else {
		// Use custom instruction
		instruction = req.Instruction
	}

	// Determine model (req > template > global config)
	if req.Model != "" {
		aiModel = req.Model
	} else if aiModel == "" {
		aiModel = cfg.AIModel
	}

	if aiModel == "" {
		aiModel = "gpt-4o-mini" // Final fallback
	}

	// Build MCP server config (proper format with args array)
	serverConfig := services.MCPServerConfig{
		Name:        req.Name,
		Description: fmt.Sprintf("Faker configuration for %s", req.Name),
		Command:     "stn", // Use stn from PATH, not absolute path
		Args: []string{
			"faker",
			"--standalone",
			"--faker-id", req.Name,
			"--ai-model", aiModel,
			"--ai-instruction", instruction,
		},
		Env: make(map[string]string),
	}

	// Add MCP server to environment
	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.AddMCPServerToEnvironment(env.Name, req.Name, serverConfig)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to create faker",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     result.Message,
		"faker_name":  req.Name,
		"environment": env.Name,
		"model":       aiModel,
		"result":      result,
	})
}

// listFakers lists all fakers in an environment
func (h *APIHandlers) listFakers(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get environment
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Get all MCP servers
	mcpService := services.NewMCPServerManagementService(h.repos)
	servers, err := mcpService.GetMCPServersForEnvironment(env.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MCP servers"})
		return
	}

	// Filter for fakers (servers using "stn faker --standalone")
	var fakers []map[string]interface{}
	for name, server := range servers {
		// Check if it's a faker by looking for "faker" in command or args
		if server.Command == "stn" && len(server.Args) > 0 && server.Args[0] == "faker" {
			// Parse faker config
			fakerInfo := map[string]interface{}{
				"name":    name,
				"command": server.Command,
				"args":    server.Args,
			}

			// Extract model and instruction from args
			for i, arg := range server.Args {
				if arg == "--ai-model" && i+1 < len(server.Args) {
					fakerInfo["model"] = server.Args[i+1]
				}
				if arg == "--ai-instruction" && i+1 < len(server.Args) {
					fakerInfo["instruction"] = server.Args[i+1]
				}
				if arg == "--faker-id" && i+1 < len(server.Args) {
					fakerInfo["faker_id"] = server.Args[i+1]
				}
			}

			fakers = append(fakers, fakerInfo)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"fakers":      fakers,
		"environment": env.Name,
		"count":       len(fakers),
	})
}

// getFakerDetails gets details of a specific faker
func (h *APIHandlers) getFakerDetails(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	fakerName := c.Param("faker_name")
	if fakerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Faker name is required"})
		return
	}

	// Get environment
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Get MCP server
	mcpService := services.NewMCPServerManagementService(h.repos)
	servers, err := mcpService.GetMCPServersForEnvironment(env.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get MCP servers"})
		return
	}

	server, exists := servers[fakerName]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Faker '%s' not found", fakerName)})
		return
	}

	// Verify it's a faker
	if server.Command != "stn" || len(server.Args) == 0 || server.Args[0] != "faker" {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Server '%s' is not a faker", fakerName)})
		return
	}

	// Build response
	fakerInfo := map[string]interface{}{
		"name":        fakerName,
		"description": server.Description,
		"command":     server.Command,
		"args":        server.Args,
		"env":         server.Env,
	}

	// Extract model and instruction from args
	for i, arg := range server.Args {
		if arg == "--ai-model" && i+1 < len(server.Args) {
			fakerInfo["model"] = server.Args[i+1]
		}
		if arg == "--ai-instruction" && i+1 < len(server.Args) {
			fakerInfo["instruction"] = server.Args[i+1]
		}
		if arg == "--faker-id" && i+1 < len(server.Args) {
			fakerInfo["faker_id"] = server.Args[i+1]
		}
	}

	c.JSON(http.StatusOK, fakerInfo)
}

// deleteFaker deletes a faker configuration
func (h *APIHandlers) deleteFaker(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	fakerName := c.Param("faker_name")
	if fakerName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Faker name is required"})
		return
	}

	// Get environment
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Delete MCP server
	mcpService := services.NewMCPServerManagementService(h.repos)
	result := mcpService.DeleteMCPServerFromEnvironment(env.Name, fakerName)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to delete faker",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
		"result":  result,
	})
}
