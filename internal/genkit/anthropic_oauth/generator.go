package anthropic_oauth

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/firebase/genkit/go/ai"
)

// ClaudeCodeSystemPrompt is required for OAuth tokens (Claude Code credentials)
const ClaudeCodeSystemPrompt = "You are Claude Code, Anthropic's official CLI for Claude."

// Generator handles Anthropic API generation requests
type Generator struct {
	client                       *anthropic.Client
	modelName                    string
	injectClaudeCodeSystemPrompt bool // Inject Claude Code system prompt for OAuth tokens
}

// NewGenerator creates a new Generator instance
func NewGenerator(client *anthropic.Client, modelName string) *Generator {
	return &Generator{
		client:    client,
		modelName: modelName,
	}
}

// WithClaudeCodeSystemPrompt enables automatic injection of Claude Code system prompt
// This is required for OAuth tokens which are only authorized for Claude Code
func (g *Generator) WithClaudeCodeSystemPrompt() *Generator {
	g.injectClaudeCodeSystemPrompt = true
	return g
}

// Generate performs the API call to Anthropic with support for streaming and tools
func (g *Generator) Generate(ctx context.Context, req *ai.ModelRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	// Build the request parameters
	params, customSystemPrompt, err := g.buildParams(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build params: %w", err)
	}

	// Build system prompt blocks
	// For OAuth tokens, we MUST include the Claude Code system prompt first
	// Custom system prompts (from dotprompt/agent config) go in the second block
	// This follows the OpenCode pattern for prompt caching compatibility
	var systemBlocks []anthropic.TextBlockParam

	if g.injectClaudeCodeSystemPrompt {
		// First block: Required Claude Code header for OAuth
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: ClaudeCodeSystemPrompt,
		})
	}

	if customSystemPrompt != "" {
		// Second block: Custom system prompt from dotprompt/agent
		systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
			Text: customSystemPrompt,
		})
	}

	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}

	// Choose streaming or non-streaming based on callback
	if cb != nil {
		return g.generateStream(ctx, params, cb)
	}
	return g.generateComplete(ctx, params)
}

// buildParams converts GenKit ModelRequest to Anthropic MessageNewParams
func (g *Generator) buildParams(req *ai.ModelRequest) (anthropic.MessageNewParams, string, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(g.modelName),
		MaxTokens: 4096, // Default, can be overridden by config
	}

	// Extract system prompt and convert messages
	var systemPrompt string
	var messages []anthropic.MessageParam

	for _, msg := range req.Messages {
		switch msg.Role {
		case ai.RoleSystem:
			// Anthropic uses a separate system field
			for _, part := range msg.Content {
				if part.IsText() {
					systemPrompt += part.Text
				}
			}
		case ai.RoleUser:
			userBlocks := g.convertUserContent(msg.Content)
			messages = append(messages, anthropic.NewUserMessage(userBlocks...))
		case ai.RoleModel:
			assistantBlocks := g.convertAssistantContent(msg.Content)
			messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))
		case ai.RoleTool:
			toolResults := g.convertToolResults(msg.Content)
			messages = append(messages, anthropic.NewUserMessage(toolResults...))
		}
	}

	params.Messages = messages

	if len(req.Tools) > 0 {
		tools := g.convertTools(req.Tools)
		params.Tools = tools
	}

	if req.Config != nil {
		g.applyConfig(&params, req.Config)
	}

	return params, systemPrompt, nil
}

// convertUserContent converts GenKit Parts to Anthropic content blocks
func (g *Generator) convertUserContent(parts []*ai.Part) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	for _, part := range parts {
		if part.IsText() {
			blocks = append(blocks, anthropic.NewTextBlock(part.Text))
		} else if part.IsMedia() {
			// Handle image content
			blocks = append(blocks, anthropic.NewImageBlockBase64(
				part.ContentType,
				part.Text, // Base64 encoded data
			))
		}
	}

	return blocks
}

// convertAssistantContent converts assistant message parts, including tool calls
func (g *Generator) convertAssistantContent(parts []*ai.Part) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	for _, part := range parts {
		if part.IsText() && part.Text != "" {
			blocks = append(blocks, anthropic.NewTextBlock(part.Text))
		} else if part.IsToolRequest() {
			// Convert tool request to tool_use block
			inputJSON, _ := json.Marshal(part.ToolRequest.Input)
			var input map[string]interface{}
			json.Unmarshal(inputJSON, &input)

			blocks = append(blocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &anthropic.ToolUseBlockParam{
					ID:    part.ToolRequest.Ref,
					Name:  part.ToolRequest.Name,
					Input: input,
				},
			})
		}
	}

	return blocks
}

// convertToolResults converts tool response parts to Anthropic format
func (g *Generator) convertToolResults(parts []*ai.Part) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	for _, part := range parts {
		if part.IsToolResponse() {
			// Convert tool output to string
			var content string
			switch v := part.ToolResponse.Output.(type) {
			case string:
				content = v
			default:
				jsonBytes, _ := json.Marshal(v)
				content = string(jsonBytes)
			}

			blocks = append(blocks, anthropic.NewToolResultBlock(
				part.ToolResponse.Ref,
				content,
				false, // is_error
			))
		}
	}

	return blocks
}

func (g *Generator) convertTools(tools []*ai.ToolDefinition) []anthropic.ToolUnionParam {
	var anthropicTools []anthropic.ToolUnionParam

	for _, tool := range tools {
		if tool == nil || tool.Name == "" {
			continue
		}

		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
		}

		if tool.InputSchema != nil {
			schemaParam := anthropic.ToolInputSchemaParam{}

			if props, ok := tool.InputSchema["properties"]; ok {
				schemaParam.Properties = props
			}
			if req, ok := tool.InputSchema["required"].([]string); ok {
				schemaParam.Required = req
			} else if reqIface, ok := tool.InputSchema["required"].([]interface{}); ok {
				for _, r := range reqIface {
					if s, ok := r.(string); ok {
						schemaParam.Required = append(schemaParam.Required, s)
					}
				}
			}

			toolParam.InputSchema = schemaParam
		}

		anthropicTools = append(anthropicTools, anthropic.ToolUnionParam{
			OfTool: &toolParam,
		})
	}

	return anthropicTools
}

// applyConfig applies GenKit config to Anthropic params
func (g *Generator) applyConfig(params *anthropic.MessageNewParams, config interface{}) {
	if configMap, ok := config.(map[string]interface{}); ok {
		if maxTokens, ok := configMap["maxOutputTokens"].(float64); ok {
			params.MaxTokens = int64(maxTokens)
		}
		if temp, ok := configMap["temperature"].(float64); ok {
			params.Temperature = anthropic.Float(temp)
		}
		if topP, ok := configMap["topP"].(float64); ok {
			params.TopP = anthropic.Float(topP)
		}
		if topK, ok := configMap["topK"].(float64); ok {
			params.TopK = anthropic.Int(int64(topK))
		}
	}
}

// generateComplete performs a non-streaming API call
func (g *Generator) generateComplete(ctx context.Context, params anthropic.MessageNewParams) (*ai.ModelResponse, error) {
	message, err := g.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	return g.buildResponse(message), nil
}

// generateStream performs a streaming API call
func (g *Generator) generateStream(ctx context.Context, params anthropic.MessageNewParams, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	stream := g.client.Messages.NewStreaming(ctx, params)

	// Accumulate the full message
	var fullMessage anthropic.Message
	var textContent string
	var toolCalls = make(map[string]*toolCallAccumulator)
	var currentToolID string

	for stream.Next() {
		event := stream.Current()

		// Accumulate into full message
		if err := fullMessage.Accumulate(event); err != nil {
			return nil, fmt.Errorf("failed to accumulate event: %w", err)
		}

		// Process different event types
		switch evt := event.AsAny().(type) {
		case anthropic.ContentBlockStartEvent:
			// Check if it's a tool_use block starting
			if toolUse, ok := evt.ContentBlock.AsAny().(anthropic.ToolUseBlock); ok {
				currentToolID = toolUse.ID
				toolCalls[currentToolID] = &toolCallAccumulator{
					id:          toolUse.ID,
					name:        toolUse.Name,
					partialJSON: "",
				}
			}

		case anthropic.ContentBlockDeltaEvent:
			switch delta := evt.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				// Stream text chunk
				textContent += delta.Text
				chunk := &ai.ModelResponseChunk{
					Content: []*ai.Part{ai.NewTextPart(delta.Text)},
				}
				if err := cb(ctx, chunk); err != nil {
					return nil, fmt.Errorf("callback error: %w", err)
				}

			case anthropic.InputJSONDelta:
				// Accumulate tool input JSON
				if currentToolID != "" {
					if acc, exists := toolCalls[currentToolID]; exists {
						acc.partialJSON += delta.PartialJSON
					}
				}
			}

		case anthropic.ContentBlockStopEvent:
			// Finalize tool call if we were building one
			if currentToolID != "" {
				if acc, exists := toolCalls[currentToolID]; exists && acc.partialJSON != "" {
					var input map[string]interface{}
					if err := json.Unmarshal([]byte(acc.partialJSON), &input); err == nil {
						acc.input = input
					}
				}
			}
			currentToolID = ""
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	// Build final response from accumulated message
	return g.buildResponse(&fullMessage), nil
}

// toolCallAccumulator helps accumulate streamed tool calls
type toolCallAccumulator struct {
	id          string
	name        string
	partialJSON string
	input       map[string]interface{}
}

// buildResponse converts an Anthropic Message to GenKit ModelResponse
func (g *Generator) buildResponse(msg *anthropic.Message) *ai.ModelResponse {
	resp := &ai.ModelResponse{
		Message: &ai.Message{
			Role:    ai.RoleModel,
			Content: make([]*ai.Part, 0),
		},
		Usage: &ai.GenerationUsage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
			TotalTokens:  int(msg.Usage.InputTokens + msg.Usage.OutputTokens),
		},
	}

	// Convert content blocks
	for _, block := range msg.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			resp.Message.Content = append(resp.Message.Content, ai.NewTextPart(b.Text))

		case anthropic.ToolUseBlock:
			// Convert tool_use to GenKit ToolRequest
			resp.Message.Content = append(resp.Message.Content, ai.NewToolRequestPart(&ai.ToolRequest{
				Ref:   b.ID,
				Name:  b.Name,
				Input: b.Input,
			}))
		}
	}

	// Set finish reason
	switch msg.StopReason {
	case anthropic.StopReasonEndTurn:
		resp.FinishReason = ai.FinishReasonStop
	case anthropic.StopReasonMaxTokens:
		resp.FinishReason = ai.FinishReasonLength
	case anthropic.StopReasonToolUse:
		resp.FinishReason = ai.FinishReasonStop
	default:
		resp.FinishReason = ai.FinishReasonUnknown
	}

	return resp
}
