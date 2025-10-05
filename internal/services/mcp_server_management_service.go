package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	Command     string                 `json:"command"`
	Args        []string               `json:"args,omitempty"`
	Env         map[string]string      `json:"env,omitempty"`
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