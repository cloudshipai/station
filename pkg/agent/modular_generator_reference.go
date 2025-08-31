package agent

import (
	"context"
	"fmt"
	"time"

	stationctx "station/pkg/context"
	"station/pkg/tools"
	"station/pkg/turns"

	"github.com/firebase/genkit/go/ai"
	"github.com/openai/openai-go"
	"go.opentelemetry.io/otel/trace"
)

// ModularGenerator is a reference implementation showing how to integrate 
// Station's context/turn/tool protection with GenKit in a modular design.
// NOTE: This is currently unused - Station uses StationGenerate() instead.
// Kept as reference for future modular component development.
type ModularGenerator struct {
	client          *openai.Client
	modelName       string
	contextManager  *stationctx.Manager
	turnLimiter     *turns.Limiter
	toolExecutor    *tools.Executor
	finalizer       *turns.ConversationFinalizer
	logCallback     func(map[string]interface{})
	span            trace.Span
}

// NewModularGenerator creates a GenKit-integrated generator with Station's protections
func NewModularGenerator(
	client *openai.Client,
	modelName string,
	logCallback func(map[string]interface{}),
) *ModularGenerator {
	// Initialize context manager with model-specific limits
	contextManager := stationctx.NewManager(modelName, logCallback)
	
	// Initialize turn limiter with sensible defaults
	turnConfig := turns.LimiterConfig{
		MaxTurns:          25, // Station's default
		WarningThreshold:  0.8,
		CriticalThreshold: 0.9,
		EnableAdaptive:    true,
		ContextAware:      true,
	}
	turnLimiter := turns.NewLimiter(turnConfig, contextManager, logCallback)
	
	// Initialize tool executor with context protection
	toolConfig := tools.ToolExecutorConfig{
		EnableContextProtection: true,
		MaxOutputTokens:        2000,
		TruncationStrategy:     tools.TruncationStrategyIntelligent,
		ContextBuffer:          1000,
		MaxConcurrentTools:     3,
		ToolTimeout:           30 * time.Second,
	}
	toolExecutor := tools.NewExecutor(toolConfig, contextManager, logCallback)
	
	// Initialize conversation finalizer
	finalizer := turns.NewConversationFinalizer(&openAIWrapper{client}, logCallback)
	
	return &ModularGenerator{
		client:         client,
		modelName:      modelName,
		contextManager: contextManager,
		turnLimiter:    turnLimiter,
		toolExecutor:   toolExecutor,
		finalizer:      finalizer,
		logCallback:    logCallback,
	}
}

// Generate implements the GenKit generation pattern with Station's protections
func (g *ModularGenerator) Generate(
	ctx context.Context,
	messages []*ai.Message,
	aiTools []*ai.ToolDefinition,
	handleChunk func(context.Context, *ai.ModelResponseChunk) error,
) (*ai.ModelResponse, error) {
	
	// Start telemetry span
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("station-enhanced").Start(ctx, "enhanced.generate")
	defer span.End()
	g.span = span
	
	startTime := time.Now()
	conversationID := g.generateConversationID()
	
	if g.logCallback != nil {
		g.logCallback(map[string]interface{}{
			"timestamp":       time.Now().Format(time.RFC3339),
			"level":          "info",
			"message":        "Enhanced generation starting",
			"conversation_id": conversationID,
			"model":          g.modelName,
			"message_count":  len(messages),
			"tool_count":     len(aiTools),
		})
	}
	
	// Phase 1: Pre-generation Checks
	if err := g.preGenerationChecks(messages); err != nil {
		return nil, fmt.Errorf("pre-generation check failed: %w", err)
	}
	
	// Phase 2: Context Management
	if err := g.updateContextUsage(messages); err != nil {
		return nil, fmt.Errorf("context management failed: %w", err)
	}
	
	// Phase 3: Turn Management
	if shouldComplete, reason, description := g.turnLimiter.ShouldForceCompletion(messages); shouldComplete {
		return g.generateFinalResponse(ctx, messages, reason, description)
	}
	
	// Phase 4: Tool Execution with Protection
	var toolResults []*tools.ExecutionResult
	if len(aiTools) > 0 {
		var err error
		toolResults, err = g.executeToolsWithProtection(ctx, messages, aiTools, conversationID)
		if err != nil {
			// Don't fail - log error and continue without tools
			if g.logCallback != nil {
				g.logCallback(map[string]interface{}{
					"level":   "warning",
					"message": "Tool execution failed, continuing without tools",
					"error":   err.Error(),
				})
			}
		}
	}
	
	// Phase 5: Generate Response
	response, err := g.generateResponse(ctx, messages, toolResults, handleChunk)
	if err != nil {
		return nil, fmt.Errorf("response generation failed: %w", err)
	}
	
	// Phase 6: Post-generation Updates
	g.postGenerationUpdates(response, time.Since(startTime))
	
	return response, nil
}

// preGenerationChecks validates the request before processing
func (g *ModularGenerator) preGenerationChecks(messages []*ai.Message) error {
	if len(messages) == 0 {
		return fmt.Errorf("no messages provided")
	}
	
	// Check if we can continue the conversation
	canContinue, reason := g.turnLimiter.CanContinue()
	if !canContinue {
		return fmt.Errorf("conversation cannot continue: %s", reason)
	}
	
	// Check context limits
	if g.contextManager.IsApproachingLimit() {
		utilization := g.contextManager.GetUtilizationPercent()
		if utilization > 0.95 { // 95% is critical
			return fmt.Errorf("context critically full: %.1f%% utilization", utilization*100)
		}
	}
	
	return nil
}

// updateContextUsage tracks token usage for conversation
func (g *ModularGenerator) updateContextUsage(messages []*ai.Message) error {
	// Estimate tokens for current messages
	estimatedTokens := g.contextManager.EstimateTokensInMessages(messages)
	
	// Check if we have space
	canExecute, reason := g.contextManager.CanExecuteAction(estimatedTokens)
	if !canExecute {
		return fmt.Errorf("insufficient context space: %s", reason)
	}
	
	return nil
}

// executeToolsWithProtection handles tool execution with full protection
func (g *ModularGenerator) executeToolsWithProtection(
	ctx context.Context,
	messages []*ai.Message,
	aiTools []*ai.ToolDefinition,
	conversationID string,
) ([]*tools.ExecutionResult, error) {
	
	if len(aiTools) == 0 {
		return nil, nil
	}
	
	// Convert GenKit tools to our tool requests (simplified for example)
	toolCalls := make([]*ai.ToolRequest, len(aiTools))
	for i, tool := range aiTools {
		toolCalls[i] = &ai.ToolRequest{
			Name:  tool.Name,
			Input: map[string]interface{}{}, // Would need actual input from conversation
		}
	}
	
	// Create execution context
	execCtx := &tools.ExecutionContext{
		ConversationID:   conversationID,
		TurnNumber:       g.turnLimiter.GetCurrentTurns(),
		RemainingTokens:  g.contextManager.GetTokensRemaining(),
		SafeActionLimit:  g.contextManager.GetSafeActionLimit(),
		ContextThreshold: 0.9,
	}
	
	// Execute tools with batch protection
	results, err := g.toolExecutor.ExecuteToolBatch(ctx, toolCalls, execCtx)
	if err != nil {
		return nil, fmt.Errorf("tool execution batch failed: %w", err)
	}
	
	// Update context with tool results
	for _, result := range results {
		if result.Success && result.TokensUsed > 0 {
			g.contextManager.TrackTokenUsage(stationctx.TokenUsage{
				OutputTokens: result.TokensUsed,
				TotalTokens:  result.TokensUsed,
			})
		}
	}
	
	return results, nil
}

// generateResponse creates the actual OpenAI API response
func (g *ModularGenerator) generateResponse(
	ctx context.Context,
	messages []*ai.Message,
	toolResults []*tools.ExecutionResult,
	handleChunk func(context.Context, *ai.ModelResponseChunk) error,
) (*ai.ModelResponse, error) {
	
	// Convert to OpenAI format (simplified - real implementation would be more complex)
	oaiMessages := g.convertMessagesToOpenAI(messages, toolResults)
	
	// Make OpenAI API call
	response, err := g.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    g.modelName,
		Messages: oaiMessages,
		// Additional parameters...
	})
	
	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}
	
	// Convert response back to GenKit format
	return g.convertFromOpenAI(response), nil
}

// generateFinalResponse creates a forced completion response
func (g *ModularGenerator) generateFinalResponse(
	ctx context.Context,
	messages []*ai.Message,
	reason turns.CompletionReason,
	description string,
) (*ai.ModelResponse, error) {
	
	if g.logCallback != nil {
		g.logCallback(map[string]interface{}{
			"level":       "warning",
			"message":     "Forcing conversation completion",
			"reason":      string(reason),
			"description": description,
		})
	}
	
	// Use the conversation finalizer to generate appropriate final response
	finalizationRequest := turns.FinalizationRequest{
		Conversation:  messages,
		Reason:        reason,
		PreserveLastN: 5, // Keep last 5 messages
		CreateSummary: true,
		ForceFinal:    true,
		ModelName:     g.modelName,
	}
	
	finalizationResponse, err := g.finalizer.GenerateFinalResponse(ctx, finalizationRequest)
	if err != nil {
		return nil, fmt.Errorf("final response generation failed: %w", err)
	}
	
	return finalizationResponse.FinalResponse, nil
}

// postGenerationUpdates handles cleanup and metrics after generation
func (g *ModularGenerator) postGenerationUpdates(response *ai.ModelResponse, duration time.Duration) {
	// Increment turn counter
	g.turnLimiter.IncrementTurn()
	
	// Track token usage if available
	if response.Usage != nil {
		g.contextManager.TrackTokenUsage(stationctx.TokenUsage{
			InputTokens:  response.Usage.InputTokens,
			OutputTokens: response.Usage.OutputTokens,
			TotalTokens:  response.Usage.TotalTokens,
		})
	}
	
	// Log completion
	if g.logCallback != nil {
		metrics := g.turnLimiter.GetMetrics()
		contextStatus := g.contextManager.GetContextStatus()
		toolMetrics := g.toolExecutor.GetToolMetrics()
		
		g.logCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Enhanced generation completed",
			"duration":  duration.Milliseconds(),
			"turns": map[string]interface{}{
				"current":     metrics.CurrentTurns,
				"max":         metrics.MaxTurns,
				"remaining":   metrics.TurnsRemaining,
				"utilization": metrics.UtilizationPercent * 100,
			},
			"context": map[string]interface{}{
				"utilization":      contextStatus.UtilizationPercent * 100,
				"tokens_remaining": contextStatus.TokensRemaining,
				"can_continue":     contextStatus.CanExecuteAction,
			},
			"tools": map[string]interface{}{
				"total_executions": toolMetrics.TotalExecutions,
				"successful":       toolMetrics.SuccessfulExecutions,
				"truncated":        toolMetrics.TruncatedExecutions,
				"tokens_saved":     toolMetrics.TokensSaved,
			},
		})
	}
}

// Helper methods
func (g *ModularGenerator) generateConversationID() string {
	return fmt.Sprintf("conv_%d", time.Now().UnixNano())
}

func (g *ModularGenerator) convertMessagesToOpenAI(messages []*ai.Message, toolResults []*tools.ExecutionResult) []openai.ChatCompletionMessageParamUnion {
	// Simplified conversion - real implementation would handle all message types
	oaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	
	for _, msg := range messages {
		content := g.concatenateContent(msg.Content)
		switch msg.Role {
		case ai.RoleSystem:
			oaiMessages = append(oaiMessages, openai.SystemMessage(content))
		case ai.RoleUser:
			oaiMessages = append(oaiMessages, openai.UserMessage(content))
		case ai.RoleModel:
			oaiMessages = append(oaiMessages, openai.AssistantMessage(content))
		}
	}
	
	// Add tool results if any
	for _, result := range toolResults {
		if result.Success {
			oaiMessages = append(oaiMessages, openai.UserMessage(
				fmt.Sprintf("Tool %s result: %s", result.ToolName, result.Output),
			))
		}
	}
	
	return oaiMessages
}

func (g *ModularGenerator) concatenateContent(content []*ai.Part) string {
	if len(content) == 0 {
		return ""
	}
	
	var result string
	for _, part := range content {
		if part.IsText() {
			result += part.Text
		}
	}
	return result
}

func (g *ModularGenerator) convertFromOpenAI(response *openai.ChatCompletion) *ai.ModelResponse {
	if len(response.Choices) == 0 {
		return &ai.ModelResponse{}
	}
	
	choice := response.Choices[0]
	
	return &ai.ModelResponse{
		Message: &ai.Message{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewTextPart(choice.Message.Content),
			},
		},
		Usage: &ai.GenerationUsage{
			InputTokens:  int(response.Usage.PromptTokens),
			OutputTokens: int(response.Usage.CompletionTokens),
			TotalTokens:  int(response.Usage.TotalTokens),
		},
		FinishReason: ai.FinishReasonStop,
	}
}

// openAIWrapper adapts openai.Client to our ConversationFinalizer interface
type openAIWrapper struct {
	client *openai.Client
}

func (w *openAIWrapper) GenerateCompletion(ctx context.Context, messages []*ai.Message, modelName string) (*ai.ModelResponse, error) {
	// Convert messages to OpenAI format and make API call
	// This is a simplified implementation
	return &ai.ModelResponse{
		Message: &ai.Message{
			Role:    ai.RoleModel,
			Content: []*ai.Part{ai.NewTextPart("Final response generated")},
		},
	}, nil
}