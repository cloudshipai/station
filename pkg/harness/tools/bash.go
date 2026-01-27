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

const (
	defaultBashTimeout  = 2 * time.Minute
	maxBashOutputLength = 30000
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)rm\s+(-[rRfF]+\s+)*(/|/\*|~|~/\*|\$HOME)`),
	regexp.MustCompile(`(?i)rm\s+(-[rRfF]+\s+)*\.\./`),
	regexp.MustCompile(`(?i)mkfs`),
	regexp.MustCompile(`(?i)dd\s+.*of=/dev/`),
	regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),
	regexp.MustCompile(`(?i)chmod\s+(-[rR]+\s+)*777\s+/`),
	regexp.MustCompile(`(?i)chown\s+(-[rR]+\s+)*root\s+/`),
	regexp.MustCompile(`(?i):(){ :|:& };:`),
	regexp.MustCompile(`(?i)/etc/passwd|/etc/shadow`),
	regexp.MustCompile(`(?i)curl\s+.*\|\s*(ba)?sh`),
	regexp.MustCompile(`(?i)wget\s+.*\|\s*(ba)?sh`),
}

var sensitiveDirectories = []string{"/etc", "/root", "/var/log", "/boot", "/sys", "/proc"}

type CommandValidationResult struct {
	Valid   bool
	Reason  string
	Warning string
}

func validateCommand(command string, workspacePath string) CommandValidationResult {
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return CommandValidationResult{Valid: false, Reason: fmt.Sprintf("command blocked: matches dangerous pattern")}
		}
	}
	for _, dir := range sensitiveDirectories {
		if strings.Contains(command, dir) && !strings.HasPrefix(workspacePath, dir) {
			return CommandValidationResult{Valid: false, Reason: fmt.Sprintf("command blocked: attempts to access sensitive directory %q", dir)}
		}
	}
	var warning string
	if strings.Contains(command, "sudo") {
		warning = "warning: command uses sudo"
	}
	return CommandValidationResult{Valid: true, Warning: warning}
}

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

			validation := validateCommand(input.Command, workspacePath)
			if !validation.Valid {
				return BashOutput{Output: validation.Reason, ExitCode: 1}, fmt.Errorf("command validation failed: %s", validation.Reason)
			}

			workdir := workspacePath
			if input.Workdir != "" {
				if filepath.IsAbs(input.Workdir) {
					workdir = input.Workdir
				} else {
					workdir = filepath.Join(workspacePath, input.Workdir)
				}
			}

			if err := validateWorkdir(workdir, workspacePath); err != nil {
				return BashOutput{Output: err.Error(), ExitCode: 1}, err
			}

			timeout := defaultBashTimeout
			if input.Timeout > 0 {
				timeout = time.Duration(input.Timeout) * time.Millisecond
			}

			execCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			var result BashOutput
			var err error
			if sb != nil {
				result, err = executeBashViaSandbox(execCtx, sb, input.Command, workdir, timeout)
			} else {
				result, err = executeBashDirect(execCtx, input.Command, workdir, timeout)
			}

			if validation.Warning != "" && err == nil {
				result.Output = validation.Warning + "\n\n" + result.Output
			}

			return result, err
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
