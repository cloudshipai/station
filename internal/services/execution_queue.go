package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cloudwego/eino/schema"
	"station/internal/db/repositories"
	"station/pkg/models"
)

// ExecutionRequest represents a request to execute an agent
type ExecutionRequest struct {
	AgentID   int64
	UserID    int64
	Task      string
	Metadata  map[string]interface{} // Additional metadata like source (cron/manual)
	Timestamp time.Time
}

// ExecutionResult represents the result of an agent execution
type ExecutionResult struct {
	Request       *ExecutionRequest
	Response      *schema.Message
	StepsTaken    int64
	ToolCalls     []interface{}
	ExecutionSteps []interface{}
	Status        string // "completed", "failed", "timeout"
	Error         error
	StartedAt     time.Time
	CompletedAt   time.Time
}

// ExecutionQueueService manages async agent execution using Go channels and worker pools
type ExecutionQueueService struct {
	// Core dependencies
	repos            *repositories.Repositories
	einoAgentService *EinoAgentService
	
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
	ExecutionService *EinoAgentService
	RequestQueue     <-chan *ExecutionRequest
	ResultQueue      chan<- *ExecutionResult
	ctx              context.Context
}

// NewExecutionQueueService creates a new execution queue service
func NewExecutionQueueService(repos *repositories.Repositories, einoAgentService *EinoAgentService, numWorkers int) *ExecutionQueueService {
	if numWorkers <= 0 {
		numWorkers = 5 // Default to 5 workers
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ExecutionQueueService{
		repos:            repos,
		einoAgentService: einoAgentService,
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
			ExecutionService: eq.einoAgentService,
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

// Stop gracefully stops the execution queue service
func (eq *ExecutionQueueService) Stop() {
	eq.mu.Lock()
	defer eq.mu.Unlock()
	
	if !eq.running {
		return
	}
	
	log.Println("Stopping execution queue service...")
	
	// Cancel context to signal workers to stop
	eq.cancel()
	
	// Close request queue to signal no more requests
	close(eq.requestQueue)
	
	// Wait for all workers and result processor to finish
	eq.wg.Wait()
	
	// Close result queue
	close(eq.resultQueue)
	
	eq.running = false
	log.Println("Execution queue service stopped")
}

// QueueExecution adds an execution request to the queue
func (eq *ExecutionQueueService) QueueExecution(agentID, userID int64, task string, metadata map[string]interface{}) error {
	eq.mu.RLock()
	defer eq.mu.RUnlock()
	
	if !eq.running {
		return fmt.Errorf("execution queue service is not running")
	}
	
	request := &ExecutionRequest{
		AgentID:   agentID,
		UserID:    userID,
		Task:      task,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}
	
	select {
	case eq.requestQueue <- request:
		log.Printf("Queued execution request for agent %d, user %d", agentID, userID)
		return nil
	case <-eq.ctx.Done():
		return fmt.Errorf("execution queue service is shutting down")
	default:
		return fmt.Errorf("execution queue is full, please try again later")
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
	
	// Execute the agent using EinoAgentService
	response, err := worker.ExecutionService.ExecuteAgent(ctx, request.AgentID, request.Task)
	
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
		
		// Extract execution details from response if available
		// This is a simplified extraction - in a real implementation you might want
		// to capture more detailed execution steps and tool calls
		if response != nil {
			result.StepsTaken = 1 // Simplified - could be extracted from response metadata
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
	}
	
	var executionSteps *models.JSONArray
	if result.ExecutionSteps != nil {
		jsonArray := models.JSONArray(result.ExecutionSteps)
		executionSteps = &jsonArray
	}
	
	// Create agent run record
	agentRun, err := eq.repos.AgentRuns.Create(
		result.Request.AgentID,
		result.Request.UserID,
		result.Request.Task,
		finalResponse,
		result.StepsTaken,
		toolCalls,
		executionSteps,
		result.Status,
		&result.CompletedAt,
	)
	
	if err != nil {
		return fmt.Errorf("failed to create agent run record: %w", err)
	}
	
	log.Printf("Created agent run record with ID %d for agent %d", agentRun.ID, result.Request.AgentID)
	return nil
}