package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"station/internal/db/repositories"
	"station/pkg/models"
)

// extractInt64FromInterface safely extracts int64 from various numeric types
func extractInt64FromInterface(value interface{}) *int64 {
	if value == nil {
		return nil
	}
	
	switch v := value.(type) {
	case int64:
		return &v
	case int:
		val := int64(v)
		return &val
	case int32:
		val := int64(v)
		return &val
	case float64:
		val := int64(v)
		return &val
	case float32:
		val := int64(v)
		return &val
	default:
		return nil
	}
}

// ExecutionRequest represents a request to execute an agent
type ExecutionRequest struct {
	AgentID   int64
	UserID    int64
	Task      string
	Metadata  map[string]interface{} // Additional metadata like source (cron/manual)
	Timestamp time.Time
	RunID     int64 // ID of the run record created when queued
}

// ExecutionResult represents the result of an agent execution
type ExecutionResult struct {
	Request         *ExecutionRequest
	Response        *Message
	StepsTaken      int64
	ToolCalls       []interface{}
	ExecutionSteps  []interface{}
	Status          string // "completed", "failed", "timeout"
	Error           error
	StartedAt       time.Time
	CompletedAt     time.Time
	// Response object metadata from Station's OpenAI plugin
	InputTokens     *int64
	OutputTokens    *int64
	TotalTokens     *int64
	DurationSeconds *float64
	ModelName       *string
	ToolsUsed       *int64
}

// ExecutionQueueService manages async agent execution using Go channels and worker pools
type ExecutionQueueService struct {
	// Core dependencies
	repos            *repositories.Repositories
	agentService     AgentServiceInterface
	webhookService   *WebhookService
	
	// Queue management
	requestQueue     chan *ExecutionRequest
	resultQueue      chan *ExecutionResult
	workers          []Worker
	numWorkers       int
	
	// Lifecycle management
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	running          bool
	mu               sync.RWMutex
}

// Worker represents a worker goroutine that processes execution requests
type Worker struct {
	ID               int
	ExecutionService AgentServiceInterface
	RequestQueue     <-chan *ExecutionRequest
	ResultQueue      chan<- *ExecutionResult
	ctx              context.Context
}

// NewExecutionQueueService creates a new execution queue service
func NewExecutionQueueService(repos *repositories.Repositories, agentService AgentServiceInterface, webhookService *WebhookService, numWorkers int) *ExecutionQueueService {
	if numWorkers <= 0 {
		numWorkers = 5 // Default to 5 workers
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ExecutionQueueService{
		repos:            repos,
		agentService:     agentService,
		webhookService:   webhookService,
		requestQueue:     make(chan *ExecutionRequest, 100), // Buffered channel for 100 pending requests
		resultQueue:      make(chan *ExecutionResult, 100),  // Buffered channel for 100 pending results
		numWorkers:       numWorkers,
		ctx:              ctx,
		cancel:           cancel,
		running:          false,
	}
}

// Start starts the execution queue service with workers and result processor
func (eq *ExecutionQueueService) Start() error {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	
	if eq.running {
		return fmt.Errorf("execution queue service is already running")
	}
	
	log.Printf("Starting execution queue service with %d workers...", eq.numWorkers)
	
	// Start workers
	eq.workers = make([]Worker, eq.numWorkers)
	for i := 0; i < eq.numWorkers; i++ {
		worker := Worker{
			ID:               i + 1,
			ExecutionService: eq.agentService,
			RequestQueue:     eq.requestQueue,
			ResultQueue:      eq.resultQueue,
			ctx:              eq.ctx,
		}
		eq.workers[i] = worker
		
		eq.wg.Add(1)
		go eq.runWorker(&worker)
	}
	
	// Start result processor
	eq.wg.Add(1)
	go eq.runResultProcessor()
	
	eq.running = true
	log.Printf("Execution queue service started successfully with %d workers", eq.numWorkers)
	
	return nil
}

// Stop gracefully stops the execution queue service with timeout
func (eq *ExecutionQueueService) Stop() {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	
	if !eq.running {
		return
	}
	
	log.Println("Stopping execution queue service...")
	
	// Cancel context to signal workers to stop immediately
	eq.cancel()
	
	// Close request queue to signal no more requests
	close(eq.requestQueue)
	
	// Wait for workers with aggressive timeout
	done := make(chan struct{})
	go func() {
		eq.wg.Wait()
		close(done)
	}()
	
	// 1 second timeout for worker shutdown
	select {
	case <-done:
		log.Println("All workers stopped gracefully")
	case <-time.After(1 * time.Second):
		log.Println("Worker shutdown timeout - forcing stop")
	}
	
	// Close result queue
	close(eq.resultQueue)
	
	eq.running = false
	log.Println("Execution queue service stopped")
}

// QueueExecution adds an execution request to the queue
func (eq *ExecutionQueueService) QueueExecution(agentID, userID int64, task string, metadata map[string]interface{}) (int64, error) {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	
	if !eq.running {
		return 0, fmt.Errorf("execution queue service is not running")
	}
	
	// Create a run record immediately when queuing (with "queued" status)
	agentRun, err := eq.repos.AgentRuns.Create(
		agentID,
		userID,
		task,
		"", // finalResponse - empty until execution completes
		0,  // stepsTaken - 0 until execution completes
		nil, // toolCalls - empty until execution completes
		nil, // executionSteps - empty until execution completes
		"queued", // status - queued until execution starts
		nil, // completedAt - nil until execution completes
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create run record: %w", err)
	}
	
	request := &ExecutionRequest{
		AgentID:   agentID,
		UserID:    userID,
		Task:      task,
		Metadata:  metadata,
		Timestamp: time.Now(),
		RunID:     agentRun.ID, // Include the run ID in the request
	}
	
	select {
	case eq.requestQueue <- request:
		log.Printf("Queued execution request for agent %d, user %d, run ID %d", agentID, userID, agentRun.ID)
		return agentRun.ID, nil
	case <-eq.ctx.Done():
		return 0, fmt.Errorf("execution queue service is shutting down")
	default:
		return 0, fmt.Errorf("execution queue is full, please try again later")
	}
}

// GetQueueStatus returns information about the current queue status
func (eq *ExecutionQueueService) GetQueueStatus() map[string]interface{} {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	
	return map[string]interface{}{
		"running":          eq.running,
		"num_workers":      eq.numWorkers,
		"pending_requests": len(eq.requestQueue),
		"pending_results":  len(eq.resultQueue),
	}
}

// runWorker runs a single worker that processes execution requests
func (eq *ExecutionQueueService) runWorker(worker *Worker) {
	defer eq.wg.Done()
	
	log.Printf("Worker %d started", worker.ID)
	defer log.Printf("Worker %d stopped", worker.ID)
	
	for {
		select {
		case request, ok := <-worker.RequestQueue:
			if !ok {
				// Channel closed, worker should exit
				return
			}
			
			// Process the execution request
			result := eq.executeRequest(worker, request)
			
			// Send result to result processor
			select {
			case worker.ResultQueue <- result:
				// Result successfully queued for processing
			case <-worker.ctx.Done():
				// Service is shutting down
				return
			default:
				// Result queue is full, log the issue
				log.Printf("Worker %d: Result queue full, dropping result for agent %d", worker.ID, request.AgentID)
			}
			
		case <-worker.ctx.Done():
			// Service is shutting down
			return
		}
	}
}

// executeRequest executes a single agent execution request
func (eq *ExecutionQueueService) executeRequest(worker *Worker, request *ExecutionRequest) *ExecutionResult {
	startTime := time.Now()
	
	log.Printf("Worker %d: Executing agent %d for user %d with task: %.50s...", 
		worker.ID, request.AgentID, request.UserID, request.Task)
	
	// Create execution context with timeout
	ctx, cancel := context.WithTimeout(worker.ctx, 10*time.Minute) // 10-minute timeout
	defer cancel()
	
	// Extract user variables from metadata if available
	var userVariables map[string]interface{}
	if request.Metadata != nil {
		if variables, ok := request.Metadata["user_variables"]; ok {
			if variablesMap, ok := variables.(map[string]interface{}); ok {
				userVariables = variablesMap
			}
		}
	}
	if userVariables == nil {
		userVariables = make(map[string]interface{}) // Default to empty map
	}
	
	// Execute the agent using AgentServiceInterface with variables support
	log.Printf("DEBUG ExecutionQueue: About to call ExecuteAgent for agent %d with %d variables", request.AgentID, len(userVariables))
	response, err := worker.ExecutionService.ExecuteAgent(ctx, request.AgentID, request.Task, userVariables)
	log.Printf("DEBUG ExecutionQueue: ExecuteAgent returned for agent %d, error: %v", request.AgentID, err)
	
	endTime := time.Now()
	
	result := &ExecutionResult{
		Request:   request,
		StartedAt: startTime,
		CompletedAt: endTime,
	}
	
	if err != nil {
		log.Printf("Worker %d: Agent %d execution failed: %v", worker.ID, request.AgentID, err)
		result.Status = "failed"
		result.Error = err
	} else {
		log.Printf("Worker %d: Agent %d execution completed successfully", worker.ID, request.AgentID)
		result.Response = response
		result.Status = "completed"
		
		// Extract detailed execution data from response.Extra if available
		if response != nil && response.Extra != nil {
			// Extract steps taken from agent execution result
			if stepsValue, exists := response.Extra["steps_taken"]; exists {
				if steps, ok := stepsValue.(int64); ok {
					result.StepsTaken = steps
				} else {
					result.StepsTaken = int64(1) // Default fallback
				}
			} else {
				result.StepsTaken = int64(1) // Default fallback
			}
			
			// Extract detailed tool calls if available
			if toolCallsValue, exists := response.Extra["tool_calls"]; exists {
				if toolCallsPtr, ok := toolCallsValue.(*models.JSONArray); ok && toolCallsPtr != nil {
					result.ToolCalls = []interface{}(*toolCallsPtr)
					log.Printf("Worker %d: Extracted %d tool calls from agent execution", worker.ID, len(result.ToolCalls))
				} else {
				}
			} else {
			}
			
			// Extract detailed execution steps if available
			if executionStepsValue, exists := response.Extra["execution_steps"]; exists {
				if executionStepsPtr, ok := executionStepsValue.(*models.JSONArray); ok && executionStepsPtr != nil {
					result.ExecutionSteps = []interface{}(*executionStepsPtr)
					log.Printf("Worker %d: Extracted %d execution steps from agent execution", worker.ID, len(result.ExecutionSteps))
				}
			}
			
			// If no detailed execution steps were provided, create basic fallback
			if result.ExecutionSteps == nil {
				result.ExecutionSteps = []interface{}{
					map[string]interface{}{
						"step": 1,
						"type": "agent_execution",
						"input": request.Task,
						"output": response.Content,
						"timestamp": startTime,
					},
				}
			}
			
			// Extract response object metadata from Station's OpenAI plugin
			if tokenUsageValue, exists := response.Extra["token_usage"]; exists {
				if tokenUsage, ok := tokenUsageValue.(map[string]interface{}); ok {
					// Handle various numeric types for token usage
					if inputVal := extractInt64FromInterface(tokenUsage["input_tokens"]); inputVal != nil {
						result.InputTokens = inputVal
					}
					if outputVal := extractInt64FromInterface(tokenUsage["output_tokens"]); outputVal != nil {
						result.OutputTokens = outputVal
					}
					if totalVal := extractInt64FromInterface(tokenUsage["total_tokens"]); totalVal != nil {
						result.TotalTokens = totalVal
					}
					log.Printf("Worker %d: Extracted token usage - Input: %v, Output: %v, Total: %v", worker.ID, result.InputTokens, result.OutputTokens, result.TotalTokens)
				}
			}
			
			if durationValue, exists := response.Extra["duration"]; exists {
				if duration, ok := durationValue.(float64); ok {
					result.DurationSeconds = &duration
					log.Printf("Worker %d: Extracted duration: %v seconds", worker.ID, duration)
				}
			}
			
			if modelNameValue, exists := response.Extra["model_name"]; exists {
				if modelName, ok := modelNameValue.(string); ok {
					result.ModelName = &modelName
					log.Printf("Worker %d: Extracted model name: %s", worker.ID, modelName)
				}
			}
			
			if toolsUsedValue, exists := response.Extra["tools_used"]; exists {
				if toolsUsed := extractInt64FromInterface(toolsUsedValue); toolsUsed != nil {
					result.ToolsUsed = toolsUsed
					log.Printf("Worker %d: Extracted tools used: %d", worker.ID, *toolsUsed)
				}
			}
		} else {
			// Fallback when no extra data is available
			result.StepsTaken = int64(1)
			result.ExecutionSteps = []interface{}{
				map[string]interface{}{
					"step": 1,
					"type": "basic_execution",
					"input": request.Task,
					"output": response.Content,
					"timestamp": startTime,
				},
			}
		}
	}
	
	return result
}

// runResultProcessor processes execution results and stores them in the database
func (eq *ExecutionQueueService) runResultProcessor() {
	defer eq.wg.Done()
	
	log.Println("Result processor started")
	defer log.Println("Result processor stopped")
	
	for {
		select {
		case result, ok := <-eq.resultQueue:
			if !ok {
				// Channel closed, processor should exit
				return
			}
			
			// Store result in database
			if err := eq.storeExecutionResult(result); err != nil {
				log.Printf("Failed to store execution result for agent %d: %v", result.Request.AgentID, err)
			} else {
				log.Printf("Stored execution result for agent %d, status: %s", result.Request.AgentID, result.Status)
				
				// Send webhook notifications for completed executions if webhook service is available
				if eq.webhookService != nil && result.Status == "completed" {
					go eq.sendWebhookNotification(result)
				}
			}
			
		case <-eq.ctx.Done():
			// Service is shutting down
			return
		}
	}
}

// storeExecutionResult stores an execution result in the agent_runs table
func (eq *ExecutionQueueService) storeExecutionResult(result *ExecutionResult) error {
	// Prepare data for database storage
	finalResponse := ""
	if result.Response != nil {
		finalResponse = result.Response.Content
	}
	
	// Handle error cases
	if result.Error != nil {
		finalResponse = fmt.Sprintf("Error: %v", result.Error)
	}
	
	// Convert tool calls and execution steps to JSONArray
	var toolCalls *models.JSONArray
	if result.ToolCalls != nil {
		jsonArray := models.JSONArray(result.ToolCalls)
		toolCalls = &jsonArray
	} else {
	}
	
	var executionSteps *models.JSONArray
	if result.ExecutionSteps != nil {
		jsonArray := models.JSONArray(result.ExecutionSteps)
		executionSteps = &jsonArray
	} else {
	}
	
	// Update the existing agent run record (created when queued) with response metadata
	runID := result.Request.RunID
	err := eq.repos.AgentRuns.UpdateCompletionWithMetadata(
		runID,
		finalResponse,
		result.StepsTaken,
		toolCalls,
		executionSteps,
		result.Status,
		&result.CompletedAt,
		result.InputTokens,
		result.OutputTokens,
		result.TotalTokens,
		result.DurationSeconds,
		result.ModelName,
		result.ToolsUsed,
	)
	
	if err != nil {
		return fmt.Errorf("failed to update agent run record %d: %w", runID, err)
	}
	
	log.Printf("Updated agent run record with ID %d for agent %d", runID, result.Request.AgentID)
	return nil
}

// sendWebhookNotification sends webhook notifications for completed agent runs
func (eq *ExecutionQueueService) sendWebhookNotification(result *ExecutionResult) {
	// Get the updated agent run record from database
	agentRun, err := eq.repos.AgentRuns.GetByID(result.Request.RunID)
	if err != nil {
		log.Printf("Failed to get agent run %d for webhook notification: %v", result.Request.RunID, err)
		return
	}
	
	// Get the agent details
	agent, err := eq.repos.Agents.GetByID(result.Request.AgentID)
	if err != nil {
		log.Printf("Failed to get agent %d for webhook notification: %v", result.Request.AgentID, err)
		return
	}
	
	// Send webhook notification
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := eq.webhookService.NotifyAgentRunCompleted(ctx, agentRun, agent); err != nil {
		log.Printf("Failed to send webhook notification for agent run %d: %v", agentRun.ID, err)
	} else {
		log.Printf("Webhook notifications sent for agent run %d", agentRun.ID)
	}
}