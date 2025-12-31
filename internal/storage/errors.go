package storage

import (
	"errors"
	"fmt"
)

var (
	// ErrFileNotFound is returned when a file doesn't exist.
	ErrFileNotFound = errors.New("file not found")

	// ErrFileTooLarge is returned when a file exceeds the maximum size.
	ErrFileTooLarge = errors.New("file exceeds maximum size")

	// ErrStorageQuotaExceeded is returned when the storage quota is exceeded.
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")

	// ErrInvalidKey is returned when a file key is invalid.
	ErrInvalidKey = errors.New("invalid file key")

	// ErrBucketNotFound is returned when the Object Store bucket doesn't exist.
	ErrBucketNotFound = errors.New("object store bucket not found")

	// ErrStoreNotInitialized is returned when operations are attempted on an uninitialized store.
	ErrStoreNotInitialized = errors.New("file store not initialized")
)

// FileError wraps an error with file context.
type FileError struct {
	Op  string // Operation that failed
	Key string // File key involved
	Err error  // Underlying error
}

func (e *FileError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("%s %s: %v", e.Op, e.Key, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *FileError) Unwrap() error {
	return e.Err
}

// NewFileError creates a new FileError.
func NewFileError(op, key string, err error) *FileError {
	return &FileError{Op: op, Key: key, Err: err}
}

// IsNotFound returns true if the error indicates a file was not found.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrFileNotFound)
}

// IsTooLarge returns true if the error indicates a file is too large.
func IsTooLarge(err error) bool {
	return errors.Is(err, ErrFileTooLarge)
}

// IsQuotaExceeded returns true if the error indicates the storage quota is exceeded.
func IsQuotaExceeded(err error) bool {
	return errors.Is(err, ErrStorageQuotaExceeded)
}
