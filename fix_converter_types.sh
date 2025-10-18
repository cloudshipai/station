#!/bin/bash

# Fix the converter.go file type issues

# Fix broken isSchemaType calls
sed -i 's/isSchemaType(schema, "array" &&/isSchemaType(schema, "array") \&\&/g' pkg/openapi/converter/converter.go
sed -i 's/isSchemaType(schema, "object" &&/isSchemaType(schema, "object") \&\&/g' pkg/openapi/converter/converter.go

# Fix direct Type assignments and comparisons
sed -i 's/\.Type = propRef\.Value\.Type/\.Type = getSchemaType(propRef.Value)/g' pkg/openapi/converter/converter.go
sed -i 's/"type": schema\.Items\.Value\.Type/"type": getSchemaType(schema.Items.Value)/g' pkg/openapi/converter/converter.go
sed -i 's/"type": propRef\.Value\.Items\.Value\.Type/"type": getSchemaType(propRef.Value.Items.Value)/g' pkg/openapi/converter/converter.go

# Fix Type comparisons
sed -i 's/\.Items\.Value\.Type == "object"/isSchemaType(schema.Items.Value, "object")/g' pkg/openapi/converter/converter.go
sed -i 's/propRef\.Value\.Type == "array"/isSchemaType(propRef.Value, "array")/g' pkg/openapi/converter/converter.go
sed -i 's/propRef\.Value\.Type == "object"/isSchemaType(propRef.Value, "object")/g' pkg/openapi/converter/converter.go
sed -i 's/propRef\.Value\.Type == ""/getSchemaType(propRef.Value) == ""/g' pkg/openapi/converter/converter.go

# Fix direct schema Type references
sed -i 's/schema\.Type != ""/getSchemaType(schema) != ""/g' pkg/openapi/converter/converter.go
sed -i 's/"type"] = schema\.Type/"type"] = getSchemaType(schema)/g' pkg/openapi/converter/converter.go

# Fix array item type checks
sed -i 's/arrayItemSchema\.Type == "object"/isSchemaType(arrayItemSchema, "object")/g' pkg/openapi/converter/converter.go
sed -i 's/arrayItemSchema\.Type != ""/getSchemaType(arrayItemSchema) != ""/g' pkg/openapi/converter/converter.go
sed -i 's/fmt.Sprintf("%s", arrayItemSchema.Type)/fmt.Sprintf("%s", getSchemaType(arrayItemSchema))/g' pkg/openapi/converter/converter.go

# Fix direct type assignments in convertNestedProperties
sed -i 's/"type"] = propRef\.Value\.Type/"type"] = getSchemaType(propRef.Value)/g' pkg/openapi/converter/converter.go
sed -i 's/propSchema\["type"\] = propRef\.Value\.Type/propSchema["type"] = getSchemaType(propRef.Value)/g' pkg/openapi/converter/converter.go
sed -i 's/"type"] = propRef\.Value\.Items\.Value\.Type/"type"] = getSchemaType(propRef.Value.Items.Value)/g' pkg/openapi/converter/converter.go

# Fix propRef.Value.Type references
sed -i 's/if propRef\.Value\.Type != ""/if getSchemaType(propRef.Value) != ""/g' pkg/openapi/converter/converter.go
sed -i 's/(Type: %s)", propRef\.Value\.Type/(Type: %s)", getSchemaType(propRef.Value)/g' pkg/openapi/converter/converter.go

echo "Fixed converter.go type issues"