package targets

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

type NomadTarget struct{}

func NewNomadTarget() *NomadTarget {
	return &NomadTarget{}
}

func (n *NomadTarget) Name() string {
	return "nomad"
}

func (n *NomadTarget) Validate(ctx context.Context) error {
	if _, err := exec.LookPath("nomad"); err != nil {
		return fmt.Errorf("nomad CLI not found: install from https://developer.hashicorp.com/nomad/install")
	}
	return nil
}

func (n *NomadTarget) GenerateConfig(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string) (map[string]string, error) {
	files := make(map[string]string)
	appName := fmt.Sprintf("station-%s", config.EnvironmentName)

	files[fmt.Sprintf("%s.nomad.hcl", appName)] = n.generateJobSpec(appName, config, secrets)

	return files, nil
}

func (n *NomadTarget) Deploy(ctx context.Context, config *deployment.DeploymentConfig, secrets map[string]string, options deployment.DeployOptions) error {
	files, err := n.GenerateConfig(ctx, config, secrets)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	outputDir := options.OutputDir
	if outputDir == "" {
		outputDir = fmt.Sprintf("nomad-%s", config.EnvironmentName)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	var jobFile string
	for filename, content := range files {
		path := fmt.Sprintf("%s/%s", outputDir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
		fmt.Printf("   ✓ Generated %s\n", path)
		jobFile = path
	}

	if options.DryRun {
		fmt.Printf("\n📄 Dry run - files generated in %s/\n", outputDir)
		fmt.Printf("   To plan: nomad job plan %s\n", jobFile)
		fmt.Printf("   To run:  nomad job run %s\n", jobFile)
		return nil
	}

	fmt.Printf("\n🔍 Planning Nomad job...\n")
	planCmd := exec.CommandContext(ctx, "nomad", "job", "plan", jobFile)
	planCmd.Stdout = os.Stdout
	planCmd.Stderr = os.Stderr
	planCmd.Run()

	if !options.AutoApprove {
		fmt.Printf("\nProceed with deployment? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			return fmt.Errorf("deployment cancelled by user")
		}
	}

	fmt.Printf("\n🚀 Running Nomad job...\n")
	runCmd := exec.CommandContext(ctx, "nomad", "job", "run", jobFile)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr

	if err := runCmd.Run(); err != nil {
		return fmt.Errorf("nomad job run failed: %w", err)
	}

	fmt.Printf("\n✅ Deployment complete!\n")
	return nil
}

func (n *NomadTarget) Destroy(ctx context.Context, config *deployment.DeploymentConfig) error {
	appName := fmt.Sprintf("station-%s", config.EnvironmentName)

	fmt.Printf("🗑️  Stopping Nomad job '%s'...\n", appName)

	cmd := exec.CommandContext(ctx, "nomad", "job", "stop", "-purge", appName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nomad job stop failed: %w", err)
	}

	fmt.Printf("✅ Job stopped and purged\n")
	return nil
}

func (n *NomadTarget) Status(ctx context.Context, config *deployment.DeploymentConfig) (*deployment.DeploymentStatus, error) {
	appName := fmt.Sprintf("station-%s", config.EnvironmentName)

	cmd := exec.CommandContext(ctx, "nomad", "job", "status", "-short", appName)
	output, err := cmd.Output()
	if err != nil {
		return &deployment.DeploymentStatus{State: "unknown", Message: err.Error()}, nil
	}

	status := &deployment.DeploymentStatus{
		State:    "unknown",
		Message:  string(output),
		Metadata: make(map[string]string),
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "running") {
		status.State = "running"
	} else if strings.Contains(outputStr, "pending") {
		status.State = "pending"
	} else if strings.Contains(outputStr, "dead") {
		status.State = "stopped"
	}

	return status, nil
}

func (n *NomadTarget) generateJobSpec(appName string, config *deployment.DeploymentConfig, secrets map[string]string) string {
	namespace := "default"
	if config.Namespace != "" {
		namespace = config.Namespace
	}

	cpu := 500
	memory := 512
	switch config.ResourceSize {
	case "medium":
		cpu = 1000
		memory = 1024
	case "large":
		cpu = 2000
		memory = 2048
	}

	count := 1
	if config.Replicas > 0 {
		count = config.Replicas
	}

	var envBlock strings.Builder
	for key, value := range secrets {
		envBlock.WriteString(fmt.Sprintf("        %s = %q\n", key, value))
	}

	return fmt.Sprintf(`job "%s" {
  namespace   = "%s"
  datacenters = ["dc1"]
  type        = "service"

  group "station" {
    count = %d

    network {
      port "mcp" {
        static = 8586
        to     = 8586
      }
      port "dynamic-mcp" {
        static = 8587
        to     = 8587
      }
    }

    volume "station_data" {
      type      = "host"
      source    = "station_data"
      read_only = false
    }

    service {
      name = "%s-dynamic-mcp"
      port = "dynamic-mcp"

      check {
        type     = "http"
        path     = "/health"
        interval = "10s"
        timeout  = "3s"
      }

      tags = [
        "traefik.enable=true",
        "traefik.http.routers.%s.rule=Host(`+"`"+`%s.example.com`+"`"+`)",
      ]
    }

    service {
      name = "%s-mcp"
      port = "mcp"

      tags = [
        "traefik.enable=true",
        "traefik.http.routers.%s-mcp.rule=Host(`+"`"+`%s.example.com`+"`"+`) && PathPrefix(`+"`"+`/mcp`+"`"+`)",
      ]
    }

    task "station" {
      driver = "docker"

      config {
        image = "%s"
        ports = ["mcp", "dynamic-mcp"]
      }

      volume_mount {
        volume      = "station_data"
        destination = "/root/.config/station"
      }

      env {
%s      }

      resources {
        cpu    = %d
        memory = %d
      }
    }
  }
}

# Host volume configuration (add to Nomad client config):
# client {
#   host_volume "station_data" {
#     path      = "/opt/nomad/station-data"
#     read_only = false
#   }
# }
`, appName, namespace, count, appName, appName, appName, appName, appName, appName, config.DockerImage, envBlock.String(), cpu, memory)
}

func init() {
	deployment.RegisterDeploymentTarget(NewNomadTarget())
}
