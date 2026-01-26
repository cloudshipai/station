package session

import (
	"context"
	"fmt"
	"time"

	"station/pkg/harness"
	"station/pkg/harness/sandbox"
	"station/pkg/harness/workspace"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// SessionExecutor wraps AgenticExecutor with session persistence.
// It maintains message history and workspace state across multiple interactions.
type SessionExecutor struct {
	genkitApp      *genkit.Genkit
	sessionManager *Manager
	historyStore   *HistoryStore
	harnessConfig  *harness.HarnessConfig
	agentConfig    *harness.AgentHarnessConfig
	modelName      string
	systemPrompt   string

	// Active session state
	currentSessionID string
	currentSession   *Session
	currentWorkspace harness.WorkspaceManager
	currentSandbox   sandbox.Sandbox
	sandboxConfig    *sandbox.Config
	tools            []ai.Tool
}

// SessionExecutorConfig holds configuration for the session executor
type SessionExecutorConfig struct {
	BasePath      string
	HarnessConfig *harness.HarnessConfig
	AgentConfig   *harness.AgentHarnessConfig
	ModelName     string
	SystemPrompt  string
	SandboxConfig *sandbox.Config
}

// NewSessionExecutor creates a new session executor
func NewSessionExecutor(genkitApp *genkit.Genkit, cfg SessionExecutorConfig) *SessionExecutor {
	if cfg.HarnessConfig == nil {
		cfg.HarnessConfig = harness.DefaultHarnessConfig()
	}
	if cfg.AgentConfig == nil {
		cfg.AgentConfig = harness.DefaultAgentHarnessConfig()
	}
	if cfg.ModelName == "" {
		cfg.ModelName = "openai/gpt-4o-mini"
	}

	return &SessionExecutor{
		genkitApp:      genkitApp,
		sessionManager: NewManager(cfg.BasePath),
		historyStore:   NewHistoryStore(cfg.BasePath),
		harnessConfig:  cfg.HarnessConfig,
		agentConfig:    cfg.AgentConfig,
		modelName:      cfg.ModelName,
		systemPrompt:   cfg.SystemPrompt,
		sandboxConfig:  cfg.SandboxConfig,
	}
}

// StartSession starts or resumes a session
func (se *SessionExecutor) StartSession(ctx context.Context, sessionID string, repoURL string, branch string) error {
	// Get or create session
	session, err := se.sessionManager.GetOrCreate(ctx, sessionID, repoURL, branch)
	if err != nil {
		return fmt.Errorf("failed to get/create session: %w", err)
	}

	// Acquire lock
	runID := fmt.Sprintf("repl-%d", time.Now().UnixNano())
	if err := se.sessionManager.AcquireLock(ctx, sessionID, runID); err != nil {
		return fmt.Errorf("failed to acquire session lock: %w", err)
	}

	// Initialize workspace
	ws := workspace.NewHostWorkspace(session.Path)
	if err := ws.Initialize(ctx); err != nil {
		se.sessionManager.ReleaseLock(ctx, sessionID, runID)
		return fmt.Errorf("failed to initialize workspace: %w", err)
	}

	// Initialize sandbox if configured (keep alive for session duration)
	var sb sandbox.Sandbox
	if se.sandboxConfig != nil {
		// Set workspace path for sandbox
		sandboxCfg := *se.sandboxConfig
		sandboxCfg.WorkspacePath = session.Path

		factory := sandbox.NewFactory(sandbox.DefaultConfig())
		sb, err = factory.Create(sandboxCfg)
		if err != nil {
			se.sessionManager.ReleaseLock(ctx, sessionID, runID)
			return fmt.Errorf("failed to create sandbox: %w", err)
		}

		if err := sb.Create(ctx); err != nil {
			se.sessionManager.ReleaseLock(ctx, sessionID, runID)
			return fmt.Errorf("failed to initialize sandbox: %w", err)
		}
	}

	se.currentSessionID = sessionID
	se.currentSession = session
	se.currentWorkspace = ws
	se.currentSandbox = sb

	return nil
}

// EndSession ends the current session, cleaning up resources
func (se *SessionExecutor) EndSession(ctx context.Context) error {
	if se.currentSessionID == "" {
		return nil
	}

	// Destroy sandbox if it exists
	if se.currentSandbox != nil {
		if err := se.currentSandbox.Destroy(ctx); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to destroy sandbox: %v\n", err)
		}
		se.currentSandbox = nil
	}

	// Release lock
	runID := fmt.Sprintf("repl-%d", time.Now().UnixNano())
	se.sessionManager.ReleaseLock(ctx, se.currentSessionID, runID)

	se.currentSessionID = ""
	se.currentSession = nil
	se.currentWorkspace = nil

	return nil
}

// SetTools sets the available tools for the session
func (se *SessionExecutor) SetTools(tools []ai.Tool) {
	se.tools = tools
}

// Execute runs a task within the current session, maintaining history
func (se *SessionExecutor) Execute(ctx context.Context, task string) (*harness.ExecutionResult, error) {
	if se.currentSessionID == "" {
		return nil, fmt.Errorf("no active session - call StartSession first")
	}

	// Load existing history
	history, err := se.historyStore.Load(se.currentSessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}

	// Convert stored messages to ai.Message format for the executor
	initialHistory := history.ToAIMessages()

	// Create executor with session state
	opts := []harness.ExecutorOption{
		harness.WithWorkspace(se.currentWorkspace),
		harness.WithModelName(se.modelName),
	}

	if se.systemPrompt != "" {
		opts = append(opts, harness.WithSystemPrompt(se.systemPrompt))
	}

	if se.currentSandbox != nil {
		opts = append(opts, harness.WithSandbox(se.currentSandbox))
	}

	// Inject history from previous session interactions
	if len(initialHistory) > 0 {
		opts = append(opts, harness.WithInitialHistory(initialHistory))
	}

	executor := harness.NewAgenticExecutor(
		se.genkitApp,
		se.harnessConfig,
		se.agentConfig,
		opts...,
	)

	// Execute with the task
	result, err := executor.Execute(ctx, se.currentSessionID, task, se.tools)

	// Save the execution history for session persistence
	// Use the full history from the result if available, otherwise construct from task/response
	if err == nil && result != nil {
		if len(result.History) > 0 {
			// Use full history from executor (includes tool calls, etc.)
			history.Messages = FromAIMessages(result.History)
		} else if result.Success {
			// Fallback: append user task and assistant response
			newMessages := []StoredMessage{
				{
					Role:      "user",
					Content:   task,
					Timestamp: time.Now(),
				},
				{
					Role:      "assistant",
					Content:   result.Response,
					Timestamp: time.Now(),
				},
			}
			history.Messages = append(history.Messages, newMessages...)
		}
		history.TotalTokens += result.TotalTokens

		if err := se.historyStore.Save(se.currentSessionID, history); err != nil {
			// Log but don't fail the execution
			fmt.Printf("Warning: failed to save history: %v\n", err)
		}
	}

	return result, err
}

// GetHistory returns the current session's message history
func (se *SessionExecutor) GetHistory() (*SessionHistory, error) {
	if se.currentSessionID == "" {
		return nil, fmt.Errorf("no active session")
	}
	return se.historyStore.Load(se.currentSessionID)
}

// ClearHistory clears the current session's message history
func (se *SessionExecutor) ClearHistory() error {
	if se.currentSessionID == "" {
		return fmt.Errorf("no active session")
	}
	return se.historyStore.Clear(se.currentSessionID)
}

// CurrentSession returns the current session info
func (se *SessionExecutor) CurrentSession() *Session {
	return se.currentSession
}

// RefreshLock refreshes the session lock (call periodically for long sessions)
func (se *SessionExecutor) RefreshLock(ctx context.Context) error {
	if se.currentSessionID == "" {
		return nil
	}
	runID := fmt.Sprintf("repl-%d", time.Now().UnixNano())
	return se.sessionManager.RefreshLock(ctx, se.currentSessionID, runID)
}
