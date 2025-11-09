package services

import "context"

// contextKey is a type used for context keys to avoid collisions
type contextKey string

const (
	// ContextKeyParentRunID is used to track the parent run ID when agents call other agents as tools
	ContextKeyParentRunID contextKey = "parent_run_id"
)

// GetParentRunIDFromContext retrieves the parent run ID from context
func GetParentRunIDFromContext(ctx context.Context) *int64 {
	if val := ctx.Value(ContextKeyParentRunID); val != nil {
		if runID, ok := val.(*int64); ok {
			return runID
		}
	}
	return nil
}

// WithParentRunID adds the parent run ID to context
func WithParentRunID(ctx context.Context, runID int64) context.Context {
	return context.WithValue(ctx, ContextKeyParentRunID, &runID)
}
