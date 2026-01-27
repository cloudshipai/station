package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"station/internal/deployment"
)

// DeployToFly deploys an environment to Fly.io
func DeployToFly(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, imageName, region string, autoStop, withOpenCode, withSandbox bool) error {
	if autoStop {
		fmt.Printf("🚢 Deploying to Fly.io (auto-stop enabled)...\n\n")
	} else {
		fmt.Printf("🚢 Deploying to Fly.io (always-on mode)...\n\n")
	}

	if _, err := exec.LookPath("fly"); err != nil {
		return fmt.Errorf("fly CLI not found. Install from https://fly.io/docs/hands-on/install-flyctl/")
	}

	appName := GetAppName(envName)
	deployConfig := deployment.DeploymentConfig{
		EnvironmentName:      envName,
		DockerImage:          imageName,
		APIPort:              "8585",
		MCPPort:              "8586",
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		FlyRegion:            region,
		FlyAlwaysOn:          !autoStop,
		EnvironmentVariables: envConfig.Variables,
	}

	flyConfig, err := deployment.GenerateDeploymentTemplate("fly", deployConfig)
	if err != nil {
		return fmt.Errorf("failed to generate fly.toml: %w", err)
	}

	flyConfigPath := fmt.Sprintf("fly-%s.toml", envName)
	if err := os.WriteFile(flyConfigPath, []byte(flyConfig), 0644); err != nil {
		return fmt.Errorf("failed to write fly.toml: %w", err)
	}
	defer os.Remove(flyConfigPath)

	fmt.Printf("   ✓ Generated fly.toml\n")

	checkCmd := exec.CommandContext(ctx, "fly", "status", "--app", appName)
	if err := checkCmd.Run(); err != nil {
		fmt.Printf("📦 Creating Fly.io app '%s'...\n", appName)
		createCmd := exec.CommandContext(ctx, "fly", "apps", "create", appName, "--org", "personal")
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create Fly app: %w", err)
		}
		fmt.Printf("   ✓ App created\n\n")
	}

	secrets, err := BuildFlySecrets(aiConfig, cloudShipConfig, envConfig)
	if err != nil {
		return fmt.Errorf("failed to build secrets: %w", err)
	}
	if err := SetFlySecrets(ctx, appName, secrets); err != nil {
		return fmt.Errorf("failed to set Fly secrets: %w", err)
	}

	fmt.Printf("🌐 Checking IP allocation...\n")
	ipCheckCmd := exec.CommandContext(ctx, "fly", "ips", "list", "--app", appName)
	ipOutput, _ := ipCheckCmd.Output()

	if !strings.Contains(string(ipOutput), "v4") {
		fmt.Printf("   Allocating public IPv4 address...\n")
		ipCmd := exec.CommandContext(ctx, "fly", "ips", "allocate-v4", "--app", appName, "--yes")
		ipCmd.Stdout = os.Stdout
		ipCmd.Stderr = os.Stderr
		if err := ipCmd.Run(); err != nil {
			return fmt.Errorf("failed to allocate IPv4: %w", err)
		}
		fmt.Printf("   ✓ IPv4 allocated\n")
	} else {
		fmt.Printf("   ✓ IPv4 already allocated\n")
	}
	fmt.Println()

	volumeName := "station_data"
	volumeCheckCmd := exec.CommandContext(ctx, "fly", "volumes", "list", "--app", appName)
	volumeOutput, _ := volumeCheckCmd.Output()

	if !strings.Contains(string(volumeOutput), volumeName) {
		fmt.Printf("💾 Creating persistent volume...\n")
		volumeCmd := exec.CommandContext(ctx, "fly", "volumes", "create", volumeName,
			"--region", region, "--size", "3", "--app", appName, "-y")
		volumeCmd.Stdout = os.Stdout
		volumeCmd.Stderr = os.Stderr
		if err := volumeCmd.Run(); err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}
		fmt.Printf("   ✓ Volume created\n\n")
	}

	fmt.Printf("🚀 Deploying to Fly.io (this may take a few minutes)...\n\n")
	deployCmd := exec.CommandContext(ctx, "fly", "deploy",
		"--config", flyConfigPath,
		"--local-only",
		"--image", imageName,
		"--app", appName,
		"--ha=false",
	)
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr

	if err := deployCmd.Run(); err != nil {
		return fmt.Errorf("fly deploy failed: %w", err)
	}

	var openCodeURL string
	if withOpenCode {
		openCodeAppName := fmt.Sprintf("%s-opencode", appName)
		openCodeURL = fmt.Sprintf("http://%s.internal:4096", openCodeAppName)
		if err := DeployOpenCode(ctx, openCodeAppName, aiConfig, region); err != nil {
			return fmt.Errorf("failed to deploy OpenCode: %w", err)
		}
		if err := SetFlySecrets(ctx, appName, map[string]string{
			"STN_CODING_OPENCODE_URL": openCodeURL,
			"STN_CODING_BACKEND":      "opencode",
		}); err != nil {
			return fmt.Errorf("failed to set OpenCode URL secret: %w", err)
		}
		fmt.Printf("   ✓ OpenCode configured via private network: %s\n\n", openCodeURL)
	}

	if withSandbox {
		sandboxSecrets := map[string]string{
			"STATION_SANDBOX_ENABLED":           "true",
			"STATION_SANDBOX_CODE_MODE_ENABLED": "true",
			"STN_SANDBOX_BACKEND":               "fly_machines",
		}
		if flyAPIToken := os.Getenv("FLY_API_TOKEN"); flyAPIToken != "" {
			sandboxSecrets["FLY_API_TOKEN"] = flyAPIToken
		}
		if flyOrg := os.Getenv("FLY_ORG"); flyOrg != "" {
			sandboxSecrets["FLY_ORG"] = flyOrg
		}
		if err := SetFlySecrets(ctx, appName, sandboxSecrets); err != nil {
			return fmt.Errorf("failed to set sandbox secrets: %w", err)
		}
		fmt.Printf("   ✓ Fly Machines sandbox backend enabled\n\n")
	}
	fmt.Printf("\n✅ Deployment Complete!\n\n")
	fmt.Printf("🤖 Agent MCP Endpoint:\n")
	fmt.Printf("   https://%s.fly.dev/mcp\n\n", appName)

	fmt.Printf("🌐 Management UI (set STN_DEV_MODE=true to enable):\n")
	fmt.Printf("   https://%s.fly.dev (API runs on port 8585, not publicly exposed)\n\n", appName)

	fmt.Printf("📋 Available Agents (%d):\n", len(envConfig.Agents))
	for _, agent := range envConfig.Agents {
		agentName := strings.TrimSuffix(agent, ".prompt")
		fmt.Printf("   - agent_%s\n", agentName)
	}

	fmt.Printf("\n💡 Add to Claude Desktop:\n")
	fmt.Printf("{\n")
	fmt.Printf("  \"mcpServers\": {\n")
	fmt.Printf("    \"%s\": {\n", appName)
	fmt.Printf("      \"url\": \"https://%s.fly.dev/mcp\"\n", appName)
	fmt.Printf("    }\n")
	fmt.Printf("  }\n")
	fmt.Printf("}\n\n")

	fmt.Printf("💡 Enable Management UI:\n")
	fmt.Printf("   fly secrets set STN_DEV_MODE=true --app %s\n\n", appName)

	return nil
}

// DeployBundleToFly deploys a bundle to Fly.io
func DeployBundleToFly(ctx context.Context, bundleID, appName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, region string, autoStop, withOpenCode, withSandbox bool) error {
	if autoStop {
		fmt.Printf("🚢 Deploying bundle to Fly.io (auto-stop enabled)...\n\n")
	} else {
		fmt.Printf("🚢 Deploying bundle to Fly.io (always-on mode)...\n\n")
	}

	if _, err := exec.LookPath("fly"); err != nil {
		return fmt.Errorf("fly CLI not found. Install from https://fly.io/docs/hands-on/install-flyctl/")
	}

	deployConfig := deployment.DeploymentConfig{
		EnvironmentName:      bundleID[:8],
		DockerImage:          BaseStationImage,
		APIPort:              "8585",
		MCPPort:              "8586",
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		FlyRegion:            region,
		FlyAlwaysOn:          !autoStop,
		EnvironmentVariables: envConfig.Variables,
	}

	flyConfig, err := deployment.GenerateDeploymentTemplate("fly", deployConfig)
	if err != nil {
		return fmt.Errorf("failed to generate fly.toml: %w", err)
	}

	flyConfigPath := fmt.Sprintf("fly-bundle-%s.toml", bundleID[:8])
	if err := os.WriteFile(flyConfigPath, []byte(flyConfig), 0644); err != nil {
		return fmt.Errorf("failed to write fly.toml: %w", err)
	}
	defer os.Remove(flyConfigPath)

	fmt.Printf("   ✓ Generated fly.toml\n")

	checkCmd := exec.CommandContext(ctx, "fly", "status", "--app", appName)
	if err := checkCmd.Run(); err != nil {
		fmt.Printf("📦 Creating Fly.io app '%s'...\n", appName)
		createCmd := exec.CommandContext(ctx, "fly", "apps", "create", appName, "--org", "personal")
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create Fly app: %w", err)
		}
		fmt.Printf("   ✓ App created\n\n")
	}

	secrets, err := BuildBundleFlySecrets(bundleID, aiConfig, cloudShipConfig, envConfig)
	if err != nil {
		return fmt.Errorf("failed to build secrets: %w", err)
	}
	if err := SetFlySecrets(ctx, appName, secrets); err != nil {
		return fmt.Errorf("failed to set Fly secrets: %w", err)
	}

	fmt.Printf("🌐 Checking IP allocation...\n")
	ipCheckCmd := exec.CommandContext(ctx, "fly", "ips", "list", "--app", appName)
	ipOutput, _ := ipCheckCmd.Output()

	if !strings.Contains(string(ipOutput), "v4") {
		fmt.Printf("   Allocating public IPv4 address...\n")
		ipCmd := exec.CommandContext(ctx, "fly", "ips", "allocate-v4", "--app", appName, "--yes")
		ipCmd.Stdout = os.Stdout
		ipCmd.Stderr = os.Stderr
		if err := ipCmd.Run(); err != nil {
			return fmt.Errorf("failed to allocate IPv4: %w", err)
		}
		fmt.Printf("   ✓ IPv4 allocated\n")
	} else {
		fmt.Printf("   ✓ IPv4 already allocated\n")
	}
	fmt.Println()

	volumeName := "station_data"
	volumeCheckCmd := exec.CommandContext(ctx, "fly", "volumes", "list", "--app", appName)
	volumeOutput, _ := volumeCheckCmd.Output()

	if !strings.Contains(string(volumeOutput), volumeName) {
		fmt.Printf("💾 Creating persistent volume...\n")
		volumeCmd := exec.CommandContext(ctx, "fly", "volumes", "create", volumeName,
			"--region", region, "--size", "3", "--app", appName, "-y")
		volumeCmd.Stdout = os.Stdout
		volumeCmd.Stderr = os.Stderr
		if err := volumeCmd.Run(); err != nil {
			return fmt.Errorf("failed to create volume: %w", err)
		}
		fmt.Printf("   ✓ Volume created\n\n")
	}

	fmt.Printf("🚀 Deploying bundle to Fly.io (this may take a few minutes)...\n\n")
	deployCmd := exec.CommandContext(ctx, "fly", "deploy",
		"--config", flyConfigPath,
		"--image", BaseStationImage,
		"--app", appName,
		"--ha=false",
	)
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr

	if err := deployCmd.Run(); err != nil {
		return fmt.Errorf("fly deploy failed: %w", err)
	}

	if withOpenCode {
		openCodeAppName := fmt.Sprintf("%s-opencode", appName)
		openCodeURL := fmt.Sprintf("http://%s.internal:4096", openCodeAppName)
		if err := DeployOpenCode(ctx, openCodeAppName, aiConfig, region); err != nil {
			return fmt.Errorf("failed to deploy OpenCode: %w", err)
		}
		if err := SetFlySecrets(ctx, appName, map[string]string{
			"STN_CODING_OPENCODE_URL": openCodeURL,
			"STN_CODING_BACKEND":      "opencode",
		}); err != nil {
			return fmt.Errorf("failed to set OpenCode URL secret: %w", err)
		}
		fmt.Printf("   ✓ OpenCode configured via private network: %s\n\n", openCodeURL)
	}

	if withSandbox {
		sandboxSecrets := map[string]string{
			"STATION_SANDBOX_ENABLED":           "true",
			"STATION_SANDBOX_CODE_MODE_ENABLED": "true",
			"STN_SANDBOX_BACKEND":               "fly_machines",
		}
		if flyAPIToken := os.Getenv("FLY_API_TOKEN"); flyAPIToken != "" {
			sandboxSecrets["FLY_API_TOKEN"] = flyAPIToken
		}
		if flyOrg := os.Getenv("FLY_ORG"); flyOrg != "" {
			sandboxSecrets["FLY_ORG"] = flyOrg
		}
		if err := SetFlySecrets(ctx, appName, sandboxSecrets); err != nil {
			return fmt.Errorf("failed to set sandbox secrets: %w", err)
		}
		fmt.Printf("   ✓ Fly Machines sandbox backend enabled\n\n")
	}

	fmt.Printf("\n✅ Bundle Deployment Complete!\n\n")
	fmt.Printf("🤖 Agent MCP Endpoint:\n")
	fmt.Printf("   https://%s.fly.dev/mcp\n\n", appName)
	fmt.Printf("📦 Bundle ID: %s\n", bundleID)
	fmt.Printf("   Container will download and install bundle on startup\n\n")

	fmt.Printf("💡 Add to Claude Desktop:\n")
	fmt.Printf("{\n")
	fmt.Printf("  \"mcpServers\": {\n")
	fmt.Printf("    \"%s\": {\n", appName)
	fmt.Printf("      \"url\": \"https://%s.fly.dev/mcp\"\n", appName)
	fmt.Printf("    }\n")
	fmt.Printf("  }\n")
	fmt.Printf("}\n\n")

	return nil
}

// SetFlySecrets sets all secrets in Fly.io at once
func SetFlySecrets(ctx context.Context, appName string, secrets map[string]string) error {
	fmt.Printf("🔐 Setting Fly secrets...\n")

	args := []string{"secrets", "set", "--app", appName}
	for k, v := range secrets {
		displayValue := v
		if IsSensitiveKey(k) {
			displayValue = MaskAPIKey(v)
		}
		fmt.Printf("   ✓ %s=%s\n", k, displayValue)
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}

	cmd := exec.CommandContext(ctx, "fly", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return err
	}

	fmt.Printf("   ✓ %d secrets set\n\n", len(secrets))
	return nil
}

// DeployOpenCode deploys the OpenCode sidecar to Fly.io
func DeployOpenCode(ctx context.Context, appName string, aiConfig *DeploymentAIConfig, region string) error {
	fmt.Printf("\n🔧 Deploying OpenCode backend '%s' (internal-only)...\n\n", appName)

	checkCmd := exec.CommandContext(ctx, "fly", "status", "--app", appName)
	if err := checkCmd.Run(); err != nil {
		fmt.Printf("📦 Creating Fly.io app '%s'...\n", appName)
		createCmd := exec.CommandContext(ctx, "fly", "apps", "create", appName, "--org", "personal")
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create OpenCode app: %w", err)
		}
		fmt.Printf("   ✓ App created\n\n")
	}

	secrets := BuildOpenCodeSecrets(aiConfig)
	if err := SetFlySecrets(ctx, appName, secrets); err != nil {
		return fmt.Errorf("failed to set OpenCode secrets: %w", err)
	}

	volumeName := "opencode_data"
	volumeCheckCmd := exec.CommandContext(ctx, "fly", "volumes", "list", "--app", appName)
	volumeOutput, _ := volumeCheckCmd.Output()

	if !strings.Contains(string(volumeOutput), volumeName) {
		fmt.Printf("💾 Creating persistent volume for OpenCode...\n")
		volumeCmd := exec.CommandContext(ctx, "fly", "volumes", "create", volumeName,
			"--region", region, "--size", "3", "--app", appName, "-y")
		volumeCmd.Stdout = os.Stdout
		volumeCmd.Stderr = os.Stderr
		if err := volumeCmd.Run(); err != nil {
			return fmt.Errorf("failed to create OpenCode volume: %w", err)
		}
		fmt.Printf("   ✓ Volume created\n\n")
	}

	flyToml := GenerateOpenCodeFlyToml(appName, region)
	flyConfigPath := fmt.Sprintf("fly-opencode-%s.toml", appName)
	if err := os.WriteFile(flyConfigPath, []byte(flyToml), 0644); err != nil {
		return fmt.Errorf("failed to write OpenCode fly.toml: %w", err)
	}
	defer os.Remove(flyConfigPath)

	fmt.Printf("   ✓ Generated fly.toml for OpenCode\n")

	fmt.Printf("🚀 Deploying OpenCode (this may take a few minutes)...\n\n")
	deployCmd := exec.CommandContext(ctx, "fly", "deploy",
		"--config", flyConfigPath,
		"--image", "ghcr.io/cloudshipai/opencode-station:latest",
		"--app", appName,
		"--ha=false",
	)
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr

	if err := deployCmd.Run(); err != nil {
		return fmt.Errorf("OpenCode deploy failed: %w", err)
	}

	fmt.Printf("\n✅ OpenCode deployed (internal-only)!\n")
	fmt.Printf("   Private URL: http://%s.internal:4096\n\n", appName)

	return nil
}

// GenerateOpenCodeFlyToml generates the fly.toml for OpenCode sidecar
func GenerateOpenCodeFlyToml(appName, region string) string {
	return fmt.Sprintf(`app = "%s"
primary_region = "%s"

[build]
  image = "ghcr.io/cloudshipai/opencode-station:latest"

[env]
  PORT = "4096"
  OPENCODE_AUTO_APPROVE = "true"

[http_service]
  internal_port = 4096
  force_https = false
  auto_stop_machines = "off"
  auto_start_machines = false
  min_machines_running = 1

[[vm]]
  size = "shared-cpu-1x"
  memory = "512mb"

[mounts]
  source = "opencode_data"
  destination = "/workspaces"
`, appName, region)
}

// DestroyFlyApp destroys a Fly.io app and its OpenCode sidecar
func DestroyFlyApp(ctx context.Context, appName string) error {
	openCodeAppName := fmt.Sprintf("%s-opencode", appName)
	checkCmd := exec.CommandContext(ctx, "fly", "status", "--app", openCodeAppName)
	if checkCmd.Run() == nil {
		fmt.Printf("🗑️  Destroying OpenCode sidecar '%s'...\n", openCodeAppName)
		destroyCmd := exec.CommandContext(ctx, "fly", "apps", "destroy", openCodeAppName, "--yes")
		destroyCmd.Stdout = os.Stdout
		destroyCmd.Stderr = os.Stderr
		if err := destroyCmd.Run(); err != nil {
			fmt.Printf("⚠️  Warning: failed to destroy OpenCode sidecar: %v\n", err)
		} else {
			fmt.Printf("✅ OpenCode sidecar '%s' destroyed\n", openCodeAppName)
		}
	}

	fmt.Printf("🗑️  Destroying Fly.io app '%s'...\n", appName)
	cmd := exec.CommandContext(ctx, "fly", "apps", "destroy", appName, "--yes")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to destroy app: %w", err)
	}
	fmt.Printf("✅ App '%s' destroyed\n", appName)

	return nil
}
