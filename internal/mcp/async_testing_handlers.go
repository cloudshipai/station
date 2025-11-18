package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"station/internal/config"
	"station/internal/services"
	"station/pkg/benchmark"
	"station/pkg/types"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
)

// Async Testing Handlers
// Handles generate_and_test_agent - complete automated testing pipeline

var (
	// Track active testing tasks
	activeTasks      = make(map[string]*types.TestingProgress)
	activeTasksMutex sync.RWMutex
)

func (s *Server) handleGenerateAndTestAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	agentIDStr, err := request.RequireString("agent_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'agent_id' parameter: %v", err)), nil
	}

	agentID, err := strconv.ParseInt(agentIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError("Invalid agent_id format"), nil
	}

	// Extract optional parameters
	scenarioCount := request.GetInt("scenario_count", 100)
	maxConcurrent := request.GetInt("max_concurrent", 10)
	variationStrategy := request.GetString("variation_strategy", "comprehensive")
	jaegerURL := request.GetString("jaeger_url", "http://localhost:16686")

	// Always use workspace directory under datasets/
	// Get agent to determine environment name
	agentData, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	env, err := s.repos.Environments.GetByID(agentData.EnvironmentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get environment: %v", err)), nil
	}

	// Create datasets directory in environment workspace using server config
	// Use s.config.Workspace directly instead of viper to ensure correct path
	workspacePath := s.config.Workspace
	if workspacePath == "" {
		return mcp.NewToolResultError("Workspace path not configured"), nil
	}
	environmentDir := filepath.Join(workspacePath, "environments", env.Name)
	datasetsDir := filepath.Join(environmentDir, "datasets")

	// Create timestamped output directory
	timestamp := time.Now().Format("20060102-150405")
	outputDir := filepath.Join(datasetsDir, fmt.Sprintf("agent-%d-%s", agentID, timestamp))

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create output directory: %v", err)), nil
	}

	// Generate task ID
	taskID := fmt.Sprintf("test-%s", uuid.New().String()[:8])

	// Get agent details for response
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Agent not found: %v", err)), nil
	}

	// Initialize progress tracking
	progress := &types.TestingProgress{
		TaskID:    taskID,
		AgentID:   agentID,
		AgentName: agent.Name,
		Status:    "initializing",
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
		Phases: map[string]types.PhaseStatus{
			"introspection":       {Status: "pending"},
			"scenario_generation": {Status: "pending"},
			"batch_execution":     {Status: "pending"},
			"trace_collection":    {Status: "pending"},
			"analysis":            {Status: "pending"},
			"export":              {Status: "pending"},
		},
		OutputFiles: types.OutputFiles{
			Progress: filepath.Join(outputDir, "progress.json"),
		},
	}

	// Store progress
	activeTasksMutex.Lock()
	activeTasks[taskID] = progress
	activeTasksMutex.Unlock()

	// Build config
	config := types.TestingConfig{
		AgentID:           agentID,
		ScenarioCount:     scenarioCount,
		MaxConcurrent:     maxConcurrent,
		VariationStrategy: variationStrategy,
		OutputDir:         outputDir,
		JaegerURL:         jaegerURL,
	}

	// Start async execution
	go s.executeTestingPipelineAsync(taskID, config)

	// Return immediate response
	response := map[string]interface{}{
		"success": true,
		"task_id": taskID,
		"status":  "running",
		"agent": map[string]interface{}{
			"id":   agentID,
			"name": agent.Name,
		},
		"config": map[string]interface{}{
			"scenario_count":     scenarioCount,
			"max_concurrent":     maxConcurrent,
			"variation_strategy": variationStrategy,
			"output_dir":         outputDir,
		},
		"progress_file":      progress.OutputFiles.Progress,
		"estimated_duration": "5-10 minutes",
		"message":            fmt.Sprintf("Test generation and execution started for agent '%s'. Monitor progress at %s", agent.Name, progress.OutputFiles.Progress),
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) executeTestingPipelineAsync(taskID string, config types.TestingConfig) {
	// Get progress tracker
	activeTasksMutex.RLock()
	progress, exists := activeTasks[taskID]
	activeTasksMutex.RUnlock()

	if !exists {
		return
	}

	// Defer cleanup and final status update
	defer func() {
		if r := recover(); r != nil {
			updateProgress(progress, "failed", fmt.Sprintf("Pipeline panic: %v", r))
		}
		saveProgress(progress)
	}()

	ctx := context.Background()

	// Phase 1: Introspect Agent
	agentCtx, err := s.introspectAgent(ctx, config.AgentID, progress)
	if err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Introspection failed: %v", err))
		return
	}

	// Phase 2: Generate Scenarios
	scenarios, err := s.generateTestScenarios(ctx, agentCtx, config, progress)
	if err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Scenario generation failed: %v", err))
		return
	}

	// Save scenarios
	scenariosPath := filepath.Join(config.OutputDir, "scenarios.json")
	if err := saveJSON(scenariosPath, scenarios); err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Failed to save scenarios: %v", err))
		return
	}
	progress.OutputFiles.Scenarios = scenariosPath

	// Phase 3: Batch Execute
	runResults, err := s.executeBatchWithFullTracing(ctx, config.AgentID, scenarios, config.MaxConcurrent, progress)
	if err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Batch execution failed: %v", err))
		return
	}

	// Phase 4: Collect Traces from Jaeger
	traces, err := s.collectJaegerTraces(ctx, runResults, config.JaegerURL, progress)
	if err != nil {
		// Non-fatal - continue without traces
		fmt.Printf("Warning: Failed to collect Jaeger traces: %v\n", err)
	}

	// Phase 5: Build Comprehensive Dataset
	dataset, err := s.buildComprehensiveDataset(ctx, agentCtx, runResults, traces, config, progress)
	if err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Dataset building failed: %v", err))
		return
	}

	// Phase 6: Analyze Dataset (Statistical)
	analysis, err := s.analyzeDataset(dataset, progress)
	if err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Analysis failed: %v", err))
		return
	}
	dataset.Analysis = analysis

	// Phase 7: LLM-as-Judge Evaluation
	llmEvaluation, err := s.evaluateDatasetWithLLM(ctx, dataset, &config, progress)
	if err != nil {
		// Non-fatal - continue without LLM evaluation
		fmt.Printf("Warning: LLM evaluation failed: %v\n", err)
	}

	// Phase 8: Export Everything
	if err := s.exportTestingResults(dataset, llmEvaluation, config.OutputDir, progress); err != nil {
		updateProgress(progress, "failed", fmt.Sprintf("Export failed: %v", err))
		return
	}

	// Mark as completed
	now := time.Now()
	progress.Status = "completed"
	progress.UpdatedAt = now
	progress.CompletedAt = &now
	saveProgress(progress)
}

func (s *Server) introspectAgent(ctx context.Context, agentID int64, progress *types.TestingProgress) (*types.AgentContext, error) {
	startPhase(progress, "introspection")

	// Get agent details
	agent, err := s.repos.Agents.GetByID(agentID)
	if err != nil {
		failPhase(progress, "introspection", fmt.Sprintf("Failed to get agent: %v", err))
		return nil, err
	}

	// Get environment
	env, err := s.repos.Environments.GetByID(agent.EnvironmentID)
	if err != nil {
		failPhase(progress, "introspection", fmt.Sprintf("Failed to get environment: %v", err))
		return nil, err
	}

	// Get assigned tools
	agentTools, err := s.repos.AgentTools.ListAgentTools(agentID)
	if err != nil {
		failPhase(progress, "introspection", fmt.Sprintf("Failed to get tools: %v", err))
		return nil, err
	}

	toolNames := make([]string, len(agentTools))
	for i, tool := range agentTools {
		toolNames[i] = tool.ToolName
	}

	// Parse input schema if available
	var inputSchema map[string]interface{}
	if agent.InputSchema != nil && *agent.InputSchema != "" {
		if err := json.Unmarshal([]byte(*agent.InputSchema), &inputSchema); err != nil {
			// Non-fatal, continue without schema
			inputSchema = nil
		}
	}

	agentCtx := &types.AgentContext{
		ID:              agent.ID,
		Name:            agent.Name,
		Description:     agent.Description,
		Prompt:          agent.Prompt,
		MaxSteps:        agent.MaxSteps,
		InputSchema:     inputSchema,
		Tools:           toolNames,
		EnvironmentID:   agent.EnvironmentID,
		EnvironmentName: env.Name,
	}

	completePhase(progress, "introspection", map[string]interface{}{
		"agent_name":  agent.Name,
		"tools_count": len(toolNames),
		"has_schema":  inputSchema != nil,
		"environment": env.Name,
	})

	return agentCtx, nil
}

func (s *Server) generateTestScenarios(ctx context.Context, agentCtx *types.AgentContext, config types.TestingConfig, progress *types.TestingProgress) ([]types.TestScenario, error) {
	startPhase(progress, "scenario_generation")

	// Build AI prompt for scenario generation
	prompt := buildScenarioGenerationPrompt(agentCtx, config.ScenarioCount, config.VariationStrategy)

	// Initialize GenKit provider to get AI capabilities
	genkitProvider := services.NewGenKitProvider()
	genkitApp, err := genkitProvider.GetApp(ctx)
	if err != nil {
		// Fallback to simple scenarios if GenKit initialization fails
		scenarios := s.generateFallbackScenarios(agentCtx, config.ScenarioCount, "AI initialization failed")
		completePhase(progress, "scenario_generation", map[string]interface{}{
			"scenarios_generated": len(scenarios),
			"strategy":            config.VariationStrategy,
			"warning":             "Used fallback scenarios due to AI initialization failure",
		})
		return scenarios, nil
	}

	// Use GenKit to generate realistic user scenarios
	response, err := s.generateScenariosWithAI(ctx, genkitApp, prompt)
	if err != nil {
		// Fallback to simple scenarios if AI generation fails
		scenarios := s.generateFallbackScenarios(agentCtx, config.ScenarioCount, "AI generation failed")
		completePhase(progress, "scenario_generation", map[string]interface{}{
			"scenarios_generated": len(scenarios),
			"strategy":            config.VariationStrategy,
			"warning":             "Used fallback scenarios due to AI generation failure",
		})
		return scenarios, nil
	}

	// Parse AI response to extract scenarios
	scenarios, err := parseAIGeneratedScenarios(response, config.ScenarioCount)
	if err != nil || len(scenarios) == 0 {
		// Fallback if parsing fails
		scenarios = s.generateFallbackScenarios(agentCtx, config.ScenarioCount, "parsing failed")
		completePhase(progress, "scenario_generation", map[string]interface{}{
			"scenarios_generated": len(scenarios),
			"strategy":            config.VariationStrategy,
			"warning":             "Used fallback scenarios due to parsing failure",
		})
		return scenarios, nil
	}

	completePhase(progress, "scenario_generation", map[string]interface{}{
		"scenarios_generated": len(scenarios),
		"strategy":            config.VariationStrategy,
	})

	return scenarios, nil
}

func (s *Server) executeBatchWithFullTracing(ctx context.Context, agentID int64, scenarios []types.TestScenario, maxConcurrent int, progress *types.TestingProgress) ([]BatchExecutionResult, error) {
	startPhase(progress, "batch_execution")

	// Setup debug logging to file
	debugLog, err := os.OpenFile("/tmp/station-batch-execution-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		defer debugLog.Close()
		fmt.Fprintf(debugLog, "\n\n=== NEW BATCH EXECUTION ===\n")
		fmt.Fprintf(debugLog, "Time: %s\n", time.Now().Format(time.RFC3339))
		fmt.Fprintf(debugLog, "Agent ID: %d\n", agentID)
		fmt.Fprintf(debugLog, "Scenarios: %d\n", len(scenarios))
		fmt.Fprintf(debugLog, "Max Concurrent: %d\n", maxConcurrent)
	}

	// Convert scenarios to batch tasks
	tasks := make([]BatchExecutionTask, len(scenarios))
	for i, scenario := range scenarios {
		tasks[i] = BatchExecutionTask{
			AgentID:   agentID,
			Task:      scenario.Task,
			Variables: scenario.Variables,
		}
		if debugLog != nil {
			fmt.Fprintf(debugLog, "Task %d: %s\n", i+1, scenario.Task)
		}
	}

	// Execute batch (reuse existing batch execution logic)
	results := make([]BatchExecutionResult, len(tasks))
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex      // For results array
	var dbMutex sync.Mutex // For SQLite write serialization

	for i, task := range tasks {
		wg.Add(1)
		if debugLog != nil {
			fmt.Fprintf(debugLog, "Launching goroutine %d/%d for task: %s\n", i+1, len(tasks), task.Task)
		}
		go func(index int, execTask BatchExecutionTask) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			if debugLog != nil {
				fmt.Fprintf(debugLog, "[Goroutine %d] Started execution\n", index+1)
			}

			execStart := time.Now()
			result := BatchExecutionResult{
				AgentID:   execTask.AgentID,
				Task:      execTask.Task,
				StartTime: execStart,
			}

			// Create run (serialize DB writes to avoid SQLite locking)
			dbMutex.Lock()
			run, err := s.repos.AgentRuns.Create(ctx, execTask.AgentID, 1, execTask.Task, "", 0, nil, nil, "running", nil)
			dbMutex.Unlock()

			if err != nil {
				if debugLog != nil {
					fmt.Fprintf(debugLog, "[Goroutine %d] ERROR creating run: %v\n", index+1, err)
				}
				result.Success = false
				result.Error = fmt.Sprintf("Failed to create run: %v", err)
				result.EndTime = time.Now()
				result.Duration = time.Since(execStart).Seconds()
				mu.Lock()
				results[index] = result
				mu.Unlock()
				return
			}
			result.RunID = run.ID
			if debugLog != nil {
				fmt.Fprintf(debugLog, "[Goroutine %d] Created run ID: %d\n", index+1, run.ID)
			}

			// Get agent
			agent, err := s.repos.Agents.GetByID(execTask.AgentID)
			if err != nil {
				if debugLog != nil {
					fmt.Fprintf(debugLog, "[Goroutine %d] ERROR getting agent: %v\n", index+1, err)
				}
				result.Success = false
				result.Error = fmt.Sprintf("Agent not found: %v", err)
				result.EndTime = time.Now()
				result.Duration = time.Since(execStart).Seconds()
				mu.Lock()
				results[index] = result
				mu.Unlock()
				return
			}

			if debugLog != nil {
				fmt.Fprintf(debugLog, "[Goroutine %d] Starting execution for agent %s (runID=%d)\n", index+1, agent.Name, run.ID)
			}

			// Execute
			agentService := services.NewAgentService(s.repos, s.lighthouseClient)
			engine := agentService.GetExecutionEngine()

			userVariables := execTask.Variables
			if userVariables == nil {
				userVariables = make(map[string]interface{})
			}

			execResult, execErr := engine.Execute(ctx, agent, execTask.Task, run.ID, userVariables)
			if execErr != nil {
				if debugLog != nil {
					fmt.Fprintf(debugLog, "[Goroutine %d] EXECUTION ERROR: %v\n", index+1, execErr)
				}
				completedAt := time.Now()
				errorMsg := fmt.Sprintf("Execution failed: %v", execErr)
				dbMutex.Lock()
				s.repos.AgentRuns.UpdateCompletionWithMetadata(
					ctx, run.ID, errorMsg, 0, nil, nil, "failed", &completedAt,
					nil, nil, nil, nil, nil, nil, &errorMsg,
				)
				dbMutex.Unlock()

				result.Success = false
				result.Error = errorMsg
				result.EndTime = completedAt
				result.Duration = time.Since(execStart).Seconds()
				mu.Lock()
				results[index] = result
				mu.Unlock()
				return
			}

			if debugLog != nil {
				fmt.Fprintf(debugLog, "[Goroutine %d] Execution completed successfully. Steps: %d, Duration: %.2fs\n", index+1, execResult.StepsTaken, execResult.Duration.Seconds())
			}

			// Update run as completed
			completedAt := time.Now()
			durationSeconds := execResult.Duration.Seconds()

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

			dbMutex.Lock()
			s.repos.AgentRuns.UpdateCompletionWithMetadata(
				ctx, run.ID, execResult.Response, execResult.StepsTaken,
				execResult.ToolCalls, execResult.ExecutionSteps, status, &completedAt,
				inputTokens, outputTokens, totalTokens, &durationSeconds,
				&execResult.ModelName, toolsUsed, errorMsg,
			)
			dbMutex.Unlock()

			result.Success = execResult.Success
			result.Response = execResult.Response
			if !execResult.Success && execResult.Error != "" {
				result.Error = execResult.Error
			}
			result.EndTime = completedAt
			result.Duration = durationSeconds

			mu.Lock()
			results[index] = result

			// Update progress
			completed := 0
			for _, r := range results {
				if r.EndTime.After(time.Time{}) {
					completed++
				}
			}
			updatePhaseProgress(progress, "batch_execution", completed, len(results))
			mu.Unlock()
		}(i, task)
	}

	wg.Wait()

	if debugLog != nil {
		fmt.Fprintf(debugLog, "All goroutines completed. Counting results...\n")
	}

	// Count successes
	successCount := 0
	for i, result := range results {
		if result.Success {
			successCount++
		}
		if debugLog != nil {
			fmt.Fprintf(debugLog, "Result %d: Success=%v, RunID=%d, Error=%s\n", i+1, result.Success, result.RunID, result.Error)
		}
	}

	if debugLog != nil {
		fmt.Fprintf(debugLog, "SUMMARY: Total=%d, Successful=%d, Failed=%d\n", len(results), successCount, len(results)-successCount)
	}

	completePhase(progress, "batch_execution", map[string]interface{}{
		"total_runs":   len(results),
		"successful":   successCount,
		"failed":       len(results) - successCount,
		"success_rate": float64(successCount) / float64(len(results)) * 100,
	})

	return results, nil
}

func (s *Server) collectJaegerTraces(ctx context.Context, results []BatchExecutionResult, jaegerURL string, progress *types.TestingProgress) (map[int64]*types.CompleteTrace, error) {
	startPhase(progress, "trace_collection")

	jaegerClient := services.NewJaegerClient(jaegerURL)
	if !jaegerClient.IsAvailable() {
		failPhase(progress, "trace_collection", "Jaeger not available")
		return nil, fmt.Errorf("Jaeger not available at %s", jaegerURL)
	}

	traces := make(map[int64]*types.CompleteTrace)
	var mu sync.Mutex

	collected := 0
	for _, result := range results {
		if result.RunID == 0 {
			continue
		}

		// Query Jaeger for this run's trace
		jaegerTrace, err := jaegerClient.QueryRunTrace(result.RunID, "station")
		if err != nil {
			// Non-fatal, continue
			continue
		}

		// Build span tree
		tree := services.BuildSpanTree(jaegerTrace.Spans)
		if tree == nil {
			continue
		}

		// Extract tool sequence and timing
		toolSequence := services.ExtractToolSequence(tree)
		timingBreakdown := services.CalculateTimingBreakdown(tree)

		// Convert to our types
		convertedToolSeq := make([]types.ToolCallTrace, len(toolSequence))
		for i, tc := range toolSequence {
			convertedToolSeq[i] = types.ToolCallTrace{
				Step:       tc.Step,
				Tool:       tc.Tool,
				SpanID:     tc.SpanID,
				StartTime:  tc.StartTime,
				DurationMs: tc.DurationMs,
				Success:    tc.Success,
				Input:      tc.Input,
				Output:     tc.Output,
				Error:      tc.Error,
			}
		}

		trace := &types.CompleteTrace{
			TraceID:          jaegerTrace.TraceID,
			TotalSpans:       len(jaegerTrace.Spans),
			ToolCallSequence: convertedToolSeq,
			TimingBreakdown: &types.TimingBreakdown{
				TotalMs:     timingBreakdown.TotalMs,
				SetupMs:     timingBreakdown.SetupMs,
				ExecutionMs: timingBreakdown.ExecutionMs,
				ToolsMs:     timingBreakdown.ToolsMs,
				ReasoningMs: timingBreakdown.ReasoningMs,
				CleanupMs:   timingBreakdown.CleanupMs,
			},
		}

		mu.Lock()
		traces[result.RunID] = trace
		collected++
		updatePhaseProgress(progress, "trace_collection", collected, len(results))
		mu.Unlock()
	}

	completePhase(progress, "trace_collection", map[string]interface{}{
		"traces_collected": len(traces),
		"total_runs":       len(results),
	})

	return traces, nil
}

func (s *Server) buildComprehensiveDataset(ctx context.Context, agentCtx *types.AgentContext, results []BatchExecutionResult, traces map[int64]*types.CompleteTrace, config types.TestingConfig, progress *types.TestingProgress) (*types.ComprehensiveDataset, error) {
	// Fetch full run details
	enrichedRuns := make([]types.EnrichedRun, 0, len(results))

	for _, result := range results {
		if result.RunID == 0 {
			continue
		}

		run, err := s.repos.AgentRuns.GetByIDWithDetails(ctx, result.RunID)
		if err != nil {
			continue
		}

		enriched := types.EnrichedRun{
			RunID:        run.ID,
			AgentID:      run.AgentID,
			AgentName:    agentCtx.Name,
			Task:         run.Task,
			Response:     run.FinalResponse,
			Status:       run.Status,
			Success:      run.Status == "completed",
			StartedAt:    run.StartedAt,
			CompletedAt:  run.CompletedAt,
			StepsTaken:   run.StepsTaken,
			InputTokens:  run.InputTokens,
			OutputTokens: run.OutputTokens,
			TotalTokens:  run.TotalTokens,
			ToolsUsed:    run.ToolsUsed,
		}

		if run.CompletedAt != nil {
			enriched.DurationSeconds = run.CompletedAt.Sub(run.StartedAt).Seconds()
		}

		if run.ModelName != nil {
			enriched.ModelName = *run.ModelName
		}

		if run.Error != nil {
			enriched.Error = *run.Error
		}

		// Add tool calls and execution steps from database
		if run.ToolCalls != nil {
			enriched.ToolCalls = run.ToolCalls
		}

		if run.ExecutionSteps != nil {
			enriched.ExecutionSteps = run.ExecutionSteps
		}

		if run.DebugLogs != nil {
			enriched.DebugLogs = run.DebugLogs
		}

		// Attach trace if available
		if trace, ok := traces[result.RunID]; ok {
			enriched.Trace = trace
		}

		enrichedRuns = append(enrichedRuns, enriched)
	}

	dataset := &types.ComprehensiveDataset{
		Metadata: types.DatasetMetadata{
			AgentID:         agentCtx.ID,
			AgentName:       agentCtx.Name,
			GeneratedAt:     time.Now(),
			TotalRuns:       len(enrichedRuns),
			ScenarioCount:   config.ScenarioCount,
			JaegerAvailable: len(traces) > 0,
			TracesCaptured:  len(traces),
		},
		Runs: enrichedRuns,
	}

	return dataset, nil
}

func (s *Server) analyzeDataset(dataset *types.ComprehensiveDataset, progress *types.TestingProgress) (*types.ComprehensiveAnalysis, error) {
	startPhase(progress, "analysis")

	// TODO: Implement comprehensive statistical analysis
	// For now, return basic placeholder
	analysis := &types.ComprehensiveAnalysis{
		Performance: &types.PerformanceMetrics{},
		ToolUsage: &types.ToolUsageMetrics{
			ToolFrequency:   make(map[string]int),
			ToolPerformance: make(map[string]types.ToolPerfMetrics),
		},
		Quality:  &types.QualityMetrics{},
		Patterns: &types.BehaviorPatterns{},
	}

	completePhase(progress, "analysis", map[string]interface{}{
		"runs_analyzed": len(dataset.Runs),
	})

	return analysis, nil
}

func (s *Server) evaluateDatasetWithLLM(ctx context.Context, dataset *types.ComprehensiveDataset, config *types.TestingConfig, progress *types.TestingProgress) (*benchmark.DatasetEvaluationResult, error) {
	startPhase(progress, "llm_evaluation")

	// Check if benchmark service is available
	if s.benchmarkService == nil {
		failPhase(progress, "llm_evaluation", "Benchmark service not available")
		return nil, fmt.Errorf("benchmark service not available")
	}

	// Convert dataset to benchmark input format
	input := &benchmark.DatasetEvaluationInput{
		DatasetID:   filepath.Base(config.OutputDir),
		AgentID:     config.AgentID,
		AgentName:   dataset.Metadata.AgentName,
		Runs:        make([]benchmark.DatasetRun, len(dataset.Runs)),
		GeneratedAt: dataset.Metadata.GeneratedAt,
	}

	// Convert runs
	for i, run := range dataset.Runs {
		// Convert tool calls to the right format
		var toolCalls []map[string]interface{}
		if run.ToolCalls != nil {
			if tc, ok := run.ToolCalls.([]map[string]interface{}); ok {
				toolCalls = tc
			} else if tcSlice, ok := run.ToolCalls.([]interface{}); ok {
				// Convert []interface{} to []map[string]interface{}
				for _, item := range tcSlice {
					if tcMap, ok := item.(map[string]interface{}); ok {
						toolCalls = append(toolCalls, tcMap)
					}
				}
			}
		}

		input.Runs[i] = benchmark.DatasetRun{
			RunID:           run.RunID,
			Task:            run.Task,
			Response:        run.Response,
			Status:          run.Status,
			Success:         run.Success,
			DurationSeconds: run.DurationSeconds,
			StepsTaken:      run.StepsTaken,
			TotalTokens:     run.TotalTokens,
			ToolCalls:       toolCalls,
		}
	}

	// Perform LLM evaluation
	result, err := s.benchmarkService.EvaluateDataset(ctx, input)
	if err != nil {
		failPhase(progress, "llm_evaluation", fmt.Sprintf("LLM evaluation failed: %v", err))
		return nil, err
	}

	completePhase(progress, "llm_evaluation", map[string]interface{}{
		"runs_evaluated":   result.RunsEvaluated,
		"overall_score":    result.OverallScore,
		"production_ready": result.ProductionReady,
		"evaluation_cost":  result.TotalJudgeCost,
	})

	return result, nil
}

func (s *Server) exportTestingResults(dataset *types.ComprehensiveDataset, llmEval *benchmark.DatasetEvaluationResult, outputDir string, progress *types.TestingProgress) error {
	startPhase(progress, "export")

	// Export dataset
	datasetPath := filepath.Join(outputDir, "dataset.json")
	if err := saveJSON(datasetPath, dataset); err != nil {
		failPhase(progress, "export", fmt.Sprintf("Failed to save dataset: %v", err))
		return err
	}
	progress.OutputFiles.Dataset = datasetPath

	// Export analysis
	if dataset.Analysis != nil {
		analysisPath := filepath.Join(outputDir, "analysis.json")
		if err := saveJSON(analysisPath, dataset.Analysis); err != nil {
			failPhase(progress, "export", fmt.Sprintf("Failed to save analysis: %v", err))
			return err
		}
		progress.OutputFiles.Analysis = analysisPath
	}

	// Export LLM evaluation results
	if llmEval != nil {
		llmEvalPath := filepath.Join(outputDir, "llm_evaluation.json")
		if err := saveJSON(llmEvalPath, llmEval); err != nil {
			failPhase(progress, "export", fmt.Sprintf("Failed to save LLM evaluation: %v", err))
			return err
		}
	}

	// Generate markdown report
	reportPath := filepath.Join(outputDir, "REPORT.md")
	report := generateMarkdownReport(dataset)
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		failPhase(progress, "export", fmt.Sprintf("Failed to save report: %v", err))
		return err
	}
	progress.OutputFiles.Report = reportPath

	completePhase(progress, "export", map[string]interface{}{
		"dataset_path":  datasetPath,
		"analysis_path": progress.OutputFiles.Analysis,
		"report_path":   reportPath,
	})

	return nil
}

// Helper functions

// generateFallbackScenarios creates simple fallback scenarios when AI generation fails
func (s *Server) generateFallbackScenarios(agentCtx *types.AgentContext, count int, reason string) []types.TestScenario {
	scenarios := make([]types.TestScenario, count)
	for i := 0; i < count; i++ {
		scenarios[i] = types.TestScenario{
			Task:         fmt.Sprintf("Test scenario %d for %s", i+1, agentCtx.Name),
			Variables:    make(map[string]interface{}),
			ScenarioType: "fallback",
			Description:  fmt.Sprintf("Fallback test case %d (%s)", i+1, reason),
		}
	}
	return scenarios
}

// generateScenariosWithAI uses GenKit to generate realistic test scenarios
func (s *Server) generateScenariosWithAI(ctx context.Context, genkitApp *genkit.Genkit, prompt string) (string, error) {
	// Get model name from config
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("failed to load config: %w", err)
	}

	modelName := fmt.Sprintf("%s/%s", cfg.AIProvider, cfg.AIModel)

	// Use genkit.Generate with ai.WithPrompt
	response, err := genkit.Generate(ctx, genkitApp,
		ai.WithPrompt(prompt),
		ai.WithModelName(modelName))
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	if response == nil || response.Text() == "" {
		return "", fmt.Errorf("AI returned empty response")
	}

	return response.Text(), nil
}

func buildDomainContext(agentCtx *types.AgentContext) string {
	// Detect domain based on agent name and description
	agentInfo := strings.ToLower(agentCtx.Name + " " + agentCtx.Description)

	// SRE/Incident Response domain
	if strings.Contains(agentInfo, "incident") || strings.Contains(agentInfo, "coordinator") {
		return `**Domain Context - SRE Incident Response:**
Your scenarios should be realistic production incidents that require investigation and resolution.

**Good Examples:**
- "PRODUCTION INCIDENT: The payments-api service started returning HTTP 503 errors at 14:30 UTC. Users cannot complete checkout. Approximately 80% of payment requests are failing. This is a SEV1 incident in production environment."
- "SEV2 ALERT: Database connection pool exhausted on user-service. Response times spiked from 200ms to 5s starting at 09:15 UTC. 30% of users affected."
- "The checkout-worker pod keeps crashing with OOMKilled errors. It restarted 15 times in the last hour. Started after yesterday's deployment at 16:00 UTC."

**Bad Examples (meta-questions, not real incidents):**
- "How many open incidents do we have?"
- "What are the details of the incident from last week?"
- "Show me the incident dashboard"

Each scenario should include: service name, symptoms, error types, time started, severity level, user impact percentage.`
	}

	// Logs analysis domain
	if strings.Contains(agentInfo, "logs") || strings.Contains(agentInfo, "log investigator") {
		return `**Domain Context - Log Analysis:**
Your scenarios should be specific log investigation requests for troubleshooting production issues.

**Good Examples:**
- "Search application logs for NullPointerException errors in the payment-service between 14:00-15:00 UTC today and show me the stack traces."
- "Find all ERROR level logs from the api-gateway in the last 2 hours that contain 'timeout' or 'connection refused'."
- "Analyze logs from the order-processing service and identify any new error patterns that appeared in the last 30 minutes."

Include: service names, time ranges, log levels, specific error messages, stack traces to search for.`
	}

	// Metrics analysis domain
	if strings.Contains(agentInfo, "metrics") || strings.Contains(agentInfo, "performance") {
		return `**Domain Context - Metrics Analysis:**
Your scenarios should be specific metric investigation requests for performance/capacity issues.

**Good Examples:**
- "Check CPU and memory usage for payment-api pods in the last hour. Did anything spike above 80%?"
- "Show me HTTP error rate for checkout-service from 14:00-15:00 UTC and compare it to the previous week's baseline."
- "Analyze database query latency for user-service. Are there any queries taking longer than 1 second in the past 30 minutes?"

Include: metric names, service names, time ranges, thresholds to check, baseline comparisons.`
	}

	// Traces analysis domain
	if strings.Contains(agentInfo, "trace") || strings.Contains(agentInfo, "apm") {
		return `**Domain Context - Distributed Tracing:**
Your scenarios should be specific trace investigation requests for latency/error debugging.

**Good Examples:**
- "Find slow traces for the checkout API endpoint in the last hour where total duration exceeded 2 seconds. Which downstream service is the bottleneck?"
- "Analyze failed traces for payment-service where HTTP status is 5xx. What's the common failure point in the call chain?"
- "Show me traces for order-processing service that include database calls taking longer than 500ms."

Include: service names, endpoints, latency thresholds, error statuses, time ranges.`
	}

	// Infrastructure domain
	if strings.Contains(agentInfo, "infrastructure") || strings.Contains(agentInfo, "infra") || strings.Contains(agentInfo, "kubernetes") {
		return `**Domain Context - Infrastructure:**
Your scenarios should be specific infrastructure investigation requests.

**Good Examples:**
- "Check Kubernetes pod health for the payments namespace. Are any pods in CrashLoopBackOff or Pending state?"
- "Investigate why ALB target health checks are failing for api-server. Show me target group health status."
- "RDS instance payment-db is showing high CPU usage. Check for long-running queries and connection count."

Include: infrastructure components (K8s, AWS, databases), specific resources, health states, error conditions.`
	}

	// Change detection domain
	if strings.Contains(agentInfo, "change") || strings.Contains(agentInfo, "deployment") {
		return `**Domain Context - Change Detection:**
Your scenarios should be requests to correlate incidents with recent changes.

**Good Examples:**
- "Payment errors started at 14:30 UTC. Find any deployments, config changes, or feature flag rollouts in the 30 minutes before that time."
- "Check if any feature flags were changed in the last hour that could affect the checkout flow."
- "What code changes were deployed to user-service between 10:00-11:00 UTC today?"

Include: incident start times, affected services, time ranges to check for changes.`
	}

	// Default for other agents
	return `**Domain Context:**
Create realistic, actionable scenarios based on the agent's description and available tools. Focus on specific tasks the agent would handle in real business situations, not meta-questions about the system itself.`
}

func buildScenarioGenerationPrompt(agentCtx *types.AgentContext, count int, strategy string) string {
	toolsList := ""
	if len(agentCtx.Tools) > 0 {
		toolsList = "\n\nAvailable Tools:\n"
		for _, toolName := range agentCtx.Tools {
			toolsList += fmt.Sprintf("- %s\n", toolName)
		}
	}

	strategyGuidance := ""
	switch strategy {
	case "edge_cases":
		strategyGuidance = "Focus on edge cases, error conditions, and boundary scenarios that test the agent's robustness."
	case "common":
		strategyGuidance = "Focus on common, typical use cases that users would frequently encounter."
	default: // comprehensive
		strategyGuidance = "Include a comprehensive mix of common use cases, edge cases, complex scenarios, and error conditions."
	}

	// Add domain-specific context based on agent name/description
	domainContext := buildDomainContext(agentCtx)

	return fmt.Sprintf(`You are a QA engineer creating realistic test scenarios for an AI agent. Generate %d diverse, realistic user tasks that this agent would handle in real-world business situations.

**Agent Information:**
- Name: %s
- Description: %s%s

**Testing Strategy:** %s

%s

**CRITICAL Instructions:**
1. Generate %d realistic user tasks/scenarios that would occur in real production/business environments
2. Each scenario MUST be:
   - A concrete business situation or problem (NOT meta-questions about the system)
   - Written from the user's perspective (what they would report/request)
   - Specific with actual details (service names, error codes, timestamps, metrics)
   - Actionable and require the agent to USE ITS TOOLS
   - Varied in complexity, severity, and focus areas
3. DO NOT generate meta-questions like:
   - "How many X do we have?" 
   - "What are the details of Y?"
   - "Show me information about Z"
   These are system queries, not real business scenarios!
4. Return ONLY a JSON array of scenario objects in this exact format:
   [
     {
       "task": "The exact user task/scenario with specific details",
       "description": "Brief description of what business situation this tests",
       "scenario_type": "common|edge_case|complex|error_handling"
     }
   ]

Generate %d scenarios now. Return ONLY the JSON array, no other text.`,
		count, agentCtx.Name, agentCtx.Description, toolsList, strategyGuidance, domainContext, count, count)
}

// parseAIGeneratedScenarios extracts test scenarios from AI-generated JSON response
func parseAIGeneratedScenarios(response string, expectedCount int) ([]types.TestScenario, error) {
	// Try to extract JSON array from response (AI might include extra text)
	startIdx := strings.Index(response, "[")
	endIdx := strings.LastIndex(response, "]")

	if startIdx == -1 || endIdx == -1 || startIdx >= endIdx {
		return nil, fmt.Errorf("no valid JSON array found in response")
	}

	jsonStr := response[startIdx : endIdx+1]

	// Parse JSON into intermediate structure
	var rawScenarios []struct {
		Task         string `json:"task"`
		Description  string `json:"description"`
		ScenarioType string `json:"scenario_type"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawScenarios); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	if len(rawScenarios) == 0 {
		return nil, fmt.Errorf("no scenarios found in parsed JSON")
	}

	// Convert to TestScenario format
	scenarios := make([]types.TestScenario, len(rawScenarios))
	for i, raw := range rawScenarios {
		scenarios[i] = types.TestScenario{
			Task:         raw.Task,
			Description:  raw.Description,
			ScenarioType: raw.ScenarioType,
			Variables:    make(map[string]interface{}),
		}
	}

	return scenarios, nil
}

func saveJSON(path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0644)
}

func saveProgress(progress *types.TestingProgress) {
	if progress.OutputFiles.Progress == "" {
		return
	}
	saveJSON(progress.OutputFiles.Progress, progress)
}

func updateProgress(progress *types.TestingProgress, status string, errorMsg string) {
	progress.Status = status
	progress.UpdatedAt = time.Now()
	if errorMsg != "" {
		progress.Error = errorMsg
	}
	saveProgress(progress)
}

func startPhase(progress *types.TestingProgress, phaseName string) {
	now := time.Now()
	phase := progress.Phases[phaseName]
	phase.Status = "in_progress"
	phase.StartedAt = &now
	progress.Phases[phaseName] = phase
	progress.Status = phaseName
	progress.UpdatedAt = now
	saveProgress(progress)
}

func completePhase(progress *types.TestingProgress, phaseName string, details map[string]interface{}) {
	now := time.Now()
	phase := progress.Phases[phaseName]
	phase.Status = "completed"
	phase.CompletedAt = &now
	if phase.StartedAt != nil {
		phase.DurationSeconds = now.Sub(*phase.StartedAt).Seconds()
	}
	phase.Details = details
	progress.Phases[phaseName] = phase
	progress.UpdatedAt = now
	saveProgress(progress)
}

func failPhase(progress *types.TestingProgress, phaseName string, errorMsg string) {
	now := time.Now()
	phase := progress.Phases[phaseName]
	phase.Status = "failed"
	phase.CompletedAt = &now
	if phase.StartedAt != nil {
		phase.DurationSeconds = now.Sub(*phase.StartedAt).Seconds()
	}
	phase.Details = map[string]interface{}{"error": errorMsg}
	progress.Phases[phaseName] = phase
	progress.Error = errorMsg
	progress.UpdatedAt = now
	saveProgress(progress)
}

func updatePhaseProgress(progress *types.TestingProgress, phaseName string, completed int, total int) {
	phase := progress.Phases[phaseName]
	if phase.Details == nil {
		phase.Details = make(map[string]interface{})
	}
	phase.Details["completed"] = completed
	phase.Details["total"] = total
	progress.Phases[phaseName] = phase
	progress.UpdatedAt = time.Now()
	// Save less frequently to avoid IO overhead
	if completed%10 == 0 || completed == total {
		saveProgress(progress)
	}
}

func generateMarkdownReport(dataset *types.ComprehensiveDataset) string {
	report := fmt.Sprintf(`# Agent Testing Report

**Agent:** %s (ID: %d)
**Generated:** %s
**Total Runs:** %d
**Scenarios:** %d
**Traces Captured:** %d

## Summary

- Success Rate: %.1f%%
- Total Runs: %d
- Jaeger Traces: %s

## Runs

`, dataset.Metadata.AgentName, dataset.Metadata.AgentID,
		dataset.Metadata.GeneratedAt.Format("2006-01-02 15:04:05"),
		dataset.Metadata.TotalRuns, dataset.Metadata.ScenarioCount,
		dataset.Metadata.TracesCaptured,
		calculateSuccessRate(dataset.Runs),
		dataset.Metadata.TotalRuns,
		func() string {
			if dataset.Metadata.JaegerAvailable {
				return fmt.Sprintf("✅ %d traces", dataset.Metadata.TracesCaptured)
			}
			return "❌ Not available"
		}())

	// Add run details
	for i, run := range dataset.Runs {
		status := "✅"
		if !run.Success {
			status = "❌"
		}
		report += fmt.Sprintf("\n### Run %d %s\n", i+1, status)
		report += fmt.Sprintf("- **Task:** %s\n", truncate(run.Task, 100))
		report += fmt.Sprintf("- **Duration:** %.2fs\n", run.DurationSeconds)
		report += fmt.Sprintf("- **Steps:** %d\n", run.StepsTaken)
		if run.Trace != nil {
			report += fmt.Sprintf("- **Tools Used:** %d\n", len(run.Trace.ToolCallSequence))
		}
		if !run.Success {
			report += fmt.Sprintf("- **Error:** %s\n", run.Error)
		}
	}

	return report
}

func calculateSuccessRate(runs []types.EnrichedRun) float64 {
	if len(runs) == 0 {
		return 0
	}
	successful := 0
	for _, run := range runs {
		if run.Success {
			successful++
		}
	}
	return float64(successful) / float64(len(runs)) * 100
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
