package benchmark

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// TestAnalyzerCreation tests that we can create an analyzer
func TestAnalyzerCreation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	analyzer := NewAnalyzer(db, nil)
	if analyzer == nil {
		t.Fatal("Expected analyzer to be created")
	}

	if analyzer.db != db {
		t.Error("Analyzer should store database reference")
	}

	if len(analyzer.thresholds) == 0 {
		t.Error("Analyzer should have default thresholds")
	}
}

// TestDefaultThresholds verifies default threshold values
func TestDefaultThresholds(t *testing.T) {
	tests := []struct {
		metric      string
		expected    float64
		lowerBetter bool
	}{
		{MetricHallucination, 0.10, true},
		{MetricRelevancy, 0.80, false},
		{MetricTaskCompletion, 0.70, false},
		{MetricFaithfulness, 0.85, false},
		{MetricToxicity, 0.05, true},
	}

	for _, tt := range tests {
		t.Run(tt.metric, func(t *testing.T) {
			threshold, exists := DefaultThresholds[tt.metric]
			if !exists {
				t.Errorf("Threshold for %s not defined", tt.metric)
			}
			if threshold != tt.expected {
				t.Errorf("Expected threshold %.2f, got %.2f", tt.expected, threshold)
			}
		})
	}
}

// TestLoadRunData tests loading run data from database
func TestLoadRunData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert test run
	runID := insertTestRun(t, db, &testRunData{
		AgentID:       1,
		Task:          "Test task",
		FinalResponse: "Test response",
		Status:        "completed",
		Duration:      1.5,
		TotalTokens:   100,
		TotalCost:     0.001,
	})

	analyzer := NewAnalyzer(db, nil)
	input, err := analyzer.loadRunData(context.Background(), runID)
	if err != nil {
		t.Fatalf("Failed to load run data: %v", err)
	}

	if input.RunID != runID {
		t.Errorf("Expected RunID %d, got %d", runID, input.RunID)
	}
	if input.Task != "Test task" {
		t.Errorf("Expected task 'Test task', got '%s'", input.Task)
	}
	if input.FinalResponse != "Test response" {
		t.Errorf("Expected response 'Test response', got '%s'", input.FinalResponse)
	}
}

// TestExtractContexts tests context extraction from tool calls
func TestExtractContexts(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	toolCalls := []ToolCall{
		{
			Name:   "checkov_scan",
			Output: "Found 47 security vulnerabilities",
		},
		{
			Name:   "trivy_scan",
			Output: "Container has 3 critical CVEs",
		},
		{
			Name:   "short", // Too short, should be ignored
			Output: "OK",
		},
	}

	contexts := analyzer.extractContexts(toolCalls)

	if len(contexts) != 2 {
		t.Errorf("Expected 2 contexts, got %d", len(contexts))
	}

	if len(contexts) > 0 && contexts[0] != "[checkov_scan]: Found 47 security vulnerabilities" {
		t.Errorf("Unexpected context[0]: %s", contexts[0])
	}
}

// TestCalculateAggregateScores tests score calculation
func TestCalculateAggregateScores(t *testing.T) {
	analyzer := NewAnalyzer(nil, nil)

	input := &EvaluationInput{
		RunID:   1,
		AgentID: 1,
		Task:    "Test task",
	}

	metrics := map[string]MetricResult{
		MetricHallucination: {
			MetricType:  MetricHallucination,
			Score:       0.05,
			Threshold:   0.10,
			Passed:      true,
			JudgeTokens: 100,
			JudgeCost:   0.0001,
		},
		MetricRelevancy: {
			MetricType:  MetricRelevancy,
			Score:       0.90,
			Threshold:   0.80,
			Passed:      true,
			JudgeTokens: 120,
			JudgeCost:   0.0002,
		},
		MetricTaskCompletion: {
			MetricType:  MetricTaskCompletion,
			Score:       0.85,
			Threshold:   0.85,
			Passed:      true,
			JudgeTokens: 150,
			JudgeCost:   0.0002,
		},
	}

	result := analyzer.calculateAggregateScores(input, metrics)

	// Check aggregate values
	if result.TotalJudgeTokens != 370 {
		t.Errorf("Expected 370 total tokens, got %d", result.TotalJudgeTokens)
	}

	expectedCost := 0.0005
	if result.TotalJudgeCost < expectedCost-0.0001 || result.TotalJudgeCost > expectedCost+0.0001 {
		t.Errorf("Expected cost ~%.4f, got %.4f", expectedCost, result.TotalJudgeCost)
	}

	// Quality score should be calculated correctly
	// Hallucination: (1-0.05)*10 = 9.5
	// Relevancy: 0.90*10 = 9.0
	// TaskCompletion: 0.85*10 = 8.5
	// Average: (9.5+9.0+8.5)/3 = 9.0
	expectedQuality := 9.0
	if result.QualityScore < expectedQuality-0.1 || result.QualityScore > expectedQuality+0.1 {
		t.Errorf("Expected quality score ~%.1f, got %.1f", expectedQuality, result.QualityScore)
	}

	// All passed, score >= 8.0 → should be production ready
	if !result.ProductionReady {
		t.Error("Expected ProductionReady to be true")
	}

	if result.Recommendation != RecommendationProductionReady {
		t.Errorf("Expected recommendation %s, got %s", RecommendationProductionReady, result.Recommendation)
	}
}

// TestStoreAndRetrieveMetrics tests database operations
func TestStoreAndRetrieveMetrics(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	runID := insertTestRun(t, db, &testRunData{
		AgentID:       1,
		Task:          "Test task",
		FinalResponse: "Test response",
		Status:        "completed",
	})

	analyzer := NewAnalyzer(db, nil)

	// Create test metrics
	metrics := map[string]MetricResult{
		MetricHallucination: {
			MetricType: MetricHallucination,
			Score:      0.05,
			Threshold:  0.10,
			Passed:     true,
			Reason:     "No hallucinations detected",
			Verdicts: []Verdict{
				{Statement: "Test", Verdict: "yes", Reason: "Agrees with context"},
			},
			JudgeTokens:          100,
			JudgeCost:            0.0001,
			EvaluationDurationMS: 500,
		},
	}

	// Store metrics
	err := analyzer.storeMetrics(context.Background(), runID, metrics)
	if err != nil {
		t.Fatalf("Failed to store metrics: %v", err)
	}

	// Retrieve metrics
	retrieved, err := analyzer.GetRunMetrics(context.Background(), runID)
	if err != nil {
		t.Fatalf("Failed to retrieve metrics: %v", err)
	}

	if len(retrieved) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(retrieved))
	}

	metric, exists := retrieved[MetricHallucination]
	if !exists {
		t.Fatal("Hallucination metric not found")
	}

	if metric.Score != 0.05 {
		t.Errorf("Expected score 0.05, got %.2f", metric.Score)
	}

	if !metric.Passed {
		t.Error("Expected metric to have passed")
	}

	if len(metric.Verdicts) != 1 {
		t.Errorf("Expected 1 verdict, got %d", len(metric.Verdicts))
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

type testRunData struct {
	AgentID       int64
	Task          string
	FinalResponse string
	ToolCalls     []ToolCall
	Status        string
	Duration      float64
	TotalTokens   int
	TotalCost     float64
}

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create minimal schema for testing
	schema := `
	CREATE TABLE agents (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT
	);

	CREATE TABLE agent_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id INTEGER NOT NULL,
		task TEXT NOT NULL,
		final_response TEXT,
		tool_calls TEXT,
		execution_steps TEXT,
		trace_id TEXT,
		duration_seconds REAL,
		total_tokens INTEGER,
		total_cost REAL,
		status TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (agent_id) REFERENCES agents(id)
	);

	CREATE TABLE benchmark_metrics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER NOT NULL,
		metric_type TEXT NOT NULL,
		score REAL NOT NULL,
		threshold REAL NOT NULL,
		passed BOOLEAN NOT NULL,
		reason TEXT,
		verdicts TEXT,
		evidence TEXT,
		judge_model TEXT DEFAULT 'gpt-4o-mini',
		judge_tokens INTEGER DEFAULT 0,
		judge_cost REAL DEFAULT 0.0,
		evaluation_duration_ms INTEGER,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (run_id) REFERENCES agent_runs(id)
	);

	INSERT INTO agents (id, name) VALUES (1, 'test-agent');
	`

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	return db
}

func insertTestRun(t *testing.T, db *sql.DB, data *testRunData) int64 {
	var toolCallsJSON string

	if len(data.ToolCalls) > 0 {
		bytes, _ := json.Marshal(data.ToolCalls)
		toolCallsJSON = string(bytes)
	}

	query := `
		INSERT INTO agent_runs (
			agent_id, task, final_response, tool_calls, status,
			duration_seconds, total_tokens, total_cost
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		data.AgentID,
		data.Task,
		data.FinalResponse,
		toolCallsJSON,
		data.Status,
		data.Duration,
		data.TotalTokens,
		data.TotalCost,
	)

	if err != nil {
		t.Fatalf("Failed to insert test run: %v", err)
	}

	id, _ := result.LastInsertId()
	return id
}

// Benchmark tests
func BenchmarkExtractContexts(b *testing.B) {
	analyzer := NewAnalyzer(nil, nil)
	toolCalls := []ToolCall{
		{Name: "tool1", Output: "This is a test output with some content"},
		{Name: "tool2", Output: "Another output with meaningful data"},
		{Name: "tool3", Output: "Third tool output for benchmarking"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.extractContexts(toolCalls)
	}
}

func BenchmarkCalculateAggregateScores(b *testing.B) {
	analyzer := NewAnalyzer(nil, nil)
	input := &EvaluationInput{RunID: 1, AgentID: 1, Task: "Test"}
	metrics := map[string]MetricResult{
		MetricHallucination:  {Score: 0.05, Threshold: 0.10, Passed: true},
		MetricRelevancy:      {Score: 0.90, Threshold: 0.80, Passed: true},
		MetricTaskCompletion: {Score: 0.85, Threshold: 0.70, Passed: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		analyzer.calculateAggregateScores(input, metrics)
	}
}
