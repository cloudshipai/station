package types

import (
	"time"
)

// TestScenario represents a single test scenario for agent testing
type TestScenario struct {
	Task         string                 `json:"task"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	ScenarioType string                 `json:"scenario_type"`
	Description  string                 `json:"description,omitempty"`
}

// TestingConfig holds configuration for async testing pipeline
type TestingConfig struct {
	AgentID           int64  `json:"agent_id"`
	ScenarioCount     int    `json:"scenario_count"`
	MaxConcurrent     int    `json:"max_concurrent"`
	VariationStrategy string `json:"variation_strategy"`
	OutputDir         string `json:"output_dir"`
	JaegerURL         string `json:"jaeger_url"`
}

// TestingProgress tracks the progress of async testing pipeline
type TestingProgress struct {
	TaskID      string                 `json:"task_id"`
	AgentID     int64                  `json:"agent_id"`
	AgentName   string                 `json:"agent_name"`
	Status      string                 `json:"status"` // introspecting, generating_scenarios, executing, analyzing, completed, failed
	StartedAt   time.Time              `json:"started_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Phases      map[string]PhaseStatus `json:"phases"`
	OutputFiles OutputFiles            `json:"output_files"`
}

// PhaseStatus tracks the status of a single phase
type PhaseStatus struct {
	Status          string                 `json:"status"` // pending, in_progress, completed, failed
	StartedAt       *time.Time             `json:"started_at,omitempty"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	DurationSeconds float64                `json:"duration_seconds"`
	Details         map[string]interface{} `json:"details,omitempty"`
}

// OutputFiles tracks generated output files
type OutputFiles struct {
	Scenarios string `json:"scenarios,omitempty"`
	Dataset   string `json:"dataset,omitempty"`
	Analysis  string `json:"analysis,omitempty"`
	Report    string `json:"report,omitempty"`
	Progress  string `json:"progress,omitempty"`
}

// AgentContext holds complete agent context for scenario generation
type AgentContext struct {
	ID              int64                  `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Prompt          string                 `json:"prompt"`
	MaxSteps        int64                  `json:"max_steps"`
	InputSchema     map[string]interface{} `json:"input_schema,omitempty"`
	Tools           []string               `json:"tools"`
	EnvironmentID   int64                  `json:"environment_id"`
	EnvironmentName string                 `json:"environment_name,omitempty"`
}

// EnrichedRun combines database run data with Jaeger trace data
type EnrichedRun struct {
	// Database fields
	RunID     int64                  `json:"run_id"`
	AgentID   int64                  `json:"agent_id"`
	AgentName string                 `json:"agent_name"`
	Task      string                 `json:"task"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	Response  string                 `json:"response"`
	Status    string                 `json:"status"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`

	// Timing
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	DurationSeconds float64    `json:"duration_seconds"`

	// Execution metrics
	StepsTaken   int64  `json:"steps_taken"`
	InputTokens  *int64 `json:"input_tokens,omitempty"`
	OutputTokens *int64 `json:"output_tokens,omitempty"`
	TotalTokens  *int64 `json:"total_tokens,omitempty"`
	ModelName    string `json:"model_name,omitempty"`
	ToolsUsed    *int64 `json:"tools_used,omitempty"`

	// Detailed execution data from database
	ToolCalls      interface{} `json:"tool_calls,omitempty"`      // Tool calls made during execution
	ExecutionSteps interface{} `json:"execution_steps,omitempty"` // Step-by-step execution log
	DebugLogs      interface{} `json:"debug_logs,omitempty"`      // Debug logs from execution

	// Jaeger trace data
	Trace *CompleteTrace `json:"trace,omitempty"`
}

// CompleteTrace holds complete trace data from Jaeger
type CompleteTrace struct {
	TraceID           string           `json:"trace_id"`
	TotalSpans        int              `json:"total_spans"`
	ExecutionTimeline []ExecutionPhase `json:"execution_timeline,omitempty"`
	ToolCallSequence  []ToolCallTrace  `json:"tool_call_sequence"`
	TimingBreakdown   *TimingBreakdown `json:"timing_breakdown"`
	ToolStatistics    *ToolStatistics  `json:"tool_statistics,omitempty"`
}

// ToolCallTrace represents a tool call extracted from trace
type ToolCallTrace struct {
	Step       int                    `json:"step"`
	Tool       string                 `json:"tool"`
	SpanID     string                 `json:"span_id"`
	StartTime  time.Time              `json:"start_time"`
	DurationMs float64                `json:"duration_ms"`
	Success    bool                   `json:"success"`
	Input      map[string]interface{} `json:"input,omitempty"`
	Output     string                 `json:"output,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// TimingBreakdown represents timing breakdown across execution phases
type TimingBreakdown struct {
	TotalMs     float64 `json:"total_ms"`
	SetupMs     float64 `json:"setup_ms"`     // MCP server start, DB queries
	ExecutionMs float64 `json:"execution_ms"` // dotprompt.execute
	ToolsMs     float64 `json:"tools_ms"`     // Sum of all tool calls
	ReasoningMs float64 `json:"reasoning_ms"` // Execution - tools
	CleanupMs   float64 `json:"cleanup_ms"`   // Final DB updates
}

// ExecutionPhase represents a phase in agent execution
type ExecutionPhase struct {
	Phase      string                 `json:"phase"` // setup, execution, cleanup
	Operation  string                 `json:"operation"`
	StartTime  time.Time              `json:"start_time"`
	DurationMs float64                `json:"duration_ms"`
	Children   []ChildOperation       `json:"children,omitempty"`
	Tags       map[string]interface{} `json:"tags,omitempty"`
}

// ChildOperation represents a child operation in an execution phase
type ChildOperation struct {
	Operation  string                 `json:"operation"`
	DurationMs float64                `json:"duration_ms"`
	Order      int                    `json:"order"`
	Input      map[string]interface{} `json:"input,omitempty"`
	Output     string                 `json:"output,omitempty"`
	Success    bool                   `json:"success"`
	Error      string                 `json:"error,omitempty"`
	Tags       map[string]interface{} `json:"tags,omitempty"`
}

// ToolStatistics holds statistics about tool usage
type ToolStatistics struct {
	TotalToolCalls        int     `json:"total_tool_calls"`
	UniqueTools           int     `json:"unique_tools"`
	AvgToolDurationMs     float64 `json:"avg_tool_duration_ms"`
	LongestTool           string  `json:"longest_tool,omitempty"`
	LongestToolDurationMs float64 `json:"longest_tool_duration_ms"`
	ToolOverheadPercent   float64 `json:"tool_overhead_percent"` // Tool time / total time * 100
}

// ComprehensiveDataset holds the complete dataset with all runs and analysis
type ComprehensiveDataset struct {
	Metadata DatasetMetadata        `json:"metadata"`
	Runs     []EnrichedRun          `json:"runs"`
	Analysis *ComprehensiveAnalysis `json:"analysis,omitempty"`
}

// DatasetMetadata holds metadata about the dataset
type DatasetMetadata struct {
	AgentID         int64     `json:"agent_id"`
	AgentName       string    `json:"agent_name"`
	GeneratedAt     time.Time `json:"generated_at"`
	TotalRuns       int       `json:"total_runs"`
	ScenarioCount   int       `json:"scenario_count"`
	FilterModel     string    `json:"filter_model,omitempty"`
	JaegerAvailable bool      `json:"jaeger_available"`
	TracesCaptured  int       `json:"traces_captured"`
}

// ComprehensiveAnalysis holds multi-dimensional analysis results
type ComprehensiveAnalysis struct {
	Performance *PerformanceMetrics `json:"performance,omitempty"`
	ToolUsage   *ToolUsageMetrics   `json:"tool_usage,omitempty"`
	Quality     *QualityMetrics     `json:"quality,omitempty"`
	Patterns    *BehaviorPatterns   `json:"patterns,omitempty"`
}

// PerformanceMetrics holds performance analysis
type PerformanceMetrics struct {
	AvgTotalDuration    float64 `json:"avg_total_duration_ms"`
	AvgSetupTime        float64 `json:"avg_setup_time_ms"`
	AvgExecutionTime    float64 `json:"avg_execution_time_ms"`
	AvgToolTime         float64 `json:"avg_tool_time_ms"`
	AvgReasoningTime    float64 `json:"avg_reasoning_time_ms"`
	P50Duration         float64 `json:"p50_duration_ms"`
	P95Duration         float64 `json:"p95_duration_ms"`
	P99Duration         float64 `json:"p99_duration_ms"`
	ToolOverheadPercent float64 `json:"tool_overhead_percent"`
}

// ToolUsageMetrics holds tool usage analysis
type ToolUsageMetrics struct {
	TotalToolCalls  int                        `json:"total_tool_calls"`
	UniqueTools     int                        `json:"unique_tools"`
	ToolFrequency   map[string]int             `json:"tool_frequency"` // tool name -> count
	ToolPerformance map[string]ToolPerfMetrics `json:"tool_performance"`
	AvgToolsPerRun  float64                    `json:"avg_tools_per_run"`
	CommonSequences []ToolSequencePattern      `json:"common_sequences,omitempty"`
}

// ToolPerfMetrics holds performance metrics for a specific tool
type ToolPerfMetrics struct {
	AvgDuration  float64 `json:"avg_duration_ms"`
	P95Duration  float64 `json:"p95_duration_ms"`
	SuccessRate  float64 `json:"success_rate"`
	UsageCount   int     `json:"usage_count"`
	FailureCount int     `json:"failure_count"`
}

// ToolSequencePattern represents a common tool usage sequence
type ToolSequencePattern struct {
	Sequence    []string `json:"sequence"`
	Frequency   int      `json:"frequency"`
	AvgDuration float64  `json:"avg_duration_ms"`
	SuccessRate float64  `json:"success_rate"`
}

// QualityMetrics holds quality analysis
type QualityMetrics struct {
	SuccessRate       float64        `json:"success_rate"`
	AvgResponseLength int            `json:"avg_response_length"`
	AvgStepsUsed      float64        `json:"avg_steps_used"`
	StepEfficiency    float64        `json:"step_efficiency"` // steps_used / max_steps
	CompleteResponses float64        `json:"complete_responses_percent"`
	ErrorPatterns     []ErrorPattern `json:"error_patterns,omitempty"`
}

// ErrorPattern represents a common error pattern
type ErrorPattern struct {
	Error        string  `json:"error"`
	Frequency    int     `json:"frequency"`
	Phase        string  `json:"phase,omitempty"` // setup, execution, tool_call
	AffectedRuns []int64 `json:"affected_runs,omitempty"`
}

// BehaviorPatterns holds behavior pattern analysis
type BehaviorPatterns struct {
	CommonApproaches []Approach     `json:"common_approaches,omitempty"`
	DecisionPoints   []Decision     `json:"decision_points,omitempty"`
	FailurePoints    []FailurePoint `json:"failure_points,omitempty"`
}

// Approach represents a common problem-solving approach
type Approach struct {
	Description  string   `json:"description"`
	Frequency    int      `json:"frequency"`
	ToolSequence []string `json:"tool_sequence"`
	AvgDuration  float64  `json:"avg_duration_ms"`
	SuccessRate  float64  `json:"success_rate"`
}

// Decision represents a decision point in agent behavior
type Decision struct {
	Context   string `json:"context"`
	Choice    string `json:"choice"`
	Frequency int    `json:"frequency"`
	Outcome   string `json:"outcome,omitempty"`
}

// FailurePoint represents a common failure point
type FailurePoint struct {
	Phase       string  `json:"phase"` // setup, tool_call, reasoning
	Frequency   int     `json:"frequency"`
	CommonCause string  `json:"common_cause"`
	Examples    []int64 `json:"example_run_ids,omitempty"`
}
