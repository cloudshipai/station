package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"station/internal/config"
	faker_ai "station/pkg/faker/ai"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/mark3labs/mcp-go/mcp"
)

// ToolDefinition represents a single MCP tool for structured AI generation
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"` // Store as raw JSON
}

// ToolsResponse represents the structured output from AI tool generation
type ToolsResponse struct {
	Tools []ToolDefinition `json:"tools"`
}

// generateToolsWithAI generates MCP tool schemas using AI based on the instruction
// Uses structured output with GenerateData for reliable tool generation
// If FAKER_TOOL_NAMES env var is set, constrains generation to those specific tool names
func (f *MCPFaker) generateToolsWithAI(ctx context.Context) ([]mcp.Tool, error) {
	if f.genkitApp == nil {
		return nil, fmt.Errorf("AI client not initialized")
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Generating tools with AI for instruction: %s\n", f.instruction)
	}

	// Check for predefined tool names (for deterministic tool generation across bundles)
	toolNamesEnv := os.Getenv("FAKER_TOOL_NAMES")
	var requestedToolNames []string

	if toolNamesEnv != "" {
		// Parse comma-separated list and trim whitespace
		rawNames := strings.Split(toolNamesEnv, ",")
		for _, name := range rawNames {
			trimmed := strings.TrimSpace(name)
			if trimmed != "" {
				requestedToolNames = append(requestedToolNames, trimmed)
			}
		}

		if len(requestedToolNames) > 0 {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] 🎯 Constrained generation mode: %d specific tools requested: %v\n",
					len(requestedToolNames), requestedToolNames)
			}
		}
	}

	// Build the prompt for tool generation (constrained or free-form)
	var prompt string
	if len(requestedToolNames) > 0 {
		prompt = buildConstrainedToolGenerationPrompt(f.instruction, requestedToolNames)
	} else {
		prompt = buildToolGenerationPrompt(f.instruction)
	}

	// Get model name using the same logic as Station's AI client
	modelName := getModelName(f.stationConfig)

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Using GenerateData with model: %s\n", modelName)
	}

	// Generate tools using AI with structured output
	result, _, err := genkit.GenerateData[ToolsResponse](ctx, f.genkitApp,
		ai.WithPrompt(prompt),
		ai.WithModelName(modelName))

	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	if len(result.Tools) == 0 {
		return nil, fmt.Errorf("AI generated no tools")
	}

	// Validate tool names match requested names (if constrained mode)
	if len(requestedToolNames) > 0 {
		if err := validateGeneratedTools(result.Tools, requestedToolNames); err != nil {
			return nil, fmt.Errorf("tool validation failed: %w", err)
		}
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] ✅ All %d requested tools generated successfully\n", len(requestedToolNames))
		}
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Generated %d tools\n", len(result.Tools))
		for _, tool := range result.Tools {
			fmt.Fprintf(os.Stderr, "[FAKER]   - %s: %s\n", tool.Name, tool.Description)
		}
	}

	// Convert ToolDefinition to mcp.Tool using NewToolWithRawSchema
	mcpTools := make([]mcp.Tool, len(result.Tools))
	for i, toolDef := range result.Tools {
		mcpTools[i] = mcp.NewToolWithRawSchema(
			toolDef.Name,
			toolDef.Description,
			toolDef.InputSchema,
		)
	}

	return mcpTools, nil
}

// getModelName returns the configured model name with provider prefix
// Uses the same logic as pkg/faker/ai package
func getModelName(cfg *config.Config) string {
	if cfg == nil {
		return "openai/gpt-4o-mini"
	}

	baseModel := cfg.AIModel
	if baseModel == "" {
		baseModel = faker_ai.GetDefaultModel(cfg.AIProvider)
	}

	switch strings.ToLower(cfg.AIProvider) {
	case "gemini", "googlegenai":
		return fmt.Sprintf("googleai/%s", baseModel)
	case "openai":
		return fmt.Sprintf("openai/%s", baseModel)
	default:
		return fmt.Sprintf("%s/%s", cfg.AIProvider, baseModel)
	}
}

// buildToolGenerationPrompt creates the prompt for AI tool generation (free-form)
func buildToolGenerationPrompt(instruction string) string {
	return fmt.Sprintf(`You are an MCP (Model Context Protocol) tool schema generator. Generate realistic tool schemas for an MCP server based on the following instruction:

INSTRUCTION: %s

Generate 5-10 realistic MCP tool definitions that would be useful for this use case. Each tool should have:
1. A clear, descriptive name (snake_case)
2. A helpful description
3. An inputSchema following JSON Schema format (with type, properties, required fields)

Return ONLY valid JSON in this exact format:
{
  "tools": [
    {
      "name": "tool_name",
      "description": "What this tool does",
      "inputSchema": {
        "type": "object",
        "properties": {
          "param1": {
            "type": "string",
            "description": "Parameter description"
          }
        },
        "required": ["param1"]
      }
    }
  ]
}

Focus on tools that would be realistic for this domain. Include both read operations (get, list, describe) and write operations (create, update, delete) where appropriate.

IMPORTANT: Return ONLY the JSON, no markdown code blocks, no explanations, just the raw JSON.`, instruction)
}

// buildConstrainedToolGenerationPrompt creates a prompt that constrains generation to specific tool names
// This ensures deterministic tool names across different Station installations (for bundle portability)
func buildConstrainedToolGenerationPrompt(instruction string, toolNames []string) string {
	toolList := strings.Join(toolNames, ", ")

	return fmt.Sprintf(`You are an MCP (Model Context Protocol) tool schema generator. Generate tool schemas for EXACTLY these specific tools:

REQUIRED TOOLS: %s

INSTRUCTION: %s

Generate schemas for each of the %d required tools listed above. The tool names MUST match exactly (including case and underscores).

For each tool:
1. Use the exact tool name from the list above
2. Infer appropriate description and parameters from the tool name and instruction
3. Create a realistic inputSchema with proper JSON Schema format

Return ONLY valid JSON in this exact format:
{
  "tools": [
    {
      "name": "exact_tool_name_from_list",
      "description": "What this tool does",
      "inputSchema": {
        "type": "object",
        "properties": {
          "param1": {
            "type": "string",
            "description": "Parameter description"
          }
        },
        "required": ["param1"]
      }
    }
  ]
}

CRITICAL: You MUST generate schemas for ALL %d tools. The tool names must match the required list EXACTLY.

IMPORTANT: Return ONLY the JSON, no markdown code blocks, no explanations, just the raw JSON.`,
		toolList, instruction, len(toolNames), len(toolNames))
}

// validateGeneratedTools ensures that AI-generated tools match the requested tool names
func validateGeneratedTools(generated []ToolDefinition, requested []string) error {
	if len(generated) != len(requested) {
		return fmt.Errorf("expected %d tools, but AI generated %d tools", len(requested), len(generated))
	}

	// Create a map of requested tool names for quick lookup
	requestedMap := make(map[string]bool)
	for _, name := range requested {
		requestedMap[name] = true
	}

	// Check each generated tool
	generatedNames := make([]string, 0, len(generated))
	for _, tool := range generated {
		generatedNames = append(generatedNames, tool.Name)
		if !requestedMap[tool.Name] {
			return fmt.Errorf("AI generated unexpected tool '%s', not in requested list: %v",
				tool.Name, requested)
		}
	}

	// Check for missing tools
	generatedMap := make(map[string]bool)
	for _, tool := range generated {
		generatedMap[tool.Name] = true
	}

	missing := []string{}
	for _, name := range requested {
		if !generatedMap[name] {
			missing = append(missing, name)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("AI failed to generate required tools: %v (generated: %v)",
			missing, generatedNames)
	}

	return nil
}
