package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"station/pkg/openapi/converter"
	"station/pkg/openapi/models"
	"station/pkg/openapi/parser"
)

// ServerConfig represents the configuration for the OpenAPI MCP server
type ServerConfig struct {
	ConfigPath string            `json:"config_path,omitempty"`
	ConfigData string            `json:"config_data,omitempty"`
	Variables  map[string]string `json:"variables,omitempty"` // Template variables for security lookups
}

// Server implements an MCP server that executes OpenAPI-based tools
type Server struct {
	config     *models.MCPConfig
	httpClient *http.Client
	variables  map[string]string // Variables for security credential lookups
}

// NewServer creates a new OpenAPI MCP server runtime
func NewServer(serverConfig ServerConfig) *Server {
	server := &Server{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		variables: serverConfig.Variables,
	}

	if serverConfig.ConfigData != "" {
		_ = server.LoadConfigFromString(serverConfig.ConfigData)
	} else if serverConfig.ConfigPath != "" {
		_ = server.LoadConfigFromFile(serverConfig.ConfigPath)
	} else {
		_ = server.LoadConfig()
	}

	return server
}

// LoadConfig loads the MCP configuration from environment or file
func (s *Server) LoadConfig() error {
	// First try to load from environment variable (inline config)
	if configStr := os.Getenv("OPENAPI_MCP_CONFIG"); configStr != "" {
		return s.LoadConfigFromString(configStr)
	}

	// Otherwise try to load from file
	configPath := os.Getenv("OPENAPI_MCP_CONFIG_PATH")
	if configPath == "" {
		configPath = "openapi-mcp-config.yaml"
	}

	return s.LoadConfigFromFile(configPath)
}

// LoadConfigFromFile loads MCP config from a file
// Supports both .openapi.json files and MCP config files
func (s *Server) LoadConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Check if this is an OpenAPI spec by looking for "openapi" or "swagger" field
	// Try JSON first, then YAML
	var check map[string]interface{}
	isJSON := false
	if err := json.Unmarshal(data, &check); err == nil {
		isJSON = true
	} else {
		// Try YAML
		if err := yaml.Unmarshal(data, &check); err != nil {
			return fmt.Errorf("failed to parse config as JSON or YAML: %w", err)
		}
	}

	// Check if it's an OpenAPI spec
	if _, hasOpenAPI := check["openapi"]; hasOpenAPI {
		// Convert YAML to JSON if needed (parser expects JSON)
		if !isJSON {
			jsonData, err := json.Marshal(check)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
			data = jsonData
		}
		// This is an OpenAPI spec - convert it to MCP config
		return s.LoadConfigFromOpenAPISpec(data)
	}
	if _, hasSwagger := check["swagger"]; hasSwagger {
		// Convert YAML to JSON if needed (parser expects JSON)
		if !isJSON {
			jsonData, err := json.Marshal(check)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
			data = jsonData
		}
		// This is a Swagger 2.0 spec - convert it to MCP config
		return s.LoadConfigFromOpenAPISpec(data)
	}

	// Otherwise treat as MCP config (YAML or JSON)
	return s.LoadConfigFromBytes(data)
}

// LoadConfigFromOpenAPISpec loads and converts an OpenAPI spec to MCP config
func (s *Server) LoadConfigFromOpenAPISpec(specData []byte) error {
	// Import the converter package
	var p = parser.NewParser()

	// Create temp file for the spec
	tmpFile, err := os.CreateTemp("", "openapi-runtime-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write spec data
	if _, err := tmpFile.Write(specData); err != nil {
		return fmt.Errorf("failed to write spec: %w", err)
	}
	tmpFile.Close()

	// Parse the OpenAPI spec
	if err := p.ParseFile(tmpFile.Name()); err != nil {
		return fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Convert to MCP config
	convertOpts := models.ConvertOptions{
		ServerName: "openapi-server",
	}
	c := converter.NewConverter(p, convertOpts)
	mcpConfig, err := c.Convert()
	if err != nil {
		return fmt.Errorf("failed to convert OpenAPI to MCP: %w", err)
	}

	// Set the config
	s.config = mcpConfig
	return nil
}

// LoadConfigFromString loads MCP config from a string
func (s *Server) LoadConfigFromString(configStr string) error {
	data := []byte(configStr)

	// Check if this is an OpenAPI spec by looking for "openapi" or "swagger" field
	var check map[string]interface{}
	isJSON := false
	if err := json.Unmarshal(data, &check); err == nil {
		isJSON = true
	} else {
		// Try YAML
		if err := yaml.Unmarshal(data, &check); err != nil {
			return fmt.Errorf("failed to parse config as JSON or YAML: %w", err)
		}
	}

	// Check if it's an OpenAPI spec
	if _, hasOpenAPI := check["openapi"]; hasOpenAPI {
		// Convert YAML to JSON if needed (parser expects JSON)
		if !isJSON {
			jsonData, err := json.Marshal(check)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
			data = jsonData
		}
		// This is an OpenAPI spec - convert it to MCP config
		return s.LoadConfigFromOpenAPISpec(data)
	}
	if _, hasSwagger := check["swagger"]; hasSwagger {
		// Convert YAML to JSON if needed (parser expects JSON)
		if !isJSON {
			jsonData, err := json.Marshal(check)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
			data = jsonData
		}
		// This is a Swagger 2.0 spec - convert it to MCP config
		return s.LoadConfigFromOpenAPISpec(data)
	}

	// Otherwise treat as MCP config
	return s.LoadConfigFromBytes(data)
}

// LoadConfigFromBytes loads MCP config from bytes
func (s *Server) LoadConfigFromBytes(data []byte) error {
	var config models.MCPConfig

	// Try YAML first
	if err := yaml.Unmarshal(data, &config); err == nil {
		s.config = &config
		return nil
	}

	// Fall back to JSON
	if err := json.Unmarshal(data, &config); err == nil {
		s.config = &config
		return nil
	}

	return fmt.Errorf("failed to parse config as YAML or JSON")
}

// GetTools returns all available tools
func (s *Server) GetTools() []models.Tool {
	if s.config == nil {
		return nil
	}
	return s.config.Tools
}

// ExecuteTool executes an OpenAPI-based tool with the given arguments
func (s *Server) ExecuteTool(toolName string, args map[string]interface{}) (interface{}, error) {
	// Find the tool
	var tool *models.Tool
	for _, t := range s.config.Tools {
		if t.Name == toolName {
			tool = &t
			break
		}
	}

	if tool == nil {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Build the HTTP request based on the tool's request template
	req, err := s.buildRequest(tool, args)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Execute the HTTP request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error status codes
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Process the response based on the response template
	result := s.processResponse(tool, body)

	return result, nil
}

// buildRequest builds an HTTP request from a tool and arguments
func (s *Server) buildRequest(tool *models.Tool, args map[string]interface{}) (*http.Request, error) {
	// Start with the base URL
	url := tool.RequestTemplate.URL

	// Process path parameters
	for _, arg := range tool.Args {
		if arg.Position == "path" {
			if value, ok := args[arg.Name]; ok {
				placeholder := fmt.Sprintf("{%s}", arg.Name)
				url = strings.ReplaceAll(url, placeholder, fmt.Sprintf("%v", value))
			} else if arg.Required {
				return nil, fmt.Errorf("required path parameter missing: %s", arg.Name)
			}
		}
	}

	// Process query parameters
	queryParams := make(map[string]string)
	for _, arg := range tool.Args {
		if arg.Position == "query" {
			if value, ok := args[arg.Name]; ok {
				queryParams[arg.Name] = fmt.Sprintf("%v", value)
			} else if arg.Required {
				return nil, fmt.Errorf("required query parameter missing: %s", arg.Name)
			}
		}
	}

	// Add query parameters to URL
	if len(queryParams) > 0 {
		params := make([]string, 0, len(queryParams))
		for k, v := range queryParams {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		url = fmt.Sprintf("%s?%s", url, strings.Join(params, "&"))
	}

	// Process body parameters
	var bodyData []byte
	var contentType string

	// Check if we have body parameters
	hasBodyParams := false
	for _, arg := range tool.Args {
		if arg.Position == "body" {
			hasBodyParams = true
			break
		}
	}

	if hasBodyParams {
		// Collect body parameters
		bodyMap := make(map[string]interface{})
		for _, arg := range tool.Args {
			if arg.Position == "body" {
				if value, ok := args[arg.Name]; ok {
					bodyMap[arg.Name] = value
				} else if arg.Required {
					return nil, fmt.Errorf("required body parameter missing: %s", arg.Name)
				}
			}
		}

		// Determine content type from headers
		for _, header := range tool.RequestTemplate.Headers {
			if strings.ToLower(header.Key) == "content-type" {
				contentType = header.Value
				break
			}
		}

		// Marshal body based on content type
		if strings.Contains(contentType, "application/json") {
			data, err := json.Marshal(bodyMap)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
			}
			bodyData = data
		} else if strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// Build form-encoded body
			params := make([]string, 0, len(bodyMap))
			for k, v := range bodyMap {
				params = append(params, fmt.Sprintf("%s=%v", k, v))
			}
			bodyData = []byte(strings.Join(params, "&"))
		}
	}

	// Create the HTTP request
	var req *http.Request
	var err error

	if len(bodyData) > 0 {
		req, err = http.NewRequest(tool.RequestTemplate.Method, url, bytes.NewReader(bodyData))
	} else {
		req, err = http.NewRequest(tool.RequestTemplate.Method, url, nil)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for _, header := range tool.RequestTemplate.Headers {
		req.Header.Set(header.Key, header.Value)
	}

	// Process header parameters
	for _, arg := range tool.Args {
		if arg.Position == "header" {
			if value, ok := args[arg.Name]; ok {
				req.Header.Set(arg.Name, fmt.Sprintf("%v", value))
			} else if arg.Required {
				return nil, fmt.Errorf("required header parameter missing: %s", arg.Name)
			}
		}
	}

	// Process cookie parameters
	for _, arg := range tool.Args {
		if arg.Position == "cookie" {
			if value, ok := args[arg.Name]; ok {
				req.AddCookie(&http.Cookie{
					Name:  arg.Name,
					Value: fmt.Sprintf("%v", value),
				})
			} else if arg.Required {
				return nil, fmt.Errorf("required cookie parameter missing: %s", arg.Name)
			}
		}
	}

	// Apply security if configured
	if tool.RequestTemplate.Security != nil && s.config.Server.SecuritySchemes != nil {
		if err := s.applySecurity(req, tool.RequestTemplate.Security); err != nil {
			return nil, fmt.Errorf("failed to apply security: %w", err)
		}
	}

	return req, nil
}

// applySecurity applies security scheme to the request
func (s *Server) applySecurity(req *http.Request, security *models.ToolSecurityRequirement) error {
	// Find the security scheme
	var scheme *models.SecurityScheme
	for _, s := range s.config.Server.SecuritySchemes {
		if s.ID == security.ID {
			scheme = &s
			break
		}
	}

	if scheme == nil {
		return fmt.Errorf("security scheme not found: %s", security.ID)
	}

	switch scheme.Type {
	case "apiKey":
		envKey := fmt.Sprintf("OPENAPI_%s_KEY", strings.ToUpper(scheme.ID))
		apiKey := s.getVariable(envKey)
		if apiKey == "" {
			apiKey = scheme.DefaultCredential
		}
		if apiKey == "" {
			return fmt.Errorf("API key not configured for scheme: %s (set %s in variables.yml or environment)", scheme.ID, envKey)
		}

		// Apply based on location
		switch scheme.In {
		case "header":
			req.Header.Set(scheme.Name, apiKey)
		case "query":
			// Add to query string
			if strings.Contains(req.URL.String(), "?") {
				req.URL.RawQuery += fmt.Sprintf("&%s=%s", scheme.Name, apiKey)
			} else {
				req.URL.RawQuery = fmt.Sprintf("%s=%s", scheme.Name, apiKey)
			}
		case "cookie":
			req.AddCookie(&http.Cookie{
				Name:  scheme.Name,
				Value: apiKey,
			})
		}

	case "http":
		if scheme.Scheme == "bearer" {
			envKey := fmt.Sprintf("OPENAPI_%s_TOKEN", strings.ToUpper(scheme.ID))
			token := s.getVariable(envKey)
			if token == "" {
				token = scheme.DefaultCredential
			}
			if token == "" {
				return fmt.Errorf("Bearer token not configured for scheme: %s (set %s in variables.yml or environment)", scheme.ID, envKey)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		} else if scheme.Scheme == "basic" {
			envKey := fmt.Sprintf("OPENAPI_%s_CREDS", strings.ToUpper(scheme.ID))
			creds := s.getVariable(envKey)
			if creds == "" {
				creds = scheme.DefaultCredential
			}
			if creds == "" {
				return fmt.Errorf("Basic auth credentials not configured for scheme: %s (set %s in variables.yml or environment)", scheme.ID, envKey)
			}
			req.Header.Set("Authorization", fmt.Sprintf("Basic %s", creds))
		}

	default:
		return fmt.Errorf("unsupported security scheme type: %s", scheme.Type)
	}

	return nil
}

func (s *Server) getVariable(key string) string {
	if s.variables != nil {
		if val, ok := s.variables[key]; ok && val != "" {
			return val
		}
	}
	return os.Getenv(key)
}

// processResponse processes the HTTP response based on the response template
func (s *Server) processResponse(tool *models.Tool, body []byte) interface{} {
	result := make(map[string]interface{})

	// Add prepend body if configured
	if tool.ResponseTemplate.PrependBody != "" {
		result["description"] = tool.ResponseTemplate.PrependBody
	}

	// Try to parse the response as JSON
	var jsonData interface{}
	if err := json.Unmarshal(body, &jsonData); err == nil {
		result["data"] = jsonData
	} else {
		// If not JSON, return as string
		result["data"] = string(body)
	}

	// Add append body if configured
	if tool.ResponseTemplate.AppendBody != "" {
		result["appendNote"] = tool.ResponseTemplate.AppendBody
	}

	return result
}

// GetServerInfo returns information about the server
func (s *Server) GetServerInfo() map[string]interface{} {
	if s.config == nil {
		return map[string]interface{}{
			"name":        "openapi-mcp-server",
			"description": "OpenAPI to MCP Server Runtime",
			"status":      "not configured",
		}
	}

	return map[string]interface{}{
		"name":        s.config.Server.Name,
		"description": fmt.Sprintf("OpenAPI MCP Server with %d tools", len(s.config.Tools)),
		"toolCount":   len(s.config.Tools),
		"status":      "ready",
	}
}

// ListTools returns a list of available tools for MCP
func (s *Server) ListTools() []map[string]interface{} {
	if s.config == nil {
		return []map[string]interface{}{}
	}

	tools := make([]map[string]interface{}, 0, len(s.config.Tools))
	for _, tool := range s.config.Tools {
		toolDef := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}

		// Build input schema from tool arguments
		if len(tool.Args) > 0 {
			properties := make(map[string]interface{})
			required := []string{}

			for _, arg := range tool.Args {
				prop := map[string]interface{}{
					"type":        arg.Type,
					"description": arg.Description,
				}

				properties[arg.Name] = prop

				if arg.Required {
					required = append(required, arg.Name)
				}
			}

			inputSchema := map[string]interface{}{
				"type":       "object",
				"properties": properties,
			}

			if len(required) > 0 {
				inputSchema["required"] = required
			}

			toolDef["inputSchema"] = inputSchema
		}

		tools = append(tools, toolDef)
	}

	return tools
}

// CallTool executes a tool and returns the result
func (s *Server) CallTool(toolName string, arguments map[string]interface{}) (interface{}, error) {
	return s.ExecuteTool(toolName, arguments)
}
