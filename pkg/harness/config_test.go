package harness

import (
	"testing"
	"time"
)

func TestDefaultHarnessConfig(t *testing.T) {
	cfg := DefaultHarnessConfig()

	if cfg.Workspace.Path != "./workspace" {
		t.Errorf("Workspace.Path = %q, want %q", cfg.Workspace.Path, "./workspace")
	}
	if cfg.Workspace.Mode != "host" {
		t.Errorf("Workspace.Mode = %q, want %q", cfg.Workspace.Mode, "host")
	}

	if !cfg.Compaction.Enabled {
		t.Error("Compaction.Enabled should be true by default")
	}
	if cfg.Compaction.Threshold != 0.85 {
		t.Errorf("Compaction.Threshold = %f, want %f", cfg.Compaction.Threshold, 0.85)
	}
	if cfg.Compaction.ProtectTokens != 40000 {
		t.Errorf("Compaction.ProtectTokens = %d, want %d", cfg.Compaction.ProtectTokens, 40000)
	}

	if !cfg.Git.AutoBranch {
		t.Error("Git.AutoBranch should be true by default")
	}
	if cfg.Git.BranchPrefix != "agent/" {
		t.Errorf("Git.BranchPrefix = %q, want %q", cfg.Git.BranchPrefix, "agent/")
	}
	if cfg.Git.WorkflowBranchStrategy != "shared" {
		t.Errorf("Git.WorkflowBranchStrategy = %q, want %q", cfg.Git.WorkflowBranchStrategy, "shared")
	}

	if !cfg.NATS.Enabled {
		t.Error("NATS.Enabled should be true by default")
	}
	if cfg.NATS.KVBucket != "harness-state" {
		t.Errorf("NATS.KVBucket = %q, want %q", cfg.NATS.KVBucket, "harness-state")
	}

	if cfg.Permissions.ExternalDirectory != "deny" {
		t.Errorf("Permissions.ExternalDirectory = %q, want %q", cfg.Permissions.ExternalDirectory, "deny")
	}

	if action, ok := cfg.Permissions.Bash["rm -rf *"]; !ok || action != "deny" {
		t.Error("rm -rf * should be denied by default")
	}
	if action, ok := cfg.Permissions.Bash["git push *"]; !ok || action != "ask" {
		t.Error("git push * should require ask by default")
	}
}

func TestDefaultAgentHarnessConfig(t *testing.T) {
	cfg := DefaultAgentHarnessConfig()

	if cfg.MaxSteps != 50 {
		t.Errorf("MaxSteps = %d, want %d", cfg.MaxSteps, 50)
	}
	if cfg.DoomLoopThreshold != 3 {
		t.Errorf("DoomLoopThreshold = %d, want %d", cfg.DoomLoopThreshold, 3)
	}
	if cfg.Timeout != 30*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Minute)
	}
}

func TestAgentHarnessConfig_Merge(t *testing.T) {
	global := DefaultHarnessConfig()

	agentCfg := &AgentHarnessConfig{
		MaxSteps: 100,
	}

	merged := agentCfg.Merge(global)

	if merged.MaxSteps != 100 {
		t.Errorf("merged.MaxSteps = %d, want %d", merged.MaxSteps, 100)
	}
	if merged.DoomLoopThreshold != 3 {
		t.Errorf("merged.DoomLoopThreshold = %d, want %d (default)", merged.DoomLoopThreshold, 3)
	}
	if merged.Timeout != 30*time.Minute {
		t.Errorf("merged.Timeout = %v, want %v (default)", merged.Timeout, 30*time.Minute)
	}
}

func TestAgentHarnessConfig_MergeWithZeroValues(t *testing.T) {
	global := DefaultHarnessConfig()

	agentCfg := &AgentHarnessConfig{}

	merged := agentCfg.Merge(global)

	if merged.MaxSteps != 50 {
		t.Errorf("merged.MaxSteps = %d, want %d (should use default)", merged.MaxSteps, 50)
	}
	if merged.DoomLoopThreshold != 3 {
		t.Errorf("merged.DoomLoopThreshold = %d, want %d (should use default)", merged.DoomLoopThreshold, 3)
	}
	if merged.Timeout != 30*time.Minute {
		t.Errorf("merged.Timeout = %v, want %v (should use default)", merged.Timeout, 30*time.Minute)
	}
}
