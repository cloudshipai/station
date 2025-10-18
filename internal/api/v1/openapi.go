package v1

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"station/pkg/openapi"
	"github.com/gin-gonic/gin"
	"station/internal/config"
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
		"success": true,
		"message": fmt.Sprintf("MCP server config saved to %s. Run 'stn sync %s' to activate.", fileName, env.Name),
		"file":    fileName,
		"environment": env.Name,
	})
}