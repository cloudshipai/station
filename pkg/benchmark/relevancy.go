package benchmark

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// Answer Relevancy Metric
// ============================================================================
// Evaluates whether the agent's output is relevant to the user's input/task
// Score: 0.0 to 1.0 (higher is better, threshold: 0.80)
// ============================================================================

// RelevancyEvaluator implements the answer relevancy metric
type RelevancyEvaluator struct {
	judge     JudgeInterface
	threshold float64
}

// NewRelevancyEvaluator creates a new relevancy evaluator
func NewRelevancyEvaluator(judge JudgeInterface) *RelevancyEvaluator {
	return &RelevancyEvaluator{
		judge:     judge,
		threshold: DefaultThresholds[MetricRelevancy],
	}
}

// Evaluate performs relevancy evaluation
func (e *RelevancyEvaluator) Evaluate(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	// Build evaluation prompt
	prompt := e.buildPrompt(input.Task, input.FinalResponse)

	// Call LLM judge
	var response RelevancyResponse
	tokens, cost, err := e.judge.Evaluate(ctx, prompt, &response)
	if err != nil {
		return MetricResult{}, fmt.Errorf("relevancy evaluation failed: %w", err)
	}

	// Calculate score from verdicts
	score := e.calculateScore(response.Verdicts)

	// Build result
	result := MetricResult{
		MetricType:  MetricRelevancy,
		Score:       score,
		Threshold:   e.threshold,
		Passed:      score >= e.threshold,
		Reason:      e.generateReason(response.Verdicts, score),
		JudgeTokens: tokens,
		JudgeCost:   cost,
	}

	// Convert to standard verdicts
	result.Verdicts = make([]Verdict, len(response.Verdicts))
	for i, v := range response.Verdicts {
		result.Verdicts[i] = Verdict{
			Statement: v.Statement,
			Verdict:   v.Verdict,
			Reason:    v.Reason,
		}
	}

	return result, nil
}

// calculateScore calculates relevancy score from verdicts
func (e *RelevancyEvaluator) calculateScore(verdicts []RelevancyVerdict) float64 {
	if len(verdicts) == 0 {
		return 1.0 // No statements = fully relevant (edge case)
	}

	relevantCount := 0
	for _, v := range verdicts {
		verdict := strings.ToLower(strings.TrimSpace(v.Verdict))
		// Count both "yes" and "idk" as relevant (idk means contextually relevant)
		if verdict == "yes" || verdict == "relevant" || verdict == "idk" {
			relevantCount++
		}
	}

	return float64(relevantCount) / float64(len(verdicts))
}

// generateReason generates a human-readable explanation
func (e *RelevancyEvaluator) generateReason(verdicts []RelevancyVerdict, score float64) string {
	total := len(verdicts)
	if total == 0 {
		return "No statements to evaluate"
	}

	relevant := int(score * float64(total))
	irrelevant := total - relevant

	if score >= 0.9 {
		return fmt.Sprintf("Highly relevant: %d/%d statements directly address the task", relevant, total)
	} else if score >= 0.7 {
		return fmt.Sprintf("Mostly relevant: %d/%d statements address the task, with %d off-topic", relevant, total, irrelevant)
	} else if score >= 0.5 {
		return fmt.Sprintf("Partially relevant: %d/%d statements address the task, with %d irrelevant", relevant, total, irrelevant)
	} else {
		return fmt.Sprintf("Low relevancy: Only %d/%d statements address the task, %d are irrelevant", relevant, total, irrelevant)
	}
}

// buildPrompt constructs the LLM evaluation prompt
func (e *RelevancyEvaluator) buildPrompt(task, actualOutput string) string {
	return fmt.Sprintf(`You are evaluating whether an AI agent's response is relevant to the user's task.

TASK: Break down the response into individual statements, then judge if each statement is relevant to the task.

IMPORTANT EVALUATION PRINCIPLES:
- Extract 3-8 key statements from the response (don't list every sentence)
- Be GENEROUS: If a statement provides useful context or partial answers, mark it as relevant
- For each statement, determine: "yes" (relevant), "no" (irrelevant), or "idk" (contextually relevant)
- "idk" means the statement provides context/metadata that's loosely related - this counts as relevant
- Provide "reason" ONLY for "no" or "idk" verdicts
- Return ONLY valid JSON with no markdown formatting

EXAMPLE:
Task: Scan terraform files for security vulnerabilities
Response: I analyzed 15 terraform files and found 47 security issues including SQL injection risks, hardcoded credentials, and public S3 buckets. The scan took 2.3 minutes and used checkov and semgrep tools.

Example JSON:
{
  "verdicts": [
    {
      "statement": "Analyzed 15 terraform files",
      "verdict": "yes"
    },
    {
      "statement": "Found 47 security issues including SQL injection, hardcoded credentials, and public S3 buckets",
      "verdict": "yes"
    },
    {
      "statement": "The scan took 2.3 minutes",
      "verdict": "idk",
      "reason": "Time taken is not directly relevant to the security findings but provides context"
    },
    {
      "statement": "Used checkov and semgrep tools",
      "verdict": "yes"
    }
  ]
}

NOW EVALUATE:

Task:
%s

Response:
%s

Return JSON with 'verdicts' array:
{
  "verdicts": [
    {
      "statement": "...",
      "verdict": "yes|no|idk",
      "reason": "..." 
    }
  ]
}

JSON:`, task, actualOutput)
}

// ============================================================================
// Response Types
// ============================================================================

// RelevancyResponse represents the LLM's relevancy judgment
type RelevancyResponse struct {
	Verdicts []RelevancyVerdict `json:"verdicts"`
}

// RelevancyVerdict represents a single statement's relevancy judgment
type RelevancyVerdict struct {
	Statement string `json:"statement"`
	Verdict   string `json:"verdict"` // "yes", "no", "idk"
	Reason    string `json:"reason,omitempty"`
}
