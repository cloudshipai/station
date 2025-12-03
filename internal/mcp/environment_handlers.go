package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"station/internal/services"
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

	// Use unified environment management service
	envService := services.NewEnvironmentManagementService(s.repos)
	env, result, err := envService.CreateEnvironment(name, desc, consoleUser.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create environment: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     result.Success,
		"environment": env,
		"result":      result,
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

	// Use unified environment management service
	envService := services.NewEnvironmentManagementService(s.repos)
	result := envService.DeleteEnvironment(name)

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleCreateBundleFromEnvironment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := req.RequireString("environmentName")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environmentName' parameter: %v", err)), nil
	}

	outputPath := req.GetString("outputPath", "")

	// Use the unified bundle handler (which now has database access for reports)
	bundleReq := BundleEnvironmentRequest{
		EnvironmentName: environmentName,
		OutputPath:      outputPath,
	}

	response, err := s.bundleHandler.CreateBundle(ctx, bundleReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Bundle creation failed: %v", err)), nil
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
