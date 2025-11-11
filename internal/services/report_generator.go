package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"station/internal/db/queries"
	"station/internal/db/repositories"
	"station/internal/logging"
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
}

// ReportGeneratorConfig configures the report generator
type ReportGeneratorConfig struct {
	JudgeModel         string // Default: "gpt-4o-mini"
	MaxConcurrentEvals int    // Default: 10
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(repos *repositories.Repositories, config *ReportGeneratorConfig) *ReportGenerator {
	if config == nil {
		config = &ReportGeneratorConfig{
			JudgeModel:         "gpt-4o-mini",
			MaxConcurrentEvals: 10,
		}
	}

	if config.JudgeModel == "" {
		config.JudgeModel = "gpt-4o-mini"
	}
	if config.MaxConcurrentEvals == 0 {
		config.MaxConcurrentEvals = 10
	}

	return &ReportGenerator{
		repos:              repos,
		genkitProvider:     NewGenKitProvider(),
		judgeModel:         config.JudgeModel,
		maxConcurrentEvals: config.MaxConcurrentEvals,
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
	Error           error
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

	// 5. Fetch all runs for all agents
	allRuns, err := rg.fetchAllRuns(ctx, agents)
	if err != nil {
		return rg.failReport(ctx, reportID, fmt.Errorf("failed to fetch runs: %w", err))
	}

	if len(allRuns) == 0 {
		return rg.failReport(ctx, reportID, fmt.Errorf("no runs found to analyze"))
	}

	// 6. Evaluate team performance
	teamEval, err := rg.evaluateTeamPerformance(ctx, teamCriteria, allRuns, agents)
	if err != nil {
		return rg.failReport(ctx, reportID, fmt.Errorf("failed to evaluate team: %w", err))
	}

	// 7. Save team evaluation results
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

	// 8. Update status: generating_agents
	if err := rg.updateReportStatus(ctx, reportID, "generating_agents", 30, "Evaluating agents..."); err != nil {
		logging.Info("Failed to update report status: %v", err)
	}

	// 9. Parse agent criteria (optional)
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

	// 10. Evaluate agents in parallel
	agentEvals := rg.evaluateAgentsParallel(ctx, reportID, agents, agentCriteria)

	// 11. Save agent evaluation results
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

	// 12. Complete report
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

func (rg *ReportGenerator) fetchAllRuns(ctx context.Context, agents []models.Agent) ([]queries.AgentRun, error) {
	allRuns := make([]queries.AgentRun, 0)

	for _, agent := range agents {
		runs, err := rg.repos.AgentRuns.GetRecentByAgent(ctx, agent.ID, 20)
		if err != nil {
			logging.Info("Failed to fetch runs for agent %s: %v", agent.Name, err)
			continue
		}
		allRuns = append(allRuns, runs...)
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
		return nil, fmt.Errorf("failed to parse team evaluation response: %w", err)
	}

	return &teamEval, nil
}

// stripMarkdownCodeBlocks removes markdown code block delimiters (```json or ``` )
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

	return strings.TrimSpace(response)
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
	}
}

type AgentMetrics struct {
	AvgDuration float64
	AvgTokens   int64
	AvgCost     float64
	SuccessRate float64
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

	return AgentMetrics{
		AvgDuration: totalDuration / count,
		AvgTokens:   totalTokens / int64(len(runs)),
		AvgCost:     avgCost,
		SuccessRate: float64(successCount) / count,
	}
}

// LLM judge integration methods

func (rg *ReportGenerator) callLLMJudge(ctx context.Context, prompt string) (string, error) {
	genkitApp, err := rg.genkitProvider.GetApp(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get GenKit app: %w", err)
	}

	// Ensure model name is prefixed with provider (e.g., "openai/gpt-4o-mini")
	modelName := rg.judgeModel
	if !strings.Contains(modelName, "/") {
		modelName = "openai/" + modelName
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

	return fmt.Sprintf(`You are an expert evaluator assessing individual agent performance.

**Agent:** %s
**Description:** %s

**Performance Metrics:**
- Runs analyzed: %d
- Success rate: %.1f%%
- Avg duration: %.2fs
- Avg tokens: %d

**Evaluation Criteria:**
%s

**Instructions:**
1. Evaluate agent based on metrics and criteria
2. Assign scores 0-10 for each criterion
3. Identify strengths and weaknesses
4. Provide actionable recommendations

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

Be specific and actionable in your feedback.`,
		agent.Name,
		agent.Description,
		len(runs),
		metrics.SuccessRate*100,
		metrics.AvgDuration,
		metrics.AvgTokens,
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
