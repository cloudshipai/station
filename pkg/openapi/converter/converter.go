package converter

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	"station/pkg/openapi/models"
	"station/pkg/openapi/parser"
)

// Converter represents an OpenAPI to MCP converter
type Converter struct {
	parser  *parser.Parser
	options models.ConvertOptions
}

// NewConverter creates a new OpenAPI to MCP converter
func NewConverter(parser *parser.Parser, options models.ConvertOptions) *Converter {
	// Set default values if not provided
	if options.ServerName == "" {
		options.ServerName = "openapi-server"
	}
	if options.ServerConfig == nil {
		options.ServerConfig = make(map[string]any)
	}

	return &Converter{
		parser:  parser,
		options: options,
	}
}

// Convert converts an OpenAPI document to an MCP configuration
func (c *Converter) Convert() (*models.MCPConfig, error) {
	if c.parser.GetDocument() == nil {
		return nil, fmt.Errorf("no OpenAPI document loaded")
	}

	// Create the MCP configuration
	config := &models.MCPConfig{
		Server: models.ServerConfig{
			Name:            c.options.ServerName,
			Config:          c.options.ServerConfig,
			SecuritySchemes: []models.SecurityScheme{},
		},
		Tools: []models.Tool{},
	}

	// Process security schemes
	if c.parser.GetDocument().Components != nil && c.parser.GetDocument().Components.SecuritySchemes != nil {
		for name, schemeRef := range c.parser.GetDocument().Components.SecuritySchemes {
			if schemeRef != nil && schemeRef.Value != nil {
				scheme := schemeRef.Value
				mcpScheme := models.SecurityScheme{
					ID:     name,
					Type:   scheme.Type,
					Scheme: scheme.Scheme,
					In:     scheme.In,
					Name:   scheme.Name,
					// DefaultCredential is not directly available in OpenAPI SecurityScheme,
					// it's an extension for MCP. User can set it via template or manually.
				}
				config.Server.SecuritySchemes = append(config.Server.SecuritySchemes, mcpScheme)
			}
		}
		// Sort security schemes by ID for consistent output
		sort.Slice(config.Server.SecuritySchemes, func(i, j int) bool {
			return config.Server.SecuritySchemes[i].ID < config.Server.SecuritySchemes[j].ID
		})
	}

	// Process each path and operation
	for path, pathItem := range c.parser.GetPaths() {
		operations := getOperations(pathItem)
		for method, operation := range operations {
			tool, err := c.convertOperation(path, method, operation)
			if err != nil {
				return nil, fmt.Errorf("failed to convert operation %s %s: %w", method, path, err)
			}
			config.Tools = append(config.Tools, *tool)
		}
	}

	// Apply template if provided
	if c.options.TemplatePath != "" {
		err := c.applyTemplate(config)
		if err != nil {
			return nil, fmt.Errorf("failed to apply template: %w", err)
		}
	}

	// Sort tools by name for consistent output
	sort.Slice(config.Tools, func(i, j int) bool {
		return config.Tools[i].Name < config.Tools[j].Name
	})

	return config, nil
}

// applyTemplate applies a template to the generated configuration
func (c *Converter) applyTemplate(config *models.MCPConfig) error {
	// Read the template file
	templateData, err := os.ReadFile(c.options.TemplatePath)
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	// Parse the template
	var templateConfig models.MCPConfigTemplate
	err = yaml.Unmarshal(templateData, &templateConfig)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Apply server config
	if templateConfig.Server.Config != nil {
		if config.Server.Config == nil {
			config.Server.Config = make(map[string]any)
		}
		for k, v := range templateConfig.Server.Config {
			config.Server.Config[k] = v
		}
	}
	// Apply server security schemes
	// If template provides security schemes, they override existing ones.
	if len(templateConfig.Server.SecuritySchemes) > 0 {
		config.Server.SecuritySchemes = templateConfig.Server.SecuritySchemes
	}

	// Apply tool template to all tools
	if templateConfig.Tools.RequestTemplate != nil || templateConfig.Tools.ResponseTemplate != nil || templateConfig.Tools.Security != nil || templateConfig.Tools.OutputSchema != nil {
		for i := range config.Tools {
			// Apply request template
			if templateConfig.Tools.RequestTemplate != nil {
				// Merge headers
				if len(templateConfig.Tools.RequestTemplate.Headers) > 0 {
					config.Tools[i].RequestTemplate.Headers = append(
						config.Tools[i].RequestTemplate.Headers,
						templateConfig.Tools.RequestTemplate.Headers...,
					)
				}

				// Apply other request template fields
				if templateConfig.Tools.RequestTemplate.Body != "" {
					config.Tools[i].RequestTemplate.Body = templateConfig.Tools.RequestTemplate.Body
				}
				if templateConfig.Tools.RequestTemplate.ArgsToJsonBody {
					config.Tools[i].RequestTemplate.ArgsToJsonBody = true
				}
				if templateConfig.Tools.RequestTemplate.ArgsToUrlParam {
					config.Tools[i].RequestTemplate.ArgsToUrlParam = true
				}
				if templateConfig.Tools.RequestTemplate.ArgsToFormBody {
					config.Tools[i].RequestTemplate.ArgsToFormBody = true
				}
				// Apply request template security
				if templateConfig.Tools.RequestTemplate.Security != nil {
					config.Tools[i].RequestTemplate.Security = templateConfig.Tools.RequestTemplate.Security
				}
			}

			// Apply response template
			if templateConfig.Tools.ResponseTemplate != nil {
				if templateConfig.Tools.ResponseTemplate.Body != "" {
					config.Tools[i].ResponseTemplate.Body = templateConfig.Tools.ResponseTemplate.Body
				}
				if templateConfig.Tools.ResponseTemplate.PrependBody != "" {
					config.Tools[i].ResponseTemplate.PrependBody = templateConfig.Tools.ResponseTemplate.PrependBody
				}
				if templateConfig.Tools.ResponseTemplate.AppendBody != "" {
					config.Tools[i].ResponseTemplate.AppendBody = templateConfig.Tools.ResponseTemplate.AppendBody
				}
			}

			// Apply security
			if templateConfig.Tools.Security != nil {
				config.Tools[i].Security = templateConfig.Tools.Security
			}

			// Apply output schema
			if templateConfig.Tools.OutputSchema != nil {
				config.Tools[i].OutputSchema = templateConfig.Tools.OutputSchema
			}
		}
	}

	return nil
}

// getOperations returns a map of HTTP method to operation
func getOperations(pathItem *openapi3.PathItem) map[string]*openapi3.Operation {
	operations := make(map[string]*openapi3.Operation)

	if pathItem.Get != nil {
		operations["get"] = pathItem.Get
	}
	if pathItem.Post != nil {
		operations["post"] = pathItem.Post
	}
	if pathItem.Put != nil {
		operations["put"] = pathItem.Put
	}
	if pathItem.Delete != nil {
		operations["delete"] = pathItem.Delete
	}
	if pathItem.Options != nil {
		operations["options"] = pathItem.Options
	}
	if pathItem.Head != nil {
		operations["head"] = pathItem.Head
	}
	if pathItem.Patch != nil {
		operations["patch"] = pathItem.Patch
	}
	if pathItem.Trace != nil {
		operations["trace"] = pathItem.Trace
	}

	return operations
}

// convertOperation converts an OpenAPI operation to an MCP tool
func (c *Converter) convertOperation(path, method string, operation *openapi3.Operation) (*models.Tool, error) {
	// Generate a tool name
	toolName := c.parser.GetOperationID(path, method, operation)
	if c.options.ToolNamePrefix != "" {
		toolName = c.options.ToolNamePrefix + toolName
	}

	// Create the tool
	tool := &models.Tool{
		Name:        toolName,
		Description: getDescription(operation),
		Args:        []models.Arg{},
	}

	// Convert parameters to arguments
	args, err := c.convertParameters(operation.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to convert parameters: %w", err)
	}
	tool.Args = append(tool.Args, args...)

	// Convert request body to arguments
	bodyArgs, err := c.convertRequestBody(operation.RequestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to convert request body: %w", err)
	}
	tool.Args = append(tool.Args, bodyArgs...)

	// Sort arguments by name for consistent output
	sort.Slice(tool.Args, func(i, j int) bool {
		return tool.Args[i].Name < tool.Args[j].Name
	})

	// Create request template
	requestTemplate, err := c.createRequestTemplate(path, method, operation)
	if err != nil {
		return nil, fmt.Errorf("failed to create request template: %w", err)
	}
	tool.RequestTemplate = *requestTemplate

	// Create response template
	responseTemplate, err := c.createResponseTemplate(operation)
	if err != nil {
		return nil, fmt.Errorf("failed to create response template: %w", err)
	}
	tool.ResponseTemplate = *responseTemplate

	// Create output schema based on OpenAPI response schema (only if response schema exists)
	outputSchema, err := c.createOutputSchema(operation)
	if err != nil {
		return nil, fmt.Errorf("failed to create output schema: %w", err)
	}
	// Only set outputSchema if it was successfully generated (not nil)
	if outputSchema != nil {
		tool.OutputSchema = outputSchema
	}

	return tool, nil
}

// convertParameters converts OpenAPI parameters to MCP arguments
func (c *Converter) convertParameters(parameters openapi3.Parameters) ([]models.Arg, error) {
	args := []models.Arg{}

	for _, paramRef := range parameters {
		param := paramRef.Value
		if param == nil {
			continue
		}

		arg := models.Arg{
			Name:        param.Name,
			Description: param.Description,
			Required:    param.Required,
			Position:    param.In, // Set position based on parameter location (query, path, header, cookie)
		}

		// Set the type based on the schema
		if param.Schema != nil && param.Schema.Value != nil {
			schema := param.Schema.Value

			// Set the type based on the schema type
			arg.Type = getSchemaType(schema)

			// Handle enum values
			if len(schema.Enum) > 0 {
				arg.Enum = schema.Enum
			}

			// Handle array type recursively
			if isSchemaType(schema, "array") && schema.Items != nil && schema.Items.Value != nil {
				arg.Items = map[string]any{
					"type": getSchemaType(schema.Items.Value),
				}
				if schema.Items.Value.Description != "" {
					arg.Items["description"] = schema.Items.Value.Description
				}

				// Recursively handle array items if they are objects
				if isSchemaType(schema.Items.Value, "object") && len(schema.Items.Value.Properties) > 0 {
					nestedProps := c.convertNestedProperties(schema.Items.Value)
					if nestedProps != nil {
						arg.Items["properties"] = nestedProps["properties"]
						if required, ok := nestedProps["required"]; ok {
							arg.Items["required"] = required
						}
					}
				}
			}

			// Handle object type recursively
			if isSchemaType(schema, "object") && len(schema.Properties) > 0 {
				nestedProps := c.convertNestedProperties(schema)
				if nestedProps != nil {
					arg.Properties = nestedProps["properties"].(map[string]any)
				}
			}
		}

		args = append(args, arg)
	}

	return args, nil
}

// convertRequestBody converts an OpenAPI request body to MCP arguments
func (c *Converter) convertRequestBody(requestBodyRef *openapi3.RequestBodyRef) ([]models.Arg, error) {
	args := []models.Arg{}

	if requestBodyRef == nil || requestBodyRef.Value == nil {
		return args, nil
	}

	requestBody := requestBodyRef.Value

	// Process each content type
	for contentType, mediaType := range requestBody.Content {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		schema := mediaType.Schema.Value

		// For JSON and form content types, convert the schema to arguments
		if strings.Contains(contentType, "application/json") ||
			strings.Contains(contentType, "application/x-www-form-urlencoded") {

			// For object type, convert each property to an argument
			if isSchemaType(schema, "object") && len(schema.Properties) > 0 {
				for propName, propRef := range schema.Properties {
					if propRef.Value == nil {
						continue
					}

					arg := models.Arg{
						Name:        propName,
						Description: propRef.Value.Description,
						Type:        getSchemaType(propRef.Value),
						Required:    contains(schema.Required, propName),
						Position:    "body", // Set position to "body" for request body parameters
					}

					// Handle enum values
					if len(propRef.Value.Enum) > 0 {
						arg.Enum = propRef.Value.Enum
					}

					// Handle array type recursively
					if isSchemaType(propRef.Value, "array") && propRef.Value.Items != nil && propRef.Value.Items.Value != nil {
						arg.Items = map[string]any{
							"type":        getSchemaType(propRef.Value.Items.Value),
							"description": propRef.Value.Items.Value.Description,
						}
						if propRef.Value.Items.Value.MinItems > 0 {
							arg.Items["minItems"] = propRef.Value.Items.Value.MinItems
						}

						// Recursively handle array items if they are objects
						if isSchemaType(propRef.Value.Items.Value, "object") && len(propRef.Value.Items.Value.Properties) > 0 {
							nestedProps := c.convertNestedProperties(propRef.Value.Items.Value)
							if nestedProps != nil {
								arg.Items["properties"] = nestedProps["properties"]
								if required, ok := nestedProps["required"]; ok {
									arg.Items["required"] = required
								}
							}
						}
					}

					// Handle object type recursively
					if isSchemaType(propRef.Value, "object") && len(propRef.Value.Properties) > 0 {
						nestedProps := c.convertNestedProperties(propRef.Value)
						if nestedProps != nil {
							arg.Properties = nestedProps["properties"].(map[string]any)
						}
					}
					// Handle allOf
					if getSchemaType(propRef.Value) == "" && len(propRef.Value.AllOf) == 1 {
						arg.Type = "object"
						arg.Properties = c.allOfHandle(propRef.Value.AllOf[0])
					}

					args = append(args, arg)
				}
			}
		}
	}

	return args, nil
}

func (c *Converter) allOfHandle(schemaRef *openapi3.SchemaRef) map[string]interface{} {
	properties := make(map[string]interface{})
	if isSchemaType(schemaRef.Value, "object") {
		for propName, propRef := range schemaRef.Value.Properties {
			if propRef.Value != nil {
				properties[propName] = map[string]interface{}{
					"type": getSchemaType(propRef.Value),
				}
				if propRef.Value.Description != "" {
					properties[propName].(map[string]interface{})["description"] = propRef.Value.Description
				}
				if getSchemaType(propRef.Value) == "" && len(propRef.Value.AllOf) == 1 {
					properties[propName].(map[string]interface{})["type"] = "object"
					properties[propName].(map[string]interface{})["properties"] = c.allOfHandle(propRef.Value.AllOf[0])
				}
			}
		}
	}

	return properties
}

// createRequestTemplate creates an MCP request template from an OpenAPI operation
func (c *Converter) createRequestTemplate(path, method string, operation *openapi3.Operation) (*models.RequestTemplate, error) {
	// Get the server URL from the OpenAPI specification
	var serverURL string
	if servers := c.parser.GetDocument().Servers; len(servers) > 0 {
		serverURL = servers[0].URL
	}

	// Remove trailing slash from server URL if present
	serverURL = strings.TrimSuffix(serverURL, "/")

	// Create the request template
	template := &models.RequestTemplate{
		URL:     serverURL + path,
		Method:  strings.ToUpper(method),
		Headers: []models.Header{},
	}

	// Process operation-level security requirements
	securitySchemeFound := false
	if operation.Security != nil {
		for _, securityRequirement := range *operation.Security {
			if securitySchemeFound {
				break
			}
			for schemeName := range securityRequirement {
				// In MCP, we just reference the scheme by ID.
				// The actual application of security (e.g., adding headers)
				// would be handled by the MCP server runtime based on this ID.
				template.Security = &models.ToolSecurityRequirement{
					ID: schemeName,
				}
				securitySchemeFound = true
				break
			}
		}
	}

	// Add Content-Type header based on request body content type
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		for contentType := range operation.RequestBody.Value.Content {
			// Add the Content-Type header
			template.Headers = append(template.Headers, models.Header{
				Key:   "Content-Type",
				Value: contentType,
			})
			break // Just use the first content type
		}
	}

	return template, nil
}

// createResponseTemplate creates an MCP response template from an OpenAPI operation
func (c *Converter) createResponseTemplate(operation *openapi3.Operation) (*models.ResponseTemplate, error) {
	// Find the success response (200, 201, etc.)
	var successResponse *openapi3.Response

	if operation.Responses != nil {
		for code, responseRef := range operation.Responses.Map() {
			if strings.HasPrefix(code, "2") && responseRef != nil && responseRef.Value != nil {
				successResponse = responseRef.Value
				break
			}
		}
	}

	// If there's no success response, don't add a response template
	if successResponse == nil || len(successResponse.Content) == 0 {
		return &models.ResponseTemplate{}, nil
	}

	// Create the response template
	template := &models.ResponseTemplate{}

	// Generate the prepend body with response schema descriptions
	var prependBody strings.Builder
	prependBody.WriteString("# API Response Information\n\n")
	prependBody.WriteString("Below is the response from an API call. To help you understand the data, I've provided:\n\n")
	prependBody.WriteString("1. A detailed description of all fields in the response structure\n")
	prependBody.WriteString("2. The complete API response\n\n")
	prependBody.WriteString("## Response Structure\n\n")

	// Process each content type
	for contentType, mediaType := range successResponse.Content {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		prependBody.WriteString(fmt.Sprintf("> Content-Type: %s\n\n", contentType))
		schema := mediaType.Schema.Value

		// Generate field descriptions using recursive function
		if isSchemaType(schema, "array") && schema.Items != nil && schema.Items.Value != nil {
			// Handle array type
			prependBody.WriteString("- **items**: Array of items (Type: array)\n")
			// Process array items recursively
			c.processSchemaProperties(&prependBody, schema.Items.Value, "items", 1, 10)
		} else if isSchemaType(schema, "object") && len(schema.Properties) > 0 {
			// Get property names and sort them alphabetically for consistent output
			propNames := make([]string, 0, len(schema.Properties))
			for propName := range schema.Properties {
				propNames = append(propNames, propName)
			}
			sort.Strings(propNames)

			// Process properties in alphabetical order
			for _, propName := range propNames {
				propRef := schema.Properties[propName]
				if propRef.Value == nil {
					continue
				}

				// Write the property description
				prependBody.WriteString(fmt.Sprintf("- **%s**: %s", propName, propRef.Value.Description))
				if getSchemaType(propRef.Value) != "" {
					prependBody.WriteString(fmt.Sprintf(" (Type: %s)", getSchemaType(propRef.Value)))
				}
				prependBody.WriteString("\n")

				// Process nested properties recursively
				c.processSchemaProperties(&prependBody, propRef.Value, propName, 1, 10)
			}
		}
	}

	prependBody.WriteString("\n## Original Response\n\n")
	template.PrependBody = prependBody.String()

	return template, nil
}

// processSchemaProperties recursively processes schema properties and writes them to the prependBody
// path is the current property path (e.g., "data.items")
// depth is the current nesting depth (starts at 1)
// maxDepth is the maximum allowed nesting depth
func (c *Converter) processSchemaProperties(prependBody *strings.Builder, schema *openapi3.Schema, path string, depth, maxDepth int) {
	if depth > maxDepth {
		return // Stop recursion if max depth is reached
	}

	// Calculate indentation based on depth
	indent := strings.Repeat("  ", depth)

	// Handle array type
	if isSchemaType(schema, "array") && schema.Items != nil && schema.Items.Value != nil {
		arrayItemSchema := schema.Items.Value

		// Include the array description if available
		// arrayDesc := schema.Description
		// if arrayDesc == "" {
		// 	arrayDesc = fmt.Sprintf("Array of %s", arrayItemSchema.Type)
		// }

		// If array items are objects, describe their properties
		if isSchemaType(arrayItemSchema, "object") && len(arrayItemSchema.Properties) > 0 {
			// Sort property names for consistent output
			propNames := make([]string, 0, len(arrayItemSchema.Properties))
			for propName := range arrayItemSchema.Properties {
				propNames = append(propNames, propName)
			}
			sort.Strings(propNames)

			// Process each property
			for _, propName := range propNames {
				propRef := arrayItemSchema.Properties[propName]
				if propRef.Value == nil {
					continue
				}

				// Write the property description
				propPath := fmt.Sprintf("%s[].%s", path, propName)
				_, _ = fmt.Fprintf(prependBody, "%s- **%s**: %s", indent, propPath, propRef.Value.Description)
				if getSchemaType(propRef.Value) != "" {
					_, _ = fmt.Fprintf(prependBody, " (Type: %s)", getSchemaType(propRef.Value))
				}
				prependBody.WriteString("\n")

				// Process nested properties recursively
				c.processSchemaProperties(prependBody, propRef.Value, propPath, depth+1, maxDepth)
			}
		} else if getSchemaType(arrayItemSchema) != "" {
			// If array items are not objects, just describe the array item type
			_, _ = fmt.Fprintf(prependBody, "%s- **%s[]**: Items of type %s\n", indent, path, getSchemaType(arrayItemSchema))
		}
		return
	}

	// Handle object type
	if isSchemaType(schema, "object") && len(schema.Properties) > 0 {
		// Sort property names for consistent output
		propNames := make([]string, 0, len(schema.Properties))
		for propName := range schema.Properties {
			propNames = append(propNames, propName)
		}
		sort.Strings(propNames)

		// Process each property
		for _, propName := range propNames {
			propRef := schema.Properties[propName]
			if propRef.Value == nil {
				continue
			}

			// Write the property description
			propPath := fmt.Sprintf("%s.%s", path, propName)
			_, _ = fmt.Fprintf(prependBody, "%s- **%s**: %s", indent, propPath, propRef.Value.Description)
			if getSchemaType(propRef.Value) != "" {
				_, _ = fmt.Fprintf(prependBody, " (Type: %s)", getSchemaType(propRef.Value))
			}
			prependBody.WriteString("\n")

			// Process nested properties recursively
			c.processSchemaProperties(prependBody, propRef.Value, propPath, depth+1, maxDepth)
		}
	}
}

// getDescription returns a description for an operation
func getDescription(operation *openapi3.Operation) string {
	if operation.Summary != "" {
		if operation.Description != "" {
			return fmt.Sprintf("%s - %s", operation.Summary, operation.Description)
		}
		return operation.Summary
	}
	return operation.Description
}

// createOutputSchema creates an MCP output schema from an OpenAPI operation response
func (c *Converter) createOutputSchema(operation *openapi3.Operation) (map[string]any, error) {
	// Find the success response (200, 201, etc.)
	var successResponse *openapi3.Response
	if operation.Responses != nil {
		for code, responseRef := range operation.Responses.Map() {
			if strings.HasPrefix(code, "2") && responseRef != nil && responseRef.Value != nil {
				successResponse = responseRef.Value
				break
			}
		}
	}

	// If there's no success response, return empty schema
	if successResponse == nil || len(successResponse.Content) == 0 {
		return nil, nil
	}

	// Process the first content type (typically application/json)
	for _, mediaType := range successResponse.Content {
		if mediaType.Schema == nil || mediaType.Schema.Value == nil {
			continue
		}

		schema := mediaType.Schema.Value

		// Convert OpenAPI schema to MCP output schema
		outputSchema := make(map[string]any)

		// Set basic type information
		if getSchemaType(schema) != "" {
			outputSchema["type"] = getSchemaType(schema)
		}

		// Add description if available
		if successResponse.Description != nil && *successResponse.Description != "" {
			outputSchema["description"] = *successResponse.Description
		}

		// Handle array type recursively
		if isSchemaType(schema, "array") && schema.Items != nil && schema.Items.Value != nil {
			itemsSchema := make(map[string]any)
			itemsSchema["type"] = getSchemaType(schema.Items.Value)
			if schema.Items.Value.Description != "" {
				itemsSchema["description"] = schema.Items.Value.Description
			}

			// Recursively handle array items if they are objects
			if isSchemaType(schema.Items.Value, "object") && len(schema.Items.Value.Properties) > 0 {
				nestedProps := c.convertProperties(schema.Items.Value.Properties, schema.Items.Value.Required)
				itemsSchema["properties"] = nestedProps
				if len(schema.Items.Value.Required) > 0 {
					itemsSchema["required"] = schema.Items.Value.Required
				}
			}

			outputSchema["items"] = itemsSchema
		}

		// Handle object type with properties
		if isSchemaType(schema, "object") && len(schema.Properties) > 0 {
			properties := c.convertProperties(schema.Properties, schema.Required)
			outputSchema["properties"] = properties

			// Add required fields if any
			if len(schema.Required) > 0 {
				outputSchema["required"] = schema.Required
			}
		}

		return outputSchema, nil
	}

	return nil, nil
}

// convertProperties recursively converts OpenAPI properties to MCP output schema format
func (c *Converter) convertProperties(properties map[string]*openapi3.SchemaRef, required []string) map[string]any {
	return c.convertPropertiesWithVisited(properties, required, make(map[*openapi3.Schema]bool))
}

// convertPropertiesWithVisited recursively converts OpenAPI properties to MCP output schema format
// with circular reference detection using a visited map
func (c *Converter) convertPropertiesWithVisited(properties map[string]*openapi3.SchemaRef, _ []string, visited map[*openapi3.Schema]bool) map[string]any {
	result := make(map[string]any)

	// Get property names and sort them alphabetically for consistent output
	propNames := make([]string, 0, len(properties))
	for propName := range properties {
		propNames = append(propNames, propName)
	}
	sort.Strings(propNames)

	// Process each property
	for _, propName := range propNames {
		propRef := properties[propName]
		if propRef.Value == nil {
			continue
		}

		// Check for circular reference
		if visited[propRef.Value] {
			// Skip this property to avoid infinite recursion
			continue
		}

		propSchema := make(map[string]any)
		propSchema["type"] = getSchemaType(propRef.Value)

		if propRef.Value.Description != "" {
			propSchema["description"] = propRef.Value.Description
		}

		// Handle nested object properties recursively
		if isSchemaType(propRef.Value, "object") && len(propRef.Value.Properties) > 0 {
			// Mark this schema as visited
			visited[propRef.Value] = true
			nestedProps := c.convertPropertiesWithVisited(propRef.Value.Properties, propRef.Value.Required, visited)
			// Unmark after processing to allow the same schema in different branches
			delete(visited, propRef.Value)

			propSchema["properties"] = nestedProps

			// Add required fields for nested objects
			if len(propRef.Value.Required) > 0 {
				propSchema["required"] = propRef.Value.Required
			}
		}

		// Handle array properties recursively
		if isSchemaType(propRef.Value, "array") && propRef.Value.Items != nil && propRef.Value.Items.Value != nil {
			itemsSchema := make(map[string]any)
			itemsSchema["type"] = getSchemaType(propRef.Value.Items.Value)
			if propRef.Value.Items.Value.Description != "" {
				itemsSchema["description"] = propRef.Value.Items.Value.Description
			}

			// Recursively handle array items if they are objects
			if isSchemaType(propRef.Value.Items.Value, "object") && len(propRef.Value.Items.Value.Properties) > 0 {
				// Check for circular reference in array items
				if !visited[propRef.Value.Items.Value] {
					// Mark this schema as visited
					visited[propRef.Value.Items.Value] = true
					nestedProps := c.convertPropertiesWithVisited(propRef.Value.Items.Value.Properties, propRef.Value.Items.Value.Required, visited)
					// Unmark after processing to allow the same schema in different branches
					delete(visited, propRef.Value.Items.Value)

					itemsSchema["properties"] = nestedProps
					if len(propRef.Value.Items.Value.Required) > 0 {
						itemsSchema["required"] = propRef.Value.Items.Value.Required
					}
				}
			}

			propSchema["items"] = itemsSchema
		}

		result[propName] = propSchema
	}

	return result
}

// convertNestedProperties recursively converts nested properties for request body arguments
func (c *Converter) convertNestedProperties(schema *openapi3.Schema) map[string]any {
	return c.convertNestedPropertiesWithVisited(schema, make(map[*openapi3.Schema]bool))
}

// convertNestedPropertiesWithVisited recursively converts nested properties for request body arguments
// with circular reference detection using a visited map
func (c *Converter) convertNestedPropertiesWithVisited(schema *openapi3.Schema, visited map[*openapi3.Schema]bool) map[string]any {
	if schema == nil {
		return nil
	}

	// Check for circular reference
	if visited[schema] {
		// Return empty map to avoid infinite recursion
		return make(map[string]any)
	}

	result := make(map[string]any)

	// Handle object type with properties
	if isSchemaType(schema, "object") && len(schema.Properties) > 0 {
		properties := make(map[string]any)

		// Mark this schema as visited
		visited[schema] = true

		for propName, propRef := range schema.Properties {
			if propRef.Value == nil {
				continue
			}

			propSchema := make(map[string]any)

			// Add fields in alphabetical order for deterministic output: default, description, enum, type
			if propRef.Value.Default != nil {
				propSchema["default"] = propRef.Value.Default
			}

			if propRef.Value.Description != "" {
				propSchema["description"] = propRef.Value.Description
			}

			if len(propRef.Value.Enum) > 0 {
				propSchema["enum"] = propRef.Value.Enum
			}

			propSchema["type"] = getSchemaType(propRef.Value)

			// Recursively handle nested object properties
			if isSchemaType(propRef.Value, "object") && len(propRef.Value.Properties) > 0 {
				nestedProps := c.convertNestedPropertiesWithVisited(propRef.Value, visited)
				if nestedProps != nil {
					// Extract the actual properties to avoid double wrapping
					if props, ok := nestedProps["properties"]; ok {
						propSchema["properties"] = props
					}
				}
			}

			// Handle array type recursively
			if isSchemaType(propRef.Value, "array") && propRef.Value.Items != nil && propRef.Value.Items.Value != nil {
				itemsSchema := make(map[string]any)
				itemsSchema["type"] = getSchemaType(propRef.Value.Items.Value)
				if propRef.Value.Items.Value.Description != "" {
					itemsSchema["description"] = propRef.Value.Items.Value.Description
				}
				if propRef.Value.Items.Value.MinItems > 0 {
					itemsSchema["minItems"] = propRef.Value.Items.Value.MinItems
				}

				// Recursively handle array items if they are objects
				if isSchemaType(propRef.Value.Items.Value, "object") && len(propRef.Value.Items.Value.Properties) > 0 {
					nestedProps := c.convertNestedPropertiesWithVisited(propRef.Value.Items.Value, visited)
					if nestedProps != nil {
						// Extract the actual properties to avoid double wrapping
						if props, ok := nestedProps["properties"]; ok {
							itemsSchema["properties"] = props
						}
					}
				}

				propSchema["items"] = itemsSchema
			}

			properties[propName] = propSchema
		}

		// Unmark after processing to allow the same schema in different branches
		delete(visited, schema)

		result["properties"] = properties

		// Add required fields if any
		if len(schema.Required) > 0 {
			result["required"] = schema.Required
		}
	}

	return result
}

// contains checks if a string slice contains a string
func contains(slice []string, str string) bool {
	return slices.Contains(slice, str)
}
