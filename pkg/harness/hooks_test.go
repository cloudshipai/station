package harness

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
)

func TestHookResult_Constants(t *testing.T) {
	if HookContinue != "continue" {
		t.Errorf("HookContinue = %q, want %q", HookContinue, "continue")
	}
	if HookBlock != "block" {
		t.Errorf("HookBlock = %q, want %q", HookBlock, "block")
	}
	if HookInterrupt != "interrupt" {
		t.Errorf("HookInterrupt = %q, want %q", HookInterrupt, "interrupt")
	}
}

func TestHookRegistry_PreHooks(t *testing.T) {
	registry := NewHookRegistry()

	hookCalled := false
	registry.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		hookCalled = true
		return HookContinue, ""
	})

	req := &ai.ToolRequest{Name: "test_tool"}
	result, _ := registry.RunPreHooks(context.Background(), req)

	if !hookCalled {
		t.Error("PreHook was not called")
	}
	if result != HookContinue {
		t.Errorf("result = %v, want %v", result, HookContinue)
	}
}

func TestHookRegistry_PreHooks_Priority(t *testing.T) {
	registry := NewHookRegistry()

	registry.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		return HookContinue, ""
	})

	registry.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		return HookBlock, "blocked"
	})

	req := &ai.ToolRequest{Name: "test_tool"}
	result, msg := registry.RunPreHooks(context.Background(), req)

	if result != HookBlock {
		t.Errorf("result = %v, want %v", result, HookBlock)
	}
	if msg != "blocked" {
		t.Errorf("msg = %q, want %q", msg, "blocked")
	}
}

func TestHookRegistry_PreHooks_InterruptPriority(t *testing.T) {
	registry := NewHookRegistry()

	registry.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		return HookBlock, "blocked"
	})

	registry.RegisterPreHook(func(ctx context.Context, req *ai.ToolRequest) (HookResult, string) {
		return HookInterrupt, "needs approval"
	})

	req := &ai.ToolRequest{Name: "test_tool"}
	result, msg := registry.RunPreHooks(context.Background(), req)

	if result != HookInterrupt {
		t.Errorf("result = %v, want %v (Interrupt has highest priority)", result, HookInterrupt)
	}
	if msg != "needs approval" {
		t.Errorf("msg = %q, want %q", msg, "needs approval")
	}
}

func TestHookRegistry_PostHooks(t *testing.T) {
	registry := NewHookRegistry()

	hookCalled := false
	var capturedOutput interface{}

	registry.RegisterPostHook(func(ctx context.Context, req *ai.ToolRequest, output interface{}) {
		hookCalled = true
		capturedOutput = output
	})

	req := &ai.ToolRequest{Name: "test_tool"}
	registry.RunPostHooks(context.Background(), req, "test output")

	if !hookCalled {
		t.Error("PostHook was not called")
	}
	if capturedOutput != "test output" {
		t.Errorf("capturedOutput = %v, want %v", capturedOutput, "test output")
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		str     string
		want    bool
	}{
		{"*", "anything", true},
		{"*", "", true},
		{"rm -rf *", "rm -rf /", true},
		{"rm -rf *", "rm -rf /home/user", true},
		{"rm -rf *", "rm /home", false},
		{"git push *", "git push origin main", true},
		{"git push *", "git pull", false},
		{"git push --force*", "git push --force origin main", true},
		{"git push --force*", "git push origin main", false},
		{"*.txt", "file.txt", true},
		{"*.txt", "file.log", false},
		{"test*end", "test123end", true},
		{"test*end", "testend", true},
		{"test*end", "test123", false},
		{"exact", "exact", true},
		{"exact", "exactnot", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.str, func(t *testing.T) {
			got := matchWildcard(tt.pattern, tt.str)
			if got != tt.want {
				t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.str, got, tt.want)
			}
		})
	}
}

func TestCheckBashPermission(t *testing.T) {
	rules := map[string]string{
		"*":                 "allow",
		"rm -rf *":          "deny",
		"git push *":        "ask",
		"git push --force*": "deny",
	}

	tests := []struct {
		command    string
		wantResult HookResult
	}{
		{"ls -la", HookContinue},
		{"cat file.txt", HookContinue},
		{"rm -rf /", HookBlock},
		{"rm -rf /home/user", HookBlock},
		{"git push origin main", HookInterrupt},
		{"git push --force origin main", HookBlock},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			result, _ := checkBashPermission(tt.command, rules)
			if result != tt.wantResult {
				t.Errorf("checkBashPermission(%q) = %v, want %v", tt.command, result, tt.wantResult)
			}
		})
	}
}
