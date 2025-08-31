package context

import (
	"time"
)

// TokenUsage represents token usage statistics for a generation request
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ConversationMetrics represents metrics for a conversation
type ConversationMetrics struct {
	TaskType            string            `json:"task_type"`
	InitialComplexity   float64           `json:"initial_complexity"`
	ToolsUsed           []string          `json:"tools_used"`
	TurnsRequired       int               `json:"turns_required"`
	FinalTokenUsage     int               `json:"final_token_usage"`
	SummarizationCount  int               `json:"summarization_count"`
	CompletionSuccess   bool              `json:"completion_success"`
	Timestamp           time.Time         `json:"timestamp"`
	ConversationLength  int               `json:"conversation_length"`
}

// ConversationSummary represents a summarized conversation
type ConversationSummary struct {
	Summary          string                 `json:"summary"`
	OriginalMessages int                    `json:"original_messages"`
	CompressedTokens int                    `json:"compressed_tokens"`
	PreservePastIndex int                   `json:"preserve_past_index"`
	CreatedAt        time.Time              `json:"created_at"`
	Metadata         map[string]interface{} `json:"metadata"`
}

// EfficiencyMetrics represents token efficiency analysis
type EfficiencyMetrics struct {
	TokensPerTurn       float64 `json:"tokens_per_turn"`
	ToolTokenRatio      float64 `json:"tool_token_ratio"`
	ContextUtilization  float64 `json:"context_utilization"`
	WastedTokens        int     `json:"wasted_tokens"`
	EfficiencyScore     float64 `json:"efficiency_score"`
}

// ContextStatus represents the current context state
type ContextStatus struct {
	CanExecuteAction    bool   `json:"can_execute_action"`
	ShouldSummarize     bool   `json:"should_summarize"`
	UtilizationPercent  float64 `json:"utilization_percent"`
	TokensRemaining     int    `json:"tokens_remaining"`
	RecommendedAction   string `json:"recommended_action"`
}

// ToolResponseEstimate represents predicted tool response characteristics
type ToolResponseEstimate struct {
	EstimatedTokens int    `json:"estimated_tokens"`
	Confidence      float64 `json:"confidence"`
	MaxPossibleSize int    `json:"max_possible_size"`
	RiskLevel       string `json:"risk_level"` // "low", "medium", "high", "critical"
}

// CompressorConfig represents configuration for conversation compression
type CompressorConfig struct {
	SummaryModel        string  `json:"summary_model"`
	MaxSummaryTokens    int     `json:"max_summary_tokens"`
	PreserveLastN       int     `json:"preserve_last_n"`
	CompressionRatio    float64 `json:"compression_ratio"`
	QualityThreshold    float64 `json:"quality_threshold"`
}

// RecoveryOptions represents options for context overflow recovery
type RecoveryOptions struct {
	Strategy           RecoveryStrategy `json:"strategy"`
	PreserveMessages   int             `json:"preserve_messages"`
	CreateSummary      bool            `json:"create_summary"`
	ForceFinalResponse bool            `json:"force_final_response"`
}

// RecoveryStrategy represents different recovery strategies
type RecoveryStrategy string

const (
	RecoveryStrategyAggressiveSummary RecoveryStrategy = "aggressive_summary"
	RecoveryStrategyProgressiveTrim   RecoveryStrategy = "progressive_trim"
	RecoveryStrategyStartFresh        RecoveryStrategy = "start_fresh"
	RecoveryStrategyFinalResponse     RecoveryStrategy = "final_response"
)

// LogCallback is a function type for progress logging
type LogCallback func(map[string]interface{})