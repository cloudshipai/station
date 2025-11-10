package faker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"station/internal/config"
	"station/internal/db"

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
	targetClient    *client.Client // Client to the real MCP server
	genkitApp       *genkit.Genkit
	stationConfig   *config.Config
	instruction     string
	debug           bool
	writeOperations map[string]bool      // Tools classified as write operations
	safetyMode      bool                 // If true, intercept write operations
	callHistory     []ToolCallHistory    // Legacy: Message history for consistency (deprecated, use session)
	sessionManager  *SessionManager      // Session-based state tracking
	session         *FakerSession        // Current faker session
	toolSchemas     map[string]*mcp.Tool // Tool definitions for schema extraction
}

// NewMCPFaker creates a new MCP faker server
func NewMCPFaker(targetCmd string, targetArgs []string, targetEnv map[string]string, instruction string, debug bool) (*MCPFaker, error) {
	ctx := context.Background()

	if debug {
		if deadline, ok := ctx.Deadline(); ok {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] NewMCPFaker context deadline: %v (timeout in %v)\n", deadline, time.Until(deadline))
		} else {
			fmt.Fprintf(os.Stderr, "[FAKER DEBUG] NewMCPFaker context has NO deadline (infinite timeout) ✓\n")
		}
	}

	// Load Station config
	stationConfig, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Only initialize GenKit if AI enrichment is requested
	var app *genkit.Genkit
	if instruction != "" {
		// Disable GenKit reflection server to prevent port conflicts
		if os.Getenv("GENKIT_ENV") == "" {
			os.Setenv("GENKIT_ENV", "prod")
		}

		// Disable telemetry
		os.Setenv("OTEL_SDK_DISABLED", "true")

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

	// Initialize target client
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: mcp.Implementation{
				Name:    "faker-mcp-client",
				Version: "1.0.0",
			},
		},
	}
	if _, err := targetClient.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("failed to initialize target client: %w", err)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Target client initialized successfully\n")
	}

	// Initialize session management (only if instruction provided for AI enrichment)
	var sessionMgr *SessionManager
	var session *FakerSession
	if instruction != "" {
		// Open database connection
		database, err := db.New(stationConfig.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open database for session management: %w", err)
		}

		sessionMgr = NewSessionManager(database.Conn(), debug)

		// Create new session for this faker instance
		session, err = sessionMgr.CreateSession(ctx, instruction)
		if err != nil {
			return nil, fmt.Errorf("failed to create faker session: %w", err)
		}

		if debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Created session %s\n", session.ID)
		}
	}

	return &MCPFaker{
		targetClient:    targetClient,
		genkitApp:       app,
		stationConfig:   stationConfig,
		instruction:     instruction,
		debug:           debug,
		writeOperations: make(map[string]bool),
		safetyMode:      true, // Always enable safety mode by default
		callHistory:     make([]ToolCallHistory, 0),
		sessionManager:  sessionMgr,
		session:         session,
		toolSchemas:     make(map[string]*mcp.Tool),
	}, nil
}

// Serve starts the faker as an MCP server on stdio
func (f *MCPFaker) Serve() error {
	ctx := context.Background()

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Starting MCP server...\n")
	}

	// Create MCP server
	mcpServer := server.NewMCPServer("faker-mcp-server", "1.0.0")

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Listing tools from target...\n")
	}

	// Get all tools from target server
	toolsResult, err := f.targetClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools from target: %w", err)
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Found %d tools from target, registering...\n", len(toolsResult.Tools))
	}

	// Classify tools as read/write operations using AI
	if f.safetyMode && f.genkitApp != nil {
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] Classifying tools for write operation detection...\n")
		}
		if err := f.classifyTools(ctx, toolsResult.Tools); err != nil {
			if f.debug {
				fmt.Fprintf(os.Stderr, "[FAKER] Warning: Tool classification failed: %v\n", err)
			}
		} else {
			f.displayToolClassification()
		}
	}

	// Register each tool with a proxy handler and store schemas
	for _, tool := range toolsResult.Tools {
		// Store tool schema for response shape consistency
		toolCopy := tool // Capture tool in closure
		f.toolSchemas[tool.Name] = &toolCopy

		mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return f.handleToolCall(ctx, request)
		})
	}

	if f.debug {
		fmt.Fprintf(os.Stderr, "[FAKER] Starting stdio server...\n")
	}

	// Serve on stdio
	return server.ServeStdio(mcpServer)
}

// handleToolCall proxies a tool call to the target, enriches the response, and returns it
func (f *MCPFaker) handleToolCall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

		return mockResult, nil
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
			return synthesizedResult, nil
		}
	}

	// Call the real target server for read operations (no write history or synthesis failed)
	result, err := f.targetClient.CallTool(ctx, request)
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
	enrichedResult, err := f.enrichToolResult(ctx, request.Params.Name, result)
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

	return enrichedResult, nil
}

// enrichToolResult uses AI to enrich a tool result
func (f *MCPFaker) enrichToolResult(ctx context.Context, toolName string, result *mcp.CallToolResult) (*mcp.CallToolResult, error) {
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
		if f.debug {
			fmt.Fprintf(os.Stderr, "[FAKER] GenKit not initialized, skipping enrichment (passthrough mode)\n")
		}
		return result, nil
	}

	span.SetAttributes(attribute.Bool("faker.ai_enrichment_enabled", true))

	// Extract content from result
	if len(result.Content) == 0 {
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
		baseModel = "gpt-4o-mini"
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
