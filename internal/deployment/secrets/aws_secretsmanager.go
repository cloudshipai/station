package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

type AWSSecretsManagerProvider struct {
	region string
}

func NewAWSSecretsManagerProvider() *AWSSecretsManagerProvider {
	return &AWSSecretsManagerProvider{}
}

func (p *AWSSecretsManagerProvider) WithRegion(region string) *AWSSecretsManagerProvider {
	p.region = region
	return p
}

func (p *AWSSecretsManagerProvider) Name() string {
	return "aws-secretsmanager"
}

func (p *AWSSecretsManagerProvider) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI not found: install from https://aws.amazon.com/cli/")
	}

	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AWS credentials not configured: run 'aws configure' or set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY")
	}

	return nil
}

func (p *AWSSecretsManagerProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	args := []string{"secretsmanager", "get-secret-value", "--secret-id", path, "--query", "SecretString", "--output", "text"}
	if p.region != "" {
		args = append(args, "--region", p.region)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get secret '%s': %w", path, err)
	}

	var secrets map[string]string
	if err := json.Unmarshal(output, &secrets); err != nil {
		return map[string]string{path: strings.TrimSpace(string(output))}, nil
	}

	return secrets, nil
}

func (p *AWSSecretsManagerProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
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

func (p *AWSSecretsManagerProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	args := []string{"secretsmanager", "list-secrets", "--query", "SecretList[].Name", "--output", "json"}
	if p.region != "" {
		args = append(args, "--region", p.region)
	}
	if path != "" {
		args = append(args, "--filters", fmt.Sprintf("Key=name,Values=%s", path))
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	var names []string
	if err := json.Unmarshal(output, &names); err != nil {
		return nil, fmt.Errorf("failed to parse secret list: %w", err)
	}

	return names, nil
}

func init() {
	deployment.RegisterSecretProvider("aws-secretsmanager", func() deployment.SecretProvider {
		return NewAWSSecretsManagerProvider()
	})
}
