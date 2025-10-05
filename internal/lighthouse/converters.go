package lighthouse

import (
	"encoding/json"
	"station/internal/lighthouse/proto"
	"station/pkg/types"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// convertAgentRunToProto converts Station AgentRun to proto LighthouseAgentRunData
func convertAgentRunToProto(run *types.AgentRun) *proto.LighthouseAgentRunData {
	if run == nil {
		return nil
	}

	metadata := run.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Add preset information to metadata
	if run.OutputSchemaPreset != "" {
		metadata["output_schema_preset"] = run.OutputSchemaPreset
	}
	if run.OutputSchema != "" {
		metadata["has_output_schema"] = "true"
	}

	// Add preset information to metadata
	if run.OutputSchemaPreset != "" {
		metadata["output_schema_preset"] = run.OutputSchemaPreset
	}
	if run.OutputSchema != "" {
		metadata["has_output_schema"] = "true"
	}

	return &proto.LighthouseAgentRunData{
		RunId:          run.ID,
		AgentId:        run.AgentID,
		AgentName:      run.AgentName,
		Task:           run.Task,
		Response:       run.Response,
		ToolCalls:      convertToolCallsToLighthouseProto(run.ToolCalls),
		ExecutionSteps: convertExecutionStepsToProto(run.ExecutionSteps),
		TokenUsage:     convertTokenUsageToLighthouseProto(run.TokenUsage),
		DurationMs:     run.DurationMs,
		ModelName:      run.ModelName,
		Status:         convertRunStatusToLighthouseProto(run.Status),
		StartedAt:      timestampFromTime(run.StartedAt),
		CompletedAt:    timestampFromTime(run.CompletedAt),
		Metadata:       metadata,
		StationVersion: "v0.11.0", // Station version for debugging/compatibility
	}
}

// convertToolCallsToProto converts Station tool calls to proto format
func convertToolCallsToProto(toolCalls []types.ToolCall) []*proto.ToolCall {
	if toolCalls == nil {
		return nil
	}

	protoToolCalls := make([]*proto.ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		// Convert parameters and result to protobuf Struct
		parametersStruct, _ := convertToProtoStruct(tc.Parameters)
		resultStruct, _ := convertStringToProtoStruct(tc.Result)

		protoToolCalls[i] = &proto.ToolCall{
			ToolName:   tc.ToolName,
			Parameters: parametersStruct,
			Result:     resultStruct,
			DurationMs: tc.DurationMs,
			Success:    tc.Success,
			Timestamp:  timestamppb.New(tc.Timestamp),
		}
	}
	return protoToolCalls
}

// convertToolCallsToLighthouseProto converts Station tool calls to lighthouse proto format
func convertToolCallsToLighthouseProto(toolCalls []types.ToolCall) []*proto.LighthouseToolCall {
	if toolCalls == nil {
		return nil
	}

	protoToolCalls := make([]*proto.LighthouseToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		protoToolCalls[i] = &proto.LighthouseToolCall{
			ToolName:   tc.ToolName,
			Parameters: convertToStringMap(tc.Parameters),
			Result:     tc.Result,
			DurationMs: tc.DurationMs,
			Success:    tc.Success,
			Timestamp:  timestampFromTime(tc.Timestamp),
		}
	}
	return protoToolCalls
}

// convertExecutionStepsToProto converts Station execution steps to proto format
func convertExecutionStepsToProto(steps []types.ExecutionStep) []*proto.ExecutionStep {
	if steps == nil {
		return nil
	}

	protoSteps := make([]*proto.ExecutionStep, len(steps))
	for i, step := range steps {
		protoSteps[i] = &proto.ExecutionStep{
			StepNumber:  int32(step.StepNumber),
			Description: step.Description,
			Type:        convertStepTypeToProto(step.Type),
			DurationMs:  step.DurationMs,
			Timestamp:   timestampFromTime(step.Timestamp),
		}
	}
	return protoSteps
}

// convertTokenUsageToProto converts Station token usage to proto format
func convertTokenUsageToProto(usage *types.TokenUsage) *proto.TokenUsage {
	if usage == nil {
		return nil
	}

	return &proto.TokenUsage{
		PromptTokens:     int32(usage.PromptTokens),
		CompletionTokens: int32(usage.CompletionTokens),
		TotalTokens:      int32(usage.TotalTokens),
		CostUsd:          usage.CostUSD,
	}
}

// convertDeploymentContextToProto converts deployment context to proto format
func convertDeploymentContextToProto(context *types.DeploymentContext) *proto.DeploymentContext {
	if context == nil {
		return nil
	}

	return &proto.DeploymentContext{
		CommandLine:      context.CommandLine,
		WorkingDirectory: context.WorkingDirectory,
		EnvVars:          context.EnvVars,
		Arguments:        context.Arguments,
		GitBranch:        context.GitBranch,
		GitCommit:        context.GitCommit,
		StationVersion:   context.StationVersion,
	}
}

// convertSystemSnapshotToProto converts system snapshot to proto format
func convertSystemSnapshotToProto(snapshot *types.SystemSnapshot) *proto.SystemSnapshot {
	if snapshot == nil {
		return nil
	}

	return &proto.SystemSnapshot{
		Agents:         convertAgentConfigsToProto(snapshot.Agents),
		McpServers:     convertMCPConfigsToProto(snapshot.MCPServers),
		Variables:      snapshot.Variables,
		AvailableTools: convertToolInfosToProto(snapshot.AvailableTools),
		Metrics:        convertSystemMetricsToProto(snapshot.Metrics),
	}
}

// convertAgentConfigsToProto converts agent configs to proto format
func convertAgentConfigsToProto(agents []types.AgentConfig) []*proto.AgentConfig {
	if agents == nil {
		return nil
	}

	protoAgents := make([]*proto.AgentConfig, len(agents))
	for i, agent := range agents {
		protoAgents[i] = &proto.AgentConfig{
			Id:             agent.ID,
			Name:           agent.Name,
			Description:    agent.Description,
			PromptTemplate: agent.PromptTemplate,
			ModelName:      agent.ModelName,
			MaxSteps:       int32(agent.MaxSteps),
			Tools:          agent.Tools,
			Variables:      agent.Variables,
			Tags:           agent.Tags,
			CreatedAt:      timestampFromTime(agent.CreatedAt),
			UpdatedAt:      timestampFromTime(agent.UpdatedAt),
		}
	}
	return protoAgents
}

// convertMCPConfigsToProto converts MCP configs to proto format
func convertMCPConfigsToProto(mcpServers []types.MCPConfig) []*proto.MCPConfig {
	if mcpServers == nil {
		return nil
	}

	protoServers := make([]*proto.MCPConfig, len(mcpServers))
	for i, server := range mcpServers {
		protoServers[i] = &proto.MCPConfig{
			Name:      server.Name,
			Command:   server.Command,
			Args:      server.Args,
			EnvVars:   server.EnvVars,
			Variables: server.Variables,
			Enabled:   server.Enabled,
			CreatedAt: timestampFromTime(server.CreatedAt),
			UpdatedAt: timestampFromTime(server.UpdatedAt),
		}
	}
	return protoServers
}

// convertToolInfosToProto converts tool info to proto format
func convertToolInfosToProto(tools []types.ToolInfo) []*proto.ToolInfo {
	if tools == nil {
		return nil
	}

	protoTools := make([]*proto.ToolInfo, len(tools))
	for i, tool := range tools {
		protoTools[i] = &proto.ToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			McpServer:   tool.MCPServer,
			Categories:  tool.Categories,
		}
	}
	return protoTools
}

// convertSystemMetricsToProto converts system metrics to proto format
func convertSystemMetricsToProto(metrics *types.SystemMetrics) *proto.SystemMetrics {
	if metrics == nil {
		return nil
	}

	return &proto.SystemMetrics{
		CpuUsagePercent:    metrics.CPUUsagePercent,
		MemoryUsagePercent: metrics.MemoryUsagePercent,
		DiskUsageMb:        metrics.DiskUsageMB,
		UptimeSeconds:      metrics.UptimeSeconds,
		ActiveConnections:  int32(metrics.ActiveConnections),
		ActiveRuns:         int32(metrics.ActiveRuns),
		NetworkInBytes:     metrics.NetworkInBytes,
		NetworkOutBytes:    metrics.NetworkOutBytes,
		AdditionalMetrics:  metrics.AdditionalMetrics,
	}
}

// Helper conversion functions

func ConvertRunStatusToProto(status string) proto.RunStatus {
	switch status {
	case "completed":
		// TODO: Remove debug logging after fixing status issue
		return proto.RunStatus_RUN_STATUS_COMPLETED
	case "failed":
		return proto.RunStatus_RUN_STATUS_FAILED
	case "timeout":
		return proto.RunStatus_RUN_STATUS_FAILED
	case "cancelled":
		return proto.RunStatus_RUN_STATUS_CANCELLED
	default:
		return proto.RunStatus_RUN_STATUS_UNKNOWN
	}
}

func convertStepTypeToProto(stepType string) proto.StepType {
	switch stepType {
	case "tool_call":
		return proto.StepType_STEP_TYPE_TOOL_CALL
	case "llm_call":
		return proto.StepType_STEP_TYPE_LLM_CALL
	case "processing":
		return proto.StepType_STEP_TYPE_PROCESSING
	default:
		return proto.StepType_STEP_TYPE_UNSPECIFIED
	}
}

// convertToStringMap converts interface{} parameters to string map for proto
func convertToStringMap(params interface{}) map[string]string {
	if params == nil {
		return nil
	}

	// If it's already a string map, return it
	if stringMap, ok := params.(map[string]string); ok {
		return stringMap
	}

	// Try to convert via JSON marshaling/unmarshaling
	jsonBytes, err := json.Marshal(params)
	if err != nil {
		return map[string]string{"error": "failed to marshal parameters"}
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return map[string]string{"error": "failed to unmarshal parameters"}
	}

	// Convert all values to strings
	stringMap := make(map[string]string)
	for k, v := range result {
		if str, ok := v.(string); ok {
			stringMap[k] = str
		} else {
			// Convert non-string values to JSON strings
			if valueBytes, err := json.Marshal(v); err == nil {
				stringMap[k] = string(valueBytes)
			} else {
				stringMap[k] = "conversion_error"
			}
		}
	}

	return stringMap
}

// timestampFromTime converts Go time.Time to protobuf Timestamp
func timestampFromTime(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// timestampNow returns current timestamp in protobuf format
func timestampNow() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// convertToProtoStruct converts interface{} to protobuf Struct
func convertToProtoStruct(data interface{}) (*structpb.Struct, error) {
	if data == nil {
		return nil, nil
	}

	// Convert to JSON and back to map[string]interface{} for structpb compatibility
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		return nil, err
	}

	return structpb.NewStruct(jsonData)
}

// convertStringToProtoStruct converts string to protobuf Struct
func convertStringToProtoStruct(data string) (*structpb.Struct, error) {
	if data == "" {
		return nil, nil
	}

	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal([]byte(data), &jsonData); err == nil {
		// If it's valid JSON, convert it
		if jsonMap, ok := jsonData.(map[string]interface{}); ok {
			return structpb.NewStruct(jsonMap)
		}
	}

	// If not valid JSON, wrap in a simple struct
	return structpb.NewStruct(map[string]interface{}{
		"value": data,
	})
}

// convertTokenUsageToLighthouseProto converts Station token usage to lighthouse proto format
func convertTokenUsageToLighthouseProto(usage *types.TokenUsage) *proto.LighthouseTokenUsage {
	if usage == nil {
		return nil
	}

	return &proto.LighthouseTokenUsage{
		PromptTokens:     int32(usage.PromptTokens),
		CompletionTokens: int32(usage.CompletionTokens),
		TotalTokens:      int32(usage.TotalTokens),
		CostUsd:          usage.CostUSD,
	}
}

// convertRunStatusToLighthouseProto converts Station run status to lighthouse proto format
func convertRunStatusToLighthouseProto(status string) proto.LighthouseRunStatus {
	switch status {
	case "queued":
		return proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_QUEUED
	case "running":
		return proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_RUNNING
	case "completed":
		return proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_COMPLETED
	case "failed":
		return proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_FAILED
	case "cancelled":
		return proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_CANCELLED
	default:
		return proto.LighthouseRunStatus_LIGHTHOUSE_RUN_STATUS_UNSPECIFIED
	}
}
