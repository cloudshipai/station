package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

// Environment Management Handlers
// Handles environment operations: list, create, delete

func (s *Server) handleListEnvironments(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environments, err := s.repos.Environments.List()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list environments: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":      true,
		"environments": environments,
		"count":        len(environments),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleCreateEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	description := request.GetString("description", "")

	// Get console user for created_by field
	consoleUser, err := s.repos.Users.GetByUsername("console")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get console user: %v", err)), nil
	}

	// Create database entry
	var desc *string
	if description != "" {
		desc = &description
	}

	env, err := s.repos.Environments.Create(name, desc, consoleUser.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create environment: %v", err)), nil
	}

	// Create file-based environment directory structure
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get user home directory: %v", err)), nil
	}

	envDir := filepath.Join(homeDir, ".config", "station", "environments", name)
	agentsDir := filepath.Join(envDir, "agents")

	// Create environment directory structure
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		// Try to cleanup database entry if directory creation fails
		s.repos.Environments.Delete(env.ID)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create environment directory: %v", err)), nil
	}

	// Create default variables.yml file
	variablesPath := filepath.Join(envDir, "variables.yml")
	defaultVariables := fmt.Sprintf("# Environment variables for %s\n# Add your template variables here\n# Example:\n# DATABASE_URL: \"your-database-url\"\n# API_KEY: \"your-api-key\"\n", name)

	if err := os.WriteFile(variablesPath, []byte(defaultVariables), 0644); err != nil {
		// Try to cleanup if variables file creation fails
		os.RemoveAll(envDir)
		s.repos.Environments.Delete(env.ID)
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create variables.yml: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":        true,
		"environment":    env,
		"directory_path": envDir,
		"variables_path": variablesPath,
		"message":        fmt.Sprintf("Environment '%s' created successfully", name),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleDeleteEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	confirm, err := request.RequireBool("confirm")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'confirm' parameter: %v", err)), nil
	}

	if !confirm {
		return mcp.NewToolResultError("Confirmation required: set 'confirm' to true to proceed"), nil
	}

	// Get environment by name
	env, err := s.repos.Environments.GetByName(name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Environment '%s' not found: %v", name, err)), nil
	}

	// Prevent deletion of default environment
	if env.Name == "default" {
		return mcp.NewToolResultError("Cannot delete the default environment"), nil
	}

	// Delete file-based configuration first
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get user home directory: %v", err)), nil
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
	response := map[string]interface{}{
		"success":          dbDeleteError == nil,
		"environment":      env.Name,
		"database_deleted": dbDeleteError == nil,
		"files_deleted":    fileCleanupError == nil,
	}

	if dbDeleteError != nil {
		response["database_error"] = dbDeleteError.Error()
	}

	if fileCleanupError != nil {
		response["file_cleanup_error"] = fileCleanupError.Error()
	}

	if dbDeleteError == nil && fileCleanupError == nil {
		response["message"] = fmt.Sprintf("Environment '%s' deleted successfully", name)
	} else if dbDeleteError == nil {
		response["message"] = fmt.Sprintf("Environment '%s' deleted from database, but file cleanup failed", name)
	} else {
		response["message"] = fmt.Sprintf("Failed to delete environment '%s'", name)
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleCreateBundleFromEnvironment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError("Bundle creation functionality not implemented in MCP mode"), nil
}