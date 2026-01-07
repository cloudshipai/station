package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

type VaultProvider struct {
	addr  string
	token string
}

func NewVaultProvider() *VaultProvider {
	return &VaultProvider{
		addr:  os.Getenv("VAULT_ADDR"),
		token: os.Getenv("VAULT_TOKEN"),
	}
}

func (p *VaultProvider) WithAddr(addr string) *VaultProvider {
	p.addr = addr
	return p
}

func (p *VaultProvider) WithToken(token string) *VaultProvider {
	p.token = token
	return p
}

func (p *VaultProvider) Name() string {
	return "vault"
}

func (p *VaultProvider) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("vault"); err != nil {
		return fmt.Errorf("vault CLI not found: install from https://developer.hashicorp.com/vault/install")
	}

	if p.addr == "" {
		return fmt.Errorf("VAULT_ADDR not set: export VAULT_ADDR=https://vault.example.com")
	}

	cmd := exec.CommandContext(ctx, "vault", "token", "lookup")
	cmd.Env = append(os.Environ(), "VAULT_ADDR="+p.addr)
	if p.token != "" {
		cmd.Env = append(cmd.Env, "VAULT_TOKEN="+p.token)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("vault authentication failed: run 'vault login' or set VAULT_TOKEN")
	}

	return nil
}

func (p *VaultProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	args := []string{"kv", "get", "-format=json", path}

	cmd := exec.CommandContext(ctx, "vault", args...)
	cmd.Env = append(os.Environ(), "VAULT_ADDR="+p.addr)
	if p.token != "" {
		cmd.Env = append(cmd.Env, "VAULT_TOKEN="+p.token)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get secret '%s': %w", path, err)
	}

	var response struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, fmt.Errorf("failed to parse vault response: %w", err)
	}

	secrets := make(map[string]string)
	for key, value := range response.Data.Data {
		secrets[key] = fmt.Sprintf("%v", value)
	}

	return secrets, nil
}

func (p *VaultProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	args := []string{"kv", "get", "-format=json", "-field=" + key, path}

	cmd := exec.CommandContext(ctx, "vault", args...)
	cmd.Env = append(os.Environ(), "VAULT_ADDR="+p.addr)
	if p.token != "" {
		cmd.Env = append(cmd.Env, "VAULT_TOKEN="+p.token)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get secret '%s' field '%s': %w", path, key, err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (p *VaultProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	args := []string{"kv", "list", "-format=json", path}

	cmd := exec.CommandContext(ctx, "vault", args...)
	cmd.Env = append(os.Environ(), "VAULT_ADDR="+p.addr)
	if p.token != "" {
		cmd.Env = append(cmd.Env, "VAULT_TOKEN="+p.token)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets at '%s': %w", path, err)
	}

	var keys []string
	if err := json.Unmarshal(output, &keys); err != nil {
		return nil, fmt.Errorf("failed to parse vault list response: %w", err)
	}

	return keys, nil
}

func init() {
	deployment.RegisterSecretProvider("vault", func() deployment.SecretProvider {
		return NewVaultProvider()
	})
}
