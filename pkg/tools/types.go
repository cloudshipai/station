package tools

import (
	"context"
	"time"

	stationctx "station/pkg/context"

	"github.com/firebase/genkit/go/ai"
)

// ExecutionResult represents the result of a tool execution
type ExecutionResult struct {
	ToolName        string                 `json:"tool_name"`
	Success         bool                   `json:"success"`
	Output          string                 `json:"output"`
	TokensUsed      int                    `json:"tokens_used"`
	Duration        time.Duration          `json:"duration"`
	Error           string                 `json:"error,omitempty"`
	Truncated       bool                   `json:"truncated"`
	OriginalLength  int                    `json:"original_length,omitempty"`
	ExecutionID     string                 `json:"execution_id"`
	Timestamp       time.Time              `json:"timestamp"`
}

// ExecutionContext provides context about the current execution state
type ExecutionContext struct {
	ConversationID    string                 `json:"conversation_id"`
	TurnNumber        int                    `json:"turn_number"`
	RemainingTokens   int                    `json:"remaining_tokens"`
	ContextThreshold  float64                `json:"context_threshold"`
	MaxOutputTokens   int                    `json:"max_output_tokens"`
	SafeActionLimit   int                    `json:"safe_action_limit"`
	PreviousToolCalls []string               `json:"previous_tool_calls"`
	MetricData        map[string]interface{} `json:"metric_data,omitempty"`
}

// ToolExecutor interface defines the contract for executing tools with context protection
type ToolExecutor interface {
	// ExecuteTool executes a single tool with context protection
	ExecuteTool(ctx context.Context, toolCall *ai.ToolRequest, execCtx *ExecutionContext) (*ExecutionResult, error)
	
	// ExecuteToolBatch executes multiple tools with shared context management
	ExecuteToolBatch(ctx context.Context, toolCalls []*ai.ToolRequest, execCtx *ExecutionContext) ([]*ExecutionResult, error)
	
	// CanExecuteTool checks if a tool can be safely executed given current context
	CanExecuteTool(toolCall *ai.ToolRequest, execCtx *ExecutionContext) (bool, string)
	
	// EstimateToolOutputTokens estimates the token usage of a tool execution
	EstimateToolOutputTokens(toolCall *ai.ToolRequest) (int, error)
	
	// GetToolMetrics returns execution metrics for analysis
	GetToolMetrics() *ToolMetrics
}

// ToolMetrics represents metrics about tool execution performance and context usage
type ToolMetrics struct {
	TotalExecutions     int                    `json:"total_executions"`
	SuccessfulExecutions int                   `json:"successful_executions"`
	FailedExecutions    int                    `json:"failed_executions"`
	TruncatedExecutions int                    `json:"truncated_executions"`
	AverageTokenUsage   float64                `json:"average_token_usage"`
	AverageDuration     time.Duration          `json:"average_duration"`
	ContextOverflows    int                    `json:"context_overflows"`
	TokensSaved         int                    `json:"tokens_saved_by_truncation"`
	ToolTypeBreakdown   map[string]int         `json:"tool_type_breakdown"`
	LastExecutionTime   time.Time              `json:"last_execution_time"`
}

// TruncationStrategy defines how to handle large tool outputs
type TruncationStrategy string

const (
	TruncationStrategyNone     TruncationStrategy = "none"      // Never truncate
	TruncationStrategyHead     TruncationStrategy = "head"      // Keep first N characters
	TruncationStrategyTail     TruncationStrategy = "tail"      // Keep last N characters 
	TruncationStrategyHeadTail TruncationStrategy = "head_tail" // Keep first N/2 and last N/2 characters
	TruncationStrategySummary  TruncationStrategy = "summary"   // Replace with summary when possible
	TruncationStrategyIntelligent TruncationStrategy = "intelligent" // Smart truncation based on content type
)

// ToolExecutorConfig configures the tool executor behavior
type ToolExecutorConfig struct {
	MaxOutputTokens       int                `json:"max_output_tokens"`       // Default max tokens per tool output
	TruncationStrategy    TruncationStrategy `json:"truncation_strategy"`     // How to handle large outputs
	EnableContextProtection bool             `json:"enable_context_protection"` // Enable context overflow protection
	ContextBuffer         int                `json:"context_buffer"`          // Reserved tokens for response generation
	MaxConcurrentTools    int                `json:"max_concurrent_tools"`    // Max tools to execute concurrently
	ToolTimeout           time.Duration      `json:"tool_timeout"`            // Timeout per tool execution
	RetryFailedTools      bool               `json:"retry_failed_tools"`      // Retry failed tool executions
	MaxRetries            int                `json:"max_retries"`             // Max retry attempts
	EnableMetrics         bool               `json:"enable_metrics"`          // Enable execution metrics tracking
}

// ContextManager interface to avoid circular imports
type ContextManager interface {
	GetUtilizationPercent() float64
	GetTokensRemaining() int
	CanExecuteAction(estimatedTokens int) (bool, string)
	GetSafeActionLimit() int
	TrackTokenUsage(usage stationctx.TokenUsage)
	IsApproachingLimit() bool
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// LogCallback is a function type for logging tool execution events
type LogCallback func(map[string]interface{})

// ToolCallID generates a unique tool call ID for tracking
type ToolCallID string

// ToolOutputProcessor handles different types of tool output processing
type ToolOutputProcessor interface {
	// ProcessOutput processes raw tool output based on content type and context constraints
	ProcessOutput(output string, toolName string, maxTokens int, strategy TruncationStrategy) (*ProcessedOutput, error)
	
	// EstimateTokens estimates token count for given output
	EstimateTokens(output string) int
	
	// SupportedContentTypes returns content types this processor can handle
	SupportedContentTypes() []string
}

// ProcessedOutput represents processed tool output with metadata
type ProcessedOutput struct {
	Content         string `json:"content"`
	TokenCount      int    `json:"token_count"`
	Truncated       bool   `json:"truncated"`
	OriginalLength  int    `json:"original_length"`
	ProcessingNotes string `json:"processing_notes,omitempty"`
	ContentType     string `json:"content_type"`
}