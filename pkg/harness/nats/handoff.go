package nats

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type HandoffManager struct {
	store *Store
}

func NewHandoffManager(store *Store) *HandoffManager {
	return &HandoffManager{store: store}
}

type StartWorkflowInput struct {
	WorkflowID    string
	WorkflowRunID string
	GitBranch     string
	SharedData    map[string]interface{}
}

func (h *HandoffManager) StartWorkflow(ctx context.Context, input StartWorkflowInput) (*WorkflowContext, error) {
	wctx := &WorkflowContext{
		WorkflowID:    input.WorkflowID,
		WorkflowRunID: input.WorkflowRunID,
		StartedAt:     time.Now(),
		GitBranch:     input.GitBranch,
		Steps:         []WorkflowStepSummary{},
		SharedData:    input.SharedData,
	}

	if err := h.store.SetWorkflowContext(ctx, wctx); err != nil {
		return nil, fmt.Errorf("save workflow context: %w", err)
	}

	return wctx, nil
}

type StartStepInput struct {
	WorkflowRunID string
	StepName      string
	AgentName     string
	RunID         string
	Task          string
}

func (h *HandoffManager) StartStep(ctx context.Context, input StartStepInput) (*RunState, *WorkflowContext, error) {
	wctx, err := h.store.GetWorkflowContext(ctx, input.WorkflowRunID)
	if err != nil {
		return nil, nil, err
	}
	if wctx == nil {
		return nil, nil, fmt.Errorf("workflow not found: %s", input.WorkflowRunID)
	}

	runState := &RunState{
		RunID:      input.RunID,
		AgentID:    input.AgentName,
		AgentName:  input.AgentName,
		WorkflowID: wctx.WorkflowID,
		StepName:   input.StepName,
		Status:     "running",
		StartedAt:  time.Now(),
		Task:       input.Task,
		GitBranch:  wctx.GitBranch,
	}

	if err := h.store.SetRunState(ctx, runState); err != nil {
		return nil, nil, fmt.Errorf("save run state: %w", err)
	}

	return runState, wctx, nil
}

type CompleteStepInput struct {
	RunID         string
	WorkflowRunID string
	Status        string
	Result        string
	Error         string
	Summary       string
	FilesModified []string
	Commits       []string
}

func (h *HandoffManager) CompleteStep(ctx context.Context, input CompleteStepInput) error {
	runState, err := h.store.GetRunState(ctx, input.RunID)
	if err != nil {
		return err
	}
	if runState == nil {
		return fmt.Errorf("run not found: %s", input.RunID)
	}

	now := time.Now()
	runState.Status = input.Status
	runState.Result = input.Result
	runState.Error = input.Error
	runState.CompletedAt = &now

	if err := h.store.SetRunState(ctx, runState); err != nil {
		return fmt.Errorf("update run state: %w", err)
	}

	stepSummary := WorkflowStepSummary{
		StepName:      runState.StepName,
		AgentName:     runState.AgentName,
		RunID:         runState.RunID,
		Status:        input.Status,
		StartedAt:     runState.StartedAt,
		CompletedAt:   &now,
		Summary:       input.Summary,
		FilesModified: input.FilesModified,
		Commits:       input.Commits,
	}

	if err := h.store.AddWorkflowStep(ctx, input.WorkflowRunID, stepSummary); err != nil {
		return fmt.Errorf("add workflow step: %w", err)
	}

	return nil
}

type PreviousStepContext struct {
	StepName      string
	AgentName     string
	Status        string
	Summary       string
	FilesModified []string
	Commits       []string
	OutputFiles   []*FileMetadata
}

func (h *HandoffManager) GetPreviousStepContext(ctx context.Context, workflowRunID string) (*PreviousStepContext, error) {
	wctx, err := h.store.GetWorkflowContext(ctx, workflowRunID)
	if err != nil {
		return nil, err
	}
	if wctx == nil || len(wctx.Steps) == 0 {
		return nil, nil
	}

	lastStep := wctx.Steps[len(wctx.Steps)-1]

	outputFiles, err := h.store.ListRunFiles(ctx, lastStep.RunID)
	if err != nil {
		outputFiles = nil
	}

	return &PreviousStepContext{
		StepName:      lastStep.StepName,
		AgentName:     lastStep.AgentName,
		Status:        lastStep.Status,
		Summary:       lastStep.Summary,
		FilesModified: lastStep.FilesModified,
		Commits:       lastStep.Commits,
		OutputFiles:   outputFiles,
	}, nil
}

func (h *HandoffManager) UploadOutputFile(ctx context.Context, runID, localPath string) (*FileMetadata, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	filename := filepath.Base(localPath)
	return h.store.PutRunFile(ctx, runID, filename, file, PutFileOptions{
		TTL: 24 * time.Hour,
		Metadata: map[string]string{
			"local_path": localPath,
		},
	})
}

func (h *HandoffManager) DownloadOutputFile(ctx context.Context, runID, filename, localPath string) error {
	reader, _, err := h.store.GetRunFile(ctx, runID, filename)
	if err != nil {
		return err
	}
	if reader == nil {
		return fmt.Errorf("file not found: %s/%s", runID, filename)
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

func (h *HandoffManager) DownloadPreviousOutputs(ctx context.Context, workflowRunID, localDir string) (int, error) {
	prevCtx, err := h.GetPreviousStepContext(ctx, workflowRunID)
	if err != nil {
		return 0, err
	}
	if prevCtx == nil || len(prevCtx.OutputFiles) == 0 {
		return 0, nil
	}

	wctx, err := h.store.GetWorkflowContext(ctx, workflowRunID)
	if err != nil {
		return 0, err
	}
	lastStep := wctx.Steps[len(wctx.Steps)-1]

	downloaded := 0
	for _, fileMeta := range prevCtx.OutputFiles {
		filename := filepath.Base(fileMeta.Key)
		localPath := filepath.Join(localDir, filename)

		if err := h.DownloadOutputFile(ctx, lastStep.RunID, filename, localPath); err != nil {
			continue
		}
		downloaded++
	}

	return downloaded, nil
}

type SharedDataUpdate struct {
	Key   string
	Value interface{}
}

func (h *HandoffManager) UpdateSharedData(ctx context.Context, workflowRunID string, updates ...SharedDataUpdate) error {
	wctx, err := h.store.GetWorkflowContext(ctx, workflowRunID)
	if err != nil {
		return err
	}
	if wctx == nil {
		return fmt.Errorf("workflow not found: %s", workflowRunID)
	}

	if wctx.SharedData == nil {
		wctx.SharedData = make(map[string]interface{})
	}

	for _, update := range updates {
		wctx.SharedData[update.Key] = update.Value
	}

	return h.store.SetWorkflowContext(ctx, wctx)
}

func (h *HandoffManager) GetSharedData(ctx context.Context, workflowRunID, key string) (interface{}, error) {
	wctx, err := h.store.GetWorkflowContext(ctx, workflowRunID)
	if err != nil {
		return nil, err
	}
	if wctx == nil {
		return nil, nil
	}

	return wctx.SharedData[key], nil
}

type PreloadFile struct {
	Key       string
	LocalPath string
	TTL       time.Duration
}

func (h *HandoffManager) PreloadFiles(ctx context.Context, files []PreloadFile) error {
	for _, f := range files {
		data, err := os.ReadFile(f.LocalPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", f.LocalPath, err)
		}

		ttl := f.TTL
		if ttl == 0 {
			ttl = 24 * time.Hour
		}

		_, err = h.store.PutSharedFile(ctx, f.Key, bytes.NewReader(data), PutFileOptions{
			TTL: ttl,
			Metadata: map[string]string{
				"local_path": f.LocalPath,
			},
		})
		if err != nil {
			return fmt.Errorf("upload %s: %w", f.Key, err)
		}
	}

	return nil
}

func (h *HandoffManager) DownloadPreloadedFile(ctx context.Context, key, localPath string) error {
	reader, _, err := h.store.GetSharedFile(ctx, key)
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
