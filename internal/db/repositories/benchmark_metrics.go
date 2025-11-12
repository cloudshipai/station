package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type BenchmarkMetric struct {
	ID                   int64     `json:"id"`
	RunID                int64     `json:"run_id"`
	MetricName           string    `json:"metric_name"`
	Score                float64   `json:"score"`
	Threshold            float64   `json:"threshold"`
	Passed               bool      `json:"passed"`
	Reason               string    `json:"reason"`
	JudgeModel           string    `json:"judge_model,omitempty"`
	JudgeTokens          int       `json:"judge_tokens"`
	JudgeCost            float64   `json:"judge_cost"`
	EvaluationDurationMS int       `json:"evaluation_duration_ms"`
	CreatedAt            time.Time `json:"created_at"`
}

type BenchmarkMetricsRepo struct {
	db *sql.DB
}

func NewBenchmarkMetricsRepo(db *sql.DB) *BenchmarkMetricsRepo {
	return &BenchmarkMetricsRepo{db: db}
}

// GetByRunID retrieves all metrics for a specific run
func (r *BenchmarkMetricsRepo) GetByRunID(ctx context.Context, runID int64) ([]BenchmarkMetric, error) {
	query := `
		SELECT id, run_id, metric_type, score, threshold, passed, reason, 
		       COALESCE(judge_model, ''), COALESCE(judge_tokens, 0), 
		       COALESCE(judge_cost, 0.0), COALESCE(evaluation_duration_ms, 0), created_at
		FROM benchmark_metrics
		WHERE run_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark metrics: %w", err)
	}
	defer rows.Close()

	var metrics []BenchmarkMetric
	for rows.Next() {
		var m BenchmarkMetric
		err := rows.Scan(
			&m.ID,
			&m.RunID,
			&m.MetricName,
			&m.Score,
			&m.Threshold,
			&m.Passed,
			&m.Reason,
			&m.JudgeModel,
			&m.JudgeTokens,
			&m.JudgeCost,
			&m.EvaluationDurationMS,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics: %w", err)
	}

	return metrics, nil
}

// ListRecent retrieves recent benchmark metrics across all runs
func (r *BenchmarkMetricsRepo) ListRecent(ctx context.Context, limit int) ([]BenchmarkMetric, error) {
	query := `
		SELECT id, run_id, metric_type, score, threshold, passed, reason,
		       COALESCE(judge_model, ''), COALESCE(judge_tokens, 0), 
		       COALESCE(judge_cost, 0.0), COALESCE(evaluation_duration_ms, 0), created_at
		FROM benchmark_metrics
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent metrics: %w", err)
	}
	defer rows.Close()

	var metrics []BenchmarkMetric
	for rows.Next() {
		var m BenchmarkMetric
		err := rows.Scan(
			&m.ID,
			&m.RunID,
			&m.MetricName,
			&m.Score,
			&m.Threshold,
			&m.Passed,
			&m.Reason,
			&m.JudgeModel,
			&m.JudgeTokens,
			&m.JudgeCost,
			&m.EvaluationDurationMS,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan metric: %w", err)
		}
		metrics = append(metrics, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating metrics: %w", err)
	}

	return metrics, nil
}
