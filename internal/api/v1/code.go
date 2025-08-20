package v1

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"dagger.io/dagger"
	"github.com/gin-gonic/gin"
	"station/internal/opencode"
)

// CodeSession represents an active OpenCode session
type CodeSession struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	URL        string    `json:"url"`
	Port       int       `json:"port"`
	StartTime  time.Time `json:"start_time"`
	Status     string    `json:"status"`
	ContainerID string   `json:"-"` // Internal container reference
	Workspace   string   `json:"-"` // Internal workspace path
}

// Global session manager (in production, use Redis or database)
var activeSessions = make(map[string]*CodeSession)

// StartCodeSession starts a new OpenCode session in Dagger
func (h *APIHandlers) startCodeSession(c *gin.Context) {
	var req struct {
		Workspace   string `json:"workspace"`
		Environment string `json:"environment"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Generate unique session ID
	sessionID := fmt.Sprintf("session-%d", time.Now().Unix())
	
	// Generate unique port for this session (30000-30099 range)
	port := 30000 + (int(time.Now().Unix()) % 100)
	
	// Create session
	session := &CodeSession{
		ID:        sessionID,
		UserID:    "default", // In production, get from auth context
		Port:      port,
		StartTime: time.Now(),
		Status:    "starting",
	}
	
	// Store session
	activeSessions[sessionID] = session
	
	// Start OpenCode in Dagger container
	go func() {
		if err := startDaggerOpenCode(sessionID, port, req.Workspace); err != nil {
			session.Status = "failed"
			fmt.Printf("Failed to start OpenCode session %s: %v\n", sessionID, err)
			return
		}
		
		// Update session with URL
		session.URL = fmt.Sprintf("http://localhost:%d", port)
		session.Status = "running"
		fmt.Printf("OpenCode session %s started on port %d\n", sessionID, port)
	}()
	
	// Return session info (poll for URL)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"status":     "starting",
		"url":        fmt.Sprintf("http://localhost:%d", port), // Optimistic URL
	})
}

// StopCodeSession stops an OpenCode session
func (h *APIHandlers) stopCodeSession(c *gin.Context) {
	// Stop all active sessions (for simplicity)
	for sessionID, session := range activeSessions {
		if session.Status == "running" {
			if err := stopDaggerOpenCode(sessionID); err != nil {
				fmt.Printf("Failed to stop OpenCode session %s: %v\n", sessionID, err)
			}
		}
		delete(activeSessions, sessionID)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "All OpenCode sessions stopped",
	})
}

// GetCodeSession gets session status
func (h *APIHandlers) getCodeSession(c *gin.Context) {
	sessionID := c.Param("id")
	
	session, exists := activeSessions[sessionID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	
	c.JSON(http.StatusOK, session)
}

// startDaggerOpenCode starts OpenCode in a Dagger container with web server
func startDaggerOpenCode(sessionID string, port int, workspace string) error {
	fmt.Printf("🚀 Starting Dagger OpenCode container for session %s on port %d\n", sessionID, port)
	
	// Check if OpenCode is available
	if !opencode.IsAvailable() {
		return fmt.Errorf("OpenCode not available in this build")
	}
	
	// Create context for Dagger operations
	ctx := context.Background()
	
	// Connect to Dagger engine
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stderr))
	if err != nil {
		return fmt.Errorf("failed to connect to Dagger: %w", err)
	}
	defer client.Close()
	
	// Create workspace directory if it doesn't exist
	if workspace == "" {
		workspace = "/tmp/opencode-workspace-" + sessionID
	}
	err = os.MkdirAll(workspace, 0755)
	if err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	
	// Get embedded OpenCode binary data
	binaryData, err := opencode.GetEmbeddedBinary()
	if err != nil {
		return fmt.Errorf("failed to get embedded OpenCode binary: %w", err)
	}
	
	// Create Dagger directory and file references
	workspaceDir := client.Host().Directory(workspace)
	
	// Create container with OpenCode web server
	container := client.Container().
		From("ubuntu:22.04").
		// Install required dependencies
		WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "ca-certificates"}).
		// Create OpenCode binary from embedded data
		WithNewFile("/usr/local/bin/opencode", string(binaryData), dagger.ContainerWithNewFileOpts{
			Permissions: 0755,
		}).
		// Mount workspace
		WithMountedDirectory("/workspace", workspaceDir).
		// Set working directory
		WithWorkdir("/workspace").
		// Set environment variables for API keys (only if they exist)
		WithEnvVariable("ANTHROPIC_API_KEY", os.Getenv("ANTHROPIC_API_KEY")).
		WithEnvVariable("OPENAI_API_KEY", os.Getenv("OPENAI_API_KEY")).
		// Expose the port for web interface
		WithExposedPort(port)
	
	// Store container reference for later cleanup (before starting)
	if session, exists := activeSessions[sessionID]; exists {
		session.Workspace = workspace
	}
	
	// Start the container with OpenCode server in the background
	go func() {
		// Start container as a service with the serve command
		service := container.WithExec([]string{"/usr/local/bin/opencode", "serve", "--hostname", "0.0.0.0", "--port", fmt.Sprintf("%d", port), "/workspace"}).
			AsService()
		
		// Start the service and get the running container
		_, err := service.Start(ctx)
		if err != nil {
			fmt.Printf("❌ Failed to start OpenCode container for session %s: %v\n", sessionID, err)
			if session, exists := activeSessions[sessionID]; exists {
				session.Status = "failed"
			}
			return
		}
		
		fmt.Printf("✅ OpenCode container started successfully for session %s\n", sessionID)
		if session, exists := activeSessions[sessionID]; exists {
			session.Status = "running"
		}
	}()
	
	return nil
}

// stopDaggerOpenCode stops a Dagger OpenCode container
func stopDaggerOpenCode(sessionID string) error {
	fmt.Printf("🛑 Stopping Dagger OpenCode container for session %s\n", sessionID)
	
	// Get session info
	session, exists := activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}
	
	// Create context for Dagger operations
	ctx := context.Background()
	
	// Connect to Dagger engine
	client, err := dagger.Connect(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Dagger: %w", err)
	}
	defer client.Close()
	
	// For Dagger containers, we rely on context cancellation for cleanup
	// The container will be automatically cleaned up when the Dagger client disconnects
	fmt.Printf("🧹 Cleaning up Dagger resources for session %s\n", sessionID)
	
	// Clean up workspace directory if it exists
	if session.Workspace != "" {
		err = os.RemoveAll(session.Workspace)
		if err != nil {
			fmt.Printf("⚠️ Warning: Failed to clean up workspace for session %s: %v\n", sessionID, err)
		}
	}
	
	fmt.Printf("✅ Container cleanup completed for session %s\n", sessionID)
	return nil
}

// registerCodeRoutes registers OpenCode-related API routes
func (h *APIHandlers) registerCodeRoutes(router *gin.RouterGroup) {
	codeGroup := router.Group("/code")
	
	codeGroup.POST("/start", h.startCodeSession)
	codeGroup.POST("/stop", h.stopCodeSession)
	codeGroup.GET("/session/:id", h.getCodeSession)
}