package converter

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"station/pkg/openapi/models"
	"station/pkg/openapi/parser"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestEndToEndConversion(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		inputFile      string
		expectedOutput string
		serverName     string
		templatePath   string
	}{
		{
			name:           "With Ref Chain API",
			inputFile:      "../../test/with-ref-chain.json",
			expectedOutput: "../../test/expected-with-ref-chain.yaml",
			serverName:     "with-ref-chain-api",
		},
		{
			name:           "With Ref API",
			inputFile:      "../../test/with-ref.json",
			expectedOutput: "../../test/expected-with-ref-mcp.yaml",
			serverName:     "with-ref-api",
		},
		{
			name:           "Petstore API",
			inputFile:      "../../test/petstore.json",
			expectedOutput: "../../test/expected-petstore-mcp.yaml",
			serverName:     "petstore",
		},
		{
			name:           "Path Parameters API",
			inputFile:      "../../test/path-params.json",
			expectedOutput: "../../test/expected-path-params-mcp.yaml",
			serverName:     "path-params-api",
		},
		{
			name:           "Header Parameters API",
			inputFile:      "../../test/header-params.json",
			expectedOutput: "../../test/expected-header-params-mcp.yaml",
			serverName:     "header-params-api",
		},
		{
			name:           "Cookie Parameters API",
			inputFile:      "../../test/cookie-params.json",
			expectedOutput: "../../test/expected-cookie-params-mcp.yaml",
			serverName:     "cookie-params-api",
		},
		{
			name:           "Request Body Types API",
			inputFile:      "../../test/request-body-types.json",
			expectedOutput: "../../test/expected-request-body-types-mcp.yaml",
			serverName:     "request-body-types-api",
		},
		{
			name:           "Petstore API with Template",
			inputFile:      "../../test/petstore.json",
			expectedOutput: "../../test/expected-petstore-template-mcp.yaml",
			serverName:     "petstore",
			templatePath:   "../../test/template.yaml",
		},
		{
			name:           "Security Schemes API",
			inputFile:      "../../test/security-test.json",
			expectedOutput: "../../test/expected-security-test-mcp.yaml",
			serverName:     "openapi-server", // Matches the default or can be specified if different
		},
		{
			name:           "Tools Args array of object",
			inputFile:      "../../test/tools-args-array-of-object.json",
			expectedOutput: "../../test/expected-tools-args-array-of-object-mcp.yaml",
			serverName:     "openapi-server",
		},
		{
			name:           "Handle AllOf Parameters",
			inputFile:      "../../test/allof-params.json",
			expectedOutput: "../../test/expected-allof-params-mcp.yaml",
			serverName:     "openapi-server",
		},
		{
			name:           "Output Schema Test",
			inputFile:      "../../test/output-schema-test.json",
			expectedOutput: "../../test/expected-output-schema-test-mcp.yaml",
			serverName:     "output-schema-api",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new parser
			p := parser.NewParser()

			// Parse the OpenAPI specification
			err := p.ParseFile(tc.inputFile)
			assert.NoError(t, err)

			// Create a new converter
			c := NewConverter(p, models.ConvertOptions{
				ServerName:   tc.serverName,
				TemplatePath: tc.templatePath,
			})

			// Convert the OpenAPI specification to an MCP configuration
			config, err := c.Convert()
			assert.NoError(t, err)

			// Marshal the MCP configuration to YAML
			var buffer bytes.Buffer
			encoder := yaml.NewEncoder(&buffer)
			encoder.SetIndent(2)

			if err := encoder.Encode(config); err != nil {
				fmt.Printf("Error encoding YAML: %v\n", err)
				return
			}
			actualYAML := buffer.Bytes()
			assert.NoError(t, err)

			// If the expected output file doesn't exist, write the actual output to it
			if _, err := os.Stat(tc.expectedOutput); os.IsNotExist(err) {
				err = os.WriteFile(tc.expectedOutput, actualYAML, 0644)
				assert.NoError(t, err)
				t.Logf("Created expected output file: %s", tc.expectedOutput)
			}

			// Read the expected output
			expectedYAML, err := os.ReadFile(tc.expectedOutput)
			assert.NoError(t, err)

			// Compare the actual and expected output
			assert.Equal(t, string(expectedYAML), string(actualYAML))
		})
	}
}

func TestCreateOutputSchema(t *testing.T) {
	// Create a new parser
	p := parser.NewParser()

	// Parse the test file
	err := p.ParseFile("../../test/output-schema-test.json")
	assert.NoError(t, err)

	// Create a converter
	c := NewConverter(p, models.ConvertOptions{
		ServerName: "test-server",
	})

	// Get the document and operations
	doc := p.GetDocument()
	userOperation := doc.Paths.Find("/user/{id}").Get

	// Test createOutputSchema
	outputSchema, err := c.createOutputSchema(userOperation)
	assert.NoError(t, err)
	assert.NotNil(t, outputSchema)

	// Verify output schema structure
	assert.Equal(t, "object", outputSchema["type"])
	assert.Equal(t, "Successful response", outputSchema["description"])

	// Verify properties
	properties, ok := outputSchema["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, properties, "id")
	assert.Contains(t, properties, "name")
	assert.Contains(t, properties, "email")
	assert.Contains(t, properties, "profile")

	// Verify id property
	idProp, ok := properties["id"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "integer", idProp["type"])
	assert.Equal(t, "User ID", idProp["description"])

	// Verify profile property (nested object)
	profileProp, ok := properties["profile"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "object", profileProp["type"])
	assert.Equal(t, "User profile information", profileProp["description"])

	// Verify nested properties in profile
	profileProps, ok := profileProp["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, profileProps, "bio")
	assert.Contains(t, profileProps, "website")

	// Verify required fields
	required, ok := outputSchema["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "id")
	assert.Contains(t, required, "name")
	assert.Contains(t, required, "email")
}

func TestCreateOutputSchemaArray(t *testing.T) {
	// Create a new parser
	p := parser.NewParser()

	// Parse the test file
	err := p.ParseFile("../../test/output-schema-test.json")
	assert.NoError(t, err)

	// Create a converter
	c := NewConverter(p, models.ConvertOptions{
		ServerName: "test-server",
	})

	// Get the document and operations
	doc := p.GetDocument()
	usersOperation := doc.Paths.Find("/users").Get

	// Test createOutputSchema for array response
	outputSchema, err := c.createOutputSchema(usersOperation)
	assert.NoError(t, err)
	assert.NotNil(t, outputSchema)

	// Verify output schema structure for array response
	assert.Equal(t, "object", outputSchema["type"])
	assert.Equal(t, "Successful response", outputSchema["description"])

	// Verify properties
	properties, ok := outputSchema["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, properties, "users")
	assert.Contains(t, properties, "total")

	// Verify users property (array)
	usersProp, ok := properties["users"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "array", usersProp["type"])
	assert.Equal(t, "List of users", usersProp["description"])

	// Verify array items
	items, ok := usersProp["items"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, "object", items["type"])

	// Verify nested properties in array items
	itemsProps, ok := items["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, itemsProps, "id")
	assert.Contains(t, itemsProps, "name")
	assert.Contains(t, itemsProps, "email")

	// Verify required fields
	required, ok := outputSchema["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "users")
	assert.Contains(t, required, "total")
}

func TestConvertPropertiesRecursive(t *testing.T) {
	// Create a new parser
	p := parser.NewParser()

	// Parse the test file
	err := p.ParseFile("../../test/output-schema-test.json")
	assert.NoError(t, err)

	// Create a converter
	c := NewConverter(p, models.ConvertOptions{
		ServerName: "test-server",
	})

	// Get the document and user operation to test nested properties
	doc := p.GetDocument()
	userOperation := doc.Paths.Find("/user/{id}").Get

	// Get the response schema
	var successResponse *openapi3.Response
	if userOperation.Responses != nil {
		for code, responseRef := range userOperation.Responses {
			if strings.HasPrefix(code, "2") && responseRef != nil && responseRef.Value != nil {
				successResponse = responseRef.Value
				break
			}
		}
	}

	assert.NotNil(t, successResponse)
	assert.NotNil(t, successResponse.Content)

	// Get the schema from the first content type
	var schema *openapi3.Schema
	for _, mediaType := range successResponse.Content {
		if mediaType.Schema != nil && mediaType.Schema.Value != nil {
			schema = mediaType.Schema.Value
			break
		}
	}

	assert.NotNil(t, schema)

	// Test the convertProperties function directly
	if schema.Type == "object" && len(schema.Properties) > 0 {
		properties := c.convertProperties(schema.Properties, schema.Required)

		// Verify top-level properties
		assert.Contains(t, properties, "id")
		assert.Contains(t, properties, "name")
		assert.Contains(t, properties, "email")
		assert.Contains(t, properties, "profile")

		// Verify nested properties in profile
		profileProp, ok := properties["profile"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "object", profileProp["type"])

		profileProps, ok := profileProp["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, profileProps, "bio")
		assert.Contains(t, profileProps, "website")

		// Verify bio property in nested profile
		bioProp, ok := profileProps["bio"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "string", bioProp["type"])
		assert.Equal(t, "User biography", bioProp["description"])
	}
}
