package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
	"gopkg.in/yaml.v2"
	"station/cmd/main/handlers/common"
	agent_bundle "station/pkg/agent-bundle"
)

// extractInt64FromTokenUsage safely extracts int64 from various numeric types in token usage
func extractInt64FromTokenUsage(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case int64:
		return &v
	case int:
		val := int64(v)
		return &val
	case int32:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	case float32:
		val := int64(v)
		return &val
	default:
		return nil
	}
}

// loadVariablesFromFile loads variables from a YAML file
func (h *AgentHandler) loadVariablesFromFile(filePath string) (map[string]interface{}, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables file: %v", err)
	}

	var variables map[string]interface{}
	if err := yaml.Unmarshal(content, &variables); err != nil {
		return nil, fmt.Errorf("failed to parse variables file: %v", err)
	}

	return variables, nil
}

// loadManifestFromPath loads a bundle manifest from the given path
func (h *AgentHandler) loadManifestFromPath(bundlePath string) (*agent_bundle.AgentBundleManifest, error) {
	manifestPath := fmt.Sprintf("%s/bundle.yml", bundlePath)
	
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle manifest: %v", err)
	}

	var manifest agent_bundle.AgentBundleManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse bundle manifest: %v", err)
	}

	return &manifest, nil
}

// promptForVariables prompts the user for variable values
func (h *AgentHandler) promptForVariables(schema map[string]agent_bundle.VariableSpec, existingVars map[string]interface{}, styles common.CLIStyles) (map[string]interface{}, error) {
	variables := make(map[string]interface{})
	
	// Copy existing variables
	for k, v := range existingVars {
		variables[k] = v
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("📋 %s\n\n", styles.Info.Render("Please provide values for the following variables:"))

	for varName, spec := range schema {
		// Skip if we already have a value
		if _, exists := variables[varName]; exists {
			fmt.Printf("✓ %s: (already set)\n", styles.Success.Render(varName))
			continue
		}

		value, err := h.promptForSingleVariable(varName, spec, reader, styles)
		if err != nil {
			return nil, err
		}

		variables[varName] = value
	}

	return variables, nil
}

// promptForSingleVariable prompts for a single variable value
func (h *AgentHandler) promptForSingleVariable(varName string, spec agent_bundle.VariableSpec, reader *bufio.Reader, styles common.CLIStyles) (interface{}, error) {
	// Show variable info
	fmt.Printf("%s: %s\n", styles.Info.Render(varName), spec.Description)
	if spec.Default != nil {
		fmt.Printf("  Default: %s\n", h.formatVariableValue(spec.Default, spec.Sensitive))
	}
	fmt.Printf("  Type: %s\n", spec.Type)
	if spec.Sensitive {
		fmt.Printf("  %s\n", styles.Warning.Render("(sensitive - input will be hidden)"))
	}

	// Get input
	var input string
	var err error

	if spec.Sensitive {
		fmt.Printf("Enter value: ")
		bytePassword, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return nil, fmt.Errorf("failed to read password: %v", err)
		}
		input = string(bytePassword)
		fmt.Println() // Add newline after password input
	} else {
		fmt.Printf("Enter value: ")
		input, err = reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %v", err)
		}
	}

	input = strings.TrimSpace(input)

	// Use default if empty
	if input == "" && spec.Default != nil {
		return spec.Default, nil
	}

	if input == "" && spec.Required {
		return nil, fmt.Errorf("variable %s is required", varName)
	}

	// Convert to the correct type
	return h.convertVariableValue(input, spec)
}

// convertVariableValue converts a string input to the appropriate type
func (h *AgentHandler) convertVariableValue(input string, spec agent_bundle.VariableSpec) (interface{}, error) {
	switch spec.Type {
	case "string":
		return input, nil
	case "int":
		return strconv.Atoi(input)
	case "bool":
		return strconv.ParseBool(input)
	case "float":
		return strconv.ParseFloat(input, 64)
	default:
		return input, nil
	}
}

// formatVariableValue formats a variable value for display
func (h *AgentHandler) formatVariableValue(value interface{}, sensitive bool) string {
	if sensitive {
		return "***"
	}
	return fmt.Sprintf("%v", value)
}

// Mock implementations for bundle operations (placeholders)

type MockResolver struct{}

func (r *MockResolver) Resolve(ctx context.Context, deps []agent_bundle.MCPBundleDependency, env string) (*agent_bundle.ResolutionResult, error) {
	return &agent_bundle.ResolutionResult{
		Success:         true,
		ResolvedBundles: []agent_bundle.MCPBundleRef{},
		MissingBundles:  []agent_bundle.MCPBundleDependency{},
		Conflicts:       []agent_bundle.ToolConflict{},
		InstallOrder:    []string{},
	}, nil
}

func (r *MockResolver) InstallMCPBundles(ctx context.Context, bundles []agent_bundle.MCPBundleRef, env string) error {
	return nil
}

func (r *MockResolver) ValidateToolAvailability(ctx context.Context, tools []agent_bundle.ToolRequirement, env string) error {
	return nil
}

func (r *MockResolver) ResolveConflicts(conflicts []agent_bundle.ToolConflict) (*agent_bundle.ConflictResolution, error) {
	return &agent_bundle.ConflictResolution{
		Strategy:    "auto",
		Resolutions: make(map[string]string),
		Warnings:    []string{},
	}, nil
}