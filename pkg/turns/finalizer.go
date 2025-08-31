package turns

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// OpenAIClient interface to avoid circular imports
type OpenAIClient interface {
	GenerateCompletion(ctx context.Context, messages []*ai.Message, modelName string) (*ai.ModelResponse, error)
}

// ConversationFinalizer handles generating final responses when limits are reached
type ConversationFinalizer struct {
	Client      OpenAIClient `json:"-"`
	LogCallback LogCallback  `json:"-"`
}

// NewConversationFinalizer creates a new conversation finalizer
func NewConversationFinalizer(client OpenAIClient, logCallback LogCallback) *ConversationFinalizer {
	return &ConversationFinalizer{
		Client:      client,
		LogCallback: logCallback,
	}
}

// GenerateFinalResponse ensures the conversation ends with an LLM response, regardless of why completion was forced
func (f *ConversationFinalizer) GenerateFinalResponse(ctx context.Context, request FinalizationRequest) (*FinalizationResponse, error) {
	startTime := time.Now()
	
	if f.LogCallback != nil {
		f.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Generating final response due to conversation limit",
			"details": map[string]interface{}{
				"reason":               string(request.Reason),
				"conversation_length":  len(request.Conversation),
				"preserve_last_n":     request.PreserveLastN,
				"force_final":         request.ForceFinal,
			},
		})
	}
	
	// Create the final prompt that instructs the LLM to provide a concluding response
	finalPrompt := f.createFinalPrompt(request.Reason, request.Conversation)
	
	// Prepare the conversation for final response
	// We keep the conversation history but add a final user message requesting completion
	finalConversation := make([]*ai.Message, 0, len(request.Conversation)+1)
	
	// If we need to preserve only the last N messages, do so
	if request.PreserveLastN > 0 && len(request.Conversation) > request.PreserveLastN {
		// Create a summary of the earlier conversation
		if request.CreateSummary {
			summaryText := f.createConversationSummary(request.Conversation[:len(request.Conversation)-request.PreserveLastN])
			summaryMessage := &ai.Message{
				Role: ai.RoleSystem,
				Content: []*ai.Part{
					ai.NewTextPart(fmt.Sprintf("CONVERSATION SUMMARY: %s", summaryText)),
				},
			}
			finalConversation = append(finalConversation, summaryMessage)
		}
		
		// Add the preserved messages
		preservedMessages := request.Conversation[len(request.Conversation)-request.PreserveLastN:]
		finalConversation = append(finalConversation, preservedMessages...)
	} else {
		// Use the full conversation
		finalConversation = append(finalConversation, request.Conversation...)
	}
	
	// Add the final prompt that requests completion
	finalMessage := &ai.Message{
		Role: ai.RoleUser,
		Content: []*ai.Part{
			ai.NewTextPart(finalPrompt),
		},
	}
	finalConversation = append(finalConversation, finalMessage)
	
	if f.LogCallback != nil {
		f.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   "Final conversation prepared",
			"details": map[string]interface{}{
				"original_length": len(request.Conversation),
				"final_length":    len(finalConversation),
				"final_prompt":    finalPrompt[:min(100, len(finalPrompt))] + "...",
			},
		})
	}
	
	// Generate the final response WITHOUT any tools to ensure text-only completion
	// This is critical - we don't want the LLM to call more tools at the final stage
	finalResponse, err := f.Client.GenerateCompletion(ctx, finalConversation, request.ModelName)
	if err != nil {
		// If the final response fails, create a fallback response
		if f.LogCallback != nil {
			f.LogCallback(map[string]interface{}{
				"timestamp": time.Now().Format(time.RFC3339),
				"level":     "error",
				"message":   "Final response generation failed, creating fallback",
				"details": map[string]interface{}{
					"error": err.Error(),
				},
			})
		}
		
		fallbackResponse := f.createFallbackResponse(request.Reason, request.Conversation)
		return &FinalizationResponse{
			FinalResponse: fallbackResponse,
			Summary:      "Conversation completed due to limits, fallback response generated",
			Success:      false,
			Error:        fmt.Sprintf("Final response generation failed: %v", err),
			TokensUsed:   0,
		}, nil
	}
	
	// Extract token usage if available
	tokensUsed := 0
	if finalResponse.Usage != nil {
		tokensUsed = finalResponse.Usage.TotalTokens
	}
	
	duration := time.Since(startTime)
	
	if f.LogCallback != nil {
		f.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Final response generated successfully",
			"details": map[string]interface{}{
				"duration_seconds":  duration.Seconds(),
				"tokens_used":       tokensUsed,
				"response_length":   len(finalResponse.Text()),
				"completion_reason": string(request.Reason),
			},
		})
	}
	
	return &FinalizationResponse{
		FinalResponse: finalResponse,
		Summary:      f.createConversationSummary(request.Conversation),
		Success:      true,
		TokensUsed:   tokensUsed,
		FinalMessage: finalResponse.Message,
		Error:        "",
	}, nil
}

// createFinalPrompt creates an appropriate prompt for final response generation
func (f *ConversationFinalizer) createFinalPrompt(reason CompletionReason, conversation []*ai.Message) string {
	var prompt strings.Builder
	
	// Base instruction that applies to all completion reasons
	prompt.WriteString("You have reached a conversation limit and must provide a final response. ")
	
	// Customize the prompt based on the specific reason
	switch reason {
	case CompletionReasonTurnLimit:
		prompt.WriteString("You have reached the maximum number of conversation turns allowed. ")
		prompt.WriteString("Please provide a comprehensive final response that summarizes what you have accomplished, ")
		prompt.WriteString("any findings you discovered, and any remaining tasks or recommendations. ")
		prompt.WriteString("Do not call any more tools - only provide your final text response.")
		
	case CompletionReasonContextLimit:
		prompt.WriteString("The conversation context is approaching its limit. ")
		prompt.WriteString("Please provide a concise final response that captures the key points of our discussion ")
		prompt.WriteString("and any important conclusions or next steps. ")
		prompt.WriteString("Keep your response focused and avoid calling additional tools.")
		
	case CompletionReasonToolLimit:
		prompt.WriteString("You have used the maximum number of tools allowed. ")
		prompt.WriteString("Based on the information you have gathered through your tool usage, ")
		prompt.WriteString("please provide a final analysis or response that incorporates your findings. ")
		prompt.WriteString("Do not attempt to use any more tools.")
		
	case CompletionReasonTimeLimit:
		prompt.WriteString("The conversation has reached its time limit. ")
		prompt.WriteString("Please provide a quick summary of what you have accomplished so far ")
		prompt.WriteString("and any immediate recommendations or conclusions.")
		
	case CompletionReasonError:
		prompt.WriteString("An error occurred that requires ending the conversation. ")
		prompt.WriteString("Please provide a helpful response based on what you were able to accomplish ")
		prompt.WriteString("before the error occurred.")
		
	default:
		prompt.WriteString("Please provide a final response summarizing our conversation ")
		prompt.WriteString("and any key outcomes or recommendations.")
	}
	
	// Add context about the conversation if we have tool usage
	toolCallCount := f.countToolCalls(conversation)
	if toolCallCount > 0 {
		prompt.WriteString(fmt.Sprintf("\n\nNote: You used %d tools during this conversation. ", toolCallCount))
		prompt.WriteString("Please reference the information you gathered and the actions you took in your final response.")
	}
	
	return prompt.String()
}

// createConversationSummary creates a brief summary of the conversation
func (f *ConversationFinalizer) createConversationSummary(messages []*ai.Message) string {
	if len(messages) == 0 {
		return "Empty conversation"
	}
	
	// Count different types of interactions
	userMessages := 0
	toolCalls := 0
	assistantResponses := 0
	
	var lastUserMessage string
	var toolsUsed []string
	
	for _, msg := range messages {
		switch msg.Role {
		case ai.RoleUser:
			userMessages++
			// Capture the last substantive user message
			for _, part := range msg.Content {
				if part.IsText() && len(part.Text) > 20 {
					lastUserMessage = part.Text
					if len(lastUserMessage) > 100 {
						lastUserMessage = lastUserMessage[:100] + "..."
					}
				}
			}
		case ai.RoleModel:
			assistantResponses++
			// Count tool calls in assistant messages
			for _, part := range msg.Content {
				if part.IsToolRequest() && part.ToolRequest != nil {
					toolCalls++
					toolName := part.ToolRequest.Name
					if !contains(toolsUsed, toolName) {
						toolsUsed = append(toolsUsed, toolName)
					}
				}
			}
		case ai.RoleTool:
			// Tool responses are already counted via tool calls
		}
	}
	
	summary := fmt.Sprintf("Conversation involved %d user messages, %d assistant responses, and %d tool calls", 
		userMessages, assistantResponses, toolCalls)
	
	if len(toolsUsed) > 0 {
		if len(toolsUsed) <= 3 {
			summary += fmt.Sprintf(" using tools: %s", strings.Join(toolsUsed, ", "))
		} else {
			summary += fmt.Sprintf(" using %d different tools including: %s", len(toolsUsed), strings.Join(toolsUsed[:3], ", "))
		}
	}
	
	if lastUserMessage != "" {
		summary += fmt.Sprintf(". Last user request: \"%s\"", lastUserMessage)
	}
	
	return summary
}

// createFallbackResponse creates a fallback response when final generation fails
func (f *ConversationFinalizer) createFallbackResponse(reason CompletionReason, conversation []*ai.Message) *ai.ModelResponse {
	var responseText strings.Builder
	
	responseText.WriteString("I apologize, but I've reached a conversation limit and need to provide a final response. ")
	
	// Analyze what was accomplished
	toolCalls := f.countToolCalls(conversation)
	if toolCalls > 0 {
		responseText.WriteString(fmt.Sprintf("During our conversation, I used %d tools to help with your request. ", toolCalls))
	}
	
	switch reason {
	case CompletionReasonTurnLimit:
		responseText.WriteString("I reached the maximum number of conversation turns, but I've done my best to address your request with the information I gathered.")
	case CompletionReasonContextLimit:
		responseText.WriteString("The conversation context became too large to continue, but I've processed the information you provided.")
	case CompletionReasonToolLimit:
		responseText.WriteString("I've used the maximum number of tools allowed and gathered the available information.")
	case CompletionReasonError:
		responseText.WriteString("Although an error occurred, I was able to make progress on your request.")
	default:
		responseText.WriteString("I've completed the analysis with the available information.")
	}
	
	responseText.WriteString(" If you need further assistance, please feel free to start a new conversation with more specific questions.")
	
	return &ai.ModelResponse{
		Message: &ai.Message{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewTextPart(responseText.String()),
			},
		},
		FinishReason: ai.FinishReasonStop,
		Usage: &ai.GenerationUsage{
			InputTokens:  0,
			OutputTokens: len(responseText.String()) / 4, // Rough estimate
			TotalTokens:  len(responseText.String()) / 4,
		},
	}
}

// countToolCalls counts the total number of tool calls in the conversation
func (f *ConversationFinalizer) countToolCalls(messages []*ai.Message) int {
	count := 0
	for _, msg := range messages {
		if msg.Role == ai.RoleModel {
			for _, part := range msg.Content {
				if part.IsToolRequest() {
					count++
				}
			}
		}
	}
	return count
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}