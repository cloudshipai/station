package services

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"station/internal/config"
	"station/pkg/benchmark"
)

// BenchmarkService provides async benchmark evaluation
type BenchmarkService struct {
	analyzer *benchmark.Analyzer
	tasks    map[string]*BenchmarkTask
	mu       sync.RWMutex
}

// BenchmarkTask tracks async benchmark evaluation
type BenchmarkTask struct {
	TaskID    string
	RunID     int64
	Status    string // pending, running, completed, failed
	StartedAt time.Time
	Result    *benchmark.BenchmarkResult
	Error     error
}

// NewBenchmarkService creates a new benchmark service
func NewBenchmarkService(db *sql.DB, cfg *config.Config) (*BenchmarkService, error) {
	// Create judge for LLM evaluations
	judge, err := benchmark.NewJudge(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create benchmark judge: %w", err)
	}

	// Create analyzer
	analyzer := benchmark.NewAnalyzer(db, judge)

	return &BenchmarkService{
		analyzer: analyzer,
		tasks:    make(map[string]*BenchmarkTask),
	}, nil
}

// EvaluateAsync starts async benchmark evaluation
func (s *BenchmarkService) EvaluateAsync(ctx context.Context, runID int64, taskID string) error {
	s.mu.Lock()
	task := &BenchmarkTask{
		TaskID:    taskID,
		RunID:     runID,
		Status:    "pending",
		StartedAt: time.Now(),
	}
	s.tasks[taskID] = task
	s.mu.Unlock()

	// Run evaluation in background
	go func() {
		s.mu.Lock()
		task.Status = "running"
		s.mu.Unlock()

		// Execute benchmark evaluation
		result, err := s.analyzer.EvaluateRun(context.Background(), runID)

		s.mu.Lock()
		defer s.mu.Unlock()

		if err != nil {
			task.Status = "failed"
			task.Error = err
		} else {
			task.Status = "completed"
			task.Result = result
		}
	}()

	return nil
}

// GetTaskStatus returns the status of a benchmark task
func (s *BenchmarkService) GetTaskStatus(taskID string) (*BenchmarkTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	return task, nil
}

// ListResults returns recent benchmark results
func (s *BenchmarkService) ListResults(limit int, runID *int64) ([]*BenchmarkTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*BenchmarkTask

	for _, task := range s.tasks {
		// Filter by runID if provided
		if runID != nil && task.RunID != *runID {
			continue
		}
		results = append(results, task)

		// Limit results
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// EvaluateDataset performs comprehensive LLM-as-judge evaluation on a dataset
func (s *BenchmarkService) EvaluateDataset(ctx context.Context, input *benchmark.DatasetEvaluationInput) (*benchmark.DatasetEvaluationResult, error) {
	return s.analyzer.EvaluateDataset(ctx, input)
}
