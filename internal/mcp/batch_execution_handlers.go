package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"station/internal/services"

	"github.com/mark3labs/mcp-go/mcp"
)

// Batch Execution Handlers
// Handles batch agent execution for testing and evaluation

// BatchExecutionTask represents a single agent execution task in a batch
type BatchExecutionTask struct {
	AgentID   int64                  `json:"agent_id"`
	Task      string                 `json:"task"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// BatchExecutionResult represents the result of a single agent execution in a batch
type BatchExecutionResult struct {
	AgentID   int64     `json:"agent_id"`
	RunID     int64     `json:"run_id"`
	Task      string    `json:"task"`
	Success   bool      `json:"success"`
	Response  string    `json:"response,omitempty"`
	Error     string    `json:"error,omitempty"`
	Duration  float64   `json:"duration_seconds"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

func (s *Server) handleBatchExecuteAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameters
	tasksParam := request.GetString("tasks", "")
	if tasksParam == "" {
		return mcp.NewToolResultError("Missing 'tasks' parameter: must provide JSON array of execution tasks"), nil
	}

	// Parse tasks JSON array
	var tasks []BatchExecutionTask
	if err := json.Unmarshal([]byte(tasksParam), &tasks); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid 'tasks' format: %v. Expected JSON array of {agent_id, task, variables?}", err)), nil
	}

	if len(tasks) == 0 {
		return mcp.NewToolResultError("'tasks' array cannot be empty"), nil
	}

	// Extract optional parameters
	maxConcurrent := request.GetInt("max_concurrent", 5) // Default to 5 concurrent executions
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	if maxConcurrent > 20 {
		maxConcurrent = 20 // Safety limit
	}

	storeRuns := request.GetBool("store_runs", true)
	iterations := request.GetInt("iterations", 1) // Number of times to execute each task
	if iterations < 1 {
		iterations = 1
	}
	if iterations > 100 {
		iterations = 100 // Safety limit
	}

	// Expand tasks by iterations (e.g., 3 tasks x 2 iterations = 6 total executions)
	expandedTasks := make([]BatchExecutionTask, 0, len(tasks)*iterations)
	for i := 0; i < iterations; i++ {
		expandedTasks = append(expandedTasks, tasks...)
	}

	// Execute all tasks concurrently using goroutines with semaphore pattern
	results := make([]BatchExecutionResult, len(expandedTasks))
	semaphore := make(chan struct{}, maxConcurrent) // Limit concurrent executions
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect results slice

	startTime := time.Now()

	for i, task := range expandedTasks {
		wg.Add(1)
		go func(index int, execTask BatchExecutionTask) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // Release slot when done

			// Execute the agent
			execStart := time.Now()
			result := BatchExecutionResult{
				AgentID:   execTask.AgentID,
				Task:      execTask.Task,
				StartTime: execStart,
			}

			var runID int64
			var response *services.Message
			var execErr error

			if storeRuns {
				// Create run first
				run, err := s.repos.AgentRuns.Create(ctx, execTask.AgentID, 1, execTask.Task, "", 0, nil, nil, "running", nil)
				if err != nil {
					result.Success = false
					result.Error = fmt.Sprintf("Failed to create run: %v", err)
					result.EndTime = time.Now()
					result.Duration = time.Since(execStart).Seconds()
					mu.Lock()
					results[index] = result
					mu.Unlock()
					return
				}
				runID = run.ID
				result.RunID = runID

				// Get agent
				agent, err := s.repos.Agents.GetByID(execTask.AgentID)
				if err != nil {
					result.Success = false
					result.Error = fmt.Sprintf("Agent not found: %v", err)
					result.EndTime = time.Now()
					result.Duration = time.Since(execStart).Seconds()
					mu.Lock()
					results[index] = result
					mu.Unlock()
					return
				}

				// Execute using execution engine (same as handleCallAgent)
				agentService := services.NewAgentService(s.repos, s.lighthouseClient)
				engine := agentService.GetExecutionEngine()

				userVariables := execTask.Variables
				if userVariables == nil {
					userVariables = make(map[string]interface{})
				}

				execResult, execErr := engine.Execute(ctx, agent, execTask.Task, runID, userVariables)
				if execErr != nil {
					// Update run as failed
					completedAt := time.Now()
					errorMsg := fmt.Sprintf("Execution failed: %v", execErr)
					s.repos.AgentRuns.UpdateCompletionWithMetadata(
						ctx, runID, errorMsg, 0, nil, nil, "failed", &completedAt,
						nil, nil, nil, nil, nil, nil, &errorMsg,
					)

					result.Success = false
					result.Error = errorMsg
					result.EndTime = completedAt
					result.Duration = time.Since(execStart).Seconds()
					mu.Lock()
					results[index] = result
					mu.Unlock()
					return
				}

				// Update run as completed
				completedAt := time.Now()
				durationSeconds := execResult.Duration.Seconds()

				// Extract token usage (same logic as handleCallAgent)
				var inputTokens, outputTokens, totalTokens *int64
				var toolsUsed *int64
				if execResult.TokenUsage != nil {
					if inputVal := extractInt64FromTokenUsage(execResult.TokenUsage["input_tokens"]); inputVal != nil {
						inputTokens = inputVal
					}
					if outputVal := extractInt64FromTokenUsage(execResult.TokenUsage["output_tokens"]); outputVal != nil {
						outputTokens = outputVal
					}
					if totalVal := extractInt64FromTokenUsage(execResult.TokenUsage["total_tokens"]); totalVal != nil {
						totalTokens = totalVal
					}
				}
				if execResult.StepsUsed > 0 {
					toolsUsedVal := int64(execResult.StepsUsed)
					toolsUsed = &toolsUsedVal
				}

				status := "completed"
				var errorMsg *string
				if !execResult.Success {
					status = "failed"
					if execResult.Error != "" {
						errorMsg = &execResult.Error
					}
				}

				s.repos.AgentRuns.UpdateCompletionWithMetadata(
					ctx, runID, execResult.Response, execResult.StepsTaken,
					execResult.ToolCalls, execResult.ExecutionSteps, status, &completedAt,
					inputTokens, outputTokens, totalTokens, &durationSeconds,
					&execResult.ModelName, toolsUsed, errorMsg,
				)

				result.Success = execResult.Success
				result.Response = execResult.Response
				if !execResult.Success && execResult.Error != "" {
					result.Error = execResult.Error
				}
				result.EndTime = completedAt
				result.Duration = durationSeconds
			} else {
				// Execute without storing run
				userVariables := execTask.Variables
				if userVariables == nil {
					userVariables = make(map[string]interface{})
				}

				response, execErr = s.agentService.ExecuteAgent(ctx, execTask.AgentID, execTask.Task, userVariables)
				result.EndTime = time.Now()
				result.Duration = time.Since(execStart).Seconds()

				if execErr != nil {
					result.Success = false
					result.Error = fmt.Sprintf("Execution failed: %v", execErr)
				} else if response != nil {
					result.Success = true
					result.Response = response.Content
				} else {
					result.Success = false
					result.Error = "Agent returned nil response"
				}
			}

			// Store result
			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, task)
	}

	// Wait for all executions to complete
	wg.Wait()
	totalDuration := time.Since(startTime)

	// Calculate summary statistics
	var successCount, failureCount int
	var totalResponseTime float64
	runIDs := make([]int64, 0)

	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
		totalResponseTime += result.Duration
		if result.RunID > 0 {
			runIDs = append(runIDs, result.RunID)
		}
	}

	avgResponseTime := totalResponseTime / float64(len(results))

	// Build response
	response := map[string]interface{}{
		"success": true,
		"summary": map[string]interface{}{
			"total_executions": len(results),
			"successful":       successCount,
			"failed":           failureCount,
			"iterations":       iterations,
			"max_concurrent":   maxConcurrent,
			"total_duration":   totalDuration.Seconds(),
			"avg_duration":     avgResponseTime,
			"runs_stored":      storeRuns,
		},
		"results": results,
		"run_ids": runIDs,
		"message": fmt.Sprintf("Batch execution completed: %d/%d successful (%.1f%% success rate)",
			successCount, len(results), float64(successCount)/float64(len(results))*100),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}
