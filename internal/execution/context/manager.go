// Package context provides context window management and protection for Station's agent execution
package context

import (
	"log/slog"
	"strings"

	"github.com/firebase/genkit/go/ai"
)

// Manager handles context window monitoring and threshold detection
type Manager struct {
	maxTokens     int     // Maximum context window size
	threshold     float64 // Usage threshold (0.0-1.0) to trigger protection
	currentTokens int     // Current estimated token usage
}

// NewManager creates a new context manager
func NewManager(maxTokens int, threshold float64) *Manager {
	return &Manager{
		maxTokens: maxTokens,
		threshold: threshold,
	}
}

// LogCallback is a function type for sending context utilization data to the UI
type LogCallback func(map[string]interface{})

// WouldExceedThreshold checks if the request would exceed the context threshold
func (m *Manager) WouldExceedThreshold(req *ai.ModelRequest) bool {
	estimatedTokens := m.estimateTokenUsage(req)
	usageRatio := float64(estimatedTokens) / float64(m.maxTokens)
	
	willExceed := usageRatio >= m.threshold
	
	slog.Debug("Context threshold check",
		"estimated_tokens", estimatedTokens,
		"max_tokens", m.maxTokens,
		"usage_ratio", usageRatio,
		"threshold", m.threshold,
		"will_exceed", willExceed,
	)
	
	return willExceed
}

// WouldExceedThresholdWithCallback checks threshold and optionally sends context info to UI
func (m *Manager) WouldExceedThresholdWithCallback(req *ai.ModelRequest, logCallback LogCallback) bool {
	estimatedTokens := m.estimateTokenUsage(req)
	usageRatio := float64(estimatedTokens) / float64(m.maxTokens)
	remainingTokens := m.maxTokens - estimatedTokens
	if remainingTokens < 0 {
		remainingTokens = 0
	}
	
	willExceed := usageRatio >= m.threshold
	
	// Send context utilization info to UI via LogCallback
	if logCallback != nil {
		logCallback(map[string]interface{}{
			"timestamp": slog.TimeKey,
			"level":     "info",
			"message":   "Context utilization check",
			"details": map[string]interface{}{
				"estimated_tokens":   estimatedTokens,
				"max_tokens":        m.maxTokens,
				"remaining_tokens":  remainingTokens,
				"usage_ratio":       usageRatio,
				"usage_percentage":  usageRatio * 100,
				"threshold":         m.threshold,
				"threshold_percentage": m.threshold * 100,
				"will_exceed":       willExceed,
				"status": func() string {
					if willExceed {
						return "warning" // Context compaction will be triggered
					} else if usageRatio > 0.7 {
						return "caution" // Getting close to threshold
					}
					return "normal" // Plenty of context remaining
				}(),
			},
		})
	}
	
	slog.Debug("Context threshold check",
		"estimated_tokens", estimatedTokens,
		"max_tokens", m.maxTokens,
		"usage_ratio", usageRatio,
		"threshold", m.threshold,
		"will_exceed", willExceed,
	)
	
	return willExceed
}

// GetCurrentUtilization returns the current context utilization ratio
func (m *Manager) GetCurrentUtilization(req *ai.ModelRequest) float64 {
	estimatedTokens := m.estimateTokenUsage(req)
	return float64(estimatedTokens) / float64(m.maxTokens)
}

// GetRemainingCapacity returns the remaining token capacity
func (m *Manager) GetRemainingCapacity(req *ai.ModelRequest) int {
	estimatedTokens := m.estimateTokenUsage(req)
	remaining := m.maxTokens - estimatedTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsOutputTooLarge checks if a tool output would be too large for context
func (m *Manager) IsOutputTooLarge(output interface{}, maxSize int) bool {
	if output == nil {
		return false
	}
	
	// Convert output to string for size estimation
	outputStr := ""
	switch v := output.(type) {
	case string:
		outputStr = v
	case []byte:
		outputStr = string(v)
	default:
		// For other types, use a rough estimation
		outputStr = strings.Repeat("x", 100) // Conservative estimate
	}
	
	// Rough token estimation: ~4 chars per token
	estimatedTokens := len(outputStr) / 4
	return estimatedTokens > maxSize
}

// estimateTokenUsage provides a rough estimation of token usage for a request
// This is a simplified implementation - production could use actual tokenizers
func (m *Manager) estimateTokenUsage(req *ai.ModelRequest) int {
	if req == nil {
		return 0
	}
	
	totalChars := 0
	
	// Estimate tokens from messages
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}
		
		for _, part := range msg.Content {
			if part == nil {
				continue
			}
			
			if part.IsText() {
				totalChars += len(part.Text)
			} else if part.IsToolRequest() {
				// Tool requests have overhead
				totalChars += 100 // Base overhead
				if part.ToolRequest != nil {
					totalChars += len(part.ToolRequest.Name) * 2 // Tool name
					// Rough estimation for input data
					totalChars += 200
				}
			} else if part.IsToolResponse() {
				// Tool responses
				totalChars += 100 // Base overhead
				if part.ToolResponse != nil {
					totalChars += len(part.ToolResponse.Name) * 2
					// Estimate output size
					if part.ToolResponse.Output != nil {
						switch v := part.ToolResponse.Output.(type) {
						case string:
							totalChars += len(v)
						default:
							totalChars += 300 // Conservative estimate
						}
					}
				}
			}
		}
	}
	
	// Add overhead for system prompts, tool definitions, etc.
	systemOverhead := 500
	
	// Rough conversion: ~4 characters per token (varies by language/model)
	estimatedTokens := (totalChars + systemOverhead) / 4
	
	return estimatedTokens
}