package turns

import (
	"time"

	"github.com/firebase/genkit/go/ai"
)

// TurnStrategy represents different turn limiting strategies
type TurnStrategy string

const (
	TurnStrategyFixed     TurnStrategy = "fixed"      // Fixed turn limit
	TurnStrategyAdaptive  TurnStrategy = "adaptive"   // Adaptive based on context and complexity
	TurnStrategyProgressive TurnStrategy = "progressive" // Progressive tightening
)

// TurnMetrics represents metrics about turn usage
type TurnMetrics struct {
	CurrentTurns        int                 `json:"current_turns"`
	MaxTurns           int                 `json:"max_turns"`
	TurnsRemaining     int                 `json:"turns_remaining"`
	UtilizationPercent float64             `json:"utilization_percent"`
	Strategy           TurnStrategy        `json:"strategy"`
	LastTurnTime       time.Time           `json:"last_turn_time"`
	AverageTurnDuration time.Duration       `json:"average_turn_duration"`
}

// CompletionReason represents why a conversation was forced to complete
type CompletionReason string

const (
	CompletionReasonTurnLimit      CompletionReason = "turn_limit_reached"
	CompletionReasonContextLimit   CompletionReason = "context_limit_reached"
	CompletionReasonToolLimit      CompletionReason = "tool_limit_reached"
	CompletionReasonTimeLimit      CompletionReason = "time_limit_reached"
	CompletionReasonManual         CompletionReason = "manual_completion"
	CompletionReasonError          CompletionReason = "error_occurred"
)

// FinalizationRequest represents a request to finalize a conversation
type FinalizationRequest struct {
	Conversation   []*ai.Message    `json:"conversation"`
	Reason         CompletionReason `json:"reason"`
	PreserveLastN  int             `json:"preserve_last_n"`
	CreateSummary  bool            `json:"create_summary"`
	ForceFinal     bool            `json:"force_final"`
	ModelName      string          `json:"model_name"`
}

// FinalizationResponse represents the response from conversation finalization
type FinalizationResponse struct {
	FinalResponse  *ai.ModelResponse `json:"final_response"`
	Summary        string           `json:"summary"`
	Success        bool             `json:"success"`
	TokensUsed     int              `json:"tokens_used"`
	FinalMessage   *ai.Message      `json:"final_message"`
	Error          string           `json:"error,omitempty"`
}

// LimiterConfig represents configuration for the turn limiter
type LimiterConfig struct {
	MaxTurns           int             `json:"max_turns"`           // Default maximum turns
	Strategy           TurnStrategy     `json:"strategy"`            // Turn limiting strategy
	WarningThreshold   float64         `json:"warning_threshold"`   // When to warn (0.8 = 80%)
	CriticalThreshold  float64         `json:"critical_threshold"`  // When to force completion (0.9 = 90%)
	EnableAdaptive     bool            `json:"enable_adaptive"`     // Enable adaptive turn limits
	ContextAware       bool            `json:"context_aware"`       // Consider context utilization
	TaskComplexityAware bool           `json:"task_complexity_aware"` // Consider task complexity
}

// TaskComplexity represents the assessed complexity of a task
type TaskComplexity string

const (
	TaskComplexitySimple   TaskComplexity = "simple"
	TaskComplexityModerate TaskComplexity = "moderate"  
	TaskComplexityComplex  TaskComplexity = "complex"
	TaskComplexityVeryComplex TaskComplexity = "very_complex"
)

// TurnAnalysis represents analysis of turn usage patterns
type TurnAnalysis struct {
	ToolHeavy          bool            `json:"tool_heavy"`           // >50% tool usage
	ProgressStalling   bool            `json:"progress_stalling"`    // Repeated similar actions
	InformationGathering bool          `json:"information_gathering"` // Mostly read operations
	ExecutionPhase     bool            `json:"execution_phase"`      // Mostly write operations
	RecommendedAction  string          `json:"recommended_action"`
	RiskLevel          string          `json:"risk_level"`
	Efficiency         float64         `json:"efficiency"`           // 0.0-1.0 efficiency score
}

// LogCallback is a function type for progress logging
type LogCallback func(map[string]interface{})

// ContextManager interface to avoid circular imports
type ContextManager interface {
	GetUtilizationPercent() float64
	ShouldSummarize() (bool, string)
	IsApproachingLimit() bool
	GetSafeActionLimit() int
}