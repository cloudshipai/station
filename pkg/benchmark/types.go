package benchmark

import (
	"time"
)

// ============================================================================
// Core Types
// ============================================================================

// BenchmarkTask represents a concrete evaluation task with measurable criteria
type BenchmarkTask struct {
	ID              int64                  `json:"id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Category        string                 `json:"category"` // security, finops, compliance, devops
	SuccessCriteria map[string]interface{} `json:"success_criteria"`
	Weight          float64                `json:"weight"`
	EnvironmentID   *int64                 `json:"environment_id,omitempty"`
	IsActive        bool                   `json:"is_active"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// BenchmarkMetric represents a quality metric evaluation for a specific run
type BenchmarkMetric struct {
	ID                   int64     `json:"id"`
	RunID                int64     `json:"run_id"`
	MetricType           string    `json:"metric_type"` // hallucination, relevancy, etc.
	Score                float64   `json:"score"`       // 0.0 to 1.0
	Threshold            float64   `json:"threshold"`
	Passed               bool      `json:"passed"`
	Reason               string    `json:"reason,omitempty"`
	Verdicts             []Verdict `json:"verdicts,omitempty"`
	Evidence             Evidence  `json:"evidence,omitempty"`
	JudgeModel           string    `json:"judge_model"`
	JudgeTokens          int       `json:"judge_tokens"`
	JudgeCost            float64   `json:"judge_cost"`
	EvaluationDurationMS int       `json:"evaluation_duration_ms"`
	CreatedAt            time.Time `json:"created_at"`
}

// TaskEvaluation represents an agent's performance on a specific task
type TaskEvaluation struct {
	ID              int64                     `json:"id"`
	TaskID          int64                     `json:"task_id"`
	AgentID         int64                     `json:"agent_id"`
	ReportID        *int64                    `json:"report_id,omitempty"`
	TaskScore       float64                   `json:"task_score"` // 0-10
	Completed       bool                      `json:"completed"`
	CriteriaResults map[string]CriteriaResult `json:"criteria_results,omitempty"`

	// Quality metrics (averages)
	AvgHallucination  float64 `json:"avg_hallucination"`
	AvgRelevancy      float64 `json:"avg_relevancy"`
	AvgTaskCompletion float64 `json:"avg_task_completion"`
	AvgFaithfulness   float64 `json:"avg_faithfulness"`
	AvgToxicity       float64 `json:"avg_toxicity"`
	AvgBias           float64 `json:"avg_bias"`

	// Performance metrics
	RunsAnalyzed       int      `json:"runs_analyzed"`
	RunIDs             []int64  `json:"run_ids,omitempty"`
	TraceIDs           []string `json:"trace_ids,omitempty"`
	AvgDurationSeconds float64  `json:"avg_duration_seconds"`
	AvgTokens          int      `json:"avg_tokens"`
	AvgCost            float64  `json:"avg_cost"`
	ToolCallsCount     int      `json:"tool_calls_count"`

	// Analysis
	Strengths  []string `json:"strengths,omitempty"`
	Weaknesses []string `json:"weaknesses,omitempty"`

	// Rankings
	Rank       int  `json:"rank"`
	IsChampion bool `json:"is_champion"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProductionReadiness represents overall deployment assessment
type ProductionReadiness struct {
	ID       int64 `json:"id"`
	ReportID int64 `json:"report_id"`

	// Overall scores
	TaskCompletionScore      float64 `json:"task_completion_score"`      // 0-10
	QualityScore             float64 `json:"quality_score"`              // 0-10
	ProductionReadinessScore float64 `json:"production_readiness_score"` // 0-100

	// Quality metrics summary
	AvgHallucination  float64 `json:"avg_hallucination"`
	AvgRelevancy      float64 `json:"avg_relevancy"`
	AvgTaskCompletion float64 `json:"avg_task_completion"`
	AvgFaithfulness   float64 `json:"avg_faithfulness"`
	AvgToxicity       float64 `json:"avg_toxicity"`
	AvgBias           float64 `json:"avg_bias"`

	// Pass rates
	HallucinationPassRate  float64 `json:"hallucination_pass_rate"`
	RelevancyPassRate      float64 `json:"relevancy_pass_rate"`
	TaskCompletionPassRate float64 `json:"task_completion_pass_rate"`
	FaithfulnessPassRate   float64 `json:"faithfulness_pass_rate"`
	ToxicityPassRate       float64 `json:"toxicity_pass_rate"`
	BiasPassRate           float64 `json:"bias_pass_rate"`

	// Deployment decision
	Recommendation string `json:"recommendation"` // PRODUCTION_READY, CONDITIONAL_GO, etc.
	RiskLevel      string `json:"risk_level"`     // LOW, MEDIUM, HIGH, CRITICAL

	RequiredActions       []string                 `json:"required_actions,omitempty"`
	Blockers              []string                 `json:"blockers,omitempty"`
	ChampionAgents        map[string]ChampionAgent `json:"champion_agents,omitempty"`
	UnderperformingAgents []UnderperformingAgent   `json:"underperforming_agents,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ============================================================================
// Supporting Types
// ============================================================================

// Verdict represents an individual LLM judgment
type Verdict struct {
	Statement string `json:"statement,omitempty"`
	Verdict   string `json:"verdict"` // "yes", "no", "idk"
	Reason    string `json:"reason,omitempty"`
}

// Evidence represents links to actual execution data
type Evidence struct {
	ToolCallIDs []string `json:"tool_call_ids,omitempty"`
	TraceIDs    []string `json:"trace_ids,omitempty"`
	SpanIDs     []string `json:"span_ids,omitempty"`
	RunIDs      []int64  `json:"run_ids,omitempty"`
}

// CriteriaResult represents evaluation of a single success criterion
type CriteriaResult struct {
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Passed   bool        `json:"passed"`
	Reason   string      `json:"reason,omitempty"`
}

// ChampionAgent represents the best performer for a task/category
type ChampionAgent struct {
	AgentID   int64   `json:"agent_id"`
	AgentName string  `json:"agent_name"`
	Score     float64 `json:"score"`
	TaskID    int64   `json:"task_id,omitempty"`
}

// UnderperformingAgent represents agents that need improvement or retirement
type UnderperformingAgent struct {
	AgentID   int64   `json:"agent_id"`
	AgentName string  `json:"agent_name"`
	Score     float64 `json:"score"`
	Action    string  `json:"action"` // IMPROVE, RETIRE, REPLACE
	Reason    string  `json:"reason"`
}

// ============================================================================
// Evaluation Input/Output Types
// ============================================================================

// EvaluationInput contains all data needed for metric evaluation
type EvaluationInput struct {
	RunID            int64      `json:"run_id"`
	AgentID          int64      `json:"agent_id"`
	AgentName        string     `json:"agent_name,omitempty"`        // For context in evaluations
	AgentDescription string     `json:"agent_description,omitempty"` // Agent's purpose for context
	Task             string     `json:"task"`
	FinalResponse    string     `json:"final_response"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ExecutionSteps   []string   `json:"execution_steps,omitempty"`
	TraceID          string     `json:"trace_id,omitempty"`
	Duration         float64    `json:"duration"`
	Tokens           int        `json:"tokens"`
	Cost             float64    `json:"cost"`
	Status           string     `json:"status"`
	Contexts         []string   `json:"contexts,omitempty"` // Extracted from tool outputs
}

// ToolCall represents a tool invocation with inputs and outputs
type ToolCall struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Output     interface{}            `json:"output,omitempty"`
	Timestamp  string                 `json:"timestamp,omitempty"`
	Duration   float64                `json:"duration,omitempty"`
}

// MetricResult represents the result of a single metric evaluation
type MetricResult struct {
	MetricType           string    `json:"metric_type"`
	Score                float64   `json:"score"`
	Threshold            float64   `json:"threshold"`
	Passed               bool      `json:"passed"`
	Reason               string    `json:"reason,omitempty"`
	Verdicts             []Verdict `json:"verdicts,omitempty"`
	Evidence             Evidence  `json:"evidence,omitempty"`
	JudgeTokens          int       `json:"judge_tokens"`
	JudgeCost            float64   `json:"judge_cost"`
	EvaluationDurationMS int       `json:"evaluation_duration_ms"`
}

// BenchmarkResult represents complete benchmark evaluation for a run
type BenchmarkResult struct {
	RunID   int64  `json:"run_id"`
	AgentID int64  `json:"agent_id"`
	Task    string `json:"task"`

	// Individual metric results
	Metrics map[string]MetricResult `json:"metrics"`

	// Aggregate scores
	QualityScore    float64 `json:"quality_score"` // 0-10
	ProductionReady bool    `json:"production_ready"`
	Recommendation  string  `json:"recommendation"`

	// Metadata
	TotalJudgeTokens int     `json:"total_judge_tokens"`
	TotalJudgeCost   float64 `json:"total_judge_cost"`
	EvaluationTimeMS int     `json:"evaluation_time_ms"`
}

// ============================================================================
// Constants
// ============================================================================

const (
	// Metric Types
	MetricHallucination  = "hallucination"
	MetricRelevancy      = "relevancy"
	MetricTaskCompletion = "task_completion"
	MetricFaithfulness   = "faithfulness"
	MetricToxicity       = "toxicity"
	MetricBias           = "bias"

	// Recommendations
	RecommendationProductionReady  = "PRODUCTION_READY"
	RecommendationConditionalGo    = "CONDITIONAL_GO"
	RecommendationNeedsImprovement = "NEEDS_IMPROVEMENT"
	RecommendationNotReady         = "NOT_READY"

	// Risk Levels
	RiskLevelLow      = "LOW"
	RiskLevelMedium   = "MEDIUM"
	RiskLevelHigh     = "HIGH"
	RiskLevelCritical = "CRITICAL"

	// Task Categories
	CategorySecurity   = "security"
	CategoryFinOps     = "finops"
	CategoryCompliance = "compliance"
	CategoryDevOps     = "devops"
	CategoryCustom     = "custom"

	// Agent Actions
	ActionImprove = "IMPROVE"
	ActionRetire  = "RETIRE"
	ActionReplace = "REPLACE"
)

// ============================================================================
// Default Thresholds
// ============================================================================

var DefaultThresholds = map[string]float64{
	MetricHallucination:  0.10, // < 10% hallucination tolerance
	MetricRelevancy:      0.80, // > 80% relevancy required
	MetricTaskCompletion: 0.70, // > 70% completion required (more realistic for partial completions)
	MetricFaithfulness:   0.85, // > 85% faithfulness required
	MetricToxicity:       0.05, // < 5% toxicity tolerance
	MetricBias:           0.05, // < 5% bias tolerance
}
