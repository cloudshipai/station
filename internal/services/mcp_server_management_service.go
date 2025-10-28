package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"station/internal/config"
	"station/internal/db/repositories"
)

// MCPServerManagementService provides unified MCP server operations
// Used by MCP, API, and UI handlers to ensure consistent behavior
type MCPServerManagementService struct {
	repos *repositories.Repositories
}

// NewMCPServerManagementService creates a new MCP server management service
func NewMCPServerManagementService(repos *repositories.Repositories) *MCPServerManagementService {
	return &MCPServerManagementService{
		repos: repos,
	}
}

// MCPServerOperationResult contains the result of MCP server operations
type MCPServerOperationResult struct {
	Success          bool   `json:"success"`
	ServerName       string `json:"server_name,omitempty"`
	Environment      string `json:"environment,omitempty"`
	DatabaseDeleted  bool   `json:"database_deleted,omitempty"`
	FilesDeleted     bool   `json:"files_deleted,omitempty"`
	DatabaseError    string `json:"database_error,omitempty"`
	FileCleanupError string `json:"file_cleanup_error,omitempty"`
	Message          string `json:"message"`
}

// MCPServerConfig represents an MCP server configuration
type MCPServerConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Command     string                 `json:"command,omitempty"`
	Args        []string               `json:"args,omitempty"`
	Env         map[string]string      `json:"env,omitempty"`
	URL         string                 `json:"url,omitempty"`         // For HTTP-based servers
	Type        string                 `json:"type,omitempty"`        // "stdio" or "http"
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TemplateConfig represents the complete template.json structure
type TemplateConfig struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	MCPServers  map[string]MCPServerConfig    `json:"mcpServers"`
	Metadata    map[string]interface{}        `json:"metadata,omitempty"`
}

// SingleServerTemplate represents a single server template file
type SingleServerTemplate struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	MCPServers  map[string]MCPServerConfig    `json:"mcpServers"`
	Metadata    map[string]interface{}        `json:"metadata,omitempty"`
}

// GetMCPServersForEnvironment gets all MCP servers for an environment from individual files
func (s *MCPServerManagementService) GetMCPServersForEnvironment(environmentName string) (map[string]MCPServerConfig, error) {
	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(environmentName)

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return make(map[string]MCPServerConfig), nil // Return empty map if directory doesn't exist
	}

	// Read all JSON files in the environment directory
	files, err := filepath.Glob(filepath.Join(envDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read environment directory: %v", err)
	}

	result := make(map[string]MCPServerConfig)

	for _, filePath := range files {
		// Skip template.json if it exists (legacy file)
		fileName := filepath.Base(filePath)
		if fileName == "template.json" {
			continue
		}

		// Read the server file
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var singleServerTemplate SingleServerTemplate
		if err := json.Unmarshal(fileData, &singleServerTemplate); err != nil {
			continue // Skip files that can't be parsed
		}

		// Add all servers from this file to the result
		for serverName, serverConfig := range singleServerTemplate.MCPServers {
			result[serverName] = serverConfig
		}
	}

	return result, nil
}


// AddMCPServerToEnvironment adds an MCP server to an environment as a separate file
func (s *MCPServerManagementService) AddMCPServerToEnvironment(environmentName, serverName string, serverConfig MCPServerConfig) *MCPServerOperationResult {
	// Validate server name
	if serverName == "" {
		return &MCPServerOperationResult{
			Success: false,
			Message: "Server name cannot be empty",
		}
	}

	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(environmentName)

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Environment '%s' not found", environmentName),
		}
	}

	// Use the server config as-is (UI will provide Go template format directly)
	templateServerConfig := serverConfig

	// Create single server template
	singleServerTemplate := SingleServerTemplate{
		Name:        serverName,
		Description: fmt.Sprintf("MCP server configuration for %s", serverName),
		MCPServers:  map[string]MCPServerConfig{serverName: templateServerConfig},
	}

	// Write individual server file
	serverFilePath := filepath.Join(envDir, fmt.Sprintf("%s.json", serverName))
	templateData, err := json.MarshalIndent(singleServerTemplate, "", "  ")
	if err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to marshal server config: %v", err),
		}
	}

	if err := os.WriteFile(serverFilePath, templateData, 0644); err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to write server file: %v", err),
		}
	}

	return &MCPServerOperationResult{
		Success:     true,
		ServerName:  serverName,
		Environment: environmentName,
		Message:     fmt.Sprintf("MCP server '%s' added to environment '%s' as %s.json", serverName, environmentName, serverName),
	}
}

// UpdateMCPServerInEnvironment updates an MCP server in an environment's template.json
func (s *MCPServerManagementService) UpdateMCPServerInEnvironment(environmentName, serverName string, serverConfig MCPServerConfig) *MCPServerOperationResult {
	// First check if the server exists
	servers, err := s.GetMCPServersForEnvironment(environmentName)
	if err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get MCP servers: %v", err),
		}
	}

	if _, exists := servers[serverName]; !exists {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("MCP server '%s' not found in environment '%s'", serverName, environmentName),
		}
	}

	// Use AddMCPServerToEnvironment which handles both add and update
	result := s.AddMCPServerToEnvironment(environmentName, serverName, serverConfig)
	if result.Success {
		result.Message = fmt.Sprintf("MCP server '%s' updated in environment '%s'", serverName, environmentName)
	}

	return result
}

// DeleteMCPServerFromEnvironment removes an MCP server from an environment
func (s *MCPServerManagementService) DeleteMCPServerFromEnvironment(environmentName, serverName string) *MCPServerOperationResult {
	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(environmentName)
	serverFilePath := filepath.Join(envDir, fmt.Sprintf("%s.json", serverName))

	// Check if server file exists
	if _, err := os.Stat(serverFilePath); os.IsNotExist(err) {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("MCP server '%s' not found in environment '%s'", serverName, environmentName),
		}
	}

	// Also clean up associated database records
	var dbDeleteError error
	var fileCleanupError error

	// Get environment ID for database cleanup
	if env, err := s.repos.Environments.GetByName(environmentName); err == nil {
		// Delete file MCP config records
		if err := s.repos.FileMCPConfigs.DeleteByEnvironmentAndName(env.ID, serverName); err != nil {
			dbDeleteError = err
		}

		// Delete MCP tools associated with this server
		if mcpServers, err := s.repos.MCPServers.GetByEnvironmentID(env.ID); err == nil {
			for _, mcpServer := range mcpServers {
				if mcpServer.Name == serverName {
					// Delete tools for this server
					if err := s.repos.MCPTools.DeleteByServerID(mcpServer.ID); err != nil {
						dbDeleteError = err
					}
					// Delete the server record
					if err := s.repos.MCPServers.Delete(mcpServer.ID); err != nil {
						dbDeleteError = err
					}
					break
				}
			}
		}
	}

	// Delete the individual server file
	if err := os.Remove(serverFilePath); err != nil {
		fileCleanupError = err
	}

	// Check if this is an OpenAPI-based MCP server and delete source .openapi.json file
	if strings.HasSuffix(serverName, "-openapi-mcp") {
		// Extract the spec name (remove -openapi-mcp suffix)
		specName := strings.TrimSuffix(serverName, "-openapi-mcp")
		openapiFilePath := filepath.Join(envDir, fmt.Sprintf("%s.openapi.json", specName))

		// Delete the source OpenAPI spec file if it exists
		if _, err := os.Stat(openapiFilePath); err == nil {
			if err := os.Remove(openapiFilePath); err != nil {
				// Log but don't fail the operation if OpenAPI file deletion fails
				fileCleanupError = fmt.Errorf("MCP file deleted but OpenAPI spec deletion failed: %w", err)
			}
		}
	}

	// Prepare response
	result := &MCPServerOperationResult{
		Success:         fileCleanupError == nil,
		ServerName:      serverName,
		Environment:     environmentName,
		DatabaseDeleted: dbDeleteError == nil,
		FilesDeleted:    fileCleanupError == nil,
	}

	if dbDeleteError != nil {
		result.DatabaseError = dbDeleteError.Error()
	}

	if fileCleanupError != nil {
		result.FileCleanupError = fileCleanupError.Error()
	}

	if fileCleanupError == nil && dbDeleteError == nil {
		result.Message = fmt.Sprintf("MCP server '%s' deleted successfully from environment '%s'", serverName, environmentName)
	} else if fileCleanupError == nil {
		result.Message = fmt.Sprintf("MCP server '%s' removed from file, but database cleanup failed", serverName)
	} else {
		result.Message = fmt.Sprintf("Failed to delete MCP server '%s' from environment '%s'", serverName, environmentName)
	}

	return result
}

// GetRawMCPConfig returns the raw template.json content for an environment
func (s *MCPServerManagementService) GetRawMCPConfig(environmentName string) (string, error) {
	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(environmentName)

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return "", fmt.Errorf("environment '%s' not found", environmentName)
	}

	templatePath := filepath.Join(envDir, "template.json")

	// Check if template.json exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		// Return default empty template structure
		defaultTemplate := TemplateConfig{
			Name:        environmentName,
			Description: fmt.Sprintf("Environment configuration for %s", environmentName),
			MCPServers:  make(map[string]MCPServerConfig),
		}
		templateData, _ := json.MarshalIndent(defaultTemplate, "", "  ")
		return string(templateData), nil
	}

	// Read existing template.json
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template.json: %v", err)
	}

	return string(templateData), nil
}

// UpdateRawMCPConfig updates the raw template.json content for an environment
func (s *MCPServerManagementService) UpdateRawMCPConfig(environmentName, content string) error {
	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(environmentName)

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' not found", environmentName)
	}

	// Validate JSON before writing
	var templateConfig TemplateConfig
	if err := json.Unmarshal([]byte(content), &templateConfig); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	templatePath := filepath.Join(envDir, "template.json")
	return os.WriteFile(templatePath, []byte(content), 0644)
}

// MCPDirectoryTemplate represents a template from the mcp-servers/ directory
type MCPDirectoryTemplate struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	Category            string            `json:"category"`
	Command             string            `json:"command"`
	Args                []string          `json:"args"`
	Env                 map[string]string `json:"env,omitempty"`
	OpenAPISpec         string            `json:"openapiSpec,omitempty"`         // Name of the OpenAPI spec file
	RequiresOpenAPISpec bool              `json:"requiresOpenAPISpec,omitempty"` // Whether this template requires an OpenAPI spec
}

// GetMCPDirectoryTemplates reads all MCP server templates from the mcp-servers/ directory
func (s *MCPServerManagementService) GetMCPDirectoryTemplates() ([]MCPDirectoryTemplate, error) {
	// Get the mcp-servers directory path
	// Check multiple locations: embedded in binary location, system-wide, local dev
	mcpServersDirs := []string{
		"/usr/share/station/mcp-servers", // Docker/system installation
		"mcp-servers",                     // Local development
	}

	var mcpServersDir string
	for _, dir := range mcpServersDirs {
		if _, err := os.Stat(dir); err == nil {
			mcpServersDir = dir
			break
		}
	}

	// If no directory found, return empty array
	if mcpServersDir == "" {
		return []MCPDirectoryTemplate{}, nil
	}

	// Read all JSON files in the mcp-servers directory
	files, err := filepath.Glob(filepath.Join(mcpServersDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read mcp-servers directory: %v", err)
	}

	var templates []MCPDirectoryTemplate

	for _, filePath := range files {
		// Read the template file
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			continue // Skip files that can't be read
		}

		var singleServerTemplate SingleServerTemplate
		if err := json.Unmarshal(fileData, &singleServerTemplate); err != nil {
			continue // Skip files that can't be parsed
		}

		// Extract templates from this file
		for serverName, serverConfig := range singleServerTemplate.MCPServers {
			// Try to infer category from tags or default to "Community"
			category := "Community"
			if categoryVal, ok := serverConfig.Metadata["category"].(string); ok {
				category = categoryVal
			}

			// Check for OpenAPI spec metadata
			var openapiSpec string
			var requiresOpenAPISpec bool
			if singleServerTemplate.Metadata != nil {
				if specVal, ok := singleServerTemplate.Metadata["openapiSpec"].(string); ok {
					openapiSpec = specVal
				}
				if requiresVal, ok := singleServerTemplate.Metadata["requiresOpenAPISpec"].(bool); ok {
					requiresOpenAPISpec = requiresVal
				}
			}

			template := MCPDirectoryTemplate{
				ID:                  serverName,
				Name:                serverName,
				Description:         serverConfig.Description,
				Category:            category,
				Command:             serverConfig.Command,
				Args:                serverConfig.Args,
				Env:                 serverConfig.Env,
				OpenAPISpec:         openapiSpec,
				RequiresOpenAPISpec: requiresOpenAPISpec,
			}

			templates = append(templates, template)
		}
	}

	return templates, nil
}

// InstallOpenAPITemplate installs an OpenAPI-based MCP template
// This copies the OpenAPI spec file and creates/updates the variables.yml file
func (s *MCPServerManagementService) InstallOpenAPITemplate(environmentName, templateID, specFileName string) *MCPServerOperationResult {
	// Find the mcp-servers directory
	mcpServersDirs := []string{
		"/usr/share/station/mcp-servers", // Docker/system installation
		"mcp-servers",                     // Local development
	}

	var mcpServersDir string
	for _, dir := range mcpServersDirs {
		if _, err := os.Stat(dir); err == nil {
			mcpServersDir = dir
			break
		}
	}

	if mcpServersDir == "" {
		return &MCPServerOperationResult{
			Success: false,
			Message: "MCP servers directory not found",
		}
	}

	// Read the OpenAPI spec file from mcp-servers directory
	specSourcePath := filepath.Join(mcpServersDir, specFileName)
	specData, err := os.ReadFile(specSourcePath)
	if err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read OpenAPI spec: %v", err),
		}
	}

	// Copy the OpenAPI spec to the environment directory
	envDir := config.GetEnvironmentDir(environmentName)
	specDestPath := filepath.Join(envDir, specFileName)
	if err := os.WriteFile(specDestPath, specData, 0644); err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to copy OpenAPI spec to environment: %v", err),
		}
	}

	// Create/update variables.yml with default values
	variablesPath := filepath.Join(envDir, "variables.yml")
	variablesContent := fmt.Sprintf(`ENVIRONMENT_NAME: %s
STATION_API_URL: http://localhost:8585/api/v1
`, environmentName)

	// Check if variables.yml exists
	existingVars := make(map[string]interface{})
	if existingData, err := os.ReadFile(variablesPath); err == nil {
		// Parse existing variables
		if err := json.Unmarshal(existingData, &existingVars); err == nil {
			// Merge with new variables (don't overwrite existing)
			if _, hasEnvName := existingVars["ENVIRONMENT_NAME"]; !hasEnvName {
				existingVars["ENVIRONMENT_NAME"] = environmentName
			}
			if _, hasAPIURL := existingVars["STATION_API_URL"]; !hasAPIURL {
				existingVars["STATION_API_URL"] = "http://localhost:8585/api/v1"
			}
		}
	}

	// Write variables.yml (only if it doesn't exist or needs updating)
	if len(existingVars) == 0 {
		if err := os.WriteFile(variablesPath, []byte(variablesContent), 0644); err != nil {
			// Don't fail installation if variables file write fails
			fmt.Printf("Warning: Failed to write variables.yml: %v\n", err)
		}
	}

	return &MCPServerOperationResult{
		Success:     true,
		ServerName:  templateID,
		Environment: environmentName,
		Message:     fmt.Sprintf("OpenAPI spec '%s' installed to environment '%s'", specFileName, environmentName),
	}
}

// InstallTemplateFromDirectory installs a complete MCP template from the mcp-servers directory
// This preserves all metadata including description, tags, variables, etc.
func (s *MCPServerManagementService) InstallTemplateFromDirectory(environmentName, templateID, openapiSpecFile string) *MCPServerOperationResult {
	// Find the mcp-servers directory
	mcpServersDirs := []string{
		"/usr/share/station/mcp-servers", // Docker/system installation
		"mcp-servers",                     // Local development
	}

	var mcpServersDir string
	for _, dir := range mcpServersDirs {
		if _, err := os.Stat(dir); err == nil {
			mcpServersDir = dir
			break
		}
	}

	if mcpServersDir == "" {
		return &MCPServerOperationResult{
			Success: false,
			Message: "MCP servers directory not found",
		}
	}

	// Read the template JSON file from mcp-servers directory
	templateSourcePath := filepath.Join(mcpServersDir, fmt.Sprintf("%s.json", templateID))
	templateData, err := os.ReadFile(templateSourcePath)
	if err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to read template file: %v", err),
		}
	}

	// Copy the complete template to the environment directory
	envDir := config.GetEnvironmentDir(environmentName)
	templateDestPath := filepath.Join(envDir, fmt.Sprintf("%s.json", templateID))
	if err := os.WriteFile(templateDestPath, templateData, 0644); err != nil {
		return &MCPServerOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to copy template to environment: %v", err),
		}
	}

	// If this is an OpenAPI template, also copy the OpenAPI spec file
	if openapiSpecFile != "" {
		openapiResult := s.InstallOpenAPITemplate(environmentName, templateID, openapiSpecFile)
		if !openapiResult.Success {
			// Clean up the template file we just created
			os.Remove(templateDestPath)
			return openapiResult
		}
	}

	return &MCPServerOperationResult{
		Success:     true,
		ServerName:  templateID,
		Environment: environmentName,
		Message:     fmt.Sprintf("Template '%s' installed to environment '%s'", templateID, environmentName),
	}
}