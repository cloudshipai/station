package mcp_agents

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"station/internal/auth"
	"station/internal/auth/oauth"
	"station/internal/config"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed openapi.yaml
var openAPISpec string

// authRequiredKey is a context key to signal that auth is required but not provided
type authRequiredKey struct{}

// ExecuteWebhookRequest represents the webhook payload for executing an agent
type ExecuteWebhookRequest struct {
	AgentName string                 `json:"agent_name,omitempty"`
	AgentID   int64                  `json:"agent_id,omitempty"`
	Task      string                 `json:"task"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// ExecuteWebhookResponse represents the webhook response
type ExecuteWebhookResponse struct {
	RunID     int64  `json:"run_id"`
	AgentID   int64  `json:"agent_id"`
	AgentName string `json:"agent_name"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// DynamicAgentServer manages a dynamic MCP server that serves database agents as individual tools
type DynamicAgentServer struct {
	repos           *repositories.Repositories
	agentService    services.AgentServiceInterface
	workflowService *services.WorkflowService
	mcpServer       *server.MCPServer
	httpServer      *server.StreamableHTTPServer
	localMode       bool
	environmentName string
	config          *config.Config
	oauthHandler    *oauth.CloudShipOAuth
}

// NewDynamicAgentServer creates a new dynamic agent MCP server with environment filtering
func NewDynamicAgentServer(repos *repositories.Repositories, agentService services.AgentServiceInterface, localMode bool, environmentName string) *DynamicAgentServer {
	return &DynamicAgentServer{
		repos:           repos,
		agentService:    agentService,
		localMode:       localMode,
		environmentName: environmentName,
	}
}

// NewDynamicAgentServerWithConfig creates a new dynamic agent MCP server with config for OAuth
func NewDynamicAgentServerWithConfig(repos *repositories.Repositories, agentService services.AgentServiceInterface, localMode bool, environmentName string, cfg *config.Config) *DynamicAgentServer {
	return &DynamicAgentServer{
		repos:           repos,
		agentService:    agentService,
		localMode:       localMode,
		environmentName: environmentName,
		config:          cfg,
	}
}

// SetWorkflowService sets the workflow service for approval endpoints
func (das *DynamicAgentServer) SetWorkflowService(svc *services.WorkflowService) {
	das.workflowService = svc
}

// Start starts the dynamic MCP server on the specified port
func (das *DynamicAgentServer) Start(ctx context.Context, port int) error {
	// Create a new MCP server
	das.mcpServer = server.NewMCPServer(
		"Station Dynamic Agents",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	if err := das.loadAgentsAsTools(); err != nil {
		return err
	}

	if err := das.loadWorkflowsAsTools(); err != nil {
		log.Printf("Warning: failed to load workflow tools: %v", err)
	}

	// Create OAuth handler if enabled
	if das.config != nil && das.config.CloudShip.OAuth.Enabled {
		das.oauthHandler = oauth.NewCloudShipOAuth(&das.config.CloudShip.OAuth)
		log.Printf("Dynamic Agent MCP OAuth authentication enabled")
	}

	// Create HTTP context function for authentication
	httpContextFunc := createDynamicAgentAuthContextFunc(das.repos, das.oauthHandler, das.localMode)

	// Start the HTTP server with auth context
	das.httpServer = server.NewStreamableHTTPServer(das.mcpServer,
		server.WithHTTPContextFunc(httpContextFunc),
	)

	// Get CloudShip base URL for OAuth discovery
	cloudshipBaseURL := "https://app.cloudshipai.com"
	if das.config != nil && das.config.CloudShip.BaseURL != "" {
		cloudshipBaseURL = das.config.CloudShip.BaseURL
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", das.handleHealth)
	mux.HandleFunc("/openapi.yaml", das.handleOpenAPISpec)
	mux.HandleFunc("/execute", das.handleExecuteWebhook)
	mux.HandleFunc("/runs/", das.handleAgentRuns)
	mux.HandleFunc("/workflow-runs/", das.handleWorkflowRuns)
	mux.HandleFunc("/workflow-runs", das.handleWorkflowRuns)
	mux.HandleFunc("/workflow-approvals/", das.handleWorkflowApprovals)
	mux.Handle("/mcp", das.httpServer)
	mux.Handle("/mcp/", das.httpServer)
	mux.HandleFunc("/", das.handleNotFound)

	// Determine the final handler based on auth configuration
	var handler http.Handler = mux

	// If OAuth is enabled and we're not in local mode, wrap with OAuth discovery middleware
	if !das.localMode && das.config != nil && das.config.CloudShip.OAuth.Enabled {
		handler = wrapWithOAuthDiscovery(mux, cloudshipBaseURL, das.repos, das.oauthHandler)
		log.Printf("Starting Dynamic Agent MCP server with OAuth discovery on %s", addr)
		log.Printf("  OAuth discovery URL: %s/.well-known/oauth-protected-resource", cloudshipBaseURL)
	} else {
		log.Printf("Starting Dynamic Agent MCP server on %s", addr)
	}

	// Log webhook status
	webhookEnabled := das.config == nil || das.config.Webhook.Enabled
	if webhookEnabled {
		log.Printf("  Webhook endpoint: http://%s/execute", addr)
		if das.config != nil && das.config.Webhook.APIKey != "" {
			log.Printf("  Webhook auth: Static API key (STN_WEBHOOK_API_KEY)")
		} else if das.localMode {
			log.Printf("  Webhook auth: Local mode (no auth required)")
		} else {
			log.Printf("  Webhook auth: Bearer token (user API key or OAuth)")
		}
	} else {
		log.Printf("  Webhook endpoint: Disabled (STN_WEBHOOK_ENABLED=false)")
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	return httpServer.ListenAndServe()
}

// handleExecuteWebhook handles the /execute webhook endpoint
func (das *DynamicAgentServer) handleExecuteWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if webhook is enabled
	if das.config != nil && !das.config.Webhook.Enabled {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Webhook endpoint is disabled",
		})
		return
	}

	// Only POST allowed
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "method_not_allowed"})
		return
	}

	// Authenticate request
	user, err := das.authenticateWebhookRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "unauthorized",
			Message: err.Error(),
		})
		return
	}

	// Parse request body
	var req ExecuteWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON: " + err.Error(),
		})
		return
	}

	// Validate required fields
	if req.AgentName == "" && req.AgentID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "Either agent_name or agent_id is required",
		})
		return
	}

	if req.Task == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "task is required",
		})
		return
	}

	// Resolve agent
	agent, err := das.resolveAgent(req.AgentName, req.AgentID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "not_found",
			Message: err.Error(),
		})
		return
	}

	// Create agent run record
	userID := user.ID
	if userID == 0 {
		userID = 1 // Default to console user for OAuth users
	}

	agentRun, err := das.repos.AgentRuns.Create(
		r.Context(),
		agent.ID,
		userID,
		req.Task,
		"",        // final_response (will be updated)
		0,         // steps_taken
		nil,       // tool_calls
		nil,       // execution_steps
		"running", // status
		nil,       // completed_at
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "internal_error",
			Message: "Failed to create agent run: " + err.Error(),
		})
		return
	}

	// Execute agent asynchronously
	go func() {
		ctx := context.Background()
		metadata := map[string]interface{}{
			"source":       "webhook",
			"triggered_by": "webhook",
			"user_id":      userID,
		}

		// Merge variables into metadata
		if req.Variables != nil {
			for k, v := range req.Variables {
				metadata[k] = v
			}
		}

		_, err := das.agentService.ExecuteAgentWithRunID(ctx, agent.ID, req.Task, agentRun.ID, metadata)
		if err != nil {
			log.Printf("Webhook agent execution failed (Run ID: %d, Agent: %s): %v", agentRun.ID, agent.Name, err)
		} else {
			log.Printf("Webhook agent execution completed (Run ID: %d, Agent: %s)", agentRun.ID, agent.Name)
		}
	}()

	// Return success response
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(ExecuteWebhookResponse{
		RunID:     agentRun.ID,
		AgentID:   agent.ID,
		AgentName: agent.Name,
		Status:    "running",
		Message:   "Agent execution started",
	})

	log.Printf("Webhook: Started agent execution (Run ID: %d, Agent: %s, Task: %s)", agentRun.ID, agent.Name, truncateString(req.Task, 50))
}

func (das *DynamicAgentServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":      "ok",
		"environment": das.environmentName,
	})
}

func (das *DynamicAgentServer) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(openAPISpec))
}

func (das *DynamicAgentServer) handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   "not_found",
		Message: fmt.Sprintf("Unknown endpoint: %s %s. Try /health, /execute, /runs/{id}, /workflow-runs, /mcp", r.Method, r.URL.Path),
	})
}

func (das *DynamicAgentServer) handleWorkflowApprovals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if das.workflowService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Workflow service not configured",
		})
		return
	}

	user, err := das.authenticateWebhookRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "unauthorized",
			Message: err.Error(),
		})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/workflow-approvals/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || parts[0] == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "Approval ID required in path: /workflow-approvals/{id}/approve or /workflow-approvals/{id}/reject",
		})
		return
	}

	approvalID := parts[0]
	action := ""
	if len(parts) >= 2 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		das.handleGetApproval(w, r, approvalID)
	case r.Method == http.MethodPost && action == "approve":
		das.handleApproveAction(w, r, approvalID, user.Username)
	case r.Method == http.MethodPost && action == "reject":
		das.handleRejectAction(w, r, approvalID, user.Username)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "method_not_allowed",
			Message: "Use GET /workflow-approvals/{id}, POST /workflow-approvals/{id}/approve, or POST /workflow-approvals/{id}/reject",
		})
	}
}

func (das *DynamicAgentServer) handleGetApproval(w http.ResponseWriter, r *http.Request, approvalID string) {
	approval, err := das.workflowService.GetApproval(r.Context(), approvalID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "not_found",
			Message: "Approval not found: " + approvalID,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approval": approval,
	})
}

func (das *DynamicAgentServer) handleApproveAction(w http.ResponseWriter, r *http.Request, approvalID, actorID string) {
	var req struct {
		Comment string `json:"comment"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	approval, err := das.workflowService.ApproveWorkflowStep(r.Context(), services.ApproveWorkflowStepRequest{
		ApprovalID: approvalID,
		ApproverID: actorID,
		Comment:    req.Comment,
	})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "approval_failed",
			Message: err.Error(),
		})
		return
	}

	log.Printf("Workflow approval approved via public API (ID: %s, Actor: %s)", approvalID, actorID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approval": approval,
		"message":  "Workflow step approved",
	})
}

func (das *DynamicAgentServer) handleRejectAction(w http.ResponseWriter, r *http.Request, approvalID, actorID string) {
	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	approval, err := das.workflowService.RejectWorkflowStep(r.Context(), services.RejectWorkflowStepRequest{
		ApprovalID: approvalID,
		RejecterID: actorID,
		Reason:     req.Reason,
	})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "rejection_failed",
			Message: err.Error(),
		})
		return
	}

	log.Printf("Workflow approval rejected via public API (ID: %s, Actor: %s, Reason: %s)", approvalID, actorID, req.Reason)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"approval": approval,
		"message":  "Workflow step rejected",
	})
}

func (das *DynamicAgentServer) handleAgentRuns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	_, err := das.authenticateWebhookRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "unauthorized",
			Message: err.Error(),
		})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/runs/")
	if path == "" || path == "/" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "Run ID required: /runs/{id}",
		})
		return
	}

	runID, err := strconv.ParseInt(path, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid run ID",
		})
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "method_not_allowed", Message: "Use GET /runs/{id}"})
		return
	}

	run, err := das.repos.AgentRuns.GetByIDWithDetails(r.Context(), runID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("Run %d not found", runID),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"run": run})
}

type StartWorkflowRequest struct {
	WorkflowID string                 `json:"workflow_id"`
	Input      map[string]interface{} `json:"input,omitempty"`
	Version    *int64                 `json:"version,omitempty"`
}

type WorkflowRunResponse struct {
	RunID      string `json:"run_id"`
	WorkflowID string `json:"workflow_id"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

func (das *DynamicAgentServer) handleWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if das.workflowService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "service_unavailable",
			Message: "Workflow service not configured",
		})
		return
	}

	_, err := das.authenticateWebhookRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "unauthorized",
			Message: err.Error(),
		})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/workflow-runs")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodPost && path == "":
		das.handleStartWorkflow(w, r)
	case r.Method == http.MethodGet && path != "":
		das.handleGetWorkflowRun(w, r, path)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "method_not_allowed",
			Message: "Use POST /workflow-runs to start, GET /workflow-runs/{id} for status",
		})
	}
}

func (das *DynamicAgentServer) handleStartWorkflow(w http.ResponseWriter, r *http.Request) {
	var req StartWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "Invalid JSON: " + err.Error(),
		})
		return
	}

	if req.WorkflowID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "bad_request",
			Message: "workflow_id is required",
		})
		return
	}

	inputJSON, _ := json.Marshal(req.Input)

	version := int64(0)
	if req.Version != nil {
		version = *req.Version
	}

	run, _, err := das.workflowService.StartRun(r.Context(), services.StartWorkflowRunRequest{
		WorkflowID: req.WorkflowID,
		Version:    version,
		Input:      inputJSON,
	})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "workflow_start_failed",
			Message: err.Error(),
		})
		return
	}

	log.Printf("Webhook: Started workflow (Run ID: %s, Workflow: %s)", run.RunID, req.WorkflowID)

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(WorkflowRunResponse{
		RunID:      run.RunID,
		WorkflowID: req.WorkflowID,
		Status:     string(run.Status),
		Message:    "Workflow run started",
	})
}

func (das *DynamicAgentServer) handleGetWorkflowRun(w http.ResponseWriter, r *http.Request, runID string) {
	run, err := das.workflowService.GetRun(r.Context(), runID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("Workflow run '%s' not found", runID),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"run": run})
}

func (das *DynamicAgentServer) authenticateWebhookRequest(r *http.Request) (*models.User, error) {
	if das.localMode {
		return &models.User{ID: 1, Username: "local", IsAdmin: true}, nil
	}

	// Extract bearer token
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("bearer token required")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == "" {
		return nil, fmt.Errorf("bearer token required")
	}

	// Check static webhook API key first (highest priority)
	if das.config != nil && das.config.Webhook.APIKey != "" {
		if token == das.config.Webhook.APIKey {
			log.Printf("Webhook auth: authenticated via static API key")
			return &models.User{ID: 1, Username: "webhook", IsAdmin: true}, nil
		}
		// If static key is configured, only that key is valid
		return nil, fmt.Errorf("invalid API key")
	}

	// Try local API key (sk-* prefix)
	if strings.HasPrefix(token, "sk-") {
		user, err := das.repos.Users.GetByAPIKey(token)
		if err == nil {
			log.Printf("Webhook auth: authenticated via user API key (user: %s)", user.Username)
			return user, nil
		}
	}

	// Try OAuth if enabled
	if das.oauthHandler != nil && das.oauthHandler.IsEnabled() {
		tokenInfo, err := das.oauthHandler.ValidateToken(token)
		if err == nil && tokenInfo.Active {
			log.Printf("Webhook auth: authenticated via OAuth (user: %s, org: %s)", tokenInfo.Email, tokenInfo.OrgID)
			return &models.User{
				ID:       0,
				Username: tokenInfo.Email,
				IsAdmin:  false,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid token")
}

// resolveAgent resolves an agent by name or ID
func (das *DynamicAgentServer) resolveAgent(name string, id int64) (*models.Agent, error) {
	// Prefer ID if provided
	if id > 0 {
		agent, err := das.repos.Agents.GetByID(id)
		if err != nil {
			return nil, fmt.Errorf("agent with ID %d not found", id)
		}
		return agent, nil
	}

	// Lookup by name within the environment
	env, err := das.repos.Environments.GetByName(das.environmentName)
	if err != nil {
		return nil, fmt.Errorf("environment '%s' not found", das.environmentName)
	}

	agents, err := das.repos.Agents.ListByEnvironment(env.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %v", err)
	}

	for _, agent := range agents {
		if agent.Name == name {
			return agent, nil
		}
	}

	return nil, fmt.Errorf("agent '%s' not found in environment '%s'", name, das.environmentName)
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// wrapWithOAuthDiscovery wraps an HTTP handler with MCP OAuth discovery support
// Returns 401 with WWW-Authenticate header when authentication is required but not provided
func wrapWithOAuthDiscovery(next http.Handler, cloudshipBaseURL string, repos *repositories.Repositories, oauthHandler *oauth.CloudShipOAuth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth check for health endpoint
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if request has valid authentication
		authHeader := r.Header.Get("Authorization")

		// No auth header - return 401 with OAuth discovery
		if authHeader == "" {
			returnOAuthChallenge(w, cloudshipBaseURL)
			return
		}

		// Must be Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			returnOAuthChallenge(w, cloudshipBaseURL)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			returnOAuthChallenge(w, cloudshipBaseURL)
			return
		}

		// Try to validate the token
		authenticated := false

		// Try local API key first (sk-* prefix)
		if strings.HasPrefix(token, "sk-") {
			_, err := repos.Users.GetByAPIKey(token)
			if err == nil {
				authenticated = true
			}
		}

		// Try CloudShip OAuth if not authenticated yet
		if !authenticated && oauthHandler != nil && oauthHandler.IsEnabled() {
			tokenInfo, err := oauthHandler.ValidateToken(token)
			if err == nil && tokenInfo.Active {
				authenticated = true
			}
		}

		// If still not authenticated, return 401
		if !authenticated {
			returnOAuthChallenge(w, cloudshipBaseURL)
			return
		}

		// Authenticated - pass to next handler
		next.ServeHTTP(w, r)
	})
}

// returnOAuthChallenge returns a 401 response with WWW-Authenticate header for MCP OAuth discovery
func returnOAuthChallenge(w http.ResponseWriter, cloudshipBaseURL string) {
	// RFC 9728: OAuth 2.0 Protected Resource Metadata
	// The resource_metadata parameter tells the client where to discover OAuth configuration
	resourceMetadataURL := fmt.Sprintf("%s/.well-known/oauth-protected-resource", cloudshipBaseURL)

	// Set WWW-Authenticate header per RFC 9728 Section 5.1
	wwwAuth := fmt.Sprintf(`Bearer resource_metadata="%s"`, resourceMetadataURL)
	w.Header().Set("WWW-Authenticate", wwwAuth)

	// CORS headers for MCP clients
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, MCP-Protocol-Version, Mcp-Session-Id")
	w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, Mcp-Session-Id")

	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error": "authentication_required", "message": "OAuth authentication required. See WWW-Authenticate header for discovery URL."}`))
}

// loadAgentsAsTools loads all agents from the specified environment as MCP tools
func (das *DynamicAgentServer) loadAgentsAsTools() error {
	// Get environment by name
	environment, err := das.repos.Environments.GetByName(das.environmentName)
	if err != nil {
		log.Printf("Failed to find environment '%s': %v", das.environmentName, err)
		return err
	}

	// Get agents from the specified environment
	agents, err := das.repos.Agents.ListByEnvironment(environment.ID)
	if err != nil {
		log.Printf("Failed to load agents from environment '%s': %v", das.environmentName, err)
		return err
	}

	log.Printf("🤖 Loading %d agents from environment '%s' as MCP tools", len(agents), das.environmentName)

	// Register each agent as an MCP tool
	for _, agent := range agents {
		toolName := "agent_" + agent.Name
		log.Printf("  📋 Registering agent '%s' as tool '%s'", agent.Name, toolName)

		// Create tool for this agent using the correct mcp package
		tool := mcp.NewTool(toolName,
			mcp.WithDescription("Execute agent: "+agent.Name),
			mcp.WithString("input", mcp.Required(), mcp.Description("Task or input to provide to the agent")),
			mcp.WithObject("variables", mcp.Description("Variables for dotprompt rendering (optional)")),
		)

		// Set handler for this agent tool
		agentID := agent.ID // capture for closure
		handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			// Extract parameters
			input := request.GetString("input", "")

			// Get variables if provided
			variables := make(map[string]interface{})
			if request.Params.Arguments != nil {
				if argsMap, ok := request.Params.Arguments.(map[string]interface{}); ok {
					if variablesArg, ok := argsMap["variables"]; ok {
						if varsMap, ok := variablesArg.(map[string]interface{}); ok {
							variables = varsMap
						}
					}
				}
			}

			// Execute the agent using the agent service
			result, err := das.agentService.ExecuteAgent(ctx, int64(agentID), input, variables)
			if err != nil {
				return mcp.NewToolResultError("Error executing agent: " + err.Error()), nil
			}

			return mcp.NewToolResultText(result.Content), nil
		}

		// Register the tool with the MCP server
		das.mcpServer.AddTool(tool, handler)
	}

	return nil
}

// Shutdown gracefully shuts down the dynamic MCP server
func (das *DynamicAgentServer) Shutdown(ctx context.Context) error {
	if das.httpServer != nil {
		return das.httpServer.Shutdown(ctx)
	}
	return nil
}

// createDynamicAgentAuthContextFunc creates an HTTPContextFunc that handles OAuth and API key auth
func createDynamicAgentAuthContextFunc(repos *repositories.Repositories, oauthHandler *oauth.CloudShipOAuth, localMode bool) server.HTTPContextFunc {
	return func(ctx context.Context, r *http.Request) context.Context {
		// In local mode, create a default admin user context
		if localMode {
			defaultUser := &models.User{
				ID:       1,
				Username: "local",
				IsAdmin:  true,
			}
			return context.WithValue(ctx, auth.UserKey, defaultUser)
		}

		// Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			return ctx
		}

		// Must be Bearer token
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return ctx
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return ctx
		}

		// Try local API key first (sk-* prefix)
		if strings.HasPrefix(token, "sk-") {
			user, err := repos.Users.GetByAPIKey(token)
			if err == nil {
				log.Printf("Dynamic Agent MCP auth: authenticated via local API key (user: %s)", user.Username)
				return context.WithValue(ctx, auth.UserKey, user)
			}
		}

		// Try CloudShip OAuth if enabled
		if oauthHandler != nil && oauthHandler.IsEnabled() {
			tokenInfo, err := oauthHandler.ValidateToken(token)
			if err == nil && tokenInfo.Active {
				// Create a virtual user from OAuth claims
				oauthUser := &models.User{
					ID:       0,
					Username: tokenInfo.Email,
					IsAdmin:  false,
				}
				log.Printf("Dynamic Agent MCP auth: authenticated via CloudShip OAuth (user: %s, org: %s)", tokenInfo.Email, tokenInfo.OrgID)

				ctx = context.WithValue(ctx, auth.UserKey, oauthUser)
				ctx = context.WithValue(ctx, "cloudship_user_id", tokenInfo.UserID)
				ctx = context.WithValue(ctx, "cloudship_org_id", tokenInfo.OrgID)
				ctx = context.WithValue(ctx, "cloudship_email", tokenInfo.Email)
				return ctx
			}
		}

		return ctx
	}
}
