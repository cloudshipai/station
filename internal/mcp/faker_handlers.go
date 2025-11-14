package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Faker Management Handlers
// Handles faker operations: create faker from existing MCP server

// TemplateConfig represents the structure of template.json
type TemplateConfig struct {
	Name        string                       `json:"name,omitempty"`
	Description string                       `json:"description,omitempty"`
	MCPServers  map[string]MCPServerTemplate `json:"mcpServers"`
}

// MCPServerTemplate represents an MCP server configuration in template.json
type MCPServerTemplate struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

func (s *Server) handleFakerCreateFromMCPServer(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Required parameters
	environmentName, err := request.RequireString("environment_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_name' parameter: %v", err)), nil
	}

	mcpServerName, err := request.RequireString("mcp_server_name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'mcp_server_name' parameter: %v", err)), nil
	}

	aiInstruction, err := request.RequireString("ai_instruction")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'ai_instruction' parameter: %v", err)), nil
	}

	// Optional parameters
	fakerName := request.GetString("faker_name", "")
	if fakerName == "" {
		fakerName = mcpServerName + "-faker"
	}

	debug := request.GetBool("debug", false)
	offline := request.GetBool("offline", false)

	// Get environment directory
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	envDir := filepath.Join(configHome, "station", "environments", environmentName)

	// Check environment exists
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("Environment '%s' does not exist at %s", environmentName, envDir)), nil
	}

	templatePath := filepath.Join(envDir, "template.json")

	// Read existing template.json
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read template.json: %v", err)), nil
	}

	var config TemplateConfig
	if err := json.Unmarshal(templateData, &config); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse template.json: %v", err)), nil
	}

	// Check if source MCP server exists
	sourceMCP, exists := config.MCPServers[mcpServerName]
	if !exists {
		availableServers := make([]string, 0, len(config.MCPServers))
		for name := range config.MCPServers {
			availableServers = append(availableServers, name)
		}
		return mcp.NewToolResultError(fmt.Sprintf("MCP server '%s' not found in environment '%s'\nAvailable servers: %s",
			mcpServerName, environmentName, strings.Join(availableServers, ", "))), nil
	}

	// Check if faker name already exists
	if _, exists := config.MCPServers[fakerName]; exists {
		return mcp.NewToolResultError(fmt.Sprintf("MCP server '%s' already exists in environment '%s'", fakerName, environmentName)), nil
	}

	// Get stn binary path
	stnPath, err := os.Executable()
	if err != nil {
		stnPath = "stn" // Fallback to PATH lookup
	}

	// Build faker MCP server configuration
	fakerMCP := MCPServerTemplate{
		Command: stnPath,
		Args:    buildFakerArgs(sourceMCP, aiInstruction, offline, debug),
	}

	// Add faker to config
	config.MCPServers[fakerName] = fakerMCP

	// Write updated template.json
	updatedData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal template.json: %v", err)), nil
	}

	if err := os.WriteFile(templatePath, updatedData, 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write template.json: %v", err)), nil
	}

	// Build success response
	response := map[string]interface{}{
		"success":      true,
		"message":      fmt.Sprintf("Created faker '%s' from MCP server '%s' in environment '%s'", fakerName, mcpServerName, environmentName),
		"environment":  environmentName,
		"source_mcp":   mcpServerName,
		"faker_name":   fakerName,
		"instruction":  aiInstruction,
		"configuration": map[string]interface{}{
			"command": fakerMCP.Command,
			"args":    fakerMCP.Args,
		},
		"next_steps": []string{
			fmt.Sprintf("Run 'stn sync %s' to discover faker tools", environmentName),
			fmt.Sprintf("The environment now has both '%s' (real) and '%s' (faker) MCP servers", mcpServerName, fakerName),
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// buildFakerArgs constructs the faker command arguments from source MCP config
func buildFakerArgs(sourceMCP MCPServerTemplate, instruction string, offline bool, debug bool) []string {
	args := []string{"faker"}

	// Add --command flag
	args = append(args, "--command", sourceMCP.Command)

	// Add --args flag if source has args
	if len(sourceMCP.Args) > 0 {
		argsStr := strings.Join(sourceMCP.Args, ",")
		args = append(args, "--args", argsStr)
	}

	// Add environment variables
	for key, value := range sourceMCP.Env {
		args = append(args, "--env", fmt.Sprintf("%s=%s", key, value))
	}

	// Add AI instruction
	args = append(args, "--ai-enabled")
	args = append(args, "--ai-instruction", instruction)

	// Add optional flags
	if offline {
		args = append(args, "--passthrough")
	}
	if debug {
		args = append(args, "--debug")
	}

	return args
}
