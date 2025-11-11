package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type BenchmarkMetric struct {
	ID         int64
	RunID      int64
	MetricName string
	Score      float64
	Threshold  float64
	Passed     bool
	Reason     string
	CreatedAt  time.Time
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
		SELECT id, run_id, metric_name, score, threshold, passed, reason, created_at
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
		SELECT id, run_id, metric_name, score, threshold, passed, reason, created_at
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
