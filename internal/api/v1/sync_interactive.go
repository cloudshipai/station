package v1

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
)

// SyncRequest represents a sync request payload
type SyncRequest struct {
	Environment string `json:"environment"`
	DryRun      bool   `json:"dry_run,omitempty"`
	Force       bool   `json:"force,omitempty"`
}

// SyncStatus represents the current state of a sync operation
type SyncStatus struct {
	ID          string                 `json:"id"`
	Status      string                 `json:"status"` // "running", "waiting_for_input", "completed", "failed"
	Environment string                 `json:"environment"`
	Progress    SyncProgress           `json:"progress"`
	Variables   *VariableRequest       `json:"variables,omitempty"`
	Result      *services.SyncResult   `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// SyncProgress tracks the progress of sync operations
type SyncProgress struct {
	CurrentStep   string `json:"current_step"`
	StepsTotal    int    `json:"steps_total"`
	StepsComplete int    `json:"steps_complete"`
	Message       string `json:"message"`
}

// VariableRequest represents a request for missing variables
type VariableRequest struct {
	ConfigName  string          `json:"config_name"`
	Variables   []VariableInput `json:"variables"`
	Message     string          `json:"message"`
}

// VariableInput represents a variable that needs user input
type VariableInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Default     string `json:"default,omitempty"`
}

// VariableResponse represents user-provided variable values
type VariableResponse struct {
	SyncID    string            `json:"sync_id"`
	Variables map[string]string `json:"variables"`
}

// VariableChannel represents a channel for variable communication
type VariableChannel struct {
	Variables chan map[string]string
	Error     chan error
}

// In-memory store for sync operations (in production, this should be Redis or database)
var activeSyncs = make(map[string]*SyncStatus)
var variableChannels = make(map[string]*VariableChannel)

// startInteractiveSync initiates an interactive sync operation
func (h *APIHandlers) startInteractiveSync(c *gin.Context) {
	var req SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Generate unique sync ID
	syncID := fmt.Sprintf("sync_%d", time.Now().UnixNano())

	// Create sync status
	syncStatus := &SyncStatus{
		ID:          syncID,
		Status:      "running",
		Environment: req.Environment,
		Progress: SyncProgress{
			CurrentStep:   "Initializing sync",
			StepsTotal:    4, // Initialize, Validate, Process, Complete
			StepsComplete: 0,
			Message:       "Starting sync operation...",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store sync status
	activeSyncs[syncID] = syncStatus

	// Create variable channel for this sync
	variableChannels[syncID] = &VariableChannel{
		Variables: make(chan map[string]string, 1),
		Error:     make(chan error, 1),
	}

	// Start sync in background
	go h.executeSyncWithVariablePrompts(syncID, req)

	c.JSON(http.StatusOK, gin.H{
		"sync_id": syncID,
		"status":  syncStatus,
	})
}

// getSyncStatus returns the current status of a sync operation
func (h *APIHandlers) getSyncStatus(c *gin.Context) {
	syncID := c.Param("id")
	
	syncStatus, exists := activeSyncs[syncID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync operation not found"})
		return
	}

	c.JSON(http.StatusOK, syncStatus)
}

// submitVariables handles user submission of required variables
func (h *APIHandlers) submitVariables(c *gin.Context) {
	var req VariableResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid variable response"})
		return
	}

	syncStatus, exists := activeSyncs[req.SyncID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sync operation not found"})
		return
	}

	if syncStatus.Status != "waiting_for_input" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Sync is not waiting for input"})
		return
	}

	// Store variables and continue sync by sending them through the channel
	if variableChannel, exists := variableChannels[req.SyncID]; exists {
		variableChannel.Variables <- req.Variables
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Variable channel not found for sync operation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "variables_received",
		"message": "Variables submitted successfully, continuing sync...",
	})
}

// executeSyncWithVariablePrompts runs the sync operation with interactive variable prompting
func (h *APIHandlers) executeSyncWithVariablePrompts(syncID string, req SyncRequest) {
	variableChannel := variableChannels[syncID]
	
	// Cleanup channels when done - delay cleanup to allow final status polling
	defer func() {
		// Wait a bit before cleanup to allow final polling requests
		go func() {
			time.Sleep(3 * time.Second)
			log.Printf("Cleaning up sync operation %s", syncID)
			delete(variableChannels, syncID)
			delete(activeSyncs, syncID)
		}()
	}()
	
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		h.updateSyncStatus(syncID, "failed", "Failed to load configuration", nil, err.Error())
		return
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		h.updateSyncStatus(syncID, "failed", "Failed to initialize database", nil, err.Error())
		return
	}
	defer database.Close()

	repos := repositories.New(database)

	// Update progress
	h.updateSyncProgress(syncID, "Validating environment", 1, "Checking environment configuration...")

	// Create custom variable resolver that uses the UI
	variableResolver := func(missingVars []services.VariableInfo) (map[string]string, error) {
		if len(missingVars) == 0 {
			return make(map[string]string), nil
		}

		// Convert to API types and create variable request
		variables := make([]VariableInput, len(missingVars))
		for i, v := range missingVars {
			variables[i] = VariableInput{
				Name:        v.Name,
				Description: v.Description,
				Required:    v.Required,
				Secret:      v.Secret,
				Default:     v.Default,
			}
		}

		variableRequest := &VariableRequest{
			ConfigName: "template",
			Variables:  variables,
			Message:    fmt.Sprintf("Missing %d variable(s) required for sync", len(missingVars)),
		}

		// Update sync status to waiting for input
		h.updateSyncStatusWithVariables(syncID, "waiting_for_input", "Waiting for variables", variableRequest)

		// Wait for user to provide variables
		select {
		case newVars := <-variableChannel.Variables:
			return newVars, nil
		case err := <-variableChannel.Error:
			return nil, err
		case <-time.After(10 * time.Minute):
			return nil, fmt.Errorf("timeout waiting for variables")
		}
	}

	// Use the existing DeclarativeSync service and inject the UI variable resolver
	syncer := services.NewDeclarativeSync(repos, cfg)
	syncer.SetVariableResolver(variableResolver)
	
	// Create sync options with interactive mode
	syncOptions := services.SyncOptions{
		DryRun:      req.DryRun,
		Validate:    false,
		Force:       req.Force,
		Verbose:     true,
		Interactive: true,
		Confirm:     true,
	}

	// Update progress
	h.updateSyncProgress(syncID, "Starting sync operation", 2, "Running declarative sync with interactive variable resolution...")

	// Execute the real sync using existing service
	result, err := syncer.SyncEnvironment(context.Background(), req.Environment, syncOptions)
	if err != nil {
		h.updateSyncStatus(syncID, "failed", "Sync operation failed", nil, err.Error())
		return
	}

	// Update progress and final status
	h.updateSyncProgress(syncID, "Processing results", 3, "Finalizing sync results...")
	h.updateSyncProgress(syncID, "Completed", 4, "Sync operation completed successfully")
	h.updateSyncStatus(syncID, "completed", "Sync completed successfully", result, "")
}

// Helper functions for updating sync status
func (h *APIHandlers) updateSyncStatus(syncID, status, message string, result *services.SyncResult, errorMsg string) {
	if syncStatus, exists := activeSyncs[syncID]; exists {
		log.Printf("Updating sync %s status to: %s", syncID, status)
		syncStatus.Status = status
		syncStatus.Progress.Message = message
		syncStatus.Result = result
		syncStatus.Error = errorMsg
		syncStatus.UpdatedAt = time.Now()
	} else {
		log.Printf("Warning: Tried to update non-existent sync %s", syncID)
	}
}

func (h *APIHandlers) updateSyncProgress(syncID, step string, stepNum int, message string) {
	if syncStatus, exists := activeSyncs[syncID]; exists {
		syncStatus.Progress.CurrentStep = step
		syncStatus.Progress.StepsComplete = stepNum
		syncStatus.Progress.Message = message
		syncStatus.UpdatedAt = time.Now()
	}
}

// Helper function to update sync status with variable request
func (h *APIHandlers) updateSyncStatusWithVariables(syncID, status, message string, variables *VariableRequest) {
	if syncStatus, exists := activeSyncs[syncID]; exists {
		syncStatus.Status = status
		syncStatus.Progress.Message = message
		syncStatus.Variables = variables
		syncStatus.UpdatedAt = time.Now()
	}
}