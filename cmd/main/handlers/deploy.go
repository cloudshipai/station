package handlers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	buildpkg "station/cmd/main/handlers/build"
	"station/internal/auth"
	"station/internal/cloudship"
	"station/internal/config"
	"station/internal/deployment"
	"station/internal/services"

	_ "station/internal/deployment/secrets"
	_ "station/internal/deployment/targets"
)

// DeploymentAIConfig holds AI configuration for deployment
type DeploymentAIConfig struct {
	Provider          string
	Model             string
	APIKey            string
	AuthType          string
	OAuthToken        string
	OAuthRefreshToken string
	OAuthExpiresAt    int64
}

// DeploymentCloudShipConfig holds CloudShip configuration for deployment
type DeploymentCloudShipConfig struct {
	Enabled         bool
	RegistrationKey string
	Name            string
	Endpoint        string
	UseTLS          bool
}

// DeploymentTelemetryConfig holds telemetry configuration for deployment
type DeploymentTelemetryConfig struct {
	Enabled  bool
	Provider string
	Endpoint string
}

// EnvironmentConfig holds the loaded environment configuration
type EnvironmentConfig struct {
	Name      string
	Path      string
	Variables map[string]string
	Template  map[string]interface{}
	Agents    []string
}

func HandleDeploy(ctx context.Context, envName, target, region, sleepAfter, instanceType string, destroy, autoStop, withOpenCode, withSandbox bool, secretsFrom, namespace, k8sContext, outputDir string, dryRun bool, bundleID, appName string, hosts []string, sshKey, sshUser, bundlePath, envFile, secretsBackend, secretsPath string) error {
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

	isBundleDeploy := bundleID != ""

	if destroy {
		if isBundleDeploy {
			return fmt.Errorf("--destroy requires an environment name, not --bundle-id")
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

	if isBundleDeploy {
		fmt.Printf("🚀 Deploying bundle '%s' to %s (region: %s)\n\n", bundleID, target, region)
		return handleBundleDeploy(ctx, bundleID, appName, target, region, sleepAfter, instanceType, autoStop, withOpenCode, withSandbox, secretsFrom, namespace, k8sContext, outputDir, dryRun, envFile, runtimeSecrets)
	}

	fmt.Printf("🚀 Deploying environment '%s' to %s (region: %s)\n\n", envName, target, region)

	aiConfig, err := detectAIConfigForDeployment()
	if err != nil {
		return fmt.Errorf("AI configuration error: %w\n\nPlease set the appropriate environment variable for your provider", err)
	}

	fmt.Printf("🔍 AI Configuration:\n")
	fmt.Printf("   ✓ Provider: %s\n", aiConfig.Provider)
	fmt.Printf("   ✓ Model: %s\n", aiConfig.Model)
	fmt.Printf("   ✓ API Key: %s***\n\n", maskAPIKey(aiConfig.APIKey))

	cloudShipConfig := detectCloudShipConfigForDeployment()
	if cloudShipConfig != nil {
		fmt.Printf("☁️  CloudShip Configuration:\n")
		fmt.Printf("   ✓ Enabled: %v\n", cloudShipConfig.Enabled)
		fmt.Printf("   ✓ Name: %s\n", cloudShipConfig.Name)
		fmt.Printf("   ✓ Endpoint: %s\n", cloudShipConfig.Endpoint)
		fmt.Printf("   ✓ Registration Key: %s***\n\n", maskAPIKey(cloudShipConfig.RegistrationKey))
	}

	telemetryConfig := detectTelemetryConfigForDeployment()
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

	envConfig, err := loadEnvironmentConfig(envName)
	if err != nil {
		return err
	}

	fmt.Printf("📦 Environment Configuration:\n")
	fmt.Printf("   ✓ Path: %s\n", envConfig.Path)
	fmt.Printf("   ✓ Variables: %d entries\n", len(envConfig.Variables))
	fmt.Printf("   ✓ Agents: %d agents\n\n", len(envConfig.Agents))

	externalSecrets := make(map[string]string)
	if secretsFrom != "" {
		fmt.Printf("🔐 Fetching secrets from external provider...\n")
		providerConfig, err := deployment.ParseSecretProviderURI(secretsFrom)
		if err != nil {
			return fmt.Errorf("invalid secrets-from URI: %w", err)
		}

		provider, ok := deployment.GetSecretProvider(providerConfig.Provider)
		if !ok {
			return fmt.Errorf("unknown secret provider: %s (supported: aws-secretsmanager, aws-ssm, vault, gcp-secretmanager, sops)", providerConfig.Provider)
		}

		if err := provider.Validate(ctx); err != nil {
			return fmt.Errorf("secret provider validation failed: %w", err)
		}

		externalSecrets, err = provider.GetSecrets(ctx, providerConfig.Path)
		if err != nil {
			return fmt.Errorf("failed to fetch secrets: %w", err)
		}

		fmt.Printf("   ✓ Provider: %s\n", providerConfig.Provider)
		fmt.Printf("   ✓ Path: %s\n", providerConfig.Path)
		fmt.Printf("   ✓ Secrets: %d entries\n\n", len(externalSecrets))
	}

	if envFile != "" {
		fmt.Printf("🔐 Loading secrets from env file...\n")
		envFileSecrets, err := parseEnvFile(envFile)
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
		imageName, err := buildDeploymentImage(ctx, envName, envConfig, aiConfig)
		if err != nil {
			return err
		}
		return deployToFly(ctx, envName, aiConfig, cloudShipConfig, envConfig, imageName, region, autoStop, withOpenCode, withSandbox)

	case "cloudflare", "cf", "cloudflare-containers":
		if withOpenCode || withSandbox {
			return fmt.Errorf("--with-opencode and --with-sandbox are only supported for Fly.io deployments")
		}
		return deployToCloudflare(ctx, envName, aiConfig, envConfig, sleepAfter, instanceType)

	case "kubernetes", "k8s":
		return deployToKubernetes(ctx, envName, aiConfig, cloudShipConfig, envConfig, externalSecrets, namespace, k8sContext, outputDir, dryRun, bundlePath, runtimeSecrets)

	case "ansible":
		return deployToAnsible(ctx, envName, aiConfig, cloudShipConfig, envConfig, externalSecrets, outputDir, dryRun, hosts, sshKey, sshUser, bundlePath, runtimeSecrets)

	default:
		return fmt.Errorf("unsupported deployment target: %s (supported: fly, kubernetes, ansible, cloudflare)", target)
	}
}

const baseStationImage = "ghcr.io/cloudshipai/station:latest"

func handleBundleDeploy(ctx context.Context, bundleID, appName, target, region, sleepAfter, instanceType string, autoStop, withOpenCode, withSandbox bool, secretsFrom, namespace, k8sContext, outputDir string, dryRun bool, envFile string, runtimeSecrets *RuntimeSecretsConfig) error {
	aiConfig, err := detectAIConfigForDeployment()
	if err != nil {
		return fmt.Errorf("AI configuration error: %w\n\nPlease set the appropriate environment variable for your provider", err)
	}

	fmt.Printf("🔍 AI Configuration:\n")
	fmt.Printf("   ✓ Provider: %s\n", aiConfig.Provider)
	fmt.Printf("   ✓ Model: %s\n", aiConfig.Model)
	fmt.Printf("   ✓ API Key: %s***\n\n", maskAPIKey(aiConfig.APIKey))

	cloudShipConfig := detectCloudShipConfigForDeployment()
	if cloudShipConfig != nil {
		fmt.Printf("☁️  CloudShip Configuration:\n")
		fmt.Printf("   ✓ Enabled: %v\n", cloudShipConfig.Enabled)
		fmt.Printf("   ✓ Name: %s\n", cloudShipConfig.Name)
		fmt.Printf("   ✓ Endpoint: %s\n", cloudShipConfig.Endpoint)
		fmt.Printf("   ✓ Registration Key: %s***\n\n", maskAPIKey(cloudShipConfig.RegistrationKey))
	}

	resolvedAppName := resolveAppName(appName, bundleID, "")
	fmt.Printf("📦 Bundle Deployment:\n")
	fmt.Printf("   ✓ Bundle ID: %s\n", bundleID)
	fmt.Printf("   ✓ App Name: %s\n", resolvedAppName)
	fmt.Printf("   ✓ Image: %s\n\n", baseStationImage)

	bundleEnvConfig := &EnvironmentConfig{
		Name:      bundleID,
		Path:      "",
		Variables: map[string]string{"STN_BUNDLE_ID": bundleID},
		Agents:    []string{},
	}

	externalSecrets := make(map[string]string)
	if secretsFrom != "" {
		fmt.Printf("🔐 Fetching secrets from external provider...\n")
		providerConfig, err := deployment.ParseSecretProviderURI(secretsFrom)
		if err != nil {
			return fmt.Errorf("invalid secrets-from URI: %w", err)
		}

		provider, ok := deployment.GetSecretProvider(providerConfig.Provider)
		if !ok {
			return fmt.Errorf("unknown secret provider: %s (supported: aws-secretsmanager, aws-ssm, vault, gcp-secretmanager, sops)", providerConfig.Provider)
		}

		if err := provider.Validate(ctx); err != nil {
			return fmt.Errorf("secret provider validation failed: %w", err)
		}

		externalSecrets, err = provider.GetSecrets(ctx, providerConfig.Path)
		if err != nil {
			return fmt.Errorf("failed to fetch secrets: %w", err)
		}

		fmt.Printf("   ✓ Provider: %s\n", providerConfig.Provider)
		fmt.Printf("   ✓ Path: %s\n", providerConfig.Path)
		fmt.Printf("   ✓ Secrets: %d entries\n\n", len(externalSecrets))
	}

	if envFile != "" {
		fmt.Printf("🔐 Loading secrets from env file...\n")
		envFileSecrets, err := parseEnvFile(envFile)
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
		return deployBundleToFly(ctx, bundleID, resolvedAppName, aiConfig, cloudShipConfig, bundleEnvConfig, region, autoStop, withOpenCode, withSandbox)

	case "kubernetes", "k8s":
		return deployBundleToKubernetes(ctx, bundleID, resolvedAppName, aiConfig, cloudShipConfig, bundleEnvConfig, externalSecrets, namespace, k8sContext, outputDir, dryRun, runtimeSecrets)

	case "ansible":
		return deployBundleToAnsible(ctx, bundleID, resolvedAppName, aiConfig, cloudShipConfig, bundleEnvConfig, externalSecrets, outputDir, dryRun, nil, "", "", runtimeSecrets)

	case "cloudflare", "cf", "cloudflare-containers":
		return fmt.Errorf("cloudflare target does not support --bundle-id (bundles are already built into the workflow)")

	default:
		return fmt.Errorf("unsupported deployment target: %s (supported: fly, kubernetes, ansible)", target)
	}
}

func resolveAppName(customName, bundleID, envName string) string {
	if customName != "" {
		return customName
	}
	cfg, err := config.Load()
	if err == nil && cfg.CloudShip.Name != "" {
		return cfg.CloudShip.Name
	}
	if envName != "" {
		return fmt.Sprintf("station-%s", envName)
	}
	if len(bundleID) >= 8 {
		return fmt.Sprintf("station-%s", bundleID[:8])
	}
	return fmt.Sprintf("station-%s", bundleID)
}

func getAppName(envName string) string {
	return resolveAppName("", "", envName)
}

func handleDeployDestroy(ctx context.Context, envName, target string) error {
	appName := getAppName(envName)

	switch strings.ToLower(target) {
	case "fly", "flyio", "fly.io":
		if _, err := exec.LookPath("fly"); err != nil {
			return fmt.Errorf("fly CLI not found")
		}

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

	case "cloudflare", "cf", "cloudflare-containers":
		if _, err := exec.LookPath("wrangler"); err != nil {
			return fmt.Errorf("wrangler CLI not found")
		}
		fmt.Printf("🗑️  Destroying Cloudflare worker '%s'...\n", appName)
		cmd := exec.CommandContext(ctx, "wrangler", "delete", "--name", appName, "--force")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to destroy worker: %w", err)
		}
		fmt.Printf("✅ Worker '%s' destroyed\n", appName)

	default:
		return fmt.Errorf("unsupported target for destroy: %s", target)
	}

	return nil
}

// detectAIConfigForDeployment uses Station's existing config.Load() to detect AI settings.
// For OAuth, it refreshes the token if expired before deploying.
func detectAIConfigForDeployment() (*DeploymentAIConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load Station config: %w", err)
	}

	aiConfig := &DeploymentAIConfig{
		Provider:          cfg.AIProvider,
		Model:             cfg.AIModel,
		APIKey:            cfg.AIAPIKey,
		AuthType:          cfg.AIAuthType,
		OAuthToken:        cfg.AIOAuthToken,
		OAuthRefreshToken: cfg.AIOAuthRefreshToken,
		OAuthExpiresAt:    cfg.AIOAuthExpiresAt,
	}

	if aiConfig.AuthType == "oauth" {
		if aiConfig.OAuthRefreshToken == "" {
			return nil, fmt.Errorf("OAuth auth type but no refresh token found in config")
		}

		// Refresh the OAuth token before deploying to ensure it's valid
		fmt.Printf("🔄 Refreshing OAuth token for deployment...\n")
		newToken, newRefresh, newExpires, err := config.RefreshOAuthToken(aiConfig.OAuthRefreshToken)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh OAuth token: %w", err)
		}

		aiConfig.OAuthToken = newToken
		if newRefresh != "" {
			aiConfig.OAuthRefreshToken = newRefresh
		}
		aiConfig.OAuthExpiresAt = newExpires
		fmt.Printf("   ✓ OAuth token refreshed (expires in %d hours)\n", (newExpires-currentTimeMs())/1000/3600)

		return aiConfig, nil
	}

	if aiConfig.APIKey == "" {
		return nil, fmt.Errorf(
			"no API key found for provider '%s'\nSet %s environment variable",
			aiConfig.Provider,
			getEnvVarNameForProvider(aiConfig.Provider),
		)
	}

	return aiConfig, nil
}

func currentTimeMs() int64 {
	return time.Now().UnixMilli()
}

func detectCloudShipConfigForDeployment() *DeploymentCloudShipConfig {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	if !cfg.CloudShip.Enabled {
		return nil
	}

	if cfg.CloudShip.RegistrationKey == "" {
		return nil
	}

	return &DeploymentCloudShipConfig{
		Enabled:         cfg.CloudShip.Enabled,
		RegistrationKey: cfg.CloudShip.RegistrationKey,
		Name:            cfg.CloudShip.Name,
		Endpoint:        cfg.CloudShip.Endpoint,
		UseTLS:          cfg.CloudShip.UseTLS,
	}
}

func detectTelemetryConfigForDeployment() *DeploymentTelemetryConfig {
	cfg, err := config.Load()
	if err != nil {
		return nil
	}

	return &DeploymentTelemetryConfig{
		Enabled:  cfg.Telemetry.Enabled,
		Provider: string(cfg.Telemetry.Provider),
		Endpoint: cfg.Telemetry.Endpoint,
	}
}

// loadEnvironmentConfig loads the environment directory and parses its contents
func loadEnvironmentConfig(envName string) (*EnvironmentConfig, error) {
	// Get Station config directory
	stationDir := config.GetStationConfigDir()
	envPath := filepath.Join(stationDir, "environments", envName)

	// Check if environment exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("environment '%s' not found at %s", envName, envPath)
	}

	envConfig := &EnvironmentConfig{
		Name:      envName,
		Path:      envPath,
		Variables: make(map[string]string),
		Agents:    []string{},
	}

	// Load variables.yml
	variablesPath := filepath.Join(envPath, "variables.yml")
	if _, err := os.Stat(variablesPath); err == nil {
		data, err := os.ReadFile(variablesPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables.yml: %w", err)
		}

		var variables map[string]interface{}
		if err := yaml.Unmarshal(data, &variables); err != nil {
			return nil, fmt.Errorf("failed to parse variables.yml: %w", err)
		}

		// Convert to string map
		for k, v := range variables {
			envConfig.Variables[k] = fmt.Sprintf("%v", v)
		}
	}

	// Load template.json (optional)
	templatePath := filepath.Join(envPath, "template.json")
	if _, err := os.Stat(templatePath); err == nil {
		data, err := os.ReadFile(templatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read template.json: %w", err)
		}

		if err := yaml.Unmarshal(data, &envConfig.Template); err != nil {
			return nil, fmt.Errorf("failed to parse template.json: %w", err)
		}
	}

	// List agents
	agentsPath := filepath.Join(envPath, "agents")
	if entries, err := os.ReadDir(agentsPath); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".prompt") {
				envConfig.Agents = append(envConfig.Agents, entry.Name())
			}
		}
	}

	return envConfig, nil
}

// buildDeploymentImage builds a bundle-based Docker image for Fly.io deployment
func buildDeploymentImage(ctx context.Context, envName string, envConfig *EnvironmentConfig, aiConfig *DeploymentAIConfig) (string, error) {
	fmt.Printf("🔨 Building Docker image for environment '%s'...\n", envName)

	// Use NEW bundle-based builder (same as stn build env)
	// This creates a runtime-init image with no baked database or config
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

func deployToFly(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, imageName, region string, autoStop, withOpenCode, withSandbox bool) error {
	if autoStop {
		fmt.Printf("🚢 Deploying to Fly.io (auto-stop enabled)...\n\n")
	} else {
		fmt.Printf("🚢 Deploying to Fly.io (always-on mode)...\n\n")
	}

	// Check if fly CLI is installed
	if _, err := exec.LookPath("fly"); err != nil {
		return fmt.Errorf("fly CLI not found. Install from https://fly.io/docs/hands-on/install-flyctl/")
	}

	appName := getAppName(envName)
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

	// Write fly.toml
	flyConfigPath := fmt.Sprintf("fly-%s.toml", envName)
	if err := os.WriteFile(flyConfigPath, []byte(flyConfig), 0644); err != nil {
		return fmt.Errorf("failed to write fly.toml: %w", err)
	}
	defer os.Remove(flyConfigPath) // Cleanup

	fmt.Printf("   ✓ Generated fly.toml\n")

	// Check if app exists, create if needed (BEFORE setting secrets)
	checkCmd := exec.CommandContext(ctx, "fly", "status", "--app", appName)
	if err := checkCmd.Run(); err != nil {
		// App doesn't exist, create it
		fmt.Printf("📦 Creating Fly.io app '%s'...\n", appName)
		createCmd := exec.CommandContext(ctx, "fly", "apps", "create", appName, "--org", "personal")
		createCmd.Stdout = os.Stdout
		createCmd.Stderr = os.Stderr
		if err := createCmd.Run(); err != nil {
			return fmt.Errorf("failed to create Fly app: %w", err)
		}
		fmt.Printf("   ✓ App created\n\n")
	}

	// Set Fly secrets (AFTER app creation)
	secrets, err := buildFlySecrets(aiConfig, cloudShipConfig, envConfig)
	if err != nil {
		return fmt.Errorf("failed to build secrets: %w", err)
	}
	if err := setFlySecrets(ctx, appName, secrets); err != nil {
		return fmt.Errorf("failed to set Fly secrets: %w", err)
	}

	// Check if IPv4 address is allocated
	fmt.Printf("🌐 Checking IP allocation...\n")
	ipCheckCmd := exec.CommandContext(ctx, "fly", "ips", "list", "--app", appName)
	ipOutput, _ := ipCheckCmd.Output()

	if !strings.Contains(string(ipOutput), "v4") {
		// Allocate IPv4 address for public access
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

	// Check if volume exists
	volumeName := "station_data"
	volumeCheckCmd := exec.CommandContext(ctx, "fly", "volumes", "list", "--app", appName)
	volumeOutput, _ := volumeCheckCmd.Output()

	if !strings.Contains(string(volumeOutput), volumeName) {
		// Create volume
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

	// Deploy
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
		if err := deployOpenCode(ctx, openCodeAppName, aiConfig, region); err != nil {
			return fmt.Errorf("failed to deploy OpenCode: %w", err)
		}
		if err := setFlySecrets(ctx, appName, map[string]string{
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
		if err := setFlySecrets(ctx, appName, sandboxSecrets); err != nil {
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

func deployToCloudflare(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, envConfig *EnvironmentConfig, sleepAfter, instanceType string) error {
	fmt.Printf("⚠️  [EXPERIMENTAL] Cloudflare Containers support is experimental.\n")
	fmt.Printf("   Known issue: Image disk size limits may cause deployment failures.\n\n")
	fmt.Printf("☁️  Deploying to Cloudflare Containers...\n\n")

	if _, err := exec.LookPath("wrangler"); err != nil {
		return fmt.Errorf("wrangler CLI not found. Install with: npm install -g wrangler")
	}

	appName := getAppName(envName)
	outputDir := fmt.Sprintf("cloudflare-%s", envName)
	srcDir := filepath.Join(outputDir, "src")
	bundlePath := filepath.Join(outputDir, fmt.Sprintf("%s.tar.gz", envName))

	if err := os.MkdirAll(srcDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	fmt.Printf("📦 Creating bundle from environment '%s'...\n", envName)
	bundleService := services.NewBundleService()
	bundleData, err := bundleService.CreateBundle(envConfig.Path)
	if err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	if err := os.WriteFile(bundlePath, bundleData, 0644); err != nil {
		return fmt.Errorf("failed to write bundle: %w", err)
	}
	fmt.Printf("   ✓ Bundle created: %s (%d bytes)\n\n", bundlePath, len(bundleData))

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var bundleID string
	if cfg.CloudShip.Enabled && (cfg.CloudShip.RegistrationKey != "" || cfg.CloudShip.APIKey != "") {
		fmt.Printf("☁️  Uploading bundle to CloudShip...\n")
		client := cloudship.NewClient(cfg)
		uploadResp, err := client.UploadBundle(bundlePath)
		if err != nil {
			fmt.Printf("   ⚠ Upload failed: %v\n", err)
			fmt.Printf("   You can manually upload later: stn bundle share %s\n\n", bundlePath)
		} else {
			bundleID = uploadResp.BundleID
			fmt.Printf("   ✓ Uploaded! Bundle ID: %s\n\n", bundleID)
		}
	} else {
		fmt.Printf("⚠️  CloudShip not configured. Manual upload required:\n")
		fmt.Printf("   stn bundle share %s\n", bundlePath)
		fmt.Printf("   Then: wrangler secret put STN_BUNDLE_ID --name %s\n\n", appName)
	}

	deployConfig := deployment.DeploymentConfig{
		EnvironmentName:        envName,
		DockerImage:            "ghcr.io/cloudshipai/station:latest",
		AIProvider:             aiConfig.Provider,
		AIModel:                aiConfig.Model,
		EnvironmentVariables:   envConfig.Variables,
		CloudflareInstanceType: instanceType,
		CloudflareSleepAfter:   sleepAfter,
		CloudflareMaxInstances: 1,
	}

	wranglerConfig, err := deployment.GenerateDeploymentTemplate("cloudflare", deployConfig)
	if err != nil {
		return fmt.Errorf("failed to generate wrangler.toml: %w", err)
	}

	workerConfig, err := deployment.GenerateDeploymentTemplate("cloudflare-worker", deployConfig)
	if err != nil {
		return fmt.Errorf("failed to generate worker.js: %w", err)
	}

	wranglerPath := filepath.Join(outputDir, "wrangler.toml")
	if err := os.WriteFile(wranglerPath, []byte(wranglerConfig), 0644); err != nil {
		return fmt.Errorf("failed to write wrangler.toml: %w", err)
	}
	fmt.Printf("   ✓ Generated %s\n", wranglerPath)

	workerPath := filepath.Join(srcDir, "worker.js")
	if err := os.WriteFile(workerPath, []byte(workerConfig), 0644); err != nil {
		return fmt.Errorf("failed to write worker.js: %w", err)
	}
	fmt.Printf("   ✓ Generated %s\n", workerPath)

	// Generate Dockerfile (wrangler builds and pushes to Cloudflare's registry)
	dockerfileContent := fmt.Sprintf("FROM %s\n", deployConfig.DockerImage)
	dockerfilePath := filepath.Join(outputDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	fmt.Printf("   ✓ Generated %s\n", dockerfilePath)

	packageJSON := fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "type": "module",
  "main": "src/worker.js",
  "dependencies": {
    "@cloudflare/containers": "^0.0.31"
  }
}
`, appName)
	packagePath := filepath.Join(outputDir, "package.json")
	if err := os.WriteFile(packagePath, []byte(packageJSON), 0644); err != nil {
		return fmt.Errorf("failed to write package.json: %w", err)
	}
	fmt.Printf("   ✓ Generated %s\n\n", packagePath)

	secrets := map[string]string{
		"STATION_AI_API_KEY": aiConfig.APIKey,
	}
	encryptionKey, err := generateEncryptionKey()
	if err != nil {
		return fmt.Errorf("failed to generate encryption key: %w", err)
	}
	secrets["STATION_ENCRYPTION_KEY"] = encryptionKey

	if bundleID != "" {
		secrets["STN_BUNDLE_ID"] = bundleID
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	fmt.Printf("🔐 Setting Cloudflare secrets...\n")
	for k, v := range secrets {
		displayValue := v
		if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "secret") {
			displayValue = maskAPIKey(v)
		}
		fmt.Printf("   ✓ %s=%s\n", k, displayValue)

		secretCmd := exec.CommandContext(ctx, "wrangler", "secret", "put", k, "--name", appName)
		secretCmd.Dir = outputDir
		secretCmd.Stdin = strings.NewReader(v)
		secretCmd.Stdout = os.Stdout
		secretCmd.Stderr = os.Stderr
		if err := secretCmd.Run(); err != nil {
			fmt.Printf("   ⚠ Warning: failed to set secret %s (app may not exist yet): %v\n", k, err)
		}
	}
	fmt.Println()

	fmt.Printf("📦 Installing dependencies...\n")
	npmCmd := exec.CommandContext(ctx, "npm", "install")
	npmCmd.Dir = outputDir
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	if err := npmCmd.Run(); err != nil {
		return fmt.Errorf("npm install failed: %w", err)
	}
	fmt.Printf("   ✓ Dependencies installed\n\n")

	fmt.Printf("🚀 Deploying to Cloudflare (this may take a few minutes)...\n\n")
	deployCmd := exec.CommandContext(ctx, "wrangler", "deploy")
	deployCmd.Dir = outputDir
	deployCmd.Stdout = os.Stdout
	deployCmd.Stderr = os.Stderr

	if err := deployCmd.Run(); err != nil {
		return fmt.Errorf("wrangler deploy failed: %w", err)
	}

	fmt.Printf("\n✅ Deployment Complete!\n\n")
	fmt.Printf("🤖 Agent MCP Endpoint:\n")
	fmt.Printf("   https://%s.<your-subdomain>.workers.dev/mcp\n\n", appName)

	fmt.Printf("📋 Available Agents (%d):\n", len(envConfig.Agents))
	for _, agent := range envConfig.Agents {
		agentName := strings.TrimSuffix(agent, ".prompt")
		fmt.Printf("   - agent_%s\n", agentName)
	}

	fmt.Printf("\n💡 Add to Claude Desktop:\n")
	fmt.Printf("{\n")
	fmt.Printf("  \"mcpServers\": {\n")
	fmt.Printf("    \"%s\": {\n", appName)
	fmt.Printf("      \"url\": \"https://%s.<your-subdomain>.workers.dev/mcp\"\n", appName)
	fmt.Printf("    }\n")
	fmt.Printf("  }\n")
	fmt.Printf("}\n\n")

	fmt.Printf("📁 Configuration files saved to: %s/\n", outputDir)
	fmt.Printf("   Bundle: %s\n", bundlePath)
	fmt.Printf("   To redeploy: cd %s && wrangler deploy\n\n", outputDir)

	if bundleID == "" {
		fmt.Printf("⚠️  IMPORTANT: Set STN_BUNDLE_ID secret before the container can start!\n")
		fmt.Printf("   1. Upload bundle: stn bundle share %s\n", bundlePath)
		fmt.Printf("   2. Set secret: wrangler secret put STN_BUNDLE_ID --name %s\n\n", appName)
	}

	fmt.Printf("⚠️  Cloudflare Containers Info:\n")
	fmt.Printf("   - Base image: ghcr.io/cloudshipai/station:latest\n")
	fmt.Printf("   - Bundle downloaded on container wake (cached after first)\n")
	fmt.Printf("   - Container sleeps after %s of inactivity\n", sleepAfter)
	if sleepAfter == "168h" {
		fmt.Printf("   - Always-on mode (default, use --auto-stop to enable suspend)\n")
	}
	fmt.Printf("   - To change: wrangler deploy with updated wrangler.toml\n\n")

	return nil
}

func buildFlySecrets(aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig) (map[string]string, error) {
	secrets := make(map[string]string)

	encryptionKey, err := generateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	secrets["STATION_ENCRYPTION_KEY"] = encryptionKey

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		case "groq":
			secrets["GROQ_API_KEY"] = aiConfig.APIKey
		case "openrouter":
			secrets["OPENROUTER_API_KEY"] = aiConfig.APIKey
		case "together":
			secrets["TOGETHER_API_KEY"] = aiConfig.APIKey
		case "fireworks":
			secrets["FIREWORKS_API_KEY"] = aiConfig.APIKey
		case "ollama":
			secrets["OLLAMA_BASE_URL"] = "http://localhost:11434"
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		secrets["STN_CLOUDSHIP_ENABLED"] = "true"
		secrets["STN_CLOUDSHIP_KEY"] = cloudShipConfig.RegistrationKey
		if cloudShipConfig.Name != "" {
			secrets["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			secrets["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
		if cloudShipConfig.UseTLS {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "true"
		} else {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "false"
		}
	}

	telemetryConfig := detectTelemetryConfigForDeployment()
	if telemetryConfig != nil {
		isLocalEndpoint := telemetryConfig.Endpoint == "" ||
			strings.Contains(telemetryConfig.Endpoint, "localhost") ||
			strings.Contains(telemetryConfig.Endpoint, "127.0.0.1")

		if isLocalEndpoint {
			secrets["STN_TELEMETRY_PROVIDER"] = "cloudship"
			secrets["STN_TELEMETRY_ENDPOINT"] = "https://telemetry.cloudshipai.com/v1/traces"
		} else {
			secrets["STN_TELEMETRY_PROVIDER"] = telemetryConfig.Provider
			secrets["STN_TELEMETRY_ENDPOINT"] = telemetryConfig.Endpoint
		}
		if telemetryConfig.Enabled {
			secrets["STN_TELEMETRY_ENABLED"] = "true"
		}
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	secrets["STATION_MCP_POOLING"] = "true"

	return secrets, nil
}

// setFlySecrets sets all secrets in Fly.io at once
func setFlySecrets(ctx context.Context, appName string, secrets map[string]string) error {
	fmt.Printf("🔐 Setting Fly secrets...\n")

	// Build secret arguments
	args := []string{"secrets", "set", "--app", appName}
	for k, v := range secrets {
		// Mask sensitive values in output
		displayValue := v
		if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "secret") {
			displayValue = maskAPIKey(v)
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

// Helper functions

func getEnvVarNameForProvider(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "OPENAI_API_KEY or STN_AI_API_KEY"
	case "gemini", "google":
		return "GOOGLE_API_KEY, GEMINI_API_KEY, or STN_AI_API_KEY"
	case "anthropic", "claude":
		return "ANTHROPIC_API_KEY or STN_AI_API_KEY"
	default:
		return "STN_AI_API_KEY"
	}
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "***"
}

func parseEnvFile(envFile string) (map[string]string, error) {
	if envFile == "" {
		return nil, nil
	}

	data, err := os.ReadFile(envFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	secrets := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if strings.Contains(value, "***MASKED***") || strings.Contains(value, "***GENERATE_NEW***") {
			continue
		}

		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}

		secrets[key] = value
	}

	return secrets, nil
}

// generateEncryptionKey generates a secure encryption key for Station
func generateEncryptionKey() (string, error) {
	// Use Station's existing GenerateAPIKey function from auth package
	// This generates a secure random key
	key, err := auth.GenerateAPIKey()
	if err != nil {
		return "", err
	}
	// Remove the "sk-" prefix as this is an encryption key, not an API key
	return key[3:], nil
}

func deployOpenCode(ctx context.Context, appName string, aiConfig *DeploymentAIConfig, region string) error {
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

	secrets := buildOpenCodeSecrets(aiConfig)
	if err := setFlySecrets(ctx, appName, secrets); err != nil {
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

	flyToml := generateOpenCodeFlyToml(appName, region)
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

func buildOpenCodeSecrets(aiConfig *DeploymentAIConfig) map[string]string {
	secrets := make(map[string]string)

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model
	secrets["OPENCODE_AUTO_APPROVE"] = "true"

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		}
	}

	return secrets
}

func generateOpenCodeFlyToml(appName, region string) string {
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

func deployToKubernetes(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, namespace, k8sContext, outputDir string, dryRun bool, bundlePath string, runtimeSecrets *RuntimeSecretsConfig) error {
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
		imageName = baseStationImage
		fmt.Printf("📦 Using base image with bundle ConfigMap:\n")
		fmt.Printf("   Image: %s\n", imageName)
		fmt.Printf("   Bundle: %s\n\n", bundlePath)
	} else {
		var err error
		imageName, err = buildDeploymentImage(ctx, envName, envConfig, aiConfig)
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

	secrets := buildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)

	options := deployment.DeployOptions{
		DryRun:     dryRun,
		OutputDir:  outputDir,
		Namespace:  namespace,
		Context:    k8sContext,
		BundlePath: bundlePath,
	}

	return target.Deploy(ctx, deployConfig, secrets, options)
}

func deployToAnsible(ctx context.Context, envName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, outputDir string, dryRun bool, hosts []string, sshKey, sshUser, bundlePath string, runtimeSecrets *RuntimeSecretsConfig) error {
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

	deployConfig := &deployment.DeploymentConfig{
		EnvironmentName:      envName,
		DockerImage:          baseStationImage,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
	}

	secrets := buildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)

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

type RuntimeSecretsConfig struct {
	Backend string
	Path    string
}

func buildAllSecrets(aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, runtimeSecrets *RuntimeSecretsConfig) map[string]string {
	secrets := make(map[string]string)

	if runtimeSecrets != nil && runtimeSecrets.Backend != "" {
		secrets["STN_SECRETS_BACKEND"] = runtimeSecrets.Backend
		if runtimeSecrets.Path != "" {
			secrets["STN_SECRETS_PATH"] = runtimeSecrets.Path
		}
	}

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		secrets["STN_CLOUDSHIP_ENABLED"] = "true"
		secrets["STN_CLOUDSHIP_KEY"] = cloudShipConfig.RegistrationKey
		if cloudShipConfig.Name != "" {
			secrets["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			secrets["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
		if cloudShipConfig.UseTLS {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "true"
		}
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	for k, v := range externalSecrets {
		secrets[k] = v
	}

	return secrets
}

func deployBundleToFly(ctx context.Context, bundleID, appName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, region string, autoStop, withOpenCode, withSandbox bool) error {
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
		DockerImage:          baseStationImage,
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

	secrets, err := buildBundleFlySecrets(bundleID, aiConfig, cloudShipConfig, envConfig)
	if err != nil {
		return fmt.Errorf("failed to build secrets: %w", err)
	}
	if err := setFlySecrets(ctx, appName, secrets); err != nil {
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
		"--image", baseStationImage,
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
		if err := deployOpenCode(ctx, openCodeAppName, aiConfig, region); err != nil {
			return fmt.Errorf("failed to deploy OpenCode: %w", err)
		}
		if err := setFlySecrets(ctx, appName, map[string]string{
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
		if err := setFlySecrets(ctx, appName, sandboxSecrets); err != nil {
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

func buildBundleFlySecrets(bundleID string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig) (map[string]string, error) {
	secrets := make(map[string]string)

	secrets["STN_BUNDLE_ID"] = bundleID

	encryptionKey, err := generateEncryptionKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	secrets["STATION_ENCRYPTION_KEY"] = encryptionKey

	secrets["STN_AI_PROVIDER"] = aiConfig.Provider
	secrets["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		secrets["STN_AI_AUTH_TYPE"] = "oauth"
		secrets["STN_AI_OAUTH_TOKEN"] = aiConfig.OAuthToken
		secrets["STN_AI_OAUTH_REFRESH_TOKEN"] = aiConfig.OAuthRefreshToken
		secrets["STN_AI_OAUTH_EXPIRES_AT"] = fmt.Sprintf("%d", aiConfig.OAuthExpiresAt)
	} else {
		secrets["STN_AI_API_KEY"] = aiConfig.APIKey
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			secrets["OPENAI_API_KEY"] = aiConfig.APIKey
		case "anthropic":
			secrets["ANTHROPIC_API_KEY"] = aiConfig.APIKey
		case "google", "gemini":
			secrets["GOOGLE_API_KEY"] = aiConfig.APIKey
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		secrets["STN_CLOUDSHIP_ENABLED"] = "true"
		secrets["STN_CLOUDSHIP_KEY"] = cloudShipConfig.RegistrationKey
		if cloudShipConfig.Name != "" {
			secrets["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			secrets["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
		if cloudShipConfig.UseTLS {
			secrets["STN_CLOUDSHIP_USE_TLS"] = "true"
		}
	}

	for k, v := range envConfig.Variables {
		secrets[k] = v
	}

	secrets["STATION_MCP_POOLING"] = "true"

	return secrets, nil
}

func deployBundleToKubernetes(ctx context.Context, bundleID, appName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, namespace, k8sContext, outputDir string, dryRun bool, runtimeSecrets *RuntimeSecretsConfig) error {
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
		DockerImage:          baseStationImage,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
		Namespace:            namespace,
	}

	secrets := buildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)
	secrets["STN_BUNDLE_ID"] = bundleID

	options := deployment.DeployOptions{
		DryRun:    dryRun,
		OutputDir: outputDir,
		Namespace: namespace,
		Context:   k8sContext,
	}

	return target.Deploy(ctx, deployConfig, secrets, options)
}

func deployBundleToAnsible(ctx context.Context, bundleID, appName string, aiConfig *DeploymentAIConfig, cloudShipConfig *DeploymentCloudShipConfig, envConfig *EnvironmentConfig, externalSecrets map[string]string, outputDir string, dryRun bool, hosts []string, sshKey, sshUser string, runtimeSecrets *RuntimeSecretsConfig) error {
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
		DockerImage:          baseStationImage,
		AIProvider:           aiConfig.Provider,
		AIModel:              aiConfig.Model,
		EnvironmentVariables: envConfig.Variables,
	}

	secrets := buildAllSecrets(aiConfig, cloudShipConfig, envConfig, externalSecrets, runtimeSecrets)
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

func DetectAIConfigForExport() (*DeploymentAIConfig, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load Station config: %w", err)
	}

	return &DeploymentAIConfig{
		Provider:          cfg.AIProvider,
		Model:             cfg.AIModel,
		APIKey:            cfg.AIAPIKey,
		AuthType:          cfg.AIAuthType,
		OAuthToken:        cfg.AIOAuthToken,
		OAuthRefreshToken: cfg.AIOAuthRefreshToken,
		OAuthExpiresAt:    cfg.AIOAuthExpiresAt,
	}, nil
}

func DetectCloudShipConfigForExport() *DeploymentCloudShipConfig {
	return detectCloudShipConfigForDeployment()
}

func DetectTelemetryConfigForExport() *DeploymentTelemetryConfig {
	return detectTelemetryConfigForDeployment()
}

func LoadEnvironmentConfigForExport(envName string) (*EnvironmentConfig, error) {
	return loadEnvironmentConfig(envName)
}
