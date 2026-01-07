// Package nats provides NATS JetStream integration for the agentic harness.
// It handles inter-agent file sharing, workflow state, and context preservation.
package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	DefaultKVBucket         = "harness-state"
	DefaultObjectBucket     = "harness-files"
	DefaultStateTTL         = 24 * time.Hour
	DefaultObjectStoreMaxGB = 1

	PrefixRun      = "run"
	PrefixWorkflow = "workflow"
	PrefixShared   = "shared"
)

// StoreConfig configures the NATS store.
type StoreConfig struct {
	// KVBucket is the name of the KV bucket for state
	KVBucket string `yaml:"kv_bucket" json:"kv_bucket"`

	// ObjectBucket is the name of the object store for files
	ObjectBucket string `yaml:"object_bucket" json:"object_bucket"`

	// MaxFileSize is the maximum file size in bytes (default: 100MB)
	MaxFileSize int64 `yaml:"max_file_size" json:"max_file_size"`

	// TTL is the default time-to-live for state entries
	TTL time.Duration `yaml:"ttl" json:"ttl"`
}

// DefaultStoreConfig returns sensible defaults.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		KVBucket:     DefaultKVBucket,
		ObjectBucket: DefaultObjectBucket,
		MaxFileSize:  100 * 1024 * 1024, // 100MB
		TTL:          DefaultStateTTL,
	}
}

// Store provides NATS-based storage for the agentic harness.
type Store struct {
	js     jetstream.JetStream
	config StoreConfig

	mu          sync.RWMutex
	kv          jetstream.KeyValue
	objectStore nats.ObjectStore

	// Legacy JS context for object store (nats.go hasn't migrated object store to jetstream pkg yet)
	legacyJS nats.JetStreamContext
}

// NewStore creates a new NATS store with the given JetStream context.
func NewStore(nc *nats.Conn, config StoreConfig) (*Store, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("create jetstream context: %w", err)
	}

	// Get legacy JS context for object store operations
	legacyJS, err := nc.JetStream()
	if err != nil {
		return nil, fmt.Errorf("create legacy jetstream context: %w", err)
	}

	if config.KVBucket == "" {
		config.KVBucket = DefaultKVBucket
	}
	if config.ObjectBucket == "" {
		config.ObjectBucket = DefaultObjectBucket
	}
	if config.TTL == 0 {
		config.TTL = DefaultStateTTL
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100 * 1024 * 1024
	}

	return &Store{
		js:       js,
		legacyJS: legacyJS,
		config:   config,
	}, nil
}

// getKV returns the KV bucket, creating it if necessary.
func (s *Store) getKV(ctx context.Context) (jetstream.KeyValue, error) {
	s.mu.RLock()
	if s.kv != nil {
		kv := s.kv
		s.mu.RUnlock()
		return kv, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.kv != nil {
		return s.kv, nil
	}

	// Try to get existing bucket
	kv, err := s.js.KeyValue(ctx, s.config.KVBucket)
	if err == nil {
		s.kv = kv
		return kv, nil
	}

	// Create bucket if it doesn't exist
	kv, err = s.js.CreateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:      s.config.KVBucket,
		Description: "Agentic harness state storage",
		TTL:         s.config.TTL,
	})
	if err != nil {
		return nil, fmt.Errorf("create KV bucket %s: %w", s.config.KVBucket, err)
	}

	s.kv = kv
	return kv, nil
}

// getObjectStore returns the object store bucket, creating it if necessary.
func (s *Store) getObjectStore(ctx context.Context) (nats.ObjectStore, error) {
	s.mu.RLock()
	if s.objectStore != nil {
		os := s.objectStore
		s.mu.RUnlock()
		return os, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.objectStore != nil {
		return s.objectStore, nil
	}

	// Try to get existing bucket
	os, err := s.legacyJS.ObjectStore(s.config.ObjectBucket)
	if err == nil {
		s.objectStore = os
		return os, nil
	}

	// Create bucket if it doesn't exist
	if err == nats.ErrBucketNotFound || err == nats.ErrStreamNotFound || strings.Contains(err.Error(), "stream not found") {
		os, err = s.legacyJS.CreateObjectStore(&nats.ObjectStoreConfig{
			Bucket:      s.config.ObjectBucket,
			Description: "Agentic harness file storage",
			MaxBytes:    DefaultObjectStoreMaxGB * 1024 * 1024 * 1024,
		})
		if err != nil {
			return nil, fmt.Errorf("create object store %s: %w", s.config.ObjectBucket, err)
		}
		s.objectStore = os
		return os, nil
	}

	return nil, fmt.Errorf("get object store %s: %w", s.config.ObjectBucket, err)
}

// ==================== KV State Operations ====================

// GetState retrieves a state value by key.
func (s *Store) GetState(ctx context.Context, key string) ([]byte, error) {
	kv, err := s.getKV(ctx)
	if err != nil {
		return nil, err
	}

	entry, err := kv.Get(ctx, key)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get state %s: %w", key, err)
	}

	return entry.Value(), nil
}

// SetState stores a state value.
func (s *Store) SetState(ctx context.Context, key string, value []byte) error {
	kv, err := s.getKV(ctx)
	if err != nil {
		return err
	}

	if _, err := kv.Put(ctx, key, value); err != nil {
		return fmt.Errorf("put state %s: %w", key, err)
	}

	return nil
}

// DeleteState removes a state value.
func (s *Store) DeleteState(ctx context.Context, key string) error {
	kv, err := s.getKV(ctx)
	if err != nil {
		return err
	}

	if err := kv.Delete(ctx, key); err != nil && err != jetstream.ErrKeyNotFound {
		return fmt.Errorf("delete state %s: %w", key, err)
	}

	return nil
}

// GetJSON retrieves and unmarshals a JSON state value.
func (s *Store) GetJSON(ctx context.Context, key string, v interface{}) error {
	data, err := s.GetState(ctx, key)
	if err != nil {
		return err
	}
	if data == nil {
		return nil // Key not found, leave v as zero value
	}
	return json.Unmarshal(data, v)
}

// SetJSON marshals and stores a JSON state value.
func (s *Store) SetJSON(ctx context.Context, key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	return s.SetState(ctx, key, data)
}

// ==================== Run State Operations ====================

// RunState represents the state of a single agent run.
type RunState struct {
	RunID       string            `json:"run_id"`
	AgentID     string            `json:"agent_id"`
	AgentName   string            `json:"agent_name"`
	WorkflowID  string            `json:"workflow_id,omitempty"`
	StepName    string            `json:"step_name,omitempty"`
	Status      string            `json:"status"` // running, completed, failed
	StartedAt   time.Time         `json:"started_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Task        string            `json:"task"`
	Result      string            `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	GitBranch   string            `json:"git_branch,omitempty"`
	GitCommit   string            `json:"git_commit,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// runStateKey returns the KV key for run state.
func runStateKey(runID string) string {
	return fmt.Sprintf("%s.%s.state", PrefixRun, runID)
}

// GetRunState retrieves the state of a run.
func (s *Store) GetRunState(ctx context.Context, runID string) (*RunState, error) {
	var state RunState
	if err := s.GetJSON(ctx, runStateKey(runID), &state); err != nil {
		return nil, err
	}
	if state.RunID == "" {
		return nil, nil // Not found
	}
	return &state, nil
}

// SetRunState saves the state of a run.
func (s *Store) SetRunState(ctx context.Context, state *RunState) error {
	return s.SetJSON(ctx, runStateKey(state.RunID), state)
}

// ==================== Workflow Context Operations ====================

// WorkflowContext represents shared context across workflow steps.
type WorkflowContext struct {
	WorkflowID    string                 `json:"workflow_id"`
	WorkflowRunID string                 `json:"workflow_run_id"`
	StartedAt     time.Time              `json:"started_at"`
	GitBranch     string                 `json:"git_branch,omitempty"`
	Steps         []WorkflowStepSummary  `json:"steps"`
	SharedData    map[string]interface{} `json:"shared_data,omitempty"`
}

// WorkflowStepSummary represents a completed workflow step.
type WorkflowStepSummary struct {
	StepName      string     `json:"step_name"`
	AgentName     string     `json:"agent_name"`
	RunID         string     `json:"run_id"`
	Status        string     `json:"status"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Summary       string     `json:"summary,omitempty"`
	FilesModified []string   `json:"files_modified,omitempty"`
	Commits       []string   `json:"commits,omitempty"`
}

// workflowContextKey returns the KV key for workflow context.
func workflowContextKey(workflowRunID string) string {
	return fmt.Sprintf("%s.%s.context", PrefixWorkflow, workflowRunID)
}

// GetWorkflowContext retrieves the context of a workflow run.
func (s *Store) GetWorkflowContext(ctx context.Context, workflowRunID string) (*WorkflowContext, error) {
	var wctx WorkflowContext
	if err := s.GetJSON(ctx, workflowContextKey(workflowRunID), &wctx); err != nil {
		return nil, err
	}
	if wctx.WorkflowRunID == "" {
		return nil, nil // Not found
	}
	return &wctx, nil
}

// SetWorkflowContext saves the context of a workflow run.
func (s *Store) SetWorkflowContext(ctx context.Context, wctx *WorkflowContext) error {
	return s.SetJSON(ctx, workflowContextKey(wctx.WorkflowRunID), wctx)
}

// AddWorkflowStep adds a completed step to the workflow context.
func (s *Store) AddWorkflowStep(ctx context.Context, workflowRunID string, step WorkflowStepSummary) error {
	wctx, err := s.GetWorkflowContext(ctx, workflowRunID)
	if err != nil {
		return err
	}
	if wctx == nil {
		return fmt.Errorf("workflow context not found: %s", workflowRunID)
	}

	wctx.Steps = append(wctx.Steps, step)
	return s.SetWorkflowContext(ctx, wctx)
}

// ==================== File Operations ====================

// FileMetadata contains metadata about a stored file.
type FileMetadata struct {
	Key         string            `json:"key"`
	Size        int64             `json:"size"`
	ContentType string            `json:"content_type,omitempty"`
	Checksum    string            `json:"checksum,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PutFileOptions configures file upload.
type PutFileOptions struct {
	ContentType string
	Description string
	TTL         time.Duration
	Metadata    map[string]string
}

// PutFile stores a file in the object store.
func (s *Store) PutFile(ctx context.Context, key string, reader io.Reader, opts PutFileOptions) (*FileMetadata, error) {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return nil, err
	}

	// Read all data (object store requires this)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read file data: %w", err)
	}

	if s.config.MaxFileSize > 0 && int64(len(data)) > s.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d)", len(data), s.config.MaxFileSize)
	}

	meta := &nats.ObjectMeta{
		Name:        key,
		Description: opts.Description,
		Headers:     make(nats.Header),
	}

	if opts.ContentType != "" {
		meta.Headers.Set("Content-Type", opts.ContentType)
	}

	if opts.TTL > 0 {
		expiresAt := time.Now().Add(opts.TTL)
		meta.Headers.Set("X-Expires-At", expiresAt.Format(time.RFC3339))
	}

	for k, v := range opts.Metadata {
		meta.Headers.Set("X-Meta-"+k, v)
	}

	_, err = os.Put(meta, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("put file %s: %w", key, err)
	}

	info := &FileMetadata{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: opts.ContentType,
		CreatedAt:   time.Now(),
		Metadata:    opts.Metadata,
	}

	if opts.TTL > 0 {
		expires := time.Now().Add(opts.TTL)
		info.ExpiresAt = &expires
	}

	return info, nil
}

// GetFile retrieves a file from the object store.
func (s *Store) GetFile(ctx context.Context, key string) (io.ReadCloser, *FileMetadata, error) {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return nil, nil, err
	}

	result, err := os.Get(key)
	if err != nil {
		if err == nats.ErrObjectNotFound {
			return nil, nil, nil // Not found
		}
		return nil, nil, fmt.Errorf("get file %s: %w", key, err)
	}

	objInfo, err := result.Info()
	if err != nil {
		result.Close()
		return nil, nil, fmt.Errorf("get file info %s: %w", key, err)
	}

	info := &FileMetadata{
		Key:       objInfo.Name,
		Size:      int64(objInfo.Size),
		CreatedAt: objInfo.ModTime,
		Metadata:  make(map[string]string),
	}

	if objInfo.Headers != nil {
		info.ContentType = objInfo.Headers.Get("Content-Type")
		info.Checksum = objInfo.Headers.Get("X-Checksum")

		if expiresStr := objInfo.Headers.Get("X-Expires-At"); expiresStr != "" {
			if t, err := time.Parse(time.RFC3339, expiresStr); err == nil {
				info.ExpiresAt = &t
			}
		}

		for k, values := range objInfo.Headers {
			if strings.HasPrefix(k, "X-Meta-") && len(values) > 0 {
				metaKey := strings.TrimPrefix(k, "X-Meta-")
				info.Metadata[metaKey] = values[0]
			}
		}
	}

	return result, info, nil
}

// DeleteFile removes a file from the object store.
func (s *Store) DeleteFile(ctx context.Context, key string) error {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return err
	}

	if err := os.Delete(key); err != nil && err != nats.ErrObjectNotFound {
		return fmt.Errorf("delete file %s: %w", key, err)
	}

	return nil
}

// ListFiles lists files with the given prefix.
func (s *Store) ListFiles(ctx context.Context, prefix string) ([]*FileMetadata, error) {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return nil, err
	}

	objects, err := os.List()
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	var files []*FileMetadata
	for _, obj := range objects {
		if prefix == "" || strings.HasPrefix(obj.Name, prefix) {
			info := &FileMetadata{
				Key:       obj.Name,
				Size:      int64(obj.Size),
				CreatedAt: obj.ModTime,
				Metadata:  make(map[string]string),
			}

			if obj.Headers != nil {
				info.ContentType = obj.Headers.Get("Content-Type")

				for k, values := range obj.Headers {
					if strings.HasPrefix(k, "X-Meta-") && len(values) > 0 {
						metaKey := strings.TrimPrefix(k, "X-Meta-")
						info.Metadata[metaKey] = values[0]
					}
				}
			}

			files = append(files, info)
		}
	}

	return files, nil
}

// ==================== Run File Operations ====================

// runFileKey returns the object store key for a run file.
func runFileKey(runID, filename string) string {
	return fmt.Sprintf("%s/%s/output/%s", PrefixRun, runID, filename)
}

// PutRunFile stores an output file for a run.
func (s *Store) PutRunFile(ctx context.Context, runID, filename string, reader io.Reader, opts PutFileOptions) (*FileMetadata, error) {
	return s.PutFile(ctx, runFileKey(runID, filename), reader, opts)
}

// GetRunFile retrieves an output file from a run.
func (s *Store) GetRunFile(ctx context.Context, runID, filename string) (io.ReadCloser, *FileMetadata, error) {
	return s.GetFile(ctx, runFileKey(runID, filename))
}

// ListRunFiles lists output files from a run.
func (s *Store) ListRunFiles(ctx context.Context, runID string) ([]*FileMetadata, error) {
	prefix := fmt.Sprintf("%s/%s/output/", PrefixRun, runID)
	return s.ListFiles(ctx, prefix)
}

// ==================== Shared File Operations ====================

// sharedFileKey returns the object store key for a shared file.
func sharedFileKey(key string) string {
	return fmt.Sprintf("%s/%s", PrefixShared, key)
}

// PutSharedFile stores a shared file accessible by any agent.
func (s *Store) PutSharedFile(ctx context.Context, key string, reader io.Reader, opts PutFileOptions) (*FileMetadata, error) {
	return s.PutFile(ctx, sharedFileKey(key), reader, opts)
}

// GetSharedFile retrieves a shared file.
func (s *Store) GetSharedFile(ctx context.Context, key string) (io.ReadCloser, *FileMetadata, error) {
	return s.GetFile(ctx, sharedFileKey(key))
}

// ==================== Cleanup ====================

// CleanupExpiredFiles removes files that have expired.
func (s *Store) CleanupExpiredFiles(ctx context.Context) (int, error) {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return 0, err
	}

	objects, err := os.List()
	if err != nil {
		return 0, fmt.Errorf("list files: %w", err)
	}

	now := time.Now()
	var deleted int

	for _, obj := range objects {
		if obj.Headers != nil {
			if expiresStr := obj.Headers.Get("X-Expires-At"); expiresStr != "" {
				if t, err := time.Parse(time.RFC3339, expiresStr); err == nil && now.After(t) {
					if err := os.Delete(obj.Name); err == nil {
						deleted++
					}
				}
			}
		}
	}

	return deleted, nil
}

// Close releases resources (no-op for NATS stores).
func (s *Store) Close() error {
	return nil
}
