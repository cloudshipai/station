package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"station/internal/deployment"
	"station/internal/services"
)

// DeployToAnsible deploys an environment using Ansible
func DeployToAnsible(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, outputDir string, dryRun bool, hosts []string, sshKey, sshUser, bundlePath string, runtimeSecrets *RuntimeSecretsConfig) error {
	fmt.Printf("🔧 Deploying with Ansible...\n\n")

	target, ok := deployment.GetDeploymentTarget("ansible")
	if !ok {
		return fmt.Errorf("ansible deployment target not registered")
	}

	if !dryRun {
		if err := target.Validate(ctx); err != nil {
			return fmt.Errorf("ansible validation failed: %w", err)
		}
	}

	// If no bundle path provided but we have an environment, create a bundle from it
	// This ensures the deployed Station has the same agents as the local environment
	var tempBundleCleanup func()
	if bundlePath == "" && envConfig.Path != "" {
		fmt.Printf("📦 Creating bundle from environment '%s'...\n", envName)
		bundleService := services.NewBundleService()
		bundleData, err := bundleService.CreateBundle(envConfig.Path)
		if err != nil {
			return fmt.Errorf("failed to create bundle from environment: %w", err)
		}

		// Always write bundle to output directory so the playbook is self-contained
		// This allows re-running ansible-playbook without the deploy command
		effectiveOutputDir := outputDir
		if effectiveOutputDir == "" {
			effectiveOutputDir = fmt.Sprintf("ansible-%s", envName)
		}
		if err := os.MkdirAll(effectiveOutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		bundleFilePath := filepath.Join(effectiveOutputDir, fmt.Sprintf("bundle-%s.tar.gz", envName))
		if err := os.WriteFile(bundleFilePath, bundleData, 0644); err != nil {
			return fmt.Errorf("failed to write bundle file: %w", err)
		}
		tempBundleCleanup = func() {} // Don't clean up - user needs this file for ansible

		bundlePath = bundleFilePath
		fmt.Printf("   ✓ Bundle created: %s (%d bytes)\n", filepath.Base(bundlePath), len(bundleData))
		fmt.Printf("   ✓ Agents: %d\n\n", len(envConfig.Agents))
	}
	if tempBundleCleanup != nil {
		defer tempBundleCleanup()
	}

	deployConfig := &deployment.DeploymentConfig{
		EnvironmentName:      envName,
		DockerImage:          BaseStationImage,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
	}

	secrets := BuildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)

	options := deployment.DeployOptions{
		DryRun:     dryRun,
		OutputDir:  outputDir,
		Hosts:      hosts,
		SSHKey:     sshKey,
		SSHUser:    sshUser,
		BundlePath: bundlePath,
	}

	return target.Deploy(ctx, deployConfig, secrets, options)
}

// DeployBundleToAnsible deploys a bundle using Ansible
func DeployBundleToAnsible(ctx context.Context, bundleID, appName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, outputDir string, dryRun bool, hosts []string, sshKey, sshUser string, runtimeSecrets *RuntimeSecretsConfig) error {
	fmt.Printf("🔧 Deploying bundle with Ansible...\n\n")

	target, ok := deployment.GetDeploymentTarget("ansible")
	if !ok {
		return fmt.Errorf("ansible deployment target not registered")
	}

	if !dryRun {
		if err := target.Validate(ctx); err != nil {
			return fmt.Errorf("ansible validation failed: %w", err)
		}
	}

	deployConfig := &deployment.DeploymentConfig{
		EnvironmentName:      appName,
		DockerImage:          BaseStationImage,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
	}

	secrets := BuildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)
	secrets["STN_BUNDLE_ID"] = bundleID

	options := deployment.DeployOptions{
		DryRun:    dryRun,
		OutputDir: outputDir,
		Hosts:     hosts,
		SSHKey:    sshKey,
		SSHUser:   sshUser,
	}

	return target.Deploy(ctx, deployConfig, secrets, options)
}
