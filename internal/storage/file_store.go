// Package storage provides file storage interfaces and implementations
// for staging files between users and sandboxes via NATS Object Store.
package storage

import (
	"context"
	"io"
	"time"
)

// FileStore defines the interface for file storage operations.
// The primary implementation uses NATS JetStream Object Store.
type FileStore interface {
	// Put stores a file and returns its metadata.
	// The key should follow the convention:
	//   - files/{file_id} for user uploads
	//   - runs/{run_id}/output/{filename} for workflow outputs
	//   - sessions/{session_id}/{filename} for session artifacts
	Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) (*FileInfo, error)

	// Get retrieves a file by key. Caller must close the returned reader.
	Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)

	// Delete removes a file by key.
	Delete(ctx context.Context, key string) error

	// List returns files matching the optional prefix.
	// Pass empty string to list all files.
	List(ctx context.Context, prefix string) ([]*FileInfo, error)

	// Exists checks if a file exists by key.
	Exists(ctx context.Context, key string) (bool, error)

	// GetInfo returns file metadata without the content.
	GetInfo(ctx context.Context, key string) (*FileInfo, error)

	// Close releases any resources held by the store.
	Close() error
}

// FileInfo contains metadata about a stored file.
type FileInfo struct {
	// Key is the unique identifier/path for the file
	Key string `json:"key"`

	// Size is the file size in bytes
	Size int64 `json:"size"`

	// ContentType is the MIME type of the file
	ContentType string `json:"content_type,omitempty"`

	// Checksum is the SHA-256 hash of the file content
	Checksum string `json:"checksum,omitempty"`

	// CreatedAt is when the file was stored
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when the file will be automatically deleted (zero means no expiration)
	ExpiresAt time.Time `json:"expires_at,omitempty"`

	// Metadata contains custom key-value pairs
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PutOptions configures file upload behavior.
type PutOptions struct {
	// ContentType is the MIME type of the file (optional, auto-detected if not set)
	ContentType string

	// Metadata is custom key-value pairs to store with the file
	Metadata map[string]string

	// TTL is how long the file should be kept (0 = no expiration)
	TTL time.Duration

	// Description is a human-readable description of the file
	Description string
}

// Config holds configuration for the file store.
type Config struct {
	// BucketName is the NATS Object Store bucket name
	BucketName string `yaml:"bucket" json:"bucket"`

	// MaxFileSize is the maximum allowed file size in bytes (default: 100 MB)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`

	// MaxTotalBytes is the maximum total storage in bytes (default: 10 GB)
	MaxTotalBytes int64 `yaml:"max_total_bytes" json:"max_total_bytes"`

	// DefaultTTL is the default time-to-live for uploaded files (0 = no expiration)
	DefaultTTL time.Duration `yaml:"default_ttl" json:"default_ttl"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BucketName:    "sandbox-files",
		MaxFileSize:   100 * 1024 * 1024,       // 100 MB
		MaxTotalBytes: 10 * 1024 * 1024 * 1024, // 10 GB
		DefaultTTL:    0,                       // No expiration
	}
}

// GenerateFileID generates a unique file ID using ULID.
func GenerateFileID() string {
	return "f_" + generateULID()
}

// GenerateRunOutputKey generates a key for workflow run outputs.
func GenerateRunOutputKey(runID, filename string) string {
	return "runs/" + runID + "/output/" + filename
}

// GenerateSessionKey generates a key for session artifacts.
func GenerateSessionKey(sessionID, filename string) string {
	return "sessions/" + sessionID + "/" + filename
}

// GenerateUserFileKey generates a key for user-uploaded files.
func GenerateUserFileKey(fileID string) string {
	return "files/" + fileID
}
