package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Dataset-Level Batch Evaluation
// ============================================================================
// Evaluates all runs in a dataset with LLM-as-judge metrics and generates
// comprehensive quality reports with tool effectiveness analysis
// ============================================================================

// DatasetEvaluationInput contains runs to evaluate
type DatasetEvaluationInput struct {
	DatasetID   string       `json:"dataset_id"`
	AgentID     int64        `json:"agent_id"`
	AgentName   string       `json:"agent_name"`
	Runs        []DatasetRun `json:"runs"`
	GeneratedAt time.Time    `json:"generated_at"`
}

// DatasetRun represents a single run in the dataset
type DatasetRun struct {
	RunID           int64                    `json:"run_id"`
	Task            string                   `json:"task"`
	Response        string                   `json:"response"`
	Status          string                   `json:"status"`
	Success         bool                     `json:"success"`
	DurationSeconds float64                  `json:"duration_seconds"`
	StepsTaken      int64                    `json:"steps_taken"`
	TotalTokens     *int64                   `json:"total_tokens,omitempty"`
	ToolCalls       []map[string]interface{} `json:"tool_calls,omitempty"`
}

// DatasetEvaluationResult contains comprehensive evaluation results
type DatasetEvaluationResult struct {
	DatasetID     string    `json:"dataset_id"`
	AgentID       int64     `json:"agent_id"`
	AgentName     string    `json:"agent_name"`
	RunsEvaluated int       `json:"runs_evaluated"`
	EvaluatedAt   time.Time `json:"evaluated_at"`

	// Aggregate LLM-as-judge scores (0.0-1.0)
	AggregateScores AggregateScores `json:"aggregate_scores"`

	// Pass rates (% of runs that passed threshold)
	PassRates PassRates `json:"pass_rates"`

	// Per-run detailed results
	PerRunResults []RunEvaluationSummary `json:"per_run_results"`

	// Tool effectiveness analysis (LLM-powered)
	ToolEffectiveness ToolEffectivenessAnalysis `json:"tool_effectiveness"`

	// Overall assessment
	OverallScore    float64  `json:"overall_score"` // 0-10
	ProductionReady bool     `json:"production_ready"`
	Recommendation  string   `json:"recommendation"`
	KeyStrengths    []string `json:"key_strengths"`
	KeyWeaknesses   []string `json:"key_weaknesses"`

	// Cost tracking
	TotalJudgeCost   float64 `json:"total_judge_cost"`
	EvaluationTimeMS int     `json:"evaluation_time_ms"`
}

// AggregateScores holds average scores across all runs
type AggregateScores struct {
	Hallucination  float64 `json:"hallucination"`   // Lower is better (0-1)
	Relevancy      float64 `json:"relevancy"`       // Higher is better (0-1)
	TaskCompletion float64 `json:"task_completion"` // Higher is better (0-1)
	Faithfulness   float64 `json:"faithfulness"`    // Higher is better (0-1)
	Toxicity       float64 `json:"toxicity"`        // Lower is better (0-1)
}

// PassRates holds percentage of runs that passed thresholds
type PassRates struct {
	Hallucination  float64 `json:"hallucination"`
	Relevancy      float64 `json:"relevancy"`
	TaskCompletion float64 `json:"task_completion"`
	Faithfulness   float64 `json:"faithfulness"`
	Toxicity       float64 `json:"toxicity"`
}

// RunEvaluationSummary summarizes evaluation for one run
type RunEvaluationSummary struct {
	RunID           int64              `json:"run_id"`
	Task            string             `json:"task"`
	Scores          map[string]float64 `json:"scores"`
	Passed          bool               `json:"passed"`
	FailedMetrics   []string           `json:"failed_metrics,omitempty"`
	Reasons         map[string]string  `json:"reasons"`
	ToolsUsed       []string           `json:"tools_used"`
	DurationSeconds float64            `json:"duration_seconds"`
}

// ToolEffectivenessAnalysis contains LLM-powered tool analysis
type ToolEffectivenessAnalysis struct {
	ToolInsights        []ToolInsight     `json:"tool_insights"`
	EffectiveSequences  []SequenceInsight `json:"effective_sequences"`
	InefficientPatterns []PatternInsight  `json:"inefficient_patterns"`
	Recommendations     []string          `json:"recommendations"`
	AnalysisReasoning   string            `json:"analysis_reasoning"`
}

// ToolInsight provides LLM analysis of a specific tool
type ToolInsight struct {
	ToolName           string  `json:"tool_name"`
	UsageCount         int     `json:"usage_count"`
	SuccessRate        float64 `json:"success_rate"`
	AvgCompletionScore float64 `json:"avg_completion_score"`
	Insight            string  `json:"insight"` // LLM-generated insight
}

// SequenceInsight identifies effective tool sequences
type SequenceInsight struct {
	Sequence    []string `json:"sequence"`
	Frequency   int      `json:"frequency"`
	SuccessRate float64  `json:"success_rate"`
	UseCases    string   `json:"use_cases"` // LLM-generated
}

// PatternInsight identifies inefficient patterns
type PatternInsight struct {
	Pattern        string  `json:"pattern"`
	Frequency      int     `json:"frequency"`
	SuccessRate    float64 `json:"success_rate"`
	Recommendation string  `json:"recommendation"` // LLM-generated
}

// ============================================================================
// Main Evaluation Entry Point
// ============================================================================

// EvaluateDataset performs comprehensive LLM-as-judge evaluation on a dataset
func (a *Analyzer) EvaluateDataset(ctx context.Context, input *DatasetEvaluationInput) (*DatasetEvaluationResult, error) {
	startTime := time.Now()

	result := &DatasetEvaluationResult{
		DatasetID:     input.DatasetID,
		AgentID:       input.AgentID,
		AgentName:     input.AgentName,
		RunsEvaluated: len(input.Runs),
		EvaluatedAt:   time.Now(),
	}

	// 1. Evaluate each run with LLM-as-judge
	perRunResults, err := a.evaluateAllRuns(ctx, input.Runs)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate runs: %w", err)
	}
	result.PerRunResults = perRunResults

	// 2. Calculate aggregate scores and pass rates
	result.AggregateScores = a.calculateDatasetAggregateScores(perRunResults)
	result.PassRates = a.calculateDatasetPassRates(perRunResults)

	// 3. Perform LLM-powered tool effectiveness analysis
	toolAnalysis, err := a.analyzeToolEffectiveness(ctx, input.Runs, perRunResults)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze tool effectiveness: %w", err)
	}
	result.ToolEffectiveness = toolAnalysis

	// 4. Generate overall assessment with LLM
	assessment, err := a.generateOverallAssessment(ctx, result)
	if err != nil {
		return nil, fmt.Errorf("failed to generate assessment: %w", err)
	}
	result.OverallScore = assessment.Score
	result.ProductionReady = assessment.ProductionReady
	result.Recommendation = assessment.Recommendation
	result.KeyStrengths = assessment.Strengths
	result.KeyWeaknesses = assessment.Weaknesses

	// 5. Calculate total cost and time
	result.TotalJudgeCost = a.calculateTotalCost(perRunResults, toolAnalysis)
	result.EvaluationTimeMS = int(time.Since(startTime).Milliseconds())

	return result, nil
}

// ============================================================================
// Step 1: Evaluate All Runs
// ============================================================================

func (a *Analyzer) evaluateAllRuns(ctx context.Context, runs []DatasetRun) ([]RunEvaluationSummary, error) {
	results := make([]RunEvaluationSummary, len(runs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(runs))

	// Evaluate runs in parallel (limit concurrency to avoid rate limits)
	semaphore := make(chan struct{}, 5) // Max 5 concurrent evaluations

	for i, run := range runs {
		wg.Add(1)
		go func(idx int, r DatasetRun) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			// Convert to EvaluationInput
			evalInput := a.convertToEvaluationInput(r)

			// Evaluate with existing metrics
			metrics, err := a.evaluateMetrics(ctx, evalInput)
			if err != nil {
				errChan <- fmt.Errorf("run %d evaluation failed: %w", r.RunID, err)
				return
			}

			// Build summary
			summary := a.buildRunSummary(r, metrics)

			mu.Lock()
			results[idx] = summary
			mu.Unlock()
		}(i, run)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return results, nil
}

func (a *Analyzer) convertToEvaluationInput(run DatasetRun) *EvaluationInput {
	input := &EvaluationInput{
		RunID:         run.RunID,
		Task:          run.Task,
		FinalResponse: run.Response,
		Status:        run.Status,
		Duration:      run.DurationSeconds,
	}

	if run.TotalTokens != nil {
		input.Tokens = int(*run.TotalTokens)
	}

	// Convert tool calls
	if len(run.ToolCalls) > 0 {
		input.ToolCalls = make([]ToolCall, len(run.ToolCalls))
		for i, tc := range run.ToolCalls {
			toolName, _ := tc["tool_name"].(string)
			params, _ := tc["parameters"].(map[string]interface{})

			input.ToolCalls[i] = ToolCall{
				Name:       toolName,
				Parameters: params,
			}
		}
	}

	return input
}

func (a *Analyzer) buildRunSummary(run DatasetRun, metrics map[string]MetricResult) RunEvaluationSummary {
	summary := RunEvaluationSummary{
		RunID:           run.RunID,
		Task:            run.Task,
		Scores:          make(map[string]float64),
		Reasons:         make(map[string]string),
		DurationSeconds: run.DurationSeconds,
	}

	// Extract scores and reasons
	passed := true
	var failedMetrics []string

	for metricType, result := range metrics {
		summary.Scores[metricType] = result.Score
		summary.Reasons[metricType] = result.Reason

		if !result.Passed {
			passed = false
			failedMetrics = append(failedMetrics, metricType)
		}
	}

	summary.Passed = passed
	summary.FailedMetrics = failedMetrics

	// Extract tool names
	toolSet := make(map[string]bool)
	for _, tc := range run.ToolCalls {
		if toolName, ok := tc["tool_name"].(string); ok {
			toolSet[toolName] = true
		}
	}
	for tool := range toolSet {
		summary.ToolsUsed = append(summary.ToolsUsed, tool)
	}

	return summary
}

// ============================================================================
// Step 2: Calculate Aggregates
// ============================================================================

func (a *Analyzer) calculateDatasetAggregateScores(results []RunEvaluationSummary) AggregateScores {
	scores := AggregateScores{}
	count := len(results)
	if count == 0 {
		return scores
	}

	var totalHallucination, totalRelevancy, totalCompletion, totalFaithfulness, totalToxicity float64

	for _, result := range results {
		totalHallucination += result.Scores[MetricHallucination]
		totalRelevancy += result.Scores[MetricRelevancy]
		totalCompletion += result.Scores[MetricTaskCompletion]
		totalFaithfulness += result.Scores[MetricFaithfulness]
		totalToxicity += result.Scores[MetricToxicity]
	}

	scores.Hallucination = totalHallucination / float64(count)
	scores.Relevancy = totalRelevancy / float64(count)
	scores.TaskCompletion = totalCompletion / float64(count)
	scores.Faithfulness = totalFaithfulness / float64(count)
	scores.Toxicity = totalToxicity / float64(count)

	return scores
}

func (a *Analyzer) calculateDatasetPassRates(results []RunEvaluationSummary) PassRates {
	rates := PassRates{}
	count := len(results)
	if count == 0 {
		return rates
	}

	var passedHallucination, passedRelevancy, passedCompletion, passedFaithfulness, passedToxicity int

	for _, result := range results {
		if result.Scores[MetricHallucination] <= DefaultThresholds[MetricHallucination] {
			passedHallucination++
		}
		if result.Scores[MetricRelevancy] >= DefaultThresholds[MetricRelevancy] {
			passedRelevancy++
		}
		if result.Scores[MetricTaskCompletion] >= DefaultThresholds[MetricTaskCompletion] {
			passedCompletion++
		}
		if result.Scores[MetricFaithfulness] >= DefaultThresholds[MetricFaithfulness] {
			passedFaithfulness++
		}
		if result.Scores[MetricToxicity] <= DefaultThresholds[MetricToxicity] {
			passedToxicity++
		}
	}

	rates.Hallucination = float64(passedHallucination) / float64(count)
	rates.Relevancy = float64(passedRelevancy) / float64(count)
	rates.TaskCompletion = float64(passedCompletion) / float64(count)
	rates.Faithfulness = float64(passedFaithfulness) / float64(count)
	rates.Toxicity = float64(passedToxicity) / float64(count)

	return rates
}

// ============================================================================
// Step 3: Tool Effectiveness Analysis (LLM-Powered)
// ============================================================================

func (a *Analyzer) analyzeToolEffectiveness(ctx context.Context, runs []DatasetRun, results []RunEvaluationSummary) (ToolEffectivenessAnalysis, error) {
	// Build context for LLM analysis
	analysisContext := a.buildToolAnalysisContext(runs, results)

	// Create prompt for LLM
	prompt := a.buildToolEffectivenessPrompt(analysisContext)

	// Call LLM judge
	var response ToolEffectivenessResponse
	_, _, err := a.judge.Evaluate(ctx, prompt, &response)
	if err != nil {
		return ToolEffectivenessAnalysis{}, fmt.Errorf("tool effectiveness LLM analysis failed: %w", err)
	}

	return ToolEffectivenessAnalysis{
		ToolInsights:        response.ToolInsights,
		EffectiveSequences:  response.EffectiveSequences,
		InefficientPatterns: response.InefficientPatterns,
		Recommendations:     response.Recommendations,
		AnalysisReasoning:   response.Reasoning,
	}, nil
}

type ToolAnalysisContext struct {
	AgentName      string               `json:"agent_name"`
	TotalRuns      int                  `json:"total_runs"`
	ToolUsageStats map[string]ToolStats `json:"tool_usage_stats"`
	RunExamples    []RunExample         `json:"run_examples"`
}

type ToolStats struct {
	UsageCount         int     `json:"usage_count"`
	SuccessfulUses     int     `json:"successful_uses"`
	AvgCompletionScore float64 `json:"avg_completion_score"`
}

type RunExample struct {
	Task            string   `json:"task"`
	ToolSequence    []string `json:"tool_sequence"`
	CompletionScore float64  `json:"completion_score"`
	Success         bool     `json:"success"`
}

type ToolEffectivenessResponse struct {
	ToolInsights        []ToolInsight     `json:"tool_insights"`
	EffectiveSequences  []SequenceInsight `json:"effective_sequences"`
	InefficientPatterns []PatternInsight  `json:"inefficient_patterns"`
	Recommendations     []string          `json:"recommendations"`
	Reasoning           string            `json:"reasoning"`
}

func (a *Analyzer) buildToolAnalysisContext(runs []DatasetRun, results []RunEvaluationSummary) ToolAnalysisContext {
	context := ToolAnalysisContext{
		TotalRuns:      len(runs),
		ToolUsageStats: make(map[string]ToolStats),
		RunExamples:    []RunExample{},
	}

	// Gather tool usage statistics
	for i, run := range runs {
		if i >= len(results) {
			break
		}
		result := results[i]

		// Extract tool sequence
		var toolSeq []string
		for _, tc := range run.ToolCalls {
			if toolName, ok := tc["tool_name"].(string); ok {
				toolSeq = append(toolSeq, toolName)

				// Update tool stats
				stats := context.ToolUsageStats[toolName]
				stats.UsageCount++
				if result.Passed {
					stats.SuccessfulUses++
				}
				stats.AvgCompletionScore += result.Scores[MetricTaskCompletion]
				context.ToolUsageStats[toolName] = stats
			}
		}

		// Add as example (limit to 10 examples)
		if len(context.RunExamples) < 10 {
			context.RunExamples = append(context.RunExamples, RunExample{
				Task:            run.Task,
				ToolSequence:    toolSeq,
				CompletionScore: result.Scores[MetricTaskCompletion],
				Success:         result.Passed,
			})
		}
	}

	// Calculate averages
	for toolName, stats := range context.ToolUsageStats {
		if stats.UsageCount > 0 {
			stats.AvgCompletionScore /= float64(stats.UsageCount)
			context.ToolUsageStats[toolName] = stats
		}
	}

	return context
}

func (a *Analyzer) buildToolEffectivenessPrompt(context ToolAnalysisContext) string {
	contextJSON, _ := json.MarshalIndent(context, "", "  ")

	return fmt.Sprintf(`You are an expert AI agent evaluator analyzing tool usage patterns to identify what makes agents effective.

**Analysis Task:**
Analyze the tool usage data below and provide insights on:
1. Which tools are most/least effective and why
2. Which tool sequences lead to successful task completion
3. Which patterns are inefficient or problematic
4. Specific recommendations to improve agent performance

**Tool Usage Data:**
%s

**Instructions:**
- Focus on NUANCE: Consider task types, tool combinations, and context
- Look for PATTERNS: Which tools work well together? Which cause problems?
- Be SPECIFIC: Don't just say "tool X is good" - explain WHY and WHEN
- Provide ACTIONABLE recommendations for prompt improvements

**Output Format (JSON):**
{
  "tool_insights": [
    {
      "tool_name": "string",
      "usage_count": int,
      "success_rate": float,
      "avg_completion_score": float,
      "insight": "string (detailed analysis of when/why this tool is effective)"
    }
  ],
  "effective_sequences": [
    {
      "sequence": ["tool1", "tool2"],
      "frequency": int,
      "success_rate": float,
      "use_cases": "string (when this sequence works well)"
    }
  ],
  "inefficient_patterns": [
    {
      "pattern": "string (describe the pattern)",
      "frequency": int,
      "success_rate": float,
      "recommendation": "string (how to fix it)"
    }
  ],
  "recommendations": ["string (actionable improvement suggestions)"],
  "reasoning": "string (overall analysis summary)"
}`, string(contextJSON))
}

// ============================================================================
// Step 4: Overall Assessment (LLM-Powered)
// ============================================================================

type OverallAssessment struct {
	Score           float64  `json:"score"` // 0-10
	ProductionReady bool     `json:"production_ready"`
	Recommendation  string   `json:"recommendation"`
	Strengths       []string `json:"strengths"`
	Weaknesses      []string `json:"weaknesses"`
}

func (a *Analyzer) generateOverallAssessment(ctx context.Context, result *DatasetEvaluationResult) (*OverallAssessment, error) {
	prompt := a.buildAssessmentPrompt(result)

	var response OverallAssessment
	_, _, err := a.judge.Evaluate(ctx, prompt, &response)
	if err != nil {
		return nil, fmt.Errorf("overall assessment LLM analysis failed: %w", err)
	}

	return &response, nil
}

func (a *Analyzer) buildAssessmentPrompt(result *DatasetEvaluationResult) string {
	summaryJSON, _ := json.MarshalIndent(map[string]interface{}{
		"agent_name":       result.AgentName,
		"runs_evaluated":   result.RunsEvaluated,
		"aggregate_scores": result.AggregateScores,
		"pass_rates":       result.PassRates,
		"failed_runs":      a.countFailedRuns(result.PerRunResults),
		"tool_analysis":    result.ToolEffectiveness,
	}, "", "  ")

	return fmt.Sprintf(`You are an expert AI agent evaluator providing production readiness assessment.

**Evaluation Summary:**
%s

**Scoring Thresholds:**
- Hallucination: <10%% (lower is better)
- Relevancy: >80%% (higher is better)
- Task Completion: >85%% (higher is better)
- Faithfulness: >85%% (higher is better)
- Toxicity: <5%% (lower is better)

**Your Task:**
Provide an overall assessment with:
1. **Score (0-10)**: Based on quality metrics and production readiness
2. **Production Ready**: true/false - Can this agent be deployed?
3. **Recommendation**: PRODUCTION_READY, CONDITIONAL_GO, NEEDS_IMPROVEMENT, or NOT_READY
4. **Strengths**: List 2-4 key strengths
5. **Weaknesses**: List 2-4 areas for improvement

**Output Format (JSON):**
{
  "score": float (0-10),
  "production_ready": boolean,
  "recommendation": "string",
  "strengths": ["string"],
  "weaknesses": ["string"]
}`, string(summaryJSON))
}

func (a *Analyzer) countFailedRuns(results []RunEvaluationSummary) int {
	count := 0
	for _, r := range results {
		if !r.Passed {
			count++
		}
	}
	return count
}

// ============================================================================
// Cost Calculation
// ============================================================================

func (a *Analyzer) calculateTotalCost(results []RunEvaluationSummary, toolAnalysis ToolEffectivenessAnalysis) float64 {
	// Cost is tracked in MetricResult objects, but we need to sum across all runs
	// For now, estimate based on run count and typical costs
	// TODO: Track actual costs during evaluation
	runCount := len(results)

	// Estimate: ~$0.002 per run evaluation (5 metrics × ~500 tokens × $0.15/1M for gpt-4o-mini)
	runEvalCost := float64(runCount) * 0.002

	// Estimate: ~$0.05 for tool effectiveness analysis (~15k tokens)
	toolAnalysisCost := 0.05

	// Estimate: ~$0.02 for overall assessment (~5k tokens)
	assessmentCost := 0.02

	return runEvalCost + toolAnalysisCost + assessmentCost
}
