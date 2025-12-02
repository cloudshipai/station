package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"station/internal/config"
	"station/internal/db/queries"
	"station/internal/db/repositories"
	"station/internal/logging"
	"station/pkg/benchmark"
	"station/pkg/models"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// ReportGenerator handles report generation with LLM-as-judge evaluation
type ReportGenerator struct {
	repos              *repositories.Repositories
	genkitProvider     *GenKitProvider
	judgeModel         string
	maxConcurrentEvals int
	db                 *sql.DB // Database connection for benchmark evaluations
}

// ReportGeneratorConfig configures the report generator
type ReportGeneratorConfig struct {
	JudgeModel         string // Default: "gpt-4o-mini"
	MaxConcurrentEvals int    // Default: 10
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(repos *repositories.Repositories, db *sql.DB, cfg *ReportGeneratorConfig) *ReportGenerator {
	if cfg == nil {
		cfg = &ReportGeneratorConfig{
			JudgeModel:         "",
			MaxConcurrentEvals: 10,
		}
	}

	// Use Station's global AI configuration for judge model if not explicitly provided
	if cfg.JudgeModel == "" {
		stationCfg, err := config.Load()
		if err == nil && stationCfg.AIModel != "" {
			cfg.JudgeModel = stationCfg.AIModel
			logging.Info("Using Station AI model for LLM judge: %s", cfg.JudgeModel)
		} else {
			// Fallback only if config loading fails
			cfg.JudgeModel = "gpt-4o-mini"
			logging.Info("Failed to load Station config for LLM judge, using fallback: gpt-4o-mini")
		}
	}

	if cfg.MaxConcurrentEvals == 0 {
		cfg.MaxConcurrentEvals = 10
	}

	return &ReportGenerator{
		repos:              repos,
		genkitProvider:     NewGenKitProvider(),
		judgeModel:         cfg.JudgeModel,
		maxConcurrentEvals: cfg.MaxConcurrentEvals,
		db:                 db,
	}
}

// TeamCriteria represents the team-level evaluation criteria
type TeamCriteria struct {
	Goal     string                         `json:"goal"`
	Criteria map[string]EvaluationCriterion `json:"criteria"`
}

// EvaluationCriterion represents a single evaluation criterion
type EvaluationCriterion struct {
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
	Threshold   float64 `json:"threshold"`
}

// CriterionScore represents the score for a single criterion
type CriterionScore struct {
	Score     float64  `json:"score"`
	Reasoning string   `json:"reasoning"`
	Examples  []string `json:"examples,omitempty"`
}

// TeamEvaluation represents the team-level evaluation result
type TeamEvaluation struct {
	Score          float64                   `json:"score"`
	Reasoning      string                    `json:"reasoning"`
	Summary        string                    `json:"summary"`
	CriteriaScores map[string]CriterionScore `json:"criteria_scores"`
	CostAnalysis   *TeamCostAnalysis         `json:"cost_analysis,omitempty"`
}

// RunExample represents a detailed example of a single run
type RunExample struct {
	RunID       int64    `json:"run_id"`
	Input       string   `json:"input"`
	Output      string   `json:"output"`
	ToolCalls   []string `json:"tool_calls"`
	Duration    float64  `json:"duration"`
	TokenCount  int64    `json:"token_count"`
	Status      string   `json:"status"`
	Explanation string   `json:"explanation"` // Why this run succeeded/failed
}

// ToolUsageStats represents usage statistics for a single tool
type ToolUsageStats struct {
	ToolName    string  `json:"tool_name"`
	UseCount    int     `json:"use_count"`
	SuccessRate float64 `json:"success_rate"`
	AvgDuration float64 `json:"avg_duration"`
}

// FailurePattern represents a recurring failure pattern
type FailurePattern struct {
	Pattern   string   `json:"pattern"`
	Frequency int      `json:"frequency"`
	Examples  []string `json:"examples"` // Run IDs
	Impact    string   `json:"impact"`   // High/Medium/Low
}

// ImprovementAction represents a specific, actionable improvement
type ImprovementAction struct {
	Issue           string `json:"issue"`
	Recommendation  string `json:"recommendation"`
	Priority        string `json:"priority"`         // High/Medium/Low
	ExpectedImpact  string `json:"expected_impact"`  // e.g., "+15% success rate"
	ConcreteExample string `json:"concrete_example"` // Actual run showing the issue
}

// CostProjection represents cost estimates at different execution frequencies
type CostProjection struct {
	Frequency       string  `json:"frequency"`         // "every_5_minutes", "hourly", "daily", "weekly", "monthly"
	RunsPerPeriod   int     `json:"runs_per_period"`   // How many runs in the period
	CostPerRun      float64 `json:"cost_per_run"`      // Average cost per agent run
	TotalCost       float64 `json:"total_cost"`        // Total cost for the period
	TokensPerPeriod int64   `json:"tokens_per_period"` // Total tokens consumed
}

// TeamCostAnalysis represents comprehensive cost analysis for the entire team
type TeamCostAnalysis struct {
	CurrentAvgCostPerRun   float64 `json:"current_avg_cost_per_run"`
	CurrentAvgTokensPerRun int64   `json:"current_avg_tokens_per_run"`
	CurrentAvgDuration     float64 `json:"current_avg_duration"`
	TotalTeamAgents        int     `json:"total_team_agents"`

	// Per-agent breakdown
	AgentCosts         map[string]float64 `json:"agent_costs"` // agent_name -> avg_cost
	MostExpensiveAgent string             `json:"most_expensive_agent"`
	MostEfficientAgent string             `json:"most_efficient_agent"` // Best cost/performance ratio

	// Projections at different frequencies
	Projections []CostProjection `json:"projections"`

	// ROI and value analysis
	EstimatedValuePerRun float64 `json:"estimated_value_per_run"` // If calculable from criteria
	ROIRatio             float64 `json:"roi_ratio"`               // Value / Cost
}

// AgentEvaluation represents the agent-level evaluation result
type AgentEvaluation struct {
	AgentID         int64
	AgentName       string
	Score           float64
	Passed          bool
	Reasoning       string
	CriteriaScores  map[string]CriterionScore
	Strengths       []string
	Weaknesses      []string
	Recommendations []string
	RunsAnalyzed    int
	RunIDs          []int64
	AvgDuration     float64
	AvgTokens       int64
	AvgCost         float64
	SuccessRate     float64

	// Enhanced enterprise data
	BestRunExample    *RunExample         `json:"best_run_example,omitempty"`
	WorstRunExample   *RunExample         `json:"worst_run_example,omitempty"`
	ToolUsageAnalysis []ToolUsageStats    `json:"tool_usage_analysis,omitempty"`
	FailurePatterns   []FailurePattern    `json:"failure_patterns,omitempty"`
	ImprovementPlan   []ImprovementAction `json:"improvement_plan,omitempty"`
	QualityMetrics    *QualityMetrics     `json:"quality_metrics,omitempty"`

	Error error
}

// GenerateReport generates a complete report with team and agent evaluations
func (rg *ReportGenerator) GenerateReport(ctx context.Context, reportID int64) error {
	logging.Info("Starting report generation for report ID: %d", reportID)
	startTime := time.Now()

	// 1. Load report from database
	report, err := rg.repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		return fmt.Errorf("failed to load report: %w", err)
	}

	// Set generation started time
	if err := rg.repos.Reports.SetGenerationStarted(ctx, reportID); err != nil {
		logging.Info("Failed to set generation started time: %v", err)
	}

	// 2. Parse team criteria
	var teamCriteria TeamCriteria
	if err := json.Unmarshal([]byte(report.TeamCriteria), &teamCriteria); err != nil {
		return rg.failReport(ctx, reportID, fmt.Errorf("failed to parse team criteria: %w", err))
	}

	// 3. Get agents from environment
	agents, err := rg.repos.Agents.GetByEnvironment(ctx, report.EnvironmentID)
	if err != nil {
		return rg.failReport(ctx, reportID, fmt.Errorf("failed to get agents: %w", err))
	}

	if len(agents) == 0 {
		return rg.failReport(ctx, reportID, fmt.Errorf("no agents found in environment"))
	}

	// 4. Update status: generating_team
	if err := rg.updateReportStatus(ctx, reportID, "generating_team", 10, "Evaluating team performance..."); err != nil {
		logging.Info("Failed to update report status: %v", err)
	}

	// 5. Fetch all runs for all agents (with optional model filtering)
	filterModel := ""
	if report.FilterModel.Valid && report.FilterModel.String != "" {
		filterModel = report.FilterModel.String
		logging.Info("Report configured to filter runs by model: %s", filterModel)
	}

	allRuns, err := rg.fetchAllRuns(ctx, agents, filterModel)
	if err != nil {
		return rg.failReport(ctx, reportID, fmt.Errorf("failed to fetch runs: %w", err))
	}

	if len(allRuns) == 0 {
		if filterModel != "" {
			return rg.failReport(ctx, reportID, fmt.Errorf("no runs found to analyze for model: %s", filterModel))
		}
		return rg.failReport(ctx, reportID, fmt.Errorf("no runs found to analyze"))
	}

	// 5.5. Run benchmark evaluations on all runs to populate individual quality metrics
	if err := rg.updateReportStatus(ctx, reportID, "evaluating_benchmarks", 15, "Running quality benchmarks on all runs..."); err != nil {
		logging.Info("Failed to update report status: %v", err)
	}

	if err := rg.evaluateBenchmarksForRuns(ctx, allRuns); err != nil {
		// Don't fail the report - just log the error and continue
		logging.Info("Warning: benchmark evaluation failed (continuing with report): %v", err)
	}

	// 6. Parse agent criteria (optional) - moved before agent evaluation
	agentCriteria := make(map[int64]map[string]EvaluationCriterion)
	if report.AgentCriteria.Valid && report.AgentCriteria.String != "" {
		var rawAgentCriteria map[string]map[string]EvaluationCriterion
		if err := json.Unmarshal([]byte(report.AgentCriteria.String), &rawAgentCriteria); err != nil {
			logging.Info("Failed to parse agent criteria, using default: %v", err)
		} else {
			for agentIDStr, criteria := range rawAgentCriteria {
				var agentID int64
				fmt.Sscanf(agentIDStr, "%d", &agentID)
				agentCriteria[agentID] = criteria
			}
		}
	}

	// 7. Update status: generating_agents (evaluate agents FIRST to get scores for team calc)
	if err := rg.updateReportStatus(ctx, reportID, "generating_agents", 30, "Evaluating agents..."); err != nil {
		logging.Info("Failed to update report status: %v", err)
	}

	// 8. Evaluate agents in parallel FIRST (needed for deterministic team score)
	agentEvals := rg.evaluateAgentsParallel(ctx, reportID, agents, agentCriteria)

	// 9. Calculate comprehensive team cost analysis
	costAnalysis := rg.calculateTeamCostAnalysis(agentEvals, agents)

	// 10. Update status: generating_team (now that we have agent scores)
	if err := rg.updateReportStatus(ctx, reportID, "generating_team", 85, "Calculating team performance..."); err != nil {
		logging.Info("Failed to update report status: %v", err)
	}

	// 11. Calculate team score deterministically from agent scores
	teamEval, err := rg.evaluateTeamPerformanceFromAgentScores(ctx, teamCriteria, agentEvals, agents)
	if err != nil {
		return rg.failReport(ctx, reportID, fmt.Errorf("failed to evaluate team: %w", err))
	}
	teamEval.CostAnalysis = costAnalysis

	// 12. Save team evaluation results
	criteriaScoresJSON, _ := json.Marshal(teamEval.CriteriaScores)
	if err := rg.repos.Reports.UpdateTeamResults(ctx, queries.UpdateReportTeamResultsParams{
		ID:                 reportID,
		ExecutiveSummary:   sql.NullString{String: teamEval.Summary, Valid: true},
		TeamScore:          sql.NullFloat64{Float64: teamEval.Score, Valid: true},
		TeamReasoning:      sql.NullString{String: teamEval.Reasoning, Valid: true},
		TeamCriteriaScores: sql.NullString{String: string(criteriaScoresJSON), Valid: true},
	}); err != nil {
		logging.Info("Failed to save team results: %v", err)
	}

	// Log cost analysis summary
	_ = costAnalysis // Use the cost analysis in teamEval
	logging.Info("Team cost analysis: Avg cost/run $%.4f, Monthly projection (daily runs): $%.2f",
		costAnalysis.CurrentAvgCostPerRun,
		costAnalysis.CurrentAvgCostPerRun*float64(len(agents))*30)

	// 13. Save agent evaluation results
	totalLLMTokens := int64(0)
	totalLLMCost := float64(0)
	agentReportsMap := make(map[string]interface{})

	for _, eval := range agentEvals {
		if eval.Error != nil {
			logging.Info("Agent %s evaluation failed: %v", eval.AgentName, eval.Error)
			continue
		}

		// Create agent report detail
		criteriaScoresJSON, _ := json.Marshal(eval.CriteriaScores)
		strengthsJSON, _ := json.Marshal(eval.Strengths)
		weaknessesJSON, _ := json.Marshal(eval.Weaknesses)
		recommendationsJSON, _ := json.Marshal(eval.Recommendations)
		runIDsStr := fmt.Sprintf("%v", eval.RunIDs)

		// Marshal enhanced enterprise data
		bestRunJSON, _ := json.Marshal(eval.BestRunExample)
		worstRunJSON, _ := json.Marshal(eval.WorstRunExample)
		toolUsageJSON, _ := json.Marshal(eval.ToolUsageAnalysis)
		failurePatternsJSON, _ := json.Marshal(eval.FailurePatterns)
		improvementPlanJSON, _ := json.Marshal(eval.ImprovementPlan)
		qualityMetricsJSON, _ := json.Marshal(eval.QualityMetrics)

		if _, err := rg.repos.Reports.CreateAgentReportDetail(ctx, queries.CreateAgentReportDetailParams{
			ReportID:           reportID,
			AgentID:            eval.AgentID,
			AgentName:          eval.AgentName,
			Score:              eval.Score,
			Passed:             eval.Passed,
			Reasoning:          sql.NullString{String: eval.Reasoning, Valid: true},
			CriteriaScores:     sql.NullString{String: string(criteriaScoresJSON), Valid: true},
			RunsAnalyzed:       sql.NullInt64{Int64: int64(eval.RunsAnalyzed), Valid: true},
			RunIds:             sql.NullString{String: runIDsStr, Valid: true},
			AvgDurationSeconds: sql.NullFloat64{Float64: eval.AvgDuration, Valid: true},
			AvgTokens:          sql.NullInt64{Int64: eval.AvgTokens, Valid: true},
			AvgCost:            sql.NullFloat64{Float64: eval.AvgCost, Valid: true},
			SuccessRate:        sql.NullFloat64{Float64: eval.SuccessRate, Valid: true},
			Strengths:          sql.NullString{String: string(strengthsJSON), Valid: true},
			Weaknesses:         sql.NullString{String: string(weaknessesJSON), Valid: true},
			Recommendations:    sql.NullString{String: string(recommendationsJSON), Valid: true},
			TelemetrySummary:   sql.NullString{},
			// Enterprise enhancements
			BestRunExample:    sql.NullString{String: string(bestRunJSON), Valid: len(bestRunJSON) > 2},
			WorstRunExample:   sql.NullString{String: string(worstRunJSON), Valid: len(worstRunJSON) > 2},
			ToolUsageAnalysis: sql.NullString{String: string(toolUsageJSON), Valid: len(toolUsageJSON) > 2},
			FailurePatterns:   sql.NullString{String: string(failurePatternsJSON), Valid: len(failurePatternsJSON) > 2},
			ImprovementPlan:   sql.NullString{String: string(improvementPlanJSON), Valid: len(improvementPlanJSON) > 2},
			QualityMetrics:    sql.NullString{String: string(qualityMetricsJSON), Valid: len(qualityMetricsJSON) > 2},
		}); err != nil {
			logging.Info("Failed to save agent report detail for %s: %v", eval.AgentName, err)
			continue
		}

		// Add to summary map
		summaryLen := min(200, len(eval.Reasoning))
		agentReportsMap[fmt.Sprintf("%d", eval.AgentID)] = map[string]interface{}{
			"score":   eval.Score,
			"summary": eval.Reasoning[:summaryLen],
		}

		// Estimate token usage
		totalLLMTokens += 1000
		totalLLMCost += 0.001
	}

	// 14. Complete report
	agentReportsJSON, _ := json.Marshal(agentReportsMap)
	duration := time.Since(startTime).Seconds()

	if err := rg.repos.Reports.CompleteReport(ctx, queries.CompleteReportParams{
		ID:                        reportID,
		GenerationDurationSeconds: sql.NullFloat64{Float64: duration, Valid: true},
		TotalRunsAnalyzed:         sql.NullInt64{Int64: int64(len(allRuns)), Valid: true},
		TotalAgentsAnalyzed:       sql.NullInt64{Int64: int64(len(agents)), Valid: true},
		TotalLlmTokens:            sql.NullInt64{Int64: totalLLMTokens, Valid: true},
		TotalLlmCost:              sql.NullFloat64{Float64: totalLLMCost, Valid: true},
		AgentReports:              sql.NullString{String: string(agentReportsJSON), Valid: true},
	}); err != nil {
		return fmt.Errorf("failed to complete report: %w", err)
	}

	logging.Info("Report generation completed in %.2fs - %d agents, %d runs analyzed", duration, len(agents), len(allRuns))
	return nil
}

// Helper methods

func (rg *ReportGenerator) failReport(ctx context.Context, reportID int64, err error) error {
	logging.Error("Report %d failed: %v", reportID, err)
	if failErr := rg.repos.Reports.FailReport(ctx, queries.FailReportParams{
		ID:           reportID,
		ErrorMessage: sql.NullString{String: err.Error(), Valid: true},
	}); failErr != nil {
		logging.Info("Failed to mark report as failed: %v", failErr)
	}
	return err
}

func (rg *ReportGenerator) updateReportStatus(ctx context.Context, reportID int64, status string, progress int, step string) error {
	return rg.repos.Reports.UpdateStatus(ctx, queries.UpdateReportStatusParams{
		ID:          reportID,
		Status:      status,
		Progress:    sql.NullInt64{Int64: int64(progress), Valid: true},
		CurrentStep: sql.NullString{String: step, Valid: true},
	})
}

func (rg *ReportGenerator) fetchAllRuns(ctx context.Context, agents []models.Agent, filterModel string) ([]queries.AgentRun, error) {
	allRuns := make([]queries.AgentRun, 0)

	for _, agent := range agents {
		var runs []queries.AgentRun
		var err error

		if filterModel != "" {
			// Filter runs by specific model
			logging.Info("Fetching runs for agent %s filtered by model: %s", agent.Name, filterModel)
			runs, err = rg.repos.AgentRuns.GetRecentByAgentAndModel(ctx, agent.ID, filterModel, 20)
		} else {
			// Fetch all runs regardless of model
			logging.Info("Fetching all runs for agent %s", agent.Name)
			runs, err = rg.repos.AgentRuns.GetRecentByAgent(ctx, agent.ID, 20)
		}

		if err != nil {
			logging.Info("Failed to fetch runs for agent %s: %v", agent.Name, err)
			continue
		}
		allRuns = append(allRuns, runs...)
	}

	if filterModel != "" {
		logging.Info("Fetched %d runs total (filtered by model: %s)", len(allRuns), filterModel)
	} else {
		logging.Info("Fetched %d runs total (all models)", len(allRuns))
	}

	return allRuns, nil
}

func (rg *ReportGenerator) evaluateTeamPerformance(ctx context.Context, criteria TeamCriteria, runs []queries.AgentRun, agents []models.Agent) (*TeamEvaluation, error) {
	prompt := rg.buildTeamEvaluationPrompt(criteria, runs, agents)

	response, err := rg.callLLMJudge(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM judge call failed: %w", err)
	}

	// Strip markdown code blocks if present
	cleanedResponse := stripMarkdownCodeBlocks(response)

	var teamEval TeamEvaluation
	if err := json.Unmarshal([]byte(cleanedResponse), &teamEval); err != nil {
		logging.Info("ERROR: Failed to parse LLM response. Raw response length: %d", len(response))
		logging.Info("ERROR: Cleaned response length: %d", len(cleanedResponse))
		logging.Info("ERROR: First 500 chars of cleaned response: %s", cleanedResponse[:min(500, len(cleanedResponse))])
		return nil, fmt.Errorf("failed to parse team evaluation response: %w", err)
	}

	return &teamEval, nil
}

// evaluateTeamPerformanceFromAgentScores calculates team score deterministically from agent scores
// then uses LLM only to generate the executive summary with full context
func (rg *ReportGenerator) evaluateTeamPerformanceFromAgentScores(ctx context.Context, criteria TeamCriteria, agentEvals []AgentEvaluation, agents []models.Agent) (*TeamEvaluation, error) {
	// Calculate deterministic team score from agent scores
	var totalScore float64
	var totalWeight float64
	validAgents := 0

	// Build agent performance summary for LLM context
	agentSummaries := make([]string, 0, len(agentEvals))

	for _, eval := range agentEvals {
		if eval.Error != nil {
			continue
		}
		validAgents++

		// Default weight is 1.0 for each agent (could be enhanced with per-agent weights)
		weight := 1.0
		totalScore += eval.Score * weight
		totalWeight += weight

		// Build summary for this agent
		summary := fmt.Sprintf("- %s: Score %.1f/10, Success Rate %.1f%%, %d runs analyzed",
			eval.AgentName, eval.Score, eval.SuccessRate*100, eval.RunsAnalyzed)
		if len(eval.Strengths) > 0 {
			summary += fmt.Sprintf(", Strengths: %s", strings.Join(eval.Strengths[:min(2, len(eval.Strengths))], ", "))
		}
		if len(eval.Weaknesses) > 0 {
			summary += fmt.Sprintf(", Weaknesses: %s", strings.Join(eval.Weaknesses[:min(2, len(eval.Weaknesses))], ", "))
		}
		agentSummaries = append(agentSummaries, summary)
	}

	// Calculate weighted average team score
	var teamScore float64
	if totalWeight > 0 {
		teamScore = totalScore / totalWeight
	}

	logging.Info("Team score calculated: %.2f from %d valid agents (total weight: %.2f)",
		teamScore, validAgents, totalWeight)

	// Now use LLM to generate executive summary with full agent context
	prompt := rg.buildTeamSummaryPrompt(criteria, teamScore, agentSummaries, agents)

	response, err := rg.callLLMJudge(ctx, prompt)
	if err != nil {
		// Fallback: return basic evaluation without LLM summary
		logging.Info("LLM summary generation failed, using fallback: %v", err)
		return &TeamEvaluation{
			Score:     teamScore,
			Reasoning: fmt.Sprintf("Team score calculated as weighted average of %d agent scores.", validAgents),
			Summary:   fmt.Sprintf("The team achieved an overall score of %.1f/10 based on %d evaluated agents.", teamScore, validAgents),
		}, nil
	}

	cleanedResponse := stripMarkdownCodeBlocks(response)

	var summaryResult struct {
		Summary        string                    `json:"summary"`
		Reasoning      string                    `json:"reasoning"`
		CriteriaScores map[string]CriterionScore `json:"criteria_scores,omitempty"`
	}
	if err := json.Unmarshal([]byte(cleanedResponse), &summaryResult); err != nil {
		logging.Info("Failed to parse LLM summary, using fallback: %v", err)
		return &TeamEvaluation{
			Score:     teamScore,
			Reasoning: fmt.Sprintf("Team score calculated as weighted average of %d agent scores.", validAgents),
			Summary:   fmt.Sprintf("The team achieved an overall score of %.1f/10 based on %d evaluated agents.", teamScore, validAgents),
		}, nil
	}

	return &TeamEvaluation{
		Score:          teamScore, // Use calculated score, NOT LLM-generated
		Reasoning:      summaryResult.Reasoning,
		Summary:        summaryResult.Summary,
		CriteriaScores: summaryResult.CriteriaScores,
	}, nil
}

// buildTeamSummaryPrompt creates prompt for generating executive summary with pre-calculated score
func (rg *ReportGenerator) buildTeamSummaryPrompt(criteria TeamCriteria, calculatedScore float64, agentSummaries []string, agents []models.Agent) string {
	criteriaDesc := ""
	for name, criterion := range criteria.Criteria {
		criteriaDesc += fmt.Sprintf("- **%s** (weight: %.1f, threshold: %.1f): %s\n",
			name, criterion.Weight, criterion.Threshold, criterion.Description)
	}

	return fmt.Sprintf(`You are an expert evaluator writing an executive summary for a team performance report.

**Team Goal:**
%s

**Evaluation Criteria:**
%s

**CALCULATED Team Score: %.1f/10** (This score is pre-calculated from agent scores - DO NOT change it)

**Agent Performance Summary:**
%s

**Instructions:**
1. Write a professional executive summary (2-3 paragraphs) explaining the team's performance
2. Reference specific agent strengths and weaknesses
3. Provide actionable recommendations
4. The team score of %.1f/10 is FIXED - explain WHY this score makes sense given the data

**Output Format (JSON):**
{
  "summary": "<2-3 paragraph executive summary>",
  "reasoning": "<1 paragraph explaining why the calculated score is appropriate>",
  "criteria_scores": {
    "criterion_name": {
      "score": <float based on agent data>,
      "reasoning": "<explanation>"
    }
  }
}

Be objective and data-driven. Reference specific agent performance metrics.`,
		criteria.Goal,
		criteriaDesc,
		calculatedScore,
		strings.Join(agentSummaries, "\n"),
		calculatedScore)
}

// stripMarkdownCodeBlocks removes markdown code block delimiters and fixes common JSON issues
func stripMarkdownCodeBlocks(response string) string {
	// Remove opening code block with optional language identifier
	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
	} else if strings.HasPrefix(response, "```") {
		response = strings.TrimPrefix(response, "```")
	}

	// Remove closing code block
	if strings.HasSuffix(response, "```") {
		response = strings.TrimSuffix(response, "```")
	}

	response = strings.TrimSpace(response)

	// Try to fix common JSON formatting issues by doing a best-effort parse and re-encode
	// This handles cases where the LLM returns valid-looking JSON with unescaped newlines
	var rawJSON interface{}
	decoder := json.NewDecoder(strings.NewReader(response))
	decoder.UseNumber() // Preserve number precision

	if err := decoder.Decode(&rawJSON); err == nil {
		// Successfully parsed, re-encode cleanly
		if cleanBytes, err := json.Marshal(rawJSON); err == nil {
			return string(cleanBytes)
		}
	}

	// If parsing failed, return the cleaned response as-is
	// The caller will handle the error
	return response
}

func (rg *ReportGenerator) evaluateAgentsParallel(ctx context.Context, reportID int64, agents []models.Agent, agentCriteria map[int64]map[string]EvaluationCriterion) []AgentEvaluation {
	var wg sync.WaitGroup
	results := make([]AgentEvaluation, len(agents))

	semaphore := make(chan struct{}, rg.maxConcurrentEvals)
	completed := 0
	progressMutex := sync.Mutex{}

	for i, agent := range agents {
		wg.Add(1)

		go func(idx int, ag models.Agent) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			eval := rg.evaluateAgent(ctx, ag, agentCriteria[ag.ID])
			results[idx] = eval

			progressMutex.Lock()
			completed++
			progress := 30 + int((float64(completed)/float64(len(agents)))*60)
			step := fmt.Sprintf("Evaluated %d/%d agents", completed, len(agents))
			progressMutex.Unlock()

			if err := rg.updateReportStatus(ctx, reportID, "generating_agents", progress, step); err != nil {
				logging.Info("Failed to update progress: %v", err)
			}
		}(i, agent)
	}

	wg.Wait()
	return results
}

func (rg *ReportGenerator) evaluateAgent(ctx context.Context, agent models.Agent, criteria map[string]EvaluationCriterion) AgentEvaluation {
	runs, err := rg.repos.AgentRuns.GetRecentByAgent(ctx, agent.ID, 20)
	if err != nil {
		return AgentEvaluation{
			AgentID:   agent.ID,
			AgentName: agent.Name,
			Error:     fmt.Errorf("failed to fetch runs: %w", err),
		}
	}

	if len(runs) == 0 {
		return AgentEvaluation{
			AgentID:   agent.ID,
			AgentName: agent.Name,
			Error:     fmt.Errorf("no runs found for agent"),
		}
	}

	metrics := rg.calculateAgentMetrics(runs)

	// Extract best and worst run examples
	bestRun, worstRun := rg.findBestAndWorstRuns(runs)

	// Analyze tool usage patterns
	toolUsage := rg.analyzeToolUsage(runs)

	// Identify failure patterns
	failurePatterns := rg.identifyFailurePatterns(runs)

	prompt := rg.buildAgentEvaluationPrompt(agent, runs, criteria, metrics)

	response, err := rg.callLLMJudge(ctx, prompt)
	if err != nil {
		return AgentEvaluation{
			AgentID:   agent.ID,
			AgentName: agent.Name,
			Error:     fmt.Errorf("LLM judge failed: %w", err),
		}
	}

	// Strip markdown code blocks if present
	cleanedResponse := stripMarkdownCodeBlocks(response)

	var judgeResult struct {
		Score           float64                   `json:"score"`
		Passed          bool                      `json:"passed"`
		Reasoning       string                    `json:"reasoning"`
		CriteriaScores  map[string]CriterionScore `json:"criteria_scores"`
		Strengths       []string                  `json:"strengths"`
		Weaknesses      []string                  `json:"weaknesses"`
		Recommendations []string                  `json:"recommendations"`
	}

	if err := json.Unmarshal([]byte(cleanedResponse), &judgeResult); err != nil {
		return AgentEvaluation{
			AgentID:   agent.ID,
			AgentName: agent.Name,
			Error:     fmt.Errorf("failed to parse judge response: %w", err),
		}
	}

	runIDs := make([]int64, len(runs))
	for i, run := range runs {
		runIDs[i] = run.ID
	}

	// Build improvement plan based on failures and weaknesses
	improvementPlan := rg.buildImprovementPlan(runs, failurePatterns, judgeResult.Weaknesses)

	return AgentEvaluation{
		AgentID:         agent.ID,
		AgentName:       agent.Name,
		Score:           judgeResult.Score,
		Passed:          judgeResult.Passed,
		Reasoning:       judgeResult.Reasoning,
		CriteriaScores:  judgeResult.CriteriaScores,
		Strengths:       judgeResult.Strengths,
		Weaknesses:      judgeResult.Weaknesses,
		Recommendations: judgeResult.Recommendations,
		RunsAnalyzed:    len(runs),
		RunIDs:          runIDs,
		AvgDuration:     metrics.AvgDuration,
		AvgTokens:       metrics.AvgTokens,
		AvgCost:         metrics.AvgCost,
		SuccessRate:     metrics.SuccessRate,

		// Enterprise enhancements
		BestRunExample:    bestRun,
		WorstRunExample:   worstRun,
		ToolUsageAnalysis: toolUsage,
		FailurePatterns:   failurePatterns,
		ImprovementPlan:   improvementPlan,
		QualityMetrics:    metrics.QualityMetrics,
	}
}

type AgentMetrics struct {
	AvgDuration float64
	AvgTokens   int64
	AvgCost     float64
	SuccessRate float64

	// LLM-as-Judge Quality Metrics (from benchmark_metrics table)
	QualityMetrics *QualityMetrics `json:"quality_metrics,omitempty"`
}

// QualityMetrics represents aggregate quality scores from LLM-as-judge evaluations
type QualityMetrics struct {
	// Average scores across evaluated runs (0.0-1.0)
	AvgTaskCompletion float64 `json:"avg_task_completion"`
	AvgRelevancy      float64 `json:"avg_relevancy"`
	AvgFaithfulness   float64 `json:"avg_faithfulness"`
	AvgHallucination  float64 `json:"avg_hallucination"` // Lower is better
	AvgToxicity       float64 `json:"avg_toxicity"`      // Lower is better

	// Pass rates (percentage of runs that met thresholds)
	TaskCompletionPassRate float64 `json:"task_completion_pass_rate"`
	RelevancyPassRate      float64 `json:"relevancy_pass_rate"`
	FaithfulnessPassRate   float64 `json:"faithfulness_pass_rate"`
	HallucinationPassRate  float64 `json:"hallucination_pass_rate"`
	ToxicityPassRate       float64 `json:"toxicity_pass_rate"`

	// Metadata
	EvaluatedRuns int `json:"evaluated_runs"` // Number of runs with benchmarks
	TotalRuns     int `json:"total_runs"`     // Total runs analyzed
}

func (rg *ReportGenerator) calculateAgentMetrics(runs []queries.AgentRun) AgentMetrics {
	if len(runs) == 0 {
		return AgentMetrics{}
	}

	var totalDuration float64
	var totalTokens int64
	successCount := 0

	for _, run := range runs {
		if run.DurationSeconds.Valid {
			totalDuration += run.DurationSeconds.Float64
		}
		if run.TotalTokens.Valid {
			totalTokens += run.TotalTokens.Int64
		}
		if run.Status == "completed" {
			successCount++
		}
	}

	count := float64(len(runs))
	// Estimate cost: ~$0.002 per 1000 tokens for gpt-4o-mini
	avgCost := (float64(totalTokens) / 1000.0) * 0.002 / count

	metrics := AgentMetrics{
		AvgDuration: totalDuration / count,
		AvgTokens:   totalTokens / int64(len(runs)),
		AvgCost:     avgCost,
		SuccessRate: float64(successCount) / count,
	}

	// Calculate quality metrics from benchmark_metrics table
	qualityMetrics := rg.calculateQualityMetrics(runs)
	if qualityMetrics != nil {
		metrics.QualityMetrics = qualityMetrics
	}

	return metrics
}

// calculateQualityMetrics aggregates LLM-as-judge scores from benchmark_metrics
func (rg *ReportGenerator) calculateQualityMetrics(runs []queries.AgentRun) *QualityMetrics {
	if rg.db == nil || len(runs) == 0 {
		return nil
	}

	// Build run IDs list for query
	runIDs := make([]string, len(runs))
	for i, run := range runs {
		runIDs[i] = fmt.Sprintf("%d", run.ID)
	}

	// Query benchmark metrics for these runs
	query := fmt.Sprintf(`
		SELECT 
			metric_type,
			AVG(score) as avg_score,
			SUM(CASE WHEN passed = 1 THEN 1 ELSE 0 END) * 100.0 / COUNT(*) as pass_rate,
			COUNT(*) as count
		FROM benchmark_metrics
		WHERE run_id IN (%s)
		GROUP BY metric_type
	`, strings.Join(runIDs, ","))

	rows, err := rg.db.Query(query)
	if err != nil {
		logging.Info("Failed to query benchmark metrics: %v", err)
		return nil
	}
	defer rows.Close()

	quality := &QualityMetrics{
		TotalRuns: len(runs),
	}

	for rows.Next() {
		var metricType string
		var avgScore, passRate float64
		var count int

		if err := rows.Scan(&metricType, &avgScore, &passRate, &count); err != nil {
			continue
		}

		// Track which runs have benchmarks
		if count > quality.EvaluatedRuns {
			quality.EvaluatedRuns = count
		}

		switch metricType {
		case "task_completion":
			quality.AvgTaskCompletion = avgScore
			quality.TaskCompletionPassRate = passRate
		case "relevancy":
			quality.AvgRelevancy = avgScore
			quality.RelevancyPassRate = passRate
		case "faithfulness":
			quality.AvgFaithfulness = avgScore
			quality.FaithfulnessPassRate = passRate
		case "hallucination":
			quality.AvgHallucination = avgScore
			quality.HallucinationPassRate = passRate
		case "toxicity":
			quality.AvgToxicity = avgScore
			quality.ToxicityPassRate = passRate
		}
	}

	// Only return if we found some benchmark data
	if quality.EvaluatedRuns == 0 {
		return nil
	}

	return quality
}

// LLM judge integration methods

func (rg *ReportGenerator) callLLMJudge(ctx context.Context, prompt string) (string, error) {
	genkitApp, err := rg.genkitProvider.GetApp(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get GenKit app: %w", err)
	}

	// Get Station's global AI configuration to determine provider
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load Station config: %w", err)
	}

	// Format model name with provider prefix (e.g., "googleai/gemini-2.5-pro", "openai/gpt-4o-mini")
	modelName := rg.judgeModel
	if !strings.Contains(modelName, "/") {
		// Use Station's configured provider
		provider := strings.ToLower(cfg.AIProvider)
		if provider == "gemini" {
			provider = "googleai" // GenKit uses "googleai" for Gemini models
		}
		modelName = fmt.Sprintf("%s/%s", provider, modelName)
		logging.Debug("Formatted LLM judge model name: %s (provider: %s)", modelName, cfg.AIProvider)
	}

	response, err := genkit.Generate(ctx, genkitApp,
		ai.WithPrompt(prompt),
		ai.WithModelName(modelName))

	if err != nil {
		return "", fmt.Errorf("GenKit generate failed: %w", err)
	}

	return response.Text(), nil
}

func (rg *ReportGenerator) buildTeamEvaluationPrompt(criteria TeamCriteria, runs []queries.AgentRun, agents []models.Agent) string {
	criteriaDesc := ""
	for name, criterion := range criteria.Criteria {
		criteriaDesc += fmt.Sprintf("- **%s** (weight: %.1f, threshold: %.1f): %s\n",
			name, criterion.Weight, criterion.Threshold, criterion.Description)
	}

	return fmt.Sprintf(`You are an expert evaluator assessing overall team performance across multiple agents.

**Team Goal:**
%s

**Evaluation Criteria:**
%s

**Team Statistics:**
- Total agents: %d
- Total runs analyzed: %d
- Agents: %s

**Instructions:**
1. Evaluate overall team performance based on run data
2. Assign scores 0-10 for each criterion
3. Calculate weighted overall score
4. Provide executive summary

**Output Format (JSON):**
{
  "score": <float>,
  "reasoning": "<why this score>",
  "summary": "<2-3 paragraph executive summary>",
  "criteria_scores": {
    "criterion_name": {
      "score": <float>,
      "reasoning": "<explanation>"
    }
  }
}

Be objective and data-driven in your evaluation.`,
		criteria.Goal,
		criteriaDesc,
		len(agents),
		len(runs),
		getAgentNames(agents))
}

func (rg *ReportGenerator) buildAgentEvaluationPrompt(agent models.Agent, runs []queries.AgentRun, criteria map[string]EvaluationCriterion, metrics AgentMetrics) string {
	criteriaDesc := "Using default quality criteria:\n"
	criteriaDesc += "- accuracy (weight: 0.4, threshold: 7.0): Correctness of outputs\n"
	criteriaDesc += "- reliability (weight: 0.3, threshold: 7.0): Consistent performance\n"
	criteriaDesc += "- efficiency (weight: 0.3, threshold: 6.0): Speed and resource usage\n"

	if len(criteria) > 0 {
		criteriaDesc = ""
		for name, criterion := range criteria {
			criteriaDesc += fmt.Sprintf("- **%s** (weight: %.1f, threshold: %.1f): %s\n",
				name, criterion.Weight, criterion.Threshold, criterion.Description)
		}
	}

	// Build quality metrics section if available
	qualityMetricsDesc := ""
	if metrics.QualityMetrics != nil {
		q := metrics.QualityMetrics
		qualityMetricsDesc = fmt.Sprintf(`
**Quality Metrics (LLM-as-Judge):**
- Task Completion: %.2f/10 (%.1f%% pass rate) - %d/%d runs evaluated
- Relevancy: %.2f/10 (%.1f%% pass rate)
- Faithfulness: %.2f/10 (%.1f%% pass rate) - how well output aligns with tool context
- Hallucination: %.2f/10 (%.1f%% pass rate) - lower is better, measures fabricated info
- Toxicity: %.2f/10 (%.1f%% pass rate) - lower is better

These scores come from automated LLM-based evaluation of actual agent outputs and tool usage.
`,
			q.AvgTaskCompletion, q.TaskCompletionPassRate, q.EvaluatedRuns, q.TotalRuns,
			q.AvgRelevancy, q.RelevancyPassRate,
			q.AvgFaithfulness, q.FaithfulnessPassRate,
			q.AvgHallucination, q.HallucinationPassRate,
			q.AvgToxicity, q.ToxicityPassRate,
		)
	}

	return fmt.Sprintf(`You are an expert evaluator assessing individual agent performance.

**Agent:** %s
**Description:** %s

**Performance Metrics:**
- Runs analyzed: %d
- Success rate: %.1f%%
- Avg duration: %.2fs
- Avg tokens: %d
%s
**Evaluation Criteria:**
%s

**Instructions:**
1. Evaluate agent based on metrics and criteria
2. Consider both performance metrics AND quality metrics in your assessment
3. Assign scores 0-10 for each criterion
4. Identify strengths and weaknesses
5. Provide actionable recommendations

**Output Format (JSON):**
{
  "score": <float>,
  "passed": <boolean>,
  "reasoning": "<overall assessment>",
  "criteria_scores": {
    "criterion_name": {
      "score": <float>,
      "reasoning": "<explanation>",
      "examples": ["<specific observation>"]
    }
  },
  "strengths": ["<strength 1>", "<strength 2>"],
  "weaknesses": ["<weakness 1>", "<weakness 2>"],
  "recommendations": ["<recommendation 1>", "<recommendation 2>"]
}

Be specific and actionable in your feedback. Pay special attention to quality metrics - they represent actual evaluation of agent outputs.`,
		agent.Name,
		agent.Description,
		len(runs),
		metrics.SuccessRate*100,
		metrics.AvgDuration,
		metrics.AvgTokens,
		qualityMetricsDesc,
		criteriaDesc)
}

func getAgentNames(agents []models.Agent) string {
	names := make([]string, len(agents))
	for i, agent := range agents {
		names[i] = agent.Name
	}
	return fmt.Sprintf("%v", names)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// findBestAndWorstRuns identifies the best and worst performing runs
func (rg *ReportGenerator) findBestAndWorstRuns(runs []queries.AgentRun) (*RunExample, *RunExample) {
	if len(runs) == 0 {
		return nil, nil
	}

	var bestRun, worstRun *queries.AgentRun
	bestScore := -1.0
	worstScore := 999999.0

	for i := range runs {
		run := &runs[i]

		// Score calculation: prioritize completed runs, then by duration and tokens
		score := 0.0
		if run.Status == "completed" {
			score = 100.0
			// Prefer faster, more efficient runs
			if run.DurationSeconds.Valid && run.DurationSeconds.Float64 > 0 {
				score -= run.DurationSeconds.Float64 / 10.0
			}
			if run.TotalTokens.Valid && run.TotalTokens.Int64 > 0 {
				score -= float64(run.TotalTokens.Int64) / 1000.0
			}
		} else {
			score = 0.0 // Failed runs
		}

		if score > bestScore {
			bestScore = score
			bestRun = run
		}
		if score < worstScore {
			worstScore = score
			worstRun = run
		}
	}

	var best, worst *RunExample
	if bestRun != nil {
		best = &RunExample{
			RunID:       bestRun.ID,
			Input:       bestRun.Task,
			Output:      bestRun.FinalResponse,
			Status:      bestRun.Status,
			Explanation: "This run completed successfully with good efficiency",
		}
		if bestRun.ToolCalls.Valid {
			tools := []string{}
			var toolCalls []map[string]interface{}
			if err := json.Unmarshal([]byte(bestRun.ToolCalls.String), &toolCalls); err == nil {
				for _, tc := range toolCalls {
					if name, ok := tc["tool_name"].(string); ok {
						tools = append(tools, name)
					}
				}
			}
			best.ToolCalls = tools
		}
		if bestRun.DurationSeconds.Valid {
			best.Duration = bestRun.DurationSeconds.Float64
		}
		if bestRun.TotalTokens.Valid {
			best.TokenCount = bestRun.TotalTokens.Int64
		}
	}

	if worstRun != nil && worstRun.ID != bestRun.ID {
		worst = &RunExample{
			RunID:  worstRun.ID,
			Input:  worstRun.Task,
			Output: worstRun.FinalResponse,
			Status: worstRun.Status,
		}
		if worstRun.Status != "completed" {
			worst.Explanation = fmt.Sprintf("Run failed with status: %s", worstRun.Status)
			if worstRun.Error.Valid {
				worst.Explanation += fmt.Sprintf(" - Error: %s", worstRun.Error.String)
			}
		} else {
			worst.Explanation = "This run completed but was slower or less efficient than others"
		}
		if worstRun.ToolCalls.Valid {
			tools := []string{}
			var toolCalls []map[string]interface{}
			if err := json.Unmarshal([]byte(worstRun.ToolCalls.String), &toolCalls); err == nil {
				for _, tc := range toolCalls {
					if name, ok := tc["tool_name"].(string); ok {
						tools = append(tools, name)
					}
				}
			}
			worst.ToolCalls = tools
		}
		if worstRun.DurationSeconds.Valid {
			worst.Duration = worstRun.DurationSeconds.Float64
		}
		if worstRun.TotalTokens.Valid {
			worst.TokenCount = worstRun.TotalTokens.Int64
		}
	}

	return best, worst
}

// analyzeToolUsage analyzes tool usage patterns across runs
func (rg *ReportGenerator) analyzeToolUsage(runs []queries.AgentRun) []ToolUsageStats {
	toolStats := make(map[string]*ToolUsageStats)

	for _, run := range runs {
		if !run.ToolCalls.Valid {
			continue
		}

		var toolCalls []map[string]interface{}
		if err := json.Unmarshal([]byte(run.ToolCalls.String), &toolCalls); err != nil {
			continue
		}

		for _, tc := range toolCalls {
			toolName, ok := tc["tool_name"].(string)
			if !ok {
				continue
			}

			if _, exists := toolStats[toolName]; !exists {
				toolStats[toolName] = &ToolUsageStats{
					ToolName: toolName,
				}
			}

			toolStats[toolName].UseCount++
			if run.Status == "completed" {
				toolStats[toolName].SuccessRate++
			}
		}
	}

	// Calculate final success rates and convert to slice
	result := []ToolUsageStats{}
	for _, stats := range toolStats {
		if stats.UseCount > 0 {
			stats.SuccessRate = stats.SuccessRate / float64(stats.UseCount)
		}
		result = append(result, *stats)
	}

	return result
}

// identifyFailurePatterns identifies common failure patterns
func (rg *ReportGenerator) identifyFailurePatterns(runs []queries.AgentRun) []FailurePattern {
	errorCounts := make(map[string]*FailurePattern)

	for _, run := range runs {
		if run.Status == "completed" {
			continue
		}

		errorKey := run.Status
		if run.Error.Valid && run.Error.String != "" {
			errorKey = run.Error.String
		}

		if _, exists := errorCounts[errorKey]; !exists {
			errorCounts[errorKey] = &FailurePattern{
				Pattern:  errorKey,
				Examples: []string{},
				Impact:   "Medium",
			}
		}

		errorCounts[errorKey].Frequency++
		if len(errorCounts[errorKey].Examples) < 3 {
			errorCounts[errorKey].Examples = append(errorCounts[errorKey].Examples, fmt.Sprintf("%d", run.ID))
		}
	}

	// Determine impact based on frequency
	result := []FailurePattern{}
	for _, pattern := range errorCounts {
		if pattern.Frequency >= 5 {
			pattern.Impact = "High"
		} else if pattern.Frequency >= 2 {
			pattern.Impact = "Medium"
		} else {
			pattern.Impact = "Low"
		}
		result = append(result, *pattern)
	}

	return result
}

// buildImprovementPlan creates actionable improvement recommendations
func (rg *ReportGenerator) buildImprovementPlan(runs []queries.AgentRun, patterns []FailurePattern, weaknesses []string) []ImprovementAction {
	plan := []ImprovementAction{}

	// Address high-impact failure patterns first
	for _, pattern := range patterns {
		if pattern.Impact == "High" {
			plan = append(plan, ImprovementAction{
				Issue:           fmt.Sprintf("Recurring failure: %s", pattern.Pattern),
				Recommendation:  "Investigate root cause and implement error handling",
				Priority:        "High",
				ExpectedImpact:  fmt.Sprintf("Reduce failures by %d%%", pattern.Frequency*10),
				ConcreteExample: fmt.Sprintf("See run IDs: %v", pattern.Examples),
			})
		}
	}

	// Address weaknesses identified by LLM judge
	for i, weakness := range weaknesses {
		priority := "Medium"
		if i == 0 {
			priority = "High" // First weakness is typically most critical
		}

		plan = append(plan, ImprovementAction{
			Issue:           weakness,
			Recommendation:  "Review agent prompt and tool selection to address this weakness",
			Priority:        priority,
			ExpectedImpact:  "+10-15% performance improvement",
			ConcreteExample: "Review failed runs for specific examples",
		})
	}

	// Check for efficiency issues
	totalDuration := 0.0
	slowRuns := 0
	for _, run := range runs {
		if run.DurationSeconds.Valid {
			totalDuration += run.DurationSeconds.Float64
			if run.DurationSeconds.Float64 > 60.0 {
				slowRuns++
			}
		}
	}

	if slowRuns > len(runs)/3 {
		plan = append(plan, ImprovementAction{
			Issue:           fmt.Sprintf("%d runs exceeded 60 seconds", slowRuns),
			Recommendation:  "Optimize tool calls and reduce unnecessary processing",
			Priority:        "Medium",
			ExpectedImpact:  "-20-30% execution time",
			ConcreteExample: "Review slow runs for optimization opportunities",
		})
	}

	return plan
}

// calculateTeamCostAnalysis creates comprehensive cost analysis with projections
func (rg *ReportGenerator) calculateTeamCostAnalysis(agentEvals []AgentEvaluation, agents []models.Agent) *TeamCostAnalysis {
	analysis := &TeamCostAnalysis{
		TotalTeamAgents: len(agents),
		AgentCosts:      make(map[string]float64),
	}

	totalCost := 0.0
	totalTokens := int64(0)
	totalDuration := 0.0
	totalRuns := 0

	mostExpensiveCost := 0.0
	mostExpensiveName := ""
	bestEfficiencyRatio := 999999.0
	mostEfficientName := ""

	// Aggregate data from all agents
	for _, eval := range agentEvals {
		if eval.Error != nil || eval.RunsAnalyzed == 0 {
			continue
		}

		agentTotalCost := eval.AvgCost * float64(eval.RunsAnalyzed)
		totalCost += agentTotalCost
		totalTokens += eval.AvgTokens * int64(eval.RunsAnalyzed)
		totalDuration += eval.AvgDuration * float64(eval.RunsAnalyzed)
		totalRuns += eval.RunsAnalyzed

		// Track per-agent costs
		analysis.AgentCosts[eval.AgentName] = eval.AvgCost

		// Find most expensive
		if eval.AvgCost > mostExpensiveCost {
			mostExpensiveCost = eval.AvgCost
			mostExpensiveName = eval.AgentName
		}

		// Find most efficient (best score per dollar)
		if eval.AvgCost > 0 {
			efficiencyRatio := eval.AvgCost / eval.Score
			if efficiencyRatio < bestEfficiencyRatio {
				bestEfficiencyRatio = efficiencyRatio
				mostEfficientName = eval.AgentName
			}
		}
	}

	if totalRuns > 0 {
		analysis.CurrentAvgCostPerRun = totalCost / float64(totalRuns)
		analysis.CurrentAvgTokensPerRun = totalTokens / int64(totalRuns)
		analysis.CurrentAvgDuration = totalDuration / float64(totalRuns)
	}

	analysis.MostExpensiveAgent = mostExpensiveName
	analysis.MostEfficientAgent = mostEfficientName

	// Calculate ROI if we have cost data
	// Assuming $100 value per successful agent execution (configurable)
	estimatedValuePerRun := 100.0
	if analysis.CurrentAvgCostPerRun > 0 {
		analysis.EstimatedValuePerRun = estimatedValuePerRun
		analysis.ROIRatio = estimatedValuePerRun / analysis.CurrentAvgCostPerRun
	}

	// Generate projections for different frequencies
	analysis.Projections = rg.generateCostProjections(
		len(agents),
		analysis.CurrentAvgCostPerRun,
		float64(analysis.CurrentAvgTokensPerRun),
		analysis.CurrentAvgTokensPerRun,
	)

	return analysis
}

// generateCostProjections creates cost estimates for different execution schedules
func (rg *ReportGenerator) generateCostProjections(teamSize int, avgCostPerRun, avgTokensPerRun float64, avgTokens int64) []CostProjection {
	// Define frequency scenarios
	scenarios := []struct {
		name         string
		runsPerMonth int
		description  string
	}{
		{"Every 5 Minutes", 8640 * teamSize, "24/7 continuous monitoring (288 runs/day per agent)"},
		{"Every 15 Minutes", 2880 * teamSize, "High-frequency monitoring (96 runs/day per agent)"},
		{"Hourly", 720 * teamSize, "24/7 hourly execution (24 runs/day per agent)"},
		{"Every 4 Hours", 180 * teamSize, "Regular monitoring (6 runs/day per agent)"},
		{"Daily (Business Hours)", 22 * teamSize, "Once per business day (22 working days)"},
		{"Daily (24/7)", 30 * teamSize, "Once per day, every day"},
		{"Weekly", 4 * teamSize, "Once per week per agent"},
		{"Monthly", 1 * teamSize, "Once per month per agent"},
	}

	projections := []CostProjection{}
	for _, scenario := range scenarios {
		totalCost := avgCostPerRun * float64(scenario.runsPerMonth)
		totalTokens := avgTokens * int64(scenario.runsPerMonth)

		projections = append(projections, CostProjection{
			Frequency:       scenario.name,
			RunsPerPeriod:   scenario.runsPerMonth,
			CostPerRun:      avgCostPerRun,
			TotalCost:       totalCost,
			TokensPerPeriod: totalTokens,
		})
	}

	return projections
}

// evaluateBenchmarksForRuns runs individual benchmark evaluations on all runs
// This populates the 5 quality metrics: hallucination, relevancy, task_completion, faithfulness, toxicity
func (rg *ReportGenerator) evaluateBenchmarksForRuns(ctx context.Context, runs []queries.AgentRun) error {
	if rg.db == nil {
		logging.Info("Skipping benchmark evaluation: database connection not available")
		return nil
	}

	// Load config to get AI provider settings for the judge
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create benchmark judge and analyzer
	judge, err := benchmark.NewJudge(cfg)
	if err != nil {
		return fmt.Errorf("failed to create benchmark judge: %w", err)
	}

	// Create analyzer with Jaeger integration if available
	var analyzer *benchmark.Analyzer
	if cfg.JaegerQueryURL != "" {
		jaegerClient := NewJaegerClient(cfg.JaegerQueryURL)
		if jaegerClient.IsAvailable() {
			adapter := NewBenchmarkJaegerAdapter(jaegerClient)
			analyzer = benchmark.NewAnalyzerWithJaeger(rg.db, judge, adapter)
			logging.Info("Report benchmarks will use Jaeger traces for evaluation context")
		} else {
			analyzer = benchmark.NewAnalyzer(rg.db, judge)
			logging.Info("Jaeger not available at %s, benchmarks will use database tool calls only", cfg.JaegerQueryURL)
		}
	} else {
		analyzer = benchmark.NewAnalyzer(rg.db, judge)
	}

	logging.Info("Running benchmark evaluations on %d runs using model %s...", len(runs), judge.GetModelName())

	// Evaluate runs in parallel with limited concurrency
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, rg.maxConcurrentEvals)
	errorsMu := sync.Mutex{}
	var errors []error

	successCount := 0

	for _, run := range runs {
		wg.Add(1)

		go func(r queries.AgentRun) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Evaluate the run (analyzer will skip if already evaluated)
			_, err := analyzer.EvaluateRun(ctx, r.ID)
			if err != nil {
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("run %d: %w", r.ID, err))
				errorsMu.Unlock()
			} else {
				errorsMu.Lock()
				successCount++
				errorsMu.Unlock()
			}
		}(run)
	}

	wg.Wait()

	logging.Info("Benchmark evaluation complete: %d successful, %d errors", successCount, len(errors))

	// Log errors but don't fail the report generation
	if len(errors) > 0 {
		logging.Info("Benchmark evaluation errors (non-fatal):")
		for _, err := range errors {
			logging.Info("  - %v", err)
		}
	}

	return nil
}
