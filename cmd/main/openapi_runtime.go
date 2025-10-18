package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"station/pkg/openapi/runtime"

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

func runOpenAPIRuntime(cmd *cobra.Command, args []string) error {
	// Create the runtime server
	server := runtime.NewServer(runtime.ServerConfig{})

	// Set up stdio communication
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	// Log to stderr to avoid interfering with JSON-RPC
	log.SetOutput(os.Stderr)
	log.Println("OpenAPI Runtime MCP Server started")

	// Main loop - read JSON-RPC requests from stdin
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}

		var req MCPRequest
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		response := handleRequest(server, req)
		if err := encoder.Encode(response); err != nil {
			log.Printf("Error sending response: %v", err)
		}
	}

	return nil
}

func handleRequest(server *runtime.Server, req MCPRequest) MCPResponse {
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
		}

		// Add server info
		serverInfo := server.GetServerInfo()
		result["serverInfo"] = serverInfo

		response.Result = result

	case "tools/list":
		// Return tools from the runtime server
		response.Result = map[string]interface{}{
			"tools": server.ListTools(),
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

		// Call the tool using the runtime server
		result, err := server.CallTool(toolName, arguments)
		if err != nil {
			response.Error = &MCPError{
				Code:    -32603,
				Message: err.Error(),
			}
			return response
		}
		response.Result = result

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