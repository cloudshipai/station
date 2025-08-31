package context

import (
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// Manager handles context window utilization and prevents overflow
type Manager struct {
	ModelContextLimit    int         `json:"model_context_limit"`
	CurrentTokens       int         `json:"current_tokens"`
	InputTokens         int         `json:"input_tokens"`
	OutputTokens        int         `json:"output_tokens"`
	ThresholdPercent    float64     `json:"threshold_percent"` // Default: 0.9
	SummarizationBuffer int         `json:"summarization_buffer"` // Reserve tokens for summarization
	LastMeasurement     time.Time   `json:"last_measurement"`
	ModelName           string      `json:"model_name"`
	LogCallback         LogCallback `json:"-"`
}

// NewManager creates a new context manager for the specified model
func NewManager(modelName string, logCallback LogCallback) *Manager {
	return &Manager{
		ModelContextLimit:   getModelContextLimit(modelName),
		ThresholdPercent:    0.9, // 90% threshold like OpenCode
		SummarizationBuffer: 4000, // Reserve tokens for summarization
		ModelName:          modelName,
		LogCallback:        logCallback,
		LastMeasurement:    time.Now(),
	}
}

// getModelContextLimit returns the context window size for different models
func getModelContextLimit(modelName string) int {
	modelLower := strings.ToLower(modelName)
	
	
	// OpenAI models (handle both with and without provider prefix)
	if strings.Contains(modelLower, "gpt-5") {
		return 128000 // GPT-5 has 128k context
	}
	if strings.Contains(modelLower, "gpt-4") {
		if strings.Contains(modelLower, "gpt-4o") {
			return 128000 // GPT-4o has 128k context
		}
		return 8192 // GPT-4 has 8k context (some versions have 32k)
	}
	if strings.Contains(modelLower, "gpt-3.5") {
		return 16385 // GPT-3.5-turbo has ~16k context
	}
	
	// Claude models (via OpenAI-compatible API)
	if strings.Contains(modelLower, "claude-3") {
		if strings.Contains(modelLower, "claude-3-5-sonnet") {
			return 200000 // Claude 3.5 Sonnet has 200k context
		}
		return 200000 // Claude 3 models generally have 200k context
	}
	
	// Gemini models
	if strings.Contains(modelLower, "gemini") {
		if strings.Contains(modelLower, "gemini-1.5") {
			return 1000000 // Gemini 1.5 has 1M context
		}
		return 32768 // Gemini 1.0 has 32k context
	}
	
	// Default fallback - 200k tokens works for most modern models
	return 200000
}

// TrackTokenUsage updates the current token usage from generation response
func (m *Manager) TrackTokenUsage(usage TokenUsage) {
	m.InputTokens = usage.InputTokens
	m.OutputTokens = usage.OutputTokens
	m.CurrentTokens = usage.TotalTokens
	m.LastMeasurement = time.Now()
	
	// Log token usage update
	if m.LogCallback != nil {
		m.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   "Context usage updated",
			"details": map[string]interface{}{
				"model":                m.ModelName,
				"total_tokens":         m.CurrentTokens,
				"input_tokens":         m.InputTokens,
				"output_tokens":        m.OutputTokens,
				"context_limit":        m.ModelContextLimit,
				"utilization_percent":  m.GetUtilizationPercent() * 100,
				"tokens_remaining":     m.GetTokensRemaining(),
			},
		})
	}
}

// EstimateTokensInMessages provides rough token estimation for messages
func (m *Manager) EstimateTokensInMessages(messages []*ai.Message) int {
	totalChars := 0
	
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.IsText() {
				totalChars += len(part.Text)
			} else if part.IsToolRequest() && part.ToolRequest != nil {
				// Estimate tokens for tool request
				totalChars += len(part.ToolRequest.Name) + 50 // Name + overhead
				if part.ToolRequest.Input != nil {
					totalChars += len(fmt.Sprintf("%v", part.ToolRequest.Input))
				}
			} else if part.IsToolResponse() && part.ToolResponse != nil {
				// Estimate tokens for tool response
				totalChars += len(part.ToolResponse.Name) + 50 // Name + overhead
				totalChars += len(fmt.Sprintf("%v", part.ToolResponse.Output))
			}
		}
	}
	
	// Rough approximation: 1 token ≈ 4 characters for English text
	estimatedTokens := totalChars / 4
	
	// Add overhead for message structure, role markers, etc.
	overhead := len(messages) * 10 // ~10 tokens per message for structure
	
	return estimatedTokens + overhead
}

// CanExecuteAction checks if an action can be safely executed without overflow
func (m *Manager) CanExecuteAction(estimatedResponseTokens int) (bool, string) {
	projectedTotal := m.CurrentTokens + estimatedResponseTokens + m.SummarizationBuffer
	
	if projectedTotal > m.ModelContextLimit {
		remainingSpace := m.ModelContextLimit - m.CurrentTokens
		return false, fmt.Sprintf("Action would exceed context limit (need %d tokens, only %d remaining)", 
			estimatedResponseTokens, remainingSpace)
	}
	
	return true, "Action can be executed safely"
}

// ShouldSummarize determines if conversation should be summarized based on utilization
func (m *Manager) ShouldSummarize() (bool, string) {
	utilization := m.GetUtilizationPercent()
	
	if utilization >= m.ThresholdPercent {
		return true, fmt.Sprintf("Context utilization %.1f%% >= %.1f%% threshold", 
			utilization*100, m.ThresholdPercent*100)
	}
	
	return false, fmt.Sprintf("Context utilization %.1f%% below threshold", utilization*100)
}

// GetUtilizationPercent returns current context utilization as percentage (0.0-1.0)
func (m *Manager) GetUtilizationPercent() float64 {
	if m.ModelContextLimit == 0 {
		return 0.0
	}
	return float64(m.CurrentTokens) / float64(m.ModelContextLimit)
}

// GetTokensRemaining returns how many tokens are left in the context window
func (m *Manager) GetTokensRemaining() int {
	remaining := m.ModelContextLimit - m.CurrentTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetSafeActionLimit returns the maximum tokens that can be safely used for next action
func (m *Manager) GetSafeActionLimit() int {
	available := m.GetTokensRemaining() - m.SummarizationBuffer
	if available < 0 {
		return 0
	}
	return available
}

// IsApproachingLimit checks if we're approaching the context limit
func (m *Manager) IsApproachingLimit() bool {
	return m.GetUtilizationPercent() >= m.ThresholdPercent
}

// GetContextStatus returns comprehensive context status information
func (m *Manager) GetContextStatus() ContextStatus {
	utilization := m.GetUtilizationPercent()
	tokensRemaining := m.GetTokensRemaining()
	
	var recommendedAction string
	canExecute := tokensRemaining > m.SummarizationBuffer
	shouldSummarize := utilization >= m.ThresholdPercent
	
	if shouldSummarize {
		recommendedAction = "Summarize conversation to free context space"
	} else if utilization > 0.7 {
		recommendedAction = "Use context-efficient tools and operations"
	} else if utilization > 0.5 {
		recommendedAction = "Monitor context usage closely"
	} else {
		recommendedAction = "Normal operation - sufficient context available"
	}
	
	return ContextStatus{
		CanExecuteAction:   canExecute,
		ShouldSummarize:    shouldSummarize,
		UtilizationPercent: utilization,
		TokensRemaining:    tokensRemaining,
		RecommendedAction:  recommendedAction,
	}
}

// LogContextWarning logs a warning about context utilization if needed
func (m *Manager) LogContextWarning() {
	status := m.GetContextStatus()
	
	if m.LogCallback == nil {
		return
	}
	
	if status.ShouldSummarize {
		m.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "warning",
			"message":   "CONTEXT THRESHOLD EXCEEDED - summarization recommended",
			"details": map[string]interface{}{
				"utilization_percent": status.UtilizationPercent * 100,
				"tokens_used":        m.CurrentTokens,
				"tokens_remaining":   status.TokensRemaining,
				"context_limit":      m.ModelContextLimit,
				"recommended_action": status.RecommendedAction,
				"risk_level":        "HIGH",
			},
		})
	} else if status.UtilizationPercent > 0.7 {
		m.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "warning",
			"message":   "Context utilization approaching limit",
			"details": map[string]interface{}{
				"utilization_percent": status.UtilizationPercent * 100,
				"tokens_remaining":   status.TokensRemaining,
				"recommended_action": status.RecommendedAction,
				"risk_level":        "MEDIUM",
			},
		})
	}
}

// Reset resets the context manager state (useful for new conversations)
func (m *Manager) Reset() {
	m.CurrentTokens = 0
	m.InputTokens = 0
	m.OutputTokens = 0
	m.LastMeasurement = time.Now()
	
	if m.LogCallback != nil {
		m.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "debug",
			"message":   "Context manager reset for new conversation",
			"details": map[string]interface{}{
				"model":         m.ModelName,
				"context_limit": m.ModelContextLimit,
			},
		})
	}
}

// UpdateModel changes the model and updates context limits accordingly
func (m *Manager) UpdateModel(modelName string) {
	oldLimit := m.ModelContextLimit
	m.ModelName = modelName
	m.ModelContextLimit = getModelContextLimit(modelName)
	
	if m.LogCallback != nil {
		m.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Context manager model updated",
			"details": map[string]interface{}{
				"old_model":      m.ModelName,
				"new_model":      modelName,
				"old_limit":      oldLimit,
				"new_limit":      m.ModelContextLimit,
				"current_usage":  m.CurrentTokens,
			},
		})
	}
}