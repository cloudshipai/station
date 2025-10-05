package v1

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"station/cmd/main/handlers/build"
	"station/cmd/main/handlers/common"
	"station/internal/config"
	"station/internal/deployment"
	"station/internal/services"
	"gopkg.in/yaml.v2"
)

// registerEnvironmentRoutes registers environment routes
func (h *APIHandlers) registerEnvironmentRoutes(group *gin.RouterGroup) {
	group.GET("", h.listEnvironments)
	group.POST("", h.createEnvironment)
	group.GET("/:env_id", h.getEnvironment)
	group.PUT("/:env_id", h.updateEnvironment)
	group.DELETE("/:env_id", h.deleteEnvironment)
	group.POST("/build-image", h.buildEnvironmentImage)
	group.GET("/:env_id/variables", h.getEnvironmentVariables)
	group.PUT("/:env_id/variables", h.updateEnvironmentVariables)
	group.POST("/:env_id/deploy", h.generateDeploymentTemplate)
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

	// Use unified environment management service
	envService := services.NewEnvironmentManagementService(h.repos)
	env, result, err := envService.CreateEnvironment(req.Name, req.Description, consoleUser.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create environment"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"environment": env,
		"result":      result,
	})
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

	// Use unified environment management service
	envService := services.NewEnvironmentManagementService(h.repos)
	result := envService.DeleteEnvironmentByID(id)

	if !result.Success {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":  "Failed to delete environment",
			"result": result,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": result.Message,
		"result":  result,
	})
}

func (h *APIHandlers) buildEnvironmentImage(c *gin.Context) {
	var req struct {
		Environment string `json:"environment" binding:"required"`
		ImageName   string `json:"image_name" binding:"required"`
		Tag         string `json:"tag" binding:"required"`
		Provider    string `json:"provider"`
		Model       string `json:"model"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Load Station config to get actual provider and model settings
	stationConfig, err := config.Load()
	if err == nil {
		// Use user's actual config instead of hardcoded defaults
		if req.Provider == "" {
			req.Provider = stationConfig.AIProvider
		}
		if req.Model == "" {
			req.Model = stationConfig.AIModel
		}
	} else {
		// Fallback to defaults if config loading fails
		if req.Provider == "" {
			req.Provider = "openai"
		}
		if req.Model == "" {
			req.Model = "gpt-4o-mini"
		}
	}

	// Get station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get station config root"})
		return
	}

	// Check if environment exists
	envPath := filepath.Join(configRoot, "environments", req.Environment)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Environment '%s' not found", req.Environment)})
		return
	}

	// Read environment variables
	variablesPath := filepath.Join(envPath, "variables.yml")
	environmentVariables := make(map[string]string)

	if _, err := os.Stat(variablesPath); err == nil {
		variablesData, err := os.ReadFile(variablesPath)
		if err == nil {
			var variables map[string]interface{}
			if yaml.Unmarshal(variablesData, &variables) == nil {
				for key, value := range variables {
					environmentVariables[key] = fmt.Sprintf("%v", value)
				}
			}
		}
	}

	// Load Station config to get AI provider and add appropriate API key placeholder
	stationConfig, configErr := config.Load()
	if configErr == nil {
		// Add provider-specific API key environment variable with placeholder
		switch stationConfig.AIProvider {
		case "openai":
			environmentVariables["OPENAI_API_KEY"] = "<your-openai-api-key>"
		case "gemini":
			environmentVariables["GOOGLE_API_KEY"] = "<your-google-api-key>"
		case "cloudflare":
			environmentVariables["CF_TOKEN"] = "<your-cloudflare-token>"
		case "ollama":
			// Ollama typically doesn't need API keys for local instances
			environmentVariables["AI_API_KEY"] = "<your-ai-api-key>"
		default:
			// For unknown providers, add as generic AI_API_KEY
			environmentVariables["AI_API_KEY"] = "<your-ai-api-key>"
		}

		// Add essential Station configuration for Docker container
		environmentVariables["STATION_AI_PROVIDER"] = stationConfig.AIProvider
		environmentVariables["STATION_AI_MODEL"] = stationConfig.AIModel
		environmentVariables["STATION_API_PORT"] = fmt.Sprintf("%d", stationConfig.APIPort)
		environmentVariables["STATION_MCP_PORT"] = fmt.Sprintf("%d", stationConfig.MCPPort)
		environmentVariables["STATION_SSH_PORT"] = fmt.Sprintf("%d", stationConfig.SSHPort)
		// LocalMode field doesn't exist in Config struct, skip this line
		environmentVariables["STATION_DEBUG"] = fmt.Sprintf("%t", stationConfig.Debug)
		environmentVariables["STATION_TELEMETRY_ENABLED"] = fmt.Sprintf("%t", stationConfig.TelemetryEnabled)
		environmentVariables["STATION_ADMIN_USERNAME"] = stationConfig.AdminUsername

		// Include encryption key for proper Station operation (get from viper since not in Config struct)
		encryptionKey := viper.GetString("encryption_key")
		if encryptionKey != "" {
			environmentVariables["STATION_ENCRYPTION_KEY"] = encryptionKey
		}

		// Add CloudShip configuration placeholders for Docker runtime configuration
		environmentVariables["STN_CLOUDSHIP_ENABLED"] = "false"  // Default disabled, enable via runtime
		environmentVariables["STN_CLOUDSHIP_KEY"] = "<your-cloudship-registration-key>"
		environmentVariables["STN_CLOUDSHIP_ENDPOINT"] = "lighthouse.cloudship.ai:443"  // Default endpoint
		environmentVariables["STN_CLOUDSHIP_STATION_ID"] = ""  // Auto-generated on first connection
	}

	// Create build options
	buildOptions := &build.BuildOptions{
		Provider: req.Provider,
		Model:    req.Model,
	}

	// Build the Docker image
	builder := build.NewEnvironmentBuilderWithOptions(req.Environment, envPath, buildOptions)
	ctx := context.Background()

	imageID, err := builder.Build(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to build Docker image: %v", err)})
		return
	}

	// Return success response with image ID and environment variables
	c.JSON(http.StatusOK, gin.H{
		"success":               true,
		"message":              "Docker image built successfully",
		"image_id":             imageID,
		"image_name":           req.ImageName,
		"tag":                  req.Tag,
		"environment_variables": environmentVariables,
	})
}

// getEnvironmentVariables returns the variables.yml content for an environment
func (h *APIHandlers) getEnvironmentVariables(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	// Get environment name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Get station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get station config root"})
		return
	}

	envPath := filepath.Join(configRoot, "environments", env.Name)
	variablesPath := filepath.Join(envPath, "variables.yml")

	// Read file content as string
	content, err := os.ReadFile(variablesPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty content if file doesn't exist
			c.JSON(http.StatusOK, gin.H{
				"content": "",
				"path":    variablesPath,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read variables file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
		"path":    variablesPath,
	})
}

// updateEnvironmentVariables updates the variables.yml content for an environment
func (h *APIHandlers) updateEnvironmentVariables(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get environment name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Validate YAML syntax
	var test map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.Content), &test); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid YAML syntax: %v", err)})
		return
	}

	// Get station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get station config root"})
		return
	}

	envPath := filepath.Join(configRoot, "environments", env.Name)
	variablesPath := filepath.Join(envPath, "variables.yml")

	if err := os.WriteFile(variablesPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write variables file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Variables updated successfully. Run 'stn sync' to apply changes.",
		"path":    variablesPath,
	})
}

// generateDeploymentTemplate generates deployment template for the specified provider
func (h *APIHandlers) generateDeploymentTemplate(c *gin.Context) {
	envID, err := strconv.ParseInt(c.Param("env_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid environment ID"})
		return
	}

	var req struct {
		Provider    string `json:"provider" binding:"required"` // aws-ecs, gcp-cloudrun, fly, docker-compose
		DockerImage string `json:"docker_image" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Get environment name
	env, err := h.repos.Environments.GetByID(envID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Environment not found"})
		return
	}

	// Get station config root
	configRoot, err := common.GetStationConfigRoot()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get station config root"})
		return
	}

	// Read config.yaml
	configPath := filepath.Join(configRoot, "config.yaml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read config.yaml"})
		return
	}

	// Read environment variables.yml
	envPath := filepath.Join(configRoot, "environments", env.Name)
	variablesPath := filepath.Join(envPath, "variables.yml")
	variablesContent := ""
	if data, err := os.ReadFile(variablesPath); err == nil {
		variablesContent = string(data)
	}

	// Load deployment config from Station config
	deployConfig, err := deployment.LoadConfigFromYAML(
		string(configContent),
		variablesContent,
		env.Name,
		req.DockerImage,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to parse config: %v", err)})
		return
	}

	// Generate deployment template
	template, err := deployment.GenerateDeploymentTemplate(req.Provider, *deployConfig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate template: %v", err)})
		return
	}

	// Determine filename
	filename := ""
	switch req.Provider {
	case "aws-ecs":
		filename = fmt.Sprintf("station-%s-ecs.yml", env.Name)
	case "gcp-cloudrun":
		filename = fmt.Sprintf("station-%s-cloudrun.yml", env.Name)
	case "fly":
		filename = "fly.toml"
	case "docker-compose":
		filename = "docker-compose.yml"
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"template": template,
		"filename": filename,
		"provider": req.Provider,
	})
}