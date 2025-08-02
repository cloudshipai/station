package variables

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/afero"
	"station/pkg/config"
)

// EnvVariableStore implements the VariableStore interface for .env files
type EnvVariableStore struct {
	fs afero.Fs
}

// NewEnvVariableStore creates a new environment variable store
func NewEnvVariableStore(fs afero.Fs) *EnvVariableStore {
	return &EnvVariableStore{
		fs: fs,
	}
}

// Load loads variables from a .env file
func (s *EnvVariableStore) Load(ctx context.Context, filePath string) (map[string]interface{}, error) {
	// Check if file exists
	exists, err := afero.Exists(s.fs, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check file existence: %w", err)
	}
	if !exists {
		return make(map[string]interface{}), nil // Return empty map if file doesn't exist
	}
	
	// Open and read the file
	file, err := s.fs.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	variables := make(map[string]interface{})
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format at line %d: %s", lineNumber, line)
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		
		// Remove quotes if present
		value = s.unquoteValue(value)
		
		// Try to parse the value as the appropriate type
		variables[key] = s.parseValue(value)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	return variables, nil
}

// Save saves variables to a .env file
func (s *EnvVariableStore) Save(ctx context.Context, filePath string, variables map[string]interface{}) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := s.fs.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Create or truncate the file
	file, err := s.fs.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()
	
	// Write header comment
	if _, err := fmt.Fprintf(file, "# Station MCP Configuration Variables\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "# Generated on %s\n", time.Now().Format(time.RFC3339)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(file, "# DO NOT commit this file to version control\n\n"); err != nil {
		return err
	}
	
	// Write variables in sorted order for consistency
	keys := make([]string, 0, len(variables))
	for key := range variables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	
	for _, key := range keys {
		value := variables[key]
		formattedValue := s.formatValue(value)
		
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, formattedValue); err != nil {
			return fmt.Errorf("failed to write variable %s: %w", key, err)
		}
	}
	
	// Set restrictive permissions for security
	if err := s.fs.Chmod(filePath, 0600); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	
	return nil
}

// Merge merges two variable maps with the new map taking precedence
func (s *EnvVariableStore) Merge(existing, new map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	// Copy existing variables
	for key, value := range existing {
		result[key] = value
	}
	
	// Override with new variables
	for key, value := range new {
		result[key] = value
	}
	
	return result
}

// Validate validates variables against required template variables
func (s *EnvVariableStore) Validate(variables map[string]interface{}, required []config.TemplateVariable) error {
	var errors []string
	
	for _, templateVar := range required {
		if templateVar.Required {
			value, exists := variables[templateVar.Name]
			if !exists {
				errors = append(errors, fmt.Sprintf("required variable %s is missing", templateVar.Name))
				continue
			}
			
			// Check if value is empty
			if s.isEmpty(value) {
				errors = append(errors, fmt.Sprintf("required variable %s is empty", templateVar.Name))
				continue
			}
			
			// Validate type if specified
			if err := s.validateType(templateVar.Name, value, templateVar.Type); err != nil {
				errors = append(errors, err.Error())
			}
			
			// Validate against constraints
			if templateVar.Validation != nil {
				if err := s.validateConstraints(templateVar.Name, value, templateVar.Validation); err != nil {
					errors = append(errors, err.Error())
				}
			}
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("validation errors: %s", strings.Join(errors, "; "))
	}
	
	return nil
}

// Helper methods

func (s *EnvVariableStore) unquoteValue(value string) string {
	// Remove surrounding quotes if present
	if len(value) >= 2 {
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
			(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func (s *EnvVariableStore) parseValue(value string) interface{} {
	// Try to parse as different types
	
	// Boolean
	if strings.ToLower(value) == "true" {
		return true
	}
	if strings.ToLower(value) == "false" {
		return false
	}
	
	// Integer
	if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
		return intVal
	}
	
	// Float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}
	
	// Array (comma-separated)
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		var array []string
		for _, part := range parts {
			array = append(array, strings.TrimSpace(part))
		}
		return array
	}
	
	// Default to string
	return value
}

func (s *EnvVariableStore) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		// Quote strings that contain spaces or special characters
		if strings.ContainsAny(v, " \t\n\r\"'\\") {
			return fmt.Sprintf("\"%s\"", strings.ReplaceAll(v, "\"", "\\\""))
		}
		return v
	case bool:
		return strconv.FormatBool(v)
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%g", v)
	case []string:
		return strings.Join(v, ",")
	case []interface{}:
		var parts []string
		for _, item := range v {
			parts = append(parts, s.formatValue(item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (s *EnvVariableStore) isEmpty(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v == ""
	case nil:
		return true
	case []string:
		return len(v) == 0
	case []interface{}:
		return len(v) == 0
	default:
		return false
	}
}

func (s *EnvVariableStore) validateType(name string, value interface{}, expectedType config.VarType) error {
	switch expectedType {
	case config.VarTypeString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("variable %s should be a string, got %T", name, value)
		}
	case config.VarTypeNumber:
		switch value.(type) {
		case int, int32, int64, float32, float64:
			// OK
		default:
			return fmt.Errorf("variable %s should be a number, got %T", name, value)
		}
	case config.VarTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("variable %s should be a boolean, got %T", name, value)
		}
	case config.VarTypeArray:
		switch value.(type) {
		case []string, []interface{}:
			// OK
		default:
			return fmt.Errorf("variable %s should be an array, got %T", name, value)
		}
	}
	return nil
}

func (s *EnvVariableStore) validateConstraints(name string, value interface{}, validation *config.VarValidation) error {
	// String length validation
	if strVal, ok := value.(string); ok {
		if validation.MinLength != nil && len(strVal) < *validation.MinLength {
			return fmt.Errorf("variable %s is too short (minimum %d characters)", name, *validation.MinLength)
		}
		if validation.MaxLength != nil && len(strVal) > *validation.MaxLength {
			return fmt.Errorf("variable %s is too long (maximum %d characters)", name, *validation.MaxLength)
		}
		
		// Pattern validation
		if validation.Pattern != nil {
			matched, err := regexp.MatchString(*validation.Pattern, strVal)
			if err != nil {
				return fmt.Errorf("variable %s pattern validation error: %w", name, err)
			}
			if !matched {
				return fmt.Errorf("variable %s does not match required pattern", name)
			}
		}
		
		// Enum validation
		if len(validation.Enum) > 0 {
			found := false
			for _, allowed := range validation.Enum {
				if strVal == allowed {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("variable %s must be one of: %s", name, strings.Join(validation.Enum, ", "))
			}
		}
	}
	
	return nil
}

// Ensure we implement the interface
var _ config.VariableStore = (*EnvVariableStore)(nil)