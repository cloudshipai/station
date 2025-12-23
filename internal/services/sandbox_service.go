package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger.io/dagger"
	"station/pkg/dotprompt"
)

type SandboxEngineMode string

const (
	SandboxEngineModeDockerSocket SandboxEngineMode = "docker_socket"
	SandboxEngineModeSidecar      SandboxEngineMode = "sidecar"
	SandboxEngineModeRemote       SandboxEngineMode = "remote"
)

type SandboxServiceConfig struct {
	Enabled           bool
	EngineMode        SandboxEngineMode
	AllowedImages     []string
	DefaultTimeout    time.Duration
	MaxStdoutBytes    int
	MaxStderrBytes    int
	MaxArtifactBytes  int
	AllowNetworkByDef bool
}

func DefaultSandboxConfig() SandboxServiceConfig {
	return SandboxServiceConfig{
		Enabled:           false,
		EngineMode:        SandboxEngineModeDockerSocket,
		AllowedImages:     []string{"python:3.11-slim", "node:20-slim", "ubuntu:22.04"},
		DefaultTimeout:    2 * time.Minute,
		MaxStdoutBytes:    200000,
		MaxStderrBytes:    200000,
		MaxArtifactBytes:  10 * 1024 * 1024,
		AllowNetworkByDef: false,
	}
}

type SandboxRunRequest struct {
	Runtime        string
	Code           string
	Args           []string
	Env            map[string]string
	Files          map[string]string
	TimeoutSeconds int
}

type SandboxArtifact struct {
	Path          string `json:"path"`
	SizeBytes     int    `json:"size_bytes"`
	ContentBase64 string `json:"content_base64,omitempty"`
}

type SandboxRunResult struct {
	OK         bool              `json:"ok"`
	Runtime    string            `json:"runtime"`
	ExitCode   int               `json:"exit_code"`
	DurationMs int64             `json:"duration_ms"`
	Stdout     string            `json:"stdout"`
	Stderr     string            `json:"stderr"`
	Artifacts  []SandboxArtifact `json:"artifacts,omitempty"`
	Error      string            `json:"error,omitempty"`
	Limits     struct {
		TimeoutSeconds int `json:"timeout_seconds"`
		MaxStdoutBytes int `json:"max_stdout_bytes"`
	} `json:"limits"`
}

type SandboxService struct {
	config SandboxServiceConfig
}

func NewSandboxService(cfg SandboxServiceConfig) *SandboxService {
	return &SandboxService{config: cfg}
}

func (s *SandboxService) IsEnabled() bool {
	return s.config.Enabled
}

func (s *SandboxService) Run(ctx context.Context, req SandboxRunRequest) (*SandboxRunResult, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("sandbox service is not enabled")
	}

	start := time.Now()

	if !s.isImageAllowed(req.Runtime) {
		return &SandboxRunResult{
			OK:       false,
			Runtime:  req.Runtime,
			Error:    fmt.Sprintf("runtime %q is not in allowed list", req.Runtime),
			ExitCode: -1,
		}, nil
	}

	timeout := s.config.DefaultTimeout
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := s.executeInDagger(execCtx, req)
	if err != nil {
		return &SandboxRunResult{
			OK:         false,
			Runtime:    req.Runtime,
			Error:      err.Error(),
			ExitCode:   -1,
			DurationMs: time.Since(start).Milliseconds(),
		}, nil
	}

	result.DurationMs = time.Since(start).Milliseconds()
	result.Limits.TimeoutSeconds = int(timeout.Seconds())
	result.Limits.MaxStdoutBytes = s.config.MaxStdoutBytes

	return result, nil
}

func (s *SandboxService) isImageAllowed(runtime string) bool {
	for _, img := range s.config.AllowedImages {
		if img == runtime || runtimeToImage(runtime) == img {
			return true
		}
	}
	return false
}

func runtimeToImage(runtime string) string {
	switch runtime {
	case "python":
		return "python:3.11-slim"
	case "node":
		return "node:20-slim"
	case "bash":
		return "ubuntu:22.04"
	default:
		return runtime
	}
}

func (s *SandboxService) executeInDagger(ctx context.Context, req SandboxRunRequest) (*SandboxRunResult, error) {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(nil))
	if err != nil {
		return nil, fmt.Errorf("dagger connect failed: %w", err)
	}
	defer client.Close()

	image := s.resolveImage(req.Runtime)
	entrypoint, filename := s.resolveEntrypoint(req.Runtime)

	ctr := client.Container().
		From(image).
		WithWorkdir("/work").
		WithNewFile("/work/"+filename, req.Code)

	for path, contents := range req.Files {
		safePath := strings.TrimPrefix(path, "/")
		ctr = ctr.WithNewFile("/work/"+safePath, contents)
	}

	for k, v := range req.Env {
		ctr = ctr.WithEnvVariable(k, v)
	}

	execArgs := append(entrypoint, req.Args...)
	ctr = ctr.WithExec(execArgs)

	stdout, stdoutErr := ctr.Stdout(ctx)
	stderr, _ := ctr.Stderr(ctx)

	exitCode := 0
	if stdoutErr != nil {
		exitCode = 1
		if stderr == "" {
			stderr = stdoutErr.Error()
		}
	}

	stdout = s.truncate(stdout, s.config.MaxStdoutBytes)
	stderr = s.truncate(stderr, s.config.MaxStderrBytes)

	return &SandboxRunResult{
		OK:       exitCode == 0,
		Runtime:  req.Runtime,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}, nil
}

func (s *SandboxService) resolveImage(runtime string) string {
	switch runtime {
	case "python":
		return "python:3.11-slim"
	case "node":
		return "node:20-slim"
	case "bash":
		return "ubuntu:22.04"
	default:
		return runtime
	}
}

func (s *SandboxService) resolveEntrypoint(runtime string) ([]string, string) {
	switch runtime {
	case "python":
		return []string{"python", "/work/main.py"}, "main.py"
	case "node":
		return []string{"node", "/work/main.js"}, "main.js"
	case "bash":
		return []string{"bash", "/work/main.sh"}, "main.sh"
	default:
		return []string{"python", "/work/main.py"}, "main.py"
	}
}

func (s *SandboxService) truncate(s1 string, maxBytes int) string {
	if len(s1) <= maxBytes {
		return s1
	}
	return s1[:maxBytes] + "\n... [truncated]"
}

func (s *SandboxService) MergeDefaults(agentSandbox *dotprompt.SandboxConfig) SandboxRunRequest {
	req := SandboxRunRequest{
		Runtime:        "python",
		TimeoutSeconds: int(s.config.DefaultTimeout.Seconds()),
	}

	if agentSandbox != nil {
		if agentSandbox.Runtime != "" {
			req.Runtime = agentSandbox.Runtime
		}
		if agentSandbox.TimeoutSeconds > 0 {
			req.TimeoutSeconds = agentSandbox.TimeoutSeconds
		}
	}

	return req
}
