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

func TestWorkflowServiceApprovalFlow(t *testing.T) {
	ctx := context.Background()
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	svc := NewWorkflowService(repos)

	definition := json.RawMessage(`{
		"id": "approval-workflow",
		"start": "request_approval",
		"states": [
			{"id":"request_approval","type":"operation","input":{},"output":{},"timeout":"5m","retry":{"max_attempts":1},"end":true}
		]
	}`)

	_, _, err = svc.CreateWorkflow(ctx, WorkflowDefinitionInput{
		WorkflowID: "approval-workflow",
		Name:       "Approval Workflow",
		Definition: definition,
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	run, _, err := svc.StartRun(ctx, StartWorkflowRunRequest{
		WorkflowID: "approval-workflow",
		Input:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	approvalID := "test-approval-1"
	_, err = repos.WorkflowApprovals.Create(ctx, repositories.CreateWorkflowApprovalParams{
		ApprovalID: approvalID,
		RunID:      run.RunID,
		StepID:     "request_approval",
		Message:    "Please approve this deployment",
	})
	if err != nil {
		t.Fatalf("failed to create approval: %v", err)
	}

	t.Run("GetApproval", func(t *testing.T) {
		approval, err := svc.GetApproval(ctx, approvalID)
		if err != nil {
			t.Fatalf("GetApproval returned error: %v", err)
		}
		if approval.ApprovalID != approvalID {
			t.Errorf("expected approval ID %s, got %s", approvalID, approval.ApprovalID)
		}
		if approval.Status != "pending" {
			t.Errorf("expected status pending, got %s", approval.Status)
		}
	})

	t.Run("ListPendingApprovals_ByRun", func(t *testing.T) {
		approvals, err := svc.ListPendingApprovals(ctx, run.RunID, 10)
		if err != nil {
			t.Fatalf("ListPendingApprovals returned error: %v", err)
		}
		if len(approvals) != 1 {
			t.Errorf("expected 1 pending approval, got %d", len(approvals))
		}
	})

	t.Run("ListPendingApprovals_Global", func(t *testing.T) {
		approvals, err := svc.ListPendingApprovals(ctx, "", 50)
		if err != nil {
			t.Fatalf("ListPendingApprovals returned error: %v", err)
		}
		if len(approvals) < 1 {
			t.Errorf("expected at least 1 pending approval, got %d", len(approvals))
		}
	})

	t.Run("ApproveWorkflowStep", func(t *testing.T) {
		approval, err := svc.ApproveWorkflowStep(ctx, ApproveWorkflowStepRequest{
			ApprovalID: approvalID,
			ApproverID: "user@example.com",
			Comment:    "Looks good, approved!",
		})
		if err != nil {
			t.Fatalf("ApproveWorkflowStep returned error: %v", err)
		}
		if approval.Status != "approved" {
			t.Errorf("expected status approved, got %s", approval.Status)
		}
		if approval.DecidedBy == nil || *approval.DecidedBy != "user@example.com" {
			t.Errorf("expected decided_by user@example.com, got %v", approval.DecidedBy)
		}
	})

	t.Run("ApproveWorkflowStep_AlreadyApproved", func(t *testing.T) {
		_, err := svc.ApproveWorkflowStep(ctx, ApproveWorkflowStepRequest{
			ApprovalID: approvalID,
			ApproverID: "another@example.com",
		})
		if err == nil {
			t.Fatal("expected error when approving already approved request")
		}
	})
}

func TestWorkflowServiceRejectApproval(t *testing.T) {
	ctx := context.Background()
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	svc := NewWorkflowService(repos)

	definition := json.RawMessage(`{
		"id": "reject-workflow",
		"start": "step1",
		"states": [{"id":"step1","type":"operation","input":{},"output":{},"timeout":"1m","end":true}]
	}`)

	_, _, err = svc.CreateWorkflow(ctx, WorkflowDefinitionInput{
		WorkflowID: "reject-workflow",
		Name:       "Reject Workflow",
		Definition: definition,
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	run, _, err := svc.StartRun(ctx, StartWorkflowRunRequest{
		WorkflowID: "reject-workflow",
		Input:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	approvalID := "reject-approval-1"
	_, err = repos.WorkflowApprovals.Create(ctx, repositories.CreateWorkflowApprovalParams{
		ApprovalID: approvalID,
		RunID:      run.RunID,
		StepID:     "step1",
		Message:    "Please review",
	})
	if err != nil {
		t.Fatalf("failed to create approval: %v", err)
	}

	t.Run("RejectWorkflowStep", func(t *testing.T) {
		approval, err := svc.RejectWorkflowStep(ctx, RejectWorkflowStepRequest{
			ApprovalID: approvalID,
			RejecterID: "reviewer@example.com",
			Reason:     "Not meeting requirements",
		})
		if err != nil {
			t.Fatalf("RejectWorkflowStep returned error: %v", err)
		}
		if approval.Status != "rejected" {
			t.Errorf("expected status rejected, got %s", approval.Status)
		}
		if approval.DecidedBy == nil || *approval.DecidedBy != "reviewer@example.com" {
			t.Errorf("expected decided_by reviewer@example.com, got %v", approval.DecidedBy)
		}
		if approval.DecisionReason == nil || *approval.DecisionReason != "Not meeting requirements" {
			t.Errorf("expected decision_reason, got %v", approval.DecisionReason)
		}
	})

	t.Run("RejectWorkflowStep_AlreadyRejected", func(t *testing.T) {
		_, err := svc.RejectWorkflowStep(ctx, RejectWorkflowStepRequest{
			ApprovalID: approvalID,
			RejecterID: "another@example.com",
		})
		if err == nil {
			t.Fatal("expected error when rejecting already rejected request")
		}
	})
}

func TestWorkflowServiceListApprovalsByRun(t *testing.T) {
	ctx := context.Background()
	testDB, err := db.NewTest(t)
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	defer testDB.Close()

	repos := repositories.New(testDB)
	svc := NewWorkflowService(repos)

	definition := json.RawMessage(`{
		"id": "list-approvals-workflow",
		"start": "step1",
		"states": [{"id":"step1","type":"operation","input":{},"output":{},"timeout":"1m","end":true}]
	}`)

	_, _, err = svc.CreateWorkflow(ctx, WorkflowDefinitionInput{
		WorkflowID: "list-approvals-workflow",
		Name:       "List Approvals Workflow",
		Definition: definition,
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	run, _, err := svc.StartRun(ctx, StartWorkflowRunRequest{
		WorkflowID: "list-approvals-workflow",
		Input:      json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	for i := 1; i <= 3; i++ {
		_, err = repos.WorkflowApprovals.Create(ctx, repositories.CreateWorkflowApprovalParams{
			ApprovalID: "list-approval-" + string(rune('0'+i)),
			RunID:      run.RunID,
			StepID:     "step1",
			Message:    "Approval request",
		})
		if err != nil {
			t.Fatalf("failed to create approval %d: %v", i, err)
		}
	}

	approvals, err := svc.ListApprovalsByRun(ctx, run.RunID)
	if err != nil {
		t.Fatalf("ListApprovalsByRun returned error: %v", err)
	}
	if len(approvals) != 3 {
		t.Errorf("expected 3 approvals, got %d", len(approvals))
	}
}
