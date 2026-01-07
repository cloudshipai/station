package harness_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"station/internal/config"
	"station/internal/genkit/anthropic_oauth"
	"station/internal/logging"
	"station/pkg/harness"
	"station/pkg/harness/tools"
	"station/pkg/harness/workspace"

	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
)

func TestAgenticExecutor_E2E_RealLLM(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	configDir := config.GetStationConfigDir()
	configPath := configDir + "/config.yaml"
	t.Logf("Looking for config in: %s", configDir)

	if _, err := os.Stat(configPath); err != nil {
		t.Logf("Config file not found at %s: %v", configPath, err)
	} else {
		t.Logf("Config file exists at %s", configPath)
	}

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	t.Logf("Loaded config - AIProvider: '%s', AIModel: '%s', AIAuthType: '%s', OAuthToken length: %d",
		cfg.AIProvider, cfg.AIModel, cfg.AIAuthType, len(cfg.AIOAuthToken))

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 3,
		Timeout:           2 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	task := "Create a file called 'hello.txt' with the content 'Hello from the agentic harness!' then read it back and confirm the content."

	t.Logf("Executing task: %s", task)
	startTime := time.Now()

	result, err := executor.Execute(ctx, "e2e-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	helloPath := tmpDir + "/hello.txt"
	if _, err := os.Stat(helloPath); os.IsNotExist(err) {
		t.Error("Expected hello.txt to be created")
	} else {
		content, _ := os.ReadFile(helloPath)
		t.Logf("Created file content: %s", string(content))
	}
}

func TestAgenticExecutor_E2E_MultiStep(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          15,
		DoomLoopThreshold: 3,
		Timeout:           3 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	task := `Do the following steps:
1. Create a directory called 'project'
2. Inside 'project', create a file called 'config.json' with content: {"name": "test", "version": "1.0"}
3. List all files in the workspace to confirm
4. Read the config.json file and tell me what version it contains`

	t.Logf("Executing multi-step task")
	startTime := time.Now()

	result, err := executor.Execute(ctx, "e2e-multi-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	configPath := tmpDir + "/project/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected project/config.json to be created")
	}
}

func initGenkit(ctx context.Context, cfg *config.Config) (*genkit.Genkit, error) {
	os.Setenv("OTEL_SDK_DISABLED", "true")

	promptDir := "/tmp/harness-e2e-prompts"
	os.MkdirAll(promptDir, 0755)

	provider := strings.ToLower(cfg.AIProvider)

	if strings.HasPrefix(strings.ToLower(cfg.AIModel), "claude") {
		provider = "anthropic"
	} else if strings.HasPrefix(strings.ToLower(cfg.AIModel), "gemini") {
		provider = "gemini"
	} else if strings.HasPrefix(strings.ToLower(cfg.AIModel), "gpt") {
		provider = "openai"
	}

	logging.Info("Initializing GenKit for e2e test with provider: %s, model: %s", provider, cfg.AIModel)

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

func formatModelName(provider, model string) string {
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

func TestAgenticExecutor_E2E_Compaction(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-compaction-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	harnessConfig.Compaction.Enabled = true
	harnessConfig.Compaction.Threshold = 0.3
	harnessConfig.Compaction.ProtectTokens = 200

	agentConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 5,
		Timeout:           3 * time.Minute,
	}

	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	compactor := harness.NewCompactor(genkitApp, modelName, harnessConfig.Compaction, 1000)

	executor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		agentConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
		harness.WithCompactor(compactor),
	)

	task := `Create two files: 'data.txt' with content 'Important data here' and 'config.json' with content '{"version": "1.0"}'. Then read both files back and summarize what they contain.`

	t.Logf("Executing compaction test task with context window=1000, threshold=0.3")
	startTime := time.Now()

	result, err := executor.Execute(ctx, "e2e-compaction-test", task, toolRegistry.All())

	t.Logf("Execution completed in %v", time.Since(startTime))
	t.Logf("Result: success=%v, steps=%d, tokens=%d, finish_reason=%s",
		result.Success, result.TotalSteps, result.TotalTokens, result.FinishReason)

	if err != nil {
		t.Logf("Execution error: %v", err)
	}

	if result.Response != "" {
		t.Logf("Response:\n%s", result.Response)
	}

	if !result.Success {
		t.Errorf("Expected successful execution, got error: %s", result.Error)
	}

	dataPath := tmpDir + "/data.txt"
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("Expected data.txt to be created")
	}

	configPath := tmpDir + "/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Expected config.json to be created")
	}

	t.Logf("Compaction test completed - context window was limited to 1000 tokens with 0.3 threshold")
}

func TestAgenticExecutor_E2E_WorkflowSimulation(t *testing.T) {
	if os.Getenv("HARNESS_E2E_TEST") != "1" {
		t.Skip("Skipping e2e test. Set HARNESS_E2E_TEST=1 to run with real LLM")
	}

	ctx := context.Background()

	if err := config.InitViper(""); err != nil {
		t.Fatalf("Failed to init viper: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	genkitApp, err := initGenkit(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize GenKit: %v", err)
	}

	tmpDir, err := os.MkdirTemp("", "harness-e2e-workflow-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ws := workspace.NewHostWorkspace(tmpDir)
	if err := ws.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	toolRegistry := tools.NewToolRegistry(genkitApp, tmpDir)
	if err := toolRegistry.RegisterBuiltinTools(); err != nil {
		t.Fatalf("Failed to register tools: %v", err)
	}

	harnessConfig := harness.DefaultHarnessConfig()
	modelName := formatModelName(cfg.AIProvider, cfg.AIModel)

	t.Log("=== Simulating Multi-Agent Workflow with Harness ===")
	t.Log("Step 1: Setup Agent - Create project structure")

	setupConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 3,
		Timeout:           2 * time.Minute,
	}

	setupExecutor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		setupConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	setupTask := "Create a directory called 'test-project' and inside it create a file called 'config.json' with content '{\"name\": \"test\", \"version\": \"1.0\"}'"

	setupResult, err := setupExecutor.Execute(ctx, "setup-agent", setupTask, toolRegistry.All())
	if err != nil {
		t.Fatalf("Setup agent failed: %v", err)
	}

	t.Logf("Setup agent: success=%v, steps=%d, tokens=%d", setupResult.Success, setupResult.TotalSteps, setupResult.TotalTokens)

	if !setupResult.Success {
		t.Fatalf("Setup agent should succeed: %s", setupResult.Error)
	}

	configPath := tmpDir + "/test-project/config.json"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Setup agent should have created test-project/config.json")
	}

	t.Log("Step 2: Verify Agent - Check created files")

	verifyConfig := &harness.AgentHarnessConfig{
		MaxSteps:          10,
		DoomLoopThreshold: 3,
		Timeout:           2 * time.Minute,
	}

	verifyExecutor := harness.NewAgenticExecutor(
		genkitApp,
		harnessConfig,
		verifyConfig,
		harness.WithWorkspace(ws),
		harness.WithModelName(modelName),
	)

	verifyTask := "List all files in the 'test-project' directory and read config.json. Report the version number found."

	verifyResult, err := verifyExecutor.Execute(ctx, "verify-agent", verifyTask, toolRegistry.All())
	if err != nil {
		t.Fatalf("Verify agent failed: %v", err)
	}

	t.Logf("Verify agent: success=%v, steps=%d, tokens=%d", verifyResult.Success, verifyResult.TotalSteps, verifyResult.TotalTokens)
	t.Logf("Verify response: %s", verifyResult.Response)

	if !verifyResult.Success {
		t.Fatalf("Verify agent should succeed: %s", verifyResult.Error)
	}

	if !strings.Contains(verifyResult.Response, "1.0") {
		t.Error("Verify agent should report version 1.0")
	}

	t.Log("=== Workflow Simulation Complete ===")
	t.Logf("Total steps: Setup=%d, Verify=%d", setupResult.TotalSteps, verifyResult.TotalSteps)
	t.Logf("Total tokens: Setup=%d, Verify=%d", setupResult.TotalTokens, verifyResult.TotalTokens)
}
