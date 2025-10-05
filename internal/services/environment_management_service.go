package services

import (
	"fmt"
	"os"
	"path/filepath"

	"station/internal/config"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// EnvironmentManagementService provides unified environment operations
// Used by MCP, API, and UI handlers to ensure consistent behavior
type EnvironmentManagementService struct {
	repos *repositories.Repositories
}

// NewEnvironmentManagementService creates a new environment management service
func NewEnvironmentManagementService(repos *repositories.Repositories) *EnvironmentManagementService {
	return &EnvironmentManagementService{
		repos: repos,
	}
}

// EnvironmentOperationResult contains the result of environment operations
type EnvironmentOperationResult struct {
	Success          bool   `json:"success"`
	Environment      string `json:"environment,omitempty"`
	DatabaseDeleted  bool   `json:"database_deleted,omitempty"`
	FilesDeleted     bool   `json:"files_deleted,omitempty"`
	DatabaseError    string `json:"database_error,omitempty"`
	FileCleanupError string `json:"file_cleanup_error,omitempty"`
	Message          string `json:"message"`
	DirectoryPath    string `json:"directory_path,omitempty"`
	VariablesPath    string `json:"variables_path,omitempty"`
}

// CreateEnvironment creates both database entry and file structure
func (s *EnvironmentManagementService) CreateEnvironment(name string, description *string, createdByUserID int64) (*models.Environment, *EnvironmentOperationResult, error) {
	// Create database entry first
	env, err := s.repos.Environments.Create(name, description, createdByUserID)
	if err != nil {
		return nil, &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create environment: %v", err),
		}, err
	}

	// Create file-based environment directory structure
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Cleanup database entry if directory creation fails
		s.repos.Environments.Delete(env.ID)
		return nil, &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get user home directory: %v", err),
		}, err
	}

	envDir := filepath.Join(homeDir, ".config", "station", "environments", name)
	agentsDir := filepath.Join(envDir, "agents")

	// Create environment directory structure
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		// Cleanup database entry if directory creation fails
		s.repos.Environments.Delete(env.ID)
		return nil, &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create environment directory: %v", err),
		}, err
	}

	// Create default variables.yml file
	variablesPath := filepath.Join(envDir, "variables.yml")
	defaultVariables := fmt.Sprintf("# Environment variables for %s\n# Add your template variables here\n# Example:\n# DATABASE_URL: \"your-database-url\"\n# API_KEY: \"your-api-key\"\n", name)

	if err := os.WriteFile(variablesPath, []byte(defaultVariables), 0644); err != nil {
		// Cleanup if variables file creation fails
		os.RemoveAll(envDir)
		s.repos.Environments.Delete(env.ID)
		return nil, &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to create variables.yml: %v", err),
		}, err
	}

	result := &EnvironmentOperationResult{
		Success:       true,
		Environment:   name,
		DirectoryPath: envDir,
		VariablesPath: variablesPath,
		Message:       fmt.Sprintf("Environment '%s' created successfully", name),
	}

	return env, result, nil
}

// DeleteEnvironment deletes both database entry and file structure
func (s *EnvironmentManagementService) DeleteEnvironment(name string) *EnvironmentOperationResult {
	// Get environment by name
	env, err := s.repos.Environments.GetByName(name)
	if err != nil {
		return &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Environment '%s' not found: %v", name, err),
		}
	}

	// Prevent deletion of default environment
	if env.Name == "default" {
		return &EnvironmentOperationResult{
			Success: false,
			Message: "Cannot delete the default environment",
		}
	}

	// Delete file-based configuration first
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Failed to get user home directory: %v", err),
		}
	}

	envDir := filepath.Join(homeDir, ".config", "station", "environments", name)

	// Remove environment directory and all contents
	var fileCleanupError error
	if _, err := os.Stat(envDir); err == nil {
		fileCleanupError = os.RemoveAll(envDir)
	}

	// Delete database entries (this also deletes associated agents, runs, etc. via foreign key constraints)
	dbDeleteError := s.repos.Environments.Delete(env.ID)

	// Prepare response with cleanup status
	result := &EnvironmentOperationResult{
		Success:         dbDeleteError == nil,
		Environment:     env.Name,
		DatabaseDeleted: dbDeleteError == nil,
		FilesDeleted:    fileCleanupError == nil,
	}

	if dbDeleteError != nil {
		result.DatabaseError = dbDeleteError.Error()
	}

	if fileCleanupError != nil {
		result.FileCleanupError = fileCleanupError.Error()
	}

	if dbDeleteError == nil && fileCleanupError == nil {
		result.Message = fmt.Sprintf("Environment '%s' deleted successfully", name)
	} else if dbDeleteError == nil {
		result.Message = fmt.Sprintf("Environment '%s' deleted from database, but file cleanup failed", name)
	} else {
		result.Message = fmt.Sprintf("Failed to delete environment '%s'", name)
	}

	return result
}

// DeleteEnvironmentByID deletes environment by ID (for API compatibility)
func (s *EnvironmentManagementService) DeleteEnvironmentByID(id int64) *EnvironmentOperationResult {
	// Get environment by ID to get the name
	env, err := s.repos.Environments.GetByID(id)
	if err != nil {
		return &EnvironmentOperationResult{
			Success: false,
			Message: fmt.Sprintf("Environment with ID %d not found: %v", id, err),
		}
	}

	// Use the name-based deletion logic
	return s.DeleteEnvironment(env.Name)
}

// GetEnvironmentFileConfig reads the raw file-based config for an environment
func (s *EnvironmentManagementService) GetEnvironmentFileConfig(name string) (map[string]interface{}, error) {
	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(name)

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("environment '%s' not found", name)
	}

	config := make(map[string]interface{})

	// Read variables.yml
	variablesPath := filepath.Join(envDir, "variables.yml")
	if _, err := os.Stat(variablesPath); err == nil {
		variablesData, err := os.ReadFile(variablesPath)
		if err == nil {
			config["variables_yml"] = string(variablesData)
		}
	}

	// Read template.json if it exists
	templatePath := filepath.Join(envDir, "template.json")
	if _, err := os.Stat(templatePath); err == nil {
		templateData, err := os.ReadFile(templatePath)
		if err == nil {
			config["template_json"] = string(templateData)
		}
	}

	// List agents directory
	agentsDir := filepath.Join(envDir, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil {
		agents := make([]string, 0)
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".prompt" {
				agents = append(agents, entry.Name())
			}
		}
		config["agents"] = agents
	}

	config["directory_path"] = envDir

	return config, nil
}

// UpdateEnvironmentFileConfig updates file-based config for an environment
func (s *EnvironmentManagementService) UpdateEnvironmentFileConfig(name, filename, content string) error {
	// Use centralized path resolution for container/host compatibility
	envDir := config.GetEnvironmentDir(name)

	// Check if environment directory exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return fmt.Errorf("environment '%s' not found", name)
	}

	// Validate filename to prevent directory traversal
	if filepath.Base(filename) != filename {
		return fmt.Errorf("invalid filename: %s", filename)
	}

	// Only allow specific files
	allowedFiles := map[string]bool{
		"variables.yml": true,
		"template.json": true,
	}

	if !allowedFiles[filename] {
		return fmt.Errorf("editing file '%s' is not allowed", filename)
	}

	filePath := filepath.Join(envDir, filename)
	return os.WriteFile(filePath, []byte(content), 0644)
}