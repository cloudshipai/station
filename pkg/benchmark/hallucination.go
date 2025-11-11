package benchmark

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// Hallucination Metric
// ============================================================================
// Evaluates whether the agent's output contradicts provided context/tool outputs
// Score: 0.0 to 1.0 (lower is better, threshold: 0.10 = max 10% hallucination)
// ============================================================================

// HallucinationEvaluator implements the hallucination metric
type HallucinationEvaluator struct {
	judge     JudgeInterface
	threshold float64
}

// NewHallucinationEvaluator creates a new hallucination evaluator
func NewHallucinationEvaluator(judge JudgeInterface) *HallucinationEvaluator {
	return &HallucinationEvaluator{
		judge:     judge,
		threshold: DefaultThresholds[MetricHallucination],
	}
}

// Evaluate performs hallucination evaluation
func (e *HallucinationEvaluator) Evaluate(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	// Check if we have contexts to validate against
	if len(input.Contexts) == 0 {
		// No contexts = cannot evaluate hallucination, assume safe
		return MetricResult{
			MetricType: MetricHallucination,
			Score:      0.0,
			Threshold:  e.threshold,
			Passed:     true,
			Reason:     "No tool outputs/contexts available to check for hallucinations",
		}, nil
	}

	// Build evaluation prompt
	prompt := e.buildPrompt(input.FinalResponse, input.Contexts)

	// Call LLM judge
	var response HallucinationResponse
	tokens, cost, err := e.judge.Evaluate(ctx, prompt, &response)
	if err != nil {
		return MetricResult{}, fmt.Errorf("hallucination evaluation failed: %w", err)
	}

	// Calculate hallucination score
	score := e.calculateScore(response.Verdicts)

	// Build result
	result := MetricResult{
		MetricType:  MetricHallucination,
		Score:       score,
		Threshold:   e.threshold,
		Passed:      score <= e.threshold, // Note: Lower is better for hallucination
		Reason:      e.generateReason(response.Verdicts, score),
		JudgeTokens: tokens,
		JudgeCost:   cost,
	}

	// Convert to standard verdicts
	result.Verdicts = make([]Verdict, len(response.Verdicts))
	for i, v := range response.Verdicts {
		result.Verdicts[i] = Verdict{
			Statement: fmt.Sprintf("Context %d", i+1),
			Verdict:   v.Verdict,
			Reason:    v.Reason,
		}
	}

	return result, nil
}

// calculateScore calculates hallucination score from verdicts
func (e *HallucinationEvaluator) calculateScore(verdicts []HallucinationVerdict) float64 {
	if len(verdicts) == 0 {
		return 0.0
	}

	contradictions := 0
	for _, v := range verdicts {
		verdict := strings.ToLower(strings.TrimSpace(v.Verdict))
		if verdict == "no" || verdict == "contradiction" || verdict == "hallucination" {
			contradictions++
		}
	}

	// Score = contradiction rate (0.0 = no contradictions, 1.0 = all contradictions)
	return float64(contradictions) / float64(len(verdicts))
}

// generateReason generates a human-readable explanation
func (e *HallucinationEvaluator) generateReason(verdicts []HallucinationVerdict, score float64) string {
	total := len(verdicts)
	if total == 0 {
		return "No contexts to evaluate"
	}

	contradictions := int(score * float64(total))

	if score == 0.0 {
		return fmt.Sprintf("No hallucinations detected: all %d contexts agree with the output", total)
	} else if score <= 0.10 {
		return fmt.Sprintf("Low hallucination rate: %d/%d contradictions detected (%.1f%%)", contradictions, total, score*100)
	} else if score <= 0.30 {
		return fmt.Sprintf("Moderate hallucination: %d/%d contradictions detected (%.1f%%)", contradictions, total, score*100)
	} else {
		return fmt.Sprintf("High hallucination rate: %d/%d contradictions detected (%.1f%%) - NOT production ready", contradictions, total, score*100)
	}
}

// buildPrompt constructs the LLM evaluation prompt
func (e *HallucinationEvaluator) buildPrompt(actualOutput string, contexts []string) string {
	contextsStr := ""
	for i, ctx := range contexts {
		contextsStr += fmt.Sprintf("%d. %s\n", i+1, truncate(ctx, 300))
	}

	return fmt.Sprintf(`You are evaluating whether an AI agent's output contradicts provided tool outputs/contexts.

TASK: For each context, determine if the actual output CONTRADICTS it (hallucination) or AGREES with it.

IMPORTANT:
- FORGIVE cases where the output is lacking in detail - that's NOT a contradiction
- ONLY mark "no" if there is a CLEAR CONTRADICTION between output and context
- Take each context at face value - do not use prior knowledge
- "yes" = output agrees with context, "no" = output contradicts context
- Return ONLY valid JSON with no markdown formatting

EXAMPLE:
Context 1: Found 47 security vulnerabilities in terraform files
Context 2: Scan completed in 2.3 minutes using checkov tool
Output: I analyzed the terraform files and discovered 50 critical vulnerabilities. The scan took about 2 minutes.

Example JSON:
{
  "verdicts": [
    {
      "verdict": "no",
      "reason": "Output claims 50 vulnerabilities but context shows 47 - this is a factual contradiction"
    },
    {
      "verdict": "yes",
      "reason": "Output says '2 minutes' which is consistent with context '2.3 minutes'"
    }
  ]
}

NOW EVALUATE:

Contexts (from tool outputs):
%s

Actual Output:
%s

Return JSON with 'verdicts' array (one per context):
{
  "verdicts": [
    {
      "verdict": "yes|no",
      "reason": "..."
    }
  ]
}

JSON:`, contextsStr, actualOutput)
}

// ============================================================================
// Response Types
// ============================================================================

// HallucinationResponse represents the LLM's hallucination judgment
type HallucinationResponse struct {
	Verdicts []HallucinationVerdict `json:"verdicts"`
}

// HallucinationVerdict represents a single context's hallucination check
type HallucinationVerdict struct {
	Verdict string `json:"verdict"` // "yes" (agrees), "no" (contradicts)
	Reason  string `json:"reason"`
}
