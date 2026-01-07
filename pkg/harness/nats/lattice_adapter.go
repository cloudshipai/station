// Package nats provides NATS integration for the agentic harness.
// This adapter bridges the harness with Station's existing lattice infrastructure.
package nats

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"station/internal/lattice/work"

	"github.com/nats-io/nats.go"
)

// LatticeAdapter bridges the agentic harness with Station's existing lattice infrastructure.
// It uses the existing WorkStore for state management and provides file storage via Object Store.
type LatticeAdapter struct {
	workStore *work.WorkStore
	fileStore *FileStore
	stationID string
	mu        sync.RWMutex
}

// LatticeAdapterConfig configures the lattice adapter.
type LatticeAdapterConfig struct {
	// StationID identifies this station
	StationID string

	// WorkStoreConfig for the lattice work store
	WorkStoreConfig work.WorkStoreConfig

	// FileStoreConfig for the object store
	FileStoreConfig FileStoreConfig
}

// FileStoreConfig configures the file object store.
type FileStoreConfig struct {
	// Bucket name for the object store
	Bucket string

	// MaxFileSize in bytes (default: 100MB)
	MaxFileSize int64

	// MaxBucketSize in bytes (default: 1GB)
	MaxBucketSize int64
}

// DefaultFileStoreConfig returns sensible defaults.
func DefaultFileStoreConfig() FileStoreConfig {
	return FileStoreConfig{
		Bucket:        "harness-files",
		MaxFileSize:   100 * 1024 * 1024,      // 100MB
		MaxBucketSize: 1 * 1024 * 1024 * 1024, // 1GB
	}
}

// NewLatticeAdapter creates a new adapter using existing lattice infrastructure.
func NewLatticeAdapter(js nats.JetStreamContext, config LatticeAdapterConfig) (*LatticeAdapter, error) {
	// Create or get the existing WorkStore
	workStore, err := work.NewWorkStore(js, config.WorkStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("create work store: %w", err)
	}

	// Create file store for harness-specific file sharing
	fileStore, err := NewFileStore(js, config.FileStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("create file store: %w", err)
	}

	return &LatticeAdapter{
		workStore: workStore,
		fileStore: fileStore,
		stationID: config.StationID,
	}, nil
}

// ==================== Work State Operations (using existing WorkStore) ====================

func (a *LatticeAdapter) CreateHarnessWork(ctx context.Context, input CreateHarnessWorkInput) (*work.WorkRecord, error) {
	contextMap := map[string]string{
		"harness":         "agentic",
		"workflow_run_id": input.WorkflowRunID,
		"step_id":         input.StepID,
	}
	if input.GitBranch != "" {
		contextMap["git_branch"] = input.GitBranch
	}

	record := &work.WorkRecord{
		WorkID:            input.WorkID,
		OrchestratorRunID: input.OrchestratorRunID,
		ParentWorkID:      input.ParentWorkID,
		SourceStation:     a.stationID,
		TargetStation:     a.stationID,
		AgentID:           input.AgentID,
		AgentName:         input.AgentName,
		Task:              input.Task,
		Context:           contextMap,
		Status:            work.StatusAssigned,
		AssignedAt:        time.Now(),
	}

	if err := a.workStore.Assign(ctx, record); err != nil {
		return nil, fmt.Errorf("assign work: %w", err)
	}

	return record, nil
}

type CreateHarnessWorkInput struct {
	WorkID            string
	WorkflowRunID     string
	StepID            string
	ParentWorkID      string
	AgentID           string
	AgentName         string
	Task              string
	OrchestratorRunID string
	GitBranch         string
}

// UpdateHarnessWorkStatus updates the status of a harness work record.
func (a *LatticeAdapter) UpdateHarnessWorkStatus(ctx context.Context, workID, status string, result *work.WorkResult) error {
	return a.workStore.UpdateStatus(ctx, workID, status, result)
}

// GetHarnessWork retrieves a harness work record.
func (a *LatticeAdapter) GetHarnessWork(ctx context.Context, workID string) (*work.WorkRecord, error) {
	return a.workStore.Get(ctx, workID)
}

// WatchHarnessWork watches for updates to a specific work record.
func (a *LatticeAdapter) WatchHarnessWork(ctx context.Context, workID string) (<-chan *work.WorkRecord, error) {
	return a.workStore.Watch(ctx, workID)
}

// GetWorkflowWork retrieves all work records for a workflow run.
func (a *LatticeAdapter) GetWorkflowWork(ctx context.Context, orchestratorRunID string) ([]*work.WorkRecord, error) {
	return a.workStore.GetByOrchestrator(ctx, orchestratorRunID)
}

// ==================== File Operations (new functionality) ====================

// UploadOutputFile uploads an output file from a harness run.
func (a *LatticeAdapter) UploadOutputFile(ctx context.Context, workID, localPath string) (*FileMetadata, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	filename := filepath.Base(localPath)
	key := fmt.Sprintf("work/%s/output/%s", workID, filename)

	return a.fileStore.Put(ctx, key, file, PutFileOptions{
		TTL: 24 * time.Hour,
		Metadata: map[string]string{
			"work_id":    workID,
			"local_path": localPath,
		},
	})
}

// DownloadOutputFile downloads an output file from a harness run.
func (a *LatticeAdapter) DownloadOutputFile(ctx context.Context, workID, filename, localPath string) error {
	key := fmt.Sprintf("work/%s/output/%s", workID, filename)

	reader, _, err := a.fileStore.Get(ctx, key)
	if err != nil {
		return err
	}
	if reader == nil {
		return fmt.Errorf("file not found: %s", key)
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ListOutputFiles lists output files from a harness run.
func (a *LatticeAdapter) ListOutputFiles(ctx context.Context, workID string) ([]*FileMetadata, error) {
	prefix := fmt.Sprintf("work/%s/output/", workID)
	return a.fileStore.List(ctx, prefix)
}

// UploadSharedFile uploads a file shared across workflow steps.
func (a *LatticeAdapter) UploadSharedFile(ctx context.Context, workflowRunID, key string, reader io.Reader, opts PutFileOptions) (*FileMetadata, error) {
	fullKey := fmt.Sprintf("workflow/%s/shared/%s", workflowRunID, key)
	return a.fileStore.Put(ctx, fullKey, reader, opts)
}

// DownloadSharedFile downloads a shared file.
func (a *LatticeAdapter) DownloadSharedFile(ctx context.Context, workflowRunID, key, localPath string) error {
	fullKey := fmt.Sprintf("workflow/%s/shared/%s", workflowRunID, key)

	reader, _, err := a.fileStore.Get(ctx, fullKey)
	if err != nil {
		return err
	}
	if reader == nil {
		return fmt.Errorf("file not found: %s", fullKey)
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// ==================== Handoff Operations ====================

// GetPreviousStepFiles gets output files from the previous workflow step.
func (a *LatticeAdapter) GetPreviousStepFiles(ctx context.Context, orchestratorRunID string) ([]*FileMetadata, error) {
	// Get all work records for the workflow
	records, err := a.workStore.GetByOrchestrator(ctx, orchestratorRunID)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, nil
	}

	// Find the most recently completed step
	var lastCompleted *work.WorkRecord
	for _, r := range records {
		if r.Status == work.StatusComplete {
			if lastCompleted == nil || r.CompletedAt.After(lastCompleted.CompletedAt) {
				lastCompleted = r
			}
		}
	}

	if lastCompleted == nil {
		return nil, nil
	}

	// List output files from that step
	return a.ListOutputFiles(ctx, lastCompleted.WorkID)
}

// DownloadPreviousStepFiles downloads all output files from the previous step to a local directory.
func (a *LatticeAdapter) DownloadPreviousStepFiles(ctx context.Context, orchestratorRunID, localDir string) (int, error) {
	files, err := a.GetPreviousStepFiles(ctx, orchestratorRunID)
	if err != nil {
		return 0, err
	}

	downloaded := 0
	for _, fileMeta := range files {
		// Extract filename from key (work/{id}/output/{filename})
		parts := strings.Split(fileMeta.Key, "/")
		if len(parts) < 4 {
			continue
		}
		filename := parts[len(parts)-1]
		workID := parts[1]

		localPath := filepath.Join(localDir, filename)
		if err := a.DownloadOutputFile(ctx, workID, filename, localPath); err != nil {
			continue
		}
		downloaded++
	}

	return downloaded, nil
}

// Close releases resources.
func (a *LatticeAdapter) Close() error {
	return a.fileStore.Close()
}

// ==================== FileStore (Object Store wrapper) ====================

// FileStore provides object storage for harness files.
type FileStore struct {
	js          nats.JetStreamContext
	objectStore nats.ObjectStore
	config      FileStoreConfig
	mu          sync.RWMutex
}

// NewFileStore creates a new file store.
func NewFileStore(js nats.JetStreamContext, config FileStoreConfig) (*FileStore, error) {
	if config.Bucket == "" {
		config.Bucket = "harness-files"
	}
	if config.MaxFileSize == 0 {
		config.MaxFileSize = 100 * 1024 * 1024
	}
	if config.MaxBucketSize == 0 {
		config.MaxBucketSize = 1 * 1024 * 1024 * 1024
	}

	return &FileStore{
		js:     js,
		config: config,
	}, nil
}

func (s *FileStore) getObjectStore(ctx context.Context) (nats.ObjectStore, error) {
	s.mu.RLock()
	if s.objectStore != nil {
		os := s.objectStore
		s.mu.RUnlock()
		return os, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.objectStore != nil {
		return s.objectStore, nil
	}

	os, err := s.js.ObjectStore(s.config.Bucket)
	if err == nil {
		s.objectStore = os
		return os, nil
	}

	if err == nats.ErrBucketNotFound || err == nats.ErrStreamNotFound || strings.Contains(err.Error(), "stream not found") {
		os, err = s.js.CreateObjectStore(&nats.ObjectStoreConfig{
			Bucket:      s.config.Bucket,
			Description: "Agentic harness file storage",
			MaxBytes:    s.config.MaxBucketSize,
		})
		if err != nil {
			return nil, fmt.Errorf("create object store %s: %w", s.config.Bucket, err)
		}
		s.objectStore = os
		return os, nil
	}

	return nil, fmt.Errorf("get object store %s: %w", s.config.Bucket, err)
}

// Put stores a file.
func (s *FileStore) Put(ctx context.Context, key string, reader io.Reader, opts PutFileOptions) (*FileMetadata, error) {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return nil, err
	}

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

// Get retrieves a file.
func (s *FileStore) Get(ctx context.Context, key string) (io.ReadCloser, *FileMetadata, error) {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return nil, nil, err
	}

	result, err := os.Get(key)
	if err != nil {
		if err == nats.ErrObjectNotFound {
			return nil, nil, nil
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

// Delete removes a file.
func (s *FileStore) Delete(ctx context.Context, key string) error {
	os, err := s.getObjectStore(ctx)
	if err != nil {
		return err
	}

	if err := os.Delete(key); err != nil && err != nats.ErrObjectNotFound {
		return fmt.Errorf("delete file %s: %w", key, err)
	}

	return nil
}

// List lists files with the given prefix.
func (s *FileStore) List(ctx context.Context, prefix string) ([]*FileMetadata, error) {
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

// Close releases resources.
func (s *FileStore) Close() error {
	return nil
}
