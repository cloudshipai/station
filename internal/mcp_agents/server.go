package mcp_agents

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

// authRequiredKey is a context key to signal that auth is required but not provided
type authRequiredKey struct{}

// DynamicAgentServer manages a dynamic MCP server that serves database agents as individual tools
type DynamicAgentServer struct {
	repos           *repositories.Repositories
	agentService    services.AgentServiceInterface
	mcpServer       *server.MCPServer
	httpServer      *server.StreamableHTTPServer
	localMode       bool
	environmentName string
	config          *config.Config
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

// Start starts the dynamic MCP server on the specified port
func (das *DynamicAgentServer) Start(ctx context.Context, port int) error {
	// Create a new MCP server
	das.mcpServer = server.NewMCPServer(
		"Station Dynamic Agents",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Load agents as tools
	if err := das.loadAgentsAsTools(); err != nil {
		return err
	}

	// Create OAuth handler if enabled
	var oauthHandler *oauth.CloudShipOAuth
	if das.config != nil && das.config.CloudShip.OAuth.Enabled {
		oauthHandler = oauth.NewCloudShipOAuth(&das.config.CloudShip.OAuth)
		log.Printf("Dynamic Agent MCP OAuth authentication enabled")
	}

	// Create HTTP context function for authentication
	httpContextFunc := createDynamicAgentAuthContextFunc(das.repos, oauthHandler, das.localMode)

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

	// If OAuth is enabled and we're not in local mode, wrap with OAuth discovery middleware
	if !das.localMode && das.config != nil && das.config.CloudShip.OAuth.Enabled {
		// Wrap the MCP handler with OAuth discovery
		// StreamableHTTPServer implements http.Handler via ServeHTTP
		wrappedHandler := wrapWithOAuthDiscovery(das.httpServer, cloudshipBaseURL, das.repos, oauthHandler)

		log.Printf("Starting Dynamic Agent MCP server with OAuth discovery on %s", addr)
		log.Printf("  OAuth discovery URL: %s/.well-known/oauth-protected-resource", cloudshipBaseURL)

		httpServer := &http.Server{
			Addr:    addr,
			Handler: wrappedHandler,
		}
		return httpServer.ListenAndServe()
	}

	// Standard startup without OAuth middleware
	return das.httpServer.Start(addr)
}

// wrapWithOAuthDiscovery wraps an HTTP handler with MCP OAuth discovery support
// Returns 401 with WWW-Authenticate header when authentication is required but not provided
func wrapWithOAuthDiscovery(next http.Handler, cloudshipBaseURL string, repos *repositories.Repositories, oauthHandler *oauth.CloudShipOAuth) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Authenticated - pass to MCP handler
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
