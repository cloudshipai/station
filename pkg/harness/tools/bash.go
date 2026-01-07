package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

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

func NewBashTool(genkitApp *genkit.Genkit, workspacePath string) ai.Tool {
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

			workdir := workspacePath
			if input.Workdir != "" {
				if filepath.IsAbs(input.Workdir) {
					workdir = input.Workdir
				} else {
					workdir = filepath.Join(workspacePath, input.Workdir)
				}
			}

			timeout := defaultBashTimeout
			if input.Timeout > 0 {
				timeout = time.Duration(input.Timeout) * time.Millisecond
			}

			execCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			cmd := exec.CommandContext(execCtx, "bash", "-c", input.Command)
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

			timedOut := execCtx.Err() == context.DeadlineExceeded

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
		},
	)
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
