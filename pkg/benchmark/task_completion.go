package benchmark

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// Task Completion Metric
// ============================================================================
// Evaluates whether an agent successfully completed the intended task
// Score: 0.0 to 1.0 (higher is better, threshold: 0.85)
// ============================================================================

// TaskCompletionEvaluator implements the task completion metric
type TaskCompletionEvaluator struct {
	judge     JudgeInterface
	threshold float64
}

// NewTaskCompletionEvaluator creates a new task completion evaluator
func NewTaskCompletionEvaluator(judge JudgeInterface) *TaskCompletionEvaluator {
	return &TaskCompletionEvaluator{
		judge:     judge,
		threshold: DefaultThresholds[MetricTaskCompletion],
	}
}

// Evaluate performs task completion evaluation
func (e *TaskCompletionEvaluator) Evaluate(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	// Extract task and outcome
	task := input.Task
	if task == "" {
		task = "Complete the user's request" // Fallback if no explicit task
	}

	outcome := e.extractOutcome(input)

	// Build evaluation prompt
	prompt := e.buildPrompt(task, outcome, input)

	// Call LLM judge
	var response TaskCompletionResponse
	tokens, cost, err := e.judge.Evaluate(ctx, prompt, &response)
	if err != nil {
		return MetricResult{}, fmt.Errorf("task completion evaluation failed: %w", err)
	}

	// Build result
	result := MetricResult{
		MetricType:  MetricTaskCompletion,
		Score:       response.Verdict,
		Threshold:   e.threshold,
		Passed:      response.Verdict >= e.threshold,
		Reason:      response.Reason,
		JudgeTokens: tokens,
		JudgeCost:   cost,
	}

	// Add verdicts (single verdict for task completion)
	result.Verdicts = []Verdict{
		{
			Statement: fmt.Sprintf("Task: %s", task),
			Verdict:   formatVerdict(response.Verdict),
			Reason:    response.Reason,
		},
	}

	return result, nil
}

// extractOutcome builds a description of what the agent actually did
func (e *TaskCompletionEvaluator) extractOutcome(input *EvaluationInput) string {
	var parts []string

	// Include final response
	if input.FinalResponse != "" {
		parts = append(parts, fmt.Sprintf("Response: %s", truncate(input.FinalResponse, 500)))
	}

	// Include tool usage summary
	if len(input.ToolCalls) > 0 {
		toolSummary := e.summarizeToolCalls(input.ToolCalls)
		parts = append(parts, fmt.Sprintf("Tools used: %s", toolSummary))
	}

	// Include execution status
	if input.Status != "" {
		parts = append(parts, fmt.Sprintf("Status: %s", input.Status))
	}

	return strings.Join(parts, ". ")
}

// summarizeToolCalls creates a concise summary of tools used
func (e *TaskCompletionEvaluator) summarizeToolCalls(toolCalls []ToolCall) string {
	if len(toolCalls) == 0 {
		return "none"
	}

	// Count tool usage
	toolCounts := make(map[string]int)
	for _, call := range toolCalls {
		toolCounts[call.Name]++
	}

	// Build summary
	var summary []string
	for name, count := range toolCounts {
		if count == 1 {
			summary = append(summary, name)
		} else {
			summary = append(summary, fmt.Sprintf("%s(%d)", name, count))
		}
	}

	return strings.Join(summary, ", ")
}

// buildPrompt constructs the LLM evaluation prompt
func (e *TaskCompletionEvaluator) buildPrompt(task, outcome string, input *EvaluationInput) string {
	return fmt.Sprintf(`You are evaluating whether an AI agent successfully completed a task.

TASK: Compare the desired task with what the agent actually accomplished and assign a completion score.

IMPORTANT:
- Score from 0.0 (completely failed) to 1.0 (perfectly completed)
- Be objective - focus on whether the task was accomplished, not on how it was done
- Partial completion should receive partial scores (e.g., 0.7 for 70%% complete)
- Return ONLY valid JSON with no markdown formatting

EXAMPLE:
Task: Scan terraform files for security vulnerabilities
Outcome: The agent scanned 15 terraform files and found 47 vulnerabilities including SQL injection risks and exposed secrets
Example JSON:
{
  "verdict": 0.95,
  "reason": "The agent successfully completed the task by scanning terraform files and identifying security vulnerabilities. Achieved 95%% completion as vulnerabilities were found and reported clearly."
}

NOW EVALUATE:

Task:
%s

Actual Outcome:
%s

Duration: %.2f seconds
Tokens used: %d
Cost: $%.4f

Return JSON with 'verdict' (0.0 to 1.0) and 'reason':
{
  "verdict": 0.0,
  "reason": "Your explanation here"
}

JSON:`, task, outcome, input.Duration, input.Tokens, input.Cost)
}

// ============================================================================
// Response Types
// ============================================================================

// TaskCompletionResponse represents the LLM's task completion judgment
type TaskCompletionResponse struct {
	Verdict float64 `json:"verdict"` // 0.0 to 1.0
	Reason  string  `json:"reason"`
}

// ============================================================================
// Helper Functions
// ============================================================================

func formatVerdict(score float64) string {
	if score >= 0.9 {
		return "excellent"
	} else if score >= 0.7 {
		return "good"
	} else if score >= 0.5 {
		return "partial"
	} else {
		return "poor"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
