package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"station/pkg/openapi/runtime"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var openapiRuntimeCmd = &cobra.Command{
	Use:    "openapi-runtime",
	Short:  "Run an OpenAPI-based MCP server",
	Hidden: true, // Hidden since it's only used internally by Station
	RunE:   runOpenAPIRuntime,
}

func init() {
	rootCmd.AddCommand(openapiRuntimeCmd)
}

func runOpenAPIRuntime(cmd *cobra.Command, args []string) error {
	// Log to stderr to avoid interfering with stdio protocol
	log.SetOutput(os.Stderr)
	log.Println("=== OpenAPI Runtime MCP Server starting ===")
	log.Printf("Args: %v", args)
	log.Printf("Environment variables:")
	for _, env := range os.Environ() {
		if strings.Contains(env, "OPENAPI") {
			log.Printf("  %s", env)
		}
	}

	// Create the OpenAPI runtime server
	log.Println("Creating OpenAPI server...")
	openapiServer := runtime.NewServer(runtime.ServerConfig{})
	log.Println("OpenAPI server created successfully")

	// Get server info for MCP server creation
	serverInfo := openapiServer.GetServerInfo()

	// Extract name and version from serverInfo map
	serverName := "OpenAPI MCP Server"
	if name, ok := serverInfo["name"].(string); ok && name != "" {
		serverName = name
	}
	serverVersion := "1.0.0"
	if version, ok := serverInfo["version"].(string); ok && version != "" {
		serverVersion = version
	}

	// Create MCP server using the official mcp-go library (same as Station's stdio command)
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
	)

	// Register tools from OpenAPI runtime
	tools := openapiServer.ListTools()
	log.Printf("Registering %d OpenAPI tools with MCP server", len(tools))

	for _, toolData := range tools {
		// Extract tool information
		toolName := toolData["name"].(string)
		toolDescription := ""
		if desc, ok := toolData["description"].(string); ok {
			toolDescription = desc
		}

		// Capture tool name in closure for handler
		currentToolName := toolName

		// Create MCP tool with input schema if available
		toolOptions := []mcp.ToolOption{
			mcp.WithDescription(toolDescription),
		}

		// Add parameters from input schema if present
		if inputSchema, ok := toolData["inputSchema"].(map[string]interface{}); ok {
			if properties, ok := inputSchema["properties"].(map[string]interface{}); ok {
				// Get required fields
				requiredFields := make(map[string]bool)
				if required, ok := inputSchema["required"].([]interface{}); ok {
					for _, field := range required {
						if fieldName, ok := field.(string); ok {
							requiredFields[fieldName] = true
						}
					}
				}

				// Add each parameter
				for paramName, paramDefRaw := range properties {
					if paramDef, ok := paramDefRaw.(map[string]interface{}); ok {
						paramDesc := ""
						if desc, ok := paramDef["description"].(string); ok {
							paramDesc = desc
						}

						paramType := "string"
						if pType, ok := paramDef["type"].(string); ok {
							paramType = pType
						}

						// Build parameter options (PropertyOption, not ToolOption)
						var paramOptions []mcp.PropertyOption
						if paramDesc != "" {
							paramOptions = append(paramOptions, mcp.Description(paramDesc))
						}
						if requiredFields[paramName] {
							paramOptions = append(paramOptions, mcp.Required())
						}

						// Add parameter based on type
						switch paramType {
						case "string":
							toolOptions = append(toolOptions, mcp.WithString(paramName, paramOptions...))
						case "number", "integer":
							toolOptions = append(toolOptions, mcp.WithNumber(paramName, paramOptions...))
						case "boolean":
							toolOptions = append(toolOptions, mcp.WithBoolean(paramName, paramOptions...))
						default:
							// For complex types, use string as fallback
							toolOptions = append(toolOptions, mcp.WithString(paramName, paramOptions...))
						}
					}
				}
			}
		}

		tool := mcp.NewTool(toolName, toolOptions...)

		// Create handler function
		handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Convert MCP tool request to OpenAPI runtime format
			arguments := make(map[string]interface{})
			if request.Params.Arguments != nil {
				if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
					arguments = argsMap
				}
			}

			// Call the OpenAPI runtime tool
			result, err := openapiServer.CallTool(currentToolName, arguments)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("tool execution failed: %v", err)), nil
			}

			// Convert result to MCP response format
			return mcp.NewToolResultText(fmt.Sprintf("%v", result)), nil
		}

		// Register tool with MCP server (same as Station does)
		mcpServer.AddTool(tool, handler)
	}

	log.Printf("=== OpenAPI Runtime MCP Server registered %d tools ===", len(tools))
	log.Println("Starting ServeStdio...")
	log.Println("MCP Server is now ready to accept stdio communication")

	// Use the mcp-go ServeStdio convenience function (same as Station)
	if err := server.ServeStdio(mcpServer); err != nil {
		log.Printf("ERROR: ServeStdio failed: %v", err)
		return fmt.Errorf("MCP stdio server error: %w", err)
	}

	log.Println("ServeStdio completed normally")
	return nil
}