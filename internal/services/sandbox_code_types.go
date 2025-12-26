package services

import (
	"fmt"
	"time"
)

type SandboxMode string

const (
	SandboxModeCompute SandboxMode = "compute"
	SandboxModeCode    SandboxMode = "code"
)

type SessionScope string

const (
	SessionScopeWorkflow SessionScope = "workflow"
	SessionScopeAgent    SessionScope = "agent"
)

type SessionKey struct {
	Scope   SessionScope
	ScopeID string
}

func (k SessionKey) String() string {
	return fmt.Sprintf("%s_%s_sandbox", k.Scope, k.ScopeID)
}

func NewWorkflowSessionKey(workflowRunID string) SessionKey {
	return SessionKey{Scope: SessionScopeWorkflow, ScopeID: workflowRunID}
}

func NewAgentSessionKey(agentRunID string) SessionKey {
	return SessionKey{Scope: SessionScopeAgent, ScopeID: agentRunID}
}

type ExecutionContext struct {
	WorkflowRunID string
	AgentRunID    string
	AgentName     string
	Environment   string
}

func (c ExecutionContext) SessionKey(scope SessionScope) SessionKey {
	switch scope {
	case SessionScopeWorkflow:
		if c.WorkflowRunID != "" {
			return NewWorkflowSessionKey(c.WorkflowRunID)
		}
		return NewAgentSessionKey(c.AgentRunID)
	default:
		return NewAgentSessionKey(c.AgentRunID)
	}
}

type SessionConfig struct {
	Runtime        string
	Image          string
	AllowNetwork   bool
	TimeoutSeconds int
	Workdir        string
	Env            map[string]string
}

type CodeSession struct {
	ID          string
	ContainerID string
	Key         SessionKey
	Config      SessionConfig
	Status      SessionStatus
	CreatedAt   time.Time
	LastUsedAt  time.Time
}

type SessionStatus string

const (
	SessionStatusCreating  SessionStatus = "creating"
	SessionStatusReady     SessionStatus = "ready"
	SessionStatusBusy      SessionStatus = "busy"
	SessionStatusError     SessionStatus = "error"
	SessionStatusDestroyed SessionStatus = "destroyed"
)

type ExecRequest struct {
	Command        string
	Args           []string
	Workdir        string
	Env            map[string]string
	TimeoutSeconds int
}

type ExecResult struct {
	ExitCode   int
	Stdout     string
	Stderr     string
	DurationMs int64
	TimedOut   bool
}

type FileEntry struct {
	Name      string
	Path      string
	Type      FileType
	SizeBytes int64
	ModTime   time.Time
}

type FileType string

const (
	FileTypeFile      FileType = "file"
	FileTypeDirectory FileType = "directory"
	FileTypeSymlink   FileType = "symlink"
)

type CodeModeLimits struct {
	TimeoutSeconds   int
	MaxFileSizeBytes int64
	MaxFiles         int
	MaxStdoutBytes   int
}

func DefaultCodeModeLimits() CodeModeLimits {
	return CodeModeLimits{
		TimeoutSeconds:   300,
		MaxFileSizeBytes: 10 * 1024 * 1024,
		MaxFiles:         100,
		MaxStdoutBytes:   200000,
	}
}

func RuntimeToDefaultImage(runtime string) string {
	switch runtime {
	case "python":
		return "python:3.11-slim"
	case "node":
		return "node:20-slim"
	case "linux", "bash", "shell", "":
		return "ubuntu:22.04"
	default:
		// Allow custom images to be specified directly
		if runtime != "" {
			return runtime
		}
		return "ubuntu:22.04"
	}
}
