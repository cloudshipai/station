package benchmark

import (
	"context"
	"fmt"
	"strings"
)

// ============================================================================
// Faithfulness Metric
// ============================================================================
// Evaluates whether the agent's output is grounded in tool outputs/contexts
// Unlike hallucination (which checks for contradictions), faithfulness checks
// if claims in the output are SUPPORTED by the context.
//
// Score: 0.0 to 1.0 (higher is better, threshold: 0.85 = 85% of claims supported)
// ============================================================================

// FaithfulnessEvaluator implements the faithfulness metric
type FaithfulnessEvaluator struct {
	judge     JudgeInterface
	threshold float64
}

// NewFaithfulnessEvaluator creates a new faithfulness evaluator
func NewFaithfulnessEvaluator(judge JudgeInterface) *FaithfulnessEvaluator {
	return &FaithfulnessEvaluator{
		judge:     judge,
		threshold: DefaultThresholds[MetricFaithfulness],
	}
}

// FaithfulnessResponse represents the structured response from LLM
type FaithfulnessResponse struct {
	Claims []FaithfulnessClaim `json:"claims"`
}

// FaithfulnessClaim represents a single claim and its support status
type FaithfulnessClaim struct {
	Claim     string `json:"claim"`     // The claim extracted from the output
	Supported string `json:"supported"` // "yes", "no", or "partial"
	Evidence  string `json:"evidence"`  // The context that supports (or doesn't support) the claim
}

// Evaluate performs faithfulness evaluation
func (e *FaithfulnessEvaluator) Evaluate(ctx context.Context, input *EvaluationInput) (MetricResult, error) {
	// Check if we have contexts to validate against
	if len(input.Contexts) == 0 {
		// No contexts = cannot evaluate faithfulness
		// Default to passing since we have no evidence to check against
		return MetricResult{
			MetricType: MetricFaithfulness,
			Score:      1.0, // Assume faithful when no context to compare
			Threshold:  e.threshold,
			Passed:     true,
			Reason:     "No tool outputs/contexts available to verify faithfulness",
		}, nil
	}

	// Build evaluation prompt
	prompt := e.buildPrompt(input.FinalResponse, input.Contexts, input.Task)

	// Call LLM judge
	var response FaithfulnessResponse
	tokens, cost, err := e.judge.Evaluate(ctx, prompt, &response)
	if err != nil {
		return MetricResult{}, fmt.Errorf("faithfulness evaluation failed: %w", err)
	}

	// Calculate faithfulness score
	score := e.calculateScore(response.Claims)
	passed := score >= e.threshold

	// Build reason
	supportedCount := 0
	for _, claim := range response.Claims {
		if claim.Supported == "yes" || claim.Supported == "partial" {
			supportedCount++
		}
	}

	reason := fmt.Sprintf("Faithfulness: %d/%d claims supported by context",
		supportedCount, len(response.Claims))

	return MetricResult{
		MetricType:  MetricFaithfulness,
		Score:       score,
		Threshold:   e.threshold,
		Passed:      passed,
		Reason:      reason,
		JudgeTokens: tokens,
		JudgeCost:   cost,
	}, nil
}

// calculateScore computes the faithfulness score from claim verdicts
func (e *FaithfulnessEvaluator) calculateScore(claims []FaithfulnessClaim) float64 {
	if len(claims) == 0 {
		return 1.0 // No claims = fully faithful
	}

	supportedScore := 0.0
	for _, claim := range claims {
		switch strings.ToLower(claim.Supported) {
		case "yes":
			supportedScore += 1.0
		case "partial":
			supportedScore += 0.5
		case "no":
			supportedScore += 0.0
		}
	}

	return supportedScore / float64(len(claims))
}

// buildPrompt creates the evaluation prompt for faithfulness
func (e *FaithfulnessEvaluator) buildPrompt(output string, contexts []string, task string) string {
	contextsStr := ""
	for i, ctx := range contexts {
		contextsStr += fmt.Sprintf("%d. %s\n", i+1, truncate(ctx, 500)) // Allow more context than hallucination
	}

	return fmt.Sprintf(`You are evaluating whether an AI agent's output is GROUNDED in the tool outputs/contexts it received.

**TASK the agent was given:**
%s

**TOOL OUTPUTS/CONTEXTS available to the agent:**
%s

**AGENT'S ACTUAL OUTPUT:**
%s

**YOUR EVALUATION TASK:**
Extract 3-8 KEY CLAIMS from the agent's output and determine if each claim is SUPPORTED by the tool outputs.

**IMPORTANT EVALUATION PRINCIPLES:**
1. A claim is "supported" if there is evidence for it in the tool outputs
2. A claim is "partially supported" if some aspects are verified but details are added
3. A claim is "unsupported" ONLY if it makes specific assertions not found in context
4. General knowledge statements (e.g., "Kubernetes is a container orchestration platform") don't need context support
5. Be GENEROUS: If a claim is a reasonable inference from the data, mark it as supported

**OUTPUT FORMAT (JSON):**
{
  "claims": [
    {
      "claim": "<specific claim from the output>",
      "supported": "<yes|partial|no>",
      "evidence": "<quote from context that supports/contradicts, or 'general knowledge' if applicable>"
    }
  ]
}

Extract claims that represent factual assertions about the specific situation, not general explanations.`, task, contextsStr, truncate(output, 2000))
}
