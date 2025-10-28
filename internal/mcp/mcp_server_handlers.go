package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"station/internal/services"
)

// MCP Server Management Handlers
// Handles MCP server operations: list, add, update, delete

func (s *Server) handleListMCPServersForEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	mcpService := services.NewMCPServerManagementService(s.repos)
	servers, err := mcpService.GetMCPServersForEnvironment(environmentName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get MCP servers: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"servers":     servers,
		"count":       len(servers),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleAddMCPServerToEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	serverName, err := request.RequireString("server_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'server_name' parameter: %v", err)), nil
	}

	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'command' parameter: %v", err)), nil
	}

	// Optional parameters
	description := request.GetString("description", "")

	// Parse args and env from Arguments
	var args []string
	var envVars map[string]string

	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			// Parse args array
			if argsInterface, exists := argsMap["args"]; exists {
				if argsSlice, ok := argsInterface.([]interface{}); ok {
					for _, arg := range argsSlice {
						if argStr, ok := arg.(string); ok {
							args = append(args, argStr)
						}
					}
				}
			}

			// Parse env object
			if envInterface, exists := argsMap["env"]; exists {
				if envMap, ok := envInterface.(map[string]interface{}); ok {
					envVars = make(map[string]string)
					for key, value := range envMap {
						if valueStr, ok := value.(string); ok {
							envVars[key] = valueStr
						}
					}
				}
			}
		}
	}

	// Create server config
	serverConfig := services.MCPServerConfig{
		Name:        serverName,
		Description: description,
		Command:     command,
		Args:        args,
		Env:         envVars,
	}

	mcpService := services.NewMCPServerManagementService(s.repos)
	result := mcpService.AddMCPServerToEnvironment(environmentName, serverName, serverConfig)

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleUpdateMCPServerInEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	serverName, err := request.RequireString("server_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'server_name' parameter: %v", err)), nil
	}

	command, err := request.RequireString("command")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'command' parameter: %v", err)), nil
	}

	// Optional parameters
	description := request.GetString("description", "")

	// Parse args and env from Arguments
	var args []string
	var envVars map[string]string

	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			// Parse args array
			if argsInterface, exists := argsMap["args"]; exists {
				if argsSlice, ok := argsInterface.([]interface{}); ok {
					for _, arg := range argsSlice {
						if argStr, ok := arg.(string); ok {
							args = append(args, argStr)
						}
					}
				}
			}

			// Parse env object
			if envInterface, exists := argsMap["env"]; exists {
				if envMap, ok := envInterface.(map[string]interface{}); ok {
					envVars = make(map[string]string)
					for key, value := range envMap {
						if valueStr, ok := value.(string); ok {
							envVars[key] = valueStr
						}
					}
				}
			}
		}
	}

	// Create server config
	serverConfig := services.MCPServerConfig{
		Name:        serverName,
		Description: description,
		Command:     command,
		Args:        args,
		Env:         envVars,
	}

	mcpService := services.NewMCPServerManagementService(s.repos)
	result := mcpService.UpdateMCPServerInEnvironment(environmentName, serverName, serverConfig)

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleDeleteMCPServerFromEnvironment(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	serverName, err := request.RequireString("server_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'server_name' parameter: %v", err)), nil
	}

	mcpService := services.NewMCPServerManagementService(s.repos)
	result := mcpService.DeleteMCPServerFromEnvironment(environmentName, serverName)

	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetRawMCPConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	mcpService := services.NewMCPServerManagementService(s.repos)
	content, err := mcpService.GetRawMCPConfig(environmentName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get raw MCP config: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"content":     content,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleUpdateRawMCPConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	content, err := request.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'content' parameter: %v", err)), nil
	}

	mcpService := services.NewMCPServerManagementService(s.repos)
	err = mcpService.UpdateRawMCPConfig(environmentName, content)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update raw MCP config: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"message":     "Raw MCP config updated successfully",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetEnvironmentFileConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	envService := services.NewEnvironmentManagementService(s.repos)
	config, err := envService.GetEnvironmentFileConfig(environmentName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environment file config: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"config":      config,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleUpdateEnvironmentFileConfig(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	filename, err := request.RequireString("filename")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'filename' parameter: %v", err)), nil
	}

	content, err := request.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'content' parameter: %v", err)), nil
	}

	envService := services.NewEnvironmentManagementService(s.repos)
	err = envService.UpdateEnvironmentFileConfig(environmentName, filename, content)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update environment file config: %v", err)), nil
	}

	response := map[string]interface{}{
		"success":     true,
		"environment": environmentName,
		"filename":    filename,
		"message":     "Environment file config updated successfully",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
