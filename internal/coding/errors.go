package coding

import (
	"errors"
	"fmt"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrBackendNotReady = errors.New("coding backend not ready")
	ErrTaskTimeout     = errors.New("task execution timed out")
	ErrTaskFailed      = errors.New("task execution failed")
)

// Error wraps coding-related errors with context.
type Error struct {
	Op      string
	Session string
	Err     error
}

func (e *Error) Error() string {
	if e.Session != "" {
		return fmt.Sprintf("coding.%s [session=%s]: %v", e.Op, e.Session, e.Err)
	}
	return fmt.Sprintf("coding.%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error {
	return e.Err
}
