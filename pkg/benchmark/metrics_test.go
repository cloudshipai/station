package benchmark

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// MockJudge implements Judge for testing without real LLM calls
type MockJudge struct {
	responses map[string]string // Map prompt substring to response
}

func NewMockJudge() *MockJudge {
	return &MockJudge{
		responses: make(map[string]string),
	}
}

func (m *MockJudge) Evaluate(ctx context.Context, prompt string, output interface{}) (int, float64, error) {
	// Find matching response based on prompt content
	var response string
	for key, resp := range m.responses {
		if strings.Contains(prompt, key) {
			response = resp
			break
		}
	}

	if response == "" {
		response = `{"verdict": 0.8, "reason": "Mock response"}`
	}

	// Parse JSON into output
	err := json.Unmarshal([]byte(response), output)
	return 100, 0.0001, err
}

func (m *MockJudge) GetModelName() string {
	return "mock-model"
}

func (m *MockJudge) SetResponse(key, response string) {
	m.responses[key] = response
}

// =============================================================================
// Task Completion Tests
// =============================================================================

func TestTaskCompletionEvaluator(t *testing.T) {
	judge := NewMockJudge()
	judge.SetResponse("Task:", `{"verdict": 0.95, "reason": "Task completed successfully"}`)

	evaluator := NewTaskCompletionEvaluator(judge)
	input := &EvaluationInput{
		Task:          "Scan terraform files",
		FinalResponse: "Found 47 vulnerabilities in 15 terraform files",
		Status:        "completed",
		Duration:      2.3,
	}

	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	if result.MetricType != MetricTaskCompletion {
		t.Errorf("Expected metric type %s, got %s", MetricTaskCompletion, result.MetricType)
	}

	if result.Score != 0.95 {
		t.Errorf("Expected score 0.95, got %.2f", result.Score)
	}

	if !result.Passed {
		t.Error("Expected metric to pass")
	}
}

func TestTaskCompletionSummarizeToolCalls(t *testing.T) {
	evaluator := NewTaskCompletionEvaluator(nil)

	toolCalls := []ToolCall{
		{Name: "checkov"},
		{Name: "trivy"},
		{Name: "checkov"},
	}

	summary := evaluator.summarizeToolCalls(toolCalls)

	// Should show checkov(2) and trivy
	if !strings.Contains(summary, "checkov(2)") {
		t.Errorf("Expected 'checkov(2)' in summary, got: %s", summary)
	}
	if !strings.Contains(summary, "trivy") {
		t.Errorf("Expected 'trivy' in summary, got: %s", summary)
	}
}

// =============================================================================
// Relevancy Tests
// =============================================================================

func TestRelevancyEvaluator(t *testing.T) {
	judge := NewMockJudge()
	judge.SetResponse("Task:", `{
		"verdicts": [
			{"statement": "Found vulnerabilities", "verdict": "yes"},
			{"statement": "Scan took 2 minutes", "verdict": "idk", "reason": "Not directly relevant"}
		]
	}`)

	evaluator := NewRelevancyEvaluator(judge)
	input := &EvaluationInput{
		Task:          "Find security issues",
		FinalResponse: "Found 47 vulnerabilities. Scan took 2 minutes.",
	}

	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// Score should be 1/2 = 0.50 (only "yes" counts as relevant)
	expectedScore := 0.50
	if result.Score != expectedScore {
		t.Errorf("Expected score %.2f, got %.2f", expectedScore, result.Score)
	}

	if len(result.Verdicts) != 2 {
		t.Errorf("Expected 2 verdicts, got %d", len(result.Verdicts))
	}
}

func TestRelevancyCalculateScore(t *testing.T) {
	evaluator := NewRelevancyEvaluator(nil)

	tests := []struct {
		name     string
		verdicts []RelevancyVerdict
		expected float64
	}{
		{
			name:     "empty",
			verdicts: []RelevancyVerdict{},
			expected: 1.0, // No statements = fully relevant
		},
		{
			name: "all relevant",
			verdicts: []RelevancyVerdict{
				{Verdict: "yes"},
				{Verdict: "yes"},
			},
			expected: 1.0,
		},
		{
			name: "half relevant",
			verdicts: []RelevancyVerdict{
				{Verdict: "yes"},
				{Verdict: "no"},
			},
			expected: 0.5,
		},
		{
			name: "with idk",
			verdicts: []RelevancyVerdict{
				{Verdict: "yes"},
				{Verdict: "idk"},
				{Verdict: "no"},
			},
			expected: 0.333, // Only "yes" counts
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := evaluator.calculateScore(tt.verdicts)
			if score < tt.expected-0.01 || score > tt.expected+0.01 {
				t.Errorf("Expected score ~%.3f, got %.3f", tt.expected, score)
			}
		})
	}
}

// =============================================================================
// Hallucination Tests
// =============================================================================

func TestHallucinationEvaluator(t *testing.T) {
	judge := NewMockJudge()
	judge.SetResponse("Contexts", `{
		"verdicts": [
			{"verdict": "yes", "reason": "Output agrees with context"},
			{"verdict": "no", "reason": "Output contradicts - says 50 but context shows 47"}
		]
	}`)

	evaluator := NewHallucinationEvaluator(judge)
	input := &EvaluationInput{
		FinalResponse: "Found 50 vulnerabilities",
		Contexts: []string{
			"Found 47 vulnerabilities",
			"Scan completed successfully",
		},
	}

	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// Score should be 1/2 = 0.50 (50% hallucination rate)
	expectedScore := 0.50
	if result.Score != expectedScore {
		t.Errorf("Expected score %.2f, got %.2f", expectedScore, result.Score)
	}

	// Should fail threshold (0.50 > 0.10)
	if result.Passed {
		t.Error("Expected metric to fail with high hallucination rate")
	}
}

func TestHallucinationNoContexts(t *testing.T) {
	evaluator := NewHallucinationEvaluator(NewMockJudge())
	input := &EvaluationInput{
		FinalResponse: "Test response",
		Contexts:      []string{},
	}

	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// No contexts = cannot evaluate = assume safe
	if result.Score != 0.0 {
		t.Errorf("Expected score 0.0 with no contexts, got %.2f", result.Score)
	}

	if !result.Passed {
		t.Error("Expected metric to pass with no contexts")
	}
}

// =============================================================================
// Toxicity Tests
// =============================================================================

func TestToxicityEvaluator(t *testing.T) {
	judge := NewMockJudge()
	judge.SetResponse("Output:", `{
		"verdicts": [
			{"opinion": "Found security issues", "verdict": "no"},
			{"opinion": "Your code is terrible", "verdict": "yes", "reason": "Insulting language"}
		]
	}`)

	evaluator := NewToxicityEvaluator(judge)
	input := &EvaluationInput{
		FinalResponse: "Found security issues. Your code is terrible.",
	}

	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// Score should be 1/2 = 0.50 (50% toxic)
	expectedScore := 0.50
	if result.Score != expectedScore {
		t.Errorf("Expected score %.2f, got %.2f", expectedScore, result.Score)
	}

	// Should fail threshold (0.50 > 0.05)
	if result.Passed {
		t.Error("Expected metric to fail with high toxicity")
	}
}

func TestToxicitySafeContent(t *testing.T) {
	judge := NewMockJudge()
	judge.SetResponse("Output:", `{
		"verdicts": [
			{"opinion": "Found 47 vulnerabilities", "verdict": "no"},
			{"opinion": "Recommend fixing critical issues", "verdict": "no"}
		]
	}`)

	evaluator := NewToxicityEvaluator(judge)
	input := &EvaluationInput{
		FinalResponse: "Found 47 vulnerabilities. Recommend fixing critical issues.",
	}

	result, err := evaluator.Evaluate(context.Background(), input)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// No toxic content
	if result.Score != 0.0 {
		t.Errorf("Expected score 0.0, got %.2f", result.Score)
	}

	if !result.Passed {
		t.Error("Expected safe content to pass")
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "needs truncation",
			input:    "hello world",
			maxLen:   5,
			expected: "hello...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatVerdict(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.95, "excellent"},
		{0.90, "excellent"},
		{0.85, "good"},
		{0.70, "good"},
		{0.60, "partial"},
		{0.50, "partial"},
		{0.40, "poor"},
	}

	for _, tt := range tests {
		result := formatVerdict(tt.score)
		if result != tt.expected {
			t.Errorf("Score %.2f: expected '%s', got '%s'", tt.score, tt.expected, result)
		}
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkTaskCompletionEvaluate(b *testing.B) {
	judge := NewMockJudge()
	judge.SetResponse("Task:", `{"verdict": 0.95, "reason": "Task completed"}`)

	evaluator := NewTaskCompletionEvaluator(judge)
	input := &EvaluationInput{
		Task:          "Test task",
		FinalResponse: "Test response",
		ToolCalls:     []ToolCall{{Name: "tool1"}, {Name: "tool2"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.Evaluate(context.Background(), input)
	}
}

func BenchmarkRelevancyCalculateScore(b *testing.B) {
	evaluator := NewRelevancyEvaluator(nil)
	verdicts := []RelevancyVerdict{
		{Verdict: "yes"},
		{Verdict: "yes"},
		{Verdict: "no"},
		{Verdict: "idk"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = evaluator.calculateScore(verdicts)
	}
}
