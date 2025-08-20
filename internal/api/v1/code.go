package v1

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// CodeSession represents an active OpenCode session
type CodeSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	URL       string    `json:"url"`
	Port      int       `json:"port"`
	StartTime time.Time `json:"start_time"`
	Status    string    `json:"status"`
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

// startDaggerOpenCode starts OpenCode in a Dagger container with xterm.js
func startDaggerOpenCode(sessionID string, port int, workspace string) error {
	// For now, simulate starting a container
	// In production, this would use Dagger Go SDK to:
	// 1. Create container with OpenCode + xterm.js setup
	// 2. Mount workspace volume
	// 3. Set environment variables (API keys)
	// 4. Expose port with xterm.js web interface
	// 5. Start OpenCode connected to xterm.js PTY
	
	fmt.Printf("🚀 Starting Dagger OpenCode container for session %s on port %d\n", sessionID, port)
	
	// Simulate container startup time
	time.Sleep(2 * time.Second)
	
	// TODO: Replace with actual Dagger implementation:
	/*
	client := dagger.Connect()
	defer client.Close()
	
	container := client.Container().
		From("ubuntu:22.04").
		WithFile("/tmp/opencode", opencodeEmbedded).
		WithMountedDirectory("/workspace", workspace).
		WithExposedPort(port).
		WithExec([]string{"bash", "-c", "setup-xterm-js && /tmp/opencode serve --port 3030 /workspace"})
		
	_, err := container.Start()
	return err
	*/
	
	return nil
}

// stopDaggerOpenCode stops a Dagger OpenCode container
func stopDaggerOpenCode(sessionID string) error {
	fmt.Printf("🛑 Stopping Dagger OpenCode container for session %s\n", sessionID)
	
	// TODO: Replace with actual Dagger container cleanup
	/*
	// Stop and remove container by session ID
	client := dagger.Connect()
	defer client.Close()
	
	return client.Container().WithLabel("session", sessionID).Stop()
	*/
	
	return nil
}

// registerCodeRoutes registers OpenCode-related API routes
func (h *APIHandlers) registerCodeRoutes(router *gin.RouterGroup) {
	codeGroup := router.Group("/code")
	
	codeGroup.POST("/start", h.startCodeSession)
	codeGroup.POST("/stop", h.stopCodeSession)
	codeGroup.GET("/session/:id", h.getCodeSession)
}