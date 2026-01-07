package targets

import (
	"context"
	"strings"
	"testing"

	"station/internal/deployment"
)

func TestAnsibleTarget_Name(t *testing.T) {
	target := NewAnsibleTarget()
	if target.Name() != "ansible" {
		t.Errorf("expected name 'ansible', got '%s'", target.Name())
	}
}

func TestAnsibleTarget_GenerateConfig(t *testing.T) {
	target := NewAnsibleTarget()
	ctx := context.Background()

	config := &deployment.DeploymentConfig{
		EnvironmentName: "test-env",
		DockerImage:     "station:latest",
		AIProvider:      "openai",
		AIModel:         "gpt-4o-mini",
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
		"inventory.ini",
		"playbook.yml",
		"vars/main.yml",
		"templates/docker-compose.yml.j2",
		"templates/station.service.j2",
	}

	for _, expected := range expectedFiles {
		if _, ok := files[expected]; !ok {
			t.Errorf("expected file '%s' not generated", expected)
		}
	}

	playbook := files["playbook.yml"]
	if !strings.Contains(playbook, "Deploy Station station-test-env") {
		t.Error("playbook should contain app name")
	}
	if !strings.Contains(playbook, "docker.io") || !strings.Contains(playbook, "docker-compose") {
		t.Error("playbook should install Docker dependencies")
	}

	vars := files["vars/main.yml"]
	if !strings.Contains(vars, "station:latest") {
		t.Error("vars should contain docker image")
	}
	if !strings.Contains(vars, "STN_AI_API_KEY") {
		t.Error("vars should contain secrets")
	}

	inventory := files["inventory.ini"]
	if !strings.Contains(inventory, "[station_servers]") {
		t.Error("inventory should contain server group")
	}
}

func TestAnsibleTarget_Registration(t *testing.T) {
	target, ok := deployment.GetDeploymentTarget("ansible")
	if !ok {
		t.Fatal("ansible target not registered")
	}
	if target.Name() != "ansible" {
		t.Errorf("expected name 'ansible', got '%s'", target.Name())
	}
}
