package faker

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"station/internal/config"
	"station/internal/db"
	"station/internal/genkit/anthropic_oauth"
	"station/pkg/faker/session"
	"station/pkg/faker/toolcache"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/openai/openai-go/option"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// ToolCallHistory represents a single tool call and response for context
type ToolCallHistory struct {
	ToolName  string
	Arguments map[string]interface{}
	Response  string
	Timestamp string
}

// MCPFaker is an MCP server that proxies another MCP server and enriches responses
type MCPFaker struct {
	targetClient      *client.Client // Client to the real MCP server (nil in standalone mode)
	genkitApp         *genkit.Genkit
	stationConfig     *config.Config
	instruction       string
	debug             bool
	writeOperations   map[string]bool      // Tools classified as write operations
	safetyMode        bool                 // If true, intercept write operations
	callHistory       []ToolCallHistory    // Legacy: Message history for consistency (deprecated, use session)
	sessionManager    session.Manager      // Session-based state tracking
	session           *session.Session     // Current faker session
	toolSchemas       map[string]*mcp.Tool // Tool definitions for schema extraction
	toolsDiscovered   bool                 // Flag to track if tools have been discovered
	discoveredTools   []mcp.Tool           // Cached tools from target server
	toolDiscoveryLock sync.Mutex           // Lock for lazy tool discovery

	// Standalone mode fields
	standaloneMode bool        // If true, no target MCP server (AI-generated tools only)
	fakerID        string      // Unique identifier for tool caching in standalone mode
	toolCache      interface{} // Tool cache for standalone mode (toolcache.Cache)

	// Response cache for deterministic behavior within a session
	responseCache     map[string]*mcp.CallToolResult // Cache key: hash(toolName + args)
	responseCacheLock sync.RWMutex                   // Lock for thread-safe cache access
}

// Global log file for faker debugging (since stderr isn't captured by parent)
var fakerLogFile *os.File

func initFakerLogFile() {
	// Create log file with absolute path
	logPath := "/home/epuerta/projects/hack/station/dev-workspace/faker-debug.log"

	var err error
	fakerLogFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Log error to stderr and try fallback
		fmt.Fprintf(os.Stderr, "[FAKER] ❌ Failed to open log file at %s: %v\n", logPath, err)
		return
	}
	fmt.Fprintf(fakerLogFile, "\n\n=== New Faker Session Started at %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(fakerLogFile, "[FAKER] ✅ Log file initialized successfully\n")
	fakerLogFile.Sync() // Flush to disk immediately
}

func logFaker(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	// Log to stderr
	fmt.Fprint(os.Stderr, msg)
	// Also log to file
	if fakerLogFile != nil {
		fmt.Fprint(fakerLogFile, msg)
		fakerLogFile.Sync() // Flush immediately so we don't lose logs if process hangs
	}
}

// generateCacheKey creates a deterministic cache key from tool name and arguments
func generateCacheKey(toolName string, args map[string]interface{}) string {
	// Serialize arguments to JSON for consistent hashing
	argsJSON, err := json.Marshal(args)
	if err != nil {
		// Fallback to toolName only if serialization fails
		return toolName
	}
	// Hash the arguments for consistent cache key
	hash := sha256.Sum256(argsJSON)
	// Return toolName:argsHash for readability in logs
	return fmt.Sprintf("%s:%x", toolName, hash[:8])
}

// cacheAndReturn stores the result in cache and returns it
func (f *MCPFaker) cacheAndReturn(cacheKey string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
	f.responseCacheLock.Lock()
	f.responseCache[cacheKey] = result
	f.responseCacheLock.Unlock()

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] 💾 Cached response for key: %s (cache size: %d)\n", cacheKey, len(f.responseCache))
	}

	return result, nil
}

// NewMCPFaker creates a new MCP faker server
func NewMCPFaker(targetCmd string, targetArgs []string, targetEnv map[string]string, instruction string, debug bool) (*MCPFaker, error) {
	// Initialize log file for debugging
	initFakerLogFile()

	ctx := context.Background()

	logFaker("[FAKER] 🚀 NewMCPFaker starting at %s\n", time.Now().Format("15:04:05"))

	if debug {
		if deadline, ok := ctx.Deadline(); ok {
			logFaker("[FAKER DEBUG] NewMCPFaker context deadline: %v (timeout in %v)\n", deadline, time.Until(deadline))
		} else {
			logFaker("[FAKER DEBUG] NewMCPFaker context has NO deadline (infinite timeout) ✓\n")
		}
	}

	// Initialize viper to discover config file (same as main CLI does)
	// This ensures faker uses workspace config.yaml if present
	if err := config.InitViper(""); err != nil {
		if debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to initialize viper: %v\n", err)
		}
	}

	// Load Station config
	stationConfig, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// ALWAYS log which database we're using (debug critical config issue)
	fmt.Fprintf(os.Stderr, "[FAKER-PROXY] Using database: %s\n", stationConfig.DatabaseURL)
	fmt.Fprintf(os.Stderr, "[FAKER-PROXY] STATION_DATABASE env: %s\n", os.Getenv("STATION_DATABASE"))

	// Initialize OpenTelemetry for faker span export
	// Check if OTEL is enabled via environment variables
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint != "" {
		// Initialize OTEL with same setup as main Station process
		if err := initializeFakerOTEL(ctx, otelEndpoint, debug); err != nil {
			if debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to initialize OTEL: %v\n", err)
			}
			// Don't fail faker if OTEL setup fails - just warn
		} else if debug {
			fmt.Fprintf(os.Stderr, "[FAKER] ✅ OpenTelemetry initialized - exporting to %s\n", otelEndpoint)
		}
	}

	// Only initialize GenKit if AI enrichment is requested
	var app *genkit.Genkit
	if instruction != "" {
		// Disable GenKit reflection server to prevent port conflicts
		if os.Getenv("GENKIT_ENV") == "" {
			os.Setenv("GENKIT_ENV", "prod")
		}

		// Enable telemetry for faker traces (using station's OTEL endpoint)
		// Note: Faker runs as subprocess, will inherit parent's OTEL config

		// Create a brand new GenKit app for the faker
		if debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Initializing fresh GenKit with provider: %s, model: %s\n",
				stationConfig.AIProvider, stationConfig.AIModel)
		}

		// Initialize based on provider
		switch strings.ToLower(stationConfig.AIProvider) {
		case "openai":
			// Create HTTP client with generous timeout for AI generation
			httpClient := &http.Client{
				Timeout: 60 * time.Second, // 60s timeout for OpenAI API calls
			}

			var opts []option.RequestOption
			opts = append(opts, option.WithHTTPClient(httpClient))
			if stationConfig.AIBaseURL != "" {
				opts = append(opts, option.WithBaseURL(stationConfig.AIBaseURL))
			}

			plugin := &openai.OpenAI{
				APIKey: stationConfig.AIAPIKey,
				Opts:   opts,
			}

			app = genkit.Init(ctx, genkit.WithPlugins(plugin))

		case "googlegenai", "gemini":
			// Use environment variable for API key
			plugin := &googlegenai.GoogleAI{}
			app = genkit.Init(ctx, genkit.WithPlugins(plugin))

		case "anthropic":
			if stationConfig.AIAuthType == "oauth" && stationConfig.AIOAuthToken != "" {
				if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Using Anthropic OAuth plugin\n")
				}
				oauthPlugin := &anthropic_oauth.AnthropicOAuth{
					OAuthToken: stationConfig.AIOAuthToken,
				}
				app = genkit.Init(ctx, genkit.WithPlugins(oauthPlugin))
			} else if stationConfig.AIAPIKey != "" {
				if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Using Anthropic API key\n")
				}
				oauthPlugin := &anthropic_oauth.AnthropicOAuth{
					APIKey: stationConfig.AIAPIKey,
				}
				app = genkit.Init(ctx, genkit.WithPlugins(oauthPlugin))
			} else {
				if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] No Anthropic credentials, falling back to OpenAI\n")
				}
				openaiKey := os.Getenv("OPENAI_API_KEY")
				if openaiKey == "" {
					return nil, fmt.Errorf("faker requires OPENAI_API_KEY when Anthropic has no credentials")
				}
				httpClient := &http.Client{Timeout: 60 * time.Second}
				plugin := &openai.OpenAI{
					APIKey: openaiKey,
					Opts:   []option.RequestOption{option.WithHTTPClient(httpClient)},
				}
				app = genkit.Init(ctx, genkit.WithPlugins(plugin))
				stationConfig.AIProvider = "openai"
				stationConfig.AIModel = "gpt-4o-mini"
				stationConfig.AIAPIKey = openaiKey
			}

		default:
			return nil, fmt.Errorf("unsupported AI provider: %s (supported: openai, gemini)", stationConfig.AIProvider)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[FAKER] GenKit initialized successfully\n")
		}
	}

	// Create client to target MCP server
	// Convert env map to []string format
	envSlice := make([]string, 0, len(targetEnv))
	for k, v := range targetEnv {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Creating MCP client to target: %s %v\n", targetCmd, targetArgs)
	}

	targetClient, err := client.NewStdioMCPClient(targetCmd, envSlice, targetArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to create target client: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Target client initialized successfully\n")
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Initializing target client...\n")
	}

	// Initialize target client with timeout to prevent hangs during stn sync
	// CRITICAL FIX: If target MCP server hangs during initialization, faker will fail fast
	initCtx, initCancel := context.WithTimeout(ctx, 15*time.Second)
	defer initCancel()

	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "faker-mcp-client",
				Version: "1.0.0",
			},
		},
	}
	if _, err := targetClient.Initialize(initCtx, initReq); err != nil {
		return nil, fmt.Errorf("failed to initialize target client (timeout after 15s): %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Target client initialized successfully\n")
	}

	// Initialize session management (only if instruction provided for AI enrichment)
	// CRITICAL FIX: Use timeout and graceful degradation to prevent hangs during stn sync
	// when multiple fakers try to connect to the database simultaneously
	var sessionMgr session.Manager
	var sess *session.Session
	if instruction != "" {
		// Try to open database with timeout - if it fails, faker still works in passthrough mode
		// INCREASED to 30s to handle concurrent faker startups during stn sync
		dbCtx, dbCancel := context.WithTimeout(ctx, 30*time.Second)
		defer dbCancel()

		dbChan := make(chan error, 1)
		var database *db.DB

		go func() {
			var err error
			database, err = db.New(stationConfig.DatabaseURL)
			dbChan <- err
		}()

		select {
		case err := <-dbChan:
			if err != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to open database (will work without session tracking): %v\n", err)
				}
				// Continue without session management - faker will work in passthrough mode
			} else {
				// Database connection successful - initialize session
				sessionMgr = session.NewManager(database.Conn(), debug)

				// Create new session for this faker instance
				sess, err = sessionMgr.CreateSession(ctx, instruction)
				if err != nil {
					if debug {
						fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to create session (will work without session tracking): %v\n", err)
					}
					// Continue without session - faker will still work
				} else if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Created session %s\n", sess.ID)
				}
			}
		case <-dbCtx.Done():
			if debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Database connection timed out after 5s (will work without session tracking)\n")
			}
			// Continue without session management - faker will work in passthrough mode
		}
	}

	return &MCPFaker{
		targetClient:      targetClient,
		genkitApp:         app,
		stationConfig:     stationConfig,
		instruction:       instruction,
		debug:             debug,
		writeOperations:   make(map[string]bool),
		safetyMode:        true, // Always enable safety mode by default
		callHistory:       make([]ToolCallHistory, 0),
		sessionManager:    sessionMgr,
		session:           sess,
		toolSchemas:       make(map[string]*mcp.Tool),
		standaloneMode:    false,
		responseCache:     make(map[string]*mcp.CallToolResult),
		responseCacheLock: sync.RWMutex{},
	}, nil
}

// NewStandaloneFaker creates a new standalone faker that generates tools via AI
// without connecting to a real MCP server. Tools are cached in the database.
func NewStandaloneFaker(fakerID string, instruction string, toolCache interface{}, debug bool) (*MCPFaker, error) {
	initFakerLogFile()
	ctx := context.Background()

	logFaker("[FAKER] 🚀 NewStandaloneFaker starting for ID: %s\n", fakerID)

	if fakerID == "" {
		return nil, fmt.Errorf("faker ID is required for standalone mode")
	}

	if instruction == "" {
		return nil, fmt.Errorf("AI instruction is required for standalone mode")
	}

	// Initialize viper to discover config file (same as main CLI does)
	// This ensures faker uses workspace config.yaml if present
	if err := config.InitViper(""); err != nil && debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to initialize viper: %v\n", err)
	}

	// Load Station config
	stationConfig, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load station config: %w", err)
	}

	// ALWAYS log which database we're using (debug critical config issue)
	fmt.Fprintf(os.Stderr, "[FAKER-STANDALONE] Using database: %s\n", stationConfig.DatabaseURL)
	fmt.Fprintf(os.Stderr, "[FAKER-STANDALONE] STATION_DATABASE env: %s\n", os.Getenv("STATION_DATABASE"))

	// Set GenKit to production mode to prevent reflection server
	if os.Getenv("GENKIT_ENV") == "" {
		os.Setenv("GENKIT_ENV", "prod")
	}

	// Initialize GenKit for AI tool generation
	var app *genkit.Genkit

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Initializing GenKit for standalone mode with provider: %s\n",
			stationConfig.AIProvider)
	}

	// Initialize the AI provider
	switch strings.ToLower(stationConfig.AIProvider) {
	case "openai":
		httpClient := &http.Client{
			Timeout: 120 * time.Second,
		}

		var opts []option.RequestOption
		opts = append(opts, option.WithHTTPClient(httpClient))
		if stationConfig.AIBaseURL != "" {
			opts = append(opts, option.WithBaseURL(stationConfig.AIBaseURL))
		}

		plugin := &openai.OpenAI{
			APIKey: stationConfig.AIAPIKey,
			Opts:   opts,
		}

		app = genkit.Init(ctx, genkit.WithPlugins(plugin))

	case "googlegenai", "gemini":
		plugin := &googlegenai.GoogleAI{}
		app = genkit.Init(ctx, genkit.WithPlugins(plugin))

	case "anthropic":
		if stationConfig.AIAuthType == "oauth" && stationConfig.AIOAuthToken != "" {
			if debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Using Anthropic OAuth plugin\n")
			}
			oauthPlugin := &anthropic_oauth.AnthropicOAuth{
				OAuthToken: stationConfig.AIOAuthToken,
			}
			app = genkit.Init(ctx, genkit.WithPlugins(oauthPlugin))
		} else if stationConfig.AIAPIKey != "" {
			if debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Using Anthropic API key\n")
			}
			oauthPlugin := &anthropic_oauth.AnthropicOAuth{
				APIKey: stationConfig.AIAPIKey,
			}
			app = genkit.Init(ctx, genkit.WithPlugins(oauthPlugin))
		} else {
			if debug {
				fmt.Fprintf(os.Stderr, "[FAKER] No Anthropic credentials, falling back to OpenAI\n")
			}
			openaiKey := os.Getenv("OPENAI_API_KEY")
			if openaiKey == "" {
				return nil, fmt.Errorf("faker requires OPENAI_API_KEY when Anthropic has no credentials")
			}
			httpClient := &http.Client{Timeout: 120 * time.Second}
			plugin := &openai.OpenAI{
				APIKey: openaiKey,
				Opts:   []option.RequestOption{option.WithHTTPClient(httpClient)},
			}
			app = genkit.Init(ctx, genkit.WithPlugins(plugin))
			stationConfig.AIProvider = "openai"
			stationConfig.AIModel = "gpt-4o-mini"
			stationConfig.AIAPIKey = openaiKey
		}

	default:
		return nil, fmt.Errorf("unsupported AI provider: %s (use 'openai' or 'gemini')", stationConfig.AIProvider)
	}

	if app == nil {
		return nil, fmt.Errorf("failed to initialize GenKit app")
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] GenKit initialized successfully\n")
	}

	// Initialize session manager only (session creation deferred to Serve() for cache-based reuse)
	// This allows Serve() to check the tool cache for an existing session ID before creating a new one
	var sessionMgr session.Manager
	if instruction != "" {
		dbCtx, dbCancel := context.WithTimeout(ctx, 30*time.Second)
		defer dbCancel()

		dbChan := make(chan error, 1)
		var database *db.DB

		go func() {
			var err error
			database, err = db.New(stationConfig.DatabaseURL)
			dbChan <- err
		}()

		select {
		case err := <-dbChan:
			if err != nil {
				if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to open database (will work without session tracking): %v\n", err)
				}
			} else {
				sessionMgr = session.NewManager(database.Conn(), debug)
				if debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Session manager initialized (session will be created/loaded in Serve)\n")
				}
			}
		case <-dbCtx.Done():
			if debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Database connection timed out\n")
			}
		}
	}

	return &MCPFaker{
		targetClient:      nil, // No target in standalone mode
		genkitApp:         app,
		stationConfig:     stationConfig,
		instruction:       instruction,
		debug:             debug,
		writeOperations:   make(map[string]bool),
		safetyMode:        true,
		callHistory:       make([]ToolCallHistory, 0),
		sessionManager:    sessionMgr,
		session:           nil, // Session created/loaded in Serve() after checking cache
		toolSchemas:       make(map[string]*mcp.Tool),
		standaloneMode:    true,
		fakerID:           fakerID,
		toolCache:         toolCache,
		responseCache:     make(map[string]*mcp.CallToolResult),
		responseCacheLock: sync.RWMutex{},
	}, nil
}

// Serve starts the faker as an MCP server on stdio
func (f *MCPFaker) Serve() error {
	ctx := context.Background()

	logFaker("[FAKER] 🚀 Serve() called at %s\n", time.Now().Format("15:04:05"))
	logFaker("[FAKER] Standalone mode: %v, Debug: %v, FakerID: %s\n", f.standaloneMode, f.debug, f.fakerID)

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Starting MCP server...\n")
	}

	// Create MCP server
	logFaker("[FAKER] Creating MCP server...\n")
	mcpServer := server.NewMCPServer("faker-mcp-server", "1.0.0")
	logFaker("[FAKER] ✅ MCP server created\n")

	var tools []mcp.Tool
	var err error

	if f.standaloneMode {
		// Standalone mode: Generate tools via AI or load from cache
		logFaker("[FAKER] 🔍 Standalone mode - checking tool cache for ID: %s\n", f.fakerID)
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Standalone mode - checking tool cache for ID: %s\n", f.fakerID)
		}

		// Try to load from cache first
		if f.toolCache != nil {
			logFaker("[FAKER] Tool cache exists, attempting type assertion...\n")
			if cache, ok := f.toolCache.(toolcache.Cache); ok {
				logFaker("[FAKER] Cache type assertion successful, calling GetTools...\n")
				cachedTools, cachedSessionID, cacheErr := cache.GetTools(ctx, f.fakerID)
				logFaker("[FAKER] GetTools returned: %d tools, sessionID: %s, error: %v\n", len(cachedTools), cachedSessionID, cacheErr)
				if cacheErr == nil && len(cachedTools) > 0 {
					if f.debug {
						fmt.Fprintf(os.Stderr, "[FAKER] Loaded %d tools from cache (session: %s)\n", len(cachedTools), cachedSessionID)
					}
					logFaker("[FAKER] ✅ Using %d cached tools\n", len(cachedTools))
					tools = cachedTools

					// CRITICAL: Load existing session if we have one cached
					// This enables session persistence across subprocess invocations
					if cachedSessionID != "" && f.sessionManager != nil {
						existingSession, sessErr := f.sessionManager.GetSession(ctx, cachedSessionID)
						if sessErr == nil && existingSession != nil {
							f.session = existingSession
							logFaker("[FAKER] ✅ Loaded existing session: %s (created: %s)\n", cachedSessionID, existingSession.CreatedAt)
							if f.debug {
								fmt.Fprintf(os.Stderr, "[FAKER] Loaded existing session: %s\n", cachedSessionID)
							}
						} else {
							// Session doesn't exist (may have been cleared) - create a new one
							logFaker("[FAKER] ⚠️  Could not load cached session %s: %v, creating new session\n", cachedSessionID, sessErr)
							newSession, newSessErr := f.sessionManager.CreateSession(ctx, f.instruction)
							if newSessErr != nil {
								logFaker("[FAKER] ⚠️  Failed to create new session: %v\n", newSessErr)
							} else {
								f.session = newSession
								logFaker("[FAKER] ✅ Created new session: %s\n", newSession.ID)
								if f.debug {
									fmt.Fprintf(os.Stderr, "[FAKER] Created new session: %s\n", newSession.ID)
								}
							}
						}
					} else if f.sessionManager != nil && f.session == nil {
						// No cached session ID, create a fresh session
						newSession, newSessErr := f.sessionManager.CreateSession(ctx, f.instruction)
						if newSessErr != nil {
							logFaker("[FAKER] ⚠️  Failed to create session: %v\n", newSessErr)
						} else {
							f.session = newSession
							logFaker("[FAKER] ✅ Created new session (no cache): %s\n", newSession.ID)
							if f.debug {
								fmt.Fprintf(os.Stderr, "[FAKER] Created new session: %s\n", newSession.ID)
							}
						}
					}
				} else {
					logFaker("[FAKER] Cache miss or error (%v), generating tools with AI...\n", cacheErr)
					if f.debug {
						fmt.Fprintf(os.Stderr, "[FAKER] Cache miss or error (%v), generating tools with AI...\n", cacheErr)
					}

					// CRITICAL: Create NEW session for this faker (cache miss = first time)
					// This session will be persisted in cache for future subprocess reuse
					if f.sessionManager != nil && f.session == nil {
						newSession, sessErr := f.sessionManager.CreateSession(ctx, f.instruction)
						if sessErr != nil {
							logFaker("[FAKER] ⚠️  Failed to create session: %v\n", sessErr)
						} else {
							f.session = newSession
							logFaker("[FAKER] ✅ Created NEW session: %s\n", newSession.ID)
							if f.debug {
								fmt.Fprintf(os.Stderr, "[FAKER] Created new session: %s\n", newSession.ID)
							}
						}
					}

					// Generate tools with AI
					logFaker("[FAKER] 🤖 Calling generateToolsWithAI...\n")
					tools, err = f.generateToolsWithAI(ctx)
					logFaker("[FAKER] generateToolsWithAI returned: %d tools, error: %v\n", len(tools), err)
					if err != nil {
						return fmt.Errorf("failed to generate tools with AI: %w", err)
					}

					// Cache the generated tools with session ID for persistence
					sessionID := ""
					if f.session != nil {
						sessionID = f.session.ID
					}
					logFaker("[FAKER] 💾 Caching %d generated tools with session %s...\n", len(tools), sessionID)
					if cacheErr := cache.SetTools(ctx, f.fakerID, tools, sessionID); cacheErr != nil {
						logFaker("[FAKER] ⚠️  Failed to cache tools: %v\n", cacheErr)
						if f.debug {
							fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to cache tools: %v\n", cacheErr)
						}
					} else {
						logFaker("[FAKER] ✅ Cached %d tools for future use (session: %s)\n", len(tools), sessionID)
						if f.debug {
							fmt.Fprintf(os.Stderr, "[FAKER] Cached %d tools for future use\n", len(tools))
						}
					}
				}
			} else {
				// Type assertion failed, generate without cache
				logFaker("[FAKER] ⚠️  Tool cache type assertion failed, generating with AI...\n")
				if f.debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Tool cache type assertion failed, generating with AI...\n")
				}
				tools, err = f.generateToolsWithAI(ctx)
				if err != nil {
					return fmt.Errorf("failed to generate tools with AI: %w", err)
				}
			}
		} else {
			// No cache available, generate with AI
			logFaker("[FAKER] ⚠️  No tool cache available, generating with AI...\n")
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] No tool cache available, generating with AI...\n")
			}
			tools, err = f.generateToolsWithAI(ctx)
			if err != nil {
				return fmt.Errorf("failed to generate tools with AI: %w", err)
			}
		}
	} else {
		// Proxy mode: List tools from target MCP server
		logFaker("[FAKER] 🔄 Proxy mode - listing tools from target...\n")
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Proxy mode - listing tools from target...\n")
		}

		// CRITICAL FIX: Add timeout for tool discovery to prevent infinite hangs during stn sync
		// If target MCP server is slow/hanging, faker will fail fast instead of blocking
		discCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// Get all tools from target server with timeout
		logFaker("[FAKER] Calling targetClient.ListTools with 10s timeout...\n")
		toolsResult, err := f.targetClient.ListTools(discCtx, mcp.ListToolsRequest{})
		logFaker("[FAKER] ListTools returned: error=%v\n", err)
		if err != nil {
			return fmt.Errorf("failed to list tools from target (timeout after 10s): %w", err)
		}

		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Found %d tools from target\n", len(toolsResult.Tools))
		}
		logFaker("[FAKER] ✅ Found %d tools from target\n", len(toolsResult.Tools))

		tools = toolsResult.Tools

		// Classify tools as read/write operations using AI (proxy mode only)
		if f.safetyMode && f.genkitApp != nil {
			logFaker("[FAKER] Classifying tools for write operation detection...\n")
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Classifying tools for write operation detection...\n")
			}
			if err := f.classifyTools(ctx, tools); err != nil {
				if f.debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Warning: Tool classification failed: %v\n", err)
				}
			} else {
				f.displayToolClassification()
			}
		}
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Registering %d tools...\n", len(tools))
	}

	// Register each tool with a proxy handler and store schemas
	for _, tool := range tools {
		// Store tool schema for response shape consistency
		toolCopy := tool // Capture tool in closure
		f.toolSchemas[tool.Name] = &toolCopy

		mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return f.handleToolCall(ctx, request)
		})
	}

	logFaker("[FAKER] 🚀 Starting stdio server...\n")

	// Serve on stdio - this blocks until shutdown
	err = server.ServeStdio(mcpServer)

	logFaker("[FAKER] 🛑 ServeStdio returned: err=%v at %s\n", err, time.Now().Format("15:04:05"))

	// Gracefully shutdown OTEL to flush remaining spans
	if err := shutdownFakerOTEL(ctx); err != nil {
		logFaker("[FAKER] ⚠️  OTEL shutdown error: %v\n", err)
	}

	logFaker("[FAKER] 👋 Faker shutting down\n")
	return err
}

// handleToolCall proxies a tool call to the target, enriches the response, and returns it
func (f *MCPFaker) handleToolCall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// CRITICAL DEBUG: Always log to stderr to confirm faker is invoked
	fmt.Fprintf(os.Stderr, "\n🎯 [FAKER] ===== TOOL CALL INTERCEPTED: %s =====\n", request.Params.Name)

	// Start OpenTelemetry span with station.faker label
	tracer := otel.Tracer("station.faker")
	ctx, span := tracer.Start(ctx, fmt.Sprintf("faker.%s", request.Params.Name),
		trace.WithAttributes(
			attribute.String("faker.tool_name", request.Params.Name),
			attribute.String("faker.ai_instruction", f.instruction),
			attribute.Bool("faker.safety_mode", f.safetyMode),
			attribute.Bool("faker.is_write_operation", f.writeOperations[request.Params.Name]),
		),
	)
	defer span.End()

	fmt.Fprintf(os.Stderr, "🔭 [FAKER] OpenTelemetry span created for: %s\n", request.Params.Name)

	// Add session ID if available
	if f.session != nil {
		span.SetAttributes(attribute.String("faker.session_id", f.session.ID))
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Handling tool call: %s\n", request.Params.Name)
		fmt.Fprintf(os.Stderr, "[FAKER DEBUG] safetyMode=%v, isWriteOp=%v, hasSession=%v\n",
			f.safetyMode, f.writeOperations[request.Params.Name], f.session != nil)

		// DEBUG: Check incoming context deadline
		if deadline, ok := ctx.Deadline(); ok {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] handleToolCall INCOMING context deadline: %v (timeout in %v) ⚠️\n", deadline, time.Until(deadline))
		} else {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] handleToolCall INCOMING context has NO deadline ✓\n")
		}
	}

	args, _ := request.Params.Arguments.(map[string]interface{})

	// Check response cache for deterministic behavior within session
	cacheKey := generateCacheKey(request.Params.Name, args)
	f.responseCacheLock.RLock()
	if cachedResult, exists := f.responseCache[cacheKey]; exists {
		f.responseCacheLock.RUnlock()
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] 💾 Cache HIT for %s (key: %s)\n", request.Params.Name, cacheKey)
		}
		span.SetAttributes(
			attribute.Bool("faker.cache_hit", true),
			attribute.String("faker.cache_key", cacheKey),
		)
		return cachedResult, nil
	}
	f.responseCacheLock.RUnlock()

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] 🔄 Cache MISS for %s (key: %s) - generating new response\n", request.Params.Name, cacheKey)
	}
	span.SetAttributes(
		attribute.Bool("faker.cache_hit", false),
		attribute.String("faker.cache_key", cacheKey),
	)

	// Check if this is a write operation and intercept it
	if f.safetyMode && f.writeOperations[request.Params.Name] {
		span.SetAttributes(
			attribute.Bool("faker.intercepted_write", true),
			attribute.Bool("faker.real_mcp_used", false),
		)

		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] ⚠️  INTERCEPTED write operation: %s (returning mock success)\n", request.Params.Name)
		}

		mockResult, err := f.createMockSuccessResponse(request.Params.Name, args)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create mock response")
			return nil, err
		}

		// Record write operation in session for state tracking
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] About to record write event: sessionMgr=%v, session=%v\n",
				f.sessionManager != nil, f.session != nil)
		}
		if err := f.recordToolEvent(ctx, request.Params.Name, args, mockResult, "write"); err != nil {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to record write event: %v\n", err)
			}
		} else if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] ✅ Write event recorded successfully\n")
		}

		// Legacy: Also record in callHistory for backward compatibility
		f.recordToolCall(request.Params.Name, args, mockResult)

		// Cache and return
		return f.cacheAndReturn(cacheKey, mockResult)
	}

	// Read operation - check if we should synthesize based on write history
	if f.shouldSynthesizeRead(ctx) {
		span.SetAttributes(attribute.Bool("faker.synthesized_response", true))

		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Read operation with write history - synthesizing response based on accumulated state\n")
		}

		synthesizedResult, err := f.synthesizeReadResponse(ctx, request.Params.Name, args)
		if err != nil {
			span.AddEvent("synthesis_failed", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Synthesis failed: %v, falling back to real tool\n", err)
			}
			// Fall through to real tool call
		} else {
			span.SetAttributes(attribute.Bool("faker.real_mcp_used", false))
			// Record synthesized read in session
			if err := f.recordToolEvent(ctx, request.Params.Name, args, synthesizedResult, "read"); err != nil {
				if f.debug {
					fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to record read event: %v\n", err)
				}
			}
			// Cache and return
			return f.cacheAndReturn(cacheKey, synthesizedResult)
		}
	}

	// STANDALONE MODE: No target server, generate simulated response immediately
	if f.standaloneMode {
		span.SetAttributes(
			attribute.Bool("faker.standalone_mode", true),
			attribute.Bool("faker.real_mcp_used", false),
		)

		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Standalone mode - generating simulated response for %s\n", request.Params.Name)
		}

		// Generate simulated response using AI instruction
		simulatedResult, simErr := f.generateSimulatedResponse(ctx, request.Params.Name, args, nil)
		if simErr != nil {
			span.RecordError(simErr)
			span.SetStatus(codes.Error, "standalone simulation failed")
			return nil, fmt.Errorf("standalone mode simulation failed: %w", simErr)
		}

		// Record simulated read in session
		if recErr := f.recordToolEvent(ctx, request.Params.Name, args, simulatedResult, "read"); recErr != nil {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to record read event: %v\n", recErr)
			}
		}

		// Record in callHistory for backward compatibility
		f.recordToolCall(request.Params.Name, args, simulatedResult)

		// Cache and return
		return f.cacheAndReturn(cacheKey, simulatedResult)
	}

	// Call the real target server for read operations (proxy mode only)
	result, err := f.targetClient.CallTool(ctx, request)

	// ALWAYS log target call result for debugging (not just in debug mode)
	fmt.Fprintf(os.Stderr, "[FAKER] Target call result: err=%v, result_exists=%v, has_genkit=%v, has_instruction=%v\n",
		err != nil, result != nil, f.genkitApp != nil, f.instruction != "")
	if result != nil {
		fmt.Fprintf(os.Stderr, "[FAKER] Result content length: %d, IsError=%v\n", len(result.Content), result.IsError)
		if len(result.Content) > 0 {
			if tc, ok := result.Content[0].(*mcp.TextContent); ok {
				preview := tc.Text
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				fmt.Fprintf(os.Stderr, "[FAKER] Content preview: %s\n", preview)
			}
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAKER] Error details: %v\n", err)
	}

	// If AI enrichment is enabled and the real call failed, generate simulated data instead
	if err != nil && f.genkitApp != nil && f.instruction != "" {
		span.RecordError(err)
		span.AddEvent("target_call_failed_generating_simulated_data", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		span.SetAttributes(
			attribute.Bool("faker.real_mcp_used", false),
			attribute.Bool("faker.synthesized_response", true),
		)

		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] ⚠️  Target call failed: %v\n", err)
			fmt.Fprintf(os.Stderr, "[FAKER] ✨ AI enrichment enabled - generating simulated data based on instruction\n")
		}

		// Generate simulated response using AI instruction
		simulatedResult, simErr := f.generateSimulatedResponse(ctx, request.Params.Name, args, err)
		if simErr != nil {
			span.RecordError(simErr)
			span.SetStatus(codes.Error, "simulation also failed")
			return nil, fmt.Errorf("target call failed and simulation failed: target_err=%w, sim_err=%v", err, simErr)
		}

		// Record simulated read in session
		if recErr := f.recordToolEvent(ctx, request.Params.Name, args, simulatedResult, "read"); recErr != nil {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to record simulated read event: %v\n", recErr)
			}
		}

		// Cache and return
		return f.cacheAndReturn(cacheKey, simulatedResult)
	}

	// If no AI enrichment or real call succeeded, handle normally
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "target tool call failed")
		return nil, fmt.Errorf("target tool call failed: %w", err)
	}

	// Mark span as using real MCP server
	span.SetAttributes(attribute.Bool("faker.real_mcp_used", true))

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Target returned result\n")
	}

	// Enrich the result
	enrichedResult, err := f.enrichToolResult(ctx, request.Params.Name, args, result)
	if err != nil {
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Enrichment failed: %v, returning original\n", err)
		}
		enrichedResult = result // Use original if enrichment fails
	}

	// Record read operation in session
	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER DEBUG] About to record read event: sessionMgr=%v, session=%v\n",
			f.sessionManager != nil, f.session != nil)
	}
	if err := f.recordToolEvent(ctx, request.Params.Name, args, enrichedResult, "read"); err != nil {
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Warning: Failed to record read event: %v\n", err)
		}
	} else if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER DEBUG] ✅ Read event recorded successfully\n")
	}

	// Legacy: Also record in callHistory for backward compatibility
	f.recordToolCall(request.Params.Name, args, enrichedResult)

	// Cache and return
	return f.cacheAndReturn(cacheKey, enrichedResult)
}

// generateSimulatedResponse creates a completely simulated response when the real MCP call fails
func (f *MCPFaker) generateSimulatedResponse(ctx context.Context, toolName string, args map[string]interface{}, originalError error) (*mcp.CallToolResult, error) {
	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Generating fully simulated response for %s\n", toolName)
	}

	// Get tool schema for response structure
	toolSchema, hasSchema := f.toolSchemas[toolName]
	var schemaInfo string
	if hasSchema {
		schemaJSON, _ := json.Marshal(toolSchema.InputSchema)
		schemaInfo = fmt.Sprintf("\nTool schema: %s", string(schemaJSON))
	}

	// Get session history for consistency (LIMIT TO LAST 3 EVENTS FOR PERFORMANCE)
	var sessionHistoryPrompt string
	if f.session != nil && f.sessionManager != nil {
		events, err := f.sessionManager.GetAllEvents(ctx, f.session.ID)
		// ALWAYS log session history status (debug critical issue)
		fmt.Fprintf(os.Stderr, "[FAKER-SESSION] Session ID: %s, Events fetched: %d, Error: %v\n",
			f.session.ID, len(events), err)
		if err == nil && len(events) > 0 {
			// PERFORMANCE FIX: Only use last 3 events to keep prompt size manageable
			// Tests show: 13 events = 137KB, 3 events = 31KB (77% reduction)
			const maxHistoryEvents = 3
			limitedEvents := events
			if len(events) > maxHistoryEvents {
				limitedEvents = events[len(events)-maxHistoryEvents:]
				fmt.Fprintf(os.Stderr, "[FAKER-SESSION] 📉 Limited history from %d to %d events (performance optimization)\n",
					len(events), maxHistoryEvents)
			}

			builder := session.NewHistoryBuilder(limitedEvents)
			sessionHistoryPrompt = "\n\n=== SESSION HISTORY (CRITICAL - READ CAREFULLY) ===\n\n" +
				builder.BuildAllEventsPrompt() +
				"\n\n=== CONSISTENCY RULES (MANDATORY) ===\n" +
				"1. If workspace 'production' had 15 resources in a previous response, it MUST have EXACTLY 15 resources in this response\n" +
				"2. Use THE EXACT SAME resource IDs, names, and values from previous responses\n" +
				"3. Do NOT invent new data - copy from the history above\n" +
				"4. If you cannot maintain consistency, return an error instead of contradicting yourself\n" +
				"5. Count carefully - if previous response listed 15 items, generate ALL 15 items, not a subset\n\n"
			fmt.Fprintf(os.Stderr, "[FAKER-SESSION] ✅ Including session history in AI prompt (%d events)\n", len(limitedEvents))
		} else {
			fmt.Fprintf(os.Stderr, "[FAKER-SESSION] ⚠️  NO session history included (first call or error)\n")
		}
	} else {
		fmt.Fprintf(os.Stderr, "[FAKER-SESSION] ⚠️  Session manager not initialized (session=%v, manager=%v)\n",
			f.session != nil, f.sessionManager != nil)
	}

	// Build comprehensive prompt for simulation
	argsJSON, _ := json.Marshal(args)
	prompt := fmt.Sprintf(`You are simulating a real AWS CloudWatch MCP tool response.

Tool: %s
Arguments: %s
%s

Original Error (IGNORE THIS): %v

YOUR TASK: %s
%s

CRITICAL: Generate a realistic, successful response in proper AWS CloudWatch JSON format.
Do NOT mention errors or authentication issues.
Generate realistic data with proper structure, timestamps, metrics, and AWS-specific fields.

Output ONLY valid JSON that matches AWS CloudWatch API responses.`,
		toolName,
		string(argsJSON),
		schemaInfo,
		originalError,
		f.instruction,
		sessionHistoryPrompt,
	)

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Simulation prompt length: %d characters\n", len(prompt))
	}

	// Use provided context (caller should have set appropriate timeout)
	// Generate simulated data with enforced timeout
	modelName := f.getModelName()

	logFaker("[FAKER] 📡 Calling OpenAI API with model %s at %s\n", modelName, time.Now().Format("15:04:05"))
	apiStartTime := time.Now()

	// Use channel to enforce timeout (GenKit sometimes doesn't respect context)
	type genResult struct {
		text string
		err  error
	}
	resultChan := make(chan genResult, 1)

	go func() {
		r, e := genkit.Generate(ctx, f.genkitApp,
			ai.WithPrompt(prompt),
			ai.WithModelName(modelName))
		if e != nil {
			resultChan <- genResult{text: "", err: e}
		} else {
			resultChan <- genResult{text: r.Text(), err: nil}
		}
	}()

	// Wait for result or timeout (3 minutes for complex agent responses)
	timeoutTimer := time.NewTimer(180 * time.Second)
	defer timeoutTimer.Stop()

	var text string
	var err error
	select {
	case result := <-resultChan:
		text = result.text
		err = result.err
		apiElapsed := time.Since(apiStartTime)
		logFaker("[FAKER] 📡 OpenAI API call completed in %v at %s\n", apiElapsed, time.Now().Format("15:04:05"))
	case <-timeoutTimer.C:
		apiElapsed := time.Since(apiStartTime)
		logFaker("[FAKER] ⏰ OpenAI API call TIMEOUT after %v at %s\n", apiElapsed, time.Now().Format("15:04:05"))
		return nil, fmt.Errorf("AI generation timeout after %v (hard limit: 180s)", apiElapsed)
	case <-ctx.Done():
		apiElapsed := time.Since(apiStartTime)
		logFaker("[FAKER] ⏰ Context cancelled after %v at %s\n", apiElapsed, time.Now().Format("15:04:05"))
		return nil, fmt.Errorf("AI generation cancelled: %w", ctx.Err())
	}

	if err != nil {
		logFaker("[FAKER] ❌ OpenAI API error: %v\n", err)
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	if text == "" {
		logFaker("[FAKER] ⚠️  OpenAI returned empty text\n")
		return nil, fmt.Errorf("AI generated empty response")
	}

	logFaker("[FAKER] ✅ Generated %d characters of response text\n", len(text))

	// Clean markdown wrappers
	text = strings.TrimPrefix(text, "```json\n")
	text = strings.TrimPrefix(text, "```\n")
	text = strings.TrimSuffix(text, "\n```")
	text = strings.TrimSpace(text)

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Generated simulated response: %s\n", text)
	}

	// Create MCP result with simulated content
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(text),
		},
		IsError: false,
	}, nil
}

// isEmptyOrTrivialResponse detects if a response is empty, contains no useful data, or contains an error
func isEmptyOrTrivialResponse(result *mcp.CallToolResult) bool {
	// No content at all
	if len(result.Content) == 0 {
		return true
	}

	// Check if result is marked as error
	if result.IsError {
		return true
	}

	// Check each content item for emptiness or error messages
	for _, content := range result.Content {
		if tc, ok := content.(*mcp.TextContent); ok {
			text := strings.TrimSpace(tc.Text)

			// Empty text
			if text == "" {
				continue
			}

			// Check for common error patterns (AWS auth errors, permission errors, etc.)
			errorPatterns := []string{
				"invalid security token",
				"security token",
				"access denied",
				"unauthorized",
				"authentication",
				"credentials",
				"permission denied",
				"forbidden",
				"error",
			}
			textLower := strings.ToLower(text)
			for _, pattern := range errorPatterns {
				if strings.Contains(textLower, pattern) {
					return true // Treat error messages as empty/trivial
				}
			}

			// Try parsing as JSON to check for empty arrays/objects
			var jsonData interface{}
			if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
				// Empty array []
				if arr, ok := jsonData.([]interface{}); ok && len(arr) == 0 {
					continue
				}
				// Empty object {}
				if obj, ok := jsonData.(map[string]interface{}); ok && len(obj) == 0 {
					continue
				}
				// Null value
				if jsonData == nil {
					continue
				}
			}

			// Found non-trivial content that's not an error
			return false
		}
	}

	// All content was empty/trivial
	return true
}

// enrichToolResult uses AI to enrich a tool result
func (f *MCPFaker) enrichToolResult(ctx context.Context, toolName string, args map[string]interface{}, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
	logFaker("[FAKER] 🔄 enrichToolResult called for %s at %s\n", toolName, time.Now().Format("15:04:05"))

	// Start enrichment span
	tracer := otel.Tracer("station.faker")
	ctx, span := tracer.Start(ctx, "faker.ai_enrichment",
		trace.WithAttributes(
			attribute.String("faker.tool_name", toolName),
			attribute.String("faker.operation", "ai_enrichment"),
		),
	)
	defer span.End()

	// Skip enrichment if GenKit not initialized (passthrough mode)
	if f.genkitApp == nil {
		span.SetAttributes(attribute.Bool("faker.ai_enrichment_enabled", false))
		logFaker("[FAKER] ⚠️  GenKit not initialized, skipping enrichment\n")
		return result, nil
	}

	span.SetAttributes(attribute.Bool("faker.ai_enrichment_enabled", true))

	// Check if response is empty or trivial - if so, generate simulated data instead
	logFaker("[FAKER] 🔍 Checking if response is empty/trivial...\n")
	isEmpty := isEmptyOrTrivialResponse(result)
	logFaker("[FAKER] 🔍 isEmpty=%v, content_len=%d\n", isEmpty, len(result.Content))

	if isEmpty {
		logFaker("[FAKER] 🔍 Detected empty/trivial response for %s - triggering simulation\n", toolName)

		// Create fresh context for simulation to avoid deadline issues (35s timeout)
		simCtx, simCancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer simCancel()

		logFaker("[FAKER] ⏱️  Starting simulation with 35s timeout at %s\n", time.Now().Format("15:04:05"))
		startTime := time.Now()

		result, err := f.generateSimulatedResponse(simCtx, toolName, args, fmt.Errorf("empty response"))

		elapsed := time.Since(startTime)
		if err != nil {
			logFaker("[FAKER] ❌ Simulation FAILED after %v: %v\n", elapsed, err)
			return nil, err
		}

		logFaker("[FAKER] ✅ Simulation SUCCESS after %v\n", elapsed)
		return result, nil
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Starting enrichment for tool: %s\n", toolName)
		resultJSON, _ := json.Marshal(result.Content)
		fmt.Fprintf(os.Stderr, "[FAKER] Original result content: %s\n", string(resultJSON))

		// DEBUG: Check context BEFORE creating new one
		if deadline, ok := ctx.Deadline(); ok {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] enrichToolResult BEFORE new context: deadline=%v (timeout in %v) ⚠️\n", deadline, time.Until(deadline))
		} else {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] enrichToolResult BEFORE new context: NO deadline ✓\n")
		}
	}

	// Create a new context with longer timeout for AI generation (30 seconds)
	// This prevents "context deadline exceeded" errors during OpenAI API calls
	enrichCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = enrichCtx

	if f.debug {
		// DEBUG: Check context AFTER creating new one
		if deadline, ok := ctx.Deadline(); ok {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] enrichToolResult AFTER new context: deadline=%v (timeout in %v) ✓\n", deadline, time.Until(deadline))
		} else {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] enrichToolResult AFTER new context: NO deadline?! ⚠️\n")
		}
	}

	// Build enrichment prompt with message history context
	instruction := f.instruction
	if instruction == "" {
		// Extract just the text content from the result as an example
		var textContent string
		for _, content := range result.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				textContent = tc.Text
				break
			}
		}
		instruction = fmt.Sprintf("Generate realistic mock data similar to this example: %s", textContent)
	}

	// Add message history context for consistency
	if len(f.callHistory) > 0 {
		historyContext := "\n\nPrevious tool calls in this session (maintain consistency with these responses):\n"
		for i, call := range f.callHistory {
			historyContext += fmt.Sprintf("\n%d. Tool: %s\n", i+1, call.ToolName)
			if len(call.Arguments) > 0 {
				argsJSON, _ := json.Marshal(call.Arguments)
				historyContext += fmt.Sprintf("   Arguments: %s\n", string(argsJSON))
			}
			historyContext += fmt.Sprintf("   Response: %s\n", call.Response)
		}
		historyContext += "\n\nIMPORTANT: Maintain consistency with previous responses. If a file/directory was created in a previous call, include it in subsequent list operations."
		instruction = instruction + historyContext
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Using instruction with %d history items\n", len(f.callHistory))
	}

	// Define output schema matching MCP Content structure
	type ContentItem struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type OutputSchema struct {
		Content []ContentItem `json:"content"`
	}

	// Get model name
	modelName := f.getModelName()

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Calling GenKit with model: %s and output schema\n", modelName)

		// DEBUG: Final context check RIGHT before calling GenKit
		if deadline, ok := ctx.Deadline(); ok {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] RIGHT BEFORE GenKit.GenerateData: deadline=%v (timeout in %v)\n", deadline, time.Until(deadline))
		} else {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] RIGHT BEFORE GenKit.GenerateData: NO deadline ⚠️\n")
		}
	}

	// Call GenKit with output schema for structured generation
	output, _, err := genkit.GenerateData[OutputSchema](ctx, f.genkitApp,
		ai.WithPrompt(instruction),
		ai.WithModelName(modelName))

	if err != nil {
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] GenerateData failed: %v, trying Generate fallback\n", err)
		}
		// Fallback to regular Generate if structured generation fails
		resp, err := genkit.Generate(ctx, f.genkitApp,
			ai.WithPrompt(instruction),
			ai.WithModelName(modelName))

		if err != nil {
			return nil, fmt.Errorf("genkit generate failed: %w", err)
		}

		text := resp.Text()
		if text == "" {
			return result, nil
		}

		// Clean markdown
		text = strings.TrimPrefix(text, "```json\n")
		text = strings.TrimPrefix(text, "```\n")
		text = strings.TrimSuffix(text, "\n```")
		text = strings.TrimSpace(text)

		var parsedOutput OutputSchema
		if err := json.Unmarshal([]byte(text), &parsedOutput); err != nil {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Text fallback also failed: %v\n", err)
			}
			return result, nil
		}
		output = &parsedOutput
	}

	if f.debug {
		outputJSON, _ := json.MarshalIndent(output, "", "  ")
		fmt.Fprintf(os.Stderr, "[FAKER] GenKit structured response: %s\n", string(outputJSON))
	}

	// Convert to mcp.Content
	// Accept any type (the AI might generate "file", "directory", "text", etc.)
	var enrichedContent []mcp.Content
	for _, item := range output.Content {
		if item.Text != "" {
			enrichedContent = append(enrichedContent, mcp.NewTextContent(item.Text))
		}
	}

	if len(enrichedContent) == 0 {
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] No valid content in AI response\n")
		}
		return result, nil
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Successfully enriched content with %d items\n", len(enrichedContent))
	}

	// Return enriched result
	return &mcp.CallToolResult{
		Content: enrichedContent,
		IsError: result.IsError,
	}, nil
}

// getModelName builds the model name with provider prefix
func (f *MCPFaker) getModelName() string {
	baseModel := f.stationConfig.AIModel
	if baseModel == "" {
		baseModel = "gpt-5-mini"
	}

	switch strings.ToLower(f.stationConfig.AIProvider) {
	case "gemini", "googlegenai":
		return fmt.Sprintf("googleai/%s", baseModel)
	case "openai":
		return fmt.Sprintf("openai/%s", baseModel)
	default:
		return fmt.Sprintf("%s/%s", f.stationConfig.AIProvider, baseModel)
	}
}

// ToolClassification represents the AI's analysis of a tool
type ToolClassification struct {
	IsWriteOperation bool   `json:"is_write_operation"`
	Reason           string `json:"reason"`
	RiskLevel        string `json:"risk_level"` // "safe", "caution", "dangerous"
}

// classifyTools uses AI to determine which tools perform write operations
// Falls back to heuristic classification if AI times out
func (f *MCPFaker) classifyTools(ctx context.Context, tools []mcp.Tool) error {
	modelName := f.getModelName()

	for _, tool := range tools {
		// Create a fresh timeout context for EACH tool classification
		// IMPORTANT: Use context.Background() to avoid inheriting parent context's short deadline
		classifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// Build classification prompt
		prompt := fmt.Sprintf(`Analyze this MCP tool and determine if it performs write operations (create, update, delete, modify data).

Tool Name: %s
Tool Description: %s

Write operations include: create, update, delete, modify, write, remove, cancel, stop, start, execute (commands), deploy, move, edit, etc.
Read operations include: get, list, describe, read, fetch, query, search, analyze, etc.

Respond with your analysis.`, tool.Name, tool.Description)

		// Call GenKit with structured output
		classification, _, err := genkit.GenerateData[ToolClassification](classifyCtx, f.genkitApp,
			ai.WithPrompt(prompt),
			ai.WithModelName(modelName))

		if err != nil {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] AI classification failed for %s: %v, using heuristic fallback\n", tool.Name, err)
			}
			// Use heuristic fallback when AI times out
			isWrite := f.heuristicClassifyTool(tool.Name, tool.Description)
			if isWrite {
				f.writeOperations[tool.Name] = true
			}
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Tool %s: write=%v (heuristic classification)\n", tool.Name, isWrite)
			}
			continue
		}

		// Store classification
		if classification.IsWriteOperation {
			f.writeOperations[tool.Name] = true
		}

		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Tool %s: write=%v, risk=%s, reason=%s\n",
				tool.Name, classification.IsWriteOperation, classification.RiskLevel, classification.Reason)
		}
	}

	return nil
}

// heuristicClassifyTool provides fast, deterministic classification when AI is unavailable
func (f *MCPFaker) heuristicClassifyTool(toolName, description string) bool {
	toolNameLower := strings.ToLower(toolName)
	descLower := strings.ToLower(description)

	// Read-only operation keywords (check FIRST to override write keywords)
	readOnlyKeywords := []string{
		"tree", "list", "read", "get", "search", "find", "stat", "info",
		"query", "fetch", "retrieve", "show", "view", "display", "check",
		"scan", "detect", "analyze", "inspect", "browse", "explore", "watch",
		"describe", "explain", "count", "size", "exists", "compare",
	}

	// Check read-only keywords first - if found, it's definitely NOT a write operation
	for _, keyword := range readOnlyKeywords {
		if strings.Contains(toolNameLower, keyword) || strings.Contains(descLower, keyword) {
			return false // Definitely a read operation
		}
	}

	// Write operation keywords
	writeKeywords := []string{
		"write", "create", "update", "delete", "remove", "modify",
		"edit", "move", "rename", "deploy", "execute", "run",
		"start", "stop", "cancel", "terminate", "kill", "set",
		"put", "post", "patch", "insert", "append", "save",
	}

	// Check tool name and description for write keywords
	for _, keyword := range writeKeywords {
		if strings.Contains(toolNameLower, keyword) || strings.Contains(descLower, keyword) {
			return true
		}
	}

	return false
}

// displayToolClassification shows the user which tools are classified as write operations
func (f *MCPFaker) displayToolClassification() {
	writeCount := 0
	var writeTools []string

	for toolName, isWrite := range f.writeOperations {
		if isWrite {
			writeCount++
			writeTools = append(writeTools, toolName)
		}
	}

	if writeCount > 0 {
		fmt.Fprintf(os.Stderr, "\n[FAKER] 🛡️  SAFETY MODE: %d write operations detected and will be INTERCEPTED:\n", writeCount)
		for i, toolName := range writeTools {
			fmt.Fprintf(os.Stderr, "[FAKER]   %d. %s\n", i+1, toolName)
		}
		fmt.Fprintf(os.Stderr, "[FAKER] These tools will return mock success responses without executing real operations.\n\n")
	} else {
		fmt.Fprintf(os.Stderr, "[FAKER] ✅ No write operations detected - all tools are read-only\n")
	}
}

// createMockSuccessResponse returns a realistic success response for intercepted write operations
// The response mimics what a real MCP tool would return without revealing it's a simulation
func (f *MCPFaker) createMockSuccessResponse(toolName string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	// Create realistic success messages based on tool type
	var mockResponse string

	switch {
	case strings.Contains(toolName, "write"):
		path, _ := arguments["path"].(string)
		mockResponse = fmt.Sprintf("Successfully wrote to %s", path)

	case strings.Contains(toolName, "create"):
		path, _ := arguments["path"].(string)
		if path == "" {
			path, _ = arguments["directory"].(string)
		}
		mockResponse = fmt.Sprintf("Successfully created %s", path)

	case strings.Contains(toolName, "edit"):
		path, _ := arguments["path"].(string)
		mockResponse = fmt.Sprintf("Successfully edited %s", path)

	case strings.Contains(toolName, "move") || strings.Contains(toolName, "rename"):
		from, _ := arguments["source"].(string)
		to, _ := arguments["destination"].(string)
		if from == "" {
			from, _ = arguments["oldPath"].(string)
			to, _ = arguments["newPath"].(string)
		}
		mockResponse = fmt.Sprintf("Successfully moved %s to %s", from, to)

	case strings.Contains(toolName, "delete") || strings.Contains(toolName, "remove"):
		path, _ := arguments["path"].(string)
		mockResponse = fmt.Sprintf("Successfully deleted %s", path)

	default:
		mockResponse = "Operation completed successfully"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(mockResponse),
		},
		IsError: false,
	}, nil
}

// recordToolCall adds a tool call and its response to history for consistency
func (f *MCPFaker) recordToolCall(toolName string, arguments map[string]interface{}, result *mcp.CallToolResult) {
	// Extract response text from result
	var responseText string
	if len(result.Content) > 0 {
		// Combine all content into a single string
		var parts []string
		for _, content := range result.Content {
			if tc, ok := content.(*mcp.TextContent); ok {
				parts = append(parts, tc.Text)
			}
		}
		responseText = strings.Join(parts, "\n")
	}

	// Limit response text to avoid prompt bloat (first 500 chars)
	if len(responseText) > 500 {
		responseText = responseText[:500] + "... (truncated)"
	}

	historyEntry := ToolCallHistory{
		ToolName:  toolName,
		Arguments: arguments,
		Response:  responseText,
		Timestamp: fmt.Sprintf("%v", time.Now().Format(time.RFC3339)),
	}

	f.callHistory = append(f.callHistory, historyEntry)

	// Keep only last 5 calls to avoid prompt bloat
	if len(f.callHistory) > 5 {
		f.callHistory = f.callHistory[len(f.callHistory)-5:]
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Recorded tool call in history (total: %d)\n", len(f.callHistory))
	}
}

// globalTracerProvider stores the tracer provider for shutdown
var globalTracerProvider *sdktrace.TracerProvider

// initializeFakerOTEL sets up OpenTelemetry for faker span export
func initializeFakerOTEL(ctx context.Context, endpoint string, debug bool) error {
	// Check if endpoint uses HTTPS before stripping protocol
	useHTTPS := strings.HasPrefix(endpoint, "https://")
	// Parse endpoint to remove protocol - OTLP HTTP exporter expects host:port format
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER OTEL] Initializing with endpoint: %s (https=%v)\n", endpoint, useHTTPS)
	}

	// Create OTLP HTTP trace exporter
	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(endpoint),
	}
	// Only use insecure (HTTP) if endpoint is not HTTPS
	if !useHTTPS {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource for faker process without schema to avoid version conflicts
	res := resource.NewSchemaless(
		semconv.ServiceName("station"),
		semconv.ServiceVersion("faker"),
	)

	// Use SimpleSpanProcessor for immediate export (better for short-lived MCP tool calls)
	// BatchSpanProcessor can delay export up to 5 seconds, causing spans to be lost
	// when faker subprocess completes before batch flush
	spanProcessor := sdktrace.NewSimpleSpanProcessor(exporter)

	// Create and set global tracer provider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanProcessor),
		sdktrace.WithResource(res),
	)

	// Store for shutdown
	globalTracerProvider = tracerProvider

	otel.SetTracerProvider(tracerProvider)

	return nil
}

// shutdownFakerOTEL gracefully shuts down OTEL and flushes remaining spans
func shutdownFakerOTEL(ctx context.Context) error {
	if globalTracerProvider == nil {
		return nil
	}

	// Create timeout context for shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	return globalTracerProvider.Shutdown(shutdownCtx)
}
