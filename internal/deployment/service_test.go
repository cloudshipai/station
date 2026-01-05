package deployment

import (
	"strings"
	"testing"
)

func TestGenerateCloudflareTemplate(t *testing.T) {
	config := DeploymentConfig{
		EnvironmentName:        "test-env",
		DockerImage:            "ghcr.io/cloudshipai/station:latest",
		AIProvider:             "openai",
		AIModel:                "gpt-4o-mini",
		CloudflareInstanceType: "basic",
		CloudflareSleepAfter:   "10m",
		CloudflareMaxInstances: 1,
		EnvironmentVariables:   map[string]string{"TEST_VAR": "test_value"},
	}

	t.Run("wrangler.toml generation", func(t *testing.T) {
		output, err := GenerateDeploymentTemplate("cloudflare", config)
		if err != nil {
			t.Fatalf("failed to generate template: %v", err)
		}

		if !strings.Contains(output, `name = "station-test-env"`) {
			t.Error("expected app name in output")
		}
		if !strings.Contains(output, `max_instances = 1`) {
			t.Error("expected max_instances in output")
		}
		if !strings.Contains(output, `STATION_AI_PROVIDER = "openai"`) {
			t.Error("expected AI provider in output")
		}
		if !strings.Contains(output, `STATION_AI_MODEL = "gpt-4o-mini"`) {
			t.Error("expected AI model in output")
		}
		if !strings.Contains(output, `image = "./Dockerfile"`) {
			t.Error("expected Dockerfile image reference in output")
		}
		if !strings.Contains(output, `[[migrations]]`) {
			t.Error("expected migrations section in output")
		}
		if !strings.Contains(output, `new_sqlite_classes = ["StationContainer"]`) {
			t.Error("expected new_sqlite_classes in migrations")
		}
	})

	t.Run("worker.js generation", func(t *testing.T) {
		output, err := GenerateDeploymentTemplate("cloudflare-worker", config)
		if err != nil {
			t.Fatalf("failed to generate template: %v", err)
		}

		if !strings.Contains(output, "export default") {
			t.Error("expected export default in worker")
		}
		if !strings.Contains(output, "StationContainer") {
			t.Error("expected StationContainer class in worker")
		}
		if !strings.Contains(output, `sleepAfter = "10m"`) {
			t.Error("expected sleepAfter in Container class")
		}
	})

	t.Run("sleep duration in worker", func(t *testing.T) {
		cfg := config
		cfg.CloudflareSleepAfter = "1h"
		output, err := GenerateDeploymentTemplate("cloudflare-worker", cfg)
		if err != nil {
			t.Fatalf("failed to generate template: %v", err)
		}
		if !strings.Contains(output, `sleepAfter = "1h"`) {
			t.Error("expected 1h sleep duration in worker")
		}
	})
}

func TestGenerateFlyTemplate(t *testing.T) {
	config := DeploymentConfig{
		EnvironmentName:      "test-env",
		DockerImage:          "station:test",
		APIPort:              "8585",
		MCPPort:              "8586",
		AIProvider:           "openai",
		AIModel:              "gpt-4o-mini",
		FlyRegion:            "ord",
		EnvironmentVariables: map[string]string{},
	}

	output, err := GenerateDeploymentTemplate("fly", config)
	if err != nil {
		t.Fatalf("failed to generate template: %v", err)
	}

	if !strings.Contains(output, `app = "station-test-env"`) {
		t.Error("expected app name in fly.toml")
	}
	if !strings.Contains(output, `primary_region = "ord"`) {
		t.Error("expected region in fly.toml")
	}
}

func TestUnsupportedProvider(t *testing.T) {
	_, err := GenerateDeploymentTemplate("unsupported-provider", DeploymentConfig{})
	if err == nil {
		t.Error("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "unsupported deployment provider") {
		t.Errorf("unexpected error message: %v", err)
	}
}
