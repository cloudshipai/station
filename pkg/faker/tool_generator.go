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
func (f *MCPFaker) generateToolsWithAI(ctx context.Context) ([]mcp.Tool, error) {
	if f.genkitApp == nil {
		return nil, fmt.Errorf("AI client not initialized")
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Generating tools with AI for instruction: %s\n", f.instruction)
	}

	// Build the prompt for tool generation
	prompt := buildToolGenerationPrompt(f.instruction)

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

// buildToolGenerationPrompt creates the prompt for AI tool generation
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
