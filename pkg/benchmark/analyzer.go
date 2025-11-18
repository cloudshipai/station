package benchmark

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// JaegerClientInterface defines the interface for querying Jaeger traces
type JaegerClientInterface interface {
	QueryRunTrace(runID int64, serviceName string) (*JaegerTrace, error)
	IsAvailable() bool
}

// Analyzer performs benchmark evaluations on agent runs
type Analyzer struct {
	db           *sql.DB
	judge        JudgeInterface
	jaegerClient JaegerClientInterface
	mu           sync.RWMutex
	thresholds   map[string]float64
}

// NewAnalyzer creates a new benchmark analyzer
func NewAnalyzer(db *sql.DB, judge JudgeInterface) *Analyzer {
	return &Analyzer{
		db:         db,
		judge:      judge,
		thresholds: DefaultThresholds,
	}
}

// NewAnalyzerWithJaeger creates a new benchmark analyzer with Jaeger integration
func NewAnalyzerWithJaeger(db *sql.DB, judge JudgeInterface, jaegerClient JaegerClientInterface) *Analyzer {
	return &Analyzer{
		db:           db,
		judge:        judge,
		jaegerClient: jaegerClient,
		thresholds:   DefaultThresholds,
	}
}

// SetThreshold sets a custom threshold for a metric
func (a *Analyzer) SetThreshold(metricType string, threshold float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.thresholds[metricType] = threshold
}

// ============================================================================
// Main Evaluation Entry Point
// ============================================================================

// EvaluateRun performs complete benchmark evaluation on a single agent run
func (a *Analyzer) EvaluateRun(ctx context.Context, runID int64) (*BenchmarkResult, error) {
	startTime := time.Now()

	// 0. Check if metrics already exist for this run
	existingCount := 0
	err := a.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM benchmark_metrics WHERE run_id = ?", runID).Scan(&existingCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check existing metrics: %w", err)
	}

	// If metrics already exist, return existing results instead of re-evaluating
	if existingCount > 0 {
		return a.loadExistingResults(ctx, runID)
	}

	// 1. Load run data from database
	input, err := a.loadRunData(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load run data: %w", err)
	}

	// 2. Extract evidence (tool outputs, traces, contexts)
	if err := a.enrichWithEvidence(ctx, input); err != nil {
		return nil, fmt.Errorf("failed to extract evidence: %w", err)
	}

	// 3. Evaluate quality metrics in parallel
	metrics, err := a.evaluateMetrics(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate metrics: %w", err)
	}

	// 4. Calculate aggregate scores
	result := a.calculateAggregateScores(input, metrics)
	result.EvaluationTimeMS = int(time.Since(startTime).Milliseconds())

	// 5. Store results in database
	if err := a.storeMetrics(ctx, runID, metrics); err != nil {
		return nil, fmt.Errorf("failed to store metrics: %w", err)
	}

	return result, nil
}

// ============================================================================
// Data Loading
// ============================================================================

func (a *Analyzer) loadRunData(ctx context.Context, runID int64) (*EvaluationInput, error) {
	query := `
		SELECT 
			ar.id,
			ar.agent_id,
			ar.task,
			ar.final_response,
			ar.tool_calls,
			ar.execution_steps,
			COALESCE(ar.duration_seconds, 0.0),
			COALESCE(ar.total_tokens, 0),
			ar.status
		FROM agent_runs ar
		WHERE ar.id = ?
	`

	var input EvaluationInput
	var toolCallsJSON, executionStepsJSON sql.NullString

	err := a.db.QueryRowContext(ctx, query, runID).Scan(
		&input.RunID,
		&input.AgentID,
		&input.Task,
		&input.FinalResponse,
		&toolCallsJSON,
		&executionStepsJSON,
		&input.Duration,
		&input.Tokens,
		&input.Status,
	)

	// Cost not tracked in current schema
	input.Cost = 0.0

	if err != nil {
		return nil, fmt.Errorf("failed to query run: %w", err)
	}

	// Note: trace_id column not available in current schema

	if err != nil {
		return nil, fmt.Errorf("failed to query run: %w", err)
	}

	// Parse tool calls JSON
	if toolCallsJSON.Valid && toolCallsJSON.String != "" {
		if err := json.Unmarshal([]byte(toolCallsJSON.String), &input.ToolCalls); err != nil {
			// Log but don't fail - tool calls are optional
			fmt.Printf("Warning: failed to parse tool calls: %v\n", err)
		}
	}

	// Parse execution steps JSON
	if executionStepsJSON.Valid && executionStepsJSON.String != "" {
		if err := json.Unmarshal([]byte(executionStepsJSON.String), &input.ExecutionSteps); err != nil {
			// Log but don't fail - execution steps are optional
			fmt.Printf("Warning: failed to parse execution steps: %v\n", err)
		}
	}

	return &input, nil
}

// ============================================================================
// Evidence Extraction
// ============================================================================

func (a *Analyzer) enrichWithEvidence(ctx context.Context, input *EvaluationInput) error {
	// First try to load tool call data from Jaeger if client is available
	if a.jaegerClient != nil && a.jaegerClient.IsAvailable() {
		trace, err := a.jaegerClient.QueryRunTrace(input.RunID, "station")
		if err == nil && trace != nil {
			// Extract tool calls with full input/output from Jaeger
			jaegerToolCalls := ExtractToolCallsFromTrace(trace)

			// If we got tool calls from Jaeger, use them (they have complete data)
			// Otherwise fall back to database tool calls
			if len(jaegerToolCalls) > 0 {
				input.ToolCalls = jaegerToolCalls
				fmt.Printf("✓ Loaded %d tool calls from Jaeger for run %d\n", len(jaegerToolCalls), input.RunID)
			}
		} else if err != nil {
			// Log but don't fail - Jaeger data is optional
			fmt.Printf("Warning: failed to load Jaeger trace for run %d: %v\n", input.RunID, err)
		}
	}

	// Extract contexts from tool call outputs (for hallucination/faithfulness metrics)
	input.Contexts = a.extractContexts(input.ToolCalls)

	return nil
}

func (a *Analyzer) extractContexts(toolCalls []ToolCall) []string {
	var contexts []string

	for _, call := range toolCalls {
		// Extract meaningful output from tool calls
		if call.Output != nil {
			switch v := call.Output.(type) {
			case string:
				if v != "" && len(v) > 10 { // Ignore trivial outputs
					contexts = append(contexts, fmt.Sprintf("[%s]: %s", call.Name, v))
				}
			case map[string]interface{}:
				// Convert structured output to string
				if jsonBytes, err := json.Marshal(v); err == nil {
					contexts = append(contexts, fmt.Sprintf("[%s]: %s", call.Name, string(jsonBytes)))
				}
			}
		}
	}

	return contexts
}

// ============================================================================
// Metric Evaluation (Parallel)
// ============================================================================

func (a *Analyzer) evaluateMetrics(ctx context.Context, input *EvaluationInput) (map[string]MetricResult, error) {
	results := make(map[string]MetricResult)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, 6) // Buffer for all metrics

	// Define metrics to evaluate
	metrics := []struct {
		name string
		fn   func(context.Context, *EvaluationInput) (MetricResult, error)
	}{
		{MetricHallucination, a.evaluateHallucination},
		{MetricRelevancy, a.evaluateRelevancy},
		{MetricTaskCompletion, a.evaluateTaskCompletion},
		{MetricFaithfulness, a.evaluateFaithfulness},
		{MetricToxicity, a.evaluateToxicity},
	}

	// Evaluate metrics in parallel
	for _, metric := range metrics {
		wg.Add(1)
		go func(name string, fn func(context.Context, *EvaluationInput) (MetricResult, error)) {
			defer wg.Done()

			result, err := fn(ctx, input)
			if err != nil {
				errChan <- fmt.Errorf("%s evaluation failed: %w", name, err)
				return
			}

			mu.Lock()
			results[name] = result
			mu.Unlock()
		}(metric.name, metric.fn)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		return nil, <-errChan
	}

	return results, nil
}

// ============================================================================
// Metric Implementations (Stubs - will be implemented in separate files)
// ============================================================================

func (a *Analyzer) evaluateHallucination(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	evaluator := NewHallucinationEvaluator(a.judge)
	return evaluator.Evaluate(ctx, input)
}

func (a *Analyzer) evaluateRelevancy(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	evaluator := NewRelevancyEvaluator(a.judge)
	return evaluator.Evaluate(ctx, input)
}

func (a *Analyzer) evaluateTaskCompletion(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	evaluator := NewTaskCompletionEvaluator(a.judge)
	return evaluator.Evaluate(ctx, input)
}

func (a *Analyzer) evaluateFaithfulness(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	// Faithfulness uses same logic as hallucination for now
	// TODO: Implement separate faithfulness metric with claims extraction
	evaluator := NewHallucinationEvaluator(a.judge)
	result, err := evaluator.Evaluate(ctx, input)
	if err != nil {
		return MetricResult{}, err
	}
	result.MetricType = MetricFaithfulness
	return result, nil
}

func (a *Analyzer) evaluateToxicity(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	evaluator := NewToxicityEvaluator(a.judge)
	return evaluator.Evaluate(ctx, input)
}

// ============================================================================
// Score Calculation
// ============================================================================

func (a *Analyzer) calculateAggregateScores(input *EvaluationInput, metrics map[string]MetricResult) *BenchmarkResult {
	result := &BenchmarkResult{
		RunID:   input.RunID,
		AgentID: input.AgentID,
		Task:    input.Task,
		Metrics: metrics,
	}

	// Calculate aggregate quality score (0-10)
	var totalScore float64
	var totalTokens int
	var totalCost float64

	for _, metric := range metrics {
		// Normalize scores to 0-10 scale
		// For metrics where lower is better (hallucination, toxicity), invert the score
		var normalizedScore float64
		switch metric.MetricType {
		case MetricHallucination, MetricToxicity, MetricBias:
			// Lower is better: invert score (0.05 hallucination = 9.5/10 quality)
			normalizedScore = (1.0 - metric.Score) * 10.0
		default:
			// Higher is better: direct scaling (0.90 relevancy = 9.0/10 quality)
			normalizedScore = metric.Score * 10.0
		}

		totalScore += normalizedScore
		totalTokens += metric.JudgeTokens
		totalCost += metric.JudgeCost
	}

	result.QualityScore = totalScore / float64(len(metrics))
	result.TotalJudgeTokens = totalTokens
	result.TotalJudgeCost = totalCost

	// Determine production readiness
	allPassed := true
	for _, metric := range metrics {
		if !metric.Passed {
			allPassed = false
			break
		}
	}

	result.ProductionReady = allPassed && result.QualityScore >= 8.0

	if result.ProductionReady {
		result.Recommendation = RecommendationProductionReady
	} else if result.QualityScore >= 7.0 {
		result.Recommendation = RecommendationConditionalGo
	} else if result.QualityScore >= 5.0 {
		result.Recommendation = RecommendationNeedsImprovement
	} else {
		result.Recommendation = RecommendationNotReady
	}

	return result
}

// ============================================================================
// Database Storage
// ============================================================================

func (a *Analyzer) storeMetrics(ctx context.Context, runID int64, metrics map[string]MetricResult) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO benchmark_metrics (
			run_id, metric_type, score, threshold, passed, reason,
			verdicts, evidence, judge_model, judge_tokens, judge_cost, evaluation_duration_ms
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, metric := range metrics {
		verdictsJSON, _ := json.Marshal(metric.Verdicts)
		evidenceJSON, _ := json.Marshal(metric.Evidence)

		_, err := tx.ExecContext(ctx, query,
			runID,
			metric.MetricType,
			metric.Score,
			metric.Threshold,
			metric.Passed,
			metric.Reason,
			string(verdictsJSON),
			string(evidenceJSON),
			"gpt-4o-mini", // Default judge model
			metric.JudgeTokens,
			metric.JudgeCost,
			metric.EvaluationDurationMS,
		)

		if err != nil {
			return fmt.Errorf("failed to insert metric %s: %w", metric.MetricType, err)
		}
	}

	return tx.Commit()
}

// ============================================================================
// Helper Methods
// ============================================================================

// GetRunMetrics retrieves stored benchmark metrics for a run
func (a *Analyzer) GetRunMetrics(ctx context.Context, runID int64) (map[string]MetricResult, error) {
	query := `
		SELECT metric_type, score, threshold, passed, reason, verdicts, evidence,
		       judge_tokens, judge_cost, evaluation_duration_ms
		FROM benchmark_metrics
		WHERE run_id = ?
		ORDER BY created_at
	`

	rows, err := a.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query metrics: %w", err)
	}
	defer rows.Close()

	metrics := make(map[string]MetricResult)

	for rows.Next() {
		var metric MetricResult
		var verdictsJSON, evidenceJSON sql.NullString

		err := rows.Scan(
			&metric.MetricType,
			&metric.Score,
			&metric.Threshold,
			&metric.Passed,
			&metric.Reason,
			&verdictsJSON,
			&evidenceJSON,
			&metric.JudgeTokens,
			&metric.JudgeCost,
			&metric.EvaluationDurationMS,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}

		// Parse JSON fields
		if verdictsJSON.Valid {
			json.Unmarshal([]byte(verdictsJSON.String), &metric.Verdicts)
		}
		if evidenceJSON.Valid {
			json.Unmarshal([]byte(evidenceJSON.String), &metric.Evidence)
		}

		metrics[metric.MetricType] = metric
	}

	return metrics, rows.Err()
}

// loadExistingResults loads pre-existing benchmark results for a run
func (a *Analyzer) loadExistingResults(ctx context.Context, runID int64) (*BenchmarkResult, error) {
	// Load run data
	input, err := a.loadRunData(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load run data: %w", err)
	}

	// Load existing metrics
	metrics, err := a.GetRunMetrics(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing metrics: %w", err)
	}

	// Calculate aggregate scores from existing metrics
	result := a.calculateAggregateScores(input, metrics)

	return result, nil
}
