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
	"time"

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
			ParallelToolCalls: openai.Bool(false), // Prevent obsessive parallel tool calling
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
	logging.Debug("Station GenKit: About to send %d messages to LLM API", len(g.request.Messages))
	
	// Enforce turn limit before making API call
	// NOTE: Turn limit enforcement moved to genkit_executor.go for proper final response handling
	// The executor will handle turn limits and attempt a final response without tools
	const MAX_TURNS = 25
	messageCount := len(g.request.Messages)
	
	// If we're close to turn limit, log warning about potential mass tool calling
	if messageCount >= MAX_TURNS - 5 {
		logging.Debug("Station GenKit: Near turn limit (%d/%d), AI should avoid multiple tool calls", messageCount, MAX_TURNS)
		// NOTE: We can't easily modify tool choice here, but we log for awareness
	}
	
	if messageCount >= MAX_TURNS {
		// Log warning but don't fail immediately - let executor handle it
		logging.Debug("Station GenKit: Conversation approaching/exceeding %d turn limit (%d messages)", MAX_TURNS, messageCount)
		
		// Log turn limit warning for real-time progress tracking
		if g.logCallback != nil {
			g.logCallback(map[string]interface{}{
				"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
				"level":     "warning",
				"message":   fmt.Sprintf("TURN LIMIT REACHED: %d messages >= %d max turns - executor will handle final response", messageCount, MAX_TURNS),
				"details": map[string]interface{}{
					"current_messages": messageCount,
					"max_turns": MAX_TURNS,
					"enforcement_point": "openai_plugin_warning_only",
					"action": "Will be handled by executor's custom turn limit logic",
				},
			})
		}
		
		// Continue execution - let the executor handle the turn limit with final response logic
	}
	
	// Log conversation approaching limits
	turnsRemaining := MAX_TURNS - messageCount
	if g.logCallback != nil && turnsRemaining <= 3 {
		g.logCallback(map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
			"level":     "warning",
			"message":   fmt.Sprintf("TURN LIMIT WARNING: %d messages, only %d turns remaining", messageCount, turnsRemaining),
			"details": map[string]interface{}{
				"current_messages": messageCount,
				"max_turns": MAX_TURNS,
				"turns_remaining": turnsRemaining,
				"urgency_level": func() string {
					if turnsRemaining <= 1 { return "CRITICAL" }
					if turnsRemaining <= 3 { return "HIGH" }
					return "MEDIUM"
				}(),
			},
		})
	}
	
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
	
	// Log API call start for real-time progress tracking
	if g.logCallback != nil {
		// Get the last user message for context (simplified approach)
		lastUserMsg := "user input"
		messageCount := len(g.request.Messages)
		if messageCount > 1 {
			lastUserMsg = fmt.Sprintf("conversation with %d messages", messageCount)
		}
		
		// List tool names for clarity
		toolNames := make([]string, len(g.tools))
		for i, tool := range g.tools {
			toolNames[i] = tool.Function.Name
		}
		
		g.logCallback(map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
			"level":     "info",
			"message":   fmt.Sprintf("Making LLM API call to %s with %d tools", g.request.Model, len(g.tools)),
			"details": map[string]interface{}{
				"model":            g.request.Model,
				"conversation_turn": len(g.request.Messages),
				"available_tools":  toolNames,
				"last_user_input":  lastUserMsg,
				"request_type":     "chat_completion",
			},
		})
	}
	
	// Create a context with timeout to detect hanging calls
	apiCallCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	
	// Start tracking API call timing
	apiCallStart := time.Now()
	
	completion, err := g.client.Chat.Completions.New(apiCallCtx, *g.request)
	apiCallDuration := time.Since(apiCallStart)
	
	if err != nil {
		logging.Debug("Station GenKit: OpenAI API call failed after %v: %v", apiCallDuration, err)
		
		// Determine failure type for better debugging
		var failureType string
		var failureDetails map[string]interface{}
		
		if apiCallCtx.Err() == context.DeadlineExceeded {
			failureType = "API_TIMEOUT"
			failureDetails = map[string]interface{}{
				"timeout_duration": "2_minutes",
				"likely_cause": "Network issue, API overload, or request too complex",
				"conversation_turn": len(g.request.Messages),
				"tools_in_request": len(g.tools),
			}
		} else {
			failureType = "API_ERROR"
			failureDetails = map[string]interface{}{
				"error_message": err.Error(),
				"duration": apiCallDuration.String(),
				"conversation_turn": len(g.request.Messages),
			}
		}
		
		// Log API call failure for real-time progress tracking
		if g.logCallback != nil {
			g.logCallback(map[string]interface{}{
				"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
				"level":     "error",
				"message":   fmt.Sprintf("OpenAI API call FAILED (%s) after %v", failureType, apiCallDuration),
				"details":   failureDetails,
			})
		}
		
		return nil, fmt.Errorf("failed to create completion after %v: %w", apiCallDuration, err)
	}
	
	// Log successful API call timing
	if g.logCallback != nil && apiCallDuration > 30*time.Second {
		g.logCallback(map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
			"level":     "warning",
			"message":   fmt.Sprintf("OpenAI API call took %v (slower than expected)", apiCallDuration),
			"details": map[string]interface{}{
				"duration": apiCallDuration.String(),
				"conversation_turn": len(g.request.Messages),
			},
		})
	}
	logging.Debug("Station GenKit: OpenAI API call successful, processing response...")
	
	// Log API call success with response details for real-time progress tracking
	if g.logCallback != nil {
		responseMessage := ""
		toolCallDetails := make([]string, 0)
		
		if len(completion.Choices) > 0 {
			choice := completion.Choices[0]
			
			// Get response content
			if choice.Message.Content != "" {
				responseMessage = choice.Message.Content
				if len(responseMessage) > 150 {
					responseMessage = responseMessage[:150] + "..."
				}
			}
			
			// Get tool call details
			if choice.Message.ToolCalls != nil {
				for _, tc := range choice.Message.ToolCalls {
					toolCallDetails = append(toolCallDetails, fmt.Sprintf("%s(...)", tc.Function.Name))
				}
			}
		}
		
		var nextAction string
		if len(toolCallDetails) > 0 {
			nextAction = fmt.Sprintf("Will execute %d tools: %v", len(toolCallDetails), toolCallDetails)
		} else {
			nextAction = "AI provided final text response"
		}
		
		g.logCallback(map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
			"level":     "info",
			"message":   fmt.Sprintf("OpenAI responded (tokens: %d in, %d out). %s", 
				completion.Usage.PromptTokens, completion.Usage.CompletionTokens, nextAction),
			"details": map[string]interface{}{
				"input_tokens":      completion.Usage.PromptTokens,
				"output_tokens":     completion.Usage.CompletionTokens,
				"total_tokens":      completion.Usage.TotalTokens,
				"tool_calls":        toolCallDetails,
				"response_preview":  responseMessage,
				"finish_reason":     completion.Choices[0].FinishReason,
			},
		})
	}

	// For OpenAI, we need to reconstruct the conversation history from the messages
	// This enables GenKit's response.History() to work properly for step capture
	historyMessages := make([]*ai.Message, 0)
	
	// Add system message if present
	for _, msg := range g.request.Messages {
		if msg.OfSystem != nil {
			historyMessages = append(historyMessages, &ai.Message{
				Role:    ai.RoleSystem,
				Content: []*ai.Part{ai.NewTextPart("System message")},
			})
			break
		}
	}
	
	// Add user message if present
	for _, msg := range g.request.Messages {
		if msg.OfUser != nil {
			historyMessages = append(historyMessages, &ai.Message{
				Role:    ai.RoleUser,
				Content: []*ai.Part{ai.NewTextPart("User message")},
			})
			break
		}
	}
	
	// Add assistant messages with tool calls
	for _, msg := range g.request.Messages {
		if msg.OfAssistant != nil && len(msg.OfAssistant.ToolCalls) > 0 {
			assistantMsg := &ai.Message{
				Role:    ai.RoleModel,
				Content: make([]*ai.Part, 0),
			}
			// Add tool calls from assistant message
			for _, tc := range msg.OfAssistant.ToolCalls {
				assistantMsg.Content = append(assistantMsg.Content, ai.NewToolRequestPart(&ai.ToolRequest{
					Ref:   tc.ID,
					Name:  tc.Function.Name,
					Input: jsonStringToMap(tc.Function.Arguments),
				}))
			}
			historyMessages = append(historyMessages, assistantMsg)
		}
	}
	
	// Add tool responses
	for _, msg := range g.request.Messages {
		if msg.OfTool != nil {
			// Extract actual tool response content instead of hardcoded placeholder
			var toolOutput interface{}
			
			// Try to extract the actual tool content from the OpenAI tool message
			// For now, marshal the entire content structure to capture whatever is there
			content := msg.OfTool.Content
			contentBytes, err := json.Marshal(content)
			if err == nil {
				// Try to parse as JSON first
				var contentObj interface{}
				if json.Unmarshal(contentBytes, &contentObj) == nil {
					toolOutput = map[string]interface{}{
						"response": contentObj,
						"tool_call_id": msg.OfTool.ToolCallID,
					}
				} else {
					// Use raw JSON string
					toolOutput = map[string]interface{}{
						"response": string(contentBytes),
						"tool_call_id": msg.OfTool.ToolCallID,
					}
				}
			} else {
				// Final fallback with call ID
				toolOutput = map[string]interface{}{
					"response": fmt.Sprintf("Tool response for call ID: %s", msg.OfTool.ToolCallID),
					"tool_call_id": msg.OfTool.ToolCallID,
				}
			}
			
			historyMessages = append(historyMessages, &ai.Message{
				Role:    ai.RoleTool,
				Content: []*ai.Part{ai.NewToolResponsePart(&ai.ToolResponse{
					Ref:    msg.OfTool.ToolCallID,
					Name:   "tool_response",
					Output: toolOutput,
				})},
			})
		}
	}

	resp := &ai.ModelResponse{
		Request: &ai.ModelRequest{
			Messages: historyMessages,
		},
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

// Helper functions for progressive logging

// getCurrentTimestampNano returns current timestamp in nanoseconds for high precision logging
func getCurrentTimestampNano() int64 {
	return time.Now().UnixNano()
}

// getMaxTokensFromRequest extracts max_tokens from the request if present
func getMaxTokensFromRequest(req *openai.ChatCompletionNewParams) interface{} {
	// The MaxTokens field is an Opt[int64], just return the value directly
	return req.MaxTokens.Value
}