package converter

import "github.com/getkin/kin-openapi/openapi3"

// getSchemaType safely extracts the primary type from a schema
func getSchemaType(schema *openapi3.Schema) string {
	if schema == nil || schema.Type == nil || len(*schema.Type) == 0 {
		return ""
	}
	// Return the first type in the list
	return (*schema.Type)[0]
}

// isSchemaType checks if schema is of a specific type
func isSchemaType(schema *openapi3.Schema, typ string) bool {
	if schema == nil || schema.Type == nil {
		return false
	}
	return schema.Type.Is(typ)
}
