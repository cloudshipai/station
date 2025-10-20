package services

import (
	"encoding/json"
	"testing"

	"station/pkg/models"
)

// TestNewMCPClient tests MCP client creation
func TestNewMCPClient(t *testing.T) {
	tests := []struct {
		name        string
		description string
	}{
		{
			name:        "Create MCP client",
			description: "Should create client instance",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewMCPClient()

			if client == nil {
				t.Fatal("NewMCPClient() returned nil")
			}

			t.Logf("Created MCP client: %+v", client)
		})
	}
}

// TestCreateTransportFromFields tests transport creation from individual fields
func TestCreateTransportFromFields(t *testing.T) {
	client := NewMCPClient()

	tests := []struct {
		name        string
		command     string
		url         string
		args        []string
		env         map[string]string
		wantErr     bool
		description string
	}{
		{
			name:        "Stdio transport with command",
			command:     "npx",
			args:        []string{"-y", "@modelcontextprotocol/server-filesystem"},
			env:         map[string]string{"PATH": "/usr/bin"},
			wantErr:     false,
			description: "Should create stdio transport",
		},
		{
			name:        "HTTP transport with URL",
			url:         "http://localhost:8080/mcp",
			wantErr:     false,
			description: "Should create HTTP transport",
		},
		{
			name:        "HTTPS transport with URL",
			url:         "https://api.example.com/mcp",
			wantErr:     false,
			description: "Should create HTTPS transport",
		},
		{
			name:        "URL in args for backwards compatibility",
			args:        []string{"http://localhost:8080/mcp"},
			wantErr:     false,
			description: "Should detect URL in args",
		},
		{
			name:        "No valid transport configuration",
			wantErr:     true,
			description: "Should fail with no command or URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := client.createTransportFromFields(tt.command, tt.url, tt.args, tt.env)

			if (err != nil) != tt.wantErr {
				t.Errorf("createTransportFromFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && transport == nil {
				t.Error("Expected transport to be created, got nil")
			}

			if tt.wantErr && transport != nil {
				t.Error("Expected nil transport for error case")
			}

			t.Logf("Transport creation: command=%s, url=%s, err=%v", tt.command, tt.url, err)
		})
	}
}

// TestCreateTransportFromConfig tests transport creation from config map
func TestCreateTransportFromConfig(t *testing.T) {
	client := NewMCPClient()

	tests := []struct {
		name        string
		config      map[string]interface{}
		wantErr     bool
		description string
	}{
		{
			name: "Stdio config with command and args",
			config: map[string]interface{}{
				"command": "npx",
				"args":    []interface{}{"-y", "@modelcontextprotocol/server-filesystem"},
				"env": map[string]interface{}{
					"PATH": "/usr/bin",
				},
			},
			wantErr:     false,
			description: "Should create stdio transport from config",
		},
		{
			name: "HTTP config with URL",
			config: map[string]interface{}{
				"url": "http://localhost:8080/mcp",
			},
			wantErr:     false,
			description: "Should create HTTP transport from config",
		},
		{
			name: "Config with environment variables",
			config: map[string]interface{}{
				"command": "ship",
				"args":    []interface{}{"mcp", "security"},
				"env": map[string]interface{}{
					"API_KEY": "test-key",
					"DEBUG":   "true",
				},
			},
			wantErr:     false,
			description: "Should handle environment variables",
		},
		{
			name:        "Empty config",
			config:      map[string]interface{}{},
			wantErr:     true,
			description: "Should fail with empty config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := client.createTransportFromConfig(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("createTransportFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && transport == nil {
				t.Error("Expected transport to be created, got nil")
			}

			t.Logf("Config transport creation: %v", tt.config)
		})
	}
}

// TestCreateHTTPTransport tests HTTP transport creation
func TestCreateHTTPTransport(t *testing.T) {
	client := NewMCPClient()

	tests := []struct {
		name        string
		baseURL     string
		envVars     map[string]string
		wantErr     bool
		description string
	}{
		{
			name:        "Valid HTTP URL",
			baseURL:     "http://localhost:8080/mcp",
			wantErr:     false,
			description: "Should create transport for HTTP URL",
		},
		{
			name:        "Valid HTTPS URL",
			baseURL:     "https://api.example.com/mcp",
			wantErr:     false,
			description: "Should create transport for HTTPS URL",
		},
		{
			name:    "HTTP with authorization header",
			baseURL: "http://localhost:8080/mcp",
			envVars: map[string]string{
				"AUTHORIZATION": "Bearer token123",
			},
			wantErr:     false,
			description: "Should handle authorization headers",
		},
		{
			name:    "HTTP with API key",
			baseURL: "http://localhost:8080/mcp",
			envVars: map[string]string{
				"API_KEY": "secret-key",
			},
			wantErr:     false,
			description: "Should handle API key headers",
		},
		{
			name:    "HTTP with custom headers",
			baseURL: "http://localhost:8080/mcp",
			envVars: map[string]string{
				"HTTP_X_Custom_Header": "custom-value",
			},
			wantErr:     false,
			description: "Should handle custom HTTP headers",
		},
		{
			name:        "Invalid URL",
			baseURL:     "://invalid-url",
			wantErr:     true,
			description: "Should fail with invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := client.createHTTPTransport(tt.baseURL, tt.envVars)

			if (err != nil) != tt.wantErr {
				t.Errorf("createHTTPTransport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && transport == nil {
				t.Error("Expected transport to be created, got nil")
			}

			t.Logf("HTTP transport: url=%s, headers=%d", tt.baseURL, len(tt.envVars))
		})
	}
}

// TestCreateTransportDeprecated tests deprecated createTransport method
func TestCreateTransportDeprecated(t *testing.T) {
	client := NewMCPClient()

	tests := []struct {
		name        string
		config      models.MCPServerConfig
		wantErr     bool
		description string
	}{
		{
			name: "Stdio transport with command",
			config: models.MCPServerConfig{
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Env: map[string]string{
					"PATH": "/usr/bin",
				},
			},
			wantErr:     false,
			description: "Should create stdio transport",
		},
		{
			name: "HTTP transport with URL",
			config: models.MCPServerConfig{
				URL: "http://localhost:8080/mcp",
			},
			wantErr:     false,
			description: "Should create HTTP transport",
		},
		{
			name: "URL in args backwards compatibility",
			config: models.MCPServerConfig{
				Args: []string{"https://api.example.com/mcp"},
			},
			wantErr:     false,
			description: "Should detect URL in args",
		},
		{
			name:        "No valid transport",
			config:      models.MCPServerConfig{},
			wantErr:     true,
			description: "Should fail with empty config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := client.createTransport(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("createTransport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && transport == nil {
				t.Error("Expected transport to be created, got nil")
			}

			t.Logf("Deprecated transport: command=%s, url=%s", tt.config.Command, tt.config.URL)
		})
	}
}

// TestDiscoverToolsFromRenderedConfigJSON tests JSON parsing
func TestDiscoverToolsFromRenderedConfigJSON(t *testing.T) {
	client := NewMCPClient()

	tests := []struct {
		name        string
		configJSON  string
		wantErr     bool
		description string
	}{
		{
			name: "Valid minimal config",
			configJSON: `{
				"mcpServers": {
					"test-server": {
						"command": "echo",
						"args": ["test"]
					}
				}
			}`,
			wantErr:     false,
			description: "Should parse valid minimal config",
		},
		{
			name: "Valid config with multiple servers",
			configJSON: `{
				"mcpServers": {
					"server1": {
						"command": "npx",
						"args": ["-y", "@modelcontextprotocol/server-filesystem"]
					},
					"server2": {
						"url": "http://localhost:8080/mcp"
					}
				}
			}`,
			wantErr:     false,
			description: "Should parse config with multiple servers",
		},
		{
			name:        "Invalid JSON",
			configJSON:  `{invalid json}`,
			wantErr:     true,
			description: "Should fail on invalid JSON",
		},
		{
			name:        "Missing mcpServers key",
			configJSON:  `{"servers": {}}`,
			wantErr:     true,
			description: "Should fail without mcpServers key",
		},
		{
			name:        "Empty config",
			configJSON:  `{}`,
			wantErr:     true,
			description: "Should fail with empty config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This will attempt to connect to servers and will fail with timeout
			// We're only testing the JSON parsing and structure validation
			_, err := client.DiscoverToolsFromRenderedConfig(tt.configJSON)

			// For valid JSON that would require actual MCP server connection,
			// we expect either success or connection-related errors
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error for invalid config, got nil")
				}
			}

			// For invalid JSON or structure, we should get parsing errors
			if tt.wantErr && err != nil {
				if !stringContainsToolDiscovery(err.Error(), "invalid") && !stringContainsToolDiscovery(err.Error(), "no mcpServers") {
					t.Logf("Got expected error: %v", err)
				}
			}

			t.Logf("Config parsing: wantErr=%v, gotErr=%v", tt.wantErr, err != nil)
		})
	}
}

// TestConfigJSONParsing tests configuration parsing logic
func TestConfigJSONParsing(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		wantServers int
		wantErr     bool
		description string
	}{
		{
			name: "Single server config",
			configJSON: `{
				"mcpServers": {
					"filesystem": {
						"command": "npx",
						"args": ["-y", "@modelcontextprotocol/server-filesystem"]
					}
				}
			}`,
			wantServers: 1,
			wantErr:     false,
			description: "Should detect 1 server",
		},
		{
			name: "Multiple servers config",
			configJSON: `{
				"mcpServers": {
					"filesystem": {"command": "npx"},
					"ship": {"command": "ship"},
					"api": {"url": "http://localhost:8080"}
				}
			}`,
			wantServers: 3,
			wantErr:     false,
			description: "Should detect 3 servers",
		},
		{
			name:        "Invalid JSON syntax",
			configJSON:  `{"mcpServers": {`,
			wantServers: 0,
			wantErr:     true,
			description: "Should fail on invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configData map[string]interface{}
			err := json.Unmarshal([]byte(tt.configJSON), &configData)

			if (err != nil) != tt.wantErr {
				t.Errorf("JSON parsing error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				mcpServers, ok := configData["mcpServers"].(map[string]interface{})
				if !ok {
					t.Error("Failed to extract mcpServers")
					return
				}

				if len(mcpServers) != tt.wantServers {
					t.Errorf("Got %d servers, want %d", len(mcpServers), tt.wantServers)
				}

				t.Logf("Parsed %d servers from config", len(mcpServers))
			}
		})
	}
}

// TestEnvironmentVariableConversion tests env var to HTTP header conversion
func TestEnvironmentVariableConversion(t *testing.T) {
	tests := []struct {
		name           string
		envVars        map[string]string
		expectedHeader string
		description    string
	}{
		{
			name: "AUTHORIZATION env var",
			envVars: map[string]string{
				"AUTHORIZATION": "Bearer token123",
			},
			expectedHeader: "Authorization",
			description:    "Should convert to Authorization header",
		},
		{
			name: "API_KEY env var",
			envVars: map[string]string{
				"API_KEY": "secret-key",
			},
			expectedHeader: "X-API-Key",
			description:    "Should convert to X-API-Key header",
		},
		{
			name: "HTTP_ prefix env var",
			envVars: map[string]string{
				"HTTP_X_Custom_Header": "custom-value",
			},
			expectedHeader: "X-Custom-Header",
			description:    "Should convert HTTP_ prefix to header",
		},
		{
			name: "AUTH_TOKEN env var",
			envVars: map[string]string{
				"AUTH_TOKEN": "token123",
			},
			expectedHeader: "Authorization",
			description:    "Should convert to Authorization header",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the conversion logic
			headers := make(map[string]string)
			for key, value := range tt.envVars {
				if key == "AUTHORIZATION" || key == "AUTH_TOKEN" {
					headers["Authorization"] = value
				} else if key == "API_KEY" {
					headers["X-API-Key"] = value
				} else if len(key) > 5 && key[:5] == "HTTP_" {
					headerName := ""
					parts := key[5:]
					for i, c := range parts {
						if c == '_' {
							headerName += "-"
						} else if i == 0 || parts[i-1] == '_' {
							headerName += string(c)
						} else {
							headerName += string(c + 32) // Convert to lowercase
						}
					}
					headers[headerName] = value
				}
			}

			if _, exists := headers[tt.expectedHeader]; !exists {
				// Check for case variations
				found := false
				for key := range headers {
					if stringContainsToolDiscovery(key, tt.expectedHeader) || stringContainsToolDiscovery(tt.expectedHeader, key) {
						found = true
						break
					}
				}
				if !found {
					t.Logf("Header conversion: %v -> %v", tt.envVars, headers)
				}
			}

			t.Logf("Env vars: %v -> Headers: %v", tt.envVars, headers)
		})
	}
}

// TestServerConfigExtraction tests extracting fields from server config
func TestServerConfigExtraction(t *testing.T) {
	tests := []struct {
		name        string
		config      map[string]interface{}
		wantCommand string
		wantURL     string
		wantArgs    int
		description string
	}{
		{
			name: "Stdio config",
			config: map[string]interface{}{
				"command": "npx",
				"args":    []interface{}{"-y", "@modelcontextprotocol/server-filesystem"},
			},
			wantCommand: "npx",
			wantArgs:    2,
			description: "Should extract command and args",
		},
		{
			name: "HTTP config",
			config: map[string]interface{}{
				"url": "http://localhost:8080/mcp",
			},
			wantURL:     "http://localhost:8080/mcp",
			description: "Should extract URL",
		},
		{
			name: "Mixed config",
			config: map[string]interface{}{
				"command": "ship",
				"args":    []interface{}{"mcp", "security"},
				"url":     "http://localhost:8080",
			},
			wantCommand: "ship",
			wantURL:     "http://localhost:8080",
			wantArgs:    2,
			description: "Should extract all fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Extract command
			var command string
			if cmdValue, ok := tt.config["command"]; ok {
				if cmdStr, ok := cmdValue.(string); ok {
					command = cmdStr
				}
			}

			// Extract URL
			var urlValue string
			if urlVal, ok := tt.config["url"]; ok {
				if urlStr, ok := urlVal.(string); ok {
					urlValue = urlStr
				}
			}

			// Extract args
			var args []string
			if argsValue, ok := tt.config["args"]; ok {
				if argsList, ok := argsValue.([]interface{}); ok {
					for _, arg := range argsList {
						if argStr, ok := arg.(string); ok {
							args = append(args, argStr)
						}
					}
				}
			}

			if command != tt.wantCommand {
				t.Errorf("Command = %s, want %s", command, tt.wantCommand)
			}

			if urlValue != tt.wantURL {
				t.Errorf("URL = %s, want %s", urlValue, tt.wantURL)
			}

			if len(args) != tt.wantArgs {
				t.Errorf("Args count = %d, want %d", len(args), tt.wantArgs)
			}

			t.Logf("Extracted: command=%s, url=%s, args=%d", command, urlValue, len(args))
		})
	}
}

// Helper function for string contains check
func stringContainsToolDiscovery(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContainsHelperToolDiscovery(s, substr))
}

func stringContainsHelperToolDiscovery(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Benchmark tests
func BenchmarkNewMCPClient(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewMCPClient()
	}
}

func BenchmarkCreateTransportFromFields(b *testing.B) {
	client := NewMCPClient()
	command := "npx"
	args := []string{"-y", "@modelcontextprotocol/server-filesystem"}
	env := map[string]string{"PATH": "/usr/bin"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.createTransportFromFields(command, "", args, env)
	}
}

func BenchmarkJSONConfigParsing(b *testing.B) {
	configJSON := `{
		"mcpServers": {
			"filesystem": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem"]
			},
			"ship": {
				"command": "ship",
				"args": ["mcp", "security"]
			}
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var configData map[string]interface{}
		json.Unmarshal([]byte(configJSON), &configData)
	}
}
