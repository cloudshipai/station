package services

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/docker/docker/api/types/registry"
)

func TestDockerBackend_buildRegistryAuth(t *testing.T) {
	tests := []struct {
		name       string
		auth       RegistryAuthConfig
		wantEmpty  bool
		wantUser   string
		wantServer string
	}{
		{
			name:      "empty auth returns empty string",
			auth:      RegistryAuthConfig{},
			wantEmpty: true,
		},
		{
			name: "username/password auth",
			auth: RegistryAuthConfig{
				Username:      "testuser",
				Password:      "testpass",
				ServerAddress: "ghcr.io",
			},
			wantEmpty:  false,
			wantUser:   "testuser",
			wantServer: "ghcr.io",
		},
		{
			name: "identity token auth",
			auth: RegistryAuthConfig{
				IdentityToken: "token123",
				ServerAddress: "123456.dkr.ecr.us-east-1.amazonaws.com",
			},
			wantEmpty:  false,
			wantServer: "123456.dkr.ecr.us-east-1.amazonaws.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultCodeModeConfig()
			cfg.RegistryAuth = tt.auth

			backend := &DockerBackend{config: cfg}
			result := backend.buildRegistryAuth()

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
				return
			}

			if result == "" {
				t.Error("expected non-empty auth string")
				return
			}

			decoded, err := base64.URLEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("failed to decode base64: %v", err)
			}

			var authConfig registry.AuthConfig
			if err := json.Unmarshal(decoded, &authConfig); err != nil {
				t.Fatalf("failed to unmarshal auth config: %v", err)
			}

			if tt.wantUser != "" && authConfig.Username != tt.wantUser {
				t.Errorf("username = %q, want %q", authConfig.Username, tt.wantUser)
			}
			if tt.wantServer != "" && authConfig.ServerAddress != tt.wantServer {
				t.Errorf("server = %q, want %q", authConfig.ServerAddress, tt.wantServer)
			}
		})
	}
}
