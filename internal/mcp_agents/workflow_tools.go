package mcp_agents

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"station/internal/services"
	"station/internal/workflows"

	"github.com/mark3labs/mcp-go/mcp"
)

func (das *DynamicAgentServer) loadWorkflowsAsTools() error {
	if das.workflowService == nil {
		log.Printf("⚠️  Workflow service not configured, skipping workflow tools")
		return nil
	}

	workflowDefs, err := das.workflowService.ListWorkflows(context.Background())
	if err != nil {
		log.Printf("Failed to load workflows: %v", err)
		return err
	}

	log.Printf("🔄 Loading %d workflows as MCP tools", len(workflowDefs))

	for _, wfDef := range workflowDefs {
		if wfDef.Status != "active" {
			continue
		}

		var def workflows.Definition
		if err := json.Unmarshal(wfDef.Definition, &def); err != nil {
			log.Printf("  ⚠️  Failed to parse workflow '%s': %v", wfDef.WorkflowID, err)
			continue
		}

		toolName := "workflow_" + sanitizeToolName(wfDef.WorkflowID)
		description := fmt.Sprintf("Start workflow: %s", wfDef.Name)
		if wfDef.Description != nil && *wfDef.Description != "" {
			description = *wfDef.Description
		}

		log.Printf("  📋 Registering workflow '%s' as tool '%s'", wfDef.WorkflowID, toolName)

		toolOpts := []mcp.ToolOption{
			mcp.WithDescription(description + " (async - returns run_id immediately)"),
		}

		if def.InputSchema != nil && len(def.InputSchema) > 0 {
			if props, ok := def.InputSchema["properties"].(map[string]interface{}); ok {
				required := getRequiredFields(def.InputSchema)
				for propName, propDef := range props {
					propOpt := buildPropertyOption(propName, propDef, required)
					if propOpt != nil {
						toolOpts = append(toolOpts, propOpt)
					}
				}
			} else {
				toolOpts = append(toolOpts, mcp.WithString("input", mcp.Description("JSON input for the workflow")))
			}
		} else {
			toolOpts = append(toolOpts, mcp.WithString("input", mcp.Description("Optional JSON input for the workflow")))
		}

		tool := mcp.NewTool(toolName, toolOpts...)

		workflowID := wfDef.WorkflowID
		workflowVersion := wfDef.Version

		handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return das.handleWorkflowExecution(ctx, request, workflowID, workflowVersion)
		}

		das.mcpServer.AddTool(tool, handler)
	}

	das.addWorkflowStatusTool()

	return nil
}

func (das *DynamicAgentServer) handleWorkflowExecution(ctx context.Context, request mcp.CallToolRequest, workflowID string, version int64) (*mcp.CallToolResult, error) {
	var input json.RawMessage

	if request.Params.Arguments != nil {
		if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
			if rawInput, ok := argsMap["input"].(string); ok && rawInput != "" {
				input = json.RawMessage(rawInput)
			} else {
				inputBytes, err := json.Marshal(argsMap)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal input: %v", err)), nil
				}
				input = json.RawMessage(inputBytes)
			}
		}
	}

	var envID int64 = 1
	if env, err := das.repos.Environments.GetByName(das.environmentName); err == nil {
		envID = env.ID
	}

	run, validation, err := das.workflowService.StartRun(ctx, services.StartWorkflowRunRequest{
		WorkflowID:    workflowID,
		Version:       version,
		EnvironmentID: envID,
		Input:         input,
	})

	if err != nil {
		if len(validation.Errors) > 0 {
			var errMsgs []string
			for _, e := range validation.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", e.Code, e.Message))
			}
			return mcp.NewToolResultError(fmt.Sprintf("Workflow validation failed: %s", strings.Join(errMsgs, "; "))), nil
		}
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start workflow: %v", err)), nil
	}

	result := map[string]interface{}{
		"run_id":      run.RunID,
		"workflow_id": run.WorkflowID,
		"version":     run.WorkflowVersion,
		"status":      run.Status,
		"message":     "Workflow started successfully. Use get_workflow_run_status to check progress.",
	}

	resultJSON, _ := json.Marshal(result)
	log.Printf("Workflow '%s' started (Run ID: %s)", workflowID, run.RunID)

	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (das *DynamicAgentServer) addWorkflowStatusTool() {
	tool := mcp.NewTool("get_workflow_run_status",
		mcp.WithDescription("Get the status and result of a workflow run"),
		mcp.WithString("run_id", mcp.Required(), mcp.Description("The run_id returned when starting the workflow")),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		runID := request.GetString("run_id", "")
		if runID == "" {
			return mcp.NewToolResultError("run_id is required"), nil
		}

		run, err := das.workflowService.GetRun(ctx, runID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Workflow run not found: %v", err)), nil
		}

		result := map[string]interface{}{
			"run_id":      run.RunID,
			"workflow_id": run.WorkflowID,
			"version":     run.WorkflowVersion,
			"status":      run.Status,
			"started_at":  run.StartedAt,
		}

		if run.CurrentStep != nil {
			result["current_step"] = *run.CurrentStep
		}

		if run.CompletedAt != nil {
			result["completed_at"] = *run.CompletedAt
		}

		if run.Error != nil && *run.Error != "" {
			result["error"] = *run.Error
		}

		if run.Summary != nil && *run.Summary != "" {
			result["summary"] = *run.Summary
		}

		if run.Status == "completed" && len(run.Result) > 0 {
			var runResult interface{}
			if err := json.Unmarshal(run.Result, &runResult); err == nil {
				result["result"] = runResult
			}
		}

		resultJSON, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(resultJSON)), nil
	}

	das.mcpServer.AddTool(tool, handler)
	log.Printf("  📋 Registered tool 'get_workflow_run_status'")
}

func sanitizeToolName(id string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, id)

	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	return strings.Trim(result, "_")
}

func getRequiredFields(schema map[string]interface{}) map[string]bool {
	required := make(map[string]bool)
	if reqArr, ok := schema["required"].([]interface{}); ok {
		for _, r := range reqArr {
			if s, ok := r.(string); ok {
				required[s] = true
			}
		}
	}
	return required
}

func buildPropertyOption(name string, propDef interface{}, required map[string]bool) mcp.ToolOption {
	propMap, ok := propDef.(map[string]interface{})
	if !ok {
		return nil
	}

	desc := ""
	if d, ok := propMap["description"].(string); ok {
		desc = d
	}

	propType := "string"
	if t, ok := propMap["type"].(string); ok {
		propType = t
	}

	var opts []mcp.PropertyOption
	if desc != "" {
		opts = append(opts, mcp.Description(desc))
	}
	if required[name] {
		opts = append(opts, mcp.Required())
	}

	switch propType {
	case "integer", "number":
		return mcp.WithNumber(name, opts...)
	case "boolean":
		return mcp.WithBoolean(name, opts...)
	case "array":
		return mcp.WithArray(name, opts...)
	case "object":
		return mcp.WithObject(name, opts...)
	default:
		return mcp.WithString(name, opts...)
	}
}
