package v1

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"station/internal/config"
	"station/pkg/openapi"
)

// OpenAPIConvertRequest represents a request to convert OpenAPI spec to MCP server
type OpenAPIConvertRequest struct {
	Spec           string `json:"spec" binding:"required"`
	ServerName     string `json:"server_name"`
	ToolNamePrefix string `json:"tool_name_prefix"`
	BaseURL        string `json:"base_url"`
	EnvironmentID  int64  `json:"environment_id"`
}

// OpenAPIConvertResponse represents the response from OpenAPI conversion
type OpenAPIConvertResponse struct {
	Success  bool   `json:"success"`
	FileName string `json:"file_name"`
	Config   string `json:"config"`
	Message  string `json:"message,omitempty"`
}

// registerOpenAPIRoutes registers OpenAPI-related routes
func (h *APIHandlers) registerOpenAPIRoutes(router *gin.RouterGroup) {
	router.POST("/convert", h.convertOpenAPISpec)
	router.POST("/save", h.saveOpenAPIMCPConfig)
	router.POST("/specs", h.createOpenAPISpec)                      // Unified endpoint for UI modal
	router.GET("/specs/:environment", h.listOpenAPISpecs)           // List OpenAPI specs for an environment
	router.GET("/specs/:environment/:name", h.getOpenAPISpec)       // Get specific OpenAPI spec
	router.PUT("/specs/:environment/:name", h.updateOpenAPISpec)    // Update OpenAPI spec
	router.DELETE("/specs/:environment/:name", h.deleteOpenAPISpec) // Delete OpenAPI spec
}

// convertOpenAPISpec converts an OpenAPI spec to MCP server configuration
func (h *APIHandlers) convertOpenAPISpec(c *gin.Context) {
	var req OpenAPIConvertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create OpenAPI service
	svc := openapi.NewService()

	// Validate the OpenAPI spec first
	if err := svc.ValidateSpec(req.Spec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid OpenAPI specification",
			"details": err.Error(),
		})
		return
	}

	// Set default server name if not provided
	if req.ServerName == "" {
		req.ServerName = "openapi-server"
	}

	// Convert the spec
	options := openapi.ConvertOptions{
		ServerName:     req.ServerName,
		ToolNamePrefix: req.ToolNamePrefix,
		BaseURL:        req.BaseURL,
	}

	config, err := svc.ConvertFromSpec(req.Spec, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to convert OpenAPI spec",
			"details": err.Error(),
		})
		return
	}

	// Generate filename
	fileName := svc.GenerateFileName(req.ServerName)

	c.JSON(http.StatusOK, OpenAPIConvertResponse{
		Success:  true,
		FileName: fileName,
		Config:   config,
		Message:  fmt.Sprintf("Successfully converted OpenAPI spec to MCP server configuration"),
	})
}

// SaveOpenAPIMCPRequest represents a request to save OpenAPI MCP config
type SaveOpenAPIMCPRequest struct {
	Config        string `json:"config" binding:"required"`
	FileName      string `json:"file_name" binding:"required"`
	EnvironmentID int64  `json:"environment_id" binding:"required"`
}

// saveOpenAPIMCPConfig saves the converted MCP config to an environment
func (h *APIHandlers) saveOpenAPIMCPConfig(c *gin.Context) {
	var req SaveOpenAPIMCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get environment
	env, err := h.repos.Environments.GetByID(req.EnvironmentID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Get workspace path
	workspacePath := config.GetStationConfigDir()

	// Build environment path
	envPath := filepath.Join(workspacePath, "environments", env.Name)

	// Sanitize filename
	fileName := req.FileName
	if !strings.HasSuffix(fileName, ".json") {
		fileName += ".json"
	}

	// Create OpenAPI service
	svc := openapi.NewService()

	// Save the config
	if err := svc.SaveToEnvironment(req.Config, fileName, envPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save MCP config",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     fmt.Sprintf("MCP server config saved to %s. Run 'stn sync %s' to activate.", fileName, env.Name),
		"file":        fileName,
		"environment": env.Name,
	})
}

// CreateOpenAPISpecRequest represents a request to create an OpenAPI spec
type CreateOpenAPISpecRequest struct {
	Name        string `json:"name" binding:"required"`
	Spec        string `json:"spec" binding:"required"`
	Environment string `json:"environment" binding:"required"`
}

// createOpenAPISpec creates an OpenAPI spec file and triggers sync
func (h *APIHandlers) createOpenAPISpec(c *gin.Context) {
	var req CreateOpenAPISpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get workspace path
	workspacePath := config.GetStationConfigDir()

	// Build environment path
	envPath := filepath.Join(workspacePath, "environments", req.Environment)

	// Create the .openapi.json file
	specFileName := fmt.Sprintf("%s.openapi.json", req.Name)
	specFilePath := filepath.Join(envPath, specFileName)

	// Write the OpenAPI spec file
	if err := os.WriteFile(specFilePath, []byte(req.Spec), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to write OpenAPI spec file",
			"details": err.Error(),
		})
		return
	}

	// Create OpenAPI service
	svc := openapi.NewService()

	// Validate the spec
	if err := svc.ValidateSpec(req.Spec); err != nil {
		// Clean up the file if validation fails
		os.Remove(specFilePath)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid OpenAPI specification",
			"details": err.Error(),
		})
		return
	}

	// Convert the spec to MCP config
	// Use relative path from config root so it works in containers
	relativeSpecPath := filepath.Join("environments", req.Environment, specFileName)
	options := openapi.ConvertOptions{
		ServerName:   req.Name,
		SpecFilePath: relativeSpecPath, // Use relative path for container compatibility
	}

	config, err := svc.ConvertFromSpec(req.Spec, options)
	if err != nil {
		os.Remove(specFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to convert OpenAPI spec to MCP config",
			"details": err.Error(),
		})
		return
	}

	// Generate MCP config filename
	mcpFileName := fmt.Sprintf("%s-openapi-mcp.json", req.Name)

	// Save the MCP config
	if err := svc.SaveToEnvironment(config, mcpFileName, envPath); err != nil {
		os.Remove(specFilePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save MCP config",
			"details": err.Error(),
		})
		return
	}

	// Check if spec contains template variables that need to be resolved
	if strings.Contains(req.Spec, "{{ .") {
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"error":       "VARIABLES_NEEDED",
			"message":     fmt.Sprintf("OpenAPI spec created. Variables detected - run 'stn sync %s' to configure.", req.Environment),
			"file":        specFileName,
			"environment": req.Environment,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"message":     fmt.Sprintf("OpenAPI spec created successfully. Run 'stn sync %s' to activate.", req.Environment),
		"file":        specFileName,
		"environment": req.Environment,
	})
}

// OpenAPISpecInfo represents metadata about an OpenAPI spec file
type OpenAPISpecInfo struct {
	Name        string `json:"name"`
	FileName    string `json:"file_name"`
	Environment string `json:"environment"`
	Size        int64  `json:"size"`
}

// listOpenAPISpecs lists all OpenAPI spec files in an environment
func (h *APIHandlers) listOpenAPISpecs(c *gin.Context) {
	envName := c.Param("environment")

	// Get workspace path
	workspacePath := config.GetStationConfigDir()
	envPath := filepath.Join(workspacePath, "environments", envName)

	// Read directory
	files, err := os.ReadDir(envPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read environment directory"})
		return
	}

	// Filter for .openapi.json files
	specs := []OpenAPISpecInfo{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".openapi.json") {
			info, _ := file.Info()
			specName := strings.TrimSuffix(file.Name(), ".openapi.json")
			specs = append(specs, OpenAPISpecInfo{
				Name:        specName,
				FileName:    file.Name(),
				Environment: envName,
				Size:        info.Size(),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"specs": specs,
		"count": len(specs),
	})
}

// getOpenAPISpec retrieves a specific OpenAPI spec file
func (h *APIHandlers) getOpenAPISpec(c *gin.Context) {
	envName := c.Param("environment")
	specName := c.Param("name")

	// Get workspace path
	workspacePath := config.GetStationConfigDir()
	specPath := filepath.Join(workspacePath, "environments", envName, specName+".openapi.json")

	// Read file
	content, err := os.ReadFile(specPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "OpenAPI spec not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    specName,
		"content": string(content),
		"path":    specPath,
	})
}

// UpdateOpenAPISpecRequest represents a request to update an OpenAPI spec
type UpdateOpenAPISpecRequest struct {
	Content string `json:"content" binding:"required"`
}

// updateOpenAPISpec updates an existing OpenAPI spec file
func (h *APIHandlers) updateOpenAPISpec(c *gin.Context) {
	envName := c.Param("environment")
	specName := c.Param("name")

	var req UpdateOpenAPISpecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate the spec
	svc := openapi.NewService()
	if err := svc.ValidateSpec(req.Content); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid OpenAPI specification",
			"details": err.Error(),
		})
		return
	}

	// Get workspace path
	workspacePath := config.GetStationConfigDir()
	specPath := filepath.Join(workspacePath, "environments", envName, specName+".openapi.json")

	// Check if file exists
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "OpenAPI spec not found"})
		return
	}

	// Write updated content
	if err := os.WriteFile(specPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update OpenAPI spec",
			"details": err.Error(),
		})
		return
	}

	// Regenerate MCP config
	// Use relative path from config root so it works in containers
	relativeSpecPath := filepath.Join("environments", envName, specName+".openapi.json")
	options := openapi.ConvertOptions{
		ServerName:   specName,
		SpecFilePath: relativeSpecPath,
	}

	mcpConfig, err := svc.ConvertFromSpec(req.Content, options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to regenerate MCP config",
			"details": err.Error(),
		})
		return
	}

	// Save updated MCP config
	envPath := filepath.Join(workspacePath, "environments", envName)
	mcpFileName := fmt.Sprintf("%s-openapi-mcp.json", specName)
	if err := svc.SaveToEnvironment(mcpConfig, mcpFileName, envPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save MCP config",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("OpenAPI spec updated successfully. Run 'stn sync %s' to apply changes.", envName),
	})
}

// deleteOpenAPISpec deletes an OpenAPI spec and its generated MCP config
func (h *APIHandlers) deleteOpenAPISpec(c *gin.Context) {
	envName := c.Param("environment")
	specName := c.Param("name")

	// Get workspace path
	workspacePath := config.GetStationConfigDir()
	envPath := filepath.Join(workspacePath, "environments", envName)

	specPath := filepath.Join(envPath, specName+".openapi.json")
	mcpPath := filepath.Join(envPath, specName+"-openapi-mcp.json")

	// Delete OpenAPI spec file
	if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete OpenAPI spec",
			"details": err.Error(),
		})
		return
	}

	// Delete MCP config file (ignore if doesn't exist)
	os.Remove(mcpPath)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("OpenAPI spec deleted successfully. Run 'stn sync %s' to apply changes.", envName),
	})
}
