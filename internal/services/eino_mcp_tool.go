package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/getkin/kin-openapi/openapi3"

	"station/pkg/models"
)

// MCPEinoTool adapts an MCP tool for use with Eino framework
type MCPEinoTool struct {
	mcpTool       *models.MCPTool
	clientService *MCPClientService
	environmentID int64
}

// NewMCPEinoTool creates a new Eino tool wrapper for an MCP tool
func NewMCPEinoTool(mcpTool *models.MCPTool, clientService *MCPClientService, environmentID int64) *MCPEinoTool {
	return &MCPEinoTool{
		mcpTool:       mcpTool,
		clientService: clientService,
		environmentID: environmentID,
	}
}

// Info returns the tool information for Eino
func (t *MCPEinoTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	// Parse the MCP tool schema into OpenAPI v3 format
	var schemaData map[string]interface{}
	if err := json.Unmarshal(t.mcpTool.Schema, &schemaData); err != nil {
		return nil, fmt.Errorf("failed to parse MCP tool schema: %w", err)
	}

	// Convert JSON schema to OpenAPI v3 schema
	openAPISchema, err := convertJSONSchemaToOpenAPI(schemaData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to OpenAPI: %w", err)
	}

	return &schema.ToolInfo{
		Name:        t.mcpTool.Name,
		Desc:        t.mcpTool.Description,
		ParamsOneOf: schema.NewParamsOneOfByOpenAPIV3(openAPISchema),
	}, nil
}

// InvokableRun executes the MCP tool via the client service
func (t *MCPEinoTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// Parse arguments from JSON
	var arguments map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &arguments); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Call the MCP tool via our client service
	result, err := t.clientService.CallTool(t.environmentID, t.mcpTool.Name, arguments)
	if err != nil {
		return "", fmt.Errorf("failed to call MCP tool %s: %w", t.mcpTool.Name, err)
	}

	// Check if the tool call had an error
	if result.Error != "" {
		return "", fmt.Errorf("MCP tool execution failed: %s", result.Error)
	}

	// Convert result to JSON string
	resultJSON, err := json.Marshal(result.Result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool result: %w", err)
	}

	return string(resultJSON), nil
}

// convertJSONSchemaToOpenAPI converts a JSON schema map to OpenAPI v3 schema
func convertJSONSchemaToOpenAPI(schemaData map[string]interface{}) (*openapi3.Schema, error) {
	// Create OpenAPI schema from the JSON schema data
	schema := &openapi3.Schema{}
	
	if schemaType, ok := schemaData["type"].(string); ok {
		schema.Type = schemaType
	}
	
	if description, ok := schemaData["description"].(string); ok {
		schema.Description = description
	}
	
	if properties, ok := schemaData["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]*openapi3.SchemaRef)
		for propName, propData := range properties {
			if propMap, ok := propData.(map[string]interface{}); ok {
				propSchema, err := convertJSONSchemaToOpenAPI(propMap)
				if err != nil {
					return nil, fmt.Errorf("failed to convert property %s: %w", propName, err)
				}
				schema.Properties[propName] = &openapi3.SchemaRef{Value: propSchema}
			}
		}
	}
	
	if required, ok := schemaData["required"].([]interface{}); ok {
		schema.Required = make([]string, len(required))
		for i, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required[i] = reqStr
			}
		}
	}
	
	return schema, nil
}

// MCPToolsLoader loads MCP tools from an environment and converts them to Eino tools
type MCPToolsLoader struct {
	clientService     *MCPClientService
	toolDiscoveryService *ToolDiscoveryService
}

// NewMCPToolsLoader creates a new MCP tools loader
func NewMCPToolsLoader(clientService *MCPClientService, toolDiscoveryService *ToolDiscoveryService) *MCPToolsLoader {
	return &MCPToolsLoader{
		clientService:     clientService,
		toolDiscoveryService: toolDiscoveryService,
	}
}

// LoadToolsForEnvironment loads all available MCP tools for an environment as Eino tools
func (l *MCPToolsLoader) LoadToolsForEnvironment(environmentID int64) ([]tool.InvokableTool, error) {
	// Get all MCP tools for the environment
	mcpTools, err := l.toolDiscoveryService.GetToolsByEnvironment(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP tools for environment %d: %w", environmentID, err)
	}

	// Convert each MCP tool to an Eino tool
	einoTools := make([]tool.InvokableTool, len(mcpTools))
	for i, mcpTool := range mcpTools {
		einoTools[i] = NewMCPEinoTool(mcpTool, l.clientService, environmentID)
	}

	return einoTools, nil
}

// LoadSpecificTools loads only the specified MCP tools by name for an environment
func (l *MCPToolsLoader) LoadSpecificTools(environmentID int64, toolNames []string) ([]tool.InvokableTool, error) {
	// Get all MCP tools for the environment first
	allMCPTools, err := l.toolDiscoveryService.GetToolsByEnvironment(environmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP tools for environment %d: %w", environmentID, err)
	}

	// Create a map for quick lookup
	toolMap := make(map[string]*models.MCPTool)
	for _, mcpTool := range allMCPTools {
		toolMap[mcpTool.Name] = mcpTool
	}

	// Filter and convert only the requested tools
	einoTools := make([]tool.InvokableTool, 0, len(toolNames))
	for _, toolName := range toolNames {
		if mcpTool, exists := toolMap[toolName]; exists {
			einoTools = append(einoTools, NewMCPEinoTool(mcpTool, l.clientService, environmentID))
		} else {
			return nil, fmt.Errorf("tool %s not found in environment %d", toolName, environmentID)
		}
	}

	return einoTools, nil
}