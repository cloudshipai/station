package cloudshipai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
)

// Generator handles CloudShip inference API generation requests (OpenAI-compatible)
type Generator struct {
	client    *openai.Client
	modelName string
}

// NewGenerator creates a new Generator instance
func NewGenerator(client *openai.Client, modelName string) *Generator {
	return &Generator{
		client:    client,
		modelName: modelName,
	}
}

// Generate performs the API call to CloudShip inference with support for streaming and tools
func (g *Generator) Generate(ctx context.Context, req *ai.ModelRequest, cb func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	// Build the request parameters
	params, err := g.buildParams(req)
	if err != nil {
		return nil, fmt.Errorf("failed to build params: %w", err)
	}

	// Choose streaming or non-streaming based on callback
	if cb != nil {
		return g.generateStream(ctx, params, cb, req)
	}
	return g.generateComplete(ctx, params, req)
}

// buildParams converts GenKit ModelRequest to OpenAI ChatCompletionNewParams
func (g *Generator) buildParams(req *ai.ModelRequest) (openai.ChatCompletionNewParams, error) {
	params := openai.ChatCompletionNewParams{
		Model: g.modelName,
	}

	// Convert messages
	var messages []openai.ChatCompletionMessageParamUnion
	for _, msg := range req.Messages {
		converted := g.convertMessage(msg)
		messages = append(messages, converted...)
	}
	params.Messages = messages

	// Convert tools if present
	if len(req.Tools) > 0 {
		tools := g.convertTools(req.Tools)
		params.Tools = tools
		log.Printf("[CloudShipAI] Added %d tools to request", len(tools))
	}

	// Apply config
	if req.Config != nil {
		g.applyConfig(&params, req.Config)
	}

	return params, nil
}

// convertMessage converts a GenKit Message to OpenAI message format
func (g *Generator) convertMessage(msg *ai.Message) []openai.ChatCompletionMessageParamUnion {
	var messages []openai.ChatCompletionMessageParamUnion

	switch msg.Role {
	case ai.RoleSystem:
		var text string
		for _, part := range msg.Content {
			if part.IsText() {
				text += part.Text
			}
		}
		messages = append(messages, openai.SystemMessage(text))

	case ai.RoleUser:
		var textParts []string
		for _, part := range msg.Content {
			if part.IsText() {
				textParts = append(textParts, part.Text)
			}
		}
		if len(textParts) > 0 {
			text := ""
			for _, t := range textParts {
				text += t
			}
			messages = append(messages, openai.UserMessage(text))
		}

	case ai.RoleModel:
		// Check for tool calls in the response
		var textContent string
		var toolCalls []openai.ChatCompletionMessageToolCallParam

		for _, part := range msg.Content {
			if part.IsText() && part.Text != "" {
				textContent += part.Text
			} else if part.IsToolRequest() {
				inputJSON, _ := json.Marshal(part.ToolRequest.Input)
				toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
					ID:   part.ToolRequest.Ref,
					Type: "function",
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      part.ToolRequest.Name,
						Arguments: string(inputJSON),
					},
				})
			}
		}

		assistantMsg := openai.ChatCompletionAssistantMessageParam{
			Role: "assistant",
		}

		if textContent != "" {
			assistantMsg.Content.OfString = openai.String(textContent)
		}

		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}

		messages = append(messages, openai.ChatCompletionMessageParamUnion{
			OfAssistant: &assistantMsg,
		})

	case ai.RoleTool:
		// Tool role messages contain tool responses
		for _, part := range msg.Content {
			if part.IsToolResponse() {
				content := g.toolOutputToString(part.ToolResponse.Output)
				messages = append(messages, openai.ToolMessage(part.ToolResponse.Ref, content))
			}
		}
	}

	return messages
}

// toolOutputToString converts tool output to string
func (g *Generator) toolOutputToString(output any) string {
	switch v := output.(type) {
	case string:
		return v
	default:
		jsonBytes, _ := json.Marshal(v)
		return string(jsonBytes)
	}
}

// convertTools converts GenKit tool definitions to OpenAI function format
func (g *Generator) convertTools(tools []*ai.ToolDefinition) []openai.ChatCompletionToolParam {
	var openaiTools []openai.ChatCompletionToolParam

	for i, tool := range tools {
		if tool == nil || tool.Name == "" {
			log.Printf("[CloudShipAI] convertTools: skipping tool at index %d (nil or empty name)", i)
			continue
		}

		// Build the function parameters from InputSchema
		parameters := make(map[string]any)
		parameters["type"] = "object"

		if tool.InputSchema != nil {
			if props, ok := tool.InputSchema["properties"]; ok {
				parameters["properties"] = props
			}
			if req, ok := tool.InputSchema["required"]; ok {
				parameters["required"] = req
			}
		} else {
			parameters["properties"] = map[string]any{}
		}

		functionDef := openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  parameters,
		}

		openaiTools = append(openaiTools, openai.ChatCompletionToolParam{
			Type:     "function",
			Function: functionDef,
		})
	}

	return openaiTools
}

// applyConfig applies GenKit config to OpenAI params
func (g *Generator) applyConfig(params *openai.ChatCompletionNewParams, config any) {
	if configMap, ok := config.(map[string]any); ok {
		if maxTokens, ok := configMap["maxOutputTokens"].(float64); ok {
			params.MaxTokens = openai.Int(int64(maxTokens))
		}
		if temp, ok := configMap["temperature"].(float64); ok {
			params.Temperature = openai.Float(temp)
		}
		if topP, ok := configMap["topP"].(float64); ok {
			params.TopP = openai.Float(topP)
		}
	}
}

// generateComplete performs a non-streaming API call
func (g *Generator) generateComplete(ctx context.Context, params openai.ChatCompletionNewParams, originalReq *ai.ModelRequest) (*ai.ModelResponse, error) {
	log.Printf("[CloudShipAI] generateComplete: calling API for model %s with %d messages, %d tools",
		params.Model, len(params.Messages), len(params.Tools))

	completion, err := g.client.Chat.Completions.New(ctx, params)
	if err != nil {
		log.Printf("[CloudShipAI] generateComplete: API error: %v", err)
		return nil, fmt.Errorf("cloudshipai API error: %w", err)
	}

	log.Printf("[CloudShipAI] generateComplete: API returned - choices=%d", len(completion.Choices))

	return g.buildResponse(completion, originalReq), nil
}

// generateStream performs a streaming API call
func (g *Generator) generateStream(ctx context.Context, params openai.ChatCompletionNewParams, cb func(context.Context, *ai.ModelResponseChunk) error, originalReq *ai.ModelRequest) (*ai.ModelResponse, error) {
	log.Printf("[CloudShipAI] generateStream: calling streaming API for model %s", params.Model)

	stream := g.client.Chat.Completions.NewStreaming(ctx, params)

	var fullContent string
	var toolCalls []toolCallAccumulator
	var finishReason string

	for stream.Next() {
		chunk := stream.Current()

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			// Accumulate text content
			if choice.Delta.Content != "" {
				fullContent += choice.Delta.Content
				streamChunk := &ai.ModelResponseChunk{
					Content: []*ai.Part{ai.NewTextPart(choice.Delta.Content)},
				}
				if err := cb(ctx, streamChunk); err != nil {
					return nil, fmt.Errorf("callback error: %w", err)
				}
			}

			// Accumulate tool calls
			for _, tc := range choice.Delta.ToolCalls {
				idx := int(tc.Index)
				for len(toolCalls) <= idx {
					toolCalls = append(toolCalls, toolCallAccumulator{})
				}

				if tc.ID != "" {
					toolCalls[idx].id = tc.ID
				}
				if tc.Function.Name != "" {
					toolCalls[idx].name = tc.Function.Name
				}
				toolCalls[idx].arguments += tc.Function.Arguments
			}

			if choice.FinishReason != "" {
				finishReason = string(choice.FinishReason)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return g.buildStreamResponse(fullContent, toolCalls, finishReason, originalReq), nil
}

// toolCallAccumulator helps accumulate streamed tool calls
type toolCallAccumulator struct {
	id        string
	name      string
	arguments string
}

// buildResponse converts OpenAI completion to GenKit ModelResponse
func (g *Generator) buildResponse(completion *openai.ChatCompletion, originalReq *ai.ModelRequest) *ai.ModelResponse {
	resp := &ai.ModelResponse{
		Request: originalReq,
		Message: &ai.Message{
			Role:    ai.RoleModel,
			Content: make([]*ai.Part, 0),
		},
		Usage: &ai.GenerationUsage{
			InputTokens:  int(completion.Usage.PromptTokens),
			OutputTokens: int(completion.Usage.CompletionTokens),
			TotalTokens:  int(completion.Usage.TotalTokens),
		},
	}

	if len(completion.Choices) > 0 {
		choice := completion.Choices[0]

		// Add text content
		if choice.Message.Content != "" {
			resp.Message.Content = append(resp.Message.Content, ai.NewTextPart(choice.Message.Content))
		}

		// Add tool calls
		for _, tc := range choice.Message.ToolCalls {
			var input map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &input)

			resp.Message.Content = append(resp.Message.Content, ai.NewToolRequestPart(&ai.ToolRequest{
				Ref:   tc.ID,
				Name:  tc.Function.Name,
				Input: input,
			}))
			log.Printf("[CloudShipAI] Tool call: %s (id=%s)", tc.Function.Name, tc.ID)
		}

		// Set finish reason
		switch choice.FinishReason {
		case "stop":
			resp.FinishReason = ai.FinishReasonStop
		case "length":
			resp.FinishReason = ai.FinishReasonLength
		case "tool_calls":
			resp.FinishReason = ai.FinishReasonStop
		default:
			resp.FinishReason = ai.FinishReasonUnknown
		}
	}

	return resp
}

// buildStreamResponse builds response from accumulated stream data
func (g *Generator) buildStreamResponse(content string, toolCalls []toolCallAccumulator, finishReason string, originalReq *ai.ModelRequest) *ai.ModelResponse {
	resp := &ai.ModelResponse{
		Request: originalReq,
		Message: &ai.Message{
			Role:    ai.RoleModel,
			Content: make([]*ai.Part, 0),
		},
	}

	// Add text content
	if content != "" {
		resp.Message.Content = append(resp.Message.Content, ai.NewTextPart(content))
	}

	// Add accumulated tool calls
	for _, tc := range toolCalls {
		if tc.name != "" {
			var input map[string]any
			json.Unmarshal([]byte(tc.arguments), &input)

			resp.Message.Content = append(resp.Message.Content, ai.NewToolRequestPart(&ai.ToolRequest{
				Ref:   tc.id,
				Name:  tc.name,
				Input: input,
			}))
			log.Printf("[CloudShipAI] Streamed tool call: %s (id=%s)", tc.name, tc.id)
		}
	}

	// Set finish reason
	switch finishReason {
	case "stop":
		resp.FinishReason = ai.FinishReasonStop
	case "length":
		resp.FinishReason = ai.FinishReasonLength
	case "tool_calls":
		resp.FinishReason = ai.FinishReasonStop
	default:
		resp.FinishReason = ai.FinishReasonUnknown
	}

	return resp
}
