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

// Station's refactored OpenAI model generator
// Now using modular architecture with context protection, turn limiting, and tool execution
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

// StationModelGenerator handles OpenAI generation requests with Station's enhanced modular architecture
type StationModelGenerator struct {
	client     *openai.Client
	modelName  string
	request    *openai.ChatCompletionNewParams
	messages   []openai.ChatCompletionMessageParamUnion
	tools      []openai.ChatCompletionToolParam
	toolChoice openai.ChatCompletionToolChoiceOptionUnionParam
	// Store any errors that occur during building
	err error
	// Progressive logging callback for real-time updates
	logCallback func(map[string]interface{})
}

func (g *StationModelGenerator) GetRequest() *openai.ChatCompletionNewParams {
	return g.request
}

// WithLogCallback sets a logging callback for real-time progress updates
func (g *StationModelGenerator) WithLogCallback(callback func(map[string]interface{})) *StationModelGenerator {
	g.logCallback = callback
	return g
}

// NewStationModelGenerator creates a new Station ModelGenerator instance
func NewStationModelGenerator(client *openai.Client, modelName string) *StationModelGenerator {
	return &StationModelGenerator{
		client:    client,
		modelName: modelName,
		request: &openai.ChatCompletionNewParams{
			Model: (modelName),
			// Note: ParallelToolCalls will be set only when tools are provided
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
		logging.Debug("🔧 STATION-GENKIT WithTools: No tools provided")
		return g
	}

	logging.Debug("🔧 STATION-GENKIT WithTools: Processing %d tools for LLM", len(tools))
	for i, tool := range tools {
		if tool != nil {
			logging.Debug("🔧 STATION-GENKIT WithTools[%d]: %s - %s", i, tool.Name, tool.Description)
		}
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
		// Only set ParallelToolCalls when tools are provided
		// OpenAI API requires this field only when tools are specified
		g.request.ParallelToolCalls = openai.Bool(false) // Prevent obsessive parallel tool calling
		logging.Debug("🔧 STATION-GENKIT WithTools: Set %d tools for OpenAI request", len(toolParams))
	} else {
		logging.Debug("🔧 STATION-GENKIT WithTools: No valid tools to set")
	}

	return g
}

// Generate executes the generation request using native OpenAI generation 
// FIXED 2025-09-01: Removed buggy ModularGenerator that pre-executed all tools with empty inputs.
// Now uses OpenAI's native API directly, letting the LLM decide which tools to call and when.
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

	logging.Debug("🔧 STATION-GENKIT Generate: Using native OpenAI API (ModularGenerator removed)")
	logging.Debug("🔧 STATION-GENKIT Generate: %d messages, %d tools", len(g.messages), len(g.tools))

	// Set the messages and tools in the request
	g.request.Messages = g.messages
	if len(g.tools) > 0 {
		g.request.Tools = g.tools
		logging.Debug("🔧 STATION-GENKIT Generate: Set %d tools for native OpenAI", len(g.tools))
	}

	// Use OpenAI's native completion API - this allows the LLM to decide which tools to call
	logging.Debug("🔧 STATION-GENKIT Generate: Calling OpenAI ChatCompletion API")
	
	completionResp, err := g.client.Chat.Completions.New(ctx, *g.request)
	if err != nil {
		logging.Debug("Station GenKit: OpenAI API call failed: %v", err)
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	// Debug: Log the OpenAI response to see if Usage is present
	if completionResp.Usage.TotalTokens > 0 {
		logging.Debug("🔧 STATION-GENKIT OpenAI Response Usage: Input: %d, Output: %d, Total: %d", 
			completionResp.Usage.PromptTokens, completionResp.Usage.CompletionTokens, completionResp.Usage.TotalTokens)
	} else {
		logging.Debug("🔧 STATION-GENKIT OpenAI Response: No Usage data (total tokens: %d)", completionResp.Usage.TotalTokens)
	}

	// Convert OpenAI response to GenKit ModelResponse
	return g.convertToModelResponse(completionResp), nil
}

// convertToModelResponse converts OpenAI ChatCompletion response to GenKit ModelResponse format
func (g *StationModelGenerator) convertToModelResponse(resp *openai.ChatCompletion) *ai.ModelResponse {
	if resp == nil || len(resp.Choices) == 0 {
		return &ai.ModelResponse{
			Message: &ai.Message{
				Role:    ai.RoleModel,
				Content: []*ai.Part{ai.NewTextPart("No response from model")},
			},
			FinishReason: ai.FinishReasonOther,
		}
	}

	choice := resp.Choices[0]
	message := choice.Message
	
	// Convert content parts
	parts := make([]*ai.Part, 0)
	
	// Add text content if present
	if message.Content != "" {
		parts = append(parts, ai.NewTextPart(message.Content))
		logging.Debug("🔧 STATION-GENKIT Response: Added text content (%d chars)", len(message.Content))
	}
	
	// Add tool calls if present - these are LLM-requested tool calls with real parameters
	if len(message.ToolCalls) > 0 {
		logging.Debug("🔧 STATION-GENKIT Response: LLM requested %d tool calls", len(message.ToolCalls))
		for i, tc := range message.ToolCalls {
			toolRequest := &ai.ToolRequest{
				Ref:   tc.ID,
				Name:  tc.Function.Name,
				Input: jsonStringToMap(tc.Function.Arguments),
			}
			parts = append(parts, ai.NewToolRequestPart(toolRequest))
			logging.Debug("🔧 STATION-GENKIT Response: LLM Tool[%d] - %s (ID: %s) with args: %s", 
				i, tc.Function.Name, tc.ID, tc.Function.Arguments)
		}
	}

	// Convert finish reason - use Stop as default since tool calls will continue in agent loop
	finishReason := ai.FinishReasonStop
	
	// Check if we have tool calls - if so, GenKit will handle the continuation
	if len(message.ToolCalls) > 0 {
		logging.Debug("🔧 STATION-GENKIT Response: Tool calls present - GenKit will continue agent loop")
	}

	logging.Debug("🔧 STATION-GENKIT Response: Converted to %d parts, finish: %v", len(parts), finishReason)
	
	// Convert usage information from OpenAI response to GenKit format
	var usage *ai.GenerationUsage
	if resp.Usage.TotalTokens > 0 {
		usage = &ai.GenerationUsage{
			InputTokens:  int(resp.Usage.PromptTokens),
			OutputTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:  int(resp.Usage.TotalTokens),
		}
		logging.Debug("🔧 STATION-GENKIT Usage: Input: %d, Output: %d, Total: %d", 
			usage.InputTokens, usage.OutputTokens, usage.TotalTokens)
	} else {
		logging.Debug("🔧 STATION-GENKIT Usage: No usage data available (total tokens: %d)", resp.Usage.TotalTokens)
	}
	
	// Store the original request context so GenKit can track conversation history
	convertedMessages := g.convertToAIMessages()
	originalRequest := &ai.ModelRequest{
		Messages: convertedMessages,
		Tools:    g.convertToAITools(),
	}
	
	// DEBUG: Log what we're putting in the request
	logging.Debug("🔧 TOOL-EXTRACTION: Converting %d OpenAI messages to %d AI messages", len(g.messages), len(convertedMessages))
	for i, aiMsg := range convertedMessages {
		logging.Debug("🔧 TOOL-EXTRACTION: AI Message[%d] - Role: %s, Parts: %d", i, aiMsg.Role, len(aiMsg.Content))
		for j, part := range aiMsg.Content {
			if part.IsToolRequest() && part.ToolRequest != nil {
				logging.Debug("🔧 TOOL-EXTRACTION: Found ToolRequest[%d,%d]: %s with input %v", i, j, part.ToolRequest.Name, part.ToolRequest.Input)
			} else if part.IsToolResponse() && part.ToolResponse != nil {
				logging.Debug("🔧 TOOL-EXTRACTION: Found ToolResponse[%d,%d]: %s with output %v", i, j, part.ToolResponse.Name, part.ToolResponse.Output)
			} else if part.IsText() {
				textLen := len(part.Text)
				if textLen > 50 {
					textLen = 50
				}
				logging.Debug("🔧 TOOL-EXTRACTION: Found Text[%d,%d]: %s", i, j, part.Text[:textLen])
			}
		}
	}
	
	return &ai.ModelResponse{
		Message: &ai.Message{
			Role:    ai.RoleModel,
			Content: parts,
		},
		FinishReason: finishReason,
		Usage: usage,
		Request: originalRequest, // Preserve conversation context for GenKit
	}
}

// convertToAIMessages converts OpenAI messages to ai.Message format
func (g *StationModelGenerator) convertToAIMessages() []*ai.Message {
	aiMessages := make([]*ai.Message, 0, len(g.messages))
	
	for _, msg := range g.messages {
		switch {
		case msg.OfSystem != nil:
			aiMessages = append(aiMessages, &ai.Message{
				Role:    ai.RoleSystem,
				Content: []*ai.Part{ai.NewTextPart(g.extractTextContent(msg.OfSystem.Content))},
			})
		case msg.OfUser != nil:
			aiMessages = append(aiMessages, &ai.Message{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart(g.extractTextContent(msg.OfUser.Content))},
			})
		case msg.OfAssistant != nil:
			parts := make([]*ai.Part, 0)
			
			// Add text content if present
			if content := g.extractTextContent(msg.OfAssistant.Content); content != "" {
				parts = append(parts, ai.NewTextPart(content))
			}
			
			// Add tool calls if present
			for _, tc := range msg.OfAssistant.ToolCalls {
				parts = append(parts, ai.NewToolRequestPart(&ai.ToolRequest{
					Ref:   tc.ID,
					Name:  tc.Function.Name,
					Input: jsonStringToMap(tc.Function.Arguments),
				}))
			}
			
			aiMessages = append(aiMessages, &ai.Message{
				Role:    ai.RoleModel,
				Content: parts,
			})
		case msg.OfTool != nil:
			aiMessages = append(aiMessages, &ai.Message{
				Role: ai.RoleTool,
				Content: []*ai.Part{
					{
						ToolResponse: &ai.ToolResponse{
							Name:   "tool_response", // We don't have the tool name in OpenAI format
							Ref:    msg.OfTool.ToolCallID,
							Output: msg.OfTool.Content,
						},
					},
				},
			})
		}
	}
	
	return aiMessages
}

// convertToAITools converts OpenAI tools to ai.ToolDefinition format
func (g *StationModelGenerator) convertToAITools() []*ai.ToolDefinition {
	aiTools := make([]*ai.ToolDefinition, 0, len(g.tools))
	
	for _, tool := range g.tools {
		aiTools = append(aiTools, &ai.ToolDefinition{
			Name:        tool.Function.Name,
			Description: tool.Function.Description.Value,
			InputSchema: tool.Function.Parameters,
		})
	}
	
	return aiTools
}

// extractTextContent extracts text content from OpenAI content union
func (g *StationModelGenerator) extractTextContent(content interface{}) string {
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		// Handle array of content parts
		result := ""
		for _, part := range c {
			if partMap, ok := part.(map[string]interface{}); ok {
				if text, exists := partMap["text"]; exists {
					result += fmt.Sprintf("%v", text)
				}
			}
		}
		return result
	default:
		return fmt.Sprintf("%v", content)
	}
}

// concatenateContent concatenates text content into a single string (kept for compatibility)
func (g *StationModelGenerator) concatenateContent(parts []*ai.Part) string {
	content := ""
	for _, part := range parts {
		content += part.Text
	}
	return content
}

// Utility functions for GenKit compatibility

// convertStationToolCalls converts Station tool calls to OpenAI format
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