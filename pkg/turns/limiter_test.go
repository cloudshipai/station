package turns

import (
	"testing"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/stretchr/testify/assert"
)

// MockContextManager implements the ContextManager interface for testing
type MockContextManager struct {
	utilizationPercent float64
	shouldSummarize    bool
	isApproachingLimit bool
	safeActionLimit    int
}

func (m *MockContextManager) GetUtilizationPercent() float64 {
	return m.utilizationPercent
}

func (m *MockContextManager) ShouldSummarize() (bool, string) {
	if m.shouldSummarize {
		return true, "Mock context needs summarization"
	}
	return false, "Mock context is fine"
}

func (m *MockContextManager) IsApproachingLimit() bool {
	return m.isApproachingLimit
}

func (m *MockContextManager) GetSafeActionLimit() int {
	return m.safeActionLimit
}

func TestNewLimiter(t *testing.T) {
	tests := []struct {
		name           string
		config         LimiterConfig
		expectedMaxTurns int
		expectedStrategy TurnStrategy
	}{
		{
			name:             "Default configuration",
			config:           LimiterConfig{},
			expectedMaxTurns: 25,
			expectedStrategy: TurnStrategyFixed,
		},
		{
			name: "Custom configuration",
			config: LimiterConfig{
				MaxTurns: 15,
				Strategy: TurnStrategyAdaptive,
				WarningThreshold: 0.7,
			},
			expectedMaxTurns: 15,
			expectedStrategy: TurnStrategyAdaptive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contextManager := &MockContextManager{}
			limiter := NewLimiter(tt.config, contextManager, nil)
			
			assert.Equal(t, tt.expectedMaxTurns, limiter.Config.MaxTurns)
			assert.Equal(t, tt.expectedStrategy, limiter.Config.Strategy)
			assert.Equal(t, 0, limiter.currentTurns)
			
			if tt.config.WarningThreshold == 0 {
				assert.Equal(t, 0.8, limiter.Config.WarningThreshold) // Default
			} else {
				assert.Equal(t, tt.config.WarningThreshold, limiter.Config.WarningThreshold)
			}
		})
	}
}

func TestLimiter_IncrementTurn(t *testing.T) {
	logCalled := false
	var logData map[string]interface{}
	
	logCallback := func(data map[string]interface{}) {
		logCalled = true
		logData = data
	}
	
	config := LimiterConfig{MaxTurns: 5}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, logCallback)
	
	// First turn should be debug level
	limiter.IncrementTurn()
	assert.Equal(t, 1, limiter.currentTurns)
	assert.True(t, logCalled)
	assert.Equal(t, "debug", logData["level"])
	
	// Reset for next test
	logCalled = false
	
	// Turn that hits warning threshold (80% of 5 = 4)
	limiter.currentTurns = 3 // Manually set to test warning
	limiter.IncrementTurn()
	assert.Equal(t, 4, limiter.currentTurns)
	assert.True(t, logCalled)
	assert.Equal(t, "warning", logData["level"])
}

func TestLimiter_CanContinue(t *testing.T) {
	config := LimiterConfig{MaxTurns: 5}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Should be able to continue initially
	canContinue, reason := limiter.CanContinue()
	assert.True(t, canContinue)
	assert.Contains(t, reason, "Can continue")
	
	// Should be able to continue up to the limit
	limiter.currentTurns = 4
	canContinue, reason = limiter.CanContinue()
	assert.True(t, canContinue)
	
	// Should NOT be able to continue at the limit
	limiter.currentTurns = 5
	canContinue, reason = limiter.CanContinue()
	assert.False(t, canContinue)
	assert.Contains(t, reason, "Turn limit reached")
	
	// Test context-aware mode
	limiter.Config.ContextAware = true
	limiter.currentTurns = 3 // Below turn limit
	contextManager.isApproachingLimit = true
	
	canContinue, reason = limiter.CanContinue()
	assert.False(t, canContinue)
	assert.Contains(t, reason, "Context limit approaching")
}

func TestLimiter_ShouldForceCompletion(t *testing.T) {
	config := LimiterConfig{
		MaxTurns: 10,
		CriticalThreshold: 0.9, // 90%
		WarningThreshold: 0.8,  // 80%
	}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Should not force completion early
	limiter.currentTurns = 7 // 70%
	should, reason, description := limiter.ShouldForceCompletion(nil)
	assert.False(t, should)
	
	// Should force completion at critical threshold
	limiter.currentTurns = 9 // 90%
	should, reason, description = limiter.ShouldForceCompletion(nil)
	assert.True(t, should)
	assert.Equal(t, CompletionReasonTurnLimit, reason)
	assert.Contains(t, description, "critical threshold")
	
	// Test context-aware forcing
	limiter.Config.ContextAware = true
	limiter.currentTurns = 5 // Below turn threshold
	contextManager.isApproachingLimit = true
	contextManager.utilizationPercent = 0.95
	
	should, reason, description = limiter.ShouldForceCompletion(nil)
	assert.True(t, should)
	assert.Equal(t, CompletionReasonContextLimit, reason)
	assert.Contains(t, description, "Context approaching limit")
}

func TestLimiter_GetAdaptiveLimit(t *testing.T) {
	config := LimiterConfig{
		MaxTurns: 20,
		EnableAdaptive: true,
		ContextAware: true,
		TaskComplexityAware: true,
	}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	tests := []struct {
		name               string
		contextUtilization float64
		taskComplexity     TaskComplexity
		expectedRange      [2]int // min, max expected range
	}{
		{
			name:               "Simple task, low context",
			contextUtilization: 0.3,
			taskComplexity:     TaskComplexitySimple,
			expectedRange:      [2]int{10, 15}, // 20 * 0.6 = 12
		},
		{
			name:               "Complex task, low context", 
			contextUtilization: 0.3,
			taskComplexity:     TaskComplexityComplex,
			expectedRange:      [2]int{20, 30}, // 20 * 1.2 = 24
		},
		{
			name:               "Simple task, high context",
			contextUtilization: 0.9,
			taskComplexity:     TaskComplexitySimple,
			expectedRange:      [2]int{5, 10}, // 20 * 0.6 * 0.6 = 7.2
		},
		{
			name:               "Very complex task, moderate context",
			contextUtilization: 0.7,
			taskComplexity:     TaskComplexityVeryComplex,
			expectedRange:      [2]int{20, 30}, // 20 * 0.8 * 1.5 = 24
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adaptiveLimit := limiter.GetAdaptiveLimit(tt.contextUtilization, tt.taskComplexity)
			
			assert.GreaterOrEqual(t, adaptiveLimit, tt.expectedRange[0])
			assert.LessOrEqual(t, adaptiveLimit, tt.expectedRange[1])
			
			// Ensure bounds are respected
			assert.GreaterOrEqual(t, adaptiveLimit, 5)  // Minimum
			assert.LessOrEqual(t, adaptiveLimit, 50)    // Maximum
		})
	}
	
	// Test disabled adaptive mode
	limiter.Config.EnableAdaptive = false
	adaptiveLimit := limiter.GetAdaptiveLimit(0.9, TaskComplexityVeryComplex)
	assert.Equal(t, 20, adaptiveLimit) // Should return base limit
}

func TestLimiter_GetMetrics(t *testing.T) {
	config := LimiterConfig{MaxTurns: 10}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Test initial metrics
	metrics := limiter.GetMetrics()
	assert.Equal(t, 0, metrics.CurrentTurns)
	assert.Equal(t, 10, metrics.MaxTurns)
	assert.Equal(t, 10, metrics.TurnsRemaining)
	assert.Equal(t, 0.0, metrics.UtilizationPercent)
	
	// Test after some turns
	limiter.IncrementTurn()
	limiter.IncrementTurn()
	limiter.IncrementTurn()
	
	metrics = limiter.GetMetrics()
	assert.Equal(t, 3, metrics.CurrentTurns)
	assert.Equal(t, 7, metrics.TurnsRemaining)
	assert.InDelta(t, 0.3, metrics.UtilizationPercent, 0.001)
	assert.NotZero(t, metrics.LastTurnTime)
}

func TestLimiter_AnalyzeTurnUsage(t *testing.T) {
	config := LimiterConfig{MaxTurns: 10}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Create a conversation with tool-heavy pattern
	messages := []*ai.Message{
		{
			Role: ai.RoleUser,
			Content: []*ai.Part{ai.NewTextPart("Please analyze this file")},
		},
		{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewToolRequestPart(&ai.ToolRequest{Name: "read_text_file"}),
			},
		},
		{
			Role: ai.RoleTool,
			Content: []*ai.Part{
				ai.NewToolResponsePart(&ai.ToolResponse{Name: "read_text_file", Output: "file content"}),
			},
		},
		{
			Role: ai.RoleModel,
			Content: []*ai.Part{
				ai.NewToolRequestPart(&ai.ToolRequest{Name: "read_text_file"}),
			},
		},
		{
			Role: ai.RoleTool,
			Content: []*ai.Part{
				ai.NewToolResponsePart(&ai.ToolResponse{Name: "read_text_file", Output: "more file content"}),
			},
		},
		{
			Role: ai.RoleTool,
			Content: []*ai.Part{
				ai.NewToolResponsePart(&ai.ToolResponse{Name: "search_files", Output: "search results"}),
			},
		},
		{
			Role: ai.RoleTool,
			Content: []*ai.Part{
				ai.NewToolResponsePart(&ai.ToolResponse{Name: "list_directory", Output: "directory listing"}),
			},
		},
	}
	
	limiter.currentTurns = 8 // 80% turn usage (8/10)
	analysis := limiter.AnalyzeTurnUsage(messages)
	
	assert.True(t, analysis.ToolHeavy) // 4 tool messages out of 7 total = 57% > 50%
	assert.True(t, analysis.InformationGathering) // Only read tools
	assert.False(t, analysis.ExecutionPhase) // No write tools
	// 80% turn utilization should be MEDIUM (>0.6), not HIGH (>0.8)
	// Let's use 90% to get HIGH risk level
	limiter.currentTurns = 9 // 90% turn usage
	analysis = limiter.AnalyzeTurnUsage(messages)
	assert.Equal(t, "HIGH", analysis.RiskLevel) // High turn utilization (90%)
	assert.Contains(t, analysis.RecommendedAction, "completion")
}

func TestLimiter_detectStalling(t *testing.T) {
	config := LimiterConfig{MaxTurns: 10}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Test repetitive tool usage (stalling)
	recentTools := []string{"read_text_file", "read_text_file", "read_text_file"}
	isStalling := limiter.detectStalling(recentTools)
	assert.True(t, isStalling)
	
	// Test alternating pattern (also stalling)
	alternatingTools := []string{"read_text_file", "list_directory", "read_text_file", "list_directory"}
	isStalling = limiter.detectStalling(alternatingTools)
	assert.True(t, isStalling)
	
	// Test normal diverse usage (not stalling)
	diverseTools := []string{"read_text_file", "search_files", "write_text_file"}
	isStalling = limiter.detectStalling(diverseTools)
	assert.False(t, isStalling)
}

func TestLimiter_Reset(t *testing.T) {
	var resetLogCalled bool
	var lastLogData map[string]interface{}
	
	logCallback := func(data map[string]interface{}) {
		lastLogData = data
		// Only mark as called if this is the reset message
		if data["message"] == "Turn limiter reset for new conversation" {
			resetLogCalled = true
		}
	}
	
	config := LimiterConfig{MaxTurns: 10}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, logCallback)
	
	// Set some state (manually to avoid callback noise)
	limiter.currentTurns = 3 // Set directly instead of using IncrementTurn
	for i := 0; i < 3; i++ {
		limiter.turnStartTimes = append(limiter.turnStartTimes, time.Now())
	}
	
	assert.Equal(t, 3, limiter.currentTurns)
	assert.Equal(t, 3, len(limiter.turnStartTimes))
	
	// Reset should clear state and log
	limiter.Reset()
	assert.Equal(t, 0, limiter.currentTurns)
	assert.Equal(t, 0, len(limiter.turnStartTimes))
	assert.True(t, resetLogCalled)
	assert.Equal(t, "Turn limiter reset for new conversation", lastLogData["message"])
}

func TestLimiter_UpdateConfig(t *testing.T) {
	logCalled := false
	logCallback := func(data map[string]interface{}) {
		logCalled = true
		assert.Equal(t, "Turn limiter configuration updated", data["message"])
	}
	
	config := LimiterConfig{MaxTurns: 10}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, logCallback)
	
	// Update configuration
	newConfig := LimiterConfig{
		MaxTurns: 15,
		Strategy: TurnStrategyAdaptive,
	}
	limiter.UpdateConfig(newConfig)
	
	assert.Equal(t, 15, limiter.Config.MaxTurns)
	assert.Equal(t, TurnStrategyAdaptive, limiter.Config.Strategy)
	assert.True(t, logCalled)
}

func TestLimiter_TurnLimitEnforcement(t *testing.T) {
	// This test verifies the core issue: maxTurns must be enforced
	config := LimiterConfig{
		MaxTurns: 3, // Very low for testing
		CriticalThreshold: 1.0, // Force completion exactly at limit
	}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Should be able to continue for turns 1 and 2
	assert.True(t, func() bool { can, _ := limiter.CanContinue(); return can }())
	limiter.IncrementTurn() // Turn 1
	
	assert.True(t, func() bool { can, _ := limiter.CanContinue(); return can }())
	limiter.IncrementTurn() // Turn 2
	
	assert.True(t, func() bool { can, _ := limiter.CanContinue(); return can }())
	limiter.IncrementTurn() // Turn 3
	
	// At turn 3 (the limit), should NOT be able to continue
	canContinue, reason := limiter.CanContinue()
	assert.False(t, canContinue)
	assert.Contains(t, reason, "Turn limit reached")
	assert.Equal(t, 3, limiter.currentTurns)
	assert.Equal(t, 3, limiter.Config.MaxTurns)
	
	// Should force completion at this point
	should, completionReason, description := limiter.ShouldForceCompletion(nil)
	assert.True(t, should)
	assert.Equal(t, CompletionReasonTurnLimit, completionReason)
	assert.Contains(t, description, "critical threshold")
}

// Benchmark test for turn analysis performance
func BenchmarkLimiter_AnalyzeTurnUsage(b *testing.B) {
	config := LimiterConfig{MaxTurns: 25}
	contextManager := &MockContextManager{}
	limiter := NewLimiter(config, contextManager, nil)
	
	// Create a large conversation
	messages := make([]*ai.Message, 50)
	for i := 0; i < 50; i++ {
		if i%2 == 0 {
			messages[i] = &ai.Message{
				Role: ai.RoleModel,
				Content: []*ai.Part{
					ai.NewToolRequestPart(&ai.ToolRequest{Name: "read_text_file"}),
				},
			}
		} else {
			messages[i] = &ai.Message{
				Role: ai.RoleTool,
				Content: []*ai.Part{
					ai.NewToolResponsePart(&ai.ToolResponse{Name: "read_text_file", Output: "content"}),
				},
			}
		}
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.AnalyzeTurnUsage(messages)
	}
}