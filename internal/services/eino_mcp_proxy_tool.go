package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// EinoMCPProxyTool adapts MCP tools through our proxy server for use with Eino
type EinoMCPProxyTool struct {
	toolName   string
	toolInfo   *schema.ToolInfo
	mcpClient  *client.Client
}

// NewEinoMCPProxyTool creates a new Eino tool that connects to our MCP proxy
func NewEinoMCPProxyTool(toolName string, mcpClient *client.Client) *EinoMCPProxyTool {
	return &EinoMCPProxyTool{
		toolName:  toolName,
		mcpClient: mcpClient,
	}
}

// Info returns the tool information for Eino (implementing BaseTool interface)
func (t *EinoMCPProxyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// Return cached tool info if we have it
	if t.toolInfo != nil {
		return t.toolInfo, nil
	}

	// Get tool information from the MCP proxy server
	toolsResult, err := t.mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from MCP proxy: %w", err)
	}

	// Find our specific tool
	for _, tool := range toolsResult.Tools {
		if tool.Name == t.toolName {
			// Debug: Log the raw MCP schema
			log.Printf("DEBUG: Raw MCP schema for tool %s: %+v", tool.Name, tool.InputSchema)
			
			// For the MCP proxy approach, we need to convert the JSON schema to OpenAPI v3
			// This is temporary - ideally Eino would support native MCP integration
			openAPISchema, err := convertMCPInputSchemaToOpenAPI(tool.InputSchema)
			if err != nil {
				return nil, fmt.Errorf("failed to convert MCP schema for tool %s: %w", tool.Name, err)
			}

			// Debug: Log the converted OpenAPI schema
			log.Printf("DEBUG: Converted OpenAPI schema for tool %s: %+v", tool.Name, openAPISchema)

			toolInfo := &schema.ToolInfo{
				Name:        tool.Name,
				Desc:        tool.Description,
				ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(openAPISchema),
			}
			
			// Cache the tool info
			t.toolInfo = toolInfo
			return toolInfo, nil
		}
	}

	return nil, fmt.Errorf("tool %s not found in MCP proxy", t.toolName)
}

// InvokableRun executes the tool via the MCP proxy (implementing InvokableTool interface)
func (t *EinoMCPProxyTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Parse arguments from JSON
	var arguments map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &arguments); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Create MCP tool call request
	request := mcp.CallToolRequest{}
	request.Params.Name = t.toolName
	request.Params.Arguments = arguments

	// Call the tool through the MCP proxy
	result, err := t.mcpClient.CallTool(ctx, request)
	if err != nil {
		return "", fmt.Errorf("MCP tool call failed for %s: %w", t.toolName, err)
	}

	// Check if the tool call had an error
	if result.IsError {
		return "", fmt.Errorf("MCP tool execution failed: %s", getErrorFromResult(result))
	}

	// Convert result to JSON string
	resultJSON, err := convertMCPResultToJSON(result)
	if err != nil {
		return "", fmt.Errorf("failed to convert tool result: %w", err)
	}

	return resultJSON, nil
}

// MCPProxyToolsLoader loads tools from an MCP proxy client as Eino tools
type MCPProxyToolsLoader struct {
	mcpClient *client.Client
}

// NewMCPProxyToolsLoader creates a new tools loader that uses our MCP proxy
func NewMCPProxyToolsLoader(mcpClient *client.Client) *MCPProxyToolsLoader {
	return &MCPProxyToolsLoader{
		mcpClient: mcpClient,
	}
}

// LoadAllTools loads all available tools from the MCP proxy as Eino tools
func (l *MCPProxyToolsLoader) LoadAllTools(ctx context.Context) ([]tool.InvokableTool, error) {
	// Get all tools from the MCP proxy
	toolsResult, err := l.mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from MCP proxy: %w", err)
	}

	// Convert each MCP tool to an Eino tool
	einoTools := make([]tool.InvokableTool, len(toolsResult.Tools))
	for i, mcpTool := range toolsResult.Tools {
		einoTools[i] = NewEinoMCPProxyTool(mcpTool.Name, l.mcpClient)
	}

	return einoTools, nil
}

// LoadSpecificTools loads only specified tools from the MCP proxy
func (l *MCPProxyToolsLoader) LoadSpecificTools(ctx context.Context, toolNames []string) ([]tool.InvokableTool, error) {
	// Get all tools from the MCP proxy first
	toolsResult, err := l.mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from MCP proxy: %w", err)
	}

	// Create a map for quick lookup
	toolMap := make(map[string]mcp.Tool)
	for _, mcpTool := range toolsResult.Tools {
		toolMap[mcpTool.Name] = mcpTool
	}

	// Filter and convert only the requested tools
	einoTools := make([]tool.InvokableTool, 0, len(toolNames))
	for _, toolName := range toolNames {
		if _, exists := toolMap[toolName]; exists {
			einoTools = append(einoTools, NewEinoMCPProxyTool(toolName, l.mcpClient))
		} else {
			return nil, fmt.Errorf("tool %s not found in MCP proxy", toolName)
		}
	}

	return einoTools, nil
}

// Helper functions

func getErrorFromResult(result *mcp.CallToolResult) string {
	// Try to extract error message from result content
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(mcp.TextContent); ok {
			return textContent.Text
		}
	}
	return "Unknown error occurred"
}

func convertMCPResultToJSON(result *mcp.CallToolResult) (string, error) {
	// Convert MCP result content to JSON
	if len(result.Content) == 0 {
		return "{}", nil
	}

	// Handle different content types
	if len(result.Content) == 1 {
		content := result.Content[0]
		
		// If it's text content, return it directly
		if textContent, ok := content.(mcp.TextContent); ok {
			// Try to parse as JSON first, if that fails return as text field
			var jsonData interface{}
			if err := json.Unmarshal([]byte(textContent.Text), &jsonData); err == nil {
				// It's valid JSON, return as-is
				return textContent.Text, nil
			} else {
				// Not JSON, wrap in a text field
				result := map[string]interface{}{
					"text": textContent.Text,
				}
				resultJSON, err := json.Marshal(result)
				return string(resultJSON), err
			}
		}
	}

	// For multiple content items or other types, return full structure
	resultJSON, err := json.Marshal(map[string]interface{}{
		"content": result.Content,
	})
	return string(resultJSON), err
}

// convertMCPInputSchemaToOpenAPI converts MCP JSON schema to OpenAPI v3 schema
func convertMCPInputSchemaToOpenAPI(inputSchema interface{}) (*openapi3.Schema, error) {
	// Convert interface{} to map[string]interface{} for processing
	var schemaData map[string]interface{}
	
	switch v := inputSchema.(type) {
	case map[string]interface{}:
		schemaData = v
	case []byte:
		if err := json.Unmarshal(v, &schemaData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &schemaData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema string: %w", err)
		}
	default:
		// Try to marshal and unmarshal to get it into the right format
		schemaBytes, err := json.Marshal(inputSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input schema: %w", err)
		}
		if err := json.Unmarshal(schemaBytes, &schemaData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema data: %w", err)
		}
	}
	
	// Use the existing conversion function from eino_mcp_tool.go
	return convertJSONSchemaToOpenAPI(schemaData)
}