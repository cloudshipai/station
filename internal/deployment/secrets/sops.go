package secrets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"station/internal/deployment"
)

type SOPSProvider struct{}

func NewSOPSProvider() *SOPSProvider {
	return &SOPSProvider{}
}

func (p *SOPSProvider) Name() string {
	return "sops"
}

func (p *SOPSProvider) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("sops"); err != nil {
		return fmt.Errorf("sops CLI not found: install from https://github.com/getsops/sops/releases")
	}
	return nil
}

func (p *SOPSProvider) GetSecrets(ctx context.Context, path string) (map[string]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("encrypted file not found: %s", path)
	}

	cmd := exec.CommandContext(ctx, "sops", "--decrypt", path)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt '%s': %w (ensure you have the decryption key)", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	secrets := make(map[string]string)

	switch ext {
	case ".yaml", ".yml":
		var data map[string]interface{}
		if err := yaml.Unmarshal(output, &data); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		secrets = flattenMap(data, "")

	case ".json":
		var data map[string]interface{}
		if err := yaml.Unmarshal(output, &data); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		secrets = flattenMap(data, "")

	case ".env":
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				secrets[parts[0]] = parts[1]
			}
		}

	default:
		return nil, fmt.Errorf("unsupported file format: %s (use .yaml, .yml, .json, or .env)", ext)
	}

	return secrets, nil
}

func (p *SOPSProvider) GetSecret(ctx context.Context, path string, key string) (string, error) {
	secrets, err := p.GetSecrets(ctx, path)
	if err != nil {
		return "", err
	}

	value, ok := secrets[key]
	if !ok {
		return "", fmt.Errorf("key '%s' not found in '%s'", key, path)
	}

	return value, nil
}

func (p *SOPSProvider) ListSecrets(ctx context.Context, path string) ([]string, error) {
	secrets, err := p.GetSecrets(ctx, path)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(secrets))
	for key := range secrets {
		keys = append(keys, key)
	}

	return keys, nil
}

func flattenMap(data map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)

	for key, value := range data {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "_" + key
		}
		fullKey = strings.ToUpper(fullKey)

		switch v := value.(type) {
		case map[string]interface{}:
			for k, val := range flattenMap(v, fullKey) {
				result[k] = val
			}
		case []interface{}:
			for i, item := range v {
				itemKey := fmt.Sprintf("%s_%d", fullKey, i)
				switch itemVal := item.(type) {
				case map[string]interface{}:
					for k, val := range flattenMap(itemVal, itemKey) {
						result[k] = val
					}
				default:
					result[itemKey] = fmt.Sprintf("%v", item)
				}
			}
		default:
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}

	return result
}

func init() {
	deployment.RegisterSecretProvider("sops", func() deployment.SecretProvider {
		return NewSOPSProvider()
	})
}
