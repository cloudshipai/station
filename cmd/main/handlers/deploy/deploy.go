package deploy

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	_ "station/internal/deployment/secrets"
	_ "station/internal/deployment/targets"
)

// HandleDeploy is the main entry point for deployment operations
func HandleDeploy(ctx context.Context, envName, target, region, sleepAfter, instanceType string, destroy, autoStop, withOpenCode, withSandbox bool, namespace, k8sContext, outputDir string, dryRun bool, bundleID, appName string, hosts []string, sshKey, sshUser, bundlePath, envFile, secretsBackend, secretsPath string) error {
	if target == "" {
		target = "fly"
	}
	if region == "" {
		region = "ord"
	}
	if sleepAfter == "" {
		sleepAfter = "10m"
	}
	if instanceType == "" {
		instanceType = "basic"
	}

	isBundleIDDeploy := bundleID != ""
	isBundlePathDeploy := bundlePath != ""

	if destroy {
		if isBundleIDDeploy || isBundlePathDeploy {
			return fmt.Errorf("--destroy requires an environment name, not --bundle-id or --bundle")
		}
		return handleDeployDestroy(ctx, envName, target)
	}

	var runtimeSecrets *RuntimeSecretsConfig
	if secretsBackend != "" {
		runtimeSecrets = &RuntimeSecretsConfig{
			Backend: secretsBackend,
			Path:    secretsPath,
		}
		fmt.Printf("🔐 Runtime Secrets Configuration:\n")
		fmt.Printf("   ✓ Backend: %s\n", secretsBackend)
		fmt.Printf("   ✓ Path: %s\n", secretsPath)
		fmt.Printf("   ℹ️  Container will fetch secrets at startup\n\n")
	}

	if isBundleIDDeploy {
		fmt.Printf("🚀 Deploying bundle '%s' to %s (region: %s)\n\n", bundleID, target, region)
		return HandleBundleDeploy(ctx, bundleID, appName, target, region, sleepAfter, instanceType, autoStop, withOpenCode, withSandbox, namespace, k8sContext, outputDir, dryRun, envFile, runtimeSecrets)
	}

	if isBundlePathDeploy {
		fmt.Printf("🚀 Deploying from bundle file '%s' to %s\n\n", bundlePath, target)
		return HandleBundlePathDeploy(ctx, bundlePath, appName, target, namespace, k8sContext, outputDir, dryRun, hosts, sshKey, sshUser, envFile, runtimeSecrets)
	}

	fmt.Printf("🚀 Deploying environment '%s' to %s (region: %s)\n\n", envName, target, region)

	aiConfig, err := DetectAIConfig()
	if err != nil {
		return fmt.Errorf("AI configuration error: %w\n\nPlease set the appropriate environment variable for your provider", err)
	}

	fmt.Printf("🔍 AI Configuration:\n")
	fmt.Printf("   ✓ Provider: %s\n", aiConfig.Provider)
	fmt.Printf("   ✓ Model: %s\n", aiConfig.Model)
	fmt.Printf("   ✓ API Key: %s***\n\n", MaskAPIKey(aiConfig.APIKey))

	cloudShipConfig := DetectCloudShipConfig()
	if cloudShipConfig != nil {
		fmt.Printf("☁️  CloudShip Configuration:\n")
		fmt.Printf("   ✓ Enabled: %v\n", cloudShipConfig.Enabled)
		fmt.Printf("   ✓ Name: %s\n", cloudShipConfig.Name)
		fmt.Printf("   ✓ Endpoint: %s\n", cloudShipConfig.Endpoint)
		fmt.Printf("   ✓ Registration Key: %s***\n\n", MaskAPIKey(cloudShipConfig.RegistrationKey))
	}

	telemetryConfig := DetectTelemetryConfig()
	if telemetryConfig != nil && telemetryConfig.Enabled {
		isLocal := telemetryConfig.Endpoint == "" ||
			strings.Contains(telemetryConfig.Endpoint, "localhost") ||
			strings.Contains(telemetryConfig.Endpoint, "127.0.0.1")
		fmt.Printf("📊 Telemetry Configuration:\n")
		if isLocal {
			fmt.Printf("   ✓ Provider: cloudship (auto-configured for deployment)\n")
			fmt.Printf("   ✓ Endpoint: https://telemetry.cloudshipai.com/v1/traces\n")
			fmt.Printf("   ℹ️  Local endpoint '%s' replaced for cloud deployment\n\n", telemetryConfig.Endpoint)
		} else {
			fmt.Printf("   ✓ Provider: %s\n", telemetryConfig.Provider)
			fmt.Printf("   ✓ Endpoint: %s\n\n", telemetryConfig.Endpoint)
		}
	}

	envConfig, err := LoadEnvironmentConfig(envName)
	if err != nil {
		return err
	}

	fmt.Printf("📦 Environment Configuration:\n")
	fmt.Printf("   ✓ Path: %s\n", envConfig.Path)
	fmt.Printf("   ✓ Variables: %d entries\n", len(envConfig.Variables))
	fmt.Printf("   ✓ Agents: %d agents\n\n", len(envConfig.Agents))

	externalSecrets := make(map[string]string)
	if envFile != "" {
		fmt.Printf("🔐 Loading secrets from env file...\n")
		envFileSecrets, err := ParseEnvFile(envFile)
		if err != nil {
			return fmt.Errorf("failed to parse env file: %w", err)
		}
		for k, v := range envFileSecrets {
			externalSecrets[k] = v
		}
		fmt.Printf("   ✓ File: %s\n", envFile)
		fmt.Printf("   ✓ Secrets: %d entries\n\n", len(envFileSecrets))
	}

	switch strings.ToLower(target) {
	case "fly", "flyio", "fly.io":
		imageName, err := BuildDeploymentImage(ctx, envName, envConfig, aiConfig)
		if err != nil {
			return err
		}
		return DeployToFly(ctx, envName, aiConfig, cloudShipConfig, envConfig, imageName, region, autoStop, withOpenCode, withSandbox)

	case "kubernetes", "k8s":
		return DeployToKubernetes(ctx, envName, aiConfig, cloudShipConfig, envConfig, externalSecrets, namespace, k8sContext, outputDir, dryRun, bundlePath, runtimeSecrets)

	case "ansible":
		return DeployToAnsible(ctx, envName, aiConfig, cloudShipConfig, envConfig, externalSecrets, outputDir, dryRun, hosts, sshKey, sshUser, bundlePath, runtimeSecrets)

	case "cloudflare", "cf", "cloudflare-containers":
		return fmt.Errorf("cloudflare target is currently disabled (experimental, not working)")

	default:
		return fmt.Errorf("unsupported deployment target: %s (supported: fly, kubernetes, ansible)", target)
	}
}

// handleDeployDestroy handles destroying a deployed environment
func handleDeployDestroy(ctx context.Context, envName, target string) error {
	appName := GetAppName(envName)

	switch strings.ToLower(target) {
	case "fly", "flyio", "fly.io":
		if _, err := exec.LookPath("fly"); err != nil {
			return fmt.Errorf("fly CLI not found")
		}
		return DestroyFlyApp(ctx, appName)

	case "cloudflare", "cf", "cloudflare-containers":
		return fmt.Errorf("cloudflare target is currently disabled (experimental, not working)")

	default:
		return fmt.Errorf("unsupported target for destroy: %s", target)
	}
}

// DetectAIConfigForExport exports AI config for external use
func DetectAIConfigForExport() (*DeploymentAIConfig, error) {
	return DetectAIConfig()
}

// DetectCloudShipConfigForExport exports CloudShip config for external use
func DetectCloudShipConfigForExport() *DeploymentCloudShipConfig {
	return DetectCloudShipConfig()
}

// DetectTelemetryConfigForExport exports telemetry config for external use
func DetectTelemetryConfigForExport() *DeploymentTelemetryConfig {
	return DetectTelemetryConfig()
}

// LoadEnvironmentConfigForExport exports environment config loading for external use
func LoadEnvironmentConfigForExport(envName string) (*EnvironmentConfig, error) {
	return LoadEnvironmentConfig(envName)
}
