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
	db       *sql.DB
	cfg      *config.Config
	analyzer *benchmark.Analyzer
	tasks    map[string]*BenchmarkTask
	mu       sync.RWMutex
	once     sync.Once
	initErr  error
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

// NewBenchmarkService creates a new benchmark service with lazy initialization
func NewBenchmarkService(db *sql.DB, cfg *config.Config) (*BenchmarkService, error) {
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("database and config are required")
	}

	// Create service with deferred initialization
	// This allows server to start even if AI provider credentials are missing
	return &BenchmarkService{
		db:    db,
		cfg:   cfg,
		tasks: make(map[string]*BenchmarkTask),
	}, nil
}

// getAnalyzer initializes the analyzer on first use (lazy initialization)
func (s *BenchmarkService) getAnalyzer() (*benchmark.Analyzer, error) {
	s.once.Do(func() {
		// Create judge for LLM evaluations
		judge, err := benchmark.NewJudge(s.cfg)
		if err != nil {
			s.initErr = fmt.Errorf("failed to create benchmark judge: %w", err)
			return
		}

		// Create Jaeger client for trace querying
		if s.cfg.JaegerQueryURL != "" {
			jaegerClient := NewJaegerClient(s.cfg.JaegerQueryURL)
			if jaegerClient.IsAvailable() {
				// Create analyzer with Jaeger integration using adapter
				adapter := NewBenchmarkJaegerAdapter(jaegerClient)
				s.analyzer = benchmark.NewAnalyzerWithJaeger(s.db, judge, adapter)
				fmt.Printf("✓ Benchmark service initialized with Jaeger integration (%s)\n", s.cfg.JaegerQueryURL)
			} else {
				// Jaeger not available, use analyzer without it
				s.analyzer = benchmark.NewAnalyzer(s.db, judge)
				fmt.Printf("⚠ Jaeger not available at %s, continuing without trace data\n", s.cfg.JaegerQueryURL)
			}
		} else {
			// No Jaeger URL configured
			s.analyzer = benchmark.NewAnalyzer(s.db, judge)
		}
	})

	if s.initErr != nil {
		return nil, s.initErr
	}
	return s.analyzer, nil
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

		// Get analyzer (lazy initialization)
		analyzer, err := s.getAnalyzer()
		if err != nil {
			s.mu.Lock()
			task.Status = "failed"
			task.Error = fmt.Errorf("benchmark service not available: %w", err)
			s.mu.Unlock()
			return
		}

		// Execute benchmark evaluation
		result, err := analyzer.EvaluateRun(context.Background(), runID)

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
