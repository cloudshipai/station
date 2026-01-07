package targets

import (
	"context"
	"strings"
	"testing"

	"station/internal/deployment"
)

func TestKubernetesTarget_Name(t *testing.T) {
	target := NewKubernetesTarget()
	if target.Name() != "kubernetes" {
		t.Errorf("expected name 'kubernetes', got '%s'", target.Name())
	}
}

func TestKubernetesTarget_GenerateConfig(t *testing.T) {
	target := NewKubernetesTarget()
	ctx := context.Background()

	config := &deployment.DeploymentConfig{
		EnvironmentName: "test-env",
		DockerImage:     "station:latest",
		AIProvider:      "openai",
		AIModel:         "gpt-4o-mini",
		Namespace:       "station-test",
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

	expectedFiles := []string{
		"namespace.yaml",
		"secret.yaml",
		"deployment.yaml",
		"service.yaml",
		"ingress.yaml",
		"pvc.yaml",
		"kustomization.yaml",
	}

	for _, expected := range expectedFiles {
		if _, ok := files[expected]; !ok {
			t.Errorf("expected file '%s' not generated", expected)
		}
	}

	deployment := files["deployment.yaml"]
	if !strings.Contains(deployment, "station:latest") {
		t.Error("deployment.yaml should contain the docker image")
	}
	if !strings.Contains(deployment, "station-test-env") {
		t.Error("deployment.yaml should contain the app name")
	}

	secret := files["secret.yaml"]
	if !strings.Contains(secret, "STN_AI_API_KEY") {
		t.Error("secret.yaml should contain secret keys")
	}
}

func TestKubernetesTarget_Registration(t *testing.T) {
	target, ok := deployment.GetDeploymentTarget("kubernetes")
	if !ok {
		t.Fatal("kubernetes target not registered")
	}
	if target.Name() != "kubernetes" {
		t.Errorf("expected name 'kubernetes', got '%s'", target.Name())
	}
}
