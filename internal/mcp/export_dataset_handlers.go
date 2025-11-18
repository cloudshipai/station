package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
)

// Dataset Export Handlers
// Exports agent runs and traces to Genkit-compatible format for external evaluation

// DatasetFormat defines the structure of exported evaluation datasets
type DatasetFormat struct {
	ExportedAt  time.Time         `json:"exported_at"`
	FilterModel string            `json:"filter_model,omitempty"`
	RunCount    int               `json:"run_count"`
	Runs        []DatasetRun      `json:"runs"`
	Statistics  DatasetStatistics `json:"statistics"`
}

// DatasetRun represents a single agent run in the dataset
type DatasetRun struct {
	RunID           int64           `json:"run_id"`
	AgentID         int64           `json:"agent_id"`
	AgentName       string          `json:"agent_name"`
	Task            string          `json:"task"`
	Response        string          `json:"response"`
	Status          string          `json:"status"`
	StartedAt       time.Time       `json:"started_at"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	DurationSeconds float64         `json:"duration_seconds"`
	StepsTaken      int64           `json:"steps_taken"`
	ToolCalls       []ToolCall      `json:"tool_calls,omitempty"`
	ExecutionSteps  []ExecutionStep `json:"execution_steps,omitempty"`
	ModelName       string          `json:"model_name,omitempty"`
	InputTokens     *int64          `json:"input_tokens,omitempty"`
	OutputTokens    *int64          `json:"output_tokens,omitempty"`
	TotalTokens     *int64          `json:"total_tokens,omitempty"`
	ToolsUsed       *int64          `json:"tools_used,omitempty"`
	Error           string          `json:"error,omitempty"`
}

// ToolCall represents a single tool invocation
type ToolCall struct {
	ToolName  string                 `json:"tool_name"`
	Input     map[string]interface{} `json:"input,omitempty"`
	Output    interface{}            `json:"output,omitempty"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ExecutionStep represents a step in agent execution
type ExecutionStep struct {
	Step       int                    `json:"step"`
	Type       string                 `json:"type"`
	Content    string                 `json:"content,omitempty"`
	ToolName   string                 `json:"tool_name,omitempty"`
	ToolInput  map[string]interface{} `json:"tool_input,omitempty"`
	ToolOutput interface{}            `json:"tool_output,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
}

// DatasetStatistics provides summary statistics for the exported dataset
type DatasetStatistics struct {
	TotalRuns      int     `json:"total_runs"`
	SuccessfulRuns int     `json:"successful_runs"`
	FailedRuns     int     `json:"failed_runs"`
	AvgDuration    float64 `json:"avg_duration_seconds"`
	AvgStepsTaken  float64 `json:"avg_steps_taken"`
	AvgTokensUsed  float64 `json:"avg_tokens_used"`
	UniqueAgents   int     `json:"unique_agents"`
	UniqueModels   int     `json:"unique_models"`
}

func (s *Server) handleExportDataset(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract optional parameters
	filterModel := request.GetString("filter_model", "")
	filterAgentID := request.GetString("filter_agent_id", "")
	limit := request.GetInt("limit", 100)
	offset := request.GetInt("offset", 0)
	outputDir := request.GetString("output_dir", "")

	// Default output directory to evals/
	if outputDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get working directory: %v", err)), nil
		}
		outputDir = filepath.Join(cwd, "evals")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create output directory: %v", err)), nil
	}

	// Fetch runs based on filters
	var runs []*models.AgentRunWithDetails
	var err error

	if filterModel != "" {
		// Filter by model
		runs, err = s.repos.AgentRuns.ListByModel(ctx, filterModel, int64(limit), int64(offset))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch runs by model: %v", err)), nil
		}
	} else if filterAgentID != "" {
		// Filter by agent
		agentID, parseErr := strconv.ParseInt(filterAgentID, 10, 64)
		if parseErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid agent_id format: %v", parseErr)), nil
		}

		basicRuns, err := s.repos.AgentRuns.ListByAgent(ctx, agentID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch runs by agent: %v", err)), nil
		}

		// Apply pagination manually
		start := offset
		end := offset + limit
		if start > len(basicRuns) {
			start = len(basicRuns)
		}
		if end > len(basicRuns) {
			end = len(basicRuns)
		}
		paginatedRuns := basicRuns[start:end]

		// Convert to detailed format
		runs = make([]*models.AgentRunWithDetails, len(paginatedRuns))
		for i, run := range paginatedRuns {
			agent, _ := s.repos.Agents.GetByID(run.AgentID)
			agentName := "Unknown"
			if agent != nil {
				agentName = agent.Name
			}

			runs[i] = &models.AgentRunWithDetails{
				AgentRun:  *run,
				AgentName: agentName,
				Username:  "Unknown",
			}
		}
	} else {
		// No filter - get recent runs
		runs, err = s.repos.AgentRuns.ListRecent(ctx, int64(limit))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch recent runs: %v", err)), nil
		}

		// Apply offset manually
		start := offset
		if start > len(runs) {
			start = len(runs)
		}
		runs = runs[start:]
	}

	if len(runs) == 0 {
		return mcp.NewToolResultText("No runs found matching the specified filters"), nil
	}

	// Convert to dataset format
	dataset := DatasetFormat{
		ExportedAt:  time.Now(),
		FilterModel: filterModel,
		RunCount:    len(runs),
		Runs:        make([]DatasetRun, len(runs)),
		Statistics:  DatasetStatistics{},
	}

	// Track statistics
	uniqueAgents := make(map[int64]bool)
	uniqueModels := make(map[string]bool)
	var totalDuration, totalSteps, totalTokens float64
	var successCount, failedCount int

	for i, run := range runs {
		// Calculate duration
		var durationSeconds float64
		if run.CompletedAt != nil {
			durationSeconds = run.CompletedAt.Sub(run.StartedAt).Seconds()
		}

		// Parse tool calls and execution steps from JSONArray
		var toolCalls []ToolCall
		var executionSteps []ExecutionStep

		if run.ToolCalls != nil {
			var rawToolCalls []map[string]interface{}
			// JSONArray is []interface{}, marshal it to get JSON bytes
			toolCallsJSON, err := json.Marshal(*run.ToolCalls)
			if err == nil && json.Unmarshal(toolCallsJSON, &rawToolCalls) == nil {
				for _, tc := range rawToolCalls {
					toolCall := ToolCall{
						ToolName: getString(tc, "tool_name"),
						Success:  getBool(tc, "success"),
						Error:    getString(tc, "error"),
					}
					if inputRaw, ok := tc["input"].(map[string]interface{}); ok {
						toolCall.Input = inputRaw
					}
					if output, ok := tc["output"]; ok {
						toolCall.Output = output
					}
					// Try to parse timestamp if available
					if tsStr := getString(tc, "timestamp"); tsStr != "" {
						if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
							toolCall.Timestamp = ts
						}
					}
					toolCalls = append(toolCalls, toolCall)
				}
			}
		}

		if run.ExecutionSteps != nil {
			var rawSteps []map[string]interface{}
			// JSONArray is []interface{}, marshal it to get JSON bytes
			execStepsJSON, err := json.Marshal(*run.ExecutionSteps)
			if err == nil && json.Unmarshal(execStepsJSON, &rawSteps) == nil {
				for _, es := range rawSteps {
					execStep := ExecutionStep{
						Step:     getInt(es, "step"),
						Type:     getString(es, "type"),
						Content:  getString(es, "content"),
						ToolName: getString(es, "tool_name"),
					}
					if inputRaw, ok := es["tool_input"].(map[string]interface{}); ok {
						execStep.ToolInput = inputRaw
					}
					if output, ok := es["tool_output"]; ok {
						execStep.ToolOutput = output
					}
					if tsStr := getString(es, "timestamp"); tsStr != "" {
						if ts, err := time.Parse(time.RFC3339, tsStr); err == nil {
							execStep.Timestamp = ts
						}
					}
					executionSteps = append(executionSteps, execStep)
				}
			}
		}

		// Build dataset run
		datasetRun := DatasetRun{
			RunID:           run.ID,
			AgentID:         run.AgentID,
			AgentName:       run.AgentName,
			Task:            run.Task,
			Response:        run.FinalResponse,
			Status:          run.Status,
			StartedAt:       run.StartedAt,
			CompletedAt:     run.CompletedAt,
			DurationSeconds: durationSeconds,
			StepsTaken:      run.StepsTaken,
			ToolCalls:       toolCalls,
			ExecutionSteps:  executionSteps,
			InputTokens:     run.InputTokens,
			OutputTokens:    run.OutputTokens,
			TotalTokens:     run.TotalTokens,
			ToolsUsed:       run.ToolsUsed,
		}

		// Add model name if available
		if run.ModelName != nil && *run.ModelName != "" {
			datasetRun.ModelName = *run.ModelName
			uniqueModels[*run.ModelName] = true
		}

		// Add error if present
		if run.Error != nil && *run.Error != "" {
			datasetRun.Error = *run.Error
		}

		dataset.Runs[i] = datasetRun

		// Update statistics
		uniqueAgents[run.AgentID] = true
		totalDuration += durationSeconds
		totalSteps += float64(run.StepsTaken)
		if run.TotalTokens != nil {
			totalTokens += float64(*run.TotalTokens)
		}

		if strings.ToLower(run.Status) == "completed" {
			successCount++
		} else if strings.ToLower(run.Status) == "failed" {
			failedCount++
		}
	}

	// Calculate statistics
	runCount := float64(len(runs))
	dataset.Statistics = DatasetStatistics{
		TotalRuns:      len(runs),
		SuccessfulRuns: successCount,
		FailedRuns:     failedCount,
		AvgDuration:    totalDuration / runCount,
		AvgStepsTaken:  totalSteps / runCount,
		AvgTokensUsed:  totalTokens / runCount,
		UniqueAgents:   len(uniqueAgents),
		UniqueModels:   len(uniqueModels),
	}

	// Generate timestamped filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("dataset-%s.json", timestamp)
	if filterModel != "" {
		modelSafe := strings.ReplaceAll(filterModel, "/", "-")
		filename = fmt.Sprintf("dataset-%s-%s.json", modelSafe, timestamp)
	}

	outputPath := filepath.Join(outputDir, filename)

	// Write dataset to file
	datasetJSON, err := json.MarshalIndent(dataset, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to marshal dataset: %v", err)), nil
	}

	if err := os.WriteFile(outputPath, datasetJSON, 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write dataset file: %v", err)), nil
	}

	// Build response
	response := map[string]interface{}{
		"success": true,
		"dataset": map[string]interface{}{
			"output_path":  outputPath,
			"run_count":    len(runs),
			"filter_model": filterModel,
			"filter_agent": filterAgentID,
			"exported_at":  dataset.ExportedAt,
		},
		"statistics": dataset.Statistics,
		"message":    fmt.Sprintf("Exported %d runs to %s", len(runs), outputPath),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// Helper functions to safely extract values from maps
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}

func getInt(m map[string]interface{}, key string) int {
	switch val := m[key].(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}
