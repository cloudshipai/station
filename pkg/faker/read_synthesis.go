package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/mark3labs/mcp-go/mcp"
)

// synthesizeReadResponse generates a read response based on accumulated write history
// This ensures that agents see consistent state across write/read operations
func (f *MCPFaker) synthesizeReadResponse(ctx context.Context, toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if f.sessionManager == nil || f.session == nil {
		return nil, fmt.Errorf("session management not initialized")
	}

	// Get all write events from this session
	writeHistory, err := f.sessionManager.GetWriteHistory(ctx, f.session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get write history: %w", err)
	}

	if f.debug {
		fmt.Printf("[FAKER READ SYNTHESIS] Synthesizing response for %s based on %d write operations\n",
			toolName, len(writeHistory))
	}

	// Create a new context with longer timeout for AI generation (30 seconds)
	// This prevents "context deadline exceeded" errors during OpenAI API calls
	synthCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = synthCtx

	// Build AI prompt with write history context
	prompt := f.buildReadSynthesisPrompt(toolName, arguments, writeHistory)

	// Use GenKit to generate response matching tool schema
	modelName := f.getModelName()

	// Define output schema for structured generation
	type SynthesizedResponse struct {
		Content []struct {
			Type string `json:"type"` // "text", "image", "resource"
			Text string `json:"text,omitempty"`
		} `json:"content"`
	}

	// Generate structured response
	output, _, err := genkit.GenerateData[SynthesizedResponse](ctx, f.genkitApp,
		ai.WithPrompt(prompt),
		ai.WithModelName(modelName))
	if err != nil {
		return nil, fmt.Errorf("AI synthesis failed: %w", err)
	}

	// Convert to MCP response format
	var mcpContent []mcp.Content
	for _, item := range output.Content {
		if item.Text != "" {
			mcpContent = append(mcpContent, mcp.NewTextContent(item.Text))
		}
	}

	if len(mcpContent) == 0 {
		return nil, fmt.Errorf("AI generated empty response")
	}

	result := &mcp.CallToolResult{
		Content: mcpContent,
		IsError: false,
	}

	if f.debug {
		resultJSON, _ := json.MarshalIndent(result, "", "  ")
		fmt.Printf("[FAKER READ SYNTHESIS] Synthesized response:\n%s\n", string(resultJSON))
	}

	return result, nil
}

// buildReadSynthesisPrompt creates an AI prompt for read synthesis
func (f *MCPFaker) buildReadSynthesisPrompt(toolName string, arguments map[string]interface{}, writeHistory []*FakerEvent) string {
	argsJSON, _ := json.MarshalIndent(arguments, "", "  ")

	prompt := fmt.Sprintf(`You are simulating a consistent world state for an AI agent.

Base Scenario:
%s

`, f.instruction)

	// Add write history
	if len(writeHistory) > 0 {
		prompt += f.sessionManager.BuildWriteHistoryPrompt(writeHistory)
	} else {
		prompt += "No previous write operations.\n\n"
	}

	// Add current read request
	prompt += fmt.Sprintf(`Current Read Request:
Tool: %s
Arguments:
%s

Task:
Generate a response that:
1. Reflects all previous write operations in this session
2. Maintains consistency with the base scenario
3. Returns realistic data that matches the tool's purpose
4. Uses valid JSON format

`, toolName, string(argsJSON))

	// Add tool schema hint if available
	if toolSchema, ok := f.toolSchemas[toolName]; ok {
		if toolSchema.Description != "" {
			prompt += fmt.Sprintf("Tool Description: %s\n\n", toolSchema.Description)
		}
	}

	prompt += `Return ONLY a JSON object with this structure:
{
  "content": [
    {
      "type": "text",
      "text": "<your synthesized response here>"
    }
  ]
}

The "text" field should contain the complete response data (can be JSON, plain text, or structured data).
Make sure the response accurately reflects the accumulated state from all previous write operations.`

	return prompt
}

// shouldSynthesizeRead determines if we should synthesize a read response
// based on whether there's relevant write history
func (f *MCPFaker) shouldSynthesizeRead(ctx context.Context) bool {
	// Only synthesize if we have session management enabled
	if f.sessionManager == nil || f.session == nil {
		return false
	}

	// Check if there are any write operations in this session
	writeHistory, err := f.sessionManager.GetWriteHistory(ctx, f.session.ID)
	if err != nil {
		return false
	}

	// Synthesize if there's write history (state has changed)
	return len(writeHistory) > 0
}

// recordToolEvent records a tool call event in the session
func (f *MCPFaker) recordToolEvent(ctx context.Context, toolName string, arguments map[string]interface{}, result *mcp.CallToolResult, operationType string) error {
	if f.sessionManager == nil || f.session == nil {
		return nil // Session not enabled, skip recording
	}

	// Extract response content for storage
	var responseContent interface{}
	if len(result.Content) > 0 {
		// Store content as array of objects
		contentArray := make([]map[string]interface{}, 0, len(result.Content))
		for _, content := range result.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				contentArray = append(contentArray, map[string]interface{}{
					"type": "text",
					"text": tc.Text,
				})
			}
		}
		responseContent = contentArray
	}

	event := &FakerEvent{
		SessionID:     f.session.ID,
		ToolName:      toolName,
		Arguments:     arguments,
		Response:      responseContent,
		OperationType: operationType,
		Timestamp:     time.Now(),
	}

	return f.sessionManager.RecordEvent(ctx, event)
}
