package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type CloneConfig struct {
	URL        string
	Branch     string
	Ref        string
	Depth      int
	Submodules bool
	SSHKeyPath string
}

type Cloner struct {
	config CloneConfig
}

func NewCloner(config CloneConfig) *Cloner {
	return &Cloner{config: config}
}

func (c *Cloner) Clone(ctx context.Context, targetPath string) error {
	if c.config.URL == "" {
		return fmt.Errorf("git URL is required")
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	args := []string{"clone"}

	if c.config.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", c.config.Depth))
	}

	if c.config.Branch != "" && c.config.Ref == "" {
		args = append(args, "--branch", c.config.Branch)
	}

	args = append(args, c.config.URL, targetPath)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = c.buildEnv()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s - %w", strings.TrimSpace(string(output)), err)
	}

	if c.config.Ref != "" {
		if err := c.checkoutRef(ctx, targetPath); err != nil {
			return err
		}
	}

	if c.config.Submodules {
		if err := c.initSubmodules(ctx, targetPath); err != nil {
			return err
		}
	}

	return nil
}

func (c *Cloner) checkoutRef(ctx context.Context, targetPath string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", c.config.Ref)
	cmd.Dir = targetPath
	cmd.Env = c.buildEnv()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout %s failed: %s - %w", c.config.Ref, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (c *Cloner) initSubmodules(ctx context.Context, targetPath string) error {
	cmd := exec.CommandContext(ctx, "git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = targetPath
	cmd.Env = c.buildEnv()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git submodule init failed: %s - %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func (c *Cloner) buildEnv() []string {
	env := os.Environ()

	if c.config.SSHKeyPath != "" {
		sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", c.config.SSHKeyPath)
		env = append(env, "GIT_SSH_COMMAND="+sshCmd)
	}

	return env
}

func (c *Cloner) IsConfigured() bool {
	return c.config.URL != ""
}
