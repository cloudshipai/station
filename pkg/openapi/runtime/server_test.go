package runtime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"station/pkg/openapi/models"
)

// Test fixtures
const (
	validOpenAPISpec = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API",
    "version": "1.0.0"
  },
  "servers": [
    {
      "url": "http://test.example.com/api/v1"
    }
  ],
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "summary": "List all users",
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    },
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "summary": "Get user by ID",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}`

	validSwaggerSpec = `{
  "swagger": "2.0",
  "info": {
    "title": "Test API",
    "version": "1.0.0"
  },
  "host": "test.example.com",
  "basePath": "/api/v1",
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "summary": "List all users",
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      }
    }
  }
}`

	validMCPConfig = `{
  "name": "test-mcp",
  "version": "1.0.0",
  "tools": [
    {
      "name": "__testTool",
      "description": "Test tool",
      "inputSchema": {
        "type": "object",
        "properties": {}
      }
    }
  ]
}`
)

// TestNewServer tests server initialization
func TestNewServer(t *testing.T) {
	tests := []struct {
		name   string
		config ServerConfig
	}{
		{
			name:   "Empty config",
			config: ServerConfig{},
		},
		{
			name: "Config with path",
			config: ServerConfig{
				ConfigPath: "/tmp/nonexistent.json",
			},
		},
		{
			name: "Config with data",
			config: ServerConfig{
				ConfigData: validMCPConfig,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.config)
			if server == nil {
				t.Fatal("NewServer returned nil")
			}
			if server.httpClient == nil {
				t.Fatal("httpClient not initialized")
			}
		})
	}
}

// TestLoadConfigFromString tests loading config from string
func TestLoadConfigFromString(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "Valid OpenAPI 3.0 spec",
			config:  validOpenAPISpec,
			wantErr: false,
		},
		{
			name:    "Valid Swagger 2.0 spec",
			config:  validSwaggerSpec,
			wantErr: false,
		},
		{
			name:    "Valid MCP config",
			config:  validMCPConfig,
			wantErr: false,
		},
		{
			name:    "Invalid JSON",
			config:  `{"invalid": json}`,
			wantErr: true,
		},
		{
			name:    "Empty string",
			config:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{
				httpClient: &http.Client{},
			}
			err := server.LoadConfigFromString(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoadConfigFromFile tests loading config from file
func TestLoadConfigFromFile(t *testing.T) {
	// Create temp file with OpenAPI spec
	tmpFile, err := os.CreateTemp("", "openapi-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(validOpenAPISpec); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "Valid OpenAPI file",
			path:    tmpFile.Name(),
			wantErr: false,
		},
		{
			name:    "Nonexistent file",
			path:    "/tmp/nonexistent-openapi-spec.json",
			wantErr: true,
		},
		{
			name:    "Empty path",
			path:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{
				httpClient: &http.Client{},
			}
			err := server.LoadConfigFromFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoadConfigFromOpenAPISpec tests OpenAPI spec conversion
func TestLoadConfigFromOpenAPISpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    []byte
		wantErr bool
	}{
		{
			name:    "Valid OpenAPI 3.0 spec",
			spec:    []byte(validOpenAPISpec),
			wantErr: false,
		},
		{
			name:    "Invalid JSON spec",
			spec:    []byte(`{invalid json}`),
			wantErr: true,
		},
		{
			name:    "Empty spec",
			spec:    []byte(``),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{
				httpClient: &http.Client{},
			}
			err := server.LoadConfigFromOpenAPISpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromOpenAPISpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoadConfigFromBytes tests loading MCP config from bytes
func TestLoadConfigFromBytes(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "Valid JSON config",
			data:    []byte(validMCPConfig),
			wantErr: false,
		},
		{
			name: "Valid YAML config",
			data: []byte(`name: test-mcp
version: 1.0.0
tools:
  - name: __testTool
    description: Test tool
    inputSchema:
      type: object
`),
			wantErr: false,
		},
		{
			name:    "Invalid config",
			data:    []byte(`{invalid}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{
				httpClient: &http.Client{},
			}
			err := server.LoadConfigFromBytes(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetTools tests retrieving tools from server
func TestGetTools(t *testing.T) {
	tests := []struct {
		name       string
		config     *models.MCPConfig
		wantNil    bool
		wantLength int
	}{
		{
			name:    "No config loaded",
			config:  nil,
			wantNil: true,
		},
		{
			name: "Config with tools",
			config: &models.MCPConfig{
				Tools: []models.Tool{
					{
						Name:        "__testTool1",
						Description: "Test tool 1",
					},
					{
						Name:        "__testTool2",
						Description: "Test tool 2",
					},
				},
			},
			wantNil:    false,
			wantLength: 2,
		},
		{
			name: "Config with no tools",
			config: &models.MCPConfig{
				Tools: []models.Tool{},
			},
			wantNil:    false,
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{
				httpClient: &http.Client{},
				config:     tt.config,
			}
			tools := server.GetTools()
			if tt.wantNil && tools != nil {
				t.Errorf("GetTools() expected nil, got %v", tools)
			}
			if !tt.wantNil && len(tools) != tt.wantLength {
				t.Errorf("GetTools() length = %d, want %d", len(tools), tt.wantLength)
			}
		})
	}
}

// TestExecuteTool tests tool execution
func TestExecuteTool(t *testing.T) {
	// Create a test HTTP server
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"data":    "test response",
		})
	}))
	defer testServer.Close()

	server := &Server{
		httpClient: &http.Client{},
		config: &models.MCPConfig{
			Tools: []models.Tool{
				{
					Name:        "__testTool",
					Description: "Test tool",
					Args: []models.Arg{
						{
							Name:        "param1",
							Description: "Test parameter",
							Type:        "string",
							Required:    false,
						},
					},
					RequestTemplate: models.RequestTemplate{
						Method: "GET",
						URL:    testServer.URL + "/api/test",
					},
				},
			},
		},
	}

	tests := []struct {
		name     string
		toolName string
		args     map[string]interface{}
		wantErr  bool
	}{
		{
			name:     "Valid tool execution",
			toolName: "__testTool",
			args: map[string]interface{}{
				"param1": "value1",
			},
			wantErr: false,
		},
		{
			name:     "Tool not found",
			toolName: "__nonexistentTool",
			args:     map[string]interface{}{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := server.ExecuteTool(tt.toolName, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteTool() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("ExecuteTool() expected result, got nil")
			}
		})
	}
}

// TestLoadConfig tests environment-based config loading
func TestLoadConfig(t *testing.T) {
	// Save original env vars
	originalConfig := os.Getenv("OPENAPI_MCP_CONFIG")
	originalPath := os.Getenv("OPENAPI_MCP_CONFIG_PATH")
	defer func() {
		os.Setenv("OPENAPI_MCP_CONFIG", originalConfig)
		os.Setenv("OPENAPI_MCP_CONFIG_PATH", originalPath)
	}()

	tests := []struct {
		name       string
		envConfig  string
		envPath    string
		wantErr    bool
		setupFiles bool
	}{
		{
			name:      "Load from env config",
			envConfig: validMCPConfig,
			wantErr:   false,
		},
		{
			name:    "No config available",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.envConfig != "" {
				os.Setenv("OPENAPI_MCP_CONFIG", tt.envConfig)
			} else {
				os.Unsetenv("OPENAPI_MCP_CONFIG")
			}
			if tt.envPath != "" {
				os.Setenv("OPENAPI_MCP_CONFIG_PATH", tt.envPath)
			} else {
				os.Unsetenv("OPENAPI_MCP_CONFIG_PATH")
			}

			server := &Server{
				httpClient: &http.Client{},
			}
			err := server.LoadConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Benchmark tests for performance analysis
func BenchmarkLoadConfigFromString(b *testing.B) {
	server := &Server{
		httpClient: &http.Client{},
	}
	for i := 0; i < b.N; i++ {
		_ = server.LoadConfigFromString(validOpenAPISpec)
	}
}

func BenchmarkGetTools(b *testing.B) {
	server := &Server{
		httpClient: &http.Client{},
		config: &models.MCPConfig{
			Tools: make([]models.Tool, 100), // Simulate 100 tools
		},
	}
	for i := 0; i < b.N; i++ {
		_ = server.GetTools()
	}
}
