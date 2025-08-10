// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Station's fixed version of the genkit OpenAI model generator
// with proper tool_call_id handling for multi-turn conversations.
package genkit

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// mapToStruct unmarshals a map[string]any to the expected config type.
func mapToStruct(m map[string]any, v any) error {
	jsonData, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(jsonData, v)
}

// StationModelGenerator handles OpenAI generation requests with Station's fixes
type StationModelGenerator struct {
	client     *openai.Client
	modelName  string
	request    *openai.ChatCompletionNewParams
	messages   []openai.ChatCompletionMessageParamUnion
	tools      []openai.ChatCompletionToolParam
	toolChoice openai.ChatCompletionToolChoiceOptionUnionParam
	// Store any errors that occur during building
	err error
}

func (g *StationModelGenerator) GetRequest() *openai.ChatCompletionNewParams {
	return g.request
}

// NewStationModelGenerator creates a new Station ModelGenerator instance
func NewStationModelGenerator(client *openai.Client, modelName string) *StationModelGenerator {
	return &StationModelGenerator{
		client:    client,
		modelName: modelName,
		request: &openai.ChatCompletionNewParams{
			Model: (modelName),
		},
	}
}

// WithMessages adds messages to the request
func (g *StationModelGenerator) WithMessages(messages []*ai.Message) *StationModelGenerator {
	// Return early if we already have an error
	if g.err != nil {
		return g
	}

	if messages == nil {
		return g
	}

	oaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		content := g.concatenateContent(msg.Content)
		switch msg.Role {
		case ai.RoleSystem:
			oaiMessages = append(oaiMessages, openai.SystemMessage(content))
		case ai.RoleModel:
			oaiMessages = append(oaiMessages, openai.AssistantMessage(content))

			am := openai.ChatCompletionAssistantMessageParam{}
			if msg.Content[0].Text != "" {
				am.Content.OfArrayOfContentParts = append(am.Content.OfArrayOfContentParts, openai.ChatCompletionAssistantMessageParamContentArrayOfContentPartUnion{
					OfText: &openai.ChatCompletionContentPartTextParam{
						Text: msg.Content[0].Text,
					},
				})
			}
			toolCalls := convertStationToolCalls(msg.Content)
			if len(toolCalls) > 0 {
				am.ToolCalls = (toolCalls)
			}
			oaiMessages = append(oaiMessages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &am,
			})
		case ai.RoleTool:
			for _, p := range msg.Content {
				if !p.IsToolResponse() {
					continue
				}
				// STATION FIX: Use the captured tool call ID (Ref) if available, otherwise fall back to tool name
				// This is the critical fix that prevents tool execution results from being used as tool_call_id
				toolCallID := p.ToolResponse.Ref
				
				logging.Debug("Station GenKit: Processing tool response - Name: %s, Ref: %s (len=%d)", 
					p.ToolResponse.Name, p.ToolResponse.Ref, len(p.ToolResponse.Ref))
				
				if toolCallID == "" {
					// Fallback: if no Ref, use tool name (this maintains compatibility but isn't ideal)
					toolCallID = p.ToolResponse.Name
					logging.Debug("Station GenKit: No Ref found for tool response, falling back to tool name: %s", toolCallID)
				} else {
					logging.Debug("Station GenKit: Using proper tool call Ref as ID: %s (len=%d)", toolCallID, len(toolCallID))
				}

				// Check if tool response ID exceeds OpenAI's 40-character limit and truncate
				if len(toolCallID) > 40 {
					logging.Debug("Station GenKit: CRITICAL - Tool response ID length %d exceeds OpenAI limit: %s", 
						len(toolCallID), toolCallID)
					originalID := toolCallID
					toolCallID = toolCallID[:40]
					logging.Debug("Station GenKit: Truncated tool response ID from %s to %s", originalID, toolCallID)
				}

				outputJSON := anyToJSONString(p.ToolResponse.Output)
				logging.Debug("Station GenKit: Creating tool message with ID: %s (len=%d) for output: %.100s...", 
					toolCallID, len(toolCallID), outputJSON)
				
				logging.Debug("Station GenKit: ToolMessage params - ID: '%s', Output: '%.100s'", 
					toolCallID, outputJSON)

				// STATION FIX: Correct parameter order for OpenAI ToolMessage 
				// The function signature is ToolMessage(content, toolCallID) not (toolCallID, content)
				tm := openai.ToolMessage(
					outputJSON,
					toolCallID,
				)
				
				if tm.OfTool != nil {
					logging.Debug("Station GenKit: Constructed ToolMessage - ToolCallID: '%s' (len=%d)", 
						tm.OfTool.ToolCallID, len(tm.OfTool.ToolCallID))
				}
				oaiMessages = append(oaiMessages, tm)
			}
		case ai.RoleUser:
			oaiMessages = append(oaiMessages, openai.UserMessage(content))

			parts := []openai.ChatCompletionContentPartUnionParam{}
			for _, p := range msg.Content {
				if p.IsMedia() {
					part := openai.ImageContentPart(
						openai.ChatCompletionContentPartImageImageURLParam{
							URL: p.Text,
						})
					parts = append(parts, part)
					continue
				}
			}
			if len(parts) > 0 {
				oaiMessages = append(oaiMessages, openai.ChatCompletionMessageParamUnion{
					OfUser: &openai.ChatCompletionUserMessageParam{
						Content: openai.ChatCompletionUserMessageParamContentUnion{OfArrayOfContentParts: parts},
					},
				})
			}
		default:
			// ignore parts from not supported roles
			continue
		}

	}
	g.messages = oaiMessages
	return g
}

// WithConfig adds configuration parameters from the model request
// see https://platform.openai.com/docs/api-reference/responses/create
// for more details on openai's request fields
func (g *StationModelGenerator) WithConfig(config any) *StationModelGenerator {
	// Return early if we already have an error
	if g.err != nil {
		return g
	}

	if config == nil {
		return g
	}

	var openaiConfig openai.ChatCompletionNewParams
	switch cfg := config.(type) {
	case openai.ChatCompletionNewParams:
		openaiConfig = cfg
	case *openai.ChatCompletionNewParams:
		openaiConfig = *cfg
	case map[string]any:
		if err := mapToStruct(cfg, &openaiConfig); err != nil {
			g.err = fmt.Errorf("failed to convert config to OpenAIConfig: %w", err)
			return g
		}
	default:
		g.err = fmt.Errorf("unexpected config type: %T", config)
		return g
	}

	// keep the original model in the updated config structure
	openaiConfig.Model = g.request.Model
	g.request = &openaiConfig
	return g
}

// WithTools adds tools to the request
func (g *StationModelGenerator) WithTools(tools []*ai.ToolDefinition) *StationModelGenerator {
	if g.err != nil {
		return g
	}

	if tools == nil {
		return g
	}

	toolParams := make([]openai.ChatCompletionToolParam, 0, len(tools))
	for _, tool := range tools {
		if tool == nil || tool.Name == "" {
			continue
		}

		toolParams = append(toolParams, openai.ChatCompletionToolParam{
			Function: (shared.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(tool.InputSchema),
				Strict:      openai.Bool(false), // TODO: implement strict mode
			}),
		})
	}

	// Set the tools in the request
	// If no tools are provided, set it to nil
	// This is important to avoid sending an empty array in the request
	// which is not supported by some vendor APIs
	if len(toolParams) > 0 {
		g.tools = toolParams
	}

	return g
}

// Generate executes the generation request
func (g *StationModelGenerator) Generate(ctx context.Context, handleChunk func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	// Check for any errors that occurred during building
	if g.err != nil {
		logging.Debug("Station GenKit: Generate failed with build error: %v", g.err)
		return nil, g.err
	}

	if len(g.messages) == 0 {
		logging.Debug("Station GenKit: Generate failed - no messages provided")
		return nil, fmt.Errorf("no messages provided")
	}
	
	logging.Debug("Station GenKit: Starting Generate with %d messages, %d tools", len(g.messages), len(g.tools))
	g.request.Messages = (g.messages)

	if len(g.tools) > 0 {
		logging.Debug("Station GenKit: Adding %d tools to request", len(g.tools))
		g.request.Tools = (g.tools)
		for i, tool := range g.tools {
			logging.Debug("Station GenKit: Tool %d: %s (type: %s)", i, tool.Function.Name, tool.Type)
		}
	}

	if handleChunk != nil {
		logging.Debug("Station GenKit: Using streaming mode")
		return g.generateStream(ctx, handleChunk)
	}
	logging.Debug("Station GenKit: Using complete mode")
	return g.generateComplete(ctx)
}

// concatenateContent concatenates text content into a single string
func (g *StationModelGenerator) concatenateContent(parts []*ai.Part) string {
	content := ""
	for _, part := range parts {
		content += part.Text
	}
	return content
}

// generateStream generates a streaming model response
func (g *StationModelGenerator) generateStream(ctx context.Context, handleChunk func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	stream := g.client.Chat.Completions.NewStreaming(ctx, *g.request)
	defer stream.Close()

	var fullResponse ai.ModelResponse
	fullResponse.Message = &ai.Message{
		Role:    ai.RoleModel,
		Content: make([]*ai.Part, 0),
	}

	// Initialize request and usage
	fullResponse.Request = &ai.ModelRequest{}
	fullResponse.Usage = &ai.GenerationUsage{
		InputTokens:  0,
		OutputTokens: 0,
		TotalTokens:  0,
	}

	var currentToolCall *ai.ToolRequest
	var currentArguments string

	for stream.Next() {
		chunk := stream.Current()
		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			switch choice.FinishReason {
			case "tool_calls", "stop":
				fullResponse.FinishReason = ai.FinishReasonStop
			case "length":
				fullResponse.FinishReason = ai.FinishReasonLength
			case "content_filter":
				fullResponse.FinishReason = ai.FinishReasonBlocked
			case "function_call":
				fullResponse.FinishReason = ai.FinishReasonOther
			default:
				fullResponse.FinishReason = ai.FinishReasonUnknown
			}

			// handle tool calls
			for _, toolCall := range choice.Delta.ToolCalls {
				// first tool call (= current tool call is nil) contains the tool call name
				if currentToolCall == nil {
					currentToolCall = &ai.ToolRequest{
						Name: toolCall.Function.Name,
					}
				}

				if toolCall.Function.Arguments != "" {
					currentArguments += toolCall.Function.Arguments
				}
			}

			// when tool call is complete
			if choice.FinishReason == "tool_calls" && currentToolCall != nil {
				// parse accumulated arguments string
				if currentArguments != "" {
					currentToolCall.Input = jsonStringToMap(currentArguments)
				}

				fullResponse.Message.Content = []*ai.Part{ai.NewToolRequestPart(currentToolCall)}
				return &fullResponse, nil
			}

			content := chunk.Choices[0].Delta.Content
			modelChunk := &ai.ModelResponseChunk{
				Content: []*ai.Part{ai.NewTextPart(content)},
			}

			if err := handleChunk(ctx, modelChunk); err != nil {
				return nil, fmt.Errorf("callback error: %w", err)
			}

			fullResponse.Message.Content = append(fullResponse.Message.Content, modelChunk.Content...)

			// Update Usage
			fullResponse.Usage.InputTokens += int(chunk.Usage.PromptTokens)
			fullResponse.Usage.OutputTokens += int(chunk.Usage.CompletionTokens)
			fullResponse.Usage.TotalTokens += int(chunk.Usage.TotalTokens)
		}
	}

	if err := stream.Err(); err != nil {
		return nil, fmt.Errorf("stream error: %w", err)
	}

	return &fullResponse, nil
}

// generateComplete generates a complete model response
func (g *StationModelGenerator) generateComplete(ctx context.Context) (*ai.ModelResponse, error) {
	logging.Debug("Station GenKit: About to send %d messages to OpenAI API", len(g.request.Messages))
	for i, msg := range g.request.Messages {
		switch {
		case msg.OfTool != nil:
			logging.Debug("Station GenKit: Message %d (Tool): tool_call_id='%s' (len=%d)", 
				i, msg.OfTool.ToolCallID, len(msg.OfTool.ToolCallID))
		case msg.OfAssistant != nil:
			if msg.OfAssistant.ToolCalls != nil {
				for j, tc := range msg.OfAssistant.ToolCalls {
					logging.Debug("Station GenKit: Message %d (Assistant) ToolCall %d: ID='%s' (len=%d)", 
						i, j, tc.ID, len(tc.ID))
				}
			}
		case msg.OfUser != nil:
			logging.Debug("Station GenKit: Message %d (User): content type", i)
		case msg.OfSystem != nil:
			logging.Debug("Station GenKit: Message %d (System): content type", i)
		}
	}
	
	logging.Debug("Station GenKit: Calling OpenAI API with model=%s", g.request.Model)
	completion, err := g.client.Chat.Completions.New(ctx, *g.request)
	if err != nil {
		logging.Debug("Station GenKit: OpenAI API call failed: %v", err)
		return nil, fmt.Errorf("failed to create completion: %w", err)
	}
	logging.Debug("Station GenKit: OpenAI API call successful, processing response...")

	resp := &ai.ModelResponse{
		Request: &ai.ModelRequest{},
		Usage: &ai.GenerationUsage{
			InputTokens:  int(completion.Usage.PromptTokens),
			OutputTokens: int(completion.Usage.CompletionTokens),
			TotalTokens:  int(completion.Usage.TotalTokens),
		},
		Message: &ai.Message{
			Role: ai.RoleModel,
		},
	}

	choice := completion.Choices[0]

	switch choice.FinishReason {
	case "stop", "tool_calls":
		resp.FinishReason = ai.FinishReasonStop
	case "length":
		resp.FinishReason = ai.FinishReasonLength
	case "content_filter":
		resp.FinishReason = ai.FinishReasonBlocked
	case "function_call":
		resp.FinishReason = ai.FinishReasonOther
	default:
		resp.FinishReason = ai.FinishReasonUnknown
	}

	// handle tool calls - STATION FIX: Properly preserve tool_call_id
	var toolRequestParts []*ai.Part
	for _, toolCall := range choice.Message.ToolCalls {
		logging.Debug("Station GenKit: Processing tool call from OpenAI - ID: %s (len=%d), Function: %s", 
			toolCall.ID, len(toolCall.ID), toolCall.Function.Name)
		
		toolRequestParts = append(toolRequestParts, ai.NewToolRequestPart(&ai.ToolRequest{
			Ref:   toolCall.ID, // CRITICAL FIX: Store the OpenAI call ID as Ref
			Name:  toolCall.Function.Name,
			Input: jsonStringToMap(toolCall.Function.Arguments),
		}))
		logging.Debug("Station GenKit: Captured tool call ID: %s for tool: %s", toolCall.ID, toolCall.Function.Name)
	}
	if len(toolRequestParts) > 0 {
		resp.Message.Content = toolRequestParts
		return resp, nil
	}

	resp.Message.Content = []*ai.Part{
		ai.NewTextPart(completion.Choices[0].Message.Content),
	}
	return resp, nil
}

func convertStationToolCalls(content []*ai.Part) []openai.ChatCompletionMessageToolCallParam {
	var toolCalls []openai.ChatCompletionMessageToolCallParam
	for _, p := range content {
		if !p.IsToolRequest() {
			continue
		}
		toolCall := convertStationToolCall(p)
		toolCalls = append(toolCalls, toolCall)
	}
	return toolCalls
}

// convertStationToolCall - STATION'S CRITICAL FIX
// This function contains the main fix for the tool_call_id bug
func convertStationToolCall(part *ai.Part) openai.ChatCompletionMessageToolCallParam {
	// STATION FIX: Use Ref (proper tool call ID) instead of Name (tool execution result)
	toolCallID := part.ToolRequest.Ref
	logging.Debug("Station GenKit: Tool call conversion - ID: %s (len=%d), Name: %s", 
		toolCallID, len(toolCallID), part.ToolRequest.Name)
		
	if toolCallID == "" {
		// Fallback: Generate a proper call ID if none exists
		// This should rarely happen with proper implementation
		toolCallID = generateToolCallID()
		logging.Debug("Station GenKit: Generated fallback tool call ID: %s (len=%d) for tool: %s", 
			toolCallID, len(toolCallID), part.ToolRequest.Name)
	}

	// Check if ID exceeds OpenAI's 40-character limit and truncate
	if len(toolCallID) > 40 {
		logging.Debug("Station GenKit: WARNING - Tool call ID length %d exceeds OpenAI limit of 40 chars: %s", 
			len(toolCallID), toolCallID)
		originalID := toolCallID
		toolCallID = toolCallID[:40]
		logging.Debug("Station GenKit: Truncated tool call ID from %s to %s", originalID, toolCallID)
	}

	logging.Debug("Station GenKit: Using tool call ID: %s (len=%d)", toolCallID, len(toolCallID))

	param := openai.ChatCompletionMessageToolCallParam{
		ID: (toolCallID), // CRITICAL FIX: Use proper call reference, not tool name
		Function: (openai.ChatCompletionMessageToolCallFunctionParam{
			Name: (part.ToolRequest.Name), // Tool name stays as function name
		}),
	}

	if part.ToolRequest.Input != nil {
		param.Function.Arguments = (anyToJSONString(part.ToolRequest.Input))
	}

	logging.Debug("Station GenKit: Tool call conversion - ID: %s, Name: %s", toolCallID, part.ToolRequest.Name)
	return param
}

// generateToolCallID creates a short tool call ID similar to OpenAI's format
func generateToolCallID() string {
	// Generate 8 random bytes for a 16-character hex string
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("call_%x", bytes)[:12] // Format: call_abc123ef (12 chars total)
}

func jsonStringToMap(jsonString string) map[string]any {
	var result map[string]any
	if err := json.Unmarshal([]byte(jsonString), &result); err != nil {
		panic(fmt.Errorf("unmarshal failed to parse json string %s: %w", jsonString, err))
	}
	return result
}

func anyToJSONString(data any) string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		panic(fmt.Errorf("failed to marshal any to JSON string: data, %#v %w", data, err))
	}
	return string(jsonBytes)
}