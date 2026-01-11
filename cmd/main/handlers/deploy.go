package handlers

import (
	"context"

	"station/cmd/main/handlers/deploy"
)

// HandleDeploy is the main entry point for deployment operations.
// This is a thin wrapper that delegates to the deploy package.
func HandleDeploy(ctx context.Context, envName, target, region, sleepAfter, instanceType string, destroy, autoStop, withOpenCode, withSandbox bool, namespace, k8sContext, outputDir string, dryRun bool, bundleID, appName string, hosts []string, sshKey, sshUser, bundlePath, envFile, secretsBackend, secretsPath string) error {
	return deploy.HandleDeploy(ctx, envName, target, region, sleepAfter, instanceType, destroy, autoStop, withOpenCode, withSandbox, namespace, k8sContext, outputDir, dryRun, bundleID, appName, hosts, sshKey, sshUser, bundlePath, envFile, secretsBackend, secretsPath)
}

// Re-export types for backwards compatibility
type DeploymentAIConfig = deploy.DeploymentAIConfig
type DeploymentCloudShipConfig = deploy.DeploymentCloudShipConfig
type DeploymentTelemetryConfig = deploy.DeploymentTelemetryConfig
type EnvironmentConfig = deploy.EnvironmentConfig
type RuntimeSecretsConfig = deploy.RuntimeSecretsConfig

// Re-export functions for backwards compatibility
var DetectAIConfigForExport = deploy.DetectAIConfigForExport
var DetectCloudShipConfigForExport = deploy.DetectCloudShipConfigForExport
var DetectTelemetryConfigForExport = deploy.DetectTelemetryConfigForExport
var LoadEnvironmentConfigForExport = deploy.LoadEnvironmentConfigForExport
