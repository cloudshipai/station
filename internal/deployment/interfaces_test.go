package deployment

import (
	"testing"
)

func TestParseSecretProviderURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		provider string
		path     string
		region   string
		wantErr  bool
	}{
		{
			name:     "aws-secretsmanager simple",
			uri:      "aws-secretsmanager://my-secret",
			provider: "aws-secretsmanager",
			path:     "my-secret",
		},
		{
			name:     "aws-secretsmanager with region",
			uri:      "aws-secretsmanager://my-secret?region=us-east-1",
			provider: "aws-secretsmanager",
			path:     "my-secret",
			region:   "us-east-1",
		},
		{
			name:     "aws-ssm path",
			uri:      "aws-ssm:///station/prod/",
			provider: "aws-ssm",
			path:     "/station/prod/",
		},
		{
			name:     "aws-ssm with region",
			uri:      "aws-ssm:///station/prod/?region=us-west-2",
			provider: "aws-ssm",
			path:     "/station/prod/",
			region:   "us-west-2",
		},
		{
			name:     "vault path",
			uri:      "vault://secret/data/station/prod",
			provider: "vault",
			path:     "secret/data/station/prod",
		},
		{
			name:     "vault with addr option",
			uri:      "vault://secret/data/station/prod?addr=https://vault.example.com",
			provider: "vault",
			path:     "secret/data/station/prod",
		},
		{
			name:     "gcp-secretmanager",
			uri:      "gcp-secretmanager://projects/my-project/secrets/station-prod",
			provider: "gcp-secretmanager",
			path:     "projects/my-project/secrets/station-prod",
		},
		{
			name:     "sops local file",
			uri:      "sops://./secrets/prod.enc.yaml",
			provider: "sops",
			path:     "./secrets/prod.enc.yaml",
		},
		{
			name:    "invalid uri",
			uri:     "invalid-uri",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ParseSecretProviderURI(tt.uri)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if config.Provider != tt.provider {
				t.Errorf("provider: got %s, want %s", config.Provider, tt.provider)
			}
			if config.Path != tt.path {
				t.Errorf("path: got %s, want %s", config.Path, tt.path)
			}
			if config.Region != tt.region {
				t.Errorf("region: got %s, want %s", config.Region, tt.region)
			}
		})
	}
}

func TestDeploymentTargetRegistry(t *testing.T) {
	targets := ListDeploymentTargets()
	if len(targets) == 0 {
		t.Skip("no targets registered (run with target package imported)")
	}

	for _, name := range targets {
		target, ok := GetDeploymentTarget(name)
		if !ok {
			t.Errorf("target %s listed but not retrievable", name)
		}
		if target.Name() != name {
			t.Errorf("target name mismatch: got %s, registered as %s", target.Name(), name)
		}
	}
}

func TestSecretProviderRegistry(t *testing.T) {
	providers := ListSecretProviders()
	if len(providers) == 0 {
		t.Skip("no providers registered (run with secrets package imported)")
	}

	for _, name := range providers {
		provider, ok := GetSecretProvider(name)
		if !ok {
			t.Errorf("provider %s listed but not retrievable", name)
		}
		if provider.Name() != name {
			t.Errorf("provider name mismatch: got %s, registered as %s", provider.Name(), name)
		}
	}
}
