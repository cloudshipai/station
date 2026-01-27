package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

type GCPSecretManagerProvider struct {
	project string
}

func NewGCPSecretManagerProvider() *GCPSecretManagerProvider {
	return &GCPSecretManagerProvider{}
}

func (p *GCPSecretManagerProvider) WithProject(project string) *GCPSecretManagerProvider {
	p.project = project
	return p
}

func (p *GCPSecretManagerProvider) Name() string {
	return "gcp-secretmanager"
}

func (p *GCPSecretManagerProvider) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found: install from https://cloud.google.com/sdk/install")
	}

	cmd := exec.CommandContext(ctx, "gcloud", "auth", "list", "--filter=status:ACTIVE", "--format=value(account)")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		return fmt.Errorf("gcloud not authenticated: run 'gcloud auth login'")
	}

	return nil
}

func (p *GCPSecretManagerProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	secretID := p.parseSecretID(path)

	args := []string{
		"secrets", "versions", "access", "latest",
		"--secret=" + secretID,
		"--format=value(payload.data)",
	}
	if p.project != "" {
		args = append(args, "--project="+p.project)
	}

	cmd := exec.CommandContext(ctx, "gcloud", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to access secret '%s': %w", secretID, err)
	}

	var secrets map[string]string
	if err := json.Unmarshal(output, &secrets); err != nil {
		return map[string]string{secretID: strings.TrimSpace(string(output))}, nil
	}

	return secrets, nil
}

func (p *GCPSecretManagerProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	secrets, err := p.GetSecrets(ctx, path)
	if err != nil {
		return "", err
	}

	value, ok := secrets[key]
	if !ok {
		return "", fmt.Errorf("key '%s' not found in secret '%s'", key, path)
	}

	return value, nil
}

func (p *GCPSecretManagerProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	args := []string{
		"secrets", "list",
		"--format=value(name)",
	}
	if p.project != "" {
		args = append(args, "--project="+p.project)
	}
	if path != "" {
		args = append(args, "--filter=name:"+path)
	}

	cmd := exec.CommandContext(ctx, "gcloud", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var names []string
	for _, line := range lines {
		if line != "" {
			names = append(names, line)
		}
	}

	return names, nil
}

func (p *GCPSecretManagerProvider) parseSecretID(path string) string {
	if strings.HasPrefix(path, "projects/") {
		parts := strings.Split(path, "/")
		for i, part := range parts {
			if part == "secrets" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return path
}

func init() {
	deployment.RegisterSecretProvider("gcp-secretmanager", func() deployment.SecretProvider {
		return NewGCPSecretManagerProvider()
	})
}
