package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"station/pkg/openapi"
	"station/pkg/openapi/runtime"
)

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// OpenAPIServer manages multiple OpenAPI specs in one MCP server
type OpenAPIServer struct {
	servers map[string]*runtime.Server // Map of spec name to server
	tools   []map[string]interface{}   // Combined list of all tools
}

func main() {
	// The server should be invoked with the environment directory as argument
	// It will scan for all *.openapi.json files in that directory
	if len(os.Args) < 2 {
		log.Fatal("Usage: openapi-mcp-server <environment-dir>")
	}

	envDir := os.Args[1]
	server := &OpenAPIServer{
		servers: make(map[string]*runtime.Server),
		tools:   []map[string]interface{}{},
	}

	// Load all OpenAPI specs from the environment directory
	if err := server.loadAllSpecs(envDir); err != nil {
		log.Fatalf("Failed to load OpenAPI specs: %v", err)
	}

	// Set up stdio communication
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	// Log to stderr to avoid interfering with JSON-RPC
	log.SetOutput(os.Stderr)
	log.Printf("OpenAPI MCP Server started with %d tools from %d specs", len(server.tools), len(server.servers))

	// Main loop - read JSON-RPC requests from stdin
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading input: %v", err)
			continue
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		response := server.handleRequest(req)
		if err := encoder.Encode(response); err != nil {
			log.Printf("Error sending response: %v", err)
		}
	}
}

func (s *OpenAPIServer) loadAllSpecs(envDir string) error {
	// Find all *.openapi.json files
	pattern := filepath.Join(envDir, "*.openapi.json")
	specFiles, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to scan for OpenAPI specs: %w", err)
	}

	if len(specFiles) == 0 {
		log.Printf("No OpenAPI specs found in %s", envDir)
		return nil
	}

	// Load each spec
	for _, specFile := range specFiles {
		specName := filepath.Base(specFile)
		specName = strings.TrimSuffix(specName, ".openapi.json")

		log.Printf("Loading OpenAPI spec: %s", specName)

		// Read the spec file
		specData, err := os.ReadFile(specFile)
		if err != nil {
			log.Printf("Failed to read %s: %v", specFile, err)
			continue
		}

		// Parse and convert the OpenAPI spec
		if err := s.loadSpec(specName, string(specData)); err != nil {
			log.Printf("Failed to load spec %s: %v", specName, err)
			continue
		}
	}

	return nil
}

func (s *OpenAPIServer) loadSpec(specName string, specContent string) error {
	// Create OpenAPI service to convert the spec
	svc := openapi.NewService()

	// Validate the spec
	if err := svc.ValidateSpec(specContent); err != nil {
		return fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	// Convert to MCP configuration
	options := openapi.ConvertOptions{
		ServerName:     specName,
		ToolNamePrefix: specName,
	}

	stationConfig, err := svc.ConvertFromSpec(specContent, options)
	if err != nil {
		return fmt.Errorf("failed to convert spec: %w", err)
	}

	// Parse the Station config to get the embedded MCP config
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(stationConfig), &config); err != nil {
		return fmt.Errorf("failed to parse station config: %w", err)
	}

	// Extract the MCP config from the environment variable
	env, ok := config["env"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no env in station config")
	}

	mcpConfigYAML, ok := env["OPENAPI_MCP_CONFIG"].(string)
	if !ok {
		return fmt.Errorf("no OPENAPI_MCP_CONFIG in env")
	}

	// Create a runtime server for this spec
	server := runtime.NewServer(runtime.ServerConfig{
		ConfigData: mcpConfigYAML,
	})

	s.servers[specName] = server

	// Add tools from this server to our combined list
	tools := server.ListTools()
	s.tools = append(s.tools, tools...)

	log.Printf("Loaded %d tools from %s", len(tools), specName)
	return nil
}

func (s *OpenAPIServer) handleRequest(req MCPRequest) MCPResponse {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		result := map[string]interface{}{
			"protocolVersion": "0.1.0",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "openapi-mcp-server",
				"version": "1.0.0",
			},
		}
		response.Result = result

	case "tools/list":
		// Return all tools from all specs
		response.Result = map[string]interface{}{
			"tools": s.tools,
		}

	case "tools/call":
		// Extract parameters
		var params map[string]interface{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			response.Error = &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			}
			return response
		}

		toolName, ok := params["name"].(string)
		if !ok {
			response.Error = &MCPError{
				Code:    -32602,
				Message: "Missing tool name",
			}
			return response
		}

		arguments, _ := params["arguments"].(map[string]interface{})

		// Find which server has this tool
		for specName, server := range s.servers {
			tools := server.ListTools()
			for _, tool := range tools {
				if toolInfo, ok := tool["name"].(string); ok && toolInfo == toolName {
					// Found the server that handles this tool
					log.Printf("Executing tool %s from spec %s", toolName, specName)
					result, err := server.CallTool(toolName, arguments)
					if err != nil {
						response.Error = &MCPError{
							Code:    -32603,
							Message: err.Error(),
						}
						return response
					}
					response.Result = result
					return response
				}
			}
		}

		response.Error = &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Tool not found: %s", toolName),
		}

	case "notifications/initialized":
		// Client has finished initialization
		return MCPResponse{} // No response for notifications

	default:
		response.Error = &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	return response
}
