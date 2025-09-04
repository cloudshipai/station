// Minimal model generator with only the essential Station fix for OpenAI tool calling
package genkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// MinimalModelGenerator handles OpenAI API calls with only essential Station fixes
type MinimalModelGenerator struct {
	client      *openai.Client
	modelName   string
	messages    []openai.ChatCompletionMessageParamUnion
	tools       []openai.ChatCompletionToolParam
	config      *openai.ChatCompletionNewParams
	logCallback func(map[string]interface{})
	err         error
}

// NewMinimalModelGenerator creates a minimal generator with essential fixes
func NewMinimalModelGenerator(client *openai.Client, modelName string, logCallback func(map[string]interface{})) *MinimalModelGenerator {
	return &MinimalModelGenerator{
		client:      client,
		modelName:   modelName,
		logCallback: logCallback,
		config: &openai.ChatCompletionNewParams{
			Model: modelName,
		},
	}
}

// WithMessages adds messages to the request
func (g *MinimalModelGenerator) WithMessages(messages []*ai.Message) *MinimalModelGenerator {
	if g.err != nil || messages == nil {
		return g
	}

	oaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	
	for _, msg := range messages {
		switch msg.Role {
		case ai.RoleSystem:
			content := g.concatenateContent(msg.Content)
			oaiMessages = append(oaiMessages, openai.SystemMessage(content))
			
		case ai.RoleUser:
			content := g.concatenateContent(msg.Content)
			oaiMessages = append(oaiMessages, openai.UserMessage(content))
			
		case ai.RoleModel:
			content := g.concatenateContent(msg.Content)
			am := openai.ChatCompletionAssistantMessageParam{}
			
			// Add text content if present
			if content != "" {
				am.Content.OfArrayOfContentParts = append(am.Content.OfArrayOfContentParts, 
					openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
						OfText: &openai.ChatCompletionContentPartTextParam{Text: content},
					})
			}
			
			// Add tool calls if present
			toolCalls := g.convertToolCalls(msg.Content)
			if len(toolCalls) > 0 {
				am.ToolCalls = toolCalls
			}
			
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessageParamUnion{OfAssistant: &am})
			
		case ai.RoleTool:
			for _, part := range msg.Content {
				if !part.IsToolResponse() {
					continue
				}
				
				// STATION'S CRITICAL FIX: Use Ref as tool_call_id, not Name
				toolCallID := part.ToolResponse.Ref
				if toolCallID == "" {
					toolCallID = part.ToolResponse.Name // Fallback only
				}
				
				// Enforce OpenAI's 40-character limit
				if len(toolCallID) > 40 {
					toolCallID = toolCallID[:40]
				}
				
				outputJSON := g.anyToJSONString(part.ToolResponse.Output)
				
				// CRITICAL FIX: Correct parameter order (content, toolCallID)
				tm := openai.ToolMessage(outputJSON, toolCallID)
				oaiMessages = append(oaiMessages, tm)
			}
		}
	}
	
	g.messages = oaiMessages
	return g
}

// WithTools adds tools to the request
func (g *MinimalModelGenerator) WithTools(tools []*ai.ToolDefinition) *MinimalModelGenerator {
	if g.err != nil || tools == nil {
		return g
	}

	toolParams := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, tool := range tools {
		if tool == nil || tool.Name == "" {
			continue
		}

		toolParams = append(toolParams, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(tool.InputSchema),
				Strict:      openai.Bool(false),
			},
		})
	}

	g.tools = toolParams
	return g
}

// WithConfig adds configuration parameters
func (g *MinimalModelGenerator) WithConfig(config any) *MinimalModelGenerator {
	if g.err != nil || config == nil {
		return g
	}

	switch cfg := config.(type) {
	case openai.ChatCompletionNewParams:
		cfg.Model = g.modelName // Preserve model name
		g.config = &cfg
	case *openai.ChatCompletionNewParams:
		cfg.Model = g.modelName
		g.config = cfg
	case map[string]any:
		var openaiConfig openai.ChatCompletionNewParams
		if err := g.mapToStruct(cfg, &openaiConfig); err != nil {
			g.err = fmt.Errorf("failed to convert config: %w", err)
			return g
		}
		openaiConfig.Model = g.modelName
		g.config = &openaiConfig
	}

	return g
}

// Generate executes the OpenAI API request
func (g *MinimalModelGenerator) Generate(ctx context.Context, handleChunk func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	if g.err != nil {
		return nil, g.err
	}

	if len(g.messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	g.config.Messages = g.messages
	if len(g.tools) > 0 {
		g.config.Tools = g.tools
	}

	// Log API call if callback available
	if g.logCallback != nil {
		g.logCallback(map[string]interface{}{
			"level":    "debug",
			"event":    "openai_api_call",
			"message":  fmt.Sprintf("Calling OpenAI API with model %s", g.modelName),
			"model":    g.modelName,
			"messages": len(g.messages),
			"tools":    len(g.tools),
		})
	}

	// Choose streaming or complete based on callback presence
	if handleChunk != nil {
		return g.generateStream(ctx, handleChunk)
	}
	return g.generateComplete(ctx)
}

// generateComplete handles non-streaming API calls
func (g *MinimalModelGenerator) generateComplete(ctx context.Context) (*ai.ModelResponse, error) {
	startTime := time.Now()
	
	completion, err := g.client.Chat.Completions.New(ctx, *g.config)
	if err != nil {
		if g.logCallback != nil {
			g.logCallback(map[string]interface{}{
				"level":    "error",
				"event":    "openai_api_error",
				"message":  "OpenAI API call failed",
				"error":    err.Error(),
				"duration": time.Since(startTime).String(),
			})
		}
		return nil, fmt.Errorf("openai api error: %w", err)
	}

	if g.logCallback != nil {
		duration := time.Since(startTime)
		g.logCallback(map[string]interface{}{
			"level":         "info",
			"event":         "openai_api_success",
			"message":       "OpenAI API call completed",
			"duration":      duration.String(),
			"input_tokens":  completion.Usage.PromptTokens,
			"output_tokens": completion.Usage.CompletionTokens,
			"total_tokens":  completion.Usage.TotalTokens,
		})
	}

	// Convert OpenAI response to GenKit format
	return g.convertToModelResponse(completion), nil
}

// generateStream handles streaming API calls  
func (g *MinimalModelGenerator) generateStream(ctx context.Context, handleChunk func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	stream := g.client.Chat.Completions.NewStreaming(ctx, *g.config)
	defer stream.Close()

	var fullResponse ai.ModelResponse
	fullResponse.Message = &ai.Message{Role: ai.RoleModel, Content: make([]*ai.Part, 0)}
	fullResponse.Usage = &ai.GenerationUsage{}

	var currentToolCall *ai.ToolRequest
	var currentArguments strings.Builder

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) == 0 {
			continue
		}

		choice := chunk.Choices[0]

		// Handle finish reason
		switch choice.FinishReason {
		case "tool_calls", "stop":
			fullResponse.FinishReason = ai.FinishReasonStop
		case "length":
			fullResponse.FinishReason = ai.FinishReasonLength
		case "content_filter":
			fullResponse.FinishReason = ai.FinishReasonBlocked
		default:
			fullResponse.FinishReason = ai.FinishReasonUnknown
		}

		// Handle tool calls in streaming
		for _, toolCall := range choice.Delta.ToolCalls {
			if currentToolCall == nil && toolCall.Function.Name != "" {
				currentToolCall = &ai.ToolRequest{Name: toolCall.Function.Name}
			}
			if toolCall.Function.Arguments != "" {
				currentArguments.WriteString(toolCall.Function.Arguments)
			}
		}

		// Complete tool call
		if choice.FinishReason == "tool_calls" && currentToolCall != nil {
			if currentArguments.Len() > 0 {
				currentToolCall.Input = g.jsonStringToMap(currentArguments.String())
			}
			fullResponse.Message.Content = []*ai.Part{ai.NewToolRequestPart(currentToolCall)}
			return &fullResponse, nil
		}

		// Handle text content
		if content := choice.Delta.Content; content != "" {
			chunk := &ai.ModelResponseChunk{
				Content: []*ai.Part{ai.NewTextPart(content)},
			}
			if err := handleChunk(ctx, chunk); err != nil {
				return nil, fmt.Errorf("chunk callback error: %w", err)
			}
			fullResponse.Message.Content = append(fullResponse.Message.Content, chunk.Content...)
		}

		// Update usage
		if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
			fullResponse.Usage.InputTokens += int(chunk.Usage.PromptTokens)
			fullResponse.Usage.OutputTokens += int(chunk.Usage.CompletionTokens)
			fullResponse.Usage.TotalTokens += int(chunk.Usage.TotalTokens)
		}
	}

	return &fullResponse, stream.Err()
}

// Helper methods

func (g *MinimalModelGenerator) concatenateContent(parts []*ai.Part) string {
	var content strings.Builder
	for _, part := range parts {
		if part.IsText() {
			content.WriteString(part.Text)
		}
	}
	return content.String()
}

func (g *MinimalModelGenerator) convertToolCalls(parts []*ai.Part) []openai.ChatCompletionMessageToolCallParam {
	var toolCalls []openai.ChatCompletionMessageToolCallParam
	
	for _, part := range parts {
		if !part.IsToolRequest() {
			continue
		}
		
		// STATION FIX: Use Ref as ID, not Name
		toolCallID := part.ToolRequest.Ref
		if toolCallID == "" {
			toolCallID = part.ToolRequest.Name // Fallback
		}
		
		// Enforce 40-char limit
		if len(toolCallID) > 40 {
			toolCallID = toolCallID[:40]
		}
		
		param := openai.ChatCompletionMessageToolCallParam{
			ID: toolCallID,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name: part.ToolRequest.Name,
			},
		}
		
		if part.ToolRequest.Input != nil {
			param.Function.Arguments = g.anyToJSONString(part.ToolRequest.Input)
		}
		
		toolCalls = append(toolCalls, param)
	}
	
	return toolCalls
}

func (g *MinimalModelGenerator) convertToModelResponse(completion *openai.ChatCompletion) *ai.ModelResponse {
	resp := &ai.ModelResponse{
		Usage: &ai.GenerationUsage{
			InputTokens:  int(completion.Usage.PromptTokens),
			OutputTokens: int(completion.Usage.CompletionTokens),
			TotalTokens:  int(completion.Usage.TotalTokens),
		},
		Message: &ai.Message{Role: ai.RoleModel},
	}

	choice := completion.Choices[0]

	// Handle finish reason
	switch choice.FinishReason {
	case "stop", "tool_calls":
		resp.FinishReason = ai.FinishReasonStop
	case "length":
		resp.FinishReason = ai.FinishReasonLength
	case "content_filter":
		resp.FinishReason = ai.FinishReasonBlocked
	default:
		resp.FinishReason = ai.FinishReasonUnknown
	}

	// Handle tool calls with STATION FIX: preserve tool_call_id properly
	if len(choice.Message.ToolCalls) > 0 {
		var toolParts []*ai.Part
		for _, tc := range choice.Message.ToolCalls {
			toolParts = append(toolParts, ai.NewToolRequestPart(&ai.ToolRequest{
				Ref:   tc.ID, // CRITICAL: Store OpenAI ID as Ref for round-trip
				Name:  tc.Function.Name,
				Input: g.jsonStringToMap(tc.Function.Arguments),
			}))
		}
		resp.Message.Content = toolParts
	} else {
		// Text response
		resp.Message.Content = []*ai.Part{ai.NewTextPart(choice.Message.Content)}
	}

	return resp
}

// Utility methods

func (g *MinimalModelGenerator) anyToJSONString(data any) string {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

func (g *MinimalModelGenerator) jsonStringToMap(jsonStr string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return make(map[string]any)
	}
	return result
}

func (g *MinimalModelGenerator) mapToStruct(m map[string]any, v any) error {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, v)
}