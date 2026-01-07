package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

type AWSSSMProvider struct {
	region string
}

func NewAWSSSMProvider() *AWSSSMProvider {
	return &AWSSSMProvider{}
}

func (p *AWSSSMProvider) WithRegion(region string) *AWSSSMProvider {
	p.region = region
	return p
}

func (p *AWSSSMProvider) Name() string {
	return "aws-ssm"
}

func (p *AWSSSMProvider) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("aws"); err != nil {
		return fmt.Errorf("AWS CLI not found: install from https://aws.amazon.com/cli/")
	}

	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("AWS credentials not configured: run 'aws configure' or set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY")
	}

	return nil
}

func (p *AWSSSMProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	args := []string{
		"ssm", "get-parameters-by-path",
		"--path", path,
		"--with-decryption",
		"--query", "Parameters[].{Name:Name,Value:Value}",
		"--output", "json",
	}
	if p.region != "" {
		args = append(args, "--region", p.region)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get parameters from '%s': %w", path, err)
	}

	var params []struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	if err := json.Unmarshal(output, &params); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	secrets := make(map[string]string)
	for _, param := range params {
		key := strings.TrimPrefix(param.Name, path)
		key = strings.ToUpper(strings.ReplaceAll(key, "/", "_"))
		secrets[key] = param.Value
	}

	return secrets, nil
}

func (p *AWSSSMProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	paramPath := path
	if !strings.HasSuffix(paramPath, "/") {
		paramPath = paramPath + "/"
	}
	paramPath = paramPath + key

	args := []string{
		"ssm", "get-parameter",
		"--name", paramPath,
		"--with-decryption",
		"--query", "Parameter.Value",
		"--output", "text",
	}
	if p.region != "" {
		args = append(args, "--region", p.region)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get parameter '%s': %w", paramPath, err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (p *AWSSSMProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	if !strings.HasSuffix(path, "/") {
		path = path + "/"
	}

	args := []string{
		"ssm", "get-parameters-by-path",
		"--path", path,
		"--query", "Parameters[].Name",
		"--output", "json",
	}
	if p.region != "" {
		args = append(args, "--region", p.region)
	}

	cmd := exec.CommandContext(ctx, "aws", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list parameters: %w", err)
	}

	var names []string
	if err := json.Unmarshal(output, &names); err != nil {
		return nil, fmt.Errorf("failed to parse parameter list: %w", err)
	}

	return names, nil
}

func init() {
	deployment.RegisterSecretProvider("aws-ssm", func() deployment.SecretProvider {
		return NewAWSSSMProvider()
	})
}
