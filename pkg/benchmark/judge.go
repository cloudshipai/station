package benchmark

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"station/internal/config"
	"station/pkg/faker/ai"
)

// JudgeInterface defines the interface for LLM evaluation
type JudgeInterface interface {
	Evaluate(ctx context.Context, prompt string, output interface{}) (int, float64, error)
	GetModelName() string
}

// Judge performs LLM-based evaluations
type Judge struct {
	client ai.Client
	model  string
}

// NewJudge creates a new LLM judge for evaluations
func NewJudge(cfg *config.Config) (*Judge, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Create AI client using existing faker/ai package
	client, err := ai.NewClient(cfg, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	return &Judge{
		client: client,
		model:  client.GetModelName(),
	}, nil
}

// Evaluate sends a prompt to the LLM and parses the JSON response
func (j *Judge) Evaluate(ctx context.Context, prompt string, output interface{}) (int, float64, error) {
	_ = time.Now() // For future latency tracking

	// Generate response
	response, err := j.client.Generate(ctx, prompt)
	if err != nil {
		return 0, 0, fmt.Errorf("LLM evaluation failed: %w", err)
	}

	// Parse JSON response
	if err := json.Unmarshal([]byte(response), output); err != nil {
		// Try to extract JSON from markdown code blocks if present
		cleaned := extractJSON(response)
		if err := json.Unmarshal([]byte(cleaned), output); err != nil {
			return 0, 0, fmt.Errorf("failed to parse LLM response as JSON: %w\nResponse: %s", err, response)
		}
	}

	// Calculate tokens and cost (approximate)
	tokens := estimateTokens(prompt + response)
	cost := estimateCost(tokens, j.model)

	return tokens, cost, nil
}

// GetModelName returns the model name being used
func (j *Judge) GetModelName() string {
	return j.model
}

// ============================================================================
// Helper Functions
// ============================================================================

// extractJSON attempts to extract JSON from markdown code blocks
func extractJSON(response string) string {
	// Remove common markdown wrappers
	cleaned := response

	// Remove ```json ... ``` blocks
	if start := findIndex(cleaned, "```json"); start != -1 {
		cleaned = cleaned[start+7:]
		if end := findIndex(cleaned, "```"); end != -1 {
			cleaned = cleaned[:end]
		}
	} else if start := findIndex(cleaned, "```"); start != -1 {
		// Remove generic ``` blocks
		cleaned = cleaned[start+3:]
		if end := findIndex(cleaned, "```"); end != -1 {
			cleaned = cleaned[:end]
		}
	}

	return trimSpace(cleaned)
}

func findIndex(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	// Simple trim implementation
	start := 0
	end := len(s)

	for start < end && isSpace(s[start]) {
		start++
	}
	for start < end && isSpace(s[end-1]) {
		end--
	}

	return s[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// estimateTokens estimates token count (rough approximation: 1 token ≈ 4 chars)
func estimateTokens(text string) int {
	return len(text) / 4
}

// estimateCost estimates API cost based on model and tokens
func estimateCost(tokens int, model string) float64 {
	// Approximate pricing per 1M tokens (as of 2024)
	pricePerMillionTokens := 0.0

	if findIndex(model, "gpt-4o-mini") != -1 {
		pricePerMillionTokens = 0.15 // $0.15 per 1M tokens
	} else if findIndex(model, "gpt-4o") != -1 {
		pricePerMillionTokens = 5.0 // $5 per 1M tokens
	} else if findIndex(model, "gemini") != -1 {
		pricePerMillionTokens = 0.075 // $0.075 per 1M tokens (Gemini Flash)
	} else {
		pricePerMillionTokens = 0.15 // Default to gpt-4o-mini pricing
	}

	return float64(tokens) / 1_000_000.0 * pricePerMillionTokens
}
