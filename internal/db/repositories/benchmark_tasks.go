package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type BenchmarkTask struct {
	ID                    int64
	Name                  string
	Category              string
	Description           string
	ExpectedOutputExample string
	EvaluationCriteria    string
	TaskCompletionWeight  float64
	RelevancyWeight       float64
	HallucinationWeight   float64
	FaithfulnessWeight    float64
	ToxicityWeight        float64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type BenchmarkTasksRepo struct {
	db *sql.DB
}

func NewBenchmarkTasksRepo(db *sql.DB) *BenchmarkTasksRepo {
	return &BenchmarkTasksRepo{db: db}
}

// GetAll retrieves all benchmark tasks
func (r *BenchmarkTasksRepo) GetAll(ctx context.Context) ([]BenchmarkTask, error) {
	query := `
		SELECT 
			id, name, category, description, 
			expected_output_example, evaluation_criteria,
			task_completion_weight, relevancy_weight, 
			hallucination_weight, faithfulness_weight, toxicity_weight,
			created_at, updated_at
		FROM benchmark_tasks
		ORDER BY category, name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark tasks: %w", err)
	}
	defer rows.Close()

	var tasks []BenchmarkTask
	for rows.Next() {
		var t BenchmarkTask
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Category,
			&t.Description,
			&t.ExpectedOutputExample,
			&t.EvaluationCriteria,
			&t.TaskCompletionWeight,
			&t.RelevancyWeight,
			&t.HallucinationWeight,
			&t.FaithfulnessWeight,
			&t.ToxicityWeight,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}

// GetByID retrieves a benchmark task by ID
func (r *BenchmarkTasksRepo) GetByID(ctx context.Context, id int64) (*BenchmarkTask, error) {
	query := `
		SELECT 
			id, name, category, description, 
			expected_output_example, evaluation_criteria,
			task_completion_weight, relevancy_weight, 
			hallucination_weight, faithfulness_weight, toxicity_weight,
			created_at, updated_at
		FROM benchmark_tasks
		WHERE id = ?
	`

	var t BenchmarkTask
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID,
		&t.Name,
		&t.Category,
		&t.Description,
		&t.ExpectedOutputExample,
		&t.EvaluationCriteria,
		&t.TaskCompletionWeight,
		&t.RelevancyWeight,
		&t.HallucinationWeight,
		&t.FaithfulnessWeight,
		&t.ToxicityWeight,
		&t.CreatedAt,
		&t.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("benchmark task with ID %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get benchmark task: %w", err)
	}

	return &t, nil
}

// GetByCategory retrieves benchmark tasks by category
func (r *BenchmarkTasksRepo) GetByCategory(ctx context.Context, category string) ([]BenchmarkTask, error) {
	query := `
		SELECT 
			id, name, category, description, 
			expected_output_example, evaluation_criteria,
			task_completion_weight, relevancy_weight, 
			hallucination_weight, faithfulness_weight, toxicity_weight,
			created_at, updated_at
		FROM benchmark_tasks
		WHERE category = ?
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query benchmark tasks: %w", err)
	}
	defer rows.Close()

	var tasks []BenchmarkTask
	for rows.Next() {
		var t BenchmarkTask
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Category,
			&t.Description,
			&t.ExpectedOutputExample,
			&t.EvaluationCriteria,
			&t.TaskCompletionWeight,
			&t.RelevancyWeight,
			&t.HallucinationWeight,
			&t.FaithfulnessWeight,
			&t.ToxicityWeight,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tasks: %w", err)
	}

	return tasks, nil
}
