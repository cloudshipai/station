package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"station/pkg/openapi/converter"
	"station/pkg/openapi/models"
	"station/pkg/openapi/parser"
	"gopkg.in/yaml.v3"
)

// Service handles OpenAPI to MCP server conversion
type Service struct{}

// NewService creates a new OpenAPI service
func NewService() *Service {
	return &Service{}
}

// ConvertOptions holds options for converting OpenAPI spec to MCP config
type ConvertOptions struct {
	ServerName     string
	ToolNamePrefix string
	BaseURL        string // Optional base URL override
}

// ConvertFromSpec converts an OpenAPI specification to MCP server configuration
// Returns the generated JSON configuration ready for file storage
func (s *Service) ConvertFromSpec(spec string, options ConvertOptions) (string, error) {
	// Create temporary file for the spec
	tmpFile, err := os.CreateTemp("", "openapi-*.yaml")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write spec to temp file
	if _, err := tmpFile.WriteString(spec); err != nil {
		return "", fmt.Errorf("failed to write spec to temp file: %w", err)
	}
	tmpFile.Close()

	// Parse the OpenAPI specification
	p := parser.NewParser()
	if err := p.ParseFile(tmpFile.Name()); err != nil {
		return "", fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Set default server name if not provided
	if options.ServerName == "" {
		options.ServerName = "openapi-server"
	}

	// Create converter with options
	convertOpts := models.ConvertOptions{
		ServerName:     options.ServerName,
		ToolNamePrefix: options.ToolNamePrefix,
	}

	// Add base URL to server config if provided
	if options.BaseURL != "" {
		convertOpts.ServerConfig = map[string]interface{}{
			"baseUrl": options.BaseURL,
		}
	}

	// Convert to MCP config
	c := converter.NewConverter(p, convertOpts)
	mcpConfig, err := c.Convert()
	if err != nil {
		return "", fmt.Errorf("failed to convert OpenAPI to MCP: %w", err)
	}

	// Convert to Station's expected MCP server format
	stationConfig := s.convertToStationFormat(mcpConfig)

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(stationConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	return string(jsonData), nil
}

// ConvertFromReader converts an OpenAPI spec from a reader
func (s *Service) ConvertFromReader(reader io.Reader, options ConvertOptions) (string, error) {
	spec, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read spec: %w", err)
	}
	return s.ConvertFromSpec(string(spec), options)
}

// convertToStationFormat converts the MCP config to Station's expected format
// For OpenAPI specs, we use the openapi-runtime-server which is built into Station
func (s *Service) convertToStationFormat(mcpConfig *models.MCPConfig) map[string]interface{} {
	// Create the Station MCP server configuration using the built-in runtime
	config := map[string]interface{}{
		"command": "stn",
		"args": []string{
			"openapi-runtime",
			"--config",
			"inline",
		},
		"env": map[string]string{
			"OPENAPI_MCP_CONFIG": s.serializeMCPConfig(mcpConfig),
		},
	}

	// Add description
	if mcpConfig.Server.Name != "" {
		config["description"] = fmt.Sprintf("OpenAPI MCP Server: %s", mcpConfig.Server.Name)
	}

	return config
}

// serializeMCPConfig serializes the MCP config to YAML for embedding in environment variable
func (s *Service) serializeMCPConfig(config *models.MCPConfig) string {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)

	if err := encoder.Encode(config); err != nil {
		// Fallback to JSON if YAML encoding fails
		jsonData, _ := json.Marshal(config)
		return string(jsonData)
	}

	return buffer.String()
}

// GenerateFileName generates a sanitized filename for the MCP server config
func (s *Service) GenerateFileName(serverName string) string {
	// Sanitize the server name for use as a filename
	name := strings.ToLower(serverName)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	// Remove any characters that aren't alphanumeric or hyphen
	var sanitized strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			sanitized.WriteRune(ch)
		}
	}

	result := sanitized.String()
	if result == "" {
		result = "openapi-server"
	}

	return result + ".json"
}

// SaveToEnvironment saves the MCP config to the appropriate environment directory
func (s *Service) SaveToEnvironment(config string, filename string, environmentPath string) error {
	// Ensure the environment directory exists
	if err := os.MkdirAll(environmentPath, 0755); err != nil {
		return fmt.Errorf("failed to create environment directory: %w", err)
	}

	// Write the config file
	configPath := filepath.Join(environmentPath, filename)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ValidateSpec validates an OpenAPI specification
func (s *Service) ValidateSpec(spec string) error {
	// Create temporary file for the spec
	tmpFile, err := os.CreateTemp("", "openapi-validate-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write spec to temp file
	if _, err := tmpFile.WriteString(spec); err != nil {
		return fmt.Errorf("failed to write spec to temp file: %w", err)
	}
	tmpFile.Close()

	// Parse and validate the OpenAPI specification
	p := parser.NewParser()
	p.SetValidation(true)
	if err := p.ParseFile(tmpFile.Name()); err != nil {
		return fmt.Errorf("OpenAPI spec validation failed: %w", err)
	}

	return nil
}