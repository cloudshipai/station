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
	"encoding/json"
	"fmt"
	"time"

	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
		// Only set ParallelToolCalls when tools are provided
		// OpenAI API requires this field only when tools are specified
		g.request.ParallelToolCalls = openai.Bool(true) // Enable parallel tool execution for complex agents
	}

	return g
}

// Generate executes the generation request
func (g *StationModelGenerator) Generate(ctx context.Context, handleChunk func(context.Context, *ai.ModelResponseChunk) error) (*ai.ModelResponse, error) {
	// Start OTEL span for OpenAI generation with enhanced agent loop attributes
	tracer := otel.Tracer("station-openai-plugin")
	ctx, span := tracer.Start(ctx, "openai.generate",
		trace.WithAttributes(
			attribute.String("model.name", g.modelName),
			attribute.Int("messages.count", len(g.messages)),
			attribute.Int("tools.count", len(g.tools)),
			attribute.Bool("streaming", handleChunk != nil),
			// Enhanced attributes for agent loop analysis
			attribute.Int("conversation.turn", len(g.messages)),
			attribute.String("request.type", "chat_completion"),
			attribute.Bool("tools.available", len(g.tools) > 0),
		),
	)
	defer span.End()

	// Check for any errors that occurred during building
	if g.err != nil {
		logging.Debug("Station GenKit: Generate failed with build error: %v", g.err)
		span.RecordError(g.err)
		span.SetStatus(codes.Error, "build error")
		return nil, g.err
	}

	if len(g.messages) == 0 {
		logging.Debug("Station GenKit: Generate failed - no messages provided")
		err := fmt.Errorf("no messages provided")
		span.RecordError(err)
		span.SetStatus(codes.Error, "no messages provided")
		return nil, err
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

	var response *ai.ModelResponse
	var err error
	
	if handleChunk != nil {
		logging.Debug("Station GenKit: Using streaming mode")
		span.SetAttributes(attribute.String("generation.mode", "streaming"))
		response, err = g.generateStream(ctx, handleChunk)
	} else {
		logging.Debug("Station GenKit: Using complete mode")
		span.SetAttributes(attribute.String("generation.mode", "complete"))
		response, err = g.generateComplete(ctx)
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "generation failed")
		return nil, err
	}

	// Add response metrics to span with enhanced agent loop analysis
	if response.Usage != nil {
		span.SetAttributes(
			attribute.Int("tokens.input", int(response.Usage.InputTokens)),
			attribute.Int("tokens.output", int(response.Usage.OutputTokens)),
			attribute.Int("tokens.total", int(response.Usage.TotalTokens)),
			// Token efficiency metrics for agent loop analysis
			attribute.Float64("tokens.output_ratio", float64(response.Usage.OutputTokens)/float64(response.Usage.TotalTokens)),
			attribute.Float64("tokens.per_turn", float64(response.Usage.TotalTokens)/float64(len(g.messages))),
		)
	}
	
	if response.Message != nil {
		span.SetAttributes(
			attribute.Int("response.parts", len(response.Message.Content)),
		)
		
		// Analyze response content for agent loop patterns
		toolCallCount := 0
		textResponseLength := 0
		for _, part := range response.Message.Content {
			if part.IsToolRequest() {
				toolCallCount++
			} else if part.IsText() {
				textResponseLength += len(part.Text)
			}
		}
		
		span.SetAttributes(
			attribute.Int("response.tool_calls", toolCallCount),
			attribute.Int("response.text_length", textResponseLength),
			attribute.Bool("response.has_tools", toolCallCount > 0),
			attribute.Bool("response.has_text", textResponseLength > 0),
			// Agent behavior patterns
			attribute.String("agent.behavior", func() string {
				if toolCallCount > 0 && textResponseLength > 0 {
					return "tool_and_text"
				} else if toolCallCount > 0 {
					return "tool_only"
				} else {
					return "text_only"
				}
			}()),
			attribute.String("agent.response_type", func() string {
				if toolCallCount >= 3 {
					return "multi_tool"
				} else if toolCallCount > 0 {
					return "single_tool"
				} else if textResponseLength > 500 {
					return "detailed_response"
				} else {
					return "brief_response"
				}
			}()),
		)
	}

	// Determine if conversation is continuing based on response content
	responseHasTools := false
	if response.Message != nil {
		for _, part := range response.Message.Content {
			if part.IsToolRequest() {
				responseHasTools = true
				break
			}
		}
	}

	span.SetAttributes(
		attribute.String("finish.reason", string(response.FinishReason)),
		// Enhanced finish reason analysis for agent loop understanding
		attribute.Bool("conversation.continuing", response.FinishReason == ai.FinishReasonStop && responseHasTools),
		attribute.Bool("conversation.completed", response.FinishReason == ai.FinishReasonStop && !responseHasTools),
		attribute.Bool("conversation.truncated", response.FinishReason == ai.FinishReasonLength),
	)
	span.SetStatus(codes.Ok, "generation completed")

	return response, nil
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
	// Create child span for API call with enhanced agent loop context
	tracer := otel.Tracer("station-openai-plugin")
	
	// Analyze conversation pattern for span attributes
	conversationAnalysis := g.analyzeConversationPattern()
	
	ctx, span := tracer.Start(ctx, "openai.api_call",
		trace.WithAttributes(
			attribute.String("api.method", "chat.completions.create"),
			attribute.String("model", g.request.Model),
			// Enhanced agent loop analysis attributes
			attribute.Int("conversation.turn", len(g.request.Messages)),
			attribute.Int("conversation.tool_messages", conversationAnalysis.ToolMessages),
			attribute.Int("conversation.user_messages", conversationAnalysis.UserMessages),
			attribute.Int("conversation.assistant_messages", conversationAnalysis.AssistantMessages),
			attribute.Int("conversation.system_messages", conversationAnalysis.SystemMessages),
			attribute.Float64("conversation.tool_ratio", conversationAnalysis.ToolRatio),
			attribute.String("conversation.pattern", conversationAnalysis.Pattern),
			attribute.Bool("conversation.has_recent_tools", conversationAnalysis.HasRecentTools),
			attribute.String("conversation.phase", conversationAnalysis.Phase),
		),
	)
	defer span.End()

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
	
	// CONTEXT MANAGEMENT: Check for large tool responses and summarize before API call
	logging.Debug("Station GenKit: About to call context management with %d messages", len(g.request.Messages))
	if err := g.manageContextSize(ctx); err != nil {
		logging.Debug("Station GenKit: Context management failed: %v", err)
		// Continue anyway - context management is best effort
	} else {
		logging.Debug("Station GenKit: Context management completed successfully")
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
	
	// Make API call with retry logic for timeout handling
	const maxRetries = 3
	var completion *openai.ChatCompletion
	var err error
	var totalDuration time.Duration
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Create a context with timeout to detect hanging calls
		apiCallCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		
		// Start tracking API call timing
		apiCallStart := time.Now()
		
		// Add span event for API call attempt
		span.AddEvent("api_call_attempt",
			trace.WithAttributes(
				attribute.Int("attempt", attempt),
				attribute.Int("max_retries", maxRetries),
			),
		)
		
		completion, err = g.client.Chat.Completions.New(apiCallCtx, *g.request)
		apiCallDuration := time.Since(apiCallStart)
		totalDuration += apiCallDuration
		
		// Record timing in span
		span.SetAttributes(
			attribute.Float64("api_call.duration_seconds", apiCallDuration.Seconds()),
			attribute.Int("api_call.attempts", attempt),
		)
		
		cancel() // Clean up context
		
		if err == nil {
			// Success! Record in span and log if this took multiple attempts
			span.AddEvent("api_call_success",
				trace.WithAttributes(
					attribute.Int("attempt", attempt),
					attribute.Float64("total_duration_seconds", totalDuration.Seconds()),
				),
			)
			
			if attempt > 1 && g.logCallback != nil {
				g.logCallback(map[string]interface{}{
					"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
					"level":     "info",
					"message":   fmt.Sprintf("OpenAI API call succeeded on attempt %d/%d after %v total", attempt, maxRetries, totalDuration),
					"details": map[string]interface{}{
						"attempt":        attempt,
						"last_duration":  apiCallDuration.String(),
						"total_duration": totalDuration.String(),
					},
				})
			}
			break
		} else {
			// Record error in span
			span.AddEvent("api_call_error",
				trace.WithAttributes(
					attribute.Int("attempt", attempt),
					attribute.String("error", err.Error()),
					attribute.Bool("is_timeout", apiCallCtx.Err() == context.DeadlineExceeded),
				),
			)
		}
		
		logging.Debug("Station GenKit: OpenAI API call attempt %d/%d failed after %v: %v", attempt, maxRetries, apiCallDuration, err)
		
		// Determine failure type for better debugging
		var failureType string
		var failureDetails map[string]interface{}
		isTimeout := apiCallCtx.Err() == context.DeadlineExceeded
		
		if isTimeout {
			failureType = "API_TIMEOUT"
			failureDetails = map[string]interface{}{
				"timeout_duration":   "2_minutes",
				"likely_cause":       "Network issue, API overload, or request too complex",
				"conversation_turn":  len(g.request.Messages),
				"tools_in_request":   len(g.tools),
				"attempt":           attempt,
				"max_retries":       maxRetries,
				"will_retry":        attempt < maxRetries,
			}
		} else {
			failureType = "API_ERROR"
			failureDetails = map[string]interface{}{
				"error_message":     err.Error(),
				"duration":          apiCallDuration.String(),
				"conversation_turn": len(g.request.Messages),
				"attempt":           attempt,
				"max_retries":       maxRetries,
				"will_retry":        attempt < maxRetries,
			}
		}
		
		// Log API call failure for real-time progress tracking
		if g.logCallback != nil {
			logLevel := "error"
			message := fmt.Sprintf("OpenAI API call FAILED (%s) on attempt %d/%d after %v", failureType, attempt, maxRetries, apiCallDuration)
			
			if attempt < maxRetries {
				logLevel = "warning"
				message += " - retrying..."
			} else {
				message += " - giving up after 3 attempts"
			}
			
			g.logCallback(map[string]interface{}{
				"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
				"level":     logLevel,
				"message":   message,
				"details":   failureDetails,
			})
		}
		
		// If this is the last attempt, return the error
		if attempt >= maxRetries {
			finalErr := fmt.Errorf("failed to create completion after %d attempts (total %v): %w", maxRetries, totalDuration, err)
			span.RecordError(finalErr)
			span.SetStatus(codes.Error, "api call failed after retries")
			span.SetAttributes(
				attribute.Float64("total_duration_seconds", totalDuration.Seconds()),
				attribute.Bool("exhausted_retries", true),
			)
			return nil, finalErr
		}
		
		// Wait before retrying (exponential backoff: 2s, 4s, 8s)
		waitTime := time.Duration(1<<attempt) * time.Second
		if g.logCallback != nil {
			g.logCallback(map[string]interface{}{
				"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
				"level":     "info",
				"message":   fmt.Sprintf("Waiting %v before retry attempt %d/%d", waitTime, attempt+1, maxRetries),
				"details": map[string]interface{}{
					"wait_time":    waitTime.String(),
					"next_attempt": attempt + 1,
					"reason":       "API timeout retry backoff",
				},
			})
		}
		
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry wait: %w", ctx.Err())
		case <-time.After(waitTime):
			// Continue to next attempt
		}
	}
	
	// Log successful API call timing
	if g.logCallback != nil && totalDuration > 30*time.Second {
		g.logCallback(map[string]interface{}{
			"timestamp": fmt.Sprintf("%d", getCurrentTimestampNano()),
			"level":     "warning",
			"message":   fmt.Sprintf("OpenAI API call took %v (slower than expected)", totalDuration),
			"details": map[string]interface{}{
				"duration": totalDuration.String(),
				"conversation_turn": len(g.request.Messages),
			},
		})
	}
	logging.Debug("Station GenKit: OpenAI API call successful, processing response...")
	
	// Record successful API call metrics in span with enhanced agent loop analysis
	if completion.Usage.TotalTokens > 0 {
		span.SetAttributes(
			attribute.Int("response.tokens.input", int(completion.Usage.PromptTokens)),
			attribute.Int("response.tokens.output", int(completion.Usage.CompletionTokens)),
			attribute.Int("response.tokens.total", int(completion.Usage.TotalTokens)),
			// Token efficiency and cost analysis
			attribute.Float64("response.tokens.efficiency", float64(completion.Usage.CompletionTokens)/float64(completion.Usage.PromptTokens)),
			attribute.Float64("response.tokens.cost_ratio", float64(completion.Usage.PromptTokens)/float64(completion.Usage.TotalTokens)),
		)
	}
	
	if len(completion.Choices) > 0 {
		choice := completion.Choices[0]
		span.SetAttributes(
			attribute.String("response.finish_reason", string(choice.FinishReason)),
		)
		
		if choice.Message.ToolCalls != nil {
			// Enhanced tool call analysis for agent loop patterns
			toolNames := make([]string, len(choice.Message.ToolCalls))
			toolTypes := make(map[string]int)
			for i, tc := range choice.Message.ToolCalls {
				toolNames[i] = tc.Function.Name
				// Categorize tool types for pattern analysis
				toolType := g.categorizeToolType(tc.Function.Name)
				toolTypes[toolType]++
			}
			
			span.SetAttributes(
				attribute.Int("response.tool_calls_count", len(choice.Message.ToolCalls)),
				attribute.StringSlice("response.tool_call_names", toolNames),
				// Tool pattern analysis
				attribute.Int("response.tools.read_operations", toolTypes["read"]),
				attribute.Int("response.tools.write_operations", toolTypes["write"]),
				attribute.Int("response.tools.search_operations", toolTypes["search"]),
				attribute.Int("response.tools.analysis_operations", toolTypes["analysis"]),
				attribute.Int("response.tools.system_operations", toolTypes["system"]),
				attribute.String("response.tools.dominant_type", g.getDominantToolType(toolTypes)),
				attribute.Bool("response.tools.mixed_operations", len(toolTypes) > 1),
				attribute.String("agent.strategy", g.inferAgentStrategy(toolTypes, len(choice.Message.ToolCalls))),
			)
			
			// Add span events for each tool call with enhanced context
			for i, tc := range choice.Message.ToolCalls {
				span.AddEvent(fmt.Sprintf("tool_call_%d", i+1),
					trace.WithAttributes(
						attribute.String("tool.name", tc.Function.Name),
						attribute.String("tool.id", tc.ID),
						attribute.String("tool.type", g.categorizeToolType(tc.Function.Name)),
						attribute.Int("tool.sequence", i+1),
						attribute.Int("tool.args_length", len(tc.Function.Arguments)),
					),
				)
			}
		} else {
			// No tool calls - analyze text response
			textLength := len(choice.Message.Content)
			span.SetAttributes(
				attribute.Int("response.text_length", textLength),
				attribute.String("response.type", "text_only"),
				attribute.String("agent.completion_type", func() string {
					if textLength > 1000 {
						return "detailed_analysis"
					} else if textLength > 300 {
						return "summary_response"
					} else {
						return "brief_response"
					}
				}()),
			)
		}
	}
	
	span.SetAttributes(
		attribute.Float64("api.total_duration_seconds", totalDuration.Seconds()),
		attribute.Bool("api.success", true),
	)
	span.SetStatus(codes.Ok, "api call completed")
	
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


// ConversationAnalysis represents the analyzed conversation pattern for agent loop insights
type ConversationAnalysis struct {
	ToolMessages      int
	UserMessages      int
	AssistantMessages int
	SystemMessages    int
	ToolRatio         float64
	Pattern           string
	HasRecentTools    bool
	Phase             string
}

// analyzeConversationPattern analyzes the conversation for agent loop patterns
func (g *StationModelGenerator) analyzeConversationPattern() ConversationAnalysis {
	analysis := ConversationAnalysis{}
	
	// Count message types
	for _, msg := range g.messages {
		switch {
		case msg.OfTool != nil:
			analysis.ToolMessages++
		case msg.OfUser != nil:
			analysis.UserMessages++
		case msg.OfAssistant != nil:
			analysis.AssistantMessages++
		case msg.OfSystem != nil:
			analysis.SystemMessages++
		}
	}
	
	totalMessages := len(g.messages)
	if totalMessages > 0 {
		analysis.ToolRatio = float64(analysis.ToolMessages) / float64(totalMessages)
	}
	
	// Determine conversation pattern
	if analysis.ToolRatio > 0.6 {
		analysis.Pattern = "tool_heavy"
	} else if analysis.ToolRatio > 0.3 {
		analysis.Pattern = "mixed"
	} else if analysis.ToolRatio > 0 {
		analysis.Pattern = "occasional_tools"
	} else {
		analysis.Pattern = "conversation_only"
	}
	
	// Check for recent tool activity (last 3 messages)
	recentToolCount := 0
	start := len(g.messages) - 3
	if start < 0 {
		start = 0
	}
	for i := start; i < len(g.messages); i++ {
		if g.messages[i].OfTool != nil {
			recentToolCount++
		}
	}
	analysis.HasRecentTools = recentToolCount > 0
	
	// Determine conversation phase
	if totalMessages < 3 {
		analysis.Phase = "initialization"
	} else if analysis.ToolRatio > 0.5 {
		analysis.Phase = "active_exploration"
	} else if analysis.HasRecentTools {
		analysis.Phase = "transitioning"
	} else {
		analysis.Phase = "conversation_focus"
	}
	
	return analysis
}

// categorizeToolType categorizes tools by their primary function for pattern analysis
func (g *StationModelGenerator) categorizeToolType(toolName string) string {
	// Read operations
	readTools := map[string]bool{
		"read_text_file": true, "get_file_info": true, "list_directory": true,
		"directory_tree": true, "read_file": true, "cat": true, "head": true,
		"tail": true, "stat": true, "ls": true,
	}
	
	// Write operations
	writeTools := map[string]bool{
		"write_text_file": true, "create_file": true, "append_file": true,
		"mkdir": true, "touch": true, "copy": true, "move": true, "rm": true,
		"chmod": true, "edit": true,
	}
	
	// Search operations  
	searchTools := map[string]bool{
		"search_files": true, "find": true, "grep": true, "search": true,
		"locate": true, "which": true,
	}
	
	// System operations
	systemTools := map[string]bool{
		"run_command": true, "execute": true, "bash": true, "sh": true,
		"ps": true, "kill": true, "top": true, "df": true, "du": true,
		"mount": true, "umount": true,
	}
	
	if readTools[toolName] {
		return "read"
	} else if writeTools[toolName] {
		return "write"
	} else if searchTools[toolName] {
		return "search"
	} else if systemTools[toolName] {
		return "system"
	} else {
		return "analysis"
	}
}

// getDominantToolType returns the most common tool type
func (g *StationModelGenerator) getDominantToolType(toolTypes map[string]int) string {
	maxCount := 0
	dominantType := "unknown"
	
	for toolType, count := range toolTypes {
		if count > maxCount {
			maxCount = count
			dominantType = toolType
		}
	}
	
	return dominantType
}

// inferAgentStrategy infers the agent's current strategy based on tool usage patterns
func (g *StationModelGenerator) inferAgentStrategy(toolTypes map[string]int, totalCalls int) string {
	// Single tool call strategies
	if totalCalls == 1 {
		for toolType := range toolTypes {
			switch toolType {
			case "read":
				return "targeted_reading"
			case "write":
				return "focused_action"
			case "search":
				return "specific_search"
			case "system":
				return "direct_execution"
			default:
				return "single_analysis"
			}
		}
	}
	
	// Multi-tool strategies
	readOps := toolTypes["read"]
	writeOps := toolTypes["write"]
	searchOps := toolTypes["search"]
	systemOps := toolTypes["system"]
	analysisOps := toolTypes["analysis"]
	
	// Information gathering pattern
	if readOps > 0 && searchOps > 0 && writeOps == 0 {
		return "information_gathering"
	}
	
	// Implementation pattern
	if readOps > 0 && writeOps > 0 {
		return "implementation_workflow"
	}
	
	// Exploration pattern
	if searchOps > 1 || (readOps > 2 && searchOps > 0) {
		return "systematic_exploration"
	}
	
	// Analysis pattern  
	if analysisOps > readOps && writeOps == 0 {
		return "deep_analysis"
	}
	
	// System administration pattern
	if systemOps > 0 && (readOps > 0 || writeOps > 0) {
		return "system_administration"
	}
	
	// Mixed strategy
	if len(toolTypes) >= 3 {
		return "multi_modal_approach"
	}
	
	// Default based on dominant operation
	if readOps > writeOps {
		return "read_heavy_workflow"
	} else if writeOps > 0 {
		return "action_oriented_workflow"
	} else {
		return "analysis_workflow"
	}
}