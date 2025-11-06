package faker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// SchemaNode represents a discovered schema structure
type SchemaNode struct {
	Type     string                 `json:"type"`      // "string", "number", "bool", "array", "object", "null"
	IsArray  bool                   `json:"is_array"`  // True if this is an array field
	ArrayMin int                    `json:"array_min"` // Min observed array length
	ArrayMax int                    `json:"array_max"` // Max observed array length
	Children map[string]*SchemaNode `json:"children"`  // For objects
	ItemType *SchemaNode            `json:"item_type"` // For arrays
	Samples  []interface{}          `json:"samples"`   // Sample values for pattern detection
}

// SchemaCache stores discovered schemas by tool name
type SchemaCache struct {
	mu      sync.RWMutex
	schemas map[string]*SchemaNode // tool name -> schema
	dir     string                 // cache directory
}

// NewSchemaCache creates a new schema cache
func NewSchemaCache(cacheDir string) (*SchemaCache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	cache := &SchemaCache{
		schemas: make(map[string]*SchemaNode),
		dir:     cacheDir,
	}

	// Load existing schemas from disk
	if err := cache.loadFromDisk(); err != nil {
		// Log but don't fail - cache will be rebuilt
		fmt.Fprintf(os.Stderr, "[faker] Warning: Failed to load schema cache: %v\n", err)
	}

	return cache, nil
}

// AnalyzeResponse analyzes a JSON response and updates the schema
func (sc *SchemaCache) AnalyzeResponse(toolName string, response interface{}) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Analyze the response structure
	schema := analyzeValue(response)

	// Get existing schema if available
	existing, exists := sc.schemas[toolName]
	if exists {
		// Merge with existing schema to capture variations
		schema = mergeSchemas(existing, schema)
	}

	sc.schemas[toolName] = schema

	// Persist to disk
	return sc.saveToDisk(toolName, schema)
}

// GetSchema retrieves a cached schema for a tool
func (sc *SchemaCache) GetSchema(toolName string) (*SchemaNode, bool) {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	schema, exists := sc.schemas[toolName]
	return schema, exists
}

// analyzeValue recursively analyzes a value and returns its schema
func analyzeValue(value interface{}) *SchemaNode {
	if value == nil {
		return &SchemaNode{Type: "null"}
	}

	switch v := value.(type) {
	case bool:
		return &SchemaNode{
			Type:    "bool",
			Samples: []interface{}{v},
		}

	case float64, int, int64:
		return &SchemaNode{
			Type:    "number",
			Samples: []interface{}{v},
		}

	case string:
		return &SchemaNode{
			Type:    "string",
			Samples: []interface{}{v},
		}

	case []interface{}:
		node := &SchemaNode{
			Type:     "array",
			IsArray:  true,
			ArrayMin: len(v),
			ArrayMax: len(v),
		}

		// Analyze array items
		if len(v) > 0 {
			// Analyze first item as representative
			node.ItemType = analyzeValue(v[0])

			// Merge schemas from other items to capture variations
			for i := 1; i < len(v) && i < 10; i++ {
				itemSchema := analyzeValue(v[i])
				node.ItemType = mergeSchemas(node.ItemType, itemSchema)
			}
		}

		return node

	case map[string]interface{}:
		node := &SchemaNode{
			Type:     "object",
			Children: make(map[string]*SchemaNode),
		}

		// Analyze each field
		for key, val := range v {
			node.Children[key] = analyzeValue(val)
		}

		return node

	default:
		return &SchemaNode{Type: "unknown"}
	}
}

// mergeSchemas merges two schemas to capture variations
func mergeSchemas(a, b *SchemaNode) *SchemaNode {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}

	// If types differ, keep the first one (could be enhanced)
	if a.Type != b.Type {
		return a
	}

	merged := &SchemaNode{
		Type:     a.Type,
		IsArray:  a.IsArray || b.IsArray,
		Children: make(map[string]*SchemaNode),
	}

	// Merge array bounds
	if a.IsArray && b.IsArray {
		merged.ArrayMin = min(a.ArrayMin, b.ArrayMin)
		merged.ArrayMax = max(a.ArrayMax, b.ArrayMax)

		// Merge item types
		if a.ItemType != nil && b.ItemType != nil {
			merged.ItemType = mergeSchemas(a.ItemType, b.ItemType)
		} else if a.ItemType != nil {
			merged.ItemType = a.ItemType
		} else {
			merged.ItemType = b.ItemType
		}
	}

	// Merge object children
	if a.Type == "object" {
		// Copy all fields from a
		for key, child := range a.Children {
			merged.Children[key] = child
		}

		// Merge fields from b
		for key, bChild := range b.Children {
			if aChild, exists := merged.Children[key]; exists {
				merged.Children[key] = mergeSchemas(aChild, bChild)
			} else {
				merged.Children[key] = bChild
			}
		}
	}

	// Merge samples (keep up to 10)
	merged.Samples = append(a.Samples, b.Samples...)
	if len(merged.Samples) > 10 {
		merged.Samples = merged.Samples[:10]
	}

	return merged
}

// saveToDisk persists a schema to disk
func (sc *SchemaCache) saveToDisk(toolName string, schema *SchemaNode) error {
	filename := filepath.Join(sc.dir, toolName+".json")

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal schema: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write schema file: %w", err)
	}

	return nil
}

// loadFromDisk loads all schemas from disk
func (sc *SchemaCache) loadFromDisk() error {
	entries, err := os.ReadDir(sc.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Cache directory doesn't exist yet
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		toolName := entry.Name()[:len(entry.Name())-5] // Remove .json
		filename := filepath.Join(sc.dir, entry.Name())

		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[faker] Warning: Failed to read schema file %s: %v\n", filename, err)
			continue
		}

		var schema SchemaNode
		if err := json.Unmarshal(data, &schema); err != nil {
			fmt.Fprintf(os.Stderr, "[faker] Warning: Failed to parse schema file %s: %v\n", filename, err)
			continue
		}

		sc.schemas[toolName] = &schema
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
