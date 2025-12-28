package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"station/internal/services"
)

// Workflow Definition Management Handlers

func (s *Server) handleListWorkflows(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflows, err := s.workflowService.ListWorkflows(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list workflows: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleGetWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'workflow_id' parameter: %v", err)), nil
	}

	version := int64(request.GetInt("version", 0))

	workflow, err := s.workflowService.GetWorkflow(ctx, workflowID, version)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Workflow not found: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"workflow": workflow,
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleCreateWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'workflow_id' parameter: %v", err)), nil
	}

	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	definition, err := request.RequireString("definition")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'definition' parameter: %v", err)), nil
	}

	description := request.GetString("description", "")

	workflow, validation, err := s.workflowService.CreateWorkflow(ctx, services.WorkflowDefinitionInput{
		WorkflowID:  workflowID,
		Name:        name,
		Description: description,
		Definition:  json.RawMessage(definition),
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create workflow: %v (validation: %+v)", err, validation)), nil
	}

	response := map[string]interface{}{
		"workflow":   workflow,
		"validation": validation,
		"message":    "Workflow created successfully",
	}

	if s.workflowExportService != nil {
		if err := s.workflowExportService.ExportWorkflowAfterSave(workflowID); err != nil {
			response["export_warning"] = fmt.Sprintf("Workflow created but export failed: %v", err)
		} else {
			response["exported"] = true
		}
	}

	result, _ := json.MarshalIndent(response, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleUpdateWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'workflow_id' parameter: %v", err)), nil
	}

	definition, err := request.RequireString("definition")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'definition' parameter: %v", err)), nil
	}

	name := request.GetString("name", "")
	description := request.GetString("description", "")

	workflow, validation, err := s.workflowService.UpdateWorkflow(ctx, services.WorkflowDefinitionInput{
		WorkflowID:  workflowID,
		Name:        name,
		Description: description,
		Definition:  json.RawMessage(definition),
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update workflow: %v (validation: %+v)", err, validation)), nil
	}

	response := map[string]interface{}{
		"workflow":   workflow,
		"validation": validation,
		"message":    fmt.Sprintf("Workflow updated to version %d", workflow.Version),
	}

	if s.workflowExportService != nil {
		if err := s.workflowExportService.ExportWorkflowAfterSave(workflowID); err != nil {
			response["export_warning"] = fmt.Sprintf("Workflow updated but export failed: %v", err)
		} else {
			response["exported"] = true
		}
	}

	result, _ := json.MarshalIndent(response, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleDeleteWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'workflow_id' parameter: %v", err)), nil
	}

	if err := s.workflowService.DisableWorkflow(ctx, workflowID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to disable workflow: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"workflow_id": workflowID,
		"message":     "Workflow disabled successfully",
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleValidateWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	definition, err := request.RequireString("definition")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'definition' parameter: %v", err)), nil
	}

	_, validation, parseErr := s.workflowService.ValidateDefinition(ctx, json.RawMessage(definition))

	resultMap := map[string]interface{}{
		"valid":      parseErr == nil,
		"validation": validation,
	}
	if parseErr != nil {
		resultMap["error"] = parseErr.Error()
	}

	result, _ := json.MarshalIndent(resultMap, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

// Workflow Run Management Handlers

func (s *Server) handleStartWorkflowRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'workflow_id' parameter: %v", err)), nil
	}

	version := int64(request.GetInt("version", 0))
	inputStr := request.GetString("input", "")

	var input json.RawMessage
	if inputStr != "" {
		input = json.RawMessage(inputStr)
	}

	run, validation, err := s.workflowService.StartRun(ctx, services.StartWorkflowRunRequest{
		WorkflowID: workflowID,
		Version:    version,
		Input:      input,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start workflow run: %v (validation: %+v)", err, validation)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"run":     run,
		"message": fmt.Sprintf("Workflow run started with ID: %s", run.RunID),
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleGetWorkflowRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	run, err := s.workflowService.GetRun(ctx, runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Workflow run not found: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"run": run,
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleListWorkflowRuns(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID := request.GetString("workflow_id", "")
	status := request.GetString("status", "")
	limit := int64(request.GetInt("limit", 50))

	runs, err := s.workflowService.ListRuns(ctx, workflowID, status, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list workflow runs: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"runs":  runs,
		"count": len(runs),
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleCancelWorkflowRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	reason := request.GetString("reason", "")

	run, err := s.workflowService.CancelRun(ctx, runID, reason)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to cancel workflow run: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"run":     run,
		"message": "Workflow run cancelled",
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handlePauseWorkflowRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	reason := request.GetString("reason", "")

	run, err := s.workflowService.PauseRun(ctx, runID, reason)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to pause workflow run: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"run":     run,
		"message": "Workflow run paused",
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleResumeWorkflowRun(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	run, err := s.workflowService.ResumeRun(ctx, runID, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to resume workflow run: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"run":     run,
		"message": "Workflow run resumed",
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleGetWorkflowRunSteps(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID, err := request.RequireString("run_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'run_id' parameter: %v", err)), nil
	}

	steps, err := s.workflowService.ListSteps(ctx, runID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list workflow run steps: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"steps": steps,
		"count": len(steps),
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

// Workflow Approval Management Handlers

func (s *Server) handleListWorkflowApprovals(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	runID := request.GetString("run_id", "")
	limit := int64(request.GetInt("limit", 50))

	approvals, err := s.workflowService.ListPendingApprovals(ctx, runID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list workflow approvals: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"approvals": approvals,
		"count":     len(approvals),
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleApproveWorkflowStep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	approvalID, err := request.RequireString("approval_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'approval_id' parameter: %v", err)), nil
	}

	comment := request.GetString("comment", "")
	approverID := request.GetString("approver_id", "mcp-user")

	approval, err := s.workflowService.ApproveWorkflowStep(ctx, services.ApproveWorkflowStepRequest{
		ApprovalID: approvalID,
		ApproverID: approverID,
		Comment:    comment,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to approve workflow step: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"approval": approval,
		"message":  "Workflow step approved",
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}

func (s *Server) handleRejectWorkflowStep(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	approvalID, err := request.RequireString("approval_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'approval_id' parameter: %v", err)), nil
	}

	reason := request.GetString("reason", "")
	rejecterID := request.GetString("rejecter_id", "mcp-user")

	approval, err := s.workflowService.RejectWorkflowStep(ctx, services.RejectWorkflowStepRequest{
		ApprovalID: approvalID,
		RejecterID: rejecterID,
		Reason:     reason,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to reject workflow step: %v", err)), nil
	}

	result, _ := json.MarshalIndent(map[string]interface{}{
		"approval": approval,
		"message":  "Workflow step rejected",
	}, "", "  ")

	return mcp.NewToolResultText(string(result)), nil
}
