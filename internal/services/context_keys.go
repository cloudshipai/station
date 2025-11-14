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

// ContextKeyCurrentRunID is the context key for the current run ID
const ContextKeyCurrentRunID contextKey = "current_run_id"

// GetCurrentRunIDFromContext extracts the current run ID from context
func GetCurrentRunIDFromContext(ctx context.Context) *int64 {
	if runID, ok := ctx.Value(ContextKeyCurrentRunID).(*int64); ok {
		return runID
	}
	return nil
}

// WithCurrentRunID adds the current run ID to context
func WithCurrentRunID(ctx context.Context, runID int64) context.Context {
	return context.WithValue(ctx, ContextKeyCurrentRunID, &runID)
}
