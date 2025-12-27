package services

import (
	"encoding/json"
	"fmt"

	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

type SandboxToolFactory struct {
	computeService  *SandboxService
	codeToolFactory *CodeModeToolFactory
}

func NewSandboxToolFactory(svc *SandboxService) *SandboxToolFactory {
	return &SandboxToolFactory{computeService: svc}
}

func NewSandboxToolFactoryWithCodeMode(computeSvc *SandboxService, codeSvc *CodeModeToolFactory) *SandboxToolFactory {
	return &SandboxToolFactory{
		computeService:  computeSvc,
		codeToolFactory: codeSvc,
	}
}

func (f *SandboxToolFactory) CreateComputeTool(agentDefaults *dotprompt.SandboxConfig) ai.Tool {
	defaults := f.computeService.MergeDefaults(agentDefaults)

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"runtime": map[string]any{
				"type":        "string",
				"enum":        []string{"python", "node", "bash"},
				"description": "Runtime environment for code execution",
				"default":     defaults.Runtime,
			},
			"code": map[string]any{
				"type":        "string",
				"description": "Source code to execute in the sandbox",
			},
			"args": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Command-line arguments to pass to the script",
			},
			"env": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"description":          "Environment variables to set",
			},
			"files": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"description":          "Map of path -> file contents to create in /work directory",
			},
			"timeout_seconds": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     3600,
				"description": "Execution timeout in seconds",
				"default":     defaults.TimeoutSeconds,
			},
		},
		"required": []string{"code"},
	}

	toolFunc := func(toolCtx *ai.ToolContext, input any) (any, error) {
		inputMap, ok := input.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("sandbox_run: expected map[string]any input, got %T", input)
		}
		req := f.parseRequest(inputMap, defaults)
		result, err := f.computeService.Run(toolCtx.Context, req)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	return ai.NewToolWithInputSchema(
		"sandbox_run",
		"Execute code in an isolated Dagger container sandbox. Use this for data processing, parsing, computations, and transformations that are too complex or large for LLM reasoning.",
		schema,
		toolFunc,
	)
}

func (f *SandboxToolFactory) parseRequest(input map[string]any, defaults SandboxRunRequest) SandboxRunRequest {
	req := SandboxRunRequest{
		Runtime:        defaults.Runtime,
		TimeoutSeconds: defaults.TimeoutSeconds,
		PipPackages:    defaults.PipPackages,
		NpmPackages:    defaults.NpmPackages,
	}

	if v, ok := input["runtime"].(string); ok && v != "" {
		req.Runtime = v
	}
	if v, ok := input["code"].(string); ok {
		req.Code = v
	}
	if v, ok := input["args"].([]any); ok {
		for _, arg := range v {
			if s, ok := arg.(string); ok {
				req.Args = append(req.Args, s)
			}
		}
	}
	if v, ok := input["env"].(map[string]any); ok {
		req.Env = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				req.Env[k] = s
			}
		}
	}
	if v, ok := input["files"].(map[string]any); ok {
		req.Files = make(map[string]string)
		for k, val := range v {
			if s, ok := val.(string); ok {
				req.Files[k] = s
			}
		}
	}
	if v, ok := input["timeout_seconds"].(float64); ok {
		req.TimeoutSeconds = int(v)
	}
	if v, ok := input["timeout_seconds"].(int); ok {
		req.TimeoutSeconds = v
	}

	return req
}

func (f *SandboxToolFactory) ShouldAddComputeTool(sandbox *dotprompt.SandboxConfig) bool {
	if sandbox == nil || f.computeService == nil {
		return false
	}
	mode := sandbox.Mode
	if mode == "" {
		mode = "compute"
	}
	return mode == "compute" && f.computeService.IsEnabled()
}

func (f *SandboxToolFactory) ShouldAddCodeTools(sandbox *dotprompt.SandboxConfig) bool {
	if sandbox == nil || f.codeToolFactory == nil {
		return false
	}
	return sandbox.Mode == "code" && f.codeToolFactory.IsEnabled()
}

func (f *SandboxToolFactory) CreateCodeTools(agentDefaults *dotprompt.SandboxConfig, execCtx ExecutionContext) []ai.Tool {
	if f.codeToolFactory == nil {
		return nil
	}
	return f.codeToolFactory.CreateAllTools(agentDefaults, execCtx)
}

func (f *SandboxToolFactory) GetSandboxTools(sandbox *dotprompt.SandboxConfig, execCtx ExecutionContext) []ai.Tool {
	if sandbox == nil {
		return nil
	}

	mode := sandbox.Mode
	if mode == "" {
		mode = "compute"
	}

	switch mode {
	case "compute":
		if f.ShouldAddComputeTool(sandbox) {
			return []ai.Tool{f.CreateComputeTool(sandbox)}
		}
	case "code":
		if f.ShouldAddCodeTools(sandbox) {
			return f.CreateCodeTools(sandbox, execCtx)
		}
	}

	return nil
}

func (f *SandboxToolFactory) ShouldAddTool(sandbox *dotprompt.SandboxConfig) bool {
	return f.ShouldAddComputeTool(sandbox)
}

func (f *SandboxToolFactory) CreateTool(agentDefaults *dotprompt.SandboxConfig) ai.Tool {
	return f.CreateComputeTool(agentDefaults)
}

func marshalResult(result *SandboxRunResult) (string, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sandbox result: %w", err)
	}
	return string(data), nil
}

type UnifiedSandboxFactory struct {
	computeFactory  *SandboxToolFactory
	codeFactory     *CodeModeToolFactory
	codeModeEnabled bool
}

func NewUnifiedSandboxFactory(
	computeSvc *SandboxService,
	sessionMgr *SessionManager,
	codeModeConfig CodeModeConfig,
) *UnifiedSandboxFactory {
	var codeFactory *CodeModeToolFactory
	if codeModeConfig.Enabled && sessionMgr != nil {
		codeFactory = NewCodeModeToolFactory(sessionMgr, codeModeConfig)
	}

	return &UnifiedSandboxFactory{
		computeFactory:  NewSandboxToolFactory(computeSvc),
		codeFactory:     codeFactory,
		codeModeEnabled: codeModeConfig.Enabled,
	}
}

func (f *UnifiedSandboxFactory) GetSandboxTools(
	sandboxCfg *dotprompt.SandboxConfig,
	execCtx ExecutionContext,
) []ai.Tool {
	if sandboxCfg == nil {
		return nil
	}

	if sandboxCfg.Mode == "code" && f.codeFactory != nil {
		return f.codeFactory.CreateAllTools(sandboxCfg, execCtx)
	}

	if f.computeFactory.ShouldAddTool(sandboxCfg) {
		return []ai.Tool{f.computeFactory.CreateTool(sandboxCfg)}
	}

	return nil
}

func (f *UnifiedSandboxFactory) IsCodeMode(sandboxCfg *dotprompt.SandboxConfig) bool {
	return sandboxCfg != nil && sandboxCfg.Mode == "code"
}

func (f *UnifiedSandboxFactory) ShouldAddTools(sandboxCfg *dotprompt.SandboxConfig) bool {
	if sandboxCfg == nil {
		return false
	}

	if sandboxCfg.Mode == "code" {
		return f.codeModeEnabled && f.codeFactory != nil
	}

	return f.computeFactory.ShouldAddTool(sandboxCfg)
}
