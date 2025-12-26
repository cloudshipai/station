package services

import (
	"encoding/json"
	"fmt"

	"station/pkg/dotprompt"

	"github.com/firebase/genkit/go/ai"
)

type SandboxToolFactory struct {
	service *SandboxService
}

func NewSandboxToolFactory(svc *SandboxService) *SandboxToolFactory {
	return &SandboxToolFactory{service: svc}
}

func (f *SandboxToolFactory) CreateTool(agentDefaults *dotprompt.SandboxConfig) ai.Tool {
	defaults := f.service.MergeDefaults(agentDefaults)

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
		result, err := f.service.Run(toolCtx.Context, req)
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

func (f *SandboxToolFactory) ShouldAddTool(sandbox *dotprompt.SandboxConfig) bool {
	return sandbox != nil && f.service.IsEnabled()
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
	backend SandboxBackend,
	codeModeEnabled bool,
) *UnifiedSandboxFactory {
	var codeFactory *CodeModeToolFactory
	if codeModeEnabled && sessionMgr != nil && backend != nil {
		codeFactory = NewCodeModeToolFactory(sessionMgr, backend)
	}

	return &UnifiedSandboxFactory{
		computeFactory:  NewSandboxToolFactory(computeSvc),
		codeFactory:     codeFactory,
		codeModeEnabled: codeModeEnabled,
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
		return f.codeFactory.CreateTools(execCtx, sandboxCfg)
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
