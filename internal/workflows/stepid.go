package workflows

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// StepContext contains the execution context needed to generate a deterministic step ID.
// This enables idempotent execution: same context = same step ID = skip if already done.
type StepContext struct {
	RunID        string   // Workflow run ID
	StateName    string   // Current state name
	BranchPath   []string // Path through parallel branches (e.g., ["parallel_1", "branch_a"])
	ForeachIndex int      // Index in foreach iteration (-1 if not in foreach)
}

// GenerateStepID creates a deterministic step ID from execution context.
// Formula: sha256(run_id + state_name + branch_path + foreach_index)[:16]
//
// This ensures:
// - Same execution context = same step ID (enables idempotency checks)
// - Different contexts = different step IDs (no collisions)
// - Reproducible after crash/restart
func GenerateStepID(ctx StepContext) string {
	var parts []string

	parts = append(parts, ctx.RunID)
	parts = append(parts, ctx.StateName)

	if len(ctx.BranchPath) > 0 {
		parts = append(parts, strings.Join(ctx.BranchPath, "/"))
	}

	if ctx.ForeachIndex >= 0 {
		parts = append(parts, fmt.Sprintf("foreach[%d]", ctx.ForeachIndex))
	}

	// Create deterministic hash
	input := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(input))

	// Return first 16 hex chars (64 bits - sufficient for uniqueness)
	return hex.EncodeToString(hash[:])[:16]
}

// NewStepContext creates a StepContext for a simple (non-parallel, non-foreach) step.
func NewStepContext(runID, stateName string) StepContext {
	return StepContext{
		RunID:        runID,
		StateName:    stateName,
		ForeachIndex: -1, // Not in foreach
	}
}

// WithBranchPath returns a new StepContext with the given branch path.
func (ctx StepContext) WithBranchPath(path ...string) StepContext {
	ctx.BranchPath = path
	return ctx
}

// WithForeachIndex returns a new StepContext with the given foreach index.
func (ctx StepContext) WithForeachIndex(index int) StepContext {
	ctx.ForeachIndex = index
	return ctx
}

// IdempotencyKey returns a string suitable for NATS message headers.
// Format: runID:stepID:attempt
func IdempotencyKey(runID, stepID string, attempt int64) string {
	return fmt.Sprintf("%s:%s:%d", runID, stepID, attempt)
}

// ParseIdempotencyKey extracts runID, stepID, and attempt from an idempotency key.
func ParseIdempotencyKey(key string) (runID, stepID string, attempt int64, ok bool) {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) != 3 {
		return "", "", 0, false
	}

	runID = parts[0]
	stepID = parts[1]

	_, err := fmt.Sscanf(parts[2], "%d", &attempt)
	if err != nil {
		return "", "", 0, false
	}

	return runID, stepID, attempt, true
}
