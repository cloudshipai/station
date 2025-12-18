package services

import (
	"context"
	"encoding/json"
	"testing"

	"station/internal/db"
	"station/internal/db/repositories"
)

func TestWorkflowServiceCreateAndStartRun(t *testing.T) {
	ctx := context.Background()
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	svc := NewWorkflowService(repos)

	definition := json.RawMessage(`{
		"id": "demo-workflow",
		"start": "start",
		"states": [
			{"id":"start","type":"operation","input":{},"output":{},"timeout":"5m","retry":{"max_attempts":3},"transition":"finish"},
			{"id":"finish","type":"operation","input":{},"output":{},"timeout":"1m","retry":{"max_attempts":1}}
		]
	}`)

	record, validation, err := svc.CreateWorkflow(ctx, WorkflowDefinitionInput{
		WorkflowID: "demo-workflow",
		Name:       "Demo Workflow",
		Definition: definition,
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected no validation errors: %+v", validation.Errors)
	}
	if record.Version != 1 {
		t.Fatalf("expected version 1, got %d", record.Version)
	}

	run, validation, err := svc.StartRun(ctx, StartWorkflowRunRequest{
		WorkflowID: "demo-workflow",
		Input:      json.RawMessage(`{"hello":"world"}`),
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if len(validation.Errors) != 0 {
		t.Fatalf("expected no validation errors when starting run: %+v", validation.Errors)
	}
	if run.WorkflowVersion != record.Version {
		t.Fatalf("expected workflow version %d, got %d", record.Version, run.WorkflowVersion)
	}
	if run.CurrentStep == nil || *run.CurrentStep != "start" {
		t.Fatalf("expected current step 'start', got %+v", run.CurrentStep)
	}
}
