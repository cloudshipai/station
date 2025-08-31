package turns

import (
	"fmt"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
)

// Limiter manages turn counting and enforces limits intelligently
type Limiter struct {
	Config          LimiterConfig   `json:"config"`
	currentTurns    int            `json:"current_turns"`
	turnStartTimes  []time.Time    `json:"-"` // Track turn timing
	ContextManager  ContextManager `json:"-"` // Interface to context manager
	LogCallback     LogCallback    `json:"-"`
}

// NewLimiter creates a new turn limiter with the specified configuration
func NewLimiter(config LimiterConfig, contextManager ContextManager, logCallback LogCallback) *Limiter {
	// Set sensible defaults
	if config.MaxTurns == 0 {
		config.MaxTurns = 25 // Default to 25 turns like current system
	}
	if config.WarningThreshold == 0 {
		config.WarningThreshold = 0.8 // 80%
	}
	if config.CriticalThreshold == 0 {
		config.CriticalThreshold = 0.9 // 90%
	}
	if config.Strategy == "" {
		config.Strategy = TurnStrategyFixed
	}

	return &Limiter{
		Config:         config,
		currentTurns:   0,
		turnStartTimes: make([]time.Time, 0),
		ContextManager: contextManager,
		LogCallback:    logCallback,
	}
}

// IncrementTurn increments the turn counter and logs if needed
func (l *Limiter) IncrementTurn() {
	l.currentTurns++
	l.turnStartTimes = append(l.turnStartTimes, time.Now())
	
	// Log turn increment with analysis
	if l.LogCallback != nil {
		metrics := l.GetMetrics()
		analysis := l.AnalyzeTurnUsage(nil) // Pass nil for basic analysis
		
		logLevel := "debug"
		message := fmt.Sprintf("Turn %d/%d completed", l.currentTurns, l.Config.MaxTurns)
		
		// Upgrade log level based on utilization
		if metrics.UtilizationPercent >= l.Config.CriticalThreshold {
			logLevel = "warning"
			message = fmt.Sprintf("CRITICAL: Turn %d/%d - approaching limit", l.currentTurns, l.Config.MaxTurns)
		} else if metrics.UtilizationPercent >= l.Config.WarningThreshold {
			logLevel = "warning"
			message = fmt.Sprintf("WARNING: Turn %d/%d - %d turns remaining", l.currentTurns, l.Config.MaxTurns, metrics.TurnsRemaining)
		}
		
		l.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     logLevel,
			"message":   message,
			"details": map[string]interface{}{
				"current_turns":        l.currentTurns,
				"max_turns":           l.Config.MaxTurns,
				"turns_remaining":     metrics.TurnsRemaining,
				"utilization_percent": metrics.UtilizationPercent * 100,
				"strategy":            l.Config.Strategy,
				"efficiency":          analysis.Efficiency,
				"recommended_action":  analysis.RecommendedAction,
			},
		})
	}
}

// CanContinue checks if the conversation can continue based on turn limits and other factors
func (l *Limiter) CanContinue() (bool, string) {
	// Always check turn limit first
	if l.currentTurns >= l.Config.MaxTurns {
		return false, fmt.Sprintf("Turn limit reached (%d/%d)", l.currentTurns, l.Config.MaxTurns)
	}
	
	// Check context manager if available and context-aware mode is enabled
	if l.Config.ContextAware && l.ContextManager != nil {
		contextUtilization := l.ContextManager.GetUtilizationPercent()
		
		// If context is full, we can't continue even if we have turns left
		if l.ContextManager.IsApproachingLimit() {
			return false, fmt.Sprintf("Context limit approaching (%.1f%% utilization) with %d turns remaining", 
				contextUtilization*100, l.Config.MaxTurns-l.currentTurns)
		}
	}
	
	return true, fmt.Sprintf("Can continue (%d/%d turns used)", l.currentTurns, l.Config.MaxTurns)
}

// ShouldForceCompletion determines if conversation should be forced to complete
func (l *Limiter) ShouldForceCompletion(messages []*ai.Message) (bool, CompletionReason, string) {
	// Check turn utilization
	utilization := float64(l.currentTurns) / float64(l.Config.MaxTurns)
	
	// Force completion if at critical threshold
	if utilization >= l.Config.CriticalThreshold {
		reason := fmt.Sprintf("Turn utilization %.1f%% >= critical threshold %.1f%%", 
			utilization*100, l.Config.CriticalThreshold*100)
		return true, CompletionReasonTurnLimit, reason
	}
	
	// Check context limits if context-aware
	if l.Config.ContextAware && l.ContextManager != nil {
		if l.ContextManager.IsApproachingLimit() {
			contextUtilization := l.ContextManager.GetUtilizationPercent()
			reason := fmt.Sprintf("Context approaching limit (%.1f%% utilization)", contextUtilization*100)
			return true, CompletionReasonContextLimit, reason
		}
	}
	
	// Analyze conversation patterns if messages provided
	if messages != nil {
		analysis := l.AnalyzeTurnUsage(messages)
		
		// Force completion if efficiency is very low and we're past warning threshold
		if analysis.Efficiency < 0.3 && utilization >= l.Config.WarningThreshold {
			reason := fmt.Sprintf("Low efficiency (%.1f) with high turn usage (%.1f%%)", 
				analysis.Efficiency, utilization*100)
			return true, CompletionReasonTurnLimit, reason
		}
		
		// Force completion if progress is stalling
		if analysis.ProgressStalling && utilization >= l.Config.WarningThreshold {
			reason := "Progress stalling detected - repeated similar actions"
			return true, CompletionReasonTurnLimit, reason
		}
	}
	
	return false, "", ""
}

// GetAdaptiveLimit calculates adaptive turn limit based on context and task complexity
func (l *Limiter) GetAdaptiveLimit(contextUtilization float64, taskComplexity TaskComplexity) int {
	if !l.Config.EnableAdaptive {
		return l.Config.MaxTurns
	}
	
	baseLimit := float64(l.Config.MaxTurns)
	
	// Adjust based on context utilization if context-aware
	if l.Config.ContextAware {
		if contextUtilization > 0.8 {
			baseLimit *= 0.6 // Reduce by 40% if context is getting full
		} else if contextUtilization > 0.6 {
			baseLimit *= 0.8 // Reduce by 20% if context is moderately full
		}
	}
	
	// Adjust based on task complexity if enabled
	if l.Config.TaskComplexityAware {
		switch taskComplexity {
		case TaskComplexitySimple:
			baseLimit *= 0.6 // Simple tasks should complete quickly
		case TaskComplexityModerate:
			baseLimit *= 0.8 // Moderate tasks get some reduction
		case TaskComplexityComplex:
			baseLimit *= 1.2 // Complex tasks get more turns
		case TaskComplexityVeryComplex:
			baseLimit *= 1.5 // Very complex tasks get significantly more turns
		}
	}
	
	// Ensure reasonable bounds
	adaptiveLimit := int(baseLimit)
	if adaptiveLimit < 5 {
		adaptiveLimit = 5 // Minimum 5 turns
	}
	if adaptiveLimit > 50 {
		adaptiveLimit = 50 // Maximum 50 turns
	}
	
	// Log adaptive adjustment if different from base
	if adaptiveLimit != l.Config.MaxTurns && l.LogCallback != nil {
		l.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Adaptive turn limit calculated",
			"details": map[string]interface{}{
				"base_limit":         l.Config.MaxTurns,
				"adaptive_limit":     adaptiveLimit,
				"context_utilization": contextUtilization * 100,
				"task_complexity":    taskComplexity,
				"adjustment_ratio":   float64(adaptiveLimit) / float64(l.Config.MaxTurns),
			},
		})
	}
	
	return adaptiveLimit
}

// GetMetrics returns current turn usage metrics
func (l *Limiter) GetMetrics() TurnMetrics {
	remaining := l.Config.MaxTurns - l.currentTurns
	if remaining < 0 {
		remaining = 0
	}
	
	var avgDuration time.Duration
	if len(l.turnStartTimes) > 1 {
		totalDuration := time.Since(l.turnStartTimes[0])
		avgDuration = totalDuration / time.Duration(len(l.turnStartTimes)-1)
	}
	
	var lastTurnTime time.Time
	if len(l.turnStartTimes) > 0 {
		lastTurnTime = l.turnStartTimes[len(l.turnStartTimes)-1]
	}
	
	return TurnMetrics{
		CurrentTurns:        l.currentTurns,
		MaxTurns:           l.Config.MaxTurns,
		TurnsRemaining:     remaining,
		UtilizationPercent: float64(l.currentTurns) / float64(l.Config.MaxTurns),
		Strategy:           l.Config.Strategy,
		LastTurnTime:       lastTurnTime,
		AverageTurnDuration: avgDuration,
	}
}

// AnalyzeTurnUsage analyzes conversation patterns to determine efficiency and issues
func (l *Limiter) AnalyzeTurnUsage(messages []*ai.Message) TurnAnalysis {
	analysis := TurnAnalysis{
		Efficiency:        1.0, // Start with perfect efficiency
		RecommendedAction: "Continue normal operation",
		RiskLevel:        "LOW",
	}
	
	if messages == nil || len(messages) == 0 {
		return analysis
	}
	
	// Count different message types
	var toolMessages, userMessages, assistantMessages int
	var toolNames []string
	var recentTools []string // Last 5 tool calls
	
	for i, msg := range messages {
		switch msg.Role {
		case ai.RoleTool:
			toolMessages++
			// Extract tool names for pattern analysis
			for _, part := range msg.Content {
				if part.IsToolResponse() && part.ToolResponse != nil {
					toolName := part.ToolResponse.Name
					toolNames = append(toolNames, toolName)
					
					// Track recent tools (last 5 messages)
					if i >= len(messages)-5 {
						recentTools = append(recentTools, toolName)
					}
				}
			}
		case ai.RoleUser:
			userMessages++
		case ai.RoleModel:
			assistantMessages++
		}
	}
	
	totalMessages := len(messages)
	if totalMessages == 0 {
		return analysis
	}
	
	// Calculate tool usage ratio
	toolRatio := float64(toolMessages) / float64(totalMessages)
	analysis.ToolHeavy = toolRatio > 0.5
	
	// Detect stalling patterns
	if len(recentTools) >= 3 {
		analysis.ProgressStalling = l.detectStalling(recentTools)
	}
	
	// Classify conversation phase
	readTools := l.countToolsByType(toolNames, []string{"read_text_file", "list_directory", "directory_tree", "get_file_info", "search_files"})
	writeTools := l.countToolsByType(toolNames, []string{"write_text_file", "create_file", "edit_file", "append_file"})
	
	analysis.InformationGathering = readTools > writeTools && toolRatio > 0.3
	analysis.ExecutionPhase = writeTools > readTools && toolRatio > 0.2
	
	// Calculate efficiency score
	turnUtilization := float64(l.currentTurns) / float64(l.Config.MaxTurns)
	
	// Efficiency factors
	if analysis.ProgressStalling {
		analysis.Efficiency -= 0.4 // Major penalty for stalling
	}
	if toolRatio > 0.8 {
		analysis.Efficiency -= 0.2 // Penalty for excessive tool use
	}
	if turnUtilization > 0.8 && analysis.InformationGathering {
		analysis.Efficiency -= 0.3 // Penalty for excessive information gathering
	}
	
	// Ensure efficiency is within bounds
	if analysis.Efficiency < 0 {
		analysis.Efficiency = 0
	}
	
	// Determine risk level and recommended action
	if turnUtilization > 0.9 {
		analysis.RiskLevel = "CRITICAL"
		analysis.RecommendedAction = "Force completion immediately"
	} else if turnUtilization > 0.8 {
		analysis.RiskLevel = "HIGH"
		if analysis.ProgressStalling {
			analysis.RecommendedAction = "Force completion - progress stalling detected"
		} else {
			analysis.RecommendedAction = "Consider completion - high turn usage"
		}
	} else if turnUtilization > 0.6 {
		analysis.RiskLevel = "MEDIUM"
		if analysis.ToolHeavy {
			analysis.RecommendedAction = "Monitor tool usage - consider more targeted approaches"
		} else {
			analysis.RecommendedAction = "Monitor progress - moderate turn usage"
		}
	}
	
	return analysis
}

// detectStalling checks if recent tool calls indicate stalling behavior
func (l *Limiter) detectStalling(recentTools []string) bool {
	if len(recentTools) < 3 {
		return false
	}
	
	// Count occurrences of each tool
	toolCounts := make(map[string]int)
	for _, tool := range recentTools {
		toolCounts[tool]++
	}
	
	// If any tool appears >50% of the time in recent calls, it's likely stalling
	threshold := len(recentTools) / 2
	for _, count := range toolCounts {
		if count > threshold {
			return true
		}
	}
	
	// Check for alternating patterns (A-B-A-B-A)
	if len(recentTools) >= 4 {
		alternating := true
		for i := 2; i < len(recentTools); i++ {
			if recentTools[i] != recentTools[i-2] {
				alternating = false
				break
			}
		}
		if alternating {
			return true
		}
	}
	
	return false
}

// countToolsByType counts how many tools of specific types were used
func (l *Limiter) countToolsByType(toolNames []string, targetTypes []string) int {
	count := 0
	for _, toolName := range toolNames {
		for _, targetType := range targetTypes {
			if strings.Contains(toolName, targetType) {
				count++
				break
			}
		}
	}
	return count
}

// Reset resets the turn counter for a new conversation
func (l *Limiter) Reset() {
	oldTurns := l.currentTurns
	l.currentTurns = 0
	l.turnStartTimes = make([]time.Time, 0)
	
	if l.LogCallback != nil && oldTurns > 0 {
		l.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Turn limiter reset for new conversation",
			"details": map[string]interface{}{
				"previous_turns": oldTurns,
				"max_turns":     l.Config.MaxTurns,
				"strategy":      l.Config.Strategy,
			},
		})
	}
}

// UpdateConfig updates the limiter configuration
func (l *Limiter) UpdateConfig(config LimiterConfig) {
	oldMaxTurns := l.Config.MaxTurns
	l.Config = config
	
	if l.LogCallback != nil && oldMaxTurns != config.MaxTurns {
		l.LogCallback(map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
			"level":     "info",
			"message":   "Turn limiter configuration updated",
			"details": map[string]interface{}{
				"old_max_turns": oldMaxTurns,
				"new_max_turns": config.MaxTurns,
				"strategy":      config.Strategy,
				"current_turns": l.currentTurns,
			},
		})
	}
}

// GetCurrentTurns returns the current turn count
func (l *Limiter) GetCurrentTurns() int {
	return l.currentTurns
}

// GetMaxTurns returns the maximum allowed turns
func (l *Limiter) GetMaxTurns() int {
	return l.Config.MaxTurns
}