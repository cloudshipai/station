package v1

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/creack/pty"
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
	PTY        *os.File  `json:"-"` // PTY for OpenCode process
	Cmd        *exec.Cmd `json:"-"` // Running OpenCode command
	mu         sync.Mutex `json:"-"` // Mutex for thread-safe access
}

// Global session manager (in production, use Redis or database)
var activeSessions = make(map[string]*CodeSession)

// WebSocket upgrader for terminal connections
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin for development
		// In production, implement proper origin checking
		return true
	},
}

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
	
	// Start OpenCode with PTY (direct approach for WebSocket integration)
	go func() {
		if err := startOpenCodeWithPTY(sessionID, req.Workspace); err != nil {
			session.Status = "failed"
			fmt.Printf("Failed to start OpenCode session %s: %v\n", sessionID, err)
			return
		}
		fmt.Printf("OpenCode session %s started with PTY\n", sessionID)
	}()
	
	// Return session info (poll for URL)
	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"status":     "starting",
		"message":    "OpenCode container is starting. Use /api/v1/code/session/{id} to get the URL when ready.",
	})
}

// StopCodeSession stops an OpenCode session
func (h *APIHandlers) stopCodeSession(c *gin.Context) {
	// Stop all active sessions (for simplicity)
	for sessionID, session := range activeSessions {
		if session.Status == "running" {
			if err := stopOpenCodeSession(sessionID); err != nil {
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

// startOpenCodeWithPTY starts OpenCode directly with PTY for WebSocket connection
func startOpenCodeWithPTY(sessionID, workspace string) error {
	fmt.Printf("🚀 Starting OpenCode with PTY for session %s\n", sessionID)
	
	// Check if OpenCode is available
	if !opencode.IsAvailable() {
		return fmt.Errorf("OpenCode not available in this build")
	}
	
	// Create workspace directory if it doesn't exist
	if workspace == "" {
		workspace = "/tmp/opencode-workspace-" + sessionID
	}
	if err := os.MkdirAll(workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}
	
	// Get session to update with PTY info
	session, exists := activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}
	
	session.mu.Lock()
	defer session.mu.Unlock()
	
	// Extract embedded OpenCode binary to temp location
	binaryData, err := opencode.GetEmbeddedBinary()
	if err != nil {
		return fmt.Errorf("failed to get embedded OpenCode binary: %w", err)
	}
	
	// Create temporary binary file
	tmpBinary := fmt.Sprintf("/tmp/opencode-%s", sessionID)
	if err := os.WriteFile(tmpBinary, binaryData, 0755); err != nil {
		return fmt.Errorf("failed to write OpenCode binary: %w", err)
	}
	
	// Set up environment for OpenCode
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		anthropicKey = "sk-ant-dummy-key-for-testing"
	}
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey == "" {
		openaiKey = "sk-dummy-key-for-testing"
	}
	
	// Create OpenCode command with workspace
	cmd := exec.Command(tmpBinary, workspace)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_API_KEY="+anthropicKey,
		"OPENAI_API_KEY="+openaiKey,
		"TERM=xterm-256color",
		"OPENCODE=1",
	)
	
	// Start OpenCode with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		os.Remove(tmpBinary)
		return fmt.Errorf("failed to start OpenCode with PTY: %w", err)
	}
	
	// Store PTY and command in session
	session.PTY = ptmx
	session.Cmd = cmd
	session.Workspace = workspace
	session.Status = "running"
	session.URL = fmt.Sprintf("ws://localhost:8585/api/v1/code/session/%s/ws", sessionID)
	session.ContainerID = fmt.Sprintf("opencode-pty-%s", sessionID)
	
	fmt.Printf("✅ OpenCode started with PTY for session %s - WebSocket at %s\n", sessionID, session.URL)
	
	// Monitor process in background
	go func() {
		defer func() {
			session.mu.Lock()
			if session.PTY != nil {
				session.PTY.Close()
			}
			session.Status = "stopped"
			session.mu.Unlock()
			os.Remove(tmpBinary)
			fmt.Printf("🛑 OpenCode session %s terminated\n", sessionID)
		}()
		
		// Wait for process to finish
		if err := cmd.Wait(); err != nil {
			fmt.Printf("⚠️ OpenCode process for session %s exited with error: %v\n", sessionID, err)
		}
	}()
	
	return nil
}

// stopOpenCodeSession stops an OpenCode PTY session
func stopOpenCodeSession(sessionID string) error {
	fmt.Printf("🛑 Stopping OpenCode session %s\n", sessionID)
	
	// Get session info
	session, exists := activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}
	
	session.mu.Lock()
	defer session.mu.Unlock()
	
	// Close PTY and terminate process
	if session.PTY != nil {
		session.PTY.Close()
		session.PTY = nil
	}
	
	if session.Cmd != nil && session.Cmd.Process != nil {
		// Try graceful termination first
		session.Cmd.Process.Signal(os.Interrupt)
		
		// Wait a bit, then force kill if needed
		go func() {
			time.Sleep(2 * time.Second)
			if session.Cmd.Process != nil {
				session.Cmd.Process.Kill()
			}
		}()
	}
	
	session.Status = "stopped"
	
	fmt.Printf("✅ OpenCode session %s stopped\n", sessionID)
	return nil
}

// handleCodeWebSocket handles WebSocket connections for OpenCode terminal sessions
func (h *APIHandlers) handleCodeWebSocket(c *gin.Context) {
	sessionID := c.Param("id")
	
	// Get session
	session, exists := activeSessions[sessionID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}
	
	session.mu.Lock()
	if session.Status != "running" || session.PTY == nil {
		session.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session not ready"})
		return
	}
	pty := session.PTY
	session.mu.Unlock()
	
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("Failed to upgrade WebSocket for session %s: %v\n", sessionID, err)
		return
	}
	defer conn.Close()
	
	fmt.Printf("🔌 WebSocket connected for OpenCode session %s\n", sessionID)
	
	// Handle bidirectional communication between WebSocket and PTY
	go func() {
		// PTY -> WebSocket (read from OpenCode, send to browser)
		buf := make([]byte, 1024)
		for {
			n, err := pty.Read(buf)
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading from PTY for session %s: %v\n", sessionID, err)
				}
				break
			}
			
			if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
				fmt.Printf("Error writing to WebSocket for session %s: %v\n", sessionID, err)
				break
			}
		}
	}()
	
	// WebSocket -> PTY (read from browser, send to OpenCode)
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error for session %s: %v\n", sessionID, err)
			}
			break
		}
		
		if _, err := pty.Write(message); err != nil {
			fmt.Printf("Error writing to PTY for session %s: %v\n", sessionID, err)
			break
		}
	}
	
	fmt.Printf("🔌 WebSocket disconnected for OpenCode session %s\n", sessionID)
}

// registerCodeRoutes registers OpenCode-related API routes
func (h *APIHandlers) registerCodeRoutes(router *gin.RouterGroup) {
	codeGroup := router.Group("/code")
	
	codeGroup.POST("/start", h.startCodeSession)
	codeGroup.POST("/stop", h.stopCodeSession)
	codeGroup.GET("/session/:id", h.getCodeSession)
	codeGroup.GET("/session/:id/ws", h.handleCodeWebSocket)
}