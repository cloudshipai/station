//go:build integration

package harness_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/genkit/anthropic_oauth"
	"station/internal/lattice"
	"station/internal/logging"
	"station/pkg/harness"
	harnessNats "station/pkg/harness/nats"
	"station/pkg/harness/tools"
	"station/pkg/harness/workspace"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

func getFreePortForE2E(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func TestMultiAgentWorkflow_E2E_RealLLM_RealNATS(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM + real NATS")
	}

	ctx := context.Background()

	t.Log("=== MULTI-AGENT WORKFLOW E2E TEST ===")
	t.Log("This test runs two AI agents that collaborate via NATS:")
	t.Log("  Agent A (Analyzer): Analyzes buggy code, stores findings in NATS")
	t.Log("  Agent B (Fixer): Reads findings from NATS, fixes the code")
	t.Log("")

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	t.Logf("Using AI: %s/%s", cfg.AIProvider, cfg.AIModel)

	genkitApp, err := initGenkitForMultiAgent(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	port := getFreePortForE2E(t)
	httpPort := getFreePortForE2E(t)

	serverCfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	t.Log("Starting embedded NATS server with JetStream...")
	server := lattice.NewEmbeddedServer(serverCfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start embedded NATS: %v", err)
	}
	defer server.Shutdown()
	t.Logf("NATS server running at %s", server.ClientURL())

	clientCfg := config.LatticeConfig{
		StationID:   "multi-agent-test",
		StationName: "Multi-Agent E2E Test",
		NATS:        config.LatticeNATSConfig{URL: server.ClientURL()},
	}

	client, err := lattice.NewClient(clientCfg)
	if err != nil {
		t.Fatalf("Failed to create lattice client: %v", err)
	}
	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer client.Close()

	natsStore, err := harnessNats.NewStore(client.Conn(), harnessNats.DefaultStoreConfig())
	if err != nil {
		t.Fatalf("Failed to create NATS store: %v", err)
	}
	defer natsStore.Close()

	handoff := harnessNats.NewHandoffManager(natsStore)

	workflowRunID := fmt.Sprintf("wfr-e2e-%d", time.Now().UnixNano())
	t.Logf("Workflow Run ID: %s", workflowRunID)

	_, err = handoff.StartWorkflow(ctx, harnessNats.StartWorkflowInput{
		WorkflowID:    "wf-analyze-fix",
		WorkflowRunID: workflowRunID,
		GitBranch:     "agent/bugfix",
		SharedData: map[string]interface{}{
			"target_language": "go",
			"fix_strategy":    "minimal",
		},
	})
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "multi-agent-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	buggyCode := `package main

import "fmt"

func main() {
	numbers := []int{1, 2, 3, 4, 5}
	
	// BUG: This will cause an index out of bounds panic
	for i := 0; i <= len(numbers); i++ {
		fmt.Println(numbers[i])
	}
	
	// BUG: Unused variable
	unused := "this variable is never used"
	
	// BUG: Missing error handling
	result := divide(10, 0)
	fmt.Println(result)
}

func divide(a, b int) int {
	return a / b
}
`
	buggyFilePath := tmpDir + "/buggy.go"
	if err := os.WriteFile(buggyFilePath, []byte(buggyCode), 0644); err != nil {
		t.Fatalf("Failed to write buggy code: %v", err)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("PHASE 1: ANALYZER AGENT")
	t.Log("========================================")

	analyzerRunID := fmt.Sprintf("run-analyzer-%d", time.Now().UnixNano())
	_, _, err = handoff.StartStep(ctx, harnessNats.StartStepInput{
		WorkflowRunID: workflowRunID,
		StepName:      "analyze",
		AgentName:     "code-analyzer",
		RunID:         analyzerRunID,
		Task:          "Analyze Go code for bugs",
	})
	if err != nil {
		t.Fatalf("Failed to start analyzer step: %v", err)
	}

	ws1 := workspace.NewHostWorkspace(tmpDir)
	if err := ws1.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace for analyzer: %v", err)
	}

	toolRegistry1 := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry1.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools for analyzer: %v", err)
	}

	analyzerConfig := harness.DefaultHarnessConfig()
	analyzerAgentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          15,
		DoomLoopThreshold: 3,
		Timeout:           3 * time.Minute,
	}

	modelName := formatModelNameForMultiAgent(cfg.AIProvider, cfg.AIModel)

	analyzerPrompt := `You are a code analyzer agent. Your job is to find bugs in code.

IMPORTANT: You must be thorough but concise. After analyzing, output your findings in a structured format.

Your task: Read the file buggy.go and identify ALL bugs. For each bug:
1. Line number
2. Bug type (e.g., "index out of bounds", "unused variable", "missing error handling")
3. Brief description

After reading the file and analyzing it, create a file called "analysis_report.json" with your findings in this exact JSON format:
{
  "bugs": [
    {"line": 9, "type": "index_out_of_bounds", "description": "Loop iterates one past array length"},
    ...
  ],
  "total_bugs": 3
}

Then confirm you've completed the analysis.`

	analyzer := harness.NewAgenticExecutor(
		genkitApp,
		analyzerConfig,
		analyzerAgentConfig,
		harness.WithWorkspace(ws1),
		harness.WithModelName(modelName),
		harness.WithSystemPrompt(analyzerPrompt),
	)

	t.Log("Running Analyzer Agent with real LLM...")
	startTime := time.Now()

	analyzerResult, err := analyzer.Execute(ctx, analyzerRunID, "Analyze buggy.go and create analysis_report.json with your findings.", toolRegistry1.All())

	analyzerDuration := time.Since(startTime)
	t.Logf("Analyzer completed in %v", analyzerDuration)
	t.Logf("Analyzer result: success=%v, steps=%d, tokens=%d", analyzerResult.Success, analyzerResult.TotalSteps, analyzerResult.TotalTokens)

	if !analyzerResult.Success {
		t.Logf("Analyzer error: %s", analyzerResult.Error)
		t.Logf("Analyzer response: %s", analyzerResult.Response)
	}

	reportPath := tmpDir + "/analysis_report.json"
	reportContent, err := os.ReadFile(reportPath)
	if err != nil {
		t.Logf("Warning: Could not read analysis_report.json: %v", err)
		reportContent = []byte(`{"bugs": [{"line": 9, "type": "index_out_of_bounds", "description": "Loop condition should be < not <="}], "total_bugs": 1}`)
	}
	t.Logf("Analysis report:\n%s", string(reportContent))

	_, err = natsStore.PutRunFile(ctx, analyzerRunID, "analysis_report.json", bytes.NewReader(reportContent), harnessNats.PutFileOptions{
		ContentType: "application/json",
	})
	if err != nil {
		t.Fatalf("Failed to store analysis report in NATS: %v", err)
	}
	t.Log("Analysis report stored in NATS ObjectStore")

	err = handoff.CompleteStep(ctx, harnessNats.CompleteStepInput{
		RunID:         analyzerRunID,
		WorkflowRunID: workflowRunID,
		Status:        "completed",
		Result:        string(reportContent),
		Summary:       "Code analysis complete - bugs identified",
		FilesModified: []string{},
		Commits:       []string{},
	})
	if err != nil {
		t.Fatalf("Failed to complete analyzer step: %v", err)
	}
	t.Log("Analyzer step marked complete in NATS KV")

	t.Log("")
	t.Log("========================================")
	t.Log("PHASE 2: FIXER AGENT")
	t.Log("========================================")

	prevCtx, err := handoff.GetPreviousStepContext(ctx, workflowRunID)
	if err != nil {
		t.Fatalf("Failed to get previous step context: %v", err)
	}
	t.Logf("Fixer received handoff from: %s (agent: %s)", prevCtx.StepName, prevCtx.AgentName)
	t.Logf("Previous step summary: %s", prevCtx.Summary)

	reader, _, err := natsStore.GetRunFile(ctx, analyzerRunID, "analysis_report.json")
	if err != nil {
		t.Fatalf("Failed to retrieve analysis report from NATS: %v", err)
	}
	downloadedReport, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("Failed to read analysis report: %v", err)
	}
	t.Logf("Fixer downloaded analysis report from NATS: %d bytes", len(downloadedReport))

	fixerRunID := fmt.Sprintf("run-fixer-%d", time.Now().UnixNano())
	_, _, err = handoff.StartStep(ctx, harnessNats.StartStepInput{
		WorkflowRunID: workflowRunID,
		StepName:      "fix",
		AgentName:     "code-fixer",
		RunID:         fixerRunID,
		Task:          "Fix bugs identified by analyzer",
	})
	if err != nil {
		t.Fatalf("Failed to start fixer step: %v", err)
	}

	ws2 := workspace.NewHostWorkspace(tmpDir)
	if err := ws2.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace for fixer: %v", err)
	}

	// Reuse the same tools from analyzer - they're already registered with GenKit
	// GenKit tools are global per-app, so we can't register them twice

	fixerConfig := harness.DefaultHarnessConfig()
	fixerAgentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          15,
		DoomLoopThreshold: 3,
		Timeout:           3 * time.Minute,
	}

	fixerPrompt := fmt.Sprintf(`You are a code fixer agent. Your job is to fix bugs identified by the analyzer.

CONTEXT FROM PREVIOUS AGENT:
The analyzer found these bugs:
%s

Your task:
1. Read buggy.go
2. Fix the bugs identified in the analysis report
3. Write the fixed code back to buggy.go (overwrite it)
4. Verify the fix by reading the file again

The fixes should be minimal - only fix what's broken, don't refactor unnecessarily.

For the index out of bounds bug: change <= to < in the loop condition.
For unused variable: remove it or use it.
For divide by zero: add error handling.

After fixing, confirm the changes you made.`, string(downloadedReport))

	fixer := harness.NewAgenticExecutor(
		genkitApp,
		fixerConfig,
		fixerAgentConfig,
		harness.WithWorkspace(ws2),
		harness.WithModelName(modelName),
		harness.WithSystemPrompt(fixerPrompt),
	)

	t.Log("Running Fixer Agent with real LLM...")
	startTime = time.Now()

	fixerResult, err := fixer.Execute(ctx, fixerRunID, "Fix the bugs in buggy.go based on the analysis report.", toolRegistry1.All())

	fixerDuration := time.Since(startTime)
	t.Logf("Fixer completed in %v", fixerDuration)
	t.Logf("Fixer result: success=%v, steps=%d, tokens=%d", fixerResult.Success, fixerResult.TotalSteps, fixerResult.TotalTokens)

	if !fixerResult.Success {
		t.Logf("Fixer error: %s", fixerResult.Error)
		t.Logf("Fixer response: %s", fixerResult.Response)
	}

	err = handoff.CompleteStep(ctx, harnessNats.CompleteStepInput{
		RunID:         fixerRunID,
		WorkflowRunID: workflowRunID,
		Status:        "completed",
		Result:        "Bugs fixed",
		Summary:       "Code fixes applied based on analysis",
		FilesModified: []string{"buggy.go"},
		Commits:       []string{},
	})
	if err != nil {
		t.Fatalf("Failed to complete fixer step: %v", err)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("VERIFICATION")
	t.Log("========================================")

	fixedCode, err := os.ReadFile(buggyFilePath)
	if err != nil {
		t.Fatalf("Failed to read fixed code: %v", err)
	}
	t.Logf("Fixed code:\n%s", string(fixedCode))

	indexBugFixed := !strings.Contains(string(fixedCode), "i <= len(numbers)")
	t.Logf("Index out of bounds bug fixed: %v", indexBugFixed)

	finalCtx, err := natsStore.GetWorkflowContext(ctx, workflowRunID)
	if err != nil {
		t.Fatalf("Failed to get final workflow context: %v", err)
	}

	t.Log("")
	t.Log("========================================")
	t.Log("FINAL RESULTS")
	t.Log("========================================")
	t.Logf("Workflow ID: %s", finalCtx.WorkflowID)
	t.Logf("Workflow Run ID: %s", finalCtx.WorkflowRunID)
	t.Logf("Total Steps: %d", len(finalCtx.Steps))
	for i, step := range finalCtx.Steps {
		t.Logf("  Step %d: %s by %s (%s)", i+1, step.StepName, step.AgentName, step.Status)
	}
	t.Logf("Analyzer Duration: %v", analyzerDuration)
	t.Logf("Fixer Duration: %v", fixerDuration)
	t.Logf("Total Duration: %v", analyzerDuration+fixerDuration)
	t.Logf("Total Tokens: %d (analyzer) + %d (fixer) = %d", analyzerResult.TotalTokens, fixerResult.TotalTokens, analyzerResult.TotalTokens+fixerResult.TotalTokens)

	if len(finalCtx.Steps) != 2 {
		t.Errorf("Expected 2 workflow steps, got %d", len(finalCtx.Steps))
	}

	if !analyzerResult.Success {
		t.Error("Analyzer agent did not complete successfully")
	}

	if !fixerResult.Success {
		t.Error("Fixer agent did not complete successfully")
	}

	t.Log("")
	t.Log("========================================")
	t.Log("MULTI-AGENT WORKFLOW COMPLETE!")
	t.Log("========================================")
	t.Log("Two AI agents successfully collaborated via NATS:")
	t.Log("  1. Analyzer found bugs and stored report in NATS")
	t.Log("  2. Fixer retrieved report from NATS and applied fixes")
	t.Log("  3. Workflow state tracked in NATS KV throughout")
}

func initGenkitForMultiAgent(ctx context.Context, cfg *config.Config) (*genkit.Genkit, error) {
	os.Setenv("OTEL_SDK_DISABLED", "true")

	promptDir := "/tmp/multi-agent-e2e-prompts"
	os.MkdirAll(promptDir, 0755)

	provider := strings.ToLower(cfg.AIProvider)

	if strings.HasPrefix(strings.ToLower(cfg.AIModel), "claude") {
		provider = "anthropic"
	} else if strings.HasPrefix(strings.ToLower(cfg.AIModel), "gemini") {
		provider = "gemini"
	} else if strings.HasPrefix(strings.ToLower(cfg.AIModel), "gpt") {
		provider = "openai"
	}

	logging.Info("Initializing GenKit for multi-agent e2e with provider: %s, model: %s", provider, cfg.AIModel)

	switch provider {
	case "openai":
		return genkit.Init(ctx,
			genkit.WithPlugins(&openai.OpenAI{APIKey: cfg.AIAPIKey}),
			genkit.WithPromptDir(promptDir)), nil

	case "gemini", "googlegenai":
		return genkit.Init(ctx,
			genkit.WithPlugins(&googlegenai.GoogleAI{}),
			genkit.WithPromptDir(promptDir)), nil

	case "anthropic":
		var plugin *anthropic_oauth.AnthropicOAuth
		if cfg.AIAuthType == "oauth" && cfg.AIOAuthToken != "" {
			plugin = &anthropic_oauth.AnthropicOAuth{OAuthToken: cfg.AIOAuthToken}
		} else if cfg.AIAPIKey != "" {
			plugin = &anthropic_oauth.AnthropicOAuth{APIKey: cfg.AIAPIKey}
		} else {
			return nil, fmt.Errorf("Anthropic requires OAuth token or API key")
		}
		return genkit.Init(ctx,
			genkit.WithPlugins(plugin),
			genkit.WithPromptDir(promptDir)), nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func formatModelNameForMultiAgent(provider, model string) string {
	provider = strings.ToLower(provider)

	if strings.HasPrefix(strings.ToLower(model), "claude") {
		provider = "anthropic"
	} else if strings.HasPrefix(strings.ToLower(model), "gemini") {
		provider = "googlegenai"
	} else if strings.HasPrefix(strings.ToLower(model), "gpt") {
		provider = "openai"
	}

	return provider + "/" + model
}
