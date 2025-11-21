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

IMPORTANT EVALUATION PRINCIPLES:
- Score from 0.0 (completely failed) to 1.0 (perfectly completed)
- Be GENEROUS and CONTEXT-AWARE: The agent can only work with the data available from tools
- REWARD EFFORT: If the agent called appropriate tools and worked with what they returned, give significant credit
- PARTIAL CREDIT: Completion percentage should map to score (70%% done = 0.7 score minimum)
- Don't penalize the agent for limitations of the underlying data sources or missing tool capabilities
- For "what if" or hypothetical questions, good explanations ARE valid task completions
- If the agent made meaningful progress toward the goal, score should be ≥0.7
- Focus on whether the core intent of the task was addressed, not perfection
- Return ONLY valid JSON with no markdown formatting

SCORING GUIDELINES:
- 0.9-1.0: Excellent - Task fully completed with high quality
- 0.7-0.8: Good - Task substantially completed (this should be common!)
- 0.5-0.6: Acceptable - Core task addressed, some limitations
- 0.3-0.4: Minimal - Some relevant work done but incomplete
- 0.0-0.2: Failed - Did not accomplish the task or went off-track

DEFAULT TO GENEROSITY: When in doubt between two score ranges, choose the higher one.

EXAMPLE 1:
Task: Get the status of the last 10 builds for 'frontend' job
Outcome: Agent called get_job tool and returned status for 3 builds with detailed information
Example JSON:
{
  "verdict": 0.5,
  "reason": "Agent successfully called the tool and returned build statuses, but only provided 3 out of 10 requested builds. This appears to be a data limitation rather than agent failure. Partial completion: 50%%."
}

EXAMPLE 2:
Task: What happens if I query a non-existent job?
Outcome: Agent explained that it would return an error, but didn't actually test it
Example JSON:
{
  "verdict": 0.6,
  "reason": "Agent provided a conceptually correct explanation of what would happen. While testing the actual behavior would be ideal, the explanation addresses the question's core intent. Good theoretical completion."
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
