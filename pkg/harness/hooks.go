// Package harness provides an agentic execution harness for Station.
package harness

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/firebase/genkit/go/ai"
)

// HookResult indicates how to proceed after a hook runs.
type HookResult string

const (
	// HookContinue allows the operation to proceed normally.
	HookContinue HookResult = "continue"
	// HookBlock stops the operation with an error returned to the LLM.
	HookBlock HookResult = "block"
	// HookInterrupt pauses the operation for human approval.
	HookInterrupt HookResult = "interrupt"
)

// PreHookFunc is called before a tool executes.
// It receives the tool request and returns a result indicating how to proceed.
type PreHookFunc func(ctx context.Context, req *ai.ToolRequest) (HookResult, string)

// PostHookFunc is called after a tool executes successfully.
// It receives the tool request and output for logging/monitoring purposes.
type PostHookFunc func(ctx context.Context, req *ai.ToolRequest, output interface{})

// HookRegistry manages pre and post tool execution hooks.
type HookRegistry struct {
	mu        sync.RWMutex
	preHooks  []PreHookFunc
	postHooks []PostHookFunc
}

// NewHookRegistry creates a new hook registry.
func NewHookRegistry() *HookRegistry {
	return &HookRegistry{
		preHooks:  make([]PreHookFunc, 0),
		postHooks: make([]PostHookFunc, 0),
	}
}

// RegisterPreHook adds a pre-execution hook.
func (hr *HookRegistry) RegisterPreHook(hook PreHookFunc) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.preHooks = append(hr.preHooks, hook)
}

// RegisterPostHook adds a post-execution hook.
func (hr *HookRegistry) RegisterPostHook(hook PostHookFunc) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	hr.postHooks = append(hr.postHooks, hook)
}

// RunPreHooks executes all pre-hooks and returns the most restrictive result.
// Priority: Interrupt > Block > Continue
func (hr *HookRegistry) RunPreHooks(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	var finalResult HookResult = HookContinue
	var finalMsg string

	for _, hook := range hr.preHooks {
		result, msg := hook(ctx, req)
		switch result {
		case HookInterrupt:
			// Interrupt has highest priority - return immediately
			return HookInterrupt, msg
		case HookBlock:
			// Block has higher priority than Continue
			finalResult = HookBlock
			finalMsg = msg
		case HookContinue:
			// Continue only if nothing else is blocking
			if finalResult == HookContinue {
				finalResult = HookContinue
			}
		}
	}

	return finalResult, finalMsg
}

// RunPostHooks executes all post-hooks.
func (hr *HookRegistry) RunPostHooks(ctx context.Context, req *ai.ToolRequest, output interface{}) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	for _, hook := range hr.postHooks {
		hook(ctx, req, output)
	}
}

// RegisterDoomLoopDetection registers a doom loop detection hook.
func (hr *HookRegistry) RegisterDoomLoopDetection(detector *DoomLoopDetector) {
	hr.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		if detector.IsInDoomLoop(req.Name, req.Input) {
			return HookInterrupt, fmt.Sprintf(
				"Doom loop detected: tool '%s' has been called %d times consecutively with similar arguments. "+
					"This may indicate the agent is stuck. Consider rephrasing the task or checking the tool output.",
				req.Name,
				detector.threshold,
			)
		}
		return HookContinue, ""
	})
}

// RegisterPermissionCheck registers permission checking hooks based on config.
func (hr *HookRegistry) RegisterPermissionCheck(perms PermissionsConfig) {
	hr.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		// Handle bash tool permission checks
		if req.Name == "bash" || req.Name == "execute_command" {
			command := extractCommand(req.Input)
			if command != "" {
				return checkBashPermission(command, perms.Bash)
			}
		}
		return HookContinue, ""
	})
}

// extractCommand extracts the command string from tool input.
func extractCommand(input interface{}) string {
	switch v := input.(type) {
	case map[string]interface{}:
		if cmd, ok := v["command"].(string); ok {
			return cmd
		}
	case string:
		return v
	}
	return ""
}

// checkBashPermission checks a command against permission rules.
// Rules are matched in order; first match wins.
func checkBashPermission(command string, rules map[string]string) (HookResult, string) {
	// Check specific rules first (longer patterns)
	type ruleScore struct {
		pattern string
		action  string
		score   int
	}

	var matches []ruleScore
	for pattern, action := range rules {
		if matchWildcard(pattern, command) {
			// Score by specificity (longer patterns are more specific)
			matches = append(matches, ruleScore{pattern, action, len(pattern)})
		}
	}

	// Sort by specificity (most specific first)
	// Use the most specific matching rule
	var bestMatch *ruleScore
	for i := range matches {
		if bestMatch == nil || matches[i].score > bestMatch.score {
			bestMatch = &matches[i]
		}
	}

	if bestMatch != nil {
		switch bestMatch.action {
		case "deny":
			return HookBlock, fmt.Sprintf("command '%s' is denied by rule '%s'", command, bestMatch.pattern)
		case "ask":
			return HookInterrupt, fmt.Sprintf("command '%s' requires approval (matched rule '%s')", command, bestMatch.pattern)
		case "allow":
			return HookContinue, ""
		}
	}

	// Default: allow if no rules match
	return HookContinue, ""
}

// matchWildcard performs OpenCode-style wildcard matching.
// "*" matches any sequence of characters.
func matchWildcard(pattern, str string) bool {
	// Handle empty pattern
	if pattern == "" {
		return str == ""
	}

	// Split pattern by wildcards
	parts := strings.Split(pattern, "*")

	// If pattern is just "*", it matches everything
	if len(parts) == 1 {
		return pattern == str
	}

	// Check if string starts with first part (if pattern doesn't start with *)
	pos := 0
	if parts[0] != "" {
		if !strings.HasPrefix(str, parts[0]) {
			return false
		}
		pos = len(parts[0])
	}

	// Check middle parts
	for i := 1; i < len(parts)-1; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(str[pos:], parts[i])
		if idx == -1 {
			return false
		}
		pos += idx + len(parts[i])
	}

	// Check if string ends with last part (if pattern doesn't end with *)
	lastPart := parts[len(parts)-1]
	if lastPart != "" {
		return strings.HasSuffix(str, lastPart) && pos <= len(str)-len(lastPart)
	}

	return true
}

// PathPermissionHook creates a hook that checks file path permissions.
func PathPermissionHook(workspacePath string, externalPolicy string) PreHookFunc {
	return func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		// File operation tools
		fileTools := map[string]bool{
			"read_file":  true,
			"write_file": true,
			"edit_file":  true,
			"list_files": true,
			"glob":       true,
			"grep":       true,
		}

		if !fileTools[req.Name] {
			return HookContinue, ""
		}

		path := extractPath(req.Input)
		if path == "" {
			return HookContinue, ""
		}

		// Check if path is outside workspace
		if !strings.HasPrefix(path, workspacePath) {
			switch externalPolicy {
			case "deny":
				return HookBlock, fmt.Sprintf("access to path '%s' outside workspace is denied", path)
			case "ask":
				return HookInterrupt, fmt.Sprintf("access to path '%s' outside workspace requires approval", path)
			}
		}

		return HookContinue, ""
	}
}

// extractPath extracts the file path from tool input.
func extractPath(input interface{}) string {
	switch v := input.(type) {
	case map[string]interface{}:
		// Try common path parameter names
		for _, key := range []string{"path", "file_path", "filePath", "file", "directory"} {
			if p, ok := v[key].(string); ok {
				return p
			}
		}
	case string:
		return v
	}
	return ""
}
