package targets

import (
	"context"
	"strings"
	"testing"

	"station/internal/deployment"
)

func TestNomadTarget_Name(t *testing.T) {
	target := NewNomadTarget()
	if target.Name() != "nomad" {
		t.Errorf("expected name 'nomad', got '%s'", target.Name())
	}
}

func TestNomadTarget_GenerateConfig(t *testing.T) {
	target := NewNomadTarget()
	ctx := context.Background()

	config := &deployment.DeploymentConfig{
		EnvironmentName: "test-env",
		DockerImage:     "station:latest",
		AIProvider:      "openai",
		AIModel:         "gpt-4o-mini",
		Namespace:       "production",
		Replicas:        2,
	}

	secrets := map[string]string{
		"STN_AI_PROVIDER": "openai",
		"STN_AI_MODEL":    "gpt-4o-mini",
		"STN_AI_API_KEY":  "sk-test-key",
	}

	files, err := target.GenerateConfig(ctx, config, secrets)
	if err != nil {
		t.Fatalf("GenerateConfig failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}

	jobFile := files["station-test-env.nomad.hcl"]
	if jobFile == "" {
		t.Fatal("job file not generated")
	}

	if !strings.Contains(jobFile, `job "station-test-env"`) {
		t.Error("job file should contain job definition")
	}
	if !strings.Contains(jobFile, "station:latest") {
		t.Error("job file should contain docker image")
	}
	if !strings.Contains(jobFile, `namespace   = "production"`) {
		t.Error("job file should contain namespace")
	}
	if !strings.Contains(jobFile, "count = 2") {
		t.Error("job file should contain replica count")
	}
	if !strings.Contains(jobFile, "STN_AI_API_KEY") {
		t.Error("job file should contain secrets in env block")
	}
}

func TestNomadTarget_Registration(t *testing.T) {
	target, ok := deployment.GetDeploymentTarget("nomad")
	if !ok {
		t.Fatal("nomad target not registered")
	}
	if target.Name() != "nomad" {
		t.Errorf("expected name 'nomad', got '%s'", target.Name())
	}
}
