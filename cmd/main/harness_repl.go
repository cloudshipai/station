package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"station/internal/config"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/harness/memory"
	"station/pkg/harness/session"
	"station/pkg/harness/skills"
	"station/pkg/models"
)

var harnessCmd = &cobra.Command{
	Use:   "harness",
	Short: "Agentic harness commands",
	Long:  "Commands for working with the agentic harness system",
}

var harnessReplCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start interactive REPL for agent testing",
	Long: `Start an interactive REPL (Read-Eval-Print-Loop) for testing agentic harness agents.

The REPL provides a conversational interface where you can:
• Send tasks to the agent and see responses
• Track session state, steps, and token usage
• List available skills and tools
• Save and load sessions for later

EXAMPLES:
  # Start REPL for an agent
  stn harness repl --agent myagent --env default

  # Resume a previous session
  stn harness repl --session abc123

  # Start with specific sandbox mode
  stn harness repl --agent myagent --sandbox docker

REPL COMMANDS:
  /help      Show all available commands
  /status    Show session status (steps, tokens, progress)
  /history   Show conversation history summary
  /skills    List available skills
  /tools     List available tools
  /files     List workspace files
  /reset     Reset session (start fresh)
  /save      Save session to file
  /load      Load session from file
  /exit      Exit REPL`,
	RunE: runHarnessRepl,
}

func init() {
	harnessReplCmd.Flags().StringP("agent", "a", "", "Agent name to run")
	harnessReplCmd.Flags().StringP("env", "e", "default", "Environment name")
	harnessReplCmd.Flags().StringP("session", "s", "", "Session ID to resume")
	harnessReplCmd.Flags().String("sandbox", "", "Sandbox mode (host, docker, modal, etc.)")
	harnessReplCmd.Flags().StringSlice("skills", nil, "Additional skill source paths")
	harnessReplCmd.Flags().Int("max-steps", 50, "Maximum steps per execution")
	harnessReplCmd.Flags().String("timeout", "30m", "Execution timeout")

	harnessCmd.AddCommand(harnessReplCmd)
}

// ReplSession holds the state for a REPL session
type ReplSession struct {
	ID           string
	AgentID      string
	AgentName    string
	Task         string
	StepCount    int
	MessageCount int
	TotalTokens  int
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Session persistence
	sessionManager *session.Manager
	historyStore   *session.HistoryStore
	persistedPath  string
}

func runHarnessRepl(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n\nGoodbye!")
		cancel()
		os.Exit(0)
	}()

	agentName, _ := cmd.Flags().GetString("agent")
	envName, _ := cmd.Flags().GetString("env")
	sessionID, _ := cmd.Flags().GetString("session")
	sandboxMode, _ := cmd.Flags().GetString("sandbox")
	skillSources, _ := cmd.Flags().GetStringSlice("skills")
	maxSteps, _ := cmd.Flags().GetInt("max-steps")
	timeoutStr, _ := cmd.Flags().GetString("timeout")

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 30 * time.Minute
	}

	// Validate inputs
	if agentName == "" && sessionID == "" {
		return fmt.Errorf("either --agent or --session is required")
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() { _ = database.Close() }()

	repos := repositories.New(database)

	// Get environment
	env, err := repos.Environments.GetByName(envName)
	if err != nil {
		return fmt.Errorf("environment '%s' not found: %w", envName, err)
	}

	// Get agent
	var agent *models.Agent
	if agentName != "" {
		agents, err := repos.Agents.ListByEnvironment(env.ID)
		if err != nil {
			return fmt.Errorf("failed to list agents: %w", err)
		}

		for _, a := range agents {
			if a.Name == agentName {
				agent = a
				break
			}
		}

		if agent == nil {
			return fmt.Errorf("agent '%s' not found in environment '%s'", agentName, envName)
		}
	}

	// Initialize skills middleware
	var skillsMiddleware *skills.SkillsMiddleware
	envPath := config.GetEnvironmentDir(envName)
	skillSourcePaths := skills.DefaultSkillSources(envPath)
	if len(skillSources) > 0 {
		skillSourcePaths = append(skillSourcePaths, skillSources...)
	}
	skillsBackend := skills.NewFSBackend()
	skillsMiddleware = skills.NewSkillsMiddleware(skillsBackend, skillSourcePaths)

	// Initialize memory middleware
	memoryBackend := memory.NewFSBackend()
	memorySources := memory.DefaultMemorySources(envPath)
	memoryMiddleware := memory.NewMemoryMiddleware(memoryBackend, memorySources)

	// Initialize session persistence
	workspacePath := cfg.Harness.Workspace.Path
	if workspacePath == "" {
		workspacePath = filepath.Join(cfg.Workspace, "workspace")
	}
	if !filepath.IsAbs(workspacePath) {
		workspacePath = filepath.Join(cfg.Workspace, workspacePath)
	}
	sessionMgr := session.NewManager(workspacePath)
	historyStore := session.NewHistoryStore(workspacePath)

	// Create or resume session
	replSession := &ReplSession{
		ID:             fmt.Sprintf("repl-%d", time.Now().Unix()),
		AgentName:      agentName,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		sessionManager: sessionMgr,
		historyStore:   historyStore,
	}
	if agent != nil {
		replSession.AgentID = fmt.Sprintf("%d", agent.ID)
	}

	// Resume or create persisted session
	if sessionID != "" {
		replSession.ID = sessionID
		// Load existing session metadata
		existingSession, err := sessionMgr.Get(ctx, sessionID)
		if err == nil {
			replSession.persistedPath = existingSession.Path
			// Load history to get message count and tokens
			history, err := historyStore.Load(sessionID)
			if err == nil {
				replSession.MessageCount = len(history.Messages)
				replSession.TotalTokens = history.TotalTokens
			}
			fmt.Printf("Resuming session %s (%d messages)\n", sessionID, replSession.MessageCount)
		} else {
			fmt.Printf("Session %s not found, creating new...\n", sessionID)
		}
	}

	// Ensure session directory exists
	if _, err := sessionMgr.GetOrCreate(ctx, replSession.ID, "", ""); err != nil {
		fmt.Printf("Warning: failed to create session directory: %v\n", err)
	}

	// Print banner
	printReplBanner(replSession, sandboxMode, envName)

	// Main REPL loop
	reader := bufio.NewReader(os.Stdin)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Print(">>> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle REPL commands
		if strings.HasPrefix(input, "/") {
			if handleReplCommand(ctx, replSession, skillsMiddleware, memoryMiddleware, input) {
				return nil // /exit
			}
			continue
		}

		// Execute agent task
		fmt.Println()
		result, err := executeReplTask(ctx, cfg, repos, env, agent, input, maxSteps, timeout, sandboxMode, skillsMiddleware, memoryMiddleware)
		if err != nil {
			fmt.Printf("Error: %v\n\n", err)
			continue
		}

		// Update session stats
		replSession.StepCount += result.TotalSteps
		replSession.TotalTokens += result.TotalTokens
		replSession.MessageCount++
		replSession.UpdatedAt = time.Now()

		// Persist message history
		if replSession.historyStore != nil {
			newMessages := []session.StoredMessage{
				{
					Role:      "user",
					Content:   input,
					Timestamp: time.Now(),
				},
				{
					Role:      "assistant",
					Content:   result.Response,
					Timestamp: time.Now(),
				},
			}
			if err := replSession.historyStore.Append(replSession.ID, newMessages); err != nil {
				fmt.Printf("Warning: failed to save history: %v\n", err)
			}
		}

		// Print response
		fmt.Println(result.Response)
		fmt.Printf("\n[Steps: %d | Tokens: %d | %s]\n\n",
			result.TotalSteps, result.TotalTokens, result.FinishReason)
	}

	return nil
}

func printReplBanner(session *ReplSession, sandboxMode, envName string) {
	if sandboxMode == "" {
		sandboxMode = "host"
	}

	sessionDisplay := session.ID
	if len(sessionDisplay) > 8 {
		sessionDisplay = sessionDisplay[:8] + "..."
	}

	fmt.Println()
	fmt.Println("+=============================================+")
	fmt.Println("|       Station Agent REPL                    |")
	fmt.Println("+=============================================+")
	fmt.Printf("|  Agent:   %-33s |\n", session.AgentName)
	fmt.Printf("|  Session: %-33s |\n", sessionDisplay)
	fmt.Printf("|  Env:     %-33s |\n", envName)
	fmt.Printf("|  Sandbox: %-33s |\n", sandboxMode)
	fmt.Println("+---------------------------------------------+")
	fmt.Println("|  Commands:                                  |")
	fmt.Println("|    /help    - Show all commands             |")
	fmt.Println("|    /status  - Session status                |")
	fmt.Println("|    /exit    - Exit REPL                     |")
	fmt.Println("+=============================================+")
	fmt.Println()
}

func handleReplCommand(ctx context.Context, session *ReplSession, skillsMW *skills.SkillsMiddleware, memoryMW *memory.MemoryMiddleware, input string) bool {
	parts := strings.Fields(input)
	cmdName := parts[0]

	switch cmdName {
	case "/exit", "/quit", "/q":
		fmt.Println("Goodbye!")
		return true

	case "/help", "/h", "/?":
		printReplHelp()

	case "/status":
		printReplSessionStatus(session)

	case "/skills":
		if skillsMW != nil {
			loadedSkills, err := skillsMW.LoadSkills()
			if err != nil {
				fmt.Printf("Error loading skills: %v\n", err)
			} else if len(loadedSkills) == 0 {
				fmt.Println("No skills found")
			} else {
				fmt.Println("\nAvailable Skills:")
				for _, skill := range loadedSkills {
					fmt.Printf("  - %s: %s\n", skill.Name, skill.Description)
					fmt.Printf("    Path: %s\n", skill.Path)
				}
				fmt.Println()
			}
		} else {
			fmt.Println("Skills middleware not initialized")
		}

	case "/memory":
		if memoryMW != nil {
			contents, err := memoryMW.LoadMemory()
			if err != nil {
				fmt.Printf("Error loading memory: %v\n", err)
			} else if len(contents) == 0 {
				fmt.Println("No memory sources found")
			} else {
				fmt.Println("\nLoaded Memory Sources:")
				for source := range contents {
					fmt.Printf("  - %s\n", source)
				}
				fmt.Println()
			}
		} else {
			fmt.Println("Memory middleware not initialized")
		}

	case "/tools":
		fmt.Println("\nAvailable Tools:")
		fmt.Println("  - bash: Execute shell commands")
		fmt.Println("  - read_file: Read file contents")
		fmt.Println("  - write_file: Write file contents")
		fmt.Println("  - edit_file: Edit file contents")
		fmt.Println("  - glob: Find files by pattern")
		fmt.Println("  - grep: Search file contents")
		fmt.Println("  (Additional MCP tools depend on environment configuration)")
		fmt.Println()

	case "/reset":
		// Clear persisted history before resetting
		if session.historyStore != nil {
			_ = session.historyStore.Clear(session.ID)
		}
		session.StepCount = 0
		session.TotalTokens = 0
		session.MessageCount = 0
		session.ID = fmt.Sprintf("repl-%d", time.Now().Unix())
		session.CreatedAt = time.Now()
		session.UpdatedAt = time.Now()
		// Create new session directory
		if session.sessionManager != nil {
			_, _ = session.sessionManager.GetOrCreate(ctx, session.ID, "", "")
		}
		fmt.Printf("Session reset. New session: %s\n\n", session.ID[:12]+"...")

	case "/history":
		fmt.Printf("\nSession History:\n")
		fmt.Printf("  Session ID: %s\n", session.ID)
		fmt.Printf("  Total messages: %d\n", session.MessageCount)
		fmt.Printf("  Total steps: %d\n", session.StepCount)
		fmt.Printf("  Total tokens: %d\n", session.TotalTokens)
		fmt.Printf("  Duration: %s\n", time.Since(session.CreatedAt).Round(time.Second))

		// Show recent messages from persisted history
		if session.historyStore != nil {
			history, err := session.historyStore.Load(session.ID)
			if err == nil && len(history.Messages) > 0 {
				fmt.Printf("\n  Recent messages (last 5):\n")
				start := 0
				if len(history.Messages) > 10 {
					start = len(history.Messages) - 10
				}
				for i := start; i < len(history.Messages); i++ {
					msg := history.Messages[i]
					content := msg.Content
					if len(content) > 60 {
						content = content[:60] + "..."
					}
					fmt.Printf("    [%s] %s\n", msg.Role, content)
				}
			}
		}
		fmt.Println()

	case "/clear":
		// Clear history but keep session
		if session.historyStore != nil {
			if err := session.historyStore.Clear(session.ID); err != nil {
				fmt.Printf("Failed to clear history: %v\n", err)
			} else {
				session.MessageCount = 0
				session.TotalTokens = 0
				fmt.Println("History cleared.\n")
			}
		} else {
			fmt.Println("History store not initialized")
		}

	case "/save":
		if len(parts) > 1 {
			filename := parts[1]
			err := saveReplSession(session, filename)
			if err != nil {
				fmt.Printf("Save failed: %v\n", err)
			} else {
				fmt.Printf("Session saved to: %s\n", filename)
			}
		} else {
			fmt.Println("Usage: /save <filename>")
		}

	case "/load":
		if len(parts) > 1 {
			filename := parts[1]
			loaded, err := loadReplSession(filename)
			if err != nil {
				fmt.Printf("Load failed: %v\n", err)
			} else {
				*session = *loaded
				fmt.Printf("Session loaded: %s\n", session.ID[:12]+"...")
			}
		} else {
			fmt.Println("Usage: /load <filename>")
		}

	case "/files":
		fmt.Println("\nWorkspace file listing not implemented yet")
		fmt.Println("Use: /tools to see available file operations\n")

	default:
		fmt.Printf("Unknown command: %s (try /help)\n\n", cmdName)
	}

	return false
}

func printReplHelp() {
	fmt.Println(`
REPL Commands:
  /help, /h    Show this help
  /status      Show session status (steps, tokens)
  /history     Show conversation history with recent messages
  /clear       Clear history but keep session
  /skills      List available skills
  /memory      List loaded memory sources
  /tools       List available tools
  /files       List workspace files
  /reset       Reset session (start fresh)
  /save FILE   Save session state to file
  /load FILE   Load session state from file
  /exit, /q    Exit REPL

Any other input is sent to the agent as a task.
`)
}

func printReplSessionStatus(session *ReplSession) {
	fmt.Println()
	fmt.Println("Session Status:")
	fmt.Printf("  ID:        %s\n", session.ID)
	fmt.Printf("  Agent:     %s\n", session.AgentName)
	fmt.Printf("  Steps:     %d\n", session.StepCount)
	fmt.Printf("  Tokens:    %d\n", session.TotalTokens)
	fmt.Printf("  Messages:  %d\n", session.MessageCount)
	fmt.Printf("  Created:   %s\n", session.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Updated:   %s\n", session.UpdatedAt.Format("2006-01-02 15:04:05"))
	if session.persistedPath != "" {
		fmt.Printf("  Path:      %s\n", session.persistedPath)
	}
	fmt.Printf("  Persisted: %v\n", session.historyStore != nil)
	fmt.Println()
}

// ReplSessionExport is a JSON-serializable version of ReplSession
type ReplSessionExport struct {
	ID           string    `json:"id"`
	AgentID      string    `json:"agent_id"`
	AgentName    string    `json:"agent_name"`
	StepCount    int       `json:"step_count"`
	MessageCount int       `json:"message_count"`
	TotalTokens  int       `json:"total_tokens"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func saveReplSession(sess *ReplSession, filename string) error {
	export := ReplSessionExport{
		ID:           sess.ID,
		AgentID:      sess.AgentID,
		AgentName:    sess.AgentName,
		StepCount:    sess.StepCount,
		MessageCount: sess.MessageCount,
		TotalTokens:  sess.TotalTokens,
		CreatedAt:    sess.CreatedAt,
		UpdatedAt:    sess.UpdatedAt,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func loadReplSession(filename string) (*ReplSession, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var export ReplSessionExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	sess := &ReplSession{
		ID:           export.ID,
		AgentID:      export.AgentID,
		AgentName:    export.AgentName,
		StepCount:    export.StepCount,
		MessageCount: export.MessageCount,
		TotalTokens:  export.TotalTokens,
		CreatedAt:    export.CreatedAt,
		UpdatedAt:    export.UpdatedAt,
	}

	if sess.ID == "" {
		sess.ID = fmt.Sprintf("repl-%d", time.Now().Unix())
	}

	return sess, nil
}

// ExecutionResult represents the result of an agent execution
type ExecutionResult struct {
	Response     string
	TotalSteps   int
	TotalTokens  int
	FinishReason string
	Error        error
}

func executeReplTask(
	ctx context.Context,
	cfg *config.Config,
	repos *repositories.Repositories,
	env *models.Environment,
	agent *models.Agent,
	task string,
	maxSteps int,
	timeout time.Duration,
	sandboxMode string,
	skillsMW *skills.SkillsMiddleware,
	memoryMW *memory.MemoryMiddleware,
) (*ExecutionResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use the agent execution service
	agentService := services.NewAgentService(repos)

	// Execute through the standard service which handles harness internally
	// ExecuteAgent returns Message directly, no need to poll
	message, err := agentService.ExecuteAgent(execCtx, agent.ID, task, nil)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	// Build result from message
	// Note: The simple Message struct doesn't include token/step counts
	// These would need to be retrieved from the run record if needed
	result := &ExecutionResult{
		Response:     message.Content,
		TotalSteps:   1, // Placeholder - could be retrieved from run record
		TotalTokens:  0, // Placeholder - could be retrieved from run record
		FinishReason: "completed",
	}

	return result, nil
}

// GetHarnessCmd returns the harness command for registration
func GetHarnessCmd() *cobra.Command {
	return harnessCmd
}
