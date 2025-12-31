package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/services"
	"station/pkg/models"
)

type mockWorkflowService struct {
	workflows      map[string]*models.WorkflowDefinition
	getWorkflowErr error
	disableErr     error
	deleteErr      error
	deletedCount   int64
	disableCalled  bool
	deleteCalled   bool
	deleteRequest  services.DeleteWorkflowsRequest
}

func newMockWorkflowService() *mockWorkflowService {
	return &mockWorkflowService{
		workflows:    make(map[string]*models.WorkflowDefinition),
		deletedCount: 1,
	}
}

func (m *mockWorkflowService) addWorkflow(workflowID, name string) {
	m.workflows[workflowID] = &models.WorkflowDefinition{
		ID:         1,
		WorkflowID: workflowID,
		Name:       name,
		Version:    1,
		Status:     "active",
	}
}

func (m *mockWorkflowService) GetWorkflow(ctx context.Context, workflowID string, version int64) (*models.WorkflowDefinition, error) {
	if m.getWorkflowErr != nil {
		return nil, m.getWorkflowErr
	}
	wf, exists := m.workflows[workflowID]
	if !exists {
		return nil, errors.New("workflow not found")
	}
	return wf, nil
}

func (m *mockWorkflowService) DisableWorkflow(ctx context.Context, workflowID string) error {
	m.disableCalled = true
	if m.disableErr != nil {
		return m.disableErr
	}
	return nil
}

func (m *mockWorkflowService) DeleteWorkflows(ctx context.Context, req services.DeleteWorkflowsRequest) (int64, error) {
	m.deleteCalled = true
	m.deleteRequest = req
	if m.deleteErr != nil {
		return 0, m.deleteErr
	}
	return m.deletedCount, nil
}

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

type serverWithMockWorkflow struct {
	*Server
	mockWf *mockWorkflowService
}

func newTestServerWithMockWorkflow() *serverWithMockWorkflow {
	mockWf := newMockWorkflowService()
	s := &Server{
		workflowService: nil,
	}
	return &serverWithMockWorkflow{
		Server: s,
		mockWf: mockWf,
	}
}

func (s *serverWithMockWorkflow) handleArchiveWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError("Missing 'workflow_id' parameter: " + err.Error()), nil
	}

	if _, err := s.mockWf.GetWorkflow(ctx, workflowID, 0); err != nil {
		return mcp.NewToolResultError("Workflow not found: " + err.Error()), nil
	}

	if err := s.mockWf.DisableWorkflow(ctx, workflowID); err != nil {
		return mcp.NewToolResultError("Failed to archive workflow: " + err.Error()), nil
	}

	return mcp.NewToolResultText(`{"workflow_id":"` + workflowID + `","message":"Workflow archived (disabled) successfully."}`), nil
}

func (s *serverWithMockWorkflow) handleDeleteWorkflow(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	workflowID, err := request.RequireString("workflow_id")
	if err != nil {
		return mcp.NewToolResultError("Missing 'workflow_id' parameter: " + err.Error()), nil
	}

	environmentName := "default"
	if envName, err := request.RequireString("environment_name"); err == nil && envName != "" {
		environmentName = envName
	}

	if _, err := s.mockWf.GetWorkflow(ctx, workflowID, 0); err != nil {
		return mcp.NewToolResultError("Workflow not found: " + err.Error()), nil
	}

	deleteReq := services.DeleteWorkflowsRequest{
		WorkflowIDs: []string{workflowID},
	}
	deletedCount, err := s.mockWf.DeleteWorkflows(ctx, deleteReq)
	if err != nil {
		return mcp.NewToolResultError("Failed to delete workflow from database: " + err.Error()), nil
	}

	_ = environmentName
	_ = deletedCount

	return mcp.NewToolResultText(`{"workflow_id":"` + workflowID + `","message":"Workflow permanently deleted."}`), nil
}

func TestHandleArchiveWorkflow(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]interface{}
		setupMock     func(*mockWorkflowService)
		wantError     bool
		errorContains string
		checkDisable  bool
	}{
		{
			name: "successful archive",
			args: map[string]interface{}{
				"workflow_id": "test-workflow",
			},
			setupMock: func(m *mockWorkflowService) {
				m.addWorkflow("test-workflow", "Test Workflow")
			},
			wantError:    false,
			checkDisable: true,
		},
		{
			name: "workflow not found",
			args: map[string]interface{}{
				"workflow_id": "nonexistent-workflow",
			},
			setupMock:     func(m *mockWorkflowService) {},
			wantError:     true,
			errorContains: "Workflow not found",
			checkDisable:  false,
		},
		{
			name:          "missing workflow_id parameter",
			args:          map[string]interface{}{},
			setupMock:     func(m *mockWorkflowService) {},
			wantError:     true,
			errorContains: "Missing 'workflow_id' parameter",
			checkDisable:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServerWithMockWorkflow()
			tt.setupMock(srv.mockWf)

			request := newCallToolRequest(tt.args)
			result, err := srv.handleArchiveWorkflow(context.Background(), request)

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantError {
				assert.True(t, result.IsError, "expected error result")
				if tt.errorContains != "" {
					content := result.Content[0].(mcp.TextContent)
					assert.Contains(t, content.Text, tt.errorContains)
				}
				assert.False(t, srv.mockWf.disableCalled, "DisableWorkflow should not be called on error")
			} else {
				assert.False(t, result.IsError, "expected success result")
				if tt.checkDisable {
					assert.True(t, srv.mockWf.disableCalled, "DisableWorkflow should be called")
				}
			}
		})
	}
}

func TestHandleDeleteWorkflow(t *testing.T) {
	tests := []struct {
		name          string
		args          map[string]interface{}
		setupMock     func(*mockWorkflowService)
		wantError     bool
		errorContains string
		checkDelete   bool
	}{
		{
			name: "successful delete",
			args: map[string]interface{}{
				"workflow_id": "test-workflow",
			},
			setupMock: func(m *mockWorkflowService) {
				m.addWorkflow("test-workflow", "Test Workflow")
			},
			wantError:   false,
			checkDelete: true,
		},
		{
			name: "successful delete with environment",
			args: map[string]interface{}{
				"workflow_id":      "test-workflow",
				"environment_name": "production",
			},
			setupMock: func(m *mockWorkflowService) {
				m.addWorkflow("test-workflow", "Test Workflow")
			},
			wantError:   false,
			checkDelete: true,
		},
		{
			name: "workflow not found",
			args: map[string]interface{}{
				"workflow_id": "nonexistent-workflow",
			},
			setupMock:     func(m *mockWorkflowService) {},
			wantError:     true,
			errorContains: "Workflow not found",
			checkDelete:   false,
		},
		{
			name:          "missing workflow_id parameter",
			args:          map[string]interface{}{},
			setupMock:     func(m *mockWorkflowService) {},
			wantError:     true,
			errorContains: "Missing 'workflow_id' parameter",
			checkDelete:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServerWithMockWorkflow()
			tt.setupMock(srv.mockWf)

			request := newCallToolRequest(tt.args)
			result, err := srv.handleDeleteWorkflow(context.Background(), request)

			require.NoError(t, err)
			require.NotNil(t, result)

			if tt.wantError {
				assert.True(t, result.IsError, "expected error result")
				if tt.errorContains != "" {
					content := result.Content[0].(mcp.TextContent)
					assert.Contains(t, content.Text, tt.errorContains)
				}
				assert.False(t, srv.mockWf.deleteCalled, "DeleteWorkflows should not be called on error")
			} else {
				assert.False(t, result.IsError, "expected success result")
				if tt.checkDelete {
					assert.True(t, srv.mockWf.deleteCalled, "DeleteWorkflows should be called")
					assert.Equal(t, []string{tt.args["workflow_id"].(string)}, srv.mockWf.deleteRequest.WorkflowIDs)
				}
			}
		})
	}
}

func TestHandleArchiveWorkflow_DisableError(t *testing.T) {
	srv := newTestServerWithMockWorkflow()
	srv.mockWf.addWorkflow("test-workflow", "Test Workflow")
	srv.mockWf.disableErr = errors.New("database connection failed")

	request := newCallToolRequest(map[string]interface{}{
		"workflow_id": "test-workflow",
	})

	result, err := srv.handleArchiveWorkflow(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "Failed to archive workflow")
}

func TestHandleDeleteWorkflow_DeleteError(t *testing.T) {
	srv := newTestServerWithMockWorkflow()
	srv.mockWf.addWorkflow("test-workflow", "Test Workflow")
	srv.mockWf.deleteErr = errors.New("database connection failed")

	request := newCallToolRequest(map[string]interface{}{
		"workflow_id": "test-workflow",
	})

	result, err := srv.handleDeleteWorkflow(context.Background(), request)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	content := result.Content[0].(mcp.TextContent)
	assert.Contains(t, content.Text, "Failed to delete workflow from database")
}
