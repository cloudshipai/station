package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"station/internal/config"
	"station/pkg/openapi/runtime"
	"strings"
	"text/template"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var openapiRuntimeCmd = &cobra.Command{
	Use:    "openapi-runtime",
	Short:  "Run an OpenAPI-based MCP server",
	Hidden: true, // Hidden since it's only used internally by Station
	RunE:   runOpenAPIRuntime,
}

func init() {
	openapiRuntimeCmd.Flags().String("spec", "", "Path to OpenAPI specification file (.openapi.json)")
	rootCmd.AddCommand(openapiRuntimeCmd)
}

func runOpenAPIRuntime(cmd *cobra.Command, args []string) error {
	// Log to stderr to avoid interfering with stdio protocol
	log.SetOutput(os.Stderr)
	log.Println("=== OpenAPI Runtime MCP Server starting ===")
	log.Printf("Args: %v", args)

	// Get the spec file path from flags
	specPath, _ := cmd.Flags().GetString("spec")

	// Create server config - either from file path or environment variable (backwards compat)
	var serverConfig runtime.ServerConfig
	if specPath != "" {
		// Resolve relative path using centralized config directory
		// This ensures it works in both host and container environments
		var fullPath string
		if filepath.IsAbs(specPath) {
			fullPath = specPath
		} else {
			// Relative path - resolve from station config directory
			configDir := config.GetStationConfigDir()
			fullPath = filepath.Join(configDir, specPath)
		}

		log.Printf("Loading OpenAPI spec from file: %s", fullPath)

		// Read the OpenAPI spec file
		specData, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("failed to read OpenAPI spec: %w", err)
		}

		variables, processedSpec, err := processTemplateVariablesWithVars(string(specData), fullPath)
		if err != nil {
			log.Printf("Warning: Failed to process template variables: %v", err)
			log.Printf("Continuing with unprocessed spec...")
			processedSpec = string(specData)
			variables = make(map[string]string)
		} else {
			log.Printf("Successfully processed template variables in OpenAPI spec")
		}

		serverConfig = runtime.ServerConfig{
			ConfigData: processedSpec,
			Variables:  variables,
		}
	} else {
		// Fallback to inline config from environment (backwards compatibility)
		log.Println("No --spec flag provided, checking OPENAPI_MCP_CONFIG environment variable")
		serverConfig = runtime.ServerConfig{}
	}

	// Create the OpenAPI runtime server
	log.Println("Creating OpenAPI server...")
	openapiServer := runtime.NewServer(serverConfig)
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

func processTemplateVariablesWithVars(specContent, specPath string) (map[string]string, string, error) {
	envName := "default"
	if strings.Contains(specPath, "environments/") {
		parts := strings.Split(specPath, "environments/")
		if len(parts) > 1 {
			envParts := strings.Split(parts[1], "/")
			if len(envParts) > 0 && envParts[0] != "" {
				envName = envParts[0]
			}
		}
	}

	log.Printf("Loading variables for environment: %s", envName)

	variables, err := loadEnvironmentVariables(envName)
	if err != nil {
		log.Printf("No variables.yml found for environment %s, using environment variables only", envName)
		variables = make(map[string]string)
	}

	for _, envPair := range os.Environ() {
		parts := strings.SplitN(envPair, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]
			if isSystemEnvVar(key) {
				continue
			}
			variables[key] = value
		}
	}

	log.Printf("Loaded %d variables for template processing", len(variables))

	rendered, err := renderTemplate(specContent, variables)
	if err != nil {
		return nil, "", err
	}
	return variables, rendered, nil
}

// loadEnvironmentVariables loads variables from environment's variables.yml
func loadEnvironmentVariables(envName string) (map[string]string, error) {
	variablesPath := config.GetVariablesPath(envName)

	data, err := os.ReadFile(variablesPath)
	if err != nil {
		return nil, err
	}

	var variables map[string]interface{}
	if err := yaml.Unmarshal(data, &variables); err != nil {
		return nil, err
	}

	// Convert to string map
	stringVars := make(map[string]string)
	for key, value := range variables {
		stringVars[key] = fmt.Sprintf("%v", value)
	}

	return stringVars, nil
}

// renderTemplate renders a template with the given variables
func renderTemplate(templateContent string, variables map[string]string) (string, error) {
	// Create a new template with the content
	tmpl, err := template.New("openapi-spec").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Convert variables to interface{} map for template execution
	templateData := make(map[string]interface{})
	for key, value := range variables {
		templateData[key] = value
	}

	// Execute the template with the variables
	var rendered bytes.Buffer
	if err := tmpl.Execute(&rendered, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return rendered.String(), nil
}

// isSystemEnvVar returns true if the variable name is a system/internal environment variable
func isSystemEnvVar(key string) bool {
	systemPrefixes := []string{
		"PATH", "HOME", "USER", "SHELL", "TERM", "LANG",
		"PWD", "OLDPWD", "SHLVL", "HOSTNAME", "HOSTTYPE",
		"_", "LS_COLORS", "GOPATH", "GOROOT", "GOCACHE",
	}

	for _, prefix := range systemPrefixes {
		if key == prefix || strings.HasPrefix(key, prefix+"_") {
			return true
		}
	}

	return false
}
