package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	buildpkg "station/cmd/main/handlers/build"
)

// BuildDeploymentImage builds a bundle-based Docker image for deployment
func BuildDeploymentImage(ctx context.Context, envName string, envConfig *EnvironmentConfig, aiConfig *DeploymentAIConfig) (string, error) {
	fmt.Printf("🔨 Building Docker image for environment '%s'...\n", envName)

	envBuilder := buildpkg.NewEnvironmentBuilderWithOptions(
		envName,
		envConfig.Path,
		&buildpkg.BuildOptions{
			Provider: aiConfig.Provider,
			Model:    aiConfig.Model,
		},
	)

	imageName, err := envBuilder.Build(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to build deployment image: %w", err)
	}

	fmt.Printf("   ✓ Built image: %s\n\n", imageName)
	return imageName, nil
}

// HandleBundleDeploy handles deployment from a CloudShip bundle ID (--bundle-id)
func HandleBundleDeploy(ctx context.Context, bundleID, appName, target, region, sleepAfter, instanceType string, autoStop, withOpenCode, withSandbox bool, namespace, k8sContext, outputDir string, dryRun bool, envFile string, runtimeSecrets *RuntimeSecretsConfig) error {
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

	resolvedAppName := ResolveAppName(appName, bundleID, "")
	fmt.Printf("📦 Bundle Deployment:\n")
	fmt.Printf("   ✓ Bundle ID: %s\n", bundleID)
	fmt.Printf("   ✓ App Name: %s\n", resolvedAppName)
	fmt.Printf("   ✓ Image: %s\n\n", BaseStationImage)

	bundleEnvConfig := &EnvironmentConfig{
		Name:      bundleID,
		Path:      "",
		Variables: map[string]string{"STN_BUNDLE_ID": bundleID},
		Agents:    []string{},
	}

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
		return DeployBundleToFly(ctx, bundleID, resolvedAppName, aiConfig, cloudShipConfig, bundleEnvConfig, region, autoStop, withOpenCode, withSandbox)

	case "kubernetes", "k8s":
		return DeployBundleToKubernetes(ctx, bundleID, resolvedAppName, aiConfig, cloudShipConfig, bundleEnvConfig, externalSecrets, namespace, k8sContext, outputDir, dryRun, runtimeSecrets)

	case "ansible":
		return DeployBundleToAnsible(ctx, bundleID, resolvedAppName, aiConfig, cloudShipConfig, bundleEnvConfig, externalSecrets, outputDir, dryRun, nil, "", "", runtimeSecrets)

	case "cloudflare", "cf", "cloudflare-containers":
		return fmt.Errorf("cloudflare target does not support --bundle-id (bundles are already built into the workflow)")

	default:
		return fmt.Errorf("unsupported deployment target: %s (supported: fly, kubernetes, ansible)", target)
	}
}

// HandleBundlePathDeploy handles deployment from a local bundle file (--bundle ./file.tar.gz)
func HandleBundlePathDeploy(ctx context.Context, bundlePath, appName, target, namespace, k8sContext, outputDir string, dryRun bool, hosts []string, sshKey, sshUser, envFile string, runtimeSecrets *RuntimeSecretsConfig) error {
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

	bundleFilename := filepath.Base(bundlePath)
	bundleName := strings.TrimSuffix(bundleFilename, ".tar.gz")
	bundleName = strings.TrimSuffix(bundleName, ".tgz")
	resolvedAppName := ResolveAppName(appName, "", bundleName)

	fmt.Printf("📦 Bundle File Deployment:\n")
	fmt.Printf("   ✓ Bundle Path: %s\n", bundlePath)
	fmt.Printf("   ✓ App Name: %s\n", resolvedAppName)
	fmt.Printf("   ✓ Image: %s\n\n", BaseStationImage)

	bundleEnvConfig := &EnvironmentConfig{
		Name:      bundleName,
		Path:      "",
		Variables: map[string]string{},
		Agents:    []string{},
	}

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
	case "kubernetes", "k8s":
		return DeployToKubernetes(ctx, bundleName, aiConfig, cloudShipConfig, bundleEnvConfig, externalSecrets, namespace, k8sContext, outputDir, dryRun, bundlePath, runtimeSecrets)

	case "ansible":
		return DeployToAnsible(ctx, bundleName, aiConfig, cloudShipConfig, bundleEnvConfig, externalSecrets, outputDir, dryRun, hosts, sshKey, sshUser, bundlePath, runtimeSecrets)

	case "fly", "flyio", "fly.io":
		return fmt.Errorf("fly target does not support --bundle (local file). Use --bundle-id for CloudShip bundles, or deploy an environment which builds the bundle into the image")

	case "cloudflare", "cf", "cloudflare-containers":
		return fmt.Errorf("cloudflare target does not support --bundle (local file)")

	default:
		return fmt.Errorf("unsupported deployment target for bundle path: %s (supported: kubernetes, ansible)", target)
	}
}
