package parser

import (
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	validOpenAPIJSON = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API",
    "version": "1.0.0",
    "description": "Test API for parser"
  },
  "servers": [
    {
      "url": "https://api.example.com/v1"
    }
  ],
  "paths": {
    "/users": {
      "get": {
        "operationId": "listUsers",
        "summary": "List users",
        "responses": {
          "200": {
            "description": "Success"
          }
        }
      },
      "post": {
        "operationId": "createUser",
        "summary": "Create user",
        "responses": {
          "201": {
            "description": "Created"
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

	validOpenAPIYAML = `openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://api.example.com/v1
paths:
  /health:
    get:
      operationId: healthCheck
      responses:
        '200':
          description: OK
`

	invalidOpenAPI = `{
  "openapi": "3.0.0",
  "info": {
    "title": "Test API"
  }
}`

	malformedJSON = `{
  "openapi": "3.0.0"
  "info": {
    "title": "broken"
  }
}`
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("NewParser returned nil")
	}
	if parser.ValidateDocument {
		t.Error("NewParser should default ValidateDocument to false")
	}
}

func TestSetValidation(t *testing.T) {
	parser := NewParser()

	parser.SetValidation(true)
	if !parser.ValidateDocument {
		t.Error("SetValidation(true) did not enable validation")
	}

	parser.SetValidation(false)
	if parser.ValidateDocument {
		t.Error("SetValidation(false) did not disable validation")
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "Valid OpenAPI JSON",
			data:    []byte(validOpenAPIJSON),
			wantErr: false,
		},
		{
			name:    "Valid OpenAPI YAML",
			data:    []byte(validOpenAPIYAML),
			wantErr: false,
		},
		{
			name:    "Malformed JSON",
			data:    []byte(malformedJSON),
			wantErr: true,
		},
		{
			name:    "Empty data",
			data:    []byte(""),
			wantErr: true,
		},
		{
			name:    "Invalid OpenAPI structure",
			data:    []byte(`{"random": "data"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			err := parser.Parse(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && parser.GetDocument() == nil {
				t.Error("Parse() succeeded but document is nil")
			}
		})
	}
}

func TestParseWithValidation(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "Valid OpenAPI with validation",
			data:    []byte(validOpenAPIJSON),
			wantErr: false,
		},
		{
			name:    "Invalid OpenAPI with validation",
			data:    []byte(invalidOpenAPI),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser()
			parser.SetValidation(true)
			err := parser.Parse(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() with validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	// Create temp file with valid OpenAPI spec
	tmpFile, err := os.CreateTemp("", "openapi-test-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(validOpenAPIJSON); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Create temp YAML file
	tmpYAMLFile, err := os.CreateTemp("", "openapi-test-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp YAML file: %v", err)
	}
	defer os.Remove(tmpYAMLFile.Name())

	if _, err := tmpYAMLFile.WriteString(validOpenAPIYAML); err != nil {
		t.Fatalf("Failed to write temp YAML file: %v", err)
	}
	tmpYAMLFile.Close()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "Valid JSON file",
			path:    tmpFile.Name(),
			wantErr: false,
		},
		{
			name:    "Valid YAML file",
			path:    tmpYAMLFile.Name(),
			wantErr: false,
		},
		{
			name:    "Nonexistent file",
			path:    "/tmp/nonexistent-openapi.json",
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
			parser := NewParser()
			err := parser.ParseFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetDocument(t *testing.T) {
	parser := NewParser()

	// Before parsing
	if doc := parser.GetDocument(); doc != nil {
		t.Error("GetDocument() should return nil before parsing")
	}

	// After parsing
	if err := parser.Parse([]byte(validOpenAPIJSON)); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	if doc := parser.GetDocument(); doc == nil {
		t.Error("GetDocument() should return document after parsing")
	}
}

func TestGetPaths(t *testing.T) {
	parser := NewParser()

	// Before parsing
	if paths := parser.GetPaths(); paths != nil {
		t.Error("GetPaths() should return nil before parsing")
	}

	// After parsing
	if err := parser.Parse([]byte(validOpenAPIJSON)); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	paths := parser.GetPaths()
	if paths == nil {
		t.Fatal("GetPaths() returned nil after parsing")
	}

	// Check expected paths
	expectedPaths := []string{"/users", "/users/{id}"}
	for _, expectedPath := range expectedPaths {
		if _, exists := paths[expectedPath]; !exists {
			t.Errorf("GetPaths() missing expected path: %s", expectedPath)
		}
	}

	// Check operations
	usersPath := paths["/users"]
	if usersPath.Get == nil {
		t.Error("GET operation missing for /users")
	}
	if usersPath.Post == nil {
		t.Error("POST operation missing for /users")
	}
}

func TestGetServers(t *testing.T) {
	parser := NewParser()

	// Before parsing
	if servers := parser.GetServers(); servers != nil {
		t.Error("GetServers() should return nil before parsing")
	}

	// After parsing
	if err := parser.Parse([]byte(validOpenAPIJSON)); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	servers := parser.GetServers()
	if servers == nil {
		t.Fatal("GetServers() returned nil after parsing")
	}

	if len(servers) != 1 {
		t.Errorf("GetServers() returned %d servers, expected 1", len(servers))
	}

	if servers[0].URL != "https://api.example.com/v1" {
		t.Errorf("GetServers() returned URL %s, expected https://api.example.com/v1", servers[0].URL)
	}
}

func TestGetInfo(t *testing.T) {
	parser := NewParser()

	// Before parsing
	if info := parser.GetInfo(); info != nil {
		t.Error("GetInfo() should return nil before parsing")
	}

	// After parsing
	if err := parser.Parse([]byte(validOpenAPIJSON)); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	info := parser.GetInfo()
	if info == nil {
		t.Fatal("GetInfo() returned nil after parsing")
	}

	if info.Title != "Test API" {
		t.Errorf("GetInfo() title = %s, expected Test API", info.Title)
	}

	if info.Version != "1.0.0" {
		t.Errorf("GetInfo() version = %s, expected 1.0.0", info.Version)
	}
}

func TestGetOperationID(t *testing.T) {
	parser := NewParser()

	if err := parser.Parse([]byte(validOpenAPIJSON)); err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	paths := parser.GetPaths()

	tests := []struct {
		name       string
		path       string
		method     string
		expectedID string
	}{
		{
			name:       "Explicit operation ID",
			path:       "/users",
			method:     "GET",
			expectedID: "listUsers",
		},
		{
			name:       "Generated operation ID",
			path:       "/users",
			method:     "POST",
			expectedID: "createUser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pathItem := paths[tt.path]
			if pathItem == nil {
				t.Fatalf("Path %s not found", tt.path)
			}

			var operation *openapi3.Operation

			switch tt.method {
			case "GET":
				operation = pathItem.Get
			case "POST":
				operation = pathItem.Post
			case "PUT":
				operation = pathItem.Put
			case "DELETE":
				operation = pathItem.Delete
			}

			if operation == nil {
				t.Fatalf("Operation %s not found for path %s", tt.method, tt.path)
			}

			opID := parser.GetOperationID(tt.path, tt.method, operation)
			if opID != tt.expectedID {
				t.Errorf("GetOperationID() = %s, expected %s", opID, tt.expectedID)
			}
		})
	}
}

// Benchmark tests
func BenchmarkParse(b *testing.B) {
	data := []byte(validOpenAPIJSON)
	for i := 0; i < b.N; i++ {
		parser := NewParser()
		_ = parser.Parse(data)
	}
}

func BenchmarkParseWithValidation(b *testing.B) {
	data := []byte(validOpenAPIJSON)
	for i := 0; i < b.N; i++ {
		parser := NewParser()
		parser.SetValidation(true)
		_ = parser.Parse(data)
	}
}

func BenchmarkGetPaths(b *testing.B) {
	parser := NewParser()
	_ = parser.Parse([]byte(validOpenAPIJSON))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.GetPaths()
	}
}
