package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"station/internal/config"
	"station/internal/db/repositories"
)

type WorkflowExportService struct {
	repos           *repositories.Repositories
	workflowService *WorkflowService
}

func NewWorkflowExportService(repos *repositories.Repositories, workflowService *WorkflowService) *WorkflowExportService {
	return &WorkflowExportService{
		repos:           repos,
		workflowService: workflowService,
	}
}

func (s *WorkflowExportService) ExportWorkflowAfterSave(workflowID string) error {
	return s.ExportWorkflowAfterSaveWithEnvironment(workflowID, "default")
}

func (s *WorkflowExportService) ExportWorkflowAfterSaveWithEnvironment(workflowID string, environmentName string) error {
	ctx := context.Background()

	wf, err := s.workflowService.GetWorkflow(ctx, workflowID, 0)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	}

	var defMap map[string]interface{}
	if err := json.Unmarshal(wf.Definition, &defMap); err != nil {
		return fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	yamlBytes, err := yaml.Marshal(defMap)
	if err != nil {
		return fmt.Errorf("failed to convert to YAML: %w", err)
	}

	outputPath := config.GetWorkflowFilePath(environmentName, workflowID)

	workflowsDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		return fmt.Errorf("failed to create workflows directory: %w", err)
	}

	if err := os.WriteFile(outputPath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("failed to write workflow file: %w", err)
	}

	log.Printf("Workflow '%s' (v%d) successfully exported to: %s", wf.WorkflowID, wf.Version, outputPath)
	return nil
}
