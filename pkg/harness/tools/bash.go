package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"station/pkg/harness/sandbox"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// dangerousPatterns defines regex patterns for commands that should be blocked
var dangerousPatterns = []*regexp.Regexp{
	// Destructive file operations
	regexp.MustCompile(`(?i)rm\s+(-[rRfF]+\s+)*(/|/\*|~|~/\*|\$HOME)`),
	regexp.MustCompile(`(?i)rm\s+(-[rRfF]+\s+)*\.\./`),
	regexp.MustCompile(`(?i)mkfs`),
	regexp.MustCompile(`(?i)dd\s+.*of=/dev/`),

	// System modification
	regexp.MustCompile(`(?i)chmod\s+(-R\s+)?[0-7]*777.*(/|~)`),
	regexp.MustCompile(`(?i)chown\s+(-R\s+)?.*(/|~)`),

	// Fork bomb patterns
	regexp.MustCompile(`:\(\)\{\s*:\|:\s*&\s*\};:`),
	regexp.MustCompile(`fork\s+while`),

	// Sensitive file access
	regexp.MustCompile(`(?i)cat\s+/etc/(passwd|shadow)`),
	regexp.MustCompile(`(?i)(cat|head|tail|less|more)\s+.*\.(pem|key|crt|p12)`),

	// Remote code execution
	regexp.MustCompile(`(?i)curl\s+.*\|\s*(ba)?sh`),
	regexp.MustCompile(`(?i)wget\s+.*\|\s*(ba)?sh`),
	regexp.MustCompile(`(?i)curl\s+.*-o\s*/`),

	// Environment manipulation
	regexp.MustCompile(`(?i)export\s+(PATH|LD_PRELOAD|LD_LIBRARY_PATH)=`),
	regexp.MustCompile(`(?i)unset\s+(PATH|HOME)`),
}

// sensitiveDirectories are paths that commands should not access
var sensitiveDirectories = []string{
	"/etc/nginx", "/etc/apache2", "/etc/ssh",
	"/root", "/var/log", "/boot", "/proc", "/sys",
}

// CommandValidationResult represents the result of command validation
type CommandValidationResult struct {
	Allowed bool
	Reason  string
	Warning string
}

// validateCommand checks if a command is safe to execute
func validateCommand(command string, workspacePath string) CommandValidationResult {
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return CommandValidationResult{
				Allowed: false,
				Reason:  fmt.Sprintf("command matches dangerous pattern: %s", pattern.String()),
			}
		}
	}

	for _, dir := range sensitiveDirectories {
		if strings.Contains(command, dir) {
			return CommandValidationResult{
				Allowed: false,
				Reason:  fmt.Sprintf("command attempts to access sensitive directory: %s", dir),
			}
		}
	}

	var warning string
	if strings.Contains(command, "sudo") {
		warning = "command uses sudo - may require elevated privileges"
	}

	return CommandValidationResult{
		Allowed: true,
		Warning: warning,
	}
}

// validateWorkdir ensures the working directory is within the workspace
func validateWorkdir(workdir, workspacePath string) error {
	if workdir == "" {
		return nil
	}

	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return fmt.Errorf("invalid workdir path: %w", err)
	}

	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("invalid workspace path: %w", err)
	}

	cleanWorkdir := filepath.Clean(absWorkdir)
	cleanWorkspace := filepath.Clean(absWorkspace)

	if !strings.HasPrefix(cleanWorkdir, cleanWorkspace) {
		return fmt.Errorf("workdir %q is outside workspace %q", workdir, workspacePath)
	}

	return nil
}

const (
	defaultBashTimeout  = 2 * time.Minute
	maxBashOutputLength = 30000
)

type BashInput struct {
	Command     string `json:"command" jsonschema:"description=The command to execute"`
	Timeout     int    `json:"timeout,omitempty" jsonschema:"description=Optional timeout in milliseconds (default 120000)"`
	Workdir     string `json:"workdir,omitempty" jsonschema:"description=Working directory for the command. Defaults to workspace root."`
	Description string `json:"description" jsonschema:"description=Clear 5-10 word description of what this command does"`
}

type BashOutput struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out,omitempty"`
}

// NewBashTool creates a bash tool that uses the host directly (legacy API).
// Prefer NewBashToolWithSandbox for sandbox isolation support.
func NewBashTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
	return NewBashToolWithSandbox(genkitApp, workspacePath, nil)
}

// NewBashToolWithSandbox creates a bash tool with optional sandbox support.
// If sb is nil, commands are executed directly on the host.
func NewBashToolWithSandbox(genkitApp *genkit.Genkit, workspacePath string, sb sandbox.Sandbox) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"bash",
		`Execute a bash command in a persistent shell session.

IMPORTANT: 
- Use the workdir parameter instead of 'cd' commands
- Quote file paths containing spaces with double quotes
- Commands timeout after 2 minutes by default
- Output is truncated at 30000 characters

Examples:
- ls -la
- git status
- npm install
- mkdir "path with spaces"`,
		func(ctx *ai.ToolContext, input BashInput) (BashOutput, error) {
			if input.Command == "" {
				return BashOutput{}, fmt.Errorf("command is required")
			}

			// Validate command for dangerous patterns
			validation := validateCommand(input.Command, workspacePath)
			if !validation.Allowed {
				return BashOutput{
					Output:   fmt.Sprintf("Command blocked: %s", validation.Reason),
					ExitCode: 1,
				}, nil
			}

			workdir := workspacePath
			if input.Workdir != "" {
				if filepath.IsAbs(input.Workdir) {
					workdir = input.Workdir
				} else {
					workdir = filepath.Join(workspacePath, input.Workdir)
				}
			}

			// Validate workdir is within workspace
			if err := validateWorkdir(workdir, workspacePath); err != nil {
				return BashOutput{
					Output:   fmt.Sprintf("Workdir validation failed: %s", err.Error()),
					ExitCode: 1,
				}, nil
			}

			timeout := defaultBashTimeout
			if input.Timeout > 0 {
				timeout = time.Duration(input.Timeout) * time.Millisecond
			}

			execCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Use sandbox if available, otherwise execute directly on host
			if sb != nil {
				return executeBashViaSandbox(execCtx, sb, input.Command, workdir, timeout)
			}

			return executeBashDirect(execCtx, input.Command, workdir, timeout)
		},
	)
}

// executeBashViaSandbox executes the command through the sandbox interface
func executeBashViaSandbox(ctx context.Context, sb sandbox.Sandbox, command, workdir string, timeout time.Duration) (BashOutput, error) {
	// Prepend cd to workdir if different from sandbox workspace
	fullCommand := command
	if workdir != "" && workdir != sb.Config().WorkspacePath {
		fullCommand = fmt.Sprintf("cd %q && %s", workdir, command)
	}

	result, err := sb.Exec(ctx, "bash", "-c", fullCommand)
	if err != nil {
		return BashOutput{}, fmt.Errorf("sandbox exec failed: %w", err)
	}

	output := result.Stdout
	if result.Stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += result.Stderr
	}

	timedOut := result.Killed && strings.Contains(result.KillReason, "timeout")

	if len(output) > maxBashOutputLength {
		output = output[:maxBashOutputLength]
		output += fmt.Sprintf("\n\n<bash_metadata>\nbash tool truncated output as it exceeded %d char limit\n</bash_metadata>", maxBashOutputLength)
	}

	if timedOut {
		output += fmt.Sprintf("\n\n<bash_metadata>\nbash tool terminated command after exceeding timeout %v\n</bash_metadata>", timeout)
	}

	return BashOutput{
		Output:   output,
		ExitCode: result.ExitCode,
		TimedOut: timedOut,
	}, nil
}

// executeBashDirect executes the command directly on the host (no sandbox)
func executeBashDirect(ctx context.Context, command, workdir string, timeout time.Duration) (BashOutput, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = workdir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += stderr.String()
	}

	timedOut := ctx.Err() == context.DeadlineExceeded

	if len(output) > maxBashOutputLength {
		output = output[:maxBashOutputLength]
		output += fmt.Sprintf("\n\n<bash_metadata>\nbash tool truncated output as it exceeded %d char limit\n</bash_metadata>", maxBashOutputLength)
	}

	if timedOut {
		output += fmt.Sprintf("\n\n<bash_metadata>\nbash tool terminated command after exceeding timeout %v\n</bash_metadata>", timeout)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if !timedOut {
			return BashOutput{}, fmt.Errorf("command execution failed: %w", err)
		}
	}

	return BashOutput{
		Output:   output,
		ExitCode: exitCode,
		TimedOut: timedOut,
	}, nil
}

func isPathInWorkspace(path, workspacePath string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absWorkspace, err := filepath.Abs(workspacePath)
	if err != nil {
		return false
	}
	return strings.HasPrefix(absPath, absWorkspace)
}
