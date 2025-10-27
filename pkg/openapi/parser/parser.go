package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Parser represents an OpenAPI parser
type Parser struct {
	doc              *openapi3.T
	ValidateDocument bool
}

// NewParser creates a new OpenAPI parser
func NewParser() *Parser {
	return &Parser{
		ValidateDocument: false, // Default to no validation
	}
}

// SetValidation sets whether to validate the OpenAPI document
func (p *Parser) SetValidation(validate bool) {
	p.ValidateDocument = validate
}

// ParseFile parses an OpenAPI document from a file
func (p *Parser) ParseFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read OpenAPI file: %w", err)
	}

	return p.Parse(data)
}

// Parse parses an OpenAPI document from bytes
func (p *Parser) Parse(data []byte) error {
	// Validate input data is not empty
	if len(data) == 0 {
		return fmt.Errorf("input data is empty")
	}

	loader := openapi3.NewLoader()

	// Try to parse as JSON first
	var doc *openapi3.T
	var err error

	// Parse the document (loader can handle both JSON and YAML)
	doc, err = loader.LoadFromData(data)

	if err != nil {
		return fmt.Errorf("failed to parse OpenAPI document: %w", err)
	}

	// Basic validation: ensure it's an OpenAPI document
	// Check for required fields that any valid OpenAPI/Swagger spec should have
	if doc.OpenAPI == "" || doc.Info == nil {
		return fmt.Errorf("invalid OpenAPI structure: missing required fields")
	}

	// Validate the document if validation is enabled
	if p.ValidateDocument {
		err = doc.Validate(context.Background())
		if err != nil {
			return fmt.Errorf("invalid OpenAPI document: %w", err)
		}
	}

	p.doc = doc
	return nil
}

// GetDocument returns the parsed OpenAPI document
func (p *Parser) GetDocument() *openapi3.T {
	return p.doc
}

// GetPaths returns all paths in the OpenAPI document
func (p *Parser) GetPaths() map[string]*openapi3.PathItem {
	if p.doc == nil || p.doc.Paths == nil {
		return nil
	}
	return p.doc.Paths.Map()
}

// GetServers returns all servers in the OpenAPI document
func (p *Parser) GetServers() []*openapi3.Server {
	if p.doc == nil {
		return nil
	}
	return p.doc.Servers
}

// GetInfo returns the info section of the OpenAPI document
func (p *Parser) GetInfo() *openapi3.Info {
	if p.doc == nil {
		return nil
	}
	return p.doc.Info
}

// isJSON checks if the data is in JSON format
func isJSON(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}

// GetOperationID generates an operation ID if one is not provided
func (p *Parser) GetOperationID(path string, method string, operation *openapi3.Operation) string {
	if operation.OperationID != "" {
		return operation.OperationID
	}

	// Generate an operation ID based on the path and method
	pathName := strings.ReplaceAll(path, "/", "_")
	pathName = strings.ReplaceAll(pathName, "{", "")
	pathName = strings.ReplaceAll(pathName, "}", "")
	return fmt.Sprintf("%s%s", strings.ToLower(method), pathName)
}
