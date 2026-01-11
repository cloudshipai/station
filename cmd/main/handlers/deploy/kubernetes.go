package deploy

import (
	"context"
	"fmt"

	"station/internal/deployment"
)

// DeployToKubernetes deploys an environment to Kubernetes
func DeployToKubernetes(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, namespace, k8sContext, outputDir string, dryRun bool, bundlePath string, runtimeSecrets *RuntimeSecretsConfig) error {
	fmt.Printf("☸️  Deploying to Kubernetes...\n\n")

	target, ok := deployment.GetDeploymentTarget("kubernetes")
	if !ok {
		return fmt.Errorf("kubernetes deployment target not registered")
	}

	if !dryRun {
		if err := target.Validate(ctx); err != nil {
			return fmt.Errorf("kubernetes validation failed: %w", err)
		}
	}

	var imageName string
	if bundlePath != "" {
		imageName = BaseStationImage
		fmt.Printf("📦 Using base image with bundle ConfigMap:\n")
		fmt.Printf("   Image: %s\n", imageName)
		fmt.Printf("   Bundle: %s\n\n", bundlePath)
	} else {
		var err error
		imageName, err = BuildDeploymentImage(ctx, envName, envConfig, aiConfig)
		if err != nil {
			return err
		}
	}

	deployConfig := &deployment.DeploymentConfig{
		EnvironmentName:      envName,
		DockerImage:          imageName,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
		Namespace:            namespace,
	}

	secrets := BuildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)

	options := deployment.DeployOptions{
		DryRun:     dryRun,
		OutputDir:  outputDir,
		Namespace:  namespace,
		Context:    k8sContext,
		BundlePath: bundlePath,
	}

	return target.Deploy(ctx, deployConfig, secrets, options)
}

// DeployBundleToKubernetes deploys a bundle to Kubernetes
func DeployBundleToKubernetes(ctx context.Context, bundleID, appName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, namespace, k8sContext, outputDir string, dryRun bool, runtimeSecrets *RuntimeSecretsConfig) error {
	fmt.Printf("☸️  Deploying bundle to Kubernetes...\n\n")

	target, ok := deployment.GetDeploymentTarget("kubernetes")
	if !ok {
		return fmt.Errorf("kubernetes deployment target not registered")
	}

	if !dryRun {
		if err := target.Validate(ctx); err != nil {
			return fmt.Errorf("kubernetes validation failed: %w", err)
		}
	}

	deployConfig := &deployment.DeploymentConfig{
		EnvironmentName:      appName,
		DockerImage:          BaseStationImage,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
		Namespace:            namespace,
	}

	secrets := BuildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)
	secrets["STN_BUNDLE_ID"] = bundleID

	options := deployment.DeployOptions{
		DryRun:    dryRun,
		OutputDir: outputDir,
		Namespace: namespace,
		Context:   k8sContext,
	}

	return target.Deploy(ctx, deployConfig, secrets, options)
}
