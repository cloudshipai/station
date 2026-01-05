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
		if !strings.Contains(output, `instance_type = "basic"`) {
			t.Error("expected instance_type in output")
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
		if !strings.Contains(output, "ghcr.io/cloudshipai/station:latest") {
			t.Error("expected docker image in output")
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
	})

	t.Run("different instance types", func(t *testing.T) {
		instanceTypes := []string{"lite", "basic", "standard-1", "standard-2", "standard-3", "standard-4"}
		for _, instanceType := range instanceTypes {
			cfg := config
			cfg.CloudflareInstanceType = instanceType
			output, err := GenerateDeploymentTemplate("cloudflare", cfg)
			if err != nil {
				t.Fatalf("failed to generate template for %s: %v", instanceType, err)
			}
			expected := `instance_type = "` + instanceType + `"`
			if !strings.Contains(output, expected) {
				t.Errorf("expected %s in output for instance type %s", expected, instanceType)
			}
		}
	})

	t.Run("always-on mode", func(t *testing.T) {
		cfg := config
		cfg.CloudflareSleepAfter = "168h"
		output, err := GenerateDeploymentTemplate("cloudflare", cfg)
		if err != nil {
			t.Fatalf("failed to generate template: %v", err)
		}
		if !strings.Contains(output, "168h") {
			t.Error("expected 168h sleep duration for always-on mode")
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
